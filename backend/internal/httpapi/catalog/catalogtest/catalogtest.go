//go:build integration

package catalogtest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

// Notifier 是測試用的補貨通知替身：job 不在這些測試執行（只有排入），兩個呼叫都是 no-op。
type Notifier struct{}

// NotifyRestocked 什麼都不做。
func (Notifier) NotifyRestocked(context.Context, catalog.RestockedSKU) error { return nil }

// PurgeSKUSubscriptions 什麼都不做。
func (Notifier) PurgeSKUSubscriptions(context.Context, []uuid.UUID) error { return nil }

// World 是一個測試的資料庫與 catalog 模組；時間固定，由 Advance 推進。
type World struct {
	T      *testing.T
	DB     *db.DB
	Module *catalog.Module
	Media  *media.Fake
	Bus    *events.Bus
	mu     sync.Mutex
	now    time.Time
	seq    int64
}

// New 建立世界：套用 media、buyer、seller、catalog 的 migration（catalog 讀 seller 的 pub_exposable_stores）。
func New(t *testing.T, pg *testdb.Server) *World {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: catalog.Migrations()},
	))
	w := &World{T: t, DB: d, Media: media.NewFake(), Bus: events.NewBus(), now: time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)}
	w.Module = catalog.New(catalog.Deps{DB: d, NewID: uuid.NewV7, Now: w.Now, Media: w.Media, Events: w.Bus, Restock: Notifier{}})
	return w
}

// Now 是世界的時間（也是 handler 的 clock）。
func (w *World) Now() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.now
}

// Exec 執行 SQL，失敗就結束測試。
func (w *World) Exec(sql string, args ...any) {
	w.T.Helper()
	if _, err := w.DB.Pool().Exec(context.Background(), sql, args...); err != nil {
		w.T.Fatalf("%v\n%s", err, sql)
	}
}

// Store 建一家可以公開曝光的店（直接寫 seller 的表，最小的列），回傳 store_id 與帳號 ID。
func (w *World) Store() (storeID, accountID int64) {
	w.T.Helper()
	ctx := context.Background()
	w.mu.Lock()
	w.seq++
	w.mu.Unlock()
	var err error
	if accountID, err = sellerfixture.NewAccount(ctx, w.DB, w.Now()); err != nil {
		w.T.Fatal(err)
	}
	if storeID, err = sellerfixture.ExposableStoreNamed(ctx, w.DB, accountID, w.Now(), "測試店"); err != nil {
		w.T.Fatal(err)
	}
	return storeID, accountID
}

func (w *World) SetExposable(storeID int64, ok bool) {
	w.T.Helper()
	if err := sellerfixture.SetPayoutCapability(context.Background(), w.DB, storeID, w.Now(), ok); err != nil {
		w.T.Fatal(err)
	}
}

// Leaf 是一個葉節點分類。
func (w *World) Leaf() uuid.UUID {
	w.T.Helper()
	var id uuid.UUID
	if err := w.DB.Pool().QueryRow(context.Background(), `SELECT id FROM catalog.categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		w.T.Fatal(err)
	}
	return id
}

// Input 是一份可以上架的輸入：一個 SKU（庫存 stock）、一張主圖（token 每次不同）。
func (w *World) Input(title string, stock int32) catalog.ProductInput {
	w.mu.Lock()
	w.seq++
	token := fmt.Sprintf("handler-image-%d", w.seq)
	w.mu.Unlock()
	return catalog.ProductInput{Title: title, CategoryID: w.Leaf(), Skus: []catalog.SkuInput{{Price: 100, StockQty: stock}}, Gallery: []catalog.Image{{Token: token}}}
}

// Create 以模組建立商品（草稿）。
func (w *World) Create(storeID, accountID int64, in catalog.ProductInput) catalog.Product {
	w.T.Helper()
	p, err := w.Module.CreateProduct(context.Background(), storeID, media.Uploader{ID: accountID}, uuid.NewV7(), in, nil)
	if err != nil {
		w.T.Fatal(err)
	}
	return p
}

// SetStatus 切換狀態（讀目前版本再切）。
func (w *World) SetStatus(storeID int64, id uuid.UUID, to catalog.ProductStatus) {
	w.T.Helper()
	cur, err := w.Module.Product(context.Background(), storeID, id)
	if err != nil {
		w.T.Fatal(err)
	}
	if _, err := w.Module.SetProductStatus(context.Background(), storeID, id, cur.Version, to, nil); err != nil {
		w.T.Fatal(err)
	}
}

// Active 建立並上架商品。
func (w *World) Active(storeID, accountID int64, in catalog.ProductInput) catalog.Product {
	w.T.Helper()
	p := w.Create(storeID, accountID, in)
	w.SetStatus(storeID, p.ID, catalog.StatusActive)
	got, err := w.Module.Product(context.Background(), storeID, p.ID)
	if err != nil {
		w.T.Fatal(err)
	}
	return got
}
