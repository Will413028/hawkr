package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

// unregisteredTestService 是一支只存在於這個測試的 gRPC service：它被註冊到
// *grpc.Server（所以 grpc-go 會把呼叫路由進 interceptor chain），但**沒有**登記
// 進 methodPolicies。
//
// 🔴 它為什麼必須存在：#3519 之前這個角色由 AIService 的死 RPC（EnhanceImage /
// ChatCompletionStream）擔任——註冊在 server 上、卻刻意不登記 policy。那批 RPC
// 已經連同 proto 定義一起刪除，於是真實服務裡再也沒有「有註冊、沒登記」的方法，
// 而那正是 default-deny 唯一會被觸發的情境。要繼續端到端驗證它，就得自己造一支。
//
// 🔴 不能改用「隨便打一個不存在的 method name」當替代：grpc-go 查不到方法時直接
// 回 Unimplemented，**根本不會進 interceptor chain**——那樣的測試在 interceptor
// 被整個拔掉時依然會「通過」，是零鑑別力的假綠燈。
const (
	unregisteredTestUnaryMethod  = "/hawkr.ai.TestOnlyUnregisteredService/Unary"
	unregisteredTestStreamMethod = "/hawkr.ai.TestOnlyUnregisteredService/Stream"
)

// unregisteredTestServiceDesc 手寫一份最小的 ServiceDesc，形狀比照 protoc-gen-go-grpc
// 的產物：unary handler 自己呼叫 server 傳進來的 interceptor（grpc-go 對 unary 的
// 約定），stream 則由 grpc-go 在呼叫 Handler 之前自己套 stream interceptor。
// handlerCalled 只在 interceptor 放行時才會被設成 true。
func unregisteredTestServiceDesc(handlerCalled *bool) *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: "hawkr.ai.TestOnlyUnregisteredService",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Unary",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(emptypb.Empty)
				if err := dec(in); err != nil {
					return nil, err
				}
				handler := func(context.Context, any) (any, error) {
					*handlerCalled = true
					return &emptypb.Empty{}, nil
				}
				if interceptor == nil {
					return handler(ctx, in)
				}
				return interceptor(ctx, in, &grpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: unregisteredTestUnaryMethod,
				}, handler)
			},
		}},
		Streams: []grpc.StreamDesc{{
			StreamName: "Stream",
			Handler: func(_ any, stream grpc.ServerStream) error {
				*handlerCalled = true
				return stream.SendMsg(&emptypb.Empty{})
			},
			ServerStreams: true,
		}},
		Metadata: "di_test.go",
	}
}

