# hev DSH plugin

> **Status:** alpha. `@owariband/hev-dsh-plugin` has not been published to npm yet. The npm command below describes the planned first release; use the source-development setup for now.

`@owariband/hev-dsh-plugin` is the single installable hev bundle for DeepSeek Harness. It exposes three Cordis entries from one npm package:

- `@owariband/hev-dsh-plugin/hev-runtime` owns `/hev` and delegates Session selection to the Go Core.
- `@owariband/hev-dsh-plugin/hev-skill-registry` replaces the native `ctx.skills` implementation and filters native Skill winners by the selected Environment.
- `@owariband/hev-dsh-plugin/hev-tool` exposes Environment Workspace management directly to the current Agent.

The package also includes the `hev` executable for supported macOS, Linux, and Windows targets. Users do not install Go or configure an executable path.

## Source layout

```text
src/hev-runtime/index.ts          Cordis service entry, Session selection, and optional /hev command
src/hev-runtime/environment.ts    Environment data and numeric status types
src/hev-runtime/cli.ts            hev process invocation and CLI v2 response validation
src/hev-runtime/executable.ts     package-local executable resolution
src/hev-skill-registry/index.ts   native DSH Skill Registry replacement and Environment filtering
src/hev-tool/index.ts             model-facing Environment Workspace tools
tests/runtime.spec.ts             hev-runtime unit coverage
tests/skill-registry.spec.ts      Registry filtering coverage
tests/tool.spec.ts                model-facing Tool coverage
tests/integration.spec.ts         Loader composition with the real Go CLI
```

The top-level source folders follow Cordis plugin ownership. Runtime helpers remain inside `hev-runtime`; `hev-skill-registry` replaces the native Skill service; `hev-tool` is the model-facing consumer.

## Planned npm quick start

```bash
npx @deepseek-ai/dsh plugin --profile web add @owariband/hev-dsh-plugin@latest
npx @deepseek-ai/dsh web
```

The human command surface supports:

```text
/hev env create <name>
/hev env rename <id-or-name> <new-name>
/hev env delete <id-or-name>
/hev env list
/hev env use <id-or-name>
/hev env quit
/hev env status
/hev skill add <skill-key> <env-name> [env-name...] [--policy auto|off]
/hev skill remove <skill-key> <env-name> [env-name...]
/hev skill list [id-or-name]
/hev skill list --global
```

Selection is persisted by the Go Core under `$DSH_HOME/.hev/session-bindings.json`, keyed by the DSH Session ID. A Session with no explicit `use` is not managed by hev: `/hev env status` reports `hev not activated`, and the Skill Registry keeps the native unfiltered view. `/hev env use` selects one Environment. `/hev env quit` moves a non-`base` selection to `base`; quitting `base` removes the Session selection and deactivates hev. The persisted `base` Environment and every newly created Environment enable the bundled `hev-guide` onboarding Skill.

The Agent can perform the same operations directly through `hev_env_status`, `hev_env_use`, `hev_env_quit`, `hev_env_create`, `hev_env_rename`, `hev_env_delete`, `hev_env_list`, `hev_skill_add`, `hev_skill_remove`, and `hev_skill_list`. Session-scoped Tools take the exact calling Agent from DSH execution context and do not expose a model-controlled `sessionId` argument.

CLI failures keep diagnostic `message` text separate from recovery `prompt` text. `/hev` and the model-facing Tools log the diagnostic while returning the recovery prompt to the caller, so an Agent can correct its next invocation without receiving storage or process details. `/hev help`, `/hev help env`, `/hev help skill`, and command-level `--help` forms describe the human command surface.

The DSH runtime fixes the Core source to `dsh`; neither the Agent nor plugin configuration chooses a storage path. The Go entry point resolves that source through `$DSH_HOME`, defaulting to `~/.dsh/.hev/`.

`/hev env list` reports all persisted Environments. `/hev skill list` reports every Skill configured in the selected Environment, `/hev skill list <env>` inspects one Environment without selecting it, and `/hev skill list --global` reports the current DSH view before Environment filtering. `env rename` preserves the Environment ID and Session bindings. `env delete` rejects `base`; a Session still bound to a deleted Environment resolves to `base` on its next hev operation. The Go Store automatically persists `base`, revision `1`, with `hev-guide` when its file is missing or contains an empty `environments` array. On read it migrates the earlier `env_base` ID to `base` and adds `hev-guide` once to existing base records that predate the guide; existing non-base Environments remain unchanged. The Core stores one canonical Environment ID per Session, then resolves the latest Environment on each filtered Skill read. `env use` never composes Environments; multiple positional Environment names on `skill add` and `skill remove` mutate several configurations without selecting them.

The Skill Registry entry keeps DSH provider discovery, winner selection, validation, and Skill loading unchanged, while contributing `hev-guide` as a bundled provider. With the native `skill-filesystem` row mounted, the candidate catalog therefore includes `<project>/.dsh/skills`, `<project>/.agents/skills`, `customSkillDirs`, `$DSH_HOME/skills`, `$DSH_AGENTS_HOME/skills`, and configured or provider-contributed bundled Skills. An active exact live Agent sees only winners from that catalog whose key has `policy.kind === 'auto'` in its current Environment through `list()` and `snapshot()`; `off` and unlisted keys stay out of automatic model discovery. Direct `get()` remains native so DSH's user-explicit `/skill-name` path can load a globally installed user-invocable Skill without hev inventing cross-Environment composition. The model-facing `skill` Tool still checks the filtered list before loading. Inactive Sessions and reads without an exact live Agent scope keep the full native view. A configured hev key that has no native winner remains hidden but does not make `use` fail.

## Development

```bash
pnpm install
pnpm typecheck
pnpm test
pnpm build
pnpm run test:package
```

`pnpm build` compiles the Cordis subpaths and supported hev binaries into the package. `pnpm run test:package` packs and extracts the publishable artifact, imports all three plugin entries, and invokes the package-local binary without an executable override. Use `pnpm run release:check` to run the tests, build, and packed-artifact check together. The integration test builds a temporary native hev executable, boots the composed rows through the DSH Loader, and verifies inactive passthrough, Environment and Skill mutations, filtered native Skill discovery, two-level `quit`, and exact-Session isolation.

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

The auto Skill appears in the model catalog; the off Skill does not. A user may still explicitly invoke a globally installed user-invocable Skill through DSH's native `/skill-name` path. A missing native Skill key does not block `use`, but it cannot appear because there is no native winner. Open another Session to verify hev is inactive there and native Skills remain unfiltered; selecting `review` there does not change the first Session.
