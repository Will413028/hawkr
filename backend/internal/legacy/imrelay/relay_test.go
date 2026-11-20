package imrelay

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var (
	messageID      = uuid.MustParse("01930000-0000-7000-8000-000000000001")
	conversationID = uuid.MustParse("01930000-0000-7000-8000-000000000002")
	eventID        = uuid.MustParse("01930000-0000-7000-8000-000000000003")
	occurredAt     = time.Date(2026, 11, 20, 3, 4, 5, 0, time.UTC)
)

type recordingJobs struct{ args []river.JobArgs }

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	q.args = append(q.args, args)
	return nil
}

func (q *recordingJobs) published() []rabbitmq.PublishArgs {
	var out []rabbitmq.PublishArgs
	for _, a := range q.args {
		if p, ok := a.(rabbitmq.PublishArgs); ok {
			out = append(out, p)
		}
	}
	return out
}

type fakeMessages struct {
	msg im.RelayMessage
	err error
}

func (f *fakeMessages) MessageForRelay(context.Context, uuid.UUID) (im.RelayMessage, error) {
	return f.msg, f.err
}

type fakeBuyers struct {
	display    []buyer.Display
	err        error
	byAccount  []int64
	accountErr error
}

func (f *fakeBuyers) BuyerIDsByAccount(context.Context, int64) ([]int64, error) {
	return f.byAccount, f.accountErr
}

func (f *fakeBuyers) BatchDisplay(context.Context, []int64) ([]buyer.Display, error) {
	return f.display, f.err
}

type fakeStores struct {
	store seller.Store
	err   error
}

func (f *fakeStores) Store(context.Context, int64) (seller.Store, error) { return f.store, f.err }

type fixture struct {
	worker   *Worker
	jobs     *recordingJobs
	messages *fakeMessages
	buyers   *fakeBuyers
	stores   *fakeStores
}

func ptr[T any](v T) *T { return &v }

// newFixture 是買家 11 寄文字給店家 22（帳號 2001）的訊息。
func newFixture() *fixture {
	f := &fixture{
		jobs: &recordingJobs{},
		messages: &fakeMessages{msg: im.RelayMessage{
			Message: im.Message{
				ID: messageID, ConversationID: conversationID, Seq: 3, SenderKind: im.PartyBuyer, Type: im.TypeText,
				Text: ptr("你好"), CreatedAt: occurredAt,
			},
			BuyerID: 11, StoreID: 22,
		}},
		buyers: &fakeBuyers{display: []buyer.Display{{BuyerID: 11, DisplayName: ptr("小明"), Avatar: ptr("avatar/11.jpg")}}},
		stores: &fakeStores{store: seller.Store{StoreID: 22, AccountID: 2001, LogoKey: ptr("logo/22.jpg")}},
	}
	f.stores.store.StoreName = "好茶舖"
	f.worker = NewWorker(f.messages, f.buyers, f.stores, f.jobs, nil)
	return f
}

func sentArgs() MessageSentArgs {
	return MessageSentArgs{EventID: eventID.String(), Occurred: occurredAt, MessageID: messageID.String()}
}

func body(t *testing.T, p rabbitmq.PublishArgs) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(p.Body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuyerMessageIsRelayedToTheSellerAccount(t *testing.T) {
	f := newFixture()
	if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
		t.Fatal(err)
	}
	pubs := f.jobs.published()
	if len(pubs) != 1 {
		t.Fatalf("送出 %d 則", len(pubs))
	}
	p := pubs[0]
	if p.Exchange != "hawkr.im" || p.RoutingKey != "chat.message.sent" || p.EventID != eventID.String() {
		t.Fatalf("publish = %+v", p)
	}
	want := map[string]any{
		"message_id": messageID.String(), "conversation_id": conversationID.String(), "sender_id": "11", "sender_name": "小明",
		"sender_avatar": "avatar/11.jpg", "receiver_id": "2001", "receiver_type": "seller", "store_id": "22",
		"content_preview": "你好", "sent_at": "2026-11-20T03:04:05Z",
	}
	got := body(t, p)
	if len(got) != len(want) {
		t.Fatalf("payload = %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %#v, want %#v", k, got[k], v)
		}
	}
}

func TestStoreMessageIsRelayedToTheBuyerId(t *testing.T) {
	f := newFixture()
	f.messages.msg.SenderKind = im.PartyStore
	if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
		t.Fatal(err)
	}
	got := body(t, f.jobs.published()[0])
	for k, v := range map[string]any{
		"sender_id": "22", "sender_name": "好茶舖", "sender_avatar": "logo/22.jpg", "receiver_id": "11", "receiver_type": "buyer", "store_id": "22",
	} {
		if got[k] != v {
			t.Errorf("%s = %#v, want %#v", k, got[k], v)
		}
	}
}

func TestPreviewSentences(t *testing.T) {
	long := strings.Repeat("字", 101)
	exact := strings.Repeat("字", 100)
	cases := []struct {
		name string
		mut  func(*im.RelayMessage)
		want string
	}{
		{"image", func(m *im.RelayMessage) { m.Type, m.Text = im.TypeImage, nil }, "傳送了一張圖片"},
		{"100 runes untouched", func(m *im.RelayMessage) { m.Text = &exact }, exact},
		{"101 runes truncated", func(m *im.RelayMessage) { m.Text = &long }, exact + "..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			tc.mut(&f.messages.msg)
			if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
				t.Fatal(err)
			}
			if got := body(t, f.jobs.published()[0])["content_preview"]; got != tc.want {
				t.Fatalf("content_preview = %#v", got)
			}
		})
	}
}

