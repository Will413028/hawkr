# accountsuspension 模組行為規格

本文是單體 `accountsuspension` 模組行為的唯一來源。帳號停權與解除停權的唯一入口：鎖帳號列、確認帳號沒有店家、呼叫 identity、發事件。決策見 ADR `docs/adr/2026-10-29-account-suspension-and-moderation.md`（A7-D15、A7-D16、A7-D17）。

## 1. 範圍與公開介面

- 根套件公開 `Module`：`Suspend(ctx, accountID, Record) (Suspension, error)`、`Unsuspend(ctx, accountID, reason) (released bool, error)`；輸入 `Record{Category, Reason, SourceRef}`；事件 `Suspended`、`Unsuspended`（`events.go`）。`Suspension` 是 `identity.AccountSuspension` 的別名。
- 依賴 identity、seller，沒有模組依賴它（目前只有 `internal/legacy/accountservice` 與之後的 moderation 呼叫）；依賴圖的唯一來源是 `internal/arch/modules.go`。
- 沒有自己的資料：停權紀錄在 `identity.account_suspensions`（規格見 `docs/modules/identity.md` §4）。登記在 `cmd/hawkr/modules.go` 的 `withoutData`。
- identity 的 `SuspendAccount`、`UnsuspendAccount` 只允許本模組呼叫，`internal/arch` 的 `TestSuspensionEntriesAreOnlyCalledByAccountsuspension` 掃非測試程式的呼叫者；新增呼叫者要改 `arch.SuspensionEntries`，那等於改架構。

## 2. Suspend

- 在呼叫方的 transaction 內執行（`db.InTx` 可重入，沒有就自己開一個）。順序：`identity.LockAccount` → `seller.CurrentStore` → `identity.SuspendAccount` → 新建紀錄時發 `Suspended`。
- 帳號擁有目前的店家（`CurrentStore` 回傳，停權中的店家也算）：回 `FailedPrecondition` 與 `ErrorReason` `ACCOUNT_HAS_STORE`，不寫紀錄、不發事件。沒有店家（NotFound）才繼續；其他錯誤原樣回傳。
- 類別必須是 `platform/policy.Category` 的值，空字串與不認得的值回 InvalidInput；說明選填（規則同 `platform/auditreason`，前後空白去掉、最多 1000 字）；來源參照選填，去掉前後空白後 1 到 200 字，identity 不解讀它。
- 帳號不存在或已刪除回 NotFound。已經停權的帳號回傳原本的紀錄，不改理由、不寫第二筆、不發事件（有店的已停權帳號仍回 `ACCOUNT_HAS_STORE`：店家檢查在前）。
- 事件 `Suspended{AccountID, SuspensionID, Category, SuspendedAt}` 在同一個 transaction 內發出，呼叫方回滾時一起回滾。A8 的通知消費它；本模組不訂閱、不接通知。

## 3. Unsuspend

- 同樣在呼叫方的 transaction 內，先 `identity.LockAccount` 再 `identity.UnsuspendAccount`。reason 必填（`platform/auditreason`），空白回 InvalidInput。
- 帳號沒有停權時成功、回傳 `released=false`、不發事件；解除時發 `Unsuspended{AccountID, SuspensionID, Category, ReleasedAt}`。
- 解除只清資料庫的停權狀態並在紀錄記解除時間與原因。停權不停用 Clerk 使用者，所以解除後下一個請求就恢復。

## 4. 鎖與序列化

- 鎖順序是 identity 帳號列 → buyer → seller，與 `accountdeletion`、seller 開店相同。本模組只鎖帳號列，並以不鎖定的讀取看 seller。
- 與開店序列化：seller 開店在同一個 transaction 持有 `identity.LockAccount`，並在前置條件拒絕被 identity 停權的帳號（`OpeningState.AccountSuspended`）。停權先拿到鎖則開店被擋；開店先拿到鎖則停權等它 commit，看到店家而回 `ACCOUNT_HAS_STORE`。結果不會是「有店且被停權」。
- 與 `accountdeletion` 並行：兩邊都先鎖帳號列，不會互等；刪帳在鎖內解除未解除的停權紀錄（`ACCOUNT_DELETED`），所以已刪除的帳號沒有未解除的紀錄。

## 5. 終態

- 有店一律拒絕是暫時的：賣家條款定出帳號停權對店、訂單、撥款的效果後，改成連帶對店家建立措施。
- A7 只有永久停權（A7-D16）；買家條款定出處罰階梯時，停權紀錄加 `ends_at`，認證讀時判斷到期。
