package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

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
