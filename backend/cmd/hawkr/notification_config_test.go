package main

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
)

func readProviders(m map[string]string) (notificationProviders, error) {
	e := config.FromMap(m)
	p := readNotificationProviders(context.Background(), e, slog.New(slog.DiscardHandler))
	return p, e.Err()
}

// 沒有設定的 provider 用替身，serve 與 worker 照常啟動。
func TestNotificationProvidersDefaultToFakes(t *testing.T) {
	p, err := readProviders(map[string]string{})
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
		if _, err := readProviders(tc.env); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s：err = %v，預期含 %q", name, err, tc.want)
		}
	}
	p, err := readProviders(map[string]string{"RESEND_API_KEY": "k", "RESEND_FROM": "Hawkr <noreply@mail.example>", "RESEND_WEBHOOK_SECRET": "whsec_x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.mail.(*resend.Client); !ok || p.webhookSecret != "whsec_x" {
		t.Fatalf("mail = %T secret = %q", p.mail, p.webhookSecret)
	}
}
