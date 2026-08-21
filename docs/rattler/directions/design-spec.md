# Rattler 环境隔离领域模型图：三方向统一设计规范

## 项目与目标

本次交付不是安装流程图、crate 依赖图，也不是面向开发实现的函数调用链，而是一张类似 ER/UML 的“领域模型关系图”。读者正在学习 Rattler，已经理解共享 Package Cache、独立 Prefix 和链接安装的基本概念，现在需要进一步回答：Rattler 代码里除了 Package 与 Prefix 之外还定义了哪些关键模型，这些模型之间是包含、来源、投影、求解结果还是变更计划关系。图必须让读者看懂 Rattler 没有一个厚重的 Environment 实体；环境的身份由 Prefix 路径承担，环境的已安装状态由 PrefixData 对 conda-meta 的视图与多个 PrefixRecord 共同表达。

## 目标受众与使用场景

目标读者是有后端经验、正在从源码学习包管理与环境隔离的工程师。图会被放在 `skill-env/docs/rattler` 下，与现有学习笔记配套使用，因此需要适合在 27 英寸显示器上独立阅读，也要方便后续嵌入 Markdown。它不是演示文稿，不需要翻页或动画；输出为 1920×1200 的单页 HTML 信息图，并同时生成 PNG 截图。所有文字以中文解释模型语义，Rust 类型名保持英文原名，避免把翻译名误认为源码中存在的 struct。

## 必须呈现的事实模型

核心模型链是 `PackageRecord → RepoDataRecord → PrefixRecord`。PackageRecord 描述包本身的身份、版本、build、平台、依赖和约束；RepoDataRecord 组合 PackageRecord，并补充 identifier、URL 与 channel，因此表达“仓库中可定位、可下载的具体包”；PrefixRecord 再组合 RepoDataRecord，并补充 requested_specs、实际文件、PrefixPaths、缓存来源 Link，因此表达“这个具体包在某个 Prefix 中的一次已安装事实”。PrefixRecord 应位于图的语义中心，因为它可类比 Prefix 与 Package 之间带有丰富关系属性的关联实体。

环境侧必须呈现 Prefix、PrefixData、PrefixRecord、PrefixPaths 与安装后的 PathsEntry。Prefix 只包装环境根路径并保证 conda-meta 存在；PrefixData 是 conda-meta 的惰性状态视图，以包名索引多个 PrefixRecord；每个 PrefixRecord 拥有一份 PrefixPaths，PrefixPaths 再拥有多个安装后 PathsEntry。包侧必须呈现 Package Archive / PathsJson：它包含多个“安装指令 PathsEntry”，与 PrefixPaths 中的“安装结果 PathsEntry”要明确区分。

求解侧必须呈现 MatchSpec、GenericVirtualPackage、SolverTask 与 SolverResult。SolverTask 接收用户 specs、可用 RepoDataRecord、locked/pinned 包、虚拟包和约束；SolverResult 输出目标 RepoDataRecord 集合。缓存侧必须呈现 PackageCache、CacheKey 与 Extracted Package Dir，说明 CacheKey 可由 PackageRecord 身份生成，RepoDataRecord 的 URL 用于填充缓存，PrefixRecord.Link 保存缓存来源。变更侧必须呈现 Transaction 与 TransactionOperation，说明它由“当前 PrefixRecord 集合”和“目标 RepoDataRecord 集合”做差得到，操作有 Install、Change、Reinstall、Remove，之后由 Installer 执行。冲突侧可呈现 ClobberRegistry / PathResolver 与 ClobberedPath，但必须标为派生索引或执行支持模型，不能让它抢走核心领域模型的视觉中心。

## 关系语义与基数

需要明确：一个 PrefixData 对应一个 Prefix 路径视图；一个 Prefix 包含零到多个 PrefixRecord；一个 PrefixRecord 组合且仅组合一个 RepoDataRecord；一个 RepoDataRecord 组合且仅组合一个 PackageRecord；一个 PrefixRecord 拥有一个 PrefixPaths，PrefixPaths 拥有零到多个安装结果 PathsEntry；一个包内 PathsJson 拥有多个安装指令 PathsEntry；多个 PrefixRecord 可以引用同一个 PackageCache 中的已解压包目录；Transaction 拥有多个 TransactionOperation。关系线必须带动词，例如“组合”“索引”“求解为”“由 current + desired 做差”“链接来源”，不使用只有箭头没有语义的装饰线。

## 视觉约束

三版共享 Rattler README 中已验证的黄黑视觉锚点，但布局逻辑必须互异。禁止通用紫色渐变、emoji、装饰性指标卡和无意义图标。节点内只放帮助理解关系的关键字段，不完整抄写全部 Rust struct 字段。所有模型卡片必须标注类型分类：持久模型、值对象、派生模型或执行服务。正文不小于 15px，字段不小于 13px；关系文字必须清晰可读。官方 README 视觉图作为小型来源标识使用，不作为大面积背景。

## 三个方向

方向 A 是关系优先的“类图蓝图”：最接近 ER/UML，字段、组合和基数最明确。方向 B 是“包身份血缘”：用同一包从意图、仓库记录、缓存、安装记录到环境状态的演化作为主轴，但保留全部模型关系。方向 C 是“聚合边界地图”：用 Shared Package World 与 Prefix Boundary 两个大边界展示跨环境共享和环境内私有状态，Transaction 位于边界之间。

