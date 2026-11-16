package social

import (
	"context"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// ListPendingSchedules 是店主看自己店待發布的排程（含代行排的）。店家由 seller.CurrentStore 推導，只讀這家店的排程。
func (h *App) ListPendingSchedules(ctx context.Context, req *connect.Request[pb.ListPendingSchedulesRequest]) (*connect.Response[pb.ListPendingSchedulesResponse], error) {
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(socialv1alpha1connect.SocialAppServiceListPendingSchedulesProcedure, strconv.FormatInt(st.StoreID, 10))
	after, err := rpc.ValidCursor[scheduleCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var cur *social.ScheduleCursor
	if after != nil {
		cur = &social.ScheduleCursor{PublishAt: after.PublishAt, ID: after.ID}
	}
	page, err := h.owner.ListPendingSchedules(ctx, st.StoreID, cur, req.Msg.GetPageSize())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, rpc.PageSizePaths)
	}
	out := &pb.ListPendingSchedulesResponse{Schedules: make([]*pb.PostSchedule, len(page.Schedules))}
	for i, s := range page.Schedules {
		out.Schedules[i] = &pb.PostSchedule{
			Id: s.ID.String(), PostId: s.PostID.String(), PublishTime: timestamppb.New(s.PublishAt),
			ScheduledBy: rpc.EnumToProto[pb.ScheduleKind](s.By.Kind), CreateTime: timestamppb.New(s.CreatedAt),
		}
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, scheduleCursor{PublishAt: page.Next.PublishAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

// CancelPostSchedule 是店主取消貼文待發布的排程（不分發起者）。
func (h *App) CancelPostSchedule(ctx context.Context, req *connect.Request[pb.CancelPostScheduleRequest]) (*connect.Response[pb.CancelPostScheduleResponse], error) {
	id, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	next, err := h.owner.CancelPostSchedule(ctx, st, id, version)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CancelPostScheduleResponse{Etag: rpc.Etag(next)}), nil
}
