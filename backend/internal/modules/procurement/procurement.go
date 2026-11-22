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
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/extract"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/retention"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/search"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/snapshot"
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

// SellerStores 是候選搜尋過濾店家要用的 seller 方法：確認營業中、排除封鎖這位買家的店（*seller.Module 滿足它）。
type SellerStores interface {
	ExposableStores(ctx context.Context, storeIDs []int64) ([]seller.ExposableStore, error)
	StoresBlockingBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]bool, error)
}

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct {
	DB     *db.DB
	Now    func() time.Time
	Logger *slog.Logger
	// Sellers 是候選搜尋過濾店家的 seller 方法；與 Search、Taxonomy 一起給才登記 CANDIDATE_SEARCH 的 executor。
	Sellers SellerStores
	// Search 是商品搜尋（分類 ID 或關鍵字，依銷量一頁一頁取），catalog 搬進單體前由 cmd/hawkr 以 internal/legacy 的 product 轉接。
	Search ProductSearcher
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

// ProductSearcher 是候選搜尋的商品搜尋 port；SearchQuery、SearchPage、SearchHit 是它的輸入與輸出。
type ProductSearcher = candidate.Searcher

type (
	// SearchQuery 是搜尋的輸入：分類 ID 與關鍵字二擇一。
	SearchQuery = candidate.Query
	// SearchPage 是依序的一頁命中，Last 表示後面沒有更多頁。
	SearchPage = candidate.Page
	// SearchHit 是命中的一件上架商品（商品 ID 與店 ID）。
	SearchHit = candidate.Hit
)

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
	searcher   search.Starter
	resumer    turn.Resumer
	snapshots  snapshot.Reader
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
	m := &Module{
		dispatcher: run.Dispatcher{DB: d.DB, Ledger: ledger, Executors: executors(d, ledger), Logger: d.Logger},
		submitter:  run.Submitter{DB: d.DB, Ledger: ledger, Jobs: d.Jobs, Limits: d.Limits},
		canceller:  run.Canceller{DB: d.DB, Ledger: ledger, Jobs: d.Jobs},
		hub:        run.NewHub(d.DB, run.MaxSubscriptions, d.Logger),
		retention:  retention.New(d.DB),
		reconciler: run.Reconciler{DB: d.DB, Ledger: ledger, Logger: d.Logger},
		purger:     run.Purger{DB: d.DB, Ledger: ledger},
	}
	m.searcher = search.Starter{Submitter: m.submitter}
	m.resumer = turn.Resumer{Submitter: m.submitter}
	m.snapshots = snapshot.Reader{DB: d.DB, Ledger: ledger}
	return m
}

// executors 在呼叫端給的登記之外補上 TURN、CANDIDATE_SEARCH 與 CLARIFICATION_RESUME 的 executor。
// TURN 需要 LLM 與 Taxonomy；CANDIDATE_SEARCH 只需要 Search、Sellers 與 Taxonomy，不依賴 LLM；CLARIFICATION_RESUME 只需要 Taxonomy（回答不呼叫模型）。缺依賴是組裝錯誤，panic 訊息指名缺的欄位。
func executors(d Deps, ledger run.Ledger) map[string]Executor {
	hasLLM := d.LLM.LLM() != nil
	switch {
	case hasLLM && d.Taxonomy == nil:
		panic("procurement.New：Deps.LLM 要與 Deps.Taxonomy 一起給")
	case d.Search != nil && d.Sellers == nil:
		panic("procurement.New：Deps.Search 要與 Deps.Sellers 一起給")
	case d.Search != nil && d.Taxonomy == nil:
		panic("procurement.New：Deps.Search 要與 Deps.Taxonomy 一起給")
	case d.Taxonomy == nil:
		return d.Executors
	}
	out := make(map[string]Executor, len(d.Executors)+3)
	for k, v := range d.Executors {
		out[k] = v
	}
	rs := resolver.New(d.Taxonomy, resolver.EmbeddedSynonyms(), d.Now)
	if _, ok := out[run.RunTypeTurn]; !ok && hasLLM {
		shape := extract.ShapeSchema // 停用的模型沒有 provider：呼叫一律回 ErrDisabled，形狀無關
		if d.LLM.Enabled() {
			var err error
			if shape, err = extract.ShapeFor(d.LLM.Provider()); err != nil {
				panic(fmt.Sprintf("procurement.New：%v", err))
			}
		}
		out[run.RunTypeTurn] = turn.Executor{
			DB: d.DB, Ledger: ledger, Extractor: extract.New(d.LLM.LLM(), d.LLM.Name(), shape),
			Resolver: rs, Now: d.Now, Logger: d.Logger,
		}
	}
	if _, ok := out[run.RunTypeResume]; !ok {
		out[run.RunTypeResume] = turn.AnswerExecutor{DB: d.DB, Ledger: ledger, Resolver: rs, Now: d.Now, Logger: d.Logger}
	}
	if _, ok := out[run.RunTypeCandidateSearch]; !ok && d.Search != nil {
		out[run.RunTypeCandidateSearch] = search.Executor{
			DB: d.DB, Ledger: ledger, Searcher: d.Search, Open: openStores{d.Sellers}, Blocks: blockingStores{d.Sellers},
			Resolver: rs, Now: d.Now, Logger: d.Logger,
		}
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

// blockingStores 把 seller.StoresBlockingBuyer 轉成 candidate.BlockedStores，依 seller 的批次上限分批；任一批失敗整個失敗（fail-closed）。
type blockingStores struct{ sellers SellerStores }

var _ candidate.BlockedStores = blockingStores{}

func (b blockingStores) StoreIDsBlockingBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]struct{}, error) {
	out := make(map[int64]struct{})
	for start := 0; start < len(storeIDs); start += seller.MaxBatchSize {
		end := min(start+seller.MaxBatchSize, len(storeIDs))
		blocked, err := b.sellers.StoresBlockingBuyer(ctx, buyerID, storeIDs[start:end])
		if err != nil {
			return nil, fmt.Errorf("procurement：讀封鎖這位買家的店家：%w", err)
		}
		for id, isBlocked := range blocked {
			if isBlocked {
				out[id] = struct{}{}
			}
		}
	}
	return out, nil
}

