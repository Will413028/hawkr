// Package accountdeletion 是 accountdeletion 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package accountdeletion

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion/internal/request"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
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

// Orders 是未結訂單的 port：買家側以 buyer_id 查，店家側（賣家是商家的訂單，含未付款與售後處理中）以帳號目前的 store_id 查。
// 兩個查詢都在 transaction 外；失敗時申請不寫入。ordering 搬進單體後由它的公開 API 實作並移進 transaction。
type Orders interface {
	HasOpenOrders(context.Context, int64) (bool, error)
	HasOpenStoreOrders(context.Context, int64) (bool, error)
}

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct {
	DB       *db.DB
	Identity *identity.Module
	Buyer    *buyer.Module
	Seller   *seller.Module
	Orders   Orders
	Now      func() time.Time
	NewID    func() uuid.UUID
	Logger   *slog.Logger
}

// Module 是 accountdeletion 模組。
type Module struct {
	request *request.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Identity == nil || d.Buyer == nil || d.Seller == nil || d.Orders == nil {
		panic("accountdeletion: missing dependency")
	}
	if d.Now == nil {
		panic("accountdeletion: Now is required")
	}
	if d.NewID == nil {
		d.NewID = uuid.NewV7
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Module{request: request.New(request.Deps{DB: d.DB, Identity: d.Identity, Buyer: d.Buyer, Seller: d.Seller, HasOpenOrders: d.Orders.HasOpenOrders, HasOpenStoreOrders: d.Orders.HasOpenStoreOrders, Now: d.Now, NewID: d.NewID, Logger: d.Logger})}
}

type RequestInput = model.RequestInput
type Requester = model.Requester
type State = model.State

const (
	AccountOwner  = model.AccountOwner
	Admin         = model.Admin
	PendingOrders = model.PendingOrders
	Completed     = model.Completed
)

func (m *Module) Request(ctx context.Context, in RequestInput) (State, error) {
	return m.request.Request(ctx, in)
}
func (m *Module) CompletePending(ctx context.Context, accountID int64) (State, error) {
	return m.request.CompletePending(ctx, accountID)
}

// AddWorkers 登記待辦申請的週期核對工作。
func (m *Module) AddWorkers(workers *river.Workers) { m.request.AddWorkers(workers) }

// PeriodicJobs 每小時排入一次待辦申請核對，啟動時立即補跑。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return m.request.PeriodicJobs() }
