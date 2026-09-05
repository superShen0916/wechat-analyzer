package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/superShen0916/wechat-analyzer/internal/ai"
	"github.com/superShen0916/wechat-analyzer/internal/stats"
)

func TestStatsJSONOutputIsCleanAndPrivateByDefault(t *testing.T) {
	path, err := filepath.Abs("../examples/sample-conversation.json")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	statsCmd.SetOut(&output)
	if err := statsCmd.Flags().Set("format", "json"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = statsCmd.Flags().Set("format", "text")
		statsCmd.SetOut(nil)
	})

	if err := statsCmd.RunE(statsCmd, []string{path}); err != nil {
		t.Fatal(err)
	}
	var envelope statsEnvelope
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not clean JSON: %v\n%s", err, output.String())
	}
	if envelope.SchemaVersion != schemaVersion || len(envelope.Results) != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
	if strings.Contains(output.String(), "Morning! Are we still on") || strings.Contains(output.String(), `"content"`) {
		t.Fatal("default JSON contains message content")
	}
}

func TestStatsForOutputContentIsOptIn(t *testing.T) {
	source := &stats.Stats{TopMessages: []stats.MessageInfo{{Content: "private"}}}
	without := statsForOutput(source, false)
	with := statsForOutput(source, true)
	if without.TopMessages[0].Content != "" {
		t.Fatal("content was not removed")
	}
	if with.TopMessages[0].Content != "private" || source.TopMessages[0].Content != "private" {
		t.Fatal("opt-in content missing or source stats mutated")
	}
}

func TestCommonAnalysisOptionsRejectsInvalidValues(t *testing.T) {
	if err := statsCmd.Flags().Set("format", "yaml"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := commonAnalysisOptions(statsCmd); err == nil {
		t.Fatal("invalid format was accepted")
	}
	_ = statsCmd.Flags().Set("format", "text")
	_ = statsCmd.Flags().Set("session-gap", "0s")
	if _, _, _, err := commonAnalysisOptions(statsCmd); err == nil {
		t.Fatal("zero session gap was accepted")
	}
	_ = statsCmd.Flags().Set("session-gap", "30m")
}

func TestAnalyzeEvidencePreviewWorksWithoutAPIKeyAndEmitsCleanJSON(t *testing.T) {
	for _, provider := range ai.SupportedProviders() {
		t.Setenv(ai.ProviderConfigs[provider].EnvVar, "")
	}
	path := filepath.Join(t.TempDir(), "conversation.json")
	fixture := `{
  "talker":{"remark":"Preview Contact"},
  "messages":[
    {"type_name":"text","create_time":3,"is_sender":true,"content":"电话 13800138000"},
    {"type_name":"text","create_time":1,"content":"邮箱 alice@example.com"},
    {"type_name":"text","create_time":2,"content":"普通消息"}
  ]
}`
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	analyzeCmd.SetOut(&output)
	setAnalyzeFlag(t, "evidence", "true")
	setAnalyzeFlag(t, "preview", "true")
	setAnalyzeFlag(t, "format", "json")
	if err := analyzeCmd.RunE(analyzeCmd, []string{path}); err != nil {
		t.Fatal(err)
	}

	var envelope evidencePreviewEnvelope
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not clean JSON: %v\n%s", err, output.String())
	}
	if envelope.SchemaVersion != evidenceSchemaVersion || envelope.Evidence == nil || len(envelope.Evidence.Messages) != 3 {
		t.Fatalf("envelope = %#v", envelope)
	}
	serialized := output.String()
	if strings.Contains(serialized, "13800138000") || strings.Contains(serialized, "alice@example.com") {
		t.Fatalf("preview leaked identifiers: %s", serialized)
	}
	if !strings.Contains(serialized, "[PHONE]") || !strings.Contains(serialized, "[EMAIL]") {
		t.Fatalf("preview missing redaction markers: %s", serialized)
	}
}

func TestAnalyzePreviewRequiresEvidenceAndSingleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conversation.json")
	if err := os.WriteFile(path, []byte(`{"messages":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	setAnalyzeFlag(t, "preview", "true")
	setAnalyzeFlag(t, "evidence", "false")
	if err := analyzeCmd.RunE(analyzeCmd, []string{path}); err == nil || !strings.Contains(err.Error(), "--evidence") {
		t.Fatalf("error = %v", err)
	}
	setAnalyzeFlag(t, "evidence", "true")
	if err := analyzeCmd.RunE(analyzeCmd, []string{path, path}); err == nil || !strings.Contains(err.Error(), "一个 JSON 文件") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvidenceTextPreviewIncludesBudgetAndAggregateFields(t *testing.T) {
	bundle := &ai.EvidenceBundle{
		PromptVersion: ai.EvidencePromptVersion,
		Messages:      []ai.EvidenceMessage{{ID: "m0001", Speaker: "them", Time: "2026-01-01T00:00:00Z", Content: "已脱敏内容"}},
		Sampling: ai.SamplingSummary{
			CandidateMessages: 10, SelectedMessages: 1, CandidateChars: 900, SelectedChars: 40,
			MaxMessages: 3, MaxChars: 500, StartTime: "2026-01-01T00:00:00Z", EndTime: "2026-01-01T00:00:00Z", Truncated: true,
		},
		Redactions: map[string]int{"email": 1},
		Statistics: ai.EvidenceStatistics{
			TotalMessages: 10, SentMessages: 4, ReceivedMessages: 6, SentRatio: 40,
			ActiveDays: 2, CalendarDays: 3, Sessions: 2, ActiveTimes: []string{"9点", "20点"},
		},
	}
	var output bytes.Buffer
	if err := printEvidencePreview(&output, "对方", bundle); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"预算：最多 3 条、500 字符；是否截断：是",
		"聚合统计：总消息 10（我 4 / 对方 6），发送比例 40.0%，活跃日 2，自然日 3，会话 2",
		"活跃时段：9点、20点",
		"m0001 [them]",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("text preview missing %q:\n%s", want, output.String())
		}
	}
}

func TestAnalysisForOutputHidesEvidenceWithoutMutatingSource(t *testing.T) {
	source := &ai.AnalysisResult{
		Claims: []ai.Claim{{EvidenceIDs: []string{"m0001"}}},
		Evidence: &ai.EvidenceBundle{
			Messages:   []ai.EvidenceMessage{{ID: "m0001", Content: "[EMAIL]"}},
			Redactions: map[string]int{"email": 1},
			Statistics: ai.EvidenceStatistics{ActiveTimes: []string{"上午"}},
		},
	}
	without := analysisForOutput(source, false)
	with := analysisForOutput(source, true)
	if without.Evidence.Messages[0].Content != "" {
		t.Fatal("default analysis output exposed evidence content")
	}
	if with.Evidence.Messages[0].Content != "[EMAIL]" || source.Evidence.Messages[0].Content != "[EMAIL]" {
		t.Fatal("opt-in content missing or source result mutated")
	}
	without.Claims[0].EvidenceIDs[0] = "changed"
	without.Evidence.Redactions["email"] = 9
	if source.Claims[0].EvidenceIDs[0] != "m0001" || source.Evidence.Redactions["email"] != 1 {
		t.Fatal("analysisForOutput did not deep-copy nested data")
	}
}

func setAnalyzeFlag(t *testing.T, name, value string) {
	t.Helper()
	flag := analyzeCmd.Flags().Lookup(name)
	previous := flag.Value.String()
	if err := analyzeCmd.Flags().Set(name, value); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = analyzeCmd.Flags().Set(name, previous)
		analyzeCmd.SetOut(nil)
	})
}
