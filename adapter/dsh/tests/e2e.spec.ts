/**
 * End-to-end proof: a Loader-booted composition (a real `cordis.yml`, not a
 * hand-built context), a real Agent from the shipping loop, two real turns
 * driven by a scripted adapter, and every claim read back FROM THE SESSION LOG.
 *
 * The audit built in `auditFromLog` is the point: after the fact, from the log
 * alone, a reader can tell which skills hev let the model see, which it withheld
 * from the model but left user-invocable, which it excluded and why, and which
 * ones were actually read.
 */
import { afterEach, describe, expect, it } from 'vitest'
import { chmod, mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { Context } from '@deepseek-ai/cordis'
import Loader from '@deepseek-ai/cordis-plugin-loader'
import Include from '@deepseek-ai/cordis-plugin-include'
import LlmRuntime, { CallId, createUserMessage, LlmAdapter } from '@deepseek-ai/dsh-llm'
import type { GenerateOptions, StreamChunk } from '@deepseek-ai/dsh-llm'
import SessionStore, { SessionId, type Session, type SessionEvent } from '@deepseek-ai/dsh-session'
import SystemPrompt from '@deepseek-ai/dsh-system-prompt'
import ToolRuntime from '@deepseek-ai/dsh-tools'
import AgentRegistry from '@deepseek-ai/dsh-agent'
import AgentLoop from '@deepseek-ai/dsh-agent-loop'
import CommandRuntime from '@deepseek-ai/dsh-commands'
import SkillRegistry from '@deepseek-ai/dsh-skill'
import * as SkillFileSystem from '@deepseek-ai/dsh-skill-filesystem'
import * as toolSkill from '@deepseek-ai/dsh-tool-skill'
import * as hev from '../src/index.ts'

const FAKE_CLI = fileURLToPath(new URL('./fixtures/fake-hev.mjs', import.meta.url))
const signal = new AbortController().signal

let context: Context | undefined
let root: string | undefined

afterEach(async () => {
  await context?.fiber.dispose()
  context = undefined
  if (root !== undefined) await rm(root, { recursive: true, force: true })
  root = undefined
})

/** Emits one `skill` tool call on the first request, plain text afterwards. */
class ScriptedAdapter extends LlmAdapter {
  readonly requests: GenerateOptions[] = []

  async * stream(options: GenerateOptions): AsyncIterable<StreamChunk> {
    this.requests.push(options)
    await Promise.resolve()
    if (this.requests.length === 1) {
      const call = { type: 'tool-call' as const, id: CallId('call-1'), name: 'skill', arguments: '{"name":"code-review"}' }
      yield { type: 'block-start', index: 0, blockType: 'tool-call' }
      yield { type: 'tool-call-delta', index: 0, id: call.id, name: call.name, argumentsDelta: call.arguments }
      yield { type: 'block-end', index: 0, block: call }
      yield { type: 'finish', reason: { kind: 'tool-calls' } }
      return
    }
    yield { type: 'block-start', index: 0, blockType: 'text' }
    yield { type: 'text-delta', index: 0, text: 'done' }
    yield { type: 'block-end', index: 0, block: { type: 'text', text: 'done' } }
    yield { type: 'finish', reason: { kind: 'stop' } }
  }
}

async function writeSkillDir(dir: string, name: string, description: string, body: string): Promise<string> {
  await mkdir(dir, { recursive: true })
  await writeFile(join(dir, 'SKILL.md'), `---\nname: ${name}\ndescription: ${description}\n---\n\n${body}\n`)
  return dir
}

/** Boot a real composition through the Loader, exactly as a deployment would. */
async function boot(): Promise<{ ctx: Context; adapter: ScriptedAdapter }> {
  root = await mkdtemp(join(tmpdir(), 'hev-e2e-'))
  await chmod(FAKE_CLI, 0o755)
  const home = join(root, 'home')
  const store = join(root, 'skills')

  await writeSkillDir(join(home, '.dsh/skills/base-skill'), 'base-skill', 'Base skill', 'Base body.')
  const codeReview = await writeSkillDir(join(store, 'code-review'), 'code-review', 'Review a diff', 'Code review body.')
  const secret = await writeSkillDir(join(store, 'secret-skill'), 'secret-skill', 'Withheld from the model', 'Secret body.')
  const mismatched = await writeSkillDir(join(store, 'actual-name'), 'actual-name', 'Mismatched', 'Mismatched body.')

  await writeFile(join(root, 'db.json'), JSON.stringify({
    environments: {
      base: { id: 'env_base', revision: 1, skills: [] },
      coding: {
        id: 'env_01',
        revision: 3,
        skills: [
          { id: 'skill_01', skillName: 'code-review', realPath: codeReview },
          { id: 'skill_02', skillName: 'secret-skill', realPath: secret, mode: { type: 'off' } },
          { id: 'skill_03', skillName: 'renamed-skill', realPath: mismatched },
          { id: 'skill_04', skillName: 'ghost-skill', realPath: join(store, 'absent') },
        ],
      },
    },
  }))
  process.env.FAKE_HEV_DB = join(root, 'db.json')

  const configPath = join(root, 'cordis.yml')
  await writeFile(configPath, [
    "- name: '@deepseek-ai/dsh-session'",
    "- name: '@deepseek-ai/dsh-llm'",
    "- name: '@deepseek-ai/dsh-system-prompt'",
    "- name: '@deepseek-ai/dsh-tools'",
    "- name: '@deepseek-ai/dsh-agent'",
    "- name: '@deepseek-ai/dsh-agent-loop'",
    "- name: '@deepseek-ai/dsh-commands'",
    "- name: '@deepseek-ai/dsh-skill'",
    "- name: '@deepseek-ai/dsh-skill-filesystem'",
    '  config:',
    `    dshHome: '${join(home, '.dsh')}'`,
    `    agentsHome: '${join(home, '.agents')}'`,
    '    watch: false',
    "- name: '@deepseek-ai/dsh-tool-skill'",
    "- name: '@hev/dsh-plugin'",
    '  config:',
    `    cli: '${FAKE_CLI}'`,
    '',
  ].join('\n'))

  const ctx = new Context()
  context = ctx
  ctx.baseUrl = `${pathToFileURL(root).href}/`
  await ctx.plugin(Loader)
  ctx.loader.builtins.include = Include
  const modules = new Map<string, unknown>([
    ['@deepseek-ai/dsh-session', SessionStore],
    ['@deepseek-ai/dsh-llm', LlmRuntime],
    ['@deepseek-ai/dsh-system-prompt', SystemPrompt],
    ['@deepseek-ai/dsh-tools', ToolRuntime],
    ['@deepseek-ai/dsh-agent', AgentRegistry],
    ['@deepseek-ai/dsh-agent-loop', AgentLoop],
    ['@deepseek-ai/dsh-commands', CommandRuntime],
    ['@deepseek-ai/dsh-skill', SkillRegistry],
    ['@deepseek-ai/dsh-skill-filesystem', SkillFileSystem],
    ['@deepseek-ai/dsh-tool-skill', toolSkill],
    ['@hev/dsh-plugin', hev],
  ])
  ctx.loader.internal = {
    version: 'v2',
    async import(specifier: string) {
      if (!modules.has(specifier)) throw new Error(`unexpected Loader import: ${specifier}`)
      return modules.get(specifier)
    },
  } as unknown as NonNullable<typeof ctx.loader.internal>
  await ctx.loader.create({ name: 'cordis:include', config: { path: pathToFileURL(configPath).href } })
  await ctx.loader.await()

  const unloaded = [...ctx.loader.entries()]
    .filter(entry => entry.fiber === undefined && !entry.disabled)
    .map(entry => entry.options.name)
  expect(unloaded).toEqual([])

  const adapter = new ScriptedAdapter()
  ctx.llm.registerAdapter(['mock'], adapter)
  return { ctx, adapter }
}

interface Audit {
  /** Every skill hev decided about, as recorded in the log. */
  decisions: { outcome: string; name: string; env: string; mode: string; reason: string | undefined }[]
  /** The model-facing catalog of the latest step. */
  offeredToModel: string[]
  /** Skills the MODEL loaded through the `skill` tool. */
  readByModel: string[]
  /** Skills the USER loaded through the `/name` gesture. */
  readByUser: string[]
  /**
   * Where the bodies that actually reached the conversation came from: every
   * loaded skill states its base directory, so the log proves the content was
   * resolved from the env's real path and not from a default skill root.
   */
  bodySources: string[]
}

/** Reconstruct the whole gating story from durable session events only. */
function auditFromLog(session: Session): Audit {
  const events: readonly SessionEvent[] = session.events
  const userMessages = events.filter((event): event is Extract<SessionEvent, { type: 'user/message' }> => event.type === 'user/message')

  const catalogs = userMessages.filter(event => event.data.source.kind === 'skill-catalog')
  const latest = catalogs.at(-1)?.data.source as { entries?: { name: string }[] } | undefined

  const decisionLine = /^hev skill (\S+): (\S+) \(env=(\S+) mode=(\S+)\)(?: — (.+))?$/u
  const decisions = events
    .filter((event): event is Extract<SessionEvent, { type: 'command/done' }> => event.type === 'command/done')
    .flatMap(event => (event.data.text ?? '').split('\n'))
    .map(line => decisionLine.exec(line))
    .filter((match): match is RegExpExecArray => match !== null)
    .map(match => ({
      outcome: match[1] as string,
      name: match[2] as string,
      env: match[3] as string,
      mode: match[4] as string,
      reason: match[5],
    }))

  const readByModel = events
    .filter((event): event is Extract<SessionEvent, { type: 'tool/call' }> => event.type === 'tool/call' && event.data.name === 'skill')
    .map(event => (JSON.parse(event.data.arguments) as { name: string }).name)

  const readByUser = userMessages
    .filter(event => event.data.source.kind === 'skill-invocation')
    .map(event => (event.data.source as { name: string }).name)

  // Both load paths carry the rendered `<skill_content>` block: the model's one
  // through a tool result, the user's through an injected message.
  const loadedTexts = [
    ...events
      .filter((event): event is Extract<SessionEvent, { type: 'tool/result' }> => event.type === 'tool/result')
      // A tool result is one `tool-result` block wrapping the model-facing blocks.
      .flatMap(event => event.data.message.content)
      .flatMap(block => block.type === 'tool-result' ? block.content : [block]),
    ...userMessages
      .filter(event => event.data.source.kind === 'skill-invocation')
      .flatMap(event => event.data.content),
  ]
    .filter((block): block is { type: 'text'; text: string } => block.type === 'text')
    .map(block => block.text)

  const bodySources = loadedTexts
    .flatMap(text => [...text.matchAll(/Base directory for this skill: (.+)/gu)])
    .map(match => match[1] as string)

  return {
    decisions,
    offeredToModel: (latest?.entries ?? []).map(entry => entry.name).sort(),
    readByModel,
    readByUser,
    bodySources,
  }
}

describe('hev adapter, end to end through the Loader', () => {
  it('gates the catalog for a real agent and leaves the whole decision auditable in the session log', { timeout: 120_000 }, async () => {
    const { ctx, adapter } = await boot()
    const agent = ctx.agentLoop.create(SessionId('hev-e2e'), { provider: 'mock', model: 'mock' }, { cwd: join(root as string, 'workspace') })

    const activation = await ctx.commands.execute(agent, '/hev env activate coding', [], signal)
    expect(activation?.result.kind).toBe('success')

    // Turn 1: the model receives the gated catalog and loads one skill.
    agent.followup(createUserMessage({ content: [{ type: 'text', text: 'review my diff' }], source: { kind: 'user' } }))
    await agent.whenIdle()

    // Turn 2: the user explicitly invokes the skill hev withheld from the model.
    agent.followup(createUserMessage({ content: [{ type: 'text', text: '/secret-skill please' }], source: { kind: 'user' } }))
    await agent.whenIdle()

    // The model really ran: one tool-call request, then two plain completions.
    expect(adapter.requests).toHaveLength(3)
    const firstRequest = JSON.stringify(adapter.requests[0]?.messages)
    expect(firstRequest).toContain('code-review')
    // What hev withheld never entered a request.
    expect(firstRequest).not.toContain('secret-skill')

    const audit = auditFromLog(agent.session)

    // 1. Which skills were admitted, withheld, or excluded — and why.
    expect(audit.decisions).toEqual([
      { outcome: 'admitted', name: 'code-review', env: 'coding', mode: 'auto', reason: undefined },
      { outcome: 'user-only', name: 'secret-skill', env: 'coding', mode: 'off', reason: undefined },
      {
        outcome: 'excluded',
        name: 'renamed-skill',
        env: 'coding',
        mode: 'auto',
        reason: 'renamed-skill: frontmatter declares name "actual-name" — the registry and the file must agree',
      },
      {
        outcome: 'excluded',
        name: 'ghost-skill',
        env: 'coding',
        mode: 'auto',
        reason: `ghost-skill: realPath is unreadable (${join(root as string, 'skills', 'absent')})`,
      },
    ])

    // 2. The model-facing catalog matches the admitted set plus the host baseline.
    expect(audit.offeredToModel).toEqual(['base-skill', 'code-review'])

    // 3. Who read what.
    expect(audit.readByModel).toEqual(['code-review'])
    expect(audit.readByUser).toEqual(['secret-skill'])

    // 4. Both bodies were resolved from the env's real paths, not from a default root.
    expect(audit.bodySources).toEqual([
      join(root as string, 'skills', 'code-review'),
      join(root as string, 'skills', 'secret-skill'),
    ])

    // 5. The withheld skill's body reached the conversation only through the
    //    user gesture, and its injection is recorded as such.
    const injected = agent.session.events.filter(event =>
      event.type === 'user/message' && event.data.source.kind === 'skill-invocation')
    expect(JSON.stringify(injected)).toContain('Secret body.')
  })
})
