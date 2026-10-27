package grpc

import (
	"context"
	"testing"
	"time"

	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestNewAIAssistantClient_UsesSharedConn(t *testing.T) {
	cc, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { cc.Close() })

	shared := &aiServiceConn{conn: cc}
	client := NewAIAssistantClient(shared)

	if client.conn != cc {
		t.Fatalf("NewAIAssistantClient did not reuse the shared conn: got %p, want %p", client.conn, cc)
	}
	if client.client == nil {
		t.Fatal("NewAIAssistantClient did not construct the AIAssistantServiceClient stub")
	}
}

// deadlineCapturingAssistantClient records the deadline each unary RPC is
// called with. The embedded (nil) interface supplies the 12 stream methods
// this test never calls, so only the 3 unary ones need real bodies.
type deadlineCapturingAssistantClient struct {
	aipb.AIAssistantServiceClient
	deadline time.Time
	ok       bool
}

func (f *deadlineCapturingAssistantClient) SellerProductAutofill(ctx context.Context, _ *aipb.SellerProductAutofillRequest, _ ...grpc.CallOption) (*aipb.SellerProductAutofillResponse, error) {
	f.deadline, f.ok = ctx.Deadline()
	return &aipb.SellerProductAutofillResponse{}, nil
}

func (f *deadlineCapturingAssistantClient) SellerLatestSession(ctx context.Context, _ *aipb.LatestSessionRequest, _ ...grpc.CallOption) (*aipb.LatestSessionResponse, error) {
	f.deadline, f.ok = ctx.Deadline()
	return &aipb.LatestSessionResponse{}, nil
}

func (f *deadlineCapturingAssistantClient) BuyerLatestSession(ctx context.Context, _ *aipb.LatestSessionRequest, _ ...grpc.CallOption) (*aipb.LatestSessionResponse, error) {
	f.deadline, f.ok = ctx.Deadline()
	return &aipb.LatestSessionResponse{}, nil
}

// TestAIAssistantClient_UnaryRPCsCarryDeadline pins CLAUDE.md §12 (no
// deadline-less cross-service call) for the 3 unary AIAssistantService RPCs:
// the caller's context is deliberately deadline-free here, so any deadline
// observed downstream can only have come from the client wrapper itself.
func TestAIAssistantClient_UnaryRPCsCarryDeadline(t *testing.T) {
	cases := []struct {
		name string
		want time.Duration
		call func(c *AIAssistantClient, ctx context.Context) error
	}{
		{"SellerProductAutofill", autofillTimeout, func(c *AIAssistantClient, ctx context.Context) error {
			_, err := c.SellerProductAutofill(ctx, &aipb.SellerProductAutofillRequest{})
			return err
		}},
		{"SellerLatestSession", latestSessionTimeout, func(c *AIAssistantClient, ctx context.Context) error {
			_, err := c.SellerLatestSession(ctx, &aipb.LatestSessionRequest{})
			return err
		}},
		{"BuyerLatestSession", latestSessionTimeout, func(c *AIAssistantClient, ctx context.Context) error {
			_, err := c.BuyerLatestSession(ctx, &aipb.LatestSessionRequest{})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &deadlineCapturingAssistantClient{}
			client := &AIAssistantClient{client: fake}

			ctx := context.Background()
			if _, hasDeadline := ctx.Deadline(); hasDeadline {
				t.Fatal("test precondition: caller context must have no deadline")
			}
			if err := tc.call(client, ctx); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}

			if !fake.ok {
				t.Fatalf("%s reached ai-service with NO deadline (CLAUDE.md §12)", tc.name)
			}
			if got := time.Until(fake.deadline); got <= tc.want-time.Second || got > tc.want {
				t.Fatalf("%s deadline = %v ahead, want ~%v", tc.name, got, tc.want)
			}
		})
	}
}

// TestAssistantStream_SatisfiedByGeneratedStreamType is a compile-time check: ensures that protoc-gen-go-grpc
// generates named interfaces for the 12 server-streaming RPCs that are structurally
// satisfied by AssistantStream. Use SellerChat as representative; the other 11 are identical.
func TestAssistantStream_SatisfiedByGeneratedStreamType(t *testing.T) {
	var _ AssistantStream = (aipb.AIAssistantService_SellerChatClient)(nil)
}
