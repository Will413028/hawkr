package live

import (
	"errors"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
)

// EventKind 是串流事件的種類。
type EventKind int

const (
	// EventReady 在補洞完成後交出一次，帶目前的總未讀。
	EventReady EventKind = iota + 1
	// EventMessage 是新訊息，或同 seq 的可見度改變（Updated）。
	EventMessage
	// EventUnread 帶呼叫者這一方的總未讀。
	EventUnread
	// EventPeerRead 是對方的已讀游標前進。
	EventPeerRead
	// EventResync 要求 client 重新取列表與缺口：ConversationID 為對話，All 為全部。
	EventResync
)

// Event 是串流上的一個業務事件。依 Kind 讀對應的欄位。
type Event struct {
	Kind EventKind
	// Unread：EventReady、EventUnread。
	Unread int64
	// Message、Updated：EventMessage；Updated 為真表示同 seq 的可見度改變（墓碑或還原後的內容），client 以 seq 取代。
	Message model.Message
	Updated bool
	// ConversationID：EventPeerRead、EventResync（All 為假時）。
	ConversationID uuid.UUID
	// PeerReadSeq：EventPeerRead 的對方游標。
	PeerReadSeq int64
	// All：EventResync 針對全部對話。
	All bool
}

// EndReason 是串流由模組結束的業務原因。
type EndReason string

const (
	// EndShutdown 是 Run 的 ctx 結束，或本進程的存活標記延長連續失敗。
	EndShutdown EndReason = "SHUTDOWN"
	// EndLostEligibility 是 Revalidate 發現呼叫者已失去讀取資格（關店）。
	EndLostEligibility EndReason = "LOST_ELIGIBILITY"
)

// Ended 是串流結束的錯誤；Next 回它之後訂閱已經關閉。
type Ended struct{ Reason EndReason }

func (e *Ended) Error() string { return "im：串流結束：" + string(e.Reason) }

// ErrClosed 是呼叫者自己 Close（或 Subscribe 的 ctx 取消）之後 Next 的回應。
var ErrClosed = errors.New("im：訂閱已關閉")
