package usecase

import (
	"context"
	"errors"
	"os"
	"runtime/debug"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	procurementdomain "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"go.uber.org/zap"
)

var ErrRequirementNotReady = errors.New("procurement: request is not REQUIREMENT_READY, cannot start candidate search")

const (
	candidateSearchLeaseTTL = 30 * time.Second
	candidateSearchBudget   = 10 * time.Second // doc 55 §9.3：候選搜尋（雙 lane）< 10s
	policyVersionV1         = 1
)

// LaneSearcher 是單一 lane 的候選搜尋能力，PLATFORM／EXTERNAL 各自實作（Task 6/7）。
type LaneSearcher interface {
	Search(ctx context.Context, slots procurementdomain.Slots) ([]repository.Candidate, error)
}

// CandidateSearchStore 是候選搜尋編排需要的持久層能力，由 repository.Repo 實作。
type CandidateSearchStore interface {
	FindCandidateSet(ctx context.Context, requestRevisionID int64, policyVersion int) (repository.CandidateSet, error)
	CreateCandidateSet(ctx context.Context, in repository.CreateCandidateSetInput) (repository.CandidateSet, error)
	// MarkCandidateSetRunning 回傳的 bool 是 claim-check 結果（true = 這個 worker 真的
	// 認領到這個 run）；false 代表 row 已終態或已被別人搶走，呼叫端必須放棄整趟搜尋。
	MarkCandidateSetRunning(ctx context.Context, id, fencingToken int64) (bool, error)
	// FinalizeCandidateSet 回傳的 bool 是「終態真的被寫進去了嗎」：false + nil error 代表
	// 這個 run 已經被 reaper 收走或被別人終結過，這次寫入被防呆條件丟掉（見
	// repository.FinalizeCandidateSet）。呼叫端靠它決定要不要記 procurement_run_total。
	// 第二個回傳值是實際寫入的 run 終態，直接當 metric 的 label——不在這一層重新推導，
	// 否則 metric 上的終態會與 DB 裡的終態各自演化。bool 為 false 時它是空字串。
	FinalizeCandidateSet(ctx context.Context, in repository.FinalizeCandidateSetInput) (bool, string, string, error)
	GetCandidateSetWithCandidates(ctx context.Context, candidateSetID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error)
	// GetCandidateSetRunWithCandidates 用 **run 身分**查（對外 GET .../candidate-searches/{runId}）。
	// 與上面那支的差別只在查詢鍵，見 repository 側的函式註解。
	GetCandidateSetRunWithCandidates(ctx context.Context, runID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error)
	RetryCandidateSet(ctx context.Context, id int64, leaseOwner string, leaseTTL time.Duration) (repository.CandidateSet, error)
	// ListSelectedCandidateIDs 供 GetRun 組裝候選卡的 selected 欄位使用。
	ListSelectedCandidateIDs(ctx context.Context, candidateSetID int64) (map[int64]bool, error)

	// CreateLedgerRunForCandidateSearch 是這個 domain 既有 lease/fencing claim
	// 之外，額外把這個 run 投影進新的 run ledger（procurement_agent_runs）——
	// candidate_set 仍是這個 domain 的權威狀態來源，這筆 ledger row 只是讓
	// CANDIDATE_SEARCH 這個 run type 也能被 Task 7-9（TURN/ANSWER/REWIND）共用
	// 的 GET replay／runtime-snapshot 端點統一讀到。不經過 ClaimRun 的冪等判
	// 斷——那層冪等已經由 procurement_candidate_sets 自己的唯一索引擋掉。
	CreateLedgerRunForCandidateSearch(ctx context.Context, set repository.CandidateSet) error
	// TerminalizeRun 供 run() 在 FinalizeCandidateSet 真的寫入終態後，額外把
	// 同一個終態投影進 ledger（完整介面契約見 repository.TerminalizeRun；這裡
	// 只是同一個介面的另一個消費端，同 usecase.RunClaimStore 的既有用法）。
	TerminalizeRun(ctx context.Context, in repository.TerminalizeRunInput) (bool, error)
	// AppendRunEvent 供 candidateSearchRail 把 Activity 步驟列寫進 ledger
	// （T23）。這條 run 是脫離 ADK graph 的背景 goroutine，沒有 session.Event
	// 可以 emit，直接寫 ledger 是它唯一的產出管道；買家端靠既有的
	// WatchRunEvents／GET /runs/{runId}/events 讀回來（那支不受 capability
	// flag 管轄，見 grpc server 的 runStore 欄位註解）。
	AppendRunEvent(ctx context.Context, in repository.AppendRunEventInput) (repository.RunEvent, error)
	// UpsertRunArtifact 產製一版 typed artifact 並在同一交易 append
	// ARTIFACT_UPSERTED（doc 57 §4.5／§6.2，完整契約見 repository 側）。這條 run
	// 用它產製 CANDIDATE_RESULTS，見 candidate_results_artifact.go。
	UpsertRunArtifact(ctx context.Context, in repository.UpsertRunArtifactInput) (repository.RunArtifact, error)
}

