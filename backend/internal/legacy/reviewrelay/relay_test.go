package reviewrelay

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
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
	product map[uuid.UUID]review.ProductStats
	store   map[int64]review.StoreStats
}

func (f *fakeReviews) ReviewForRelay(context.Context, uuid.UUID) (review.RelayReview, error) {
	if f.gone {
		return review.RelayReview{}, apperr.NotFound("gone")
	}
	return review.RelayReview{Review: f.review, Public: !f.hidden}, nil
}

func (f *fakeReviews) ProductStats(_ context.Context, id uuid.UUID) (review.ProductStats, error) {
	return f.product[id], nil
}

func (f *fakeReviews) StoreStats(_ context.Context, id int64) (review.StoreStats, error) {
	return f.store[id], nil
}

var (
	eventID       = uuid.MustParse("01990000-0000-7000-8000-000000000001")
	reviewID      = uuid.MustParse("01990000-0000-7000-8000-0000000000a1")
	product700    = uuid.MustParse("01990000-0000-7000-8000-000000000700")
	occurredAt    = time.Date(2026, 11, 20, 9, 30, 15, 123456789, time.FixedZone("TST", 8*3600))
	wireTimestamp = "2026-11-20T01:30:15.123456789Z"
	createdAt     = time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
)

func newFixture() (*Worker, *fakeReviews) {
	r := &fakeReviews{
		review: review.Review{
			ID: reviewID, OrderNo: "O1", OrderItemID: 11, BuyerID: 301, StoreID: 91, ProductID: product700, Rating: 5, Content: "很甜",
			Images: []review.Image{{AssetID: uuid.MustParse("01990000-0000-7000-8000-0000000000b1"), ObjectKey: "review/1.jpg"}, {AssetID: uuid.MustParse("01990000-0000-7000-8000-0000000000b2"), ObjectKey: "review/2.jpg"}}, CreatedAt: createdAt,
		},
		product: map[uuid.UUID]review.ProductStats{product700: {Count: 30, Average: 4.9}},
		store:   map[int64]review.StoreStats{91: {Count: 120, Average: 4.8}},
	}
	return NewWorker(r, &recordingJobs{}, nil), r
}

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
	return review.ReviewCreated{EventID: eventID, OccurredAt: occurredAt, ReviewID: reviewID, StoreID: 91, ProductID: product700}
}

func TestReviewCreatedPayload(t *testing.T) {
	w, _ := newFixture()
	got := run(t, w, OnReviewCreated, created())
	if len(got) != 1 || got[0].Exchange != "hawkr.review" || got[0].RoutingKey != "review.created" || got[0].EventID != eventID.String() {
		t.Fatalf("published = %+v", got)
	}
	m := body(t, got[0])
	want := map[string]any{
		"event_id": eventID.String(), "event_type": "REVIEW_CREATED", "version": "1.0", "timestamp": wireTimestamp, "source": "review-service",
		"review_id": float64(0), "order_no": "O1", "order_item_id": float64(11), "store_id": float64(91),
		"buyer_id": float64(301), "rating": float64(5), "content": "很甜", "image_keys": []any{"review/1.jpg", "review/2.jpg"},
		"created_at": "2026-11-20T01:00:00Z", "avg_rating": 4.8, "total_count": float64(120), "product_id": product700.String(),
		"product_avg_rating": 4.9, "product_review_count": float64(30),
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
	w, r := newFixture()
	r.gone = true
	if got := run(t, w, OnReviewCreated, created()); len(got) != 0 {
		t.Fatalf("published = %+v", got)
	}
}

// 評價在 job 執行時已被隱藏或作者在刪帳中：不送 review.created（不外送內容、不通知賣家）。
func TestReviewCreatedSkipsAReviewThatIsNoLongerPublic(t *testing.T) {
	w, r := newFixture()
	r.hidden = true
	if got := run(t, w, OnReviewCreated, created()); len(got) != 0 {
		t.Fatalf("published = %+v", got)
	}
}

// 沒有任何可見評價時，聚合是 0 而且仍然是 JSON 數字（Java 的 ReviewStatsConsumer 缺欄位或非數字會進 DLQ，不能省略）。
func TestZeroAggregatesAreStillNumbers(t *testing.T) {
	w, r := newFixture()
	r.product = map[uuid.UUID]review.ProductStats{}
	r.store = map[int64]review.StoreStats{}
	m := body(t, run(t, w, OnReviewCreated, created())[0])
	for _, k := range []string{"product_avg_rating", "product_review_count", "avg_rating", "total_count"} {
		if v, ok := m[k].(float64); !ok || v != 0 {
			t.Errorf("%s = %#v，預期數字 0", k, m[k])
		}
	}
}
