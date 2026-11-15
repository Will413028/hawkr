package social

import (
	"context"
	"fmt"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// Writes 是互動寫入的來源（*social.Module）：分享、讚、留言與討論區。PublicPost 取新留言所屬貼文的店家，組回應的 viewer_* 欄位。
type Writes interface {
	PublicPost(ctx context.Context, c social.Caller, id uuid.UUID) (social.Post, error)
	RecordShare(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error)
	LikePost(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error)
	UnlikePost(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error)
	LikeComment(ctx context.Context, c social.Caller, commentID uuid.UUID) (bool, error)
	UnlikeComment(ctx context.Context, c social.Caller, commentID uuid.UUID) (bool, error)
	CreateComment(ctx context.Context, c social.Caller, in social.CommentInput) (social.Comment, error)
	UpdateComment(ctx context.Context, c social.Caller, id uuid.UUID, body string) (social.Comment, error)
	DeleteComment(ctx context.Context, c social.Caller, id uuid.UUID) error
	CreateForumEntry(ctx context.Context, c social.Caller, in social.ForumInput) (social.ForumEntry, error)
	UpdateForumEntry(ctx context.Context, c social.Caller, id uuid.UUID, body string) (social.ForumEntry, error)
	DeleteForumEntry(ctx context.Context, c social.Caller, id uuid.UUID) error
}

// App 實作 SocialAppService。
type App struct {
	*presenter
	writes Writes
}

var _ socialv1alpha1connect.SocialAppServiceHandler = (*App)(nil)

// NewApp 建立登入者互動寫入的 handler；webOrigin 與 Public 共用，組回應裡貼文的單篇網址，不可是零值。
func NewApp(writes Writes, buyers Buyers, stores Stores, images rpc.Images, webOrigin rpc.WebOrigin) (*App, error) {
	if webOrigin.IsZero() {
		return nil, fmt.Errorf("httpapi/social：貼文的單篇網址需要 WebOrigin")
	}
	return &App{presenter: &presenter{buyers: buyers, stores: stores, images: images, webOrigin: webOrigin}, writes: writes}, nil
}

var (
	createPaths     = rpc.FieldPaths{"Body": "body", "ReplyToID": "reply_to_id", "RequestID": "request_id"}
	updateBodyPaths = rpc.FieldPaths{"Body": "body"}
)

// account 回傳呼叫者與他的互動身分；只有顧客帳號能呼叫，沒有登入由 interceptor 擋（unauthenticated），管理員回 permission_denied。
func (h *App) account(ctx context.Context) (social.Caller, error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAccount {
		return social.Caller{}, rpc.Error(connect.CodePermissionDenied, "%s", "需要買家身分")
	}
	return h.caller(ctx)
}

// optionalUUID 解析選填的 ID；沒設是 nil。
func optionalUUID(path string, raw *string) (*uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := rpc.ParseUUID(path, *raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// requestID 解析選填的 request_id：UUIDv4（API 契約 field_info.format = UUID4）；不設是 nil。
func requestID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := rpc.ParseUUID("request_id", raw)
	if err != nil {
		return nil, err
	}
	if id[6]>>4 != 4 {
		return nil, rpc.RequiredField("request_id", "request_id 必須是 UUIDv4")
	}
	return &id, nil
}

func (h *App) SharePost(ctx context.Context, req *connect.Request[pb.SharePostRequest]) (*connect.Response[pb.SharePostResponse], error) {
	postID, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.writes.RecordShare(ctx, c.BuyerID, postID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SharePostResponse{}), nil
}

func (h *App) LikePost(ctx context.Context, req *connect.Request[pb.LikePostRequest]) (*connect.Response[pb.LikePostResponse], error) {
	postID, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.writes.LikePost(ctx, c.BuyerID, postID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.LikePostResponse{}), nil
}

func (h *App) UnlikePost(ctx context.Context, req *connect.Request[pb.UnlikePostRequest]) (*connect.Response[pb.UnlikePostResponse], error) {
	postID, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.writes.UnlikePost(ctx, c.BuyerID, postID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnlikePostResponse{}), nil
}

func (h *App) LikeComment(ctx context.Context, req *connect.Request[pb.LikeCommentRequest]) (*connect.Response[pb.LikeCommentResponse], error) {
	id, err := rpc.ParseUUID("comment_id", req.Msg.GetCommentId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.writes.LikeComment(ctx, c, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.LikeCommentResponse{}), nil
}

func (h *App) UnlikeComment(ctx context.Context, req *connect.Request[pb.UnlikeCommentRequest]) (*connect.Response[pb.UnlikeCommentResponse], error) {
	id, err := rpc.ParseUUID("comment_id", req.Msg.GetCommentId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.writes.UnlikeComment(ctx, c, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnlikeCommentResponse{}), nil
}

func (h *App) CreateComment(ctx context.Context, req *connect.Request[pb.CreateCommentRequest]) (*connect.Response[pb.CreateCommentResponse], error) {
	msg := req.Msg
	postID, err := rpc.ParseUUID("post_id", msg.GetPostId())
	if err != nil {
		return nil, err
	}
	parentID, err := optionalUUID("parent_id", msg.ParentId)
	if err != nil {
		return nil, err
	}
	replyToID, err := optionalUUID("reply_to_id", msg.ReplyToId)
	if err != nil {
		return nil, err
	}
	reqID, err := requestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	created, err := h.writes.CreateComment(ctx, c, social.CommentInput{PostID: postID, Body: msg.GetBody(), ParentID: parentID, ReplyToID: replyToID, RequestID: reqID})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createPaths)
	}
	post, err := h.writes.PublicPost(ctx, c, postID)
	if err != nil {
		return nil, err
	}
	comments, err := h.toComments(ctx, c, post.StoreID, []social.Comment{created})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateCommentResponse{Comment: comments[0]}), nil
}

func (h *App) UpdateComment(ctx context.Context, req *connect.Request[pb.UpdateCommentRequest]) (*connect.Response[pb.UpdateCommentResponse], error) {
	id, err := rpc.ParseUUID("comment_id", req.Msg.GetCommentId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := h.writes.UpdateComment(ctx, c, id, req.Msg.GetBody())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updateBodyPaths)
	}
	return connect.NewResponse(&pb.UpdateCommentResponse{Body: updated.Body, UpdateTime: timestamppb.New(updated.UpdatedAt)}), nil
}

