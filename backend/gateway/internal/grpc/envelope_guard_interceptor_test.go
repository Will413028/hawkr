package grpc

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeInvoker returns a grpc.UnaryInvoker that fills `reply` with the canned values.
// `replyCode` / `replyMsg` simulate what the delegation module returned in the response envelope.
func fakeInvoker(replyCode int32, replyMsg string, retErr error) grpc.UnaryInvoker {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		if retErr != nil {
			return retErr
		}
		if r, ok := reply.(*accountpb.AuthResponse); ok {
			r.Code = replyCode
			r.Message = replyMsg
		}
		return nil
	}
}

func TestEnvelopeGuard_Success_NoWrap(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	reply := &accountpb.AuthResponse{}
	err := intercept(context.Background(), "/m", &accountpb.GoogleOAuthRequest{}, reply, nil,
		fakeInvoker(0, "OK", nil))
	if err != nil {
		t.Fatalf("expected nil error for code=0, got %v", err)
	}
}

func TestEnvelopeGuard_InvokerError_Passthrough(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	want := errors.New("connection refused")
	got := intercept(context.Background(), "/m", &accountpb.GoogleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
		fakeInvoker(0, "", want))
	if got == nil || got.Error() != want.Error() {
		t.Fatalf("expected invoker error to passthrough, got %v", got)
	}
}

// 4291 (AI 修圖本週額度已用完) needs both halves of the mapping: ResourceExhausted
// (so it becomes HTTP 429, not 409) *and* the "[4291] " prefix (so response.GrpcErr
// can restore the number into the envelope). Without the prefix the numeric code
// dies at the gRPC boundary and both frontends can only see HTTP 429 — which the
// rate limiter also emits — so a throttled request gets misreported as "weekly
// quota used up" (#2272).
func TestEnvelopeGuard_QuotaExceeded_MapsToResourceExhaustedWithBizCode(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	got := intercept(context.Background(), "/m", &accountpb.AppleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
		fakeInvoker(4291, "本週 AI 修圖額度已用完（10/10），下週一重置", nil))

	st, ok := status.FromError(got)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", got)
	}
	if st.Code() != codes.ResourceExhausted {
		t.Errorf("code = %v, want ResourceExhausted", st.Code())
	}
	if st.Message() != "[4291] 本週 AI 修圖額度已用完（10/10），下週一重置" {
		t.Errorf("message = %q, want [4291]-prefixed quota message", st.Message())
	}
}

func TestEnvelopeGuard_ProductVisibilityCodes_MapToNotFoundWithBizCode(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	for _, code := range []int32{3021, 3022, 3023} {
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			got := intercept(context.Background(), "/m", &accountpb.AppleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
				fakeInvoker(code, "商品不可公開", nil))
			st, ok := status.FromError(got)
			if !ok {
				t.Fatalf("expected gRPC status error, got %v", got)
			}
			if st.Code() != codes.NotFound {
				t.Fatalf("code = %v, want NotFound", st.Code())
			}
			want := fmt.Sprintf("[%d] 商品不可公開", code)
			if st.Message() != want {
				t.Fatalf("message = %q, want %q", st.Message(), want)
			}
		})
	}
}

func TestEnvelopeGuard_UnknownBusinessCode_AddsPrefix(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	got := intercept(context.Background(), "/m", &accountpb.AppleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
		fakeInvoker(9999, "something broke", nil))

	st, ok := status.FromError(got)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", got)
	}
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("code = %v, want FailedPrecondition (default for unknown)", st.Code())
	}
	if st.Message() != "[9999] something broke" {
		t.Errorf("message = %q, want %q", st.Message(), "[9999] something broke")
	}
}

// HTTP 鏡射碼沒有可保留的業務碼（它本身就是 HTTP status），訊息必須逐字不變。
func TestEnvelopeGuard_KnownCodeMapping_Unprefixed(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	cases := []struct {
		code int32
		want codes.Code
	}{
		{400, codes.InvalidArgument},
		{401, codes.Unauthenticated},
		{403, codes.PermissionDenied},
		{404, codes.NotFound},
		{409, codes.AlreadyExists},
		{412, codes.FailedPrecondition},
		{500, codes.Internal},
		{503, codes.Unavailable},
	}
	for _, tc := range cases {
		err := intercept(context.Background(), "/m", &accountpb.AppleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
			fakeInvoker(tc.code, "msg", nil))
		st, _ := status.FromError(err)
		if st.Code() != tc.want {
			t.Errorf("code=%d: got %v, want %v", tc.code, st.Code(), tc.want)
		}
		if st.Message() != "msg" {
			t.Errorf("code=%d: message=%q, want unprefixed %q", tc.code, st.Message(), "msg")
		}
	}
}

