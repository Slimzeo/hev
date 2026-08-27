import { execFile } from 'node:child_process'
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { promisify } from 'node:util'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Context } from '@deepseek-ai/cordis'
import Include from '@deepseek-ai/cordis-plugin-include'
import Loader from '@deepseek-ai/cordis-plugin-loader'
import AgentRegistry, { Inbox } from '@deepseek-ai/dsh-agent'
import type { Agent } from '@deepseek-ai/dsh-agent'
import { composeEntries, loadOverlayPatches } from '@deepseek-ai/dsh-app-boot'
import CommandRuntime from '@deepseek-ai/dsh-commands'
import { CallId } from '@deepseek-ai/dsh-llm'
import { Session, SessionId } from '@deepseek-ai/dsh-session'
import SkillRegistry from '@deepseek-ai/dsh-skill'
import SystemPrompt from '@deepseek-ai/dsh-system-prompt'
import ToolRuntime from '@deepseek-ai/dsh-tools'
import EnvironmentController from '@slimzeo/hev-dsh-plugin/hev-runtime'
import HevSkillRegistry from '@slimzeo/hev-dsh-plugin/hev-skill-registry'
import * as HevTool from '@slimzeo/hev-dsh-plugin/hev-tool'

const execFileAsync = promisify(execFile)
const adapterRoot = fileURLToPath(new URL('..', import.meta.url))
const hevRoot = fileURLToPath(new URL('../../..', import.meta.url))
const dshRoot = fileURLToPath(new URL('../../../../deepseek-harness/', import.meta.url))
const signal = new AbortController().signal

let context: Context | undefined
let temporaryRoot: string | undefined

afterEach(async () => {
  vi.unstubAllEnvs()
  await context?.fiber.dispose()
  context = undefined
  if (temporaryRoot !== undefined) await rm(temporaryRoot, { recursive: true, force: true })
  temporaryRoot = undefined
})

