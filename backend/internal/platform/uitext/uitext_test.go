package uitext

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	data, err := os.ReadFile("testdata/golden.gen.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("golden 是空的")
	}
	return cases
}

// 共用 golden：期望值由 icu 參考格式器算出，Go 生成碼要全部相符（TS、Dart 各自斷言同一份檔）。
func TestGoldenMatchesGeneratedCode(t *testing.T) {
	cases := loadGolden(t)
	nonNil := map[string]int{}
	omitted := 0
	for _, c := range cases {
		var text Text
		switch c.Audience {
		case "default":
			text = For(Locale(c.Locale))
		case "staff":
			text = Staff()
		default:
			t.Fatalf("%s：未知的受眾 %q", c.Key, c.Audience)
		}
		got, ok := goldenCall(text, c)
		if c.Want == nil {
			omitted++
			if ok {
				t.Errorf("%s [%s %s] %v：應該沒有文字，得到 %q", c.Key, c.Audience, c.Locale, c.Params, got)
			}
			continue
		}
		nonNil[c.Fn]++
		if !ok || got != *c.Want {
			t.Errorf("%s [%s %s] %v：got %q (ok=%v)，want %q", c.Key, c.Audience, c.Locale, c.Params, got, ok, *c.Want)
		}
	}
	for _, c := range cases {
		if nonNil[c.Fn] == 0 {
			t.Errorf("%s：golden 裡沒有任何一筆有文字，這個函式沒有被實際驗證", c.Fn)
		}
	}
	if omitted == 0 {
		t.Error("golden 要含「沒有文字」的案例（omit 的值、名詞參數不認得）")
	}
}

func TestGoldenCoversAudiencesAndLocales(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range loadGolden(t) {
		seen[c.Audience+"/"+c.Locale] = true
	}
	for _, want := range []string{"default/zh-TW", "default/en", "staff/zh-TW"} {
		if !seen[want] {
			t.Errorf("golden 缺 %s", want)
		}
	}
}

// en 沒有翻譯的 key 一律輸出 zh-TW 的文字（§2.4 缺字的行為）。
func TestMissingEnglishFallsBackToChinese(t *testing.T) {
	zh, _ := For(ZhTW).SellerMeasureKind("WARNING")
	en, ok := For(EN).SellerMeasureKind("WARNING")
	if !ok || en != zh || zh != "警告" {
		t.Errorf("en=%q zh=%q ok=%v", en, zh, ok)
	}
	if got := For(EN).CommonUnknown(); got != "未知" {
		t.Errorf("CommonUnknown(en)=%q", got)
	}
}

func TestUnknownValuesHaveNoText(t *testing.T) {
	for _, v := range []string{"", "NOPE", "UNSPECIFIED", "MEASURE_KIND_WARNING"} {
		if got, ok := For(ZhTW).SellerMeasureKind(v); ok || got != "" {
			t.Errorf("SellerMeasureKind(%q) = %q, %v", v, got, ok)
		}
	}
	// 名詞參數不認得時整個函式沒有文字，不拼出半句。
	if got, ok := For(ZhTW).SellerMeasureTitleRisk("NOPE"); ok || got != "" {
		t.Errorf("SellerMeasureTitleRisk(NOPE) = %q, %v", got, ok)
	}
	if got, ok := For(ZhTW).SellerMeasureTitleRisk("FRAUD"); !ok || got != "風險：詐欺" {
		t.Errorf("SellerMeasureTitleRisk(FRAUD) = %q, %v", got, ok)
	}
	// select 的參數不認得走 other。
	if got, _ := For(ZhTW).SellerMeasureState("ACTIVE", "NOPE"); got != "停權中" {
		t.Errorf("ACTIVE + 不認得的 kind = %q", got)
	}
	// 內容下架不會是 ACTIVE，萬一出現也不能被說成停權中。
	for kind, want := range map[string]string{"SUSPENSION": "停權中", "CONTENT_TAKEDOWN": "內容已下架"} {
		if got, _ := For(ZhTW).SellerMeasureState("ACTIVE", kind); got != want {
			t.Errorf("ACTIVE + %s = %q，要 %q", kind, got, want)
		}
	}
	if got, _ := For(ZhTW).SellerMeasureState("ISSUED", "CONTENT_TAKEDOWN"); got != "已發出" {
		t.Errorf("ISSUED + CONTENT_TAKEDOWN = %q", got)
	}
	if got, ok := For(ZhTW).SellerMeasureKind("CONTENT_TAKEDOWN"); !ok || got != "內容下架" {
		t.Errorf("CONTENT_TAKEDOWN 的名稱 = %q, %v", got, ok)
	}
	if got, _ := For(ZhTW).SellerMeasureState("ACTIVE", "TERMINATION"); got != "合作已終止" {
		t.Errorf("ACTIVE + TERMINATION = %q", got)
	}
	if got, ok := For(ZhTW).CommonError("CANCELED"); ok || got.Text != "" {
		t.Errorf("CANCELED 不顯示文字：%q, %v", got, ok)
	}
}

