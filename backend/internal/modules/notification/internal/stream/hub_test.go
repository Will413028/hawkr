package stream

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func fakeRead(_ context.Context, key Key, storeID int64) (Inbox, error) {
	return Inbox{UnreadCount: int32(key.AccountID*10 + storeID)}, nil
}

func buyer(account int64) Key    { return Key{AccountID: account, Audience: model.AudienceBuyer} }
func storeKey(account int64) Key { return Key{AccountID: account, Audience: model.AudienceStore} }

func next(t *testing.T, s *Subscription) Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ev, err := s.Next(ctx)
	if err != nil {
		t.Fatalf("Next：%v", err)
	}
	return ev
}

func wantNone(t *testing.T, s *Subscription) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if ev, err := s.Next(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("預期沒有訊號，實際：%+v %v", ev, err)
	}
}

func subscribe(t *testing.T, h *Hub, key Key, storeID int64) *Subscription {
	t.Helper()
	s, err := h.Subscribe(key, storeID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestFirstSignalCarriesSummaryWithoutHints(t *testing.T) {
	h := newHub(fakeRead, 3)
	s := subscribe(t, h, buyer(4), 0)
	ev := next(t, s)
	if ev.Inbox == nil || ev.Inbox.UnreadCount != 40 || len(ev.Inbox.Hints) != 0 || ev.Workspace != 0 {
		t.Fatalf("第一則應是無提示的 inbox 訊號：%+v", ev)
	}
	wantNone(t, s)
}

func TestInboxSignalReachesOnlyTheSameAccountAndAudience(t *testing.T) {
	h := newHub(fakeRead, 8)
	b := subscribe(t, h, buyer(1), 0)
	st := subscribe(t, h, storeKey(1), 7)
	other := subscribe(t, h, buyer(2), 0)
	for _, s := range []*Subscription{b, st, other} {
		next(t, s)
	}
	h.InboxChanged(buyer(1), 0, model.StateHintOrder)
	if ev := next(t, b); ev.Inbox == nil || !slices.Equal(ev.Inbox.Hints, []model.StateHint{model.StateHintOrder}) {
		t.Fatalf("買家訂閱者應收到帶提示的訊號：%+v", ev)
	}
	wantNone(t, st)
	wantNone(t, other)
}

func TestStoreInboxSignalIsScopedToTheStore(t *testing.T) {
	h := newHub(fakeRead, 8)
	a := subscribe(t, h, storeKey(1), 7)
	b := subscribe(t, h, storeKey(1), 8)
	next(t, a)
	next(t, b)
	h.InboxChanged(storeKey(1), 8, model.StateHintNone)
	if ev := next(t, b); ev.Inbox == nil || ev.Inbox.UnreadCount != 18 {
		t.Fatalf("店 8 的訂閱者應收到訊號：%+v", ev)
	}
	wantNone(t, a)
}

func TestSignalsCoalesceAndHintsAreAUnion(t *testing.T) {
	h := newHub(fakeRead, 8)
	s := subscribe(t, h, buyer(1), 0)
	next(t, s)
	h.InboxChanged(buyer(1), 0, model.StateHintOrder)
	h.InboxChanged(buyer(1), 0, model.StateHintOrder)
	h.InboxChanged(buyer(1), 0, model.StateHintNone)
	h.InboxChanged(buyer(1), 0, model.StateHintAfterSale)
	ev := next(t, s)
	if ev.Inbox == nil || !slices.Equal(ev.Inbox.Hints, []model.StateHint{model.StateHintOrder, model.StateHintAfterSale}) {
		t.Fatalf("提示應是不重複的聯集：%+v", ev)
	}
	wantNone(t, s)
}

func TestStreamCapPerAccountRejectsWithResourceExhausted(t *testing.T) {
	h := newHub(fakeRead, 2)
	subscribe(t, h, buyer(1), 0)
	subscribe(t, h, storeKey(1), 7)
	_, err := h.Subscribe(buyer(1), 0)
	if apperr.KindOf(err) != apperr.KindResourceExhausted {
		t.Fatalf("超過上限應回 resource_exhausted：%v", err)
	}
	if _, err := h.Subscribe(buyer(2), 0); err != nil {
		t.Fatalf("別的帳號不受影響：%v", err)
	}
}

func TestClosingASubscriptionFreesItsSlot(t *testing.T) {
	h := newHub(fakeRead, 1)
	s, err := h.Subscribe(buyer(1), 0)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s.Close()
	if _, err := s.Next(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("關閉後 Next 應回 ErrClosed：%v", err)
	}
	if _, err := h.Subscribe(buyer(1), 0); err != nil {
		t.Fatalf("關閉後應能再開：%v", err)
	}
}

func TestSellerWorkspaceReachesOnlyThatStoresSubscribers(t *testing.T) {
	h := newHub(fakeRead, 8)
	mine := subscribe(t, h, storeKey(1), 7)
	otherStore := subscribe(t, h, storeKey(2), 8)
	buyerSide := subscribe(t, h, buyer(1), 0)
	for _, s := range []*Subscription{mine, otherStore, buyerSide} {
		next(t, s)
	}
	h.workspaceChanged(7, SellerOrder)
	h.workspaceChanged(7, SellerOrder)
	h.workspaceChanged(7, SellerAfterSale)
	if ev := next(t, mine); ev.Workspace != SellerOrder || ev.Inbox != nil {
		t.Fatalf("應先收到訂單訊號：%+v", ev)
	}
	if ev := next(t, mine); ev.Workspace != SellerAfterSale {
		t.Fatalf("再收到售後訊號：%+v", ev)
	}
	wantNone(t, mine)
	wantNone(t, otherStore)
	wantNone(t, buyerSide)
}

func TestSubscribeValidatesAudienceAndStore(t *testing.T) {
	h := newHub(fakeRead, 8)
	for name, tc := range map[string]struct {
		key     Key
		storeID int64
	}{
		"STORE 沒有店": {storeKey(1), 0},
		"BUYER 帶了店": {buyer(1), 3},
		"沒有帳號":      {buyer(0), 0},
		"未知受眾":      {Key{AccountID: 1, Audience: "X"}, 0},
	} {
		if _, err := h.Subscribe(tc.key, tc.storeID); err == nil {
			t.Errorf("%s：應被拒絕", name)
		}
	}
}

// 讀取內容失敗時，已取走的收件匣標記與提示放回去，下一次 Next 重讀並帶出提示；第一則（連線建立）也一樣。
// mutation：Next 讀取失敗不呼叫 restoreInbox → 重試時沒有訊號（逾時）或提示遺失，失敗。
func TestFailedReadKeepsTheDirtyMarkAndHints(t *testing.T) {
	fail := true
	h := newHub(func(ctx context.Context, key Key, storeID int64) (Inbox, error) {
		if fail {
			return Inbox{}, errors.New("db down")
		}
		return fakeRead(ctx, key, storeID)
	}, 8)
	s := subscribe(t, h, buyer(1), 0)
	if _, err := s.Next(context.Background()); err == nil {
		t.Fatal("第一則讀取失敗應回錯誤")
	}
	fail = false
	if ev := next(t, s); ev.Inbox == nil {
		t.Fatalf("重試應再送第一則：%+v", ev)
	}
	h.InboxChanged(buyer(1), 0, model.StateHintOrder)
	fail = true
	if _, err := s.Next(context.Background()); err == nil {
		t.Fatal("讀取失敗應回錯誤")
	}
	h.InboxChanged(buyer(1), 0, model.StateHintAfterSale)
	fail = false
	ev := next(t, s)
	if ev.Inbox == nil || !slices.Equal(ev.Inbox.Hints, []model.StateHint{model.StateHintOrder, model.StateHintAfterSale}) {
		t.Fatalf("失敗前取走的提示不該遺失，期間新來的也要合併：%+v", ev)
	}
	wantNone(t, s)
}
