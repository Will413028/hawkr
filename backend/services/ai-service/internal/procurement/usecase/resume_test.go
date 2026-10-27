package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// fakeResumeStore 是 usecase.ResumeRunStore 的記憶體替身（Task 14b）：內嵌
// fakeRunStore 取得 ClaimRun／AppendRunEvent／TerminalizeRun／CreateInterrupt／
// HasIdempotencyKey／FindOpenInterruptForRequest，只額外加 ResumeRun——並且
// 真的呼叫呼叫端（ClaimAndDispatchResume）傳進來的 ResolveInput closure，跟
// 真正的 repository.ResumeRun 一樣在「T0 交易內」執行它，這樣測到的是 M4／M5
// 語意邏輯本身（kind／schema 檢查、payload 解碼、input_ref 組裝），不只是
// dispatch 編排。
type fakeResumeStore struct {
	*fakeRunStore

	isNew      bool
	childID    int64
	resumeErr  error // 不呼叫 ResolveInput 就直接回錯——模擬 T0 guard（步驟 1-7）失敗
	resolveCtx repository.ResumeResolveContext

	lastResolved repository.ResolvedRunInput

	restoreCalls  []repository.RestoreInterruptAfterFailedResumeInput
	restoreResult bool
	restoreErr    error
}

func (f *fakeResumeStore) RestoreInterruptAfterFailedResume(
	_ context.Context, in repository.RestoreInterruptAfterFailedResumeInput,
) (bool, error) {
	f.restoreCalls = append(f.restoreCalls, in)
	return f.restoreResult, f.restoreErr
}

func newFakeResumeStore() *fakeResumeStore {
	return &fakeResumeStore{fakeRunStore: newFakeRunStore(), isNew: true, childID: 9500, restoreResult: true}
}

func (f *fakeResumeStore) ResumeRun(ctx context.Context, in repository.ResumeRunInput) (repository.ResumeRunOutput, error) {
	if f.resumeErr != nil {
		return repository.ResumeRunOutput{}, f.resumeErr
	}
	if !f.isNew {
		return repository.ResumeRunOutput{Child: repository.Run{ID: f.childID, Status: domain.RunStatusQueued()}}, nil
	}
	// 同真正 repository.ResumeRun 的組法：kind／version／RequestID／
	// ExpectedRevisionID 是這個 fixture 事先配置好的（模擬 T0 從鎖住的
	// interrupt／parent row 讀出來的值），ResponsePayload 則必須是這次呼叫
	// 真正帶進來的 in.ResponsePayload——早期版本忘記轉傳這個欄位，導致
	// ResolveInput 收到的 rc.ResponsePayload 永遠是 nil，json.Unmarshal(nil,
	// ...) 失敗、所有本該成功的測試都誤判成 ErrUnsupportedInterruptKind。
	rc := f.resolveCtx
	rc.ResponsePayload = in.ResponsePayload
	resolved, err := in.ResolveInput(ctx, rc)
	if err != nil {
		return repository.ResumeRunOutput{}, err
	}
	f.lastResolved = resolved
	return repository.ResumeRunOutput{
		Child: repository.Run{
			ID: f.childID, BuyerID: in.BuyerID, RunType: domain.RunTypeClarificationResume,
			Status: domain.RunStatusQueued(), Version: 1, LastEventSeq: 1,
		},
		IsNew: true, Resolved: resolved,
	}, nil
}

func resumeCropInterruptCtx(reqID, revID int64) repository.ResumeResolveContext {
	asked, err := json.Marshal(pendingCropFixture())
	if err != nil {
		panic(err)
	}
	return repository.ResumeResolveContext{
		InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1",
		RequestID: reqID, ExpectedRevisionID: revID,
		// 伺服器當初問的那一題：真正的 repository.ResumeRun 從被 FOR UPDATE
		// 鎖住的那一列 interrupt 讀出 payload_json 放進這個欄位。
		InterruptPayload: asked,
	}
}

