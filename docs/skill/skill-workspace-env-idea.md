# Skill Workspace / Environment Idea Backup

> 记录日期：2026-08-20（Asia/Shanghai）  
> 状态：原始构想备份，用于防止后续调研与实现讨论覆盖最初问题意识。  
> 来源：围绕 SkillLens 论文、`paper-explain` 创建与持续进化的讨论。
> 2026-08-21 增量：加入 **user-owned Skill Pack + community registry**。用户不是被动的 Skill consumer，而是组合、策展、发布和最终批准的主体；平台提供协议、证据与安全基础设施。

## 一句话 Idea

**Skill 不应只是一份 `SKILL.md` 文件夹，而应成为 Skill Workspace 控制平面中的可发布 artifact；用户拥有 Skill Pack 的组合与最终批准权，同类 Skill 共享经验池、环境、Evals 与 family-level Evolver，全局 Supervisor 负责调度、隔离、预算、晋升 Gate 和回滚执行。**

新增原则：**多 Skill 管理不应全部收归一个算法 selector。用户天然会按自己的工作流制作整合包；平台的核心职责是让这些整合包可组合、可验证、可分享、可追溯、可回滚，而不是替用户决定唯一正确的 Skill 集合。**

## 为什么需要 Workspace / Env

当前 Skill 更像静态能力包：

```text
skill/
├── SKILL.md
├── agents/openai.yaml
├── references/
├── scripts/
└── assets/
```

这能解决“如何被发现、加载和执行”，却没有原生表达：

- Skill family 与成员关系；
- target model / harness / tool / permission 环境；
- good case、bad case、用户纠错和原始 trace；
- development / discovery / held-out eval 分离；
- incumbent / challenger / lineage；
- Evolver 的触发、预算与作用域；
- canary、promotion、freeze、rollback；
- 谁可以修改 Skill，谁只能评估。

SkillLens 表明同一 Skill 在不同 target/harness/domain 中可能产生相反结果，因此 Skill 版本不能脱离运行环境单独谈“好坏”。

## 三个核心概念

### Skill Workspace

持久的治理、数据和版本边界。它管理同一类 Skill 的完整生命周期。

```text
paper-skill-workspace/
├── workspace.yaml
├── skills/
│   ├── paper-explain/
│   ├── paper-compare/
│   └── paper-article/
├── packs/
│   └── evidence-first-paper-studio/
├── agents/
│   ├── supervisor.md
│   ├── collector.md
│   ├── curator.md
│   ├── evaluator.md
│   └── evolver.md
├── experience/
│   ├── raw/
│   ├── curated/
│   └── quarantine/
├── evals/
│   ├── development/
│   ├── discovery/
│   └── heldout/
├── candidates/
├── registry/
│   ├── subscriptions.json
│   └── advisories/
├── reports/
└── state/
    ├── incumbent.json
    ├── lineage.json
    └── promotions.jsonl
```

### Execution Environment

一次结果成立的具体运行条件。

```yaml
env_id: trae-gpt54-paper-v1
target:
  model: gpt-5.4
  harness: traex
  reasoning_effort: medium
tools:
  pdf_parser: pymupdf
  browser: playwright
permissions:
  network: true
skill_set:
  - paper-explain@1.4.2
```

同一个 Workspace 可以包含多个 Env，例如 TraeX、Codex、Claude Code，以及在线/离线 PDF 环境。原子 Skill 的评测主键至少应包含：

```text
skill_version × target_model × harness × tools × permission × domain
```

Skill Pack 还必须把实际解析出的成员和组合策略纳入主键：

```text
pack_version × resolved_skill_lock × composition_policy
× target_model × harness × tools × permission × task_slice
```

社区报告只有匹配这些条件时才能作为较强证据；否则只是帮助用户决定是否值得在本地复测的先验。

### User-owned Skill Pack

大型 Skill Library 不一定首先是一个“让模型在几千个 Skill 中自动检索”的问题。用户通常已经知道自己要完成哪类工作，也会自然形成稳定组合，例如：

```text
论文研究包
  = paper-explain
  + citation-check
  + figure-extract
  + huashu-design
  + publish-offline-html
```

