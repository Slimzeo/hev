# hev — your personal Skill Environment manager

hev groups native agent Skills into selectable Environments. Each live Session has exactly one current Environment; the current DSH adapter supports the minimal `create env → skill add env → use env` flow.

```bash
npx @deepseek-ai/dsh plugin --profile web add @slimzeo/hev-dsh-plugin@latest
npx @deepseek-ai/dsh web
```

The npm package includes the `hev` executable; users do not need Go or a separate launcher. See [adapter/dsh/README.md](adapter/dsh/README.md) for commands and source-development setup.

## Go Core layout

```text
cmd/hev/main.go                         process startup and dependency assembly
internal/handler/command.go             Cobra commands and argument handling
internal/domain/environment/model.go    Environment model, policy, validation, and status errors
internal/domain/environment/service.go  Environment operations and the Store dependency they require
internal/dal/json/environment.go        locked JSON persistence and atomic replacement
internal/packer/response.go             domain results and errors to CLI v2 JSON
contracts/cli/v2/                       process protocol schema and fixtures
test/hev_test.go                        Go Core tests
```

The dependency direction is `main -> handler -> domain/environment`; `dal/json` implements the Store required by the Environment service, and `packer` owns only CLI response conversion. Host-specific DSH types stay under `adapter/dsh`.
