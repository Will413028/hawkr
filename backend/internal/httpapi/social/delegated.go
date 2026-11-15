package social

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

// DelegatedDeclaration 是 SocialDelegatedService 每個方法的代行宣告（API 設計規範（Connect）§二的 Delegated）：需要的代行能力，以及稽核列的動作與資源種類
// （讀取被拒絕時寫 POST_READ）。key 是生成的 procedure 常數；interceptor（httpapi/delegation）啟動時檢查它與 service 的方法一一對應。
var DelegatedDeclaration = httpdelegation.Declaration{
	Service: pb.File_hawkr_social_v1alpha1_social_proto.Services().ByName("SocialDelegatedService"),
	Methods: map[string]moddelegation.DelegatedMethod{
		socialv1alpha1connect.SocialDelegatedServiceListStorePostsProcedure:     postMethod(moddelegation.CapabilityPostsRead, moddelegation.AuditPostRead),
		socialv1alpha1connect.SocialDelegatedServiceGetStorePostProcedure:       postMethod(moddelegation.CapabilityPostsRead, moddelegation.AuditPostRead),
		socialv1alpha1connect.SocialDelegatedServiceCreateStorePostProcedure:    postMethod(moddelegation.CapabilityPostsWrite, moddelegation.AuditPostCreate),
		socialv1alpha1connect.SocialDelegatedServiceUpdateStorePostProcedure:    postMethod(moddelegation.CapabilityPostsWrite, moddelegation.AuditPostUpdate),
		socialv1alpha1connect.SocialDelegatedServicePublishStorePostProcedure:   postMethod(moddelegation.CapabilityPostsPublish, moddelegation.AuditPostPublish),
		socialv1alpha1connect.SocialDelegatedServiceUnpublishStorePostProcedure: postMethod(moddelegation.CapabilityPostsPublish, moddelegation.AuditPostUnpublish),
		socialv1alpha1connect.SocialDelegatedServiceScheduleStorePostProcedure:  postMethod(moddelegation.CapabilityPostsPublish, moddelegation.AuditPostSchedule),
	},
}

// delegatedCreatePostPaths 由店主建立的欄位對照表衍生：委派建立的 request_id 在 operation 之內。
var delegatedCreatePostPaths = func() rpc.FieldPaths {
	p := maps.Clone(createPostPaths)
	p["RequestID"] = "operation.request_id"
	return p
}()

func postMethod(c moddelegation.Capability, a moddelegation.AuditAction) moddelegation.DelegatedMethod {
	return moddelegation.DelegatedMethod{Capability: c, Action: a, Resource: moddelegation.ResourcePost}
}

// DelegationRunner 是 handler 需要的 delegation 公開 API（*delegation.Module）：授權、重送偵測與稽核（DelegatedWrite；不開 transaction，稽核由 social 的事件在 social 自己的 transaction 內寫，A7-D26），與讀取的授權（DelegatedRead）。
type DelegationRunner interface {
	DelegatedRead(ctx context.Context, c moddelegation.DelegatedCaller, read func(ctx context.Context, st seller.Store) error) error
	DelegatedWrite(ctx context.Context, c moddelegation.DelegatedCaller, w moddelegation.DelegatedWrite, do func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error) (moddelegation.DelegatedWriteResult, error)
}

// DelegatedPosts 是代行讀寫貼文的來源（*social.Module）：與店主同一組方法（店家由 delegation 以 session 的目標店家讀出後交給模組），加上重送時讀回排程時間。
type DelegatedPosts interface {
	Owner
	PendingSchedule(ctx context.Context, storeID int64, postID uuid.UUID) (social.Schedule, error)
}

// Delegated 實作 SocialDelegatedService：只做傳輸（解析請求、組回應）；授權、稽核、冪等與失敗紀錄在 delegation 的 DelegatedRead／DelegatedWrite，貼文的寫入在 social 模組。
type Delegated struct {
	*presenter
	runner DelegationRunner
	posts  DelegatedPosts
}

var _ socialv1alpha1connect.SocialDelegatedServiceHandler = (*Delegated)(nil)

// NewDelegated 建立 handler；webOrigin 與 App 共用，組回應裡貼文的單篇網址，不可是零值。
func NewDelegated(runner DelegationRunner, posts DelegatedPosts, buyers Buyers, stores Stores, images rpc.Images, webOrigin rpc.WebOrigin) (*Delegated, error) {
	if webOrigin.IsZero() {
		return nil, fmt.Errorf("httpapi/social：貼文的單篇網址需要 WebOrigin")
	}
	return &Delegated{presenter: &presenter{buyers: buyers, stores: stores, images: images, webOrigin: webOrigin}, runner: runner, posts: posts}, nil
}

