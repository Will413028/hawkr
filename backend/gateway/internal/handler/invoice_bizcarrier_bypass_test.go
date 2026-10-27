package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/gin-gonic/gin"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// ---------------------------------------------------------------------------
// 這個檔案守的是什麼
// ---------------------------------------------------------------------------
//
// 在 bizcarrier 之前，「handler 有沒有記得檢查 resp.GetCode()」由一道 AST gate 守
// （TestInvoiceHandlersGuardPreserveDataCodes，掃 handler 原始碼找呼叫點之後有沒有
// GetCode 檢查）。它被 9 種寫法繞過，全部 go build exit=0、gofmt 過、既有測試全綠。
// 兩個根因都是「原始碼形狀」這個判定材料本身的極限：
//
//   - per-site fail-OPEN：`if len(after) == 0 { continue }` —— 「這個呼叫點之後找不到
//     成功出口」被當成安全的證明，而它同樣可以只是「我看不到而已」。
//   - 有 token ≠ 真的擋住：AST 只問得出「子樹裡存不存在一個 response.Biz 節點」，
//     問不出「非零碼到不了成功出口」——後者是路徑性質，靜態判不了。
//
// 下面 9 支 handler 逐一重現那 9 種寫法。**它們全都是真的沒有守門**（或守門形同虛設），
// 放在這裡就是要證明：判定材料換成執行期事實之後，寫成什麼形狀都不影響結論。
//
// 每一支的斷言都一樣：HTTP 200 ＋ envelope code 9331 ＋ data 原樣透傳。
//
// ⚠️ 這些 handler 只存在於測試檔，不會被路由掛載。它們不是「可接受的寫法範本」——
// production 仍應顯式處理業務碼，carrier 是安全網不是正常路徑（每次攔截都會記
// ERROR log ＋ gateway_bizcode_swallowed_total）。

const bypassProbeCode int32 = 9331 // INVOICE_PROVIDER_CREDENTIAL_INVALID（preserveData）

func bypassProbeData() *pb.SellerProviderAccountData {
	return &pb.SellerProviderAccountData{
		Provider:      "AMEGO",
		Status:        "CREDENTIAL_INVALID",
		LastRequestId: "req-bypass-001",
	}
}

// bypassServer 對本檔用到的兩支 RPC 一律回「gRPC OK ＋ envelope 9331 ＋ data 仍在」，
// 鏡射 invoice-service 的 provider_account.go。
type bypassServer struct {
	pb.UnimplementedInvoiceServiceServer
}

func (s *bypassServer) HealthCheckProviderAccountForAdmin(_ context.Context, _ *pb.HealthCheckProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	return &pb.GetProviderAccountForAdminResponse{
		Code: bypassProbeCode, Message: "憑證驗證失敗", Data: bypassProbeData(),
	}, nil
}

func (s *bypassServer) ListProviderAccountsForAdmin(_ context.Context, _ *pb.ListProviderAccountsForAdminRequest) (*pb.ListProviderAccountsForAdminResponse, error) {
	return &pb.ListProviderAccountsForAdminResponse{
		Code: bypassProbeCode, Message: "憑證驗證失敗",
		Data: &pb.ListProviderAccountsForAdminData{Total: "0", Page: "1", PageSize: "20"},
	}, nil
}

// newBypassRouter 起 bufconn fake server ＋ production 的 EnvelopeGuardInterceptor
// ＋ production 的 bizcarrier.Middleware，把 probe handler 掛在 /probe。
//
// 三者都必須是 production 的真貨：preserveData 這個概念只存在於 interceptor 的
// `return nil` 分支，特權的發放與否只存在於 carrier 在不在場。注入 mock proto client
// 會讓兩者的差別整個消失，那種測試無論 handler 怎麼寫都會綠。
func newBypassRouter(t *testing.T, probe func(h *Holder) gin.HandlerFunc) http.Handler {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := gogrpc.NewServer()
	pb.RegisterInvoiceServiceServer(srv, &bypassServer{})
	go srv.Serve(lis) //nolint:errcheck

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithChainUnaryInterceptor(grpcclient.EnvelopeGuardInterceptor()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(); srv.Stop() })

	h := &Holder{Invoice: grpcclient.NewInvoiceServiceClientForTest(pb.NewInvoiceServiceClient(conn))}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	r.GET("/probe", probe(h))
	return r
}

