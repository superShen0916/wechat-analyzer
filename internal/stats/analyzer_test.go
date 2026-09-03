package stats

import (
	"math"
	"testing"
	"time"

	"github.com/superShen0916/wechat-analyzer/internal/loader"
)

func TestAnalyzeConversation(t *testing.T) {
	dayOneMorning := time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local).Unix()
	dayOneEvening := time.Date(2026, 1, 1, 20, 0, 0, 0, time.Local).Unix()
	dayTwoMorning := time.Date(2026, 1, 2, 9, 30, 0, 0, time.Local).Unix()
	dayTwoEvening := time.Date(2026, 1, 2, 20, 30, 0, 0, time.Local).Unix()
	conversation := &loader.Conversation{
		Total: 99,
		Messages: []loader.Message{
			{Content: "你好", TypeName: "text", IsSender: true, CreateTime: dayOneMorning},
			{Content: "abc", TypeName: "text", IsSender: false, CreateTime: dayOneEvening},
			{Content: "", TypeName: "image", IsSender: false, CreateTime: dayTwoMorning},
			{Content: "x", TypeName: "text", IsSender: true, CreateTime: dayTwoEvening},
		},
	}

	got, err := AnalyzeConversation(conversation)
	if err != nil {
		t.Fatalf("AnalyzeConversation() error = %v", err)
	}
	if got.Total != 4 {
		t.Fatalf("Total = %d, want 4", got.Total)
	}
	assertClose(t, "AvgLength", got.AvgLength, 1.5)
	assertClose(t, "MsgPerDay", got.MsgPerDay, 2)
	if got.SentTotal != 2 || got.ReceivedTotal != 2 {
		t.Fatalf("sent/received = %d/%d, want 2/2", got.SentTotal, got.ReceivedTotal)
	}
	assertClose(t, "SentRatio", got.SentRatio, 50)
	if got.FirstMessageCount != 1 {
		t.Fatalf("FirstMessageCount = %d, want 1", got.FirstMessageCount)
	}
	assertClose(t, "FirstMessageRatio", got.FirstMessageRatio, 50)
	if got.MsgPerHour[9] != 2 || got.MsgPerHour[20] != 2 {
		t.Fatalf("hour counts = 09:%d 20:%d, want 2 and 2", got.MsgPerHour[9], got.MsgPerHour[20])
	}
	if got.MsgTypes["text"] != 3 || got.MsgTypes["image"] != 1 {
		t.Fatalf("message types = %#v", got.MsgTypes)
	}
	if len(got.TopMessages) != 3 || got.TopMessages[0].Length != 3 {
		t.Fatalf("TopMessages = %#v", got.TopMessages)
	}
}

func TestAnalyzeConversationRejectsNoMessages(t *testing.T) {
	tests := []*loader.Conversation{nil, {}, {Total: 12}}
	for _, conversation := range tests {
		if _, err := AnalyzeConversation(conversation); err == nil {
			t.Fatal("AnalyzeConversation() error = nil, want empty conversation error")
		}
	}
}

func TestStatsHelpersHandleSmallInputs(t *testing.T) {
	stats := &Stats{
		MsgPerHour: make([]int, 24),
		MsgTypes:   map[string]int{"text": 1},
		ActiveDays: map[string]int{"2026-01-02": 1, "2026-01-01": 2},
	}
	stats.MsgPerHour[8] = 2

	if got := stats.GetMostActiveTime(); len(got) != 1 || got[0] != "8点" {
		t.Fatalf("GetMostActiveTime() = %v", got)
	}
	start, end := stats.GetActiveDateRange()
	if start != "2026-01-01" || end != "2026-01-02" {
		t.Fatalf("GetActiveDateRange() = %q, %q", start, end)
	}
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("%s = %f, want %f", name, got, want)
	}
}
