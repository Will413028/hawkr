package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Will413028/hawkr/backend/internal/legacy/reviewservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerserver"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/payuni"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// kycKeys 是 KYC 個資的金鑰：加密金鑰環與比對用的 HMAC 金鑰環。
type kycKeys struct {
	enc *seal.Keyring
	mac *seal.MAC
}

// readKYCKeys 讀 KYC 的金鑰設定：
//
//   - KYC_ENCRYPTION_KEYS：身分證號、銀行帳號的 AES-256-GCM 加密金鑰，逗號分隔的 id:base64(32 bytes)，第一把是目前的金鑰；
//     換金鑰時把新的放最前面、舊的留到它加密的資料都不在了。
//   - KYC_HASH_KEYS：身分比對（blind index）的 HMAC 金鑰，格式相同、每把至少 32 bytes，第一把計算、全部可以查詢；處分的身分
//     快照沒有明文可以重算，所以舊的金鑰要留到用它算的快照所屬的處分都結束。與加密金鑰必須是不同的金鑰（啟動時檢查）。
//
// 兩個都是必填（沒設定、空的、太短或兩者共用同一把金鑰都會讓啟動失敗）。
func readKYCKeys(e *config.Env) *kycKeys {
	keys, err := parseKYCKeys(e.String("KYC_ENCRYPTION_KEYS"), e.String("KYC_HASH_KEYS"))
	if err != nil {
		e.Fail("KYC 金鑰：%v", err)
	}
	return keys
}

func parseKYCKeys(enc, mac string) (*kycKeys, error) {
	if enc == "" || mac == "" {
		return nil, errors.New("KYC_ENCRYPTION_KEYS 與 KYC_HASH_KEYS 都必須設定")
	}
	ring, err := seal.Parse(enc)
	if err != nil {
		return nil, fmt.Errorf("KYC_ENCRYPTION_KEYS：%w", err)
	}
	macs, err := seal.ParseMAC(mac)
	if err != nil {
		return nil, fmt.Errorf("KYC_HASH_KEYS：%w", err)
	}
	if err := seal.Distinct(ring, macs); err != nil {
		return nil, err
	}
	return &kycKeys{enc: ring, mac: macs}, nil
}

// payuniSetup 是 PayUni 的設定結果：client 為 nil 表示沒有設定（綁定記為 PAYUNI_NOT_CONFIGURED，輪詢不排）。
type payuniSetup struct {
	client     seller.PayUniClient
	storefront string
}

// readPayUni 讀 PayUni 代理商 API 的設定，沒有任何預設值（網址與金鑰都不進版控、不指向任何主機）：
//
//   - PAYUNI_BASE_URL、PAYUNI_AGENT_ID、PAYUNI_HASH_KEY（32 bytes）、PAYUNI_HASH_IV（16 bytes）：四個一起設定才會啟用，
//     都沒設定時 PayUni 是關閉的（本機開發的預設；註冊記為失敗，設定好之後由管理員重新綁定），只設一部分、長度不對或
//     網址格式不合會讓啟動失敗。PAYUNI_TIMEOUT 是單次呼叫的上限（預設 15 秒）；PAYUNI_MAX_QPS 是這個程序所有 PayUni 呼叫合計的速率上限（預設 2，PayUni 的限制）。
//   - PAYUNI_STOREFRONT_BASE_URL：店家頁的網站根網址（選填；申請沒有填官網時，MerURL 是它加 /sellers/<store_code>）。
func readPayUni(e *config.Env, log *slog.Logger) *payuniSetup {
	cfg := payuni.Config{
		BaseURL: e.StringOr("PAYUNI_BASE_URL", ""), AgentID: e.StringOr("PAYUNI_AGENT_ID", ""),
		HashKey: e.StringOr("PAYUNI_HASH_KEY", ""), HashIV: e.StringOr("PAYUNI_HASH_IV", ""),
		Timeout: e.Duration("PAYUNI_TIMEOUT", payuni.DefaultTimeout),
		MaxQPS:  float64(e.Int("PAYUNI_MAX_QPS", int(payuni.DefaultMaxQPS))),
	}
	if cfg.MaxQPS < 1 {
		e.Fail("PayUni：PAYUNI_MAX_QPS 必須是正整數")
	}
	setup := &payuniSetup{storefront: e.StringOr("PAYUNI_STOREFRONT_BASE_URL", "")}
	set := 0
	for _, v := range []string{cfg.BaseURL, cfg.AgentID, cfg.HashKey, cfg.HashIV} {
		if v != "" {
			set++
		}
	}
	switch set {
	case 0:
		log.Warn("PayUni 沒有設定：核准後的綁定會記為 PAYUNI_NOT_CONFIGURED，不會輪詢")
		return setup
	case 4:
	default:
		e.Fail("PayUni：PAYUNI_BASE_URL、PAYUNI_AGENT_ID、PAYUNI_HASH_KEY、PAYUNI_HASH_IV 必須一起設定")
		return setup
	}
	client, err := payuni.New(cfg)
	if err != nil {
		e.Fail("PayUni：%v", err)
		return setup
	}
	setup.client = client
	return setup
}

