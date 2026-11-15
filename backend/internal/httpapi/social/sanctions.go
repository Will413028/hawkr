package social

import (
	"context"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// Authored 是作者讀自己被處分與被隱藏的內容的來源（*social.Module，A7-D9）。兩個方法都只讀，作者本人以買家身分呼叫。
type Authored interface {
	MySanctions(ctx context.Context, buyerID int64, after *social.SanctionCursor, size int32) (social.SanctionPage, error)
	MyHiddenContent(ctx context.Context, buyerID int64, after *social.HiddenCursor, size int32) (social.HiddenPage, error)
}

type sanctionCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}

func (c sanctionCursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

type hiddenCursor struct {
	HiddenAt time.Time `json:"h"`
	ID       uuid.UUID `json:"i"`
}

func (c hiddenCursor) Valid() bool { return !c.HiddenAt.IsZero() && c.ID != uuid.UUID{} }

func (h *App) ListMySanctions(ctx context.Context, req *connect.Request[pb.ListMySanctionsRequest]) (*connect.Response[pb.ListMySanctionsResponse], error) {
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(socialv1alpha1connect.SocialAppServiceListMySanctionsProcedure, c.BuyerID)
	after, err := rpc.ValidCursor[sanctionCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var cur *social.SanctionCursor
	if after != nil {
		cur = &social.SanctionCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	page, err := h.writes.MySanctions(ctx, c.BuyerID, cur, req.Msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	out := &pb.ListMySanctionsResponse{Sanctions: make([]*pb.Sanction, len(page.Sanctions))}
	for i, s := range page.Sanctions {
		out.Sanctions[i] = &pb.Sanction{
			Id:         s.ID.String(),
			Kind:       rpc.EnumToProto[pb.SanctionKind](s.Kind),
			Category:   rpc.EnumToProto[commonpb.PolicyCategory](s.Category),
			Reason:     s.Text,
			CreateTime: timestamppb.New(s.CreatedAt),
		}
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, sanctionCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

func (h *App) ListMyHiddenContent(ctx context.Context, req *connect.Request[pb.ListMyHiddenContentRequest]) (*connect.Response[pb.ListMyHiddenContentResponse], error) {
	c, err := h.account(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(socialv1alpha1connect.SocialAppServiceListMyHiddenContentProcedure, c.BuyerID)
	after, err := rpc.ValidCursor[hiddenCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var cur *social.HiddenCursor
	if after != nil {
		cur = &social.HiddenCursor{HiddenAt: after.HiddenAt, HideID: after.ID}
	}
	page, err := h.writes.MyHiddenContent(ctx, c.BuyerID, cur, req.Msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	out := &pb.ListMyHiddenContentResponse{HiddenContents: make([]*pb.HiddenContent, len(page.Items))}
	for i, it := range page.Items {
		row := &pb.HiddenContent{
			Id:         it.HideID.String(),
			Kind:       rpc.EnumToProto[pb.HiddenContentKind](it.Target.Kind),
			ContentId:  it.Target.ID.String(),
			Body:       it.Body,
			CreateTime: timestamppb.New(it.CreatedAt),
			Category:   rpc.EnumToProto[commonpb.PolicyCategory](it.Category),
			Reason:     it.Text,
			HideTime:   timestamppb.New(it.HiddenAt),
		}
		if it.PostID != nil {
			p := it.PostID.String()
			row.PostId = &p
		}
		if it.StoreID != nil {
			s := rpc.FormatID(*it.StoreID)
			row.StoreId = &s
		}
		out.HiddenContents[i] = row
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, hiddenCursor{HiddenAt: page.Next.HiddenAt, ID: page.Next.HideID})
	}
	return connect.NewResponse(out), nil
}
