package turn

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"google.golang.org/adk/v2/session"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func call(name string, args map[string]any) scriptedCall {
	return scriptedCall{Chunks: []chunk{{FunctionCall: &chunkCall{Name: name, Args: args}}}}
}

func say(s string) scriptedCall { return scriptedCall{Chunks: []chunk{{Text: s}}} }

func toolResults(evs []model.Event) []model.Event {
	var out []model.Event
	for _, ev := range evs {
		if ev.Kind == model.EventToolResult {
			out = append(out, ev)
		}
	}
	return out
}

func sampleOrders() *fakeOrders {
	return &fakeOrders{
		orders:  []buyeragent.Order{{OrderNo: "A1", Status: "30", SellerName: "店", Amount: 10}},
		details: map[string]buyeragent.OrderDetail{"A1": {OrderNo: "A1", Status: "30", Amount: 10}},
	}
}

// 每個 agent 的每支 tool 都至少被呼叫一次，而且每次都經過包裝層（有一個 tool_result 與一次觀察點呼叫）。
func TestEveryToolOfEveryAgentIsReportedThroughTheWrapper(t *testing.T) {
	cases := []struct {
		name   string
		topic  model.Topic
		script []scriptedCall
		want   []model.Tool
	}{
		{"orders specialist", model.TopicOrders, []scriptedCall{
			call("get_my_orders", map[string]any{"date_range": "all"}),
			call("get_buyer_order_detail", map[string]any{"order_no": "A1"}),
			say("好"),
		}, []model.Tool{model.ToolGetMyOrders, model.ToolGetBuyerOrderDetail}},
		{"tracking specialist", model.TopicTracking, []scriptedCall{call("track_my_package", nil), say("好")},
			[]model.Tool{model.ToolTrackMyPackage}},
		{"coordinator delegating to both specialists", model.TopicChat, []scriptedCall{
			call("buyer_order_specialist", map[string]any{"request": "查"}),
			call("get_my_orders", map[string]any{"date_range": "all"}),
			call("get_buyer_order_detail", map[string]any{"order_no": "A1"}),
			say("訂單好了"),
			call("buyer_tracking_specialist", map[string]any{"request": "追"}),
			call("track_my_package", nil),
			say("包裹好了"),
			say("彙整"),
		}, []model.Tool{model.ToolGetMyOrders, model.ToolGetBuyerOrderDetail, model.ToolTrackMyPackage}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var (
				mu       sync.Mutex
				observed []model.Tool
			)
			e := newExecutor(t, &scriptedLLM{script: c.script}, sampleOrders(), func(cfg *Config) {
				cfg.ToolObserver = func(tc buyeragent.ToolCall) {
					mu.Lock()
					observed = append(observed, tc.Tool)
					mu.Unlock()
				}
			})
			ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: c.topic, Message: "x"})
			if err != nil {
				t.Fatal(err)
			}
			evs := collect(t, e, buyer, ref.TurnID, "")
			if evs[len(evs)-1].Kind != model.EventFinished {
				t.Fatalf("要正常結束：%+v", evs)
			}
			var got []model.Tool
			for _, ev := range toolResults(evs) {
				if !ev.ToolSucceeded {
					t.Errorf("%v 應該成功", ev.Tool)
				}
				got = append(got, ev.Tool)
			}
			mu.Lock()
			defer mu.Unlock()
			if !slices.Equal(got, c.want) || !slices.Equal(observed, c.want) {
				t.Fatalf("tool_result %v、觀察點 %v，要 %v", got, observed, c.want)
			}
		})
	}
}

func TestToolArgumentValidationFailureIsAFailedToolResultAndTheModelSeesACode(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{
		call("get_my_orders", map[string]any{"date_range": "all", "page_size": "not-a-number"}),
		say("抱歉"),
	}}
	e := newExecutor(t, llm, sampleOrders(), nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "查"})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	tr := toolResults(evs)
	if len(tr) != 1 || tr[0].Tool != model.ToolGetMyOrders || tr[0].ToolSucceeded {
		t.Fatalf("參數驗證失敗要有一個失敗的 tool_result：%+v", evs)
	}
	resp, _ := llm.seen()[1].FunctionResponsesBack[0]["response"].(map[string]any)
	if resp["error"] != "tool_failed" || len(resp) != 1 {
		t.Fatalf("模型只看到固定代碼，看不到驗證訊息：%v", resp)
	}
	if evs[len(evs)-1].Kind != model.EventFinished {
		t.Fatalf("之後助理仍要回覆並結束：%+v", evs)
	}
}

func TestUnknownToolNameFromTheModelHasNoToolResultAndTheModelSeesACode(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{call("drop_all_tables", map[string]any{"x": 1}), say("好")}}
	e := newExecutor(t, llm, sampleOrders(), nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "查"})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	if n := len(toolResults(evs)); n != 0 {
		t.Fatalf("沒有對應 AssistantTool 的呼叫不得有 tool_result（UNSPECIFIED 不是合法值）：%+v", evs)
	}
	resp, _ := llm.seen()[1].FunctionResponsesBack[0]["response"].(map[string]any)
	if resp["error"] != "unknown_tool" || len(resp) != 1 {
		t.Fatalf("模型看到的要是代碼（ADK 預設文字會列出所有 tool 名稱）：%v", resp)
	}
	if evs[len(evs)-1].Kind != model.EventFinished {
		t.Fatalf("要正常結束：%+v", evs)
	}
}

