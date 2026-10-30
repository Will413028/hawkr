# identity 模組行為規格

本文是單體 `identity` 模組行為的唯一來源。誰在使用系統：登入的帳號（每個帳號都是買家，賣家是另外申請的角色）與管理員、帳號的角色與狀態。顧客以 Google、LINE 聯合登入，OAuth 與 session 由本模組自己做，後台由 Cloudflare Access 保護；決策見 ADR `docs/adr/2026-10-30-identity-self-hosted-federated-login.md` 與 `docs/adr/2026-09-27-identity-opaque-sessions.md`。代行的授權、工作階段、稽核與編排由 delegation 擁有。

## 1. 認證

- 顧客的登入、session 與帳號連結由本模組處理（登入方式見 §7）；後台的登入與白名單由 Cloudflare Access 處理。憑證是 opaque session token：web 是 httpOnly 的 `__Host-hk_session` cookie，App 是 `Authorization: Bearer`，其他位置不收，兩者都有時以 Bearer 為準。
- 模組依 token 的 issuer 選 verifier：後台 Access JWT（或本機 dev admin token，以 `iss` 精確比對）只會認證成管理員；其餘 token 一律當顧客的 opaque session，只認證成帳號。Access 驗 team domain 的 JWKS、application AUD tag、`iss`、`exp`、`nbf`。
- 管理員在首次認證時建立；顧客的帳號在第一次登入時建立（§2）。cookie、`Authorization` header 與 CSRF 在 `internal/httpapi`，不在本模組。
- 停權、刪除與角色以本模組的資料為準，每個請求都讀（session 列被刪除，下一個請求就失效）。停權的帳號照樣認證成功，結果帶停權時間（`Authenticated.SuspendedAt`），能做什麼由入口依動作的性質決定（`internal/httpapi` 的 `customerEffects`，A7-D18）；已刪除的帳號回 Unauthenticated。

### App 原生登入與 session 管理

- App 以原生 SDK 取得 ID token：`IssueAppLoginNonce` 發單次、5 分鐘的 nonce，`ExchangeIdToken` 驗 ID token 後登入並發 App 種類的 session（閒置 14 天、絕對 90 天）。ID token 的 `nonce` 必須等於請求的 nonce，驗證通過才消耗它（無效的 token 不作廢別人的 nonce）；Google：`iss` 為 `https://accounts.google.com` 或 `accounts.google.com`、`aud` 為 web client ID（`GOOGLE_WEB_CLIENT_ID`）、`azp` 在 `GOOGLE_APP_CLIENT_IDS`；LINE：ES256、以 `https://api.line.me/oauth2/v2.1/certs` 驗、`aud` 為 channel ID。ADR D2 的「SDK 帶不了 nonce 時改短效 token 加單次使用紀錄」尚未採用（步驟 6 實測後定）。
- `SignOut` 刪目前的 session（web 另清 cookie）；`LinkIdentity`（App 帶 ID token 與 nonce，web 走 `GET /api/auth/{provider}/start?link=1`）連結 provider 身分，身分已屬於別的帳號（含墓碑）回 `ALREADY_EXISTS` 與 `IDENTITY_ALREADY_LINKED`；`ListSessions` 列有效的 session 並標出目前這個；`ListLinkedIdentities` 列呼叫者帳號已連結的登入身分（只有 provider 與連結時間，不回 subject 與 email；同一 provider 可能有多筆，不分頁）；`RevokeSession` 要求目前的 session 在 `ReauthWindow`（15 分鐘）內以 provider 驗證過，否則回 `FAILED_PRECONDITION` 與 `REAUTHENTICATION_REQUIRED`，重新驗證就是再走一次登入（登入換發新 session，`auth_time` 是登入時間）。五者停權的帳號都能呼叫（自助／讀取，A7-D18）。
- web 的連結：start 把已登入的帳號寫進暫存列，callback 要通過登入前 cookie、`state`，而且現在登入的帳號等於發起的帳號才連結，不換發 session；失敗回返回網址並附 `link_error`。

## 2. 帳號建立

