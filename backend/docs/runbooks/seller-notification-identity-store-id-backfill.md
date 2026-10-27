# 賣家通知身分 store_id backfill：device_tokens + notification_preferences（issue #2707 C1/C2，task 8）

> 對應 [13-通知模板引擎與收件箱（Notification Template）詳細設計書](../詳細設計書/13-通知模板引擎與收件箱（Notification%20Template）詳細設計書.md) §12.1.4「三張表身分空間治理模型」。
> **何時跑**：與整個 issue #2707 分支（Task 1～8，含 `send_push.go` 推播身分解耦）**同一個 deploy 窗口**內，**部署新 pod 之前**執行（理由見下方「部署順序」——這不是可有可無的建議，倒過來跑會讓這個 backfill transaction 本身有機會跟 code 部署後的新寫入撞鍵而整批失敗，見下）。
> **兩張表**：`device_tokens`（FCM token 註冊）與 `notification_preferences`（推播偏好開關）都要 backfill，同一份 runbook、同一個窗口一起做——只做其中一張，另一張在 code 部署後會讓分歧帳號（`store_id <> account_id` 的賣家）在那張表上原地重演本 task 要修的那個 bug（漏做 device_tokens＝重演 C1，漏做 notification_preferences＝重演 C2），不會自動變好。
> **兩個 DB**：`hawkr_seller`（`sellers` 表）與 `hawkr_notification`（`device_tokens`／`notification_preferences` 表）——跨 DB，無法在單一 migration 內完成，故走人工 gate，鏡像 [product-rekey-backfill-gate.md](./product-rekey-backfill-gate.md) §0 的「兩個 DB、無法 JOIN」處理方式。
> **DB 連線**：bastion + 1Password 取憑證（同 food-safety runbook 慣例）；dev 與 stage/prod 各跑一輪。
> **最終收尾（2026-09-04）**：本 runbook 只處理 `device_tokens`／`notification_preferences` 的既有資料；`notifications.user_id` 不在這裡 mass rewrite。通知收件匣已固定為 buyer=`buyer_id`、seller=`store_id`，歷史通知／push record 維持 append-only，退休 stint 由 lifecycle cleanup 與 90-day cleanup 處理。

## 0. 背景與安全性論證

**這份 runbook 涵蓋兩張表，不是只有 `notification_preferences`**：

- `notification_preferences` 是「目前設定」表（`UNIQUE (user_id, user_type, category)`），不是像 `notifications` 那樣的 append-only 歷史紀錄。把既有列的 `user_id` 從 `account_id` 改寫成該帳號**現任** `store_id`，延續的是「這個人本來想要的設定」，不是洩漏舊 incarnation 的資料——跟 `notifications` 表的稽核/歷史語意不同，不適用「禁止遷移既有列」那條規則。
- `device_tokens` 的**寫入路徑**已由更早的 Task 1（commit `57cefff9f`）原子化：新註冊一律寫入店家**現任** store_id。但寫入路徑原子化 ≠ 既有資料已經是 store_id——Task 1 之前註冊、仍留在 account_id 空間的舊列**不會被自動取代或停用**，要等該實體裝置的 token 下次自然輪替、重新註冊才會翻到 store_id 空間（13 號設計書 §12.1.3-C 已記載這個既有陷阱）。issue #2707 task 8 的 review 抓到：本分支把 `send_push.go` 對未遷移 event_type 的推播查詢從「沿用 inbox 的 account_id」改成「永遠查詢新鮮解析的 store_id」之後，**分歧帳號**（`store_id <> account_id` 的賣家）這批**還沒自然輪替**的既有 account_id 鍵 token 會全部查不到（grandfathered 帳號因為 `store_id` 這個數字本來就等於 `account_id`，不受影響——見下方「部署順序」）——這正是本 backfill 存在的原因：不是「鏡像一個更早 task 已經做完的原子遷移」，而是**這個 task 第一次**把 `device_tokens` 既有資料也一次補齊到 store_id。

