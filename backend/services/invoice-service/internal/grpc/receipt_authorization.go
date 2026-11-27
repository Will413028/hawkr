package grpc

import (
	"context"
	"errors"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


// GetSellerReceiptAuthorization 是端點 28。
func (s *InvoiceServer) GetSellerReceiptAuthorization(
	ctx context.Context, req *pb.GetSellerReceiptAuthorizationRequest,
) (*pb.SellerReceiptAuthorizationResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	view, err := s.query.GetSellerReceiptAuthorization(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.SellerReceiptAuthorizationResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_receipt_authorization.failed", err)
	}
	return &pb.SellerReceiptAuthorizationResponse{
		Code: 0, Message: msgSuccess, Data: toReceiptAuthorizationData(view),
	}, nil
}

// ConfirmSellerReceiptAuthorization 是端點 29。
//
// authorized=false 是**取消授權**，是正常的業務結果（code 0）不是錯誤：
// 回 4xx 會讓賣家把「我剛剛取消了」看成「取消失敗」。
func (s *InvoiceServer) ConfirmSellerReceiptAuthorization(
	ctx context.Context, req *pb.ConfirmSellerReceiptAuthorizationRequest,
) (*pb.SellerReceiptAuthorizationResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}

	view, err := s.taxProfiles.ConfirmReceiptAuthorization(ctx, usecase.ConfirmReceiptAuthorizationInput{
		StoreID:         storeID,
		ConsentVersion:  req.GetConsentVersion(),
		Authorized:      req.GetAuthorized(),
		ExpectedVersion: version,
		IdempotencyKey:  req.GetIdempotencyKey(),
	})
	if err != nil {
		if st, ok := receiptInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.SellerReceiptAuthorizationResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.seller.receipt_authorization.confirm.failed", err)
	}
	return &pb.SellerReceiptAuthorizationResponse{
		Code: 0, Message: msgSuccess, Data: toReceiptAuthorizationData(view),
	}, nil
}

func toReceiptAuthorizationData(v usecase.ReceiptAuthorizationView) *pb.SellerReceiptAuthorizationData {
	return &pb.SellerReceiptAuthorizationData{
		Eligible:         v.Eligible,
		IneligibleReason: v.IneligibleReason,
		ApprovalBasisKey:               v.ApprovalBasisKey,
		ApprovedAt:                     fmtTime(v.ApprovedAt),
		ApprovalValidUntil:             fmtTime(v.ApprovalValidUntil),
		AuthorizedAt:                   fmtTime(v.AuthorizedAt),
		ConsentVersion:                 v.ConsentVersion,
		ReceiptTemplateKey:             v.ReceiptTemplateKey,
		Version:                        fmtInt64(v.Version),
		ApprovalBasisAssetId:           fmtIDPtr(v.ApprovalBasisAssetID),
		ApprovalBasisAvailable:         v.ApprovalBasisAvailable,
		ApprovalBasisUnavailableReason: v.ApprovalBasisUnavailableReason,
	}
}

func receiptInvalidArgument(err error) (error, bool) {
	switch {
	case errors.Is(err, usecase.ErrReceiptConsentRequired),
		errors.Is(err, usecase.ErrReceiptConsentTooLong):
		return status.Errorf(codes.InvalidArgument, "%v", err), true
	}
	return nil, false
}