// readGCIS 讀統一編號查詢（經濟部商業司的公開資料，不是舊公司的服務）的設定：GCIS_BASE_URL（預設是公開資料平台
// https://data.gcis.nat.gov.tw）、GCIS_TIMEOUT（一次查詢的總上限，預設 5 秒；公司與商業登記兩個資料集的請求共用）。
// serve（統編查詢的 Connect 方法、送件時排入登記資料的查詢）、worker（執行查詢的 job）與建構 seller 模組的子命令都需要它。
// 網址格式不合會讓啟動失敗。
func readGCIS(e *config.Env) *gcis.Client {
	client, err := gcis.New(gcis.Config{BaseURL: e.StringOr("GCIS_BASE_URL", ""), Timeout: e.Duration("GCIS_TIMEOUT", gcis.DefaultTimeout)})
	if err != nil {
		e.Fail("GCIS：%v", err)
	}
	return client
}

// newSeller 組裝 seller 模組。serve 與 worker 都需要它：serve 給 accountdeletion 當參與者，worker 另外執行它的
// job（圖片、治理佐證與 KYC 證件的引用收斂、處分的到期與對帳、匿名通報人聯絡欄位的清理、關店滿保留期的個資清理、PayUni
// 的綁定與同步、通知的套用、輪詢、商家名單圖片的引用收斂、商品瀏覽事件的彙總與清理；排程的是 PeriodicJobs）。orders 是店家未結訂單的 port（自己關店用；刪帳號的
// accountdeletion 用同一個實作），products 是商品所屬店家的 port（彙總瀏覽事件用，見 sellerProducts）。
// 它的事件由 subscribeSellerRelay 轉成給舊服務的 RabbitMQ 訊息（sellerrelay），治理事件的通知在 A8。
func newSeller(d *db.DB, bus *events.Bus, jc jobs.ReportingEnqueuer, id *identity.Module, med media.Media, orders seller.Orders, products seller.ProductStores, kyc *kycKeys, pay *payuniSetup, registry *gcis.Client, log *slog.Logger) *seller.Module {
	return seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: jc, Identity: id, Media: med, Orders: orders, KYCKeys: kyc.enc, KYCMAC: kyc.mac,
		ProductStores: products, PayUni: pay.client, Registry: registry, StorefrontBaseURL: pay.storefront, Now: time.Now, Logger: log,
	})
}

// legacySellerRatings 以舊的 review-service 實作 legacy SellerService 的評價統計 port（sellerserver.Ratings）。
type legacySellerRatings struct{ client *reviewservice.Client }

func (r legacySellerRatings) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]sellerserver.RatingStats, error) {
	return ratingStats[sellerserver.RatingStats](ctx, r.client, storeIDs)
}