兩張表都是單一 owner（gateway 寫、notification-service 讀／讀寫）、零跨服務 producer 依賴；`notifications` 的 seller template 也已在最終收尾固定使用 `store_id`，`identity_migrated` 只保留作為 audit／contract marker，不再驅動 account_id fallback。單一 owner、無跨服務依賴的表原子遷移既有資料，不建漸進機制，是本次判準。

Grandfathered 帳號（`store_id == account_id`，尚未 re-key 過）不受影響——下方第 2 步兩張表的 UPDATE 對這批帳號都是 no-op（`SET user_id = X WHERE user_id = X`）。只有真正分歧的帳號（`store_id <> account_id`）需要搬遷，而這批帳號的母體不小：2026-07-18 起新建立的賣家都可能落在這格，不是稀有 edge case。

## 部署順序（先跑 SQL，再部署 code）——重新核算過的理由

> 2026-08-30 review 修正：本節初版誤把「9 個已遷移 event_type」當成**當下已經在 prod 生效**的狀態來論證，這是錯的——issue #2707 Task 1 到 Task 8 全部都還在同一個未合併的分支上，**目前沒有任何一個 task 部署到任何環境**。下面是重新核算過、對齊這個事實的版本。

**歷史基準（2026-08-30，本 runbook 原始部署前）**：當時 prod 跑的是修復前的舊 code——`device_tokens` 100% account_id 鍵、`notification_preferences` 100% account_id 鍵、推播查詢對全部 seller event_type 一律用 account_id。三者互相一致，當時沒有 live 問題；現在的通知收件匣已另由最終 migration 固定使用 canonical buyer_id/store_id。

**結論不變，但理由不同：本 backfill 應在部署本分支 code 之前執行**，兩者間隔越短越好，理想上同一個 deploy 窗口內背靠背完成。

> 2026-08-30 第二次 review 修正：本節上一版在這裡寫「先部署 code 會讓幾乎全部現任賣家（不分是否分歧）暫時收不到推播，先跑 SQL 只影響分歧帳號子集」，這個「兩種順序波及範圍不對稱」的講法是錯的，這裡訂正。

**先界定清楚：「暫時失效範圍」在兩種順序下其實是對稱的，這不是選 SQL-first 的理由**。不管哪種順序先跑，過渡窗口內會暫時查不到資料的都**只有分歧帳號**（`store_id <> account_id` 的賣家）——grandfathered 帳號的 `store_id` 這個數字本來就等於 `account_id`，不管查詢端這次用哪一個當 key、資料端存的是哪一個 space 寫的，兩者永遠是同一個數字，查詢必然命中；grandfathered 帳號在兩種順序底下都完全不受影響。所以兩種順序波及的都是**同一個**分歧帳號子集，範圍沒有差別。

**真正的差異、也是選 SQL-first 的理由，是 backfill 這個 transaction 本身會不會撞鍵失敗**：

- **先部署 code、後跑 SQL**：code 部署的瞬間，Task 1 的 device_tokens 寫入路徑與本 task 的 gateway 偏好寫入路徑（`resolveStoreID`）都已生效——在等 backfill 執行的這段窗口內，只要有任何一個分歧帳號的賣家剛好觸發一次新裝置註冊或更新一次偏好設定，就會**立刻**寫進一列 store_id 鍵的新資料。backfill 稍後執行到這個帳號時，`UPDATE ... SET user_id = <store_id> WHERE user_id = <account_id>` 的目標鍵值已經被那筆新資料佔走 → 撞 `uni_device_tokens_user_token` 或 `uni_notification_preferences_user_category` → **這筆連帶讓整個 backfill transaction ROLLBACK**（本 runbook §2 是整批帳號共用一個 transaction，一筆撞到就是全部重來，不是只有那一筆失敗）。分歧帳號數量越多、等待 backfill 的窗口拖得越久，撞到的機率越高。
- **先跑 SQL、後部署 code**：backfill 執行當下，兩張表尚未有任何一列是 store_id 鍵——Task 1 與本 task 的寫入路徑都還沒生效，沒有任何 code 寫過 store_id 鍵的 seller 列。backfill 的 `UPDATE` 目標鍵值保證是全新的，**不可能撞到任何已存在的資料**，一次到位乾淨執行完成，不需要重試或排除衝突帳號。

