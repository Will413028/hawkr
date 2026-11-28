//go:build integration

package im_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/live"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// NOTIFY 只在 commit 後送達；payload 欄位正確。
func TestNotifyIsDeliveredOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	raw := f.listenRaw(t)
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		if _, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: f.s1, Type: im.TypeText, Text: "rolled back"}); err != nil {
			return err
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("預期回滾，得到 %v", err)
	}
	if got := f.scalar(t, "SELECT count(*) FROM im.messages"); got != 0 {
		t.Fatalf("回滾後有 %d 則訊息", got)
	}
	if err := f.db.InTx(ctx, func(ctx context.Context) error {
		_, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: f.s1, Type: im.TypeText, Text: "committed"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	f.say(t, f.buyerParty(b1), f.s1, "sentinel")
	got := raw.waitFor(t, f, 2)
	conv := f.conversation(t, b1, f.s1)
	want := []live.Payload{live.Message(conv, b1, f.s1, 1), live.Message(conv, b1, f.s1, 2)}
	if !slices.Equal(got, want) {
		t.Fatalf("回滾的送出不該發通知、commit 的各恰好一個且欄位正確：%+v，預期 %+v", got, want)
	}
}

// 兩個進程（各自的 hub 與專用連線）共用一個資料庫：A 送出、B 的訂閱者收到。
func TestSubscriberOnAnotherModuleReceivesTheMessage(t *testing.T) {
	t.Parallel()
	f := setup(t)
	other := f.newModule(im.LiveConfig{})
	f.run(t, f.m)
	f.run(t, other)
	sub := f.subscribe(t, other, f.storeParty(t, f.s1), nil)
	untilReady(t, sub)
	sent := f.say(t, f.buyerParty(b1), f.s1, "across processes")
	evs := collectUntil(t, sub, messageSeq(sent.ConversationID, 1))
	if m := messages(evs, false); len(m) != 1 || m[0].ID != sent.ID {
		t.Fatalf("B 應收到 A 送出的那一則：%+v", m)
	}
}

// 監聽連線被殺：期間在 delivered 內的對話送出 3 則，重連後以 epoch 補齊，順序正確、不重複。
func TestListenerReconnectBackfillsDeliveredConversations(t *testing.T) {
	t.Parallel()
	f := setup(t)
	gate := make(chan struct{})
	slept := make(chan struct{}, 16)
	cfg := fastBackoff()
	cfg.Sleep = func(ctx context.Context, _ time.Duration) error {
		slept <- struct{}{}
		select {
		case <-gate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m := f.newModule(cfg)
	f.run(t, m)
	first := f.say(t, f.buyerParty(b1), f.s1, "m1")
	conv := first.ConversationID
	sub := f.subscribe(t, m, f.storeParty(t, f.s1), map[uuid.UUID]int64{conv: 0})
	untilReady(t, sub)
	f.exec(t, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid() AND query LIKE 'SELECT now()%'`)
	<-slept
	for i := 2; i <= 4; i++ {
		f.say(t, f.buyerParty(b1), f.s1, fmt.Sprintf("m%d", i))
	}
	close(gate)
	evs := collectUntil(t, sub, messageSeq(conv, 4))
	var seqs []int64
	for _, e := range messages(evs, false) {
		seqs = append(seqs, e.Seq)
	}
	f.say(t, f.buyerParty(b1), f.s1, "sentinel")
	more := collectUntil(t, sub, messageSeq(conv, 5))
	for _, e := range messages(more, false) {
		seqs = append(seqs, e.Seq)
	}
	if want := []int64{2, 3, 4, 5}; !slices.Equal(seqs, want) {
		t.Fatalf("重連後交出的 seq %v，預期依序不重複的 %v", seqs, want)
	}
}

// 重連的 Resync 範圍：中斷期間有新訊息、且不在 delivered 的對話收到 Resync{conversation}；十個訂閱者同時重連沒有 Resync{all}。
func TestListenerReconnectResyncsOnlyActiveUnknownConversations(t *testing.T) {
	t.Parallel()
	f := setup(t)
	gate := make(chan struct{})
	slept := make(chan struct{}, 16)
	cfg := fastBackoff()
	cfg.Sleep = func(ctx context.Context, _ time.Duration) error {
		slept <- struct{}{}
		select {
		case <-gate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m := f.newModule(cfg)
	a := f.say(t, f.buyerParty(b1), f.s1, "a0").ConversationID
	f.say(t, f.buyerParty(b2), f.s1, "b0")
	f.run(t, m)
	var subs []*im.Subscription
	for range 10 {
		s := f.subscribe(t, m, f.storeParty(t, f.s1), nil)
		untilReady(t, s)
		subs = append(subs, s)
	}
	f.exec(t, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid() AND query LIKE 'SELECT now()%'`)
	<-slept
	f.say(t, f.buyerParty(b1), f.s1, "a1")
	close(gate)
	for i, s := range subs {
		evs := collectUntil(t, s, func(e im.Event) bool { return e.Kind == im.EventResync })
		last := evs[len(evs)-1]
		if last.All || last.ConversationID != a {
			t.Fatalf("訂閱者 %d：預期 Resync{%v}，得到 %+v", i, a, last)
		}
	}
}

// 登記先於補洞：補洞讀取前後各插入一則，兩則都恰好收到一次。
func TestRegisterPrecedesBackfill(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.run(t, f.m)
	conv := f.say(t, f.buyerParty(b1), f.s1, "m1").ConversationID
	im.SetLiveHooks(f.m,
		func() { f.say(t, f.buyerParty(b1), f.s1, "before backfill") },
		func() { f.say(t, f.buyerParty(b1), f.s1, "after backfill") },
	)
	sub := f.subscribe(t, f.m, f.storeParty(t, f.s1), map[uuid.UUID]int64{conv: 1})
	evs := collectUntil(t, sub, messageSeq(conv, 3))
	im.SetLiveHooks(f.m, nil, nil)
	f.say(t, f.buyerParty(b1), f.s1, "sentinel")
	evs = append(evs, collectUntil(t, sub, messageSeq(conv, 4))...)
	var seqs []int64
	for _, m := range messages(evs, false) {
		seqs = append(seqs, m.Seq)
	}
	if !slices.Equal(seqs, []int64{2, 3, 4}) {
		t.Fatalf("補洞前後插入的訊息要各收到一次：%v", seqs)
	}
}

func (f *fixture) hideInput(id uuid.UUID) im.HideInput {
	return im.HideInput{MessageID: id, AdminID: 1, Category: string(policy.CommunityAssessable()[0])}
}

// Hide、Restore 在外層 transaction：回滾不發；commit 後雙方收到 Message{Updated}（墓碑 → 還原），seq 相同。
func TestHideAndRestoreNotifyOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.run(t, f.m)
	buyerSub := f.subscribe(t, f.m, f.buyerParty(b1), nil)
	storeSub := f.subscribe(t, f.m, f.storeParty(t, f.s1), nil)
	untilReady(t, buyerSub)
	untilReady(t, storeSub)
	sent := f.say(t, f.buyerParty(b1), f.s1, "to be hidden")
	conv := sent.ConversationID
	for _, s := range []*im.Subscription{buyerSub, storeSub} {
		collectUntil(t, s, messageSeq(conv, 1))
	}
	if err := f.db.InTx(ctx, func(ctx context.Context) error {
		if _, err := f.m.Hide(ctx, f.hideInput(sent.ID)); err != nil {
			return err
		}
		return errRollback
	}); !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	f.say(t, f.buyerParty(b1), f.s1, "sentinel")
	for name, s := range map[string]*im.Subscription{"買家": buyerSub, "店家": storeSub} {
		if n := len(messages(collectUntil(t, s, messageSeq(conv, 2)), true)); n != 0 {
			t.Fatalf("%s：回滾的隱藏不該有更新，得到 %d", name, n)
		}
	}
	if _, err := f.m.Hide(ctx, f.hideInput(sent.ID)); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]*im.Subscription{"買家": buyerSub, "店家": storeSub} {
		evs := collectUntil(t, s, func(e im.Event) bool { return e.Kind == im.EventMessage && e.Updated })
		up := evs[len(evs)-1].Message
		if up.Seq != 1 || up.ConversationID != conv || !up.Hidden || up.Text != nil {
			t.Fatalf("%s：隱藏後要收到同 seq 的墓碑：%+v", name, up)
		}
	}
	if _, err := f.m.Restore(ctx, im.RestoreInput{MessageID: sent.ID, AdminID: 1, Reason: "restored"}); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]*im.Subscription{"買家": buyerSub, "店家": storeSub} {
		evs := collectUntil(t, s, func(e im.Event) bool { return e.Kind == im.EventMessage && e.Updated })
		up := evs[len(evs)-1].Message
		if up.Seq != 1 || up.Hidden || up.Text == nil || *up.Text != "to be hidden" {
			t.Fatalf("%s：還原後要收到同 seq、原內容：%+v", name, up)
		}
	}
}

