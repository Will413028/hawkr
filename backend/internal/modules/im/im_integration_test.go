//go:build integration

package im_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

const (
	b1 = int64(11)
	b2 = int64(12)
	s1 = int64(22)
	s2 = int64(23)
)

// seq：兩個 goroutine 對同一個（還不存在的）對話並行送 50 則，seq 是 1..100 且不重複，對話只有一列；結束時寄件人各自那一方的游標等於
// 他最後一則的 seq。
func TestSeqIsConsecutiveUnderConcurrentFirstSends(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var mu sync.Mutex
	var seqs []int64
	var wg sync.WaitGroup
	send := func(from im.Party, counterpart int64) {
		defer wg.Done()
		for i := range 50 {
			m, err := f.m.Send(ctx, im.SendInput{Sender: from, CounterpartID: counterpart, Type: im.TypeText, Text: fmt.Sprintf("m%d", i)})
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			seqs = append(seqs, m.Seq)
			mu.Unlock()
		}
	}
	wg.Add(2)
	go send(f.buyerParty(b1), s1)
	go send(f.storeParty(t, s1), b1)
	wg.Wait()
	if t.Failed() {
		return
	}
	slices.Sort(seqs)
	for i, s := range seqs {
		if s != int64(i+1) {
			t.Fatalf("seq 要是 1..100 不重複：第 %d 個是 %d", i, s)
		}
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.conversations"); n != 1 {
		t.Fatalf("對話 %d 列", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 100 {
		t.Fatalf("訊息 %d 則", n)
	}
	for kind, col := range map[string]string{"BUYER": "buyer_last_read_seq", "STORE": "store_last_read_seq"} {
		last := f.scalar(t, "SELECT max(seq) FROM im.messages WHERE sender_kind = $1", kind)
		if got := f.scalar(t, "SELECT "+col+" FROM im.conversations"); got != last {
			t.Fatalf("%s 的游標 %d，他最後一則的 seq 是 %d", kind, got, last)
		}
	}
}

// 已讀游標：買家寄 3 則後店家的未讀是 3、買家是 0；MarkRead 只前進、不超過 last_seq；CHECK 擋下超出範圍的游標。
func TestUnreadEquationAndMarkRead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for i := range 3 {
		f.say(t, f.buyerParty(b1), s1, fmt.Sprintf("m%d", i))
	}
	conv := f.conversation(t, b1, s1)
	store := f.storeParty(t, s1)
	buyerP := f.buyerParty(b1)
	unread := func(p im.Party) int64 {
		n, err := f.m.UnreadTotal(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := unread(store); got != 3 {
		t.Fatalf("店家未讀 %d，預期 3", got)
	}
	if got := unread(buyerP); got != 0 {
		t.Fatalf("寄件人自己的未讀 %d，預期 0", got)
	}
	if err := f.m.MarkRead(ctx, conv, store, 99); err != nil {
		t.Fatal(err)
	}
	if got := f.scalar(t, "SELECT store_last_read_seq FROM im.conversations"); got != 3 {
		t.Fatalf("MarkRead(99) 只推到 last_seq：%d", got)
	}
	if err := f.m.MarkRead(ctx, conv, store, 1); err != nil {
		t.Fatal(err)
	}
	if got := f.scalar(t, "SELECT store_last_read_seq FROM im.conversations"); got != 3 {
		t.Fatalf("MarkRead 給較小的 seq 不倒退：%d", got)
	}
	if got := unread(store); got != 0 {
		t.Fatalf("已讀後未讀 %d", got)
	}
	for _, sql := range []string{
		"UPDATE im.conversations SET buyer_last_read_seq = last_seq + 1",
		"UPDATE im.conversations SET store_last_read_seq = last_seq + 1",
		"UPDATE im.conversations SET buyer_last_read_seq = -1",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, sql); err == nil {
			t.Fatalf("CHECK 要擋下：%s", sql)
		}
	}
	f.say(t, f.buyerParty(b1), s1, "again")
	if err := f.m.MarkAllRead(ctx, store); err != nil {
		t.Fatal(err)
	}
	if got := unread(store); got != 0 {
		t.Fatalf("MarkAllRead 後未讀 %d", got)
	}
}

// 讀取方法不改狀態（E12）：ListMessages、MessagesAfter、ListConversations、UnreadTotal 跑完後兩個游標都不變。
func TestReadsDoNotMoveTheReadCursors(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "a")
	f.say(t, f.storeParty(t, s1), b1, "b")
	f.say(t, f.buyerParty(b1), s1, "c")
	conv := f.conversation(t, b1, s1)
	cursors := func() [2]int64 {
		return [2]int64{f.scalar(t, "SELECT buyer_last_read_seq FROM im.conversations"), f.scalar(t, "SELECT store_last_read_seq FROM im.conversations")}
	}
	before := cursors()
	for _, p := range []im.Party{f.buyerParty(b1), f.storeParty(t, s1)} {
		if _, err := f.m.ListMessages(ctx, conv, p, 0, 10); err != nil {
			t.Fatal(err)
		}
		if _, err := f.m.MessagesAfter(ctx, conv, p, 0, 10); err != nil {
			t.Fatal(err)
		}
		if _, err := f.m.ListConversations(ctx, p, nil, 10); err != nil {
			t.Fatal(err)
		}
		if _, err := f.m.UnreadTotal(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	if after := cursors(); after != before {
		t.Fatalf("讀取改了游標：%v -> %v", before, after)
	}
}

// ID9：插入 25 則，以 beforeSeq 翻三頁，不重複、不漏；翻頁中間有新訊息插入時頁面穩定。
func TestListMessagesKeysetPagesAreStable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for i := range 25 {
		f.say(t, f.buyerParty(b1), s1, fmt.Sprintf("m%d", i))
	}
	conv := f.conversation(t, b1, s1)
	p := f.buyerParty(b1)
	var seen []int64
	var before int64
	for page := range 3 {
		got, err := f.m.ListMessages(ctx, conv, p, before, 10)
		if err != nil {
			t.Fatal(err)
		}
		if want := []int{10, 10, 5}[page]; len(got.Items) != want {
			t.Fatalf("第 %d 頁 %d 則，預期 %d", page, len(got.Items), want)
		}
		if got.HasMore != (page < 2) {
			t.Fatalf("第 %d 頁 HasMore=%v", page, got.HasMore)
		}
		for _, m := range got.Items {
			seen = append(seen, m.Seq)
		}
		before = got.Items[len(got.Items)-1].Seq
		if page == 0 {
			f.say(t, f.buyerParty(b1), s1, "inserted between pages")
		}
	}
	if len(seen) != 25 {
		t.Fatalf("共 %d 則", len(seen))
	}
	for i, s := range seen {
		if want := int64(25 - i); s != want {
			t.Fatalf("第 %d 則是 seq %d，預期 %d（不重複、不漏、新到的訊息不擠進舊頁）", i, s, want)
		}
	}
	after, err := f.m.MessagesAfter(ctx, conv, p, 24, 10)
	if err != nil || len(after.Items) != 2 || after.Items[0].Seq != 25 || after.Items[1].Seq != 26 {
		t.Fatalf("MessagesAfter(24) = %+v %v", after, err)
	}
}

// ID6：A 的對話 ID 交給 B 讀或寫，回 NotFound；同一個數字同時是 buyer_id 與 store_id 時不會讀到另一個身分空間的對話。
func TestConversationAuthorizationIsByIDAndSide(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "hi")
	conv := f.conversation(t, b1, s1)
	// 買家 22、店家 11：數字與對話的 (11, 22) 對調。
	wrong := []im.Party{f.buyerParty(b2), f.buyerParty(s1), f.storeParty(t, s2), f.storeParty(t, b1)}
	for i, p := range wrong {
		if _, err := f.m.ListMessages(ctx, conv, p, 0, 10); kindOf(err) != apperr.KindNotFound {
			t.Fatalf("案例 %d ListMessages：%v", i, err)
		}
		if _, err := f.m.MessagesAfter(ctx, conv, p, 0, 10); kindOf(err) != apperr.KindNotFound {
			t.Fatalf("案例 %d MessagesAfter：%v", i, err)
		}
		if err := f.m.MarkRead(ctx, conv, p, 1); kindOf(err) != apperr.KindNotFound {
			t.Fatalf("案例 %d MarkRead：%v", i, err)
		}
		in := im.SendInput{Sender: p, ConversationID: conv, Type: im.TypeText, Text: "intrude"}
		if _, err := f.m.Send(ctx, in); kindOf(err) != apperr.KindNotFound {
			t.Fatalf("案例 %d Send：%v", i, err)
		}
		page, err := f.m.ListConversations(ctx, p, nil, 10)
		if err != nil || len(page.Items) != 0 {
			t.Fatalf("案例 %d ListConversations 不該看到別人的對話：%+v %v", i, page, err)
		}
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 1 {
		t.Fatalf("越權的送出留下了訊息：%d", n)
	}
	if _, err := f.m.ListMessages(ctx, uuid.New(), f.buyerParty(b1), 0, 10); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的對話：%v", err)
	}
}

// ID5：seller.Store 或 Orders 回錯誤時，送出回錯誤，訊息表沒有新列。
func TestSendFailsClosedAndWritesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("store lookup fails", func(t *testing.T) {
		f := setup(t)
		f.stores.err = fmt.Errorf("seller down")
		_, err := f.m.Send(ctx, im.SendInput{Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "hi"})
		if kindOf(err) != apperr.KindUnavailable {
			t.Fatalf("err = %v", err)
		}
		if n := f.scalar(t, "SELECT count(*) FROM im.messages") + f.scalar(t, "SELECT count(*) FROM im.conversations"); n != 0 {
			t.Fatalf("留下了 %d 列", n)
		}
	})
	t.Run("orders lookup fails while blocked", func(t *testing.T) {
		f := setup(t)
		f.buyers.blocking = true
		f.orders.err = fmt.Errorf("order-service down")
		_, err := f.m.Send(ctx, im.SendInput{Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "hi"})
		if kindOf(err) != apperr.KindUnavailable {
			t.Fatalf("err = %v", err)
		}
		if n := f.scalar(t, "SELECT count(*) FROM im.messages") + f.scalar(t, "SELECT count(*) FROM im.conversations"); n != 0 {
			t.Fatalf("留下了 %d 列", n)
		}
	})
	t.Run("blocked with an open order goes through", func(t *testing.T) {
		f := setup(t)
		f.stores.block = true
		f.orders.open = true
		f.say(t, f.storeParty(t, s1), b1, "about your order")
	})
}

