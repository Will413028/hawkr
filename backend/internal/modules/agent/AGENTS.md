# agent 模組

## 職責

沒有帳號的供應商從採購詢價（RFQ）邀請信進來查看詢價、寫草稿、正式送出時的存取模型：邀請、兌換、session、撤銷與清理。給對外入口（`internal/httpapi/agent`）、notification 的邀請信與採購使用。行為規格的唯一來源是 `docs/modules/agent.md`。

## 擁有的資料

schema `agent`（`db/schema.sql`）：`vendor_portal_invites`（每個 RFQ 收件人一張；`invited_email` 是模組唯一的個資）、`vendor_portal_sessions`（`invite_id` FK，`ON DELETE CASCADE`）。欄位見規格 §9。讀取模型：無。

## 發布的讀取介面

無。沒有 `pub_*`。對外是 Go 公開 API：`EnsureInvite`、`MarkInviteDelivered`、`ExchangeInvite`、`PreviewInvite`、`ValidateSession`、`VerifySession`、`OnProspectArchived`、`OnProspectMerged`，形狀與錯誤見規格 §2 至 §6。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與錯誤，加上 `ports.go`（`Prospects`，由 `*seller.Module` 實作）。
- `internal/model` 放對外型別、代碼值、期限常數、錯誤與 email 的純函式；`internal/vendorportal` 是子領域（邀請、兌換、session、撤銷、清理與排程）；`internal/store` 是 sqlc 的查詢套件。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加 use case 或子領域時同步修改圖。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。
- 事件的接線在 `cmd/hawkr/agent_events.go`；模組本身不發事件。

## 不變量

- 存取範圍是一張邀請：session 只屬於一張邀請，兌換不撤銷別張邀請的 session。守門：`TestExchangeCapsSessionsPerInviteAndLeavesOtherInvitesAlone`、`sessions.invite_id`。規格：§3、§4。
- 邀請 secret 與 session token 只存 SHA-256，明文只出現一次。守門：DB CHECK 長度 32、UNIQUE；`TestEnsureInviteCreatesTheInviteAndStoresOnlyTheHash`。規格：§5。
- 不可用的 secret 或 session 一律不分原因：兌換與預覽同一個 NotFound，`ValidateSession` 同一個 Unauthenticated。守門：`TestExchangeAndPreviewShareOneOpaqueNotFound`、`TestValidateSessionRejectsEveryUnusableStateWithOneError`。規格：§3、§4。
- session 有效的條件只在 `ValidateSession`、`VerifySession` 的 SQL 述詞一處，且不超過邀請的到期。守門：`TestSessionNeverOutlivesTheInvite`、`TestValidateSessionFailsWhenTheInviteExpires`。規格：§4。
- `VerifySession` 是 CAS，比對邀請寄出時的 email 快照；驗證紀錄三欄同有同無。守門：`TestVerifySessionComparesTheNormalizedEmail`、`TestVerificationUsesTheSnapshotNotTheCurrentProspectEmail`、DB CHECK。規格：§4。
- 撤銷不可逆，撤銷時間與原因同有同無。守門：`TestProspectArchivedRevokesItsInvitesAndSessions`、DB CHECK。規格：§6。
- 封存之後沒有有效的邀請：Ensure 只替進行中的 prospect 建立或換 secret，並與封存序列化。守門：`TestEnsureInviteRequiresAnActiveProspect`、`TestProspectArchiveAndMergeRevokeVendorPortalAccess`（cmd/hawkr）。規格：§6。
- 一張邀請屬於一個 prospect，合併是唯一改掛的路徑且不撤銷。守門：`TestEnsureInviteRejectsAnotherProspectsRecipient`、`TestProspectMergedRepointsTheSourceInvitesToTheWinner`。規格：§2、§6。
- Mark 只認目前的 secret。守門：`TestMarkInviteDeliveredIgnoresASupersededSecret`、`TestMarkInviteDeliveredRacesWithRotation`。規格：§2。
- 撤銷 handler 在發布方的 transaction 內，在外面呼叫時拒絕。守門：`TestRevocationHandlersRequireATransactionAndRollBackWithIt`。規格：§6。
- 一張邀請最多 5 個有效 session，不擠掉較高保證的 session。守門：`TestExchangeOverCapRevokesTheOldestUnverifiedFirst`、`TestExchangeOverCapKeepsTheNewSessionAndRevokesTheOldestVerifiedWhenAllAreVerified`、`TestConcurrentExchangeKeepsTheCap`。規格：§4。
- 驗證紀錄只用於授權，`SessionInfo` 交出 `SessionID` 與 `Verification`。守門：`TestSessionInfoCarriesTheVerificationEvidence`。規格：§7。
- `EnsureInvite` 的 `ExpiresAt` 晚於現在、不超過 180 天。守門：`TestEnsureInviteValidatesExpiry`。規格：§2。
- email 隨邀請到期或撤銷滿 90 天後刪除。守門：`TestPurgeDeletesOnlyWhatIsPastRetention`。規格：§8。
- 代碼值的 Go 集合與 CHECK 一致。守門：`TestChecksMatchGo`、`TestDatabaseRejectsInconsistentRows`。規格：§9。

## 不負責

- 對外的 Connect API、cookie、`Idempotency-Key`、兌換與驗證的限流、Google token 的驗證（經 identity）、正式送出與撤回的「要求已驗證 session」檢查：對外入口 `internal/httpapi/agent`。
- notification 呼叫的 gRPC adapter：`internal/legacy/sellerserver/invites.go`（對應模組搬進單體後刪除）。
- email OTP 與寄信：notification。
- RFQ 的內容、草稿、送出、`rfq_recipient_id` 的存在與歸屬、買家取消發布時撤銷邀請：ai-service（採購搬進單體後由採購負責）。
- prospect 的資料與 email：seller（`ProspectContact`）。

## 決策紀錄

從 seller-service 沿用的每個機制、當時的決定與理由，以及與 seller-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-10-03-vendor-portal-invite-scoped-access.md`。
