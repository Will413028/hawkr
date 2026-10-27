package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func setupMarketplaceRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	pub := v1.Group("/public")
	pub.Use(fakeOptionalAuth())
	pub.GET("/marketplace", h.GetMarketplace)
	return r
}

// ==================== Mock SocialServiceClient ====================

type mockSocialServiceClient struct {
	GetPostListFn        func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error)
	GetPostDetailFn      func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error)
	TogglePostLikeFn     func(ctx context.Context, in *socialpb.TogglePostLikeRequest, opts ...grpc.CallOption) (*socialpb.TogglePostLikeResponse, error)
	SharePostFn          func(ctx context.Context, in *socialpb.SharePostRequest, opts ...grpc.CallOption) (*socialpb.SharePostResponse, error)
	ReportPostFn         func(ctx context.Context, in *socialpb.ReportPostRequest, opts ...grpc.CallOption) (*socialpb.ReportPostResponse, error)
	GetCommentListFn     func(ctx context.Context, in *socialpb.GetCommentListRequest, opts ...grpc.CallOption) (*socialpb.CommentListResponse, error)
	CreateCommentFn      func(ctx context.Context, in *socialpb.CreateCommentRequest, opts ...grpc.CallOption) (*socialpb.CommentResponse, error)
	UpdateCommentFn      func(ctx context.Context, in *socialpb.UpdateCommentRequest, opts ...grpc.CallOption) (*socialpb.CommentResponse, error)
	DeleteCommentFn      func(ctx context.Context, in *socialpb.DeleteCommentRequest, opts ...grpc.CallOption) (*socialpb.CommentOperationResponse, error)
	ToggleCommentLikeFn  func(ctx context.Context, in *socialpb.ToggleCommentLikeRequest, opts ...grpc.CallOption) (*socialpb.ToggleCommentLikeResponse, error)
	GetSellerForumFn     func(ctx context.Context, in *socialpb.GetSellerForumRequest, opts ...grpc.CallOption) (*socialpb.ForumListResponse, error)
	GetForumRepliesFn    func(ctx context.Context, in *socialpb.GetForumRepliesRequest, opts ...grpc.CallOption) (*socialpb.ForumListResponse, error)
	CreateForumCommentFn func(ctx context.Context, in *socialpb.CreateForumCommentRequest, opts ...grpc.CallOption) (*socialpb.ForumCommentResponse, error)
	ReplyForumCommentFn  func(ctx context.Context, in *socialpb.ReplyForumCommentRequest, opts ...grpc.CallOption) (*socialpb.ForumCommentResponse, error)
	UpdateForumCommentFn func(ctx context.Context, in *socialpb.UpdateForumCommentRequest, opts ...grpc.CallOption) (*socialpb.ForumCommentResponse, error)
	DeleteForumCommentFn func(ctx context.Context, in *socialpb.DeleteForumCommentRequest, opts ...grpc.CallOption) (*socialpb.ForumOperationResponse, error)
	SubmitReportFn       func(ctx context.Context, in *socialpb.SubmitReportRequest, opts ...grpc.CallOption) (*socialpb.SubmitReportResponse, error)
	ListMyReportsFn      func(ctx context.Context, in *socialpb.ListMyReportsRequest, opts ...grpc.CallOption) (*socialpb.ListMyReportsResponse, error)
	BlockUserFn          func(ctx context.Context, in *socialpb.BlockUserRequest, opts ...grpc.CallOption) (*socialpb.BlockUserResponse, error)
	UnblockUserFn        func(ctx context.Context, in *socialpb.UnblockUserRequest, opts ...grpc.CallOption) (*socialpb.UnblockUserResponse, error)
	ListBlocksFn         func(ctx context.Context, in *socialpb.ListBlocksRequest, opts ...grpc.CallOption) (*socialpb.ListBlocksResponse, error)
	AdminGetReportFn     func(ctx context.Context, in *socialpb.AdminGetReportRequest, opts ...grpc.CallOption) (*socialpb.AdminGetReportResponse, error)
	AdminResolveReportFn func(ctx context.Context, in *socialpb.AdminResolveReportRequest, opts ...grpc.CallOption) (*socialpb.AdminResolveReportResponse, error)
	AdminReopenReportFn  func(ctx context.Context, in *socialpb.AdminReopenReportRequest, opts ...grpc.CallOption) (*socialpb.AdminReopenReportResponse, error)
}

