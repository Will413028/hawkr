package turn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
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

func trackResponse(t *testing.T, llm *scriptedLLM, orders *fakeOrders) (map[string]any, []model.Event) {
	t.Helper()
	e := newExecutor(t, llm, orders, nil)
	ref, err := e.Submit(context.Background(), SubmitInput{BuyerID: buyer, Topic: model.TopicTracking, Message: "追"})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, e, buyer, ref.TurnID, "")
	resp, _ := llm.seen()[1].FunctionResponsesBack[0]["response"].(map[string]any)
	return resp, evs
}

// 追蹤 tool 只選 CodeMeta OrderStatus 的 20、30、32、35、40、54；每個包裹帶自己的狀態碼（A9-D16 第 1 點）。
func TestTrackingSelectsExactlyTheInTransitFamilyOfStatusesAndEachPackageCarriesItsCode(t *testing.T) {
	orders := &fakeOrders{details: map[string]buyeragent.OrderDetail{}}
	for _, st := range []string{"0", "10", "20", "30", "32", "35", "40", "50", "54", "60", "70", "80"} {
		no := "O" + st
		orders.orders = append(orders.orders, buyeragent.Order{OrderNo: no, Status: st})
		orders.details[no] = buyeragent.OrderDetail{OrderNo: no, Status: st}
	}
	llm := &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("好")}}
	resp, _ := trackResponse(t, llm, orders)
	var got []string
	pkgs, _ := resp["packages"].([]any)
	for _, p := range pkgs {
		m, _ := p.(map[string]any)
		if m["order_no"] != "O"+m["status"].(string) {
			t.Fatalf("包裹的狀態碼要是它自己的：%v", m)
		}
		got = append(got, m["status"].(string))
	}
	slices.Sort(got)
	if want := []string{"20", "30", "32", "35", "40", "54"}; !slices.Equal(got, want) {
		t.Fatalf("追蹤的狀態 = %v，要 %v", got, want)
	}
	if _, has := resp["note"]; has {
		t.Fatalf("全部詳情都查到時不帶 note：%v", resp)
	}
}

func TestTrackingFailsWithUpstreamUnavailableWhenEveryDetailLookupFails(t *testing.T) {
	orders := &fakeOrders{
		orders:     []buyeragent.Order{{OrderNo: "A", Status: "30"}, {OrderNo: "B", Status: "20"}},
		detailErrs: map[string]error{"A": errors.New("timeout"), "B": errors.New("timeout")},
	}
	llm := &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("抱歉")}}
	resp, evs := trackResponse(t, llm, orders)
	if resp["error"] != "upstream_unavailable" || len(resp) != 1 {
		t.Fatalf("全部詳情失敗要回 upstream_unavailable：%v", resp)
	}
	tr := toolResults(evs)
	if len(tr) != 1 || tr[0].Tool != model.ToolTrackMyPackage || tr[0].ToolSucceeded {
		t.Fatalf("要有一個失敗的 tool_result：%+v", evs)
	}
}

func TestTrackingSucceedsWithANoteCodeWhenSomePackageDetailsCannotBeLoaded(t *testing.T) {
	orders := &fakeOrders{
		orders:     []buyeragent.Order{{OrderNo: "A", Status: "30"}, {OrderNo: "B", Status: "20"}},
		details:    map[string]buyeragent.OrderDetail{"A": {OrderNo: "A", Status: "30"}},
		detailErrs: map[string]error{"B": errors.New("timeout")},
	}
	llm := &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("好")}}
	resp, evs := trackResponse(t, llm, orders)
	pkgs, _ := resp["packages"].([]any)
	if resp["note"] != "some_packages_unavailable" || len(pkgs) != 1 {
		t.Fatalf("部分失敗要成功並帶 note 代碼：%v", resp)
	}
	if tr := toolResults(evs); len(tr) != 1 || !tr[0].ToolSucceeded {
		t.Fatalf("部分失敗的 tool_result 要是成功：%+v", evs)
	}
}

// 全部失敗一律 upstream_unavailable，不看哪一筆最後失敗；順序兩種都測。已不存在的訂單不改變結論。
func TestTrackingAllFailingIsUpstreamUnavailableWhateverTheCausesAndTheirOrder(t *testing.T) {
	gone := fmt.Errorf("adapter：%w", buyeragent.ErrOrderNotFound)
	boom := errors.New("timeout")
	for name, errs := range map[string][2]error{"notfound-last": {boom, gone}, "notfound-first": {gone, boom}, "all-gone": {gone, gone}} {
		t.Run(name, func(t *testing.T) {
			orders := &fakeOrders{
				orders:     []buyeragent.Order{{OrderNo: "A", Status: "30"}, {OrderNo: "B", Status: "20"}},
				detailErrs: map[string]error{"A": errs[0], "B": errs[1]},
			}
			llm := &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("抱歉")}}
			resp, _ := trackResponse(t, llm, orders)
			if resp["error"] != "upstream_unavailable" || len(resp) != 1 {
				t.Fatalf("要回 upstream_unavailable：%v", resp)
			}
		})
	}
}

