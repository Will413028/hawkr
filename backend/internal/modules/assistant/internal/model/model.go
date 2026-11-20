package model

import (
	"errors"
	"time"
)

// Topic 是一則訊息要助理處理的主題。零值不是合法主題。
type Topic int

const (
	// TopicOrders 查詢買家的訂單，直接交給訂單 specialist。
	TopicOrders Topic = iota + 1
	// TopicTracking 查詢物流進度，直接交給物流 specialist。
	TopicTracking
	// TopicReturnPolicy 退貨政策：固定文案，不呼叫 LLM、不建 session。
	TopicReturnPolicy
	// TopicChat 一般問答，交給 coordinator。
	TopicChat
)

// Valid 回主題是不是上面四個之一。
func (t Topic) Valid() bool { return t >= TopicOrders && t <= TopicChat }

// Tool 是助理可以使用的工具，對外是 proto 的 AssistantTool。零值不是合法的工具。
type Tool int

const (
	// ToolGetMyOrders 查詢買家的訂單清單。
	ToolGetMyOrders Tool = iota + 1
	// ToolGetBuyerOrderDetail 查詢買家的單筆訂單。
	ToolGetBuyerOrderDetail
	// ToolTrackMyPackage 查詢配送中包裹的物流狀態。
	ToolTrackMyPackage
)

// Valid 回工具是不是上面三個之一。
func (t Tool) Valid() bool { return t >= ToolGetMyOrders && t <= ToolTrackMyPackage }

// EventKind 是輪次事件的種類。
type EventKind int

const (
	// EventText 是回覆文字的增量。
	EventText EventKind = iota + 1
	// EventToolResult 是一支 tool 執行結束。
	EventToolResult
	// EventFinished 是這一輪正常結束，之後沒有其他事件。
	EventFinished
	// EventError 是這一輪失敗，之後沒有其他事件。
	EventError
)

// Terminal 回這個種類是不是一輪的最後一個事件。
func (k EventKind) Terminal() bool { return k == EventFinished || k == EventError }

// FailureKind 說明一輪為什麼失敗；映射到對外 ErrorReason 由 handler 決定。
type FailureKind int

const (
	// FailureNoOutput 是失敗時還沒有任何文字送出（或最後一次 tool 回應之後還沒有）。
	FailureNoOutput FailureKind = iota + 1
	// FailureInterrupted 是已送出部分文字之後失敗（含事件數超過每輪上限）。
	FailureInterrupted
)

// Event 是一輪的一個事件。Cursor 是它在這一輪的位置，client 當不透明字串。
type Event struct {
	Cursor string
	Kind   EventKind

	// Text 是 EventText 的文字增量。
	Text string

	// Tool 與 ToolSucceeded 是 EventToolResult 的內容：用了哪支工具、有沒有成功。參數與回傳內容不進事件（A9-D13 修訂）。
	Tool          Tool
	ToolSucceeded bool

	// SessionID 是 EventFinished 所屬的對話；退貨政策沒有對話，是請求帶的值（可為空）。
	SessionID string

	// Failure 與 Message 是 EventError 的內容；Message 是依請求語系的使用者面文字。
	Failure FailureKind
	Message string
}

// 模組層的錯誤，由 internal/httpapi/assistant 對應到 Connect 的 code 與 ErrorReason。
var (
	// ErrInvalidTopic 是主題不是合法值。
	ErrInvalidTopic = errors.New("assistant：主題不合法")
	// ErrMessageRequired 是訊息是空字串（退貨政策除外，它是固定文案）；入口層（handler）先驗必填，這是模組自己的防線。
	ErrMessageRequired = errors.New("assistant：訊息必填")
	// ErrSessionNotFound 是帶了 session ID 但該買家沒有這個對話。
	ErrSessionNotFound = errors.New("assistant：找不到對話")
	// ErrSessionCreate 是建立對話失敗。
	ErrSessionCreate = errors.New("assistant：建立對話失敗")
	// ErrTurnInProgress 是同一個對話已有一輪進行中（A9-D14）；對外是 FAILED_PRECONDITION，reason ASSISTANT_TURN_IN_PROGRESS。
	ErrTurnInProgress = errors.New("assistant：這個對話已有一輪進行中")
	// ErrTurnNotFound 是輪次 ID 不存在、已過保留期、單體重啟過，或不屬於呼叫的買家。
	ErrTurnNotFound = errors.New("assistant：找不到這一輪")
	// ErrCursorAhead 是 cursor 超前於最新事件，或不是伺服器發出的值；對外 REPLAY_CURSOR_AHEAD。
	ErrCursorAhead = errors.New("assistant：cursor 超前")
	// ErrIdempotencyMismatch 是同一個冪等鍵（client_request_id）帶了不同的內容；對外 INVALID_ARGUMENT。
	ErrIdempotencyMismatch = errors.New("assistant：同一個 client_request_id 的內容不同")
	// ErrShuttingDown 是執行器已 Shutdown，不再受理新的一輪。
	ErrShuttingDown = errors.New("assistant：服務關閉中，不受理新的一輪")
	// ErrOuterTx 是在已有 transaction 的 ctx 上送出：輪次會在背景執行並寫 session，不得綁在呼叫者的 transaction 上。
	ErrOuterTx = errors.New("assistant：送出不能開在 transaction 內")
)

// Role 是對話中一則訊息的發言者。
type Role int

const (
	// RoleUser 是買家。
	RoleUser Role = iota + 1
	// RoleAssistant 是助理。
	RoleAssistant
)

// Message 是對話歷史中的一則文字訊息。
type Message struct {
	Role Role
	Text string
	Time time.Time
}

// History 是買家最近一次對話與它的訊息（時間由舊到新）；沒有對話時 SessionID 是空字串。
type History struct {
	SessionID string
	Messages  []Message
}
