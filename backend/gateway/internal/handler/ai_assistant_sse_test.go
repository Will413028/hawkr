package handler

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

var ioEOFForTest = io.EOF

// fakeSellerChatStream is a minimal grpcclient.AssistantStream implementation for testing.
// It sends one text message and then EOF, validating that the handler properly passes
// the gRPC client stream to proxy.ServeAIAssistantStream.
type fakeSellerChatStream struct {
	grpc.ClientStream
	sent bool
}

func (s *fakeSellerChatStream) Recv() (*aipb.StreamEvent, error) {
	if !s.sent {
		s.sent = true
		return &aipb.StreamEvent{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "您好"}}}, nil
	}
	return nil, ioEOFForTest
}

type mockAIAssistantStreamClient struct {
	aipb.AIAssistantServiceClient
	gotSellerChatReq *aipb.AssistantRequest
	sellerChatStream *fakeSellerChatStream
}

func (m *mockAIAssistantStreamClient) SellerChat(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_SellerChatClient, error) {
	m.gotSellerChatReq = in
	return m.sellerChatStream, nil
}

func TestProxyAIAssistantSSE_CallsSellerChatWithStoreIDContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockAIAssistantStreamClient{sellerChatStream: &fakeSellerChatStream{}}
	h := &Holder{
		AIAssistant: grpcclient.NewAIAssistantClientForTest(mock),
		Store:       storeResolverForTest(8001),
	}

	body := `{"message":"幫我查訂單"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("accountId", int64(42))

	h.ProxyAIAssistantSSE(c)

	if mock.gotSellerChatReq.GetMessage() != "幫我查訂單" {
		t.Fatalf("request not mapped: %+v", mock.gotSellerChatReq)
	}
	// AG-UI SSE frames have no `event:` line — the receiver dispatches purely
	// on the `type` field (see proxy/ag_ui_events.go's doc comment).
	if !strings.Contains(w.Body.String(), `"type":"TEXT_MESSAGE_CONTENT"`) {
		t.Fatalf("expected a TEXT_MESSAGE_CONTENT frame in SSE output, got %q", w.Body.String())
	}
}

type mockAIAssistantStreamClientMulti struct {
	aipb.AIAssistantServiceClient
	calls  map[string]*aipb.AssistantRequest
	stream *fakeSellerChatStream // 5 支共用一顆最小 fake stream，測試只驗證有沒有被正確呼叫
}

func newMockAIAssistantStreamClientMulti() *mockAIAssistantStreamClientMulti {
	return &mockAIAssistantStreamClientMulti{calls: map[string]*aipb.AssistantRequest{}, stream: &fakeSellerChatStream{}}
}

func (m *mockAIAssistantStreamClientMulti) SellerOrders(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_SellerOrdersClient, error) {
	m.calls["SellerOrders"] = in
	return m.stream, nil
}
func (m *mockAIAssistantStreamClientMulti) SellerInventory(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_SellerInventoryClient, error) {
	m.calls["SellerInventory"] = in
	return m.stream, nil
}
func (m *mockAIAssistantStreamClientMulti) SellerPayment(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_SellerPaymentClient, error) {
	m.calls["SellerPayment"] = in
	return m.stream, nil
}
func (m *mockAIAssistantStreamClientMulti) SellerProductListingChat(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_SellerProductListingChatClient, error) {
	m.calls["SellerProductListingChat"] = in
	return m.stream, nil
}

func TestSellerSSEHandlers_CallCorrectRPC(t *testing.T) {
	tests := []struct {
		name    string
		handler func(h *Holder, c *gin.Context)
		rpcName string
	}{
		{"orders", func(h *Holder, c *gin.Context) { h.ProxySellerOrdersSSE(c) }, "SellerOrders"},
		{"inventory", func(h *Holder, c *gin.Context) { h.ProxySellerInventorySSE(c) }, "SellerInventory"},
		{"payment", func(h *Holder, c *gin.Context) { h.ProxySellerPaymentSSE(c) }, "SellerPayment"},
		{"product-listing", func(h *Holder, c *gin.Context) { h.ProxySellerProductListingSSE(c) }, "SellerProductListingChat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			mock := newMockAIAssistantStreamClientMulti()
			h := &Holder{AIAssistant: grpcclient.NewAIAssistantClientForTest(mock), Store: storeResolverForTest(8001)}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/x", strings.NewReader(`{"message":"m"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("accountId", int64(42))

			tt.handler(h, c)

			if mock.calls[tt.rpcName] == nil {
				t.Fatalf("expected %s to be called, calls=%v", tt.rpcName, mock.calls)
			}
			if mock.calls[tt.rpcName].GetMessage() != "m" {
				t.Fatalf("request not mapped for %s: %+v", tt.rpcName, mock.calls[tt.rpcName])
			}
		})
	}
}

