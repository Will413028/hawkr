package grpc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
)

func TestToRfqPricePreviewResponseProto_UnsetBuyerRangeIsAbsentOnTheWire(t *testing.T) {
	got := toRfqPricePreviewResponseProto(pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1,
	})

	require.Nil(t, got.GetBuyerRequestedRange(),
		"買家略過預算時整個欄位要缺席，不能送一個全空字串的物件——前端 schema 要求 currency=\"TWD\"、"+
			"priceComponent=\"GOODS_ONLY\" 與正數金額，空物件會讓 safeParse 失敗，買家卡在"+
			"「目前無法完成價格確認」（2026-09-19 實測）")
}

func TestToRfqPricePreviewResponseProto_SetBuyerRangeStillTravels(t *testing.T) {
	got := toRfqPricePreviewResponseProto(pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1,
		BuyerRequestedRange: pricing.BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: "90", UpperBoundAmount: "120",
			BasisQuantity: "1", BasisUnit: "KG",
		},
	})

	require.NotNil(t, got.GetBuyerRequestedRange())
	require.Equal(t, "90", got.GetBuyerRequestedRange().GetLowerBoundAmount())
	require.Equal(t, "TWD", got.GetBuyerRequestedRange().GetCurrency())
}
