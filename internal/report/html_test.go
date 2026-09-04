package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/superShen0916/wechat-analyzer/internal/loader"
	"github.com/superShen0916/wechat-analyzer/internal/stats"
)

func TestGenerateHTMLReportCreatesSelfContainedReport(t *testing.T) {
	conversation := &loader.Conversation{Talker: loader.Contact{Remark: `A/B <Demo>`}}
	statistics := &stats.Stats{
		Total:         4,
		SentTotal:     3,
		ReceivedTotal: 1,
		SentRatio:     75,
		MsgPerHour:    make([]int, 24),
		ActiveDays:    map[string]int{"2026-01-01": 4},
		MsgTypes:      map[string]int{"text": 4},
		TopMessages:   []stats.MessageInfo{{Content: "private message", Length: 15}},
		Relationship: stats.RelationshipStats{
			SessionGapSeconds:   1800,
			TotalSessions:       2,
			LongestActiveStreak: 1,
			Monthly:             []stats.MonthlyStats{{Month: "2026-01", Total: 4, Sessions: 2, ActiveDays: 1}},
		},
	}
	statistics.MsgPerHour[9] = 4

	outputDir := t.TempDir()
	outputFile, err := GenerateHTMLReport(outputDir, conversation, statistics, nil)
	if err != nil {
		t.Fatalf("GenerateHTMLReport() error = %v", err)
	}
	if filepath.Base(outputFile) != "A_B _Demo__report.html" {
		t.Fatalf("output filename = %q", filepath.Base(outputFile))
	}

	contents, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	html := string(contents)
	for _, want := range []string{"A/B &lt;Demo&gt;", "<svg", "关系动态", "响应节奏", "2026-01", "Generated locally by wechat-analyzer"} {
		if !strings.Contains(html, want) {
			t.Fatalf("report does not contain %q", want)
		}
	}
	if strings.Contains(html, "https://") || strings.Contains(html, "http://") {
		t.Fatal("report contains an external URL; expected a self-contained document")
	}
	if strings.Contains(html, "private message") {
		t.Fatal("default report exposed message content")
	}

	withContent, err := GenerateHTMLReportWithOptions(t.TempDir(), conversation, statistics, nil, ReportOptions{IncludeContent: true})
	if err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(withContent)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "private message") {
		t.Fatal("report with IncludeContent did not contain message content")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := map[string]string{
		"":       "unknown",
		"a/b:c":  "a_b_c",
		"normal": "normal",
	}
	for input, want := range tests {
		if got := sanitizeFilename(input); got != want {
			t.Fatalf("sanitizeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}
