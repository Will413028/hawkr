package main

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
)

type notificationProviders struct {
	push          fcm.Sender
	mail          resend.Sender
	webhookSecret string
	suppression   notification.SuppressionKeys
}

const devSuppressionKey = "hawkr-dev-email-suppression-hmac-key"

func readNotificationProviders(ctx context.Context, e *config.Env, log *slog.Logger) notificationProviders {
	p := notificationProviders{webhookSecret: e.StringOr("RESEND_WEBHOOK_SECRET", "")}
	project, saJSON := e.StringOr("FCM_PROJECT_ID", ""), e.StringOr("FCM_SERVICE_ACCOUNT_JSON", "")
	switch {
	case project == "" && saJSON == "":
		if err := requireDevEnvironment(e); err != nil {
			e.Fail("FCM_PROJECT_ID 與 FCM_SERVICE_ACCOUNT_JSON 沒有設定：替身只能在 dev 環境選到：%v", err)
			break
		}
		log.Warn("推播沒有設定 FCM：推播只記 log、不送出")
		p.push = fcm.NewFake(log)
	case project == "" || saJSON == "":
		e.Fail("FCM_PROJECT_ID 與 FCM_SERVICE_ACCOUNT_JSON 要一起設定")
	default:
		c, err := fcm.New(ctx, fcm.Config{ProjectID: project, ServiceAccountJSON: []byte(saJSON)})
		if err != nil {
			e.Fail("FCM：%v", err)
		} else {
			p.push = c
		}
	}
	p.suppression = readSuppressionKeys(e)
	key, from := e.StringOr("RESEND_API_KEY", ""), e.StringOr("RESEND_FROM", "")
	switch {
	case key == "" && from == "":
		if err := requireDevEnvironment(e); err != nil {
			e.Fail("RESEND_API_KEY 與 RESEND_FROM 沒有設定：替身只能在 dev 環境選到：%v", err)
			break
		}
		log.Warn("email 沒有設定 Resend：email 只記 log、不寄出")
		p.mail = resend.NewFake(log)
	case key == "" || from == "":
		e.Fail("RESEND_API_KEY 與 RESEND_FROM 要一起設定")
	default:
		c, err := resend.New(resend.Config{APIKey: key, From: from})
		if err != nil {
			e.Fail("Resend：%v", err)
		} else {
			p.mail = c
		}
	}
	return p
}

func newNotification(d *db.DB, id *identity.Module, b *buyer.Module, sl *seller.Module, coupons *coupon.Module, reporters notification.CaseReporters, jc jobs.Enqueuer, box *inbox.Inbox, p notificationProviders, images rpc.Images, log *slog.Logger) *notification.Module {
	return notification.New(notification.Deps{
		DB: d, Identity: id, Buyer: b, Seller: sl, Directory: notification.NewDirectory(b, sl), Resolver: notification.NewResolver(b, sl, reporters), Validity: notification.NewValidity(coupons), StoreCodes: notification.NewStoreCodes(sl), Jobs: jc, Now: time.Now, NewID: uuid.NewV7, Logger: log,
		Contacts: id, Locales: id, Push: p.push, Mail: p.mail, ImageURL: pushImageURL(images), SuppressionKeys: p.suppression, Inbox: box, ResendWebhookSecret: p.webhookSecret,
	})
}

func subscribeNotification(bus *events.Bus, m *notification.Module) { m.Subscribe(bus) }

func readSuppressionKeys(e *config.Env) notification.SuppressionKeys {
	cur, prev := e.StringOr("EMAIL_SUPPRESSION_HMAC_KEY", ""), e.StringOr("EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS", "")
	if cur == "" {
		if prev != "" {
			e.Fail("EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS 要與 EMAIL_SUPPRESSION_HMAC_KEY 一起設定")
			return notification.SuppressionKeys{}
		}
		if err := requireDevEnvironment(e); err != nil {
			e.Fail("EMAIL_SUPPRESSION_HMAC_KEY 沒有設定：固定的 dev 密鑰只能在 dev 環境選到：%v", err)
			return notification.SuppressionKeys{}
		}
		return notification.SuppressionKeys{Current: []byte(devSuppressionKey)}
	}
	if len(cur) < notification.MinSuppressionKeyBytes || (prev != "" && len(prev) < notification.MinSuppressionKeyBytes) {
		e.Fail("EMAIL_SUPPRESSION_HMAC_KEY 與 EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS 至少 %d 個字元", notification.MinSuppressionKeyBytes)
		return notification.SuppressionKeys{}
	}
	if prev == cur {
		e.Fail("EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS 不得與 EMAIL_SUPPRESSION_HMAC_KEY 相同")
		return notification.SuppressionKeys{}
	}
	return notification.SuppressionKeys{Current: []byte(cur), Previous: []byte(prev)}
}

func pushImageURL(images rpc.Images) func(key string) string {
	return func(key string) string { return images.URL(key, rpc.PushImagePreset) }
}
