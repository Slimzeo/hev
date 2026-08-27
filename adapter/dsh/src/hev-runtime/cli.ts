/** Strict hev CLI v2 decoding over an injected native command runner.
 * @module @slimzeo/hev-dsh-plugin/hev-runtime/cli
 */

import { runNativeCommand } from '@deepseek-ai/dsh-native-command'
import type { NativeCommandRunner } from '@deepseek-ai/dsh-native-command'
import { EnvironmentId, StatusCode } from './environment.ts'
import type {
  AddedEnvironmentSkill,
  CreatedEnvironment,
  Environment,
  EnvironmentSession,
  EnvironmentSkillSpec,
  EnvironmentSummary,
  FailureStatusCode,
  Source,
  SkillPolicyKind,
} from './environment.ts'

const SCHEMA_VERSION = 2
const NAME = /^[a-z0-9]+(?:-[a-z0-9]+)*$/u
const SOURCE: Source = 'dsh'

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

  /** Resolve one host Session's current hev state.
   * @param sessionId - opaque host Session ID.
   * @param signal - operation cancellation signal.
   * @returns the latest validated Session state.
   */
  async current(sessionId: string, signal: AbortSignal): Promise<EnvironmentSession> {
    validateSessionId(sessionId)
    const response = await this.invoke(['env', 'status', '--session-id', sessionId], signal)
    return decodeSession(response.data.session, sessionId)
  }

  /** Select one Environment for a host Session through the CLI.
   * @param name - Environment ID or name.
   * @param sessionId - opaque host Session ID.
   * @param signal - operation cancellation signal.
   * @returns the committed Session state.
   */
  async use(
    name: string,
    sessionId: string,
    signal: AbortSignal,
  ): Promise<EnvironmentSession> {
    if (name.length === 0
      || name.trim() !== name
      || /\s/u.test(name)
      || name.startsWith('-')) {
      throw new HevCliError(
        StatusCode.InvalidArgument,
        'environment reference must be a non-empty argv value without whitespace',
        'Call hev_env_list, then retry with one existing Environment ID or lowercase kebab-case name.',
      )
    }
    validateSessionId(sessionId)
    const response = await this.invoke(['env', 'use', name, '--session-id', sessionId], signal)
    return decodeSession(response.data.session, sessionId)
  }

  /** Leave one Environment tier for a host Session.
   * @param sessionId - opaque host Session ID.
   * @param signal - operation cancellation signal.
   * @returns the committed Session state.
   */
  async quit(sessionId: string, signal: AbortSignal): Promise<EnvironmentSession> {
    validateSessionId(sessionId)
    const response = await this.invoke(['env', 'quit', '--session-id', sessionId], signal)
    return decodeSession(response.data.session, sessionId)
  }

  /** Create one empty Environment through the CLI.
   * @param name - lowercase kebab-case Environment name.
   * @param signal - operation cancellation signal.
   * @returns the CLI message and created Environment.
   */
  async create(name: string, signal: AbortSignal): Promise<CreatedEnvironment> {
    if (!NAME.test(name)) {
      throw new HevCliError(
        StatusCode.InvalidArgument,
        'environment name must be lowercase kebab-case',
        'Retry with a lowercase kebab-case Environment name such as "coding-tools".',
      )
    }
    const response = await this.invoke(['env', 'create', name], signal)
    return Object.freeze({
      message: response.message,
      environment: decodeEnvironment(response.data.environment),
    })
  }

  /** List all current Environments through the CLI.
   * @param signal - operation cancellation signal.
   * @returns validated Environment metadata ordered by the Core.
   */
  async listEnvironments(signal: AbortSignal): Promise<readonly EnvironmentSummary[]> {
    const response = await this.invoke(['env', 'list'], signal)
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
   * @param skillKey - logical Skill key.
   * @param environmentNames - target Environment names.
   * @param policy - Environment-owned Skill policy.
   * @param signal - operation cancellation signal.
   * @returns the committed binding and target summaries.
   */
  async addSkill(
    skillKey: string,
    environmentNames: readonly string[],
    policy: SkillPolicyKind,
    signal: AbortSignal,
  ): Promise<AddedEnvironmentSkill> {
    if (!NAME.test(skillKey)) {
      throw new HevCliError(
        StatusCode.InvalidArgument,
        'skill key must be lowercase kebab-case',
        'Call hev_skill_list with global=true, then retry with a listed lowercase kebab-case Skill key.',
      )
    }
    if (environmentNames.length === 0 || environmentNames.some(name => !NAME.test(name))) {
      throw new HevCliError(
        StatusCode.InvalidArgument,
        'at least one lowercase kebab-case environment name is required',
        'Call hev_env_list, then provide one or more listed Environment names.',
      )
    }
    if (new Set(environmentNames).size !== environmentNames.length) {
      throw new HevCliError(
        StatusCode.InvalidArgument,
        'environment names must not contain duplicates',
        'Retry with each target Environment listed only once.',
      )
    }
    const response = await this.invoke([
      'skill', 'add', skillKey, ...environmentNames, '--policy', policy,
    ], signal)
    const environmentSkill = decodeSkill(response.data.environmentSkill, 'data.environmentSkill')
    const environments = array(response.data.environments, 'data.environments')
    if (environments.length === 0) protocol('data.environments must not be empty')
    const seen = new Set<string>()
    const summaries = environments.map((value, index) => {
      const environment = decodeEnvironmentSummary(value, `data.environments[${String(index)}]`)
      if (seen.has(environment.id)) protocol(`duplicate environment id "${environment.id}"`)
      seen.add(environment.id)
      return environment
    })
    return Object.freeze({
      message: response.message,
      environmentSkill,
      environments: Object.freeze(summaries),
    })
  }

  private async invoke(args: readonly string[], signal: AbortSignal): Promise<BaseResponse> {
    let stdout: string
    try {
      stdout = (await this.runner(
        this.executable,
        ['--source', SOURCE, ...args, '--output', 'json'],
        signal,
      )).stdout
    } catch (error: unknown) {
      const candidate = (error as RunnerFailure | null)?.stdout ?? ''
      if (candidate.trim() === '') {
        throw new HevCliError(
          StatusCode.Unavailable,
          renderThrown(error),
          'Retry the hev operation. If it still fails, verify that the bundled hev executable can run.',
          error,
        )
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
    throw new HevCliError(
      StatusCode.ProtocolError,
      'hev CLI stdout is not valid JSON',
      'Update or reinstall the hev plugin so the adapter and Core use the same CLI protocol.',
      cause ?? error,
    )
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

function decodeEnvironment(value: BaseResponse['data'], jsonPath = 'data.environment'): Environment {
  const input = record(value, jsonPath)

  const source = decodeSource(input.source, `${jsonPath}.source`)
  if (source !== SOURCE) protocol(`${jsonPath}.source must be "${SOURCE}"`)
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
    source,
    id: EnvironmentId(id),
    name,
    revision,
    skills: Object.freeze(skills),
  })
}

function decodeSession(value: BaseResponse['data'], expectedSessionId: string): EnvironmentSession {
  const jsonPath = 'data.session'
  const input = record(value, jsonPath)
  const source = decodeSource(input.source, `${jsonPath}.source`)
  if (source !== SOURCE) protocol(`${jsonPath}.source must be "${SOURCE}"`)
  const sessionId = nonEmptyString(input.sessionId, `${jsonPath}.sessionId`)
  if (sessionId !== expectedSessionId) protocol(`${jsonPath}.sessionId does not match the requested Session`)
  const environment = input.environment === null
    ? null
    : decodeEnvironment(input.environment, `${jsonPath}.environment`)
  if (environment !== null && environment.source !== source) {
    protocol(`${jsonPath}.environment.source must match ${jsonPath}.source`)
  }
  return Object.freeze({ source, sessionId, environment })
}

function decodeEnvironmentSummary(value: BaseResponse['data'], jsonPath: string): EnvironmentSummary {
  const input = record(value, jsonPath)
  const source = decodeSource(input.source, `${jsonPath}.source`)
  if (source !== SOURCE) protocol(`${jsonPath}.source must be "${SOURCE}"`)
  const id = environmentId(input.id, `${jsonPath}.id`)
  const name = nonEmptyString(input.name, `${jsonPath}.name`)
  if (!NAME.test(name)) protocol(`${jsonPath}.name must be lowercase kebab-case`)
  const revision = integer(input.revision, `${jsonPath}.revision`)
  if (revision < 1) protocol(`${jsonPath}.revision must be positive`)
  return Object.freeze({ source, id: EnvironmentId(id), name, revision })
}

function decodeSource(value: BaseResponse['data'], jsonPath: string): Source {
  const source = string(value, jsonPath)
  if (source !== 'standalone' && source !== 'dsh' && source !== 'claude-code'
    && source !== 'codex' && source !== 'opencode') {
    protocol(`${jsonPath} is not a supported source`)
  }
  return source
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

function validateSessionId(sessionId: string): void {
  if (sessionId.length === 0) {
    throw new HevCliError(
      StatusCode.InvalidArgument,
      'session id must not be empty',
      'Retry from an active DSH Session.',
    )
  }
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
  throw new HevCliError(
    StatusCode.ProtocolError,
    message,
    'Update or reinstall the hev plugin so the adapter and Core use the same CLI protocol.',
  )
}

function renderThrown(value: unknown): string {
  try {
    return String(value)
  } catch {
    return '<unrenderable CLI failure>'
  }
}

export type { NativeCommandRunner } from '@deepseek-ai/dsh-native-command'
