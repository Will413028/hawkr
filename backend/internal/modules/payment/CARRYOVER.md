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
| 賣家與管理員觸發的重讀名額（`seller_read_claimed_at`，10 秒） | 外部讀取由賣家或管理員觸發，頻率不受 job 唯一鍵約束 | 成立（D33） | 對由使用者觸發的外部呼叫限頻 | 在讀 provider 之前以條件式 UPDATE 搶名額，失敗與並行的讀取都占名額，名額內依 `fetched_at` 回成功或 `ResourceExhausted`（帶剩餘秒數）。`httpapi` 的限流器是傳輸層行程內的固定時間窗計數（`internal/httpapi/ratelimit.go`），不看帳戶列上的讀取結果；這裡的回答要看快取是否已涵蓋占名額的讀取，所以名額放在帳戶列。呼叫拿掉時，名額與該欄一起拿掉 |
| 快取存 `requirements` 的原始 JSON | 補件內容只用來顯示，狀態不從它推導 | 成立（D13） | 開通完成看 `stripe_transfers.status`，`requirements` 只用來顯示要補什麼 | 整份原樣存（非物件的值存成 `{}`，`accounts_requirements_check`），只有後台經 `httpapi/payment` 轉成公開條件（`requirements()` 不匯出，賣家端的 `PaymentAccount` 不帶 `requirements`）；不另建欄位表，Stripe 增減條目不需要 migration |
| 記憶體內的 fake provider | 本機與測試要在沒有真的金流商時走完開通 | 成立 | 以 fake 取代外部服務，正式環境不可選 | `PAYMENT_PROVIDER=fake` 只在 `requireDevEnvironment` 成立時選得到（有 `DEV_AUTH_KEY_FILE` 且沒有 Cloudflare Access 設定），否則啟動失敗（`cmd/hawkr/payment.go`）；每個呼叫檢查不在 DB transaction 內且有 deadline（規則 5），`AutoActive` 讓不認得的外部帳戶也回轉帳已啟用 |
| 核准時把聯絡信箱複製到帳戶列（`contact_email`） | 帳戶還在 seller，核准與建立在同一個模組 | 不成立（D50，修訂 D23） | 下層不存上層個資的副本，需要時向擁有者讀 | 欄位刪除；`Owners.OwnerContactEmail` 在建立外部帳戶的當下讀擁有者目前的信箱，讀不到時不建立、記成建立失敗（永久失敗，補填後由管理員重試）；建立分兩步（D55）：第一步只帶 `metadata.store_id` 與列上的冪等鍵，信箱在第二步的更新寫入，所以信箱換了也不會 `idempotency_error` |
| 結果未知的建立請求放超過 Stripe v2 冪等鍵的保存期（30 天）再重試 | 冪等鍵保存期內同一把鍵回同一個帳戶（D55） | 部分成立：超過保存期同一把鍵會建出第二個帳戶 | 以 `metadata.store_id` 對帳，找出並關閉多出的帳戶 | 已處理（D59，9g）：列上有「已呼叫」時刻（`create_called_at`），建立與清理在 30 天內重送同一把鍵，超過 30 天先列出 provider 沒有關閉的帳戶以 `metadata.store_id` 與命名空間比對（`Provider.ResolveAccount` 只找不建；已關閉的帳戶 `metadata` 被清成 null，同店舊帳戶不會被比對到，spike Q24）：找到就用它，整份列完都沒有才換新鍵（建立）或刪列（清理）；列出失敗或翻頁超過上限回錯誤，不當成沒有帳戶。比對要 `store_id` 與 `metadata.hawkr_namespace`（`STRIPE_ACCOUNT_NAMESPACE`）都相符（D61：兩個環境可能共用一個 Stripe 帳號）；同一家店有多個沒關閉的帳戶時，清理全部關閉才刪列，建立採用最新的一個並先關閉其餘（任何一個關閉失敗就重試、不寫外部 ID）。一次列出的總時間受 `platform/stripe` 呼叫上限（30 秒）與 100 頁（2,000 個帳戶）限制，超過回錯誤等重試，期限到了進終局。觸發重新評估：平台的 Stripe 帳戶數接近 2,000 時改成按頁各自呼叫、或改用 Stripe 的搜尋 |
| `provider` 欄位：帳戶列記下建立它的 provider | 同一個部署同時只設定一個 provider，但列要知道外部 ID 屬於誰 | 成立 | 外部 ID 與 provider 一起存，唯一鍵含 provider | 列的 provider 與目前設定不同時，同步、建立與重新整理略過它，賣家端重讀回 `FailedPrecondition`，清理不關也不刪、記警示 log（列在清理 job 的第一步已標 `purge_pending_at`）；舊列沒有自動搬遷或重建的去向，換 provider 的流程還沒有設計。觸發重新評估：接第二家金流商時 |
| 清理的永久失敗沒有終局狀態 | 金流商回報帳戶不存在、或列的 provider 沒有設定時，關不掉也不能刪列 | 部分成立：分不出「帳戶真的不存在」與「金鑰指向另一個 sandbox」，所以不刪對應 | 有終局狀態或人工處理入口 | 已處理（D59、D60，9g）：永久拒絕立即、其餘從標成等待清理起 7 天（`PurgeDeadline`）仍關不掉或問不到 provider，列標 `purge_blocked_at` 與原因進終局，週期掃描與清理 job 不再處理（沒有設定 provider 時掃描仍重排等待清理的列，呼叫過 provider 的列才會在期限後進終局；`TestSweepWithoutAProviderRequeuesPurgeAndTheRowReachesTheTerminalStateAfterTheDeadline`），後台待辦 `PAYMENT_ACCOUNT_PURGE_STUCK`，管理員「重試關閉」（清除終局、期限重算）或「已在金流商處理，刪除對應」（寫 `payment.account_mapping_deletions`、刪列，權限 `payment.accounts.delete_mapping`）。沒有終局之前的限制不再成立。永久拒絕不含 Stripe 的 401、403（金鑰或權限設定問題，維持可重試到期限）；空的對帳列表要等標記滿 1 小時才刪列；一次 job 最多關閉 3 個多餘帳戶；job 自己被取消不記成 `deadline_exceeded`（細節見 `docs/modules/payment.md` §2） |
| 同步讀到已關閉的外部帳戶沒有終局狀態 | 已關閉的帳戶讀取時回應沒有 `stripe_transfers` 能力，adapter 回一般錯誤，同步 job 當成暫時性失敗重試（sandbox 實測 2026-11-14，計畫附件 `spike/outputs/q22_adapter.txt`） | 部分成立：清理是關閉後就刪列，正常讀不到；帳戶在金流商那邊被關閉時才會遇到 | 讀取分辨「已關閉」並把列帶到終局狀態或人工處理入口 | 已處理（D61，9g）：adapter 在讀取時看 `closed`（先於能力檢查，不再讀銀行帳號），回 `Snapshot.Closed`；`StatusOf` 推導成 `DISABLED`、原因碼 `closed`（`ReasonClosed`），轉帳能力關閉並照常發布 `PayoutAccountChanged`；輪詢的 SQL 把原因碼 `closed` 視為終局，不再排同步。原因碼只給後台，沒有新的對外列舉值或賣家文字（賣家看到的是 `DISABLED` 的既有目錄文字）。不經輪詢的明確動作（webhook 事件、賣家與管理員的重新整理）仍會重讀，結果相同 |
| 帳戶列的建立事實：第一次呼叫前記 `create_called_at`、第一步拿到外部 ID 就寫、第二步成功才記 `create_completed_at`（「已建立」「可開通」「輪詢」「同步」看它，不看有沒有外部 ID） | 建立是兩次對外呼叫、外部呼叫不在 transaction 內，兩次之間與回應遺失都可能中斷；冪等鍵會過期 | 成立（D55、D59） | 外部副作用先記意圖（outbox），拿到結果立刻存，完成與否用單獨的事實表示 | 欄位約束（`accounts_created_check`、`accounts_snapshot_check`、`accounts_create_failed_check`、`accounts_completed_check`、`accounts_called_check`）以 `create_completed_at` 判斷；半建立的列對外仍是 `NOT_CREATED`；不另建 outbox 表，意圖就是列上的 `create_called_at`。必要條件是建立分兩步，改成一步時三個欄位與約束一起拿掉 |
| `ResolveAccount` 只找不建（列出沒有關閉的帳戶，以 `metadata.store_id` 與 `metadata.hawkr_namespace` 比對，回傳全部符合的） | 第一步已寫入 `metadata.store_id`；列出含空帳戶（spike Q24）；冪等鍵 30 天後不可當查詢鍵 | 成立（2026-11-14 sandbox 實測） | 以不變的業務鍵對帳 | 取代原先「取得或建立」的 `ResolveAccount`；重送改用 `AcquireAccount`（同一把鍵）。fixture 是合成的（`platform/stripe/testdata/list_accounts_page*.json`），待 sandbox 重錄（錄製在加入命名空間之前，帳戶沒有 `hawkr_namespace`，現有測試只驗證它們不被比對到；重錄後補正向比對）。必要條件是 provider 列表含空帳戶並帶 metadata，不成立時超過 30 天的列直接進終局交人處理 |
| 清理終局（`purge_blocked_at`、`purge_blocked_reason`）與後台待辦 `PAYMENT_ACCOUNT_PURGE_STUCK`、紀錄表 `account_mapping_deletions` | 關閉會被金流商拒絕或 provider 長期問不到，要有人處理，且處理（刪對應）要留下是誰做的 | 成立（D59、D60） | 終局狀態加人工處理入口、不可逆動作留稽核紀錄 | 期限 7 天（`PurgeDeadline`）；管理員兩個動作各有權限；紀錄表只增不改、模組沒有改寫路徑（沒有資料庫層的不可改約束）。觸發重新評估：出現第二種擁有者，或需要紀錄不可被資料庫管理員改寫時 |
| 輪詢補漏（15 分鐘）對所有未啟用的帳戶，沒有年齡上限或退避 | 事件可能遺失，要有補漏 | 成立 | 輪詢加上限或退避 | 沿用，不設上限。觸發：上線前定上限 |
| `inbox.events` 的未處理事件永不刪、沒有終局狀態 | 目前只有收款帳戶一個 handler，補掃重派到被處理為止 | 成立（只有一個 handler） | 事件有終局狀態（死信），到期後移出補掃 | 沿用：`Purge` 只刪已處理而且過了保存期的事件，未處理的由補掃或模組的掃描處理，每個新 handler 要自己補掃。觸發：第二個 handler 接上 inbox 時 |