因此需要一个独立于原子 Skill 的一等 artifact：`SkillPack`。它是用户或社区作者维护的、有明确用途和环境边界的组合配方，不是把多个 Skill 文本粗暴拼接进 prompt。

```yaml
kind: SkillPack
version: 1

metadata:
  name: evidence-first-paper-studio
  owner: local-user
  purpose: 论文取证、解读与离线图文发布

members:
  - skill: paper-explain
    version: 1.4.2
    role: research
  - skill: citation-check
    version: 0.3.0
    role: verify
  - skill: huashu-design
    version: 2.1.0
    role: presentation

composition:
  default_order: [research, verify, presentation]
  activation: user_or_supervisor
  conflicts: []

environment:
  harness: traex
  tools: [browser, pdf_parser]
  permissions:
    network: true
    filesystem: workspace-write

evidence:
  tested_envs: [trae-gpt54-paper-v1]
  eval_report: reports/evidence-first-paper-studio.json
  known_failures: [scanned_pdf_without_ocr]

updates:
  policy: manual-approval
  lockfile: skill-pack.lock
```

### Skill Pack 的权属原则

- 用户决定装什么、怎么组合、何时启用和是否升级；
- 用户可以 fork 社区 Pack，并保留本地 patch 与私有 Skill；
- Supervisor 可以建议增删、发现冲突、预估权限和展示证据，但不能静默改包；
- Evolver 生成 challenger，不直接覆盖用户当前组合；
- Pack 发布者提供用途、顺序、参数、环境和已知失败，不宣称跨环境普适；
- 社区热度是发现信号，不是 utility 证明；最终采用仍要经过本地环境评测。

## 四层生态，而不是一个万能 Selector

```text
Atomic Skill
  可独立版本化的最小能力 artifact
        ↓ 由用户/作者组合
Skill Pack
  面向一个工作流的配方、版本锁和组合规则
        ↓ 安装到个人边界
Personal Workspace
  本地偏好、私有数据、cases、环境与 incumbent
        ↕ 选择性发布/订阅
Community Registry
  Skill/Pack 分发、provenance、兼容性证据、评测与 case 交换
```

这里的社区不是一个中央模型替所有人做决定，而是一套可 fork 的知识与证据网络：

- 作者发布原子 Skill；
- 领域用户发布整合包和使用配方；
- 评测者贡献公开 benchmark、失败报告与兼容性矩阵；
- 平台维护 schema、签名、权限、版本锁、sandbox、eval runner 和 rollback；
- 每个用户保留本地选择权，默认不上传原始会话、私有文件和未脱敏 cases。

## 用户、社区、平台与 Supervisor 的职责边界

| 角色 | 应负责 | 不应默认负责 |
| --- | --- | --- |
| 用户 / Pack 作者 | 定义真实意图、组合工作流、选择版本、审批权限、接受或拒绝升级 | 证明每个组合在所有模型和环境中都安全 |
| Skill 作者 | 维护原子能力、版本、依赖、触发条件和已知边界 | 决定所有用户应该搭配哪些其他 Skill |
| 社区 | 分享 Pack、case、评测结果、兼容性经验与 fork lineage | 用 star、下载量或口碑替代真实 utility |
| 平台 / 协议 | manifest、lockfile、provenance、签名、sandbox、eval、diff、rollback | 垄断策展或生成唯一官方组合 |
| Supervisor | 推荐组合、做冲突/权限检查、调度 eval、解释证据 | 静默安装、静默改写 Pack、越过用户意图自动晋升 |
| Evolver | 在限定 family/env 内生成候选并解释变更 | 同时改 Skill、考卷和生产 incumbent |

“交给用户”并不等于平台什么都不做。没有版本锁、环境声明、provenance、权限审计和本地 held-out eval，用户整合包仍可能把多个单独有效的 Skill 组合成负迁移或供应链风险。正确分工是：**组合权归用户，可信计算与治理原语归平台。**

## Evolver 分层

不应该让一个万能 Evolver 直接修改所有 Skill。建议分两层：

### Global Skill Supervisor

只管理控制面：

