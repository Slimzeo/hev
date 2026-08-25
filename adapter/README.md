# Host adapters

Host adapters connect the [hev CLI v2 contract](../contracts/cli/v2/) to one agent harness without moving host-specific types into the Go core.

- [`dsh/`](./dsh/) contains the implemented DeepSeek Harness adapter and bundle.
- [`opencode/`](./opencode/) reserves the OpenCode integration boundary; it is not implemented yet.

