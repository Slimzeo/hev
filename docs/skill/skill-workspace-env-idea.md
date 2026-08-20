# Skill Workspace / Environment Idea Backup

> 记录日期：2026-08-20（Asia/Shanghai）  
> 状态：原始构想备份，用于防止后续调研与实现讨论覆盖最初问题意识。  
> 来源：围绕 SkillLens 论文、`paper-explain` 创建与持续进化的讨论。

## 一句话 Idea

**Skill 不应只是一份 `SKILL.md` 文件夹，而应成为 Skill Workspace 控制平面中的可发布 artifact；同类 Skill 共享经验池、环境、Evals 与 family-level Evolver，全局 Supervisor 只负责任务调度、隔离、预算、晋升和回滚。**

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

## 两个不同概念

### Skill Workspace

持久的治理、数据和版本边界。它管理同一类 Skill 的完整生命周期。

```text
paper-skill-workspace/
├── workspace.yaml
├── skills/
│   ├── paper-explain/
│   ├── paper-compare/
│   └── paper-article/
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

同一个 Workspace 可以包含多个 Env，例如 TraeX、Codex、Claude Code，以及在线/离线 PDF 环境。评测主键至少应包含：

```text
skill_version × target_model × harness × tools × permission × domain
```

## Evolver 分层

不应该让一个万能 Evolver 直接修改所有 Skill。建议分两层：

### Global Skill Supervisor

只管理控制面：

- 发现哪个 Skill family 出现退化；
- 判断是否达到 evolution trigger；
- 分配模型、并发、token 与评测预算；
- 保证 discovery 与 held-out 隔离；
- 决定 freeze、canary、promote、rollback；
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
  └── pass → PROMOTE
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

## Manifest 草案

```yaml
kind: SkillWorkspace
version: 1

metadata:
  name: paper-understanding
  family: paper

skills:
  incumbent:
    paper-explain: 1.4.2
  members:
    - paper-explain
    - paper-compare
    - paper-article

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
  canary:
    target_scoped: true
  rollback:
    retain_versions: 5
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

## 当前最小 MVP

先只做一个 `paper-understanding` Workspace：

- incumbent：现有 `paper-explain`；
- family evolver：`paper-evolver`；
- evaluator：hard gates + 结构化 LLM judge + 人工反馈；
- store：Git 管 Skill 版本，SQLite 管 case/eval/run metadata，本地目录管大 artifact；
- 环境：TraeX + 当前主模型；
- promotion：手工批准，不自动发布；
- UI：先用 CLI/静态报告，验证顺手后再产品化。

这与“先个人本地使用顺手，再考虑平台化”的方向一致。
