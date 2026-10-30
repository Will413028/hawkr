# accountsuspension 模組

## 職責

帳號停權與解除停權的唯一入口（A7-D15）：鎖帳號列、確認帳號沒有店家、呼叫 identity 的停權或解除入口、在同一個 transaction 發事件。moderation 與 legacy `AccountService.SuspendAccount` 都經它。行為規格的唯一來源是 `docs/modules/accountsuspension.md`。

## 擁有的資料

無。停權紀錄在 identity（`identity.account_suspensions`），沒有 schema 與 migration，登記在 `cmd/hawkr/modules.go` 的 `withoutData`。

## 發布的讀取介面

無。讀目前的停權紀錄用 `identity.CurrentSuspension`。

## 模組內部結構

只有根套件：`Suspend`、`Unsuspend`、輸入 `Record`，事件 `Suspended`、`Unsuspended`（`events.go`）。依賴 identity、seller；依賴表的唯一來源是 `internal/arch/modules.go`。

## 鎖順序

identity 帳號列 → buyer → seller，與 `accountdeletion`、seller 開店相同。本模組只用到第一個和最後一個：先 `identity.LockAccount`，再讀 `seller.CurrentStore`（不鎖店家列），之後才寫 identity。不得先讀店家再鎖帳號。守門：`TestSuspendAndAccountDeletionRunConcurrentlyWithoutDeadlock`。

## 不變量

- 擁有店家的帳號回 `FailedPrecondition` 與 `ACCOUNT_HAS_STORE`，什麼都不寫、不發事件。只有這裡檢查，identity 不查店家。守門：`TestSuspendRefusesAnAccountThatHasAStore`。
- 與開店序列化：停權和開店都先鎖帳號列，結果不是「有店且被停權」。守門：`TestSuspendWaitsForAnOpeningStoreAndThenSeesIt`、`TestOpeningWaitsForASuspensionAndThenIsRefused`、`TestConcurrentSuspendAndOpenNeverLeaveASuspendedStore`。
- 在呼叫方的 transaction 內執行，事件在同一個 transaction，呼叫方回滾則紀錄、狀態與事件都回滾。守門：`TestSuspendAndUnsuspendRunInTheCallersTransaction`。
- 已停權的帳號重複停權回傳原本的紀錄，不改理由、不發第二個事件；沒有停權時解除成功且不發事件。守門：`TestSuspendRecordsTheReasonAndPublishesTheEvent`、`TestUnsuspendReleasesAndPublishesTheEventOnlyWhenSuspended`。
- identity 的停權、解除入口只有本模組呼叫。守門：`internal/arch` 的 `TestSuspensionEntriesAreOnlyCalledByAccountsuspension`。

## 不負責

- 停權紀錄、`suspended_at`、停權時刪除該帳號全部 session：identity。
- 受限登入與動作性質宣告表：`internal/httpapi`（`customerEffects`，A7-D18）。
- 停權對店家、未結訂單與撥款的效果：賣家條款定出之前，有店的帳號一律拒絕。
- 誰有權停權、案件與處置：moderation（A7-1d）。通知：A8 消費 `Suspended`、`Unsuspended`。

## 決策紀錄

沿用盤點在 `CARRYOVER.md`。ADR：`docs/adr/2026-10-29-account-suspension-and-moderation.md`（A7-D14 到 A7-D17 與終態觸發）、`docs/adr/2026-09-30-account-deletion-module.md`（同層編排模組的前例）。
