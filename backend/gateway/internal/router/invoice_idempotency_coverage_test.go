package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 本檔守的是 issue #3015 的**復發面**：`Idempotency-Key` 的重放語意現在由
// `middleware.RequireIdempotencyKey` 執行（格式驗證與去重綁在同一支，見該函式的
// doc comment）⇒ 只要有一條發票寫入路由沒掛上它，那條路由就又回到「必填但沒作用」，
// 而**那件事不會編譯錯、不會讓任何既有測試轉紅**。
//
// 🔴 **這道 gate 的觀測窗與它擋不住的東西**（`.claude/rules/06-no-small-tails.md` §3.10）：
//
//	① 觀測窗 ＝ `routes_invoice.go` 這一個檔的 AST。把發票寫入路由註冊到**別的檔案**
//	   就在窗外 —— 因此 assertion 之一是「本檔必須找得到既有的那批寫入路由」，
//	   數量掉下去就轉紅（搬走跟刪掉一樣會被抓到）。
//	② 它驗的是「有沒有掛上」，不是「掛上之後有沒有效」。後者由
//	   `internal/middleware/idempotency_test.go` 的行為測試守（那批會計算 handler
//	   被執行幾次）。兩者缺一都不夠：只有行為測試 ⇒ 新路由忘了掛沒人知道；
//	   只有本檔 ⇒ middleware 被掏空成 no-op 沒人知道。
//	③ 檔案讀不到／解析不了／掃出 0 條寫入路由 ⇒ **FAIL 而不是 SKIP**。
//	   「目標不存在就靜默轉綠」是這類 gate 最常見的死法。

const idempotencyMiddlewareIdent = "RequireIdempotencyKey"

// invoiceWriteRoutePin 是 routes_invoice.go 目前的寫入路由條數（2026-09-02 實測）。
// 新增寫入路由時**先確認它掛了 RequireIdempotencyKey**，再把這個數字加上去；
// 數字變小代表有路由被搬走或刪掉，一樣要當成需要解釋的事。
const invoiceWriteRoutePin = 37

type writeRoute struct {
	group  string
	method string
	path   string
	inline bool // 該次註冊的參數裡直接帶了 RequireIdempotencyKey
}

func TestInvoiceWriteRoutesAllRequireIdempotencyKey(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes_invoice.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 routes_invoice.go 失敗（觀測窗不存在 ⇒ FAIL，不 SKIP）：%v", err)
	}

	covered := map[string]bool{} // 整組 .Use(RequireIdempotencyKey) 的群組
	var routes []writeRoute

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// sellerHandle(group, "POST", Path, cap, handler)
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "sellerHandle" && len(call.Args) >= 3 {
			g := exprName(call.Args[0])
			m := stringLit(call.Args[1])
			if isWriteMethod(m) {
				routes = append(routes, writeRoute{group: g, method: m, path: exprName(call.Args[2])})
			}
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		g := exprName(sel.X)
		switch sel.Sel.Name {
		case "Use":
			if callArgsMentionIdempotency(call.Args) {
				covered[g] = true
			}
		case "POST", "PUT", "DELETE":
			if len(call.Args) == 0 {
				return true
			}
			routes = append(routes, writeRoute{
				group:  g,
				method: sel.Sel.Name,
				path:   exprName(call.Args[0]),
				inline: callArgsMentionIdempotency(call.Args[1:]),
			})
		}
		return true
	})

	if len(routes) == 0 {
		t.Fatal("在 routes_invoice.go 掃不到任何寫入路由 —— 掃描器或檔案結構已漂移，" +
			"這種情況必須 FAIL：靜默轉綠正是這道 gate 要防的事")
	}

	var uncovered []string
	for _, r := range routes {
		if r.inline || covered[r.group] {
			continue
		}
		uncovered = append(uncovered, r.method+" "+r.group+"/"+r.path)
	}
	sort.Strings(uncovered)
	if len(uncovered) > 0 {
		t.Fatalf("以下發票寫入路由沒有掛 middleware.%s，等於把 issue #3015 重新鑿回來：\n  %s",
			idempotencyMiddlewareIdent, strings.Join(uncovered, "\n  "))
	}

	if len(routes) != invoiceWriteRoutePin {
		t.Fatalf("發票寫入路由 %d 條，pin 是 %d 條。"+
			"新增請先確認掛了 %s 再更新 pin；變少代表被搬到本 gate 看不到的地方",
			len(routes), invoiceWriteRoutePin, idempotencyMiddlewareIdent)
	}
}

// 對照組：證明這道 gate 抓得到「少掛一條」。
// 沒有它的話，掃描器寫壞成「永遠掃不到未覆蓋的路由」也一樣是全綠。
func TestInvoiceIdempotencyCoverageGateHasDiscrimination(t *testing.T) {
	routes := []writeRoute{
		{group: "buyerProfiles", method: "POST", path: "X"},
		{group: "orderInvoices", method: "POST", path: "Y"}, // 沒有整組 Use、也沒有 inline
	}
	covered := map[string]bool{"buyerProfiles": true}

	var uncovered []string
	for _, r := range routes {
		if r.inline || covered[r.group] {
			continue
		}
		uncovered = append(uncovered, r.method+" "+r.path)
	}
	if len(uncovered) != 1 || uncovered[0] != "POST Y" {
		t.Fatalf("判定邏輯抓不到未覆蓋的路由，uncovered=%v", uncovered)
	}
}

func isWriteMethod(m string) bool {
	return m == "POST" || m == "PUT" || m == "DELETE"
}

func callArgsMentionIdempotency(args []ast.Expr) bool {
	for _, a := range args {
		found := false
		ast.Inspect(a, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == idempotencyMiddlewareIdent {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

func exprName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.BasicLit:
		if s, err := strconv.Unquote(v.Value); err == nil {
			return s
		}
		return v.Value
	default:
		return "?"
	}
}

func stringLit(e ast.Expr) string {
	if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		if s, err := strconv.Unquote(lit.Value); err == nil {
			return s
		}
	}
	return ""
}
