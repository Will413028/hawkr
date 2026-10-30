# identity/internal/login

登入流程、身分連結與 state／nonce。

## 不變量

- Google 自動連結只在 `email_verified` 為真，且恰好一個存活帳號上有 `auto_link_eligible` 身分的 email 相符時發生；比對的是身分的 email，不是 `accounts.email`。守門：`TestAutoLinking`、`TestAutoLinkIgnoresTombstones`。規格：§1。
- LINE 從不自動連結，LINE 的已驗證 email 也不是目標。守門：`TestLineVerifiedEmailIsNotAnAutoLinkAnchor`。規格：§1。
- 同一個 (provider, subject) 並發的首次登入只建一個帳號。守門：`TestConcurrentFirstLoginsShareOneAccount`、`TestLoginIsAtomic`。規格：§1。
- 登入流程的 `state` 與 App 的 nonce 單次使用，驗證失敗也已作廢，逾時為 `session.FlowTTL`、`session.AppNonceTTL`；返回網址只收站內相對路徑。守門：`TestLoginFlowIsSingleUse`、`TestLoginFlowRefusesUnsafeReturnPathsAndUnknownProviders`、`TestAppNonceIsSingleUseAndShortLived`。規格：§1。
