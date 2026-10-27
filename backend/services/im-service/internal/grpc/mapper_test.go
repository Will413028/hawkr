package grpc

import (
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
)

// B-135 compliance: the gRPC read-path mapper must never expose the original text
// of a moderation-hidden or recalled message. The row is kept (seq continuity);
// only Content is replaced with a neutral placeholder.
func TestToPbChatMessage_TombstonesHiddenAndDeleted(t *testing.T) {
	const secret = "原文機密內容"
	ts := time.Now()

	visible := toPbChatMessage(gen.Message{ID: 1, SenderID: 1, ReceiverID: 2, SeqID: 5, Content: secret})
	if visible.Content != secret {
		t.Errorf("visible: Content = %q, want original %q", visible.Content, secret)
	}

	hidden := toPbChatMessage(gen.Message{ID: 2, SenderID: 1, ReceiverID: 2, SeqID: 6, Content: secret, HiddenAt: &ts})
	if hidden.Content == secret {
		t.Errorf("hidden message leaked original content: %q", hidden.Content)
	}
	if hidden.SeqId != "6" {
		t.Errorf("hidden: SeqId = %q, want 6 (row must stay in sequence)", hidden.SeqId)
	}

	deleted := toPbChatMessage(gen.Message{ID: 3, SenderID: 1, ReceiverID: 2, SeqID: 7, Content: secret, DeletedAt: &ts})
	if deleted.Content == secret {
		t.Errorf("deleted message leaked original content: %q", deleted.Content)
	}
}

// 49 §6.6 im/S4：圖片訊息（type=2）content 為裸 objectKey，服務讀取路徑不組 URL
// （顯示 URL 由 Gateway REST／前端 WS 組）；被 tombstone 的圖片仍回佔位字。
func TestToPbChatMessage_ImageContentIsBareKey(t *testing.T) {
	const key = "public/chat/1/20260717/abc.jpg"

	img := toPbChatMessage(gen.Message{ID: 4, SenderID: 1, ReceiverID: 2, SeqID: 8, Type: 2, Content: key})
	if img.Content != key {
		t.Errorf("image: Content = %q, want bare objectKey %q", img.Content, key)
	}

	ts := time.Now()
	hiddenImg := toPbChatMessage(gen.Message{ID: 5, SenderID: 1, ReceiverID: 2, SeqID: 9, Type: 2, Content: key, HiddenAt: &ts})
	if hiddenImg.Content == key {
		t.Errorf("hidden image must render placeholder, got %q", hiddenImg.Content)
	}
}

func TestMessageTypeToString(t *testing.T) {
	tests := []struct {
		want  string
		input pb.MessageType
	}{
		{"text", pb.MessageType_MESSAGE_TYPE_TEXT},
		{"image", pb.MessageType_MESSAGE_TYPE_IMAGE},
		{"message", pb.MessageType_MESSAGE_TYPE_UNSPECIFIED},
		{"message", pb.MessageType(99)},
	}
	for _, tt := range tests {
		got := messageTypeToString(tt.input)
		if got != tt.want {
			t.Errorf("messageTypeToString(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
