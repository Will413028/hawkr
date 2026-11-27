//go:build integration

package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/announce"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/deliver"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/suppress"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

type fakeContacts struct {
	mu sync.Mutex
	m  map[int64]identity.ContactEmail
}

func (c *fakeContacts) ContactEmail(_ context.Context, id int64) (identity.ContactEmail, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[id]
	return e, ok, nil
}

func (c *fakeContacts) set(id int64, address string, source identity.ContactEmailSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[id] = identity.ContactEmail{Address: address, Source: source}
}

type deliveryState struct {
	Status string
	Reason *string
	Title  *string
	Body   *string
	Locale *string
}

func (f *fixture) deliveryID(t *testing.T, account int64, channel string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.deliveries WHERE account_id = $1 AND channel = $2`, account, channel).Scan(&id); err != nil {
		t.Fatalf("帳號 %d 的 %s 投遞紀錄：%v", account, channel, err)
	}
	return id
}

func (f *fixture) state(t *testing.T, id uuid.UUID) deliveryState {
	t.Helper()
	var s deliveryState
	err := f.db.Pool().QueryRow(context.Background(),
		`SELECT status, failure_reason, sent_title, sent_body, locale FROM notification.deliveries WHERE id = $1`, id).Scan(&s.Status, &s.Reason, &s.Title, &s.Body, &s.Locale)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fixture) deliver(id uuid.UUID, final bool) error {
	return f.module.deliver.Deliver(context.Background(), id, final)
}

func (f *fixture) addToken(t *testing.T, account int64, app, token string) {
	t.Helper()
	f.mustExec(t, `INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at) VALUES ($1, $2, $3, $4)`, token, account, app, time.Now())
}

func (f *fixture) tokens(t *testing.T, account int64) []string {
	t.Helper()
	rows, err := f.db.Pool().Query(context.Background(), `SELECT token FROM notification.device_tokens WHERE account_id = $1 ORDER BY token`, account)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			t.Fatal(err)
		}
		out = append(out, tok)
	}
	return out
}

func (f *fixture) pushDelivery(t *testing.T, account int64, event string) uuid.UUID {
	t.Helper()
	f.accept(t, "test_push", pushParams{AccountID: account, EventID: event})
	f.run(t)
	return f.deliveryID(t, account, "PUSH")
}

func (f *fixture) emailDelivery(t *testing.T, account int64) uuid.UUID {
	t.Helper()
	f.publish(t, suspended(account, uuid.NewV7()))
	f.run(t)
	return f.deliveryID(t, account, "EMAIL")
}

const buyerAddress = "buyer@example.com"

// 暫時性錯誤：第一次送出失敗，投遞留在 PENDING 並回錯誤讓 River 重試；重試成功後只送出一次，之後再執行同一個 job 也不重送。
func TestPushTransientErrorIsRetriedAndDeliveredOnce(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-1")
	id := f.pushDelivery(t, acct, "e1")
	f.push.Script("tok-1", fcm.OutcomeTransient)

	if err := f.deliver(id, false); err == nil {
		t.Fatal("暫時性錯誤要回錯誤讓 River 重試")
	}
	if s := f.state(t, id); s.Status != "PENDING" || s.Title != nil {
		t.Fatalf("暫時性錯誤後狀態 = %+v，預期 PENDING 且沒有送出文字", s)
	}
	if err := f.deliver(id, false); err != nil {
		t.Fatalf("重試：%v", err)
	}
	s := f.state(t, id)
	if s.Status != "SENT" || s.Title == nil || *s.Title != "t" || s.Locale == nil || *s.Locale != "zh-TW" {
		t.Fatalf("重試後狀態 = %+v，預期 SENT 並記下送出的文字與語系", s)
	}
	if err := f.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	if n := f.push.Delivered(); n != 1 {
		t.Fatalf("送出 %d 次，預期只有 1 次", n)
	}
	var item uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.inbox_items WHERE account_id = $1`, acct).Scan(&item); err != nil {
		t.Fatal(err)
	}
	calls := f.push.Calls()
	if len(calls) == 0 || calls[len(calls)-1].Message.Data[deliver.DataInboxItemID] != item.String() {
		t.Fatalf("推播 data = %+v，預期帶 inbox_item_id %s", calls, item)
	}
}

