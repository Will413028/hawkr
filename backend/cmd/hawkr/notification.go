package main

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
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

// notificationProviders 是推播與 email 的 provider 設定。沒有設定的 provider 只在 dev 環境（requireDevEnvironment 成立）用替身：替身只記一行 log、回成功，
// 在其他環境會把投遞紀錄誤標為 SENT，所以沒有設定就讓啟動失敗。
type notificationProviders struct {
	push          fcm.Sender
	mail          resend.Sender
	webhookSecret string
	suppression   notification.SuppressionKeys
}

// devSuppressionKey 是 dev 環境沒有設定 EMAIL_SUPPRESSION_HMAC_KEY 時的固定密鑰（只在 requireDevEnvironment 成立時選到；不是 secret）。
const devSuppressionKey = "hawkr-dev-email-suppression-hmac-key"

// readNotificationProviders 讀推播與 email 的設定：
// FCM_PROJECT_ID 與 FCM_SERVICE_ACCOUNT_JSON（服務帳戶金鑰的內容，不是路徑；secret）要一起設，只設一個啟動失敗，都沒設時 dev 環境用替身、其他環境啟動失敗；
// RESEND_API_KEY（secret）與 RESEND_FROM（寄件人，例如 `Hawkr <noreply@mail.example.com>`）要一起設，只設一個啟動失敗，都沒設時 dev 環境用替身、其他環境啟動失敗；
// RESEND_WEBHOOK_SECRET（退信與投訴 webhook 的 Svix 簽章密鑰，secret；`whsec_` 開頭）獨立：沒設時 webhook 回 503；
// EMAIL_SUPPRESSION_HMAC_KEY（email suppression 名單的位址雜湊密鑰，secret，至少 32 個字元）沒設時 dev 環境用固定的 dev 密鑰、其他環境啟動失敗；
// EMAIL_SUPPRESSION_HMAC_KEY_PREVIOUS（輪替期間的舊密鑰，同樣至少 32 個字元，不得與目前的相同）只用於查詢，沒有 EMAIL_SUPPRESSION_HMAC_KEY 時不得設。
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

// newNotification 組裝 notification 模組；serve 與 worker 都要建：serve 訂閱來源事件並排意圖 job，worker 執行意圖 job 與渠道 job。
// box 是 webhook inbox：模組在這裡登記 Resend 的退信與投訴事件；images 組推播圖片的絕對網址（與 API 回應同一個 MEDIA_ORIGIN 與 URI 樣板，規格用 rpc.PushImagePreset）。
// 帳號語系讀 identity（preferred_locale，多語系 ADR D1）。
func newNotification(d *db.DB, id *identity.Module, b *buyer.Module, sl *seller.Module, jc jobs.Enqueuer, box *inbox.Inbox, p notificationProviders, images rpc.Images, log *slog.Logger) *notification.Module {
	return notification.New(notification.Deps{
		DB: d, Identity: id, Buyer: b, Seller: sl, Directory: notification.NewDirectory(b, sl), Resolver: notification.NewResolver(b, sl), Jobs: jc, Now: time.Now, NewID: uuid.NewV7, Logger: log,
		Contacts: id, Locales: id, Push: p.push, Mail: p.mail, ImageURL: pushImageURL(images), SuppressionKeys: p.suppression, Inbox: box, ResendWebhookSecret: p.webhookSecret,
	})
}

// subscribeNotification 讓來源事件在發布的 transaction 內排成通知意圖 job（D10），刪除與退場事件排成個資清除 job（D16）；
// 停權與刪帳號另在同一個 transaction 刪掉帳號的裝置 token（唯一不只排 job 的訂閱者，通知 ADR 決策 4）。
func subscribeNotification(bus *events.Bus, m *notification.Module) {
	events.Subscribe[accountsuspension.Suspended](bus, m.OnSuspended)
	events.Subscribe[accountsuspension.Suspended](bus, m.PurgeTokensOnSuspended)
	events.Subscribe[accountsuspension.Unsuspended](bus, m.OnUnsuspended)
	events.Subscribe[identity.AccountDeleted](bus, m.OnAccountDeleted)
	events.Subscribe[identity.AccountDeleted](bus, m.PurgeTokensOnAccountDeleted)
	events.Subscribe[buyer.BuyerDeleted](bus, m.OnBuyerDeleted)
	events.Subscribe[seller.StoreRetired](bus, m.OnStoreRetired)
}

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

// pushImageURL 把型別的圖片 object key 組成推播用的絕對網址；媒體網域由 rpc.NewImages 驗證過，啟動時一定有。
func pushImageURL(images rpc.Images) func(key string) string {
	return func(key string) string { return images.URL(key, rpc.PushImagePreset) }
}
