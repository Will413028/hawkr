package model

import (
	"reflect"
	"testing"
)

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
				return
			}
			for i := range typ.NumField() {
				f := typ.Field(i)
				walk(path+"."+f.Name, f.Type)
			}
			return
		}
		if typ == reflect.TypeOf("") {
			found = append(found, path)
		}
	}
	walk("Candidate", reflect.TypeOf(Candidate{}))
	for _, p := range found {
		if !buyerFacingStrings[p] {
			t.Errorf("Candidate 有純 string 欄位 %s：買家回應不得帶自由文字（A9-D8）；要加請登記白名單", p)
		}
	}
	if len(found) == 0 {
		t.Fatal("沒走到任何 string 欄位，檢查範圍不對")
	}
}
