import { Context } from '@deepseek-ai/cordis'
import AgentRegistry, { Inbox } from '@deepseek-ai/dsh-agent'
import type { Agent } from '@deepseek-ai/dsh-agent'
import { CallId } from '@deepseek-ai/dsh-llm'
import { Session, SessionId } from '@deepseek-ai/dsh-session'
import SystemPrompt from '@deepseek-ai/dsh-system-prompt'
import ToolRuntime from '@deepseek-ai/dsh-tools'
import { describe, expect, it, vi } from 'vitest'
import type EnvironmentController from '../src/hev-runtime/index.ts'
import { EnvironmentId, HevCliError, StatusCode } from '../src/hev-runtime/index.ts'
import type { Environment } from '../src/hev-runtime/index.ts'
import * as HevTool from '../src/hev-tool/index.ts'

const signal = new AbortController().signal
let calls = 0

function environment(name = 'coding'): Environment {
  return {
    source: 'dsh',
    id: EnvironmentId(name === 'base' ? 'base' : `env-${name}`),
    name,
    revision: 1,
    skills: [{ skillKey: 'code-review', policy: { kind: 'auto' } }],
  }
}

function agent(id: string): Agent {
  const session = Session.create(SessionId(id))
  return {
    id: session.id,
    options: {},
    session,
    inbox: new Inbox(session, { inserted: () => {}, discarded: () => {}, claimed: () => {} }),
    status: 'idle',
    ctx: new Context(),
    send: () => {},
    followup: () => {},
    steer: () => {},
    inject: () => {},
    cancel: () => {},
    runMaintenance: task => task(new AbortController().signal),
    whenIdle: () => Promise.resolve(),
  }
}

function text(result: { content: Array<{ type: string; text?: string }> }): string {
  return result.content.flatMap(block => block.type === 'text' && block.text !== undefined ? [block.text] : []).join('')
}

async function setup() {
  const ctx = new Context()
  await ctx.plugin(SystemPrompt)
  await ctx.plugin(ToolRuntime)
  await ctx.plugin(AgentRegistry)
  const current = vi.fn(async () => undefined)
  const use = vi.fn(async () => environment())
  const quit = vi.fn(async () => environment('base'))
  const create = vi.fn(async () => ({ message: 'environment created', environment: environment('review') }))
  const rename = vi.fn(async () => ({ message: 'environment renamed', environment: environment('backend') }))
  const deleteEnvironment = vi.fn(async () => ({ message: 'environment deleted', environment: environment('scratch') }))
  const list = vi.fn(async () => [{ source: 'dsh' as const, id: EnvironmentId('base'), name: 'base', revision: 1 }])
  const addSkill = vi.fn(async () => ({
    message: 'skill added to environment',
    environmentSkill: { skillKey: 'code-review', policy: { kind: 'auto' as const } },
    environments: [{ source: 'dsh' as const, id: EnvironmentId('env-coding'), name: 'coding', revision: 2 }],
  }))
  const removeSkill = vi.fn(async () => ({
    message: 'skill removed from environment',
    skillKey: 'code-review',
    environments: [{ source: 'dsh' as const, id: EnvironmentId('env-coding'), name: 'coding', revision: 3 }],
  }))
  const listEnvironmentSkills = vi.fn(async () => environment('review'))
  const listGlobalSkills = vi.fn(async () => [{ name: 'code-review' }])
  ctx.provide('environment', {
    current, use, quit, create, rename, deleteEnvironment, list, addSkill, removeSkill, listEnvironmentSkills, listGlobalSkills,
  } as unknown as EnvironmentController)
  await ctx.plugin(HevTool)
  return { ctx, current, use, quit, create, rename, deleteEnvironment, list, addSkill, removeSkill, listEnvironmentSkills, listGlobalSkills }
}

function callTool(ctx: Context, name: string, args: unknown, caller?: Agent) {
  return ctx.tools.execute({
    signal,
    callId: CallId(`hev-tool-${++calls}`),
    name,
    arguments: args,
    ...(caller === undefined ? {} : { agent: caller }),
  })
}

