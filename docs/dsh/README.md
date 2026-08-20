# DeepSeek Harness / DSH 论文研究资料包

本目录整理了论文 **A Programming Paradigm for Spatiotemporal Composability** 的原文、逐页笔记、三种受众版本的深度解读，以及三份可交互研究 Demo。

论文作者：Yifan Shi、Wei Zhang、Tianyi Cui；署名单位为 Peking University 与 DeepSeek-AI。当前收录的是用户提供的 **2026-08-13 本地技术手稿**，共 88 个物理 PDF 页。正文未给出 venue、DOI 或 arXiv 编号，因此这里不把它描述成已正式发表版本。

## 论文到底解决什么问题

论文研究的不是“怎样把插件动态加载进来”这一项功能，而是：

> 在同一长寿命进程中，组件反复加载、卸载、替换和重配时，系统怎样既只撤销离场组件自己的副作用，又在 provider 出现、消失或换身份时，让依赖它的组件按正确顺序重新解析和迁移生命周期？

作者把问题拆成两个正交维度：

- **时间可组合性（temporal composability）**：组件离开后，能够撤销自身对共享环境的贡献，同时保留其他独立组件的变化。
- **空间可组合性（spatial composability）**：组件能够声明依赖，并在 provider 动态变化时重新解析依赖、协调生命周期。

既有的进程/容器重启粒度过粗；手写 `activate/deactivate` 或 `dispose` 难以从安装动作结构化导出完整恢复；初始化期 DI 也没有定义运行中 provider 消失、换身份与异步 teardown 的顺序。

## 论文提出的方法

这不是单一算法，而是一套由局部机制逐步提升为系统保证的运行时范式：

1. **Revertible effects**：每次 context 变更同时返回针对本次状态的 inverse，runtime 按 LIFO 组合 inverse。
2. **Reactive coeffects**：组件声明 dependency specification；context 变化被分类为 activating、deactivating 或 neutral。
3. **Unified context**：将 effect 与 coeffect 放进同一种递归 context，以观测等价和 key-level mediation 连接恢复与依赖。
4. **Component / fiber calculus**：用 Inactive、Reloading、Active、Unloading 四态和十条规则处理撤出、迭代、异步、失败与 dependent drain。
5. **Cordis implementation**：用 `ctx.effect/set/get/use`、fiber 字段、声明式 loader、configuration reconciliation 与 transactional HMR 落地。

## 证据为什么可信，以及可信到哪里

论文的证据分成三层，三者不能相互替代：

1. **形式证据**：Table 1 穷举 lifecycle rules；Theorem 59/61/63/64/66/73 在显式前提下分别证明保持性、选择性恢复、依赖顺序、解析一致性、进展与合流，并覆盖任意合法 interleaving，而不是某一条示例执行。
2. **构造证据**：Table 2 与 Algorithm 1–10 把理论对象映射到 Cordis API、fiber lifecycle、loader 与 HMR，说明抽象存在可执行实现路径。
3. **案例证据**：Koishi 的服务端、Web Console 与 4000+ community plugins 支持存在性、表达力与采用度。

需要同时保留以下边界：

- Cordis runtime 不会自动证明组件提供的 inverse 语义正确。
- 选择性恢复依赖 independence、confinement 等纪律；隐藏共享状态或绕过 context 的操作会破坏保证。
- Progress 和 Confluence 还依赖无环、有限、有界、provision total、无 failure 等不同条件。
- 已经越过 system boundary 的网络发送、支付、邮件等 emission 不能靠 inverse 自动倒带。
- 论文没有提供 Cordis v4 的延迟、吞吐、CPU、内存或开发者生产率 benchmark。
- Koishi 案例使用 Cordis v3；不能据此声称 v4 全部语义已经同规模生产验证。
- Self-evolving Agent Harness 是论文结尾提出的未来验证方向，不是已经完成的实验。

## 目录与文档说明

