/**
 * Vitest config for the hev → dsh adapter prototype.
 *
 * The dsh checkout has no built `lib/` yet, so every `@deepseek-ai/*` import is
 * resolved to its workspace SOURCE entry. This file imports nothing but Node
 * builtins on purpose: it is loaded by dsh's own vitest binary, and this
 * directory has no `node_modules` of its own.
 *
 * Run:
 *   cd <dsh-repo> && npx vitest run --config <this file>
 */
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
// dsh's own shared test plugin: `dsh-commands` uses standard TypeScript
// decorators, which Vite's default parser rejects. Reused rather than
// reimplemented so this prototype transforms sources exactly as dsh does.
import { standardDecoratorPlugin } from '../../../deepseek-harness/vitest.shared.ts'

const here = dirname(fileURLToPath(import.meta.url))
const DSH = process.env.DSH_REPO ?? join(here, '../../../deepseek-harness')

/** Map every `@deepseek-ai/*` package name in the dsh workspace to its directory. */
function packageDirs(): Map<string, string> {
  const found = new Map<string, string>()
  const record = (dir: string): void => {
    const manifest = join(dir, 'package.json')
    if (!existsSync(manifest) || !existsSync(join(dir, 'src'))) return
    const { name } = JSON.parse(readFileSync(manifest, 'utf8')) as { name?: string }
    if (name !== undefined && name.startsWith('@deepseek-ai/')) found.set(name, dir)
  }
  const groups = join(DSH, 'packages')
  for (const group of readdirSync(groups, { withFileTypes: true })) {
    if (!group.isDirectory()) continue
    for (const pkg of readdirSync(join(groups, group.name), { withFileTypes: true })) {
      if (pkg.isDirectory()) record(join(groups, group.name, pkg.name))
    }
  }
  for (const pkg of readdirSync(join(DSH, 'vendor'), { withFileTypes: true })) {
    if (pkg.isDirectory()) record(join(DSH, 'vendor', pkg.name))
  }
  return found
}

const dirs = packageDirs()
const hevDirs = new Map([
  ['@hev/dsh-runtime', join(here, 'packages/runtime')],
  ['@hev/dsh-skill', join(here, 'packages/skill')],
])

const dshSource = {
  name: 'dsh-source-resolver',
  enforce: 'pre' as const,
  resolveId(id: string): string | null {
    if (id.startsWith('@hev/')) {
      const parts = id.split('/')
      const dir = hevDirs.get(`${parts[0] ?? ''}/${parts[1] ?? ''}`)
      if (dir === undefined) return null
      const sub = parts.slice(2).join('/')
      if (sub === '') return join(dir, 'src', 'index.ts')
      const direct = join(dir, 'src', `${sub}.ts`)
      return existsSync(direct) ? direct : join(dir, 'src', sub, 'index.ts')
    }
    if (!id.startsWith('@deepseek-ai/')) return null
    const parts = id.split('/')
    const dir = dirs.get(`${parts[0] ?? ''}/${parts[1] ?? ''}`)
    if (dir === undefined) return null
    const sub = parts.slice(2).join('/')
    if (sub === '') return join(dir, 'src', 'index.ts')
    const direct = join(dir, 'src', `${sub}.ts`)
    return existsSync(direct) ? direct : join(dir, 'src', sub, 'index.ts')
  },
}

export default {
  root: here,
  cacheDir: '/tmp/hev-vite-cache',
  plugins: [dshSource, standardDecoratorPlugin()],
  server: { fs: { allow: [here, DSH] } },
  test: { include: ['packages/*/tests/**/*.spec.ts'], environment: 'node' as const },
}
