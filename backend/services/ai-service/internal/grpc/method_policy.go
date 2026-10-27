package grpc

import (
	"context"
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/actor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ActorRequirement 是一支 RPC 方法允許呼叫的 actor 種類。
type ActorRequirement int

const (
	ActorBuyer ActorRequirement = iota
	ActorSeller
	// ActorNone 是明確登記為免身分的方法：resolveActor 直接放行，不解析
	// metadata、不寫入 actor.Actor。授權完全交給 request message 本身
	// （例如一次性 token 欄位），不是 Gateway 斷言的 principal。
	// fail-closed（未登記方法一律拒絕）語意不變——這個值只是讓「明確聲明不
	// 需要 Gateway 斷言身分」變成合法登記狀態，不是「不受管轄」。
	ActorNone

	// ActorAdmin 是只有平台管理者可呼叫的方法。授權判準是 gateway 注入的
	// x-account-type == 4，與 gateway requireAdmin() 一致。不解析也不寫入
	// actor.Actor——admin 沒有 ADK session 分區（見 actor.ResolveAdminGRPCMetadata）。
	ActorAdmin
)

// MethodPolicy 是一支 RPC 方法的 method-policy registry 登記項。
type MethodPolicy struct {
	Actor ActorRequirement
}

// methodPolicies 是完整 gRPC method name（如 "/hawkr.ai.AIAssistantService/BuyerChat"）
// 到 MethodPolicy 的對照表。未登記的方法一律 fail closed 拒絕——見 UnaryMethodPolicyInterceptor
// / StreamMethodPolicyInterceptor。各 handler 檔案在自己的 init() 呼叫 registerMethodPolicy
// 登記，避免所有方法擠在同一份 map literal 裡造成跨 task 的 merge 衝突。
var methodPolicies = map[string]MethodPolicy{}

func registerMethodPolicy(fullMethod string, policy MethodPolicy) {
	methodPolicies[fullMethod] = policy
}

// infraMethodPrefixes 是 method-policy 的唯二豁免：gRPC 內建的 health 與
// reflection。它們不帶使用者身分（k8s 探活、grpcurl），納管等於讓 pod 永遠
// NotReady。
//
// 🔴 這裡刻意只有一份清單。前一版有兩份方向相反的表——service 前綴白名單
// （預設不管）與方法登記表（預設拒絕）——交集處的失效方向是 fail **open**：
// 登記了方法卻忘了加前綴，該方法就靜默放行。單表之後那個狀態不存在：沒登記
// 就是拒絕，方向只有一個。
var infraMethodPrefixes = []string{
	"/grpc.health.v1.Health/",
	"/grpc.reflection.v1.ServerReflection/",
	"/grpc.reflection.v1alpha.ServerReflection/",
}

func isInfraMethod(fullMethod string) bool {
	for _, p := range infraMethodPrefixes {
		if strings.HasPrefix(fullMethod, p) {
			return true
		}
	}
	return false
}

type actorContextKey struct{}

// ActorFromContext 取出 method-policy interceptor 解析好、寫入 context 的 actor.Actor。
func ActorFromContext(ctx context.Context) (actor.Actor, bool) {
	a, ok := ctx.Value(actorContextKey{}).(actor.Actor)
	return a, ok
}

// resolveActor 依 policy.Actor 決定要從 metadata 讀哪個 stateKey，解析失敗回
// Unauthenticated（表示呼叫有帶 registry 認得的方法，但身份本身有問題——跟
// PermissionDenied「方法未登記」語意分開）。
func resolveActor(ctx context.Context, policy MethodPolicy) (context.Context, error) {
	if policy.Actor == ActorNone {
		return ctx, nil
	}
	if policy.Actor == ActorAdmin {
		switch actor.ResolveAdminGRPCMetadata(ctx) {
		case actor.AdminMetadataAdmin:
			return ctx, nil
		case actor.AdminMetadataNotAdmin:
			// PermissionDenied：metadata 有帶 x-account-type，值只是不是 admin——
			// 身分解析得出來，只是類型不對。
			return ctx, status.Error(codes.PermissionDenied, "admin account type required")
		default: // actor.AdminMetadataAbsent
			// Unauthenticated：完全沒帶 x-account-type，跟 store/buyer 分支的
			// 「身分本身有問題」是同一類——client 沒帶身分，不是「身分不是 admin」。
			return ctx, status.Error(codes.Unauthenticated, "x-account-type metadata missing")
		}
	}
	stateKey := actor.StateKeyBuyer
	if policy.Actor == ActorSeller {
		stateKey = actor.StateKeyStore
	}
	a, ok := actor.ParseGRPCMetadata(ctx, stateKey)
	if !ok {
		return ctx, status.Error(codes.Unauthenticated, "actor identity missing or does not match method's required actor type")
	}
	return context.WithValue(ctx, actorContextKey{}, a), nil
}

// UnaryMethodPolicyInterceptor 回傳 unary server interceptor：查 methodPolicies，
// 未登記的方法一律拒絕（fail closed），已登記的方法驗證 actor type 並把解析好的
// actor.Actor 寫入 context 供 handler 用 ActorFromContext 取得。
func UnaryMethodPolicyInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if isInfraMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		policy, ok := methodPolicies[info.FullMethod]
		if !ok {
			return nil, status.Errorf(codes.PermissionDenied, "method %s is not registered in the method-policy registry", info.FullMethod)
		}
		ctx, err := resolveActor(ctx, policy)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// wrappedServerStream 讓 StreamMethodPolicyInterceptor 能把解析好的 context 換進
// grpc.ServerStream（ServerStream.Context() 預設回傳原始 ctx，這裡覆寫）。
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// StreamMethodPolicyInterceptor 是 StreamServerInterceptor 版本，語意與
// UnaryMethodPolicyInterceptor 相同。
func StreamMethodPolicyInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isInfraMethod(info.FullMethod) {
			return handler(srv, ss)
		}
		policy, ok := methodPolicies[info.FullMethod]
		if !ok {
			return status.Errorf(codes.PermissionDenied, "method %s is not registered in the method-policy registry", info.FullMethod)
		}
		ctx, err := resolveActor(ss.Context(), policy)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
	}
}