type CandidateSearchUseCase struct {
	store    CandidateSearchStore
	platform LaneSearcher
	external LaneSearcher
	hostname string
	// newID 鑄 Activity rail 每一列的 opaque row id，沿用既有 TSID 產生器慣例
	// （同 graph_nodes.go 那幾個 node 的 newID 參數，fx 注入 tsid.NewGenerator）。
	newID func() int64
}

func NewCandidateSearchUseCase(store CandidateSearchStore, platform, external LaneSearcher, newID func() int64) *CandidateSearchUseCase {
	host, err := os.Hostname()
	if err != nil {
		host = "ai-service"
	}
	return &CandidateSearchUseCase{store: store, platform: platform, external: external, hostname: host, newID: newID}
}

type StartSearchInput struct {
	RequestID         int64
	RequestRevisionID int64
	BuyerID           identity.BuyerID
	RequirementReady  bool
	Slots             procurementdomain.Slots
}

// StartSearchResult 帶 **兩個** 身分：RunID 是這次執行、CandidateSetID 是這批候選。
// 兩者刻意是不同的值（設計書 57 §1.2-3），呼叫端不得拿其中一個當另一個用。
//
// CandidateSetID 一起回是 §5.3 的要求：RunAccepted envelope 要帶 allowlisted domainRef
// 指向 candidateSetId。少了它，前端在 accept 當下只有 runId，得等第一次 GET 才知道自己
// 在等哪一批候選。
type StartSearchResult struct {
	RunID          int64
	CandidateSetID int64
	Status         string
}

