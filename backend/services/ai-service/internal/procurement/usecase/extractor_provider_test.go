package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// unwrapRetrying 拆掉 NewSlotExtractor 外面包的 retryingExtractor，讓既有測試能
// 繼續斷言「選對了哪個 provider」——retry 行為本身由 extract_retry_test.go 覆蓋。
func unwrapRetrying(t *testing.T, e SlotExtractor) SlotExtractor {
	t.Helper()
	re, ok := e.(*retryingExtractor)
	require.True(t, ok, "NewSlotExtractor 必須回傳包了 retryingExtractor 的 extractor")
	return re.inner
}

func TestNewSlotExtractor_SelectsGeminiByDefault(t *testing.T) {
	e := NewSlotExtractor(llmprovider.NewProcurement(&config.Config{ExtractorModel: "gemini-2.5-flash", GeminiAPIKey: "k", GeminiModel: "gemini-2.5-flash"}, zap.NewNop()))
	inner := unwrapRetrying(t, e)
	require.IsType(t, &modelLLMSlotExtractor{}, inner)
	require.Equal(t, "gemini-2.5-flash", inner.(*modelLLMSlotExtractor).modelName)
}

func TestNewSlotExtractor_SelectsDeepSeek(t *testing.T) {
	e := NewSlotExtractor(llmprovider.NewProcurement(&config.Config{ExtractorModel: llmprovider.DeepSeekModelName, DeepSeekAPIKey: "k"}, zap.NewNop()))
	inner := unwrapRetrying(t, e)
	require.IsType(t, &modelLLMSlotExtractor{}, inner)
	require.Equal(t, llmprovider.DeepSeekModelName, inner.(*modelLLMSlotExtractor).modelName)
}

// backend#3329 起，缺金鑰不再回 error（那會讓整個 ai-service 起不來），
// 改成回 fail-closed 的停用實作：建構成功、**呼叫時**回 9703。
//
// 本測試取代原本的 TestNewSlotExtractor_DeepSeekRejectsEmptyAPIKey（斷言相反）。
func TestNewSlotExtractor_DeepSeekEmptyAPIKeyDegradesFailClosed(t *testing.T) {
	e := NewSlotExtractor(llmprovider.NewProcurement(&config.Config{ExtractorModel: llmprovider.DeepSeekModelName, DeepSeekAPIKey: ""}, zap.NewNop()))
	require.IsType(t, disabledSlotExtractor{}, e)

	// 🔴 陰性斷言：停用實作**不得**回「空 patch ＋ nil error」。
	// 空 patch 是抽取器的合法輸出（使用者那句話沒帶新資訊時本來就回空），
	// 呼叫端會照常續行 ⇒ 買家經歷「問什麼都沒反應」而監控上看不出 LLM 沒接上。
	_, callErr := e.Extract(context.Background(), "我要買高麗菜", domain.Slots{})
	require.Error(t, callErr, "停用實作必須在呼叫時 fail-closed")
	assertProcurementDisabled(t, callErr)
}

func TestNewSlotExtractor_SelectsGemma(t *testing.T) {
	e := NewSlotExtractor(llmprovider.NewProcurement(&config.Config{ExtractorModel: "google/gemma-4-26b-a4b-it:free", OpenRouterAPIKey: "k"}, zap.NewNop()))
	inner := unwrapRetrying(t, e)
	require.IsType(t, &modelLLMSlotExtractor{}, inner)
	require.Equal(t, "google/gemma-4-26b-a4b-it:free", inner.(*modelLLMSlotExtractor).modelName)
}

func TestNewSlotExtractor_GemmaEmptyAPIKeyDegradesFailClosed(t *testing.T) {
	e := NewSlotExtractor(llmprovider.NewProcurement(&config.Config{ExtractorModel: "google/gemma-4-26b-a4b-it:free", OpenRouterAPIKey: ""}, zap.NewNop()))
	require.IsType(t, disabledSlotExtractor{}, e)
	_, callErr := e.Extract(context.Background(), "我要買高麗菜", domain.Slots{})
	assertProcurementDisabled(t, callErr)
}

