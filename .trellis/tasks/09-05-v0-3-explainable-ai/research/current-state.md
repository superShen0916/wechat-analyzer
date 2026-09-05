# v0.3 方向调研

## 核心问题

当前项目已有确定性关系指标和多供应商 AI 入口，但 AI 层的“结论深度”与
“证据深度”不匹配：它不读消息内容，仍然生成人格、语言风格和话题判断。

## 现有数据流

```text
loader.Conversation
    ├── stats.AnalyzeConversationWithOptions
    └── ai.buildPrompt（仅聚合统计）
            └── go-openai CreateChatCompletion
                    └── parseResponse（行前缀解析）
                            ├── CLI
                            ├── JSON
                            └── HTML
```

## 可利用的基础

- Loader 已有消息时间、发送方和文本字段。
- v0.2 已实现稳定排序、Session/Turn 和本地时区契约。
- CLI 已有纯 JSON stdout、stderr 诊断和 `--include-content` 的展示隐私边界。
- HTML 已使用 `html/template` 和单文件无外链约束。
- 六个供应商都通过 OpenAI-compatible 聊天接口编排，MVP 可避免供应商专属分支。

## 主要缺口

1. 证据缺口：“话题”或“说话风格”判断无法回溯。
2. 隐私缺口：若直接加正文，会跳过已写入 spec 的预览和授权要求。
3. 契约缺口：行前缀解析不能区分完整结果和部分结果。
4. 测试缺口：客户端创建与业务逻辑耦合，无法无网络断言请求载荷。
5. 规模缺口：没有抽样和 Prompt 预算，不能安全处理长聊天记录。

## 候选路线

| 方向 | 用户价值 | 工程深度 | 主要代价 |
|---|---|---|---|
| 可解释 AI + 隐私预览 | 修正当前 AI 结论无证据问题 | 抽样、脱敏、结构校验、fake transport、证据 UI | 显式发送部分正文，需严格隐私边界 |
| 本地话题/词频趋势 | 完全离线 | 中文分词、停用词、趋势图 | 语义较浅，数据文件有维护成本 |
| 发布工程/性能基准 | 安装与大文件体验更稳 | benchmark、profiling、release automation | 不增加报告本身的分析深度 |

## 建议

优先可解释 AI，但保留默认 aggregate-only 路径。“显式 `--evidence` + 无网络
`--preview` + best-effort 脱敏 + 本地引用校验”可以把新的隐私风险变成可见、可测试的契约。
