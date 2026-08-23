/**
 * hev → DeepSeek Harness adapter (prototype).
 *
 * One command surface (`/hev`) and one per-agent skill provider. Activating an
 * environment group registers a provider into THAT agent's layer of
 * `ctx.skills`, so one session's model-facing skill catalog changes without
 * touching any other session and without recomposing an agent preset.
 *
 * The provider is registered through `agent.ctx.get('skills')`: the registry
 * files a registration into the layer of the CALLING context's scope, and an
 * agent's scope key is the agent itself.
 *
 * @module @hev/dsh-plugin
 */

import { execFile } from 'node:child_process'
import { stat } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import type { Context } from '@deepseek-ai/cordis'
import type { Agent } from '@deepseek-ai/dsh-agent'
import type { CommandResult } from '@deepseek-ai/dsh-commands/types'
import {
  isSkillName,
  type SkillCandidate,
  type SkillDefinition,
  type SkillInvocationPolicy,
  type SkillLookupOptions,
  type SkillProvider,
} from '@deepseek-ai/dsh-skill'
import { FileSystemSkillProvider } from '@deepseek-ai/dsh-skill-filesystem'

/** Cordis plugin name. */
export const name = 'hev'
/** Services this adapter consumes. */
export const inject = ['skills', 'commands']

/** The one provider name this adapter registers per agent. */
const PROVIDER = 'hev-env'
/** First rank handed to an activated skill; later skills in the group rank higher (lose ties). */
const FIRST_RANK = 100
const DEFAULT_TIMEOUT_MS = 10_000
const SCHEMA_VERSION = 1

/** Adapter configuration. */
export interface Config {
  /** `hev` executable, resolved on PATH when relative. */
  cli?: string
  /** Milliseconds one CLI call may take before it is abandoned. */
  timeoutMs?: number
  /**
   * Environment group bound to every agent as it is created — the
   * non-interactive path (one-shot runs, automation) and the shape session
   * restore will reuse. It runs through the same `/hev` command as an
   * interactive activation, so the decision set reaches the session log
   * identically.
   */
  activate?: string[]
}

/** Skill execution mode as the CLI reports it; only `off` changes dsh invocation policy today. */
export interface HevMode {
  type: 'auto' | 'always' | 'interval' | 'off'
  everyTurns?: number
}

interface HevEnvSkill {
  id: string
  identity: { skillName: string; creator?: string; version?: string }
  realPath: string
  mode: HevMode
}

interface HevEnv {
  id: string
  name: string
  revision: number
  skills: HevEnvSkill[]
}

interface LoadedSkill {
  /** What this adapter advertises: env-level rank, source, mode-adjusted invocation. */
  candidate: SkillCandidate
  /**
   * The same skill addressed as dsh's own filesystem provider expects it, so
   * every frontmatter read goes through dsh's parser instead of a second one.
   */
  parserCandidate: SkillCandidate
}

/**
 * What this adapter decided about one skill of the activated group. Recorded
 * verbatim in the session log through the command's `command/done` text, so a
 * later reader can tell what the model was allowed to see from what was
 * withheld — without hev-private state and without a custom session event type
 * (dsh refuses to interpret a log carrying event types it does not know).
 */
export interface SkillDecision {
  name: string
  /** `admitted` = in the model catalog; `user-only` = hidden from the model, still `/name`-loadable; `excluded` = not installed. */
  outcome: 'admitted' | 'user-only' | 'excluded'
  env: string
  mode: HevMode['type']
  reason?: string
}

/** One agent's live environment binding. */
interface Binding {
  envs: { id: string; name: string; revision: number }[]
  dispose: () => void
  provider: SkillProvider
  decisions: SkillDecision[]
}

/** Render the decision set as the durable, greppable text the session log keeps. */
function renderDecisions(envs: readonly HevEnv[], decisions: readonly SkillDecision[]): string {
  const count = (outcome: SkillDecision['outcome']): number => decisions.filter(decision => decision.outcome === outcome).length
  return [
    `hev env: ${envs.map(env => `${env.name}@${String(env.revision)}`).join(' ')} — `
    + `${String(count('admitted'))} model-visible, ${String(count('user-only'))} user-only, ${String(count('excluded'))} excluded`,
    ...decisions.map(decision => `hev skill ${decision.outcome}: ${decision.name} (env=${decision.env} mode=${decision.mode})`
      + (decision.reason === undefined ? '' : ` — ${decision.reason}`)),
  ].join('\n')
}

class HevCliError extends Error {
  constructor(readonly code: string, message: string) {
    super(message)
  }
}

/** Provider over one agent's activated environment group. */
class HevEnvProvider implements SkillProvider {
  readonly name = PROVIDER

