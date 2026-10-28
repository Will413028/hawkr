# identity 模組行為規格

本文是單體 `identity` 模組行為的唯一來源。誰在使用系統：登入的帳號（每個帳號都是買家，賣家是另外申請的角色）與管理員、帳號的角色與狀態。顧客登入與 session 由 Clerk 處理，後台由 Cloudflare Access 保護；決策見 ADR `docs/adr/2026-09-27-identity-clerk.md`（含文末修訂）。代行的授權、工作階段、稽核與編排由 delegation 擁有。

## 1. 認證

- 顧客的登入、session 與帳號連結由 Clerk 處理；後台的登入與白名單由 Cloudflare Access 處理。
- 模組依 token 的 issuer 選 verifier：顧客 Clerk token 只會認證成帳號；後台 Access JWT（或本機 dev admin token）只會認證成管理員。Access 驗 team domain 的 JWKS、application AUD tag、`iss`、`exp`、`nbf`。
- 首次認證建立帳號或管理員。cookie、`Authorization` header 與 CSRF 在 `internal/httpapi`，不在本模組。
- 停權、刪除與角色以本模組的資料為準，每個請求都讀（token 在 Clerk 撤銷之後的效期內仍驗得過）。停權的帳號在下一個請求回 PermissionDenied，已刪除的帳號回 Unauthenticated。

## 2. 帳號建立

- 同一個 Clerk 使用者同時的第一個請求只建一個帳號：`clerk_user_id` UNIQUE 加 `ON CONFLICT DO NOTHING`。
- 建帳不授予任何業務角色。建帳的 transaction 內發布 `AccountCreated`，buyer 模組訂閱並在同一個 transaction 內建立買家（ADR `docs/adr/2026-09-30-account-created-provisions-buyer.md`）。
- `email` 是建帳時 Clerk session 的 email claim，只給後台顯示與搜尋，不是識別，不隨 Clerk 端的變更同步。

## 3. 帳號角色

- 角色只有 `SELLER`，帳號另外申請，由 seller 授予與移除；移除角色不刪帳號。買家不是角色：帳號活著就是買家。
- 不是角色的值（例如 `BUYER`）授予或移除都回 InvalidInput；DB CHECK `account_roles_role_check` 只允許 `SELLER`。
- `account_roles.source_id` 是 seller-service 以非同步方式移除 SELLER 時的柵欄：移除帶來源，目前的授予屬於較新的來源就不移除。
  - 授予帶來源（該次開店的 `store_id`）；帳號已有同角色時 `source_id` 改指到這次授予帶的值，值相同則沒有變化（回傳 0 列）。授予不帶來源也會把既有的來源改成 NULL，呼叫方永遠帶當下的來源。
  - 移除的條件是目前的授予沒有來源、或來源等於呼叫帶的來源，比對在同一句 `DELETE` 內。被柵欄擋下時（有角色但沒有移除）呼叫端視為成功，帳號保留。
- 授予、移除角色與刪帳號都先鎖住帳號列。

## 4. 停權、刪除與生命週期

- 停權、刪帳與排入停用、刪除 Clerk 使用者的 job 在同一個 transaction。停權冪等：已經停權時不再排 job。
- 刪帳號留下墓碑與 `clerk_user_id`，同一個 Clerk 使用者不會再建一個帳號；`email` 清為 NULL（`accounts_deleted_check`）。刪帳同時發出 `AccountDeleted`。
- 帳號的生命週期是 ACTIVE → PENDING_DELETION → DELETED（`Account.Lifecycle`；欄位 `deletion_pending_at`、`deleted_at`）。
  - `BeginDeletion` 只由 accountdeletion 在受理會等待未結訂單的申請時呼叫。
  - 等待刪除時仍可登入，但 `GrantRole` 回 FailedPrecondition。
  - `DeleteAccount` 清掉等待的狀態；`accounts_deletion_pending_check` 保證已刪除的帳號沒有等待狀態。
  - 狀態在 identity，流程在 accountdeletion（ADR `docs/adr/2026-09-30-account-deletion-module.md` 的修訂）。
