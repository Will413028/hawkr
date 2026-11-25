package buyerrelay

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"go.yaml.in/yaml/v3"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type relayBuyers struct{}

func (relayBuyers) AccountIDByBuyerID(context.Context, int64) (int64, error) { return 99, nil }
func (relayBuyers) ProfileByBuyerID(context.Context, int64) (buyer.Profile, error) {
	return buyer.Profile{DisplayName: "買家甲"}, nil
}

type relayProducts struct{ calls int }

func (p *relayProducts) Cards(context.Context, []int64, bool) (map[int64]Product, error) {
	p.calls++
	return map[int64]Product{11: {ID: 11, StoreID: 91, Name: "蘋果", ImageKey: "public/apple"}}, nil
}

type relayStores struct{}

func (relayStores) Store(context.Context, int64) (Store, error) {
	return Store{StoreID: 91, AccountID: 88, Status: seller.Active}, nil
}

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

func TestEachBuyerEventHasItsOwnJobKind(t *testing.T) {
	args := []river.JobArgs{FavoriteAddedArgs{}, FavoriteRemovedArgs{}, BuyerDeletedArgs{}}
	kinds := map[string]bool{}
	for _, a := range args {
		if kinds[a.Kind()] {
			t.Fatalf("duplicate kind %s", a.Kind())
		}
		kinds[a.Kind()] = true
	}
	body, err := json.Marshal(FavoriteAddedArgs{eventFields: fields("event-1"), ProductID: 11})
	if err != nil {
		t.Fatal(err)
	}
	var decoded FavoriteAddedArgs
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.EventID != "event-1" || decoded.BuyerID != 42 || decoded.ProductID != 11 {
		t.Fatalf("job roundtrip=%+v error=%v body=%s", decoded, err, body)
	}
}

func TestBuyerDeletedKeepsLegacyWireKeys(t *testing.T) {
	q := &recordingJobs{}
	w := NewWorker(nil, nil, nil, q, nil)
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

func TestLegacyEventsEnrichProductAndSellerFields(t *testing.T) {
	for _, tc := range []struct {
		name, exchange, key string
		work                func(*Worker) error
		fields              map[string]any
	}{
		{"favorite", "hawkr.buyer", "buyer.favorite.added", func(w *Worker) error {
			return w.favoriteAdded(t.Context(), FavoriteAddedArgs{eventFields: fields("event-1"), ProductID: 11})
		}, map[string]any{"product_name": "蘋果", "seller_account_id": "88", "store_id": "91", "buyer_name": "買家甲"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &recordingJobs{}
			products := &relayProducts{}
			w := NewWorker(relayBuyers{}, products, relayStores{}, q, nil)
			if err := tc.work(w); err != nil {
				t.Fatal(err)
			}
			a, payload := q.published(t)
			if a.Exchange != tc.exchange || a.RoutingKey != tc.key {
				t.Fatalf("routing=%s/%s", a.Exchange, a.RoutingKey)
			}
			if !topologyDeclaresExchange(t, a.Exchange) {
				t.Errorf("exchange %s 不在 infra/mq/topology/topology.yaml 的 exchanges", a.Exchange)
			}
			for key, want := range tc.fields {
				if payload[key] != want {
					t.Errorf("%s=%v want %v", key, payload[key], want)
				}
			}
		})
	}
}

// storeFunc 依呼叫回傳指定的店家或錯誤。
type storeFunc func(context.Context, int64) (Store, error)

func (f storeFunc) Store(ctx context.Context, id int64) (Store, error) { return f(ctx, id) }

// 店家已不存在是永久狀態：不回錯誤讓 job 重試到上限。收藏缺店家欄位照送（計數依 product_id）。店家沒有帳號時送空字串，不送 "0"（舊 buyer-service 的 publisher）。暫時錯誤照樣重試。
func TestRelayTreatsMissingStoreAsPermanent(t *testing.T) {
	missing := storeFunc(func(context.Context, int64) (Store, error) { return Store{}, apperr.NotFound("店家不存在") })
	closed := storeFunc(func(context.Context, int64) (Store, error) {
		return Store{StoreID: 91, AccountID: 88, Status: seller.Closed}, nil
	})
	noAccount := storeFunc(func(context.Context, int64) (Store, error) { return Store{StoreID: 91, Status: seller.Active}, nil })
	down := storeFunc(func(context.Context, int64) (Store, error) { return Store{}, errors.New("seller unavailable") })
	favorite := func(w *Worker) error {
		return w.favoriteAdded(t.Context(), FavoriteAddedArgs{eventFields: fields("event-1"), ProductID: 11})
	}
	for _, tc := range []struct {
		name    string
		stores  Stores
		work    func(*Worker) error
		wantErr bool
		fields  map[string]any // nil：不送
	}{
		{"favorite 店家不存在", missing, favorite, false, map[string]any{"seller_account_id": "", "store_id": "91", "product_id": "11"}},
		{"favorite 店家沒有帳號", noAccount, favorite, false, map[string]any{"seller_account_id": "", "store_id": "91"}},
		{"favorite 店家已關閉", closed, favorite, false, map[string]any{"seller_account_id": "", "store_id": "91"}},
		{"favorite 暫時錯誤", down, favorite, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &recordingJobs{}
			w := NewWorker(relayBuyers{}, &relayProducts{}, tc.stores, q, slog.New(slog.DiscardHandler))
			err := tc.work(w)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.fields == nil {
				if len(q.args) != 0 {
					t.Fatalf("published %d jobs", len(q.args))
				}
				return
			}
			_, payload := q.published(t)
			for key, want := range tc.fields {
				if payload[key] != want {
					t.Errorf("%s=%v want %v", key, payload[key], want)
				}
			}
		})
	}
}

// topologyDeclaresExchange 讀 topology.yaml（exchange 的唯一登記處），回答 exchange 有沒有宣告。
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
