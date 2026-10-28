// services/ai-service/internal/procurement/usecase/graph_interrupt.go
package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// CropDisambiguationInterruptPayload 是掛起事件的 Payload——執行緒（任務 9 的
// executeTurnPipeline 等價層）讀 Kind 判斷要不要呼叫既有 Postgres run ledger 的
// CreateInterrupt，對齊既有 interruptOutcomeFor（run_dispatch.go:74-79）的訊號
// 語意（判準是 Question.SlotKey == domain.SlotCropDisambiguation，不是 act
// code——這裡的 Kind 是同一個哨兵字串 "crop_disambiguation"
// (domain.SlotCropDisambiguation, pick_next_question.go:33) 的 node 端等價物）。
//
// 這個型別同時是**落庫形狀**：graph_dispatch.go 把它序列化進
// procurement_agent_run_interrupts.payload_json，crop_gate.go 與 GetSnapshot
// 再從那裡讀回來。刻意不另外定義一個「DB 版」的孿生 struct——node 邊界送的與
// 落庫的本來就是同一份內容，兩份定義只會漂移（CLAUDE.md §15）。
//
// PendingReason 是顯式欄位，不從 len(Candidates) 推導：0 候選的待處理狀態
// （UNAVAILABLE）本來就沒有候選可存，靠長度推斷結構性地表達不出它——那正是
// Cut 1 新增 crop_resolution_pending_reason 要解決的問題，不能在搬家的時候
// 又推回去。
type CropDisambiguationInterruptPayload struct {
	Kind          string                            `json:"kind"`
	PendingReason domain.CropPendingReason          `json:"pendingReason"`
	Candidates    []domain.CropDisambiguationOption `json:"candidates"`
	Selection     *domain.CropSelection             `json:"selection,omitempty"`
	// WorkflowInterruptID targets this specific ADK pause; the ledger has its own ID.
	WorkflowInterruptID string `json:"workflowInterruptId,omitempty"`
	// FactsAsOf 走這裡是因為 node 邊界只有 payload 這一條通道。落庫時它被
	// 抄進 procurement_agent_run_interrupts.facts_as_of 那個獨立欄位——同一次
	// INSERT、同一個變數，兩者不可能漂移，欄位是 payload 這個值的可查詢投影
	// （同 procurement_candidates 把 facts_as_of 拉成欄位而非埋在
	// matched_facts_json 裡的既有做法）。
	FactsAsOf time.Time `json:"factsAsOf"`
}

const cropDisambiguationInterruptKind = "crop_disambiguation"

const cropWorkflowInterruptStateKey = "crop_disambiguation_workflow_interrupt_id"

// NewCropDisambiguationInterruptNode parks each classification stage without
// rerunning extraction or retrieval. Resume re-enters this one node with the
// output already computed by resumeGraphTurn. Pending answers create another
// pause; only a completed answer reaches crop_resume_reconcile and terminal.
//
// Each stage needs a distinct workflow ID: ADK reconstructs every answer in the
// invocation and considers an earlier ID resolved forever. A loop through a
// handoff successor is also unsafe because ADK skips already-completed
// successors on later resumes. Re-entry avoids both assumptions.
//
// Production supplies the same ID generator as the other graph nodes. The
// optional argument preserves the old initial ID for existing clients/tests;
// subsequent IDs still change, including after a process restart.
func NewCropDisambiguationInterruptNode(idGenerators ...func() int64) *workflow.FunctionNode {
	rerun := true
	return workflow.NewEmittingFunctionNode(
		"crop_disambiguation_interrupt",
		func(ctx agent.Context, in CropGateOutput, emit func(*session.Event) error) (CropGateOutput, error) {
			previousID, err := stateValue[string](ctx, cropWorkflowInterruptStateKey)
			if err != nil && !errors.Is(err, session.ErrStateKeyNotExist) {
				return CropGateOutput{}, err
			}
			if previousID == "" {
				previousID = cropDisambiguationInterruptKind
			}
			response, resumed := ctx.ResumedInput(previousID)
			if resumed {
				in, err = decodeResumedCropGateOutput(response)
				if err != nil {
					return CropGateOutput{}, err
				}
				if err := syncResumedCropProgress(ctx, emit, in); err != nil {
					return CropGateOutput{}, err
				}
			}
			if !in.Pending {
				return in, nil
			}

			interruptID := cropDisambiguationInterruptKind
			if len(idGenerators) > 0 && idGenerators[0] != nil {
				interruptID = fmt.Sprintf("%s-%d", cropDisambiguationInterruptKind, idGenerators[0]())
			} else if resumed {
				interruptID = previousID + "-next"
			}
			ev := workflow.NewRequestInputEvent(ctx, session.RequestInput{
				InterruptID: interruptID,
				Message:     "需要買家確認品項分類或描述品項",
				Payload: CropDisambiguationInterruptPayload{
					Kind:                cropDisambiguationInterruptKind,
					PendingReason:       in.PendingReason,
					Candidates:          in.Candidates,
					Selection:           in.Selection,
					WorkflowInterruptID: interruptID,
					FactsAsOf:           in.FactsAsOf,
				},
			})
			ev.Actions.StateDelta[cropWorkflowInterruptStateKey] = interruptID
			if err := emit(ev); err != nil {
				return CropGateOutput{}, err
			}
			return CropGateOutput{}, workflow.ErrNodeInterrupted
		},
		workflow.NodeConfig{RerunOnResume: &rerun},
	)
}

