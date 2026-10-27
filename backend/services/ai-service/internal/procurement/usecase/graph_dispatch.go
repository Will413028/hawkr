// services/ai-service/internal/procurement/usecase/graph_dispatch.go
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// ledgerEventTypeForActivity translates an ActivityEventPayload into a
// ledger event type + JSON payload, the same "input type -> (eventType,
// []byte)" shape turn.go's ledgerEventTypeFor uses for the non-graph
// dispatch path (turn.go:1101).
//
// 唯一的 Activity payload marshal 點，兩個 producer 共用：graph 路徑（本檔下方
// 的翻譯迴圈，同一份 bytes 也送進 activityCh 給 live 串流）與候選搜尋 run
// （candidate_search.go，脫離 graph 的背景 goroutine，直接寫 ledger）。兩邊各自
// marshal 遲早會分岔，而這個形狀同時被 POST 串流與 GET replay 端點消費
// （CLAUDE.md §15）。
//
// Always returns AGENT_ACTIVITY_SNAPSHOT —
// this plan only ever produces full-snapshot rows, never deltas (spec
// doesn't define a delta scenario for these three nodes; AGENT_ACTIVITY_DELTA
// stays unused by this change).
func ledgerEventTypeForActivity(payload ActivityEventPayload) (eventType string, publicPayload []byte) {
	row := map[string]any{
		"rowId":         strconv.FormatInt(payload.RowID, 10),
		"attempt":       payload.Attempt,
		"maxAttempts":   payload.MaxAttempts,
		"retryGroupKey": strconv.FormatInt(payload.RetryGroupKey, 10),
		"code":          payload.Code,
		"text":          payload.Text,
		"label":         payload.Label,
		"status":        payload.Status,
	}
	// parentRowId／count 只在有值時才進 payload——兩者都是「這一列有沒有這個
	// 概念」的問題，不是「值是多少」：top-level 列沒有父列，非 lane 列沒有計數。
	// 補 "0"／"" 會讓 consumer 分不出「沒有」與「是 0」。id 與計數依 API/Proto
	// 規範 §6 一律字串。
	if payload.ParentRowID != 0 {
		row["parentRowId"] = strconv.FormatInt(payload.ParentRowID, 10)
	}
	if payload.Count != nil {
		row["count"] = strconv.Itoa(*payload.Count)
	}
	publicPayload, _ = json.Marshal(row)
	return codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value, publicPayload
}

