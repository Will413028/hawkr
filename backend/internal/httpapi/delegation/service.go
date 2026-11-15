// Package delegation 是 DelegationDelegatedService 的 handler 與它專屬的 interceptor：代行營運中的平台管理員以代行 session 讀寫代行店家的貼文。
// 只做傳輸：解析請求、呼叫 delegation 模組的 use case（稽核與 social 呼叫都在模組內）、把結果轉成回應。
package delegation

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/delegation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/delegation/v1alpha1/delegationv1alpha1connect"
	socialpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpsocial "github.com/Will413028/hawkr/backend/internal/httpapi/social"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// Posts 是代行貼文 use case 的來源（*delegation.Module）：稽核、冪等與 social 呼叫都在模組內，handler 只轉換。
type Posts interface {
	DelegatedCreatePost(ctx context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedCreatePostInput) (moddelegation.DelegatedPostResult, error)
	DelegatedUpdatePost(ctx context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedUpdatePostInput) (moddelegation.DelegatedPostResult, error)
	DelegatedPublishPost(ctx context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedPostTransitionInput) (moddelegation.DelegatedPostResult, error)
	DelegatedUnpublishPost(ctx context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedPostTransitionInput) (moddelegation.DelegatedPostResult, error)
	DelegatedSchedulePost(ctx context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedSchedulePostInput) (moddelegation.DelegatedPostResult, error)
	DelegatedPost(ctx context.Context, c moddelegation.DelegatedCaller, id uuid.UUID) (moddelegation.DelegatedPostResult, error)
	DelegatedPosts(ctx context.Context, c moddelegation.DelegatedCaller, status social.PostStatus, after *social.OwnPostCursor, size int32) (moddelegation.DelegatedPostPageResult, error)
}

// Service 實作 DelegationDelegatedService。
type Service struct {
	posts Posts
	own   httpsocial.OwnPosts
}

var _ delegationv1alpha1connect.DelegationDelegatedServiceHandler = (*Service)(nil)

// New 建立 handler；own 是與店主方法同一份的貼文回應組裝。
func New(posts Posts, own httpsocial.OwnPosts) *Service { return &Service{posts: posts, own: own} }

// ownPostCursor 是清單的 page token 內容：(CreatedAt, ID)。
type ownPostCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c ownPostCursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

var (
	createPaths = rpc.FieldPaths{"Body": "body", "ImageToken": "image_staged_token", "ProductID": "product_id"}
	updatePaths = rpc.FieldPaths{"Body": "post.body", "ImageToken": "post.image_staged_token", "ClearImage": "post.image_staged_token", "ProductID": "post.product_id"}
	schedPaths  = rpc.FieldPaths{"PublishAt": "publish_time"}
)

// updatableFields 是 UpdatePost 的 update_mask 可以列的欄位（etag 列進去會略過）。
var updatableFields = []string{"body", "image_staged_token", "product_id", "hashtag_inputs"}

// operation 解析每個寫入共同的稽核資料：request_id 必填（UUIDv4）；操作原因的值域在模組內檢查；notify_seller 不設是通知。
func operation(op *pb.DelegatedOperation) (moddelegation.DelegatedOperation, error) {
	if !op.ProtoReflect().IsValid() {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation", "operation 必填")
	}
	id, err := rpc.ParseRequestID(op.GetRequestId())
	if err != nil || id == nil {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation.request_id", "operation.request_id 必填，而且必須是 UUIDv4")
	}
	reason := strings.TrimSpace(op.GetOperationReason())
	if reason == "" || utf8.RuneCountInString(reason) > 100 {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation.operation_reason", "operation.operation_reason 必填，去除首尾空白後 1 到 100 個字元")
	}
	return moddelegation.DelegatedOperation{RequestID: *id, Reason: reason, NotifySeller: op.NotifySeller == nil || op.GetNotifySeller()}, nil
}

func (h *Service) result(ctx context.Context, r moddelegation.DelegatedPostResult) (*socialpb.Post, error) {
	return h.own.Post(ctx, r.Store, r.Post)
}

