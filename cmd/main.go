package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/superShen0916/wechat-analyzer/internal/ai"
	"github.com/superShen0916/wechat-analyzer/internal/loader"
	"github.com/superShen0916/wechat-analyzer/internal/report"
	"github.com/superShen0916/wechat-analyzer/internal/stats"
)

const (
	schemaVersion         = "v0.2"
	evidenceSchemaVersion = "v0.3"
)

type statsOutput struct {
	Contact    string       `json:"contact"`
	Statistics *stats.Stats `json:"statistics"`
	ReportPath string       `json:"report_path,omitempty"`
}

type statsEnvelope struct {
	SchemaVersion string        `json:"schema_version"`
	Results       []statsOutput `json:"results"`
}

type analysisOutput struct {
	Contact    string             `json:"contact"`
	Statistics *stats.Stats       `json:"statistics"`
	Analysis   *ai.AnalysisResult `json:"analysis"`
	ReportPath string             `json:"report_path,omitempty"`
}

type analysisEnvelope struct {
	SchemaVersion string           `json:"schema_version"`
	Results       []analysisOutput `json:"results"`
}

type evidencePreviewEnvelope struct {
	SchemaVersion string             `json:"schema_version"`
	Contact       string             `json:"contact"`
	Evidence      *ai.EvidenceBundle `json:"evidence"`
}

type comparisonEnvelope struct {
	SchemaVersion string            `json:"schema_version"`
	Contact       string            `json:"contact"`
	Comparison    *stats.Comparison `json:"comparison"`
}

var rootCmd = &cobra.Command{
	Use:   "wechat-analyzer",
	Short: "微信聊天记录 AI 分析工具",
	Long: `基于 wechat-export 导出的 JSON 文件，做统计分析和 AI 人格画像。

支持的 AI 提供商：
  deepseek   - DeepSeek
  moonshot   - Kimi
  qwen       - 通义千问
  doubao     - 豆包
  zhipu      - 智谱 GLM
  anthropic  - Claude
`,
}

// 颜色配置
var (
	colorTitle    = color.New(color.FgHiCyan, color.Bold)
	colorStatName = color.New(color.FgHiBlue)
	colorStatVal  = color.New(color.FgHiGreen)
	colorLabel    = color.New(color.FgHiYellow)
	colorSuccess  = color.New(color.FgGreen)
	colorInfo     = color.New(color.FgCyan)
)

func colorPrintf(printer *color.Color, format string, args ...any) {
	_, _ = printer.Printf(format, args...)
}

func colorPrintln(printer *color.Color, args ...any) {
	_, _ = printer.Println(args...)
}

// 打印分隔线
func printDivider(char string, length int) {
	fmt.Println(colorLabel.Sprint(strings.Repeat(char, length)))
}

// 打印带标题的区块
func printBlock(title string, maxWidth int) {
	printDivider("═", maxWidth)
	colorPrintf(colorTitle, " %s ", title)
	fmt.Println()
	printDivider("═", maxWidth)
}

// 打印统计项
func printStat(name string, value string) {
	colorPrintf(colorStatName, "%20s:", name)
	colorPrintf(colorStatVal, " %8s", value)
	fmt.Println()
}