func TestPartyIdentityIsolation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.run(t, f.m)
	const otherBuyer = int64(7)
	sameID := f.s1
	seedBuyers(t, f.db, []int64{sameID})
	x := f.say(t, f.buyerParty(sameID), f.s2, "buyer to another store").ConversationID
	y := f.say(t, f.buyerParty(otherBuyer), sameID, "another buyer to same id store").ConversationID
	buyer := f.subscribe(t, f.m, f.buyerParty(sameID), map[uuid.UUID]int64{y: 0})
	store := f.subscribe(t, f.m, f.storeParty(t, sameID), map[uuid.UUID]int64{x: 0})
	for name, s := range map[string]*im.Subscription{"買家": buyer, "店家": store} {
		before, _ := untilReady(t, s)
		if len(before) != 0 {
			t.Fatalf("%s：別人的對話 ID 作游標要被忽略，卻收到 %+v", name, before)
		}
	}
	f.exec(t, "SELECT pg_notify('im_events', $1)", live.Message(y, sameID, f.s2, 1).Encode())
	f.say(t, f.buyerParty(sameID), f.s2, "buyer 2")
	f.say(t, f.buyerParty(otherBuyer), sameID, "store 2")
	evs := collectUntil(t, buyer, messageSeq(x, 2))
	for _, m := range messages(evs, false) {
		if m.ConversationID == y {
			t.Fatalf("buyer %d 讀到同 ID 店家的對話：%+v", sameID, m)
		}
	}
	evs = collectUntil(t, store, messageSeq(y, 2))
	for _, m := range messages(evs, false) {
		if m.ConversationID == x {
			t.Fatalf("store %d 讀到同 ID 買家的對話：%+v", sameID, m)
		}
	}
}