describe('@slimzeo/hev-dsh-plugin/hev-tool', () => {
  it('uses the exact calling Agent for Session-scoped operations', async () => {
    const { ctx, current, use, quit } = await setup()
    const caller = agent('caller-session')

    expect(text(await callTool(ctx, 'hev_env_status', {}, caller))).toBe('hev not activated')
    expect(text(await callTool(ctx, 'hev_env_use', { environment: 'coding' }, caller))).toContain('coding')
    expect(text(await callTool(ctx, 'hev_env_quit', {}, caller))).toContain('base')

    expect(current).toHaveBeenCalledWith(caller.session, signal)
    expect(use).toHaveBeenCalledWith(caller, 'coding', signal)
    expect(quit).toHaveBeenCalledWith(caller, signal)
  })

  it('exposes Environment and Skill management without a sessionId argument', async () => {
    const { ctx, create, rename, deleteEnvironment, list, addSkill, removeSkill, listEnvironmentSkills, listGlobalSkills } = await setup()
    const caller = agent('manager-session')

    expect(text(await callTool(ctx, 'hev_env_create', { name: 'review' }, caller))).toContain('review')
    expect(text(await callTool(ctx, 'hev_env_rename', { environment: 'review', name: 'backend' }, caller))).toContain('backend')
    expect(text(await callTool(ctx, 'hev_env_delete', { environment: 'scratch' }, caller))).toBe('deleted scratch (env-scratch)')
    expect(text(await callTool(ctx, 'hev_env_list', {}, caller))).toBe('environments:\n- base (base rev 1)')
    expect(text(await callTool(ctx, 'hev_skill_add', {
      skill: 'code-review', environments: ['coding'], policy: 'auto',
    }, caller))).toBe('added code-review to coding')
    expect(text(await callTool(ctx, 'hev_skill_remove', {
      skill: 'code-review', environments: ['coding'],
    }, caller))).toBe('removed code-review from coding')
    expect(text(await callTool(ctx, 'hev_skill_list', { environment: 'review' }, caller))).toContain('review:')
    expect(text(await callTool(ctx, 'hev_skill_list', { global: true }, caller))).toBe('global:\n- code-review')

    expect(text(await callTool(ctx, 'hev_skill_list', {
      global: true, environment: 'review',
    }, caller))).toBe('Error: Set either global=true or environment, not both.')

    expect(create).toHaveBeenCalledWith('review', signal)
    expect(rename).toHaveBeenCalledWith('review', 'backend', signal)
    expect(deleteEnvironment).toHaveBeenCalledWith('scratch', signal)
    expect(list).toHaveBeenCalledWith(signal)
    expect(addSkill).toHaveBeenCalledWith('code-review', ['coding'], 'auto', signal)
    expect(removeSkill).toHaveBeenCalledWith('code-review', ['coding'], signal)
    expect(listEnvironmentSkills).toHaveBeenCalledWith('review', signal)
    expect(listGlobalSkills).toHaveBeenCalledWith(caller, signal)
    const schemas = ctx.tools.schemas().filter(schema => schema.name.startsWith('hev_'))
    expect(schemas.map(schema => schema.name).sort()).toEqual([
      'hev_env_create',
      'hev_env_delete',
      'hev_env_list',
      'hev_env_quit',
      'hev_env_rename',
      'hev_env_status',
      'hev_env_use',
      'hev_skill_add',
      'hev_skill_list',
      'hev_skill_remove',
    ])
    for (const schema of schemas) {
      const properties = (schema.parameters as { properties?: Record<string, unknown> }).properties ?? {}
      expect(properties).not.toHaveProperty('sessionId')
      expect(properties).not.toHaveProperty('session_id')
      expect(properties).not.toHaveProperty('source')
      expect(properties).not.toHaveProperty('stateDir')
      expect(properties).not.toHaveProperty('state_dir')
    }
  })

  it('rejects calls without an owning Agent', async () => {
    const { ctx } = await setup()
    const result = await callTool(ctx, 'hev_env_status', {})
    expect(result.isError).toBe(true)
    expect(text(result)).toContain('require a calling Agent')
  })

  it('returns recovery prompts to the Agent and keeps diagnostics in logs', async () => {
    const { ctx, use } = await setup()
    const warnings: string[] = []
    ctx.logger.warn = ((message: unknown) => { warnings.push(String(message)) }) as typeof ctx.logger.warn
    use.mockRejectedValueOnce(new HevCliError(
      StatusCode.NotFound,
      'environment not found: missing',
      'Call hev_env_list and retry with an existing Environment.',
    ))

    const result = await callTool(ctx, 'hev_env_use', { environment: 'missing' }, agent('caller'))

    expect(result.isError).toBe(true)
    expect(text(result)).toBe('Error: Call hev_env_list and retry with an existing Environment.')
    expect(warnings).toEqual(['hev tool failed (404): environment not found: missing'])
  })
})
