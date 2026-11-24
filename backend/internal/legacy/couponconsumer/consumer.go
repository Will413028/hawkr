// Package couponconsumer 把舊 Java 服務的 order.completed 事件接到 coupon 模組的 MEMBER 消費紀錄投影（過渡：ordering 搬進單體後，
// 改由訂單完成的同 transaction 事件餵入並 backfill，B8）。
// handler 解碼、驗證後直接呼叫 coupon 的 RecordOrderCompleted：投影是 (店家, 買家) 的 upsert，重放天然冪等，不需要再排 job 去重。
// cmd/hawkr 的 worker 在 B7-5 才接上 Run；在那之前只有測試接線。
package couponconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Recorder 是 coupon 模組的投影入口；正式組裝傳 *coupon.Module。
type Recorder interface {
	RecordOrderCompleted(ctx context.Context, storeID, buyerID int64) (recorded bool, err error)
}

// Consumer 是 coupon 的 legacy consumer。
type Consumer struct {
	Coupon Recorder
}

type permanentError struct{ error }

func invalid(format string, args ...any) error { return permanentError{fmt.Errorf(format, args...)} }

// isPermanent：解碼失敗與必要欄位缺漏、格式不合都是 permanent（進 dead-letter queue、不重試）；其餘（資料庫）照常重排。
func isPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// orderCompletedEvent 只列投影用到的欄位（Java 的 OrderCompletedPayload 其餘忽略；ID 是字串）。
type orderCompletedEvent struct {
	BuyerID string `json:"buyer_id"`
	StoreID string `json:"store_id"`
}

func positiveID(field, v string) (int64, error) {
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, invalid("couponconsumer：order.completed %s 不是正整數", field)
	}
	return id, nil
}

// OrderCompleted 驗證 order.completed 並寫入投影；同一筆事件重送不改變結果。
func (c Consumer) OrderCompleted(ctx context.Context, body []byte) error {
	var e orderCompletedEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return permanentError{fmt.Errorf("couponconsumer：order.completed payload：%w", err)}
	}
	buyerID, err := positiveID("buyer_id", e.BuyerID)
	if err != nil {
		return err
	}
	storeID, err := positiveID("store_id", e.StoreID)
	if err != nil {
		return err
	}
	_, err = c.Coupon.RecordOrderCompleted(ctx, storeID, buyerID)
	return err
}
