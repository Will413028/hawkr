package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// 守 #3269：平台憑證只有一半時，barcode verifier 會靜默停用——這支測試要求它**不靜默**。
//
// ⚠️ 三個 case 不能並行：gauge 是 package-level 單例，並行會互相覆寫。
func TestNewPreference_PlatformCredentialSignal(t *testing.T) {
	cases := []struct {
		name         string
		invoice      string
		appKey       string
		wantGauge    float64
		wantM34      float64
		wantVerifier bool
		wantErrLog   bool
	}{
		{"兩半都在：接上且無訊號", "12345678", "key", 0, 1, true, false},
		{"兩半都缺：功能未啟用，不是缺陷", "", "", 0, 0, false, false},
		{"只有統編：這才是缺陷", "12345678", "", 1, 0, false, true},
		{"只有 AppKey：反向也算缺陷", "", "key", 1, 0, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			platformCredentialIncomplete.Set(-1)
			mobileBarcodeVerifierEnabled.Set(-1)

			core, logs := observer.New(zap.ErrorLevel)
			cfg := &config.Config{}
			cfg.Amego.PlatformInvoice = c.invoice
			cfg.Amego.PlatformAppKey = c.appKey

			u := newPreference(nil, nil, nil, nil, cfg, nil, zap.New(core))

			if got := testutil.ToFloat64(platformCredentialIncomplete); got != c.wantGauge {
				t.Errorf("gauge = %v，預期 %v", got, c.wantGauge)
			}
			if gotV := u.barcode != nil; gotV != c.wantVerifier {
				t.Errorf("verifier 掛上 = %v，預期 %v", gotV, c.wantVerifier)
			}
			if got := testutil.ToFloat64(mobileBarcodeVerifierEnabled); got != c.wantM34 {
				t.Errorf("M34 gauge = %v，預期 %v", got, c.wantM34)
			}
			if gotL := logs.Len() > 0; gotL != c.wantErrLog {
				t.Errorf("ERROR log = %v，預期 %v", gotL, c.wantErrLog)
			}
			if c.wantErrLog {
				if msg := logs.All()[0].Message; msg == "" {
					t.Error("ERROR log 訊息不得為空")
				}
				for _, fld := range logs.All()[0].Context {
					if fld.String == "key" || fld.String == c.appKey && c.appKey != "" {
						t.Errorf("AppKey 洩漏進 log 欄位 %q", fld.Key)
					}
				}
			}
		})
	}
}