## 傳輸層的沿用盤點

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 後台的 3 支收款帳戶 RPC 在 `SellerAdminService` | 帳戶在 seller | 不成立 | 資源的 API 在擁有者的 service | 搬到 `hawkr.admin.payment.v1alpha1.PaymentAdminService`，權限改成 `payment.accounts.*`（D30） |
| 賣家端的 `GetMyPaymentAccount`、`CreatePaymentOnboardingSession` 在 `SellerAppService` | 要先確認呼叫者是店家擁有者 | 成立：payment 是葉節點，不能向上讀 seller 驗證擁有者 | 擁有者驗證留在上層的入口 | 沿用；seller 的 handler 確認店家後呼叫 payment（D30） |
| 網站頁面的路徑由伺服器持有（`rpc/web_origin.go`） | 回跳網址在向金流商建立連結時就要給定，origin 是部署設定（`PUBLIC_WEB_ORIGIN`） | 成立 | 伺服器依部署設定組絕對回跳網址 | 路徑與語系前綴只在這一處，client 不傳網址（D49-2）；`TestOnboardingReturnPagesExistInTheWebsiteRoutes` 守門路徑在 website 的路由存在 |
| 回跳目的地（`OnboardingReturnDestination`） | 網站認證頁與 App 的回跳頁不同（App 的頁面免登入、沒有語系前綴） | 成立 | client 傳目的地的 enum，不傳網址 | 請求帶 enum，沒有網址也沒有 `locale`（語系走 `Accept-Language`，D58）；未指定回 `INVALID_ARGUMENT` |
| 內嵌能力宣告（`embedded_runtimes`） | 只有呼叫端知道自己能不能承載內嵌元件，決定表要用它 | 成立 | 呼叫端宣告能力，伺服器據以回不同形狀的資源（Stripe 的內嵌元件不能放 App 的 web view） | `repeated` enum，空表示不能內嵌，不認得的值與 `UNSPECIFIED` 忽略；形狀只由 `account.ChooseSession`（帳戶列的 provider、收件方式加宣告）決定，website 宣告 `STRIPE_CONNECT_JS`，seller-app 送空清單（D58） |
| 自動重讀的觸發點（進入認證頁、從開通頁回來、App 回前景） | 賣家沒有手動重新整理的按鈕，狀態靠 webhook 加重讀更新 | 成立 | 動作只在一個設定頁，狀態由 webhook 與自動重讀更新 | client 在這三個時點呼叫 `RefreshMyPaymentAccount`，任務已完成或鎖住時不呼叫，失敗不顯示；伺服器端 10 秒間隔（D33） |
| webhook 入口 `POST /webhooks/stripe` 在身分與 CSRF 中介層之外 | 簽章就是驗證 | 成立 | 同 | 沿用；handler 搬到 `internal/httpapi/payment`，寫 inbox 後才回 2xx |

## 與 舊服務的差異

- 系統未上線，資料沒有搬移：seller 的 `payment_accounts` 與 `payment_provider_events` 刪除，`payment.accounts` 與 `inbox.events` 是新的 baseline。
- 後台 RPC 的 service 與權限名稱改變（見上表）；admin 同一次更新。