// TestClaimAndDispatchResume_HappyPath_ChildSucceedsAndDispatchesAnswerPipeline
// 是這個 task 修的核心 bug 的正向釘子：child run 真的被執行（不再永遠停在
// QUEUED），走完整套 executeAnswerPipeline（複用既有、完全未修改的
// AnswerUseCase.composeAnswerViaGraph——procurement graph cutover，
// 2026-08-31，前身 Handle 已刪除），以 SUCCEEDED 收尾，且 input_ref 帶著伺服器
// 推導完成的 resolved input（B2）。
func TestClaimAndDispatchResume_HappyPath_ChildSucceedsAndRunsTheGraph(t *testing.T) {
	store, _, deps := newGraphResumeFixture(t, "seed-resume-happy")

	payload := []byte(`{"optionId":"crop-CROP-X"}`)
	out, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-happy-1", ClientRequestID: "cr-resume-happy-1", ResponsePayload: payload,
	}, store, NewRunDispatcher(), deps)

	require.NoError(t, err)
	require.Equal(t, int64(9500), out.RunID)
	require.Equal(t, domain.RunStatusSucceeded(), out.RunStatus)

	// child 真的被執行過——這正是 Task 14 留下、當時修的 plan-wide Critical
	// gap：child 不能停在 QUEUED。resume 只剩 graph 一條控制流之後，證據是
	// runGraphOutcome 自己的終態化（TerminalizeRun 一次、SUCCEEDED、帶
	// RUN_FINISHED 終態事件型別），不是 answer pipeline 那種逐 step 的
	// AppendRunEvent 序列——這個最小 graph fixture 不產生中間步驟事件。
	require.Len(t, store.terminals, 1, "child run 必須真的跑完 graph 並被終態化，不能停在 QUEUED")
	require.Equal(t, domain.RunStatusSucceeded(), store.terminals[0].Status)
	require.Equal(t, int64(9500), store.terminals[0].RunID)
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunFinished.Value, store.terminals[0].TerminalEventType)

	// B2：input_ref 是伺服器推導完成的 resolved input，帶著 requestId／
	// expectedRevisionId／slotKey（伺服器推導）與 optionId（買家送的）。
	require.Equal(t, "v1", store.lastResolved.InputSchemaVersion)
	require.Equal(t, "BUYER_INPUT", store.lastResolved.RetentionClass)
	require.Contains(t, string(store.lastResolved.InputRefJSON), `"optionId":"crop-CROP-X"`)
	require.Contains(t, string(store.lastResolved.InputRefJSON), `"slotKey":"crop_disambiguation"`)
}

// TestClaimAndDispatchResume_NotNew_DoesNotDispatch 是 C2 裁決的正向釘子：
// 冪等命中（!IsNew）只回現況，不 dispatch、不補派、不重新終態化——與
// ClaimAndDispatchTurn／ClaimAndDispatchAnswer／ClaimAndDispatchRewind 的
// !isNew 分支同形狀。
func TestClaimAndDispatchResume_NotNew_DoesNotDispatch(t *testing.T) {
	store := newFakeResumeStore()
	store.isNew = false
	store.childID = 7000

	// answer 傳 nil：C2 裁決之下 !IsNew 分支完全不會 dereference 它，這裡故意
	// 傳 nil 讓測試本身成為「!IsNew 真的沒有嘗試 dispatch」的斷言。
	out, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-replay-1", ClientRequestID: "cr-resume-replay-1",
	}, store, NewRunDispatcher(), GraphControllerDeps{})

	require.NoError(t, err)
	require.Equal(t, int64(7000), out.RunID)
	require.Equal(t, domain.RunStatusQueued(), out.RunStatus, "!IsNew 只回現況——這裡現況固定回 QUEUED，模擬 crash 殘留的已知缺口（C2 §6.1）")
	require.Empty(t, store.events, "!IsNew 不得 dispatch，不得產生任何 event")
	require.Empty(t, store.terminals, "!IsNew 不得重新終態化")
}

