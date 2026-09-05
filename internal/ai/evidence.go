package ai

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/superShen0916/wechat-analyzer/internal/loader"
	"github.com/superShen0916/wechat-analyzer/internal/stats"
)

const (
	// EvidencePromptVersion identifies the evidence request and response contract.
	EvidencePromptVersion   = "evidence-v1"
	DefaultEvidenceMessages = 80
	DefaultEvidenceChars    = 12000
	MinEvidenceMessages     = 3
	MaxEvidenceMessages     = 500
	MinEvidenceChars        = 500
	MaxEvidenceChars        = 100000
)

// EvidenceOptions controls deterministic local evidence preparation.
type EvidenceOptions struct {
	MaxMessages int
	MaxChars    int
	Location    *time.Location
}

// EvidenceMessage is a locally identified, redacted chat excerpt.
type EvidenceMessage struct {
	ID        string `json:"id"`
	Speaker   string `json:"speaker"`
	Time      string `json:"time"`
	Content   string `json:"content,omitempty"`
	Truncated bool   `json:"truncated"`
}

// SamplingSummary describes the limits and coverage of an evidence sample.
type SamplingSummary struct {
	CandidateMessages int    `json:"candidate_messages"`
	SelectedMessages  int    `json:"selected_messages"`
	CandidateChars    int    `json:"candidate_chars"`
	SelectedChars     int    `json:"selected_chars"`
	MaxMessages       int    `json:"max_messages"`
	MaxChars          int    `json:"max_chars"`
	StartTime         string `json:"start_time,omitempty"`
	EndTime           string `json:"end_time,omitempty"`
	Truncated         bool   `json:"truncated"`
}

// EvidenceStatistics contains only aggregate, non-identifying conversation facts.
type EvidenceStatistics struct {
	TotalMessages    int      `json:"total_messages"`
	SentMessages     int      `json:"sent_messages"`
	ReceivedMessages int      `json:"received_messages"`
	SentRatio        float64  `json:"sent_ratio"`
	ActiveDays       int      `json:"active_days"`
	CalendarDays     int      `json:"calendar_days"`
	Sessions         int      `json:"sessions"`
	ActiveTimes      []string `json:"active_times"`
}

// EvidenceBundle is the exact local data object serialized into an evidence prompt.
type EvidenceBundle struct {
	PromptVersion string             `json:"prompt_version"`
	Messages      []EvidenceMessage  `json:"messages"`
	Sampling      SamplingSummary    `json:"sampling"`
	Redactions    map[string]int     `json:"redactions"`
	Statistics    EvidenceStatistics `json:"statistics"`
}

type evidenceCandidate struct {
	message    loader.Message
	content    string
	redactions map[string]int
}

var (
	urlPattern    = regexp.MustCompile(`https?://[^\s<>"']+`)
	emailPattern  = regexp.MustCompile(`(?i)[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+`)
	idCardPattern = regexp.MustCompile(`[1-9][0-9]{5}(?:18|19|20)[0-9]{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12][0-9]|3[01])[0-9]{3}[0-9Xx]`)
	phonePattern  = regexp.MustCompile(`(?:\+?86[- ]?)?1[3-9][0-9]{9}`)
	wxidPattern   = regexp.MustCompile(`(?i)\bwxid[_:-]?[a-z0-9_-]+\b`)
)

