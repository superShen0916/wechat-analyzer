# CLI 输出与日志

当前项目没有日志框架。用户输出集中在 `cmd/main.go`，使用 `fmt` 和
`fatih/color` 写入 stdout。

## 约定

- 彩色、带图标的展示只放在 CLI 边界。
- 内部包原则上返回类型化结果和错误，不直接打印。当前
  `internal/loader.LoadDir`、`stats.Stats.Print` 和
  `ai.AnalyzeConversation` 中的打印属于遗留行为，新代码不得继续复制。
- 输出必须确定性排序。渲染 map 前先排序 key，参考消息类型和活跃时段输出。
- JSON 等机器可读模式不得混入颜色、进度提示或额外说明。

## 隐私

禁止打印 API Key、供应商请求 Header、完整 Prompt 或聊天正文作为调试日志。
只有用户明确请求、且隐私行为已经说明的报告才可以包含消息内容。

## 后续演进

不要为了零散调试引入日志依赖。如果未来增加结构化日志，应写入 stderr，保持
stdout 可被脚本稳定消费。
