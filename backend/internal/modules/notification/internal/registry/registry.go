// Package registry 是通知型別註冊表：每個型別的 typed 參數、受眾、偏好分類（D4）、呈現分類（D6）、渠道、可選的聚合鍵、冪等鍵來源、
// actor、文案、深連結與可選的推播圖片都寫成 Go 程式（Definition），沒有資料表、沒有模板。
//
// 型別只在這裡定義；參數快照存成 JSON（收件匣與意圖），讀取時用型別自己的參數結構解回來再組字。Validate 與 registry_test.go 是
// 註冊表自檢：缺冪等鍵來源、分類不在 D4 清單、呈現分類不在 D6 清單、聚合鍵與可聚合宣告不一致、參數帶第三方名稱欄位都會失敗。
package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"time"
	"uuid"

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
	// Audiences 是這個型別能到的受眾身分面：走意圖管線的型別剛好一個（收件人與受眾一一對應）；Fanout 型別可以有多個。
	Audiences []model.Audience
	// Fanout 宣告型別不經意圖管線：收件人不在參數快照裡，由型別自己的派送 job 逐一展開（公告）。Fanout 型別不需要 IdempotencyKey、OccurredAt 與 Recipient，
	// 也不能被意圖管線登記或展開；收件匣列的事件時間由派送寫入。
	Fanout bool
	// FixedLocale 非空時，型別的文字永遠是這個語系，不依請求語系（例如公告只有英文）；自檢改為要求每個語系都組出這個語系的文字。
	FixedLocale uitext.Locale
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
	// OccurredAt 由參數取出來源事件發生的時間（事件自己的時間，不是處理時間）：聚合合併時以它判斷哪一份參數比較新。
	OccurredAt func(P) time.Time
	// Recipient 由參數取出收件人：帳號 ID（事件來源身分之外，事件的對象帳號也要進參數快照），STORE 受眾另帶 StoreID；BUYER 受眾的 StoreID 一定是 0。
	Recipient func(P) Recipient
	// Expand 在 worker 端解析收件人：事件只帶 store_id、buyer_id、沒有 account_id 的型別用它取代 Recipient（兩者擇一，Expand 與 Recipient、Actors 互斥）。
	// 回傳 nil 表示沒有存活的收件人（買家或店家已關閉），Process 略過；解析結果的形狀不合法是永久錯誤。收件人與 actor 只在這裡解析，
	// 封鎖略過與自我略過由 Process 依回傳的 Expansion 套用。
	Expand func(ctx context.Context, r Resolver, p P) (*Expansion, error)
	// Broadcast 宣告型別是 fan-out 的母意圖（一對多，例如追蹤的店發文）：不能有收件人、文案、渠道與聚合，只有冪等鍵、事件時間與 Broadcast。
	Broadcast *Broadcast[P]
	// Spawned 宣告型別只由 Broadcast 的 fan-out 建立：冪等鍵由派送決定（SpawnedKey），所以沒有 IdempotencyKey、OccurredAt（沿用母意圖的），
	// 也不能被同步訂閱者 Accept。必須有 Expand。
	Spawned bool
	// Aggregatable 宣告型別可聚合（D12：讚、追蹤）；為 true 才可以有 AggregationKey，而且一定要有。
	Aggregatable   bool
	AggregationKey func(P) string
	// Actors 回傳參數裡的 actor 帳號 ID（只存帳號 ID，名稱與頭像讀取時向擁有模組解析）；沒有 actor 的型別留空。
	Actors func(P) []int64
	// Text 依語系組字。需要讀取時解析的內容（參數只存 ID）的型別改用 Load 與 TextLoaded，兩者擇一。
	Text func(l uitext.Locale, p P) (Content, error)
	// Load 在讀取時向擁有資料的地方（Sources）一次解析一批參數裡的 ID（一頁一次查詢），回傳與 ps 等長、同序的結果，交給 TextLoaded 組字；
	// 查不到的 ID 留零值的 Loaded（TextLoaded 會拒絕），讀取本身失敗才回 error。有 Load 就一定要有 TextLoaded，反之亦然。
	Load       func(ctx context.Context, src Sources, ps []P) ([]Loaded, error)
	TextLoaded func(l uitext.Locale, p P, ld Loaded) (Content, error)
	// ExampleLoaded 是範例參數對應的 Load 結果（自檢與固定輸出測試用）。
	ExampleLoaded Loaded
	// DeepLink 回傳深連結；留空表示沒有目的地。
	DeepLink func(P) string
	// ImageKey 回傳推播圖片的物件 key（D14，商品類型別帶）；留空表示沒有圖。
	ImageKey func(P) string
	// Example 是固定輸出測試與註冊表自檢用的範例參數。
	Example P
}