// ── stats 命令 ────────────────────────────────────────────────────────────────
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "统计分析聊天记录",
	Long:  "分析聊天记录的数量、时间分布、消息类型等基础统计",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("请提供要分析的 JSON 文件或目录路径")
		}
		format, opts, includeContent, err := commonAnalysisOptions(cmd)
		if err != nil {
			return err
		}
		html, _ := cmd.Flags().GetBool("html")
		outputDir, _ := cmd.Flags().GetString("output")
		var results []statsOutput
		for _, path := range args {
			if _, statErr := os.Stat(path); statErr != nil {
				warnf(cmd.ErrOrStderr(), "  ⚠️  无法读取: %s (%v)\n", path, statErr)
				continue
			}
			if err := walkPath(path, func(conv *loader.Conversation) error {
				if format == "text" {
					colorPrintf(colorInfo, "\n🔍 分析: %s (%d 条消息)\n\n", conv.Talker.DisplayName(), len(conv.Messages))
				}
				s, err := stats.AnalyzeConversationWithOptions(conv, opts)
				if err != nil {
					warnf(cmd.ErrOrStderr(), "  ⚠️  统计失败: %v\n", err)
					return nil
				}
				item := statsOutput{Contact: conv.Talker.DisplayName(), Statistics: statsForOutput(s, includeContent)}
				if format == "text" {
					printStats(s, conv)
				}
				if html {
					var htmlDir string
					if outputDir == "" {
						htmlDir = "./wechat_analyze_stats"
					} else {
						htmlDir = outputDir
					}
					reportPath, err := report.GenerateHTMLReportWithOptions(htmlDir, conv, s, nil, report.ReportOptions{IncludeContent: includeContent})
					if err != nil {
						warnf(cmd.ErrOrStderr(), "  ⚠️  生成报告失败: %v\n", err)
						return nil
					}
					item.ReportPath = reportPath
					if format == "text" {
						fmt.Printf("\n📄 HTML 报告已生成: %s\n", reportPath)
					}
				}
				results = append(results, item)
				return nil
			}); err != nil {
				return err
			}
		}
		if format == "json" {
			return writeJSON(cmd.OutOrStdout(), statsEnvelope{SchemaVersion: schemaVersion, Results: results})
		}
		return nil
	},
}

func printStats(s *stats.Stats, conv *loader.Conversation) {
	width := 60
	printBlock(fmt.Sprintf("📊 聊天记录统计 (%s)", conv.Talker.DisplayName()), width)

	printStat("总消息数", fmt.Sprintf("%d 条", s.Total))
	printStat("平均每条长度", fmt.Sprintf("%.2f 字符", s.AvgLength))
	printStat("活跃日日均", fmt.Sprintf("%.2f 条/天", s.MsgPerActiveDay))
	printStat("自然日日均", fmt.Sprintf("%.2f 条/天（%d 天）", s.MsgPerCalendarDay, s.CalendarDays))
	fmt.Println()

	printStat("我发的消息", fmt.Sprintf("%d (%.1f%%)", s.SentTotal, s.SentRatio))
	printStat("对方发的", fmt.Sprintf("%d (%.1f%%)", s.ReceivedTotal, 100-s.SentRatio))
	printStat("我先开口", fmt.Sprintf("%d (%.1f%%)", s.FirstMessageCount, s.FirstMessageRatio))
	fmt.Println()

	colorPrintln(colorLabel, "🤝 关系动态:")
	printStat("会话数", fmt.Sprintf("%d", s.Relationship.TotalSessions))
	printStat("平均每次会话", fmt.Sprintf("%.1f 条", s.Relationship.AvgMessagesPerSession))
	printStat("最长会话", fmt.Sprintf("%d 条 / %s", s.Relationship.LongestSessionMessages, formatDuration(float64(s.Relationship.LongestSessionSeconds))))
	printStat("开始方（我/对方）", fmt.Sprintf("%d / %d（%.1f%% / %.1f%%）", s.Relationship.StartedByMe, s.Relationship.StartedByThem, s.Relationship.StartedByMeRatio, s.Relationship.StartedByThemRatio))
	printStat("结束方（我/对方）", fmt.Sprintf("%d / %d（%.1f%% / %.1f%%）", s.Relationship.EndedByMe, s.Relationship.EndedByThem, s.Relationship.EndedByMeRatio, s.Relationship.EndedByThemRatio))
	printStat("最长连续活跃", fmt.Sprintf("%d 天", s.Relationship.LongestActiveStreak))
	printResponse("我的响应", s.Relationship.MyResponses)
	printResponse("对方响应", s.Relationship.TheirResponses)
	colorPrintln(colorInfo, "  注：响应时间只描述聊天节奏，不代表关系质量。")
	fmt.Println()

	colorPrintln(colorLabel, "⏰ 活跃时段分布:")
	var peakHours []int
	for hour := 0; hour < 24; hour++ {
		if s.MsgPerHour[hour] > 0 {
			peakHours = append(peakHours, hour)
		}
	}

	// 按消息数排序并取前 5 个
	sort.Slice(peakHours, func(i, j int) bool {
		left, right := peakHours[i], peakHours[j]
		if s.MsgPerHour[left] == s.MsgPerHour[right] {
			return left < right
		}
		return s.MsgPerHour[left] > s.MsgPerHour[right]
	})
	if len(peakHours) > 5 {
		peakHours = peakHours[:5]
	}

	if len(peakHours) > 0 {
		maxCount := s.MsgPerHour[peakHours[0]]
		for _, h := range peakHours {
			count := s.MsgPerHour[h]
			// 计算长度，最长 20 个方块
			barLen := count * 20 / maxCount
			bar := strings.Repeat("█", barLen)
			colorPrintf(colorStatVal, "  %02d点%02d → %d条 %s\n", h, h+1, count, bar)
		}
		fmt.Println()
	}

	colorPrintln(colorLabel, "💬 消息类型分布:")
	types := make([]string, 0, len(s.MsgTypes))
	for messageType := range s.MsgTypes {
		types = append(types, messageType)
	}
	sort.Strings(types)
	for _, messageType := range types {
		cnt := s.MsgTypes[messageType]
		p := float64(cnt) / float64(s.Total) * 100
		colorPrintf(colorStatVal, "  %-10s: %d (%.1f%%)\n", messageType, cnt, p)
	}
	fmt.Println()

	colorPrintln(colorLabel, "📅 月度趋势:")
	for _, month := range s.Relationship.Monthly {
		colorPrintf(colorStatVal, "  %s: %d 条（我 %d / 对方 %d）· %d 次会话 · %d 个活跃日\n", month.Month, month.Total, month.Sent, month.Received, month.Sessions, month.ActiveDays)
	}
	fmt.Println()
}

