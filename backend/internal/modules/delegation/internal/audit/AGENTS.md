# delegation/internal/audit

代行操作的稽核：兩階段 intent／confirm（舊 gateway）、代行 Connect 的單段 `SUCCEEDED` 與 `FAILED`、排程到點發布的稽核與回看。

## 不變量

- 到點重驗與 `StartDelegation` 共用 `internal/session/authority.go` 的授權述詞，全部不鎖的讀取，session 只擋 `REVOKED`；單段 `SUCCEEDED` 稽核由 `OnPostPublished` 在發布的 transaction 內寫，冪等鍵 `schedule:<排程 ID>`。守門：`TestAuthorityLostByDirectDataChangeCancelsAtRun`、`TestScheduledPublishOfAnEndedSessionPublishesWithOneSucceededAudit`。規格：§3、§6。
- 共用代行元件（`DelegatedWrite`、`DelegatedRead`、`AuthorizeDelegated`，不含資源專屬程式）：同 transaction 檢查能力、寫單段 `SUCCEEDED`（`request_id` 即冪等鍵）、呼叫呼叫方的寫入；拒絕與業務失敗另開 transaction 寫 `FAILED`，`failure_reason` 是拒絕的代碼、`ErrorReason` 或 `INTERNAL`（`model.FailureReasonOf`；`biz_code` 只有 gateway confirm 寫，A10 刪除）。守門：`TestEveryDelegatedPostWriteLeavesOneSucceededAuditRow`、`TestSocialFailureLeavesNoSucceededRowButAFailedRow`、`TestFailureReasonOfClassifiesTheCause`、`TestFailureReasonCheckRejectsUnknownCodes`。規格：§6。
- 稽核 intent 是寫入前置；confirm 只從 PENDING 轉移一次，成功且需通知時在同 transaction 發事件。守門：`TestAuditIntentAndConfirm`、`TestConfirmRollsBackWhenTheEventIsNotQueued`。規格：§6。
