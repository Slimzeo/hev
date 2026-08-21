# Rattler 运行时环境隔离技术图设计规范

## 目标

用一张简洁的 HTML 技术图解释：当包安装与依赖管理全部交给用户和 pip 时，Rattler 如何仅凭现成 Prefix 构造激活上下文并运行命令。它是工程讨论材料，不是完整包管理流程图。

## 内容边界

- 主流程：`existing prefix path -> Activator -> activated environment -> child process`。
- Prefix 内容由用户与 pip 直接修改；图中不出现 desired state、求解、事务或安装。
- 环境隔离落在 PATH 选择、CONDA_PREFIX、激活栈和 Prefix 自身 Python/site-packages。
- `PrefixData` 只作为可选的 Conda 元数据视图，并明确不能盘点 pip 安装状态。
- 冲突只讨论激活/生命周期边界；跨环境版本隔离天然成立，同环境依赖冲突归 pip。

## 视觉规范

- 只交付一个 HTML，不生成 PNG 或其他图片副本。
- 白底、灰色关系线、灰蓝节点和单一蓝色强调。
- 节点使用矩形框，关系使用细箭头，文字只保留类型名、角色和必要语义。
- 不使用品牌大色块、渐变、阴影、网格背景、装饰图标、图片资产、动画或多个视觉方向。
- 风格参考用户给出的 ER 图、状态机和交互流程图：技术关系优先，装饰让位于可读性。
