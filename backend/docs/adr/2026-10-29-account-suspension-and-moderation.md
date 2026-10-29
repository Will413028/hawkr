# 帳號停權的單一入口、受限登入與檢舉案件粒度

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-10-29（Will 決定） |
| 影響範圍 | `identity`（停權紀錄與解除入口）、新模組 `accountsuspension`、之後的 `moderation`、`internal/httpapi/rpc` 的認證與允許清單、`internal/legacy/accountservice` |
| 取代 | `2026-09-18-content-moderation-consolidation.md` 的 D1（沿用 `reports`／`moderation_actions` 的 schema）與 D3（`account-service` 新增 `SuspendAccount`、借 Clerk 停用使用者）；D2、D4 不變 |
| 修訂 | `2026-09-27-identity-clerk.md`（停權不再停用 Clerk 使用者，見該文末〈修訂：受限登入〉） |
| 計畫 | `docs/plans/2026-10-18-a7-social.md` 的 A7-D14 到 A7-D17、步驟 A7-1d′、A7-1d″、A7-1d；決定簡報 `docs/plans/2026-10-18-a7-social/a7-1d-decision-brief.md` |

## 脈絡

2026-09-18 的收斂 ADR 沿用舊的 `reports`／`moderation_actions` schema，並讓 `account-service` 新增 `SuspendAccount`。搬進單體後三件事不成立：

- 處分紀錄要歸執行效果的模組（單體 10e R12，A7-D9）。現在 `identity.SuspendAccount` 的理由只寫 log，`identity.accounts` 只有 `suspended_at`，被停權者與營運都讀不到理由。
- `identity` 是葉節點，不能 import `seller` 查帳號有沒有店；店家帳號被停權會讓店無人可登入，而 seller 的措施、曝光與未結訂單都不知道。
- 停權即 `PermissionDenied` 加 Clerk ban，被停權者讀不到理由，與「作者讀得到理由」（A7-D9）矛盾。

## 決策

- **A7-D14 案件粒度**：每個對象 `(target_kind, target_id)` 至多一個進行中案件，同對象的檢舉掛進該案，結案把決定寫進各檢舉的 outcome，不依類別分案；處置冪等＝案件 open → closed 的狀態轉移加 `version`。結案後的新檢舉開新案件。去重只比對還沒有結果的檢舉；每日上限 20 在同 transaction 的計數列原子 upsert 內強制；檢舉人與買家對象用 `buyer_id` 單一鍵。
- **A7-D15 停權的單一入口**：新編排模組 `accountsuspension`（與 `accountdeletion` 同層，沒有自己的資料）是停權與解除的唯一入口，「擁有商店的帳號不能被停權」只在這裡（`ErrorReason` `ACCOUNT_HAS_STORE`）；identity 的停權、解除入口只被它呼叫，`internal/arch` 掃非測試程式的呼叫者。它在鎖住帳號列之後讀店家，與開店（同一個 transaction 持有同一把鎖）序列化；開店在同一份狀態裡拒絕被停權的帳號。
- **A7-D16 只有永久停權與手動解除**：identity 的停權紀錄與 social 的處分紀錄都沒有 `ends_at`，停權型別只有永久。
- **A7-D17 受限登入**：停權不再停用 Clerk 使用者；被停權帳號可登入，能做什麼依動作的性質（A7-D18）：`internal/httpapi` 的宣告表把每個顧客 procedure 標為讀取、寫入或自助，`withSession` 執行（公開方法以訪客執行，讀取與自助放行，寫入回 `ACCOUNT_SUSPENDED`；登出是 client 端的 Clerk）；identity 的認證結果帶停權狀態。停權與解除時 `accountsuspension` 在同一個 transaction 發事件（含理由類別），通知由 A8 消費。

## 終態與觸發

| 現在的做法 | 終態 | 觸發 |
|---|---|---|
| 有店的帳號一律拒絕停權（`ACCOUNT_HAS_STORE`） | 停權連帶對店家建立措施，處理未結訂單與撥款 | 賣家條款定出帳號停權對店家、訂單、撥款的效果 |
| 只有永久停權與手動解除 | 停權紀錄加 `ends_at`，認證讀時判斷到期，排程 job 寫到期解除紀錄並發事件 | 買家條款定出處罰階梯 |
| 允許清單不含申訴 | 申訴加進允許清單 | 申訴流程的產品決定 |
| 依檢舉數不調優先度 | 依檢舉數或觸及量提高案件優先度 | 單日檢舉量讓 72 小時期限逾期，或出現病毒式散播的違規內容 |

## 步驟 A7-1d′ 的事實

- 停權紀錄表 `identity.account_suspensions`：型別 `PERMANENT`、理由類別（`platform/policy.Category` 的全部值）、說明、不透明的來源參照、停權時間、解除時間與解除原因；同一帳號未解除的紀錄最多一筆（partial UNIQUE）。與 `accounts.suspended_at` 在同一個 transaction 寫入；解除入口清 `suspended_at` 並記在紀錄上。
- 帳號刪除時未解除的紀錄以 `release_reason = 'ACCOUNT_DELETED'` 解除（刪除的帳號不再是停權狀態，`accounts_deleted_check`）。
- A7-1d″ 已移除 ban job，解除後下一個請求就恢復；先前被停用的開發用 Clerk 測試帳號由 Will 手動解除。
- 這一步 `seller` 開店的前置條件加上 identity 的帳號停權（`OpeningSuspended`），否則停權與開店並行時，停權先 commit 的帳號仍可開店。
