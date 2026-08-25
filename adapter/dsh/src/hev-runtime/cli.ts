/** Strict hev CLI v2 decoding over an injected native command runner.
 * @module @slimzeo/hev-dsh-plugin/hev-runtime/cli
 */

import { runNativeCommand } from '@deepseek-ai/dsh-native-command'
import type { NativeCommandRunner } from '@deepseek-ai/dsh-native-command'
import { EnvironmentId, StatusCode } from './environment.ts'
import type { Environment, EnvironmentSkillSpec, EnvironmentSummary, FailureStatusCode } from './environment.ts'

const SCHEMA_VERSION = 2
const NAME = /^[a-z0-9]+(?:-[a-z0-9]+)*$/u

/** Structured failure returned by hev or raised while decoding its output. */
export class HevCliError extends Error {
  /** HTTP-style status code from hev or the local adapter. */
  readonly statusCode: FailureStatusCode
  /** Recovery guidance returned by the CLI. */
  readonly prompt: string

  /**
   * Create a CLI failure.
   * @param statusCode - HTTP-style failure status.
   * @param message - human-readable failure.
   * @param prompt - optional CLI recovery guidance.
   * @param cause - underlying runner or parser failure.
   */
  constructor(statusCode: FailureStatusCode, message: string, prompt = '', cause?: unknown) {
    super(message, cause === undefined ? undefined : { cause })
    this.name = 'HevCliError'
    this.statusCode = statusCode
    this.prompt = prompt
  }
}

interface RunnerFailure {
  readonly stdout?: string
}

interface BaseResponse {
  readonly schemaVersion: number
  readonly code: number
  readonly message: string
  readonly prompt: string
  // Command-specific JSON is validated by the operation that consumes it.
  readonly data: any
}

/** hev CLI operations used by the runtime and its optional slash command. */
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

  /** Resolve the default Environment through the CLI.
   * @param signal - operation cancellation signal.
   * @returns the latest validated default Environment.
   */
  async defaultEnvironment(signal: AbortSignal): Promise<Environment> {
    const response = await this.invoke(['env', 'use', '--output', 'json'], signal)
    return decodeEnvironment(response.data.environment)
  }

  /** Resolve one Environment through the CLI.
   * @param name - Environment ID or name.
   * @param signal - operation cancellation signal.
   * @returns the latest validated Environment.
   */
  async use(
    name: string,
    signal: AbortSignal,
  ): Promise<Environment> {
    if (name.length === 0
      || name.trim() !== name
      || /\s/u.test(name)
      || name.startsWith('-')) {
      throw new HevCliError(StatusCode.InvalidArgument, 'environment reference must be a non-empty argv value without whitespace')
    }
    const response = await this.invoke(['env', 'use', name, '--output', 'json'], signal)
    return decodeEnvironment(response.data.environment)
  }

  /** Create one empty Environment through the CLI.
   * @param name - lowercase kebab-case Environment name.
   * @param signal - operation cancellation signal.
   * @returns the CLI's success message.
   */
  async create(name: string, signal: AbortSignal): Promise<string> {
    if (!NAME.test(name)) {
      throw new HevCliError(StatusCode.InvalidArgument, 'environment name must be lowercase kebab-case')
    }
    const response = await this.invoke(['env', 'create', name, '--output', 'json'], signal)
    decodeEnvironment(response.data.environment)
    return response.message
  }

  /** List all current Environments through the CLI.
   * @param signal - operation cancellation signal.
   * @returns validated Environment metadata ordered by the Core.
   */
  async listEnvironments(signal: AbortSignal): Promise<readonly EnvironmentSummary[]> {
    const response = await this.invoke(['env', 'list', '--output', 'json'], signal)
    const environments = array(response.data.environments, 'data.environments')
    if (environments.length === 0) protocol('data.environments must not be empty')
    const seen = new Set<string>()
    const decoded = environments.map((value, index) => {
      const environment = decodeEnvironmentSummary(value, `data.environments[${String(index)}]`)
      if (seen.has(environment.id)) protocol(`duplicate environment id "${environment.id}"`)
      seen.add(environment.id)
      return environment
    })
    return Object.freeze(decoded)
  }

  /** Add one Skill binding through the CLI.
   * @param args - exact `skill add` argv excluding the JSON-output suffix.
   * @param signal - operation cancellation signal.
   * @returns the CLI's success message.
   */
  async addSkill(args: readonly string[], signal: AbortSignal): Promise<string> {
    const response = await this.invoke([...args, '--output', 'json'], signal)
    decodeSkill(response.data.environmentSkill, 'data.environmentSkill')
    const environments = array(response.data.environments, 'data.environments')
    if (environments.length === 0) protocol('data.environments must not be empty')
    const seen = new Set<string>()
    environments.forEach((value, index) => {
      const environment = decodeEnvironmentSummary(value, `data.environments[${String(index)}]`)
      if (seen.has(environment.id)) protocol(`duplicate environment id "${environment.id}"`)
      seen.add(environment.id)
    })
    return response.message
  }

  private async invoke(args: readonly string[], signal: AbortSignal): Promise<BaseResponse> {
    let stdout: string
    try {
      stdout = (await this.runner(this.executable, args, signal)).stdout
    } catch (error: unknown) {
      const candidate = (error as RunnerFailure | null)?.stdout ?? ''
      if (candidate.trim() === '') {
        throw new HevCliError(StatusCode.Unavailable, renderThrown(error), '', error)
      }
      return decodeResponse(candidate, error)
    }
    return decodeResponse(stdout)
  }
}