// TestClaimAndDispatchResume_UnsupportedInterruptKind_ZeroMutation 釘住 M5：
// kind／schema version guard 在 T0 交易內、任何 mutation 之前執行——這裡的
// fakeResumeStore.ResumeRun 完全比照真正 repository 的順序（先呼叫
// ResolveInput，失敗就直接回錯，不建立 child、不寫任何 event），所以
// store.events／terminals／interrupts 全部保持空，就是「零 mutation」的斷言。
func TestClaimAndDispatchResume_UnsupportedInterruptKind_ZeroMutation(t *testing.T) {
	store := newFakeResumeStore()
	store.resolveCtx = repository.ResumeResolveContext{InterruptKind: "QUOTE_APPROVAL", ResponseSchemaVersion: "v1"}

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-unsupported-1", ClientRequestID: "cr-resume-unsupported-1", ResponsePayload: []byte(`{}`),
	}, store, NewRunDispatcher(), GraphControllerDeps{})

	require.ErrorIs(t, err, ErrUnsupportedInterruptKind)
	require.Empty(t, store.events)
	require.Empty(t, store.terminals)
	require.Empty(t, store.interrupts)
}

// TestClaimAndDispatchResume_UnsupportedSchemaVersion_ZeroMutation 是上一個
// 測試的另一半：kind 對但 version 不是 v1。
func TestClaimAndDispatchResume_UnsupportedSchemaVersion_ZeroMutation(t *testing.T) {
	store := newFakeResumeStore()
	store.resolveCtx = repository.ResumeResolveContext{InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v2"}

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-unsupported-2", ClientRequestID: "cr-resume-unsupported-2", ResponsePayload: []byte(`{"optionId":"crop-a"}`),
	}, store, NewRunDispatcher(), GraphControllerDeps{})

	require.ErrorIs(t, err, ErrUnsupportedInterruptKind)
	require.Empty(t, store.events)
}

// TestClaimAndDispatchResume_MalformedPayload_ReturnsErrUnsupportedInterruptKind
// 釘住 payload 不是合法 JSON 時的分類——這也是「payload 語意上解不出來」的
// 一種，同樣映射 ErrUnsupportedInterruptKind（見 ClaimAndDispatchResume 內
// ResolveInput closure 的分支）。
func TestClaimAndDispatchResume_MalformedPayload_ReturnsErrUnsupportedInterruptKind(t *testing.T) {
	store := newFakeResumeStore()
	store.resolveCtx = repository.ResumeResolveContext{InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1"}

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-malformed-1", ClientRequestID: "cr-resume-malformed-1", ResponsePayload: []byte(`not json`),
	}, store, NewRunDispatcher(), GraphControllerDeps{})

	require.ErrorIs(t, err, ErrUnsupportedInterruptKind)
}

// TestClaimAndDispatchResume_MissingOptionID_ReturnsErrInvalidOptionID 釘住
// payload 解得出 JSON、但 optionId 是空字串——這是內容非法而不是格式不受支援，
// 映射 ErrInvalidOptionID（422 ANSWER_SCHEMA_INVALID），與 kind／version 不
// 支援（400 SCHEMA_VERSION_UNSUPPORTED）是兩種不同的錯誤。
func TestClaimAndDispatchResume_MissingOptionID_ReturnsErrInvalidOptionID(t *testing.T) {
	store := newFakeResumeStore()
	store.resolveCtx = repository.ResumeResolveContext{InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1"}

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-missing-opt-1", ClientRequestID: "cr-resume-missing-opt-1", ResponsePayload: []byte(`{}`),
	}, store, NewRunDispatcher(), GraphControllerDeps{})

	require.ErrorIs(t, err, ErrInvalidOptionID)
}

