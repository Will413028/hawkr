package handler

import (
	"context"

	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"google.golang.org/grpc"
)

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

// ListStoreIDsWithPublicProductsInternal is the seller module's caller-side RPC
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
