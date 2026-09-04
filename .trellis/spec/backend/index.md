# Go CLI 开发规范

这里记录 `wechat-analyzer` 当前真实的代码约定。修改代码前，根据任务范围加载
对应规范。

| 规范 | 使用场景 |
|---|---|
| [Go 项目结构](./directory-structure.md) | 新增包、命令、分析器或报告组件 |
| [错误处理](./error-handling.md) | 新增校验、批处理、文件或供应商错误 |
| [CLI 输出与日志](./logging-guidelines.md) | 修改终端或机器可读输出 |
| [质量规范](./quality-guidelines.md) | 实现或审查任何行为变更 |
| [会话数据与隐私](./data-privacy.md) | 读取、派生、展示或传输聊天数据 |
| [关系动态与时期对比](./relationship-analysis.md) | 修改 Session、Turn、响应时间、日期或 compare 输出 |

项目专属文档、Trellis 任务产物和开发日志默认使用中文；代码标识符、命令及无法
自然翻译的技术名词保留英文。

本项目是轻依赖的 Go CLI。必须保持清晰包边界、确定性输出、合成测试数据、
跨平台行为和 local-first 隐私模型。
