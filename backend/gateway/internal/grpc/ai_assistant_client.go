package grpc

import (
	"context"
	"time"

	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc"
)

// AssistantStream is the common shape shared by all 12 AIAssistantService
// server-streaming RPC clients (SellerChat, BuyerChat, ...). protoc-gen-go-grpc
// generates a distinct named interface per RPC even though their method sets
// are identical — this local interface lets the SSE translation layer
// (gateway/internal/proxy/ai_assistant_stream.go) consume any of them without
// knowing which RPC produced it.
type AssistantStream interface {
	Recv() (*aipb.StreamEvent, error)
	grpc.ClientStream
}

// AIAssistantClient is the gRPC client for ai-service's AIAssistantService
// (Phase A, PR #2438) — the assistant-domain RPCs Gateway calls directly as
// of Phase B instead of proxying HTTP.
type AIAssistantClient struct {
	conn   *grpc.ClientConn
	client aipb.AIAssistantServiceClient
}

// NewAIAssistantClient wraps the shared ai-service connection (see
// ai_service_conn.go) with the AIAssistantService stub — the same
// connection NewAIClient uses for its AIImageService/AIAdminService/
// RecommendationService stubs (the three services #3519 split the legacy
// AIService into), not a second dial.
func NewAIAssistantClient(shared *aiServiceConn) *AIAssistantClient {
	return &AIAssistantClient{
		conn:   shared.conn,
		client: aipb.NewAIAssistantServiceClient(shared.conn),
	}
}

// ---- 5 支賣家 stream RPC ----

func (c *AIAssistantClient) SellerChat(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.SellerChat(ctx, req)
}

func (c *AIAssistantClient) SellerOrders(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.SellerOrders(ctx, req)
}

func (c *AIAssistantClient) SellerInventory(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.SellerInventory(ctx, req)
}

func (c *AIAssistantClient) SellerPayment(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.SellerPayment(ctx, req)
}

func (c *AIAssistantClient) SellerProductListingChat(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.SellerProductListingChat(ctx, req)
}

// ---- 5 支買家 stream RPC ----

func (c *AIAssistantClient) BuyerChat(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.BuyerChat(ctx, req)
}

func (c *AIAssistantClient) BuyerOrders(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.BuyerOrders(ctx, req)
}

func (c *AIAssistantClient) BuyerTracking(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.BuyerTracking(ctx, req)
}

func (c *AIAssistantClient) BuyerReturnPolicy(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.BuyerReturnPolicy(ctx, req)
}

func (c *AIAssistantClient) BuyerSellerMatch(ctx context.Context, req *aipb.AssistantRequest) (AssistantStream, error) {
	return c.client.BuyerSellerMatch(ctx, req)
}

// ---- 3 支 unary RPC ----
//
// 這三支帶自己的 deadline（CLAUDE.md §12：禁止無 deadline 的跨服務呼叫），
// 與 AIClient 每支方法的既有做法一致。stream RPC 不在此列：它們的存活時間由
// 呼叫端的 request context 與 proxy.ServeAIAssistantStream 的 idle/write
// timeout 控制，硬套一個總長度上限會攔腰砍掉正常的長對話。

// autofillTimeout 對齊 ai-service ProductAutofill handler 自己的 30 秒預算
// （services/ai-service/internal/handler/seller_assistant.go）——比它短會在
// 上游還在正常工作時就放棄，比它長則只是白等一個必定已經放棄的呼叫。
const autofillTimeout = 30 * time.Second

// latestSessionTimeout：這兩支只是讀 session 歷史，沒有 LLM 呼叫，用與
// AIClient 其他純讀取方法相同的較短預算。
const latestSessionTimeout = 15 * time.Second

func (c *AIAssistantClient) SellerProductAutofill(ctx context.Context, req *aipb.SellerProductAutofillRequest) (*aipb.SellerProductAutofillResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, autofillTimeout)
	defer cancel()

	return c.client.SellerProductAutofill(ctx, req)
}

func (c *AIAssistantClient) SellerLatestSession(ctx context.Context, req *aipb.LatestSessionRequest) (*aipb.LatestSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, latestSessionTimeout)
	defer cancel()

	return c.client.SellerLatestSession(ctx, req)
}

func (c *AIAssistantClient) BuyerLatestSession(ctx context.Context, req *aipb.LatestSessionRequest) (*aipb.LatestSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, latestSessionTimeout)
	defer cancel()

	return c.client.BuyerLatestSession(ctx, req)
}
