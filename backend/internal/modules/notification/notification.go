// Package notification 是 notification 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package notification

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/app"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
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

// Buyers 是 worker 對 buyer 的需求（*buyer.Module 滿足它）：以 FOR KEY SHARE 鎖住收件買家列，並回報買家是否已關閉。
type Buyers = intake.Buyers

// Stores 是 worker 對 seller 的需求（*seller.Module 滿足它）：以 FOR KEY SHARE 鎖住收件店家列，店家已退場回 NotFound。
type Stores = intake.Stores

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct {
	DB       *db.DB
	Identity Accounts
	Buyer    Buyers
	Seller   Stores
	Jobs     jobs.Enqueuer
	Now      func() time.Time
	NewID    func() uuid.UUID
	Logger   *slog.Logger
}

// Module 是 notification 模組。
type Module struct {
	intake *intake.Service
	purge  *purge.Service
	app    *app.Service
}

// New 建立模組。
func New(d Deps) *Module { return newModule(d, registry.Registry{}) }

func newModule(d Deps, reg intake.Registry) *Module {
	return &Module{
		intake: intake.New(intake.Deps{
			DB: d.DB, Accounts: d.Identity, Buyers: d.Buyer, Stores: d.Seller, Registry: reg, Jobs: d.Jobs, Now: d.Now, NewID: d.NewID, Logger: d.Logger,
		}),
		purge: purge.New(purge.Deps{DB: d.DB, Jobs: d.Jobs, Now: d.Now, Logger: d.Logger}),
		app:   app.New(app.Deps{DB: d.DB, Accounts: d.Identity, Registry: registry.Registry{}, Now: d.Now}),
	}
}

// OnSuspended 是 accountsuspension.Suspended 的同步訂閱者：只在發布的 transaction 內登記意圖並排 job。
func (m *Module) OnSuspended(ctx context.Context, e accountsuspension.Suspended) error {
	return m.intake.OnSuspended(ctx, e)
}

// OnUnsuspended 是 accountsuspension.Unsuspended 的同步訂閱者，同 OnSuspended。
func (m *Module) OnUnsuspended(ctx context.Context, e accountsuspension.Unsuspended) error {
	return m.intake.OnUnsuspended(ctx, e)
}

// OnAccountDeleted 是 identity.AccountDeleted 的同步訂閱者：只在發布的 transaction 內排個資清除 job。
func (m *Module) OnAccountDeleted(ctx context.Context, e identity.AccountDeleted) error {
	return m.purge.OnAccountDeleted(ctx, e)
}

// OnBuyerDeleted 是 buyer.BuyerDeleted 的同步訂閱者，同 OnAccountDeleted。
func (m *Module) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return m.purge.OnBuyerDeleted(ctx, e)
}

// OnStoreRetired 是 seller.StoreRetired 的同步訂閱者，同 OnAccountDeleted。
func (m *Module) OnStoreRetired(ctx context.Context, e seller.StoreRetired) error {
	return m.purge.OnStoreRetired(ctx, e)
}

// AddWorkers 登記 River worker（意圖展開、保留期清除、個資清除）。由 cmd/hawkr 的 worker 接上。
func (m *Module) AddWorkers(workers *river.Workers) {
	m.intake.AddWorkers(workers)
	purge.AddWorkers(workers, m.purge)
}

// PeriodicJobs 回傳週期 job：保留期清除。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return purge.PeriodicJobs() }