  constructor(
    private readonly skills: readonly LoadedSkill[],
    private readonly parser: FileSystemSkillProvider,
  ) {}

  list(): Promise<SkillCandidate[]> {
    return Promise.resolve(this.skills.map(skill => skill.candidate))
  }

  async get(candidate: SkillCandidate, options: SkillLookupOptions): Promise<SkillDefinition | undefined> {
    const match = this.skills.find(skill => skill.candidate.name === candidate.name)
    if (match === undefined) return undefined
    // Body and frontmatter come fresh from disk through dsh's own parser; the
    // file may have changed since activation.
    const parsed = await this.parser.get(match.parserCandidate, options)
    if (parsed === undefined) return undefined
    // A file renamed after activation would be dropped by the registry anyway
    // (and would invalidate the catalog); refuse it here instead.
    if (parsed.name !== candidate.name) return undefined
    // Policy and identity stay the ADAPTER's: the env-level mode override would
    // otherwise be lost on every load.
    return {
      ...parsed,
      provider: PROVIDER,
      source: candidate.source,
      invocation: candidate.invocation,
      ...candidate.metadata === undefined ? {} : { metadata: candidate.metadata },
    }
  }
}

/**
 * Register the `/hev` command and the per-agent environment provider.
 * @param ctx - mounting context; a plain (unscoped) context is correct here — the
 *   per-agent registrations happen later, through each agent's own context.
 * @param config - CLI location and timeout.
 */
export function apply(ctx: Context, config: Config = {}): void {
  const cli = config.cli ?? 'hev'
  const timeoutMs = config.timeoutMs ?? DEFAULT_TIMEOUT_MS
  const bindings = new Map<Agent, Binding>()
  // Activation is serialized process-wide: two concurrent activations of one
  // agent would race the dispose/register pair below.
  let queue: Promise<unknown> = Promise.resolve()

  // dsh's filesystem provider used as a PARSER ONLY: no default roots, no
  // custom roots, no watchers. Its `get()` reads exactly the file named by the
  // locator it is handed, so hev keeps one frontmatter dialect (dsh's) without
  // inheriting root discovery — nothing is scanned, and no sibling of a skill's
  // real path is ever visible to it.
  const parserLifecycle = new AbortController()
  const parser = new FileSystemSkillProvider(
    ctx,
    { signal: parserLifecycle.signal, invalidate: () => {} },
    { providerName: 'hev-parser', includeDefaultRoots: false, customSkillDirs: [], watch: false },
  )

  ctx.effect(function* () {
    yield () => {
      for (const binding of [...bindings.values()]) binding.dispose()
      bindings.clear()
      parserLifecycle.abort(new Error('hev unmounted'))
    }
  }, 'hev env bindings')

  const serialize = async <T>(task: () => Promise<T>): Promise<T> => {
    const run = queue.then(task, task)
    queue = run.catch(() => undefined)
    return await run
  }

  /** Replace one agent's binding: validate everything first, then swap atomically. */
  const bind = async (agent: Agent, args: string[], signal: AbortSignal): Promise<CommandResult> => {
    let envs: HevEnv[]
    try {
      envs = await callCli(cli, args, timeoutMs, signal)
    } catch (error) {
      return { kind: 'error', text: cliFailureText(error) }
    }

    // Load and validate BEFORE touching the registry: provider names are unique
    // per layer, so the new provider cannot be registered while the old one is
    // still in place. Pre-validation shrinks the failure window to the single
    // `registerProvider` call below.
    const loaded: LoadedSkill[] = []
    const decisions: SkillDecision[] = []
    let rank = FIRST_RANK
    for (const env of envs) {
      for (const skill of env.skills) {
        const result = await loadSkill(parser, env, skill, rank, signal)
        rank += 1
        if ('warning' in result) {
          decisions.push({
            name: skill.identity.skillName,
            outcome: 'excluded',
            env: env.name,
            mode: skill.mode.type,
            reason: result.warning,
          })
          continue
        }
        loaded.push(result.skill)
        decisions.push({
          name: result.skill.candidate.name,
          outcome: result.skill.candidate.invocation.modelInvocable ? 'admitted' : 'user-only',
          env: env.name,
          mode: skill.mode.type,
        })
      }
    }

    const registry = agent.ctx.get('skills')
    if (registry === undefined) return { kind: 'error', text: 'hev: this deployment has no skill registry' }

    const previous = bindings.get(agent)
    previous?.dispose()
    const provider = new HevEnvProvider(loaded, parser)
    let dispose: () => void
    try {
      dispose = registry.registerProvider(() => provider)
    } catch (error) {
      // Roll back to the previous environment group so a failed switch does not
      // silently leave the agent with no skills at all.
      if (previous !== undefined) {
        try {
          const restored = registry.registerProvider(() => previous.provider)
          bindings.set(agent, { ...previous, dispose: restored })
        } catch {
          // The agent is being torn down; there is nothing left to restore into.
          bindings.delete(agent)
        }
      }
      return { kind: 'error', text: `hev: could not install environment: ${errorText(error)}` }
    }
    bindings.set(agent, {
      envs: envs.map(env => ({ id: env.id, name: env.name, revision: env.revision })),
      dispose,
      provider,
      decisions,
    })
    return { kind: 'success', text: renderDecisions(envs, decisions) }
  }

  ctx.commands.register({
    name: 'hev',
    description: 'Manage hev skill environments for this session.',
    input: { hint: 'env activate <name...> | env deactivate | env status' },
    handler: async ({ agent, rawInput, signal }) => {
      const words = rawInput.trim().split(/\s+/u).filter(word => word.length > 0)
      if (words[0] !== 'env') return { kind: 'error', text: 'hev: usage: /hev env activate <name...> | deactivate | status' }
      const verb = words[1]
      const rest = words.slice(2)
      if (verb === 'activate') {
        if (rest.length === 0) return { kind: 'error', text: 'hev: activate needs at least one environment name' }
        return await serialize(() => bind(agent, ['env', 'activate', ...rest, '--output', 'json'], signal))
      }
      if (verb === 'deactivate') {
        return await serialize(() => bind(agent, ['env', 'deactivate', '--output', 'json'], signal))
      }
      if (verb === 'status') {
        const binding = bindings.get(agent)
        if (binding === undefined) return { kind: 'success', text: 'hev: no environment activated in this session' }
        return {
          kind: 'success',
          text: binding.envs.map(env => `${env.name} (${env.id} rev ${String(env.revision)})`).join(', '),
        }
      }
      return { kind: 'error', text: `hev: unknown subcommand "${String(verb)}"` }
    },
  })

  const startup = new Map<Agent, Promise<unknown>>()
  const activate = config.activate ?? []
  if (activate.length > 0) {
    // Dispatched through the command registry rather than calling `bind` here,
    // so a non-interactive activation lands the same `command/run` /
    // `command/done` pair in the session log as a typed one.
    ctx.on('agent/created', ({ agent }) => {
      startup.set(agent, ctx.commands.execute(
        agent,
        `/hev env activate ${activate.join(' ')}`,
        [],
        new AbortController().signal,
      ).then((execution) => {
        if (execution?.result.kind === 'error') ctx.logger.warn(`hev: startup activation failed: ${execution.result.text}`)
      }, (error: unknown) => {
        ctx.logger.warn(`hev: startup activation threw: ${errorText(error)}`)
      }))
    })
    // No step may compose a catalog before the startup binding has landed,
    // or the first request would advertise the pre-activation skill set.
    ctx.on('agent/pre-step', async ({ agent }, next) => {
      await startup.get(agent)
      return await next()
    })
  }
}

