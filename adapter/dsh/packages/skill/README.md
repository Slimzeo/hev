# `@hev/dsh-skill`

`@hev/dsh-skill` replaces the DSH `skill` service while inheriting its native `SkillRegistry` implementation. Provider registration, scope merging, ranking, validation, and Skill loading remain owned by `@deepseek-ai/dsh-skill`.

The replacement changes only Registry reads. When `SkillViewOptions.scope` is an exact live Agent, `list()`, `snapshot()`, and `get()` expose only native winners whose HEV entry has policy `auto`; `off`, unlisted, and configured-but-missing native Skills are hidden. A Session without an explicit `use` reads the empty `base` Environment. Only reads without an exact registered live Agent scope keep the native DSH result. Missing native Skills do not make `use` fail.

Each filtered read asks `ctx.environment.current()` for the latest Environment records. The package does not create another provider, copy Skill bodies, or modify DeepSeek Harness source.
