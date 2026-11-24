package main

import (
	"context"
	"log/slog"
	"time"
	"uuid"

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
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// notificationProviders 是推播與 email 的 provider 設定。沒有設定的 provider 用替身：只記一行 log、不送出，所以本機與還沒開 Firebase、Resend 帳號的環境照常啟動。
type notificationProviders struct {
	push          fcm.Sender
	mail          resend.Sender
	webhookSecret string
}

// readNotificationProviders 讀推播與 email 的設定：
// FCM_PROJECT_ID 與 FCM_SERVICE_ACCOUNT_JSON（服務帳戶金鑰的內容，不是路徑；secret）要一起設，只設一個啟動失敗，都沒設用替身；
// RESEND_API_KEY（secret）與 RESEND_FROM（寄件人，例如 `Hawkr <noreply@mail.example.com>`）要一起設，只設一個啟動失敗，都沒設用替身；
// RESEND_WEBHOOK_SECRET（退信與投訴 webhook 的 Svix 簽章密鑰，secret；`whsec_` 開頭）獨立：沒設時 webhook 回 503。
func readNotificationProviders(ctx context.Context, e *config.Env, log *slog.Logger) notificationProviders {
	p := notificationProviders{webhookSecret: e.StringOr("RESEND_WEBHOOK_SECRET", "")}
	project, saJSON := e.StringOr("FCM_PROJECT_ID", ""), e.StringOr("FCM_SERVICE_ACCOUNT_JSON", "")
	switch {
	case project == "" && saJSON == "":
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
	key, from := e.StringOr("RESEND_API_KEY", ""), e.StringOr("RESEND_FROM", "")
	switch {
	case key == "" && from == "":
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

// accountLocales 是帳號 preferred_locale 的來源。identity 還沒有 preferred_locale 欄位（多語系 ADR D1 規定由 identity 擁有、通知讀它），
// 在它建好之前所有帳號是預設語系；改成讀 identity 時只換這一個型別。
type accountLocales struct{}

func (accountLocales) PreferredLocale(context.Context, int64) (uitext.Locale, error) {
	return uitext.DefaultLocale, nil
}

// newNotification 組裝 notification 模組；serve 與 worker 都要建：serve 訂閱來源事件並排意圖 job，worker 執行意圖 job 與渠道 job。
// box 是 webhook inbox：模組在這裡登記 Resend 的退信與投訴事件。
func newNotification(d *db.DB, id *identity.Module, b *buyer.Module, sl *seller.Module, jc jobs.Enqueuer, box *inbox.Inbox, p notificationProviders, log *slog.Logger) *notification.Module {
	return notification.New(notification.Deps{
		DB: d, Identity: id, Buyer: b, Seller: sl, Directory: notification.NewDirectory(b, sl), Jobs: jc, Now: time.Now, NewID: uuid.NewV7, Logger: log,
		Contacts: id, Locales: accountLocales{}, Push: p.push, Mail: p.mail, Inbox: box, ResendWebhookSecret: p.webhookSecret,
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
