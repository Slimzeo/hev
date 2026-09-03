import { rm } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'

const generatedLibrary = fileURLToPath(new URL('../lib/', import.meta.url))
await rm(generatedLibrary, { recursive: true, force: true })
