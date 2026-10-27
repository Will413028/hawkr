# order-service 的事件可靠性走「同 tx outbox」，不補「會傳遞失敗的 publish 變體」

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18 |
| 影響範圍 | order-service 的全部 domain 事件發布路徑 |

## 脈絡

order-service 有多個事件的前置寫入是**永久的**：

- `order.delivered`：`updateOrderDelivered` 的 CAS allowed-from 不含 `DELIVERED` 自己 ⇒ 成功一次之後該 row 就離開候選集合。
- `order.sales.accumulated`：`payment_event_dedup` 會擋住重投。

而 `OrderEventPublisher.publish` **吞掉所有例外**。兩者相加的後果是：狀態／dedup 已在事務內落地，事件卻在 `afterCommit`／tx 外 best-effort 直發時失敗 ⇒ **永久遺失且零告警**。

依《多副本排程搶佔規範》，這個形狀有兩條合法出路：

- **例外 A**：把事件寫進與業務狀態同一個事務的 outbox。
- **出路 4**：把 publish 拉進 CAS 所在的單列短 tx，失敗即 rollback、下輪重試（前提是 publisher 會把失敗訊號傳遞出來，因此要新增一支「會 throw 的 publish 變體」）。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 出路 4：新增 `tryPublishXxx`（會傳遞失敗的 publish 變體） | ① order-service 已有 `order_outbox`，且 `order.paid`／`order.cancelled`／`order.completed`／`order.receipt.confirmed` 全部走它 ⇒ 補一支變體等於在同一個類別裡造出**第二種做法**，下一個人得先判斷「這個事件走哪一種」才知道怎麼加新事件 ② 仍留有「publish 成功、commit 失敗」的暫態窗口 | 否決 |
| **例外 A：同 tx 寫 `order_outbox`** | 事件多繞一次 DB 與一次排程輪詢（延遲，不是遺失） | **採用** |

## 決策

order-service 的所有 domain 事件一律**在業務狀態變更的同一個事務內寫 `order_outbox`**，由 `OrderOutboxPublisherScheduler` 投遞。不新增任何「會傳遞失敗的 publish 變體」，`OrderEventPublisher` 的契約不動。

## 後果

- order-service 內「事件要怎麼發」只有一個答案。
- 與 aftersales-service 的做法不同（該服務走出路 4）——**差別在基建而不在判準**：aftersales 沒有通用事件 outbox。
- 重新評估的條件：若未來 order-service 出現「必須在 CAS 當下同步確認 broker 已收下」的事件（outbox 的延遲不可接受），才需要重開這個決策。
