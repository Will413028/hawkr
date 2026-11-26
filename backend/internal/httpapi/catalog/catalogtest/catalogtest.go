// Package catalogtest 是 catalog Connect handler 與買家「追蹤中」動態的 integration 測試共用的世界：真的 catalog 模組與資料庫，
// 以及最小的 seller 資料（直接寫表），讓「店家可不可曝光」走真實的 seller.pub_exposable_stores。只給測試用。
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
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

// FirstStoreID 是第一家店的 store_id：與帳號 ID（從 1001 起）刻意拉開，誤把帳號 ID 當店家 ID 的實作會讀到別人的資料或找不到店。
const FirstStoreID = 7000

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
	mu     sync.Mutex
	now    time.Time
	seq    int64
}

// New 建立世界：套用 media、buyer、seller、catalog 的 migration（catalog 讀 seller 的 pub_exposable_stores）。
func New(t *testing.T, pg *testdb.Server) *World {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: catalog.Migrations()},
	))
	w := &World{T: t, DB: d, Media: media.NewFake(), now: time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)}
	w.Module = catalog.New(catalog.Deps{DB: d, NewID: uuid.NewV7, Now: w.Now, Media: w.Media, Restock: Notifier{}})
	// 只關心 catalog 讀 seller 的 pub_exposable_stores：KYC 的表放寬成可以直接寫最小的列（它自己的規則在 seller 的測試）。
	w.Exec(`DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	w.Exec(fmt.Sprintf(`ALTER TABLE seller.stores ALTER COLUMN store_id RESTART WITH %d`, FirstStoreID))
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
	w.mu.Lock()
	w.seq++
	accountID = 1000 + w.seq
	w.mu.Unlock()
	w.Exec(`INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $1, '測試店', 'logo/k', $3, '63000010', $2, $2)`, accountID, w.Now(), uuid.NewV7())
	if err := w.DB.Pool().QueryRow(context.Background(), "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		w.T.Fatal(err)
	}
	w.Exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, w.Now(), uuid.NewV7())
	w.Exec(`INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, w.Now())
	return storeID, accountID
}

// SetExposable 讓店家回到可曝光（true）或不可曝光（false，KYC 退回）。
func (w *World) SetExposable(storeID int64, ok bool) {
	w.T.Helper()
	status := "APPROVED"
	if !ok {
		status = "REJECTED"
	}
	w.Exec("UPDATE seller.kyc_applications SET status = $2 WHERE store_id = $1", storeID, status)
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
	p, err := w.Module.CreateProduct(context.Background(), storeID, media.Uploader{ID: accountID}, uuid.NewV7(), in)
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
	if err := w.Module.SetProductStatus(context.Background(), storeID, id, cur.Version, to); err != nil {
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
