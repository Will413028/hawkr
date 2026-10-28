package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 每一條反向代理路由都必須宣告「上游不可達時回什麼 status」，而判準是**這條路由的
// 消費者是誰**：
//
//   - 我們自己的前端 → proxy.FailClientFacing（424）。Cloudflare 會把 origin 的 5xx
//     body 整個換成 text/plain 錯誤頁，前端 response.json() 直接拋錯、拿不到 envelope。
//   - 第三方 webhook 送信端 → proxy.FailThirdPartyWebhook（502）。5xx 對 PayUni 是
//     「等一下重送」、4xx 是「別再送了」，而 10 號設計書 §4.5.3／§4.5.5.D 的整套失敗
//     回收（T2 失敗 → PayUni 重送 → dedup 命中 FAILED → replay 接手）就是靠重送觸發。
//
// 「必填參數」只保證有人回答了這個問題，不保證答對。這支測試補的是後者：它把答案
// 逐條釘死在下面兩張表裡。**沒有這兩張表，把 webhook 那條一起改成 424 的回歸不會有
// 任何徵兆**——編譯過、單元測試綠、CI 綠，要等到某次 payuni-webhook-service 短暫
// 不可用、PayUni 不再重送、付款狀態永久不同步才會被發現。
//
// ⚠️ **兩個套件都要掃**。只掃 router 會漏掉 proxy 套件內直接呼叫
// `writeUpstreamFailureEnvelope` 的路徑——`ai_assistant_stream.go` 現在
// 就是這個形狀（值寫死正確，但不受表約束）。
func TestProxyRoutesDeclareFailureStatusByConsumer(t *testing.T) {
	t.Run("router 套件：路由宣告", func(t *testing.T) {
		// key = "檔名:代理建構函式:第一個參數（service 常數）"，value = 宣告的失敗模式。
		expected := map[string]string{
			// 機敏檔案串流：消費者是 web / admin / App。
			"routes_media.go:CreateProxy:ServiceMedia": "proxy.FailClientFacing",
			// 發票／收據 PDF 授權下載（58 §6.1.4）：買家／賣家／ADM 三條子路由同檔同
			// 常數，消費者全是我們自己的前端 ⇒ 只需要這一列。
			"routes_invoice.go:CreateProxy:ServiceInvoice": "proxy.FailClientFacing",
			// routes_seller.go:CreateProxy:ServiceSeller 已於 #2813 移除——那是條死路由
			// （seller 模組未實作 /app/sellers/{id}、四個前端 repo 零呼叫端），
			// 且它每個請求都 panic 回 500 空 body。不要因為看到這裡少一條就補回去。
			// 開發用 webhook 模擬器：我們自己用瀏覽器開的工具，不是第三方送信端。
			"routes_webhook_simulator.go:proxyToPath:ServicePayuniWebhook": "proxy.FailClientFacing",
			// ⚠️ 唯一一條第三方 webhook。改成 4xx ＝ PayUni 停止重送 ＝ 10 號的 replay
			// 路徑永遠等不到觸發。要動它之前先讀 proxy.UpstreamFailureStatus 的說明。
			"routes_payment.go:CreateProxyWithPathRewrite:ServicePayuniWebhook": "proxy.FailThirdPartyWebhook",
		}
		got := scan(t, ".", func(fn string, call *ast.CallExpr, file, enclosing string) (string, string, bool) {
			switch fn {
			case "proxy.CreateProxy", "proxy.CreateProxyWithPathRewrite", "proxyToPath":
			default:
				return "", "", false
			}
			if len(call.Args) < 2 {
				t.Errorf("%s: %s 沒有傳失敗模式參數", file, fn)
				return "", "", false
			}
			short := strings.TrimPrefix(fn, "proxy.")
			return file + ":" + short + ":" + exprString(call.Args[0]),
				exprString(call.Args[len(call.Args)-1]), true
		}, map[string]bool{
			// proxyToPath 是把模式原樣往下傳的泛用 wrapper，本身不是一條路由——
			// 要檢查的是它的呼叫端。其他 wrapper 若出現，其內部呼叫會以參數名被
			// 記成「未登記」而轉紅，那是刻意的：新 wrapper 必須被看見。
			"proxyToPath": true,
		})
		compare(t, "router 路由", got, expected)
	})

	t.Run("proxy 套件：writeUpstreamFailureEnvelope 的呼叫點", func(t *testing.T) {
		// key = "檔名:所在函式"，value = 傳給第 3 個參數（status）的運算式。
		expected := map[string]string{
			// 泛用建構函式：模式由呼叫端傳入，答案在上面那張表。這裡把值釘成參數名，
			// 是為了在有人把它寫死成某個常數時轉紅（那會讓上面那張表失去效力）。
			"reverse_proxy.go:CreateProxy":                "onUpstreamFailure",
			"reverse_proxy.go:CreateProxyWithPathRewrite": "onUpstreamFailure",
			// ai-service 的 SSE 路徑：消費者全是我們自己的前端（web AI 助手、兩支 App
			// 的採購助手），沒有第三方送信端。
			"ai_assistant_stream.go:serveAIAssistantStreamWith": "FailClientFacing",
			// Task 12：GET replay 端點，消費者同樣是我們自己的前端（web 的
			// procurement-assistant 頁重新整理／重連時的重播來源），沒有第三方送信端。
			"ai_run_events_stream.go:serveAIProcurementRunEventsStreamWith": "FailClientFacing",
		}
		got := scan(t, filepath.Join("..", "proxy"), func(fn string, call *ast.CallExpr, file, enclosing string) (string, string, bool) {
			if fn != "writeUpstreamFailureEnvelope" {
				return "", "", false
			}
			if len(call.Args) < 4 {
				t.Errorf("%s: writeUpstreamFailureEnvelope 參數數量異常（%d）", file, len(call.Args))
				return "", "", false
			}
			return file + ":" + enclosing, exprString(call.Args[2]), true
		}, nil)
		compare(t, "proxy 寫入點", got, expected)
	})
}