- 同一個 (provider, subject) 並發的第一次登入只建一個帳號：`account_identities` 的主鍵 `(provider, subject)` 加同一個 transaction。沒有 email（LINE 使用者可以拒給）也照樣建帳。登入遇到已刪除帳號留下的墓碑身分時被拒絕，不再建帳號。
- 建帳不授予任何業務角色。建帳的 transaction 內發布 `AccountCreated`，buyer 模組訂閱並在同一個 transaction 內建立買家（ADR `docs/adr/2026-09-30-account-created-provisions-buyer.md`）。
- `accounts.email` 是 provider 回報的已驗證 email，每次登入更新，作通知的寄件地址，也給後台顯示與搜尋；不是識別、不唯一，沒有已驗證 email 時保持原值。

## 3. 帳號角色

- 角色只有 `SELLER`，帳號另外申請，由 seller 授予與移除；移除角色不刪帳號。買家不是角色：帳號活著就是買家。
- 不是角色的值（例如 `BUYER`）授予或移除都回 InvalidInput；DB CHECK `account_roles_role_check` 只允許 `SELLER`。
- `account_roles.source_id` 是移除 SELLER 時的柵欄：移除帶來源，目前的授予屬於較新的來源就不移除。
  - 授予帶來源（該次開店的 `store_id`）；帳號已有同角色時 `source_id` 改指到這次授予帶的值，值相同則沒有變化（回傳 0 列）。授予不帶來源也會把既有的來源改成 NULL，呼叫方永遠帶當下的來源。
  - 移除的條件是目前的授予沒有來源、或來源等於呼叫帶的來源，比對在同一句 `DELETE` 內。被柵欄擋下時（有角色但沒有移除）呼叫端視為成功，帳號保留。
- 授予、移除角色與刪帳號都先鎖住帳號列。

## 4. 停權、刪除與生命週期

- 停權與刪帳在同一個 transaction 刪該帳號全部的 session。停權冪等：已經停權時不寫第二筆。
- 停權紀錄（`account_suspensions`，A7-D9）：型別只有 `PERMANENT`（A7-D16，沒有 `ends_at`），理由類別是 `platform/policy.Category` 的值（`TestCodeValueChecksMatchTheGoTypes`），另有說明、不透明的來源參照（identity 不讀它）、停權時間、解除時間與解除原因。`SuspendAccount(SuspendInput)` 在同一個 transaction 寫紀錄、設 `suspended_at`；已經停權時回傳原本的紀錄（`created=false`），不改理由、不寫第二筆；`uidx_account_suspensions_active` 保證同一帳號未解除的紀錄最多一筆。紀錄與狀態任何一步失敗都整筆回滾（`TestFailedRecordWriteLeavesTheAccountNotSuspended`、`TestFailureAfterTheRecordRollsTheRecordBack`）。
- `UnsuspendAccount(accountID, reason)`：reason 必填；同一個 transaction 清 `suspended_at`、在目前的紀錄記解除時間與原因；沒有停權時成功並回傳 `released=false`。解除留列不刪，之後可以再停權（新的一筆）。`CurrentSuspension(accountID)` 回傳目前未解除的紀錄，給 A7-2 的「讀自己的停權」。
- 停權、解除入口只允許 `accountsuspension` 呼叫（`internal/arch` 的 `TestSuspensionEntriesAreOnlyCalledByAccountsuspension` 掃非測試程式）：「擁有商店的帳號不能停權」在那裡檢查，identity 是葉節點、不查店家。刪帳號時未解除的紀錄以 `release_reason = ACCOUNT_DELETED` 解除。手動解除（`UnsuspendAccount`）拒絕這個值，只有帳號刪除路徑能寫。
- 受限登入（ADR `2026-10-29-account-suspension-and-moderation.md`）：認證不因停權回錯誤。入口 `withSession` 依 `customerEffects` 的宣告（讀取、寫入、自助）處理：公開與受邀供應商的 service 以訪客執行，讀取與自助放行，寫入回 `PermissionDenied` 與 `ACCOUNT_SUSPENDED`，舊 gateway 的 REST 依 HTTP method；`GetSession` 回 `suspension.suspended_at`。守門：`TestSuspendedAccountIsAuthorizedByTheNatureOfTheAction`、`TestSuspendedAccountStillAuthenticatesAndCarriesTheSuspensionTime`、`TestSuspendedAccountHasRestrictedAccess`。
- 刪帳號留下帳號墓碑，`account_identities` 的列也留作墓碑（保留 provider 與 subject，清掉 email），同一個登入身分不會再建一個帳號，以它登入回 `Unauthenticated`；`email` 清為 NULL（`accounts_deleted_check`）。刪帳同時發出 `AccountDeleted`。
- 帳號的生命週期是 ACTIVE → PENDING_DELETION → DELETED（`Account.Lifecycle`；欄位 `deletion_pending_at`、`deleted_at`）。
  - `BeginDeletion` 只由 accountdeletion 在受理會等待未結訂單的申請時呼叫。
  - 等待刪除時仍可登入，但 `GrantRole` 回 FailedPrecondition。
  - `DeleteAccount` 清掉等待的狀態；`accounts_deletion_pending_check` 保證已刪除的帳號沒有等待狀態。
  - 狀態在 identity，流程在 accountdeletion（ADR `docs/adr/2026-09-30-account-deletion-module.md` 的修訂）。