// 最後一次嘗試仍是暫時性錯誤：標永久失敗，不讓紀錄永遠停在 PENDING。
func TestFinalAttemptOfATransientFailureMarksTheDeliveryFailed(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-1")
	id := f.pushDelivery(t, acct, "e1")
	f.push.Script("tok-1", fcm.OutcomeTransient)
	if err := f.deliver(id, true); err != nil {
		t.Fatalf("最後一次嘗試：%v", err)
	}
	if s := f.state(t, id); s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != deliver.ReasonRetriesExhausted || s.Title != nil {
		t.Fatalf("狀態 = %+v", s)
	}
}

// 永久性錯誤（例如 INVALID_ARGUMENT）：不重試（不回錯誤）、標永久失敗，而且不能停用 token。
func TestPushPermanentErrorIsNotRetriedAndKeepsTheToken(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-1")
	id := f.pushDelivery(t, acct, "e1")
	f.push.Script("tok-1", fcm.OutcomePermanent)

	if err := f.deliver(id, false); err != nil {
		t.Fatalf("永久性錯誤不該回錯誤（River 會重試）：%v", err)
	}
	s := f.state(t, id)
	if s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != deliver.ReasonPushRejected || s.Title != nil {
		t.Fatalf("狀態 = %+v", s)
	}
	if got := f.tokens(t, acct); !slices.Equal(got, []string{"tok-1"}) {
		t.Fatalf("永久性錯誤不能停用 token，剩下 %v", got)
	}
	if calls := len(f.push.Calls()); calls != 1 {
		t.Fatalf("呼叫 %d 次，預期 1 次", calls)
	}
}

// email：暫時性錯誤重試後只寄一次，永久性錯誤不重試且不留送出文字；標題是帳號語系的文案。
func TestEmailTransientErrorIsRetriedOnceAndPermanentErrorIsNot(t *testing.T) {
	f := setupIntake(t)
	transient, permanent := f.newAccount(t), f.newAccount(t)
	f.contacts.set(transient, "transient@example.com", identity.ContactEmailVerified)
	f.contacts.set(permanent, "permanent@example.com", identity.ContactEmailVerified)
	tid, pid := f.emailDelivery(t, transient), f.emailDelivery(t, permanent)
	f.mail.Script("transient@example.com", resend.ErrTransient)
	f.mail.Script("permanent@example.com", resend.ErrPermanent)

	if err := f.deliver(tid, false); err == nil {
		t.Fatal("暫時性錯誤要回錯誤讓 River 重試")
	}
	if err := f.deliver(tid, false); err != nil {
		t.Fatalf("重試：%v", err)
	}
	if err := f.deliver(tid, false); err != nil {
		t.Fatal(err)
	}
	if s := f.state(t, tid); s.Status != "SENT" || s.Title == nil || *s.Title != "帳號已被停權" {
		t.Fatalf("暫時性錯誤重試後 = %+v", s)
	}
	if err := f.deliver(pid, false); err != nil {
		t.Fatalf("永久性錯誤不該回錯誤：%v", err)
	}
	if s := f.state(t, pid); s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != deliver.ReasonEmailRejected || s.Title != nil || s.Body != nil || s.Locale != nil {
		t.Fatalf("永久性錯誤後 = %+v", s)
	}
	sent := f.mail.Sent()
	if len(sent) != 1 || sent[0].Email.To != "transient@example.com" || sent[0].Email.IdempotencyKey != tid.String() {
		t.Fatalf("寄出 %+v，預期只有 transient 帳號一封，且以投遞紀錄 ID 當冪等鍵", sent)
	}
}

