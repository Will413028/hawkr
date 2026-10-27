package app

import (
	"go.uber.org/fx"

	notifclient "github.com/Will413028/hawkr/notification-service/internal/client"
	"github.com/Will413028/hawkr/notification-service/internal/config"
	notifgrpc "github.com/Will413028/hawkr/notification-service/internal/grpc"
	"github.com/Will413028/hawkr/notification-service/internal/infrastructure"
	"github.com/Will413028/hawkr/notification-service/internal/mq"
	"github.com/Will413028/hawkr/notification-service/internal/repository"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"github.com/Will413028/hawkr/notification-service/internal/worker"
)

// New creates the application with all modules assembled.
func New() *fx.App {
	return fx.New(
		// Layer 0: Config & Logger
		config.Module,

		// Layer 1: Infrastructure (telemetry, Firebase)
		infrastructure.Module,

		// Layer 2: Data access
		repository.Module,

		// Layer 2.5: MQ publisher (41 號設計書 — notification 首次成為 producer)
		mq.Module,

		// Layer 2.6: buyer-service/seller-service gRPC clients (chat-notification
		// PII 清除流程列舉帳號歷史 buyer_id/store_id surrogate)
		notifclient.Module,

		// Layer 3: Business logic
		usecase.Module,

		// Layer 4: Transport (gRPC)
		notifgrpc.Module,

		// Layer 5: Event consumers (RabbitMQ)
		worker.Module,
	)
}
