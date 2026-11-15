# delegation 模組

## 職責

代行營運的授權、工作階段、操作稽核與管理員代行資格，以及代行 Connect 共用的稽核元件；舊 gateway 經 `internal/legacy/delegationservice` 呼叫本模組。行為規格的唯一來源是 `backend/docs/modules/delegation.md`（業務依據：設計書 56）。

## 擁有的資料

schema `delegation`（`db/schema.sql`）五張表：`admin_eligibilities`（代行資格）、`delegated_operation_sessions`（工作階段，主鍵是 bigint IDENTITY）、`seller_delegation_authorizations`（每店一列的授權）、`delegation_authorization_audit_logs`（授權治理歷史，只追加）、`admin_operation_audit_logs`（操作稽核）。欄位與語意見 `backend/docs/modules/delegation.md` §2。只存其他模組的 ID，不下跨 schema FK。讀取模型：無。

## 發布的讀取介面

無。沒有 `db/pub.sql`，所以沒有 `pub_*`。

## 模組內部結構

- 根套件只組裝 `Module`、轉呼叫、公開 `internal/model` 的型別別名與事件；整合測試在根套件，只經公開 API。
- 根套件的 `Deps` 是組裝 API，各子領域宣告最小 `Deps`。`internal/session` 管工作階段的寫入、狀態、授權述詞與管理員登出結束；`internal/authorization` 管授權與治理歷史，撤回時經 session 撤銷；`internal/audit` 管稽核與回看，經 session 判斷工作階段是否可用。
- `internal/model` 放公開型別及不碰 I/O 的規則（含 TTL 等常數）；`internal/store` 是 sqlc 生成的資料存取葉節點。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- credential 與 handoff code 只以 SHA-256 存，handoff 以一條 CAS 兌換一次，兩者不進 log。守門：`TestHandoffAndOperationCredentialsUseIndependentRandomTokens`、`TestStartDelegationReplay`。規格：`backend/docs/modules/delegation.md` §3。
- 管理員同時只代行一家、店家同時只被一位管理員代行。守門：DB 約束 `uidx_delegated_operation_sessions_active_actor`、`uidx_delegated_operation_sessions_active_store`。規格：§3。
- 狀態只儲存 ACTIVE、REVOKED（儲存的代碼一律 UPPER_SNAKE，ADR 2026-09-25-codemeta-retirement D3；舊 gRPC 與事件仍傳小寫，轉換在 `internal/legacy/delegationservice/wire.go`；守門：`TestStoredCodesInChecksAreUpperSnake`），到期在讀取時推導；輪詢不延長閒置期限。守門：`TestDelegationStatusFollowsTheOrderedRules`、`TestIdleTimeout`。規格：§3。
- 能力是封閉的九項，拒絕履約、退款與金流。守門：`TestCapabilitiesAreTheClosedNine`、DB CHECK `delegated_operation_sessions_capabilities_check`。規格：§3。
- 店家層級授權列一旦存在，即遮蔽全域列。守門：`TestPreflightDenials`。規格：§4。
- 授權寫入與治理歷史同 transaction；撤回同時撤銷該店進行中的工作階段。守門：`TestAuthorizationAndItsHistoryCommitTogether`、`TestRevokingTheAuthorizationEndsRunningSessions`。規格：§4。
- 撤回代行資格與撤銷管理員角色在同一個 transaction 撤銷進行中的 session（固定值 `RevokeReason*`）；後台登出是「結束」（`EndDelegationsOfAdmin`，`expires_at` 夾值同 `EndDelegationSession`，憑證立刻失效），不寫 `REVOKED`、不取消排程。守門：`TestDisablingAdminEligibilityRevokesSessionsAtomically`、`TestAdminSignOutEndsItsDelegations`、`TestRoleRevocationAndDelegationRevocationRollBackTogether`。規格：§3「撤銷」。
- 排程發布（social 擁有排程）的授權失效有四個觸發點：撤回店家授權、撤回代行資格、撤銷管理員角色、治理撤銷單一 session；同 transaction 內一律「先取消排程、再鎖或改 delegation 的列」（social 在 delegation 之前鎖）。守門：`TestTriggersCancelSchedulesIncludingEndedSessions`、`TestRevokeAndScheduledJobDoNotDeadlock`、`platform/lockorder`。規格：§3、§6。
- 第五個觸發點（A7-D25）：seller 的 `StoreSuspended`、`StoreClosed` 同 transaction 結束該店進行中的 session（`EndDelegationsOfStore`，同登出的「結束」，不取消排程）；請求時發現店家不可代行只拒絕。守門：`TestStoreSuspendedAndClosedEndThatStoresSessions`、`TestStoreNotDelegableAtRequestTimeRejectsWithoutRevoking`。規格：§3。
- 稽核（兩階段 intent／confirm、代行 Connect 的單段 `SUCCEEDED` 與 `FAILED`、排程發布的重驗與稽核）的不變量在 `internal/audit/AGENTS.md`。
- 開始代行要有 `delegation.sessions.create` 權限。守門：`TestStartDelegationRequiresTheCreatePermission`。規格：§3。
- 店家狀態只經 seller 的公開 API 讀取，`Deps.Seller` 必填，句子由 seller 提供。`Deps.Social` 必填（取消排程）。守門：`TestPreflightDenials`。規格：§5。

## 不負責

- 管理員身分與 Access 驗證：identity。identity 不 import delegation。
- 店家狀態、商品、貼文、訂單等業務資料：各資料擁有者；本模組只保存必要快照，店家狀態在需要時向 seller 讀取。
- HTTP 認證、限流與錯誤映射：`internal/httpapi`；舊契約映射在 `internal/legacy/delegationservice`。

## 決策紀錄

從 account-service 沿用的每個機制、當時的決定與理由，以及與 account-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-09-27-delegation-module.md`（含文末修訂：資格歸屬、登出與憑證核發）、`docs/adr/2026-09-25-row-identity-by-data-kind.md`（session ID 的型別）、`docs/adr/2026-10-04-admin-roles.md`（撤銷角色時撤銷進行中的代行）。
