package mq

import (
	"context"
	"encoding/json"

	"github.com/Will413028/hawkr/go-pkg/logger"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/zap"
)

func productViewHandler(productViewUC *usecase.ProductViewUseCase) func(ctx context.Context, routingKey string, body []byte) error {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var event pkgmq.ProductViewedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			logger.Warn(ctx, "product_view: malformed event, skipping", zap.Error(err))
			return nil
		}

		if event.SellerID == 0 || event.ProductID == 0 {
			logger.Warn(ctx, "product_view: missing SellerID or ProductID, skipping")
			return nil
		}

		if err := productViewUC.RecordView(ctx, event.SellerID, event.ProductID, "app"); err != nil {
			logger.Warn(ctx, "product_view: failed to record view, dropping",
				zap.Int64("seller_id", event.SellerID),
				zap.Int64("product_id", event.ProductID),
				zap.Error(err))
		}

		return nil // best-effort analytics，不重試
	}
}
