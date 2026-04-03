// Package ai AI 分析客户端
package ai

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/superShen0916/wechat-analyzer/internal/loader"
	"github.com/superShen0916/wechat-analyzer/internal/stats"
)

// AIProvider AI 提供商
type AIProvider string

const (
	ProviderAnthropic AIProvider = "anthropic"
	ProviderDeepSeek  AIProvider = "deepseek"
	ProviderMoonshot  AIProvider = "moonshot"
	ProviderQwen      AIProvider = "qwen"
	ProviderDoubao    AIProvider = "doubao"
	ProviderZhipu     AIProvider = "zhipu"
)

func (p AIProvider) String() string {
	return string(p)
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	EnvVar  string `json:"env_var"`  // 环境变量名
	BaseURL string `json:"base_url"` // API 地址
	Model   string `json:"model"`    // 模型名
	ChatModel string `json:"chat_model,omitempty"` // 聊天模型名
}

// ProviderConfigs 全局配置，供外部访问
var ProviderConfigs = map[AIProvider]ProviderConfig{
	ProviderAnthropic: {
		EnvVar:  "ANTHROPIC_API_KEY",
		BaseURL: "https://api.anthropic.com/v1/",
		Model:   "claude-opus-4-6",
		ChatModel: "claude-opus-4-6",
	},
	ProviderDeepSeek: {
		EnvVar:  "DEEPSEEK_API_KEY",
		BaseURL: "https://api.deepseek.com/v1/",
		Model:   "deepseek-chat",
		ChatModel: "deepseek-chat",
	},
	ProviderMoonshot: {
		EnvVar:  "MOONSHOT_API_KEY",
		BaseURL: "https://api.moonshot.cn/v1/",
		Model:   "moonshot-v1-8k",
		ChatModel: "moonshot-v1-8k",
	},
	ProviderQwen: {
		EnvVar:  "DASHSCOPE_API_KEY",
		BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1/",
		Model:   "qwen-turbo",
		ChatModel: "qwen-turbo",
	},
	ProviderDoubao: {
		EnvVar:  "DOUBAO_API_KEY",
		BaseURL: "https://ark.cn-beijing.volces.com/api/v3/",
		Model:   "doubao-pro-4k",
		ChatModel: "doubao-pro-4k",
	},
	ProviderZhipu: {
		EnvVar:  "ZHIPU_API_KEY",
		BaseURL: "https://open.bigmodel.cn/api/paas/v4/",
		Model:   "glm-4-flash",
		ChatModel: "glm-4-flash",
	},
}

// AnalysisResult AI 分析结果
type AnalysisResult struct {
	Title           string   `json:"title"`           // 人格称号
	PersonalityTags []string `json:"tags"`            // 人格标签
	Archetype       string   `json:"archetype"`        // 人格类型

	Personality string `json:"personality"` // 人格画像
	Relationship string `json:"relationship"` // 关系分析

	Topics  []string `json:"topics"`   // 常聊话题
	Summary string   `json:"summary"`   // 总结
}

// AnalyzeConversation AI 分析对话
func AnalyzeConversation(ctx context.Context, conv *loader.Conversation, stats *stats.Stats, provider AIProvider) (*AnalysisResult, error) {
	// 检查配置
	cfg, ok := ProviderConfigs[provider]
	if !ok {
		return nil, fmt.Errorf("不支持的提供商: %s", provider)
	}

	k := os.Getenv(cfg.EnvVar)
	if k == "" {
		return nil, fmt.Errorf("请设置环境变量 %s", cfg.EnvVar)
	}

	// 准备 prompt
	systemPrompt, userPrompt := buildPrompt(conv, stats)

	// 创建客户端
	client := openai.NewClient(k)
	if cfg.BaseURL != "" {
		config := openai.DefaultConfig(k)
		config.BaseURL = cfg.BaseURL
		client = openai.NewClientWithConfig(config)
	}

	// 调用聊天接口
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: cfg.ChatModel,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 2000,
	})

	if err != nil {
		return nil, fmt.Errorf("API 调用失败: %w", err)
	}

	content := resp.Choices[0].Message.Content
	fmt.Println(content)

	// 解析响应
	result := parseResponse(content, conv)
	result.Title = generateTitle(result.Archetype)
	return result, nil
}

