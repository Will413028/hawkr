package grpc

import (
	"context"
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

// CreateInvoiceCorrection 是 §6.1 端點 11（B06 買家申請修改發票資料）。
//
// 🔴 **這支在 2026-08-31 之前完全不存在**：proto 有宣告、gateway 有呼叫、
// invoice-service 零實作 ⇒ 那條路由打下去必定回 Unimplemented。#2909 的起點。
//
// 授權述詞照 §8.1.4(b)：`order_no` 缺席**不是「不篩」而是契約破了**——它是三段擁有者
// 述詞之一，而那一段正是防「ADM 事後改過買受人」的。真正的比對在
// DocumentRepository.GetForBuyer 的 SQL 述詞內，本層只負責不讓空值溜過去。
func (s *InvoiceServer) CreateInvoiceCorrection(
	ctx context.Context, req *pb.CreateInvoiceCorrectionRequest,
) (*pb.InvoiceCorrectionResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	if req.GetOrderNo() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order_no: %v", errEmptyID)
	}
	expected, err := strconv.ParseInt(req.GetExpectedSnapshotVersion(), 10, 32)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_snapshot_version: %v", err)
	}
	if req.GetIdempotencyKey() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "idempotency_key: %v", errEmptyID)
	}

	out, err := s.correctionIntake.Submit(ctx, usecase.SubmitCorrectionRequest{
		BuyerID:                 buyerID,
		OrderNo:                 req.GetOrderNo(),
		ExpectedSnapshotVersion: int32(expected),
		NewInvoice:              toCheckoutInvoiceSpec(req.GetNewInvoice()),
		Reason:                  req.GetReason(),
		IdempotencyKey:          req.GetIdempotencyKey(),
	})
	if err != nil {
		return s.correctionError(ctx, err)
	}

	return &pb.InvoiceCorrectionResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.InvoiceCorrectionData{
			CorrectionId:     fmtID(out.CorrectionID),
			HandlingMode:     out.HandlingMode,
			Status:           out.Status,
			StatusNotice:     usecase.CorrectionStatusNotices[out.Status],
			RejectReasonCode: out.RejectReason,
		},
	}, nil
}

func (s *InvoiceServer) correctionError(ctx context.Context, err error) (*pb.InvoiceCorrectionResponse, error) {
	switch {
	case errors.Is(err, usecase.ErrCorrectionAlreadyInProgress):
		return &pb.InvoiceCorrectionResponse{
			Code:    bizcode.InvoiceOperationInProgress,
			Message: "此發票有處理中的調整，請稍後再試",
		}, nil
	case errors.Is(err, usecase.ErrCorrectionSnapshotStale):
		return &pb.InvoiceCorrectionResponse{
			Code:    bizcode.InvoiceSnapshotVersionConflict,
			Message: "發票資料已被更新，請重新整理後再試",
		}, nil
	case errors.Is(err, usecase.ErrCorrectionDocumentInProgress):
		return &pb.InvoiceCorrectionResponse{
			Code:    bizcode.InvoiceOperationInProgress,
			Message: "此發票正在處理中，請稍後再試",
		}, nil
	case errors.Is(err, usecase.ErrCorrectionSendInFlight):
		return &pb.InvoiceCorrectionResponse{
			Code:    bizcode.InvoiceOperationInProgress,
			Message: "此發票正在開立中，請稍後再試",
		}, nil
	}
	if code, msg, ok := bizOf(err); ok {
		return &pb.InvoiceCorrectionResponse{Code: code, Message: msg}, nil
	}
	return nil, s.internal(ctx, "invoice.correction.create.failed", err)
}

func toCheckoutInvoiceSpec(s *pb.InvoiceInputSpec) usecase.CheckoutInvoiceSpec {
	if s == nil {
		return usecase.CheckoutInvoiceSpec{}
	}
	spec := usecase.CheckoutInvoiceSpec{
		Present:       true,
		CarrierType:   s.GetCarrierType(),
		Value:         s.GetValue(),
		BuyerTaxID:    s.GetBuyerTaxId(),
		BuyerName:     s.GetBuyerName(),
		Email:         s.GetEmail(),
		SaveAsDefault: s.GetSaveAsDefault(),
	}
	if id, err := parseID(s.GetProfileId()); err == nil {
		spec.ProfileID = id
	}
	return spec
}
