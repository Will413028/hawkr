package assistant

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/assistant/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/assistant/v1alpha1/assistantv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

const (
	reqID = "11111111-1111-4111-8111-111111111111"
	// accountID 與 buyerID 刻意不同：handler 要經買家對應取 buyer_id，不能把帳號 ID 當買家 ID。
	accountID = int64(7)
	buyerID   = int64(4242)
)

type buyerLookup struct{}

func (buyerLookup) CurrentBuyerID(_ context.Context, account int64) (int64, error) {
	switch account {
	case accountID:
		return buyerID, nil
	case accountID + 1:
		return buyerID + 1, nil
	}
	return 0, fmt.Errorf("沒有買家")
}

// fakeModule 是 Module 的替身；Watch 的擁有者檢查與真實的輪次執行器相同：不是自己的輪次回 ErrTurnNotFound。
type fakeModule struct {
	mu        sync.Mutex
	submitted []assistant.SubmitTurnInput
	submitErr error
	turns     map[string]int64
	events    []assistant.TurnEvent
	watchErr  error
	history   assistant.History
	historyBy []int64
	live      chan assistant.TurnEvent // 非 nil 時 Watch 阻塞等它
}

func (f *fakeModule) SubmitTurn(_ context.Context, in assistant.SubmitTurnInput) (assistant.TurnRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submitted = append(f.submitted, in)
	if f.submitErr != nil {
		return assistant.TurnRef{}, f.submitErr
	}
	return assistant.TurnRef{TurnID: "turn-1", SessionID: "sess-1"}, nil
}

func (f *fakeModule) WatchTurn(ctx context.Context, buyer int64, turnID, cursor string) (iter.Seq2[assistant.TurnEvent, error], error) {
	if f.watchErr != nil {
		return nil, f.watchErr
	}
	if owner, ok := f.turns[turnID]; !ok || owner != buyer {
		return nil, assistant.ErrTurnNotFound
	}
	return func(yield func(assistant.TurnEvent, error) bool) {
		for _, ev := range f.events {
			if !yield(ev, nil) {
				return
			}
		}
		if f.live == nil {
			return
		}
		for {
			select {
			case ev := <-f.live:
				if !yield(ev, nil) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}, nil
}

func (f *fakeModule) LatestSession(_ context.Context, buyer int64) (assistant.History, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.historyBy = append(f.historyBy, buyer)
	return f.history, nil
}

func setup(t *testing.T, m Module, window time.Duration) (assistantv1alpha1connect.AssistantAppServiceClient, struct{}) {
	t.Helper()
	app := NewApp(m, buyerLookup{}, slog.New(slog.DiscardHandler)).WithWatchWindow(window)
	path, h := assistantv1alpha1connect.NewAssistantAppServiceHandler(app, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Account") != "" {
			acct, _ := strconv.ParseInt(r.Header.Get("X-Test-Account"), 10, 64)
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: acct}))
		}
		h.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return assistantv1alpha1connect.NewAssistantAppServiceClient(srv.Client(), srv.URL), struct{}{}
}

func asAccount[T any](req *connect.Request[T], account int64) *connect.Request[T] {
	req.Header().Set("X-Test-Account", strconv.FormatInt(account, 10))
	return req
}

func send(topic pb.AssistantTopic, msg string) *pb.SendAssistantMessageRequest {
	return &pb.SendAssistantMessageRequest{Topic: topic, Message: msg, ClientRequestId: reqID}
}

func reasonOf(t *testing.T, err error) string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatal(derr)
		}
		if info, ok := v.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}

func wantCode(t *testing.T, err error, code connect.Code, reason string) {
	t.Helper()
	if err == nil {
		t.Fatalf("沒有錯誤，要 %s %q", code, reason)
	}
	if got := connect.CodeOf(err); got != code {
		t.Fatalf("code = %s，要 %s（%v）", got, code, err)
	}
	if got := reasonOf(t, err); got != reason {
		t.Fatalf("reason = %q，要 %q（%v）", got, reason, err)
	}
}

