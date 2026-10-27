package interceptor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	goldenSecret       = "test-shared-secret-please-change"
	goldenServiceName  = "order-service"
	goldenTs           = "1700000000"
	goldenNonce        = "00112233445566778899aabbccddeeff"
	goldenAccountID    = "123456789"
	goldenSigWithAcct  = "TXm3Uefzoq0Z8ru9n++yFm4DjaEs9XZ2GWMpHsi3BOk="
	goldenSvcEmptyAcct = "crm-service"
	goldenNonceEmpty   = "abcdef00112233445566778899aabbcc"
	goldenSigEmptyAcct = "HTg2eOBanXJvJEs67j5dVl+DQXIjU3rvQAmIySTmcZg="
)

func TestInternalAuthSign_GoldenCrossLanguage(t *testing.T) {
	canonical := InternalAuthCanonical(goldenServiceName, goldenTs, goldenNonce, goldenAccountID)
	if got := InternalAuthSign(goldenSecret, canonical); got != goldenSigWithAcct {
		t.Fatalf("with-account sig mismatch:\n got=%s\nwant=%s", got, goldenSigWithAcct)
	}

	canonicalEmpty := InternalAuthCanonical(goldenSvcEmptyAcct, goldenTs, goldenNonceEmpty, "")
	if got := InternalAuthSign(goldenSecret, canonicalEmpty); got != goldenSigEmptyAcct {
		t.Fatalf("empty-account sig mismatch:\n got=%s\nwant=%s", got, goldenSigEmptyAcct)
	}
}

func okHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return "ok", nil
}

func serverInfo() *grpc.UnaryServerInfo {
	return &grpc.UnaryServerInfo{FullMethod: "/hawkr.order.OrderService/GetOrder"}
}

func signedMD(t *testing.T, svc, accountID string) metadata.MD {
	t.Helper()
	cfg := loadInternalAuthConfig()

	nowTs := nowTsStr()
	nonce := newNonce()
	sig := InternalAuthSign(cfg.secret, InternalAuthCanonical(svc, nowTs, nonce, accountID))
	pairs := []string{
		HeaderInternalService, svc,
		HeaderInternalTs, nowTs,
		HeaderInternalNonce, nonce,
		HeaderInternalSig, sig,
	}
	if accountID != "" {
		pairs = append(pairs, headerAccountID, accountID)
	}
	return metadata.Pairs(pairs...)
}

func nowTsStr() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func TestInternalAuthServer_Permissive(t *testing.T) {
	t.Setenv(envInternalAuthMode, InternalAuthModePermissive)
	t.Setenv(envInternalSecret, goldenSecret)
	interceptorFn := InternalAuthServerInterceptor(nil)

	t.Run("non-internal call (no x-internal-service) passes", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(headerAccountID, "555"))
		if _, err := interceptorFn(ctx, nil, serverInfo(), okHandler); err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})

	t.Run("unsigned internal call passes (WARN)", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderInternalService, "ai-service"))
		if _, err := interceptorFn(ctx, nil, serverInfo(), okHandler); err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})

	t.Run("valid signature passes", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), signedMD(t, "order-service", "123456789"))
		if _, err := interceptorFn(ctx, nil, serverInfo(), okHandler); err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})

	t.Run("invalid signature passes in permissive", func(t *testing.T) {
		md := metadata.Pairs(
			HeaderInternalService, "order-service",
			HeaderInternalTs, nowTsStr(),
			HeaderInternalNonce, "deadbeef",
			HeaderInternalSig, "not-a-valid-signature",
		)
		ctx := metadata.NewIncomingContext(context.Background(), md)
		if _, err := interceptorFn(ctx, nil, serverInfo(), okHandler); err != nil {
			t.Fatalf("expected pass in permissive, got %v", err)
		}
	})
}

