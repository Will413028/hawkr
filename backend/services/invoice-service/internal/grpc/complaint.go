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

// 本檔是 B07「未收到發票／收據申訴」寫入面的 Layer 5（§6.1.6 端點 13／63／64／65）。
//
// 三條規則：
//
//	① **業務錯誤走 envelope、故障走 gRPC error**。GetCode() 是 Gateway 的
//	   EnvelopeGuardInterceptor 唯一的判定依據。
//	② **Tier A 的回應必須與「不存在」逐位元相同**：四支帶 complaint_id 的 RPC 在越權
//	   與查無時都回同一個 9309 envelope，因為那個判定發生在 SQL 的 WHERE，本層沒有
//	   第二個分支可以回不同的東西。
//	③ **長度／必填是契約違規不是業務結果** ⇒ 回 InvalidArgument（400），不借業務碼：
//	   借一個近似的碼會讓前端把「你的字太長」顯示成別的意思。
//
// 🔴 9310（重複申訴）是**唯一一個帶 data 的非零碼**：HTTP 是 200、`data` 是既有那件
// 案件的完整單筆（§6.1.6 第 2 條）。少了 data，前端導不了頁而只能顯示一句紅字，
// 而買家什麼都沒做錯。

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
		// 9310 ＋ 既有案件的完整單筆。code 非零但 data 必須在（envelopeCodePreserveData）。
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

// complaintActionInput 解析三支動作端點的共通入參。
//
// expected_version **必填**（§6.1.6 共通形狀）：缺席就是契約破了，不是「用 0 試試看」——
// 0 會是一件從未被更新過的案件的合法版本，那時 CAS 會通過而買家看到的是三分鐘前那一版。
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

// complaintAction 是三支動作端點共用的錯誤分流與回應組裝。
//
// 一律回**轉移後**的單筆：前端據此直接覆蓋 query cache，不再打一次端點 15。
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

// complaintInvalidArgument 把 usecase 的四個型別邊界錯誤映成 400。
//
// 它們刻意不是 DomainError（usecase/complaint.go 的註解逐字）：沒有任何業務碼描述
// 「你的字太長」，而 bizOf 會把 DomainError 折成 envelope 的業務碼。
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
