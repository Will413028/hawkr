package model

import (
	"reflect"
	"testing"
)

// buyerFacingStrings 是 Candidate 裡允許的「純 string」欄位（路徑含陣列元素）：都是伺服器的固定文字或依買家填的值組出的說明，
// 不是賣家或 LLM 寫的自由文字。列舉型（具名的 string 型別，例如 SlotKey）不算自由文字。
var buyerFacingStrings = map[string]bool{
	"Candidate.MatchedFacts[].Label":       true,
	"Candidate.UnverifiableFacts[].Reason": true,
}

// AD7（A9-D8）：買家回應的候選結構不含自由文字欄位。新增任何純 string 欄位（例如 ResearchSummary）都會讓它失敗，
// 要加就得明確登記上面的白名單並說明它為什麼不是自由文字。
func TestCandidateCarriesNoFreeText(t *testing.T) {
	var found []string
	var walk func(path string, typ reflect.Type)
	walk = func(path string, typ reflect.Type) {
		kind := typ.Kind()
		if kind == reflect.Slice || kind == reflect.Array || kind == reflect.Pointer {
			walk(path+"[]", typ.Elem())
			return
		}
		if kind == reflect.Struct {
			if typ.PkgPath() != reflect.TypeOf(Candidate{}).PkgPath() {
				return // time.Time 等標準庫型別
			}
			for i := range typ.NumField() {
				f := typ.Field(i)
				walk(path+"."+f.Name, f.Type)
			}
			return
		}
		// 具名的 string 型別（例如 SlotKey）是列舉，不是自由文字；只有純 string 才算。
		if typ == reflect.TypeOf("") {
			found = append(found, path)
		}
	}
	walk("Candidate", reflect.TypeOf(Candidate{}))
	// path 在陣列上是 "Candidate.MatchedFacts[]" + ".Label"
	for _, p := range found {
		if !buyerFacingStrings[p] {
			t.Errorf("Candidate 有純 string 欄位 %s：買家回應不得帶自由文字（A9-D8）；要加請登記白名單", p)
		}
	}
	if len(found) == 0 {
		t.Fatal("沒走到任何 string 欄位，檢查範圍不對")
	}
}

// LLMCandidate 才有研究摘要；ForBuyer 轉成買家看得到的型別時不帶它。
func TestForBuyerDropsTheResearchSummary(t *testing.T) {
	c := LLMCandidate{Candidate: Candidate{StoreID: 7, Score: 3}, ResearchSummary: "只給 LLM"}
	got := c.ForBuyer()
	if got.StoreID != 7 || got.Score != 3 {
		t.Fatalf("ForBuyer = %+v", got)
	}
	if _, ok := reflect.TypeOf(got).FieldByName("ResearchSummary"); ok {
		t.Fatal("買家型別不得有 ResearchSummary")
	}
}
