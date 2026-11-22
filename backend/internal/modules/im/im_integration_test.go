//go:build integration

package im_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

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
			m, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: from, CounterpartID: counterpart, Type: im.TypeText, Text: fmt.Sprintf("m%d", i)})
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
		in := im.SendInput{RequestID: uuid.NewV4(), Sender: p, ConversationID: conv, Type: im.TypeText, Text: "intrude"}
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
		_, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "hi"})
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
		_, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "hi"})
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
	f.storeRetired(t, s1, 2000+s1)
	if err := f.m.ErasePartyMessages(ctx, im.PartyStore, s1); err != nil {
		t.Fatal(err)
	}
	if pv := preview(); !pv.Erased || pv.Text != nil {
		t.Fatalf("抹除後的預覽：%+v", pv)
	}
}

// buyerDeleted 關閉買家（buyer 的 CloseForDeletion，事實的唯一來源）；buyer 在同 transaction 發 BuyerDeleted，im 的訂閱者排抹除 job。
func (f *fixture) buyerDeleted(t *testing.T, buyerID int64) {
	t.Helper()
	ctx := context.Background()
	if err := f.buyers.owner.CloseForDeletion(ctx, buyerID); err != nil {
		t.Fatal(err)
	}
}

// storeRetired 讓店家退場（seller 的 retired_at 是事實的唯一來源，最小的列）並在同 transaction 發 StoreRetired。
func (f *fixture) storeRetired(t *testing.T, storeID, accountID int64) {
	t.Helper()
	ctx := context.Background()
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		if err := closeStoreRows(ctx, f.db, storeID); err != nil {
			return err
		}
		return events.Publish(ctx, f.bus, seller.StoreRetired{EventID: uuid.New(), OccurredAt: f.clock(), StoreID: storeID, AccountID: accountID})
	})
	if err != nil {
		t.Fatal(err)
	}
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

	f.storeRetired(t, s1, 5000)
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

// 買家關閉、店家退場之後送出被擋：寄件人已停用是 FailedPrecondition，寄給已停用者是 RECEIVER_DELETED；兩者都不留列。
// 讀取側也要對：relay 的 receiver_inactive 讀擁有者的 pub 函式。
func TestPartiesCannotSendOrReceiveAfterClosedOrRetired(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "hi")
	f.say(t, f.buyerParty(b2), s2, "to the store that will retire")
	var relayID uuid.UUID
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT m.id FROM im.messages m JOIN im.conversations c ON c.id = m.conversation_id WHERE c.buyer_id = $1 AND m.sender_kind = 'BUYER'", b2).Scan(&relayID); err != nil {
		t.Fatal(err)
	}
	if r, err := f.m.MessageForRelay(ctx, relayID); err != nil || r.ReceiverInactive {
		t.Fatalf("退場之前收件人營運中：%+v %v", r, err)
	}

	f.buyerDeleted(t, b1)
	if _, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "again"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已關閉的寄件人：%v", err)
	}
	_, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.storeParty(t, s1), CounterpartID: b1, Type: im.TypeText, Text: "anyone there"})
	if kindOf(err) != apperr.KindFailedPrecondition || im.ReasonOf(err) != im.ReasonReceiverDeleted {
		t.Fatalf("寄給已關閉者：%v", err)
	}

	f.storeRetired(t, s2, 2000+s2)
	_, err = f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b2), CounterpartID: s2, Type: im.TypeText, Text: "to retired"})
	if kindOf(err) != apperr.KindFailedPrecondition || im.ReasonOf(err) != im.ReasonReceiverDeleted {
		t.Fatalf("寄給已退場的店：%v", err)
	}
	_, err = f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.storeParty(t, s2), CounterpartID: b2, Type: im.TypeText, Text: "from retired"})
	if kindOf(err) != apperr.KindFailedPrecondition || im.ReasonOf(err) == im.ReasonReceiverDeleted {
		t.Fatalf("已退場的寄件人要是寄件人已停用，不是 RECEIVER_DELETED：%v", err)
	}
	if got := f.scalar(t, "SELECT count(*) FROM im.messages"); got != 2 {
		t.Fatalf("被擋的送出留下了列：%d", got)
	}
	if r, err := f.m.MessageForRelay(ctx, relayID); err != nil || !r.ReceiverInactive {
		t.Fatalf("退場之後 relay 的 receiver_inactive 要為真：%+v %v", r, err)
	}
	r, err := f.m.MessageForRelay(ctx, mustMessageID(t, f, b1))
	if err != nil || r.ReceiverInactive {
		t.Fatalf("買家關閉時，買家寄出的訊息收件人是店家（營運中）：%+v %v", r, err)
	}
}

