package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

var refNow = time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

func TestSlots_Apply_AcceptsValidPatch(t *testing.T) {
	got, rejected := Slots{}.Apply(SlotPatch{
		Item:          ptr("寶島梨禮盒"),
		Quantity:      ptr(50),
		Unit:          ptr("KG"),
		BudgetPerUnit: ptr(800),
		DeliveryCity:  ptr("桃園市"),
		DeliveryDate:  ptr("2026-08-20"),
	}, refNow)

	require.Empty(t, rejected)
	require.Equal(t, "寶島梨禮盒", *got.Item)
	require.Equal(t, 50, *got.Quantity)
	require.Equal(t, "KG", *got.Unit)
	require.Equal(t, 800, *got.BudgetPerUnit)
	require.Equal(t, "桃園市", *got.DeliveryCity)
	require.Equal(t, "2026-08-20", *got.DeliveryDate)
	require.True(t, *got.ExactMatchRequired, "未指定時預設為 true")
}

func TestSlots_Apply_DropsInvalidFieldsButKeepsValidOnes(t *testing.T) {
	// 單一欄位不合法不得讓整包 patch 失敗——否則 LLM 抽錯一個欄位，
	// 使用者說對的其他欄位也會一起消失。
	got, rejected := Slots{}.Apply(SlotPatch{
		Item:         ptr("寶島梨禮盒"),
		Quantity:     ptr(0),            // 不合法：下限 1
		Unit:         ptr("箱子"),         // 不合法：不在 enum
		DeliveryCity: ptr("不存在市"),       // 不合法：不在縣市 allowlist
		DeliveryDate: ptr("2026-08-01"), // 不合法：早於 now
	}, refNow)

	require.Equal(t, "寶島梨禮盒", *got.Item)
	require.Nil(t, got.Quantity)
	require.Nil(t, got.Unit)
	require.Nil(t, got.DeliveryCity)
	require.Nil(t, got.DeliveryDate)

	keys := make([]string, len(rejected))
	for i, r := range rejected {
		keys[i] = r.Key
	}
	require.ElementsMatch(t, []string{"quantity", "unit", "delivery_city", "delivery_date"}, keys)
	for _, r := range rejected {
		require.NotEmpty(t, r.Reason, "每個被丟棄的欄位都要說得出原因")
	}
}

func TestSlots_Apply_TruncatesOverlongText(t *testing.T) {
	long := ""
	for i := 0; i < MaxTextLen+50; i++ {
		long += "梨"
	}
	got, rejected := Slots{}.Apply(SlotPatch{Item: ptr(long)}, refNow)

	require.Len(t, []rune(*got.Item), MaxTextLen, "以 rune 計長，中文不得被切壞")
	require.Len(t, rejected, 1)
	require.Equal(t, "item", rejected[0].Key)
	require.Contains(t, rejected[0].Reason, "截斷")
}

func TestSlots_Apply_RejectsDateTooFarOut(t *testing.T) {
	_, rejected := Slots{}.Apply(SlotPatch{DeliveryDate: ptr("2028-01-01")}, refNow)
	require.Len(t, rejected, 1)
	require.Equal(t, "delivery_date", rejected[0].Key)
}

func TestSlots_Apply_PreservesUntouchedFields(t *testing.T) {
	base, _ := Slots{}.Apply(SlotPatch{Item: ptr("寶島梨禮盒"), Quantity: ptr(50)}, refNow)
	got, rejected := base.Apply(SlotPatch{Quantity: ptr(80)}, refNow)

	require.Empty(t, rejected)
	require.Equal(t, "寶島梨禮盒", *got.Item, "沒被 patch 到的欄位必須原封不動")
	require.Equal(t, 80, *got.Quantity)
}

func TestSlots_Hash_StableAndSensitive(t *testing.T) {
	a, _ := Slots{}.Apply(SlotPatch{Item: ptr("梨"), Quantity: ptr(50)}, refNow)
	b, _ := Slots{}.Apply(SlotPatch{Quantity: ptr(50), Item: ptr("梨")}, refNow)
	c, _ := Slots{}.Apply(SlotPatch{Item: ptr("梨"), Quantity: ptr(51)}, refNow)

	require.Equal(t, a.Hash(), b.Hash(), "同內容不同賦值順序必須同 hash")
	require.NotEqual(t, a.Hash(), c.Hash())
}

