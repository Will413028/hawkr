// services/ai-service/internal/procurement/usecase/turn_run_lifecycle.go
package usecase

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
)

// replayTurnRun rebuilds a completed run's result for a repeated
// (BuyerID, ClientRequestID) claim. History: generalized from
// TurnUseCase.replayExistingRun, a method on the legacy TurnUseCase/
// ClaimAndDispatchTurn pipeline — both later deleted (2026-08-30
// turn-run-lifecycle-shared-executor plan, Task 3) once this free function
// and dispatchTurnViaExecutor (below) took over. Works for any
// RunClaimStore-backed executor's run, not just that deleted legacy
// pipeline's — a run.ResultRevisionID of 0 or a missing answer_patch both
// mean honestly "nothing to replay yet", regardless of which executor
// produced (or has yet to produce) that run's result.
//
// Known gap carried forward unchanged, not fixed by this task: does not
// return InterruptID/InterruptKind (same gap the deleted
// TurnUseCase.replayExistingRun already had).
func replayTurnRun(ctx context.Context, run repository.Run, in TurnInput, runStore RunClaimStore) (TurnResult, error) {
	if run.ResultRevisionID == 0 {
		return TurnResult{}, ErrRunResultUnavailable
	}
	prev, err := runStore.FindByIdempotencyKey(ctx, run.BuyerID, in.ClientRequestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return TurnResult{}, ErrRunResultUnavailable
		}
		return TurnResult{}, err
	}
	if in.RequestID != 0 && in.RequestID != prev.RequestID {
		return TurnResult{}, domain.ErrIdempotencyKeyConflict
	}
	return replayResult(prev)
}

// terminalizeFailedRun is the free-function form of
// TurnUseCase.terminalizeFailedRun (turn.go:1058-1079, deleted in Task 3
// once its only caller moves here) — moved verbatim except for dropping
// the *TurnUseCase receiver, which it never used. Called after a
// dispatched executor panics, so the run doesn't sit forever in a
// non-terminal state (Track 1 has no lazy reaper for TURN runs).
func terminalizeFailedRun(ctx context.Context, run repository.Run, buyerID identity.BuyerID, runStore RunClaimStore) {
	wrote, err := runStore.TerminalizeRun(ctx, repository.TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version, Status: domain.RunStatusFailed(),
		FailureReason:     codemeta.ProcurementRunFailureReasonTurnPipelineError.Value,
		TerminalEventType: codemeta.ProcurementLedgerEventTypeRunFailed.Value,
		PublicPayloadJSON: domain.RunFailedPayload(codemeta.ProcurementRunFailureReasonTurnPipelineError.Value),
	})
	switch {
	case err != nil:
		pkglogger.Error(ctx, "procurement: failed to terminalize panicked run",
			zap.Error(err), zap.Int64("run_id", run.ID))
	case !wrote:
		pkglogger.Warn(ctx, "procurement: terminalize panicked run CAS lost, FAILED state was discarded",
			zap.Int64("run_id", run.ID), zap.Int64("expected_version", run.Version))
	}
}

// TurnExecutor is "how a claimed run actually gets executed" — everything
// specific to one dispatch engine: iterating its own event stream,
// translating events into ledger writes, deciding whether this run's
// outcome needs an interrupt and calling CreateInterrupt if so, and
// calling TerminalizeRun itself. This is exactly executeTurnPipeline's
// existing shape (turn.go:939-1056) — TurnUseCase.executeTurnPipeline,
// used as a bound method value, IS a TurnExecutor with zero adaptation
// (Task 3). dispatchTurnViaExecutor below owns claim/replay/dispatch/
// panic-recovery; it does not interpret or write to the run's terminal
// state itself — that stays each executor's own responsibility, because
// the two executors this plan has (legacy, graph) detect "does this run
// need an interrupt?" by fundamentally different means (final TurnResult
// fields vs in-flight event inspection) that don't unify cleanly into one
// shared decision point.
type TurnExecutor func(ctx context.Context, run repository.Run, in TurnInput, runStore RunClaimStore) (turnPipelineOutcome, error)

