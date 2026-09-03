import { execFile } from 'node:child_process'
import { constants } from 'node:fs'
import { access, mkdir, mkdtemp, readFile, readdir, rm, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { promisify } from 'node:util'
import { Context } from '@deepseek-ai/cordis'

const execFileAsync = promisify(execFile)
const packageRoot = fileURLToPath(new URL('../', import.meta.url))
const temporaryRoot = await mkdtemp(join(tmpdir(), 'hev-package-'))

try {
  const archiveDirectory = join(temporaryRoot, 'archive')
  const unpackDirectory = join(temporaryRoot, 'unpacked')
  await mkdir(archiveDirectory)
  await mkdir(unpackDirectory)

  await execFileAsync('pnpm', [
    '--config.ignore-scripts=true',
    'pack',
    '--pack-destination',
    archiveDirectory,
  ], { cwd: packageRoot })
  const archives = (await readdir(archiveDirectory)).filter(name => name.endsWith('.tgz'))
  if (archives.length !== 1) throw new Error('pnpm pack must produce exactly one tarball')

  await execFileAsync('tar', ['-xzf', join(archiveDirectory, archives[0]), '-C', unpackDirectory])
  const extractedPackage = join(unpackDirectory, 'package')
  await symlink(join(packageRoot, 'node_modules'), join(extractedPackage, 'node_modules'), 'junction')

  const entries = ['hev-runtime', 'hev-skill-registry', 'hev-tool']
  const modules = await Promise.all(entries.map(async entry => await import(
    pathToFileURL(join(extractedPackage, 'lib', entry, 'index.js')).href
  )))
  for (const [index, module] of modules.entries()) {
    if (typeof module.apply !== 'function' && typeof module.default !== 'function') {
      throw new Error('packed ' + entries[index] + ' entry exports no Cordis plugin')
    }
  }

  process.env.DSH_HOME = join(temporaryRoot, 'dsh-home')
  const Runtime = modules[0].default
  if (typeof Runtime !== 'function') throw new Error('packed hev-runtime has no default service export')
  const ctx = new Context()
  try {
    const runtime = new Runtime(ctx)
    const environments = await runtime.list(new AbortController().signal)
    if (environments.length !== 1 || environments[0]?.id !== 'base') {
      throw new Error('packed hev executable did not return the default base Environment')
    }
    const filename = process.platform === 'win32' ? 'hev.exe' : 'hev'
    const binary = join(extractedPackage, 'bin', process.platform + '-' + process.arch, filename)
    await access(binary, process.platform === 'win32' ? constants.F_OK : constants.X_OK)
  } finally {
    await ctx.fiber.dispose()
  }

  const packageJson = JSON.parse(await readFile(join(extractedPackage, 'package.json'), 'utf8'))
  process.stdout.write('verified packed ' + packageJson.name + '@' + packageJson.version + '\n')
} finally {
  await rm(temporaryRoot, { recursive: true, force: true })
}
