# CLI contract

Each child directory is one CLI protocol version. [`v2/`](./v2/) contains the current JSON Schema and fixtures used by the Go CLI and host adapters. Environment summaries, full Environments, and Sessions carry the resolved Coding Agent `source`. Session-scoped success responses carry `data.session` with that source, the opaque host `sessionId`, and either the latest Environment or `null` when hev is inactive.

Every response embeds the same `schemaVersion`, numeric `code`, `message`, and `prompt` fields. `message` is diagnostic detail for logs and traces. `prompt` is frontend guidance telling a user or Agent what to do next; failure prompts are selected by the concrete failure case rather than by status code alone. Successful responses currently use an empty prompt. Text-mode failures render the same distinction as `hev: <message>` followed by `hint: <prompt>`.
