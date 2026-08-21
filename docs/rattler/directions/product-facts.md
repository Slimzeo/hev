# Rattler 模型图事实清单

- 事实来源优先级：当前本地 Rattler 仓库源码 > 仓库 README 视觉资产 > 外部搜索摘要。
- Rattler 是提供 Conda 生态常用能力的 Rust crate 集合；本图不声明具体发布版本。
- 官方视觉锚点来自仓库 `assets/rattler-readme-image.png`：Prefix.dev 黄色背景、黑色字与蛇箱插画。
- `Prefix` 源码定义为 Conda environment prefix directory 的路径包装。
- `PrefixData` 是 Prefix 内 `conda-meta` 目录的惰性视图，并按 `PackageName` 索引 `PrefixRecord`。
- `PrefixRecord` 的源码注释明确称其为“安装在某个 environment 中的单个 package 记录”。
- `PrefixRecord` 内嵌一个 `RepoDataRecord`；`RepoDataRecord` 内嵌一个 `PackageRecord`。
- `PackageRecord` 包含 name/version/build/platform/subdir/depends/constrains/hash 等包属性。
- `RepoDataRecord` 在 PackageRecord 基础上增加 archive identifier、URL、channel。
- `PrefixRecord` 在 RepoDataRecord 基础上增加 requested specs、实际文件、PrefixPaths、缓存来源 Link 等安装事实。
- `SolverTask` 的求解输入包括可用包、locked packages、pinned packages、virtual packages、specs 和 constraints；`SolverResult` 输出 `Vec<RepoDataRecord>`。
- `Transaction::from_current_and_desired` 对当前状态与目标状态做差，产生 Install / Change / Reinstall / Remove 操作。
- `PackageCache` 管理磁盘上已解压的 Conda package cache，可包含多个 cache layer。
- 包内 `PathsJson` 是安装指令；PrefixRecord 内 `PrefixPaths` 是安装结果。
- `ClobberRegistry` 内部使用 `PathResolver` 路径 trie 处理多个包占用相同路径的冲突。
- 本图把 Solver 与 Installer 画为执行服务，把 SolverResult、Transaction、ClobberedPath 画为派生模型，避免将行为组件和持久领域实体混为一类。
