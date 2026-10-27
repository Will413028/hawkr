// Package category 是 category service（由 product-service 實作）的 gRPC
// client，供 pricebook-service 驗證 admin categoryId→cropCode/groupCode
// mapping 用的 categoryId 是否存在且 ACTIVE（issue #2786 Task 8）與計算分類樹
// 差集（Task 9）。
//
// 本檔仿 services/ai-service/internal/client/category/adapter.go 撰寫，
// 兩邊各自獨立維護（同一模式在不同服務各自持有一份 client，與 moa client 的
// 既有慣例一致）。
package category

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
)

// Client wraps the gRPC CategoryServiceClient.
type Client struct {
	client categorypb.CategoryServiceClient
}

// NewCategoryClient 建立連線至 product-service gRPC 的 CategoryClient。
func NewCategoryClient(cfg *config.Config) (*Client, error) {
	target := fmt.Sprintf("%s:%s", cfg.ProductServiceHost, cfg.ProductServicePort)
	conn, err := grpcclient.Dial(target,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to category service at %s: %w", target, err)
	}
	return &Client{client: categorypb.NewCategoryServiceClient(conn)}, nil
}

// GetCategoryTree 取得分類樹。
//
//	status: "active" 或空字串都僅回 ACTIVE；admin 用 includeDeleted=true 才回含 INACTIVE 軟刪除
//
// 空 status **不是**「不篩選」——product-service 的
// CategoryGrpcService.getCategoryTree 把 status 空字串／"active"／"9004" 一律
// 當成 activeOnly，只有 include_deleted=true 才會回停用節點。簽章與
// gateway/internal/grpc/category_client.go 的同名方法一致。
//
// admin mapping 驗證 categoryId 是否存在且 ACTIVE（Task 8）與待處理清單差集運算
// （Task 9）都走這支——category service 沒有「查單一 categoryId」的 RPC，呼叫端
// 對回傳的樹做一次查找即可。
func (c *Client) GetCategoryTree(ctx context.Context, status string, includeDeleted bool) (*categorypb.GetCategoryTreeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout10s)
	defer cancel()
	return c.client.GetCategoryTree(ctx, &categorypb.GetCategoryTreeRequest{
		Status:         status,
		IncludeDeleted: includeDeleted,
	})
}
