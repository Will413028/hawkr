package turn

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 這支測試鎖的是舊 ai-service 買家助理「現在的行為」（錄下來的 fixture），不是判斷那些行為是否正確。
// 舊路徑只送 text／done／error 三種事件；新串流多了 tool_result，且 error 是終止事件，所以比對規則是：
//   - text：種類、順序、每段文字完全相同。
//   - done → finished（帶 session ID）；error 之後舊路徑還會送 done，新串流 error 之後沒有事件，只比到 error。
//   - error：code 500 的兩句文案對應 FailureNoOutput（查詢失敗）與 FailureInterrupted（回應中斷），文字相同。
//   - tool_result：舊路徑沒有，tool 身分取自 scripted_llm 的 function_call（限三支 tool，agenttool 的委派不算），
//     成功與否取自 observed_llm_requests 裡回給 LLM 的 function response 有沒有 error；
//     所有 fixture 的對外文字都在 tool 呼叫之後，所以預期序列是 tool_result 在前、舊事件在後，實際序列不同就失敗。
//     事件不帶參數：tool 名稱與參數改由 Config.ToolObserver 觀察到的呼叫與 scripted_llm 的 function_call 逐一比對。
//   - 刻意偏離：回給 LLM 的 tool 結果不再帶中文狀態標籤與下游錯誤文字，改為狀態代碼與錯誤代碼；
//     追蹤 tool 的查詢狀態集合改為 20、30、32、35、40、54，單筆詳情失敗改為回 note 代碼；見 fedBackDeviations、fedBackSubstitutions 與 statusRemap，每一筆有理由與明確的替換。

const fixtureDir = "../../../../../docs/plans/2026-11-19-a9-agent/fixtures/assistant"

type fixture struct {
	Scenario string `json:"scenario"`
	Input    struct {
		RPC      string `json:"rpc"`
		Message  string `json:"message"`
		ImageURL string `json:"image_url"`
		BuyerID  int64  `json:"buyer_id"`
	} `json:"input"`
	Backend struct {
		Orders      []fixtureOrder           `json:"orders"`
		OrdersError string                   `json:"orders_error"`
		Details     map[string]fixtureDetail `json:"details"`
		DetailError map[string]string        `json:"detail_error_for"`
	} `json:"fake_tool_backend"`
	Script   []scriptedCall    `json:"scripted_llm"`
	Observed []json.RawMessage `json:"observed_llm_requests"`
	Emitted  []struct {
		Kind      string `json:"kind"`
		Text      string `json:"text"`
		SessionID string `json:"session_id"`
		Code      int    `json:"code"`
		Message   string `json:"message"`
	} `json:"emitted_events"`
	RPC struct {
		StatusCode string `json:"status_code"`
	} `json:"rpc_result"`
}

type fixtureOrder struct {
	OrderNo      string
	Status       string
	SellerName   string
	Amount       int64
	CreateTimeMs int64
	ItemCount    int64
}

type fixtureDetail struct {
	OrderNo        string
	Status         string
	SellerName     string
	ShippingMethod string
	ShippingStatus string
	RecipientName  string
	PaymentMethod  string
	Amount         int64
	CreateTimeMs   int64
	ShippingFee    int64
}

type observed struct {
	Agent                 string           `json:"agent"`
	CallIndex             int              `json:"call_index"`
	LastUserTexts         []string         `json:"last_user_texts"`
	FileURIs              []string         `json:"file_uris"`
	FunctionResponsesBack []map[string]any `json:"function_responses_fed_back"`
	HistoryContents       int              `json:"history_contents"`
}

// skippedFixtures 是驗 RPC 回應、不是輪次事件的 fixture（GetLatestAssistantSession，在 internal/history 比對）。
var skippedFixtures = map[string]string{
	"latest_session_after_chat": "BuyerLatestSession 是 GetLatestAssistantSession（讀 session 歷史），不在輪次執行器；由 internal/history 的測試比對這份 fixture",
	"latest_session_empty":      "BuyerLatestSession 是 GetLatestAssistantSession（讀 session 歷史），不在輪次執行器；由 internal/history 的測試比對這份 fixture",
}