// ID1：隱藏最後一則之後，列表的預覽 Hidden=true 而且不帶文字；還原後恢復；抹除之後 Erased=true。
func TestPreviewIsDerivedFromTheLastMessage(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "first")
	last := f.say(t, f.storeParty(t, s1), b1, "秘密內容")
	p := f.buyerParty(b1)
	preview := func() im.Preview {
		page, err := f.m.ListConversations(ctx, p, nil, 10)
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("%+v %v", page, err)
		}
		return page.Items[0].Preview
	}
	if pv := preview(); pv.Text == nil || *pv.Text != "秘密內容" || pv.SenderKind != im.PartyStore {
		t.Fatalf("可見的預覽：%+v", pv)
	}
	h, err := f.m.Hide(ctx, im.HideInput{MessageID: last.ID, AdminID: 1, Category: string(policy.CommunityAssessable()[0])})
	if err != nil || !h.Created {
		t.Fatalf("%+v %v", h, err)
	}
	if again, err := f.m.Hide(ctx, im.HideInput{MessageID: last.ID, AdminID: 2, Category: string(policy.CommunityAssessable()[0])}); err != nil || again.Created || again.ID != h.ID {
		t.Fatalf("重複隱藏要回既有那一筆：%+v %v", again, err)
	}
	if pv := preview(); !pv.Hidden || pv.Text != nil {
		t.Fatalf("隱藏後的預覽洩漏內容：%+v", pv)
	}
	msgs, err := f.m.ListMessages(ctx, f.conversation(t, b1, s1), p, 0, 10)
	if err != nil || !msgs.Items[0].Hidden || msgs.Items[0].Text != nil {
		t.Fatalf("隱藏的訊息要回墓碑：%+v %v", msgs, err)
	}
	if _, err := f.m.Restore(ctx, im.RestoreInput{MessageID: last.ID, AdminID: 1, Reason: "誤判"}); err != nil {
		t.Fatal(err)
	}
	if pv := preview(); pv.Hidden || pv.Text == nil {
		t.Fatalf("還原後的預覽：%+v", pv)
	}
	if _, err := f.m.Restore(ctx, im.RestoreInput{MessageID: last.ID, AdminID: 1, Reason: "again"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有未還原的隱藏：%v", err)
	}
	f.deactivate(t, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, seller.StoreRetired{EventID: uuid.New(), OccurredAt: f.clock(), StoreID: s1, AccountID: 2000 + s1})
	})
	if err := f.m.ErasePartyMessages(ctx, im.PartyStore, s1); err != nil {
		t.Fatal(err)
	}
	if pv := preview(); !pv.Erased || pv.Text != nil {
		t.Fatalf("抹除後的預覽：%+v", pv)
	}
}