// TestClaimAndDispatchResume_AppendRunEventFails_StillTerminalizesCorrectly
// 釘住 m11 已知缺口在 resume 路徑上同樣成立且不阻斷：ledger event 寫入失敗
// 只記 log 續跑，child 仍正確終態化成 SUCCEEDED。
func TestClaimAndDispatchResume_AppendRunEventFails_StillTerminalizesCorrectly(t *testing.T) {
	store, _, deps := newGraphResumeFixture(t, "seed-resume-appendfail")
	store.appendErr = errBoom

	out, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-appendfail-1", ClientRequestID: "cr-resume-appendfail-1",
		ResponsePayload: []byte(`{"optionId":"crop-CROP-X"}`),
	}, store, NewRunDispatcher(), deps)

	require.NoError(t, err)
	require.Equal(t, domain.RunStatusSucceeded(), out.RunStatus)
	require.Len(t, store.terminals, 1)
	require.Equal(t, domain.RunStatusSucceeded(), store.terminals[0].Status)
}

// TestClaimAndDispatchResume_BackgroundPanic_DoesNotHangCaller 釘住背景執行
// panic 之後呼叫端不會被卡住，且 child run 被收成 FAILED——同
// TestClaimAndDispatchAnswer_BackgroundPanic_DoesNotHangCaller 的理由與斷言
// 形狀。
//
// panic 來源改成 graph 路徑真的會執行的那一層（resumeGraphTurn 開頭的
// GetSnapshot）——resume 已不再經過 answer pipeline，包 AnswerStore 的
// panickingAnswerStore 在這條路上根本不會被呼叫到。
func TestClaimAndDispatchResume_BackgroundPanic_DoesNotHangCaller(t *testing.T) {
	store, repo, deps := newGraphResumeFixture(t, "seed-resume-panic")
	deps.RevisionStore = panickingRevisionStore{RevisionStore: repo}

	type outcome struct {
		res ResumeResult
		err error
	}
	ch := make(chan outcome, 1)
	go func() {
		res, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
			BuyerID: 1234, InterruptID: "int-panic-1", ClientRequestID: "cr-resume-panic-1",
			ResponsePayload: []byte(`{"optionId":"crop-CROP-X"}`),
		}, store, NewRunDispatcher(), deps)
		ch <- outcome{res, err}
	}()

	select {
	case got := <-ch:
		require.Error(t, got.err, "panic 之後呼叫端必須拿到錯誤，不是零值結果")
	case <-time.After(5 * time.Second):
		t.Fatal("背景 panic 之後 ClaimAndDispatchResume 沒有返回——呼叫端被卡住了")
	}

	require.Len(t, store.terminals, 1)
	require.Equal(t, domain.RunStatusFailed(), store.terminals[0].Status)
	require.Equal(t, codemeta.ProcurementRunFailureReasonTurnPipelineError.Value, store.terminals[0].FailureReason)
}

// TestClaimAndDispatchResume_ResumeErr_PropagatesRaw 釘住 T0 guard（步驟 1-7）
// 失敗時（interrupt already resolved／not found／run not resumable／version
// conflict…）ClaimAndDispatchResume 原樣往上傳，不吞、不包裝。
func TestClaimAndDispatchResume_ResumeErr_PropagatesRaw(t *testing.T) {
	store := newFakeResumeStore()
	store.resumeErr = repository.ErrInterruptAlreadyResolved

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: identity.BuyerID(1234), InterruptID: "int-resolved-1", ClientRequestID: "cr-resume-resolved-1",
	}, store, NewRunDispatcher(), GraphControllerDeps{})

	require.ErrorIs(t, err, repository.ErrInterruptAlreadyResolved)
}

