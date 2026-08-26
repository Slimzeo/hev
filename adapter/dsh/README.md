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
/hev env list
/hev env use <id-or-name>
/hev env quit
/hev env status
/hev skill add <skill-key> <env-name> [env-name...] [--policy auto|off]
/hev skill list
/hev skill list --global
```

Selection is process-local and keyed by the exact live DSH `Session` object. A Session with no explicit `use` is not managed by hev: `/hev env status` reports `hev not activated`, and the Skill Registry keeps the native unfiltered view. `/hev env use` selects one Environment. `/hev env quit` moves a non-`base` selection to `base`; quitting `base` removes the Session selection and deactivates hev. The persisted `base` Environment and every newly created Environment enable the bundled `hev-guide` onboarding Skill.

`/hev env list` reports all persisted Environments. `/hev skill list` reports every Skill configured in the selected Environment, including `off` entries, while `/hev skill list --global` reports the current DSH view before Environment filtering. The Go Store automatically persists `base`, revision `1`, with `hev-guide` when its file is missing or contains an empty `environments` array. On read it migrates the earlier `env_base` ID to `base` and adds `hev-guide` once to existing base records that predate the guide; existing non-base Environments remain unchanged. The runtime stores one canonical Environment ID, then resolves it again on each filtered Skill read so Environment changes use the latest record. `env use` never composes Environments; multiple positional Environment names on `skill add` mutate several configurations without selecting them.

The Skill Registry entry keeps DSH provider discovery, winner selection, validation, and Skill loading unchanged, while contributing `hev-guide` as a bundled provider. With the native `skill-filesystem` row mounted, the candidate catalog therefore includes `<project>/.dsh/skills`, `<project>/.agents/skills`, `customSkillDirs`, `$DSH_HOME/skills`, `$DSH_AGENTS_HOME/skills`, and configured or provider-contributed bundled Skills. An active exact live Agent sees only winners from that catalog whose key has `policy.kind === 'auto'` in its current Environment; `off` and unlisted keys are unavailable through `list()`, `snapshot()`, and `get()`. Inactive Sessions and reads without an exact live Agent scope keep the full native view. A configured hev key that has no native winner remains hidden but does not make `use` fail.

## Development

```bash
pnpm install
pnpm typecheck
pnpm test
pnpm build
```

`pnpm build` compiles both Cordis subpaths and the supported hev binaries into the package. The integration test builds a temporary native hev executable, boots the composed rows through the DSH Loader, and verifies inactive passthrough, `create -> add -> use -> filtered native SkillRegistry`, two-level `quit`, and exact-Session isolation.

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
/hev skill add <native-auto-skill> review --policy auto
/hev skill add <native-off-skill> review --policy off
/hev env use review
```

The auto Skill is listed and loadable; the off Skill is not. A missing native Skill key does not block `use`, but it cannot appear because there is no native winner. Open another Session to verify hev is inactive there and native Skills remain unfiltered; selecting `review` there does not change the first Session.
