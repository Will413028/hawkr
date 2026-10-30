# identity/internal/login

登入流程、身分連結與 state／nonce。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/identity.md`。

## 不變量

- Google 自動連結只在 `email_verified` 為真、email 網域是 `gmail.com` 或帶 `hd`、而且恰好一個存活帳號上有「符合自動連結資格的 Google 身分」（`auto_link_eligible`：Google、已驗證、gmail.com 或帶 `hd`）的 email 相符時發生；比對的是身分的 email，不是 `accounts.email`；LINE 從不自動連結，LINE 的已驗證 email 也不是目標。同一個 (provider, subject) 並發的首次登入只建一個帳號。
- 登入流程的 `state` 與 App 的 nonce 單次使用，驗證失敗也已作廢，10 分鐘與 5 分鐘逾時；返回網址只收站內相對路徑。
