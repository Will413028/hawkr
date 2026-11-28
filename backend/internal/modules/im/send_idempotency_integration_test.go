//go:build integration

package im_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/live"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func (f *fixture) unread(t *testing.T, p im.Party) int64 {
	t.Helper()
	page, err := f.m.ListConversations(context.Background(), p, nil, 10)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("%+v %v", page, err)
	}
	return page.Items[0].Unread
}

func (f *fixture) sendWith(t *testing.T, in im.SendInput) (im.Message, error) {
	t.Helper()
	return f.m.Send(context.Background(), in)
}

// 同一個 (對話, 寄件方, request_id) 重送回第一次寫入的那則：同一個 ID 與 seq，不配新 seq、不增加未讀、不再發事件、不再排 job、不再 NOTIFY。
func TestResendReturnsTheFirstMessageWithoutSideEffects(t *testing.T) {
	t.Parallel()
	f := setup(t)
	raw := f.listenRaw(t)
	buyer := f.buyerParty(b1)
	in := im.SendInput{RequestID: uuid.NewV4(), Sender: buyer, CounterpartID: f.s1, Type: im.TypeText, Text: "  你好  "}
	first, err := f.sendWith(t, in)
	if err != nil {
		t.Fatal(err)
	}
	conv := first.ConversationID
	unread := f.unread(t, f.storeParty(t, f.s1))
	events := len(f.sent)
	for _, again := range []im.SendInput{
		in,
		{RequestID: in.RequestID, Sender: buyer, ConversationID: conv, Type: im.TypeText, Text: "你好"},
	} {
		got, err := f.sendWith(t, again)
		if err != nil || got.ID != first.ID || got.Seq != first.Seq || got.ConversationID != conv {
			t.Fatalf("重送要回第一則 %+v，實際 %+v %v", first, got, err)
		}
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 1 {
		t.Fatalf("訊息數 = %d", n)
	}
	if n := f.scalar(t, "SELECT last_seq FROM im.conversations WHERE id = $1", conv); n != 1 {
		t.Fatalf("last_seq = %d", n)
	}
	if got := f.unread(t, f.storeParty(t, f.s1)); got != unread {
		t.Fatalf("未讀數 %d → %d", unread, got)
	}
	if len(f.sent) != events {
		t.Fatalf("重送不該再發 MessageSent：%d → %d", events, len(f.sent))
	}
	f.say(t, buyer, f.s1, "sentinel")
	want := []live.Payload{live.Message(conv, b1, f.s1, 1), live.Message(conv, b1, f.s1, 2)}
	if got := raw.waitFor(t, f, 2); !slices.Equal(got, want) {
		t.Fatalf("重送不該 NOTIFY：%+v，預期 %+v", got, want)
	}
}

// 已成功送出的訊息，重送不因之後才發生的封鎖而失敗；新的 request_id 仍被擋。
func TestResendSucceedsAfterALaterBlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	in := im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: f.s1, Type: im.TypeText, Text: "先送出"}
	first, err := f.sendWith(t, in)
	if err != nil {
		t.Fatal(err)
	}
	f.buyers.blocking = true
	got, err := f.sendWith(t, in)
	if err != nil || got.ID != first.ID {
		t.Fatalf("封鎖後重送：%+v %v", got, err)
	}
	fresh := in
	fresh.RequestID = uuid.NewV4()
	if _, err := f.sendWith(t, fresh); kindOf(err) != apperr.KindFailedPrecondition || im.ReasonOf(err) != im.ReasonChatBlocked {
		t.Fatalf("新的 request_id 仍應被封鎖擋下：%v", err)
	}
}

// 同一個 request_id 但內容不同回 InvalidInput 指向 RequestID；圖片只比 Type。
func TestResendWithDifferentContentIsRejected(t *testing.T) {
	t.Parallel()
	f := setup(t)
	buyer := f.buyerParty(b1)
	in := im.SendInput{RequestID: uuid.NewV4(), Sender: buyer, CounterpartID: f.s1, Type: im.TypeText, Text: "原文"}
	if _, err := f.sendWith(t, in); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*im.SendInput){
		"文字不同": func(i *im.SendInput) { i.Text = "改過" },
		"類型不同": func(i *im.SendInput) { i.Type, i.Text, i.StagedToken = im.TypeImage, "", "tok-reuse" },
	} {
		again := in
		mutate(&again)
		_, err := f.sendWith(t, again)
		var ae *apperr.Error
		if kindOf(err) != apperr.KindInvalidInput || !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "RequestID" {
			t.Fatalf("%s：err = %v", name, err)
		}
	}
	tokenA, _ := f.stage(t, 1000+b1)
	img := im.SendInput{RequestID: uuid.NewV4(), Sender: buyer, CounterpartID: f.s1, Type: im.TypeImage, StagedToken: tokenA}
	first, err := f.sendWith(t, img)
	if err != nil {
		t.Fatal(err)
	}
	img.StagedToken = "tok-gone"
	if got, err := f.sendWith(t, img); err != nil || got.ID != first.ID {
		t.Fatalf("圖片重送（staged token 不比）：%+v %v", got, err)
	}
}

