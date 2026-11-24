package main

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
)

func readProviders(m map[string]string) (notificationProviders, error) {
	e := config.FromMap(m)
	p := readNotificationProviders(context.Background(), e, slog.New(slog.DiscardHandler))
	return p, e.Err()
}

// 沒有設定的 provider 只在 dev 環境用替身；其他環境（沒有 dev 金鑰、或與 Cloudflare Access 並存）啟動失敗，替身不會在正式環境把投遞標成 SENT。
func TestNotificationProviderFakesOnlyInDevEnvironment(t *testing.T) {
	keyFile := writeDevKey(t)
	for name, env := range map[string]map[string]string{
		"沒有 dev 金鑰":         {},
		"dev 金鑰與 Access 並存": {"DEV_AUTH_KEY_FILE": keyFile, "CF_ACCESS_TEAM_DOMAIN": "https://team.cloudflareaccess.com", "CF_ACCESS_ADMIN_AUD": "aud"},
	} {
		_, err := readProviders(env)
		if err == nil || !strings.Contains(err.Error(), "FCM_PROJECT_ID") || !strings.Contains(err.Error(), "RESEND_API_KEY") {
			t.Errorf("%s：err = %v，預期推播與 email 都讓啟動失敗", name, err)
		}
	}
	p, err := readProviders(map[string]string{"DEV_AUTH_KEY_FILE": keyFile})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.push.(*fcm.Fake); !ok {
		t.Fatalf("push = %T，預期替身", p.push)
	}
	if _, ok := p.mail.(*resend.Fake); !ok {
		t.Fatalf("mail = %T，預期替身", p.mail)
	}
}

// 金鑰與其必要的搭配設定要一起給，只給一半啟動失敗；Resend 兩個都給時用真的 client。
func TestNotificationProvidersRequireTheirSettingsTogether(t *testing.T) {
	dev := map[string]string{"DEV_AUTH_KEY_FILE": writeDevKey(t)}
	with := func(m map[string]string) map[string]string {
		for k, v := range dev {
			m[k] = v
		}
		return m
	}
	for name, tc := range map[string]struct {
		env  map[string]string
		want string
	}{
		"FCM 只有專案":     {map[string]string{"FCM_PROJECT_ID": "p"}, "FCM_PROJECT_ID"},
		"FCM 只有金鑰":     {map[string]string{"FCM_SERVICE_ACCOUNT_JSON": "{}"}, "FCM_SERVICE_ACCOUNT_JSON"},
		"FCM 金鑰格式不合":   {map[string]string{"FCM_PROJECT_ID": "p", "FCM_SERVICE_ACCOUNT_JSON": "nope"}, "FCM"},
		"Resend 只有金鑰":  {map[string]string{"RESEND_API_KEY": "k"}, "RESEND_API_KEY"},
		"Resend 只有寄件人": {map[string]string{"RESEND_FROM": "a@b.example"}, "RESEND_FROM"},
	} {
		if _, err := readProviders(with(tc.env)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s：err = %v，預期含 %q", name, err, tc.want)
		}
	}
	p, err := readProviders(with(map[string]string{"RESEND_API_KEY": "k", "RESEND_FROM": "Hawkr <noreply@mail.example>", "RESEND_WEBHOOK_SECRET": "whsec_x"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.mail.(*resend.Client); !ok || p.webhookSecret != "whsec_x" {
		t.Fatalf("mail = %T secret = %q", p.mail, p.webhookSecret)
	}
}

// 推播圖片的網址與 API 回應同一個 MEDIA_ORIGIN，preset 是推播專用的（媒體網域由 rpc.NewImages 驗證，零值不會到這裡）。
func TestPushImageURLUsesTheMediaOriginAndPushPreset(t *testing.T) {
	images, err := rpc.NewImages("https://media.example")
	if err != nil {
		t.Fatal(err)
	}
	f := pushImageURL(images)
	if f == nil {
		t.Fatal("有媒體網域時要有 ImageURL")
	}
	if got, want := f("public/product/p1/a.jpg"), "https://media.example/i/"+rpc.PushImagePreset+"/public/product/p1/a.jpg"; got != want {
		t.Fatalf("ImageURL = %q，預期 %q", got, want)
	}
}

// suppression 名單的 HMAC 密鑰：dev 環境沒設用固定 dev 密鑰；其他環境沒設啟動失敗；太短、舊密鑰與目前相同、只設舊密鑰都失敗；兩把都合格時帶進去。
func TestSuppressionKeysConfig(t *testing.T) {
	dev := map[string]string{"DEV_AUTH_KEY_FILE": writeDevKey(t)}
	with := func(kv ...string) map[string]string {
		m := map[string]string{}
		for k, v := range dev {
			m[k] = v
		}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	p, err := readProviders(with())
	if err != nil || string(p.suppression.Current) != devSuppressionKey || len(p.suppression.Previous) != 0 {
		t.Fatalf("dev 預設：%+v %v", p.suppression, err)
	}
	if _, err := readProviders(map[string]string{}); err == nil || !strings.Contains(err.Error(), "EMAIL_SUPPRESSION_HMAC_KEY") {
		t.Fatalf("非 dev 沒有密鑰必須失敗：%v", err)
	}
	k1, k2 := strings.Repeat("a", 32), strings.Repeat("b", 32)
	for name, env := range map[string]map[string]string{
		"太短":       with("EMAIL_SUPPRESSION_HMAC_KEY", "short"),
		"舊密鑰太短":    with("EMAIL_SUPPRESSION_HMAC_KEY", k1, "EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS", "short"),
		"舊密鑰與目前相同": with("EMAIL_SUPPRESSION_HMAC_KEY", k1, "EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS", k1),
		"只有舊密鑰":    with("EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS", k2),
	} {
		if _, err := readProviders(env); err == nil || !strings.Contains(err.Error(), "EMAIL_SUPPRESSION_HMAC_KEY") {
			t.Errorf("%s：err = %v", name, err)
		}
	}
	p, err = readProviders(with("EMAIL_SUPPRESSION_HMAC_KEY", k1, "EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS", k2))
	if err != nil || string(p.suppression.Current) != k1 || string(p.suppression.Previous) != k2 {
		t.Fatalf("輪替：%+v %v", p.suppression, err)
	}
}
