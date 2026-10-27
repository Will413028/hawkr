package cropidentity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
)

func gateResolutionFor(t *testing.T, uc *DisambiguationUseCase, item string) *domain.ItemResolution {
	t.Helper()
	before := domain.Slots{}
	after := domain.Slots{Item: &item}
	got := ComputeGate(context.Background(), uc, before, after)
	require.True(t, got.Evaluated(), "%+v", got)
	return got.Resolution
}

func TestDirectCropResolutionCarriesItsCategory(t *testing.T) {
	pb := &perVariantPricebookFake{byVariant: map[string][]pbusecase.PriceableEntityMatch{
		"甘藍-初秋": {pm("CROP", "LA1", "甘藍-初秋", "甘藍-初秋", "CANONICAL", 0, 1, []string{"1242"}, "G-CAB", true)},
	}}
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, nil)
	uc.pricebook = pb

	got := gateResolutionFor(t, uc, "甘藍-初秋")

	require.NotNil(t, got)
	require.Equal(t, "1242", got.CategoryID,
		"精確品名一輪定案、不出卡片時分類也要帶著。dev 實測（2026-09-19）：「甘藍-初秋 20 公斤 送台中市」"+
			"category_id 是空的，候選搜尋回 0 平台 0 站外——愈精確的買家拿到愈差的結果")
}

func TestDirectGroupResolutionCarriesItsCategory(t *testing.T) {
	pb := &perVariantPricebookFake{byVariant: map[string][]pbusecase.PriceableEntityMatch{
		"甘藍": {pm("CROP_GROUP", "G-CAB", "甘藍", "甘藍", "CANONICAL", 0, 1, []string{"1242"}, "G-CAB", true)},
	}}
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), nil, nil)
	uc.pricebook = pb

	got := gateResolutionFor(t, uc, "甘藍")

	require.NotNil(t, got)
	require.Equal(t, "1242", got.CategoryID,
		"大類底下湊不出兩個子品項時會直接定案成 GROUP，那條路同樣要帶分類")
}

func TestDirectCommodityResolutionCarriesItsCategory(t *testing.T) {
	pb := &perVariantPricebookFake{byVariant: map[string][]pbusecase.PriceableEntityMatch{
		"毛豬（規格豬）": {pm("COMMODITY", "12", "毛豬（規格豬）", "毛豬（規格豬）", "CANONICAL", 0, 1, []string{"2008"}, "", true)},
	}}
	uc := v2UC(&categoryPricebookFake{}, v2CategoryTree(), commodityMapping("2008", "毛豬（規格豬）", "pt_pig"), nil)
	uc.pricebook = pb

	got := gateResolutionFor(t, uc, "毛豬（規格豬）")

	require.NotNil(t, got)
	require.Equal(t, "2008", got.CategoryID,
		"COMMODITY 走 resolveCategory 建 Selection，這條在 dev 上本來就對；列進來是為了讓 recall.Kind 的值域被完整覆蓋，"+
			"少一個 Kind 就是少一條會靜默掉分類的路")
}
