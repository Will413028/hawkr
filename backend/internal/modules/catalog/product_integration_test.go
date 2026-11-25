//go:build integration

package catalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/hashtag"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

type productFixture struct {
	t     *testing.T
	d     *db.DB
	m     *Module
	media *media.Fake
	ct    context.Context
	mu    sync.Mutex
	now   time.Time
	seq   int64
}

func newProductFixture(t *testing.T) *productFixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: Migrations()},
	))
	f := &productFixture{t: t, d: d, ct: context.Background(), media: media.NewFake(), now: time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)}
	f.m = New(Deps{DB: d, NewID: uuid.NewV7, Now: f.clock, Media: f.media})
	// 測試只關心 catalog 讀 seller 的 pub_exposable_stores：把 KYC 的表放寬成可以直接寫最小的列（它自己的規則在 seller 的測試）。
	f.exec(`DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	return f
}

func (f *productFixture) clock() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *productFixture) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func (f *productFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.d.Pool().Exec(f.ct, sql, args...); err != nil {
		f.t.Fatalf("%v\n%s", err, sql)
	}
}

func (f *productFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.d.Pool().QueryRow(f.ct, sql, args...).Scan(&n); err != nil {
		f.t.Fatalf("%v\n%s", err, sql)
	}
	return n
}

func (f *productFixture) leaf() uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	if err := f.d.Pool().QueryRow(f.ct, `SELECT id FROM catalog.categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	return id
}