func (f *fixture) deactivate(t *testing.T, publish func(context.Context) error) {
	t.Helper()
	if err := f.db.InTx(context.Background(), publish); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) buyerDeleted(t *testing.T, buyerID int64) {
	t.Helper()
	f.deactivate(t, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, buyer.BuyerDeleted{EventID: uuid.New(), OccurredAt: f.clock(), BuyerID: buyerID, AccountID: 1000 + buyerID})
	})
}

func (f *fixture) erased(t *testing.T, kind string) (erased, kept int64) {
	t.Helper()
	return f.scalar(t, "SELECT count(*) FROM im.messages WHERE sender_kind = $1 AND erased_at IS NOT NULL", kind),
		f.scalar(t, "SELECT count(*) FROM im.messages WHERE sender_kind = $1 AND erased_at IS NULL AND content IS NOT NULL", kind)
}

// ID3：同一個帳號有買家與店家兩個身分；BuyerDeleted 之後只有買家身分寄的訊息被抹除，店家身分寄的還在；StoreRetired 反之。
func TestErasureIsPerIdentity(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	// 帳號 5000 同時是買家 11 與店家 22（店家 22 與買家 12 對話，買家 11 與店家 23 對話：自己不跟自己聊）。
	f.buyers.accounts[b1] = 5000
	f.stores.byID[s1] = activeStore(s1, 5000)
	f.say(t, f.buyerParty(b1), s2, "buyer identity message")
	f.say(t, f.storeParty(t, s1), b2, "store identity message")
	f.say(t, f.buyerParty(b2), s1, "other buyer")
	f.say(t, f.storeParty(t, s2), b1, "other store")

	f.buyerDeleted(t, b1)
	if err := f.m.ErasePartyMessages(ctx, im.PartyBuyer, b1); err != nil {
		t.Fatal(err)
	}
	if got := f.scalar(t, "SELECT count(*) FROM im.messages m JOIN im.conversations c ON c.id = m.conversation_id WHERE c.buyer_id = $1 AND m.sender_kind = 'BUYER' AND m.erased_at IS NOT NULL", b1); got != 1 {
		t.Fatalf("買家 11 寄的訊息要被抹除：%d", got)
	}
	if got := f.scalar(t, "SELECT count(*) FROM im.messages m JOIN im.conversations c ON c.id = m.conversation_id WHERE c.store_id = $1 AND m.sender_kind = 'STORE' AND m.erased_at IS NULL", s1); got != 1 {
		t.Fatalf("同一個帳號的店家身分寄的訊息要還在：%d", got)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages WHERE erased_at IS NOT NULL"); n != 1 {
		t.Fatalf("只該抹一則：%d", n)
	}

	f.deactivate(t, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, seller.StoreRetired{EventID: uuid.New(), OccurredAt: f.clock(), StoreID: s1, AccountID: 5000})
	})
	if err := f.m.ErasePartyMessages(ctx, im.PartyStore, s1); err != nil {
		t.Fatal(err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages WHERE erased_at IS NOT NULL"); n != 2 {
		t.Fatalf("店家 22 退場後共抹 2 則：%d", n)
	}
	if got := f.scalar(t, "SELECT count(*) FROM im.messages WHERE content = 'other buyer' AND erased_at IS NULL"); got != 1 {
		t.Fatal("別的買家寄給該店的訊息要還在")
	}
	if got := f.scalar(t, "SELECT count(*) FROM im.messages WHERE content = 'other store' AND erased_at IS NULL"); got != 1 {
		t.Fatal("別的店寄給該買家的訊息要還在")
	}
}

