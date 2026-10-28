package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func confirmed(label string) *ItemResolution {
	return &ItemResolution{Kind: ItemResolvedCategoryConfirmed, Label: label, CategoryID: "4"}
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
	before := Slots{Item: item("高麗菜"), ItemResolution: confirmed("甘藍-初秋")}

	after, rejected := before.Apply(SlotPatch{Item: item("番茄")}, time.Now())

	require.Equal(t, "番茄", *after.Item)
	require.Nil(t, after.ItemResolution, "舊解析描述的是上一個品項，留著會讓B 品項沿用 A 的分類")
	require.Empty(t, rejected)
}

func TestApply_ItemUnchanged_KeepsResolution(t *testing.T) {
	res := confirmed("甘藍-初秋")
	before := Slots{Item: item("高麗菜"), ItemResolution: res}

	after, rejected := before.Apply(SlotPatch{Quantity: ptr(50)}, time.Now())

	require.Equal(t, res, after.ItemResolution)
	require.Empty(t, rejected)
}

func TestApply_ItemPatchRejected_KeepsResolution(t *testing.T) {
	res := confirmed("甘藍-初秋")
	before := Slots{Item: item("高麗菜"), ItemResolution: res}

	after, rejected := before.Apply(SlotPatch{Item: item("   ")}, time.Now())

	require.Equal(t, "高麗菜", *after.Item)
	require.Equal(t, res, after.ItemResolution)
	require.Len(t, rejected, 1)
	require.Equal(t, string(SlotItem), rejected[0].Key)
}

func TestApply_ResolutionWithoutItem_Rejected(t *testing.T) {
	after, rejected := Slots{}.Apply(SlotPatch{ItemResolution: confirmed("甘藍-初秋")}, time.Now())

	require.Nil(t, after.ItemResolution)
	require.Len(t, rejected, 1)
	require.Equal(t, string(SlotItemResolution), rejected[0].Key)
}

func TestApply_MalformedResolution_Rejected(t *testing.T) {
	cases := []struct {
		name string
		res  ItemResolution
	}{
		{"CROP 已不存在", ItemResolution{Kind: "CROP", CategoryID: "4"}},
		{"GROUP 已不存在", ItemResolution{Kind: "GROUP", CategoryID: "4"}},
		{"CATEGORY_CONFIRMED 缺分類 ID", ItemResolution{Kind: ItemResolvedCategoryConfirmed, Label: "甘藍"}},
		{"CATEGORY_CONFIRMED 分類 ID 只有空白", ItemResolution{Kind: ItemResolvedCategoryConfirmed, CategoryID: "  "}},
		{"BUYER_DESCRIBED 缺描述", ItemResolution{Kind: ItemResolvedBuyerDescribed}},
		{"BUYER_DESCRIBED 描述只有空白", ItemResolution{Kind: ItemResolvedBuyerDescribed, Label: "  "}},
		{"BUYER_DESCRIBED 不可帶分類 ID", ItemResolution{Kind: ItemResolvedBuyerDescribed, Label: "野生黑木耳", CategoryID: "4"}},
		{"未知 kind", ItemResolution{Kind: "L3", CategoryID: "4"}},
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
		{"CATEGORY_CONFIRMED 帶標籤", ItemResolution{Kind: ItemResolvedCategoryConfirmed, Label: "甘藍", CategoryID: "4"}},
		{"CATEGORY_CONFIRMED 無標籤", ItemResolution{Kind: ItemResolvedCategoryConfirmed, CategoryID: "4"}},
		{"BUYER_DESCRIBED 帶描述、無分類 ID", ItemResolution{Kind: ItemResolvedBuyerDescribed, Label: "野生黑木耳"}},
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
	before := Slots{Item: item("星際迷航火星藜麥"), ItemResolution: confirmed("舊的")}
	res := ItemResolution{Kind: ItemResolvedCategoryConfirmed, CategoryID: "4"}

	after, rejected := before.Apply(SlotPatch{Item: item("火星藜麥"), ItemResolution: &res}, time.Now())

	require.Empty(t, rejected)
	require.Equal(t, "火星藜麥", *after.Item)
	require.Equal(t, res, *after.ItemResolution)
}

func TestRecordedThisTurn_ItemResolution(t *testing.T) {
	current := Slots{Item: item("高麗菜")}
	res := ItemResolution{Kind: ItemResolvedCategoryConfirmed, CategoryID: "4", Label: "甘藍-初秋"}
	next := Slots{Item: item("高麗菜"), ItemResolution: &res}

	got := RecordedThisTurn(SlotPatch{ItemResolution: &res}, current, next)

	require.Len(t, got, 1)
	require.Equal(t, string(SlotItemResolution), got[0].Key)
	require.Equal(t, "品項分類", got[0].Label)
	require.Equal(t, "甘藍-初秋", got[0].Value)
}

func TestRecordedThisTurn_ItemResolutionUnchanged_NotRecorded(t *testing.T) {
	res := ItemResolution{Kind: ItemResolvedCategoryConfirmed, CategoryID: "4", Label: "甘藍-初秋"}
	same := ItemResolution{Kind: ItemResolvedCategoryConfirmed, CategoryID: "4", Label: "甘藍-初秋"}
	current := Slots{Item: item("高麗菜"), ItemResolution: &res}
	next := Slots{Item: item("高麗菜"), ItemResolution: &same}

	require.Empty(t, RecordedThisTurn(SlotPatch{ItemResolution: &same}, current, next))
}

func TestRecordedThisTurn_ItemResolutionOrder(t *testing.T) {
	res := ItemResolution{Kind: ItemResolvedCategoryConfirmed, CategoryID: "4", Label: "甘藍"}
	next := Slots{Item: item("高麗菜"), ItemResolution: &res, Quantity: ptr(100)}

	got := RecordedThisTurn(SlotPatch{Item: item("高麗菜"), ItemResolution: &res, Quantity: ptr(100)}, Slots{}, next)

	require.Len(t, got, 3)
	require.Equal(t, string(SlotItem), got[0].Key)
	require.Equal(t, string(SlotItemResolution), got[1].Key)
	require.Equal(t, string(SlotQuantity), got[2].Key)
}

func TestItemResolution_DisplayValue(t *testing.T) {
	require.Equal(t, "甘藍", ItemResolution{Kind: ItemResolvedCategoryConfirmed, CategoryID: "4", Label: " 甘藍 "}.DisplayValue())
}

func TestNewCategoryConfirmedItemResolution(t *testing.T) {
	got := NewCategoryConfirmedItemResolution(&CropSelection{Stage: CropSelectionDescribe, CategoryID: "24", Label: "豬肉／五花肉", ParentCategoryID: "8"})
	require.Equal(t, &ItemResolution{Kind: ItemResolvedCategoryConfirmed, Label: "豬肉／五花肉", CategoryID: "24"}, got)
	require.Empty(t, validateItemResolution(*got))

	require.Nil(t, NewCategoryConfirmedItemResolution(nil))
	require.Nil(t, NewCategoryConfirmedItemResolution(&CropSelection{Stage: CropSelectionDescribe}))
	require.Nil(t, NewCategoryConfirmedItemResolution(&CropSelection{Stage: CropSelectionDescribe, ParentCategoryID: "8", ParentLabel: "豬肉"}),
		"只確認到大分類、細項沒定時不產生解析結果")
}

func TestSlots_ItemResolution_NotRequired(t *testing.T) {
	require.False(t, IsRequiredSlot(SlotItemResolution))
	require.NotContains(t, Slots{}.MissingRequired(), SlotItemResolution)
}
