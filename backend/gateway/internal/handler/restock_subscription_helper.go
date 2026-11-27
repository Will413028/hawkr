package handler

import (
	"context"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

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
