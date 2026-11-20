package model

import "errors"

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
	// FailureInterrupted 是已送出部分文字之後失敗。
	FailureInterrupted
)

// Event 是一輪的一個事件。Cursor 是它在這一輪的位置，client 當不透明字串。
type Event struct {
	Cursor string
	Kind   EventKind

	// Text 是 EventText 的文字增量。
	Text string

	// ToolName、ToolFailed、ToolArgs 是 EventToolResult 的內容；ToolArgs 是 tool 解析後的參數，沒有參數時是 nil。
	ToolName   string
	ToolFailed bool
	ToolArgs   map[string]any

	// SessionID 是 EventFinished 所屬的對話；退貨政策沒有對話，是請求帶的值（可為空）。
	SessionID string

	// Failure 與 Message 是 EventError 的內容；Message 是依請求語系的使用者面文字。
	Failure FailureKind
	Message string
}

// 模組層的錯誤，由 A9-2c 的 handler 對應到 Connect 的 code 與 ErrorReason。
var (
	// ErrInvalidTopic 是主題不是合法值。
	ErrInvalidTopic = errors.New("assistant：主題不合法")
	// ErrMessageRequired 是訊息是空字串，而且該主題沒有預設訊息。
	ErrMessageRequired = errors.New("assistant：訊息必填")
	// ErrSessionNotFound 是帶了 session ID 但該買家沒有這個對話。
	ErrSessionNotFound = errors.New("assistant：找不到對話")
	// ErrSessionCreate 是建立對話失敗。
	ErrSessionCreate = errors.New("assistant：建立對話失敗")
	// ErrTurnInProgress 是同一個對話已有一輪進行中（A9-D14）；對外是 FAILED_PRECONDITION，reason ASSISTANT_TURN_IN_PROGRESS。
	ErrTurnInProgress = errors.New("assistant：這個對話已有一輪進行中")
	// ErrTurnNotFound 是輪次 ID 不存在、已過保留期、單體重啟過，或不屬於呼叫的買家。
	ErrTurnNotFound = errors.New("assistant：找不到這一輪")
	// ErrCursorExpired 是 cursor 早於緩衝保留的第一個事件；對外 REPLAY_CURSOR_EXPIRED。
	ErrCursorExpired = errors.New("assistant：cursor 已過期")
	// ErrCursorAhead 是 cursor 超前於最新事件，或不是伺服器發出的值；對外 REPLAY_CURSOR_AHEAD。
	ErrCursorAhead = errors.New("assistant：cursor 超前")
	// ErrOuterTx 是在已有 transaction 的 ctx 上送出：輪次會在背景執行並寫 session，不得綁在呼叫者的 transaction 上。
	ErrOuterTx = errors.New("assistant：送出不能開在 transaction 內")
)