// UNREGISTERED 與 SENDER_ID_MISMATCH 的 token 被停用（刪除），同一批收下的其他 token 保留，投遞算已送出。
func TestPushUnregisteredTokenIsDeactivatedAndOthersKept(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-gone")
	f.addToken(t, acct, "BUYER", "tok-ok")
	id := f.pushDelivery(t, acct, "e1")
	f.push.Script("tok-gone", fcm.OutcomeUnregistered)

	if err := f.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	if got := f.tokens(t, acct); !slices.Equal(got, []string{"tok-ok"}) {
		t.Fatalf("剩下的 token = %v，預期只有 tok-ok", got)
	}
	if s := f.state(t, id); s.Status != "SENT" {
		t.Fatalf("狀態 = %+v", s)
	}
}

// 所有 token 都失效：全部停用，投遞標永久失敗（沒有可送的裝置）。
func TestPushWithOnlyUnregisteredTokensFailsTheDelivery(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-gone")
	id := f.pushDelivery(t, acct, "e1")
	f.push.Script("tok-gone", fcm.OutcomeUnregistered)
	if err := f.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	if s := f.state(t, id); s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != deliver.ReasonTokenGone {
		t.Fatalf("狀態 = %+v", s)
	}
	if got := f.tokens(t, acct); len(got) != 0 {
		t.Fatalf("失效的 token 還在：%v", got)
	}
}

// THIRD_PARTY_AUTH_ERROR 是設定問題：log 一筆 error 告警，token 不停用，投遞重試。
func TestPushAuthErrorAlertsAndKeepsTheToken(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-1")
	id := f.pushDelivery(t, acct, "e1")
	f.push.Script("tok-1", fcm.OutcomeAuth)
	if err := f.deliver(id, false); err == nil {
		t.Fatal("憑證錯誤要重試（修好設定後可能成功）")
	}
	if got := f.tokens(t, acct); !slices.Equal(got, []string{"tok-1"}) {
		t.Fatalf("token = %v", got)
	}
	logs := f.logs.String()
	if !strings.Contains(logs, "level=ERROR") || !strings.Contains(logs, "THIRD_PARTY_AUTH_ERROR") {
		t.Fatalf("沒有 error 等級的告警 log：%s", logs)
	}
}

