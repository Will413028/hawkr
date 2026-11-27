package reply

import (
	"reflect"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestCandidateFactsFromTheRevision(t *testing.T) {
	q, b := int32(20), int64(30)
	d := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	full := slots.Set{
		Item: sp("蘋果"), Quantity: &q, Unit: sp("KG"), BudgetPerUnit: &b, DeliveryCity: sp("臺北市"), DeliveryDate: &d,
		Resolution: &slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: categoryID(5), Label: "蘋果"},
	}
	matched, unverifiable := CandidateFacts(uitext.ZhTW, full)
	if want := []model.MatchedFact{{SlotKey: slots.Item, Label: "品項"}}; !reflect.DeepEqual(matched, want) {
		t.Errorf("matched = %+v", matched)
	}
	want := []model.UnverifiableFact{
		{SlotKey: slots.BudgetPerUnit, Reason: "需向賣家確認能否符合 30 元／" + unitLabel(uitext.ZhTW, "KG") + " 的單價"},
		{SlotKey: slots.Quantity, Reason: "需向賣家確認是否備有 20 " + unitLabel(uitext.ZhTW, "KG") + " 的供應量"},
		{SlotKey: slots.DeliveryCity, Reason: "需向賣家確認是否配送至臺北市"},
		{SlotKey: slots.DeliveryDate, Reason: "需向賣家確認能否於 2026-12-01 前到貨"},
	}
	if !reflect.DeepEqual(unverifiable, want) {
		t.Errorf("unverifiable = %+v", unverifiable)
	}
}

func TestCandidateFactsOnlyListFilledFields(t *testing.T) {
	matched, unverifiable := CandidateFacts(uitext.ZhTW, slots.Set{})
	if matched == nil || unverifiable == nil || len(matched) != 0 || len(unverifiable) != 0 {
		t.Fatalf("空需求要回空的非 nil slice：%#v %#v", matched, unverifiable)
	}
	q := int32(5)
	_, unverifiable = CandidateFacts(uitext.ZhTW, slots.Set{Item: sp("蘋果"), Quantity: &q})
	if len(unverifiable) != 1 || unverifiable[0].Reason != "需向賣家確認是否備有 5 的供應量" {
		t.Errorf("沒填單位不留空格：%+v", unverifiable)
	}
	if matched, _ := CandidateFacts(uitext.ZhTW, slots.Set{Item: sp("  ")}); len(matched) != 0 {
		t.Errorf("空白品項不算符合：%+v", matched)
	}
}

func TestCandidateFactsFollowTheLocale(t *testing.T) {
	c := "Taipei"
	_, unverifiable := CandidateFacts(uitext.EN, slots.Set{DeliveryCity: &c})
	if len(unverifiable) != 1 || unverifiable[0].Reason != "Ask the seller whether they deliver to Taipei" {
		t.Fatalf("unverifiable = %+v", unverifiable)
	}
	matched, _ := CandidateFacts(uitext.EN, slots.Set{Item: sp("apple")})
	if len(matched) != 1 || matched[0].Label == "品項" {
		t.Fatalf("matched = %+v", matched)
	}
}

func sp(s string) *string { return &s }