func (m *mockSocialServiceClient) GetPostList(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
	if m.GetPostListFn != nil {
		return m.GetPostListFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) GetPostDetail(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
	if m.GetPostDetailFn != nil {
		return m.GetPostDetailFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) CreatePost(ctx context.Context, in *socialpb.CreatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
	return nil, nil
}
func (m *mockSocialServiceClient) UpdatePost(ctx context.Context, in *socialpb.UpdatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
	return nil, nil
}
func (m *mockSocialServiceClient) DeletePost(ctx context.Context, in *socialpb.DeletePostRequest, opts ...grpc.CallOption) (*socialpb.PostOperationResponse, error) {
	return nil, nil
}
func (m *mockSocialServiceClient) UpdateSellerPostStatus(ctx context.Context, in *socialpb.UpdateSellerPostStatusRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
	return nil, nil
}
func (m *mockSocialServiceClient) TogglePostLike(ctx context.Context, in *socialpb.TogglePostLikeRequest, opts ...grpc.CallOption) (*socialpb.TogglePostLikeResponse, error) {
	if m.TogglePostLikeFn != nil {
		return m.TogglePostLikeFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) SharePost(ctx context.Context, in *socialpb.SharePostRequest, opts ...grpc.CallOption) (*socialpb.SharePostResponse, error) {
	if m.SharePostFn != nil {
		return m.SharePostFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) GetCommentList(ctx context.Context, in *socialpb.GetCommentListRequest, opts ...grpc.CallOption) (*socialpb.CommentListResponse, error) {
	if m.GetCommentListFn != nil {
		return m.GetCommentListFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) CreateComment(ctx context.Context, in *socialpb.CreateCommentRequest, opts ...grpc.CallOption) (*socialpb.CommentResponse, error) {
	if m.CreateCommentFn != nil {
		return m.CreateCommentFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) UpdateComment(ctx context.Context, in *socialpb.UpdateCommentRequest, opts ...grpc.CallOption) (*socialpb.CommentResponse, error) {
	if m.UpdateCommentFn != nil {
		return m.UpdateCommentFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) DeleteComment(ctx context.Context, in *socialpb.DeleteCommentRequest, opts ...grpc.CallOption) (*socialpb.CommentOperationResponse, error) {
	if m.DeleteCommentFn != nil {
		return m.DeleteCommentFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) ToggleCommentLike(ctx context.Context, in *socialpb.ToggleCommentLikeRequest, opts ...grpc.CallOption) (*socialpb.ToggleCommentLikeResponse, error) {
	if m.ToggleCommentLikeFn != nil {
		return m.ToggleCommentLikeFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) GetSellerForum(ctx context.Context, in *socialpb.GetSellerForumRequest, opts ...grpc.CallOption) (*socialpb.ForumListResponse, error) {
	if m.GetSellerForumFn != nil {
		return m.GetSellerForumFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) GetForumReplies(ctx context.Context, in *socialpb.GetForumRepliesRequest, opts ...grpc.CallOption) (*socialpb.ForumListResponse, error) {
	if m.GetForumRepliesFn != nil {
		return m.GetForumRepliesFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) CreateForumComment(ctx context.Context, in *socialpb.CreateForumCommentRequest, opts ...grpc.CallOption) (*socialpb.ForumCommentResponse, error) {
	if m.CreateForumCommentFn != nil {
		return m.CreateForumCommentFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) ReplyForumComment(ctx context.Context, in *socialpb.ReplyForumCommentRequest, opts ...grpc.CallOption) (*socialpb.ForumCommentResponse, error) {
	if m.ReplyForumCommentFn != nil {
		return m.ReplyForumCommentFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) UpdateForumComment(ctx context.Context, in *socialpb.UpdateForumCommentRequest, opts ...grpc.CallOption) (*socialpb.ForumCommentResponse, error) {
	if m.UpdateForumCommentFn != nil {
		return m.UpdateForumCommentFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) DeleteForumComment(ctx context.Context, in *socialpb.DeleteForumCommentRequest, opts ...grpc.CallOption) (*socialpb.ForumOperationResponse, error) {
	if m.DeleteForumCommentFn != nil {
		return m.DeleteForumCommentFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) ReportPost(ctx context.Context, in *socialpb.ReportPostRequest, opts ...grpc.CallOption) (*socialpb.ReportPostResponse, error) {
	if m.ReportPostFn != nil {
		return m.ReportPostFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) ListMyPosts(ctx context.Context, in *socialpb.ListMyPostsRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
	return nil, nil
}
func (m *mockSocialServiceClient) AdminListPosts(ctx context.Context, in *socialpb.AdminListPostsRequest, opts ...grpc.CallOption) (*socialpb.AdminPostListResponse, error) {
	return nil, nil
}
func (m *mockSocialServiceClient) AdminModeratePost(ctx context.Context, in *socialpb.AdminModeratePostRequest, opts ...grpc.CallOption) (*socialpb.PostOperationResponse, error) {
	return nil, nil
}

// B-153 hashtag stubs.
func (m *mockSocialServiceClient) SearchHashtags(ctx context.Context, in *socialpb.SearchHashtagsRequest, opts ...grpc.CallOption) (*socialpb.SearchHashtagsResponse, error) {
	return nil, nil
}
func (m *mockSocialServiceClient) GetTrendingHashtags(ctx context.Context, in *socialpb.GetTrendingHashtagsRequest, opts ...grpc.CallOption) (*socialpb.GetTrendingHashtagsResponse, error) {
	return nil, nil
}

// B-135 stubs.
func (m *mockSocialServiceClient) SubmitReport(ctx context.Context, in *socialpb.SubmitReportRequest, opts ...grpc.CallOption) (*socialpb.SubmitReportResponse, error) {
	if m.SubmitReportFn != nil {
		return m.SubmitReportFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) ListMyReports(ctx context.Context, in *socialpb.ListMyReportsRequest, opts ...grpc.CallOption) (*socialpb.ListMyReportsResponse, error) {
	if m.ListMyReportsFn != nil {
		return m.ListMyReportsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) BlockUser(ctx context.Context, in *socialpb.BlockUserRequest, opts ...grpc.CallOption) (*socialpb.BlockUserResponse, error) {
	if m.BlockUserFn != nil {
		return m.BlockUserFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) UnblockUser(ctx context.Context, in *socialpb.UnblockUserRequest, opts ...grpc.CallOption) (*socialpb.UnblockUserResponse, error) {
	if m.UnblockUserFn != nil {
		return m.UnblockUserFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) ListBlocks(ctx context.Context, in *socialpb.ListBlocksRequest, opts ...grpc.CallOption) (*socialpb.ListBlocksResponse, error) {
	if m.ListBlocksFn != nil {
		return m.ListBlocksFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) AdminListReports(ctx context.Context, in *socialpb.AdminListReportsRequest, opts ...grpc.CallOption) (*socialpb.AdminListReportsResponse, error) {
	return nil, nil
}
func (m *mockSocialServiceClient) AdminGetReport(ctx context.Context, in *socialpb.AdminGetReportRequest, opts ...grpc.CallOption) (*socialpb.AdminGetReportResponse, error) {
	if m.AdminGetReportFn != nil {
		return m.AdminGetReportFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) AdminResolveReport(ctx context.Context, in *socialpb.AdminResolveReportRequest, opts ...grpc.CallOption) (*socialpb.AdminResolveReportResponse, error) {
	if m.AdminResolveReportFn != nil {
		return m.AdminResolveReportFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSocialServiceClient) AdminReopenReport(ctx context.Context, in *socialpb.AdminReopenReportRequest, opts ...grpc.CallOption) (*socialpb.AdminReopenReportResponse, error) {
	if m.AdminReopenReportFn != nil {
		return m.AdminReopenReportFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== Mock ProductServiceClient ====================

type mockProductServiceClient struct {
	GetProductListFn               func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error)
	GetPublicProductFn             func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error)
	CreateProductFn                func(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error)
	UpdateProductFn                func(ctx context.Context, in *productpb.UpdateProductRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error)
	GetProductFn                   func(ctx context.Context, in *productpb.GetProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error)
	DeleteProductFn                func(ctx context.Context, in *productpb.DeleteProductRequest, opts ...grpc.CallOption) (*productpb.DeleteProductResponse, error)
	GetPublicSellerCitiesSummaryFn func(ctx context.Context, in *productpb.GetPublicSellerCitiesSummaryRequest, opts ...grpc.CallOption) (*productpb.GetPublicSellerCitiesSummaryResponse, error)

	CalculateShippingOptionsFn func(ctx context.Context, in *productpb.CalculateShippingOptionsRequest, opts ...grpc.CallOption) (*productpb.CalculateShippingOptionsResponse, error)
	PurgeProductCacheFn        func(ctx context.Context, in *productpb.PurgeProductCacheRequest, opts ...grpc.CallOption) (*productpb.PurgeProductCacheResponse, error)
	VerifyCertificationFn      func(ctx context.Context, in *productpb.VerifyCertificationRequest, opts ...grpc.CallOption) (*productpb.VerifyCertificationResponse, error)
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
func (m *mockProductServiceClient) CalculateShippingOptions(ctx context.Context, in *productpb.CalculateShippingOptionsRequest, opts ...grpc.CallOption) (*productpb.CalculateShippingOptionsResponse, error) {
	if m.CalculateShippingOptionsFn != nil {
		return m.CalculateShippingOptionsFn(ctx, in, opts...)
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

// ListStoreIDsWithPublicProductsInternal is seller-service's caller-side RPC
// (02 §5.13c) — the gateway never issues it; the method exists here only to keep
// this mock satisfying productpb.ProductServiceClient.
func (m *mockProductServiceClient) ListStoreIDsWithPublicProductsInternal(ctx context.Context, in *productpb.ListStoreIDsWithPublicProductsInternalRequest, opts ...grpc.CallOption) (*productpb.ListStoreIDsWithPublicProductsInternalResponse, error) {
	return nil, nil
}

// ===== product-import 服務內部用 RPC（02 §9.1 / §9.2 / §9.x，#40 PR2 新增） =====

func (m *mockProductServiceClient) ValidateLeafCategoryInternal(ctx context.Context, in *productpb.ValidateLeafCategoryInternalRequest, opts ...grpc.CallOption) (*productpb.ValidateLeafCategoryInternalResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) CreateProductFromImportInternal(ctx context.Context, in *productpb.CreateProductFromImportInternalRequest, opts ...grpc.CallOption) (*productpb.CreateProductFromImportInternalResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) BatchCheckSellerProductNames(ctx context.Context, in *productpb.BatchCheckSellerProductNamesRequest, opts ...grpc.CallOption) (*productpb.BatchCheckSellerProductNamesResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) ValidateProductDraft(ctx context.Context, in *productpb.ValidateProductDraftRequest, opts ...grpc.CallOption) (*productpb.ValidateProductDraftResponse, error) {
	return nil, nil
}

func (m *mockProductServiceClient) GetProductsBriefInternal(ctx context.Context, in *productpb.GetProductsBriefInternalRequest, opts ...grpc.CallOption) (*productpb.GetProductsBriefInternalResponse, error) {
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

// ==================== Test Helpers ====================

func setupMarketplaceHolder(sellerMock *mockSellerServiceClient, socialMock *mockSocialServiceClient, productMock *mockProductServiceClient) *Holder {
	h := &Holder{}
	if sellerMock != nil {
		h.Seller = grpcclient.NewSellerClientForTest(sellerMock)
	}
	if socialMock != nil {
		h.Social = grpcclient.NewSocialClientForTest(socialMock)
	}
	if productMock != nil {
		h.Product = grpcclient.NewProductClientForTest(productMock)
	}
	return h
}

func defaultSellerMock() *mockSellerServiceClient {
	return &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			return &sellerpb.GetRecommendedSellersResponse{
				Code: 0,
				Sellers: []*sellerpb.RecommendedSeller{
					{StoreId: "1001", Name: "Store A", LogoKey: "logo_a.jpg", CoverKey: "cover_a.jpg", Description: "Desc A", Rating: 4.8, City: "台北市", District: "大安區"},
					{StoreId: "1002", Name: "Store B", LogoKey: "logo_b.jpg", CoverKey: "cover_b.jpg", Description: "Desc B", Rating: 4.5, City: "桃園市", District: "桃園區"},
				},
			}, nil
		},
	}
}

func defaultSocialMock() *mockSocialServiceClient {
	return &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			return &socialpb.PostListResponse{
				Data: []*socialpb.PostListItem{
					{Id: "1", Content: "Post 1", SellerId: "1001", LikeCount: 10, CommentCount: 5, ViewCount: 100, ShareCount: 2, CreatedAt: 1700000000},
				},
				Total: 1, Page: 1, PageSize: 10,
			}, nil
		},
	}
}

func defaultProductMock() *mockProductServiceClient {
	return &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code: 0,
				Products: []*productpb.ProductListItem{
					{Id: "prod-1", Name: "Product A", ImageKey: "img_a.jpg", PriceMin: "100", PriceMax: "200", StoreId: "1001", Rating: "4.5"},
				},
				Total: "1", Page: "1", PageSize: "10",
			}, nil
		},
	}
}

// ==================== Tests ====================

func TestGetMarketplace_AllSuccess(t *testing.T) {
	h := setupMarketplaceHolder(defaultSellerMock(), defaultSocialMock(), defaultProductMock())

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})

	sellers := data["recommendedSellers"].([]interface{})
	if len(sellers) != 2 {
		t.Errorf("expected 2 sellers, got %d", len(sellers))
	}
	first := sellers[0].(map[string]interface{})
	if first["storeName"] != "Store A" {
		t.Errorf("first store_name = %q, want %q", first["storeName"], "Store A")
	}
	if first["city"] != "台北市" {
		t.Errorf("first city = %q, want 台北市", first["city"])
	}
	if first["district"] != "大安區" {
		t.Errorf("first district = %q, want 大安區", first["district"])
	}
	second := sellers[1].(map[string]interface{})
	if second["city"] != "桃園市" {
		t.Errorf("second city = %q, want 桃園市", second["city"])
	}
	if second["district"] != "桃園區" {
		t.Errorf("second district = %q, want 桃園區", second["district"])
	}

	posts := data["latestPosts"].([]interface{})
	if len(posts) != 1 {
		t.Errorf("expected 1 post, got %d", len(posts))
	}

	products := data["hotProducts"].([]interface{})
	if len(products) != 1 {
		t.Errorf("expected 1 product, got %d", len(products))
	}
	prod := products[0].(map[string]interface{})
	if prod["name"] != "Product A" {
		t.Errorf("product name = %q, want %q", prod["name"], "Product A")
	}
}

func TestGetMarketplace_SellerServiceDown(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := setupMarketplaceHolder(sellerMock, defaultSocialMock(), defaultProductMock())

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	sellers := data["recommendedSellers"].([]interface{})
	if len(sellers) != 0 {
		t.Errorf("expected 0 sellers (service down), got %d", len(sellers))
	}
	// other sections should still work
	posts := data["latestPosts"].([]interface{})
	if len(posts) != 1 {
		t.Errorf("expected 1 post, got %d", len(posts))
	}
	products := data["hotProducts"].([]interface{})
	if len(products) != 1 {
		t.Errorf("expected 1 product, got %d", len(products))
	}
}

func TestGetMarketplace_SocialServiceDown(t *testing.T) {
	socialMock := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := setupMarketplaceHolder(defaultSellerMock(), socialMock, defaultProductMock())

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	posts := data["latestPosts"].([]interface{})
	if len(posts) != 0 {
		t.Errorf("expected 0 posts (service down), got %d", len(posts))
	}
	sellers := data["recommendedSellers"].([]interface{})
	if len(sellers) != 2 {
		t.Errorf("expected 2 sellers, got %d", len(sellers))
	}
}

func TestGetMarketplace_ProductServiceDown(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := setupMarketplaceHolder(defaultSellerMock(), defaultSocialMock(), productMock)

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	products := data["hotProducts"].([]interface{})
	if len(products) != 0 {
		t.Errorf("expected 0 products (service down), got %d", len(products))
	}
	sellers := data["recommendedSellers"].([]interface{})
	if len(sellers) != 2 {
		t.Errorf("expected 2 sellers, got %d", len(sellers))
	}
}

func TestGetMarketplace_AllServicesDown(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	socialMock := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := setupMarketplaceHolder(sellerMock, socialMock, productMock)

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	sellers := data["recommendedSellers"].([]interface{})
	posts := data["latestPosts"].([]interface{})
	products := data["hotProducts"].([]interface{})
	if len(sellers) != 0 || len(posts) != 0 || len(products) != 0 {
		t.Errorf("expected all empty arrays, got sellers=%d posts=%d products=%d", len(sellers), len(posts), len(products))
	}
}

func TestGetMarketplace_CustomLimits(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			if in.Limit != 5 {
				t.Errorf("seller limit = %d, want 5", in.Limit)
			}
			return &sellerpb.GetRecommendedSellersResponse{Code: 0, Sellers: []*sellerpb.RecommendedSeller{}}, nil
		},
	}
	socialMock := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			if in.PageSize != 8 {
				t.Errorf("post page_size = %d, want 8", in.PageSize)
			}
			return &socialpb.PostListResponse{Data: []*socialpb.PostListItem{}, Total: 0, Page: 1, PageSize: 8}, nil
		},
	}
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			if in.PageSize != "3" {
				t.Errorf("product page_size = %q, want %q", in.PageSize, "3")
			}
			return &productpb.GetProductListResponse{Code: 0, Products: []*productpb.ProductListItem{}, Total: "0", Page: "1", PageSize: "3"}, nil
		},
	}
	h := setupMarketplaceHolder(sellerMock, socialMock, productMock)

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace?seller_limit=5&post_limit=8&product_limit=3", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
}

func TestGetMarketplace_LimitsCapped(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			if in.Limit != 50 {
				t.Errorf("seller limit = %d, want 50 (capped from 100)", in.Limit)
			}
			return &sellerpb.GetRecommendedSellersResponse{Code: 0, Sellers: []*sellerpb.RecommendedSeller{}}, nil
		},
	}
	h := setupMarketplaceHolder(sellerMock, defaultSocialMock(), defaultProductMock())

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace?seller_limit=100&post_limit=100&product_limit=100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

func TestGetMarketplace_HotProducts_SellerFields(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code: 0,
				Products: []*productpb.ProductListItem{
					{
						Id: "prod-1", Name: "屏東蓮霧", ImageKey: "img.jpg",
						PriceMin: "100", PriceMax: "200", StoreId: "9001", Rating: "4.8",
						SkuNames: []string{"大", "中"},
						Seller: &productpb.SellerInfo{
							StoreId: "9001",
							Name:    "黑貓測試家",
							LogoKey: "https://test-media.example.com/seller/avatar.jpg",
						},
					},
					{
						Id: "prod-2", Name: "全平台商品", ImageKey: "img2.jpg",
						PriceMin: "50", PriceMax: "50", StoreId: "9002", Rating: "4.0",
					},
				},
				Total: "2", Page: "1", PageSize: "10",
			}, nil
		},
	}
	h := setupMarketplaceHolder(defaultSellerMock(), defaultSocialMock(), productMock)

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	var body struct {
		Data struct {
			HotProducts []struct {
				ID              string `json:"id"`
				StoreID         string `json:"storeId"` // #2110: accountId → storeId（值一直是 store 代理鍵）
				SellerName      string `json:"sellerName"`
				SellerAvatarUrl string `json:"sellerLogoUrl"`
			} `json:"hotProducts"`
			LatestPosts []struct {
				ID       string `json:"id"`
				SellerID string `json:"sellerId"`
			} `json:"latestPosts"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Data.HotProducts) != 2 {
		t.Fatalf("hot_products count = %d, want 2", len(body.Data.HotProducts))
	}

	p0 := body.Data.HotProducts[0]
	if p0.SellerName != "黑貓測試家" {
		t.Errorf("hot_products[0].sellerName = %q, want %q", p0.SellerName, "黑貓測試家")
	}
	if p0.SellerAvatarUrl != "https://test-media.example.com/seller/avatar.jpg" {
		t.Errorf("hot_products[0].sellerLogoUrl = %q, want URL", p0.SellerAvatarUrl)
	}
	// #2110: storeId 為 string（與 /public/products 一致），取代舊名 accountId
	if p0.StoreID != "9001" {
		t.Errorf("hot_products[0].storeId = %q, want %q", p0.StoreID, "9001")
	}

	p1 := body.Data.HotProducts[1]
	if p1.SellerName != "" {
		t.Errorf("hot_products[1].sellerName = %q, want empty (no seller)", p1.SellerName)
	}
	if p1.SellerAvatarUrl != "" {
		t.Errorf("hot_products[1].sellerLogoUrl = %q, want empty (no seller)", p1.SellerAvatarUrl)
	}
	// B-139 guard: Seller 為 nil 時 storeId 為空字串（不暴露 orphan store 代理鍵）。
	// 注意 mock 的 p1 雖有 StoreId: "9002" 但無 Seller，handler 只在 Seller!=nil 時才回填 StoreID。
	if p1.StoreID != "" {
		t.Errorf("hot_products[1].storeId = %q, want empty when seller is nil (B-139 guard)", p1.StoreID)
	}

	if len(body.Data.LatestPosts) > 0 && body.Data.LatestPosts[0].SellerID != "1001" {
		t.Errorf("latest_posts[0].seller_id = %q, want 1001", body.Data.LatestPosts[0].SellerID)
	}
}

// TestGetMarketplace_HotProducts_PriceAndHashtags 驗證 B-171：
// /public/marketplace.hot_products 補齊 listPrice/salePrice/hashtags 欄位，
// 與 /public/products 的 ProductItem 對齊。
func TestGetMarketplace_HotProducts_PriceAndHashtags(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code: 0,
				Products: []*productpb.ProductListItem{
					{
						Id: "prod-1", Name: "屏東蓮霧", ImageKey: "img.jpg",
						PriceMin: "80", PriceMax: "150",
						ListPriceMin: "100", ListPriceMax: "200",
						SalePriceMin: "80", SalePriceMax: "150",
						StoreId: "9001", Rating: "4.8",
						Hashtags: []*productpb.HashtagItem{
							{Name: "lianwu", DisplayName: "蓮霧", ProductCount: "12"},
							{Name: "pingtung", DisplayName: "屏東", ProductCount: "34"},
						},
					},
				},
				Total: "1", Page: "1", PageSize: "10",
			}, nil
		},
	}
	h := setupMarketplaceHolder(defaultSellerMock(), defaultSocialMock(), productMock)

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	var body struct {
		Data struct {
			HotProducts []struct {
				ID           string `json:"id"`
				ListPriceMin string `json:"listPriceMin"`
				ListPriceMax string `json:"listPriceMax"`
				SalePriceMin string `json:"salePriceMin"`
				SalePriceMax string `json:"salePriceMax"`
				Hashtags     []struct {
					Name         string `json:"name"`
					DisplayName  string `json:"displayName"`
					ProductCount string `json:"productCount"`
				} `json:"hashtags"`
			} `json:"hotProducts"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Data.HotProducts) != 1 {
		t.Fatalf("hot_products count = %d, want 1", len(body.Data.HotProducts))
	}

	p := body.Data.HotProducts[0]
	if p.ListPriceMin != "100" || p.ListPriceMax != "200" {
		t.Errorf("listPriceMin/Max = %q/%q, want 100/200", p.ListPriceMin, p.ListPriceMax)
	}
	if p.SalePriceMin != "80" || p.SalePriceMax != "150" {
		t.Errorf("salePriceMin/Max = %q/%q, want 80/150", p.SalePriceMin, p.SalePriceMax)
	}
	if len(p.Hashtags) != 2 {
		t.Fatalf("hashtags count = %d, want 2", len(p.Hashtags))
	}
	if p.Hashtags[0].DisplayName != "蓮霧" {
		t.Errorf("hashtags[0].displayName = %q, want 蓮霧", p.Hashtags[0].DisplayName)
	}
	if p.Hashtags[0].ProductCount != "12" {
		t.Errorf("hashtags[0].productCount = %q, want \"12\"", p.Hashtags[0].ProductCount)
	}
}

// TestGetMarketplace_HotProducts_NoSalePrice 驗證 B-171：
// proto 端沒有特價時，salePriceMin/Max 為空字串並被 omitempty 省略，
// hashtags 為空時也不出現（與 /public/products 行為一致）。
func TestGetMarketplace_HotProducts_NoSalePrice(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code: 0,
				Products: []*productpb.ProductListItem{
					{
						Id: "prod-1", Name: "全價商品", ImageKey: "img.jpg",
						PriceMin: "100", PriceMax: "200",
						ListPriceMin: "100", ListPriceMax: "200",
						// SalePriceMin/Max 不設（皆無特價）
						// Hashtags 不設
						StoreId: "9001", Rating: "4.0",
					},
				},
				Total: "1", Page: "1", PageSize: "10",
			}, nil
		},
	}
	h := setupMarketplaceHolder(defaultSellerMock(), defaultSocialMock(), productMock)

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	// 用 map[string]interface{} 直接看 raw JSON 才能驗證欄位有沒有被 omitempty 省略
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	products := data["hotProducts"].([]interface{})
	if len(products) != 1 {
		t.Fatalf("hot_products count = %d, want 1", len(products))
	}
	prod := products[0].(map[string]interface{})

	// listPrice 有值，應出現
	if prod["listPriceMin"] != "100" {
		t.Errorf("listPriceMin = %v, want 100", prod["listPriceMin"])
	}

	// salePrice 為空，應被 omitempty 省略
	if _, ok := prod["salePriceMin"]; ok {
		t.Errorf("salePriceMin should be omitted when empty, got %v", prod["salePriceMin"])
	}
	if _, ok := prod["salePriceMax"]; ok {
		t.Errorf("salePriceMax should be omitted when empty, got %v", prod["salePriceMax"])
	}

	// hashtags 空，應被 omitempty 省略
	if _, ok := prod["hashtags"]; ok {
		t.Errorf("hashtags should be omitted when empty, got %v", prod["hashtags"])
	}

	// isFavorited 沒 omitempty（ProductItem.IsFavorited 是預設 false 的 placeholder），
	// 必須恆在；marketplace 目前未實作聚合，永遠回 false（PR description 標為 YAGNI 保留欄位）。
	if val, ok := prod["isFavorited"]; !ok {
		t.Errorf("isFavorited should always be present (no omitempty)")
	} else if val != false {
		t.Errorf("isFavorited = %v, want false (marketplace 未聚合，固定回 false)", val)
	}
}

// TestGetMarketplace_EnrichPairsLogoKeyAndURL 驗證 §231 收尾：
// enrichPostsWithSellerInfo 以 seller-service 資料覆蓋貼文賣家 logo 時，
// seller_logo_key（objectKey）與 seller_logo（URL）必須成對回填，
// 不可只蓋 URL 讓 key 留空造成不對稱。
func TestGetMarketplace_EnrichPairsLogoKeyAndURL(t *testing.T) {
	sellerMock := defaultSellerMock()
	sellerMock.GetSellerBatchFn = func(ctx context.Context, in *sellerpb.GetSellerBatchRequest, opts ...grpc.CallOption) (*sellerpb.SellerBatchResponse, error) {
		return &sellerpb.SellerBatchResponse{
			Code: 0,
			Data: []*sellerpb.SellerInfo{
				{AccountId: 1001, StoreId: "1001", Name: "覆蓋店名", LogoKey: "seller/logo_1001.jpg"},
			},
		}, nil
	}
	h := setupMarketplaceHolder(sellerMock, defaultSocialMock(), emptyProductMock())

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	var body struct {
		Data struct {
			LatestPosts []struct {
				SellerLogoKey string `json:"logoKey"`
				SellerLogoUrl string `json:"logoUrl"`
				SellerName    string `json:"sellerName"`
			} `json:"latestPosts"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.LatestPosts) != 1 {
		t.Fatalf("latest_posts count = %d, want 1", len(body.Data.LatestPosts))
	}
	p := body.Data.LatestPosts[0]
	if p.SellerLogoKey != "seller/logo_1001.jpg" {
		t.Errorf("logoKey = %q, want objectKey seller/logo_1001.jpg (key 必回填)", p.SellerLogoKey)
	}
	// canonical logoUrl 必成對非空（49 §6.1）
	if p.SellerLogoUrl == "" {
		t.Errorf("logoUrl is empty after enrichment (want non-empty)")
	}
	if p.SellerName != "覆蓋店名" {
		t.Errorf("seller_name = %q, want 覆蓋店名 (enrich 覆蓋)", p.SellerName)
	}
}

func TestGetMarketplace_DoesNotUseAccountIDAsMissingStoreIDFallback(t *testing.T) {
	sellerMock := defaultSellerMock()
	sellerMock.GetSellerBatchFn = func(context.Context, *sellerpb.GetSellerBatchRequest, ...grpc.CallOption) (*sellerpb.SellerBatchResponse, error) {
		return &sellerpb.SellerBatchResponse{
			Code: 0,
			Data: []*sellerpb.SellerInfo{
				{AccountId: 1001, StoreId: "", Name: "不應套用的店名"},
			},
		}, nil
	}
	h := setupMarketplaceHolder(sellerMock, defaultSocialMock(), emptyProductMock())

	router := setupMarketplaceRouter(h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil))
	assertHTTPStatus(t, w, http.StatusOK)

	var body struct {
		Data struct {
			LatestPosts []struct {
				SellerName string `json:"sellerName"`
			} `json:"latestPosts"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.LatestPosts) != 1 {
		t.Fatalf("latest_posts count = %d, want 1", len(body.Data.LatestPosts))
	}
	if got := body.Data.LatestPosts[0].SellerName; got == "不應套用的店名" {
		t.Fatalf("sellerName = %q; missing store_id must not fall back to account_id", got)
	}
}

// ==================== ② buyer store-stint re-key (W2/W3 PR1) ====================

// emptyProductMock 回傳零筆商品，讓 GetMarketplace 的 isRestockSubscribed 聚合
// 區塊（accountID!=0 && len(products)>0 才觸發，會呼叫 h.Buyer）被跳過——這兩個
// 測試只驗證貼文區段的 buyerid 解析，不需要 wire buyer client。
func emptyProductMock() *mockProductServiceClient {
	return &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{Code: 0, Products: []*productpb.ProductListItem{}, Total: "0", Page: "1", PageSize: "10"}, nil
		},
	}
}

func TestGetMarketplace_BuyerRoleResolvesCurrentBuyerIDForIsLiked(t *testing.T) {
	var captured *socialpb.GetPostListRequest
	socialMock := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			captured = in
			return &socialpb.PostListResponse{Data: []*socialpb.PostListItem{}, Total: 0, Page: 1, PageSize: 10}, nil
		},
	}
	h := &Holder{
		Seller:  grpcclient.NewSellerClientForTest(defaultSellerMock()),
		Social:  grpcclient.NewSocialClientForTest(socialMock),
		Product: grpcclient.NewProductClientForTest(emptyProductMock()),
		BuyerID: buyerResolverForTest(9999),
	}

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Login-Role", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("captured.BuyerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

func TestGetMarketplace_NoCurrentBuyerFallsBackToAnonymous(t *testing.T) {
	var captured *socialpb.GetPostListRequest
	socialMock := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			captured = in
			return &socialpb.PostListResponse{Data: []*socialpb.PostListItem{}, Total: 0, Page: 1, PageSize: 10}, nil
		},
	}
	h := &Holder{
		Seller:  grpcclient.NewSellerClientForTest(defaultSellerMock()),
		Social:  grpcclient.NewSocialClientForTest(socialMock),
		Product: grpcclient.NewProductClientForTest(emptyProductMock()),
		BuyerID: noBuyerResolverForTest(),
	}

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Login-Role", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 「無現任買家」對匿名個人化不是硬錯誤：整體請求仍 200，只是 posts 區退回
	// 「匿名視角」（is_liked 全 false），不阻斷 marketplace 頁面。
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 0 {
		t.Errorf("captured.BuyerId = %v, want 0 (anonymous fallback, not raw account_id 42 and not a 403)", captured)
	}
}

// TestGetMarketplace_SellerLogoURLFields proves GET /public/marketplace
// exposes both the recommended_sellers logo_url (already correct) and the
// latest_posts seller_logo_url (newly added, B-238 / issue #1782).
func TestGetMarketplace_SellerLogoURLFields(t *testing.T) {
	socialMock := &mockSocialServiceClient{
		GetPostListFn: func(ctx context.Context, in *socialpb.GetPostListRequest, opts ...grpc.CallOption) (*socialpb.PostListResponse, error) {
			return &socialpb.PostListResponse{
				Data: []*socialpb.PostListItem{
					{
						Id:            "post-1",
						Content:       "Test post with logo",
						SellerId:      "1001",
						SellerName:    "Test Seller",
						SellerLogoKey: "public/post-seller/logo.png",
						LikeCount:     10,
						CommentCount:  5,
						ViewCount:     100,
						ShareCount:    2,
						CreatedAt:     1700000000,
					},
				},
				Total: 1, Page: 1, PageSize: 10,
			}, nil
		},
	}

	h := setupMarketplaceHolder(defaultSellerMock(), socialMock, defaultProductMock())

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	posts := data["latestPosts"].([]interface{})
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}

	post := posts[0].(map[string]interface{})

	// Check that canonical logoUrl is present and non-empty (49 §6.1)
	logoURL, ok := post["logoUrl"]
	if !ok || logoURL == nil || logoURL == "" {
		t.Errorf("logoUrl = %v, want a non-empty built URL in latest_posts[0]", logoURL)
	}

	// recommended_sellers[0].logoUrl: this test's mock (defaultSellerMock) also returns
	// recommended sellers with a non-empty LogoKey, so assert it here directly.
	sellers := data["recommendedSellers"].([]interface{})
	if len(sellers) == 0 {
		t.Fatalf("expected at least 1 recommended seller, got 0")
	}
	seller0 := sellers[0].(map[string]interface{})
	if got := seller0["logoUrl"]; got == nil || got == "" {
		t.Errorf("recommended_sellers[0].logoUrl = %v, want a non-empty built URL", got)
	}
}

// TestGetMarketplace_HotProducts_LogoCanonicalFields proves hot_products[].seller
// exposes the canonical logo_key/logo_url fields (issue #1782/B-238) — this
// construction site was missed by the original Task 7 sweep (final whole-branch
// review finding) and only set the deprecated camelCase logoKey/logoUrl pair.
func TestGetMarketplace_HotProducts_LogoCanonicalFields(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code: 0,
				Products: []*productpb.ProductListItem{
					{
						Id: "prod-1", Name: "屏東蓮霧", ImageKey: "img.jpg",
						PriceMin: "100", PriceMax: "200", StoreId: "9001", Rating: "4.8",
						Seller: &productpb.SellerInfo{
							StoreId: "9001",
							Name:    "黑貓測試家",
							LogoKey: "public/seller/9001/logo.png",
						},
					},
				},
				Total: "1", Page: "1", PageSize: "10",
			}, nil
		},
	}
	h := setupMarketplaceHolder(defaultSellerMock(), defaultSocialMock(), productMock)

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	hotProducts := data["hotProducts"].([]interface{})
	if len(hotProducts) != 1 {
		t.Fatalf("hot_products count = %d, want 1", len(hotProducts))
	}
	seller := hotProducts[0].(map[string]interface{})["seller"].(map[string]interface{})

	// Exact-value key check (canonical camelCase; 49 §6.1)
	if got := seller["logoKey"]; got != "public/seller/9001/logo.png" {
		t.Errorf("hot_products[0].seller.logoKey = %v, want %q", got, "public/seller/9001/logo.png")
	}
	if got := seller["logoUrl"]; got == nil || got == "" {
		t.Errorf("hot_products[0].seller.logoUrl = %v, want a non-empty built URL", got)
	}
}

