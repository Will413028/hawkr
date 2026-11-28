package grpc

import (
	"context"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/client/buyer"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
)

// BuyerNameResolver 是買家顯示名的出向面（實作在 client/buyer）。
//
// 介面宣告在消費端：本層只需要「給我一批 buyer_id、還我顯示名與註銷旗標」。
type BuyerNameResolver interface {
	BatchGetBuyerDisplay(ctx context.Context, buyerIDs []int64) (map[int64]buyer.Display, error)
}

func (s *InvoiceServer) resolveBuyerNames(ctx context.Context, buyerIDs []int64) map[int64]string {
	ids := dedupeIDs(buyerIDs)
	if s.buyerNames == nil || len(ids) == 0 {
		return map[int64]string{}
	}
	got, err := s.buyerNames.BatchGetBuyerDisplay(ctx, ids)
	if err != nil {
		if s.log != nil {
			s.log.Warn("invoice.query.buyer_names_unavailable",
				zap.Int("buyer_count", len(ids)),
				zap.String("degrade", "buyer_masked_name 退回空字串，申訴卡認不出人"),
				zap.Error(err))
		}
		return map[int64]string{}
	}
	out := make(map[int64]string, len(got))
	for id, d := range got {
		out[id] = usecase.MaskDisplayName(d.DisplayName, d.Deleted)
	}
	return out
}

func (s *InvoiceServer) fillBuyerNames(
	ctx context.Context, records []*pb.InvoiceComplaintInfo, views []usecase.ComplaintView,
) {
	if len(records) == 0 || len(records) != len(views) {
		return
	}
	ids := make([]int64, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.Complaint.ComplainantBuyerID)
	}
	names := s.resolveBuyerNames(ctx, ids)
	for i, rec := range records {
		if rec == nil {
			continue
		}
		rec.BuyerMaskedName = names[views[i].Complaint.ComplainantBuyerID]
	}
}

func (s *InvoiceServer) fillOneBuyerName(ctx context.Context, rec *pb.InvoiceComplaintInfo, v usecase.ComplaintView) {
	if rec == nil {
		return
	}
	id := v.Complaint.ComplainantBuyerID
	rec.BuyerMaskedName = s.resolveBuyerNames(ctx, []int64{id})[id]
}
