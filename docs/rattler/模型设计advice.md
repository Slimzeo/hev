## 8.22 第一版本improve advice

不过当前设计里还有三个需要纠正的地方：

1. `SkillManagement` 与 `EnvironmentSkill` 职责重复，建议删除。
2. `SkillSharedCache` 更像 Repository/Infrastructure，不是核心 Domain。
3. “当前激活环境”不属于 `Environment`，而属于某个 shell/Agent 会话。

**一、先看完整的领域关系**

```
SkillIdentity
      │ 标识逻辑版本
      ▼
SkillArtifactIdentity
      │ 标识不可变内容
      │
      ▲ N
EnvironmentSkill
      │ N:1
      ▼
Environment
      │
      ▼
EnvironmentRegistry
```

```
ShellSession
      │ 0..1
      ▼
Environment
```

更直观地说：

```
Environment A
├── Skill X v1，以模式 M1 使用
├── Skill Y v2，以模式 M2 使用
└── 独立 Python prefix

Environment B
├── Skill X v1，以模式 M3 使用
└── 独立 Python prefix

共享 Skill 仓库
├── Skill X v1 的真实数据
└── Skill Y v2 的真实数据
```

同一份 Skill 数据可以被多个环境复用，但 `mode`、顺序和环境内配置属于 `EnvironmentSkill`。

**二、逐个调整你现有的模型**

**1. `SkillIdentity`：保留**

```rust
/// Skill 的逻辑版本身份。
pub struct SkillIdentity {
    pub skill_name: String,
    pub creator: String,
    pub version: String,
}
```

它回答：

> 这是哪个作者发布的哪个 Skill 的哪个版本？

但它还不能证明两份实际数据完全相同。

因为可能出现：

```
name = github
creator = alice
version = 1.0.0
```

发布者却修改了 1.0.0 的文件内容。

因此建议增加内容摘要：

```rust
/// 一份不可变 Skill 制品的精确身份。
pub struct SkillArtifactIdentity {
    /// Skill 的逻辑版本。
    pub skill: SkillIdentity,
    /// 整个 Skill 目录树的内容摘要。
    pub content_digest: String,
}
```

两者的区别：

```
SkillIdentity = 逻辑坐标 = alice/github@1.0.0
SkillArtifactIdentity = 精确数据版本 = alice/github@1.0.0 + sha256:abc...
```

`Environment` 应该锁定 `SkillArtifactIdentity`，否则同一个版本的内容被原地修改后，环境会悄悄漂移。

**2. `SkillData`：建议改成 `SkillArtifact`**

```rust
/// 一份可以被环境引用的不可变 Skill 制品。
pub struct SkillArtifact {
    /// 精确制品身份。
    pub id: SkillArtifactIdentity,
}
```

它表达“Skill 内容本身是什么”。

但真实磁盘路径属于持久化/基础设施模型：

```rust
/// Skill 制品在共享存储中的实际位置。
struct SkillArtifactRecord {
    pub artifact_id: SkillArtifactIdentity,
    pub root_path: PathBuf,
}
```

这样可以区分：

- `SkillArtifact` = Domain，表示哪份 Skill 内容
- `SkillArtifactRecord` = Repository，表示它实际存在哪里

**3. `SkillSharedCache`：先确定它到底是 Cache 还是 Store**

这个命名需要特别注意。

Cache 的语义是：

> 随时可以删除 → 可以从原始来源重新构建

Store 的语义是：

> 它是环境运行所依赖的正式数据 → 删除后环境会损坏

如果环境最终只引用这份共享数据：

```
EnvironmentSkill → shared path → SkillData
```

那么它不是 Cache，而是：

`SkillArtifactStore`

如果原有 Skill 安装目录仍是权威来源，共享目录只是可以重建的副本，才可以叫：

`SkillSharedCache`

它也不应该是 Domain Model，而应该是 Repository/Infrastructure：

