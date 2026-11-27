package chat

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	callerAccount = 1
	callerBuyer   = 70
	callerStoreID = 11
	otherStoreID  = 99
	otherBuyerID  = 5
)

var (
	at     = time.Date(2026, 11, 2, 3, 4, 5, 0, time.UTC)
	convID = uuid.UUID{7}
	reqID  = "0b1f1c2e-3d4a-4b5c-8d6e-7f8091a2b3c4"
)

type fakeReads struct {
	parties      im.ConversationParties
	partiesErr   error
	convPage     im.ConversationPage
	conversation im.Conversation
	byBuyer      int64
	msgPage      im.MessagePage
	unread       int64
	presence     []im.PresenceState
	err          error

	calls     []string
	party     im.Party
	after     *im.Cursor
	size      int32
	conv      uuid.UUID
	beforeSeq int64
	afterSeq  int64
	presIDs   []uuid.UUID
}

func (f *fakeReads) ListConversations(_ context.Context, p im.Party, after *im.Cursor, size int32) (im.ConversationPage, error) {
	f.calls, f.party, f.after, f.size = append(f.calls, "ListConversations"), p, after, size
	return f.convPage, f.err
}

func (f *fakeReads) StoreConversationByBuyer(_ context.Context, p im.Party, buyerID int64) (im.Conversation, error) {
	f.calls, f.party, f.byBuyer = append(f.calls, "StoreConversationByBuyer"), p, buyerID
	return f.conversation, f.err
}

func (f *fakeReads) ListMessages(_ context.Context, id uuid.UUID, p im.Party, before int64, size int32) (im.MessagePage, error) {
	f.calls, f.party, f.conv, f.beforeSeq, f.size = append(f.calls, "ListMessages"), p, id, before, size
	return f.msgPage, f.err
}

func (f *fakeReads) MessagesAfter(_ context.Context, id uuid.UUID, p im.Party, after int64, size int32) (im.MessagePage, error) {
	f.calls, f.party, f.conv, f.afterSeq, f.size = append(f.calls, "MessagesAfter"), p, id, after, size
	return f.msgPage, f.err
}

func (f *fakeReads) UnreadTotal(_ context.Context, p im.Party) (int64, error) {
	f.calls, f.party = append(f.calls, "UnreadTotal"), p
	return f.unread, f.err
}

func (f *fakeReads) Presence(_ context.Context, p im.Party, ids []uuid.UUID) ([]im.PresenceState, error) {
	f.calls, f.party, f.presIDs = append(f.calls, "Presence"), p, ids
	return f.presence, f.err
}

func (f *fakeReads) ConversationParties(_ context.Context, id uuid.UUID) (im.ConversationParties, error) {
	f.calls = append(f.calls, "ConversationParties")
	if f.partiesErr != nil {
		return im.ConversationParties{}, f.partiesErr
	}
	p := f.parties
	p.ConversationID = id
	return p, nil
}

type fakeWrites struct {
	msg   im.Message
	err   error
	calls []string
	send  im.SendInput
	party im.Party
	conv  uuid.UUID
	upto  int64
}

func (f *fakeWrites) Send(_ context.Context, in im.SendInput) (im.Message, error) {
	f.calls, f.send = append(f.calls, "Send"), in
	return f.msg, f.err
}

func (f *fakeWrites) MarkRead(_ context.Context, id uuid.UUID, p im.Party, upto int64) error {
	f.calls, f.conv, f.party, f.upto = append(f.calls, "MarkRead"), id, p, upto
	return f.err
}

func (f *fakeWrites) MarkAllRead(_ context.Context, p im.Party) error {
	f.calls, f.party = append(f.calls, "MarkAllRead"), p
	return f.err
}

type fakeBuyers struct {
	displays []buyer.Display
	batches  [][]int64
	swapped atomic.Int64
}

func (f *fakeBuyers) CurrentBuyerID(_ context.Context, accountID int64) (int64, error) {
	if id := f.swapped.Load(); id != 0 {
		return id, nil
	}
	if accountID != callerAccount {
		return 0, apperr.NotFound("no buyer")
	}
	return callerBuyer, nil
}

func (f *fakeBuyers) BatchDisplay(_ context.Context, ids []int64) ([]buyer.Display, error) {
	f.batches = append(f.batches, slices.Clone(ids))
	return f.displays, nil
}

type fakeStores struct {
	own   *seller.Store
	cards []seller.StoreCard
	asked [][]int64
}

func (f *fakeStores) CurrentStore(context.Context, int64) (seller.Store, error) {
	if f.own == nil {
		return seller.Store{}, apperr.NotFound("no store")
	}
	return *f.own, nil
}

func (f *fakeStores) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	f.asked = append(f.asked, slices.Clone(ids))
	return f.cards, nil
}

func images(t *testing.T) rpc.Images {
	t.Helper()
	im, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return im
}

func signedIn() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})
}

func asAdmin() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 9})
}

func codeOf(err error) connect.Code {
	return connect.CodeOf(rpc.ConnectError(context.Background(), err))
}

func reasonOf(err error) string {
	for _, d := range rpc.ConnectError(context.Background(), err).Details() {
		if v, e := d.Value(); e == nil {
			if info, ok := v.(*errdetails.ErrorInfo); ok {
				return info.GetReason()
			}
		}
	}
	return ""
}

func badField(err error) string {
	for _, d := range rpc.ConnectError(context.Background(), err).Details() {
		if v, e := d.Value(); e == nil {
			if br, ok := v.(*errdetails.BadRequest); ok && len(br.GetFieldViolations()) == 1 {
				return br.GetFieldViolations()[0].GetField()
			}
		}
	}
	return ""
}

func ownStore() *seller.Store { return &seller.Store{StoreID: callerStoreID, AccountID: callerAccount} }

type env struct {
	reads  *fakeReads
	writes *fakeWrites
	buyers *fakeBuyers
	stores *fakeStores
	app    *App
}

type reasonedError struct {
	error
	reason im.Reason
}

func withReason(err error, r im.Reason) error { return reasonedError{err, r} }

func (e reasonedError) Unwrap() error { return e.error }

func testReasonOf(err error) im.Reason {
	var r reasonedError
	if errors.As(err, &r) {
		return r.reason
	}
	return ""
}