// scan 解析 dir 下的非測試 .go 檔，對每個 CallExpr 呼叫 pick 取出 (key, value)。
// skipFuncs 內的函式整個不進掃描（泛用 wrapper）。
func scan(
	t *testing.T,
	dir string,
	pick func(fn string, call *ast.CallExpr, file, enclosing string) (string, string, bool),
	skipFuncs map[string]bool,
) map[string]map[string]int {
	t.Helper()
	got := map[string]map[string]int{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("讀取 %s 失敗：%v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("解析 %s 失敗：%v", name, perr)
		}
		for _, decl := range f.Decls {
			fd, isFunc := decl.(*ast.FuncDecl)
			if !isFunc {
				continue
			}
			if skipFuncs[fd.Name.Name] {
				continue
			}
			ast.Inspect(fd, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				key, val, take := pick(calleeName(call.Fun), call, name, fd.Name.Name)
				if !take {
					return true
				}
				if got[key] == nil {
					got[key] = map[string]int{}
				}
				got[key][val]++
				return true
			})
		}
	}
	return got
}

func compare(t *testing.T, label string, got map[string]map[string]int, expected map[string]string) {
	t.Helper()
	for key, modes := range got {
		want, listed := expected[key]
		if !listed {
			t.Errorf("[%s] 新增了未登記的項目 %q（模式 %v）——請加進 expected 表，"+
				"並回答「這條的消費者是我們自己的前端還是第三方送信端」", label, key, sortedModes(modes))
			continue
		}
		if len(modes) != 1 || modes[want] == 0 {
			t.Errorf("[%s] %q 的失敗模式 = %v，want 全部都是 %s。"+
				"改動這個值之前先讀 proxy.UpstreamFailureStatus 的說明：判準是消費者是誰，"+
				"不是錯誤的技術性質", label, key, sortedModes(modes), want)
		}
	}
	for key := range expected {
		if _, present := got[key]; !present {
			t.Errorf("[%s] expected 表列了 %q 但原始碼裡找不到——被刪或被改名時請同步這張表", label, key)
		}
	}
}

func calleeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if pkg, ok := v.X.(*ast.Ident); ok {
			return pkg.Name + "." + v.Sel.Name
		}
	}
	return ""
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return calleeName(v)
	case *ast.BasicLit:
		return v.Value
	}
	return "<expr>"
}

func sortedModes(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
