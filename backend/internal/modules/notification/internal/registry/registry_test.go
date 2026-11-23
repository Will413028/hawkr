package registry

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func zhOnly() Content {
	return Content{Title: uitext.Message{Text: "t", Locale: uitext.ZhTW}, Body: uitext.Message{Text: "b", Locale: uitext.ZhTW}}
}

func mixed(l uitext.Locale) Content {
	return Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: uitext.ZhTW}, Fragments: []uitext.Message{{Text: "f", Locale: uitext.EN}}}
}

func TestRegistryHoldsTheTypesOfThisStep(t *testing.T) {
	var names []string
	for _, e := range All() {
		names = append(names, e.Spec().Name)
	}
	if !slices.Equal(names, []string{"account_suspended", "account_unsuspended"}) {
		t.Fatalf("A8-1a 只有兩個型別，現在 %v", names)
	}
}

// 註冊表自檢：每個型別有冪等鍵來源、分類在 D4 清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別。
func TestEveryRegisteredTypePassesTheSelfCheck(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range All() {
		name := e.Spec().Name
		if seen[name] {
			t.Errorf("型別名稱 %s 重複", name)
		}
		seen[name] = true
		for _, err := range Validate(e) {
			t.Error(err)
		}
		s := e.Spec()
		if s.IdempotencySource == "" {
			t.Errorf("%s 沒有冪等鍵來源", name)
		}
		if !model.ValidCategory(s.Category) || !slices.Contains(model.Presentations(), s.Presentation) {
			t.Errorf("%s 的分類不在清單", name)
		}
		if _, ok := Lookup(name); !ok {
			t.Errorf("Lookup(%s) 找不到", name)
		}
	}
}

// 自檢本身要抓得到違規：每種違規各造一個壞的型別定義。
func TestSelfCheckRejectsBrokenDefinitions(t *testing.T) {
	type params struct{ ID int64 }
	textIn := func(l uitext.Locale, _ params) (Content, error) {
		return Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
	}
	good := func() Definition[params] {
		return Definition[params]{
			Name: "broken", Audience: model.AudienceBuyer, Category: model.CategoryChat, Presentation: model.PresentationMessage,
			Channels:          []model.Channel{model.ChannelInbox},
			IdempotencySource: "test.Event.ID",
			IdempotencyKey:    func(p params) string { return "x" },
			Recipient:         func(p params) Recipient { return Recipient{AccountID: p.ID} },
			Text:              textIn,
			Example:           params{ID: 1},
		}
	}
	if errs := Validate(Define(good())); len(errs) != 0 {
		t.Fatalf("基準定義應該通過：%v", errs)
	}
	cases := map[string]func(*Definition[params]){
		"沒有冪等鍵來源":          func(d *Definition[params]) { d.IdempotencySource = ""; d.IdempotencyKey = nil },
		"冪等鍵來源是空的":         func(d *Definition[params]) { d.IdempotencyKey = func(params) string { return "" } },
		"分類不在 D4":          func(d *Definition[params]) { d.Category = "SMS_ALERTS" },
		"呈現分類不在 D6":        func(d *Definition[params]) { d.Presentation = "ANNOUNCEMENT" },
		"提示不在 D6":          func(d *Definition[params]) { d.StateHint = "PAYMENT" },
		"受眾不存在":            func(d *Definition[params]) { d.Audience = "SELLER" },
		"沒有渠道":             func(d *Definition[params]) { d.Channels = nil },
		"渠道不存在":            func(d *Definition[params]) { d.Channels = []model.Channel{"SMS"} },
		"聚合鍵出現在未宣告可聚合":     func(d *Definition[params]) { d.AggregationKey = func(params) string { return "k" } },
		"宣告可聚合卻沒有聚合鍵":      func(d *Definition[params]) { d.Aggregatable = true },
		"名稱不是 lower_snake": func(d *Definition[params]) { d.Name = "Broken-Type" },
		"沒有收件人解析":          func(d *Definition[params]) { d.Recipient = nil },
		"BUYER 帶 StoreID": func(d *Definition[params]) {
			d.Recipient = func(params) Recipient { return Recipient{AccountID: 1, StoreID: 9} }
		},
		"STORE 沒有 StoreID": func(d *Definition[params]) { d.Audience = model.AudienceStore; d.Category = model.CategoryOrder },
		"沒有文案":             func(d *Definition[params]) { d.Text = nil },
		"分類不屬於受眾":          func(d *Definition[params]) { d.Category = model.CategoryShipmentAnomaly },
		"en 退回中文": func(d *Definition[params]) {
			d.Text = func(_ uitext.Locale, _ params) (Content, error) { return zhOnly(), nil }
		},
		"標題與內文語系不一致": func(d *Definition[params]) {
			d.Text = func(l uitext.Locale, _ params) (Content, error) { return mixed(l), nil }
		},
		"冪等鍵超過 200 字元": func(d *Definition[params]) {
			d.IdempotencyKey = func(params) string { return strings.Repeat("x", 200) }
		},
	}
	for name, mutate := range cases {
		d := good()
		mutate(&d)
		if len(Validate(Define(d))) == 0 {
			t.Errorf("%s：自檢沒有抓到", name)
		}
	}
	ok := good()
	ok.Aggregatable = true
	ok.AggregationKey = func(params) string { return "k" }
	if errs := Validate(Define(ok)); len(errs) != 0 {
		t.Errorf("宣告可聚合並帶聚合鍵應該通過：%v", errs)
	}
}

