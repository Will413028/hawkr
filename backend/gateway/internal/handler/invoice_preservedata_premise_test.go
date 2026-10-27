package handler

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// 🔴 2026-08-31：舊 AST gate（TestInvoiceHandlersGuardPreserveDataCodes）已刪除
// ---------------------------------------------------------------------------
//
// **它的核心判斷是 fail-OPEN**：`if len(after) == 0 { continue }` —— 「這個呼叫點之後
// 找不到成功出口」被當成安全的證明，但它同樣可以只是「我看不到而已」。實測 10 種變異
// （每一種都 go build exit=0、gofmt 過）：
//
//	變異                                   舊 gate    bizcarrier 的行為測試
//	m0 直接刪掉守門                         FAIL 抓到   PASS（不變式仍成立）
//	m1 import 取別名 rsp "…/response"       PASS 全盲   PASS
//	m2 RPC 抽成 helper（call/emit 分兩函式）  PASS 全盲   PASS
//	m3 守門內再包一層 `== 9332`              PASS 全盲   PASS
//	m4 守門塞進 `if c.Query(...) != ""`      FAIL 抓到   PASS
//	m5 讀了 code 但從不 reject               FAIL 抓到   PASS
//	m6 有 Biz 但忘了 return                  FAIL 抓到   PASS
//	m7 第二個未被守門的回傳點                 FAIL 抓到   PASS
//	m8 先 OK 再守門                          FAIL 抓到   PASS
//	m9 handler 搬到別的 package              PASS 全盲   PASS
//
// 它最後被降級成「顯式處理的風格閘門」而留著。**但降級不改變它是 fail-OPEN 這件事**：
// 對那個比較弱的主張，m1／m2／m3／m9 一樣是「看不到 ⇒ 報零 finding ⇒ 綠燈」。一道
// 看不到就轉綠的閘門，它的綠燈不代表任何事，而測試的名字讀起來像個保證 ——
// 那正是「假的保證會排擠掉真的警覺」（.claude/rules/02-judgment.md §2.1.2）。
//
// 取代它的東西分成三層，每一層都是行為面的、不靠原始碼形狀：
//
//  1. **不變式**：internal/bizcarrier ＋ internal/response 的出口矩陣。任何寫出
//     code==0 的 envelope 都會先查 carrier，跟 handler 長什麼樣、住哪個 package、
//     import 取什麼別名完全無關。
//  2. **前提**：本檔的 TestPreserveDataPremiseStillHolds —— preserveData 名單與它的
//     `return nil` 分支是整套設計的前提，消失就 FAIL 不 SKIP。
//  3. **顯式處理的衛生**：改由 gateway_bizcode_swallowed_total ＋ ERROR log 承擔。
//     它是**執行期**訊號，看不到的東西不會讓它靜默轉綠 —— 這正是舊 gate 做不到的。
//     代價誠實寫在這裡：那個訊號要等該碼真的出現在流量裡才會響，而 71 支 handler 的
//     守門在此期間可能已經被刪掉一支。這是「延遲偵測」換掉「假保證」，不是零成本。
//
// 保留下來的 71 支 handler 守門（`if resp.GetCode() != 0 { response.Biz(...); return }`）
// 一支都沒動：它們不是第二道防線，它們就是**正確的主要寫法**，carrier 是安全網。

const (
	// preserveDataInterceptorPath 是前提的路徑（相對於本 package 目錄）。
	preserveDataInterceptorPath = "../grpc/envelope_guard_interceptor.go"
	// preserveDataMapName 是 interceptor 內那張「不轉 error」的名單。
	preserveDataMapName = "envelopeCodePreserveData"
	// responsePkgPath 是成功出口所在的 package，用來逐檔解析 import 別名。
	responsePkgPath = "github.com/Will413028/hawkr/gateway/internal/response"
)

func premiseParseFile(t *testing.T, fset *token.FileSet, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("無法解析 %s：%v\n（目標讀不到時一律 FAIL 而非 SKIP：靜默跳過等於把檢查關掉）", path, err)
	}
	return f
}

