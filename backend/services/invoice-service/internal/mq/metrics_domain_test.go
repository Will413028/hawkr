package mq

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// M30 的值域必須涵蓋每一個真的會被呼叫的組合
//
// 🔴 `observeEventDropped` 對值域外的組合**直接 return**（刻意不造新序列）。那是
// fail-open：新增一個 consumer、呼叫它、忘了把 event_type 加進 eventDroppedTypes
// ⇒ 那一整類「事件連 DB 列都沒有」的失效**不會被計數**，而 M30 正是它唯一的觀測點
// （inbox 沒有列、DLQ 是 0、M8 的基準是「有那一列」）。
// 症狀是：/metrics 上永遠看不到，dashboard 一切正常。
//
// 這一支把「呼叫點」與「值域宣告」兩份材料對帳。兩份都從**原始碼**讀，不從執行期
// 讀：執行期只看得到被跑過的那些路徑，而沒被跑過的呼叫點正是問題所在。
// ---------------------------------------------------------------------------

// TestEventDroppedDomainCoversEveryCallSite 是上面那段的機械保證。
func TestEventDroppedDomainCoversEveryCallSite(t *testing.T) {
	callTypes, callReasons := observeEventDroppedCallSites(t)
	declTypes := identsInSliceLiteral(t, "eventDroppedTypes")
	declReasons := stringsInSliceLiteral(t, "eventDroppedReasons")

	if len(callTypes) == 0 || len(callReasons) == 0 {
		t.Fatal("在本 package 找不到任何 observeEventDropped 呼叫點 —— 掃描器壞了，" +
			"不是「沒有呼叫點」。這支測試會恆綠")
	}

	assertSetsEqual(t, "event_type", callTypes, declTypes,
		"呼叫點用了但沒宣告 ⇒ observeEventDropped 靜默忽略該次觀測（fail-open）",
		"宣告了但沒有任何呼叫點 ⇒ /metrics 上多一條永遠為 0 的序列，讓人以為那條路徑被守著")
	assertSetsEqual(t, "reason", callReasons, declReasons,
		"呼叫點用了但沒宣告 ⇒ 同上，該 reason 的觀測全部被丟掉",
		"宣告了但沒有任何呼叫點 ⇒ 同上，一條不會動的假序列")
}

// TestEventDroppedDomainScannerHasDiscriminatingPower 驗掃描器**抓得到**缺漏。
//
// 🔴 沒有這一支的話，掃描器只要回空集合（package 佈局改變、函式改名、AST 走訪寫錯），
// 上面那支就會靜默轉綠——那是把 gate 關掉，而它長得跟修好一模一樣。
func TestEventDroppedDomainScannerHasDiscriminatingPower(t *testing.T) {
	callTypes, callReasons := observeEventDroppedCallSites(t)

	// ① 掃描器必須真的認得出本波新增的那一個（不是掃到空集合而恆綠）。
	for _, want := range []string{"EventTypeOrderCancelled", "EventTypeOrderPaid", "EventTypeOrderRefundFinalized"} {
		if !callTypes[want] {
			t.Errorf("掃描器沒找到 observeEventDropped(%s, ...) 的呼叫點", want)
		}
	}
	if !callReasons["empty_buyer_id"] {
		t.Error("掃描器沒找到 reason=empty_buyer_id 的呼叫點 —— 它是 #2904 新增的那一個")
	}

	// ② 餵一個「呼叫點用了一個沒宣告的值」的來源，掃描器必須讓對帳失敗。
	//    這裡直接對 assertSetsEqual 餵反例，證明比對本身有鑑別力
	//    （而不是「兩個空集合相等」那種假綠）。
	sub := &testing.T{}
	assertSetsEqual(sub, "event_type",
		map[string]bool{"EventTypeOrderPaid": true, "EventTypeGhost": true},
		map[string]bool{"EventTypeOrderPaid": true},
		"a", "b")
	if !sub.Failed() {
		t.Fatal("反例沒有被抓到：呼叫點多了一個未宣告的 event_type 竟然通過了")
	}
}