// 抹除之後送出被擋：寄件人已刪除是 FailedPrecondition，寄給已刪除者是 RECEIVER_DELETED；兩者都不留列。
func TestInactivePartiesCannotSendOrReceive(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "hi")
	f.buyerDeleted(t, b1)
	before := f.scalar(t, "SELECT count(*) FROM im.messages")
	if _, err := f.m.Send(ctx, im.SendInput{Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "again"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已刪除的寄件人：%v", err)
	}
	_, err := f.m.Send(ctx, im.SendInput{Sender: f.storeParty(t, s1), CounterpartID: b1, Type: im.TypeText, Text: "anyone there"})
	if kindOf(err) != apperr.KindFailedPrecondition || im.ReasonOf(err) != im.ReasonReceiverDeleted {
		t.Fatalf("寄給已刪除者：%v", err)
	}
	if got := f.scalar(t, "SELECT count(*) FROM im.messages"); got != before {
		t.Fatalf("被擋的送出留下了列：%d -> %d", before, got)
	}
}

// 刪帳的訂閱者排抹除 job（拿掉 Enqueue 時這個測試失敗），而且與複本在同一個 transaction（排入失敗時複本一起回滾）。
func TestSubscriberEnqueuesTheEraseJobAtomically(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.say(t, f.buyerParty(b1), s1, "hi")
	f.buyerDeleted(t, b1)
	if n := f.jobs.count("im.erase_party"); n != 1 {
		t.Fatalf("抹除 job %d 個", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.inactive_parties WHERE kind = 'BUYER' AND party_id = $1", b1); n != 1 {
		t.Fatalf("複本 %d 列", n)
	}
	// 冪等：重送同一個事件不重複寫複本。
	f.buyerDeleted(t, b1)
	if n := f.scalar(t, "SELECT count(*) FROM im.inactive_parties"); n != 1 {
		t.Fatalf("複本 %d 列", n)
	}
	// 在 transaction 外呼叫訂閱者是程式錯誤。
	if err := f.m.OnBuyerDeleted(context.Background(), buyer.BuyerDeleted{BuyerID: b2, OccurredAt: time.Now()}); err == nil {
		t.Fatal("transaction 外要回錯誤")
	}
}

// 抹除 job：分批、重跑結果相同；只抹該身分；保留列與 seq。
func TestEraseJobBatchesAndIsRerunnable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "seed")
	conv := f.conversation(t, b1, s1)
	// 直接寫 450 則買家訊息與 10 則店家訊息：超過兩批（每批 200）。
	f.exec(t, `INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, content, created_at)
		SELECT uuidv7(), $1, 1 + g, 'BUYER', 'TEXT', 'bulk ' || g, now() FROM generate_series(1, 450) AS g`, conv)
	f.exec(t, `INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, content, created_at)
		SELECT uuidv7(), $1, 451 + g, 'STORE', 'TEXT', 'store ' || g, now() FROM generate_series(1, 10) AS g`, conv)
	f.exec(t, "UPDATE im.conversations SET last_seq = 461, buyer_last_read_seq = 461")
	f.buyerDeleted(t, b1)
	if err := f.m.ErasePartyMessages(ctx, im.PartyBuyer, b1); err != nil {
		t.Fatal(err)
	}
	erased, kept := f.erased(t, "BUYER")
	if erased != 451 || kept != 0 {
		t.Fatalf("買家的訊息抹除 %d、仍有內容 %d", erased, kept)
	}
	if e, k := f.erased(t, "STORE"); e != 0 || k != 10 {
		t.Fatalf("店家的訊息不該被動：抹除 %d、仍有內容 %d", e, k)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 461 {
		t.Fatalf("列要保留：%d", n)
	}
	firstRun := f.scalar(t, "SELECT extract(epoch FROM max(erased_at))::bigint FROM im.messages")
	if err := f.m.ErasePartyMessages(ctx, im.PartyBuyer, b1); err != nil {
		t.Fatal(err)
	}
	if again := f.scalar(t, "SELECT extract(epoch FROM max(erased_at))::bigint FROM im.messages"); again != firstRun {
		t.Fatalf("重跑改了已抹除的列：%d -> %d", firstRun, again)
	}
	if err := f.m.ErasePartyMessages(ctx, "ROBOT", 1); err == nil {
		t.Fatal("不認得的身分要回錯誤")
	}
}