- 跨模組的刪帳流程（accountdeletion 刪帳並關閉買家）在自己的 transaction 內以 `LockAccount` 取同一把帳號列鎖，再讀角色或呼叫參與者。`DeleteAccount` 只由 accountdeletion 呼叫，其他入口不得直接刪帳號。
- Clerk 停擺時新登入失敗，已登入的使用者在下一次換 token（最多約 60 秒）之後也失效。

## 5. 管理員與管理員角色

- 後台只根據驗證通過的 Access JWT（或本機 dev admin token）建立管理員，以 `sub` 對應（`admins.access_subject` UNIQUE）；Access 的 email 白名單是開通的唯一來源，`email` 只供顯示。
- 誰是管理員由 Access 決定，角色由 `admin_roles` 決定。角色目前只有 OWNER，擁有 `platform/adminperm` 目錄裡的全部權限（由程式計算，不逐條列）；零角色等於沒有任何權限。
- 每個請求認證管理員時一併讀角色（`Principal.AdminRoles`，與帳號的 `Roles` 分開），不快取。
- 授予與撤銷只有 CLI `hawkr admin-role grant|revoke`（`GrantAdminRole`、`RevokeAdminRole`）。方法與 `/api/files/admin/…` 路由的權限檢查在 `internal/httpapi/adminauth`，本模組只提供角色。
- 授予、撤銷角色與寫入 `admin_role_events` 在同一個 transaction。授予的對象必須是已存在的管理員（Access 白名單的人登入一次之後），不預建。重複授予或撤銷沒有的角色回錯誤、不寫事件。
- 事件只追加（queries 只有 INSERT 與 SELECT）：記 action（GRANT、REVOKE）、必填理由（規則同 `platform/auditreason`）、source（目前只有 `CLI`）、`db_user`（`session_user`）與 `os_user`（取不到時 NULL）；後兩者未經驗證、只供參考，沒有自填操作者的參數。
- 撤銷在同一個 transaction 內發出 `AdminRoleRevoked`；delegation 訂閱，剩下的角色不再允許開始代行時撤銷該管理員進行中的代行，訂閱者失敗時撤銷回滾。授予不發此事件。
- 撤掉最後一個 OWNER 是允許的；`AdminRoleChange.HolderRemains` 讓 CLI 警告。
- `AdminAllows` 給 delegation 在自己的 transaction 內檢查權限：以 `FOR SHARE` 讀角色列，與撤銷的 DELETE 互相等待；不在 transaction 內呼叫回錯誤。
- 後台按鈕登出的 Connect handler 直接呼叫 delegation 撤銷進行中的代行；Access 在其他地方登出或 session 到期不會觸發這個呼叫。
- 管理員沒有停用的功能（後台沒有管理員管理的畫面）。停用單體內的權限是撤銷他的角色（`admin-role revoke`，同時撤銷進行中的代行）；離職處置另須從 Access 白名單移除、在 Zero Trust 撤銷該使用者的 Access sessions，並撤回 delegation 的代行資格。`admins` 的列保留，供代行的歷史 ID 與角色事件參照。
- 角色只有 OWNER，授予只有 CLI、沒有經過驗證的操作者。

## 6. 視角

- Connect 沒有視角（ADR `docs/adr/2026-10-07-connect-without-perspective.md`）；「正在看哪個工作區」是 client 的 UI 狀態，本模組不認識視角。
- `X-Active-Role` 只有舊 REST 送：`internal/legacy/gatewayproxy` 讀它，以 `internal/legacy/accounttype.OfLoginRole` 推導 `X-Login-Role` 給舊 gateway。
- 舊契約的 `account_type` bitmask（買家 1、賣家 2、管理員 4）只在 `internal/legacy/accounttype` 合成：存活的帳號一定有買家位元，有 `SELLER` 時加賣家位元；`X-Account-Type`、`X-Login-Role` 與 legacy `AccountService` 回傳的 `account_type` 都來自它。單體內只有角色。
- legacy `AccountService.RemoveRole`（`internal/legacy/accountservice`，給還沒搬的服務）只移除 `SELLER`：`ROLE_BUYER` 與未指定的角色回 InvalidInput；`source_id` 為 0 表示沒有帶來源；移除不刪帳號，`account_fully_deleted` 恆為 false。

