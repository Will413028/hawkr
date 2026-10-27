package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func cropRes(code, label string) *ItemResolution {
	return &ItemResolution{Kind: ItemResolvedCrop, Code: &code, Label: label, Provenance: ItemResolutionBySystem}
}

func TestSlots_LegacyJSON_StillDecodes(t *testing.T) {
	var s Slots
	require.NoError(t, json.Unmarshal([]byte(`{"item":"高麗菜","quantity":100,"unit":"KG"}`), &s))

	require.NotNil(t, s.Item)
	require.Equal(t, "高麗菜", *s.Item)
	require.Nil(t, s.ItemResolution, "舊 revision 沒有解析結果，不能憑空生出一個")
	require.Equal(t, 100, *s.Quantity)
}

func TestSlots_WithoutResolution_MarshalsIdenticallyToLegacy(t *testing.T) {
	b, err := json.Marshal(Slots{Item: item("高麗菜"), Quantity: ptr(100)})
	require.NoError(t, err)
	require.JSONEq(t, `{"item":"高麗菜","quantity":100}`, string(b))
	require.NotContains(t, string(b), "item_resolution")
}

func TestApply_ItemChanged_ClearsStaleResolution(t *testing.T) {
	before := Slots{Item: item("高麗菜"), ItemResolution: cropRes("LA1", "甘藍-初秋")}

	after, rejected := before.Apply(SlotPatch{Item: item("番茄")}, time.Now())

	require.Equal(t, "番茄", *after.Item)
	require.Nil(t, after.ItemResolution, "舊解析描述的是上一個品項，留著會讓買家拿 A 的行情對 B 報價")
	require.Empty(t, rejected)
}

func TestApply_ItemUnchanged_KeepsResolution(t *testing.T) {
	res := cropRes("LA1", "甘藍-初秋")
	before := Slots{Item: item("高麗菜"), ItemResolution: res}

	after, rejected := before.Apply(SlotPatch{Quantity: ptr(50)}, time.Now())

	require.Equal(t, res, after.ItemResolution)
	require.Empty(t, rejected)
}

func TestApply_ItemPatchRejected_KeepsResolution(t *testing.T) {
	res := cropRes("LA1", "甘藍-初秋")
	before := Slots{Item: item("高麗菜"), ItemResolution: res}

	after, rejected := before.Apply(SlotPatch{Item: item("   ")}, time.Now())

	require.Equal(t, "高麗菜", *after.Item)
	require.Equal(t, res, after.ItemResolution)
	require.Len(t, rejected, 1)
	require.Equal(t, string(SlotItem), rejected[0].Key)
}

func TestApply_ResolutionWithoutItem_Rejected(t *testing.T) {
	after, rejected := Slots{}.Apply(SlotPatch{ItemResolution: cropRes("LA1", "甘藍-初秋")}, time.Now())

	require.Nil(t, after.ItemResolution)
	require.Len(t, rejected, 1)
	require.Equal(t, string(SlotItemResolution), rejected[0].Key)
}

