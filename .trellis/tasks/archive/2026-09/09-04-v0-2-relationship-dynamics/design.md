# v0.2 技术设计

## 总体结构

```text
loader.Conversation
        │（复制并稳定排序）
        ▼
stats.AnalyzeConversationWithOptions
        ├── 基础统计
        ├── Sessionizer
        │     └── Turn / Response 样本
        └── 月度与连续活跃统计
              │
              ├── CLI text/json
              ├── report HTML
              └── ComparePeriods
```

确定性计算全部位于 `internal/stats`。`cmd` 只负责参数解析、路径编排和展示；
`internal/report` 只把类型化结果转换为报告 View Model。

## 核心类型

在 `internal/stats` 中新增：

```go
const DefaultSessionGap = 30 * time.Minute

type AnalyzeOptions struct {
    SessionGap time.Duration
    Location   *time.Location
}

type RelationshipStats struct {
    SessionGapSeconds      int64
    TotalSessions          int
    AvgMessagesPerSession  float64
    LongestSessionMessages int
    LongestSessionSeconds  int64
    StartedByMe            int
    StartedByThem          int
    EndedByMe              int
    EndedByThem            int
    MyResponses            ResponseStats
    TheirResponses         ResponseStats
    LongestActiveStreak    int
    Monthly                []MonthlyStats
}

type ResponseStats struct {
    Count         int
    MedianSeconds float64
    P90Seconds    float64
}
```

`Stats` 新增 `CalendarDays`、`ActiveDayCount`、`MsgPerCalendarDay` 和
`Relationship`，同时保留 `MsgPerDay` 作为兼容字段。

## 会话与 Turn 算法

1. 复制 `conv.Messages`，使用 `sort.SliceStable` 按 `CreateTime` 排序。
2. 第一条消息创建 Session。
3. 若当前消息与上一消息差值 `> SessionGap`，关闭旧 Session 并开启新 Session。
4. Session 内相邻相同 `IsSender` 的消息合并为一个 Turn。
5. 说话方切换时，响应值为新 Turn 第一条时间减旧 Turn 最后一条时间。
6. Session 结束时累计开始方、结束方、消息数和持续时间。

分位数先排序样本。中位数采用常见的中点平均；P90 使用 nearest-rank
`ceil(0.9*n)-1`，并在测试与 README 中固定该定义。无样本时 Count 为 0，时间为
0，展示层输出“暂无样本”。
时期对比中的响应中位数使用可空值区分“无样本”和真实的 0 秒响应；
任一时期无响应样本时，对应变化值也为 `null`，不生成误导性差值。

## 日期与月度统计

- 默认 `Location` 为 `time.Local`。
- `CalendarDays` 为最早和最晚本地日期的闭区间天数。
- `MsgPerDay` 保持 `Total / ActiveDayCount`。
- `MsgPerCalendarDay` 为 `Total / CalendarDays`。
- 连续活跃按排序后的唯一日期逐日比较，不受一天内消息顺序影响。
- Session 的月份归属由第一条消息时间决定。

## 时期解析与对比

Period 解析放在 `internal/stats`，使 CLI 和测试共享一致契约：

```go
type Period struct { Label string; Start, End time.Time }
func ParsePeriod(spec string, loc *time.Location) (Period, error)
func ComparePeriods(conv *loader.Conversation, first, second Period, opts AnalyzeOptions) (*Comparison, error)
```

`YYYY` 和 `YYYY-MM` 展开为完整闭区间；日期范围的 End 转换为次日零点前的闭区间
语义。过滤使用 `[Start, EndExclusive)`，避免纳秒边界问题。Comparison 保存两个
时期的摘要和绝对 Delta；百分比仅在第一时期非零时通过可空字段输出。

## CLI 设计

- `stats`、`analyze` 新增 `--session-gap 30m`、`--format text|json` 和
  `--include-content`。
- `compare` 需要一个文件和恰好两个 `--period`，支持相同 gap 与 format。
- JSON 输出使用稳定 Envelope，包含 schema version、联系人和结果。
- JSON 模式先收集结果再由 `json.Encoder` 一次写入 stdout；警告改写 stderr。
- `--format json` 与 `--html` 可并用，生成报告路径写入 JSON Envelope，不额外
  打印成功提示。

## HTML 与隐私

`GenerateHTMLReport` 改为接收 `ReportOptions{IncludeContent bool}`。为兼容已有调用，
保留原函数作为默认安全包装，并新增带 Options 的函数。

关系动态使用纯 HTML/CSS/SVG，不引入 JavaScript/CDN。月度趋势根据统计切片生成
归一化柱形 View Model。最长消息区域仅在 `IncludeContent` 为 true 时渲染。

## 兼容性与回滚

- 原 `AnalyzeConversation` 调用新 Options API 的默认配置，已有调用无需修改。
- 原 `GenerateHTMLReport` 保留签名，默认不包含正文；需要正文的 CLI 调用 Options
  版本。
- 新字段只追加，不删除旧 JSON 字段。
- 如 compare CLI 影响发布，可单独回退命令注册，不影响统计结构与普通报告。

## 风险

- “响应时间”容易被误读为关系质量：展示层必须说明它只描述聊天节奏。
- 本地时区改变会改变跨日/月结果：本版明确记录时区，不隐式宣称全局一致。
- 大量月度数据增加 HTML 长度，但只输出聚合值，增长与月数成正比而非消息数。
