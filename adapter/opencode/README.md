# OpenCode adapter

This adapter is planned but not implemented. A future implementation should reuse the [hev CLI v2 contract](../../contracts/cli/v2/) and keep OpenCode-specific runtime and Skill models inside this directory. It must map each live Session to exactly one current Environment; cross-Environment Skill invocation, if added, is an operation-local qualified call and does not compose or change that selection.

