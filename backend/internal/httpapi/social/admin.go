package social

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/social/v1alpha1/socialv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// AdminPosts 是後台貼文列表的來源（*social.Module）。
type AdminPosts interface {
	ListAdminPosts(ctx context.Context, f social.AdminPostFilter, after *social.AdminPostCursor, size int32) (social.AdminPostPage, error)
}

// OpenReportCounts 是貼文在進行中案件裡的檢舉數的來源（*moderation.Module）：沒有進行中檢舉的貼文不在結果裡。
// 用介面接，handler 不 import moderation：組合兩個模組的讀取是這一層的事。
type OpenReportCounts interface {
	OpenReportCounts(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID]int64, error)
}

// Admin 實作 SocialAdminService。授權（只有管理員、逐方法的權限）由 rpc 的 interceptor 與 adminauth 的宣告表檢查。
type Admin struct {
	posts   AdminPosts
	reports OpenReportCounts
}

var _ socialv1alpha1connect.SocialAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler；兩個來源都必填。
func NewAdmin(posts AdminPosts, reports OpenReportCounts) (*Admin, error) {
	if posts == nil || reports == nil {
		return nil, fmt.Errorf("httpapi/social：SocialAdminService 需要貼文與檢舉數的來源")
	}
	return &Admin{posts: posts, reports: reports}, nil
}

// adminPostCursor 是後台貼文清單的 page token 內容：(CreatedAt, ID)。
type adminPostCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c adminPostCursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

func (h *Admin) ListPosts(ctx context.Context, req *connect.Request[adminpb.ListPostsRequest]) (*connect.Response[adminpb.ListPostsResponse], error) {
	msg := req.Msg
	filter := social.AdminPostFilter{TakenDown: msg.GetTakenDownOnly()}
	scope := rpc.FilterScope(socialv1alpha1connect.SocialAdminServiceListPostsProcedure, fmt.Sprint(filter.TakenDown))
	after, err := rpc.ValidCursor[adminPostCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var cur *social.AdminPostCursor
	if after != nil {
		cur = &social.AdminPostCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	page, err := h.posts.ListAdminPosts(ctx, filter, cur, msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(page.Posts))
	for i, p := range page.Posts {
		ids[i] = p.ID
	}
	counts, err := h.reports.OpenReportCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListPostsResponse{Posts: make([]*adminpb.AdminPost, len(page.Posts))}
	for i, p := range page.Posts {
		item := &adminpb.AdminPost{
			Id:              p.ID.String(),
			StoreId:         rpc.FormatID(p.StoreID),
			Body:            p.Body,
			Status:          rpc.EnumToProto[pb.PostStatus](p.Status),
			CreateTime:      timestamppb.New(p.CreatedAt),
			UpdateTime:      timestamppb.New(p.UpdatedAt),
			OpenReportCount: counts[p.ID],
		}
		if p.PublishedAt != nil {
			item.PublishTime = timestamppb.New(*p.PublishedAt)
		}
		if p.TakedownMeasureID != nil {
			id := p.TakedownMeasureID.String()
			item.TakedownMeasureId = &id
		}
		out.Posts[i] = item
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, adminPostCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}
