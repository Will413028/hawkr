package grpc

import (
	"context"
	"errors"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔是兩支賣家寫入端點的 Layer 5：
//
//	端點 36 POST /app/seller/invoices/complaints/{id}/responses   （S10 回覆申訴，K02／K03）
//	端點 62 POST /app/seller/invoices/jobs/{id}/external_issuances（S07 回報自行開立，J30）
//
// 🔴 兩支都是**停權賣家的放行路徑**（§3 D16 ＋ §6.1.11）。放行由 Gateway 的能力層決定，
// 本層不重複判斷——但也不得自行加嚴。加嚴的後果分別是：
//   - 端點 36：因為停權所以無法回覆，因為無法回覆所以繼續累積違規（死結）
//   - 端點 62：ADM 雙人解鎖完成、賣家自行開了票，然後沒有地方回報 ⇒ operation 永遠停在
//     非終態，買家端永遠顯示「處理中」，法定期限照樣倒數

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
	// 一律回**轉移後**的完整單筆（§6.1.6 共通形狀）：前端據此直接覆蓋 query cache，
	// 不必再打一次端點 35；少了它，兩次請求之間的窗口會讓畫面顯示轉移前的狀態。
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

// externalIssuanceInvalidArgument 把端點 62 的型別邊界錯誤映成 400。
//
// 四個號碼／日期／金額類錯誤與端點 56 共用（同一組 usecase 變數）：兩支端點對同一種
// 輸入錯誤必須回同一件事，否則賣家與 ADM 會看到互相矛盾的說法。
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
