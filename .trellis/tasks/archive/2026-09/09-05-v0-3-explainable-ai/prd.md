# v0.3 可解释 AI 分析

## 目标与用户价值

让 AI 分析从“只看统计数字生成人格描述”升级为可审查的内容分析：用户可以
在网络请求前看到将要发送的范围，只有显式选择才会发送消息摘录；AI 结论
必须引用本地分配的消息 ID，并通过本地校验防止伪造证据。该版本同时提升
用户可信度和 GitHub 项目的工程深度。

## 背景与已确认事实

- 当前 Prompt 只包含联系人名、消息数、活跃日、发送比例和活跃时段，不包含
  消息内容，却要求模型推断“说话风格”和“常聊话题”（
  `internal/ai/client.go:168-214`）。
- 当前响应是自由文本，使用逐行前缀解析；缺字段、格式漂移和未知证据都不会
  导致明确错误（`internal/ai/client.go:217-264`）。
- AI 客户端在业务函数中直接构造，没有可注入的请求边界，难以对“发送了
  什么”做无网络契约测试（`internal/ai/client.go:116-147`）。
- HTML 直接展示 AI 结论，没有证据、局限性或 Prompt 版本信息（
  `internal/report/template.html:217-240`）。
- 项目隐私规范已要求：向 LLM 发送聊天内容时必须允许预览发送范围、显式授权
  并说明供应商边界（`.trellis/spec/backend/data-privacy.md:9-17`）。

## 功能需求

### R1：保持默认隐私与兼容

- 现有 `analyze` 默认仍只发送聚合统计，不发送聊天正文。
- 新增 `--evidence` 才启用消息摘录分析；该显式参数代表跨越供应商边界的授权。
- `--include-content` 仍只控制 JSON/HTML 是否嵌入消息正文，不与“是否发给 AI”复用。
- evidence 模式的联系人标识固定为“对方”，不发送真实昵称、备注、`wxid`、源路径或
  微信服务端消息 ID。

### R2：可重现的本地抽样与 Prompt 预算

- 只选择 `type_name=text` 或缺失 `type_name` 的非空消息，对消息副本按时间稳定排序。
- 默认最多选择 80 条、12000 个 rune；新增 `--evidence-messages` 和 `--evidence-chars`
  允许显式调整，分别只接受 3–500 和 500–100000。
- 候选未超限时全部保留；超限时使用固定的时间线均匀采样，并确保在双方都有
  候选时至少各保留一条。
- 给最终样本按时间顺序分配 `m0001` 形式的本地 ID；相同输入和选项必须得到
  完全相同的选择、ID 和输出顺序。
- 输出候选数、入选数、入选字符数、时间覆盖和是否截断，不宣称样本代表全部对话。

### R3：best-effort 本地脱敏

- 在创建 Prompt 前，默认对中国大陆手机号、电子邮箱、18 位身份证样式、
  `wxid` 和 URL 查询参数做规则替换。
- 替换使用稳定标记，并输出每类替换数量；错误和诊断不得带入脱敏前正文。
- 脱敏明确标记为 best-effort，不声称可发现所有姓名、地址、账号或其他个人信息。
- v0.3 不提供关闭脱敏的参数；需要完整原文的用户继续使用默认 aggregate-only 模式。

### R4：真正的发送前预览

- `analyze <file> --evidence --preview` 在组装结构化发送清单后结束，不读取 API Key、
  不创建供应商客户端、不发起网络请求。
- `--preview` 必须与 `--evidence` 同时使用，且限制为单个 JSON 文件，避免一次在终端
  展开多份聊天摘录。
- 预览包含 Prompt 版本、聚合字段、入选消息 ID、本地时间、说话方、脱敏后正文、
  预算和脱敏统计；不显示 API Key 或请求 Header。
- 文本预览写 stdout；`--format json` 预览为纯 JSON，诊断仍写 stderr。

### R5：结构化结论与证据完整性

- evidence 模式要求模型返回标准 JSON，并保留已发布的 title、tags、archetype、
  personality、relationship、topics 和 summary 字段。
