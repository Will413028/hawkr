package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

// wireRunID 是 **POST**（StartCandidateSearch）對外 runId 這個值的唯一組裝點。
// #2446 身分分離的 stop-using 波（本次）：POST 對外的 runId 是真正的 run 身分，不再等於
// candidate set id（GET／GetCandidateSearch 不受影響，那裡從 expand 波起就是 echo-back，
// 見該函式註解）。
//
// 設計書 57 §1.2-3：runId 不得同時代表 candidateSetId、requestId 或 Temporal Workflow ID。
// 這支測試把這個不變式釘成可執行的斷言：`wireRunID` 的輸出必須是它拿到的 runID 本身，
// 不能悄悄改回接受 candidateSetID 當參數、或改回回傳一個跟 candidateSetID 相等的值。
func TestWireRunID_ReturnsRunIdentityNotCandidateSetIdentity(t *testing.T) {
	const runID int64 = 111

	got := wireRunID(runID)

	if want := "111"; got != want {
		t.Fatalf("wireRunID(%d) = %q, want %q — 對外 runId 必須是真正的 run 身分", runID, got, want)
	}
}

// fakeSnapshotReaderReady 是 StartCandidateSearch 需要的 handler.SnapshotReader 最小
// 實作：固定回一個 REQUIREMENT_READY 的 request 與帶空 slots 的 revision，讓呼叫端
// 走得到 candidateSearch.StartSearch 那一步。
type fakeSnapshotReaderReady struct {
	requestID  int64
	revisionID int64
}

func (f fakeSnapshotReaderReady) GetSnapshot(context.Context, int64, identity.BuyerID) (repository.Request, repository.Revision, error) {
	return repository.Request{ID: f.requestID, Status: "REQUIREMENT_READY"},
		repository.Revision{ID: f.revisionID, RequestID: f.requestID, SlotsJSON: []byte(`{}`)}, nil
}

func (fakeSnapshotReaderReady) FindLatestPublicationForRequest(context.Context, int64, identity.BuyerID) (repository.LatestPublicationForRequest, error) {
	return repository.LatestPublicationForRequest{}, repository.ErrPublicationNotFound
}

// fakeRunIdentityStore 只實作 StartCandidateSearch 這條路徑需要的兩個方法：
// FindCandidateSet 回 ErrNotFound 讓 StartSearch 走「新建」分支，CreateCandidateSet
// 回一個 id／run_id **刻意不同**的 CandidateSet——這才是這支測試存在的理由：如果 handler
// 把兩個值傳反（`wireRunID(result.CandidateSetID)` 而不是 `wireRunID(result.RunID)`），
// 唯有 id 與 run_id 不同值時斷言才抓得到，兩者若剛好相等（例如 fake 偷懶回同一個數字）
// 這個 bug 會無聲通過。MarkCandidateSetRunning 回 claimed=false，讓 StartSearch 內部
// 派出的背景 goroutine（uc.run）在認領前就放棄，不需要再假 lane searcher／
// FinalizeCandidateSet。
type fakeRunIdentityStore struct{ set repository.CandidateSet }

