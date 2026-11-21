// Package rows 把 store 的列轉成對外型別，隱藏與抹除的訊息在這裡換成墓碑（不帶內容）。
package rows

import (
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
)

// MessageRow 與 store.ListMessagesBeforeRow、store.ListMessagesAfterRow 同欄位同順序，可以直接轉型。
type MessageRow struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	Seq            int64
	SenderKind     string
	MessageType    string
	Content        *string
	ImageAssetID   *string
	ImageObjectKey *string
	CreatedAt      time.Time
	ErasedAt       *time.Time
	Hidden         bool
}

// Message 轉成對外的訊息：隱藏或抹除的不帶內容。
func Message(r MessageRow) model.Message {
	m := model.Message{
		ID: r.ID, ConversationID: r.ConversationID, Seq: r.Seq,
		SenderKind: model.PartyKind(r.SenderKind), Type: model.MessageType(r.MessageType),
		CreatedAt: r.CreatedAt, Hidden: r.Hidden, Erased: r.ErasedAt != nil,
	}
	if m.Hidden || m.Erased {
		return m
	}
	m.Text = r.Content
	if r.ImageAssetID != nil && r.ImageObjectKey != nil {
		m.Image = &model.Image{AssetID: *r.ImageAssetID, ObjectKey: *r.ImageObjectKey}
	}
	return m
}

// Before 轉 ListMessagesBefore 的一批，保留順序。
func Before(in []store.ListMessagesBeforeRow) []model.Message {
	out := make([]model.Message, len(in))
	for i, r := range in {
		out[i] = Message(MessageRow(r))
	}
	return out
}

// After 轉 ListMessagesAfter 的一批，保留順序。
func After(in []store.ListMessagesAfterRow) []model.Message {
	out := make([]model.Message, len(in))
	for i, r := range in {
		out[i] = Message(MessageRow(r))
	}
	return out
}

// PreviewOf 由最後一則訊息推導預覽：文字只給前 MaxPreviewRunes 個字，圖片沒有文字，隱藏或抹除的不給任何內容。
func PreviewOf(senderKind, messageType string, content *string, erasedAt *time.Time, hidden bool) model.Preview {
	p := model.Preview{Type: model.MessageType(messageType), SenderKind: model.PartyKind(senderKind), Hidden: hidden, Erased: erasedAt != nil}
	if hidden || p.Erased || content == nil || p.Type != model.TypeText {
		return p
	}
	text := truncate(*content, model.MaxPreviewRunes)
	p.Text = &text
	return p
}

func truncate(s string, n int) string {
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

// Hide 轉成對外的隱藏紀錄。
func Hide(r store.ImHide, created bool) model.Hide {
	h := model.Hide{ID: r.ID, MessageID: r.MessageID, Category: r.ReasonCategory, Text: r.ReasonText, AdminID: r.AdminID, HiddenAt: r.HiddenAt, RestoredAt: r.RestoredAt, Created: created}
	if r.SourceRef != nil {
		h.SourceRef = *r.SourceRef
	}
	return h
}
