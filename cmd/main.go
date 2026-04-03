package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/superShen0916/wechat-analyzer/internal/ai"
	"github.com/superShen0916/wechat-analyzer/internal/loader"
	"github.com/superShen0916/wechat-analyzer/internal/report"
	"github.com/superShen0916/wechat-analyzer/internal/stats"
)

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
	colorError    = color.New(color.FgHiRed)
	colorSuccess  = color.New(color.FgGreen)
	colorInfo     = color.New(color.FgCyan)
)

// 打印分隔线
func printDivider(char string, length int) {
	fmt.Println(colorLabel.Sprint(strings.Repeat(char, length)))
}

// 打印带标题的区块
func printBlock(title string, maxWidth int) {
	printDivider("═", maxWidth)
	colorTitle.Printf(" %s ", title)
	fmt.Println()
	printDivider("═", maxWidth)
}

// 打印统计项
func printStat(name string, value string) {
	colorStatName.Printf("%20s:", name)
	colorStatVal.Printf(" %8s", value)
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

		html, _ := cmd.Flags().GetBool("html")
		outputDir, _ := cmd.Flags().GetString("output")

		paths := args
		for _, path := range paths {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				fmt.Printf("  ⚠️  文件不存在: %s\n", path)
				continue
			}

			walkPath(path, func(conv *loader.Conversation) error {
				colorInfo.Printf("\n🔍 分析: %s (%d 条消息)\n\n", conv.Talker.DisplayName(), len(conv.Messages))

				// 统计分析
				s, err := stats.AnalyzeConversation(conv)
				if err != nil {
					fmt.Printf("  ⚠️  统计失败: %v\n", err)
					return nil
				}

				// 打印到终端
				printStats(s, conv)

				// 生成 HTML 报告
				if html {
					var htmlDir string
					if outputDir == "" {
						htmlDir = "./wechat_analyze_stats"
					} else {
						htmlDir = outputDir
					}
					path, err := report.GenerateHTMLReport(htmlDir, conv, s, nil)
					if err != nil {
						fmt.Printf("  ⚠️  生成报告失败: %v\n", err)
						return nil
					}
					fmt.Printf("\n📄 HTML 报告已生成: %s\n", path)
				}

				return nil
			})
		}

		return nil
	},
}

func printStats(s *stats.Stats, conv *loader.Conversation) {
	width := 60
	printBlock(fmt.Sprintf("📊 聊天记录统计 (%s)", conv.Talker.DisplayName()), width)

	printStat("总消息数", fmt.Sprintf("%d 条", s.Total))
	printStat("平均每条长度", fmt.Sprintf("%.2f 字符", s.AvgLength))
	printStat("日均消息数", fmt.Sprintf("%.2f 条/天", s.MsgPerDay))
	fmt.Println()

	printStat("我发的消息", fmt.Sprintf("%d (%.1f%%)", s.SentTotal, s.SentRatio))
	printStat("对方发的", fmt.Sprintf("%d (%.1f%%)", s.ReceivedTotal, 100-s.SentRatio))
	printStat("我先开口", fmt.Sprintf("%d (%.1f%%)", s.FirstMessageCount, s.FirstMessageRatio))
	fmt.Println()

	colorLabel.Println("⏰ 活跃时段分布:")
	var peakHours []int
	for hour := 0; hour < 24; hour++ {
		if s.MsgPerHour[hour] > 0 {
			peakHours = append(peakHours, hour)
		}
	}

	// 按消息数排序并取前5个
	func() {
		max := 5
		if len(peakHours) > max {
			peakHours = peakHours[:max]
		}
	}()

	if len(peakHours) > 0 {
		maxCount := s.MsgPerHour[peakHours[0]]
		for _, h := range peakHours {
			count := s.MsgPerHour[h]
			// 计算长度，最长 20 个方块
			barLen := count * 20 / maxCount
			bar := strings.Repeat("█", barLen)
			colorStatVal.Printf("  %02d点%02d → %d条 %s\n", h, h+1, count, bar)
		}
		fmt.Println()
	}

	colorLabel.Println("💬 消息类型分布:")
	for t, cnt := range s.MsgTypes {
		p := float64(cnt) / float64(s.Total) * 100
		colorStatVal.Printf("  %-10s: %d (%.1f%%)\n", t, cnt, p)
	}
	fmt.Println()
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

		providerStr, _ := cmd.Flags().GetString("provider")
		if providerStr == "" {
			autoProviders := ai.DetectProviders()
			if len(autoProviders) == 0 {
				return fmt.Errorf("未检测到任何已配置的 AI 提供商，请设置以下环境变量之一:\n  %s", listProviderEnvs())
			}
			providerStr = autoProviders[0].String()
			cmd.Printf("🤖 自动使用提供商: %s\n\n", providerStr)
		}

		provider := ai.AIProvider(providerStr)
		supported := false
		for _, p := range []ai.AIProvider{
			ai.ProviderAnthropic, ai.ProviderDeepSeek, ai.ProviderMoonshot,
			ai.ProviderQwen, ai.ProviderDoubao, ai.ProviderZhipu,
		} {
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

		for _, path := range args {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				fmt.Printf("  ⚠️  文件不存在: %s\n", path)
				continue
			}

			walkPath(path, func(conv *loader.Conversation) error {
				colorInfo.Printf("\n🤖 AI 分析 %s (%d 条消息)...\n\n", conv.Talker.DisplayName(), len(conv.Messages))

				// 先做基础统计
				s, err := stats.AnalyzeConversation(conv)
				if err != nil {
					fmt.Printf("  ⚠️  统计失败: %v\n", err)
					return nil
				}

				// AI 分析
				aiRes, err := ai.AnalyzeConversation(ctx, conv, s, provider)
				if err != nil {
					fmt.Printf("  ⚠️  AI 分析失败: %v\n", err)
					return nil
				}

				// 打印 AI 结果
				printAIResult(aiRes)

				// 生成 HTML 报告
				if html {
					var htmlDir string
					if outputDir == "" {
						htmlDir = "./wechat_analyze_ai"
					} else {
						htmlDir = outputDir
					}
					reportPath, err := report.GenerateHTMLReport(htmlDir, conv, s, aiRes)
					if err != nil {
						fmt.Printf("  ⚠️  生成报告失败: %v\n", err)
						return nil
					}
					fmt.Printf("\n📄 HTML 报告已生成: %s\n", reportPath)
				}

				return nil
			})
		}

		return nil
	},
}

