package ai

import (
	"context"
	"reflect"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"github.com/superShen0916/wechat-analyzer/internal/loader"
	"github.com/superShen0916/wechat-analyzer/internal/stats"
)

func TestSupportedProvidersOrder(t *testing.T) {
	want := []AIProvider{
		ProviderDeepSeek,
		ProviderMoonshot,
		ProviderQwen,
		ProviderDoubao,
		ProviderZhipu,
		ProviderAnthropic,
	}
	if got := SupportedProviders(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedProviders() = %v, want %v", got, want)
	}
}

func TestDetectProvidersUsesDeterministicPriority(t *testing.T) {
	for _, provider := range SupportedProviders() {
		t.Setenv(ProviderConfigs[provider].EnvVar, "")
	}
	t.Setenv(ProviderConfigs[ProviderAnthropic].EnvVar, "anthropic-key")
	t.Setenv(ProviderConfigs[ProviderQwen].EnvVar, "qwen-key")

	want := []AIProvider{ProviderQwen, ProviderAnthropic}
	if got := DetectProviders(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectProviders() = %v, want %v", got, want)
	}
}

func TestParseResponseHandlesShortTopicList(t *testing.T) {
	raw := `标题：可靠性守门员
人格类型：INTJ-系统派
标签：#可靠 #务实
人格画像：重视边界条件。
关系分析：沟通直接。
常聊话题：测试、可靠性
总结：先把失败路径想清楚。`

	got := parseResponse(raw, &loader.Conversation{})
	if got.Title != "可靠性守门员" {
		t.Fatalf("Title = %q", got.Title)
	}
	if !reflect.DeepEqual(got.Topics, []string{"测试", "可靠性"}) {
		t.Fatalf("Topics = %v", got.Topics)
	}
	if !reflect.DeepEqual(got.PersonalityTags, []string{"可靠", "务实"}) {
		t.Fatalf("PersonalityTags = %v", got.PersonalityTags)
	}
}

func TestGenerateTitle(t *testing.T) {
	if got := generateTitle("INTP-理性派"); got != "思考机器型" {
		t.Fatalf("generateTitle() = %q", got)
	}
	if got := generateTitle("OTHER"); got != "灵魂伙伴" {
		t.Fatalf("generateTitle() default = %q", got)
	}
}

func TestAggregateAnalysisSendsStatisticsWithoutMessageContent(t *testing.T) {
	conversation := &loader.Conversation{
		Talker:   loader.Contact{Remark: "Contact"},
		Messages: []loader.Message{{TypeName: "text", Content: "TOP-SECRET-MESSAGE"}},
	}
	statistics := &stats.Stats{Total: 1, ActiveDays: map[string]int{"2026-01-01": 1}}
	client := &fakeCompletionClient{response: openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message: openai.ChatCompletionMessage{Content: "标题：结果\n人格类型：类型\n标签：#标签\n人格画像：画像\n关系分析：关系\n常聊话题：话题\n总结：总结"},
	}}}}
	if _, err := analyzeAggregateWithClient(context.Background(), client, "model", conversation, statistics, AnalysisOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, message := range client.request.Messages {
		if strings.Contains(message.Content, "TOP-SECRET-MESSAGE") {
			t.Fatal("aggregate-only prompt included message content")
		}
	}
}