function decodeResponse(stdout: string, cause?: unknown): BaseResponse {
  let response: BaseResponse
  try {
    response = JSON.parse(stdout) as BaseResponse
  } catch (error: unknown) {
    throw new HevCliError(StatusCode.ProtocolError, 'hev CLI stdout is not valid JSON', '', cause ?? error)
  }
  record(response, 'response')
  const schemaVersion = integer(response.schemaVersion, 'schemaVersion')
  if (schemaVersion !== SCHEMA_VERSION) protocol(`unsupported schemaVersion ${String(schemaVersion)}`)
  const decodedCode = integer(response.code, 'code')
  if (decodedCode !== StatusCode.Ok && !isCliFailureStatus(decodedCode)) {
    protocol('code must be 200, 400, 404, 409, or 500')
  }
  const code = decodedCode as typeof StatusCode.Ok | FailureStatusCode
  const message = nonEmptyString(response.message, 'message')
  const prompt = string(response.prompt, 'prompt')
  record(response.data, 'data')
  if (code !== StatusCode.Ok) throw new HevCliError(code, message, prompt, cause)
  if (cause !== undefined) {
    throw new HevCliError(StatusCode.ProtocolError, 'hev CLI exited unsuccessfully with a success response', '', cause)
  }
  if (prompt !== '') protocol('prompt must be empty for a success response')
  return response
}

function decodeEnvironment(value: BaseResponse['data']): Environment {
  const jsonPath = 'data.environment'
  const input = record(value, jsonPath)

  const id = environmentId(input.id, `${jsonPath}.id`)
  const name = nonEmptyString(input.name, `${jsonPath}.name`)
  if (!NAME.test(name)) protocol(`${jsonPath}.name must be lowercase kebab-case`)
  const revision = integer(input.revision, `${jsonPath}.revision`)
  if (revision < 1) protocol(`${jsonPath}.revision must be positive`)
  const rawSkills = array(input.skills, `${jsonPath}.skills`)
  const localSkills = new Set<string>()
  const skills = rawSkills.map((skill, index) => {
    const decoded = decodeSkill(skill, `${jsonPath}.skills[${String(index)}]`)
    if (localSkills.has(decoded.skillKey)) protocol(`${jsonPath}.skills contains duplicate skill key "${decoded.skillKey}"`)
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

function decodeEnvironmentSummary(value: BaseResponse['data'], jsonPath: string): EnvironmentSummary {
  const input = record(value, jsonPath)
  const id = environmentId(input.id, `${jsonPath}.id`)
  const name = nonEmptyString(input.name, `${jsonPath}.name`)
  if (!NAME.test(name)) protocol(`${jsonPath}.name must be lowercase kebab-case`)
  const revision = integer(input.revision, `${jsonPath}.revision`)
  if (revision < 1) protocol(`${jsonPath}.revision must be positive`)
  return Object.freeze({ id: EnvironmentId(id), name, revision })
}

function decodeSkill(value: BaseResponse['data'], jsonPath: string): EnvironmentSkillSpec {
  const input = record(value, jsonPath)
  const skillKey = nonEmptyString(input.skillKey, `${jsonPath}.skillKey`)
  if (!NAME.test(skillKey)) protocol(`${jsonPath}.skillKey must be lowercase kebab-case`)
  const policy = record(input.policy, `${jsonPath}.policy`)
  const kind = policy.kind
  if (kind !== 'auto' && kind !== 'off') protocol(`${jsonPath}.policy.kind must be "auto" or "off"`)
  return Object.freeze({ skillKey, policy: Object.freeze({ kind }) })
}

function record(value: BaseResponse['data'], jsonPath: string): Record<string, BaseResponse['data']> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) protocol(`${jsonPath} must be an object`)
  return value as Record<string, BaseResponse['data']>
}

function array(value: BaseResponse['data'], jsonPath: string): BaseResponse['data'][] {
  if (!Array.isArray(value)) protocol(`${jsonPath} must be an array`)
  return value
}

function string(value: BaseResponse['data'], jsonPath: string): string {
  if (typeof value !== 'string') protocol(`${jsonPath} must be a string`)
  return value
}

function nonEmptyString(value: BaseResponse['data'], jsonPath: string): string {
  const decoded = string(value, jsonPath)
  if (decoded.length === 0) protocol(`${jsonPath} must not be empty`)
  return decoded
}

function environmentId(value: BaseResponse['data'], jsonPath: string): string {
  const decoded = nonEmptyString(value, jsonPath)
  if (decoded.trim() !== decoded || /\s/u.test(decoded)) protocol(`${jsonPath} must not contain whitespace`)
  return decoded
}

function integer(value: BaseResponse['data'], jsonPath: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) protocol(`${jsonPath} must be a safe integer`)
  return value
}

function isCliFailureStatus(value: number): value is FailureStatusCode {
  return value === StatusCode.InvalidArgument
    || value === StatusCode.NotFound
    || value === StatusCode.Conflict
    || value === StatusCode.InternalError
}

function protocol(message: string): never {
  throw new HevCliError(StatusCode.ProtocolError, message)
}

function renderThrown(value: unknown): string {
  try {
    return String(value)
  } catch {
    return '<unrenderable CLI failure>'
  }
}

export type { NativeCommandRunner } from '@deepseek-ai/dsh-native-command'
