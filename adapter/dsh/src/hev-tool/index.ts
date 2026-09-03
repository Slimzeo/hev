/** Model-facing hev Environment Workspace tools for DeepSeek Harness.
 * @module @owariband/hev-dsh-plugin/hev-tool
 */

import type { Context } from '@deepseek-ai/cordis'
import type { Agent } from '@deepseek-ai/dsh-agent'
import { defineTool } from '@deepseek-ai/dsh-tools'
import { HevCliError, StatusCode } from '../hev-runtime/index.ts'
import type { Environment, EnvironmentSummary, SkillPolicyKind } from '../hev-runtime/index.ts'
import type {} from '../hev-runtime/index.ts'

export const name = 'hev-tool'
export const inject = ['tools', 'environment']

const TEXT_OUTPUT = {
  schema: { type: 'string' as const },
  render: (_args: unknown, value: string) => [{ type: 'text' as const, text: value }],
}

/** Register Agent-facing Environment Workspace tools. */
export function apply(ctx: Context): void {
  ctx.tools.register(defineTool({
    name: 'hev_env_status',
    description: "Read the current Session's hev Environment. Call this before Environment-scoped operations; an inactive result means native DSH Skill visibility is unchanged.",
    parameters: {},
    output: TEXT_OUTPUT,
    execute: async (_args, exec) => executeWithPrompt(ctx, async () => renderEnvironment(
      await ctx.environment.current(requireAgent(exec.agent).session, exec.signal),
    )),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_env_use',
    description: 'Select exactly one existing hev Environment for the current Session. Call hev_env_list first when the Environment is unknown.',
    parameters: {
      environment: {
        type: 'string',
        required: true,
        description: 'Existing Environment ID or lowercase kebab-case name.',
      },
    },
    output: TEXT_OUTPUT,
    execute: async (args, exec) => executeWithPrompt(ctx, async () => renderEnvironment(
      await ctx.environment.use(requireAgent(exec.agent), args.environment, exec.signal),
    )),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_env_quit',
    description: 'Leave one hev Environment tier for the current calling Session: non-base becomes base, and base becomes inactive.',
    parameters: {},
    output: TEXT_OUTPUT,
    execute: async (_args, exec) => executeWithPrompt(ctx, async () => renderEnvironment(
      await ctx.environment.quit(requireAgent(exec.agent), exec.signal),
    )),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_env_create',
    description: 'Create a new hev Environment in this DSH installation. Creation adds hev-guide but does not activate the Environment.',
    parameters: {
      name: { type: 'string', required: true, description: 'Lowercase kebab-case Environment name.' },
    },
    output: TEXT_OUTPUT,
    execute: async (args, exec) => executeWithPrompt(ctx, async () => {
      requireAgent(exec.agent)
      return renderEnvironment((await ctx.environment.create(args.name, exec.signal)).environment)
    }),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_env_rename',
    description: 'Rename one non-base hev Environment without changing its stable ID or current Session bindings.',
    parameters: {
      environment: { type: 'string', required: true, description: 'Existing Environment ID or name.' },
      name: { type: 'string', required: true, description: 'New lowercase kebab-case Environment name.' },
    },
    output: TEXT_OUTPUT,
    execute: async (args, exec) => executeWithPrompt(ctx, async () => {
      requireAgent(exec.agent)
      return renderEnvironment((await ctx.environment.rename(args.environment, args.name, exec.signal)).environment)
    }),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_env_delete',
    description: 'Delete one non-base hev Environment. Sessions still bound to it resolve to base on their next hev operation.',
    parameters: {
      environment: { type: 'string', required: true, description: 'Existing non-base Environment ID or name.' },
    },
    output: TEXT_OUTPUT,
    execute: async (args, exec) => executeWithPrompt(ctx, async () => {
      requireAgent(exec.agent)
      const result = await ctx.environment.deleteEnvironment(args.environment, exec.signal)
      return `deleted ${result.environment.name} (${result.environment.id})`
    }),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_env_list',
    description: 'List Environments stored for DSH. Use the returned ID or name with hev_env_use; this does not report the current Session selection.',
    parameters: {},
    output: TEXT_OUTPUT,
    execute: async (_args, exec) => executeWithPrompt(ctx, async () => {
      requireAgent(exec.agent)
      return renderEnvironmentList(await ctx.environment.list(exec.signal))
    }),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_skill_add',
    description: 'Bind one native Skill key to existing Environments. This does not install the Skill or activate an Environment; call hev_skill_list with global=true and hev_env_list first when values are unknown.',
    parameters: {
      skill: { type: 'string', required: true, description: 'Native Skill key to bind.' },
      environments: {
        type: 'array',
        required: true,
        description: 'One or more existing Environment names, with no duplicates.',
        items: { type: 'string' },
      },
      policy: {
        type: 'string',
        enum: ['auto', 'off'],
        description: 'auto includes the Skill in automatic model discovery; off excludes it from that catalog. Defaults to auto.',
      },
    },
    output: TEXT_OUTPUT,
    execute: async (args, exec) => executeWithPrompt(ctx, async () => {
      requireAgent(exec.agent)
      const result = await ctx.environment.addSkill(
        args.skill,
        args.environments,
        (args.policy ?? 'auto') as SkillPolicyKind,
        exec.signal,
      )
      return `added ${result.environmentSkill.skillKey} to ${result.environments.map(value => value.name).join(', ')}`
    }),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_skill_list',
    description: "List all bindings in one named or current Session Environment, including off entries. Set global=true to list native DSH-discoverable Skills before hev filtering.",
    parameters: {
      global: { type: 'boolean', description: 'List the unfiltered native DSH Skill catalog.' },
      environment: { type: 'string', description: 'Inspect this Environment ID or name without selecting it.' },
    },
    output: TEXT_OUTPUT,
    execute: async (args, exec) => executeWithPrompt(ctx, async () => {
      const agent = requireAgent(exec.agent)
      if (args.global === true && args.environment !== undefined) {
        throw new HevCliError(
          StatusCode.InvalidArgument,
          'global and environment cannot be used together',
          'Set either global=true or environment, not both.',
        )
      }
      if (args.global === true) {
        const skills = await ctx.environment.listGlobalSkills(agent, exec.signal)
        return skills.length === 0
          ? 'global: no skills available'
          : ['global:', ...skills.map(skill => `- ${skill.name}`)].join('\n')
      }
      if (args.environment !== undefined) {
        return renderEnvironmentSkills(await ctx.environment.listEnvironmentSkills(args.environment, exec.signal))
      }
      return renderEnvironmentSkills(await ctx.environment.current(agent.session, exec.signal))
    }),
  }))

  ctx.tools.register(defineTool({
    name: 'hev_skill_remove',
    description: 'Remove one existing Skill binding from one or more Environments atomically.',
    parameters: {
      skill: { type: 'string', required: true, description: 'Bound Skill key to remove.' },
      environments: {
        type: 'array',
        required: true,
        description: 'One or more existing Environment names, with no duplicates.',
        items: { type: 'string' },
      },
    },
    output: TEXT_OUTPUT,
    execute: async (args, exec) => executeWithPrompt(ctx, async () => {
      requireAgent(exec.agent)
      const result = await ctx.environment.removeSkill(args.skill, args.environments, exec.signal)
      return `removed ${result.skillKey} from ${result.environments.map(value => value.name).join(', ')}`
    }),
  }))
}

async function executeWithPrompt<T>(ctx: Context, operation: () => Promise<T>): Promise<T> {
  try {
    return await operation()
  } catch (error: unknown) {
    if (!(error instanceof HevCliError)) throw error
    ctx.logger.warn(`hev tool failed (${String(error.statusCode)}): ${error.message}`)
    throw new Error(
      error.prompt === '' ? 'Retry the hev operation. If it still fails, inspect the DSH logs.' : error.prompt,
      { cause: error },
    )
  }
}

function requireAgent(agent: Agent | undefined): Agent {
  if (agent === undefined) throw new Error('hev tools require a calling Agent')
  return agent
}

function renderEnvironment(environment: Environment | undefined): string {
  return environment === undefined
    ? 'hev not activated'
    : `${environment.name} (${environment.id} rev ${String(environment.revision)})`
}

function renderEnvironmentList(environments: readonly EnvironmentSummary[]): string {
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