// 參數快照不存第三方個資：欄位名稱不得是名稱、頭像、聯絡方式，actor 與其他第三方只存 ID。檢查會往巢狀的結構、切片、map 與指標裡走；
// 身分與物件 key（名稱以 ID、Key 結尾，或 json tag 以 _id、_ids、_key 結尾）只是參照，不算內容，例如 ImageKey。
var thirdPartyField = regexp.MustCompile(`(?i)(name|nick|avatar|photo|image|picture|email|mail|phone|mobile|address|text|body|title|content)`)

var referenceField = regexp.MustCompile(`(ID|IDs|Id|Key)$|_(id|ids|key)$`)

func isThirdPartyField(name, tag string) bool {
	for _, n := range []string{name, tag} {
		if n != "" && thirdPartyField.MatchString(n) && !referenceField.MatchString(n) {
			return true
		}
	}
	return false
}

// paramsProblems 回傳參數型別 t 的違規；根一定要是結構（快照的欄位名要固定）。
func paramsProblems(t reflect.Type) []string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return []string{fmt.Sprintf("參數型別 %s 不是結構", t)}
	}
	var out []string
	walkParams(t, t.Name(), map[reflect.Type]bool{}, &out)
	return out
}

func walkParams(t reflect.Type, path string, seen map[reflect.Type]bool, out *[]string) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		at := path + "." + f.Name
		if isThirdPartyField(f.Name, tag) {
			*out = append(*out, at+" 看起來是第三方的名稱或內容；只存 ID，讀取時向擁有模組解析")
		}
		if tag == "" || tag == "-" {
			*out = append(*out, at+" 沒有 json tag（快照的欄位名要固定）")
		}
		walkParams(f.Type, at, seen, out)
	}
}

func TestParamsDeclareNoThirdPartyNameFields(t *testing.T) {
	for _, e := range All() {
		_, example, err := e.ExampleParams()
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paramsProblems(reflect.TypeOf(example)) {
			t.Errorf("%s：%s", e.Spec().Name, p)
		}
	}
}

func TestThirdPartyFieldGuard(t *testing.T) {
	type nestedBad struct {
		DisplayName string `json:"display_name"`
	}
	type reference struct {
		ImageKey string  `json:"image_key"`
		ActorID  int64   `json:"actor_id"`
		Ids      []int64 `json:"ids"`
	}
	type topImage struct {
		Image string `json:"image"`
	}
	type withNested struct {
		Inner nestedBad `json:"inner"`
	}
	type withSlice struct {
		Items []nestedBad `json:"items"`
	}
	type withPointer struct {
		Inner *nestedBad `json:"inner"`
	}
	type withMap struct {
		M map[string]nestedBad `json:"m"`
	}
	type untagged struct{ ID int64 }
	type recursive struct {
		Next *recursive `json:"next"`
		Name string     `json:"name"`
	}
	for name, c := range map[string]struct {
		typ  reflect.Type
		want bool
	}{
		"頂層名稱":        {reflect.TypeOf(nestedBad{}), true},
		"image 欄位":    {reflect.TypeOf(topImage{}), true},
		"巢狀結構":        {reflect.TypeOf(withNested{}), true},
		"切片裡的結構":      {reflect.TypeOf(withSlice{}), true},
		"指標裡的結構":      {reflect.TypeOf(withPointer{}), true},
		"map 裡的結構":    {reflect.TypeOf(withMap{}), true},
		"沒有 json tag": {reflect.TypeOf(untagged{}), true},
		"遞迴型別不會無限迴圈":  {reflect.TypeOf(recursive{}), true},
		"根是指標":        {reflect.TypeOf(&nestedBad{}), true},
		"根不是結構":       {reflect.TypeOf(""), true},
		"根是切片":        {reflect.TypeOf([]int64{}), true},
		"key 與 ID 欄位": {reflect.TypeOf(reference{}), false},
	} {
		if got := len(paramsProblems(c.typ)) > 0; got != c.want {
			t.Errorf("%s：違規=%v，預期 %v（%v）", name, got, c.want, paramsProblems(c.typ))
		}
	}
	for _, bad := range []string{"SenderName", "ActorNickname", "AvatarURL", "Email", "ContentPreview", "ProductTitle", "Image"} {
		if !isThirdPartyField(bad, "") {
			t.Errorf("%s 應該被擋", bad)
		}
	}
	for _, ok := range []string{"SuspensionID", "Category", "SuspendedAt", "ActorAccountID", "ImageKey"} {
		if isThirdPartyField(ok, "") {
			t.Errorf("%s 不該被擋", ok)
		}
	}
}

