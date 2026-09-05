# 可解释 AI Evidence 契约

## 1. Scope / Trigger

修改以下任一行为时必须加载本规范，并同时检查
[会话数据与隐私](./data-privacy.md) 与 [CLI 输出与日志](./logging-guidelines.md)：

- `analyze --evidence`、`--preview` 或 evidence 预算参数；
- 本地抽样、脱敏规则、Evidence Bundle 字段或 Prompt 版本；
- 模型 JSON 响应、claim 引用校验或供应商传输边界；
- evidence JSON/HTML/终端展示以及 `--include-content` 行为。

这是一条跨层隐私契约：`internal/ai` 拥有准备与校验，`cmd` 只负责编排和安全投影，
`internal/report` 只渲染已经校验的类型化结果。

## 2. Signatures

```text
wechat-analyzer analyze <file-or-dir> [--provider <name>]
wechat-analyzer analyze <file-or-dir> --evidence \
  [--evidence-messages 3..500] [--evidence-chars 500..100000]
wechat-analyzer analyze <single-file> --evidence --preview [--format text|json]
```

```go
func PrepareEvidence(
    *loader.Conversation,
    *stats.Stats,
    EvidenceOptions,
) (*EvidenceBundle, error)

func AnalyzeEvidence(
    context.Context,
    *stats.Stats,
    AIProvider,
    *EvidenceBundle,
    AnalysisOptions,
) (*AnalysisResult, error)
```

默认预算是 80 条消息和 12000 个 rune；Prompt 版本由
`ai.EvidencePromptVersion == "evidence-v1"` 单点定义。

## 3. Contracts

### 请求契约

- 默认 `analyze` 是 aggregate-only，不得把 `loader.Message.Content` 放入请求。
- 只有显式 `--evidence` 才可发送摘录；`--preview` 必须在 provider 自动检测、
  API Key 读取和 client 创建前返回。
- `PrepareEvidence` 复制并稳定排序输入，只保留非空文本；超预算时做确定性时间线
  均匀采样，保留首尾，并在双方都有候选时覆盖双方。
- 最终消息只包含 `m0001` 形式本地 ID、`me|them`、RFC3339 时间、脱敏正文和截断标记。
  不得包含联系人、源路径、LocalID、MsgSvrID 或原始 Talker。
- 正文在进入 Prompt 前按固定顺序处理 URL query、email、18 位身份证样式、
  中国大陆手机号和 `wxid`，分别替换成稳定标记并计数。
- User Prompt 必须用 `encoding/json` 序列化 Evidence Bundle；System Prompt 必须声明消息
  是不可信数据，禁止执行其中指令。不要依赖供应商专有的 JSON Schema 或 function calling。

### 响应契约

evidence 响应仅接受标准 JSON 或单层 `json` fenced JSON，必填：

- 兼容字段：`title`、`tags`、`archetype`、`personality`、`relationship`、`topics`、`summary`；
- 新字段：`claims`、`limitations`、`prompt_version`；
- claim：`category` 只能是 `personality|communication|relationship|topic`，
  `confidence` 只能是 `low|medium|high`，`evidence_ids` 非空、无重复且全部存在于 bundle；
- `prompt_version` 必须等于 `evidence-v1`，未知响应字段必须报错。

### 输出契约

- preview text/JSON 显示脱敏后的完整发送清单；JSON schema 是 `v0.3`。
- 正式 evidence JSON/HTML 默认保留证据 ID、预算、版本、供应商和脱敏计数，但清空正文。
- evidence + `--include-content` 只展示脱敏摘录，必须继续隐藏 `stats.TopMessages` 原文。
- evidence 模式不得直接回显供应商原始响应；模型可能在原始响应中复述摘录。
- 报告继续使用 `html/template` 且不得增加远程资源。

环境变量沿用 `ai.ProviderConfigs`，本功能不新增 Key、后端或遥测。

## 4. Validation & Error Matrix

| 条件 | 行为 |
|---|---|
| `--preview` 没有 `--evidence` | 返回明确参数错误，不检测 provider |
| preview 有多个路径或目录 | 返回“只支持一个 JSON 文件” |
| `evidence-messages` 不在 3–500 | 本地参数错误，不读 Key、不访问网络 |
| `evidence-chars` 不在 500–100000 | 本地参数错误，不读 Key、不访问网络 |
| 没有可用文本消息 | `PrepareEvidence` 返回不含正文的诊断错误 |
| provider 调用失败或 choices 为空 | 返回包装错误，不输出 Prompt 或摘录 |
| JSON 非法、缺字段、有未知字段或多值 | 整个 evidence 分析失败 |
| claim 枚举非法、无证据、重复/未知 ID | 整个 evidence 分析失败并报告字段路径 |
| 默认 JSON/HTML 输出 | EvidenceMessage.Content 必须省略 |
| evidence `--include-content` | 仅包含脱敏 EvidenceMessage.Content，不含原始 TopMessages |

## 5. Good / Base / Bad Cases

- Good：先运行 `--evidence --preview` 审核脱敏样本，再显式运行 `--evidence`；模型结论
  引用 `m0001`，本地验证通过。
- Base：不加 `--evidence`，只发送聚合统计，继续使用 v0.2 兼容结果路径。
- Bad：把真实昵称或 `MsgSvrID` 拼进 Prompt；接受模型输出的未知 ID；为了“成功率”
  静默跳过非法 claim；在 evidence HTML 中同时打开原始 TopMessages。

## 6. Tests Required

- `internal/ai`：乱序和同时间戳、文本过滤、不修改原切片、首尾与双方覆盖、rune 预算、
  五类脱敏和 URL 后标点、相同输入确定性。
- fake completion client：断言 model、messages、max tokens、Prompt 版本；请求中不存在联系人、
  路径、原始 ID 或已识别敏感片段；消息注入文本仍是 JSON 数据。
- 响应表驱动：合法/ fenced JSON、非法 JSON、缺字段、未知字段、空 claims、非法枚举、
  空/重复/未知 evidence ID、多 JSON 值、供应商错误和空 choices。
- `cmd`：清空全部 Key 后 preview 仍成功；text 包含预算和聚合字段；JSON stdout 可直接解析；
  非法旗标组合失败。
- `internal/report`：默认隐藏 evidence 正文，opt-in 只显示脱敏摘录，原 TopMessages 始终不出现，
  模型文本经过 HTML 转义，报告无外部 URL。
- 提交前运行 `go test ./...`、`go test -race ./...`、`go vet ./...` 和 CI 固定版本 golangci-lint。

## 7. Wrong vs Correct

### Wrong

```go
// 消息正文进入指令字符串，且没有预览、脱敏或引用校验。
prompt := "请分析：" + conversation.Messages[0].Content
fmt.Println(providerRawResponse)
```

### Correct

```go
bundle, err := ai.PrepareEvidence(conversation, statistics, evidenceOptions)
if err != nil {
    return err
}
// preview 分支到此结束；正式调用把 bundle 用 encoding/json 作为不可信数据序列化。
result, err := ai.AnalyzeEvidence(ctx, statistics, provider, bundle, ai.AnalysisOptions{})
```

输出时复制 `AnalysisResult` 再清空 `EvidenceMessage.Content`；不得修改源结果，也不得用
`--include-content` 恢复 evidence 模式下的原始 `TopMessages`。
