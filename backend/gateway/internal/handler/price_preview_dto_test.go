package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

func TestRfqPricePreviewResponseFromProto_UnsetBuyerRangeSerializesAsNull(t *testing.T) {
	dto := rfqPricePreviewResponseFromProto(&pb.RfqPricePreviewResponse{
		PricePreviewId: "1", Version: "1",
	})

	raw, err := json.Marshal(dto)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	v, ok := out["buyerRequestedRange"]
	require.True(t, ok, "鍵仍要在，與 quoteTotalRange 同慣例")
	require.Nil(t, v,
		"買家略過預算時要送 null，不是一個全空字串的物件——後者會讓前端 safeParse 失敗"+
			"（currency 期望 \"TWD\"、priceComponent 期望 \"GOODS_ONLY\"、金額期望正數），"+
			"買家卡在「目前無法完成價格確認」（2026-09-19 實測）")
}

func TestRfqPricePreviewResponseFromProto_SetBuyerRangeStillSerializes(t *testing.T) {
	dto := rfqPricePreviewResponseFromProto(&pb.RfqPricePreviewResponse{
		PricePreviewId: "1", Version: "1",
		BuyerRequestedRange: &pb.BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: "90", UpperBoundAmount: "120",
		},
	})

	require.NotNil(t, dto.BuyerRequestedRange)
	require.Equal(t, "TWD", dto.BuyerRequestedRange.Currency)
	require.Equal(t, "90", dto.BuyerRequestedRange.LowerBoundAmount)
}
