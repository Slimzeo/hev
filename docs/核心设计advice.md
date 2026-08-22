# Skill Env 核心设计建议

> 状态：团队讨论稿  
> 日期：2026-08-22  
> 目的：收敛项目核心定位、系统边界和一期实现范围。本文件不是最终技术规范，未决项需要团队评审后再写入正式开发方案。

## 一、核心结论

Skill Env 不应以 Python 虚拟环境管理器为核心，也不应重复 Codex、Claude Code、TraeX 等 Harness 已经提供的 Skill/Plugin 安装与发现能力。

项目建议定位为：

> 面向通用 Agent Harness 的、本地优先的可复现 Agent Context Environment 管理器。

它负责把 Skill、Plugin、MCP、Hooks、模型配置、权限和可选运行条件组织成一个可命名、可锁定、可切换、可审计的环境，并通过 Host Adapter 将该环境物化为不同 Harness 能识别的目录和配置。

最重要的边界是：

- Skill Env 决定“这个环境应该包含什么”；
- Host Adapter 决定“如何表达给具体 Harness”；
- Codex、Claude Code、TraeX 等 Harness 负责真正发现、加载、执行、审批和沙箱；
- Python venv、容器等只是可选 Runtime Provider，不是 Environment 的必选组成部分。

## 二、为什么仍然需要 Skill Env

当前主流 Harness 已经分别具备一部分 Context 管理能力，例如：

- 从用户级、仓库级或系统级目录发现 Skill；
- 安装、启用和禁用 Plugin；
- 加载项目指令、MCP、Hooks 和权限配置；
- 管理本地、Worktree 或云端执行环境。

但这些能力主要由各 Harness 独立实现，仍缺少统一的：

1. 命名环境：一次切换完整的 Agent Context，而不是逐项修改配置；
2. 精确锁定：记录实际 Skill/Plugin 版本、内容摘要和依赖；
3. 跨 Harness 表达：同一个逻辑环境可以解析为 Codex、Claude Code、TraeX 等宿主配置；
4. 差异预览：切换前展示 Skill、工具、权限和配置变化；
5. 可复现与审计：明确某次结果是在什么模型、Harness、工具和权限条件下产生；
6. 安全升级与回滚：环境更新失败时不破坏当前可用状态。

因此项目的差异不应只是“安装或启停 Skill”，而应是：

```text
可复现 Agent Context
= resolved Skills / Plugins
+ MCP / Hooks
+ model / harness configuration
+ permissions
+ optional runtime
```

## 三、系统总体结构

```text
EnvironmentSpec
    用户声明想要什么
        │
        ▼
EnvironmentResolver
    解析版本、来源、内容摘要、兼容性和宿主能力
        │
        ▼
EnvironmentLock
    固定本次环境的精确结果
        │
        ▼
ReconcilePlanner
    比较 current 与 desired，生成变更计划
        │
        ▼
HostAdapter
    转换为目标 Harness 能识别的结构
        │
        ▼
MaterializedEnvironment
    实际目录、配置、链接和启动方式
        │
        ▼
Harness
    发现、加载并执行 Agent Context
```

建议的工程结构：

```text
skill-env Core
├── CLI / Local API
├── Environment registry
├── Resolver
├── Lockfile
├── Artifact Store
├── Reconcile planner
├── Runtime providers（可选）
└── Host adapters
    ├── Codex adapter / launcher
    ├── Claude Code adapter
    └── TraeX adapter
```

一期可以只实现 CLI，不必先运行常驻本地服务。Core 应设计为可复用库，后续 Plugin、桌面 UI 或本地服务都调用相同的 Core。

## 四、核心领域模型

### 4.1 EnvironmentSpec

用户可编辑的声明模型，表达期望状态，不保证所有引用已经解析为精确制品。

示例：

```yaml
kind: AgentEnvironment
schema_version: 1

metadata:
  name: paper-research
  description: 论文检索、取证和分析环境

skills:
  - ref: paper-analysis@^1.4
    mode: auto
  - ref: citation-check@0.3.0
    mode: explicit

plugins:
  - ref: lark-suite@2.1.0

tools:
  - ref: openai-docs
  - ref: academic-search

hosts:
  codex:
    reasoning_effort: high
  traex:
    agent: paper-researcher

permissions:
  network: true
  filesystem: workspace-write

runtime:
  provider: inherit
```

