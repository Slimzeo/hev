# 当前多 Skill 视角的不足：从算法选择转向用户策展与社区生态

> 讨论对象：SkillLens，*From Raw Experience to Skill Consumption: A Systematic Study of Model-Generated Agent Skills*，arXiv:2605.23899v1。  
> 论文读取覆盖：本地 [`paper.pdf`](paper.pdf) 物理 p.1–24，包含正文与附录。  
> 文档性质：论文证据边界 + 本解读的批评 + Skill Env 产品主张。三者不混写。

## 一句话判断

SkillLens 很好地回答了一个算法与评测问题：**一份模型生成的 Skill 对某个 target 是否真的产生下游 utility？**

但它没有进入另一个同样关键的产品与生态问题：

> 用户本来就会围绕自己的工作流组合、修改和传播 Skill Pack；平台是否应该少做一个“替所有人决定最佳组合”的中央算法，多做一套让用户组合可声明、可验证、可分享、可追溯和可撤销的基础设施？

我们的答案是：**是。组合权应默认归用户，社区沉淀可复用整合包，平台承担协议、兼容性、安全与证据底座，Supervisor 只在用户授权的 Pack 和权限范围内做运行时建议与调度。**

这不是 SkillLens 已证明的结论，而是从其单 Skill 证据边界继续向产品生态推演出的主张。

## 1. 论文对多 Skill 到底做了什么

| 层次 | 论文实际做法 | 证据状态 |
| --- | --- | --- |
| 单 Skill | 每个 target/domain 最多提炼一个 domain-level Skill，直接注入 system prompt | 主实验已验证 |
| 多 Skill 暴露 | `list_skills -> view_skill -> read_skill_file` 渐进披露 | 附录协议模板 |
| 大型 Skill Library | 将 selection、composition、interference、safety 列为未来问题 | 未做实验 |
| 用户整合包 | 未定义 Pack、作者、fork、override、lockfile 或社区治理 | 未研究 |

论文在 Appendix A 明确说，它选择了 `interpretability over coverage`：每个 target 的经验被合并为一个 Skill，再直接放进 system prompt，以便把 performance delta 尽可能归因于 Skill，而不是 retrieval policy、agent scaffold 或其他混杂变量。

同一页把以下方向列为未来工作：

- richer agent harnesses；
- retrieval、planning、tool-use scaffolds；
- substantially larger skill libraries；
- selection、composition、interference；
- 大型 library scale 下的 safety。

**Locator：**[`paper.pdf`](paper.pdf) p.15，Appendix A；[`evidence-ledger.md`](evidence-ledger.md) 的“单 skill 直接注入”条目。

Appendix B.4 / Table 6 确实提供了多 Skill 的渐进披露接口，但它只回答“Skill 怎样按需展示给模型”，没有回答：

- 模型是否选对；
- 多个 Skill 是否应该同时启用；
- 顺序是否影响结果；
- 两个 Skill 是否发生指令或工具冲突；
- 用户是否接受这个组合；
- 组合在版本更新后是否仍然成立。

**Locator：**[`paper.pdf`](paper.pdf) p.17、p.20，Appendix B.4、Table 6。

## 2. 为什么作者没有把管理交给用户

我们无法从论文中知道作者的主观动机，因此不能断言“作者不相信用户”或“作者想让算法接管”。从研究设计可以确认的原因有三个。

### 2.1 用户策展会破坏当前实验的变量隔离

如果不同用户自行选择、修改和排序 Skill，那么测到的结果同时包含：

```text
Skill 内容效应
+ 用户选择效应
+ Pack 组合效应
+ 顺序与参数效应
+ 用户经验与熟练度效应
```

SkillLens 当前想先隔离第一项，所以刻意移除了用户策展和复杂 Harness。这是合理的实验收缩。

### 2.2 它把研究单位定义成 extractor、target、domain

论文的主要变量是：

```text
experience pool × extractor × target × domain
```

这里的 `target` 是消费 Skill 的模型，不是有目标、偏好、私有上下文和维护习惯的最终用户。论文没有用户研究、Pack 作者行为或社区数据，因此也没有证据回答用户如何策展。

### 2.3 Benchmark 更容易测模型 utility，难以测社区价值

任务分数、no-skill baseline 和 `delta` 容易形成受控实验；社区复用、信任、维护成本、fork 质量和用户满意度需要长期系统与用户研究。论文选择了前者。