// dispatchTurnViaExecutor is the shared run-claim + replay + dispatch-
// decoupling + panic-recovery layer that ClaimAndDispatchTurnViaGraph
// (graph_dispatch.go) calls into today — its only production caller.
// History: the 2026-08-30 turn-run-lifecycle-shared-executor plan
// extracted it from the legacy ClaimAndDispatchTurn's inline body so both
// ClaimAndDispatchTurn (that plan's Task 3) and ClaimAndDispatchTurnViaGraph
// (Task 4) could share one claim/replay/dispatch/panic-recovery
// implementation instead of each having its own. The T12
// procurement-graph-cutover plan later deleted ClaimAndDispatchTurn and
// the legacy pipeline it dispatched to entirely (Task 6a), leaving
// ClaimAndDispatchTurnViaGraph as the sole caller.
//
// controllerKind (Sub-project 3b, Task 2) records which caller is
// claiming this run — ClaimAndDispatchResume (Task 8) later reads it off
// the run row to decide whether a resume answer replays into the graph or
// into the legacy answer pipeline.
//
// Deliberately does NOT call GuardNoOpenInterrupt itself: whether and when
// to guard is caller policy, not something this shared function should
// own (2026-08-30 turn-run-lifecycle-shared-executor plan's Global
// Constraints — kept un-unified on purpose so a caller-specific
// guard-skip condition can't get silently changed by unifying it here).
// Callers guard before calling this function; ClaimAndDispatchTurnViaGraph
// — its only caller today — does so in graph_dispatch.go.
func dispatchTurnViaExecutor(
	ctx context.Context, in TurnInput, runStore RunClaimStore, dispatcher *RunDispatcher,
	controllerKind domain.ControllerKind, executor TurnExecutor, hooks TurnStreamHooks,
) (SubmitTurnResult, error) {
	run, isNew, err := runStore.ClaimRun(ctx, repository.ClaimRunInput{
		BuyerID:         in.BuyerID,
		ClientRequestID: in.ClientRequestID,
		CommandKind:     "SUBMIT_TURN",
		PayloadHash:     canonicalTurnHash(in),
		RunType:         domain.RunTypeTurn,
		ControllerKind:  controllerKind,
		RequestID:       in.RequestID,
	})
	if err != nil {
		return SubmitTurnResult{}, err
	}
	// 刻意放在 !isNew 分支之前：重播也是一個真實存在、已 claim 成功的 run，
	// handler 一樣要先送出 RunAccepted，否則重送的請求會拿到一串沒有 run 身分
	// 的事件（見本 plan 的 Global Constraints）。
	if hooks.OnClaimed != nil {
		hooks.OnClaimed(run.ID)
	}
	if !isNew {
		result, err := replayTurnRun(ctx, run, in, runStore)
		if err != nil {
			// RunID 刻意保留在這個錯誤路徑上（不是回傳零值 SubmitTurnResult{}），
			// 跟本函式下面 dispatch-error 分支（`return SubmitTurnResult{RunID:
			// run.ID, Events: ...}, dispatched.err`）同一個理由：ClaimRun 已經
			// commit，這個 run 在 ledger 裡真的存在，只是這次重播沒有結果可還原
			// ——藏掉 RunID 只會讓呼叫端拿不到任何指向這個真實 run 的線索。目前
			// 每個呼叫端在 err != nil 時都直接丟棄回傳的 SubmitTurnResult，這點
			// 暫時不可觀測，但保留 RunID 是刻意的，不是疏漏。
			return SubmitTurnResult{RunID: run.ID}, err
		}
		return SubmitTurnResult{
			RunID:  run.ID,
			Result: result,
			Events: []TurnEvent{{Kind: TurnEventResult, Result: result}},
		}, nil
	}

	done := make(chan turnDispatchResult, 1)
	dispatcher.Dispatch(ctx, func(bgCtx context.Context) {
		out := turnDispatchResult{err: errTurnPipelinePanicked}
		defer func() {
			r := recover()
			if r != nil {
				terminalizeFailedRun(bgCtx, run, in.BuyerID, runStore)
			}
			done <- out
			if r != nil {
				panic(r) // 交還給 SafeGo 記錄完整 stack，不在這一層靜默吞掉
			}
		}()
		outcome, execErr := executor(bgCtx, run, in, runStore)
		out = turnDispatchResult{outcome: outcome, err: execErr}
	})

	// 這個迴圈仍然「一定要等到 done 才離開」——沒有 timeout、沒有 default、
	// 沒有 ctx.Done() 分支。dispatchTurnViaExecutor 對呼叫端的既有保證（回傳時
	// 背景 executor 已經跑完、TerminalizeRun 已經寫過）不因為多了 activity
	// 轉送而改變。
	//
	// hooks.ActivityCh 為 nil 時，該 case 永遠不會 ready（nil channel 的接收
	// 永遠阻塞），select 只會在 done 上醒來——零值 hooks 的行為與這個迴圈加入
	// 前的 `dispatched := <-done` 完全相同，因此不需要額外的分支。
	var dispatched turnDispatchResult
	for waiting := true; waiting; {
		select {
		case raw := <-hooks.ActivityCh:
			if hooks.OnActivity != nil {
				hooks.OnActivity(raw)
			}
		case dispatched = <-done:
			waiting = false
		}
	}
	drainActivity(hooks.ActivityCh, hooks.OnActivity)
	if dispatched.err != nil {
		return SubmitTurnResult{
			RunID:  run.ID,
			Events: dispatched.outcome.events,
		}, dispatched.err
	}
	return SubmitTurnResult{
		RunID:         run.ID,
		Result:        dispatched.outcome.result,
		Events:        dispatched.outcome.events,
		InterruptID:   dispatched.outcome.interruptID,
		InterruptKind: dispatched.outcome.interruptKind,
	}, nil
}

