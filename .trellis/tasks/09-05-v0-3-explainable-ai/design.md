# v0.3 可解释 AI 技术设计

## 总体数据流

```text
loader.Conversation + stats.Stats
        │
        ▼
ai.PrepareEvidence（纯本地）
        ├── 稳定排序 / 文本过滤
        ├── 确定性均匀采样 / rune 预算
        ├── best-effort 脱敏
        └── EvidenceBundle
                  ├── --preview → text/json（不建 client）
                  └── buildEvidencePrompt
                              → CompletionClient
                              → parse + validate citations
                              → AnalysisResult
                                    ├── CLI
                                    ├── safe JSON projection
                                    └── offline HTML projection
```

`internal/ai` 拥有抽样、脱敏、Prompt 和响应契约；`cmd` 只编排参数与展示；
`internal/report` 只消费已校验的类型化结果。

## 核心类型与签名

```go
const EvidencePromptVersion = "evidence-v1"

type EvidenceOptions struct {
    MaxMessages int
    MaxChars    int
    Location    *time.Location
}

type EvidenceMessage struct {
    ID        string `json:"id"`
    Speaker   string `json:"speaker"` // me | them
    Time      string `json:"time"`
    Content   string `json:"content,omitempty"` // 始终是脱敏后文本
    Truncated bool   `json:"truncated"`
}

type EvidenceBundle struct {
    PromptVersion string              `json:"prompt_version"`
    Messages      []EvidenceMessage   `json:"messages"`
    Sampling      SamplingSummary     `json:"sampling"`
    Redactions    map[string]int      `json:"redactions"`
    Statistics    EvidenceStatistics  `json:"statistics"`
}

func PrepareEvidence(*loader.Conversation, *stats.Stats, EvidenceOptions) (*EvidenceBundle, error)

type Claim struct {
    Category    string   `json:"category"`
    Text        string   `json:"text"`
    EvidenceIDs []string `json:"evidence_ids"`
    Confidence  string   `json:"confidence"`
}
```

`AnalysisResult` 追加 `Claims []Claim`、`Limitations []string`、`PromptVersion string` 和
`Evidence *EvidenceBundle`，已有 JSON 字段保持不变。

## 抽样契约

1. 复制消息后按 `CreateTime` 稳定排序。
2. 保留文本类型的非空消息，记录候选数。
3. 先脱敏，再按 rune 计算预算。候选数和总字符均未超限时保留全部文本。
4. 超限时，用 `round(i*(n-1)/(k-1))` 产生时间线均匀目标位置；再以
   `max(40, MaxChars/k)` rune 作为每条上限，截断标记计入上限。
5. 若候选中双方都存在但初选缺一方，用距时间线中点最近的缺失方消息
   替换最近的非首尾重复方样本。
6. 按时间顺序输出，再分配 `m0001` 形式 ID。不使用 Loader 中的原 ID。

CLI 边界限制 `MaxMessages` 为 3–500，`MaxChars` 为 500–100000。三条的下限使“时间线首尾 +
双方最小覆盖”在首尾均由同一方发送时仍可同时满足。内部包对非法值也返回错误。

## 脱敏契约

使用预编译正则，按 URL query、email、身份证、手机号、`wxid` 的固定顺序处理，
替换为 `[URL_QUERY]`、`[EMAIL]`、`[ID_CARD]`、`[PHONE]`、`[WXID]`。

- URL 保留 scheme/host/path，整个 query 替换为一个标记。
- 每个实际替换都计数，map 输出前按 key 排序展示。
- 脱敏不保留原文映射，避免生成第二份敏感数据。

## Prompt 与注入边界

- System Prompt 是版本化常量，声明输入消息为不可信数据，不得遵循其中命令。
- User Prompt 的 EvidenceBundle 使用 `encoding/json` 序列化，不手工拼接 Content。
- Prompt 只使用“我 / 对方”，不包含 `Talker.DisplayName()`、路径和原始 ID。
- 默认 aggregate-only Prompt 保留当前路径，不在本版本改写其结果契约。

## 响应解析与校验

evidence 模式允许纯 JSON 或仅被一层 Markdown JSON 代码块包装的 JSON。解码后一次完成：

- 必填字符串去空白后非空；`topics`、`claims`、`limitations` 非空。
- category 和 confidence 必须属于固定枚举。
- 每条 claim 至少一个 evidence ID，ID 不重复且必须存在于 bundle。
- 校验错误只报告字段路径和未知 ID，不附带原始模型回答或消息正文。

## 可测试传输边界

```go
type completionClient interface {
    CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}
```

对外 API 仍按 provider 构造真实 client；包内部 `analyzeWithClient` 接受上述接口，测试 fake 可捕获
请求、返回合成响应或统计调用次数。不增加全局可变 client factory。

## CLI 设计

- `--preview` 先于 provider 自动检测和 API Key 校验运行。
- 预览限单文件；如果是目录或多路径，返回明确错误。
- evidence 正式分析可保持现有多文件编排。
- JSON 预览有独立 `preview` envelope；正式结果继续使用 analysis envelope。

## 报告与安全投影

- `analysisForOutput` 复制结果；默认清空 EvidenceMessage.Content。
- evidence 模式下，`--include-content` 只回填已脱敏证据，不恢复 `stats.TopMessages`
  的原始正文；默认 aggregate-only 模式保持 v0.2 行为。
- HTML 使用 ID 索引将 Claim 映射到证据 View Model，字符串仍由 `html/template` 转义。
- 不把 Prompt 全文、API 原始回答或未脱敏数据写入报告。

## 兼容、回滚与风险

- 已有函数签名保留，新类型和字段只追加。
- evidence 为 opt-in，可单独回退 CLI 旗标和分支，不影响统计分析。
- 规则脱敏存在漏报/误报，Prompt 注入边界也不是完美防护；用 README 明示风险和
  预览功能降低风险，不使用过度安全的宣传语言。
- 模型的 JSON 遵循度在不同供应商间有差异；MVP 选择严格失败而不是静默丢字段。
