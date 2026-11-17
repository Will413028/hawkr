package social

import (
	"context"
	"slices"
	"strconv"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

// Owner 是店主的貼文管理、店家範圍的留言與動態數的來源（*social.Module）：店家身分由 handler 以 seller.CurrentStore 推導後交給模組，
// 賣家能力在模組內檢查（帳號路徑與代行路徑同一個入口），handler 不檢查。
type Owner interface {
	CreatePost(ctx context.Context, store seller.Store, in social.CreatePostInput, actor *delegatedactor.Actor) (social.Post, error)
	UpdatePost(ctx context.Context, store seller.Store, in social.UpdatePostInput, actor *delegatedactor.Actor) (social.Post, error)
	PublishPost(ctx context.Context, store seller.Store, postID uuid.UUID, version int64, actor *delegatedactor.Actor) (social.Post, error)
	UnpublishPost(ctx context.Context, store seller.Store, postID uuid.UUID, version int64, actor *delegatedactor.Actor) (social.Post, error)
	DeleteOwnPost(ctx context.Context, store seller.Store, postID uuid.UUID, version int64) error
	SchedulePost(ctx context.Context, store seller.Store, in social.SchedulePostInput, actor *delegatedactor.Actor) (social.ScheduledPost, error)
	CancelPostSchedule(ctx context.Context, store seller.Store, postID uuid.UUID, version int64) (int64, error)
	ListPendingSchedules(ctx context.Context, storeID int64, after *social.ScheduleCursor, size int32) (social.SchedulePage, error)
	MyPost(ctx context.Context, store seller.Store, id uuid.UUID) (social.Post, error)
	ListMyPosts(ctx context.Context, store seller.Store, status social.PostStatus, after *social.CreatedPostCursor, size int32) (social.CreatedPostPage, error)
	ListStoreComments(ctx context.Context, store seller.Store, postID *uuid.UUID, unreplied bool, after *social.CommentCursor, size int32) (social.CommentPage, error)
	StoreSocialSummary(ctx context.Context, store seller.Store) (social.StoreSummary, error)
}

var (
	createPostPaths = rpc.FieldPaths{
		"Body": "body", "ImageToken": "image_staged_token", "ProductID": "product_id", "RequestID": "request_id",
	}
	updatePostPaths = rpc.FieldPaths{
		"Body": "post.body", "ImageToken": "post.image_staged_token", "ClearImage": "post.image_staged_token", "ProductID": "post.product_id",
	}
	schedulePostPaths = rpc.FieldPaths{"PublishAt": "publish_time"}
)

// updatablePostFields 是 UpdatePost 的 update_mask 可以列的欄位（etag 列進去會略過：它是版本檢查，不是被修改的欄位）。
var updatablePostFields = []string{"body", "image_staged_token", "product_id", "hashtag_inputs"}

// store 回傳呼叫者此刻的店；只有顧客帳號能呼叫（沒有登入由 interceptor 擋），沒有店回 not_found（seller.CurrentStore 的結果）。
// 能力不在這裡檢查：由模組檢查，宣告見 declarations。
func (h *App) store(ctx context.Context) (seller.Store, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return seller.Store{}, err
	}
	return h.stores.CurrentStore(ctx, p.ID)
}

// postState 是店主與後台讀貼文才有的三個欄位：狀態、建立時間與下架措施 ID（沒有被下架是 nil）。
func postState(post social.Post) (pb.PostStatus, *timestamppb.Timestamp, *string) {
	var takedown *string
	if post.TakedownMeasureID != nil {
		id := post.TakedownMeasureID.String()
		takedown = &id
	}
	return rpc.EnumToProto[pb.PostStatus](post.Status), timestamppb.New(post.CreatedAt), takedown
}

// ownPosts 把店主讀自己的貼文轉成回應：公開貼文的欄位加上 status、etag、create_time 與 takedown_measure_id（只有店主讀自己的貼文才設）。
func (p *presenter) ownPosts(ctx context.Context, st seller.Store, posts []social.Post) ([]*pb.Post, error) {
	out, err := p.toPosts(ctx, social.Caller{Store: &st}, posts)
	if err != nil {
		return nil, err
	}
	for i, post := range posts {
		out[i].Status, out[i].CreateTime, out[i].TakedownMeasureId = postState(post)
		out[i].Etag = rpc.Etag(post.Version)
	}
	return out, nil
}

func (p *presenter) ownPost(ctx context.Context, st seller.Store, post social.Post) (*pb.Post, error) {
	posts, err := p.ownPosts(ctx, st, []social.Post{post})
	if err != nil {
		return nil, err
	}
	return posts[0], nil
}