需要注意：`tools` 最终可能被拆成 Plugin、MCP 和 Harness 内置 Tool 三类。当前示例保留统一入口，但 Resolver 必须在 Lock 中记录其真实类型和来源。

### 4.2 EnvironmentLock

机器生成的精确解析结果，不建议用户手工编辑。它至少应固定：

- EnvironmentSpec 的摘要；
- Skill/Plugin 的精确版本；
- 来源与内容 digest；
- MCP/Hook 的精确配置摘要；
- 目标 Harness 与兼容性结果；
- 权限和 Runtime 的解析结果；
- lock schema 版本。

示意：

```yaml
lock_version: 1
environment: paper-research
spec_digest: sha256:spec123

artifacts:
  - kind: skill
    id: paper-analysis
    version: 1.4.2
    source: github:example/paper-analysis
    digest: sha256:skill123
  - kind: plugin
    id: lark-suite
    version: 2.1.0
    source: marketplace:lark-suite
    digest: sha256:plugin123

targets:
  codex:
    compatible: true
    adapter_version: 1
```

### 4.3 Artifact

Artifact 表示不可变的 Skill、Plugin 或其他可分发制品。

逻辑版本身份和真实内容身份必须分开：

```text
ArtifactIdentity
= kind + publisher + name + version

ArtifactRevision
= ArtifactIdentity + content_digest
```

同一个逻辑版本正常情况下只能对应一个内容 digest。如果发现同名同版本对应不同内容，应报告制品污染或来源冲突，不能静默覆盖。

### 4.4 EnvironmentBinding

环境对 Artifact 的使用方式属于 Environment，而不属于共享 Artifact：

```text
EnvironmentBinding
├── artifact revision
├── enabled
├── invocation mode
├── order / priority（确认确有需要后再保留）
├── environment-local configuration
└── host-specific override
```

同一份 Skill 制品可以被多个环境引用，但执行模式和本地配置可以不同。

### 4.5 MaterializedEnvironment

表示某个 EnvironmentLock 针对某个 Harness 的实际物化结果，例如：

```text
MaterializedEnvironment
├── environment id
├── lock digest
├── host id
├── adapter version
├── root path
├── generated files
├── linked artifacts
└── launch descriptor
```

它是派生状态，可以根据 Lock 和 Artifact Store 重新生成，不是用户意图的权威来源。

## 五、Host Adapter

Host Adapter 是跨 Harness 能力成立的关键，当前开发方案中这一层尚未明确设计。

建议最小接口：

```text
HostAdapter
├── probe()          检测 Harness 版本和当前能力
├── validate(lock)   检查目标环境是否可表达
├── plan(lock)       生成物化计划和差异
├── materialize()    写入或链接目标目录与配置
├── launch()         启动新的 Harness 会话
└── inspect()        检查实际状态是否与 Lock 一致
```

Adapter 不应悄悄降低能力。例如目标 Harness 不支持某种 Hook、权限或调用模式时，应返回：

```text
supported
unsupported
degraded（需要用户明确接受）
```

### 5.1 Codex Adapter 的可能实现

Codex 已经负责 Skill/Plugin/MCP/Hook 的最终发现与执行。Adapter 可以利用：

- repo/user/admin/system Skill 路径；
- symlink Skill 目录；
- `config.toml` 中的 Skill、MCP、Hook 和权限设置；
- 项目级 `.codex` 配置；
- 独立 `CODEX_HOME`；
- Plugin marketplace 和 Plugin manifest；
- 新会话启动时重新加载 Context。

一期优先采用启动前物化：

```bash
skill-env run paper-research -- codex
```

而不是承诺在一个已经运行的 Codex 会话中硬切换全部 Context。

### 5.2 其他 Adapter

Claude Code、TraeX 等 Adapter 应遵循相同逻辑，但分别生成其原生目录和配置。Core 不应依赖某个 Harness 的文件命名或配置结构。