// recordedDefaultMessages 是舊服務按鈕端點在空訊息時代填的預設訊息（orders_happy_path、tracking_partial_detail_failure 的錄製以它送出）。
// 新服務不再有預設訊息：入口層對空訊息回 INVALID_ARGUMENT，這兩份 fixture 在測試裡補回當時的文字，輪次與 LLM 請求的比對維持不變。
// 退貨政策不看訊息，不在表內。
var recordedDefaultMessages = map[string]string{
	"BuyerOrders":   "查詢我的訂單",
	"BuyerTracking": "查詢包裹狀態",
}

var toolIDs = map[string]model.Tool{
	"get_my_orders":          model.ToolGetMyOrders,
	"get_buyer_order_detail": model.ToolGetBuyerOrderDetail,
	"track_my_package":       model.ToolTrackMyPackage,
}

// fedBackDeviations 列出回給 LLM 的 tool 結果刻意與錄製不同的 fixture及理由；
// 其餘 fixture 必須逐字相同。每一筆偏離在 fedBackSubstitutions 有明確的替換，比對時對錄製值套用替換後必須與實際相同。
var fedBackDeviations = map[string]string{
	"orders_happy_path":                  "status 由中文標籤改為狀態代碼（A9-D15 第 2 點）：「待出貨（物流單已建立，待出貨）」→ 20、「配送中」→ 30",
	"llm_error_after_tool_call":          "同 orders_happy_path：status 由標籤改為代碼",
	"order_detail_via_coordinator":       "同 orders_happy_path：訂單詳情的 status 由標籤改為代碼",
	"tool_error_assistant_still_replies": "tool 錯誤不再把下游錯誤文字（「查詢訂單失敗: …」）給模型，改為代碼 upstream_unavailable（A9-D15 第 3 點）",
	"tracking_no_packages":               "「目前沒有配送中的包裹」中文句 message 改為代碼 note=no_packages_in_transit；packages 仍是 null（nil slice 的 JSON）",
	"tracking_partial_detail_failure":    "status 由標籤改為代碼，且單筆詳情失敗不再靜默略過：回應多一個 note=some_packages_unavailable（A9-D16 第 1 點）",
}

var statusLabelToCode = map[string]string{
	"待出貨（物流單已建立，待出貨）": "20",
	"配送中": "30",
}

// replaceStatusLabels 遞迴把 key 為 status 的中文標籤換成代碼。
func replaceStatusLabels(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			if str, ok := e.(string); ok && k == "status" {
				if code, ok := statusLabelToCode[str]; ok {
					out[k] = code
					continue
				}
			}
			out[k] = replaceStatusLabels(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = replaceStatusLabels(e)
		}
		return out
	default:
		return v
	}
}

// withNote 刪掉舊的 message、加上 note 代碼。
func withNote(note string) func(any) any {
	return func(v any) any {
		m := v.(map[string]any)
		out := map[string]any{}
		for k, e := range m {
			if k != "message" {
				out[k] = e
			}
		}
		out["note"] = note
		return out
	}
}

// fedBackSubstitutions 是每筆偏離的明確替換：作用在每個 tool 的 function response 的 response 值上。
var fedBackSubstitutions = map[string]func(any) any{
	"orders_happy_path":                  replaceStatusLabels,
	"llm_error_after_tool_call":          replaceStatusLabels,
	"order_detail_via_coordinator":       replaceStatusLabels,
	"tool_error_assistant_still_replies": func(any) any { return map[string]any{"error": "upstream_unavailable"} },
	"tracking_no_packages":               withNote("no_packages_in_transit"),
	"tracking_partial_detail_failure": func(v any) any {
		return withNote("some_packages_unavailable")(replaceStatusLabels(v))
	},
}

// substituteFedBack 對錄製的 function response 套用該 fixture 的替換；coordinator 委派的回應（response 是 specialist 的文字）不受影響。
func substituteFedBack(scenario string, recorded []map[string]any) []map[string]any {
	sub, ok := fedBackSubstitutions[scenario]
	if !ok {
		return recorded
	}
	out := make([]map[string]any, len(recorded))
	for i, fr := range recorded {
		cp := map[string]any{}
		for k, v := range fr {
			cp[k] = v
		}
		name, _ := fr["name"].(string)
		if toolIDs[name] != 0 {
			cp["response"] = sub(fr["response"])
		}
		out[i] = cp
	}
	return out
}

