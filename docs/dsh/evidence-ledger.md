# Claim–Evidence Ledger

> 论文版本：本地 `paper.pdf`，2026-08-13，88 个物理页面。页码均指 PDF 物理页码。

| Claim | 归属 | Locator | 证据能支持什么 | 不能支持什么 |
| --- | --- | --- | --- | --- |
| 动态组合包含时间与空间两个正交维度 | 论文主张与问题定义 | p.4–6, §1.1–1.3 | 给出 temporal/spatial composability 的工作定义与动机 | 不是经验研究证明的唯一分类法 |
| Revertible effect 可在单组件范围恢复上下文 | 形式化结论 | p.9–17, Def.1–19, Thm.7/11/16/20 | inverse 随 effect 返回并逆序组合；在 witness/independence 条件下可选择性恢复 | runtime 不会自动验证 inverse 正确；外部不可逆 emission 不在保证内 |
| Reactive coeffect 提供局部空间组合 | 形式化机制 | p.17–22, Def.22–31 | specification 与 context change 可分类为 activating/deactivating/neutral；isolation/interception 改变解析与使用 | 不自动解决接口版本兼容或不可信代码 sandbox |
| 统一 context 让 coeffect 的观测等价为 effect 独立性提供条件 | 形式化桥梁 | p.22–27, Def.32–41, Thm.42 | 将 effect/coeffect 组合成一种 context paradigm，并解释 distinct keys 的独立性 | 依赖所有共享位置经由 key/operation 暴露；隐藏状态会破坏纪律 |
| Calculus 将局部保证提升到交错组件系统 | 论文核心增量 | p.28–53, §4 | component/fiber/registry 与 10 条 lifecycle rule 构成 operational semantics | 结论依赖每条 theorem 的额外假设，不是无条件保证 |
| Registry well-formedness 被规则保持 | 定理 | p.42–43, Thm.59 | 父子、provision 唯一性、committed view 与 provider 安装状态保持一致 | 不证明业务逻辑正确、inverse 正确或没有 dependency cycle |
| 一个 fiber 的 accumulator 能只撤销自身贡献 | 定理 | p.43–45, Thm.61, Cor.62 | pairwise independence 下，交错执行中仍有 recovery exactness / terminal recovery | 若 effect 不独立、注册子 fiber 参与执行或 witness 不成立，不能直接套用 |
| provider 的卸载晚于依赖它的 consumer | 定理 | p.45–46, Thm.63 | consumer 在自身 teardown 全程仍能读取 committed dependency | 只适用于 calculus 的 guarded withdrawal；不能覆盖任意外部引用 |
| 一次 transition 不跨越两个 dependency resolution | 定理 | p.46–47, Thm.64 | target 变化会 finish 或 divert/raise 后恢复，保证 resolution coherence | 不等同于 FRP 的全局 glitch freedom |
| Lifecycle 最终到达 quiescent state | 定理 | p.47–49, Thm.66 | precedence acyclic、iterator 有界、fiber 名有限且只走 lifecycle steps时，无死锁且终止 | orchestration 持续注入、cycle、无界 iterator 不在结论内 |
| 最终状态与按最终配置静态装配一致 | 定理 | p.49–53, Thm.73 | 无失败、pairwise independent、provision total、quiescent 等条件下，normal form 唯一到观测等价/控制字段等价 | 不保证过程中的 emission 相同；failure 可导致真实分歧 |
| Cordis core 对应形式模型 | 实现说明 | p.54–61, Table 2, Alg.1–6 | 给出 ctx.effect / set/get / use 与 fiber lifecycle 的可执行映射 | 伪代码不是性能 benchmark；inverse witness 由作者负责 |
| Loader 可按 declarative config 增量 reconciliation | 实现说明与定理应用 | p.61–64, Def.74, Alg.7 | 字段级最小扰动、realm reassignment、依赖无需手排 load order | 组件若不 total，其 active set 还取决于 config/运行行为 |
| HMR 使用分类、stale detection 与事务式 reload | 实现算法 | p.64–66, Alg.8–10 | import 失败时恢复 module cache 并重建旧 fiber，避免 half-reloaded state | 不代表任意外部 side effect 均可回滚 |
| Koishi 表明抽象能支撑真实开放插件生态 | 案例证据 | p.66–67, §5.3 | 单一 TypeScript 生态中存在性、表达力、通用性与采纳度 | 非 controlled comparison；无 overhead/productivity 数字；Koishi 用 v3 |
| Self-evolving agent harness 是未来验证方向 | 作者展望 | p.79, §8 | 说明论文作者认为该场景值得验证的两类保证 | 不是已完成实验或生产落地 |

## 关键成立条件清单

1. Atomic effect 的 inverse 在实际应用状态上满足 witness；Cordis runtime 不自动证明它。
2. 不同 fiber 的 effect/iterator 满足论文定义的 pairwise independence；共享操作需通过可交换 coeffect operation 介导。
3. Effect 的读写被 confined 到本 fiber、声明的 coeffect 与允许的注册操作。
4. Provision 在同一 realm 内不冲突；calculus 简化为单 realm，完整实现通过 isolation 扩展。
5. Progress/Confluence 要求 precedence acyclic、iterator 长度有限、fiber 集合有限。
6. Confluence 进一步要求 quiescent、无 failed fiber、component total on its provision。
7. 系统边界内的 acquisition 必须可排他修改且可恢复；越界 emission 只能不追踪或补偿。

## 未确认或论文未覆盖

- venue / DOI / arXiv id。
- Cordis v4 对比其他 runtime 的开销。
- 多语言实现与跨生态的外部有效性。
- 开发者漏写 inverse 的真实发生率与调试成本。
- 接口兼容、版本选择与 key collision 的统一方案。
- Agent 自演化高频替换下的故障率、回滚时延与安全边界。