// 政策分類（platform/policy）的每個值在目錄都有名稱，名稱是術語表「治理」的詞；舊的違規類別（ViolationCategory）沒有了。
func TestEveryPolicyCategoryHasACatalogName(t *testing.T) {
	names := map[policy.Category]string{}
	for _, c := range policy.Values() {
		name, ok := For(ZhTW).CommonPolicyCategory(string(c))
		if !ok || name == "" {
			t.Errorf("政策分類 %q 在目錄沒有名稱", c)
		}
		for other, taken := range names {
			if taken == name {
				t.Errorf("%q 與 %q 的名稱相同：%q", c, other, name)
			}
		}
		names[c] = name
	}
	if got := names[policy.Privacy]; got != "不當使用個人資料" {
		t.Errorf("PRIVACY 的名稱 = %q，術語表是「不當使用個人資料」", got)
	}
	if got, ok := For(ZhTW).CommonPolicyCategory("UNAUTHORIZED_SALE"); ok || got != "" {
		t.Errorf("改名之前的 UNAUTHORIZED_SALE 不該有名稱：%q, %v", got, ok)
	}
	if got, ok := For(ZhTW).CommonPolicyCategory(""); ok || got != "" {
		t.Errorf("空字串（UNSPECIFIED）不該有名稱：%q, %v", got, ok)
	}
}

// 變體依受眾 omit：APPROVED 沒有 action；CLOSED 的請求狀態標籤只有 staff 有。
func TestOmitByAudience(t *testing.T) {
	if got, ok := For(ZhTW).SellerKYCFlowStatusAction("APPROVED"); ok {
		t.Errorf("APPROVED 沒有 action，得到 %q", got)
	}
	if got, ok := For(ZhTW).SellerKYCFlowStatusAction("SUBMITTED"); !ok || got != "查看進度" {
		t.Errorf("SUBMITTED action = %q, %v", got, ok)
	}
	if got, ok := For(ZhTW).SellerMeasureRequestStateWithKind("CLOSED", "APPEAL"); ok {
		t.Errorf("賣家端不顯示 CLOSED，得到 %q", got)
	}
	if got, ok := Staff().SellerMeasureRequestStateWithKind("CLOSED", "APPEAL"); !ok || got != "申覆已結案" {
		t.Errorf("staff CLOSED = %q, %v", got, ok)
	}
}

// staff 的名詞與預設受眾不同；沒有 staff 的 enum，後台用預設受眾。
func TestStaffAudience(t *testing.T) {
	d, _ := For(ZhTW).SellerKYCFlowStatus("SUBMITTED")
	s, _ := Staff().SellerKYCFlowStatus("SUBMITTED")
	if d != "平台審核中" || s != "待審核" {
		t.Errorf("default=%q staff=%q", d, s)
	}
	if got, _ := Staff().SellerMeasureKind("WARNING"); got != "警告" {
		t.Errorf("沒有 staff 的 enum 退回預設受眾：%q", got)
	}
	if Staff().Locale() != ZhTW {
		t.Errorf("Staff 固定 zh-TW：%q", Staff().Locale())
	}
}

// 整數參數三端都輸出十進位 ASCII、不分組（golden 也鎖了 TS、Dart）。
func TestIntegerIsPlainDecimal(t *testing.T) {
	got := For(ZhTW).SellerMeasureRejectionCooldownWarning(1234)
	if !strings.Contains(got, " 1234 日") {
		t.Errorf("got %q", got)
	}
}

func TestPluralCategory(t *testing.T) {
	cases := []struct {
		l    Locale
		n    int
		want string
	}{
		{ZhTW, 1, "other"}, {ZhTW, 2, "other"}, {EN, 1, "one"}, {EN, 0, "other"}, {EN, 2, "other"}, {EN, 1234, "other"},
	}
	for _, c := range cases {
		if got := PluralCategory(c.l, c.n); got != c.want {
			t.Errorf("PluralCategory(%s, %d) = %q, want %q", c.l, c.n, got, c.want)
		}
	}
}

func TestLocaleSeam(t *testing.T) {
	ctx := context.Background()
	if got := LocaleFrom(ctx); got != DefaultLocale || DefaultLocale != ZhTW {
		t.Errorf("沒有寫入時是預設語系：%q", got)
	}
	if got := LocaleFrom(WithLocale(ctx, EN)); got != EN {
		t.Errorf("WithLocale 之後：%q", got)
	}
	var zero Text
	if zero.Locale() != DefaultLocale {
		t.Errorf("zero Text 的語系：%q", zero.Locale())
	}
}

// 伺服器文字的時間版面只有 FormatDateTime 一處：台北時間、到分鐘；沒有核可版面的語系用預設語系的。
func TestFormatDateTimeIsTaipeiMinutes(t *testing.T) {
	at := time.Date(2026, 10, 1, 23, 30, 45, 0, time.UTC)
	for _, l := range []Locale{ZhTW, EN, Locale("fr")} {
		if got := FormatDateTime(l, at); got != "2026/10/02 07:30" {
			t.Errorf("%s：%q", l, got)
		}
	}
}

// Message.Locale 是實際使用的語系：en 缺字時退回 zh-TW，就必須標 zh-TW（ADR D3）；zh-TW 一律標 zh-TW。
func TestCommonErrorReportsTheResolvedLocale(t *testing.T) {
	zh, ok := For(ZhTW).CommonError("UNAUTHENTICATED")
	if !ok || zh.Locale != ZhTW || zh.Text == "" {
		t.Fatalf("zh-TW：%+v %v", zh, ok)
	}
	en, ok := For(EN).CommonError("UNAUTHENTICATED")
	if !ok {
		t.Fatal("en 缺字時要退回 zh-TW 的文字")
	}
	if en.Text == zh.Text && en.Locale != ZhTW {
		t.Errorf("en 退回 zh-TW 的文字，locale 卻標成 %q", en.Locale)
	}
	if en.Text != zh.Text && en.Locale != EN {
		t.Errorf("en 有自己的文字，locale 卻標成 %q", en.Locale)
	}
	if staff, _ := Staff().CommonError("UNAUTHENTICATED"); staff.Locale != ZhTW {
		t.Errorf("staff 只有 zh-TW：%+v", staff)
	}
}
