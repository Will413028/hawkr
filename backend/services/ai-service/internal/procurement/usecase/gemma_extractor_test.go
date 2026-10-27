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

func TestGemmaSlotExtractor_Extract_SendsToolCallRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/responses", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// 模型名不能再由測試任意指定：EXTRACTOR_MODEL 的值域封閉在 llmprovider
		// 的登記表，能送上 wire 的就只有表裡那些。這正是這個設計要的——
		// 測試也不例外，否則測試驗過的 wire 形狀與正式送出去的又會分岔。
		require.Equal(t, gemmaTestModel, body["model"])

		// instructions／tool_choice 的釘法與理由同 deepseek_extractor_test.go：
		// 抽取規則整套掛在 instructions，強制 tool-call 機制的 wire 形狀是
		// tool_choice。OpenRouter 這條路徑也必須送出同樣的兩個欄位。
		instructions, ok := body["instructions"].(string)
		require.True(t, ok, "請求應把 SystemInstruction 送成 instructions 字串")
		require.NotEmpty(t, instructions)
		require.Equal(t, extractInstruction, instructions)
		require.Contains(t, instructions, strings.Join(domain.AllowedUnits, "/"),
			"單位允許清單必須真的進到 wire 上的 instructions")
		require.Equal(t, "required", body["tool_choice"])

		resp := map[string]any{
			"id": "resp_test", "object": "response", "created_at": 0, "status": "completed",
			"model": "gemma-test-model",
			"output": []map[string]any{{
				"type": "function_call", "id": "fc_test", "call_id": "call_test",
				"name": "extract_slot_patch", "status": "completed",
				// 買家只提到 item，其餘欄位（含 quantity）不應出現在 arguments——
				// 這是 §4 要求的「模糊句不硬填」回歸驗證：tool-call 機制下模型
				// 對沒提到的欄位應該省略 key，不是塞入猜測值。
				"arguments": `{"item":"寶島梨禮盒"}`,
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	e := newTestExtractor(t, "gemma", "test-key", gemmaTestModel, srv.URL)
	result, err := e.Extract(t.Context(), "我要訂寶島梨禮盒", domain.Slots{})
	require.NoError(t, err)
	require.Equal(t, "寶島梨禮盒", *result.Patch.Item)
	require.Nil(t, result.Patch.Quantity, "沒提到的欄位必須是 nil（未填），不是模型硬填的猜測值")
}

func TestNewGemmaSlotExtractor_UsesConfigValues(t *testing.T) {
	p := llmprovider.ProcurementForTest(
		&config.Config{ExtractorModel: "google/gemma-4-26b-a4b-it:free", OpenRouterAPIKey: "from-config"}, "")
	require.True(t, p.Enabled())
	require.Equal(t, "google/gemma-4-26b-a4b-it:free", p.ModelName())
	require.NotNil(t, newBaseSlotExtractor(p))
}

// 錯誤分類比照 Task 4（deepseek_extractor_test.go）的 5xx／401 案例——判斷邏輯
// 共用同一個 classifyOpenAICompatExtractErr（model_llm_extractor.go），不必每個
// 狀態碼都重複整套，只驗證 Gemma（經 OpenRouter）這條路徑接得上。
func TestGemmaSlotExtractor_Extract_Wraps5xxAsProviderUnavailable(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream overloaded"}}`))
	}))
	defer srv.Close()

	e := newTestExtractor(t, "gemma", "test-key", gemmaTestModel, srv.URL)
	_, err := e.Extract(t.Context(), "任何訊息", domain.Slots{})
	require.Error(t, err)

	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable),
		"5xx 是上游基礎設施問題，重送有機會成功，必須標成可重試")
	require.Equal(t, 1, calls,
		"SDK 層重試必須關掉，理由同 deepseek_extractor_test.go")
}

func TestGemmaSlotExtractor_Extract_ReturnsErrorOnHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"No auth credentials found","code":401}}`))
	}))
	defer srv.Close()

	e := newTestExtractor(t, "gemma", "bad-key", gemmaTestModel, srv.URL)
	_, err := e.Extract(t.Context(), "任何訊息", domain.Slots{})
	require.Error(t, err)

	var unavailable *aidomain.ProviderUnavailableError
	require.False(t, errors.As(err, &unavailable),
		"401 是請求本身的問題（api key 失效），重送不會變好，不得標成可重試")
}

// gemmaTestModel 是登記表裡的 Gemma 模型名。測試引用它而不是寫死字面值，
// 這樣改名時是編譯期或單一處失敗，不是散在四支測試裡各紅一次。
const gemmaTestModel = "google/gemma-4-26b-a4b-it:free"