// newTerminalCompletionRunnerWithSession is newTerminalCompletionRunner's
// (graph_dispatch_test.go) resume-test sibling: same Start -> seed ->
// ready_to_finalize_terminal chain, but takes a caller-supplied
// session.Service instead of always creating a fresh, empty one — so a
// test can pre-seed the session state a genuine graph pause would already
// have persisted (see the doc comment on
// TestClaimAndDispatchResume_GraphControllerKind_RoutesToResumeGraphTurn
// for why that matters here). Kept local to this file rather than adding a
// SessionService parameter to the existing helper: that helper has six
// pre-existing callers across graph_dispatch_test.go/turn_test.go that all
// want their own fresh session and none of this parking/pre-seeding
// concern.
func newTerminalCompletionRunnerWithSession(t *testing.T, progress graphTurnProgress, svc session.Service) *runner.Runner {
	t.Helper()

	seedNode := workflow.NewEmittingFunctionNode(
		"test_seed_progress_for_completion",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return nil, emitGraphTurnProgress(ctx, emit, progress)
		},
		workflow.NodeConfig{},
	)
	terminalNode := workflow.NewEmittingFunctionNode(
		"test_ready_to_finalize_terminal",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return runGraphTerminalNode(ctx, emit, nil, nil, nil)
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "resume_test_completion_chain",
		Edges: workflow.Chain(workflow.Start, seedNode, terminalNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

// newGraphResumeFixture 組一次 resume 走得完整條 graph 的最小裝備：seed 過的
// request／已配置好中斷 payload 的 store／有 session 的 runner。
//
// 2026-09-05 起 resume 只有 graph 一條控制流（ControllerKind 分支退場），所以
// 每一支要驗「child 真的被執行」的測試都需要這組 deps——先前那些測試靠
// fakeResumeStore 預設的零值 ControllerKind 落進 answer pipeline 分支，那條路
// 已經不存在了。純 T0 階段的測試（kind／schema／白名單擋下，零 mutation）不需要
// 這組，因為它們在 dispatch 之前就回來了。
// panickingRevisionStore 讓 graph resume 的第一個 I/O 就 panic——
// resumeGraphTurn 開頭那次 GetSnapshot。
type panickingRevisionStore struct{ RevisionStore }

func (panickingRevisionStore) GetSnapshot(context.Context, int64, identity.BuyerID) (repository.Request, repository.Revision, error) {
	panic("boom inside the graph resume pipeline")
}

func newGraphResumeFixture(t *testing.T, seed string) (*fakeResumeStore, *fakeRepo, GraphControllerDeps) {
	t.Helper()
	repo := newFakeRepo()
	reqID, revID := seedPendingDisambiguationRequest(t, repo, seed)
	store := newFakeResumeStore()
	store.resolveCtx = resumeCropInterruptCtx(reqID, revID)

	svc := session.InMemoryService()
	_, err := svc.Create(context.Background(), &session.CreateRequest{
		AppName: string(ProcurementAppName), UserID: "1234", SessionID: ProcurementSessionID(reqID),
		State: map[string]any{
			"request_id": reqID, "current_slots": domain.Slots{},
			"history": []repository.RevisionWithPatch(nil), "rfq_state": domain.RfqNone,
		},
	})
	require.NoError(t, err)
	full := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")}
	r := newTerminalCompletionRunnerWithSession(t, graphTurnProgress{NextSlots: full}, svc)
	return store, repo, GraphControllerDeps{RevisionStore: repo, Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RunStore: store}
}

// TestClaimAndDispatchResume_GraphControllerKind_StaleRevision_TerminalizesFailed
// is resumeGraphTurn's own version of
// TestClaimAndDispatchResume_StaleRevision_TerminalizesFailed_NoRollback
// (DURABLE, below): the graph path must enforce the identical
// ExpectedRevisionID staleness guard answer.go's composeAnswerViaGraph()
// enforces.
// GuardNoOpenInterrupt's own doc comment accepts a narrow
// CreateInterrupt/TerminalizeRun race specifically because it trusts the
// downstream resume path to reject a stale resume via ErrStaleRevision —
// that promise must hold for a GRAPH-claimed parent too, not just a
// DURABLE one, and it is directly reachable (not just theoretical):
// rewind.go has no GuardNoOpenInterrupt call today, so a rewind landing on
// the same request while this run is parked is a real way for
// ExpectedRevisionID to go stale.
//
// GraphControllerDeps.Runner is left nil on purpose: the staleness check
// must short-circuit before resumeGraphTurn ever calls runGraphOutcome, so
// a nil-Runner panic here would itself be proof this test (or the
// production code) is wrong. Also proves the child is correctly
// terminalized as FAILED via resumeGraphTurn's own `fail` helper — not
// left stuck QUEUED — and that no ledger event is appended.
func TestClaimAndDispatchResume_StaleRevision_TerminalizesFailed_NoRollback(t *testing.T) {
	store, _, deps := newGraphResumeFixture(t, "seed-resume-graph-stale")
	// ExpectedRevisionID 刻意錯誤——模擬買家在 park 期間，同一張單被 rewind 過
	// （rewind.go 今天沒有 GuardNoOpenInterrupt，這條路徑因此是真的可達，不只是
	// 理論上的 race）。
	store.resolveCtx = resumeCropInterruptCtx(store.resolveCtx.RequestID, store.resolveCtx.ExpectedRevisionID+999)
	// Runner 清成 nil：守門必須在 runGraphOutcome 之前短路，這裡若 panic 本身
	// 就是這支測試（或生產碼）錯了的證據。
	deps.Runner = nil
	deps.ResumeRunner = ResumeRunner{Runner: deps.Runner}

	out, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-graph-stale-1", ClientRequestID: "cr-resume-graph-stale-1",
		ResponsePayload: []byte(`{"optionId":"crop-CROP-X"}`),
	}, store, NewRunDispatcher(), deps)

	require.ErrorIs(t, err, ErrStaleRevision)
	require.Equal(t, int64(9500), out.RunID, "失敗仍要交出 run_id，呼叫端才查得到這個 run")
	require.Len(t, store.terminals, 1)
	require.Equal(t, domain.RunStatusFailed(), store.terminals[0].Status)
	require.Equal(t, codemeta.ProcurementRunFailureReasonTurnPipelineError.Value, store.terminals[0].FailureReason)
	require.Empty(t, store.events, "the staleness guard must short-circuit before runGraphOutcome ever appends any ledger event")
}

