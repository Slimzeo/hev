# HEV DSH adapter

This workspace contains two Cordis plugins and one bundle manifest:

- `@hev/dsh-runtime` owns `/hev` and the live `Session` to Environment selection.
- `@hev/dsh-skill` replaces the native `ctx.skills` implementation and filters native Skill winners by the selected Environment.
- `@hev/dsh-adapter` is only the installation bundle that disables the native Skill Registry row and inserts the two HEV plugins.

The MVP supports exactly these commands:

```text
/hev env create <name>
/hev skill add <skill-key> --env <name> [--env <name>...] [--policy auto|off]
/hev env activate <id-or-name> [id-or-name...]
```

Activation state is process-local and keyed by the exact live DSH `Session` object. The runtime stores canonical Environment IDs, then resolves them again on each Skill read so Environment changes use the latest current record. It does not write a Session event and does not provide restart or resume restoration.

`@hev/dsh-skill` keeps DSH provider discovery, winner selection, caching, and Skill parsing unchanged. With no live selection it returns the native view. With a selection it exposes only native winners whose key has `policy.kind === 'auto'`; `off` and unlisted keys are unavailable through `list()`, `snapshot()`, and `get()`.

## Development

```bash
pnpm install
pnpm typecheck
pnpm test
pnpm build
```

The integration test builds the real Go CLI in a temporary directory, boots the composed rows through the DSH Loader, and verifies `create -> add -> activate -> filtered native SkillRegistry`.
