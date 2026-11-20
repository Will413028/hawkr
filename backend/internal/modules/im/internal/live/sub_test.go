package live

import (
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

func msgPayload(c uuid.UUID, b, s, q int64) string { return Message(c, b, s, q).Encode() }

// 沒有游標的對話，在第一次 Next 之前連來兩則：合併成一格之後仍依序各交一次。
func TestCoalescedSignalsStillDeliverEveryMessageOnce(t *testing.T) {
	t.Parallel()
	r := newFakeReader()
	h := newTestHub(t, r, Config{})
	c := uuid.NewV7()
	r.add(c, 1, model.PartyStore)
	r.add(c, 2, model.PartyStore)
	sub, err := h.Subscribe(t.Context(), buyerParty(5), nil)
	if err != nil {
		t.Fatal(err)
	}
	h.dispatch(msgPayload(c, 5, 9, 1))
	h.dispatch(msgPayload(c, 5, 9, 2))
	h.dispatch(Visible(c, 5, 9, 1).Encode())
	h.dispatch(Visible(c, 5, 9, 2).Encode())
	evs := drain(t, sub)
	if got := seqs(evs, false); !slices.Equal(got, []int64{1, 2}) {
		t.Fatalf("訊息 seq %v，預期 [1 2]", got)
	}
	if got := seqs(evs, true); !slices.Equal(got, []int64{1, 2}) {
		t.Fatalf("可見度更新的 seq %v，預期各重讀一次 [1 2]", got)
	}
	if got := kinds(evs); got[len(got)-1] != EventReady {
		t.Fatalf("補洞完成後交 Ready：%v", got)
	}
	h.dispatch(msgPayload(c, 5, 9, 2))
	if got := seqs(drain(t, sub), false); len(got) != 0 {
		t.Fatalf("已交出的 seq 不重複：%v", got)
	}
}

// 一輪裡多則別人寄的訊息只算一次總未讀。
func TestUnreadIsComputedOncePerRound(t *testing.T) {
	t.Parallel()
	r := newFakeReader()
	h := newTestHub(t, r, Config{})
	c := uuid.NewV7()
	sub, err := h.Subscribe(t.Context(), buyerParty(5), map[uuid.UUID]int64{c: 0})
	if err != nil {
		t.Fatal(err)
	}
	drain(t, sub)
	base := r.calls.unread
	for q := int64(1); q <= 5; q++ {
		r.add(c, q, model.PartyStore)
		h.dispatch(msgPayload(c, 5, 9, q))
	}
	r.unread = 5
	evs := drain(t, sub)
	var unread []int64
	for _, e := range evs {
		if e.Kind == EventUnread {
			unread = append(unread, e.Unread)
		}
	}
	if !slices.Equal(unread, []int64{5}) || r.calls.unread-base != 1 {
		t.Fatalf("Unread 事件 %v、計算 %d 次，預期一次", unread, r.calls.unread-base)
	}
}

// 自己寄的訊息（其他裝置）不改變自己的未讀，不觸發 Unread。
func TestOwnMessagesDoNotTriggerUnread(t *testing.T) {
	t.Parallel()
	r := newFakeReader()
	h := newTestHub(t, r, Config{})
	c := uuid.NewV7()
	sub, _ := h.Subscribe(t.Context(), buyerParty(5), map[uuid.UUID]int64{c: 0})
	drain(t, sub)
	r.add(c, 1, model.PartyBuyer)
	h.dispatch(msgPayload(c, 5, 9, 1))
	for _, e := range drain(t, sub) {
		if e.Kind == EventUnread {
			t.Fatal("自己寄的訊息不該有 Unread")
		}
	}
}

// 同一個數字的買家與店家是不同的 Party。
func TestPartyKeyIncludesKind(t *testing.T) {
	t.Parallel()
	r := newFakeReader()
	h := newTestHub(t, r, Config{})
	buyer5, err := h.Subscribe(t.Context(), buyerParty(5), nil)
	if err != nil {
		t.Fatal(err)
	}
	store5, err := h.Subscribe(t.Context(), storeParty(5), nil)
	if err != nil {
		t.Fatal(err)
	}
	h.dispatch(msgPayload(uuid.NewV7(), 5, 9, 1))
	buyer5.mu.Lock()
	nb := len(buyer5.pending)
	buyer5.mu.Unlock()
	store5.mu.Lock()
	ns := len(store5.pending)
	store5.mu.Unlock()
	if nb != 1 || ns != 0 {
		t.Fatalf("買家 5 收到 %d、店家 5 收到 %d，預期 1、0", nb, ns)
	}
}

// 待處理超過上限：清空並交 Resync{all}，記憶體有界。
func TestPendingIsBoundedAndOverflowRequestsResync(t *testing.T) {
	t.Parallel()
	r := newFakeReader()
	h := newTestHub(t, r, Config{MaxPending: 3})
	sub, err := h.Subscribe(t.Context(), buyerParty(5), nil)
	if err != nil {
		t.Fatal(err)
	}
	drain(t, sub)
	for range 10 {
		h.dispatch(msgPayload(uuid.NewV7(), 5, 9, 1))
	}
	sub.mu.Lock()
	n, over := len(sub.pending), sub.overflow
	sub.mu.Unlock()
	if n > 3 || !over {
		t.Fatalf("pending %d、overflow %v，預期有界且標記 overflow", n, over)
	}
	evs := drain(t, sub)
	if len(evs) == 0 || evs[0].Kind != EventResync || !evs[0].All {
		t.Fatalf("預期 Resync{all}：%v", evs)
	}
	if got := seqs(evs, false); len(got) != 0 {
		t.Fatalf("overflow 之後不再交訊息：%v", got)
	}
}

// mark 不阻塞：消費者完全不 Next，監聽端呼叫仍然立刻返回。
func TestMarkNeverBlocksOnAStalledConsumer(t *testing.T) {
	t.Parallel()
	h := newTestHub(t, newFakeReader(), Config{MaxPending: 50})
	if _, err := h.Subscribe(t.Context(), buyerParty(5), nil); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 10000 {
			h.dispatch(msgPayload(uuid.NewV7(), 5, 9, 1))
		}
	}()
	<-done
}