所以，说它具有明显的“算法研究视角”是成立的：它主要把世界表示为可控制的模型、轨迹、Skill 文本和 benchmark delta。但这应被理解为**观察单位和证据类型的局限**，不是对作者个人的动机判断。

## 3. 论文视角的八个不足

这些不足有两类：有些是作者主动声明的 scope boundary；有些是把论文迁移到真实产品时才暴露的缺口。

### 3.1 用户被压缩成被动 Consumer

在论文框架里，target 接收一个 Skill，执行任务，产出分数。真实用户却会：

- 按工作流挑选 Skill；
- 改写说明、参数和顺序；
- 把多个能力封装成自己的整合包；
- 对模型建议说“不”；
- 保留私有 patch；
- 分享或 fork 别人的组合。

这些不是噪声，而是 Skill 生态中的重要知识来源。

### 3.2 研究单位停留在原子 Skill

真实复用的对象经常不是一个 Skill，而是一个带版本锁、参数、顺序和权限的工作流 Pack。

```text
Atomic Skill：某项可复用能力
Skill Pack：用户为了完成真实工作而组织出的能力配方
```

只优化原子 Skill，不能保证组合后的 Pack 有效。

### 3.3 把多 Skill 首先表述成算法 Selection 问题

论文把 selection、composition、interference 列为大型 library 的一等问题，这是正确的；但从产品角度，它们不必全部由中央 selector 从零求解。

用户制作的 Pack 已经提供一个强先验：

```text
全局几千个 Skill
  -> 用户先策展成一个小型、目的明确的 Pack
  -> Supervisor 只在 Pack 内做请求级选择和调度
```

用户策展不是 selection 的竞争方案，而是缩小选择空间、注入真实意图和领域 tacit knowledge 的上层机制。

### 3.4 Utility 主要等同于 Benchmark Delta

论文的 `delta` 很有价值，但真实用户效用还包括：

- 是否减少返工；
- 是否遵守个人风格和组织约束；
- 是否便于理解与修改；
- 是否降低 token、延迟和权限成本；
- 是否能稳定复现；
- 用户是否愿意长期保留。

这些不能被一次任务分数完全代表。

### 3.5 缺少社区 Artifact 与治理对象

论文没有定义：

- Pack manifest 和 lockfile；
- 作者、维护者和 fork lineage；
- 兼容性报告；
- 环境限定的 eval badge；
- security advisory；
- deprecation 与 migration；
- 用户 override 与本地 patch；
- 社区 case 的隐私、许可和 provenance。

因此它能评价 Skill 文本，却还不能支撑 Skill 生态。

### 3.6 缺少长期版本与更新语义

论文测的是固定版本、固定 split 和固定运行设置。真实系统需要回答：

- Skill 更新后 Pack 是否自动升级；
- target model 或 Harness 更新后旧证据是否过期；
- 社区作者删库或改权限时如何处理；
- 本地 patch 怎样 rebase；
- 哪个版本可以 rollback。

### 3.7 Progressive Disclosure 只是信息协议

`list/view/read` 有助于控制上下文，但不自动解决：

- relevance ranking；
- 组合冲突；
- 权限叠加；
- credit assignment；
- 版本兼容；
- 用户信任。

不能把 Table 6 写成经过实验证明的多 Skill 编排架构。

### 3.8 Safety 被放在未来工作，没有形成社区责任模型

论文已经提醒：坏经验会携带 bias 或 unsafe shortcut，更强 Skill 也可能被滥用。但它没有回答 Skill/Pack 作者、Registry、平台、Supervisor 和用户分别承担什么责任。

## 4. 我们的核心修正：组合权归用户，可信底座归平台

“把 Skill 管理交给用户”不能理解为平台退出。更准确的分层是：

```text
用户定义目的、信任范围和组合
        ↓
社区沉淀可 fork 的 Skill Pack 与真实经验
        ↓
平台提供协议、版本、权限、证据与回滚
        ↓
Supervisor 在授权边界内做请求级选择和调度
```

### 四层 Artifact