// TestClaimAndDispatchResume_OptionIDNotInCandidates_ZeroMutation 釘住
// 2026-09-05 的裁決（ADR crop-answer-whitelist-in-resume-transaction）：候選白
// 名單比對移進 T0 的 ResolveInput，所以一個不在候選裡的 optionId 在**任何
// mutation 之前**就被擋下——中斷維持 PENDING、沒有 child run、沒有 ledger 事件。
//
// 修正前這道比對只在 child pipeline 的 buildCropDisambiguationPatch 執行，而
// T0 早已 commit 並把中斷標記解決，於是「送錯一次 optionId」＝那張卡永久燒掉：
// 買家改送正確答案只會拿到 409 APPROVAL_ALREADY_RESOLVED。
func TestClaimAndDispatchResume_OptionIDNotInCandidates_ZeroMutation(t *testing.T) {
	store := newFakeResumeStore()
	store.resolveCtx = resumeCropInterruptCtx(4242, 77)

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-not-a-candidate-1", ClientRequestID: "cr-resume-not-a-candidate-1",
		// pendingCropFixture 唯一的候選是 CROP-X（optionId "crop-CROP-X"）。
		ResponsePayload: []byte(`{"optionId":"crop-CROP-NOT-OFFERED"}`),
	}, store, NewRunDispatcher(), GraphControllerDeps{})

	require.ErrorIs(t, err, ErrInvalidOptionID)
	require.Empty(t, store.events, "T0 必須在不可逆點之前 ROLLBACK，不得留下 ledger 事件")
	require.Empty(t, store.terminals)
	require.Empty(t, store.interrupts)
	require.Zero(t, store.lastResolved, "ResolveInput 不得回傳 resolved input——它是 T0 繼續往下寫的輸入")
}

