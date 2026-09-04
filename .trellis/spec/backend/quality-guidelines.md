# 质量规范

## 必须通过的检查

每次行为变更都必须运行：

```bash
gofmt -w <修改过的 Go 文件>
go test ./...
go vet ./...
```

CI 会在 Linux、macOS、Windows 重复测试和 vet，并执行 golangci-lint；配置见
`.github/workflows/ci.yml`。

## 测试约定

- 单元测试与对应内部包放在一起。
- 同一契约的多个输入使用表驱动子测试，参考 `TestContactDisplayName` 和
  `TestSanitizeFilename`。
- 文件测试使用 `t.TempDir`，环境变量测试使用 `t.Setenv`。
- 会话测试使用固定时间戳的合成数据，禁止使用真实聊天记录。
- 修改报告时保留 `internal/report/html_test.go` 中的自包含约束：HTML 不得
  引入外部 HTTP 资源。
- 新统计指标必须覆盖乱序输入、边界时间、空样本或单边样本，以及分母定义。

## API 与兼容性

- 已发布的 JSON Tag 保持稳定；新增字段应向后兼容，不得暗中改变旧字段语义。
- 计算和输出排序必须确定，确保报告与 Golden Test 可复现。
- README 中的 `wechat-export` JSON 结构是输入契约；可选字段缺失时 Loader
  应保持容错。

## Review 清单

- 计算是否依赖消息原始顺序？如果依赖，是否先排序或验证？
- 时区、自然日与活跃日语义是否清楚？
- 比率是否防止除零？
- 新输出是否暴露聊天内容或联系人标识？
- 终端、JSON 和 HTML 是否复用同一个类型化结果？
- 第三方 API 假设是否有契约测试或可配置默认值？

禁止依赖真实时间的脆弱测试、单元测试中的网络调用、依赖 map 遍历顺序的输出，
以及为标准库已有能力引入新依赖。