- 跨模組的刪帳流程（accountdeletion 刪帳並關閉買家）在自己的 transaction 內以 `LockAccount` 取同一把帳號列鎖，再讀角色或呼叫參與者。`DeleteAccount` 只由 accountdeletion 呼叫，其他入口不得直接刪帳號。
- Google 或 LINE 停擺時新登入失敗；已登入的使用者不受影響，session 只靠我方資料庫。

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

## 7. 登入方式與 provider 設定

- 只提供 Google 與 LINE；不提供 Sign in with Apple（Will 2026-10-30，ADR 自建登入）。Apple Pay 是付款，與登入無關。
- web：單體提供 `GET /api/auth/{google,line}/start` 與 `/api/auth/{google,line}/callback`（純 HTTP，每家獨立路徑防 mix-up，經 website 的同源反代到達）。start 寫一列暫存的登入流程（`state`、PKCE verifier、`nonce`、預期 provider、返回網址，10 分鐘、單次使用）並設短效的 `__Host-hk_login` cookie；callback 比對該 cookie、暫存列與 `state`，Google 的授權回應缺 `iss` 一律拒絕，換 code 後驗 ID token（Google 以 JWKS；LINE 的 web ID token 以 channel secret 驗 HS256），成功時換發新 session 並設 `__Host-hk_session`（httpOnly、Secure、`SameSite=Lax`，Max-Age 等於絕對逾時 90 天），失敗回登入頁並附固定的錯誤代碼。返回網址只收 `/` 開頭且不是 `//` 的站內路徑。
- App：原生 SDK 取得 ID token 後走 §1 的 `IssueAppLoginNonce`／`ExchangeIdToken`。
- 設定（環境變數名稱；值不進版控）：`GOOGLE_WEB_CLIENT_ID` 與 `GOOGLE_WEB_CLIENT_SECRET`（Google 的 web OAuth client）、`LINE_CHANNEL_ID` 與 `LINE_CHANNEL_SECRET`（LINE Login channel）各自整組選填，只設一個是設定錯誤，沒設定的 provider 其 start 回 404；redirect URI 是 `PUBLIC_WEB_ORIGIN` 加 `/api/auth/{google,line}/callback`，要登記在各家的 console。`GOOGLE_APP_CLIENT_IDS` 是 App 原生登入的 Google ID token 的 `azp` 允許清單，空的時候 Google 的 App 登入不啟用。`GOOGLE_CLIENT_IDS` 只用於 vendor portal 的 step-up 驗證，與登入無關。後台：`CF_ACCESS_TEAM_DOMAIN`、`CF_ACCESS_ADMIN_AUD`。
- 本機可以不設這些：顧客用 §8 的 dev session，後台用 dev admin token。要用真的 Google／LINE 登入驗收時，在啟動 `make serve` 的 shell 匯出上面的變數（Makefile 不讀檔），並在各家 console 登記 `http://localhost:3000/api/auth/{google,line}/callback`（`PUBLIC_WEB_ORIGIN` 的本機預設是 `http://localhost:3000`）。

## 8. 本機開發與測試登入（DevAuth）

