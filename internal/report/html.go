// Package report 生成 HTML 报告
package report

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/superShen0916/wechat-analyzer/internal/ai"
	"github.com/superShen0916/wechat-analyzer/internal/loader"
	"github.com/superShen0916/wechat-analyzer/internal/stats"
)

// HTMLReportData HTML 报告渲染数据
type HTMLReportData struct {
	Talker        string             `json:"talker"`
	Stats         *stats.Stats       `json:"stats"`
	AIResult      *ai.AnalysisResult `json:"ai_result"`
	StartDate     string             `json:"start_date"`
	EndDate       string             `json:"end_date"`
	HourBars      []HourBar          `json:"hour_bars"`
	ReceivedRatio float64            `json:"received_ratio"`
	ExportedAt    string             `json:"exported_at"`
	Today         string             `json:"today"`
}

// HourBar contains presentation-ready values for one hour in the activity chart.
type HourBar struct {
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Height int    `json:"height"`
	Y      int    `json:"y"`
}

//go:embed template.html
var reportTemplate string

// GenerateHTMLReport 生成单人对话的 HTML 报告
func GenerateHTMLReport(outputDir string, conv *loader.Conversation, stats *stats.Stats, aiResult *ai.AnalysisResult) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	filename := sanitizeFilename(conv.Talker.DisplayName()) + "_report.html"
	outputPath := filepath.Join(outputDir, filename)

	maxHourlyCount := 0
	for _, count := range stats.MsgPerHour {
		if count > maxHourlyCount {
			maxHourlyCount = count
		}
	}
	hourBars := make([]HourBar, 24)
	for i, count := range stats.MsgPerHour {
		height := 0
		if maxHourlyCount > 0 {
			height = count * 100 / maxHourlyCount
			if count > 0 && height < 4 {
				height = 4
			}
		}
		hourBars[i] = HourBar{
			Label:  fmt.Sprintf("%02d", i),
			Count:  count,
			Height: height,
			Y:      100 - height,
		}
	}

	// 准备数据
	startDate, endDate := getDateRange(stats)
	data := HTMLReportData{
		Talker:        conv.Talker.DisplayName(),
		Stats:         stats,
		AIResult:      aiResult,
		StartDate:     startDate,
		EndDate:       endDate,
		HourBars:      hourBars,
		ReceivedRatio: 100 - stats.SentRatio,
		ExportedAt:    time.Now().Format(time.RFC3339),
		Today:         time.Now().Format("2006-01-02"),
	}

	funcMap := template.FuncMap{
		"toFixed": func(f float64, n int) string {
			return fmt.Sprintf(fmt.Sprintf("%%.%df", n), f)
		},
	}

	// 解析模板
	tpl, err := template.New("report").Funcs(funcMap).Parse(reportTemplate)
	if err != nil {
		return "", fmt.Errorf("模板解析失败: %w", err)
	}

	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("渲染报告失败: %w", err)
	}
	if err := os.WriteFile(outputPath, rendered.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("写入报告失败: %w", err)
	}

	return outputPath, nil
}

// ── 辅助函数 ──────────────────────────────────────────────────────────────────

func getDateRange(stats *stats.Stats) (string, string) {
	if len(stats.ActiveDays) == 0 {
		return "", ""
	}

	dates := make([]string, 0, len(stats.ActiveDays))
	for d := range stats.ActiveDays {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates[0], dates[len(dates)-1]
}

func sanitizeFilename(name string) string {
	if name == "" {
		name = "unknown"
	}
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	s := replacer.Replace(name)
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}
