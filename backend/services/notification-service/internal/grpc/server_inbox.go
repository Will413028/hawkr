package grpc

import (
	"context"
	"fmt"
	"strconv"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

func (s *Server) ListNotifications(ctx context.Context, req *pb.ListNotificationsRequest) (*pb.ListNotificationsResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.ListNotificationsResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
	}
	cursorID, _ := strconv.ParseInt(req.CursorId, 10, 64)

	result, err := s.inboxUC.ListNotifications(ctx, userID, req.UserType, cursorID, req.PageSize, req.Category)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ListNotificationsResponse{Code: int32(code)}, nil
		}
		return nil, err
	}

	records := make([]*pb.NotificationItem, 0, len(result.Records))
	for _, r := range result.Records {
		item := &pb.NotificationItem{
			Id:          strconv.FormatInt(r.ID, 10),
			EventType:   r.EventType,
			Title:       r.Title,
			Body:        r.Body,
			ReferenceId: r.ReferenceID,
			IsRead:      r.IsRead,
			CreatedAt:   r.CreatedAt,
			Data:        r.Data,
		}
		if r.ReadAt != nil {
			item.ReadAt = *r.ReadAt
		}
		records = append(records, item)
	}

	// 跟 s.inboxUC.ListNotifications 內部實際用來查詢的值同一個計算方式，
	// 否則回應的 PageSize 欄位可能跟真正查詢用的值對不上（例如 PageSize
	// 超過上限時，查詢端會 clamp，這裡如果只做 <1 判斷就會回顯原始超量值）。
	pageSize := pagination.NormalizePageSize(req.PageSize, usecase.DefaultInboxPageSize, usecase.MaxInboxPageSize)

	return &pb.ListNotificationsResponse{
		Code:     0,
		Records:  records,
		Total:    fmt.Sprintf("%d", result.Total),
		PageSize: fmt.Sprintf("%d", pageSize),
		HasMore:  result.HasMore,
	}, nil
}

func (s *Server) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.GetUnreadCountResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
	}
	count, err := s.inboxUC.GetUnreadCount(ctx, userID, req.UserType)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.GetUnreadCountResponse{Code: int32(code)}, nil
		}
		return nil, err
	}
	return &pb.GetUnreadCountResponse{Code: 0, Count: strconv.FormatInt(count, 10)}, nil
}

func (s *Server) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pb.MarkAsReadResponse, error) {
	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil {
		return &pb.MarkAsReadResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid id"}, nil
	}
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.MarkAsReadResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid user_id"}, nil
	}
	if err := s.inboxUC.MarkAsRead(ctx, id, userID, req.UserType); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.MarkAsReadResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.MarkAsReadResponse{Code: 0, Message: "ok"}, nil
}

func (s *Server) MarkAllAsRead(ctx context.Context, req *pb.MarkAllAsReadRequest) (*pb.MarkAllAsReadResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.MarkAllAsReadResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
	}
	updated, err := s.inboxUC.MarkAllAsRead(ctx, userID, req.UserType)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.MarkAllAsReadResponse{Code: int32(code)}, nil
		}
		return nil, err
	}
	return &pb.MarkAllAsReadResponse{Code: 0, Updated: strconv.FormatInt(updated, 10)}, nil
}

func (s *Server) DeleteNotifications(ctx context.Context, req *pb.DeleteNotificationsRequest) (*pb.DeleteNotificationsResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.DeleteNotificationsResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
	}
	ids := make([]int64, 0, len(req.Ids))
	for _, s := range req.Ids {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return &pb.DeleteNotificationsResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
		}
		ids = append(ids, id)
	}
	deleted, err := s.inboxUC.DeleteNotifications(ctx, ids, userID, req.UserType)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.DeleteNotificationsResponse{Code: int32(code)}, nil
		}
		return nil, err
	}
	return &pb.DeleteNotificationsResponse{Code: 0, Deleted: strconv.FormatInt(deleted, 10)}, nil
}
