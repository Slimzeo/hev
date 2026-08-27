/** hev-filtered implementation of the native DSH Skill Registry.
 * @module @slimzeo/hev-dsh-plugin/hev-skill-registry
 */

import type { Context } from '@deepseek-ai/cordis'
import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import type {} from '@deepseek-ai/dsh-agent'
import SkillRegistry from '@deepseek-ai/dsh-skill'
import { BUNDLED_SKILL_RANK } from '@deepseek-ai/dsh-skill'
import type {
  Config,
  SkillCandidate,
  SkillCatalogSnapshot,
  SkillDefinition,
  SkillProvider,
  SkillSummary,
  SkillViewOptions,
} from '@deepseek-ai/dsh-skill'
import type { Environment } from '../hev-runtime/index.ts'

const hevGuideName = 'hev-guide'
const hevGuideDescription = 'Guide users through hev skill environments, distinguish the current environment from all DSH-discoverable Skills, find suitable Skills, and add them to the active environment. Use when the user asks what hev is, which environment or Skills are active, why a discovered Skill is unavailable, how to find or enable a Skill, or how to configure the hev plugin.'
const hevGuideUrl = new URL('../../skills/hev-guide/SKILL.md', import.meta.url)
const hevGuideResourceBase = {
  kind: 'directory',
  path: fileURLToPath(new URL('../../skills/hev-guide/', import.meta.url)),
} as const
const hevGuideCandidate: SkillCandidate = {
  name: hevGuideName,
  description: hevGuideDescription,
  invocation: { modelInvocable: true, userInvocable: true },
  provider: hevGuideName,
  source: 'bundled',
  resourceBase: hevGuideResourceBase,
  rank: BUNDLED_SKILL_RANK,
  locator: hevGuideUrl,
}

const hevGuideProvider: SkillProvider = {
  name: hevGuideName,
  list: () => Promise.resolve([hevGuideCandidate]),
  async get(): Promise<SkillDefinition> {
    return {
      ...hevGuideCandidate,
      content: skillBody(await readFile(hevGuideUrl, 'utf8')),
    }
  },
}

/** Native DSH Skill Registry with hev Environment filtering applied to catalog reads. */
export class HevSkillRegistry extends SkillRegistry {
  static inject = ['agents', 'environment']

  /**
   * Create the replacement Registry while retaining native Registry configuration.
   * @param ctx - Cordis context carrying the Agent registry and hev runtime.
   * @param config - native Skill Registry configuration.
   */
  constructor(ctx: Context, config: Config = {}) {
    super(ctx, config)
    this.registerProvider(() => hevGuideProvider)
  }

  /**
   * Return all native winners for one DSH view without applying an Environment.
   * @param options - native lookup options, including the calling Agent scope.
   * @returns the unfiltered native catalog.
   */
  async listAll(options: SkillViewOptions = {}): Promise<SkillSummary[]> {
    return (await super.snapshot(options)).skills
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

  // Keep the native get(): the model-facing skill Tool checks this filtered
  // catalog first, while DSH's explicit /skill-name path intentionally does not.

  /** Resolve the active Environment's model-discoverable Skill names. */
  private async allowedSkillNames(options: SkillViewOptions): Promise<ReadonlySet<string> | undefined> {
    const agent = options.scope === undefined
      ? undefined
      : this.ctx.agents.list().find(candidate => candidate === options.scope)
    if (agent === undefined) return undefined
    const environment = await this.ctx.environment.current(agent.session, options.signal)
    if (environment === undefined) return undefined
    return autoSkillNames(environment)
  }
}

function autoSkillNames(environment: Environment): ReadonlySet<string> {
  return new Set(environment.skills
    .filter(skill => skill.policy.kind === 'auto')
    .map(skill => skill.skillKey))
}

function skillBody(document: string): string {
  const match = /^---\r?\n[\s\S]*?\r?\n---\r?\n([\s\S]*)$/u.exec(document)
  if (match?.[1] === undefined) throw new Error('bundled hev-guide SKILL.md has invalid frontmatter')
  return match[1].trim()
}

export default HevSkillRegistry