// optionalProductID 解析選填的商品 ID；沒設是 nil。
func optionalProductID(path string, raw *string) (*int64, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := rpc.ParseID(path, *raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// postStatusFilter 解析清單的 status 篩選；UNSPECIFIED 是全部（空字串）。
func postStatusFilter(s pb.PostStatus) (social.PostStatus, error) {
	if s == pb.PostStatus_POST_STATUS_UNSPECIFIED {
		return "", nil
	}
	status := rpc.EnumFromProto[social.PostStatus](s)
	if !slices.Contains(social.PostStatusValues(), status) {
		return "", rpc.RequiredField("status", "status 不是有效的貼文狀態")
	}
	return status, nil
}

// updatePostInput 把 UpdatePost 的 post 與 update_mask 轉成模組的輸入：只有 mask 列出的欄位被修改，列了不能修改的欄位或沒有列任何欄位是錯誤。
// 店主與代行的 UpdatePost 共用。
func updatePostInput(msg *pb.Post, mask *fieldmaskpb.FieldMask, id uuid.UUID, version int64) (social.UpdatePostInput, error) {
	paths, err := rpc.WritableFields(msg, mask)
	if err != nil {
		return social.UpdatePostInput{}, err
	}
	paths = slices.DeleteFunc(paths, func(p string) bool { return p == "etag" })
	for _, p := range paths {
		if !slices.Contains(updatablePostFields, p) {
			return social.UpdatePostInput{}, rpc.RequiredField("update_mask", "update_mask 的 "+p+" 不是可以修改的欄位（可以修改：body、image_staged_token、product_id、hashtag_inputs）")
		}
	}
	if len(paths) == 0 {
		return social.UpdatePostInput{}, rpc.RequiredField("update_mask", "update_mask 必須列至少一個可以修改的欄位（body、image_staged_token、product_id、hashtag_inputs）")
	}
	has := func(p string) bool { return slices.Contains(paths, p) }
	var productID *int64
	if has("product_id") {
		if productID, err = optionalProductID("post.product_id", msg.ProductId); err != nil {
			return social.UpdatePostInput{}, err
		}
	}
	in := social.UpdatePostInput{PostID: id, Version: version, SetProduct: has("product_id"), ProductID: productID}
	if has("body") {
		body := msg.GetBody()
		in.Body = &body
	}
	if has("image_staged_token") {
		if msg.GetImageStagedToken() == "" {
			in.ClearImage = true
		} else {
			in.ImageToken = msg.GetImageStagedToken()
		}
	}
	if has("hashtag_inputs") {
		tags := slices.Clone(msg.GetHashtagInputs())
		if tags == nil {
			tags = []string{}
		}
		in.Hashtags = &tags
	}
	return in, nil
}

func (h *App) CreatePost(ctx context.Context, req *connect.Request[pb.CreatePostRequest]) (*connect.Response[pb.CreatePostResponse], error) {
	msg := req.Msg
	productID, err := optionalProductID("product_id", msg.ProductId)
	if err != nil {
		return nil, err
	}
	reqID, err := rpc.ParseRequestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	created, err := h.owner.CreatePost(ctx, st, social.CreatePostInput{
		Body: msg.GetBody(), ImageToken: msg.GetImageStagedToken(), ProductID: productID, Hashtags: msg.GetHashtagInputs(),
		Publish: msg.GetPublish(), RequestID: reqID,
	}, nil)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createPostPaths)
	}
	post, err := h.ownPost(ctx, st, created)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreatePostResponse{Post: post}), nil
}

func (h *App) GetMyPost(ctx context.Context, req *connect.Request[pb.GetMyPostRequest]) (*connect.Response[pb.GetMyPostResponse], error) {
	id, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.owner.MyPost(ctx, st, id)
	if err != nil {
		return nil, err
	}
	post, err := h.ownPost(ctx, st, p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetMyPostResponse{Post: post}), nil
}

