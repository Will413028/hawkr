// Package registry 是通知型別註冊表：每個型別的 typed 參數、受眾、偏好分類（D4）、呈現分類（D6）、渠道、可選的聚合鍵、冪等鍵來源、
// actor、文案、深連結與可選的推播圖片都寫成 Go 程式（Definition），沒有資料表、沒有模板。
//
// 型別只在這裡定義；參數快照存成 JSON（收件匣與意圖），讀取時用型別自己的參數結構解回來再組字。Validate 與 registry_test.go 是
// 註冊表自檢：缺冪等鍵來源、分類不在 D4 清單、呈現分類不在 D6 清單、聚合鍵與可聚合宣告不一致、參數帶第三方名稱欄位都會失敗。
package registry

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Content 是一則通知組好的字。標題與內文各帶它實際使用的語系（uitext.Message.Locale）；通知的語系由它們決定，不另外宣告。
type Content struct {
	Title uitext.Message
	Body  uitext.Message
	// Fragments 是嵌進標題或內文的其他文字（例如政策分類的顯示文字），它們的語系要和標題、內文一致，不在一句話裡夾另一種語言。
	Fragments []uitext.Message
}

// locale 回傳 Content 實際使用的語系；各部分的語系不一致時回錯誤。
func (c Content) locale() (uitext.Locale, error) {
	l := c.Title.Locale
	for _, m := range append([]uitext.Message{c.Body}, c.Fragments...) {
		if m.Locale != l {
			return "", fmt.Errorf("文字各部分的語系不一致：%q 與 %q", l, m.Locale)
		}
	}
	if l == "" {
		return "", fmt.Errorf("文字沒有標明語系")
	}
	return l, nil
}

// Definition 是一個通知型別，P 是它的 typed 參數（只存 ID 與事件事實，不存第三方的名稱、頭像或聯絡方式）。
type Definition[P any] struct {
	// Name 是型別名稱（lower_snake），也是 intents.type、inbox_items.type。
	Name string
	// Audience 是這個型別的收件人身分面。
	Audience model.Audience
	// Category 是偏好分類（D4）。
	Category model.PrefCategory
	// Presentation 是收件匣的呈現分類（D6）。
	Presentation model.Presentation
	// StateHint 是推播與串流帶的狀態重讀提示（D6）；空值表示不需要重讀。取代舊模板的 silent 推播。
	StateHint model.StateHint
	// Channels 是這個型別會進的渠道（收件匣、推播、email）；使用者的偏好再從中過濾。
	Channels []model.Channel
	// IdempotencySource 說明冪等鍵取自哪個事件來源身分（人讀；例如 accountsuspension.Suspended.SuspensionID）。
	IdempotencySource string
	// IdempotencyKey 由參數取出事件來源身分；Entry.IdempotencyKey 會加上型別名稱前綴，所以不同型別不會互撞。
	IdempotencyKey func(P) string
	// Aggregatable 宣告型別可聚合（D12：讚、追蹤）；為 true 才可以有 AggregationKey，而且一定要有。
	Aggregatable   bool
	AggregationKey func(P) string
	// Actors 回傳參數裡的 actor 帳號 ID（只存帳號 ID，名稱與頭像讀取時向擁有模組解析）；沒有 actor 的型別留空。
	Actors func(P) []int64
	// Text 依語系組字。
	Text func(l uitext.Locale, p P) (Content, error)
	// DeepLink 回傳深連結；留空表示沒有目的地。
	DeepLink func(P) string
	// ImageKey 回傳推播圖片的物件 key（D14，商品類型別帶）；留空表示沒有圖。
	ImageKey func(P) string
	// Example 是固定輸出測試與註冊表自檢用的範例參數。
	Example P
}

// Spec 是型別的靜態屬性（不含參數相關的函式）。
type Spec struct {
	Name              string
	Audience          model.Audience
	Category          model.PrefCategory
	Presentation      model.Presentation
	StateHint         model.StateHint
	Channels          []model.Channel
	IdempotencySource string
	Aggregatable      bool
}