func mustMessageID(t *testing.T, f *fixture, buyerID int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT m.id FROM im.messages m JOIN im.conversations c ON c.id = m.conversation_id WHERE c.buyer_id = $1 AND m.sender_kind = 'BUYER'", buyerID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// 刪帳申請中的買家仍可聊天（計畫「不能動」）：BeginDeletion 之後買家與店家雙向仍可送、訊息不被抹除、沒有抹除 job；CloseForDeletion 之後才擋、才有 job。
func TestPendingDeletionBuyerCanStillSend(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "before")
	if err := f.buyers.owner.BeginDeletion(ctx, b1); err != nil {
		t.Fatal(err)
	}
	f.say(t, f.buyerParty(b1), s1, "buyer while pending")
	f.say(t, f.storeParty(t, s1), b1, "store to pending buyer")
	if n := f.jobs.count("im.erase_party"); n != 0 {
		t.Fatalf("刪除申請不排抹除 job：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages WHERE erased_at IS NOT NULL"); n != 0 {
		t.Fatalf("刪除申請不抹訊息：%d", n)
	}
	f.buyerDeleted(t, b1)
	if n := f.jobs.count("im.erase_party"); n != 1 {
		t.Fatalf("關閉之後才排抹除 job：%d", n)
	}
	if _, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "after"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("關閉之後被擋：%v", err)
	}
}

// Send 不得在寫入 transaction 內呼叫 legacy 的 Orders（列鎖不跨網路呼叫）：封鎖時 guard 會查進行中的訂單，替身記下呼叫時 ctx 是否帶 transaction。
func TestSendCallsOrdersOutsideTheWriteTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.stores.block = true
	f.orders.open = true
	f.say(t, f.buyerParty(b1), s1, "blocked but open order")
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	if f.orders.calls == 0 {
		t.Fatal("封鎖時 guard 要查訂單")
	}
	if f.orders.inTx {
		t.Fatal("Orders 在寫入 transaction 內被呼叫")
	}
}

// 刪帳的訂閱者排抹除 job（拿掉 Enqueue 時這個測試失敗）；不存任何複本。
func TestSubscriberEnqueuesTheEraseJob(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.say(t, f.buyerParty(b1), s1, "hi")
	f.buyerDeleted(t, b1)
	if n := f.jobs.count("im.erase_party"); n != 1 {
		t.Fatalf("抹除 job %d 個", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM pg_tables WHERE schemaname = 'im' AND tablename = 'inactive_parties'"); n != 0 {
		t.Fatalf("複本表已刪除：%d", n)
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
	f.exec(t, `INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, content, created_at, request_id)
		SELECT uuidv7(), $1, 1 + g, 'BUYER', 'TEXT', 'bulk ' || g, now(), gen_random_uuid() FROM generate_series(1, 450) AS g`, conv)
	f.exec(t, `INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, content, created_at, request_id)
		SELECT uuidv7(), $1, 451 + g, 'STORE', 'TEXT', 'store ' || g, now(), gen_random_uuid() FROM generate_series(1, 10) AS g`, conv)
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

// 保底掃描：job 被丟棄後，已關閉或已退場又有未抹除訊息的身分被補排（來源是擁有者的 pub 函式）；抹完之後不再補排。
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

// 保底掃描分頁走完全部：超過一頁（100）時最後面的身分也會被補排，前面的身分一直失敗不會讓它餓死。
func TestSweepReachesPartiesBeyondOneBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	const parties = 230
	last := int64(1000 + parties - 1)
	f.exec(t, `INSERT INTO buyer.buyers (buyer_id, account_id, status, display_name, created_at, updated_at, deleted_at)
		OVERRIDING SYSTEM VALUE SELECT g, 1000000 + g, 'CLOSED', 'b', now(), now(), now() FROM generate_series(1000, $1::bigint) AS g`, last)
	f.exec(t, `INSERT INTO im.conversations (id, buyer_id, store_id, last_seq, last_message_at, created_at)
		SELECT gen_random_uuid(), g, 1, 1, now(), now() FROM generate_series(1000, $1::bigint) AS g`, last)
	f.exec(t, `INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, content, created_at, request_id)
		SELECT gen_random_uuid(), c.id, 1, 'BUYER', 'TEXT', 'x', now(), gen_random_uuid() FROM im.conversations AS c`)
	seen := func() map[int64]int {
		f.jobs.mu.Lock()
		defer f.jobs.mu.Unlock()
		out := map[int64]int{}
		for _, a := range f.jobs.args {
			var v struct {
				PartyID int64 `json:"party_id"`
			}
			b, _ := json.Marshal(a)
			if err := json.Unmarshal(b, &v); err != nil {
				t.Fatal(err)
			}
			out[v.PartyID]++
		}
		return out
	}
	n, err := f.m.SweepErase(ctx)
	if err != nil || n != parties {
		t.Fatalf("掃到 %d 個身分 err=%v，預期 %d", n, err, parties)
	}
	got := seen()
	if len(got) != parties || got[last] != 1 || got[1000] != 1 {
		t.Fatalf("補排了 %d 個不同的身分，最後一個 %d 次、第一個 %d 次", len(got), got[last], got[1000])
	}
}

// ID8：圖片訊息送出時排媒體同步；抹除圖片訊息後，退場資產有紀錄、媒體同步 job 有再排。
func TestImageMessagesSyncAndRetireOnErase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	m, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeImage, StagedToken: "tok1"})
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

