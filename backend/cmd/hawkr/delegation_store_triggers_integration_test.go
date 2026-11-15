//go:build integration

package main

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// 正式組裝（newDelegation，serve、worker 與 CLI 共用；這裡經 newDelegationOffline 走到它）訂閱了 seller 的 StoreSuspended、StoreClosed 與 StorePendingDeletion：事件在 transaction 內
// 結束該店進行中的代行 session（expires_at 拉回現在、不寫 REVOKED），別家店的 session 不受影響。拿掉 newDelegation 裡的 events.Subscribe(bus, m.OnStoreSuspended)
// 、events.Subscribe(bus, m.OnStoreClosed) 或 events.Subscribe(bus, m.OnStorePendingDeletion)，對應的子測試失敗。
func TestNewDelegationEndsSessionsOfASuspendedOrClosedStore(t *testing.T) {
	for name, publish := range map[string]func(ctx context.Context, bus *events.Bus, storeID int64) error{
		"StoreSuspended": func(ctx context.Context, bus *events.Bus, storeID int64) error {
			return events.Publish(ctx, bus, seller.StoreSuspended{EventID: uuid.NewV7(), OccurredAt: time.Now(), StoreID: storeID, AccountID: storeID + 100, PeriodID: 1})
		},
		"StorePendingDeletion": func(ctx context.Context, bus *events.Bus, storeID int64) error {
			return events.Publish(ctx, bus, seller.StorePendingDeletion{EventID: uuid.NewV7(), OccurredAt: time.Now(), StoreID: storeID, StoreCode: storeID + 200, AccountID: storeID + 100})
		},
		"StoreClosed": func(ctx context.Context, bus *events.Bus, storeID int64) error {
			return events.Publish(ctx, bus, seller.StoreClosed{EventID: uuid.NewV7(), OccurredAt: time.Now(), StoreID: storeID, StoreCode: storeID + 200, AccountID: storeID + 100})
		},
	} {
		t.Run(name, func(t *testing.T) {
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
			var adminA, adminB int64
			for i, dst := range []*int64{&adminA, &adminB} {
				if err := d.Pool().QueryRow(ctx, `INSERT INTO identity.admins (access_subject, email) VALUES ($1, $2) RETURNING id`,
					[]string{"sub-a", "sub-b"}[i], []string{"a@example.com", "b@example.com"}[i]).Scan(dst); err != nil {
					t.Fatal(err)
				}
			}
			session := func(admin, storeID int64, code, hash string) int64 {
				var id int64
				if err := d.Pool().QueryRow(ctx, `INSERT INTO delegation.delegated_operation_sessions (
					display_code, credential_hash, actor_admin_id, actor_display_name, target_store_id,
					target_seller_account_id, target_store_name, reason_code, reason_label, capabilities,
					issued_at, expires_at, last_active_at, status, idempotency_key, create_request_id, create_correlation_id
				) VALUES ($1, decode(repeat($2, 32), 'hex'), $3, 'ops', $4, 2, 'store',
					'SELLER_REQUEST', 'seller request', ARRAY['posts.read'],
					date_trunc('second', now()), date_trunc('second', now()) + interval '30 minutes',
					date_trunc('second', now()), 'ACTIVE', $1, $1, $1) RETURNING id`, code, hash, admin, storeID).Scan(&id); err != nil {
					t.Fatal(err)
				}
				return id
			}
			target, other := session(adminA, 1, "OPS-0000000000011", "33"), session(adminB, 2, "OPS-0000000000012", "44")

			bus := events.NewBus()
			log := newLogger()
			id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
			if _, err := newDelegationOffline(d, bus, id, log); err != nil {
				t.Fatal(err)
			}
			if err := d.InTx(ctx, func(ctx context.Context) error { return publish(ctx, bus, 1) }); err != nil {
				t.Fatalf("publish：%v", err)
			}
			state := func(sessionID int64) (ended bool, status string, expired bool) {
				if err := d.Pool().QueryRow(ctx, `SELECT ended_at IS NOT NULL, status, expires_at <= now() FROM delegation.delegated_operation_sessions WHERE id = $1`, sessionID).Scan(&ended, &status, &expired); err != nil {
					t.Fatal(err)
				}
				return ended, status, expired
			}
			if ended, status, expired := state(target); !ended || status != "ACTIVE" || !expired {
				t.Fatalf("該店的 session：ended=%v status=%s expired=%v，預期已結束（不是撤銷）並到期（訂閱沒接上？）", ended, status, expired)
			}
			if ended, status, expired := state(other); ended || status != "ACTIVE" || expired {
				t.Fatalf("別家店的 session 不受影響：ended=%v status=%s expired=%v", ended, status, expired)
			}
		})
	}
}