// observeEventDroppedCallSites 掃本 package 的非測試檔，收集每一個
// observeEventDropped(arg0, arg1) 的 arg0 選擇子名與 arg1 字串字面值。
//
// arg0 的形狀固定是 `string(pkgmq.EventTypeXxx)` ⇒ 取最內層的 Sel 名。刻意**不解析
// 常數的值**：值要對到 pkgmq 得再引入一份對照，而那份對照又會是手寫的
// ——本 skill 要防的就是「手寫材料自己跟自己對」。比對識別字本身是純原始碼對原始碼。
func observeEventDroppedCallSites(t *testing.T) (types, reasons map[string]bool) {
	t.Helper()
	types, reasons = map[string]bool{}, map[string]bool{}

	// 不用 parser.ParseDir：Go 1.25 起 deprecated，且它不看 build tag ——
	// 對「哪些檔屬於這個 package」的判定不精確，而本掃描的鑑別力正好建立在
	// 「掃到全部生產檔」上。改成自己列檔，掃不到任何檔就 Fatal（fail-closed）。
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("讀取 package 目錄: %v", err)
	}
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("掃到 0 個生產檔 —— 掃描器壞了，不是「沒有違規」")
	}

	{
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if !ok || id.Name != "observeEventDropped" || len(call.Args) != 2 {
					return true
				}
				if name := innermostSelector(call.Args[0]); name != "" {
					types[name] = true
				}
				if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil {
						reasons[v] = true
					}
				}
				return true
			})
		}
	}
	return types, reasons
}

// innermostSelector 從 `string(pkgmq.EventTypeOrderPaid)` 取出 `EventTypeOrderPaid`。
func innermostSelector(e ast.Expr) string {
	for {
		switch v := e.(type) {
		case *ast.CallExpr:
			if len(v.Args) != 1 {
				return ""
			}
			e = v.Args[0]
		case *ast.ParenExpr:
			e = v.X
		case *ast.SelectorExpr:
			return v.Sel.Name
		default:
			return ""
		}
	}
}

// identsInSliceLiteral 讀 metrics.go 裡某個 []string 變數的元素，回傳每個元素最內層的
// 選擇子名（元素形狀是 `string(pkgmq.EventTypeXxx)`）。
func identsInSliceLiteral(t *testing.T, varName string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	forEachSliceElem(t, varName, func(e ast.Expr) {
		if name := innermostSelector(e); name != "" {
			out[name] = true
		}
	})
	if len(out) == 0 {
		t.Fatalf("%s 解出 0 個元素 —— 解析器壞了，不是值域空了", varName)
	}
	return out
}

// stringsInSliceLiteral 同上，但元素是字串字面值。
func stringsInSliceLiteral(t *testing.T, varName string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	forEachSliceElem(t, varName, func(e ast.Expr) {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				out[v] = true
			}
		}
	})
	if len(out) == 0 {
		t.Fatalf("%s 解出 0 個元素 —— 解析器壞了，不是值域空了", varName)
	}
	return out
}

func forEachSliceElem(t *testing.T, varName string, fn func(ast.Expr)) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "metrics.go", nil, 0)
	if err != nil {
		t.Fatalf("parse metrics.go: %v", err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || vs.Names[0].Name != varName || len(vs.Values) != 1 {
			return true
		}
		cl, ok := vs.Values[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		found = true
		for _, e := range cl.Elts {
			fn(e)
		}
		return false
	})
	if !found {
		t.Fatalf("在 metrics.go 找不到 %s 的宣告 —— 檢查本身失效了", varName)
	}
}

func assertSetsEqual(t *testing.T, label string, calls, decls map[string]bool, missingMsg, extraMsg string) {
	t.Helper()
	for k := range calls {
		if !decls[k] {
			t.Errorf("%s %q 出現在 observeEventDropped 的呼叫點，但不在值域宣告裡。\n%s",
				label, k, missingMsg)
		}
	}
	for k := range decls {
		if !calls[k] {
			t.Errorf("%s %q 宣告在值域裡，但沒有任何 observeEventDropped 呼叫點用它。\n%s\n"+
				"（目前的呼叫點：%v）", label, k, extraMsg, sortedKeys(calls))
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
