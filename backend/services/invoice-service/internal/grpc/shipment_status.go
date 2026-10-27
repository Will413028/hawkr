package grpc

import (
	"context"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// S06／A12 的「出貨狀態」欄（#3019）
//
// 🔴 這一欄在 2026-09-02 之前**全服務零賦值**：proto 有 `shipment_status`、
// gateway 照實透傳（`invoice_dto.go` 的 `d.ShipmentStatus = m.GetShipmentStatus()`）、
// 只有生產者那一端從來沒填 ⇒ 賣家對帳時看不到「這筆訂單出貨了沒」，
// 而 S06 正是他處理發票的主畫面。
//
// 🔴 **空字串與「查不到」必須分得開。** ADM 的 A12 頁面逐字寫著
// 「這一欄尤其不能畫：空白會被讀成『還沒出貨』」——所以降級**不能**靠回空字串，
// 否則修完之後症狀只是換一個形式重演（前端畫了一欄，而那一欄在 shipping-service
// 抖動時會集體說謊）。降級訊號走 data 層的 `shipment_status_degraded`：
// 它是**整批**的性質（一次 RPC 涵蓋整頁），也因此不必往 shipment_status 的值域裡
// 塞 ShippingStatus CodeMeta 以外的哨兵值（§8 禁止硬編碼）。
// ---------------------------------------------------------------------------

// ShipmentStatusResolver 是出貨狀態的出向面（實作在 client/shipping）。
//
// 介面宣告在消費端：本層只需要「給我一批 order_no、還我狀態碼」，
// 不需要知道 shipment 還有哪些欄位。
type ShipmentStatusResolver interface {
	BatchGetShipmentStatus(ctx context.Context, orderNos []string) (map[string]string, error)
}

// resolveShipmentStatuses 批次取出貨狀態。
//
// 第二個回傳值 degraded ＝「這一批沒取到，別把空值讀成沒出貨」。
//
// 🔴 **失敗降級但不中止列表**：出貨狀態是識別欄不是資料本身，
// 做成 fail-loud 會讓 shipping-service 一次抖動就把整個發票紀錄頁打不開，
// 而發票資料本身完全健康。但降級**必須說出口**（degraded=true），
// 這正是它與 resolveStoreNames 的差別：店名取不到會退回 store_id，
// 畫面上看得出來是「沒有名字」；出貨狀態取不到退回空字串，
// 畫面上看起來是「還沒出貨」——一個看得出來，一個會說謊。
//
// ⚠️ 降級同時要留痕（Warn），否則「出貨狀態又全空了」在下一次發生時無從診斷。
func (s *InvoiceServer) resolveShipmentStatuses(ctx context.Context, orderNos []string) (map[string]string, bool) {
	ids := dedupeStrings(orderNos)
	if s.shipmentStatuses == nil || len(ids) == 0 {
		// resolver 沒接上 ⇒ 這一欄從來就沒有來源，與「接上了但這次取不到」不同：
		// 前者回 degraded=false（畫面退回修這件事之前的樣子），後者才是降級。
		return map[string]string{}, false
	}
	statuses, err := s.shipmentStatuses.BatchGetShipmentStatus(ctx, ids)
	if err != nil {
		if s.log != nil {
			s.log.Warn("invoice.query.shipment_status_unavailable",
				zap.Int("order_count", len(ids)),
				zap.String("degrade", "shipment_status 整批退回空字串，回應帶 shipmentStatusDegraded=true"),
				zap.Error(err))
		}
		return map[string]string{}, true
	}
	return statuses, false
}

// dedupeStrings 去重並濾掉空值。一頁裡同一張訂單出現多列是常態
// （作廢重開會產生 SUPERSEDED ＋ 新票兩列，兩列同一個 order_no）。
func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