// store 建一家可以公開曝光的店（直接寫 seller 的表，最小的列），回傳 store_id 與帳號。
func (f *productFixture) store() (storeID, accountID int64) {
	f.t.Helper()
	f.seq++
	accountID = 1000 + f.seq
	f.exec(`INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $1, '測試店', 'logo/k', $3, '63000010', $2, $2)`, accountID, f.clock(), uuid.NewV7())
	if err := f.d.Pool().QueryRow(f.ct, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		f.t.Fatal(err)
	}
	f.exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, f.clock(), uuid.NewV7())
	f.exec(`INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, f.clock())
	return storeID, accountID
}

// setExposable 讓店家回到可曝光（true）或不可曝光（false，KYC 退回）。
func (f *productFixture) setExposable(storeID int64, ok bool) {
	f.t.Helper()
	status := "APPROVED"
	if !ok {
		status = "REJECTED"
	}
	f.exec("UPDATE seller.kyc_applications SET status = $2 WHERE store_id = $1", storeID, status)
}

// input 是一份可以上架的輸入：帶一張主圖（token 每次不同）。
func (f *productFixture) input(title string) ProductInput {
	f.mu.Lock()
	f.seq++
	token := fmt.Sprintf("input-image-%d", f.seq)
	f.mu.Unlock()
	return ProductInput{Title: title, CategoryID: f.leaf(), Skus: []SkuInput{{Price: 100}}, Gallery: []Image{{Token: token}}}
}

func (f *productFixture) create(storeID, accountID int64, in ProductInput) Product {
	f.t.Helper()
	p, err := f.m.CreateProduct(f.ct, storeID, media.Uploader{ID: accountID}, in)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *productFixture) activate(storeID int64, id uuid.UUID) {
	f.t.Helper()
	if err := f.m.SetProductStatus(f.ct, storeID, id, StatusActive); err != nil {
		f.t.Fatal(err)
	}
}

func (f *productFixture) ids(page ProductPage) []uuid.UUID {
	var out []uuid.UUID
	for _, it := range page.Items {
		out = append(out, it.ID)
	}
	return out
}

func colorSize() []Axis {
	return []Axis{{Name: "顏色", Values: []string{"紅", "藍"}}, {Name: "尺寸", Values: []string{"S", "M"}}}
}

func TestProductLifecycleAndFirstPublishedAt(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p := f.create(storeID, acct, f.input("蘋果"))
	if p.Status != StatusDraft || p.FirstPublishedAt != nil {
		t.Fatalf("a new product is a draft without first_published_at: %+v", p)
	}
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusArchived); err != nil {
		t.Fatalf("DRAFT to ARCHIVED is allowed: %v", err)
	}
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, ProductStatus("")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty status must not be valid: %v", err)
	}
	if got, _ := f.m.Product(f.ct, storeID, p.ID); got.FirstPublishedAt != nil {
		t.Fatalf("archiving a draft does not set first_published_at: %+v", got)
	}
	f.activate(storeID, p.ID)
	first := f.clock()
	got, _ := f.m.Product(f.ct, storeID, p.ID)
	if got.Status != StatusActive || got.FirstPublishedAt == nil || !got.FirstPublishedAt.Equal(first) {
		t.Fatalf("ACTIVE sets first_published_at: %+v", got)
	}
	f.advance(time.Hour)
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusDraft); err != nil {
		t.Fatalf("ACTIVE to DRAFT is allowed: %v", err)
	}
	got, _ = f.m.Product(f.ct, storeID, p.ID)
	if got.Status != StatusDraft || got.FirstPublishedAt == nil || !got.FirstPublishedAt.Equal(first) {
		t.Fatalf("going back to DRAFT keeps first_published_at: %+v (want %v)", got, first)
	}
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusArchived); err != nil {
		t.Fatalf("DRAFT to ARCHIVED: %v", err)
	}
	f.advance(time.Hour)
	f.activate(storeID, p.ID)
	got, _ = f.m.Product(f.ct, storeID, p.ID)
	if got.Status != StatusActive || !got.FirstPublishedAt.Equal(first) {
		t.Fatalf("re-activating keeps the first publish time: %+v (want %v)", got.FirstPublishedAt, first)
	}
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusActive); err != nil {
		t.Fatalf("same status is a no-op: %v", err)
	}
}

func TestActiveProductNeedsAGalleryImageOnEveryWrite(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	up := media.Uploader{ID: acct}
	bare := f.input("沒圖")
	bare.Gallery = nil
	p := f.create(storeID, acct, bare)
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusActive); !errors.Is(err, ErrActiveRequiresGalleryImage) {
		t.Fatalf("activating without a gallery image: %v", err)
	}
	if got, _ := f.m.Product(f.ct, storeID, p.ID); got.Status != StatusDraft || got.FirstPublishedAt != nil {
		t.Fatalf("a rejected activation changes nothing: %+v", got)
	}
	// 描述圖不算主圖。
	withDescImage := bare
	withDescImage.Description = []Block{{Kind: BlockImage, Image: &Image{Token: "only-description"}}}
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, withDescImage); err != nil {
		t.Fatal(err)
	}
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusActive); !errors.Is(err, ErrActiveRequiresGalleryImage) {
		t.Fatalf("a description image is not a gallery image: %v", err)
	}
	// 草稿與下架的商品可以存沒有主圖的內容；有主圖才能上架。
	withImage := f.input("有圖")
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, withImage); err != nil {
		t.Fatal(err)
	}
	f.activate(storeID, p.ID)
	// ACTIVE 的商品整體取代時不給主圖：拒絕，內容不變，也不自動降回草稿。
	bare.Title = "改名"
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, bare); !errors.Is(err, ErrActiveRequiresGalleryImage) {
		t.Fatalf("updating an ACTIVE product without a gallery image: %v", err)
	}
	got, _ := f.m.Product(f.ct, storeID, p.ID)
	if got.Status != StatusActive || got.Title != "有圖" || len(got.Gallery) != 1 {
		t.Fatalf("a rejected update changes nothing and does not demote: %+v", got)
	}
	// 下架後可以把主圖清掉，再上架時被擋。
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusArchived); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, bare); err != nil {
		t.Fatalf("an ARCHIVED product may be saved without a gallery image: %v", err)
	}
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusActive); !errors.Is(err, ErrActiveRequiresGalleryImage) {
		t.Fatalf("re-activating without a gallery image: %v", err)
	}
}

func TestDeleteKeepsTheRowAndIsIrreversible(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	other, _ := f.store()
	p := f.create(storeID, acct, f.input("x"))
	if err := f.m.DeleteProduct(f.ct, other, p.ID); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("another store must not delete it: %v", err)
	}
	if err := f.m.DeleteProduct(f.ct, storeID, p.ID); err != nil {
		t.Fatal(err)
	}
	if n := f.count(`SELECT count(*) FROM catalog.products WHERE id = $1 AND deleted_at IS NOT NULL`, p.ID); n != 1 {
		t.Fatalf("the row stays with deleted_at: %d", n)
	}
	if _, err := f.m.Product(f.ct, storeID, p.ID); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("read after delete: %v", err)
	}
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, StatusActive); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("a deleted product cannot come back: %v", err)
	}
	if _, err := f.m.UpdateProduct(f.ct, storeID, media.Uploader{ID: acct}, p.ID, f.input("y")); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("update after delete: %v", err)
	}
	if page, _ := f.m.ListSellerProducts(f.ct, storeID, nil, 10); len(page.Items) != 0 {
		t.Fatalf("deleted products leave the seller list: %+v", page.Items)
	}
}

func TestSkuCombinationsMustBeUniqueAndWithinOptionValues(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("T 恤")
	in.Axes = colorSize()
	in.Skus = []SkuInput{{Option1: "紅", Option2: "S", Price: 100}, {Option1: "藍", Option2: "M", Price: 200}}
	p := f.create(storeID, acct, in)
	if len(p.Axes) != 2 || len(p.Skus) != 2 || p.Axes[1].Values[1] != "M" {
		t.Fatalf("%+v", p)
	}

	bad := in
	bad.Skus = []SkuInput{{Option1: "綠", Option2: "S", Price: 1}}
	if _, err := f.m.CreateProduct(f.ct, storeID, media.Uploader{ID: acct}, bad); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a combination outside the option values: %v", err)
	}
	dup := in
	dup.Skus = []SkuInput{{Option1: "紅", Option2: "S", Price: 1}, {Option1: "紅", Option2: "S", Price: 2}}
	if _, err := f.m.UpdateProduct(f.ct, storeID, media.Uploader{ID: acct}, p.ID, dup); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a duplicate combination: %v", err)
	}
	// 沒有軸的商品只有一個預設 SKU。
	flat := f.input("單一規格")
	flat.Skus = []SkuInput{{Price: 1}, {Option1: "紅", Price: 1}}
	if _, err := f.m.CreateProduct(f.ct, storeID, media.Uploader{ID: acct}, flat); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a product without axes has exactly one default sku: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM catalog.products WHERE title IN ('單一規格')`); n != 0 {
		t.Fatalf("a rejected create leaves nothing behind: %d", n)
	}
	// 失敗的修改不改動商品。
	if got, _ := f.m.Product(f.ct, storeID, p.ID); len(got.Skus) != 2 {
		t.Fatalf("%+v", got.Skus)
	}
}

