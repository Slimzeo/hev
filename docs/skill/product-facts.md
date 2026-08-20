# Product and paper facts

> 核验日期：2026-08-20（Asia/Shanghai）
> 用途：给 SkillLens 论文研究包固化版本、身份、读取覆盖与不可外推项。论文事实以用户提供的本地 `paper.pdf` 为最高优先级。

## 论文身份

- 标题：*From Raw Experience to Skill Consumption: A Systematic Study of Model-Generated Agent Skills*。
- 作者：Zisu Huang、Jingwen Xu、Yifan Yang、Ziyang Gong、Qihao Yang、Muzhao Tian、Xiaohua Wang、Changze Lv、Xuemei Gao、Qi Dai、Bei Liu、Kai Qiu、Xue Yang、Dongdong Chen、Xiaoqing Zheng、Chong Luo。
- 单位：Fudan University、Microsoft Research、Shanghai Jiao Tong University。
- 规范身份：arXiv:2605.23899v1，cs.AI，提交于 2026-05-22。
- 本次读取版本：用户提供的 `paper.pdf`，SHA-256 `a5d88b47f668f7b24f091ff817bc7fade594d09fea6c0f57e2457be37bc51ade`。
- 物理页数：24 页。正文 p.1–11；参考文献 p.12–14；附录 A–H 为 p.15–24。macOS `file` 曾粗略报告 15 页，但 PyMuPDF 可枚举并渲染 24 个物理页；本研究包以逐页解析结果为准。
- 读取覆盖：p.1–24 全部提取原生文本层与整页 PNG；正文、公式、Figure 1–5、Table 1–15、附录提示词、统计检验、behavioral analysis、alternative harness 与 contrastive cases 均已读取。

## 命名边界：目录叫 skillOpt，论文不是 SkillOpt

- 当前工作目录名为 `skillOpt/`，这是用户工作区命名，不是论文标题。
- 本地 PDF 与 arXiv 内容明确对应 **SkillLens**。首页代码链接为 `https://aka.ms/SkillLens`。
- Microsoft 另有独立项目 **SkillOpt**；不能把其方法、指标或仓库事实写进本论文。研究包正文统一使用“SkillLens 论文”或论文全名。

## 官方实现身份

- 官方仓库：`microsoft/SkillLens`，论文首页短链与仓库 README 相互指向。
- 项目页：`https://microsoft.github.io/SkillLens/`。
- 许可证：MIT（以仓库 README / LICENSE 为准）。
- README 将可执行流水线写成四段：raw experience generation → schema normalization → skill extraction → skill consumption。论文为便于研究表述，将前两项合并到 experience generation 的概念阶段。
- 官方框架公开两种 extraction method：`sequential` 与 `parallel`；论文主实验使用 per-trajectory analysis + hierarchical consolidation 的 parallel-style 最小框架。
- 五个公开集成：ALFWorld、BFCL v4、SEAL-0、SpreadsheetBench、SWE-bench Verified。仓库提交 held-out test pool，并给每个 benchmark 独立 setup 文档。

## 实验口径

- 主矩阵：5 domains × 6 target models × 5 extractor models = 150 个 `Δ(E,M,D)` 单元。
- Table 1 精确计数：112 个正迁移、37 个负迁移、1 个零变化；作者在正文概括为 75% positive / 25% negative。
- 每个 domain 使用一致的 1:1 experience-generation / held-out test split；Table 1 的 Base 与 skill-augmented 条件均平均 3 次运行。
- 目标模型 6 个；Qwen3.5-9B 因不能稳定遵循结构化 extraction protocol，只作为 target，不作为 extractor。
- 闭源模型 reasoning/thinking level 设为 medium；开源 Qwen 由单节点 8×NVIDIA B200 上的 vLLM 服务。

## 不应外推

- 论文证明的是 **单个 domain-level skill 直接注入 system prompt** 的受控设置，不是大规模 skill library 的检索、组合、冲突与长期更新。
- `Δ` 是同一 domain 内的百分点变化；不同 benchmark 的绝对分数、难度和指标不可横向相加成总冠军。
- EE / TE 是对当前 extractor/target 集合的平均，换模型、版本、harness、预算或提示词后必须重测。
- “format effect 不显著”只在 SpreadsheetBench、四种语义保持的重写和当前 target 设置中成立；不能推成“格式永远不重要”。
- 46.4% → 73.8% 是 GPT-5.4 对 151 个高差距 skill pair 的 pairwise selection accuracy，不是 agent 任务成功率。
- validated meta-skill 的 +1.55 pp 是 3 domains × 3 targets 的 9 个单元相对原始 skill 的平均改进；不是所有 domain、所有 target 或生产环境的普遍收益。
- Appendix D 的行为机制来自 SpreadsheetBench 中 GPT-5.4 与 Qwen3.5-9B 的定性 trajectory analysis；它解释代表案例，不构成全模型因果证明。
- Alternative harness 只复测 SpreadsheetBench 子集，平均 +0.4 pp 且仍有负迁移；它缓解 harness artifact 疑虑，不等于跨 harness 全面稳健。

## 设计资产事实

- 主色从论文自身采样：Microsoft blue `#2563EB`、paper white `#F7F8FA`、ink `#0F172A`、slate `#475569`、positive green `#16803A`、negative red `#B42318`。
- 内容必需图来自本地 PDF 的物理页裁图；所有裁图保留 `PDF p.N / Figure / Table` locator。
- 解读者重绘必须显式标注，不冒充论文图；不使用 stock photo 或通用 AI 插画。
