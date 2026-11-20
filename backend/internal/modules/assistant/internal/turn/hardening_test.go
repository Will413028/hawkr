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
	if len(evs) < 2 || evs[0].Kind != model.EventToolResult || !evs[0].ToolFailed {
		t.Fatalf("tool panic 要有一個 failed 的 tool_result：%+v", evs)
	}
	if evs[len(evs)-1].Kind != model.EventFinished {
		t.Fatalf("tool panic 後助理仍要回覆並結束：%+v", evs)
	}
}

func TestToolArgsAreWhatTheModelSent(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{
		// page_size 0 是模型送的；解析成 struct 再序列化會因 omitempty 把它吃掉。
		{Chunks: []chunk{{FunctionCall: &chunkCall{Name: "get_my_orders", Args: map[string]any{"date_range": "all", "page_size": float64(0)}}}}},
		{Chunks: []chunk{{Text: "好"}}},
	}}
	e := newExecutor(t, llm, &fakeOrders{}, nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "查我的訂單"})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	args := evs[0].ToolArgs
	if evs[0].Kind != model.EventToolResult || len(args) != 2 || args["date_range"] != "all" || args["page_size"] != float64(0) {
		t.Fatalf("ToolArgs 要是模型送的原樣：%+v", evs[0])
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
	if inner["order_no"] != "A1" || inner["status"] != "配送中" || inner["amount_twd"] != float64(120) {
		t.Fatalf("回給 LLM 的結果要是 {result:{…}}：%v", fed)
	}
}
