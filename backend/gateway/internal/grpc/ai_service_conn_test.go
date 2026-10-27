package grpc

import (
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestNewAIClient_UsesSharedConn 確認 NewAIClient 不自己 dial，而是直接用傳入的
// *aiServiceConn——這是 §3.1 設計決策（避免 AIClient/AIAssistantClient 各自開一條連線）
// 的可驗證行為：兩者的 AIClient.conn 必須是同一個 *grpc.ClientConn 指標。
func TestNewAIClient_UsesSharedConn(t *testing.T) {
	cc, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { cc.Close() })

	shared := &aiServiceConn{conn: cc}
	client := NewAIClient(shared)

	if client.conn != cc {
		t.Fatalf("NewAIClient did not reuse the shared conn: got %p, want %p", client.conn, cc)
	}
	// #3519：三個 stub 都必須建出來。少建一個不會在這個測試以外的任何地方
	// 報錯——對應的那批 RPC 會在第一次呼叫時 nil pointer panic。
	if client.image == nil {
		t.Fatal("NewAIClient did not construct the AIImageServiceClient stub")
	}
	if client.admin == nil {
		t.Fatal("NewAIClient did not construct the AIAdminServiceClient stub")
	}
	if client.recommend == nil {
		t.Fatal("NewAIClient did not construct the RecommendationServiceClient stub")
	}
}
