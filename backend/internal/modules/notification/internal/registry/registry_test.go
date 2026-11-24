package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// wholeRegistries 是整份註冊表檢查要跑的兩份清單：正式清單，以及加上一組 Broadcast 母意圖與它的 Spawned 子型別的清單
// （正式清單還沒有 fan-out 型別，第一個真的 fan-out 型別加進來時要與後者走同一條規則，不能讓整份註冊表的測試先紅）。
func wholeRegistries() map[string][]Entry {
	return map[string][]Entry{
		"正式清單":                  All(),
		"含 Broadcast 與 Spawned": append(All(), Define(broadcastDef()), Define(spawnedDef())),
	}
}

// pipelineTypes 是走意圖管線、自己有冪等鍵的型別：排除 Fanout 型別（沒有冪等鍵與收件人）與 Spawned 型別（冪等鍵由派送決定）。
func pipelineTypes(entries []Entry) []Entry {
	return slices.DeleteFunc(slices.Clone(entries), func(e Entry) bool { return e.Spec().Fanout || e.Spec().Spawned })
}

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
	var governance []string
	for _, e := range governanceTypes {
		governance = append(governance, e.Spec().Name)
	}
	want := append([]string{
		"account_suspended", "account_unsuspended", "announcement",
		"comment_created", "comment_replied_buyer", "comment_replied_store", "comment_liked_buyer", "comment_liked_store", "post_liked", "store_followed",
		"review_created", "post_published", "post_published_item", "chat_message_buyer", "chat_message_store", "product_restocked",
	}, governance...)
	want = append(want, "voucher_expiring")
	if !slices.Equal(names, want) {
		t.Fatalf("型別清單 %v", names)
	}
}