// 慢消費者：訂閱者不 Next，1000 則橫跨 600 個對話的送出全部完成（mark 不阻塞）；恢復後收到 Resync{all}。
func TestStalledSubscriberDoesNotBlockSendAndResyncsOnResume(t *testing.T) {
	t.Parallel()
	f := setup(t)
	m := f.newModule(im.LiveConfig{MaxPending: 500})
	buyerIDs := make([]int64, 0, 600)
	for i := range 600 {
		buyerIDs = append(buyerIDs, int64(100+i))
	}
	seedBuyers(t, f.db, buyerIDs)
	f.run(t, m)
	sub := f.subscribe(t, m, f.storeParty(t, f.s1), nil)
	untilReady(t, sub)
	before := im.LiveReceived(m)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 1000 {
			f.say(t, f.buyerParty(int64(100+i%600)), f.s1, "x")
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Minute):
		t.Fatal("送出被停住的訂閱者拖住")
	}
	f.eventually(t, "通知全部抵達", func() bool { return im.LiveReceived(m)-before >= 1000 })
	evs := collectUntil(t, sub, func(e im.Event) bool { return e.Kind == im.EventResync && e.All })
	if len(messages(evs, false)) != 0 {
		t.Fatalf("overflow 之後不交訊息：%d", len(messages(evs, false)))
	}
}

// Revalidate：關店 → LostEligibility；停權 → 不結束且仍收訊息。
func TestRevalidateEndsOnClosedStoreButNotOnSuspension(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.run(t, f.m)
	active := f.storeParty(t, f.s1)
	sub := f.subscribe(t, f.m, active, nil)
	untilReady(t, sub)
	closed := f.storeParty(t, f.s1)
	st := *closed.Store
	st.Status, st.Conditions.Active = seller.Closed, false
	closed.Store = &st
	var e *im.Ended
	if err := sub.Revalidate(ctx, closed); !asEnded(err, &e) || e.Reason != im.EndLostEligibility {
		t.Fatalf("關店：預期 Ended{LostEligibility}，得到 %v", err)
	}
	if _, err := sub.Next(ctx); !asEnded(err, &e) || e.Reason != im.EndLostEligibility {
		t.Fatalf("關店之後 Next：%v", err)
	}

	sub2 := f.subscribe(t, f.m, active, nil)
	untilReady(t, sub2)
	suspended := f.storeParty(t, f.s1)
	st2 := *suspended.Store
	st2.Sanctions = seller.SanctionedBy(seller.EffectiveMeasure{MeasureID: uuid.NewV7(), Kind: seller.MeasureSuspension})
	suspended.Store = &st2
	if err := sub2.Revalidate(ctx, suspended); err != nil {
		t.Fatalf("停權不結束串流：%v", err)
	}
	sent := f.say(t, f.buyerParty(b1), f.s1, "after suspension")
	collectUntil(t, sub2, messageSeq(sent.ConversationID, 1))
}