// delegatedOperation 解析每個寫入共同的稽核資料：request_id 必填（UUIDv4）；操作原因 1 到 100 字；notify_seller 不設是通知。
func delegatedOperation(op *pb.DelegatedOperation) (moddelegation.DelegatedOperation, error) {
	if !op.ProtoReflect().IsValid() {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation", "operation 必填")
	}
	id, err := rpc.ParseRequestID(op.GetRequestId())
	if err != nil || id == nil {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation.request_id", "operation.request_id 必填，而且必須是 UUIDv4")
	}
	reason, ok := moddelegation.LegalOperationReason(op.GetOperationReason())
	if !ok {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation.operation_reason",
			fmt.Sprintf("operation.operation_reason 必填，去除首尾空白後 1 到 %d 個字元，不得含控制字元", moddelegation.OperationReasonMaxRunes))
	}
	return moddelegation.DelegatedOperation{RequestID: *id, Reason: reason, NotifySeller: op.NotifySeller == nil || op.GetNotifySeller()}, nil
}

// postPreview 是稽核列的資源名稱：內文去除首尾空白後的前 60 個字元。
func postPreview(body string) string {
	b := strings.TrimSpace(body)
	if r := []rune(b); len(r) > 60 {
		return string(r[:60])
	}
	return b
}

// delegatedWrite 是每個代行寫入共同的流程：交給 delegation 檢查重送與店家、帶著代行者（明確的參數，A7-D27）執行 do（social 在自己的 transaction 內發帶代行者的事件，稽核同 transaction 寫入）；重送（do 沒被呼叫）時讀回第一次寫入的貼文目前的樣子。
// 回傳貼文、店家、稽核列 ID，與排程的發布時間（只有排程）。
func (h *Delegated) delegatedWrite(ctx context.Context, w moddelegation.DelegatedWrite,
	do func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error)) (post social.Post, st seller.Store, auditID string, publishAt *time.Time, err error) {
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return social.Post{}, seller.Store{}, "", nil, err
	}
	res, err := h.runner.DelegatedWrite(ctx, c, w, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error {
		p, at, err := do(ctx, st, actor)
		if err != nil {
			return err
		}
		post, publishAt = p, at
		return nil
	})
	if err != nil {
		return social.Post{}, seller.Store{}, "", nil, err
	}
	if res.Replayed {
		id, err := uuid.Parse(res.ResourceID)
		if err != nil {
			return social.Post{}, seller.Store{}, "", nil, fmt.Errorf("httpapi/social：稽核列 %s 的資源 ID %q：%w", res.AuditLogID, res.ResourceID, err)
		}
		if post, err = h.posts.MyPost(ctx, res.Store, id); err != nil {
			return social.Post{}, seller.Store{}, "", nil, err
		}
		if c.Method.Action == moddelegation.AuditPostSchedule {
			s, err := h.posts.PendingSchedule(ctx, res.Store.StoreID, id)
			switch {
			case err == nil:
				at := s.PublishAt
				publishAt = &at
			case apperr.KindOf(err) != apperr.KindNotFound:
				return social.Post{}, seller.Store{}, "", nil, err
			}
		}
	}
	return post, res.Store, res.AuditLogID.String(), publishAt, nil
}