// StartSearch 冪等地建立（或回傳既有）候選搜尋 run，並非同步派出背景 goroutine 執行——
// HTTP handler 立刻拿到 runId 回應，實際搜尋由 SSE／GET 端點輪詢同一張表得知進度
// （doc 55 §9.2 無 in-memory 跨請求狀態的 NFR，見計畫 Design Decisions #6）。
//
// 冪等鍵命中但那個 run 已經是 FAILED（典型情境：worker 死掉、lease 過期、Task 5 的
// lazy reaper 把它標成 WORKER_LOST 並把 request 退回 REQUIREMENT_READY）時，不能直接
// 原樣回傳那筆死掉的紀錄——(request_revision_id, policy_version) 這個冪等鍵沒有變，
// 會讓買家永遠卡在同一筆 FAILED 上、再也叫不出新的 run，跟計畫 Design Decision #7
// 講的「買家重新呼叫 POST candidate-searches 即可拿到新的 run」矛盾。這裡改用
// RetryCandidateSet 把同一列從 FAILED 轉回 QUEUED（同一個 candidate_set.id 繼續當
// run id，不新增第二列，見 Design Decision #1／#7），再重新派工。判斷只看 status，
// 不看 failureReason——不論成因是 WORKER_LOST 還是 LANE_ALL_FAILED，「FAILED」本身就是
// 終止狀態，一律視為可重試，不細分成因。
func (uc *CandidateSearchUseCase) StartSearch(ctx context.Context, in StartSearchInput) (StartSearchResult, error) {
	if existing, err := uc.store.FindCandidateSet(ctx, in.RequestRevisionID, policyVersionV1); err == nil {
		if existing.Status == codemeta.ProcurementRunStatusFailed.Value {
			if retried, retryErr := uc.store.RetryCandidateSet(ctx, existing.ID, uc.hostname, candidateSearchLeaseTTL); retryErr == nil {
				// 重試配到的是一個**新的** run 身分（RetryCandidateSet 在交易內換 run_id），
				// 所以跟新建路徑一樣要為它投影一列 ledger run——同一個 candidate set 底下
				// 的第二列 agent_runs（1:N）。少了這一句，買家拿到的 runId 在 ledger 上
				// 根本不存在：SSE 訂閱不到任何事件，終態也寫不進去。
				//
				// 排在 dispatch **之前**：背景 goroutine 跑完會對這個 run 寫終態，run row
				// 必須先存在。錯誤只記 log 不阻斷（同新建路徑的取捨：ledger 是投影不是
				// SSOT，不能讓觀測層拖垮既有功能）。
				uc.createLedgerRunProjection(ctx, retried)
				uc.dispatch(ctx, retried, in.Slots)
				return startSearchResultOf(retried), nil
			}
			// RetryCandidateSet 失敗：輸給另一個併發的重試呼叫、或這一列已經不再是
			// FAILED（另一個呼叫先一步重試並跑完了）。兩者都是良性的 race 落敗，不視
			// 為錯誤，但**必須重讀一次再回覆**。
			//
			// 舊行為是直接回 existing 那個快照，理由寫著「買家頂多看到稍舊的狀態」。
			// 那個結論成立的前提是 **run 身分不會變**——重試沿用同一個 run_id 時，稍舊
			// 的快照裡唯一過期的只有 status 這種下一次輪詢就會修正的欄位。現在一次重試
			// 會換一個新的 run 身分，同一個快照裡過期的就變成**身分本身**：落敗方會把
			// 一個已被贏家取代的 runId 交給前端，那個 client 於是整段訂閱在錯誤的 run
			// 上——它會讀到前一次嘗試的終態事件並顯示「搜尋失敗」，而真正在跑的是贏家
			// 那個 run。重讀讓落敗方回覆贏家剛寫進去的當前身分。
			//
			// 重讀本身再失敗才退回 existing：那是「稍舊的快照」與「完全沒有回應」之間
			// 的取捨，前者仍然可用（下一次 POST 會自我修正）。
			if refreshed, refreshErr := uc.store.FindCandidateSet(ctx, in.RequestRevisionID, policyVersionV1); refreshErr == nil {
				return startSearchResultOf(refreshed), nil
			}
		}
		return startSearchResultOf(existing), nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return StartSearchResult{}, err
	}

	if !in.RequirementReady {
		return StartSearchResult{}, ErrRequirementNotReady
	}

	set, err := uc.store.CreateCandidateSet(ctx, repository.CreateCandidateSetInput{
		RequestID: in.RequestID, RequestRevisionID: in.RequestRevisionID, BuyerID: in.BuyerID,
		PolicyVersion: policyVersionV1, LeaseOwner: uc.hostname, LeaseTTL: candidateSearchLeaseTTL,
	})
	if err != nil {
		return StartSearchResult{}, err
	}

	// 額外建立 ledger run 投影（claim 用 candidate_set 自己的 run_id 當 domain
	// ref，不重新產生冪等鍵——這筆 ledger run 沒有自己獨立的 client_request_id
	// 語意，直接呼叫 CreateLedgerRunForCandidateSearch 而不經過 ClaimRun 的冪等
	// 判斷，因為冪等已經由 candidate_set 自己那層擋掉了）。
	uc.createLedgerRunProjection(ctx, set)

	uc.dispatch(ctx, set, in.Slots)

	return startSearchResultOf(set), nil
}

// createLedgerRunProjection 為「這一次嘗試」投影一列 ledger run，是新建與重試兩條路徑
// 共用的唯一投影點。
//
// 兩條路徑都要投影，因為兩者都是一次**新的執行**、各自持有不同的 run 身分
// （one run = one immutable execution attempt，見 repository.RetryCandidateSet）。
// 同一個 candidate set 底下因此會有 N 列 agent_runs，全部以 domain_ref 指回它。
//
// 寫 ledger 失敗不阻斷既有候選搜尋流程——ledger 是投影不是 SSOT，記 log 供之後排查即可，
// 不能讓一個新增的觀測層拖垮既有功能。**代價要說清楚**：這一次投影失敗的話，買家拿到的
// runId 在 ledger 上不存在，SSE 訂閱不到事件、終態也無處可寫，那個 run 對串流端等同從未
// 發生；候選搜尋本身仍會正常跑完並寫進 candidate_set（買家輪詢讀得到結果）。
func (uc *CandidateSearchUseCase) createLedgerRunProjection(ctx context.Context, set repository.CandidateSet) {
	if err := uc.store.CreateLedgerRunForCandidateSearch(ctx, set); err != nil {
		pkglogger.Error(ctx, "procurement: failed to create ledger run projection for candidate search",
			zap.Error(err), zap.Int64("candidate_set_id", set.ID), zap.Int64("run_id", set.PublicRunID()))
	}
}

