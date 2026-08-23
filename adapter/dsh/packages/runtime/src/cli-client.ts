/** Strict HEV CLI v1 decoding over an injected native command runner.
 * @module @hev/dsh-runtime/cli-client
 */

import { runNativeCommand } from '@deepseek-ai/dsh-native-command'
import type { NativeCommandRunner } from '@deepseek-ai/dsh-native-command'
import { EnvironmentId } from './types.ts'
import type { Environment, EnvironmentSkillSpec, ResolvedEnvironmentSnapshot } from './types.ts'

const SCHEMA_VERSION = 1
const NAME = /^[a-z0-9]+(?:-[a-z0-9]+)*$/u
const ERROR_CODES = new Set([
  'INVALID_ARGUMENT',
  'ENV_NOT_FOUND',
  'ENV_ALREADY_EXISTS',
  'SKILL_ALREADY_BOUND',
  'SKILL_CONFLICT',
  'INTERNAL_ERROR',
])

/** Structured failure returned by HEV or raised while decoding its output. */
export class HevCliError extends Error {
  /** Stable HEV error code, or a local adapter protocol code. */
  readonly code: string
  /** Recovery guidance returned by the CLI. */
  readonly prompt: string

  /**
   * Create a CLI failure.
   * @param code - stable machine-readable code.
   * @param message - human-readable failure.
   * @param prompt - optional CLI recovery guidance.
   * @param cause - underlying runner or parser failure.
   */
  constructor(code: string, message: string, prompt = '', cause?: unknown) {
    super(message, cause === undefined ? undefined : { cause })
    this.name = 'HevCliError'
    this.code = code
    this.prompt = prompt
  }
}

interface RunnerFailure {
  readonly stdout?: unknown
}

interface Envelope {
  readonly code: number
  readonly message: string
  readonly prompt: string
  readonly data: Record<string, unknown>
}

/** HEV CLI operations used by the runtime and its optional slash command. */
export class HevCliClient {
  /**
   * Create a client.
   * @param executable - executable path or PATH name.
   * @param runner - injected no-shell runner.
   */
  constructor(
    private readonly executable: string,
    private readonly runner: NativeCommandRunner = runNativeCommand,
  ) {}

  /** Resolve an ordered Environment group through the CLI.
   * @param environmentRefs - Environment IDs or names in composition order.
   * @param signal - operation cancellation signal.
   * @returns the latest validated snapshot.
   */
  async activate(
    environmentRefs: readonly string[],
    signal: AbortSignal,
  ): Promise<ResolvedEnvironmentSnapshot> {
    if (environmentRefs.length === 0) {
      throw new HevCliError('INVALID_ARGUMENT', 'at least one environment is required')
    }
    if (environmentRefs.some(reference => reference.length === 0
      || reference.trim() !== reference
      || /\s/u.test(reference)
      || reference.startsWith('-'))) {
      throw new HevCliError('INVALID_ARGUMENT', 'environment references must be non-empty argv values without whitespace')
    }
    const envelope = await this.invoke(['env', 'activate', ...environmentRefs, '--output', 'json'], signal)
    const snapshot = record(envelope.data.snapshot, 'data.snapshot')
    const environments = array(snapshot.environments, 'data.snapshot.environments')
    if (environments.length === 0) protocol('data.snapshot.environments must not be empty')

    const seenEnvironments = new Set<string>()
    const seenSkills = new Set<string>()
    const decoded = environments.map((value, index) => {
      const environment = decodeEnvironment(value, `data.snapshot.environments[${String(index)}]`)
      if (seenEnvironments.has(environment.id)) protocol(`duplicate environment id "${environment.id}"`)
      seenEnvironments.add(environment.id)
      for (const skill of environment.skills) {
        if (seenSkills.has(skill.skillKey)) protocol(`duplicate skill key "${skill.skillKey}"`)
        seenSkills.add(skill.skillKey)
      }
      return environment
    })
    return Object.freeze({ environments: Object.freeze(decoded) })
  }

  /** Create one empty Environment through the CLI.
   * @param name - lowercase kebab-case Environment name.
   * @param signal - operation cancellation signal.
   * @returns the CLI's success message.
   */
  async create(name: string, signal: AbortSignal): Promise<string> {
    if (!NAME.test(name)) {
      throw new HevCliError('INVALID_ARGUMENT', 'environment name must be lowercase kebab-case')
    }
    const envelope = await this.invoke(['env', 'create', name, '--output', 'json'], signal)
    decodeEnvironment(envelope.data.environment, 'data.environment')
    return envelope.message
  }

  /** Add one Skill binding through the CLI.
   * @param args - exact `skill add` argv excluding the JSON-output suffix.
   * @param signal - operation cancellation signal.
   * @returns the CLI's success message.
   */
  async addSkill(args: readonly string[], signal: AbortSignal): Promise<string> {
    const envelope = await this.invoke([...args, '--output', 'json'], signal)
    decodeSkill(envelope.data.environmentSkill, 'data.environmentSkill')
    const environments = array(envelope.data.environments, 'data.environments')
    if (environments.length === 0) protocol('data.environments must not be empty')
    const seen = new Set<string>()
    environments.forEach((value, index) => {
      const summary = decodeEnvironmentSummary(value, `data.environments[${String(index)}]`)
      if (seen.has(summary.id)) protocol(`duplicate environment id "${summary.id}"`)
      seen.add(summary.id)
    })
    return envelope.message
  }

