package usecase

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekSlotExtractor_Extract_SendsToolCallRequest(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/responses", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		require.Equal(t, llmprovider.DeepSeekModelName, capturedBody["model"],
			"送上 wire 的 model 必須來自唯一定義，寫死字面值會在改名時靜默漂移")
		tools, ok := capturedBody["tools"].([]any)
		require.True(t, ok, "請求應帶 tools 陣列")
		require.NotEmpty(t, tools)

		// instructions 是 openaimodel 對 Config.SystemInstruction 的 wire 欄位，
		// 整套抽取規則（單位允許清單、不要正規化城市、不要猜）都掛在這裡；
		// 掉了模型照樣會回 tool call，但抽出來的東西全錯，所以要釘住。
		instructions, ok := capturedBody["instructions"].(string)
		require.True(t, ok, "請求應把 SystemInstruction 送成 instructions 字串")
		require.NotEmpty(t, instructions)
		require.Equal(t, extractInstruction, instructions)
		require.Contains(t, instructions, strings.Join(domain.AllowedUnits, "/"),
			"單位允許清單必須真的進到 wire 上的 instructions")

		// tool_choice 是強制 tool-call 機制的 wire 形狀。只宣告一個 tool 且不設
		// AllowedFunctionNames 時，openaimodel 送的是相容性最好的字串
		// "required"（不是 {"type":"allowed_tools",…}）——見 applyToolCallConfig。
		require.Equal(t, "required", capturedBody["tool_choice"])

		// reasoning.effort=none 關掉 DeepSeek flash 預設的 Thinking mode。
		// 2026-08-30 對正式 /responses 端點實測：少了這個欄位，強制 tool_choice
		// 一律回 400 "Thinking mode does not support this tool_choice"，
		// 也就是整個抽取路徑會全滅。DeepSeek 專屬，Gemma 不加。
		require.Equal(t, map[string]any{"effort": "none"}, capturedBody["reasoning"],
			"少了 reasoning.effort=none，DeepSeek 會因 Thinking mode 拒收強制 tool_choice（HTTP 400）")

		resp := map[string]any{
			"id": "resp_test", "object": "response", "created_at": 0, "status": "completed",
			"model": llmprovider.DeepSeekModelName,
			"output": []map[string]any{{
				"type": "function_call", "id": "fc_test", "call_id": "call_test",
				"name":      "extract_slot_patch",
				"status":    "completed",
				"arguments": `{"item":"寶島梨禮盒","quantity":50,"unit":"BOX"}`,
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	e := newTestExtractor(t, "deepseek", "test-key", "", srv.URL)
	result, err := e.Extract(t.Context(), "我要訂寶島梨禮盒 50 盒", domain.Slots{})
	require.NoError(t, err)
	require.Equal(t, "寶島梨禮盒", *result.Patch.Item)
	require.Equal(t, 50, *result.Patch.Quantity)
	require.Equal(t, "BOX", *result.Patch.Unit)
}

func TestNewDeepSeekSlotExtractor_UsesConfigAPIKey(t *testing.T) {
	p := llmprovider.ProcurementForTest(
		&config.Config{ExtractorModel: llmprovider.DeepSeekModelName, DeepSeekAPIKey: "from-config"}, "")
	require.True(t, p.Enabled())
	require.NotNil(t, newBaseSlotExtractor(p))
}

func TestDeepSeekSlotExtractor_Extract_Wraps5xxAsProviderUnavailable(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream overloaded"}}`))
	}))
	defer srv.Close()

	e := newTestExtractor(t, "deepseek", "test-key", "", srv.URL)
	_, err := e.Extract(t.Context(), "任何訊息", domain.Slots{})
	require.Error(t, err)

	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable),
		"5xx 是上游基礎設施問題，重送有機會成功，必須標成可重試")
	require.Equal(t, 1, calls,
		"openai-go 預設 MaxRetries=2 會對 503 自行重送；重試層只留 retryingExtractor 一層，"+
			"SDK 層必須關掉（見 llmprovider.buildProcurementLLM 的 option.WithMaxRetries(0)）")
}

func TestDeepSeekSlotExtractor_Extract_ReturnsErrorOnHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Authentication Fails, Your api key is invalid","type":"authentication_error"}}`))
	}))
	defer srv.Close()

	e := newTestExtractor(t, "deepseek", "bad-key", "", srv.URL)
	_, err := e.Extract(t.Context(), "任何訊息", domain.Slots{})
	require.Error(t, err)

	var unavailable *aidomain.ProviderUnavailableError
	require.False(t, errors.As(err, &unavailable),
		"401 是請求本身的問題（api key 失效），重送不會變好，不得標成可重試")
}

// newTestExtractor 用 llmprovider 的測試 seam 建一顆指向 httptest.Server 的
// provisioned LLM，再組出該 provider 的 extractor。
//
// 重構前每個 provider 各有一支 *WithBaseURL 建構子，那正是 client 建構被複製成
// 四份的來源之一。現在測試與正式路徑走同一份建構碼，只有 baseURL 不同——
// 「測試驗過的 wire 形狀」與「正式送出去的 wire 形狀」因此不可能分岔。
func newTestExtractor(t *testing.T, provider, apiKey, modelName, baseURL string) SlotExtractor {
	t.Helper()
	cfg := &config.Config{}
	switch provider {
	case "deepseek":
		cfg.ExtractorModel = llmprovider.DeepSeekModelName
		cfg.DeepSeekAPIKey = apiKey
	case "gemma":
		cfg.ExtractorModel = modelName
		cfg.OpenRouterAPIKey = apiKey
	}
	p := llmprovider.ProcurementForTest(cfg, baseURL)
	require.True(t, p.Enabled(), "測試 provider 必須是開通狀態")
	return newBaseSlotExtractor(p)
}
