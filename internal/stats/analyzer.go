// Package stats 聊天记录统计分析
package stats

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/superShen0916/wechat-analyzer/internal/loader"
)

const DefaultSessionGap = 30 * time.Minute

type AnalyzeOptions struct {
	SessionGap time.Duration
	Location   *time.Location
}

// Stats 的新字段只追加，保留 v0.1 JSON 契约。
type Stats struct {
	Total     int     `json:"total_messages"`
	AvgLength float64 `json:"avg_msg_length"`

	MsgPerDay         float64 `json:"msgs_per_day"` // 兼容字段：活跃日日均
	ActiveDayCount    int     `json:"active_day_count"`
	CalendarDays      int     `json:"calendar_days"`
	MsgPerActiveDay   float64 `json:"msgs_per_active_day"`
	MsgPerCalendarDay float64 `json:"msgs_per_calendar_day"`
	MsgPerHour        []int   `json:"msgs_per_hour"`
	Timezone          string  `json:"timezone"`

	SentTotal     int     `json:"sent_total"`
	ReceivedTotal int     `json:"received_total"`
	SentRatio     float64 `json:"sent_ratio"`

	FirstMessageCount int     `json:"first_message_count"`
	FirstMessageRatio float64 `json:"first_message_ratio"`

	ActiveDays  map[string]int `json:"active_days"`
	TopMessages []MessageInfo  `json:"top_messages,omitempty"`
	MsgTypes    map[string]int `json:"msg_types"`

	Relationship RelationshipStats `json:"relationship"`
}

type MessageInfo struct {
	Content    string `json:"content,omitempty"`
	Length     int    `json:"length"`
	IsSender   bool   `json:"is_sender"`
	CreateTime int64  `json:"create_time"`
}

type RelationshipStats struct {
	SessionGapSeconds      int64          `json:"session_gap_seconds"`
	TotalSessions          int            `json:"total_sessions"`
	AvgMessagesPerSession  float64        `json:"avg_messages_per_session"`
	LongestSessionMessages int            `json:"longest_session_messages"`
	LongestSessionSeconds  int64          `json:"longest_session_seconds"`
	StartedByMe            int            `json:"started_by_me"`
	StartedByThem          int            `json:"started_by_them"`
	StartedByMeRatio       float64        `json:"started_by_me_ratio"`
	StartedByThemRatio     float64        `json:"started_by_them_ratio"`
	EndedByMe              int            `json:"ended_by_me"`
	EndedByThem            int            `json:"ended_by_them"`
	EndedByMeRatio         float64        `json:"ended_by_me_ratio"`
	EndedByThemRatio       float64        `json:"ended_by_them_ratio"`
	MyResponses            ResponseStats  `json:"my_responses"`
	TheirResponses         ResponseStats  `json:"their_responses"`
	LongestActiveStreak    int            `json:"longest_active_streak"`
	Monthly                []MonthlyStats `json:"monthly"`
}

type ResponseStats struct {
	Count         int     `json:"count"`
	MedianSeconds float64 `json:"median_seconds"`
	P90Seconds    float64 `json:"p90_seconds"`
}

type MonthlyStats struct {
	Month      string `json:"month"`
	Total      int    `json:"total_messages"`
	Sent       int    `json:"sent_messages"`
	Received   int    `json:"received_messages"`
	Sessions   int    `json:"sessions"`
	ActiveDays int    `json:"active_days"`
}

func AnalyzeConversation(conv *loader.Conversation) (*Stats, error) {
	return AnalyzeConversationWithOptions(conv, AnalyzeOptions{})
}