// 冪等鍵不得超過 db/schema.sql 的 CHECK（200）：太長的來源身分在註冊表就是錯誤，而不是寫入時才被 CHECK 擋下。
func TestIdempotencyKeyLengthIsBounded(t *testing.T) {
	type params struct{ K string }
	def := func(k string) Entry {
		return Define(Definition[params]{IdempotencySource: "x", IdempotencyKey: func(p params) string { return p.K }, Name: "t"})
	}
	e := def("")
	long := strings.Repeat("a", MaxIdempotencyKeyLen-len("t:")+1)
	if _, err := e.IdempotencyKey(json.RawMessage(`{"K":"` + long + `"}`)); err == nil {
		t.Error("201 字元的冪等鍵要回錯誤")
	}
	if k, err := e.IdempotencyKey(json.RawMessage(`{"K":"` + long[1:] + `"}`)); err != nil || len(k) != MaxIdempotencyKeyLen {
		t.Errorf("剛好 200 字元要通過：%d、%v", len(k), err)
	}
	for _, e := range All() {
		raw, _, _ := e.ExampleParams()
		if k, err := e.IdempotencyKey(raw); err != nil || len(k) > MaxIdempotencyKeyLen {
			t.Errorf("%s 的範例冪等鍵長度 %d（%v）", e.Spec().Name, len(k), err)
		}
	}
}

// 冪等鍵是型別名稱加事件來源身分；同一個事件來源身分在不同型別下不撞。
func TestIdempotencyKeysAreNamespacedByType(t *testing.T) {
	keys := map[string]string{}
	for _, e := range All() {
		raw, _, err := e.ExampleParams()
		if err != nil {
			t.Fatal(err)
		}
		k, err := e.IdempotencyKey(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(k, e.Spec().Name+":") {
			t.Errorf("%s 的冪等鍵 %q 沒有型別前綴", e.Spec().Name, k)
		}
		if other, dup := keys[k]; dup {
			t.Errorf("%s 與 %s 的冪等鍵相同：%s", e.Spec().Name, other, k)
		}
		keys[k] = e.Spec().Name
	}
}

func TestBadSnapshotIsAnError(t *testing.T) {
	e, _ := Lookup("account_suspended")
	if _, err := e.Render(uitext.ZhTW, json.RawMessage(`{"suspension_id":"nope"}`)); err == nil {
		t.Error("壞的快照要回錯誤")
	}
	if _, err := e.Render(uitext.ZhTW, json.RawMessage(`{"suspension_id":"0190a000-0000-7000-8000-000000000001","category":"NOT_A_CATEGORY","suspended_at":"2026-11-23T04:30:00Z"}`)); err == nil {
		t.Error("沒有顯示文字的政策分類要回錯誤")
	}
}

// 組好的文字各部分語系不一致時，Render 回錯誤，不挑一個語系矇混。
func TestRenderRejectsMixedLocales(t *testing.T) {
	type params struct{ ID int64 }
	e := Define(Definition[params]{
		Name: "mixed", IdempotencySource: "x", IdempotencyKey: func(params) string { return "k" },
		Text: func(l uitext.Locale, _ params) (Content, error) { return mixed(l), nil },
	})
	if _, err := e.Render(uitext.EN, json.RawMessage(`{"ID":1}`)); err == nil {
		t.Error("語系不一致要回錯誤")
	}
}
