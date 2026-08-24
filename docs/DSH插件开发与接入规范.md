# HEV DSH 双插件开发与接入规范

(不是很规范，仅作为建议advice)

> 依据 DeepSeek Harness `0.1.0-rc.8`（源码提交 `141eb6f`）。DSH 仍处于 Developer Preview，升级可能包含破坏性变更。

## 方案定位

HEV Core 是独立 Go CLI，通过 `$HOME/.hev/environments.json` 持久化 Environment 当前记录。DSH Adapter 由两个 Cordis 运行时插件和一个安装 bundle 组成：

1. **`@hev/dsh-runtime`**：通过无 shell 子进程调用 HEV CLI，提供 `ctx.environment`，并在 `@deepseek-ai/dsh-commands` 存在时注册 `/hev`。
2. **`@hev/dsh-skill`**：继承官方 `@deepseek-ai/dsh-skill` 的 `SkillRegistry`，继续提供 `ctx.skills`，并按精确 live Agent/Session 的当前 Environment 过滤原生 Skill winner。
3. **`@hev/dsh-adapter`**：只声明 bundle 和 workspace 依赖，不是第三个运行时插件。

依赖方向保持单向：

```text
HEV Go CLI + JSON Store
          ↑ subprocess JSON v1
@hev/dsh-runtime（提供 ctx.environment 与 /hev）
          ↑ inject: ['agents', 'environment']
@hev/dsh-skill（提供兼容的 ctx.skills）
          ↑
官方 skill-filesystem / tool-skill 等消费者
```

## 开发规范

### `@hev/dsh-runtime`

- 使用 `Service` 插件提供 `ctx.environment`，并用 TypeScript 声明合并补充 Context 类型。
- `executable` 是唯一配置项，默认值为 `hev`；通过 argv 直接调用，不经过 shell。
- 通过 `ctx.commands.register()` 注册 `/hev`。它是人类命令，不注册成模型工具。
- MVP 只公开 `env create`、`skill add` 和 `env use`；Plugin 固定追加 `--output json`，先校验 JSON v1 envelope 和完整结果，再使用返回数据。
- `use(agent, refs)` 以精确 live `agent.session` 对象为键保存 CLI 返回的规范 Environment ID。失败不得替换原选择。
- `current(session)` 对未显式 `use` 的 Session 执行 `hev env use base --output json`；对已选择的 Session 使用保存的规范 ID。每次调用都读取最新 Store 记录，并将返回的规范 ID 写回同一个 Session。
- 选择只保存在进程内 `WeakMap<Session, readonly EnvironmentId[]>`，不同 `Session` 对象即使 ID 相同也互不共享。

### `@hev/dsh-skill`

- 必须继续注册同一个 Cordis 服务键 `ctx.skills`，不得另建平行 Skill Registry。
- 继承官方实现，保留 provider 注册、scope 合并、同名 winner、校验和 Skill 正文加载。
- 过滤依据来自每次调用的 `SkillViewOptions.scope`。只有与 `ctx.agents.list()` 中对象完全相同的 live Agent 才进入 HEV 过滤；找不到精确 Agent 时保留原生 view。
- 精确 live Agent 即使未显式 `use`，也必须通过 `current()` 使用 `base`，不得回退到原生 view。
- `snapshot()` 与 `get()` 使用同一 allow-set；父类 `list()` 动态调用 override 后的 `snapshot()`。
- allow-set 只包含 current snapshot 中 `policy.kind === 'auto'` 的 `skillKey`。`off`、未列入以及没有原生 winner 的 key 均不可见。HEV CLI 不查询原生 Registry，因此缺少原生 Skill 不阻止 `use`。

### JSON Store

首次读取缺失的 Store 文件，或读取到空的 `environments` 数组时，`JsonEnvironmentStore` 自动持久化：

```json
{
  "schemaVersion": 1,
  "environments": [
    {
      "id": "env_base",
      "name": "base",
      "revision": 1,
      "skills": []
    }
  ]
}
```

`base` 是普通 Environment。`revision` 只记录当前记录的新旧：创建值为 `1`，每次成功更新递增；`use` 与 `current()` 始终读取最新记录，不按 revision 选择或回退。

