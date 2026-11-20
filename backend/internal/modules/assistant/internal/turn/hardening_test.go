package turn

import (
	"context"
	"errors"
	"iter"
	"sync"
	"testing"
	"time"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

// quietLLM 等到 ctx 結束就停止迭代，不回錯誤：模擬一個遇到取消時安靜收工的模型。
type quietLLM struct{ started chan struct{} }

func (quietLLM) Name() string { return "quiet" }

func (l quietLLM) GenerateContent(ctx context.Context, _ *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(func(*adkmodel.LLMResponse, error) bool) {
		l.started <- struct{}{}
		<-ctx.Done()
	}
}

func TestCancelledTurnWhoseModelStopsQuietlyEndsAsAnError(t *testing.T) {
	l := quietLLM{started: make(chan struct{}, 1)}
	e := newExecutor(t, l, &fakeOrders{}, nil)
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	<-l.started
	if err := e.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if last := evs[len(evs)-1]; last.Kind != model.EventError || last.Failure != model.FailureNoOutput {
		t.Fatalf("被取消的輪次要以 error 結束，得到 %+v", last)
	}
}

// partialThenQuietLLM 先回一段 partial 文字，再等到 ctx 結束就停止迭代、不回錯誤：事件流在沒有最終回應時正常結束。
type partialThenQuietLLM struct{ started chan struct{} }

func (partialThenQuietLLM) Name() string { return "partial-quiet" }

func (l partialThenQuietLLM) GenerateContent(ctx context.Context, _ *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		if !yield(&adkmodel.LLMResponse{Partial: true, Content: genai.NewContentFromText("半句", genai.RoleModel)}, nil) {
			return
		}
		l.started <- struct{}{}
		<-ctx.Done()
	}
}

func TestStreamThatEndsWithoutTheFinalResponseIsNotFinished(t *testing.T) {
	l := partialThenQuietLLM{started: make(chan struct{}, 1)}
	e := newExecutor(t, l, &fakeOrders{}, nil)
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	<-l.started
	// 半句文字已進緩衝，才取消：否則「已送出部分文字」要看排程。
	seq, err := e.Watch(context.Background(), buyer, ref.TurnID, "")
	if err != nil {
		t.Fatal(err)
	}
	for ev, err := range seq {
		if err != nil {
			t.Fatal(err)
		}
		if ev.Kind == model.EventText {
			break
		}
	}
	if err := e.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if last := evs[len(evs)-1]; last.Kind != model.EventError || last.Failure != model.FailureInterrupted {
		t.Fatalf("被取消、已送出部分文字的輪次要以 interrupted 結束：%+v", evs)
	}
}

// emptyLLM 的回應串流沒有任何事件就正常結束，也沒有取消。
type emptyLLM struct{}

func (emptyLLM) Name() string { return "empty" }

func (emptyLLM) GenerateContent(context.Context, *adkmodel.LLMRequest, bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(func(*adkmodel.LLMResponse, error) bool) {}
}

func TestStreamEndingNormallyWithoutAnyFinalResponseIsNotFinished(t *testing.T) {
	e := newExecutor(t, emptyLLM{}, &fakeOrders{}, nil)
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if last := evs[len(evs)-1]; last.Kind != model.EventError || last.Failure != model.FailureNoOutput {
		t.Fatalf("迭代正常結束但沒有最終回應，要以 error 結束，不得是 finished：%+v", evs)
	}
}

func TestSubmitAfterShutdownIsRejectedBeforeAnySessionIsCreated(t *testing.T) {
	cs := &countingSessions{Service: session.InMemoryService()}
	e := newExecutor(t, textLLM("hi"), &fakeOrders{}, func(c *Config) { c.Sessions = cs })
	if err := e.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, topic := range []model.Topic{model.TopicChat, model.TopicOrders, model.TopicReturnPolicy} {
		if _, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: topic, Message: "你好"}); !errors.Is(err, model.ErrShuttingDown) {
			t.Fatalf("%v：Shutdown 後 Submit = %v，要 ErrShuttingDown", topic, err)
		}
	}
	if n := cs.creates.Load(); n != 0 {
		t.Fatalf("Shutdown 後建立了 %d 個 session", n)
	}
	e.mu.Lock()
	n := len(e.turns)
	e.mu.Unlock()
	if n != 0 {
		t.Fatalf("Shutdown 後登記了 %d 輪", n)
	}
}

