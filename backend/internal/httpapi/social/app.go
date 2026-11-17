package social

import (
	"context"
	"fmt"
	"slices"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// Writes 是互動寫入的來源（*social.Module）：分享、讚、留言與討論區。
type Writes interface {
	RecordShare(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error)
	LikePost(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error)
	UnlikePost(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error)
	LikeComment(ctx context.Context, c social.Caller, commentID uuid.UUID) (bool, error)
	UnlikeComment(ctx context.Context, c social.Caller, commentID uuid.UUID) (bool, error)
	CreateComment(ctx context.Context, c social.Caller, in social.CommentInput) (social.Comment, error)
	UpdateComment(ctx context.Context, c social.Caller, id uuid.UUID, version int64, body string) (social.Comment, error)
	DeleteComment(ctx context.Context, c social.Caller, id uuid.UUID, version int64) error
	CreateForumEntry(ctx context.Context, c social.Caller, in social.ForumInput) (social.ForumEntry, error)
	UpdateForumEntry(ctx context.Context, c social.Caller, id uuid.UUID, version int64, body string) (social.ForumEntry, error)
	DeleteForumEntry(ctx context.Context, c social.Caller, id uuid.UUID, version int64) error
}

// App 實作 SocialAppService。
type App struct {
	*presenter
	writes   Writes
	authored Authored
	owner    Owner
}

var _ socialv1alpha1connect.SocialAppServiceHandler = (*App)(nil)

// NewApp 建立登入者互動寫入與店主貼文管理的 handler；webOrigin 與 Public 共用，組回應裡貼文的單篇網址，不可是零值。
// 能力宣告表（declarations）與 SocialAppService 的方法對不上時回錯誤，不能啟動。
func NewApp(writes Writes, authored Authored, owner Owner, buyers Buyers, stores Stores, products Products, images rpc.Images, webOrigin rpc.WebOrigin) (*App, error) {
	return newApp(declarations, writes, authored, owner, buyers, stores, products, images, webOrigin)
}

// newApp 的宣告表由參數給（測試用缺一列的表驗證啟動檢查）。
func newApp(table map[string]declaration, writes Writes, authored Authored, owner Owner, buyers Buyers, stores Stores, products Products, images rpc.Images, webOrigin rpc.WebOrigin) (*App, error) {
	if webOrigin.IsZero() {
		return nil, fmt.Errorf("httpapi/social：貼文的單篇網址需要 WebOrigin")
	}
	if err := checkDeclarations(table, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{presenter: &presenter{buyers: buyers, stores: stores, products: products, ownerView: true, images: images, webOrigin: webOrigin, liked: noViewerLikes}, writes: writes, authored: authored, owner: owner}, nil
}

// noViewerLikes 是店主路徑的 liked：店主的回應不帶買家的讚，永遠是空的。
func noViewerLikes(context.Context, int64, []uuid.UUID) (map[uuid.UUID]bool, error) {
	return nil, nil
}

var (
	createPaths        = rpc.FieldPaths{"Body": "body", "ReplyToID": "reply_to_id", "RequestID": "request_id"}
	updateCommentPaths = rpc.FieldPaths{"Body": "comment.body"}
	updateEntryPaths   = rpc.FieldPaths{"Body": "entry.body"}
)

// account 回傳呼叫者與他的互動身分；只有顧客帳號能呼叫，沒有登入由 interceptor 擋（unauthenticated），管理員回 permission_denied。
func (h *App) account(ctx context.Context) (social.Caller, error) {
	if _, err := rpc.RequireAccount(ctx); err != nil {
		return social.Caller{}, err
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

// bodyUpdate 解析 Update 請求：資源的 id、必填的 etag、update_mask 要列 body（內文是唯一可以修改的欄位）；field 是資源在請求裡的欄位名。
func bodyUpdate(field string, resource proto.Message, mask *fieldmaskpb.FieldMask, id, etag, body string) (uuid.UUID, int64, string, error) {
	if !resource.ProtoReflect().IsValid() {
		return uuid.UUID{}, 0, "", rpc.RequiredField(field, field+" 必填")
	}
	uid, err := rpc.ParseUUID(field+".id", id)
	if err != nil {
		return uuid.UUID{}, 0, "", err
	}
	version, err := rpc.RequiredVersionField(etag, field+".etag")
	if err != nil {
		return uuid.UUID{}, 0, "", err
	}
	paths, err := rpc.WritableFields(resource, mask)
	if err != nil {
		return uuid.UUID{}, 0, "", err
	}
	if !slices.Contains(paths, "body") {
		return uuid.UUID{}, 0, "", rpc.RequiredField("update_mask", "update_mask 必須列 body（內文是唯一可以修改的欄位）")
	}
	return uid, version, body, nil
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
	reqID, err := rpc.ParseRequestID(msg.GetRequestId())
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
	comments, err := h.toComments(ctx, c, created.StoreID, []social.Comment{created})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateCommentResponse{Comment: comments[0]}), nil
}

func (h *App) UpdateComment(ctx context.Context, req *connect.Request[pb.UpdateCommentRequest]) (*connect.Response[pb.UpdateCommentResponse], error) {
	msg := req.Msg.GetComment()
	id, version, body, err := bodyUpdate("comment", msg, req.Msg.GetUpdateMask(), msg.GetId(), msg.GetEtag(), msg.GetBody())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := h.writes.UpdateComment(ctx, c, id, version, body)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updateCommentPaths)
	}
	comments, err := h.toComments(ctx, c, updated.StoreID, []social.Comment{updated})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateCommentResponse{Comment: comments[0]}), nil
}

func (h *App) DeleteComment(ctx context.Context, req *connect.Request[pb.DeleteCommentRequest]) (*connect.Response[pb.DeleteCommentResponse], error) {
	id, err := rpc.ParseUUID("comment_id", req.Msg.GetCommentId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(req.Msg.GetEtag())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.writes.DeleteComment(ctx, c, id, version); err != nil {
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
	reqID, err := rpc.ParseRequestID(msg.GetRequestId())
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
	msg := req.Msg.GetEntry()
	id, version, body, err := bodyUpdate("entry", msg, req.Msg.GetUpdateMask(), msg.GetId(), msg.GetEtag(), msg.GetBody())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := h.writes.UpdateForumEntry(ctx, c, id, version, body)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updateEntryPaths)
	}
	entries, err := h.toForumEntries(ctx, c, updated.StoreID, []social.ForumEntry{updated})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateForumEntryResponse{Entry: entries[0]}), nil
}

func (h *App) DeleteForumEntry(ctx context.Context, req *connect.Request[pb.DeleteForumEntryRequest]) (*connect.Response[pb.DeleteForumEntryResponse], error) {
	id, err := rpc.ParseUUID("entry_id", req.Msg.GetEntryId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(req.Msg.GetEtag())
	if err != nil {
		return nil, err
	}
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.writes.DeleteForumEntry(ctx, c, id, version); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteForumEntryResponse{}), nil
}
