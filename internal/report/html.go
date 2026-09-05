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
	Talker                 string              `json:"talker"`
	Stats                  *stats.Stats        `json:"stats"`
	AIResult               *ai.AnalysisResult  `json:"ai_result"`
	Claims                 []ClaimView         `json:"claims,omitempty"`
	EvidenceSampling       *ai.SamplingSummary `json:"evidence_sampling,omitempty"`
	EvidenceMeta           *EvidenceReportMeta `json:"evidence_meta,omitempty"`
	StartDate              string              `json:"start_date"`
	EndDate                string              `json:"end_date"`
	HourBars               []HourBar           `json:"hour_bars"`
	MonthlyBars            []MonthlyBar        `json:"monthly_bars"`
	IncludeContent         bool                `json:"include_content"`
	IncludeEvidenceContent bool                `json:"include_evidence_content"`
	ReceivedRatio          float64             `json:"received_ratio"`
	ExportedAt             string              `json:"exported_at"`
	Today                  string              `json:"today"`
}

type ReportOptions struct {
	IncludeContent         bool
	IncludeEvidenceContent bool
}

// ClaimView connects a model claim to locally prepared evidence messages.
type ClaimView struct {
	Category   string
	Text       string
	Confidence string
	Evidence   []ai.EvidenceMessage
}

type EvidenceReportMeta struct {
	PromptVersion string
	Provider      string
	Redactions    []RedactionView
}

type RedactionView struct {
	Kind  string
	Count int
}

// HourBar contains presentation-ready values for one hour in the activity chart.
type HourBar struct {
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Height int    `json:"height"`
	Y      int    `json:"y"`
}

type MonthlyBar struct {
	Month string `json:"month"`
	Total int    `json:"total"`
	Width int    `json:"width"`
}

//go:embed template.html
var reportTemplate string

// GenerateHTMLReport 生成单人对话的 HTML 报告
func GenerateHTMLReport(outputDir string, conv *loader.Conversation, stats *stats.Stats, aiResult *ai.AnalysisResult) (string, error) {
	return GenerateHTMLReportWithOptions(outputDir, conv, stats, aiResult, ReportOptions{})
}

// GenerateHTMLReportWithOptions 只在 IncludeContent 明确开启时展示消息原文。
func GenerateHTMLReportWithOptions(outputDir string, conv *loader.Conversation, stats *stats.Stats, aiResult *ai.AnalysisResult, opts ReportOptions) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建报告目录失败: %w", err)
	}
	maxMonthlyCount := 0
	for _, month := range stats.Relationship.Monthly {
		if month.Total > maxMonthlyCount {
			maxMonthlyCount = month.Total
		}
	}
	monthlyBars := make([]MonthlyBar, 0, len(stats.Relationship.Monthly))
	for _, month := range stats.Relationship.Monthly {
		width := 0
		if maxMonthlyCount > 0 {
			width = month.Total * 100 / maxMonthlyCount
		}
		monthlyBars = append(monthlyBars, MonthlyBar{Month: month.Month, Total: month.Total, Width: width})
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
	claims, sampling, evidenceMeta := buildClaimViews(aiResult)

	// 准备数据
	startDate, endDate := getDateRange(stats)
	data := HTMLReportData{
		Talker:                 conv.Talker.DisplayName(),
		Stats:                  stats,
		AIResult:               aiResult,
		Claims:                 claims,
		EvidenceSampling:       sampling,
		EvidenceMeta:           evidenceMeta,
		StartDate:              startDate,
		EndDate:                endDate,
		HourBars:               hourBars,
		MonthlyBars:            monthlyBars,
		IncludeContent:         opts.IncludeContent,
		IncludeEvidenceContent: opts.IncludeEvidenceContent,
		ReceivedRatio:          100 - stats.SentRatio,
		ExportedAt:             time.Now().Format(time.RFC3339),
		Today:                  time.Now().Format("2006-01-02"),
	}

	funcMap := template.FuncMap{
		"toFixed": func(f float64, n int) string {
			return fmt.Sprintf(fmt.Sprintf("%%.%df", n), f)
		},
		"duration": formatDuration,
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

func buildClaimViews(result *ai.AnalysisResult) ([]ClaimView, *ai.SamplingSummary, *EvidenceReportMeta) {
	if result == nil || result.Evidence == nil {
		return nil, nil, nil
	}
	byID := make(map[string]ai.EvidenceMessage, len(result.Evidence.Messages))
	for _, message := range result.Evidence.Messages {
		byID[message.ID] = message
	}
	views := make([]ClaimView, 0, len(result.Claims))
	for _, claim := range result.Claims {
		view := ClaimView{Category: claim.Category, Text: claim.Text, Confidence: claim.Confidence}
		for _, id := range claim.EvidenceIDs {
			if message, ok := byID[id]; ok {
				view.Evidence = append(view.Evidence, message)
			}
		}
		views = append(views, view)
	}
	sampling := result.Evidence.Sampling
	redactionKeys := make([]string, 0, len(result.Evidence.Redactions))
	for key := range result.Evidence.Redactions {
		redactionKeys = append(redactionKeys, key)
	}
	sort.Strings(redactionKeys)
	meta := &EvidenceReportMeta{PromptVersion: result.PromptVersion, Provider: result.Provider}
	for _, key := range redactionKeys {
		meta.Redactions = append(meta.Redactions, RedactionView{Kind: key, Count: result.Evidence.Redactions[key]})
	}
	return views, &sampling, meta
}

func formatDuration(value any) string {
	var seconds float64
	switch typed := value.(type) {
	case int64:
		seconds = float64(typed)
	case float64:
		seconds = typed
	default:
		return "0 秒"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0f 秒", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.1f 分钟", seconds/60)
	}
	return fmt.Sprintf("%.1f 小时", seconds/3600)
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
