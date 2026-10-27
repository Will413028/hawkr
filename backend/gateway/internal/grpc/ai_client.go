package grpc

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc"
)

// AIClient 持三個 stub，共用同一條 ai-service 連線（見 ai_service_conn.go）。
// #3519 之前這裡是單一 pb.AIServiceClient；拆分後 stub 變三個，連線不變——
// 三個 service 仍然由同一個 ai-service process 提供。
type AIClient struct {
	conn      *grpc.ClientConn
	image     pb.AIImageServiceClient
	admin     pb.AIAdminServiceClient
	recommend pb.RecommendationServiceClient
}

// NewAIClient wraps the shared ai-service connection with the three stubs the
// legacy AIService was split into. The connection itself is dialed once by
// newAIServiceConn and shared with NewAIAssistantClient — see
// ai_service_conn.go. NewAIClient no longer dials or can return an error; fx
// wiring in di.go reflects this.
func NewAIClient(shared *aiServiceConn) *AIClient {
	return &AIClient{
		conn:      shared.conn,
		image:     pb.NewAIImageServiceClient(shared.conn),
		admin:     pb.NewAIAdminServiceClient(shared.conn),
		recommend: pb.NewRecommendationServiceClient(shared.conn),
	}
}

// PersonalizedRecommend calls the AI service for personalized recommendations
func (c *AIClient) PersonalizedRecommend(ctx context.Context, userID int64) (*pb.PersonalizedRecommendResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return c.recommend.PersonalizedRecommend(ctx, &pb.PersonalizedRequest{
		UserId: userID,
		Scene:  "home",
	})
}

// ==================== AI 風格管理 ====================

// ListAiStyles 取得啟用中的風格列表（App 用）。storeID 是呼叫端已解析好的現任
// store_id（account_id 命名債正名），非原始登入 account_id。
//
// #3519：store 身分不再走 request 欄位（ListAiStylesRequest 的 store_id 已從
// proto 移除並 reserved），改由 x-store-id metadata 承載，ai-service 的
// method-policy（ActorSeller）在 handler 執行前從那裡解析。
//
// 🔴 WithStoreID 刻意放在這個方法**內部**，而不是要求呼叫端自己包 ctx：
// 那樣一來「漏帶身分」就是編譯錯誤（少傳 storeID 參數編不過），而不是 runtime
// 的 Unauthenticated。呼叫端自己包的版本形狀相反——刪掉那一行仍然編得過、
// 單元測試也全綠（handler 測試注入 mock stub，根本不看 ctx），而 dev 上賣家
// AI 修圖會 100% 失敗。同 handler.go 對 method-policy 前綴做的事：讓失效模式
// 結構上不可能存在，而不是靠人記得。
//
// ⇒ 呼叫端不必、也不該自己再包一次 WithStoreID。
func (c *AIClient) ListAiStyles(ctx context.Context, storeID identity.StoreID, category string) (*pb.ListAiStylesResponse, error) {
	ctx = WithStoreID(ctx, storeID)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.image.ListAiStyles(ctx, &pb.ListAiStylesRequest{
		Category: category,
	})
}

// ListAiStylesForAdmin 取得全部風格（Admin 用）。category 為空字串代表不篩選。
func (c *AIClient) ListAiStylesForAdmin(ctx context.Context, category string) (*pb.ListAiStylesForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.admin.ListAiStylesForAdmin(ctx, &pb.ListAiStylesForAdminRequest{Category: category})
}

// CreateAiStyle 新增風格
func (c *AIClient) CreateAiStyle(ctx context.Context, req *pb.CreateAiStyleRequest) (*pb.AiStyleResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.admin.CreateAiStyle(ctx, req)
}

// UpdateAiStyle 修改風格
func (c *AIClient) UpdateAiStyle(ctx context.Context, req *pb.UpdateAiStyleRequest) (*pb.AiStyleResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.admin.UpdateAiStyle(ctx, req)
}

// DeleteAiStyle 刪除風格
func (c *AIClient) DeleteAiStyle(ctx context.Context, id string) (*pb.AiStyleResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.admin.DeleteAiStyle(ctx, &pb.DeleteAiStyleRequest{Id: id})
}

// ==================== AI 額度管理 ====================

// GetAiQuota 查詢賣家額度。storeID 是呼叫端已解析好的現任 store_id（account_id
// 命名債正名），非原始登入 account_id。
func (c *AIClient) GetAiQuota(ctx context.Context, storeID string) (*pb.AiQuotaResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.admin.GetAiQuota(ctx, &pb.GetAiQuotaRequest{StoreId: storeID})
}

// UpdateAiQuota 設定賣家額度
func (c *AIClient) UpdateAiQuota(ctx context.Context, req *pb.UpdateAiQuotaRequest) (*pb.AiQuotaResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.admin.UpdateAiQuota(ctx, req)
}

// DeleteAiQuota 刪除賣家自訂額度
func (c *AIClient) DeleteAiQuota(ctx context.Context, storeID string) (*pb.AiQuotaResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.admin.DeleteAiQuota(ctx, &pb.DeleteAiQuotaRequest{StoreId: storeID})
}

// ==================== AI 圖片轉換 ====================

// TransformImage 透過 gRPC 呼叫 ai-service 進行圖片轉換。storeID 是呼叫端已解析
// 好的現任 store_id（account_id 命名債正名），非原始登入 account_id。
//
// #3519：store 身分只走 x-store-id metadata，且與 ListAiStyles 一樣在方法內部
// 注入——理由（為什麼不交給呼叫端包 ctx）見該方法註解。
func (c *AIClient) TransformImage(ctx context.Context, storeID identity.StoreID, imageData []byte, mimeType, prompt string) (*pb.TransformImageResponse, error) {
	ctx = WithStoreID(ctx, storeID)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second) // Gemini 回應可能較慢
	defer cancel()

	return c.image.TransformImage(ctx, &pb.TransformImageRequest{
		ImageData: imageData,
		MimeType:  mimeType,
		Prompt:    prompt,
	})
}