// AnalyzeConversationWithOptions 先复制并稳定排序，不修改 Loader 返回的切片。
func AnalyzeConversationWithOptions(conv *loader.Conversation, opts AnalyzeOptions) (*Stats, error) {
	if conv == nil || len(conv.Messages) == 0 {
		return nil, fmt.Errorf("没有消息可分析")
	}
	opts, err := normalizeOptions(opts)
	if err != nil {
		return nil, err
	}
	messages := append([]loader.Message(nil), conv.Messages...)
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].CreateTime < messages[j].CreateTime })

	result := &Stats{
		Total:        len(messages),
		ActiveDays:   make(map[string]int),
		MsgTypes:     make(map[string]int),
		MsgPerHour:   make([]int, 24),
		Timezone:     timezoneLabel(opts.Location, messages[0].CreateTime),
		Relationship: RelationshipStats{SessionGapSeconds: int64(opts.SessionGap / time.Second)},
	}
	result.AvgLength = float64(totalCharCount(messages)) / float64(result.Total)
	firstByDay := make(map[string]loader.Message)
	monthly := make(map[string]*MonthlyStats)
	monthlyDays := make(map[string]map[string]struct{})

	for _, msg := range messages {
		if msg.IsSender {
			result.SentTotal++
		} else {
			result.ReceivedTotal++
		}
		t := time.Unix(msg.CreateTime, 0).In(opts.Location)
		result.MsgPerHour[t.Hour()]++
		date, month := t.Format("2006-01-02"), t.Format("2006-01")
		result.ActiveDays[date]++
		if _, exists := firstByDay[date]; !exists {
			firstByDay[date] = msg
		}
		entry := monthly[month]
		if entry == nil {
			entry = &MonthlyStats{Month: month}
			monthly[month] = entry
			monthlyDays[month] = make(map[string]struct{})
		}
		entry.Total++
		if msg.IsSender {
			entry.Sent++
		} else {
			entry.Received++
		}
		monthlyDays[month][date] = struct{}{}
		if msg.TypeName != "" {
			result.MsgTypes[msg.TypeName]++
		}
	}

	result.SentRatio = percentage(result.SentTotal, result.Total)
	for _, first := range firstByDay {
		if first.IsSender {
			result.FirstMessageCount++
		}
	}
	result.ActiveDayCount = len(result.ActiveDays)
	result.MsgPerDay = ratio(result.Total, result.ActiveDayCount)
	result.MsgPerActiveDay = result.MsgPerDay
	result.FirstMessageRatio = percentage(result.FirstMessageCount, result.ActiveDayCount)
	result.CalendarDays = calendarDaySpan(messages[0].CreateTime, messages[len(messages)-1].CreateTime, opts.Location)
	result.MsgPerCalendarDay = ratio(result.Total, result.CalendarDays)
	result.TopMessages = getTopMessages(messages, 10)

	result.Relationship = analyzeRelationships(messages, opts)
	result.Relationship.LongestActiveStreak = longestActiveStreak(result.ActiveDays, opts.Location)
	for _, session := range buildSessions(messages, opts.SessionGap) {
		month := time.Unix(session[0].CreateTime, 0).In(opts.Location).Format("2006-01")
		monthly[month].Sessions++
	}
	months := make([]string, 0, len(monthly))
	for month := range monthly {
		months = append(months, month)
	}
	sort.Strings(months)
	for _, month := range months {
		entry := monthly[month]
		entry.ActiveDays = len(monthlyDays[month])
		result.Relationship.Monthly = append(result.Relationship.Monthly, *entry)
	}
	return result, nil
}

func normalizeOptions(opts AnalyzeOptions) (AnalyzeOptions, error) {
	if opts.SessionGap == 0 {
		opts.SessionGap = DefaultSessionGap
	}
	if opts.SessionGap < 0 {
		return opts, fmt.Errorf("会话间隔必须大于 0")
	}
	if opts.Location == nil {
		opts.Location = time.Local
	}
	return opts, nil
}

