//go:build integration

package notification

import (
	"context"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// recipient 是測試裡的一個收件人（帳號、受眾、店）與它的通知資料：每個收件人有收件匣列、帶送出文字的投遞紀錄與一個意圖。
type recipient struct {
	name     string
	account  int64
	audience string
	storeID  int64
	// intents 是這個收件人的意圖冪等鍵。
	intents []string
}

func (r recipient) inbox(t *testing.T, f *fixture) int {
	q := `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND audience = $2 AND store_id IS NOT DISTINCT FROM $3`
	return f.count(t, q, r.account, r.audience, r.storeIDOrNil())
}

func (r recipient) storeIDOrNil() *int64 {
	if r.storeID == 0 {
		return nil
	}
	return &r.storeID
}

func (r recipient) deliveries(t *testing.T, f *fixture) int {
	return f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1 AND audience = $2 AND store_id IS NOT DISTINCT FROM $3`,
		r.account, r.audience, r.storeIDOrNil())
}

func (r recipient) deliveriesWithText(t *testing.T, f *fixture) int {
	return f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1 AND audience = $2 AND store_id IS NOT DISTINCT FROM $3
		AND sent_title IS NOT NULL AND sent_body IS NOT NULL AND locale IS NOT NULL`, r.account, r.audience, r.storeIDOrNil())
}

func (r recipient) scrubbedIntents(t *testing.T, f *fixture) (scrubbed, total int) {
	for _, key := range r.intents {
		var params string
		if err := f.db.Pool().QueryRow(context.Background(), `SELECT params::text FROM notification.intents WHERE idempotency_key = $1`, key).Scan(&params); err != nil {
			t.Fatalf("%s 的意圖 %s：%v", r.name, key, err)
		}
		total++
		if params == "{}" {
			scrubbed++
		}
	}
	return scrubbed, total
}

func prefCount(t *testing.T, f *fixture, account int64, audience string) int {
	return f.count(t, `SELECT count(*) FROM notification.preference_overrides WHERE account_id = $1 AND audience = $2`, account, audience)
}

// seedRecipients 建 5 個收件人的資料，全部走真正的管線：
// A/BUYER、A/STORE 店 11、A/STORE 店 12（同帳號的另一間店）、B/BUYER、B/STORE 店 21；
// 每個收件人都有收件匣與帶送出文字的投遞紀錄，每個帳號與受眾有一列偏好覆寫。
func seedRecipients(t *testing.T, f *fixture) (a, b int64, rs map[string]recipient) {
	t.Helper()
	a, b = f.newAccount(t), f.newAccount(t)
	rs = map[string]recipient{
		"A/BUYER":   {name: "A/BUYER", account: a, audience: "BUYER", intents: []string{"test_like:a-like", "test_push:a-push"}},
		"A/STORE11": {name: "A/STORE11", account: a, audience: "STORE", storeID: 11, intents: []string{"test_store_note:a-s11-note", "test_store_push_only:a-s11-only"}},
		"A/STORE12": {name: "A/STORE12", account: a, audience: "STORE", storeID: 12, intents: []string{"test_store_note:a-s12-note", "test_store_push_only:a-s12-only"}},
		"B/BUYER":   {name: "B/BUYER", account: b, audience: "BUYER", intents: []string{"test_like:b-like", "test_push:b-push"}},
		"B/STORE21": {name: "B/STORE21", account: b, audience: "STORE", storeID: 21, intents: []string{"test_store_note:b-s21-note", "test_store_push_only:b-s21-only"}},
	}
	actor := f.newAccount(t)
	f.accept(t, "test_like", likeParams{AccountID: a, EventID: "a-like", PostID: "p", ActorID: actor})
	f.accept(t, "test_push", pushParams{AccountID: a, EventID: "a-push"})
	f.accept(t, "test_like", likeParams{AccountID: b, EventID: "b-like", PostID: "p", ActorID: actor})
	f.accept(t, "test_push", pushParams{AccountID: b, EventID: "b-push"})
	for _, r := range []recipient{rs["A/STORE11"], rs["A/STORE12"], rs["B/STORE21"]} {
		suffix := fmt.Sprintf("s%d", r.storeID)
		prefix := "a-"
		if r.account == b {
			prefix = "b-"
		}
		f.accept(t, "test_store_note", storeNoteParams{AccountID: r.account, StoreID: r.storeID, EventID: prefix + suffix + "-note"})
		f.accept(t, "test_store_push_only", storeNoteParams{AccountID: r.account, StoreID: r.storeID, EventID: prefix + suffix + "-only"})
	}
	f.run(t)
	f.mustExec(t, `UPDATE notification.deliveries SET status = 'SENT', sent_at = $1, locale = 'zh-TW', sent_title = '標題', sent_body = '內文'`, epoch)
	for _, acct := range []int64{a, b} {
		for _, aud := range []string{"BUYER", "STORE"} {
			f.mustExec(t, `INSERT INTO notification.preference_overrides (account_id, audience, category, channel, enabled, updated_at) VALUES ($1, $2, 'LIKE', 'PUSH', false, $3)`, acct, aud, epoch)
		}
	}
	for name, r := range rs {
		if r.inbox(t, f) == 0 && r.audience != "STORE" {
			t.Fatalf("%s 沒有收件匣列，測試資料不對", name)
		}
		if r.deliveries(t, f) == 0 || r.deliveriesWithText(t, f) != r.deliveries(t, f) {
			t.Fatalf("%s 的投遞紀錄不對", name)
		}
	}
	return a, b, rs
}

