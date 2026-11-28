// Package stock 是庫存的冪等扣減與歸還：
//   - 每次操作是 stock_operations 的一列，冪等鍵在各自範圍內唯一（結帳的操作全域唯一，賣家調整依店家）；先插入操作列、再條件式扣減（WHERE stock_qty >= n），不做「先查再扣」，
//     所以同一個鍵的併發重送只生效一次。
//   - 歸還依原扣減：只帶原扣減的冪等鍵，累計不超過原扣減量；items 省略表示歸還剩下的全部。
//   - 全部寫入在呼叫方的 transaction 內（ctx 沒有時自己開一個）；失敗整筆回滾，包括操作列。
//   - SKU 列依 ID 升序鎖定，兩個多品項扣減不會互相死結。
//   - 庫存增加（賣家調整與設定、歸還）讓 SKU 由 0 變可買時，同 transaction 排補貨通知的 job（restock 套件，D26）。
package stock

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/restock"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/idempotency"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

const (
	kindDeduct  = "DEDUCT"
	kindRestore = "RESTORE"
	kindAdjust  = "ADJUST"
	kindSet     = "SET"
	maxKeyRunes = 200
)

// Deps 是庫存的依賴。
type Deps struct {
	DB    *db.DB
	NewID func() uuid.UUID
	Now   func() time.Time
	// Restock 偵測庫存由 0 變可買並排補貨通知的 job（在寫入的 transaction 內）。
	Restock *restock.Service
}

// Service 是庫存的 use case。
type Service struct{ d Deps }

// New 建立庫存 use case。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// Deduct 扣減庫存。同一個 key 重送且內容相同時不再扣減、回 nil；內容不同回 ErrIdempotencyKeyReused。
// 任何一個 SKU 庫存不足整次不生效，回 *model.InsufficientStockError。
func (s *Service) Deduct(ctx context.Context, key string, items []model.StockItem) error {
	items, err := normalize(key, items)
	if err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		now := s.d.Now()
		opID := s.d.NewID()
		n, err := q.InsertStockOperation(ctx, store.InsertStockOperationParams{ID: opID, IdempotencyKey: key, Kind: kindDeduct, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return s.replay(ctx, q, key, kindDeduct, nil, items)
		}
		for _, it := range items {
			affected, err := q.DeductSkuStock(ctx, store.DeductSkuStockParams{ID: it.SkuID, Quantity: it.Quantity, At: now})
			if err != nil {
				return err
			}
			if affected == 0 {
				return s.whyNotDeducted(ctx, q, it.SkuID, now)
			}
			if err := q.InsertStockMovement(ctx, store.InsertStockMovementParams{OperationID: opID, SkuID: it.SkuID, Delta: -it.Quantity}); err != nil {
				return err
			}
		}
		return nil
	})
}

// Adjust 是賣家調整自己店家某個 SKU 的庫存，delta 是有號的變動量（不為 0）；帳本記一列 ADJUST 操作與一筆 movement。
// 用變動量而不是設成絕對值：movements 存的是有號 delta（只增不改的稽核帳本），變動量重送時內容固定、可比對，
// 絕對值的結果取決於當時庫存，重送無法判斷是否相同。店家身分由呼叫端推導後傳入，SKU 不屬於該店家（或已刪除）回 ErrSkuNotFound。
// 調整後的庫存必須在 0 到 int32 上限之間，否則整次不生效（減少超過現有量回 *InsufficientStockError，增加溢位回 ErrInvalidStockInput）。
// 同一個 key 重送且內容相同不再調整、回 nil；內容不同回 ErrIdempotencyKeyReused。
func (s *Service) Adjust(ctx context.Context, storeID int64, key string, skuID uuid.UUID, delta int32) error {
	if err := checkKey(key); err != nil {
		return err
	}
	if storeID <= 0 || delta == 0 || skuID == uuid.Nil() {
		return model.ErrInvalidStockInput
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		now := s.d.Now()
		opID := s.d.NewID()
		hash := idempotency.Hash(kindAdjust, skuID.String(), strconv.Itoa(int(delta)))
		n, err := q.InsertStockOperation(ctx, store.InsertStockOperationParams{ID: opID, IdempotencyKey: key, Kind: kindAdjust, StoreID: &storeID, RequestHash: hash, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return s.replayOwn(ctx, q, storeID, key, skuID, kindAdjust, hash)
		}
		affected, err := q.AdjustOwnSkuStock(ctx, store.AdjustOwnSkuStockParams{ID: skuID, StoreID: storeID, Delta: delta, Now: now})
		if err != nil {
			return err
		}
		if affected == 0 {
			if _, err := q.GetOwnLiveSku(ctx, store.GetOwnLiveSkuParams{ID: skuID, StoreID: storeID}); errors.Is(err, pgx.ErrNoRows) {
				return model.ErrSkuNotFound
			} else if err != nil {
				return err
			}
			if delta < 0 {
				return &model.InsufficientStockError{SkuID: skuID}
			}
			return model.ErrInvalidStockInput
		}
		if err := q.InsertStockMovement(ctx, store.InsertStockMovementParams{OperationID: opID, SkuID: skuID, Delta: delta}); err != nil {
			return err
		}
		return s.d.Restock.StockIncreased(ctx, skuID, delta)
	})
}