## Bundle 与接入

根 `adapter/dsh/package.json` 声明 `@hev/dsh-adapter` bundle，并依赖两个运行时包。`cordis.patch.yml` 必须先禁用 base 中原生 Skill Registry，再插入 HEV 插件：

```yaml
- id: skill
  name: '@deepseek-ai/dsh-skill'
  disabled: true

- insert:
    - id: hev-runtime
      name: '@hev/dsh-runtime'

    - id: hev-skill
      name: '@hev/dsh-skill'
```

`@hev/dsh-adapter` 不对应 Loader 行。`skill` 保留原包名并禁用，`hev-skill` 提供唯一启用的 `ctx.skills`；`hev-runtime` 在 `hev-skill` 前插入，且后者通过 `inject = ['agents', 'environment']` 声明依赖。

### 本地 checkout 接入

在 HEV 根目录构建 CLI 与 Adapter：

```bash
mkdir -p .local/bin
go build -o "$PWD/.local/bin/hev" ./cmd/hev
pnpm --dir adapter/dsh install
pnpm --dir adapter/dsh typecheck
pnpm --dir adapter/dsh test
pnpm --dir adapter/dsh build
```

DSH profile 无法仅从根 workspace link 解析两个子包。本地开发必须一次安装两个 plain dependency 和根 bundle，根 bundle 放在最后便于辨认：

```bash
cd <deepseek-harness-root>
pnpm dsh plugin --profile web add \
  /absolute/path/to/hev/adapter/dsh/packages/runtime \
  /absolute/path/to/hev/adapter/dsh/packages/skill \
  /absolute/path/to/hev/adapter/dsh
```

发布到 registry 后，安装 `@hev/dsh-adapter` 会通过其 dependencies 安装两个子包，无需分别添加。

在 `$DSH_HOME/profiles/web/cordis.patch.yml` 设置 CLI 绝对路径（`$DSH_HOME` 默认是 `~/.dsh`）：

```yaml
- id: hev-runtime
  config:
    executable: /absolute/path/to/hev/.local/bin/hev
```

检查组合结果：

```bash
pnpm dsh --profile web --dump-config
```

输出必须包含禁用的 `id: skill` / `name: '@deepseek-ai/dsh-skill'`，以及启用的 `hev-runtime` 和 `hev-skill`。随后启动 Web profile：

```bash
pnpm dsh web --no-open
```

### 行为验证

新建 Session 后，普通空 `base` 应使 Skill 列表为空。在同一 Session 中执行：

```text
/hev env create review
/hev skill add <native-auto-skill> --env review --policy auto
/hev skill add <native-off-skill> --env review --policy off
/hev env use review
```

`<native-auto-skill>` 应可见且可加载，`<native-off-skill>` 应不可见。也可以加入一个不存在于原生 Registry 的 kebab-case key；它不会阻止 `/hev env use review`，但不会出现在结果中。

再新建一个 Session：它仍默认使用空 `base`，不会继承第一个 Session 的 `review`；回到第一个 Session 时，`review` 的 auto/off 结果保持不变。

## 注意事项

- Bundle 层只负责组装；两个插件运行在 Node Host，HEV Core 运行在 CLI 子进程。
- Profile patch 的 `config` 是整体替换，不是深度合并；覆盖 `hev-runtime` 时必须保留该行需要的全部配置。
- 插件在 agent 沙箱之外运行，应把 `executable` 指向受信任的 HEV 二进制文件。
- DSH/Cordis peer dependency 应锁定已验证版本；升级后重新运行 typecheck、package tests 和真实 profile 装配验证。

## 参考文档

- [打包与安装插件](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/docs/user/develop/basic/publish.zh.md)
- [Cordis 生命周期与 Effect](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/docs/cordis-tutorial/02-lifecycle-and-effects.zh.md)
- [服务与依赖](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/docs/user/develop/framework/service.zh.md)
- [官方 Skill Registry 契约](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/packages/skill/skill/README.zh.md)
- [命令注册表](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/packages/interaction/commands/README.zh.md)
