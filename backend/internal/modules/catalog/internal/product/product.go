// Package product 是商品的寫入與讀取：建立、整體修改、狀態切換、刪除，選項軸與 SKU 組合，價格區間，描述區塊與圖片引用，
// hashtag，searchable_text，以及賣家清單與公開列表。
//   - 同一個商品的寫入從 LockOwnProduct 的列鎖開始，SKU、選項、圖片與價格區間的重算在同一個 transaction，所以並行的寫入不會交錯。
//   - 新圖在寫入的 transaction 內 Confirm（ctx 帶 transaction，失敗整筆回滾）；退場的資產在同一個 transaction 內 MarkDeleted。
//   - 店家身分由呼叫端推導後傳入：這裡的每個寫入與賣家讀取都以 storeID 限定，不接受 client 送的店家。
package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/description"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	kindGallery     = "GALLERY"
	kindDescription = "DESCRIPTION"
)

// Deps 是商品的依賴。
type Deps struct {
	DB    *db.DB
	NewID func() uuid.UUID
	Now   func() time.Time
	Media model.Media
}

// Service 是商品的 use case。
type Service struct{ d Deps }

// New 建立商品 use case。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// Create 建立一個草稿商品。uploader 是新圖 staged token 的上傳者（店家所屬的帳號，由呼叫端推導）。
func (s *Service) Create(ctx context.Context, storeID int64, uploader media.Uploader, in model.ProductInput) (model.Product, error) {
	if storeID <= 0 {
		return model.Product{}, model.Invalid("StoreID", model.ReasonRequired)
	}
	p, err := prepare(in)
	if err != nil {
		return model.Product{}, err
	}
	var out model.Product
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		id, now := s.d.NewID(), s.d.Now()
		catName, err := s.leafCategoryText(ctx, q, p.categoryID)
		if err != nil {
			return err
		}
		// product_images.product_id 的外鍵不可延後：先插入商品列（描述先放空陣列），圖片寫入取得資產 id 後再以 UpdateProductContent 補上
		// 描述 JSON 與 searchable_text。價格區間先放 0，下面的 SKU 寫入之後由 RecomputePriceRange 算；建立一律是草稿，不需要 first_published_at。
		if err := q.InsertProduct(ctx, store.InsertProductParams{
			ID: id, StoreID: storeID, CategoryID: p.categoryID, Title: p.title, Description: []byte("[]"), Status: string(model.StatusDraft), Now: now,
		}); err != nil {
			return err
		}
		if err := s.writeImages(ctx, q, id, uploader, nil, &p); err != nil {
			return err
		}
		descJSON, err := description.Encode(p.desc)
		if err != nil {
			return err
		}
		if err := q.UpdateProductContent(ctx, store.UpdateProductContentParams{
			ID: id, CategoryID: p.categoryID, Title: p.title, Description: descJSON, SearchableText: searchableText(p, catName), Now: now,
		}); err != nil {
			return err
		}
		if err := s.writeOptions(ctx, q, id, p.axes); err != nil {
			return err
		}
		if err := s.writeSkus(ctx, q, id, nil, p.skus); err != nil {
			return err
		}
		if err := s.writeTags(ctx, q, id, p); err != nil {
			return err
		}
		out, err = s.load(ctx, q, id, storeID, nil)
		return err
	})
	return out, err
}

// Update 整體取代商品的內容（狀態不變）。選項軸、hashtag 與圖片整組取代；SKU 以選項值的組合辨識：組合還在就更新價格（庫存不動），
// 新組合新增，不在輸入裡的組合刪除（deleted_at）。
func (s *Service) Update(ctx context.Context, storeID int64, uploader media.Uploader, productID uuid.UUID, in model.ProductInput) (model.Product, error) {
	p, err := prepare(in)
	if err != nil {
		return model.Product{}, err
	}
	var out model.Product
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		cur, err := s.lock(ctx, q, storeID, productID)
		if err != nil {
			return err
		}
		if model.ProductStatus(cur.Status) == model.StatusActive && len(p.gallery) == 0 {
			return model.ErrActiveRequiresGalleryImage
		}
		now := s.d.Now()
		catName, err := s.leafCategoryText(ctx, q, p.categoryID)
		if err != nil {
			return err
		}
		existing, err := q.ListProductImages(ctx, productID)
		if err != nil {
			return err
		}
		if err := s.writeImages(ctx, q, productID, uploader, existing, &p); err != nil {
			return err
		}
		descJSON, err := description.Encode(p.desc)
		if err != nil {
			return err
		}
		if err := q.UpdateProductContent(ctx, store.UpdateProductContentParams{
			ID: productID, CategoryID: p.categoryID, Title: p.title, Description: descJSON, SearchableText: searchableText(p, catName), Now: now,
		}); err != nil {
			return err
		}
		if err := q.DeleteOptionValues(ctx, productID); err != nil {
			return err
		}
		if err := q.DeleteOptionAxes(ctx, productID); err != nil {
			return err
		}
		if err := s.writeOptions(ctx, q, productID, p.axes); err != nil {
			return err
		}
		live, err := q.ListLiveSkus(ctx, productID)
		if err != nil {
			return err
		}
		if err := s.writeSkus(ctx, q, productID, live, p.skus); err != nil {
			return err
		}
		if err := q.DeleteProductHashtags(ctx, productID); err != nil {
			return err
		}
		if err := s.writeTags(ctx, q, productID, p); err != nil {
			return err
		}
		out, err = s.load(ctx, q, productID, storeID, nil)
		return err
	})
	return out, err
}