  private async invoke(args: readonly string[], signal: AbortSignal): Promise<Envelope> {
    let stdout: string
    try {
      stdout = (await this.runner(this.executable, args, signal)).stdout
    } catch (error: unknown) {
      const candidate = typeof (error as RunnerFailure | null)?.stdout === 'string'
        ? (error as RunnerFailure).stdout as string
        : ''
      if (candidate.trim() === '') {
        throw new HevCliError('CLI_UNAVAILABLE', renderThrown(error), '', error)
      }
      return decodeEnvelope(candidate, error)
    }
    return decodeEnvelope(stdout)
  }
}

function decodeEnvelope(stdout: string, cause?: unknown): Envelope {
  let value: unknown
  try {
    value = JSON.parse(stdout)
  } catch (error: unknown) {
    throw new HevCliError('CLI_PROTOCOL', 'HEV CLI stdout is not valid JSON', '', cause ?? error)
  }
  const envelope = record(value, 'response')
  const schemaVersion = integer(envelope.schemaVersion, 'schemaVersion')
  if (schemaVersion !== SCHEMA_VERSION) protocol(`unsupported schemaVersion ${String(schemaVersion)}`)
  const code = integer(envelope.code, 'code')
  const message = nonEmptyString(envelope.message, 'message')
  const prompt = string(envelope.prompt, 'prompt')
  const data = record(envelope.data, 'data')
  if (code !== 200) {
    if (code < 400 || code > 599) protocol(`code must be 200 or an integer from 400 through 599`)
    const errorCode = nonEmptyString(data.errorCode, 'data.errorCode')
    if (!ERROR_CODES.has(errorCode)) protocol(`unsupported data.errorCode "${errorCode}"`)
    throw new HevCliError(errorCode, message, prompt, cause)
  }
  if (cause !== undefined) {
    throw new HevCliError('CLI_PROTOCOL', 'HEV CLI exited unsuccessfully with a success response', '', cause)
  }
  if (prompt !== '') protocol('prompt must be empty for a success response')
  return { code, message, prompt, data }
}

function decodeEnvironment(value: unknown, path: string): Environment {
  const input = record(value, path)
  const id = environmentId(input.id, `${path}.id`)
  const name = nonEmptyString(input.name, `${path}.name`)
  if (!NAME.test(name)) protocol(`${path}.name must be lowercase kebab-case`)
  const revision = integer(input.revision, `${path}.revision`)
  if (revision < 1) protocol(`${path}.revision must be positive`)
  const rawSkills = array(input.skills, `${path}.skills`)
  const localSkills = new Set<string>()
  const skills = rawSkills.map((skill, index) => {
    const decoded = decodeSkill(skill, `${path}.skills[${String(index)}]`)
    if (localSkills.has(decoded.skillKey)) protocol(`${path}.skills contains duplicate skill key "${decoded.skillKey}"`)
    localSkills.add(decoded.skillKey)
    return decoded
  })
  return Object.freeze({
    id: EnvironmentId(id),
    name,
    revision,
    skills: Object.freeze(skills),
  })
}

function decodeEnvironmentSummary(value: unknown, path: string): { id: EnvironmentId } {
  const input = record(value, path)
  const id = environmentId(input.id, `${path}.id`)
  const name = nonEmptyString(input.name, `${path}.name`)
  if (!NAME.test(name)) protocol(`${path}.name must be lowercase kebab-case`)
  const revision = integer(input.revision, `${path}.revision`)
  if (revision < 1) protocol(`${path}.revision must be positive`)
  return { id: EnvironmentId(id) }
}

function decodeSkill(value: unknown, path: string): EnvironmentSkillSpec {
  const input = record(value, path)
  const skillKey = nonEmptyString(input.skillKey, `${path}.skillKey`)
  if (!NAME.test(skillKey)) protocol(`${path}.skillKey must be lowercase kebab-case`)
  const policy = record(input.policy, `${path}.policy`)
  const kind = policy.kind
  if (kind !== 'auto' && kind !== 'off') protocol(`${path}.policy.kind must be "auto" or "off"`)
  return Object.freeze({ skillKey, policy: Object.freeze({ kind }) })
}

function record(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) protocol(`${path} must be an object`)
  return value as Record<string, unknown>
}

function array(value: unknown, path: string): unknown[] {
  if (!Array.isArray(value)) protocol(`${path} must be an array`)
  return value
}

function string(value: unknown, path: string): string {
  if (typeof value !== 'string') protocol(`${path} must be a string`)
  return value as string
}

function nonEmptyString(value: unknown, path: string): string {
  const decoded = string(value, path)
  if (decoded.length === 0) protocol(`${path} must not be empty`)
  return decoded
}

function environmentId(value: unknown, path: string): string {
  const decoded = nonEmptyString(value, path)
  if (decoded.trim() !== decoded || /\s/u.test(decoded)) protocol(`${path} must not contain whitespace`)
  return decoded
}

function integer(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) protocol(`${path} must be a safe integer`)
  return value as number
}

function protocol(message: string): never {
  throw new HevCliError('CLI_PROTOCOL', message)
}

function renderThrown(value: unknown): string {
  try {
    return String(value)
  } catch {
    return '<unrenderable CLI failure>'
  }
}

export type { NativeCommandRunner } from '@deepseek-ai/dsh-native-command'