// 三個刪除事件各自只清除自己範圍內的收件人：收件匣列與偏好刪除、投遞紀錄的送出文字清掉（列留著）、意圖參數換成 '{}'；
// 其他帳號、同帳號的另一個受眾、同帳號的另一間店都不動。
func TestPurgeAfterEachDeletionEvent(t *testing.T) {
	type scenario struct {
		name string
		// publish 發布事件並回傳預期排出的 job 範圍。
		publish func(t *testing.T, f *fixture, a int64) purge.RecipientArgs
		gone    []string
		// prefsGone 是偏好被刪的 (帳號, 受眾)：偏好是帳號與受眾的，沒有店別。
		prefsGone []string
	}
	scenarios := []scenario{
		{
			name: "AccountDeleted",
			publish: func(t *testing.T, f *fixture, a int64) purge.RecipientArgs {
				if err := f.identity.DeleteAccount(context.Background(), a); err != nil {
					t.Fatal(err)
				}
				return purge.RecipientArgs{AccountID: a}
			},
			gone:      []string{"A/BUYER", "A/STORE11", "A/STORE12"},
			prefsGone: []string{"A/BUYER", "A/STORE"},
		},
		{
			name: "BuyerDeleted",
			publish: func(t *testing.T, f *fixture, a int64) purge.RecipientArgs {
				ctx := context.Background()
				id, err := f.buyer.CurrentBuyerID(ctx, a)
				if err != nil {
					t.Fatal(err)
				}
				if err := f.buyer.BeginDeletion(ctx, id); err != nil {
					t.Fatal(err)
				}
				if err := f.buyer.CloseForDeletion(ctx, id); err != nil {
					t.Fatal(err)
				}
				return purge.RecipientArgs{AccountID: a, Audience: "BUYER"}
			},
			gone:      []string{"A/BUYER"},
			prefsGone: []string{"A/BUYER"},
		},
		{
			name: "StoreRetired",
			publish: func(t *testing.T, f *fixture, a int64) purge.RecipientArgs {
				err := f.db.InTx(context.Background(), func(ctx context.Context) error {
					return events.Publish(ctx, f.bus, seller.StoreRetired{OccurredAt: epoch, EventID: uuid.NewV7(), StoreID: 11, StoreCode: 1011, AccountID: a})
				})
				if err != nil {
					t.Fatal(err)
				}
				return purge.RecipientArgs{AccountID: a, Audience: "STORE", StoreID: 11}
			},
			gone:      []string{"A/STORE11"},
			prefsGone: []string{"A/STORE"},
		},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			f := setupIntake(t)
			a, b, rs := seedRecipients(t, f)
			f.jobs.takePurges()
			deliveriesBefore := map[string]int{}
			inboxBefore := map[string]int{}
			for name, r := range rs {
				deliveriesBefore[name], inboxBefore[name] = r.deliveries(t, f), r.inbox(t, f)
			}
			want := sc.publish(t, f, a)
			jobs := f.jobs.takePurges()
			if len(jobs) != 1 || jobs[0] != want {
				t.Fatalf("排出的清除 job = %+v，預期只有 %+v", jobs, want)
			}
			if err := f.module.purge.Recipient(context.Background(), jobs[0]); err != nil {
				t.Fatal(err)
			}
			isGone := func(name string) bool {
				for _, g := range sc.gone {
					if g == name {
						return true
					}
				}
				return false
			}
			for name, r := range rs {
				scrubbed, total := r.scrubbedIntents(t, f)
				if isGone(name) {
					if n := r.inbox(t, f); n != 0 {
						t.Errorf("%s 的收件匣還有 %d 列", name, n)
					}
					if n := r.deliveriesWithText(t, f); n != 0 {
						t.Errorf("%s 還有 %d 筆投遞紀錄留著送出文字", name, n)
					}
					if n := r.deliveries(t, f); n != deliveriesBefore[name] {
						t.Errorf("%s 的投遞紀錄列從 %d 變成 %d；列要留著（D16）", name, deliveriesBefore[name], n)
					}
					if scrubbed != total {
						t.Errorf("%s 的意圖參數只清了 %d/%d", name, scrubbed, total)
					}
					continue
				}
				if n := r.inbox(t, f); n != inboxBefore[name] {
					t.Errorf("%s 不在範圍內，收件匣卻從 %d 變成 %d", name, inboxBefore[name], n)
				}
				if n := r.deliveriesWithText(t, f); n != deliveriesBefore[name] {
					t.Errorf("%s 不在範圍內，送出文字被清了（%d/%d 筆還在）", name, n, deliveriesBefore[name])
				}
				if scrubbed != 0 {
					t.Errorf("%s 不在範圍內，意圖參數被清了 %d/%d", name, scrubbed, total)
				}
			}
			for _, p := range []struct {
				key     string
				account int64
				aud     string
			}{{"A/BUYER", a, "BUYER"}, {"A/STORE", a, "STORE"}, {"B/BUYER", b, "BUYER"}, {"B/STORE", b, "STORE"}} {
				gone := false
				for _, k := range sc.prefsGone {
					gone = gone || k == p.key
				}
				want := 1
				if gone {
					want = 0
				}
				if n := prefCount(t, f, p.account, p.aud); n != want {
					t.Errorf("%s 的偏好覆寫 %d 列，預期 %d", p.key, n, want)
				}
			}
			// 重跑不改變結果、不出錯（job 重試）。
			if err := f.module.purge.Recipient(context.Background(), jobs[0]); err != nil {
				t.Fatalf("重跑清除：%v", err)
			}
		})
	}
}

