// services/ai-service/internal/procurement/usecase/surface_realizer.go
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// RealizeTimeout 是單次 SurfaceRealizer 呼叫的硬上限。4 秒：realize 掛在
// composeXxx 尾巴的 syncutil 平行群組裡，跟 narrateStepOutcome（narrationTimeout
// ＝3s）併排跑，整個 tail 是 max 而不是 sum。被刪版本的 20s 是序列時代的值，
// 照抄會把每一輪最壞情況多加 20 秒。
const RealizeTimeout = 4 * time.Second

// realizeBlocksToolName 是強制 tool-call 路徑的 function 名稱，語意同
// extractSlotPatchToolName（model_llm_extractor.go）。
const realizeBlocksToolName = "write_narrative_blocks"

// SurfaceContext 是 SurfaceRealizer 的 bounded 輸入（issue #2549 §D：
// 「只接收 bounded SurfaceContext 與 exact DialoguePlan」）。Args 直接沿用
// 呼叫端已經組好、傳給 domain.RenderPlan 的同一份 domain.SafeRenderArgs——
// 確保模型「知道」的內容跟 deterministic fallback 會說的內容完全一致，不
// 是兩套各自推導的欄位解析邏輯。
type SurfaceContext struct {
	Plan             domain.DialoguePlanV1
	Args             domain.SafeRenderArgs
	RecentNarratives []string // anti-repetition，最多 1-2 筆，見 surface_context.go
}

// SurfaceRealizer 是 stateless 模型潤飾層的唯一介面（issue #2549 §D）。
type SurfaceRealizer interface {
	Realize(ctx context.Context, sc SurfaceContext) ([]RealizedBlock, error)
}

// NewSurfaceRealizer 組裝敘述潤飾層。client 由 internal/llmprovider 提供——
// 這裡不建、也不判斷開通與否。
//
// 請求形狀比照 modelLLMSlotExtractor（model_llm_extractor.go）：DeepSeek／Gemma
// 走強制 tool-call，Gemini 走原生 structured output。兩條路徑的理由見
// model_llm_extractor.go:78-82：openaimodel 對 Config.ResponseSchema 會強制
// OpenAI strict-mode schema，所有欄位變 required。
//
// 🔴 這支曾經自己刻過一份 client 建構，漏掉 DeepSeek 的 reasoning.effort=none
// （2026-09-11 whole-branch review C1）。正式端點上少了它就是 100% HTTP 400、
// 且 400 不可重試——表現出來只是「每一輪都退回模板」的靜默失敗，單元測試與 CI
// 全綠。那次的修法是改呼叫既有的收斂點；這次的修法是讓「自己刻一份」變成寫不
// 出來的東西（llmprovider ＋ scripts/check-llm-client-construction.py）。
//
// **永遠不回 error**：fx.Provide 的直接入口，回 error 就是 fx abort。
func NewSurfaceRealizer(p llmprovider.Procurement) SurfaceRealizer {
	if !p.Enabled() {
		return disabledSurfaceRealizer{}
	}
	inner := &modelLLMSurfaceRealizer{llm: p.LLM(), modelName: p.ModelName()}
	switch p.Provider() {
	case "deepseek", "gemma":
		inner.applyConfig = applyBlocksToolCallConfig
		inner.parse = toolCallParseBlocks
		inner.classifyErr = classifyOpenAICompatExtractErr
	default: // gemini——Enabled() 為真時 Provider() 必為三者之一
		inner.applyConfig = applyBlocksSchemaConfig
		inner.parse = schemaParseBlocks
		inner.classifyErr = classifyGeminiExtractErr
	}
	return &retryingSurfaceRealizer{inner: inner}
}

type modelLLMSurfaceRealizer struct {
	llm         model.LLM
	modelName   string
	applyConfig func(*genai.GenerateContentConfig)
	parse       func(*model.LLMResponse) ([]RealizedBlock, error)
	classifyErr func(error) error
}

func (e *modelLLMSurfaceRealizer) Realize(ctx context.Context, sc SurfaceContext) ([]RealizedBlock, error) {
	callCtx, cancel := context.WithTimeout(ctx, RealizeTimeout)
	defer cancel()

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(surfaceRealizeInstruction, genai.RoleUser),
		// 敘述潤飾需要自然變化，不像 slot extraction 要求逐字可重現；仍保留
		// 中低值避免離題。
		Temperature: genai.Ptr(float32(0.7)),
	}
	e.applyConfig(cfg)

	req := &model.LLMRequest{
		Model:    e.modelName,
		Contents: []*genai.Content{genai.NewContentFromText(buildSurfaceRealizePrompt(sc), genai.RoleUser)},
		Config:   cfg,
	}

	var resp *model.LLMResponse
	var callErr error
	for r, err := range e.llm.GenerateContent(callCtx, req, false) {
		resp, callErr = r, err
	}
	if resp != nil {
		observability.RecordProcurementLLMUsage(ctx, "realize", resp.UsageMetadata)
	}
	if callErr != nil {
		return nil, e.classifyErr(callErr)
	}
	if resp == nil || resp.Content == nil {
		return nil, &aidomain.ProviderUnavailableError{Err: errors.New("surface realize 回應為空")}
	}
	return e.parse(resp)
}