// 保底掃描：job 被丟棄後，仍在 inactive_parties 又有未抹除訊息的身分被補排；抹完之後不再補排。
func TestSweepRequeuesDroppedEraseJobs(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "hi")
	f.buyerDeleted(t, b1)
	f.jobs.reset() // 模擬 job 用完重試被丟棄
	n, err := f.m.SweepErase(ctx)
	if err != nil || n != 1 || f.jobs.count("im.erase_party") != 1 {
		t.Fatalf("補排 n=%d jobs=%d err=%v", n, f.jobs.count("im.erase_party"), err)
	}
	if err := f.m.ErasePartyMessages(ctx, im.PartyBuyer, b1); err != nil {
		t.Fatal(err)
	}
	f.jobs.reset()
	if n, err := f.m.SweepErase(ctx); err != nil || n != 0 || f.jobs.count("im.erase_party") != 0 {
		t.Fatalf("抹完不再補排：n=%d err=%v", n, err)
	}
}

// ID8：圖片訊息送出時排媒體同步；抹除圖片訊息後，退場資產有紀錄、媒體同步 job 有再排。
func TestImageMessagesSyncAndRetireOnErase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	m, err := f.m.Send(ctx, im.SendInput{Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeImage, StagedToken: "tok1"})
	if err != nil || m.Image == nil || m.Image.AssetID != media.AssetFor("tok1") {
		t.Fatalf("%+v %v", m, err)
	}
	if got := f.jobs.count("im.media_sync"); got != 1 {
		t.Fatalf("送出時排 1 個同步 job：%d", got)
	}
	if res, err := f.m.SyncMedia(ctx, media.Ref{Type: media.RefTypeChatMessage, ID: m.ID.String()}); err != nil {
		t.Fatalf("%+v %v", res, err)
	}
	if !f.media.HasRef(media.Ref{Type: media.RefTypeChatMessage, ID: m.ID.String()}, m.Image.AssetID) {
		t.Fatal("同步後 media 要有這個引用")
	}
	f.buyerDeleted(t, b1)
	if err := f.m.ErasePartyMessages(ctx, im.PartyBuyer, b1); err != nil {
		t.Fatal(err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.retired_assets WHERE ref_id = $1 AND asset_id = $2", m.ID.String(), m.Image.AssetID); n != 1 {
		t.Fatalf("退場資產紀錄 %d 筆", n)
	}
	if got := f.jobs.count("im.media_sync"); got != 2 {
		t.Fatalf("抹除要再排同步 job：%d", got)
	}
	if res, err := f.m.SyncMedia(ctx, media.Ref{Type: media.RefTypeChatMessage, ID: m.ID.String()}); err != nil {
		t.Fatalf("%+v %v", res, err)
	}
	if f.media.HasRef(media.Ref{Type: media.RefTypeChatMessage, ID: m.ID.String()}, m.Image.AssetID) {
		t.Fatal("抹除後同步要解除引用")
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages WHERE image_asset_id IS NOT NULL"); n != 0 {
		t.Fatalf("抹除後圖片欄位要清空：%d", n)
	}
}