func analyzeRelationships(messages []loader.Message, opts AnalyzeOptions) RelationshipStats {
	rel := RelationshipStats{SessionGapSeconds: int64(opts.SessionGap / time.Second)}
	var mine, theirs []float64
	for _, session := range buildSessions(messages, opts.SessionGap) {
		rel.TotalSessions++
		rel.AvgMessagesPerSession += float64(len(session))
		if len(session) > rel.LongestSessionMessages {
			rel.LongestSessionMessages = len(session)
		}
		duration := session[len(session)-1].CreateTime - session[0].CreateTime
		if duration > rel.LongestSessionSeconds {
			rel.LongestSessionSeconds = duration
		}
		if session[0].IsSender {
			rel.StartedByMe++
		} else {
			rel.StartedByThem++
		}
		if session[len(session)-1].IsSender {
			rel.EndedByMe++
		} else {
			rel.EndedByThem++
		}

		previousTurnLast := session[0]
		for i := 1; i < len(session); i++ {
			msg := session[i]
			if msg.IsSender == previousTurnLast.IsSender {
				previousTurnLast = msg
				continue
			}
			seconds := float64(msg.CreateTime - previousTurnLast.CreateTime)
			if msg.IsSender {
				mine = append(mine, seconds)
			} else {
				theirs = append(theirs, seconds)
			}
			previousTurnLast = msg
		}
	}
	if rel.TotalSessions > 0 {
		rel.AvgMessagesPerSession /= float64(rel.TotalSessions)
	}
	rel.StartedByMeRatio = percentage(rel.StartedByMe, rel.TotalSessions)
	rel.StartedByThemRatio = percentage(rel.StartedByThem, rel.TotalSessions)
	rel.EndedByMeRatio = percentage(rel.EndedByMe, rel.TotalSessions)
	rel.EndedByThemRatio = percentage(rel.EndedByThem, rel.TotalSessions)
	rel.MyResponses, rel.TheirResponses = responseStats(mine), responseStats(theirs)
	return rel
}

func buildSessions(messages []loader.Message, gap time.Duration) [][]loader.Message {
	if len(messages) == 0 {
		return nil
	}
	sessions, start := make([][]loader.Message, 0, 1), 0
	for i := 1; i < len(messages); i++ {
		if time.Duration(messages[i].CreateTime-messages[i-1].CreateTime)*time.Second > gap {
			sessions = append(sessions, messages[start:i])
			start = i
		}
	}
	return append(sessions, messages[start:])
}

func responseStats(samples []float64) ResponseStats {
	if len(samples) == 0 {
		return ResponseStats{}
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	middle, median := len(sorted)/2, float64(0)
	median = sorted[middle]
	if len(sorted)%2 == 0 {
		median = (sorted[middle-1] + sorted[middle]) / 2
	}
	p90Index := int(math.Ceil(0.9*float64(len(sorted)))) - 1
	return ResponseStats{Count: len(sorted), MedianSeconds: median, P90Seconds: sorted[p90Index]}
}

func calendarDaySpan(firstUnix, lastUnix int64, loc *time.Location) int {
	first, last := time.Unix(firstUnix, 0).In(loc), time.Unix(lastUnix, 0).In(loc)
	firstDay := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, loc)
	lastDay := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, loc)
	days := 1
	for day := firstDay; day.Before(lastDay); day = day.AddDate(0, 0, 1) {
		days++
	}
	return days
}

