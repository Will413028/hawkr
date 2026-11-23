//go:build integration

package notification

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// setup 給這支測試一個只套用了本模組 migration 的 database：integration 測試只跑在自己的 schema 上。排入 River
// job 的模組在前面加 testdb.Jobs()（見 content 模組）。獨立的測試開頭呼叫 t.Parallel()。
func setup(t *testing.T) *db.DB {
	t.Helper()
	return pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "notification", Dir: Migrations()}))
}

var epoch = time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

func exec(t *testing.T, d *db.DB, sql string, args ...any) error {
	t.Helper()
	_, err := d.Pool().Exec(context.Background(), sql, args...)
	return err
}

func mustExec(t *testing.T, d *db.DB, sql string, args ...any) {
	t.Helper()
	if err := exec(t, d, sql, args...); err != nil {
		t.Fatal(err)
	}
}

// wantConstraint 要求 err 是違反名為 name 的約束（唯一鍵 23505、CHECK 23514）。
func wantConstraint(t *testing.T, err error, code, name string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code || pgErr.ConstraintName != name {
		t.Fatalf("預期違反 %s（%s），實際：%v", name, code, err)
	}
}

func newIntent(t *testing.T, d *db.DB, key string) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	mustExec(t, d, `INSERT INTO notification.intents (id, type, idempotency_key, params, created_at) VALUES ($1, 'account_suspended', $2, '{}', $3)`, id, key, epoch)
	return id
}

// 意圖冪等：同一個事件來源身分只能有一個意圖（A8-1b 的 worker 靠它做到「投遞兩次只有一列」）。
func TestIntentIdempotencyKeyIsUnique(t *testing.T) {
	t.Parallel()
	d := setup(t)
	newIntent(t, d, "account_suspended:x")
	err := exec(t, d, `INSERT INTO notification.intents (id, type, idempotency_key, params, created_at) VALUES ($1, 'account_suspended', 'account_suspended:x', '{}', $2)`, uuid.NewV7(), epoch)
	wantConstraint(t, err, "23505", "intents_idempotency_key_key")
}

// 投遞冪等：(意圖, 帳號, 受眾, 渠道) 只有一列；不同渠道各一列；收件匣不是投遞渠道。
func TestDeliveryIsUniquePerIntentRecipientAndChannel(t *testing.T) {
	t.Parallel()
	d := setup(t)
	intent := newIntent(t, d, "k")
	insert := func(channel string) error {
		return exec(t, d, `INSERT INTO notification.deliveries (id, intent_id, account_id, audience, channel, status, created_at, updated_at)
			VALUES ($1, $2, 7, 'BUYER', $3, 'PENDING', $4, $4)`, uuid.NewV7(), intent, channel, epoch)
	}
	if err := insert("EMAIL"); err != nil {
		t.Fatal(err)
	}
	if err := insert("PUSH"); err != nil {
		t.Fatal(err)
	}
	wantConstraint(t, insert("EMAIL"), "23505", "deliveries_recipient_channel_key")
	wantConstraint(t, insert("INBOX"), "23514", "deliveries_channel_check")
}

// 收件人是 (account_id, 受眾, store_id)：BUYER 沒有 store_id，STORE 一定有。
func TestRecipientShapeIsEnforced(t *testing.T) {
	t.Parallel()
	d := setup(t)
	intent := newIntent(t, d, "k")
	insert := func(audience string, storeID *int64) error {
		return exec(t, d, `INSERT INTO notification.inbox_items (id, account_id, audience, store_id, type, params, intent_id, created_at, updated_at)
			VALUES ($1, 7, $2, $3, 'account_suspended', '{}', $4, $5, $5)`, uuid.NewV7(), audience, storeID, intent, epoch)
	}
	store := int64(9)
	if err := insert("BUYER", nil); err != nil {
		t.Fatal(err)
	}
	if err := insert("STORE", &store); err != nil {
		t.Fatal(err)
	}
	wantConstraint(t, insert("BUYER", &store), "23514", "inbox_items_store_check")
	wantConstraint(t, insert("STORE", nil), "23514", "inbox_items_store_check")
	wantConstraint(t, insert("SELLER", &store), "23514", "inbox_items_audience_check")
}

// 有聚合鍵的未讀列同一個收件人只有一列；已讀之後的新意圖另開一列。
func TestUnreadAggregationKeyIsUniquePerRecipient(t *testing.T) {
	t.Parallel()
	d := setup(t)
	intent := newIntent(t, d, "k")
	insert := func(account int64) error {
		return exec(t, d, `INSERT INTO notification.inbox_items (id, account_id, audience, type, params, aggregation_key, intent_id, created_at, updated_at)
			VALUES ($1, $2, 'BUYER', 'account_suspended', '{}', 'like:1', $3, $4, $4)`, uuid.NewV7(), account, intent, epoch)
	}
	if err := insert(7); err != nil {
		t.Fatal(err)
	}
	if err := insert(8); err != nil {
		t.Fatalf("不同收件人不互相影響：%v", err)
	}
	wantConstraint(t, insert(7), "23505", "inbox_items_unread_aggregation")
	mustExec(t, d, `UPDATE notification.inbox_items SET read_at = $1 WHERE account_id = 7`, epoch)
	if err := insert(7); err != nil {
		t.Fatalf("已讀之後可以另開一列：%v", err)
	}
}