func asEnded(err error, target **im.Ended) bool {
	return errors.As(err, target)
}

// Run 的 ctx 結束 → 全部 Shutdown、presence 列已刪。
func TestRunShutdownEndsEverySubscriptionAndDeletesRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	stop := f.run(t, f.m)
	a := f.subscribe(t, f.m, f.buyerParty(b1), nil)
	b := f.subscribe(t, f.m, f.storeParty(t, f.s1), nil)
	if got := f.scalar(t, "SELECT count(*) FROM im.presence_streams"); got != 2 {
		t.Fatalf("存活標記 %d，預期 2", got)
	}
	stop()
	for _, s := range []*im.Subscription{a, b} {
		_, err := s.Next(context.Background())
		var e *im.Ended
		if !asEnded(err, &e) || e.Reason != im.EndShutdown {
			t.Fatalf("預期 Ended{Shutdown}，得到 %v", err)
		}
	}
	if got := f.scalar(t, "SELECT count(*) FROM im.presence_streams"); got != 0 {
		t.Fatalf("Run 結束後存活標記 %d，預期 0", got)
	}
}

// 未讀。
func TestUnreadEvents(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	m := f.newModule(im.LiveConfig{})
	f.run(t, m)
	store := f.storeParty(t, f.s1)
	sub := f.subscribe(t, m, store, nil)
	second := f.subscribe(t, m, store, nil)
	untilReady(t, sub)
	untilReady(t, second)
	received := im.LiveReceived(m)
	var conv uuid.UUID
	for i := range 3 {
		conv = f.say(t, f.buyerParty(b1), f.s1, fmt.Sprintf("u%d", i)).ConversationID
	}
	f.eventually(t, "三則通知抵達", func() bool { return im.LiveReceived(m)-received >= 3 })
	evs := collectUntil(t, sub, func(e im.Event) bool { return e.Kind == im.EventUnread })
	if n := len(messages(evs, false)); n != 3 {
		t.Fatalf("一輪三則只算一次總數：第一個 Unread 之前要已交出 3 則，得到 %d", n)
	}
	total, err := f.m.UnreadTotal(ctx, store)
	if err != nil || evs[len(evs)-1].Unread != total || total != 3 {
		t.Fatalf("Unread %d，UnreadTotal %d（%v），預期都是 3", evs[len(evs)-1].Unread, total, err)
	}
	collectUntil(t, second, func(e im.Event) bool { return e.Kind == im.EventUnread })
	f.say(t, f.buyerParty(b1), f.s1, "fourth")

	if err := f.m.MarkRead(ctx, conv, store, 4); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]*im.Subscription{"原串流": sub, "第二個串流": second} {
		ev := collectUntil(t, s, func(e im.Event) bool { return e.Kind == im.EventUnread && e.Unread == 0 })
		if ev[len(ev)-1].Unread != 0 {
			t.Fatalf("%s：MarkRead 後未讀要是 0", name)
		}
	}

	raw := f.listenRaw(t)
	f.say(t, f.buyerParty(b1), f.s1, "again")
	f.say(t, f.buyerParty(b2), f.s1, "other conversation")
	raw.waitFor(t, f, 2)
	if err := f.m.MarkAllRead(ctx, store); err != nil {
		t.Fatal(err)
	}
	f.say(t, f.buyerParty(b2), f.s1, "tail")
	all := raw.waitFor(t, f, 4)
	var batch int
	var reads int
	for _, p := range all {
		switch p.Kind {
		case live.KindReadBatch:
			batch++
		case live.KindRead:
			reads++
		}
	}
	if batch != 1 || reads != 0 {
		t.Fatalf("MarkAllRead 要發一個 u、不逐對話發 r：u=%d r=%d", batch, reads)
	}
}