func TestPriceRangeFollowsSkuChanges(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	up := media.Uploader{ID: acct}
	in := f.input("T 恤")
	in.Axes = colorSize()
	in.Skus = []SkuInput{{Option1: "紅", Option2: "S", Price: 100, StockQty: 5}, {Option1: "藍", Option2: "M", Price: 200}}
	p := f.create(storeID, acct, in)
	if p.MinPrice != 100 || p.MaxPrice != 200 {
		t.Fatalf("create: %d..%d", p.MinPrice, p.MaxPrice)
	}

	f.exec(`UPDATE catalog.product_skus SET stock_qty = 7 WHERE product_id = $1 AND option1_value = '紅'`, p.ID)
	in.Skus = []SkuInput{{Option1: "紅", Option2: "S", Price: 50, StockQty: 99}, {Option1: "藍", Option2: "M", Price: 300}}
	p, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if p.MinPrice != 50 || p.MaxPrice != 300 {
		t.Fatalf("update prices: %d..%d", p.MinPrice, p.MaxPrice)
	}
	for _, s := range p.Skus {
		if s.Option1 == "紅" && s.StockQty != 7 {
			t.Fatalf("an existing combination keeps its stock (only stock operations change it): %d", s.StockQty)
		}
	}

	in.Skus = []SkuInput{{Option1: "藍", Option2: "M", Price: 300}, {Option1: "藍", Option2: "S", Price: 120, StockQty: 4}}
	p, err = f.m.UpdateProduct(f.ct, storeID, up, p.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if p.MinPrice != 120 || p.MaxPrice != 300 || len(p.Skus) != 2 {
		t.Fatalf("removed and added combinations: %d..%d %+v", p.MinPrice, p.MaxPrice, p.Skus)
	}
	if n := f.count(`SELECT count(*) FROM catalog.product_skus WHERE product_id = $1 AND deleted_at IS NOT NULL`, p.ID); n != 1 {
		t.Fatalf("a removed combination is soft deleted: %d", n)
	}
	var lo, hi int64
	if err := f.d.Pool().QueryRow(f.ct, `SELECT min_price, max_price FROM catalog.products WHERE id = $1`, p.ID).Scan(&lo, &hi); err != nil || lo != 120 || hi != 300 {
		t.Fatalf("stored range %d..%d %v", lo, hi, err)
	}
}

func TestImagesConfirmedInTheWriteAndMarkedDeletedWhenRemoved(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	up := media.Uploader{ID: acct}
	in := f.input("圖")
	in.Gallery = []Image{{Token: "g1", Alt: "正面"}, {Token: "g2"}}
	in.Description = []Block{
		{Kind: BlockParagraph, Text: "說明"},
		{Kind: BlockImage, Image: &Image{Token: "d1", Alt: "細節"}},
	}
	p := f.create(storeID, acct, in)
	g1, g2, d1 := media.AssetFor("g1"), media.AssetFor("g2"), media.AssetFor("d1")
	if len(p.Gallery) != 2 || p.Gallery[0].AssetID != g1 || p.Description[1].Image.AssetID != d1 || p.Description[1].Image.Alt != "細節" {
		t.Fatalf("%+v", p)
	}
	if !f.media.IsConfirmed(g1) || !f.media.IsConfirmed(d1) {
		t.Fatal("new images are confirmed")
	}
	if n := f.count(`SELECT count(*) FROM catalog.product_images WHERE product_id = $1`, p.ID); n != 3 {
		t.Fatalf("image references: %d", n)
	}

	// 保留 g1 與描述圖（以資產 ID），移除 g2。
	in.Gallery = []Image{{AssetID: g1, Alt: "正面"}}
	in.Description = []Block{{Kind: BlockImage, Image: &Image{AssetID: d1, Alt: "細節"}}}
	p, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if !f.media.IsDeleted(g2) {
		t.Fatal("a removed image is marked deleted")
	}
	if f.media.IsDeleted(g1) || f.media.IsDeleted(d1) {
		t.Fatal("kept images are not marked deleted")
	}
	if len(p.Gallery) != 1 || p.Gallery[0].ObjectKey == "" {
		t.Fatalf("%+v", p.Gallery)
	}

	// 不是這個商品的資產不能引用。
	in.Gallery = []Image{{AssetID: uuid.NewV7()}}
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, in); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a foreign asset: %v", err)
	}
	// 移除全部圖片。
	in.Gallery, in.Description = nil, nil
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, in); err != nil {
		t.Fatal(err)
	}
	if !f.media.IsDeleted(g1) || !f.media.IsDeleted(d1) {
		t.Fatal("all removed images are marked deleted")
	}
}