/** Run one CLI command and validate the documented JSON contract. */
async function callCli(cli: string, args: string[], timeoutMs: number, signal: AbortSignal): Promise<HevEnv[]> {
  const stdout = await new Promise<string>((resolve, reject) => {
    execFile(cli, args, { timeout: timeoutMs, signal, encoding: 'utf8' }, (error, out) => {
      // A documented failure still prints its structured error on stdout, so a
      // non-zero exit is only fatal when nothing parseable came back.
      if (error !== null && out.trim().length === 0) reject(new HevCliError('CLI_UNAVAILABLE', errorText(error)))
      else resolve(out)
    })
  })
  let payload: unknown
  try {
    payload = JSON.parse(stdout)
  } catch {
    throw new HevCliError('CLI_PROTOCOL', 'stdout was not a JSON object')
  }
  if (typeof payload !== 'object' || payload === null) throw new HevCliError('CLI_PROTOCOL', 'stdout was not a JSON object')
  const envelope = payload as { schemaVersion?: unknown; ok?: unknown; data?: unknown; error?: unknown }
  if (envelope.schemaVersion !== SCHEMA_VERSION) {
    throw new HevCliError('CLI_PROTOCOL', `unsupported schemaVersion ${String(envelope.schemaVersion)}`)
  }
  if (envelope.ok !== true) {
    const error = envelope.error as { code?: unknown; message?: unknown } | undefined
    throw new HevCliError(
      typeof error?.code === 'string' ? error.code : 'INTERNAL_ERROR',
      typeof error?.message === 'string' ? error.message : 'unspecified failure',
    )
  }
  const environments = (envelope.data as { environments?: unknown } | undefined)?.environments
  if (!Array.isArray(environments)) throw new HevCliError('CLI_PROTOCOL', 'data.environments must be an array')
  return environments.map(validateEnv)
}