// assertNotSwallowed 斷言回應是「HTTP 200 ＋ code 9331 ＋ data 原樣」。
//
// 用 json.Decoder 讀**第一個** JSON 物件而不是整個 body：繞道 ⑥（忘了 return）與
// ⑧（順序顛倒）會寫兩次，而客戶端解析到的就是第一個 —— 那才是使用者實際看到的東西。
func assertNotSwallowed(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	// 從字串副本讀，不從 w.Body 直接讀：json.Decoder 會消耗掉 buffer，
	// 後續還要看 raw body 的斷言會拿到空字串。
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&body); err != nil {
		t.Fatalf("decode body: %v；raw=%s", err, w.Body.String())
	}
	code, _ := body["code"].(float64)
	if int32(code) != bypassProbeCode {
		t.Fatalf("envelope code = %v, want %d —— 業務失敗被寫成了成功回應；body=%v",
			body["code"], bypassProbeCode, body)
	}
	if body["data"] == nil {
		t.Errorf("data 必須原樣透傳（前端靠 providerRequestId 對帳），得到 nil；body=%v", body)
	}
}

func runProbe(t *testing.T, probe func(h *Holder) gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	r := newBypassRouter(t, probe)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe?flag=", nil))
	return w
}

func probeCall(h *Holder, c *gin.Context) (*pb.GetProviderAccountForAdminResponse, bool) {
	resp, err := h.Invoice.HealthCheckProviderAccountForAdmin(c.Request.Context(),
		&pb.HealthCheckProviderAccountForAdminRequest{ActorAccountId: 1, StoreId: "2685914682"})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return nil, false
	}
	return resp, true
}

// ⚠️ 繞道 ① （import 取別名 `rsp "…/response"`）在同目錄的
// invoice_bizcarrier_alias_test.go —— 同一個檔案裡把同一個 package import 兩次會被
// staticcheck ST1019 擋下，而那個 lint 剛好不是本次要驗的東西。

// ---------------------------------------------------------------------------
// ② RPC 呼叫抽成 helper：call 與 emit 分居兩個 function body
//
// 舊 gate 在 emit 所在的 body 內找不到呼叫點、在 call 所在的 body 內找不到 emit ——
// 兩半各自「乾淨」，全部跳過。carrier 掛在 request context 上，跨任意函式邊界存活。
// ---------------------------------------------------------------------------
func fetchProviderHealth(h *Holder, ctx context.Context) (*pb.GetProviderAccountForAdminResponse, error) {
	return h.Invoice.HealthCheckProviderAccountForAdmin(ctx,
		&pb.HealthCheckProviderAccountForAdminRequest{ActorAccountId: 1, StoreId: "2685914682"})
}

func TestBypass02_RPCExtractedToHelper(t *testing.T) {
	assertNotSwallowed(t, runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, err := fetchProviderHealth(h, c.Request.Context())
			if err != nil {
				response.GrpcErr(c, "invoice", err)
				return
			}
			response.OK(c, convInvSellerProviderAccountData(resp.GetData()))
		}
	}))
}

// ---------------------------------------------------------------------------
// ③ 守門 body 內再包一層條件：只處理 9332，9331 直接落到成功出口
//
// 舊 gate 的 bodyRejects 用 ast.Inspect 遞迴問「子樹裡存不存在 response.Biz 與
// return」——存在即算數，不問「9331 走不走得到」。
// ---------------------------------------------------------------------------
func TestBypass03_GuardBodyNarrowedByInnerCondition(t *testing.T) {
	assertNotSwallowed(t, runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			data := convInvSellerProviderAccountData(resp.GetData())
			if resp.GetCode() != 0 {
				if resp.GetCode() == 9332 { // 9331 不在此列 ⇒ 守門形同虛設
					response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
					return
				}
			}
			response.OK(c, data)
		}
	}))
}