```text
dsh/
├── README.md
├── paper.pdf
├── index.html
├── research-spine.md
├── evidence-ledger.md
├── product-facts.md
├── notes/
│   └── paper-detailed-notes.html
├── articles/
│   ├── 01-engineering-walkthrough.html
│   ├── 02-formal-research-reading.html
│   └── 03-architecture-decision-guide.html
├── design-demos/
│   ├── 01-cinematic-trace.html
│   ├── 02-distill-explorable.html
│   └── 03-takram-research-lab.html
└── screenshots/
    ├── 01-cinematic-trace.png
    ├── 02-distill-explorable.png
    └── 03-takram-research-lab.png
```

### 核心入口

- [`paper.pdf`](paper.pdf)：论文原文，88 个物理页面。
- [`index.html`](index.html)：资料包导航入口，可跳转到笔记、文章和 Demo。
- [`research-spine.md`](research-spine.md)：最集中地回答“为什么写、问题是什么、方法是什么、依据是什么、为什么可信”。
- [`evidence-ledger.md`](evidence-ledger.md)：Claim–Evidence 账本，记录核心主张、PDF locator、支持范围和不可外推项。
- [`product-facts.md`](product-facts.md)：论文版本、公开实现身份及事实边界。

### 逐页笔记

- [`notes/paper-detailed-notes.html`](notes/paper-detailed-notes.html)：按 PDF p.1–88 顺序整理的 30 个详细内容块。每块包含核心思想、推理链、定义/公式直觉、工程映射、证据边界与英文难词解释。

### 三版深度文章

- [`articles/01-engineering-walkthrough.html`](articles/01-engineering-walkthrough.html)：面向后端与 Agent Runtime 工程师，强调 `ctx.effect`、依赖通知、生命周期、loader 与 HMR。
- [`articles/02-formal-research-reading.html`](articles/02-formal-research-reading.html)：面向 PL / Systems 读者，重点解释定义链、证明依赖与 Theorem 61/63/66/73。
- [`articles/03-architecture-decision-guide.html`](articles/03-architecture-decision-guide.html)：面向技术负责人和架构评审，包含适用条件、替代方案比较、迁移 Gate、风险与 ADR 模板。

### 三份交互 Demo

- [`design-demos/01-cinematic-trace.html`](design-demos/01-cinematic-trace.html)：暗场运行轨迹，突出 effect → inverse 与 dependency pulse。
- [`design-demos/02-distill-explorable.html`](design-demos/02-distill-explorable.html)：正文与实时实验同屏，可加载/卸载组件并观察 registry、inverse stack 和依赖状态。
- [`design-demos/03-takram-research-lab.html`](design-demos/03-takram-research-lab.html)：研究工作台，可在问题、形式机制、元理论和 Cordis 映射之间切换。

## 推荐阅读顺序

### 15 分钟快速了解

1. `research-spine.md`
2. `articles/01-engineering-walkthrough.html` 的“研究问题与依据”及“边界与局限”
3. `design-demos/02-distill-explorable.html`

### 工程与架构评估

1. `articles/01-engineering-walkthrough.html`
2. `articles/03-architecture-decision-guide.html`
3. `evidence-ledger.md`
4. 原论文 p.39–68

### 形式化深读

1. `articles/02-formal-research-reading.html`
2. `notes/paper-detailed-notes.html`
3. 原论文 p.9–27、p.28–53

## 文件可移植性

所有 HTML 中的论文图片均已转换为 `data:image/...;base64` 内嵌资源，因此单独发送某个 HTML 时不会丢图，也不依赖 CDN、远程字体或远程脚本。

需要注意：

- HTML 中的“打开原始 PDF”链接仍指向本目录的 `paper.pdf`。若只单发一个 HTML，页面内容和图片正常，但 PDF locator 链接需要对方同时收到 `paper.pdf` 才能打开。
- `index.html` 是导航页，跨文档链接依赖本目录结构；分享整个资料包时请保留目录层级。
- 三份 Demo 使用内联 JavaScript，建议用现代 Chrome、Edge、Safari 或 Firefox 打开。
