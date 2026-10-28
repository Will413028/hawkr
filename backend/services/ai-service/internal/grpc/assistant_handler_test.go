package grpc

import (
	"context"
	"strings"
	"testing"
	"time"

	actorpkg "github.com/Will413028/hawkr/ai-service/internal/actor"
	"github.com/Will413028/hawkr/ai-service/internal/cache"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeAssistantStream 實作 pb.AIAssistantService_SellerChatServer 等 stream server
// 介面的最小子集合（Send + Context），足以驅動 GRPCEventSink 並讓測試檢查送出的事件。
type fakeAssistantStream struct {
	ctx  context.Context
	sent []*pb.StreamEvent
}

func (f *fakeAssistantStream) Send(ev *pb.StreamEvent) error {
	f.sent = append(f.sent, ev)
	return nil
}
func (f *fakeAssistantStream) Context() context.Context { return f.ctx }

// 以下四個方法是 grpc.ServerStream 介面要求但本測試不會用到的部分，回傳零值即可。
func (f *fakeAssistantStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeAssistantStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeAssistantStream) SetTrailer(metadata.MD)       {}
func (f *fakeAssistantStream) SendMsg(m interface{}) error  { return nil }
func (f *fakeAssistantStream) RecvMsg(m interface{}) error  { return nil }

func TestSellerInventory_RegistersActorSeller(t *testing.T) {
	// 驗證 method-policy registry 有登記這支方法、且要求 seller actor —— 不需要真的
	// 跑 ADK runner（那需要真的 model API key），只驗證 registry 登記與 actor 解析
	// 這兩層本 task 新增的邏輯。
	fullMethod := "/hawkr.ai.AIAssistantService/SellerInventory"
	policy, ok := methodPolicies[fullMethod]
	if !ok {
		t.Fatalf("%s not registered in methodPolicies", fullMethod)
	}
	if policy.Actor != ActorSeller {
		t.Fatalf("policy.Actor = %v, want ActorSeller", policy.Actor)
	}
}

func TestBuyerTracking_RegistersActorBuyer(t *testing.T) {
	fullMethod := "/hawkr.ai.AIAssistantService/BuyerTracking"
	policy, ok := methodPolicies[fullMethod]
	if !ok {
		t.Fatalf("%s not registered in methodPolicies", fullMethod)
	}
	if policy.Actor != ActorBuyer {
		t.Fatalf("policy.Actor = %v, want ActorBuyer", policy.Actor)
	}
}

func TestAllEightGenericStreamingMethods_Registered(t *testing.T) {
	want := []string{
		"/hawkr.ai.AIAssistantService/SellerChat",
		"/hawkr.ai.AIAssistantService/SellerInventory",
		"/hawkr.ai.AIAssistantService/SellerPayment",
		"/hawkr.ai.AIAssistantService/SellerProductListingChat",
		"/hawkr.ai.AIAssistantService/BuyerOrders",
		"/hawkr.ai.AIAssistantService/BuyerTracking",
		"/hawkr.ai.AIAssistantService/BuyerSellerMatch",
		"/hawkr.ai.AIAssistantService/BuyerChat",
	}
	for _, m := range want {
		if _, ok := methodPolicies[m]; !ok {
			t.Errorf("%s not registered in methodPolicies", m)
		}
	}
}

func TestNewAIAssistantGrpcServer_ImplementsInterface(t *testing.T) {
	var _ pb.AIAssistantServiceServer = (*AIAssistantGrpcServer)(nil)
}

func TestSellerOrders_Registered(t *testing.T) {
	policy, ok := methodPolicies["/hawkr.ai.AIAssistantService/SellerOrders"]
	if !ok {
		t.Fatal("SellerOrders not registered")
	}
	if policy.Actor != ActorSeller {
		t.Fatalf("policy.Actor = %v, want ActorSeller", policy.Actor)
	}
}

// 以下三個測試透過 fakeAssistantStream 直接驅動 genericStreamRPC，鎖定本 task
// 新增的三段早期回傳邏輯——全部發生在 RunAssistantStream 真的呼叫 r.Run 之前
// （nil-runner 防護是 RunAssistantStream 進迴圈前的第一件事），所以不需要真的
// ADK model API key 也能安全測試。

func TestGenericStreamRPC_NoActorInContext_ReturnsPermissionDenied(t *testing.T) {
	s := &AIAssistantGrpcServer{}
	stream := &fakeAssistantStream{ctx: context.Background()} // 沒有寫入 actorContextKey
	err := s.genericStreamRPC(stream, &pb.AssistantRequest{Message: "hi"}, nil, "")

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("err = %v, want status.Code() == PermissionDenied", err)
	}
	if len(stream.sent) != 0 {
		t.Fatalf("sent %d events, want 0 (應在觸碰 sink 之前就失敗)", len(stream.sent))
	}
}