// SetQuantity 是賣家把自己店家某個 SKU 的庫存設成絕對值 quantity（0 到 int32 上限），以 expected 比對現值：現值不等於 expected 整次不生效，
// 回 *model.StockMismatchError（帶現值）；帳本記一列 SET 操作與一筆 movement（差額 quantity − expected，差額為 0 時只有操作列）。
// 比對與寫入是同一句條件式 UPDATE，沒有「先查再設」；expected 與 quantity 都在範圍內才可能成立。同一個 key 重送且內容（SKU、quantity、expected）相同不再設定、回 nil，
// 不再比對現值（第一次已生效）；內容不同回 ErrIdempotencyKeyReused。SKU 不屬於該店家或已刪除回 ErrSkuNotFound。
func (s *Service) SetQuantity(ctx context.Context, storeID int64, key string, skuID uuid.UUID, quantity, expected int32) error {
	if err := checkKey(key); err != nil {
		return err
	}
	if storeID <= 0 || skuID == uuid.Nil() || quantity < 0 || expected < 0 {
		return model.ErrInvalidStockInput
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		now := s.d.Now()
		opID := s.d.NewID()
		hash := idempotency.Hash(kindSet, skuID.String(), strconv.Itoa(int(quantity)), strconv.Itoa(int(expected)))
		n, err := q.InsertStockOperation(ctx, store.InsertStockOperationParams{ID: opID, IdempotencyKey: key, Kind: kindSet, StoreID: &storeID, RequestHash: hash, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return s.replayOwn(ctx, q, storeID, key, skuID, kindSet, hash)
		}
		affected, err := q.SetOwnSkuStock(ctx, store.SetOwnSkuStockParams{ID: skuID, StoreID: storeID, Quantity: quantity, Expected: expected, Now: now})
		if err != nil {
			return err
		}
		if affected == 0 {
			cur, err := q.GetOwnLiveSku(ctx, store.GetOwnLiveSkuParams{ID: skuID, StoreID: storeID})
			if errors.Is(err, pgx.ErrNoRows) {
				return model.ErrSkuNotFound
			}
			if err != nil {
				return err
			}
			return &model.StockMismatchError{SkuID: skuID, Current: cur.StockQty}
		}
		if delta := quantity - expected; delta != 0 {
			if err := q.InsertStockMovement(ctx, store.InsertStockMovementParams{OperationID: opID, SkuID: skuID, Delta: delta}); err != nil {
				return err
			}
		}
		if expected == 0 {
			return s.d.Restock.StockIncreased(ctx, skuID, quantity)
		}
		return nil
	})
}

func (s *Service) replayOwn(ctx context.Context, q *store.Queries, storeID int64, key string, skuID uuid.UUID, kind string, hash []byte) error {
	own, err := q.SkuInStore(ctx, store.SkuInStoreParams{ID: skuID, StoreID: storeID})
	if err != nil {
		return err
	}
	if !own {
		return model.ErrSkuNotFound
	}
	op, err := q.GetStoreStockOperationByKey(ctx, store.GetStoreStockOperationByKeyParams{StoreID: storeID, IdempotencyKey: key})
	if err != nil {
		return err
	}
	if op.Kind != kind || !bytes.Equal(op.RequestHash, hash) {
		return model.ErrIdempotencyKeyReused
	}
	return nil
}

