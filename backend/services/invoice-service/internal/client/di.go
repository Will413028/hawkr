// Package client 是 Layer 3：AMEGO provider client ＋ peer gRPC client。
//
// internal/client/amego 是唯一會打 AMEGO 的 package（§4.2.2 的跨模組硬規則）：
// 其他任何檔案都不得 import net/http 去打 invoice-api.amego.tw。
package client

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/client/buyer"
	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/client/order"
	"github.com/Will413028/hawkr/invoice-service/internal/client/payment"
	"github.com/Will413028/hawkr/invoice-service/internal/client/seller"
	"github.com/Will413028/hawkr/invoice-service/internal/client/shipping"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicegrpc "github.com/Will413028/hawkr/invoice-service/internal/grpc"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module 提供 amego.Client ＋ 三支 peer client（SEL／ORD／PAY）。
//
// 🔴 **ORD 與 PAY 的 provider 住在本檔而不是 `internal/mq/di.go`**：兩支 client 回傳
// `internal/mq` 定義的 port 型別（`mq.OrderFacts`／`mq.PaymentFacts`），所以
// `internal/client/*` → `internal/mq` 這個方向必須是單向的。把 provider 放回 mq 那側
// 會讓 `internal/mq` 反向 import `internal/client` ⇒ import cycle（編譯期就紅，
// 不是靜默失效，但方向錯了改起來要動兩層）。
//
// ⚠️ ORD／PAY 的 fail-loud 佔位（`mq.Unavailable*Client`）已不再被註冊，但**型別留著**：
// 它們是 `internal/mq` 自己的測試用得到的替身，也是「這條路徑必須 fail-loud」這條規則
// 的可執行說明。
//
// `newSellerClient` 的消費點是 `internal/worker/di.go` 的 `newSellerAccountResolver`
// （2026-08-30 接上；`*seller.Client` 逐字滿足 `worker.SellerAccountResolver`，
// `internal/client/seller/client_test.go` 有編譯期斷言守住）。
//
// ⚠️ 「fx.Provide 是惰性的」這件事本身要有機制守：一個沒有 consumer 的 Provide 在圖上
// 等同不存在，壞了也沒人知道。守它的是 `internal/app/app_test.go` 的整圖 ValidateApp
// ——各層自己的 di_test.go 都用 `func() X { return nil }` 供跨層依賴，**驗不到這件事**
// （2026-08-30 實際踩到：worker 那層的圖測試綠，而真正缺的 client.Module 只有開機才會發現）。
var Module = fx.Module("client",
	fx.Provide(newAmegoClient),
	fx.Provide(newSellerClient),
	fx.Provide(fx.Annotate(newSellerClient, fx.As(new(invoicegrpc.SellerNameResolver)))),
	fx.Provide(newMediaClient),
	fx.Provide(fx.Annotate(newMediaClient, fx.As(new(usecase.SecureAssetReader)))),
	fx.Provide(fx.Annotate(newMediaClient, fx.As(new(usecase.TaxEvidenceReader)))),
	fx.Provide(fx.Annotate(newMediaClient, fx.As(new(invoicegrpc.EvidenceMetadataResolver)))),
	fx.Provide(fx.Annotate(newMediaClient, fx.As(new(usecase.UnlockEvidenceFinalizer)))),
	fx.Provide(fx.Annotate(newOrderClient, fx.As(new(invoicemq.OrderFactsClient)))),
	fx.Provide(fx.Annotate(newOrderClient, fx.As(new(usecase.OrderSnapshotReviser)))),
	fx.Provide(fx.Annotate(newPaymentClient, fx.As(new(invoicemq.PaymentFactsClient)))),
	fx.Provide(fx.Annotate(newShippingClient, fx.As(new(invoicegrpc.ShipmentStatusResolver)))),
	fx.Provide(fx.Annotate(newBuyerClient, fx.As(new(invoicegrpc.BuyerNameResolver)))),
)

func newOrderClient(lc fx.Lifecycle, cfg *config.Config, prefs domain.BuyerPreferenceRepository) (*order.Client, error) {
	return order.NewClient(lc, cfg.Peers.OrderAddr, order.RepoPreferenceReader{Repo: prefs})
}

func newPaymentClient(lc fx.Lifecycle, cfg *config.Config) (*payment.Client, error) {
	return payment.NewClient(lc, cfg.Peers.PaymentAddr)
}

func newShippingClient(lc fx.Lifecycle, cfg *config.Config) (*shipping.Client, error) {
	return shipping.NewClient(lc, cfg.Peers.ShippingAddr)
}

func newBuyerClient(lc fx.Lifecycle, cfg *config.Config) (*buyer.Client, error) {
	return buyer.NewClient(lc, cfg.Peers.BuyerAddr)
}

func newSellerClient(lc fx.Lifecycle, cfg *config.Config) (*seller.Client, error) {
	return seller.NewClient(lc, cfg.Peers.SellerAddr)
}

func newMediaClient(lc fx.Lifecycle, cfg *config.Config) (*media.Client, error) {
	return media.NewClient(lc, cfg.Peers.MediaAddr)
}

func newAmegoClient(cfg *config.Config, audits domain.AuditRepository, log *zap.Logger) (amego.Client, error) {
	deadlines := amego.Deadlines{
		Issue: time.Duration(cfg.Amego.TimeoutIssueMS) * time.Millisecond,
		Query: time.Duration(cfg.Amego.TimeoutQueryMS) * time.Millisecond,
		List:  20 * time.Second,
		Time:  5 * time.Second,
	}
	transport, err := amego.NewTransport(context.Background(), cfg.Amego.BaseURL, cfg.Amego.ProxyCallerKeyFile, deadlines.Issue+5*time.Second)
	if err != nil {
		return nil, err
	}
	return amego.New(amego.Options{
		BaseURL:          cfg.Amego.BaseURL,
		HTTPClient:       transport,
		Audit:            &repoAuditSink{repo: audits, log: log},
		AllowRealInvoice: cfg.Amego.AllowRealInvoice,
		TestInvoice:      cfg.Amego.TestInvoice,
		RateLimitQPS:     cfg.Amego.RateLimitQPS,
		Breaker: amego.BreakerSettings{
			MaxRequests: uint32(cfg.Amego.CBMaxRequests), //nolint:gosec // 值域由 Config.Validate 守
			Interval:    30 * time.Second,
			Timeout:     time.Duration(cfg.Amego.CBTimeoutSeconds) * time.Second,
			Failures:    uint32(cfg.Amego.CBFailureThreshold), //nolint:gosec // 同上
		},
		Deadlines: deadlines,
	})
}