```rust
pub struct SkillArtifactRepository {
    pub root: PathBuf,
}
```

如果只是想描述某一时刻里面有什么，可以定义只读模型：

```rust
pub struct SkillCacheSnapshot {
    pub entries: Vec<SkillArtifactRecord>,
}
```

你的 map 也建议拆成两层：

```
SkillIdentity → SkillArtifactIdentity → SkillArtifactRecord → 实际目录
```

而不是：

```
SkillIdentity → [Skill]
```

因为 `SkillIdentity` 已经包含 version，正常情况下一个逻辑版本应该只对应一个不可变 digest；如果发现同一身份对应两个 digest，应当报告制品冲突。

**4. `SkillManagement`：建议删除**

目前：

```rust
pub struct SkillManagement {
    pub id: SkillIdentity,
    pub env_skills: Vec<EnvironmentSkill>,
}
```

有两个问题：

第一，`Management` 更像一个 Service 名称，不像业务实体。

第二，它维护 Skill 到 Environment 的反向关系，而 Environment 又维护 Skill 列表，会产生双份事实：

```
Environment.skills
SkillManagement.env_skills
```

两边可能不一致。

正确的唯一关系模型应该是 `EnvironmentSkill`：

```rust
/// 某一环境对某份 Skill 制品的使用方式。
pub struct EnvironmentSkill {
    /// 环境锁定的精确 Skill 制品。
    pub skill: SkillArtifactIdentity,
    /// 该 Skill 在当前环境中的执行或注入方式。
    pub mode: SkillExecuteMode,
    /// 上下文注入顺序；只有顺序确实影响结果时才保留。
    pub order: u32,
}
```

如果以后需要查询“哪些环境使用了 Skill X”，从所有 Environment 的 manifest 中查询或建立派生索引即可，不要在 Skill 上维护 `belong_envs`。

**5. `Environment`：保留，但成员类型要改**

你目前写的是：

```
skill_list [SkillManagement]
```

应该改成：

```rust
/// 一个由环境管理器管理的隔离环境。
pub struct Environment {
    /// 环境的稳定身份。
    pub id: EnvironmentIdentity,
    /// Python 依赖和环境本地配置所在的根目录。
    pub prefix: PathBuf,
    /// 当前环境选择和锁定的 Skill。
    pub skills: Vec<EnvironmentSkill>,
}
```

环境里存的是“当前环境如何使用 Skill”，所以必须是 `EnvironmentSkill`，不是全局 `SkillManagement`。

**6. `EnvironmentIdentity`**

如果环境名永远不能修改，可以直接把 name 当唯一键：

```rust
pub struct EnvironmentIdentity {
    pub name: String,
}
```

如果未来支持重命名，建议分开：

```rust
pub struct EnvironmentIdentity {
    pub id: String,
}

pub struct Environment {
    pub id: EnvironmentIdentity,
    pub name: String,
    pub prefix: PathBuf,
    pub skills: Vec<EnvironmentSkill>,
}
```

这样：

- `id` 是稳定身份；
- `name` 是用户可修改的展示名称。

**7. `EnvironmentRegistry`**

可以保留：

```rust
/// 本应用管理的全部环境。
pub struct EnvironmentRegistry {
    pub environments: BTreeMap<EnvironmentIdentity, Environment>,
}
```

它承载：

- 环境唯一性；
- 环境 ID 到 Environment 的映射；
- 环境列表；
- Skill 引用关系的全量扫描入口。

但其 JSON 读写、锁文件和原子更新仍属于 Repository。

**三、Skill 隔离和 Python 依赖隔离是两件事**

你现在的 `prefix` 不能独自解释所有隔离。

**Python 依赖隔离**

```
Environment.prefix
└── bin/python
└── bin/pip
└── site-packages
```

用户激活环境后执行：

```bash
pip install xxx
```

由于 PATH 指向当前环境，包会装到该环境自己的 `site-packages`。

**Skill 内容隔离**