// startSearchResultOf 是 StartSearch 三條回傳路徑（重試、冪等命中、新建）的唯一組裝點。
// 三處各自手寫過一次 `RunID: xxx.ID`，那正是 runId／candidateSetId 混為一談的來源；收成
// 一個函式後，兩個身分的取法只有一個地方要看對。
func startSearchResultOf(set repository.CandidateSet) StartSearchResult {
	return StartSearchResult{RunID: set.PublicRunID(), CandidateSetID: set.ID, Status: set.Status}
}

// dispatch 派出背景 goroutine 執行 uc.run，供「新建 run」與「重試一個死掉的 run」共用
// 同一套背景執行與 panic-recovery 邏輯——兩者除了「這個 candidate_set 從哪裡來」以外
// 沒有任何差異。背景執行不可直接沿用呼叫端的 ctx（HTTP request 回應後 ctx 會被取消）；
// 改用 context.WithoutCancel(ctx) 衍生一個與 request 生命週期脫鉤、但仍保留 trace span
// 與 request-scoped logger 的 context，再疊上獨立的預算上限（doc 55 §4.1「長任務不綁在
// 單一 HTTP 連線生命週期上」）——issue #3519 slice B Task 5：先前用 context.Background()
// 連 trace 一起丟，而背景搜尋正是最容易卡住、最需要 trace 的一段。
//
// panic 防護改用 pkg/syncutil.SafeGo，不再手刻 defer/recover——這個 goroutine 在 HTTP
// handler 回應之後才真正執行大部分工作，沒有任何上游的 request-scoped recover() 能接住
// 它，uc.run 或它呼叫的 lane searcher 若 panic，沒有這層防護會直接把整個 ai-service
// process 帶走。不在這裡把 candidate_set 標成 FAILED——lease 到期後 lazy reaper（Task 5，
// GetCandidateSetWithCandidates 讀取時觸發）會自然把它收斂成 FAILED/WORKER_LOST，不需要
// recover handler 自己再做一次容易寫錯 fencing_token 的終態寫入。SafeGo 的預設 panic log
// 不含 candidate_set_id/request_id 這兩個結構化欄位（見本計畫 Global Constraints）——完整
// stack trace 仍會指出是這個 dispatch 內的呼叫 panic，reaper 的自我修復也不依賴 log 內容。
func (uc *CandidateSearchUseCase) dispatch(ctx context.Context, set repository.CandidateSet, slots procurementdomain.Slots) {
	// ⚠️ 同一個 WithTx-leak 隱患見 run_dispatch.go RunDispatcher.Dispatch 的
	// 完整說明：context.WithoutCancel(ctx) 保留 context values，若未來從
	// pkg/database.WithTx 的 closure 內呼叫本函式，呼叫端的 pgx.Tx 會被
	// database.GetDBTX 解析進這個背景 goroutine——不可從交易內呼叫 dispatch。
	detached := context.WithoutCancel(ctx)
	syncutil.SafeGo(detached, func() {
		bgCtx, cancel := context.WithTimeout(detached, candidateSearchBudget)
		defer cancel()
		uc.run(bgCtx, set, slots)
	})
}

