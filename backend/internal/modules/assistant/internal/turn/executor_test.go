package turn

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

const buyer = int64(4242)

// gatedLLM 在第一次被呼叫時通知 started，然後等 release 關閉才回答；之後的呼叫不再等。
type gatedLLM struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func newGatedLLM() *gatedLLM {
	return &gatedLLM{started: make(chan struct{}, 16), release: make(chan struct{})}
}

func (g *gatedLLM) Name() string { return "gated" }

func (g *gatedLLM) GenerateContent(ctx context.Context, _ *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		g.calls.Add(1)
		g.started <- struct{}{}
		select {
		case <-g.release:
		case <-ctx.Done():
			yield(nil, ctx.Err())
			return
		}
		yield(&adkmodel.LLMResponse{Content: genai.NewContentFromText("好的", genai.RoleModel), FinishReason: genai.FinishReasonStop}, nil)
	}
}

func submitChat(t *testing.T, e *Executor, sessionID, msg string) (Ref, error) {
	t.Helper()
	return e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicChat, SessionID: sessionID, Message: msg})
}

func TestSecondSubmitOnSameSessionIsRejectedWhileATurnIsInFlight(t *testing.T) {
	g := newGatedLLM()
	e := newExecutor(t, g, &fakeOrders{}, nil)

	first, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	<-g.started // 第一輪的 LLM 呼叫已開始，對話被佔用

	if _, err := submitChat(t, e, first.SessionID, "再問一次"); !errors.Is(err, model.ErrTurnInProgress) {
		t.Fatalf("進行中再送出 = %v，要 ErrTurnInProgress", err)
	}
	// 不同的對話不受影響。
	other, err := submitChat(t, e, "", "另一個對話")
	if err != nil {
		t.Fatalf("另一個對話被擋：%v", err)
	}
	<-g.started

	close(g.release)
	for _, r := range []Ref{first, other} {
		evs := collect(t, e, buyer, r.TurnID, "")
		if last := evs[len(evs)-1]; last.Kind != model.EventFinished {
			t.Fatalf("結束事件 = %+v", last)
		}
	}
	// 看到結束事件就代表佔用已釋放：同一個對話可以開下一輪。
	next, err := submitChat(t, e, first.SessionID, "第二輪")
	if err != nil {
		t.Fatalf("結束後再送出：%v", err)
	}
	collect(t, e, buyer, next.TurnID, "")
}

func TestGuardIsReleasedWhenTheAgentPanics(t *testing.T) {
	e := newExecutor(t, panicLLM{}, &fakeOrders{}, nil)
	first, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, first.TurnID, "")
	if last := evs[len(evs)-1]; last.Kind != model.EventError || last.Failure != model.FailureNoOutput {
		t.Fatalf("panic 的輪次要以 FailureNoOutput 的 error 結束，得到 %+v", last)
	}
	if _, err := submitChat(t, e, first.SessionID, "再試"); err != nil {
		t.Fatalf("panic 後對話沒有釋放：%v", err)
	}
}

type panicLLM struct{}

func (panicLLM) Name() string { return "panic" }
func (panicLLM) GenerateContent(context.Context, *adkmodel.LLMRequest, bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	panic("boom")
}

func TestGuardIsReleasedWhenTheTurnContextIsCancelled(t *testing.T) {
	g := newGatedLLM() // release 永不關閉：只有取消能讓它結束
	e := newExecutor(t, g, &fakeOrders{}, nil)
	first, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	<-g.started
	if err := e.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, first.TurnID, "")
	if last := evs[len(evs)-1]; last.Kind != model.EventError {
		t.Fatalf("被取消的輪次要以 error 結束，得到 %+v", last)
	}
	e.mu.Lock()
	n := len(e.active)
	e.mu.Unlock()
	if n != 0 {
		t.Fatalf("取消後還有 %d 個對話被佔用", n)
	}
}