// ---------------------------------------------------------------------------
// 前提：preserveData 名單與它的 return nil 分支
// ---------------------------------------------------------------------------
//
// 這兩樣東西同時是「失效模式存不存在」與「bizcarrier 有沒有存在意義」的判定材料。
// 名單清空或分支改成回 error ⇒ handler 的 `if err != nil` 攔得住了，整套 carrier
// 要由人決定是否退場。靜默轉綠會讓那個決定沒有人做。
func TestPreserveDataPremiseStillHolds(t *testing.T) {
	fset := token.NewFileSet()
	f := premiseParseFile(t, fset, preserveDataInterceptorPath)

	var (
		declFound bool
		codes     []int32
	)
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range vs.Names {
			if name.Name != preserveDataMapName || i >= len(vs.Values) {
				continue
			}
			declFound = true
			cl, ok := vs.Values[i].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				lit, ok := kv.Key.(*ast.BasicLit)
				if !ok || lit.Kind != token.INT {
					continue
				}
				enabled, ok := kv.Value.(*ast.Ident)
				if !ok || enabled.Name != "true" {
					continue
				}
				v, err := strconv.ParseInt(lit.Value, 0, 32)
				if err != nil {
					continue
				}
				codes = append(codes, int32(v))
			}
		}
		return true
	})

	if !declFound {
		t.Fatalf("在 %s 找不到 %s 宣告。\n"+
			"那張名單是 bizcarrier 整套設計的前提（名單內的碼 interceptor 不轉 error），"+
			"找不到就無從判定，必須由人決定 carrier 是否一起退場。",
			preserveDataInterceptorPath, preserveDataMapName)
	}
	if !preserveDataBranchReturnsNil(f) {
		t.Fatalf("%s 內找不到 `if %s[code] { … return nil }` 這個分支。\n"+
			"那個 `return nil` 就是前提——它一旦改成回 error，handler 的 `if err != nil` "+
			"就攔得住了，bizcarrier 的存在理由要重新推導。",
			preserveDataInterceptorPath, preserveDataMapName)
	}
	if len(codes) == 0 {
		t.Fatalf("%s 內已無任何 preserveData 碼。\n"+
			"若這是刻意移除，請連同 bizcarrier 一起處置並在 PR 說明；"+
			"靜默轉綠會讓「handler 不檢查 code」重新變成無人看守的行為。",
			preserveDataInterceptorPath)
	}

	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	// 結帳熱路徑（4001／4091／4092）與發票 93xx 都在同一張名單上。前者被列出來不是
	// 附帶說明：bizcarrier 改動的是 GrpcResult／Biz 這幾支結帳也在用的出口，
	// 「這不只影響 invoice」必須看得見。
	var checkout, invoice []int32
	for _, c := range codes {
		if c >= 4000 && c < 5000 {
			checkout = append(checkout, c)
		}
		if c >= 9300 && c <= 9399 {
			invoice = append(invoice, c)
		}
	}
	if len(checkout) == 0 {
		t.Errorf("preserveData 名單裡已無結帳碼（4001/4091/4092）——"+
			"那是結帳熱路徑的契約，消失要有人解釋。目前名單：%v", codes)
	}
	if len(invoice) == 0 {
		t.Errorf("preserveData 名單裡已無發票 93xx 碼。目前名單：%v", codes)
	}
	t.Logf("preserveData 名單 %d 碼：結帳 %v、發票 %v", len(codes), checkout, invoice)
}

func preserveDataBranchReturnsNil(f *ast.File) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		ifs, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		idx, ok := ifs.Cond.(*ast.IndexExpr)
		if !ok {
			return true
		}
		id, ok := idx.X.(*ast.Ident)
		if !ok || id.Name != preserveDataMapName {
			return true
		}
		ast.Inspect(ifs.Body, func(m ast.Node) bool {
			ret, ok := m.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return true
			}
			if lit, ok := ret.Results[0].(*ast.Ident); ok && lit.Name == "nil" {
				found = true
			}
			return true
		})
		return true
	})
	return found
}