// drainActivity 非阻塞地清空 ch，把還留在裡面的 payload 交給 onActivity。
// 刻意不 range、不 close：channel 由呼叫端（SubmitTurnStream）建立，沒有任何
// 人 close 它——close 之後 graph 那邊還在跑的 producer 一送就 panic——dispatch
// 結束後整條 channel 由 GC 回收。
//
// 這是 best-effort 收尾。上面的 select 迴圈在正常情況下已經先消費完，drain 多半
// 是 no-op；它存在只是為了涵蓋「executor 送完最後一筆之後立刻回傳，done 與最後
// 那筆 payload 同時 ready，select 隨機挑中 done」這個時序。
//
// 迴圈一定會結束：producer 的每一次送出都發生在 executor 內，而 done 是
// executor 回傳之後才被寫入的（dispatchTurnViaExecutor 的 defer），所以走到
// 這裡時已經沒有任何 producer 還活著，最多轉發 cap(ch) 筆就會落到 default。
//
// 測試策略（誠實記錄，不要為了好看補測試）：
//   - 本函式自身的轉發行為有直接測試釘住：TestDrainActivity_ForwardsBufferedPayloadsAndReturns
//     用預先填好的 buffered channel 直接呼叫它，完全不涉及 scheduler。
//   - 沒有被釘住的是「drain 是唯一遞送路徑」這個情境：它無法被穩定重現（select
//     迴圈通常先消費完，兩個 case 同時 ready 時誰先醒由 scheduler 決定），所以
//     沒有寫那種測試——那只會偵測 scheduler 運氣，綠燈不代表 drain 有效。
//   - 「activity 的送出必定早於 done 被寫入」是同一個 goroutine 內依序執行的
//     結構性性質，從函式外部不可觀測，沒有寫測試。
func drainActivity(ch chan []byte, onActivity func([]byte)) {
	if ch == nil || onActivity == nil {
		return
	}
	for {
		select {
		case raw := <-ch:
			onActivity(raw)
		default:
			return
		}
	}
}
