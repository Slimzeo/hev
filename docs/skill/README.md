# SkillLens / Agent Skill 研究资料包

本目录归档了论文 **From Raw Experience to Skill Consumption: A Systematic Study of Model-Generated Agent Skills** 的原文、逐页取证、证据账本、三种受众版本的深度解读、交互研究 Demo，以及由论文结论进一步推演出的 Skill Workspace / Environment 构想。

> 命名说明：旧工作目录名为 `skillOpt`，但这里研究的论文真实身份是 **SkillLens**，不是 Microsoft 的另一个 SkillOpt 项目。

## 论文身份

- 标题：*From Raw Experience to Skill Consumption: A Systematic Study of Model-Generated Agent Skills*
- 规范版本：arXiv:2605.23899v1，2026-05-22
- 作者：Zisu Huang、Jingwen Xu、Yifan Yang、Ziyang Gong 等
- 单位：Fudan University、Microsoft Research、Shanghai Jiao Tong University
- 本地原文：[`paper.pdf`](paper.pdf)，24 个物理页
- PDF SHA-256：`a5d88b47f668f7b24f091ff817bc7fade594d09fea6c0f57e2457be37bc51ade`
- 归档日期：2026-08-20

详细版本与事实边界见 [`product-facts.md`](product-facts.md)。

## 论文到底研究什么

这篇论文没有把 Skill 当作“写得清楚、结构完整的一份提示词”，而是把它视为一次需要在目标模型上实测的**行为干预**。作者研究一条完整生命周期：

```text
目标模型产生原始成功/失败轨迹
  -> Extractor 从经验中提炼 domain-level Skill
  -> 同一目标模型在 held-out tasks 中消费 Skill
  -> 对比 no-skill baseline，得到真实 utility delta
```

论文集中回答三个问题：

1. 模型生成的 Skill 是否稳定有效，还是也会产生负迁移？
2. Skill 的效用来自经验池、Extractor、Skill 文本，还是目标模型的消费能力？
3. 能否从高低效 Skill 的差异中发现规则，再反过来改进 Skill 生成？

## 核心结论

### 1. Skill 平均有用，但绝不默认安全

主实验覆盖 `5 domains x 6 targets x 5 extractors = 150` 个单元。Table 1 中有 112 个正迁移、37 个负迁移、1 个零变化。平均收益会掩盖明显的 target、extractor 与 domain 交互，因此不能因为“同一份 Skill 在强模型上有效”就直接推广给其他模型或 harness。

### 2. Skill 的 utility 属于运行环境，不只属于文本

同一 Skill 换一个 target model，收益可能从正变负；Appendix F 换成 Claude Code / Codex harness 后也仍然存在明显分化。工程上更准确的评测主键应接近：

```text
skill_version x target_model x harness x tools x permission x domain
```

### 3. “看起来合理”不是可靠的质量代理

GPT-5.4 对 151 个 high-gap Skill pairs 做无指导文本判断时，整体准确率只有 46.4%；差距达到 5 个百分点时只有 15.8%。清晰、完整、格式漂亮，可能只是文本审美，并不能证明 Skill 会让 Agent 做得更好。

### 4. 真正有价值的是可执行的失败知识

论文从真实 utility 对照中筛出三项更有效的内容特征：

- Failure Mechanism Encoding：说清楚失败为什么发生；
- Actionable Specificity：给出可执行、可落地的 remedy；
- High-Risk Action Blacklist：明确哪些高风险动作不要做。

这三项把 pairwise judge accuracy 从 46.4% 提升到 73.8%；作为 generation-time meta-skill，在 3 domains x 3 targets 的 9 个验证单元中全部优于原始生成方式，平均提升 1.55 个百分点。

### 5. 论文没有解决完整的 Skill 生命周期治理

主实验是“单个 domain Skill 直接注入 system prompt”的受控设置。它没有解决大型 Skill library 的检索、组合、冲突、长期漂移、用户反馈治理、candidate promotion、canary 和 rollback。目录中的 [`skill-workspace-env-idea.md`](skill-workspace-env-idea.md) 是我们沿着这些缺口做的架构推演，**不是论文作者已经实现或验证的系统**。

## 资料目录

```text
skill/
├── README.md
├── paper.pdf
├── index.html
├── product-facts.md
├── research-spine.md
├── evidence-ledger.md
├── skill-workspace-env-idea.md
├── cur_multi_skill_shortage.md
├── verification-report.json
├── notes/
│   └── paper-detailed-notes.html
├── articles/
│   ├── 01-engineering-walkthrough.html
│   ├── 02-formal-research-reading.html
│   └── 03-architecture-decision-guide.html
├── design-demos/
│   ├── 01-lifecycle-trace.html
│   ├── 02-explorable-skill-lab.html
│   └── 03-evidence-lens-lab.html
├── inspire/
│   └── 01-learning-the-user-paper-reading-paradigm.md
└── screenshots/
    ├── index.png
    ├── paper-detailed-notes.png
    ├── 01-engineering-walkthrough.png
    ├── 02-formal-research-reading.png
    ├── 03-architecture-decision-guide.png
    ├── 01-lifecycle-trace.png
    ├── 02-explorable-skill-lab.png
    └── 03-evidence-lens-lab.png
```

