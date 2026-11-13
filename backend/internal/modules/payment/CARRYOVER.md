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
| 手寫的 Stripe HTTP client（`internal/platform/stripe`） | 沒有官方 SDK | 不成立（D51） | 用官方 SDK | 改用 `github.com/stripe/stripe-go/v87` v87.1.0，所有呼叫用 SDK 釘住的 API 版本（`2026-10-28.endive`），不覆寫 `Stripe-Version`（D51、E1 修訂）。手寫保留的只有 SDK 不提供的：429 依 `Retry-After` 退避（最多 3 次、一次呼叫總上限 30 秒；SDK 的自動重試關掉，其餘失敗交給 River job 重試）、在 HTTP 層記下非 2xx 的狀態與本文（SDK 的錯誤型別依 API 版本與種類而異、有的不帶狀態碼，錯誤分類只看這份）、取回應本文的 `requirements` 原始 JSON（SDK 型別只保留它認得的欄位）、webhook 簽章時間戳的容忍（SDK 只拒絕過舊的）、`CloseAccount` 對已關閉帳戶的 403 改讀帳戶確認；webhook 簽章只用 SDK 驗證，SDK 只拒絕過舊的時間戳、不拒絕未來太遠的時間戳（舊手寫版兩邊都拒），簽章之後只讀 `id`、`type`、`related_object` 不用 SDK 的型別化結構 |
| 收款帳戶快取整列覆寫、不比時間 | 同一帳戶同一時間只有一個同步（River 唯一鍵） | 不成立：賣家端的同步重讀在 job 之外呼叫外部讀取，與同步 job 並行（D32、D49） | 以讀取開始時間做樂觀條件，較舊的讀取不覆蓋較新的 | `ApplyPaymentSnapshot` 只有讀取開始時間不早於已存的 `fetched_at` 才寫；被擋下的那次不標記 inbox 事件、不發 `PayoutAccountChanged`。必要條件是有 job 以外的同步呼叫；那個呼叫拿掉時，條件一起拿掉 |
| 賣家觸發的重讀名額（`seller_read_claimed_at`，10 秒） | 外部讀取由賣家觸發，頻率不受 job 唯一鍵約束 | 成立（D33） | 對由使用者觸發的外部呼叫限頻 | 在讀 provider 之前以條件式 UPDATE 搶名額，失敗與並行的讀取都占名額，名額內依 `fetched_at` 回成功或 `ResourceExhausted`（帶剩餘秒數）。`httpapi` 的限流器是傳輸層行程內的固定時間窗計數（`internal/httpapi/ratelimit.go`），不看帳戶列上的讀取結果；這裡的回答要看快取是否已涵蓋占名額的讀取，所以名額放在帳戶列。呼叫拿掉時，名額與該欄一起拿掉 |
| 快取存 `requirements` 的原始 JSON | 補件內容只用來顯示，狀態不從它推導 | 成立（D13） | 開通完成看 `stripe_transfers.status`，`requirements` 只用來顯示要補什麼 | 整份原樣存（非物件的值存成 `{}`，`accounts_requirements_check`），後台與賣家端經 `httpapi/payment` 轉成公開條件；不另建欄位表，Stripe 增減條目不需要 migration |
| 記憶體內的 fake provider | 本機與測試要在沒有真的金流商時走完開通 | 成立 | 以 fake 取代外部服務，正式環境不可選 | `PAYMENT_PROVIDER=fake` 只在 `requireDevEnvironment` 成立時選得到（有 `DEV_AUTH_KEY_FILE` 且沒有 Cloudflare Access 設定），否則啟動失敗（`cmd/hawkr/payment.go`）；每個呼叫檢查不在 DB transaction 內且有 deadline（規則 5），`AutoActive` 讓不認得的外部帳戶也回轉帳已啟用 |
| 核准時把聯絡信箱複製到帳戶列（`contact_email`） | 帳戶還在 seller，核准與建立在同一個模組 | 不成立（D50，修訂 D23） | 下層不存上層個資的副本，需要時向擁有者讀 | 欄位刪除；`Owners.OwnerContactEmail` 在建立外部帳戶的當下讀擁有者目前的信箱，讀不到時不建立、記成建立失敗（永久失敗，補填後由管理員重試）；建立分兩步（D55）：第一步只帶 `metadata.store_id` 與列上的冪等鍵，信箱在第二步的更新寫入，所以信箱換了也不會 `idempotency_error` |
| 結果未知的建立請求放超過 Stripe v2 冪等鍵的保存期（30 天）再重試 | 冪等鍵保存期內同一把鍵回同一個帳戶（D55） | 部分成立：超過保存期同一把鍵會建出第二個帳戶 | 以 `metadata.store_id` 對帳，找出並關閉多出的帳戶 | 沒有對帳：只靠 River 的重試窗與後台重試都遠短於 30 天。觸發重新評估：上線前 |
| `provider` 欄位：帳戶列記下建立它的 provider | 同一個部署同時只設定一個 provider，但列要知道外部 ID 屬於誰 | 成立 | 外部 ID 與 provider 一起存，唯一鍵含 provider | 列的 provider 與目前設定不同時，同步、建立與重新整理略過它，賣家端重讀回 `FailedPrecondition`，清理不關也不刪、記警示 log（列在清理 job 的第一步已標 `purge_pending_at`）；舊列沒有自動搬遷或重建的去向，換 provider 的流程還沒有設計。觸發重新評估：接第二家金流商時 |
| 清理的永久失敗沒有終局狀態 | 金流商回報帳戶不存在、或列的 provider 沒有設定時，關不掉也不能刪列 | 部分成立：分不出「帳戶真的不存在」與「金鑰指向另一個 sandbox」，所以不刪對應 | 有終局狀態或人工處理入口 | 沒有終局狀態：這些列每次週期掃描都重排清理、重試並記警示 log，對應不刪。觸發重新評估：上線前 |
| 同步讀到已關閉的外部帳戶沒有終局狀態 | 已關閉的帳戶讀取時回應沒有 `stripe_transfers` 能力，adapter 回一般錯誤，同步 job 當成暫時性失敗重試（sandbox 實測 2026-11-14，計畫附件 `spike/outputs/q22_adapter.txt`） | 部分成立：清理是關閉後就刪列，正常讀不到；帳戶在金流商那邊被關閉時才會遇到 | 讀取分辨「已關閉」並把列帶到終局狀態或人工處理入口 | 與上一列一起處理（Will 2026-11-14）。觸發重新評估：上線前 |
| 輪詢補漏（15 分鐘）對所有未啟用的帳戶，沒有年齡上限或退避 | 事件可能遺失，要有補漏 | 成立 | 輪詢加上限或退避 | 沿用，不設上限。觸發：上線前定上限 |
| `inbox.events` 的未處理事件永不刪、沒有終局狀態 | 目前只有收款帳戶一個 handler，補掃重派到被處理為止 | 成立（只有一個 handler） | 事件有終局狀態（死信），到期後移出補掃 | 沿用：`Purge` 只刪已處理而且過了保存期的事件，未處理的由補掃或模組的掃描處理，每個新 handler 要自己補掃。觸發：第二個 handler 接上 inbox 時 |

## 傳輸層的沿用盤點

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 後台的 3 支收款帳戶 RPC 在 `SellerAdminService` | 帳戶在 seller | 不成立 | 資源的 API 在擁有者的 service | 搬到 `hawkr.admin.payment.v1alpha1.PaymentAdminService`，權限改成 `payment.accounts.*`（D30） |
| 賣家端的 `GetMyPaymentAccount`、`CreatePaymentOnboardingSession` 在 `SellerAppService` | 要先確認呼叫者是店家擁有者 | 成立：payment 是葉節點，不能向上讀 seller 驗證擁有者 | 擁有者驗證留在上層的入口 | 沿用；seller 的 handler 確認店家後呼叫 payment（D30） |
| webhook 入口 `POST /webhooks/stripe` 在身分與 CSRF 中介層之外 | 簽章就是驗證 | 成立 | 同 | 沿用；handler 搬到 `internal/httpapi/payment`，寫 inbox 後才回 2xx |

## 與 舊服務的差異

- 系統未上線，資料沒有搬移：seller 的 `payment_accounts` 與 `payment_provider_events` 刪除，`payment.accounts` 與 `inbox.events` 是新的 baseline。
- 後台 RPC 的 service 與權限名稱改變（見上表）；admin 同一次更新。