// 鍵含寄件方：買家與店家用同一個 request_id 各送一則都成功，各自是獨立的訊息。
func TestSameRequestIDFromBothSidesCreatesTwoMessages(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id := uuid.NewV4()
	b, err := f.sendWith(t, im.SendInput{RequestID: id, Sender: f.buyerParty(b1), CounterpartID: f.s1, Type: im.TypeText, Text: "買家"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.sendWith(t, im.SendInput{RequestID: id, Sender: f.storeParty(t, f.s1), CounterpartID: b1, Type: im.TypeText, Text: "店家"})
	if err != nil || s.ID == b.ID || s.Seq != 2 {
		t.Fatalf("店家 %+v %v", s, err)
	}
}

// 並行的同 request_id 只留一則，輸家回傳同一則；seq 沒有空洞（輸家的 transaction 整個回滾）。
func TestConcurrentSameRequestIDWritesOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	const requests, dup = 4, 3
	ids := make([]uuid.UUID, requests)
	for i := range ids {
		ids[i] = uuid.NewV4()
	}
	type result struct {
		rid int
		m   im.Message
		err error
	}
	out := make(chan result, requests*dup)
	var wg sync.WaitGroup
	for i := range requests {
		for range dup {
			wg.Add(1)
			go func() {
				defer wg.Done()
				m, err := f.m.Send(context.Background(), im.SendInput{RequestID: ids[i], Sender: f.buyerParty(b1), CounterpartID: f.s1, Type: im.TypeText, Text: "並行" + string(rune('A'+i))})
				out <- result{i, m, err}
			}()
		}
	}
	wg.Wait()
	close(out)
	first := map[int]uuid.UUID{}
	for r := range out {
		if r.err != nil {
			t.Fatalf("request %d：%v", r.rid, r.err)
		}
		if id, ok := first[r.rid]; ok && id != r.m.ID {
			t.Fatalf("request %d 回了不同的訊息", r.rid)
		}
		first[r.rid] = r.m.ID
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != requests {
		t.Fatalf("訊息數 = %d，預期 %d", n, requests)
	}
	if n := f.scalar(t, "SELECT last_seq FROM im.conversations"); n != requests {
		t.Fatalf("last_seq = %d", n)
	}
	if n := f.scalar(t, "SELECT count(DISTINCT seq) FILTER (WHERE seq BETWEEN 1 AND $1) FROM im.messages", int64(requests)); n != requests {
		t.Fatalf("seq 不是 1..%d 連續：%d", requests, n)
	}
}

// ConversationParties、MessageParties：命中、不存在 NotFound、隱藏與抹除的訊息照樣回歸屬。
func TestPartiesReadsOwnershipOnly(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	m := f.say(t, f.buyerParty(b1), f.s1, "歸屬")
	want := im.ConversationParties{ConversationID: m.ConversationID, BuyerID: b1, StoreID: f.s1}
	if got, err := f.m.ConversationParties(ctx, m.ConversationID); err != nil || got != want {
		t.Fatalf("ConversationParties = %+v %v，預期 %+v", got, err, want)
	}
	if got, err := f.m.MessageParties(ctx, m.ID); err != nil || got != want {
		t.Fatalf("MessageParties = %+v %v", got, err)
	}
	if _, err := f.m.ConversationParties(ctx, uuid.New()); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的對話：%v", err)
	}
	if _, err := f.m.MessageParties(ctx, uuid.New()); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的訊息：%v", err)
	}
	if _, err := f.m.Hide(ctx, im.HideInput{MessageID: m.ID, AdminID: 1, Category: string(policy.CommunityAssessable()[0])}); err != nil {
		t.Fatal(err)
	}
	if got, err := f.m.MessageParties(ctx, m.ID); err != nil || got != want {
		t.Fatalf("隱藏後 MessageParties = %+v %v", got, err)
	}
	if err := f.m.ErasePartyMessages(ctx, im.PartyBuyer, b1); err != nil {
		t.Fatal(err)
	}
	if got, err := f.m.MessageParties(ctx, m.ID); err != nil || got != want {
		t.Fatalf("抹除後 MessageParties = %+v %v", got, err)
	}
}

