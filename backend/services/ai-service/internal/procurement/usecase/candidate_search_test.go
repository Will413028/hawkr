package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	procurementdomain "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

type fakeCandidateStore struct {
	set              repository.CandidateSet
	finalizeCalled   bool
	finalizeInput    repository.FinalizeCandidateSetInput
	markRunningCalls int
	// claimFails 讓測試模擬 claim-check 落空（rows affected == 0）：row 已終態、或已被
	// reaper／另一次 RetryCandidateSet 搶走。零值 false = 正常認領成功，既有測試不受影響。
	claimFails bool
	// finalizeNoOp 讓測試模擬「finalize 影響 0 列」：run 已被 reaper 收走或被別人終結過，
	// 這次寫入被防呆條件整段丟掉（nil error + wrote=false）。零值 false = 正常寫入成功。
	finalizeNoOp bool
	// finalizeStatus 是 FinalizeCandidateSet 第二個回傳值（實際寫進 DB 的 run 終態）。
	// 空字串代表用預設的 SUCCEEDED。這個欄位的意義是能餵一個「從 lane 狀態推導不出來」的值，
	// 藉此證明 metric label 真的取自 repository 的回報，而不是在 usecase 重新推導。
	finalizeStatus string
	// finalizeFailureReason 是第三個回傳值（run 失敗的機器碼），空字串代表非 FAILED。
	finalizeFailureReason string
	// panicOnMarkRunning 讓 MarkCandidateSetRunning 直接 panic，用於證明 dispatch()
	// 外層改用 syncutil.SafeGo 之後，仍然接得住 run() 在 lane fan-out 之前（早於
	// searchLane 自己那層 per-goroutine recover）就發生的 panic。零值 false = 既有
	// 行為不變。
	panicOnMarkRunning bool
	// panicReached 只在 panicOnMarkRunning=true 時由測試初始化；MarkCandidateSetRunning
	// 在呼叫 panic() 前先 close 它，讓測試不用猜「背景 goroutine 跑到哪了」，能確定性地
	// 等到「panic 這個陳述式已經執行」這個時間點，取代 time.Sleep 這種可能在重載環境下
	// false-pass 的等待方式（panic 之後 run() 的其餘同步程式碼——包含 FinalizeCandidateSet
	// 呼叫——保證不會再執行，這是 Go panic 語意本身保證的，不需要額外等 recover 完成）。
	panicReached chan struct{}

	// railEvents 收下 T23 的 Activity 步驟列事件；railErr 讓測試模擬 ledger 寫入
	// 失敗（應該只被 log 吞掉，不影響搜尋本身）。railMu 見 AppendRunEvent。
	railMu     sync.Mutex
	railEvents []repository.AppendRunEventInput
	railErr    error

	// createLedgerRunCalled／createLedgerRunSet 記錄 CreateLedgerRunForCandidateSearch
	// 有沒有被呼叫、呼叫時收到的 set 是什麼——供 Task 10 的 ledger 投影測試斷言。
	// createLedgerRunErr 讓測試模擬 ledger 寫入失敗（應該只被 log 吞掉，不影響既有的
	// 候選搜尋流程本身）。
	createLedgerRunCalled bool
	createLedgerRunSet    repository.CandidateSet
	createLedgerRunErr    error
	// terminalizeCalled／terminalizeInput 記錄 TerminalizeRun 有沒有被呼叫、收到的輸入。
	// terminalizeNoOp 模擬 CAS 沒過（wrote=false），terminalizeErr 模擬呼叫失敗——兩者都
	// 應該只被記 log，不影響 run() 既有的 metric 計數或控制流。
	terminalizeCalled bool
	terminalizeInput  repository.TerminalizeRunInput
	terminalizeNoOp   bool
	terminalizeErr    error

	// persistedCandidates 是 FinalizeCandidateSet 之後由 DB 讀回來的候選（帶已指派的
	// ID）——CANDIDATE_RESULTS artifact 的 producer 需要它們才組得出 candidateId。
	persistedCandidates []repository.Candidate
	// upsertArtifactCalls 收下每一次 artifact 產製；upsertArtifactErr 模擬寫入失敗
	// （應只被 log 吞掉，不影響已寫定的 candidate set 終態）。
	upsertArtifactCalls []repository.UpsertRunArtifactInput
	upsertArtifactErr   error
}

func (f *fakeCandidateStore) UpsertRunArtifact(_ context.Context, in repository.UpsertRunArtifactInput) (repository.RunArtifact, error) {
	f.upsertArtifactCalls = append(f.upsertArtifactCalls, in)
	if f.upsertArtifactErr != nil {
		return repository.RunArtifact{}, f.upsertArtifactErr
	}
	return repository.RunArtifact{ArtifactID: 777, RunID: in.RunID, ArtifactType: in.ArtifactType, Version: 1}, nil
}

func (f *fakeCandidateStore) FindCandidateSet(context.Context, int64, int) (repository.CandidateSet, error) {
	return repository.CandidateSet{}, repository.ErrNotFound
}
func (f *fakeCandidateStore) CreateCandidateSet(context.Context, repository.CreateCandidateSetInput) (repository.CandidateSet, error) {
	return f.set, nil
}
func (f *fakeCandidateStore) MarkCandidateSetRunning(context.Context, int64, int64) (bool, error) {
	if f.panicOnMarkRunning {
		close(f.panicReached)
		panic("boom: simulated panic before lane fan-out")
	}
	f.markRunningCalls++
	return !f.claimFails, nil
}
func (f *fakeCandidateStore) FinalizeCandidateSet(_ context.Context, in repository.FinalizeCandidateSetInput) (bool, string, string, error) {
	f.finalizeCalled = true
	f.finalizeInput = in
	status := f.finalizeStatus
	if status == "" {
		status = codemeta.ProcurementRunStatusSucceeded.Value
	}
	return !f.finalizeNoOp, status, f.finalizeFailureReason, nil
}

// GetCandidateSetRunWithCandidates 委派給同一個回傳值：這些 fake 都只用來測 StartSearch，
// 不驗兩把身分的解析差異（那是 repository 層 integration test 的事）。
func (f *fakeCandidateStore) GetCandidateSetRunWithCandidates(ctx context.Context, runID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	return f.GetCandidateSetWithCandidates(ctx, runID, requestID, buyerID)
}
func (f *fakeCandidateStore) GetCandidateSetWithCandidates(context.Context, int64, int64, identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	return f.set, f.persistedCandidates, nil
}
func (f *fakeCandidateStore) RetryCandidateSet(context.Context, int64, string, time.Duration) (repository.CandidateSet, error) {
	panic("RetryCandidateSet 不該被呼叫：fakeCandidateStore.FindCandidateSet 永遠回傳 ErrNotFound，不會走冪等命中分支")
}
func (f *fakeCandidateStore) ListSelectedCandidateIDs(context.Context, int64) (map[int64]bool, error) {
	panic("ListSelectedCandidateIDs 不該被呼叫：這個 fake 只用於測 StartSearch，不會被傳進 GetRun")
}
func (f *fakeCandidateStore) CreateLedgerRunForCandidateSearch(_ context.Context, set repository.CandidateSet) error {
	f.createLedgerRunCalled = true
	f.createLedgerRunSet = set
	return f.createLedgerRunErr
}

// AppendRunEvent 收下 T23 的 Activity 步驟列事件供斷言。用 mutex：兩條 lane 的
// goroutine 會併發呼叫它（production 端序列化發生在 AppendRunEvent 自己的 DB
// 交易，測試替身沒有那層，不加鎖會被 -race 抓到）。
func (f *fakeCandidateStore) AppendRunEvent(_ context.Context, in repository.AppendRunEventInput) (repository.RunEvent, error) {
	f.railMu.Lock()
	defer f.railMu.Unlock()
	f.railEvents = append(f.railEvents, in)
	return repository.RunEvent{}, f.railErr
}

// railRows 把收到的事件依 rowId 合併成「每一列的最後狀態」——正是 consumer 端
// merge-by-rowId 的行為，測試據此斷言終態而不必數事件筆數。
func (f *fakeCandidateStore) railRows(t *testing.T) map[string]map[string]any {
	t.Helper()
	f.railMu.Lock()
	defer f.railMu.Unlock()
	rows := map[string]map[string]any{}
	for _, ev := range f.railEvents {
		var row map[string]any
		if err := json.Unmarshal(ev.PublicPayloadJSON, &row); err != nil {
			t.Fatalf("rail event payload is not valid JSON: %v", err)
		}
		rows[row["code"].(string)] = row
	}
	return rows
}

func (f *fakeCandidateStore) TerminalizeRun(_ context.Context, in repository.TerminalizeRunInput) (bool, error) {
	f.terminalizeCalled = true
	f.terminalizeInput = in
	if f.terminalizeErr != nil {
		return false, f.terminalizeErr
	}
	return !f.terminalizeNoOp, nil
}