// TestClaimAndDispatchResume_CropOtherOptionID_NotWhitelistChecked 是上一個測試
// 的邊界另一半：crop-other 逃生艙**不在**候選清單裡（它是伺服器固定提供的選項，
// 不是 pricebook 候選），新增的白名單比對不得把它一起擋掉——否則
// PendingReason=UNAVAILABLE、0 候選那個情境下買家就完全無路可走。
// 這裡斷言它通過了 T0（拿到 child run），而不是被 ErrInvalidOptionID 擋下。
func TestClaimAndDispatchResume_CropOtherOptionID_NotWhitelistChecked(t *testing.T) {
	store, _, deps := newGraphResumeFixture(t, "seed-resume-crop-other")

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-crop-other-1", ClientRequestID: "cr-resume-crop-other-1",
		ResponsePayload: []byte(`{"optionId":"crop-other","freeText":"野生黑木耳","selfEnteredPriceAmount":"150"}`),
	}, store, NewRunDispatcher(), deps)

	require.NoError(t, err)
	require.NotZero(t, store.lastResolved.InputRefJSON, "crop-other 必須通過 T0 的白名單檢查")
}

var errResumeModelUnavailable = errors.New("failed to call model: Error 503, Status: UNAVAILABLE")

func newModelFailingRunner(t *testing.T, svc session.Service, failWith error) *runner.Runner {
	t.Helper()

	failingNode := workflow.NewEmittingFunctionNode(
		"test_model_call_fails",
		func(_ agent.Context, _ any, _ func(*session.Event) error) (any, error) {
			return nil, failWith
		},
		workflow.NodeConfig{},
	)
	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "resume_test_model_failure_chain",
		Edges: workflow.Chain(workflow.Start, failingNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: svc, AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

func TestClaimAndDispatchResume_PipelineFails_RestoresInterruptSoBuyerCanAnswerAgain(t *testing.T) {
	store, _, deps := newGraphResumeFixture(t, "seed-resume-restore-recoverable")
	svc := session.InMemoryService()
	deps.Runner = newModelFailingRunner(t, svc, errResumeModelUnavailable)
	deps.ResumeRunner = ResumeRunner{Runner: deps.Runner}

	out, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-restore-1", ClientRequestID: "cr-restore-1",
		ResponsePayload: []byte(`{"optionId":"crop-CROP-X"}`),
	}, store, NewRunDispatcher(), deps)

	require.Error(t, err)
	require.Equal(t, int64(9500), out.RunID)
	require.Len(t, store.restoreCalls, 1)
	require.Equal(t, "int-restore-1", store.restoreCalls[0].InterruptID)
	require.Equal(t, identity.BuyerID(1234), store.restoreCalls[0].BuyerID)
	require.Equal(t, int64(9500), store.restoreCalls[0].ChildRunID)
}

func TestClaimAndDispatchResume_StaleRevision_DoesNotRestoreInterrupt(t *testing.T) {
	store, _, deps := newGraphResumeFixture(t, "seed-resume-restore-stale")
	store.resolveCtx = resumeCropInterruptCtx(store.resolveCtx.RequestID, store.resolveCtx.ExpectedRevisionID+999)
	deps.Runner = nil
	deps.ResumeRunner = ResumeRunner{Runner: deps.Runner}

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-restore-stale-1", ClientRequestID: "cr-restore-stale-1",
		ResponsePayload: []byte(`{"optionId":"crop-CROP-X"}`),
	}, store, NewRunDispatcher(), deps)

	require.ErrorIs(t, err, ErrStaleRevision)
	require.Empty(t, store.restoreCalls)
}

func TestClaimAndDispatchResume_Succeeds_DoesNotRestoreInterrupt(t *testing.T) {
	store, _, deps := newGraphResumeFixture(t, "seed-resume-restore-happy")

	_, err := ClaimAndDispatchResume(context.Background(), ResumeInput{
		BuyerID: 1234, InterruptID: "int-restore-happy-1", ClientRequestID: "cr-restore-happy-1",
		ResponsePayload: []byte(`{"optionId":"crop-CROP-X"}`),
	}, store, NewRunDispatcher(), deps)

	require.NoError(t, err)
	require.Empty(t, store.restoreCalls)
}
