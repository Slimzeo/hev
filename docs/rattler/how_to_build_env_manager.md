
## 功能考虑
MVP思路：
1. 创建env folder 
2. 将skill link_package
3. 设置访问path

但是如果下一次：该env需要下载新的包或者更新总体环境
（TODO: Skill是否仍然存在更新的概念）
必须要支持：
1. env里面已经有了什么？
2. 最终期望结果？
3. 旧的怎么处理？
4. 新的怎么装？
5. 不同skill但是同名怎么处理？
6. 可恢复地写在？（是否该支持？有必要吗？）
7. Skill安装过程调度：下载、链接以及失败处理
....


## 环境隔离总逻辑


共享缓存Cache存放真正Val
构建different prefix存放Pointer
但是实际上，对应到operation system里面的file存在形式是：
- hardlink
- reflink
- symlink
- copy
存在这些区分

或者换个说法：
不可变 Store 保存 Skill Artifact；
每个 Environment 保存对 Artifact ID 的引用，
并按需通过目录级 symlink 物化成可访问的 Skill 目录。

因为：

- hardlink 是同一个 inode 的多个目录项；
- symlink 才更像“路径指针”；
- reflink 是写时复制；
- copy 是完全独立文件。

所以 prefix 并不一定全是 Pointer，它是一个“环境物化视图”。



也就是说，环境隔离本身甚至算不上什么复杂算法
复杂代码主要都在处理这些工程问题：

- 更新已有环境时，哪些包新增、删除、替换；
- 文件能否 hardlink、reflink，还是必须 copy；
- 脚本和二进制里写死的旧 prefix 要替换；
- 两个包同时提供 bin/python 怎么办；
- Python 改版本后 noarch 包要重新链接；
- Windows 文件正在运行，无法直接删除；
- 安装到一半失败，实际文件不能和 conda-meta 对不上；
- 为了快，还要并发下载、解压、链接，同时避免路径竞争。

## 工程设计
总体结构

Prefix / PrefixRecord
  → “一个环境是什么、怎么记账”

link_package
  → “怎么把包变成环境文件”

Transaction
  → “已有环境如何变成目标环境”

Solver
  → “目标环境具体应该是什么”


## 环境隔离算法
Prefix: env_root_folder
> eg: P = /Users/you/miniconda3/envs/ml

这个路径就是 ml 环境的 prefix。

之所以叫prefix，因为同一个环境里所有安装文件路径，理应有同一个公共前缀
所以Prefix成为环境隔离关键概念
> /Users/you/miniconda3/envs/ml/bin/python
> /Users/you/miniconda3/envs/ml/lib/libpython3.11.dylib
> /Users/you/miniconda3/envs/ml/conda-meta/python-3.11.json
> \______________________________/
>               prefix

环境名：ml
环境 prefix：/Users/you/miniconda3/envs/ml

名字只是给人看的索引；真正决定环境身份和文件位置的是这个完整路径。
“prefix replacement”：将构建期 prefix 占位符替换为运行时 target prefix。

某些包构建时，文件中会写死构建机器的路径。例如包在构建机上原本装到了：
eg:
/opt/conda-build-placeholder/bin/python
现在你把它安装到：
/Users/you/miniconda3/envs/ml
那么包里的脚本可能有：
#!/opt/conda-build-placeholder/bin/python
必须改成：
#!/Users/you/miniconda3/envs/ml/bin/python


## 关键模型
外部html有图


## MVP——一期


我们先不要支持Skill包管理加载
而是对user自己下载出来的Skill进行环境管理，复杂度减少：意图、求解、目标、安装，，

因为我们没有install的包管理下载需求，因此rattler存在的model：
 EnvironmentYaml / desired / Solver / Transaction / Installer -> 我们都用不上
核心接口也很少：

  create(name, python)
  list()
  inspect(name)
  activate(name)
  run(name, command)
  remove(name)
比如所用户创建了一个环境env，然后自己pip install了东西（注意这里是pip），我们不关心install，这个是user自己的命令决定的
但是一旦在我们的env里面，就天然与其他env隔离了这个依赖的加载
所以我说，我想要做的和install感觉没关系。我们顶多思考一下，如何处理冲突，所以哪里来的env yaml的这种“desired”的想法和设定？
我感觉这样的需求的话，不沾边啊。

暂时不用考虑做的多复杂。

## 二期策略

二期可以增加对skill的统一包管理机制，对标完善的env环境



## 可以抽出的Rattler一期整体链路