func TestMediaFailuresRollBackTheProductWrite(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	up := media.Uploader{ID: acct}
	in := f.input("回滾")
	in.Gallery = []Image{{Token: "g1"}}

	f.media.FailConfirm = errors.New("confirm down")
	if _, err := f.m.CreateProduct(f.ct, storeID, up, in); err == nil {
		t.Fatal("a Confirm error fails the create")
	}
	if n := f.count(`SELECT count(*) FROM catalog.products WHERE title = '回滾'`); n != 0 {
		t.Fatalf("the product write rolled back with the Confirm error: %d", n)
	}
	f.media.FailConfirm = nil

	p := f.create(storeID, acct, in)
	in.Title = "新名稱"
	in.Gallery = nil
	f.media.FailMarkDeleted = errors.New("mark down")
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, in); err == nil {
		t.Fatal("a MarkDeleted error fails the update")
	}
	got, _ := f.m.Product(f.ct, storeID, p.ID)
	if got.Title != "回滾" || len(got.Gallery) != 1 {
		t.Fatalf("the update rolled back with the MarkDeleted error: %+v", got)
	}
}

func TestHashtagsUseThePlatformNormalization(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("標籤")
	in.Hashtags = []string{"  #Fruit ", "FRUIT", "有機", "Org"}
	p := f.create(storeID, acct, in)
	wantTags, err := hashtag.RequireSet(in.Hashtags)
	if err != nil {
		t.Fatal(err)
	}
	var got []hashtag.Tag
	for _, h := range p.Hashtags {
		got = append(got, hashtag.Tag{Name: h.Name, Display: h.Display})
	}
	if !slices.Equal(got, wantTags) || got[0].Name != "fruit" || got[0].Display != "Fruit" {
		t.Fatalf("got %+v want %+v", got, wantTags)
	}
	var names []string
	rows, err := f.d.Pool().Query(f.ct, `SELECT name FROM catalog.product_hashtags WHERE product_id = $1 ORDER BY position`, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	if !slices.Equal(names, []string{"fruit", "有機", "org"}) {
		t.Fatalf("%v", names)
	}
	in.Hashtags = []string{"a b"}
	if _, err := f.m.UpdateProduct(f.ct, storeID, media.Uploader{ID: acct}, p.ID, in); !errors.Is(err, ErrInvalidInput) || !errors.Is(err, hashtag.ErrInvalidChars) {
		t.Fatalf("an invalid tag: %v", err)
	}
}

func TestSearchableTextAssembly(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("山藥")
	in.Description = []Block{
		{Kind: BlockParagraph, Text: "產地直送"},
		{Kind: BlockList, Items: []string{"無農藥"}},
		{Kind: BlockTable, Rows: [][]string{{"重量", "600g"}}},
	}
	in.Hashtags = []string{"#Yam"}
	p := f.create(storeID, acct, in)
	var text, catName string
	if err := f.d.Pool().QueryRow(f.ct, `SELECT p.searchable_text, COALESCE(c.name ->> 'en', c.name ->> 'zh-TW') FROM catalog.products p JOIN catalog.categories c ON c.id = p.category_id WHERE p.id = $1`, p.ID).Scan(&text, &catName); err != nil {
		t.Fatal(err)
	}
	want := "山藥 產地直送 無農藥 重量 600g yam " + catName
	if text != want {
		t.Fatalf("searchable_text = %q, want %q", text, want)
	}
}

func TestPublicListOnlyActiveProductsOfExposableStores(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	okStore, okAcct := f.store()
	badStore, badAcct := f.store()

	visible := f.create(okStore, okAcct, f.input("可見"))
	f.activate(okStore, visible.ID)
	draft := f.create(okStore, okAcct, f.input("草稿"))
	archived := f.create(okStore, okAcct, f.input("下架"))
	f.activate(okStore, archived.ID)
	if err := f.m.SetProductStatus(f.ct, okStore, archived.ID, StatusArchived); err != nil {
		t.Fatal(err)
	}
	deleted := f.create(okStore, okAcct, f.input("已刪除"))
	f.activate(okStore, deleted.ID)
	if err := f.m.DeleteProduct(f.ct, okStore, deleted.ID); err != nil {
		t.Fatal(err)
	}
	suspended := f.create(badStore, badAcct, f.input("停權店商品"))
	f.activate(badStore, suspended.ID)
	f.setExposable(badStore, false)

	page, err := f.m.ListPublicProducts(f.ct, f.clock(), nil, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.ids(page); !slices.Equal(got, []uuid.UUID{visible.ID}) {
		t.Fatalf("public list must hold only ACTIVE products of exposable stores, got %v (visible %v)", got, visible.ID)
	}
	if _, err := f.m.PublicProduct(f.ct, suspended.ID, f.clock()); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("a suspended store's product is not publicly readable: %v", err)
	}
	if _, err := f.m.PublicProduct(f.ct, draft.ID, f.clock()); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("a draft is not publicly readable: %v", err)
	}
	if got, err := f.m.PublicProduct(f.ct, visible.ID, f.clock()); err != nil || got.ID != visible.ID {
		t.Fatalf("%v", err)
	}

	f.setExposable(badStore, true)
	page, _ = f.m.ListPublicProducts(f.ct, f.clock(), nil, nil, 50)
	if len(page.Items) != 2 {
		t.Fatalf("an exposable store's active product appears again: %v", f.ids(page))
	}
	cat := f.leaf()
	page, _ = f.m.ListPublicProducts(f.ct, f.clock(), &cat, nil, 50)
	if len(page.Items) != 2 {
		t.Fatalf("category filter: %v", f.ids(page))
	}
	other := uuid.NewV7()
	if page, _ = f.m.ListPublicProducts(f.ct, f.clock(), &other, nil, 50); len(page.Items) != 0 {
		t.Fatalf("another category: %v", f.ids(page))
	}
}