func TestToolOutputsToTheModelAreLanguageNeutralCodes(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{
		call("get_my_orders", map[string]any{"date_range": "all"}),
		say("好"),
	}}
	e := newExecutor(t, llm, sampleOrders(), nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "查"})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, ref.TurnID, "")
	resp, _ := llm.seen()[1].FunctionResponsesBack[0]["response"].(map[string]any)
	orders, _ := resp["orders"].([]any)
	first, _ := orders[0].(map[string]any)
	if first["status"] != "30" {
		t.Fatalf("status 要是代碼：%v", first)
	}

	llm2 := &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("好")}}
	e2 := newExecutor(t, llm2, &fakeOrders{}, nil)
	ref2, err := e2.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicTracking, Message: "追"})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e2, buyer, ref2.TurnID, "")
	resp2, _ := llm2.seen()[1].FunctionResponsesBack[0]["response"].(map[string]any)
	if resp2["note"] != "no_packages_in_transit" {
		t.Fatalf("沒有包裹要回代碼：%v", resp2)
	}
}

func TestOrderNotFoundIsACodeAndTheDownstreamTextIsNotShown(t *testing.T) {
	llm := &scriptedLLM{script: []scriptedCall{call("get_buyer_order_detail", map[string]any{"order_no": "ZZ"}), say("好")}}
	orders := &fakeOrders{detailErrs: map[string]error{"ZZ": errors.Join(buyeragent.ErrOrderNotFound, errors.New("grpc code=404 host=order-service.internal"))}}
	e := newExecutor(t, llm, orders, nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicOrders, Message: "ZZ"})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, ref.TurnID, "")
	resp, _ := llm.seen()[1].FunctionResponsesBack[0]["response"].(map[string]any)
	if resp["error"] != "order_not_found" || len(resp) != 1 {
		t.Fatalf("查無訂單要回代碼：%v", resp)
	}
}

func TestAnotherBuyersSessionIsNotFound(t *testing.T) {
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, nil)
	mine, err := submitChat(t, e, "", "你好")
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, buyer, mine.TurnID, "")
	_, err = e.Submit(context.Background(), SubmitInput{BuyerID: buyer + 1, Topic: model.TopicChat, SessionID: mine.SessionID, Message: "偷看"})
	if !errors.Is(err, model.ErrSessionNotFound) {
		t.Fatalf("別的買家的 session = %v，要 ErrSessionNotFound", err)
	}
	e.mu.Lock()
	n := len(e.active)
	e.mu.Unlock()
	if n != 0 {
		t.Fatalf("被拒絕的 Submit 沒有釋放佔用：%d", n)
	}
}

// flakySessions 的 Create 失敗一次。
type flakySessions struct {
	session.Service
	mu   sync.Mutex
	fail bool
}

func (f *flakySessions) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	f.mu.Lock()
	failNow := f.fail
	f.fail = false
	f.mu.Unlock()
	if failNow {
		return nil, errors.New("store down")
	}
	return f.Service.Create(ctx, req)
}

func TestFailedSessionWriteReleasesTheGuardAndTheWaitGroup(t *testing.T) {
	fs := &flakySessions{Service: session.InMemoryService(), fail: true}
	e := newExecutor(t, textLLM("嗨"), &fakeOrders{}, func(c *Config) { c.Sessions = fs })
	if _, err := submitChat(t, e, "", "你好"); !errors.Is(err, model.ErrSessionCreate) {
		t.Fatalf("Create 失敗 = %v", err)
	}
	e.mu.Lock()
	n := len(e.active)
	e.mu.Unlock()
	if n != 0 {
		t.Fatalf("失敗後還有 %d 個佔用", n)
	}
	// wg 也要放掉：否則 Shutdown 會卡到逾時。
	ctx, cancel := context.WithTimeout(context.Background(), 5_000_000_000)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown 卡住，wg 沒有放掉：%v", err)
	}
}

func TestReturnPolicyTextFollowsTheRequestLocale(t *testing.T) {
	e := newExecutor(t, textLLM("不該被呼叫"), &fakeOrders{}, nil)
	for _, loc := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
		ctx := uitext.WithLocale(context.Background(), loc)
		ref, err := e.Submit(ctx, SubmitInput{BuyerID: buyer, Topic: model.TopicReturnPolicy})
		if err != nil {
			t.Fatal(err)
		}
		evs := collect(t, e, buyer, ref.TurnID, "")
		want := text.ReturnPolicyTitle(loc).Text + "\n\n" + text.ReturnPolicyFacts(loc).Text + "\n\n" + text.ReturnPolicyClosing(loc).Text
		if evs[0].Text != want {
			t.Fatalf("%v 的退貨政策文字不符：%q", loc, evs[0].Text)
		}
	}
	zh := text.ReturnPolicyFacts(uitext.ZhTW).Text
	en := text.ReturnPolicyFacts(uitext.EN).Text
	if zh == en || strings.ContainsAny(en, "退貨") {
		t.Fatal("en 與 zh-TW 的退貨政策要是不同的文字")
	}
}