func printResponse(label string, response stats.ResponseStats) {
	if response.Count == 0 {
		printStat(label, "暂无样本")
		return
	}
	printStat(label, fmt.Sprintf("中位 %s / P90 %s（%d 次）", formatDuration(response.MedianSeconds), formatDuration(response.P90Seconds), response.Count))
}

func formatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.0f秒", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.1f分钟", seconds/60)
	}
	return fmt.Sprintf("%.1f小时", seconds/3600)
}

// ── analyze 命令 ──────────────────────────────────────────────────────────────
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "AI 分析聊天记录",
	Long:  "调用 AI 分析对方的人格特征、说话风格、沟通模式",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("请提供要分析的 JSON 文件路径")
		}
		format, opts, includeContent, err := commonAnalysisOptions(cmd)
		if err != nil {
			return err
		}
		evidence, _ := cmd.Flags().GetBool("evidence")
		preview, _ := cmd.Flags().GetBool("preview")
		evidenceOpts, err := evidenceOptions(cmd, opts.Location)
		if err != nil {
			return err
		}
		if preview {
			if !evidence {
				return fmt.Errorf("--preview 需要同时指定 --evidence")
			}
			if len(args) != 1 {
				return fmt.Errorf("--preview 只支持一个 JSON 文件")
			}
			return previewEvidence(cmd, args[0], format, opts, evidenceOpts)
		}

		providerStr, _ := cmd.Flags().GetString("provider")
		if providerStr == "" {
			autoProviders := ai.DetectProviders()
			if len(autoProviders) == 0 {
				return fmt.Errorf("未检测到任何已配置的 AI 提供商，请设置以下环境变量之一:\n  %s", listProviderEnvs())
			}
			providerStr = autoProviders[0].String()
			if format == "text" {
				cmd.Printf("🤖 自动使用提供商: %s\n\n", providerStr)
			}
		}

		provider := ai.AIProvider(providerStr)
		supported := false
		for _, p := range ai.SupportedProviders() {
			if p.String() == providerStr {
				supported = true
				break
			}
		}
		if !supported {
			return fmt.Errorf("不支持的提供商: %s，支持的有: %s", providerStr, listSupportedProviders())
		}

		html, _ := cmd.Flags().GetBool("html")
		outputDir, _ := cmd.Flags().GetString("output")
		ctx := context.Background()
		var results []analysisOutput

		for _, path := range args {
			if _, statErr := os.Stat(path); statErr != nil {
				warnf(cmd.ErrOrStderr(), "  ⚠️  无法读取: %s (%v)\n", path, statErr)
				continue
			}

			if err := walkPath(path, func(conv *loader.Conversation) error {
				if format == "text" {
					colorPrintf(colorInfo, "\n🤖 AI 分析 %s (%d 条消息)...\n\n", conv.Talker.DisplayName(), len(conv.Messages))
				}

				// 先做基础统计
				s, err := stats.AnalyzeConversationWithOptions(conv, opts)
				if err != nil {
					warnf(cmd.ErrOrStderr(), "  ⚠️  统计失败: %v\n", err)
					return nil
				}

				// AI 分析。默认模式只发送聚合统计；evidence 模式发送本地脱敏后的抽样摘录。
				var aiRes *ai.AnalysisResult
				if evidence {
					bundle, evidenceErr := ai.PrepareEvidence(conv, s, evidenceOpts)
					if evidenceErr != nil {
						warnf(cmd.ErrOrStderr(), "  ⚠️  准备证据失败: %v\n", evidenceErr)
						return nil
					}
					aiRes, err = ai.AnalyzeEvidence(ctx, s, provider, bundle, ai.AnalysisOptions{})
				} else {
					aiRes, err = ai.AnalyzeConversationWithOptions(ctx, conv, s, provider, ai.AnalysisOptions{EchoRaw: format == "text"})
				}
				if err != nil {
					warnf(cmd.ErrOrStderr(), "  ⚠️  AI 分析失败: %v\n", err)
					return nil
				}
				item := analysisOutput{
					Contact:    conv.Talker.DisplayName(),
					Statistics: statsForOutput(s, includeContent && !evidence),
					Analysis:   analysisForOutput(aiRes, includeContent && evidence),
				}
				if format == "text" {
					printAIResult(aiRes)
				}

				// 生成 HTML 报告
				if html {
					var htmlDir string
					if outputDir == "" {
						htmlDir = "./wechat_analyze_ai"
					} else {
						htmlDir = outputDir
					}
					reportPath, err := report.GenerateHTMLReportWithOptions(htmlDir, conv, s, aiRes, report.ReportOptions{
						IncludeContent:         includeContent && !evidence,
						IncludeEvidenceContent: includeContent && evidence,
					})
					if err != nil {
						warnf(cmd.ErrOrStderr(), "  ⚠️  生成报告失败: %v\n", err)
						return nil
					}
					item.ReportPath = reportPath
					if format == "text" {
						fmt.Printf("\n📄 HTML 报告已生成: %s\n", reportPath)
					}
				}
				results = append(results, item)
				return nil
			}); err != nil {
				return err
			}
		}
		if format == "json" {
			version := schemaVersion
			if evidence {
				version = evidenceSchemaVersion
			}
			return writeJSON(cmd.OutOrStdout(), analysisEnvelope{SchemaVersion: version, Results: results})
		}
		return nil
	},
}

