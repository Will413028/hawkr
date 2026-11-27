package buyerconsumer

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
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
	b := rabbitmq.Binding{Queue: "order", Handle: c.OrderCompleted}
	log := slog.New(slog.DiscardHandler)
	rabbitmq.Process(t.Context(), amqp.Delivery{Acknowledger: acks, Body: []byte(`{"event_id":"bad"}`)}, b, isPermanent, log)
	rabbitmq.Process(t.Context(), amqp.Delivery{Acknowledger: acks, Body: []byte(`{"event_id":"good","buyer_id":"71"}`)}, b, isPermanent, log)
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

	worker.complete = func(context.Context, int64) error { return nil }
	worker.storeAccount = func(context.Context, int64) (int64, error) { return 0, apperr.NotFound("store") }
	if err := worker.Work(t.Context(), job); err != nil {
		t.Fatalf("unknown store: %v", err)
	}

	worker.storeAccount = nil
	if err := worker.Work(t.Context(), job); err == nil {
		t.Fatal("missing store resolver was ignored")
	}

	noStore := &river.Job[OrderCompletedArgs]{Args: OrderCompletedArgs{EventID: "evt-2", BuyerID: 71}}
	if err := worker.Work(t.Context(), noStore); err != nil {
		t.Fatal(err)
	}
	if err := c.OrderCompleted(t.Context(), []byte(`{"event_id":"evt-3","buyer_id":"71","store_id":"x"}`)); !isPermanent(err) {
		t.Fatalf("bad store_id: %v", err)
	}
}
