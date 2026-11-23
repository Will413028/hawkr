package registry

import (
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

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
	good := func() Definition[params] {
		return Definition[params]{
			Name: "broken", Audience: model.AudienceBuyer, Category: model.CategoryChat, Presentation: model.PresentationMessage,
			Channels:          []model.Channel{model.ChannelInbox},
			IdempotencySource: "test.Event.ID",
			IdempotencyKey:    func(p params) string { return "x" },
			Text:              func(uitext.Locale, params) (Content, error) { return Content{Title: "t", Body: "b"}, nil },
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
		"沒有文案":             func(d *Definition[params]) { d.Text = nil },
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

// 參數快照不存第三方個資：欄位名稱不得是名稱、頭像、聯絡方式，actor 與其他第三方只存 ID。
var thirdPartyField = regexp.MustCompile(`(?i)(name|nick|avatar|photo|image|picture|email|mail|phone|mobile|address|text|body|title|content)`)

func TestParamsDeclareNoThirdPartyNameFields(t *testing.T) {
	for _, e := range All() {
		_, example, err := e.ExampleParams()
		if err != nil {
			t.Fatal(err)
		}
		rt := reflect.TypeOf(example)
		for i := range rt.NumField() {
			f := rt.Field(i)
			tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if thirdPartyField.MatchString(f.Name) || thirdPartyField.MatchString(tag) {
				t.Errorf("%s 的參數欄位 %s 看起來是第三方的名稱或內容；只存 ID，讀取時向擁有模組解析", e.Spec().Name, f.Name)
			}
			if tag == "" || tag == "-" {
				t.Errorf("%s 的參數欄位 %s 沒有 json tag（快照的欄位名要固定）", e.Spec().Name, f.Name)
			}
		}
	}
}

func TestThirdPartyFieldPatternCatchesNames(t *testing.T) {
	for _, bad := range []string{"SenderName", "ActorNickname", "AvatarURL", "Email", "ContentPreview", "ProductTitle"} {
		if !thirdPartyField.MatchString(bad) {
			t.Errorf("%s 應該被擋", bad)
		}
	}
	for _, ok := range []string{"SuspensionID", "Category", "SuspendedAt", "ActorAccountID"} {
		if thirdPartyField.MatchString(ok) {
			t.Errorf("%s 不該被擋", ok)
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