// ---------------------------------------------------------------------------
// 逐一枚舉 invoice handler 的 71 個成功出口呼叫點
// ---------------------------------------------------------------------------
//
// 這支測試回答的是：「71 個呼叫點都被 carrier 蓋到了嗎」——而且答案由 AST 現數，
// 不是由誰記得。它與舊 gate 有一個關鍵差別：**別名由 import 宣告解析**，所以
// `rsp "…/response"`（m1，舊 gate 的致命盲點）在這裡照樣數得到。
//
// ⚠️ 它證明什麼、不證明什麼，講清楚：
//   - 證明：這 71 個點用的都是 internal/response 的出口，而每一支出口都在
//     bizcarrier_emitter_matrix_test.go 裡跑過 idle／armed 兩種狀態。
//   - 不證明：每個點各自送的 data 長什麼樣。data 對出口是不透明的（`interface{}`
//     一路交給 c.JSON），出口的形狀不隨呼叫點改變 —— 所以「逐點斷言 body」會是把
//     同樣三行跑 71 次。真正可能因為加了一層包裝而改變位元的是 payload 的**種類**
//     （nil／零值／空 slice／RawMessage），那一組在 emitter 矩陣的
//     TestEmitterMatrix_PayloadKindsUnchanged 裡逐種驗過。
//
// 這裡仍然逐點跑一次對應的出口（idle → code 0 ＋ message success；armed → 改寫成
// 業務碼），因為「逐點」本身是廉價的，而它讓 57／14 這兩個數字是被跑出來的、
// 不是被宣稱的。

// invoiceHandlerFiles 以 glob 現數，不是寫死的檔名清單。
//
// 🔴 寫死清單是 fail-OPEN：新增一個 invoice_*.go 就完全看不到 —— 那正是舊 AST gate
// 的病。同一個形狀在本次修改中被變異抓到過一次（emitter 完整性檢查原本寫死
// response.go／bizhttp.go，新檔案裡的出口照樣 PASS），所以這裡一開始就用 glob。
// 檔案集合變動 ⇒ 呼叫點數變動 ⇒ 下面的 pin 轉紅 ⇒ 有人來看。
func invoiceHandlerFiles(t *testing.T) []string {
	t.Helper()
	all, err := filepath.Glob("invoice_*.go")
	if err != nil {
		t.Fatalf("glob invoice_*.go 失敗：%v", err)
	}
	var out []string
	for _, p := range all {
		if !strings.HasSuffix(p, "_test.go") {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		t.Fatal("找不到任何 invoice_*.go —— invoice handler 被搬走／改名了，" +
			"本檢查已經在守一個不存在的東西，FAIL 而非 SKIP")
	}
	sort.Strings(out)
	return out
}

// responseEmitters 是 internal/response 對外的寫 envelope 出口。與該 package 的
// emitter 矩陣同一份母體；這裡只用來分類呼叫點。
var responseEmitters = map[string]bool{
	"OK": true, "OKProto": true, "Page": true, "GrpcResult": true,
	"Biz": true, "BizDeny": true, "BizStatus": true, "AbortBizStatus": true,
	"BizStatusData": true, "Err": true, "BadRequest": true, "BadRequestFields": true,
	"Unauthorized": true, "AbortUnauthorized": true, "Forbidden": true, "NotFound": true,
	"ServiceUnavailable": true, "PhoneVerificationRequired": true, "TooManyRequests": true,
	"GrpcErr": true, "GrpcDomainErr": true,
}

// successEmitters 是會寫出 code==0 的那幾支（引數為 0 時）。71 個成功出口全落在這裡。
var successEmitters = map[string]bool{"OK": true, "OKProto": true, "Page": true, "GrpcResult": true}

type emitSite struct {
	file string
	line int
	fn   string
	emit string
}

type aliasState int

const (
	aliasAbsent    aliasState = iota // 這個檔案根本沒 import response ⇒ 零呼叫點，正常
	aliasUsable                      // 具名（含別名）import ⇒ 呼叫點歸屬得了
	aliasAmbiguous                   // dot-import／blank import ⇒ 歸屬不了，必須 FAIL
)

// responseAlias 從 import 宣告解析出 internal/response 在這個檔案裡的本地名。
//
// 舊 gate 硬寫識別字 "response"，於是 `rsp "…/response"` 讓整個檔案失明（m1，
// 而且那是純風格 refactor，完全不需要惡意）。這裡改成從 import 宣告解析，
// 別名再怎麼取都數得到。
//
// 三態而不是兩態：「沒 import」與「import 了但歸屬不了」必須分開 —— 前者是正常的
// （invoice_dto.go 只放型別），後者是真的看不到，只有後者該 FAIL。兩者混成一個
// false 就會退化成「看不到 ⇒ 一律 FAIL」的假警報或「一律放行」的 fail-OPEN。
func responseAlias(f *ast.File) (string, aliasState) {
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != responsePkgPath {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				return "", aliasAmbiguous
			}
			return imp.Name.Name, aliasUsable
		}
		return "response", aliasUsable
	}
	return "", aliasAbsent
}