// PeerRead：前進才通知；沒前進（重送）不發 NOTIFY；MarkAllRead 不產生 PeerRead。
func TestPeerReadOnlyWhenTheCursorAdvances(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.run(t, f.m)
	buyerSub := f.subscribe(t, f.m, f.buyerParty(b1), nil)
	untilReady(t, buyerSub)
	conv := f.say(t, f.buyerParty(b1), f.s1, "hello").ConversationID
	store := f.storeParty(t, f.s1)
	raw := f.listenRaw(t)
	if err := f.m.MarkRead(ctx, conv, store, 1); err != nil {
		t.Fatal(err)
	}
	if err := f.m.MarkRead(ctx, conv, store, 1); err != nil {
		t.Fatal(err)
	}
	f.say(t, f.buyerParty(b1), f.s1, "second")
	got := raw.waitFor(t, f, 2)
	if len(got) != 2 || got[0].Kind != live.KindRead || got[0].Seq != 1 || got[0].Side != live.SideStore || got[1].Kind != live.KindMessage {
		t.Fatalf("游標沒前進不發通知：%+v", got)
	}
	var sawMessage, sawPeer bool
	evs := collectUntil(t, buyerSub, func(e im.Event) bool {
		sawMessage = sawMessage || messageSeq(conv, 2)(e)
		sawPeer = sawPeer || e.Kind == im.EventPeerRead
		return sawMessage && sawPeer
	})
	var peer []im.Event
	for _, e := range evs {
		if e.Kind == im.EventPeerRead {
			peer = append(peer, e)
		}
	}
	if len(peer) != 1 || peer[0].ConversationID != conv || peer[0].PeerReadSeq != 1 {
		t.Fatalf("預期恰好一個 PeerRead{conv, 1}：%+v", peer)
	}
	if err := f.m.MarkAllRead(ctx, store); err != nil {
		t.Fatal(err)
	}
	f.say(t, f.buyerParty(b1), f.s1, "third")
	evs = collectUntil(t, buyerSub, messageSeq(conv, 3))
	if n := count(evs, im.EventPeerRead); n != 0 {
		t.Fatalf("MarkAllRead 不產生 PeerRead，得到 %d", n)
	}
}

// presence。
func TestPresence(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	beat := make(chan time.Time, 1)
	cfg := im.LiveConfig{
		PresenceBeat: 20 * time.Second,
		NewTicker: func(d time.Duration) (<-chan time.Time, func()) {
			if d == 20*time.Second {
				return beat, func() {}
			}
			return make(chan time.Time), func() {}
		},
	}
	m := f.newModule(cfg)
	f.run(t, m)
	conv := f.say(t, f.buyerParty(b1), f.s1, "hi").ConversationID
	buyerP := f.buyerParty(b1)
	online := func() bool {
		st, err := f.m.Presence(ctx, buyerP, []uuid.UUID{conv})
		if err != nil || len(st) != 1 || st[0].ConversationID != conv {
			t.Fatalf("Presence：%+v %v", st, err)
		}
		return st[0].Online
	}
	if online() {
		t.Fatal("沒有串流時對方離線")
	}
	sub := f.subscribe(t, m, f.storeParty(t, f.s1), nil)
	if !online() {
		t.Fatal("對方開了串流要在線")
	}
	f.exec(t, "UPDATE im.presence_streams SET expires_at = now() + interval '1 second'")
	beat <- time.Time{}
	f.eventually(t, "延長批次", func() bool {
		return f.scalar(t, "SELECT count(*) FROM im.presence_streams WHERE expires_at > now() + interval '30 seconds'") == 1
	})
	f.exec(t, "UPDATE im.presence_streams SET expires_at = now() - interval '1 second'")
	if online() {
		t.Fatal("過期的列不算在線")
	}
	f.exec(t, "UPDATE im.presence_streams SET expires_at = now() + interval '1 minute'")
	if !online() {
		t.Fatal("未過期要在線")
	}
	if _, err := f.buyers.owner.BlockStore(ctx, b1, f.s1); err != nil {
		t.Fatal(err)
	}
	if online() {
		t.Fatal("買家封鎖店家要離線")
	}
	if _, err := f.buyers.owner.UnblockStore(ctx, b1, f.s1); err != nil {
		t.Fatal(err)
	}
	if !online() {
		t.Fatal("解除封鎖後要在線")
	}
	f.stores.block = true
	if online() {
		t.Fatal("店家封鎖買家要離線")
	}
	f.stores.block = false
	f.stores.blockErr = errRollback
	if _, err := f.m.Presence(ctx, buyerP, []uuid.UUID{conv}); err == nil {
		t.Fatal("封鎖查詢失敗要回錯誤，不放行")
	}
	f.stores.blockErr = nil
	if _, err := f.m.Presence(ctx, f.buyerParty(b2), []uuid.UUID{conv}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("非共有對話要 NotFound：%v", err)
	}
	sub.Close()
	if online() {
		t.Fatal("Close 之後要離線")
	}
	sub = f.subscribe(t, m, f.storeParty(t, f.s1), nil)
	if !online() {
		t.Fatal("重開串流後要在線")
	}
	store, err := f.stores.owner.Store(ctx, f.s1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.stores.owner.CloseCurrent(ctx, store.AccountID); err != nil {
		t.Fatal(err)
	}
	f.advance(seller.StoreRetireAfter + time.Second)
	if _, err := f.stores.owner.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if online() {
		t.Fatal("對方在 seller.pub_retired_stores() 要離線")
	}
	sub.Close()
	over := make([]uuid.UUID, im.MaxPresenceConversations+1)
	if _, err := f.m.Presence(ctx, buyerP, over); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("超過上限要 InvalidInput：%v", err)
	}
}