// 買家已關閉或店家已退場之後才處理的意圖，不替已清除的收件人建收件匣列或投遞紀錄；沒受影響的收件人照常寫入。
func TestIntentProcessedAfterBuyerDeletedOrStoreRetiredCreatesNothing(t *testing.T) {
	ctx := context.Background()
	t.Run("BuyerDeleted", func(t *testing.T) {
		f := setupIntake(t)
		gone, kept := f.newAccount(t), f.newAccount(t)
		id, err := f.buyer.CurrentBuyerID(ctx, gone)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.buyer.BeginDeletion(ctx, id); err != nil {
			t.Fatal(err)
		}
		if err := f.buyer.CloseForDeletion(ctx, id); err != nil {
			t.Fatal(err)
		}
		f.accept(t, "test_push", pushParams{AccountID: gone, EventID: "after-close"})
		f.accept(t, "test_push", pushParams{AccountID: kept, EventID: "kept"})
		f.run(t)
		if n := f.inboxCount(t, gone); n != 0 {
			t.Fatalf("已關閉的買家仍寫了 %d 列收件匣", n)
		}
		if n := f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1`, gone); n != 0 {
			t.Fatalf("已關閉的買家仍寫了 %d 筆投遞紀錄", n)
		}
		if f.inboxCount(t, kept) != 1 || f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1`, kept) != 2 {
			t.Fatal("其他買家的通知要照常寫入")
		}
	})
	t.Run("StoreRetired", func(t *testing.T) {
		f := setupIntake(t)
		acct := f.newAccount(t)
		f.stores.retire(11)
		f.accept(t, "test_store_note", storeNoteParams{AccountID: acct, StoreID: 11, EventID: "after-retire"})
		f.accept(t, "test_store_note", storeNoteParams{AccountID: acct, StoreID: 12, EventID: "other-store"})
		f.run(t)
		if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND store_id = 11`, acct); n != 0 {
			t.Fatalf("已退場的店仍寫了 %d 列收件匣", n)
		}
		if n := f.count(t, `SELECT count(*) FROM notification.deliveries WHERE account_id = $1 AND store_id = 11`, acct); n != 0 {
			t.Fatalf("已退場的店仍寫了 %d 筆投遞紀錄", n)
		}
		if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND store_id = 12`, acct); n != 1 {
			t.Fatalf("同帳號另一間店的通知要照常寫入：%d 列", n)
		}
	})
}

// retentionFixture 在固定時鐘下直接寫資料：每筆的年齡是相對 now 的天數。
type retentionFixture struct {
	f   *fixture
	now time.Time
	svc *purge.Service
}

func newRetentionFixture(t *testing.T) *retentionFixture {
	f := setupIntake(t)
	now := time.Date(2027, 6, 1, 12, 0, 0, 0, time.UTC)
	return &retentionFixture{f: f, now: now, svc: purge.New(purge.Deps{DB: f.db, Jobs: f.jobs, Now: func() time.Time { return now }})}
}

func (r *retentionFixture) ago(days int) time.Time {
	return r.now.Add(-time.Duration(days) * 24 * time.Hour)
}

