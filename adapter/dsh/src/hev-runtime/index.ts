/** Live, session-scoped hev Environment selection for DeepSeek Harness.
 * @module @slimzeo/hev-dsh-plugin/hev-runtime
 */

import { Context, Service } from '@deepseek-ai/cordis'
import type { Agent } from '@deepseek-ai/dsh-agent'
import type { Session } from '@deepseek-ai/dsh-session'
import z from '@deepseek-ai/schemastery'
import { HevCliClient, HevCliError } from './cli.ts'
import type { NativeCommandRunner } from './cli.ts'
import { bundledExecutable } from './executable.ts'
import type { Environment, EnvironmentId } from './environment.ts'

// Type-only edge: resolves ctx.commands for the optional command child.
import type {} from '@deepseek-ai/dsh-commands'
export { EnvironmentId } from './environment.ts'
export { StatusCode } from './environment.ts'
export type {
  Environment,
  EnvironmentSkillPolicy,
  EnvironmentSkillSpec,
  FailureStatusCode,
  SkillPolicyKind,
} from './environment.ts'
export { HevCliClient, HevCliError } from './cli.ts'
export type { NativeCommandRunner } from './cli.ts'

declare module '@deepseek-ai/cordis' {
  interface Context {
    environment: EnvironmentController
  }
}

/** Runtime plugin configuration. */
export interface Config {
  /** hev executable path or PATH name; defaults to the binary bundled with this package. */
  readonly executable?: string
}

interface RuntimeDependencies {
  readonly runner?: NativeCommandRunner
}

const USAGE = 'usage: /hev env create <name> | env use <id-or-name> | env status | skill add <skill-key> --env <name> [--env <name>...] [--policy auto|off] | skill list'

/** Owns live Environment selection and the optional `/hev` command. */
export class EnvironmentController extends Service {
  static Config: z<Config> = z.object({
    executable: z.string(),
  })

  private readonly environmentIdBySession = new WeakMap<Session, EnvironmentId>()
  private readonly cli: HevCliClient

  /**
   * Create the runtime service.
   * @param ctx - Cordis context receiving the Environment service.
   * @param config - executable configuration.
   * @param dependencies - test-only native runner injection.
   */
  constructor(ctx: Context, config: Config = {}, dependencies: RuntimeDependencies = {}) {
    super(ctx, 'environment')
    const executable = config.executable ?? bundledExecutable()
    if (executable.trim() === '') throw new Error('hev executable must not be empty')
    this.cli = new HevCliClient(executable, dependencies.runner)

    ctx.inject(['commands'], (commandCtx) => {
      commandCtx.commands.register({
        name: 'hev',
        description: 'Manage skill environments',
        input: { hint: 'env create | env use | env status | skill add | skill list' },
        handler: async ({ agent, rawInput, signal }) => {
          try {
            const words = rawInput.trim().split(/\s+/u).filter(word => word.length > 0)
            return await this.executeCommand(agent, words, signal)
          } catch (error: unknown) {
            return { kind: 'error', text: renderFailure(error) }
          }
        },
      })
    })
  }

  /**
   * Resolve the latest Environment for one exact live Session.
   * @param session - exact in-process Session identity.
   * @param signal - optional operation cancellation signal.
   * @returns the latest record for the Session's selection, or the default `base` Environment.
   */
  async current(
    session: Session,
    signal?: AbortSignal,
  ): Promise<Environment> {
    const operationSignal = signal ?? new AbortController().signal
    const environmentId = this.environmentIdBySession.get(session)
    const environment = environmentId === undefined
      ? await this.cli.defaultEnvironment(operationSignal)
      : await this.cli.use(environmentId, operationSignal)
    this.environmentIdBySession.set(session, environment.id)
    return environment
  }
  /**
   * Resolve and atomically select one Environment for a live agent.
   * @param agent - exact agent whose Session receives the selection.
   * @param name - Environment ID or name.
   * @param signal - optional operation cancellation signal.
   * @returns the committed latest Environment.
   */
  async use(
    agent: Agent,
    name: string,
    signal?: AbortSignal,
  ): Promise<Environment> {
    const operationSignal = signal ?? new AbortController().signal
    const environment = await this.cli.use(name, operationSignal)
    this.environmentIdBySession.set(agent.session, environment.id)
    return environment
  }

  private async executeCommand(
    agent: Agent,
    words: readonly string[],
    signal: AbortSignal,
  ): Promise<{ kind: 'success'; text?: string } | { kind: 'error'; text: string }> {
    if (words[0] === 'skill' && words[1] === 'list' && words.length === 2) {
      const environment = await this.current(agent.session, signal)
      return {
        kind: 'success',
        text: environment.skills.length === 0
          ? `${environment.name}: no skills configured`
          : [
              `${environment.name}:`,
              ...environment.skills.map(skill => `- ${skill.skillKey} (${skill.policy.kind})`),
            ].join('\n'),
      }
    }
    if (words[0] === 'env' && words[1] === 'status' && words.length === 2) {
      const environment = await this.current(agent.session, signal)
      return {
        kind: 'success',
        text: `${environment.name} (${environment.id} rev ${String(environment.revision)})`,
      }
    }
    if (words[0] === 'env' && words[1] === 'use') {
      const name = words[2]
      if (words.length !== 3 || name === undefined || name.startsWith('-')) {
        return { kind: 'error', text: USAGE }
      }
      const environment = await this.use(agent, name, signal)
      return {
        kind: 'success',
        text: `${environment.name} (${environment.id} rev ${String(environment.revision)})`,
      }
    }
    if (words[0] === 'env' && words[1] === 'create' && words.length === 3) {
      return { kind: 'success', text: await this.cli.create(words[2] as string, signal) }
    }
    if (words[0] === 'skill' && words[1] === 'add' && validSkillAdd(words)) {
      return { kind: 'success', text: await this.cli.addSkill(words, signal) }
    }
    return { kind: 'error', text: USAGE }
  }
}

function validSkillAdd(words: readonly string[]): boolean {
  if (words.length < 5 || words[2] === undefined || words[2].startsWith('-')) return false
  let envCount = 0
  let index = 3
  while (index < words.length) {
    const flag = words[index]
    const value = words[index + 1]
    if (flag === '--env' && value !== undefined && !value.startsWith('-')) envCount += 1
    else if (flag !== '--policy' || (value !== 'auto' && value !== 'off')) return false
    index += 2
  }
  return index === words.length && envCount > 0
}

function renderFailure(error: unknown): string {
  if (error instanceof HevCliError) {
    return `hev: ${String(error.statusCode)}: ${error.message}${error.prompt === '' ? '' : ` (${error.prompt})`}`
  }
  try {
    return `hev: ${String(error)}`
  } catch {
    return 'hev: <unrenderable failure>'
  }
}

export default EnvironmentController
