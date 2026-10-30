# accountsuspension 模組的沿用盤點

## 沿用盤點

新模組，沒有來源服務。逐項判斷的是被它包起來的既有機制：舊 `account-service` 的 `SuspendAccount`、`identity.SuspendAccount` 的實作、`accountdeletion` 的編排形狀。系統從未上線，沒有正式資料搬移。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 舊 `account-service.SuspendAccount`：呼叫方直接停權，理由只寫 log，不看帳號有沒有店 | social 的 `BAN_USER` 要停權使用者 | 呼叫方成立（legacy accountservice 到 A7-5 刪除）；沒有紀錄與沒有店家檢查不成立：店家帳號被停權後店無人可登入，而 seller 的措施、曝光、訂單都不知道 | 不變量放在擁有狀態的服務，跨聚合的前置條件放在編排層（Etsy、eBay、Shopee 的停權連帶店鋪） | `internal/legacy/accountservice` 改呼叫本模組，理由類別固定 `OTHER`，舊的 reason 字串當說明；有店回 `ACCOUNT_HAS_STORE`，legacy 轉成 failed precondition |
| 停權檢查「有沒有店」放在 identity 或 moderation | 有店不能停權要在一處檢查 | 不成立：identity 是葉節點（`internal/arch/modules.go`），不能 import seller；放 moderation 會讓每個呼叫者各寫一份 | 編排層擁有跨聚合的前置條件 | 放本模組，identity 的入口只允許本模組呼叫（arch 測試守），所以不可能繞過 |
| `accountdeletion` 的形狀：根套件編排 identity、buyer、seller，鎖順序帳號列 → buyer → seller | 跨模組流程要序列化 | 成立 | 編排者先取最上游的鎖，再讀下游 | 同層、同鎖順序；本模組不需要 buyer，只用帳號列與 seller 的非鎖定讀取；沒有自己的資料，所以沒有 schema、沒有 `internal/` |
| 有店一律拒絕停權 | 停權對店家、未結訂單與撥款的效果未定 | 成立，缺資料（賣家條款與營運政策）；不是因為程式限制 | 停權連帶店鋪與未結訂單（marketplace 的做法） | 暫時拒絕；終態是賣家條款定出效果後連帶對店建立措施，取代拒絕（ADR 2026-10-29 的終態表） |
| （歷史：Clerk 已於 2026-10-30 移除）停權時 Clerk ban job 與認證回 `PermissionDenied` | 停權要擋住被停權者 | 擋住成立；停用 Clerk 使用者與讀不到理由不成立（A7-D17：受限登入） | X、Reddit 的停權帳號可登入、可瀏覽、不可發文 | A7-1d″ 已移除 ban、認證帶停權狀態，入口依動作性質的宣告表授權（A7-D18）；解除前已被停用的 Clerk 測試帳號由 Will 在 Clerk dashboard 手動解除 |
| 事件 `Suspended`、`Unsuspended` 同 transaction 發出 | 停權時通知被停權者（Will 2026-10-29） | 成立；消費者在 A8，本步不接 | 狀態變更與通知意圖同一個交易（outbox） | `events.Publish` 在同一個 transaction，含理由類別；沒有訂閱者時只是空操作 |