// SetStatus 切換狀態：三個狀態任兩個都能互切。設成 ACTIVE 時 first_published_at 只在還是空的時候填，切回其他狀態不清除。
// 切成目前的狀態是 no-op；切成 ACTIVE 而商品沒有主圖回 model.ErrActiveRequiresGalleryImage。
func (s *Service) SetStatus(ctx context.Context, storeID int64, productID uuid.UUID, to model.ProductStatus) error {
	if !to.Valid() {
		return model.Invalid("Status", model.ReasonUnknown)
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		cur, err := s.lock(ctx, q, storeID, productID)
		if err != nil {
			return err
		}
		from := model.ProductStatus(cur.Status)
		if from == to {
			return nil
		}
		if !from.CanTransition(to) {
			return model.ErrInvalidTransition
		}
		if to == model.StatusActive {
			imgs, err := q.ListProductImages(ctx, productID)
			if err != nil {
				return err
			}
			if !slices.ContainsFunc(imgs, func(i store.CatalogProductImage) bool { return i.Kind == kindGallery }) {
				return model.ErrActiveRequiresGalleryImage
			}
		}
		return q.SetProductStatus(ctx, store.SetProductStatusParams{ID: productID, Status: string(to), Now: s.d.Now()})
	})
}

// Delete 刪除商品：設 deleted_at，不可還原，列與圖片引用保留（訂單與評價仍引用）。
func (s *Service) Delete(ctx context.Context, storeID int64, productID uuid.UUID) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if _, err := s.lock(ctx, q, storeID, productID); err != nil {
			return err
		}
		now := s.d.Now()
		return q.SoftDeleteProduct(ctx, store.SoftDeleteProductParams{ID: productID, Now: &now})
	})
}

func (s *Service) lock(ctx context.Context, q *store.Queries, storeID int64, productID uuid.UUID) (store.LockOwnProductRow, error) {
	row, err := q.LockOwnProduct(ctx, store.LockOwnProductParams{ID: productID, StoreID: storeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, model.ErrProductNotFound
	}
	return row, err
}

// leafCategoryText 確認分類是葉節點，回傳 searchable_text 要用的分類名稱（en，缺時 zh-TW）。
func (s *Service) leafCategoryText(ctx context.Context, q *store.Queries, id uuid.UUID) (string, error) {
	leaf, err := q.IsLeafCategory(ctx, id)
	if err != nil {
		return "", err
	}
	if !leaf {
		return "", model.Invalid("CategoryID", model.ReasonInvalid)
	}
	c, err := q.GetCategory(ctx, id)
	if err != nil {
		return "", err
	}
	var name map[string]string
	if err := json.Unmarshal(c.Name, &name); err != nil {
		return "", fmt.Errorf("catalog: decode category name: %w", err)
	}
	if name["en"] != "" {
		return name["en"], nil
	}
	return name["zh-TW"], nil
}

// searchableText 組 searchable_text：名稱、描述文字、hashtag 名稱、分類名稱，以單一空格相接（schema.sql 的註解）。
func searchableText(p prepared, categoryName string) string {
	parts := []string{p.title, description.Text(p.desc)}
	for _, t := range p.tags {
		parts = append(parts, t.Name)
	}
	parts = append(parts, categoryName)
	return strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), " ")
}