func TestSellerListIsUnfilteredByStatusAndExposure(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	other, otherAcct := f.store()
	a := f.create(storeID, acct, f.input("草稿"))
	b := f.create(storeID, acct, f.input("上架"))
	f.activate(storeID, b.ID)
	c := f.create(storeID, acct, f.input("下架"))
	f.activate(storeID, c.ID)
	if err := f.m.SetProductStatus(f.ct, storeID, c.ID, StatusArchived); err != nil {
		t.Fatal(err)
	}
	gone := f.create(storeID, acct, f.input("刪除"))
	if err := f.m.DeleteProduct(f.ct, storeID, gone.ID); err != nil {
		t.Fatal(err)
	}
	foreign := f.create(other, otherAcct, f.input("別家"))
	f.setExposable(storeID, false)

	page, err := f.m.ListSellerProducts(f.ct, storeID, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	got := f.ids(page)
	if len(got) != 3 || !slices.Contains(got, a.ID) || !slices.Contains(got, b.ID) || !slices.Contains(got, c.ID) || slices.Contains(got, foreign.ID) {
		t.Fatalf("the seller sees all of their non-deleted products, whatever the status or exposure: %v", got)
	}
	if _, err := f.m.Product(f.ct, storeID, foreign.ID); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("another store's product: %v", err)
	}
}

