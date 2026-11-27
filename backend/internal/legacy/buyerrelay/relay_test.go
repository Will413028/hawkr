package buyerrelay

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"go.yaml.in/yaml/v3"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
)

type recordingJobs struct {
	args []river.JobArgs
	opts []*river.InsertOpts
}

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	q.args = append(q.args, args)
	q.opts = append(q.opts, opts)
	return nil
}
func (q *recordingJobs) published(t *testing.T) (rabbitmq.PublishArgs, map[string]any) {
	t.Helper()
	if len(q.args) != 1 {
		t.Fatalf("queued jobs=%d", len(q.args))
	}
	a, ok := q.args[0].(rabbitmq.PublishArgs)
	if !ok {
		t.Fatalf("job type=%T", q.args[0])
	}
	if q.opts[0] == nil || q.opts[0].MaxAttempts != rabbitmq.InsertOpts().MaxAttempts {
		t.Fatalf("publish opts=%+v", q.opts[0])
	}
	var payload map[string]any
	if err := json.Unmarshal(a.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if a.EventID == "" || a.EventID != payload["event_id"] {
		t.Fatalf("PublishArgs.EventID=%q, body event_id=%v", a.EventID, payload["event_id"])
	}
	return a, payload
}

func at() time.Time { return time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC) }
func fields(eventID string) eventFields {
	return eventFields{EventID: eventID, Occurred: at(), BuyerID: 42}
}

func TestBuyerDeletedJobKindAndRoundtrip(t *testing.T) {
	if got := (BuyerDeletedArgs{}).Kind(); got != "legacy.buyer_deleted" {
		t.Fatalf("kind=%s", got)
	}
	body, err := json.Marshal(BuyerDeletedArgs{eventFields: fields("event-1"), AccountID: 99})
	if err != nil {
		t.Fatal(err)
	}
	var decoded BuyerDeletedArgs
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.EventID != "event-1" || decoded.BuyerID != 42 || decoded.AccountID != 99 {
		t.Fatalf("job roundtrip=%+v error=%v body=%s", decoded, err, body)
	}
}

func TestBuyerDeletedKeepsLegacyWireKeys(t *testing.T) {
	q := &recordingJobs{}
	w := NewWorker(q)
	if err := w.buyerDeleted(t.Context(), BuyerDeletedArgs{eventFields: fields("event-1"), AccountID: 99}); err != nil {
		t.Fatal(err)
	}
	a, payload := q.published(t)
	if a.Exchange != "hawkr.buyer" || a.RoutingKey != "buyer.deleted" {
		t.Fatalf("routing=%s/%s", a.Exchange, a.RoutingKey)
	}
	if payload["accountId"] != float64(99) || payload["buyerId"] != float64(42) {
		t.Fatalf("payload=%v", payload)
	}
}

func TestBuyerDeletedExchangeIsDeclaredInTopology(t *testing.T) {
	q := &recordingJobs{}
	w := NewWorker(q)
	if err := w.buyerDeleted(t.Context(), BuyerDeletedArgs{eventFields: fields("event-1"), AccountID: 99}); err != nil {
		t.Fatal(err)
	}
	a, _ := q.published(t)
	if !topologyDeclaresExchange(t, a.Exchange) {
		t.Errorf("exchange %s 不在 infra/mq/topology/topology.yaml 的 exchanges", a.Exchange)
	}
}

func topologyDeclaresExchange(t *testing.T, exchange string) bool {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "infra", "mq", "topology", "topology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var topology struct {
		Exchanges []struct {
			Name string `yaml:"name"`
		} `yaml:"exchanges"`
	}
	if err := yaml.Unmarshal(body, &topology); err != nil {
		t.Fatal(err)
	}
	for _, e := range topology.Exchanges {
		if e.Name == exchange {
			return true
		}
	}
	return false
}
