// Package assistant 是 assistant 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package assistant

import (
	"context"
	"embed"
	"io/fs"
	"iter"
	"time"

	adkmodel "google.golang.org/adk/v2/model"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/adksession"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/feature"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/turn"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct {
	DB *db.DB
	// LLM 取自 internal/platform/llm 的單一建構點（Model.LLM()）；模組不自建 client。
	LLM adkmodel.LLM
	// Orders 是讀買家訂單的 port，由 cmd/hawkr 以訂單服務的 client 實作。
	Orders OrderReader
	// Now 回現在時間，空值取系統時間。
	Now func() time.Time
}

// Module 是 assistant 模組。
type Module struct {
	feature  *feature.Service
	sessions *adksession.Service
	turns    *turn.Executor
}

// New 建立模組。
func New(d Deps) *Module {
	sessions := adksession.New(d.DB)
	return &Module{
		feature:  feature.New(feature.Deps{}),
		sessions: sessions,
		turns:    turn.New(turn.Config{Sessions: sessions, Model: d.LLM, Orders: d.Orders, Now: d.Now}),
	}
}

// Shutdown 取消進行中的輪次（它們以失敗結束）並等它們收尾，或等到 ctx 結束。
func (m *Module) Shutdown(ctx context.Context) error { return m.turns.Shutdown(ctx) }

// SubmitTurn 受理一則買家訊息，立刻回輪次 ID，agent 在背景執行（A9-D12）。必須在最上層呼叫，不得在 transaction 內。
// 同一個對話已有一輪進行中回 ErrTurnInProgress（A9-D14）。語系取自 ctx（uitext.LocaleFrom）。
func (m *Module) SubmitTurn(ctx context.Context, in SubmitTurnInput) (TurnRef, error) {
	return m.turns.Submit(ctx, in)
}

// WatchTurn 訂閱一輪的事件，從 cursor 之後開始；錯誤與迭代器語意見 turn.Executor.Watch。
func (m *Module) WatchTurn(ctx context.Context, buyerID int64, turnID, cursor string) (iter.Seq2[TurnEvent, error], error) {
	return m.turns.Watch(ctx, buyerID, turnID, cursor)
}

// OrderReader 是買家助理讀訂單的 port。
type OrderReader = buyeragent.Orders

// BuyerOrder 與 BuyerOrderDetail 是 OrderReader 回傳的訂單。
type (
	BuyerOrder       = buyeragent.Order
	BuyerOrderDetail = buyeragent.OrderDetail
)

// SubmitTurnInput 是送出一則訊息；TurnRef 是受理結果；TurnEvent 是輪次事件。
type (
	SubmitTurnInput = turn.SubmitInput
	TurnRef         = turn.Ref
	TurnEvent       = model.Event
)

// Topic 是訊息的主題。
type Topic = model.Topic

// 主題。
const (
	TopicOrders       = model.TopicOrders
	TopicTracking     = model.TopicTracking
	TopicReturnPolicy = model.TopicReturnPolicy
	TopicChat         = model.TopicChat
)

// 事件種類與失敗種類。
type (
	EventKind   = model.EventKind
	FailureKind = model.FailureKind
)

// 事件種類。
const (
	EventText       = model.EventText
	EventToolResult = model.EventToolResult
	EventFinished   = model.EventFinished
	EventError      = model.EventError
)

// 失敗種類。
const (
	FailureNoOutput    = model.FailureNoOutput
	FailureInterrupted = model.FailureInterrupted
)

// 模組錯誤（A9-2c 的 handler 對應到 Connect code 與 ErrorReason）。
var (
	ErrInvalidTopic    = model.ErrInvalidTopic
	ErrMessageRequired = model.ErrMessageRequired
	ErrSessionNotFound = model.ErrSessionNotFound
	ErrSessionCreate   = model.ErrSessionCreate
	ErrTurnInProgress  = model.ErrTurnInProgress
	ErrTurnNotFound    = model.ErrTurnNotFound
	ErrCursorExpired   = model.ErrCursorExpired
	ErrCursorAhead     = model.ErrCursorAhead
	ErrOuterTx         = model.ErrOuterTx
)
