//go:build integration

package catalog

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
)

// racingMedia 在第一次 Confirm 之前先跑 before（贏家的整個 CreateProduct 在這裡提交），再回 media 在 token 已被別的 transaction 確認時的錯誤。
type racingMedia struct {
	Media
	once   sync.Once
	before func()
}

func (r *racingMedia) Confirm(ctx context.Context, u media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error) {
	ran := false
	r.once.Do(func() { r.before(); ran = true })
	if ran {
		return media.ConfirmedAsset{}, errors.Join(apperr.FailedPrecondition("used"), media.ErrUploadAlreadyConfirmed)
	}
	return r.Media.Confirm(ctx, u, token, want)
}

func TestCreateLoserReadsBackTheWinnerWhenItsImageTokenWasConfirmedElsewhere(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	up := media.Uploader{ID: acct}
	in := ProductInput{Title: "併發", CategoryID: f.leaf(), Skus: []SkuInput{{Price: 100}}, Gallery: []Image{{Token: "race1"}}}
	key := uuid.NewV7()

	var winner Product
	rm := &racingMedia{Media: f.media}
	rm.before = func() {
		w, err := f.m.CreateProduct(f.ct, storeID, up, key, in)
		if err != nil {
			t.Error(err)
		}
		winner = w
	}
	loser := New(f.deps(f.d, rm))
	// 輸家的第一次查詢（transaction 內）還沒有商品；贏家在輸家確認圖片時才提交。
	got, err := loser.CreateProduct(f.ct, storeID, up, key, in)
	if err != nil {
		t.Fatalf("the loser reads back the winner: %v", err)
	}
	if winner.ID == (uuid.UUID{}) || got.ID != winner.ID {
		t.Fatalf("the loser returns the winner's product: %v vs %v", got.ID, winner.ID)
	}
	if n := f.count(`SELECT count(*) FROM catalog.products WHERE store_id = $1`, storeID); n != 1 {
		t.Fatalf("one product, got %d", n)
	}
}

// 在 B4-2b 時期寫下的 ADJUST 操作沒有 request_hash；migration 20261125175058 以操作唯一的 movement 重算它，
// 同鍵重送（同內容）因此仍是重送，不同內容仍是鍵重用；沒有 movement 的操作任何重送都是鍵重用。
func TestMigrationBackfillsTheRequestHashOfPreexistingAdjustOperations(t *testing.T) {
	t.Parallel()
	const target = "20261125175058"
	d := pg.NewDB(t)
	ctx := context.Background()
	f := &fixture{t: t, d: d, ct: ctx}
	f.must(migrate.InstallExtensions(ctx, d))

	dir := Migrations()
	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	apply := func(name string) {
		t.Helper()
		body, err := fs.ReadFile(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		f.must(f.exec(string(body)))
	}
	var targetFile string
	for _, n := range names {
		if strings.HasPrefix(n, target) {
			targetFile = n
			continue
		}
		if n < target {
			apply(n)
		}
	}
	if targetFile == "" {
		t.Fatalf("migration %s not found", target)
	}

	sku, noMovementSku := f.sku(12), f.sku(5)
	op, orphan := uuid.NewV7(), uuid.NewV7()
	f.must(f.exec(`INSERT INTO catalog.stock_operations (id, idempotency_key, kind, store_id, created_at) VALUES ($1, 'legacy:1', 'ADJUST', 501, $2), ($3, 'legacy:orphan', 'ADJUST', 501, $2)`, op, epoch, orphan))
	f.must(f.exec(`INSERT INTO catalog.stock_movements (operation_id, sku_id, delta) VALUES ($1, $2, 7)`, op, sku))

	apply(targetFile)
	f.m = New(Deps{DB: d, NewID: uuid.NewV7, Now: func() time.Time { return epoch }})

	f.must(f.m.AdjustStock(ctx, 501, "legacy:1", sku, 7))
	if got := f.stockOf(sku); got != 12 {
		t.Fatalf("a replay of a pre-migration operation must not apply again: stock = %d, want 12", got)
	}
	if err := f.m.AdjustStock(ctx, 501, "legacy:1", sku, 8); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("different content under a pre-migration key = %v, want ErrIdempotencyKeyReused", err)
	}
	if err := f.m.AdjustStock(ctx, 501, "legacy:orphan", noMovementSku, 1); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("an operation without a movement treats any replay as key reuse: %v", err)
	}
}
