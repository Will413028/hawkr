package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

// readPayment 讀收款開通的 provider：PAYMENT_PROVIDER 為空表示沒有設定（核准不建立收款帳戶，開通相關的方法回 failed_precondition），
// fake 是記憶體內的 provider（所有帳戶都回轉帳能力已啟用，只能在 requireDevEnvironment 成立的 dev 環境選到，否則啟動失敗），stripe 是 Stripe Connect（Accounts v2）；其他值讓啟動失敗。
// stripe 讀 STRIPE_SECRET_KEY（沒有設定時等於沒有 provider，serve 照常啟動）、STRIPE_WEBHOOK_SECRET（有金鑰卻沒有它，啟動失敗）與
// STRIPE_REQUIREMENT_COLLECTION（新建帳戶列的收件方式：provider 是 Stripe 收件，預設；platform 是平台收件；其他值啟動失敗。開通 session 的形狀由 ChooseSession 決定，不看這個設定）與
// STRIPE_PUBLISHABLE_KEY（內嵌 session 的公開金鑰，不是 secret；有金鑰卻沒有它，啟動失敗，與收件方式無關：既有的平台收件帳戶仍會開內嵌 session）。
func readPayment(e *config.Env, log *slog.Logger) payment.Provider {
	switch name := e.StringOr("PAYMENT_PROVIDER", ""); name {
	case "":
		log.Warn("收款開通沒有設定：核准後不會建立收款帳戶，店家不會通過開通閘門")
		return nil
	case "fake":
		if err := requireDevEnvironment(e); err != nil {
			e.Fail("PAYMENT_PROVIDER=fake 只能在 dev 環境選到：%v", err)
			return nil
		}
		fake := payment.NewFake()
		fake.AutoActive = true
		return fake
	case "stripe":
		return readStripe(e, log)
	default:
		e.Fail("PAYMENT_PROVIDER：不認得的值 %q（可用：fake、stripe）", name)
		return nil
	}
}

func readStripe(e *config.Env, log *slog.Logger) payment.Provider {
	var collection payment.RequirementCollection
	switch v := e.StringOr("STRIPE_REQUIREMENT_COLLECTION", "provider"); v {
	case "provider":
		collection = payment.CollectionProvider
	case "platform":
		collection = payment.CollectionPlatform
	default:
		e.Fail("STRIPE_REQUIREMENT_COLLECTION：不認得的值 %q（可用：provider、platform）", v)
		return nil
	}
	key := e.StringOr("STRIPE_SECRET_KEY", "")
	if key == "" {
		log.Warn("PAYMENT_PROVIDER=stripe 但沒有 STRIPE_SECRET_KEY：核准後不會建立收款帳戶，店家不會通過開通閘門")
		return nil
	}
	whsecret := e.StringOr("STRIPE_WEBHOOK_SECRET", "")
	if whsecret == "" {
		e.Fail("STRIPE_WEBHOOK_SECRET：設了 STRIPE_SECRET_KEY 就必填")
		return nil
	}
	pubkey := e.StringOr("STRIPE_PUBLISHABLE_KEY", "")
	if pubkey == "" {
		e.Fail("STRIPE_PUBLISHABLE_KEY：設了 STRIPE_SECRET_KEY 就必填")
		return nil
	}
	client, err := stripe.New(stripe.Config{SecretKey: key, WebhookSecret: whsecret})
	if err != nil {
		e.Fail("Stripe：%v", err)
		return nil
	}
	provider, err := payment.NewStripe(client, collection, pubkey)
	if err != nil {
		e.Fail("Stripe：%v", err)
		return nil
	}
	return provider
}

// sellerOwners 以 seller 與 identity 的公開 API 實作 payment 的擁有者查詢（payment 是葉節點，不讀上層）：店家存在而且沒有關閉才算營運中；
// 聯絡信箱是店家所屬帳號的聯絡信箱（identity 已驗證的信箱優先，其次自填，與送審檢查同一條路徑）。
// payment 先於 seller 組裝（seller 依賴 payment），所以 seller 在組裝之後才設定（bind）。
type sellerOwners struct{ m *seller.Module }

func (o *sellerOwners) bind(m *seller.Module) { o.m = m }

// OwnerContactEmail 回傳店家所屬帳號目前的聯絡信箱：由 seller 解析（與核准前的送審檢查同一個規則）；店家不存在或沒有可用的信箱時 ok 是 false。
func (o *sellerOwners) OwnerContactEmail(ctx context.Context, storeID int64) (string, bool, error) {
	return o.m.OwnerContactEmail(ctx, storeID)
}