func newEnv(t *testing.T, store *seller.Store) *env {
	t.Helper()
	e := &env{reads: &fakeReads{parties: im.ConversationParties{BuyerID: callerBuyer, StoreID: otherStoreID}}, writes: &fakeWrites{}, buyers: &fakeBuyers{}, stores: &fakeStores{own: store}}
	var err error
	if e.app, err = NewApp(e.reads, e.writes, &fakeStreams{}, &fakeAuth{}, e.buyers, e.stores, images(t), slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	e.app.reasonOf = testReasonOf
	return e
}

func (e *env) asStoreSide() {
	e.reads.parties = im.ConversationParties{BuyerID: otherBuyerID, StoreID: callerStoreID}
}

func text(s string) *pb.MessageContent {
	return &pb.MessageContent{Content: &pb.MessageContent_Text{Text: s}}
}

func sampleMessage(seq int64) im.Message {
	s := "哈囉"
	return im.Message{ID: uuid.UUID{byte(seq)}, ConversationID: convID, Seq: seq, SenderKind: im.PartyBuyer, Type: im.TypeText, Text: &s, CreatedAt: at}
}

func assertBuyerParty(t *testing.T, p im.Party) {
	t.Helper()
	if p.Kind != im.PartyBuyer || p.BuyerID != callerBuyer || p.Store != nil || p.ID() != callerBuyer {
		t.Errorf("要以買家 %d 的身分傳給模組：%+v", callerBuyer, p)
	}
}

func assertStoreParty(t *testing.T, p im.Party) {
	t.Helper()
	if p.Kind != im.PartyStore || p.Store == nil || p.Store.StoreID != callerStoreID || p.ID() != callerStoreID {
		t.Errorf("要以店 %d 的身分傳給模組：%+v", callerStoreID, p)
	}
}

// 13 支 unary 方法逐一：成功路徑，並斷言傳給模組的 Party 種類與 ID（買家版取自買家身分、店家版取自 seller.CurrentStore、對話方法取自歸屬推導）。
func TestEveryMethodSucceedsAndPassesTheDerivedPartyToTheModule(t *testing.T) {
	ctx := signedIn()
	cid := convID.String()
	t.Run("ListConversations", func(t *testing.T) {
		e := newEnv(t, ownStore())
		if _, err := e.app.ListConversations(ctx, connect.NewRequest(&pb.ListConversationsRequest{PageSize: 3})); err != nil {
			t.Fatal(err)
		}
		assertBuyerParty(t, e.reads.party)
		if e.reads.size != 3 {
			t.Errorf("size = %d", e.reads.size)
		}
	})
	t.Run("ListStoreConversations", func(t *testing.T) {
		e := newEnv(t, ownStore())
		if _, err := e.app.ListStoreConversations(ctx, connect.NewRequest(&pb.ListStoreConversationsRequest{})); err != nil {
			t.Fatal(err)
		}
		assertStoreParty(t, e.reads.party)
	})
	t.Run("ListMessages 買家方", func(t *testing.T) {
		e := newEnv(t, ownStore())
		if _, err := e.app.ListMessages(ctx, connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid})); err != nil {
			t.Fatal(err)
		}
		assertBuyerParty(t, e.reads.party)
		if e.reads.conv != convID {
			t.Errorf("conv = %v", e.reads.conv)
		}
	})
	t.Run("ListMessages 店家方", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.asStoreSide()
		if _, err := e.app.ListMessages(ctx, connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid})); err != nil {
			t.Fatal(err)
		}
		assertStoreParty(t, e.reads.party)
	})
	t.Run("SendMessage 買家方", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.writes.msg = sampleMessage(1)
		if _, err := e.app.SendMessage(ctx, connect.NewRequest(&pb.SendMessageRequest{ConversationId: cid, RequestId: reqID, Content: text("嗨")})); err != nil {
			t.Fatal(err)
		}
		assertBuyerParty(t, e.writes.send.Sender)
		if e.writes.send.ConversationID != convID || e.writes.send.CounterpartID != 0 {
			t.Errorf("以對話 ID 送出：%+v", e.writes.send)
		}
	})
	t.Run("SendMessage 店家方", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.asStoreSide()
		e.writes.msg = sampleMessage(1)
		if _, err := e.app.SendMessage(ctx, connect.NewRequest(&pb.SendMessageRequest{ConversationId: cid, RequestId: reqID, Content: text("嗨")})); err != nil {
			t.Fatal(err)
		}
		assertStoreParty(t, e.writes.send.Sender)
	})
	t.Run("SendMessageToStore", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.writes.msg = sampleMessage(1)
		if _, err := e.app.SendMessageToStore(ctx, connect.NewRequest(&pb.SendMessageToStoreRequest{StoreId: "99", RequestId: reqID, Content: text("嗨")})); err != nil {
			t.Fatal(err)
		}
		assertBuyerParty(t, e.writes.send.Sender)
		if e.writes.send.CounterpartID != 99 || e.writes.send.ConversationID != (uuid.UUID{}) {
			t.Errorf("以對方 store_id 送出：%+v", e.writes.send)
		}
	})
	t.Run("SendMessageToBuyer", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.writes.msg = sampleMessage(1)
		if _, err := e.app.SendMessageToBuyer(ctx, connect.NewRequest(&pb.SendMessageToBuyerRequest{BuyerId: "5", RequestId: reqID, Content: text("嗨")})); err != nil {
			t.Fatal(err)
		}
		assertStoreParty(t, e.writes.send.Sender)
		if e.writes.send.CounterpartID != 5 {
			t.Errorf("以對方 buyer_id 送出：%+v", e.writes.send)
		}
	})
	t.Run("MarkConversationRead", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.asStoreSide()
		if _, err := e.app.MarkConversationRead(ctx, connect.NewRequest(&pb.MarkConversationReadRequest{ConversationId: cid, UptoSeq: 4})); err != nil {
			t.Fatal(err)
		}
		assertStoreParty(t, e.writes.party)
		if e.writes.conv != convID || e.writes.upto != 4 {
			t.Errorf("conv=%v upto=%d", e.writes.conv, e.writes.upto)
		}
	})
	t.Run("MarkAllRead", func(t *testing.T) {
		e := newEnv(t, ownStore())
		if _, err := e.app.MarkAllRead(ctx, connect.NewRequest(&pb.MarkAllReadRequest{})); err != nil {
			t.Fatal(err)
		}
		assertBuyerParty(t, e.writes.party)
	})
	t.Run("MarkAllStoreRead", func(t *testing.T) {
		e := newEnv(t, ownStore())
		if _, err := e.app.MarkAllStoreRead(ctx, connect.NewRequest(&pb.MarkAllStoreReadRequest{})); err != nil {
			t.Fatal(err)
		}
		assertStoreParty(t, e.writes.party)
	})
	t.Run("GetUnreadCount", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.reads.unread = 6
		resp, err := e.app.GetUnreadCount(ctx, connect.NewRequest(&pb.GetUnreadCountRequest{}))
		if err != nil || resp.Msg.GetUnreadCount() != 6 {
			t.Fatalf("%v %v", resp, err)
		}
		assertBuyerParty(t, e.reads.party)
	})
	t.Run("GetStoreUnreadCount", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.reads.unread = 8
		resp, err := e.app.GetStoreUnreadCount(ctx, connect.NewRequest(&pb.GetStoreUnreadCountRequest{}))
		if err != nil || resp.Msg.GetUnreadCount() != 8 {
			t.Fatalf("%v %v", resp, err)
		}
		assertStoreParty(t, e.reads.party)
	})
	t.Run("GetPresence", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.reads.presence = []im.PresenceState{{ConversationID: convID, Online: true}}
		resp, err := e.app.GetPresence(ctx, connect.NewRequest(&pb.GetPresenceRequest{ConversationIds: []string{cid}}))
		if err != nil || len(resp.Msg.GetPresences()) != 1 || !resp.Msg.GetPresences()[0].GetOnline() || resp.Msg.GetPresences()[0].GetConversationId() != cid {
			t.Fatalf("%v %v", resp, err)
		}
		assertBuyerParty(t, e.reads.party)
		if len(e.reads.presIDs) != 1 || e.reads.presIDs[0] != convID {
			t.Errorf("ids = %v", e.reads.presIDs)
		}
	})
	t.Run("GetStorePresence", func(t *testing.T) {
		e := newEnv(t, ownStore())
		e.reads.presence = []im.PresenceState{{ConversationID: convID}}
		resp, err := e.app.GetStorePresence(ctx, connect.NewRequest(&pb.GetStorePresenceRequest{ConversationIds: []string{cid}}))
		if err != nil || len(resp.Msg.GetPresences()) != 1 || resp.Msg.GetPresences()[0].GetOnline() {
			t.Fatalf("%v %v", resp, err)
		}
		assertStoreParty(t, e.reads.party)
	})
}

