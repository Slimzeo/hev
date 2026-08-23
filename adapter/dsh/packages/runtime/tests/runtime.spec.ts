import { Context } from '@deepseek-ai/cordis'
import { Inbox } from '@deepseek-ai/dsh-agent'
import type { Agent } from '@deepseek-ai/dsh-agent'
import CommandRuntime from '@deepseek-ai/dsh-commands'
import { Session, SessionId } from '@deepseek-ai/dsh-session'
import { describe, expect, it, vi } from 'vitest'
import EnvironmentController, { HevCliError } from '../src/index.ts'
import type { NativeCommandRunner, ResolvedEnvironmentSnapshot } from '../src/index.ts'

const signal = new AbortController().signal

function response(environments: ResolvedEnvironmentSnapshot['environments']): string {
  return JSON.stringify({
    schemaVersion: 1,
    code: 200,
    message: 'environment snapshot resolved',
    prompt: '',
    data: { snapshot: { environments } },
  })
}

function success(message: string, data: unknown): string {
  return JSON.stringify({ schemaVersion: 1, code: 200, message, prompt: '', data })
}

function environment(id: string, revision: number, skills = ['code-review']) {
  return {
    id,
    name: 'coding',
    revision,
    skills: skills.map(skillKey => ({ skillKey, policy: { kind: 'auto' as const } })),
  }
}

