/** Package-local hev executable resolution.
 * @module @slimzeo/hev-dsh-plugin/hev-runtime/executable
 */

import { fileURLToPath } from 'node:url'

const SUPPORTED_TARGETS = new Set([
  'darwin-arm64',
  'darwin-x64',
  'linux-arm64',
  'linux-x64',
  'win32-x64',
])

/** Resolve the hev binary bundled for the current Node platform.
 * @returns absolute path to the package-local executable.
 */
export function bundledExecutable(): string {
  const target = `${process.platform}-${process.arch}`
  if (!SUPPORTED_TARGETS.has(target)) {
    throw new Error(`hev does not provide a binary for ${target}`)
  }
  const filename = process.platform === 'win32' ? 'hev.exe' : 'hev'
  return fileURLToPath(new URL(`../../bin/${target}/${filename}`, import.meta.url))
}
