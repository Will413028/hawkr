package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

type fakeResolver struct {
	buyers    map[int64]int64
	stores    map[int64]int64
	followers []FollowerRef
	reporters map[uuid.UUID][]ReporterRef
}

func (f fakeResolver) BuyerAccounts(_ context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	for _, id := range ids {
		if a, ok := f.buyers[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

func (f fakeResolver) StoreAccounts(_ context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	for _, id := range ids {
		if a, ok := f.stores[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

func (fakeResolver) BuyersBlockedBetween(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (fakeResolver) BuyerBlocksStore(context.Context, int64, int64) (bool, error) { return false, nil }

func (fakeResolver) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) { return false, nil }

func (f fakeResolver) CaseReporters(_ context.Context, caseID uuid.UUID, afterRank int64, limit int) ([]ReporterRef, error) {
	var out []ReporterRef
	for _, r := range f.reporters[caseID] {
		if r.Rank > afterRank && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f fakeResolver) StoreFollowers(_ context.Context, _, after int64, limit int, _ time.Time) ([]FollowerRef, error) {
	var out []FollowerRef
	for _, r := range f.followers {
		if r.BuyerID > after && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

type expandParams struct {
	BuyerID int64  `json:"buyer_id"`
	StoreID int64  `json:"store_id"`
	EventID string `json:"EventID"`
}

func expandText(l uitext.Locale, _ expandParams) (Content, error) {
	return Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
}

var fixedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func buyerExpandDef() Definition[expandParams] {
	return Definition[expandParams]{
		Name: "expand_buyer", Audiences: []model.Audience{model.AudienceBuyer}, Category: model.CategoryLike, Presentation: model.PresentationSocial,
		Channels:          []model.Channel{model.ChannelInbox},
		IdempotencySource: "test.Event.EventID",
		IdempotencyKey:    func(p expandParams) string { return p.EventID },
		OccurredAt:        func(expandParams) time.Time { return fixedAt },
		Expand: func(ctx context.Context, r Resolver, p expandParams) (*Expansion, error) {
			return BuyerRecipient(ctx, r, p.BuyerID, Principal{Side: model.AudienceStore, ID: p.StoreID})
		},
		Text:    expandText,
		Example: expandParams{BuyerID: 1, StoreID: 2, EventID: "e"},
	}
}

func spawnedDef() Definition[expandParams] {
	return Definition[expandParams]{
		Name: "expand_child", Audiences: []model.Audience{model.AudienceBuyer}, Category: model.CategoryLike, Presentation: model.PresentationSocial,
		Channels: []model.Channel{model.ChannelInbox}, Spawned: true,
		Expand: func(ctx context.Context, r Resolver, p expandParams) (*Expansion, error) {
			return BuyerRecipient(ctx, r, p.BuyerID, Principal{Side: model.AudienceStore, ID: p.StoreID})
		},
		Text:    expandText,
		Example: expandParams{BuyerID: 1, StoreID: 2},
	}
}

func broadcastDef() Definition[expandParams] {
	return Definition[expandParams]{
		Name:              "expand_parent",
		IdempotencySource: "test.Event.EventID",
		IdempotencyKey:    func(p expandParams) string { return p.EventID },
		OccurredAt:        func(expandParams) time.Time { return fixedAt },
		Broadcast: &Broadcast[expandParams]{
			Child: "expand_child",
			Targets: func(ctx context.Context, r Resolver, p expandParams, req PageRequest) (TargetPage, error) {
				page, err := r.StoreFollowers(ctx, p.StoreID, req.After, req.Limit, req.OccurredAt)
				if err != nil {
					return TargetPage{}, err
				}
				out := make([]Target, len(page))
				for i, f := range page {
					out[i] = Target{ID: f.BuyerID, Params: expandParams{BuyerID: f.BuyerID, StoreID: p.StoreID}}
				}
				next, done := FollowersCursor(page, req.After, req.Limit)
				return TargetPage{Targets: out, Next: next, Done: done}, nil
			},
		},
		Example: expandParams{StoreID: 2, EventID: "e"},
	}
}

func TestExpandResolvesRecipientSelfAndActorAccounts(t *testing.T) {
	t.Parallel()
	e := Define(buyerExpandDef())
	r := fakeResolver{buyers: map[int64]int64{5: 105}, stores: map[int64]int64{9: 209}}
	raw := json.RawMessage(`{"buyer_id":5,"store_id":9,"EventID":"e1"}`)
	x, ok, err := e.Expand(context.Background(), r, raw)
	if err != nil || !ok {
		t.Fatalf("Expand = %v %v", ok, err)
	}
	if x.Recipient != (Recipient{AccountID: 105}) || x.Self != (Principal{Side: model.AudienceBuyer, ID: 5}) {
		t.Errorf("收件人或 Self 不對：%+v", x)
	}
	if len(x.Actors) != 1 || x.Actors[0].AccountID != 209 || len(x.ActorAccounts) != 1 || x.ActorAccounts[0] != 209 {
		t.Errorf("actor 不對：%+v", x)
	}
}

func TestExpandHasNoRecipientWhenTheBuyerIsClosed(t *testing.T) {
	t.Parallel()
	e := Define(buyerExpandDef())
	_, ok, err := e.Expand(context.Background(), fakeResolver{}, json.RawMessage(`{"buyer_id":5,"store_id":9,"EventID":"e1"}`))
	if err != nil || ok {
		t.Fatalf("Expand = %v %v，預期沒有收件人且沒有錯誤", ok, err)
	}
}

func TestExpandKeepsAnUnresolvedActorForBlockChecks(t *testing.T) {
	t.Parallel()
	e := Define(buyerExpandDef())
	r := fakeResolver{buyers: map[int64]int64{5: 105}}
	x, ok, err := e.Expand(context.Background(), r, json.RawMessage(`{"buyer_id":5,"store_id":9,"EventID":"e1"}`))
	if err != nil || !ok {
		t.Fatalf("Expand = %v %v", ok, err)
	}
	if len(x.Actors) != 1 || x.Actors[0].AccountID != 0 || len(x.ActorAccounts) != 0 {
		t.Errorf("查不到帳號的 actor 要留在 Actors（封鎖用）、不進 ActorAccounts（自我略過與收件匣用）：%+v", x)
	}
}

func TestExpandOfATypeWithoutExpandMatchesResolve(t *testing.T) {
	t.Parallel()
	e, _ := Lookup("account_suspended")
	raw, _, err := e.ExampleParams()
	if err != nil {
		t.Fatal(err)
	}
	want, err := e.Resolve(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := e.Expand(context.Background(), nil, raw)
	if err != nil || !ok || got.Recipient != want.Recipient || got.Self != (Principal{}) || len(got.Actors) != 0 {
		t.Fatalf("Expand = %+v %v %v，預期等同 Resolve %+v", got, ok, err, want)
	}
}

func TestExpandRejectsAnInvalidShapeAsPermanent(t *testing.T) {
	t.Parallel()
	bad := map[string]*Expansion{
		"BUYER 沒有 buyer_id": {Recipient: Recipient{AccountID: 1}},
		"BUYER 帶 store_id":  {Recipient: Recipient{AccountID: 1, StoreID: 3}, BuyerID: 2},
		"沒有帳號":              {BuyerID: 2},
		"actor 身分面不合法":      {Recipient: Recipient{AccountID: 1}, BuyerID: 2, Actors: []Actor{{Principal: Principal{Side: "SELLER", ID: 1}}}},
		"actor 沒有 ID":       {Recipient: Recipient{AccountID: 1}, BuyerID: 2, Actors: []Actor{{Principal: Principal{Side: model.AudienceStore}}}},
	}
	for name, x := range bad {
		d := buyerExpandDef()
		d.Expand = func(context.Context, Resolver, expandParams) (*Expansion, error) { return x, nil }
		_, _, err := Define(d).Expand(context.Background(), fakeResolver{}, json.RawMessage(`{}`))
		if !errors.Is(err, ErrBadParams) {
			t.Errorf("%s：%v，預期 ErrBadParams", name, err)
		}
	}
	d := buyerExpandDef()
	d.Audiences = []model.Audience{model.AudienceStore}
	d.Category = model.CategoryOrder
	d.Presentation = model.PresentationOrder
	d.Expand = func(context.Context, Resolver, expandParams) (*Expansion, error) {
		return &Expansion{Recipient: Recipient{AccountID: 1}}, nil
	}
	if _, _, err := Define(d).Expand(context.Background(), fakeResolver{}, json.RawMessage(`{}`)); !errors.Is(err, ErrBadParams) {
		t.Errorf("STORE 沒有 store_id：%v，預期 ErrBadParams", err)
	}
}

func TestExpandPassesResolverErrorsThroughAsTransient(t *testing.T) {
	t.Parallel()
	boom := errors.New("db down")
	d := buyerExpandDef()
	d.Expand = func(context.Context, Resolver, expandParams) (*Expansion, error) { return nil, boom }
	_, _, err := Define(d).Expand(context.Background(), fakeResolver{}, json.RawMessage(`{}`))
	if !errors.Is(err, boom) || errors.Is(err, ErrBadParams) {
		t.Fatalf("%v，預期原錯誤且不是 ErrBadParams（會被重試）", err)
	}
}

func TestSpawnedKeyIsPerParentAndPerRecipient(t *testing.T) {
	t.Parallel()
	a := SpawnedKey("child", "parent:1", 7)
	if a != SpawnedKey("child", "parent:1", 7) {
		t.Error("同一個母意圖與收件人的鍵要決定性")
	}
	if a == SpawnedKey("child", "parent:1", 8) {
		t.Error("不同收件人的鍵不能相同")
	}
	if a == SpawnedKey("child", "parent:2", 7) {
		t.Error("不同母意圖的鍵不能相同")
	}
	if a == SpawnedKey("other", "parent:1", 7) {
		t.Error("不同子型別的鍵不能相同")
	}
	if long := SpawnedKey("child", strings.Repeat("x", 300), 7); len(long) > MaxIdempotencyKeyLen {
		t.Errorf("鍵長 %d 超過上限", len(long))
	}
	if SpawnedKey("child", strings.Repeat("x", 300), 7) == SpawnedKey("child", strings.Repeat("x", 300), 8) {
		t.Error("過長改存雜湊時收件人仍要進鍵")
	}
}

func TestSpawnPagesTargetsAndMarshalsChildParams(t *testing.T) {
	t.Parallel()
	e := Define(broadcastDef())
	r := fakeResolver{followers: []FollowerRef{{BuyerID: 3, AccountID: 103}, {BuyerID: 4, AccountID: 104}, {BuyerID: 9, AccountID: 109}}}
	raw := json.RawMessage(`{"store_id":2,"EventID":"e"}`)
	page, err := e.Spawn(context.Background(), r, raw, PageRequest{After: 3, Limit: 5, OccurredAt: fixedAt})
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != 4 || page.Items[1].ID != 9 || !page.Done {
		t.Fatalf("Spawn = %+v %v", page, err)
	}
	var child expandParams
	if err := json.Unmarshal(page.Items[0].Params, &child); err != nil || child.BuyerID != 4 || child.StoreID != 2 {
		t.Fatalf("子參數 %s：%v", page.Items[0].Params, err)
	}
	full, err := e.Spawn(context.Background(), r, raw, PageRequest{After: 0, Limit: 2, OccurredAt: fixedAt})
	if err != nil || full.Done || full.Next != 4 || len(full.Items) != 2 {
		t.Fatalf("滿頁 = %+v %v，預期游標 4 且未結束", full, err)
	}
}

func filteringBroadcast(drop func(int64) bool) Definition[expandParams] {
	d := broadcastDef()
	d.Broadcast.Targets = func(ctx context.Context, r Resolver, p expandParams, req PageRequest) (TargetPage, error) {
		page, err := r.StoreFollowers(ctx, p.StoreID, req.After, req.Limit, req.OccurredAt)
		if err != nil {
			return TargetPage{}, err
		}
		var out []Target
		for _, f := range page {
			if !drop(f.BuyerID) {
				out = append(out, Target{ID: f.BuyerID, Params: expandParams{BuyerID: f.BuyerID, StoreID: p.StoreID}})
			}
		}
		next, done := FollowersCursor(page, req.After, req.Limit)
		return TargetPage{Targets: out, Next: next, Done: done}, nil
	}
	return d
}

// 型別濾掉一整頁的列時，游標仍沿底層前進、結束只由底層決定。
func TestSpawnCursorAndDoneComeFromTheUnderlyingPageNotTheTargets(t *testing.T) {
	t.Parallel()
	e := Define(filteringBroadcast(func(id int64) bool { return id < 5 }))
	r := fakeResolver{followers: []FollowerRef{{BuyerID: 1, AccountID: 101}, {BuyerID: 2, AccountID: 102}, {BuyerID: 7, AccountID: 107}}}
	page, err := e.Spawn(context.Background(), r, json.RawMessage(`{"store_id":2,"EventID":"e"}`), PageRequest{Limit: 2, OccurredAt: fixedAt})
	if err != nil || len(page.Items) != 0 || page.Done || page.Next != 2 {
		t.Fatalf("Spawn = %+v %v，預期沒有收件人、未結束、游標 2", page, err)
	}
}

func TestSpawnRejectsAnInvalidCursorOrPageRequest(t *testing.T) {
	t.Parallel()
	page := func(tp TargetPage) Definition[expandParams] {
		d := broadcastDef()
		d.Broadcast.Targets = func(context.Context, Resolver, expandParams, PageRequest) (TargetPage, error) { return tp, nil }
		return d
	}
	cases := map[string]struct {
		d   Definition[expandParams]
		req PageRequest
	}{
		"游標沒有前進":     {page(TargetPage{Next: 3}), PageRequest{After: 3, Limit: 5}},
		"游標落在最後一個之前": {page(TargetPage{Targets: []Target{{ID: 9, Params: 1}}, Next: 4}), PageRequest{After: 3, Limit: 5}},
		"ID 重複":      {page(TargetPage{Targets: []Target{{ID: 5, Params: 1}, {ID: 5, Params: 2}}, Next: 5}), PageRequest{Limit: 10}},
		"一頁超過要求":     {page(TargetPage{Targets: []Target{{ID: 1, Params: 1}, {ID: 2, Params: 2}}, Done: true}), PageRequest{Limit: 1}},
		"limit 超過上限": {broadcastDef(), PageRequest{Limit: MaxFollowerPage + 1}},
		"limit 0":    {broadcastDef(), PageRequest{}},
		"游標為負":       {broadcastDef(), PageRequest{After: -1, Limit: 1}},
		"非母意圖":       {buyerExpandDef(), PageRequest{Limit: 10}},
		"Targets 回 InvalidInput": {func() Definition[expandParams] {
			d := broadcastDef()
			d.Broadcast.Targets = func(context.Context, Resolver, expandParams, PageRequest) (TargetPage, error) {
				return TargetPage{}, apperr.InvalidInput("分頁參數不合法")
			}
			return d
		}(), PageRequest{Limit: 10}},
	}
	for name, c := range cases {
		if _, err := Define(c.d).Spawn(context.Background(), fakeResolver{}, json.RawMessage(`{}`), c.req); !errors.Is(err, ErrBadParams) {
			t.Errorf("%s：%v，預期 ErrBadParams", name, err)
		}
	}
}

func TestSpawnPassesTransientTargetErrorsThrough(t *testing.T) {
	t.Parallel()
	boom := errors.New("db down")
	d := broadcastDef()
	d.Broadcast.Targets = func(context.Context, Resolver, expandParams, PageRequest) (TargetPage, error) {
		return TargetPage{}, boom
	}
	_, err := Define(d).Spawn(context.Background(), fakeResolver{}, json.RawMessage(`{}`), PageRequest{Limit: 10})
	if !errors.Is(err, boom) || errors.Is(err, ErrBadParams) {
		t.Fatalf("%v，預期原錯誤且不是 ErrBadParams（會被重試）", err)
	}
}

// ID 小於等於 0 在呼叫 Resolver 之前就是永久錯誤；依賴模組回的 InvalidInput 也是永久錯誤（不是重試 25 次的暫時性錯誤）。
func TestNonPositiveIDsAndInvalidInputAreBadParams(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, id := range []int64{0, -3} {
		if _, err := BuyerRecipient(ctx, fakeResolver{}, id); !errors.Is(err, ErrBadParams) {
			t.Errorf("BuyerRecipient(%d)：%v，預期 ErrBadParams", id, err)
		}
		if _, err := StoreRecipient(ctx, fakeResolver{}, id); !errors.Is(err, ErrBadParams) {
			t.Errorf("StoreRecipient(%d)：%v，預期 ErrBadParams", id, err)
		}
		if _, err := ResolveActors(ctx, fakeResolver{}, Principal{Side: model.AudienceStore, ID: id}); !errors.Is(err, ErrBadParams) {
			t.Errorf("ResolveActors(%d)：%v，預期 ErrBadParams", id, err)
		}
	}
	d := buyerExpandDef()
	d.Expand = func(context.Context, Resolver, expandParams) (*Expansion, error) {
		return nil, fmt.Errorf("解析收件人：%w", apperr.InvalidInput("id 不合法"))
	}
	if _, _, err := Define(d).Expand(ctx, fakeResolver{}, json.RawMessage(`{}`)); !errors.Is(err, ErrBadParams) {
		t.Errorf("Expand 回 InvalidInput：%v，預期 ErrBadParams", err)
	}
	d.Expand = func(context.Context, Resolver, expandParams) (*Expansion, error) { return nil, apperr.NotFound("x") }
	if _, _, err := Define(d).Expand(ctx, fakeResolver{}, json.RawMessage(`{}`)); errors.Is(err, ErrBadParams) {
		t.Errorf("Expand 回 NotFound 不是永久不合法：%v", err)
	}
}

func TestSpawnRejectsNonIncreasingIDs(t *testing.T) {
	t.Parallel()
	d := broadcastDef()
	d.Broadcast.Targets = func(context.Context, Resolver, expandParams, PageRequest) (TargetPage, error) {
		return TargetPage{Targets: []Target{{ID: 5, Params: 1}, {ID: 5, Params: 2}}, Next: 5}, nil
	}
	if _, err := Define(d).Spawn(context.Background(), fakeResolver{}, json.RawMessage(`{}`), PageRequest{Limit: 10}); !errors.Is(err, ErrBadParams) {
		t.Fatalf("%v，預期 ErrBadParams（重複的 ID 會讓游標原地踏步）", err)
	}
	if _, err := Define(buyerExpandDef()).Spawn(context.Background(), fakeResolver{}, json.RawMessage(`{}`), PageRequest{Limit: 10}); !errors.Is(err, ErrBadParams) {
		t.Fatalf("非母意圖：%v，預期 ErrBadParams", err)
	}
}

func TestSpawnedTypesCannotBeAcceptedDirectly(t *testing.T) {
	t.Parallel()
	e := Define(spawnedDef())
	if _, err := e.IdempotencyKey(json.RawMessage(`{}`)); err == nil {
		t.Error("Spawned 型別的冪等鍵由派送決定，不能經 Accept 登記")
	}
	if _, err := e.OccurredAt(json.RawMessage(`{}`)); err == nil {
		t.Error("Spawned 型別沒有自己的事件時間")
	}
}

func TestSelfCheckAcceptsExpandSpawnedAndBroadcastTypes(t *testing.T) {
	t.Parallel()
	for name, d := range map[string]Definition[expandParams]{"expand": buyerExpandDef(), "spawned": spawnedDef(), "broadcast": broadcastDef()} {
		if errs := Validate(Define(d)); len(errs) != 0 {
			t.Errorf("%s：%v", name, errs)
		}
	}
}

func TestSelfCheckRejectsBrokenResolutionDefinitions(t *testing.T) {
	t.Parallel()
	recipient := func(expandParams) Recipient { return Recipient{AccountID: 1} }
	cases := map[string]func() Definition[expandParams]{
		"Expand 與 Recipient 並存": func() Definition[expandParams] { d := buyerExpandDef(); d.Recipient = recipient; return d },
		"Expand 與 Actors 並存": func() Definition[expandParams] {
			d := buyerExpandDef()
			d.Actors = func(expandParams) []int64 { return nil }
			return d
		},
		"Spawned 沒有 Expand": func() Definition[expandParams] { d := spawnedDef(); d.Expand = nil; return d },
		"Spawned 帶冪等鍵": func() Definition[expandParams] {
			d := spawnedDef()
			d.IdempotencyKey = func(expandParams) string { return "k" }
			return d
		},
		"Spawned 帶 Recipient": func() Definition[expandParams] { d := spawnedDef(); d.Recipient = recipient; return d },
		"母意圖帶受眾": func() Definition[expandParams] {
			d := broadcastDef()
			d.Audiences = []model.Audience{model.AudienceBuyer}
			return d
		},
		"母意圖帶文案":        func() Definition[expandParams] { d := broadcastDef(); d.Text = expandText; return d },
		"母意圖沒有 Targets": func() Definition[expandParams] { d := broadcastDef(); d.Broadcast.Targets = nil; return d },
		"母意圖帶偏好分類":      func() Definition[expandParams] { d := broadcastDef(); d.Category = model.CategoryLike; return d },
		"母意圖帶呈現分類": func() Definition[expandParams] {
			d := broadcastDef()
			d.Presentation = model.PresentationSocial
			return d
		},
		"母意圖帶狀態提示": func() Definition[expandParams] {
			d := broadcastDef()
			d.StateHint = model.StateHints()[0]
			return d
		},
		"母意圖帶深連結": func() Definition[expandParams] {
			d := broadcastDef()
			d.DeepLink = func(expandParams) string { return "x" }
			return d
		},
		"Spawned 的範例展開不出收件人": func() Definition[expandParams] {
			d := spawnedDef()
			d.Expand = func(context.Context, Resolver, expandParams) (*Expansion, error) { return nil, nil }
			return d
		},
		"Spawned 的範例 ID 不合法": func() Definition[expandParams] { d := spawnedDef(); d.Example = expandParams{}; return d },
		"Expand 的範例 ID 不合法":  func() Definition[expandParams] { d := buyerExpandDef(); d.Example = expandParams{}; return d },
		"母意圖子型別是自己":          func() Definition[expandParams] { d := broadcastDef(); d.Broadcast.Child = d.Name; return d },
		"母意圖沒有冪等鍵": func() Definition[expandParams] {
			d := broadcastDef()
			d.IdempotencyKey = nil
			d.IdempotencySource = ""
			return d
		},
	}
	for name, build := range cases {
		if errs := Validate(Define(build())); len(errs) == 0 {
			t.Errorf("%s：自檢應該失敗", name)
		}
	}
}

func TestEveryBroadcastChildIsASpawnedRegisteredType(t *testing.T) {
	t.Parallel()
	for _, e := range All() {
		s := e.Spec()
		if !s.Broadcast {
			continue
		}
		child, ok := Lookup(s.Child)
		if !ok || !child.Spec().Spawned {
			t.Errorf("%s 的子型別 %q 沒有註冊或不是 Spawned", s.Name, s.Child)
		}
	}
	for _, e := range All() {
		if e.Spec().Spawned && !spawnedByABroadcast(e.Spec().Name) {
			t.Errorf("%s 是 Spawned，卻沒有任何母意圖指向它", e.Spec().Name)
		}
	}
}

func spawnedByABroadcast(name string) bool {
	for _, e := range All() {
		if e.Spec().Broadcast && e.Spec().Child == name {
			return true
		}
	}
	return false
}
