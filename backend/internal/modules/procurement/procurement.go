// Package procurement 是 procurement 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package procurement

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/conversation"
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
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
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
	// Entitlements 讀買家現在有效的方案與額度（*buyer.Module 滿足它）；SubmitTurn 需要，沒給時 SubmitTurn 回錯誤。
	Entitlements EntitlementReader
	// Limits 是每位買家的送出上限，零值取預設（同時進行中 2、每分鐘送出 10）。
	Limits run.Limits
}

// EntitlementReader 是 SubmitTurn 取額度要用的 buyer 方法：每月上限與期間（*buyer.Module 滿足它）。
type EntitlementReader interface {
	Entitlement(ctx context.Context, buyerID int64) (buyer.Entitlement, error)
}

type quotaOf struct{ r EntitlementReader }

func (q quotaOf) TurnQuota(ctx context.Context, buyerID int64) (turn.Quota, error) {
	e, err := q.r.Entitlement(ctx, buyerID)
	if err != nil {
		return turn.Quota{}, err
	}
	return turn.Quota{Limit: e.MonthlyAnswerLimit, PeriodStart: e.PeriodStart, PeriodEnd: e.PeriodEnd}, nil
}

// noQuota 是沒給 Entitlements 時的額度來源：送出一律失敗（組裝錯誤），不當成不限額。
type noQuota struct{}