// Restore 依 deductKey 指的原扣減歸還庫存。items 為空表示歸還剩下的全部；有給時每個 SKU 必須在原扣減內，累計歸還量不得超過原扣減量。
// 同一個 key 重送且內容相同時不再歸還、回 nil。
func (s *Service) Restore(ctx context.Context, key, deductKey string, items []model.StockItem) error {
	if err := checkKey(key); err != nil {
		return err
	}
	if err := checkKey(deductKey); err != nil {
		return err
	}
	if len(items) > 0 {
		var err error
		if items, err = normalize(key, items); err != nil {
			return err
		}
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		now := s.d.Now()
		deduct, err := q.GetStockOperationByKey(ctx, deductKey)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && deduct.Kind != kindDeduct) {
			return model.ErrDeductionNotFound
		}
		if err != nil {
			return err
		}
		if _, err := q.LockStockOperation(ctx, deduct.ID); err != nil {
			return err
		}
		opID := s.d.NewID()
		n, err := q.InsertStockOperation(ctx, store.InsertStockOperationParams{ID: opID, IdempotencyKey: key, Kind: kindRestore, DeductOperationID: &deduct.ID, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return s.replay(ctx, q, key, kindRestore, &deduct.ID, items)
		}
		want, err := s.restorable(ctx, q, deduct.ID)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			for _, sku := range sortedSkus(want) {
				if want[sku] > 0 {
					items = append(items, model.StockItem{SkuID: sku, Quantity: want[sku]})
				}
			}
			if len(items) == 0 {
				return model.ErrRestoreExceedsDeduction
			}
		}
		for _, it := range items {
			if it.Quantity > want[it.SkuID] {
				return model.ErrRestoreExceedsDeduction
			}
			if _, err := q.RestoreSkuStock(ctx, store.RestoreSkuStockParams{ID: it.SkuID, Quantity: it.Quantity, Now: now}); err != nil {
				return err
			}
			if err := q.InsertStockMovement(ctx, store.InsertStockMovementParams{OperationID: opID, SkuID: it.SkuID, Delta: it.Quantity}); err != nil {
				return err
			}
			if err := s.d.Restock.StockIncreased(ctx, it.SkuID, it.Quantity); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) restorable(ctx context.Context, q *store.Queries, deductID uuid.UUID) (map[uuid.UUID]int32, error) {
	moves, err := q.ListStockMovements(ctx, deductID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int32, len(moves))
	for _, m := range moves {
		out[m.SkuID] = -m.Delta
	}
	restored, err := q.SumRestoredBySku(ctx, &deductID)
	if err != nil {
		return nil, err
	}
	for _, r := range restored {
		out[r.SkuID] -= r.Restored
	}
	return out, nil
}

func (s *Service) replay(ctx context.Context, q *store.Queries, key, kind string, deductID *uuid.UUID, items []model.StockItem) error {
	op, err := q.GetStockOperationByKey(ctx, key)
	if err != nil {
		return err
	}
	if op.Kind != kind || (deductID == nil) != (op.DeductOperationID == nil) || (deductID != nil && *deductID != *op.DeductOperationID) {
		return model.ErrIdempotencyKeyReused
	}
	if len(items) == 0 {
		return nil
	}
	moves, err := q.ListStockMovements(ctx, op.ID)
	if err != nil {
		return err
	}
	if len(moves) != len(items) {
		return model.ErrIdempotencyKeyReused
	}
	for i, m := range moves {
		delta := items[i].Quantity
		if kind == kindDeduct {
			delta = -delta
		}
		if m.SkuID != items[i].SkuID || m.Delta != delta {
			return model.ErrIdempotencyKeyReused
		}
	}
	return nil
}

func (s *Service) whyNotDeducted(ctx context.Context, q *store.Queries, sku uuid.UUID, now time.Time) error {
	state, err := q.GetLiveSkuStock(ctx, store.GetLiveSkuStockParams{At: now, ID: sku})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrSkuNotFound
	}
	if err != nil {
		return err
	}
	if !state.ProductVisible {
		return model.ErrProductNotPurchasable
	}
	return &model.InsufficientStockError{SkuID: sku}
}

func checkKey(key string) error {
	if n := utf8.RuneCountInString(key); n < 1 || n > maxKeyRunes || trimtext.Trim(key) != key {
		return model.ErrInvalidStockInput
	}
	return nil
}

func normalize(key string, items []model.StockItem) ([]model.StockItem, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	if len(items) == 0 || len(items) > model.MaxStockItems {
		return nil, model.ErrInvalidStockInput
	}
	out := slices.Clone(items)
	slices.SortFunc(out, func(a, b model.StockItem) int { return bytes.Compare(a.SkuID[:], b.SkuID[:]) })
	for i, it := range out {
		if it.Quantity <= 0 || (i > 0 && out[i-1].SkuID == it.SkuID) {
			return nil, model.ErrInvalidStockInput
		}
	}
	return out, nil
}

func sortedSkus(m map[uuid.UUID]int32) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.SortFunc(out, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	return out
}
