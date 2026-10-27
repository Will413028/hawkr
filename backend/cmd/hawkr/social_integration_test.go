//go:build integration

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// social-recount 對全部可重算的計數覆寫並印出漂移；沒有漂移時印 0。
func TestSocialRecountRewritesDriftedCounts(t *testing.T) {
	url := pg.NewURL(t)
	t.Setenv("DATABASE_URL", url)
	ctx := context.Background()
	if err := run(ctx, []string{"migrate"}); err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, `INSERT INTO social.posts (id, store_id, body, status, created_at, updated_at)
		VALUES ('0196f2a0-0000-7000-8000-000000000001', 1, 'x', 'DRAFT', now(), now())`); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runSocialRecount(ctx, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "貼文 1") || !strings.Contains(out.String(), "漂移 0") {
		t.Fatalf("%q", out.String())
	}
	if _, err := conn.Exec(ctx, "UPDATE social.post_stats SET comment_count = 5"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runSocialRecount(ctx, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "漂移 1") || !strings.Contains(out.String(), "POST") {
		t.Fatalf("%q", out.String())
	}
	var n int64
	if err := conn.QueryRow(ctx, "SELECT comment_count FROM social.post_stats").Scan(&n); err != nil || n != 0 {
		t.Fatalf("覆寫成重算的結果：%d %v", n, err)
	}
}
