package client

import (
	"context"
	"errors"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

// backend#3329：seller-service 兩個選填依賴的接線契約。

// 🔴 PayuniClient 未開通時回的是**停用實作**而不是 nil：三支方法都必須回 9704。
//
// 為什麼這比 nil 守門重要：PayuniClient 在 usecase 有十幾個散落的呼叫點，
// nil 守門只要有一個新呼叫點忘了加就是 nil pointer panic，而沒有任何 gate 攔得住
// 「忘了加」。把 fail-closed 放進型別本身，新呼叫點自動拿到 9704。
func TestProvidePayuniClient_DisabledFailsClosed(t *testing.T) {
	payuniMerchantEnabled.Set(-1)
	payuniMerchantConfigIncomplete.Set(-1)

	c := providePayuniClient(&config.Config{}, zap.NewNop())
	if c == nil {
		t.Fatal("未開通時必須回停用實作而不是 nil——nil 會在第一個呼叫點 panic")
	}
	if v := testutil.ToFloat64(payuniMerchantEnabled); v != 0 {
		t.Fatalf("seller_payuni_merchant_enabled 應為 0，實際 %v", v)
	}
	if v := testutil.ToFloat64(payuniMerchantConfigIncomplete); v != 0 {
		t.Fatalf("兩者皆空＝尚未開通，不是配置錯誤，incomplete 應為 0，實際 %v", v)
	}

	// 三支全部都要擋——漏一支就是一次帶著空金鑰打出去的真實 PayUni 呼叫。
	_, err := c.BindSeller(context.Background(), &usecase.BindSellerRequest{})
	assertClientBizCode(t, err, 9704, "BindSeller")
	_, err = c.GetSellerStatus(context.Background(), "HAWK123")
	assertClientBizCode(t, err, 9704, "GetSellerStatus")
	_, err = c.UpdateMerchant(context.Background(), &usecase.UpdateMerchantRequest{})
	assertClientBizCode(t, err, 9704, "UpdateMerchant")
}

// 「只有一半」與「尚未開通」必須分得出來：前者要有人去修，後者不必理會。
func TestProvidePayuniClient_PartialIsMisconfiguration(t *testing.T) {
	payuniMerchantEnabled.Set(-1)
	payuniMerchantConfigIncomplete.Set(-1)

	cfg := &config.Config{}
	cfg.Payuni.HashKey = "12345678901234567890123456789012"
	// HashIV 刻意留空

	c := providePayuniClient(cfg, zap.NewNop())
	if _, err := c.BindSeller(context.Background(), &usecase.BindSellerRequest{}); err == nil {
		t.Fatal("憑證只有一半時必須 fail-closed，不得帶著半套金鑰打 PayUni")
	}
	if v := testutil.ToFloat64(payuniMerchantConfigIncomplete); v != 1 {
		t.Fatalf("只有一半＝配置錯誤，incomplete 應為 1，實際 %v", v)
	}
	if v := testutil.ToFloat64(payuniMerchantEnabled); v != 0 {
		t.Fatalf("seller_payuni_merchant_enabled 應為 0，實際 %v", v)
	}
}

// 陽性對照：憑證齊全時必須回真正的 client，不能被守門誤擋成永遠停用。
func TestProvidePayuniClient_EnabledReturnsRealClient(t *testing.T) {
	payuniMerchantEnabled.Set(-1)
	cfg := &config.Config{}
	cfg.Payuni.AgentID = "AGENT"
	cfg.Payuni.HashKey = "12345678901234567890123456789012"
	cfg.Payuni.HashIV = "1234567890123456"

	c := providePayuniClient(cfg, zap.NewNop())
	if _, ok := c.(disabledPayuniClient); ok {
		t.Fatal("憑證齊全時不得回停用實作")
	}
	if v := testutil.ToFloat64(payuniMerchantEnabled); v != 1 {
		t.Fatalf("seller_payuni_merchant_enabled 應為 1，實際 %v", v)
	}
}

// 🔴 GeocodeClient 未開通時必須是 untyped nil：typed-nil 會讓 usecase 的
// `u.client == nil` 恆為 false，fail-closed 退化成 nil pointer panic。
func TestProvideGeocodeClient_UnprovisionedReturnsUntypedNil(t *testing.T) {
	addressLookupEnabled.Set(-1)
	addressLookupConfigIncomplete.Set(-1)

	got := provideGeocodeClient(&config.Config{}, zap.NewNop())
	if got != nil {
		t.Fatalf("未開通時必須回 untyped nil，實際 %#v", got)
	}
	if v := testutil.ToFloat64(addressLookupEnabled); v != 0 {
		t.Fatalf("seller_address_lookup_enabled 應為 0，實際 %v", v)
	}
	if v := testutil.ToFloat64(addressLookupConfigIncomplete); v != 0 {
		t.Fatalf("四項全空＝尚未開通，incomplete 應為 0，實際 %v", v)
	}
}

// env 齊全但金鑰檔讀不到 —— #3329 盤點時漏掉的第二個失敗點（舊行為是 fx abort）。
func TestProvideGeocodeClient_MissingKeyFileDegrades(t *testing.T) {
	addressLookupEnabled.Set(-1)
	addressLookupConfigIncomplete.Set(-1)

	got := provideGeocodeClient(&config.Config{
		TGOSBaseURL:            "https://proxy.example",
		TGOSQueryAddrAppID:     "app",
		TGOSQueryAddrAPIKey:    "key",
		TGOSProxyCallerKeyFile: "/nonexistent/backend-3329/key.json",
	}, zap.NewNop())
	if got != nil {
		t.Fatalf("金鑰檔讀不到時必須降級為停用（回 nil），而不是讓整個服務起不來，實際 %#v", got)
	}
	if v := testutil.ToFloat64(addressLookupConfigIncomplete); v != 1 {
		t.Fatalf("env 齊全卻讀不到金鑰檔＝配置錯誤，incomplete 應為 1，實際 %v", v)
	}
}

func assertClientBizCode(t *testing.T, err error, wantCode int, what string) {
	t.Helper()
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) {
		t.Fatalf("%s 必須回帶業務碼的 DomainError，實際 %T：%v", what, err, err)
	}
	if got := de.DomainCode(); got != wantCode {
		t.Fatalf("%s 業務碼應為 %d，實際 %d", what, wantCode, got)
	}
}
