/**
 * Adapter prototype tests: drive `/hev` through the real dsh command registry
 * and assert what the MODEL would see — the published skill catalog — plus the
 * isolation, ordering, and rollback behaviour the §7 contract implies.
 */
import { beforeAll, describe, expect, it } from 'vitest'
import { chmod, mkdir, mkdtemp, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { Context } from '@deepseek-ai/cordis'
import { createUserMessage, CallId } from '@deepseek-ai/dsh-llm'
import { createScope } from '@deepseek-ai/dsh-scope'
import { Session, SessionId, type SessionEvent } from '@deepseek-ai/dsh-session'
import SystemPrompt from '@deepseek-ai/dsh-system-prompt'
import ToolRuntime from '@deepseek-ai/dsh-tools'
import AgentRegistry, { agentEvents, Inbox, type Agent } from '@deepseek-ai/dsh-agent'
import SkillRegistry from '@deepseek-ai/dsh-skill'
import * as SkillFileSystem from '@deepseek-ai/dsh-skill-filesystem'
import * as toolSkill from '@deepseek-ai/dsh-tool-skill'
import CommandRuntime from '@deepseek-ai/dsh-commands'
import * as hev from '../src/index.ts'

const FAKE_CLI = fileURLToPath(new URL('./fixtures/fake-hev.mjs', import.meta.url))
const signal = new AbortController().signal

beforeAll(async () => { await chmod(FAKE_CLI, 0o755) })

async function writeSkillDir(root: string, name: string, description: string, body: string, extra = ''): Promise<string> {
  const dir = join(root, name)
  await mkdir(dir, { recursive: true })
  await writeFile(join(dir, 'SKILL.md'), `---\nname: ${name}\ndescription: ${description}\n${extra}---\n\n${body}\n`)
  return dir
}

interface World {
  ctx: Context
  db: string
  home: string
  store: string
}

async function world(): Promise<World> {
  const home = await mkdtemp(join(tmpdir(), 'hev-home-'))
  const store = await mkdtemp(join(tmpdir(), 'hev-skills-'))
  // The always-present baseline comes from dsh's own user root, so the tests can
  // tell "hev added a skill" apart from "hev replaced everything".
  await writeSkillDir(join(home, '.dsh/skills'), 'base-skill', 'Base skill', 'Base body.')

  const codeReview = await writeSkillDir(store, 'code-review', 'Review a diff', 'Code review body.')
  const noisy = await writeSkillDir(store, 'noisy-skill', 'Noisy skill', 'Noisy body.')
  const codingShared = await writeSkillDir(join(store, 'coding'), 'shared-skill', 'Shared, coding flavour', 'Coding shared body.')
  const writingShared = await writeSkillDir(join(store, 'writing'), 'shared-skill', 'Shared, writing flavour', 'Writing shared body.')
  // Registry says `renamed-skill`; the file says `actual-name`. dsh would drop
  // this definition at load time, so the adapter must refuse it up front.
  const mismatched = await writeSkillDir(store, 'actual-name', 'Mismatched', 'Mismatched body.')

  const db = join(store, 'db.json')
  await writeFile(db, JSON.stringify({
    environments: {
      base: { id: 'env_base', revision: 1, skills: [] },
      coding: {
        id: 'env_01',
        revision: 2,
        skills: [
          { id: 'skill_01', skillName: 'code-review', realPath: codeReview },
          { id: 'skill_02', skillName: 'noisy-skill', realPath: noisy, mode: { type: 'off' } },
          { id: 'skill_03', skillName: 'shared-skill', realPath: codingShared },
        ],
      },
      writing: {
        id: 'env_02',
        revision: 1,
        skills: [{ id: 'skill_04', skillName: 'shared-skill', realPath: writingShared }],
      },
      broken: {
        id: 'env_03',
        revision: 1,
        skills: [
          { id: 'skill_05', skillName: 'renamed-skill', realPath: mismatched },
          { id: 'skill_06', skillName: 'ghost-skill', realPath: join(store, 'does-not-exist') },
        ],
      },
    },
  }))

  process.env.FAKE_HEV_DB = db
  const ctx = new Context()
  await ctx.plugin(SystemPrompt)
  await ctx.plugin(ToolRuntime)
  await ctx.plugin(AgentRegistry)
  await ctx.plugin(CommandRuntime)
  await ctx.plugin(SkillRegistry)
  await ctx.plugin(SkillFileSystem, { dshHome: join(home, '.dsh'), agentsHome: join(home, '.agents'), watch: false })
  await ctx.plugin(toolSkill, {})
  await ctx.plugin(hev, { cli: FAKE_CLI })
  return { ctx, db, home, store }
}

/** An agent shaped like the real one: scope key = the agent, `agent.ctx` = its scope context. */
async function mintAgent(ctx: Context, id: string, cwd: string): Promise<Agent> {
  const sessionId = SessionId(id)
  const session = Session.create(sessionId, [], { version: 0, id: sessionId, createdAt: 0, cwd })
  const agent = {
    id: sessionId,
    options: {},
    session,
    inbox: new Inbox(session, { inserted: () => {}, discarded: () => {}, claimed: () => {} }),
    status: 'running',
    ctx: new Context(),
    send: () => {},
    followup: () => {},
    steer: () => {},
    inject: () => { throw new Error('unexpected inject') },
    cancel() {},
    runMaintenance: (task: (abort: AbortSignal) => unknown) => task(new AbortController().signal),
    whenIdle: () => Promise.resolve(),
  } as unknown as Agent
  let scoped!: Context
  await ctx.plugin(Object.assign((inner: Context) => { scoped = createScope(inner, agent).ctx }, { inject: ['tools'] }))
  Object.assign(agent, { ctx: scoped })
  return agent
}

let turn = 0
async function step(ctx: Context, agent: Agent): Promise<string[]> {
  turn += 1
  agent.session.append('turn/start', { turn })
  agent.session.append('user/message', createUserMessage({
    content: [{ type: 'text', text: `turn ${String(turn)}` }],
    source: { kind: 'user' },
  }), { surfaceOp: 'append' })
  const decision = await agentEvents(ctx, agent).waterfall(
    'agent/pre-step',
    { messages: [], turn, step: 1, signal },
    () => Promise.resolve({ kind: 'enter' as const, messages: [] }),
  )
  if (decision.kind === 'enter') {
    for (const message of decision.messages) agent.session.append('user/message', message, { surfaceOp: 'append' })
  }
  return catalogNames(agent.session)
}

function catalogNames(session: Session): string[] {
  const catalogs = session.events.filter((event): event is Extract<SessionEvent, { type: 'user/message' }> =>
    event.type === 'user/message' && event.data.source.kind === 'skill-catalog')
  const latest = catalogs.at(-1)
  const entries = (latest?.data.source as { entries?: { name: string }[] } | undefined)?.entries ?? []
  return entries.map(entry => entry.name).sort()
}

async function runCommand(ctx: Context, agent: Agent, line: string): Promise<{ kind: string; text?: string }> {
  const execution = await ctx.commands.execute(agent, line, [], signal)
  if (execution === undefined) throw new Error(`command did not resolve: ${line}`)
  return execution.result
}

describe('hev dsh adapter', () => {
  it('activates an environment mid-session and publishes it to that agent alone', async () => {
    const { ctx } = await world()
    const mine = await mintAgent(ctx, 'hev-mine', '/workspace/mine')
    const other = await mintAgent(ctx, 'hev-other', '/workspace/other')

    expect(await step(ctx, mine)).toEqual(['base-skill'])

    const activated = await runCommand(ctx, mine, '/hev env activate coding')
    expect(activated.kind).toBe('success')
    expect(activated.text).toContain('coding@2')

    // `noisy-skill` is mode=off: absent from the model catalog...
    expect(await step(ctx, mine)).toEqual(['base-skill', 'code-review', 'shared-skill'])
    // ...but still loadable through the user's explicit gesture.
    const off = await ctx.skills.get('noisy-skill', { cwd: '/workspace/mine', scope: mine })
    expect(off?.content).toBe('Noisy body.')
    expect(off?.metadata).toMatchObject({ hevEnvName: 'coding', hevMode: { type: 'off' } })

    // The model-facing loader returns the body from the env's realPath.
    const loaded = await ctx.tools.execute({
      signal, callId: CallId('load-1'), name: 'skill', arguments: { name: 'code-review' }, agent: mine,
    })
    expect(loaded.isError).toBe(false)
    expect(JSON.stringify(loaded.content)).toContain('Code review body.')

    // The other live session is untouched.
    expect(await step(ctx, other)).toEqual(['base-skill'])
  })

  it('resolves a duplicate skill name by environment-group order', async () => {
    const { ctx } = await world()
    const agent = await mintAgent(ctx, 'hev-order', '/workspace/order')
    expect((await runCommand(ctx, agent, '/hev env activate coding writing')).kind).toBe('success')
    const shared = await ctx.skills.get('shared-skill', { cwd: '/workspace/order', scope: agent })
    expect(shared?.content).toBe('Coding shared body.')

    const reversed = await mintAgent(ctx, 'hev-order-rev', '/workspace/order-rev')
    expect((await runCommand(ctx, reversed, '/hev env activate writing coding')).kind).toBe('success')
    const flipped = await ctx.skills.get('shared-skill', { cwd: '/workspace/order-rev', scope: reversed })
    expect(flipped?.content).toBe('Writing shared body.')
  })

  it('refuses a skill whose frontmatter name disagrees with the registry, and reports it', async () => {
    const { ctx } = await world()
    const agent = await mintAgent(ctx, 'hev-broken', '/workspace/broken')
    const result = await runCommand(ctx, agent, '/hev env activate broken')
    expect(result.kind).toBe('success')
    expect(result.text).toContain('0 model-visible, 0 user-only, 2 excluded')
    expect(result.text).toContain('frontmatter declares name "actual-name"')
    expect(result.text).toContain('ghost-skill: realPath is unreadable')
    expect(await step(ctx, agent)).toEqual(['base-skill'])
  })

  it('keeps the previous environment when the CLI fails, and reverts on deactivate', async () => {
    const { ctx } = await world()
    const agent = await mintAgent(ctx, 'hev-rollback', '/workspace/rollback')
    expect((await runCommand(ctx, agent, '/hev env activate coding')).kind).toBe('success')
    expect(await step(ctx, agent)).toEqual(['base-skill', 'code-review', 'shared-skill'])

    const failed = await runCommand(ctx, agent, '/hev env activate ghost')
    expect(failed.kind).toBe('error')
    expect(failed.text).toContain('ENV_NOT_FOUND')
    // The failed switch left the working environment in place.
    expect(await step(ctx, agent)).toEqual(['base-skill', 'code-review', 'shared-skill'])
    expect((await runCommand(ctx, agent, '/hev env status')).text).toContain('coding')

    expect((await runCommand(ctx, agent, '/hev env deactivate')).kind).toBe('success')
    expect(await step(ctx, agent)).toEqual(['base-skill'])
  })
})