// 店家呼叫：對方是買家；店家封鎖買家（BuyersBlockedByStore）、買家封鎖店家、封鎖查詢失敗；一次多個對話、順序對應輸入、含別人的對話整個 NotFound。
func TestPresenceStoreCallerAndBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	m := f.newModule(im.LiveConfig{})
	f.run(t, m)
	c1 := f.say(t, f.buyerParty(b1), f.s1, "hi").ConversationID
	c2 := f.say(t, f.buyerParty(b2), f.s1, "hi").ConversationID
	other := f.say(t, f.buyerParty(b1), f.s2, "hi").ConversationID
	storeP := f.storeParty(t, f.s1)
	states := func(ids ...uuid.UUID) []bool {
		st, err := f.m.Presence(ctx, storeP, ids)
		if err != nil || len(st) != len(ids) {
			t.Fatalf("Presence：%+v %v", st, err)
		}
		out := make([]bool, len(st))
		for i, x := range st {
			if x.ConversationID != ids[i] {
				t.Fatalf("順序要對應輸入：%+v", st)
			}
			out[i] = x.Online
		}
		return out
	}
	sub := f.subscribe(t, m, f.buyerParty(b1), nil)
	defer sub.Close()
	if got := states(c2, c1, c1); got[0] || !got[1] || !got[2] {
		t.Fatalf("只有 b1 開了串流，重複的 ID 各回一筆：%v", got)
	}
	f.stores.block = true
	if got := states(c1, c2); got[0] || got[1] {
		t.Fatalf("店家封鎖買家，買家對店家顯示離線：%v", got)
	}
	f.stores.blockErr = errRollback
	if _, err := f.m.Presence(ctx, storeP, []uuid.UUID{c1}); err == nil {
		t.Fatal("店家端的封鎖查詢失敗要回錯誤，不放行")
	}
	f.stores.blockErr = nil
	f.stores.block = false
	if got := states(c1); !got[0] {
		t.Fatalf("沒有封鎖要在線：%v", got)
	}
	if _, err := f.buyers.owner.BlockStore(ctx, b1, f.s1); err != nil {
		t.Fatal(err)
	}
	if got := states(c1, c2); got[0] || got[1] {
		t.Fatalf("買家封鎖店家，店家看買家離線：%v", got)
	}
	if st, err := f.m.Presence(ctx, storeP, nil); err != nil || len(st) != 0 {
		t.Fatalf("空清單：%+v %v", st, err)
	}
	if _, err := f.m.Presence(ctx, storeP, []uuid.UUID{c1, other}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("含一個不是自己的對話時整個請求要 NotFound：%v", err)
	}
	if _, err := f.m.Presence(ctx, storeP, []uuid.UUID{c1, uuid.New()}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("含不存在的對話時整個請求要 NotFound：%v", err)
	}
}

