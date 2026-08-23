/** Live, session-scoped HEV Environment selection for DeepSeek Harness.
 * @module @hev/dsh-runtime
 */

import { Context, Service } from '@deepseek-ai/cordis'
import type { Agent } from '@deepseek-ai/dsh-agent'
import type { Session } from '@deepseek-ai/dsh-session'
import z from '@deepseek-ai/schemastery'
import { HevCliClient, HevCliError } from './cli-client.ts'
import type { NativeCommandRunner } from './cli-client.ts'
import type { EnvironmentId, ResolvedEnvironmentSnapshot } from './types.ts'

// Type-only edge: resolves ctx.commands for the optional command child.
import type {} from '@deepseek-ai/dsh-commands'
export { EnvironmentId } from './types.ts'
export type {
  Environment,
  EnvironmentSkillPolicy,
  EnvironmentSkillSpec,
  ResolvedEnvironmentSnapshot,
  SkillPolicyKind,
} from './types.ts'
export { HevCliClient, HevCliError } from './cli-client.ts'
export type { NativeCommandRunner } from './cli-client.ts'

declare module '@deepseek-ai/cordis' {
  interface Context {
    environment: EnvironmentController
  }
}

/** Runtime plugin configuration. */
export interface Config {
  /** HEV executable path or PATH name; defaults to `hev`. */
  readonly executable?: string
}

interface RuntimeDependencies {
  readonly runner?: NativeCommandRunner
}

const DEFAULT_EXECUTABLE = 'hev'
const USAGE = 'usage: /hev env create <name> | skill add <skill-key> --env <name> [--env <name>...] [--policy auto|off] | env activate <id-or-name> [id-or-name...]'

/** Owns live Environment selection and the optional `/hev` command. */
export class EnvironmentController extends Service {
  static Config: z<Config> = z.object({
    executable: z.string().default(DEFAULT_EXECUTABLE),
  })

  private readonly active = new WeakMap<Session, readonly EnvironmentId[]>()
  private readonly cli: HevCliClient

  /**
   * Create the runtime service.
   * @param ctx - Cordis context receiving the Environment service.
   * @param config - executable configuration.
   * @param dependencies - test-only native runner injection.
   */
  constructor(ctx: Context, config: Config = {}, dependencies: RuntimeDependencies = {}) {
    super(ctx, 'environment')
    const executable = config.executable ?? DEFAULT_EXECUTABLE
    if (executable.trim() === '') throw new Error('HEV executable must not be empty')
    this.cli = new HevCliClient(executable, dependencies.runner)

    ctx.inject(['commands'], (commandCtx) => {
      commandCtx.commands.register({
        name: 'hev',
        description: 'Manage HEV skill environments',
        input: { hint: 'env create | skill add | env activate' },
        handler: async ({ agent, rawInput, signal }) => {
          try {
            return await this.executeCommand(agent, splitWords(rawInput), signal)
          } catch (error: unknown) {
            return { kind: 'error', text: renderFailure(error) }
          }
        },
      })
    })
  }

  /**
   * Resolve the latest snapshot for one exact live Session.
   * @param session - exact in-process Session identity.
   * @param signal - optional operation cancellation signal.
   * @returns latest snapshot, or `undefined` when this Session has no selection.
   */
  async current(
    session: Session,
    signal?: AbortSignal,
  ): Promise<ResolvedEnvironmentSnapshot | undefined> {
    const environmentIds = this.active.get(session)
    if (environmentIds === undefined) return undefined
    return await this.cli.activate(environmentIds, signal ?? new AbortController().signal)
  }

  /**
   * Resolve and atomically select an Environment group for one live agent.
   * @param agent - exact agent whose Session receives the selection.
   * @param environmentRefs - Environment IDs or names in composition order.
   * @param signal - optional operation cancellation signal.
   * @returns the committed latest snapshot.
   */
  async activate(
    agent: Agent,
    environmentRefs: readonly string[],
    signal?: AbortSignal,
  ): Promise<ResolvedEnvironmentSnapshot> {
    const operationSignal = signal ?? new AbortController().signal
    const snapshot = await this.cli.activate(environmentRefs, operationSignal)
    const canonicalIds = Object.freeze(snapshot.environments.map(environment => environment.id))
    this.active.set(agent.session, canonicalIds)
    return snapshot
  }

  private async executeCommand(
    agent: Agent,
    words: readonly string[],
    signal: AbortSignal,
  ): Promise<{ kind: 'success'; text?: string } | { kind: 'error'; text: string }> {
    if (words[0] === 'env' && words[1] === 'activate') {
      const environmentRefs = words.slice(2)
      if (environmentRefs.length === 0 || environmentRefs.some(reference => reference.startsWith('-'))) {
        return { kind: 'error', text: USAGE }
      }
      const snapshot = await this.activate(agent, environmentRefs, signal)
      return {
        kind: 'success',
        text: snapshot.environments
          .map(environment => `${environment.name} (${environment.id} rev ${String(environment.revision)})`)
          .join(', '),
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

function splitWords(rawInput: string): readonly string[] {
  return rawInput.trim().split(/\s+/u).filter(word => word.length > 0)
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
    return `hev: ${error.code}: ${error.message}${error.prompt === '' ? '' : ` (${error.prompt})`}`
  }
  try {
    return `hev: ${String(error)}`
  } catch {
    return 'hev: <unrenderable failure>'
  }
}

export default EnvironmentController