## 文档说明

### 研究入口与审计底稿

- [`index.html`](index.html)：整个研究包的离线导航入口。
- [`research-spine.md`](research-spine.md)：最集中地回答 why、problem、method、evidence、mechanism 与 boundary。
- [`evidence-ledger.md`](evidence-ledger.md)：逐条记录核心 claim、PDF locator、证据能支持什么、不能支持什么。
- [`product-facts.md`](product-facts.md)：固定论文身份、版本、实验口径、官方实现与禁止外推项。
- [`verification-report.json`](verification-report.json)：8 个 HTML、4 个 viewport 和 6 个核心交互的无头浏览器验证结果。

### 全文顺序取证

- [`notes/paper-detailed-notes.html`](notes/paper-detailed-notes.html)：按物理 PDF p.1-24 顺序覆盖正文、参考文献与附录。它承担完整性和 locator 追溯，不要求正式文章机械复述所有内容。

### 三版深度解读

- [`articles/01-engineering-walkthrough.html`](articles/01-engineering-walkthrough.html)：面向 Agent / Skill 工程师，按 Experience -> Extract -> Consume -> Delta 的运行因果解释 Skill 为什么改变行为。
- [`articles/02-formal-research-reading.html`](articles/02-formal-research-reading.html)：面向研究读者，重点审查变量隔离、统计口径、judge 可信度、因果强度与外部有效性。
- [`articles/03-architecture-decision-guide.html`](articles/03-architecture-decision-guide.html)：面向架构评审，给出 Adoption Gate、benchmark、rollout、stop condition、ADR 与 Definition of Done。

### 三份交互研究 Demo

- [`design-demos/01-lifecycle-trace.html`](design-demos/01-lifecycle-trace.html)：沿完整生命周期观察 Skill utility 到 `delta` 才显现。
- [`design-demos/02-explorable-skill-lab.html`](design-demos/02-explorable-skill-lab.html)：交互改变经验成功率，观察不同 domain 为什么没有统一最优经验配比。
- [`design-demos/03-evidence-lens-lab.html`](design-demos/03-evidence-lens-lab.html)：在 Problem、Framework、Evidence、Boundary 四个视角间切换。

### 后续设计与方法复盘

- [`skill-workspace-env-idea.md`](skill-workspace-env-idea.md)：Skill Workspace、Execution Env、全局 Supervisor、family-level Evolver、case curator、held-out eval、promotion 与 rollback 的原始构想备份。
- [`cur_multi_skill_shortage.md`](cur_multi_skill_shortage.md)：区分论文实证与产品推演，分析其多 Skill / 用户视角不足，并提出 user-owned Skill Pack、社区 Registry、职责边界与未来研究方案。
- [`inspire/01-learning-the-user-paper-reading-paradigm.md`](inspire/01-learning-the-user-paper-reading-paradigm.md)：对 DeepSeek Harness 论文解读范式的学习总结，以及旧版解读失败原因。

## 推荐阅读路径

### 15 分钟快速理解

1. [`research-spine.md`](research-spine.md)
2. [`articles/01-engineering-walkthrough.html`](articles/01-engineering-walkthrough.html)
3. [`design-demos/03-evidence-lens-lab.html`](design-demos/03-evidence-lens-lab.html)

### 设计 Skill Env

1. [`articles/03-architecture-decision-guide.html`](articles/03-architecture-decision-guide.html)
2. [`evidence-ledger.md`](evidence-ledger.md)
3. [`cur_multi_skill_shortage.md`](cur_multi_skill_shortage.md)
4. [`skill-workspace-env-idea.md`](skill-workspace-env-idea.md)
5. 原论文 p.7-11、p.15、p.17、p.20-24

### 研究复核与引用

1. [`product-facts.md`](product-facts.md)
2. [`notes/paper-detailed-notes.html`](notes/paper-detailed-notes.html)
3. [`evidence-ledger.md`](evidence-ledger.md)
4. [`paper.pdf`](paper.pdf)

## 文件可移植性

所有正式 HTML 中的论文图片都已转换为 `data:image/...;base64` 内嵌资源，不依赖 CDN、远程字体、远程脚本或外部图片。单独发送 HTML 时，正文和图片仍可正常显示。

需要注意：

- 两篇文章中的“打开原始 PDF”链接仍指向相对路径 `../paper.pdf`，单发 HTML 时需同时保留 PDF 才能使用该链接。
- `index.html` 是跨文档导航页，分享整个资料包时应保留当前目录层级。
- 三份 Demo 使用内联 JavaScript，建议使用现代 Chrome、Edge、Safari 或 Firefox。
- 本次归档没有复制旧版浅层稿、原始裁图、构建脚本、逐页中间文本和重复 PDF；这些过程材料仍保留在原研究目录 `/Users/bytedance/workspace/paper-article/skillOpt`。

## 最重要的工程结论

不要问“这个 Skill 好不好”，而应问：

> 这个不可变 Skill 版本，在这个 target model、harness、工具与权限组合下，相对 incumbent 和 no-skill baseline，是否在冻结的 held-out cases 上产生可重复的正 utility，并且具备可追溯、可灰度、可回滚的证据？

这也是本资料包与 `skill-env` 项目的直接连接点。
