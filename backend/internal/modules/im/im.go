// Package im 是 im 模組的公開 API：買家與店家之間的一對一聊天（對話、訊息、送出、已讀、刪帳與退場的抹除）。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package im

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/read"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/write"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
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

// Orders 是 im 對訂單的唯一依賴：雙方之間是否有進行中的訂單或售後（查不到回錯誤，呼叫端一律擋）。
type Orders = model.Orders

// Buyers 是送出用到的 buyer 公開 API；正式組裝傳 *buyer.Module。
type Buyers = write.Buyers

// Stores 是送出用到的 seller 公開 API；正式組裝傳 *seller.Module。
type Stores = write.Stores

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Media  media.Media
	Buyers Buyers
	Stores Stores
	Orders Orders
	// NewID 產生實體的主鍵與事件 ID，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now    func() time.Time
	Logger *slog.Logger
}

type (
	Party            = model.Party
	PartyKind        = model.PartyKind
	MessageType      = model.MessageType
	Message          = model.Message
	Image            = model.Image
	Preview          = model.Preview
	Conversation     = model.Conversation
	ConversationPage = model.ConversationPage
	MessagePage      = model.MessagePage
	Cursor           = model.Cursor
	SendInput        = model.SendInput
	RelayMessage     = model.RelayMessage
	HideInput        = model.HideInput
	RestoreInput     = model.RestoreInput
	Hide             = model.Hide
	MessageSent      = model.MessageSent
	Reason           = model.Reason
)

const (
	PartyBuyer = model.PartyBuyer
	PartyStore = model.PartyStore

	TypeText  = model.TypeText
	TypeImage = model.TypeImage

	MaxTextRunes    = model.MaxTextRunes
	MaxPreviewRunes = model.MaxPreviewRunes
	DefaultPageSize = model.DefaultPageSize
	MaxPageSize     = model.MaxPageSize

	ReasonSelfMessageNotAllowed = model.ReasonSelfMessageNotAllowed
	ReasonSellerSuspended       = model.ReasonSellerSuspended
	ReasonStoreClosed           = model.ReasonStoreClosed
	ReasonChatBlocked           = model.ReasonChatBlocked
	ReasonReceiverDeleted       = model.ReasonReceiverDeleted
	ReasonModerationRejected    = model.ReasonModerationRejected
)

// ErasePartyArgs 是抹除 job 的參數（job 種類 im.erase_party）。
type ErasePartyArgs = purge.ErasePartyArgs

// ReasonOf 回傳 err 帶的 im 業務原因；沒有時是空字串。
func ReasonOf(err error) Reason { return model.ReasonOf(err) }

