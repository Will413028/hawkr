package grpc

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ResendInvoiceNotification 是端點 32（賣家重新寄送）。
func (s *InvoiceServer) ResendInvoiceNotification(
	ctx context.Context, req *pb.ResendInvoiceNotificationRequest,
) (*pb.ResendInvoiceNotificationResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}
	res, err := s.resends.ResendForSeller(ctx, storeID, documentID)
	return s.resendResponse(ctx, "invoice.seller.notification_resend.failed", res, err)
}

// ResendBuyerInvoiceNotification 是端點 71（買家重新寄送）。
//
// 它與賣家版**必須是兩支 RPC**：買家版的請求只有 buyer_id，而賣家版只有 store_id ——
// 共用一支的話 Gateway 送不出買家身分，§8.1.4 的 Tier A 擁有者述詞在服務端建不起來
// （＝任何人都能觸發任何人的重寄）。
func (s *InvoiceServer) ResendBuyerInvoiceNotification(
	ctx context.Context, req *pb.ResendBuyerInvoiceNotificationRequest,
) (*pb.ResendInvoiceNotificationResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}
	res, err := s.resends.ResendForBuyer(ctx, buyerID, documentID)
	return s.resendResponse(ctx, "invoice.buyer.notification_resend.failed", res, err)
}

// ResendInvoiceNotificationForAdmin 是端點 49（ADM 重新寄送）。
//
// 與端點 32 **共用同一個 10 分鐘計數器**（key ＝ document_id，不是 document_id × actor）：
// 否則 ADM 幫忙點一次、賣家再點一次，買家會在十分鐘內收到兩封。
func (s *InvoiceServer) ResendInvoiceNotificationForAdmin(
	ctx context.Context, req *pb.ResendInvoiceNotificationForAdminRequest,
) (*pb.ResendInvoiceNotificationResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}
	res, err := s.resends.ResendForAdmin(ctx, documentID)
	return s.resendResponse(ctx, "invoice.admin.notification_resend.failed", res, err)
}

func (s *InvoiceServer) resendResponse(
	ctx context.Context, event string, res usecase.ResendResult, err error,
) (*pb.ResendInvoiceNotificationResponse, error) {
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			if code == bizcode.InvoiceNotificationResendTooFrequent {
				return nil, status.Errorf(codes.ResourceExhausted, "[%d] %s", code, msg)
			}
			return &pb.ResendInvoiceNotificationResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, event, err)
	}
	return &pb.ResendInvoiceNotificationResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ResendInvoiceNotificationData{
			Resent:   res.Resent,
			SentAt:   fmtTimeValue(res.SentAt),
			ToMasked: res.ToMasked,
		},
	}, nil
}
