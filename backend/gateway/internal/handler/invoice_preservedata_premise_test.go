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

const (
	preserveDataInterceptorPath = "../grpc/envelope_guard_interceptor.go"
	preserveDataMapName         = "envelopeCodePreserveData"
	responsePkgPath             = "github.com/Will413028/hawkr/gateway/internal/response"
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

var responseEmitters = map[string]bool{
	"OK": true, "OKProto": true, "Page": true, "GrpcResult": true,
	"Biz": true, "BizDeny": true, "BizStatus": true, "AbortBizStatus": true,
	"BizStatusData": true, "Err": true, "BadRequest": true, "BadRequestFields": true,
	"Unauthorized": true, "AbortUnauthorized": true, "Forbidden": true, "NotFound": true,
	"ServiceUnavailable": true, "PhoneVerificationRequired": true, "TooManyRequests": true,
	"GrpcErr": true, "GrpcDomainErr": true,
}

var successEmitters = map[string]bool{"OK": true, "OKProto": true, "Page": true, "GrpcResult": true}

type emitSite struct {
	file string
	line int
	fn   string
	emit string
}

type aliasState int

const (
	aliasAbsent aliasState = iota
	aliasUsable
	aliasAmbiguous
)

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
			continue
		case aliasAmbiguous:
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

	var uncovered []string
	for _, s := range sites {
		if !responseEmitters[s.emit] {
			uncovered = append(uncovered, fmt.Sprintf("%s:%d %s → response.%s", s.file, s.line, s.fn, s.emit))
		}
	}
	if len(uncovered) > 0 {
		t.Errorf("下列呼叫點的出口不在 emitter 矩陣母體內：\n  %s", strings.Join(uncovered, "\n  "))
	}

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