func TestStoreVersionsWithoutAStoreAreNotFoundAndNeverReachTheModule(t *testing.T) {
	e := newEnv(t, nil)
	ctx := signedIn()
	calls := map[string]func() error{
		"GetStoreConversationByBuyer": func() error {
			_, err := e.app.GetStoreConversationByBuyer(ctx, connect.NewRequest(&pb.GetStoreConversationByBuyerRequest{BuyerId: "5"}))
			return err
		},
		"ListStoreConversations": func() error {
			_, err := e.app.ListStoreConversations(ctx, connect.NewRequest(&pb.ListStoreConversationsRequest{}))
			return err
		},
		"SendMessageToBuyer": func() error {
			_, err := e.app.SendMessageToBuyer(ctx, connect.NewRequest(&pb.SendMessageToBuyerRequest{BuyerId: "5", RequestId: reqID, Content: text("嗨")}))
			return err
		},
		"MarkAllStoreRead": func() error {
			_, err := e.app.MarkAllStoreRead(ctx, connect.NewRequest(&pb.MarkAllStoreReadRequest{}))
			return err
		},
		"GetStoreUnreadCount": func() error {
			_, err := e.app.GetStoreUnreadCount(ctx, connect.NewRequest(&pb.GetStoreUnreadCountRequest{}))
			return err
		},
		"GetStorePresence": func() error {
			_, err := e.app.GetStorePresence(ctx, connect.NewRequest(&pb.GetStorePresenceRequest{ConversationIds: []string{convID.String()}}))
			return err
		},
	}
	for name, call := range calls {
		if err := call(); codeOf(err) != connect.CodeNotFound {
			t.Errorf("%s 沒有店：%v", name, err)
		}
	}
	if len(e.reads.calls) != 0 || len(e.writes.calls) != 0 {
		t.Errorf("不得呼叫模組：%v %v", e.reads.calls, e.writes.calls)
	}
	if _, err := e.app.ListConversations(asAdmin(), connect.NewRequest(&pb.ListConversationsRequest{})); codeOf(err) != connect.CodePermissionDenied {
		t.Errorf("管理員：%v", err)
	}
}

func TestOwnershipDerivationNeitherSideAndMissingConversationAreIndistinguishable(t *testing.T) {
	cid := convID.String()
	ctx := signedIn()
	neither := newEnv(t, ownStore())
	neither.reads.parties = im.ConversationParties{BuyerID: otherBuyerID, StoreID: otherStoreID}
	missing := newEnv(t, ownStore())
	missing.reads.partiesErr = apperr.NotFound("模組自己的找不到句")
	var msgs []string
	for name, e := range map[string]*env{"兩邊都不是": neither, "不存在": missing} {
		for method, call := range map[string]func() error{
			"ListMessages": func() error {
				_, err := e.app.ListMessages(ctx, connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid}))
				return err
			},
			"SendMessage": func() error {
				_, err := e.app.SendMessage(ctx, connect.NewRequest(&pb.SendMessageRequest{ConversationId: cid, RequestId: reqID, Content: text("x")}))
				return err
			},
			"MarkConversationRead": func() error {
				_, err := e.app.MarkConversationRead(ctx, connect.NewRequest(&pb.MarkConversationReadRequest{ConversationId: cid}))
				return err
			},
		} {
			err := call()
			if codeOf(err) != connect.CodeNotFound || reasonOf(err) != "" {
				t.Errorf("%s/%s：%v", name, method, err)
			}
			msgs = append(msgs, rpc.ConnectError(context.Background(), err).Error())
		}
		if len(e.writes.calls) != 0 || slices.Contains(e.reads.calls, "ListMessages") {
			t.Errorf("%s：不得呼叫模組的讀寫：%v %v", name, e.reads.calls, e.writes.calls)
		}
	}
	for _, m := range msgs {
		if m != msgs[0] {
			t.Fatalf("兩種 not_found 必須完全相同：%q vs %q", m, msgs[0])
		}
	}
}

func TestOwnershipDerivationRejectsAccountThatIsBothSidesAsInternal(t *testing.T) {
	e := newEnv(t, ownStore())
	e.reads.parties = im.ConversationParties{BuyerID: callerBuyer, StoreID: callerStoreID}
	_, err := e.app.ListMessages(signedIn(), connect.NewRequest(&pb.ListMessagesRequest{ConversationId: convID.String()}))
	if codeOf(err) != connect.CodeInternal || slices.Contains(e.reads.calls, "ListMessages") {
		t.Fatalf("%v %v", err, e.reads.calls)
	}
}

