/** Public Environment data returned by HEV CLI response schema v1.
 * @module @hev/dsh-runtime/types
 */

import type { Branded } from '@deepseek-ai/dsh-brand'

/** Stable HEV Environment identity. */
export type EnvironmentId = Branded<'EnvironmentId'>

/** Brand a validated CLI string as an {@link EnvironmentId}.
 * @param value - validated non-empty Environment ID.
 * @returns the same string with its compile-time brand.
 */
export function EnvironmentId(value: string): EnvironmentId {
  return value as EnvironmentId
}

/** Skill policies supported by the HEV MVP. */
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

/** Latest Environment record returned by the HEV store. */
export interface Environment {
  readonly id: EnvironmentId
  readonly name: string
  readonly revision: number
  readonly skills: readonly EnvironmentSkillSpec[]
}

/** Ordered, validated Environment group resolved by HEV. */
export interface ResolvedEnvironmentSnapshot {
  readonly environments: readonly Environment[]
}
