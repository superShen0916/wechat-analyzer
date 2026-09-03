package ai

import (
	"reflect"
	"testing"

	"github.com/superShen0916/wechat-analyzer/internal/loader"
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
