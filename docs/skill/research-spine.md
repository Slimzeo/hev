# 论文研究主线：为什么“会写 Skill”不等于“Skill 有用”

> 论文：*From Raw Experience to Skill Consumption: A Systematic Study of Model-Generated Agent Skills*  
> 版本：用户提供的 arXiv:2605.23899v1 本地 PDF，24 个物理页。页码均指该 PDF。

## 一句话 thesis

论文真正改变的不是又一种 skill 生成 prompt，而是**把 skill 从一份看起来合理的文本，改写成一个必须经过完整生命周期和目标模型实测的干预**：经验池决定可提炼的信息，extractor 决定能否把轨迹压成具体程序性知识，target 决定能否把同一份 skill 转化为稳定行为。作者用 held-out downstream `Δ` 把三者拆开测量，证明平均收益掩盖了显著负迁移；再从真实高低效 skill 对照中发现“失败机制、可执行具体性、高风险黑名单”比清晰、完整、格式漂亮更接近真实 utility，并把该规律反馈为 generation-time meta-skill。

## 1. 为什么写这篇论文？

自动生成 skill 的方法快速增加，但评测大多只覆盖生命周期的一截：有的拿人工或公开 skill 测 consumption，有的测检索与编排，有的把 skill 限制成可执行函数组合。缺少的不是又一个成功案例，而是一个受控问题：**同一目标模型自己的原始轨迹，经不同 extractor 压缩后，再喂回该模型，究竟何时涨、何时跌、为什么？**

论文因此拒绝用“文本读起来不错”当质量代理，把 held-out task performance delta 作为 utility proxy。

**Locator：**PDF p.1–3，§1–2；Figure 1 p.3。

## 2. 精确研究问题是什么？

1. **RQ1 / 是否可靠：**model-generated domain-level skill 在不同 domain、extractor、target 上是否稳定带来收益？
2. **RQ2 / utility 从哪里来：**经验池的成功/失败构成、skill 文本内容与 target consumption ability 分别怎样影响下游结果？
3. **RQ3 / 能否闭环改进：**能否把诊断出来的 utility 特征做成一个 drop-in meta-skill，反过来指导 extraction？

这三个问题必须连起来。只回答“平均涨分”无法解释负迁移；只做文本质量 judge 无法知道 target 是否真会执行；只提出 rubric 而不重新跑 downstream task，也不能证明 rubric 有用。

**Locator：**PDF p.2，§1；p.7–11，§4.2–6。

## 3. 既有评测为什么不够？

- **只测 consumption：**人工、公开仓库或 task-seeded skill 绕过 extraction，不能回答 raw experience 怎样变成 utility。
- **只测 extractor 文本：**清晰、完整、通顺、格式整齐可能只是 LLM 的文本偏好；论文的 unguided judge 实验显示它甚至会系统性偏向低 utility skill。
- **只看平均分：**同一 domain 的 target / extractor interaction 很强，平均数会隐藏局部负迁移。
- **只用一个 harness：**skill 可能与执行 scaffold 耦合；Appendix F 因此用 Claude Code / Codex 对 SpreadsheetBench 子集做替代 harness 检查。
- **只做大而复杂的 extraction pipeline：**很难区分收益来自 extractor 还是 scaffold。论文故意剥掉 domain heuristics、sub-agent fleet、conflict resolution 与 skill deepening。

**Locator：**PDF p.2–4，§1–3.2；p.9、p.18、p.20–21。

## 4. 方法是什么？

方法不是一个模型，而是一套五层研究协议。

1. **Target-specific experience。**目标模型 `M` 在 experience split 上产生 `(task, trajectory, outcome)`，成功与失败都进入 pool。
2. **Minimal extraction。**Extractor `E` 逐轨迹提取最多 3 个 success/failure pattern，再以 group size 10 分层合并，最后生成最多 1 个、最长 3000 chars 的 domain skill。
3. **Held-out consumption。**同一个 target `M` 在未见 test split 上分别跑 no-skill 与 with-skill 条件。
4. **角色分离指标。**`Δ(E,M,D)` 测单元效应；`EE(E,D)` 跨 target 平均 extractor；`TE(M,D)` 跨 extractor 平均 target evolvability。
5. **诊断闭环。**对 experience composition、format、pairwise plausibility、cross-model transfer 与行为变化分别干预/分析；再从 high-gap pair 发现 rubric，筛选维度，并把 validated rubric 注入 extractor。

**Locator：**PDF p.3–5，Figure 1、Eq.1–5；p.15–20，Appendix B/E。

## 5. 方法凭什么说“有效”？证据阶梯是什么？

### 第一级：全矩阵结果——是否有用，也是否会伤害

5 domains × 6 targets × 5 extractors 形成 150 个单元。Table 1 精确计数为 112 正、37 负、1 零；论文概括为 75% positive / 25% negative。ALFWorld 14/30 为负，SpreadsheetBench 与 SWE-bench 各 4/30 为负。

