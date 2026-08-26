---
name: hev-guide
description: Guide users through Hev skill environments, distinguish the current environment from all DSH-discoverable Skills, find suitable Skills, and add them to the active environment. Use when the user asks what Hev is, which environment or Skills are active, why a discovered Skill is unavailable, how to find or enable a Skill, or how to configure the hev plugin.
---

# Hev Guide

Treat each Hev environment as an allowlist over the DSH Skill catalog. A Skill may be installed and globally discoverable without being enabled in the current environment.

The `base` Environment and every Environment created by hev include this guide with the `auto` policy.

## Inspect the environment

Use these commands and keep their meanings separate:

- `/hev env status` — show the Hev environment selected by this Session.
- `/hev skill list` — show bindings in that environment, including `auto` and `off` policies.

Do not infer current availability only from files under `~/.dsh/skills`. Files show installation; the current Hev binding controls whether the session catalog can expose them.

## Find DSH-discoverable Skills

Use DSH's native discovery order. The project root is the nearest ancestor of the current working directory that contains `.git`; if no such ancestor exists, DSH uses the current working directory.

1. `<project>/.dsh/skills`
2. `<project>/.agents/skills`
3. every directory in the active `skill-filesystem` provider's `customSkillDirs` configuration
4. `${DSH_HOME:-~/.dsh}/skills`
5. `${DSH_AGENTS_HOME:-~/.agents}/skills`
6. bundled Skills contributed by DSH or installed plugins, including the configured `bundledSkillDir` or `$DSH_BUNDLED_SKILL_DIR`

Inspect the direct children of the filesystem roots with the available filesystem or shell tools. Treat a child directory containing `SKILL.md`, or a flat Markdown Skill file accepted by DSH, as a candidate. Read its frontmatter to report the canonical `name` and `description`. Ignore `.system` under the DSH user root.

The native Registry resolves duplicate names by provider layer, then source rank and provider order. Do not present every duplicate file as a separate available Skill. Prefer the winning canonical name when it can be determined; otherwise report the possible collision. Packaged bundled Skills may be exposed directly by a provider rather than copied into a shared `skills` directory, so use the session catalog or effective DSH composition when available instead of assuming every bundled Skill has a user-visible filesystem path.

Clearly label this as the DSH-discoverable set, not the current environment's enabled set. If configuration or filesystem inspection is unavailable, explain which sources could not be checked instead of guessing.

## Add a Skill

1. Read the current environment name from `/hev env status`.
2. Inspect all DSH discovery sources above and match candidate descriptions to the user's task.
3. Confirm the choice when more than one candidate is plausible.
4. Add the selected Skill:

   ```text
   /hev skill add <skill-name> --env <environment-name> --policy auto
   ```

5. Verify with `/hev skill list`. On the next agent turn, confirm that the Skill appears in the session catalog and load it before use.

When adding more than one Skill, put every `/hev skill add` command in its own fenced `text` block with a short label before it. Never combine multiple commands in one block; keep each command independently copyable.

Use `--policy off` only when the user wants the binding recorded but unavailable to the model.

If slash commands cannot be executed from the current model/tool surface, give the user the exact command to run. Never claim that a Skill was added until the command succeeds.

## Explain common states

- Installed + globally discoverable + bound as `auto`: available in the current Session.
- Installed + globally discoverable + unbound: hidden by Hev; add it to the environment.
- Bound as `off`: intentionally hidden; do not describe it as enabled.
- Bound but absent from DSH discovery and the session catalog: configuration exists, but DSH has no usable Skill definition. Install or repair the Skill first.
- Different Sessions may select different environments. Always inspect the current Session instead of assuming another Session's selection.

When answering “what Skills are available?”, report both the environment bindings and the DSH-discoverable Skills when useful, and label them explicitly.