```text
Atomic Skill
  最小能力、独立版本、明确输入输出和边界
        ↓ 用户/作者组合
Skill Pack
  面向工作流的成员、参数、顺序、冲突和版本锁
        ↓ 安装到个人边界
Personal Workspace
  私有偏好、cases、环境、评测结果和本地 patch
        ↕ 选择性发布/订阅
Community Registry
  分发、fork、provenance、兼容性证据和安全公告
```

完整架构已同步到 [`skill-workspace-env-idea.md`](skill-workspace-env-idea.md)。

### 职责边界

| 角色 | 应负责 | 不应默认负责 |
| --- | --- | --- |
| 用户 / Pack 作者 | 定义意图、组合、启停、参数、版本选择、高风险授权和最终升级批准 | 手工证明所有组合在所有模型上安全 |
| Skill 作者 | 维护原子能力、依赖、触发条件、版本和已知边界 | 决定所有用户的最佳工作流 |
| 社区 | 分享 Pack、fork、公开 case、兼容性经验和 benchmark | 用热度替代 utility，或承担最终安全保证 |
| 平台 | schema、Registry、签名、lockfile、sandbox、权限 diff、eval runner、rollback | 垄断策展或静默替用户改包 |
| Supervisor | 在已授权 Pack 内推荐、路由、排冲突、控预算、降级并解释关键决定 | 静默安装、扩权或越过用户明确选择 |
| Evolver | 生成有证据的 challenger，说明变更与适用环境 | 同时修改 Skill、held-out 考卷与生产 incumbent |

## 5. 为什么不能把一切都甩给用户

用户适合表达意图和策展，但不应独自承担系统性风险：

1. **隐性干扰：**两个单独有效的 Skill 组合后可能争夺指令优先级、上下文和工具。
2. **版本漂移：**模型、Harness、工具协议和依赖更新会让旧 Pack 证据失效。
3. **权限叠加：**多个低风险 Skill/Plugin 组合后可能形成高风险操作链。
4. **成本竞争：**全量加载会增加 token、延迟和注意力干扰。
5. **普通用户缺少评测能力：**用户知道自己想要什么，不等于能发现统计回归和供应链问题。
6. **生态碎片化：**没有 manifest、lockfile 和 lineage，私人整合包无法可靠复现。

所以正确命题不是“用户管理 vs 平台管理”，而是：

> 用户拥有组合与最终决定权；平台负责让决定具有足够证据，并在运行时守住权限、兼容性和恢复边界。

## 6. 对 SkillLens 的未来研究拓展建议

### RQ-U1：用户策展能否优于全局自动检索

比较五种条件：

```text
No Skill
Best single Skill
User-curated Pack
Global auto-selector
User-curated Pack + constrained supervisor
```

不仅看任务分数，还要看：

- selection error；
- 用户 override rate；
- time-to-success 与返工；
- token / latency / tool cost；
- permission incidents；
- 跨版本稳定性。

### RQ-U2：Pack 是否是比原子 Skill 更真实的复用单位

对同一工作流比较：

- 各原子 Skill 单独启用；
- 全量平铺注入；
- Pack 声明顺序与角色；
- Supervisor 动态路由；
- 用户固定配方 + Supervisor 局部调整。

这能区分 Skill 内容收益与组合机制收益。

### RQ-U3：社区证据能否跨用户迁移

社区不应只存 star、下载量和文字评价，而应存环境化证据：

```text
pack_version
× resolved_skill_lock
× composition_policy
× target_model
× harness
× tools
× permission
× domain/task slice
× evaluator_version
```

研究同一 Pack 在作者本地、相似用户和陌生用户环境中的 utility 衰减。

### RQ-U4：怎样进行多 Skill Credit Assignment

建议至少加入：

- leave-one-skill-out ablation；
- 顺序交换；
- 参数敏感性；
- 冲突注入；
- 版本扰动；
- 用户 override 后的 counterfactual replay。

目标不是给每个 Skill 一个虚假的全局分数，而是定位哪种组合在什么环境下产生增益或退化。

### RQ-U5：用户反馈怎样进入 Evolver 而不污染系统

```text
Raw feedback
  -> privacy/provenance check
  -> dedup + failure attribution
  -> curated case
  -> development/discovery eval
  -> challenger
  -> frozen held-out
  -> user-approved promotion
```

点赞、点踩、安装量不能直接成为 ground truth；用户改写、返工、撤销和明确纠错才是更强但仍需审查的信号。

### RQ-U6：社区与供应链安全