// PrepareEvidence creates a deterministic, redacted evidence bundle without network access.
func PrepareEvidence(conv *loader.Conversation, statistics *stats.Stats, opts EvidenceOptions) (*EvidenceBundle, error) {
	if conv == nil {
		return nil, fmt.Errorf("准备证据失败: 对话不能为空")
	}
	if statistics == nil {
		return nil, fmt.Errorf("准备证据失败: 统计结果不能为空")
	}
	if opts.MaxMessages < MinEvidenceMessages || opts.MaxMessages > MaxEvidenceMessages {
		return nil, fmt.Errorf("证据消息上限必须在 %d 到 %d 之间", MinEvidenceMessages, MaxEvidenceMessages)
	}
	if opts.MaxChars < MinEvidenceChars || opts.MaxChars > MaxEvidenceChars {
		return nil, fmt.Errorf("证据字符上限必须在 %d 到 %d 之间", MinEvidenceChars, MaxEvidenceChars)
	}
	location := opts.Location
	if location == nil {
		location = time.Local
	}

	messages := append([]loader.Message(nil), conv.Messages...)
	sort.SliceStable(messages, func(i, j int) bool {
		return messages[i].CreateTime < messages[j].CreateTime
	})

	candidates := make([]evidenceCandidate, 0, len(messages))
	candidateChars := 0
	for _, message := range messages {
		if message.TypeName != "" && message.TypeName != "text" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		candidateRedactions := newRedactionCounts()
		content = redactEvidenceText(content, candidateRedactions)
		candidateChars += utf8.RuneCountInString(content)
		candidates = append(candidates, evidenceCandidate{message: message, content: content, redactions: candidateRedactions})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("准备证据失败: 没有可用的文本消息")
	}

	indices, perMessageLimit := selectEvidenceIndices(candidates, opts.MaxMessages, opts.MaxChars, candidateChars)
	indices = ensureSpeakerCoverage(candidates, indices)
	sort.Ints(indices)

	selected := make([]EvidenceMessage, 0, len(indices))
	selectedChars := 0
	contentTruncated := false
	redactions := newRedactionCounts()
	for _, index := range indices {
		candidate := candidates[index]
		for key, count := range candidate.redactions {
			redactions[key] += count
		}
		content, truncated := truncateRunes(candidate.content, perMessageLimit)
		contentTruncated = contentTruncated || truncated
		selectedChars += utf8.RuneCountInString(content)
		speaker := "them"
		if candidate.message.IsSender {
			speaker = "me"
		}
		selected = append(selected, EvidenceMessage{
			ID:        fmt.Sprintf("m%04d", len(selected)+1),
			Speaker:   speaker,
			Time:      time.Unix(candidate.message.CreateTime, 0).In(location).Format(time.RFC3339),
			Content:   content,
			Truncated: truncated,
		})
	}

	sampling := SamplingSummary{
		CandidateMessages: len(candidates),
		SelectedMessages:  len(selected),
		CandidateChars:    candidateChars,
		SelectedChars:     selectedChars,
		MaxMessages:       opts.MaxMessages,
		MaxChars:          opts.MaxChars,
		Truncated:         len(selected) < len(candidates) || contentTruncated,
	}
	if len(selected) > 0 {
		sampling.StartTime = selected[0].Time
		sampling.EndTime = selected[len(selected)-1].Time
	}

	return &EvidenceBundle{
		PromptVersion: EvidencePromptVersion,
		Messages:      selected,
		Sampling:      sampling,
		Redactions:    redactions,
		Statistics: EvidenceStatistics{
			TotalMessages:    statistics.Total,
			SentMessages:     statistics.SentTotal,
			ReceivedMessages: statistics.ReceivedTotal,
			SentRatio:        statistics.SentRatio,
			ActiveDays:       statistics.ActiveDayCount,
			CalendarDays:     statistics.CalendarDays,
			Sessions:         statistics.Relationship.TotalSessions,
			ActiveTimes:      append([]string(nil), statistics.GetMostActiveTime()...),
		},
	}, nil
}

func newRedactionCounts() map[string]int {
	return map[string]int{
		"email":     0,
		"id_card":   0,
		"phone":     0,
		"url_query": 0,
		"wxid":      0,
	}
}

