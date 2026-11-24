//go:build integration

package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"slices"
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
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Media: media.NewFake(), Now: time.Now, Logger: log})
	jobs := &dispatchJobs{}
	box := inbox.New(inbox.Deps{DB: d, Jobs: jobs, Now: time.Now, Logger: log})
	m := newModule(Deps{
		DB: d, Identity: id, Buyer: buyers, Seller: &fakeStores{retired: map[int64]bool{}}, Validity: noVouchers{}, Jobs: &jobQueue{}, Now: time.Now, NewID: uuid.NewV7, Logger: log,
		Contacts: &fakeContacts{m: map[int64]identity.ContactEmail{}}, Locales: id,
		Push: fcm.NewFake(nil), Mail: resend.NewFake(nil), Inbox: box, ResendWebhookSecret: secret, SuppressionKeys: testSuppressionKeys,
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

const undeterminedBounce = `{"type":"email.bounced","data":{"to":["maybe@example.com"],"bounce":{"type":"Undetermined"}}}`

// dispatchAll 派送目前排的所有 inbox job（等同 River 取件）。
func (f *webhookFixture) dispatchAll(t *testing.T) {
	t.Helper()
	for _, j := range f.jobs.take() {
		if err := f.box.Dispatch(context.Background(), j.Provider, j.EventID); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *webhookFixture) receive(t *testing.T, id, body string) {
	t.Helper()
	if err := f.module.ReceiveResendEvent(context.Background(), signedResend(resendSecret, id, body), []byte(body)); err != nil {
		t.Fatalf("事件 %s：%v", id, err)
	}
}

// 永久退信與投訴：先寫 platform/inbox（這時 suppression 名單還沒有）並排派送 job，派送之後位址的雜湊才進名單（原因分 HARD_BOUNCE 與 COMPLAINT）；
// 暫時退信與未定的退信不寫入；同一則事件重送不重複寫入、不重複排 job。
func TestBounceAndComplaintWebhooksGoThroughTheInboxBeforeSuppression(t *testing.T) {
	t.Parallel()
	f := setupWebhook(t, resendSecret)
	for i, body := range []string{permanentBounce, complaint, transientBounce, undeterminedBounce} {
		f.receive(t, "msg_"+strconv.Itoa(i), body)
	}
	if n := f.count(t, `SELECT count(*) FROM inbox.events WHERE provider = 'resend'`); n != 2 {
		t.Fatalf("inbox 有 %d 筆 Resend 事件，預期 2（暫時與未定的退信不寫入）", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.email_suppressions`); n != 0 {
		t.Fatalf("派送之前名單已有 %d 筆；事件要先寫 inbox", n)
	}
	f.receive(t, "msg_0", permanentBounce)
	if got := len(f.jobs.args); got != 2 {
		t.Fatalf("派送 job %d 個，預期 2（重送的事件不重複排）", got)
	}
	f.dispatchAll(t)
	for addr, reason := range map[string]string{"gone@example.com": "HARD_BOUNCE", "angry@example.com": "COMPLAINT"} {
		if n := f.count(t, `SELECT count(*) FROM notification.email_suppressions WHERE address_hash = $1 AND reason = $2`, testAddressHash(testSuppressionKeys.Current, addr), reason); n != 1 {
			t.Errorf("%s 沒有以 %s 與目前密鑰的雜湊進名單", addr, reason)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM notification.email_suppressions`); n != 2 {
		t.Fatalf("名單有 %d 筆，預期 2（軟退信不得進名單）", n)
	}
	if n := f.count(t, `SELECT count(*) FROM inbox.events WHERE provider = 'resend' AND processed_at IS NOT NULL`); n != 2 {
		t.Fatalf("已處理 %d 筆，預期 2", n)
	}
}

// 名單沒有明文位址的欄位；inbox 的 payload 從寫入起就只有原因與雜湊（處理前後都沒有位址、也沒有原始事件的其他欄位）。
func TestSuppressionKeepsNoPlainAddressAnywhere(t *testing.T) {
	t.Parallel()
	f := setupWebhook(t, resendSecret)
	rows, err := f.db.Pool().Query(context.Background(), `SELECT column_name FROM information_schema.columns WHERE table_schema = 'notification' AND table_name = 'email_suppressions' ORDER BY column_name`)
	if err != nil {
		t.Fatal(err)
	}
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, c)
	}
	rows.Close()
	if !slices.Equal(cols, []string{"address_hash", "created_at", "reason"}) {
		t.Fatalf("email_suppressions 的欄位 = %v", cols)
	}
	f.receive(t, "msg_0", permanentBounce)
	f.receive(t, "msg_1", complaint)
	check := func(stage string) {
		t.Helper()
		if n := f.count(t, `SELECT count(*) FROM inbox.events WHERE provider = 'resend' AND (convert_from(payload, 'UTF8') ILIKE '%example.com%' OR convert_from(payload, 'UTF8') ILIKE '%@%' OR convert_from(payload, 'UTF8') ILIKE '%"type"%')`); n != 0 {
			t.Fatalf("%s：inbox 的 payload 含位址或原始事件欄位", stage)
		}
		if n := f.count(t, `SELECT count(*) FROM inbox.events WHERE provider = 'resend' AND convert_from(payload, 'UTF8')::jsonb->>'reason' IN ('HARD_BOUNCE', 'COMPLAINT') AND jsonb_array_length(convert_from(payload, 'UTF8')::jsonb->'address_hashes') = 1`); n != 2 {
			t.Fatalf("%s：payload 不是預期的縮減形狀（%d 筆合格）", stage, n)
		}
	}
	check("處理前")
	f.dispatchAll(t)
	check("處理後")
}

// 退信可由管理員（內部方法）移除，投訴永遠不可移除；位址不在名單回 false。輪替期間舊密鑰寫的列同樣找得到；投訴會把先前的退信列升級成 COMPLAINT。
func TestHardBounceIsRemovableButComplaintIsNot(t *testing.T) {
	t.Parallel()
	f := setupWebhook(t, resendSecret)
	ctx := context.Background()
	f.receive(t, "msg_0", permanentBounce)
	f.receive(t, "msg_1", complaint)
	f.dispatchAll(t)

	if _, err := f.module.RemoveEmailSuppression(ctx, "Angry@Example.com"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("移除投訴：err = %v，預期 FailedPrecondition", err)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.email_suppressions WHERE reason = 'COMPLAINT'`); n != 1 {
		t.Fatal("投訴的列不得被移除")
	}
	if removed, err := f.module.RemoveEmailSuppression(ctx, "GONE@example.com"); err != nil || !removed {
		t.Fatalf("移除退信：removed=%v err=%v", removed, err)
	}
	if removed, err := f.module.RemoveEmailSuppression(ctx, "gone@example.com"); err != nil || removed {
		t.Fatalf("再移除一次：removed=%v err=%v", removed, err)
	}
	if removed, err := f.module.RemoveEmailSuppression(ctx, "not-an-address"); apperr.KindOf(err) != apperr.KindInvalidInput || removed {
		t.Fatalf("格式不合：removed=%v err=%v，預期 InvalidInput（不同於「名單裡沒有」的 false, nil）", removed, err)
	}

	// 舊密鑰寫的退信列：移除找得到。
	f.mustInsertHash(t, testSuppressionKeys.Previous, "old@example.com", "HARD_BOUNCE")
	if removed, err := f.module.RemoveEmailSuppression(ctx, "old@example.com"); err != nil || !removed {
		t.Fatalf("舊密鑰的列：removed=%v err=%v", removed, err)
	}

	// 先永久退信、之後投訴：升級成 COMPLAINT，之後不可移除；已是 COMPLAINT 的不會被退信降級。
	f.receive(t, "msg_2", `{"type":"email.bounced","data":{"to":["both@example.com"],"bounce":{"type":"Permanent"}}}`)
	f.dispatchAll(t)
	f.receive(t, "msg_3", `{"type":"email.complained","data":{"to":["both@example.com"]}}`)
	f.dispatchAll(t)
	f.receive(t, "msg_4", `{"type":"email.bounced","data":{"to":["both@example.com"],"bounce":{"type":"Permanent"}}}`)
	f.dispatchAll(t)
	h := testAddressHash(testSuppressionKeys.Current, "both@example.com")
	if n := f.count(t, `SELECT count(*) FROM notification.email_suppressions WHERE address_hash = $1 AND reason = 'COMPLAINT'`, h); n != 1 {
		t.Fatal("投訴要把退信列升級成 COMPLAINT，且之後的退信不得降級")
	}
	if _, err := f.module.RemoveEmailSuppression(ctx, "both@example.com"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("升級後移除：err = %v", err)
	}
}

func (f *webhookFixture) mustInsertHash(t *testing.T, key []byte, address, reason string) {
	t.Helper()
	if _, err := f.db.Pool().Exec(context.Background(), `INSERT INTO notification.email_suppressions (address_hash, reason, created_at) VALUES ($1, $2, now())`, testAddressHash(key, address), reason); err != nil {
		t.Fatal(err)
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