func (h *App) DeleteComment(ctx context.Context, req *connect.Request[pb.DeleteCommentRequest]) (*connect.Response[pb.DeleteCommentResponse], error) {
	id, err := rpc.ParseUUID("comment_id", req.Msg.GetCommentId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.writes.DeleteComment(ctx, c, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteCommentResponse{}), nil
}

func (h *App) CreateForumEntry(ctx context.Context, req *connect.Request[pb.CreateForumEntryRequest]) (*connect.Response[pb.CreateForumEntryResponse], error) {
	msg := req.Msg
	storeID, err := rpc.ParseID("store_id", msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	parentID, err := optionalUUID("parent_id", msg.ParentId)
	if err != nil {
		return nil, err
	}
	replyToID, err := optionalUUID("reply_to_id", msg.ReplyToId)
	if err != nil {
		return nil, err
	}
	reqID, err := requestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	created, err := h.writes.CreateForumEntry(ctx, c, social.ForumInput{StoreID: storeID, Body: msg.GetBody(), ParentID: parentID, ReplyToID: replyToID, RequestID: reqID})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createPaths)
	}
	entries, err := h.toForumEntries(ctx, c, storeID, []social.ForumEntry{created})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateForumEntryResponse{Entry: entries[0]}), nil
}

func (h *App) UpdateForumEntry(ctx context.Context, req *connect.Request[pb.UpdateForumEntryRequest]) (*connect.Response[pb.UpdateForumEntryResponse], error) {
	id, err := rpc.ParseUUID("entry_id", req.Msg.GetEntryId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := h.writes.UpdateForumEntry(ctx, c, id, req.Msg.GetBody())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updateBodyPaths)
	}
	return connect.NewResponse(&pb.UpdateForumEntryResponse{Body: updated.Body, UpdateTime: timestamppb.New(updated.UpdatedAt)}), nil
}

func (h *App) DeleteForumEntry(ctx context.Context, req *connect.Request[pb.DeleteForumEntryRequest]) (*connect.Response[pb.DeleteForumEntryResponse], error) {
	id, err := rpc.ParseUUID("entry_id", req.Msg.GetEntryId())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.writes.DeleteForumEntry(ctx, c, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteForumEntryResponse{}), nil
}
