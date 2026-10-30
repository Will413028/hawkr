# Clerk 使用點盤點

快照 commit：`9b133ff`（2026-10-30）。每組的檔數：

```sh
rg -il clerk -g '!node_modules' -g '!gen/**' -g '!*.g.dart' -g '!pnpm-lock*' <目錄> | wc -l
```

收尾時每一列的「去向」都要填好，`rg -il clerk`（同上排除條件，再排除 `backend/docs/adr` 與本計畫目錄）要沒有結果。

| 組 | 檔數 | 主要位置 | 現在做什麼 | 去向 |
|---|---|---|---|---|
| identity 模組 | 24 | `internal/clerk`、`internal/provider/{providers,dev}.go`、`internal/auth/auth.go`（`CreateForClerk`）、`internal/account/jobs.go`（River job `identity.clerk_user`）、`accounts.clerk_user_id`（migration `20260927071536_clerk_identity.sql`、`uidx_accounts_clerk_user_id`）、`model.go` 的 `Clerk`／`ClerkIdentity`、`arch/modules.go:148-150` | 驗 Clerk session token、第一次認證建帳、刪帳時刪 Clerk 使用者 | 步驟 2 改寫成 session 解析與聯合登入；步驟 4 刪除 `internal/clerk`、job、欄位 |
| `internal/httpapi` | 17 | `session.go`（`withSession`）、`rpc/session.go`（Bearer，沒有時讀 `__session` cookie）、`characterize/*`、約 8 個 integration 測試 | 取出憑證交給 identity | 步驟 2：Bearer 或 `__Host-hk_session` cookie；`__session` 刪除 |
| backend 入口與設定 | 7＋5 | `cmd/hawkr/identity.go`（`readClerk`、`CLERK_*`、`sk_live_` 判定）、`main.go`、`.env.template:236-245`、`Makefile:260-313`、`scripts/clerk-env.sh`、`scripts/characterize.sh`、`infra/seed-data/01-account.sql` | 讀 Clerk 設定、本機 dev token | 步驟 2 換成各 provider 的設定；步驟 4 刪 `clerk-env.sh` 與 `CLERK_*` |
| proto | 3 | `api/hawkr/identity/…/identity.proto:5-6,55,64`、admin `identity.proto:89`、`error_reason.proto:52` | 只有註解 | 步驟 2 改註解，新增登入相關 RPC |
| website | 72 | `@clerk/nextjs` 7.9.7、`middleware.ts` 的 `clerkMiddleware`、`next.config.ts` 的 CSP（`clerkFrontendApi`）、`lib/auth/*`（`clerk-token`、`context`、`session-resolver`、`sso-callback`、`useOAuthSignIn`、`useAuthGate`）、`lib/api`、`lib/connect` transport、SSE、AI chat、媒體上傳、`lib/config/env.ts`、登入與 SSO callback 頁、刪帳區塊、`RoleZoneGuard`、`RootDocument`、約 30 個測試（含 e2e 的 `clerk-session.ts`、`playwright.config.ts`） | Clerk SDK 登入，請求前 `getToken()` 帶 Bearer | 步驟 3 全換成 cookie session（同源反代，transport 不再帶 token） |
| admin | 2 | `README.md`、`AGENTS.md` | 只有文件 | 步驟 5 改文件 |
| `packages/hawkr_auth` | 8 | `clerk_auth` 0.0.18-beta、`lib/clerk.dart`、`providers.dart`、2 個測試 | 兩支 App 的 Clerk session | 步驟 6 改寫成原生 SDK 加 token 交換 |
| `packages/hawkr_api` | 1 | `test/transport_test.dart` | 測試提到 | 步驟 6 |
| buyer-app | 19 | `clerk_session_provider.dart`、auth repository／controller、network interceptor、`fake_clerk_session` | App 登入 | 步驟 6 |
| seller-app | 28 | 同上，另有 `Makefile`、`tool/coverage_baseline.json` | App 登入 | 步驟 6（coverage baseline 依 CI 重算） |
| CI | 1 | `.github/workflows/website-e2e.yml:88-93` | 假的 `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY`、`CLERK_SECRET_KEY` | 步驟 3 刪除 |
| 文件 | 約 60 | `backend/docs/adr`、`backend/docs/plans`、模組 `AGENTS.md`／`CARRYOVER.md`、各 App README、根目錄 `AGENTS.md` 契約表 | 描述 Clerk | 步驟 5；ADR 只加取代註記，不改內文 |

## 可重用的既有程式

- `identity/internal/oidc`：在用。依 JWKS 驗 JWT，已有 Google、Apple 的 issuer；`provider` 拿它驗 Google、Apple 與 Cloudflare Access。LINE 的 ID token 要確認簽章演算法（步驟 1 的 spike）。
- `identity/internal/apple`：在用。client secret 簽發、授權碼交換、撤銷；`applerevoke` 經 `LinkAppleCredential`（`httpapi/identity/app.go:72`）呼叫。
- `VerifyGoogle`：在用，但只給廠商入口的二次驗證（`httpapi/agent/invitee.go:58`）。
- Clerk 之前的自建實作：舊 repo `../dailyfresh` 的 `a61dc78b02^`（`SignIn`、`SignOut`、`sessions`、`oauth_links`、`link_tokens` 各表與 queries）。只當參考，識別碼已改名、手機驗證與連結確認不再成立。
- identity 現有的表：`accounts`、`account_suspensions`、`account_roles`、`apple_tokens`、`admins`、`admin_roles`、`admin_role_events`（15 個 migration，`20260926174906_identity_schema` 到 `20261029150426_account_suspensions`）。
- 後端目前沒有 LINE 登入；LINE 現在經 Clerk 的 OAuth。