// ClaimAndDispatchTurnViaGraph 是 ClaimAndDispatchTurn（turn.go）的平行
// entry point，走 T8 的 ADK graph（NewProcurementGraph）而非既有 HandleStream。
// 不修改 ClaimAndDispatchTurn 本身一行。
//
// 2026-08-31（procurement graph cutover，Task 4）：SubmitTurn／
// SubmitTurnStream（internal/grpc/procurement_handler.go）現在無條件呼叫這個
// 函式——不再有平行的 flag-off／DurableRun 分支可以退回。ClaimAndDispatchTurn
// 與 TurnUseCase.Handle／HandleStream 本身仍留著（Task 6a 才會清掉），但已經
// 沒有任何 production 呼叫路徑會再走到它們。
//
// 2026-08-30（turn run lifecycle 共用層）：claim／replay／dispatch 脫鉤／
// panic recovery 四塊改用共用的 dispatchTurnViaExecutor（turn_run_lifecycle.go），
// 跟 ClaimAndDispatchTurn 共用同一份保證，不再各自實作一份——這正是本次改動
// 存在的理由：這個函式原本明確缺 dispatcher 脫鉤／panic recovery／冪等重播
// 三項（見本次改動前的舊版文件），現在跟 legacy 路徑共用同一套機制，不是各自
// 補一份。
//
// 範圍（刻意比 executeTurnPipeline 窄，不是它的完整對等物）：graphTurnExecutor
// 現在真的呼叫 store.CreateInterrupt／store.TerminalizeRun 收尾一個真實的 run，
// 並把 crop_disambiguation 掛起訊號翻譯成既有 ledger 的 CreateInterrupt 呼叫。
//
// 2026-08-30（sub-project 2）：三個 terminal node（graph.go）現在組出真正的
// TurnResult 內容（composeGraphTurnResult，graph_terminal.go），
// graphTurnExecutor 呼叫真正的 revStore.AppendRevision 持久化，並比照
// executeTurnPipeline 做 conversation_id 回填。
//
// 2026-08-31（sub-project 3a）：quota／price-preview／RFQ 三個橫切關注點
// 已補齊——graphTurnExecutor 自己做 quota reserve／confirm／release，
// composeGraphTurnResult 算真正的 price-preview nextAction 與 RFQ 新鮮度。
// 跟 executeTurnPipeline 仍有的差距只剩 crop-disambiguation 掛起時跳過
// AppendRevision（見下方該分支的註解，deferred to sub-project 3b）。
//
// runner／session.Service／graph 的建構由 DI 佈線層注入（internal/procurement
// 的 fx module，見 module.go 的 usecase.NewProcurementGraphRunner）。
//
// GuardNoOpenInterrupt 的跳過條件比照 ClaimAndDispatchTurn（turn.go）：
// in.RequestID != 0 且非重送才檢查。2026-08-30（sub-project 2）修正——原本
// 無條件呼叫的理由是「這個函式目前唯一支援的情境是 RequestID==0」，多輪續談
// 落地後這個前提不再成立，RequestID != 0 成為每個對話第二句話起的正常情境。
// GraphControllerDeps is the T8 graph-controller dependency bag — moved
// here from internal/grpc (Sub-project 3b, Task 5) because usecase, not
// the transport layer, is where both ClaimAndDispatchTurnViaGraph and the
// new resumeGraphTurn (Task 8) actually consume it; a transport package
// cannot be imported back into usecase, so the canonical definition has to
// live on this side. internal/grpc now holds a usecase.GraphControllerDeps
// field directly instead of maintaining its own parallel struct that got
// unpacked into positional args at every call site.
//
// cropidentity.Resolver is new relative to the pre-Task-5 shape: ClaimAndDispatchTurnViaGraph
// itself never reads it (crop resolution for a fresh dispatch is closed
// into NewProcurementGraph's crop_gate node at construction time, not
// threaded per-call) — it exists here solely for resumeGraphTurn (Task 8),
// which resolves a buyer's crop-disambiguation answer in Go-land, outside
// the graph, and needs to call cropidentity.Resolver.Persist the same way
// answer.go's Handle already does.
//
// Config *config.Config used to live here, only to satisfy a now-deleted
// PROCUREMENT_LLM_CONTROLLER_ENABLED check inside ClaimAndDispatchTurnViaGraph
// (removed by Task 4 of the T12 procurement-graph-cutover plan). Neither
// ClaimAndDispatchTurnViaGraph nor resumeGraphTurn ever reads deps.Config
// for anything else, so T12 Task 7 drops the field entirely (same
// precedent as DurableRunDeps.Config, removed by Task 6a) rather than
// leaving a config dependency with zero readers.
type GraphControllerDeps struct {
	RunStore      RunClaimStore
	RevisionStore RevisionStore
	Dispatcher    *RunDispatcher
	Runner        *runner.Runner
	// ResumeRunner 關掉 AutoCreateSession，只給 resumeGraphTurn 用：checkpoint
	// 被清掉之後必須回錯，不能靜默開一個空 session 當成新的一輪跑。
	ResumeRunner          ResumeRunner
	RfqConfirmationReader RfqConfirmationReader
	Entitlement           EntitlementClient
	Quota                 QuotaReserver
	CropResolver          cropidentity.Resolver
	// Narrator 讓消歧暫停那條分支（interruptID != "" 時 graphTurnExecutor 自己
	// compose 的那一輪）也能敘述 UNDERSTAND_INPUT／PICK_NEXT_QUESTION 兩碼。
	// 走 terminal node 的正常輪次是由 graph 構造時閉包進去的，不經過這裡。
	Narrator StepNarrator
	// Realizer（#2549 Phase 3, Task 8）跟 Narrator 同一個理由、同一個分工：
	// 走 terminal node 的正常輪次由 graph 構造時閉包進去（NewProcurementGraph
	// 的 realizer 參數），不經過這裡；這個欄位只服務消歧暫停那條分支
	// （runGraphOutcome 直接呼叫 composeGraphTurnResult 那一次）。Narrator 這個
	// 欄位曾經漏過 di.go 的組裝（見該欄位註解與
	// graph_controller_deps_wiring_test.go），這裡不重蹈覆轍。
	Realizer SurfaceRealizer
}

// TurnStreamHooks 是 per-request 的串流掛勾，由 SubmitTurnStream 每次請求各自
// 建立。⚠️ 絕不可放進 GraphControllerDeps——那是 fx 提供的 process-wide
// singleton，把 per-request channel 掛上去會造成跨買家內容外洩與 data race。
//
// 零值（三個欄位都是 nil）代表呼叫端不要 live 事件，行為與本機制加入前完全
// 相同——SubmitTurn（unary）傳零值。
type TurnStreamHooks struct {
	// OnClaimed 在 ClaimRun 成功後、dispatcher.Dispatch 之前呼叫，讓 handler
	// 能在任何 activity frame 之前先送出 RunAccepted（見 Global Constraints）。
	OnClaimed func(runID int64)
	// ActivityCh 承載 graph 執行期間產生的 Activity payload。producer 端一律
	// 非阻塞送出（滿了就丟），沒有人 close 它。
	ActivityCh chan []byte
	// OnActivity 在 dispatch 期間、與 OnClaimed 同一個 goroutine 上被呼叫，
	// 因此 handler 在其中送 gRPC frame 不需要 mutex。
	OnActivity func(payload []byte)
}

func ClaimAndDispatchTurnViaGraph(ctx context.Context, in TurnInput, deps GraphControllerDeps, hooks TurnStreamHooks) (SubmitTurnResult, error) {
	if in.RequestID != 0 {
		isRetry, err := deps.RunStore.HasIdempotencyKey(ctx, in.BuyerID, in.ClientRequestID)
		if err != nil {
			return SubmitTurnResult{}, err
		}
		if !isRetry {
			if err := GuardNoOpenInterrupt(ctx, deps.RunStore, in.RequestID, in.BuyerID); err != nil {
				return SubmitTurnResult{}, err
			}
		}
	}
	return dispatchTurnViaExecutor(ctx, in, deps.RunStore, deps.Dispatcher, domain.ControllerKindGraph,
		graphTurnExecutor(deps.Runner, deps.RevisionStore, deps.RfqConfirmationReader, deps.Entitlement, deps.Quota, hooks.ActivityCh, deps.Narrator, deps.Realizer),
		hooks)
}