// surfaceRealizeInstruction 是固定的 system prompt 骨架；每次呼叫時把
// allowed／forbidden claim code、maxSentences、tone 等 plan-specific 內容
// 動態組進 user content（見 buildSurfaceRealizePrompt），不是這裡。
const surfaceRealizeInstruction = `你是一個採購助手的敘述潤飾器。你的任務是把「已經決定好的事實與下一步」換句話說成自然、親切的繁體中文，不是自己決定要說什麼。

規則：
- 只能使用你被明確告知的事實與下一步，不得推測、不得補充任何未提供的資訊。
- 每個段落（block）用 kind 分類語意：UNDERSTOOD（已理解／已記錄的事實）、CONTEXT（補充脈絡）、RATIONALE（解釋原因）、WARNING（提醒或未採用的內容）、NEXT_STEP（接下來要做什麼／要問什麼）、COMPLETE（這一步已完成）。
- 每個 block 必須附上 claimCodes：從你被告知的 allowedClaimCodes 清單中選出這段文字實際主張的項目，不得使用清單外的值，不得留空陣列（每個 block 至少一個 claimCode）。
- 絕對不能使用 forbiddenClaimCodes 對應的任何主張，也不能在文字裡暗示這些事已經發生。
- 除了上面明確告知你的事實值、以及「還缺幾個欄位」這一個數量以外，不得出現任何數字。
- 數量一律用阿拉伯數字寫（「還缺 2 項」，不要寫成「還缺兩項」）——驗證只認阿拉伯數字，中文數字會讓它看不見你講了什麼。
- 不得提及候選、賣家、報價或訂單的數量與狀態——那些不在你被告知的範圍內，系統也還沒做那些事。
- 不使用 URL、HTML 標籤、Markdown 語法。
- 用繁體中文，語氣依 toneHints 指示。
- 只輸出 JSON 物件：{"blocks":[{"kind":"...","text":"...","claimCodes":["..."]}]}，不要輸出任何額外文字。`

func buildSurfaceRealizePrompt(sc SurfaceContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "actCode: %s\n", sc.Plan.ActCode)
	fmt.Fprintf(&b, "allowedClaimCodes: %s\n", strings.Join(sc.Plan.AllowedClaimCodes, ", "))
	fmt.Fprintf(&b, "forbiddenClaimCodes: %s\n", strings.Join(sc.Plan.ForbiddenClaimCodes, ", "))
	fmt.Fprintf(&b, "maxSentences: %d\n", sc.Plan.MaxSentences)
	fmt.Fprintf(&b, "toneHints.verbosity: %s\n", sc.Plan.ToneHints.Verbosity)
	if len(sc.Args.SalientFactLabels) > 0 {
		fmt.Fprintf(&b, "已記錄的事實: %s\n", strings.Join(sc.Args.SalientFactLabels, "、"))
	}
	if len(sc.Args.MissingRequiredLabels) > 0 {
		fmt.Fprintf(&b, "還缺少的欄位: %s\n", strings.Join(sc.Args.MissingRequiredLabels, "、"))
	}
	if sc.Args.NextQuestionPrompt != "" {
		fmt.Fprintf(&b, "下一個問題: %s\n", sc.Args.NextQuestionPrompt)
	}
	if len(sc.Args.RejectedLabels) > 0 {
		fmt.Fprintf(&b, "這次沒辦法採用的內容: %s\n", strings.Join(sc.Args.RejectedLabels, "、"))
	}
	if len(sc.RecentNarratives) > 0 {
		fmt.Fprintf(&b, "\n以下是最近已經說過的內容，這次的敘述請避免重複相同句型與用詞：\n%s\n", strings.Join(sc.RecentNarratives, "\n---\n"))
	}
	return b.String()
}

