package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// SocialClient is the gRPC client for social-service
type SocialClient struct {
	conn   *grpc.ClientConn
	client pb.SocialServiceClient
}

func NewSocialClient() (*SocialClient, error) {
	host := getEnvOrDefault("SOCIAL_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("SOCIAL_SERVICE_GRPC_PORT", "9918")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to social service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to social gRPC service at %s", target)

	return &SocialClient{
		conn:   conn,
		client: pb.NewSocialServiceClient(conn),
	}, nil
}

func (c *SocialClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== Post Methods ====================

func (c *SocialClient) GetPostList(ctx context.Context, buyerID int64, sellerID string, page, pageSize int32, tag string) (*pb.PostListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetPostList(ctx, &pb.GetPostListRequest{
		BuyerId:  buyerID,
		SellerId: sellerID,
		Page:     page,
		PageSize: pageSize,
		Tag:      tag,
	})
}

func (c *SocialClient) GetPostDetail(ctx context.Context, buyerID int64, postID string) (*pb.PostDetailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetPostDetail(ctx, &pb.GetPostDetailRequest{
		BuyerId: buyerID,
		PostId:  postID,
	})
}

// GetMyPostDetail is the owner's read of their own post. owner_store_id with no
// DelegationContext is what selects social-service's owner branch (any publish
// state, NotFound for another store's post, no view_count).
func (c *SocialClient) GetMyPostDetail(ctx context.Context, storeID identity.StoreID, postID string) (*pb.PostDetailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetPostDetail(ctx, &pb.GetPostDetailRequest{
		PostId:       postID,
		OwnerStoreId: int64(storeID),
	})
}

func (c *SocialClient) TogglePostLike(ctx context.Context, buyerID int64, postID string, storeID identity.StoreID) (*pb.TogglePostLikeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.TogglePostLike(ctx, &pb.TogglePostLikeRequest{
		BuyerId: buyerID,
		PostId:  postID,
		StoreId: int64(storeID), // proto 欄位仍是裸 int64，轉型只在組裝 wire request 這一刻發生
	})
}

func (c *SocialClient) CreatePost(ctx context.Context, storeID identity.StoreID, content string, stagedToken string, productID *string, tags []string) (*pb.PostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.CreatePostRequest{
		SellerId: int64(storeID),
		Content:  content,
		Tags:     tags, // nil OK (proto3 repeated default = empty slice)
	}
	if stagedToken != "" {
		req.StagedTokens = []string{stagedToken}
	}
	if productID != nil {
		req.ProductIds = []string{*productID}
	}
	return c.client.CreatePost(ctx, req)
}

func (c *SocialClient) UpdatePost(ctx context.Context, postID string, storeID identity.StoreID, content string, keepAssetID string, newStagedToken string, productID *string, tagsPresent bool, tags []string) (*pb.PostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.UpdatePostRequest{
		PostId:      postID,
		SellerId:    int64(storeID),
		Content:     content,
		TagsPresent: tagsPresent,
		Tags:        tags,
	}
	if keepAssetID != "" {
		req.KeepAssetIds = []string{keepAssetID}
	}
	if newStagedToken != "" {
		req.NewStagedTokens = []string{newStagedToken}
	}
	if productID != nil {
		req.ProductIds = []string{*productID}
	}
	return c.client.UpdatePost(ctx, req)
}

func (c *SocialClient) DeletePost(ctx context.Context, postID string, storeID identity.StoreID) (*pb.PostOperationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.DeletePost(ctx, &pb.DeletePostRequest{
		PostId:   postID,
		SellerId: int64(storeID),
	})
}

func (c *SocialClient) SharePost(ctx context.Context, postID string, userID int64, platform string) (*pb.SharePostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.SharePost(ctx, &pb.SharePostRequest{
		PostId:   postID,
		UserId:   userID,
		Platform: platform,
	})
}

func (c *SocialClient) ReportPost(ctx context.Context, reporterID int64, postID string, reasonCode, description string) (*pb.ReportPostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ReportPost(ctx, &pb.ReportPostRequest{
		ReporterId:  reporterID,
		PostId:      postID,
		ReasonCode:  reasonCode,
		Description: description,
	})
}

func (c *SocialClient) ListMyPosts(ctx context.Context, storeID identity.StoreID, status string, page, pageSize int32) (*pb.PostListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ListMyPosts(ctx, &pb.ListMyPostsRequest{
		SellerId: int64(storeID),
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	})
}

// UpdateMyPostStatus is the seller's own publish/unpublish call. It shares the RPC
// with the delegated path but carries no DelegationContext — that field being nil is
// what tells social-service this is the owner acting on their own post, and it is
// also why this cannot go through UpdateSellerPostStatusForDelegation (that wrapper
// requires a context and rejects a nil one).
func (c *SocialClient) UpdateMyPostStatus(ctx context.Context, postID string, storeID identity.StoreID, targetStatus string) (*pb.PostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.UpdateSellerPostStatus(ctx, &pb.UpdateSellerPostStatusRequest{
		PostId:       postID,
		SellerId:     int64(storeID),
		TargetStatus: targetStatus,
	})
}

func (c *SocialClient) AdminListPosts(ctx context.Context, status string, page, pageSize int32) (*pb.AdminPostListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.AdminListPosts(ctx, &pb.AdminListPostsRequest{
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	})
}

func (c *SocialClient) AdminModeratePost(ctx context.Context, postID string, action, reason, reasonCode string, operatorID int64) (*pb.PostOperationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.AdminModeratePost(ctx, &pb.AdminModeratePostRequest{
		PostId:     postID,
		Action:     action,
		Reason:     reason,
		OperatorId: operatorID,
		ReasonCode: reasonCode,
	})
}

// SocialCaller 是呼叫者此刻的兩個業務身分（單體帶來，沒有的是 0）。互動身分由 social 依目標的歸屬推導，
// gateway 只轉送這兩個 id。
type SocialCaller struct {
	BuyerID int64
	StoreID int64
}

// ==================== Comment Methods ====================

func (c *SocialClient) GetCommentList(ctx context.Context, postID string, caller SocialCaller, page, pageSize int32) (*pb.CommentListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetCommentList(ctx, &pb.GetCommentListRequest{
		PostId:   postID,
		BuyerId:  caller.BuyerID,
		StoreId:  caller.StoreID,
		Page:     page,
		PageSize: pageSize,
	})
}

func (c *SocialClient) CreateComment(ctx context.Context, postID string, caller SocialCaller, content string, parentID, replyToID string, clientRequestID string) (*pb.CommentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.CreateComment(ctx, &pb.CreateCommentRequest{
		PostId:          postID,
		BuyerId:         caller.BuyerID,
		StoreId:         caller.StoreID,
		Content:         content,
		ParentId:        parentID,
		ReplyToId:       replyToID,
		ClientRequestId: clientRequestID,
	})
}

func (c *SocialClient) UpdateComment(ctx context.Context, commentID string, caller SocialCaller, content string) (*pb.CommentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.UpdateComment(ctx, &pb.UpdateCommentRequest{
		CommentId: commentID,
		BuyerId:   caller.BuyerID,
		StoreId:   caller.StoreID,
		Content:   content,
	})
}

func (c *SocialClient) DeleteComment(ctx context.Context, commentID string, caller SocialCaller) (*pb.CommentOperationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.DeleteComment(ctx, &pb.DeleteCommentRequest{
		CommentId: commentID,
		BuyerId:   caller.BuyerID,
		StoreId:   caller.StoreID,
	})
}

func (c *SocialClient) ToggleCommentLike(ctx context.Context, commentID string, caller SocialCaller) (*pb.ToggleCommentLikeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ToggleCommentLike(ctx, &pb.ToggleCommentLikeRequest{
		CommentId: commentID,
		BuyerId:   caller.BuyerID,
		StoreId:   caller.StoreID,
	})
}

func (c *SocialClient) GetCommentReplies(ctx context.Context, postID string, caller SocialCaller, parentID string, page, pageSize int32) (*pb.CommentListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetCommentList(ctx, &pb.GetCommentListRequest{
		PostId:   postID,
		BuyerId:  caller.BuyerID,
		StoreId:  caller.StoreID,
		Page:     page,
		PageSize: pageSize,
		ParentId: parentID,
	})
}

// ==================== Forum Methods ====================

func (c *SocialClient) GetSellerForum(ctx context.Context, sellerID string, caller SocialCaller, page, pageSize int32) (*pb.ForumListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetSellerForum(ctx, &pb.GetSellerForumRequest{
		SellerId: sellerID,
		BuyerId:  caller.BuyerID,
		StoreId:  caller.StoreID,
		Page:     page,
		PageSize: pageSize,
	})
}

func (c *SocialClient) GetForumReplies(ctx context.Context, parentID string, caller SocialCaller, page, pageSize int32) (*pb.ForumListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetForumReplies(ctx, &pb.GetForumRepliesRequest{
		ParentId: parentID,
		BuyerId:  caller.BuyerID,
		StoreId:  caller.StoreID,
		Page:     page,
		PageSize: pageSize,
	})
}

func (c *SocialClient) CreateForumComment(ctx context.Context, sellerID string, caller SocialCaller, content string) (*pb.ForumCommentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.CreateForumComment(ctx, &pb.CreateForumCommentRequest{
		SellerId: sellerID,
		BuyerId:  caller.BuyerID,
		StoreId:  caller.StoreID,
		Content:  content,
	})
}

func (c *SocialClient) ReplyForumComment(ctx context.Context, parentID string, caller SocialCaller, content string) (*pb.ForumCommentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ReplyForumComment(ctx, &pb.ReplyForumCommentRequest{
		ParentId: parentID,
		BuyerId:  caller.BuyerID,
		StoreId:  caller.StoreID,
		Content:  content,
	})
}

func (c *SocialClient) UpdateForumComment(ctx context.Context, entryID string, caller SocialCaller, content string) (*pb.ForumCommentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.UpdateForumComment(ctx, &pb.UpdateForumCommentRequest{
		EntryId: entryID,
		BuyerId: caller.BuyerID,
		StoreId: caller.StoreID,
		Content: content,
	})
}

func (c *SocialClient) DeleteForumComment(ctx context.Context, entryID string, caller SocialCaller) (*pb.ForumOperationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.DeleteForumComment(ctx, &pb.DeleteForumCommentRequest{
		EntryId: entryID,
		BuyerId: caller.BuyerID,
		StoreId: caller.StoreID,
	})
}

// ==================== B-153: Post Hashtag ====================

func (c *SocialClient) SearchHashtags(ctx context.Context, q string, mode pb.HashtagSearchMode, limit int32) (*pb.SearchHashtagsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.SearchHashtags(ctx, &pb.SearchHashtagsRequest{
		Q:     q,
		Mode:  mode,
		Limit: limit,
	})
}

func (c *SocialClient) GetTrendingHashtags(ctx context.Context, startFrom, endTo string, limit int32) (*pb.GetTrendingHashtagsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.GetTrendingHashtags(ctx, &pb.GetTrendingHashtagsRequest{
		StartFrom: startFrom,
		EndTo:     endTo,
		Limit:     limit,
	})
}

// ==================== B-135: Reporting & Blocking ====================

func (c *SocialClient) SubmitReport(ctx context.Context, reporterID int64, targetType string, targetID string, reasonCode, description string, evidenceKeys []string) (*pb.SubmitReportResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SubmitReport(ctx, &pb.SubmitReportRequest{
		ReporterId:   reporterID,
		TargetType:   targetType,
		TargetId:     targetID,
		ReasonCode:   reasonCode,
		Description:  description,
		EvidenceKeys: evidenceKeys,
	})
}

func (c *SocialClient) ListMyReports(ctx context.Context, reporterID int64, page, pageSize int32) (*pb.ListMyReportsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListMyReports(ctx, &pb.ListMyReportsRequest{
		ReporterId: reporterID,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (c *SocialClient) BlockUser(ctx context.Context, blockerID int64, blockedID string) (*pb.BlockUserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.BlockUser(ctx, &pb.BlockUserRequest{BlockerId: blockerID, BlockedId: blockedID})
}

func (c *SocialClient) UnblockUser(ctx context.Context, blockerID int64, blockedID string) (*pb.UnblockUserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.UnblockUser(ctx, &pb.UnblockUserRequest{BlockerId: blockerID, BlockedId: blockedID})
}

func (c *SocialClient) ListBlocks(ctx context.Context, blockerID int64, page, pageSize int32) (*pb.ListBlocksResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListBlocks(ctx, &pb.ListBlocksRequest{
		BlockerId: blockerID,
		Page:      page,
		PageSize:  pageSize,
	})
}

func (c *SocialClient) AdminListReports(ctx context.Context, status, targetType, reasonCode string, page, pageSize int32) (*pb.AdminListReportsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminListReports(ctx, &pb.AdminListReportsRequest{
		Status:     status,
		TargetType: targetType,
		ReasonCode: reasonCode,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (c *SocialClient) AdminGetReport(ctx context.Context, reportID string) (*pb.AdminGetReportResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminGetReport(ctx, &pb.AdminGetReportRequest{ReportId: reportID})
}

func (c *SocialClient) AdminResolveReport(ctx context.Context, reportID string, adminID int64, action, note string) (*pb.AdminResolveReportResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminResolveReport(ctx, &pb.AdminResolveReportRequest{
		ReportId: reportID,
		AdminId:  adminID,
		Action:   action,
		Note:     note,
	})
}

func (c *SocialClient) AdminReopenReport(ctx context.Context, reportID string, adminID int64, note string) (*pb.AdminReopenReportResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.AdminReopenReport(ctx, &pb.AdminReopenReportRequest{
		ReportId: reportID,
		AdminId:  adminID,
		Note:     note,
	})
}
