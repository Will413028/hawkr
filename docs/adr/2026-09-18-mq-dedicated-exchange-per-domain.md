# 代行營運與發票各自開新 exchange，而不是掛既有領域 exchange

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18 |
| 影響範圍 | `hawkr.delegation`（56 號）、`hawkr.invoice`（58 號） |

## 脈絡

新增一個事件領域時有兩條路：掛既有 exchange（零接線成本），或開一個新的
（要動 notification-service 的對照表、`topology.yaml`、並重啟服務）。

以 `#` 綁定的 consumer（notification-service 正是如此）**會收到該 exchange 上的每一條 routing key**，
這讓「掛既有 exchange」的代價不是零。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| **代行事件掛 `hawkr.governance` 或 `hawkr.seller`** | 治理事件是平台**對**賣家的處分、賣家領域事件是賣家自身的生命週期，而代行是平台**替**賣家操作且橫跨商品／店面／貼文／訂單——不從屬於任一資源領域。掛進去會讓該領域的事件表出現與它無關的 routing key，而 `#` 綁定的 consumer 收得到卻無法分流 | ❌ |
| **發票事件掛 `hawkr.order`** | 完全不動 notification-service（`notification.order` 是 `'#'` 綁定），但會讓 `hawkr.order` 出現第二個 publisher、ownership 稀釋，且**發票死信混進 `hawkr.order.dlx`** | ❌ |
| **各自開新 exchange** | 三項接線（notification map 一列 ＋ `topology.yaml` 一條 queue ＋ 一次服務重啟），**三者缺一則該領域的通知永遠不會送出，唯一訊號是 notification-service 啟動時一行 ERROR log** | ✅ **採用** |

## 決策

代行營運走 `hawkr.delegation`，發票走 `hawkr.invoice`，各自擁有自己的 DLX。

## 後果

- 每新增一個領域 exchange 都要付那三項接線；漏掉任何一項是**靜默失效**，
  因此它被寫成 36 號 §10.1 的強制流程與 §3.3.1 的「四面登記」。
- ⚠️ **這條理由只對「出向」成立**：發票的兩條入向 queue
  （`invoice.order.paid`／`invoice.order.refund.finalized`）綁的是 `hawkr.order`，
  其 `.dlq` **必然**掛在 `hawkr.order.dlx` 上，無法也不該迴避。
