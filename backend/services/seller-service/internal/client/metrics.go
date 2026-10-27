// services/seller-service/internal/client/metrics.go
package client

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// 選填外部依賴的開通狀態指標（backend#3329）
//
// 🔴 存在的理由：#3329 把 TGOS 四項與 PayUni 代理商憑證從「啟動必要條件」改成選填。
// 少了這些指標，改動的淨效果只是把 CrashLoop 換成**靜默失能**——問題照樣發生，
// 只是沒有人看得到（06-no-small-tails.md §3.9 第 6 種外殼）。
//
// 每一支都是 0/1 gauge，在 fx 接線當下 Set 一次：接線是這件事唯一的決定點。
// ---------------------------------------------------------------------------

var addressLookupEnabled = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "seller_address_lookup_enabled",
	Help: "1 when the TGOS geocode client is wired; 0 means SuggestAddress/ValidateAddress fail closed with 9700",
})

var addressLookupConfigIncomplete = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "seller_address_lookup_config_incomplete",
	Help: "1 when the TGOS settings are partially filled (a misconfiguration, not an un-provisioned environment)",
})

// payuniMerchantEnabled 答「金流商家建檔到底能不能用」。
//
// 0 ＝ BindSeller／GetSellerStatus／UpdateMerchant 一律回 9704。賣家會看到「尚未開通」
// 而不是一個卡在 PENDING 永遠不動的申請——後者是 fail-open 會產生的症狀。
var payuniMerchantEnabled = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "seller_payuni_merchant_enabled",
	Help: "1 when the PayUni agent credentials are present; 0 means merchant onboarding fails closed with 9704",
})

var payuniMerchantConfigIncomplete = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "seller_payuni_merchant_config_incomplete",
	Help: "1 when exactly one of PAYUNI_AGENT_HASH_KEY/PAYUNI_AGENT_HASH_IV is set (a misconfiguration)",
})
