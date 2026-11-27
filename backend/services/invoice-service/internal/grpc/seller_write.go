package grpc

import (
	"context"
	"errors"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


// RespondSellerInvoiceComplaint 是賣家回覆申訴（端點 36）。
//
// action 三值：RESENT／REISSUED 走 K02（→ SELLER_RESPONDED），NEED_PLATFORM_HELP 走
// K03（→ ESCALATED_TO_ADM，在賣家側的 S09 歸入「已回覆」分頁）。
// 失敗：狀態不允許回覆 → 9336；越權 → 9309（與「不存在」逐位元相同）。
func (s *InvoiceServer) RespondSellerInvoiceComplaint(
	ctx context.Context, req *pb.RespondSellerInvoiceComplaintRequest,
) (*pb.InvoiceComplaintResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	complaintID, err := parseID(req.GetComplaintId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "complaint_id: %v", err)
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}

	view, err := s.sellerComplaints.Respond(ctx, usecase.RespondComplaintInput{
		StoreID:         storeID,
		ComplaintID:     complaintID,
		Action:          req.GetAction(),
		Note:            req.GetNote(),
		ExpectedVersion: version,
		IdempotencyKey:  req.GetIdempotencyKey(),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrComplaintActionInvalid) {
			return nil, status.Errorf(codes.InvalidArgument, "action: %v", err)
		}
		if st, ok := complaintInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceComplaintResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.complaint.seller_respond.failed", err)
	}
	info := toComplaintInfo(view)
	s.fillOneBuyerName(ctx, info, view)
	return &pb.InvoiceComplaintResponse{Code: 0, Message: msgSuccess, Data: info}, nil
}

// ReportExternalIssuance 是賣家回報自行開立的發票號碼（端點 62 ＝ J30）。
//
// 🔴 號碼格式不合回 **400 而不是 9349**：9349 的語意是「本作業尚未開放自行開立回報」，
// 賣家的下一步是等 ADM 解鎖；格式錯的下一步是把號碼重打一次。兩者混成一種，賣家會
// 一直等一個永遠不會來的解鎖。
func (s *InvoiceServer) ReportExternalIssuance(
	ctx context.Context, req *pb.ReportExternalIssuanceRequest,
) (*pb.GetSellerInvoiceJobResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	jobID, err := parseID(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "job_id: %v", err)
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}

	view, err := s.sellerJobs.ReportExternalIssuance(ctx, usecase.ReportExternalIssuanceInput{
		StoreID:           storeID,
		JobID:             jobID,
		InvoiceNo:         req.GetInvoiceNumber(),
		InvoiceDate:       req.GetInvoiceDate(),
		EvidenceAssetKeys: req.GetEvidenceAssetKeys(),
		Note:              req.GetNote(),
		ExpectedVersion:   version,
		IdempotencyKey:    req.GetIdempotencyKey(),
	})
	if err != nil {
		if st, ok := externalIssuanceInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetSellerInvoiceJobResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.seller.external_issuance.failed", err)
	}
	return &pb.GetSellerInvoiceJobResponse{Code: 0, Message: msgSuccess, Data: s.toJobInfoView(view)}, nil
}

func externalIssuanceInvalidArgument(err error) (error, bool) {
	switch {
	case errors.Is(err, usecase.ErrExternalInvoiceNoInvalid),
		errors.Is(err, usecase.ErrExternalInvoiceDateInvalid),
		errors.Is(err, usecase.ErrExternalInvoiceDateTooEarly),
		errors.Is(err, usecase.ErrJobNoteTooLong),
		errors.Is(err, usecase.ErrExternalEvidenceTooMany),
		errors.Is(err, usecase.ErrExternalEvidenceInvalid):
		return status.Errorf(codes.InvalidArgument, "%v", err), true
	}
	return nil, false
}