func TestSendValidatesTheRequestAtTheEntry(t *testing.T) {
	m := &fakeModule{}
	c, _ := setup(t, m, time.Second)
	ctx := t.Context()
	for name, req := range map[string]*pb.SendAssistantMessageRequest{
		"空訊息":                         send(pb.AssistantTopic_ASSISTANT_TOPIC_ORDERS, ""),
		"空訊息（退貨政策）":                   send(pb.AssistantTopic_ASSISTANT_TOPIC_RETURN_POLICY, ""),
		"未指定主題":                       send(pb.AssistantTopic_ASSISTANT_TOPIC_UNSPECIFIED, "嗨"),
		"沒有 client_request_id":        {Topic: pb.AssistantTopic_ASSISTANT_TOPIC_CHAT, Message: "嗨"},
		"client_request_id 不是 UUIDv4": {Topic: pb.AssistantTopic_ASSISTANT_TOPIC_CHAT, Message: "嗨", ClientRequestId: "abc"},
	} {
		_, err := c.SendAssistantMessage(ctx, asAccount(connect.NewRequest(req), accountID))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，要 INVALID_ARGUMENT", name, err)
		}
	}
	if len(m.submitted) != 0 {
		t.Fatalf("被拒絕的請求不得到達模組：%+v", m.submitted)
	}
}

func TestSendPassesTheBuyerNotTheAccountAndReturnsTheTurn(t *testing.T) {
	m := &fakeModule{}
	c, _ := setup(t, m, time.Second)
	img := "https://example.test/a.png"
	req := send(pb.AssistantTopic_ASSISTANT_TOPIC_TRACKING, "我的包裹")
	req.SessionId = "sess-1"
	req.ImageUrl = &img
	resp, err := c.SendAssistantMessage(t.Context(), asAccount(connect.NewRequest(req), accountID))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetTurnId() != "turn-1" || resp.Msg.GetSessionId() != "sess-1" {
		t.Fatalf("response = %v", resp.Msg)
	}
	got := m.submitted[0]
	if got.BuyerID != buyerID || got.Topic != assistant.TopicTracking || got.SessionID != "sess-1" || got.Message != "我的包裹" || got.ImageURL != img || got.RequestID != reqID {
		t.Fatalf("SubmitTurnInput = %+v", got)
	}
}

func TestSendMapsModuleErrors(t *testing.T) {
	for _, c := range []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{assistant.ErrTurnInProgress, connect.CodeFailedPrecondition, "ASSISTANT_TURN_IN_PROGRESS"},
		{assistant.ErrSessionNotFound, connect.CodeNotFound, ""},
		{assistant.ErrShuttingDown, connect.CodeUnavailable, ""},
		{assistant.ErrIdempotencyMismatch, connect.CodeInvalidArgument, ""},
		{assistant.ErrMessageRequired, connect.CodeInvalidArgument, ""},
		{fmt.Errorf("%w: boom", assistant.ErrSessionCreate), connect.CodeInternal, ""},
		{assistant.ErrOuterTx, connect.CodeInternal, ""},
	} {
		m := &fakeModule{submitErr: c.err}
		cl, _ := setup(t, m, time.Second)
		_, err := cl.SendAssistantMessage(t.Context(), asAccount(connect.NewRequest(send(pb.AssistantTopic_ASSISTANT_TOPIC_CHAT, "嗨")), accountID))
		t.Run(c.err.Error(), func(t *testing.T) { wantCode(t, err, c.code, c.reason) })
	}
}

func TestRequestsWithoutAnAccountAreRejected(t *testing.T) {
	c, _ := setup(t, &fakeModule{}, time.Second)
	_, err := c.SendAssistantMessage(t.Context(), connect.NewRequest(send(pb.AssistantTopic_ASSISTANT_TOPIC_CHAT, "嗨")))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("未登入 = %v", err)
	}
}