// ---------------------------------------------------------------------------
// ④ 守門塞進一個實際不成立的分支（`if c.Query("flag") != ""`）
//
// 舊 gate 只看 if 的**條件式裡有沒有提到 GetCode**（qualifyingGuards），不看那個
// 分支跑不跑得到。請求帶的是 `?flag=`（空值）⇒ 守門整段不執行。
// ---------------------------------------------------------------------------
func TestBypass04_GuardBehindUnrelatedCondition(t *testing.T) {
	assertNotSwallowed(t, runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			data := convInvSellerProviderAccountData(resp.GetData())
			if c.Query("flag") != "" {
				if resp.GetCode() != 0 {
					response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
					return
				}
			}
			response.OK(c, data)
		}
	}))
}

// ---------------------------------------------------------------------------
// ⑤ 讀了 code 但沒有 response.Biz（只記個 log 就走成功出口）
// ---------------------------------------------------------------------------
func TestBypass05_ReadsCodeButNeverRejects(t *testing.T) {
	assertNotSwallowed(t, runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			if resp.GetCode() != 0 {
				c.Set("observed_biz_code", resp.GetCode()) // 讀了，然後什麼也沒做
			}
			response.OK(c, convInvSellerProviderAccountData(resp.GetData()))
		}
	}))
}

// ---------------------------------------------------------------------------
// ⑥ 有 response.Biz 但忘了 return
//
// 舊 gate 的 bodyRejects 只問「子樹裡存不存在一個 return 節點」，不問它守不守得住這
// 條路徑。這裡 Biz 先寫一份、接著 OK 又寫一份 ——「客戶端讀到的是第一個 JSON 物件」，
// 所以使用者仍然看得到 9331（碼與 data 都對）。
// ---------------------------------------------------------------------------
func TestBypass06_RejectWithoutReturn(t *testing.T) {
	assertNotSwallowed(t, runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			data := convInvSellerProviderAccountData(resp.GetData())
			if resp.GetCode() != 0 {
				response.Biz(c, resp.GetCode(), resp.GetMessage(), data) // 缺 return
			}
			response.OK(c, data)
		}
	}))
}

// ---------------------------------------------------------------------------
// ⑦ 多重回傳點：守門只蓋住其中一條，另一條直達成功出口
// ---------------------------------------------------------------------------
func TestBypass07_SecondUnguardedReturnPath(t *testing.T) {
	assertNotSwallowed(t, runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			data := convInvSellerProviderAccountData(resp.GetData())
			if c.Query("mode") == "strict" {
				if resp.GetCode() != 0 {
					response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
					return
				}
				response.OK(c, data)
				return
			}
			response.OK(c, data) // 這一條沒有任何守門
		}
	}))
}

// ---------------------------------------------------------------------------
// ⑧ 順序顛倒：先 response.OK 再寫守門
// ---------------------------------------------------------------------------
func TestBypass08_EmitBeforeGuard(t *testing.T) {
	assertNotSwallowed(t, runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			data := convInvSellerProviderAccountData(resp.GetData())
			response.OK(c, data) // 先送
			if resp.GetCode() != 0 {
				response.Biz(c, resp.GetCode(), resp.GetMessage(), data) // 才守
			}
		}
	}))
}

// ---------------------------------------------------------------------------
// ⑨ 換一個成功出口寫法：response.Page（分頁端點）
//
// 舊 gate 的 successEmitters 是三個寫死的名字，任何一個沒被列進去的出口都等於失明。
// 這裡順帶驗證改正後的分頁 data 逐欄不變（records/total/page/pageSize）。
//
// ⚠️ 「handler 搬到別的 package」這個形狀由同目錄的
// invoice_bizcarrier_external_test.go（package handler_test）覆蓋。
// ---------------------------------------------------------------------------
func TestBypass09_PageEmitter(t *testing.T) {
	w := runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, err := h.Invoice.ListProviderAccountsForAdmin(c.Request.Context(),
				&pb.ListProviderAccountsForAdminRequest{ActorAccountId: 1})
			if err != nil {
				response.GrpcErr(c, "invoice", err)
				return
			}
			d := resp.GetData()
			response.Page(c, []string{}, d.GetTotal(), d.GetPage(), d.GetPageSize())
		}
	})
	assertNotSwallowed(t, w)

	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 應為分頁物件，得到 %#v", body["data"])
	}
	for _, k := range []string{"records", "total", "page", "pageSize"} {
		if _, present := data[k]; !present {
			t.Errorf("改正後的分頁 data 缺欄位 %q（前端讀 data.pageSize，缺了就是 undefined）", k)
		}
	}
}