- 顧客：dev 使用者以帳號 ID 識別，不建假的登入身分。`make dev-token args='-account <帳號 ID>'` 為既有的帳號在資料庫建一個 App 種類的 session 並印出 token（以 `Authorization: Bearer` 帶；帳號不存在是錯誤，不會默默建立）；不帶 `-account` 時建立一個沒有登入身分的新帳號（ID 印到 stderr）。
- 後台：`make dev-token args='-admin -user dev_ops'` 以本機的 RSA 金鑰簽 24 小時的 Access 形狀 JWT（使用者是 `dev_` 開頭的 ID），第一個請求時建立管理員。`-account` 與 `-admin` 不能並用。
- `DEV_AUTH_KEY_FILE`（`make dev-key` 產生）設了才接受後台的 dev token 與執行 `dev-token`、`dev-seed`；不能與 Cloudflare Access 設定同時使用，同時設定時 serve 拒絕啟動。

## 9. 資料

schema `identity`（`db/schema.sql`）：

- `accounts`：登入的 principal。主鍵目前是 IDENTITY（過渡期，對外可列舉）；`email` 是寄件地址，不是識別。
- `account_roles`：`(account_id, role)`，role 只允許 `SELLER`。
- `admins`：管理員，與帳號是不同的 principal；`access_subject` 是 Access JWT 的 `sub`（UNIQUE），`email` 是首次登入時的顯示快照。
- `admin_roles`：`(admin_id, role)`，role 的 CHECK 只允許 `OWNER`。
- `admin_role_events`：角色授予與撤銷的歷史，只追加；`admin_role_events_*_check` 限定 role、action、reason（1 到 1000 字且不能全是空白）與 source。
- `account_suspensions`：停權紀錄。
- `account_identities`：provider 身分，`(provider, subject)` 為主鍵；帳號刪除時留作墓碑並清掉 email。
- `sessions`：opaque session，只存 token 的 SHA-256。
- `login_flows`：web 登入流程的單次暫存。
- `app_login_nonces`：App 登入的單次 nonce。
- 代行資格與行為參數由 delegation 擁有。

外部 provider 的 client 在 `internal/`：`oidc`（JWT 簽章與 claims 的驗證，Access 與 Google、LINE 的 ID token 共用）。web 登入的 OAuth client（authorization code ＋ PKCE）在 `internal/provider`。

## 10. 已知限制

- 帳號與管理員的主鍵是 IDENTITY，對外可列舉；舊 Java 服務仍存成 `BIGINT`。代行 session 的過渡 ID 記在 delegation。
- 帳號的 email 只在登入時以 provider 回報的已驗證 email 更新（LINE 使用者可以拒給）；後台的清單以它顯示與搜尋。
- Google 自動連結信任 `hd`：Workspace 管理員刪除使用者後，同一個地址的新 `sub` 會自動連到舊帳號（ADR 自建登入「已知限制」）。
- 賣家 CRM 以手機找顧客（legacy 的 `GetAccountByPhone`）一律回「找不到」：帳號沒有手機。
- 外部 Access 登出不會自動撤銷代行。
- 代行資格與撤回時的工作階段撤銷由 delegation 管理（`docs/modules/delegation.md` §7）。

## 11. 模組內部結構

- `internal/account`：帳號、角色、清單、登入時建帳。`internal/admin`：管理員查詢。`internal/auth`：Authenticate 的路由（後台 JWT 以 iss 辨認，其餘當 session）、認證時建管理員、舊 Google step-up。`internal/session`：session 的建立、解析（逾時、續期、帳號狀態與角色）、撤銷，以及 web 登入流程與 App nonce 的單次使用。`internal/login`：以 provider 身分登入、建帳、Google 自動連結、手動連結。
- `internal/provider` 把 `oidc` 與 web 登入的 OAuth client 接成模組公開介面，並提供後台的 dev token；`internal/store` 是 sqlc 查詢套件。

## 12. 不變量的守門

`internal/modules/identity/internal/<子領域>/AGENTS.md` 的不變量與守門測試或約束的對照（開頭為該不變量的第一句）；子領域檔是各條守門的現行來源，本節是跨子領域的總表。