type stubPlatformSearcher struct {
	candidates []repository.Candidate
	err        error
}

func (s stubPlatformSearcher) Search(context.Context, procurementdomain.Slots) ([]repository.Candidate, error) {
	return s.candidates, s.err
}

type stubExternalSearcher struct {
	candidates []repository.Candidate
	err        error
}

func (s stubExternalSearcher) Search(context.Context, procurementdomain.Slots) ([]repository.Candidate, error) {
	return s.candidates, s.err
}

func TestRun_BothLanesSucceed_FinalizesSucceeded(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if !store.finalizeCalled {
		t.Fatal("FinalizeCandidateSet was never called")
	}
	if store.finalizeInput.Platform.Status != codemeta.ProcurementLaneStatusSucceeded.Value || store.finalizeInput.External.Status != codemeta.ProcurementLaneStatusSucceeded.Value {
		t.Errorf("lane statuses = %+v, want both SUCCEEDED", store.finalizeInput)
	}
	if store.markRunningCalls != 1 {
		t.Errorf("MarkCandidateSetRunning called %d times, want 1", store.markRunningCalls)
	}
}

func TestRun_OneLaneFails_OtherStillFinalizes(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{err: context.DeadlineExceeded},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if store.finalizeInput.Platform.Status != codemeta.ProcurementLaneStatusFailed.Value {
		t.Errorf("Platform.Status = %q, want FAILED", store.finalizeInput.Platform.Status)
	}
	if store.finalizeInput.External.Status != codemeta.ProcurementLaneStatusSucceeded.Value {
		t.Errorf("External.Status = %q, want SUCCEEDED", store.finalizeInput.External.Status)
	}
	if len(store.finalizeInput.External.Candidates) != 1 {
		t.Errorf("External lane 的候選不應因 Platform lane 失敗而消失")
	}
}

// panicSearcher 是一個會 panic 的 LaneSearcher，用來守住 searchLane 的 per-goroutine
// recover()——若未來有人「簡化」把兩個 lane goroutine 各自的 recover 收斂回 run() 或
// StartSearch 外層那一個，這個測試會失敗（用 fakeCandidateStore 直接呼叫 uc.run 就會讓
// panic 沒有任何 recover 接住，整個測試 binary 會被打掛，而不是這個測試單純變紅）。
type panicSearcher struct{}

func (panicSearcher) Search(context.Context, procurementdomain.Slots) ([]repository.Candidate, error) {
	panic("boom: simulated lane searcher panic")
}

func TestRun_PlatformLanePanics_RecoveredAsFailedAndExternalUnaffected(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store,
		panicSearcher{},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
		testRailIDs())

	// 呼叫 uc.run 是同步的（測試主 goroutine），不必經過 StartSearch 的背景 goroutine
	// 派送——run() 內部仍然會各自開一個 goroutine 平行跑兩條 lane，這正是要驗證的路徑。
	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if !store.finalizeCalled {
		t.Fatal("FinalizeCandidateSet was never called after a lane panic — searchLane's per-goroutine recover must still send a result on the channel, or run() deadlocks forever")
	}
	if store.finalizeInput.Platform.Status != codemeta.ProcurementLaneStatusFailed.Value {
		t.Errorf("Platform.Status = %q, want FAILED (recovered panic)", store.finalizeInput.Platform.Status)
	}
	if store.finalizeInput.External.Status != codemeta.ProcurementLaneStatusSucceeded.Value {
		t.Errorf("External.Status = %q, want SUCCEEDED — a panic in the PLATFORM lane must not affect EXTERNAL", store.finalizeInput.External.Status)
	}
	if len(store.finalizeInput.External.Candidates) != 1 {
		t.Errorf("External lane 的候選不應因 Platform lane panic 而消失")
	}
}

// recordingSearcher 只記錄「有沒有被呼叫過」。用 atomic.Bool 而不是裸 bool：run() 正常
// 路徑會在兩個 goroutine 裡各呼叫一次 Search，裸 bool 在 -race 下會被判成 data race。
type recordingSearcher struct{ called atomic.Bool }

func (s *recordingSearcher) Search(context.Context, procurementdomain.Slots) ([]repository.Candidate, error) {
	s.called.Store(true)
	return nil, nil
}

// TestRun_ClaimCheckFails_AbandonsRunWithoutSearchingLanes 守住 MarkCandidateSetRunning
// 的 claim-check 結果真的被拿來當控制流程用。
//
// 底層 SQL 是 `WHERE id = $1 AND fencing_token = $2 AND status = 'QUEUED'`——影響 0 列
// 代表這個 worker 根本沒認領到（row 已終態，或 lease 過期被 reap／被另一次
// RetryCandidateSet 搶走、fencing_token 已經 +1）。早期版本 repository 直接把
// rows-affected 丟掉、run() 也只檢查 err，於是這種情況下 worker 會照樣跑完整趟雙 lane
// 搜尋：一次真實的 product-service gRPC 呼叫加一次 candidate_leads 掃描，而結果最後一定
// 被 FinalizeCandidateSet 的同一組條件丟掉。這裡斷言「兩個 lane searcher 都沒有被呼叫」，
// 而不只是斷言 finalize 沒被呼叫——白燒的正是 lane 搜尋那段，finalize 本來就寫不進去。
func TestRun_ClaimCheckFails_AbandonsRunWithoutSearchingLanes(t *testing.T) {
	store := &fakeCandidateStore{
		set:        repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5},
		claimFails: true,
	}
	platform := &recordingSearcher{}
	external := &recordingSearcher{}
	uc := NewCandidateSearchUseCase(store, platform, external, testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if store.markRunningCalls != 1 {
		t.Fatalf("MarkCandidateSetRunning called %d times, want 1", store.markRunningCalls)
	}
	if platform.called.Load() {
		t.Error("PLATFORM lane 在 claim-check 落空後仍被搜尋——這是一次真實的 product-service gRPC 白工")
	}
	if external.called.Load() {
		t.Error("EXTERNAL lane 在 claim-check 落空後仍被搜尋——這是一次真實的 candidate_leads 掃描白工")
	}
	if store.finalizeCalled {
		t.Error("claim-check 落空後不該再呼叫 FinalizeCandidateSet——那次寫入必然影響 0 列")
	}
}

// TestRun_NoOpFinalizeDoesNotCountAsRun 守住 procurement_run_total 的正確性。
//
// FinalizeCandidateSet 對「run 已被 reaper 收走／已被別人終結」與「真的寫進去了」都回
// nil error——這是持久層的正確行為（兩者都沒有東西壞掉），但對呼叫端不是同一件事。早期
// 版本只看 err，於是一個 zombie worker 被丟掉的寫入照樣被計成一次全新的 SUCCEEDED：
// 不只多算一筆不存在的 run，貼上的 status label 還是錯的（這個 worker 以為自己成功了，
// 那個 run 實際上是 WORKER_LOST）。doc 55 §8.2 拿這個 counter 當主要 run 健康訊號，
// 被 zombie 灌水的 counter 會讓「worker 大量死亡」看起來像一切正常。
//
// 兩個 case 一起跑才有意義：只驗「no-op 不計數」的話，把整段 .Inc() 刪掉也會通過。
func TestRun_NoOpFinalizeDoesNotCountAsRun(t *testing.T) {
	reader := installTestMeterProvider(t)

	t.Run("finalize 真的寫入 → 計一筆", func(t *testing.T) {
		store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5}}
		uc := NewCandidateSearchUseCase(store,
			stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1")}},
			stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
			testRailIDs())

		before := counterValue(t, reader, "procurement_run_total", "status", codemeta.ProcurementRunStatusSucceeded.Value)
		uc.run(context.Background(), store.set, procurementdomain.Slots{})
		after := counterValue(t, reader, "procurement_run_total", "status", codemeta.ProcurementRunStatusSucceeded.Value)

		if after-before != 1 {
			t.Errorf("procurement_run_total{status=SUCCEEDED} 增加了 %v，want 1", after-before)
		}
	})

	t.Run("finalize 是 no-op → 不計數", func(t *testing.T) {
		store := &fakeCandidateStore{
			set:          repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5},
			finalizeNoOp: true,
		}
		uc := NewCandidateSearchUseCase(store,
			stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1")}},
			stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
			testRailIDs())

		before := counterValue(t, reader, "procurement_run_total", "status", codemeta.ProcurementRunStatusSucceeded.Value)
		uc.run(context.Background(), store.set, procurementdomain.Slots{})
		after := counterValue(t, reader, "procurement_run_total", "status", codemeta.ProcurementRunStatusSucceeded.Value)

		if !store.finalizeCalled {
			t.Fatal("test precondition: FinalizeCandidateSet 應該有被呼叫（只是影響 0 列）")
		}
		if after != before {
			t.Errorf("procurement_run_total{status=SUCCEEDED} 增加了 %v，want 0——被丟掉的寫入不是一次 run", after-before)
		}
	})
}