func TestToolErrorIsReportedAsAFailedToolResultAndTheTurnStillFinishes(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{
		{Chunks: []chunk{{FunctionCall: &chunkCall{Name: "get_my_orders", Args: map[string]any{"date_range": "all"}}}}},
		{Chunks: []chunk{{Text: "抱歉，查不到。"}}},
	}}
	orders := &fakeOrders{ordersErr: errors.New("order service unavailable")}
	e := newExecutor(t, llm, orders, nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "查我的訂單"})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if len(evs) != 3 || evs[0].Kind != model.EventToolResult || evs[0].ToolSucceeded || evs[0].Tool != model.ToolGetMyOrders {
		t.Fatalf("要先有一個 failed 的 tool_result：%+v", evs)
	}
	if evs[1].Kind != model.EventText || evs[2].Kind != model.EventFinished {
		t.Fatalf("tool 失敗後助理仍要回覆並正常結束：%+v", evs)
	}
	fed := llm.seen()[1].FunctionResponsesBack
	if len(fed) != 1 {
		t.Fatalf("回給 LLM 的 tool 結果 = %v", fed)
	}
	// 模型只看到不帶語言的代碼，看不到下游的錯誤文字。
	if resp, _ := fed[0]["response"].(map[string]any); resp["error"] != "upstream_unavailable" || len(resp) != 1 {
		t.Fatalf("tool 的錯誤要以代碼回給 LLM：%v", fed)
	}
}

// tamperedSessions 把新建 session 的 buyer_id 改成別人，模擬 session state 與這一輪的買家不一致。
type tamperedSessions struct{ session.Service }

func (s tamperedSessions) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	req.State = map[string]any{buyeragent.StateKeyBuyer: "999"}
	return s.Service.Create(ctx, req)
}

func TestToolsRefuseASessionWhoseBuyerIsNotTheTurnsBuyer(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{
		{Chunks: []chunk{{FunctionCall: &chunkCall{Name: "get_my_orders", Args: map[string]any{"date_range": "all"}}}}},
		{Chunks: []chunk{{Text: "好"}}},
	}}
	orders := &fakeOrders{orders: []buyeragent.Order{{OrderNo: "X"}}}
	e := newExecutor(t, llm, orders, func(c *Config) { c.Sessions = tamperedSessions{c.Sessions} })
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "查"})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if evs[0].Kind != model.EventToolResult || evs[0].ToolSucceeded {
		t.Fatalf("身分範圍不符的 tool 要失敗：%+v", evs[0])
	}
	if resp, _ := llm.seen()[1].FunctionResponsesBack[0]["response"].(map[string]any); resp["error"] != "forbidden" {
		t.Fatalf("身分範圍不符要回代碼 forbidden：%v", resp)
	}
	if orders.listCalls != 0 {
		t.Fatalf("身分範圍不符時不可呼叫訂單服務，呼叫了 %d 次", orders.listCalls)
	}
}

func TestUnknownSessionIsRejectedInsteadOfAutoCreated(t *testing.T) {
	e := newExecutor(t, textLLM("x"), &fakeOrders{}, nil)
	if _, err := submitChat(t, e, "no-such-session", "你好"); !errors.Is(err, model.ErrSessionNotFound) {
		t.Fatalf("未知的 session = %v，要 ErrSessionNotFound", err)
	}
}

func TestSecondTurnOfASessionSeesTheFirstTurnsHistory(t *testing.T) {
	llm := textLLM("嗨")
	e := newExecutor(t, llm, &fakeOrders{}, nil)
	first, err := submitChat(t, e, "", "一")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, first.TurnID, "")
	second, err := submitChat(t, e, first.SessionID, "二")
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionID != first.SessionID {
		t.Fatalf("session ID 變了：%s → %s", first.SessionID, second.SessionID)
	}
	collect(t, e, buyer, second.TurnID, "")
	seen := llm.seen()
	if got := seen[len(seen)-1].HistoryContents; got != 3 {
		t.Fatalf("第二輪的 LLM 請求歷史 = %d 則，要 3（使用者一、助理一、使用者二）", got)
	}
}

