package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type fakeSender struct {
	got []string
	err error
}

func (f *fakeSender) Publish(_ context.Context, exchange, routingKey string, body []byte) error {
	f.got = append(f.got, exchange+"|"+routingKey+"|"+string(body))
	return f.err
}

func TestWorkerPublishesTheBodyAsQueued(t *testing.T) {
	s := &fakeSender{}
	job := &river.Job[PublishArgs]{JobRow: &rivertype.JobRow{}, Args: PublishArgs{
		Exchange: "hawkr.account", RoutingKey: "account.deleted", Body: json.RawMessage(`{"event_id":"e1"}`),
	}}
	if err := NewWorker(s).Work(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if len(s.got) != 1 || s.got[0] != `hawkr.account|account.deleted|{"event_id":"e1"}` {
		t.Fatalf("送出 = %v", s.got)
	}
	// broker 不可用時回錯誤，由 River 重試同一則（event_id 不變）。
	s.err = errors.New("connection refused")
	if err := NewWorker(s).Work(context.Background(), job); err == nil {
		t.Fatal("送出失敗時要回錯誤")
	}
	if opts := InsertOpts(); opts.MaxAttempts != publishMaxAttempts || !opts.UniqueOpts.ByArgs {
		t.Fatalf("InsertOpts = %+v，要有重試次數與以 event_id 做的 unique", opts)
	}
}

func TestNewRequiresAURL(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("沒有網址應該失敗")
	}
}
