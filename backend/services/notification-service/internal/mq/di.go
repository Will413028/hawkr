package mq

import (
	"context"

	"github.com/Will413028/hawkr/notification-service/internal/config"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module provides MQ publisher dependencies.
// 2026-06 新增：notification-service 首次成為 producer，供 41 號 SSE 推播平台使用。
var Module = fx.Module("notification-mq",
	fx.Provide(
		newPublisher,
		asEventPublisher, // adapter：把 *Publisher 對應到 usecase.EventPublisher，避免 Go interface nil pitfall
	),
)

// newPublisher 建立 MQ publisher；連 RabbitMQ 失敗時 log 並 return nil-safe 實作（不阻塞服務啟動）。
// 41 號設計書 §5.0「啟動策略不阻塞」精神：MQ 不可用時 publish 變 no-op，不影響 inbox / FCM 主流程。
func newPublisher(lc fx.Lifecycle, cfg *config.Config, l *zap.Logger) *Publisher {
	p, err := NewPublisher(cfg.RabbitMQ.URL)
	if err != nil {
		l.Warn("notification mq publisher init failed, SSE realtime publish will be no-op",
			zap.Error(err))
		// 不退出：MQ 不可用時改成 best-effort no-op，避免 notification-service 整段掛掉
		return nil
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			p.Close()
			return nil
		},
	})
	l.Info("notification mq publisher ready", zap.String("exchange", "hawkr.notification"))
	return p
}

// asEventPublisher 把 *Publisher 轉成 usecase.EventPublisher。
// 注意：當 p 為 nil 時必須回 typed nil interface（不能 return p），否則 inboxUC 的 nil-check 會失效（Go interface nil pitfall）。
func asEventPublisher(p *Publisher) usecase.EventPublisher {
	if p == nil {
		return nil
	}
	return p
}