func TestSendMessageValidatesRequestIDAndContent(t *testing.T) {
	cases := []struct {
		name  string
		reqID string
		cont  *pb.MessageContent
		field string
	}{
		{"沒有 request_id", "", text("x"), "request_id"},
		{"request_id 不是 UUIDv4", "not-a-uuid", text("x"), "request_id"},
		{"request_id 是 v1", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", text("x"), "request_id"},
		{"沒有內容", reqID, nil, "content"},
		{"內容是空 oneof", reqID, &pb.MessageContent{}, "content"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, ownStore())
			ctx := signedIn()
			cid := convID.String()
			for method, call := range map[string]func() error{
				"SendMessage": func() error {
					_, err := e.app.SendMessage(ctx, connect.NewRequest(&pb.SendMessageRequest{ConversationId: cid, RequestId: c.reqID, Content: c.cont}))
					return err
				},
				"SendMessageToStore": func() error {
					_, err := e.app.SendMessageToStore(ctx, connect.NewRequest(&pb.SendMessageToStoreRequest{StoreId: "99", RequestId: c.reqID, Content: c.cont}))
					return err
				},
				"SendMessageToBuyer": func() error {
					_, err := e.app.SendMessageToBuyer(ctx, connect.NewRequest(&pb.SendMessageToBuyerRequest{BuyerId: "5", RequestId: c.reqID, Content: c.cont}))
					return err
				},
			} {
				err := call()
				if codeOf(err) != connect.CodeInvalidArgument || badField(err) != c.field {
					t.Errorf("%s：code=%v field=%q err=%v", method, codeOf(err), badField(err), err)
				}
			}
			if len(e.writes.calls) != 0 {
				t.Errorf("不得送出：%v", e.writes.calls)
			}
		})
	}
	e := newEnv(t, ownStore())
	if _, err := e.app.SendMessage(signedIn(), connect.NewRequest(&pb.SendMessageRequest{ConversationId: "x", RequestId: reqID, Content: text("x")})); badField(err) != "conversation_id" {
		t.Errorf("conversation_id 格式：%v", err)
	}
	if _, err := e.app.SendMessageToStore(signedIn(), connect.NewRequest(&pb.SendMessageToStoreRequest{StoreId: "0", RequestId: reqID, Content: text("x")})); badField(err) != "store_id" {
		t.Errorf("store_id 格式：%v", err)
	}
	if _, err := e.app.SendMessageToBuyer(signedIn(), connect.NewRequest(&pb.SendMessageToBuyerRequest{BuyerId: "-1", RequestId: reqID, Content: text("x")})); badField(err) != "buyer_id" {
		t.Errorf("buyer_id 格式：%v", err)
	}
}

func TestSendMessagePassesRequestIDAndContentToTheModule(t *testing.T) {
	e := newEnv(t, ownStore())
	e.writes.msg = sampleMessage(1)
	if _, err := e.app.SendMessage(signedIn(), connect.NewRequest(&pb.SendMessageRequest{ConversationId: convID.String(), RequestId: reqID, Content: text("你好")})); err != nil {
		t.Fatal(err)
	}
	in := e.writes.send
	if in.RequestID.String() != reqID || in.Type != im.TypeText || in.Text != "你好" || in.StagedToken != "" {
		t.Errorf("文字：%+v", in)
	}
	img := &pb.MessageContent{Content: &pb.MessageContent_ImageStagedToken{ImageStagedToken: "tok"}}
	if _, err := e.app.SendMessageToStore(signedIn(), connect.NewRequest(&pb.SendMessageToStoreRequest{StoreId: "99", RequestId: reqID, Content: img})); err != nil {
		t.Fatal(err)
	}
	in = e.writes.send
	if in.RequestID.String() != reqID || in.Type != im.TypeImage || in.StagedToken != "tok" || in.Text != "" {
		t.Errorf("圖片：%+v", in)
	}
}

func TestSendMessageMapsModuleReasonsToTheContractReasons(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		code   connect.Code
		reason string
	}{
		{"自己傳給自己", withReason(apperr.InvalidInput("自己"), im.ReasonSelfMessageNotAllowed), connect.CodeInvalidArgument, "SELF_MESSAGE_NOT_ALLOWED"},
		{"店家停權", withReason(apperr.PermissionDenied("停權"), im.ReasonSellerSuspended), connect.CodePermissionDenied, "SELLER_SUSPENDED"},
		{"停業", withReason(apperr.FailedPrecondition("停業"), im.ReasonStoreClosed), connect.CodeFailedPrecondition, "STORE_CLOSED"},
		{"封鎖", withReason(apperr.FailedPrecondition("封鎖"), im.ReasonChatBlocked), connect.CodeFailedPrecondition, "CHAT_BLOCKED"},
		{"對方已刪除", withReason(apperr.FailedPrecondition("刪除"), im.ReasonReceiverDeleted), connect.CodeFailedPrecondition, "RECEIVER_DELETED"},
		{"詞典拒絕不帶 reason", withReason(apperr.InvalidInputOn([]string{"Text"}, "不合規"), im.ReasonModerationRejected), connect.CodeInvalidArgument, ""},
		{"找不到對方不帶 reason", apperr.NotFound("找不到對方"), connect.CodeNotFound, ""},
		{"暫時無法驗證", apperr.Unavailable("稍後再試"), connect.CodeUnavailable, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, ownStore())
			e.writes.err = c.err
			_, err := e.app.SendMessageToStore(signedIn(), connect.NewRequest(&pb.SendMessageToStoreRequest{StoreId: "99", RequestId: reqID, Content: text("x")}))
			if codeOf(err) != c.code || reasonOf(err) != c.reason {
				t.Errorf("code=%v reason=%q，預期 %v %q", codeOf(err), reasonOf(err), c.code, c.reason)
			}
		})
	}
}

func TestModuleFieldErrorsMapToTheRequestFields(t *testing.T) {
	for module, want := range map[string]string{"Text": "content.text", "StagedToken": "content.image_staged_token", "RequestID": "request_id", "Type": "content"} {
		e := newEnv(t, ownStore())
		e.writes.err = apperr.InvalidInputOn([]string{module}, "x")
		_, err := e.app.SendMessage(signedIn(), connect.NewRequest(&pb.SendMessageRequest{ConversationId: convID.String(), RequestId: reqID, Content: text("x")}))
		if badField(err) != want {
			t.Errorf("%s 要指向 %s：%q", module, want, badField(err))
		}
	}
	e := newEnv(t, ownStore())
	e.writes.err = apperr.InvalidInputOn([]string{"CounterpartID"}, "x")
	if _, err := e.app.SendMessageToStore(signedIn(), connect.NewRequest(&pb.SendMessageToStoreRequest{StoreId: "99", RequestId: reqID, Content: text("x")})); badField(err) != "store_id" {
		t.Errorf("CounterpartID 要指向 store_id：%v", err)
	}
	if _, err := e.app.SendMessageToBuyer(signedIn(), connect.NewRequest(&pb.SendMessageToBuyerRequest{BuyerId: "5", RequestId: reqID, Content: text("x")})); badField(err) != "buyer_id" {
		t.Errorf("CounterpartID 要指向 buyer_id：%v", err)
	}
}

