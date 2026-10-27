package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// CancelledOrderRepository 是 invoice_cancelled_orders 的 thin adapter（#2904）。
//
// 這張表沒有 status 欄、沒有租約欄、沒有排程欄，也**沒有任何 worker 掃它**：它是
// append-only 的事實記錄，不是候選集合。因此《多副本排程搶佔規範》的整組判準
// （claim／租約／候選窗口）對它不適用——不是「豁免」，是它根本不是那個形狀。
//
// 🔴 本檔沒有、也不得新增 Delete／Update：一張訂單被取消是不可逆的事實，而讀路徑
// 拿它回答「這句『付款完成後會立即開立發票』該不該說」。刪掉那一列 ⇒ 該訂單靜默
// 退回原本的錯誤文案，且沒有任何東西會報。
type CancelledOrderRepository struct {
	baseRepo
}

// NewCancelledOrderRepository 建立 repository。
func NewCancelledOrderRepository(pool *pgxpool.Pool) *CancelledOrderRepository {
	return &CancelledOrderRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.CancelledOrderRepository = (*CancelledOrderRepository)(nil)

// Insert 記下「這張訂單已取消」。
//
// rows==0 ⇒ 已經記過（at-least-once 重投），回 inserted=false 讓呼叫端自己決定語意；
// 它**不是**錯誤，回 error 會讓 consumer 走 retry/DLQ，而那則訊息本來就該被 ACK。
func (r *CancelledOrderRepository) Insert(ctx context.Context, in domain.CancelledOrder) (bool, error) {
	rows, err := r.q(ctx).InsertCancelledOrder(ctx, gen.InsertCancelledOrderParams{
		OrderNo: in.OrderNo,
		BuyerID: in.BuyerID,
		// 空字串折成 NULL：producer 對「買家自願取消」這一格送的是 JSON null，
		// 兩者在本表要是同一個值，否則同一件事會有兩種表示。
		CancelReason: nilIfBlank(in.CancelReason),
		OccurredAt:   ts(in.OccurredAt),
		Now:          ts(in.Now),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// IsCancelledForBuyer 回答「這張訂單對這位買家而言是不是已取消」。
//
// 🔴 兩個參數都進 WHERE。授權述詞寫在 SQL 裡而不是撈出來再比對，理由與本 package
// 其餘 Tier A 查詢一致（repo.go 的 notFound 註解）：兩者在這一層長得一樣，才不會
// 有人「順手」把越權與查無分成兩種回應。
func (r *CancelledOrderRepository) IsCancelledForBuyer(ctx context.Context, orderNo string, buyerID int64) (bool, error) {
	return r.q(ctx).IsOrderCancelledForBuyer(ctx, gen.IsOrderCancelledForBuyerParams{
		OrderNo: orderNo,
		BuyerID: buyerID,
	})
}
