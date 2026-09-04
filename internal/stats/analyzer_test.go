package stats

import (
	"math"
	"reflect"
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

func TestAnalyzeConversationSortsCopyAndSessionBoundaries(t *testing.T) {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	conversation := &loader.Conversation{Messages: []loader.Message{
		{LocalID: 3, IsSender: true, CreateTime: base.Add(60*time.Minute + time.Second).Unix()},
		{LocalID: 1, IsSender: true, CreateTime: base.Unix()},
		{LocalID: 2, IsSender: false, CreateTime: base.Add(30 * time.Minute).Unix()},
	}}
	original := append([]loader.Message(nil), conversation.Messages...)

	got, err := AnalyzeConversationWithOptions(conversation, AnalyzeOptions{Location: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(conversation.Messages, original) {
		t.Fatal("AnalyzeConversationWithOptions modified the input message slice")
	}
	if got.Relationship.TotalSessions != 2 {
		t.Fatalf("TotalSessions = %d, want 2", got.Relationship.TotalSessions)
	}
	if got.Relationship.LongestSessionMessages != 2 {
		t.Fatalf("LongestSessionMessages = %d, want 2", got.Relationship.LongestSessionMessages)
	}
	if got.Relationship.TheirResponses.Count != 1 || got.Relationship.TheirResponses.MedianSeconds != 1800 {
		t.Fatalf("TheirResponses = %#v", got.Relationship.TheirResponses)
	}
}

func TestAnalyzeConversationTurnResponsesAndPercentiles(t *testing.T) {
	base := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	offsets := []time.Duration{0, time.Minute, 3 * time.Minute, 4 * time.Minute, 8 * time.Minute, 9 * time.Minute, 19 * time.Minute}
	senders := []bool{true, true, false, false, true, false, true}
	conversation := &loader.Conversation{}
	for i := range offsets {
		conversation.Messages = append(conversation.Messages, loader.Message{IsSender: senders[i], CreateTime: base.Add(offsets[i]).Unix()})
	}
	got, err := AnalyzeConversationWithOptions(conversation, AnalyzeOptions{Location: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	// 我的响应是 4 和 10 分钟；对方是 2 和 1 分钟。同方 burst 不产生样本。
	if got.Relationship.MyResponses.Count != 2 || got.Relationship.MyResponses.MedianSeconds != 420 || got.Relationship.MyResponses.P90Seconds != 600 {
		t.Fatalf("MyResponses = %#v", got.Relationship.MyResponses)
	}
	if got.Relationship.TheirResponses.Count != 2 || got.Relationship.TheirResponses.MedianSeconds != 90 || got.Relationship.TheirResponses.P90Seconds != 120 {
		t.Fatalf("TheirResponses = %#v", got.Relationship.TheirResponses)
	}
}

func TestAnalyzeConversationCalendarAndMonthlyMetrics(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	conversation := &loader.Conversation{Messages: []loader.Message{
		{IsSender: true, CreateTime: time.Date(2026, 1, 30, 23, 50, 0, 0, loc).Unix()},
		{IsSender: false, CreateTime: time.Date(2026, 1, 31, 0, 10, 0, 0, loc).Unix()},
		{IsSender: false, CreateTime: time.Date(2026, 2, 1, 0, 0, 0, 0, loc).Unix()},
		{IsSender: true, CreateTime: time.Date(2026, 2, 3, 0, 0, 0, 0, loc).Unix()},
	}}
	got, err := AnalyzeConversationWithOptions(conversation, AnalyzeOptions{Location: loc})
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveDayCount != 4 || got.CalendarDays != 5 || got.MsgPerDay != 1 || got.MsgPerCalendarDay != .8 {
		t.Fatalf("day metrics = active:%d calendar:%d per-active:%v per-calendar:%v", got.ActiveDayCount, got.CalendarDays, got.MsgPerDay, got.MsgPerCalendarDay)
	}
	if got.Relationship.LongestActiveStreak != 3 {
		t.Fatalf("LongestActiveStreak = %d, want 3", got.Relationship.LongestActiveStreak)
	}
	if len(got.Relationship.Monthly) != 2 || got.Relationship.Monthly[0].Month != "2026-01" || got.Relationship.Monthly[0].Sessions != 1 || got.Relationship.Monthly[1].Sessions != 2 {
		t.Fatalf("Monthly = %#v", got.Relationship.Monthly)
	}
}

func TestAnalyzeConversationSingleMessageHasNoResponses(t *testing.T) {
	got, err := AnalyzeConversation(&loader.Conversation{Messages: []loader.Message{{IsSender: true, CreateTime: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Relationship.StartedByMe != 1 || got.Relationship.EndedByMe != 1 || got.Relationship.MyResponses.Count != 0 || got.Relationship.TheirResponses.Count != 0 {
		t.Fatalf("Relationship = %#v", got.Relationship)
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
