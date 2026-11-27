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

	for _, want := range []string{"EventTypeOrderCancelled", "EventTypeOrderPaid", "EventTypeOrderRefundFinalized"} {
		if !callTypes[want] {
			t.Errorf("掃描器沒找到 observeEventDropped(%s, ...) 的呼叫點", want)
		}
	}
	if !callReasons["empty_buyer_id"] {
		t.Error("掃描器沒找到 reason=empty_buyer_id 的呼叫點 —— 它是 #2904 新增的那一個")
	}

	sub := &testing.T{}
	assertSetsEqual(sub, "event_type",
		map[string]bool{"EventTypeOrderPaid": true, "EventTypeGhost": true},
		map[string]bool{"EventTypeOrderPaid": true},
		"a", "b")
	if !sub.Failed() {
		t.Fatal("反例沒有被抓到：呼叫點多了一個未宣告的 event_type 竟然通過了")
	}
}

func observeEventDroppedCallSites(t *testing.T) (types, reasons map[string]bool) {
	t.Helper()
	types, reasons = map[string]bool{}, map[string]bool{}

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
