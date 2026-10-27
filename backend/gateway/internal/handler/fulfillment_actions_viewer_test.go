package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// 🔴 兩個 handler 傳給 allowedActionsOf 的 viewer 是**硬寫常數**，而它決定整個 actor 過濾的方向。
//
// 把 GetOrderLogistics 的 ViewerBuyer 換成 ViewerSeller 就地復活 QA 的原症狀
// （買家端回三條掛 seller capability 的 href，買家跟隨即 403），
// 而在本測試存在之前，那個改動不會讓任何測試轉紅。
//
// **為什麼硬寫是對的設計**（不要「順手」改成從 context 取帳號類型）：
// AccountType 有 SELLER_AND_BUYER=3，若 viewer 取自帳號，type 3 的使用者會與
// 任何 actor 值都不相等 ⇒ allowedActions 恆空。
// 端點定義的是「此刻扮演的角色」，不是「這個帳號有什麼能力」——所以它屬於路由，不屬於身分。
//
// 用 AST 而不是字串 grep：換行、註解、引數順序改變都不該讓它誤報或漏報。
func TestAllowedActionsViewerIsPinnedPerEndpoint(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "shipping.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("解析 shipping.go 失敗：%v", err)
	}

	want := map[string]string{
		"GetOrderLogistics":         "ViewerBuyer",  // 買家端：/app/orders/{no}/logistics
		"GetSellerShipmentTracking": "ViewerSeller", // 賣家端：/app/shipments/{no}/tracking
	}
	found := map[string]string{}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if _, watched := want[fn.Name.Name]; !watched {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "allowedActionsOf" || len(call.Args) != 2 {
				return true
			}
			arg, ok := call.Args[1].(*ast.Ident)
			if !ok {
				t.Fatalf("%s 的 viewer 引數不是具名常數（%T）——"+
					"改成運算式會讓本測試失去鑑別力，請同步更新它", fn.Name.Name, call.Args[1])
			}
			found[fn.Name.Name] = arg.Name
			return true
		})
	}

	// 🔴 對「目標不存在」FAIL 而不是 skip：handler 改名／allowedActionsOf 被包一層，
	// 都會讓這道釘樁靜默失效，而失效的症狀就是它守的那個 bug 回來了。
	for fn, expect := range want {
		got, ok := found[fn]
		if !ok {
			t.Fatalf("在 %s 內找不到 allowedActionsOf(...) 呼叫——"+
				"handler 被改名或該呼叫被包了一層？本測試對目標不存在一律 FAIL", fn)
		}
		if got != expect {
			t.Fatalf("%s 的 viewer 應為 %s，實得 %s —— "+
				"換了方向就等於把 QA 的原症狀放回去", fn, expect, got)
		}
	}
}