// StartCandidateSearchInput 是開始候選搜尋的輸入：ClientRequestID 是冪等鍵（以 run 的 (buyer, client_request_id) 為準）。
type StartCandidateSearchInput = search.StartInput

// CandidateSearchStarted 是開始搜尋的結果：run、候選集與 run 當下的狀態（QUEUED 等）。
type CandidateSearchStarted = search.Started

// StartCandidateSearch 開始一次候選搜尋（A9-D29）：不扣額度、不改需求狀態；成功的搜尋才把需求轉 MATCHING。
// 需求不屬於買家回 NOT_FOUND；需求不在 REQUIREMENT_READY 或 MATCHING 回 REQUIREMENT_NOT_READY；同需求已有進行中的 run 回 PROCUREMENT_RUN_IN_PROGRESS。
// 重送同一個 client_request_id 回第一次的 run 與候選集（內容不同回 INVALID_ARGUMENT）。
func (m *Module) StartCandidateSearch(ctx context.Context, in StartCandidateSearchInput) (CandidateSearchStarted, error) {
	return m.searcher.Start(ctx, in)
}

// ResumeRunInput 是回答一個中斷的輸入：OptionID 與 FreeText 恰一個（ClarificationResponse）。
type ResumeRunInput = turn.ResumeInput

// ResumeRunResult 是 ResumeRun 的結果：回答 run 的 ID 與它當下的狀態；結果由 WatchRun 取得，不在這裡回（A9-D30(2)）。
type ResumeRunResult = turn.Resumed

// ResumeRun 回答買家自己的中斷，排入回答 run 後立即回傳（A9-D26、A9-D30）。不是這位買家的中斷回 NOT_FOUND；
// 回答形狀不合（OptionID 與 FreeText 不是恰一個、文字只有空白或超過 2000 字元、選項不在卡上）回 INVALID_ARGUMENT 並指出欄位；
// 中斷已回答回 FAILED_PRECONDITION／APPROVAL_ALREADY_RESOLVED；中斷已取消或 run 不在等待輸入回 RUN_NOT_ACTIVE；同需求已有進行中的 run 回 PROCUREMENT_RUN_IN_PROGRESS；
// 買家上限回 RESOURCE_EXHAUSTED；不扣額度。重送同一個 ClientRequestID 回同一個回答 run。答案在解析品項時被拒，回答 run 以 ANSWER_REJECTED 失敗，中斷維持 PENDING。
func (m *Module) ResumeRun(ctx context.Context, in ResumeRunInput) (ResumeRunResult, error) {
	return m.resumer.Resume(ctx, in)
}

// RuntimeSnapshot 是 GetRuntimeSnapshot 的讀取模型，以及它的元素型別。
type (
	RuntimeSnapshot     = model.RuntimeSnapshot
	ConversationMessage = model.ConversationMessage
	RecordedField       = model.RecordedField
	RunSummary          = model.RunSummary
	OpenConfirmation    = model.OpenConfirmation
	ClarifyQuestion     = model.ClarifyQuestion
	QuestionOption      = model.QuestionOption
	PublishedRfq        = model.PublishedRfq
)

// GetRuntimeSnapshot 回傳買家自己的對話的快照：訊息、目前需求的 run（新到舊最多 20 筆）、進行中的 run、待回答的中斷（含處理中）與最新的發布結果，在同一份快照內讀齊。
// 對話不存在、已刪除或屬於別的買家回 NOT_FOUND。ctx 不可已在 transaction 內。
func (m *Module) GetRuntimeSnapshot(ctx context.Context, buyerID int64, conversationID uuid.UUID) (RuntimeSnapshot, error) {
	return m.snapshots.Get(ctx, buyerID, conversationID)
}
