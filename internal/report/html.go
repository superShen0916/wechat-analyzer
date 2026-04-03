// Package report 生成 HTML 报告
package report

import (
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
	Talker      string                `json:"talker"`
	Stats       *stats.Stats          `json:"stats"`
	AIResult    *ai.AnalysisResult    `json:"ai_result"`
	StartDate   string                `json:"start_date"`
	EndDate     string                `json:"end_date"`
	HourLabels  []string              `json:"hour_labels"`
	ExportedAt  string                `json:"exported_at"`
	Today       string                `json:"today"`
}

// GenerateHTMLReport 生成单人对话的 HTML 报告
func GenerateHTMLReport(outputDir string, conv *loader.Conversation, stats *stats.Stats, aiResult *ai.AnalysisResult) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	filename := sanitizeFilename(conv.Talker.DisplayName()) + "_report.html"
	outputPath := filepath.Join(outputDir, filename)

	// 准备小时标签
	hourLabels := make([]string, 24)
	for i := 0; i < 24; i++ {
		hourLabels[i] = fmt.Sprintf("%d点", i)
	}

	// 准备数据
	startDate, endDate := getDateRange(stats)
	data := HTMLReportData{
		Talker:      conv.Talker.DisplayName(),
		Stats:       stats,
		AIResult:    aiResult,
		StartDate:   startDate,
		EndDate:     endDate,
		HourLabels:  hourLabels,
		ExportedAt:  time.Now().Format(time.RFC3339),
		Today:       time.Now().Format("2006-01-02"),
	}

	// 加载模板
	tplPath := filepath.Join("internal", "report", "template.html")
	tplContent, err := os.ReadFile(tplPath)
	if err != nil {
		// 尝试直接从当前目录找
		tplPath = "template.html"
		tplContent, err = os.ReadFile(tplPath)
		if err != nil {
			return "", fmt.Errorf("读取模板文件失败: %w\n路径: %s", err, tplPath)
		}
	}

	funcMap := template.FuncMap{
		"toFixed": func(f float64, n int) string {
			return fmt.Sprintf(fmt.Sprintf("%%.%df", n), f)
		},
	}

	// 解析模板
	tpl, err := template.New("report").Funcs(funcMap).Parse(string(tplContent))
	if err != nil {
		return "", fmt.Errorf("模板解析失败: %w", err)
	}

	// 生成报告
	f, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	if err := tpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("渲染报告失败: %w", err)
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