func TestListPaginationWalksEveryProductOnce(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	var all []uuid.UUID
	for i := range 5 {
		p := f.create(storeID, acct, f.input("p"+string(rune('a'+i))))
		f.activate(storeID, p.ID)
		all = append(all, p.ID)
		f.advance(time.Minute)
	}
	walk := func(list func(after *ListCursor) (ProductPage, error)) []uuid.UUID {
		var seen []uuid.UUID
		var after *ListCursor
		for range 10 {
			page, err := list(after)
			if err != nil {
				t.Fatal(err)
			}
			seen = append(seen, f.ids(page)...)
			if page.Next == nil {
				return seen
			}
			after = page.Next
		}
		t.Fatal("pagination did not end")
		return nil
	}
	pub := walk(func(after *ListCursor) (ProductPage, error) {
		return f.m.ListPublicProducts(f.ct, f.clock(), nil, after, 2)
	})
	own := walk(func(after *ListCursor) (ProductPage, error) { return f.m.ListSellerProducts(f.ct, storeID, after, 2) })
	for name, seen := range map[string][]uuid.UUID{"public": pub, "seller": own} {
		if len(seen) != len(all) {
			t.Fatalf("%s: %d of %d", name, len(seen), len(all))
		}
		for _, id := range all {
			if !slices.Contains(seen, id) {
				t.Fatalf("%s misses %v", name, id)
			}
		}
		if seen[0] != all[len(all)-1] {
			t.Fatalf("%s: newest first", name)
		}
	}
}

func TestOnlyLeafCategoriesAcceptedOnWrite(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	var top uuid.UUID
	if err := f.d.Pool().QueryRow(f.ct, `SELECT id FROM catalog.categories WHERE parent_id IS NULL LIMIT 1`).Scan(&top); err != nil {
		t.Fatal(err)
	}
	in := f.input("分類")
	in.CategoryID = top
	if _, err := f.m.CreateProduct(f.ct, storeID, media.Uploader{ID: acct}, in); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a top-level group takes no products: %v", err)
	}
}

