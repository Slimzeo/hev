# HEV DSH 双插件开发与接入规范

(不是很规范，仅作为建议advice)

> 依据 DeepSeek Harness `0.1.1-rc.2`（源码提交 `b150a55`）。DSH 仍处于 Developer Preview，升级可能包含破坏性变更。

## 方案定位

HEV 在 DSH Host 层实现两个 Cordis 插件：

1. **`@hev/dsh-plugin`**：HEV 主插件，包含原计划中 Go CLI 与 adapter 的全部能力，包括领域模型、JSON 持久化、Environment/Skill 管理、SessionBinding 和 `/hev` 命令。
2. **`@hev/dsh-skill`**：基于官方 `@deepseek-ai/dsh-skill` 修改的替代实现，继续提供 `ctx.skills`，但根据当前 Session/Agent 绑定的 Environment 过滤可见 Skill。

不再存在独立 Go CLI、子进程调用或 CLI/Plugin JSON IPC 契约。

建议依赖方向保持单向：

```text
@hev/dsh-plugin（提供 ctx.hev）
             ↓ inject: ['hev']
@hev/dsh-skill（提供兼容的 ctx.skills）
             ↓
官方 skill-filesystem / tool-skill 等消费者
```

HEV 主插件不应反向依赖 `ctx.skills`。Environment 变更通过 `ctx.hev` 或类型化事件通知替代 Skill Registry 清理缓存并发出 `skills/change`，避免循环依赖。

## 开发规范

### HEV 主插件

- 建议使用 `Service` 插件提供稳定的 `ctx.hev`，并用 TypeScript 声明合并补充 Context 类型。
- 内部直接实现 Environment、Skill、SessionBinding、Repository 和应用服务；持久化目录默认使用 `~/.hev`，可通过 Config schema 配置。
- 通过 `ctx.commands.register()` 注册 `/hev`。它是人类命令，不应注册成模型工具；命令名必须匹配 `[a-z][a-z0-9_-]*`。
- SessionBinding 以稳定的 `SessionId` 为键；未绑定时返回 `base` Environment。恢复同一 Session 时直接读取绑定，不依赖进程内“当前环境”。
- 同一 Session 的 activate/deactivate/update 必须串行化；校验、持久化和内存发布应保持原子性，失败时保留旧绑定。
- JSON 存储保留 `schemaVersion`；新增可选字段可保持版本，删除字段、改变类型或语义必须升级版本并提供迁移。
- 所有可部署参数通过 Schemastery `Config` 声明；非法配置在加载阶段失败。

### 替代 `dsh-skill` 插件

- 必须继续注册同一个 Cordis 服务键 `ctx.skills`，不得另建平行 Skill Registry。
- 应尽量从官方实现 fork，并保持以下公开契约兼容：
  - `registerProvider()`、`register()`、`snapshot()`、`list()`、`get()`；
  - `skills/change` 事件；
  - provider 的 `list/get`、locator、rank、缓存、失效和 disposer 行为；
  - `SkillSummary`、`SkillDefinition`、`SkillInvocationPolicy` 语义。
- 过滤依据必须来自每次调用的 `SkillViewOptions.scope` 所对应的 Agent/Session，不得使用进程级全局 active environment。
- `snapshot/list/get` 必须应用同一套过滤规则；禁止出现“目录不可见但按名称仍可 get”的绕过。
- 保留官方 scope 合并和同名裁决规则，再应用 HEV Environment 过滤；不要破坏 agent scope 对全局层的遮蔽关系。
- Skill 名继续使用 kebab-case。模型和用户入口仍分别遵守 `modelInvocable`、`userInvocable`，`get()` 本身不替消费方做权限判断。
- Environment 或 Skill 关联改变后，应使相关缓存失效并发出 `skills/change`，使 `tool-skill` 更新模型可见目录。
- `mode=off` 必须从目录和加载路径中排除；`auto/always/interval` 的自动执行属于 HEV 策略，不应混入基础 Skill Registry 的发现契约。
- 需要定期对照上游 `@deepseek-ai/dsh-skill` 更新。官方 API、缓存、校验或 scope 语义变化时，替代实现必须同步并重新做兼容测试。

### 通用要求

- 两者均为 ESM TypeScript Host 插件，导出明确的 `name`、`inject` 和入口。
- 依赖通过 `inject` 表达，不依赖 `cordis.patch.yml` 行顺序。
- 注册、监听器和子插件使用 Cordis effect 生命周期；watcher、timer、文件句柄等资源放入 `ctx.effect()` 并返回 disposer。
- 异步读取应接受并及时响应 `AbortSignal`；卸载和 HMR 后不得保留旧 Session 状态或重复注册。
- 当前为预览版本，DSH/Cordis peer dependency 应锁定已验证版本；升级后必须运行兼容测试。

## Bundle 与接入

建议使用一个可安装 Bundle 管理两个插件包：