func (h *Delegated) ListStorePosts(ctx context.Context, req *connect.Request[pb.ListStorePostsRequest]) (*connect.Response[pb.ListStorePostsResponse], error) {
	msg := req.Msg
	status, err := postStatusFilter(msg.GetStatus())
	if err != nil {
		return nil, err
	}
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(socialv1alpha1connect.SocialDelegatedServiceListStorePostsProcedure, strconv.FormatInt(c.Session.Target.StoreID, 10), string(status))
	after, err := rpc.ValidCursor[ownPostCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var cur *social.OwnPostCursor
	if after != nil {
		cur = &social.OwnPostCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	out := &pb.ListStorePostsResponse{}
	err = h.runner.DelegatedRead(ctx, c, func(ctx context.Context, st seller.Store) error {
		page, err := h.posts.ListMyPosts(ctx, st, status, cur, msg.GetPageSize())
		if err != nil {
			return err
		}
		if out.Posts, err = h.ownPosts(ctx, st, page.Posts); err != nil {
			return err
		}
		if page.Next != nil {
			out.NextPageToken = rpc.TokenFor(scope, ownPostCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

func (h *Delegated) GetStorePost(ctx context.Context, req *connect.Request[pb.GetStorePostRequest]) (*connect.Response[pb.GetStorePostResponse], error) {
	id, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return nil, err
	}
	out := &pb.GetStorePostResponse{}
	err = h.runner.DelegatedRead(ctx, c, func(ctx context.Context, st seller.Store) error {
		p, err := h.posts.MyPost(ctx, st, id)
		if err != nil {
			return err
		}
		out.Post, err = h.ownPost(ctx, st, p)
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

func (h *Delegated) CreateStorePost(ctx context.Context, req *connect.Request[pb.CreateStorePostRequest]) (*connect.Response[pb.CreateStorePostResponse], error) {
	msg := req.Msg
	op, err := delegatedOperation(msg.GetOperation())
	if err != nil {
		return nil, err
	}
	productID, err := optionalProductID("product_id", msg.ProductId)
	if err != nil {
		return nil, err
	}
	post, st, auditID, _, err := h.delegatedWrite(ctx, moddelegation.DelegatedWrite{
		Op: op, ResourceName: postPreview(msg.GetBody()),
		PayloadHash: moddelegation.DelegatedPayloadHash(moddelegation.AuditPostCreate, msg.GetBody(), msg.GetImageStagedToken(), productID, msg.GetHashtagInputs()),
	}, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error) {
		p, err := h.posts.CreatePost(ctx, st, social.CreatePostInput{Body: msg.GetBody(), ImageToken: msg.GetImageStagedToken(), ProductID: productID, Hashtags: msg.GetHashtagInputs()}, actor)
		return p, nil, err
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, delegatedCreatePostPaths)
	}
	out, err := h.ownPost(ctx, st, post)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateStorePostResponse{Post: out, AuditLogId: auditID}), nil
}

func (h *Delegated) UpdateStorePost(ctx context.Context, req *connect.Request[pb.UpdateStorePostRequest]) (*connect.Response[pb.UpdateStorePostResponse], error) {
	op, err := delegatedOperation(req.Msg.GetOperation())
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
	in, err := updatePostInput(msg, req.Msg.GetUpdateMask(), id, version)
	if err != nil {
		return nil, err
	}
	post, st, auditID, _, err := h.delegatedWrite(ctx, moddelegation.DelegatedWrite{
		Op: op, ResourceID: id.String(), Version: &version,
		PayloadHash: moddelegation.DelegatedPayloadHash(moddelegation.AuditPostUpdate, in.PostID, in.Version, in.Body, in.ImageToken, in.ClearImage, in.SetProduct, in.ProductID, in.Hashtags),
	}, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error) {
		p, err := h.posts.UpdatePost(ctx, st, in, actor)
		return p, nil, err
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updatePostPaths)
	}
	out, err := h.ownPost(ctx, st, post)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateStorePostResponse{Post: out, AuditLogId: auditID}), nil
}

// transition 是發布與下架共同的流程：operation、貼文 ID 與必填的 etag，再由 do 呼叫模組。
func (h *Delegated) transition(ctx context.Context, op *pb.DelegatedOperation, postID, etag string, action moddelegation.AuditAction,
	do func(context.Context, seller.Store, uuid.UUID, int64, *delegatedactor.Actor) (social.Post, error)) (*pb.Post, string, error) {
	o, err := delegatedOperation(op)
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
	post, st, auditID, _, err := h.delegatedWrite(ctx, moddelegation.DelegatedWrite{
		Op: o, ResourceID: id.String(), Version: &version, PayloadHash: moddelegation.DelegatedPayloadHash(action, id, version),
	}, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error) {
		p, err := do(ctx, st, id, version, actor)
		return p, nil, err
	})
	if err != nil {
		return nil, "", err
	}
	out, err := h.ownPost(ctx, st, post)
	return out, auditID, err
}

func (h *Delegated) PublishStorePost(ctx context.Context, req *connect.Request[pb.PublishStorePostRequest]) (*connect.Response[pb.PublishStorePostResponse], error) {
	post, audit, err := h.transition(ctx, req.Msg.GetOperation(), req.Msg.GetPostId(), req.Msg.GetEtag(), moddelegation.AuditPostPublish, h.posts.PublishPost)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.PublishStorePostResponse{Post: post, AuditLogId: audit}), nil
}

func (h *Delegated) UnpublishStorePost(ctx context.Context, req *connect.Request[pb.UnpublishStorePostRequest]) (*connect.Response[pb.UnpublishStorePostResponse], error) {
	post, audit, err := h.transition(ctx, req.Msg.GetOperation(), req.Msg.GetPostId(), req.Msg.GetEtag(), moddelegation.AuditPostUnpublish, h.posts.UnpublishPost)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnpublishStorePostResponse{Post: post, AuditLogId: audit}), nil
}

func (h *Delegated) ScheduleStorePost(ctx context.Context, req *connect.Request[pb.ScheduleStorePostRequest]) (*connect.Response[pb.ScheduleStorePostResponse], error) {
	msg := req.Msg
	op, err := delegatedOperation(msg.GetOperation())
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
	publishAt := msg.GetPublishTime().AsTime()
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return nil, err
	}
	by := social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: strconv.FormatInt(c.Session.ID, 10)}
	post, st, auditID, at, err := h.delegatedWrite(ctx, moddelegation.DelegatedWrite{
		Op: op, ResourceID: id.String(), Version: &version,
		PayloadHash: moddelegation.DelegatedPayloadHash(moddelegation.AuditPostSchedule, id, version, publishAt.UTC()),
	}, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error) {
		s, err := h.posts.SchedulePost(ctx, st, social.SchedulePostInput{PostID: id, Version: version, PublishAt: publishAt, By: by}, actor)
		sat := s.Schedule.PublishAt
		return s.Post, &sat, err
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, schedulePostPaths)
	}
	out, err := h.ownPost(ctx, st, post)
	if err != nil {
		return nil, err
	}
	resp := &pb.ScheduleStorePostResponse{Post: out, AuditLogId: auditID}
	if at != nil {
		resp.PublishTime = timestamppb.New(*at)
	}
	return connect.NewResponse(resp), nil
}