// 🔴 預設 provider（gemini）沒給金鑰 —— #3329 盤點時**漏掉的第二條 Gemini 中止路徑**。
//
// 實測坐實的舊行為：`建立 gemini model 失敗: api key is required for Google AI backend`
// ⇒ fx abort。而 EXTRACTOR_MODEL 預設就是 gemini-2.5-flash，所以一個沒設任何 AI 金鑰的
// 環境**連 ai-service 都起不來**——那正是 #3230 的形狀。
func TestNewSlotExtractor_GeminiEmptyAPIKeyDegradesFailClosed(t *testing.T) {
	e := NewSlotExtractor(llmprovider.NewProcurement(&config.Config{ExtractorModel: "gemini-2.5-flash", GeminiAPIKey: "", GeminiModel: "gemini-2.5-flash"}, zap.NewNop()))
	require.IsType(t, disabledSlotExtractor{}, e)
	_, callErr := e.Extract(context.Background(), "我要買高麗菜", domain.Slots{})
	assertProcurementDisabled(t, callErr)
}

// EXTRACTOR_MODEL 打錯字同樣降級（#3329）：在此之前一個 env 的錯字就讓整個
// 服務起不來。錯字照樣吵（ai_procurement_llm_config_incomplete=1 ＋ Error log），
// 但不再連坐 59 支 RPC。
func TestNewSlotExtractor_UnknownProviderDegradesFailClosed(t *testing.T) {
	e := NewSlotExtractor(llmprovider.NewProcurement(&config.Config{ExtractorModel: "not-a-real-model"}, zap.NewNop()))
	require.IsType(t, disabledSlotExtractor{}, e)
	_, callErr := e.Extract(context.Background(), "我要買高麗菜", domain.Slots{})
	assertProcurementDisabled(t, callErr)
}

// assertProcurementDisabled 釘住跨層契約：碼與訊息硬寫字面值，不引用被測物的常數。
func assertProcurementDisabled(t *testing.T, err error) {
	t.Helper()
	var de *pkgerrors.DomainError
	require.True(t, errors.As(err, &de), "必須是帶業務碼的 DomainError，實際 %T：%v", err, err)
	require.Equal(t, 9703, de.DomainCode())
	require.Equal(t, "AI 助理尚未開通，請稍後再試", de.DomainMessage())
}

// TestRetryingExtractor_NoRetry_AttemptIsOne／TestRetryingExtractor_OneRetry_AttemptIsTwo
// 覆蓋 retryingExtractor 新增的 attempt 追蹤：只有這一層知道重試有沒有真的發生，
// provider（parseExtractResult）不會、也不該碰 Attempt／MaxAttempts。複用既有的
// scriptedExtractor 假物件（extract_retry_test.go），不另造第二個 fake。
func TestRetryingExtractor_NoRetry_AttemptIsOne(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{
		{shortIntent: "ok"},
	}}
	r := NewRetryingExtractor(inner)
	result, err := r.Extract(context.Background(), "hello", domain.Slots{})
	require.NoError(t, err)
	require.Equal(t, 1, result.Attempt)
	require.Equal(t, 2, result.MaxAttempts)
}

func TestRetryingExtractor_OneRetry_AttemptIsTwo(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{
		{err: &aidomain.ProviderUnavailableError{Err: errors.New("timeout")}},
		{shortIntent: "ok on retry"},
	}}
	r := NewRetryingExtractor(inner)
	result, err := r.Extract(context.Background(), "hello", domain.Slots{})
	require.NoError(t, err)
	require.Equal(t, 2, result.Attempt)
	require.Equal(t, 2, result.MaxAttempts)
	require.Equal(t, "ok on retry", result.ShortIntent)
}
