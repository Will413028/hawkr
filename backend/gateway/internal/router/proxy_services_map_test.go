package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 每一個真的被傳進 CreateProxy／proxyToPath 的 service name，都必須是
// proxy/reverse_proxy.go 的 `services` map 的 key。
//
// **為什麼需要這一支**：2026-08-29 的獨立驗收實測——把 `proxy/reverse_proxy.go` 的
// `services` map 拿掉 `"invoice"` 那一整列，`go build` 過、gateway 全部 16 個 package
// 的 `go test` 零 FAIL。而 `CreateProxy` 查不到 service name 時回的是一個**永遠**
// `response.NotFound` 的 handler ⇒ 該服務的全部反代路由靜默退化成 404。
//
// 58 §12.2.5 第 33 列與 §6.1.4 都預言了這個症狀（「下載按鈕點了說找不到」）並標成
// 「①② 漏了**不會編譯錯**」，但在此之前沒有任何東西會轉紅——`proxy_failure_status_test.go`
// 掃的是 `routes_invoice.go` 的 `CreateProxy` **呼叫點**，不碰 map 本身。
//
// ⚠️ **不變式刻意是「呼叫端 ⊆ map」而不是「Service* 常數 ⊆ map」**：實查 ServiceCart
// 與 ServiceCoupon 在 constants.go 之外零使用（它們是給別處當字串用的命名，不是反代
// 宣告），拿全部常數去比對會產生假陽性。真正會壞的只有「有人呼叫、但 map 沒有」。
func TestProxiedServiceNamesExistInServicesMap(t *testing.T) {
	called := scanProxiedServiceNames(t)
	mapKeys := parseProxyServicesMapKeys(t)

	if len(called) == 0 {
		t.Fatal("掃不到任何 CreateProxy／proxyToPath 呼叫端——掃描器壞了，不是「沒有反代路由」")
	}
	if len(mapKeys) == 0 {
		t.Fatal("解析不到任何 services map key——解析器壞了")
	}

	inMap := map[string]bool{}
	for _, k := range mapKeys {
		inMap[k] = true
	}
	for _, name := range called {
		if !inMap[name] {
			t.Errorf("service name %q 被傳進 CreateProxy／proxyToPath，但 proxy/reverse_proxy.go 的 "+
				"services map 沒有它——CreateProxy 查不到 name 會回一個永遠 response.NotFound 的 "+
				"handler，該服務的反代路由全部靜默 404（不編譯錯、也沒有別的測試會轉紅）", name)
		}
	}
}

// scanProxiedServiceNames 掃 router 的非測試檔，取 CreateProxy／
// CreateProxyWithPathRewrite／proxyToPath 第一個參數的**值**。
// 參數是 Service* 常數時解析成它在 constants.go 的字面值；是字串字面值時直接取。
func scanProxiedServiceNames(t *testing.T) []string {
	t.Helper()
	constByName := parseServiceConstants(t)
	seen := map[string]bool{}
	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("讀取 router 目錄失敗：%v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("解析 %s 失敗：%v", name, perr)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			switch calleeName(call.Fun) {
			case "proxy.CreateProxy", "proxy.CreateProxyWithPathRewrite", "proxyToPath":
			default:
				return true
			}
			switch arg := call.Args[0].(type) {
			case *ast.Ident:
				// proxyToPath 的包裝函式自己也符合形狀，但它的第一個參數是參數名
				// serviceName（不是常數）⇒ 查不到就跳過，不誤報。
				if v, found := constByName[arg.Name]; found {
					seen[v] = true
				}
			case *ast.BasicLit:
				if arg.Kind == token.STRING {
					if str, uerr := strconv.Unquote(arg.Value); uerr == nil {
						seen[str] = true
					}
				}
			}
			return true
		})
	}
	out := make([]string, 0, len(seen))
	for str := range seen {
		out = append(out, str)
	}
	sort.Strings(out)
	return out
}

// parseServiceConstants 回傳 constants.go 內 ServiceXxx = "..." 的 名稱→值。
func parseServiceConstants(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "constants.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("解析 constants.go 失敗：%v", err)
	}
	out := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
			return true
		}
		name := vs.Names[0].Name
		if !strings.HasPrefix(name, "Service") {
			return true
		}
		bl, ok := vs.Values[0].(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return true
		}
		if str, uerr := strconv.Unquote(bl.Value); uerr == nil {
			out[name] = str
		}
		return true
	})
	return out
}

// parseProxyServicesMapKeys 取 `var services = map[string]ServiceConfig{...}` 的 key。
func parseProxyServicesMapKeys(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "../proxy/reverse_proxy.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("解析 reverse_proxy.go 失敗：%v", err)
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "services" || len(vs.Values) != 1 {
			return true
		}
		lit, ok := vs.Values[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if bl, ok := kv.Key.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				if str, uerr := strconv.Unquote(bl.Value); uerr == nil {
					out = append(out, str)
				}
			}
		}
		return false
	})
	sort.Strings(out)
	return out
}
