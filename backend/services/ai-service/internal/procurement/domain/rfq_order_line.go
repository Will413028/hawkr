package domain

import (
	"errors"
	"fmt"
	"strings"
)

// ErrRfqOrderLineNameUnavailable 代表組不出真實品名。
//
// 🔴 它是**終態錯誤**，不是可重試的抖動：canonical payload 是不可變的，重跑一次還是
// 同一份內容。⛔ 呼叫端不得退回任何預設字串——doc 64 §6.4.3 逐字禁止 sku_name 使用
// sentinel，因為退款、售後、發票三條下游都吃它。
var ErrRfqOrderLineNameUnavailable = errors.New("procurement: rfq order line name unavailable")

// ComposeRfqOrderLineName 組出 RFQ 成交明細的 order_items.sku_name。
//
// RFQ 明細是 non-catalog line（doc 64 §6.4.9）：沒有商品主檔可以反查品名，品名就是
// order_items 自己的快照欄。而 §6.4.3 要求**真實數量與單位寫進品名**——因為
// order_items.quantity 是 INT，賣家報價的 offered_quantity 是 NUMERIC(14,3)
// （「12.5 公斤」是合法報價），小數在既有型別下表達不出來。
//
// 兩個來源各有其權威：
//   - 品名來自買家確認過的 canonical payload（受 payload_hash 保護）
//   - 數量與單位來自**被接受的那一版**賣家報價，不是買家原本要的量
//
// ⚠️ quantity 傳進來前已由 pgxutil.NumericToCanonicalString 去掉尾零（"200.000" →
// "200"），這裡不再處理格式。
func ComposeRfqOrderLineName(canonicalPayloadJSON []byte, offeredQuantity, offeredQuantityUnit string) (string, error) {
	payload, err := CompileSellerFacingPayload(canonicalPayloadJSON, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrRfqOrderLineNameUnavailable, err)
	}

	// ItemLabel 是品項解析後的正規名稱，Item 是買家自己打的原話。優先用前者，因為
	// 它與賣家在報價畫面上看到的是同一個字串（CompileSellerFacingPayload 的輸出）。
	item := strings.TrimSpace(payload.ItemLabel)
	if item == "" {
		item = strings.TrimSpace(payload.Item)
	}
	quantity := strings.TrimSpace(offeredQuantity)
	unit := strings.TrimSpace(offeredQuantityUnit)
	if item == "" || quantity == "" || unit == "" {
		return "", fmt.Errorf("%w: item=%q quantity=%q unit=%q", ErrRfqOrderLineNameUnavailable, item, quantity, unit)
	}
	return fmt.Sprintf("%s %s %s", item, quantity, unit), nil
}
