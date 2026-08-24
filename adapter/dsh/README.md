# HEV DSH adapter

This workspace contains two Cordis plugins and one bundle manifest:

- `@hev/dsh-runtime` owns `/hev` and the live `Session` to Environment selection.
- `@hev/dsh-skill` replaces the native `ctx.skills` implementation and filters native Skill winners by the selected Environment.
- `@hev/dsh-adapter` is only the installation bundle that disables the native Skill Registry row and inserts the two HEV plugins.

The MVP supports exactly these commands:

```text
/hev env create <name>
/hev skill add <skill-key> --env <name> [--env <name>...] [--policy auto|off]
/hev env use <id-or-name> [id-or-name...]
```

Selection is process-local and keyed by the exact live DSH `Session` object. An exact live Session with no explicit `use` starts from the ordinary empty `base` Environment. The Go Store automatically persists that `env_base`, revision `1`, `skills: []` record when its file is missing or contains an empty `environments` array. The runtime stores canonical Environment IDs, then resolves them again on each Skill read so Environment changes use the latest current record.

`@hev/dsh-skill` keeps DSH provider discovery, winner selection, validation, and Skill loading unchanged. An exact live Agent sees only native winners whose key has `policy.kind === 'auto'` in its current Environment; `off` and unlisted keys are unavailable through `list()`, `snapshot()`, and `get()`. Only a read without an exact live Agent scope keeps the native view. A configured HEV key that has no native winner remains hidden but does not make `use` fail.

## Development

```bash
pnpm install
pnpm typecheck
pnpm test
pnpm build
```

The integration test builds the real Go CLI in a temporary directory, boots the composed rows through the DSH Loader, and verifies the default `base`, `create -> add -> use -> filtered native SkillRegistry`, and exact-Session isolation.

## Local DSH integration

Build HEV and the adapter, then install this out-of-tree bundle into the DSH Web profile. Use absolute paths so the profile link and executable configuration do not depend on either checkout's working directory:

```bash
cd <hev-root>
mkdir -p .local/bin
go build -o "$PWD/.local/bin/hev" ./cmd/hev
pnpm --dir adapter/dsh install
pnpm --dir adapter/dsh build
cd <deepseek-harness-root>
pnpm dsh plugin --profile web add \
  /absolute/path/to/hev/adapter/dsh/packages/runtime \
  /absolute/path/to/hev/adapter/dsh/packages/skill \
  /absolute/path/to/hev/adapter/dsh
```

The two package paths install as plain profile dependencies; the final root path declares the bundle layer. A local workspace link does not make its sibling packages resolvable from the profile, so all three paths are required. Once the packages are published to a registry, installing the root bundle also installs its declared dependencies.

Set the runtime executable in `$DSH_HOME/profiles/web/cordis.patch.yml` (`$DSH_HOME` defaults to `~/.dsh`):

```yaml
- id: hev-runtime
  config:
    executable: /absolute/path/to/hev/.local/bin/hev
```

Inspect the effective composition before booting:

```bash
pnpm dsh --profile web --dump-config
```

The dump must keep `id: skill` with `name: '@deepseek-ai/dsh-skill'` and `disabled: true`, then contain enabled `hev-runtime` and `hev-skill` rows. Start the profile without opening a browser:

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