真实 Skill 文件可以共享，但每个环境拥有不同的 manifest：

```
Environment A
└── EnvironmentSkill(X v1, mode=A)

Environment B
└── EnvironmentSkill(X v1, mode=B)
```

运行时流程：

```
当前 EnvironmentIdentity → 读取 Environment → 得到 Vec → 根据 SkillArtifactIdentity 查询共享 Skill 数据 → 按环境自己的 mode/order 进行 context 注入
```

所以隔离来源是：

- `prefix` → 隔离 Python 依赖
- `Environment.skills` → 隔离 Skill 选择、版本、模式和上下文注入

不是简单地“不同环境复制不同 Skill 目录”。

**四、“当前激活环境”究竟是什么**

最容易理解的业务 PSM 类比是：

```
Environment ≈ Tenant 记录
ActiveEnvironmentContext ≈ 当前请求 Context 中的 tenant_id
```

一个 Tenant 不会有全局字段：

```
tenant.IsCurrent
true
```

因为：

```
请求 A 当前属于 tenant-1
请求 B 当前属于 tenant-2
```

同理，一个 Environment 不能有：

```rust
pub active: bool
```

因为：

```
终端 A 激活 env-a
终端 B 激活 env-b
Agent 进程 C 显式使用 env-c
```

“当前环境”必须属于会话上下文，而不是 Environment 自身。

**五、激活后是否一定传递 Environment 唯一标识**

概念上是的，但不同类型的调用有不同传递方式。

**子进程**

激活脚本设置：

```bash
HEV_ID=env-a
HEV_PREFIX=/.../env-a
HEV_MANIFEST=/.../env-a/manifest.json
PATH=/.../env-a/bin:$PATH
```

后续启动的子进程会自动继承这些环境变量。

用户执行：

```bash
pip install xxx
```

`pip` 不需要理解 `HEV_ID`，因为 PATH 已经让它使用目标环境里的 Python。

Skill Resolver 则可以读取：

`HEV_ID`

再找到对应 Environment 和 `EnvironmentSkill` 列表。

**RPC 或跨进程请求**

如果下游处理依赖当前环境，应该显式携带：

`environment_id`

可以放在请求字段、Header 或统一 Context 中，类似业务 PSM 的 `tenant_id`。

**进程内部**

不要让每个函数都偷偷读取全局环境变量。

应该在入口解析一次：

```
HEV_ID → ActiveEnvironmentContext
```

后面的 Environment 相关逻辑显式接收 Context。

**六、activate/deactivate 相关模型怎么设计**

这些不是核心 Environment Domain，而是 Session/Application/Runtime 模型。

**1. 当前激活环境上下文**

```rust
/// 当前会话选择的环境上下文。
pub struct ActiveEnvironmentContext {
    /// 当前环境的逻辑身份。
    pub environment_id: EnvironmentIdentity,
    /// 当前环境的物理 prefix。
    pub prefix: PathBuf,
    /// 当前环境的 Skill manifest。
    pub skill_manifest_path: PathBuf,
}
```

它回答：

> 当前这次命令、Agent 或 shell 会话正在使用哪个环境？

**2. 当前 shell 快照**

```rust
/// 执行激活或退出前读取到的 shell 状态。
pub struct ShellSessionSnapshot {
    /// 当前已激活环境；没有时为空。
    pub active_environment: Option<ActiveEnvironmentContext>,
    /// 当前激活嵌套层级。
    pub activation_level: u32,
    /// 当前 PATH。
    pub path: Vec<PathBuf>,
    /// 当前相关环境变量。
    pub variables: BTreeMap<String, String>,
}
```

这与 Rattler 当前的 `ActivationVariables` 接近。

**3. 环境变化模型**

```rust
/// 一次 shell 环境切换。
pub struct EnvironmentTransition {
    /// 切换前的环境。
    pub from: Option<ActiveEnvironmentContext>,
    /// 切换后的环境。
    pub to: Option<ActiveEnvironmentContext>,
}
```

