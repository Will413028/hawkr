package buyerconsumer

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/riverqueue/river"
)

type fakeBuyer struct {
	accountID int64
	gotID     int64
}

type recordedJobs struct{ args []river.JobArgs }

func (r *recordedJobs) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	r.args = append(r.args, args)
	return nil
}

type recordedAcks struct {
	acks, nacks int
	requeue     bool
}

func (r *recordedAcks) Ack(uint64, bool) error { r.acks++; return nil }
func (r *recordedAcks) Nack(_ uint64, _ bool, requeue bool) error {
	r.nacks++
	r.requeue = requeue
	return nil
}
func (r *recordedAcks) Reject(uint64, bool) error { return nil }

func (f *fakeBuyer) AccountIDByBuyerID(_ context.Context, id int64) (int64, error) {
	f.gotID = id
	return f.accountID, nil
}

func (*fakeBuyer) FanOut(context.Context, buyer.FanOutInput) (bool, error) {
	return false, nil
}

type snapshotBuyer struct {
	calls     int
	productID int64
	details   buyer.FanOutInput
}

func (*snapshotBuyer) AccountIDByBuyerID(context.Context, int64) (int64, error) { return 0, nil }

func (f *snapshotBuyer) FanOut(_ context.Context, input buyer.FanOutInput) (bool, error) {
	f.calls++
	f.productID = input.ProductID
	f.details = input
	return f.calls < 3, nil
}

