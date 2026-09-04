# v0.2 实施计划

## 1. 统计模型与算法

- [x] 在 `internal/stats` 增加 Options、关系统计、响应和月度类型。
- [x] 对消息副本稳定排序，统一基础统计与关系统计的数据源。
- [x] 实现 Session/Turn 重建、双方开始/结束、P50/P90。
- [x] 实现自然日、活跃日日均、最长连续活跃和月度聚合。
- [x] 补充乱序、边界 gap、同方 Burst、跨日跨月和空响应测试。
- [x] 运行 `go test ./internal/stats`。

## 2. 时期解析与对比

- [x] 实现 `YYYY`、`YYYY-MM`、日期闭区间解析。
- [x] 实现不修改原会话的时期过滤和两个时期摘要/Delta。
- [x] 覆盖非法格式、反向范围、空时期、零基线与本地时区测试。
- [x] 运行 `go test ./internal/stats`。

## 3. CLI 输出

- [x] 抽取共用 session-gap 和 format 参数校验。
- [x] 扩展 `stats` 和 `analyze`，保持默认文本输出兼容。
- [x] 新增 `compare` 命令和文本展示。
- [x] 实现无装饰的 JSON Envelope；批处理警告写入 stderr。
- [x] 增加 CLI 层参数与 JSON 解码测试。

## 4. HTML 与隐私默认值

- [x] 增加 `ReportOptions`，保留旧函数兼容包装。
- [x] 生成关系、响应和月度趋势 View Model。
- [x] 扩展模板，保持响应式与完全离线。
- [x] 默认移除原文，`--include-content` 显式启用。
- [x] 更新 HTML 测试，覆盖默认隐藏和显式展示。

## 5. 文档与发布检查

- [x] 更新 README 的命令示例、指标定义、时区及隐私说明。
- [x] `gofmt` 所有修改过的 Go 文件。
- [x] 运行 `go test ./...` 和 `go vet ./...`。
- [x] 运行仓库配置的 golangci-lint。
- [x] 用合成样例执行 text、JSON、HTML 和 compare smoke test。
- [x] 检查 `git diff`，确认没有真实聊天记录和无关文件。

## 风险点与回滚点

- `internal/stats.Stats` 是跨 CLI、AI、报告的公共数据契约；完成第 1 步后先跑全量
  编译测试再继续。
- JSON 模式会触及现有打印路径；若无法保证 stdout 纯净，优先限制为单文件并在
  PRD 重新评审，不用隐藏式 workaround。
- 报告默认隐藏正文是有意的隐私行为变化；保留 `--include-content` 作为显式兼容
  路径。