// graphTurnExecutor adapts a graph *runner.Runner into a TurnExecutor —
// extracted from ClaimAndDispatchTurnViaGraph's previous inline body
// (pre-2026-08-30), now sharing dispatchTurnViaExecutor's claim/replay/
// dispatch/panic-recovery guarantees with the legacy executor instead of
// ClaimAndDispatchTurnViaGraph reimplementing them inline. The r.Run()
// iteration, Activity-event translation, and crop-disambiguation interrupt
// handling below are unchanged from the pre-refactor body — only the
// surrounding claim/dispatch machinery moved out.
func graphTurnExecutor(r *runner.Runner, revStore RevisionStore, rfqConfirmationReader RfqConfirmationReader, entitlementClient EntitlementClient, quotaReserver QuotaReserver, activityCh chan []byte, narrator StepNarrator, realizer SurfaceRealizer) TurnExecutor {
	return func(ctx context.Context, run repository.Run, in TurnInput, runStore RunClaimStore) (turnPipelineOutcome, error) {
		// Quota reservation — verbatim port of HandleStream's own ordering
		// and fail-closed semantics (turn.go:347-390): reserved before
		// resolveGraphTurnState's CreateRequest, so a blocked attempt
		// leaves no orphan procurement_request row; released via defer
		// unless confirmed by a successful AppendRevision below.
		var (
			entitlement     Entitlement
			reservationID   int64
			confirmedBefore int
			quotaConfirmed  bool
		)
		quotaEnabled := entitlementClient != nil && quotaReserver != nil
		if quotaEnabled {
			var err error
			entitlement, err = entitlementClient.GetBuyerEntitlement(ctx, in.BuyerID)
			if err != nil {
				// terminalizeFailedRun 收尾（2026-08-31 final review finding
				// 1）：這三個 fail-closed early return 都發生在 ClaimRun 之後，
				// 若不主動 terminalize，這個 run 會卡在 QUEUED 永遠收斂不了
				// ——TURN run 沒有 lazy reaper（turn_run_lifecycle.go 同一句
				// 註解），跟 dispatchTurnViaExecutor 對 panic 的既有處理同一個
				// helper、同一個理由。
				terminalizeFailedRun(ctx, run, in.BuyerID, runStore)
				return turnPipelineOutcome{}, fmt.Errorf("resolve entitlement: %w", err)
			}
			var reserved bool
			reservationID, reserved, confirmedBefore, err = quotaReserver.ReserveQuota(ctx, in.BuyerID, entitlement.PeriodStart, entitlement.MonthlyAnswerLimit)
			if err != nil {
				terminalizeFailedRun(ctx, run, in.BuyerID, runStore)
				return turnPipelineOutcome{}, fmt.Errorf("reserve quota: %w", err)
			}
			if !reserved {
				terminalizeFailedRun(ctx, run, in.BuyerID, runStore)
				return turnPipelineOutcome{}, &QuotaExceededError{Limit: entitlement.MonthlyAnswerLimit, ResetAt: entitlement.PeriodEnd}
			}
			defer func() {
				if !quotaConfirmed {
					quotaReserver.ReleaseQuota(ctx, reservationID, in.BuyerID)
				}
			}()
		}

		requestID, state, err := resolveGraphTurnState(ctx, revStore, rfqConfirmationReader, in)
		if err != nil {
			// terminalizeFailedRun 收尾（2026-08-31 final review finding 1，
			// 同上面三個 quota early return 一樣的理由）：這個 early return
			// 也發生在 ClaimRun 之後，不主動 terminalize 這個 run 會卡在
			// QUEUED 永遠收斂不了，讓同一個 clientRequestId 的任何重試永遠
			// 拿到 ErrRunResultUnavailable（replayTurnRun，turn_run_lifecycle.go）。
			terminalizeFailedRun(ctx, run, in.BuyerID, runStore)
			return turnPipelineOutcome{}, err
		}
		sessionID := ProcurementSessionID(requestID)
		userID := strconv.FormatInt(int64(in.BuyerID), 10)
		msg := &genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: in.Message}}}

		outcome, confirmed, err := runGraphOutcome(ctx, r, revStore, runStore, run, in, requestID, state,
			userID, sessionID, msg, quotaEnabled, reservationID, entitlement, confirmedBefore,
			activityCh, narrator, realizer,
			runner.WithStateDelta(state))
		quotaConfirmed = confirmed
		return outcome, err
	}
}