// TestRun_FinalizeWrites_TerminalizesLedgerRunAsFinished 守住 Task 10 新增的
// side effect：FinalizeCandidateSet 真的把終態寫進 candidate_set 之後（wrote
// == true），run() 要額外呼叫 TerminalizeRun 把同一個終態投影進新的 run
// ledger，帶對的 run 身分（set.PublicRunID()，不是 candidateSetId）、buyer、
// 終態與對應的 ledger event type。
func TestRun_FinalizeWrites_TerminalizesLedgerRunAsFinished(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, RunID: 900, FencingToken: 1, RequestID: 5, BuyerID: identity.BuyerID(7)}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if !store.terminalizeCalled {
		t.Fatal("TerminalizeRun was never called after FinalizeCandidateSet wrote a terminal state")
	}
	if store.terminalizeInput.RunID != 900 {
		t.Errorf("TerminalizeRun RunID = %d, want 900 (set.PublicRunID(), not the candidateSetId 100)", store.terminalizeInput.RunID)
	}
	if store.terminalizeInput.BuyerID != identity.BuyerID(7) {
		t.Errorf("TerminalizeRun BuyerID = %v, want 7", store.terminalizeInput.BuyerID)
	}
	if store.terminalizeInput.Status != codemeta.ProcurementRunStatusSucceeded.Value {
		t.Errorf("TerminalizeRun Status = %q, want SUCCEEDED", store.terminalizeInput.Status)
	}
	if store.terminalizeInput.TerminalEventType != codemeta.ProcurementLedgerEventTypeRunFinished.Value {
		t.Errorf("TerminalizeRun TerminalEventType = %q, want RUN_FINISHED", store.terminalizeInput.TerminalEventType)
	}
	// 不再帶 Version（2026-09-07 決策：terminalize 不對 version 做 CAS）：舊斷言要求
	// Version == 1，前提是「ledger run 建立之後沒有任何中途寫入會 bump version」——
	// 那個前提被 RetryCandidateSet（沿用同一 run_id）與 CancelRun（走同一支帶 version
	// 的 query）打破，一旦成立，CAS 落空會讓這個 run 永遠沒有終態 ledger event。終態
	// 分支的守衛已改成狀態轉移合法性，這裡必須是零值，否則就是有人又把版本號帶回來了。
	if store.terminalizeInput.Version != 0 {
		t.Errorf("TerminalizeRun Version = %d, want 0——終態轉移不得帶 version（守衛是狀態轉移合法性）", store.terminalizeInput.Version)
	}
}