func drain(t *testing.T, c assistantv1alpha1connect.AssistantAppServiceClient, req *pb.WatchAssistantTurnRequest, account int64) ([]*pb.WatchAssistantTurnResponse, error) {
	t.Helper()
	stream, err := c.WatchAssistantTurn(t.Context(), asAccount(connect.NewRequest(req), account))
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()
	var out []*pb.WatchAssistantTurnResponse
	for stream.Receive() {
		out = append(out, stream.Msg())
	}
	return out, stream.Err()
}

func TestWatchConvertsEveryEventKind(t *testing.T) {
	m := &fakeModule{turns: map[string]int64{"turn-1": buyerID}, events: []assistant.TurnEvent{
		{Cursor: "1", Kind: assistant.EventText, Text: "你"},
		{Cursor: "2", Kind: assistant.EventToolResult, Tool: assistant.ToolTrackMyPackage, ToolSucceeded: true},
		{Cursor: "3", Kind: assistant.EventToolResult, Tool: assistant.ToolGetMyOrders},
		{Cursor: "4", Kind: assistant.EventFinished, SessionID: "sess-1"},
	}}
	c, _ := setup(t, m, time.Second)
	got, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: "turn-1"}, accountID)
	if err != nil || len(got) != 4 {
		t.Fatalf("events = %v, %v", got, err)
	}
	if got[0].GetCursor() != "1" || got[0].GetText().GetText() != "你" {
		t.Fatalf("text = %v", got[0])
	}
	if tr := got[1].GetToolResult(); tr.GetTool() != pb.AssistantTool_ASSISTANT_TOOL_TRACK_MY_PACKAGE || !tr.GetSucceeded() {
		t.Fatalf("tool_result = %v", got[1])
	}
	if tr := got[2].GetToolResult(); tr.GetTool() != pb.AssistantTool_ASSISTANT_TOOL_GET_MY_ORDERS || tr.GetSucceeded() {
		t.Fatalf("tool_result = %v", got[2])
	}
	if got[3].GetFinished().GetSessionId() != "sess-1" || got[3].GetCursor() != "4" {
		t.Fatalf("finished = %v", got[3])
	}
}

// error 事件的 reason 一律是 ASSISTANT_TURN_FAILED，FailureKind 的細分不外流。
func TestWatchErrorEventCarriesTheTurnFailedReason(t *testing.T) {
	for _, kind := range []assistant.FailureKind{assistant.FailureNoOutput, assistant.FailureInterrupted} {
		m := &fakeModule{turns: map[string]int64{"turn-1": buyerID}, events: []assistant.TurnEvent{
			{Cursor: "1", Kind: assistant.EventError, Failure: kind, Message: "查詢失敗，請稍後再試。"},
		}}
		c, _ := setup(t, m, time.Second)
		got, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: "turn-1"}, accountID)
		if err != nil || len(got) != 1 {
			t.Fatalf("events = %v, %v", got, err)
		}
		e := got[0].GetError()
		if e.GetReason() != commonpb.ErrorReason_ASSISTANT_TURN_FAILED || e.GetLocalizedMessage() != "查詢失敗，請稍後再試。" || len(e.GetMetadata()) != 0 {
			t.Fatalf("error 事件 = %v", e)
		}
	}
}

// 訂閱只能讀自己的輪次：別的買家、不存在、已清除的輪次一律 NOT_FOUND，且不洩漏它存在。
func TestWatchRefusesATurnThatIsNotTheCallers(t *testing.T) {
	m := &fakeModule{turns: map[string]int64{"turn-1": buyerID + 1}, events: []assistant.TurnEvent{{Cursor: "1", Kind: assistant.EventFinished}}}
	c, _ := setup(t, m, time.Second)
	_, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: "turn-1"}, accountID)
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: "evicted-or-never-existed"}, accountID)
	wantCode(t, err, connect.CodeNotFound, "")
}

