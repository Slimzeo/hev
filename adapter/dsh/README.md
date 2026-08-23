# hev → dsh adapter (原型)

把一个 hev environment 组装成**某一个 agent 自己的** skill 集合:`/hev env activate <name...>` 调 hev CLI 拿到环境定义,然后把一个 `SkillProvider` 注册进**那个 agent 的** `ctx.skills` 层。会话中途切换即时生效,不需要 recompose agent-preset,也不影响同进程的其他会话。

## 跑测试

dsh 当前没有构建产物(`lib/` 不存在),所以本原型直接指向 dsh 的源码,并借用 dsh 的 vitest:

```bash
cd ../../../deepseek-harness
npx vitest run --config ../hev/adapter/dsh/vitest.config.ts
```

`vitest.config.ts` 做两件事:把 `@deepseek-ai/*` 解析到 dsh workspace 的 `src/`,以及复用 dsh 的 `standardDecoratorPlugin()`(`dsh-commands` 用了标准 TS 装饰器)。

## 原型专属的临时手段(正式实现要去掉)

1. **`node_modules` 软链** 指向 dsh 的 `node_modules`,只为让 `typescript` / `vitest` 可解析。正式做法是 hev 自己的 `package.json` + 依赖 dsh 的**已发布**包。
2. **假 CLI** `tests/fixtures/fake-hev.mjs` 实现了《技术方案》§7 契约的 `env activate` / `env deactivate`,让测试不依赖真 Go 二进制。

## 端到端(`tests/e2e.spec.ts`)

不是手搭 context,而是:临时写一份真 `cordis.yml` → 过 Loader 引导 → `ctx.agentLoop.create()` 拿真 Agent → 脚本化 LLM 适配器驱动两个真 turn(第一轮模型发 `skill` 工具调用,第二轮用户用 `/secret-skill` 手势)→ **所有断言从 session log 读回**。

`auditFromLog(session)` 只用持久事件重建整个管控过程:

| 审计字段 | 来源事件 | 含义 |
|---|---|---|
| `decisions` | `command/done.text` | hev 对每个 skill 的裁决:`admitted` / `user-only` / `excluded` + 原因 |
| `offeredToModel` | `user/message` (`source.kind='skill-catalog'`) 的 `entries` | 那一步真正进入模型上下文的目录 |
| `readByModel` | `tool/call` (`name='skill'`) 的 `arguments` | 模型实际加载了哪些 |
| `readByUser` | `user/message` (`source.kind='skill-invocation'`) 的 `name` | 用户用 `/name` 手势加载了哪些 |
| `bodySources` | `tool/result` 与注入消息里的 `Base directory for this skill:` | 正文是从 env 的 realPath 解析出来的,不是默认 root |

**为什么裁决走 `command/done.text` 而不是自定义 session 事件**:dsh 的读路径拒绝解释含未知事件类型的日志(`session-persistence/src/coordinator.ts:1055-1065`),而 `ignorable` 标记生产者无法设置,库外插件的事件注册面在上游尚未开放。`command/done` 会把 handler 返回的 `text` 逐字记进日志,所以裁决集合用**结构化文本行**承载,任何 stock dsh 构建都能读。

## 已验证的行为(`tests/hev-plugin.spec.ts`)

- 会话中途 activate,下一步自动发**替换目录**;另一个在跑的会话完全不受影响。
- `mode: off` 的 skill 从模型目录消失,但仍能被用户 `/name` 手势加载(load 时从 candidate 取策略,不重读文件,所以 env 级 override 不会丢)。
- env 组内重名按**入参顺序**定胜负(`coding writing` 与 `writing coding` 结果相反)。
- CLI 失败(`ENV_NOT_FOUND`)时保留原环境,`/hev env deactivate` 回到 base。
- frontmatter `name` 与 CLI `identity.skillName` 不一致的 skill 被拒绝并告警;realPath 不存在的 skill 降级为告警而非整体失败。

## 谁解析 SKILL.md:按精确路径复用 dsh 的解析器

adapter 持有**一个没有任何 root 的** `FileSystemSkillProvider`(`includeDefaultRoots: false` + `customSkillDirs: []` + `watch: false`),只调它的 `get(candidate)`。`get()` 只读 `candidate.locator.path` 指定的那一个文件(`skill-filesystem/src/index.ts:206-222`),不经过 root 发现,所以:

- **不扫描任何目录**,realPath 的同级 skill 永远不可见(测试里 `store/` 下就放了 `actual-name` 等同级 skill,目录中不会出现);
- 没有 watcher,没有全量重读;
- frontmatter 语义(`disable-model-invocation`、`user-invocable`、legacy key 拒绝、`metadata`)与 dsh 完全一致,hev 不维护第二套方言。

被否掉的做法(留档):把 env 的逻辑目录当 `customSkillDirs` 的 root。它其实只扫 env 目录、不扫 realPath 的父目录,但要求 hev 物化并同步一份符号链接目录,而且每次缓存未命中会把该目录下所有 SKILL.md 全量重读、为每个 root 挂 watcher。不值得。

残留耦合:依赖 dsh 导出的 `FileSystemSkillProvider` 类。dsh 处于 pre-release,明确允许自由重命名/重组包;更干净的长期解法是请上游导出一个纯 frontmatter 解析函数。

## 尚未实现

- `mode: always` / `interval` 的注入(需要 adapter 自己的 `agent/pre-step` 监听 + 轮次计数)。
- 会话恢复时重放 activate(阻塞项:SessionBinding 存哪儿 —— dsh 的 session log 不接受库外插件的自定义事件类型)。
- env 内容变更的感知(CLI 契约目前没有 watch/通知面)。
- 默认 roots 的隔离(需要在 agent-preset 里关掉 `skill-filesystem`;本原型是叠加模式)。