function agent(id: string): Agent {
  const sessionId = SessionId(id)
  const session = Session.create(sessionId, [], {
    version: 0, id: sessionId, createdAt: 0, cwd: '/workspace',
  })
  return {
    id: sessionId,
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

describe('hev DSH bundle', () => {
  it('runs the real Go CLI and applies Environment selection per exact Session', { timeout: 120_000 }, async () => {
    temporaryRoot = await mkdtemp(join(tmpdir(), 'hev-dsh-'))
    const home = join(temporaryRoot, 'home')
    const executable = join(temporaryRoot, 'hev')
    await execFileAsync('go', ['build', '-o', executable, './cmd/hev'], {
      cwd: hevRoot,
      env: { ...process.env, GOCACHE: '/private/tmp/hev-go-cache' },
    })
    vi.stubEnv('DSH_HOME', home)

    const warnings: string[] = []
    const entries = composeEntries([
      loadOverlayPatches('hev-test', join(dshRoot, 'packages/bundle/base/cordis.patch.yml')),
      loadOverlayPatches('hev-test', join(adapterRoot, 'cordis.patch.yml')),
      [{ id: 'hev-runtime', config: { executable } }],
    ], warning => void warnings.push(warning))
    expect(warnings).toEqual([])

    const selectedIds = new Set([
      'agent',
      'commands',
      'skill',
      'system-prompt',
      'tools',
      'hev-runtime',
      'hev-skill-registry',
      'hev-tool',
    ])
    const selected = entries.filter(entry => entry.id !== undefined && selectedIds.has(entry.id))
    const configPath = join(temporaryRoot, 'cordis.json')
    await writeFile(configPath, `${JSON.stringify(selected, null, 2)}\n`)

    const ctx = new Context()
    context = ctx
    ctx.baseUrl = `${pathToFileURL(temporaryRoot).href}/`
    await ctx.plugin(Loader)
    ctx.loader.builtins.include = Include
    const modules = new Map<string, unknown>([
      ['@deepseek-ai/dsh-agent', AgentRegistry],
      ['@deepseek-ai/dsh-commands', CommandRuntime],
      ['@deepseek-ai/dsh-system-prompt', SystemPrompt],
      ['@deepseek-ai/dsh-tools', ToolRuntime],
      ['@deepseek-ai/dsh-skill', SkillRegistry],
      ['@slimzeo/hev-dsh-plugin/hev-runtime', EnvironmentController],
      ['@slimzeo/hev-dsh-plugin/hev-skill-registry', HevSkillRegistry],
      ['@slimzeo/hev-dsh-plugin/hev-tool', HevTool],
    ])
    ctx.loader.internal = {
      version: 'v2',
      async import(specifier: string) {
        if (!modules.has(specifier)) throw new Error(`unexpected Loader import: ${specifier}`)
        return modules.get(specifier)
      },
    } as unknown as NonNullable<typeof ctx.loader.internal>
    await ctx.loader.create({
      name: 'cordis:include',
      config: { path: pathToFileURL(configPath).href },
    })
    await ctx.loader.await()

    const rows = new Map([...ctx.loader.entries()]
      .filter(entry => entry.options.id !== undefined)
      .map(entry => [entry.options.id as string, entry]))
    expect(rows.get('skill')?.options).toMatchObject({
      name: '@deepseek-ai/dsh-skill', disabled: true,
    })
    expect(rows.get('skill')?.fiber).toBeUndefined()
    expect(rows.get('hev-runtime')?.fiber).toBeDefined()
    expect(rows.get('hev-skill-registry')?.fiber).toBeDefined()
    expect(rows.get('hev-tool')?.fiber).toBeDefined()

    for (const name of ['allowed-skill', 'off-skill', 'outside-skill']) {
      ctx.skills.register({
        name,
        description: `${name} description`,
        source: 'integration',
        content: `${name} content`,
      })
    }
    const owner = agent('hev-integration')
    const other = agent('hev-other')
    ctx.agents.register(owner)
    ctx.agents.register(other)

    const view = { scope: owner, signal }
    const otherView = { scope: other, signal }
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    await expect(ctx.commands.execute(owner, '/hev skill list', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    const allSkillNames = ['allowed-skill', 'hev-guide', 'off-skill', 'outside-skill']
    await expect(ctx.commands.execute(owner, '/hev skill list --global', [], signal))
      .resolves.toMatchObject({
        result: {
          kind: 'success',
          text: 'global:\n- allowed-skill\n- hev-guide\n- off-skill\n- outside-skill',
        },
      })
    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(allSkillNames)
    expect((await ctx.skills.list(otherView)).map(skill => skill.name)).toEqual(allSkillNames)
    expect(await ctx.skills.get('outside-skill', view)).toMatchObject({ name: 'outside-skill' })

    await expect(ctx.commands.execute(owner, '/hev env list', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'environments:\n- base (base rev 1)' } })
    const store = JSON.parse(await readFile(join(home, '.hev', 'environments.json'), 'utf8')) as {
      schemaVersion: number
      environments: Array<{ source: string; id: string; name: string; skills: Array<{ skillKey: string }> }>
    }
    expect(store.schemaVersion).toBe(1)
    expect(store.environments.map(environment => environment.name)).toContain('base')
    expect(store.environments.find(environment => environment.name === 'base')).toMatchObject({
      source: 'dsh',
      id: 'base',
      name: 'base',
      skills: [{ skillKey: 'hev-guide' }],
    })

    await expect(ctx.commands.execute(owner, '/hev env create coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success' } })
    await expect(ctx.commands.execute(
      owner,
      '/hev skill add allowed-skill coding --policy auto',
      [],
      signal,
    )).resolves.toMatchObject({ result: { kind: 'success' } })
    await expect(ctx.commands.execute(
      owner,
      '/hev skill add off-skill coding --policy off',
      [],
      signal,
    )).resolves.toMatchObject({ result: { kind: 'success' } })

    const useResult = await ctx.tools.execute({
      signal,
      callId: CallId('hev-use'),
      name: 'hev_env_use',
      arguments: { environment: 'coding' },
      agent: owner,
    })
    expect(useResult).toMatchObject({ isError: false, value: expect.stringMatching(/^coding /u) })
    const resumed = agent('hev-integration')
    await expect(ctx.environment.current(resumed.session, signal))
      .resolves.toMatchObject({ name: 'coding', revision: 3 })
    const bindings = JSON.parse(await readFile(join(home, '.hev', 'session-bindings.json'), 'utf8')) as {
      schemaVersion: number
      bindings: Array<{ sessionId: string; environmentId: string }>
    }
    expect(bindings).toMatchObject({
      schemaVersion: 1,
      bindings: [{ sessionId: 'hev-integration', environmentId: expect.stringMatching(/^env_/u) }],
    })
    expect(bindings.bindings[0]).not.toHaveProperty('source')
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringMatching(/^coding \(env_[^)]+ rev 3\)$/u) } })
    await expect(ctx.commands.execute(owner, '/hev skill list', [], signal))
      .resolves.toMatchObject({
        result: {
          kind: 'success',
          text: 'coding:\n- hev-guide (auto)\n- allowed-skill (auto)\n- off-skill (off)',
        },
      })
    await expect(ctx.commands.execute(other, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    await expect(ctx.commands.execute(other, '/hev skill list', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })

    await expect(ctx.commands.execute(other, '/hev skill list coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringContaining('coding:') } })

    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(['allowed-skill', 'hev-guide'])
    expect((await ctx.skills.list(otherView)).map(skill => skill.name)).toEqual(allSkillNames)
    expect(await ctx.skills.get('allowed-skill', view)).toMatchObject({ name: 'allowed-skill' })
    expect(await ctx.skills.get('off-skill', view)).toMatchObject({ name: 'off-skill' })
    expect(await ctx.skills.get('outside-skill', view)).toMatchObject({ name: 'outside-skill' })
    await expect(ctx.commands.execute(owner, '/hev skill list --global', [], signal))
      .resolves.toMatchObject({
        result: {
          kind: 'success',
          text: 'global:\n- allowed-skill\n- hev-guide\n- off-skill\n- outside-skill',
        },
      })

    await expect(ctx.commands.execute(owner, '/hev env rename coding review', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'environment renamed' } })
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringMatching(/^review /u) } })
    await expect(ctx.commands.execute(owner, '/hev skill remove off-skill review', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'skill removed from environment' } })
    await expect(ctx.commands.execute(owner, '/hev skill list review', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringContaining('allowed-skill (auto)') } })

    await expect(ctx.commands.execute(owner, '/hev env quit', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'base (base rev 1)' } })
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'base (base rev 1)' } })
    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(['hev-guide'])

    await expect(ctx.commands.execute(owner, '/hev env quit', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(allSkillNames)
    expect(await ctx.skills.get('outside-skill', view)).toMatchObject({ name: 'outside-skill' })

    await expect(ctx.commands.execute(owner, '/hev env create scratch', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success' } })
    await expect(ctx.commands.execute(owner, '/hev env use scratch', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringMatching(/^scratch /u) } })
    await expect(ctx.commands.execute(owner, '/hev env delete scratch', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'environment deleted' } })
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'base (base rev 1)' } })
  })
})
