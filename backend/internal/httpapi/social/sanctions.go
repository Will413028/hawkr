package social

import (
	"context"

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

func (h *App) ListMySanctions(ctx context.Context, req *connect.Request[pb.ListMySanctionsRequest]) (*connect.Response[pb.ListMySanctionsResponse], error) {
	buyerID, err := h.buyerID(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(socialv1alpha1connect.SocialAppServiceListMySanctionsProcedure, buyerID)
	after, err := rpc.ValidCursor[social.SanctionCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.authored.MySanctions(ctx, buyerID, after, req.Msg.GetPageSize())
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
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *App) ListMyHiddenContent(ctx context.Context, req *connect.Request[pb.ListMyHiddenContentRequest]) (*connect.Response[pb.ListMyHiddenContentResponse], error) {
	buyerID, err := h.buyerID(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(socialv1alpha1connect.SocialAppServiceListMyHiddenContentProcedure, buyerID)
	after, err := rpc.ValidCursor[social.HiddenCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.authored.MyHiddenContent(ctx, buyerID, after, req.Msg.GetPageSize())
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
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}