func TestSubmitValidation(t *testing.T) {
	e := newExecutor(t, textLLM("x"), &fakeOrders{}, nil)
	ctx := context.Background()
	if _, err := e.Submit(ctx, SubmitInput{BuyerID: buyer, Topic: 0, Message: "x"}); !errors.Is(err, model.ErrInvalidTopic) {
		t.Fatalf("未指定主題 = %v", err)
	}
	if _, err := e.Submit(ctx, SubmitInput{BuyerID: buyer, Topic: model.Topic(99), Message: "x"}); !errors.Is(err, model.ErrInvalidTopic) {
		t.Fatalf("超出範圍的主題 = %v", err)
	}
	if _, err := e.Submit(ctx, SubmitInput{BuyerID: buyer, Topic: model.TopicChat}); !errors.Is(err, model.ErrMessageRequired) {
		t.Fatalf("一般問答空訊息 = %v", err)
	}
}

func TestEventsCarryTheBuyersTurnOnly(t *testing.T) {
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, nil)
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, ref.TurnID, "")
	if _, err := e.Watch(context.Background(), buyer+1, ref.TurnID, ""); !errors.Is(err, model.ErrTurnNotFound) {
		t.Fatalf("別的買家訂閱 = %v，要 ErrTurnNotFound", err)
	}
	if _, err := e.Watch(context.Background(), buyer, "no-such-turn", ""); !errors.Is(err, model.ErrTurnNotFound) {
		t.Fatalf("未知的輪次 = %v，要 ErrTurnNotFound", err)
	}
}

// manyPartials 讓一輪產生 n 個 text 事件。
func manyPartials(n int) *scriptedLLM {
	var chunks []chunk
	full := ""
	for i := range n {
		piece := fmt.Sprintf("%d,", i)
		full += piece
		chunks = append(chunks, chunk{Partial: true, Text: piece})
	}
	chunks = append(chunks, chunk{Text: full})
	return &scriptedLLM{script: []scriptedCall{{Chunks: chunks}}}
}

func TestWatchReplaysFromACursor(t *testing.T) {
	e := newExecutor(t, manyPartials(5), &fakeOrders{}, nil)
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	all := collect(t, e, buyer, ref.TurnID, "")
	if len(all) != 6 { // 5 個 text 加 finished
		t.Fatalf("事件數 = %d", len(all))
	}
	for i, ev := range all {
		if ev.Cursor != strconv.Itoa(i+1) {
			t.Fatalf("第 %d 個事件的 cursor = %q", i, ev.Cursor)
		}
	}
	rest := collect(t, e, buyer, ref.TurnID, all[2].Cursor)
	if len(rest) != 3 || rest[0].Text != all[3].Text || rest[2].Kind != model.EventFinished {
		t.Fatalf("從第 3 個事件之後接續：%+v", rest)
	}
	// 從最後一個事件之後接續：沒有新事件，也沒有錯誤；一輪已結束，迭代器立刻結束。
	seq, err := e.Watch(context.Background(), buyer, ref.TurnID, all[5].Cursor)
	if err != nil {
		t.Fatal(err)
	}
	for ev, err := range seq {
		t.Fatalf("終止事件之後不該再有事件：%+v %v", ev, err)
	}
}

func TestWatchCursorAhead(t *testing.T) {
	e := newExecutor(t, manyPartials(10), &fakeOrders{}, nil)
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	e.wg.Wait() // 這一輪已結束，終止事件已寫入
	// 共 11 個事件；緩衝不覆蓋舊事件，任何 0 到 11 的 cursor 都接得上。
	for _, c := range []string{"", "0", "6", "11"} {
		if _, err := e.Watch(context.Background(), buyer, ref.TurnID, c); err != nil {
			t.Fatalf("cursor %q = %v", c, err)
		}
	}
	if evs := collect(t, e, buyer, ref.TurnID, "7"); len(evs) != 4 {
		t.Fatalf("cursor 7 要接到 4 個事件：%+v", evs)
	}
	for _, c := range []string{"12", "999", "not-a-cursor", "-1", "+3"} {
		if _, err := e.Watch(context.Background(), buyer, ref.TurnID, c); !errors.Is(err, model.ErrCursorAhead) {
			t.Fatalf("cursor %q = %v，要 ErrCursorAhead", c, err)
		}
	}
}

