// Package catalogconsumer 把舊 Java order 的事件接到 catalog 模組：order.cancelled 歸還庫存、order.sales.accumulated 累計銷量（過渡：ordering 搬進單體後，
// 改由訂單取消與付款成功的同 transaction 呼叫，B8）。本套件隨 Java 的 order 刪除。
//
// handler 解碼、驗證後直接呼叫 catalog，冪等由 catalog 保證：歸還以 stock_tx_id 派生的鍵（catalogserver.RestoreKey，與 gRPC 的 RestoreStock 共用）指向原扣減，
// 重送只歸還一次；銷量以 order_no 去重，重送不再累計。MQ 會重送，所以兩者都不另外去重。
package catalogconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/catalogserver"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

// Catalog 是 consumer 需要的 catalog 模組方法（*catalog.Module 滿足）。
type Catalog interface {
	RestoreStock(ctx context.Context, key, deductKey string, items []catalog.StockItem) error
	RecordSales(ctx context.Context, orderNo string, items []catalog.SalesItem) error
}

// Consumer 是 catalog 的 legacy consumer。
type Consumer struct {
	Catalog Catalog
}

type permanentError struct{ error }

func invalid(format string, args ...any) error { return permanentError{fmt.Errorf(format, args...)} }

func isPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

type number string

func (n *number) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*n = number(s)
		return nil
	}
	*n = number(b)
	return nil
}

func (n number) positiveInt32() (int32, bool) {
	v, err := strconv.ParseInt(string(n), 10, 32)
	return int32(v), err == nil && v > 0
}

type cancelledEvent struct {
	OrderNo   string `json:"order_no"`
	StockTxID string `json:"stock_tx_id"`
	Items     []struct {
		SkuID    string `json:"sku_id"`
		Quantity number `json:"quantity"`
	} `json:"items"`
}

// OrderCancelled 驗證 order.cancelled 並依 stock_tx_id 歸還庫存；同一個 stock_tx_id 重送只歸還一次。
// stock_tx_id 空、品項格式不合是 permanent；沒有品項是不需要歸還（訂單沒有扣過庫存）直接略過，不讓 catalog 把空品項解讀成「歸還剩下的全部」；
// 找不到原扣減（舊契約：視為補償成功）略過。
func (c Consumer) OrderCancelled(ctx context.Context, body []byte) error {
	var e cancelledEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return permanentError{fmt.Errorf("catalogconsumer：order.cancelled payload：%w", err)}
	}
	if e.StockTxID == "" {
		return invalid("catalogconsumer：order.cancelled 缺 stock_tx_id（order_no %q）", e.OrderNo)
	}
	if len(e.Items) == 0 {
		return nil
	}
	items := make([]catalog.StockItem, 0, len(e.Items))
	for _, it := range e.Items {
		id, err := uuid.Parse(it.SkuID)
		if err != nil {
			return invalid("catalogconsumer：order.cancelled sku_id %q 不是 UUID", it.SkuID)
		}
		q, ok := it.Quantity.positiveInt32()
		if !ok {
			return invalid("catalogconsumer：order.cancelled quantity %q 不是正整數", string(it.Quantity))
		}
		items = append(items, catalog.StockItem{SkuID: id, Quantity: q})
	}
	err := c.Catalog.RestoreStock(ctx, catalogserver.RestoreKey(e.StockTxID), e.StockTxID, items)
	switch {
	case err == nil, errors.Is(err, catalog.ErrDeductionNotFound):
		return nil
	case errors.Is(err, catalog.ErrInvalidStockInput), errors.Is(err, catalog.ErrIdempotencyKeyReused), errors.Is(err, catalog.ErrRestoreExceedsDeduction):
		return permanentError{err}
	}
	return err
}

type salesEvent struct {
	OrderNo string `json:"order_no"`
	Items   []struct {
		ProductID string `json:"product_id"`
		Quantity  number `json:"quantity"`
	} `json:"items"`
}

// OrderSalesAccumulated 驗證 order.sales.accumulated 並依 order_no 累計銷量；同一個 order_no 重送只累計一次。
// order_no 缺是 permanent（沒有去重鍵不可累計）；product_id 空或 quantity 不是正整數的品項略過（舊 consumer 同），product_id 有值但不是 UUID 是 permanent；
// 全部被略過或沒有品項就整則略過。catalog 回商品不存在（整筆含登記回滾）也是 permanent：不重試不會變好，進 dead-letter queue 由人處理。
func (c Consumer) OrderSalesAccumulated(ctx context.Context, body []byte) error {
	var e salesEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return permanentError{fmt.Errorf("catalogconsumer：order.sales.accumulated payload：%w", err)}
	}
	if e.OrderNo == "" {
		return invalid("catalogconsumer：order.sales.accumulated 缺 order_no")
	}
	items := make([]catalog.SalesItem, 0, len(e.Items))
	for _, it := range e.Items {
		q, ok := it.Quantity.positiveInt32()
		if it.ProductID == "" || !ok {
			continue
		}
		id, err := uuid.Parse(it.ProductID)
		if err != nil {
			return invalid("catalogconsumer：order.sales.accumulated product_id %q 不是 UUID", it.ProductID)
		}
		items = append(items, catalog.SalesItem{ProductID: id, Quantity: q})
	}
	if len(items) == 0 {
		return nil
	}
	err := c.Catalog.RecordSales(ctx, e.OrderNo, items)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, catalog.ErrProductNotFound), errors.Is(err, catalog.ErrInvalidInput):
		return permanentError{err}
	}
	return err
}
