# hev Roadmap

## Current boundary

hev currently manages one Skill Environment per host Session. An active Environment filters the DSH model-visible Skill catalog to bindings with `policy.kind == auto`; an inactive Session keeps the native unfiltered catalog.

Cross-Environment Skill invocation and Environment composition are intentionally not hev features. The DSH adapter extends the native Skill Registry, while `dsh-tool-skill` already owns explicit user invocation through `/skill-name`. hev therefore filters automatic discovery through `snapshot()` and `list()`, but preserves native `get()` so a user can explicitly invoke a globally installed user-invocable Skill without changing or composing the current Environment. The model-facing `skill` Tool still checks the filtered catalog before loading, so the model cannot use this path to bypass the active Environment.

An Environment change takes effect when DSH assembles the next model step and publishes the replacement Skill catalog. Skill instructions already injected into the durable Session transcript are historical input and are not erased retroactively.

## Remaining work

- Add an explicit command for changing an existing Skill binding between `auto` and `off`; `skill add` remains create-only and reports conflict for an existing binding.
- Define additional invocation policies beyond `auto` and `off`, such as `always`, `every N turns`, or one-shot behavior. Policy belongs to each Environment-Skill binding.
- Add source adapters for Claude Code, Codex, and OpenCode. Each adapter must supply its own trusted source and current Session ID while reusing the Go Core.
- Decide whether Environment scope should later include Tools, MCP servers, model selection, system prompts, permissions, or filesystem workspace settings. None are managed by the current MVP.
- Define subagent Environment selection and inheritance without conflating Environment identity with Agent identity.
- Add Skill invocation statistics and recommendation inputs for guided Environment initialization, including the earlier `common_env_init` direction for curating base from the native catalog.
- Split the JSON DAL into Environment records, Session bindings, file IO, and persisted models once those responsibilities need independent evolution; keep them in one `dal` package and do not add ceremonial layers.
- Add a persistent Web UI indicator for the current Environment; `/hev env status` remains the current explicit status surface.
- Fix and guard the packaged DSH artifact: resolve the bundled executable from the final package layout, include generated shared JavaScript chunks, and add a pack-install-import smoke test.

The existing technical and development proposals remain historical design inputs. This Wiki records the implemented direction and current backlog.
