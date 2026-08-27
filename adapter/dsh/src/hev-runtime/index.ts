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
import type {
  AddedEnvironmentSkill,
  CreatedEnvironment,
  Environment,
  EnvironmentSummary,
  SkillPolicyKind,
} from './environment.ts'

// Type-only edge: resolves ctx.commands for the optional command child.
import type {} from '@deepseek-ai/dsh-commands'
import type { SkillSummary, SkillViewOptions } from '@deepseek-ai/dsh-skill'
export { EnvironmentId } from './environment.ts'
export { StatusCode } from './environment.ts'
export type {
  Environment,
  EnvironmentSession,
  CreatedEnvironment,
  EnvironmentSkillPolicy,
  EnvironmentSkillSpec,
  EnvironmentSummary,
  FailureStatusCode,
  Source,
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

interface GlobalSkillReader {
  listAll(options?: SkillViewOptions): Promise<SkillSummary[]>
}

const HELP = new Map<string, string>([
  ['', `Manage hev Skill Environments for the current DSH Session.

Commands:
  /hev env create <name>
  /hev env list
  /hev env use <id-or-name>
  /hev env status
  /hev env quit
  /hev skill add <skill-key> <env-name> [env-name...] [--policy auto|off]
  /hev skill list [--global]

Use /hev help env or /hev help skill for details.`],
  ['env', `Manage Environments for the current DSH Session.

  /hev env create <name>       Create an Environment without activating it.
  /hev env list                List DSH Environments.
  /hev env use <id-or-name>    Select exactly one Environment.
  /hev env status              Show the current selection.
  /hev env quit                Move non-base to base, or base to inactive.`],
  ['env create', 'usage: /hev env create <lowercase-kebab-case-name>'],
  ['env list', 'usage: /hev env list'],
  ['env use', 'usage: /hev env use <environment-id-or-name>'],
  ['env status', 'usage: /hev env status'],
  ['env quit', 'usage: /hev env quit'],
  ['skill', `Manage Skill bindings in hev Environments.

  /hev skill add <skill-key> <env-name> [env-name...] [--policy auto|off]
      Bind a native Skill; this does not install the Skill or activate an Environment.
  /hev skill list              List bindings in the current Environment.
  /hev skill list --global     List native DSH-discoverable Skills.`],
  ['skill add', 'usage: /hev skill add <skill-key> <env-name> [env-name...] [--policy auto|off]'],
  ['skill list', 'usage: /hev skill list [--global]'],
])
/** Provides DSH Session access to Core-owned Environment selection and the optional `/hev` command. */
export class EnvironmentController extends Service {
  static Config: z<Config> = z.object({
    executable: z.string(),
  })

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
        description: 'Manage Skill Environments for the current Session',
        input: { hint: 'help | env create/list/use/status/quit | skill add/list' },
        handler: async ({ agent, rawInput, signal }) => {
          try {
            const words = rawInput.trim().split(/\s+/u).filter(word => word.length > 0)
            return await this.executeCommand(agent, words, signal)
          } catch (error: unknown) {
            this.ctx.logger.warn(`hev command failed: ${renderDiagnostic(error)}`)
            return { kind: 'error', text: renderPrompt(error) }
          }
        },
      })
    })
  }

  /**
   * Resolve the latest Environment for one exact live Session.
   * @param session - exact in-process Session identity.
   * @param signal - optional operation cancellation signal.
   * @returns the latest selected Environment, or `undefined` when hev is not activated.
   */
  async current(
    session: Session,
    signal?: AbortSignal,
  ): Promise<Environment | undefined> {
    const operationSignal = signal ?? new AbortController().signal
    const current = await this.cli.current(String(session.id), operationSignal)
    return current.environment ?? undefined
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
    const current = await this.cli.use(name, String(agent.session.id), operationSignal)
    if (current.environment === null) throw new Error('hev env use returned an inactive Session')
    return current.environment
  }

  /** Leave the current Environment tier for one live agent.
   * @param agent - exact agent whose Session leaves its current Environment.
   * @param signal - optional operation cancellation signal.
   * @returns `base` after leaving a non-base Environment, or `undefined` after leaving `base` or when already inactive.
   */
  async quit(agent: Agent, signal?: AbortSignal): Promise<Environment | undefined> {
    const current = await this.cli.quit(
      String(agent.session.id),
      signal ?? new AbortController().signal,
    )
    return current.environment ?? undefined
  }

  /** Create an Environment through the shared Core. */
  async create(name: string, signal: AbortSignal): Promise<CreatedEnvironment> {
    return await this.cli.create(name, signal)
  }

  /** List Environments through the shared Core. */
  async list(signal: AbortSignal): Promise<readonly EnvironmentSummary[]> {
    return await this.cli.listEnvironments(signal)
  }

  /** Bind one Skill to one or more Environments through the shared Core. */
  async addSkill(
    skillKey: string,
    environments: readonly string[],
    policy: SkillPolicyKind,
    signal: AbortSignal,
  ): Promise<AddedEnvironmentSkill> {
    return await this.cli.addSkill(skillKey, environments, policy, signal)
  }

  /** List the unfiltered native DSH Skill catalog for one live Agent. */
  async listGlobalSkills(agent: Agent, signal: AbortSignal): Promise<readonly SkillSummary[]> {
    const skills = this.ctx.get('skills') as Partial<GlobalSkillReader> | undefined
    if (typeof skills?.listAll !== 'function') {
      throw new Error('hev global Skill listing requires the hev Skill Registry')
    }
    const cwd = agent.session.header.cwd
    return await skills.listAll({
      scope: agent,
      signal,
      ...(cwd === undefined ? {} : { cwd }),
    })
  }

  private async executeCommand(
    agent: Agent,
    words: readonly string[],
    signal: AbortSignal,
  ): Promise<{ kind: 'success'; text?: string } | { kind: 'error'; text: string }> {
    const help = requestedHelp(words)
    if (help !== undefined) return { kind: 'success', text: help }
    if (words[0] === 'skill' && words[1] === 'list' && words.length === 3 && words[2] === '--global') {
      return { kind: 'success', text: renderGlobalSkills(await this.listGlobalSkills(agent, signal)) }
    }
    if (words[0] === 'skill' && words[1] === 'list' && words.length === 2) {
      return { kind: 'success', text: renderEnvironmentSkills(await this.current(agent.session, signal)) }
    }
    if (words[0] === 'env' && words[1] === 'list' && words.length === 2) {
      const environments = await this.list(signal)
      return { kind: 'success', text: renderEnvironmentList(environments) }
    }
    if (words[0] === 'env' && words[1] === 'quit' && words.length === 2) {
      const environment = await this.quit(agent, signal)
      return { kind: 'success', text: renderEnvironment(environment) }
    }
    if (words[0] === 'env' && words[1] === 'status' && words.length === 2) {
      const environment = await this.current(agent.session, signal)
      return { kind: 'success', text: renderEnvironment(environment) }
    }
    if (words[0] === 'env' && words[1] === 'use') {
      const name = words[2]
      if (words.length !== 3 || name === undefined || name.startsWith('-')) {
        return { kind: 'error', text: nearestHelp(words) }
      }
      const environment = await this.use(agent, name, signal)
      return { kind: 'success', text: renderEnvironment(environment) }
    }
    if (words[0] === 'env' && words[1] === 'create' && words.length === 3) {
      return { kind: 'success', text: (await this.create(words[2] as string, signal)).message }
    }
    if (words[0] === 'skill' && words[1] === 'add' && validSkillAdd(words)) {
      const policyIndex = words.indexOf('--policy', 3)
      const environmentEnd = policyIndex < 0 ? words.length : policyIndex
      const policy = policyIndex < 0 ? 'auto' : words[policyIndex + 1] as SkillPolicyKind
      const result = await this.addSkill(
        words[2] as string,
        words.slice(3, environmentEnd),
        policy,
        signal,
      )
      return { kind: 'success', text: result.message }
    }
    return { kind: 'error', text: nearestHelp(words) }
  }

}