// Module 是 im 模組。
type Module struct {
	write     *write.Service
	read      *read.Service
	mediaSync *media.RefSyncer
	purge     *purge.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.Now == nil || d.NewID == nil {
		panic("im: Now 與 NewID 是必要的")
	}
	if d.Orders == nil || d.Buyers == nil || d.Stores == nil {
		panic("im: Orders、Buyers 與 Stores 是必要的")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Module{
		write: write.New(write.Deps{
			DB: d.DB, Events: d.Events, Media: d.Media, Jobs: d.Jobs, Buyers: d.Buyers, Stores: d.Stores, Orders: d.Orders,
			NewID: d.NewID, Now: d.Now, Logger: d.Logger,
		}),
		read:      read.New(read.Deps{DB: d.DB}),
		purge:     purge.New(purge.Deps{DB: d.DB, Jobs: d.Jobs, NewID: d.NewID, Now: d.Now, Logger: d.Logger}),
		mediaSync: mediaref.NewSyncer(d.DB, d.Media, d.Jobs, d.Now, d.Logger),
	}
}

// Send 送出一則訊息：寄件人由呼叫者（httpapi 從 session 解析）決定，對話以 ID 授權；守門順序、例外與錯誤見 AGENTS.md。
func (m *Module) Send(ctx context.Context, in SendInput) (Message, error) {
	return m.write.Send(ctx, in)
}

// MarkRead 把呼叫者這一方的已讀游標推到 uptoSeq（只前進，不超過對話的最後一個 seq）。
func (m *Module) MarkRead(ctx context.Context, conversationID uuid.UUID, p Party, uptoSeq int64) error {
	return m.write.MarkRead(ctx, conversationID, p, uptoSeq)
}

// MarkAllRead 把呼叫者這一方的全部對話標成已讀。
func (m *Module) MarkAllRead(ctx context.Context, p Party) error { return m.write.MarkAllRead(ctx, p) }

// Hide 隱藏一則訊息（moderation 呼叫），冪等；隱藏的訊息在所有讀取路徑都回墓碑。
func (m *Module) Hide(ctx context.Context, in HideInput) (Hide, error) { return m.write.Hide(ctx, in) }

// Restore 還原目前未還原的隱藏。
func (m *Module) Restore(ctx context.Context, in RestoreInput) (Hide, error) {
	return m.write.Restore(ctx, in)
}

// ListConversations 是呼叫者這一方的對話（新的在前，keyset，不回總數，不因封鎖隱藏對話）。size 為 0 用預設，超過 MaxPageSize 以上限計。
func (m *Module) ListConversations(ctx context.Context, p Party, after *Cursor, size int32) (ConversationPage, error) {
	return m.read.ListConversations(ctx, p, after, size)
}

// ListMessages 是對話裡 seq 小於 beforeSeq 的訊息（新的在前；beforeSeq 為 0 從最新開始）。對話不屬於呼叫者回 NotFound。
func (m *Module) ListMessages(ctx context.Context, conversationID uuid.UUID, p Party, beforeSeq int64, size int32) (MessagePage, error) {
	return m.read.ListMessages(ctx, conversationID, p, beforeSeq, size)
}

// MessagesAfter 是對話裡 seq 大於 afterSeq 的訊息（舊的在前），給斷線補洞。
func (m *Module) MessagesAfter(ctx context.Context, conversationID uuid.UUID, p Party, afterSeq int64, size int32) (MessagePage, error) {
	return m.read.MessagesAfter(ctx, conversationID, p, afterSeq, size)
}

// UnreadTotal 是呼叫者這一方的總未讀數。
func (m *Module) UnreadTotal(ctx context.Context, p Party) (int64, error) {
	return m.read.UnreadTotal(ctx, p)
}

// MessageForRelay 讀單則訊息與它的對話的原始資料，不套可見性（Hidden、Erased 標出）；不存在是 NotFound。
func (m *Module) MessageForRelay(ctx context.Context, id uuid.UUID) (RelayMessage, error) {
	return m.read.MessageForRelay(ctx, id)
}

// OnBuyerDeleted 是買家關閉時同 transaction 的訂閱者（單體規則 4 的原子狀態變更）：把這個 buyer_id 記進 inactive_parties、排 im.erase_party job，
// 不在這裡改訊息列。不訂 BuyerDeletionRequested：等待刪除的買家仍能登入，為開啟中的訂單聯絡店家。
// 鎖的順序 identity → buyer → seller → im：accountdeletion 在派發事件之前先鎖完買家與店家。
func (m *Module) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return m.purge.Deactivate(ctx, PartyBuyer, e.BuyerID, e.OccurredAt)
}

// OnStoreRetired 是店家退場時同 transaction 的訂閱者：把這個 store_id 記進 inactive_parties、排 im.erase_party job。
// 只抹店家身分寄出的訊息；同一個帳號的買家身分由 OnBuyerDeleted 負責。
func (m *Module) OnStoreRetired(ctx context.Context, e seller.StoreRetired) error {
	return m.purge.Deactivate(ctx, PartyStore, e.StoreID, e.OccurredAt)
}

// ErasePartyMessages 抹除一個身分寄出的訊息（分批，冪等）。它是 im.erase_party job 的內容，給 worker 與測試呼叫。
func (m *Module) ErasePartyMessages(ctx context.Context, kind PartyKind, id int64) error {
	return m.purge.Erase(ctx, kind, id)
}

// SweepErase 補排仍在 inactive_parties 又還有未抹除訊息的身分的抹除 job（保底掃描的內容，給 worker 與測試呼叫）。
func (m *Module) SweepErase(ctx context.Context) (int, error) { return m.purge.Sweep(ctx) }

// SyncMedia 同步一個 ref 的 media 引用（job 與測試用）。
func (m *Module) SyncMedia(ctx context.Context, ref media.Ref) (media.SyncResult, error) {
	return m.mediaSync.Sync(ctx, ref)
}

// SweepMedia 補排版本比已同步的新的 ref。
func (m *Module) SweepMedia(ctx context.Context) (int, error) { return m.mediaSync.Sweep(ctx) }

// AddWorkers 登記 River worker（媒體引用同步與保底掃描、抹除與其保底掃描）。由 cmd/hawkr 接上。
func (m *Module) AddWorkers(workers *river.Workers) {
	mediaref.AddWorkers(workers, m.mediaSync)
	purge.AddWorkers(workers, m.purge)
}

// PeriodicJobs 回傳保底掃描的排程（媒體引用、抹除）。
func (m *Module) PeriodicJobs() []*river.PeriodicJob {
	return append(mediaref.PeriodicJobs(), purge.PeriodicJobs()...)
}