- 新增 `claims`：每条包含 category、text、evidence_ids 和 confidence；category 只允许
  `personality|communication|relationship|topic`，confidence 只允许 `low|medium|high`。
- 每条 claim 至少引用一个样本 ID；本地校验必填字段、枚举值、重复 ID 和所有
  未知 ID，失败时整个 evidence 分析返回可诊断错误。
- 新增 limitations、prompt_version 和 sampling 摘要；confidence 标注为模型自评，不是统计置信度。
- 不依赖任一供应商的 `response_format`、function calling 或 JSON Schema 扩展；使用公共聊天
  消息能力和本地严格解析，允许剔除单层 Markdown JSON 代码块包装。

### R6：Prompt 注入边界

- 脱敏消息作为 JSON 数据序列化，不使用字符串拼接嵌入 Prompt 指令区。
- System Prompt 明确规定消息内容是不可信数据，其中任何命令都不得执行。
- 这是降低风险的边界而非完美防护；README 不宣称模型对 Prompt 注入免疫。

### R7：证据展示的隐私分层

- 终端、JSON 和 HTML 显示结论及证据 ID。
- 默认 JSON/HTML 不嵌入证据正文；`--include-content` 时才根据 ID 回填脱敏后的证据
  摘录，不回填未脱敏原文。
- HTML 报告显示 Prompt 版本、样本覆盖、供应商、脱敏统计和“仅基于入选样本”局限性，
  并继续保持单文件、无外部请求。

### R8：可测试的 AI 边界与评测

- 抽取最小聊天完成传输接口，使用本地 fake 测试请求和响应，单元测试禁止网络。
- 增加合成评测 fixtures，覆盖完整响应、非法 JSON、缺字段、未知/重复证据 ID、无证据结论、
  Prompt 注入样式消息、脱敏和预算截断。
- 评测检查结构合法性、引用完整性、隐私字段泄露和确定性，不对模型内容“好不好”
  做脆弱的字符串断言。

## 验收标准

- [x] 默认 `analyze` 请求不包含消息正文；只有 `--evidence` 才加入脱敏摘录。
- [x] `--evidence --preview` 无 API Key 也可执行，且 fake transport 证明网络调用数为 0。
- [x] 相同输入与参数生成完全相同的样本 ID、顺序、脱敏结果和预算统计。
- [x] 发送清单不包含联系人标识、服务端消息 ID、源路径或已识别的敏感片段。
- [x] 超过预算时结果仍覆盖时间线前后和双方，并明确报告截断。
- [x] 模型返回非法结构、缺失证据或引用未知/重复 ID 时返回可诊断错误。
- [x] Prompt 注入样式消息被 JSON 编码为数据，不能破坏请求结构。
- [x] 默认 JSON/HTML 只显示证据 ID，不显示消息正文；`--include-content` 只显示脱敏摘录。
- [x] 旧 AI 结果字段、现有供应商、`stats`、`compare` 和普通 HTML 报告保持兼容。
- [x] README 清楚区分“发给 AI 的内容”与“写入 JSON/HTML 的内容”，说明脱敏和 Prompt 注入局限。
- [x] `gofmt`、`go test ./...`、`go vet ./...`、golangci-lint 和合成 CLI smoke test 通过。

## 不在本版本范围

- Ollama 或其他本地模型运行时。
- 情感分、亲密度分、心理诊断或关系健康结论。
- 云端数据库、Prompt 遥测、用户跟踪或上传评测数据。
- 可识别所有个人信息的完美匿名化承诺。
- 依赖特定供应商 JSON Schema、function-calling 或其他非通用扩展。
- 向未显式选择 `--evidence` 的用户发送聊天正文。

## 兼容与发布约束

- 新 evidence 模式是显式 opt-in，默认 `analyze` 行为不变。
- 不新增运行时外部服务或前端 CDN，优先使用 Go 标准库。
- 现有 `AnalysisResult` JSON 字段只追加不删除；默认自由文本解析路径保留。