func (f fakeRunIdentityStore) FindCandidateSet(context.Context, int64, int) (repository.CandidateSet, error) {
	return repository.CandidateSet{}, repository.ErrNotFound
}
func (f fakeRunIdentityStore) CreateCandidateSet(context.Context, repository.CreateCandidateSetInput) (repository.CandidateSet, error) {
	return f.set, nil
}
func (fakeRunIdentityStore) MarkCandidateSetRunning(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (fakeRunIdentityStore) FinalizeCandidateSet(context.Context, repository.FinalizeCandidateSetInput) (bool, string, string, error) {
	panic("FinalizeCandidateSet 不該被呼叫：MarkCandidateSetRunning 回 claimed=false，run() 認領失敗就提早 return")
}
func (fakeRunIdentityStore) GetCandidateSetWithCandidates(context.Context, int64, int64, identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	panic("這支測試只測 POST（StartCandidateSearch），不該打到 GET 的讀取路徑")
}
func (fakeRunIdentityStore) GetCandidateSetRunWithCandidates(context.Context, int64, int64, identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	panic("這支測試只測 POST（StartCandidateSearch），不該打到 GET 的讀取路徑")
}
func (fakeRunIdentityStore) RetryCandidateSet(context.Context, int64, string, time.Duration) (repository.CandidateSet, error) {
	panic("RetryCandidateSet 不該被呼叫：FindCandidateSet 永遠回 ErrNotFound，不會走冪等命中分支")
}
func (fakeRunIdentityStore) ListSelectedCandidateIDs(context.Context, int64) (map[int64]bool, error) {
	panic("ListSelectedCandidateIDs 屬於 GetRun，不該被 StartCandidateSearch 呼叫")
}

// CreateLedgerRunForCandidateSearch 在 StartSearch 的「新建」同步路徑上會被呼叫
// 到（見 candidate_search.go），這支測試的 FindCandidateSet 固定回 ErrNotFound，
// 走的就是那條路徑——回 nil 讓既有的 wireRunID 斷言不受這個 Task 10 新增的
// side effect 影響。
func (fakeRunIdentityStore) CreateLedgerRunForCandidateSearch(context.Context, repository.CandidateSet) error {
	return nil
}
func (fakeRunIdentityStore) AppendRunEvent(context.Context, repository.AppendRunEventInput) (repository.RunEvent, error) {
	panic("AppendRunEvent 不該被呼叫：Activity 步驟列在 claim 成功之後才開始寫，而這裡 MarkCandidateSetRunning 回 claimed=false")
}

func (fakeRunIdentityStore) UpsertRunArtifact(context.Context, repository.UpsertRunArtifactInput) (repository.RunArtifact, error) {
	panic("UpsertRunArtifact 不該被呼叫：CANDIDATE_RESULTS 在 finalize 寫入終態之後才產製，而這裡 MarkCandidateSetRunning 回 claimed=false")
}

func (fakeRunIdentityStore) TerminalizeRun(context.Context, repository.TerminalizeRunInput) (bool, error) {
	panic("TerminalizeRun 不該被呼叫：MarkCandidateSetRunning 回 claimed=false，run() 認領失敗就提早 return，不會走到 FinalizeCandidateSet 之後的終態投影")
}

// TestStartCandidateSearch_WiresRunIdNotCandidateSetId 是對抗式審查點出的缺口：既有
// 測試矩陣裡沒有任何一支在 gRPC handler 層級驗證 StartCandidateSearch 的回應真的把
// `RunId` 接到 `result.RunID`、`CandidateSetId` 接到 `result.CandidateSetID`，而不是
// 兩個參數互換（`go test` 全綠的情況下手動把呼叫點改成
// `wireRunID(result.CandidateSetID)` 驗證過：沒有這支測試會漏放）。
func TestStartCandidateSearch_WiresRunIdNotCandidateSetId(t *testing.T) {
	const (
		requestID      int64 = 7001
		revisionID     int64 = 7002
		candidateSetID int64 = 501 // 刻意跟 runID 不同值，見 fakeRunIdentityStore 註解
		runID          int64 = 601
	)

	store := fakeRunIdentityStore{set: repository.CandidateSet{
		ID: candidateSetID, RunID: runID, RequestID: requestID, RequestRevisionID: revisionID,
		Status: "QUEUED",
	}}
	uc := usecase.NewCandidateSearchUseCase(store, nil, nil, func() int64 { return 1 })

	srv := &AIProcurementGrpcServer{
		snapshot:        fakeSnapshotReaderReady{requestID: requestID, revisionID: revisionID},
		candidateSearch: uc,
	}

	ctx := buyerActorContext("1")
	resp, err := srv.StartCandidateSearch(ctx, &aipb.StartCandidateSearchRequest{RequestId: itoa64(requestID)})
	if err != nil {
		t.Fatalf("StartCandidateSearch error = %v, want nil", err)
	}

	if resp.GetRunId() != itoa64(runID) {
		t.Errorf("RunId = %q, want %q (result.RunID, not result.CandidateSetID)", resp.GetRunId(), itoa64(runID))
	}
	if resp.GetCandidateSetId() != itoa64(candidateSetID) {
		t.Errorf("CandidateSetId = %q, want %q", resp.GetCandidateSetId(), itoa64(candidateSetID))
	}
	if resp.GetRunId() == resp.GetCandidateSetId() {
		t.Fatalf("RunId (%q) == CandidateSetId (%q) — fixture 本身設計成不同值，相等代表兩個欄位被接到同一個來源",
			resp.GetRunId(), resp.GetCandidateSetId())
	}
}
