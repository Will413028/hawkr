package usecase

import (
	"context"

	procurementdomain "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

// candidateSearchRail 是候選搜尋 run 的 Activity 步驟列發送器（T23，v6 設計稿
// 01 的「任務」面板上半）。
//
// 它跟 graph 那三個節點用的 twoPhaseActivity 是同一條 rail 的兩個 producer，
// 共用同一個 payload 型別與同一個 marshal 點（ledgerEventTypeForActivity），
// 但走不同的產出管道：graph 節點 emit session.Event 交給 graph_dispatch 翻譯，
// 候選搜尋是脫離 graph 的背景 goroutine，只能直接寫 ledger。買家端兩者都從
// GET /runs/{runId}/events 讀得到（doc57 §2.1：兩條路送同一種 frame）。
//
// 這是**第二個 ledger run**：clarify 階段的步驟屬 TURN run，候選搜尋屬
// CANDIDATE_SEARCH run（CreateLedgerRunForCandidateSearch）。設計稿把兩者畫成
// 一條連續的清單，合併由前端做——它本來就同時握有兩個 runId（POST
// candidate-searches 會回 runId），後端不另造一層跨 run 的投影，兩個 run 的
// 終態語意才保持獨立乾淨。
//
// 併發安全：rowIDs 在建構後只讀，兩條 lane 的 goroutine 各自呼叫 emit 互不
// 干擾，序列化發生在 AppendRunEvent 自己的交易（FOR UPDATE 鎖 run row 分配
// seq）。
type candidateSearchRail struct {
	store   CandidateSearchStore
	runID   int64
	buyerID identity.BuyerID
	// rowIDs 是這次執行為每個步驟碼鑄的 row id。同一個 code 的每一筆事件都用
	// 同一個 row id，consumer 靠 merge-by-rowId 看到一列就地更新——這是既有
	// twoPhaseActivity 建立的慣例，不是本次新增的機制。
	rowIDs map[procurementdomain.StepCode]int64
}

// newCandidateSearchRail 為這次執行鑄一批 row id。不發任何事件——宣告清單是
// declarePlan 的事，兩者分開讓呼叫端能在 claim 成功之後才開始寫 ledger。
func (uc *CandidateSearchUseCase) newCandidateSearchRail(set repository.CandidateSet) *candidateSearchRail {
	rowIDs := make(map[procurementdomain.StepCode]int64, len(procurementdomain.CandidateSearchRailPlan()))
	for _, row := range procurementdomain.CandidateSearchRailPlan() {
		rowIDs[row.Code] = uc.newID()
	}
	return &candidateSearchRail{
		store: uc.store, runID: set.PublicRunID(), buyerID: set.BuyerID, rowIDs: rowIDs,
	}
}

// declarePlan 送出這個 run 的完整前瞻步驟清單，每一列 status=PENDING——設計稿
// 01 的「等待中」。
//
// 只有固定 pipeline 能誠實做這件事（理由見 domain.CandidateSearchRailPlan）。
// 呼叫時機是 claim 成功之後：claim 失敗代表這個 worker 不是這個 run 的擁有者
// （row 已終態，或已被 reaper／重試搶走），對一個別人擁有的 run 寫步驟列會在
// 買家畫面上疊出兩份互相打架的進度。
func (r *candidateSearchRail) declarePlan(ctx context.Context) {
	for _, row := range procurementdomain.CandidateSearchRailPlan() {
		r.emit(ctx, row.Code, codemeta.ProcurementStepStatusPending.Value, nil)
	}
}

// update 把一列推進到新的狀態，count 為 nil 表示這一列沒有計數概念。
func (r *candidateSearchRail) update(ctx context.Context, code procurementdomain.StepCode, status string, count *int) {
	r.emit(ctx, code, status, count)
}

// laneStatus 把一條 lane 的搜尋結果翻成步驟狀態＋計數。lane 失敗時計數是 nil
// 而不是 0：「查失敗」與「查到 0 家」對買家是兩件不同的事，後者是有效結果。
func laneRailOutcome(result repository.LaneResult) (status string, count *int) {
	if result.Status == codemeta.ProcurementLaneStatusFailed.Value {
		return codemeta.ProcurementStepStatusFailed.Value, nil
	}
	n := len(result.Candidates)
	return codemeta.ProcurementStepStatusSucceeded.Value, &n
}

// parentRailStatus 決定父列「搜尋候選供應商」的終態：只要有一條 lane 成功，
// 這個階段對買家就是有產出的，跟 FinalizeCandidateSet 只在**兩條都失敗**時才
// 把整個 run 記成 FAILED 是同一個判準（見該函式的 LANE_ALL_FAILED 語意），不
// 在這裡另立一套。
func parentRailStatus(platform, external repository.LaneResult) string {
	if platform.Status == codemeta.ProcurementLaneStatusFailed.Value &&
		external.Status == codemeta.ProcurementLaneStatusFailed.Value {
		return codemeta.ProcurementStepStatusFailed.Value
	}
	return codemeta.ProcurementStepStatusSucceeded.Value
}

// emit 寫一筆 Activity 事件進 ledger。
//
// 失敗只記 log、不回傳錯誤，也不中斷候選搜尋——跟 graph 那三個節點的
// emitActivityEvent 呼叫點、以及本檔既有的 ledger 投影（
// CreateLedgerRunForCandidateSearch／TerminalizeRun）同一套紀律：ledger 上的
// 進度顯示是投影，candidate_set 才是這個 domain 的 SSOT，寫不進去不該讓買家
// 的搜尋整趟失敗。
func (r *candidateSearchRail) emit(ctx context.Context, code procurementdomain.StepCode, status string, count *int) {
	rowID, ok := r.rowIDs[code]
	if !ok {
		// 只可能是有人改了 CandidateSearchRailPlan 卻沒改呼叫端——記下來，
		// 不要靜默送一筆 rowId=0 的列（那會跟其他列在 consumer 端合併成同一列）。
		pkglogger.Error(ctx, "procurement: candidate search rail has no row id for step code",
			zap.String("code", string(code)), zap.Int64("run_id", r.runID))
		return
	}
	var parentRowID int64
	for _, row := range procurementdomain.CandidateSearchRailPlan() {
		if row.Code == code && row.Parent != "" {
			parentRowID = r.rowIDs[row.Parent]
			break
		}
	}
	eventType, payload := ledgerEventTypeForActivity(ActivityEventPayload{
		RowID: rowID, Attempt: 1, MaxAttempts: 1, RetryGroupKey: rowID,
		Code: string(code), Label: procurementdomain.StepLabels[code],
		// Text 恆空：這條 run 沒有 StepNarrator（整趟沒有 LLM 呼叫），沒有
		// 「這一次實際發生了什麼」的敘述可寫。consumer 據此顯示 Label。
		Text:   "",
		Status: status, ParentRowID: parentRowID, Count: count,
	})
	if _, err := r.store.AppendRunEvent(ctx, repository.AppendRunEventInput{
		RunID: r.runID, BuyerID: r.buyerID, EventType: eventType,
		SchemaVersion: "v1", PublicPayloadJSON: payload,
	}); err != nil {
		pkglogger.Error(ctx, "procurement: failed to append candidate search rail event, continuing search",
			zap.Error(err), zap.Int64("run_id", r.runID), zap.String("code", string(code)),
			zap.String("status", status))
	}
}
