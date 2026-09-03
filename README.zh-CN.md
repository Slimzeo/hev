# hev

> 面向 AI Agent Harness 的、按 Session 生效的 Skill Environment。

[English](README.md)

hev 允许一个 Agent Harness 继续保留其原生、全局已安装的 Skill，同时让每个宿主 Session 拥有一个更小、更明确的**模型自动发现**目录。Environment 是一组 Skill key 与调用策略组成的 allowlist；它不是第二套 Skill 安装器、文件系统隔离层，也不是仅靠提示词约定的软规则。

> [!IMPORTANT]
> hev 当前处于 alpha。DeepSeek Harness（DSH）adapter 已实现，但 `@owariband/hev-dsh-plugin` 尚未发布到 npm。完成首发后，下方 npm 安装命令才可用。

## 为什么需要 hev

Agent 安装久了会不断累积 Skill。让每一个已安装 Skill 都进入每一轮模型上下文，会增加上下文压力，也让自动选择变得嘈杂。把 Skill 拆进不同目录虽然能减少列表，却会迫使宿主忘掉自己的原生发现模型；只在 prompt 里要求模型“不要使用某些 Skill”也不是可执行的边界。

hev 选择更小的一步：保留宿主的原生 catalog，再在宿主 Skill Registry 的边界上，为当前 Session 投影一份 allowlist。

```text
DSH 原生 Provider
  └─ 发现 candidate，并选出 native winner                保持不变
       └─ hev 解析当前 Session 选择的 Environment
            └─ 仅保留 policy = auto 的绑定
                 └─ 模型自动可见的 Skill catalog          被过滤

用户显式调用 /skill-name 仍保留为 DSH 原生路径。
```

## 一个最小例子

假设 DSH 已经可以发现 `github`、`paper-explain` 等全局安装的 Skill。一个 Session 可以选择面向 review 的 Environment，而不影响其他 Session：

```text
# Session A
/hev env create review
/hev skill add github review --policy auto
/hev env use review

# Session A 的模型自动发现目录
hev-guide, github

# Session B 没有启用 hev
仍使用原生 DSH catalog
```

`off` 会在 Environment 中记录一个 Skill，但不会让它进入自动发现目录。用户仍可以通过 DSH 原生 `/skill-name` 显式调用一个全局已安装、允许用户调用的 Skill；hev 不会通过 Environment 组合或平行 Registry 绕过这条 catalog 边界。

## 当前能力边界

- **已实现宿主：** DeepSeek Harness（DSH）。
- **选择模型：** 每个 host Session 只选择一个 Environment。未选择的 Session 不由 hev 管理，继续使用 DSH 原生视图。
- **持久化：** Go Core 把 DSH 状态存放在 `$DSH_HOME/.hev`，默认是 `~/.dsh/.hev`。
- **隔离：** adapter 固定 `source=dsh` 并从 DSH 读取当前调用 Session；模型不能选择其他 source、Session ID 或状态目录。
- **当前策略：** `auto` 与 `off`。

hev 当前面向本地单用户开发。共享状态授权、审计策略、更多调用策略和非 DSH adapter 都仍是后续工作。

## 架构

```text
Go Core
  Environment 记录 + policy + 按 source 隔离的 Session binding
        ▲ CLI v2 JSON
DSH adapter
  hev-runtime: 精确 Session 访问与 /hev 命令
  hev-skill-registry: native winner -> Environment allowlist
  hev-tool: 面向 Agent 的 Environment 操作
```

Core 有意不存储 DSH 的 `SkillCandidate`、Provider locator、物理 Skill 路径或 Skill 正文。DSH 仍负责 Provider discovery、winner 选择、校验和延迟加载；adapter 只负责为一个 live Session 解释这份原生 catalog。

## 计划中的安装方式

首个 npm 版本发布后：

```bash
npx @deepseek-ai/dsh plugin --profile web add @owariband/hev-dsh-plugin@latest
npx @deepseek-ai/dsh web
```

包内会包含 `hev` 可执行文件和三个 DSH plugin entry。用户无需单独安装 Go 或配置额外 launcher。

## 命令

```text
/hev env create <name>
/hev env rename <id-or-name> <new-name>
/hev env delete <id-or-name>
/hev env list
/hev env use <id-or-name>
/hev env quit
/hev env status
/hev skill add <skill-key> <env-name> [env-name...] [--policy auto|off]
/hev skill remove <skill-key> <env-name> [env-name...]
/hev skill list [id-or-name]
/hev skill list --global
```

DSH 包也会向当前 Agent 暴露等价的 `hev_*` tools。涉及 Session 的 tool 会从 DSH 取得调用方 Session，而不接受模型传入的 Session ID。

## 从源码开发

Go Core 需要 Go 1.24 或更高版本：

```bash
go build -o .local/bin/hev ./cmd/hev
go test ./...
./.local/bin/hev --help
```

当前 DSH adapter 开发要求 `hev` 与 `deepseek-harness` 是同级目录：

```text
workspace/
├── hev/
└── deepseek-harness/
```

使用 Node.js 22.19 或更高版本，并使用仓库锁定的 pnpm：

```bash
corepack pnpm@11.7.0 --dir adapter/dsh install
corepack pnpm@11.7.0 --dir adapter/dsh typecheck
corepack pnpm@11.7.0 --dir adapter/dsh test
corepack pnpm@11.7.0 --dir adapter/dsh run release:check
```

`release:check` 会运行 adapter 测试、构建各平台二进制、打包 npm artifact、导入 plugin entry，并调用包内的 `hev` 可执行文件。具体本地 profile 接入方式见 [DSH adapter guide](adapter/dsh/README.md)。

## 项目状态

[roadmap](wiki/roadmap.md) 将已实现边界与后续工作明确区分。提交 PR 前，请运行上面的 Go 测试与相应 DSH 检查。

## License

hev 使用 [MIT License](LICENSE)。
