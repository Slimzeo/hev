# Claim–Evidence Ledger

> 论文版本：本地 `paper.pdf`，arXiv:2605.23899v1，24 个物理页。页码均指该 PDF。

| Claim | 归属 | Locator | 证据能支持什么 | 不能支持什么 |
| --- | --- | --- | --- | --- |
| SkillLens 覆盖 experience generation、skill extraction、skill consumption 三段 | 研究设计 | p.2–4, Figure 1, §3.1 | 给出 target 自生成轨迹、extractor 抽取、同 target held-out consumption 的闭环 | 不是长期在线自演化或大规模 library orchestration |
| Extraction framework 刻意保持最小 | 实验控制 | p.4, §3.2; p.16–18, Table 2–4 | 差异尽量归因 extractor，而非 domain heuristic / sub-agent scaffold | 不能代表更复杂生产 extraction pipeline 的绝对效果 |
| `Δ` 用 held-out no-skill baseline 衡量 utility | 指标定义 | p.5, Eq.3 | 同 target/domain/test split 下隔离 skill intervention 的百分点变化 | 不同 benchmark 的分数不可横向相加；仍可能受 sampling noise 影响 |
| EE 与 TE 分别观察 extractor 与 target | 指标定义 | p.5, Eq.4–5 | 在固定候选集合中分开平均两种角色 | 不是模型内在、跨版本稳定属性；平均会隐藏 interaction |
| 主实验 150 cells 中约 75% positive、25% negative | 主结果 | p.6–7, Table 1 | 精确计数 112 positive / 37 negative / 1 zero；skill 平均有益但会负迁移 | 不能推成每个 domain/target 都有 25% 风险，或某次上线风险恰为 25% |
| 负迁移随 domain 变化 | 主结果 | p.6–7, Table 1 | ALFWorld 14/30 negative；SpreadsheetBench、SWE-bench 各 4/30 | 不证明 domain 名称本身导致风险，模型/harness/任务结构均可能参与 |
| Better executor 不一定是 better extractor | 交互结果 | p.6–7, Table 1, §4.2 | SpreadsheetBench 中 Gemini-3.1-FL EE +5.86，GPT-5.4 EE +1.67 | 单一 domain 排名不能推广到所有 domain |
| Skill utility target-dependent | 交互结果 | p.6–7, p.9–10, Table 1, Figure 4 | 相同 extractor/skill 对不同 target delta 差异大；strong-pool +1.8 到 +9.5 | 不证明 target scale 或 baseline performance 是唯一解释 |
| Experience pool composition 影响 utility | 受控干预 | p.8, Figure 2, §5.1 | 固定 extractor 操纵 success ratio；all-failure 一致最差，optimal mix domain-specific | 仅 3 domains × 3 targets；没有估计全量最优采样策略 |
| Skill format 不解释主要差异 | 统计实验 | p.8–9; p.18, p.21, Table 8 | SpreadsheetBench 四种语义保持格式对所有 targets `p>0.34`、σ-ratio<1；extractor 对 5/6 显著 | 不表示格式对检索、可读性、长上下文或所有任务永远无效 |
| Unguided textual plausibility 不预测 utility | Pairwise 实验 | p.9, Figure 3; p.20, Appendix E | 151 pairs、9 votes、order randomized、gap>0.5pp；overall 46.4%，δ≥5pp 为 15.8% | GPT-5.4 judge 不是人类评审；pair subset 不是全部技能空间 |
| Concrete remedies 比 generic advice 更接近 utility | 对照案例 | p.9; p.23–24, Table 14–15 | 两个 high-gap pair 显示环境 failure mechanism 与 executable remedy 的差异 | 两个案例不能单独建立普适因果；后续 rubric screening 才提供规模化信号 |
| Validated rubric 三维与 higher-Δ 对齐 | 自动发现 + 筛选 | p.10–11; p.23, Table 13 | 17 high-gap pair 归纳 7 维；三维 better-rate 64–66% | 64–66% 不是完美预测；Environment/Tool Semantics 等未入最终三维不代表无价值 |
| Rubric-guided judge 从 46.4% 提到 73.8% | 评估干预 | p.9–11, Figure 3, §6 | 同 151 high-gap pairs 上，明确 rubric 能改善同一 judge 的 utility selection | 不能替代 downstream execution；未报告独立 rubric-discovery/test pair split |
| Validated meta-skill 改善 9/9 extraction cells | 生成干预 | p.10–11, Figure 5; p.22, Table 11 | 3 domains × 3 targets，平均 +1.55 pp；plausibility 平均 −0.59 pp | 仅有限矩阵；不证明跨 extractor/domain/模型版本普适，也未给置信区间 |
| Skill consumption reshapes default policy | 定性机制 | p.10; p.18–19, Appendix D | SpreadsheetBench 代表 target 的 decision/exploration/tool-use 变化支持 strategy correction 解释 | 定性分析、两个 target；不是 mediation analysis 或全模型因果证明 |
| Alternative harness 仍呈现正均值与强 variance | 稳健性补强 | p.20–21, Table 10 | Claude Code / Codex 子集平均 +0.4 pp，仍有 positive/negative target interaction | 只覆盖 SpreadsheetBench 子集；不证明跨 harness 全面稳健 |
| 单 skill 直接注入是为可解释控制 | 设计边界 | p.15, Appendix A | 减少 retrieval/scaffold confound，支持 cross-extractor/target 比较 | 不能回答 selection、composition、interference、大规模 library 问题 |
| 官方 SkillLens 仓库可执行四阶段 pipeline | 官方实现事实 | 论文 p.1 code link；`microsoft/SkillLens` README | 框架、CLI、五 benchmark integration、sequential/parallel extraction 公开可用 | 代码公开不等于论文全部数值已在本机复现 |

## 关键实验条件

1. 每个 domain 使用固定 1:1 experience-generation / held-out test split；同 domain 各组合共用 split。
2. Table 1 Base 与 with-skill 均为 3 次独立运行平均。
3. 主实验每个 target experience pool 分别由该 target 自己生成；extractor 只负责从该 pool 抽取。
4. 主实验每次最多合成 1 个 domain-level skill，最长 3000 chars；逐轨迹最多 3 modes；merge group size 10；temperature 1.0。
5. 单 skill 直接 inline system prompt；多 skill protocol 只在附录模板讨论，不是主实验的大型 library 设置。
6. Closed-source reasoning/thinking level 设 medium；Qwen3.5-9B 不作为 extractor。
7. Pairwise judge：GPT-5.4、每 pair 9 votes、多数投票、顺序随机、within `(target,domain)`、`|Δ| gap > 0.5pp`。
8. Meta-skill extraction 只验证 ALFWorld / SpreadsheetBench / SWE-bench × GPT-5.4 / Gemini-3.1-Pro / Qwen3.5-35B。

## 未确认或论文未覆盖

- Table 1 三轮均值的方差、置信区间和每 task 原始结果。
- Experience composition 实验的每个 target/domain 明细、采样 pool size 与随机敏感性。
- Rubric discovery 的 17 pairs 与 151-pair judge evaluation 是否有信息重叠，以及维度筛选的多重比较控制。
- Meta-skill 9 cells 的独立随机重复、置信区间和 extractor 覆盖范围。
- 长期 skill drift、online update、skill selection、retrieval、composition、conflict、interference。
- 生产 P50/P99、token/call cost、人工维护成本、安全审核与 rollback SLA。
- 不同模型版本、reasoning effort、tool harness 与 prompt policy 下的复现稳定性。
- 论文作者没有提供一个可跳过 downstream execution 的“静态 skill 质量分”。