// 終態：訊息在 job 執行前已被抹除、隱藏或不存在，不送；推播不能帶出已抹除的內容。
func TestTerminalMessagesAreNotRelayed(t *testing.T) {
	cases := map[string]func(*fixture){
		"erased":                            func(f *fixture) { f.messages.msg.Erased = true },
		"hidden":                            func(f *fixture) { f.messages.msg.Hidden = true },
		"gone":                              func(f *fixture) { f.messages.err = apperr.NotFound("gone") },
		"store gone for buyer-sent message": func(f *fixture) { f.stores.err = apperr.NotFound("gone") },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			mut(f)
			if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
				t.Fatal(err)
			}
			if n := len(f.jobs.published()); n != 0 {
				t.Fatalf("送出 %d 則", n)
			}
		})
	}
}

// 失敗路徑：查詢失敗與資料不合法都回錯誤（River 重試），而且沒有送出任何訊息；查不到的名稱降級成空字串。
func TestFailuresReturnErrorsAndPublishNothing(t *testing.T) {
	boom := errors.New("boom")
	cases := map[string]func(*fixture){
		"messages":             func(f *fixture) { f.messages.err = boom },
		"buyers":               func(f *fixture) { f.buyers.err = boom },
		"stores":               func(f *fixture) { f.stores.err = boom },
		"unknown sender":       func(f *fixture) { f.messages.msg.SenderKind = "ROBOT" },
		"text without content": func(f *fixture) { f.messages.msg.Text = nil },
		"unknown type":         func(f *fixture) { f.messages.msg.Type = "CARD" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			mut(f)
			if err := f.worker.messageSent(t.Context(), sentArgs()); err == nil {
				t.Fatal("要回錯誤")
			}
			if n := len(f.jobs.published()); n != 0 {
				t.Fatalf("送出 %d 則", n)
			}
		})
	}
	if err := newFixture().worker.messageSent(t.Context(), MessageSentArgs{MessageID: "not-a-uuid"}); err == nil {
		t.Fatal("非 UUID 的訊息 ID 要回錯誤")
	}

	f := newFixture()
	f.buyers.display = []buyer.Display{{BuyerID: 11, Deleted: true}}
	if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
		t.Fatal(err)
	}
	got := body(t, f.jobs.published()[0])
	if got["sender_name"] != "" || got["sender_avatar"] != "" {
		t.Fatalf("刪帳的買家要送空字串：%v", got)
	}
}

func TestOnMessageSentEnqueuesOneJobPerEvent(t *testing.T) {
	q := &recordingJobs{}
	err := OnMessageSent(q)(t.Context(), im.MessageSent{EventID: eventID, OccurredAt: occurredAt, MessageID: messageID})
	if err != nil {
		t.Fatal(err)
	}
	if len(q.args) != 1 {
		t.Fatalf("jobs = %v", q.args)
	}
	if a, ok := q.args[0].(MessageSentArgs); !ok || a != sentArgs() {
		t.Fatalf("job = %#v", q.args[0])
	}
}

func storeSender(f *fixture, logo *string) {
	f.messages.msg.SenderKind = im.PartyStore
	f.stores.store.LogoKey = logo
}

func TestStoreSenderAvatarPrefersLogo(t *testing.T) {
	f := newFixture()
	storeSender(f, ptr("logo/22.jpg"))
	f.buyers.byAccount = []int64{90}
	f.buyers.display = []buyer.Display{{BuyerID: 90, Avatar: ptr("avatar/owner.jpg")}}
	if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
		t.Fatal(err)
	}
	if got := body(t, f.jobs.published()[0])["sender_avatar"]; got != "logo/22.jpg" {
		t.Fatalf("sender_avatar = %v", got)
	}
}

func TestStoreSenderAvatarFallsBackToTheLiveOwnerStint(t *testing.T) {
	for _, logo := range []*string{nil, ptr("")} {
		f := newFixture()
		storeSender(f, logo)
		f.buyers.byAccount = []int64{80, 90}
		f.buyers.display = []buyer.Display{
			{BuyerID: 80, Deleted: true, Avatar: ptr("avatar/old.jpg")},
			{BuyerID: 90, Avatar: ptr("avatar/owner.jpg")},
		}
		if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
			t.Fatal(err)
		}
		if got := body(t, f.jobs.published()[0])["sender_avatar"]; got != "avatar/owner.jpg" {
			t.Fatalf("logo %v：sender_avatar = %v", logo, got)
		}
	}
}

func TestStoreSenderAvatarOwnerNotFoundIsEmptyAndLookupFailureRetries(t *testing.T) {
	f := newFixture()
	storeSender(f, nil)
	if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
		t.Fatal(err)
	}
	if got := body(t, f.jobs.published()[0])["sender_avatar"]; got != "" {
		t.Fatalf("sender_avatar = %v", got)
	}

	f = newFixture()
	storeSender(f, nil)
	f.buyers.accountErr = errors.New("db down")
	if err := f.worker.messageSent(t.Context(), sentArgs()); err == nil || len(f.jobs.published()) != 0 {
		t.Fatalf("要回錯誤且不送：err = %v", err)
	}
}