未来 benchmark 应覆盖：

- 恶意或被污染的 Skill/Pack；
- 权限升级和跨 Skill capability composition；
- 作者账号/签名变化；
- 依赖撤回与 typosquatting；
- unsafe shortcut 从经验池进入 Skill；
- Registry 下架后本地可恢复性。

## 7. 建议建立的社区数据源

### 可作为强证据

- 可复现的 Pack-level eval run；
- 明确环境和版本的成功/失败 case；
- 用户纠错前后的 artifact diff；
- leave-one-out 与顺序实验；
- 权限拒绝、rollback 与 incompatibility report；
- 维护者发布的 migration / deprecation record。

### 只能作为弱发现信号

- star；
- 下载与安装量；
- 点赞、点踩；
- “好用”评论；
- LLM 对 Skill 文本的静态评分。

SkillLens 已经说明，文本 plausibility 甚至可能与真实 utility 反向，因此社区热度不能代替环境化下游评测。

## 8. 平台最小能力，而不是中央智能大脑

平台第一阶段不需要解决“从一万个 Skill 中自动找出最优组合”。更值得先建立：

1. `skill.yaml`：原子 Skill 身份、版本、依赖、权限、环境和边界；
2. `pack.yaml`：用户组合、角色、顺序、参数与冲突声明；
3. `skill-pack.lock`：精确版本与内容 hash；
4. provenance/signature：作者、来源、fork lineage 和完整性；
5. permission diff：安装或升级前显示新增能力；
6. compatibility matrix：证据按 target/harness/env 分片；
7. local eval runner：用户可在私有 case 上验证；
8. candidate diff + rollback：更新可审查、可拒绝、可撤销；
9. Registry：发布、fork、订阅、security advisory 和弃用通知；
10. constrained Supervisor：只在已授权 Pack 内建议与调度。

## 9. 推荐的双层评测

```text
第一层：Atomic Skill Causal Eval
固定 target/harness，Skill vs No Skill
回答：内容本身有没有 utility？

第二层：Pack Integration Eval
Pack vs incumbent Pack，并做成员消融、顺序与权限测试
回答：放进真实用户工作流后是否仍有效？
```

第一层沿用 SkillLens 的强项；第二层补上用户策展、社区复用和多 Skill 集成。二者不能互相替代。

## 10. 对 SkillLens 的最终评价

公平的评价不是“作者为什么不直接做社区”，而是：

- 它有意收缩问题，成功建立了单 Skill utility 的受控证据；
- 它证明“看起来像好 Skill”远远不够，并暴露 target/domain/harness compatibility；
- 但其研究对象仍以模型、Extractor、文本和 benchmark 为中心；
- 用户的策展知识、Pack ownership、社区分发和长期治理尚未进入模型；
- 因而不能从该论文直接推出由平台算法全面接管大型 Skill Library。

我们继承它的 **utility-grounded** 原则，但把研究和产品单位向上扩展：

```text
SkillLens：这个 Skill 对这个 target 有用吗？

Skill Env：
这个用户为什么选择这组 Skill？
这个 Pack 在哪些环境中有证据？
社区如何复用又不抹掉个体差异？
平台怎样帮助而不夺走用户控制权？
更新失败时怎样回到可工作的版本？
```

这才是从“算法生成 Skill”走向“用户拥有的 Skill 生态”的下一步。

## 11. 归属边界，避免以后误引

### 论文实证

- 单 Skill 的 downstream utility 依赖 extractor、target 和 domain；
- 存在显著负迁移；
- 文本 plausibility 不是可靠 utility proxy；
- Appendix A 将大型 library 的 selection、composition、interference 与 safety 列为未来工作。

### 论文启发但未验证

- 用 progressive disclosure 控制 Skill 上下文；
- 将 utility-grounded eval 延伸到大型 Skill Library；
- 在 richer harness 中继续测 compatibility。

### 本文的产品与研究主张

- 用户拥有 Skill Pack 的组合与最终批准权；
- 社区 Registry 沉淀 Pack、fork、case 和环境化证据；
- 平台提供可信底座，而不是唯一组合答案；
- Supervisor 在授权范围内调度，不静默安装、扩权或晋升；
- 原子 Skill 与 Pack 分两层评测。

以上第三组不能写成“SkillLens 提出”或“论文已经证明”。
