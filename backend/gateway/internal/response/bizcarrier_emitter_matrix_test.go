package response

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	"github.com/Will413028/hawkr/go-pkg/envelope"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------------------------------------------------------------------
// 「行為零改變」的證據 —— 這一份有鑑別力，上一份沒有
// ---------------------------------------------------------------------------
//
// 2026-08-31 的第一版證據是一支叫 TestSuccessShapesUnchangedWhenCarrierIdle 的測試，
// 函式名自己就寫著它只跑 idle。實測：把 bizcarrier 的取碼函式改成恆回 false（＝整個
// 機制關掉），`go build` exit=0 而**那支測試照樣 PASS** —— 它分辨不出「機制在」與
// 「機制不在」。同一個變異讓 9 支 bypass 測試轉紅，證明變異本身是真的。
//
// 本檔的三件事讓它有鑑別力：
//
//  1. **矩陣的母體由 AST 決定，不是由我記得幾支**（TestEmitterTableCoversEveryExportedEmitter）。
//     新增一支 exported 出口而沒進表 ⇒ FAIL。
//  2. **每一支出口在 idle 與 armed 兩種狀態下各跑一次**。idle 要與改動前逐位元相同；
//     armed 要與手寫 Biz 逐位元相同（零碼出口）或完全不變（非零出口）。
//  3. **非零出口另外對帳逃逸計數器**：忘了守衛的出口不會改變 body，只會讓
//     Middleware 收尾把它記成「繞過 response 層」。不查那個計數器，漏守衛就是靜默的。
//
// ⚠️ 母體不含 SetRateLimitHeaders：它只寫 header 不寫 body，不是出口。這是本檔唯一
// 的豁免，寫在 nonEmitters 裡並附理由；豁免清單長出第二筆時要有人回答為什麼。

// runEmit 起一個掛了 bizcarrier.Middleware 的最小 engine，跑一次 fn。
func runEmit(t *testing.T, fn gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	r.GET("/e", fn)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/e", nil))
	return w
}

// runEmitArmed 同上，但在 handler 跑之前先掛一個未消化的業務碼。
func runEmitArmed(t *testing.T, code int32, msg string, fn gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	return runEmit(t, func(c *gin.Context) {
		bizcarrier.Arm(c.Request.Context(), armMethod, code, msg)
		fn(c)
	})
}

const (
	armMethod = "/hawkr.invoice.InvoiceService/HealthCheckProviderAccountForAdmin"
	armCode   = int32(9331)
	armMsg    = "憑證驗證失敗"
)

func assertIdentical(t *testing.T, name string, got, want *httptest.ResponseRecorder) {
	t.Helper()
	if got.Code != want.Code {
		t.Errorf("%s: status %d != %d", name, got.Code, want.Code)
	}
	if got.Header().Get("Content-Type") != want.Header().Get("Content-Type") {
		t.Errorf("%s: content-type %q != %q", name, got.Header().Get("Content-Type"), want.Header().Get("Content-Type"))
	}
	if got.Body.String() != want.Body.String() {
		t.Errorf("%s: body 不同\n  got:  %s\n  want: %s", name, got.Body.String(), want.Body.String())
	}
}