// token 是主鍵：同一個 token 只綁一個帳號與一支 App（登入時 upsert 改綁，不是新增第二列）。
func TestDeviceTokenIsBoundToOneAccount(t *testing.T) {
	t.Parallel()
	d := setup(t)
	mustExec(t, d, `INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at) VALUES ('t1', 7, 'BUYER', $1)`, epoch)
	wantConstraint(t, exec(t, d, `INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at) VALUES ('t1', 8, 'BUYER', $1)`, epoch), "23505", "device_tokens_pkey")
	wantConstraint(t, exec(t, d, `INSERT INTO notification.device_tokens (token, account_id, app, last_registered_at) VALUES ('t2', 7, 'WEB', $1)`, epoch), "23514", "device_tokens_app_check")
}

// 意圖先被清除（依 created_at 掃）時，收件匣列留著：它的保留期從 updated_at（最後一次合併）起算，不隨來源意圖消失；intent_id 設為 NULL。
func TestDeletingTheIntentKeepsTheInboxItem(t *testing.T) {
	t.Parallel()
	d := setup(t)
	intent := newIntent(t, d, "k")
	item := uuid.NewV7()
	mustExec(t, d, `INSERT INTO notification.inbox_items (id, account_id, audience, type, params, aggregation_key, intent_id, created_at, updated_at)
		VALUES ($1, 7, 'BUYER', 'account_suspended', '{}', 'like:1', $2, $3, $3)`, item, intent, epoch)
	mustExec(t, d, `DELETE FROM notification.intents WHERE id = $1`, intent)
	var intentID *uuid.UUID
	if err := d.Pool().QueryRow(context.Background(), `SELECT intent_id FROM notification.inbox_items WHERE id = $1`, item).Scan(&intentID); err != nil {
		t.Fatalf("收件匣列應該還在：%v", err)
	}
	if intentID != nil {
		t.Errorf("intent_id = %v，預期 NULL", *intentID)
	}
}

// 同一個帳號先後擁有兩間店（舊店已關閉）時，兩間店是兩個收件人：未讀聚合列與投遞列都不互相合併、不互相擋。
func TestTwoStoresOfOneAccountAreSeparateRecipients(t *testing.T) {
	t.Parallel()
	d := setup(t)
	intent := newIntent(t, d, "k")
	insertItem := func(store int64) error {
		return exec(t, d, `INSERT INTO notification.inbox_items (id, account_id, audience, store_id, type, params, aggregation_key, intent_id, created_at, updated_at)
			VALUES ($1, 7, 'STORE', $2, 'account_suspended', '{}', 'follow:1', $3, $4, $4)`, uuid.NewV7(), store, intent, epoch)
	}
	if err := insertItem(9); err != nil {
		t.Fatal(err)
	}
	if err := insertItem(10); err != nil {
		t.Fatalf("同帳號的另一間店是另一個收件人：%v", err)
	}
	wantConstraint(t, insertItem(9), "23505", "inbox_items_unread_aggregation")

	insertDelivery := func(audience string, store *int64) error {
		return exec(t, d, `INSERT INTO notification.deliveries (id, intent_id, account_id, audience, store_id, channel, status, created_at, updated_at)
			VALUES ($1, $2, 7, $3, $4, 'PUSH', 'PENDING', $5, $5)`, uuid.NewV7(), intent, audience, store, epoch)
	}
	s9, s10 := int64(9), int64(10)
	if err := insertDelivery("STORE", &s9); err != nil {
		t.Fatal(err)
	}
	if err := insertDelivery("STORE", &s10); err != nil {
		t.Fatalf("同帳號的另一間店各有自己的投遞列：%v", err)
	}
	wantConstraint(t, insertDelivery("STORE", &s9), "23505", "deliveries_recipient_channel_key")
	if err := insertDelivery("BUYER", nil); err != nil {
		t.Fatal(err)
	}
	wantConstraint(t, insertDelivery("BUYER", nil), "23505", "deliveries_recipient_channel_key")
}

// BUYER 列的 store_id 是 NULL：未讀聚合的唯一鍵用 NULLS NOT DISTINCT，NULL 不會讓同一個買家有兩列未讀的同聚合鍵。
// （TestUnreadAggregationKeyIsUniquePerRecipient 已涵蓋；這裡確認買家與店家身分的同帳號互不相擋。）
func TestBuyerAndStoreIdentitiesOfOneAccountDoNotCollide(t *testing.T) {
	t.Parallel()
	d := setup(t)
	intent := newIntent(t, d, "k")
	mustExec(t, d, `INSERT INTO notification.inbox_items (id, account_id, audience, type, params, aggregation_key, intent_id, created_at, updated_at)
		VALUES ($1, 7, 'BUYER', 'account_suspended', '{}', 'k:1', $2, $3, $3)`, uuid.NewV7(), intent, epoch)
	mustExec(t, d, `INSERT INTO notification.inbox_items (id, account_id, audience, store_id, type, params, aggregation_key, intent_id, created_at, updated_at)
		VALUES ($1, 7, 'STORE', 9, 'account_suspended', '{}', 'k:1', $2, $3, $3)`, uuid.NewV7(), intent, epoch)
}