// fakeClock 是可調的時鐘。
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestRetentionSweepDropsEndedTurnsOnlyAfterTheRetentionWindow(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}
	const retention = 10 * time.Minute
	e := newExecutor(t, textLLM("好的"), &fakeOrders{}, func(c *Config) { c.Now = clock.now; c.Retention = retention })
	ref, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, ref.TurnID, "")

	clock.advance(retention) // 剛好等於保留期：還在
	if evs := collect(t, e, buyer, ref.TurnID, ""); len(evs) == 0 {
		t.Fatal("保留期內要還能訂閱")
	}

	clock.advance(time.Nanosecond)
	if _, err := e.Watch(context.Background(), buyer, ref.TurnID, ""); !errors.Is(err, model.ErrTurnNotFound) {
		t.Fatalf("過了保留期 Watch = %v，要 ErrTurnNotFound", err)
	}
	e.mu.Lock()
	_, kept := e.turns[ref.TurnID]
	e.mu.Unlock()
	if kept {
		t.Fatal("過期的輪次要從記憶體移除")
	}
}

func TestRetentionSweepAlsoRunsWhenANewTurnIsRegistered(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}
	e := newExecutor(t, textLLM("好的"), &fakeOrders{}, func(c *Config) { c.Now = clock.now; c.Retention = time.Minute })
	old, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, old.TurnID, "")
	clock.advance(2 * time.Minute)
	next, err := submitChat(t, e, "", "再來")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, next.TurnID, "")
	e.mu.Lock()
	_, kept := e.turns[old.TurnID]
	e.mu.Unlock()
	if kept {
		t.Fatal("登記新一輪時要順便清掉過期的輪次")
	}
}

type panicOrders struct{ fakeOrders }

func (*panicOrders) MyOrders(context.Context, int64, string, int, int) ([]buyeragent.Order, error) {
	panic("order client exploded")
}

func TestToolPanicIsReportedAsAFailedToolResult(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{
		{Chunks: []chunk{{FunctionCall: &chunkCall{Name: "get_my_orders", Args: map[string]any{"date_range": "all"}}}}},
		{Chunks: []chunk{{Text: "抱歉，查不到。"}}},
	}}
	e := newExecutor(t, llm, &panicOrders{}, nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "查我的訂單"})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if len(evs) < 2 || evs[0].Kind != model.EventToolResult || evs[0].ToolSucceeded {
		t.Fatalf("tool panic 要有一個 failed 的 tool_result：%+v", evs)
	}
	if evs[len(evs)-1].Kind != model.EventFinished {
		t.Fatalf("tool panic 後助理仍要回覆並結束：%+v", evs)
	}
}

func TestToolObserverSeesTheArgsTheModelSentAndTheEventCarriesNone(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{
		// page_size 0 是模型送的；解析成 struct 再序列化會因 omitempty 把它吃掉。
		{Chunks: []chunk{{FunctionCall: &chunkCall{Name: "get_my_orders", Args: map[string]any{"date_range": "all", "page_size": float64(0)}}}}},
		{Chunks: []chunk{{Text: "好"}}},
	}}
	var (
		mu    sync.Mutex
		calls []buyeragent.ToolCall
	)
	e := newExecutor(t, llm, &fakeOrders{}, func(c *Config) {
		c.ToolObserver = func(tc buyeragent.ToolCall) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, tc)
			tc.Args["date_range"] = "tampered" // 觀察點拿到的是拷貝：改它不影響 ADK
		}
	})
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "查我的訂單"})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 || calls[0].Name != "get_my_orders" || calls[0].Tool != model.ToolGetMyOrders || calls[0].Failed || len(calls[0].Args) != 2 || calls[0].Args["page_size"] != float64(0) {
		t.Fatalf("觀察到的呼叫：%+v", calls)
	}
	if want := (model.Event{Kind: model.EventToolResult, Cursor: "1", Tool: model.ToolGetMyOrders, ToolSucceeded: true}); evs[0] != want {
		t.Fatalf("tool_result 事件 = %+v，要 %+v（不帶參數）", evs[0], want)
	}
}

func TestOrderDetailResultIsWrappedInResult(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{
		{Chunks: []chunk{{FunctionCall: &chunkCall{Name: "get_buyer_order_detail", Args: map[string]any{"order_no": "A1"}}}}},
		{Chunks: []chunk{{Text: "好"}}},
	}}
	orders := &fakeOrders{details: map[string]buyeragent.OrderDetail{"A1": {OrderNo: "A1", Status: "30", Amount: 120}}}
	e := newExecutor(t, llm, orders, nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "A1"})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, ref.TurnID, "")
	fed := llm.seen()[1].FunctionResponsesBack
	resp, _ := fed[0]["response"].(map[string]any)
	inner, _ := resp["result"].(map[string]any)
	if inner["order_no"] != "A1" || inner["status"] != "30" || inner["amount_twd"] != float64(120) {
		t.Fatalf("回給 LLM 的結果要是 {result:{…}}：%v", fed)
	}
}