// AnnouncementText 是公告的標題與內文（英文）。
type AnnouncementText struct{ Title, Body string }

// Sources 是型別在讀取時解析參數裡的 ID 要問的地方；由通知模組自己的用例實作（公告在本模組的資料表）。
type Sources interface {
	// Announcements 一次讀多則公告；查不到的 ID 不在結果裡。
	Announcements(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]AnnouncementText, error)
}

// Loaded 是 Load 解析出來的內容；沒有 Load 的型別是零值。
type Loaded struct{ Announcement *AnnouncementText }

// Recipient 是型別從參數取出的收件人：(帳號, 受眾) 的帳號與 STORE 受眾的店。受眾本身是型別的屬性（Spec.Audience）。
type Recipient struct {
	AccountID int64
	StoreID   int64
}

// Resolved 是參數快照裡與投遞有關的事實（不含文字）：worker 用它解析收件人、聚合與略過自己。
type Resolved struct {
	Recipient      Recipient
	AggregationKey string
	ActorAccounts  []int64
}

// Spec 是型別的靜態屬性（不含參數相關的函式）。
type Spec struct {
	Name              string
	Audiences         []model.Audience
	Fanout            bool
	Category          model.PrefCategory
	Presentation      model.Presentation
	StateHint         model.StateHint
	Channels          []model.Channel
	IdempotencySource string
	Aggregatable      bool
	// Broadcast 與 Child：型別是 fan-out 的母意圖與它的子型別名稱。Spawned：型別只由 fan-out 建立。
	Broadcast bool
	Child     string
	Spawned   bool
}

// Audience 回傳走意圖管線的型別的唯一受眾（Validate 保證非 Fanout 型別剛好一個）。
func (s Spec) Audience() model.Audience { return s.Audiences[0] }

// Reaches 回報型別是否能到受眾 a。
func (s Spec) Reaches(a model.Audience) bool { return slices.Contains(s.Audiences, a) }

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
	// IdempotencyKey 回傳意圖的冪等鍵：型別名稱加事件來源身分（超過長度上限時改存雜湊，見 scopedKey）。
	IdempotencyKey(params json.RawMessage) (string, error)
	// OccurredAt 回傳來源事件發生的時間。
	OccurredAt(params json.RawMessage) (time.Time, error)
	// Render 依語系組字；需要 Load 的型別請改用 Load 加 RenderLoaded。
	Render(l uitext.Locale, params json.RawMessage) (Rendered, error)
	// Load 在讀取時一次解析一批參數裡的 ID（沒有 Load 的型別回零值）。回傳與 params 等長：loaded[i] 是第 i 個的結果，rowErrs[i] 非 nil 表示
	// 第 i 個的參數快照解不開（只影響那一列）；err 是讀取失敗（整批失敗，呼叫端不該略過）。
	Load(ctx context.Context, src Sources, params []json.RawMessage) (loaded []Loaded, rowErrs []error, err error)
	// RenderLoaded 以 Load 的結果組字；沒有 Load 的型別等同 Render。
	RenderLoaded(l uitext.Locale, params json.RawMessage, ld Loaded) (Rendered, error)
	// Resolve 由參數快照取出收件人、聚合鍵與 actor，不組字；只適用參數裡已有帳號的型別（沒有 Expand 的型別）。
	Resolve(params json.RawMessage) (Resolved, error)
	// Expand 在 worker 端解析收件人：有 Expand 的型別向 Resolver 讀 buyer、seller，其餘型別等同 Resolve（Self 與 Actors 是空的，不做封鎖檢查）。
	// ok 為 false 表示沒有存活的收件人。錯誤是 ErrBadParams 時永久不合法，其餘是暫時性。fan-out 的母意圖與 Fanout 型別回 ErrBadParams。
	Expand(ctx context.Context, r Resolver, params json.RawMessage) (x Expanded, ok bool, err error)
	// Spawn 分頁展開 fan-out 母意圖的收件人，回傳子型別的參數（每位收件人一筆，依 ID 由小到大）；不是 Broadcast 型別回 ErrBadParams。
	Spawn(ctx context.Context, r Resolver, params json.RawMessage, afterID int64, limit int) ([]Spawned, error)
	// ExampleParams 回傳範例參數的 JSON 與解碼後的值（自檢與固定輸出測試用）。
	ExampleParams() (json.RawMessage, any, error)
	// ExampleLoaded 回傳範例參數對應的 Load 結果（沒有 Load 的型別是零值）。
	ExampleLoaded() Loaded
	// validate 檢查型別定義本身；回傳全部違規。
	validate() []error
}