func bodyCode(t *testing.T, w *httptest.ResponseRecorder) (int32, bool) {
	t.Helper()
	var env struct {
		Code *int32 `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code == nil {
		return 0, false
	}
	return *env.Code, true
}

// ---------------------------------------------------------------------------
// 出口矩陣
// ---------------------------------------------------------------------------

type sample struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Flag bool   `json:"flag"`
}

var payload = sample{ID: "2685914682", Name: "測試店", Flag: false}

// emitterCase 是一支出口在一組引數下的完整規格。
//
// declaredCode 是這支出口在這組引數下**實際寫進 envelope 的 Code**（nonZeroDownstream
// 代表「碼由下游決定但恆非零」，只有 GrpcErr 家族適用）。測試會實跑並比對，宣告錯
// 就是 FAIL —— 這是「宣告的碼 == 真的寫出去的碼」唯一的保證，其餘都只是註解。
type emitterCase struct {
	fn       string // 對應的 exported 函式名，供 AST 覆蓋率檢查對帳
	name     string
	declared int32
	invoke   gin.HandlerFunc
	// legacy 是「改動前那一版」的逐位元行為：轉發型出口直呼 pkg/envelope，
	// Gateway 專屬出口手寫同一份 c.JSON。idle 狀態下新舊必須一致。
	legacy gin.HandlerFunc
	// rewriteData 是 declared==0 時，改寫成 Biz 應該帶走的 data。
	rewriteData interface{}
}

const nonZeroDownstream int32 = -1

// legacyBiz／legacyBizDeny／legacyBizStatus 是三個 Gateway 專屬出口在加上守衛**之前**
// 的函式體，逐行照抄。它們是 idle 對照組 —— 沒有它們，「行為零改變」就只是宣稱。
func legacyBiz(c *gin.Context, code int32, message string, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Code: code, Message: message, Data: data, TraceID: envelope.ExtractTraceID(c)})
}

func legacyBizDeny(c *gin.Context, code int32, message string) {
	c.JSON(http.StatusConflict, Envelope{Code: code, Message: message, MessageZh: message, TraceID: envelope.ExtractTraceID(c)})
}

func legacyBizStatus(c *gin.Context, code int32, message string, data interface{}) {
	c.JSON(StatusForBizCode(code), Envelope{
		Code: code, Message: message, MessageZh: message, Data: data, TraceID: envelope.ExtractTraceID(c),
	})
}

func legacyBadRequestFields(c *gin.Context, message string, fields []FieldError) {
	c.JSON(http.StatusBadRequest, Envelope{
		Code: http.StatusBadRequest, Message: message,
		Data: gin.H{"fieldErrors": fields}, TraceID: envelope.ExtractTraceID(c),
	})
}

var fieldErrs = []FieldError{{Field: "contactPhone", Message: "必填"}}

func emitterCases() []emitterCase {
	protoMsg := timestamppb.New(time.Unix(1756600000, 0).UTC())
	return []emitterCase{
		// ---- 會寫出 code==0 的出口（carrier 必須改寫）----
		{fn: "OK", name: "OK", declared: 0,
			invoke:      func(c *gin.Context) { OK(c, payload) },
			legacy:      func(c *gin.Context) { envelope.OK(c, payload) },
			rewriteData: payload},
		{fn: "OKProto", name: "OKProto", declared: 0,
			invoke:      func(c *gin.Context) { OKProto(c, protoMsg) },
			legacy:      func(c *gin.Context) { envelope.OKProto(c, protoMsg) },
			rewriteData: ProtoToRaw(protoMsg)},
		{fn: "Page", name: "Page", declared: 0,
			invoke:      func(c *gin.Context) { Page(c, []sample{payload}, "1", "1", "20") },
			legacy:      func(c *gin.Context) { envelope.Page(c, []sample{payload}, "1", "1", "20") },
			rewriteData: PageData{Records: []sample{payload}, Total: "1", Page: "1", PageSize: "20"}},
		{fn: "GrpcResult", name: "GrpcResult(code=0)", declared: 0,
			invoke:      func(c *gin.Context) { GrpcResult(c, 0, "success", payload) },
			legacy:      func(c *gin.Context) { envelope.GrpcResult(c, 0, "success", payload) },
			rewriteData: payload},
		// 🔴 第十種繞道：已存在的 exported helper，零新程式碼。
		// handler/checkout.go:553/643/651 用的是同一支，只是那三處今天送的是非零碼。
		{fn: "Biz", name: "Biz(code=0) —— 第十種繞道", declared: 0,
			invoke:      func(c *gin.Context) { Biz(c, 0, "success", payload) },
			legacy:      func(c *gin.Context) { legacyBiz(c, 0, "success", payload) },
			rewriteData: payload},
		{fn: "BizDeny", name: "BizDeny(code=0)", declared: 0,
			invoke:      func(c *gin.Context) { BizDeny(c, 0, "success") },
			legacy:      func(c *gin.Context) { legacyBizDeny(c, 0, "success") },
			rewriteData: nil},
		{fn: "BizStatus", name: "BizStatus(code=0)", declared: 0,
			invoke:      func(c *gin.Context) { BizStatus(c, 0, "success") },
			legacy:      func(c *gin.Context) { legacyBizStatus(c, 0, "success", nil) },
			rewriteData: nil},
		{fn: "AbortBizStatus", name: "AbortBizStatus(code=0)", declared: 0,
			invoke:      func(c *gin.Context) { AbortBizStatus(c, 0, "success") },
			legacy:      func(c *gin.Context) { c.Abort(); legacyBizStatus(c, 0, "success", nil) },
			rewriteData: nil},
		{fn: "BizStatusData", name: "BizStatusData(code=0)", declared: 0,
			invoke:      func(c *gin.Context) { BizStatusData(c, 0, "success", payload) },
			legacy:      func(c *gin.Context) { legacyBizStatus(c, 0, "success", payload) },
			rewriteData: payload},
		{fn: "Err", name: "Err(code=0)", declared: 0,
			invoke:      func(c *gin.Context) { Err(c, http.StatusOK, 0, "success") },
			legacy:      func(c *gin.Context) { envelope.Err(c, http.StatusOK, 0, "success") },
			rewriteData: nil},

		// ---- 恆非零的出口（carrier 不得改寫，但必須留下痕跡）----
		{fn: "BadRequest", name: "BadRequest", declared: 400,
			invoke: func(c *gin.Context) { BadRequest(c, "invalid") },
			legacy: func(c *gin.Context) { envelope.BadRequest(c, "invalid") }},
		{fn: "BadRequestFields", name: "BadRequestFields", declared: 400,
			invoke: func(c *gin.Context) { BadRequestFields(c, "invalid", fieldErrs) },
			legacy: func(c *gin.Context) { legacyBadRequestFields(c, "invalid", fieldErrs) }},
		{fn: "Unauthorized", name: "Unauthorized", declared: 401,
			invoke: func(c *gin.Context) { Unauthorized(c) },
			legacy: func(c *gin.Context) { envelope.Unauthorized(c) }},
		{fn: "AbortUnauthorized", name: "AbortUnauthorized", declared: 401,
			invoke: func(c *gin.Context) { AbortUnauthorized(c, "no token") },
			legacy: func(c *gin.Context) { envelope.AbortUnauthorized(c, "no token") }},
		{fn: "Forbidden", name: "Forbidden", declared: 403,
			invoke: func(c *gin.Context) { Forbidden(c, "denied") },
			legacy: func(c *gin.Context) { envelope.Forbidden(c, "denied") }},
		{fn: "AbortForbidden", name: "AbortForbidden", declared: 403,
			invoke: func(c *gin.Context) { AbortForbidden(c, "denied") },
			legacy: func(c *gin.Context) { envelope.Forbidden(c, "denied") }},
		{fn: "NotFound", name: "NotFound", declared: 404,
			invoke: func(c *gin.Context) { NotFound(c, "missing") },
			legacy: func(c *gin.Context) { envelope.NotFound(c, "missing") }},
		{fn: "ServiceUnavailable", name: "ServiceUnavailable", declared: 503,
			invoke: func(c *gin.Context) { ServiceUnavailable(c, "invoice") },
			legacy: func(c *gin.Context) { envelope.ServiceUnavailable(c, "invoice") }},
		{fn: "TooManyRequests", name: "TooManyRequests", declared: 429,
			invoke: func(c *gin.Context) { TooManyRequests(c, "too many") },
			legacy: func(c *gin.Context) {
				SetRateLimitHeaders(c, 0, 0, ConnCapRetryAfter)
				envelope.Err(c, http.StatusTooManyRequests, 429, "too many")
			}},
		{fn: "GrpcResult", name: "GrpcResult(code=4092)", declared: 4092,
			invoke: func(c *gin.Context) { GrpcResult(c, 4092, "訂單已建立", payload) },
			legacy: func(c *gin.Context) { envelope.GrpcResult(c, 4092, "訂單已建立", payload) }},
		{fn: "Biz", name: "Biz(code=9310)", declared: 9310,
			invoke: func(c *gin.Context) { Biz(c, 9310, "重複申訴", payload) },
			legacy: func(c *gin.Context) { legacyBiz(c, 9310, "重複申訴", payload) }},
		{fn: "BizDeny", name: "BizDeny(code=9262)", declared: 9262,
			invoke: func(c *gin.Context) { BizDeny(c, 9262, "能力不足") },
			legacy: func(c *gin.Context) { legacyBizDeny(c, 9262, "能力不足") }},
		{fn: "BizStatus", name: "BizStatus(code=9262)", declared: 9262,
			invoke: func(c *gin.Context) { BizStatus(c, 9262, "能力不足") },
			legacy: func(c *gin.Context) { legacyBizStatus(c, 9262, "能力不足", nil) }},
		{fn: "AbortBizStatus", name: "AbortBizStatus(code=9262)", declared: 9262,
			invoke: func(c *gin.Context) { AbortBizStatus(c, 9262, "能力不足") },
			legacy: func(c *gin.Context) { c.Abort(); legacyBizStatus(c, 9262, "能力不足", nil) }},
		{fn: "BizStatusData", name: "BizStatusData(code=9266)", declared: 9266,
			invoke: func(c *gin.Context) { BizStatusData(c, 9266, "版本衝突", payload) },
			legacy: func(c *gin.Context) { legacyBizStatus(c, 9266, "版本衝突", payload) }},
		{fn: "Err", name: "Err(code=409)", declared: 409,
			invoke: func(c *gin.Context) { Err(c, http.StatusConflict, 409, "conflict") },
			legacy: func(c *gin.Context) { envelope.Err(c, http.StatusConflict, 409, "conflict") }},

		// ---- 碼由下游決定但恆非零 ----
		{fn: "GrpcErr", name: "GrpcErr(FailedPrecondition)", declared: nonZeroDownstream,
			invoke: func(c *gin.Context) {
				GrpcErr(c, "invoice", grpcstatus.Error(codes.FailedPrecondition, "[9332] provider 不可用"))
			},
			legacy: func(c *gin.Context) {
				c.JSON(http.StatusConflict, Envelope{Code: 9332, Message: "provider 不可用", MessageZh: "provider 不可用", TraceID: envelope.ExtractTraceID(c)})
			}},
		{fn: "GrpcDomainErr", name: "GrpcDomainErr(NotFound)", declared: nonZeroDownstream,
			invoke: func(c *gin.Context) { GrpcDomainErr(c, "invoice", grpcstatus.Error(codes.NotFound, "not found")) },
			legacy: func(c *gin.Context) { envelope.GrpcErr(c, "invoice", grpcstatus.Error(codes.NotFound, "not found")) }},
	}
}

// ---------------------------------------------------------------------------
// ① 母體：表必須蓋住每一支 exported 出口
// ---------------------------------------------------------------------------

// nonEmitters 是「第一個參數是 *gin.Context 但不寫 body」的例外。每一筆都要附理由。
//
// ⚠️ 這是本檔唯一的豁免清單。它與舊 AST gate 的 fail-OPEN 不同：這裡的預設是
// **在名單外就必須進矩陣**（找不到＝FAIL），而不是「看不到就放行」。
var nonEmitters = map[string]string{
	"SetRateLimitHeaders": "只寫 Retry-After / X-RateLimit-* header，不寫 body，沒有 envelope code 可言",
}

func TestEmitterTableCoversEveryExportedEmitter(t *testing.T) {
	fset := token.NewFileSet()
	covered := map[string]bool{}
	for _, cse := range emitterCases() {
		covered[cse.fn] = true
	}

	// 🔴 掃**整個 package 目錄**，不是寫死的檔名清單。
	//
	// 這一版是被變異抓出來的：第一版寫死 []string{"response.go", "bizhttp.go"}，
	// 於是「在新檔案裡新增一支 exported 出口」完全看不到 —— 實測加一支
	// MutationProbeEmitter 到 mutation_probe_emitter.go，本檢查照樣 PASS。
	// 那正是舊 AST gate 的病（看不到就當安全）在我自己寫的檢查裡復發。
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("列 package 檔案失敗：%v", err)
	}
	var srcPaths []string
	for _, p := range paths {
		if !strings.HasSuffix(p, "_test.go") {
			srcPaths = append(srcPaths, p)
		}
	}
	if len(srcPaths) == 0 {
		t.Fatal("package 目錄內找不到任何非測試 .go 檔——目標消失，FAIL 而非 SKIP")
	}

	var declared []string
	for _, path := range srcPaths {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			// 目標讀不到一律 FAIL 不 SKIP：靜默跳過等於把檢查關掉。
			t.Fatalf("解析 %s 失敗：%v", path, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}
			if len(fn.Type.Params.List) == 0 {
				continue
			}
			star, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			sel, ok := star.X.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Context" {
				continue
			}
			declared = append(declared, fn.Name.Name)
		}
	}
	sort.Strings(declared)

	var missing []string
	for _, name := range declared {
		if covered[name] {
			continue
		}
		if _, exempt := nonEmitters[name]; exempt {
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		t.Fatalf("下列 exported 出口不在 emitterCases 矩陣內：%v\n"+
			"每一支寫 envelope 的出口都必須在矩陣裡跑過 idle／armed 兩種狀態，"+
			"否則「它有沒有查 carrier」就沒有任何東西在守。\n"+
			"確定它不寫 body 的話，加進 nonEmitters 並寫下理由。", missing)
	}

	// 反向：矩陣裡不得有已經不存在的函式（表腐化 ⇒ 覆蓋率是假的）。
	declaredSet := map[string]bool{}
	for _, n := range declared {
		declaredSet[n] = true
	}
	for fn := range covered {
		if !declaredSet[fn] {
			t.Errorf("矩陣列了不存在的出口 %q", fn)
		}
	}
	t.Logf("掃了 %d 個原始檔：exported 出口 %d 支，矩陣 %d 個 case，豁免 %d 支",
		len(srcPaths), len(declared), len(emitterCases()), len(nonEmitters))
}

// ---------------------------------------------------------------------------
// ② idle：逐位元零改變
// ---------------------------------------------------------------------------

func TestEmitterMatrix_IdleShapesUnchanged(t *testing.T) {
	for _, cse := range emitterCases() {
		cse := cse
		t.Run(cse.name, func(t *testing.T) {
			assertIdentical(t, cse.name, runEmit(t, cse.invoke), runEmit(t, cse.legacy))
		})
	}
}

// ③ 宣告的碼 == 真的寫出去的碼。矩陣其餘所有斷言都建立在這一條上。
func TestEmitterMatrix_DeclaredCodeMatchesBody(t *testing.T) {
	for _, cse := range emitterCases() {
		cse := cse
		t.Run(cse.name, func(t *testing.T) {
			got, ok := bodyCode(t, runEmit(t, cse.invoke))
			if !ok {
				t.Fatalf("%s 沒有寫出可解析的 envelope（body=%q）", cse.name, runEmit(t, cse.invoke).Body.String())
			}
			if cse.declared == nonZeroDownstream {
				if got == 0 {
					t.Fatalf("%s 宣告恆非零，實際寫出 code=0", cse.name)
				}
				return
			}
			if got != cse.declared {
				t.Fatalf("%s 宣告 code=%d，實際寫出 code=%d —— 宣告錯了，守衛就守錯了東西",
					cse.name, cse.declared, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ④ armed：零碼出口必須被改寫，且與手寫 Biz 逐位元相同
// ---------------------------------------------------------------------------
//
// 🔴 這一組就是上一版缺的鑑別力。把 bizcarrier.Emit 的改寫分支神經切斷（恆回 false）
// ⇒ 每一個 case 都會轉紅，因為 body 還是 code:0。

func TestEmitterMatrix_ZeroCodeEmittersAreRewritten(t *testing.T) {
	n := 0
	for _, cse := range emitterCases() {
		if cse.declared != 0 {
			continue
		}
		n++
		cse := cse
		t.Run(cse.name, func(t *testing.T) {
			got := runEmitArmed(t, armCode, armMsg, cse.invoke)
			want := runEmit(t, func(c *gin.Context) { Biz(c, armCode, armMsg, cse.rewriteData) })
			assertIdentical(t, cse.name+" 改寫 vs 手寫 Biz", got, want)

			code, ok := bodyCode(t, got)
			if !ok || code != armCode {
				t.Fatalf("%s 在 armed 狀態下仍寫出 code=%v —— 這正是端點 44 的假成功", cse.name, code)
			}
		})
	}
	if n == 0 {
		t.Fatal("矩陣裡一個零碼出口都沒有：不可能，母體錯了")
	}
	t.Logf("零碼出口 %d 支，全部驗過 armed 改寫", n)
}

// ⑤ armed：非零出口不得被改寫，**且不得被記成逃逸**。
//
// 後半是這一組的重點。忘了守衛的非零出口不會改變 body（測 body 抓不到），只會讓
// Middleware 收尾把它記成「繞過 response 層」——不查那個計數器，漏守衛就是靜默的。
func TestEmitterMatrix_NonZeroEmittersNeverGoUndetected(t *testing.T) {
	label := func() float64 {
		return testutil.ToFloat64(bizcarrier.EscapedCounterFor(armCode, "/e"))
	}
	for _, cse := range emitterCases() {
		if cse.declared == 0 {
			continue
		}
		cse := cse
		t.Run(cse.name, func(t *testing.T) {
			before := label()
			got := runEmitArmed(t, armCode, armMsg, cse.invoke)
			assertIdentical(t, cse.name+" armed 不得改寫", got, runEmit(t, cse.invoke))
			if after := label(); after != before {
				t.Fatalf("%s 在 armed 狀態下被記成「繞過 internal/response」（%v → %v）——"+
					"代表它沒有呼叫 guard／guardNonSuccess 留下痕跡", cse.name, before, after)
			}
		})
	}
}

// ⑥ 逃逸偵測本身要有鑑別力：真的繞過 response 層時計數器必須跳。
//
// 沒有這一支，⑤的「計數器沒動」可能只是因為那個計數器永遠不會動。
func TestEscapeCounterActuallyFiresOnRealBypass(t *testing.T) {
	before := testutil.ToFloat64(bizcarrier.EscapedCounterFor(armCode, "/e"))
	runEmitArmed(t, armCode, armMsg, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"}) // 完全繞過 internal/response
	})
	if after := testutil.ToFloat64(bizcarrier.EscapedCounterFor(armCode, "/e")); after != before+1 {
		t.Fatalf("真的繞過 response 層時逃逸計數器沒有跳（%v → %v）——"+
			"那⑤的「沒跳」就什麼都證明不了", before, after)
	}
}

// ---------------------------------------------------------------------------
// ⑦ GrpcErr 家族恆非零（guardNonSuccess 的全部依據）
// ---------------------------------------------------------------------------

func TestGrpcErrNeverWritesZeroCode(t *testing.T) {
	allCodes := []codes.Code{
		codes.OK, codes.Canceled, codes.Unknown, codes.InvalidArgument, codes.DeadlineExceeded,
		codes.NotFound, codes.AlreadyExists, codes.PermissionDenied, codes.ResourceExhausted,
		codes.FailedPrecondition, codes.Aborted, codes.OutOfRange, codes.Unimplemented,
		codes.Internal, codes.Unavailable, codes.DataLoss, codes.Unauthenticated,
	}
	type probe struct {
		name string
		err  error
	}
	probes := make([]probe, 0, 2*len(allCodes)+2)
	for _, c := range allCodes {
		probes = append(probes,
			probe{"status/" + c.String(), grpcstatus.Error(c, "boom")},
			// 帶 "[NNNN] " 前綴：走登錄表還原路徑（writeBizCodeEnvelopeMapped）。
			probe{"status/" + c.String() + "/bizprefix", grpcstatus.Error(c, "[9262] 能力不足")},
		)
	}
	probes = append(probes,
		probe{"nil-error", nil},
		probe{"plain-error", errors.New("not a grpc status")},
	)

	for _, p := range probes {
		p := p
		t.Run(p.name, func(t *testing.T) {
			for _, fn := range []struct {
				name string
				emit func(c *gin.Context)
			}{
				{"GrpcErr", func(c *gin.Context) { GrpcErr(c, "invoice", p.err) }},
				{"GrpcDomainErr", func(c *gin.Context) { GrpcDomainErr(c, "invoice", p.err) }},
			} {
				w := runEmit(t, fn.emit)
				code, ok := bodyCode(t, w)
				if !ok {
					t.Fatalf("%s(%s) 沒有寫出可解析的 envelope：%q", fn.name, p.name, w.Body.String())
				}
				if code == 0 {
					t.Fatalf("%s(%s) 寫出 code=0 —— guardNonSuccess 的前提（恆非零）不成立，"+
						"那條路必須改成交出實際的 code", fn.name, p.name)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ⑧ payload 形狀矩陣：零值／空集合／nil 是「加一層包裝會改變位元」的唯一可能來源
// ---------------------------------------------------------------------------

func TestEmitterMatrix_PayloadKindsUnchanged(t *testing.T) {
	kinds := []struct {
		name string
		data interface{}
	}{
		{"nil", nil},
		{"zero-struct", sample{}},
		{"empty-slice", []sample{}},
		{"nil-slice", []sample(nil)},
		{"map", gin.H{"a": 1, "b": "x", "c": false}},
		{"raw-json", json.RawMessage(`{"k":0}`)},
		{"string", ""},
		{"number-zero", 0},
		{"bool-false", false},
	}
	for _, k := range kinds {
		k := k
		t.Run("OK/"+k.name, func(t *testing.T) {
			assertIdentical(t, "OK/"+k.name,
				runEmit(t, func(c *gin.Context) { OK(c, k.data) }),
				runEmit(t, func(c *gin.Context) { envelope.OK(c, k.data) }))
		})
		t.Run("Page/"+k.name, func(t *testing.T) {
			assertIdentical(t, "Page/"+k.name,
				runEmit(t, func(c *gin.Context) { Page(c, k.data, "0", "1", "20") }),
				runEmit(t, func(c *gin.Context) { envelope.Page(c, k.data, "0", "1", "20") }))
		})
		t.Run("GrpcResult/"+k.name, func(t *testing.T) {
			assertIdentical(t, "GrpcResult/"+k.name,
				runEmit(t, func(c *gin.Context) { GrpcResult(c, 0, "success", k.data) }),
				runEmit(t, func(c *gin.Context) { envelope.GrpcResult(c, 0, "success", k.data) }))
		})
	}
}

// ---------------------------------------------------------------------------
// ⑨ 作用域：跨 method 的失敗不會被抹掉（A1 在 response 層的端到端證據）
// ---------------------------------------------------------------------------

func TestZeroCodeEmitIsBlockedByAnotherMethodsPendingFailure(t *testing.T) {
	const otherMethod = "/hawkr.seller.SellerService/GetSellerSimple"
	w := runEmit(t, func(c *gin.Context) {
		bizcarrier.Arm(c.Request.Context(), armMethod, armCode, armMsg) // RPC#1 失敗
		bizcarrier.Disarm(c.Request.Context(), otherMethod)             // RPC#2 成功（別支）
		OK(c, payload)                                                  // handler 照常回成功
	})
	code, _ := bodyCode(t, w)
	if code != armCode {
		t.Fatalf("別支 RPC 的成功抹掉了 %d，body code=%d —— 這正是根因 A 搬家後的住所", armCode, code)
	}
	// 而「同一支重試成功」仍然要解除。
	w2 := runEmit(t, func(c *gin.Context) {
		bizcarrier.Arm(c.Request.Context(), armMethod, armCode, armMsg)
		bizcarrier.Disarm(c.Request.Context(), armMethod)
		OK(c, payload)
	})
	if code2, _ := bodyCode(t, w2); code2 != 0 {
		t.Fatalf("同一支 method 重試成功之後應解除，body code=%d", code2)
	}
	if !strings.Contains(w2.Body.String(), `"message":"success"`) {
		t.Errorf("解除之後應是一般成功回應，得到 %s", w2.Body.String())
	}
}