完整 run 调用链

  这个需求真正应该阅读的主链是：

  rattler CLI
  │
  ├─ crates/rattler-bin/src/main.rs
  │    main()
  │      ↓
  │    async_main()
  │      ↓
  │    Opt::parse()
  │      ↓
  │    Command::Run
  │
  ├─ crates/rattler-bin/src/commands/run.rs
  │    run(opt)
  │      ↓
  │    将 target_prefix 转为绝对路径
  │      ↓
  │    ShellEnum::from_env()
  │      ↓
  │    run_command_in_environment(...)
  │
  ├─ crates/rattler_shell/src/run/mod.rs
  │    run_command_in_environment(prefix, command, shell, ...)
  │      ↓
  │    Activator::from_path(prefix, shell, platform)
  │      ↓
  │    构造 ActivationVariables
  │      ↓
  │    Activator::run_activation(...)
  │      ↓
  │    得到 activated_env: HashMap<String, String>
  │      ↓
  │    tokio::process::Command
  │      .envs(activated_env)
  │      .status()
  │
  └─ 子进程
       PATH 优先找到 prefix/bin/python
         ↓
       Python 根据自己的 sys.prefix / sys.path
         ↓
       加载 prefix/lib/pythonX.Y/site-packages

  入口对应：

  - CLI 分发：crates/rattler-bin/src/main.rs:115
  - run 命令：crates/rattler-bin/src/commands/run.rs:28
  - 子进程执行：crates/rattler_shell/src/run/mod.rs:30
  - Activator 构造：crates/rattler_shell/src/activation.rs:367
  - 激活计算：crates/rattler_shell/src/activation.rs:462

  这里非常关键：

  > Rattler 负责把正确的 prefix/bin 放到 PATH 前面；后续 Python 如何找到 site-packages，是 Python 解释器自己的行为，不是 Rattler 的代码。




  应该按“角色”分类，而不是看到 struct 就全部算 model。

   类型                       角色                                                是否核心
  ━━━━━━━━━━━━━━━━━━━━━━━━━  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  ━━━━━━━━━━━━━━━━━━━━
   prefix: &Path              环境身份                                                  是
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   ActivationVariables        激活前宿主进程状态                                        是
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   Activator<T>               基于 Prefix 预计算的激活计划                              是
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   ActivationResult<T>        Shell 激活脚本及新 PATH                                   是
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   HashMap<String, String>    计算完成的子进程环境变量                                  是
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   ShellEnum / Shell          Bash、Zsh、Fish、PowerShell 等语法适配                    是
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   ShellScript<T>             生成具体 Shell 命令                                       是
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   PrefixData                 conda-meta 查询视图                                     可选
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   PrefixRecord               某个 Conda 包的安装记录                                 可选
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   LazyRecordEntry            PrefixRecord 的懒加载缓存                     否，纯实现细节
  ─────────────────────────  ────────────────────────────────────────  ────────────────────
   Prefix                     安装阶段使用的路径包装                    你的主链不一定需要

  你的直觉中可能把 PrefixData 看成“环境状态核心”，但在当前需求里它不是。

  run_command_in_environment 完全不会调用 PrefixData：

  run_command_in_environment
    → Activator::from_path
    → activation
    → child process

  PrefixData 的真实调用链是另一条：

  rattler list -p /env
    ↓
  commands/list.rs
    ↓
  PrefixData::new(prefix)
    ↓
  扫描 prefix/conda-meta/*.json
    ↓
  懒加载 PrefixRecord
    ↓
  展示 Conda 包列表

  对应入口在 crates/rattler-bin/src/commands/list.rs:28。

  而用户执行：

  python -m pip install requests

  通常只会写：

  site-packages/requests/
  site-packages/requests-*.dist-info/

  不会写：

  conda-meta/requests-*.json

  所以 PrefixData 看不到这些 pip 包。它最多能看到由 Conda 安装的 pip 这个包本身，不能表示 pip 后续安装了哪些 distributions。

  activation.rs 内部真正发生什么

  Activator::from_path 从已有 Prefix 读取四类东西：

  target_prefix
    ├─ 平台对应的 PATH entries
    │    Unix:    prefix/bin
    │    Windows: prefix, Scripts, Library/bin, ...
    │
    ├─ etc/conda/activate.d/*
    ├─ etc/conda/deactivate.d/*
    └─ 环境变量
         ├─ conda-meta/state
         └─ etc/conda/env_vars.d/*.json

  然后构造：

  Activator {
      target_prefix,
      shell_type,
      paths,
      activation_scripts,
      deactivation_scripts,
      env_vars,
      post_activation_env_vars,
      platform,
  }

  ActivationVariables 则是激活之前的宿主状态：

  当前 CONDA_PREFIX
  当前 PATH
  PATH 采用 Replace / Append / Prepend
  当前全部环境变量

  之后 activation()：

  1. 如果已有旧 CONDA_PREFIX，移除旧 Prefix 的 PATH 并运行 deactivate 脚本；
  2. 把新 Prefix 的路径放入 PATH；
  3. 增加 CONDA_SHLVL；
  4. 备份旧 CONDA_PREFIX；
  5. 设置新 CONDA_PREFIX；
  6. 应用 Prefix 中的环境变量；
  7. 执行 activate 脚本。

  所以：

  ActivationVariables = 激活前状态
  Activator            = 要激活哪个 Prefix，以及从那里读到了什么
  ActivationResult     = 应该对 Shell 做哪些修改

  这三个才形成完整模型，不只是你贴出的两个 struct。

  为什么分在这些 crate 和目录里

  真实依赖方向是：

  rattler-bin
      │
      ├──依赖──> rattler_shell
      │              │
      │              └──依赖──> rattler_conda_types
      │
      └──还依赖──> rattler / solver / cache / gateway / ...
                   但这些不在你的 run 主链中

  各层职责：

  rattler-bin
    CLI adapter / composition root
    解析参数、选择子命令、调用库

  rattler_shell
    环境激活领域能力
    处理 Shell 差异、PATH、CONDA_PREFIX、脚本和子进程

  rattler_conda_types
    Conda 生态共享数据类型
    Platform、PrefixRecord、PrefixData 等

  rattler
    包下载、安装、Transaction、link/unlink
    你的当前需求不使用

  这也是为什么 prefix_data.rs 在 rattler_conda_types：

  - 它围绕 PrefixRecord 和 conda-meta 建立只读视图；
  - 它不负责激活，也不负责执行命令；
  - CLI、Python binding 或其他 crate 都能独立复用它。

  而 activation.rs 放在 rattler_shell：

  - 激活是 Shell 行为；
  - 它需要理解 Bash、Zsh、Fish、PowerShell 等语法；
  - 不应该为了设置 PATH 被迫依赖下载器、Solver 或 Installer。

  shell/mod.rs 其实也是核心文件，因为它定义了：

  Shell trait
  ShellEnum
  ShellScript
  不同 Shell 的 set_env_var / set_path / run_script 实现

  crates/rattler_shell/src/shell/mod.rs:22 把“激活语义”与“具体 Shell 语法”隔离开。

  至于：

  rattler_shell/src/run/mod.rs

  这是普通 Rust module 约定：

  pub mod run;

  既可以对应：

  src/run.rs

  也可以对应：

  src/run/mod.rs

  选择目录形式通常是为了允许以后增加：

  run/
  ├── mod.rs
  ├── process.rs
  └── error.rs

  现在只有 mod.rs，并不表示它是 main。

  对你需求真正相关的文件集合

  核心 CLI 链：

  - crates/rattler-bin/Cargo.toml:14
  - crates/rattler-bin/src/main.rs:71
  - crates/rattler-bin/src/commands/run.rs:28
  - crates/rattler-bin/src/commands/shell_hook.rs:48

  核心运行时链：

  - crates/rattler_shell/src/activation.rs:81
  - crates/rattler_shell/src/run/mod.rs:30
  - crates/rattler_shell/src/shell/mod.rs:22

  可选观察链：

  - crates/rattler_conda_types/src/prefix_data.rs:29
  - crates/rattler_conda_types/src/prefix_record.rs:340
  - crates/rattler-bin/src/commands/list.rs:28

  因此不是只有三个文件，而是：

  CLI 入口  4 个左右
  运行核心  3 个
  可选观察  3 个

  真正的热路径仍然很小，主要就是：

  main
  → commands::run
  → run_command_in_environment
  → Activator::from_path
  → run_activation
  → child process

  这正说明：当你不负责 package lifecycle 后，环境隔离本身确实没有那么复杂。Rattler 仓库大，是因为它还同时服务求解、缓存、安装、网络、索引、锁文件、上
  传、虚拟包、sandbox 等完全不同的能力，并不是每次运行一个环境都要穿过所有这些 crate。


## rattler-bin main函数解读示例


  它们是 rattler 这个 CLI 暴露的主要功能，不是整个 Rattler 仓库的全部能力。

  大致可以分类为：

   类别              命令
  ━━━━━━━━━━━━━━━━  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   环境创建与运行    Create、Run、ShellHook、List
  ────────────────  ─────────────────────────────────────────
   依赖与包查询      Search、Solve、Inspect、VirtualPackages
  ────────────────  ─────────────────────────────────────────
   包文件操作        Download、FetchFile、Extract、Link
  ────────────────  ─────────────────────────────────────────
   Prefix 修改       InjectIntoPrefix、RemoveFromPrefix
  ────────────────  ─────────────────────────────────────────
   系统集成          InstallMenu、RemoveMenu
  ────────────────  ─────────────────────────────────────────
   发布与认证        Auth、Upload
  ────────────────  ─────────────────────────────────────────
   CLI 辅助          Completion、Exec

  每个命令只是一个高层用例，里面还会调用大量底层 crate。
  例如：

  Command::Create
    → repodata gateway
    → virtual packages
    → solver
    → package cache
    → installer
    → prefix records

  而：

  Command::Run
    → rattler_shell
    → Activator
    → child process


  我们与 Rattler 的差异不能按命令表做减法

  更准确的功能裁剪如下：

   Rattler 能力                             我们一期
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Create                                   改成创建/注册一个空 Python 环境，不负责安装包
  ───────────────────────────────────────  ───────────────────────────────────────────────
   Run                                      保留，核心能力
  ───────────────────────────────────────  ───────────────────────────────────────────────
   ShellHook                                保留，核心能力
  ───────────────────────────────────────  ───────────────────────────────────────────────
   List                                     改成列环境或列某环境中的 pip 包
  ───────────────────────────────────────  ───────────────────────────────────────────────
   Search                                   删除远程 channel 搜索，新增本地跨环境查询
  ───────────────────────────────────────  ───────────────────────────────────────────────
   Solve                                    删除，交给 pip
  ───────────────────────────────────────  ───────────────────────────────────────────────
   Inspect                                  删除原语义，必要时重新定义为查看环境
  ───────────────────────────────────────  ───────────────────────────────────────────────
   VirtualPackages                          删除
  ───────────────────────────────────────  ───────────────────────────────────────────────
   Download / FetchFile / Extract / Link    全部删除
  ───────────────────────────────────────  ───────────────────────────────────────────────
   InjectIntoPrefix / RemoveFromPrefix      删除，包增删交给 pip
  ───────────────────────────────────────  ───────────────────────────────────────────────
   InstallMenu / RemoveMenu                 删除
  ───────────────────────────────────────  ───────────────────────────────────────────────
   Auth / Upload                            删除
  ───────────────────────────────────────  ───────────────────────────────────────────────
   Completion                               可选保留，普通 CLI 体验
  ───────────────────────────────────────  ───────────────────────────────────────────────
   Exec                                     删除，它实际会求解并创建临时环境



## rattler 相关代码链路整理


  1. 编译边界与入口

  crates/rattler-bin/Cargo.toml:18
  → crates/rattler-bin/src/main.rs:53
  → crates/rattler-bin/src/commands/mod.rs:1

  commands/mod.rs 现在明确说明：没有在这里声明的 create.rs /
  solve.rs / link.rs 等旧文件，即使物理上还在目录里，也不属于当前二
  进制。

  2. 运行环境隔离热路径

  crates/rattler-bin/src/commands/run.rs:28
  → crates/rattler_shell/src/lib.rs:1
  → crates/rattler_shell/src/run/mod.rs:36
  → crates/rattler_shell/src/activation.rs:371 中：

  Activator::from_path
  → Activator::activation
  → Activator::run_activation

  → crates/rattler_shell/src/shell/mod.rs:43
  → crates/rattler_conda_types/src/platform.rs:128
  → tokio::process::Command 启动目标进程。

  注释里也明确了真实隔离语义：它不是容器式“清空全部宿主环境”，而是
  继承父进程环境，再用目标 prefix 产生的 PATH、CONDA_PREFIX 等变化
  覆盖对应变量。

  3. ShellHook 分支

  crates/rattler-bin/src/commands/shell_hook.rs:48
  → Activator::from_path
  → Activator::activation
  → ShellScript::contents
  → 输出到 stdout。

  它只生成激活脚本，不启动命令，也不会直接修改父 Shell；调用者需要
  自己 eval/source。

  4. List 观测支线

  crates/rattler-bin/src/commands/list.rs:31
  → crates/rattler_conda_types/src/prefix_data.rs:52
  → crates/rattler_conda_types/src/prefix_record.rs:274
  → crates/rattler_conda_types/src/record_traits.rs:12

  这里专门标明了：

  - 只扫描 prefix/conda-meta/*.json。
  - 不读取 pip 安装状态。
  - LazyRecordEntry 只是内部懒加载缓存，不是环境领域模型。
  - 当前链使用的是 PrefixRecord::from_path，不是安装阶段的
    write_to_path。

  - 这条支线不参与环境激活和命令运行。

  5. CLI 辅助

  crates/rattler-bin/src/commands/completion.rs:60 也已标记，但明确
  说明它只通过 Clap 读取当前命令树，不接触 prefix。