```text
packages/
├── dsh-plugin/       # @hev/dsh-plugin，同时声明 dsh.bundle
└── dsh-skill/        # @hev/dsh-skill，普通依赖包
```

`@hev/dsh-plugin` 的 `package.json` 声明：

```json
{
  "name": "@hev/dsh-plugin",
  "version": "0.1.0",
  "type": "module",
  "main": "./lib/index.js",
  "types": "./lib/types/index.d.ts",
  "files": ["lib", "cordis.patch.yml"],
  "dependencies": {
    "@hev/dsh-skill": "0.1.0"
  },
  "dsh": {
    "bundle": { "patch": "./cordis.patch.yml" }
  }
}
```

Bundle 的 `cordis.patch.yml` 同时插入 HEV 服务并替换官方 `skill` 行：

```yaml
- insert:
    - id: hev
      name: '@hev/dsh-plugin'

- id: skill
  name: '@hev/dsh-skill'
```

说明：

- `hev` 是新插件实例 ID。
- `skill` 必须沿用 dsh-base 中官方 Skill Registry 的实例 ID，才能覆盖 `@deepseek-ai/dsh-skill`，而不是并存两个 `ctx.skills` 提供方。
- `@hev/dsh-skill` 在源码中声明 `inject = ['hev']`；即使配置行写在前面，Cordis 也会等待服务就绪。
- 官方 `skill-filesystem`、`skill-badge` 和 `tool-skill` 行可以保留，它们继续通过兼容的 `ctx.skills` 工作。

安装并验证：

```bash
dsh plugin --profile web add @hev/dsh-plugin
dsh --profile web --dump-config
dsh --profile web
```

`--dump-config` 中应同时满足：存在 `id: hev`，且 `id: skill` 的 `name` 已变为 `@hev/dsh-skill`。

本地开发可安装 workspace/link 包，或用一个绝对路径 patch 同时指向两个构建入口；不要让开发 patch 同时保留官方 `id: skill` 实例。

## 注意事项

- Bundle 层只负责组装；两个插件都运行在 Node Host。配置优先级为 bundle → profile patch → `$DSH_HOME/cordis.patch.yml` → `--patch`，后层仍可再次覆盖 `id: skill`。
- 按 `id` 修改插件行时，`config` 是整体替换而非深度合并；替换官方行时应检查上游是否新增必要配置。
- `agent/session-start` 是同步通知，不能等待异步恢复。最佳方案是启动时预加载 HEV 存储，并让 Skill Registry 在每次带 scope 查询时按 SessionId 读取绑定；若恢复必须异步完成，则必须接入 Agent create/resume 的 `setup(agentCtx)` 事务。
- 替换核心 Service Definition 的兼容风险高于普通 provider 插件。必须覆盖官方消费者 `dsh-skill-filesystem`、`dsh-tool-skill`、HMR 和多 Session 并发场景。
- 插件运行在宿主进程且位于 agent 沙箱之外，应视为受信任代码。文件路径必须规范化，持久化应采用临时文件加原子替换，并防止并发写覆盖。
- 最低测试范围：官方 `ctx.skills` API 契约、scope 隔离、list/get 一致性、base 默认环境、激活切换、缓存失效、Session 恢复、并发写、卸载/HMR，以及真实 profile 装配。

## 参考文档

- [打包与安装插件](https://github.com/deepseek-ai/deepseek-harness/blob/b150a551b8d465e31e418e1b2eaf5e79bbb7d28e/docs/user/develop/basic/publish.zh.md)
- [Cordis 生命周期与 Effect](https://github.com/deepseek-ai/deepseek-harness/blob/b150a551b8d465e31e418e1b2eaf5e79bbb7d28e/docs/cordis-tutorial/02-lifecycle-and-effects.zh.md)
- [服务与依赖](https://github.com/deepseek-ai/deepseek-harness/blob/b150a551b8d465e31e418e1b2eaf5e79bbb7d28e/docs/user/develop/framework/service.zh.md)
- [官方 Skill Registry 契约](https://github.com/deepseek-ai/deepseek-harness/blob/b150a551b8d465e31e418e1b2eaf5e79bbb7d28e/packages/skill/skill/README.zh.md)
- [官方 filesystem provider](https://github.com/deepseek-ai/deepseek-harness/blob/b150a551b8d465e31e418e1b2eaf5e79bbb7d28e/packages/skill/skill-filesystem/README.zh.md)
- [命令注册表](https://github.com/deepseek-ai/deepseek-harness/blob/b150a551b8d465e31e418e1b2eaf5e79bbb7d28e/packages/interaction/commands/README.zh.md)
- [Agent 生命周期与 setup](https://github.com/deepseek-ai/deepseek-harness/blob/b150a551b8d465e31e418e1b2eaf5e79bbb7d28e/packages/core/agent/README.zh.md)
