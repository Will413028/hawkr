//go:build integration

package main

import (
	"context"
	"strconv"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// 正式組裝（newDelegation，serve、worker 與 CLI 共用；這裡經 newDelegationOffline 走到它）訂閱了 PostPublished：代行發起的排程發布事件
// 在 transaction 內產生一列 SUCCEEDED 稽核。拿掉 newDelegation 裡的 events.SubscribeNamed(bus, delegatedactor.AuditHandler, m.OnPostPublished)，這個測試失敗。
func TestNewDelegationWritesScheduledPublishAudit(t *testing.T) {
	ctx := context.Background()
	url := pg.NewURL(t)
	t.Setenv("DATABASE_URL", url)
	if err := run(ctx, []string{"migrate"}); err != nil {
		t.Fatalf("migrate：%v", err)
	}
	d, err := db.Open(ctx, db.Config{URL: url, MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	var adminID, sessionID int64
	if err := d.Pool().QueryRow(ctx, `INSERT INTO identity.admins (access_subject, email) VALUES ('sub-1', 'ops@example.com') RETURNING id`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if err := d.Pool().QueryRow(ctx, `INSERT INTO delegation.delegated_operation_sessions (
		display_code, credential_hash, actor_admin_id, actor_display_name, target_store_id,
		target_seller_account_id, target_store_name, reason_code, reason_label, capabilities,
		issued_at, expires_at, last_active_at, status, idempotency_key, create_request_id, create_correlation_id
	) VALUES ('OPS-0000000000002', decode(repeat('22', 32), 'hex'), $1, 'ops', 1, 2, 'store',
		'SELLER_REQUEST', 'seller request', ARRAY['posts.publish'],
		date_trunc('second', now()), date_trunc('second', now()) + interval '30 minutes',
		date_trunc('second', now()), 'ACTIVE', 'wire-test', 'wire-test', 'wire-test') RETURNING id`, adminID).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}

	bus := events.NewBus()
	log := newLogger()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	if _, err := newDelegationOffline(d, bus, id, log); err != nil {
		t.Fatal(err)
	}

	scheduleID, postID := uuid.NewV7(), uuid.NewV7()
	ev := social.PostPublished{
		EventID: uuid.NewV7(), OccurredAt: time.Now(), PostID: postID, StoreID: 1, Preview: "hello",
		Origin: social.OriginScheduled, ScheduleID: scheduleID,
		ScheduledBy: social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: strconv.FormatInt(sessionID, 10)},
	}
	if err := d.InTx(ctx, func(ctx context.Context) error { return events.Publish(ctx, bus, ev) }); err != nil {
		t.Fatalf("publish：%v", err)
	}
	var result string
	if err := d.Pool().QueryRow(ctx, `SELECT result FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND idempotency_key = $2`,
		sessionID, "schedule:"+scheduleID.String()).Scan(&result); err != nil {
		t.Fatalf("沒有排程發布的稽核列（OnPostPublished 沒被訂閱？）：%v", err)
	}
	if result != "SUCCEEDED" {
		t.Fatalf("result = %s", result)
	}
}
