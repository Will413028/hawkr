package envelope

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// parseBizMessage 要求 "[NNNN]" 在訊息開頭。SelfDeleteSeller 曾因 gateway client 用
// fmt.Errorf("...gRPC: %w") 包裝 status，使 grpc-go 把 message 換成整串 wrapped 字串，
// 把 "[9204]" 擠到中間 → 解析失敗、業務碼漏掉。此測試鎖住該契約。
func TestParseBizMessage(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantCode int32
		wantMsg  string
	}{
		{"prefix at start", "[9204] 帳號已在刪除流程中", 9204, "帳號已在刪除流程中"},
		{"active orders", "[9205] 您有 2 筆未完成訂單", 9205, "您有 2 筆未完成訂單"},
		{
			"prefix mid-string (wrapped status) must NOT parse",
			"SelfDeleteSeller gRPC: rpc error: code = FailedPrecondition desc = [9204] 帳號已在刪除流程中",
			0, "",
		},
		{"no prefix", "plain error", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, msg := parseBizMessage(tc.in)
			if code != tc.wantCode || msg != tc.wantMsg {
				t.Fatalf("parseBizMessage(%q) = (%d, %q), want (%d, %q)",
					tc.in, code, msg, tc.wantCode, tc.wantMsg)
			}
		})
	}
}

// GrpcErr 對 FailedPrecondition 的業務碼統一透傳（issue #1479）：訊息帶
// "[NNNN] " 前綴一律還原為 envelope Code、message 去前綴；不再侷限舊
// bizCodePassthrough 白名單（9240/9241 停權家族，該白名單已隨本次改動移除）。
func TestGrpcErr_FailedPrecondition_BizCodePassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/app/checkout/validate", nil)

	// 一般業務碼（非停權家族）：4015 SELF_PURCHASE_NOT_ALLOWED，過去會被舊白名單
	// 排除、塌縮成 {code:409, message:"[4015] ..."}；統一後應還原業務碼。
	err := grpcstatus.Error(codes.FailedPrecondition, "[4015] 不可購買自己上架的商品")
	GrpcErr(c, "order", err)

	if w.Code != http.StatusConflict {
		t.Fatalf("HTTP status = %d, want 409", w.Code)
	}
	var env Envelope
	if e := json.Unmarshal(w.Body.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if env.Code != 4015 {
		t.Errorf("code = %d, want 4015", env.Code)
	}
	if env.Message != "不可購買自己上架的商品" {
		t.Errorf("message = %q, want prefix stripped", env.Message)
	}
	if env.MessageZh != "不可購買自己上架的商品" {
		t.Errorf("messageZh = %q, want clean text", env.MessageZh)
	}
}

// Removing the NotFound business-code restoration would collapse distinct
// PRODUCT_NOT_FOUND / PRODUCT_NOT_PUBLISHED / PRODUCT_REMOVED responses back
// to the generic code 404 even though the gRPC description carries CodeMeta.
func TestGrpcErr_NotFound_BizCodePassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/products/1", nil)

	err := grpcstatus.Error(codes.NotFound, "[3022] 商品從未公開")
	GrpcErr(c, "product", err)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var got Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != 3022 || got.Message != "商品從未公開" || got.MessageZh != "商品從未公開" {
		t.Fatalf("envelope = %+v, want code=3022 with clean localized message", got)
	}
}

// 無 "[NNNN] " 前綴的 FailedPrecondition message 維持塌縮 {code:409}——沒有業務碼
// 可還原時的既有行為不變。
func TestGrpcErr_FailedPrecondition_NoPrefixCollapsesTo409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/app/checkout/validate", nil)

	err := grpcstatus.Error(codes.FailedPrecondition, "plain precondition failure")
	GrpcErr(c, "order", err)

	if w.Code != http.StatusConflict {
		t.Fatalf("HTTP status = %d, want 409", w.Code)
	}
	var env Envelope
	if e := json.Unmarshal(w.Body.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if env.Code != 409 {
		t.Errorf("code = %d, want 409 (no bizcode prefix to recover)", env.Code)
	}
	if env.Message != "plain precondition failure" {
		t.Errorf("message = %q, want passthrough unchanged", env.Message)
	}
}

// issue #1624：呼叫方主動斷線（分頁關閉/App 背景化）造成 c.Request.Context() 取消，
// 沿此 context 的 gRPC 呼叫收到 codes.Canceled——不是下游服務異常，不該計入 503。
func TestGrpcErr_Canceled_ClientDisconnected_Returns499NotServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/public/categories/tree", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	c.Request = req.WithContext(ctx)

	err := grpcstatus.Error(codes.Canceled, "context canceled")
	GrpcErr(c, "product", err)

	if w.Code != 499 {
		t.Fatalf("HTTP status = %d, want 499 (client-canceled, must not pollute 5xx rate)", w.Code)
	}
}