// holdSend 啟動一個 Send（買家 b1 寄給店家 s1），讓它的 transaction 停在已持有買家與店家守門鎖之後（stores.afterLock）；回傳放行函式與結果 channel。
func (f *fixture) holdSend(t *testing.T) (release func(), done <-chan error) {
	t.Helper()
	held, rel := make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	f.stores.afterLock = func() {
		if first.CompareAndSwap(false, true) {
			close(held)
			<-rel
		}
	}
	res := make(chan error, 1)
	go func() {
		_, err := f.m.Send(context.Background(), im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "持鎖中"})
		res <- err
	}()
	select {
	case <-held:
	case err := <-res:
		t.Fatalf("Send 沒有停在持鎖中：%v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Send 沒有走到守門鎖")
	}
	return func() { close(rel) }, res
}

// waitLockWaiter 等到有連線等在鎖上（每個整合測試有自己的資料庫）；等的過程中 done 先回傳就是失敗（沒有等在鎖上）。
func (f *fixture) waitLockWaiter(t *testing.T, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for f.scalar(t, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'") == 0 {
		select {
		case err := <-done:
			t.Fatalf("應該等在守門鎖上，卻已回傳：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("沒有等在守門鎖上")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// 刪帳競態：Send 的 transaction 持買家列的 FOR KEY SHARE 時，買家關閉（FOR UPDATE）要等；Send 先提交，之後的送出被擋。
// 拿掉 Send 的 LockNotClosed 時關閉不會等，這個測試失敗。
func TestSendWaitsForClosingAndIsRejected(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	release, sent := f.holdSend(t)
	closed := make(chan error, 1)
	go func() { closed <- f.buyers.owner.CloseForDeletion(ctx, b1) }()
	f.waitLockWaiter(t, closed)
	if st := f.scalar(t, "SELECT count(*) FROM buyer.buyers WHERE buyer_id = $1 AND status = 'ACTIVE'", b1); st != 1 {
		t.Fatal("守門鎖釋放前買家不能已關閉")
	}
	release()
	if err := <-sent; err != nil {
		t.Fatalf("先取得守門鎖的 Send 要成功：%v", err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 1 {
		t.Fatalf("關閉之前提交的訊息要在：%d", n)
	}
	if _, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "晚到的"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("關閉之後要被擋：%v", err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 1 {
		t.Fatalf("被擋的送出不能留下訊息：%d", n)
	}
}

// 店家退場的批次對被守門鎖住的店 FOR UPDATE SKIP LOCKED：Send 持守門鎖時跳過它、不等；Send 提交之後下一輪才選得到。
func TestRetireSkipsAStoreHeldByASend(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	release, sent := f.holdSend(t)
	pick := func() int {
		rows, err := f.db.Pool().Query(ctx, "SELECT store_id FROM seller.stores WHERE store_id = $1 FOR UPDATE SKIP LOCKED", s1)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			n++
		}
		return n
	}
	if n := pick(); n != 0 {
		t.Fatalf("持守門鎖的店要被跳過：%d", n)
	}
	release()
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	if n := pick(); n != 1 {
		t.Fatalf("Send 提交之後下一輪要選得到：%d", n)
	}
}

// Restore 帶來源參照：沒有未還原的隱藏與隱藏是別案造成的是兩種不同的 FailedPrecondition，後者不動隱藏。
// 殺死的變異：Restore 忽略 SourceRef；兩種情況回同一句。
func TestRestoreSourceMismatchIsDistinctFromNotHidden(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.say(t, f.buyerParty(b1), s1, "first")
	last := f.say(t, f.storeParty(t, s1), b1, "內容")
	if _, err := f.m.Restore(ctx, im.RestoreInput{MessageID: last.ID, AdminID: 1, Reason: "誤判", SourceRef: "case-1"}); kindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "沒有被隱藏") {
		t.Fatalf("沒有隱藏：%v", err)
	}
	if _, err := f.m.Hide(ctx, im.HideInput{MessageID: last.ID, AdminID: 1, Category: string(policy.CommunityAssessable()[0]), SourceRef: "case-1"}); err != nil {
		t.Fatal(err)
	}
	_, err := f.m.Restore(ctx, im.RestoreInput{MessageID: last.ID, AdminID: 1, Reason: "誤判", SourceRef: "case-2"})
	if kindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "不是由這個案件造成") {
		t.Fatalf("來源不符：%v", err)
	}
	msgs, err := f.m.ListMessages(ctx, f.conversation(t, b1, s1), f.buyerParty(b1), 0, 10)
	stillHidden := false
	for _, m := range msgs.Items {
		if m.Seq == last.Seq {
			stillHidden = m.Hidden
		}
	}
	if err != nil || !stillHidden {
		t.Fatalf("來源不符時隱藏仍在：%+v %v", msgs, err)
	}
	if _, err := f.m.Restore(ctx, im.RestoreInput{MessageID: last.ID, AdminID: 1, Reason: "誤判", SourceRef: "case-1"}); err != nil {
		t.Fatalf("來源相符可還原：%v", err)
	}
}

// 檢舉證據的前後文：已隱藏的訊息（文字或圖片）是墓碑，只帶旗標，不帶 Text 與 AssetID；未隱藏的照常帶內容。
func TestMessageForReportReturnsTombstonesForHiddenContext(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	buyerSide := f.buyerParty(b1)
	visibleText := f.say(t, buyerSide, s1, "可見的文字")
	hiddenText := f.say(t, buyerSide, s1, "被隱藏的文字")
	hiddenImage, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: buyerSide, CounterpartID: s1, Type: im.TypeImage, StagedToken: "hid"})
	if err != nil {
		t.Fatal(err)
	}
	visibleImage, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: buyerSide, CounterpartID: s1, Type: im.TypeImage, StagedToken: "vis"})
	if err != nil {
		t.Fatal(err)
	}
	target := f.say(t, buyerSide, s1, "被檢舉的訊息")
	for _, id := range []uuid.UUID{hiddenText.ID, hiddenImage.ID} {
		if _, err := f.m.Hide(ctx, f.hideInput(id)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.m.MessageForReport(ctx, target.ID, f.storeParty(t, s1), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Context) != 4 {
		t.Fatalf("前後文應有 4 則：%+v", got.Context)
	}
	byID := map[uuid.UUID]im.ReportedChatMessage{}
	for _, c := range got.Context {
		byID[c.ID] = c
	}
	for _, id := range []uuid.UUID{hiddenText.ID, hiddenImage.ID} {
		c := byID[id]
		if !c.Hidden || c.Erased || c.Text != "" || c.AssetID != "" {
			t.Fatalf("已隱藏的前後文要是墓碑（只帶旗標）：%+v", c)
		}
	}
	if c := byID[visibleText.ID]; c.Hidden || c.Erased || c.Text != "可見的文字" || c.AssetID != "" {
		t.Fatalf("未隱藏的文字要帶內容：%+v", c)
	}
	if c := byID[visibleImage.ID]; c.Hidden || c.Erased || c.Text != "" || c.AssetID != media.AssetFor("vis") {
		t.Fatalf("未隱藏的圖片要帶 AssetID：%+v", c)
	}
}
