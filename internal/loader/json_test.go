package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestContactDisplayName(t *testing.T) {
	tests := []struct {
		name    string
		contact Contact
		want    string
	}{
		{name: "remark", contact: Contact{Remark: "Remark", NickName: "Nick", UserName: "wxid"}, want: "Remark"},
		{name: "nickname", contact: Contact{NickName: "Nick", UserName: "wxid"}, want: "Nick"},
		{name: "username", contact: Contact{UserName: "wxid"}, want: "wxid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.contact.DisplayName(); got != tt.want {
				t.Fatalf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	tempDir := t.TempDir()
	fileName := filepath.Join(tempDir, "conversation.json")
	data := `{
  "talker": {"nick_name": "Example"},
  "messages": [
    {"local_id": 1, "type_name": "text", "content": "hello"},
    {"local_id": 2, "type_name": "text", "content": "world"}
  ]
}`
	if err := os.WriteFile(fileName, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	conversation, err := LoadFile(fileName)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if conversation.Total != 2 {
		t.Fatalf("Total = %d, want 2", conversation.Total)
	}
	if conversation.SourceFile != fileName {
		t.Fatalf("SourceFile = %q, want %q", conversation.SourceFile, fileName)
	}
}

func TestLoadFileRejectsInvalidJSON(t *testing.T) {
	fileName := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(fileName, []byte(`{"talker":`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFile(fileName); err == nil {
		t.Fatal("LoadFile() error = nil, want malformed JSON error")
	}
}

func TestLoadDirReturnsValidConversationsInNameOrder(t *testing.T) {
	tempDir := t.TempDir()
	files := map[string]string{
		"b.json":     `{"talker":{"nick_name":"B"},"messages":[{"local_id":2}]}`,
		"a.json":     `{"talker":{"nick_name":"A"},"messages":[{"local_id":1}]}`,
		"empty.json": `{"talker":{"nick_name":"Empty"},"messages":[]}`,
		"notes.txt":  "ignored",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(tempDir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	conversations, err := LoadDir(tempDir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	got := []string{conversations[0].Talker.DisplayName(), conversations[1].Talker.DisplayName()}
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("conversation order = %v, want %v", got, want)
	}
}