- 发现哪个 Skill family 出现退化；
- 判断是否达到 evolution trigger；
- 分配模型、并发、token 与评测预算；
- 保证 discovery 与 held-out 隔离；
- 计算 freeze、canary、promote、rollback gate，向 owner 给出带证据的建议；
- 只执行用户预先授权的自动策略；默认 promotion 和新增权限需要 owner 批准；
- 防止 Evolver 同时修改 Skill 和考卷；
- 管理跨 family 的安全、隐私与发布规则。

### Family / Domain Evolver

只理解一类任务，例如 paper、coding、resume：

- 从 curated cases 找重复 failure mechanism；
- 对比 incumbent/challenger 的高低 utility 输出；
- 提炼 failure signature、executable remedy、blacklist 与 boundary；
- 判断修改位置：`SKILL.md`、reference、script、asset 或 trigger description；
- 生成 immutable candidate；
- 不直接覆盖生产 incumbent。

## Case 与 Evals 分工

“用户反馈”不能直接等于 ground truth。建议拆成：

```text
Collector → Case Curator → Evaluators → Evolver
```

### Collector

保存原始证据：

- 用户点赞、点踩、纠错、要求重做；
- 任务输入、工具调用、读取覆盖、输出 artifact；
- 测试、构建、文件 diff、引用与公式核验；
- model、harness、Skill version、tool policy、cost；
- 用户是否手工大改或继续追问。

### Case Curator

- 处理隐私、权限与数据保留；
- 去重；
- 区分 Skill 缺陷、模型能力、工具故障和用户风格偏好；
- 按 domain / input route / output mode / failure cause 分桶；
- 将恶意、可疑、低置信 feedback 放入 quarantine；
- 只把可解释 cases 提交给 eval/evolver。

### Evaluators

只输出裁决，不修改 Skill：

```yaml
outcome: fail
reason_code: missing_appendix_evidence
evidence:
  - expected: Appendix F alternative harness
  - actual: article omitted it
suggested_action:
  - add appendix coverage gate
confidence: high
```

评测至少分为：

- `development`：快速调试；
- `discovery`：寻找 failure cluster、生成 rubric；
- `heldout`：冻结的晋升考卷，Evolver 不可见。

### Evolver

只消费 curated development/discovery cases，不能访问 held-out 答案。

## Supervisor 状态机

```text
OBSERVE
  ↓ 足够新证据 / 检测到退化
CURATE
  ↓ 数据与权限合格
EVALUATE INCUMBENT
  ↓ 存在稳定失败簇
EVOLVE CANDIDATE
  ↓
OFFLINE HELD-OUT
  ├── fail → REJECT / FREEZE
  └── pass
        ↓
TARGET-SCOPED CANARY
  ├── regress → ROLLBACK
  └── pass → OWNER REVIEW
                    ├── reject → KEEP INCUMBENT
                    └── approve → PROMOTE
```

不要因单个 bad case 立即演化。触发器可以是：

```yaml
evolution_trigger:
  min_new_cases: 20
  min_repeated_failure_cluster: 5
  or:
    - critical_user_correction: true
    - target_model_changed: true
    - harness_changed: true
    - heldout_regression_gt: 2.0pp
```

## Promotion Gate

同时比较：

```text
Δ_inc  = Utility(challenger) - Utility(incumbent)
Δ_none = Utility(challenger) - Utility(no-skill)
```

候选相对旧版提升，不代表已经优于 no-skill。Promotion 至少要求：

- hard gates 通过；
- `Δ_inc > 0`；
- `Δ_none >= 0`；
- 关键 target/domain slice 无严重回归；
- 预算、延迟和人工成本在预注册范围；
- canary 与 rollback 已演练；
- candidate、evaluator、eval set 和环境均可追溯。
- Pack owner 已审阅成员、权限与版本 diff；除非明确配置自动晋升，否则需要人工批准。

## Manifest 草案

