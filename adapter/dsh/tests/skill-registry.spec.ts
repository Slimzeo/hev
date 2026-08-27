import { Context } from '@deepseek-ai/cordis'
import AgentRegistry, { Inbox } from '@deepseek-ai/dsh-agent'
import type { Agent } from '@deepseek-ai/dsh-agent'
import * as SkillFileSystem from '@deepseek-ai/dsh-skill-filesystem'
import { Session, SessionId } from '@deepseek-ai/dsh-session'
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it, vi } from 'vitest'
import { EnvironmentId } from '../src/hev-runtime/index.ts'
import type { Environment } from '../src/hev-runtime/index.ts'
import HevSkillRegistry from '../src/hev-skill-registry/index.ts'

function sessionAgent(id: string): Agent {
  const sessionId = SessionId(id)
  const session = Session.create(sessionId)
  return {
    id: sessionId,
    options: {},
    session,
    inbox: new Inbox(session, { inserted: () => {}, discarded: () => {}, claimed: () => {} }),
    status: 'idle',
    ctx: new Context(),
    send: () => {},
    followup: () => {},
    steer: () => {},
    inject: () => {},
    cancel: () => {},
    runMaintenance: task => task(new AbortController().signal),
    whenIdle: () => Promise.resolve(),
  }
}

function environment(skills: Array<{ skillKey: string; kind: 'auto' | 'off' }>): Environment {
  return {
    source: 'dsh',
    id: EnvironmentId('env-coding'),
    name: 'coding',
    revision: 1,
    skills: skills.map(skill => ({
      skillKey: skill.skillKey,
      policy: { kind: skill.kind },
    })),
  }
}

async function world(
  current: (session: Session, signal?: AbortSignal) => Promise<Environment | undefined>,
) {
  const ctx = new Context()
  await ctx.plugin(AgentRegistry)
  const currentSpy = vi.fn(current)
  ctx.provide('environment', { current: currentSpy })
  await ctx.plugin(HevSkillRegistry)
  for (const name of ['allowed-skill', 'off-skill', 'absent-skill']) {
    ctx.skills.register({
      name,
      description: `${name} description`,
      source: 'runtime',
      content: `${name} body`,
    })
  }
  return { ctx, current: currentSpy }
}