// Rendered 是一個型別對一份參數快照與一個語系的完整輸出。
type Rendered struct {
	Title          string
	Body           string
	DeepLink       string
	ImageKey       string
	Presentation   model.Presentation
	StateHint      model.StateHint
	AggregationKey string
	ActorAccounts  []int64
	// Locale 是文字實際使用的語系，取自組出來的文字（請求的語系沒有文字時是 zh-TW）。
	Locale uitext.Locale
}

// Entry 是註冊表裡一個型別的非泛型介面：參數以 JSON 快照傳進來。
type Entry interface {
	Spec() Spec
	// IdempotencyKey 回傳意圖的冪等鍵：型別名稱加事件來源身分。
	IdempotencyKey(params json.RawMessage) (string, error)
	Render(l uitext.Locale, params json.RawMessage) (Rendered, error)
	// ExampleParams 回傳範例參數的 JSON 與解碼後的值（自檢與固定輸出測試用）。
	ExampleParams() (json.RawMessage, any, error)
	// validate 檢查型別定義本身；回傳全部違規。
	validate() []error
}

// MaxIdempotencyKeyLen 是冪等鍵的長度上限，與 db/schema.sql intents.idempotency_key 的 CHECK 一致。
const MaxIdempotencyKeyLen = 200

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// Define 把一個 Definition 變成註冊表的 Entry。
func Define[P any](d Definition[P]) Entry { return &entry[P]{d: d} }

type entry[P any] struct{ d Definition[P] }

func (e *entry[P]) Spec() Spec {
	d := e.d
	return Spec{
		Name: d.Name, Audience: d.Audience, Category: d.Category, Presentation: d.Presentation, StateHint: d.StateHint,
		Channels: slices.Clone(d.Channels), IdempotencySource: d.IdempotencySource, Aggregatable: d.Aggregatable,
	}
}

func (e *entry[P]) decode(params json.RawMessage) (P, error) {
	var p P
	if err := json.Unmarshal(params, &p); err != nil {
		return p, fmt.Errorf("通知型別 %s 的參數快照解碼失敗：%w", e.d.Name, err)
	}
	return p, nil
}

func (e *entry[P]) IdempotencyKey(params json.RawMessage) (string, error) {
	p, err := e.decode(params)
	if err != nil {
		return "", err
	}
	return e.idempotencyKey(p)
}

func (e *entry[P]) idempotencyKey(p P) (string, error) {
	if e.d.IdempotencyKey == nil {
		return "", fmt.Errorf("通知型別 %s 沒有冪等鍵來源", e.d.Name)
	}
	id := e.d.IdempotencyKey(p)
	if id == "" {
		return "", fmt.Errorf("通知型別 %s 的冪等鍵來源是空的", e.d.Name)
	}
	key := e.d.Name + ":" + id
	if len(key) > MaxIdempotencyKeyLen {
		return "", fmt.Errorf("通知型別 %s 的冪等鍵 %d 字元，超過上限 %d", e.d.Name, len(key), MaxIdempotencyKeyLen)
	}
	return key, nil
}

func (e *entry[P]) Render(l uitext.Locale, params json.RawMessage) (Rendered, error) {
	p, err := e.decode(params)
	if err != nil {
		return Rendered{}, err
	}
	return e.render(l, p)
}

func (e *entry[P]) render(l uitext.Locale, p P) (Rendered, error) {
	d := e.d
	c, err := d.Text(l, p)
	if err != nil {
		return Rendered{}, fmt.Errorf("通知型別 %s 組字失敗：%w", d.Name, err)
	}
	l, err = c.locale()
	if err != nil {
		return Rendered{}, fmt.Errorf("通知型別 %s：%w", d.Name, err)
	}
	r := Rendered{Title: c.Title.Text, Body: c.Body.Text, Presentation: d.Presentation, StateHint: d.StateHint, Locale: l}
	if d.DeepLink != nil {
		r.DeepLink = d.DeepLink(p)
	}
	if d.ImageKey != nil {
		r.ImageKey = d.ImageKey(p)
	}
	if d.AggregationKey != nil {
		r.AggregationKey = d.AggregationKey(p)
	}
	if d.Actors != nil {
		r.ActorAccounts = d.Actors(p)
	}
	return r, nil
}

