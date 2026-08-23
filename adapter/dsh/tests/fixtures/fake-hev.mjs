#!/usr/bin/env node
/**
 * Stand-in for the Go `hev` CLI: implements exactly the §7 contract surface the
 * adapter consumes, reading its "storage" from the JSON file named by
 * `FAKE_HEV_DB`. Keeps the plugin tests independent of the real binary.
 */
import { readFileSync } from 'node:fs'

const argv = process.argv.slice(2)
const db = JSON.parse(readFileSync(process.env.FAKE_HEV_DB, 'utf8'))

function fail(code, message) {
  process.stdout.write(`${JSON.stringify({ schemaVersion: 1, ok: false, error: { code, message } })}\n`)
  process.exit(1)
}

function environment(name) {
  const env = db.environments[name]
  if (env === undefined) fail('ENV_NOT_FOUND', `environment not found: ${name}`)
  return {
    id: env.id,
    name,
    revision: env.revision,
    skills: env.skills.map(skill => ({
      id: skill.id,
      identity: { skillName: skill.skillName, creator: skill.creator ?? 'user', version: skill.version ?? '1.0.0' },
      realPath: skill.realPath,
      mode: skill.mode ?? { type: 'auto' },
    })),
  }
}

if (argv[0] !== 'env') fail('INVALID_ARGUMENT', `unsupported command: ${argv.join(' ')}`)

if (argv[1] === 'activate') {
  const names = argv.slice(2).filter(word => word !== '--output' && word !== 'json')
  if (names.length === 0) fail('INVALID_ARGUMENT', 'activate needs an environment')
  const environments = names.map(environment)
  process.stdout.write(`${JSON.stringify({ schemaVersion: 1, ok: true, data: { environments } })}\n`)
  process.exit(0)
}

if (argv[1] === 'deactivate') {
  process.stdout.write(`${JSON.stringify({ schemaVersion: 1, ok: true, data: { environments: [environment('base')] } })}\n`)
  process.exit(0)
}

fail('INVALID_ARGUMENT', `unsupported subcommand: ${String(argv[1])}`)