func (o *sellerOwners) OwnerActive(ctx context.Context, storeID int64) (bool, error) {
	s, err := o.m.Store(ctx, storeID)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return s.Status != seller.Closed, nil
}

// newInbox 組裝 platform 的 webhook inbox（D29）。serve（入口寫入）與 worker（派送、補掃與清理）共用同一個組裝；模組在自己的 New 裡登記 provider。
func newInbox(d *db.DB, jc jobs.Enqueuer, log *slog.Logger) *inbox.Inbox {
	return inbox.New(inbox.Deps{DB: d, Jobs: jc, Now: time.Now, Logger: log})
}

// newPayment 組裝 payment 模組（Stripe 計畫 D28）。回傳的 sellerOwners 要在 seller 組裝之後 bind；PayoutAccountChanged 的訂閱者由
// subscribeSellerPayout 接上（serve 與 worker 都要：同步 job 在 worker 發布，seller 的種子在子命令發布）。
func newPayment(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, box *inbox.Inbox, provider payment.Provider, log *slog.Logger) (*payment.Module, *sellerOwners) {
	owners := &sellerOwners{}
	return payment.New(payment.Deps{DB: d, Events: bus, Jobs: jc, Inbox: box, Provider: provider, Owners: owners, Now: time.Now, Logger: log}), owners
}

// subscribeSellerPayout 讓 seller 在 payment 同步的 transaction 內更新開通閘門的副本與防詐 hash（Stripe 計畫 D12、D28）。seller 依賴 payment，
// 下層通知上層的接線只在這裡。
func subscribeSellerPayout(bus *events.Bus, module *seller.Module) {
	events.Subscribe(bus, module.OnPayoutAccountChanged)
}

// discardJobs 是種子用的排入：種子直接執行建立與同步，不把 job 排進 River（worker 若用別的 provider，不會去碰 fake 的帳戶）。
type discardJobs struct{}

func (discardJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

// seedPaymentAccounts 讓種子店家有「可以收款」的收款帳戶（Stripe 計畫 7d′）：以 payment 的 fake provider（AutoActive）走正式的核准寫入、建立與同步，
// 同步發的 PayoutAccountChanged 經 seller 的訂閱者寫入開通閘門的副本。聯絡信箱由建立時向店家所屬帳號讀（種子帳號的 identity 信箱，D50）。已有帳戶列的店家沿用（D19）；帳戶是別的 provider 的（例如本機
// 接過 Stripe sandbox）時 fake 不碰它，只印出提醒，那家店的閘門照該帳戶的狀態。
func seedPaymentAccounts(ctx context.Context, c *sellerCommand, fake *payment.Fake, storeIDs []int64, out io.Writer) error {
	jc := discardJobs{}
	m := payment.New(payment.Deps{DB: c.DB, Events: c.Bus, Jobs: jc, Inbox: newInbox(c.DB, jc, c.Logger), Provider: fake, Owners: c.Owners, Now: time.Now, Logger: c.Logger})
	for _, id := range storeIDs {
		if err := c.DB.InTx(ctx, func(ctx context.Context) error { return m.RequestAccount(ctx, id, time.Now().UTC()) }); err != nil {
			return fmt.Errorf("種子店家 %d 的收款帳戶：%w", id, err)
		}
		if err := m.EnsureAccount(ctx, id, false); err != nil {
			return fmt.Errorf("種子店家 %d 建立收款帳戶：%w", id, err)
		}
		if _, err := m.SyncAccount(ctx, id); err != nil {
			return fmt.Errorf("種子店家 %d 同步收款帳戶：%w", id, err)
		}
		acct, err := m.Account(ctx, id)
		if err != nil {
			return err
		}
		if acct.Provider != payment.FakeName {
			if _, err := fmt.Fprintf(out, "提醒：種子店家 %d 的收款帳戶屬於 %s，種子沒有改它（開通閘門照該帳戶的狀態）\n", id, acct.Provider); err != nil {
				return err
			}
			continue
		}
		if acct.Status != payment.StatusNotCreated {
			continue
		}
		if active, err := c.Owners.OwnerActive(ctx, id); err != nil {
			return err
		} else if active {
			return fmt.Errorf("種子店家 %d 的收款帳戶沒有外部帳戶（建立沒有完成）", id)
		}
	}
	return nil
}
