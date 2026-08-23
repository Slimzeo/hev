# `@hev/dsh-skill`

`@hev/dsh-skill` replaces the DSH `skill` service while inheriting its native `SkillRegistry` implementation. Provider registration, scope merging, ranking, discovery caching, validation, and Skill loading remain owned by `@deepseek-ai/dsh-skill`.

The replacement changes only Registry reads. When `SkillViewOptions.scope` is an exact live Agent with an Environment selected through `@hev/dsh-runtime`, `list()`, `snapshot()`, and `get()` expose only Environment entries whose policy is `auto`. `off` and absent Skills are hidden. Reads without an exact live Agent or without a selected Environment keep the native DSH result.

Each filtered read asks `ctx.environment.current()` for the latest Environment records. The package does not cache a filtered catalog, create another provider, copy Skill bodies, or modify DeepSeek Harness source.