// run 執行雙 lane 搜尋並寫回終態；獨立成不經過 goroutine 派送的方法，讓測試可以同步呼叫、
// 不必依賴 goroutine 排程時序（見 candidate_search_test.go）。
func (uc *CandidateSearchUseCase) run(ctx context.Context, set repository.CandidateSet, slots procurementdomain.Slots) {
	// MarkCandidateSetRunning 是 claim-check（QUEUED + 對的 fencing_token 才轉 RUNNING），
	// 兩種失敗都必須就地放棄：
	//   err != nil   —— DB 出錯，什麼都不知道；
	//   claimed == false —— 沒認領到，row 已終態或已被 reaper／另一次重試搶走
	//     （例如 lease 過期被 reap，或買家重新呼叫 POST candidate-searches 觸發
	//      RetryCandidateSet，fencing_token 已經 +1）。
	// 早期版本只檢查 err，把「0 rows affected」當成成功繼續往下跑——這代表對一個永遠寫不
	// 回去的 run 做完整趟雙 lane 搜尋（兩次真實的跨服務 gRPC 呼叫：product-service
	// 與 seller-service），結果最後一定被 FinalizeCandidateSet 的同一組條件丟掉，純粹白燒資源，還會讓
	// 下方的 procurement_run_total 多算一筆不存在的 run。
	//
	// Fire-and-forget by design（同 FinalizeCandidateSet 下方那句的理由）：放棄就是放棄，
	// 不重試、不把 request 打回任何狀態——lease 到期後 reaper 會自然收斂。但「不重試」不
	// 等於「不留痕跡」，這句 log 是診斷「run 卡住不動」時唯一的線索來源。
	claimed, err := uc.store.MarkCandidateSetRunning(ctx, set.ID, set.FencingToken)
	if err != nil || !claimed {
		pkglogger.Error(ctx, "procurement: MarkCandidateSetRunning failed, abandoning run",
			zap.Error(err), zap.Bool("claimed", claimed),
			zap.Int64("candidate_set_id", set.ID), zap.Int64("request_id", set.RequestID),
			zap.Int64("fencing_token", set.FencingToken))
		return
	}

	// Activity 步驟列（T23）。claim 成功之後才開始寫：claim 失敗代表這個 worker
	// 不是這個 run 的擁有者，寫出去會在買家畫面上疊出兩份互相打架的進度。
	rail := uc.newCandidateSearchRail(set)
	rail.declarePlan(ctx)
	rail.update(ctx, procurementdomain.StepSearchCandidates, codemeta.ProcurementStepStatusRunning.Value, nil)

	platformCh := make(chan repository.LaneResult, 1)
	externalCh := make(chan repository.LaneResult, 1)

	// lane 名稱一律走 domain 常數，不寫裸字串：這兩個值同時是 metric label
	// （ProcurementCandidateZeroTotal）與 candidate.Lane 的寫入值，而 handler 端的分流
	// 是拿 candidate.Lane 跟同一組常數比對。散落的字面值會讓「改常數」變成一次沒有編譯
	// 錯誤、也沒有測試失敗的靜默分裂。
	// 兩條 lane 各自推進自己的子列。rail.emit 在 lane 的 goroutine 裡呼叫是
	// 安全的：rowIDs 建構後只讀，seq 分配序列化在 AppendRunEvent 自己的交易。
	go func() {
		rail.update(ctx, procurementdomain.StepSearchPlatform, codemeta.ProcurementStepStatusRunning.Value, nil)
		result := uc.searchLane(ctx, set, string(procurementdomain.LanePlatform), uc.platform, slots)
		laneStatus, laneCount := laneRailOutcome(result)
		rail.update(ctx, procurementdomain.StepSearchPlatform, laneStatus, laneCount)
		platformCh <- result
	}()
	go func() {
		rail.update(ctx, procurementdomain.StepSearchExternal, codemeta.ProcurementStepStatusRunning.Value, nil)
		result := uc.searchLane(ctx, set, string(procurementdomain.LaneExternal), uc.external, slots)
		laneStatus, laneCount := laneRailOutcome(result)
		rail.update(ctx, procurementdomain.StepSearchExternal, laneStatus, laneCount)
		externalCh <- result
	}()

	platform := <-platformCh
	external := <-externalCh

	rail.update(ctx, procurementdomain.StepSearchCandidates, parentRailStatus(platform, external), nil)
	rail.update(ctx, procurementdomain.StepSummarize, codemeta.ProcurementStepStatusRunning.Value, nil)

	in := repository.FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: set.RequestID,
		// 這個 run 屬於哪個 revision，決定它還有沒有資格改寫 request.status——
		// 見 repository.FinalizeCandidateSet 註解（買家在這 10 秒背景視窗內送出下一輪
		// turn 時，舊 run 不得把新的 CLARIFYING 蓋回 CANDIDATES_READY／REQUIREMENT_READY）。
		RequestRevisionID: set.RequestRevisionID,
		Platform:          platform,
		External:          external,
	}
	wrote, finalStatus, failureReason, err := uc.store.FinalizeCandidateSet(ctx, in)
	if err != nil {
		rail.update(ctx, procurementdomain.StepSummarize, codemeta.ProcurementStepStatusFailed.Value, nil)
		// 同上：fire-and-forget，不重試（reaper 是安全網），但要留 log。這正是拿掉
		// debug print 之後、診斷「run 卡在 RUNNING 不動」這類問題唯一的線索來源。
		pkglogger.Error(ctx, "procurement: FinalizeCandidateSet failed, run left for lease reaper to reclaim",
			zap.Error(err), zap.Int64("candidate_set_id", set.ID), zap.Int64("request_id", set.RequestID))
		return
	}
	if !wrote {
		// rail 上這一列停在 RUNNING 不再推進——這個 worker 已經不是這個 run 的
		// 擁有者了（見下方說明），它對這一列的終態沒有發言權，硬寫一個
		// SUCCEEDED/FAILED 只會跟真正終結這個 run 的那一方打架。
		//
		// 這個 run 已經被 reaper 收走或被別的呼叫終結過，剛剛那次寫入被防呆條件整段丟掉。
		// 它的終態（若有）早就由真正寫進去的那個人計過了（reaper 那條路徑見
		// repository.reapCandidateSet 的 ProcurementRunTotal("FAILED")）。在這裡照樣計數
		// 等於把一個 zombie worker 被拒絕的寫入算成一次全新的 COMPLETED/PARTIAL/FAILED——
		// 不只多算，還會貼上錯的 status label（這個 worker 以為自己成功了，實際上這個 run
		// 是 WORKER_LOST）。
		return
	}

	// finalStatus 直接用 repository 回報的實際寫入值，不在這裡重新推導——見
	// CandidateSearchStore.FinalizeCandidateSet 的介面註解。同一個值也決定
	// rail 上「整理候選清單」這一列的終態，不另外推導一份。
	observability.RecordProcurementRun(ctx, finalStatus)
	summarizeStatus := codemeta.ProcurementStepStatusFailed.Value
	if finalStatus == codemeta.ProcurementRunStatusSucceeded.Value {
		summarizeStatus = codemeta.ProcurementStepStatusSucceeded.Value
	}
	rail.update(ctx, procurementdomain.StepSummarize, summarizeStatus, nil)

	// CANDIDATE_RESULTS artifact 排在終態投影**之前**：終態事件必須是這個 run
	// ledger 上的最後一筆，artifact 落在它後面等於買家 replay 到終態、收線之後才
	// 收到一個他永遠看不到的事件。
	//
	// 只在 SUCCEEDED 時產製：run 失敗時沒有候選結果可以宣告，硬產一個空的
	// CANDIDATE_RESULTS 等於對前端宣稱「搜尋完成、結果是零家」，而實際上是搜尋
	// 失敗——那是 ERROR_RECOVERY 的職責（該型 producer 尚未落地，不在本次範圍）。
	if finalStatus == codemeta.ProcurementRunStatusSucceeded.Value {
		uc.produceCandidateResultsArtifact(ctx, set, platform, external)
	} else {
		// 失敗時產 ERROR_RECOVERY——同樣排在終態投影之前，理由同上。這一型是
		// CANDIDATE_RESULTS 在失敗路徑上的誠實替代品：不謊稱「搜尋完成、零家結果」，
		// 而是把失敗原因與可否重試明講出來。
		uc.produceErrorRecoveryArtifact(ctx, set, failureReason)
	}

	// 額外把同一個終態投影進 ledger（同 StartSearch 那端 CreateLedgerRunForCandidateSearch
	// 的取捨：ledger 是投影不是 SSOT，這裡失敗不影響 candidate_set 已經寫定的終態，
	// 只記 log）。finalStatus 只會是 SUCCEEDED 或 FAILED（見 FinalizeCandidateSet），
	// 對映到 ledger 的終態 event type 只有這兩種。
	terminalEventType := codemeta.ProcurementLedgerEventTypeRunFinished.Value
	if finalStatus == codemeta.ProcurementRunStatusFailed.Value {
		terminalEventType = codemeta.ProcurementLedgerEventTypeRunFailed.Value
	}
	// 不帶 Version（2026-09-07 決策）：finalStatus 恆為 SUCCEEDED 或 FAILED，走的是
	// TerminalizeRun 的終態分支，守衛是「現在還不是終態」而不是 version。舊版這裡
	// 寫死 Version: 1，前提是「這筆 ledger run 建立之後沒有任何中途寫入會 bump
	// version」——那個前提被兩條路徑打破：RetryCandidateSet 沿用同一個 run_id（已知
	// 並接受），以及 CancelRun 走同一支帶 version 的 query（沒被評估過）。任一發生，
	// CAS 就必然落空、只留下面那行 Warn，該 run 於是永遠沒有終態 ledger event。
	termWrote, termErr := uc.store.TerminalizeRun(ctx, repository.TerminalizeRunInput{
		RunID: set.PublicRunID(), BuyerID: set.BuyerID,
		Status: finalStatus, TerminalEventType: terminalEventType,
		// 成功時 failureReason 是空字串，RunFailedPayload 回字面 `{}`——同一支
		// helper 兩種終態都成立，不需要在這裡分支。
		PublicPayloadJSON: procurementdomain.RunFailedPayload(failureReason),
	})
	switch {
	case termErr != nil:
		pkglogger.Error(ctx, "procurement: failed to terminalize ledger run projection for candidate search",
			zap.Error(termErr), zap.Int64("candidate_set_id", set.ID))
	case !termWrote:
		// 被狀態守衛擋下：這個 run 已經是終態了。
		//
		// 重試**不再**落在這裡：一次重試會配一個新的 run 身分並投影一列新的 ledger
		// run（見 repository.RetryCandidateSet），那一列還沒有終態，寫得進去。現在
		// 剩下的成因是真正的併發——lazy reaper 先一步把這個 run 終態化成
		// WORKER_LOST（repository.reapCandidateSet），而這個 worker 其實還活著並跑
		// 完了。那種情況下先寫進去的那一方才有發言權，這裡只記 log 不重試。
		//
		// ⚠️ 這個分支的判準已從「version 對不上」換成「已經是終態」：前者會把
		// cancel／retry 造成的版本推進誤判成正常狀況並靜默丟棄終態，後者只在
		// run 真的已經結束時才成立。
		pkglogger.Warn(ctx, "procurement: terminalize ledger run projection CAS lost for candidate search, discarded",
			zap.Int64("candidate_set_id", set.ID), zap.Int64("run_id", set.PublicRunID()))
	}
}