func printAIResult(res *ai.AnalysisResult) {
	width := 65
	printBlock("🎭 AI 人格画像", width)

	colorLabel.Println("人格称号:")
	fmt.Printf("  %s\n\n", res.Title)

	colorLabel.Println("人格类型:")
	fmt.Printf("  %s\n\n", res.Archetype)

	colorLabel.Println("人格标签:")
	for i, tag := range res.PersonalityTags {
		if i > 0 {
			fmt.Print("  ")
		}
		colorSuccess.Printf("#%s ", tag)
	}
	fmt.Println("\n")

	colorLabel.Println("人格画像:")
	fmt.Printf("  %s\n\n", res.Personality)

	colorLabel.Println("关系分析:")
	fmt.Printf("  %s\n\n", res.Relationship)

	colorLabel.Println("常聊话题:")
	for _, topic := range res.Topics {
		colorStatVal.Printf("  • %s\n", topic)
	}
	fmt.Println()

	colorLabel.Println("一句话总结:")
	colorStatName.Printf("  %s\n", res.Summary)
	colorLabel.Println(strings.Repeat("═", width))
}

// ── providers 命令 ────────────────────────────────────────────────────────────
var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "列出所有支持的 AI 提供商",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("📋 支持的 AI 提供商:\n")

		providers := []ai.AIProvider{
			ai.ProviderDeepSeek, ai.ProviderMoonshot, ai.ProviderQwen,
			ai.ProviderDoubao, ai.ProviderZhipu, ai.ProviderAnthropic,
		}

		for _, p := range providers {
			cfg := ai.ProviderConfigs[p]
			k := os.Getenv(cfg.EnvVar)
			status := "❌"
			if k != "" {
				status = "✅"
			}
			fmt.Printf("  %s %-12s 环境变量: %s\n", status, p, cfg.EnvVar)
		}

		fmt.Println("\n📝 提示：设置对应环境变量后，工具会自动检测可用提供商。")
		return nil
	},
}

// ── 辅助函数 ──────────────────────────────────────────────────────────────────

// walkPath 遍历路径，可以是单个文件或目录
func walkPath(path string, handler func(*loader.Conversation) error) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		if strings.HasSuffix(path, ".json") {
			conv, err := loader.LoadFile(path)
			if err != nil {
				fmt.Printf("  ⚠️  加载 JSON 失败: %s (%v)\n", path, err)
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
				fmt.Printf("  ⚠️  加载 JSON 失败: %s (%v)\n", entry.Name(), err)
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
	for _, p := range []ai.AIProvider{
		ai.ProviderDeepSeek, ai.ProviderMoonshot, ai.ProviderQwen,
		ai.ProviderDoubao, ai.ProviderZhipu, ai.ProviderAnthropic,
	} {
		names = append(names, p.String())
	}
	return strings.Join(names, ", ")
}

func listProviderEnvs() string {
	var envs []string
	for _, p := range []ai.AIProvider{
		ai.ProviderDeepSeek, ai.ProviderMoonshot, ai.ProviderQwen,
		ai.ProviderDoubao, ai.ProviderZhipu, ai.ProviderAnthropic,
	} {
		envs = append(envs, ai.ProviderConfigs[p].EnvVar)
	}
	return strings.Join(envs, ", ")
}

func init() {
	rootCmd.AddCommand(statsCmd, analyzeCmd, providersCmd)

	// stats 子命令
	statsCmd.Flags().Bool("html", false, "生成 HTML 统计报告")
	statsCmd.Flags().StringP("output", "o", "", "输出目录")

	// analyze 子命令
	analyzeCmd.Flags().StringP("provider", "p", "", "AI 提供商（支持: deepseek, moonshot, qwen, doubao, zhipu）")
	analyzeCmd.Flags().Bool("html", false, "生成 AI 分析 HTML 报告")
	analyzeCmd.Flags().StringP("output", "o", "", "输出目录")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
