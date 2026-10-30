# identity 模組

## 職責

誰在使用系統：登入的帳號（每個帳號都是買家，賣家是另外申請的角色）與管理員、帳號的角色與狀態。顧客登入與 session 由 Clerk 處理，後台由 Cloudflare Access 保護（ADR `docs/adr/2026-09-27-identity-clerk.md` 含文末修訂）。代行的授權、工作階段、稽核與編排由 delegation 擁有。行為規格的唯一來源是 `docs/modules/identity.md`。

## 擁有的資料

schema `identity`（`db/schema.sql`，欄位說明見規格 §9）。讀取模型：無。

## 發布的讀取介面

無。沒有 `db/pub.sql`，沒有 `pub_*`。

## 模組內部結構

- 根套件組裝 `Module`，保留公開型別別名、事件、錯誤、provider 建構函式與轉呼叫方法。根套件不得 import `store`。
- 根套件的 `Deps` 是組裝 API；`internal/model` 定義公開型別、事件與 provider 介面；各子領域宣告最小 `Deps`。
- 子領域套件與 `internal/provider`、`internal/store` 的分工在 `docs/modules/identity.md` §11。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- 後台 Access/dev admin token（JWT，以 `iss` 精確比對）只認證成管理員；其餘 token 一律當顧客的 opaque session，只認證成帳號。規格：`docs/modules/identity.md` §1。
- 子領域的守門測試與約束在 `internal/<子領域>/AGENTS.md`；`docs/modules/identity.md` §12 是同一份的對照表。
- 單一子領域的不變量在 `internal/<子領域>/AGENTS.md`（碰該目錄時才載入）；跨子領域的才放這裡。

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