func printAIResult(res *ai.AnalysisResult) {
	width := 65
	printBlock("🎭 AI 人格画像", width)

	colorPrintln(colorLabel, "人格称号:")
	fmt.Printf("  %s\n\n", res.Title)

	colorPrintln(colorLabel, "人格类型:")
	fmt.Printf("  %s\n\n", res.Archetype)

	colorPrintln(colorLabel, "人格标签:")
	for i, tag := range res.PersonalityTags {
		if i > 0 {
			fmt.Print("  ")
		}
		colorPrintf(colorSuccess, "#%s ", tag)
	}
	fmt.Println()
	fmt.Println()

	colorPrintln(colorLabel, "人格画像:")
	fmt.Printf("  %s\n\n", res.Personality)

	colorPrintln(colorLabel, "关系分析:")
	fmt.Printf("  %s\n\n", res.Relationship)

	colorPrintln(colorLabel, "常聊话题:")
	for _, topic := range res.Topics {
		colorPrintf(colorStatVal, "  • %s\n", topic)
	}
	fmt.Println()

	colorPrintln(colorLabel, "一句话总结:")
	colorPrintf(colorStatName, "  %s\n", res.Summary)
	if len(res.Claims) > 0 {
		fmt.Println()
		if res.Evidence != nil {
			colorPrintln(colorLabel, "证据契约:")
			truncated := "否"
			if res.Evidence.Sampling.Truncated {
				truncated = "是"
			}
			fmt.Printf("  Prompt %s · Provider %s · 样本 %d/%d 条、%d/%d 字符\n", res.PromptVersion, res.Provider,
				res.Evidence.Sampling.SelectedMessages, res.Evidence.Sampling.CandidateMessages,
				res.Evidence.Sampling.SelectedChars, res.Evidence.Sampling.CandidateChars)
			fmt.Printf("  时间 %s 至 %s · 截断 %s\n", res.Evidence.Sampling.StartTime, res.Evidence.Sampling.EndTime, truncated)
			keys := make([]string, 0, len(res.Evidence.Redactions))
			for key := range res.Evidence.Redactions {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Printf("  %s=%d", key, res.Evidence.Redactions[key])
			}
			fmt.Println()
			fmt.Println()
		}
		colorPrintln(colorLabel, "证据化结论:")
		for _, claim := range res.Claims {
			fmt.Printf("  • [%s / %s] %s（%s）\n", claim.Category, claim.Confidence, claim.Text, strings.Join(claim.EvidenceIDs, ", "))
		}
	}
	if len(res.Limitations) > 0 {
		fmt.Println()
		colorPrintln(colorLabel, "分析局限:")
		for _, limitation := range res.Limitations {
			fmt.Printf("  • %s\n", limitation)
		}
	}
	colorPrintln(colorLabel, strings.Repeat("═", width))
}