// BUYER 受眾只推 BUYER App、STORE 受眾只推 SELLER App：同一個帳號兩支 App 都有 token 時，不跨 App。
func TestBuyerAudienceNeverPushesToTheSellerApp(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "buyer-app-token")
	f.addToken(t, acct, "SELLER", "seller-app-token")

	buyerPush := f.pushDelivery(t, acct, "buyer-event")
	f.accept(t, "test_store_note", storeNoteParams{AccountID: acct, StoreID: 11, EventID: "store-event"})
	f.run(t)
	var storePush uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.deliveries WHERE account_id = $1 AND audience = 'STORE' AND channel = 'PUSH'`, acct).Scan(&storePush); err != nil {
		t.Fatal(err)
	}

	for _, id := range []uuid.UUID{buyerPush, storePush} {
		if err := f.deliver(id, false); err != nil {
			t.Fatal(err)
		}
	}
	var got [][]string
	for _, c := range f.push.Calls() {
		got = append(got, c.Tokens)
	}
	want := [][]string{{"buyer-app-token"}, {"seller-app-token"}}
	if len(got) != 2 || !slices.Equal(got[0], want[0]) || !slices.Equal(got[1], want[1]) {
		t.Fatalf("推播目標 = %v，預期 BUYER 受眾只到 buyer App、STORE 受眾只到 seller App：%v", got, want)
	}
}

// 停權與刪帳號：帳號的 token 在發布事件的同一個 transaction 內消失（transaction rollback 時 token 還在，commit 後才真的沒有），別的帳號不受影響。
func TestSuspensionAndAccountDeletionRemoveTokensInTheSameTransaction(t *testing.T) {
	errRollback := errors.New("rollback")
	cases := []struct {
		name string
		do   func(ctx context.Context, f *fixture, account int64) error
	}{
		{"Suspended", func(ctx context.Context, f *fixture, account int64) error {
			return events.Publish(ctx, f.bus, suspended(account, uuid.NewV7()))
		}},
		{"AccountDeleted", func(ctx context.Context, f *fixture, account int64) error {
			return f.identity.DeleteAccount(ctx, account)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupIntake(t)
			acct, other := f.newAccount(t), f.newAccount(t)
			f.addToken(t, acct, "BUYER", "a-buyer")
			f.addToken(t, acct, "SELLER", "a-seller")
			f.addToken(t, other, "BUYER", "o-buyer")

			err := f.db.InTx(context.Background(), func(ctx context.Context) error {
				if err := c.do(ctx, f, acct); err != nil {
					return err
				}
				var n int
				if err := f.db.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM notification.device_tokens WHERE account_id = $1`, acct).Scan(&n); err != nil {
					return err
				}
				if n != 0 {
					return fmt.Errorf("事件的 transaction 內還有 %d 個 token", n)
				}
				return errRollback
			})
			if !errors.Is(err, errRollback) {
				t.Fatal(err)
			}
			if got := f.tokens(t, acct); len(got) != 2 {
				t.Fatalf("transaction rollback 後 token = %v，預期兩個都還在", got)
			}
			if err := f.db.InTx(context.Background(), func(ctx context.Context) error { return c.do(ctx, f, acct) }); err != nil {
				t.Fatal(err)
			}
			if got := f.tokens(t, acct); len(got) != 0 {
				t.Fatalf("commit 後 token = %v，預期都沒有", got)
			}
			if got := f.tokens(t, other); !slices.Equal(got, []string{"o-buyer"}) {
				t.Fatalf("別的帳號的 token = %v", got)
			}
		})
	}
}

// 60 天沒有重新註冊的 token 被每日 job 清除，近期註冊過的留著。
func TestStaleDeviceTokensArePurgedAfterSixtyDays(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	now := time.Now()
	for token, age := range map[string]time.Duration{"stale": 61 * 24 * time.Hour, "fresh": 59 * 24 * time.Hour} {
		f.mustExec(t, `INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at) VALUES ($1, $2, 'BUYER', $3)`, token, acct, now.Add(-age))
	}
	n, err := f.module.purge.SweepStaleTokens(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("清除 %d 個、錯誤 %v，預期 1 個", n, err)
	}
	if got := f.tokens(t, acct); !slices.Equal(got, []string{"fresh"}) {
		t.Fatalf("剩下 %v，預期只有 fresh", got)
	}
}

func testAddressHash(key []byte, normalized string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(normalized))
	return m.Sum(nil)
}

// 位址的雜湊在 suppression 名單內不寄（比對的是正規化後位址的雜湊；目前與輪替期間的舊密鑰都查），投遞標永久失敗。
func TestSuppressedAddressIsNotEmailed(t *testing.T) {
	for name, key := range map[string][]byte{"目前的密鑰": testSuppressionKeys.Current, "輪替期間的舊密鑰": testSuppressionKeys.Previous} {
		t.Run(name, func(t *testing.T) {
			f := setupIntake(t)
			acct := f.newAccount(t)
			f.contacts.set(acct, "Buyer@Example.COM", identity.ContactEmailVerified)
			f.mustExec(t, `INSERT INTO notification.email_suppressions (address_hash, reason, created_at) VALUES ($1, 'HARD_BOUNCE', $2)`, testAddressHash(key, buyerAddress), epoch)
			id := f.emailDelivery(t, acct)

			if err := f.deliver(id, false); err != nil {
				t.Fatal(err)
			}
			if s := f.state(t, id); s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != deliver.ReasonSuppressed {
				t.Fatalf("狀態 = %+v", s)
			}
			if sent := f.mail.Sent(); len(sent) != 0 {
				t.Fatalf("suppression 名單內的位址收到了信：%+v", sent)
			}
		})
	}
}

