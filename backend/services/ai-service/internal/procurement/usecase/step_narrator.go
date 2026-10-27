// services/ai-service/internal/procurement/usecase/step_narrator.go
//
// StepNarrator turns validate_slots/crop_gate's structured outcome into
// one short buyer-facing sentence (T12 sub-project 3c design §4.3) —
// mirrors SlotExtractor's provider selection (extractor_provider.go:
// same provider switch, same API keys) so this is a second
// small client for a narration-specific prompt, not a new provider/config
// surface. Deliberately a separate client instance from SlotExtractor's
// (not literally shared) — the graph itself holds no model.LLM; this file
// still builds its own client from p.LLM() rather than reusing
// SlotExtractor's, matching the same per-usecase construction pattern
// (extractor_provider.go, surface_realizer.go) instead of introducing
// shared mutable state across usecases. This trades one extra lightweight
// client object (same construction pattern, no new infrastructure) for a
// simpler, decoupled DI graph.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// narrationTimeout bounds the narration call — short relative to
// ExtractTimeout (extract.go) because this is decorative UX, not on the
// business-critical path: the node's real work has already finished by
// the time narrateStepOutcome is called (see design doc §4.3/§5), so a
// slow or failing narration call must never meaningfully extend a turn.
const narrationTimeout = 3 * time.Second

// narrationInstruction constrains the model to paraphrasing only — no new
// facts, no numbers/amounts/store names not already in the summary (same
// discipline as extract.go's extractInstruction's ShortIntent guidance).
const narrationInstruction = `你是採購助理。用一句話（不超過 30 字）跟買家說明剛剛這個步驟的判斷或結果。
只能重述輸入摘要裡已經有的內容，不得臆測、不得新增摘要沒提到的事實。`

// StepNarrator turns a structured outcome summary into one short sentence.
type StepNarrator interface {
	Narrate(ctx context.Context, code domain.StepCode, summary string) (string, error)
}

// NewStepNarrator constructs the provider-selected StepNarrator — same
// provider switch as extractor_provider.go's
// newBaseSlotExtractor, same API keys, same "unknown provider" error
// shape.
func NewStepNarrator(p llmprovider.Procurement) StepNarrator {
	// backend#3329：開通判定與 client 建構都在 internal/llmprovider。永遠不回
	// error——這支是 fx.Provide 的直接入口，回 error 就是 fx abort。
	//
	// 這裡原本有一整份與 llm_provider.go 相同的 provider switch（含自己再寫一次
	// 的 DeepSeek reasoning.effort=none）。那正是 2026-09-11 C1 的溫床：同樣的
	// quirk 散在四份拷貝裡，漏掉任何一份都是靜默失敗。
	if !p.Enabled() {
		return disabledStepNarrator{}
	}
	return &modelLLMStepNarrator{llm: p.LLM(), modelName: p.ModelName()}
}

type modelLLMStepNarrator struct {
	llm       model.LLM
	modelName string
}

func (n *modelLLMStepNarrator) Narrate(ctx context.Context, code domain.StepCode, summary string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, narrationTimeout)
	defer cancel()

	req := &model.LLMRequest{
		Model:    n.modelName,
		Contents: []*genai.Content{genai.NewContentFromText(summary, genai.RoleUser)},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(narrationInstruction, genai.RoleUser),
			Temperature:       genai.Ptr(float32(0)),
		},
	}

	var resp *model.LLMResponse
	var callErr error
	start := time.Now()
	for r, err := range n.llm.GenerateContent(callCtx, req, false) {
		resp, callErr = r, err
	}
	observability.RecordProcurementNarrationDuration(ctx, string(code), time.Since(start).Seconds())
	if resp != nil {
		observability.RecordProcurementLLMUsage(ctx, "narrate", resp.UsageMetadata)
	}
	if callErr != nil {
		return "", fmt.Errorf("narrating %s: %w", code, callErr)
	}
	if resp == nil || resp.Content == nil {
		return "", fmt.Errorf("narrating %s: empty model response", code)
	}
	return firstContentText(resp.Content), nil
}

// narrateStepOutcome guards a StepNarrator call the same way guardShortIntent
// guards extract_slots' model sentence: empty, too-long, or whitespace-only
// output falls back to fallbackText, as does any provider error or
// exceeding narrationTimeout (enforced inside Narrate itself). Never
// blocks or fails the caller's node — summary is built from output that
// already exists by the time this is called (design doc §4.3/§5).
// ctx 型別是 context.Context 而不是 agent.Context：/answers 與 /rewinds 不跑
// 圖，手上只有一般的 context.Context，而這個函式用到 ctx 的三個地方
// （narrator.Narrate、observability.RecordProcurementNarration 與
// pkglogger.Warn）本來就都只要 context.Context。
func narrateStepOutcome(ctx context.Context, narrator StepNarrator, code domain.StepCode, summary, fallbackText string) string {
	text, err := narrator.Narrate(ctx, code, summary)
	trimmed := strings.TrimSpace(text)
	result := "success"
	switch {
	case errors.Is(err, ErrProcurementLLMDisabled):
		result = "disabled"
	case err != nil:
		result = "error"
	case trimmed == "" || len([]rune(trimmed)) > shortIntentMaxLen:
		result = "rejected"
	default:
		// 事實邊界（doc 57 §3.1 第 3 條），與 ShortIntent 共用同一份判準。
		// reason 直接當 metric 的 result 值——它是固定字面值，低基數，
		// 不含買家原話（見 public_text_policy.go）。
		if reason := publicTextRejection(trimmed); reason != "" {
			result = reason
		}
	}
	observability.RecordProcurementNarration(ctx, string(code), result)
	if result != "success" {
		pkglogger.Warn(ctx, "procurement: narration failed or rejected by guard, using fallback text",
			zap.String("code", string(code)), zap.String("result", result), zap.Error(err))
		return fallbackText
	}
	return trimmed
}
