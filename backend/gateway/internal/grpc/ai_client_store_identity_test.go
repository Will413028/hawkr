package grpc

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc"
)

// storeIDCapturingImageClient 記錄每支 RPC 拿到的 context，讓測試檢查
// AIClient 是否真的把 store 身分放進去。內嵌（nil）介面供應本測試不呼叫的方法。
type storeIDCapturingImageClient struct {
	aipb.AIImageServiceClient
	gotCtx context.Context
}

func (f *storeIDCapturingImageClient) ListAiStyles(ctx context.Context, _ *aipb.ListAiStylesRequest, _ ...grpc.CallOption) (*aipb.ListAiStylesResponse, error) {
	f.gotCtx = ctx
	return &aipb.ListAiStylesResponse{}, nil
}

func (f *storeIDCapturingImageClient) TransformImage(ctx context.Context, _ *aipb.TransformImageRequest, _ ...grpc.CallOption) (*aipb.TransformImageResponse, error) {
	f.gotCtx = ctx
	return &aipb.TransformImageResponse{}, nil
}

// TestAIImageClient_InjectsStoreIdentity 釘住 #3519 的授權來源：AIImageService
// 那兩支在 ai-service 端登記為 ActorSeller，身分只從 x-store-id metadata 解析
// （request message 的 store_id 欄位已從 proto 移除並 reserved）。metadata 由
// AuthMetadataInterceptor 從 CtxKeyStoreID 這個 context value 產生（該轉換由
// metadata_interceptor_test.go 的 TestAuthMetadataInterceptor_ForwardsBuyerAndStoreID
// 覆蓋），所以這裡驗的是鏈條的上半段：AIClient 有沒有把值放進 ctx。
//
// 🔴 為什麼需要這支：注入點在 #3519 的第一版是寫在 handler 裡的
// `ctx := WithStoreID(...)`，刪掉那一行**編譯過、handler 單元測試全綠**
// （那些測試注入 mock stub，根本不看 ctx），而 dev 上賣家 AI 修圖會 100%
// Unauthenticated。注入點移進 client 之後漏傳 storeID 已是編譯錯誤；這支測試
// 守的是另一半——注入那一行本身被刪掉或寫錯。
func TestAIImageClient_InjectsStoreIdentity(t *testing.T) {
	const wantStoreID = identity.StoreID(2625627849)

	cases := []struct {
		name string
		call func(*AIClient, context.Context) error
	}{
		{
			name: "ListAiStyles",
			call: func(c *AIClient, ctx context.Context) error {
				_, err := c.ListAiStyles(ctx, wantStoreID, "product")
				return err
			},
		},
		{
			name: "TransformImage",
			call: func(c *AIClient, ctx context.Context) error {
				_, err := c.TransformImage(ctx, wantStoreID, []byte("img"), "image/jpeg", "prompt")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &storeIDCapturingImageClient{}
			client := NewAIImageClientForTest(stub)

			// 刻意傳一個完全乾淨的 context：觀察到的 store 身分只可能來自
			// client 方法自己，不可能是呼叫端先包好的。
			if err := tc.call(client, context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stub.gotCtx == nil {
				t.Fatal("stub was never called")
			}

			got, ok := stub.gotCtx.Value(CtxKeyStoreID).(identity.StoreID)
			if !ok {
				t.Fatalf("outgoing context carries no %v; AuthMetadataInterceptor would emit no x-store-id and ai-service would reject with Unauthenticated", CtxKeyStoreID)
			}
			if got != wantStoreID {
				t.Fatalf("store id = %d, want %d", got, wantStoreID)
			}
		})
	}
}