// 刪帳競態（鎖 + 重查）：送出與訂閱者以 LockParty 串行。測試持有同一把鎖（等同訂閱者的 transaction 持鎖中），Send 要等在鎖上；
// 持鎖者提交 inactive_parties 之後 Send 被擋、不留訊息。拿掉 Send 的 LockParty 時它不會等；拿掉 transaction 內的重查時訊息會寫進去。
func TestSendWaitsForDeactivationAndIsRejected(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	tx, err := f.db.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('im.party:' || 'BUYER' || ':' || $1::bigint::text, 0))", b1); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.m.Send(ctx, im.SendInput{Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "晚到的"})
		done <- err
	}()
	f.waitForPartyLock(t, "BUYER", b1, done)
	if _, err := tx.Exec(ctx, "INSERT INTO im.inactive_parties (kind, party_id, since) VALUES ('BUYER', $1, now())", b1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("鎖釋放後要回 FailedPrecondition：%v", err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 0 {
		t.Fatalf("刪帳者不能留下訊息：%d", n)
	}
}

// 反過來：訂閱者也要持鎖。測試持有同一把鎖（等同 Send 的 transaction 持鎖中），真正的 OnBuyerDeleted 在另一個 transaction 必須等在鎖上、
// 不能先寫 inactive_parties；鎖釋放後才完成。
func TestDeactivateWaitsForPartyLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	tx, err := f.db.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('im.party:' || 'BUYER' || ':' || $1::bigint::text, 0))", b1); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- f.db.InTx(ctx, func(ctx context.Context) error {
			return f.m.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: b1, AccountID: 1000 + b1, OccurredAt: f.clock()})
		})
	}()
	f.waitForPartyLock(t, "BUYER", b1, done)
	if n := f.scalar(t, "SELECT count(*) FROM im.inactive_parties"); n != 0 {
		t.Fatalf("鎖釋放前不能已寫入 inactive_parties：%d", n)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.inactive_parties"); n != 1 {
		t.Fatalf("鎖釋放後 Deactivate 完成：%d", n)
	}
}

// waitForPartyLock 等到有 session 等在這個身分的 advisory lock 上（只看這個 key，不受平行測試影響）：
// bigint key 的高 32 位在 classid、低 32 位在 objid，objsubid = 1。等的過程中對方先回傳就是失敗（沒有等在鎖上）。
func (f *fixture) waitForPartyLock(t *testing.T, kind string, id int64, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		n := f.scalar(t, `
SELECT count(*) FROM pg_locks l,
  LATERAL (SELECT hashtextextended('im.party:' || $1::text || ':' || $2::bigint::text, 0) AS k) h
WHERE l.locktype = 'advisory' AND NOT l.granted AND l.objsubid = 1
  AND l.classid::bigint = (h.k >> 32) & 4294967295 AND l.objid::bigint = h.k & 4294967295`, kind, id)
		if n > 0 {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("應該等在 party 鎖上，卻已回傳：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("沒有等在 party 鎖上")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