// 沒有設定雜湊密鑰時判斷不了名單：不寄、job 回錯誤重試（fail closed）。
func TestEmailIsNotSentWithoutSuppressionKeys(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.contacts.set(acct, buyerAddress, identity.ContactEmailVerified)
	id := f.emailDelivery(t, acct)
	svc := deliver.New(deliver.Deps{
		DB: f.db, Locks: intake.Locks{Accounts: f.identity, Buyers: f.buyer, Stores: f.stores}, Contacts: f.contacts, Locales: f.identity,
		Registry: testRegistry{}, Sources: f.module.app, Push: f.push, Mail: f.mail, Suppression: suppress.NewHasher(SuppressionKeys{}), Now: time.Now,
	})
	if err := svc.Deliver(context.Background(), id, false); err == nil {
		t.Fatal("沒有密鑰要回錯誤讓 job 重試")
	}
	if s := f.state(t, id); s.Status != "PENDING" || len(f.mail.Sent()) != 0 {
		t.Fatalf("狀態 = %+v，寄出 %d 封", s, len(f.mail.Sent()))
	}
}

// 只寄已驗證的信箱：自填的信箱與沒有信箱都不寄。
func TestEmailOnlyGoesToAVerifiedAddress(t *testing.T) {
	f := setupIntake(t)
	selfEntered, none := f.newAccount(t), f.newAccount(t)
	f.contacts.set(selfEntered, "self@example.com", identity.ContactEmailSelfEntered)
	for _, acct := range []int64{selfEntered, none} {
		id := f.emailDelivery(t, acct)
		if err := f.deliver(id, false); err != nil {
			t.Fatal(err)
		}
		if s := f.state(t, id); s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != deliver.ReasonNoEmailAddress {
			t.Fatalf("帳號 %d：%+v", acct, s)
		}
	}
	if sent := f.mail.Sent(); len(sent) != 0 {
		t.Fatalf("寄出 %+v", sent)
	}
}

// 送出前收件人已刪除：投遞標永久失敗、不寫送出文字，provider 沒有被呼叫（清除之後才執行的待送 job 不得寫回文字，R1-4）。
func TestDeliveryToADeletedRecipientFailsWithoutSentText(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-1")
	f.contacts.set(acct, buyerAddress, identity.ContactEmailVerified)
	pushID := f.pushDelivery(t, acct, "e1")
	emailID := f.deliveryID(t, acct, "EMAIL")
	if err := f.identity.DeleteAccount(context.Background(), acct); err != nil {
		t.Fatal(err)
	}

	for _, id := range []uuid.UUID{pushID, emailID} {
		if err := f.deliver(id, false); err != nil {
			t.Fatal(err)
		}
		s := f.state(t, id)
		if s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != deliver.ReasonRecipientGone || s.Title != nil || s.Body != nil || s.Locale != nil {
			t.Fatalf("已刪除的收件人：%+v", s)
		}
	}
	if len(f.push.Calls()) != 0 || len(f.mail.Sent()) != 0 {
		t.Fatalf("已刪除的收件人仍呼叫了 provider：push=%d mail=%d", len(f.push.Calls()), len(f.mail.Sent()))
	}
}

type beforeSend struct {
	push *fcm.Fake
	mail *resend.Fake
	hook func()
}

func (b beforeSend) Send(ctx context.Context, tokens []string, m fcm.Message) ([]fcm.Result, error) {
	b.hook()
	return b.push.Send(ctx, tokens, m)
}

type beforeMail beforeSend

func (b beforeMail) Send(ctx context.Context, e resend.Email) error {
	b.hook()
	return b.mail.Send(ctx, e)
}

