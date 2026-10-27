//go:build integration

package identity_test

import (
	"context"
	"io/fs"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/identity"
)

func TestAdminAccessMigrationPreservesExistingRows(t *testing.T) {
	t.Parallel()
	const target = "20260927124651_admin_access.sql"
	ctx := context.Background()
	d := pg.NewDB(t)
	dir := Migrations()
	files, err := fs.Glob(dir, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range files {
		if name == target {
			found = true
			break
		}
		body, err := fs.ReadFile(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Pool().Exec(ctx, string(body)); err != nil {
			t.Fatalf("套用 %s：%v", name, err)
		}
	}
	if !found {
		t.Fatalf("找不到 %s", target)
	}
	const subject = "dev_ops"
	var id int64
	if err := d.Pool().QueryRow(ctx, `INSERT INTO identity.admins (clerk_user_id, email, name, picture_url) VALUES ($1, $2, $3, $4) RETURNING id`, subject, "ops@dev.localhost", "ops", "https://example.test/avatar").Scan(&id); err != nil {
		t.Fatal(err)
	}
	body, err := fs.ReadFile(dir, target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(ctx, string(body)); err != nil {
		t.Fatal(err)
	}
	var gotID int64
	var gotSubject, gotEmail string
	if err := d.Pool().QueryRow(ctx, `SELECT id, access_subject, email FROM identity.admins WHERE id = $1`, id).Scan(&gotID, &gotSubject, &gotEmail); err != nil {
		t.Fatal(err)
	}
	if gotID != id || gotSubject != subject || gotEmail != "ops@dev.localhost" {
		t.Fatalf("管理員資料 = %d, %q, %q", gotID, gotSubject, gotEmail)
	}
}