type mockAIAssistantBuyerStreamClient struct {
	aipb.AIAssistantServiceClient
	calls  map[string]*aipb.AssistantRequest
	stream *fakeSellerChatStream
}

func newMockAIAssistantBuyerStreamClient() *mockAIAssistantBuyerStreamClient {
	return &mockAIAssistantBuyerStreamClient{calls: map[string]*aipb.AssistantRequest{}, stream: &fakeSellerChatStream{}}
}

func (m *mockAIAssistantBuyerStreamClient) BuyerChat(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_BuyerChatClient, error) {
	m.calls["BuyerChat"] = in
	return m.stream, nil
}
func (m *mockAIAssistantBuyerStreamClient) BuyerOrders(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_BuyerOrdersClient, error) {
	m.calls["BuyerOrders"] = in
	return m.stream, nil
}
func (m *mockAIAssistantBuyerStreamClient) BuyerTracking(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_BuyerTrackingClient, error) {
	m.calls["BuyerTracking"] = in
	return m.stream, nil
}
func (m *mockAIAssistantBuyerStreamClient) BuyerSellerMatch(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_BuyerSellerMatchClient, error) {
	m.calls["BuyerSellerMatch"] = in
	return m.stream, nil
}
func (m *mockAIAssistantBuyerStreamClient) BuyerReturnPolicy(ctx context.Context, in *aipb.AssistantRequest, opts ...grpc.CallOption) (aipb.AIAssistantService_BuyerReturnPolicyClient, error) {
	m.calls["BuyerReturnPolicy"] = in
	return m.stream, nil
}

func TestBuyerSSEHandlers_CallCorrectRPC(t *testing.T) {
	tests := []struct {
		name    string
		handler func(h *Holder, c *gin.Context)
		rpcName string
	}{
		{"chat", func(h *Holder, c *gin.Context) { h.ProxyBuyerAssistantSSE(c) }, "BuyerChat"},
		{"orders", func(h *Holder, c *gin.Context) { h.ProxyBuyerOrdersSSE(c) }, "BuyerOrders"},
		{"tracking", func(h *Holder, c *gin.Context) { h.ProxyBuyerTrackingSSE(c) }, "BuyerTracking"},
		{"seller-match", func(h *Holder, c *gin.Context) { h.ProxyBuyerSellerMatchSSE(c) }, "BuyerSellerMatch"},
		{"return-policy", func(h *Holder, c *gin.Context) { h.ProxyBuyerReturnPolicySSE(c) }, "BuyerReturnPolicy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			mock := newMockAIAssistantBuyerStreamClient()
			h := &Holder{AIAssistant: grpcclient.NewAIAssistantClientForTest(mock), BuyerID: buyerResolverForTest(8001)}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/x", strings.NewReader(`{"message":"m"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("accountId", int64(42))

			tt.handler(h, c)

			if mock.calls[tt.rpcName] == nil {
				t.Fatalf("expected %s to be called, calls=%v", tt.rpcName, mock.calls)
			}
			if mock.calls[tt.rpcName].GetMessage() != "m" {
				t.Fatalf("request not mapped for %s: %+v", tt.rpcName, mock.calls[tt.rpcName])
			}
		})
	}
}
