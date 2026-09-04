# 关系动态与时期对比

## 1. Scope / Trigger

修改 `internal/stats` 中的时间线、Session、Turn、响应时间、日期统计，或修改
`stats` / `analyze` / `compare` 的文本、JSON、HTML 输出时，必须遵守本规范。
目标是让同一份派生指标在所有展示层语义一致，并避免把“无样本”误表示为数值 0。

## 2. Signatures

```go
func AnalyzeConversationWithOptions(*loader.Conversation, AnalyzeOptions) (*Stats, error)
func ParsePeriod(string, *time.Location) (Period, error)
func ComparePeriods(*loader.Conversation, Period, Period, AnalyzeOptions) (*Comparison, error)
```

```text
wechat-analyzer stats <file-or-dir> [--session-gap 30m] [--format text|json]
wechat-analyzer analyze <file-or-dir> [--session-gap 30m] [--format text|json]
wechat-analyzer compare <file> --period <period> --period <period> [--format text|json]
```

## 3. Contracts

- 分析前复制并稳定排序消息；不修改 Loader 的原切片，同时间戳保持输入顺序。
- 相邻消息间隔 `> SessionGap` 才切新 Session；默认 30 分钟，恰好 30 分钟不切分。
- 说话方连续的消息归入同一 Turn；只有说话方切换时产生响应样本。
- `msgs_per_day` 继续表示活跃日日均；自然日日均使用 `msgs_per_calendar_day`。
- Period 支持 `YYYY`、`YYYY-MM`、`YYYY-MM-DD..YYYY-MM-DD`，起止日均包含。
- compare 的响应中位数为可空 JSON 字段；无样本为 `null`，真实的 0 秒响应为 `0`。
- 只有两个时期都有对应响应样本时才生成该响应变化；否则 Delta 为 `null`。
- 消息正文默认不进入 JSON/HTML，只能由 `--include-content` 显式开启。

## 4. Validation & Error Matrix

| 条件 | 行为 |
|---|---|
| 会话为 nil 或无消息 | 返回“没有消息可分析” |
| `--session-gap <= 0` | CLI 返回错误 |
| `--format` 非 `text/json` | CLI 返回错误 |
| compare 不是恰好两个 period | CLI 返回错误 |
| Period 格式非法或起始日晚于结束日 | 解析返回错误 |
| 任一 compare 时期无消息 | 返回错误，不生成对比 |
| 响应样本缺失 | 文本显示“暂无样本”，JSON 值与 Delta 为 `null` |
| 变化基线为数值 0 | 保留绝对变化，百分比变化为 `null` |

## 5. Good / Base / Bad Cases

- Good：乱序、跨日、跨月消息经稳定排序后得到确定性月度与会话结果。
- Base：单条消息会话同时有开始方和结束方，但没有响应样本。
- Bad：把无响应样本的中位数输出为 `0`，并进一步计算“提速 100%”。

## 6. Tests Required

- 时间线：乱序输入结果一致，且原切片未被修改。
- Session：小于、等于、大于 gap 边界。
- Turn / 分位数：同方 burst、双方样本、偶数中位数、nearest-rank P90。
- 日期：活跃日、自然日、连续活跃、跨月 Session 归属。
- compare：三种 Period、反向范围、空时期、数值零基线、无响应样本。
- 输出：JSON 可直接解析且 stdout 无杂质；HTML 无外链且默认无正文。

## 7. Wrong vs Correct

```go
// Wrong: Count == 0 时仍把零值当成有效中位数。
median := response.MedianSeconds

// Correct: 先用 Count 判断样本是否存在，再输出可空中位数。
if response.Count == 0 {
    return nil
}
median := response.MedianSeconds
return &median
```