func TestGenericStreamRPC_EmptyMessageNoDefault_ReturnsInvalidArgument(t *testing.T) {
	s := &AIAssistantGrpcServer{}
	ctx := context.WithValue(context.Background(), actorContextKey{}, actorpkg.NewStoreActor(12345))
	stream := &fakeAssistantStream{ctx: ctx}
	// endpointKey="" 對應 chat 端點，DefaultMessages 沒有 "" 這個 key。
	err := s.genericStreamRPC(stream, &pb.AssistantRequest{}, nil, "")

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, want status.Code() == InvalidArgument", err)
	}
	if len(stream.sent) != 0 {
		t.Fatalf("sent %d events, want 0 (應在觸碰 sink 之前就失敗)", len(stream.sent))
	}
}

func TestGenericStreamRPC_NilRunner_ReturnsInternal(t *testing.T) {
	s := &AIAssistantGrpcServer{}
	ctx := context.WithValue(context.Background(), actorContextKey{}, actorpkg.NewStoreActor(12345))
	stream := &fakeAssistantStream{ctx: ctx}
	// r=nil 觸發 RunAssistantStream 的 ErrRunnerNotReady 早期回傳（在 r.Run 之前）。
	err := s.genericStreamRPC(stream, &pb.AssistantRequest{Message: "hi"}, nil, "")

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("err = %v, want status.Code() == Internal", err)
	}
	if len(stream.sent) != 0 {
		t.Fatalf("sent %d events, want 0（nil runner 應在建立/使用 sink 之前就回錯）", len(stream.sent))
	}
}

func TestBuyerReturnPolicy_SendsTextThenDone_NoLLM(t *testing.T) {
	registerMethodPolicy("/hawkr.ai.AIAssistantService/BuyerReturnPolicy", MethodPolicy{Actor: ActorBuyer})
	md := metadata.Pairs("x-buyer-id", "2623900520")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	fake := &fakeAssistantStream{ctx: ctx}

	s := NewAIAssistantGrpcServer(nil, nil, nil, nil, nil) // 5 個參數對齊本 plan 最終建構子（runners, cfg, sessionSvc, orderClient, categoryIndex）；本測試全部用不到，故全 nil——這支不走 LLM
	// 先手動跑過 method-policy interceptor 邏輯讓 actor 進 context（單元測試不透過真正的 gRPC server）。
	act, _ := actorpkg.ParseGRPCMetadata(ctx, actorpkg.StateKeyBuyer)
	ctx = context.WithValue(ctx, actorContextKey{}, act)
	fake.ctx = ctx

	err := s.BuyerReturnPolicy(&pb.AssistantRequest{}, fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.sent) != 2 {
		t.Fatalf("sent %d events, want 2 (text, done)", len(fake.sent))
	}
	if fake.sent[0].GetText() == nil {
		t.Fatal("first event is not TextEvent")
	}
	if fake.sent[1].GetDone() == nil {
		t.Fatal("second event is not DoneEvent")
	}
	if fake.sent[1].GetDone().GetSessionId() != "" {
		t.Fatalf("done.session_id = %q, want empty string (this endpoint never creates a session)", fake.sent[1].GetDone().GetSessionId())
	}
}

