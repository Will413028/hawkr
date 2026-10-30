# identity 模組

## 職責

誰在使用系統：登入的帳號（每個帳號都是買家，賣家是另外申請的角色）與管理員、帳號的角色與狀態。顧客登入與 session 由 Clerk 處理，後台由 Cloudflare Access 保護（ADR `docs/adr/2026-09-27-identity-clerk.md` 含文末修訂）。代行的授權、工作階段、稽核與編排由 delegation 擁有。行為規格的唯一來源是 `docs/modules/identity.md`。

## 擁有的資料

schema `identity`，資料表與欄位的說明在 `docs/modules/identity.md` §9。`clerk_user_id` 過渡期可為 NULL（自建登入的帳號沒有），Clerk 退場時刪欄；帳號與管理員的主鍵過渡期是 IDENTITY，見 `CARRYOVER.md`。讀取模型：無。

## 發布的讀取介面

無。沒有 `db/pub.sql`，沒有 `pub_*`。

## 模組內部結構

- 根套件組裝 `Module`，保留公開型別別名、事件、錯誤、provider 建構函式與轉呼叫方法。根套件不得 import `store`。
- 根套件的 `Deps` 是組裝 API；`internal/model` 定義公開型別、事件與 provider 介面；各子領域宣告最小 `Deps`。
- 子領域套件（`internal/account`、`admin`、`auth`、`session`、`login`、`applerevoke`）與 `internal/provider`、`internal/store` 的分工在 `docs/modules/identity.md` §11。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- 後台 Access/dev admin token（JWT，以 `iss` 精確比對）只認證成管理員；其餘 token 一律當顧客的 opaque session，只認證成帳號。規格：`docs/modules/identity.md` §1。
- session 只存 token 的 SHA-256；閒置逾時 web 7 天、App 14 天，絕對逾時 90 天，`last_used_at` 一天最多寫一次；登入一律換發新 token。停權與刪帳在同一個 transaction 刪該帳號全部 session。
- App 原生登入的 ID token：Google 驗 `iss`（兩種寫法）、`aud` 為 web client ID、`azp` 在允許清單、`nonce`；LINE 以 ES256 與 JWKS 驗；nonce 先比對 ID token、通過才單次消耗。`RevokeSession` 的重新驗證期限是 `ReauthWindow`。規格：`docs/modules/identity.md` §1。
- Google 自動連結只在 `email_verified` 為真、email 網域是 `gmail.com` 或帶 `hd`、而且恰好一個存活帳號上有「符合自動連結資格的 Google 身分」（`auto_link_eligible`：Google、已驗證、gmail.com 或帶 `hd`）的 email 相符時發生；比對的是身分的 email，不是 `accounts.email`；LINE 從不自動連結，LINE 的已驗證 email 也不是目標。同一個 (provider, subject) 並發的首次登入只建一個帳號。
- 登入流程的 `state` 與 App 的 nonce 單次使用，驗證失敗也已作廢，10 分鐘與 5 分鐘逾時；返回網址只收站內相對路徑。
- 新帳號不授予角色（買家由 buyer 訂閱 `AccountCreated` 建立）。Clerk 時期的 `clerk_user_id` UNIQUE 在 Clerk 退場前保留。規格：§2。
- 刪帳與（有 Clerk 使用者時）排入刪除它的 job 在同一個 transaction，刪帳留下墓碑並發出 `AccountDeleted`；停權不排 Clerk job，認證結果帶 `SuspendedAt`、不回錯誤。規格：§4。
- 停權紀錄與 `suspended_at` 同一個 transaction，未解除的紀錄最多一筆；停權、解除入口只由 `accountsuspension` 呼叫。規格：§4。
- 帳號生命週期 ACTIVE → PENDING_DELETION → DELETED；等待刪除時 `GrantRole` 回 FailedPrecondition。規格：§4。
- 授予、移除角色與刪帳都先鎖帳號列；`DeleteAccount` 只由 accountdeletion 呼叫。守門：無（只有規格）。規格：§3、§4。
- 不是角色的值（例如 `BUYER`）授予或移除回 InvalidInput。規格：§3。
- 授予、撤銷管理員角色與寫 `admin_role_events` 在同一個 transaction；事件只追加；撤銷在同一個 transaction 內發出 `AdminRoleRevoked`，授予不發。規格：§5。
- `AdminAllows` 在呼叫方的 transaction 內以 `FOR SHARE` 讀角色列，與撤銷互相等待。規格：§5。
- 每個請求都讀角色、不快取；零角色沒有任何權限。規格：§5。
- Apple 憑證只在 ID token 與連結時身分相同才保存；身分不符或帳號已刪除才當場撤銷，暫時性失敗不撤銷；刪帳在同一個 transaction 取走 token 並排入撤銷。規格：§7。
- 每條不變量的守門測試與約束列在 `docs/modules/identity.md` §12。

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
