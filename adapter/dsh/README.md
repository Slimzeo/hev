# hev DSH plugin

`@slimzeo/hev-dsh-plugin` is the single installable hev bundle for DeepSeek Harness. It exposes two Cordis entries from one npm package:

- `@slimzeo/hev-dsh-plugin/hev-runtime` owns `/hev` and the live `Session` to Environment selection.
- `@slimzeo/hev-dsh-plugin/hev-skill-registry` replaces the native `ctx.skills` implementation and filters native Skill winners by the selected Environment.

The package also includes the `hev` executable for supported macOS, Linux, and Windows targets. Users do not install Go or configure an executable path.

## Source layout

```text
src/hev-runtime/index.ts          Cordis service entry, Session selection, and optional /hev command
src/hev-runtime/environment.ts    Environment data and numeric status types
src/hev-runtime/cli.ts            hev process invocation and CLI v2 response validation
src/hev-runtime/executable.ts     package-local executable resolution
src/hev-skill-registry/index.ts   native DSH Skill Registry replacement and Environment filtering
tests/runtime.spec.ts             hev-runtime unit coverage
tests/skill-registry.spec.ts      Registry filtering coverage
tests/integration.spec.ts         Loader composition with the real Go CLI
```

The top-level source folders follow Cordis plugin ownership. Runtime helpers remain inside `hev-runtime`; `hev-skill-registry` stays separate because it replaces a different DSH service.

## Quick start

```bash
npx @deepseek-ai/dsh plugin --profile web add @slimzeo/hev-dsh-plugin@latest
npx @deepseek-ai/dsh web
```

The MVP supports exactly these commands:

```text
/hev env create <name>
/hev skill add <skill-key> --env <name> [--env <name>...] [--policy auto|off]
/hev env use <id-or-name>
```

Selection is process-local and keyed by the exact live DSH `Session` object. Every live Session has exactly one current Environment. A Session with no explicit `use` starts from the ordinary empty `base` Environment. The Go Store automatically persists that `env_base`, revision `1`, `skills: []` record when its file is missing or contains an empty `environments` array. The runtime stores one canonical Environment ID, then resolves it again on each Skill read so Environment changes use the latest current record. `env use` never composes Environments; repeated `--env` on `skill add` remains valid because it mutates several Environment configurations without selecting them.

The Skill Registry entry keeps DSH provider discovery, winner selection, validation, and Skill loading unchanged. An exact live Agent sees only native winners whose key has `policy.kind === 'auto'` in its current Environment; `off` and unlisted keys are unavailable through `list()`, `snapshot()`, and `get()`. Only a read without an exact live Agent scope keeps the native view. A configured hev key that has no native winner remains hidden but does not make `use` fail.

## Development

```bash
pnpm install
pnpm typecheck
pnpm test
pnpm build
```

`pnpm build` compiles both Cordis subpaths and the supported hev binaries into the package. The integration test builds a temporary native hev executable, boots the composed rows through the DSH Loader, and verifies the default `base`, `create -> add -> use -> filtered native SkillRegistry`, and exact-Session isolation.

## Local DSH integration

Build the complete package, then add its one local path to the DSH Web profile:

```bash
cd <hev-root>
pnpm --dir adapter/dsh install
pnpm --dir adapter/dsh build
cd <deepseek-harness-root>
pnpm dsh plugin --profile web add /absolute/path/to/hev/adapter/dsh
```

The package resolves its own platform binary. To test against a separately built binary, override only the environment row in `$DSH_HOME/profiles/web/cordis.patch.yml` (`$DSH_HOME` defaults to `~/.dsh`):

```yaml
- id: hev-runtime
  config:
    executable: /absolute/path/to/hev/.local/bin/hev
```

Inspect the effective composition before booting:

```bash
pnpm dsh --profile web --dump-config
```

The dump must keep `id: skill` with `name: '@deepseek-ai/dsh-skill'` and `disabled: true`, then contain enabled `hev-runtime` and `hev-skill-registry` rows. DSH profiles intentionally disable peer auto-install and resolve host DSH peers from the DSH installation. `pnpm peers check` may therefore report those host peers as missing; do not install duplicate copies into the profile. Use the config dump and an actual profile start as the integration checks. Start the profile without opening a browser:

```bash
pnpm dsh web --no-open
```

In one live Session, create an Environment, add an `auto` key and an `off` key that correspond to native DSH Skills, then select it:

```text
/hev env create review
/hev skill add <native-auto-skill> --env review --policy auto
/hev skill add <native-off-skill> --env review --policy off
/hev env use review
```

The auto Skill is listed and loadable; the off Skill is not. A missing native Skill key does not block `use`, but it cannot appear because there is no native winner. Open another Session to verify it still uses the empty `base` Environment; selecting `review` there does not change the first Session.
