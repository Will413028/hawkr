package grpc

import (
	"context"
	"fmt"
	"time"

	gatewayconfig "github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/go-pkg/serviceauth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// IMClient is the gRPC client for IM service
type IMClient struct {
	conn   *grpc.ClientConn
	client pb.IMServiceClient
}

// NewIMClient creates a new IM gRPC client
func NewIMClient(cfg *gatewayconfig.Config) (*IMClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("gateway config is required")
	}
	host := getEnvOrDefault("IM_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("IM_SERVICE_GRPC_PORT", "9933")
	target := fmt.Sprintf("%s:%s", host, port)

	signer, signerErr := newIMChatStreamSigner(cfg.IMChatStreamJWTSecret, cfg.IMChatStreamJWTSecretPrev)
	if signerErr != nil {
		return nil, fmt.Errorf("failed to configure IM chat stream authentication: %w", signerErr)
	}
	dialOpts := IMDialOpts(signer)

	conn, err := grpcclient.Dial(target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IM service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to IM gRPC service at %s", target)

	return &IMClient{
		conn:   conn,
		client: pb.NewIMServiceClient(conn),
	}, nil
}

func newIMChatStreamSigner(currentKey, previousKey string) (*serviceauth.Signer, error) {
	return serviceauth.NewSigner(serviceauth.Config{
		CurrentKey:  currentKey,
		PreviousKey: previousKey,
		Issuer:      "gateway",
		Audience:    "im-service",
		Purpose:     "im-chat-stream",
	})
}

// Close closes the gRPC connection
func (c *IMClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendMessage sends a chat message
func (c *IMClient) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SendMessage(ctx, req)
}

// ChatStream opens the long-lived Gateway-to-IM receive-only event stream.
// Unlike unary calls it intentionally has no synthetic timeout; cancellation
// belongs to the public WebSocket request context and transport keepalive.
func (c *IMClient) ChatStream(ctx context.Context, opts ...grpc.CallOption) (pb.IMService_ChatStreamClient, error) {
	return c.client.ChatStream(ctx, opts...)
}

// GetChatHistory gets chat history with cursor-based pagination
func (c *IMClient) GetChatHistory(ctx context.Context, req *pb.GetChatHistoryRequest) (*pb.GetChatHistoryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetChatHistory(ctx, req)
}

// GetConversations gets conversation list
func (c *IMClient) GetConversations(ctx context.Context, req *pb.GetConversationsRequest) (*pb.GetConversationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetConversations(ctx, req)
}

// GetUnreadCount gets unread message count
func (c *IMClient) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetUnreadCount(ctx, req)
}

// MarkMessagesRead marks specific messages as read
func (c *IMClient) MarkMessagesRead(ctx context.Context, req *pb.MarkMessagesReadRequest) (*pb.MarkMessagesReadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.MarkMessagesRead(ctx, req)
}

// MarkConversationRead marks all messages in a conversation as read
func (c *IMClient) MarkConversationRead(ctx context.Context, req *pb.MarkConversationReadRequest) (*pb.MarkConversationReadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.MarkConversationRead(ctx, req)
}

// MarkAllRead marks all messages as read
func (c *IMClient) MarkAllRead(ctx context.Context, req *pb.MarkAllReadRequest) (*pb.MarkAllReadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.MarkAllRead(ctx, req)
}
