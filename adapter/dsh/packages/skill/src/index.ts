/** HEV-filtered implementation of the native DSH Skill Registry.
 * @module @hev/dsh-skill
 */

import type { Context } from '@deepseek-ai/cordis'
import type {} from '@deepseek-ai/dsh-agent'
import SkillRegistry from '@deepseek-ai/dsh-skill'
import type {
  Config,
  SkillCatalogSnapshot,
  SkillDefinition,
  SkillViewOptions,
} from '@deepseek-ai/dsh-skill'
import type { ResolvedEnvironmentSnapshot } from '@hev/dsh-runtime'

/** Native DSH Skill Registry with HEV Environment visibility applied at reads. */
export class HevSkillRegistry extends SkillRegistry {
  static inject = ['agents', 'environment']

  /**
   * Create the replacement Registry while retaining native Registry configuration.
   * @param ctx - Cordis context carrying the Agent registry and HEV runtime.
   * @param config - native Skill Registry configuration.
   */
  constructor(ctx: Context, config: Config = {}) {
    super(ctx, config)
  }

  /**
   * Return native winners visible in the calling Agent's current Environment.
   * @param options - native lookup options; an exact live Agent may be supplied as the scope.
   * @returns the filtered catalog, or the native catalog when no exact live Agent is supplied.
   */
  override async snapshot(options: SkillViewOptions = {}): Promise<SkillCatalogSnapshot> {
    const catalog = await super.snapshot(options)
    const allowed = await this.allowedSkillNames(options)
    if (allowed === undefined) return catalog
    return { ...catalog, skills: catalog.skills.filter(skill => allowed.has(skill.name)) }
  }

  /**
   * Load a native winner only when the calling Agent's current Environment allows it.
   * @param name - native Skill name.
   * @param options - native lookup options; an exact live Agent may be supplied as the scope.
   * @returns the native definition when visible, otherwise `undefined`.
   */
  override async get(
    name: string,
    options: SkillViewOptions = {},
  ): Promise<SkillDefinition | undefined> {
    const allowed = await this.allowedSkillNames(options)
    if (allowed !== undefined && !allowed.has(name)) return undefined
    return await super.get(name, options)
  }

  private async allowedSkillNames(options: SkillViewOptions): Promise<ReadonlySet<string> | undefined> {
    const agent = options.scope === undefined
      ? undefined
      : this.ctx.agents.list().find(candidate => candidate === options.scope)
    if (agent === undefined) return undefined
    const snapshot = await this.ctx.environment.current(agent.session, options.signal)
    return autoSkillNames(snapshot)
  }
}

function autoSkillNames(snapshot: ResolvedEnvironmentSnapshot): ReadonlySet<string> {
  return new Set(snapshot.environments.flatMap(environment => environment.skills
    .filter(skill => skill.policy.kind === 'auto')
    .map(skill => skill.skillKey)))
}

export default HevSkillRegistry