// 註冊表自檢：每個型別有冪等鍵來源、分類在 D4 清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別。
func TestEveryRegisteredTypePassesTheSelfCheck(t *testing.T) {
	for label, entries := range wholeRegistries() {
		seen := map[string]bool{}
		for _, e := range entries {
			name := e.Spec().Name
			if seen[name] {
				t.Errorf("%s：型別名稱 %s 重複", label, name)
			}
			seen[name] = true
			for _, err := range Validate(e) {
				t.Errorf("%s：%v", label, err)
			}
			s := e.Spec()
			if s.IdempotencySource == "" && !s.Fanout && !s.Spawned {
				t.Errorf("%s：%s 沒有冪等鍵來源", label, name)
			}
			if !s.Broadcast && (!model.ValidCategory(s.Category) || !slices.Contains(model.Presentations(), s.Presentation)) {
				t.Errorf("%s：%s 的分類不在清單", label, name)
			}
			if label == "正式清單" {
				if _, ok := Lookup(name); !ok {
					t.Errorf("Lookup(%s) 找不到", name)
				}
			}
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
			Name: "broken", Audiences: []model.Audience{model.AudienceBuyer}, Category: model.CategoryChat, Presentation: model.PresentationMessage,
			Channels:          []model.Channel{model.ChannelInbox},
			IdempotencySource: "test.Event.ID",
			IdempotencyKey:    func(p params) string { return "x" },
			OccurredAt:        func(params) time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
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
		"受眾不存在":            func(d *Definition[params]) { d.Audiences = []model.Audience{"SELLER"} },
		"沒有渠道":             func(d *Definition[params]) { d.Channels = nil },
		"渠道不存在":            func(d *Definition[params]) { d.Channels = []model.Channel{"SMS"} },
		"聚合鍵出現在未宣告可聚合":     func(d *Definition[params]) { d.AggregationKey = func(params) string { return "k" } },
		"宣告可聚合卻沒有聚合鍵":      func(d *Definition[params]) { d.Aggregatable = true },
		"名稱不是 lower_snake": func(d *Definition[params]) { d.Name = "Broken-Type" },
		"沒有事件時間來源":         func(d *Definition[params]) { d.OccurredAt = nil },
		"事件時間是空的":          func(d *Definition[params]) { d.OccurredAt = func(params) time.Time { return time.Time{} } },
		"沒有收件人解析":          func(d *Definition[params]) { d.Recipient = nil },
		"BUYER 帶 StoreID": func(d *Definition[params]) {
			d.Recipient = func(params) Recipient { return Recipient{AccountID: 1, StoreID: 9} }
		},
		"STORE 沒有 StoreID": func(d *Definition[params]) {
			d.Audiences = []model.Audience{model.AudienceStore}
			d.Category = model.CategoryOrder
		},
		"沒有文案": func(d *Definition[params]) { d.Text = nil },
		"Text 與 TextLoaded 並存": func(d *Definition[params]) {
			d.TextLoaded = func(l uitext.Locale, _ params, _ Loaded) (Content, error) { return Content{}, nil }
		},
		"Load 沒有 TextLoaded": func(d *Definition[params]) {
			d.Load = func(context.Context, Sources, []params) ([]Loaded, error) { return nil, nil }
		},
		"走意圖管線卻有多個受眾": func(d *Definition[params]) {
			d.Audiences = []model.Audience{model.AudienceBuyer, model.AudienceStore}
		},
		"受眾重複": func(d *Definition[params]) {
			d.Fanout = true
			d.Audiences = []model.Audience{model.AudienceBuyer, model.AudienceBuyer}
		},
		"fan-out 型別帶收件人": func(d *Definition[params]) { d.Fanout = true },
		"固定語系與文字不符":      func(d *Definition[params]) { d.FixedLocale = uitext.EN },
		"分類不屬於受眾":        func(d *Definition[params]) { d.Category = model.CategoryShipmentAnomaly },
		"en 退回中文": func(d *Definition[params]) {
			d.Text = func(_ uitext.Locale, _ params) (Content, error) { return zhOnly(), nil }
		},
		"標題與內文語系不一致": func(d *Definition[params]) {
			d.Text = func(l uitext.Locale, _ params) (Content, error) { return mixed(l), nil }
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

// publicCatalogNames 是明確的例外：商品名稱是公開目錄資料，不是個人姓名（D28）；仍受 180 天保留期約束。只列「型別.欄位」的完整路徑，其他型別的同名欄位照擋。
var publicCatalogNames = map[string]bool{"ProductRestockedParams.ProductName": true}

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
		if isThirdPartyField(f.Name, tag) && !publicCatalogNames[at] {
			*out = append(*out, at+" 看起來是第三方的名稱或內容；只存 ID，讀取時向擁有模組解析")
		}
		if tag == "" || tag == "-" {
			*out = append(*out, at+" 沒有 json tag（快照的欄位名要固定）")
		}
		walkParams(f.Type, at, seen, out)
	}
}

func TestParamsDeclareNoThirdPartyNameFields(t *testing.T) {
	for label, entries := range wholeRegistries() {
		for _, e := range entries {
			_, example, err := e.ExampleParams()
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range paramsProblems(reflect.TypeOf(example)) {
				t.Errorf("%s：%s：%s", label, e.Spec().Name, p)
			}
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
	type otherProduct struct {
		ProductName string `json:"product_name"`
	}
	if len(paramsProblems(reflect.TypeOf(otherProduct{}))) == 0 {
		t.Error("白名單只放行 ProductRestockedParams.ProductName，其他型別的同名欄位要被擋")
	}
	if !publicCatalogNames["ProductRestockedParams.ProductName"] || len(publicCatalogNames) != 1 {
		t.Errorf("公開目錄名稱白名單只該有補貨的商品名稱：%v", publicCatalogNames)
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

// 冪等鍵與聚合鍵不得超過 db/schema.sql 的 CHECK（200）：太長的來源身分改存「型別名稱:sha256」，決定性且不失敗。
func TestLongKeysAreHashedToFit(t *testing.T) {
	type params struct{ K string }
	e := Define(Definition[params]{
		IdempotencySource: "x", IdempotencyKey: func(p params) string { return p.K }, Name: "t",
		Aggregatable: true, AggregationKey: func(p params) string { return p.K },
		Recipient: func(params) Recipient { return Recipient{AccountID: 1} }, Audiences: []model.Audience{model.AudienceBuyer},
	})
	raw := func(k string) json.RawMessage { return json.RawMessage(`{"K":"` + k + `"}`) }
	exact := strings.Repeat("a", MaxIdempotencyKeyLen-len("t:"))
	if k, err := e.IdempotencyKey(raw(exact)); err != nil || k != "t:"+exact {
		t.Errorf("剛好 200 字元要原樣保留：%d、%v", len(k), err)
	}
	long := strings.Repeat("b", 250)
	sum := sha256.Sum256([]byte(long))
	want := "t:" + hex.EncodeToString(sum[:])
	k, err := e.IdempotencyKey(raw(long))
	if err != nil || k != want || len(k) > MaxIdempotencyKeyLen {
		t.Errorf("250 字元的冪等鍵 = %q、%v，應該是 %q", k, err, want)
	}
	if k2, _ := e.IdempotencyKey(raw(long)); k2 != k {
		t.Error("雜湊鍵不是決定性的")
	}
	if other, _ := e.IdempotencyKey(raw(long + "c")); other == k {
		t.Error("不同來源不該得到同一個雜湊鍵")
	}
	r, err := e.Resolve(raw(long))
	if err != nil || r.AggregationKey != want {
		t.Errorf("250 字元的聚合鍵 = %q、%v，應該是 %q", r.AggregationKey, err, want)
	}
	for label, entries := range wholeRegistries() {
		for _, e := range pipelineTypes(entries) {
			raw, _, _ := e.ExampleParams()
			if k, err := e.IdempotencyKey(raw); err != nil || len(k) > MaxIdempotencyKeyLen {
				t.Errorf("%s：%s 的範例冪等鍵長度 %d（%v）", label, e.Spec().Name, len(k), err)
			}
		}
	}
}

// 聚合鍵加型別名稱前綴：兩個型別的同名鍵存成不同的值，不會合併成同一列（Resolve 與 Render 一致）。
func TestAggregationKeysAreNamespacedByType(t *testing.T) {
	type params struct{ K string }
	def := func(name string) Entry {
		return Define(Definition[params]{
			Name: name, Audiences: []model.Audience{model.AudienceBuyer}, Aggregatable: true,
			AggregationKey: func(p params) string { return p.K },
			Recipient:      func(params) Recipient { return Recipient{AccountID: 1} },
			Text: func(l uitext.Locale, _ params) (Content, error) {
				return Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
			},
		})
	}
	raw := json.RawMessage(`{"K":"post-7"}`)
	a, err := def("type_a").Resolve(raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := def("type_b").Resolve(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.AggregationKey != "type_a:post-7" || b.AggregationKey != "type_b:post-7" || a.AggregationKey == b.AggregationKey {
		t.Errorf("聚合鍵 = %q、%q", a.AggregationKey, b.AggregationKey)
	}
	if r, err := def("type_a").Render(uitext.ZhTW, raw); err != nil || r.AggregationKey != a.AggregationKey {
		t.Errorf("Render 的聚合鍵 = %q、%v，應該與 Resolve 一致", r.AggregationKey, err)
	}
}

// 冪等鍵是型別名稱加事件來源身分；同一個事件來源身分在不同型別下不撞。
func TestIdempotencyKeysAreNamespacedByType(t *testing.T) {
	for label, entries := range wholeRegistries() {
		keys := map[string]string{}
		for _, e := range pipelineTypes(entries) {
			raw, _, err := e.ExampleParams()
			if err != nil {
				t.Fatal(err)
			}
			k, err := e.IdempotencyKey(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(k, e.Spec().Name+":") {
				t.Errorf("%s：%s 的冪等鍵 %q 沒有型別前綴", label, e.Spec().Name, k)
			}
			if other, dup := keys[k]; dup {
				t.Errorf("%s：%s 與 %s 的冪等鍵相同：%s", label, e.Spec().Name, other, k)
			}
			keys[k] = e.Spec().Name
		}
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