// 已不存在的訂單靜默略過，不觸發 some_packages_unavailable；上游錯誤才觸發。
func TestTrackingDropsGoneOrdersSilentlyButNotesUpstreamFailures(t *testing.T) {
	base := func(errB error) *fakeOrders {
		return &fakeOrders{
			orders:     []buyeragent.Order{{OrderNo: "A", Status: "30"}, {OrderNo: "B", Status: "20"}},
			details:    map[string]buyeragent.OrderDetail{"A": {OrderNo: "A", Status: "30"}},
			detailErrs: map[string]error{"B": errB},
		}
	}
	llm := &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("好")}}
	resp, _ := trackResponse(t, llm, base(fmt.Errorf("adapter：%w", buyeragent.ErrOrderNotFound)))
	if pkgs, _ := resp["packages"].([]any); len(pkgs) != 1 {
		t.Fatalf("package 要剩 1 筆：%v", resp)
	}
	if _, has := resp["note"]; has {
		t.Fatalf("只有已不存在的訂單時不帶 note：%v", resp)
	}
	llm = &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("好")}}
	resp, _ = trackResponse(t, llm, base(errors.New("timeout")))
	if resp["note"] != "some_packages_unavailable" {
		t.Fatalf("上游錯誤要帶 note：%v", resp)
	}
}

// 滿頁（等於一次查的筆數）時帶 more_packages_not_shown；不滿頁不帶；兩種但書可同時出現。
func TestTrackingNotesWhenThePageIsFull(t *testing.T) {
	mk := func(n int, failLast bool) *fakeOrders {
		o := &fakeOrders{details: map[string]buyeragent.OrderDetail{}, detailErrs: map[string]error{}}
		for i := range n {
			no := fmt.Sprintf("O%d", i)
			o.orders = append(o.orders, buyeragent.Order{OrderNo: no, Status: "30"})
			o.details[no] = buyeragent.OrderDetail{OrderNo: no, Status: "30"}
		}
		if failLast {
			o.detailErrs[fmt.Sprintf("O%d", n-1)] = errors.New("timeout")
		}
		return o
	}
	cases := []struct {
		n        int
		failLast bool
		want     any
	}{{10, false, "more_packages_not_shown"}, {9, false, nil}, {10, true, "some_packages_unavailable,more_packages_not_shown"}}
	for _, c := range cases {
		llm := &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("好")}}
		resp, _ := trackResponse(t, llm, mk(c.n, c.failLast))
		if resp["note"] != c.want {
			t.Fatalf("n=%d failLast=%v：note = %v，要 %v", c.n, c.failLast, resp["note"], c.want)
		}
	}
}

// 日誌方案：全部失敗只由 guardedTool 記一次（合併的原因，含訂單號）；部分失敗逐筆記（含訂單號與 tool 名稱）。
func TestTrackingLogsEachUpstreamCauseExactlyOnceWithOrderNoAndToolName(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	all := &fakeOrders{
		orders:     []buyeragent.Order{{OrderNo: "A1", Status: "30"}, {OrderNo: "B2", Status: "20"}},
		detailErrs: map[string]error{"A1": errors.New("causeA"), "B2": errors.New("causeB")},
	}
	llm := &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("抱歉")}}
	trackResponse(t, llm, all)
	if n := strings.Count(buf.String(), "causeA"); n != 1 {
		t.Fatalf("全部失敗時每個原因只記一次，causeA 出現 %d 次：\n%s", n, buf.String())
	}
	if !strings.Contains(buf.String(), "A1") || !strings.Contains(buf.String(), "track_my_package") {
		t.Fatalf("log 要含訂單號與 tool 名稱：\n%s", buf.String())
	}

	buf.Reset()
	part := &fakeOrders{
		orders:     []buyeragent.Order{{OrderNo: "A1", Status: "30"}, {OrderNo: "B2", Status: "20"}},
		details:    map[string]buyeragent.OrderDetail{"A1": {OrderNo: "A1", Status: "30"}},
		detailErrs: map[string]error{"B2": errors.New("causeB")},
	}
	llm = &scriptedLLM{script: []scriptedCall{call("track_my_package", nil), say("好")}}
	trackResponse(t, llm, part)
	if n := strings.Count(buf.String(), "causeB"); n != 1 || !strings.Contains(buf.String(), "B2") || !strings.Contains(buf.String(), "track_my_package") {
		t.Fatalf("部分失敗要逐筆記一次（含訂單號與 tool 名稱），causeB %d 次：\n%s", n, buf.String())
	}
}