**結論**：先跑 SQL 才能保證 backfill 這個一次性操作本身穩定成功，不受「跟 code 部署後的新寫入賽跑」影響；先部署 code 則讓 backfill 暴露在撞鍵風險下，窗口越長風險越高，失敗的代價是整批 transaction 重來（且要先找出撞鍵的帳號、排除或另案處理才能重跑）。這才是先跑 SQL 的真正理由——不是「波及的賣家比較少」（兩種順序波及範圍相同，見上）。

**不嘗試消除這段過渡窗口**：這是本 repo 對「app code 不建 reconciliation job / 一次性資料清整用人工審核 SQL」既定原則（`docs/開發規範/後端服務責任邊界規範.md`）下的自然結果——SQL 由人工執行，deploy 由 CI/CD 執行，兩者天生無法做到單一原子事務。可接受的緩解方式是把間隔壓到最短（理想上排在同一次維運窗口、背靠背執行），不是搭建自動化補償機制。

## 1. 產生 UPDATE 語句（在 `hawkr_seller` 執行，唯讀）

跨資料庫無法 JOIN，改用「用 SQL 產生 SQL」：在 `hawkr_seller` 對 `sellers` 表跑一條唯讀查詢，直接把要在另一個資料庫執行的 `UPDATE` 語句組成文字輸出，人工複製、審閱後才貼到下一步執行——不寫成腳本、不做成可重複呼叫的自動化 pipeline。兩張表分開產生，方便分別審閱筆數與抽查。

```sql
-- 只為「真正分歧」的帳號產生語句（store_id <> account_id）；grandfathered 帳號
-- 跳過，反正對它們是 no-op，跳過只是讓輸出更小、更容易人工審閱。

-- (a) notification_preferences
SELECT format(
  'UPDATE notification_preferences SET user_id = %s WHERE user_id = %s AND user_type = ''seller''; -- account_id=%s -> store_id=%s',
  store_id, account_id, account_id, store_id
) AS stmt
FROM sellers
WHERE is_current
  AND store_id <> account_id
ORDER BY account_id;

-- (b) device_tokens —— 同一組帳號，同一個目標 store_id，另一張表。
SELECT format(
  'UPDATE device_tokens SET user_id = %s WHERE user_id = %s AND user_type = ''seller''; -- account_id=%s -> store_id=%s',
  store_id, account_id, account_id, store_id
) AS stmt
FROM sellers
WHERE is_current
  AND store_id <> account_id
ORDER BY account_id;
```

執行前先跑一次 `SELECT count(*) FROM sellers WHERE is_current AND store_id <> account_id;` 確認筆數量級（預期隨時間增長，2026-07-18 後新建帳號都可能落在這格），把兩段輸出各存成一個檔案（例如 `backfill-preferences-2026-08-30.sql` / `backfill-device-tokens-2026-08-30.sql`）供下一步使用與存證。

## 2. 審閱 + 執行（在 `hawkr_notification` 執行，寫入）

人工審閱上一步輸出的語句（抽查幾筆、確認 `store_id`／`account_id` 沒有對調、沒有異常巨大的筆數），兩張表分開跑，各自一個 transaction（互相獨立，一張表核對有異常不影響已經 COMMIT 的另一張）：

```sql
BEGIN;

-- 貼入第 1(a) 步產生的所有 UPDATE 語句（示意）：
-- UPDATE notification_preferences SET user_id = 7001 WHERE user_id = 42 AND user_type = 'seller';
-- UPDATE notification_preferences SET user_id = ... WHERE user_id = ... AND user_type = 'seller';
-- ...

-- 執行完，核對總影響列數是否合理（不必等於 sellers 分歧帳號數——很多分歧帳號
-- 可能從未設定過任何偏好，UPDATE 影響 0 列是正常的，不是錯誤）：
SELECT user_id, user_type, category, enabled FROM notification_preferences WHERE user_type = 'seller' ORDER BY user_id;

COMMIT; -- 核對無誤才 COMMIT；有異常先 ROLLBACK
```

