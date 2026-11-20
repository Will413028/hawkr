# crm 模組

## 職責

店家對買家的客戶管理：分類（`NORMAL`／`VIP`）、標籤、客戶列表與詳情、`VIPStatuses`；以及消費統計（`total_spent`、`order_count`）的過渡投影，與買家刪帳、店家退場的資料清除。給 httpapi（A6-5 的 `SellerCrmService`）與 buyer 的會員卡列表使用。行為規格：`backend/docs/plans/2026-11-19-a6-review-im-crm/step-3-design.md`；A6-3b 尚無對外入口，`cmd/hawkr` 除登記 migration 外尚未接線（consumer、worker、訂閱者、`VIPStatuses` 的切換都在 A6-6e）。

## 擁有的資料

schema `crm`（`db/schema.sql`）：`customers`（主鍵 `(store_id, buyer_id)`）、`tags`、`customer_tags`、`order_nets`（每單淨額與是否收過全額退款，投影）、`ledger`（已處理事件，投影）、`inactive_parties`（刪帳者、退場店家的複本）。沒有個資欄（暱稱、電話等）。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與訂閱者。
- `internal/write`（分類與標籤）、`internal/read`（列表、詳情、標籤、VIP）、`internal/project`（投影：`net.go` 是純函式，`project.go` 是 transaction）、`internal/purge`（刪帳與退場的訂閱者邏輯、清除 job 與保底掃描）、`internal/text`（伺服器句子目錄）、`internal/model`、`internal/store`（sqlc）。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。依賴 buyer、seller 只為訂閱 `BuyerDeleted`、`StoreRetired`；暱稱由 httpapi 經 `buyer.BatchDisplay` 補。
- 消費事件的 consumer 在 `internal/legacy/crmconsumer`（只在測試啟用）。

## 不變量

- 分類只有 `NORMAL`、`VIP`（CHECK 與 `model.CustomerType` 同值域）；`BLOCKLIST` 不存在於任何層，封鎖只在店家封鎖名單。`SetCustomerType` 對非客戶回 NotFound。守門：`TestCustomerTypeDomain`、`TestSetCustomerType`。
- `Customer` 型別沒有任何聯絡或個資欄位（A6-D8）。守門：`TestCustomerHasNoContactFields`。
- 標籤名稱經 `trimtext.Trim`、1–50 字元，`UNIQUE (store_id, name)`；`customer_tags` 以複合 FK 保證標籤與客戶同店、且只有客戶才能貼標。每店 200 個、每位客戶 20 個（先鎖客戶列再鎖店家標籤鎖 `LockStoreTags`；建立、貼標、改名、刪除標籤都持這把鎖，名稱到 ID 的解析與檢查、插入串行）。`AddCustomerTags` 單 transaction，任何一步失敗整批回滾。
- 客戶只在首筆完成訂單時建立，退款不建。`total_spent` 恆等於該客戶 completed 訂單 `SUM(GREATEST(net_amount, 0))`：事件在鎖住的 `order_nets` 列上算這張單貢獻的變化量，加到客戶（`total_spent + delta`，不以 `SUM` 重算）。`order_count` 是完成訂單數，退款不扣。守門：`project/net_test.go`、`TestConcurrentOrdersOfOneCustomer`。
- 投影冪等靠 `ledger.dedup_key` 的 UNIQUE（`order:<event_id>`；退款 `refund:<external_ref_id 或 AS_REFUND_<case_no>>`，Java 每次重發退款都換 `event_id`）。River 的 unique 只是減少重複排程。
- 刪帳與退場：訂閱者同 transaction 只寫 `inactive_parties`、排 `crm.erase_party`；投影在 transaction 內依序持買家、店家的 advisory lock（共享模式，投影之間不互相排隊；`Deactivate` 持獨占）再查 `inactive_parties`，已停用就不寫。清除 job 先 `order_nets`（ledger cascade）後 `customers`（`customer_tags` cascade），店家另刪 `tags`；`crm.erase_sweep` 每 15 分鐘補排。
- 可見性：`inactive_parties` 裡的買家或店家，讀取（列表、詳情、標籤、`VIPStatuses`）看不到，賣家寫入回 NotFound（客戶不存在、標籤不存在），不等清除 job 跑完。守門：`TestInactiveBuyerIsInvisibleAndReadOnlyToSellers`、`TestInactiveStoreIsInvisibleAndReadOnlyToSellers`。
- 儲存的代碼是 UPPER_SNAKE。

## 不負責

- 封鎖名單：seller（`StoreBlocksBuyer` 等）。暱稱與頭像：buyer。優惠券數、訂單明細：各擁有者（A6-5）。
- 客戶列表的關鍵字搜尋：不做（A6-D18 W5）。
- 消費統計的資料來源：Java 訂單與售後，ordering 搬進單體後投影退場。

## 決策紀錄

從舊服務沿用的每個機制、當時的決定與理由，以及與舊服務的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。決策：計畫 A6-D2、A6-D8、A6-D9、A6-D18。
