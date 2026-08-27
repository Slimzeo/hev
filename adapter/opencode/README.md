# OpenCode adapter

This adapter is planned but not implemented. A future implementation should reuse the [hev CLI v2 contract](../../contracts/cli/v2/) and keep OpenCode-specific runtime and Skill models inside this directory. OpenCode Custom Tools expose the current `context.sessionID`; the adapter should pass that value to `--session-id` and fix the Core source to `opencode`. Core resolves that source through `OPENCODE_CONFIG_DIR`, then `XDG_CONFIG_HOME/opencode`, then `~/.config/opencode`. Core already enforces one current Environment per Session. Cross-Environment Skill invocation, if added, is an operation-local qualified call and does not compose or change that selection.

