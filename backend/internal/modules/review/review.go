// Package review 是 review 模組的公開 API：訂單項目的買家評價、店家回覆、隱藏與還原、評價統計。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package review

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/read"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/write"
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

// Orders 是 review 對訂單的唯一依賴；訂單不存在回 ErrOrderNotFound，其他錯誤是暫時性故障。
type Orders = model.Orders

// Buyers 是提交時查上傳者帳號的 buyer 公開 API；正式組裝傳 *buyer.Module。
type Buyers = write.Buyers

// Stores 是讀店家（取 store_code）的 seller 公開 API；正式組裝傳 *seller.Module。
type Stores = write.Stores

// ErrOrderNotFound 是 Orders 的實作回報「訂單不存在」的方式。
var ErrOrderNotFound = model.ErrOrderNotFound

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Media  media.Media
	Jobs   jobs.Enqueuer
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
	Review                 = model.Review
	RelayReview            = model.RelayReview
	Image                  = model.Image
	Reply                  = model.Reply
	Cursor                 = model.Cursor
	Page                   = model.Page
	Filter                 = model.Filter
	SubmitInput            = model.SubmitInput
	ReplyInput             = model.ReplyInput
	HideInput              = model.HideInput
	RestoreInput           = model.RestoreInput
	Hide                   = model.Hide
	ProductStats           = model.ProductStats
	StoreStats             = model.StoreStats
	StoreRating            = model.StoreRating
	OrderForReview         = model.OrderForReview
	OrderItem              = model.OrderItem
	Reason                 = model.Reason
	ReviewCreated          = model.ReviewCreated
	ReviewAggregateChanged = model.ReviewAggregateChanged
	Target                 = model.Target
)

const (
	MaxContentRunes          = model.MaxContentRunes
	MaxImages                = model.MaxImages
	MaxPageSize              = model.MaxPageSize
	MaxStoreRatingStatsBatch = model.MaxStoreRatingStatsBatch
	OrderStatusCompleted     = model.OrderStatusCompleted

	ReasonAlreadyExists      = model.ReasonAlreadyExists
	ReasonOrderNotFound      = model.ReasonOrderNotFound
	ReasonOrderNotReviewable = model.ReasonOrderNotReviewable
	ReasonWindowExpired      = model.ReasonWindowExpired
	ReasonNotReplyable       = model.ReasonNotReplyable
	ReasonModerationRejected = model.ReasonModerationRejected
)

// ReasonOf 回傳 err 帶的 review 業務原因；沒有時是空字串。
func ReasonOf(err error) Reason { return model.ReasonOf(err) }

// Module 是 review 模組。
type Module struct {
	write     *write.Service
	read      *read.Service
	mediaSync *media.RefSyncer
	purge     *purge.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.Now == nil || d.NewID == nil {
		panic("review: Now 與 NewID 是必要的")
	}
	if d.Orders == nil || d.Buyers == nil || d.Stores == nil {
		panic("review: Orders、Buyers 與 Stores 是必要的")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Module{
		write: write.New(write.Deps{
			DB: d.DB, Events: d.Events, Media: d.Media, Jobs: d.Jobs, Buyers: d.Buyers, Stores: d.Stores, Orders: d.Orders,
			NewID: d.NewID, Now: d.Now, Logger: d.Logger,
		}),
		read: read.New(read.Deps{DB: d.DB}),
		purge: purge.New(purge.Deps{
			DB: d.DB, Events: d.Events, Jobs: d.Jobs, Stores: d.Stores, NewID: d.NewID, Now: d.Now, Logger: d.Logger,
		}),
		mediaSync: mediaref.NewSyncer(d.DB, d.Media, d.Jobs, d.Now, d.Logger),
	}
}

// Submit 提交一則評價：訂單要已完成、屬於買家、在期限內；product_id、sku_name、price、商品圖都取自訂單項目。
func (m *Module) Submit(ctx context.Context, in SubmitInput) (Review, error) {
	return m.write.Submit(ctx, in)
}

// Reply 是店家回覆評價（可覆蓋）；店家要有 CapMarketing。
func (m *Module) Reply(ctx context.Context, in ReplyInput) (Review, error) {
	return m.write.Reply(ctx, in)
}

// Hide 隱藏一則評價（moderation 呼叫），冪等；新隱藏時發 ReviewAggregateChanged。
func (m *Module) Hide(ctx context.Context, in HideInput) (Hide, error) { return m.write.Hide(ctx, in) }

// Restore 還原目前未還原的隱藏，發 ReviewAggregateChanged。
func (m *Module) Restore(ctx context.Context, in RestoreInput) (Hide, error) {
	return m.write.Restore(ctx, in)
}

// ListProductReviews 是商品的公開評價；viewerBuyerID 大於 0 時套用觀看者的封鎖。size 為 0 用預設，超過 MaxPageSize 以上限計。
func (m *Module) ListProductReviews(ctx context.Context, productID, viewerBuyerID int64, f Filter, after *Cursor, size int32) (Page, error) {
	return m.read.ListProductReviews(ctx, productID, viewerBuyerID, f, after, size)
}

// ListStoreReviews 是店家的公開評價，規則同 ListProductReviews。
func (m *Module) ListStoreReviews(ctx context.Context, storeID, viewerBuyerID int64, f Filter, after *Cursor, size int32) (Page, error) {
	return m.read.ListStoreReviews(ctx, storeID, viewerBuyerID, f, after, size)
}

// ListOwnStoreReviews 是店家自己後台的評價：公開可讀，不套觀看者。
func (m *Module) ListOwnStoreReviews(ctx context.Context, storeID int64, f Filter, after *Cursor, size int32) (Page, error) {
	return m.read.ListOwnStoreReviews(ctx, storeID, f, after, size)
}

// ListMyReviews 是作者本人的評價歷史（隱藏的不給作者）。
func (m *Module) ListMyReviews(ctx context.Context, buyerID int64, after *Cursor, size int32) (Page, error) {
	return m.read.ListMyReviews(ctx, buyerID, after, size)
}

// GetReview 是單筆公開讀取：不可讀的與不存在的同為 NotFound。
func (m *Module) GetReview(ctx context.Context, id uuid.UUID, viewerBuyerID int64) (Review, error) {
	return m.read.GetReview(ctx, id, viewerBuyerID)
}

// ProductStats 是商品的評價統計。
func (m *Module) ProductStats(ctx context.Context, productID int64) (ProductStats, error) {
	return m.read.ProductStats(ctx, productID)
}

// StoreStats 是店家的評價統計。
func (m *Module) StoreStats(ctx context.Context, storeID int64) (StoreStats, error) {
	return m.read.StoreStats(ctx, storeID)
}

// StoreRatingStats 批次回傳多家店的評分（1 到 MaxStoreRatingStatsBatch 家、只回有公開評價的店、失敗回錯誤）。
func (m *Module) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]StoreRating, error) {
	return m.read.StoreRatingStats(ctx, storeIDs)
}