// ── providers 命令 ────────────────────────────────────────────────────────────
var compareCmd = &cobra.Command{
	Use:   "compare <JSON 文件>",
	Short: "对比两个时期的关系动态",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, opts, _, err := commonAnalysisOptions(cmd)
		if err != nil {
			return err
		}
		periodSpecs, _ := cmd.Flags().GetStringSlice("period")
		if len(periodSpecs) != 2 {
			return fmt.Errorf("请使用两次 --period 指定对比时期")
		}
		first, err := stats.ParsePeriod(periodSpecs[0], time.Local)
		if err != nil {
			return err
		}
		second, err := stats.ParsePeriod(periodSpecs[1], time.Local)
		if err != nil {
			return err
		}
		conversation, err := loader.LoadFile(args[0])
		if err != nil {
			return err
		}
		comparison, err := stats.ComparePeriods(conversation, first, second, opts)
		if err != nil {
			return err
		}
		if format == "json" {
			return writeJSON(cmd.OutOrStdout(), comparisonEnvelope{SchemaVersion: schemaVersion, Contact: conversation.Talker.DisplayName(), Comparison: comparison})
		}
		printComparison(conversation.Talker.DisplayName(), comparison)
		return nil
	},
}

func printComparison(contact string, comparison *stats.Comparison) {
	printBlock(fmt.Sprintf("📈 时期对比 (%s)", contact), 68)
	fmt.Printf("%-22s %14s %14s %14s\n", "指标", comparison.First.Period, comparison.Second.Period, "变化")
	printComparisonRow("消息数", float64(comparison.First.Messages), float64(comparison.Second.Messages), comparison.Delta.Messages, "%.0f")
	printComparisonRow("会话数", float64(comparison.First.Sessions), float64(comparison.Second.Sessions), comparison.Delta.Sessions, "%.0f")
	printComparisonRow("我开始会话", comparison.First.StartedByMeRatio, comparison.Second.StartedByMeRatio, comparison.Delta.StartedByMeRatio, "%.1f%%")
	printComparisonRow("对方开始会话", comparison.First.StartedByThemRatio, comparison.Second.StartedByThemRatio, comparison.Delta.StartedByThemRatio, "%.1f%%")
	printOptionalComparisonRow("我的响应中位数（秒）", comparison.First.MyResponseMedianSeconds, comparison.Second.MyResponseMedianSeconds, comparison.Delta.MyResponseMedianSeconds)
	printOptionalComparisonRow("对方响应中位数（秒）", comparison.First.TheirResponseMedianSeconds, comparison.Second.TheirResponseMedianSeconds, comparison.Delta.TheirResponseMedianSeconds)
	printComparisonRow("活跃天数", float64(comparison.First.ActiveDays), float64(comparison.Second.ActiveDays), comparison.Delta.ActiveDays, "%.0f")
	fmt.Println("\n注：响应时间只描述聊天节奏；基线为 0 时不计算百分比变化。")
}