一个模型覆盖三种操作：

- activate： None → Some(env-a)
- switch： Some(env-a) → Some(env-b)
- deactivate： Some(env-a) → None

因此不需要分别定义：

```
ActivateModel
DeactivateModel
SwitchModel
```

**4. Shell 变化结果**

```rust
/// 需要应用到当前 shell 的变化集合。
pub struct ShellChangeSet {
    /// 应设置或覆盖的环境变量。
    pub set_variables: BTreeMap<String, String>,
    /// 应删除的环境变量。
    pub unset_variables: Vec<String>,
    /// 应用变化后的 PATH。
    pub path: Vec<PathBuf>,
    /// 本次环境切换关系。
    pub transition: EnvironmentTransition,
}
```

最后再由不同 Shell renderer 转成：

```
Bash/Zsh: export / unset
Fish: set -gx / set -e
PowerShell: $env:...
```

**5. 退出环境需要恢复什么**

仅保存当前 Environment ID 不够。

`deactivate` 还必须知道：

- 激活前的 PATH；
- 被覆盖变量的旧值；
- 激活前是否还有上一层环境；
- 当前嵌套层级。

Conda/Rattler 使用类似：

```
CONDA_SHLVL
CONDA_ENV_SHLVL__
```

在 shell 环境变量中保存备份。

我们的概念模型可以叫：

```rust
/// 一层激活需要保存的恢复信息。
pub struct ActivationFrame {
    /// 这一层激活的环境。
    pub environment: ActiveEnvironmentContext,
    /// 激活前的 PATH。
    pub previous_path: Vec<PathBuf>,
    /// 被修改变量在激活前的值；None 表示此前不存在。
    pub previous_variables: BTreeMap<String, Option<String>>,
    /// 激活前的上一环境。
    pub previous_environment: Option<ActiveEnvironmentContext>,
}
```

但注意：CLI 每次调用后进程就退出，不能只把它存在 Rust 内存里。

它最终必须编码到：

- 当前 shell 的环境变量；
- 或者由 shell function 持有；
- 不建议写成一个全局“当前环境文件”，否则多个终端会互相覆盖。

**七、推荐的最终模型关系**

**Domain**

```
SkillIdentity
SkillArtifactIdentity
EnvironmentIdentity
EnvironmentSkill
Environment
```

**可选 Domain 聚合**

```
EnvironmentRegistry
```

**Repository 模型**

```
SkillArtifactRecord
EnvironmentRecord
RegistryFile
EnvironmentManifestRecord
```

**Session/Application 模型**

```
ActiveEnvironmentContext
EnvironmentTransition
```

**Runtime 模型**

```
ShellSessionSnapshot
ActivationFrame
ShellChangeSet
```

**八、你当前方案最应该做的修改**

```
SkillSharedCache
→ 如果是权威数据，改名 SkillArtifactStore
→ 如果可删除重建，保留 Cache，但放 Repository/Infrastructure

SkillManagement
→ 删除

EnvironmentSkill
→ 保留，作为 Environment 和 SkillArtifact 的关联实体

SkillData
→ 改成 SkillArtifact / SkillArtifactRecord

SkillIdentity
→ 保留，并补 content_digest 形成精确制品身份

Environment.skill_list
→ 改为 Vec

Environment.active
→ 不要增加

当前环境
→ 放 ActiveEnvironmentContext，由每个 shell/Agent 会话独立持有
```

还有三个关键不变量建议写进设计：

1. 同一个 `SkillIdentity` 只能对应一个 `content_digest`；否则说明版本被污染。
2. Environment 只保存正向 `EnvironmentSkill` 关系；不在 Skill 上维护 `belong_envs`，避免双写。
3. 删除共享 Skill 制品前，通过所有 Environment manifest 做 mark-and-sweep；不要维护容易漂移的手工引用计数。