// waitBlockedBy 等到有別的連線被 pid 這個 backend 持有的鎖擋住（pg_blocking_pids，不看 SQL 文字）；條件成立才往下，不假設時間。
// done 先有結果代表那個呼叫沒有等鎖就結束了，直接讓測試失敗並帶出它的錯誤。
func (f *productFixture) waitBlockedBy(pid int, done <-chan error) {
	f.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			f.t.Fatalf("the call finished (err=%v) without waiting for the lock held by backend %d", err, pid)
		default:
		}
		if f.count(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND $1::int = ANY(pg_blocking_pids(pid))`, pid) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.t.Fatalf("no session blocked on a lock held by backend %d", pid)
}

// lockedNow 回報這一刻 query（必須是 FOR UPDATE NOWAIT 的 SELECT）要鎖的列是不是已經被別的 transaction 持有。
func (f *productFixture) lockedNow(query string, args ...any) bool {
	f.t.Helper()
	tx, err := f.d.Pool().Begin(f.ct)
	if err != nil {
		f.t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(f.ct) }()
	_, err = tx.Exec(f.ct, query, args...)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return true
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return false
}

// Update 以反過來的 SKU 順序送：它對既有 SKU 列的鎖仍依 ID 升序（與 DeductStock 同序），所以卡在最小 ID 的列時，還沒鎖較大的列。
// 以固定的交錯斷言順序：外部先持有最小 ID 的列，Update 卡住後較大的列必須還沒被它鎖住。
func TestUpdateLocksExistingSkusInAscendingIdOrderWhateverTheInputOrder(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("鎖順序")
	in.Axes = colorSize()
	in.Skus = []SkuInput{{Option1: "紅", Option2: "S", Price: 100}, {Option1: "藍", Option2: "M", Price: 200}}
	p := f.create(storeID, acct, in)
	lo, hi := p.Skus[0], p.Skus[1]
	if lo.ID.String() > hi.ID.String() {
		lo, hi = hi, lo
	}
	holder, err := f.d.Pool().Begin(f.ct)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(f.ct) }()
	if _, err := holder.Exec(f.ct, `SELECT id FROM catalog.product_skus WHERE id = $1 FOR UPDATE`, lo.ID); err != nil {
		t.Fatal(err)
	}
	var holderPID int
	if err := holder.QueryRow(f.ct, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	reversed := f.input("鎖順序")
	reversed.Axes = colorSize()
	byID := map[uuid.UUID]Sku{lo.ID: lo, hi.ID: hi}
	for _, id := range []uuid.UUID{hi.ID, lo.ID} {
		k := byID[id]
		reversed.Skus = append(reversed.Skus, SkuInput{Option1: k.Option1, Option2: k.Option2, Price: k.Price + 1})
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.m.UpdateProduct(f.ct, storeID, media.Uploader{ID: acct}, p.ID, reversed)
		done <- err
	}()
	f.waitBlockedBy(holderPID, done)
	if f.lockedNow(`SELECT id FROM catalog.product_skus WHERE id = $1 FOR UPDATE NOWAIT`, hi.ID) {
		t.Error("Update locked the larger sku id before the smaller one")
	}
	if err := holder.Rollback(f.ct); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got, err := f.m.Product(f.ct, storeID, p.ID)
	if err != nil || len(got.Skus) != 2 {
		t.Fatalf("%v %+v", err, got.Skus)
	}
}

// 並行的 Update（反向輸入）與多品項 DeductStock（升序鎖）不死結，兩邊都成功。
func TestUpdateAndMultiSkuDeductDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("並行")
	in.Axes = colorSize()
	in.Skus = []SkuInput{{Option1: "紅", Option2: "S", Price: 100, StockQty: 1000}, {Option1: "藍", Option2: "M", Price: 200, StockQty: 1000}}
	p := f.create(storeID, acct, in)
	a, b := p.Skus[0], p.Skus[1]
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			up := f.input("並行")
			up.Axes = colorSize()
			up.Skus = []SkuInput{{Option1: b.Option1, Option2: b.Option2, Price: int64(300 + i)}, {Option1: a.Option1, Option2: a.Option2, Price: int64(100 + i)}}
			if i%2 == 1 {
				up.Skus[0], up.Skus[1] = up.Skus[1], up.Skus[0]
			}
			if _, err := f.m.UpdateProduct(f.ct, storeID, media.Uploader{ID: acct}, p.ID, up); err != nil {
				t.Errorf("update: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := f.m.DeductStock(f.ct, fmt.Sprintf("d%d", i), items(b.ID, 1, a.ID, 1)); err != nil {
				t.Errorf("deduct: %v", err)
			}
		}()
	}
	wg.Wait()
}

// 描述的圖片區塊指向 product_images：商品列與圖片在同一份快照讀出。圖片被並行的修改在兩個語句之間清掉時，讀取不能得到
// 「描述還有圖片、圖片表已經沒有」的錯誤，只能是修改前或修改後的完整內容。
func TestReadIsOneSnapshotWhileAWriteCommitsBetweenItsParts(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("快照")
	in.Description = []Block{{Kind: BlockParagraph, Text: "說明"}, {Kind: BlockImage, Image: &Image{Token: "snap-d1"}}}
	p := f.create(storeID, acct, in)
	// 第二個連線池的 tracer 在 GetOwnProduct 這句讀完之後把讀取停住：單句實作到此已經讀完全部內容，
	// 拆成兩句的實作還沒讀圖片。停住的期間提交修改，再放行。
	gate := &readGate{reached: make(chan struct{}), release: make(chan struct{})}
	defer gate.open()
	d2, err := db.Open(f.ct, db.Config{URL: f.d.Pool().Config().ConnConfig.ConnString(), MaxConns: 2, ConnectRetryBudget: 10 * time.Second, Tracer: gate})
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	reader := New(Deps{DB: d2, NewID: uuid.NewV7, Now: f.clock, Media: f.media})
	type result struct {
		p   Product
		err error
	}
	done := make(chan result, 1)
	go func() {
		got, err := reader.Product(f.ct, storeID, p.ID)
		done <- result{got, err}
	}()
	select {
	case <-gate.reached:
	case r := <-done:
		t.Fatalf("the read finished without passing the gate: %v", r.err)
	case <-time.After(30 * time.Second):
		t.Fatal("the read never reached the gate")
	}
	w, err := f.d.Pool().Begin(f.ct)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Rollback(f.ct) }()
	if _, err := w.Exec(f.ct, `DELETE FROM catalog.product_images WHERE product_id = $1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Exec(f.ct, `UPDATE catalog.products SET description = '[]'::jsonb WHERE id = $1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.Commit(f.ct); err != nil {
		t.Fatal(err)
	}
	gate.open()
	r := <-done
	if r.err != nil {
		t.Fatalf("a read overlapping a write must see one version: %v", r.err)
	}
	if len(r.p.Description) != 2 || len(r.p.Gallery) != 1 {
		t.Fatalf("the read began before the write committed, want the whole old version: description=%d gallery=%d", len(r.p.Description), len(r.p.Gallery))
	}
}

// readGate 在 GetOwnProduct 讀完時停住該連線，直到 open。
type readGate struct {
	reached chan struct{}
	release chan struct{}
	once    sync.Once
	opened  sync.Once
}

type gateKey struct{}

func (g *readGate) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "name: GetOwnProduct") {
		return context.WithValue(ctx, gateKey{}, true)
	}
	return ctx
}

func (g *readGate) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if ctx.Value(gateKey{}) == nil {
		return
	}
	g.once.Do(func() { close(g.reached) })
	<-g.release
}

func (g *readGate) open() { g.opened.Do(func() { close(g.release) }) }

// 刪除商品時它的 SKU 一併刪除：扣減找不到；刪除之前的扣減仍能歸還（歸還不看刪除）。
func TestDeleteProductMakesItsSkusUndeductibleButRestorable(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("刪除庫存")
	in.Skus = []SkuInput{{Price: 100, StockQty: 10}}
	p := f.create(storeID, acct, in)
	sku := p.Skus[0].ID
	if err := f.m.DeductStock(f.ct, "before-delete", items(sku, 3)); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteProduct(f.ct, storeID, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeductStock(f.ct, "after-delete", items(sku, 1)); !errors.Is(err, ErrSkuNotFound) {
		t.Fatalf("deduct after the product was deleted: %v", err)
	}
	if err := f.m.AdjustStock(f.ct, storeID, "adj-after-delete", sku, 1); !errors.Is(err, ErrSkuNotFound) {
		t.Fatalf("adjust after the product was deleted: %v", err)
	}
	if err := f.m.RestoreStock(f.ct, "restore", "before-delete", nil); err != nil {
		t.Fatalf("restoring an earlier deduction still works: %v", err)
	}
	var qty int
	if err := f.d.Pool().QueryRow(f.ct, `SELECT stock_qty FROM catalog.product_skus WHERE id = $1`, sku).Scan(&qty); err != nil || qty != 10 {
		t.Fatalf("qty=%d err=%v", qty, err)
	}
}

// confirmProbe 在 Confirm 被呼叫的那一刻檢查商品列是否已被這個寫入鎖住。
type confirmProbe struct {
	*media.Fake
	f         *productFixture
	productID uuid.UUID
	calls     int
	lockedAt  int
}

func (c *confirmProbe) Confirm(ctx context.Context, u media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error) {
	c.calls++
	if c.f.lockedNow(`SELECT id FROM catalog.products WHERE id = $1 FOR UPDATE NOWAIT`, c.productID) {
		c.lockedAt++
	}
	return c.Fake.Confirm(ctx, u, token, want)
}

// 上層先呼叫下層的 API 再鎖自己的列（platform/lockorder）：Update 在鎖商品列之前就 Confirm 新圖。
func TestUpdateConfirmsNewImagesBeforeLockingTheProductRow(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p := f.create(storeID, acct, f.input("確認順序"))
	probe := &confirmProbe{Fake: f.media, f: f, productID: p.ID}
	m := New(Deps{DB: f.d, NewID: uuid.NewV7, Now: f.clock, Media: probe})
	in := f.input("確認順序")
	in.Gallery = []Image{{Token: "probe-new"}}
	if _, err := m.UpdateProduct(f.ct, storeID, media.Uploader{ID: acct}, p.ID, in); err != nil {
		t.Fatal(err)
	}
	if probe.calls != 1 || probe.lockedAt != 0 {
		t.Fatalf("confirm calls=%d, of which with the product row already locked=%d", probe.calls, probe.lockedAt)
	}
}
