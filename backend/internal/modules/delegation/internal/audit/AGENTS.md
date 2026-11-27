# delegation/internal/audit

代行操作的稽核：兩階段 intent／confirm（舊 gateway）、代行 Connect 的單段 `SUCCEEDED` 與 `FAILED`、排程到點發布的稽核與回看。

## 不變量

- 到點重驗與 `StartDelegation` 共用 `internal/session/authority.go` 的授權述詞，全部不鎖的讀取，session 只擋 `REVOKED`；單段 `SUCCEEDED` 稽核由 `OnPostPublished` 在發布的 transaction 內寫，冪等鍵 `schedule:<排程 ID>`。守門：`TestAuthorityLostByDirectDataChangeCancelsAtRun`、`TestScheduledPublishOfAnEndedSessionPublishesWithOneSucceededAudit`。規格：§3、§6。
- 共用代行元件（`DelegatedWrite`、`DelegatedRead`、`AuthorizeDelegated`；A7-D26、A7-D27）不開 transaction：呼叫資源模組前以 `LookupOperation` 查重送；資源模組在自己的 transaction 內發帶代行者（明確參數）的一般貼文事件，訂閱者（`OnPostCreated` 等）依事件種類決定動作與能力、呼叫 `RecordOperation` 檢查能力、寫單段 `SUCCEEDED`（`request_id` 即冪等鍵，撞鍵回 `ErrOperationKeyTaken` 讓 transaction rollback）。規格：§6。
- 上一條的守門：`TestDelegatedWriteOpensNoTransactionAroundTheResourceWrite`、`TestDelegatedWriteReplayReturnsTheFirstResultWhateverTheStoreState`、`TestPostWriteSubscribersMapEachEventKindToItsAuditActionAndCapability`、`TestPublishRequiredFailsClosedWithoutTheNamedHandler`。
- 代行寫入的成功與失敗列：每個寫入恰好一列 `SUCCEEDED`；拒絕與業務失敗另開 transaction 寫 `FAILED`，`failure_reason` 是拒絕的代碼、`ErrorReason` 或 `INTERNAL`（`model.FailureReasonOf`；`biz_code` 只有 gateway confirm 寫，A10 刪除）。守門：`TestEveryDelegatedPostWriteLeavesOneSucceededAuditRow`、`TestSocialFailureLeavesNoSucceededRowButAFailedRow`、`TestConcurrentReplaysConvergeOnOneWriteAndOneAuditRow`、`TestFailureReasonOfClassifiesTheCause`、`TestFailureReasonCheckRejectsUnknownCodes`。規格：§6。
- 稽核 intent 是寫入前置；confirm 只從 PENDING 轉移一次，成功且需通知時在同 transaction 發事件。守門：`TestAuditIntentAndConfirm`、`TestConfirmRollsBackWhenTheEventIsNotQueued`。規格：§6。
- 商品的代行寫入同貼文（`OnProductCreated` 等，能力與動作由 `ProductWriteFor` 一處決定）；批次下架（D29）是一個 `ProductsUnpublished` 事件、一列 `PRODUCT_BATCH_UNPUBLISH`（`resource_id` 為 NULL、名稱是商品數）加 `admin_operation_audit_log_items` 的逐項記錄（版本前後）、一則賣家通知；`LookupOperation` 讀回批次列時帶回商品清單，重送回原結果。守門：`TestBatchUnpublishWritesOneAuditRowWithItsItemsAndOneNotification`、`TestBatchUnpublishConflictWritesNothingAndTheRetryRecordsOneRow`、`TestMissingProductCapabilityIsRejectedForEveryWriteAndLeavesFailedRows`。

