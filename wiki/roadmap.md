# hev Roadmap

## Current boundary

hev currently manages one Skill Environment per host Session. An active Environment filters the DSH model-visible Skill catalog to bindings with `policy.kind == auto`; an inactive Session keeps the native unfiltered catalog.

Cross-Environment Skill invocation and Environment composition are intentionally not hev features. The DSH adapter extends the native Skill Registry, while `dsh-tool-skill` already owns explicit user invocation through `/skill-name`. hev therefore filters automatic discovery through `snapshot()` and `list()`, but preserves native `get()` so a user can explicitly invoke a globally installed user-invocable Skill without changing or composing the current Environment. The model-facing `skill` Tool still checks the filtered catalog before loading, so the model cannot use this path to bypass the active Environment.

An Environment change takes effect when DSH assembles the next model step and publishes the replacement Skill catalog. Skill instructions already injected into the durable Session transcript are historical input and are not erased retroactively.

## Remaining work

- Add an explicit command for changing an existing Skill binding between `auto` and `off`; `skill add` remains create-only and reports conflict for an existing binding.
- Define additional invocation policies beyond `auto` and `off`, such as `always`, `every N turns`, or one-shot behavior. Policy belongs to each Environment-Skill binding.
- Add source adapters for Claude Code and OpenCode. Each adapter must supply its own trusted source and current Session ID while reusing the Go Core.
- Research the released Codex source path from Skill discovery through `SkillCatalogEntry.prompt_visible` and model-context rendering, and keep the findings source-backed against the Codex version used by the adapter.
- Propose and submit an upstream `openai/codex` PR for a general Session-scoped Skill visibility extension that can hide Environment-external Skills from the model-visible catalog without disabling explicit `$skill-name` invocation. Build and verify the change on a fork before submitting it upstream; do not add hev-specific behavior to Codex Core.
- Treat real Codex catalog filtering as the Codex adapter acceptance gate: an active Environment must reduce model-visible Skill metadata and context pressure while preserving explicit global Skill invocation. A Plugin or Hook that only tells the model to ignore Skills does not establish hev's product value and must not be described as a completed Codex adapter.
- Decide whether Environment scope should later include Tools, MCP servers, model selection, system prompts, permissions, or filesystem workspace settings. None are managed by the current MVP.
- Define subagent Environment selection and inheritance without conflating Environment identity with Agent identity.
- Add Skill invocation statistics and recommendation inputs for guided Environment initialization, including the earlier `common_env_init` direction for curating base from the native catalog.
- Split the JSON DAL into Environment records, Session bindings, file IO, and persisted models once those responsibilities need independent evolution; keep them in one `dal` package and do not add ceremonial layers.
- Add a persistent Web UI indicator for the current Environment; `/hev env status` remains the current explicit status surface.
- Publish `@owariband/hev-dsh-plugin` to the public npm registry after confirming ownership of the `@owariband` scope, enabling npm publish authentication, and running `pnpm run release:check`. The package has not been published yet.
- Add CI release jobs for macOS, Linux, and Windows so every bundled binary is executed on its target platform before publishing; local development currently cross-compiles all targets but executes only the host binary.
- Add Environment authorization and audit policy before using shared state in a multi-user deployment. Current DSH model tools may mutate source-wide Environment configuration.
- Add Session-binding garbage collection for Sessions that never return after their Environment is deleted. Active or resumed Sessions already converge to base on their next hev operation.
- Decide whether the standalone CLI should keep accepting `--source` for adapter development or move platform selection behind adapter-owned launchers. Model tools already hide source and state paths, but a human process can currently choose any supported source explicitly.

The existing technical and development proposals remain historical design inputs. This Wiki records the implemented direction and current backlog.
