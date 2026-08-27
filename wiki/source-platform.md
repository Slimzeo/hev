# Source 平台与 Session 隔离

## 结论

hev Core 复用同一套 Environment、Skill 和 Session 业务逻辑，但不同 Coding Agent 平台的数据必须隔离。Core 使用 `source` 识别数据属于哪个平台，并把它映射到该平台自己的 `.hev` 目录。

`source` 不是模型参数，也不是 Environment 命令临时选择的路径。每个 adapter 在调用 Core 时固定填写自己的 source；Session ID 由宿主运行时提供。模型调用 hev Tool 时只填写真正的业务参数。

## Core 模型

Go Core 定义闭合的 `Source` 枚举：

```go
type Source string

const (
    SourceStandalone Source = "standalone"
    SourceDSH        Source = "dsh"
    SourceClaudeCode Source = "claude-code"
    SourceCodex      Source = "codex"
    SourceOpenCode   Source = "opencode"
)
```

`Environment` 和 `Session` 都携带 `Source`：

```go
type Environment struct {
    Source   Source
    ID       EnvironmentID
    Name     string
    Revision uint64
    Skills   []EnvironmentSkill
}

type Session struct {
    Source      Source
    SessionID   string
    Environment *Environment
}
```

这里的 `Session` 表示某个平台 Session 当前解析出的 hev 状态。`Environment == nil` 表示该 Session 没有启用 hev。Session 只绑定一个 Environment ID；读取状态时再获取该 Environment 的最新内容，不持久化 Environment snapshot。

## 状态目录

目录前缀由 Core 中的常量固定，adapter 和模型都不能传任意状态目录。平台标准环境变量只用于确定该平台自己的 home：

| source | 状态目录 |
| --- | --- |
| `standalone` | `~/.hev/` |
| `dsh` | `$DSH_HOME/.hev/`，默认 `~/.dsh/.hev/` |
| `claude-code` | `$CLAUDE_CONFIG_DIR/.hev/`，默认 `~/.claude/.hev/` |
| `codex` | `$CODEX_HOME/.hev/`，默认 `~/.codex/.hev/` |
| `opencode` | `$OPENCODE_CONFIG_DIR/.hev/`；否则 `$XDG_CONFIG_HOME/opencode/.hev/`，默认 `~/.config/opencode/.hev/` |

每个 source 的目录包含：

- `environments.json`：该平台的 Environment。每条 Environment 持久化自己的 source。
- `session-bindings.json`：该平台的 `sessionId -> environmentId` 绑定。文件已经由 source 目录隔离，因此 binding 不重复保存 source。

Core 必须拒绝 source 不受支持、Environment source 与当前 Store source 不一致、Session 绑定指向不存在 Environment 等情况。不同 source 下相同的 Session ID 和 Environment 名称互不影响。

## 调用职责

以 DSH 为例：

```text
当前 Agent
  -> DSH adapter 读取 agent.session.id
  -> adapter 固定传 --source dsh 和 --session-id <id>
  -> Go Core 解析 ~/.dsh/.hev/ 下的平台状态
  -> 返回 source + sessionId + 当前 Environment
```

Agent-facing Tool 不暴露 `source`、`sessionId` 或文件路径：

- `source` 由安装在哪个平台的 adapter 决定。
- `sessionId` 由当前 Tool 调用的宿主上下文决定。
- 状态目录由 Core 的 source 映射决定。

这样既允许 Agent 管理当前 Session 和 Environment，又不会让模型伪造其他平台、其他 Session 或任意文件目录。未来 Claude Code、Codex 和 OpenCode adapter 只需要提供各自的 Session ID、固定 source 并接入本平台的 Skill 过滤点，不复制 Core 业务逻辑。
