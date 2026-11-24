// Package catalog 是 catalog 模組的公開 API：商品、規格、庫存與平台分類。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package catalog

import (
	"context"
	"embed"
	"io/fs"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/stock"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/taxonomy"
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

type (
	// StockItem 是一個 SKU 的扣減或歸還量。
	StockItem = model.StockItem
	// Category 是平台分類樹的一個節點。
	Category = model.Category
	// InsufficientStockError 表示庫存不足，整次扣減沒有生效。
	InsufficientStockError = model.InsufficientStockError
)

// 庫存操作的錯誤；用 errors.Is 判斷。
var (
	ErrInvalidStockInput       = model.ErrInvalidStockInput
	ErrSkuNotFound             = model.ErrSkuNotFound
	ErrIdempotencyKeyReused    = model.ErrIdempotencyKeyReused
	ErrDeductionNotFound       = model.ErrDeductionNotFound
	ErrRestoreExceedsDeduction = model.ErrRestoreExceedsDeduction
)

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB *db.DB
	// NewID 產生實體的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now func() time.Time
}

// Module 是 catalog 模組。
type Module struct {
	stock    *stock.Service
	taxonomy *taxonomy.Service
}

// New 建立模組。
func New(d Deps) *Module {
	return &Module{
		stock:    stock.New(stock.Deps{DB: d.DB, NewID: d.NewID, Now: d.Now}),
		taxonomy: taxonomy.New(taxonomy.Deps{DB: d.DB}),
	}
}

// DeductStock 以冪等鍵 key 扣減庫存，與呼叫方的 transaction 同進退（ctx 沒有 transaction 時自己開一個）。
// 同一個 key 重送且內容相同不再扣減；任何一個 SKU 庫存不足整次不生效，回 *InsufficientStockError。
// 在呼叫方的 transaction 內失敗時，呼叫方必須讓整個 transaction 回滾，不能吞掉錯誤繼續。
func (m *Module) DeductStock(ctx context.Context, key string, items []StockItem) error {
	return m.stock.Deduct(ctx, key, items)
}

// RestoreStock 依 deductKey 指的原扣減歸還庫存，同一個 key 重送不再歸還。items 為空表示歸還剩下的全部；有給時累計不得超過原扣減量
// （超過回 ErrRestoreExceedsDeduction）。transaction 規則同 DeductStock。
func (m *Module) RestoreStock(ctx context.Context, key, deductKey string, items []StockItem) error {
	return m.stock.Restore(ctx, key, deductKey, items)
}

// Categories 回傳整棵平台分類樹，頂層在前，同層依 position。
func (m *Module) Categories(ctx context.Context) ([]Category, error) { return m.taxonomy.List(ctx) }

// IsLeafCategory 回報分類是否存在且可掛商品（葉節點）。
func (m *Module) IsLeafCategory(ctx context.Context, id uuid.UUID) (bool, error) {
	return m.taxonomy.RequireLeaf(ctx, id)
}
