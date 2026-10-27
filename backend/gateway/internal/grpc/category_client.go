package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"google.golang.org/grpc"
)

// CategoryClient is the gRPC client for category service.
//
// 2026-04-25 政策大改造後：
//   - 移除 BatchUpdateCategoryStatus（無審核機制）
//   - 新增 4 個 admin RPC：CreateCategoryForAdmin / UpdateCategoryForAdmin /
//     BatchSoftDeleteCategoryForAdmin / RestoreCategoryForAdmin
type CategoryClient struct {
	conn   *grpc.ClientConn
	client pb.CategoryServiceClient
}

// NewCategoryClient creates a new category gRPC client
func NewCategoryClient() (*CategoryClient, error) {
	host := getEnvOrDefault("PRODUCT_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("PRODUCT_SERVICE_GRPC_PORT", "9913")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to category service: %w", err)
	}

	return &CategoryClient{
		conn:   conn,
		client: pb.NewCategoryServiceClient(conn),
	}, nil
}

// GetCategoryTree gets categories as a tree structure.
//
//	status: "active" 或空字串都僅回 ACTIVE；admin 用 includeDeleted=true 才回含 INACTIVE 軟刪除
func (c *CategoryClient) GetCategoryTree(ctx context.Context, status string, includeDeleted bool) (*pb.GetCategoryTreeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetCategoryTreeRequest{
		Status:         status,
		IncludeDeleted: includeDeleted,
	}

	return c.client.GetCategoryTree(ctx, req)
}

// GetChildCategories gets direct children of a category (僅 ACTIVE)
func (c *CategoryClient) GetChildCategories(ctx context.Context, parentID string) (*pb.GetChildCategoriesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetChildCategoriesRequest{
		ParentId: parentID,
	}

	return c.client.GetChildCategories(ctx, req)
}

// CreateCategory creates a new category (賣家用，自動套用同名同父)
func (c *CategoryClient) CreateCategory(ctx context.Context, parentID string, name string, createdBy int64) (*pb.CreateCategoryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.CreateCategoryRequest{
		ParentId:  parentID,
		Name:      name,
		CreatedBy: createdBy,
	}

	return c.client.CreateCategory(ctx, req)
}

// CreateCategoryForAdmin creates a new category as admin (任何層級，自動套用)
func (c *CategoryClient) CreateCategoryForAdmin(ctx context.Context, parentID string, name string, createdBy int64) (*pb.CreateCategoryForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.CreateCategoryForAdminRequest{
		ParentId:  parentID,
		Name:      name,
		CreatedBy: createdBy,
	}

	return c.client.CreateCategoryForAdmin(ctx, req)
}

// UpdateCategoryForAdmin updates the name of a category (admin 僅可改名)
func (c *CategoryClient) UpdateCategoryForAdmin(ctx context.Context, categoryID string, name string, updatedBy int64) (*pb.UpdateCategoryForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.UpdateCategoryForAdminRequest{
		CategoryId: categoryID,
		Name:       name,
		UpdatedBy:  updatedBy,
	}

	return c.client.UpdateCategoryForAdmin(ctx, req)
}

// BatchSoftDeleteCategoryForAdmin batch soft-deletes categories
func (c *CategoryClient) BatchSoftDeleteCategoryForAdmin(ctx context.Context, categoryIDs []string, updatedBy int64) (*pb.BatchSoftDeleteCategoryForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.BatchSoftDeleteCategoryForAdminRequest{
		CategoryIds: categoryIDs,
		UpdatedBy:   updatedBy,
	}

	return c.client.BatchSoftDeleteCategoryForAdmin(ctx, req)
}

// RestoreCategoryForAdmin restores a soft-deleted category
func (c *CategoryClient) RestoreCategoryForAdmin(ctx context.Context, categoryID string, updatedBy int64) (*pb.RestoreCategoryForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.RestoreCategoryForAdminRequest{
		CategoryId: categoryID,
		UpdatedBy:  updatedBy,
	}

	return c.client.RestoreCategoryForAdmin(ctx, req)
}

// Close closes the gRPC connection
func (c *CategoryClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
