package usecase

import (
	"context"
	"errors"
	"net/http"
	"testing"

	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

func TestModelLLMSlotExtractor_Extract_ToolCall_ParsesFunctionCallArgs(t *testing.T) {
	fake := &testkit.FakeLLM{
		Resp: &model.LLMResponse{
			Content: &genai.Content{
				Parts: []*genai.Part{{
					FunctionCall: &genai.FunctionCall{
						Name: extractSlotPatchToolName,
						Args: map[string]any{"item": "寶島梨禮盒", "quantity": float64(50), "unit": "BOX"},
					},
				}},
			},
		},
	}
	e := &modelLLMSlotExtractor{
		llm:           fake,
		modelName:     "test-model",
		applyConfig:   applyToolCallConfig,
		parseResponse: toolCallParse,
		classifyErr:   func(err error) error { return err },
	}

	result, err := e.Extract(t.Context(), "我要訂寶島梨禮盒 50 盒", domain.Slots{})
	require.NoError(t, err)
	require.Equal(t, "寶島梨禮盒", *result.Patch.Item)
	require.Equal(t, 50, *result.Patch.Quantity)
	require.Equal(t, "BOX", *result.Patch.Unit)

	// 順便釘住請求組裝：tool 有掛上、強制呼叫、model 名稱正確傳遞。
	require.Equal(t, "test-model", fake.LastReq.Model)
	require.Len(t, fake.LastReq.Config.Tools, 1)
	require.Equal(t, extractSlotPatchToolName, fake.LastReq.Config.Tools[0].FunctionDeclarations[0].Name)
	require.Equal(t, genai.FunctionCallingConfigModeAny, fake.LastReq.Config.ToolConfig.FunctionCallingConfig.Mode)
	require.Empty(t, fake.LastReq.Config.ToolConfig.FunctionCallingConfig.AllowedFunctionNames,
		"只宣告一個 tool 時 allow-list 沒有語意價值，卻會讓 openaimodel 送出相容性最差的 "+
			`tool_choice:{"type":"allowed_tools"} wire 形狀（見 applyToolCallConfig）`)
}

func TestModelLLMSlotExtractor_Extract_ToolCall_ReturnsErrorWhenNoFunctionCall(t *testing.T) {
	fake := &testkit.FakeLLM{Resp: &model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{{Text: "抱歉我不太懂"}}}}}
	e := &modelLLMSlotExtractor{llm: fake, modelName: "test-model", applyConfig: applyToolCallConfig, parseResponse: toolCallParse, classifyErr: func(err error) error { return err }}

	_, err := e.Extract(t.Context(), "任何訊息", domain.Slots{})
	require.Error(t, err)
	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable))
}

func TestModelLLMSlotExtractor_Extract_ReturnsErrorOnNilContent(t *testing.T) {
	fake := &testkit.FakeLLM{Resp: &model.LLMResponse{}}
	e := &modelLLMSlotExtractor{llm: fake, modelName: "test-model", applyConfig: applyToolCallConfig, parseResponse: toolCallParse, classifyErr: func(err error) error { return err }}

	_, err := e.Extract(t.Context(), "任何訊息", domain.Slots{})
	require.Error(t, err)
	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable))
}

func TestModelLLMSlotExtractor_Extract_CallsClassifyErrOnLLMError(t *testing.T) {
	sentinel := errors.New("boom")
	fake := &testkit.FakeLLM{Err: sentinel}
	classified := &aidomain.ProviderUnavailableError{Err: sentinel}
	e := &modelLLMSlotExtractor{
		llm: fake, modelName: "test-model", applyConfig: applyToolCallConfig, parseResponse: toolCallParse,
		classifyErr: func(err error) error {
			require.Equal(t, sentinel, err)
			return classified
		},
	}

	_, err := e.Extract(t.Context(), "任何訊息", domain.Slots{})
	require.ErrorIs(t, err, classified)
}

func TestModelLLMSlotExtractor_Extract_Schema_ParsesTextResponse(t *testing.T) {
	fake := &testkit.FakeLLM{
		Resp: &model.LLMResponse{
			Content: &genai.Content{Parts: []*genai.Part{{Text: `{"item":"寶島梨禮盒","quantity":50,"unit":"BOX"}`}}},
		},
	}
	e := &modelLLMSlotExtractor{
		llm: fake, modelName: "test-model", applyConfig: applySchemaConfig, parseResponse: schemaParse,
		classifyErr: func(err error) error { return err },
	}

	result, err := e.Extract(t.Context(), "我要訂寶島梨禮盒 50 盒", domain.Slots{})
	require.NoError(t, err)
	require.Equal(t, "寶島梨禮盒", *result.Patch.Item)
	require.Equal(t, 50, *result.Patch.Quantity)
	require.Equal(t, "BOX", *result.Patch.Unit)

	require.Equal(t, "application/json", fake.LastReq.Config.ResponseMIMEType)
	require.NotNil(t, fake.LastReq.Config.ResponseSchema)
}

func TestModelLLMSlotExtractor_Extract_Schema_ReturnsErrorOnEmptyText(t *testing.T) {
	fake := &testkit.FakeLLM{Resp: &model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{{Text: ""}}}}}
	e := &modelLLMSlotExtractor{llm: fake, modelName: "test-model", applyConfig: applySchemaConfig, parseResponse: schemaParse, classifyErr: func(err error) error { return err }}

	_, err := e.Extract(t.Context(), "任何訊息", domain.Slots{})
	require.Error(t, err)
	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable))
}

