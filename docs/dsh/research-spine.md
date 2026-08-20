# 论文研究主线：问题、方法、依据与可信边界

> 论文：*A Programming Paradigm for Spatiotemporal Composability*
> 版本：用户提供的 2026-08-13 本地技术手稿，88 个物理 PDF 页；venue、DOI、arXiv 未确认。

## 一句话 thesis

论文要解决的是：**运行时组件在同一进程内反复加入、退出、替换时，怎样既只撤销离场组件自己的副作用，又让依赖它的组件在 provider 出现、消失或换身份时按正确顺序重新解析和迁移生命周期。**作者把前者称为时间可组合性（temporal composability），把后者称为空间可组合性（spatial composability）；方法是把 effect/coeffect 从静态分析概念提升为运行时机制，再用 fiber lifecycle calculus 把两者组合起来。主要依据是带显式前提的形式证明；Cordis 说明这些构造可实现；Koishi 只提供存在性与采用度证据。关键边界是：这不是性能论文，也没有证明任意外部副作用或任意组件代码都可安全回滚。

## 1. 为什么写这篇论文？

传统组合主要在编译期或词法作用域内固定：函数调用、模块导入、类继承、RAII、静态依赖解析都有预先确定的边界。插件系统和未来可能自修改的 Agent Harness 却会在运行时增删、替换组件，且希望保留同一进程中的缓存、连接、会话与在途计算。论文认为，这类“动态组合”缺少与静态组合同样清晰的形式基础。

论文用两组动机把缺口具体化：VSCode executable extension 无法单独 live unload，`activate` 与 `deactivate` 分离使完整清理难核验；进程/容器重启虽然能回收资源，却以过粗的进程/服务为边界，丢弃进程内状态，并无法自然表达同地址空间组件间的动态依赖。

**Locator：**PDF p.4–6，§1.1–1.3。VSCode Top-100 数字的抓取日期为 2026-06-09，只是动机样本，不代表全部插件系统。

## 2. 精确研究问题是什么？

论文提出两个正交问题：

1. **时间问题：**组件卸载后，能否完整撤销它对共享环境的资源分配、事件注册与状态修改，同时保留其他组件已经产生的独立变化？
2. **空间问题：**组件能否声明依赖；当 provider 出现、消失或换身份时，runtime 能否自动判断 consumer 应激活、停用还是保持不变，并保证 provider/consumer 的退出顺序？

两者必须一起解决。只有 cleanup，无法保证依赖变化时谁先退场；只有依赖注入，无法保证组件离场后它留下的效果被完整撤回。

**Locator：**PDF p.4–6，§1.1、§1.3；p.8，§2.3。

## 3. 既有方法为什么不够？

- **进程/容器重启：**能粗粒度回收，却丢失整个进程的局部状态；组件间本可本地调用的关系被推到网络边界。
- **手写 deactivate/dispose：**创建与清理分离，完整性依赖作者纪律；半完成异步初始化、交错组件和 dependent drain 没有统一语义。
- **静态 effect/coeffect：**能描述“计算修改什么、需要什么”，但通常在编译期与固定词法 scope 中工作，不能预知部署后才出现的组件和配置。
- **初始化期 DI / 普通 HMR：**可以接线或换代码，却不自动给出 provider 消失后的异步生命周期协调与可组合恢复。相关工作中的 OSGi/iPOJO 最接近可用性反应式组件模型，但其 teardown 仍主要依赖手写、同步回调。

**Locator：**PDF p.4–8；相关工作比较见 p.76–79。

## 4. 本文的方法是什么？

方法不是单一算法，而是一套五层运行时范式：

1. **Revertible effects。**把 effect 写成 `Γ → Γ × (Γ → Γ)`：一次修改同时返回新 context 与针对该次应用状态的 inverse；runtime 按 LIFO 组合 inverse。
2. **Reactive coeffects。**把依赖环境写成 typed context 与 specification；每次 context 变化被分类为 activating、deactivating 或 neutral，并触发生命周期。isolation 改变 key 解析到谁，interception 改变 binding 如何使用。
3. **Unified context。**把 effect context 与 coeffect context 合一；用 coeffect operation 所尊重的观测等价与 key-level mediation，给跨组件 effect independence 提供可检查的接口纪律。
4. **Component/fiber calculus。**component 是 `(dependencies d, provisions p, witnessed effect e)`；每个实例成为 fiber，进入 Inactive / Reloading / Active / Unloading 四态。十条小步规则处理插入、退役、撤出、迭代、异步、失败和依赖 drain。
5. **Cordis implementation。**`ctx.effect`、`ctx.set/get/use` 与 `fiber.target/committed/dispose/inertia` 对应形式对象；声明式 loader 做 configuration reconciliation，HMR 通过 cache backup 与 fiber rebuild 避免 half-reloaded state。

