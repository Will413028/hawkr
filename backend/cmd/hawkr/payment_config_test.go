package main

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

func readPaymentWith(env map[string]string) (payment.Provider, error) {
	e := config.FromMap(env)
	p := readPayment(e, slog.New(slog.DiscardHandler))
	return p, e.Err()
}

func TestPaymentProviderConfig(t *testing.T) {
	if p, err := readPaymentWith(nil); p != nil || err != nil {
		t.Errorf("沒有設定：%v %v", p, err)
	}
	if p, err := readPaymentWith(map[string]string{"PAYMENT_PROVIDER": "fake"}); p == nil || err != nil {
		t.Errorf("fake：%v %v", p, err)
	}
	if _, err := readPaymentWith(map[string]string{"PAYMENT_PROVIDER": "unsupported"}); err == nil || !strings.Contains(err.Error(), "PAYMENT_PROVIDER") {
		t.Errorf("不認得的 provider：%v", err)
	}
}

func TestStripePaymentConfig(t *testing.T) {
	full := map[string]string{"PAYMENT_PROVIDER": "stripe", "STRIPE_SECRET_KEY": "sk_test_x", "STRIPE_WEBHOOK_SECRET": "whsec_x"}
	with := func(kv ...string) map[string]string {
		m := map[string]string{}
		for k, v := range full {
			m[k] = v
		}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}

	p, err := readPaymentWith(full)
	if err != nil || p == nil || p.Name() != "stripe" {
		t.Fatalf("預設：%v %v", p, err)
	}
	if c := p.Capabilities(); c.RequirementCollection != payment.CollectionProvider || c.OnboardingSurface != payment.SurfaceHosted {
		t.Errorf("預設要是 Stripe 收件加託管頁面：%+v", c)
	}
	p, err = readPaymentWith(with("STRIPE_REQUIREMENT_COLLECTION", "platform"))
	if err != nil || p == nil {
		t.Fatalf("platform：%v %v", p, err)
	}
	if c := p.Capabilities(); c.RequirementCollection != payment.CollectionPlatform || c.OnboardingSurface != payment.SurfaceEmbedded {
		t.Errorf("platform 要是平台收件加內嵌元件：%+v", c)
	}
	if _, err := readPaymentWith(with("STRIPE_REQUIREMENT_COLLECTION", "both")); err == nil || !strings.Contains(err.Error(), "STRIPE_REQUIREMENT_COLLECTION") {
		t.Errorf("不認得的收件方式：%v", err)
	}
	if p, err := readPaymentWith(map[string]string{"PAYMENT_PROVIDER": "stripe", "STRIPE_WEBHOOK_SECRET": "whsec_x"}); p != nil || err != nil {
		t.Errorf("缺金鑰要等於沒有 provider：%v %v", p, err)
	}
	if p, err := readPaymentWith(map[string]string{"PAYMENT_PROVIDER": "stripe"}); p != nil || err != nil {
		t.Errorf("什麼都沒設：%v %v", p, err)
	}
	if _, err := readPaymentWith(map[string]string{"PAYMENT_PROVIDER": "stripe", "STRIPE_SECRET_KEY": "sk_test_x"}); err == nil || !strings.Contains(err.Error(), "STRIPE_WEBHOOK_SECRET") {
		t.Errorf("有金鑰沒有 webhook 密鑰：%v", err)
	}
}
