package reviewrelay

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type recordingJobs struct{ args []river.JobArgs }

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	q.args = append(q.args, args)
	return nil
}

func (q *recordingJobs) published() []rabbitmq.PublishArgs {
	var out []rabbitmq.PublishArgs
	for _, a := range q.args {
		if p, ok := a.(rabbitmq.PublishArgs); ok {
			out = append(out, p)
		}
	}
	return out
}

func (q *recordingJobs) relayJobs() []river.JobArgs {
	var out []river.JobArgs
	for _, a := range q.args {
		if _, ok := a.(rabbitmq.PublishArgs); !ok {
			out = append(out, a)
		}
	}
	return out
}

func (q *recordingJobs) publishJobs() []river.JobArgs {
	var out []river.JobArgs
	for _, a := range q.args {
		if _, ok := a.(rabbitmq.PublishArgs); ok {
			out = append(out, a)
		}
	}
	return out
}

type fakeReviews struct {
	review  review.Review
	hidden  bool
	gone    bool
	product map[int64]review.ProductStats
	store   map[int64]review.StoreStats
	targets []review.Target
}

func (f *fakeReviews) ReviewForRelay(context.Context, uuid.UUID) (review.RelayReview, error) {
	if f.gone {
		return review.RelayReview{}, apperr.NotFound("gone")
	}
	return review.RelayReview{Review: f.review, Public: !f.hidden}, nil
}

func (f *fakeReviews) ProductStats(_ context.Context, id int64) (review.ProductStats, error) {
	return f.product[id], nil
}

func (f *fakeReviews) StoreStats(_ context.Context, id int64) (review.StoreStats, error) {
	return f.store[id], nil
}

func (f *fakeReviews) AuthorTargets(context.Context, int64) ([]review.Target, error) {
	return f.targets, nil
}

type fakeBuyers struct{ display map[int64]buyer.Display }

func (b *fakeBuyers) BatchDisplay(_ context.Context, ids []int64) ([]buyer.Display, error) {
	var out []buyer.Display
	for _, id := range ids {
		if d, ok := b.display[id]; ok {
			out = append(out, d)
		}
	}
	return out, nil
}

type fakeStores struct{ stores map[int64]seller.Store }

func (s *fakeStores) Store(_ context.Context, id int64) (seller.Store, error) {
	st, ok := s.stores[id]
	if !ok {
		return seller.Store{}, apperr.NotFound("no store")
	}
	return st, nil
}

var (
	eventID    = uuid.MustParse("01990000-0000-7000-8000-000000000001")
	reviewID   = uuid.MustParse("01990000-0000-7000-8000-0000000000a1")
	occurredAt = time.Date(2026, 11, 20, 9, 30, 15, 123456789, time.FixedZone("TST", 8*3600))
	// timestamp 是 Go time.Time 的 JSON：UTC、RFC3339Nano。
	wireTimestamp = "2026-11-20T01:30:15.123456789Z"
	createdAt     = time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
)

func strp(s string) *string { return &s }

func newFixture() (*Worker, *fakeReviews, *fakeBuyers, *fakeStores) {
	r := &fakeReviews{
		review: review.Review{
			ID: reviewID, OrderNo: "O1", OrderItemID: 11, BuyerID: 301, StoreID: 91, ProductID: 700, Rating: 5, Content: "很甜",
			Images: []review.Image{{AssetID: "a1", ObjectKey: "review/1.jpg"}, {AssetID: "a2", ObjectKey: "review/2.jpg"}}, CreatedAt: createdAt,
		},
		product: map[int64]review.ProductStats{700: {Count: 30, Average: 4.9}},
		store:   map[int64]review.StoreStats{91: {Count: 120, Average: 4.8}},
	}
	b := &fakeBuyers{display: map[int64]buyer.Display{301: {BuyerID: 301, DisplayName: strp("小明"), Avatar: strp("avatars/301")}}}
	s := &fakeStores{stores: map[int64]seller.Store{91: {StoreID: 91, AccountID: 7001}}}
	return NewWorker(r, b, s, &recordingJobs{}, nil), r, b, s
}