func collectInvoiceEmitSites(t *testing.T) []emitSite {
	t.Helper()
	fset := token.NewFileSet()
	files := invoiceHandlerFiles(t)
	var sites []emitSite
	for _, path := range files {
		f := premiseParseFile(t, fset, path)
		alias, state := responseAlias(f)
		switch state {
		case aliasAbsent:
			continue // 只放型別／DTO 的檔案，零呼叫點
		case aliasAmbiguous:
			// dot-import／blank import 會讓呼叫點無從歸屬 ⇒ FAIL 不 SKIP。
			t.Fatalf("%s 以 dot／blank 方式 import %s —— 呼叫點無從歸屬，"+
				"必須由人決定這個檔案要怎麼被涵蓋", path, responsePkgPath)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok || id.Name != alias || !responseEmitters[sel.Sel.Name] {
					return true
				}
				p := fset.Position(call.Pos())
				sites = append(sites, emitSite{file: path, line: p.Line, fn: fn.Name.Name, emit: sel.Sel.Name})
				return true
			})
		}
	}
	return sites
}

// 期望值 pin。改了要有人解釋為什麼（新增／刪除端點都會動它）。
// 2026-08-31 實測：57 支 OK ＋ 14 支 Page ＝ 71 個成功出口；71 支 Biz 守門。
//
// 2026-09-02（#3019）：端點 30／47 的分頁 data 多帶一個 shipmentStatusDegraded，
// 而 response.Page 只組四個欄位 ⇒ 兩支改用 response.OK ＋ 內嵌 response.PageData 的
// InvoiceDocumentsPageData。**成功出口總數不變（71）、Biz 守門不變（71）**，
// 只是兩個位點從 Page 那一格移到 OK 那一格：57+2=59、14−2=12。
const (
	wantOKSites   = 60
	wantPageSites = 12
	wantBizGuards = 72
)