func (h *Service) ListPosts(ctx context.Context, req *connect.Request[pb.ListPostsRequest]) (*connect.Response[pb.ListPostsResponse], error) {
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	var status social.PostStatus
	if msg.GetStatus() != socialpb.PostStatus_POST_STATUS_UNSPECIFIED {
		status = rpc.EnumFromProto[social.PostStatus](msg.GetStatus())
		if !slices.Contains(social.PostStatusValues(), status) {
			return nil, rpc.RequiredField("status", "status 不是有效的貼文狀態")
		}
	}
	scope := rpc.FilterScope(delegationv1alpha1connect.DelegationDelegatedServiceListPostsProcedure, strconv.FormatInt(c.Session.Target.StoreID, 10), string(status))
	after, err := rpc.ValidCursor[ownPostCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var cur *social.OwnPostCursor
	if after != nil {
		cur = &social.OwnPostCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	res, err := h.posts.DelegatedPosts(ctx, c, status, cur, msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	posts, err := h.own.Posts(ctx, res.Store, res.Page.Posts)
	if err != nil {
		return nil, err
	}
	out := &pb.ListPostsResponse{Posts: posts}
	if res.Page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, ownPostCursor{CreatedAt: res.Page.Next.CreatedAt, ID: res.Page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

func (h *Service) GetPost(ctx context.Context, req *connect.Request[pb.GetPostRequest]) (*connect.Response[pb.GetPostResponse], error) {
	id, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.posts.DelegatedPost(ctx, c, id)
	if err != nil {
		return nil, err
	}
	post, err := h.result(ctx, res)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetPostResponse{Post: post}), nil
}

func (h *Service) CreatePost(ctx context.Context, req *connect.Request[pb.CreatePostRequest]) (*connect.Response[pb.CreatePostResponse], error) {
	msg := req.Msg
	op, err := operation(msg.GetOperation())
	if err != nil {
		return nil, err
	}
	var productID *int64
	if msg.ProductId != nil {
		id, err := rpc.ParseID("product_id", msg.GetProductId())
		if err != nil {
			return nil, err
		}
		productID = &id
	}
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.posts.DelegatedCreatePost(ctx, c, moddelegation.DelegatedCreatePostInput{
		Op: op, Body: msg.GetBody(), ImageToken: msg.GetImageStagedToken(), ProductID: productID, Hashtags: msg.GetHashtagInputs(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createPaths)
	}
	post, err := h.result(ctx, res)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreatePostResponse{Post: post, AuditLogId: res.AuditLogID.String()}), nil
}

func (h *Service) UpdatePost(ctx context.Context, req *connect.Request[pb.UpdatePostRequest]) (*connect.Response[pb.UpdatePostResponse], error) {
	op, err := operation(req.Msg.GetOperation())
	if err != nil {
		return nil, err
	}
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
	paths, err := rpc.WritableFields(msg, req.Msg.GetUpdateMask())
	if err != nil {
		return nil, err
	}
	paths = slices.DeleteFunc(paths, func(p string) bool { return p == "etag" })
	for _, p := range paths {
		if !slices.Contains(updatableFields, p) {
			return nil, rpc.RequiredField("update_mask", "update_mask 的 "+p+" 不是可以修改的欄位（可以修改：body、image_staged_token、product_id、hashtag_inputs）")
		}
	}
	if len(paths) == 0 {
		return nil, rpc.RequiredField("update_mask", "update_mask 必須列至少一個可以修改的欄位（body、image_staged_token、product_id、hashtag_inputs）")
	}
	has := func(p string) bool { return slices.Contains(paths, p) }
	in := social.UpdatePostInput{PostID: id, Version: version, SetProduct: has("product_id")}
	if in.SetProduct && msg.ProductId != nil {
		pid, err := rpc.ParseID("post.product_id", msg.GetProductId())
		if err != nil {
			return nil, err
		}
		in.ProductID = &pid
	}
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
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.posts.DelegatedUpdatePost(ctx, c, moddelegation.DelegatedUpdatePostInput{Op: op, Update: in})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updatePaths)
	}
	post, err := h.result(ctx, res)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdatePostResponse{Post: post, AuditLogId: res.AuditLogID.String()}), nil
}

// transition 是發布與下架共同的流程：operation、貼文 ID 與必填的 etag，再由 do 呼叫模組。
func (h *Service) transition(ctx context.Context, op *pb.DelegatedOperation, postID, etag string,
	do func(context.Context, moddelegation.DelegatedCaller, moddelegation.DelegatedPostTransitionInput) (moddelegation.DelegatedPostResult, error)) (*socialpb.Post, string, error) {
	o, err := operation(op)
	if err != nil {
		return nil, "", err
	}
	id, err := rpc.ParseUUID("post_id", postID)
	if err != nil {
		return nil, "", err
	}
	version, err := rpc.RequiredVersionField(etag, "etag")
	if err != nil {
		return nil, "", err
	}
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, "", err
	}
	res, err := do(ctx, c, moddelegation.DelegatedPostTransitionInput{Op: o, PostID: id, Version: version})
	if err != nil {
		return nil, "", err
	}
	post, err := h.result(ctx, res)
	return post, res.AuditLogID.String(), err
}

func (h *Service) PublishPost(ctx context.Context, req *connect.Request[pb.PublishPostRequest]) (*connect.Response[pb.PublishPostResponse], error) {
	post, audit, err := h.transition(ctx, req.Msg.GetOperation(), req.Msg.GetPostId(), req.Msg.GetEtag(), h.posts.DelegatedPublishPost)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.PublishPostResponse{Post: post, AuditLogId: audit}), nil
}

func (h *Service) UnpublishPost(ctx context.Context, req *connect.Request[pb.UnpublishPostRequest]) (*connect.Response[pb.UnpublishPostResponse], error) {
	post, audit, err := h.transition(ctx, req.Msg.GetOperation(), req.Msg.GetPostId(), req.Msg.GetEtag(), h.posts.DelegatedUnpublishPost)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnpublishPostResponse{Post: post, AuditLogId: audit}), nil
}

func (h *Service) SchedulePost(ctx context.Context, req *connect.Request[pb.SchedulePostRequest]) (*connect.Response[pb.SchedulePostResponse], error) {
	msg := req.Msg
	op, err := operation(msg.GetOperation())
	if err != nil {
		return nil, err
	}
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
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.posts.DelegatedSchedulePost(ctx, c, moddelegation.DelegatedSchedulePostInput{Op: op, PostID: id, Version: version, PublishAt: msg.GetPublishTime().AsTime()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, schedPaths)
	}
	post, err := h.result(ctx, res)
	if err != nil {
		return nil, err
	}
	out := &pb.SchedulePostResponse{Post: post, AuditLogId: res.AuditLogID.String()}
	if res.PublishAt != nil {
		out.PublishTime = timestamppb.New(*res.PublishAt)
	}
	return connect.NewResponse(out), nil
}
