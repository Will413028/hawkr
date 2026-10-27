package sellerconsumer

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type notifier struct {
	got []seller.PayUniNotification
	err error
}

func (n *notifier) ReceivePayUniNotification(_ context.Context, v seller.PayUniNotification) error {
	n.got = append(n.got, v)
	return n.err
}

type acks struct{ acks, nacks int }

func (a *acks) Ack(uint64, bool) error        { a.acks++; return nil }
func (a *acks) Nack(uint64, bool, bool) error { a.nacks++; return nil }
func (a *acks) Reject(uint64, bool) error     { return nil }

const goodBody = `{"event_id":"e","provider":"PAYUNI","webhook_type":"AGENT_NOTIFY","mer_id":"HAWK2600000001","event_key":"HAWK2600000001:1:20261002120000",` +
	`"payload":{"MerID":"HAWK2600000001","NotifyType":"MerStatus","MerStatus":"1","MerStatusNote":"審核通過","ExecuteTime":"20261002120000"}}`

func TestReviewResultMapsTheJavaEnvelope(t *testing.T) {
	n := &notifier{}
	if err := (Consumer{Seller: n}).ReviewResult(t.Context(), []byte(goodBody)); err != nil {
		t.Fatal(err)
	}
	want := seller.PayUniNotification{
		MerID: "HAWK2600000001", EventKey: "HAWK2600000001:1:20261002120000", MerStatus: "1", MerStatusNote: "審核通過", ExecuteTime: "20261002120000",
	}
	if len(n.got) != 1 || n.got[0] != want {
		t.Fatalf("got=%+v", n.got)
	}
}

func TestReviewResultFallsBackToPayloadMerIDAndReason(t *testing.T) {
	n := &notifier{}
	body := `{"event_key":"k","payload":{"MerID":"HAWK1","MerStatus":"91","Reason":"補件","ExecuteTime":"20261002120000"}}`
	if err := (Consumer{Seller: n}).ReviewResult(t.Context(), []byte(body)); err != nil {
		t.Fatal(err)
	}
	if n.got[0].MerID != "HAWK1" || n.got[0].MerStatusNote != "補件" {
		t.Fatalf("got=%+v", n.got[0])
	}
}

// 形狀不合與模組判定不合法的訊息是永久錯誤（進 dead-letter，不阻塞後面的訊息）；模組暫時失敗則 requeue。
func TestMalformedIsPermanentButTransientFailuresAreNot(t *testing.T) {
	if err := (Consumer{Seller: &notifier{}}).ReviewResult(t.Context(), []byte(`not json`)); !isPermanent(err) {
		t.Fatalf("malformed JSON: %v", err)
	}
	if err := (Consumer{Seller: &notifier{err: apperr.InvalidInput("MerID 格式不合")}}).ReviewResult(t.Context(), []byte(goodBody)); !isPermanent(err) {
		t.Fatalf("invalid input: %v", err)
	}
	if err := (Consumer{Seller: &notifier{err: errors.New("database down")}}).ReviewResult(t.Context(), []byte(goodBody)); err == nil || isPermanent(err) {
		t.Fatalf("transient: %v", err)
	}
}

func TestPoisonMessageDoesNotBlockTheNextOne(t *testing.T) {
	n := &notifier{}
	c := Consumer{Seller: n}
	a := &acks{}
	b := rabbitmq.Binding{Queue: "q", Handle: c.ReviewResult}
	log := slog.New(slog.DiscardHandler)
	rabbitmq.Process(t.Context(), amqp.Delivery{Acknowledger: a, Body: []byte(`{`)}, b, isPermanent, log)
	rabbitmq.Process(t.Context(), amqp.Delivery{Acknowledger: a, Body: []byte(goodBody)}, b, isPermanent, log)
	if a.nacks != 1 || a.acks != 1 || len(n.got) != 1 {
		t.Fatalf("acks=%d nacks=%d got=%d", a.acks, a.nacks, len(n.got))
	}
}
