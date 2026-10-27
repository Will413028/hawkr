package grpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// CrmClient is the gRPC client for crm service
type CrmClient struct {
	conn   *grpc.ClientConn
	client pb.CrmServiceClient
}

// NewCrmClient creates a new crm gRPC client
func NewCrmClient() (*CrmClient, error) {
	host := getEnvOrDefault("CRM_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("CRM_SERVICE_GRPC_PORT", "10030")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to crm service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to crm gRPC service at %s", target)

	return &CrmClient{
		conn:   conn,
		client: pb.NewCrmServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *CrmClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== CRM: Customers ====================

func (c *CrmClient) GetCustomerList(ctx context.Context, storeID identity.StoreID, page, pageSize int32, status, keyword, tagID, sortBy, sortOrder string) (*pb.CustomerListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetCustomerList(ctx, &pb.GetCustomerListRequest{
		StoreId:      strconv.FormatInt(int64(storeID), 10),
		Page:         page,
		PageSize:     pageSize,
		CustomerType: status,
		Keyword:      keyword,
		TagId:        tagID,
		SortBy:       sortBy,
		SortOrder:    sortOrder,
	})
}

func (c *CrmClient) GetCustomerStats(ctx context.Context, storeID identity.StoreID, customerBuyerID identity.BuyerID) (*pb.CustomerStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetCustomerStats(ctx, &pb.GetCustomerStatsRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
		BuyerId: strconv.FormatInt(int64(customerBuyerID), 10),
	})
}

func (c *CrmClient) UpdateCustomerStatus(ctx context.Context, storeID identity.StoreID, customerBuyerID identity.BuyerID, status string) (*pb.CrmResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.UpdateCustomerStatus(ctx, &pb.UpdateCustomerStatusRequest{
		StoreId:      strconv.FormatInt(int64(storeID), 10),
		BuyerId:      strconv.FormatInt(int64(customerBuyerID), 10),
		CustomerType: status,
	})
}

// ==================== CRM: Tags ====================
func (c *CrmClient) CreateTag(ctx context.Context, storeID identity.StoreID, name string) (*pb.TagResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.CreateTag(ctx, &pb.CreateTagRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
		Name:    name,
	})
}

func (c *CrmClient) ListTags(ctx context.Context, storeID identity.StoreID) (*pb.TagListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ListTags(ctx, &pb.ListTagsRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
	})
}

func (c *CrmClient) UpdateTag(ctx context.Context, storeID identity.StoreID, tagID, name string) (*pb.CrmResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.UpdateTag(ctx, &pb.UpdateTagRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
		TagId:   tagID,
		Name:    name,
	})
}

func (c *CrmClient) DeleteTag(ctx context.Context, storeID identity.StoreID, tagID string) (*pb.CrmResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.DeleteTag(ctx, &pb.DeleteTagRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
		TagId:   tagID,
	})
}

func (c *CrmClient) AddCustomerTags(ctx context.Context, storeID identity.StoreID, customerBuyerID identity.BuyerID, tagNames []string) (*pb.CrmResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.AddCustomerTags(ctx, &pb.AddCustomerTagsRequest{
		StoreId:  strconv.FormatInt(int64(storeID), 10),
		BuyerId:  strconv.FormatInt(int64(customerBuyerID), 10),
		TagNames: tagNames,
	})
}

func (c *CrmClient) RemoveCustomerTag(ctx context.Context, storeID identity.StoreID, customerBuyerID identity.BuyerID, tagID int64) (*pb.CrmResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.RemoveCustomerTag(ctx, &pb.RemoveCustomerTagRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
		BuyerId: strconv.FormatInt(int64(customerBuyerID), 10),
		TagId:   strconv.FormatInt(tagID, 10),
	})
}

func (c *CrmClient) GetCustomerTags(ctx context.Context, storeID identity.StoreID, customerBuyerID identity.BuyerID) (*pb.TagListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetCustomerTags(ctx, &pb.GetCustomerTagsRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
		BuyerId: strconv.FormatInt(int64(customerBuyerID), 10),
	})
}

// ==================== CRM: Search Customers by Tags ====================

func (c *CrmClient) SearchCustomersByTags(ctx context.Context, storeID identity.StoreID, tagIDs []string) (*pb.SearchCustomersByTagsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.SearchCustomersByTags(ctx, &pb.SearchCustomersByTagsRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
		TagIds:  tagIDs,
	})
}
