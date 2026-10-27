package grpc

import (
	"context"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/actor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// 這些 fixture full method name 使用任意的 "/hawkr.ai.AIAssistantService/..."
// 前綴——單表 default-deny 之下沒有「受管轄前綴」這個概念，任何未登記的完整
// method name 都會被拒絕，前綴本身不影響測試行為。

func TestUnaryMethodPolicyInterceptor_FailClosed_UnregisteredMethod(t *testing.T) {
	registerMethodPolicy("/hawkr.ai.AIAssistantService/TestFixtureRegistered", MethodPolicy{Actor: ActorBuyer})
	interceptor := UnaryMethodPolicyInterceptor()

	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return nil, nil
	}
	md := metadata.Pairs("x-buyer-id", "123")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/hawkr.ai.AIAssistantService/TestFixtureNotRegistered"}, handler)
	if called {
		t.Fatal("handler was called for an unregistered method, want fail-closed rejection")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("err code = %v, want PermissionDenied", status.Code(err))
	}
}

func TestUnaryMethodPolicyInterceptor_ActorMismatch(t *testing.T) {
	registerMethodPolicy("/hawkr.ai.AIAssistantService/TestFixtureSellerOnly", MethodPolicy{Actor: ActorSeller})
	interceptor := UnaryMethodPolicyInterceptor()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}
	// 帶的是 buyer header，但方法要求 seller。
	md := metadata.Pairs("x-buyer-id", "123")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/hawkr.ai.AIAssistantService/TestFixtureSellerOnly"}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("err code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestUnaryMethodPolicyInterceptor_ValidActor_InjectsContext(t *testing.T) {
	registerMethodPolicy("/hawkr.ai.AIAssistantService/TestFixtureBuyerOnly", MethodPolicy{Actor: ActorBuyer})
	interceptor := UnaryMethodPolicyInterceptor()

	var gotActor actor.Actor
	var gotOK bool
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		gotActor, gotOK = ActorFromContext(ctx)
		return "ok", nil
	}
	md := metadata.Pairs("x-buyer-id", "2623900520")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/hawkr.ai.AIAssistantService/TestFixtureBuyerOnly"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotOK {
		t.Fatal("ActorFromContext returned ok=false inside handler, want true")
	}
	if gotActor.UserID != "2623900520" {
		t.Fatalf("actor.UserID = %q, want 2623900520", gotActor.UserID)
	}
}

// 未登記的方法一律拒絕——這是單表 default-deny 的全部內容。
// 以前這裡要分「在管轄內但未登記」與「不在管轄內」兩種情形，前者拒絕、後者放行；
// 後者就是 fail-open 的來源，現在它不存在了。
func TestUnaryUnregisteredMethodIsDenied(t *testing.T) {
	interceptor := UnaryMethodPolicyInterceptor()
	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	}

	_, err := interceptor(t.Context(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/hawkr.ai.SomeUnknownService/Whatever"}, handler)

	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("code = %v, want PermissionDenied", status.Code(err))
	}
	if handlerCalled {
		t.Error("handler was called for an unregistered method")
	}
}

// fakeServerStream 是最小可用的 grpc.ServerStream 假實作，只提供
// StreamMethodPolicyInterceptor 會用到的 Context()。
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context {
	return f.ctx
}

