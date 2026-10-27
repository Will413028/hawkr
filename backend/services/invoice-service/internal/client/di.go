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
	// 同一個 *seller.Client 也是 ADM 工作台識別欄的店名來源（#3017）。
	// binding 在提供端（同 OrderSnapshotReviser 那一列的理由）。
	fx.Provide(fx.Annotate(newSellerClient, fx.As(new(invoicegrpc.SellerNameResolver)))),
	fx.Provide(newMediaClient),
	// 同一個 *media.Client 也是 §6.1.4 下載路徑的機敏檔讀回面。
	// binding 在提供端（同 OrderSnapshotReviser 那一列的理由）：介面由消費端
	// （usecase）宣告，usecase 的 di.go 不得 import client/media。
	fx.Provide(fx.Annotate(newMediaClient, fx.As(new(usecase.SecureAssetReader)))),
	// 同一個 *media.Client 也是後台稅籍佐證檔的讀回面（plan 8f-1）。binding 在提供端，同上。
	fx.Provide(fx.Annotate(newMediaClient, fx.As(new(usecase.TaxEvidenceReader)))),
	// 同一個 *media.Client 也是稅籍佐證中介資料的來源（#3031）。
	fx.Provide(fx.Annotate(newMediaClient, fx.As(new(invoicegrpc.EvidenceMetadataResolver)))),
	// 同一個 *media.Client 也是 A4 取消證據的換發面（#3108）。binding 在提供端，同上。
	fx.Provide(fx.Annotate(newMediaClient, fx.As(new(usecase.UnlockEvidenceFinalizer)))),
	fx.Provide(fx.Annotate(newOrderClient, fx.As(new(invoicemq.OrderFactsClient)))),
	// 同一個 *order.Client 也是 B06 更正鏈的寫入面（§6.2.5）。
	//
	// 🔴 綁在**這一層**而不是 usecase 的 di.go：後者 import client/order 會形成
	// usecase → client/order → mq → usecase 的循環（實測擋下）。介面由消費端宣告、
	// binding 由提供端做，是這個 repo 既有的方向。
	fx.Provide(fx.Annotate(newOrderClient, fx.As(new(usecase.OrderSnapshotReviser)))),
	fx.Provide(fx.Annotate(newPaymentClient, fx.As(new(invoicemq.PaymentFactsClient)))),
	// SHIP peer：S06／A12 的出貨狀態欄（#3019）。binding 在提供端，同上。
	fx.Provide(fx.Annotate(newShippingClient, fx.As(new(invoicegrpc.ShipmentStatusResolver)))),
	// 同一個 *shipping.Client 也是行事曆鏡像的來源（INV.gov 每日整段覆蓋，
	// §5.1.14；#3089 第 17 項）。binding 在提供端，同上。
	fx.Provide(fx.Annotate(newShippingClient, fx.As(new(usecase.HolidaySource)))),
	// BUY peer：申訴畫面的「是誰提的」欄（#3017 後半）。binding 在提供端，同上。
	fx.Provide(fx.Annotate(newBuyerClient, fx.As(new(invoicegrpc.BuyerNameResolver)))),
)

// newOrderClient 建出 ORD peer client（§6.2）＋ 它需要的 buyer preference reader。
//
// `order.RepoPreferenceReader` 三支能力全部落地（2026-08-30 補齊 GetDefault 與明文出口）。
// `GetDefault`（USE_BUYER_DEFAULT）與 `OpenCarrierValue`（手機條碼／自然人憑證的明文）
// 兩支缺 repository 能力 ⇒ 走到那兩條路會 fail-loud 進 DLQ。缺什麼、為什麼不能用
// `ListBuyerInvoicePreferences` 湊，寫在 `internal/client/order/carrier.go` 的
// `ErrPreferenceCapabilityUnavailable`。
func newOrderClient(lc fx.Lifecycle, cfg *config.Config, prefs domain.BuyerPreferenceRepository) (*order.Client, error) {
	return order.NewClient(lc, cfg.Peers.OrderAddr, order.RepoPreferenceReader{Repo: prefs})
}

// newPaymentClient 建出 PAY peer client（§7.1.2 步驟 5 ②、註 2）。
func newPaymentClient(lc fx.Lifecycle, cfg *config.Config) (*payment.Client, error) {
	return payment.NewClient(lc, cfg.Peers.PaymentAddr)
}

