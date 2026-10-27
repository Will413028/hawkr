package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestNewGeminiSlotExtractor_BuildsSuccessfully(t *testing.T) {
	p := llmprovider.ProcurementForTest(
		&config.Config{ExtractorModel: "gemini-2.5-flash", GeminiAPIKey: "test-key", GeminiModel: "gemini-2.5-flash"}, "")
	require.True(t, p.Enabled())
	require.NotNil(t, newBaseSlotExtractor(p))
}

func TestBuildSlotSchema_CoversAllSlotKeysAndClosedEnums(t *testing.T) {
	s := buildSlotSchema()
	require.NotNil(t, s.Properties)

	for _, k := range []domain.SlotKey{
		domain.SlotItem, domain.SlotQuantity, domain.SlotUnit,
		domain.SlotPackaging, domain.SlotBudgetPerUnit, domain.SlotDeliveryCity,
		domain.SlotDeliveryDate, domain.SlotExactMatchRequired,
	} {
		require.Contains(t, s.Properties, string(k), "schema 必須涵蓋每個 slot key")
	}

	// 單位是封閉集合，schema 層就要擋住模型自創單位。
	require.ElementsMatch(t, domain.AllowedUnits, s.Properties[string(domain.SlotUnit)].Enum)

	// 沒有必填：抽不到就留空，不逼模型硬填。
	require.Empty(t, s.Required)
}

func TestBuildSlotSchema_IncludesShortIntent(t *testing.T) {
	schema := buildSlotSchema()
	prop, ok := schema.Properties["shortIntent"]
	require.True(t, ok, "buildSlotSchema() must declare a shortIntent property")
	require.Equal(t, genai.TypeString, prop.Type)
}

func TestParseExtractResult_ParsesShortIntentAlongsideSlots(t *testing.T) {
	raw := []byte(`{"item":"香蕉","shortIntent":"正在讀你的需求，抓品項、數量和交期"}`)
	result, err := parseExtractResult(raw)
	require.NoError(t, err)
	require.NotNil(t, result.Patch.Item)
	require.Equal(t, "香蕉", *result.Patch.Item)
	require.Equal(t, "正在讀你的需求，抓品項、數量和交期", result.ShortIntent)
}

func TestParseExtractResult_MissingShortIntent_EmptyString(t *testing.T) {
	raw := []byte(`{"item":"香蕉"}`)
	result, err := parseExtractResult(raw)
	require.NoError(t, err)
	require.Equal(t, "", result.ShortIntent)
}

func TestParseExtractResult_MapsFieldsAndIgnoresUnknownKeys(t *testing.T) {
	raw := []byte(`{
		"item": "寶島梨禮盒",
		"quantity": 50,
		"unit": "KG",
		"budget_per_unit": 800,
		"delivery_city": "桃園市",
		"delivery_date": "2026-08-20",
		"totally_unknown_key": "should be ignored"
	}`)

	result, err := parseExtractResult(raw)
	require.NoError(t, err)
	require.Equal(t, "寶島梨禮盒", *result.Patch.Item)
	require.Equal(t, 50, *result.Patch.Quantity)
	require.Equal(t, "KG", *result.Patch.Unit)
	require.Equal(t, 800, *result.Patch.BudgetPerUnit)
	require.Equal(t, "桃園市", *result.Patch.DeliveryCity)
	require.Equal(t, "2026-08-20", *result.Patch.DeliveryDate)
}

func TestParseExtractResult_RejectsMalformedJSON(t *testing.T) {
	_, err := parseExtractResult([]byte(`not json at all`))
	require.Error(t, err)
}

func TestParseExtractResult_EmptyObjectYieldsEmptyPatch(t *testing.T) {
	result, err := parseExtractResult([]byte(`{}`))
	require.NoError(t, err)
	require.Nil(t, result.Patch.Item)
	require.Nil(t, result.Patch.Quantity)
}

func TestClassifyGeminiExtractErr_WrapsDeadlineExceededAsProviderUnavailable(t *testing.T) {
	err := classifyGeminiExtractErr(context.DeadlineExceeded)

	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable),
		"逾時是基礎設施問題，重送有機會成功，必須標成可重試")
}

func TestClassifyGeminiExtractErr_WrapsWrappedDeadlineExceededAsProviderUnavailable(t *testing.T) {
	err := classifyGeminiExtractErr(fmt.Errorf("slot 抽取失敗: %w", context.DeadlineExceeded))

	var unavailable *aidomain.ProviderUnavailableError
	require.True(t, errors.As(err, &unavailable),
		"逾時錯誤可能被上層再包一層，errors.Is 要能穿透 wrap 鏈判斷")
}

func TestClassifyGeminiExtractErr_LeavesOtherErrorsUnwrapped(t *testing.T) {
	original := errors.New("slot 抽取失敗: 400 invalid argument")
	err := classifyGeminiExtractErr(original)

	require.Same(t, original, err, "非逾時錯誤（Gemini SDK 未暴露結構化狀態碼，保守不重試）原樣放行")

	var unavailable *aidomain.ProviderUnavailableError
	require.False(t, errors.As(err, &unavailable))
}

func TestClassifyGeminiExtractErr_NilIsNil(t *testing.T) {
	require.NoError(t, classifyGeminiExtractErr(nil))
}

// gateway/internal/proxy/ai_assistant_stream.go 的 streamTurnDeadline 是 120s；
// 首次加重試必須先於它觸發，否則 extract 卡住時買家看到的是斷線而不是降級。
func TestExtractTimeout_FirstPlusRetryStaysUnderHalfGatewayDeadline(t *testing.T) {
	const gatewayStreamTurnDeadline = 120 * time.Second
	require.LessOrEqual(t, ExtractTimeout+RetryTimeout, gatewayStreamTurnDeadline/2)
}