// 回歸測試（原 TestDeepSeekSlotExtractor_Extract_WrapsMalformedResponseAsProviderUnavailable
// 的等價版本，改釘在共用骨架這一層）：模型吐出形狀不對的回應是機率性的，
// 重送有機會成功，必須包成 aidomain.ProviderUnavailableError，否則
// retryingExtractor 會當成不可重試、該輪直接降級。
func TestModelLLMSlotExtractor_Extract_ToolCall_WrapsMalformedArgsAsProviderUnavailable(t *testing.T) {
	fake := &testkit.FakeLLM{
		Resp: &model.LLMResponse{
			Content: &genai.Content{
				Parts: []*genai.Part{{
					FunctionCall: &genai.FunctionCall{
						Name: extractSlotPatchToolName,
						// quantity 在 domain.SlotPatch 是 *int，字串進不去。
						Args: map[string]any{"item": "寶島梨禮盒", "quantity": "not-a-number"},
					},
				}},
			},
		},
	}
	e := &modelLLMSlotExtractor{
		llm: fake, modelName: "test-model", applyConfig: applyToolCallConfig, parseResponse: toolCallParse,
		classifyErr: func(err error) error { return err },
	}

	_, err := e.Extract(t.Context(), "我要訂寶島梨禮盒", domain.Slots{})
	require.Error(t, err)
	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable),
		"回應解析失敗必須可重試，否則 retryingExtractor 不會重送、該輪直接降級")
}

// schema 分支（Gemini）與 tool-call 分支共用同一個 parseSlotPatch 包裝規則，
// 兩條路徑都釘住，避免只修一邊。
func TestModelLLMSlotExtractor_Extract_Schema_WrapsMalformedTextAsProviderUnavailable(t *testing.T) {
	fake := &testkit.FakeLLM{
		Resp: &model.LLMResponse{
			Content: &genai.Content{Parts: []*genai.Part{{Text: `{"item":"寶島梨禮盒","quantity":"not-a-number"}`}}},
		},
	}
	e := &modelLLMSlotExtractor{
		llm: fake, modelName: "test-model", applyConfig: applySchemaConfig, parseResponse: schemaParse,
		classifyErr: func(err error) error { return err },
	}

	_, err := e.Extract(t.Context(), "我要訂寶島梨禮盒", domain.Slots{})
	require.Error(t, err)
	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable),
		"回應解析失敗必須可重試，否則 retryingExtractor 不會重送、該輪直接降級")
}

func TestClassifyOpenAICompatExtractErr_5xxIsRetryable(t *testing.T) {
	err := classifyOpenAICompatExtractErr(&openai.Error{StatusCode: http.StatusServiceUnavailable})
	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable))
}

func TestClassifyOpenAICompatExtractErr_429IsRetryable(t *testing.T) {
	err := classifyOpenAICompatExtractErr(&openai.Error{StatusCode: http.StatusTooManyRequests})
	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable))
}

func TestClassifyOpenAICompatExtractErr_401IsNotRetryable(t *testing.T) {
	err := classifyOpenAICompatExtractErr(&openai.Error{StatusCode: http.StatusUnauthorized})
	var unavailable *aidomain.ProviderUnavailableError
	require.False(t, errors.As(err, &unavailable),
		"401 是請求本身的問題（api key 失效），重送不會變好，不得標成可重試")
}

func TestClassifyOpenAICompatExtractErr_NonAPIErrorIsRetryable(t *testing.T) {
	// 連線層錯誤（DNS、連不上）不會是 *openai.Error，保守視為可重試——
	// 同現有 DeepSeek／Gemma 手刻邏輯對「httpClient.Do 本身失敗」的判斷。
	err := classifyOpenAICompatExtractErr(errors.New("dial tcp: connection refused"))
	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable))
}

func TestClassifyOpenAICompatExtractErr_NilIsNil(t *testing.T) {
	require.NoError(t, classifyOpenAICompatExtractErr(nil))
}

func TestModelLLMSlotExtractor_Extract_RecordsTokenUsage(t *testing.T) {
	collect := observability.InstallManualReader(t)
	fake := &testkit.FakeLLM{Resp: &model.LLMResponse{
		Content: &genai.Content{Parts: []*genai.Part{{
			FunctionCall: &genai.FunctionCall{Name: extractSlotPatchToolName, Args: map[string]any{"item": "高麗菜"}},
		}}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 500, CandidatesTokenCount: 20},
	}}
	e := &modelLLMSlotExtractor{
		llm:           fake,
		modelName:     "test-model",
		applyConfig:   applyToolCallConfig,
		parseResponse: toolCallParse,
		classifyErr:   classifyOpenAICompatExtractErr,
	}

	_, err := e.Extract(context.Background(), "我要高麗菜", domain.Slots{})
	require.NoError(t, err)

	rm := collect()
	v, ok := observability.CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "extract", "kind": "prompt"})
	require.True(t, ok)
	require.EqualValues(t, 500, v)
	v, ok = observability.CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "extract", "kind": "completion"})
	require.True(t, ok)
	require.EqualValues(t, 20, v)
}
