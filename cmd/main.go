package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
				fmt.Printf("\n🔍 分析: %s (%d 条消息)\n", conv.Talker.DisplayName(), len(conv.Messages))

				// 统计分析
				s, err := stats.AnalyzeConversation(conv)
				if err != nil {
					fmt.Printf("  ⚠️  统计失败: %v\n", err)
					return nil
				}

				// 打印到终端
				s.Print(conv)

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
				fmt.Printf("\n🤖 AI 分析 %s (%d 条消息)...\n\n", conv.Talker.DisplayName(), len(conv.Messages))

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
				fmt.Println(strings.Repeat("=", 70))
				fmt.Printf("🎭 AI 分析结果\n")
				fmt.Println(strings.Repeat("=", 70))
				fmt.Printf("人格称号：%s\n", aiRes.Title)
				fmt.Printf("人格类型：%s\n", aiRes.Archetype)
				fmt.Printf("人格标签：#%s\n\n", strings.Join(aiRes.PersonalityTags, " #"))
				fmt.Printf("人格画像：\n  %s\n\n", aiRes.Personality)
				fmt.Printf("关系分析：\n  %s\n\n", aiRes.Relationship)
				fmt.Printf("常聊话题：\n  %s\n\n", strings.Join(aiRes.Topics, "\n  "))
				fmt.Printf("一句话总结：\n  %s\n", aiRes.Summary)
				fmt.Println(strings.Repeat("=", 70))

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