// TestEnvelopeGuard_RefreshTokenUnavailable_MapsToGRPCUnavailable pins the
// actual production guarantee behind issue #1544 L2's "DB hiccup must not
// look like an invalid refresh token" contract.
//
// gateway/internal/handler/auth.go's RefreshToken has an `if resp.Code ==
// int32(errors.CodeUnavailable)` branch, but that branch is DEAD on the wire:
// client.go's DefaultDialOpts chains EnvelopeGuardInterceptor onto every
// Gateway gRPC client (including account), and this interceptor turns any
// non-zero resp.Code into a gRPC error before invoker returns — so by the
// time the handler runs, `err != nil` is already true and `resp` is nil.
// `if resp.Code != 0` can never be reached in production.
//
// The handler-level test (auth_refresh_test.go's
// TestRefreshToken_StoreUnavailableIsHTTP503NotUnauthorized) only stays
// meaningful because grpc/testing.go's NewAccountClientForTest bypasses this
// interceptor chain entirely and hands the handler a raw mock response —
// that's a valid test of the handler's branch, but it is NOT the thing that
// keeps production honest. The real guarantee is this mapping: 503 →
// codes.Unavailable in envelopeCodeToGRPC (which response.GrpcErr then turns
// into HTTP 503). If someone removes the `503: {grpcCode: codes.Unavailable}`
// line from that table, auth_refresh_test.go stays green while every
// production DB hiccup on refresh silently collapses to HTTP 401 and
// force-logs-out both Flutter apps. This test is the one that actually
// catches that regression.
func TestEnvelopeGuard_RefreshTokenUnavailable_MapsToGRPCUnavailable(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		r, ok := reply.(*accountpb.RefreshTokenResponse)
		if !ok {
			t.Fatalf("reply type = %T, want *accountpb.RefreshTokenResponse", reply)
		}
		r.Code = 503
		r.Message = "credential store unavailable"
		return nil
	}

	err := intercept(context.Background(), accountpb.AccountService_RefreshToken_FullMethodName,
		&accountpb.RefreshTokenRequest{}, &accountpb.RefreshTokenResponse{}, nil, invoker)

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Errorf("code = %v, want codes.Unavailable (this is what response.GrpcErr maps to HTTP 503; "+
			"gateway/internal/handler/auth.go's resp.Code == errors.CodeUnavailable check is unreachable "+
			"in production because err != nil short-circuits before that line)", st.Code())
	}
}

// 保留業務碼的已知碼：status 走表定值，數字碼靠前綴穿過 gRPC 邊界。
func TestEnvelopeGuard_KnownCodeMapping_PrefixedBizCode(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	cases := []struct {
		code int32
		want codes.Code
	}{
		{4291, codes.ResourceExhausted},
		{9400, codes.InvalidArgument},
	}
	for _, tc := range cases {
		err := intercept(context.Background(), "/m", &accountpb.AppleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
			fakeInvoker(tc.code, "msg", nil))
		st, _ := status.FromError(err)
		if st.Code() != tc.want {
			t.Errorf("code=%d: got %v, want %v", tc.code, st.Code(), tc.want)
		}
		want := fmt.Sprintf("[%d] msg", tc.code)
		if st.Message() != want {
			t.Errorf("code=%d: message=%q, want %q", tc.code, st.Message(), want)
		}
	}
}

// ---------------------------------------------------------------------------
// preserveData × bizcarrier：特權只在「有人接得住」時才發
// ---------------------------------------------------------------------------

