package grpc

import (
	"context"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/go-pkg/timeutil"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// Admin 手動停權（不服勸導，32 §13 / 44 §13.2）gRPC handlers。同 governance_handler.go
// 「不回 grpc.Status，回 proto code field」風格。時間欄位依 §6 走 ISO 8601 UTC 字串
// （timeutil.FormatISO8601），非 ms 數字。

func toProtoSellerSuspension(s *domain.SellerSuspension) *pb.SellerSuspension {
	if s == nil {
		return nil
	}
	return &pb.SellerSuspension{
		Id:        formatID(s.ID),
		AccountId: formatID(s.AccountID),
		// issue #1534：DB/domain 已直接存 CodeMeta Value 數字碼（"9601"），原樣
		// 輸出；#1514 的輸出邊界 Name→Value 轉換已移除。
		ReasonCode:         s.ReasonCode,
		ReasonDetail:       s.ReasonDetail,
		Status:             string(s.Status),
		SuspendedByAdminId: formatID(s.SuspendedByAdminID),
		SuspendedAt:        timeutil.FormatISO8601(s.SuspendedAt),
		LiftedByAdminId:    optID(s.LiftedByAdminID),
		LiftedAt:           timeutil.FormatISO8601Ptr(s.LiftedAt),
		CreatedAt:          timeutil.FormatISO8601(s.CreatedAt),
	}
}

func (s *SellerServer) SuspendSellerForAdmin(ctx context.Context, req *pb.SuspendSellerForAdminRequest) (*pb.SuspendSellerForAdminResponse, error) {
	result, err := s.suspensionUC.Suspend(ctx, parseInt64(req.AccountId), req.ReasonCode, req.ReasonDetail, req.AdminId)
	if err != nil {
		c, m := errToCode(err)
		return &pb.SuspendSellerForAdminResponse{Code: c, Message: m}, nil
	}
	return &pb.SuspendSellerForAdminResponse{
		Suspension:       toProtoSellerSuspension(result.Suspension),
		AlreadySuspended: result.AlreadySuspended,
	}, nil
}

func (s *SellerServer) LiftSellerSuspensionForAdmin(ctx context.Context, req *pb.LiftSellerSuspensionForAdminRequest) (*pb.LiftSellerSuspensionForAdminResponse, error) {
	result, err := s.suspensionUC.Lift(ctx, parseInt64(req.AccountId), parseInt64(req.Id), req.AdminId)
	if err != nil {
		c, m := errToCode(err)
		return &pb.LiftSellerSuspensionForAdminResponse{Code: c, Message: m}, nil
	}
	return &pb.LiftSellerSuspensionForAdminResponse{
		Suspension:    toProtoSellerSuspension(result.Suspension),
		AlreadyLifted: result.AlreadyLifted,
	}, nil
}

func (s *SellerServer) ListSellerSuspensionsForAdmin(ctx context.Context, req *pb.ListSellerSuspensionsForAdminRequest) (*pb.ListSellerSuspensionsForAdminResponse, error) {
	suspensions, err := s.suspensionUC.List(ctx, parseInt64(req.AccountId), req.Limit, req.Offset)
	if err != nil {
		c, m := errToCode(err)
		return &pb.ListSellerSuspensionsForAdminResponse{Code: c, Message: m}, nil
	}
	out := make([]*pb.SellerSuspension, 0, len(suspensions))
	for _, sus := range suspensions {
		out = append(out, toProtoSellerSuspension(sus))
	}
	return &pb.ListSellerSuspensionsForAdminResponse{Suspensions: out}, nil
}

func (s *SellerServer) GetActiveSellerSuspensionForAdmin(ctx context.Context, req *pb.GetActiveSellerSuspensionForAdminRequest) (*pb.GetActiveSellerSuspensionForAdminResponse, error) {
	active, err := s.suspensionUC.GetActive(ctx, parseInt64(req.AccountId))
	if err != nil {
		c, m := errToCode(err)
		return &pb.GetActiveSellerSuspensionForAdminResponse{Code: c, Message: m}, nil
	}
	// active == nil (no active suspension) → Suspension left nil; toProtoSellerSuspension
	// already handles nil, and the Gateway/frontend distinguishes by an empty id.
	return &pb.GetActiveSellerSuspensionForAdminResponse{Suspension: toProtoSellerSuspension(active)}, nil
}