**Locator：**PDF p.9–27（局部机制）；p.28–53（calculus/metatheory）；p.54–66（Cordis、loader、HMR）。

## 5. 方法凭什么说“有效”？证据阶梯是什么？

### 第一级：形式证据——证明模型内的核心性质

- Theorem 7/16/20：在 inverse witness 与相应 independence 条件下，effect 可组合、可逆序或选择性撤销。
- Theorem 59：十条规则保持 registry well-formedness。
- Theorem 61/Corollary 62：交错执行中，一个 fiber 的 accumulator 只撤销自己的 tracked contribution。
- Theorem 63/64：provider 先于 consumer 激活、晚于 consumer 卸载；一次 transition 不跨两个 dependency resolution。
- Theorem 66：在 precedence 无环、iterator 有界、fiber 名有限等条件下，无 lifecycle deadlock 且最终 quiescent。
- Theorem 73：在无失败、pairwise independence、provision totality 等条件下，相同 orchestration 输入的成功执行收敛到与最终静态装配等价的状态。

**为什么这一级可靠：**结论、状态空间和前提都被形式化；Table 1 穷举十条 rule 的 state map 与 control-field write set；证明覆盖任意合法 interleaving/scheduling，而不是只演示一条 happy path；强结论旁同时写出失效条件。可靠性只在该模型与前提内成立。

**Locator：**PDF p.9–17；p.39–53，尤其 Table 1 p.40、Theorem 61 p.44–45、Theorem 63 p.45–46、Theorem 66 p.47–49、Theorem 73 p.52–53。

### 第二级：构造证据——形式对象有可执行对应

Table 2 将理论对象逐项映射到 Cordis API/字段，Algorithm 1–10 给出 effect tracking、notification、component lifecycle、context access、isolation reassignment 与 transactional HMR 的伪代码。

**为什么这一级可靠：**它说明方法不是只存在于符号里，且关键控制点能落到具体 runtime state；但论文没有给出机器检查的 refinement proof，因此只能支持“构造可实现、设计对应清楚”，不能单独证明实现无 bug 或性能更优。

**Locator：**PDF p.54–66，Table 2 p.55，Algorithm 1 p.56，Algorithm 5 p.59–60，Algorithm 10 p.66。

### 第三级：案例证据——真实开放生态中存在且被采用

Koishi 是建立在 Cordis 核心模型上的开源 chatbot framework，论文报告其四年间积累 4000+ 社区插件，服务端与 web console 两个不同 runtime 都使用该组合模型。

**为什么这一级有价值：**它能反驳“抽象只在玩具系统成立”，支持存在性、表达力与采用度。**为什么它不够强：**这是单一 TypeScript 生态的观察性案例，没有 controlled baseline；Koishi 当前使用 Cordis v3，而论文描述 v4。因此它不证明 v4 全语义已在同等规模生产验证，也不证明延迟、吞吐、资源或生产率优势。

**Locator：**PDF p.66–67，§5.3 及 footnote 4。

## 6. 最终可以相信什么？

可以相信的精确版本是：**若组件所有可恢复修改都经统一 context，被正确 inverse 见证；跨 fiber 操作满足 independence/confinement；依赖图、iterator 与 fiber 集满足相应有限性；并在 confluence 场景下无失败且 provision total，那么该 calculus 能在任意合法交错下提供局部恢复、依赖有序退出、生命周期进展与成功最终状态合流。**Cordis 展示了这套模型的具体实现路径，Koishi 表明相近核心模型可以支撑真实开放插件生态。

不能从论文推出：

- runtime 会自动验证 inverse 正确；
- 隐藏全局状态或绕过 context 的操作仍享受保证；
- 网络发送、支付、邮件等越过 system boundary 的 emission 可自动倒带；
- failure 下仍有完整 confluence；
- Cordis 比容器、DI、HMR、OSGi 或 FRP 更快；
- self-evolving agent harness 已完成生产验证。

**Locator：**证明条件见 p.43–53；system boundary / acquisition / emission 见 p.67–68；案例威胁见 p.67；Agent Harness 未来验证见 p.79。