// 被隱藏或被抹除的訊息，重送也回墓碑（不帶內容、Hidden 或 Erased 標出），仍是同一個 ID 與 seq。
func TestResendOfAHiddenOrErasedMessageReturnsATombstone(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	buyer := f.buyerParty(b1)
	in := im.SendInput{RequestID: uuid.NewV4(), Sender: buyer, CounterpartID: f.s1, Type: im.TypeText, Text: "之後被隱藏"}
	first, err := f.sendWith(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Hide(ctx, im.HideInput{MessageID: first.ID, AdminID: 1, Category: string(policy.CommunityAssessable()[0])}); err != nil {
		t.Fatal(err)
	}
	got, err := f.sendWith(t, in)
	if err != nil || got.ID != first.ID || got.Seq != first.Seq || got.Text != nil || !got.Hidden {
		t.Fatalf("隱藏後重送要回同一則的墓碑：%+v %v", got, err)
	}
	if err := f.m.ErasePartyMessages(ctx, im.PartyBuyer, b1); err != nil {
		t.Fatal(err)
	}
	got, err = f.sendWith(t, in)
	if err != nil || got.ID != first.ID || got.Seq != first.Seq || got.Text != nil || !got.Erased {
		t.Fatalf("抹除後重送要回同一則的墓碑：%+v %v", got, err)
	}
}

// 並行的同一個 request_id 的圖片訊息（同一個 staged token）：入口的 advisory lock 讓輸家等贏家提交後走重送判定回同一則，
// 不會走到 Confirm；只有一個資產被確認、只有一則訊息、seq 沒有空洞（B1-D26）。
func TestConcurrentSameRequestIDImageSendsConfirmOneAsset(t *testing.T) {
	t.Parallel()
	f := setup(t)
	token, asset := f.stage(t, 1000+b1)
	rid := uuid.NewV4()
	const dup = 4
	type result struct {
		m   im.Message
		err error
	}
	out := make(chan result, dup)
	var wg sync.WaitGroup
	for range dup {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, err := f.m.Send(context.Background(), im.SendInput{RequestID: rid, Sender: f.buyerParty(b1), CounterpartID: f.s1, Type: im.TypeImage, StagedToken: token})
			out <- result{m, err}
		}()
	}
	wg.Wait()
	close(out)
	var first uuid.UUID
	for r := range out {
		if r.err != nil {
			t.Fatalf("並行的同 request_id 圖片送出：%v", r.err)
		}
		if r.m.Image == nil || r.m.Image.AssetID != asset {
			t.Fatalf("回傳的訊息要帶被確認的資產：%+v", r.m)
		}
		if first != (uuid.UUID{}) && first != r.m.ID {
			t.Fatalf("並行的同 request_id 回了不同的訊息：%s、%s", first, r.m.ID)
		}
		first = r.m.ID
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 1 {
		t.Fatalf("訊息數 = %d，預期 1", n)
	}
	if n := f.scalar(t, "SELECT last_seq FROM im.conversations"); n != 1 {
		t.Fatalf("last_seq = %d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM media.upload_claims"); n != 1 {
		t.Fatalf("確認的認領列數 = %d，預期 1", n)
	}
	if got := f.assetState(t, asset); got != "ACTIVE" {
		t.Fatalf("資產 state=%s，預期 ACTIVE", got)
	}
}

// 同一個 request_id 的第二個 Send 在第一個還沒提交時，要等在入口的 advisory lock 上，而不是先去 Confirm：
// 第一個 Send 停在持有守門鎖之後（已持有 request_id 的鎖、還沒 Confirm），第二個必須等在鎖上；放行後兩個都回同一則，資產只被確認一次（B1-D26）。
// 拿掉 `LockSendRequest` 時第二個 Send 不會等（立刻 Confirm 並提交），這個測試失敗。
func TestSecondSendOfTheSameRequestWaitsOnTheEntryLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	token, asset := f.stage(t, 1000+b1)
	in := im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: f.s1, Type: im.TypeImage, StagedToken: token}
	held, rel := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(rel) }) }
	t.Cleanup(release)
	var first atomic.Bool
	f.stores.afterLock = func() {
		if first.CompareAndSwap(false, true) {
			close(held)
			<-rel
		}
	}
	type result struct {
		m   im.Message
		err error
	}
	a, b := make(chan result, 1), make(chan result, 1)
	go func() {
		m, err := f.m.Send(context.Background(), in)
		a <- result{m, err}
	}()
	select {
	case <-held:
	case r := <-a:
		t.Fatalf("第一個 Send 沒有停在持鎖中：%+v", r)
	case <-time.After(10 * time.Second):
		t.Fatal("第一個 Send 沒有走到守門鎖")
	}
	go func() {
		m, err := f.m.Send(context.Background(), in)
		b <- result{m, err}
	}()
	waiting := make(chan error, 1)
	go func() {
		r := <-b
		b <- r
		waiting <- r.err
	}()
	f.waitLockWaiter(t, waiting)
	release()
	var ra, rb result
	for got := 0; got < 2; got++ {
		select {
		case ra = <-a:
		case rb = <-b:
		case <-time.After(30 * time.Second):
			t.Fatal("放行後兩個 Send 沒有在時限內回傳")
		}
	}
	if ra.err != nil || rb.err != nil {
		t.Fatalf("兩個 Send 都該成功：%v、%v", ra.err, rb.err)
	}
	if ra.m.ID != rb.m.ID || ra.m.Image == nil || ra.m.Image.AssetID != asset {
		t.Fatalf("要回同一則、帶被確認的資產：%+v、%+v", ra.m, rb.m)
	}
	if n := f.scalar(t, "SELECT count(*) FROM im.messages"); n != 1 {
		t.Fatalf("訊息數 = %d，預期 1", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM media.upload_claims"); n != 1 {
		t.Fatalf("確認的認領列數 = %d，預期 1", n)
	}
}