func TestApply_MalformedResolution_Rejected(t *testing.T) {
	cases := []struct {
		name string
		res  ItemResolution
	}{
		{"CROP 缺 code", ItemResolution{Kind: ItemResolvedCrop, Provenance: ItemResolutionBySystem}},
		{"GROUP 的 code 是空白", ItemResolution{Kind: ItemResolvedGroup, Code: ptr("  "), Provenance: ItemResolutionBySystem}},
		{"CROP 帶單價", ItemResolution{Kind: ItemResolvedCrop, Code: ptr("LA1"), PricePerKg: ptr(75), Provenance: ItemResolutionBySystem}},
		{"SELF_PRICED 帶 code", ItemResolution{Kind: ItemResolvedSelfPriced, Code: ptr("LA1"), PricePerKg: ptr(75), Provenance: ItemResolutionByBuyerChoice}},
		{"SELF_PRICED 缺單價", ItemResolution{Kind: ItemResolvedSelfPriced, Provenance: ItemResolutionByBuyerChoice}},
		{"SELF_PRICED 單價為 0", ItemResolution{Kind: ItemResolvedSelfPriced, PricePerKg: ptr(0), Provenance: ItemResolutionByBuyerChoice}},
		{"SELF_PRICED 單價超過上界", ItemResolution{Kind: ItemResolvedSelfPriced, PricePerKg: ptr(MaxPricePerKg + 1), Provenance: ItemResolutionByBuyerChoice}},
		{"未知 kind", ItemResolution{Kind: "L3", Code: ptr("X"), Provenance: ItemResolutionBySystem}},
		{"未知 provenance", ItemResolution{Kind: ItemResolvedCrop, Code: ptr("LA1"), Provenance: "GUESS"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.res
			after, rejected := Slots{Item: item("高麗菜")}.Apply(
				SlotPatch{ItemResolution: &res}, time.Now())

			require.Nil(t, after.ItemResolution, "不合格的解析結果不得落地")
			require.Len(t, rejected, 1)
			require.Equal(t, string(SlotItemResolution), rejected[0].Key)
			require.NotEmpty(t, rejected[0].Reason)
		})
	}
}

func TestApply_ValidResolutions_Accepted(t *testing.T) {
	cases := []struct {
		name string
		res  ItemResolution
	}{
		{"CROP", ItemResolution{Kind: ItemResolvedCrop, Code: ptr("LA1"), Label: "甘藍-初秋", FallbackGroupCode: ptr("LA"), Provenance: ItemResolutionBySystem}},
		{"GROUP", ItemResolution{Kind: ItemResolvedGroup, Code: ptr("LA"), Label: "甘藍", Provenance: ItemResolutionByBuyerChoice}},
		{"SELF_PRICED", ItemResolution{Kind: ItemResolvedSelfPriced, PricePerKg: ptr(75), Provenance: ItemResolutionByBuyerChoice}},
		{"SELF_PRICED 剛好在上界", ItemResolution{Kind: ItemResolvedSelfPriced, PricePerKg: ptr(MaxPricePerKg), Provenance: ItemResolutionByBuyerChoice}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.res
			after, rejected := Slots{Item: item("高麗菜")}.Apply(
				SlotPatch{ItemResolution: &res}, time.Now())

			require.Empty(t, rejected)
			require.NotNil(t, after.ItemResolution)
			require.Equal(t, tc.res, *after.ItemResolution)
		})
	}
}

func TestApply_ItemAndResolutionInSamePatch(t *testing.T) {
	before := Slots{Item: item("星際迷航火星藜麥"), ItemResolution: cropRes("OLD", "舊的")}
	res := ItemResolution{Kind: ItemResolvedSelfPriced, PricePerKg: ptr(75), Provenance: ItemResolutionByBuyerChoice}

	after, rejected := before.Apply(SlotPatch{Item: item("火星藜麥"), ItemResolution: &res}, time.Now())

	require.Empty(t, rejected)
	require.Equal(t, "火星藜麥", *after.Item)
	require.Equal(t, res, *after.ItemResolution)
}

func TestRecordedThisTurn_ItemResolution(t *testing.T) {
	current := Slots{Item: item("高麗菜")}
	res := ItemResolution{Kind: ItemResolvedCrop, Code: ptr("LA1"), Label: "甘藍-初秋", Provenance: ItemResolutionByBuyerChoice}
	next := Slots{Item: item("高麗菜"), ItemResolution: &res}

	got := RecordedThisTurn(SlotPatch{ItemResolution: &res}, current, next)

	require.Len(t, got, 1)
	require.Equal(t, string(SlotItemResolution), got[0].Key)
	require.Equal(t, "品項分類", got[0].Label)
	require.Equal(t, "甘藍-初秋", got[0].Value)
}

