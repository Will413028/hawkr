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
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

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
	namespace := e.StringOr("STRIPE_ACCOUNT_NAMESPACE", "")
	if namespace == "" {
		e.Fail("STRIPE_ACCOUNT_NAMESPACE：設了 STRIPE_SECRET_KEY 就必填")
		return nil
	}
	if err := stripe.ValidateNamespace(namespace); err != nil {
		e.Fail("STRIPE_ACCOUNT_NAMESPACE：%v", err)
		return nil
	}
	client, err := stripe.New(stripe.Config{SecretKey: key, WebhookSecret: whsecret, AccountNamespace: namespace, Logger: log})
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

func readCharge(e *config.Env, log *slog.Logger) payment.ChargeProvider {
	switch name := e.StringOr("PAYMENT_CHARGE_PROVIDER", ""); name {
	case "":
		log.Warn("付款沒有設定：付款的呼叫會回 failed_precondition，付款 webhook 回 503")
		return nil
	case "stripe":
		cfg, ok := readChargeStripeConfig(e, log)
		if !ok {
			return nil
		}
		client, err := stripe.New(stripe.Config{SecretKey: cfg.SecretKey, WebhookSecret: cfg.WebhookSecret, AccountNamespace: cfg.Namespace, Logger: log})
		if err != nil {
			e.Fail("Stripe：%v", err)
			return nil
		}
		provider, err := payment.NewChargeStripe(client)
		if err != nil {
			e.Fail("Stripe：%v", err)
			return nil
		}
		return provider
	default:
		e.Fail("PAYMENT_CHARGE_PROVIDER：不認得的值 %q（可用：stripe）", name)
		return nil
	}
}

type chargeStripeConfig struct {
	SecretKey     string
	WebhookSecret string
	Namespace     string
}

func readChargeStripeConfig(e *config.Env, log *slog.Logger) (cfg chargeStripeConfig, ok bool) {
	cfg.SecretKey = e.StringOr("STRIPE_SECRET_KEY", "")
	if cfg.SecretKey == "" {
		log.Warn("PAYMENT_CHARGE_PROVIDER=stripe 但沒有 STRIPE_SECRET_KEY：付款沒有設定")
		return cfg, false
	}
	cfg.WebhookSecret = e.StringOr("STRIPE_PAYMENT_WEBHOOK_SECRET", "")
	if cfg.WebhookSecret == "" {
		e.Fail("STRIPE_PAYMENT_WEBHOOK_SECRET：付款設了 STRIPE_SECRET_KEY 就必填（付款事件的 event destination 有自己的簽章密鑰）")
		return cfg, false
	}
	if cfg.WebhookSecret == e.StringOr("STRIPE_WEBHOOK_SECRET", "") {
		e.Fail("STRIPE_PAYMENT_WEBHOOK_SECRET 不能與 STRIPE_WEBHOOK_SECRET 相同：兩個 event destination 各有自己的簽章密鑰")
		return cfg, false
	}
	cfg.Namespace = e.StringOr("STRIPE_ACCOUNT_NAMESPACE", "")
	if cfg.Namespace == "" {
		e.Fail("STRIPE_ACCOUNT_NAMESPACE：付款設了 STRIPE_SECRET_KEY 就必填")
		return cfg, false
	}
	if err := stripe.ValidateNamespace(cfg.Namespace); err != nil {
		e.Fail("STRIPE_ACCOUNT_NAMESPACE：%v", err)
		return cfg, false
	}
	return cfg, true
}

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

func newInbox(d *db.DB, jc jobs.Enqueuer, log *slog.Logger) *inbox.Inbox {
	return inbox.New(inbox.Deps{DB: d, Jobs: jc, Now: time.Now, Logger: log})
}

func newPayment(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, box *inbox.Inbox, provider payment.Provider, charge payment.ChargeProvider, orders payment.PaymentOrders, log *slog.Logger) (*payment.Module, *sellerOwners) {
	owners := &sellerOwners{}
	return payment.New(payment.Deps{DB: d, Events: bus, Jobs: jc, Inbox: box, Audit: adminaudit.New(d, time.Now), Provider: provider, Charge: charge, PaymentOrders: orders, Owners: owners, Now: time.Now, Logger: log}), owners
}

func subscribeSellerPayout(bus *events.Bus, module *seller.Module) {
	events.Subscribe(bus, module.OnPayoutAccountChanged)
}

type discardJobs struct{}

func (discardJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

func seedPaymentAccounts(ctx context.Context, c *sellerCommand, fake *payment.Fake, storeIDs []int64, out io.Writer) error {
	jc := discardJobs{}
	m := payment.New(payment.Deps{DB: c.DB, Audit: adminaudit.New(c.DB, time.Now), Events: c.Bus, Jobs: jc, Inbox: newInbox(c.DB, jc, c.Logger), Provider: fake, Owners: c.Owners, Now: time.Now, Logger: c.Logger})
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
