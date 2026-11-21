// Package procurement 是 procurement 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package procurement

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/retention"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
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

// Schema 是本模組的 schema 名稱，也是 ADK session 四張表（internal/platform/adksession）所在的 schema。
const Schema = "procurement"

// BuyerDeleted 是 buyer 關閉買家時同 transaction 發布的事件（本模組的訂閱者見 Module.OnBuyerDeleted）。
type BuyerDeleted = buyer.BuyerDeleted

// SellerStores 是候選搜尋確認店家是否營業中要用的 seller 方法（*seller.Module 滿足它）。
type SellerStores interface {
	ExposableStores(ctx context.Context, storeIDs []int64) ([]seller.ExposableStore, error)
}

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct {
	DB     *db.DB
	Now    func() time.Time
	Logger *slog.Logger
	// Sellers 是候選搜尋確認店家營業中的 seller 方法；A9-3b 掛上候選搜尋時才有呼叫者。
	Sellers SellerStores
}

// Module 是 procurement 模組。
type Module struct {
	retention *retention.Service
	sweeper   run.Sweeper
	purger    run.Purger
}

// New 建立模組。
func New(d Deps) *Module {
	switch {
	case d.DB == nil:
		panic("procurement.New：Deps.DB 必填")
	case d.Now == nil:
		panic("procurement.New：Deps.Now 必填")
	case d.Logger == nil:
		panic("procurement.New：Deps.Logger 必填")
	}
	ledger := run.Ledger{DB: d.DB, Now: d.Now}
	return &Module{
		retention: retention.New(d.DB),
		sweeper:   run.Sweeper{DB: d.DB, Ledger: ledger, QueuedTimeout: run.QueuedTimeout, RunTimeout: run.RunningTimeout, Logger: d.Logger},
		purger:    run.Purger{DB: d.DB, Ledger: ledger},
	}
}

// AddWorkers 登記 run 的 sweeper 與事件清除的 River worker。
func (m *Module) AddWorkers(workers *river.Workers) { run.AddWorkers(workers, m.sweeper, m.purger) }

// PeriodicJobs 回傳 sweeper 與事件清除的排程。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return run.PeriodicJobs() }

// OnBuyerDeleted 是買家關閉時同 transaction 的訂閱者（A9-D19 S10）：刪除買家的採購資料，賣家看得到的紀錄保留並把 buyer_id 設 NULL。冪等。
func (m *Module) OnBuyerDeleted(ctx context.Context, e BuyerDeleted) error {
	return m.retention.OnBuyerDeleted(ctx, e.BuyerID)
}

// openStores 把 seller.ExposableStores 轉成 candidate.OpenStores，依 seller 的批次上限分批。
type openStores struct{ sellers SellerStores }

var _ candidate.OpenStores = openStores{}

func (o openStores) OpenStoreIDs(ctx context.Context, storeIDs []int64) (map[int64]struct{}, error) {
	out := make(map[int64]struct{}, len(storeIDs))
	for start := 0; start < len(storeIDs); start += seller.MaxBatchSize {
		end := min(start+seller.MaxBatchSize, len(storeIDs))
		rows, err := o.sellers.ExposableStores(ctx, storeIDs[start:end])
		if err != nil {
			return nil, fmt.Errorf("procurement：讀可曝光的店家：%w", err)
		}
		for _, r := range rows {
			out[r.StoreID] = struct{}{}
		}
	}
	return out, nil
}
