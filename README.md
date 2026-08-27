# hev — your personal Skill Environment manager

hev lets an Agent organize native Skills into Environment Workspaces and select one Workspace for each host Session. Without a selection, hev is inactive and native Skill visibility is unchanged. `base` and every newly created Environment include the bundled `hev-guide` Skill for discovering and enabling other Skills after activation.

```bash
npx @deepseek-ai/dsh plugin --profile web add @slimzeo/hev-dsh-plugin@latest
npx @deepseek-ai/dsh web
```

The npm package includes the `hev` executable and Agent-facing `hev_*` Tools; users do not need Go or a separate launcher. See [adapter/dsh/README.md](adapter/dsh/README.md) for commands and source-development setup.

## Go Core layout

```text
cmd/hev/main.go                         process startup and dependency assembly
internal/routers/
  base.go                               Cobra root initialization and execution
  hev.go                                env and skill command registration
internal/handler/
  environment.go                       env create, rename, delete, list, use, status, and quit handlers
  skill.go                             skill add, remove, and Environment list handlers
internal/model/
  environment.go                       Environment data structures
  session.go                           resolved Session Environment state
  skill.go                             Skill and Environment-Skill data structures
  source.go                            supported Coding Agent platform enum
internal/service/
  environment.go                       Environment operations, Session state transitions, and Store interface
  skill.go                             Add a Skill to one or more Environments
internal/dal/environment.go             locked Environment and Session-binding persistence
internal/common/response/               classified application errors and CLI status values
internal/common/utils.go                shared Cobra argument and output helpers
internal/constants/
  common.go                            fixed CLI, store, and validation values
  source.go                            platform home variables and directory names
internal/packer/
  environment.go                       Environment and Session CLI output packing
  skill.go                             Skill CLI output packing
contracts/cli/v2/                       process protocol schema and fixtures
test/hev_test.go                        Go Core tests
wiki/source-platform.md                 source ownership and storage isolation
wiki/cli-help-and-prompts.md            CLI help and message/prompt ownership
wiki/roadmap.md                         implemented boundary and remaining work
```

The command path is `main -> routers -> handler -> service -> model`; `main` injects one `EnvironmentDAL` through the `EnvironmentStore` interface, and `packer` owns only CLI response conversion. Session bindings store `sessionId + environmentId`; the service resolves the latest Environment before returning `model.Session`. Host-specific DSH types and the model-facing Tools stay under `adapter/dsh`.

Session-aware CLI calls require only an explicit host Session ID:

```bash
hev env use coding --session-id session-123
hev env status --session-id session-123
hev env quit --session-id session-123
hev skill list --session-id session-123
hev skill list coding
```

Host adapters internally add a source identifier such as `--source dsh`. The Go entry point maps that trusted source to the host's own `.hev` directory; neither the Agent nor an Environment operation chooses a filesystem path.
The CLI v2 response includes the resolved source on Environment and Session models so an adapter can reject data from another platform. See [Source 平台与 Session 隔离](wiki/source-platform.md).
