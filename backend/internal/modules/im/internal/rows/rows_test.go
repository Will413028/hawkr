package rows

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
)

func ptr[T any](v T) *T { return &v }

func TestMessageTombstones(t *testing.T) {
	now := time.Now()
	base := MessageRow{ID: uuid.UUID{1}, Seq: 2, SenderKind: "BUYER", MessageType: "TEXT", Content: ptr("hi"), CreatedAt: now}
	m := Message(base)
	if m.Text == nil || *m.Text != "hi" || m.Hidden || m.Erased {
		t.Fatalf("可見的文字訊息：%+v", m)
	}
	hidden := base
	hidden.Hidden = true
	if m := Message(hidden); m.Text != nil || !m.Hidden {
		t.Fatalf("隱藏的訊息不帶內容：%+v", m)
	}
	erased := base
	erased.Content, erased.ErasedAt = nil, &now
	if m := Message(erased); m.Text != nil || !m.Erased {
		t.Fatalf("抹除的訊息不帶內容：%+v", m)
	}
	img := MessageRow{MessageType: "IMAGE", ImageAssetID: ptr(uuid.UUID{2}), ImageObjectKey: ptr("k")}
	if m := Message(img); m.Image == nil || m.Image.ObjectKey != "k" {
		t.Fatalf("圖片訊息：%+v", m)
	}
	img.Hidden = true
	if m := Message(img); m.Image != nil {
		t.Fatalf("隱藏的圖片不帶圖：%+v", m)
	}
}

func TestPreviewOf(t *testing.T) {
	now := time.Now()
	if p := PreviewOf("BUYER", "TEXT", ptr("你好"), nil, false); p.Text == nil || *p.Text != "你好" || p.SenderKind != model.PartyBuyer {
		t.Fatalf("文字預覽：%+v", p)
	}
	long := strings.Repeat("字", model.MaxPreviewRunes+5)
	if p := PreviewOf("STORE", "TEXT", &long, nil, false); p.Text == nil || len([]rune(*p.Text)) != model.MaxPreviewRunes {
		t.Fatalf("預覽以字元截到 %d：%+v", model.MaxPreviewRunes, p)
	}
	if p := PreviewOf("BUYER", "IMAGE", nil, nil, false); p.Text != nil || p.Type != model.TypeImage {
		t.Fatalf("圖片沒有文字：%+v", p)
	}
	// ID1：隱藏與抹除的預覽不帶任何內容。
	if p := PreviewOf("BUYER", "TEXT", ptr("秘密"), nil, true); p.Text != nil || !p.Hidden {
		t.Fatalf("隱藏的預覽洩漏內容：%+v", p)
	}
	if p := PreviewOf("BUYER", "TEXT", ptr("秘密"), &now, false); p.Text != nil || !p.Erased {
		t.Fatalf("抹除的預覽洩漏內容：%+v", p)
	}
}