func longestActiveStreak(activeDays map[string]int, loc *time.Location) int {
	if len(activeDays) == 0 {
		return 0
	}
	dates := make([]time.Time, 0, len(activeDays))
	for date := range activeDays {
		parsed, err := time.ParseInLocation("2006-01-02", date, loc)
		if err == nil {
			dates = append(dates, parsed)
		}
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	longest, current := 1, 1
	for i := 1; i < len(dates); i++ {
		if dates[i-1].AddDate(0, 0, 1).Equal(dates[i]) {
			current++
		} else {
			current = 1
		}
		if current > longest {
			longest = current
		}
	}
	return longest
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func percentage(numerator, denominator int) float64 { return ratio(numerator, denominator) * 100 }

func timezoneLabel(loc *time.Location, unix int64) string {
	if loc.String() != "Local" {
		return loc.String()
	}
	return time.Unix(unix, 0).In(loc).Format("MST (Z07:00)")
}

// Print 保留作为 v0.1 的终端可读 API。
func (s *Stats) Print(conv *loader.Conversation) {
	fmt.Printf("\n📊 聊天记录统计 (%s)\n", conv.Talker.DisplayName())
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("%20s: %8d 条\n", "总消息数", s.Total)
	fmt.Printf("%20s: %8.2f 字符\n", "平均每条长度", s.AvgLength)
	fmt.Printf("%20s: %8.2f 条/天\n\n", "日均消息数", s.MsgPerDay)
	fmt.Printf("%20s: %8d (%.1f%%)\n", "我发的消息", s.SentTotal, s.SentRatio)
	fmt.Printf("%20s: %8d (%.1f%%)\n", "对方发的", s.ReceivedTotal, 100-s.SentRatio)
	fmt.Printf("%20s: %8d (%.1f%%)\n\n", "我先开口", s.FirstMessageCount, s.FirstMessageRatio)

	fmt.Println("⏰ 活跃时段分布:")
	var peakHours []int
	for hour := 0; hour < 24; hour++ {
		if s.MsgPerHour[hour] > 0 {
			peakHours = append(peakHours, hour)
		}
	}
	sort.SliceStable(peakHours, func(i, j int) bool {
		if s.MsgPerHour[peakHours[i]] == s.MsgPerHour[peakHours[j]] {
			return peakHours[i] < peakHours[j]
		}
		return s.MsgPerHour[peakHours[i]] > s.MsgPerHour[peakHours[j]]
	})
	if len(peakHours) > 5 {
		peakHours = peakHours[:5]
	}
	for _, hour := range peakHours {
		count := s.MsgPerHour[hour]
		bar := strings.Repeat("■", count*50/s.MsgPerHour[peakHours[0]])
		fmt.Printf("  %d点:%02d → %d条 %s\n", hour, hour+1, count, bar)
	}
	fmt.Println("\n💬 消息类型分布:")
	var types []string
	for messageType := range s.MsgTypes {
		types = append(types, messageType)
	}
	sort.Slice(types, func(i, j int) bool {
		if s.MsgTypes[types[i]] == s.MsgTypes[types[j]] {
			return types[i] < types[j]
		}
		return s.MsgTypes[types[i]] > s.MsgTypes[types[j]]
	})
	if len(types) > 5 {
		types = types[:5]
	}
	for _, messageType := range types {
		count := s.MsgTypes[messageType]
		fmt.Printf("  %-10s: %8d (%.1f%%)\n", messageType, count, percentage(count, s.Total))
	}
}

func totalCharCount(msgs []loader.Message) int {
	count := 0
	for _, message := range msgs {
		count += utf8.RuneCountInString(message.Content)
	}
	return count
}

func getTopMessages(msgs []loader.Message, topN int) []MessageInfo {
	var infos []MessageInfo
	for _, msg := range msgs {
		if msg.TypeName == "text" || msg.TypeName == "" {
			infos = append(infos, MessageInfo{Content: msg.Content, Length: utf8.RuneCountInString(msg.Content), IsSender: msg.IsSender, CreateTime: msg.CreateTime})
		}
	}
	sort.SliceStable(infos, func(i, j int) bool { return infos[i].Length > infos[j].Length })
	if len(infos) > topN {
		return infos[:topN]
	}
	return infos
}

func (s *Stats) GetMostActiveTime() []string {
	var hours []string
	max := 0
	for hour, count := range s.MsgPerHour {
		if count > max {
			max, hours = count, []string{fmt.Sprintf("%d点", hour)}
		} else if count == max && count > 0 {
			hours = append(hours, fmt.Sprintf("%d点", hour))
		}
	}
	return hours
}

func (s *Stats) GetActiveDateRange() (string, string) {
	if len(s.ActiveDays) == 0 {
		return "", ""
	}
	dates := make([]string, 0, len(s.ActiveDays))
	for date := range s.ActiveDays {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	return dates[0], dates[len(dates)-1]
}