## 7. Sign in with Apple 授權撤銷

- App 原生登入後以 `LinkAppleCredential` 交出 identity token 與 authorization code。驗證 token（簽章、issuer、audience）後，把 `sub`、audience 與加密的 code 排入 job，請求內不呼叫 Apple。
- job 換 refresh token。Apple 簽發的 ID token 與連結時的身分相同，才以 AAD（表、欄、帳號 ID）加密存入 `identity.apple_tokens`（每帳號一筆，重新連結取代）。身分不符或帳號已刪除，當場撤銷換得的 token。
- 暫時性失敗（驗證 ID token 連不到公鑰、保存失敗）不撤銷，改排 `identity.save_apple_token` job 帶加密的 token 回應重試；最後一次仍失敗就記 error 並放棄，下一次原生登入再交。
- 每種失敗都以 error 等級記 log（原因與帳號 ID，不含 code 與 token）。不呼叫 Clerk Backend API 核對 `sub`。
- 所有刪除路徑都經 `DeleteAccount`；它在同一個 transaction 內取走 token、刪列並排入 `identity.revoke_apple_token`。沒有保存的 token 就沒有東西可撤銷，刪帳不受影響。
- 部署的環境 Apple 設定必填（缺任何一個啟動失敗）；本機開發可不設，此時 `LinkAppleCredential` 回 unavailable。啟動時 `CheckAppleKeys` 檢查保存的密文與排隊中 job 參數的密文用到的金鑰都在 `IDENTITY_TOKEN_KEYS`。
- web 登入的 Apple 使用者沒有保存的 token（授權碼在 Clerk），website 維持人工提示。

## 8. 本機開發與測試登入（DevAuth）

- 本機的 RSA 金鑰簽 dev token，使用者是 `dev_` 開頭的 ID，不連外部服務（`hawkr dev-token`）。顧客 token 為 Clerk 形狀，後台 token 為 Access 形狀。
- serve 設了 `DEV_AUTH_KEY_FILE` 才接受；不能與正式 Clerk 或 Access 設定同時使用。

## 9. 資料

schema `identity`（`db/schema.sql`）：

- `accounts`：登入的 principal。`clerk_user_id` 是顧客的 Clerk application 的使用者 ID（UNIQUE，墓碑也保留）。主鍵目前是 IDENTITY（過渡期，對外可列舉）。
- `account_roles`：`(account_id, role)`，role 只允許 `SELLER`。
- `admins`：管理員，與帳號是不同的 principal；`access_subject` 是 Access JWT 的 `sub`（UNIQUE），`email` 是首次登入時的顯示快照。
- `admin_roles`：`(admin_id, role)`，role 的 CHECK 只允許 `OWNER`。
- `admin_role_events`：角色授予與撤銷的歷史，只追加；`admin_role_events_*_check` 限定 role、action、reason（1 到 1000 字且不能全是空白）與 source。
- `apple_tokens`：保存的 Apple refresh token 密文，每帳號一筆。
- 代行資格與行為參數由 delegation 擁有。

外部 provider 的 client 在 `internal/`：`clerk`（顧客 session token 的驗證、Backend API、dev token）、`oidc`（Google、Apple 的 ID token；Clerk 與 Access 的驗證也用它）、`apple`（token 的交換與撤銷）、`seal`（AES-GCM，加密排入 job 與保存的 Apple 憑證）。

## 10. 已知限制

- 帳號與管理員的主鍵是 IDENTITY，對外可列舉；舊 Java 服務仍存成 `BIGINT`。代行 session 的過渡 ID 記在 delegation。
- 帳號的 email 是建帳時的快照，在 Clerk 改了 email 不會同步；後台的清單以它顯示與搜尋。
- 賣家 CRM 以手機找顧客（legacy 的 `GetAccountByPhone`）一律回「找不到」：帳號沒有手機。
- 外部 Access 登出不會自動撤銷代行。
- 代行資格與撤回時的工作階段撤銷由 delegation 管理（設計書 56 §8.5.3）。