```sql
BEGIN;

-- 貼入第 1(b) 步產生的所有 UPDATE 語句（示意）：
-- UPDATE device_tokens SET user_id = 7001 WHERE user_id = 42 AND user_type = 'seller';
-- ...

-- 同樣核對：不必等於分歧帳號數，很多帳號可能沒有任何已註冊的 token。
SELECT user_id, user_type, platform, is_active FROM device_tokens WHERE user_type = 'seller' ORDER BY user_id;

COMMIT;
```

## 3. Verification（backfill 後）

```sql
-- 兩張表都應為 0 列：不該再有 seller 列的 user_id 剛好等於某個「已分歧」帳號的
-- account_id（也就是還沒搬走的殘留）。把第 1 步存的 account_id 清單整理成
-- IN (...) 貼進來核對（清單通常不大，可人工貼）。
SELECT * FROM notification_preferences
WHERE user_type = 'seller' AND user_id IN (/* 第 1(a) 步輸出的 account_id 清單 */);

SELECT * FROM device_tokens
WHERE user_type = 'seller' AND user_id IN (/* 第 1(b) 步輸出的 account_id 清單 */);
```

`scripts/audit-notification-identity.py` 也已擴充成掃四張表（含 `notification_preferences`），backfill 跑完、code 部署完之後，對 dev/prod 各跑一次應該看到 🔴 `BAD_same_role` 合計歸零（或只剩與本次 backfill 無關的既有違規）——backfill 跑完但 code 還沒部署的窗口內跑這支腳本，會看到暫時性的 🔴（見上方「部署順序」），這是預期中的過渡訊號，不是腳本壞掉。

## 4. Rollback

`account_id` 本身從不改變（只有 `store_id` 因 re-key 而變動），若需要回復，重跑第 1 步查詢並把 `SET` 與 `WHERE` 的欄位對調即可重新產生反向語句；這也是為什麼可以放心先跑 SQL 再部署 code——回滾成本低、可重複執行、不是破壞性操作。

## 已知風險（刻意接受，不做成自動化補償）

- **UNIQUE key 碰撞，兩種來源分開看**：(a) **跟 code 部署競速出來的碰撞**——只要確實依照「部署順序」在 code 部署**之前**執行本 backfill，這個來源的碰撞機率是 0，不是「低」，機制見上方「部署順序」，這裡不重複。(b) **殘留的 TSID 巧合碰撞**（不管順序都存在、可忽略）：兩個不相干帳號的 `store_id`／`account_id` TSID 剛好相等的機率極低，本 repo 其他地方也未對這類理論碰撞加防護。撞到時（不論哪個來源）該筆連帶整個 transaction ROLLBACK（不會半途污染資料），逐筆排除該帳號、記錄於 PR，另案處理。
- **過渡窗口的暫時性故障**：見上方「部署順序」一節，範圍限於「已分歧且尚未跑完 backfill／部署完 code」的帳號、時間窗限於本次 deploy 動作的長度，不是永久性問題。backfill 完成後，新寫入固定使用 canonical store_id。
- **`push_notifications` 去重鍵的身分空間位移**：`send_push.go` 的 `ExistsProcessed(event_type, reference_id, user_id, user_type)` 與 DB 的 `push_notifications_dedup_idx`（`(user_id, user_type, event_type, reference_id)`）都是拿 push identity 當去重鍵的一部分。若舊版 account_id 與新版 store_id 的訊息在部署邊界前後重送（retry／DLQ replay），可能多推播一次；歷史 push record 不回寫，交由 audit 的窗口外 HISTORIC 分級處理。
