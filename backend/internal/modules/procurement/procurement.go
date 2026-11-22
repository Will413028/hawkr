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
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/extract"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/retention"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/turn"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
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
	// Jobs 排入與取消 run 的 River job（*jobs.Client 滿足它）；送出與取消路徑需要，只跑 worker 的程序可以不給。
	Jobs JobQueue
	// Executors 依 run 類型登記執行者（run.RunTypeTurn 等）；沒有登記的類型，worker 領到時把 run 標為該類型的 pipeline 失敗。
	// 給了 LLM 與 Taxonomy 時，New 登記 TURN 的 executor（這裡已有 TURN 的項目則保留它）。
	Executors map[string]Executor
	// LLM 是採購用的模型（platform/llm 的 Procurement 角色）；與 Taxonomy 一起給才登記 TURN 的 executor。停用的模型也要給：run 會以 ErrDisabled 失敗退額度。
	LLM llm.Model
	// Taxonomy 讀平台的分類樹（品項解析），catalog 搬進單體前由 cmd/hawkr 以 internal/legacy 的 client 轉接。
	Taxonomy TaxonomyReader
	// Limits 是每位買家的送出上限，零值取預設（同時進行中 2、每分鐘送出 10）。
	Limits run.Limits
}

// TaxonomyReader 讀平台的啟用分類樹（扁平節點，根的 ParentID 為 0）。
type TaxonomyReader = resolver.TaxonomyReader

// TaxonomyNode 是分類樹的一個節點。
type TaxonomyNode = resolver.Node

// JobQueue 是 run 需要的 River 操作。
type JobQueue = run.JobQueue

// Executor 執行一個被領走的 run（契約見 run.Executor）。
type Executor = run.Executor

// Module 是 procurement 模組。
type Module struct {
	retention  *retention.Service
	reconciler run.Reconciler
	purger     run.Purger
	dispatcher run.Dispatcher
	submitter  run.Submitter
	canceller  run.Canceller
	hub        *run.Hub
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
		dispatcher: run.Dispatcher{DB: d.DB, Ledger: ledger, Executors: executors(d, ledger), Logger: d.Logger},
		submitter:  run.Submitter{DB: d.DB, Ledger: ledger, Jobs: d.Jobs, Limits: d.Limits},
		canceller:  run.Canceller{DB: d.DB, Ledger: ledger, Jobs: d.Jobs},
		hub:        run.NewHub(d.DB, run.MaxSubscriptions, d.Logger),
		retention:  retention.New(d.DB),
		reconciler: run.Reconciler{DB: d.DB, Ledger: ledger, Logger: d.Logger},
		purger:     run.Purger{DB: d.DB, Ledger: ledger},
	}
}

// executors 在呼叫端給的登記之外補上 TURN 的 executor：LLM 與 Taxonomy 都給才登記，只給其中一個是組裝錯誤。
func executors(d Deps, ledger run.Ledger) map[string]Executor {
	hasLLM := d.LLM.LLM() != nil
	if hasLLM != (d.Taxonomy != nil) {
		panic("procurement.New：Deps.LLM 與 Deps.Taxonomy 要一起給")
	}
	if !hasLLM {
		return d.Executors
	}
	out := make(map[string]Executor, len(d.Executors)+1)
	for k, v := range d.Executors {
		out[k] = v
	}
	if _, ok := out[run.RunTypeTurn]; ok {
		return out
	}
	shape := extract.ShapeSchema // 停用的模型沒有 provider：呼叫一律回 ErrDisabled，形狀無關
	if d.LLM.Enabled() {
		var err error
		if shape, err = extract.ShapeFor(d.LLM.Provider()); err != nil {
			panic(fmt.Sprintf("procurement.New：%v", err))
		}
	}
	out[run.RunTypeTurn] = turn.Executor{
		DB: d.DB, Ledger: ledger, Extractor: extract.New(d.LLM.LLM(), d.LLM.Name(), shape),
		Resolver: resolver.New(d.Taxonomy, resolver.EmbeddedSynonyms(), d.Now), Now: d.Now, Logger: d.Logger,
	}
	return out
}

// AddWorkers 登記執行 run 的 worker、reconciler 與事件清除的 River worker。
func (m *Module) AddWorkers(workers *river.Workers) {
	m.dispatcher.AddWorker(workers)
	run.AddWorkers(workers, m.reconciler, m.purger)
}

// RunHub 維持 WatchRun 共用的 LISTEN 連線直到 ctx 結束（回傳 ctx.Err()）；serve 程序啟動時另開 goroutine 跑它。
func (m *Module) RunHub(ctx context.Context) error { return m.hub.Run(ctx) }

// PeriodicJobs 回傳 reconciler 與事件清除的排程。
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
