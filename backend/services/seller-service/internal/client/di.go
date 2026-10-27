package client

import (
	"context"

	pkggeocode "github.com/Will413028/hawkr/go-pkg/geocode"
	"github.com/Will413028/hawkr/seller-service/internal/client/account"
	buyerclient "github.com/Will413028/hawkr/seller-service/internal/client/buyer"
	gcisclient "github.com/Will413028/hawkr/seller-service/internal/client/gcis"
	geocodeclient "github.com/Will413028/hawkr/seller-service/internal/client/geocode"
	mediaclient "github.com/Will413028/hawkr/seller-service/internal/client/media"
	orderclient "github.com/Will413028/hawkr/seller-service/internal/client/order"
	payuniclient "github.com/Will413028/hawkr/seller-service/internal/client/payuni"
	productclient "github.com/Will413028/hawkr/seller-service/internal/client/product"
	reviewclient "github.com/Will413028/hawkr/seller-service/internal/client/review"
	socialclient "github.com/Will413028/hawkr/seller-service/internal/client/social"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("client",
	fx.Provide(providePayuniClient),
	fx.Provide(fx.Annotate(gcisclient.NewClient, fx.As(new(usecase.GcisClient)))),
	fx.Provide(fx.Annotate(account.NewClient, fx.As(new(usecase.AccountClient)), fx.As(new(usecase.AccountRoleRemover)), fx.As(new(usecase.AccountRoleGranter)))),
	fx.Provide(fx.Annotate(mediaclient.NewClient, fx.As(new(usecase.MediaClient)))),
	fx.Provide(fx.Annotate(orderclient.NewClient, fx.As(new(usecase.OrderClient)), fx.As(new(usecase.ActiveMerchantOrderCountClient)))),
	fx.Provide(fx.Annotate(buyerclient.NewClient, fx.As(new(usecase.BuyerClient)))),
	fx.Provide(fx.Annotate(reviewclient.NewClient, fx.As(new(usecase.ReviewClient)))),
	fx.Provide(fx.Annotate(socialclient.NewClient, fx.As(new(usecase.SocialClient)))),
	fx.Provide(fx.Annotate(productclient.NewClient, fx.As(new(usecase.ProductClient)))),
	fx.Provide(provideGeocodeClient),
)

// providePayuniClient 接上 PayUni 代理商建檔；**憑證未設定時回一個 fail-closed 的
// 停用實作**，而不是讓服務起不來（backend#3329）。
//
// 🔴 為什麼用 disabledPayuniClient 而不是回 nil ＋ 各呼叫點 nil 檢查：
// PayuniClient 的三支方法在 usecase 裡有十幾個呼叫點，nil 檢查只要有一個新呼叫點忘了加
// 就是 nil pointer panic，而且沒有任何 gate 攔得住「忘了加」。停用實作把 fail-closed
// 放進型別本身——新呼叫點**自動**拿到 9704，忘不掉。
func providePayuniClient(cfg *config.Config, log *zap.Logger) usecase.PayuniClient {
	switch {
	case cfg.PayuniMerchantConfigured():
		payuniMerchantConfigIncomplete.Set(0)
		payuniMerchantEnabled.Set(1)
		return payuniclient.NewClient(cfg)
	case cfg.PayuniMerchantPartiallyConfigured():
		payuniMerchantConfigIncomplete.Set(1)
		payuniMerchantEnabled.Set(0)
		log.Error("PayUni 代理商憑證只有一半，金流商家建檔已停用",
			zap.Bool("has_hash_key", cfg.Payuni.HashKey != ""),
			zap.Bool("has_hash_iv", cfg.Payuni.HashIV != ""),
			zap.String("impact", "建檔／狀態同步／商家資料更新回 9704；其餘 RPC 不受影響"),
			zap.String("fix", "補齊 PAYUNI_AGENT_HASH_KEY 與 PAYUNI_AGENT_HASH_IV"))
		return disabledPayuniClient{}
	default:
		// 兩者皆空 ＝ 尚未開通。不是配置錯誤，所以 incomplete 維持 0。
		payuniMerchantConfigIncomplete.Set(0)
		payuniMerchantEnabled.Set(0)
		log.Warn("PayUni 代理商憑證尚未設定，金流商家建檔將回 9704",
			zap.String("impact", "seller-service 其餘 RPC 正常服務"))
		return disabledPayuniClient{}
	}
}

// disabledPayuniClient 是 PayuniClient 的 fail-closed 停用實作：每一支都回 9704。
//
// 🔴 三支方法**沒有一支**回「成功但空結果」。回空結果會讓「尚未開通」長得像
// 「PayUni 說查無此商家」，賣家的申請會卡在 PENDING 永遠不動，而監控上什麼都看不到。
type disabledPayuniClient struct{}

func (disabledPayuniClient) BindSeller(context.Context, *usecase.BindSellerRequest) (*usecase.PayuniResponse, error) {
	return nil, usecase.ErrPayuniMerchantDisabled
}

func (disabledPayuniClient) GetSellerStatus(context.Context, string) (*usecase.SellerStatusResponse, error) {
	return nil, usecase.ErrPayuniMerchantDisabled
}

func (disabledPayuniClient) UpdateMerchant(context.Context, *usecase.UpdateMerchantRequest) (*usecase.PayuniResponse, error) {
	return nil, usecase.ErrPayuniMerchantDisabled
}

// provideGeocodeClient 接上 TGOS 地址查詢；未開通時回 nil（見 usecase 的 nil 守門）。
//
// 🔴 回傳型別刻意是 usecase.GeocodeClient（介面）而不是 *geocodeclient.Client ＋ fx.As：
// 回一個 nil 的具體指標再包成介面，得到的是 **typed-nil**——使用點的 `client != nil`
// 會是 true，fail-closed 那一格永遠走不到，症狀變成 nil pointer panic。
//
// 🔴 與 PayUni 用了不同的手法（nil vs 停用實作）是有理由的，不是不一致：
// GeocodeClient 只有兩個呼叫點且都在同一個 use case 裡，nil 守門看得完；
// PayuniClient 有十幾個散落的呼叫點，只能靠型別本身守。
//
// 兩個失敗點都在這裡收斂：env 沒設，以及 env 設了但 GCP SA 金鑰檔沒掛上
// （後者是比 env 更早的一道，#3329 盤點時曾被漏掉）。
func provideGeocodeClient(cfg *config.Config, log *zap.Logger) usecase.GeocodeClient {
	if cfg.GeocodePartiallyConfigured() {
		addressLookupConfigIncomplete.Set(1)
		addressLookupEnabled.Set(0)
		log.Error("TGOS 地址查詢設定只有一半，地址查詢已停用",
			zap.Bool("has_base_url", cfg.TGOSBaseURL != ""),
			zap.Bool("has_app_id", cfg.TGOSQueryAddrAppID != ""),
			zap.Bool("has_api_key", cfg.TGOSQueryAddrAPIKey != ""),
			zap.Bool("has_caller_key_file", cfg.TGOSProxyCallerKeyFile != ""),
			zap.String("impact", "SuggestAddress／ValidateAddress 回 9700；其餘 RPC 不受影響"))
		return nil
	}
	if !cfg.GeocodeConfigured() {
		addressLookupConfigIncomplete.Set(0)
		addressLookupEnabled.Set(0)
		log.Warn("TGOS 地址查詢尚未開通，SuggestAddress／ValidateAddress 將回 9700",
			zap.String("impact", "seller-service 其餘 RPC 正常服務"))
		return nil
	}
	tgos, err := pkggeocode.NewTGOSClient(context.Background(), cfg.TGOSBaseURL, cfg.TGOSQueryAddrAppID, cfg.TGOSQueryAddrAPIKey, cfg.TGOSProxyCallerKeyFile)
	if err != nil {
		addressLookupConfigIncomplete.Set(1)
		addressLookupEnabled.Set(0)
		log.Error("TGOS 憑證檔讀取失敗，地址查詢已停用",
			zap.String("key_file", cfg.TGOSProxyCallerKeyFile),
			zap.Error(err),
			zap.String("impact", "SuggestAddress／ValidateAddress 回 9700；其餘 RPC 不受影響"))
		return nil //nolint:nilerr // #3329：單一功能未開通不得拖垮整個服務
	}
	addressLookupConfigIncomplete.Set(0)
	addressLookupEnabled.Set(1)
	return geocodeclient.NewClient(tgos)
}
