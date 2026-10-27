package domain

import (
	"reflect"
	"slices"
	"testing"
)

func TestNumberTokensIn_StripsLeadingZeros(t *testing.T) {
	// 到貨日期 2026-09-30 與買家口語「9月30日」必須抽出同一組 token，
	// 否則模型把 ISO 日期改寫成口語講法會被 fact-echo 誤擋。
	got := NumberTokensIn("到貨日期：2026-09-30")
	want := []string{"2026", "9", "30"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NumberTokensIn = %v, want %v", got, want)
	}
}

func TestNumberTokensIn_KeepsDistinctMagnitudes(t *testing.T) {
	// 50 與 500 不得互相涵蓋——這正是 fact-echo 要擋的那種竄改。
	got := NumberTokensIn("數量：500")
	for _, tok := range got {
		if tok == "50" {
			t.Fatalf("NumberTokensIn(500) 不該產出 50：%v", got)
		}
	}
}

func TestNumberTokensIn_IgnoresChineseNumerals(t *testing.T) {
	if got := NumberTokensIn("還有兩項要補，下週送達"); len(got) != 0 {
		t.Fatalf("中文數量詞不該被抽出：%v", got)
	}
}

func TestNumberTokensIn_Decimal(t *testing.T) {
	got := NumberTokensIn("預算：12.5")
	want := []string{"12.5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NumberTokensIn = %v, want %v", got, want)
	}
}

func TestCanonicalCitiesIn_NormalizesAlias(t *testing.T) {
	got := CanonicalCitiesIn("送到桃園就好")
	want := []string{"桃園市"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CanonicalCitiesIn = %v, want %v", got, want)
	}
}

func TestCanonicalCitiesIn_OldCountyNameMapsToNewCity(t *testing.T) {
	got := CanonicalCitiesIn("台北縣")
	want := []string{"新北市"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CanonicalCitiesIn = %v, want %v", got, want)
	}
}

func TestCanonicalCitiesIn_NoMatch(t *testing.T) {
	if got := CanonicalCitiesIn("主要資訊都到齊了"); len(got) != 0 {
		t.Fatalf("不該命中任何城市：%v", got)
	}
}

func TestCanonicalUnitsIn_ChineseAlias(t *testing.T) {
	got := CanonicalUnitsIn("50 公斤")
	want := []string{"KG"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CanonicalUnitsIn = %v, want %v", got, want)
	}
}

func TestCanonicalUnitsIn_CanonicalAndLowercase(t *testing.T) {
	for _, in := range []string{"單位：KG", "單位：kg"} {
		got := CanonicalUnitsIn(in)
		if !reflect.DeepEqual(got, []string{"KG"}) {
			t.Fatalf("CanonicalUnitsIn(%q) = %v, want [KG]", in, got)
		}
	}
}

func TestCanonicalUnitsIn_IgnoresChocolate(t *testing.T) {
	// 「克」已移除，「克力」不應誤命中 GRAM。
	// 生鮮採購常見品項：巧克力禮盒、蘭姆葡萄乾巧克力等。
	got := CanonicalUnitsIn("品項：巧克力禮盒")
	if len(got) != 0 {
		t.Fatalf("CanonicalUnitsIn(品項：巧克力禮盒) = %v, want []", got)
	}
}

func TestCanonicalCitiesIn_MultipleNoOverlapWithShortAlias(t *testing.T) {
	// 多城市同時出現不漏抽。
	// 「桃園」別名優先，「台中」直接匹配 AllowedCities。
	got := CanonicalCitiesIn("從桃園送到台中")
	want := map[string]bool{"桃園市": true, "台中市": true}
	if len(got) != 2 {
		t.Fatalf("CanonicalCitiesIn(從桃園送到台中) 返回 %d 城市，期望 2 個：%v", len(got), got)
	}
	for _, city := range got {
		if !want[city] {
			t.Fatalf("CanonicalCitiesIn(從桃園送到台中) 不應返回 %s：%v", city, got)
		}
	}
}

func TestCanonicalCitiesIn_CountyAliasBlocksRootAlias(t *testing.T) {
	// 縣別名優先於根別名，且同句還有另一個不相干城市。
	// 「台北縣」→新北市（別名）；「高雄縣」→高雄市（別名）；「台南」→台南市（別名）。
	// 不應產出「台北市」（因為「台北」被「台北縣」的 skip 機制擋掉）。
	got := CanonicalCitiesIn("從台北縣送到高雄縣，再轉去台南")
	want := map[string]bool{"新北市": true, "高雄市": true, "台南市": true}
	if len(got) != 3 {
		t.Fatalf("CanonicalCitiesIn(...) 返回 %d 城市，期望 3 個：%v", len(got), got)
	}
	for _, city := range got {
		if !want[city] {
			t.Fatalf("CanonicalCitiesIn(...) 不應返回 %s：%v", city, got)
		}
	}
	// 明確檢查不含台北市。
	if slices.Contains(got, "台北市") {
		t.Fatalf("CanonicalCitiesIn(...) 不應返回台北市（應被台北縣別名擋掉）：%v", got)
	}
}

func TestCanonicalCitiesIn_FoldsTaiwanVariantForm(t *testing.T) {
	// whole-branch review I1：cityAliases 把「台北」→台北市、「臺北」→臺北市
	// 映成兩個不同的 canonical 值，fact-echo 用集合精確相等比對——不摺異體字
	// 的話 committed「臺北市」而模型寫「台北」就會被判成 fact_mismatch。
	// 臺北／臺中／臺南／臺東四個都會都受影響，而 delivery_city 是必填欄位。
	for _, pair := range [][2]string{
		{"配送城市：臺北市", "台北"},
		{"配送城市：台北市", "臺北"},
		{"配送城市：臺中市", "台中"},
		{"配送城市：台南市", "臺南"},
		{"配送城市：臺東縣", "台東"},
	} {
		haystack, text := CanonicalCitiesIn(pair[0]), CanonicalCitiesIn(pair[1])
		if !reflect.DeepEqual(haystack, text) {
			t.Fatalf("CanonicalCitiesIn(%q)=%v 與 CanonicalCitiesIn(%q)=%v 必須收斂到同一組 token",
				pair[0], haystack, pair[1], text)
		}
	}
}

func TestCanonicalCitiesIn_VariantFormsDoNotDoubleCount(t *testing.T) {
	// 同一句同時出現兩種寫法時只算一個城市，不會因為摺疊而重複加入。
	got := CanonicalCitiesIn("臺北市與台北市是同一個地方")
	if !reflect.DeepEqual(got, []string{"台北市"}) {
		t.Fatalf("CanonicalCitiesIn = %v, want [台北市]", got)
	}
}

func TestCanonicalUnitsIn_IgnoresEnglishWordsContainingGram(t *testing.T) {
	// 小寫 "gram" 已移除：program／Instagram／telegram／diagram 不得誤命中 GRAM，
	// 理由跟「g」與「克」完全相同（子字串比對命中更長的無關字詞）。
	for _, in := range []string{"已排入 program 流程", "Instagram 的貼文", "telegram 群組", "流程 diagram"} {
		if got := CanonicalUnitsIn(in); len(got) != 0 {
			t.Fatalf("CanonicalUnitsIn(%q) = %v, want []", in, got)
		}
	}
	// 正式值與中文別名仍必須抽得出來。
	for _, in := range []string{"單位：GRAM", "500 公克"} {
		if got := CanonicalUnitsIn(in); !reflect.DeepEqual(got, []string{"GRAM"}) {
			t.Fatalf("CanonicalUnitsIn(%q) = %v, want [GRAM]", in, got)
		}
	}
}