func (r *retentionFixture) intent(t *testing.T, key string, ageDays int) uuid.UUID {
	id := uuid.NewV7()
	r.f.mustExec(t, `INSERT INTO notification.intents (id, type, idempotency_key, params, occurred_at, created_at) VALUES ($1, 'test_push', $2, '{"x":1}', $3, $3)`, id, key, r.ago(ageDays))
	return id
}

func (r *retentionFixture) delivery(t *testing.T, intent uuid.UUID, ageDays int) uuid.UUID {
	id := uuid.NewV7()
	r.f.mustExec(t, `INSERT INTO notification.deliveries (id, intent_id, account_id, audience, channel, status, created_at, updated_at) VALUES ($1, $2, 1, 'BUYER', 'PUSH', 'PENDING', $3, $3)`, id, intent, r.ago(ageDays))
	return id
}

// inbox 寫一列收件匣：建立得很早，保留期看 updatedDays（最後一次合併）。
func (r *retentionFixture) inbox(t *testing.T, updatedDays int, linked ...uuid.UUID) uuid.UUID {
	id := uuid.NewV7()
	r.f.mustExec(t, `INSERT INTO notification.inbox_items (id, account_id, audience, type, params, occurred_at, created_at, updated_at) VALUES ($1, 1, 'BUYER', 'test_push', '{}', $2, $2, $3)`, id, r.ago(400), r.ago(updatedDays))
	for _, in := range linked {
		r.f.mustExec(t, `INSERT INTO notification.inbox_item_intents (intent_id, inbox_item_id) VALUES ($1, $2)`, in, id)
	}
	return id
}

func (r *retentionFixture) exists(t *testing.T, table string, id uuid.UUID) bool {
	return r.f.count(t, fmt.Sprintf(`SELECT count(*) FROM notification.%s WHERE id = $1`, table), id) == 1
}

// 超過 180 天的收件匣、投遞紀錄與意圖硬刪，179 天的留著；過期意圖只有在已沒有投遞紀錄時才刪，投遞紀錄還年輕的意圖連同它的投遞紀錄留著
// （作業不能因為 RESTRICT 失敗）。收件匣以 updated_at 起算，不看 created_at。
func TestRetentionSweepDeletesExpiredRowsOnly(t *testing.T) {
	r := newRetentionFixture(t)
	oldIntentOldDelivery := r.intent(t, "k-old-old", 181)
	oldDelivery := r.delivery(t, oldIntentOldDelivery, 181)
	oldIntentNoDelivery := r.intent(t, "k-old-none", 181)
	oldIntentYoungDelivery := r.intent(t, "k-old-young", 181)
	youngDelivery := r.delivery(t, oldIntentYoungDelivery, 179)
	youngIntent := r.intent(t, "k-young", 179)
	youngIntentDelivery := r.delivery(t, youngIntent, 179)
	oldInbox := r.inbox(t, 181)
	youngInbox := r.inbox(t, 179, oldIntentNoDelivery)

	swept, err := r.svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("清除要成功（過期意圖還有年輕的投遞紀錄時不能出錯）：%v", err)
	}
	if swept.Deliveries != 1 || swept.Intents != 2 || swept.InboxItems != 1 {
		t.Fatalf("清除列數 = %+v，預期 投遞 1、意圖 2、收件匣 1", swept)
	}
	for _, c := range []struct {
		name  string
		table string
		id    uuid.UUID
		want  bool
	}{
		{"過期投遞", "deliveries", oldDelivery, false},
		{"過期意圖（投遞也過期）", "intents", oldIntentOldDelivery, false},
		{"過期意圖（沒有投遞）", "intents", oldIntentNoDelivery, false},
		{"過期意圖（投遞年輕）", "intents", oldIntentYoungDelivery, true},
		{"年輕投遞（意圖過期）", "deliveries", youngDelivery, true},
		{"年輕意圖", "intents", youngIntent, true},
		{"年輕意圖的投遞", "deliveries", youngIntentDelivery, true},
		{"過期收件匣", "inbox_items", oldInbox, false},
		{"updated_at 179 天的收件匣（created_at 更早）", "inbox_items", youngInbox, true},
	} {
		if got := r.exists(t, c.table, c.id); got != c.want {
			t.Errorf("%s：存在 = %v，預期 %v", c.name, got, c.want)
		}
	}
	if n := r.f.count(t, `SELECT count(*) FROM notification.inbox_item_intents`); n != 0 {
		t.Errorf("意圖被刪時連結要跟著刪，還剩 %d 筆", n)
	}
	// 第二次沒有東西可刪。
	again, err := r.svc.Sweep(context.Background())
	if err != nil || again != (purge.Swept{}) {
		t.Fatalf("重跑 = %+v、%v，預期什麼都不刪", again, err)
	}
}