func (e *entry[P]) ExampleParams() (json.RawMessage, any, error) {
	raw, err := json.Marshal(e.d.Example)
	return raw, e.d.Example, err
}

func (e *entry[P]) validate() []error {
	d := e.d
	var errs []error
	bad := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf("通知型別 %q："+format, append([]any{d.Name}, a...)...))
	}
	if !namePattern.MatchString(d.Name) {
		bad("名稱要是 lower_snake")
	}
	if !slices.Contains(model.Audiences(), d.Audience) {
		bad("受眾 %q 不是 BUYER 或 STORE", d.Audience)
	}
	if !model.ValidCategory(d.Category) {
		bad("偏好分類 %q 不在 D4 清單", d.Category)
	} else if !model.ValidCategoryFor(d.Audience, d.Category) {
		bad("偏好分類 %q 不屬於受眾 %q", d.Category, d.Audience)
	}
	if !slices.Contains(model.Presentations(), d.Presentation) {
		bad("呈現分類 %q 不在 D6 清單", d.Presentation)
	}
	if d.StateHint != model.StateHintNone && !slices.Contains(model.StateHints(), d.StateHint) {
		bad("狀態重讀提示 %q 不在 D6 清單", d.StateHint)
	}
	if len(d.Channels) == 0 {
		bad("沒有宣告渠道")
	}
	seen := map[model.Channel]bool{}
	for _, ch := range d.Channels {
		if !slices.Contains(model.Channels(), ch) {
			bad("渠道 %q 不存在", ch)
		}
		if seen[ch] {
			bad("渠道 %q 重複", ch)
		}
		seen[ch] = true
	}
	if d.IdempotencySource == "" || d.IdempotencyKey == nil {
		bad("沒有冪等鍵來源（IdempotencySource 與 IdempotencyKey 都要有）")
	} else if _, err := e.idempotencyKey(d.Example); err != nil {
		bad("範例參數取不出冪等鍵：%v", err)
	}
	if d.Aggregatable != (d.AggregationKey != nil) {
		bad("聚合鍵只能在宣告可聚合的型別，而且可聚合的型別一定要有聚合鍵（Aggregatable=%v、AggregationKey 有值=%v）", d.Aggregatable, d.AggregationKey != nil)
	} else if d.AggregationKey != nil && d.AggregationKey(d.Example) == "" {
		bad("範例參數的聚合鍵是空的")
	}
	if d.Text == nil {
		bad("沒有文案")
	} else {
		for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
			c, err := d.Text(l, d.Example)
			if err != nil || c.Title.Text == "" || c.Body.Text == "" {
				bad("語系 %s 的範例組不出標題與內文（%v）", l, err)
				continue
			}
			if got, err := c.locale(); err != nil {
				bad("語系 %s 的範例：%v", l, err)
			} else if got != l {
				bad("語系 %s 的範例實際用了 %s；en 沒有文字不能退回中文", l, got)
			}
		}
	}
	return errs
}

// Validate 檢查一個型別定義，回傳全部違規。
func Validate(e Entry) []error { return e.validate() }

var all = []Entry{accountSuspended, accountUnsuspended}

// All 回傳全部型別，順序固定。
func All() []Entry { return slices.Clone(all) }

// Lookup 依名稱找型別。
func Lookup(name string) (Entry, bool) {
	for _, e := range all {
		if e.Spec().Name == name {
			return e, true
		}
	}
	return nil, false
}
