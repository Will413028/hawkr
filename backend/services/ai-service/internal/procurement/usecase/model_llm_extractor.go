package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/openai/openai-go/v3"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// extractSlotPatchToolName 是強制 tool-call 抽取用的 function 名稱
// （DeepSeek／Gemma 共用，見 applyToolCallConfig）。
const extractSlotPatchToolName = "extract_slot_patch"

// modelLLMSlotExtractor 是三個 provider 共用的抽取骨架：訊息截斷、組已知
// slots JSON、建 model.LLMRequest 都相同，applyConfig／parseResponse／
// classifyErr 三個欄位讓呼叫端決定「用 tool-call 還是 schema」「怎麼從
// 回應拿出 SlotPatch」「怎麼分類錯誤」——不需要為兩種形狀重新設計
// SlotExtractor 介面本身。見
// docs/superpowers/specs/2026-08-30-procurement-slot-extractor-model-llm-design.md §2.2。
type modelLLMSlotExtractor struct {
	llm           model.LLM
	modelName     string
	applyConfig   func(*genai.GenerateContentConfig)
	parseResponse func(*model.LLMResponse) (ExtractResult, error)
	classifyErr   func(error) error
}

func (e *modelLLMSlotExtractor) Extract(ctx context.Context, message string, current domain.Slots) (ExtractResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, ExtractTimeout)
	defer cancel()

	if r := []rune(message); len(r) > MaxMessageRunes {
		message = string(r[:MaxMessageRunes])
	}
	knownJSON, err := json.Marshal(current)
	if err != nil {
		return ExtractResult{}, err
	}

	userText := fmt.Sprintf("目前已知的需求（JSON，勿重複輸出已正確的欄位）：\n%s\n\n買家訊息：\n%s",
		string(knownJSON), message)

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(extractInstruction, genai.RoleUser),
		Temperature:       genai.Ptr(float32(0)),
	}
	e.applyConfig(cfg)

	req := &model.LLMRequest{
		Model:    e.modelName,
		Contents: []*genai.Content{genai.NewContentFromText(userText, genai.RoleUser)},
		Config:   cfg,
	}

	var resp *model.LLMResponse
	var callErr error
	for r, err := range e.llm.GenerateContent(callCtx, req, false) {
		resp, callErr = r, err
	}
	if resp != nil {
		observability.RecordProcurementLLMUsage(ctx, "extract", resp.UsageMetadata)
	}
	if callErr != nil {
		return ExtractResult{}, e.classifyErr(callErr)
	}
	if resp == nil || resp.Content == nil {
		return ExtractResult{}, &aidomain.ProviderUnavailableError{Err: errors.New("slot 抽取回應為空")}
	}
	return e.parseResponse(resp)
}

// applyToolCallConfig 讓模型必須呼叫 extractSlotPatchToolName，拿
// schema-guaranteed 的 tool-call arguments 當抽取結果——DeepSeek／Gemma
// 共用（見 spec §1.3：兩者都不能走 Config.ResponseSchema，openaimodel 對
// 該路徑會強制 OpenAI strict-mode schema，所有欄位變 required）。
func applyToolCallConfig(cfg *genai.GenerateContentConfig) {
	cfg.Tools = []*genai.Tool{{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        extractSlotPatchToolName,
			Description: "把買家訊息中明確提到的採購條件寫入這個 tool 的參數。",
			Parameters:  buildSlotSchema(),
		}},
	}}
	// 只設 Mode 不設 AllowedFunctionNames：這個請求只宣告一個 tool，
	// allow-list 在語意上買不到任何東西（模型沒有第二個 tool 可挑），但
	// openaimodel 會因此把請求映射成 tool_choice:{"type":"allowed_tools",…}
	// 這種最新、相容性最差的 wire 形狀（tools.go convertToolChoice）。留空
	// 則映射成廣泛支援的 tool_choice:"required"，對 DeepSeek／OpenRouter 這種
	// 只實作部分 OpenAI API surface 的 provider 安全得多。
	cfg.ToolConfig = &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: genai.FunctionCallingConfigModeAny,
		},
	}
}

// toolCallParse 從回應找到 extractSlotPatchToolName 的 FunctionCall.Args，
// 複用既有 parseExtractResult（避免第二套 map→struct 映射邏輯）。解析失敗包成
// aidomain.ProviderUnavailableError：模型吐出形狀不對的回應是機率性的，重送
// 有機會成功，必須維持可重試（spec §2.3；extractor_provider.go 的
// retryingExtractor 只重試這個型別）。
func toolCallParse(resp *model.LLMResponse) (ExtractResult, error) {
	for _, part := range resp.Content.Parts {
		if part.FunctionCall != nil && part.FunctionCall.Name == extractSlotPatchToolName {
			b, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return ExtractResult{}, &aidomain.ProviderUnavailableError{
					Err: fmt.Errorf("tool call 參數編碼失敗: %w", err),
				}
			}
			result, err := parseExtractResult(b)
			if err != nil {
				return ExtractResult{}, &aidomain.ProviderUnavailableError{Err: err}
			}
			return result, nil
		}
	}
	return ExtractResult{}, &aidomain.ProviderUnavailableError{Err: errors.New("模型未回傳預期的 tool call")}
}

// applySchemaConfig 讓模型走原生 structured output（Gemini 專用——見 spec
// §1.3，DeepSeek／Gemma 不能走這條路徑）。
func applySchemaConfig(cfg *genai.GenerateContentConfig) {
	cfg.ResponseMIMEType = "application/json"
	cfg.ResponseSchema = buildSlotSchema()
}

// schemaParse 從回應的純文字部分解析 ExtractResult，複用 firstContentText
// （graph_nodes.go，原名 userMessageText——這裡拿的是模型回應而非使用者
// 輸入，改成中性名稱後兩處都能用）與既有 parseExtractResult。解析失敗的包裝
// 理由同 toolCallParse。
func schemaParse(resp *model.LLMResponse) (ExtractResult, error) {
	text := firstContentText(resp.Content)
	if text == "" {
		return ExtractResult{}, &aidomain.ProviderUnavailableError{Err: errors.New("slot 抽取回應為空")}
	}
	result, err := parseExtractResult([]byte(text))
	if err != nil {
		return ExtractResult{}, &aidomain.ProviderUnavailableError{Err: err}
	}
	return result, nil
}

// classifyOpenAICompatExtractErr 把 DeepSeek／Gemma（走 openaimodel）的
// 底層錯誤分類成可不可重試，語意對齊舊版 DeepSeek／Gemma 各自手刻的
// resp.StatusCode 判斷（429／5xx 可重試，其餘 4xx 不可重試）。openaimodel
// 把底層錯誤包成 fmt.Errorf("openai: call failed: %w", err)，err 有 HTTP
// 回應時是 *openai.Error（github.com/openai/openai-go/v3 的
// type alias，有 StatusCode int 欄位）；連線層錯誤（連不上、DNS 失敗）
// openai-go 不會產生 *openai.Error，保守視為可重試。
func classifyOpenAICompatExtractErr(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500 {
			return &aidomain.ProviderUnavailableError{Err: err}
		}
		return err
	}
	return &aidomain.ProviderUnavailableError{Err: err}
}
