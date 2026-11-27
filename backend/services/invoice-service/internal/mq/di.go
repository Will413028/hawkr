// Package mq 是 Layer 3.5：入向事件去重與落 outbox、出向事件發布。
//
// consumer 只負責把工作「排進來」，不在 consumer 內直接打 AMEGO（§4.2.2）。
// publisher 是薄 wrapper：建不起來回 nil 不 panic（§4.2.1）。
package mq

import (
	"context"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ConsumerMaxRetries 是三條入向 queue 的 Subscription.MaxRetries。
//
// 🔴 **它必須等於 `infra/mq/topology/topology.yaml` 上這三條 queue 的 `retry.max_attempts`。**
// 這是 §6.3.17 五組「漏了不會有任何東西轉紅」的成對登記之一，而且是其中**沒有 CI gate**
// 的那一組：不一致的後果是 framework 的 passive 檢查找不到 `{queue}.retry` exchange
// ⇒ 本機綠、CI 綠、上線整條 subscription 被 skip，而發票一張都不會開。
//
// 本波補上的機械保證是 `topology_contract_test.go`：它讀 topology.yaml 逐 queue 對拍。
//
// 值是 3 而不是更大：handler 一律是 DB-only（consumer 絕不呼叫 AMEGO，§6.3.1），
// 重試沒有外部副作用，3 次即足夠。
const ConsumerMaxRetries = 3

// Module 是 Layer 3.5 的組裝。
//
// ⚠️ 新增 consumer 的同一個 PR 必須加 infra/mq/topology/topology.yaml 條目，
// 漏了是完全靜默的（passive declare 查不到 queue 不 crash、不進 DLQ）。
var Module = fx.Module("mq",
	fx.Provide(newEventPublisher),
	fx.Provide(newAdjustmentApplier),
	fx.Provide(fx.Annotate(newIssuancePreparer, fx.As(new(IssuancePreparer)))),
	fx.Provide(NewOrderPaidConsumer),
	fx.Provide(NewRefundFinalizedConsumer),
	fx.Provide(NewOrderCancelledConsumer),
	fx.Invoke(startConsumers),
)

func newEventPublisher(cfg *config.Config, log *zap.Logger, lc fx.Lifecycle) *EventPublisher {
	p := NewEventPublisher(cfg.MQ.URL, log)
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return p.Close() }})
	return p
}

func newIssuancePreparer(u *usecase.Issuance) *usecase.Issuance { return u }

func newAdjustmentApplier(uc *usecase.RefundAdjustment) AdjustmentApplier {
	return refundAdjustmentApplier{uc: uc}
}

type refundAdjustmentApplier struct{ uc *usecase.RefundAdjustment }

func (a refundAdjustmentApplier) ApplyRefund(
	ctx context.Context, in RefundFinalized, doc domain.Document,
) (*RefundDisposition, error) {
	skip, err := a.uc.Apply(ctx, usecase.RefundAdjustmentInput{
		OrderNo:                in.OrderNo,
		RefundRef:              in.RefundRef,
		RefundAmount:           in.RefundAmount,
		CumulativeRefundAmount: in.CumulativeRefundAmount,
		OrderTotalActualAmount: in.OrderTotalActualAmount,
		OccurredAt:             in.OccurredAt,
	}, doc)
	if err != nil || skip == nil {
		return nil, err
	}
	return &RefundDisposition{Disposition: skip.Disposition, Reason: skip.Reason}, nil
}

// Subscriptions 是本服務全部入向 queue 的唯一定義。
//
// 它是**匯出的**，因為 topology_contract_test.go 必須拿得到真正被註冊的那一份
// （queue 名 ＋ MaxRetries）去跟 topology.yaml 對拍。把 subscription 直接寫在
// startConsumers 內、測試自己再抄一份，對拍的就是抄本不是本尊。
func Subscriptions(
	paid *OrderPaidConsumer,
	refund *RefundFinalizedConsumer,
	cancelled *OrderCancelledConsumer,
) []pkgmq.Subscription {
	return []pkgmq.Subscription{
		{
			Exchange:    pkgmq.ExchangeOrder,
			Queue:       pkgmq.QueueInvoiceOrderPaid,
			RoutingKeys: []string{pkgmq.RoutingKeyOrderPaid},
			Handler:     paid.Handle,
			MaxRetries:  ConsumerMaxRetries,
		},
		{
			Exchange:    pkgmq.ExchangeOrder,
			Queue:       pkgmq.QueueInvoiceOrderRefundFinalized,
			RoutingKeys: []string{pkgmq.RoutingKeyOrderRefundFinalized},
			Handler:     refund.Handle,
			MaxRetries:  ConsumerMaxRetries,
		},
		{
			Exchange:    pkgmq.ExchangeOrder,
			Queue:       pkgmq.QueueInvoiceOrderCancelled,
			RoutingKeys: []string{pkgmq.RoutingKeyOrderCancelled},
			Handler:     cancelled.Handle,
			MaxRetries:  ConsumerMaxRetries,
		},
	}
}

func startConsumers(
	lc fx.Lifecycle,
	cfg *config.Config,
	paid *OrderPaidConsumer,
	refund *RefundFinalizedConsumer,
	cancelled *OrderCancelledConsumer,
	log *zap.Logger,
) {
	l := orNopLogger(log)
	if cfg.MQ.URL == "" {
		l.Error("invoice.mq.consumers_disabled", zap.String("reason", "RABBITMQ_URL is empty"))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	consumer := pkgmq.NewResilientConsumer(cfg.MQ.URL, Subscriptions(paid, refund, cancelled))

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go consumer.Run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

var _ IssuancePreparer = (*usecase.Issuance)(nil)