func (noQuota) TurnQuota(context.Context, int64) (turn.Quota, error) {
	return turn.Quota{}, errors.New("procurement：SubmitTurn 需要 Deps.Entitlements")
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
	turns      turn.Submitter
	canceller  run.Canceller
	hub        *run.Hub
	watcher    run.Watcher
	searcher   search.Starter
	selector   search.Selector
	resumer    turn.Resumer
	snapshots  snapshot.Reader
	convs      conversation.Service
	candidates search.Reader
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
	hub := run.NewHub(d.DB, run.MaxSubscriptions, d.Logger)
	var quota turn.QuotaSource = noQuota{}
	if d.Entitlements != nil {
		quota = quotaOf{d.Entitlements}
	}
	m := &Module{
		dispatcher: run.Dispatcher{DB: d.DB, Ledger: ledger, Executors: executors(d, ledger), Logger: d.Logger},
		submitter:  run.Submitter{DB: d.DB, Ledger: ledger, Jobs: d.Jobs, Limits: d.Limits},
		canceller:  run.Canceller{DB: d.DB, Ledger: ledger, Jobs: d.Jobs},
		hub:        hub,
		watcher:    run.Watcher{DB: d.DB, Hub: hub, Window: WatchWindow},
		retention:  retention.New(d.DB),
		reconciler: run.Reconciler{DB: d.DB, Ledger: ledger, Logger: d.Logger},
		purger:     run.Purger{DB: d.DB, Ledger: ledger},
	}
	m.turns = turn.Submitter{Submitter: m.submitter, Quota: quota}
	m.searcher = search.Starter{Submitter: m.submitter}
	m.selector = search.Selector{DB: d.DB, Now: d.Now}
	m.resumer = turn.Resumer{Submitter: m.submitter}
	m.snapshots = snapshot.Reader{DB: d.DB, Ledger: ledger}
	m.convs = conversation.Service{DB: d.DB, Now: d.Now, Canceller: m.canceller}
	m.candidates = search.Reader{DB: d.DB}
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

// ToggleSelectionInput 是勾選或取消勾選一個候選的輸入：Selected 是目標狀態。
type ToggleSelectionInput = search.SelectInput

// ToggleSelection 把一個候選設成指定的勾選狀態（目標狀態的方法，重送結果相同，不建收據）。需求不屬於買家、對話已刪除、候選集不屬於這個需求或候選不在候選集裡回 NOT_FOUND；
// 候選集不是有效的候選集（需求不在 MATCHING，或不是最新一筆成功搜尋的候選集）回 ABORTED／VERSION_CONFLICT，conflictTarget 是 candidate_set。
func (m *Module) ToggleSelection(ctx context.Context, in ToggleSelectionInput) error {
	return m.selector.Select(ctx, in)
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
	// SlotKey 是需求欄位的代碼（proto enum 值名去掉 SLOT_KEY_ 前綴）。
	SlotKey = model.SlotKey
)

// GetRuntimeSnapshot 回傳買家自己的對話的快照：訊息、目前需求的 run（新到舊最多 20 筆）、進行中的 run、待回答的中斷（含處理中）與最新的發布結果，在同一份快照內讀齊。
// 對話不存在、已刪除或屬於別的買家回 NOT_FOUND。ctx 不可已在 transaction 內。
func (m *Module) GetRuntimeSnapshot(ctx context.Context, buyerID int64, conversationID uuid.UUID) (RuntimeSnapshot, error) {
	return m.snapshots.Get(ctx, buyerID, conversationID)
}

// WatchWindow 是一條 WatchRun 串流的最長時間（Vercel 外部 rewrite 的上限），到了正常結束，client 以最後的 cursor 接續。
const WatchWindow = 100 * time.Second

// SubmitTurnInput 是送出一則訊息的輸入：RequestID 為 nil 表示新建需求（連同它的對話），ClientRequestID 是冪等鍵。
type SubmitTurnInput = turn.SubmitInput

// TurnSubmitted 是送出的結果：需求、對話與這一輪的 run。
type TurnSubmitted = turn.Submitted

// SubmitTurn 受理買家的一則訊息（A9-D21）：解析或新建需求與對話、冪等建 TURN run、扣一次額度、排入 job，立即回傳；這一輪的過程與結果從 WatchRun 取得。
// 訊息去頭尾空白後為空或超過 2000 字元回 INVALID_ARGUMENT（欄位 Message）；指定的需求不屬於買家回 NOT_FOUND；
// 同需求進行中回 PROCUREMENT_RUN_IN_PROGRESS；買家上限回 RESOURCE_EXHAUSTED；額度用完回 RESOURCE_EXHAUSTED／AGENT_MONTHLY_QUOTA_EXCEEDED。
// 重送同一個 ClientRequestID 回第一次的需求、對話與 run，不再扣額度。同需求等待輸入的 run 被這一輪取代（A9-D23(2)）。
func (m *Module) SubmitTurn(ctx context.Context, in SubmitTurnInput) (TurnSubmitted, error) {
	return m.turns.Submit(ctx, in)
}

// 取消結果（CancelRunOutcome.Disposition）。
const (
	CancelDispositionRequested        = run.DispositionRequested
	CancelDispositionAlreadyCancelled = run.DispositionAlreadyCancelled
	CancelDispositionAlreadyTerminal  = run.DispositionAlreadyTerminal
)

// CancelRunOutcome 是取消命令的結果：Status 是命令之後 run 的狀態代碼，Disposition 是 REQUESTED、ALREADY_CANCELLED 或 ALREADY_TERMINAL。
type CancelRunOutcome = run.CancelOutcome

// CancelRun 取消買家自己的 run（冪等）；不是這位買家的 run 回 NOT_FOUND。買家取消不退額度（A9-D6）。
func (m *Module) CancelRun(ctx context.Context, buyerID int64, runID uuid.UUID) (CancelRunOutcome, error) {
	return m.canceller.Cancel(ctx, buyerID, runID)
}

// RunEvent 是 run 事件串流上的一個事件：Cursor 是它的位置（不透明字串），Payload 是 `{"<事件名>": {...}}` 的 proto JSON
// （WatchRunResponse 的 oneof 成員，不含 cursor 與 event_time）。
type RunEvent struct {
	Cursor     string
	Payload    []byte
	OccurredAt time.Time
}

// WatchRun 從 cursor 之後依序呼叫 send，直到送出結尾事件（finished）、WatchWindow 到期或 ctx 結束；run 不屬於買家回 NOT_FOUND，
// cursor 格式不對回 INVALID_ARGUMENT（欄位 cursor），早於保留範圍回 REPLAY_CURSOR_EXPIRED、超前回 REPLAY_CURSOR_AHEAD。
// 需要 RunHub 在跑（serve 程序啟動時開）。
func (m *Module) WatchRun(ctx context.Context, buyerID int64, runID uuid.UUID, cursor string, send func(RunEvent) error) error {
	return m.watcher.Watch(ctx, buyerID, runID, cursor, func(e run.Event) error {
		return send(RunEvent{Cursor: run.Cursor(e.Seq), Payload: e.Payload, OccurredAt: e.OccurredAt})
	})
}

// 讀取模型與對話管理的型別。
type (
	RequestSnapshot      = model.RequestSnapshot
	RequirementSlots     = model.RequirementSlots
	ItemResolution       = model.ItemResolution
	ConversationSummary  = model.ConversationSummary
	ConversationPage     = model.ConversationPage
	ConversationMessages = model.ConversationMessages
	CandidateSearch      = model.CandidateSearch
	Candidate            = model.Candidate
	MatchedFact          = model.MatchedFact
	UnverifiableFact     = model.UnverifiableFact
)

// MaxConversationTitleRunes 是對話標題的長度上限（字元數）。
const MaxConversationTitleRunes = conversation.MaxTitleRunes

// GetSnapshot 回傳買家自己的需求的快照（以需求為鍵）：現行版本的欄位、缺漏的必填欄位、品項消歧卡的分類名稱、run、待回答的中斷與最新的發布結果
// （含由確認快照重算的 hash）。需求不存在、不屬於買家或它的對話已被刪除回 NOT_FOUND。ctx 不可已在 transaction 內。
func (m *Module) GetSnapshot(ctx context.Context, buyerID int64, requestID uuid.UUID) (RequestSnapshot, error) {
	return m.snapshots.GetByRequest(ctx, buyerID, requestID)
}

// GetConversationMessages 回傳買家自己的對話的訊息（由舊到新）、需求狀態與標題；對話不存在、已刪除或不是買家的回 NOT_FOUND。ctx 不可已在 transaction 內。
func (m *Module) GetConversationMessages(ctx context.Context, buyerID int64, conversationID uuid.UUID) (ConversationMessages, error) {
	return m.snapshots.Messages(ctx, buyerID, conversationID)
}

// ListConversations 回傳買家自己未刪除的對話，依 (updated_at, id) 由新到舊的一頁（keyset）。pageSize 為 0 是 20、超過 50 以 50 計、負數回
// INVALID_ARGUMENT（欄位 page_size）；pageToken 不是上一頁發出的 token 回 INVALID_ARGUMENT（欄位 page_token）；NextPageToken 為空字串是最後一頁。
func (m *Module) ListConversations(ctx context.Context, buyerID int64, pageSize int32, pageToken string) (ConversationPage, error) {
	return m.convs.List(ctx, buyerID, pageSize, pageToken)
}

// RenameConversation 改對話標題（目標狀態的方法，重送結果相同）。標題去頭尾空白後為空、超過 MaxConversationTitleRunes 個字元或含換行回
// INVALID_ARGUMENT（欄位 title）；對話不存在、已刪除或不是買家的回 NOT_FOUND。
func (m *Module) RenameConversation(ctx context.Context, buyerID int64, conversationID uuid.UUID, title string) error {
	return m.convs.Rename(ctx, buyerID, conversationID, title)
}

// DeleteConversation 刪除對話，同一個 transaction 取消它需求還沒結束的 run（走 CancelRun 的路徑，買家取消不退額度）；已發布的詢價不受影響。
// 已刪除的重送回成功；不存在或不是買家的回 NOT_FOUND。
func (m *Module) DeleteConversation(ctx context.Context, buyerID int64, conversationID uuid.UUID) error {
	return m.convs.Delete(ctx, buyerID, conversationID)
}

// GetCandidateSearch 回傳買家自己的候選搜尋 run 的狀態與結果。run 不存在、不是買家的、不是 CANDIDATE_SEARCH 或不屬於這個需求回 NOT_FOUND；
// 只有 SUCCEEDED 有候選（事實依需求現行版本與 locale 讀取時組出），FAILED 帶失敗原因代碼。ctx 不可已在 transaction 內。
func (m *Module) GetCandidateSearch(ctx context.Context, buyerID int64, requestID, runID uuid.UUID, locale uitext.Locale) (CandidateSearch, error) {
	return m.candidates.Get(ctx, buyerID, requestID, runID, locale)
}