describe('@slimzeo/hev-dsh-plugin/hev-skill-registry', () => {
  it('filters active Sessions and leaves inactive Sessions unfiltered', async () => {
    const active = sessionAgent('active')
    const inactive = sessionAgent('inactive')
    let selected = environment([
      { skillKey: 'allowed-skill', kind: 'auto' },
      { skillKey: 'off-skill', kind: 'off' },
    ])
    const { ctx, current } = await world(async session => session === active.session ? selected : undefined)
    ctx.agents.register(active)
    ctx.agents.register(inactive)

    expect((await ctx.skills.list()).map(skill => skill.name)).toEqual([
      'absent-skill',
      'allowed-skill',
      'hev-guide',
      'off-skill',
    ])
    expect((await ctx.skills.list({ scope: inactive })).map(skill => skill.name)).toEqual([
      'absent-skill',
      'allowed-skill',
      'hev-guide',
      'off-skill',
    ])
    expect((await ctx.skills.snapshot({ scope: inactive })).skills.map(skill => skill.name)).toEqual([
      'absent-skill',
      'allowed-skill',
      'hev-guide',
      'off-skill',
    ])
    expect(await ctx.skills.get('absent-skill', { scope: inactive })).toMatchObject({ name: 'absent-skill' })

    const signal = new AbortController().signal
    const view = { scope: active, signal }
    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(['allowed-skill'])
    expect((await ctx.skills.snapshot(view)).skills.map(skill => skill.name)).toEqual(['allowed-skill'])
    expect(await ctx.skills.get('off-skill', view)).toBeUndefined()
    expect(await ctx.skills.get('absent-skill', view)).toBeUndefined()
    expect(await ctx.skills.get('allowed-skill', view)).toMatchObject({ name: 'allowed-skill' })
    expect((await ctx.skills.listAll(view)).map(skill => skill.name)).toEqual([
      'absent-skill',
      'allowed-skill',
      'hev-guide',
      'off-skill',
    ])

    selected = environment([{ skillKey: 'off-skill', kind: 'auto' }])
    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(['off-skill'])
    expect(current).toHaveBeenLastCalledWith(active.session, signal)
  })

  it('ships a loadable Hev guide', async () => {
    const active = sessionAgent('guide')
    const { ctx } = await world(async () => environment([
      { skillKey: 'hev-guide', kind: 'auto' },
    ]))
    ctx.agents.register(active)

    await expect(ctx.skills.get('hev-guide', { scope: active })).resolves.toMatchObject({
      name: 'hev-guide',
      source: 'bundled',
      content: expect.stringContaining('<project>/.dsh/skills'),
    })
    const guide = await ctx.skills.get('hev-guide', { scope: active })
    expect(guide?.content).toContain('<project>/.agents/skills')
    expect(guide?.content).toContain('customSkillDirs')
    expect(guide?.content).toContain('${DSH_HOME:-~/.dsh}/skills')
    expect(guide?.content).toContain('${DSH_AGENTS_HOME:-~/.agents}/skills')
    expect(guide?.content).toContain('bundledSkillDir')
  })

  it('retains native discovery across every filesystem source before Environment filtering', async () => {
    const root = await mkdtemp(join(tmpdir(), 'hev-skill-sources-'))
    const project = join(root, 'project')
    const custom = join(root, 'custom')
    const dshHome = join(root, 'dsh-home')
    const agentsHome = join(root, 'agents-home')
    const bundled = join(root, 'bundled')
    const roots = [
      { path: join(project, '.dsh/skills'), name: 'project-dsh', source: 'project-dsh' },
      { path: join(project, '.agents/skills'), name: 'project-agents', source: 'project-agents' },
      { path: custom, name: 'custom', source: 'custom' },
      { path: join(dshHome, 'skills'), name: 'user-dsh', source: 'user-dsh' },
      { path: join(agentsHome, 'skills'), name: 'user-agents', source: 'user-agents' },
      { path: bundled, name: 'bundled', source: 'bundled' },
    ] as const
    const ctx = new Context()
    try {
      await mkdir(join(project, '.git'), { recursive: true })
      for (const rootEntry of roots) {
        const directory = join(rootEntry.path, rootEntry.name)
        await mkdir(directory, { recursive: true })
        await writeFile(join(directory, 'SKILL.md'), [
          '---',
          `name: ${rootEntry.name}`,
          `description: Skill from ${rootEntry.source}.`,
          '---',
          '',
          `Use ${rootEntry.name}.`,
          '',
        ].join('\n'))
      }

      await ctx.plugin(AgentRegistry)
      const active = sessionAgent('all-sources')
      ctx.provide('environment', {
        current: async () => environment([
          ...roots.map(rootEntry => ({ skillKey: rootEntry.name, kind: 'auto' as const })),
          { skillKey: 'hev-guide', kind: 'auto' as const },
        ]),
      })
      await ctx.plugin(HevSkillRegistry)
      await ctx.plugin(SkillFileSystem, {
        dshHome,
        agentsHome,
        customSkillDirs: [custom],
        bundledSkillDir: bundled,
        watch: false,
      })
      ctx.agents.register(active)

      const skills = await ctx.skills.list({ scope: active, cwd: project })
      expect(skills.map(skill => [skill.name, skill.source])).toEqual([
        ['bundled', 'bundled'],
        ['custom', 'custom'],
        ['hev-guide', 'bundled'],
        ['project-agents', 'project-agents'],
        ['project-dsh', 'project-dsh'],
        ['user-agents', 'user-agents'],
        ['user-dsh', 'user-dsh'],
      ])
    } finally {
      await ctx.fiber.dispose()
      await rm(root, { recursive: true, force: true })
    }
  })

  it('still keeps native views without a registered exact Agent scope', async () => {
    const { ctx, current } = await world(async () => environment([{ skillKey: 'allowed-skill', kind: 'auto' }]))

    expect((await ctx.skills.list({ scope: {} })).map(skill => skill.name)).toEqual([
      'absent-skill',
      'allowed-skill',
      'hev-guide',
      'off-skill',
    ])
    expect(current).not.toHaveBeenCalled()
  })
})