// TestGRPCServer_MethodPolicyEndToEnd 是最接近 grpcurl -plaintext list 那種
// 端到端訊號、但不需要真實 DB 的驗證：直接呼叫 NewGRPCServer（略過 fx，繞開 DI
// 需要真實 Postgres 才能啟動的問題），用 bufconn 撥號，各打一次真正的 RPC，
// 涵蓋 unary 與 stream 兩條 interceptor chain。
//
// 單表 default-deny 之下不再有「不受管轄」這個狀態：
//   - TestOnlyUnregisteredService.Unary（註冊在 server 上但未登記 policy，見上方
//     型別註解）——必須是 PermissionDenied，證明未登記方法真的被 fail-closed
//     擋下，而不是靜默放行進 handler。
//   - AIAssistantService.BuyerLatestSession（已登記 unary、要求 buyer actor）
//     在完全不帶 actor metadata 的情況下呼叫——必須是 Unauthenticated（method
//     -policy interceptor 的 resolveActor 專屬錯誤碼，見 method_policy.go），
//     證明真的是 method-policy 這層在擋，而不是 handler 內部自己的
//     ActorFromContext 檢查在擋。
//   - AIAssistantService.BuyerChat（已登記 stream、要求 buyer actor）在完全
//     不帶 actor metadata 的情況下呼叫——必須是 Unauthenticated，且完全收不到
//     任何 StreamEvent，證明 di.go 的 ChainStreamInterceptor 真的把
//     StreamMethodPolicyInterceptor 接上了，不是只有 unary chain 有效。
//   - TestOnlyUnregisteredService.Stream（同樣未登記的 stream）——必須是
//     PermissionDenied，證明 stream chain 對未登記方法一樣 fail-closed。
func TestGRPCServer_MethodPolicyEndToEnd(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	logger := zap.NewNop()
	handler := &AIGrpcServer{}
	assistantHandler := &AIAssistantGrpcServer{}
	server := NewGRPCServer(handler, assistantHandler, logger)

	// 必須在 Serve 之前註冊（grpc-go 的 RegisterService 約定）。
	unregisteredHandlerCalled := false
	server.RegisterService(unregisteredTestServiceDesc(&unregisteredHandlerCalled), struct{}{})

	go func() {
		_ = server.Serve(lis)
	}()
	t.Cleanup(server.Stop)

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 未登記的 unary：呼叫必須是 PermissionDenied——證明單表 default-deny 真的
	// 擋在 handler 之前，而不是放行讓 handler 自己回應。
	err = conn.Invoke(ctx, unregisteredTestUnaryMethod, &emptypb.Empty{}, &emptypb.Empty{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unregistered unary: err code = %v, want PermissionDenied (unregistered method must be fail-closed)", status.Code(err))
	}

	// 新 AIAssistantService（unary）：已登記進 methodPolicies，要求 buyer
	// actor；完全不帶 actor metadata 呼叫必須被拒絕。斷言精確到
	// codes.Unauthenticated（method-policy interceptor 的 resolveActor 專屬
	// 錯誤碼）而非只檢查「有錯誤」，因為 handler 內部自己的 ActorFromContext
	// 檢查對這個情境也會回錯誤，太籠統的斷言分不出真正擋下呼叫的是哪一層。
	assistantClient := pb.NewAIAssistantServiceClient(conn)
	_, err = assistantClient.BuyerLatestSession(ctx, &pb.LatestSessionRequest{})
	if err == nil {
		t.Fatal("BuyerLatestSession: expected rejection with no actor metadata, got nil error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("BuyerLatestSession: err code = %v, want Unauthenticated (method-policy interceptor's resolveActor rejection)", status.Code(err))
	}

	// 新 AIAssistantService（stream）：BuyerChat 同樣已登記、要求 buyer
	// actor，完全不帶 actor metadata 呼叫必須被拒絕——這是 stream 路徑的
	// 端到端證明，StreamMethodPolicyInterceptor 是否真的被
	// ChainStreamInterceptor 接上，只有這種真實 RPC 呼叫才驗得到。
	//
	// Server-streaming RPC 的 client stub（BuyerChat）在建立 stream 階段只是
	// 把 request 寫上線，不會同步等伺服器回應；interceptor 的拒絕只會在第一次
	// Recv() 才浮現，所以斷言要打在 Recv() 的回傳值上。
	buyerChatStream, err := assistantClient.BuyerChat(ctx, &pb.AssistantRequest{
		SessionId: "test-session",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("BuyerChat: unexpected error initiating stream: %v", err)
	}
	event, err := buyerChatStream.Recv()
	if err == nil {
		t.Fatalf("BuyerChat: expected rejection with no actor metadata, got StreamEvent: %v", event)
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("BuyerChat: err code = %v, want Unauthenticated (method-policy interceptor's resolveActor rejection, stream path)", status.Code(err))
	}
	if event != nil {
		t.Fatalf("BuyerChat: got non-nil StreamEvent %v alongside rejection error, want nil", event)
	}

	// 未登記的 stream：鏡射上面 unary 的驗證，證明 stream chain 對未登記方法
	// 也是 fail-closed。
	unregisteredStream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, unregisteredTestStreamMethod)
	if err != nil {
		t.Fatalf("unregistered stream: unexpected error initiating stream: %v", err)
	}
	if err := unregisteredStream.SendMsg(&emptypb.Empty{}); err != nil && status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unregistered stream: SendMsg err = %v, want nil or PermissionDenied", err)
	}
	_ = unregisteredStream.CloseSend()
	if err := unregisteredStream.RecvMsg(&emptypb.Empty{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unregistered stream: err code = %v, want PermissionDenied (unregistered stream method must be fail-closed)", status.Code(err))
	}

	// 兩條路徑都不得抵達 handler。少了這個斷言，上面兩個 PermissionDenied 有可能
	// 是「handler 跑完之後才被別的東西轉成錯誤」——那不是 fail-closed。
	if unregisteredHandlerCalled {
		t.Fatal("unregistered service handler was reached; method-policy must reject before the handler runs")
	}
}
