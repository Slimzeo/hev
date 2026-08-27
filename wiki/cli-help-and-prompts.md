# CLI Help 与 Prompt

hev 将命令说明、诊断信息和恢复提示分开处理：

- Cobra `Use`、`Short`、`Long` 和 `Example` 说明命令在执行前如何使用。
- `message` 记录失败事实和调用链上下文，供日志与 trace 排障。
- `prompt` 面向前端、用户和 Agent，说明失败后的下一步操作。
- `code` 只表示稳定错误类别；同一个 400 可以对应多种不同 prompt。

Go 使用匿名嵌入组合响应类型。`BaseResponse` 保存公共字段，Environment、Environment List、Session、Skill Added、Skill Removed 和 Error 响应分别声明自己的 `data`。这些响应只有数据不同，不需要 Response interface。

错误产生位置知道最准确的恢复动作，因此内部 `response.Error` 同时携带 status、message 和 prompt。最外层只负责序列化：JSON 输出保留两个字段，文本输出显示诊断行和 `hint:` 行。未分类内部错误使用统一重试提示。

DSH adapter 保持相同语义：

- `/hev` 命令把诊断 message 写入日志，把 prompt 返回给命令前端。
- `hev_*` model Tools 把诊断 message 写入 DSH 日志，把 prompt 作为失败结果交给 Agent。
- Tool description 是调用前帮助；`hev-guide` 负责多步骤工作流，不代替单次错误 prompt。
