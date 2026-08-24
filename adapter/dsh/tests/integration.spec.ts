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
import { Session, SessionId } from '@deepseek-ai/dsh-session'
import SkillRegistry from '@deepseek-ai/dsh-skill'
import EnvironmentController from '@hev/dsh-runtime'
import HevSkillRegistry from '@hev/dsh-skill'

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

describe('HEV DSH bundle', () => {
  it('runs the real Go CLI, persists base by default, and keeps exact Session filtering isolated', { timeout: 120_000 }, async () => {
    temporaryRoot = await mkdtemp(join(tmpdir(), 'hev-dsh-'))
    const home = join(temporaryRoot, 'home')
    const executable = join(temporaryRoot, 'hev')
    await execFileAsync('go', ['build', '-o', executable, './cmd/hev'], {
      cwd: hevRoot,
      env: { ...process.env, GOCACHE: '/private/tmp/hev-go-cache' },
    })
    vi.stubEnv('HOME', home)

    const warnings: string[] = []
    const entries = composeEntries([
      loadOverlayPatches('hev-test', join(dshRoot, 'packages/bundle/base/cordis.patch.yml')),
      loadOverlayPatches('hev-test', join(adapterRoot, 'cordis.patch.yml')),
      [{ id: 'hev-runtime', config: { executable } }],
    ], warning => void warnings.push(warning))
    expect(warnings).toEqual([])

    const selectedIds = new Set(['agent', 'commands', 'skill', 'hev-runtime', 'hev-skill'])
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
      ['@deepseek-ai/dsh-skill', SkillRegistry],
      ['@hev/dsh-runtime', EnvironmentController],
      ['@hev/dsh-skill', HevSkillRegistry],
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
    expect(rows.get('hev-skill')?.fiber).toBeDefined()

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
    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual([])
    expect((await ctx.skills.list(otherView)).map(skill => skill.name)).toEqual([])
    const store = JSON.parse(await readFile(join(home, '.hev', 'environments.json'), 'utf8')) as {
      schemaVersion: number
      environments: Array<{ id: string; name: string }>
    }
    expect(store.schemaVersion).toBe(1)
    expect(store.environments.map(environment => environment.name)).toContain('base')
    expect(store.environments.find(environment => environment.name === 'base')).toMatchObject({
      id: 'env_base',
      name: 'base',
    })

    await expect(ctx.commands.execute(owner, '/hev env create coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success' } })
    await expect(ctx.commands.execute(
      owner,
      '/hev skill add allowed-skill --env coding --policy auto',
      [],
      signal,
    )).resolves.toMatchObject({ result: { kind: 'success' } })
    await expect(ctx.commands.execute(
      owner,
      '/hev skill add off-skill --env coding --policy off',
      [],
      signal,
    )).resolves.toMatchObject({ result: { kind: 'success' } })

    await expect(ctx.commands.execute(owner, '/hev env use coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success' } })

    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(['allowed-skill'])
    expect((await ctx.skills.list(otherView)).map(skill => skill.name)).toEqual([])
    expect(await ctx.skills.get('allowed-skill', view)).toMatchObject({ name: 'allowed-skill' })
    expect(await ctx.skills.get('off-skill', view)).toBeUndefined()
    expect(await ctx.skills.get('outside-skill', view)).toBeUndefined()
  })
})