// TestGetMarketplace_CarriesStoreCode pins the seller display code onto both
// seller-bearing shapes of this aggregate (web#365 storefront canonicalization).
// The web home/marketplace rails link to a seller by store_code; when the field is
// absent the frontend helper deliberately falls back to the numeric store id, and
// every such link then costs one 308 redirect (crawlers included). Both shapes are
// enriched upstream already — the recommended-seller rows carry it on
// seller.RecommendedSeller, the hot products on product.SellerInfo (the very same
// RPC that already feeds /public/products' storeCode) — so a regression here would
// be a silent drop inside this handler, invisible to a shape-only check.
func TestGetMarketplace_CarriesStoreCode(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			return &sellerpb.GetRecommendedSellersResponse{
				Code: 0,
				Sellers: []*sellerpb.RecommendedSeller{
					{StoreId: "1001", StoreCode: "2612345678", Name: "Store A"},
				},
			}, nil
		},
	}
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code: 0,
				Products: []*productpb.ProductListItem{
					{
						Id: "prod-1", Name: "Product A", StoreId: "9001",
						Seller: &productpb.SellerInfo{
							StoreId:   "9001",
							StoreCode: "2687654321",
							Name:      "Store Hot",
							LogoKey:   "public/seller/9001/logo.png",
						},
					},
				},
				Total: "1", Page: "1", PageSize: "10",
			}, nil
		},
	}
	h := setupMarketplaceHolder(sellerMock, defaultSocialMock(), productMock)

	router := setupMarketplaceRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data := body["data"].(map[string]interface{})

	sellers := data["recommendedSellers"].([]interface{})
	if len(sellers) != 1 {
		t.Fatalf("recommended_sellers count = %d, want 1", len(sellers))
	}
	if got := sellers[0].(map[string]interface{})["storeCode"]; got != "2612345678" {
		t.Errorf("recommended_sellers[0].store_code = %v, want %q", got, "2612345678")
	}

	hotProducts := data["hotProducts"].([]interface{})
	if len(hotProducts) != 1 {
		t.Fatalf("hot_products count = %d, want 1", len(hotProducts))
	}
	seller, ok := hotProducts[0].(map[string]interface{})["seller"].(map[string]interface{})
	if !ok {
		t.Fatalf("hot_products[0].seller missing, got %#v", hotProducts[0])
	}
	if got := seller["storeCode"]; got != "2687654321" {
		t.Errorf("hot_products[0].seller.storeCode = %v, want %q", got, "2687654321")
	}
}