// ---------------------------------------------------------------------------
// 反向鑑別力：非 preserveData 碼**不得**被 carrier 吞成 200
//
// 只驗「9331 會被抓到」是陽性驗證；這一支是陰性驗證 —— 證明守門沒有順手把錯誤
// 也一起吞掉。9332 兩張表都沒登記 ⇒ fallback FailedPrecondition ⇒ HTTP 409 ＋ [9332]。
// ---------------------------------------------------------------------------
type nonPreserveServer struct {
	pb.UnimplementedInvoiceServiceServer
}

func (s *nonPreserveServer) HealthCheckProviderAccountForAdmin(_ context.Context, _ *pb.HealthCheckProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	return &pb.GetProviderAccountForAdminResponse{Code: 9332, Message: "provider 暫時不可用"}, nil
}

func TestBypass_NonPreserveDataCodeStillErrors(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := gogrpc.NewServer()
	pb.RegisterInvoiceServiceServer(srv, &nonPreserveServer{})
	go srv.Serve(lis) //nolint:errcheck
	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithChainUnaryInterceptor(grpcclient.EnvelopeGuardInterceptor()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(); srv.Stop() })

	h := &Holder{Invoice: grpcclient.NewInvoiceServiceClientForTest(pb.NewInvoiceServiceClient(conn))}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	r.GET("/probe", func(c *gin.Context) {
		resp, ok := probeCall(h, c)
		if !ok {
			return
		}
		response.OK(c, convInvSellerProviderAccountData(resp.GetData()))
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))

	if w.Code != http.StatusConflict {
		t.Fatalf("HTTP status = %d, want 409（9332 走 fallback FailedPrecondition）；body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "9332") {
		t.Errorf("業務碼必須活著抵達前端，body=%s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ⑪ response.Biz(c, 0, "success", data) —— 零新程式碼的繞道
//
// 前十種都需要 handler 換一種寫法。這一種不需要：`Biz` 是**已經存在且正在被使用**的
// exported helper（handler/checkout.go:553/643/651 用的是同一支），它的 code 是引數，
// 傳 0 就吐出 {"code":0,"message":"success","data":…} HTTP 200 —— 與 response.OK
// 逐位元同構的假成功。
//
// 🔴 為什麼它比前十種更嚴重：bizcarrier 的第一版把「要不要查 carrier」綁在**函式身分**
// 上（四支成功出口查、其餘 18 支呼叫 MarkEmitted）。而 MarkEmitted 會把 emitted 設成
// true，正是 Middleware 收尾偵測讀的那個旗標 ⇒ 走這條路的假成功不只沒被改寫，還把
// 逃逸偵測一起關掉：carrier 原封不動 armed 著沒人看、swallowed／escaped 兩個計數器都
// 不動、ERROR log 一行都不出。**舊 AST gate 至少只是沉默，那一版是沉默加上蓋了一個
// 「已檢查」的章。**
//
// 修法是把述詞從「呼叫了哪個函式」換成「實際寫出去的 envelope code 是不是 0」
// （internal/response 的 guard／bizcarrier.Emit）。
// ---------------------------------------------------------------------------
func TestBypass11_BizWithZeroCode(t *testing.T) {
	w := runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			// 沒有守門，且刻意用「非成功出口」把成功形狀寫出去。
			response.Biz(c, 0, "success", convProbeData(resp))
		}
	})
	assertNotSwallowed(t, w)
}

// ⑪b 同形變體：BizStatus(c, 0, …)。它連 HTTP status 都不是 200（塌縮成 409），
// 但 envelope 的 code 仍是 0 —— 前端讀 code 判成功，一樣是假成功。
func TestBypass11b_BizStatusWithZeroCode(t *testing.T) {
	w := runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			_ = resp
			response.BizStatus(c, 0, "success")
		}
	})
	// 改寫之後形狀回到 preserveData 契約：HTTP 200 ＋ 9331。
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&body); err != nil {
		t.Fatalf("decode body: %v；raw=%s", err, w.Body.String())
	}
	if code, _ := body["code"].(float64); int32(code) != bypassProbeCode {
		t.Fatalf("envelope code = %v, want %d；body=%v", body["code"], bypassProbeCode, body)
	}
}

