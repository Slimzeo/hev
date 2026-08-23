/** Focused Vitest composition for the HEV runtime and skill workspaces. */

import { join } from 'node:path'
import base from '../../vitest.config.ts'

const adapterRoot = join(import.meta.dirname, '../..')

const hevSource = {
  name: 'hev-source-resolver',
  enforce: 'pre' as const,
  resolveId(id: string): string | null {
    if (id === '@hev/dsh-runtime') return join(adapterRoot, 'packages/runtime/src/index.ts')
    if (id.startsWith('@hev/dsh-runtime/')) {
      return join(adapterRoot, 'packages/runtime/src', `${id.slice('@hev/dsh-runtime/'.length)}.ts`)
    }
    return null
  },
}

export default {
  ...base,
  plugins: [hevSource, ...(base.plugins ?? [])],
  test: {
    ...base.test,
    include: ['packages/*/tests/**/*.spec.ts', 'tests/**/*.spec.ts'],
  },
}
