package stats

import (
	"fmt"
	"regexp"
	"time"

	"github.com/superShen0916/wechat-analyzer/internal/loader"
)

var (
	yearPattern  = regexp.MustCompile(`^\d{4}$`)
	monthPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)
	rangePattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\.\.(\d{4}-\d{2}-\d{2})$`)
)

// Period 表示起止日期均包含的本地日期区间。
type Period struct {
	Label string    `json:"label"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type PeriodSummary struct {
	Period                     string   `json:"period"`
	Messages                   int      `json:"messages"`
	Sessions                   int      `json:"sessions"`
	StartedByMeRatio           float64  `json:"started_by_me_ratio"`
	StartedByThemRatio         float64  `json:"started_by_them_ratio"`
	MyResponseMedianSeconds    *float64 `json:"my_response_median_seconds"`
	TheirResponseMedianSeconds *float64 `json:"their_response_median_seconds"`
	ActiveDays                 int      `json:"active_days"`
}

type DeltaValue struct {
	Absolute      float64  `json:"absolute"`
	PercentChange *float64 `json:"percent_change"`
}

type ComparisonDelta struct {
	Messages                   DeltaValue  `json:"messages"`
	Sessions                   DeltaValue  `json:"sessions"`
	StartedByMeRatio           DeltaValue  `json:"started_by_me_ratio"`
	StartedByThemRatio         DeltaValue  `json:"started_by_them_ratio"`
	MyResponseMedianSeconds    *DeltaValue `json:"my_response_median_seconds"`
	TheirResponseMedianSeconds *DeltaValue `json:"their_response_median_seconds"`
	ActiveDays                 DeltaValue  `json:"active_days"`
}

type Comparison struct {
	First  PeriodSummary   `json:"first"`
	Second PeriodSummary   `json:"second"`
	Delta  ComparisonDelta `json:"delta"`
}

func ParsePeriod(spec string, loc *time.Location) (Period, error) {
	if loc == nil {
		loc = time.Local
	}
	if yearPattern.MatchString(spec) {
		start, err := time.ParseInLocation("2006", spec, loc)
		if err != nil {
			return Period{}, fmt.Errorf("无效时期 %q: %w", spec, err)
		}
		return Period{Label: spec, Start: start, End: start.AddDate(1, 0, -1)}, nil
	}
	if monthPattern.MatchString(spec) {
		start, err := time.ParseInLocation("2006-01", spec, loc)
		if err != nil {
			return Period{}, fmt.Errorf("无效时期 %q: %w", spec, err)
		}
		return Period{Label: spec, Start: start, End: start.AddDate(0, 1, -1)}, nil
	}
	match := rangePattern.FindStringSubmatch(spec)
	if len(match) == 3 {
		start, err := time.ParseInLocation("2006-01-02", match[1], loc)
		if err != nil {
			return Period{}, fmt.Errorf("无效时期起始日期 %q: %w", match[1], err)
		}
		end, err := time.ParseInLocation("2006-01-02", match[2], loc)
		if err != nil {
			return Period{}, fmt.Errorf("无效时期结束日期 %q: %w", match[2], err)
		}
		if end.Before(start) {
			return Period{}, fmt.Errorf("时期起始日期不能晚于结束日期")
		}
		return Period{Label: spec, Start: start, End: end}, nil
	}
	return Period{}, fmt.Errorf("无效时期 %q，请使用 YYYY、YYYY-MM 或 YYYY-MM-DD..YYYY-MM-DD", spec)
}

func ComparePeriods(conv *loader.Conversation, first, second Period, opts AnalyzeOptions) (*Comparison, error) {
	if conv == nil {
		return nil, fmt.Errorf("没有对话可对比")
	}
	firstStats, err := analyzePeriod(conv, first, opts)
	if err != nil {
		return nil, err
	}
	secondStats, err := analyzePeriod(conv, second, opts)
	if err != nil {
		return nil, err
	}
	a, b := summary(first, firstStats), summary(second, secondStats)
	return &Comparison{
		First:  a,
		Second: b,
		Delta: ComparisonDelta{
			Messages:                   delta(float64(a.Messages), float64(b.Messages)),
			Sessions:                   delta(float64(a.Sessions), float64(b.Sessions)),
			StartedByMeRatio:           delta(a.StartedByMeRatio, b.StartedByMeRatio),
			StartedByThemRatio:         delta(a.StartedByThemRatio, b.StartedByThemRatio),
			MyResponseMedianSeconds:    optionalDelta(a.MyResponseMedianSeconds, b.MyResponseMedianSeconds),
			TheirResponseMedianSeconds: optionalDelta(a.TheirResponseMedianSeconds, b.TheirResponseMedianSeconds),
			ActiveDays:                 delta(float64(a.ActiveDays), float64(b.ActiveDays)),
		},
	}, nil
}

func analyzePeriod(conv *loader.Conversation, period Period, opts AnalyzeOptions) (*Stats, error) {
	filtered := &loader.Conversation{Talker: conv.Talker, ExportedAt: conv.ExportedAt, SourceFile: conv.SourceFile}
	endExclusive := period.End.AddDate(0, 0, 1)
	for _, message := range conv.Messages {
		createdAt := time.Unix(message.CreateTime, 0).In(period.Start.Location())
		if !createdAt.Before(period.Start) && createdAt.Before(endExclusive) {
			filtered.Messages = append(filtered.Messages, message)
		}
	}
	if len(filtered.Messages) == 0 {
		return nil, fmt.Errorf("时期 %s 内没有消息", period.Label)
	}
	filtered.Total = len(filtered.Messages)
	return AnalyzeConversationWithOptions(filtered, opts)
}

func summary(period Period, value *Stats) PeriodSummary {
	return PeriodSummary{
		Period:                     period.Label,
		Messages:                   value.Total,
		Sessions:                   value.Relationship.TotalSessions,
		StartedByMeRatio:           value.Relationship.StartedByMeRatio,
		StartedByThemRatio:         value.Relationship.StartedByThemRatio,
		MyResponseMedianSeconds:    responseMedian(value.Relationship.MyResponses),
		TheirResponseMedianSeconds: responseMedian(value.Relationship.TheirResponses),
		ActiveDays:                 value.ActiveDayCount,
	}
}

func responseMedian(response ResponseStats) *float64 {
	if response.Count == 0 {
		return nil
	}
	value := response.MedianSeconds
	return &value
}

func optionalDelta(before, after *float64) *DeltaValue {
	if before == nil || after == nil {
		return nil
	}
	value := delta(*before, *after)
	return &value
}

func delta(before, after float64) DeltaValue {
	result := DeltaValue{Absolute: after - before}
	if before != 0 {
		percent := (after - before) / before * 100
		result.PercentChange = &percent
	}
	return result
}