func redactEvidenceText(content string, counts map[string]int) string {
	content = urlPattern.ReplaceAllStringFunc(content, func(value string) string {
		query := strings.IndexByte(value, '?')
		if query < 0 {
			return value
		}
		counts["url_query"]++
		// The broad URL matcher stops at whitespace. Treat common CJK prose
		// punctuation as the end of the query too, so a pasted URL followed directly
		// by a Chinese sentence does not cause that sentence to be discarded.
		queryEnd := len(value)
		if offset := strings.IndexByte(value[query+1:], '#'); offset >= 0 {
			queryEnd = query + 1 + offset
		}
		if offset := strings.IndexAny(value[query+1:queryEnd], "，。；：！？）】》"); offset >= 0 {
			queryEnd = query + 1 + offset
		}
		return value[:query+1] + "[URL_QUERY]" + value[queryEnd:]
	})
	content = replaceAndCount(content, emailPattern, "[EMAIL]", counts, "email")
	content = replaceAndCount(content, idCardPattern, "[ID_CARD]", counts, "id_card")
	content = replaceAndCount(content, phonePattern, "[PHONE]", counts, "phone")
	content = replaceAndCount(content, wxidPattern, "[WXID]", counts, "wxid")
	return content
}

func replaceAndCount(content string, pattern *regexp.Regexp, replacement string, counts map[string]int, key string) string {
	return pattern.ReplaceAllStringFunc(content, func(string) string {
		counts[key]++
		return replacement
	})
}

func selectEvidenceIndices(candidates []evidenceCandidate, maxMessages, maxChars, totalChars int) ([]int, int) {
	if len(candidates) <= maxMessages && totalChars <= maxChars {
		indices := make([]int, len(candidates))
		for i := range candidates {
			indices[i] = i
		}
		return indices, maxChars
	}

	k := min(len(candidates), maxMessages, maxChars/40)
	if k < 1 {
		k = 1
	}
	perMessageLimit := max(40, maxChars/k)
	indices := make([]int, k)
	if k == 1 {
		indices[0] = 0
		return indices, min(perMessageLimit, maxChars)
	}
	for i := range k {
		indices[i] = (i*(len(candidates)-1) + (k-1)/2) / (k - 1)
	}
	return indices, perMessageLimit
}

func ensureSpeakerCoverage(candidates []evidenceCandidate, indices []int) []int {
	if len(indices) < 2 {
		return indices
	}
	hasCandidate := map[bool]bool{}
	hasSelected := map[bool]bool{}
	selectedIndices := make(map[int]bool, len(indices))
	for _, candidate := range candidates {
		hasCandidate[candidate.message.IsSender] = true
	}
	for _, index := range indices {
		hasSelected[candidates[index].message.IsSender] = true
		selectedIndices[index] = true
	}
	for _, speaker := range []bool{true, false} {
		if !hasCandidate[speaker] || hasSelected[speaker] {
			continue
		}
		target := nearestSpeakerIndex(candidates, selectedIndices, speaker)
		replaceAt := replaceableSelection(candidates, indices, speaker)
		delete(selectedIndices, indices[replaceAt])
		indices[replaceAt] = target
		selectedIndices[target] = true
		hasSelected[speaker] = true
	}
	return indices
}

func nearestSpeakerIndex(candidates []evidenceCandidate, selected map[int]bool, speaker bool) int {
	middleTwice := len(candidates) - 1
	best, bestDistance := -1, int(^uint(0)>>1)
	for index, candidate := range candidates {
		if candidate.message.IsSender != speaker || selected[index] {
			continue
		}
		distance := abs(2*index - middleTwice)
		if distance < bestDistance {
			best, bestDistance = index, distance
		}
	}
	return best
}

func replaceableSelection(candidates []evidenceCandidate, indices []int, missingSpeaker bool) int {
	best, bestDistance := -1, int(^uint(0)>>1)
	middleTwice := len(candidates) - 1
	for position, index := range indices {
		if candidates[index].message.IsSender == missingSpeaker {
			continue
		}
		if len(indices) > 2 && (position == 0 || position == len(indices)-1) {
			continue
		}
		distance := abs(2*index - middleTwice)
		if distance < bestDistance {
			best, bestDistance = position, distance
		}
	}
	if best >= 0 {
		return best
	}
	return len(indices) - 1
}

func truncateRunes(content string, limit int) (string, bool) {
	if utf8.RuneCountInString(content) <= limit {
		return content, false
	}
	runes := []rune(content)
	if limit == 1 {
		return "…", true
	}
	return string(runes[:limit-1]) + "…", true
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
