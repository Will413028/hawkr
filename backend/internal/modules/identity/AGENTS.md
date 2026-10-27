# identity 模組

誰在使用系統：登入的帳號（每個帳號都是買家，賣家是另外申請的角色）與管理員、帳號的角色與狀態。顧客登入與 session 由 Clerk 處理，後台由 Cloudflare Access 保護（ADR `2026-09-27-identity-clerk.md` 文末修訂）。來源是 account-service（遷移計畫 A2，步驟與進度在 `docs/plans/2026-09-27-a2-identity.md`），代行的授權、工作階段、稽核與編排由 delegation 擁有。

> 搬遷的進度與剩下的項目在計畫 `docs/plans/2026-09-27-a2-identity.md`。account-service 已刪除；代行的資料與行為已搬到 delegation；舊 AccountService 經 `internal/legacy/accountservice` 呼叫本模組。

## 職責

- 認證（ADR `2026-09-27-identity-clerk.md` 文末修訂）：顧客的登入、session 與帳號連結由 Clerk 處理；後台的登入與白名單由 Cloudflare Access 處理。這裡依 issuer 驗證顧客 Clerk token 或後台 Access JWT（team domain 的 JWKS、application AUD tag、`iss`、`exp`、`nbf`），首次認證建立帳號或管理員。cookie、`Authorization` header 與 CSRF 在 `internal/httpapi`。
- 角色（設計書 44）：只有要另外申請的 SELLER，由 seller 授予與移除；移除角色不刪帳號。買家不是角色：帳號活著就是買家（ADR `2026-09-30-account-created-provisions-buyer`）。
- 管理員角色（ADR `2026-10-04-admin-roles`）：誰是管理員由 Access 決定，角色由本模組的 `admin_roles` 決定；角色目前只有 OWNER，擁有 `platform/adminperm` 目錄裡的全部權限（由程式計算，不逐條列），零角色等於沒有任何權限。每個請求認證管理員時一併讀角色（`Principal.AdminRoles`，與帳號的 `Roles` 分開），不快取。授予與撤銷只有 CLI `hawkr admin-role grant|revoke`（`GrantAdminRole`、`RevokeAdminRole`）；方法與 `/api/files/admin/…` 路由的權限檢查在 `internal/httpapi/adminauth`，本模組只提供角色。
- 視角：Connect 沒有視角（ADR `2026-10-07-connect-without-perspective`）；「正在看哪個工作區」是 client 的 UI 狀態。`X-Active-Role` 只有舊 REST 送，由 `internal/legacy/gatewayproxy` 讀它、以 `internal/legacy/accounttype.OfLoginRole` 推導 `X-Login-Role` 給舊 gateway；本模組不認識視角。
- 帳號的停權與刪除：以本模組的資料為準，每個請求都讀（token 在 Clerk 撤銷之後的效期內仍驗得過）；同一個 transaction 排 job 停用或刪除 Clerk 的使用者。
- Sign in with Apple 的授權撤銷（App Store 審核準則 5.1.1(v)；Clerk 刪除使用者時不撤銷）：App 原生登入後以 `LinkAppleCredential` 交出 identity token 與 authorization code；驗證 token 後把 `sub`、audience 與加密的 code 排入 job，job 換 refresh token，Apple 簽發的 ID token 與連結時的身分相同才以 AAD（表、欄、帳號 ID）加密存入 `identity.apple_tokens`（每帳號一筆，重新連結取代），身分不符或帳號已刪除才當場撤銷換得的 token；暫時性失敗（驗證 ID token 連不到公鑰、保存失敗）不撤銷（Apple 的撤銷會撤掉使用者剛給的整個授權），改排 `identity.save_apple_token` job 帶加密的 token 回應重試，最後一次仍失敗就記 error 並放棄（下一次原生登入再交）。每種失敗都以 error 等級記 log（原因與帳號 ID，不含 code 與 token）。不呼叫 Clerk Backend API 核對 `sub`。所有刪除路徑都經 `DeleteAccount`，它在同一個 transaction 內取走 token、刪列並排入 `identity.revoke_apple_token`。部署的環境 Apple 設定必填（缺任何一個啟動失敗），本機開發可不設，此時 `LinkAppleCredential` 回 unavailable、刪帳號不受影響；啟動時 `CheckAppleKeys` 檢查保存的密文與排隊中 job 參數的密文用到的金鑰都在 `IDENTITY_TOKEN_KEYS`。web 登入的 Apple 使用者沒有保存的 token（授權碼在 Clerk），website 維持人工提示。
- 本機開發與測試的登入（`DevAuth`）：本機的 RSA 金鑰簽的 dev token，使用者是 `dev_` 開頭的 ID，不連外部服務（`hawkr dev-token`；顧客 token 為 Clerk 形狀、後台 token 為 Access 形狀；serve 設了 `DEV_AUTH_KEY_FILE` 才接受，不能與正式 Clerk 或 Access 設定同時使用）。

## 擁有的資料