// Spawned 是 fan-out 展開出來的一位收件人：ID 同 Target.ID，Params 是子型別參數的 JSON。
type Spawned struct {
	ID     int64
	Params json.RawMessage
}

// MaxIdempotencyKeyLen 是冪等鍵與聚合鍵的長度上限，與 db/schema.sql 的 CHECK（intents.idempotency_key、inbox_items.aggregation_key）一致。
const MaxIdempotencyKeyLen = 200

// scopedKey 把型別內的鍵加上型別名稱前綴，不同型別不會撞鍵。超過上限時改存「型別名稱:來源的 sha256 十六進位」（決定性、長度固定，
// 型別名稱最長 63 字元，合計不超過 128），而不是讓寫入時才被 CHECK 擋下。
func scopedKey(typeName, source string) string {
	key := typeName + ":" + source
	if len(key) <= MaxIdempotencyKeyLen {
		return key
	}
	sum := sha256.Sum256([]byte(source))
	return typeName + ":" + hex.EncodeToString(sum[:])
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// Define 把一個 Definition 變成註冊表的 Entry。
func Define[P any](d Definition[P]) Entry { return &entry[P]{d: d} }

type entry[P any] struct{ d Definition[P] }

func (e *entry[P]) Spec() Spec {
	d := e.d
	return Spec{
		Name: d.Name, Audiences: slices.Clone(d.Audiences), Fanout: d.Fanout, Category: d.Category, Presentation: d.Presentation, StateHint: d.StateHint,
		Channels: slices.Clone(d.Channels), IdempotencySource: d.IdempotencySource, Aggregatable: d.Aggregatable,
		Broadcast: d.Broadcast != nil, Child: childName(d.Broadcast), Spawned: d.Spawned,
	}
}

func childName[P any](b *Broadcast[P]) string {
	if b == nil {
		return ""
	}
	return b.Child
}

func (e *entry[P]) decode(params json.RawMessage) (P, error) {
	var p P
	if err := json.Unmarshal(params, &p); err != nil {
		return p, fmt.Errorf("%w：通知型別 %s 的參數快照解碼失敗：%w", ErrBadParams, e.d.Name, err)
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

func (e *entry[P]) notFanout() error {
	if e.d.Fanout {
		return fmt.Errorf("通知型別 %s 是 fan-out 型別，不經意圖管線", e.d.Name)
	}
	return nil
}

func (e *entry[P]) idempotencyKey(p P) (string, error) {
	if err := e.notFanout(); err != nil {
		return "", err
	}
	if e.d.Spawned {
		return "", fmt.Errorf("通知型別 %s 只由 fan-out 建立，冪等鍵由派送決定，不能直接登記", e.d.Name)
	}
	if e.d.IdempotencyKey == nil {
		return "", fmt.Errorf("通知型別 %s 沒有冪等鍵來源", e.d.Name)
	}
	id := e.d.IdempotencyKey(p)
	if id == "" {
		return "", fmt.Errorf("通知型別 %s 的冪等鍵來源是空的", e.d.Name)
	}
	return scopedKey(e.d.Name, id), nil
}

func (e *entry[P]) OccurredAt(params json.RawMessage) (time.Time, error) {
	p, err := e.decode(params)
	if err != nil {
		return time.Time{}, err
	}
	return e.occurredAt(p)
}

func (e *entry[P]) occurredAt(p P) (time.Time, error) {
	if err := e.notFanout(); err != nil {
		return time.Time{}, err
	}
	if e.d.Spawned {
		return time.Time{}, fmt.Errorf("通知型別 %s 只由 fan-out 建立，事件時間沿用母意圖", e.d.Name)
	}
	if e.d.OccurredAt == nil {
		return time.Time{}, fmt.Errorf("通知型別 %s 沒有事件時間來源", e.d.Name)
	}
	t := e.d.OccurredAt(p)
	if t.IsZero() {
		return time.Time{}, fmt.Errorf("通知型別 %s 的事件時間是空的", e.d.Name)
	}
	return t, nil
}

func (e *entry[P]) Render(l uitext.Locale, params json.RawMessage) (Rendered, error) {
	return e.RenderLoaded(l, params, Loaded{})
}

func (e *entry[P]) Load(ctx context.Context, src Sources, params []json.RawMessage) ([]Loaded, []error, error) {
	loaded := make([]Loaded, len(params))
	rowErrs := make([]error, len(params))
	if e.d.Load == nil {
		return loaded, rowErrs, nil
	}
	ps := make([]P, 0, len(params))
	idx := make([]int, 0, len(params))
	for i, raw := range params {
		p, err := e.decode(raw)
		if err != nil {
			rowErrs[i] = err
			continue
		}
		ps = append(ps, p)
		idx = append(idx, i)
	}
	if len(ps) == 0 {
		return loaded, rowErrs, nil
	}
	got, err := e.d.Load(ctx, src, ps)
	if err != nil {
		return nil, nil, fmt.Errorf("通知型別 %s 解析參數失敗：%w", e.d.Name, err)
	}
	if len(got) != len(ps) {
		return nil, nil, fmt.Errorf("通知型別 %s 的 Load 回了 %d 筆，要 %d 筆", e.d.Name, len(got), len(ps))
	}
	for k, i := range idx {
		loaded[i] = got[k]
	}
	return loaded, rowErrs, nil
}

func (e *entry[P]) RenderLoaded(l uitext.Locale, params json.RawMessage, ld Loaded) (Rendered, error) {
	p, err := e.decode(params)
	if err != nil {
		return Rendered{}, err
	}
	return e.render(l, p, ld)
}

func (e *entry[P]) text(l uitext.Locale, p P, ld Loaded) (Content, error) {
	if e.d.TextLoaded != nil {
		return e.d.TextLoaded(l, p, ld)
	}
	if e.d.Text == nil {
		return Content{}, fmt.Errorf("通知型別 %s 沒有文案", e.d.Name)
	}
	return e.d.Text(l, p)
}

func (e *entry[P]) render(l uitext.Locale, p P, ld Loaded) (Rendered, error) {
	d := e.d
	c, err := e.text(l, p, ld)
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
	r.AggregationKey = e.aggregationKey(p)
	if d.Actors != nil {
		r.ActorAccounts = d.Actors(p)
	}
	return r, nil
}

func (e *entry[P]) Resolve(params json.RawMessage) (Resolved, error) {
	p, err := e.decode(params)
	if err != nil {
		return Resolved{}, err
	}
	return e.resolve(p)
}

func (e *entry[P]) resolve(p P) (Resolved, error) {
	d := e.d
	if err := e.notFanout(); err != nil {
		return Resolved{}, err
	}
	if d.Recipient == nil {
		return Resolved{}, fmt.Errorf("通知型別 %s 沒有收件人解析", d.Name)
	}
	r := d.Recipient(p)
	if r.AccountID <= 0 {
		return Resolved{}, fmt.Errorf("通知型別 %s 的參數沒有收件帳號", d.Name)
	}
	if (d.Audiences[0] == model.AudienceStore) != (r.StoreID > 0) || r.StoreID < 0 {
		return Resolved{}, fmt.Errorf("通知型別 %s：受眾 %s 的收件人 StoreID 是 %d（STORE 一定有、BUYER 一定沒有）", d.Name, d.Audiences[0], r.StoreID)
	}
	out := Resolved{Recipient: r}
	out.AggregationKey = e.aggregationKey(p)
	if d.Actors != nil {
		out.ActorAccounts = d.Actors(p)
	}
	return out, nil
}

func (e *entry[P]) Expand(ctx context.Context, r Resolver, params json.RawMessage) (Expanded, bool, error) {
	p, err := e.decode(params)
	if err != nil {
		return Expanded{}, false, err
	}
	d := e.d
	if d.Expand == nil {
		res, err := e.resolve(p)
		if err != nil {
			return Expanded{}, false, fmt.Errorf("%w：%w", ErrBadParams, err)
		}
		return Expanded{Resolved: res}, true, nil
	}
	if err := e.notFanout(); err != nil {
		return Expanded{}, false, fmt.Errorf("%w：%w", ErrBadParams, err)
	}
	x, err := d.Expand(ctx, r, p)
	if err != nil {
		return Expanded{}, false, err
	}
	if x == nil {
		return Expanded{}, false, nil
	}
	if err := x.validate(d.Audiences[0]); err != nil {
		return Expanded{}, false, fmt.Errorf("通知型別 %s：%w", d.Name, err)
	}
	out := Expanded{Resolved: Resolved{Recipient: x.Recipient, AggregationKey: e.aggregationKey(p)}, Actors: x.Actors}
	out.Self = Principal{Side: d.Audiences[0], ID: x.BuyerID}
	if d.Audiences[0] == model.AudienceStore {
		out.Self.ID = x.Recipient.StoreID
	}
	for _, a := range x.Actors {
		if a.AccountID > 0 {
			out.ActorAccounts = append(out.ActorAccounts, a.AccountID)
		}
	}
	return out, true, nil
}

func (e *entry[P]) Spawn(ctx context.Context, r Resolver, params json.RawMessage, afterID int64, limit int) ([]Spawned, error) {
	p, err := e.decode(params)
	if err != nil {
		return nil, err
	}
	b := e.d.Broadcast
	if b == nil {
		return nil, fmt.Errorf("%w：通知型別 %s 不是 fan-out 母意圖", ErrBadParams, e.d.Name)
	}
	targets, err := b.Targets(ctx, r, p, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("通知型別 %s 展開收件人失敗：%w", e.d.Name, err)
	}
	out := make([]Spawned, len(targets))
	prev := afterID
	for i, t := range targets {
		if t.ID <= prev {
			return nil, fmt.Errorf("%w：通知型別 %s 的收件人 ID 要大於游標且遞增（%d 之後是 %d）", ErrBadParams, e.d.Name, prev, t.ID)
		}
		prev = t.ID
		raw, err := json.Marshal(t.Params)
		if err != nil {
			return nil, fmt.Errorf("%w：通知型別 %s 的子參數無法序列化：%w", ErrBadParams, e.d.Name, err)
		}
		out[i] = Spawned{ID: t.ID, Params: raw}
	}
	return out, nil
}

// aggregationKey 是儲存用的聚合鍵：型別內的鍵加型別名稱前綴（兩個型別的同名鍵不會合併成一列），沒有聚合鍵時是空字串。
func (e *entry[P]) aggregationKey(p P) string {
	if e.d.AggregationKey == nil {
		return ""
	}
	k := e.d.AggregationKey(p)
	if k == "" {
		return ""
	}
	return scopedKey(e.d.Name, k)
}

func (e *entry[P]) ExampleParams() (json.RawMessage, any, error) {
	raw, err := json.Marshal(e.d.Example)
	return raw, e.d.Example, err
}

func (e *entry[P]) ExampleLoaded() Loaded { return e.d.ExampleLoaded }

func (e *entry[P]) validate() []error {
	d := e.d
	var errs []error
	bad := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf("通知型別 %q："+format, append([]any{d.Name}, a...)...))
	}
	if !namePattern.MatchString(d.Name) {
		bad("名稱要是 lower_snake")
	}
	if d.Broadcast != nil {
		return append(errs, e.validateBroadcast()...)
	}
	audiencesOK := len(d.Audiences) > 0 && (d.Fanout || len(d.Audiences) == 1)
	if !audiencesOK {
		bad("走意圖管線的型別剛好一個受眾，fan-out 型別至少一個（現在 %v）", d.Audiences)
	}
	for i, a := range d.Audiences {
		if !slices.Contains(model.Audiences(), a) {
			bad("受眾 %q 不是 BUYER 或 STORE", a)
			audiencesOK = false
		} else if slices.Contains(d.Audiences[:i], a) {
			bad("受眾 %q 重複", a)
			audiencesOK = false
		}
	}
	if !model.ValidCategory(d.Category) {
		bad("偏好分類 %q 不在 D4 清單", d.Category)
	} else {
		for _, a := range d.Audiences {
			if !model.ValidCategoryFor(a, d.Category) {
				bad("偏好分類 %q 不屬於受眾 %q", d.Category, a)
			}
		}
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
	switch {
	case d.Fanout:
		if d.IdempotencyKey != nil || d.OccurredAt != nil || d.Recipient != nil || d.Expand != nil || d.Spawned || d.Aggregatable {
			bad("fan-out 型別不經意圖管線，不能有冪等鍵、事件時間、收件人、Expand、Spawned 或聚合")
		}
	case d.Spawned:
		if d.Expand == nil {
			bad("Spawned 型別的收件人在 worker 端解析，一定要有 Expand")
		}
		if d.IdempotencyKey != nil || d.OccurredAt != nil || d.IdempotencySource != "" || d.Recipient != nil || d.Actors != nil {
			bad("Spawned 型別的冪等鍵由派送決定、事件時間沿用母意圖，不能有 IdempotencyKey、IdempotencySource、OccurredAt、Recipient 或 Actors")
		}
	default:
		if d.IdempotencySource == "" || d.IdempotencyKey == nil {
			bad("沒有冪等鍵來源（IdempotencySource 與 IdempotencyKey 都要有）")
		} else if _, err := e.idempotencyKey(d.Example); err != nil {
			bad("範例參數取不出冪等鍵：%v", err)
		}
		if _, err := e.occurredAt(d.Example); err != nil {
			bad("範例參數取不出事件時間：%v", err)
		}
		switch {
		case d.Expand != nil:
			if d.Recipient != nil || d.Actors != nil {
				bad("有 Expand 的型別收件人與 actor 都在 worker 端解析，不能同時有 Recipient 或 Actors")
			}
		case audiencesOK:
			if _, err := e.resolve(d.Example); err != nil {
				bad("範例參數解析不出收件人：%v", err)
			}
		}
	}
	if d.Aggregatable != (d.AggregationKey != nil) {
		bad("聚合鍵只能在宣告可聚合的型別，而且可聚合的型別一定要有聚合鍵（Aggregatable=%v、AggregationKey 有值=%v）", d.Aggregatable, d.AggregationKey != nil)
	} else if d.AggregationKey != nil && d.AggregationKey(d.Example) == "" {
		bad("範例參數的聚合鍵是空的")
	}
	switch {
	case (d.Text == nil) == (d.TextLoaded == nil):
		bad("Text 與 TextLoaded 要有且只有一個")
	case (d.Load != nil) != (d.TextLoaded != nil):
		bad("Load 與 TextLoaded 要成對")
	default:
		for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
			want := l
			if d.FixedLocale != "" {
				want = d.FixedLocale
			}
			c, err := e.text(l, d.Example, d.ExampleLoaded)
			if err != nil || c.Title.Text == "" || c.Body.Text == "" {
				bad("語系 %s 的範例組不出標題與內文（%v）", l, err)
				continue
			}
			if got, err := c.locale(); err != nil {
				bad("語系 %s 的範例：%v", l, err)
			} else if got != want {
				bad("語系 %s 的範例實際用了 %s，預期 %s；en 沒有文字不能退回中文", l, got, want)
			}
		}
	}
	return errs
}

// validateBroadcast 檢查 fan-out 母意圖：只有冪等鍵、事件時間與 Broadcast，沒有收件人、文案、渠道與聚合。
func (e *entry[P]) validateBroadcast() []error {
	d := e.d
	var errs []error
	bad := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf("通知型別 %q："+format, append([]any{d.Name}, a...)...))
	}
	if d.IdempotencySource == "" || d.IdempotencyKey == nil {
		bad("沒有冪等鍵來源（IdempotencySource 與 IdempotencyKey 都要有）")
	} else if _, err := e.idempotencyKey(d.Example); err != nil {
		bad("範例參數取不出冪等鍵：%v", err)
	}
	if _, err := e.occurredAt(d.Example); err != nil {
		bad("範例參數取不出事件時間：%v", err)
	}
	if !namePattern.MatchString(d.Broadcast.Child) || d.Broadcast.Child == d.Name {
		bad("子型別名稱 %q 不合法（要是另一個 lower_snake 型別）", d.Broadcast.Child)
	}
	if d.Broadcast.Targets == nil {
		bad("fan-out 沒有 Targets")
	}
	if d.Fanout || d.Spawned || d.Recipient != nil || d.Expand != nil || d.Actors != nil || d.Aggregatable || d.AggregationKey != nil ||
		len(d.Audiences) > 0 || len(d.Channels) > 0 || d.Text != nil || d.TextLoaded != nil || d.Load != nil {
		bad("fan-out 母意圖不產生收件匣列，不能有受眾、渠道、文案、收件人、Expand、actor、聚合、Fanout 或 Spawned")
	}
	return errs
}

// Validate 檢查一個型別定義，回傳全部違規。
func Validate(e Entry) []error { return e.validate() }

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

// Registry 是正式的型別註冊表（All、Lookup 的方法形式，給需要注入註冊表的呼叫端）。
type Registry struct{}

// Lookup 依名稱找型別。
func (Registry) Lookup(name string) (Entry, bool) { return Lookup(name) }
