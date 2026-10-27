package grpc

import (
	"context"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/client/buyer"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// 申訴畫面的「是誰提的」欄（#3017 後半）
//
// 🔴 `buyer_masked_name` 在 2026-09-02 之前**全服務零賦值**：proto 有這一欄、
// gateway 照實透傳、只有生產者從來沒填 ⇒ A06／A07（平台）與 S09／S10（賣家）
// 的每一張申訴卡都認不出人。
//
// 🔴 **遮罩後才出去**：讀這一欄的是賣家與平台管理員，不是資料主體本人
// （58A §0.4 ③）。值的來源是 `buyers.display_name`（暱稱），本服務從不持有買家真名
// （§5.1.5：B2C 的 `recipient_name` 恆為常數「消費者」）。
// ---------------------------------------------------------------------------

// BuyerNameResolver 是買家顯示名的出向面（實作在 client/buyer）。
//
// 介面宣告在消費端：本層只需要「給我一批 buyer_id、還我顯示名與註銷旗標」。
type BuyerNameResolver interface {
	BatchGetBuyerDisplay(ctx context.Context, buyerIDs []int64) (map[int64]buyer.Display, error)
}

// resolveBuyerNames 批次取顯示名，**回傳的已經是遮罩後的值**。
//
// 遮罩放在這一層而不是呼叫端：讓「拿到名字」與「遮罩」在型別上無法拆開，
// 呼叫端拿不到明碼 ⇒ 不可能有人忘了遮。
//
// 🔴 **失敗降級成空 map，不中止列表**：申訴資料本身完全健康，buyer-service 一次抖動
// 不該把整個申訴佇列打不開。與 shipment_status 不同的是，這一欄的空字串**沒有歧義**
// ——它就是「顯示不出名字」，不會被讀成別的意思 ⇒ 不需要額外的 degraded 旗標。
//
// ⚠️ 降級要留痕，否則「買家名又全空了」在下一次發生時無從診斷。
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

// fillBuyerNames 是「一頁申訴 → 補上 buyer_masked_name」的唯一入口。
//
// 逐列取 `ComplainantBuyerID`，一次 RPC 取完整頁。records 與 views 必須等長且同序
// （records[i] 是 views[i] 折出來的）——由呼叫端保證，它們就在同一個迴圈裡產生。
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

// fillOneBuyerName 是單筆版本（端點 15／35／36／59 等詳情路徑）。
func (s *InvoiceServer) fillOneBuyerName(ctx context.Context, rec *pb.InvoiceComplaintInfo, v usecase.ComplaintView) {
	if rec == nil {
		return
	}
	id := v.Complaint.ComplainantBuyerID
	rec.BuyerMaskedName = s.resolveBuyerNames(ctx, []int64{id})[id]
}