// SyncMedia 同步一個 ref 的 media 引用（job 與測試用）。
func (m *Module) SyncMedia(ctx context.Context, ref media.Ref) (media.SyncResult, error) {
	return m.mediaSync.Sync(ctx, ref)
}

// SweepMedia 補排版本比已同步的新的 ref。
func (m *Module) SweepMedia(ctx context.Context) (int, error) { return m.mediaSync.Sweep(ctx) }

// SweepPurge 補排仍在 inactive_authors 又還有評價列的買家的清除 job（保底掃描的內容，給 worker 與測試呼叫）。
func (m *Module) SweepPurge(ctx context.Context) (int, error) { return m.purge.Sweep(ctx) }

// OnBuyerDeletionRequested 是買家進入刪除中時同 transaction 的訂閱者（單體規則 4 的原子狀態變更）：把買家記進 inactive_authors
// （評價從這個 transaction 起不可讀、不計分）、排 review.purge_author job。不在這裡刪列：
// 刪帳者的評價筆數沒有上限。鎖的順序 identity → buyer → seller → review：accountdeletion 在派發事件之前先鎖完買家與店家。
func (m *Module) OnBuyerDeletionRequested(ctx context.Context, e buyer.BuyerDeletionRequested) error {
	return m.purge.Deactivate(ctx, e.BuyerID, e.OccurredAt)
}

// OnBuyerDeleted 是買家關閉時同 transaction 的訂閱者：同 OnBuyerDeletionRequested（冪等）。沒有開啟中的訂單時買家直接關閉，只發這個
// 事件、沒有 BuyerDeletionRequested，所以兩個事件都要訂。
func (m *Module) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return m.purge.Deactivate(ctx, e.BuyerID, e.OccurredAt)
}

// PurgeAuthor 刪除買家的評價（分批，冪等）。它是 review.purge_author job 的內容，給 worker 與測試呼叫。
func (m *Module) PurgeAuthor(ctx context.Context, buyerID int64) error {
	return m.purge.Purge(ctx, buyerID)
}

// ReviewForRelay 讀單則評價的原始列，不套公開可讀述詞（事件的 payload 要這則評價現在的內容），Public 是現在是否公開可讀；已清除的是 NotFound。
func (m *Module) ReviewForRelay(ctx context.Context, id uuid.UUID) (RelayReview, error) {
	return m.read.ReviewForRelay(ctx, id)
}

// AddWorkers 登記 River worker（媒體引用同步與保底掃描、刪帳清除）。由 cmd/hawkr 接上。
func (m *Module) AddWorkers(workers *river.Workers) {
	mediaref.AddWorkers(workers, m.mediaSync)
	purge.AddWorkers(workers, m.purge)
}

// PeriodicJobs 回傳保底掃描的排程（媒體引用、刪帳清除）。
func (m *Module) PeriodicJobs() []*river.PeriodicJob {
	return append(mediaref.PeriodicJobs(), purge.PeriodicJobs()...)
}
