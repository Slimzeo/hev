# hev DSH 单包双入口开发与接入规范

(不是很规范，仅作为建议advice)

> 依据 DeepSeek Harness `0.1.0-rc.8`（源码提交 `141eb6f`）。DSH 仍处于 Developer Preview，升级可能包含破坏性变更。

## 方案定位

hev Core 是独立 Go CLI，通过 `$HOME/.hev/environments.json` 持久化 Environment 当前记录。DSH Adapter 对用户只发布一个 bundle `@slimzeo/hev-dsh-plugin`，包内保留两个职责独立的 Cordis 入口：

1. **`@slimzeo/hev-dsh-plugin/hev-runtime`**：通过无 shell 子进程调用包内 hev CLI，提供 `ctx.environment`，并在 `@deepseek-ai/dsh-commands` 存在时注册 `/hev`。
2. **`@slimzeo/hev-dsh-plugin/hev-skill-registry`**：继承官方 `@deepseek-ai/dsh-skill` 的 `SkillRegistry`，继续提供 `ctx.skills`，并按精确 live Agent/Session 的当前 Environment 过滤原生 Skill winner。

依赖方向保持单向：

```text
hev Go CLI + JSON Store
          ↑ subprocess JSON v2
@slimzeo/hev-dsh-plugin/hev-runtime（提供 ctx.environment 与 /hev）
          ↑ inject: ['agents', 'environment']
@slimzeo/hev-dsh-plugin/hev-skill-registry（提供兼容的 ctx.skills）
          ↑
官方 skill-filesystem / tool-skill 等消费者
```

## 开发规范

源码继续按 plugin 分组，只有 `hev-runtime` 自己的实现细节在其目录内拆分：

```text
adapter/dsh/src/
  hev-runtime/
    index.ts          Cordis Service 入口、Session 选择与 /hev 命令
    environment.ts    Environment 数据与数字状态类型
    cli.ts            hev 子进程调用与 CLI v2 响应校验
    executable.ts     包内可执行文件定位
  hev-skill-registry/
    index.ts          ctx.skills 替换实现
```

不能把 `environment` 或 CLI client 提升成与 plugin 平级的源码目录；它们都由 `hev-runtime` 拥有。

### `hev-runtime` 入口

- 使用 `Service` 插件提供 `ctx.environment`，并用 TypeScript 声明合并补充 Context 类型。
- `executable` 是唯一可选配置项；默认解析 npm 包内与当前平台匹配的 `hev`，只在源码调试或测试时显式覆盖。通过 argv 直接调用，不经过 shell。
- 通过 `ctx.commands.register()` 注册 `/hev`。它是人类命令，不注册成模型工具。
- MVP 只公开 `env create`、`skill add` 和 `env use`；Plugin 固定追加 `--output json`，先按 [CLI v2 schema](../contracts/cli/v2/) 校验响应字段和完整结果，再使用返回数据。
- `use(agent, ref)` 只接受一个 Environment ID 或名称，并以精确 live `agent.session` 对象为键保存 CLI 返回的规范 Environment ID。失败不得替换原选择。
- `current(session)` 对未显式 `use` 的 Session 执行 `hev env use --output json`，由 Core 返回默认 Environment；对已选择的 Session 使用保存的规范 ID。每次调用都读取最新 Store 记录，并将返回的规范 ID 写回同一个 Session。
- 选择只保存在进程内 `WeakMap<Session, EnvironmentId>`。一个 Session 始终只有一个 current Environment；不同 `Session` 对象即使 ID 相同也互不共享。
- `skill add --env ... --env ...` 仍可一次修改多个 Environment，因为它是配置写入，不是 Session 激活或 Environment 组合。

### `hev-skill-registry` 入口

- 必须继续注册同一个 Cordis 服务键 `ctx.skills`，不得另建平行 Skill Registry。
- 继承官方实现，保留 provider 注册、scope 合并、同名 winner、校验和 Skill 正文加载。
- 过滤依据来自每次调用的 `SkillViewOptions.scope`。只有与 `ctx.agents.list()` 中对象完全相同的 live Agent 才进入 hev 过滤；找不到精确 Agent 时保留原生 view。
- 精确 live Agent 即使未显式 `use`，也必须通过 `current()` 使用 `base`，不得回退到原生 view。
- `snapshot()` 与 `get()` 使用同一 allow-set；父类 `list()` 动态调用 override 后的 `snapshot()`。
- allow-set 只包含 current Environment 中 `policy.kind === 'auto'` 的 `skillKey`。`off`、未列入以及没有原生 winner 的 key 均不可见。hev CLI 不查询原生 Registry，因此缺少原生 Skill 不阻止 `use`。