func printComparisonRow(label string, first, second float64, change stats.DeltaValue, format string) {
	changeText := fmt.Sprintf("%+.1f", change.Absolute)
	if change.PercentChange != nil {
		changeText += fmt.Sprintf(" (%+.1f%%)", *change.PercentChange)
	}
	fmt.Printf("%-22s %14s %14s %14s\n", label, fmt.Sprintf(format, first), fmt.Sprintf(format, second), changeText)
}

func printOptionalComparisonRow(label string, first, second *float64, change *stats.DeltaValue) {
	firstText, secondText, changeText := "暂无样本", "暂无样本", "-"
	if first != nil {
		firstText = fmt.Sprintf("%.1f", *first)
	}
	if second != nil {
		secondText = fmt.Sprintf("%.1f", *second)
	}
	if change != nil {
		changeText = fmt.Sprintf("%+.1f", change.Absolute)
		if change.PercentChange != nil {
			changeText += fmt.Sprintf(" (%+.1f%%)", *change.PercentChange)
		}
	}
	fmt.Printf("%-22s %14s %14s %14s\n", label, firstText, secondText, changeText)
}

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "列出所有支持的 AI 提供商",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("📋 支持的 AI 提供商:")
		fmt.Println()

		for _, p := range ai.SupportedProviders() {
			cfg := ai.ProviderConfigs[p]
			k := os.Getenv(cfg.EnvVar)
			status := "❌"
			if k != "" {
				status = "✅"
			}
			fmt.Printf("  %s %-12s 环境变量: %s\n", status, p, cfg.EnvVar)
		}

		fmt.Println()
		fmt.Println("📝 提示：设置对应环境变量后，工具会自动检测可用提供商。")
		return nil
	},
}

// ── 辅助函数 ──────────────────────────────────────────────────────────────────

// walkPath 遍历路径，可以是单个文件或目录
func commonAnalysisOptions(cmd *cobra.Command) (string, stats.AnalyzeOptions, bool, error) {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return "", stats.AnalyzeOptions{}, false, err
	}
	if format != "text" && format != "json" {
		return "", stats.AnalyzeOptions{}, false, fmt.Errorf("不支持的输出格式 %q，请使用 text 或 json", format)
	}
	gap, err := cmd.Flags().GetDuration("session-gap")
	if err != nil {
		return "", stats.AnalyzeOptions{}, false, err
	}
	if gap <= 0 {
		return "", stats.AnalyzeOptions{}, false, fmt.Errorf("--session-gap 必须大于 0")
	}
	includeContent := false
	if cmd.Flags().Lookup("include-content") != nil {
		includeContent, err = cmd.Flags().GetBool("include-content")
		if err != nil {
			return "", stats.AnalyzeOptions{}, false, err
		}
	}
	return format, stats.AnalyzeOptions{SessionGap: gap, Location: time.Local}, includeContent, nil
}

func statsForOutput(source *stats.Stats, includeContent bool) *stats.Stats {
	copyStats := *source
	copyStats.TopMessages = append([]stats.MessageInfo(nil), source.TopMessages...)
	if !includeContent {
		for i := range copyStats.TopMessages {
			copyStats.TopMessages[i].Content = ""
		}
	}
	return &copyStats
}

