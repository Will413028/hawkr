# identity 模組

## 職責

誰在使用系統：登入的帳號（每個帳號都是買家，賣家是另外申請的角色）與管理員、帳號的角色與狀態。顧客登入與 session 由 Clerk 處理，後台由 Cloudflare Access 保護（ADR `docs/adr/2026-09-27-identity-clerk.md` 含文末修訂）。代行的授權、工作階段、稽核與編排由 delegation 擁有。行為規格的唯一來源是 `docs/modules/identity.md`。

## 擁有的資料

schema `identity`（`db/schema.sql`）：`accounts`（`clerk_user_id` UNIQUE，墓碑也保留）、`account_roles`、`account_suspensions`（停權紀錄）、`admins`（`access_subject` UNIQUE）、`admin_roles`、`admin_role_events`（只追加）、`apple_tokens`。帳號與管理員的主鍵過渡期是 IDENTITY，見 `CARRYOVER.md`。讀取模型：無。

## 發布的讀取介面

無。沒有 `db/pub.sql`，沒有 `pub_*`。

## 模組內部結構

- 根套件組裝 `Module`，保留公開型別別名、事件、錯誤、provider 建構函式與轉呼叫方法。根套件不得 import `store`。
- 根套件的 `Deps` 是組裝 API；`internal/model` 定義公開型別、事件與 provider 介面；各子領域宣告最小 `Deps`。
- `internal/account`：帳號、角色、清單、首次認證建帳與 Clerk 使用者 job。`internal/admin`：管理員查詢。`internal/auth`：token 路由、建帳、認證時建管理員、舊 Google step-up。`internal/applerevoke`：Apple 憑證的連結、換 token、保存的 job 與刪帳時撤銷。
- `internal/provider` 把 `clerk`、`apple`、`oidc`、`seal` 的 client 接成模組公開介面，並提供本機 dev token；`internal/store` 是 sqlc 查詢套件。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- 認證以 token 的 issuer 選 verifier：顧客 token 只認證成帳號，後台 Access/dev admin token 只認證成管理員。守門：`TestAuthenticationErrors`、`TestServicesAreSeparatedByAudience`（httpapi/identity）。規格：`docs/modules/identity.md` §1。
- 同一個 Clerk 使用者同時的第一個請求只建一個帳號，不授予角色。守門：`TestConcurrentFirstAuthenticationsShareAnAccount`；`accounts.clerk_user_id` UNIQUE。規格：§2。
- 停權、刪帳與排入停用或刪除 Clerk 使用者的 job 在同一個 transaction，刪帳留下墓碑並發出 `AccountDeleted`。守門：`TestSuspensionEndsAuthenticationAndBansTheClerkUser`、`TestDeletionLeavesATombstoneAndDeletesTheClerkUser`；`accounts_deleted_check`。規格：§4。
- 停權紀錄與 `suspended_at` 同一個 transaction，未解除的紀錄最多一筆；停權、解除入口只由 `accountsuspension` 呼叫。守門：`TestFailureAfterTheRecordRollsTheRecordBack`、`TestRepeatedSuspendKeepsTheFirstRecord`、arch 的 `TestSuspensionEntriesAreOnlyCalledByAccountsuspension`。規格：§4。
- 帳號生命週期 ACTIVE → PENDING_DELETION → DELETED；等待刪除時 `GrantRole` 回 FailedPrecondition。守門：`TestDeletionPendingRefusesGrantRole`；`accounts_deletion_pending_check`。規格：§4。
- 授予、移除角色與刪帳都先鎖帳號列；`DeleteAccount` 只由 accountdeletion 呼叫。守門：無（只有規格）。規格：§3、§4。
- 不是角色的值（例如 `BUYER`）授予或移除回 InvalidInput。守門：`TestRemoveRole`；`account_roles_role_check`。規格：§3。
- 授予、撤銷管理員角色與寫 `admin_role_events` 在同一個 transaction；事件只追加；撤銷在同一個 transaction 內發出 `AdminRoleRevoked`，授予不發。守門：`TestAdminRoleEventsAreAppendOnly`、`TestRevokePublishesAdminRoleRevokedInTheSameTransaction`。規格：§5。
- `AdminAllows` 在呼叫方的 transaction 內以 `FOR SHARE` 讀角色列，與撤銷互相等待。守門：`TestAdminAllowsReadsRolesInsideTheCallersTransaction`。規格：§5。
- 每個請求都讀角色、不快取；零角色沒有任何權限。守門：`TestOwnerPassesEveryCheckAndRevocationTakesEffectOnTheNextRequest`（httpapi/identity）。規格：§5。
- Apple 憑證只在 ID token 與連結時身分相同才保存；身分不符或帳號已刪除才當場撤銷，暫時性失敗不撤銷；刪帳在同一個 transaction 取走 token 並排入撤銷。守門：`TestExchangeRevokesWhenTheCodeBelongsToSomeoneElse`（applerevoke）、`TestAppleTokenIsStoredReplacedAndRevokedOnDeletion`。規格：§7。

## 不負責

- 刪除帳號的申請、等待未結訂單與參與者的關閉：`accountdeletion`；identity 只提供刪帳號與帳號鎖。
- 買家、賣家的 profile 與業務身分（`buyer_id`、`store_id`）：buyer、seller。identity 只知道帳號有沒有賣家角色，不依賴 buyer 或 seller，也不判斷帳號要不要當買家（買家由 buyer 訂閱 `AccountCreated` 建立）。
- 舊契約的買家位元（`account_type`、`X-Login-Role`）：`internal/legacy/accounttype` 合成。
- 代行的授權、工作階段、稽核與編排：delegation。
- 顧客的登入、登出、session、帳號連結與登入防濫用：Clerk。後台的登入、白名單與 session：Cloudflare Access。cookie、`Authorization` header 與 CSRF：`internal/httpapi`。方法與路由的權限檢查：`internal/httpapi/adminauth`。
- 視角：本模組不認識（ADR `docs/adr/2026-10-07-connect-without-perspective.md`，規格 §6）。
- 推播的 device token：notification（過渡期是 notification-service）。

## 決策紀錄

從 account-service 沿用的每個機制、當時的決定與理由，以及與 account-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-09-27-identity-clerk.md`、`docs/adr/2026-10-04-admin-roles.md`、`docs/adr/2026-09-30-account-created-provisions-buyer.md`、`docs/adr/2026-09-30-account-deletion-module.md`。