// TestRun_FinalizeFails_ProducesErrorRecoveryArtifact：run 失敗時要產出
// ERROR_RECOVERY artifact。
//
// 先前失敗的 run **完全不產 artifact**：CANDIDATE_RESULTS 只在 SUCCEEDED 時產製
// （硬產一個空的等於對前端宣稱「搜尋完成、結果是零家」，而實際上是搜尋失敗），
// 而誠實的替代品一直沒落地。於是「搜尋失敗」在 artifact 這條線上完全沒有表示。
func TestRun_FinalizeFails_ProducesErrorRecoveryArtifact(t *testing.T) {
	store := &fakeCandidateStore{
		set:                   repository.CandidateSet{ID: 100, RunID: 900, FencingToken: 1, RequestID: 5},
		finalizeStatus:        codemeta.ProcurementRunStatusFailed.Value,
		finalizeFailureReason: codemeta.ProcurementRunFailureReasonLaneAllFailed.Value,
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{err: context.DeadlineExceeded},
		stubExternalSearcher{err: context.DeadlineExceeded},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if len(store.upsertArtifactCalls) != 1 {
		t.Fatalf("UpsertRunArtifact 呼叫 %d 次, want 1", len(store.upsertArtifactCalls))
	}
	got := store.upsertArtifactCalls[0]
	if got.ArtifactType != codemeta.ProcurementArtifactTypeErrorRecovery.Value {
		t.Errorf("ArtifactType = %q, want ERROR_RECOVERY", got.ArtifactType)
	}
	if got.Status != codemeta.ProcurementArtifactStatusReady.Value {
		t.Errorf("Status = %q, want READY", got.Status)
	}
	if got.RunID != store.set.PublicRunID() {
		t.Errorf("RunID = %d, want %d", got.RunID, store.set.PublicRunID())
	}
	if got.DomainRefType != "candidateSetId" || got.DomainRefID != "100" {
		t.Errorf("domainRef = %s/%s, want candidateSetId/100", got.DomainRefType, got.DomainRefID)
	}
	if got.Checksum == "" {
		t.Error("Checksum 不得為空")
	}

	var data procurementdomain.ErrorRecoveryData
	if err := json.Unmarshal(got.PublicDataJSON, &data); err != nil {
		t.Fatalf("PublicDataJSON 解不開: %v", err)
	}
	if data.Reason != codemeta.ProcurementRunFailureReasonLaneAllFailed.Value {
		t.Errorf("Reason = %q, want LANE_ALL_FAILED", data.Reason)
	}
	if data.Message != codemeta.ProcurementRunFailureReasonLaneAllFailed.Label {
		t.Errorf("Message = %q, want CodeMeta label %q", data.Message, codemeta.ProcurementRunFailureReasonLaneAllFailed.Label)
	}
	if !data.Retryable {
		t.Error("LANE_ALL_FAILED 是基礎設施層的暫時失敗，且 RetryCandidateSet 這條路存在，應為 retryable")
	}
}

// TestRun_FinalizeSucceeds_DoesNotProduceErrorRecoveryArtifact：成功的 run 只產
// CANDIDATE_RESULTS，不得順手補一個 ERROR_RECOVERY。
func TestRun_FinalizeSucceeds_DoesNotProduceErrorRecoveryArtifact(t *testing.T) {
	store := &fakeCandidateStore{
		set: repository.CandidateSet{ID: 100, RunID: 900, FencingToken: 1, RequestID: 5},
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("p1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("e1")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	for _, call := range store.upsertArtifactCalls {
		if call.ArtifactType == codemeta.ProcurementArtifactTypeErrorRecovery.Value {
			t.Fatal("成功的 run 不得產出 ERROR_RECOVERY artifact")
		}
	}
}

// TestRun_FinalizeFails_TerminalEventCarriesFailureReason 守住 doc57 §4.2：
// RUN_FAILED 的 public payload 必須自描述地帶著 failureReason，gateway 才組得出
// 原生 AG-UI RUN_ERROR 的 reason。原因取自 FinalizeCandidateSet 的回傳值而不是在
// 這一層重新推導——理由與 finalStatus 逐字相同（見該函式註解）：兩份邏輯各自演化的
// 話，wire 上的原因會與 DB 裡的原因悄悄分家。
func TestRun_FinalizeFails_TerminalEventCarriesFailureReason(t *testing.T) {
	store := &fakeCandidateStore{
		set:                   repository.CandidateSet{ID: 100, RunID: 900, FencingToken: 1, RequestID: 5},
		finalizeStatus:        codemeta.ProcurementRunStatusFailed.Value,
		finalizeFailureReason: codemeta.ProcurementRunFailureReasonLaneAllFailed.Value,
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{err: context.DeadlineExceeded},
		stubExternalSearcher{err: context.DeadlineExceeded},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if !store.terminalizeCalled {
		t.Fatal("TerminalizeRun was never called")
	}
	want := `{"failureReason":"` + codemeta.ProcurementRunFailureReasonLaneAllFailed.Value + `"}`
	if got := string(store.terminalizeInput.PublicPayloadJSON); got != want {
		t.Errorf("PublicPayloadJSON = %s, want %s", got, want)
	}
}

// TestRun_FinalizeFails_TerminalizesLedgerRunAsFailed 守住 finalStatus ==
// FAILED 時，ledger 那端也要用 RUN_FAILED（不是 RUN_FINISHED）收斂，跟
// candidate_set 自己的終態語意一致。
func TestRun_FinalizeFails_TerminalizesLedgerRunAsFailed(t *testing.T) {
	store := &fakeCandidateStore{
		set:            repository.CandidateSet{ID: 100, RunID: 900, FencingToken: 1, RequestID: 5},
		finalizeStatus: codemeta.ProcurementRunStatusFailed.Value,
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{err: context.DeadlineExceeded},
		stubExternalSearcher{err: context.DeadlineExceeded},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if !store.terminalizeCalled {
		t.Fatal("TerminalizeRun was never called after FinalizeCandidateSet wrote a FAILED terminal state")
	}
	if store.terminalizeInput.Status != codemeta.ProcurementRunStatusFailed.Value {
		t.Errorf("TerminalizeRun Status = %q, want FAILED", store.terminalizeInput.Status)
	}
	if store.terminalizeInput.TerminalEventType != codemeta.ProcurementLedgerEventTypeRunFailed.Value {
		t.Errorf("TerminalizeRun TerminalEventType = %q, want RUN_FAILED", store.terminalizeInput.TerminalEventType)
	}
}

// TestRun_NoOpFinalize_DoesNotTerminalizeLedgerRun 守住「FinalizeCandidateSet
// 影響 0 列（wrote == false）時不呼叫 TerminalizeRun」——這個 run 已經被 reaper
// 收走或被別人終結過，本次寫入沒有東西落地，ledger 那端也不該憑空補一次終態
// （會覆寫掉真正贏家寫入的終態，或對一個早就終態化過的 ledger run 做一次注定
// CAS 落空的呼叫）。
func TestRun_NoOpFinalize_DoesNotTerminalizeLedgerRun(t *testing.T) {
	store := &fakeCandidateStore{
		set:          repository.CandidateSet{ID: 100, RunID: 900, FencingToken: 1, RequestID: 5},
		finalizeNoOp: true,
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if store.terminalizeCalled {
		t.Error("TerminalizeRun 不該在 FinalizeCandidateSet 是 no-op（wrote=false）時被呼叫")
	}
}

func TestStartSearch_ReturnsExistingRunWhenIdempotent(t *testing.T) {
	existing := repository.CandidateSet{ID: 200, Status: codemeta.ProcurementRunStatusSucceeded.Value}
	store := &fakeCandidateStoreIdempotent{existing: existing}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	got, err := uc.StartSearch(context.Background(), StartSearchInput{
		RequestID: 5, RequestRevisionID: 9, BuyerID: 1, RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RunID != 200 {
		t.Errorf("RunID = %d, want 200 (existing run, no new goroutine dispatched)", got.RunID)
	}
}

func TestStartSearch_RetriesDeadRunWhenIdempotencyKeyHitsFailed(t *testing.T) {
	existing := repository.CandidateSet{ID: 300, RequestID: 5, FencingToken: 1, Status: codemeta.ProcurementRunStatusFailed.Value, FailureReason: codemeta.ProcurementRunFailureReasonWorkerLost.Value}
	retried := repository.CandidateSet{ID: 300, RequestID: 5, FencingToken: 2, Status: codemeta.ProcurementRunStatusQueued.Value}
	store := &fakeCandidateStoreRetryable{
		existing: existing, retried: retried,
		markRunningCh: make(chan struct{}, 1),
	}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	got, err := uc.StartSearch(context.Background(), StartSearchInput{
		RequestID: 5, RequestRevisionID: 9, BuyerID: 1, RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.retryCalled {
		t.Fatal("RetryCandidateSet was never called for an idempotency hit on a FAILED run")
	}
	if got.RunID != 300 || got.Status != "QUEUED" {
		t.Errorf("StartSearch result = %+v, want the retried run {RunID:300 Status:QUEUED}, not the stale FAILED snapshot", got)
	}

	select {
	case <-store.markRunningCh:
		// 背景 goroutine 真的重新跑到 run()、對「重試後」的 candidate_set 呼叫了
		// MarkCandidateSetRunning——不只是 StartSearch 的同步回傳值看起來對而已。
	case <-time.After(2 * time.Second):
		t.Fatal("background run was never (re-)dispatched for the retried candidate set")
	}
}

// TestStartSearch_RetryProjectsLedgerRunForTheNewAttempt 守的是 B9 在 usecase 這一側的
// 那一半：重試配到一個新的 run 身分之後，**必須**為它投影一列 ledger run。
//
// 少了這一句的失敗形狀是靜默的：候選搜尋本身照常跑完、買家輪詢也讀得到結果，但他拿到的
// runId 在 procurement_agent_runs 上根本不存在——SSE 訂閱不到任何事件，終態也無處可寫。
func TestStartSearch_RetryProjectsLedgerRunForTheNewAttempt(t *testing.T) {
	existing := repository.CandidateSet{ID: 300, RunID: 700, RequestID: 5, FencingToken: 1, Status: codemeta.ProcurementRunStatusFailed.Value, FailureReason: codemeta.ProcurementRunFailureReasonWorkerLost.Value}
	// 同一列 candidate_set（ID 不變），新的一次嘗試（RunID 換值）。
	retried := repository.CandidateSet{ID: 300, RunID: 701, RequestID: 5, FencingToken: 2, Status: codemeta.ProcurementRunStatusQueued.Value}
	store := &fakeCandidateStoreRetryable{existing: existing, retried: retried, markRunningCh: make(chan struct{}, 1)}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	got, err := uc.StartSearch(context.Background(), StartSearchInput{
		RequestID: 5, RequestRevisionID: 9, BuyerID: 1, RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RunID != 701 {
		t.Errorf("StartSearch result RunID = %d, want 701（重試那一次嘗試的新 run 身分）", got.RunID)
	}
	if len(store.ledgerRunsCreated) != 1 {
		t.Fatalf("CreateLedgerRunForCandidateSearch 被呼叫 %d 次, want 1——重試也是一次新的執行，必須投影它自己的 ledger run", len(store.ledgerRunsCreated))
	}
	if store.ledgerRunsCreated[0].PublicRunID() != 701 {
		t.Errorf("投影的 run 身分 = %d, want 701（不是被取代的 700）", store.ledgerRunsCreated[0].PublicRunID())
	}

	select {
	case <-store.markRunningCh:
	case <-time.After(2 * time.Second):
		t.Fatal("background run was never (re-)dispatched for the retried candidate set")
	}
}

// TestStartSearch_LostRetryRace_ReturnsCurrentRunIdentity 守的是重試落敗分支。
//
// 兩個併發的 POST candidate-searches 打在同一筆 FAILED 的 run 上，只有一個的
// RetryCandidateSet 會影響到列，另一個拿到 ErrNoRows。落敗方**不能**回它一開始讀到的
// 那個快照：run 身分現在每次嘗試都換，那個快照裡的 runId 已經被贏家取代，交出去等於讓
// 那個 client 整段訂閱在錯誤的 run 上——它會讀到前一次嘗試的終態事件並顯示「搜尋失敗」，
// 而真正在跑的是贏家那一個。
//
// 這條在 run_id 穩定的舊模型下不存在（兩邊的 runId 相同），是 B9 的連帶面。
func TestStartSearch_LostRetryRace_ReturnsCurrentRunIdentity(t *testing.T) {
	existing := repository.CandidateSet{ID: 300, RunID: 700, RequestID: 5, FencingToken: 1, Status: codemeta.ProcurementRunStatusFailed.Value, FailureReason: codemeta.ProcurementRunFailureReasonWorkerLost.Value}
	// 贏家已經寫進去的當前狀態：同一列，但 run 身分是它剛配的 702。
	winner := repository.CandidateSet{ID: 300, RunID: 702, RequestID: 5, FencingToken: 2, Status: codemeta.ProcurementRunStatusRunning.Value}
	store := &fakeCandidateStoreRetryable{
		existing: existing, refreshed: &winner,
		retryErr: repository.ErrNotFound, // 影響 0 列＝輸掉這場 race
	}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	got, err := uc.StartSearch(context.Background(), StartSearchInput{
		RequestID: 5, RequestRevisionID: 9, BuyerID: 1, RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.retryCalled {
		t.Fatal("test precondition: RetryCandidateSet 應該被嘗試過一次")
	}
	if got.RunID != 702 {
		t.Errorf("StartSearch result RunID = %d, want 702（贏家寫進去的當前 run 身分）；回 %d 等於交出一個已被取代的 runId", got.RunID, existing.RunID)
	}
	if got.Status != codemeta.ProcurementRunStatusRunning.Value {
		t.Errorf("Status = %q, want RUNNING（當前狀態，不是那筆 FAILED 的舊快照）", got.Status)
	}
	if len(store.ledgerRunsCreated) != 0 {
		t.Errorf("落敗方不得投影 ledger run——那一列是贏家建的，重複建會撞 primary key（got %d 次）", len(store.ledgerRunsCreated))
	}
}

func TestStartSearch_RejectsWhenRequirementNotReady(t *testing.T) {
	store := &fakeCandidateStore{}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	_, err := uc.StartSearch(context.Background(), StartSearchInput{
		RequestID: 5, RequestRevisionID: 9, BuyerID: 1, RequirementReady: false,
	})
	if !errors.Is(err, ErrRequirementNotReady) {
		t.Errorf("err = %v, want ErrRequirementNotReady", err)
	}
}

// TestStartSearch_CreatesLedgerRunProjection 守住 Task 10 的核心 side effect：
// 一個全新建立的 candidate_set，除了走既有 lease/fencing 流程，還要額外在新
// run ledger 投影一筆對應的 procurement_agent_runs row（見設計書 57 §1.2-3
// 「domainRef 指向 candidateSetId」），讓 CANDIDATE_SEARCH 這個 run type 之後
// 能被 Task 7-9 共用的 GET replay／runtime-snapshot 端點統一讀到。
func TestStartSearch_CreatesLedgerRunProjection(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, RunID: 900, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	got, err := uc.StartSearch(context.Background(), StartSearchInput{
		RequestID: 5, RequestRevisionID: 9, BuyerID: 1, RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.createLedgerRunCalled {
		t.Fatal("CreateLedgerRunForCandidateSearch was never called for a newly created candidate search")
	}
	if store.createLedgerRunSet.ID != 100 {
		t.Errorf("CreateLedgerRunForCandidateSearch received set.ID = %d, want 100 (this run's candidateSetId — becomes domain_ref_id in the repository layer)", store.createLedgerRunSet.ID)
	}
	if got.CandidateSetID != 100 {
		t.Errorf("StartSearchResult.CandidateSetID = %d, want 100", got.CandidateSetID)
	}
}

// TestStartSearch_LedgerProjectionFailureDoesNotBlockSearch 守住這個 side
// effect 的 fire-and-forget 契約：ledger 是投影不是 SSOT，寫入失敗只能記
// log，絕不能讓候選搜尋本身跟著失敗或不派工。
func TestStartSearch_LedgerProjectionFailureDoesNotBlockSearch(t *testing.T) {
	store := &fakeCandidateStore{
		set:                repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5},
		createLedgerRunErr: errors.New("boom: simulated ledger write failure"),
	}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	got, err := uc.StartSearch(context.Background(), StartSearchInput{
		RequestID: 5, RequestRevisionID: 9, BuyerID: 1, RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	if err != nil {
		t.Fatalf("ledger projection failure must not surface as a StartSearch error, got: %v", err)
	}
	if got.CandidateSetID != 100 {
		t.Errorf("CandidateSetID = %d, want 100 — the candidate search itself must still succeed", got.CandidateSetID)
	}
	if !store.createLedgerRunCalled {
		t.Fatal("CreateLedgerRunForCandidateSearch should still have been attempted")
	}
}

// fakeCandidateStoreIdempotent 模擬「已存在同一 revision+policy_version 的 run」。
type fakeCandidateStoreIdempotent struct {
	existing repository.CandidateSet
}

func (f *fakeCandidateStoreIdempotent) FindCandidateSet(context.Context, int64, int) (repository.CandidateSet, error) {
	return f.existing, nil
}
func (f *fakeCandidateStoreIdempotent) AppendRunEvent(context.Context, repository.AppendRunEventInput) (repository.RunEvent, error) {
	return repository.RunEvent{}, nil
}

func (f *fakeCandidateStoreIdempotent) UpsertRunArtifact(context.Context, repository.UpsertRunArtifactInput) (repository.RunArtifact, error) {
	return repository.RunArtifact{}, nil
}

func (f *fakeCandidateStoreIdempotent) CreateCandidateSet(context.Context, repository.CreateCandidateSetInput) (repository.CandidateSet, error) {
	panic("CreateCandidateSet 不該在冪等命中時被呼叫")
}
func (f *fakeCandidateStoreIdempotent) MarkCandidateSetRunning(context.Context, int64, int64) (bool, error) {
	return true, nil
}
func (f *fakeCandidateStoreIdempotent) FinalizeCandidateSet(context.Context, repository.FinalizeCandidateSetInput) (bool, string, string, error) {
	return true, codemeta.ProcurementRunStatusSucceeded.Value, "", nil
}

// GetCandidateSetRunWithCandidates 委派給同一個回傳值：這些 fake 都只用來測 StartSearch，
// 不驗兩把身分的解析差異（那是 repository 層 integration test 的事）。
func (f *fakeCandidateStoreIdempotent) GetCandidateSetRunWithCandidates(ctx context.Context, runID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	return f.GetCandidateSetWithCandidates(ctx, runID, requestID, buyerID)
}
func (f *fakeCandidateStoreIdempotent) GetCandidateSetWithCandidates(context.Context, int64, int64, identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	return f.existing, nil, nil
}
func (f *fakeCandidateStoreIdempotent) RetryCandidateSet(context.Context, int64, string, time.Duration) (repository.CandidateSet, error) {
	panic("RetryCandidateSet 不該被呼叫：這個 fake 的 existing.Status 固定是 COMPLETED，非 FAILED，不會觸發重試分支")
}
func (f *fakeCandidateStoreIdempotent) ListSelectedCandidateIDs(context.Context, int64) (map[int64]bool, error) {
	panic("ListSelectedCandidateIDs 不該被呼叫：這個 fake 只用於測 StartSearch 的冪等命中分支，不會被傳進 GetRun")
}
func (f *fakeCandidateStoreIdempotent) CreateLedgerRunForCandidateSearch(context.Context, repository.CandidateSet) error {
	panic("CreateLedgerRunForCandidateSearch 不該被呼叫：冪等命中（非 FAILED）沒有新建 candidate_set，也沒有新派工，不需要新的 ledger 投影")
}
func (f *fakeCandidateStoreIdempotent) TerminalizeRun(context.Context, repository.TerminalizeRunInput) (bool, error) {
	panic("TerminalizeRun 不該被呼叫：這個 fake 只用於測 StartSearch 的冪等命中分支，不會執行到 run()")
}

// fakeCandidateStoreRetryable 模擬「冪等鍵命中一筆已經被 reaper 收斂成 FAILED 的死 run」——
// StartSearch 應該呼叫 RetryCandidateSet 把它救回來重新派工並回傳新狀態，而不是照舊行為
// 原樣回傳這筆死掉的紀錄（見計畫 Design Decision #7：「買家重新呼叫即可拿到新的 run」）。
type fakeCandidateStoreRetryable struct {
	existing repository.CandidateSet
	retried  repository.CandidateSet
	retryErr error
	// refreshed 是「重試落敗之後重讀一次」會拿到的值——贏家已經寫進去的當前狀態。
	// 零值代表這個 fake 沒有在測落敗分支，FindCandidateSet 每次都回 existing。
	refreshed *repository.CandidateSet

	retryCalled       bool
	findCalls         int
	ledgerRunsCreated []repository.CandidateSet
	// markRunningCh 讓測試能確認背景 goroutine 真的重新執行到 run()（不只是 StartSearch
	// 同步回傳值看起來對），呼叫端用 select + timeout 等待這個訊號。
	markRunningCh chan struct{}
}

// FindCandidateSet 兩次呼叫回不同的值：第一次是 StartSearch 開頭的冪等查詢（那筆 FAILED
// 的舊快照），第二次是「重試落敗之後重讀一次」拿到的當前狀態。兩者刻意不同，才驗得出落敗
// 分支回的是哪一個——這正是那條路徑上會過期的東西從 status 變成 run **身分**之後的重點。
// 兩次都由 StartSearch 在呼叫端 goroutine 內同步執行，不需要鎖。
func (f *fakeCandidateStoreRetryable) FindCandidateSet(context.Context, int64, int) (repository.CandidateSet, error) {
	f.findCalls++
	if f.refreshed != nil && f.findCalls > 1 {
		return *f.refreshed, nil
	}
	return f.existing, nil
}
func (f *fakeCandidateStoreRetryable) AppendRunEvent(context.Context, repository.AppendRunEventInput) (repository.RunEvent, error) {
	return repository.RunEvent{}, nil
}

func (f *fakeCandidateStoreRetryable) UpsertRunArtifact(context.Context, repository.UpsertRunArtifactInput) (repository.RunArtifact, error) {
	return repository.RunArtifact{}, nil
}

func (f *fakeCandidateStoreRetryable) CreateCandidateSet(context.Context, repository.CreateCandidateSetInput) (repository.CandidateSet, error) {
	panic("CreateCandidateSet 不該在「冪等鍵命中一筆 FAILED 的死 run」時被呼叫——這種情況該走 RetryCandidateSet，不是新建一列（unique index 也會擋掉）")
}
func (f *fakeCandidateStoreRetryable) RetryCandidateSet(context.Context, int64, string, time.Duration) (repository.CandidateSet, error) {
	// 這一行只會在 StartSearch 呼叫端（測試主 goroutine）同步執行完才會派出背景
	// goroutine，讀寫這個欄位不需要額外的鎖。
	f.retryCalled = true
	if f.retryErr != nil {
		return repository.CandidateSet{}, f.retryErr
	}
	return f.retried, nil
}
func (f *fakeCandidateStoreRetryable) MarkCandidateSetRunning(context.Context, int64, int64) (bool, error) {
	if f.markRunningCh != nil {
		f.markRunningCh <- struct{}{}
	}
	return true, nil
}
func (f *fakeCandidateStoreRetryable) FinalizeCandidateSet(context.Context, repository.FinalizeCandidateSetInput) (bool, string, string, error) {
	return true, codemeta.ProcurementRunStatusSucceeded.Value, "", nil
}

// GetCandidateSetRunWithCandidates 委派給同一個回傳值：這些 fake 都只用來測 StartSearch，
// 不驗兩把身分的解析差異（那是 repository 層 integration test 的事）。
func (f *fakeCandidateStoreRetryable) GetCandidateSetRunWithCandidates(ctx context.Context, runID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	return f.GetCandidateSetWithCandidates(ctx, runID, requestID, buyerID)
}
func (f *fakeCandidateStoreRetryable) GetCandidateSetWithCandidates(context.Context, int64, int64, identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	return f.retried, nil, nil
}
func (f *fakeCandidateStoreRetryable) ListSelectedCandidateIDs(context.Context, int64) (map[int64]bool, error) {
	panic("ListSelectedCandidateIDs 不該被呼叫：這個 fake 只用於測 StartSearch 的重試分支，不會被傳進 GetRun")
}

// CreateLedgerRunForCandidateSearch 記下被投影的那一次嘗試。重試路徑**必須**呼叫它：
// 重試會換一個新的 run 身分，那一列 ledger run 不存在的話，買家拿到的 runId 訂閱不到任何
// 事件、終態也無處可寫。（舊版這裡是 panic，守的是「重試沿用同一個 run_id、不重新投影」
// 那個已被推翻的模型。）
func (f *fakeCandidateStoreRetryable) CreateLedgerRunForCandidateSearch(_ context.Context, set repository.CandidateSet) error {
	f.ledgerRunsCreated = append(f.ledgerRunsCreated, set)
	return nil
}
func (f *fakeCandidateStoreRetryable) TerminalizeRun(context.Context, repository.TerminalizeRunInput) (bool, error) {
	return true, nil
}

// TestDispatch_PanicInBackgroundRun_DoesNotCrashProcess is a characterization
// test, not a RED→GREEN one: dispatch() (candidate_search.go:126-154) already
// has a hand-rolled defer/recover today (see plan's Global Constraints — the
// task contract's "無防護裸 goroutine" description of this site turned out to
// be inaccurate; it already recovers panics, just via duplicated code instead
// of pkg/syncutil). This test passes against both the old and the new
// implementation — it exists to pin the invariant this refactor must not
// break: a panic anywhere in the background run (here, before the lane
// fan-out, exercising a different code path than searchLane's own per-lane
// recover covered by TestRun_PlatformLanePanics_RecoveredAsFailedAndExternalUnaffected)
// must not crash the test process, and dispatch() itself must still return
// immediately (fire-and-forget) rather than blocking on the background work.
func TestDispatch_PanicInBackgroundRun_DoesNotCrashProcess(t *testing.T) {
	store := &fakeCandidateStore{
		set:                repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5},
		panicOnMarkRunning: true,
		panicReached:       make(chan struct{}),
	}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	returned := make(chan struct{})
	go func() {
		uc.dispatch(context.Background(), store.set, procurementdomain.Slots{})
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("dispatch() 應該立刻回傳（fire-and-forget），卻卡住了")
	}

	// 沒有讓測試 process 被未 recover 的 panic 打掛，就是這個測試的核心斷言——
	// 同 pkg/syncutil 的 TestSafeGo_PanicDoesNotCrashOrBlockCaller。等 panicReached
	// 關閉（MarkCandidateSetRunning 呼叫 panic() 之前的最後一步）取代 time.Sleep：
	// 一旦確定 panic 陳述式已執行，run() 之後的同步程式碼——包含 FinalizeCandidateSet
	// 呼叫——依 Go panic 語意保證不會再跑，不需要額外猜要睡多久才夠讓 recover 完成。
	select {
	case <-store.panicReached:
	case <-time.After(time.Second):
		t.Fatal("MarkCandidateSetRunning 應該要被呼叫到並 panic，卻沒有觀察到")
	}
	if store.finalizeCalled {
		t.Errorf("MarkCandidateSetRunning panic 之後不應該還呼叫 FinalizeCandidateSet")
	}
}

// seamCheckStore 只用來釘住 GetRun 呼叫的是哪一個介面方法。GetCandidateSetWithCandidates
// 直接 panic：如果有人把 GetRun 內的呼叫改回它（例如合併衝突、複製貼上寫錯），這支測試
// 必須紅，而不是像改動前那樣——把 GetCandidateSetRunWithCandidates 換回
// GetCandidateSetWithCandidates 不會讓任何既有測試失敗（fakeCandidateStore 兩個方法回傳
// 同一個值，兩條路徑测起来無法區分）。
type seamCheckStore struct {
	CandidateSearchStore // nil：其他方法本測試不會呼叫，呼叫到就是 nil pointer panic
	runWithCandidates    repository.CandidateSet
	gotRunID             int64
}

func (s *seamCheckStore) GetCandidateSetWithCandidates(context.Context, int64, int64, identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	panic("GetRun 必須呼叫 GetCandidateSetRunWithCandidates（run 身分），不是 GetCandidateSetWithCandidates（candidate set 身分）——見設計書 57 §1.2-3")
}
func (s *seamCheckStore) GetCandidateSetRunWithCandidates(_ context.Context, runID, _ int64, _ identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	s.gotRunID = runID
	return s.runWithCandidates, nil, nil
}
func (s *seamCheckStore) ListSelectedCandidateIDs(context.Context, int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

func TestGetRun_UsesRunIdentityNotCandidateSetIdentity(t *testing.T) {
	store := &seamCheckStore{runWithCandidates: repository.CandidateSet{ID: 501, RunID: 601, Status: codemeta.ProcurementRunStatusSucceeded.Value}}
	uc := NewCandidateSearchUseCase(store, stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	const askedRunID = int64(601)
	set, _, _, err := uc.GetRun(context.Background(), GetRunInput{RunID: askedRunID, RequestID: 77, BuyerID: identity.BuyerID(1)})
	if err != nil {
		t.Fatalf("GetRun error = %v, want nil", err)
	}
	if store.gotRunID != askedRunID {
		t.Fatalf("GetCandidateSetRunWithCandidates 收到的 runID = %d, want %d", store.gotRunID, askedRunID)
	}
	if set.ID != 501 {
		t.Fatalf("set.ID = %d, want 501（stub 回傳值原樣透傳）", set.ID)
	}
}

// testRailIDs 是 Activity 步驟列 row id 的確定性替身：從 9001 起遞增，讓測試
// 能直接寫出期望的 rowId／parentRowId 而不必攔截 TSID 產生器。
func testRailIDs() func() int64 {
	next := int64(9000)
	return func() int64 {
		next++
		return next
	}
}

// ---- T23：候選搜尋 Activity 步驟列 ----

// TestRun_Rail_DeclaresPendingPlanThenAdvancesRows 是 rail 的骨幹測試：run 一
// 開始就宣告完整的前瞻清單（每列 PENDING，＝設計稿 01 的「等待中」），之後每
// 一步的真實事件把對應的列就地推進。
func TestRun_Rail_DeclaresPendingPlanThenAdvancesRows(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, RunID: 900, BuyerID: 7, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1"), validEvidenceCandidate("2")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("3")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	// 前 4 筆事件必須是完整清單、全部 PENDING——買家在任何一步真的開始之前，
	// 就該看得到「接下來會做哪幾步」。
	if len(store.railEvents) < 4 {
		t.Fatalf("rail events = %d, want at least the 4-row plan", len(store.railEvents))
	}
	plan := procurementdomain.CandidateSearchRailPlan()
	for i, row := range plan {
		var got map[string]any
		if err := json.Unmarshal(store.railEvents[i].PublicPayloadJSON, &got); err != nil {
			t.Fatalf("plan event %d is not valid JSON: %v", i, err)
		}
		if got["code"] != string(row.Code) {
			t.Errorf("plan event %d code = %v, want %q（順序即顯示順序）", i, got["code"], row.Code)
		}
		if got["status"] != codemeta.ProcurementStepStatusPending.Value {
			t.Errorf("plan event %d status = %v, want PENDING", i, got["status"])
		}
		if got["label"] != procurementdomain.StepLabels[row.Code] {
			t.Errorf("plan event %d label = %v, want %q", i, got["label"], procurementdomain.StepLabels[row.Code])
		}
	}

	// 合併後的終態（consumer 的 merge-by-rowId 行為）。
	rows := store.railRows(t)
	for code, wantStatus := range map[procurementdomain.StepCode]string{
		procurementdomain.StepSearchCandidates: codemeta.ProcurementStepStatusSucceeded.Value,
		procurementdomain.StepSearchPlatform:   codemeta.ProcurementStepStatusSucceeded.Value,
		procurementdomain.StepSearchExternal:   codemeta.ProcurementStepStatusSucceeded.Value,
		procurementdomain.StepSummarize:        codemeta.ProcurementStepStatusSucceeded.Value,
	} {
		row, ok := rows[string(code)]
		if !ok {
			t.Fatalf("rail is missing row %q", code)
		}
		if row["status"] != wantStatus {
			t.Errorf("row %q final status = %v, want %v", code, row["status"], wantStatus)
		}
	}

	// 計數只掛在兩條 lane 上，且是字串（計數依 API/Proto 規範 §6 對外用字串）。
	if got := rows[string(procurementdomain.StepSearchPlatform)]["count"]; got != "2" {
		t.Errorf("platform lane count = %v, want \"2\"", got)
	}
	if got := rows[string(procurementdomain.StepSearchExternal)]["count"]; got != "1" {
		t.Errorf("external lane count = %v, want \"1\"", got)
	}
	if _, has := rows[string(procurementdomain.StepSummarize)]["count"]; has {
		t.Error("整理候選清單這一列沒有計數概念，不該帶 count")
	}
}

// TestRun_Rail_NestsLaneRowsUnderParent 釘住巢狀：兩條 lane 子列的 parentRowId
// 必須指向父列自己的 rowId，而父列不得有 parentRowId（只支援一層）。
func TestRun_Rail_NestsLaneRowsUnderParent(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, RunID: 900, BuyerID: 7, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	rows := store.railRows(t)
	parent := rows[string(procurementdomain.StepSearchCandidates)]
	if _, has := parent["parentRowId"]; has {
		t.Error("父列不得帶 parentRowId——rail 只支援一層巢狀")
	}
	parentRowID := parent["rowId"]
	for _, code := range []procurementdomain.StepCode{
		procurementdomain.StepSearchPlatform, procurementdomain.StepSearchExternal,
	} {
		if got := rows[string(code)]["parentRowId"]; got != parentRowID {
			t.Errorf("row %q parentRowId = %v, want %v（父列的 rowId）", code, got, parentRowID)
		}
	}
	if got := rows[string(procurementdomain.StepSummarize)]["parentRowId"]; got != nil {
		t.Errorf("整理候選清單是 top-level 列，不該有 parentRowId，got %v", got)
	}
}

// TestRun_Rail_LaneFailureIsAStatusNotACode 是 T23「FAILED 放 status 軸」裁決
// 的落地測試：lane 失敗時那一列的 code 仍然是 SEARCH_EXTERNAL、只有 status 變
// FAILED；rail 上不會冒出一個 code=FAILED 的列。
//
// 同時釘住失敗時不帶 count：「查失敗」與「查到 0 家」對買家是兩件不同的事，
// 後者是有效結果，補一個 0 會把前者顯示成後者。
func TestRun_Rail_LaneFailureIsAStatusNotACode(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, RunID: 900, BuyerID: 7, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1")}},
		stubExternalSearcher{err: errors.New("boom")},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	rows := store.railRows(t)
	external := rows[string(procurementdomain.StepSearchExternal)]
	if external["status"] != codemeta.ProcurementStepStatusFailed.Value {
		t.Errorf("failed lane status = %v, want FAILED", external["status"])
	}
	if external["code"] != string(procurementdomain.StepSearchExternal) {
		t.Errorf("failed lane code = %v, want SEARCH_EXTERNAL（失敗是 status 不是 code）", external["code"])
	}
	if _, has := external["count"]; has {
		t.Error("查失敗的 lane 不得帶 count——那會跟「查到 0 家」混淆")
	}
	if _, has := rows["FAILED"]; has {
		t.Error("rail 上不得出現 code=FAILED 的列")
	}
	// 一條 lane 成功就代表這個階段對買家有產出——跟 FinalizeCandidateSet 只在
	// 兩條都失敗時才記 FAILED 是同一個判準。
	if got := rows[string(procurementdomain.StepSearchCandidates)]["status"]; got != codemeta.ProcurementStepStatusSucceeded.Value {
		t.Errorf("parent status = %v, want SUCCEEDED（還有一條 lane 成功）", got)
	}
}

// TestRun_Rail_BothLanesFail_ParentIsFailed 補上 parentRailStatus 的另一半。
func TestRun_Rail_BothLanesFail_ParentIsFailed(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, RunID: 900, BuyerID: 7, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{err: errors.New("boom")},
		stubExternalSearcher{err: errors.New("boom")},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	rows := store.railRows(t)
	if got := rows[string(procurementdomain.StepSearchCandidates)]["status"]; got != codemeta.ProcurementStepStatusFailed.Value {
		t.Errorf("parent status = %v, want FAILED（兩條 lane 都失敗）", got)
	}
}

// TestRun_Rail_TargetsThePublicRunID 釘住 rail 寫到的是這個 run 對外的身分
// （set.PublicRunID()），不是 candidateSetId——買家拿到的 runId 是前者，寫錯
// 這個值等於整條 rail 送進一個沒有人在看的 run。
func TestRun_Rail_TargetsThePublicRunID(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, RunID: 900, BuyerID: 7, FencingToken: 1, RequestID: 5}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if len(store.railEvents) == 0 {
		t.Fatal("no rail events emitted")
	}
	for i, ev := range store.railEvents {
		if ev.RunID != 900 {
			t.Fatalf("rail event %d RunID = %d, want 900 (set.PublicRunID(), not candidateSetId 100)", i, ev.RunID)
		}
		if ev.BuyerID != identity.BuyerID(7) {
			t.Fatalf("rail event %d BuyerID = %v, want 7", i, ev.BuyerID)
		}
		if ev.EventType != codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value {
			t.Fatalf("rail event %d EventType = %q, want AGENT_ACTIVITY_SNAPSHOT（與 graph 路徑同一種 frame）", i, ev.EventType)
		}
	}
}

// TestRun_Rail_ClaimLost_EmitsNothing：claim 失敗代表這個 worker 不是這個 run
// 的擁有者，寫步驟列會在買家畫面上疊出兩份互相打架的進度。
func TestRun_Rail_ClaimLost_EmitsNothing(t *testing.T) {
	store := &fakeCandidateStore{
		set:        repository.CandidateSet{ID: 100, RunID: 900, BuyerID: 7, FencingToken: 1, RequestID: 5},
		claimFails: true,
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{}, stubExternalSearcher{}, testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if len(store.railEvents) != 0 {
		t.Fatalf("rail events = %d, want 0（認領失敗的 worker 不得寫進度）", len(store.railEvents))
	}
}

// TestRun_Rail_LedgerWriteFailure_DoesNotBreakSearch：rail 是投影不是 SSOT，
// 寫不進去不該讓買家的候選搜尋整趟失敗——跟本檔既有的 CreateLedgerRunForCandidateSearch
// ／TerminalizeRun 投影同一套紀律。
func TestRun_Rail_LedgerWriteFailure_DoesNotBreakSearch(t *testing.T) {
	store := &fakeCandidateStore{
		set:     repository.CandidateSet{ID: 100, RunID: 900, BuyerID: 7, FencingToken: 1, RequestID: 5},
		railErr: errors.New("ledger down"),
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("2")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if !store.finalizeCalled {
		t.Fatal("FinalizeCandidateSet was never called — rail 寫入失敗不該中斷搜尋")
	}
}

// validEvidenceCandidate 是一個滿足 doc 57 §1.6 evidence contract 的最小候選。
// 三欄以外的欄位不填——這個 helper 只為「能通過閘門」而存在。
func validEvidenceCandidate(sourceID string) repository.Candidate {
	return repository.Candidate{
		SourceID:              sourceID,
		MatchedFactsJSON:      []byte("[]"),
		UnverifiableFactsJSON: []byte("[]"),
		FactsAsOf:             time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC),
	}
}

// TestRun_CandidateFailingEvidenceContract_IsDroppedBeforeFinalize 釘住 doc 57 §1.6
// 的記錄層閘門：三欄不合契約的候選不得被寫進 candidate set，也就不會被呈現。
//
// 這條防線的對象是**未來新增的 lane 或 lane 內的重構**，不是現行兩條 lane——它們
// 今天都自己把三欄填對了。閘門的價值在於那個不變式不再依賴「每個作者都記得」：
// 少填 factsAsOf 的候選會在買家畫面上渲染出 0001-01-01，那比不顯示更糟。
func TestRun_CandidateFailingEvidenceContract_IsDroppedBeforeFinalize(t *testing.T) {
	store := &fakeCandidateStore{set: repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5}}
	good := validEvidenceCandidate("good")
	missingFactsAsOf := validEvidenceCandidate("no-facts-as-of")
	missingFactsAsOf.FactsAsOf = time.Time{}
	nullFacts := validEvidenceCandidate("null-facts")
	nullFacts.UnverifiableFactsJSON = []byte("null")

	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{good, missingFactsAsOf}},
		stubExternalSearcher{candidates: []repository.Candidate{nullFacts}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	platform := store.finalizeInput.Platform
	if len(platform.Candidates) != 1 || platform.Candidates[0].SourceID != "good" {
		t.Errorf("Platform.Candidates = %+v, want 只留下 good", platform.Candidates)
	}
	// 整條 lane 不因為一筆壞候選而失敗：合法的那筆仍要交付。
	if platform.Status != codemeta.ProcurementLaneStatusSucceeded.Value {
		t.Errorf("Platform.Status = %q, want SUCCEEDED（單筆被擋不等於整條 lane 失敗）", platform.Status)
	}
	if len(store.finalizeInput.External.Candidates) != 0 {
		t.Errorf("External.Candidates = %+v, want 全數被擋下", store.finalizeInput.External.Candidates)
	}
}

// persistedCandidate 是一筆「已經落庫、拿到 ID」的候選，供 CANDIDATE_RESULTS
// artifact 的 producer 測試使用。
func persistedCandidate(id int64, lane procurementdomain.Lane, sourceID string) repository.Candidate {
	c := validEvidenceCandidate(sourceID)
	c.ID = id
	c.Lane = string(lane)
	c.DisplayName = "候選 " + sourceID
	return c
}

// TestRun_Succeeded_ProducesCandidateResultsArtifact 是 Slice F 的第一個真 producer：
// 13 個 artifact 型別在此之前全部零 producer，寫入路徑（表在 schema 裡但零 Go 碼碰它）
// 也從未被任何一條真實流程走過。CANDIDATE_RESULTS 是 4 個已展開 data 形狀的型別之一，
// 拿它當第一條路徑，不需要預先發明任何未經業務設計驗證的欄位（doc 57 §4.5）。
func TestRun_Succeeded_ProducesCandidateResultsArtifact(t *testing.T) {
	store := &fakeCandidateStore{
		set: repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5, BuyerID: 42},
		persistedCandidates: []repository.Candidate{
			persistedCandidate(9001, procurementdomain.LanePlatform, "p1"),
			persistedCandidate(9002, procurementdomain.LaneExternal, "e1"),
		},
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("p1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("e1")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if len(store.upsertArtifactCalls) != 1 {
		t.Fatalf("UpsertRunArtifact 呼叫 %d 次, want 1", len(store.upsertArtifactCalls))
	}
	got := store.upsertArtifactCalls[0]
	if got.ArtifactType != codemeta.ProcurementArtifactTypeCandidateResults.Value {
		t.Errorf("ArtifactType = %q, want CANDIDATE_RESULTS", got.ArtifactType)
	}
	if got.Status != codemeta.ProcurementArtifactStatusReady.Value {
		t.Errorf("Status = %q, want READY", got.Status)
	}
	if got.RunID != store.set.PublicRunID() {
		t.Errorf("RunID = %d, want %d（artifact 掛在 run 身分上，不是 candidateSetId）", got.RunID, store.set.PublicRunID())
	}
	// domainRef 指回產製來源，供 owner restore 用（doc 57 §4.5 allowlisted）。
	if got.DomainRefType != "candidateSetId" || got.DomainRefID != "100" {
		t.Errorf("domainRef = %s/%s, want candidateSetId/100", got.DomainRefType, got.DomainRefID)
	}
	if got.Checksum == "" {
		t.Error("Checksum 不得為空")
	}

	var data procurementdomain.CandidateResultsData
	if err := json.Unmarshal(got.PublicDataJSON, &data); err != nil {
		t.Fatalf("PublicDataJSON 不是 CandidateResultsData: %v (%s)", err, got.PublicDataJSON)
	}
	if len(data.PlatformLane.Candidates) != 1 || data.PlatformLane.Candidates[0].CandidateID != 9001 {
		t.Errorf("PlatformLane.Candidates = %+v, want 一筆 id=9001", data.PlatformLane.Candidates)
	}
	if len(data.ExternalLane.Candidates) != 1 || data.ExternalLane.Candidates[0].CandidateID != 9002 {
		t.Errorf("ExternalLane.Candidates = %+v, want 一筆 id=9002", data.ExternalLane.Candidates)
	}
}

// TestRun_ArtifactIsProducedBeforeTerminalEvent 釘住 ledger 事件順序：終態事件必須
// 是這個 run 的最後一筆。artifact 在 RUN_FINISHED 之後才進 ledger，等於買家 replay
// 到終態、關掉訂閱之後才收到一個他永遠看不到的事件。
func TestRun_ArtifactIsProducedBeforeTerminalEvent(t *testing.T) {
	store := &orderRecordingStore{fakeCandidateStore: &fakeCandidateStore{
		set: repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5, BuyerID: 42},
	}}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("p1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("e1")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	want := []string{"artifact", "terminalize"}
	if len(store.order) != 2 || store.order[0] != want[0] || store.order[1] != want[1] {
		t.Errorf("呼叫順序 = %v, want %v", store.order, want)
	}
}

// orderRecordingStore 只記錄兩個寫入的先後，其餘行為完全沿用內嵌的 fake。
type orderRecordingStore struct {
	*fakeCandidateStore
	order []string
}

func (s *orderRecordingStore) UpsertRunArtifact(ctx context.Context, in repository.UpsertRunArtifactInput) (repository.RunArtifact, error) {
	s.order = append(s.order, "artifact")
	return s.fakeCandidateStore.UpsertRunArtifact(ctx, in)
}

func (s *orderRecordingStore) TerminalizeRun(ctx context.Context, in repository.TerminalizeRunInput) (bool, error) {
	s.order = append(s.order, "terminalize")
	return s.fakeCandidateStore.TerminalizeRun(ctx, in)
}

// TestRun_Failed_DoesNotProduceCandidateResultsArtifact：run 失敗時沒有候選結果可以
// 宣告。硬產一個空的 CANDIDATE_RESULTS 等於對前端宣稱「搜尋完成、結果是零家」，而實
// 際上是搜尋失敗——那是 ERROR_RECOVERY 的職責，不是這一型的。
func TestRun_Failed_DoesNotProduceCandidateResultsArtifact(t *testing.T) {
	store := &fakeCandidateStore{
		set:            repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5, BuyerID: 42},
		finalizeStatus: codemeta.ProcurementRunStatusFailed.Value,
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{err: context.DeadlineExceeded},
		stubExternalSearcher{err: context.DeadlineExceeded},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	// ⚠️ 判準是**型別**不是「一筆都沒有」：ERROR_RECOVERY producer 落地後，失敗的 run
	// 會產出那一型（見 TestRun_FinalizeFails_ProducesErrorRecoveryArtifact）。本測試
	// 守的是「不得謊稱搜尋完成」，不是「失敗時什麼都不產」——後者曾經成立，但它是那時
	// 還沒有誠實替代品的副作用，不是這條規則本身。
	for _, call := range store.upsertArtifactCalls {
		if call.ArtifactType == codemeta.ProcurementArtifactTypeCandidateResults.Value {
			t.Errorf("run 失敗時不得產製 CANDIDATE_RESULTS, got %+v", call)
		}
	}
}

// TestRun_ArtifactWriteFails_DoesNotBreakRun：artifact 是投影不是 SSOT，
// 同 CreateLedgerRunForCandidateSearch／TerminalizeRun 的既有取捨——寫失敗只記 log，
// 不能讓一個新增的觀測層把已經寫定的候選結果拖下水。
func TestRun_ArtifactWriteFails_DoesNotBreakRun(t *testing.T) {
	store := &fakeCandidateStore{
		set:               repository.CandidateSet{ID: 100, FencingToken: 1, RequestID: 5, BuyerID: 42},
		upsertArtifactErr: errors.New("boom"),
	}
	uc := NewCandidateSearchUseCase(store,
		stubPlatformSearcher{candidates: []repository.Candidate{validEvidenceCandidate("p1")}},
		stubExternalSearcher{candidates: []repository.Candidate{validEvidenceCandidate("e1")}},
		testRailIDs())

	uc.run(context.Background(), store.set, procurementdomain.Slots{})

	if !store.terminalizeCalled {
		t.Error("artifact 寫入失敗不得中斷後續的 ledger 終態投影")
	}
}
