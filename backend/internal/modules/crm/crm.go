// Package crm 是 crm 模組的公開 API：店家對買家的客戶分類與標籤、客戶列表與詳情、VIP 狀態，以及消費統計的過渡投影與刪帳、退場的清除。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package crm

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/project"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/read"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/write"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
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

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB   *db.DB
	Jobs jobs.Enqueuer
	// NewID 產生標籤的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now    func() time.Time
	Logger *slog.Logger
}

type (
	CustomerType      = model.CustomerType
	SortBy            = model.SortBy
	Tag               = model.Tag
	Customer          = model.Customer
	CustomerCursor    = model.CustomerCursor
	CustomerListInput = model.CustomerListInput
	CustomerPage      = model.CustomerPage
	OrderCompleted    = model.OrderCompleted
	RefundOutcome     = model.RefundOutcome
	Refund            = model.Refund
	Reason            = model.Reason
)

const (
	TypeNormal = model.TypeNormal
	TypeVIP    = model.TypeVIP

	SortFirstOrderAt = model.SortFirstOrderAt
	SortTotalSpent   = model.SortTotalSpent
	SortOrderCount   = model.SortOrderCount

	RefundPartial = model.RefundPartial
	RefundFull    = model.RefundFull

	MaxTagNameRunes    = model.MaxTagNameRunes
	MaxTagsPerCall     = model.MaxTagsPerCall
	MaxTagsPerStore    = model.MaxTagsPerStore
	MaxTagsPerCustomer = model.MaxTagsPerCustomer
	MaxStoreIDs        = model.MaxStoreIDs
	DefaultPageSize    = model.DefaultPageSize
	MaxPageSize        = model.MaxPageSize
	MaxEventKeyLen     = model.MaxEventKeyLen
	MaxOrderNoLen      = model.MaxOrderNoLen

	ReasonCustomerNotFound = model.ReasonCustomerNotFound
	ReasonTagNotFound      = model.ReasonTagNotFound
	ReasonTagNameTaken     = model.ReasonTagNameTaken
	ReasonTagLimit         = model.ReasonTagLimit
)

// ErrInvalidEvent 是投影事件的欄位不合法（重試不會變好）；ErrPartiesMismatch 是同單號的買家或店家與已記錄的不同。
var (
	ErrInvalidEvent    = model.ErrInvalidEvent
	ErrPartiesMismatch = model.ErrPartiesMismatch
)

// ErasePartyArgs 是清除 job 的參數（job 種類 crm.erase_party）。
type ErasePartyArgs = purge.ErasePartyArgs

// ReasonOf 回傳 err 帶的 crm 業務原因；沒有時是空字串。
func ReasonOf(err error) Reason { return model.ReasonOf(err) }

// Module 是 crm 模組。
type Module struct {
	write   *write.Service
	read    *read.Service
	project *project.Service
	purge   *purge.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Jobs == nil || d.Now == nil || d.NewID == nil {
		panic("crm：DB、Jobs、Now 與 NewID 是必要的")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Module{
		write:   write.New(write.Deps{DB: d.DB, NewID: d.NewID, Now: d.Now}),
		read:    read.New(read.Deps{DB: d.DB}),
		project: project.New(project.Deps{DB: d.DB, Now: d.Now}),
		purge:   purge.New(purge.Deps{DB: d.DB, Jobs: d.Jobs, Logger: d.Logger}),
	}
}

// ListCustomers 是店家的客戶（keyset，不回總數）；沒有關鍵字搜尋（A6-D18 W5）。
func (m *Module) ListCustomers(ctx context.Context, in CustomerListInput) (CustomerPage, error) {
	return m.read.ListCustomers(ctx, in)
}

// GetCustomer 是一位客戶與他的標籤；沒有電話、Email、地址。店家沒有這位客戶回 NotFound。
func (m *Module) GetCustomer(ctx context.Context, storeID, buyerID int64) (Customer, error) {
	return m.read.GetCustomer(ctx, storeID, buyerID)
}

// ListTags 是店家全部的標籤（名稱升序，不分頁，數量受 MaxTagsPerStore 約束）。
func (m *Module) ListTags(ctx context.Context, storeID int64) ([]Tag, error) {
	return m.read.ListTags(ctx, storeID)
}

// VIPStatuses 回傳買家在各店是否是 VIP（每個請求的店家一筆）；取代 legacy/crmservice.VIPStatuses。
func (m *Module) VIPStatuses(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]bool, error) {
	return m.read.VIPStatuses(ctx, buyerID, storeIDs)
}

