# crm 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

來源：`backend/docs/plans/2026-11-19-a6-review-im-crm/step-3-design.md` §12 與 §0.4，Will 的決定 A6-D2（黑名單）、A6-D8（只顯示暱稱與訂單統計）、A6-D18（W1–W5）。本檔對應 A6-3b（模組、`legacy/crmconsumer`）；Connect 與 handler 在 A6-5，consumer、worker、訂閱者與 `VIPStatuses` 的接線在 A6-6e。

## 沿用盤點

crm-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| ledger＋投影（`crm_customer_ledger` 逐事件加減 `total_spent`、`order_count`） | 訂單與退款的資料在 Java，crm 拿不到訂單表 | 成立（ordering 尚未搬進單體，Q7） | 讀時聚合 ordering 的資料，不複製 | 沿用（過渡）：資料在 Java 時只能投影；ordering 搬進單體後 `order_nets`、`ledger`、consumer 退場，改讀 ordering 的 `pub_*`；不做讀時聚合、重算工具、回填 |
| advisory lock（`hashtext(order_no)`）序列化同單事件 | 同一單的訂單與退款事件不可交錯 | 成立，但 32 位元雜湊會讓不相干的單互等，且鎖不在列上 | 每單一列，用列鎖序列化 | 改：`order_nets` 每單一列，`LockOrderNet` 鎖列；`customers` 只加這張單貢獻的變化量（`total_spent + delta`），不以 `SUM` 重算 |
| 全額退款以 ledger 自己的淨額沖銷（不信 Java 的 `refund_total_amount`） | Java 的全額退款金額可能與 crm 記錄的不一致 | 成立 | 以自己記錄的淨額為準 | 沿用：全額退款把該單 `net_amount` 歸零；後到的訂單遇到已有全額退款記 0；「收過全額退款」是 `order_nets.full_refunded`（在已上鎖的列上讀寫），`ledger` 不存 kind、不為這個查詢另建索引 |
| 部分退款無下限、退款以 `order_count=0` 建客戶 | 無（舊碼的缺陷） | 不成立：`total_spent` 可能為負、出現 0 筆訂單的客戶 | 下限寫進 CHECK 並由計算結構保證 | 改：每單貢獻 `max(net, 0)`，`customers` CHECK `>= 0`；退款不建客戶列；順帶吸收舊註解自承的「部分退款排在全額退款之後再扣一次」 |
| 退款冪等鍵 `external_ref_id`，缺時 `"AS_REFUND_"+case_no` | 上游沒有穩定的事件 ID：Java 每次建構 `RefundCompletedPayload` 都換新 UUID，`RefundSyncWorker` 每小時重發 | 成立 | 上游給穩定的事件 ID | 沿用（Java 不改）；鍵加命名空間前綴（`order:`、`refund:`），上限 128；ordering、aftersales 搬進單體後換單體事件 |
| MQ handler 直接寫 DB、重試交給 broker | 無 | 不成立：單體有 River | consumer 只驗證並排 job，重試與觀測交給 job 系統 | 改：`legacy/crmconsumer` 的 handler 只解碼、驗證、排 River job；worker 在 transaction 外解析訂單、再投影；正確性來自 `ledger` 的 `UNIQUE(dedup_key)`，River 的 unique 只減少重複排程 |
| 壞輸入的分類（`order.completed` 壞 JSON 丟棄、缺欄位重試、ID 為 0 丟棄；退款一律 permanent） | 壞輸入重試不會變好 | 成立 | 同一種壞輸入同一種分類 | 改：兩個事件的壞 JSON、缺必要欄位、ID 非正整數、金額非整數或為負、未知退款結果、鍵過長一律 permanent；訂單不存在 permanent，訂單服務或 DB 的錯誤重試 |
| 退款在 transaction 外呼叫訂單服務取店家與買家（CD10） | 事件沒有帶店家與買家 | 成立 | 事件自帶，或 in-process 讀 | 接受：移到 River worker；`orderservice.OrderParties` 失敗時 fail-closed 重試；ordering 搬進單體後 in-process |
| 暱稱、電話快取欄＋列表讀取後非同步回寫（M9） | crm 要顯示暱稱與按暱稱搜尋 | 不成立：A6-D8 只留暱稱，buyer 有 `BatchDisplay` | 讀時補，不存個資副本 | 改：刪 `nickname`、`phone`；暱稱由 httpapi 讀時經 buyer 補（刪帳者顯示 Deleted） |
| 列表的關鍵字搜尋（暱稱、電話 `ILIKE`） | 賣家要按名字找客戶 | 不成立：沒有個資副本，buyer 沒有搜尋 API 與 profile 變更事件 | 搜尋走擁有者的索引 | 不做（A6-D18 W5 (a)）；賣家以分類、標籤、排序找人；UI 改動在 A6-6，要先取得 Will 同意 |
| `GetCustomer` 回電話、Email、預設地址、優惠券數（向 identity、buyer、coupon 取） | CRM 詳情要聯絡方式 | 不成立：A6-D8 只顯示暱稱與訂單統計 | 賣家看不到買家聯絡方式，除非有訂單履約需要 | 改：`Customer` 沒有聯絡欄位；去 identity 依賴；優惠券數不做、不建 coupon legacy client（W3）；訂單由擁有者提供（A6-5） |
| TSID `id`（客戶）、`BIGSERIAL`（`customer_tags`） | 別的服務或 client 拿這個 ID 當鍵 | 不成立：client 都以 `buyerId` 定位 | 關聯表用自然鍵，自有實體用 UUIDv7（row-identity ADR） | 改：`customers` 主鍵 `(store_id, buyer_id)`；`tags.id` 是 UUIDv7；`customer_tags` 主鍵是三欄自然鍵 |
| `customer_type` 字串無 CHECK，註解 `normal, vip, blocklist` | 賣家要區分一般與 VIP；封鎖要在客戶列表裡操作 | 不成立：封鎖只在店家封鎖名單（A6-D2 (i)） | enum＋CHECK | 改：CHECK 只有 `NORMAL`、`VIP`；儲存的代碼是 UPPER_SNAKE（ADR 2026-09-25-codemeta-retirement D3 與 `TestStoredCodesInChecksAreUpperSnake`，設計書寫的小寫與它衝突，以測試強制的為準）；`SetCustomerType` 驗值域與列數 |
| 標籤：`crm_tags` 無 `UNIQUE(store_id,name)`、`customer_tags` FK 只指 `tags(id)`、`AddCustomerTags` 任何錯誤當不存在而新建、單筆失敗只記 warn 並回成功 | 同店標籤名稱唯一、標籤與客戶同店、批次是原子的 | 成立（舊碼的缺陷） | 唯一鍵＋複合 FK＋單 transaction | 改：`UNIQUE(store_id,name)`；複合 FK `(store_id, tag_id)`；`AddCustomerTags` 單 transaction，名稱不存在才建，任何錯誤整批回滾；加標籤數上限（每店 200、每客戶 20，W4）；刪除 `SearchCustomersByTags` 等沒有呼叫者的 RPC |
| `page`／`size`＋`COUNT`、`LIMIT/OFFSET`、`ORDER BY` 無次序鍵（M18） | 賣家要翻頁 | 成立，但 OFFSET 隨深度變慢、同值的列跨頁會重複或遺漏 | keyset，不回總數 | 改：`(排序鍵, buyer_id)` keyset；三個排序欄位 × 兩個方向各一句查詢與索引 |
| `buyer.deleted` 只抹 `nickname`、`phone`，客戶列、標籤、ledger 保留；店家退場沒有處理 | 刪帳後不留對可識別個人的主觀標註 | 不成立：沒有個資欄後只剩標註與統計，標籤是對可識別個人的主觀註記 | 刪帳清除，並關閉「刪後重放舊事件重建」 | 改（A6-D18 W2 (b)）：訂閱 `BuyerDeleted`、`StoreRetired`，同 transaction 只排清除 job（A6-3c 起不存複本）；投影先取擁有者守門 `LockNotClosed`、`LockNotRetired`（`FOR KEY SHARE`，不互相排隊，與轉移的 `FOR UPDATE` 互斥，取代原本的 advisory lock 與複本）；清除 job 分批刪；保底掃描；`customers(buyer_id)` 索引是這個決定的必要條件 |
| `first_order_at` 的語意（舊 `created_at`） | 列表預設排序要有「最早成為客戶」的時間 | 成立 | 取事件的完成時間 | 沿用語意、取處理時間：Java 的 `completed_at` 是未規定格式的字串，且舊資料不搬，所以用投影收到事件的時間（`Now()`）；ordering 搬進單體後改取訂單的完成時間 |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。本步（A6-3b）沒有對外入口，下列各項的落地在 A6-5。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 認證與身分（gateway 以「有訂單」判斷店家能不能操作某位客戶） | 店家只能操作自己的客戶 | 成立 | store_id 由 session 決定，資源以 `(store_id, buyer_id)` 授權 | 改：所有方法帶呼叫者決定的 `store_id`，客戶列不屬於這家店回 NotFound；貼標的資格由 DB 保證（只有客戶才有 `customer_tags`）；session 解析在 A6-5 |
| 錯誤（舊 gateway 的錯誤碼） | client 要能分辨可處理的情境 | 成立 | 錯誤分類＋ reason | 改：`apperr` 分類加模組端的 `Reason` 常數（`CRM_CUSTOMER_NOT_FOUND`、`CRM_TAG_NOT_FOUND`、`CRM_TAG_NAME_TAKEN`、`CRM_TAG_LIMIT`）；proto 的 `ErrorReason` 在 A6-5 定案；句子在 `internal/text`（zh-TW、en） |
| 分頁 page／size＋總數 | 見 M18 | 不成立 | keyset | 改：見上；不回總數（Q9） |
| 下游呼叫的逾時與重試 | 訂單服務不能無限等 | 成立 | 明確 deadline、失敗重試 | 沿用 `orderservice` 的逾時；失敗由 River 退避重試，有上限 |
| 請求大小與限流 | 防濫用 | 成立 | 入口統一 | 不在模組：A6-5；名稱 50 字元、單次 20 個名稱、`VIPStatuses` 100 家店在模組與 DB CHECK |