// 以下四個測試鎖定 SellerProductAutofill 早期回傳邏輯（categoryIndex.Ready() →
// 空文字 → 過長文字），全部發生在真的呼叫 ADK/Gemini 之前，不需要 model API key。
// 驗證順序刻意與 HTTP 版 SellerAssistantHandler.ProductAutofill 一致：
// categoryIndex.Ready() 在文字驗證之前——這是既有行為，不是新設計選擇，所以
// 「categoryIndex 未就緒」與「文字驗證」的測試都需要一個非 nil 的
// categoryIndex（cache.CategoryIndex.Ready() 對 nil receiver 不是 nil-safe，
// 會 panic），不能沿用其餘測試常見的全 nil 建構模式。

// #3330：改為 FailedPrecondition —— 「分類資料未就緒」是資料狀態，服務本身健康，
// 而 gateway 的 circuit breaker 把 Unavailable 當 transport 失敗計入（per-target 共用一顆）。
func TestSellerProductAutofill_CategoryIndexNotReady_ReturnsFailedPrecondition(t *testing.T) {
	// 零值 *cache.CategoryIndex 是非 nil pointer，Ready() 回傳 false（未 warm-up）。
	s := NewAIAssistantGrpcServer(nil, nil, nil, nil, &cache.CategoryIndex{})
	md := metadata.Pairs("x-store-id", "2685914682")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	act, _ := actorpkg.ParseGRPCMetadata(ctx, actorpkg.StateKeyStore)
	ctx = context.WithValue(ctx, actorContextKey{}, act)

	_, err := s.SellerProductAutofill(ctx, &pb.SellerProductAutofillRequest{Text: "新鮮台灣芭樂一箱"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("err code = %v, want FailedPrecondition", status.Code(err))
	}
}

func TestSellerProductAutofill_EmptyText_ReturnsInvalidArgument(t *testing.T) {
	// categoryIndex 必須是已就緒的非 nil index，讓 Ready() 先過關，
	// 才能真的測到接下來的空文字檢查（而不是被 FailedPrecondition 蓋過去）。
	s := NewAIAssistantGrpcServer(nil, nil, nil, nil, cache.NewReadyCategoryIndex(nil))
	md := metadata.Pairs("x-store-id", "2685914682")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	act, _ := actorpkg.ParseGRPCMetadata(ctx, actorpkg.StateKeyStore)
	ctx = context.WithValue(ctx, actorContextKey{}, act)

	_, err := s.SellerProductAutofill(ctx, &pb.SellerProductAutofillRequest{Text: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("err code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestSellerProductAutofill_TextTooLong_ReturnsInvalidArgument(t *testing.T) {
	s := NewAIAssistantGrpcServer(nil, nil, nil, nil, cache.NewReadyCategoryIndex(nil))
	md := metadata.Pairs("x-store-id", "2685914682")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	act, _ := actorpkg.ParseGRPCMetadata(ctx, actorpkg.StateKeyStore)
	ctx = context.WithValue(ctx, actorContextKey{}, act)

	longText := strings.Repeat("字", 2001)
	_, err := s.SellerProductAutofill(ctx, &pb.SellerProductAutofillRequest{Text: longText})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("err code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestSellerProductAutofill_Registered(t *testing.T) {
	policy, ok := methodPolicies["/hawkr.ai.AIAssistantService/SellerProductAutofill"]
	if !ok || policy.Actor != ActorSeller {
		t.Fatal("SellerProductAutofill not registered as ActorSeller")
	}
}

func TestSellerLatestSession_And_BuyerLatestSession_Registered(t *testing.T) {
	for method, want := range map[string]ActorRequirement{
		"/hawkr.ai.AIAssistantService/SellerLatestSession": ActorSeller,
		"/hawkr.ai.AIAssistantService/BuyerLatestSession":  ActorBuyer,
	} {
		policy, ok := methodPolicies[method]
		if !ok {
			t.Errorf("%s not registered", method)
			continue
		}
		if policy.Actor != want {
			t.Errorf("%s actor = %v, want %v", method, policy.Actor, want)
		}
	}
}

// --- SellerLatestSession／BuyerLatestSession behavior tests ---
//
// 除了上面 registry 登記測試，這裡額外用真的 session.InMemoryService() 跑一次
// end-to-end 路徑，鎖住 toLatestSessionResponse 的 pb 欄位對應（SessionId/
// Messages[].Role/Text/Timestamp）沒有接錯。

func TestSellerLatestSession_NoActorInContext_ReturnsPermissionDenied(t *testing.T) {
	s := &AIAssistantGrpcServer{}
	_, err := s.SellerLatestSession(context.Background(), &pb.LatestSessionRequest{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("err code = %v, want PermissionDenied", status.Code(err))
	}
}

func TestBuyerLatestSession_NoActorInContext_ReturnsPermissionDenied(t *testing.T) {
	s := &AIAssistantGrpcServer{}
	_, err := s.BuyerLatestSession(context.Background(), &pb.LatestSessionRequest{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("err code = %v, want PermissionDenied", status.Code(err))
	}
}

func TestSellerLatestSession_ReturnsLatestSessionData(t *testing.T) {
	svc := session.InMemoryService()
	actor := actorpkg.NewStoreActor(2685914682)
	ctx := context.Background()
	created, err := svc.Create(ctx, &session.CreateRequest{
		AppName: "hawkr_ai",
		UserID:  actor.UserID,
		State:   map[string]any{actor.StateKey: actor.StateVal},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	ts := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	if err := svc.AppendEvent(ctx, created.Session, &session.Event{
		ID:        "evt1",
		Author:    genai.RoleModel,
		Timestamp: ts,
		LLMResponse: model.LLMResponse{
			Content: genai.NewContentFromText("庫存還有 20 箱", genai.RoleModel),
		},
	}); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	s := NewAIAssistantGrpcServer(nil, nil, svc, nil, nil)
	reqCtx := context.WithValue(context.Background(), actorContextKey{}, actor)
	resp, err := s.SellerLatestSession(reqCtx, &pb.LatestSessionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSessionId() != created.Session.ID() {
		t.Errorf("SessionId = %q, want %q", resp.GetSessionId(), created.Session.ID())
	}
	if len(resp.GetMessages()) != 1 {
		t.Fatalf("Messages = %+v, want 1 item", resp.GetMessages())
	}
	got := resp.GetMessages()[0]
	if got.GetRole() != "model" || got.GetText() != "庫存還有 20 箱" || got.GetTimestamp() != ts.UTC().Format(time.RFC3339) {
		t.Errorf("Messages[0] = role=%q text=%q timestamp=%q, want role=model text=%q timestamp=%q",
			got.GetRole(), got.GetText(), got.GetTimestamp(), "庫存還有 20 箱", ts.UTC().Format(time.RFC3339))
	}
}

func TestBuyerLatestSession_NoSessions_ReturnsEmptyNotNilMessages(t *testing.T) {
	svc := session.InMemoryService()
	actor := actorpkg.NewBuyerActor(555)

	s := NewAIAssistantGrpcServer(nil, nil, svc, nil, nil)
	ctx := context.WithValue(context.Background(), actorContextKey{}, actor)
	resp, err := s.BuyerLatestSession(ctx, &pb.LatestSessionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSessionId() != "" {
		t.Errorf("SessionId = %q, want empty", resp.GetSessionId())
	}
	if resp.GetMessages() == nil {
		t.Error("Messages is nil, want non-nil empty slice")
	}
	if len(resp.GetMessages()) != 0 {
		t.Errorf("len(Messages) = %d, want 0", len(resp.GetMessages()))
	}
}
