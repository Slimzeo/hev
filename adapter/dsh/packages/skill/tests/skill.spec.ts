import { Context } from '@deepseek-ai/cordis'
import AgentRegistry, { Inbox } from '@deepseek-ai/dsh-agent'
import type { Agent } from '@deepseek-ai/dsh-agent'
import { Session, SessionId } from '@deepseek-ai/dsh-session'
import { describe, expect, it, vi } from 'vitest'
import type { ResolvedEnvironmentSnapshot } from '@hev/dsh-runtime'
import HevSkillRegistry from '../src/index.ts'

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

function snapshot(skills: Array<{ skillKey: string; kind: 'auto' | 'off' }>): ResolvedEnvironmentSnapshot {
  return {
    environments: [{
      id: 'env-coding',
      name: 'coding',
      revision: 1,
      skills: skills.map(skill => ({
        skillKey: skill.skillKey,
        policy: { kind: skill.kind },
      })),
    }],
  }
}

async function world(
  current: (session: Session, signal?: AbortSignal) => Promise<ResolvedEnvironmentSnapshot>,
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

describe('@hev/dsh-skill', () => {
  it('keeps native views without an exact Agent and filters all reads through the current Environment snapshot', async () => {
    const active = sessionAgent('active')
    const inactive = sessionAgent('inactive')
    const base = snapshot([])
    let selected = snapshot([
      { skillKey: 'allowed-skill', kind: 'auto' },
      { skillKey: 'off-skill', kind: 'off' },
    ])
    const { ctx, current } = await world(async session => session === active.session ? selected : base)
    ctx.agents.register(active)
    ctx.agents.register(inactive)

    expect((await ctx.skills.list()).map(skill => skill.name)).toEqual([
      'absent-skill',
      'allowed-skill',
      'off-skill',
    ])
    expect((await ctx.skills.list({ scope: inactive })).map(skill => skill.name)).toEqual([])

    expect((await ctx.skills.snapshot({ scope: inactive })).skills).toEqual([])

    const signal = new AbortController().signal
    const view = { scope: active, signal }
    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(['allowed-skill'])
    expect((await ctx.skills.snapshot(view)).skills.map(skill => skill.name)).toEqual(['allowed-skill'])
    expect(await ctx.skills.get('off-skill', view)).toBeUndefined()
    expect(await ctx.skills.get('absent-skill', view)).toBeUndefined()
    expect(await ctx.skills.get('allowed-skill', view)).toMatchObject({ name: 'allowed-skill' })

    selected = snapshot([{ skillKey: 'off-skill', kind: 'auto' }])
    expect((await ctx.skills.list(view)).map(skill => skill.name)).toEqual(['off-skill'])
    expect(current).toHaveBeenLastCalledWith(active.session, signal)
  })

  it('still keeps native views without a registered exact Agent scope', async () => {
    const { ctx, current } = await world(async () => snapshot([{ skillKey: 'allowed-skill', kind: 'auto' }]))

    expect((await ctx.skills.list({ scope: {} })).map(skill => skill.name)).toEqual([
      'absent-skill',
      'allowed-skill',
      'off-skill',
    ])
    expect(current).not.toHaveBeenCalled()
  })
})
