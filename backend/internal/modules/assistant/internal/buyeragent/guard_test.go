package buyeragent

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

type fakeInner struct {
	name   string
	result map[string]any
	err    error
	boom   any
	gotArg any
}

func (f *fakeInner) Name() string                            { return f.name }
func (f *fakeInner) Description() string                     { return "" }
func (f *fakeInner) IsLongRunning() bool                     { return false }
func (f *fakeInner) Declaration() *genai.FunctionDeclaration { return nil }
func (f *fakeInner) Run(_ agent.Context, args any) (map[string]any, error) {
	f.gotArg = args
	if f.boom != nil {
		panic(f.boom)
	}
	return f.result, f.err
}

func guardOver(inner callable, out func(ToolCall)) *guardedTool {
	return &guardedTool{inner: inner, id: model.ToolGetMyOrders, out: out}
}

func TestGuardRecoversAPanicAndReportsAFailedCallWithAFixedCode(t *testing.T) {
	var calls []ToolCall
	g := guardOver(&fakeInner{name: "get_my_orders", boom: "stack secrets: 10.0.0.1"}, func(c ToolCall) { calls = append(calls, c) })
	res, err := g.Run(nil, map[string]any{"date_range": "all"})
	if res != nil || err == nil || err.Error() != codeToolFailed {
		t.Fatalf("panic 要變成固定代碼 %q：%v %v", codeToolFailed, res, err)
	}
	if len(calls) != 1 || !calls[0].Failed || calls[0].Tool != model.ToolGetMyOrders || calls[0].Args["date_range"] != "all" {
		t.Fatalf("panic 也要回報一次失敗的呼叫：%+v", calls)
	}
}

func TestGuardReportsEveryCallOnceAndHidesDownstreamErrorText(t *testing.T) {
	cases := []struct {
		name     string
		inner    *fakeInner
		wantCode string
		failed   bool
	}{
		{"success", &fakeInner{name: "get_my_orders", result: map[string]any{"ok": 1}}, "", false},
		{"untyped error, e.g. argument validation", &fakeInner{name: "get_my_orders", err: errors.New("invalid args: page_size: expected integer\nstack: /srv/x.go")}, codeToolFailed, true},
		{"typed error keeps its code, drops the cause", &fakeInner{name: "get_my_orders", err: toolFail(codeUpstreamUnavailable, errors.New("dial tcp 10.0.0.9: refused"))}, codeUpstreamUnavailable, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls []ToolCall
			g := guardOver(c.inner, func(tc ToolCall) { calls = append(calls, tc) })
			res, err := g.Run(nil, map[string]any{"page_size": "x"})
			if len(calls) != 1 || calls[0].Failed != c.failed {
				t.Fatalf("要回報一次、failed=%v：%+v", c.failed, calls)
			}
			if c.failed && (res != nil || err == nil || err.Error() != c.wantCode) {
				t.Fatalf("錯誤要只剩代碼 %q：%v %v", c.wantCode, res, err)
			}
			if !c.failed && (err != nil || res["ok"] != 1) {
				t.Fatalf("成功要原樣回：%v %v", res, err)
			}
		})
	}
}

func TestGuardGivesTheObserverACopyOfTheArgs(t *testing.T) {
	inner := &fakeInner{name: "get_my_orders", result: map[string]any{}}
	var seen ToolCall
	g := guardOver(inner, func(c ToolCall) { seen = c })
	args := map[string]any{"a": map[string]any{"b": []any{"x"}}}
	if _, err := g.Run(nil, args); err != nil {
		t.Fatal(err)
	}
	seen.Args["a"].(map[string]any)["b"].([]any)[0] = "tampered"
	if args["a"].(map[string]any)["b"].([]any)[0] != "x" {
		t.Fatal("觀察點改參數不得影響 ADK 的 args（要深拷貝）")
	}
}