// health / reflection 是唯二豁免：k8s 探活與 grpcurl 都不帶使用者身分，
// 把它們納管等於讓 pod 永遠 NotReady。
//
// unary 與 stream 兩條 interceptor chain 都要驗證：runtime 上這幾支方法實際
// 走的 RPC 型別不同——grpc.health.v1.Health/Check 是 unary，Health/Watch 是
// server-streaming，ServerReflectionInfo（v1 與 v1alpha 皆然）是
// bidi-streaming，而 di.go 把 StreamMethodPolicyInterceptor 掛在 stream chain
// 上（EnableStreamInterceptors: true）。只測 unary 版本的話，stream 側
// infraMethodPrefixes 前綴打錯字或漏加不會有任何測試變紅——grpcurl 與
// streaming health check 會在 runtime 全斷，而這正是 fix round 1 review 抓到
// 的缺口（上一輪把守 stream 路徑放行的測試整支刪掉，沒有留替代覆蓋）。
func TestInfraMethodsAreExempt(t *testing.T) {
	unary := UnaryMethodPolicyInterceptor()
	for _, m := range []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
		"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
	} {
		called := false
		handler := func(ctx context.Context, req any) (any, error) { called = true; return nil, nil }
		if _, err := unary(t.Context(), nil, &grpc.UnaryServerInfo{FullMethod: m}, handler); err != nil {
			t.Errorf("unary %s: unexpected error %v", m, err)
		}
		if !called {
			t.Errorf("unary %s: handler was not called", m)
		}
	}

	stream := StreamMethodPolicyInterceptor()
	for _, m := range []string{
		"/grpc.health.v1.Health/Watch",
		"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
	} {
		called := false
		handler := func(srv any, ss grpc.ServerStream) error { called = true; return nil }
		fakeStream := &fakeServerStream{ctx: t.Context()}
		if err := stream(nil, fakeStream, &grpc.StreamServerInfo{FullMethod: m}, handler); err != nil {
			t.Errorf("stream %s: unexpected error %v", m, err)
		}
		if !called {
			t.Errorf("stream %s: handler was not called", m)
		}
	}
}

// ActorAdmin：帶 x-account-type=4 放行，其餘拒絕。
func TestActorAdminPolicy(t *testing.T) {
	const method = "/hawkr.ai.AIAdminService/GetAiQuota"
	interceptor := UnaryMethodPolicyInterceptor()

	adminCtx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-account-type", "4"))
	called := false
	handler := func(ctx context.Context, req any) (any, error) { called = true; return nil, nil }
	if _, err := interceptor(adminCtx, nil, &grpc.UnaryServerInfo{FullMethod: method}, handler); err != nil {
		t.Fatalf("admin call rejected: %v", err)
	}
	if !called {
		t.Fatal("handler not called for admin")
	}

	sellerCtx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-account-type", "2"))
	called = false
	_, err := interceptor(sellerCtx, nil, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("code = %v, want PermissionDenied", status.Code(err))
	}
	// 光憑 PermissionDenied 分不出「ActorAdmin 分支真的拒絕了非 admin」與
	// 「GetAiQuota 根本沒登記」——未登記方法也回同一個 code（見
	// TestUnaryUnregisteredMethodIsDenied）。斷言到訊息內容，鎖定是
	// resolveActor 的 admin 分支在說話。
	if err == nil || !strings.Contains(err.Error(), "admin account type required") {
		t.Errorf("err = %v, want message containing %q", err, "admin account type required")
	}
	if called {
		t.Error("handler was called for a non-admin caller")
	}

	// 完全沒帶 x-account-type（client 沒帶身分）必須是 Unauthenticated，
	// 跟「帶了但值不是 admin」的 PermissionDenied 分開——on-call 才分得出
	// 是 client 沒帶 header 還是新方法忘了註冊（15 號設計書 §8.1）。
	noHeaderCtx := metadata.NewIncomingContext(t.Context(), metadata.MD{})
	called = false
	_, err = interceptor(noHeaderCtx, nil, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", status.Code(err))
	}
	if called {
		t.Error("handler was called for a caller with no x-account-type metadata")
	}
}

func TestResolveActor_ActorNone_SkipsMetadataParsing_NoActorInContext(t *testing.T) {
	ctx := context.Background() // no metadata at all
	policy := MethodPolicy{Actor: ActorNone}

	gotCtx, err := resolveActor(ctx, policy)
	if err != nil {
		t.Fatalf("resolveActor with ActorNone returned error: %v", err)
	}
	if _, ok := ActorFromContext(gotCtx); ok {
		t.Fatalf("ActorFromContext should return ok=false for an ActorNone-resolved context, got an actor")
	}
}
