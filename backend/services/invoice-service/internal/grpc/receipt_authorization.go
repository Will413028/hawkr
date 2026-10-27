package grpc

import (
	"context"
	"errors"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔是 S05 免用統一發票的收據代製授權（§6.1.9，端點 28 讀／29 寫）的 Layer 5。
//
// 兩支**必須回完全一樣的形狀**（同一個 proto message），所以只有一個 mapper。
// 各寫一份的後果是漂移：29 成功之後前端拿到的欄位與 28 不同，而畫面是同一頁。

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

// toReceiptAuthorizationData 折出 §6.1.9 的 fail-closed 三態。
//
// 🔴 `approval_basis_asset_id` 只在 available=true 時有值（usecase 已保證）：
// Gateway 用它組 approvalBasisDownloadPath，另外兩態回了就等於給賣家一個點下去
// 必定 404 的連結，而真正的問題（ADM 沒指名／指名錯人）沒有任何人看得到。
func toReceiptAuthorizationData(v usecase.ReceiptAuthorizationView) *pb.SellerReceiptAuthorizationData {
	return &pb.SellerReceiptAuthorizationData{
		Eligible:         v.Eligible,
		IneligibleReason: v.IneligibleReason,
		// approval_basis_key 是 SSOT（objectKey），即使 available=false 也照實回：
		// ADM 要能從回應看出「指名的是哪一份」才查得出為什麼上傳者不是本賣家。
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

// receiptInvalidArgument 把型別邊界錯誤映成 400。
func receiptInvalidArgument(err error) (error, bool) {
	switch {
	case errors.Is(err, usecase.ErrReceiptConsentRequired),
		errors.Is(err, usecase.ErrReceiptConsentTooLong):
		return status.Errorf(codes.InvalidArgument, "%v", err), true
	}
	return nil, false
}
