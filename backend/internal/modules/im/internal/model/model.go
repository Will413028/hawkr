// Package model 是 im 模組的對外型別、常數、事件與錯誤原因。
package model

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	// MaxTextRunes 是文字訊息的長度上限，以字元（rune）計；DB 的 messages_content_check 同值。
	MaxTextRunes = 4096
	// MaxPreviewRunes 是對話預覽文字的長度上限，以字元計。
	MaxPreviewRunes = 100
	// DefaultPageSize 與 MaxPageSize 是列表的預設與上限（AIP-158）。
	DefaultPageSize    = 20
	MaxPageSize        = 100
	MaxHideReasonRunes = 500
	MaxSourceRefRunes  = 200
)

// PartyKind 是對話的一方：買家或店家。值與 DB 的 CHECK 一致。
type PartyKind string

const (
	PartyBuyer PartyKind = "BUYER"
	PartyStore PartyKind = "STORE"
)

// Valid 回報 k 是不是已知的一方；空字串不是。
func (k PartyKind) Valid() bool { return k == PartyBuyer || k == PartyStore }

// MessageType 是訊息類型。2a 只有文字與圖片；卡片隨 A6-5 的契約加。
type MessageType string

const (
	TypeText  MessageType = "TEXT"
	TypeImage MessageType = "IMAGE"
)

// Party 是呼叫者在對話裡的身分，由 httpapi 從 session 解析；模組不接受 client 帶的寄件人。
// 買家只用 BuyerID；店家要帶模組讀出的 seller.Store（條件已推導）。
type Party struct {
	Kind    PartyKind
	BuyerID int64
	Store   *seller.Store
}

// ID 回傳這一方在對話裡的業務鍵（buyer_id 或 store_id）；身分不完整時回 0。
func (p Party) ID() int64 {
	switch {
	case p.Kind == PartyBuyer:
		return p.BuyerID
	case p.Kind == PartyStore && p.Store != nil:
		return p.Store.StoreID
	}
	return 0
}

// Authorize 確認呼叫者可以讀寫自己這一方的對話：身分要完整；店家要有 CapChatRead（關店後沒有）。
func (p Party) Authorize() error {
	if !p.Kind.Valid() || p.ID() <= 0 {
		return errors.New("im：呼叫者身分不完整")
	}
	if p.Kind == PartyStore {
		return seller.Authorize(*p.Store, seller.CapChatRead)
	}
	return nil
}

// Reason 是 im 的業務原因（Go 端的常數，與 apperr 的分類搭配使用；A6-5 加進 proto 的 ErrorReason）。
type Reason string

const (
	ReasonSelfMessageNotAllowed Reason = "SELF_MESSAGE_NOT_ALLOWED"
	ReasonSellerSuspended       Reason = "SELLER_SUSPENDED"
	ReasonStoreClosed           Reason = "STORE_CLOSED"
	ReasonChatBlocked           Reason = "CHAT_BLOCKED"
	ReasonReceiverDeleted       Reason = "RECEIVER_DELETED"
	ReasonModerationRejected    Reason = "MODERATION_REJECTED"
)

// reasoned 把業務原因附在 apperr 的錯誤上；Unwrap 讓 apperr.KindOf 與 errors.As(*apperr.Error) 照常運作。
type reasoned struct {
	err    *apperr.Error
	reason Reason
}

func (e *reasoned) Error() string { return e.err.Error() }
func (e *reasoned) Unwrap() error { return e.err }

// WithReason 回傳帶業務原因的 err。
func WithReason(err *apperr.Error, r Reason) error { return &reasoned{err: err, reason: r} }

// ReasonOf 回傳 err 帶的業務原因；沒有時是空字串。
func ReasonOf(err error) Reason {
	var r *reasoned
	if errors.As(err, &r) {
		return r.reason
	}
	return ""
}

// Orders 是 im 對訂單的唯一依賴。
type Orders interface {
	// OpenOrderBetween 回報 buyer 與 store 之間是否有進行中的訂單或未結案的售後。查不到時回錯誤，呼叫端一律擋（fail-closed）。
	OpenOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, error)
}

// Image 是圖片訊息的圖片。
type Image struct {
	AssetID   string
	ObjectKey string
}

// Message 是一則訊息。隱藏或抹除的訊息不帶內容（Text、Image 為 nil），Hidden、Erased 標出原因。
type Message struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	Seq            int64
	SenderKind     PartyKind
	Type           MessageType
	Text           *string
	Image          *Image
	CreatedAt      time.Time
	Hidden         bool
	Erased         bool
}

// Preview 是對話列表的最後一則訊息的結構化預覽；顯示用的句子由 client 的呈現層依 enum 產生。
type Preview struct {
	Type       MessageType
	Text       *string
	Hidden     bool
	Erased     bool
	SenderKind PartyKind
}

// Conversation 是對話列表的一列。Unread 是呼叫者這一方的未讀數。
type Conversation struct {
	ID            uuid.UUID
	BuyerID       int64
	StoreID       int64
	LastSeq       int64
	Unread        int64
	LastMessageAt time.Time
	Preview       Preview
}

// Cursor 是對話列表的 keyset 游標：上一頁最後一列的 (LastMessageAt, ID)。
type Cursor struct {
	At time.Time
	ID uuid.UUID
}

// ConversationPage 是一頁對話；Next 為 nil 表示沒有下一頁。不回總數。
type ConversationPage struct {
	Items []Conversation
	Next  *Cursor
}

// MessagePage 是一頁訊息；HasMore 表示在這一頁之外還有訊息。不回總數。
type MessagePage struct {
	Items   []Message
	HasMore bool
}

// SendInput 是送出的輸入。給 ConversationID 時對話以 ID 授權；首則訊息改給 CounterpartID（買家寄時是 store_id，店家寄時是 buyer_id）。
type SendInput struct {
	Sender         Party
	ConversationID uuid.UUID
	CounterpartID  int64
	Type           MessageType
	Text           string
	StagedToken    string
}

// RelayMessage 是 relay 組推播 payload 要的原始資料：不套可見性，Hidden 與 Erased 讓呼叫端決定要不要送。
type RelayMessage struct {
	Message
	BuyerID int64
	StoreID int64
	Preview Preview
}

// HideInput 是隱藏的輸入；Category 是 policy.CommunityAssessable 的值。
type HideInput struct {
	MessageID uuid.UUID
	AdminID   int64
	Category  string
	Text      string
	SourceRef string
}

// RestoreInput 是還原的輸入。
type RestoreInput struct {
	MessageID uuid.UUID
	AdminID   int64
	Reason    string
}

// Hide 是一筆隱藏紀錄；Created 為假表示回的是既有的未還原隱藏（冪等）。
type Hide struct {
	ID         uuid.UUID
	MessageID  uuid.UUID
	Category   string
	Text       string
	SourceRef  string
	AdminID    int64
	HiddenAt   time.Time
	RestoredAt *time.Time
	Created    bool
}

// MessageSent 在訊息寫入成功的 transaction 內發布；relay 與 2b 的 hub 訂閱它。
type MessageSent struct {
	EventID        uuid.UUID
	OccurredAt     time.Time
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	Seq            int64
	SenderKind     PartyKind
	BuyerID        int64
	StoreID        int64
}
