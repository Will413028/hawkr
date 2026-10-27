# 運費費率快取不做 warm-up、不做排程刷新

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18 |
| 影響範圍 | order-service / product-service 的運費費率 L1 快取（呼叫 shipping-service） |

## 脈絡

運費費率一年只變動數次，但每筆結帳都要查。呼叫端因此在本機持有一份 5 分鐘 TTL 的
記憶體快取，miss 時才打 shipping-service gRPC。

問題是：這份快取要不要在 pod 啟動時預熱、要不要用排程定期刷新。

規格本身見 `docs/詳細設計書/21-運費計算模組（Shipping Fee Module）詳細設計書.md` §3.2。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| `@PostConstruct` warm-up | 把 order-service 的**啟動**綁死在 shipping-service 的可用性上。dev 環境曾因此產生約 21 秒的冷啟動拒絕視窗（結帳一路回 `ALL_SHIPPING_EXCEEDED`），而那段時間 order-service 自己是健康的 | 否決 |
| `@Scheduled` 排程刷新 | shipping-service 只要抖一下，整批排程刷新一起失敗；而 TTL 自然 expire 是逐 key、隨流量攤開的，抖動只影響當下那一筆 | 否決 |
| **首次請求觸發 cold load ＋ TTL 自然 expire** | pod 啟動後第一個請求承擔一次 gRPC RTT | **採用** |

## 連帶決策：Circuit Breaker fail-open 用 stale cache

shipping-service 連續失敗 ⇒ CB 開路 ⇒ 不再嘗試 gRPC ⇒ 用既有 cache 回答（資料可能
stale 5～30 秒）。**只有 cache 完全為空時**才回 `SHIPPING_SERVICE_UNAVAILABLE`。

取捨：運費費率是低變動資料，用 30 秒前的值算運費，遠好過讓買家結不了帳。
這與「session 撤銷不可用 stale 值」是相反的判斷，因為兩者的失效代價不同——
費率 stale 的代價是幾塊錢的誤差，而且下一次刷新就自癒。

## 後果

- 第一個請求較慢是預期行為，不是 bug，不要為它加 warm-up 回來。
- stale cache 放行意味著費率調整不會在所有 pod 同時生效，最長落後一個 TTL。
  若未來出現「費率必須秒級一致」的需求（例如限時活動費率），這個決策要重新評估。