// carrierCtx 用 production 的 bizcarrier.Middleware 造一個帶 carrier 的 context，
// 而不是自己組一個測試用的 —— 自己組的話測到的就不是實際掛在 engine 上的那條路徑。
func carrierCtx(t *testing.T) (context.Context, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	var ctx context.Context
	var gc *gin.Context
	r.GET("/probe", func(c *gin.Context) {
		ctx, gc = c.Request.Context(), c
		c.Status(200)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/probe", nil))
	if ctx == nil {
		t.Fatal("probe handler 沒被呼叫")
	}
	return ctx, gc
}

// preserveData 碼 ＋ carrier 在場：interceptor 不轉 error（data 才過得去），
// 業務碼掛在 carrier 上等成功出口來取。
func TestEnvelopeGuard_PreserveData_ArmsCarrier(t *testing.T) {
	ctx, gc := carrierCtx(t)
	intercept := EnvelopeGuardInterceptor()
	reply := &accountpb.AuthResponse{}
	err := intercept(ctx, "/m", &accountpb.GoogleOAuthRequest{}, reply, nil,
		fakeInvoker(4001, "商品不可購買", nil))
	if err != nil {
		t.Fatalf("preserveData 碼不該轉成 gRPC error，得到 %v", err)
	}
	code, armed := bizcarrier.Armed(gc)
	if !armed || code != 4001 {
		t.Fatalf("carrier = (%d, %v)，want (4001, true)", code, armed)
	}
}

// 🔴 fail-closed：carrier 不在場時**不得**發 preserveData 特權。
//
// 舊 AST gate 的第一個根因就是相反的預設 —— 「這個呼叫點之後找不到成功出口」被當成
// 安全的證明並靜默 continue。這裡把「我保護不到」當成撤銷特權的理由：落回一般錯誤
// 映射，data 掉了（降級），但 handler 的 `if err != nil` 一定接得到，
// **絕不會產生 code=0 的假成功**。
func TestEnvelopeGuard_PreserveData_NoCarrier_FailsClosed(t *testing.T) {
	intercept := EnvelopeGuardInterceptor()
	reply := &accountpb.AuthResponse{}
	err := intercept(context.Background(), "/m", &accountpb.GoogleOAuthRequest{}, reply, nil,
		fakeInvoker(9331, "憑證驗證失敗", nil))
	if err == nil {
		t.Fatal("沒有 carrier 時 preserveData 碼必須降級成 gRPC error，不得 return nil")
	}
	st, _ := status.FromError(err)
	// 9331 兩張表都沒登記 ⇒ fallback = FailedPrecondition ＋ 保留數字碼前綴。
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("gRPC code = %v, want FailedPrecondition", st.Code())
	}
	if !strings.Contains(st.Message(), "[9331]") {
		t.Errorf("降級後業務碼必須仍在 message 裡，得到 %q", st.Message())
	}
}

// 🔴 解除必須逐 method：**別支** RPC 成功不得抹掉先前那支掛上的業務碼。
//
// 這支測試在 2026-08-31 之前斷言的是相反的事（`/m1` 掛 4091、`/m2` 成功就必須解除），
// 也就是把根因 A（拿「沒有觀察到危險」推論「沒有危險」）寫成了驗收條件。真實後果：
// 一個請求裡先失敗的 RPC 被後面任何一支成功的 RPC 靜默抹掉，handler 接著走
// response.OK ⇒ 假成功照樣送出去，而 carrier 是「已解除」所以連 ERROR log 都沒有。
func TestEnvelopeGuard_OtherMethodSuccessDoesNotDisarm(t *testing.T) {
	ctx, gc := carrierCtx(t)
	intercept := EnvelopeGuardInterceptor()
	if err := intercept(ctx, "/m1", &accountpb.GoogleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
		fakeInvoker(4091, "價格已變動", nil)); err != nil {
		t.Fatalf("arm 階段不該有 error: %v", err)
	}
	if _, armed := bizcarrier.Armed(gc); !armed {
		t.Fatal("前置條件不成立：第一支 RPC 應該已經 arm")
	}
	if err := intercept(ctx, "/m2", &accountpb.GoogleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
		fakeInvoker(0, "success", nil)); err != nil {
		t.Fatalf("code=0 不該有 error: %v", err)
	}
	if code, armed := bizcarrier.Armed(gc); !armed || code != 4091 {
		t.Fatalf("別支 RPC 成功之後 carrier 必須仍掛著 4091，得到 (%d, %v)", code, armed)
	}
}

// 同一支 method 重試成功 ⇒ 它自己掛上的碼解除。這是「後來的成功真的取代了先前的
// 失敗」唯一成立的情形，也是 Disarm 存在的全部理由。
func TestEnvelopeGuard_SameMethodRetrySuccessDisarms(t *testing.T) {
	ctx, gc := carrierCtx(t)
	intercept := EnvelopeGuardInterceptor()
	if err := intercept(ctx, "/m1", &accountpb.GoogleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
		fakeInvoker(4091, "價格已變動", nil)); err != nil {
		t.Fatalf("arm 階段不該有 error: %v", err)
	}
	if err := intercept(ctx, "/m1", &accountpb.GoogleOAuthRequest{}, &accountpb.AuthResponse{}, nil,
		fakeInvoker(0, "success", nil)); err != nil {
		t.Fatalf("code=0 不該有 error: %v", err)
	}
	if _, armed := bizcarrier.Armed(gc); armed {
		t.Error("同一支 method 重試成功之後 carrier 必須已解除")
	}
}