function agent(session: Session): Agent {
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

async function world(runner: NativeCommandRunner) {
  const ctx = new Context()
  const runtime = new EnvironmentController(ctx, { executable: 'hev-test' }, { runner })
  return { ctx, runtime }
}

describe('@hev/dsh-runtime', () => {
  it('uses exact activation argv, stores canonical IDs, and rereads current revisions', async () => {
    let revision = 2
    const runner = vi.fn<NativeCommandRunner>(async () => ({
      stdout: response([environment('env_canonical', revision)]),
      stderr: '',
    }))
    const { runtime } = await world(runner)
    const session = Session.create(SessionId('one'), [], {
      version: 0, id: SessionId('one'), createdAt: 0, cwd: '/workspace/one',
    })
    const owner = agent(session)
    const activated = await runtime.activate(owner, ['coding'], signal)
    expect(activated.environments[0]?.revision).toBe(2)
    expect(runner).toHaveBeenNthCalledWith(1, 'hev-test', [
      'env', 'activate', 'coding', '--output', 'json',
    ], signal)

    revision = 3
    expect((await runtime.current(session, signal))?.environments[0]?.revision).toBe(3)
    expect(runner).toHaveBeenNthCalledWith(2, 'hev-test', [
      'env', 'activate', 'env_canonical', '--output', 'json',
    ], signal)
  })

  it('isolates exact Session objects', async () => {
    const runner = vi.fn<NativeCommandRunner>(async () => ({
      stdout: response([environment('env_1', 1, ['code-review'])]),
      stderr: '',
    }))
    const { runtime } = await world(runner)
    const selected = Session.create(SessionId('same'))
    const impostor = Session.create(SessionId('same'))

    await runtime.activate(agent(selected), ['coding'], signal)

    expect(await runtime.current(impostor, signal)).toBeUndefined()
    expect(runner).toHaveBeenCalledTimes(1)
  })

  it('preserves the previous IDs across structured and malformed CLI failures', async () => {
    const outputs: Array<{ stdout: string; reject?: boolean }> = [
      { stdout: response([environment('env_good', 1)]) },
      {
        stdout: JSON.stringify({
          schemaVersion: 1, code: 404, message: 'missing', prompt: 'create it',
          data: { errorCode: 'ENV_NOT_FOUND' },
        }),
        reject: true,
      },
      { stdout: '{bad json' },
      { stdout: response([environment('env_good', 5)]) },
    ]
    const runner = vi.fn<NativeCommandRunner>(async () => {
      const output = outputs.shift()
      if (output === undefined) throw new Error('missing fixture')
      if (output.reject === true) throw Object.assign(new Error('exit 1'), { stdout: output.stdout, stderr: '' })
      return { stdout: output.stdout, stderr: '' }
    })
    const { runtime } = await world(runner)
    const owner = agent(Session.create(SessionId('cli-rollback')))

    await runtime.activate(owner, ['good'], signal)
    await expect(runtime.activate(owner, ['missing'], signal)).rejects.toMatchObject({
      code: 'ENV_NOT_FOUND', prompt: 'create it',
    })
    await expect(runtime.activate(owner, ['malformed'], signal)).rejects.toBeInstanceOf(HevCliError)
    expect((await runtime.current(owner.session, signal))?.environments[0]?.revision).toBe(5)
  })

  it('forwards the three supported /hev operations with fixed JSON argv', async () => {
    const runner = vi.fn<NativeCommandRunner>(async (_command, args) => {
      if (args[0] === 'env' && args[1] === 'create') {
        return {
          stdout: success('environment created', { environment: environment('env_created', 1, []) }),
          stderr: '',
        }
      }
      if (args[0] === 'skill' && args[1] === 'add') {
        return {
          stdout: success('skill added to environment', {
            environmentSkill: { skillKey: 'code-review', policy: { kind: 'off' } },
            environments: [{ id: 'env_created', name: 'coding', revision: 2 }],
          }),
          stderr: '',
        }
      }
      return { stdout: response([environment('env_created', 2)]), stderr: '' }
    })
    const ctx = new Context()
    await ctx.plugin(CommandRuntime)
    class TestEnvironmentController extends EnvironmentController {
      constructor(pluginContext: Context) {
        super(pluginContext, { executable: 'hev-test' }, { runner })
      }
    }
    const fiber = await ctx.plugin(TestEnvironmentController)
    const owner = agent(Session.create(SessionId('commands')))

    await expect(ctx.commands.execute(owner, '/hev env create coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'environment created' } })
    await expect(ctx.commands.execute(
      owner,
      '/hev skill add code-review --env coding --policy off',
      [],
      signal,
    )).resolves.toMatchObject({ result: { kind: 'success', text: 'skill added to environment' } })
    await expect(ctx.commands.execute(owner, '/hev env activate coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success' } })

    expect(runner.mock.calls.map(call => call[1])).toEqual([
      ['env', 'create', 'coding', '--output', 'json'],
      ['skill', 'add', 'code-review', '--env', 'coding', '--policy', 'off', '--output', 'json'],
      ['env', 'activate', 'coding', '--output', 'json'],
    ])

    await fiber.dispose()
    expect(await ctx.commands.execute(owner, '/hev env create removed', [], signal)).toBeUndefined()
  })

  it('strictly rejects malformed CLI v1 snapshots', async () => {
    const invalid: unknown[] = [
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [environment('env_1', 1)] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: {} },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: 'unexpected', data: { snapshot: { environments: [environment('env_1', 1)] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [{ ...environment(' ', 1) }] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [{ ...environment('env_1', 1), name: 'Bad Name' }] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [{ ...environment('env_1', 1), revision: 0 }] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [{ ...environment('env_1', 1), revision: 1.5 }] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [{ ...environment('env_1', 1), skills: [{ skillKey: 'Bad Skill', policy: { kind: 'auto' } }] }] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [{ ...environment('env_1', 1), skills: [{ skillKey: 'code-review', policy: { kind: 'always' } }] }] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [environment('env_1', 1), { ...environment('env_1', 2), name: 'writing' }] } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { snapshot: { environments: [environment('env_1', 1), { ...environment('env_2', 1), name: 'writing' }] } } },
    ]
    for (const envelope of invalid) {
      const runner: NativeCommandRunner = async () => ({ stdout: JSON.stringify(envelope), stderr: '' })
      const { runtime } = await world(runner)
      await expect(runtime.activate(agent(Session.create(SessionId(crypto.randomUUID()))), ['coding'], signal))
        .rejects.toMatchObject({ code: 'CLI_PROTOCOL' })
    }
  })
})