## 六、Plugin 形态

项目可以提供 Plugin，但不建议把全部实现封装在 Plugin 内。

Plugin 适合承担：

- 环境创建、查询、差异预览和切换引导；
- 调用 Skill Env 本地 CLI 或 MCP 服务；
- 在 SessionStart 校验当前环境；
- 展示冲突、兼容性和权限提示；
- 将常用操作包装成 Agent 可调用的 Skill。

Plugin 不适合单独承担：

- 在当前会话中卸载已经被 Harness 发现的 Skill；
- 修改父进程环境变量；
- 保证所有 Harness 的硬隔离；
- 作为跨 Harness 唯一入口。

原因是 Plugin 本身已经运行在 Harness Context 内。多数 Harness 在会话开始时加载 Skill 和 Plugin，运行中的 Plugin 很难可靠重建父级 Context。

因此建议：

```text
Standalone Core + CLI/Launcher       权威实现
Codex/Claude/TraeX Plugin            交互入口与适配器
可选 Local MCP/API                   提供结构化调用能力
```

## 七、Sub-agent 环境注入流程

Sub-agent 默认继承父 Agent 的环境；需要专用能力时，通过预定义的环境绑定进行覆盖：

```text
解析并锁定父环境及所有子环境
→ Host Adapter 预先物化 Skill、MCP 和 Agent Profile
→ 启动主 Agent
→ 主 Agent 按 agent_type 创建 Sub-agent
→ Harness 应用对应的环境配置
→ SubagentStart Hook 校验 Environment ID
→ Sub-agent 执行并将结果返回主 Agent
```

建议提供三种绑定模式：

- `inherit`：完全继承父环境；
- `overlay`：继承父环境，并覆盖 Skill、模型或 MCP 配置；
- `isolated`：通过独立进程或 Worktree 启动，提供更强隔离。

Sub-agent 的权限不能超过父会话的权限上限。Hook 只负责校验和补充上下文，不能代替环境物化；需要硬隔离时，应由 Skill Env 启动独立 Agent 进程。

## 八、Rattler 思想如何复用

Rattler/Conda 不只是 Python 管理器。其通用思想可以用于 Skill Environment：

| Rattler | Skill Env |
| --- | --- |
| PackageCache | Artifact Store |
| PackageRecord | Artifact 元数据 |
| PrefixRecord | Environment 物化记录 |
| Prefix | Environment 可见视图 |
| EnvironmentYaml | EnvironmentSpec |
| Explicit/locked state | EnvironmentLock |
| current vs desired | 当前状态与目标状态 |
| Transaction | ReconcilePlan |
| link/copy/reflink | Artifact 物化策略 |
| Activator | Host Adapter / Launcher |
| path collision | Skill、命令和资源冲突 |

建议保留的思想：

1. 不可变共享 Artifact Store；
2. Environment 是对 Artifact 的物化视图；
3. Current → Desired → Plan → Apply；
4. 原子更新、失败恢复和回滚；
5. 激活状态属于会话，不属于 Environment 全局实体；
6. 声明意图与精确安装结果分离。

不应直接搬用：

- Conda channel、repodata 和 MatchSpec；
- Python `site-packages` 模型；
- `CONDA_PREFIX`、`CONDA_SHLVL`；
- Conda Solver 和 Prefix replacement；
- Conda 专属 Package Record。

因此 Rattler 调研应重新定位为“Artifact Store、Environment Materialization 与 Reconciliation 的设计参考”，而不是一期 Python 环境实现主线。

## 九、一期 MVP

一期只选择一个真实 Harness，建议优先选择团队最方便验证的 Codex 或 TraeX。

### 9.1 必须完成

```text
创建命名环境
→ 添加本地 Skill
→ 生成精确 Lock
→ 将 Skill 物化到目标 Harness 可发现的位置
→ 启动新的 Agent 会话
→ 验证 Agent 只能发现当前环境授权的 Skill
→ 切换到另一个环境并观察 Skill 集合变化
→ inspect / diff / remove
```

建议最小命令：

