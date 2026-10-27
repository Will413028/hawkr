package grpc

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔是「重新寄送發票通知」的三個入口（端點 32 賣家／49 ADM／71 買家）的 Layer 5。
//
// 三支回**同一個** proto message，因為它們是同一件事的三個身分入口。差別只有兩處：
// 身分欄（store_id／buyer_id／actor_account_id）與歸屬述詞，而後者在 usecase／SQL 裡。
//
// 🔴 **9352 走 codes.ResourceExhausted**（§6.1.10 逐字）而不是 envelope 的非零碼：
// 它要對應到 HTTP 429，而 Gateway 的 `envelopeCodeToGRPC` 是以 gRPC status 判定的。
// 回成 `Code: 9352` 的成功回應會變成 HTTP 200，前端的「請稍候再試」永遠不會出現。
// bizOf 走的是 DomainError → envelope 那條路，所以本檔必須自己攔 9352。

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

// resendResponse 是三支共用的錯誤分流與回應組裝。
func (s *InvoiceServer) resendResponse(
	ctx context.Context, event string, res usecase.ResendResult, err error,
) (*pb.ResendInvoiceNotificationResponse, error) {
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			// 9352 → ResourceExhausted（HTTP 429）。message 帶 `[9352] ` 前綴讓
			// Gateway 的 envelopeCodeToGRPC 認得出是哪一個業務碼。
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
			Resent: res.Resent,
			SentAt: fmtTimeValue(res.SentAt),
			// 沒有 email 的文件（載具發票／收據）回空字串——誠實的「沒有寄件對象」，
			// 不是遮罩失敗。
			ToMasked: res.ToMasked,
		},
	}, nil
}