// codes.Canceled 理论上也可能不是由 c.Request.Context() 取消引起（例如呼叫端自己組的
// sub-context）——這種情況 ctx.Err() 為 nil，維持既有 503 行為，不誤判成 client 噪音。
func TestGrpcErr_Canceled_NotFromClientContext_StillServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/public/categories/tree", nil)

	err := grpcstatus.Error(codes.Canceled, "context canceled")
	GrpcErr(c, "product", err)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("HTTP status = %d, want 503 (ctx not client-canceled)", w.Code)
	}
}

// 46 §7.2 停權家族（9240）：改前既有白名單案例，改後（統一透傳）行為必須不回歸——
// 仍是 {code:9240, message 去前綴}。用 GrpcErr 直測（GrpcDomainErr 現為其別名）。
func TestGrpcErr_FailedPrecondition_SuspensionFamily9240NoRegression(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/app/sellers/me/products", nil)

	err := grpcstatus.Error(codes.FailedPrecondition, "[9240] 賣家帳號停權中，無法執行此操作")
	GrpcErr(c, "seller", err)

	if w.Code != http.StatusConflict {
		t.Fatalf("HTTP status = %d, want 409", w.Code)
	}
	var env Envelope
	if e := json.Unmarshal(w.Body.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if env.Code != 9240 {
		t.Errorf("code = %d, want 9240 (no regression from pre-#1479 whitelist behavior)", env.Code)
	}
	if env.Message != "賣家帳號停權中，無法執行此操作" {
		t.Errorf("message = %q, want clean text", env.Message)
	}
}

// GrpcDomainErr 對「未包裝、前綴在開頭」的 FailedPrecondition status，
// 應回 {code:NNNN, message, messageZh}（HTTP 409）。
func TestGrpcDomainErr_BusinessCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/app/sellers/me", nil)

	err := grpcstatus.Error(codes.FailedPrecondition, "[9204] 帳號已在刪除流程中")
	GrpcDomainErr(c, "seller", err)

	if w.Code != http.StatusConflict {
		t.Fatalf("HTTP status = %d, want 409", w.Code)
	}
	var env Envelope
	if e := json.Unmarshal(w.Body.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if env.Code != 9204 || env.MessageZh != "帳號已在刪除流程中" {
		t.Fatalf("envelope = {code:%d, messageZh:%q}, want {9204, 帳號已在刪除流程中}",
			env.Code, env.MessageZh)
	}
}

func TestOKProto_WrapsProtoInEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	OKProto(c, &accountpb.RefreshTokenResponse{ExpiresIn: 123})

	var env map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v; body=%s", err, w.Body.String())
	}
	if string(env["code"]) != "0" {
		t.Errorf("want code 0; got %s", env["code"])
	}
	if string(env["message"]) != `"success"` {
		t.Errorf("want message success; got %s", env["message"])
	}
	if !strings.Contains(string(env["data"]), `"expiresIn":"123"`) {
		t.Errorf("want data carrying camelCase int64-string; got %s", env["data"])
	}
}

// Pins the gateway-wide protojson contract: camelCase field names + int64
// rendered as JSON string + zero values emitted. Flipping UseProtoNames or
// dropping EmitUnpopulated breaks every passthrough endpoint silently.
func TestProtoToRaw_MarshalerContract(t *testing.T) {
	// camelCase (UseProtoNames=false) AND int64-as-string.
	got := string(ProtoToRaw(&accountpb.RefreshTokenResponse{ExpiresIn: 123}))
	if !strings.Contains(got, `"expiresIn":"123"`) {
		t.Errorf("want camelCase int64-as-string `\"expiresIn\":\"123\"`; got %s", got)
	}

	// EmitUnpopulated: a zero-value bool field is still present (camelCase too).
	gotZero := string(ProtoToRaw(&accountpb.AuthResponse{}))
	if !strings.Contains(gotZero, `"needPhoneVerification":false`) {
		t.Errorf("want emit-unpopulated `\"needPhoneVerification\":false`; got %s", gotZero)
	}
}

func TestGrpcResult_SuccessAlwaysUsesCanonicalMessage(t *testing.T) {
	for _, downstreamMessage := range []string{"", "created"} {
		t.Run(downstreamMessage, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			GrpcResult(c, 0, downstreamMessage, map[string]bool{"ok": true})

			var env Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if env.Message != "success" {
				t.Fatalf("message = %q, want canonical success", env.Message)
			}
		})
	}
}

func TestGrpcResult_ErrorPreservesDownstreamMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	GrpcResult(c, 9204, "account deletion pending", nil)

	var env Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Code != 9204 || env.Message != "account deletion pending" {
		t.Fatalf("envelope = {code:%d message:%q}, want downstream business error", env.Code, env.Message)
	}
}
