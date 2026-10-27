package grpc

import (
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// aiServiceConn is the single *grpc.ClientConn to ai-service, shared by
// AIClient (AIImageService 2 + AIAdminService 7 + RecommendationService 1
// RPCs — the three services #3519 split the legacy AIService into) and
// AIAssistantClient (AIAssistantService, 15 RPCs, Phase B) so all the service
// stubs multiplex over one HTTP/2 connection and this connection has exactly
// one owner responsible for Close() (registered in di.go's clientClosers, not
// on either client).
type aiServiceConn struct {
	conn *grpc.ClientConn
}

// newAIServiceConn dials ai-service once. Both NewAIClient and
// NewAIAssistantClient depend on this via fx instead of dialing themselves.
func newAIServiceConn() (*aiServiceConn, error) {
	host := getEnvOrDefault("AI_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("AI_SERVICE_GRPC_PORT", "9939")
	target := fmt.Sprintf("%s:%s", host, port)

	opts := append(DefaultDialOpts(), grpc.WithDefaultCallOptions(
		grpc.MaxCallSendMsgSize(20*1024*1024), // 20MB，AIImageService 傳送原始圖片沿用既有上限
		grpc.MaxCallRecvMsgSize(20*1024*1024), // 20MB，接收 base64 轉換結果
	))
	conn, err := grpcclient.Dial(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to AI gRPC service at %s (shared by AIClient + AIAssistantClient)", target)
	return &aiServiceConn{conn: conn}, nil
}

func (c *aiServiceConn) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