func (h *App) ListMyPosts(ctx context.Context, req *connect.Request[pb.ListMyPostsRequest]) (*connect.Response[pb.ListMyPostsResponse], error) {
	msg := req.Msg
	status, err := postStatusFilter(msg.GetStatus())
	if err != nil {
		return nil, err
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(socialv1alpha1connect.SocialAppServiceListMyPostsProcedure, strconv.FormatInt(st.StoreID, 10), string(status))
	after, err := rpc.ValidCursor[createdPostCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var cur *social.CreatedPostCursor
	if after != nil {
		cur = &social.CreatedPostCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	page, err := h.owner.ListMyPosts(ctx, st, status, cur, msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	posts, err := h.ownPosts(ctx, st, page.Posts)
	if err != nil {
		return nil, err
	}
	out := &pb.ListMyPostsResponse{Posts: posts}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, createdPostCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

func (h *App) UpdatePost(ctx context.Context, req *connect.Request[pb.UpdatePostRequest]) (*connect.Response[pb.UpdatePostResponse], error) {
	msg := req.Msg.GetPost()
	if !msg.ProtoReflect().IsValid() {
		return nil, rpc.RequiredField("post", "post 必填")
	}
	id, err := rpc.ParseUUID("post.id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "post.etag")
	if err != nil {
		return nil, err
	}
	in, err := updatePostInput(msg, req.Msg.GetUpdateMask(), id, version)
	if err != nil {
		return nil, err
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := h.owner.UpdatePost(ctx, st, in, nil)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updatePostPaths)
	}
	post, err := h.ownPost(ctx, st, updated)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdatePostResponse{Post: post}), nil
}

// transition 是發布與下架共同的流程：貼文 ID、必填的 etag、店家，再由 do 呼叫模組。
func (h *App) transition(ctx context.Context, postID, etag string, do func(context.Context, seller.Store, uuid.UUID, int64, *delegatedactor.Actor) (social.Post, error)) (*pb.Post, error) {
	id, err := rpc.ParseUUID("post_id", postID)
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(etag, "etag")
	if err != nil {
		return nil, err
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	p, err := do(ctx, st, id, version, nil)
	if err != nil {
		return nil, err
	}
	return h.ownPost(ctx, st, p)
}

func (h *App) PublishPost(ctx context.Context, req *connect.Request[pb.PublishPostRequest]) (*connect.Response[pb.PublishPostResponse], error) {
	post, err := h.transition(ctx, req.Msg.GetPostId(), req.Msg.GetEtag(), h.owner.PublishPost)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.PublishPostResponse{Post: post}), nil
}

func (h *App) UnpublishPost(ctx context.Context, req *connect.Request[pb.UnpublishPostRequest]) (*connect.Response[pb.UnpublishPostResponse], error) {
	post, err := h.transition(ctx, req.Msg.GetPostId(), req.Msg.GetEtag(), h.owner.UnpublishPost)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnpublishPostResponse{Post: post}), nil
}

func (h *App) DeletePost(ctx context.Context, req *connect.Request[pb.DeletePostRequest]) (*connect.Response[pb.DeletePostResponse], error) {
	id, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(req.Msg.GetEtag())
	if err != nil {
		return nil, err
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.owner.DeleteOwnPost(ctx, st, id, version); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeletePostResponse{}), nil
}

func (h *App) SchedulePost(ctx context.Context, req *connect.Request[pb.SchedulePostRequest]) (*connect.Response[pb.SchedulePostResponse], error) {
	msg := req.Msg
	id, err := rpc.ParseUUID("post_id", msg.GetPostId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	if msg.GetPublishTime() == nil || msg.GetPublishTime().CheckValid() != nil {
		return nil, rpc.RequiredField("publish_time", "publish_time 必填，而且必須是有效的時間")
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	scheduled, err := h.owner.SchedulePost(ctx, st, social.SchedulePostInput{
		PostID: id, Version: version, PublishAt: msg.GetPublishTime().AsTime(), By: social.ScheduledBy{Kind: social.ScheduleByOwner},
	}, nil)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, schedulePostPaths)
	}
	post, err := h.ownPost(ctx, st, scheduled.Post)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SchedulePostResponse{Post: post, PublishTime: timestamppb.New(scheduled.Schedule.PublishAt)}), nil
}

func (h *App) ListStoreComments(ctx context.Context, req *connect.Request[pb.ListStoreCommentsRequest]) (*connect.Response[pb.ListStoreCommentsResponse], error) {
	msg := req.Msg
	postID, err := optionalUUID("post_id", msg.PostId)
	if err != nil {
		return nil, err
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	scopePost := ""
	if postID != nil {
		scopePost = postID.String()
	}
	scope := rpc.FilterScope(socialv1alpha1connect.SocialAppServiceListStoreCommentsProcedure, strconv.FormatInt(st.StoreID, 10), scopePost, boolKey(msg.GetUnreplied()))
	after, err := rpc.ValidCursor[commentCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var cur *social.CommentCursor
	if after != nil {
		cur = &social.CommentCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	page, err := h.owner.ListStoreComments(ctx, st, postID, msg.GetUnreplied(), cur, msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	comments, err := h.toComments(ctx, social.Caller{Store: &st}, page.StoreID, page.Comments)
	if err != nil {
		return nil, err
	}
	out := &pb.ListStoreCommentsResponse{Comments: comments}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, commentCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

func boolKey(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (h *App) GetStoreSocialSummary(ctx context.Context, _ *connect.Request[pb.GetStoreSocialSummaryRequest]) (*connect.Response[pb.GetStoreSocialSummaryResponse], error) {
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	s, err := h.owner.StoreSocialSummary(ctx, st)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetStoreSocialSummaryResponse{ScheduledPostCount: s.Scheduled, PostCounts: postStatusCounts(s)}), nil
}

// postStatusCounts 每個貼文狀態一項（沒有貼文是 0），店主與代行共用。
func postStatusCounts(s social.StoreSummary) []*pb.PostStatusCount {
	var out []*pb.PostStatusCount
	for _, status := range social.PostStatusValues() {
		out = append(out, &pb.PostStatusCount{Status: rpc.EnumToProto[pb.PostStatus](status), Count: s.Posts[status]})
	}
	return out
}