// 重連後的 epoch：已交出游標的對話從游標補讀；不在游標裡的對話只對有新訊息的交 Resync{conversation}，沒有 Resync{all}。
func TestEpochReadsFromCursorsAndResyncsOnlyActiveUnknownConversations(t *testing.T) {
	t.Parallel()
	r := newFakeReader()
	h := newTestHub(t, r, Config{})
	known, active, quiet := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	r.add(known, 1, model.PartyStore)
	sub, err := h.Subscribe(t.Context(), buyerParty(5), map[uuid.UUID]int64{known: 1})
	if err != nil {
		t.Fatal(err)
	}
	drain(t, sub)
	r.add(known, 2, model.PartyStore)
	r.add(known, 3, model.PartyStore)
	r.active = []uuid.UUID{known, active}
	_ = quiet
	h.epoch(time.Unix(100, 0))
	evs := drain(t, sub)
	if got := seqs(evs, false); !slices.Equal(got, []int64{2, 3}) {
		t.Fatalf("epoch 補讀 %v，預期 [2 3]", got)
	}
	var resync []uuid.UUID
	for _, e := range evs {
		if e.Kind == EventResync {
			if e.All {
				t.Fatal("不該有 Resync{all}")
			}
			resync = append(resync, e.ConversationID)
		}
	}
	if !slices.Equal(resync, []uuid.UUID{active}) {
		t.Fatalf("Resync 的對話 %v，預期只有 %v", resync, active)
	}
}

// 不屬於這個 Party 的游標：靜默忽略，與沒有新訊息不可區分。
func TestForeignCursorsAreSilentlyIgnored(t *testing.T) {
	t.Parallel()
	r := newFakeReader()
	h := newTestHub(t, r, Config{})
	foreign := uuid.NewV7()
	r.owner[foreign] = partyKey{model.PartyBuyer, 99}
	r.add(foreign, 1, model.PartyStore)
	sub, err := h.Subscribe(t.Context(), buyerParty(5), map[uuid.UUID]int64{foreign: 0})
	if err != nil {
		t.Fatal(err)
	}
	evs := drain(t, sub)
	if len(evs) != 1 || evs[0].Kind != EventReady {
		t.Fatalf("只該有 Ready：%v", evs)
	}
	sub.mu.Lock()
	_, kept := sub.delivered[foreign]
	sub.mu.Unlock()
	if kept {
		t.Fatal("被拒的對話游標要移除")
	}
}