func TestInternalAuthServer_Enforcing(t *testing.T) {
	t.Setenv(envInternalAuthMode, InternalAuthModeEnforcing)
	t.Setenv(envInternalSecret, goldenSecret)
	interceptorFn := InternalAuthServerInterceptor(nil)

	t.Run("non-internal call passes", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(headerAccountID, "555"))
		if _, err := interceptorFn(ctx, nil, serverInfo(), okHandler); err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})

	t.Run("unsigned internal call rejected", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderInternalService, "ai-service"))
		_, err := interceptorFn(ctx, nil, serverInfo(), okHandler)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected UNAUTHENTICATED, got %v", err)
		}
	})

	t.Run("valid signature passes", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), signedMD(t, "order-service", "123456789"))
		if _, err := interceptorFn(ctx, nil, serverInfo(), okHandler); err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})

	t.Run("invalid signature rejected", func(t *testing.T) {
		md := metadata.Pairs(
			HeaderInternalService, "order-service",
			HeaderInternalTs, nowTsStr(),
			HeaderInternalNonce, "deadbeef",
			HeaderInternalSig, "not-a-valid-signature",
		)
		ctx := metadata.NewIncomingContext(context.Background(), md)
		_, err := interceptorFn(ctx, nil, serverInfo(), okHandler)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected UNAUTHENTICATED, got %v", err)
		}
	})
}

func TestInternalAuthClient_SignsAndBinds(t *testing.T) {
	t.Setenv(envInternalAuthMode, InternalAuthModePermissive)
	t.Setenv(envInternalSecret, goldenSecret)
	ci := InternalAuthClientInterceptor("buyer-service", true)

	var captured metadata.MD
	invoker := func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		captured, _ = metadata.FromOutgoingContext(ctx)
		return nil
	}

	inCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(headerAccountID, "987654321"))
	if err := ci(inCtx, "/m", nil, nil, nil, invoker); err != nil {
		t.Fatalf("client interceptor err: %v", err)
	}
	if got := firstMD(captured, HeaderInternalService); got != "buyer-service" {
		t.Fatalf("x-internal-service=%q", got)
	}
	if got := firstMD(captured, headerAccountID); got != "987654321" {
		t.Fatalf("forwarded x-account-id=%q", got)
	}

	t.Setenv(envInternalSecret, goldenSecret)
	srv := InternalAuthServerInterceptor(nil)
	srvCtx := metadata.NewIncomingContext(context.Background(), captured)
	if _, err := srv(srvCtx, nil, serverInfo(), okHandler); err != nil {
		t.Fatalf("round-trip verify failed: %v", err)
	}
}

func TestResolveInternalAuthMode_NoSilentFallback(t *testing.T) {
	t.Run("三個合法值原樣通過（大小寫與前後空白正規化）", func(t *testing.T) {
		for raw, want := range map[string]string{
			"off":          InternalAuthModeOff,
			"permissive":   InternalAuthModePermissive,
			"enforcing":    InternalAuthModeEnforcing,
			"  ENFORCING ": InternalAuthModeEnforcing,
		} {
			got, err := ResolveInternalAuthMode(raw, true)
			if err != nil {
				t.Fatalf("ResolveInternalAuthMode(%q, true) 非預期 error: %v", raw, err)
			}
			if got != want {
				t.Fatalf("ResolveInternalAuthMode(%q, true) = %q, want %q", raw, got, want)
			}
		}
	})

	t.Run("缺值是 error，不是 permissive", func(t *testing.T) {
		got, err := ResolveInternalAuthMode("", false)
		if err == nil {
			t.Fatalf("缺值必須回 error，實得 mode=%q", got)
		}
		if got != "" {
			t.Fatalf("回 error 時不得同時回傳模式，實得 %q", got)
		}
	})

	t.Run("無法辨識的值是 error，不是 permissive", func(t *testing.T) {

		for _, raw := range []string{"enforcng", "ENFORCE", "true", "1", "", "  "} {
			got, err := ResolveInternalAuthMode(raw, true)
			if err == nil {
				t.Fatalf("ResolveInternalAuthMode(%q, true) 必須回 error，實得 mode=%q", raw, got)
			}
		}
	})

	t.Run("error 訊息要指名 env 名稱與合法值", func(t *testing.T) {
		_, err := ResolveInternalAuthMode("enforcng", true)
		if err == nil {
			t.Fatal("expected error")
		}
		for _, want := range []string{envInternalAuthMode, InternalAuthModeOff, InternalAuthModePermissive, InternalAuthModeEnforcing} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error 訊息缺少 %q：%v", want, err)
			}
		}
	})
}

func TestLoadInternalAuthConfig_PanicsOnInvalid(t *testing.T) {
	t.Setenv(envInternalAuthMode, "enforcng")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("無法辨識的 INTERNAL_AUTH_MODE 必須 panic，實際沒有")
		}
		if !strings.Contains(fmt.Sprint(r), envInternalAuthMode) {
			t.Fatalf("panic 訊息要指名 env 名稱，實得：%v", r)
		}
	}()
	_ = InternalAuthServerInterceptor(nil)
}