// run 把模組事件交給 OnX，取出排入的 job，再交給 worker，回傳送出的訊息。
func run[E any](t *testing.T, w *Worker, on func(jobs.Enqueuer) func(context.Context, E) error, event E) []rabbitmq.PublishArgs {
	t.Helper()
	relayed := &recordingJobs{}
	if err := on(relayed)(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(relayed.args) != 1 {
		t.Fatalf("relay jobs=%d", len(relayed.args))
	}
	published := &recordingJobs{}
	w2 := *w
	w2.Jobs = published
	// 逐個執行排入的 relay job（刪帳會扇出成每組一個 job），直到只剩送出的 PublishArgs。
	pending := relayed.args
	for len(pending) > 0 {
		next := &recordingJobs{}
		w2.Jobs = next
		for _, a := range pending {
			if err := work(&w2, a); err != nil {
				t.Fatal(err)
			}
		}
		published.args = append(published.args, next.publishJobs()...)
		pending = next.relayJobs()
	}
	return published.published()
}

func work(w *Worker, args river.JobArgs) error {
	ctx := context.Background()
	switch a := args.(type) {
	case ReviewCreatedArgs:
		return w.reviewCreated(ctx, a)
	case ReviewAggregateChangedArgs:
		return w.reviewAggregateChanged(ctx, a)
	case ReviewAuthorDeactivatedArgs:
		return w.reviewAuthorDeactivated(ctx, a)
	}
	panic("unknown job args")
}

func body(t *testing.T, a rabbitmq.PublishArgs) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(a.Body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func created() review.ReviewCreated {
	return review.ReviewCreated{EventID: eventID, OccurredAt: occurredAt, ReviewID: reviewID, StoreID: 91, ProductID: 700}
}

func TestReviewCreatedPayload(t *testing.T) {
	w, _, _, _ := newFixture()
	got := run(t, w, OnReviewCreated, created())
	if len(got) != 1 || got[0].Exchange != "hawkr.review" || got[0].RoutingKey != "review.created" || got[0].EventID != eventID.String() {
		t.Fatalf("published = %+v", got)
	}
	m := body(t, got[0])
	want := map[string]any{
		"event_id": eventID.String(), "event_type": "REVIEW_CREATED", "version": "1.0", "timestamp": wireTimestamp, "source": "review-service",
		"review_id": float64(0), "order_no": "O1", "order_item_id": float64(11), "store_id": float64(91), "notify_seller_account_id": float64(7001),
		"buyer_id": float64(301), "rating": float64(5), "content": "很甜", "image_keys": []any{"review/1.jpg", "review/2.jpg"},
		"created_at": "2026-11-20T01:00:00Z", "avg_rating": 4.8, "total_count": float64(120), "product_id": float64(700),
		"product_avg_rating": 4.9, "product_review_count": float64(30), "buyer_nickname": "小明", "buyer_avatar_key": "avatars/301",
	}
	if len(m) != len(want) {
		t.Errorf("keys = %d，預期 %d：%v", len(m), len(want), m)
	}
	for k, v := range want {
		if got := m[k]; !equalJSON(got, v) {
			t.Errorf("%s = %v，預期 %v", k, got, v)
		}
	}
}

func equalJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// 評價已清除（作者刪帳後清除先於 relay job）：不送。
func TestReviewCreatedSkipsAPurgedReview(t *testing.T) {
	w, r, _, _ := newFixture()
	r.gone = true
	if got := run(t, w, OnReviewCreated, created()); len(got) != 0 {
		t.Fatalf("published = %+v", got)
	}
}

// 評價在 job 執行時已被隱藏或作者在刪帳中：不送 review.created（不外送內容、不通知賣家）。
func TestReviewCreatedSkipsAReviewThatIsNoLongerPublic(t *testing.T) {
	w, r, _, _ := newFixture()
	r.hidden = true
	if got := run(t, w, OnReviewCreated, created()); len(got) != 0 {
		t.Fatalf("published = %+v", got)
	}
}

// 刪帳不在一個 job 裡跑完全部：每組各排一個 job，job 數等於組數。
func TestAuthorDeactivatedFansOutOneJobPerPair(t *testing.T) {
	w, r, _, _ := newFixture()
	r.targets = []review.Target{{StoreID: 91, ProductID: 700}, {StoreID: 92, ProductID: 800}}
	q := &recordingJobs{}
	w.Jobs = q
	if err := w.reviewAuthorDeactivated(context.Background(), ReviewAuthorDeactivatedArgs{eventFields{eventID.String(), occurredAt}, 301}); err != nil {
		t.Fatal(err)
	}
	if len(q.args) != 2 || len(q.published()) != 0 {
		t.Fatalf("要排兩個扇出 job、不直接送：%+v", q.args)
	}
	for _, a := range q.args {
		if _, ok := a.(ReviewAggregateChangedArgs); !ok {
			t.Fatalf("job 種類 = %T", a)
		}
	}
}

// 刪帳的買家沒有暱稱與頭像；店家查不到時 notify_seller_account_id 是 0。
func TestReviewCreatedForDeletedBuyerAndMissingStore(t *testing.T) {
	w, _, b, s := newFixture()
	b.display[301] = buyer.Display{BuyerID: 301, Deleted: true}
	delete(s.stores, 91)
	m := body(t, run(t, w, OnReviewCreated, created())[0])
	if _, ok := m["buyer_nickname"]; ok {
		t.Errorf("刪帳買家不帶暱稱：%v", m)
	}
	if _, ok := m["buyer_avatar_key"]; ok {
		t.Errorf("刪帳買家不帶頭像：%v", m)
	}
	if v, ok := m["notify_seller_account_id"]; !ok || v != float64(0) {
		t.Errorf("notify_seller_account_id = %v", v)
	}
}

// 統計在 job 執行當下重算：事件發生到 job 執行之間改變的值，送出的是執行當下的。
func TestStatsAreRecomputedWhenTheJobRuns(t *testing.T) {
	w, r, _, _ := newFixture()
	relayed := &recordingJobs{}
	if err := OnReviewAggregateChanged(relayed)(context.Background(), review.ReviewAggregateChanged{EventID: eventID, OccurredAt: occurredAt, StoreID: 91, ProductID: 700}); err != nil {
		t.Fatal(err)
	}
	r.product[700] = review.ProductStats{Count: 29, Average: 4.88}
	r.store[91] = review.StoreStats{Count: 119, Average: 4.79}
	out := &recordingJobs{}
	w.Jobs = out
	if err := work(w, relayed.args[0]); err != nil {
		t.Fatal(err)
	}
	m := body(t, out.published()[0])
	if out.published()[0].RoutingKey != "review.hidden" || m["product_avg_rating"] != 4.88 || m["product_review_count"] != float64(29) || m["avg_rating"] != 4.79 || m["total_count"] != float64(119) {
		t.Fatalf("payload = %v", m)
	}
	if m["review_id"] != float64(0) || m["store_id"] != float64(91) || m["product_id"] != float64(700) || m["event_type"] != "REVIEW_HIDDEN" {
		t.Fatalf("payload = %v", m)
	}
}

// 沒有任何可見評價時，聚合是 0 而且仍然是 JSON 數字（Java 的 ReviewStatsConsumer 缺欄位或非數字會進 DLQ，不能省略）。
func TestZeroAggregatesAreStillNumbers(t *testing.T) {
	w, r, _, _ := newFixture()
	r.product = map[int64]review.ProductStats{}
	r.store = map[int64]review.StoreStats{}
	m := body(t, run(t, w, OnReviewAggregateChanged, review.ReviewAggregateChanged{EventID: eventID, OccurredAt: occurredAt, StoreID: 91, ProductID: 700})[0])
	for _, k := range []string{"product_avg_rating", "product_review_count", "avg_rating", "total_count"} {
		if v, ok := m[k].(float64); !ok || v != 0 {
			t.Errorf("%s = %#v，預期數字 0", k, m[k])
		}
	}
}

// 刪帳：作者評過的每組 (店, 商品) 各送一則 review.hidden，event_id 彼此不同、由事件 ID 衍生且穩定。
func TestAuthorDeactivatedSendsOneHiddenPerPair(t *testing.T) {
	w, r, _, _ := newFixture()
	r.targets = []review.Target{{StoreID: 91, ProductID: 700}, {StoreID: 91, ProductID: 701}, {StoreID: 92, ProductID: 800}}
	r.product[701] = review.ProductStats{Count: 2, Average: 3.5}
	event := review.ReviewAuthorDeactivated{EventID: eventID, OccurredAt: occurredAt, BuyerID: 301}
	first := run(t, w, OnReviewAuthorDeactivated, event)
	if len(first) != 3 {
		t.Fatalf("三組送三則：%d", len(first))
	}
	seen := map[string]bool{}
	var pairs [][2]float64
	for _, p := range first {
		if p.RoutingKey != "review.hidden" || p.Exchange != "hawkr.review" || seen[p.EventID] || p.EventID == eventID.String() {
			t.Fatalf("每則自己的 event_id：%+v", p)
		}
		seen[p.EventID] = true
		m := body(t, p)
		if m["event_id"] != p.EventID {
			t.Errorf("PublishArgs.EventID 要等於 body 的 event_id：%v vs %s", m["event_id"], p.EventID)
		}
		pairs = append(pairs, [2]float64{m["store_id"].(float64), m["product_id"].(float64)})
		if m["review_id"] != float64(0) {
			t.Errorf("review_id = %v", m["review_id"])
		}
	}
	if !slices.Equal(pairs, [][2]float64{{91, 700}, {91, 701}, {92, 800}}) {
		t.Fatalf("pairs = %v", pairs)
	}
	second := run(t, w, OnReviewAuthorDeactivated, event)
	for i := range first {
		if first[i].EventID != second[i].EventID {
			t.Fatalf("重跑的 event_id 要穩定：%s vs %s", first[i].EventID, second[i].EventID)
		}
	}
}

// 清除已經跑完時沒有任何組可列：不送，也不是錯誤。
func TestAuthorDeactivatedWithNothingLeftSendsNothing(t *testing.T) {
	w, _, _, _ := newFixture()
	if got := run(t, w, OnReviewAuthorDeactivated, review.ReviewAuthorDeactivated{EventID: eventID, OccurredAt: occurredAt, BuyerID: 301}); len(got) != 0 {
		t.Fatalf("published = %+v", got)
	}
}