func analysisForOutput(source *ai.AnalysisResult, includeEvidenceContent bool) *ai.AnalysisResult {
	if source == nil {
		return nil
	}
	result := *source
	result.PersonalityTags = append([]string(nil), source.PersonalityTags...)
	result.Topics = append([]string(nil), source.Topics...)
	result.Limitations = append([]string(nil), source.Limitations...)
	result.Claims = append([]ai.Claim(nil), source.Claims...)
	for i := range result.Claims {
		result.Claims[i].EvidenceIDs = append([]string(nil), source.Claims[i].EvidenceIDs...)
	}
	if source.Evidence != nil {
		bundle := *source.Evidence
		bundle.Messages = append([]ai.EvidenceMessage(nil), source.Evidence.Messages...)
		bundle.Redactions = make(map[string]int, len(source.Evidence.Redactions))
		for key, value := range source.Evidence.Redactions {
			bundle.Redactions[key] = value
		}
		bundle.Statistics.ActiveTimes = append([]string(nil), source.Evidence.Statistics.ActiveTimes...)
		if !includeEvidenceContent {
			for i := range bundle.Messages {
				bundle.Messages[i].Content = ""
			}
		}
		result.Evidence = &bundle
	}
	return &result
}

func evidenceOptions(cmd *cobra.Command, location *time.Location) (ai.EvidenceOptions, error) {
	maxMessages, err := cmd.Flags().GetInt("evidence-messages")
	if err != nil {
		return ai.EvidenceOptions{}, err
	}
	maxChars, err := cmd.Flags().GetInt("evidence-chars")
	if err != nil {
		return ai.EvidenceOptions{}, err
	}
	if maxMessages < ai.MinEvidenceMessages || maxMessages > ai.MaxEvidenceMessages {
		return ai.EvidenceOptions{}, fmt.Errorf("--evidence-messages 必须在 %d 到 %d 之间", ai.MinEvidenceMessages, ai.MaxEvidenceMessages)
	}
	if maxChars < ai.MinEvidenceChars || maxChars > ai.MaxEvidenceChars {
		return ai.EvidenceOptions{}, fmt.Errorf("--evidence-chars 必须在 %d 到 %d 之间", ai.MinEvidenceChars, ai.MaxEvidenceChars)
	}
	return ai.EvidenceOptions{MaxMessages: maxMessages, MaxChars: maxChars, Location: location}, nil
}

func previewEvidence(cmd *cobra.Command, path, format string, statsOpts stats.AnalyzeOptions, evidenceOpts ai.EvidenceOptions) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("无法读取 %s: %w", path, err)
	}
	if info.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
		return fmt.Errorf("--preview 只支持一个 JSON 文件")
	}
	conversation, err := loader.LoadFile(path)
	if err != nil {
		return err
	}
	statistics, err := stats.AnalyzeConversationWithOptions(conversation, statsOpts)
	if err != nil {
		return err
	}
	bundle, err := ai.PrepareEvidence(conversation, statistics, evidenceOpts)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(cmd.OutOrStdout(), evidencePreviewEnvelope{
			SchemaVersion: evidenceSchemaVersion,
			Contact:       conversation.Talker.DisplayName(),
			Evidence:      bundle,
		})
	}
	return printEvidencePreview(cmd.OutOrStdout(), conversation.Talker.DisplayName(), bundle)
}

func printEvidencePreview(writer io.Writer, contact string, bundle *ai.EvidenceBundle) error {
	truncated := "否"
	if bundle.Sampling.Truncated {
		truncated = "是"
	}
	if _, err := fmt.Fprintf(writer, "Evidence Bundle 预览：%s\nPrompt 版本：%s\n样本：%d/%d 条，%d/%d 字符\n预算：最多 %d 条、%d 字符；是否截断：%s\n时间：%s 至 %s\n聚合统计：总消息 %d（我 %d / 对方 %d），发送比例 %.1f%%，活跃日 %d，自然日 %d，会话 %d\n活跃时段：%s\n\n",
		contact, bundle.PromptVersion, bundle.Sampling.SelectedMessages, bundle.Sampling.CandidateMessages,
		bundle.Sampling.SelectedChars, bundle.Sampling.CandidateChars, bundle.Sampling.MaxMessages, bundle.Sampling.MaxChars,
		truncated, bundle.Sampling.StartTime, bundle.Sampling.EndTime, bundle.Statistics.TotalMessages,
		bundle.Statistics.SentMessages, bundle.Statistics.ReceivedMessages, bundle.Statistics.SentRatio,
		bundle.Statistics.ActiveDays, bundle.Statistics.CalendarDays, bundle.Statistics.Sessions,
		strings.Join(bundle.Statistics.ActiveTimes, "、")); err != nil {
		return err
	}
	keys := make([]string, 0, len(bundle.Redactions))
	for key := range bundle.Redactions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if _, err := fmt.Fprintln(writer, "脱敏计数："); err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := fmt.Fprintf(writer, "  %s: %d\n", key, bundle.Redactions[key]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer, "\n入选摘录："); err != nil {
		return err
	}
	for _, message := range bundle.Messages {
		if _, err := fmt.Fprintf(writer, "  %s [%s] %s %s\n", message.ID, message.Speaker, message.Time, message.Content); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("输出 JSON 失败: %w", err)
	}
	return nil
}

