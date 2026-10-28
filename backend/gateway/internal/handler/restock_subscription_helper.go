package handler

import (
	"context"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

// applyRestockSubscribed batch-checks ACTIVE restock subscriptions for the
// given buyer and writes the result back via setter.
//
// 對應 issue #1005 / B-203 §5.5b：買家可見的所有 list / search / detail / recommendation
// 都透過此 helper 於 Gateway 層批次聚合 isRestockSubscribed，避免 N+1 RPC。
//
// buyer_id-first（照抄 BatchCheckFavorites 的既驗證 pattern）：呼叫端必須自己把
// accountID 解析成 buyerID 才呼叫本 helper——已有解析結果（例如同函式內收藏聚合
// 已呼叫過 h.BuyerID.CurrentBuyerID）就直接重複使用那個值，不要重複解析；還沒
// 解析過的呼叫點，自己呼叫 h.BuyerID.CurrentBuyerID 解析一次。真正需要容忍的情境
// 是「已登入但沒有 buyer stint」（例如純賣家帳號逛公開商品頁），不是「匿名」——
// 匿名（account_id==0）在所有呼叫點都已被上游擋掉，根本到不了這支 RPC。
//
// 行為：
//   - buyerID <= 0（未解析出 current buyer_id，含呼叫端解析失敗直接傳 0 的情況）
//     或 productIDs 為空 → no-op，所有 item 保持 false
//   - BatchCheckRestockSubscriptions RPC 失敗（含 buyer 模組端 GetCurrentByBuyerID
//     驗證 stint 失敗）→ fail-open（pkglogger.Debug 紀錄；不阻斷 list 主流程，所有
//     item 保持 false）
//   - 部分商品有 ACTIVE 訂閱 → setter 對該 productID 寫 true，其餘 false
func (h *Holder) applyRestockSubscribed(
	ctx context.Context,
	buyerID int64,
	productIDs []string,
	setter func(productID string, subscribed bool),
) {
	if buyerID <= 0 || len(productIDs) == 0 {
		return
	}
	m, err := h.Buyer.BatchCheckRestockSubscriptions(ctx, buyerID, productIDs)
	if err != nil {
		pkglogger.Debug(ctx, "Failed to batch check restock subscriptions",
			zap.Int64("buyerId", buyerID),
			zap.Int("productCount", len(productIDs)),
			zap.Error(err),
		)
		return
	}
	for _, pid := range productIDs {
		setter(pid, m[pid])
	}
}