func TestCursorsOverTheLimitAreRejected(t *testing.T) {
	t.Parallel()
	h := newTestHub(t, newFakeReader(), Config{MaxCursors: 2})
	cur := map[uuid.UUID]int64{uuid.NewV7(): 0, uuid.NewV7(): 0, uuid.NewV7(): 0}
	if _, err := h.Subscribe(t.Context(), buyerParty(5), cur); err == nil {
		t.Fatal("游標超過上限要拒絕")
	}
}

func TestCloseIsIdempotentAndEndsNext(t *testing.T) {
	t.Parallel()
	h := newTestHub(t, newFakeReader(), Config{})
	sub, _ := h.Subscribe(t.Context(), buyerParty(5), nil)
	sub.Close()
	sub.Close()
	if _, err := sub.Next(t.Context()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Close 之後 Next 回 ErrClosed，得到 %v", err)
	}
	if got := h.streams.(*fakeStreams).deleted; got != 1 {
		t.Fatalf("存活標記刪除 %d 次，預期 1", got)
	}
}

func TestShutdownEndsEverySubscription(t *testing.T) {
	t.Parallel()
	h := newTestHub(t, newFakeReader(), Config{})
	a, _ := h.Subscribe(t.Context(), buyerParty(5), nil)
	b, _ := h.Subscribe(t.Context(), storeParty(5), nil)
	h.shutdown()
	for _, s := range []*Subscription{a, b} {
		var e *Ended
		if _, err := s.Next(t.Context()); !errors.As(err, &e) || e.Reason != EndShutdown {
			t.Fatalf("預期 Ended{Shutdown}，得到 %v", err)
		}
	}
	if _, err := h.Subscribe(t.Context(), buyerParty(5), nil); err == nil {
		t.Fatal("shutdown 之後 Subscribe 要失敗")
	}
}

func TestRevalidateRejectsAnotherParty(t *testing.T) {
	t.Parallel()
	h := newTestHub(t, newFakeReader(), Config{})
	sub, _ := h.Subscribe(t.Context(), buyerParty(5), nil)
	if err := sub.Revalidate(t.Context(), buyerParty(6)); err == nil {
		t.Fatal("換成別的身分要拒絕")
	}
	if err := sub.Revalidate(t.Context(), buyerParty(5)); err != nil {
		t.Fatal(err)
	}
}

// Revalidate 以新的 Party 取代舊快照；失去資格（關店）時以 LostEligibility 結束。
func TestRevalidateReplacesThePartySnapshotAndEndsOnLostEligibility(t *testing.T) {
	t.Parallel()
	h := newTestHub(t, newFakeReader(), Config{})
	old := storeParty(5)
	sub, err := h.Subscribe(t.Context(), old, nil)
	if err != nil {
		t.Fatal(err)
	}
	fresh := storeParty(5)
	if err := sub.Revalidate(t.Context(), fresh); err != nil {
		t.Fatal(err)
	}
	sub.mu.Lock()
	replaced := sub.party.Store == fresh.Store && sub.party.Store != old.Store
	sub.mu.Unlock()
	if !replaced {
		t.Fatal("Revalidate 要以新的 Party 取代舊快照")
	}
	closed := storeParty(5)
	st := *closed.Store
	st.Status = seller.Closed
	st.Conditions.Active = false
	closed.Store = &st
	var e *Ended
	if err := sub.Revalidate(t.Context(), closed); !errors.As(err, &e) || e.Reason != EndLostEligibility {
		t.Fatalf("預期 Ended{LostEligibility}，得到 %v", err)
	}
	if _, err := sub.Next(t.Context()); !errors.As(err, &e) || e.Reason != EndLostEligibility {
		t.Fatalf("Next 預期 Ended{LostEligibility}，得到 %v", err)
	}
	if h.streams.(*fakeStreams).deleted != 1 {
		t.Fatal("結束時要刪存活標記")
	}
}