// runGraphOutcome drives r.Run() to its next pause or completion and
// persists whatever it produces — extracted out of graphTurnExecutor
// (Sub-project 3b, Task 7, spec §3 Approach A / §4.2) so resumeGraphTurn
// (Task 8) can share the exact same event-translation, crop-content-
// composition, AppendRevision, and TerminalizeRun logic instead of
// duplicating it — the two callers differ only in what genai.Content
// message and sessionID/userID they build beforehand, and whether a quota
// reservation is in play (resumeGraphTurn never reserves: it always passes
// quotaEnabled=false, reservationID=0, entitlement=Entitlement{}, the same
// nil-safe shape TestClaimAndDispatchTurnViaGraph_NilEntitlementOrQuota_SkipsQuotaEntirely
// already pins for a fresh dispatch with no quota deps wired).
//
// Both new turns and resumes supply the current immutable revision and history.
// A category answer can park again, so pause composition must use that refreshed
// baseline rather than the original invocation's state or an empty Slots value.
//
// Returns quotaConfirmed so the caller's own release-defer (if it has one)
// knows whether to skip releasing.
func runGraphOutcome(
	ctx context.Context, r *runner.Runner, revStore RevisionStore, runStore RunClaimStore,
	run repository.Run, in TurnInput, requestID int64, state map[string]any,
	userID, sessionID string, msg *genai.Content,
	quotaEnabled bool, reservationID int64, entitlement Entitlement, confirmedBefore int,
	activityCh chan []byte,
	narrator StepNarrator,
	realizer SurfaceRealizer,
	runOpts ...runner.RunOption,
) (turnPipelineOutcome, bool, error) {
	var composed *graphTurnResultPayload
	var progress graphTurnProgress
	var interruptID, interruptKind string
	var runErr error

	// 取消觀察器掛在事件邊界上：ADK 的 event 迴圈是這條路徑唯一會週期性回到
	// 我們手上的地方，所以也是唯一能在不改 ADK 的前提下中止執行的切點。
	// 中止只發生在兩個事件之間——node 內部的狀態不會被切一半。
	cancelObserver := newRunCancelObserver(runStore, run.ID, in.BuyerID)
	cancelled := false

	for ev, evErr := range r.Run(ctx, userID, sessionID, msg, agent.RunConfig{}, runOpts...) {
		if evErr != nil {
			runErr = evErr
			break
		}
		// ⚠️ 檢查排在 evErr 之後、處理這個事件之前：已經發生的錯誤是事實，要照
		// 實回報成 FAILED，不能被一個後到的取消蓋成「使用者取消的」。
		if cancelObserver.cancelRequested(ctx) {
			cancelled = true
			break
		}
		if ev != nil && ev.Actions.StateDelta != nil {
			// graph_turn_progress accumulates across extract_slots/
			// validate_slots/crop_gate (graph_terminal.go's
			// emitGraphTurnProgress) — captured here so a crop-
			// disambiguation pause (which happens BEFORE any terminal
			// node runs composeGraphTurnResult) can still compose real
			// content below using the same progress a normal
			// completion would have used (Sub-project 3b, Task 6).
			if raw, ok := ev.Actions.StateDelta["graph_turn_progress"]; ok {
				if p, ok := decodeGraphTurnProgress(raw); ok {
					progress = p
				}
			}
		}
		if ev != nil && ev.Content != nil {
			for _, p := range ev.Content.Parts {
				fc := p.FunctionCall
				if fc == nil {
					continue
				}
				if fc.Name == emitTurnResultFuncName {
					if payload, ok := fc.Args["payload"].(graphTurnResultPayload); ok {
						composed = &payload
					}
					continue
				}
				if fc.Name != emitActivityEventFuncName {
					continue
				}
				payload, ok := fc.Args["payload"].(ActivityEventPayload)
				if !ok {
					continue
				}
				eventType, publicPayload := ledgerEventTypeForActivity(payload)
				if _, appendErr := runStore.AppendRunEvent(ctx, repository.AppendRunEventInput{
					RunID: run.ID, BuyerID: in.BuyerID, EventType: eventType,
					SchemaVersion: "v1", PublicPayloadJSON: publicPayload,
				}); appendErr != nil {
					pkglogger.Error(ctx, "procurement: failed to append activity run event, continuing graph dispatch",
						zap.Error(appendErr), zap.Int64("run_id", run.ID))
				}
				// 非阻塞送一份給 live 串流消費端。刻意放在 AppendRunEvent 之後、
				// 且不受它成敗影響——ledger 才是完整紀錄，ledger 寫失敗時 live
				// 顯示仍應盡力送達。
				//
				// default 是防止病態消費端拖住整個 graph 執行的最後保險：正常
				// 一輪最多 6 個事件（twoPhaseActivity 的三個 production 呼叫點
				// ——extract_slots／validate_slots／crop_gate 各一組 entry ＋
				// completion），遠低於 handler 端 channel 的容量（SubmitTurnStream
				// 每個請求各建一條，容量 64），實務上不會走到。
				//
				// 外層的 nil 判斷是為了讓「這兩個呼叫端刻意不要 live 事件」
				// （SubmitTurn unary、resumeGraphTurn）這件事在這裡顯式可讀，
				// 不是正確性所需：nil channel 的送出放進 select 也只會落到
				// default，不會像裸送那樣永久阻塞。
				if activityCh != nil {
					select {
					case activityCh <- publicPayload:
					default:
					}
				}
			}
		}
		if ev == nil || len(ev.LongRunningToolIDs) == 0 || ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			fc := p.FunctionCall
			if fc == nil {
				continue
			}
			raw, ok := fc.Args["payload"]
			if !ok {
				continue
			}
			payload, ok := raw.(CropDisambiguationInterruptPayload)
			if !ok || payload.Kind != cropDisambiguationInterruptKind {
				continue
			}
			// 候選清單與待處理原因跟中斷本體同一次 INSERT。刀 2 之前它們是
			// crop_gate node 自己的一次 UPDATE（procurement_requests 上的兩個
			// 欄位），跟這裡的 CreateInterrupt 各自 log-and-swallow——於是有
			// 「候選落庫但中斷沒建成」與「中斷建成但候選沒落庫」兩個不對稱的
			// 中間狀態。合成一次 INSERT 之後兩者結構上不存在：這一行失敗就是
			// 兩者都沒有，run 照既有降級走完成路徑，買家重打品項即可。
			payloadJSON, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				pkglogger.Error(ctx, "procurement: marshalling crop disambiguation interrupt payload failed, falling back to normal completion",
					zap.Error(marshalErr), zap.Int64("run_id", run.ID))
				continue
			}
			interrupt, createErr := runStore.CreateInterrupt(ctx, repository.CreateInterruptInput{
				RunID: run.ID, InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1",
				PayloadJSON: payloadJSON, FactsAsOf: payload.FactsAsOf,
			})
			if createErr != nil {
				pkglogger.Error(ctx, "procurement: failed to create interrupt for crop disambiguation via graph, falling back to normal completion",
					zap.Error(createErr), zap.Int64("run_id", run.ID))
				continue
			}
			interruptID, interruptKind = interrupt.ID, interrupt.InterruptKind
		}
	}

	terminalStatus := domain.RunStatusSucceeded()
	terminalEventType := codemeta.ProcurementLedgerEventTypeRunFinished.Value
	failureReason := ""
	terminalPayload := []byte(`{}`)
	if interruptID != "" {
		terminalStatus = domain.RunStatusWaiting()
		terminalEventType = codemeta.ProcurementLedgerEventTypeConfirmationRequired.Value
		terminalPayload, _ = json.Marshal(map[string]string{"interruptId": interruptID, "interruptKind": interruptKind})
	}

	var final TurnResult
	var quotaConfirmed bool
	if !cancelled && runErr == nil && interruptID != "" {
		// Crop-disambiguation pause: reuse composeGraphTurnResult
		// exactly as a normal completion would (Sub-project 3b, Task 6)
		// — it already handles progress.CropPending by substituting
		// domain.BuildCropDisambiguationQuestion for the ordinary next
		// question (graph_terminal.go), so this produces the same
		// fidelity content HandleStream's own disambiguation-pause
		// branch does, with zero new question-composition logic.
		// pricePreviewChecker is passed nil here (unlike a terminal
		// node's own call, which has it closed in from graph
		// construction) — graphTurnExecutor itself has no access to
		// it (spec §4.4's disclosed, minor divergence: NextAction's
		// "REVIEW_PRICE_PREVIEW" hint is empty during a disambiguation
		// pause; ActCode is ActItemNeedsDisambiguation regardless, so
		// no user-facing act text is affected, only the standalone
		// NextAction hint field). realizer (#2549 Phase 3, Task 8) is NOT
		// in the same boat — unlike pricePreviewChecker it is threaded all
		// the way down to this function's own parameter (via
		// GraphControllerDeps.Realizer), so this disambiguation-pause
		// branch gets model narration too, not just a terminal node's own
		// call.
		current, _ := state["current_slots"].(domain.Slots)
		history, _ := state["history"].([]repository.RevisionWithPatch)
		rfqState, _ := state["rfq_state"].(domain.RfqConfirmationState)
		nextSlots, status, payload, nextAction := composeGraphTurnResult(ctx, requestID, current, history, progress, in.Message, rfqState, nil, narrator, realizer)
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			runErr = err
		} else if rev, err := revStore.AppendRevision(ctx, repository.AppendInput{
			RequestID: requestID, BuyerID: in.BuyerID, Slots: nextSlots,
			Source: "LLM_EXTRACT", ClientRequestID: in.ClientRequestID, PayloadJSON: payloadJSON,
			RequestStatus: status, QuotaReservationID: reservationID, QuotaPeriodStart: entitlement.PeriodStart,
		}); err != nil {
			runErr = err
		} else {
			quotaConfirmed = true

			var quotaSnapshot QuotaSnapshot
			if quotaEnabled {
				quotaSnapshot = QuotaSnapshot{
					PlanCode: entitlement.PlanCode, Limit: entitlement.MonthlyAnswerLimit,
					Remaining: entitlement.MonthlyAnswerLimit - confirmedBefore - 1,
					ResetAt:   entitlement.PeriodEnd,
				}
			}

			if run.ConversationID == 0 && rev.ConversationID != 0 {
				if err := runStore.SetRunConversationID(ctx, run.ID, rev.ConversationID); err != nil {
					pkglogger.Error(ctx, "procurement: failed to backfill run conversation_id",
						zap.Error(err), zap.Int64("run_id", run.ID), zap.Int64("conversation_id", rev.ConversationID))
				}
			}
			final = TurnResult{
				RequestID: requestID, ConversationID: rev.ConversationID, RevisionID: rev.ID,
				Slots: nextSlots, Rejected: payload.Rejected,
				MissingRequired: nextSlots.MissingRequired(), RecordedThisTurn: payload.RecordedThisTurn,
				Degraded: payload.Degraded, DegradedReason: reasonFor(payload.Degraded),
				Reply: payload.Reply, Blocks: payload.Blocks, RenderMode: payload.RenderMode,
				Steps: payload.Steps, Question: payload.Question,
				NextAction: nextAction, Quota: quotaSnapshot,
				DialoguePlan: *payload.DialoguePlan,
			}
		}
	} else if !cancelled && runErr == nil && interruptID == "" {
		if composed == nil {
			// Defensive backstop, not an expected path (spec §4.7):
			// the graph's edges are exhaustive (graph.go's own
			// routing-completeness argument) — every non-interrupt,
			// non-error run must reach exactly one terminal node,
			// which always emits this marker. If this ever triggers,
			// treat it identically to a run error rather than
			// silently persisting nothing.
			runErr = errors.New("procurement: graph run completed without emitting a turn result")
		} else {
			payloadJSON, err := json.Marshal(composed.Payload)
			if err != nil {
				runErr = err
			} else if rev, err := revStore.AppendRevision(ctx, repository.AppendInput{
				RequestID: requestID, BuyerID: in.BuyerID, Slots: composed.Slots,
				Source: "LLM_EXTRACT", ClientRequestID: in.ClientRequestID, PayloadJSON: payloadJSON,
				RequestStatus: composed.Status, QuotaReservationID: reservationID, QuotaPeriodStart: entitlement.PeriodStart,
			}); err != nil {
				runErr = err
			} else {
				// AppendRevision 成功＝預約已經在它自己的交易內被轉成 durable
				// 用量（repository revision.go confirmQuotaInTx）——同
				// HandleStream turn.go:671-675 的既有理由，上面的 defer 因此
				// 不得再釋放。quotaEnabled==false 時 reservationID 是 0，
				// AppendRevision 那邊會跳過確認，這個旗標對它沒有意義但設了
				// 也無害。
				quotaConfirmed = true

				var quotaSnapshot QuotaSnapshot
				if quotaEnabled {
					quotaSnapshot = QuotaSnapshot{
						PlanCode: entitlement.PlanCode, Limit: entitlement.MonthlyAnswerLimit,
						Remaining: entitlement.MonthlyAnswerLimit - confirmedBefore - 1,
						ResetAt:   entitlement.PeriodEnd,
					}
				}

				if run.ConversationID == 0 && rev.ConversationID != 0 {
					if err := runStore.SetRunConversationID(ctx, run.ID, rev.ConversationID); err != nil {
						pkglogger.Error(ctx, "procurement: failed to backfill run conversation_id",
							zap.Error(err), zap.Int64("run_id", run.ID), zap.Int64("conversation_id", rev.ConversationID))
					}
				}
				final = TurnResult{
					RequestID: requestID, ConversationID: rev.ConversationID, RevisionID: rev.ID,
					Slots: composed.Slots, Rejected: composed.Payload.Rejected,
					MissingRequired: composed.Slots.MissingRequired(), RecordedThisTurn: composed.Payload.RecordedThisTurn,
					Degraded: composed.Payload.Degraded, DegradedReason: reasonFor(composed.Payload.Degraded),
					Reply: composed.Payload.Reply, Blocks: composed.Payload.Blocks, RenderMode: composed.Payload.RenderMode,
					Steps: composed.Payload.Steps, Question: composed.Payload.Question,
					NextAction: composed.NextAction, Quota: quotaSnapshot,
					// DialoguePlan (json:"-", not wire-visible directly) is
					// what toTurnResultProto (procurement_handler.go) reads
					// ActCode/PlanHash/PolicyVersion/RenderMode/TemplateVersion
					// off of — composed.Payload.DialoguePlan is a *domain.
					// DialoguePlanV1 (composeGraphTurnResult, Task 4/Sub-project
					// 2), always non-nil on a successful compose; leaving
					// TurnResult's own DialoguePlan at its zero value here
					// would make the wire response's dialogue-plan summary
					// hollow even though a real plan was computed.
					DialoguePlan: *composed.Payload.DialoguePlan,
				}
			}
		}
	}
	if runErr != nil {
		terminalStatus = domain.RunStatusFailed()
		terminalEventType = codemeta.ProcurementLedgerEventTypeRunFailed.Value
		failureReason = codemeta.ProcurementRunFailureReasonTurnPipelineError.Value
		terminalPayload = domain.RunFailedPayload(failureReason)
		// 這是 runErr 最後一次還帶著原因的地方。往下 failure_reason 只剩粗粒度的
		// TURN_PIPELINE_ERROR（CodeMeta 明文規定不得展開，否則 procurement_run_total
		// 的 label cardinality 會爆），terminalPayload 是公開欄位不能放內部錯誤，
		// 而 gRPC 層的 procurementUsecaseErrorStatus 會把未映射的錯誤重建成
		// codes.Internal「internal error」——原始訊息就此消失。CodeMeta 那條規定
		// 把細節指向 log／trace，這一行就是那個 log。
		pkglogger.Error(ctx, "procurement: graph turn pipeline failed",
			zap.Error(runErr), zap.Int64("run_id", run.ID),
			zap.Int64("buyer_id", int64(in.BuyerID)), zap.Int64("request_id", in.RequestID))
	} else if cancelled {
		// 取消走與成功／失敗**同一條** TerminalizeRun，只是終態不同——它是
		// doc57 §5.4 的終態之一，不是一種失敗：failureReason 留空、payload 不帶
		// reason、日誌用 Info。寫成 FAILED 會讓買家自己按的停止在
		// ERROR_RECOVERY 那條線上長出一個「可重試的錯誤」，那是假的。
		//
		// 🔴 上面兩個寫 revision 的分支都被 !cancelled 擋掉了：取消的這一輪
		// **沒有結果**，所以不寫 revision、不 confirm 配額（quotaConfirmed 維持
		// false）、final 留零值。寫了等於讓一個買家明說不要的結果變成需求的最新
		// 狀態。
		terminalStatus = domain.RunStatusCancelled()
		terminalEventType = codemeta.ProcurementLedgerEventTypeRunCancelled.Value
		pkglogger.Info(ctx, "procurement: graph turn cancelled by buyer request",
			zap.Int64("run_id", run.ID), zap.Int64("buyer_id", int64(in.BuyerID)),
			zap.Int64("request_id", in.RequestID))
	}

	wrote, terminalizeErr := runStore.TerminalizeRun(ctx, repository.TerminalizeRunInput{
		RunID: run.ID, BuyerID: in.BuyerID, Version: run.Version, Status: terminalStatus,
		FailureReason: failureReason, TerminalEventType: terminalEventType,
		// ResultRevisionID 是重播的錨點（turn.go:923-932 legacy 路徑同一個
		// 理由）：replayTurnRun（turn_run_lifecycle.go）靠 run.ResultRevisionID
		// != 0 判斷「這個 run 真的留下了可重播的結果」。runErr／interrupt 路徑
		// final 維持零值，這裡傳 0 進去，跟 legacy 路徑失敗時的行為一致。
		ResultRevisionID:  final.RevisionID,
		PublicPayloadJSON: terminalPayload,
	})
	switch {
	case terminalizeErr != nil:
		pkglogger.Error(ctx, "procurement: failed to terminalize graph run", zap.Error(terminalizeErr), zap.Int64("run_id", run.ID))
	case !wrote:
		pkglogger.Warn(ctx, "procurement: terminalize graph run CAS lost, this turn's terminal state was discarded",
			zap.Int64("run_id", run.ID), zap.Int64("expected_version", run.Version),
			zap.String("attempted_status", terminalStatus), zap.String("attempted_event_type", terminalEventType))
	}

	return turnPipelineOutcome{
		result: final, events: []TurnEvent{{Kind: TurnEventResult, Result: final}},
		interruptID: interruptID, interruptKind: interruptKind,
	}, quotaConfirmed, runErr
}

