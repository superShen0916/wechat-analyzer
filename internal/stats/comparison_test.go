package stats

import (
	"testing"
	"time"

	"github.com/superShen0916/wechat-analyzer/internal/loader"
)

func TestParsePeriod(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	tests := map[string][2]string{
		"2026":                   {"2026-01-01", "2026-12-31"},
		"2026-02":                {"2026-02-01", "2026-02-28"},
		"2024-02":                {"2024-02-01", "2024-02-29"},
		"2026-01-02..2026-03-04": {"2026-01-02", "2026-03-04"},
	}
	for spec, want := range tests {
		t.Run(spec, func(t *testing.T) {
			got, err := ParsePeriod(spec, loc)
			if err != nil {
				t.Fatal(err)
			}
			if got.Start.Format("2006-01-02") != want[0] || got.End.Format("2006-01-02") != want[1] {
				t.Fatalf("range = %s..%s", got.Start, got.End)
			}
		})
	}
	for _, spec := range []string{"", "2026-13", "2026-02-03..2026-02-02", "02/2026"} {
		if _, err := ParsePeriod(spec, loc); err == nil {
			t.Fatalf("ParsePeriod(%q) error = nil", spec)
		}
	}
}

func TestComparePeriodsAndMissingResponseSamples(t *testing.T) {
	loc := time.UTC
	conversation := &loader.Conversation{Messages: []loader.Message{
		{IsSender: true, CreateTime: time.Date(2025, 1, 1, 10, 0, 0, 0, loc).Unix()},
		{IsSender: false, CreateTime: time.Date(2025, 1, 1, 10, 5, 0, 0, loc).Unix()},
		{IsSender: false, CreateTime: time.Date(2026, 1, 1, 10, 0, 0, 0, loc).Unix()},
		{IsSender: true, CreateTime: time.Date(2026, 1, 1, 10, 10, 0, 0, loc).Unix()},
		{IsSender: false, CreateTime: time.Date(2026, 1, 2, 10, 0, 0, 0, loc).Unix()},
	}}
	first, _ := ParsePeriod("2025", loc)
	second, _ := ParsePeriod("2026", loc)
	got, err := ComparePeriods(conversation, first, second, AnalyzeOptions{Location: loc})
	if err != nil {
		t.Fatal(err)
	}
	if got.Delta.Messages.Absolute != 1 || got.Delta.Messages.PercentChange == nil || *got.Delta.Messages.PercentChange != 50 {
		t.Fatalf("message delta = %#v", got.Delta.Messages)
	}
	if got.First.MyResponseMedianSeconds != nil || got.Second.MyResponseMedianSeconds == nil || got.Delta.MyResponseMedianSeconds != nil {
		t.Fatalf("missing response samples were not preserved: first=%v second=%v delta=%v", got.First.MyResponseMedianSeconds, got.Second.MyResponseMedianSeconds, got.Delta.MyResponseMedianSeconds)
	}
}

func TestComparePeriodsResponseZeroBaseline(t *testing.T) {
	loc := time.UTC
	conversation := &loader.Conversation{Messages: []loader.Message{
		{IsSender: false, CreateTime: time.Date(2025, 1, 1, 10, 0, 0, 0, loc).Unix()},
		{IsSender: true, CreateTime: time.Date(2025, 1, 1, 10, 0, 0, 0, loc).Unix()},
		{IsSender: false, CreateTime: time.Date(2026, 1, 1, 10, 0, 0, 0, loc).Unix()},
		{IsSender: true, CreateTime: time.Date(2026, 1, 1, 10, 10, 0, 0, loc).Unix()},
	}}
	first, _ := ParsePeriod("2025", loc)
	second, _ := ParsePeriod("2026", loc)
	got, err := ComparePeriods(conversation, first, second, AnalyzeOptions{Location: loc})
	if err != nil {
		t.Fatal(err)
	}
	if got.First.MyResponseMedianSeconds == nil || *got.First.MyResponseMedianSeconds != 0 {
		t.Fatalf("first median = %v, want zero-valued sample", got.First.MyResponseMedianSeconds)
	}
	if got.Delta.MyResponseMedianSeconds == nil || got.Delta.MyResponseMedianSeconds.Absolute != 600 || got.Delta.MyResponseMedianSeconds.PercentChange != nil {
		t.Fatalf("zero baseline delta = %#v", got.Delta.MyResponseMedianSeconds)
	}
}

func TestComparePeriodsRejectsEmptyPeriod(t *testing.T) {
	conversation := &loader.Conversation{Messages: []loader.Message{{CreateTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()}}}
	first, _ := ParsePeriod("2025", time.UTC)
	second, _ := ParsePeriod("2026", time.UTC)
	if _, err := ComparePeriods(conversation, first, second, AnalyzeOptions{Location: time.UTC}); err == nil {
		t.Fatal("ComparePeriods() error = nil")
	}
}