func TestRecordedThisTurn_ItemResolutionUnchanged_NotRecorded(t *testing.T) {
	res := ItemResolution{Kind: ItemResolvedCrop, Code: ptr("LA1"), Label: "甘藍-初秋", Provenance: ItemResolutionBySystem}
	same := ItemResolution{Kind: ItemResolvedCrop, Code: ptr("LA1"), Label: "甘藍-初秋", Provenance: ItemResolutionBySystem}
	current := Slots{Item: item("高麗菜"), ItemResolution: &res}
	next := Slots{Item: item("高麗菜"), ItemResolution: &same}

	require.Empty(t, RecordedThisTurn(SlotPatch{ItemResolution: &same}, current, next))
}

func TestRecordedThisTurn_ItemResolutionOrder(t *testing.T) {
	res := ItemResolution{Kind: ItemResolvedGroup, Code: ptr("LA"), Label: "甘藍", Provenance: ItemResolutionBySystem}
	next := Slots{Item: item("高麗菜"), ItemResolution: &res, Quantity: ptr(100)}

	got := RecordedThisTurn(SlotPatch{Item: item("高麗菜"), ItemResolution: &res, Quantity: ptr(100)}, Slots{}, next)

	require.Len(t, got, 3)
	require.Equal(t, string(SlotItem), got[0].Key)
	require.Equal(t, string(SlotItemResolution), got[1].Key)
	require.Equal(t, string(SlotQuantity), got[2].Key)
}

func TestItemResolution_DisplayValue(t *testing.T) {
	require.Equal(t, "甘藍-初秋", ItemResolution{Kind: ItemResolvedCrop, Code: ptr("LA1"), Label: "甘藍-初秋"}.DisplayValue())
	require.Equal(t, "甘藍（整個大類）", ItemResolution{Kind: ItemResolvedGroup, Code: ptr("LA"), Label: "甘藍"}.DisplayValue())
	require.Equal(t, "自訂品項，單價 75 元／公斤", ItemResolution{Kind: ItemResolvedSelfPriced, PricePerKg: ptr(75)}.DisplayValue())
	require.Equal(t, "LA1", ItemResolution{Kind: ItemResolvedCrop, Code: ptr("LA1")}.DisplayValue(),
		"沒有 label 時退回 code，不能顯示空字串")
}

func TestSlots_ItemResolution_NotRequired(t *testing.T) {
	require.False(t, IsRequiredSlot(SlotItemResolution))
	require.NotContains(t, Slots{}.MissingRequired(), SlotItemResolution)
}

func TestNewBuyerChoiceItemResolution_GroupOption_UsesResolvedLabelNotButtonText(t *testing.T) {
	code := "moa-group:f102791d6f6f4949"
	got := NewBuyerChoiceItemResolution(CropDisambiguationOption{
		Label: "整個甘藍都可以", ResolvedLabel: "甘藍",
		GroupCode: &code, FallbackGroupCode: &code,
	})

	require.NotNil(t, got)
	require.Equal(t, ItemResolvedGroup, got.Kind)
	require.Equal(t, "甘藍", got.Label)
	require.Equal(t, ItemResolutionByBuyerChoice, got.Provenance)
	require.Equal(t, "甘藍（整個大類）", got.DisplayValue(),
		"確認段的 chip 內容——後綴標示粒度，label 不再自己也講一遍")
}

func TestNewBuyerChoiceItemResolution_CropOption_FallsBackToLabel(t *testing.T) {
	crop, group := "LA1", "moa-group:f102791d6f6f4949"
	got := NewBuyerChoiceItemResolution(CropDisambiguationOption{
		Label: "甘藍-初秋", CropCode: &crop, FallbackGroupCode: &group,
	})

	require.NotNil(t, got)
	require.Equal(t, "甘藍-初秋", got.Label)
	require.Equal(t, "甘藍-初秋", got.DisplayValue(), "CROP 不補後綴")
}