func TestWatchCursorBeyondTheBufferReturnsReplayCursorAhead(t *testing.T) {
	m := &fakeModule{turns: map[string]int64{"turn-1": buyerID}, watchErr: assistant.ErrCursorAhead}
	c, _ := setup(t, m, time.Second)
	_, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: "turn-1", Cursor: "999"}, accountID)
	wantCode(t, err, connect.CodeFailedPrecondition, "REPLAY_CURSOR_AHEAD")
}

func TestWatchRequiresATurnID(t *testing.T) {
	c, _ := setup(t, &fakeModule{}, time.Second)
	_, err := drain(t, c, &pb.WatchAssistantTurnRequest{}, accountID)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("沒有 turn_id = %v", err)
	}
}

// 串流滿窗口就正常結束（沒有錯誤、沒有終止事件）：client 以最後的 cursor 接續。
func TestWatchEndsNormallyWhenTheWindowIsUp(t *testing.T) {
	m := &fakeModule{turns: map[string]int64{"turn-1": buyerID}, events: []assistant.TurnEvent{{Cursor: "1", Kind: assistant.EventText, Text: "嗨"}}, live: make(chan assistant.TurnEvent)}
	c, _ := setup(t, m, 150*time.Millisecond)
	start := time.Now()
	got, err := drain(t, c, &pb.WatchAssistantTurnRequest{TurnId: "turn-1"}, accountID)
	if err != nil || len(got) != 1 {
		t.Fatalf("events = %v, %v；窗口到期要正常結束", got, err)
	}
	if d := time.Since(start); d < 100*time.Millisecond || d > 5*time.Second {
		t.Fatalf("串流在 %v 結束，窗口是 150ms", d)
	}
}

func TestDefaultWatchWindowIs100Seconds(t *testing.T) {
	if WatchWindow != 100*time.Second {
		t.Fatalf("WatchWindow = %v", WatchWindow)
	}
	if NewApp(&fakeModule{}, buyerLookup{}, nil).window != WatchWindow {
		t.Fatal("預設窗口不是 WatchWindow")
	}
}

func TestGetLatestSessionUsesTheCallersBuyerAndConvertsMessages(t *testing.T) {
	at := time.Date(2026, 11, 20, 1, 2, 3, 0, time.UTC)
	m := &fakeModule{history: assistant.History{SessionID: "sess-1", Messages: []assistant.Message{
		{Role: assistant.RoleUser, Text: "嗨", Time: at},
		{Role: assistant.RoleAssistant, Text: "你好", Time: at.Add(time.Second)},
	}}}
	c, _ := setup(t, m, time.Second)
	resp, err := c.GetLatestAssistantSession(t.Context(), asAccount(connect.NewRequest(&pb.GetLatestAssistantSessionRequest{}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	r := resp.Msg
	if r.GetSessionId() != "sess-1" || len(r.GetMessages()) != 2 ||
		r.GetMessages()[0].GetRole() != pb.AssistantMessageRole_ASSISTANT_MESSAGE_ROLE_USER || r.GetMessages()[0].GetText() != "嗨" ||
		r.GetMessages()[1].GetRole() != pb.AssistantMessageRole_ASSISTANT_MESSAGE_ROLE_ASSISTANT || !r.GetMessages()[1].GetCreateTime().AsTime().Equal(at.Add(time.Second)) {
		t.Fatalf("response = %v", r)
	}
	if len(m.historyBy) != 1 || m.historyBy[0] != buyerID {
		t.Fatalf("讀歷史的買家 = %v，要 %d", m.historyBy, buyerID)
	}
}

func TestGetLatestSessionWithNoConversationIsEmpty(t *testing.T) {
	c, _ := setup(t, &fakeModule{}, time.Second)
	resp, err := c.GetLatestAssistantSession(t.Context(), asAccount(connect.NewRequest(&pb.GetLatestAssistantSessionRequest{}), accountID))
	if err != nil || resp.Msg.GetSessionId() != "" || len(resp.Msg.GetMessages()) != 0 {
		t.Fatalf("response = %v, %v", resp, err)
	}
}