// Postgres-backed ADK sessions hydrate interface values as JSON maps. A direct
// CropGateOutput assertion would miss the answer and ask the original question
// again, so use the same typed/JSON boundary as stateValue.
func decodeResumedCropGateOutput(raw any) (CropGateOutput, error) {
	if out, ok := raw.(CropGateOutput); ok {
		return out, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return CropGateOutput{}, fmt.Errorf("encode resumed crop gate output: %w", err)
	}
	var out *CropGateOutput
	if err := json.Unmarshal(b, &out); err != nil {
		return CropGateOutput{}, fmt.Errorf("decode resumed crop gate output: %w", err)
	}
	if out == nil {
		return CropGateOutput{}, errors.New("resumed crop gate output is null")
	}
	return *out, nil
}

func syncResumedCropProgress(ctx agent.Context, emit func(*session.Event) error, in CropGateOutput) error {
	progress, err := readGraphTurnProgress(ctx)
	if err != nil {
		return err
	}
	progress.CropPending = in.Pending
	progress.CropCandidates = in.Candidates
	progress.CropSelection = in.Selection
	progress.CropGateEvaluated = false
	progress.Narration = withoutStepNarration(progress.Narration, domain.StepCropGate)
	progress.NarrationSummaries = withoutNarrationSummary(progress.NarrationSummaries, domain.StepCropGate)
	if in.Item != nil {
		progress.NextSlots.Item = in.Item
	}
	if in.Resolution != nil {
		progress.NextSlots.ItemResolution = in.Resolution
		progress.Patch.ItemResolution = in.Resolution
	}
	return emitGraphTurnProgress(ctx, emit, progress)
}

// NewCropResumeReconcileNode also supports a resolved handoff from previously
// parked sessions and direct graph callers. New re-entry resumes already sync
// their progress in the interrupt node; this then passes through unchanged.
func NewCropResumeReconcileNode() *workflow.FunctionNode {
	return workflow.NewEmittingFunctionNode(
		"crop_resume_reconcile",
		func(ctx agent.Context, in CropGateOutput, emit func(*session.Event) error) (CropGateOutput, error) {
			progress, err := readGraphTurnProgress(ctx)
			if err != nil {
				return CropGateOutput{}, err
			}
			if !progress.CropPending || in.Pending {
				return in, nil
			}
			progress.CropPending = false
			progress.CropCandidates = nil
			progress.CropSelection = nil
			// 🔴 gate 的敘述狀態必須跟 CropPending 一起收乾淨。這份 progress 是
			// 被暫停那一輪留下來的（ADK session state 跨 interrupt 持久化），裡面
			// CropGateEvaluated 為 true、NarrationSummaries[CROP_GATE] 是那一輪算的
			// 「品項尚待買家從 N 個候選作物中選擇」摘要。買家現在剛選完，那句話已經
			// 不成立，但 terminal 只看這幾個欄位，會照樣在 resume 這一輪為 CROP_GATE
			// 生成一句敘述並填上那個過期的摘要。
			//
			// 清掉而不是重新算摘要：本節點是純狀態調和，不做 I/O，沒有理由讓一個
			// 已經作廢的判定在 terminal 那一端多耗一次敘述；選擇結果本來就會出現在
			// 這一輪的回覆與 recordedThisTurn 裡。
			progress.CropGateEvaluated = false
			progress.Narration = withoutStepNarration(progress.Narration, domain.StepCropGate)
			progress.NarrationSummaries = withoutNarrationSummary(progress.NarrationSummaries, domain.StepCropGate)
			if in.Item != nil {
				progress.NextSlots.Item = in.Item
			}
			// 買家挑中的解析結果寫進這一輪的 slots 與 patch——跟 crop_gate 在正常
			// 路徑上做的是同一件事，只是這一輪 crop_gate 不會重跑（handoff 模式），
			// 所以由這個節點代勞。Patch 那一份是①確認段「品項分類＝甘藍-初秋」的
			// 唯一來源：terminal 的 RecordedThisTurn 只看 patch 有沒有帶到。
			if in.Resolution != nil {
				progress.NextSlots.ItemResolution = in.Resolution
				progress.Patch.ItemResolution = in.Resolution
			}
			if err := emitGraphTurnProgress(ctx, emit, progress); err != nil {
				return CropGateOutput{}, err
			}
			return in, nil
		},
		workflow.NodeConfig{},
	)
}
