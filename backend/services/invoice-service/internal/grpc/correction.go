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
		// 一路傳到 ORD.ReviseOrderInvoiceSnapshotInternal 當冪等鍵；order-service 側
		// 的 partial unique index 對 NULL 不生效 ⇒ 空的會讓每次重送變成一次新的更正。
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
			CorrectionId: fmtID(out.CorrectionID),
			HandlingMode: out.HandlingMode,
			Status:       out.Status,
			// #3031：端點 11 的回應就是 B06「成功 toast」的來源（58A §B06 逐字：
			// statusNotice → toast 的 body）。兩個生產者共用同一張表，
			// 不在這裡另寫一句——兩份文案必然漂移。
			StatusNotice:     usecase.CorrectionStatusNotices[out.Status],
			RejectReasonCode: out.RejectReason,
		},
	}, nil
}

// correctionError 把三種可辨識失敗折成業務碼，其餘走既有的 bizOf／internal。
//
// 🔴 **不要全塞 internal**：這三種買家自己就能處理（稍後再送／重新載入／稍後再送），
// 而 internal 會讓 B06 只顯示「系統忙碌」——那句話對他沒有任何下一步。
func (s *InvoiceServer) correctionError(ctx context.Context, err error) (*pb.InvoiceCorrectionResponse, error) {
	switch {
	case errors.Is(err, usecase.ErrCorrectionAlreadyInProgress):
		// 序列閘門（I5）擋下：這張發票已有進行中的修改請求。
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
		// 憑證層：document 在 ISSUING／VOIDING。**不是**婉拒——買家等幾秒再送就能受理。
		// 2026-09-02 之前這一格會落到 decideCorrectionRoute 的 fallback，
		// 回 REJECTED ＋ PAST_VOID_DEADLINE（#3020）。
		return &pb.InvoiceCorrectionResponse{
			Code:    bizcode.InvoiceOperationInProgress,
			Message: "此發票正在處理中，請稍後再試",
		}, nil
	case errors.Is(err, usecase.ErrCorrectionSendInFlight):
		// R03 來不及（operation 已 SENDING）、R04 又還不適用（document 尚未 ISSUED）。
		// §7.3.5 沒有這一格；買家稍後再送就會落到其中一邊。
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

// toCheckoutInvoiceSpec 把 proto 的買家發票輸入折成 usecase 的形狀。
//
// 沿用 checkout 的型別而不是另立一份：那是本服務唯一一份買家發票輸入的形狀，
// 兩份會讓驗證規則漂移（而漂移的症狀是「常用資料存得進去，結帳時被擋」）。
//
// ⚠️ disclosure 兩欄已於 2026-08-31 從 proto 移除（DEC-20260831-210934），
// 這裡不再有對應欄位。
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