func TestInvoiceSuccessEmitSitesAreAllCarrierGuarded(t *testing.T) {
	sites := collectInvoiceEmitSites(t)
	if len(sites) == 0 {
		t.Fatalf("在 %v 找不到任何 internal/response 呼叫點——"+
			"invoice handler 被搬走／改名了，本檢查已經在守一個不存在的東西", invoiceHandlerFiles(t))
	}

	byEmitter := map[string][]emitSite{}
	for _, s := range sites {
		byEmitter[s.emit] = append(byEmitter[s.emit], s)
	}

	if got := len(byEmitter["OK"]); got != wantOKSites {
		t.Errorf("response.OK 呼叫點 %d 個，pin 是 %d —— 端點有增減就更新 pin 並在 PR 說明", got, wantOKSites)
	}
	if got := len(byEmitter["Page"]); got != wantPageSites {
		t.Errorf("response.Page 呼叫點 %d 個，pin 是 %d", got, wantPageSites)
	}
	if got := len(byEmitter["Biz"]); got != wantBizGuards {
		t.Errorf("response.Biz 守門 %d 個，pin 是 %d —— 守門被刪掉時這裡會先出聲，"+
			"比等 gateway_bizcode_swallowed_total 在流量裡響早得多", got, wantBizGuards)
	}

	// 每一個成功出口都必須是 emitter 矩陣蓋過的那幾支。
	var uncovered []string
	for _, s := range sites {
		if !responseEmitters[s.emit] {
			uncovered = append(uncovered, fmt.Sprintf("%s:%d %s → response.%s", s.file, s.line, s.fn, s.emit))
		}
	}
	if len(uncovered) > 0 {
		t.Errorf("下列呼叫點的出口不在 emitter 矩陣母體內：\n  %s", strings.Join(uncovered, "\n  "))
	}

	// 逐點跑一次它用的那支出口：idle 必須是一般成功形狀，armed 必須被改寫成業務碼。
	// 這一段讓 57／14 是被跑出來的數字，而不是被宣稱的。
	okRun, pageRun := 0, 0
	for _, s := range sites {
		if !successEmitters[s.emit] {
			continue
		}
		name := fmt.Sprintf("%s:%d/%s/%s", s.file, s.line, s.fn, s.emit)
		switch s.emit {
		case "OK":
			okRun++
			assertSuccessSiteShape(t, name, func(c *gin.Context) { response.OK(c, gin.H{"probe": s.fn}) })
		case "Page":
			pageRun++
			assertPageSiteShape(t, name, func(c *gin.Context) {
				response.Page(c, []gin.H{{"probe": s.fn}}, "1", "1", "20")
			})
		default:
			t.Errorf("%s 用了未預期的成功出口 response.%s", name, s.emit)
		}
	}
	if okRun != wantOKSites || pageRun != wantPageSites {
		t.Errorf("逐點實跑數 OK=%d Page=%d，與枚舉數 %d/%d 不符", okRun, pageRun, wantOKSites, wantPageSites)
	}
	t.Logf("invoice handler %v：成功出口 %d 個（OK %d ＋ Page %d）逐點實跑，Biz 守門 %d 個",
		invoiceHandlerFiles(t), okRun+pageRun, okRun, pageRun, len(byEmitter["Biz"]))
}

const (
	probeArmMethod = "/hawkr.invoice.InvoiceService/Probe"
	probeArmCode   = int32(9331)
	probeArmMsg    = "憑證驗證失敗"
)

func runSiteProbe(t *testing.T, arm bool, fn gin.HandlerFunc) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	r.GET("/e", func(c *gin.Context) {
		if arm {
			bizcarrier.Arm(c.Request.Context(), probeArmMethod, probeArmCode, probeArmMsg)
		}
		fn(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/e", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal %q: %v", w.Body.String(), err)
	}
	return body
}

func assertSuccessSiteShape(t *testing.T, name string, fn gin.HandlerFunc) {
	t.Helper()
	idle := runSiteProbe(t, false, fn)
	if idle["code"] != float64(0) || idle["message"] != "success" {
		t.Errorf("%s idle 形狀變了：%v", name, idle)
	}
	armed := runSiteProbe(t, true, fn)
	if armed["code"] != float64(probeArmCode) || armed["message"] != probeArmMsg {
		t.Errorf("%s armed 時沒有被改寫（假成功）：%v", name, armed)
	}
}

func assertPageSiteShape(t *testing.T, name string, fn gin.HandlerFunc) {
	t.Helper()
	idle := runSiteProbe(t, false, fn)
	if idle["code"] != float64(0) || idle["message"] != "success" {
		t.Errorf("%s idle 形狀變了：%v", name, idle)
	}
	data, ok := idle["data"].(map[string]any)
	if !ok {
		t.Fatalf("%s 分頁 data 不是物件：%v", name, idle["data"])
	}
	// 四欄逐欄檢查：前端讀的是通用的 data.pageSize，少一欄就是 undefined。
	for _, k := range []string{"records", "total", "page", "pageSize"} {
		if _, ok := data[k]; !ok {
			t.Errorf("%s 分頁少了 data.%s：%v", name, k, data)
		}
	}
	armed := runSiteProbe(t, true, fn)
	if armed["code"] != float64(probeArmCode) {
		t.Errorf("%s armed 時沒有被改寫（假成功）：%v", name, armed)
	}
	adata, ok := armed["data"].(map[string]any)
	if !ok {
		t.Fatalf("%s 改寫後 data 不是物件：%v", name, armed["data"])
	}
	for _, k := range []string{"records", "total", "page", "pageSize"} {
		if _, ok := adata[k]; !ok {
			t.Errorf("%s 改寫後分頁少了 data.%s —— 前端讀到 undefined：%v", name, k, adata)
		}
	}
}
