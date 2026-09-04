# 当前实现调研

## 数据流

`cmd/main.go` 通过 `walkPath` 读取单文件或目录，Loader 解码为
`loader.Conversation`，随后 Stats 同时被终端和报告消费；AI 仅消费统计摘要。

## 关键约束

- `Stats` 已带 JSON Tag，追加字段可以保持向后兼容。
- 当前 `time.Unix(...).Hour()` 和日期格式化隐式使用 `time.Local`，v0.2 先显式化
  这一既有行为，不新增时区参数。
- 当前 `walkPath` 在内部直接打印警告，JSON stdout 需要把诊断改到 stderr。
- HTML 使用 Go Template 自动转义，并以源码内嵌模板生成单文件。
- `TopMessages` 保存原文且模板默认渲染，是分享报告时的隐私风险。

## 设计选择

- Session gap 默认 30 分钟但可配置，兼顾开箱即用和可解释性。
- 响应按 Turn 切换计算，而不是每条消息，避免连续发送消息放大样本量。
- 不引入综合亲密度分数；只提供可解释的原子指标。
- 不新增统计依赖；排序、分位数和日期解析使用标准库。