// buildResumedCropGateOutput 把買家的消歧答題轉成 resume 要送進圖裡的
// CropGateOutput——resume 由 interrupt node 重新進入，完成後轉交
// 下游（crop_resume_reconcile）的 typed 輸入。
//
// 抽成獨立函式而不是內嵌在 resumeGraphTurn 裡：那支函式要一個 Runner、一個
// RevisionStore、一個 RunStore 才跑得起來，而它的 fake runner 天生忽略傳進去的
// 訊息——也就是說內嵌的話，這幾行「要轉交哪些欄位」的決定不會被任何測試守到。
// 實測過：把 Resolution 從這裡拿掉，整個 procurement 套件仍然全綠。
//
// Item：真候選不改寫（買家挑的是「我剛說的是哪一種」，原話留著），逃生艙才改寫成
// 買家自己的描述。Resolution：兩條路徑都要轉交，否則買家挑完候選，DB 有解析結果、
// 畫面上卻什麼都沒有——這條線一開始要修的就是這件事。
func buildResumedCropGateOutput(current domain.Slots, patch domain.SlotPatch) CropGateOutput {
	item := current.Item
	if patch.Item != nil {
		item = patch.Item
	}
	return CropGateOutput{Item: item, Resolution: patch.ItemResolution, Pending: false}
}