function validateEnv(value: unknown, index: number): HevEnv {
  const env = value as Partial<HevEnv>
  if (typeof env.id !== 'string' || typeof env.name !== 'string' || typeof env.revision !== 'number') {
    throw new HevCliError('CLI_PROTOCOL', `environment ${String(index)} is missing id, name, or revision`)
  }
  if (!Array.isArray(env.skills)) throw new HevCliError('CLI_PROTOCOL', `environment "${env.name}" has no skills array`)
  return {
    id: env.id,
    name: env.name,
    revision: env.revision,
    skills: env.skills.map((skill: unknown) => {
      const entry = skill as Partial<HevEnvSkill>
      const skillName = entry.identity?.skillName
      if (typeof skillName !== 'string' || typeof entry.realPath !== 'string') {
        throw new HevCliError('CLI_PROTOCOL', `environment "${env.name as string}" has a skill without identity.skillName or realPath`)
      }
      return {
        id: typeof entry.id === 'string' ? entry.id : skillName,
        identity: entry.identity as HevEnvSkill['identity'],
        realPath: entry.realPath,
        mode: normalizeMode(entry.mode),
      }
    }),
  }
}

function normalizeMode(mode: unknown): HevMode {
  const type = (mode as HevMode | undefined)?.type
  return type === 'always' || type === 'interval' || type === 'off' ? { type } : { type: 'auto' }
}

/**
 * Read one env skill through dsh's parser and turn it into a candidate, or
 * explain why it was skipped.
 * @param parser - rootless filesystem provider used only to parse one file.
 */
async function loadSkill(
  parser: FileSystemSkillProvider,
  env: HevEnv,
  skill: HevEnvSkill,
  rank: number,
  signal: AbortSignal,
): Promise<{ skill: LoadedSkill } | { warning: string }> {
  let path: string
  let directory: string
  try {
    const info = await stat(skill.realPath)
    if (info.isDirectory()) {
      path = join(skill.realPath, 'SKILL.md')
      directory = skill.realPath
    } else {
      path = skill.realPath
      directory = dirname(skill.realPath)
    }
  } catch {
    return { warning: `${skill.identity.skillName}: realPath is unreadable (${skill.realPath})` }
  }
  const source = `hev:${env.name}`
  const parserCandidate: SkillCandidate = {
    name: skill.identity.skillName,
    description: 'pending frontmatter',
    invocation: { modelInvocable: true, userInvocable: true },
    source,
    provider: 'hev-parser',
    rank,
    locator: { path, directory },
    path,
  }
  const parsed = await parser.get(parserCandidate, { signal })
  if (parsed === undefined) return { warning: `${skill.identity.skillName}: no readable SKILL.md with valid frontmatter at ${path}` }
  if (parsed.name !== skill.identity.skillName) {
    // dsh drops a definition whose loaded name differs from its candidate name,
    // so a registry that disagrees with the file would churn the catalog.
    return { warning: `${skill.identity.skillName}: frontmatter declares name "${parsed.name}" — the registry and the file must agree` }
  }
  if (!isSkillName(parsed.name)) return { warning: `${parsed.name}: not a kebab-case skill name` }
  if (parsed.description.length === 0) return { warning: `${parsed.name}: frontmatter requires a description` }
  return {
    skill: {
      parserCandidate,
      candidate: {
        name: parsed.name,
        description: parsed.description,
        ...parsed.whenToUse === undefined ? {} : { whenToUse: parsed.whenToUse },
        invocation: invocationFor(skill.mode, parsed.invocation),
        source,
        provider: PROVIDER,
        rank,
        locator: { path },
        path,
        resourceBase: { kind: 'directory', path: directory },
        metadata: {
          ...parsed.metadata,
          hevEnvId: env.id,
          hevEnvName: env.name,
          hevSkillId: skill.id,
          hevMode: skill.mode,
        },
      },
    },
  }
}

/**
 * Apply the env-level mode over the file's own policy. `off` is the only mode
 * dsh can express structurally today: the skill leaves the model catalog but
 * stays reachable through the user's explicit `/name` gesture.
 */
function invocationFor(mode: HevMode, filePolicy: SkillInvocationPolicy): SkillInvocationPolicy {
  if (mode.type === 'off') return { modelInvocable: false, userInvocable: filePolicy.userInvocable }
  return filePolicy
}

function cliFailureText(error: unknown): string {
  return error instanceof HevCliError ? `hev: ${error.code}: ${error.message}` : `hev: ${errorText(error)}`
}

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}
