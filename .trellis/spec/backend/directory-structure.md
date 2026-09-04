# Go 项目结构

## 包边界

本仓库是单一 Go Module。依赖应从 CLI 入口指向职责明确的内部包：

```text
cmd -> loader -> stats -> ai -> report
```

实际调用不是严格的线性链路：`cmd` 负责编排所有包，`ai` 使用 `loader`
和 `stats` 的类型，`report` 使用统计与 AI 结果。任何内部包都不得反向依赖
`cmd`。

- `cmd/main.go`：Cobra 命令、参数、终端展示、路径遍历和进程退出。
- `internal/loader`：输入数据结构和聊天导出文件解析。
- `internal/stats`：基于消息的确定性统计计算。
- `internal/ai`：模型供应商配置、Prompt、API 调用和响应解析。
- `internal/report`：报告 View Model、文件名安全处理和 HTML 渲染。
- `examples/`：仅存放合成样例，禁止提交真实聊天记录。

参考：`cmd/main.go`、`internal/loader/json.go`、
`internal/stats/analyzer.go`、`internal/ai/client.go`、
`internal/report/html.go`。

## 新增功能

- 可复用的分析逻辑放入 `internal` 包，不要写在 Cobra `RunE` 闭包中。
- 统计包只返回类型化数据，终端和 HTML 展示逻辑不得混入统计计算。
- 只有出现稳定、独立的职责时才新增包；小型辅助函数与调用方放在一起，
  例如 `internal/report` 中的 `sanitizeFilename`。
- 报告资源使用 `go:embed`；生成的 HTML 必须能够完全离线打开。

## 命名和文件

- 包名使用简短的小写名称，文件名遵循 Go 惯例。
- 测试放在同包 `_test.go` 文件中，与当前各内部包保持一致。
- 只有跨包调用的符号才导出；包内优先使用 `totalCharCount`、
  `getTopMessages`、`parseResponse` 这类非导出函数。

禁止循环依赖、没有明确职责的 `utils` 包，以及把业务计算塞进 HTML 模板
或 CLI 打印函数。