// searchLane 執行單一 lane 的搜尋並在自己的 goroutine 內就地 recover panic。
//
// 這一層 recover 不能省，也不能靠 StartSearch 那個外層的 recover 頂替：recover() 只
// 對「跟 defer 同一個 goroutine」的 panic 生效，run() 對兩個 lane 是各自開一個
// `go func(){...}()` 平行執行（fan-out/fan-in，見檔頭 Design Decision #6 的說明），
// 那兩個 goroutine 跟外層 StartSearch 的背景 goroutine 不是同一個——lane searcher（含
// Task 6/7 呼叫 product-service／seller-service 查詢的程式碼）裡的 panic 會直接跳過
// 外層那個 recover、把整個 process 帶走。此外，若這裡不回送任何結果，run() 對
// platformCh／externalCh 的阻塞式讀取會永遠收不到值——不是 10s bgCtx 超時就能救回來的
// goroutine leak，因為那個讀取沒有 select ctx.Done()。所以 recover 到的 panic 一律
// 轉成 FAILED 的 LaneResult 送回 channel，讓 run() 能正常走到 finalize，另一條 lane
// 也不會被拖累（同 TestRun_OneLaneFails_OtherStillFinalizes 的精神）。
func (uc *CandidateSearchUseCase) searchLane(ctx context.Context, set repository.CandidateSet, lane string, searcher LaneSearcher, slots procurementdomain.Slots) (result repository.LaneResult) {
	defer func() {
		if r := recover(); r != nil {
			// 先賦值再記 log 是 best-effort 的防禦性順序，不是一道真的防線——這裡要說清楚，
			// 免得未來的讀者高估它：如果這個 defer 內部自己 panic（例如某個 exotic 的 panic
			// value 讓 zap encoder 爆掉），第二個 panic 會直接往外傳出整個函式，`result` 是
			// 不是已經賦值都無所謂，caller 那句 `platformCh <- laneOutcome{result: ...}`
			// 根本不會執行，run() 的阻塞式讀取就永遠收不到值。那是已知且未緩解的殘留風險
			// （跟任何 goroutine 內未 recover 的 panic 一樣：整個 process 會被帶走）。
			//
			// 這個順序仍然值得維持：它零成本，而且對「未來某個版本的 defer 內容有辦法在
			// 中途正常收尾／自行 recover」那種情況是對的——真正撐住這條路徑的是「這裡有
			// recover 且會回送 FAILED」本身，不是賦值與記 log 的先後。
			result = repository.LaneResult{Status: codemeta.ProcurementLaneStatusFailed.Value}
			// ctx（本函式的參數），不是 context.Background()：這個 defer 與
			// searcher.Search 同一個 goroutine、函式尚未返回，ctx 仍然有效——
			// 用它記 log 讓這條 panic 訊息帶著呼叫鏈的 trace span，而不是在
			// 最需要診斷資訊的這一刻從 trace 上消失（issue #3519 slice B Task 5）。
			pkglogger.Error(ctx, "procurement: panic recovered in lane search goroutine",
				zap.String("lane", lane),
				zap.Any("panic", r),
				zap.Int64("candidate_set_id", set.ID),
				zap.Int64("request_id", set.RequestID),
				zap.ByteString("stack", debug.Stack()),
			)
		}
	}()
	candidates, err := searcher.Search(ctx, slots)
	return laneResultFrom(ctx, candidates, err, lane)
}

