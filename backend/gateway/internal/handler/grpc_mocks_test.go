package handler

import (
	"context"

	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"google.golang.org/grpc"
)

// ==================== Mock ProductServiceClient ====================

type mockProductServiceClient struct {
	GetProductListFn               func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error)
	GetPublicProductFn             func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error)
	CreateProductFn                func(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error)
	UpdateProductFn                func(ctx context.Context, in *productpb.UpdateProductRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error)
	GetProductFn                   func(ctx context.Context, in *productpb.GetProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error)
	DeleteProductFn                func(ctx context.Context, in *productpb.DeleteProductRequest, opts ...grpc.CallOption) (*productpb.DeleteProductResponse, error)
	GetPublicSellerCitiesSummaryFn func(ctx context.Context, in *productpb.GetPublicSellerCitiesSummaryRequest, opts ...grpc.CallOption) (*productpb.GetPublicSellerCitiesSummaryResponse, error)

	PurgeProductCacheFn   func(ctx context.Context, in *productpb.PurgeProductCacheRequest, opts ...grpc.CallOption) (*productpb.PurgeProductCacheResponse, error)
	VerifyCertificationFn func(ctx context.Context, in *productpb.VerifyCertificationRequest, opts ...grpc.CallOption) (*productpb.VerifyCertificationResponse, error)
}

