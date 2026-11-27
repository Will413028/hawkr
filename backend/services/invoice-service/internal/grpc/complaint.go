package grpc

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


// CreateInvoiceComplaint 建立一件申訴（端點 13 ＝ K01）。
func (s *InvoiceServer) CreateInvoiceComplaint(
	ctx context.Context, req *pb.CreateInvoiceComplaintRequest,
) (*pb.InvoiceComplaintResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	if req.GetOrderNo() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_no: must not be empty")
	}

	res, err := s.complaints.File(ctx, usecase.FileComplaintInput{
		BuyerID:     buyerID,
		OrderNo:     req.GetOrderNo(),
		ReasonCode:  req.GetReasonCode(),
		Description: req.GetDescription(),
	})
	if err != nil {
		if st, ok := complaintInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceComplaintResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.complaint.create.failed", err)
	}
	if res.Duplicate {
		return &pb.InvoiceComplaintResponse{
			Code:    bizcode.InvoiceComplaintDuplicate,
			Message: bizcode.Message(bizcode.InvoiceComplaintDuplicate),
			Data:    toBuyerComplaintInfo(res.View),
		}, nil
	}
	return &pb.InvoiceComplaintResponse{Code: 0, Message: msgSuccess, Data: toBuyerComplaintInfo(res.View)}, nil
}

// RejectInvoiceComplaintResponse 是買家不接受賣家的回覆（端點 63 ＝ K07）。
func (s *InvoiceServer) RejectInvoiceComplaintResponse(
	ctx context.Context, req *pb.RejectInvoiceComplaintResponseRequest,
) (*pb.InvoiceComplaintResponse, error) {
	in, err := complaintActionInput(req.GetBuyerId(), req.GetOrderNo(), req.GetComplaintId(), req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	in.Reason = req.GetReason()
	return s.complaintAction(ctx, "invoice.complaint.reject.failed", func() (usecase.ComplaintView, error) {
		return s.complaints.Reject(ctx, in)
	})
}

// WithdrawInvoiceComplaint 是買家撤回（端點 64 ＝ K11）。
//
// 沒有 reason：§7 K11 沒有必填理由的守衛，端點層不自行加嚴。
func (s *InvoiceServer) WithdrawInvoiceComplaint(
	ctx context.Context, req *pb.WithdrawInvoiceComplaintRequest,
) (*pb.InvoiceComplaintResponse, error) {
	in, err := complaintActionInput(req.GetBuyerId(), req.GetOrderNo(), req.GetComplaintId(), req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	return s.complaintAction(ctx, "invoice.complaint.withdraw.failed", func() (usecase.ComplaintView, error) {
		return s.complaints.Withdraw(ctx, in)
	})
}

// ReopenInvoiceComplaint 是買家重開（端點 65 ＝ K13）。
func (s *InvoiceServer) ReopenInvoiceComplaint(
	ctx context.Context, req *pb.ReopenInvoiceComplaintRequest,
) (*pb.InvoiceComplaintResponse, error) {
	in, err := complaintActionInput(req.GetBuyerId(), req.GetOrderNo(), req.GetComplaintId(), req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	in.Reason = req.GetReason()
	return s.complaintAction(ctx, "invoice.complaint.reopen.failed", func() (usecase.ComplaintView, error) {
		return s.complaints.Reopen(ctx, in)
	})
}

func complaintActionInput(buyerID int64, orderNo, complaintID, expectedVersion string) (usecase.ComplaintActionInput, error) {
	principal, err := requirePrincipal(buyerID, "buyer_id")
	if err != nil {
		return usecase.ComplaintActionInput{}, err
	}
	if orderNo == "" {
		return usecase.ComplaintActionInput{}, status.Error(codes.InvalidArgument, "order_no: must not be empty")
	}
	id, err := parseID(complaintID)
	if err != nil {
		return usecase.ComplaintActionInput{}, status.Errorf(codes.InvalidArgument, "complaint_id: %v", err)
	}
	version, err := parseVersion(expectedVersion)
	if err != nil {
		return usecase.ComplaintActionInput{}, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}
	return usecase.ComplaintActionInput{
		BuyerID:         principal,
		OrderNo:         orderNo,
		ComplaintID:     id,
		ExpectedVersion: version,
	}, nil
}

func (s *InvoiceServer) complaintAction(
	ctx context.Context, event string, run func() (usecase.ComplaintView, error),
) (*pb.InvoiceComplaintResponse, error) {
	view, err := run()
	if err != nil {
		if st, ok := complaintInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceComplaintResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, event, err)
	}
	return &pb.InvoiceComplaintResponse{Code: 0, Message: msgSuccess, Data: toBuyerComplaintInfo(view)}, nil
}

func complaintInvalidArgument(err error) (error, bool) {
	switch {
	case errors.Is(err, usecase.ErrComplaintDescriptionTooLong),
		errors.Is(err, usecase.ErrComplaintReasonRequired),
		errors.Is(err, usecase.ErrComplaintReasonTooLong),
		errors.Is(err, usecase.ErrComplaintReasonCodeInvalid):
		return status.Errorf(codes.InvalidArgument, "%v", err), true
	}
	return nil, false
}