// TestSlots_MissingRequired_SerializesAsEmptyArrayWhenComplete 是 D3：欄位填齊時
// missingRequired 不得序列化成 null。這正是前端要偵測的「需求就緒」轉換點，
// 若恰好在那一刻從 [] 變成 null，前端得為同一個欄位寫兩套判斷。
func TestSlots_MissingRequired_SerializesAsEmptyArrayWhenComplete(t *testing.T) {
	full, rejected := Slots{}.Apply(SlotPatch{
		Item:          ptr("寶島梨禮盒"),
		DeliveryCity:  ptr("桃園市"),
		Quantity:      ptr(50),                                          // 選填
		Unit:          ptr("KG"),                                        // 必填（2026-08-23 併入 requiredSlots）
		BudgetPerUnit: ptr(800),                                         // 選填
		DeliveryDate:  ptr(refNow.AddDate(0, 0, 14).Format(DateLayout)), // 選填
	}, refNow)
	require.Empty(t, rejected)

	missing := full.MissingRequired()
	require.Empty(t, missing, "三個必填欄位（Item、DeliveryCity、Unit）都齊了")
	require.NotNil(t, missing, "空集合要是空 slice，不是 nil")

	b, err := json.Marshal(missing)
	require.NoError(t, err)
	require.Equal(t, "[]", string(b), "序列化必須是 []，不是 null")

	// 尚未填齊時當然還是一般陣列，不能為了修 null 而讓內容消失。
	partial, _ := Slots{}.Apply(SlotPatch{Item: ptr("梨")}, refNow)
	pb, err := json.Marshal(partial.MissingRequired())
	require.NoError(t, err)
	require.Contains(t, string(pb), string(SlotDeliveryCity))
}

func TestSlots_Apply_NormalizesCityAlias(t *testing.T) {
	got, rejected := Slots{}.Apply(SlotPatch{DeliveryCity: ptr("桃園")}, refNow)
	require.Empty(t, rejected)
	require.Equal(t, "桃園市", *got.DeliveryCity)
}

func TestSlots_Apply_AmbiguousCityStaysRejected(t *testing.T) {
	// 新竹市／新竹縣並存，不該幫買家猜——維持既有「不可辨識」的 reject 語意。
	got, rejected := Slots{}.Apply(SlotPatch{DeliveryCity: ptr("新竹")}, refNow)
	require.Nil(t, got.DeliveryCity)
	require.Len(t, rejected, 1)
	require.Equal(t, "delivery_city", rejected[0].Key)
}

func TestSlots_Apply_NormalizesDeliveryDateWithYear(t *testing.T) {
	got, rejected := Slots{}.Apply(SlotPatch{DeliveryDate: ptr("2026/8/20")}, refNow)
	require.Empty(t, rejected)
	require.Equal(t, "2026-08-20", *got.DeliveryDate)
}

func TestSlots_Apply_RejectsDeliveryDateMissingYear_WithClearReason(t *testing.T) {
	// 設計書 §5.3：缺年份要問而非猜，所以「8/20」不會被自動補年份，
	// 而是照既有 reject／再問流程走，但原因要點名是缺年份。
	got, rejected := Slots{}.Apply(SlotPatch{DeliveryDate: ptr("8/20")}, refNow)
	require.Nil(t, got.DeliveryDate)
	require.Len(t, rejected, 1)
	require.Equal(t, "delivery_date", rejected[0].Key)
	require.Contains(t, rejected[0].Reason, "缺少西元年份")
}

// TestSlots_Apply_GoldenRequestFromFlow19 還原 buyer-flow-verify flow 19 步驟 1
// 抓到的黃金需求句：「我要訂寶島梨禮盒 50 盒，單盒預算 800 元，8/20 送到桃園」。
//
// 修法前：prompt 同時要求「禁止推測」與「delivery_city/delivery_date 要正規化」，
// 模型遇衝突對兩個需正規化的欄位整個不輸出，missingRequired 誤含
// budget_per_unit（其實買家有講，只是跟 delivery_city 用同一種矛盾邏輯處理）。
//
// 修法後（2026-08-09）：budget_per_unit 與 delivery_date 改為選填；city 用別名表可解。
// delivery_date 因為原句真的沒講年份仍被拒，但因不再是必填欄位，不會出現在 missingRequired。
// 這是改版後的新語意：delivery_date 缺值時不強行補問，由 UI 在確認前主動提示。
//
// unit 不在這個 fixture 裡：AllowedUnits 收斂成純重量單位後，「盒」不再映射到任何
// 允許值，這個 golden 案例現在的抽取結果就是不帶 unit——這則測試本來就不斷言
// unit／budget_per_unit 的值，只驗證 city／date 這兩個既有斷言不受影響。
//
// unit 於 2026-08-23 併入 requiredSlots 後，這個 golden 句子（原話沒講單位）現在
// 會真的落在 missing 裡：這正是新語意——不像 delivery_date（選填、缺值不強行補
// 問），unit 缺席時買家必須被再問一次才能到 REQUIREMENT_READY（否則會撞進
// price-preview 的 ErrPriceBasisUnavailable，見 slots.go requiredSlots 的 doc
// comment）。
func TestSlots_Apply_GoldenRequestFromFlow19(t *testing.T) {
	got, rejected := Slots{}.Apply(SlotPatch{
		Item:          ptr("寶島梨禮盒"),
		Quantity:      ptr(50),
		BudgetPerUnit: ptr(800),
		DeliveryCity:  ptr("桃園"),   // 買家原話，未正規化
		DeliveryDate:  ptr("8/20"), // 買家原話，缺年份
	}, refNow)

	require.Equal(t, "桃園市", *got.DeliveryCity, "縣市別名應由 domain 層解出")
	require.Nil(t, got.DeliveryDate, "缺年份不得被猜，必須留待再問（或由前端自行決定如何處理）")

	missing := got.MissingRequired()
	require.Equal(t, []SlotKey{SlotUnit}, missing,
		"Item 與 DeliveryCity 都已解出；delivery_date 雖缺但不是必填，不會出現在 missing；"+
			"unit 原話沒講，2026-08-23 起是必填，因此仍會出現在 missing")

	require.Len(t, rejected, 1)
	require.Equal(t, "delivery_date", rejected[0].Key)
	require.Contains(t, rejected[0].Reason, "缺少西元年份")
}