func (s *Service) writeOptions(ctx context.Context, q *store.Queries, id uuid.UUID, axes []model.Axis) error {
	for i, a := range axes {
		pos := int32(i + 1)
		if err := q.InsertOptionAxis(ctx, store.InsertOptionAxisParams{ProductID: id, Position: pos, Name: a.Name}); err != nil {
			return err
		}
		for j, v := range a.Values {
			if err := q.InsertOptionValue(ctx, store.InsertOptionValueParams{ProductID: id, AxisPosition: pos, Position: int32(j), Value: v}); err != nil {
				return err
			}
		}
	}
	return nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// writeSkus 把輸入的 SKU 對上現有的（以組合辨識）：還在的更新價格，新的新增，不在輸入裡的標刪除，最後重算價格區間。
func (s *Service) writeSkus(ctx context.Context, q *store.Queries, id uuid.UUID, live []store.ListLiveSkusRow, in []model.SkuInput) error {
	now := s.d.Now()
	byCombo := make(map[[2]string]store.ListLiveSkusRow, len(live))
	for _, l := range live {
		byCombo[[2]string{deref(l.Option1Value), deref(l.Option2Value)}] = l
	}
	for _, sk := range in {
		key := [2]string{sk.Option1, sk.Option2}
		if cur, ok := byCombo[key]; ok {
			delete(byCombo, key)
			if err := q.UpdateSkuPrices(ctx, store.UpdateSkuPricesParams{ID: cur.ID, Price: sk.Price, CompareAtPrice: sk.CompareAtPrice, Now: now}); err != nil {
				return err
			}
			continue
		}
		if err := q.InsertSku(ctx, store.InsertSkuParams{
			ID: s.d.NewID(), ProductID: id, Option1Value: optional(sk.Option1), Option2Value: optional(sk.Option2),
			Price: sk.Price, CompareAtPrice: sk.CompareAtPrice, StockQty: sk.StockQty, Now: now,
		}); err != nil {
			return err
		}
	}
	for _, gone := range byCombo {
		if err := q.SoftDeleteSku(ctx, store.SoftDeleteSkuParams{ID: gone.ID, Now: &now}); err != nil {
			return err
		}
	}
	n, err := q.RecomputePriceRange(ctx, store.RecomputePriceRangeParams{ID: id, Now: now})
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("catalog: price range of product %s not recomputed", id)
	}
	return nil
}

func (s *Service) writeTags(ctx context.Context, q *store.Queries, id uuid.UUID, p prepared) error {
	for i, t := range p.tags {
		if err := q.InsertProductHashtag(ctx, store.InsertProductHashtagParams{ProductID: id, Name: t.Name, DisplayName: t.Display, Position: int32(i)}); err != nil {
			return err
		}
	}
	return nil
}

// imageSlot 是一個要寫進 product_images 的位置，ref 指向 p.gallery 或描述區塊裡的圖片。
type imageSlot struct {
	field string
	kind  string
	ref   *model.Image
}

// writeImages 解決圖庫與描述裡的圖片引用並寫進 product_images：新的 token 在這個 transaction 內 Confirm；既有的 asset 必須是這個商品現有的圖；
// 沒有被引用的現有資產在同一個 transaction 內 MarkDeleted（媒體 ADR 2026-10-03）。Confirm 或 MarkDeleted 失敗整筆回滾。
// 成功後 p.gallery 與描述區塊裡的圖片都帶 AssetID 與 ObjectKey。
func (s *Service) writeImages(ctx context.Context, q *store.Queries, id uuid.UUID, uploader media.Uploader, existing []store.CatalogProductImage, p *prepared) error {
	var slots []imageSlot
	for i := range p.gallery {
		slots = append(slots, imageSlot{field: fmt.Sprintf("Gallery[%d]", i), kind: kindGallery, ref: &p.gallery[i]})
	}
	for i, ref := range description.ImageRefs(p.desc) {
		slots = append(slots, imageSlot{field: fmt.Sprintf("Description.Image[%d]", i), kind: kindDescription, ref: ref})
	}
	byAsset := make(map[uuid.UUID]store.CatalogProductImage, len(existing))
	var before []uuid.UUID
	for _, e := range existing {
		byAsset[e.AssetID] = e
		before = append(before, e.AssetID)
	}
	var items []media.ConfirmItem
	var confirming []*imageSlot
	for i := range slots {
		sl := &slots[i]
		if sl.ref.Token != "" {
			items = append(items, media.ConfirmItem{Field: sl.field + ".Token", Token: sl.ref.Token})
			confirming = append(confirming, sl)
			continue
		}
		cur, ok := byAsset[sl.ref.AssetID]
		if !ok {
			return model.Invalid(sl.field+".AssetID", model.ReasonUnknown)
		}
		sl.ref.ObjectKey = cur.ObjectKey
	}
	if len(items) > 0 {
		assets, err := media.ConfirmFields(ctx, s.d.Media, uploader, items, media.VisibilityPublic)
		if err != nil {
			return err
		}
		for i, sl := range confirming {
			sl.ref.AssetID, sl.ref.ObjectKey, sl.ref.Token = assets[i].ID, assets[i].ObjectKey, ""
		}
	}
	if err := q.DeleteProductImages(ctx, id); err != nil {
		return err
	}
	var after []uuid.UUID
	pos := map[string]int32{}
	for _, sl := range slots {
		if slices.Contains(after, sl.ref.AssetID) {
			return model.Invalid(sl.field, model.ReasonDuplicate)
		}
		after = append(after, sl.ref.AssetID)
		if err := q.InsertProductImage(ctx, store.InsertProductImageParams{
			ProductID: id, Kind: sl.kind, Position: pos[sl.kind], AssetID: sl.ref.AssetID, ObjectKey: sl.ref.ObjectKey, Alt: sl.ref.Alt,
		}); err != nil {
			return err
		}
		pos[sl.kind]++
	}
	if retired := media.RetiredAssets(before, after); len(retired) > 0 {
		return s.d.Media.MarkDeleted(ctx, retired)
	}
	return nil
}