func buildPrompt(conv *loader.Conversation, stats *stats.Stats) (string, string) {
	startDate, endDate := stats.GetActiveDateRange()
	talkName := conv.Talker.DisplayName()

	systemPrompt := `
你是一个顶尖的微信聊天记录分析专家，对人性洞察敏锐且幽默犀利。

请严格按照以下结构输出，不要有任何其他文字：
1. 人格称号（给对方起一个4-8字的有趣称号，如'深夜代码诗人'、'Bug终结者'）
2. 人格类型（类似MBTI的四个字母，但要更贴切的形容词，如'INTP-理性派程序员'）
3. 人格标签（4个标签，用#号分隔，如#深夜修仙 #洁癖程序员 #注释恐惧症）
4. 人格画像（用一段生动的话描述对方的性格、说话风格，200字内）
5. 关系分析（分析我和对方的沟通模式：谁更主动、谁更话多、关系是怎样的，200字内）
6. 常聊话题（提取对方常聊的三个话题或关键词）
7. 一句话总结（总结性评价，风趣犀利，像江湖传说一样）

格式示例：
标题：深夜代码诗人
人格类型：INTP-理性派程序员
标签：#深夜修仙 #洁癖程序员 #注释恐惧症 #性能偏执狂
人格画像：对方是一个严谨而理性的工程师，说话言简意赅，不喜欢废话，深夜是创作高峰...
关系分析：你们的沟通纯粹高效，对方掌握技术话语权，你会向对方请教问题...
常聊话题：代码优化、项目进度、技术选型
总结：一个真正的代码匠人，只在深夜发光
`

	userPrompt := fmt.Sprintf(`
聊天对象：%s
聊天周期：从 %s 到 %s 共计 %.1f 天
总消息数：%d条 日均 %.2f条
消息比例：我发了 %.1f%% (%d条) 对方发了 %.1f%% (%d条)
我先开口：%.1f%% 的对话是我先发起的
活跃时段：%s
人格类型：根据聊天记录推断，给出有趣的称号和类型

请分析这组聊天记录，重点放在：
1. 对方的人格特征和说话风格
2. 我们之间的沟通模式和关系
3. 最常聊的话题和关注点
`, talkName, startDate, endDate,
		float64(len(stats.ActiveDays)), stats.Total, stats.MsgPerDay,
		stats.SentRatio, stats.SentTotal,
		100-stats.SentRatio, stats.ReceivedTotal,
		stats.FirstMessageRatio,
		strings.Join(stats.GetMostActiveTime(), ", "))

	return systemPrompt, userPrompt
}

func parseResponse(raw string, conv *loader.Conversation) *AnalysisResult {
	result := &AnalysisResult{}

	// 提取各部分
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "标题：") {
			result.Title = strings.TrimPrefix(line, "标题：")
		} else if strings.HasPrefix(line, "人格类型：") {
			result.Archetype = strings.TrimPrefix(line, "人格类型：")
		} else if strings.HasPrefix(line, "标签：") {
			tagLine := strings.TrimPrefix(line, "标签：")
			tags := strings.Split(tagLine, "#")
			for _, t := range tags {
				t = strings.TrimSpace(t)
				if t != "" {
					result.PersonalityTags = append(result.PersonalityTags, t)
				}
			}
		} else if strings.HasPrefix(line, "人格画像：") {
			result.Personality = strings.TrimPrefix(line, "人格画像：")
		} else if strings.HasPrefix(line, "关系分析：") {
			result.Relationship = strings.TrimPrefix(line, "关系分析：")
		} else if strings.HasPrefix(line, "常聊话题：") {
			line = strings.TrimPrefix(line, "常聊话题：")
			topics := strings.Split(line, "、")
			for _, t := range topics[:3] {
				t = strings.TrimSpace(t)
				if t != "" {
					result.Topics = append(result.Topics, t)
				}
			}
		} else if strings.HasPrefix(line, "总结：") {
			result.Summary = strings.TrimPrefix(line, "总结：")
		}
	}

	return result
}

func generateTitle(archetype string) string {
	// 如果用户没有用prompt生成标题，自动生成一个
	if strings.HasPrefix(archetype, "INTJ") {
		return "运筹帷幄型"
	} else if strings.HasPrefix(archetype, "INTP") {
		return "思考机器型"
	} else if strings.HasPrefix(archetype, "ENTJ") {
		return "领袖型"
	} else if strings.HasPrefix(archetype, "ENFJ") {
		return "温暖大家型"
	}
	// 默认标题
	return "灵魂伙伴"
}

// DetectProviders 自动检测已配置的提供商
func DetectProviders() []AIProvider {
	var providers []AIProvider
	for p, cfg := range ProviderConfigs {
		if os.Getenv(cfg.EnvVar) != "" {
			providers = append(providers, p)
		}
	}
	return providers
}