func convProbeData(resp *pb.GetProviderAccountForAdminResponse) gin.H {
	return gin.H{
		"provider":          resp.GetData().GetProvider(),
		"status":            resp.GetData().GetStatus(),
		"providerRequestId": resp.GetData().GetLastRequestId(),
	}
}

// ⑫ 自己組 Envelope ＋ 用非 JSON writer 寫出（真實形態，非合成）。
//
// 🔴 這一支對應的是 repo 裡**今天就在 prod 路徑上**的一條路：
// pricebook.go 的 writeGzippedIfAccepted 自己 json.Marshal(response.Envelope{Code:0,…})
// 再用 c.Data 寫出。兩條 ruleguard lint 都不命中（banDirectJSON 認 c.JSON、
// banEnvelopeSuccessEmitters 認 envelope.OK/Page/…），實跑 0 issues。
//
// carrier 對這條路是**偵測不是阻擋**（carrier.go 檔頭失效模式 4）——除非那條路徑
// 自己問一次 Emit。本測試守的就是「它有問」這件事。
//
// ⚠️ 本支測的是**形狀**（合成 handler），不是 pricebook 那支函式本身 ——
// 動 pricebook.go 不會讓它轉紅。守住真實呼叫點的是下面的
// TestWriteGzippedIfAcceptedAsksCarrier，兩支分工不同、缺一不可。
func TestBypass12_HandRolledEnvelopeWithDataWriter(t *testing.T) {
	w := runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, ok := probeCall(h, c)
			if !ok {
				return
			}
			// 照 pricebook.go 的形狀：問過 carrier 才組 Envelope。
			code, msg, rewrite := bizcarrier.Emit(c, 0)
			if !rewrite {
				code, msg = 0, "success"
			}
			body, _ := json.Marshal(response.Envelope{Code: code, Message: msg, Data: convProbeData(resp)})
			c.Data(http.StatusOK, "application/json; charset=utf-8", body)
		}
	})
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&body); err != nil {
		t.Fatalf("decode body: %v；raw=%s", err, w.Body.String())
	}
	if code, _ := body["code"].(float64); int32(code) != bypassProbeCode {
		t.Fatalf("envelope code = %v, want %d —— 自己組 Envelope 的路徑沒有問 carrier，"+
			"非零業務碼被寫成假成功；body=%s", body["code"], bypassProbeCode, w.Body.String())
	}
}

// TestWriteGzippedIfAcceptedAsksCarrier 直接打 pricebook.go 的那支函式本身。
//
// 🔴 與上面的 ⑫ 分工：⑫ 守「這個形狀會被 carrier 接住」，本支守「repo 裡真的
// 那一條路有問 carrier」。只有 ⑫ 的話，pricebook.go 改回不問也不會轉紅 —— 我
// 第一版就是這樣寫的，變異當場證明它零鑑別力（06 §3.10：機制的觀測窗要涵蓋
// 它宣稱守住的東西）。
func TestWriteGzippedIfAcceptedAsksCarrier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	r.GET("/probe", func(c *gin.Context) {
		if !bizcarrier.Arm(c.Request.Context(), "/probe.Svc/M", bypassProbeCode, "credential invalid") {
			t.Fatal("Arm 失敗——carrier 沒掛上")
		}
		writeGzippedIfAccepted(c, json.RawMessage(`{"k":"v"}`))
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	r.ServeHTTP(w, req)

	gz, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("回應不是 gzip：%v", err)
	}
	raw, _ := io.ReadAll(gz)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode: %v；raw=%s", err, raw)
	}
	if code, _ := body["code"].(float64); int32(code) != bypassProbeCode {
		t.Fatalf("envelope code = %v, want %d —— writeGzippedIfAccepted 沒有問 carrier，"+
			"非零業務碼被寫成假成功；raw=%s", body["code"], bypassProbeCode, raw)
	}
}
