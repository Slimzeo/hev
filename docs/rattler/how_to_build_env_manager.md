
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