- 後台 Access/dev admin token（JWT，以 `iss` 精確比對）只認證成管理員；其餘 token 一律當顧客的 opaque session，只認證成帳號：`TestAuthenticationErrors`、`TestServicesAreSeparatedByAudience`（httpapi/identity）
- session 只存 token 的 SHA-256；閒置逾時 web 7 天、App 14 天，絕對逾時 90 天，`last_used_at` 一天最多寫一次；登入一律換發新 token：`TestSessionIdleAndAbsoluteExpiry`、`TestSessionRenewalWritesAtMostOncePerDay`、`TestSuspensionAndSessionDeletionAreAtomic`、`TestDeletionIsAtomicAcrossSessionsAndIdentities`、`TestLoginIssuesAFreshSessionAndRevokesThePreviousOne`
- App 原生登入的 ID token：Google 驗 `iss`（兩種寫法）、`aud` 為 web client ID、`azp` 在允許清單、`nonce`；LINE 以 ES256 與 JWKS 驗；nonce 先比對 ID token、通過才單次消耗：`TestGoogleAppLoginVerifiesTheIDToken`、`TestLineAppLoginVerifiesTheES256IDTokenWithTheJWKS`（provider）、`TestRevokeSessionNeedsRecentAuthentication`（httpapi/identity）
- Google 自動連結只在 `email_verified` 為真、email 網域是 `gmail.com` 或帶 `hd`、而且恰好一個存活帳號上有「符合自動連結資格的 Google 身分」（`auto_link_eligible`：Google、已驗證、gmail.com 或帶 `hd`）的 email 相符時發生；比對的是身分的 email，不是 `accounts.email`；LINE 從不自動連結，LINE 的已驗證 email 也不是目標：`TestAutoLinking`、`TestLineVerifiedEmailIsNotAnAutoLinkAnchor`、`TestAutoLinkIgnoresTombstones`、`TestConcurrentFirstLoginsShareOneAccount`、`TestLoginIsAtomic`
- 登入流程的 `state` 與 App 的 nonce 單次使用，驗證失敗也已作廢，10 分鐘與 5 分鐘逾時；返回網址只收站內相對路徑：`TestLoginFlowIsSingleUse`、`TestLoginFlowRefusesUnsafeReturnPathsAndUnknownProviders`、`TestAppNonceIsSingleUseAndShortLived`
- 新帳號不授予角色（買家由 buyer 訂閱 `AccountCreated` 建立）：`TestLoginCreatesAnAccountForEachProvider`
- 刪帳留下墓碑（登入身分的列保留，同一個身分不會再建帳號）並發出 `AccountDeleted`，不排任何 job；停權不排 job，認證結果帶 `SuspendedAt`、不回錯誤：`TestSuspendedAccountStillAuthenticatesAndCarriesTheSuspensionTime`、`TestDeletionLeavesATombstoneAndKeepsTheLoginIdentity`；`accounts_deleted_check`
- 停權紀錄與 `suspended_at` 同一個 transaction，未解除的紀錄最多一筆；停權、解除入口只由 `accountsuspension` 呼叫：`TestFailureAfterTheRecordRollsTheRecordBack`、`TestRepeatedSuspendKeepsTheFirstRecord`、arch 的 `TestSuspensionEntriesAreOnlyCalledByAccountsuspension`
- 帳號生命週期 ACTIVE → PENDING_DELETION → DELETED；等待刪除時 `GrantRole` 回 FailedPrecondition：`TestDeletionPendingRefusesGrantRole`；`accounts_deletion_pending_check`
- 不是角色的值（例如 `BUYER`）授予或移除回 InvalidInput：`TestRemoveRole`；`account_roles_role_check`
- 授予、撤銷管理員角色與寫 `admin_role_events` 在同一個 transaction；事件只追加；撤銷在同一個 transaction 內發出 `AdminRoleRevoked`，授予不發：`TestAdminRoleEventsAreAppendOnly`、`TestRevokePublishesAdminRoleRevokedInTheSameTransaction`
- `AdminAllows` 在呼叫方的 transaction 內以 `FOR SHARE` 讀角色列，與撤銷互相等待：`TestAdminAllowsReadsRolesInsideTheCallersTransaction`
- 每個請求都讀角色、不快取；零角色沒有任何權限：`TestOwnerPassesEveryCheckAndRevocationTakesEffectOnTheNextRequest`（httpapi/identity）

