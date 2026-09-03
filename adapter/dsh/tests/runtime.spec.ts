import { Context } from '@deepseek-ai/cordis'
import { join } from 'node:path'
import AgentRegistry, { Inbox } from '@deepseek-ai/dsh-agent'
import type { Agent } from '@deepseek-ai/dsh-agent'
import CommandRuntime from '@deepseek-ai/dsh-commands'
import { Session, SessionId } from '@deepseek-ai/dsh-session'
import { describe, expect, it, vi } from 'vitest'
import EnvironmentController, { EnvironmentId, HevCliClient, StatusCode } from '../src/hev-runtime/index.ts'
import type { Environment, NativeCommandRunner } from '../src/hev-runtime/index.ts'
import HevSkillRegistry from '../src/hev-skill-registry/index.ts'

const signal = new AbortController().signal
function sessionResponse(
  sessionId: string,
  environment: Environment | null,
  message = 'session status resolved',
): string {
  return JSON.stringify({
    schemaVersion: 2,
    code: 200,
    message,
    prompt: '',
    data: { session: { source: 'dsh', sessionId, environment } },
  })
}

function success(message: string, data: unknown): string {
  return JSON.stringify({ schemaVersion: 2, code: 200, message, prompt: '', data })
}

function environment(id: string, revision: number, skills = ['code-review'], name = 'coding'): Environment {
  return {
    source: 'dsh',
    id: EnvironmentId(id),
    name,
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

describe('@owariband/hev-dsh-plugin/hev-runtime', () => {
  it('uses the package-local platform binary by default', async () => {
    const runner = vi.fn<NativeCommandRunner>(async () => ({
      stdout: sessionResponse('bundled-binary', environment('base', 1, [], 'base'), 'environment selected'),
      stderr: '',
    }))
    const runtime = new EnvironmentController(new Context(), {}, { runner })
    const session = Session.create(SessionId('bundled-binary'))

    await runtime.use(agent(session), 'base', signal)

    const executable = process.platform === 'win32' ? 'hev.exe' : 'hev'
    expect(runner).toHaveBeenCalledWith(
      expect.stringContaining(join('bin', `${process.platform}-${process.arch}`, executable)),
      ['--source', 'dsh', 'env', 'use', 'base', '--session-id', 'bundled-binary', '--output', 'json'],
      signal,
    )
  })

  it('uses exact Session-aware argv and rereads current revisions', async () => {
    let revision = 2
    const runner = vi.fn<NativeCommandRunner>(async () => ({
      stdout: sessionResponse('one', environment('env_canonical', revision)),
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
      '--source', 'dsh', 'env', 'use', 'coding', '--session-id', 'one', '--output', 'json',
    ], signal)

    revision = 3
    expect((await runtime.current(session, signal))?.revision).toBe(3)
    expect(runner).toHaveBeenNthCalledWith(2, 'hev-test', [
      '--source', 'dsh', 'env', 'status', '--session-id', 'one', '--output', 'json',
    ], signal)
  })

  it('uses stable Session IDs and keeps different IDs isolated', async () => {
    const selectedIds = new Set<string>()
    const runner = vi.fn<NativeCommandRunner>(async (_command, args) => {
      const sessionIndex = args.indexOf('--session-id')
      const sessionId = args[sessionIndex + 1] as string
      if (args.includes('use')) selectedIds.add(sessionId)
      return {
        stdout: sessionResponse(
          sessionId,
          selectedIds.has(sessionId) ? environment('env_1', 1, ['code-review']) : null,
        ),
        stderr: '',
      }
    })
    const { runtime } = await world(runner)
    const selected = Session.create(SessionId('selected'))
    const other = Session.create(SessionId('other'))

    await expect(runtime.current(other, signal)).resolves.toBeUndefined()

    await runtime.use(agent(selected), 'coding', signal)

    expect((await runtime.current(selected, signal))?.id).toBe('env_1')
    await expect(runtime.current(other, signal)).resolves.toBeUndefined()
    expect(runner.mock.calls.map(call => call[1])).toEqual([
      ['--source', 'dsh', 'env', 'status', '--session-id', 'other', '--output', 'json'],
      ['--source', 'dsh', 'env', 'use', 'coding', '--session-id', 'selected', '--output', 'json'],
      ['--source', 'dsh', 'env', 'status', '--session-id', 'selected', '--output', 'json'],
      ['--source', 'dsh', 'env', 'status', '--session-id', 'other', '--output', 'json'],
    ])
  })

  it('surfaces structured and malformed CLI failures without adapter state', async () => {
    const outputs: Array<{ stdout: string; reject?: boolean }> = [
      { stdout: sessionResponse('cli-rollback', environment('env_good', 1), 'environment selected') },
      {
        stdout: JSON.stringify({
          schemaVersion: 2, code: 404, message: 'missing', prompt: 'create it', data: {},
        }),
        reject: true,
      },
      { stdout: '{bad json' },
      { stdout: sessionResponse('cli-rollback', environment('env_good', 5)) },
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
    expect((await runtime.current(owner.session, signal))?.revision).toBe(5)
  })

  it('returns Core prompts from slash-command failures and logs diagnostics', async () => {
    const runner = vi.fn<NativeCommandRunner>(async () => {
      const stdout = JSON.stringify({
        schemaVersion: 2,
        code: 404,
        message: 'environment not found: missing',
        prompt: 'List Environments and retry with an existing name.',
        data: {},
      })
      throw Object.assign(new Error('exit 1'), { stdout, stderr: '' })
    })
    const ctx = new Context()
    await ctx.plugin(AgentRegistry)
    await ctx.plugin(CommandRuntime)
    const warnings: string[] = []
    ctx.logger.warn = ((message: unknown) => { warnings.push(String(message)) }) as typeof ctx.logger.warn
    class TestEnvironmentController extends EnvironmentController {
      constructor(pluginContext: Context) {
        super(pluginContext, { executable: 'hev-test' }, { runner })
      }
    }
    await ctx.plugin(TestEnvironmentController)
    const owner = agent(Session.create(SessionId('prompt-command')))
    ctx.agents.register(owner)

    await expect(ctx.commands.execute(owner, '/hev env use missing', [], signal)).resolves.toMatchObject({
      result: { kind: 'error', text: 'List Environments and retry with an existing name.' },
    })
    expect(warnings).toEqual(['hev command failed: 404: environment not found: missing'])
  })

  it('forwards commands and applies the two-level quit transition', async () => {
    let current: Environment | null = null
    const runner = vi.fn<NativeCommandRunner>(async (_command, args) => {
      const command = args.slice(2, -2)
      if (command[0] === 'env' && command[1] === 'create') {
        return {
          stdout: success('environment created', { environment: environment('env_created', 1, []) }),
          stderr: '',
        }
      }
      if (command[0] === 'env' && command[1] === 'rename') {
        return {
          stdout: success('environment renamed', {
            environment: environment('env_created', 3, [], 'backend'),
          }),
          stderr: '',
        }
      }
      if (command[0] === 'env' && command[1] === 'delete') {
        return {
          stdout: success('environment deleted', {
            environment: environment('env_scratch', 1, [], 'scratch'),
          }),
          stderr: '',
        }
      }
      if (command[0] === 'skill' && command[1] === 'add') {
        return {
          stdout: success('skill added to environment', {
            environmentSkill: { skillKey: 'code-review', policy: { kind: 'off' } },
            environments: [
              { source: 'dsh', id: 'env_created', name: 'coding', revision: 2 },
              { source: 'dsh', id: 'env_writing', name: 'writing', revision: 4 },
            ],
          }),
          stderr: '',
        }
      }
      if (command[0] === 'skill' && command[1] === 'list' && command.length === 3) {
        return {
          stdout: success('environment skills listed', { environment: environment('env_created', 2) }),
          stderr: '',
        }
      }
      if (command[0] === 'skill' && command[1] === 'remove') {
        return {
          stdout: success('skill removed from environment', {
            skillKey: 'code-review',
            environments: [
              { source: 'dsh', id: 'env_created', name: 'coding', revision: 3 },
              { source: 'dsh', id: 'env_writing', name: 'writing', revision: 5 },
            ],
          }),
          stderr: '',
        }
      }
      if (command[0] === 'env' && command[1] === 'list') {
        return {
          stdout: success('environments listed', {
            environments: [
              { source: 'dsh', id: 'base', name: 'base', revision: 1 },
              { source: 'dsh', id: 'env_created', name: 'coding', revision: 2 },
            ],
          }),
          stderr: '',
        }
      }
      if (command[0] === 'env' && command[1] === 'quit') {
        const id = command[3] as string
        current = current !== null && current.id !== 'base'
          ? environment('base', 1, [], 'base')
          : null
        return { stdout: sessionResponse(id, current, 'session environment changed'), stderr: '' }
      }
      if (command[0] === 'env' && command[1] === 'use') {
        const id = command[4] as string
        current = environment('env_created', 2)
        return { stdout: sessionResponse(id, current, 'environment selected'), stderr: '' }
      }
      const id = command[3] as string
      return { stdout: sessionResponse(id, current), stderr: '' }
    })
    const ctx = new Context()
    await ctx.plugin(AgentRegistry)
    await ctx.plugin(CommandRuntime)
    class TestEnvironmentController extends EnvironmentController {
      constructor(pluginContext: Context) {
        super(pluginContext, { executable: 'hev-test' }, { runner })
      }
    }
    const fiber = await ctx.plugin(TestEnvironmentController)
    await ctx.plugin(HevSkillRegistry)
    for (const name of ['code-review', 'outside-skill']) {
      ctx.skills.register({
        name,
        description: `${name} description`,
        source: 'runtime',
        content: `${name} body`,
      })
    }
    const owner = agent(Session.create(SessionId('commands')))
    ctx.agents.register(owner)

    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    await expect(ctx.commands.execute(owner, '/hev help env use', [], signal))
      .resolves.toMatchObject({
        result: { kind: 'success', text: 'usage: /hev env use <environment-id-or-name>' },
      })
    await expect(ctx.commands.execute(owner, '/hev help env rename', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringContaining('env rename') } })
    await expect(ctx.commands.execute(owner, '/hev help skill remove', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringContaining('skill remove') } })
    await expect(ctx.commands.execute(owner, '/hev skill --help', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringContaining('/hev skill add') } })
    await expect(ctx.commands.execute(owner, '/hev skill list', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    await expect(ctx.commands.execute(owner, '/hev skill list --global', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'global:\n- code-review\n- hev-guide\n- outside-skill' } })
    await expect(ctx.commands.execute(owner, '/hev skill list coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: expect.stringContaining('coding:') } })
    await expect(ctx.commands.execute(owner, '/hev skill list coding extra', [], signal))
      .resolves.toMatchObject({ result: { kind: 'error', text: expect.stringContaining('skill list') } })
    await expect(ctx.commands.execute(owner, '/hev env list', [], signal))
      .resolves.toMatchObject({
        result: {
          kind: 'success',
          text: 'environments:\n- base (base rev 1)\n- coding (env_created rev 2)',
        },
      })
    await expect(ctx.commands.execute(owner, '/hev env create coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'environment created' } })
    await expect(ctx.commands.execute(owner, '/hev env rename coding backend', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'environment renamed' } })
    await expect(ctx.commands.execute(owner, '/hev env delete scratch', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'environment deleted' } })
    await expect(ctx.commands.execute(owner, '/hev skill add code-review --env coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'error', text: expect.stringContaining('skill add') } })
    await expect(ctx.commands.execute(
      owner,
      '/hev skill add code-review coding writing --policy off',
      [],
      signal,
    )).resolves.toMatchObject({ result: { kind: 'success', text: 'skill added to environment' } })
    await expect(ctx.commands.execute(
      owner,
      '/hev skill remove code-review coding writing',
      [],
      signal,
    )).resolves.toMatchObject({ result: { kind: 'success', text: 'skill removed from environment' } })
    await expect(ctx.commands.execute(owner, '/hev env use coding', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'coding (env_created rev 2)' } })
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'coding (env_created rev 2)' } })
    await expect(ctx.commands.execute(owner, '/hev env quit', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'base (base rev 1)' } })
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'base (base rev 1)' } })
    await expect(ctx.commands.execute(owner, '/hev skill list', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'base: no skills configured' } })
    await expect(ctx.commands.execute(owner, '/hev env quit', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    await expect(ctx.commands.execute(owner, '/hev env status', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    await expect(ctx.commands.execute(owner, '/hev env quit', [], signal))
      .resolves.toMatchObject({ result: { kind: 'success', text: 'hev not activated' } })
    await expect(ctx.commands.execute(owner, '/hev env status extra', [], signal))
      .resolves.toMatchObject({ result: { kind: 'error', text: expect.stringContaining('env status') } })
    await expect(ctx.commands.execute(owner, '/hev env use coding writing', [], signal))
      .resolves.toMatchObject({ result: { kind: 'error', text: expect.stringContaining('env use <environment-id-or-name>') } })

    expect(runner.mock.calls.map(call => call[1])).toEqual([
      ['--source', 'dsh', 'env', 'status', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'env', 'status', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'skill', 'list', 'coding', '--output', 'json'],
      ['--source', 'dsh', 'env', 'list', '--output', 'json'],
      ['--source', 'dsh', 'env', 'create', 'coding', '--output', 'json'],
      ['--source', 'dsh', 'env', 'rename', 'coding', 'backend', '--output', 'json'],
      ['--source', 'dsh', 'env', 'delete', 'scratch', '--output', 'json'],
      ['--source', 'dsh', 'skill', 'add', 'code-review', 'coding', 'writing', '--policy', 'off', '--output', 'json'],
      ['--source', 'dsh', 'skill', 'remove', 'code-review', 'coding', 'writing', '--output', 'json'],
      ['--source', 'dsh', 'env', 'use', 'coding', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'env', 'status', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'env', 'quit', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'env', 'status', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'env', 'status', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'env', 'quit', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'env', 'status', '--session-id', 'commands', '--output', 'json'],
      ['--source', 'dsh', 'env', 'quit', '--session-id', 'commands', '--output', 'json'],
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
      await expect(client.use(environmentRef, 'session', signal)).rejects.toMatchObject({ statusCode: StatusCode.InvalidArgument })
    }
    await expect(client.create('Bad Name', signal)).rejects.toMatchObject({ statusCode: StatusCode.InvalidArgument })
    expect(runner).not.toHaveBeenCalled()

    await expect(client.use('coding', 'session', signal)).rejects.toMatchObject({ statusCode: StatusCode.Unavailable })
  })

  it('strictly rejects malformed CLI v2 environments with protocol status 502', async () => {
    const invalid: unknown[] = [
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'codex', sessionId: 'invalid', environment: { ...environment('env_1', 1), source: 'codex' } } } },
      { schemaVersion: 1, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'invalid', environment: environment('env_1', 1) } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: {} },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: 'unexpected', data: { session: { source: 'dsh', sessionId: 'invalid', environment: environment('env_1', 1) } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: '', environment: environment('env_1', 1) } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'other', environment: environment('env_1', 1) } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'invalid', environment: { ...environment(' ', 1) } } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'invalid', environment: { ...environment('env_1', 1), name: 'Bad Name' } } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'invalid', environment: { ...environment('env_1', 1), revision: 0 } } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'invalid', environment: { ...environment('env_1', 1), revision: 1.5 } } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'invalid', environment: { ...environment('env_1', 1), skills: [{ skillKey: 'Bad Skill', policy: { kind: 'auto' } }] } } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'invalid', environment: { ...environment('env_1', 1), skills: [{ skillKey: 'code-review', policy: { kind: 'always' } }] } } } },
      { schemaVersion: 2, code: 200, message: 'ok', prompt: '', data: { session: { source: 'dsh', sessionId: 'invalid', environment: { ...environment('env_1', 1), skills: [{ skillKey: 'code-review', policy: { kind: 'auto' } }, { skillKey: 'code-review', policy: { kind: 'off' } }] } } } },
    ]
    for (const envelope of invalid) {
      const runner: NativeCommandRunner = async () => ({ stdout: JSON.stringify(envelope), stderr: '' })
      const { runtime } = await world(runner)
      await expect(runtime.use(agent(Session.create(SessionId(crypto.randomUUID()))), 'coding', signal))
        .rejects.toMatchObject({ statusCode: StatusCode.ProtocolError })
    }
  })

  it('strictly rejects malformed Environment lists with protocol status 502', async () => {
    const invalid: unknown[] = [
      [],
      [{ source: 'dsh', id: 'base', name: 'base', revision: 0 }],
      [
        { source: 'dsh', id: 'base', name: 'base', revision: 1 },
        { source: 'dsh', id: 'base', name: 'coding', revision: 1 },
      ],
    ]
    for (const environments of invalid) {
      const runner: NativeCommandRunner = async () => ({
        stdout: success('environments listed', { environments }),
        stderr: '',
      })
      const client = new HevCliClient('hev-test', runner)
      await expect(client.listEnvironments(signal))
        .rejects.toMatchObject({ statusCode: StatusCode.ProtocolError })
    }
  })
})