schema `identity`（`db/schema.sql`，計畫步驟 1 起）：

- `accounts`：登入的 principal。`clerk_user_id` 是顧客的 Clerk application 的使用者 ID（UNIQUE，墓碑也保留）；`email` 是建帳時 Clerk session 的 email claim，只給後台顯示與搜尋，不是識別、不同步。主鍵過渡期是 IDENTITY（見 `CARRYOVER.md` 的沿用盤點）。
- `account_roles`：帳號另外申請的角色（只有 `SELLER`），`(account_id, role)`。
- `admins`：管理員，與帳號是不同的 principal；`access_subject` 是 Access JWT 的 `sub`（UNIQUE），`email` 是首次登入時的顯示快照。
- `admin_roles`：管理員目前的角色，`(admin_id, role)`，role 的 CHECK 只允許 `OWNER`；沒有列的管理員沒有任何權限。
- `admin_role_events`：角色的授予與撤銷歷史，只追加（queries 只有 INSERT 與 SELECT，`TestAdminRoleEventsAreAppendOnly` 檢查）。記 action（GRANT、REVOKE）、必填理由（規則同 `platform/auditreason`）、source（目前只有 `CLI`）、`db_user`（`session_user`）與 `os_user`（取不到時 NULL），後兩者未經驗證、只供參考，沒有自填操作者的參數。

代行資格與行為參數由 delegation 擁有。

外部 provider 的 client 在 `internal/`：`clerk`（顧客 session token 的驗證、Backend API、dev token）、`oidc`（Google、Apple 的 ID token；Clerk 與 Access 的驗證也用它）、`apple`（token 的交換與撤銷）、`seal`（AES-GCM，加密排入 job 與保存的 Apple 憑證）。

## 模組內部結構

- 根套件組裝 `Module`，保留公開型別別名、事件、錯誤、provider 建構函式與轉呼叫方法；整合測試在根套件，只經公開 API。
- 根套件的 `Deps` 是組裝 API；`internal/model` 定義公開型別、事件與 provider 介面；各子領域宣告最小 `Deps`。`internal/account` 管帳號、角色、清單、首次認證建帳與 Clerk 使用者 job；`internal/admin` 管管理員查詢；`internal/auth` 管 token 路由、呼叫 account 建帳、認證時建管理員與舊 Google step-up；`internal/applerevoke` 管 Apple 憑證的連結、換 token 與保存的 job，以及刪帳時撤銷保存的 token。
- `internal/provider` 把 `clerk`、`apple`、`oidc`、`seal` 的 client 接成模組公開介面，並提供本機 dev token；`internal/store` 是 sqlc 生成的查詢套件。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；根套件不得 import `store`。

## 不變量

- 認證以 token 的 issuer 選 verifier：顧客 Clerk token 只會認證成帳號，後台 Access/dev admin token 只會認證成管理員。停權的帳號在下一個請求回 PermissionDenied，刪除則回 Unauthenticated；刪除的帳號留下墓碑與 `clerk_user_id`，同一個 Clerk 使用者不會再建一個帳號。
- 同一個 Clerk 使用者同時的第一個請求只建一個帳號（`clerk_user_id` 的 UNIQUE 加 `ON CONFLICT DO NOTHING`）。
- 停權、刪帳與排入停用、刪除 Clerk 使用者的 job 在同一個 transaction；刪帳同時發出 `AccountDeleted`。
- 帳號的生命週期是 ACTIVE → PENDING_DELETION → DELETED（`Account.Lifecycle`；`deletion_pending_at`、`deleted_at`）：`BeginDeletion` 只由 accountdeletion 在受理會等待未結訂單的申請時呼叫；等待刪除時仍可登入，但 `GrantRole` 回 FailedPrecondition；`DeleteAccount` 清掉等待的狀態（`accounts_deletion_pending_check`）。狀態在 identity，流程在 accountdeletion（ADR `2026-09-30-account-deletion-module` 的修訂）。`TestDeletionPendingRefusesGrantRole` 驗證。
- 授予、移除角色與刪帳號都先鎖住帳號列；跨模組的流程（`accountdeletion` 刪帳並關閉買家）在自己的 transaction 內以 `LockAccount` 取同一把鎖，再讀角色或呼叫參與者。`DeleteAccount` 只由 `accountdeletion` 呼叫，其他入口不得直接刪帳號（否則買家不會關閉）。
- 不是角色的值（例如舊的 `BUYER`）授予或移除都回 InvalidInput；DB CHECK 只允許 `SELLER`。
- 後台只根據驗證通過的 Access JWT（或本機 dev admin token）建立管理員；Access 的 email 白名單是開通的唯一來源，email 字串只供顯示。
- 授予、撤銷角色與寫入 `admin_role_events` 在同一個 transaction；授予的對象必須是已存在的管理員（Access 白名單的人登入一次之後），不預建；重複授予或撤銷沒有的角色回錯誤、不寫事件。撤銷在同一個 transaction 內發出 `AdminRoleRevoked`，delegation 訂閱，剩下的角色不再允許開始代行時撤銷該管理員進行中的代行，訂閱者失敗時撤銷回滾。撤掉最後一個 OWNER 是允許的（CLI 本身就是 break-glass），`AdminRoleChange.HolderRemains` 讓 CLI 警告。
- `AdminAllows` 給 delegation 在自己的 transaction 內檢查權限：以 `FOR SHARE` 讀角色列，與撤銷的 DELETE 互相等待；不在 transaction 內呼叫回錯誤。
- 後台按鈕登出的 Connect handler 直接呼叫 delegation 撤銷進行中的代行；Access 在其他地方登出或 session 到期不會觸發這個呼叫。

