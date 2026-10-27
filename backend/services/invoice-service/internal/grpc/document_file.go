package grpc

import (
	"context"
	"errors"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

// GetInvoiceDocumentFile 是 §6.1.4 端點 10／70 的中介資料面。
//
// 🔴 **這支在 2026-09-02 之前完全不存在**：proto 有宣告、gateway 兩支 handler 都會
// 呼叫它（invoice_buyer.go 的 GetOrderInvoiceDocumentFile 與買家發票中心那支），
// 而 internal/grpc 零實作 ⇒ 由 pb.UnimplementedInvoiceServiceServer 承接 ⇒
// codes.Unimplemented ⇒ gateway 映射成 503「invoice service unavailable」。
// 那正是 #3008 實測到的第二條失敗路徑（第一條是 10042 一律 501）。
//
// 🔴 **它不回 bytes，也不回任何可直接取檔的 URL**（§6.1.4 逐字）：
// 回的是 invoiceFileKey（我方 media objectKey）＋ fileName ＋ style。
// AMEGO 的 file_url 絕不外流——那個 token 不需認證即可下載且可重複使用，
// 本身就是一張 bearer 憑證。
//
// 授權述詞（recipient_ref_id 必須是這位買家）寫在 repository 的 SQL 裡，
// 越權與查無列走同一個出口（9301）——見 usecase/document_file.go 檔頭。
func (s *InvoiceServer) GetInvoiceDocumentFile(
	ctx context.Context, req *pb.GetInvoiceDocumentFileRequest,
) (*pb.GetInvoiceDocumentFileResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}

	res, err := s.files.Resolve(ctx, usecase.FileRequest{
		Actor:      usecase.FileActorBuyer,
		DocumentID: documentID,
		BuyerID:    buyerID,
		Style:      req.GetStyle(),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidFileStyle) {
			return nil, status.Errorf(codes.InvalidArgument, "style: %v", err)
		}
		// 9301／9312／9313 走 envelope（HTTP 仍是 200，gateway 依碼映射），
		// 其餘走 internal。**不得把 9312 當錯誤**：載具發票中獎前沒有 PDF 是
		// 正常業務分支，前端據它不渲染下載按鈕（§6.1.4 鐵則 4）。
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetInvoiceDocumentFileResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.document_file.resolve.failed", err)
	}

	return &pb.GetInvoiceDocumentFileResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.InvoiceFileData{
			DocumentId:     fmtID(res.DocumentID),
			InvoiceFileKey: res.ObjectKey,
			FileName:       res.FileName,
			Style:          res.Style,
		},
	}, nil
}