```yaml
kind: SkillWorkspace
version: 1

metadata:
  name: paper-understanding
  family: paper
  owner: local-user

skills:
  incumbent:
    paper-explain: 1.4.2
  members:
    - paper-explain
    - paper-compare
    - paper-article

packs:
  incumbent:
    evidence-first-paper-studio: 0.1.0
  lockfile: skill-pack.lock
  ownership:
    final_composition_authority: user
    allow_silent_mutation: false

agents:
  supervisor: paper-skill-supervisor
  collector: paper-case-collector
  curator: paper-case-curator
  evaluator: paper-evaluator
  evolver: paper-evolver

environments:
  - trae-gpt54
  - codex-gpt54
  - claude-opus

experience:
  raw_store: experience/raw
  curated_store: experience/curated
  quarantine_store: experience/quarantine

evals:
  development: evals/development
  discovery: evals/discovery
  heldout: evals/heldout
  freeze_heldout: true

promotion:
  require:
    - hard_gates_pass
    - delta_vs_incumbent_positive
    - delta_vs_no_skill_non_negative
    - no_critical_slice_regression
    - owner_approval
  canary:
    target_scoped: true
  rollback:
    retain_versions: 5

community:
  registry: optional
  publish_default: private
  accept_popularity_as_utility: false
  require_provenance: true
```

## 当前 TraeX 的边界

TraeX 当前已有项目级资源作用域：

```text
<workspace>/.trae/skills/
<workspace>/.trae/agents/
<workspace>/.trae/hooks.json
```

它可以拼出 MVP：

- 项目 Skill 提供被测 artifacts；
- 自定义 agents 承担 supervisor/evaluator/evolver role；
- hooks 从 `UserPromptSubmit`、`PostToolUse`、`Stop`、`SessionEnd` 收集事件；
- plugin 可打包 skills、hooks、MCP 与 app。

但当前原生标准没有：

- Skill family / lifecycle workspace；
- user-owned Skill Pack manifest、lockfile 与本地 override；
- 社区 Registry 的 provenance、compatibility evidence 与 advisory 协议；
- persistent experience pool；
- frozen held-out set；
- challenger/incumbent registry；
- promotion/rollback state machine；
- hook 直接启动 agent 的能力（当前 command hook 可执行，prompt/agent hook 会被跳过）。

因此本 Idea 是在现有“资源 workspace”之上新增一个“Skill 生命周期控制平面”。

## 待验证问题

1. Workspace 是 TraeX 原生能力、插件，还是独立 local-first service？
2. Experience store 使用 JSONL/SQLite/Git 还是事件日志 + artifact store？
3. 用户反馈如何绑定具体 turn、Skill version 和 output artifact？
4. Evals 如何既复用真实 case 又避免 train/test leakage？
5. 全局 Supervisor 是否只做调度，还是还负责跨 family rubric？
6. 多个 Skill 同时加载时，如何做 credit assignment？
7. Skill family 的最小粒度是什么，如何避免 Evolver 过度共享规则？
8. 如何对恶意反馈、poisoned traces 与危险 Skill 做安全隔离？
9. 是否需要 UI 展示 lineage、cases、eval matrix、candidate diff 和 rollout？
10. 与现有 local-first AI workspace 产品的边界是什么？
11. Pack 内 request-level routing 应由用户固定、Supervisor 建议，还是允许按策略自动调整？
12. 社区 case/eval 怎样在保护隐私的同时形成可复用证据？

## 当前最小 MVP

先只做一个 `paper-understanding` Workspace：

- incumbent：现有 `paper-explain`；
- user-owned Pack：`evidence-first-paper-studio`，先用本地 `pack.yaml + lockfile` 固定组合；
- ownership：用户手工选择成员、审批权限与升级；Supervisor 只生成建议和 diff；
- family evolver：`paper-evolver`；
- evaluator：hard gates + 结构化 LLM judge + 人工反馈；
- store：Git 管 Skill 版本，SQLite 管 case/eval/run metadata，本地目录管大 artifact；
- 环境：TraeX + 当前主模型；
- promotion：手工批准，不自动发布；
- community：第一阶段只支持导出/导入 Pack 和 eval report，不先建设中心化推荐算法；
- UI：先用 CLI/静态报告，验证顺手后再产品化。

这与“先个人本地使用顺手，再考虑平台化”的方向一致。