// Subscribe 的授權與全域並行上限。
func TestSubscribeLimitsAndAuthorization(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	m1, m2 := f.newModule(im.LiveConfig{}), f.newModule(im.LiveConfig{})
	p := f.buyerParty(b1)

	closed := f.storeParty(t, f.s1)
	st := *closed.Store
	st.Status, st.Conditions.Active = seller.Closed, false
	closed.Store = &st
	if _, err := m1.Subscribe(ctx, closed, nil); err == nil {
		t.Fatal("關店的店家不能訂閱")
	}
	cursors := map[uuid.UUID]int64{}
	for range 51 {
		cursors[uuid.NewV7()] = 0
	}
	if _, err := m1.Subscribe(ctx, p, cursors); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("游標超過上限要 InvalidInput：%v", err)
	}

	var subs []*im.Subscription
	for i := range 10 {
		m := m1
		if i >= 6 {
			m = m2
		}
		subs = append(subs, f.subscribe(t, m, p, nil))
	}
	if _, err := m2.Subscribe(ctx, p, nil); kindOf(err) != apperr.KindResourceExhausted {
		t.Fatalf("跨進程合計第 11 條要被拒：%v", err)
	}
	subs[0].Close()
	again, err := m2.Subscribe(ctx, p, nil)
	if err != nil {
		t.Fatalf("Close 之後名額要釋出：%v", err)
	}
	again.Close()
	for _, s := range subs {
		s.Close()
	}

	other := f.buyerParty(b2)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, exhausted := 0, 0
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := m1
			if i%2 == 1 {
				m = m2
			}
			s, err := m.Subscribe(ctx, other, nil)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
				t.Cleanup(s.Close)
			case kindOf(err) == apperr.KindResourceExhausted:
				exhausted++
			default:
				t.Errorf("Subscribe：%v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 10 || exhausted != 2 {
		t.Fatalf("並行 Subscribe：成功 %d、被拒 %d，預期 10 與 2", ok, exhausted)
	}

	seedBuyers(t, f.db, []int64{99})
	f.exec(t, "ALTER TABLE im.presence_streams RENAME TO presence_streams_gone")
	_, err = m1.Subscribe(ctx, f.buyerParty(99), nil)
	if err == nil || kindOf(err) == apperr.KindResourceExhausted {
		t.Fatalf("寫入失敗要讓 Subscribe 失敗：%v", err)
	}
}

// listener 不佔連線池。
func TestListenerDoesNotHoldAPoolConnection(t *testing.T) {
	t.Parallel()
	f := setup(t)
	before := f.db.Pool().Stat().AcquiredConns()
	f.run(t, f.m)
	if after := f.db.Pool().Stat().AcquiredConns(); after != before {
		t.Fatalf("listener 運行時連線池被借走的連線 %d → %d", before, after)
	}
}

// 連貫性：同一對話 50 則並行送出，串流收到的 seq 嚴格遞增且連續。
func TestConcurrentSendsAreDeliveredInSeqOrder(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.run(t, f.m)
	conv := f.say(t, f.buyerParty(b1), f.s1, "first").ConversationID
	sub := f.subscribe(t, f.m, f.storeParty(t, f.s1), map[uuid.UUID]int64{conv: 1})
	untilReady(t, sub)
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.say(t, f.buyerParty(b1), f.s1, fmt.Sprintf("c%d", i))
		}()
	}
	wg.Wait()
	evs := collectUntil(t, sub, messageSeq(conv, 51))
	var seqs []int64
	for _, m := range messages(evs, false) {
		seqs = append(seqs, m.Seq)
	}
	if len(seqs) != 50 {
		t.Fatalf("收到 %d 則，預期 50：%v", len(seqs), seqs)
	}
	for i, s := range seqs {
		if s != int64(i)+2 {
			t.Fatalf("seq 要嚴格遞增且連續：%v", seqs)
		}
	}
}