func laneResultFrom(ctx context.Context, candidates []repository.Candidate, err error, lane string) repository.LaneResult {
	if err != nil {
		return repository.LaneResult{Status: codemeta.ProcurementLaneStatusFailed.Value}
	}
	candidates = acceptEvidence(ctx, candidates, lane)
	if len(candidates) == 0 {
		observability.RecordProcurementCandidateZero(ctx, lane)
	}
	return repository.LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: candidates}
}

// acceptEvidence 是 doc 57 §1.6 evidence contract 的記錄層閘門落點：**候選被呈現
// 之前**對三欄做 accept/reject。規則本身在 domain.ValidateCandidateEvidence，這裡
// 只負責「擋下來之後怎麼辦」。
//
// 放在這一層而不是各 lane 內，是因為這裡是兩條 lane 唯一的匯流點——新增第三條 lane
// 不必記得自己再掛一次，而 lane 內部的重構（例如把 unverifiable 的組裝抽成 helper）
// 也不可能繞過它。
//
// 擋下的是**單一候選**不是整條 lane：一筆壞候選讓整條 lane 變 FAILED，等於用一個
// 資料品質問題換掉其餘所有合法候選的可見性，代價與問題不成比例。被擋下的候選只留
// log——它代表某條 lane 的產出路徑有 bug，不是買家的查詢有問題，所以不對外呈現任何
// 東西，也不影響 lane 終態。零候選那條既有的 metric 仍會在全數被擋時正常計數。
func acceptEvidence(ctx context.Context, candidates []repository.Candidate, lane string) []repository.Candidate {
	out := make([]repository.Candidate, 0, len(candidates))
	for _, c := range candidates {
		if err := procurementdomain.ValidateCandidateEvidence(c.MatchedFactsJSON, c.UnverifiableFactsJSON, c.FactsAsOf); err != nil {
			pkglogger.Error(ctx, "procurement: candidate rejected by evidence contract, dropped before presentation",
				zap.Error(err), zap.String("lane", lane), zap.String("source_id", c.SourceID))
			continue
		}
		out = append(out, c)
	}
	return out
}

type GetRunInput struct {
	RunID     int64
	RequestID int64
	BuyerID   identity.BuyerID
}

// GetRun 供 Task 10 的 GET 端點與 SSE 輪詢共用：一次讀取同時拿到終態判斷與（若已終態）候選內容，
// 並附上每張候選目前的勾選狀態。selected map 只在有候選時才查（COMPLETED/PARTIAL 以外的狀態
// candidates 必為空陣列，查詢徒增一趟無意義的 DB 往返）。
func (uc *CandidateSearchUseCase) GetRun(ctx context.Context, in GetRunInput) (repository.CandidateSet, []repository.Candidate, map[int64]bool, error) {
	set, candidates, err := uc.store.GetCandidateSetRunWithCandidates(ctx, in.RunID, in.RequestID, in.BuyerID)
	if err != nil {
		return repository.CandidateSet{}, nil, nil, err
	}
	if len(candidates) == 0 {
		return set, candidates, map[int64]bool{}, nil
	}
	selected, err := uc.store.ListSelectedCandidateIDs(ctx, set.ID)
	if err != nil {
		return repository.CandidateSet{}, nil, nil, err
	}
	return set, candidates, selected, nil
}
