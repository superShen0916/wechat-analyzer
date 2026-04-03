// Package stats 聊天记录统计分析
package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/superShen0916/wechat-analyzer/internal/loader"
)

// Stats 统计结果
type Stats struct {
	Total         int    `json:"total_messages"`    // 总消息数
	AvgLength     float64 `json:"avg_msg_length"` // 平均消息长度

	MsgPerDay     float64 `json:"msgs_per_day"`    // 日均消息数
	MsgPerHour    []int   `json:"msgs_per_hour"`    // 每小时消息数

	SentTotal     int     `json:"sent_total"`      // 自己发的
	ReceivedTotal int     `json:"received_total"` // 收到的
	SentRatio     float64 `json:"sent_ratio"`      // 我发的比例

	FirstMessageCount int `json:"first_message_count"` // 我先开口的次数
	FirstMessageRatio float64 `json:"first_message_ratio"`

	ActiveDays   map[string]int `json:"active_days"`    // 每天消息数
	TopMessages  []MessageInfo `json:"top_messages"` // 消息长度排名

	MsgTypes     map[string]int `json:"msg_types"`    // 各类型消息数
}

// MessageInfo 单条消息信息（用于排名）
type MessageInfo struct {
	Content     string    `json:"content"`
	Length      int       `json:"length"`
	IsSender    bool      `json:"is_sender"`
	CreateTime  int64     `json:"create_time"`
}

// AnalyzeConversation 分析一段对话的统计数据
func AnalyzeConversation(conv *loader.Conversation) (*Stats, error) {
	if conv.Total == 0 {
		return nil, fmt.Errorf("没有消息可分析")
	}

	stats := &Stats{
		ActiveDays: make(map[string]int),
		MsgTypes:   make(map[string]int),
		MsgPerHour: make([]int, 24),
	}

	// 总览
	stats.Total = len(conv.Messages)
	stats.AvgLength = float64(totalCharCount(conv.Messages)) / float64(len(conv.Messages))

	// 发送方统计
	firstCount := 0
	for i, msg := range conv.Messages {
		if msg.IsSender {
			stats.SentTotal++
		} else {
			stats.ReceivedTotal++
		}

		// 先开口统计
		if msg.IsSender && i == 0 {
			firstCount++
		}
		if !msg.IsSender && i > 0 && !conv.Messages[i-1].IsSender {
			// 连续对方发的，不算
		} else if msg.IsSender && i > 0 && !conv.Messages[i-1].IsSender {
			// 对方上一条，我下一条，算我先开口
			firstCount++
		}

		// 小时分布
		t := time.Unix(msg.CreateTime, 0)
		hour := t.Hour()
		stats.MsgPerHour[hour]++

		// 活跃日期
		dateStr := t.Format("2006-01-02")
		stats.ActiveDays[dateStr]++

		// 消息类型
		if msg.TypeName != "" {
			stats.MsgTypes[msg.TypeName]++
		}
	}

	// 计算比率
	stats.SentRatio = float64(stats.SentTotal) / float64(conv.Total) * 100
	stats.FirstMessageCount = firstCount
	stats.FirstMessageRatio = float64(firstCount) / float64(conv.Total) * 100

	// 日均消息数
	activeDayCount := len(stats.ActiveDays)
	if activeDayCount > 0 {
		stats.MsgPerDay = float64(conv.Total) / float64(activeDayCount)
	}

	// 长消息排名
	stats.TopMessages = getTopMessages(conv.Messages, 10)

	return stats, nil
}

// Print 打印统计结果（终端可读）
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
	// 按消息数排序
	sort.Slice(peakHours, func(i, j int) bool {
		return s.MsgPerHour[peakHours[i]] > s.MsgPerHour[peakHours[j]]
	})
	for _, h := range peakHours[:5] {
		count := s.MsgPerHour[h]
		bar := strings.Repeat("■", count*50/s.MsgPerHour[peakHours[0]])
		fmt.Printf("  %d点:%02d → %d条 %s\n", h, h+1, count, bar)
	}
	fmt.Println("\n💬 消息类型分布:")
	var types []string
	for k := range s.MsgTypes {
		types = append(types, k)
	}
	sort.Slice(types, func(i, j int) bool {
		return s.MsgTypes[types[i]] > s.MsgTypes[types[j]]
	})
	for _, t := range types[:5] {
		count := s.MsgTypes[t]
		p := float64(count) / float64(s.Total) * 100
		fmt.Printf("  %-10s: %8d (%.1f%%)\n", t, count, p)
	}
}

// ── 辅助函数 ──────────────────────────────────────────────────────────────────

func totalCharCount(msgs []loader.Message) int {
	cnt := 0
	for _, m := range msgs {
		cnt += len(m.Content)
	}
	return cnt
}

func getTopMessages(msgs []loader.Message, topN int) []MessageInfo {
	var infos []MessageInfo
	for _, msg := range msgs {
		if msg.TypeName == "text" || msg.TypeName == "" {
			infos = append(infos, MessageInfo{
				Content:     msg.Content,
				Length:      len(msg.Content),
				IsSender:    msg.IsSender,
				CreateTime:  msg.CreateTime,
			})
		}
	}

	// 按长度排序
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Length > infos[j].Length
	})

	if len(infos) > topN {
		return infos[:topN]
	}
	return infos
}

func (s *Stats) GetMostActiveTime() []string {
	var hours []string
	max := 0
	for i, cnt := range s.MsgPerHour {
		if cnt > max {
			max = cnt
			hours = []string{fmt.Sprintf("%d点", i)}
		} else if cnt == max && cnt > 0 {
			hours = append(hours, fmt.Sprintf("%d点", i))
		}
	}
	return hours
}

func (s *Stats) GetActiveDateRange() (string, string) {
	if len(s.ActiveDays) == 0 {
		return "", ""
	}
	dates := make([]string, 0, len(s.ActiveDays))
	for d := range s.ActiveDays {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates[0], dates[len(dates)-1]
}
