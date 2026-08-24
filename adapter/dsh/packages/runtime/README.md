# `@hev/dsh-runtime`

`@hev/dsh-runtime` connects DeepSeek Harness to the HEV CLI. It resolves Environment groups through CLI response schema v1 and exposes the selected group through `ctx.environment`.

Selection is live and process-local. The service stores only the ordered canonical Environment IDs for each exact `Session` object. `current(session)` invokes `hev env use <ids...> --output json` on every read, so callers observe the latest Environment revisions. When that exact Session has no explicit selection, `current()` resolves `base` and stores its canonical ID for subsequent reads. The Go Store automatically persists `base` as the ordinary empty `{ id: "env_base", revision: 1, skills: [] }` Environment when the store file is missing or contains an empty `environments` array.

When `@deepseek-ai/dsh-commands` is composed, the plugin registers `/hev` with the MVP operations `env create`, `skill add`, and `env use`. Create and add are direct CLI forwarding operations. Use resolves and validates the complete CLI response before replacing the Session's IDs; a CLI failure leaves the prior selection unchanged.

## Configuration

```yaml
- name: '@hev/dsh-runtime'
  config:
    executable: hev
```

`executable` defaults to `hev`. The CLI is invoked directly with argv and never through a shell.