// resumeGraphTurn resumes a graph-parked run given the buyer's crop-
// disambiguation answer — the graph-path counterpart of
// executeAnswerPipeline, called from ClaimAndDispatchResume (run_dispatch.go)
// when the parent run's ControllerKind is GRAPH (Sub-project 3b, spec §4.3).
//
// Manual prices use buildCropDisambiguationPatch; taxonomy answers use
// ResolveSelection and may pause for another question.
// Selected candidates preserve the original item text; crop-other accepts the
// buyer's replacement description.
//
// ADK requires both WorkflowInputFunctionCallName and the saved workflow pause
// ID. The public ledger interrupt ID is a separate identifier. Response payload
// is decoded by the re-entry node from either typed memory or JSON session data.
func resumeGraphTurn(ctx context.Context, deps GraphControllerDeps, child repository.Run, in AnswerInput) (turnPipelineOutcome, error) {
	// fail terminalizes child as FAILED before returning err — every
	// early-return below (before runGraphOutcome takes over terminalization
	// itself) must go through this, otherwise child is left stuck QUEUED
	// forever with no v1 sweeper to reclaim it (same reasoning as
	// terminalizeFailedRun's other call sites in this file/package: a
	// dispatched run has exactly one owner goroutine, and nobody else will
	// ever call TerminalizeRun for it if that owner returns without doing
	// so).
	fail := func(err error) (turnPipelineOutcome, error) {
		terminalizeFailedRun(ctx, child, in.BuyerID, deps.RunStore)
		return turnPipelineOutcome{}, err
	}

	_, activeRev, err := deps.RevisionStore.GetSnapshot(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return fail(err)
	}
	// Staleness guard — mirrors answer.go's Handle() exactly (same
	// GetSnapshot call, same comparison, same sentinel), and for the same
	// reason: ExpectedRevisionID is what repository.ResumeRun captured from
	// the parent's ResultRevisionID at park time, which can legitimately
	// diverge from the *current* active revision by the time this
	// background dispatch goroutine actually runs (e.g. a rewind on the
	// same request landed in between — rewind.go has no
	// GuardNoOpenInterrupt call today, so this is directly reachable, not
	// hypothetical). GuardNoOpenInterrupt's own doc comment accepts a
	// narrow CreateInterrupt/TerminalizeRun race specifically because its
	// documented worst case is "the resume path safely rejects via
	// ErrStaleRevision" — that promise has to hold on this path too, not
	// just answer.go's.
	if activeRev.ID != in.ExpectedRevisionID {
		return fail(ErrStaleRevision)
	}
	// Each stage resumes from the current immutable revision. Refreshing these
	// values also gives the pause-composition path a real baseline when a child
	// itself parks for the next category question.
	stateRequestID, state, err := resolveGraphTurnState(ctx, deps.RevisionStore, deps.RfqConfirmationReader, TurnInput{
		BuyerID: in.BuyerID, RequestID: in.RequestID, ClientRequestID: in.ClientRequestID,
	})
	if err != nil {
		return fail(err)
	}
	current := state["current_slots"].(domain.Slots)
	resumed, err := resolveResumedCropGate(ctx, deps.CropResolver, current, in)
	if err != nil {
		return fail(err)
	}
	if in.PendingCrop.Selection != nil {
		state["graph_turn_progress"] = graphTurnProgress{
			NextSlots: current, CropPending: true,
			CropCandidates: in.PendingCrop.Candidates, CropSelection: in.PendingCrop.Selection,
		}
	}
	workflowInterruptID := in.PendingCrop.WorkflowInterruptID
	if workflowInterruptID == "" {
		workflowInterruptID = cropDisambiguationInterruptKind
	}

	sessionID := ProcurementSessionID(in.RequestID)
	userID := strconv.FormatInt(int64(in.BuyerID), 10)
	msg := &genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{
		FunctionResponse: &genai.FunctionResponse{
			ID: workflowInterruptID, Name: workflow.WorkflowInputFunctionCallName,
			Response: map[string]any{"payload": resumed},
		},
	}}}

	outcome, _, runErr := runGraphOutcome(ctx, deps.ResumeRunner.Runner, deps.RevisionStore, deps.RunStore, child, TurnInput{
		BuyerID: in.BuyerID, RequestID: in.RequestID, ClientRequestID: in.ClientRequestID,
	}, stateRequestID, state, userID, sessionID, msg, false, 0, Entitlement{}, 0, nil, deps.Narrator, deps.Realizer, runner.WithStateDelta(state))
	// resumeGraphTurn never reserves quota (resuming an already-parked
	// interrupt is not "a new turn" — answer.go/rewind.go's own
	// executeAnswerPipeline/executeRewindPipeline never touch quota
	// either), so unlike graphTurnExecutor there is no release-defer here
	// at all for runGraphOutcome's quotaConfirmed return value to gate —
	// nothing consumes it either way, so it is discarded. (Note:
	// quotaConfirmed is NOT always false here — runGraphOutcome sets it
	// true unconditionally on any successful AppendRevision regardless of
	// quotaEnabled; the reason it's safe to discard is the absent
	// release-defer, not the value itself.)

	return outcome, runErr
}