// 中斷期間只有隱藏與已讀（沒有新訊息、沒有游標變化）：重連後 delivered 內的對話仍收到 Resync{conversation}。
func TestListenerReconnectResyncsDeliveredConversationsWithHiddenAndReadChanges(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	gate := make(chan struct{})
	slept := make(chan struct{}, 16)
	cfg := fastBackoff()
	cfg.Sleep = func(ctx context.Context, _ time.Duration) error {
		slept <- struct{}{}
		select {
		case <-gate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m := f.newModule(cfg)
	f.run(t, m)
	sent := f.say(t, f.buyerParty(b1), f.s1, "m1")
	conv := sent.ConversationID
	sub := f.subscribe(t, m, f.buyerParty(b1), map[uuid.UUID]int64{conv: 1})
	untilReady(t, sub)
	f.exec(t, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid() AND query LIKE 'SELECT now()%'`)
	<-slept
	if _, err := f.m.Hide(ctx, f.hideInput(sent.ID)); err != nil {
		t.Fatal(err)
	}
	if err := f.m.MarkRead(ctx, conv, f.storeParty(t, f.s1), 1); err != nil {
		t.Fatal(err)
	}
	close(gate)
	evs := collectUntil(t, sub, func(e im.Event) bool { return e.Kind == im.EventResync && e.ConversationID == conv })
	if last := evs[len(evs)-1]; last.All {
		t.Fatalf("預期 Resync{%v}，得到 %+v", conv, last)
	}
}

// 在第一次 LISTEN 成功之前就開的串流：之後 commit 的訊息在第一條連線連上時補上。
func TestStreamOpenedBeforeTheFirstListenReceivesLaterCommits(t *testing.T) {
	t.Parallel()
	f := setup(t)
	conv := f.say(t, f.buyerParty(b1), f.s1, "m1").ConversationID
	m := f.newModule(fastBackoff())
	dialing := make(chan struct{}, 1)
	gate := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- im.RunWithGatedDial(ctx, m, dialing, gate) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run 結束：%v", err)
		}
	})
	select {
	case <-dialing:
	case <-time.After(guard):
		t.Fatal("等不到第一次撥號")
	}
	sub := f.subscribe(t, m, f.storeParty(t, f.s1), map[uuid.UUID]int64{conv: 1})
	untilReady(t, sub)
	f.say(t, f.buyerParty(b1), f.s1, "m2")
	close(gate)
	collectUntil(t, sub, messageSeq(conv, 2))
}

// 延長批次把已被清掉的存活標記補回，串流仍被計數。
func TestPresenceExtendReinsertsSweptRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	beat := make(chan time.Time)
	const beatEvery = 3 * time.Hour
	cfg := fastBackoff()
	cfg.PresenceBeat = beatEvery
	cfg.NewTicker = func(d time.Duration) (<-chan time.Time, func()) {
		if d == beatEvery {
			return beat, func() {}
		}
		return make(chan time.Time), func() {}
	}
	m := f.newModule(cfg)
	f.run(t, m)
	sub := f.subscribe(t, m, f.buyerParty(b1), nil)
	untilReady(t, sub)
	if n := f.scalar(t, `SELECT count(*) FROM im.presence_streams WHERE kind = 'BUYER' AND party_id = $1`, b1); n != 1 {
		t.Fatalf("訂閱後應有 1 列，得到 %d", n)
	}
	f.exec(t, `DELETE FROM im.presence_streams`)
	beat <- time.Time{}
	beat <- time.Time{}
	if n := f.scalar(t, `SELECT count(*) FROM im.presence_streams WHERE kind = 'BUYER' AND party_id = $1 AND expires_at > now()`, b1); n != 1 {
		t.Fatalf("延長後被清掉的列要補回，得到 %d", n)
	}
}

// 兩個並行的 MarkRead 推進同一個游標：後到的在列鎖上等，拿到前者提交後的值，所以只有一個 NOTIFY r。
func TestConcurrentMarkReadNotifiesOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	conv := f.say(t, f.buyerParty(b1), f.s1, "hello").ConversationID
	store := f.storeParty(t, f.s1)
	raw := f.listenRaw(t)
	first := make(chan struct{})
	release := make(chan struct{})
	holder := make(chan error, 1)
	go func() {
		holder <- f.db.InTx(ctx, func(ctx context.Context) error {
			if err := f.m.MarkRead(ctx, conv, store, 1); err != nil {
				return err
			}
			close(first)
			<-release
			return nil
		})
	}()
	<-first
	second := make(chan error, 1)
	go func() { second <- f.m.MarkRead(ctx, conv, store, 1) }()
	f.eventually(t, "第二個 MarkRead 在列鎖上等", func() bool {
		return f.scalar(t, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`) > 0
	})
	close(release)
	if err := <-holder; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	f.say(t, f.buyerParty(b1), f.s1, "sentinel")
	got := raw.waitFor(t, f, 2)
	reads := 0
	for _, p := range got {
		if p.Kind == live.KindRead {
			reads++
		}
	}
	if reads != 1 {
		t.Fatalf("預期恰好一個 r 通知，得到 %d：%+v", reads, got)
	}
}

// 訊息的 created_at 就是配發 seq 時寫進對話的 last_message_at（同一個資料庫時鐘值）。
func TestMessageTimeIsTheConversationLastMessageTime(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.say(t, f.buyerParty(b1), f.s1, "first")
	f.say(t, f.buyerParty(b1), f.s1, "second")
	n := f.scalar(t, `SELECT count(*) FROM im.messages m JOIN im.conversations c ON c.id = m.conversation_id AND c.last_seq = m.seq WHERE m.created_at = c.last_message_at`)
	if n != 1 {
		t.Fatalf("最後一則訊息的 created_at 應等於 last_message_at，符合的對話 %d", n)
	}
}
