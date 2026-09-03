# hev

> Session-scoped Skill Environments for AI agent harnesses.

[简体中文](README.zh-CN.md)

hev lets one agent harness keep its native, globally installed Skills while giving each host Session a deliberately smaller catalog for **automatic model discovery**. An Environment is an allowlist of Skill keys and policies; it is not another Skill installer, filesystem sandbox, or prompt-only convention.

> [!IMPORTANT]
> hev is alpha software. The DeepSeek Harness (DSH) adapter is implemented, but `@owariband/hev-dsh-plugin` has not been published to npm yet. The planned npm command below will work after the first release.

## Why hev

Agent installations tend to accumulate Skills. Leaving every installed Skill in every model turn creates unnecessary context pressure and makes automatic selection noisy. Putting Skills into separate folders fixes that by making the host forget its own discovery model. Telling the model to ignore Skills does not create an enforceable boundary.

hev takes the smaller step: it preserves the host catalog, then projects a Session-specific allowlist at the host's Skill Registry boundary.

```text
Native DSH providers
  └─ discover candidates and choose native winners        unchanged
       └─ hev resolves this Session's Environment
            └─ keep only bindings with policy = auto
                 └─ model-visible Skill catalog           filtered

Explicit user /skill-name invocation remains a native DSH path.
```

## What it looks like

Assume DSH can already discover `github`, `paper-explain`, and other installed Skills. One Session can opt into a focused Environment without changing what another Session sees:

```text
# Session A
/hev env create review
/hev skill add github review --policy auto
/hev env use review

# Automatic model catalog for Session A
hev-guide, github

# Session B, with no hev selection
native DSH catalog remains unchanged
```

`off` records a Skill in an Environment without making it automatically visible. A user can still explicitly invoke a globally installed, user-invocable Skill through DSH's native `/skill-name` behavior; hev does not invent cross-Environment composition to bypass its catalog boundary.

## Current scope

- **Implemented host:** DeepSeek Harness (DSH).
- **Selection:** one Environment per host Session. A Session with no selection is unmanaged and keeps DSH's native view.
- **Persistence:** the Go core stores DSH state under `$DSH_HOME/.hev`, defaulting to `~/.dsh/.hev`.
- **Isolation:** the adapter fixes `source=dsh` and obtains the calling Session from DSH. The model cannot choose another source, Session ID, or state directory.
- **Current policies:** `auto` and `off`.

hev currently targets local, single-user development. Shared-state authorization, audit policy, additional invocation policies, and non-DSH adapters remain future work.

## Architecture

```text
Go Core
  Environment records + policies + source-isolated Session bindings
        ▲ CLI v2 JSON
DSH adapter
  hev-runtime: exact Session access and /hev command
  hev-skill-registry: native winners -> Environment allowlist
  hev-tool: Agent-facing Environment operations
```

The core deliberately does not store DSH `SkillCandidate` objects, provider locators, physical Skill paths, or Skill bodies. DSH remains responsible for provider discovery, winner selection, validation, and lazy loading; the adapter is responsible for interpreting that catalog for one live Session.

## Planned installation

After the first npm release:

```bash
npx @deepseek-ai/dsh plugin --profile web add @owariband/hev-dsh-plugin@latest
npx @deepseek-ai/dsh web
```

The package includes the `hev` executable and all three DSH plugin entries. Users will not need Go or a separate launcher.

## Commands

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

The DSH package also exposes equivalent `hev_*` tools to the current Agent. Session-scoped tools receive their calling Session from DSH and do not accept a model-controlled Session ID.

## Develop from source

The Go core requires Go 1.24 or later:

```bash
go build -o .local/bin/hev ./cmd/hev
go test ./...
./.local/bin/hev --help
```

DSH adapter development currently expects the `hev` and `deepseek-harness` repositories to be sibling directories:

```text
workspace/
├── hev/
└── deepseek-harness/
```

Use Node.js 22.19 or later and the repository-pinned pnpm version:

```bash
corepack pnpm@11.7.0 --dir adapter/dsh install
corepack pnpm@11.7.0 --dir adapter/dsh typecheck
corepack pnpm@11.7.0 --dir adapter/dsh test
corepack pnpm@11.7.0 --dir adapter/dsh run release:check
```

`release:check` runs adapter tests, builds all supported binaries, packs the npm artifact, imports its plugin entries, and invokes its package-local `hev` executable. See the [DSH adapter guide](adapter/dsh/README.md) for local profile integration.

## Project status

The [roadmap](wiki/roadmap.md) distinguishes the implemented boundary from future work. Before opening a pull request, run the Go tests and relevant DSH checks above.

## License

hev is licensed under the [MIT License](LICENSE).
