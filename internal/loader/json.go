// Package loader 负责加载 wechat-export 导出的 JSON 文件
package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Message 消息（与 wechat-export 的 JSON 格式对应）
type Message struct {
	LocalID        int64  `json:"local_id"`
	MsgSvrID       string `json:"msg_svr_id"`
	Type           int    `json:"type"`
	TypeName       string `json:"type_name"`
	IsSender       bool   `json:"is_sender"`
	CreateTime     int64  `json:"create_time"`
	Talker         string `json:"talker"`
	Content        string `json:"content"`
	DisplayContent string `json:"display_content"`
}

// Contact 联系人
type Contact struct {
	UserName string `json:"user_name"`
	NickName string `json:"nick_name"`
	Alias    string `json:"alias"`
	Remark   string `json:"remark"`
	IsGroup  bool   `json:"is_group"`
}

// Conversation 一段对话
type Conversation struct {
	Talker     Contact   `json:"talker"`
	Total      int       `json:"total"`
	ExportedAt string    `json:"exported_at"`
	Messages   []Message `json:"messages"`
	SourceFile string    `json:"-"` // 来源文件路径
}

// DisplayName 返回联系人显示名称
func (c Contact) DisplayName() string {
	if c.Remark != "" {
		return c.Remark
	}
	if c.NickName != "" {
		return c.NickName
	}
	return c.UserName
}

// LoadFile 加载单个 JSON 文件
func LoadFile(path string) (*Conversation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	conv.SourceFile = path
	if conv.Total == 0 {
		conv.Total = len(conv.Messages)
	}
	return &conv, nil
}

// LoadDir 加载目录中所有 JSON 文件
func LoadDir(dir string) ([]*Conversation, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var convs []*Conversation
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		conv, err := LoadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			fmt.Printf("  ⚠️  跳过 %s: %v\n", e.Name(), err)
			continue
		}
		if len(conv.Messages) > 0 {
			convs = append(convs, conv)
		}
	}

	if len(convs) == 0 {
		return nil, fmt.Errorf("目录 %s 中没有找到有效的聊天记录 JSON 文件", dir)
	}
	return convs, nil
}
