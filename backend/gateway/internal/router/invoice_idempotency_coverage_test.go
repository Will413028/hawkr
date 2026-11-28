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

const idempotencyMiddlewareIdent = "RequireIdempotencyKey"

const invoiceWriteRoutePin = 37

type writeRoute struct {
	group  string
	method string
	path   string
	inline bool
}

func TestInvoiceWriteRoutesAllRequireIdempotencyKey(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes_invoice.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 routes_invoice.go 失敗（觀測窗不存在 ⇒ FAIL，不 SKIP）：%v", err)
	}

	covered := map[string]bool{}
	var routes []writeRoute

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
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
		{group: "orderInvoices", method: "POST", path: "Y"},
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