func warnf(writer io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(writer, format, args...)
}

func walkPath(path string, handler func(*loader.Conversation) error) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		if strings.HasSuffix(path, ".json") {
			conv, err := loader.LoadFile(path)
			if err != nil {
				warnf(os.Stderr, "  ⚠️  加载 JSON 失败: %s (%v)\n", path, err)
				return nil
			}
			return handler(conv)
		}
		return nil
	}

	// 遍历目录
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			if err := walkPath(filepath.Join(path, entry.Name()), handler); err != nil {
				continue
			}
			continue
		}

		if strings.HasSuffix(entry.Name(), ".json") {
			conv, err := loader.LoadFile(filepath.Join(path, entry.Name()))
			if err != nil {
				warnf(os.Stderr, "  ⚠️  加载 JSON 失败: %s (%v)\n", entry.Name(), err)
				continue
			}
			if err := handler(conv); err != nil {
				continue
			}
		}
	}

	return nil
}

func listSupportedProviders() string {
	var names []string
	for _, p := range ai.SupportedProviders() {
		names = append(names, p.String())
	}
	return strings.Join(names, ", ")
}

func listProviderEnvs() string {
	var envs []string
	for _, p := range ai.SupportedProviders() {
		envs = append(envs, ai.ProviderConfigs[p].EnvVar)
	}
	return strings.Join(envs, ", ")
}

func init() {
	rootCmd.AddCommand(statsCmd, analyzeCmd, compareCmd, providersCmd)

	// stats 子命令
	statsCmd.Flags().Bool("html", false, "生成 HTML 统计报告")
	statsCmd.Flags().StringP("output", "o", "", "输出目录")
	addAnalysisFlags(statsCmd, true)

	// analyze 子命令
	analyzeCmd.Flags().StringP("provider", "p", "", "AI 提供商（支持: deepseek, moonshot, qwen, doubao, zhipu）")
	analyzeCmd.Flags().Bool("html", false, "生成 AI 分析 HTML 报告")
	analyzeCmd.Flags().StringP("output", "o", "", "输出目录")
	analyzeCmd.Flags().Bool("evidence", false, "发送本地脱敏抽样摘录，生成可引用证据的结构化分析")
	analyzeCmd.Flags().Bool("preview", false, "仅预览 Evidence Bundle，不调用 AI（需配合 --evidence）")
	analyzeCmd.Flags().Int("evidence-messages", ai.DefaultEvidenceMessages, "证据样本的最大消息数")
	analyzeCmd.Flags().Int("evidence-chars", ai.DefaultEvidenceChars, "证据样本的最大字符数")
	addAnalysisFlags(analyzeCmd, true)

	compareCmd.Flags().StringSlice("period", nil, "对比时期，需要两个（YYYY、YYYY-MM 或日期范围）")
	addAnalysisFlags(compareCmd, false)
}

func addAnalysisFlags(command *cobra.Command, withContent bool) {
	command.Flags().Duration("session-gap", stats.DefaultSessionGap, "切分会话的消息间隔")
	command.Flags().String("format", "text", "输出格式：text 或 json")
	if withContent {
		command.Flags().Bool("include-content", false, "在 HTML/JSON 中包含最长消息原文（分享时请谨慎）")
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