### JSON Store

首次读取缺失的 Store 文件，或读取到空的 `environments` 数组时，Go Core 的 JSON Store 自动持久化：

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

`adapter/dsh/package.json` 本身就是唯一发布和安装的 `@slimzeo/hev-dsh-plugin` bundle。两个实现入口通过同一包的 subpath export 提供，`cordis.patch.yml` 必须先禁用 base 中原生 Skill Registry，再插入两个 hev 行：

```yaml
- id: skill
  name: '@deepseek-ai/dsh-skill'
  disabled: true

- insert:
    - id: hev-runtime
      name: '@slimzeo/hev-dsh-plugin/hev-runtime'

    - id: hev-skill-registry
      name: '@slimzeo/hev-dsh-plugin/hev-skill-registry'
```

根包不对应额外 Loader 行。`skill` 保留原包名并禁用，`hev-skill-registry` 提供唯一启用的 `ctx.skills`；`hev-runtime` 在它之前插入，且后者通过 `inject = ['agents', 'environment']` 声明依赖。

### 用户安装

正式发布后，用户只需要安装一个包并启动原有 DSH Web profile：

```bash
npx @deepseek-ai/dsh plugin --profile web add @slimzeo/hev-dsh-plugin@latest
npx @deepseek-ai/dsh web
```

安装包已经包含两个 JavaScript 入口和各支持平台的预编译 hev binary；用户不需要安装 Go、单独安装内部插件、修改 PATH 或新增启动器。

### 本地 checkout 接入

在 hev 根目录构建完整 bundle：

```bash
pnpm --dir adapter/dsh install
pnpm --dir adapter/dsh typecheck
pnpm --dir adapter/dsh test
pnpm --dir adapter/dsh build
```

本地开发也只安装这个 bundle 目录：

```bash
cd <deepseek-harness-root>
pnpm dsh plugin --profile web add /absolute/path/to/hev/adapter/dsh
```

包内 binary 是默认路径。如果需要测试单独构建的 Go binary，可以在 `$DSH_HOME/profiles/web/cordis.patch.yml` 覆盖绝对路径（`$DSH_HOME` 默认是 `~/.dsh`）：

```yaml
- id: hev-runtime
  config:
    executable: /absolute/path/to/hev/.local/bin/hev
```

检查组合结果：

```bash
pnpm dsh --profile web --dump-config
```

输出必须包含禁用的 `id: skill` / `name: '@deepseek-ai/dsh-skill'`，以及启用的 `hev-runtime` 和 `hev-skill-registry`。随后启动 Web profile：

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

- Bundle 层只负责组装；两个入口运行在 Node Host，hev Core 运行在 CLI 子进程。
- Profile patch 的 `config` 是整体替换，不是深度合并；覆盖 `hev-runtime` 时必须保留该行需要的全部配置。
- 默认只执行本包发布的 hev binary；显式 `executable` 覆盖必须指向受信任的文件。
- DSH Profile 会关闭 peer 自动安装，并从 DSH 安装目录提供统一的宿主 peer。`pnpm peers check` 可能看不到该运行时 fallback；不要为消除提示而把 Cordis 或 DSH Service Definition 改成普通依赖，应以 `--dump-config` 和真实 Profile 启动为验收。
- DSH/Cordis peer dependency 应锁定已验证版本；升级后重新运行 typecheck、package tests 和真实 profile 装配验证。

## 参考文档

- [打包与安装插件](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/docs/user/develop/basic/publish.zh.md)
- [Cordis 生命周期与 Effect](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/docs/cordis-tutorial/02-lifecycle-and-effects.zh.md)
- [服务与依赖](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/docs/user/develop/framework/service.zh.md)
- [官方 Skill Registry 契约](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/packages/skill/skill/README.zh.md)
- [命令注册表](https://github.com/deepseek-ai/deepseek-harness/blob/141eb6fef83422698aef7a981029e843e8161534/packages/interaction/commands/README.zh.md)
