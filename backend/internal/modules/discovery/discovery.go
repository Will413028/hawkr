// Package discovery 是 discovery 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package discovery

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/discovery/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery/internal/ranking"
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

// Deps 是模組需要的外部依賴，全部必填；clock 由組裝點注入。Orders、Products、Reviews 在上游搬進單體之前由
// cmd/hawkr 以 internal/legacy 的 client 實作，Stores 是 seller 模組。
type Deps struct {
	DB       *db.DB
	Orders   Orders
	Products Products
	Reviews  Reviews
	Stores   Stores
	Now      func() time.Time
	Logger   *slog.Logger
}

// Module 是 discovery 模組。
type Module struct {
	ranking *ranking.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Orders == nil || d.Products == nil || d.Reviews == nil || d.Stores == nil {
		panic("discovery: missing dependency")
	}
	if d.Now == nil {
		panic("discovery: Now is required")
	}
	return &Module{ranking: ranking.New(ranking.Deps{
		DB: d.DB, Orders: d.Orders, Products: d.Products, Reviews: d.Reviews, Stores: d.Stores, Now: d.Now, Logger: d.Logger,
	})}
}

// RankedSeller 是排行的一列：店家 ID，與排名用到的訂單事實（近 30 天的成立訂單數，和排名一致，來自同一份快照）。
// 排名用的評價只留在快照內，不對外輸出；店名、logo、粉絲數、評價等顯示欄位不在這裡，由 internal/httpapi 組裝。
type RankedSeller = ranking.Ranked

// Recommendation 是 RecommendedSellers 的結果：排序後的店家，與快照的計算時間 ComputedAt（那次重算的開始時間）。
// ComputedAt 是 nil 表示從來沒有成功重算過（「還沒算過」）；它有值而 Sellers 是空的，才是「算過了，沒有合格店家」。
type Recommendation = ranking.Recommendation

// MaxLimit 是 RecommendedSellers 的 limit 上限。
const MaxLimit = ranking.MaxLimit

// RecommendedSellers 依分數由高到低回傳最多 limit 家目前可以公開曝光的店家。limit 必須是 1 到 MaxLimit，否則回
// InvalidInput（預設值屬於傳輸層）。分數是最近一次成功重算的快照，曝光資格每次讀取都即時檢查（停權立刻生效）；整個讀取
// 在同一個快照內，ComputedAt 屬於讀到的那份快照。呼叫端的 ctx 不可已經在 transaction 內。
func (m *Module) RecommendedSellers(ctx context.Context, limit int) (Recommendation, error) {
	return m.ranking.Recommended(ctx, limit)
}

// RecomputeResult 是一次重算的結果。
type RecomputeResult = ranking.Result

// RecomputeSellerScores 立即重算分數快照（排程每 15 分鐘做同一件事）。上游失敗時回 error，快照不變、失敗記進
// recompute_state；已有更晚開始的重算成功時，這次的結果被丟棄（Written 是 false，不是錯誤）。
func (m *Module) RecomputeSellerScores(ctx context.Context) (RecomputeResult, error) {
	return m.ranking.Recompute(ctx)
}

// AddWorkers 登記本模組的 River worker（分數重算）。
func (m *Module) AddWorkers(workers *river.Workers) { m.ranking.AddWorkers(workers) }

// PeriodicJobs 回傳啟動即執行的排程：每 15 分鐘重算一次分數快照。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return m.ranking.PeriodicJobs() }

// FormulaVersion 是目前的分數公式版本（存在每一列）。
const FormulaVersion = model.FormulaVersion