func (m *mockProductServiceClient) GetProductList(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
	if m.GetProductListFn != nil {
		return m.GetProductListFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockProductServiceClient) CreateProduct(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
	if m.CreateProductFn != nil {
		return m.CreateProductFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockProductServiceClient) UpdateProduct(ctx context.Context, in *productpb.UpdateProductRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
	if m.UpdateProductFn != nil {
		return m.UpdateProductFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockProductServiceClient) GetProduct(ctx context.Context, in *productpb.GetProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
	if m.GetProductFn != nil {
		return m.GetProductFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockProductServiceClient) GetPublicProduct(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
	if m.GetPublicProductFn != nil {
		return m.GetPublicProductFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockProductServiceClient) GetProductForAdmin(ctx context.Context, in *productpb.GetProductForAdminRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) UpdateProductStatus(ctx context.Context, in *productpb.UpdateProductStatusRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) PurgeProductCache(ctx context.Context, in *productpb.PurgeProductCacheRequest, opts ...grpc.CallOption) (*productpb.PurgeProductCacheResponse, error) {
	if m.PurgeProductCacheFn != nil {
		return m.PurgeProductCacheFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockProductServiceClient) DeductStock(ctx context.Context, in *productpb.DeductStockRequest, opts ...grpc.CallOption) (*productpb.StockOperationResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) RestoreStock(ctx context.Context, in *productpb.RestoreStockRequest, opts ...grpc.CallOption) (*productpb.StockOperationResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) GetStock(ctx context.Context, in *productpb.GetStockRequest, opts ...grpc.CallOption) (*productpb.GetStockResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) GetProductSimple(ctx context.Context, in *productpb.GetProductSimpleRequest, opts ...grpc.CallOption) (*productpb.GetProductSimpleResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) BatchGetSkuCartInfo(ctx context.Context, in *productpb.BatchGetSkuCartInfoRequest, opts ...grpc.CallOption) (*productpb.BatchGetSkuCartInfoResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) BatchGetProductForCheckout(ctx context.Context, in *productpb.BatchGetProductForCheckoutRequest, opts ...grpc.CallOption) (*productpb.BatchGetProductForCheckoutResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) GetProductsByIDs(ctx context.Context, in *productpb.GetProductsByIDsRequest, opts ...grpc.CallOption) (*productpb.GetProductsByIDsResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) GetRelatedProducts(ctx context.Context, in *productpb.GetRelatedProductsRequest, opts ...grpc.CallOption) (*productpb.GetRelatedProductsResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) DeleteProduct(ctx context.Context, in *productpb.DeleteProductRequest, opts ...grpc.CallOption) (*productpb.DeleteProductResponse, error) {
	if m.DeleteProductFn != nil {
		return m.DeleteProductFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockProductServiceClient) VerifyCertification(ctx context.Context, in *productpb.VerifyCertificationRequest, opts ...grpc.CallOption) (*productpb.VerifyCertificationResponse, error) {
	if m.VerifyCertificationFn != nil {
		return m.VerifyCertificationFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockProductServiceClient) UpdateSellerProductStatus(ctx context.Context, in *productpb.UpdateSellerProductStatusRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) BatchUpdateSellerProductStatus(ctx context.Context, in *productpb.BatchUpdateSellerProductStatusRequest, opts ...grpc.CallOption) (*productpb.BatchUpdateSellerProductStatusResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) BatchGetSellerProductsForDelegation(ctx context.Context, in *productpb.BatchGetSellerProductsForDelegationRequest, opts ...grpc.CallOption) (*productpb.BatchGetSellerProductsForDelegationResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) ListProductsByCategoryForAdmin(ctx context.Context, in *productpb.ListProductsByCategoryForAdminRequest, opts ...grpc.CallOption) (*productpb.ListProductsByCategoryForAdminResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) BatchChangeProductCategoryForAdmin(ctx context.Context, in *productpb.BatchChangeProductCategoryForAdminRequest, opts ...grpc.CallOption) (*productpb.BatchChangeProductCategoryForAdminResponse, error) {
	return nil, nil
}
func (m *mockProductServiceClient) SearchHashtags(ctx context.Context, in *productpb.SearchHashtagsRequest, opts ...grpc.CallOption) (*productpb.SearchHashtagsResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) GetPopularHashtags(ctx context.Context, in *productpb.GetPopularHashtagsRequest, opts ...grpc.CallOption) (*productpb.GetPopularHashtagsResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) GetPublicOriginsSummary(ctx context.Context, in *productpb.GetPublicOriginsSummaryRequest, opts ...grpc.CallOption) (*productpb.GetPublicOriginsSummaryResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) GetPublicSellerCitiesSummary(ctx context.Context, in *productpb.GetPublicSellerCitiesSummaryRequest, opts ...grpc.CallOption) (*productpb.GetPublicSellerCitiesSummaryResponse, error) {
	if m.GetPublicSellerCitiesSummaryFn != nil {
		return m.GetPublicSellerCitiesSummaryFn(ctx, in, opts...)
	}
	return nil, nil
}

// ListStoreIDsWithPublicProductsInternal is the seller module's caller-side RPC
// (02 §5.13c) — the gateway never issues it; the method exists here only to keep
// this mock satisfying productpb.ProductServiceClient.
func (m *mockProductServiceClient) ListStoreIDsWithPublicProductsInternal(ctx context.Context, in *productpb.ListStoreIDsWithPublicProductsInternalRequest, opts ...grpc.CallOption) (*productpb.ListStoreIDsWithPublicProductsInternalResponse, error) {
	return nil, nil
}

// ---- 商品認證申請與審核（19 號設計書 §10.1）----

func (m *mockProductServiceClient) SubmitProductCertification(ctx context.Context, in *productpb.SubmitProductCertificationRequest, opts ...grpc.CallOption) (*productpb.ProductCertificationApplicationResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) ListProductCertificationApplications(ctx context.Context, in *productpb.ListProductCertificationApplicationsRequest, opts ...grpc.CallOption) (*productpb.ListProductCertificationApplicationsResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) WithdrawProductCertification(ctx context.Context, in *productpb.WithdrawProductCertificationRequest, opts ...grpc.CallOption) (*productpb.ProductCertificationApplicationResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) ListProductCertificationApplicationsForAdmin(ctx context.Context, in *productpb.ListProductCertificationApplicationsForAdminRequest, opts ...grpc.CallOption) (*productpb.ListProductCertificationApplicationsForAdminResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) GetProductCertificationApplicationForAdmin(ctx context.Context, in *productpb.GetProductCertificationApplicationForAdminRequest, opts ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) ListProductCertificationAuditLogsForAdmin(ctx context.Context, in *productpb.ListProductCertificationAuditLogsForAdminRequest, opts ...grpc.CallOption) (*productpb.ListProductCertificationAuditLogsForAdminResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) ApproveProductCertificationForAdmin(ctx context.Context, in *productpb.ReviewProductCertificationForAdminRequest, opts ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) RejectProductCertificationForAdmin(ctx context.Context, in *productpb.ReviewProductCertificationForAdminRequest, opts ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) RevokeProductCertificationForAdmin(ctx context.Context, in *productpb.ReviewProductCertificationForAdminRequest, opts ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) ReverifyProductCertificationForAdmin(ctx context.Context, in *productpb.ReverifyProductCertificationForAdminRequest, opts ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	return nil, nil
}
