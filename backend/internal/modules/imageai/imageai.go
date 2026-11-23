// Package imageai 是 imageai 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package imageai

import (
	"context"
	"embed"
	"io/fs"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/limit"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/quota"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
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

// Schema 是本模組的 schema 名稱。
const Schema = "imageai"

const (
	// DefaultWeeklyLimit 是沒有個別設定的店每週可修圖的次數。
	DefaultWeeklyLimit = model.DefaultWeeklyLimit
	// MaxWeeklyLimit 是後台能為一家店設定的每週次數上限。
	MaxWeeklyLimit = model.MaxWeeklyLimit
)

// ErrQuotaExhausted 是本週額度用完。
var ErrQuotaExhausted = model.ErrQuotaExhausted

// ErrNothingToRefund 是 RefundQuota 找不到可退的次數（那一週的計數已不存在或已歸零）。
var ErrNothingToRefund = model.ErrNothingToRefund

// Usage 是一次扣額度之後的本週狀態。
type Usage = model.Usage

// StoreRetired 是 seller 在店家退場時同 transaction 發布的事件（本模組的訂閱者見 Module.OnStoreRetired）。
type StoreRetired = seller.StoreRetired

// Config 是模組的設定。
type Config struct {
	// MaxConcurrent 是每個 pod 同時進行的修圖數上限。
	MaxConcurrent int
}

// ReadConfig 讀 IMAGEAI_MAX_CONCURRENT（選填，預設 21，至少 1）。
func ReadConfig(e *config.Env) Config {
	n := e.Int("IMAGEAI_MAX_CONCURRENT", model.DefaultMaxConcurrent)
	if n < 1 {
		e.Fail("IMAGEAI_MAX_CONCURRENT 至少要 1，得到 %d", n)
	}
	return Config{MaxConcurrent: n}
}

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct {
	DB  *db.DB
	Now func() time.Time
	// MaxConcurrent 是每個 pod 同時進行的修圖數上限；0 取預設。
	MaxConcurrent int
}

// Module 是 imageai 模組。
type Module struct {
	quota   *quota.Service
	limiter *limit.Limiter
}

// New 建立模組。
func New(d Deps) *Module {
	n := d.MaxConcurrent
	if n == 0 {
		n = model.DefaultMaxConcurrent
	}
	return &Module{quota: quota.New(d.DB, d.Now), limiter: limit.New(n)}
}

// AcquireTransformSlot 不等待地佔一個同時修圖的名額；滿了回 ok=false。release 可重複呼叫，只釋放一次。
func (m *Module) AcquireTransformSlot() (release func(), ok bool) { return m.limiter.TryAcquire() }

// ConsumeQuota 扣店家本週一次；用完回 ErrQuotaExhausted（Usage 仍帶上限與重置時間）。
func (m *Module) ConsumeQuota(ctx context.Context, storeID int64) (Usage, error) {
	return m.quota.Consume(ctx, storeID)
}

// RefundQuota 退回 ConsumeQuota 扣的那一次（用回傳的 Usage.PeriodStart）；沒有可退的回 ErrNothingToRefund。永遠在呼叫端的 transaction 之外執行。
func (m *Module) RefundQuota(ctx context.Context, storeID int64, periodStart time.Time) error {
	return m.quota.Refund(ctx, storeID, periodStart)
}

// OnStoreRetired 是店家退場時同 transaction 的訂閱者：刪該店的個別額度與用量。冪等。
func (m *Module) OnStoreRetired(ctx context.Context, e StoreRetired) error {
	return m.quota.OnStoreRetired(ctx, e.StoreID)
}