func TestNewSystemItemResolution_CommodityCode_ProducesValidCommodityKind(t *testing.T) {
	code := "958000000000000001"
	got := NewSystemItemResolution(nil, nil, &code, nil, "毛豬（規格豬）", CategoryAbsent("測試"))
	if got == nil {
		t.Fatal("commodityCode 非 nil 時必須組出 ItemResolution")
	}
	if got.Kind != ItemResolvedCommodity {
		t.Errorf("Kind = %q，預期 %q", got.Kind, ItemResolvedCommodity)
	}
	if got.Code == nil || *got.Code != code {
		t.Errorf("Code = %v，預期 %q", got.Code, code)
	}
	if got.FallbackGroupCode != nil {
		t.Errorf("commodity 不該有 FallbackGroupCode，得到 %q", *got.FallbackGroupCode)
	}
	if msg := validateItemResolution(*got); msg != "" {
		t.Errorf("commodity 解析應通過驗證，被拒理由：%s", msg)
	}
	if got.PricebookCommodityCode() == nil {
		t.Error("PricebookCommodityCode 必須回得到碼，否則 previewCommodityPrice 永遠走不到")
	}
	if c, g := got.PricebookQueryCodes(); c != nil || g != nil {
		t.Errorf("commodity 不該回 crop/group 查詢碼，得到 %v/%v", c, g)
	}
}

func TestNewSystemItemResolution_CropWinsOverCommodity(t *testing.T) {
	crop, commodity := "CROP-1", "958000000000000001"
	got := NewSystemItemResolution(&crop, nil, &commodity, nil, "高麗菜", CategoryAbsent("測試"))
	if got.Kind != ItemResolvedCrop {
		t.Errorf("Kind = %q，預期 %q", got.Kind, ItemResolvedCrop)
	}
}

func TestValidateItemResolution_CommodityWithoutCodeRejected(t *testing.T) {
	if msg := validateItemResolution(ItemResolution{
		Kind: ItemResolvedCommodity, Provenance: ItemResolutionBySystem,
	}); msg == "" {
		t.Error("沒有 code 的 commodity 解析必須被拒")
	}
}

func TestNewSystemItemResolution_CarriesConfirmedCategory(t *testing.T) {
	sel := &CropSelection{Stage: CropSelectionPrice, CategoryID: "2008", Label: "豬肉", ParentCategoryID: "198", ParentLabel: "豬肉"}
	code := "SpecPig_AvgPrice"

	got := NewSystemItemResolution(nil, nil, &code, nil, "毛豬（規格豬）", CategoryFromSelection(sel))

	require.NotNil(t, got)
	require.Equal(t, ItemResolvedCommodity, got.Kind)
	require.Equal(t, "2008", got.CategoryID, "下游要靠它找同分類的供應商")
}

func TestNewSystemItemResolution_NilSelectionLeavesCategoryEmpty(t *testing.T) {
	crop := "LA1"
	group := "N04-001"

	got := NewSystemItemResolution(&crop, nil, nil, &group, "甘藍-初秋", CategoryAbsent("測試"))

	require.NotNil(t, got)
	require.Empty(t, got.CategoryID, "crop 走 pricebook 不經分類樹，沒有分類 id 是正常的")
}

func TestNewBuyerChoiceItemResolution_CropOption_CarriesEntityCategoryID(t *testing.T) {
	crop, group := "LA1", "moa-group:f102791d6f6f4949"
	got := NewBuyerChoiceItemResolution(CropDisambiguationOption{
		Label: "甘藍-初秋", CropCode: &crop, FallbackGroupCode: &group,
		EntityCategoryID: "533",
	})

	require.NotNil(t, got)
	require.Equal(t, "533", got.CategoryID,
		"從作物消歧卡定案的品項也要帶分類 id，站外業者比對才拿得到結構化訊號")
}

func TestNewBuyerChoiceItemResolution_GroupOption_CarriesEntityCategoryID(t *testing.T) {
	group := "moa-group:f102791d6f6f4949"
	got := NewBuyerChoiceItemResolution(CropDisambiguationOption{
		Label: "整個甘藍都可以", ResolvedLabel: "甘藍",
		GroupCode: &group, FallbackGroupCode: &group,
		EntityCategoryID: "533",
	})

	require.NotNil(t, got)
	require.Equal(t, "533", got.CategoryID, "選「整個大類」同樣要帶分類 id")
}
