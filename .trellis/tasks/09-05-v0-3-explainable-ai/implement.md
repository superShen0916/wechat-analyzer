# v0.3 可解释 AI 实施计划

## 1. 本地 Evidence Bundle

- [x] 在 `internal/ai` 新增 EvidenceOptions、EvidenceMessage、SamplingSummary 和 EvidenceBundle。
- [x] 实现文本过滤、稳定排序、个体 rune 截断与均匀时间线采样。
- [x] 实现双方最小覆盖、字符/消息预算和 `m0001` 稳定 ID。
- [x] 实现手机号、邮箱、身份证样式、wxid 和 URL query 脱敏与分类计数。
- [x] 覆盖乱序、同时间戳、空/非文本、双方覆盖、超长消息、边界预算和不修改原切片。

## 2. Prompt 与结构化契约

- [x] 实现版本化 evidence System Prompt 和 JSON 序列化用户数据。
- [x] 扩展 AnalysisResult，增加 Claim、Limitations、PromptVersion 和 Evidence 元数据。
- [x] 实现纯 JSON / 单层 fenced JSON 提取与严格字段校验。
- [x] 实现 category/confidence 枚举、空引用、未知 ID 和重复 ID 校验。
- [x] 用 Prompt 注入样式合成消息证明内容只被 JSON 编码为数据。

## 3. 可测试的供应商边界

- [x] 抽取最小 completionClient 接口和包内 analyzeWithClient 编排函数。
- [x] 保留现有 provider 配置、环境变量和 aggregate-only 分析路径。
- [x] 使用 fake client 断言 model、messages、max tokens、Prompt 版本和载荷无标识泄露。
- [x] 覆盖供应商错误、空 choices、非法响应和成功解析，单元测试不访问网络。

## 4. CLI 预览与 evidence 编排

- [x] 新增 `--evidence`、`--preview`、`--evidence-messages` 和 `--evidence-chars`。
- [x] 在 provider/API Key 检测前完成单文件预览分支，并校验旗标组合与边界。
- [x] 实现 text 预览和纯 JSON preview envelope。
- [x] 正式 evidence 分析复用已准备 bundle，并保持多文件错误写 stderr 行为。
- [x] 新增 CLI 测试：无 Key 预览、非法参数、JSON 纯净性、默认不发正文。

## 5. 安全输出与 HTML

- [x] 实现 analysisForOutput 深拷贝，默认移除 evidence content。
- [x] evidence + include-content 只输出脱敏证据，不输出 TopMessages 原文。
- [x] 构建 Claim / Evidence HTML View Model，显示 category、confidence、ID、局限性和样本摘要。
- [x] 更新模板并保留单文件无外链，默认不包含消息正文。
- [x] 测试 HTML 转义、默认隐藏、显式显示脱敏摘录和原敏感内容不存在。

## 6. 评测、文档与发布检查

- [x] 添加全合成 evidence fixtures 和表驱动契约评测。
- [x] 更新 README：两种 AI 模式、预览流程、抽样预算、脱敏局限、Prompt 注入风险和隐私分层。
- [x] 运行 `gofmt`、`go test ./...`、`go vet ./...` 和 golangci-lint v2.13.2。
- [x] 用合成样例执行 aggregate-only、preview text/json、evidence fake-client 契约和 HTML smoke test。
- [x] 检查 git diff，确认没有真实聊天记录、API Key、未脱敏 fixture 或无关文件。

## 风险点与回滚点

- 抽样与脱敏是最高风险的纯本地边界；第 1 阶段测试通过后再进入 Prompt 编排。
- 供应商 JSON 输出不稳定时返回错误，不为了表面成功恢复为自由文本或忽略字段。
- 报告投影不得改变 EvidenceBundle 原对象；默认隐藏测试在任何模板调整后必须保留。