// statusRemap 是追蹤 tool 查詢狀態改變帶來的 fixture 輸入調整：舊 tool 只查狀態 20，新 tool 查 20、30、32、35、40、54；
// 為了讓被選中的訂單與原錄製相同，fixture 裡原本不會被選中的狀態 30 改成不在追蹤集合的 50，其餘輸入不動。
var statusRemap = map[string]string{
	"tracking_no_packages":            "舊 tool 只選狀態 20：fixture 的 30（新集合會選中）改為 50，被選中的訂單與原錄製相同",
	"tracking_partial_detail_failure": "同 tracking_no_packages",
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(fixtureDir, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("找不到 fixture：%v", err)
	}
	var out []fixture
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var f fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatalf("%s：%v", p, err)
		}
		out = append(out, f)
	}
	return out
}

func (f fixture) observedRequests(t *testing.T) []observed {
	t.Helper()
	var out []observed
	for _, raw := range f.Observed {
		var o observed
		if err := json.Unmarshal(raw, &o); err != nil {
			t.Fatal(err)
		}
		out = append(out, o)
	}
	return out
}

func (f fixture) orders() *fakeOrders {
	o := &fakeOrders{details: map[string]buyeragent.OrderDetail{}, detailErrs: map[string]error{}}
	if f.Backend.OrdersError != "" {
		o.ordersErr = errors.New(f.Backend.OrdersError)
	}
	remap := func(status string) string {
		if _, ok := statusRemap[f.Scenario]; ok {
			if status == "30" {
				return "50"
			}
		}
		return status
	}
	for _, x := range f.Backend.Orders {
		x.Status = remap(x.Status)
		o.orders = append(o.orders, buyeragent.Order{OrderNo: x.OrderNo, Status: x.Status, SellerName: x.SellerName, Amount: x.Amount, CreateTime: time.UnixMilli(x.CreateTimeMs), ItemCount: x.ItemCount})
	}
	for k, d := range f.Backend.Details {
		d.Status = remap(d.Status)
		o.details[k] = buyeragent.OrderDetail{
			OrderNo: d.OrderNo, Status: d.Status, SellerName: d.SellerName, ShippingMethod: d.ShippingMethod, ShippingStatus: d.ShippingStatus,
			RecipientName: d.RecipientName, PaymentMethod: d.PaymentMethod, Amount: d.Amount, CreateTime: time.UnixMilli(d.CreateTimeMs), ShippingFee: d.ShippingFee,
		}
	}
	for k, msg := range f.Backend.DetailError {
		o.detailErrs[k] = errors.New(msg)
	}
	return o
}

func topicOf(t *testing.T, rpc string) model.Topic {
	t.Helper()
	switch rpc {
	case "BuyerChat":
		return model.TopicChat
	case "BuyerOrders":
		return model.TopicOrders
	case "BuyerTracking":
		return model.TopicTracking
	case "BuyerReturnPolicy":
		return model.TopicReturnPolicy
	}
	t.Fatalf("未知的 rpc %q", rpc)
	return 0
}

// expectedEvents 把舊事件序列翻成新串流的事件（不含 cursor）。sessionID 取代 fixture 的 <SESSION_ID>。
func (f fixture) expectedEvents(sessionID string) []model.Event {
	var tools []model.Event
	failedTool := map[string]bool{}
	for _, raw := range f.Observed {
		var o observed
		_ = json.Unmarshal(raw, &o)
		for _, fr := range o.FunctionResponsesBack {
			name, _ := fr["name"].(string)
			if resp, ok := fr["response"].(map[string]any); ok {
				if _, hasErr := resp["error"]; hasErr && toolIDs[name] != 0 {
					failedTool[name] = true
				}
			}
		}
	}
	for _, c := range f.Script {
		for _, ch := range c.Chunks {
			if ch.FunctionCall == nil || toolIDs[ch.FunctionCall.Name] == 0 {
				continue
			}
			tools = append(tools, model.Event{Kind: model.EventToolResult, Tool: toolIDs[ch.FunctionCall.Name], ToolSucceeded: !failedTool[ch.FunctionCall.Name]})
		}
	}
	out := tools
	for _, e := range f.Emitted {
		switch e.Kind {
		case "text":
			out = append(out, model.Event{Kind: model.EventText, Text: e.Text})
		case "error":
			ev := model.Event{Kind: model.EventError, Message: e.Message}
			switch e.Message {
			case "查詢失敗，請稍後再試。":
				ev.Failure = model.FailureNoOutput
			case "回應中斷，請稍後再試。":
				ev.Failure = model.FailureInterrupted
			}
			out = append(out, ev)
			return out // 新串流 error 即終止；舊路徑接著的 done 不比
		case "done":
			sid := e.SessionID
			if sid == "<SESSION_ID>" {
				sid = sessionID
			}
			out = append(out, model.Event{Kind: model.EventFinished, SessionID: sid})
		}
	}
	return out
}

