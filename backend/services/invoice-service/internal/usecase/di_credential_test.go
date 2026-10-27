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
		// wantM34 一律寫字面值，**不得**由 wantVerifier 推導——期望值跟著被測物走就驗不到任何東西。
		{"兩半都在：接上且無訊號", "12345678", "key", 0, 1, true, false},
		{"兩半都缺：功能未啟用，不是缺陷", "", "", 0, 0, false, false},
		{"只有統編：這才是缺陷", "12345678", "", 1, 0, false, true},
		{"只有 AppKey：反向也算缺陷", "", "key", 1, 0, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 🔴 兩支 gauge 都先設成哨兵值。原本靠「前一個 case 的殘留值」抓漏 Set，
			// 但那依賴 case 順序：本表 case 2 與 case 3 的 wantM34 都是 0，
			// 拿掉 case 3 那格的 Set(0) 時殘留恰好＝期望值 ⇒ 變異不轉紅（實測 exit=0）。
			// platformCredentialIncomplete 同樣有這個盲點（case 1／2 的 wantGauge 都是 0）。
			// -1 不可能是任何一格的合法值 ⇒ 漏 Set 一律轉紅，且不依賴 case 排列。
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
			// 🔴 這一條與上一條**不等價**，不要因為看起來重複而刪掉（#3230 R5 F1）：
			// 上面讀的是 u.barcode（物件掛上沒有），這裡讀的是 di.go 有沒有真的呼叫 Set。
			// 漏掉某一格的 Set(0) 時 u.barcode 照樣正確，而 gauge 會殘留前一個 case 的值——
			// gauge 是 package-level 單例且 case 循序執行，殘留正是抓漏的機制（見上方 ⚠️）。
			//
			// 為什麼非補不可：告警 InvoiceMobileBarcodeVerifierDisabled 的觸發條件是
			// verifier==0 and platform_credential_incomplete==0，也就是本表第二列
			// 「兩半都缺」——而那正是 prod 部署後會落的那一格。沒有這一條，
			// 就是一道告警建立在零覆蓋的分支上。
			if got := testutil.ToFloat64(mobileBarcodeVerifierEnabled); got != c.wantM34 {
				t.Errorf("M34 gauge = %v，預期 %v", got, c.wantM34)
			}
			// 🔴 光有 gauge 不夠：gauge 要有人建告警才看得到，而 log 是值班第一手。
			if gotL := logs.Len() > 0; gotL != c.wantErrLog {
				t.Errorf("ERROR log = %v，預期 %v", gotL, c.wantErrLog)
			}
			if c.wantErrLog {
				if msg := logs.All()[0].Message; msg == "" {
					t.Error("ERROR log 訊息不得為空")
				}
				// 沒有這一條，日後有人把 AppKey 加進 log 欄位不會被發現。
				for _, fld := range logs.All()[0].Context {
					if fld.String == "key" || fld.String == c.appKey && c.appKey != "" {
						t.Errorf("AppKey 洩漏進 log 欄位 %q", fld.Key)
					}
				}
			}
		})
	}
}
