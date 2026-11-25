// Package model 是 catalog 模組的對外型別與錯誤。
package model

import (
	"errors"
	"fmt"
	"uuid"
)

// StockItem 是一個 SKU 的扣減或歸還量，Quantity 是正數。
type StockItem struct {
	SkuID    uuid.UUID
	Quantity int32
}

// MaxStockItems 是一次扣減或歸還的 SKU 數上限。
const MaxStockItems = 100

// 庫存操作的錯誤。呼叫端（對外的 API、結帳）用 errors.Is／errors.As 判斷並對應成對外的錯誤原因。
var (
	// ErrInvalidStockInput 表示輸入不合法：冪等鍵空白或過長、沒有品項、數量不是正數、同一個 SKU 出現兩次、品項過多。
	ErrInvalidStockInput = errors.New("catalog: invalid stock input")
	// ErrSkuNotFound 表示有 SKU 不存在，或扣減時已被刪除。
	ErrSkuNotFound = errors.New("catalog: sku not found")
	// ErrIdempotencyKeyReused 表示同一個冪等鍵以不同的內容（種類、原扣減或品項）重送；重送要與第一次完全相同。
	ErrIdempotencyKeyReused = errors.New("catalog: idempotency key reused with a different request")
	// ErrDeductionNotFound 表示歸還指向的扣減不存在。
	ErrDeductionNotFound = errors.New("catalog: deduction not found")
	// ErrRestoreExceedsDeduction 表示歸還的累計量會超過原扣減量，或歸還了原扣減沒有的 SKU。
	ErrRestoreExceedsDeduction = errors.New("catalog: restore exceeds the original deduction")
)

// InsufficientStockError 表示庫存不足，整次扣減沒有生效。SkuID 是第一個不足的 SKU（依 ID 升序檢查）。
type InsufficientStockError struct{ SkuID uuid.UUID }

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("catalog: insufficient stock for sku %s", e.SkuID)
}

// Category 是平台分類樹的一個節點。Name 是 LocalizedText：zh-TW 必有，en 可能為空。
type Category struct {
	ID                uuid.UUID
	ParentID          *uuid.UUID
	NameZhTW          string
	NameEn            string
	ShopifyTaxonomyID string
	Position          int32
}

// IsLeaf 回報節點是否可掛商品（有上層的節點；樹只有兩層，所以有上層就是葉節點）。
func (c Category) IsLeaf() bool { return c.ParentID != nil }
