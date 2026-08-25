import { execFileSync } from 'node:child_process'
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const packageRoot = fileURLToPath(new URL('../', import.meta.url))
const repositoryRoot = fileURLToPath(new URL('../../../', import.meta.url))
const targets = [
  ['darwin', 'arm64'],
  ['darwin', 'amd64'],
  ['linux', 'arm64'],
  ['linux', 'amd64'],
  ['windows', 'amd64'],
]

for (const [goos, goarch] of targets) {
  const nodePlatform = goos === 'windows' ? 'win32' : goos
  const nodeArch = goarch === 'amd64' ? 'x64' : goarch
  const directory = join(packageRoot, 'bin', `${nodePlatform}-${nodeArch}`)
  const executable = join(directory, goos === 'windows' ? 'hev.exe' : 'hev')
  mkdirSync(directory, { recursive: true })
  execFileSync('go', [
    'build',
    '-trimpath',
    '-ldflags=-s -w',
    '-o',
    executable,
    './cmd/hev',
  ], {
    cwd: repositoryRoot,
    env: { ...process.env, CGO_ENABLED: '0', GOARCH: goarch, GOOS: goos },
    stdio: 'inherit',
  })
}
