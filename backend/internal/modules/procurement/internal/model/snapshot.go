package model

import (
	"time"

	"uuid"
)

// RuntimeSnapshot 是重建一個對話畫面需要的全部事實，對應 GetRuntimeSnapshotResponse（A9-D30(4)）。
// 狀態、角色、run 類型是 proto enum 值名去掉前綴的代碼（例如 CLARIFYING、USER、CLARIFICATION_RESUME）；轉成 proto enum 是傳輸層的事。
type RuntimeSnapshot struct {
	ConversationID uuid.UUID
	RequestID      uuid.UUID
	Status         string
	// Title 是對話標題；買家沒改過、品項未知時是空字串（client 顯示預設字樣）。
	Title    string
	Messages []ConversationMessage
	// Runs 是目前需求的 run，新到舊，最多 RuntimeSnapshotMaxRuns 筆。
	Runs []RunSummary
	// ActiveRunID 是進行中（QUEUED、RUNNING、CANCEL_REQUESTED）的 run；沒有時為 nil。
	ActiveRunID      *uuid.UUID
	OpenConfirmation *OpenConfirmation
	PublishedRfq     *PublishedRfq
}

// RuntimeSnapshotMaxRuns 是 RuntimeSnapshot.Runs 的上限（A9-D30(4)，不分頁）。
const RuntimeSnapshotMaxRuns = 20

// ConversationMessage 是對話裡的一則訊息。買家的訊息 ID 是該輪的 client_request_id，助理的訊息 ID 是輪次 ID（與 turn_result 的 message_id 相同）。
// 回答中斷的一輪，買家的訊息是答案：選項的標籤或打的文字。Steps 沒有填（伺服器不產生步驟，A9-D24(2)）。
type ConversationMessage struct {
	ID               uuid.UUID
	Role             string
	Text             string
	RecordedThisTurn []RecordedField
	CreateTime       time.Time
}

// RecordedField 是這一輪記下的欄位。
type RecordedField struct {
	Key   SlotKey
	Label string
	Value string
}

// RunSummary 是一個 run 的摘要。LastCursor 是最新事件的 cursor，還沒有事件時是空字串。
type RunSummary struct {
	RunID      uuid.UUID
	RunType    string
	Status     string
	LastCursor string
	Terminal   bool
}

// OpenConfirmation 是待買家回答的中斷。Processing 為真代表已有回答它的 run 在進行中（A9-D26(5)、A9-D30(3)）。
type OpenConfirmation struct {
	InterruptID uuid.UUID
	Kind        string
	Question    ClarifyQuestion
	Processing  bool
}

// ClarifyQuestion 是待回答的問題，讓重新載入之後能還原同一個畫面。
type ClarifyQuestion struct {
	SlotKey       SlotKey
	Prompt        string
	Options       []QuestionOption
	UnitOptions   []QuestionOption
	AllowFreeText bool
	CanSkip       bool
}

// QuestionOption 是問題的一個選項。
type QuestionOption struct {
	OptionID string
	Label    string
}

// PublishedRfq 是需求最新一次的發布結果。RfqSnapshotHash 是快照的 hash，由發布的寫入者（A9-3c）定義如何重算；目前沒有寫入者也沒有重算函式，所以不填。
type PublishedRfq struct {
	PublicationID          uuid.UUID
	Version                int32
	PublishTime            time.Time
	RecipientCount         int32
	QueuedCount            int32
	ConfirmedRfqSnapshotID uuid.UUID
	RfqSnapshotHash        string
}