// 有 token 收下之後，失效 token 的刪除失敗不得讓 job 回錯誤（重試會對已收到的裝置重推）：先定案 SENT，刪除失敗只記 log，token 留到下次推送再清（R1-2）。
func TestTokenDeleteFailureAfterASuccessfulPushStillSettlesSent(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-ok")
	f.addToken(t, acct, "BUYER", "tok-gone")
	id := f.pushDelivery(t, acct, "e1")
	f.push.Script("tok-gone", fcm.OutcomeUnregistered)
	f.mustExec(t, `CREATE FUNCTION notification.refuse_token_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'delete refused'; END $$`)
	f.mustExec(t, `CREATE TRIGGER refuse_token_delete BEFORE DELETE ON notification.device_tokens FOR EACH ROW EXECUTE FUNCTION notification.refuse_token_delete()`)

	if err := f.deliver(id, false); err != nil {
		t.Fatalf("刪除 token 失敗不該讓 job 回錯誤：%v", err)
	}
	if s := f.state(t, id); s.Status != "SENT" {
		t.Fatalf("狀態 = %+v，預期 SENT", s)
	}
	if got := f.push.Delivered(); got != 1 {
		t.Fatalf("送出 %d 次，預期 1", got)
	}
	if err := f.deliver(id, false); err != nil || len(f.push.Calls()) != 1 {
		t.Fatalf("再執行不得重推：err=%v calls=%d", err, len(f.push.Calls()))
	}
	if got := f.tokens(t, acct); !slices.Equal(got, []string{"tok-gone", "tok-ok"}) {
		t.Fatalf("token = %v，刪除被拒時兩個都該留著", got)
	}
}

func (f *fixture) deletingService(acct int64) *deliver.Service {
	hook := beforeSend{push: f.push, mail: f.mail, hook: func() {
		if err := f.identity.DeleteAccount(context.Background(), acct); err != nil {
			panic(err)
		}
	}}
	return deliver.New(deliver.Deps{
		DB: f.db, Locks: intake.Locks{Accounts: f.identity, Buyers: f.buyer, Stores: f.stores}, Contacts: f.contacts, Locales: f.identity,
		Registry: testRegistry{}, Sources: f.module.app, Push: hook, Mail: beforeMail(hook), Suppression: suppress.NewHasher(testSuppressionKeys), Now: time.Now,
	})
}

// 送出之後收件人才被刪除：provider 已收下，投遞照記 SENT，但不留送出文字與語系（D16；settle 的「sent 且收件人已不在」分支）。
func TestRecipientDeletedAfterSendIsSentWithoutSentText(t *testing.T) {
	ctx := context.Background()
	t.Run("push", func(t *testing.T) {
		f := setupIntake(t)
		acct := f.newAccount(t)
		f.addToken(t, acct, "BUYER", "tok-1")
		id := f.pushDelivery(t, acct, "e1")
		if err := f.deletingService(acct).Deliver(ctx, id, false); err != nil {
			t.Fatal(err)
		}
		if s := f.state(t, id); s.Status != "SENT" || s.Title != nil || s.Body != nil || s.Locale != nil {
			t.Fatalf("%+v，預期 SENT 且沒有送出文字", s)
		}
		if len(f.push.Calls()) != 1 {
			t.Fatalf("推播應該已送出一次：%d", len(f.push.Calls()))
		}
	})
	t.Run("email", func(t *testing.T) {
		f := setupIntake(t)
		acct := f.newAccount(t)
		f.contacts.set(acct, buyerAddress, identity.ContactEmailVerified)
		id := f.emailDelivery(t, acct)
		if err := f.deletingService(acct).Deliver(ctx, id, false); err != nil {
			t.Fatal(err)
		}
		if s := f.state(t, id); s.Status != "SENT" || s.Title != nil || s.Body != nil || s.Locale != nil {
			t.Fatalf("%+v，預期 SENT 且沒有送出文字", s)
		}
		if len(f.mail.Sent()) != 1 {
			t.Fatalf("email 應該已寄出一次：%d", len(f.mail.Sent()))
		}
	})
}