func TestSendMessageReturnsTheFullMessage(t *testing.T) {
	e := newEnv(t, ownStore())
	e.writes.msg = sampleMessage(3)
	resp, err := e.app.SendMessage(signedIn(), connect.NewRequest(&pb.SendMessageRequest{ConversationId: convID.String(), RequestId: reqID, Content: text("x")}))
	if err != nil {
		t.Fatal(err)
	}
	m := resp.Msg.GetMessage()
	if m.GetId() != (uuid.UUID{3}).String() || m.GetConversationId() != convID.String() || m.GetSeq() != 3 || m.GetSender() != pb.ChatParty_CHAT_PARTY_BUYER ||
		m.GetKind() != pb.MessageKind_MESSAGE_KIND_TEXT || m.GetText() != "哈囉" || m.Image != nil || m.GetHidden() || m.GetErased() || !m.GetCreateTime().AsTime().Equal(at) {
		t.Errorf("%v", m)
	}
}

func TestMessagesNeverLeakObjectKeysAccountIDsOrTombstoneContent(t *testing.T) {
	e := newEnv(t, ownStore())
	txt := "不該出現"
	hidden := sampleMessage(1)
	hidden.Hidden, hidden.Text = true, &txt
	erased := sampleMessage(2)
	erased.Erased, erased.Type, erased.Image = true, im.TypeImage, &im.Image{AssetID: uuid.UUID{0xa2}, ObjectKey: "chat/secret-erased.jpg"}
	erased.Text = &txt
	live := sampleMessage(3)
	live.SenderKind, live.Type, live.Text = im.PartyStore, im.TypeImage, nil
	live.Image = &im.Image{AssetID: uuid.UUID{0xa3}, ObjectKey: "chat/secret-live.jpg"}
	e.reads.msgPage = im.MessagePage{Items: []im.Message{hidden, erased, live}, HasMore: true, PeerReadSeq: 2}
	resp, err := e.app.ListMessages(signedIn(), connect.NewRequest(&pb.ListMessagesRequest{ConversationId: convID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetMessages()
	if len(got) != 3 || !resp.Msg.GetHasMore() || resp.Msg.GetPeerReadSeq() != 2 {
		t.Fatalf("%v", resp.Msg)
	}
	for _, m := range got[:2] {
		if m.Text != nil || m.Image != nil || (!m.GetHidden() && !m.GetErased()) {
			t.Errorf("墓碑不帶內容：%v", m)
		}
	}
	if got[2].GetImage().GetUriTemplate() != "https://media.example.test/i/{preset}/chat/secret-live.jpg" || got[2].GetSender() != pb.ChatParty_CHAT_PARTY_STORE || got[2].GetKind() != pb.MessageKind_MESSAGE_KIND_IMAGE {
		t.Errorf("圖片訊息：%v", got[2])
	}
	raw, err := protojson.Marshal(resp.Msg)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"secret-erased", "ObjectKey", "objectKey", "不該出現", uuid.UUID{0xa3}.String(), "accountId"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("回應不得含 %q：%s", leak, raw)
		}
	}
}

func TestListMessagesBeforeAndAfterSeqAreMutuallyExclusiveAndRouteToTheRightModuleCall(t *testing.T) {
	e := newEnv(t, ownStore())
	cid := convID.String()
	five, nine, neg := int64(5), int64(9), int64(-1)
	if _, err := e.app.ListMessages(signedIn(), connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid, BeforeSeq: &five, AfterSeq: &nine})); codeOf(err) != connect.CodeInvalidArgument || badField(err) != "after_seq" {
		t.Fatalf("兩者都給：%v", err)
	}
	if len(e.reads.calls) != 0 {
		t.Fatalf("不得呼叫模組：%v", e.reads.calls)
	}
	for field, req := range map[string]*pb.ListMessagesRequest{
		"before_seq": {ConversationId: cid, BeforeSeq: &neg}, "after_seq": {ConversationId: cid, AfterSeq: &neg}, "page_size": {ConversationId: cid, PageSize: -1},
	} {
		if _, err := e.app.ListMessages(signedIn(), connect.NewRequest(req)); codeOf(err) != connect.CodeInvalidArgument || badField(err) != field {
			t.Errorf("%s：%v", field, err)
		}
	}
	if _, err := e.app.ListMessages(signedIn(), connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid, BeforeSeq: &five, PageSize: 7})); err != nil {
		t.Fatal(err)
	}
	if e.reads.calls[len(e.reads.calls)-1] != "ListMessages" || e.reads.beforeSeq != 5 || e.reads.size != 7 {
		t.Errorf("before：%v %d %d", e.reads.calls, e.reads.beforeSeq, e.reads.size)
	}
	if _, err := e.app.ListMessages(signedIn(), connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid, AfterSeq: &nine})); err != nil {
		t.Fatal(err)
	}
	if e.reads.calls[len(e.reads.calls)-1] != "MessagesAfter" || e.reads.afterSeq != 9 {
		t.Errorf("after：%v %d", e.reads.calls, e.reads.afterSeq)
	}
	zero := int64(0)
	if _, err := e.app.ListMessages(signedIn(), connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid, AfterSeq: &zero})); err != nil || e.reads.calls[len(e.reads.calls)-1] != "MessagesAfter" {
		t.Errorf("after_seq=0 是有效的補洞起點：%v %v", err, e.reads.calls)
	}
}

// 明確給 before_seq 為 0 是「seq 小於 0」的空頁，不是從最新開始；不設才是從最新開始。
func TestListMessagesExplicitZeroBeforeSeqIsNotTheLatestPage(t *testing.T) {
	e := newEnv(t, ownStore())
	cid := convID.String()
	if _, err := e.app.ListMessages(signedIn(), connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid})); err != nil || e.reads.beforeSeq != 0 {
		t.Fatalf("沒設 before_seq 要從最新開始（模組收到 0）：%v %d", err, e.reads.beforeSeq)
	}
	zero := int64(0)
	if _, err := e.app.ListMessages(signedIn(), connect.NewRequest(&pb.ListMessagesRequest{ConversationId: cid, BeforeSeq: &zero})); err != nil || e.reads.beforeSeq == 0 {
		t.Fatalf("before_seq=0 不得以 0 交給模組（0 在模組是從最新開始）：%v %d", err, e.reads.beforeSeq)
	}
}

