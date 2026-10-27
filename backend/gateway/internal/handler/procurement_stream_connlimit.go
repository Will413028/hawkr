package handler

import "sync"

// maxProcurementStreamConnsPerBuyer 限制每買家同時開啟的 turns/stream SSE
// 連線數。每條連線只服務一次 turn（最長受 Compose 的 ~30 秒上限約束），這裡
// 只是防一個買家開一堆分頁／重試造成大量並發連線，數字給幾個分頁的餘裕。
const maxProcurementStreamConnsPerBuyer = 5

// 獨立於 chat_ws_connlimit.go 的 wsConnCount——WS 聊天室跟這裡是不同資源池，
// 混用會讓兩個不相干的功能互相排擠彼此的配額。
var (
	procurementStreamConnMu    sync.Mutex
	procurementStreamConnCount = map[int64]int{}
)

// acquireProcurementStreamSlot 保留一個連線位子給 buyerID。回傳的 release
// idempotent-safe，可以安全 defer；ok=false（release 為 nil）代表已達上限。
func acquireProcurementStreamSlot(buyerID int64) (release func(), ok bool) {
	procurementStreamConnMu.Lock()
	defer procurementStreamConnMu.Unlock()
	if procurementStreamConnCount[buyerID] >= maxProcurementStreamConnsPerBuyer {
		return nil, false
	}
	procurementStreamConnCount[buyerID]++
	return func() {
		procurementStreamConnMu.Lock()
		defer procurementStreamConnMu.Unlock()
		if procurementStreamConnCount[buyerID] <= 1 {
			delete(procurementStreamConnCount, buyerID)
			return
		}
		procurementStreamConnCount[buyerID]--
	}, true
}