// 推播與 email 依帳號 preferred_locale 在送出時組字（讀真的 identity）：同一個意圖，英文帳號收到英文標題，投遞紀錄記下實際使用的語系。
func TestEmailIsComposedInTheRecipientsPreferredLocale(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.contacts.set(acct, buyerAddress, identity.ContactEmailVerified)
	if changed, err := f.identity.SetPreferredLocale(context.Background(), acct, uitext.EN); err != nil || !changed {
		t.Fatalf("設定帳號語系：changed=%v err=%v", changed, err)
	}
	id := f.emailDelivery(t, acct)
	var params string
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT params::text FROM notification.inbox_items WHERE account_id = $1`, acct).Scan(&params); err != nil {
		t.Fatal(err)
	}
	entry, _ := registry.Lookup("account_suspended")
	en, err := entry.Render(uitext.EN, []byte(params))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	s := f.state(t, id)
	if s.Status != "SENT" || s.Locale == nil || *s.Locale != "en" || s.Title == nil || *s.Title != en.Title || *s.Body != en.Body {
		t.Fatalf("狀態 = %+v，預期英文標題 %q", s, en.Title)
	}
	sent := f.mail.Sent()
	if len(sent) != 1 || sent[0].Email.Subject != en.Title || sent[0].Email.Text != en.Body {
		t.Fatalf("寄出 %+v", sent)
	}
}

// 帶圖片的型別：渠道 job 以注入的 ImageURL 組出絕對網址，送進 FCM 的請求本文有 apns.fcm_options.image；沒有圖的型別不帶。
func TestPushOfAnImageTypeCarriesTheImageInTheFCMPayload(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	f.addToken(t, acct, "BUYER", "tok-1")
	f.accept(t, "test_push_image", pushParams{AccountID: acct, EventID: "img"})
	f.run(t)
	id := f.deliveryID(t, acct, "PUSH")
	if err := f.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	calls := f.push.Calls()
	if len(calls) != 1 {
		t.Fatalf("推播 = %+v", calls)
	}
	want := "https://media.example/i/card/" + testImageKey
	if calls[0].Message.ImageURL != want {
		t.Fatalf("ImageURL = %q，預期 %q", calls[0].Message.ImageURL, want)
	}
	body := fcm.Payload("tok-1", calls[0].Message)["message"].(map[string]any)
	apns := body["apns"].(map[string]any)
	if got := apns["fcm_options"].(map[string]string)["image"]; got != want {
		t.Fatalf("apns.fcm_options.image = %q，預期 %q", got, want)
	}
	other := f.newAccount(t)
	f.addToken(t, other, "BUYER", "tok-2")
	plain := f.pushDelivery(t, other, "plain")
	if err := f.deliver(plain, false); err != nil {
		t.Fatal(err)
	}
	if got := f.push.Calls(); len(got) != 2 || got[1].Message.ImageURL != "" {
		t.Fatalf("沒有圖的型別不該帶圖：%+v", got)
	}
}

// App 啟動時呼叫 RegisterDeviceToken 帶著協商出的語系：不同才更新帳號的 preferred_locale，之後的渠道 job 以新語系組字（整條路徑都是真的 identity）。
func TestRegisterDeviceTokenRecordsTheRequestLocaleForLaterDeliveries(t *testing.T) {
	f := setupIntake(t)
	ctx := context.Background()
	acct := f.newAccount(t)
	f.contacts.set(acct, buyerAddress, identity.ContactEmailVerified)
	id := f.emailDelivery(t, acct)
	if err := f.module.RegisterDeviceToken(uitext.WithRequestedLocale(ctx, uitext.EN), acct, DeviceAppBuyer, "tok-1"); err != nil {
		t.Fatal(err)
	}
	if got, err := f.identity.PreferredLocale(ctx, acct); err != nil || got != uitext.EN {
		t.Fatalf("語系 = %q %v，預期 en", got, err)
	}
	if err := f.deliver(id, false); err != nil {
		t.Fatal(err)
	}
	if s := f.state(t, id); s.Status != "SENT" || s.Locale == nil || *s.Locale != "en" {
		t.Fatalf("狀態 = %+v，預期以英文組字", s)
	}
	if err := f.module.RegisterDeviceToken(uitext.WithRequestedLocale(ctx, uitext.ZhTW), acct, DeviceAppBuyer, "tok-1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.identity.PreferredLocale(ctx, acct); got != uitext.ZhTW {
		t.Fatalf("回前景改成 zh-TW 後語系 = %q", got)
	}
	var tokens int
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM notification.device_tokens WHERE account_id = $1`, acct).Scan(&tokens); err != nil || tokens != 1 {
		t.Fatalf("token 數 = %d %v", tokens, err)
	}
}

