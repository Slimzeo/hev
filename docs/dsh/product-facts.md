# Product and paper facts

> 核验日期：2026-08-20（Asia/Shanghai）
> 用途：为论文研究 Demo 固化可核验事实；论文事实仍以本地 `paper.pdf` 为最高优先级。

## 论文身份

- 标题：*A Programming Paradigm for Spatiotemporal Composability*。
- 作者：Yifan Shi、Wei Zhang、Tianyi Cui。
- 单位：Peking University、DeepSeek-AI。
- 本次读取版本：用户提供的本地 PDF，88 个物理页面；PDF 元数据创建时间为 2026-08-13，Creator 为 Typst 0.15.1。
- 版本边界：PDF 正文没有给出 venue、DOI 或 arXiv 编号；截至核验时，公开检索也未找到规范出版页。因此产物统一称为“2026-08-13 本地论文/技术手稿版本”，不声称已经正式发表。
- 读取覆盖：全文 1–88 页均有原生文本层；核心正文 1–79 页逐页阅读，参考文献 80–88 页按研究谱系整理；关键公式、Figure 1/2、Table 1/2、Algorithm 1–10 结合页面渲染核对。

## 公开实现身份

- DeepSeek Harness 的官方 GitHub 仓库为 `deepseek-ai/deepseek-harness`；其公开 README 表述该项目由 Cordis 驱动，并指向本论文所描述的设计。
- Cordis 是论文中的 meta-framework；Koishi 是论文用来做生产存在性与采用度验证的案例。
- 论文第 66 页明确说明：Koishi 当前使用 Cordis v3，而论文描述 Cordis v4；二者共享核心组合模型，但 loader 与 effect/coeffect 语义在 v4 中被细化。不能把 Koishi 的现状直接当成论文中所有 v4 细节均已生产验证。

## 不应外推的事实

- 论文没有报告 Cordis 的延迟、吞吐、内存或 CPU overhead。
- 论文没有做开发者生产率的受控对照实验。
- “4000+ community plugins”是生态规模/采纳度证据，不是性能或正确性的实验数字。
- Self-evolving agent harness 是动机与未来验证方向；论文没有声称已经在该场景完成生产验证。

## 设计参照核验

- Distill：其官方站将自身描述为面向清晰解释、原生于 Web 的研究期刊，常使用交互媒体；2016–2021 运营，目前无限期暂停。Demo 只借鉴“正文与响应式图解共同解释”的媒介方法。
- Takram：其官方项目页将工作横跨 R&D、design engineering、digital products、brand design 等领域。Demo 只借鉴精密、跨设计与工程的研究呈现逻辑，不复制具体作品。