func TestSlots_MissingRequired(t *testing.T) {
	s, _ := Slots{}.Apply(SlotPatch{Item: ptr("梨"), Quantity: ptr(50), Unit: ptr("KG")}, refNow)
	missing := s.MissingRequired()
	require.NotContains(t, missing, SlotItem, "Item 已填")
	require.Contains(t, missing, SlotDeliveryCity, "DeliveryCity 未填且是必填")
	require.NotContains(t, missing, SlotUnit, "Unit 已填")
	require.NotContains(t, missing, SlotDeliveryDate, "DeliveryDate 已改為選填（2026-08-09）")
	require.NotContains(t, missing, SlotBudgetPerUnit, "BudgetPerUnit 已改為選填（2026-08-09）")
}

// TestSlots_MissingRequired_UnitMissing 是 TestSlots_MissingRequired 的對稱情境：
// unit 未填時必須出現在 missing（2026-08-23 併入 requiredSlots，見 slots.go 該
// 變數的 doc comment）。
func TestSlots_MissingRequired_UnitMissing(t *testing.T) {
	s, _ := Slots{}.Apply(SlotPatch{Item: ptr("梨"), DeliveryCity: ptr("桃園市")}, refNow)
	missing := s.MissingRequired()
	require.Contains(t, missing, SlotUnit, "Unit 未填且現在是必填")
}

// TestSlots_Apply_RejectsUnitNotInAllowedUnits 驗證 domain.AllowedUnits 是封閉
// 集合：曾經短暫允許過的計數單位（BOX/CASE/PIECE/SET，理由見 AllowedUnits 的
// doc comment）仍然一律被拒絕，不會悄悄又被接受。
func TestSlots_Apply_RejectsUnitNotInAllowedUnits(t *testing.T) {
	s := Slots{}
	unit := "BOX"
	patch := SlotPatch{Unit: &unit}

	got, rejected := s.Apply(patch, refNow)

	require.Nil(t, got.Unit, "BOX 不在 AllowedUnits 內，不得被採用")
	found := false
	for _, r := range rejected {
		if r.Key == string(SlotUnit) {
			found = true
			require.NotEmpty(t, r.Reason, "被拒的 unit 一定要有原因")
		}
	}
	require.True(t, found, "rejected = %v, want a RejectedField for SlotUnit", rejected)
}

// TestSlots_Apply_PreservesUnitOnRejectionWhenNewUnitInvalid 驗證「patch 帶了
// 不合法的新 unit」時，先前已確認的 Unit 維持原值，不會被清成 nil——這是
// Apply() 在其他欄位（例如 BudgetPerUnit 的邊界檢查）一貫的行為，unit 驗證
// 不該是唯一的例外。
func TestSlots_Apply_PreservesUnitOnRejectionWhenNewUnitInvalid(t *testing.T) {
	s, _ := Slots{}.Apply(SlotPatch{Unit: ptr("KG")}, refNow)
	require.Equal(t, "KG", *s.Unit)

	unit := "BOX"
	patch := SlotPatch{Unit: &unit}
	got, rejected := s.Apply(patch, refNow)

	require.Equal(t, "KG", *got.Unit, "previously confirmed Unit should stay when patch is rejected")
	require.NotEmpty(t, rejected, "patch should be rejected")
	found := false
	for _, r := range rejected {
		if r.Key == string(SlotUnit) {
			found = true
		}
	}
	require.True(t, found, "Unit rejection should be recorded")
}
