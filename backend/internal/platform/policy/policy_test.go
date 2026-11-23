package policy

import (
	"regexp"
	"slices"
	"testing"
)

var upperSnake = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*$`)

func TestValuesAreDistinctUpperSnakeCodes(t *testing.T) {
	values := Values()
	if len(values) != 17 {
		t.Fatalf("政策分類有 %d 個值，預期 17 個", len(values))
	}
	seen := map[Category]bool{}
	for _, v := range values {
		if !upperSnake.MatchString(string(v)) {
			t.Errorf("%q 不是 UPPER_SNAKE", v)
		}
		if seen[v] {
			t.Errorf("%q 重複", v)
		}
		seen[v] = true
	}
}

func TestSubsetsAreSubsetsOfValues(t *testing.T) {
	for name, c := range map[string]struct {
		subset []Category
		want   int
	}{
		"ReporterSelectable": {ReporterSelectable(), len(Values()) - len(Community()) - 1},
		"Assessable":         {Assessable(), len(Values()) - 1},
	} {
		for _, v := range c.subset {
			if !slices.Contains(Values(), v) {
				t.Errorf("%s 的 %q 不在 Values()", name, v)
			}
		}
		if len(c.subset) != c.want {
			t.Errorf("%s 有 %d 個值，預期 %d 個", name, len(c.subset), c.want)
		}
	}
}

// Other 只給檢舉人選；PlatformAbuse 只給平台判定與認定。兩個子集各缺一個值，而且缺的值不同。
func TestOtherIsOnlyForReportersAndPlatformAbuseOnlyForAssessment(t *testing.T) {
	if !slices.Contains(ReporterSelectable(), Other) || slices.Contains(Assessable(), Other) {
		t.Error("Other 只能在檢舉人的子集")
	}
	if slices.Contains(ReporterSelectable(), PlatformAbuse) || !slices.Contains(Assessable(), PlatformAbuse) {
		t.Error("PlatformAbuse 只能在平台判定與認定的子集")
	}
	for _, v := range Values() {
		if v == Other || v == PlatformAbuse || v.Community() {
			continue
		}
		if !slices.Contains(ReporterSelectable(), v) || !slices.Contains(Assessable(), v) {
			t.Errorf("%q 兩個入口都要接受", v)
		}
	}
}

// 社群類別不給店家治理的檢舉人選，但店家治理的認定與案件類別接受（內容下架的認定）；社群的子集不含 Other。
func TestCommunityCategoriesAreNotReporterSelectable(t *testing.T) {
	if got := Community(); !slices.Equal(got, []Category{Spam, Harassment, Hate, SexualContent}) {
		t.Fatalf("Community() = %v", got)
	}
	for _, v := range Community() {
		if !slices.Contains(Values(), v) || slices.Contains(ReporterSelectable(), v) || !slices.Contains(Assessable(), v) {
			t.Errorf("%q 要在 Values() 與 Assessable()，不在 ReporterSelectable()", v)
		}
		if !slices.Contains(CommunityAssessable(), v) || !v.CommunityAssessable() || !v.Community() {
			t.Errorf("%q 要在社群內容的子集", v)
		}
	}
	if slices.Contains(CommunityAssessable(), Other) {
		t.Error("Other 不能在社群內容的子集")
	}
	for _, v := range Values() {
		if got, want := v.CommunityAssessable(), slices.Contains(CommunityAssessable(), v); got != want {
			t.Errorf("%q.CommunityAssessable() = %v，子集函式說 %v", v, got, want)
		}
	}
}

func TestMethodsAgreeWithTheSubsetFunctions(t *testing.T) {
	for _, v := range Values() {
		if got, want := v.ReporterSelectable(), slices.Contains(ReporterSelectable(), v); got != want {
			t.Errorf("%q.ReporterSelectable() = %v，子集函式說 %v", v, got, want)
		}
		if got, want := v.Assessable(), slices.Contains(Assessable(), v); got != want {
			t.Errorf("%q.Assessable() = %v，子集函式說 %v", v, got, want)
		}
	}
	for _, unknown := range []Category{"", "UNSPECIFIED", "UNAUTHORIZED_SALE", "food_safety"} {
		if unknown.ReporterSelectable() || unknown.Assessable() {
			t.Errorf("%q 不是政策分類，兩個入口都不能接受", unknown)
		}
	}
}

// 每次回傳新的切片：呼叫端改動回傳值不會影響之後的呼叫。
func TestFunctionsReturnFreshSlices(t *testing.T) {
	first := Values()
	first[0] = Other
	if Values()[0] != FoodSafety {
		t.Error("Values() 回傳了共用的切片")
	}
	subset := Assessable()
	subset[0] = Other
	if Assessable()[0] != FoodSafety {
		t.Error("Assessable() 回傳了共用的切片")
	}
}

// 社群內容的子集是明列的十一個類別；商品與賣家才成立的類別、Other 不在其中，而且每個類別都必須明確歸在「可用」或「不可用」
// （新增類別時這個測試失敗，逼出一次決定，而不是預設進子集）。
func TestCommunityAssessableIsExplicit(t *testing.T) {
	want := []Category{
		MisleadingInfo, IPInfringement, Fraud, OffPlatformTrade, Privacy, Impersonation, PlatformAbuse,
		Spam, Harassment, Hate, SexualContent,
	}
	if got := CommunityAssessable(); !slices.Equal(got, want) {
		t.Fatalf("CommunityAssessable() = %v，預期 %v", got, want)
	}
	notForCommunity := []Category{FoodSafety, ProhibitedGoods, UnlicensedSale, FakeCertification, Counterfeit, Other}
	for _, v := range notForCommunity {
		if v.CommunityAssessable() || slices.Contains(CommunityAssessable(), v) {
			t.Errorf("%q 只對商品或賣家成立（或是 Other），不能在社群內容的子集", v)
		}
	}
	for _, v := range Values() {
		if in, out := slices.Contains(want, v), slices.Contains(notForCommunity, v); in == out {
			t.Errorf("%q 沒有明確歸類到社群內容的「可用」或「不可用」", v)
		}
	}
}

// 社群檢舉入口的檢舉人可選的子集恰好八個，店家通報的子集（ReporterSelectable）不因此改變：社群類別仍不在其中。
func TestCommunityReporterSelectableIsExplicit(t *testing.T) {
	want := []Category{Spam, Harassment, Hate, SexualContent, MisleadingInfo, FoodSafety, Fraud, Other}
	got := CommunityReporterSelectable()
	if !slices.Equal(got, want) || len(got) != 8 {
		t.Fatalf("CommunityReporterSelectable() = %v，預期 %v", got, want)
	}
	for _, v := range Values() {
		if g, w := v.CommunityReporterSelectable(), slices.Contains(want, v); g != w {
			t.Errorf("%q.CommunityReporterSelectable() = %v，預期 %v", v, g, w)
		}
	}
	for _, v := range Community() {
		if v.ReporterSelectable() {
			t.Errorf("%q 是社群類別，店家通報的檢舉人不能選", v)
		}
		if !v.CommunityReporterSelectable() {
			t.Errorf("%q 是社群類別，社群檢舉的檢舉人要能選", v)
		}
	}
	if PlatformAbuse.CommunityReporterSelectable() {
		t.Error("PlatformAbuse 只給平台判定與認定")
	}
	for _, unknown := range []Category{"", "SEXUAL", "POLITICAL", "FAKE_NEWS"} {
		if unknown.CommunityReporterSelectable() {
			t.Errorf("%q 不是政策分類", unknown)
		}
	}
	got[0] = Other
	if CommunityReporterSelectable()[0] != Spam {
		t.Error("CommunityReporterSelectable() 回傳了共用的切片")
	}
}

// 店家檢舉入口可選的子集恰好十二個，順序同 proto 的 accepted_policy_categories；四個社群類別與 PlatformAbuse 不在其中，每個值都是有效的分類。
func TestReporterSelectableIsExplicit(t *testing.T) {
	want := []Category{
		FoodSafety, ProhibitedGoods, UnlicensedSale, MisleadingInfo, FakeCertification, Counterfeit,
		IPInfringement, Fraud, OffPlatformTrade, Privacy, Impersonation, Other,
	}
	got := ReporterSelectable()
	if !slices.Equal(got, want) {
		t.Fatalf("ReporterSelectable() = %v，預期 %v", got, want)
	}
	for _, v := range got {
		if !slices.Contains(Values(), v) {
			t.Errorf("%q 不在 Values()", v)
		}
	}
	got[0] = Spam
	if ReporterSelectable()[0] != FoodSafety {
		t.Error("ReporterSelectable() 回傳了共用的切片")
	}
}

func TestGeneratedSelectableListsContainOnlyValidCategories(t *testing.T) {
	for name, list := range map[string][]Category{
		"seller.CreateReportRequest.category":     acceptedSellerCreateReportRequestCategory,
		"moderation.SubmitReportRequest.category": acceptedModerationSubmitReportRequestCategory,
	} {
		for _, v := range list {
			if v == "" || !slices.Contains(Values(), v) {
				t.Errorf("%s 的 %q 不是有效的 Category", name, v)
			}
		}
	}
}