// 請求沒有帶語系（沒有 Accept-Language，ctx 只有預設值）時不得用預設的 zh-TW 蓋掉已存的 en。
func TestRegisterDeviceTokenWithoutALocaleKeepsTheStoredPreference(t *testing.T) {
	f := setupIntake(t)
	ctx := context.Background()
	acct := f.newAccount(t)
	if changed, err := f.identity.SetPreferredLocale(ctx, acct, uitext.EN); err != nil || !changed {
		t.Fatalf("先存 en：changed=%v err=%v", changed, err)
	}
	if err := f.module.RegisterDeviceToken(ctx, acct, DeviceAppBuyer, "tok-2"); err != nil {
		t.Fatal(err)
	}
	if err := f.module.RegisterDeviceToken(uitext.WithLocale(ctx, uitext.ZhTW), acct, DeviceAppBuyer, "tok-2"); err != nil {
		t.Fatal(err)
	}
	if got, err := f.identity.PreferredLocale(ctx, acct); err != nil || got != uitext.EN {
		t.Fatalf("語系 = %q %v，沒帶語系的請求不得改成預設的 zh-TW", got, err)
	}
}

// 公告的推播文字讀取時才解析（參數只有公告 ID）：渠道 job 讀得到公告內容，推給收件人的 BUYER App。
func TestAnnouncementPushIsDeliveredWithTheAnnouncementText(t *testing.T) {
	f := setupAnnounce(t, 0)
	acct := f.newAccount(t)
	mustExec(t, f.db, `INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at) VALUES ('tok', $1, 'BUYER', $2)`, acct, epoch)
	a := f.create(t, announce.CreateInput{Audiences: []string{"BUYER"}})
	f.drain(t)
	var id uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.deliveries WHERE intent_id = $1 AND account_id = $2`, a.ID, acct).Scan(&id); err != nil {
		t.Fatal(err)
	}
	push := fcm.NewFake(nil)
	svc := deliver.New(deliver.Deps{
		DB: f.db, Locks: intake.Locks{Accounts: f.identity, Buyers: f.buyers, Stores: f.stores}, Contacts: &fakeContacts{m: map[int64]identity.ContactEmail{}},
		Locales: f.identity, Registry: registry.Registry{}, Sources: f.inbox, Push: push, Mail: resend.NewFake(nil), Now: time.Now,
	})
	if err := svc.Deliver(context.Background(), id, false); err != nil {
		t.Fatal(err)
	}
	calls := push.Calls()
	if len(calls) != 1 || calls[0].Message.Title != a.Title || calls[0].Message.Body != a.Body || !slices.Equal(calls[0].Tokens, []string{"tok"}) {
		t.Fatalf("推播 = %+v，預期公告的標題與內文", calls)
	}
	if got := f.scan(t, `SELECT count(*) FROM notification.deliveries WHERE id = $1 AND status = 'SENT' AND locale = 'en'`, id); got != 1 {
		t.Fatal("公告的投遞紀錄要記為已送出、語系 en")
	}
}
