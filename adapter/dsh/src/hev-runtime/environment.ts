/** Public Environment data returned by hev CLI response schema v2.
 * @module @slimzeo/hev-dsh-plugin/hev-runtime/environment
 */

import type { Branded } from '@deepseek-ai/dsh-brand'

/** Coding Agent platforms supported by hev Core. */
export type Source = 'standalone' | 'dsh' | 'claude-code' | 'codex' | 'opencode'

/** Numeric statuses used by the hev CLI protocol and adapter-local failures. */
export const StatusCode = Object.freeze({
  Ok: 200,
  InvalidArgument: 400,
  NotFound: 404,
  Conflict: 409,
  InternalError: 500,
  ProtocolError: 502,
  Unavailable: 503,
} as const)

/** Numeric status carried by a hev CLI or adapter failure. */
export type FailureStatusCode = Exclude<
  typeof StatusCode[keyof typeof StatusCode],
  typeof StatusCode.Ok
>

/** Stable hev Environment identity. */
export type EnvironmentId = Branded<'EnvironmentId'>

/** Brand a validated CLI string as an {@link EnvironmentId}.
 * @param value - validated non-empty Environment ID.
 * @returns the same string with its compile-time brand.
 */
export function EnvironmentId(value: string): EnvironmentId {
  return value as EnvironmentId
}

/** Skill policies supported by the hev MVP. */
export type SkillPolicyKind = 'auto' | 'off'

/** Environment-owned policy for one Skill. */
export interface EnvironmentSkillPolicy {
  readonly kind: SkillPolicyKind
}

/** One Skill binding in an Environment. */
export interface EnvironmentSkillSpec {
  readonly skillKey: string
  readonly policy: EnvironmentSkillPolicy
}

/** Latest Environment record returned by the hev store. */
export interface Environment {
  readonly source: Source
  readonly id: EnvironmentId
  readonly name: string
  readonly revision: number
  readonly skills: readonly EnvironmentSkillSpec[]
}

/** Resolved hev state for one host Session. */
export interface EnvironmentSession {
  readonly source: Source
  readonly sessionId: string
  readonly environment: Environment | null
}

/** Current Environment metadata returned by `hev env list`. */
export interface EnvironmentSummary {
  readonly source: Source
  readonly id: EnvironmentId
  readonly name: string
  readonly revision: number
}

/** Result of adding one Skill binding to one or more Environments. */
export interface AddedEnvironmentSkill {
  readonly message: string
  readonly environmentSkill: EnvironmentSkillSpec
  readonly environments: readonly EnvironmentSummary[]
}

/** Result of removing one Skill binding from one or more Environments. */
export interface RemovedEnvironmentSkill {
  readonly message: string
  readonly skillKey: string
  readonly environments: readonly EnvironmentSummary[]
}

/** Result of creating, renaming, or deleting one Environment. */
export interface EnvironmentResult {
  readonly message: string
  readonly environment: Environment
}