// buildBlocksSchema 是 blocks[] 的 JSON schema。kind 用封閉 enum（doc 57 §3.3
// 的六個值）——值域寫進 schema 而不是只靠 prompt，validator 的 unknown_kind
// 檢查因此是第二道而非唯一一道。
func buildBlocksSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"blocks": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"kind": {Type: genai.TypeString, Enum: []string{
							string(domain.BlockUnderstood), string(domain.BlockContext),
							string(domain.BlockRationale), string(domain.BlockWarning),
							string(domain.BlockNextStep), string(domain.BlockComplete),
						}},
						"text":       {Type: genai.TypeString, MaxLength: genai.Ptr(int64(maxBlockTextRunes))},
						"claimCodes": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					},
				},
			},
		},
	}
}

func applyBlocksSchemaConfig(cfg *genai.GenerateContentConfig) {
	cfg.ResponseMIMEType = "application/json"
	cfg.ResponseSchema = buildBlocksSchema()
}

func applyBlocksToolCallConfig(cfg *genai.GenerateContentConfig) {
	cfg.Tools = []*genai.Tool{{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        realizeBlocksToolName,
			Description: "把已經決定好的事實與下一步換句話說成自然的繁體中文段落。",
			Parameters:  buildBlocksSchema(),
		}},
	}}
	// 只設 Mode 不設 AllowedFunctionNames，理由同 applyToolCallConfig
	// （model_llm_extractor.go）：留空會映射成廣泛支援的 tool_choice:"required"。
	cfg.ToolConfig = &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAny},
	}
}

func toolCallParseBlocks(resp *model.LLMResponse) ([]RealizedBlock, error) {
	for _, part := range resp.Content.Parts {
		if part.FunctionCall != nil && part.FunctionCall.Name == realizeBlocksToolName {
			b, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, &aidomain.ProviderUnavailableError{Err: fmt.Errorf("tool call 參數編碼失敗: %w", err)}
			}
			return parseRealizedBlocks(b)
		}
	}
	return nil, &aidomain.ProviderUnavailableError{Err: errors.New("模型未回傳預期的 tool call")}
}

func schemaParseBlocks(resp *model.LLMResponse) ([]RealizedBlock, error) {
	text := firstContentText(resp.Content)
	if text == "" {
		return nil, &aidomain.ProviderUnavailableError{Err: errors.New("surface realize 回應為空")}
	}
	return parseRealizedBlocks([]byte(text))
}

// parseRealizedBlocks 只負責形狀；語意合法性（kind 值域、claim code、長度、
// fact-echo）全部由 validateSurfaceBlocks 把關，這裡不重複判斷。
func parseRealizedBlocks(raw []byte) ([]RealizedBlock, error) {
	var out struct {
		Blocks []RealizedBlock `json:"blocks"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &aidomain.ProviderUnavailableError{Err: fmt.Errorf("surface realize JSON 解析失敗: %w", err)}
	}
	if len(out.Blocks) == 0 {
		return nil, &aidomain.ProviderUnavailableError{Err: errors.New("surface realize blocks 為空")}
	}
	return out.Blocks, nil
}

// retryingSurfaceRealizer 比照 extractor_provider.go 的 retryingExtractor：
// 只重試一次、只重試 aidomain.ProviderUnavailableError，並記錄耗時 metric。
type retryingSurfaceRealizer struct {
	inner SurfaceRealizer
}

// maxRealizeAttempts bounds the retry loop in Realize below: the worst-case
// number of attempts (1 initial + up to maxRealizeAttempts-1 retries) before
// Realize gives up. It is a real, load-bearing input to that loop, not just
// a label — changing it changes how many times Realize actually retries.
// It is also the source observability.RealizeAttemptsForBuckets mirrors so
// procurement_surface_realize_duration_seconds's histogram bucket top covers
// a retried call (maxRealizeAttempts× RealizeTimeout), not just the fast
// path — see TestRealizeAttempts_MatchesHistogramBucketMirror.
//
// (final review Finding 4, 2026-09-19: this constant used to be declared
// but never read — the retry itself was a single hand-written if-block, so
// changing this number changed nothing about Realize's actual behaviour
// while the histogram mirror stayed green, which is exactly the "guard that
// only appears to guard" class this branch exists to remove. Realize is now
// driven by the constant instead.)
const maxRealizeAttempts = 2

func (r *retryingSurfaceRealizer) Realize(ctx context.Context, sc SurfaceContext) ([]RealizedBlock, error) {
	start := time.Now()
	blocks, err := r.inner.Realize(ctx, sc)

	var unavailable *aidomain.ProviderUnavailableError
	for attempt := 2; attempt <= maxRealizeAttempts && err != nil && errors.As(err, &unavailable); attempt++ {
		retryCtx, cancel := context.WithTimeout(ctx, RealizeTimeout)
		blocks, err = r.inner.Realize(retryCtx, sc)
		cancel()
	}

	observability.RecordProcurementSurfaceRealizeDuration(ctx, time.Since(start).Seconds())
	return blocks, err
}
