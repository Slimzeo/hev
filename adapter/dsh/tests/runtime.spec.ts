import { Context } from '@deepseek-ai/cordis'
import { join } from 'node:path'
import { Inbox } from '@deepseek-ai/dsh-agent'
import type { Agent } from '@deepseek-ai/dsh-agent'
import CommandRuntime from '@deepseek-ai/dsh-commands'
import { Session, SessionId } from '@deepseek-ai/dsh-session'
import { describe, expect, it, vi } from 'vitest'
import EnvironmentController, { EnvironmentId, HevCliClient, StatusCode } from '../src/hev-runtime/index.ts'
import type { Environment, NativeCommandRunner } from '../src/hev-runtime/index.ts'

const signal = new AbortController().signal

function response(environment: Environment): string {
  return JSON.stringify({
    schemaVersion: 2,
    code: 200,
    message: 'environment resolved',
    prompt: '',
    data: { environment },
  })
}

function success(message: string, data: unknown): string {
  return JSON.stringify({ schemaVersion: 2, code: 200, message, prompt: '', data })
}

function environment(id: string, revision: number, skills = ['code-review']): Environment {
  return {
    id: EnvironmentId(id),
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

describe('@slimzeo/hev-dsh-plugin/hev-runtime', () => {
  it('uses the package-local platform binary by default', async () => {
    const runner = vi.fn<NativeCommandRunner>(async () => ({
      stdout: response(environment('env_base', 1, [])),
      stderr: '',
    }))
    const runtime = new EnvironmentController(new Context(), {}, { runner })
    const session = Session.create(SessionId('bundled-binary'))

    await runtime.current(session, signal)

    const executable = process.platform === 'win32' ? 'hev.exe' : 'hev'
    expect(runner).toHaveBeenCalledWith(
      expect.stringContaining(join('bin', `${process.platform}-${process.arch}`, executable)),
      ['env', 'use', '--output', 'json'],
      signal,
    )
  })

  it('uses exact use argv, stores canonical IDs, and rereads current revisions', async () => {
    let revision = 2
    const runner = vi.fn<NativeCommandRunner>(async () => ({
      stdout: response(environment('env_canonical', revision)),
      stderr: '',
    }))
    const { runtime } = await world(runner)
    const session = Session.create(SessionId('one'), [], {
      version: 0, id: SessionId('one'), createdAt: 0, cwd: '/workspace/one',
    })
    const owner = agent(session)
    const selected = await runtime.use(owner, 'coding', signal)
    expect(selected.revision).toBe(2)
    expect(runner).toHaveBeenNthCalledWith(1, 'hev-test', [
      'env', 'use', 'coding', '--output', 'json',
    ], signal)

    revision = 3
    expect((await runtime.current(session, signal)).revision).toBe(3)
    expect(runner).toHaveBeenNthCalledWith(2, 'hev-test', [
      'env', 'use', 'env_canonical', '--output', 'json',
    ], signal)
  })

  it('defaults to base for an unselected Session and then isolates exact Session overrides', async () => {
    const runner = vi.fn<NativeCommandRunner>(async () => ({
      stdout: response(environment('env_1', 1, ['code-review'])),
      stderr: '',
    }))
    const { runtime } = await world(runner)
    const selected = Session.create(SessionId('same'))
    const impostor = Session.create(SessionId('same'))

    expect((await runtime.current(impostor, signal)).id).toBe('env_1')
    expect(runner).toHaveBeenNthCalledWith(1, 'hev-test', [
      'env', 'use', '--output', 'json',
    ], signal)

    await runtime.use(agent(selected), 'coding', signal)

    expect((await runtime.current(selected, signal)).id).toBe('env_1')
    expect((await runtime.current(impostor, signal)).id).toBe('env_1')
    expect(runner.mock.calls.map(call => call[1])).toEqual([
      ['env', 'use', '--output', 'json'],
      ['env', 'use', 'coding', '--output', 'json'],
      ['env', 'use', 'env_1', '--output', 'json'],
      ['env', 'use', 'env_1', '--output', 'json'],
    ])
  })

  it('preserves the previous IDs across structured and malformed CLI failures', async () => {
    const outputs: Array<{ stdout: string; reject?: boolean }> = [
      { stdout: response(environment('env_good', 1)) },
      {
        stdout: JSON.stringify({
          schemaVersion: 2, code: 404, message: 'missing', prompt: 'create it', data: {},
        }),
        reject: true,
      },
      { stdout: '{bad json' },
      { stdout: response(environment('env_good', 5)) },
    ]
    const runner = vi.fn<NativeCommandRunner>(async () => {
      const output = outputs.shift()
      if (output === undefined) throw new Error('missing fixture')
      if (output.reject === true) throw Object.assign(new Error('exit 1'), { stdout: output.stdout, stderr: '' })
      return { stdout: output.stdout, stderr: '' }
    })
    const { runtime } = await world(runner)
    const owner = agent(Session.create(SessionId('cli-rollback')))

    await runtime.use(owner, 'good', signal)
    await expect(runtime.use(owner, 'missing', signal)).rejects.toMatchObject({
      statusCode: StatusCode.NotFound, prompt: 'create it',
    })
    await expect(runtime.use(owner, 'malformed', signal)).rejects.toMatchObject({ statusCode: StatusCode.ProtocolError })
    expect((await runtime.current(owner.session, signal)).revision).toBe(5)
  })

  it('forwards the supported /hev operations and reports the current Session Environment', async () => {
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
      return { stdout: response(environment('env_created', 2)), stderr: '' }
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

    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'coding (env_created rev 2)' } })
    await expect(ctx.commands.execute(owner, '/hev skill list', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'coding:\n- code-review (auto)' } })
    await expect(ctx.commands.execute(owner, '/hev skill list extra', [], signal))
      .resolves.toMatchObject({ result: { kind: 'error', text: expect.stringContaining('skill list') } })
    await expect(ctx.commands.execute(owner, '/hev env create coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'environment created' } })
    await expect(ctx.commands.execute(
      owner,
      '/hev skill add code-review --env coding --policy off',
      [],
      signal,
    )).resolves.toMatchObject({ result: { kind: 'success', text: 'skill added to environment' } })
    await expect(ctx.commands.execute(owner, '/hev env use coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'coding (env_created rev 2)' } })
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'coding (env_created rev 2)' } })
    await expect(ctx.commands.execute(owner, '/hev env status extra', [], signal))
      .resolves.toMatchObject({ result: { kind: 'error', text: expect.stringContaining('env status') } })
    await expect(ctx.commands.execute(owner, '/hev env use coding writing', [], signal))
      .resolves.toMatchObject({ result: { kind: 'error', text: expect.stringContaining('env use <id-or-name>') } })

    expect(runner.mock.calls.map(call => call[1])).toEqual([
      ['env', 'use', '--output', 'json'],
      ['env', 'use', 'env_created', '--output', 'json'],
      ['env', 'create', 'coding', '--output', 'json'],
      ['skill', 'add', 'code-review', '--env', 'coding', '--policy', 'off', '--output', 'json'],
      ['env', 'use', 'coding', '--output', 'json'],
      ['env', 'use', 'env_created', '--output', 'json'],
    ])

    await fiber.dispose()
    expect(await ctx.commands.execute(owner, '/hev env create removed', [], signal)).toBeUndefined()
  })

  it('uses numeric status codes for local validation and unavailable CLI failures', async () => {
    const runner = vi.fn<NativeCommandRunner>(async () => {
      throw new Error('spawn failed')
    })
    const client = new HevCliClient('hev-test', runner)

    for (const environmentRef of ['', ' coding', 'bad name', '-flag']) {
      await expect(client.use(environmentRef, signal)).rejects.toMatchObject({ statusCode: StatusCode.InvalidArgument })
    }
    await expect(client.create('Bad Name', signal)).rejects.toMatchObject({ statusCode: StatusCode.InvalidArgument })
    expect(runner).not.toHaveBeenCalled()

    await expect(client.use('coding', signal)).rejects.toMatchObject({ statusCode: StatusCode.Unavailable })
  })

  it('strictly rejects malformed CLI v2 environments with protocol status 502', async () => {
    const invalid: unknown[] = [
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { environment: environment('env_1', 1) } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: {} },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: 'unexpected', data: { environment: environment('env_1', 1) } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { environment: { ...environment(' ', 1) } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { environment: { ...environment('env_1', 1), name: 'Bad Name' } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { environment: { ...environment('env_1', 1), revision: 0 } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { environment: { ...environment('env_1', 1), revision: 1.5 } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { environment: { ...environment('env_1', 1), skills: [{ skillKey: 'Bad Skill', policy: { kind: 'auto' } }] } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { environment: { ...environment('env_1', 1), skills: [{ skillKey: 'code-review', policy: { kind: 'always' } }] } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { environment: { ...environment('env_1', 1), skills: [{ skillKey: 'code-review', policy: { kind: 'auto' } }, { skillKey: 'code-review', policy: { kind: 'off' } }] } } },
    ]
    for (const envelope of invalid) {
      const runner: NativeCommandRunner = async () => ({ stdout: JSON.stringify(envelope), stderr: '' })
      const { runtime } = await world(runner)
      await expect(runtime.use(agent(Session.create(SessionId(crypto.randomUUID()))), 'coding', signal))
        .rejects.toMatchObject({ statusCode: StatusCode.ProtocolError })
    }
  })
})