## 與 crm-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。

- 表：`customers`（主鍵 `(store_id, buyer_id)`、`first_order_at`；無 TSID `id`、`nickname`、`phone`、`created_at`）、`tags`（UUIDv7、`UNIQUE(store_id,name)`）、`customer_tags`（三欄自然鍵、複合 FK）、`order_nets`、`ledger`（`id` uuidv7、`dedup_key` UNIQUE；無 `store_id`、`buyer_id`、`count_delta`、`amount_delta`）。
- 儲存的代碼（`customer_type`、清除 job 的 `party_kind`）是 UPPER_SNAKE（`NORMAL`、`VIP`、`BUYER`、`STORE`）。
- 沒有 `blocklist`；沒有客戶列表的關鍵字搜尋；詳情沒有電話、Email、地址、優惠券數。
- 退款不建客戶列；同一單號的第二個 `order.completed`（換了 `event_id`）不重複累加。
- `legacy/crmservice`（只有 `VIPStatuses`）與 `services/crm-service` 不動：並行期單一生效者是舊服務，切換與刪除在 A6-6e。

## A6-3c 停用身分改由擁有者守門的沿用盤點

設計：`backend/docs/plans/2026-11-19-a6-review-im-crm/step-3c-design.md` §9.3、§9.5。

| 照搬的機制 | 從零設計 | 結論 |
|---|---|---|
| `inactive_parties`、`LockParty`／`LockPartyShared`（基準形狀但放錯擁有者） | 事實與守門應在擁有者 | 不成立；改 `LockNotClosed`＋`LockNotRetired`，刪表（`crm_drop_inactive_parties`）與 advisory lock |
| 「Deactivate 與 retireBatch 的死結論證」 | 新設計的 retireBatch 是 `FOR UPDATE SKIP LOCKED`，不等待、不進 lockorder 的等待集合 | 論證不再需要，已刪除；`TestRetiringManyStoresWhileProjectingDoesNotDeadlock` 對新機制驗證 |
| 標籤與客戶寫入只讀複本、不取鎖 | 寫入要與轉移串行 | 缺口，已補：`TestTagWriteInFlightAcrossStoreRetirementLeavesNothing`（先以測試 commit 重現，再修）；`requireActiveStore`、`IsStoreInactive` 刪除，守門的 NotFound 對應 `tagNotFound`／客戶 NotFound |
| 抹除 job、保底掃描 | 抹除形狀在 crm 自己 | 成立，保留；`Deactivate` 改名 `EnqueueErase`（只排 job），掃描改 `UNION ALL` 讀兩個 pub，保留 `(kind, party_id)` keyset |
| 投影端的守門 | ordering 搬遷時投影退場 | 過渡程式；退場點與 `step-3-design.md` §0 一致 |
| 投影遇到不存在的買家或店家（W1 (a′)） | 買家、店家列關閉與退場後仍保留，不存在只可能是壞資料 | 行為變更：原本照寫，改後跳過（不重試）；守門回 NotFound 時才多讀一次 `pub_closed_buyers()`／`pub_retired_stores()` 分辨：已關閉、已退場記 info，不存在記 error；transaction 內只記守門的結果並提交空的 transaction，分辨的讀取在 transaction 之後（非 tx 連線，查詢失敗不會中止要提交的 transaction）；分辨查詢失敗記 error 並照樣跳過（Will 2026-11-21 修訂） |