func TestNewAppRejectsMissingReadsAndWrites(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	if _, err := NewApp(nil, &fakeWrites{}, &fakeStreams{}, &fakeAuth{}, &fakeBuyers{}, &fakeStores{}, images(t), log); err == nil {
		t.Error("缺 Reads 要啟動失敗")
	}
	if _, err := NewApp(&fakeReads{}, nil, &fakeStreams{}, &fakeAuth{}, &fakeBuyers{}, &fakeStores{}, images(t), log); err == nil {
		t.Error("缺 Writes 要啟動失敗")
	}
}

func sampleConversation(id byte, storeID, buyerID int64) im.Conversation {
	txt := "最後一句"
	return im.Conversation{
		ID: uuid.UUID{id}, BuyerID: buyerID, StoreID: storeID, LastSeq: 9, Unread: 4, LastMessageAt: at, PeerReadSeq: 6,
		Preview: im.Preview{Type: im.TypeText, Text: &txt, SenderKind: im.PartyStore},
	}
}

func TestListConversationsBuyerViewAssemblesStoreCardAndPreview(t *testing.T) {
	e := newEnv(t, ownStore())
	name, logo := "好店", "stores/logo.jpg"
	e.stores.cards = []seller.StoreCard{{StoreID: 99, StoreCode: 1234, Name: &name, LogoKey: &logo, Status: "OPEN"}}
	e.reads.convPage = im.ConversationPage{Items: []im.Conversation{sampleConversation(1, 99, callerBuyer), sampleConversation(2, 99, callerBuyer), sampleConversation(3, 98, callerBuyer)}}
	resp, err := e.app.ListConversations(signedIn(), connect.NewRequest(&pb.ListConversationsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.stores.asked) != 1 || !slices.Equal(e.stores.asked[0], []int64{99, 98}) {
		t.Errorf("店家卡片以一次批次查詢、去重：%v", e.stores.asked)
	}
	got := resp.Msg.GetConversations()
	c := got[0]
	if c.GetId() != (uuid.UUID{1}).String() || c.GetUnreadCount() != 4 || !c.GetLastMessageTime().AsTime().Equal(at) || c.GetPeerReadSeq() != 6 || c.Buyer != nil {
		t.Errorf("%v", c)
	}
	if c.GetStore().GetName() != "好店" || c.GetStore().GetLogo().GetUriTemplate() != "https://media.example.test/i/{preset}/stores/logo.jpg" {
		t.Errorf("店家卡片：%v", c.GetStore())
	}
	if p := c.GetPreview(); p.GetKind() != pb.MessageKind_MESSAGE_KIND_TEXT || p.GetText() != "最後一句" || p.GetSender() != pb.ChatParty_CHAT_PARTY_STORE || p.GetHidden() || p.GetErased() {
		t.Errorf("預覽：%v", p)
	}
	if got[2].Store != nil {
		t.Error("沒有卡片的店不設 store")
	}
}

func TestListStoreConversationsAssemblesBuyerDisplayAndMarksDeletedBuyers(t *testing.T) {
	e := newEnv(t, ownStore())
	name, avatar := "阿美", "avatars/5.jpg"
	e.buyers.displays = []buyer.Display{{BuyerID: 5, DisplayName: &name, Avatar: &avatar}, {BuyerID: 6, Deleted: true}}
	e.reads.convPage = im.ConversationPage{Items: []im.Conversation{sampleConversation(1, callerStoreID, 5), sampleConversation(2, callerStoreID, 6), sampleConversation(3, callerStoreID, 7)}}
	resp, err := e.app.ListStoreConversations(signedIn(), connect.NewRequest(&pb.ListStoreConversationsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.buyers.batches) != 1 || len(e.buyers.batches[0]) != 3 {
		t.Errorf("買家以一次批次查詢：%v", e.buyers.batches)
	}
	got := resp.Msg.GetConversations()
	if b := got[0].GetBuyer(); b.GetId() != "5" || b.GetName() != "阿美" || b.GetAvatar().GetUriTemplate() != "https://media.example.test/i/{preset}/avatars/5.jpg" || b.GetDeleted() || got[0].Store != nil {
		t.Errorf("買家：%v", got[0])
	}
	for _, c := range got[1:] {
		if b := c.GetBuyer(); !b.GetDeleted() || b.Name != nil || b.Avatar != nil {
			t.Errorf("已刪除或查不到的買家沒有名稱與頭像：%v", c)
		}
	}
}

func TestConversationJSONDoesNotLeakCounterpartAccountOrObjectKeys(t *testing.T) {
	e := newEnv(t, ownStore())
	avatar := "avatars/secret-key.jpg"
	name := "阿美"
	e.buyers.displays = []buyer.Display{{BuyerID: 5, DisplayName: &name, Avatar: &avatar}}
	e.reads.convPage = im.ConversationPage{Items: []im.Conversation{sampleConversation(1, callerStoreID, 5)}}
	resp, err := e.app.ListStoreConversations(signedIn(), connect.NewRequest(&pb.ListStoreConversationsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := protojson.Marshal(resp.Msg)
	for _, leak := range []string{"accountId", "account_id", "objectKey", "object_key", "assetId"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("回應不得含 %q：%s", leak, raw)
		}
	}
}

func TestPreviewIsStructuredAndTombstonePreviewsCarryNoText(t *testing.T) {
	txt := "不該出現"
	for name, p := range map[string]im.Preview{
		"隱藏": {Type: im.TypeText, Text: &txt, Hidden: true, SenderKind: im.PartyBuyer},
		"抹除": {Type: im.TypeText, Text: &txt, Erased: true, SenderKind: im.PartyBuyer},
	} {
		got := toPreview(p)
		if got.Text != nil || got.GetHidden() != p.Hidden || got.GetErased() != p.Erased || got.GetKind() != pb.MessageKind_MESSAGE_KIND_TEXT {
			t.Errorf("%s：%v", name, got)
		}
	}
	if got := toPreview(im.Preview{Type: im.TypeImage, SenderKind: im.PartyStore}); got.GetKind() != pb.MessageKind_MESSAGE_KIND_IMAGE || got.Text != nil {
		t.Errorf("圖片預覽：%v", got)
	}
}

func TestConversationPageTokenRoundTripsAndIsBoundToProcedureAndIdentity(t *testing.T) {
	e := newEnv(t, ownStore())
	next := im.Cursor{At: at, ID: uuid.UUID{9}}
	e.reads.convPage = im.ConversationPage{Items: []im.Conversation{sampleConversation(1, 99, callerBuyer)}, Next: &next}
	resp, err := e.app.ListConversations(signedIn(), connect.NewRequest(&pb.ListConversationsRequest{}))
	if err != nil || resp.Msg.GetNextPageToken() == "" {
		t.Fatalf("有下一頁要帶 token：%v %v", resp, err)
	}
	token := resp.Msg.GetNextPageToken()
	if _, err := e.app.ListConversations(signedIn(), connect.NewRequest(&pb.ListConversationsRequest{PageToken: token})); err != nil {
		t.Fatal(err)
	}
	if e.reads.after == nil || !e.reads.after.At.Equal(at) || e.reads.after.ID != next.ID {
		t.Errorf("游標要傳進模組：%+v", e.reads.after)
	}
	if _, err := e.app.ListStoreConversations(signedIn(), connect.NewRequest(&pb.ListStoreConversationsRequest{PageToken: token})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("買家版的 token 不能用在店家版：%v", err)
	}
	other := newEnv(t, ownStore())
	other.buyers = &fakeBuyers{}
	app, err := NewApp(other.reads, other.writes, &fakeStreams{}, &fakeAuth{}, otherBuyerStub{}, other.stores, images(t), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ListConversations(signedIn(), connect.NewRequest(&pb.ListConversationsRequest{PageToken: token})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("換了買家身分不能沿用 token：%v", err)
	}
	if _, err := e.app.ListConversations(signedIn(), connect.NewRequest(&pb.ListConversationsRequest{PageToken: "garbage"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("壞 token：%v", err)
	}
	if _, err := e.app.ListConversations(signedIn(), connect.NewRequest(&pb.ListConversationsRequest{PageSize: -1})); badField(err) != "page_size" {
		t.Errorf("負的 page_size：%v", err)
	}
}

type otherBuyerStub struct{}

func (otherBuyerStub) CurrentBuyerID(context.Context, int64) (int64, error) { return 71, nil }

func (otherBuyerStub) BatchDisplay(context.Context, []int64) ([]buyer.Display, error) {
	return nil, nil
}

func TestPresenceValidatesCountAndFormatAndPropagatesNotFound(t *testing.T) {
	e := newEnv(t, ownStore())
	ids := make([]string, im.MaxPresenceConversations+1)
	for i := range ids {
		ids[i] = (uuid.UUID{byte(i + 1)}).String()
	}
	for name, call := range map[string]func([]string) error{
		"GetPresence": func(v []string) error {
			_, err := e.app.GetPresence(signedIn(), connect.NewRequest(&pb.GetPresenceRequest{ConversationIds: v}))
			return err
		},
		"GetStorePresence": func(v []string) error {
			_, err := e.app.GetStorePresence(signedIn(), connect.NewRequest(&pb.GetStorePresenceRequest{ConversationIds: v}))
			return err
		},
	} {
		if err := call(ids); codeOf(err) != connect.CodeInvalidArgument || badField(err) != "conversation_ids" {
			t.Errorf("%s 超過 50：%v", name, err)
		}
		if err := call(nil); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s 空的：%v", name, err)
		}
		if err := call([]string{"x"}); codeOf(err) != connect.CodeInvalidArgument || badField(err) != "conversation_ids[0]" {
			t.Errorf("%s 格式：%v", name, err)
		}
		if err := call(ids[:im.MaxPresenceConversations]); err != nil {
			t.Errorf("%s 剛好 50 個：%v", name, err)
		}
		e.reads.err = apperr.NotFound("不是自己的對話")
		if err := call(ids[:2]); codeOf(err) != connect.CodeNotFound {
			t.Errorf("%s 不是自己的對話整個請求 not_found：%v", name, err)
		}
		e.reads.err = nil
	}
}

func TestMarkConversationReadRejectsNegativeUptoSeq(t *testing.T) {
	e := newEnv(t, ownStore())
	if _, err := e.app.MarkConversationRead(signedIn(), connect.NewRequest(&pb.MarkConversationReadRequest{ConversationId: convID.String(), UptoSeq: -1})); badField(err) != "upto_seq" || len(e.writes.calls) != 0 {
		t.Fatalf("%v %v", err, e.writes.calls)
	}
}

func TestEveryModuleReasonTheClientBranchesOnIsMappedToAContractReason(t *testing.T) {
	want := map[im.Reason]string{
		im.ReasonSelfMessageNotAllowed: "SELF_MESSAGE_NOT_ALLOWED", im.ReasonSellerSuspended: "SELLER_SUSPENDED", im.ReasonStoreClosed: "STORE_CLOSED",
		im.ReasonChatBlocked: "CHAT_BLOCKED", im.ReasonReceiverDeleted: "RECEIVER_DELETED",
	}
	if len(reasons) != len(want) {
		t.Fatalf("對照表 %d 項，預期 %d", len(reasons), len(want))
	}
	for r, name := range want {
		if got := reasons[r].String(); got != name {
			t.Errorf("%s 對到 %q，預期 %q", r, got, name)
		}
	}
}

func TestMessageEnumsMapBothWaysAndNeverCarryUnspecifiedToTheModule(t *testing.T) {
	if got := rpc.EnumMismatches[pb.MessageKind]([]im.MessageType{im.TypeText, im.TypeImage}); len(got) != 0 {
		t.Errorf("MessageKind：%v", got)
	}
	if got := rpc.EnumMismatches[pb.ChatParty]([]im.PartyKind{im.PartyBuyer, im.PartyStore}); len(got) != 0 {
		t.Errorf("ChatParty：%v", got)
	}
	if _, err := sendInput(reqID, &pb.MessageContent{}); badField(err) != "content" {
		t.Errorf("沒有內容：%v", err)
	}
}

// 宣告表與 ChatAppService 的方法一一對應：缺一個或多一個都不能啟動（NewApp 檢查的就是這個）。
func TestDeclarationsMatchTheServiceMethods(t *testing.T) {
	svc := serviceDescriptor()
	if err := checkDeclarations(declarations, svc); err != nil {
		t.Fatal(err)
	}
	if svc.Methods().Len() != 16 || len(declarations) != 16 {
		t.Fatalf("方法 %d、宣告 %d", svc.Methods().Len(), len(declarations))
	}
	for procedure := range declarations {
		short := make(map[string]declaration, len(declarations))
		for k, v := range declarations {
			if k != procedure {
				short[k] = v
			}
		}
		if err := checkDeclarations(short, svc); err == nil {
			t.Errorf("缺 %s 的宣告卻通過了檢查", procedure)
		}
		if _, err := newApp(short, &fakeReads{}, &fakeWrites{}, &fakeStreams{}, &fakeAuth{}, &fakeBuyers{}, &fakeStores{}, images(t), slog.New(slog.DiscardHandler)); err == nil {
			t.Errorf("缺 %s 的宣告表要不能啟動", procedure)
		}
	}
	extra := map[string]declaration{"/hawkr.chat.v1alpha1.ChatAppService/Nope": exempt("x")}
	for k, v := range declarations {
		extra[k] = v
	}
	if err := checkDeclarations(extra, svc); err == nil {
		t.Error("多一個指向不存在方法的宣告卻通過了檢查")
	}
	for name, d := range map[string]declaration{
		"兩者皆無":   {},
		"兩者皆有":   {capability: seller.CapChatRead, exemptReason: "x"},
		"不存在的能力": requires("CAP_NOPE"),
	} {
		bad := map[string]declaration{}
		for k, v := range declarations {
			bad[k] = v
		}
		bad["/hawkr.chat.v1alpha1.ChatAppService/SendMessage"] = d
		if err := checkDeclarations(bad, svc); err == nil {
			t.Errorf("%s 的宣告通過了檢查", name)
		}
	}
}

// 能力宣告與模組實際檢查的一致：店家送出要 CapChatSend，讀取類要 CapChatRead，買家專屬的方法全部豁免。
func TestDeclaredCapabilitiesMatchWhatTheModuleChecks(t *testing.T) {
	want := map[string]seller.Capability{
		"ListStoreConversations": seller.CapChatRead, "GetStoreConversationByBuyer": seller.CapChatRead, "MarkAllStoreRead": seller.CapChatRead, "GetStoreUnreadCount": seller.CapChatRead,
		"GetStorePresence": seller.CapChatRead, "StreamStoreEvents": seller.CapChatRead, "ListMessages": seller.CapChatRead, "MarkConversationRead": seller.CapChatRead,
		"SendMessageToBuyer": seller.CapChatSend, "SendMessage": seller.CapChatSend,
	}
	for procedure, d := range declarations {
		name := procedure[strings.LastIndex(procedure, "/")+1:]
		if c, ok := want[name]; ok {
			if d.capability != c {
				t.Errorf("%s 宣告 %q，預期 %q", name, d.capability, c)
			}
		} else if d.exemptReason == "" {
			t.Errorf("%s 是買家專屬，應該豁免", name)
		}
	}
}

func TestConstructorsRequireTheirDependencies(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	if _, err := NewApp(&fakeReads{}, &fakeWrites{}, &fakeStreams{}, &fakeAuth{}, &fakeBuyers{}, &fakeStores{}, images(t), nil); err == nil {
		t.Error("需要 logger")
	}
	if _, err := NewApp(&fakeReads{}, &fakeWrites{}, &fakeStreams{}, &fakeAuth{}, nil, &fakeStores{}, images(t), log); err == nil {
		t.Error("需要 Buyers")
	}
	if _, err := NewApp(&fakeReads{}, &fakeWrites{}, &fakeStreams{}, &fakeAuth{}, &fakeBuyers{}, nil, images(t), log); err == nil {
		t.Error("需要 Stores")
	}
	if _, err := NewApp(&fakeReads{}, &fakeWrites{}, nil, &fakeAuth{}, &fakeBuyers{}, &fakeStores{}, images(t), log); err == nil {
		t.Error("需要 Streams")
	}
	if _, err := NewApp(&fakeReads{}, &fakeWrites{}, &fakeStreams{}, nil, &fakeBuyers{}, &fakeStores{}, images(t), log); err == nil {
		t.Error("需要 Reauthenticator")
	}
}

func TestGetStoreConversationByBuyerDerivesTheStoreFromTheCallerAndAssemblesTheBuyer(t *testing.T) {
	e := newEnv(t, ownStore())
	name := "阿美"
	e.buyers.displays = []buyer.Display{{BuyerID: 5, DisplayName: &name}}
	e.reads.conversation = sampleConversation(1, callerStoreID, 5)
	resp, err := e.app.GetStoreConversationByBuyer(signedIn(), connect.NewRequest(&pb.GetStoreConversationByBuyerRequest{BuyerId: "5"}))
	if err != nil {
		t.Fatal(err)
	}
	assertStoreParty(t, e.reads.party)
	if e.reads.byBuyer != 5 {
		t.Errorf("buyer = %d", e.reads.byBuyer)
	}
	if b := resp.Msg.GetConversation().GetBuyer(); b.GetId() != "5" || b.GetName() != "阿美" || b.GetDeleted() || resp.Msg.GetConversation().Store != nil {
		t.Errorf("買家：%v", resp.Msg)
	}
}

func TestGetStoreConversationByBuyerMarksADeletedBuyer(t *testing.T) {
	e := newEnv(t, ownStore())
	e.buyers.displays = []buyer.Display{{BuyerID: 6, Deleted: true}}
	e.reads.conversation = sampleConversation(1, callerStoreID, 6)
	resp, err := e.app.GetStoreConversationByBuyer(signedIn(), connect.NewRequest(&pb.GetStoreConversationByBuyerRequest{BuyerId: "6"}))
	if err != nil {
		t.Fatal(err)
	}
	if b := resp.Msg.GetConversation().GetBuyer(); !b.GetDeleted() || b.Name != nil || b.Avatar != nil {
		t.Errorf("已刪除的買家：%v", b)
	}
}

func TestGetStoreConversationByBuyerValidatesTheBuyerIDAndMapsModuleNotFound(t *testing.T) {
	e := newEnv(t, ownStore())
	for _, bad := range []string{"", "abc", "0", "-3"} {
		_, err := e.app.GetStoreConversationByBuyer(signedIn(), connect.NewRequest(&pb.GetStoreConversationByBuyerRequest{BuyerId: bad}))
		if codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("buyer_id %q：%v", bad, err)
		}
	}
	if len(e.reads.calls) != 0 {
		t.Errorf("驗證失敗不得呼叫模組：%v", e.reads.calls)
	}
	e.reads.err = apperr.NotFound("沒有對話")
	if _, err := e.app.GetStoreConversationByBuyer(signedIn(), connect.NewRequest(&pb.GetStoreConversationByBuyerRequest{BuyerId: "5"})); codeOf(err) != connect.CodeNotFound {
		t.Errorf("沒有對話：%v", err)
	}
	req := func(e *env) error {
		_, err := e.app.GetStoreConversationByBuyer(signedIn(), connect.NewRequest(&pb.GetStoreConversationByBuyerRequest{BuyerId: "5"}))
		return err
	}
	noStore := newEnv(t, nil)
	closed := newEnv(t, ownStore())
	closed.reads.err = apperr.NotFound("店家已關閉的模組句")
	missing := newEnv(t, ownStore())
	missing.reads.err = apperr.NotFound("另一句")
	want := req(missing)
	for name, err := range map[string]error{"沒有店": req(noStore), "店家關閉": req(closed)} {
		if codeOf(err) != connect.CodeNotFound || err.Error() != want.Error() || reasonOf(err) != reasonOf(want) {
			t.Errorf("%s 必須與沒有對話相同：%v vs %v", name, err, want)
		}
	}
}