func TestOnToolErrorMapsUnknownAndDelegateFailuresToCodes(t *testing.T) {
	cb := onToolError(map[string]bool{"known_agent_tool": true})
	res, err := cb(nil, namedTool("hallucinated"), nil, errors.New("tool 'hallucinated' not found. Available tools: get_my_orders"))
	if err != nil || res["error"] != codeUnknownTool || len(res) != 1 {
		t.Fatalf("未知 tool = %v %v", res, err)
	}
	res, err = cb(nil, namedTool("known_agent_tool"), nil, errors.New("child exploded: secret"))
	if err != nil || res["error"] != codeDelegateFailed || len(res) != 1 {
		t.Fatalf("委派失敗 = %v %v", res, err)
	}
	g := guardOver(&fakeInner{name: "get_my_orders"}, nil)
	if res, err := cb(nil, g, nil, toolFail(codeForbidden, nil)); res != nil || err == nil || err.Error() != codeForbidden {
		t.Fatalf("guardedTool 的錯誤原樣通過：%v %v", res, err)
	}
}

type namedTool string

func (n namedTool) Name() string      { return string(n) }
func (namedTool) Description() string { return "" }
func (namedTool) IsLongRunning() bool { return false }

// TestStatusGuideListsExactlyTheOrderStatusCodesOfTheAuthoritativeSource 守住系統提示的狀態碼表與 CodeMeta〈業務狀態碼〉的 OrderStatus 一致。
func TestStatusGuideListsExactlyTheOrderStatusCodesOfTheAuthoritativeSource(t *testing.T) {
	raw, err := os.ReadFile("../../../../../docs/codemeta/business-status-codes.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	start := strings.Index(doc, "## OrderStatus\n")
	if start < 0 {
		t.Fatal("找不到 OrderStatus")
	}
	section := doc[start+1:]
	if end := strings.Index(section, "\n---"); end > 0 {
		section = section[:end]
	}
	row := regexp.MustCompile(`(?m)^\| [A-Z_]+ \| [A-Z_]+ \| (\d+) \| ([^|]+) \|`)
	want := map[string]string{}
	for _, m := range row.FindAllStringSubmatch(section, -1) {
		want[m[1]] = strings.TrimSpace(m[2])
	}
	if len(want) < 10 {
		t.Fatalf("解析出的狀態碼太少：%v", want)
	}
	got := map[string]bool{}
	for _, m := range regexp.MustCompile(`(\d+)=`).FindAllStringSubmatch(statusGuide, -1) {
		got[m[1]] = true
	}
	for code, label := range want {
		if !got[code] {
			t.Errorf("狀態碼表缺 %s（%s）", code, label)
		}
	}
	for code := range got {
		if _, ok := want[code]; !ok {
			t.Errorf("狀態碼表多了 %s，CodeMeta 沒有", code)
		}
	}
	wantTracking := map[string]string{"20": "待出貨", "30": "配送中", "32": "待取貨", "35": "物流異常", "40": "已送達", "54": "配送失敗未退款（終態）"}
	gotTracking := strings.Split(trackingStatuses, ",")
	slices.Sort(gotTracking)
	wantCodes := make([]string, 0, len(wantTracking))
	for code, label := range wantTracking {
		wantCodes = append(wantCodes, code)
		if want[code] != label {
			t.Errorf("trackingStatuses 的 %s：CodeMeta 是 %q，預期 %q", code, want[code], label)
		}
	}
	slices.Sort(wantCodes)
	if !slices.Equal(gotTracking, wantCodes) {
		t.Errorf("trackingStatuses = %q（排序後 %v），要正好是 %v", trackingStatuses, gotTracking, wantCodes)
	}
}

func TestInstructionsFollowTheRequestLocaleOnlyAsAFallback(t *testing.T) {
	zh, en := coordinatorInstruction(uitext.ZhTW), coordinatorInstruction(uitext.EN)
	if !strings.Contains(zh, "無法判斷時使用繁體中文") || !strings.Contains(en, "無法判斷時使用English") {
		t.Fatalf("回覆語言的後備要依請求語系：\n%s\n%s", zh[len(zh)-120:], en[len(en)-120:])
	}
	for _, s := range []string{zh, en, orderSpecialistInstruction(uitext.EN), trackingSpecialistInstruction(uitext.EN)} {
		if !strings.Contains(s, "以買家這則訊息使用的語言回覆") || strings.Contains(s, "使用繁體中文回覆") {
			t.Fatalf("以買家訊息的語言為準，不得寫死語言：%s", s)
		}
	}
	if strings.Contains(zh, "20=已出貨") {
		t.Fatal("系統提示不得再有與 CodeMeta 不符的狀態碼")
	}
	if zh == en {
		t.Fatal("退貨流程文案要依語系")
	}
}
