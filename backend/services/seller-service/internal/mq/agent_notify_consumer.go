package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/zap"
)

type agentNotifyEnvelope struct {
	Payload map[string]string `json:"payload"`
	MerID   string            `json:"mer_id"`
}

func agentNotifyHandler(payuniUC *usecase.PayuniUseCase) func(ctx context.Context, routingKey string, body []byte) error {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var env agentNotifyEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			logger.Warn(ctx, "agent_notify: malformed event", zap.Error(err))
			return fmt.Errorf("unmarshal agent notify event: %w", err)
		}

		merStatus := env.Payload["MerStatus"]
		if merStatus == "" {
			logger.Warn(ctx, "agent_notify: missing MerStatus",
				zap.String("merId", env.MerID))
			return fmt.Errorf("agent notify event missing MerStatus")
		}

		if err := payuniUC.HandleAgentNotify(ctx, env.MerID, merStatus, env.Payload); err != nil {
			logger.Warn(ctx, "agent_notify: handle failed, will requeue",
				zap.String("merId", env.MerID),
				zap.String("merStatus", merStatus),
				zap.Error(err))
			return err
		}
		return nil
	}
}
