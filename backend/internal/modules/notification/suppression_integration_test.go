//go:build integration

package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// dispatchJobs 記下 inbox 排的派送 job，由測試決定何時派送（等同 River 取件）。
type dispatchJobs struct {
	mu   sync.Mutex
	args []inbox.DispatchArgs
}

func (d *dispatchJobs) Enqueue(_ context.Context, a river.JobArgs, _ *river.InsertOpts) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	da, ok := a.(inbox.DispatchArgs)
	if !ok {
		panic("unknown job args")
	}
	d.args = append(d.args, da)
	return nil
}

func (d *dispatchJobs) take() []inbox.DispatchArgs {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.args
	d.args = nil
	return out
}

const resendSecret = "whsec_MDEyMzQ1Njc4OWFiY2RlZg==" //nolint:gosec // 測試用的假簽章密鑰

type webhookFixture struct {
	db     *db.DB
	module *Module
	box    *inbox.Inbox
	jobs   *dispatchJobs
}

func setupWebhook(t *testing.T, secret string) *webhookFixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: inbox.Schema, Dir: inbox.Migrations()},
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "notification", Dir: Migrations()},
	))
	log := slog.New(slog.DiscardHandler)
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: nopJobs{}, Media: media.NewFake(), Now: time.Now, Logger: log})
	jobs := &dispatchJobs{}
	box := inbox.New(inbox.Deps{DB: d, Jobs: jobs, Now: time.Now, Logger: log})
	m := newModule(Deps{
		DB: d, Identity: id, Buyer: buyers, Seller: &fakeStores{retired: map[int64]bool{}}, Jobs: nopJobs{}, Now: time.Now, NewID: uuid.NewV7, Logger: log,
		Contacts: &fakeContacts{m: map[int64]identity.ContactEmail{}}, Locales: &fakeLocales{m: map[int64]uitext.Locale{}},
		Push: fcm.NewFake(nil), Mail: resend.NewFake(nil), Inbox: box, ResendWebhookSecret: secret,
	}, testRegistry{})
	return &webhookFixture{db: d, module: m, box: box, jobs: jobs}
}

func signedResend(secret, id string, body string) http.Header {
	key, _ := base64.StdEncoding.DecodeString(secret[len("whsec_"):])
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(id + "." + ts + "." + body))
	h := http.Header{}
	h.Set("svix-id", id)
	h.Set("svix-timestamp", ts)
	h.Set("svix-signature", "v1,"+base64.StdEncoding.EncodeToString(m.Sum(nil)))
	return h
}

func (f *webhookFixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

const (
	permanentBounce = `{"type":"email.bounced","data":{"to":["Gone@Example.com"],"bounce":{"type":"Permanent"}}}`
	transientBounce = `{"type":"email.bounced","data":{"to":["slow@example.com"],"bounce":{"type":"Transient"}}}`
	complaint       = `{"type":"email.complained","data":{"to":["angry@example.com"]}}`
)

// 永久退信與投訴：先寫 platform/inbox（這時 suppression 名單還沒有）並排派送 job，派送之後位址才進名單（正規化成小寫）；
// 暫時退信不寫入；同一則事件重送不重複寫入、不重複排 job。
func TestBounceAndComplaintWebhooksGoThroughTheInboxBeforeSuppression(t *testing.T) {
	t.Parallel()
	f := setupWebhook(t, resendSecret)
	ctx := context.Background()
	for i, body := range []string{permanentBounce, complaint, transientBounce} {
		if err := f.module.ReceiveResendEvent(ctx, signedResend(resendSecret, "msg_"+strconv.Itoa(i), body), []byte(body)); err != nil {
			t.Fatalf("事件 %d：%v", i, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM inbox.events WHERE provider = 'resend'`); n != 2 {
		t.Fatalf("inbox 有 %d 筆 Resend 事件，預期 2（暫時退信不寫入）", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.email_suppressions`); n != 0 {
		t.Fatalf("派送之前名單已有 %d 筆；事件要先寫 inbox", n)
	}
	body := permanentBounce
	if err := f.module.ReceiveResendEvent(ctx, signedResend(resendSecret, "msg_0", body), []byte(body)); err != nil {
		t.Fatal(err)
	}
	jobs := f.jobs.take()
	if len(jobs) != 2 {
		t.Fatalf("派送 job %d 個，預期 2（重送的事件不重複排）", len(jobs))
	}
	for _, j := range jobs {
		if err := f.box.Dispatch(ctx, j.Provider, j.EventID); err != nil {
			t.Fatal(err)
		}
	}
	for addr, reason := range map[string]string{"gone@example.com": "BOUNCE", "angry@example.com": "COMPLAINT"} {
		if n := f.count(t, `SELECT count(*) FROM notification.email_suppressions WHERE address = $1 AND reason = $2`, addr, reason); n != 1 {
			t.Errorf("%s 沒有以 %s 進名單", addr, reason)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM notification.email_suppressions`); n != 2 {
		t.Fatalf("名單有 %d 筆，預期 2", n)
	}
	if n := f.count(t, `SELECT count(*) FROM inbox.events WHERE provider = 'resend' AND processed_at IS NOT NULL`); n != 2 {
		t.Fatalf("已處理 %d 筆，預期 2", n)
	}
}

// 簽章不合回 invalid_argument、沒有寫入；沒有設定密鑰回 failed_precondition。
func TestResendWebhookRejectsBadSignaturesAndMissingSecret(t *testing.T) {
	t.Parallel()
	f := setupWebhook(t, resendSecret)
	ctx := context.Background()
	bad := signedResend(resendSecret, "msg_1", permanentBounce)
	if err := f.module.ReceiveResendEvent(ctx, bad, []byte(permanentBounce+" ")); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("竄改的本文：%v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM inbox.events`); n != 0 {
		t.Fatalf("簽章不合卻寫入了 %d 筆", n)
	}
	unset := setupWebhook(t, "")
	if err := unset.module.ReceiveResendEvent(ctx, signedResend(resendSecret, "msg_1", permanentBounce), []byte(permanentBounce)); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有設定密鑰：%v", err)
	}
}
