package live

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeReader struct {
	mu     sync.Mutex
	convs  map[uuid.UUID][]model.Message
	owner  map[uuid.UUID]partyKey
	unread int64
	calls  struct{ unread int }
	active []uuid.UUID
}

func newFakeReader() *fakeReader {
	return &fakeReader{convs: map[uuid.UUID][]model.Message{}, owner: map[uuid.UUID]partyKey{}}
}

func (f *fakeReader) add(c uuid.UUID, seq int64, sender model.PartyKind) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.convs[c] = append(f.convs[c], model.Message{ID: uuid.NewV7(), ConversationID: c, Seq: seq, SenderKind: sender, Type: model.TypeText})
}

func (f *fakeReader) MessagesAfter(_ context.Context, c uuid.UUID, p model.Party, after int64, size int32) (model.MessagePage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if o, ok := f.owner[c]; ok && o != (partyKey{p.Kind, p.ID()}) {
		return model.MessagePage{}, apperr.NotFound("not found")
	}
	var items []model.Message
	for _, m := range f.convs[c] {
		if m.Seq > after {
			items = append(items, m)
		}
	}
	slices.SortFunc(items, func(a, b model.Message) int { return int(a.Seq - b.Seq) })
	if len(items) > int(size) {
		return model.MessagePage{Items: items[:size], HasMore: true}, nil
	}
	return model.MessagePage{Items: items}, nil
}

func (f *fakeReader) UnreadTotal(context.Context, model.Party) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls.unread++
	return f.unread, nil
}

func (f *fakeReader) ActiveSince(context.Context, model.Party, time.Time, int) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.active), nil
}

type fakeStreams struct {
	mu      sync.Mutex
	open    int
	deleted int
}

func (f *fakeStreams) Open(context.Context, model.PartyKind, int64, uuid.UUID, int, time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.open++
	return nil
}
func (f *fakeStreams) Extend(context.Context, []uuid.UUID, time.Duration) error { return nil }
func (f *fakeStreams) Delete(_ context.Context, ids []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted += len(ids)
	return nil
}

func newTestHub(t *testing.T, r Reader, cfg Config) *Hub {
	t.Helper()
	if cfg.NewID == nil {
		cfg.NewID = uuid.NewV7
	}
	return NewHub(r, &fakeStreams{}, cfg)
}

func buyerParty(id int64) model.Party { return model.Party{Kind: model.PartyBuyer, BuyerID: id} }

func storeParty(id int64) model.Party {
	st := seller.Store{
		StoreID: id, AccountID: 2000 + id, Status: seller.Active, Sanctions: seller.NoSanctions(),
		Conditions: seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true},
	}
	return model.Party{Kind: model.PartyStore, Store: &st}
}

// drain 取走目前所有能立刻取得的事件：用 poll，沒有事件時不阻塞，所以不靠時間判斷「沒有更多了」。
func drain(t *testing.T, s *Subscription) []Event {
	t.Helper()
	var out []Event
	for {
		ev, ok, err := s.poll(t.Context())
		if err != nil {
			t.Fatalf("poll：%v", err)
		}
		if !ok {
			return out
		}
		out = append(out, ev)
	}
}

func seqs(evs []Event, updated bool) []int64 {
	var out []int64
	for _, e := range evs {
		if e.Kind == EventMessage && e.Updated == updated {
			out = append(out, e.Message.Seq)
		}
	}
	return out
}

func kinds(evs []Event) []EventKind {
	out := make([]EventKind, len(evs))
	for i, e := range evs {
		out[i] = e.Kind
	}
	return out
}
