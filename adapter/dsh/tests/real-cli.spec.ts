/**
 * The real `dsh` CLI, in a subprocess, with hev mounted as a profile plugin.
 *
 * This is the closest thing to a real conversation that runs without an API key:
 * the product binary boots its own `headless` profile, hev activates an
 * environment as the agent is created, the loop runs a real turn that loads an
 * env skill through the `skill` tool, and the assertions come from the session
 * log the app persisted on disk. Only the model's bytes are scripted
 * (`fixtures/hev-mock-llm.ts`), exactly as dsh's own keyless snapshots do.
 */
import { afterEach, describe, expect, it } from 'vitest'
import { copyFile, mkdir, mkdtemp, readdir, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { LOADER_SMOKE_TEST_TIMEOUT_MS, runLoaderSmoke } from '@deepseek-ai/dsh-loader-smoke'

const DSH = fileURLToPath(new URL('../../../../deepseek-harness/', import.meta.url))
const dshBin = join(DSH, 'apps/cli/src/bin.ts')
const dshTsconfig = join(DSH, 'tsconfig.json')
const pluginSource = fileURLToPath(new URL('../src/index.ts', import.meta.url))
const mockLlmSource = fileURLToPath(new URL('./fixtures/hev-mock-llm.ts', import.meta.url))
const fakeCli = fileURLToPath(new URL('./fixtures/fake-hev.mjs', import.meta.url))

let overlayDir: string | undefined

afterEach(async () => {
  if (overlayDir !== undefined) await rm(overlayDir, { recursive: true, force: true })
  overlayDir = undefined
})

/** The patch layer that turns the shipped `headless` profile into a keyless hev run. */
async function writeOverlay(): Promise<string> {
  overlayDir = await mkdtemp(join(tmpdir(), 'hev-overlay-'))
  const path = join(overlayDir, 'hev.cordis.yml')
  await writeFile(path, [
    '- id: agent-default-model',
    '  config:',
    '    provider: cli-mock',
    '    model: cli-mock',
    '',
    // The Typert gateway loads generated `lib/typert.host.js` contributors, which
    // exist only after `pnpm run build`. A one-shot run serves no Remote
    // endpoints, so the row is disabled to keep this test buildless. (dsh's own
    // headless snapshots hit the same prerequisite on an unbuilt checkout.)
    '- id: typert-loader',
    '  disabled: true',
    '',
    '- insert:',
    '    - id: hev-mock-llm',
    "      name: './snapshot-fixtures/hev-mock-llm.ts'",
    '    - id: hev',
    "      name: './snapshot-fixtures/hev-plugin.ts'",
    '      config:',
    `        cli: '${fakeCli}'`,
    '        activate:',
    '          - coding',
    '',
  ].join('\n'))
  return path
}

async function writeSkill(dir: string, name: string, description: string, body: string): Promise<string> {
  await mkdir(dir, { recursive: true })
  await writeFile(join(dir, 'SKILL.md'), `---\nname: ${name}\ndescription: ${description}\n---\n\n${body}\n`)
  return dir
}

/** Install hev, its keyless model, its env skills, and the fake CLI's storage into the run's cwd. */
async function prepare(cwd: string): Promise<void> {
  const fixtureDir = join(cwd, '.dsh', 'profiles', 'headless', 'snapshot-fixtures')
  await mkdir(fixtureDir, { recursive: true })
  await Promise.all([
    copyFile(pluginSource, join(fixtureDir, 'hev-plugin.ts')),
    copyFile(mockLlmSource, join(fixtureDir, 'hev-mock-llm.ts')),
    writeFile(join(fixtureDir, 'package.json'), '{"type":"module"}\n'),
  ])

  const store = join(cwd, 'hev-skills')
  const codeReview = await writeSkill(join(store, 'code-review'), 'code-review', 'Review a diff', 'Code review body.')
  const secret = await writeSkill(join(store, 'secret-skill'), 'secret-skill', 'Withheld from the model', 'Secret body.')
  const mismatched = await writeSkill(join(store, 'actual-name'), 'actual-name', 'Mismatched', 'Mismatched body.')

  await writeFile(join(cwd, 'hev-db.json'), JSON.stringify({
    environments: {
      base: { id: 'env_base', revision: 1, skills: [] },
      coding: {
        id: 'env_01',
        revision: 4,
        skills: [
          { id: 'skill_01', skillName: 'code-review', realPath: codeReview },
          { id: 'skill_02', skillName: 'secret-skill', realPath: secret, mode: { type: 'off' } },
          { id: 'skill_03', skillName: 'renamed-skill', realPath: mismatched },
        ],
      },
    },
  }))
}

interface LoggedEvent {
  type?: string
  data?: Record<string, unknown>
}

/** Read the app's own persisted session log from the run's isolated DSH home. */
async function persistedEvents(cwd: string): Promise<LoggedEvent[]> {
  const home = join(cwd, '.dsh')
  const logs = (await readdir(home, { recursive: true }))
    .filter(file => file.endsWith('.jsonl') && file.includes('session'))
  expect(logs, `no persisted session log under ${home}`).toHaveLength(1)
  const content = await readFile(join(home, logs[0] as string), 'utf8')
  return content.split('\n').filter(line => line.trim().length > 0).map(line => JSON.parse(line) as LoggedEvent)
}

describe('real dsh CLI with hev mounted', () => {
  it('activates an env at agent creation, runs a real turn, and leaves the decisions in the persisted log', async () => {
    const overlay = await writeOverlay()
    const task = 'Load the review skill and report what it says.'
    const result = await runLoaderSmoke({
      label: 'hev headless profile run',
      tempDirPrefix: 'hev-real-cli-',
      binScript: dshBin,
      configPath: overlay,
      binArgs: ['--profile', 'headless', '--patch', overlay, task],
      tsconfigPath: dshTsconfig,
      processTimeoutMs: 25_000,
      env: {
        FAKE_HEV_DB: 'hev-db.json',
        DSH_PERMISSION_MODE: 'danger-full-access',
        DSH_TELEMETRY_DISABLED: '1',
        NODE_OPTIONS: [process.env.NODE_OPTIONS, '--disable-warning=ExperimentalWarning'].filter(Boolean).join(' '),
      },
      prepare,
      inspect: async (cwd) => {
        const events = await persistedEvents(cwd)

        // 1. hev's decision set, recorded by the command it dispatched at startup.
        const decisionLines = events
          .filter(event => event.type === 'command/done')
          .flatMap(event => String((event.data as { text?: string } | undefined)?.text ?? '').split('\n'))
          .filter(line => line.startsWith('hev '))
        expect(decisionLines).toEqual([
          'hev env: coding@4 — 1 model-visible, 1 user-only, 1 excluded',
          'hev skill admitted: code-review (env=coding mode=auto)',
          'hev skill user-only: secret-skill (env=coding mode=off)',
          'hev skill excluded: renamed-skill (env=coding mode=auto) — renamed-skill: frontmatter declares name "actual-name" — the registry and the file must agree',
        ])

        // 2. The catalog the model actually received.
        const catalogs = events.filter(event =>
          event.type === 'user/message'
          && (event.data as { source?: { kind?: string } } | undefined)?.source?.kind === 'skill-catalog')
        const offered = ((catalogs.at(-1)?.data as { source?: { entries?: { name: string }[] } } | undefined)
          ?.source?.entries ?? []).map(entry => entry.name)
        expect(offered).toEqual(['code-review'])

        // 3. The skill the model loaded, through the product's own tool path.
        const skillCalls = events
          .filter(event => event.type === 'tool/call' && (event.data as { name?: string } | undefined)?.name === 'skill')
          .map(event => JSON.parse(String((event.data as { arguments?: string } | undefined)?.arguments ?? '{}')) as { name?: string })
        expect(skillCalls.map(call => call.name)).toEqual(['code-review'])

        // 4. The withheld skill never appears anywhere the model could read.
        const modelVisible = JSON.stringify(events.filter(event =>
          event.type === 'user/message' || event.type === 'tool/result' || event.type === 'assistant/message'))
        expect(modelVisible).not.toContain('secret-skill')
        expect(modelVisible).not.toContain('Secret body.')
      },
    })

    // The env skill's body made the full round trip through the real binary.
    expect(result.stdout).toBe('HEV_OK: Code review body.\n')
    expect(result.stderr).toBe('')
  }, 180_000)
})