func TestRestockWorkerKeepsUpstreamSnapshotAcrossBatches(t *testing.T) {
	queue := &recordedJobs{}
	c := Consumer{Jobs: queue}
	const occurred = "2026-09-29T12:34:56Z"
	if err := c.ProductRestock(t.Context(), []byte(`{"event_id":"restock-1","product_id":"9","product_name":"蘋果","image_key":"public/apple","occurred_at":"`+occurred+`"}`)); err != nil {
		t.Fatal(err)
	}
	f := &snapshotBuyer{}
	job := &river.Job[RestockArgs]{Args: queue.args[0].(RestockArgs)}
	if err := (&restockWorker{buyer: f}).Work(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse(time.RFC3339Nano, occurred)
	if f.calls != 3 || f.productID != 9 || f.details.ProductName != "蘋果" || f.details.ImageKey != "public/apple" || !f.details.OccurredAt.Equal(want) {
		t.Fatalf("calls=%d product=%d details=%+v", f.calls, f.productID, f.details)
	}
}

func TestOrderCompletedResolvesBuyerThenCompletesPending(t *testing.T) {
	f := &fakeBuyer{accountID: 8}
	jobs := &recordedJobs{}
	var completed int64
	c := Consumer{Buyer: f, Jobs: jobs, Complete: func(_ context.Context, id int64) error { completed = id; return nil }}
	if err := c.OrderCompleted(t.Context(), []byte(`{"event_id":"evt-1","buyer_id":"71"}`)); err != nil {
		t.Fatal(err)
	}
	if len(jobs.args) != 1 {
		t.Fatalf("jobs = %d", len(jobs.args))
	}
	worker := &orderCompletedWorker{buyer: f, complete: c.Complete}
	job := &river.Job[OrderCompletedArgs]{Args: jobs.args[0].(OrderCompletedArgs)}
	if err := worker.Work(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	if f.gotID != 71 || completed != 8 {
		t.Fatalf("buyer_id=%d, completed account_id=%d", f.gotID, completed)
	}
	worker.complete = func(_ context.Context, id int64) error { return errors.New("orders unavailable") }
	if err := worker.Work(t.Context(), job); err == nil {
		t.Fatal("CompletePending 失敗時必須重試事件")
	}
}

func TestPoisonEventIsAcknowledgedAndFollowingEventIsEnqueued(t *testing.T) {
	jobs := &recordedJobs{}
	c := Consumer{Jobs: jobs}
	acks := &recordedAcks{}
	b := rabbitmq.Binding{Queue: "restock", Handle: c.ProductRestock}
	log := slog.New(slog.DiscardHandler)
	rabbitmq.Process(t.Context(), amqp.Delivery{Acknowledger: acks, Body: []byte(`{"event_id":"bad"}`)}, b, isPermanent, log)
	rabbitmq.Process(t.Context(), amqp.Delivery{Acknowledger: acks, Body: []byte(`{"event_id":"good","product_id":"9","occurred_at":"2026-09-30T00:00:00Z"}`)}, b, isPermanent, log)
	if acks.acks != 1 || acks.nacks != 1 || acks.requeue || len(jobs.args) != 1 {
		t.Fatalf("acks=%d nacks=%d jobs=%d", acks.acks, acks.nacks, len(jobs.args))
	}
}

// 店家是商家的訂單與買家的訂單一樣阻擋刪帳：訂單結案時兩側的帳號都要試著完成等待中的刪除，一側失敗不擋另一側。
func TestOrderCompletedAlsoCompletesTheStoresAccount(t *testing.T) {
	f := &fakeBuyer{accountID: 8}
	jobs := &recordedJobs{}
	c := Consumer{Buyer: f, Jobs: jobs}
	if err := c.OrderCompleted(t.Context(), []byte(`{"event_id":"evt-1","buyer_id":"71","store_id":"5"}`)); err != nil {
		t.Fatal(err)
	}
	args := jobs.args[0].(OrderCompletedArgs)
	if args.StoreID != 5 {
		t.Fatalf("args=%+v", args)
	}
	job := &river.Job[OrderCompletedArgs]{Args: args}
	var completed []int64
	var storeLookups []int64
	worker := &orderCompletedWorker{
		buyer: f,
		complete: func(_ context.Context, id int64) error {
			completed = append(completed, id)
			return nil
		},
		storeAccount: func(_ context.Context, storeID int64) (int64, error) {
			storeLookups = append(storeLookups, storeID)
			return 9, nil
		},
	}
	if err := worker.Work(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	if len(completed) != 2 || completed[0] != 8 || completed[1] != 9 || len(storeLookups) != 1 || storeLookups[0] != 5 {
		t.Fatalf("completed=%v storeLookups=%v", completed, storeLookups)
	}

	// 買家那一側失敗，店家那一側仍然完成，整體回錯誤讓 River 重試。
	completed = nil
	worker.complete = func(_ context.Context, id int64) error {
		completed = append(completed, id)
		if id == 8 {
			return errors.New("orders unavailable")
		}
		return nil
	}
	if err := worker.Work(t.Context(), job); err == nil || len(completed) != 2 {
		t.Fatalf("err=%v completed=%v", err, completed)
	}

	// 不在單體的店家（舊服務）不是錯誤。
	worker.complete = func(context.Context, int64) error { return nil }
	worker.storeAccount = func(context.Context, int64) (int64, error) { return 0, apperr.NotFound("store") }
	if err := worker.Work(t.Context(), job); err != nil {
		t.Fatalf("unknown store: %v", err)
	}

	// 事件有店家、卻沒有設定解析器是組裝錯誤，不能默默略過。
	worker.storeAccount = nil
	if err := worker.Work(t.Context(), job); err == nil {
		t.Fatal("missing store resolver was ignored")
	}

	// 沒有 store_id 的事件只處理買家；store_id 不合法是永久錯誤。
	noStore := &river.Job[OrderCompletedArgs]{Args: OrderCompletedArgs{EventID: "evt-2", BuyerID: 71}}
	if err := worker.Work(t.Context(), noStore); err != nil {
		t.Fatal(err)
	}
	if err := c.OrderCompleted(t.Context(), []byte(`{"event_id":"evt-3","buyer_id":"71","store_id":"x"}`)); !isPermanent(err) {
		t.Fatalf("bad store_id: %v", err)
	}
}
