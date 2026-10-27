package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product_import"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// ProductImportClient 是 product-import-service 的 gRPC 用戶端
type ProductImportClient struct {
	conn   *grpc.ClientConn
	client pb.ProductImportServiceClient
}

// NewProductImportClient 建立新的 product-import gRPC 用戶端
func NewProductImportClient() (*ProductImportClient, error) {
	host := getEnvOrDefault("PRODUCT_IMPORT_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("PRODUCT_IMPORT_SERVICE_GRPC_PORT", "9961")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to product-import service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to product-import gRPC service at %s", target)

	return &ProductImportClient{
		conn:   conn,
		client: pb.NewProductImportServiceClient(conn),
	}, nil
}

// Close 關閉連線
func (c *ProductImportClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== Job 級 ====================

// CreateImportJob 建立匯入任務（含 L1+L2 同步驗證）
func (c *ProductImportClient) CreateImportJob(ctx context.Context, req *pb.CreateImportJobRequest) (*pb.CreateImportJobResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.CreateImportJob(ctx, req)
}

// ReuploadImportJobCsv 重傳 CSV（CSV_INVALID 狀態才可）
func (c *ProductImportClient) ReuploadImportJobCsv(ctx context.Context, req *pb.ReuploadImportJobCsvRequest) (*pb.ReuploadImportJobCsvResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ReuploadImportJobCsv(ctx, req)
}

// ListImportJobs 查詢任務列表
func (c *ProductImportClient) ListImportJobs(ctx context.Context, req *pb.ListImportJobsRequest) (*pb.ListImportJobsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListImportJobs(ctx, req)
}

// GetImportJob 取得任務詳情
func (c *ProductImportClient) GetImportJob(ctx context.Context, req *pb.GetImportJobRequest) (*pb.GetImportJobResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetImportJob(ctx, req)
}

// GetImportJobPreviewListing 取得商城模擬列表（scope by job）
func (c *ProductImportClient) GetImportJobPreviewListing(ctx context.Context, req *pb.GetImportJobPreviewListingRequest) (*pb.GetImportJobPreviewListingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.GetImportJobPreviewListing(ctx, req)
}

// CancelImportJob 取消任務
func (c *ProductImportClient) CancelImportJob(ctx context.Context, req *pb.CancelImportJobRequest) (*pb.CancelImportJobResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.CancelImportJob(ctx, req)
}

// CommitImportJob 確認提交（排入 commit queue）
func (c *ProductImportClient) CommitImportJob(ctx context.Context, req *pb.CommitImportJobRequest) (*pb.CommitImportJobResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.CommitImportJob(ctx, req)
}

// ==================== Item 級 ====================

// ListImportItems 查詢任務內 items
func (c *ProductImportClient) ListImportItems(ctx context.Context, req *pb.ListImportItemsRequest) (*pb.ListImportItemsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListImportItems(ctx, req)
}

// GetImportItem 取得單一 item 詳情
func (c *ProductImportClient) GetImportItem(ctx context.Context, req *pb.GetImportItemRequest) (*pb.GetImportItemResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetImportItem(ctx, req)
}

// GetImportItemPreview 取得單一 item 商城模擬詳情
func (c *ProductImportClient) GetImportItemPreview(ctx context.Context, req *pb.GetImportItemPreviewRequest) (*pb.GetImportItemPreviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.GetImportItemPreview(ctx, req)
}

// UpdateImportItem 編輯單一 item（含樂觀鎖 + RPC 重驗證）
func (c *ProductImportClient) UpdateImportItem(ctx context.Context, req *pb.UpdateImportItemRequest) (*pb.UpdateImportItemResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return c.client.UpdateImportItem(ctx, req)
}

// RemoveImportItems 批次移除（軟刪）
func (c *ProductImportClient) RemoveImportItems(ctx context.Context, req *pb.RemoveImportItemsRequest) (*pb.RemoveImportItemsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.RemoveImportItems(ctx, req)
}

// RestoreImportItems 批次還原
func (c *ProductImportClient) RestoreImportItems(ctx context.Context, req *pb.RestoreImportItemsRequest) (*pb.RestoreImportItemsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.RestoreImportItems(ctx, req)
}

// RetryImportItemAssets 重試 FAILED 圖片
func (c *ProductImportClient) RetryImportItemAssets(ctx context.Context, req *pb.RetryImportItemAssetsRequest) (*pb.RetryImportItemAssetsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.RetryImportItemAssets(ctx, req)
}

// ==================== 批次操作 ====================

// BatchUpdateItemsShipping 批次覆寫運送（覆寫所選 items 所有 SKU）
func (c *ProductImportClient) BatchUpdateItemsShipping(ctx context.Context, req *pb.BatchUpdateItemsShippingRequest) (*pb.BatchUpdateItemsShippingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.BatchUpdateItemsShipping(ctx, req)
}

// BatchUpdateItemsTargetStatus 批次套用上架狀態
func (c *ProductImportClient) BatchUpdateItemsTargetStatus(ctx context.Context, req *pb.BatchUpdateItemsTargetStatusRequest) (*pb.BatchUpdateItemsTargetStatusResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.BatchUpdateItemsTargetStatus(ctx, req)
}

// BatchUpdateItemsCategory 批次套用商品類別（需 confirmation_token）
func (c *ProductImportClient) BatchUpdateItemsCategory(ctx context.Context, req *pb.BatchUpdateItemsCategoryRequest) (*pb.BatchUpdateItemsCategoryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.BatchUpdateItemsCategory(ctx, req)
}

// GetCategoryConfirmationToken 發給前端「分類二次確認 token」
func (c *ProductImportClient) GetCategoryConfirmationToken(ctx context.Context, req *pb.GetCategoryConfirmationTokenRequest) (*pb.GetCategoryConfirmationTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetCategoryConfirmationToken(ctx, req)
}