func TestCharacterizationBuyerAssistantMatchesRecordedOldBehavior(t *testing.T) {
	for _, f := range loadFixtures(t) {
		t.Run(f.Scenario, func(t *testing.T) {
			if reason, skip := skippedFixtures[f.Scenario]; skip {
				t.Skip(reason)
			}
			llm := &scriptedLLM{script: f.Script}
			orders := f.orders()
			var (
				obsMu sync.Mutex
				calls []buyeragent.ToolCall
			)
			e := newExecutor(t, llm, orders, func(c *Config) {
				c.ToolObserver = func(tc buyeragent.ToolCall) {
					obsMu.Lock()
					calls = append(calls, tc)
					obsMu.Unlock()
				}
				if f.Scenario == "session_create_failure" {
					c.Sessions = failingSessions{c.Sessions}
				}
			})
			ctx := uitext.WithLocale(context.Background(), uitext.ZhTW)
			message := f.Input.Message
			if message == "" {
				message = recordedDefaultMessages[f.Input.RPC]
			}
			ref, err := e.Submit(ctx, SubmitInput{BuyerID: f.Input.BuyerID, Topic: topicOf(t, f.Input.RPC), Message: message, ImageURL: f.Input.ImageURL})

			switch f.RPC.StatusCode {
			case "InvalidArgument":
				if !errors.Is(err, model.ErrMessageRequired) {
					t.Fatalf("Submit 錯誤 = %v，要 ErrMessageRequired", err)
				}
				if n := len(llm.seen()); n != 0 {
					t.Fatalf("被拒絕的訊息不該呼叫 LLM，呼叫了 %d 次", n)
				}
				return
			case "Internal":
				if !errors.Is(err, model.ErrSessionCreate) {
					t.Fatalf("Submit 錯誤 = %v，要 ErrSessionCreate", err)
				}
				if n := len(llm.seen()); n != 0 {
					t.Fatalf("建立 session 失敗不該呼叫 LLM，呼叫了 %d 次", n)
				}
				return
			}
			if err != nil {
				t.Fatalf("Submit：%v", err)
			}

			got := collect(t, e, f.Input.BuyerID, ref.TurnID, "")
			for i := range got {
				got[i].Cursor = ""
			}
			want := f.expectedEvents(ref.SessionID)
			if !reflect.DeepEqual(got, want) {
				gj, _ := json.MarshalIndent(got, "", " ")
				wj, _ := json.MarshalIndent(want, "", " ")
				t.Fatalf("事件序列不同\n got: %s\nwant: %s", gj, wj)
			}
			// tool 名稱與參數：由觀察點與錄製的 function_call 逐一比對（事件不帶參數）。
			obsMu.Lock()
			gotCalls := append([]buyeragent.ToolCall(nil), calls...)
			obsMu.Unlock()
			var wantCalls []buyeragent.ToolCall
			for _, sc := range f.Script {
				for _, ch := range sc.Chunks {
					if ch.FunctionCall != nil && toolIDs[ch.FunctionCall.Name] != 0 {
						var args map[string]any
						if len(ch.FunctionCall.Args) > 0 {
							args = ch.FunctionCall.Args
						}
						wantCalls = append(wantCalls, buyeragent.ToolCall{Tool: toolIDs[ch.FunctionCall.Name], Name: ch.FunctionCall.Name, Args: args})
					}
				}
			}
			if len(gotCalls) != len(wantCalls) {
				t.Fatalf("tool 呼叫 %d 次，錄製是 %d 次：%+v", len(gotCalls), len(wantCalls), gotCalls)
			}
			for i := range gotCalls {
				g, w := gotCalls[i], wantCalls[i]
				if g.Tool != w.Tool || g.Name != w.Name || !reflect.DeepEqual(g.Args, w.Args) {
					t.Errorf("第 %d 次 tool 呼叫 = %+v，錄製是 %+v", i, g, w)
				}
			}
			if f.Input.RPC != "BuyerReturnPolicy" && ref.SessionID == "" {
				t.Fatal("非退貨政策的輪次要有 session ID")
			}

			// 每個 fixture 的 text 串接結果（錄製時的「增量串接」）。
			var gotText, wantText strings.Builder
			for _, ev := range got {
				gotText.WriteString(ev.Text)
			}
			for _, ev := range want {
				wantText.WriteString(ev.Text)
			}
			if gotText.String() != wantText.String() {
				t.Fatalf("文字串接結果不同：%q vs %q", gotText.String(), wantText.String())
			}

			// 送給 LLM 的請求：路由到哪個 agent、使用者文字、圖片、回給 LLM 的 tool 結果、歷史長度。
			seen := llm.seen()
			obs := f.observedRequests(t)
			if len(seen) != len(f.Script) {
				t.Fatalf("LLM 被呼叫 %d 次，錄製是 %d 次", len(seen), len(f.Script))
			}
			if len(seen) != len(obs) {
				t.Fatalf("LLM 請求 %d 筆，錄製的觀察是 %d 筆", len(seen), len(obs))
			}
			for i, o := range obs {
				s := seen[i]
				if s.Agent != o.Agent {
					t.Errorf("第 %d 次呼叫的 agent = %s，錄製是 %s", i, s.Agent, o.Agent)
				}
				if !reflect.DeepEqual(s.LastUserTexts, o.LastUserTexts) {
					t.Errorf("第 %d 次呼叫的使用者文字 = %v，錄製是 %v", i, s.LastUserTexts, o.LastUserTexts)
				}
				if !reflect.DeepEqual(s.FileURIs, o.FileURIs) {
					t.Errorf("第 %d 次呼叫的檔案 = %v，錄製是 %v", i, s.FileURIs, o.FileURIs)
				}
				if s.HistoryContents != o.HistoryContents {
					t.Errorf("第 %d 次呼叫的歷史長度 = %d，錄製是 %d", i, s.HistoryContents, o.HistoryContents)
				}
				gj, _ := json.Marshal(s.FunctionResponsesBack)
				wj, _ := json.Marshal(substituteFedBack(f.Scenario, o.FunctionResponsesBack))
				if string(gj) != string(wj) && (len(s.FunctionResponsesBack) != 0 || len(o.FunctionResponsesBack) != 0) {
					t.Errorf("第 %d 次呼叫回給 LLM 的 tool 結果不同（已套用 fedBackSubstitutions）\n got: %s\nwant: %s", i, gj, wj)
				}
			}
		})
	}
}

func TestCharacterizationReturnPolicyTextIsTheSharedCatalogText(t *testing.T) {
	for _, f := range loadFixtures(t) {
		if f.Scenario != "return_policy_fixed_text" {
			continue
		}
		want := f.Emitted[0].Text
		got := text.ReturnPolicyTitle(uitext.ZhTW).Text + "\n\n" + text.ReturnPolicyFacts(uitext.ZhTW).Text + "\n\n" + text.ReturnPolicyClosing(uitext.ZhTW).Text
		if got != want {
			t.Fatalf("退貨政策文字與舊固定文案不同\n got: %q\nwant: %q", got, want)
		}
		return
	}
	t.Fatal("找不到 return_policy_fixed_text")
}

// 每一筆偏離都要有理由與明確的替換，反之亦然。
func TestEveryFedBackDeviationHasAReasonAndASubstitution(t *testing.T) {
	for name, reason := range fedBackDeviations {
		if reason == "" || fedBackSubstitutions[name] == nil {
			t.Errorf("%s：偏離要有理由與替換", name)
		}
	}
	for name := range fedBackSubstitutions {
		if fedBackDeviations[name] == "" {
			t.Errorf("%s：有替換但沒有理由", name)
		}
	}
}
