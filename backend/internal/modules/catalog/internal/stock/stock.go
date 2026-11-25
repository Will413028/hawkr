// Package stock 是庫存的冪等扣減與歸還（B4 D2）：
//   - 每次操作是 stock_operations 的一列，冪等鍵 UNIQUE；先插入操作列、再條件式扣減（WHERE stock_qty >= n），不做「先查再扣」，
//     所以同一個鍵的併發重送只生效一次（CD1）。
//   - 歸還依原扣減：只帶原扣減的冪等鍵，累計不超過原扣減量（CD2）；items 省略表示歸還剩下的全部。
//   - 全部寫入在呼叫方的 transaction 內（ctx 沒有時自己開一個）；失敗整筆回滾，包括操作列。
//   - SKU 列依 ID 升序鎖定，兩個多品項扣減不會互相死結。
package stock

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

const (
	kindDeduct  = "DEDUCT"
	kindRestore = "RESTORE"
	kindAdjust  = "ADJUST"
	maxKeyRunes = 200
)

// Deps 是庫存的依賴。
type Deps struct {
	DB    *db.DB
	NewID func() uuid.UUID
	Now   func() time.Time
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
			affected, err := q.DeductSkuStock(ctx, store.DeductSkuStockParams{ID: it.SkuID, Quantity: it.Quantity, Now: now})
			if err != nil {
				return err
			}
			if affected == 0 {
				return s.whyNotDeducted(ctx, q, it.SkuID)
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
		n, err := q.InsertStockOperation(ctx, store.InsertStockOperationParams{ID: opID, IdempotencyKey: key, Kind: kindAdjust, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return s.replayAdjust(ctx, q, storeID, key, skuID, delta)
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
		return q.InsertStockMovement(ctx, store.InsertStockMovementParams{OperationID: opID, SkuID: skuID, Delta: delta})
	})
}

// replayAdjust 處理同一個 key 重送：先確認 SKU 屬於呼叫的店家（別家店重送同一個 key 與內容，回 ErrSkuNotFound，與第一次送別家 SKU 相同），
// 再比對原操作的內容。
func (s *Service) replayAdjust(ctx context.Context, q *store.Queries, storeID int64, key string, skuID uuid.UUID, delta int32) error {
	own, err := q.SkuInStore(ctx, store.SkuInStoreParams{ID: skuID, StoreID: storeID})
	if err != nil {
		return err
	}
	if !own {
		return model.ErrSkuNotFound
	}
	op, err := q.GetStockOperationByKey(ctx, key)
	if err != nil {
		return err
	}
	if op.Kind != kindAdjust {
		return model.ErrIdempotencyKeyReused
	}
	moves, err := q.ListStockMovements(ctx, op.ID)
	if err != nil {
		return err
	}
	if len(moves) != 1 || moves[0].SkuID != skuID || moves[0].Delta != delta {
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
		// 鎖住原扣減：同一次扣減的多筆歸還排隊，下面的累計檢查看得到前一筆已提交的歸還。
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
		}
		return nil
	})
}

// restorable 回傳原扣減每個 SKU 還能歸還的量（原扣減量減已歸還的累計）。
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

// replay 處理冪等鍵已存在：內容相同回 nil，不同回 ErrIdempotencyKeyReused。
// 歸還省略 items 的重送，只要求種類與原扣減相同（剩下的全部取決於當時，不能重算）。
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

// whyNotDeducted 區分扣減失敗的原因：SKU 不存在或已刪除，或庫存不足。
func (s *Service) whyNotDeducted(ctx context.Context, q *store.Queries, sku uuid.UUID) error {
	_, err := q.GetLiveSkuStock(ctx, sku)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrSkuNotFound
	}
	if err != nil {
		return err
	}
	return &model.InsufficientStockError{SkuID: sku}
}

func checkKey(key string) error {
	if n := utf8.RuneCountInString(key); n < 1 || n > maxKeyRunes || trimtext.Trim(key) != key {
		return model.ErrInvalidStockInput
	}
	return nil
}

// normalize 檢查輸入並依 SKU ID 升序排好（鎖定順序與重送比對都依這個順序）。
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
