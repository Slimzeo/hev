# hev — your personal Skill Environment manager

hev groups native agent Skills into selectable Environments. A live Session may select one Environment; without a selection, hev is inactive and native Skill visibility is unchanged. `base` and every newly created Environment include the bundled `hev-guide` Skill for discovering and enabling other Skills after activation.

```bash
npx @deepseek-ai/dsh plugin --profile web add @slimzeo/hev-dsh-plugin@latest
# npx @deepseek-ai/dsh plugin --profile web add /Users/bytedance/workspace/other-project/hev/adapter/dsh
npx @deepseek-ai/dsh web
```

The npm package includes the `hev` executable; users do not need Go or a separate launcher. See [adapter/dsh/README.md](adapter/dsh/README.md) for commands and source-development setup.

## Go Core layout

```text
cmd/hev/main.go                         process startup and dependency assembly
internal/routers/
  base.go                               Cobra root initialization and execution
  hev.go                                env and skill command registration
internal/handler/
  environment.go                       env create, list, and use handlers
  skill.go                             skill add handler
internal/model/
  environment.go                       Environment data structures
  skill.go                             Skill and Environment-Skill data structures
internal/service/
  environment.go                       Environment create, list, and resolve operations plus Store interface
  skill.go                             Add a Skill to one or more Environments
internal/dal/environment.go             locked Environment persistence and atomic replacement
internal/common/response/               classified application errors and CLI status values
internal/common/utils.go                shared Cobra argument and output helpers
internal/constants/common.go            fixed CLI, store, and validation values
internal/packer/
  environment.go                       Environment CLI output packing
  skill.go                             Skill CLI output packing
contracts/cli/v2/                       process protocol schema and fixtures
test/hev_test.go                        Go Core tests
```

The command path is `main -> routers -> handler -> service -> model`; `main` injects `dal.EnvironmentDAL` through the Store interface, and `packer` owns only CLI response conversion. Host-specific DSH types stay under `adapter/dsh`.