觸發條件（門檻是推測，沒有量測）：(1) 店家 `retired_at` 落後於排定時點超過 1 小時（4 輪）：該店在下一輪不用 `SKIP LOCKED`、改為等待取鎖；(2) `crm.erase_sweep` 單輪耗時超過 30 秒，或 `SELECT count(*) FROM buyer.buyers WHERE status = 'CLOSED'` 超過 10 萬列：改為 buyer 加部分索引、seller 加 `WHERE retired_at IS NOT NULL` 的索引（設計 W5 (a)）。

## A6-5d 可見性與讀寫一致（fix round 2）

| 照搬的機制 | 從零設計 | 結論 |
|---|---|---|
| handler 在寫入提交後以 `BatchDisplay.deleted` 判斷客戶不可見 | 可見性由擁有資料的模組在寫入前判斷 | 不成立；刪除 handler 檢查。寫入路徑的 `lockCustomerParties` 以 `buyer.LockNotClosed`（與 `pub_closed_buyers` 同一事實）在寫入前回 NotFound，`BatchDisplay` 只做顯示對應；`TestInactiveBuyerIsInvisibleAndReadOnlyToSellers` 驗證 NotFound 且什麼都沒寫 |
| 讀取在店家關閉或不存在時回空頁或略過檢查，寫入回 NotFound | 讀寫同一個結論 | 改：讀取也回 NotFound（客戶層 `CUSTOMER_NOT_FOUND`，`ListTags` 為 `TAG_NOT_FOUND`）；`TestInactiveStoreIsInvisibleAndReadOnlyToSellers` 期望同步改 |

D1（停權時收縮類方法放行）待 Will 決定：`CapContentContract` 的定義只含內容動作，沒有適用 CRM 資料的收縮類能力，目前 9 個方法維持 `CapMarketing`。