// resolveGraphTurnState is the I/O-bearing per-turn-constant resolution
// step graphTurnExecutor runs once, before r.Run() — replaces stateFor
// (pure, always-synthetic-new-conversation, removed 2026-08-30, Sub-project
// 2). Mirrors HandleStream's own opening sequence (turn.go's
// CreateRequest-if-new then unconditional GetSlots/GetSnapshot/
// ListBranchRevisions) rather than re-implementing it: a freshly created
// request's reads already return the correct empty-conversation values, so
// no in.RequestID==0 special case is needed past step 1.
//
// 2026-08-31 (Sub-project 3a): also resolves rfqState (RFQ confirmation
// freshness, turn.go:577-589's exact derivation), seeded into the returned
// state map under "rfq_state" — unlike current_slots/
// request_id/history, this is not read by any graph node's typed input; it
// exists in state purely so composeGraphTurnResult (called from inside a
// terminal node, which cannot receive per-call Go arguments the way
// graphTurnExecutor's own callers can) can read it back via stateValue.
//
// The returned requestID is the *resolved* value (never 0) — callers must
// use it, not in.RequestID, wherever a stable per-conversation identifier
// is needed (see graphTurnExecutor's ProcurementSessionID call: using
// in.RequestID directly there was a latent bug, every brand-new
// conversation collided on the same ADK session).
func resolveGraphTurnState(ctx context.Context, store RevisionStore, rfqConfirmationReader RfqConfirmationReader, in TurnInput) (int64, map[string]any, error) {
	requestID := in.RequestID
	if requestID == 0 {
		req, _, err := store.CreateRequest(ctx, in.BuyerID)
		if err != nil {
			return 0, nil, err
		}
		requestID = req.ID
	}
	current, err := store.GetSlots(ctx, requestID, in.BuyerID)
	if err != nil {
		return 0, nil, err
	}
	// 刀 2 之前這裡還會多查一次 GetSnapshot 塞進 state 的 "request_snapshot"，
	// 供 crop_gate node 讀 request 上的待答狀態。那份狀態搬進中斷之後沒有任何
	// node 再讀它，而這次查詢本身也是重複的——GetSlots 內部就是
	// GetSnapshot(requestID, buyerID)（revision.go），連擁有權檢查都已經做過了。
	history, err := store.ListBranchRevisions(ctx, requestID, in.BuyerID)
	if err != nil {
		return 0, nil, err
	}

	// RFQ confirmation freshness — see resolveRfqState's doc comment
	// (Sub-project 3b, Task 4 dedup) — shared with HandleStream's own
	// derivation, no longer a separate duplicate copy.
	rfqState := resolveRfqState(ctx, rfqConfirmationReader, requestID, in.BuyerID, history)

	return requestID, map[string]any{
		"current_slots": current,
		"request_id":    requestID,
		"history":       history,
		"rfq_state":     rfqState,
		// .UTC()：application／event 時間一律 UTC（backend-payuni CLAUDE.md
		// §4／§21），比照原 stateFor 既有慣例。
		"now": time.Now().UTC(),
	}, nil
}