## 已知限制

- `account_roles.source_id` 的柵欄只為了 seller-service 以非同步方式移除 SELLER，A3b 步驟 11 seller 搬進單體後拿掉。
- 帳號與管理員的主鍵是 IDENTITY，對外可列舉：舊 Java 服務仍存成 `BIGINT`；最後一個數字 ID 消費端改寫後以 migration 換成 uuid（ADR `2026-09-25-row-identity-by-data-kind.md`，遷移計畫階段 C）。代行 session 的過渡 ID 記在 delegation。
- Clerk 停擺時新登入失敗，已登入的使用者在下一次換 token（最多約 60 秒）之後也失效（ADR 的後果）。
- 帳號的 email 是建帳時的快照，之後在 Clerk 改了 email 不會同步（不接 webhook）；後台的清單以它顯示與搜尋。
- 管理員沒有停用的功能（後台沒有管理員管理的畫面）：停用單體內的權限是撤銷他的角色（`admin-role revoke`，同時撤銷進行中的代行）；離職處置另須從 Access 白名單移除、在 Zero Trust 撤銷該使用者的 Access sessions，並撤回 delegation 的代行資格。外部 Access 登出不會自動撤銷代行。`admins` 的列保留，供代行的歷史 ID 與角色事件參照。
- 角色只有 OWNER，授予只有 CLI、沒有經過驗證的操作者（事件記來源與未經驗證的 `db_user`、`os_user`）；第二位管理員加入且職務確定時才定義第一個非 OWNER 角色、`GetSession` 回 permissions 並評估把授予搬進後台 UI（ADR 的重新評估條件）。
- 賣家 CRM 以手機找顧客（legacy 的 `GetAccountByPhone`）一律回「找不到」：帳號不再有手機；CRM 的手機查詢改查 buyer 自己維護的聯絡電話，A6 隨 crm 接上（A3 決定 10a）。
- 代行資格與撤回時的工作階段撤銷由 delegation 管理（設計書 56 §8.5.3）。

## 讀取模型

無。

## 不負責

- 刪除帳號的申請、等待未結訂單與參與者的關閉：`accountdeletion`；identity 只提供刪帳號與帳號鎖。
- 買家、賣家的 profile 與業務身分（`buyer_id`、`store_id`）：buyer、seller。identity 只知道帳號有沒有賣家角色。
- 買家的建立：identity 在建帳的 transaction 內發布 `AccountCreated`，buyer 模組訂閱，在同一個 transaction 內建立買家（ADR `2026-09-30-account-created-provisions-buyer`），不授予角色。identity 不依賴 buyer，也不判斷帳號要不要當買家。
- 舊契約的買家位元（`account_type`、`X-Account-Type`、`X-Login-Role`）：`internal/legacy/accounttype` 在邊界合成，identity 不存。
- 代行的授權、工作階段、稽核與編排：delegation；過渡期跨服務編排仍在 gateway 與 `internal/legacy`。identity 不依賴 seller 或 delegation。
- 顧客的登入、登出、session、帳號連結與登入防濫用：Clerk。後台的登入、白名單與 session：Cloudflare Access。cookie、`Authorization` header 與 CSRF：`internal/httpapi`。
- 推播的 device token：notification（A8）；過渡期是 notification-service。

## 決策紀錄

從 account-service 沿用的每個機制、當時的決定與理由，以及與 account-service 的差異在同目錄的 `CARRYOVER.md`。改動或移除沿用的機制前，先讀那一列。

## 特性測試

不寫 D4 的特性測試（Will 2026-09-29，比照 content 的退役）：舊的一側 account-service 已刪除，無法以原設定啟動；對外行為也依 ADR `2026-09-27-identity-clerk.md` 刻意改變（登入交給 Clerk，沒有 JWT、refresh 與手機驗證），同一份情境在兩邊不會得到相同結果。行為改由兩份 integration 測試涵蓋：`identity_integration_test.go`（模組：建帳、停權、刪除、墓碑、Apple 憑證連結、保存與撤銷、管理員）與 `internal/httpapi/identity/identity_integration_test.go`（入口：Bearer 與 cookie 的邊界、跨來源寫入、audience 分流、錯誤分流），OAuth 的登入以本機金鑰簽的 token 代替真的 provider。