func TestATurnThatOverflowsTheBufferEndsWithAnErrorAndReleasesTheSession(t *testing.T) {
	// 上限 4：前 3 格給一般事件，第 4 格留給終止事件。10 個 text 增量寫不下。
	e := newExecutor(t, manyPartials(10), &fakeOrders{}, func(c *Config) { c.BufferCapacity = 4 })
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if len(evs) != 4 {
		t.Fatalf("事件數 = %d，要正好是上限 4：%+v", len(evs), evs)
	}
	for i, ev := range evs[:3] {
		if ev.Kind != model.EventText || ev.Text != strconv.Itoa(i)+"," {
			t.Fatalf("第 %d 個事件被改動（不得覆蓋）：%+v", i, ev)
		}
	}
	if last := evs[3]; last.Kind != model.EventError || last.Failure != model.FailureInterrupted {
		t.Fatalf("寫滿後要以 interrupted 的 error 結束：%+v", last)
	}
	// 終止事件之後佔用已釋放。
	e.wg.Wait()
	next, err := submitChat(t, e, ref.SessionID, "再問")
	if err != nil {
		t.Fatalf("緩衝寫滿結束後對話沒有釋放：%v", err)
	}
	collect(t, e, buyer, next.TurnID, "")
}

func TestBufferNeverOverwritesAndKeepsTheLastSlotForTheTerminalEvent(t *testing.T) {
	b := newBuffer(3)
	now := time.Now()
	if !b.append(model.Event{Kind: model.EventText, Text: "a"}, now) || !b.append(model.Event{Kind: model.EventText, Text: "b"}, now) {
		t.Fatal("前兩個一般事件要寫得進去")
	}
	if b.append(model.Event{Kind: model.EventText, Text: "c"}, now) {
		t.Fatal("一般事件不得佔用保留給終止事件的最後一格")
	}
	if !b.append(model.Event{Kind: model.EventError}, now) {
		t.Fatal("終止事件永遠寫得進去")
	}
	if b.append(model.Event{Kind: model.EventText, Text: "d"}, now) {
		t.Fatal("終止之後不再寫入")
	}
	evs, _, finished := b.next(0)
	if !finished && len(evs) != 3 || evs[0].Text != "a" || evs[1].Text != "b" || evs[2].Kind != model.EventError {
		t.Fatalf("事件 = %+v", evs)
	}
}

func TestWatchEndsQuietlyWhenItsContextEnds(t *testing.T) {
	g := newGatedLLM()
	e := newExecutor(t, g, &fakeOrders{}, nil)
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	<-g.started
	ctx, cancel := context.WithCancel(context.Background())
	seq, err := e.Watch(ctx, buyer, ref.TurnID, "")
	if err != nil {
		t.Fatal(err)
	}
	cancel() // 呼叫者的期限到了（例如 100 秒）：迭代器正常結束，輪次不受影響
	for ev, err := range seq {
		t.Fatalf("不該有事件或錯誤：%+v %v", ev, err)
	}
	close(g.release)
	evs := collect(t, e, buyer, ref.TurnID, "")
	if evs[len(evs)-1].Kind != model.EventFinished {
		t.Fatalf("訂閱結束不該影響輪次：%+v", evs)
	}
}

func TestReturnPolicyDoesNotCallTheLLMOrCreateASession(t *testing.T) {
	llm := textLLM("不該被呼叫")
	sessions := &countingSessions{Service: session.InMemoryService()}
	e := newExecutor(t, llm, &fakeOrders{}, func(c *Config) { c.Sessions = sessions })
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicReturnPolicy})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if len(evs) != 2 || evs[0].Kind != model.EventText || evs[1].Kind != model.EventFinished {
		t.Fatalf("退貨政策事件 = %+v", evs)
	}
	if n := len(llm.seen()); n != 0 || sessions.creates.Load() != 0 {
		t.Fatalf("LLM 呼叫 %d 次、建立 session %d 次，都要 0", n, sessions.creates.Load())
	}
}

type countingSessions struct {
	session.Service
	creates atomic.Int32
}

func (c *countingSessions) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	c.creates.Add(1)
	return c.Service.Create(ctx, req)
}