function renderEnvironment(environment: Environment | undefined): string {
  return environment === undefined
    ? 'hev not activated'
    : `${environment.name} (${environment.id} rev ${String(environment.revision)})`
}

function renderEnvironmentList(environments: readonly { name: string; id: string; revision: number }[]): string {
  return ['environments:', ...environments.map(environment => (
    `- ${environment.name} (${environment.id} rev ${String(environment.revision)})`
  ))].join('\n')
}

function renderEnvironmentSkills(environment: Environment | undefined): string {
  if (environment === undefined) return 'hev not activated'
  return environment.skills.length === 0
    ? `${environment.name}: no skills configured`
    : [
        `${environment.name}:`,
        ...environment.skills.map(skill => `- ${skill.skillKey} (${skill.policy.kind})`),
      ].join('\n')
}

function renderGlobalSkills(skills: readonly SkillSummary[]): string {
  return skills.length === 0
    ? 'global: no skills available'
    : ['global:', ...skills.map(skill => `- ${skill.name}`)].join('\n')
}

function validSkillAdd(words: readonly string[]): boolean {
  if (words.length < 4 || words[2] === undefined || words[2].startsWith('-')) return false
  const policyIndex = words.indexOf('--policy', 3)
  const environmentEnd = policyIndex < 0 ? words.length : policyIndex
  if (environmentEnd === 3 || words.slice(3, environmentEnd).some(word => word.startsWith('-'))) return false
  return policyIndex < 0
    || (policyIndex === words.length - 2 && (words[policyIndex + 1] === 'auto' || words[policyIndex + 1] === 'off'))
}

function requestedHelp(words: readonly string[]): string | undefined {
  if (words.length === 0) return HELP.get('')
  if (words[0] === 'help') return HELP.get(words.slice(1).join(' ')) ?? HELP.get('')
  if (words.at(-1) === 'help' || words.at(-1) === '--help') {
    return HELP.get(words.slice(0, -1).join(' ')) ?? HELP.get('')
  }
  return undefined
}

function nearestHelp(words: readonly string[]): string {
  return HELP.get(words.slice(0, 2).join(' '))
    ?? HELP.get(words[0] ?? '')
    ?? HELP.get('')
    ?? 'usage: /hev help'
}

function renderDiagnostic(error: unknown): string {
  if (error instanceof HevCliError) return `${String(error.statusCode)}: ${error.message}`
  try {
    return String(error)
  } catch {
    return '<unrenderable failure>'
  }
}

function renderPrompt(error: unknown): string {
  return error instanceof HevCliError && error.prompt !== ''
    ? error.prompt
    : 'Retry the hev operation. If it still fails, inspect the DSH logs.'
}

export default EnvironmentController