// SetCustomerType 設定分類；分類值不合法回 InvalidInput，店家沒有這位客戶回 NotFound。
func (m *Module) SetCustomerType(ctx context.Context, storeID, buyerID int64, t CustomerType) error {
	return m.write.SetCustomerType(ctx, storeID, buyerID, t)
}

// CreateTag 建立標籤；撞名回 AlreadyExists（CRM_TAG_NAME_TAKEN）。
func (m *Module) CreateTag(ctx context.Context, storeID int64, name string) (Tag, error) {
	return m.write.CreateTag(ctx, storeID, name)
}

// RenameTag 改標籤名稱。
func (m *Module) RenameTag(ctx context.Context, storeID int64, tagID uuid.UUID, name string) error {
	return m.write.RenameTag(ctx, storeID, tagID, name)
}

// DeleteTag 刪標籤與它在客戶身上的關聯。
func (m *Module) DeleteTag(ctx context.Context, storeID int64, tagID uuid.UUID) error {
	return m.write.DeleteTag(ctx, storeID, tagID)
}

// AddCustomerTags 替客戶貼上一批標籤（不存在的先建立）；整批原子：任何一步失敗，整批都沒有寫入。
func (m *Module) AddCustomerTags(ctx context.Context, storeID, buyerID int64, names []string) error {
	return m.write.AddCustomerTags(ctx, storeID, buyerID, names)
}

// RemoveCustomerTag 移除客戶身上的一個標籤（冪等）。
func (m *Module) RemoveCustomerTag(ctx context.Context, storeID, buyerID int64, tagID uuid.UUID) error {
	return m.write.RemoveCustomerTag(ctx, storeID, buyerID, tagID)
}

// RecordOrderCompleted 投影一筆完成的訂單（只給 legacy/crmconsumer）。recorded 為 false 表示重放，或買家、店家已刪帳、退場。
func (m *Module) RecordOrderCompleted(ctx context.Context, e OrderCompleted) (recorded bool, err error) {
	return m.project.RecordOrderCompleted(ctx, e)
}

// RecordRefund 投影一筆完成的退款（只給 legacy/crmconsumer）；Refund 的 StoreID、BuyerID 由呼叫端經訂單解析。
func (m *Module) RecordRefund(ctx context.Context, r Refund) (recorded bool, err error) {
	return m.project.RecordRefund(ctx, r)
}

// OnBuyerDeleted 是買家關閉時同 transaction 的訂閱者（單體規則 4 的原子狀態變更）：把這個 buyer_id 記進 inactive_parties、排 crm.erase_party job，
// 不在這裡改客戶列。不訂 BuyerDeletionRequested：等待刪除的買家仍能登入，為開啟中的訂單聯絡店家。
// 鎖的順序 identity → buyer → seller → crm：accountdeletion 在派發事件之前先鎖完買家與店家。
func (m *Module) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return m.purge.Deactivate(ctx, purge.Buyer, e.BuyerID, e.OccurredAt)
}

// OnStoreRetired 是店家退場時同 transaction 的訂閱者：把這個 store_id 記進 inactive_parties、排 crm.erase_party job。
func (m *Module) OnStoreRetired(ctx context.Context, e seller.StoreRetired) error {
	return m.purge.Deactivate(ctx, purge.Store, e.StoreID, e.OccurredAt)
}

// ErasePartyData 清除一個身分的 crm 資料（分批，冪等）。kind 是 "BUYER" 或 "STORE"。它是 crm.erase_party job 的內容，給 worker 與測試呼叫。
func (m *Module) ErasePartyData(ctx context.Context, kind string, id int64) error {
	return m.purge.Erase(ctx, purge.Kind(kind), id)
}

// SweepErase 補排仍在 inactive_parties 又還有資料的身分的清除 job（保底掃描的內容，給 worker 與測試呼叫）。
func (m *Module) SweepErase(ctx context.Context) (int, error) { return m.purge.Sweep(ctx) }

// AddWorkers 登記 River worker（清除與其保底掃描）。由 cmd/hawkr 接上（A6-6e）。
func (m *Module) AddWorkers(workers *river.Workers) { purge.AddWorkers(workers, m.purge) }

// PeriodicJobs 回傳保底掃描的排程。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return slices.Concat(purge.PeriodicJobs()) }