```bash
skill-env create <name>
skill-env add <name> <skill-path-or-ref>
skill-env lock <name>
skill-env inspect <name>
skill-env diff <name>
skill-env run <name> -- <harness-command>
skill-env remove <name>
```

### 9.2 一期暂不实现

- 自动选择数千个 Skill；
- 社区中心化推荐；
- Skill 自动 Evolver；
- 完整评测和 Promotion 平台；
- Conda/Python 包求解；
- 容器编排；
- 多 Harness 同时正式支持；
- 运行中会话的无损热切换；
- 常驻本地服务和复杂 UI。

### 9.3 一期验收标准

核心验收标准：

> `env-a` 与 `env-b` 锁定不同 Skill 集合；分别通过 Skill Env 启动 Agent 后，Harness 实际发现的 Skill 与各自 Lock 一致，并且切换不会修改共享 Artifact 内容或污染另一个环境。

同时验证：

- 重复 materialize 结果幂等；
- 中途失败不会破坏旧环境；
- Lock 与实际物化结果不一致时 `inspect` 能发现；
- 同名 Skill 或同版本不同 digest 会显式报错；
- 删除 Environment 不会误删仍被其他环境引用的 Artifact。

## 十、后续阶段

### 二期：更多 Context 类型

- Plugin、MCP、Hooks 纳入 Resolver 和 Lock；
- 可选 Python Runtime Provider；
- Host capability/degradation 检查；
- 环境导入、导出和共享；
- 第二个 Harness Adapter。

### 三期：治理与评测

- Skill Pack；
- provenance 和 advisory；
- case/eval/run metadata；
- challenger/incumbent；
- promotion、canary 和 rollback；
- 社区报告的环境兼容性证据。

## 十一、需要团队讨论的关键问题

1. 一期选择 Codex 还是 TraeX 作为第一个 Host Adapter？
2. EnvironmentSpec 是否允许模糊版本，还是一期只接受本地路径和精确版本？
3. 一期物化采用独立 `CODEX_HOME`、项目级目录还是临时启动目录？
4. Artifact Store 是正式权威存储，还是可从原始来源重建的 Cache？
5. Plugin、MCP、Hook 是否一期进入模型但暂不实现，还是完全推迟到二期？
6. 环境是否允许引用用户全局 Skill，还是必须将所有输入解析并锁定？
7. 同名 Skill 的冲突规则是什么：禁止、命名空间还是显式 alias？
8. 跨 Harness 无法等价表达的能力，应拒绝、降级还是允许 Adapter override？
9. `activate` 是否保留，还是一期只支持更可靠的 `run <env> -- <command>`？
10. 项目名称 `skill-env` 是否仍准确，还是应突出 Agent Context/Profile？

## 十二、对现有开发方案的调整建议

1. 将 `Environment.prefix: PathBuf` 从核心必选字段改为物化路径或可选 Runtime 状态；
2. 引入 `EnvironmentSpec`、`EnvironmentLock` 和 `MaterializedEnvironment`；
3. 将 `SkillSharedCache` 明确为 `ArtifactStore` 或可重建 Cache；
4. 保留 `EnvironmentBinding`，删除 Skill 与 Environment 的双向事实维护；
5. 增加独立的 `HostAdapter` 层；
6. 将 Python/Rattler 从一期主线调整为后续 Runtime 和事务设计参考；
7. 将 Plugin 定义为 Core 的适配与交互入口，而不是唯一实现；
8. 先完成单 Harness 的端到端闭环，再扩展 Pack、Evolver 和社区治理。

## 十三、参考资料

仓库内已有材料：

- `docs/技术方案.md`
- `docs/模型设计advice.md`
- `docs/rattler/how_to_build_env_manager.md`
- `docs/skill/skill-workspace-env-idea.md`

相关官方资料：

- OpenAI Agent Skills：https://developers.openai.com/plugins/build/skills
- OpenAI Plugins：https://developers.openai.com/plugins/build/plugins
- OpenAI Plugin architecture：https://developers.openai.com/plugins/concepts/plugins
- Agent Skills specification：https://agentskills.io/specification