// newShippingClient 建出 SHIP peer client（#3019）。
//
// 位址走既有的 `SHIPPING_SERVICE_ADDR`（`config.PeerConfig.ShippingAddr`，
// k8s/values/invoice-service.yaml 已有此鍵）⇒ 本波不需要新增任何 env var。
// NetworkPolicy 也已放行（`k8s/external-services/networkpolicy.yaml` 的
// `shipping-service-ingress` 已列 invoice-service ⇒ §14.5.0c 三問全部已滿足）。
func newShippingClient(lc fx.Lifecycle, cfg *config.Config) (*shipping.Client, error) {
	return shipping.NewClient(lc, cfg.Peers.ShippingAddr)
}

// newBuyerClient 建出 BUY peer client（#3017 後半）。
//
// 位址走既有的 `BUYER_SERVICE_ADDR`（`config.PeerConfig.BuyerAddr`，
// k8s/values/invoice-service.yaml 已有此鍵）；buyer-service 的
// `networkPolicy.grpcAllowedCallers` 也早已列入 invoice-service
// （`k8s/values/buyer-service.yaml`，**不在 k8s/external-services**，寫錯地方會撞
// 單一來源 gate）⇒ §14.5.0c 三問全部已滿足，本波不需要新增任何 env 或 netpol。
func newBuyerClient(lc fx.Lifecycle, cfg *config.Config) (*buyer.Client, error) {
	return buyer.NewClient(lc, cfg.Peers.BuyerAddr)
}

// newSellerClient 建出 SEL peer client（§6.2）。
//
// 位址走既有的 `SELLER_SERVICE_ADDR`（`config.PeerConfig.SellerAddr`，
// k8s/values/invoice-service.yaml 已有此鍵）⇒ 本波不需要新增任何 env var。
func newSellerClient(lc fx.Lifecycle, cfg *config.Config) (*seller.Client, error) {
	return seller.NewClient(lc, cfg.Peers.SellerAddr)
}

// newMediaClient 建出 MED peer client（§6.1.7 稅籍證明的 staged token 換發）。
//
// 位址走既有的 `MEDIA_SERVICE_HOST`＋`MEDIA_SERVICE_GRPC_PORT`（`config.PeerConfig.MediaAddr`）
// ⇒ 本波不需要新增任何 env var。
func newMediaClient(lc fx.Lifecycle, cfg *config.Config) (*media.Client, error) {
	return media.NewClient(lc, cfg.Peers.MediaAddr)
}

// newAmegoClient 依 §4.5.9 的兩種情境建出 client——差異壓縮到只有兩個設定值
// （AMEGO_BASE_URL 與 AMEGO_PROXY_CALLER_KEY_FILE），程式碼分支為零。
//
// http.Client.Timeout 取 deadlineIssue + 5s 當 backstop：idtoken.NewClient 不設
// timeout，proxy 卡住會無限期掛住呼叫端。權威仍是 per-call ctx deadline。
func newAmegoClient(cfg *config.Config, audits domain.AuditRepository, log *zap.Logger) (amego.Client, error) {
	deadlines := amego.Deadlines{
		Issue: time.Duration(cfg.Amego.TimeoutIssueMS) * time.Millisecond,
		Query: time.Duration(cfg.Amego.TimeoutQueryMS) * time.Millisecond,
		// deadlineList 刻意不與 deadlineQuery 共用一個 env：併回去會讓 INV.op 的
		// 租約下界少算 10s（§4.5.12／§7.5.4）。
		List: 20 * time.Second,
		Time: 5 * time.Second,
	}
	transport, err := amego.NewTransport(context.Background(), cfg.Amego.BaseURL, cfg.Amego.ProxyCallerKeyFile, deadlines.Issue+5*time.Second)
	if err != nil {
		return nil, err
	}
	return amego.New(amego.Options{
		BaseURL:    cfg.Amego.BaseURL,
		HTTPClient: transport,
		// 每一次 AMEGO 往返落 invoice_provider_audit_logs（§8.2.1，5 年保存）。
		// 接上之前這裡是 NoopAuditSink ⇒ 發票開得出來而稽核表一列都沒有，
		// 且沒有任何測試會紅（2026-08-30 dev E2E 實際踩到）。
		Audit: &repoAuditSink{repo: audits, log: log},
		// 🔴 判定「准不准開真發票」的是 AMEGO_ALLOW_REAL_INVOICE，不是 Env=="prod"。
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
