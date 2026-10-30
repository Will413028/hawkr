//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// CLI 的撤銷與 serve 用同一個組裝（newDelegation）：撤掉 OWNER 時，delegation 的訂閱在同一個 transaction 內撤銷該管理員進行中的
// 代行；授予與撤銷各留下 source 是 CLI 的事件。
func TestAdminRoleCLIRevokeEndsRunningDelegations(t *testing.T) {
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
	var adminID int64
	if err := d.Pool().QueryRow(ctx, `INSERT INTO identity.admins (access_subject, email) VALUES ('sub-1', 'Ops@Example.com') RETURNING id`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runAdminRole(ctx, []string{"grant", "-admin", "ops@example.com", "-role", "OWNER", "-reason", "上線前授予自己"}, &out); err != nil {
		t.Fatalf("grant：%v", err)
	}
	if err := runAdminRole(ctx, []string{"grant", "-admin", "ops@example.com", "-role", "OWNER", "-reason", "重複"}, &out); err == nil {
		t.Fatal("重複授予應該失敗")
	}

	_, err = d.Pool().Exec(ctx, `INSERT INTO delegation.delegated_operation_sessions (
		display_code, credential_hash, actor_admin_id, actor_display_name, target_store_id,
		target_seller_account_id, target_store_name, reason_code, reason_label, capabilities,
		issued_at, expires_at, last_active_at, status, idempotency_key, create_request_id, create_correlation_id
	) VALUES ('OPS-0000000000001', decode(repeat('11', 32), 'hex'), $1, 'ops', 1, 2, 'store',
		'SELLER_REQUEST', 'seller request', ARRAY['products.read'],
		date_trunc('second', now()), date_trunc('second', now()) + interval '30 minutes',
		date_trunc('second', now()), 'ACTIVE', 'cli-test', 'cli-test', 'cli-test')`, adminID)
	if err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := runAdminRole(ctx, []string{"revoke", "-admin", "ops@example.com", "-role", "OWNER", "-reason", "離職"}, &out); err != nil {
		t.Fatalf("revoke：%v", err)
	}
	if !strings.Contains(out.String(), "警告") {
		t.Errorf("撤掉最後一個 OWNER 應該警告：%q", out.String())
	}
	var status, reason string
	if err := d.Pool().QueryRow(ctx, `SELECT status, revoke_reason FROM delegation.delegated_operation_sessions WHERE actor_admin_id = $1`, adminID).Scan(&status, &reason); err != nil {
		t.Fatal(err)
	}
	if status != "REVOKED" || reason != "admin_role_revoked" {
		t.Fatalf("代行 = %s %s", status, reason)
	}
	rows, err := d.Pool().Query(ctx, `SELECT action, source, reason, os_user IS NOT NULL FROM identity.admin_role_events WHERE admin_id = $1 ORDER BY occurred_at, action`, adminID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var action, source, reason string
		var hasOSUser bool
		if err := rows.Scan(&action, &source, &reason, &hasOSUser); err != nil {
			t.Fatal(err)
		}
		got = append(got, action+"/"+source+"/"+reason)
	}
	if len(got) != 2 || got[0] != "GRANT/CLI/上線前授予自己" || got[1] != "REVOKE/CLI/離職" {
		t.Fatalf("事件 = %v", got)
	}
}

// 本機 seed 的 dev 管理員是 OWNER，授予留下一筆事件，重跑 seed 不重複。
func TestSeedGrantsTheDevAdminOwnerOnce(t *testing.T) {
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
	seed, err := os.ReadFile(filepath.Join("..", "..", "infra", "seed-data", "01-account.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := d.Pool().Exec(ctx, string(seed)); err != nil {
			t.Fatalf("seed：%v", err)
		}
	}
	var roles, events int
	if err := d.Pool().QueryRow(ctx, `SELECT count(*) FROM identity.admin_roles r JOIN identity.admins a ON a.id = r.admin_id WHERE a.access_subject = 'dev_ops' AND r.role = 'OWNER'`).Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if err := d.Pool().QueryRow(ctx, `SELECT count(*) FROM identity.admin_role_events e JOIN identity.admins a ON a.id = e.admin_id WHERE a.access_subject = 'dev_ops' AND e.action = 'GRANT' AND e.source = 'CLI'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if roles != 1 || events != 1 {
		t.Fatalf("dev_ops 的 OWNER = %d 列、GRANT 事件 = %d 筆，預期各 1", roles, events)
	}
}