**支持：**model-generated skill 平均有益，但不能假定安全；extractor、target、domain 之间存在显著 interaction。  
**不支持：**某个 extractor 或 skill 对所有模型普遍最好；跨 benchmark 的 `Δ` 可直接相加。

**Locator：**PDF p.6–7，Table 1、§4.2。

### 第二级：阶段干预——utility 为什么变化

- Experience：固定 extractor，操纵 success ratio；all-failure 一致最差，但最佳配比依 domain 改变。
- Extraction：语义保持的四种格式没有可检出 effect；换 extractor 对 5/6 targets 有显著 effect。
- Text judge：151 个 high-gap pairs 上，unguided accuracy 46.4%；`δ ≥ 5pp` 时 15.8%，说明“看起来好”与 utility 脱钩。
- Consumption：固定 skill 文本跨 target 注入，strong-pool skill 的 gain 从 +1.8 到 +9.5；weak-pool skill 对 GPT-5.4 为 −2.0。

**支持：**三段生命周期各有独立 variation source；target consumption ability 是效用上限的一部分。  
**不支持：**Figure 2 给出全 domain 通用最佳 success ratio；定性 behavior analysis 已证明完整因果机制。

**Locator：**PDF p.8–10，Figure 2–4；p.18–21，Table 8–10。

### 第三级：从诊断到干预——rubric 是否真的能改 extraction

作者从 17 个 high-gap pairs 归纳 7 个 raw dimensions，再用 better-rate 做 utility screening。最终三项为 Failure Mechanism Encoding、Actionable Specificity、High-Risk Action Blacklist。它们把同一 judge 的 pairwise accuracy 从 46.4% 提至 73.8%；作为 extractor meta-skill 时，在 9 个验证单元全部优于 original，平均 +1.55 pp，而 naive plausibility rubric 平均 −0.59 pp。

**支持：**utility-grounded criteria 不仅可改 judge，还能在当前 3×3 设置中改生成结果；“文本审美”可能反向伤害。  
**不支持：**这三维是普适、充分、永不过时的 skill quality definition；9 cells 足以覆盖所有模型与 domain。

**Locator：**PDF p.9–11，Figure 3/5、§6；p.22–24，Table 11–15。

### 第四级：外部有效性补强——换 harness 后是否仍有信号

SpreadsheetBench 子集在 Claude Code / Codex 上平均 `Δ = +0.4 pp`，但各 target 仍强烈分化，甚至出现 −5.5 pp。

**支持：**主结论不完全由固定 Python-script harness 伪造。  
**不支持：**跨 harness 稳健、收益很大或负迁移已解决。

**Locator：**PDF p.20–21，Appendix F、Table 10。

## 6. 为什么有效？

论文能直接支持的机制是 **policy reshaping / strategy correction**，不是“skill 给模型植入了新能力”。在 SpreadsheetBench 的代表性分析中：

- GPT-5.4 被引导从写公式字符串转向 Python 预计算静态值并写回，增加结构检查、anchor-based addressing 与 post-write validation；这些动作更对齐 evaluator。
- Qwen3.5-9B 也更早探索、更倾向 openpyxl 原地编辑，但复杂 workflow 超出其稳定执行能力时，结构忠实度换来更多执行错误。
- 高 utility skill 把环境真实语义与失败机制压成具体 remedy；低 utility skill 常只有“resolve the contract”“edit minimally”之类无法阻止主导错误的过程口号。

所以有效链条是：**真实轨迹暴露错误 → extractor 编码具体机制 → skill 改变 target 的初始策略与工具用法 → evaluator-aligned 行为概率上升。**链条中任何一段不匹配，都可能负迁移。

**Locator：**PDF p.9–11、p.18–19、p.23–24。

## 7. 最终可以相信什么？

可以相信的精确版本是：**在论文的单 domain skill、直接 system-prompt 注入、固定 split、当前模型与 harness 设置下，skill 的真实 utility 是 extractor × target × domain 的交互结果；文本表面质量不足以做可靠代理。用真实 downstream delta 发现并筛选的三项内容维度，在有限验证矩阵中同时改善了 pairwise 选择与 skill extraction。**

不能从论文推出：

- 自动生成的 skill 默认应该上线；
- 一个“最强 extractor”可以服务所有 target；
- 只收失败轨迹或只收成功轨迹是普遍最优；
- 格式、检索、渐进披露在大型 skill library 中不重要；
- 73.8% judge accuracy 足以替代真实 task evaluation；
- validated meta-skill 已经解决 safety、长期漂移、selection、composition、interference 与生产 rollout；
- 本地目录名 `skillOpt` 对应论文 SkillOpt。

**Locator：**PDF p.7–11、p.15、p.20–24。
