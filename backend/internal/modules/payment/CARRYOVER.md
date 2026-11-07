# payment 模組的沿用盤點

## 沿用盤點

這個模組不是從舊服務搬來的：收款帳戶在 Stripe 計畫步驟 4–7d 寫在 seller 的 `internal/payment`，7d′ 搬到這裡（D28、D29）。下表是搬遷時照搬或改掉的機制，每一項都重新判斷過「從零設計會怎麼做」。舊的 Java payment-service 不在這次範圍（B2）。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| seller 內的 webhook inbox 表（`seller.payment_provider_events`） | 只有一個模組收 webhook | 不成立：B2、B10 也會收 provider 事件，各建一份同形狀的表 | transactional inbox 是共用的基礎設施，模組只寫業務列並標記已處理 | 不沿用；改用 platform 的 `inbox.events`（D29），payment 只登記 handler 與保存期 |
| 核准時 upsert 帳戶列（還沒有外部 ID 時以新信箱覆寫） | 核准與帳戶在同一個模組 | 不成立：帳戶在下層模組，核准的 transaction 已鎖住 seller 的列，再鎖 payment 的既有列違反全域鎖順序 | 上層只新增、下層的既有列由下層自己的 job 寫 | 改成 `INSERT … ON CONFLICT DO NOTHING`；再次核准不改信箱（規格 §8 已知限制） |
| 同步 job 直接寫閘門與防詐 hash、重新公告 | 快取與閘門在同一個模組 | 不成立：閘門、hash、公告屬於 seller | 擁有者發事件，上層以同 transaction 的訂閱者複製需要的事實 | 改成 `PayoutAccountChanged`，seller 的訂閱者寫副本、hash、公告（D12、D28） |
| 建立 job 讀店家狀態、輪詢 SQL join 店家表 | 帳戶與店家在同一個 schema | 不成立：payment 不讀 seller | 由組裝點注入擁有者查詢（依賴反轉） | `Owners.OwnerActive`；輪詢先選候選再問 |
| 擁有者鍵 `store_id` | 擁有者只有店家 | 成立（D28） | 單一擁有者時用業務鍵，第二種擁有者出現再通用化 | 沿用；不建跨 schema FK |
| 一店一個建立、一個同步 job（River 唯一鍵）、15 分鐘輪詢、冪等鍵 UUID 存在列上 | 外部呼叫在 transaction 外、事件可能遺失或合併 | 成立 | 同（Stripe 的冪等鍵與 webhook 重送建議） | 沿用；job kind 改成 `payment.*` |
| 收到事件後經 platform 的派送 job 再呼叫模組的 handler（多一跳） | 入口不能在請求內呼叫模組的處理（規則 5：commit 後才回 2xx）、platform 不認得模組的 handler | 成立：platform 只依 provider 查登記的 handler（D29），派送 job 讓 handler 失敗時由 River 重試、用完由補掃重派 | transactional inbox 由背景 worker 消化，入口只持久化 | 採用；派送 job 以 (provider, event_id) 為唯一鍵 |
| 收款事件 payload 保存 30 天（`EventRetention`） | 需要涵蓋 provider 的重送期與事後查核 | 部分成立：Stripe 重送最多 3 天；30 天是查核用的預設值，沒有法規或合約要求 | 保存期依用途訂，到期刪除 | 先用 30 天預設；觸發重新評估：需要更長的爭議／稽核追溯、或隱私政策訂出 webhook 資料的保存期時 |
| 後台待辦的成員函式在 seller（`review_payment_account_create_failed`） | 帳戶列在 seller | 不成立 | 擁有者發布 `pub_*_review_items`，reviewqueue 排序分頁 | 改由 `payment.pub_review_items` 發布；店名由後台入口組裝 |

## 傳輸層的沿用盤點

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 後台的 3 支收款帳戶 RPC 在 `SellerAdminService` | 帳戶在 seller | 不成立 | 資源的 API 在擁有者的 service | 搬到 `hawkr.admin.payment.v1alpha1.PaymentAdminService`，權限改成 `payment.accounts.*`（D30） |
| 賣家端的 `GetMyPaymentAccount`、`CreatePaymentOnboardingSession` 在 `SellerAppService` | 要先確認呼叫者是店家擁有者 | 成立：payment 是葉節點，不能向上讀 seller 驗證擁有者 | 擁有者驗證留在上層的入口 | 沿用；seller 的 handler 確認店家後呼叫 payment（D30） |
| webhook 入口 `POST /webhooks/stripe` 在身分與 CSRF 中介層之外 | 簽章就是驗證 | 成立 | 同 | 沿用；handler 搬到 `internal/httpapi/payment`，寫 inbox 後才回 2xx |

## 與 舊服務的差異

- 系統未上線，資料沒有搬移：seller 的 `payment_accounts` 與 `payment_provider_events` 刪除，`payment.accounts` 與 `inbox.events` 是新的 baseline。
- 後台 RPC 的 service 與權限名稱改變（見上表）；admin 同一次更新。
