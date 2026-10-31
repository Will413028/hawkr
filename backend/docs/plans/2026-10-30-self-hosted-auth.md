# 自建身分驗證取代 Clerk

**Goal：** 顧客以 Google、LINE 聯合登入，憑證是 identity 模組存在 Postgres 的 opaque session（web 是 httpOnly cookie，App 是 Bearer）；repo 裡沒有 Clerk 的依賴、設定、欄位與程式（收尾步驟的 `rg -il clerk` 檢查）。後台維持 Cloudflare Access。

**上層：** `2026-09-24-go-modular-monolith.md`（全域約束與完成定義）；決定來源是 `2026-10-18-a7-social.md` 的 A7-D19。主清單見本機個人脈絡檔。

**附件：** `2026-10-30-self-hosted-auth/`：`clerk-inventory.md`（Clerk 使用點盤點，快照 `9b133ff`）。

## 前提（開工時核對過，2026-10-30）

- 會改到或取代的既有部署是否在線：不在線（系統從未上線，根目錄 `AGENTS.md`「上線前不受相容性約束」）。不需要相容性窗口與資料搬移；開發資料裡以 `clerk_user_id` 建的帳號直接捨棄（seed 重建）。
- 不用 Clerk，改自建 Google、LINE 聯合登入加 Postgres opaque session，重用 `identity/internal/oidc`；後台維持 Cloudflare Access（A7-D19，Will 2026-10-30；已確認）。
- 不提供 Sign in with Apple（Will 2026-10-30：沒有 Apple Developer 帳號，不為開發期付年費 99 美元）。修訂 A7-D19 的登入方式。既有的 Apple 程式（`internal/apple`、`applerevoke`、`apple_tokens`、`LinkAppleCredential`）在步驟 4 刪除。重新加入的觸發：App 要上架 App Store——App Store 審核準則 4.8 要求提供第三方登入的 App 也提供一個保護隱私的登入選項（推論自準則原文，上架前核對），且上架本身就需要付費帳號。
- 帳號只在 Google 回報 email 已驗證時自動連結；LINE 只能登入後手動連結（A7-D19，Will 2026-10-30；已確認）。
- 後端與 website 一波切換，不留同時接受新舊憑證的相容層；兩支 App 在步驟 6 改完之前本機登不進（A7-D19，Will 2026-10-30；已確認）。
- 停權者的授權依動作性質（`internal/httpapi/effects.go`，`withSession` 是唯一執行點），和用哪一家登入無關，沿用（A7-D18，Will 2026-10-30；已確認）。
- 方法或設計取捨交給 Will 之前，先寫方法簡報（前提題、業界基準、REAL／SELF-IMPOSED 約束帳）並由 fresh subagent 批判（Will 2026-10-30；已確認）。
- 安全參數照業界預設（預設，可推翻）：session token 32 bytes CSPRNG、只存 SHA-256，閒置逾時 web 7 天、App 14 天、絕對逾時 90 天、續期一天最多寫一次（ADR `2026-09-27-identity-opaque-sessions` D2；[OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)）；web 的 OAuth 是 authorization code ＋ PKCE（S256）＋ `state` ＋ `nonce`（[RFC 9700](https://www.rfc-editor.org/rfc/rfc9700) §2.1）；cookie `__Host-hk_session`、`Secure`、`HttpOnly`、`SameSite=Lax`；改變狀態的請求檢查 `Origin`（[OWASP CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)）；登入後換發新 session（防 session fixation）。
- 架構（Will 2026-10-30 依方法簡報確認）：website 經同源反代連單體、單體發 httpOnly cookie session，就是 BFF（[draft-ietf-oauth-browser-based-apps](https://datatracker.ietf.org/doc/draft-ietf-oauth-browser-based-apps/) 的 BFF 形狀：OAuth 由伺服器端 confidential client 做，瀏覽器只有 session cookie），不另立 BFF 服務。另立 BFF 的觸發：API 要給第三方用、web 與 API 不再同源、或要代表使用者長期呼叫第三方 API。provider 的 token 登入後即丟，不保存。
- 安全細節（Will 2026-10-30 確認；來源 [RFC 9700](https://www.rfc-editor.org/rfc/rfc9700)、OWASP、ASVS 5.0）：
  - mix-up 防護（RFC 9700 §4.4.2）：每家獨立 redirect URI，伺服器端暫存紀錄記預期的 provider；回應有 `iss` 就驗（是否每家都送由 spike 確認），ID token 的 `iss`／`aud` 一律驗。
  - `state` 存伺服器端暫存紀錄、單次使用、失敗即作廢；PKCE S256 與 `nonce` 都帶。
  - 登入後的返回網址只收 `/` 開頭且非 `//` 的相對路徑，存在暫存紀錄，不經 query 回傳。
  - CSRF：cookie 認證的非安全方法檢查 `Sec-Fetch-Site` 為 `same-origin`（瀏覽器沒送時退回 `Origin`），Bearer 不檢查；登出只收 POST；callback 是 GET（Google、LINE 都用 query 回傳），不需要 CSRF 例外。
  - Google 自動連結條件：`email_verified` 為真，且 email 是 `gmail.com` 或 token 帶 `hd`（nOAuth 教訓）。
  - 登入相關回應 `Cache-Control: no-store`；登入頁 `frame-ancestors 'none'`；start、callback、token 交換每 IP 限流（單體內的限流器）。
  - 不做（上線前過度設計）：獨立登入事件表（結構化 log 即可）、異常偵測與新裝置通知、session 綁裝置（DPoP、DBSC）。
- provider 的事實（spike 結果，附件 `spikes.md`，2026-10-30 查官方文件；標「未確認」的仍待 1a 實測）：
  - LINE：web 登入的 ID token 是 HS256（以 channel secret 驗），原生 SDK 的是 ES256（以 `https://api.line.me/oauth2/v2.1/certs` 驗），驗證器依來源分兩條；discovery 只列 ES256，不能只照 discovery。PKCE 只收 S256、支援 nonce。同一個 provider 底下的 channel `sub` 才相同：web 與 App 的 channel 必須在同一個 LINE provider，之後不能換。
  - Apple（不做，留作重新加入時的參考）：要 name／email 就必須 `form_post`；redirect URI 不能是 IP 或 `localhost`，本機驗收要一個真的 HTTPS 網域（tunnel）；discovery 沒宣告 PKCE、授權參數也沒有 `code_challenge`，所以 Apple 這條靠 `state` 與 `nonce`，不以 PKCE 當防線。伺服器通知是 Apple 簽的 JWT（以 `https://appleid.apple.com/auth/keys` 驗），事件是 `email-disabled`、`email-enabled`、`consent-revoked`、`account-deleted`。「web 登入要先有上架 App Store 的 App」出自官方設定頁，但是否要求「上架」而不只是存在的 App ID，未確認。
  - Google：`http://localhost` 可當 redirect URI；身分鍵用 `sub`；三家只有 Google 在授權回應帶 `iss`（三份 discovery 實抓）。iOS client 的 ID token `aud` 是哪個 client ID，未確認。
- 工具鏈（預設，可推翻）：OAuth 用 `golang.org/x/oauth2`，ID token 用既有的 `internal/oidc`；本機開發與 e2e 繼續用 dev token（`provider/dev.go`），改成簽發 dev session。

**驗收基線：** 開工時以 `9b133ff` 為準：CI 6/6 綠（A7-1d″ 收尾時的紀錄，本計畫開工時重看一次各 job 結論）。本機基線在步驟 2 開工時跑 backend monolith 的 gofmt、vet、`go test ./...`、`-tags integration -p 2`、`make lint-monolith`，既有失敗記在進度表，不在本階段修。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 憑證只在 `withSession` 解析一次，之後只看 principal | `internal/httpapi/session.go`、`rpc/session.go`、`effects.go` 的動作性質宣告、`rpc.CheckDeclarations`、舊 gateway 轉送（`/api/v1/*`） |
| 停權、角色、刪帳以 identity 的資料為準，下一個請求就生效 | `account_suspensions`、`account_roles`、`accountsuspension`、`accountdeletion` 模組、`GetSession` |
| 其他模組只以 identity 的 account ID 參照帳號 | buyer、seller、social（`2026-10-17-social-actor-identity`）、通知的 email 來源（clerk ADR 2026-10-05 修訂） |
| website 經第一方同源反代打單體（`/api/connect/*`、`/api/v1/*`） | `__Host-` cookie 落在 web host；website middleware 判斷登入；SSE、媒體上傳、AI chat 的 transport |

## 範圍

- 做：新 ADR；後端的 session、聯合登入（web redirect 流程與 App 的 token 交換）、手動連結、登出；website 切換；刪除 Clerk；兩支 App 改用原生 SDK；文件。
- 不做：passkey、MFA、密碼、email 驗證碼（Clerk ADR D2 起就不提供，需求沒變）；後台的登入（維持 Access）。
- 做（Will 2026-10-30）：使用者檢視自己的 session 並在重新驗證後終止（ASVS 5.0 L2，條號待開工時對照原文）。
- 延後：
  - 解除連結（觸發：有使用者需要換掉某個登入方式；那時決定「最後一個登入方式不能解除」等規則）。
  - passkey（觸發：要提供不依賴第三方的登入）。
  - session 讀取的快取（觸發：量測顯示 session 讀取是熱路徑瓶頸；opaque-sessions ADR 的重新評估條件）。
  - Sign in with Apple（觸發：App 要上架 App Store；見前提段）。
  - Swift 原生 App 的登入（觸發：Swift 版開工）。
  - App 內連結登入方式（`LinkIdentity`；web 已有；觸發：App 要進入測試發佈，Will 2026-10-31）。
  - 部署環境的第一方反代確認 3xx、`Location` 與多個 `Set-Cookie` 原樣轉給瀏覽器（觸發：第一次部署 website；系統從未部署，步驟 3 時沒有對象可驗，本機代理已實測；Will 2026-10-31）。

## 步驟

步驟 1a、1b 完成前不開步驟 2。步驟 2、3、4 必須連著做：步驟 2 合進 main 後 website 就登不進，到步驟 3 完成為止。

- [x] **1a. 前置驗證**（被擋於：無）
  - 範圍：
    - spike（產出事實，不是產品程式）：
      - a. LINE Login v2.1：ID token 的簽章（web channel 是否 HS256＋channel secret、原生 SDK 是否 ES256＋JWKS）、`nonce`、email 要申請的權限與沒給 email 時的行為、`sub` 在同一 provider 底下跨 channel 是否相同。
      - b. （取消：不提供 Sign in with Apple，見前提段。）
      - c. Google：web client 的 `localhost` redirect、`email_verified` 與 `hd` 的語意。
      - d. website 在 Cloudflare Workers（OpenNext）的同源反代：`Set-Cookie` 與 302 是否原樣轉給瀏覽器、`__Host-` 是否成立；Next.js 的 server component、SSR、server action 在伺服器端呼叫單體時怎麼帶 cookie、帶不帶得到 `Origin`／`Sec-Fetch-Site`（決定 D6）。
      - e. 各家授權回應是否帶 `iss`（RFC 9207）。
      - f. 開工前跑一次 `govulncheck ./...`（backend）當基線，結果寫進 `spikes.md`。
  - 消費端：無（spike）。
  - 不能動：產品程式碼。spike 的程式放 `2026-10-30-self-hosted-auth/spike/`，結論寫進 ADR 後刪除。
  - 驗收：每個 spike 一段「事實、來源（官方文件 URL 或實測指令與輸出）、對設計的影響」寫在附件 `spikes.md`。
  - 停止條件：LINE 的事實讓「帳號只在 Google 的 email 已驗證時自動連結」或「web 用 redirect 流程」做不到時，停下帶事實回報 Will。
  - 需要人做的事：Will 登入 Google Cloud Console 與 LINE Developers（建立開發用 OAuth client 由 agent 操作瀏覽器完成，secret 直接存進本機 `secrets/`，不經對話）。

- [x] **1b. 新 ADR**（被擋於：步驟 1a、D2–D6）
  - 範圍：
    - 方法簡報（前提題、業界基準、REAL／SELF-IMPOSED 約束帳），fresh subagent 批判後交 Will 拍板 D2–D6。
    - 寫新 ADR（decision-log skill）取代 `2026-09-27-identity-clerk.md`：寫明它的各修訂哪些沿用（後台 Access、受限登入、email 當通知寄件地址改為登入時取得），哪些作廢（D1–D8 的 Clerk 部分、job `identity.clerk_user`、2026-10-08 的 Apple 授權撤銷修訂：不再提供 Apple 登入）；`2026-09-27-identity-opaque-sessions.md` 的狀態改成依 D1 的結論恢復。兩份舊 ADR 只改狀態列並加取代註記。
    - A7-D17「登出是 client 端，無伺服器 RPC」改為伺服器端 `SignOut`（刪 session 列）：在 `2026-10-18-a7-social.md` 的決定表加修訂（日期、Will），ADR 一併寫明。
  - 消費端：無（文件）。
  - 驗收：ADR 照 second-brain `.agents/guidance/adr-gate.md` 的清單逐項勾；A7 計畫的 A7-D17 列有修訂日期。

- [x] **2. 後端：session 與聯合登入**（被擋於：步驟 1b）
  - 範圍：
    - schema（identity 模組，Atlas migration）：`sessions`（token 雜湊、account、client kind、建立、最後使用、絕對到期）、`account_identities`（provider、`sub`、UNIQUE(provider, sub)、email、`email_verified`）、web 登入流程的暫存狀態（D3：單次使用的一列）、App 換 session 用的單次 nonce（D2）。
    - identity 公開 API：解析 session（一併讀帳號狀態與角色）、以 provider 的身分登入或建帳（自動連結只在 Google 且 `email_verified` 為真時，比對既有帳號的已驗證 email）、手動連結（已登入，provider 的 `sub` 已屬別的帳號時回錯誤）、登出（刪列）；停權、刪帳刪掉該帳號全部 session（同一個 transaction）。
    - 入口：web 的 `GET /api/auth/{provider}/start`、`GET /api/auth/{provider}/callback`（純 HTTP handler，不走 Connect；依 D3），callback 設 `__Host-hk_session` 後 302 回 website；App 的 token 交換 RPC（原生 SDK 給的 ID token／授權碼換 session，依 D2）；`SignOut`、`LinkIdentity` RPC。
    - `rpc/session.go`：Bearer 或 `__Host-hk_session`，其他位置不收；`__session` 刪除。CSRF（D6）：只對以 cookie 認證的請求檢查——非 GET／HEAD 的請求 `Sec-Fetch-Site` 必須是 `same-origin`，瀏覽器沒送時 `Origin` 必須是 website 的 origin，兩者都沒有就拒絕；cookie 請求只有宣告 `NO_SIDE_EFFECTS` 的 procedure 能走 GET；以 Bearer 認證的請求（App、admin BFF 轉的 Access JWT）不帶 ambient credential，不檢查。
    - start／callback 是純 HTTP 路由，不在 `rpc.CheckDeclarations` 的對照內：在 `internal/httpapi` 以測試列出全部 `/api/auth/*` 路由並斷言它們不經 `withSession` 的寫入路徑；`ratelimit.go` 為它們加每 IP 的限流；CORS 不放行（只走同源反代）。
    - 舊 gateway：`backend/internal/legacy/gatewayproxy` 照舊只轉單體解析出的 principal，不把 `__Host-hk_session` 或 Bearer 原樣轉出。
    - session 管理：`ListSessions`、`RevokeSession`（終止前要求 15 分鐘內重新以任一已連結的 provider 驗證過，依 ASVS）。
    - 動作性質宣告表（`effects.go`）補新 procedure：登入與 token 交換是公開、`SignOut`、`LinkIdentity`、`ListSessions`、`RevokeSession` 是自助。
    - dev token 改成直接建 dev session（`make dev-token` 介面不變）。
  - 消費端：`rg -n 'withSession|rpc\.Session|SessionFrom|__session' backend/internal`；`rg -n 'GetSession' api website/lib buyer-app/lib seller-app/lib packages`；舊 gateway 轉送 `rg -n 'Authorization|Cookie' backend/internal/legacy/gatewayproxy`。
  - 不能動：`effects.go` 的語意與 `withSession` 是唯一執行點；account ID；`GetSession` 的回應形狀（A7-1d″ 的巢狀 message）。
  - 驗收：backend monolith CI 等價（gofmt、vet、test、`-tags integration -p 2`、`make lint-monolith` 0 issues、`GOWORK=off go mod tidy` 無 diff、`buf lint`；`buf breaking` 照 `api-contract.yml` 現行設定跑，契約有破壞性變更時照上線前規則在 commit 訊息寫明）。integration 測試：兩家各一條登入建帳、Google 已驗證 email 自動連結、Google 未驗證與 LINE 相同 email 不連結而建新帳、手動連結衝突、停權後 session 讀到停權狀態且寫入回 `ACCOUNT_SUSPENDED`、刪帳後 session 失效、閒置與絕對逾時、`state`／`nonce`／PKCE 不符拒絕、cookie 請求 `Sec-Fetch-Site` 為 cross-site 或 `Origin` 不符時拒絕、Bearer 請求放行、重放 `state` 被拒、返回網址 `//evil` 與絕對網址被拒、登出用 GET 被拒、Google 非 gmail 且無 `hd` 不自動連結、mix-up（以 A 家的 code 打 B 家的 callback）被拒、未重新驗證時 `RevokeSession` 被拒、登入回應帶 `no-store`、`/api/auth/*` 超過限流回 429。mutation：拿掉 CSRF 檢查要紅、callback 不比對登入前 cookie 要紅（login CSRF）、App 交換不比對 nonce 要紅、cookie 請求的寫入 procedure 改走 GET 被放行要紅、`state` 改成可重用要紅、返回網址不檢查要紅、自動連結拿掉 `hd`／gmail 條件要紅、拿掉預期 provider 比對要紅、自動連結條件拿掉 `email_verified` 要紅、自動連結放行 LINE 要紅、停權不刪 session 且解析不讀狀態要紅、不檢查 `state` 要紅、token 從 query 也收要紅、登入不換發新 session 要紅。
  - 停止條件：步驟 1 的 spike 結論與本步驟寫法不同時，先改計畫再做。
  - 完成後的狀態：website 與兩支 App 登不進（Clerk 的 token 不再被接受），必須接著做步驟 3。

- [x] **3. website 切換**（被擋於：步驟 2）
  - 範圍：移除 `@clerk/nextjs` 與 `clerkMiddleware`；登入頁的兩個按鈕（Google、LINE；Apple 按鈕刪除）導向 `/api/auth/{provider}/start`，SSO callback 頁刪除；middleware 只看 `__Host-hk_session` 在不在做樂觀導向（不打 API），權威判斷是頁面與 client 呼叫 `GetSession`（收到 `unauthenticated` 導回登入）；`lib/auth/*` 改成以 `GetSession` 判斷登入、`SignOut` 登出；`lib/api`、`lib/connect`、SSE、AI chat、媒體上傳的 transport 不再帶 Bearer（同源 cookie）；`next.config.ts` 的 CSP 拿掉 Clerk 網域；`lib/config/env.ts`、`website-e2e.yml` 的 `CLERK_*` 刪除；e2e helper `clerk-session.ts` 換成 session cookie 的 mock；本機代理 `website/app/api/connect/[...path]/route.ts:23` 與 `website/app/api/v1/[...path]/route.ts:282` 的 `fetch` 加 `redirect: "manual"`，3xx 與 `Location` 原樣交給瀏覽器（現在會自己跟隨，OAuth 的 302 到不了瀏覽器）；伺服器端身分改成轉送 session cookie：`website/features/seller-center/profile/actions.ts:21`（Clerk `auth()`）、`website/lib/api/client.ts:490`（Bearer），依 D6；帳號設定加「連結 Google／LINE」（UI 改動，畫面先給 Will 看過）。
  - 消費端：`rg -il clerk website .github/workflows`（盤點 72 檔）；Storybook `rg -n 'clerk' website/.storybook`。
  - 不能動：畫面版面與既有文案（連結區以外）；同源反代的路徑。
  - 驗收：website CI 等價的 lint、typecheck、`test:unit`（node:test 指令）、vitest、`pnpm run build`（帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值 `NEXT_PUBLIC_API_BASE_URL`）、`design:check --apps-root ..`；e2e 與 Storybook 交給 CI（`website-e2e.yml`），回報為待 CI。真人登入：Will 在本機以兩家各登入一次、登出、連結一次。mutation：transport 又帶 Bearer 而 cookie 缺時要紅（未登入 e2e）、middleware 不認 cookie 要紅。
  - 需要人做的事：Will 的真人登入驗收；連結區的畫面同意；畫面完成後 Will 以登入畫面截圖申請 LINE email 權限，核准後實測「拒給 email」並把結果補進 `spikes.md`；部署環境的第一方反代確認 3xx 與多個 `Set-Cookie` 原樣轉給瀏覽器。
  - 完成後的狀態：web 可登入；兩支 App 仍登不進。

- [x] **4. 後端刪除 Clerk**（被擋於：步驟 3）
  - 範圍：刪 `internal/clerk`、`Clerk`／`ClerkIdentity` 型別、River job `identity.clerk_user`（含佇列裡殘留的 job：上線前，直接清）、`accounts.clerk_user_id` 與其索引（migration）、`readClerk` 與 `CLERK_*`、`scripts/clerk-env.sh`、`Makefile:260-313` 的 Clerk 部分、seed 改用 `account_identities`；刪 Apple 登入的程式：`internal/apple`、`applerevoke`、`apple_tokens` 表（migration）、`LinkAppleCredential`／`RevokeAppleSignIn` 等 proto 與四端的呼叫、`provider` 的 Apple verifier、`internal/oidc` 的 Apple issuer 常數；`arch/modules.go` 的規則。
  - 消費端：`rg -il clerk backend -g '!**/docs/**' -g '!*.md'`；Apple `rg -n -i 'LinkAppleCredential|RevokeAppleSignIn|applerevoke|apple_tokens|AppleIssuer|internal/apple' api backend/internal backend/cmd website/lib website/features buyer-app/lib seller-app/lib packages`（排除 Apple Pay：`-g '!**/applepay/**'`，Apple Pay 與登入無關，不動）。
  - 不能動：`internal/oidc` 給 Access 與廠商入口用的部分。
  - 驗收：同步驟 2 的 backend CI 等價；`rg -il clerk backend -g '!**/docs/**' -g '!*.md'` 與上面的 Apple 搜尋都沒有結果；`atlas migrate validate`。
  - 完成後的狀態：後端與 website 完成切換。

- [x] **5. 文件**（被擋於：步驟 4）
  - 範圍：`docs/modules/identity.md`、identity 的 `CARRYOVER.md` 沿用盤點（session、`internal/oidc` 寫「從零設計會怎麼做」）、`backend/AGENTS.md`、根目錄 `AGENTS.md` 的身分驗證契約列、admin 文件、wiki `local-runbook.md`「Clerk 真人登入」改寫（second-brain 端）、本機個人脈絡檔的 Clerk 條目。
  - 消費端：`rg -il clerk -g '*.md' . -g '!backend/docs/adr/**' -g '!backend/docs/plans/**' -g '!buyer-app/**' -g '!seller-app/**' -g '!packages/hawkr_auth/**'`（App 的文件在步驟 6 改）。
  - 驗收：上面那個 rg 只剩明確標為歷史的段落；`python3 scripts/check-agents-md.py`。

- [x] **6a. Flutter 升到 3.47.7**（被擋於：無）
  - 範圍：所有 `.fvmrc`（兩支 App 與 `packages/` 各套件）從 3.41.2 升到 3.47.7（2026-10-08 stable，Dart 3.13.5；`flutter_line_sdk` 3.0.0 要 Flutter ≥3.44，Will 2026-10-31 選升級而非用 2.7.2），重解 `pubspec.lock`，修新版 analyzer／lint 的發現，文件裡寫死的版本一併改；不新增或升級其他依賴（`pub get` 被迫升級時停下回報）。
  - 消費端：`find . -name .fvmrc -not -path '*/node_modules/*'`；`rg -l '3\.41\.2' -g '!*.lock'`；CI 經 `.fvmrc` 讀版本（`_flutter-quality.yml`、`api-contract.yml`、`dart-packages.yml`）。
  - 驗收：每個 Flutter 專案 analyze、custom_lint、守門測試、`deadcode.sh`、build_runner 生成比對；完整測試與 coverage 交 CI，CI 全綠才開 6b。

- [ ] **6b. 兩支 App 改原生登入**（被擋於：步驟 6a、D2）
  - 範圍：`packages/hawkr_auth` 改用原生 SDK（`google_sign_in`、LINE SDK；`sign_in_with_apple` 移除，依 D2）取得 ID token／授權碼，呼叫步驟 2 的 token 交換 RPC，session token 存 secure storage；`clerk_auth` 移除；兩支 App 與 `hawkr_auth` 的 README、ARCHITECTURE 一併改；兩支 App 的 session provider、interceptor、測試 fake 改寫；收到 `unauthenticated` 回登入頁。
  - 消費端：`rg -il clerk packages buyer-app seller-app`（盤點 56 檔）。
  - 驗收：實測 Google iOS client 的 ID token `aud`，結果補進 `spikes.md` 並對齊步驟 2 接受的 audience；兩支 App 的 analyze、custom_lint、改到的測試加以字串讀原始碼的守門測試、`bash scripts/deadcode.sh`、build_runner 生成比對（diff 有 `part` 宣告的檔時）；完整測試與 coverage ratchet 交給 CI（seller 依 CI 的 artifact 重算 baseline）。真機或模擬器登入：Will。mutation：interceptor 不帶 Bearer 要紅、token 存非 secure storage 的守門測試要紅。
  - 需要人做的事：Will 以模擬器或真機兩家各登入一次；iOS 拿掉 Sign in with Apple capability，加 LINE 的 URL scheme。
  - 已定（Will 2026-10-31）：依賴 `google_sign_in` 7.2.0、`flutter_line_sdk` 3.0.0，移除 `clerk_auth`、`sign_in_with_apple`、`flutter_web_auth_2`；Google 的 nonce 只能經 `GoogleSignIn.instance.initialize(nonce:)` 帶，每次登入先取新 nonce 再重新 initialize（文件寫只呼叫一次，iOS 實作只覆寫設定；以測試守住順序，升 SDK 時重驗）；App 內連結登入方式（`LinkIdentity`）不在本步，見範圍的「延後」。兩支 App 沒有 `android/`，只做 iOS。

- [ ] **7. 收尾**：完成定義逐項核對；獨立設計審查（design-review skill）；`rg -il clerk -g '!node_modules' -g '!gen/**' -g '!*.g.dart' -g '!pnpm-lock*' . -g '!backend/docs/adr/**' -g '!backend/docs/plans/**'` 沒有結果，盤點每列去向填完；延後項附觸發條件搬回主清單；需要人做的事列給 Will（刪除開發用 Clerk instance 是不可逆的對外動作，列給 Will 做）。

## 決定

- **D1 恢復 opaque-sessions ADR 的哪幾條**（狀態：已決 2026-10-30，Will；擋住步驟：1；需要的事實：無）
  - 背景：A7-D19 寫「恢復 D1、D2、D4、D5」，但那份 ADR 的 D4 是手機 OTP 的儲存、D5 是三竹簡訊；手機驗證與三竹已隨 Clerk ADR 刪除（「新帳號不用驗手機」，Will 2026-09-27），本計畫的登入方式也沒有手機。
  - A：恢復 D1（憑證）、D2（期限）、D3 全文（撤銷即刪列）；D4、D5 維持作廢——符合現在的登入方式（建議）。
  - B：照字面恢復 D4、D5——要重新引入三竹與手機驗證，與 A7-D19 的登入方式矛盾。
  - 結論：A。恢復 D1、D2、D3 全文，D4、D5 維持作廢；ADR：步驟 1b 的新 ADR。

- **D2 App 怎麼取得 provider 的身分**（狀態：已決 2026-10-30，Will；擋住步驟：2、6；需要的事實：spike a）
  - A：原生 SDK（Google Sign-In、LINE SDK）在裝置上取得 ID token，App 呼叫 token 交換 RPC 換 session——業界最常見（Firebase、Supabase 的原生登入都這樣），體驗最好；後端要接受各 provider 給原生 client 的 audience（建議）。
  - B：系統瀏覽器走與 web 相同的 authorization code＋PKCE 流程，callback 以 App 的 universal link 帶回一次性 code 再換 session（[RFC 8252](https://www.rfc-editor.org/rfc/rfc8252)）——後端只有一條流程；但 Google、LINE 原生 SDK 的帳號選擇體驗較好（LINE 可直接跳 LINE App 授權）。
  - 結論：A。後端先發單次 nonce；Google 帶不了 nonce 時改用短效 token＋單次使用紀錄（步驟 6 實測）；Google `aud`＝web client ID 加 `azp` 允許清單。ADR：`2026-10-30-identity-self-hosted-federated-login.md` D2。

- **D3 web 的登入流程放在哪一層**（狀態：已決 2026-10-30，Will；擋住步驟：2、3；需要的事實：spike d）
  - A：單體負責 start 與 callback（純 HTTP handler，經同源反代），流程暫存狀態放在短效（10 分鐘）的簽章 cookie 或 identity 的一列，callback 直接設 session cookie——憑證只由後端處理，website 沒有 secret（建議）。
  - B：website 的 route handler 做 OAuth，再以 ID token 呼叫單體換 session——website 要持有兩家的 client secret，Workers 上要多管一份 secret。
  - 結論：A，暫存放 identity 的一列（簽章 cookie 做不到單次使用，否決）；另設短效 `__Host-` 登入前 cookie 防 login CSRF。ADR：`2026-10-30-identity-self-hosted-federated-login.md` D3。

- **D4 LINE 沒有可用 email 時**（狀態：已決 2026-10-30，Will；擋住步驟：2；需要的事實：spike a）
  - 背景：通知以 email 寄送（clerk ADR 2026-10-05 修訂）；LINE 要申請權限、使用者可以不同意。
  - A：照樣建帳，email 留空，通知以 App 推播或站內為主，需要寄信時在帳號設定補填並驗證（建議；不擋登入）。
  - B：沒有 email 就要求使用者當場填寫並驗證——要做 email 驗證流程，與「沒有 email 驗證碼」的範圍衝突。
  - 結論：A，沿用 `2026-10-05-notification-module.md` 決策 8；寄件地址改為每次登入以已驗證 email 更新（取代 Clerk webhook）。ADR：`2026-10-30-identity-self-hosted-federated-login.md` D4。

- **D5 Apple 授權碼與 `LinkAppleCredential` 的去向**（狀態：作廢 2026-10-30，Will 決定不提供 Apple 登入；Apple 的程式在步驟 4 刪除）

- **D6 Next.js 伺服器端呼叫單體的寫入**（狀態：已決 2026-10-30，Will；擋住步驟：2、3；需要的事實：spike d）
  - 背景：伺服器端發出的請求沒有瀏覽器的 `Sec-Fetch-Site`／`Origin`，CSRF 檢查會擋下寫入。
  - A：伺服器端只做讀取（GET 與 Connect 的唯讀 procedure），寫入一律由瀏覽器直接經同源反代送出——CSRF 檢查不需例外（建議）。
  - B：server action 轉送瀏覽器原本的 `Origin`／`Sec-Fetch-Site` 與 cookie——單體要信任 Worker 轉來的 header，多一個要守的信任邊界。
  - 結論：A；cookie 請求只有 `NO_SIDE_EFFECTS` 的 procedure 能走 GET。ADR：`2026-10-30-identity-self-hosted-federated-login.md` D6。

拍板：D1 已決、D5 作廢；D2、D3、D4、D6 於 2026-10-30 依方法簡報（fresh subagent 批判後修正）由 Will 一次拍板，全採建議。

- **步驟 2 的實作決定**（2026-10-30，主 session 依 SSOT 裁決）
  - `accounts.clerk_user_id` 過渡期改 nullable，步驟 4 刪除。
  - 自動連結的目標只限符合資格的 Google identity（`auto_link_eligible`）。
  - 刪帳保留登入身分墓碑。
  - CSRF 疊加在既有 CrossOriginProtection 上。
  - session cookie Max-Age 90 天。
  - LINE web token 沒有 `email_verified`，不寫寄件地址；步驟 3 實測 email 權限時重看。
  - Google 授權回應缺 `iss` 一律拒絕。
  - 重新驗證＝重新登入。
  - 延後：Workspace 刪除使用者後，同地址的新 `sub` 會自動連到舊帳號（信任 `hd` 的固有限制），步驟 5 寫進 ADR。
  - 延後：過期的 session、flow、nonce 只機會式清除，上線前加排程清除 job。

- **步驟 3 的連結區**（2026-10-31，Will）：帳號設定「帳號安全」群組加「登入方式」，每家一列，已連結顯示「已連結」、未連結顯示「連結」按鈕（導向 `/api/auth/{provider}/start?link=1`），回跳帶 `link_error` 時區塊上方顯示錯誤列。為了顯示狀態，identity 新增唯讀 `IdentityAppService.ListLinkedIdentities`（只回 provider 與連結時間，不回 `sub`、email）。schema 允許同一帳號在同一家有多個身分（主鍵 `(provider, subject)`），所以每個身分回一筆、同家可多筆，client 依 provider 判斷（主session 依 schema 裁決，2026-10-31）。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1a 前置驗證 | 完成 | 官方文件查證（a、c、e）與 repo 代理程式閱讀（d 前半）、govulncheck 基線（f），見 `spikes.md`；開發用 OAuth client 已建（GCP 專案 `hawkr` 的 web 與 buyer／seller iOS client；LINE provider `Hawkr` 的 channel `Hawkr Login (dev)`，callback 與 iOS bundle ID 已設）；b 取消（不提供 Apple） | 移到後續步驟的實測：LINE 拒給 email 的回應（email 權限要附登入畫面截圖，步驟 3 做出畫面後申請，驗收併入步驟 3）；Google iOS `aud`（步驟 6）；部署環境反代對 3xx、多個 `Set-Cookie` 的處理（反代程式不在 repo，步驟 3 的部署驗收） | `5cfb680`、`584ead2` |
| 1b 新 ADR | 完成 | 方法簡報經 fresh subagent 批判（8 項全數處理）後 Will 拍板 D2–D4、D6；ADR `2026-10-30-identity-self-hosted-federated-login.md` 寫成，clerk ADR 標 superseded、opaque-sessions ADR 狀態改為 D1–D3 恢復；A7-D17 已修訂 | ADR gate 的獨立草稿未另跑（決策已經過方法簡報與 fresh 批判） | （本 commit） |
| 2 後端：session 與聯合登入 | 完成 | 本機 gofmt、vet（含 `-tags integration`）、`go test ./...`、`make lint-monolith` 0 issues、`go mod tidy` 無 diff、`atlas migrate validate`、`buf lint`、`buf breaking` 無破壞、TS／Dart 生成與 `@hawkr/api` lint／test、`hawkr_api` analyze、uitext 套件測試；unit 層 mutation 實跑（自動連結資格、返回網址、CSRF、login-CSRF cookie、mix-up、query token、gateway 刪憑證、Google 缺 iss、App nonce、azp、RevokeSession auth_time 等）；三個單元各經獨立審查，MED 全數修正（LINE email 預先劫持、Google 缺 iss）；CI（run 41078485573，`2fd62c5`）的 integration：identity、httpapi/buyer、httpapi/identity 的登入與 CSRF、gatewayproxy 全過（首輪 `8c8d62a` 的 10 個失敗皆為測試錯，`c6b9c13` 修正） | dev-token 端對端未跑（要 DB）；integration 層的 mutation 未逐項實跑；同 run 的 delegation `reason_code` CHECK 失敗不屬本步驟（`988f82c`，由該 session 處理） | `9288f89`、`1d67121`、`5feac69`、`765873c`、`e27adbf`、`9544216`、`c8d4730`、`8c8d62a`、`c6b9c13` |
| 3 website 切換 | 完成 | 主 session 重跑 website 的 lint、vitest、`test:unit`、`pnpm run build`（loopback API base）、`design:check --apps-root ..` 皆 exit 0；B1（`ListLinkedIdentities`）重跑 gofmt、vet（含 `-tags integration`）、`go test ./...`、`make lint-monolith` 0 issues、`buf lint`，`make sqlc` 以映像重生無 diff；mutation 實跑（transport 加回 Bearer、middleware 不認 cookie、代理拿掉 `redirect: "manual"`、連結 href 拿掉 `link=1`、`identity_already_linked` 對錯句、連結區移到別的 section、`ListLinkedIdentities` 的 effects 宣告與 account id）皆紅；三個單元各經獨立審查無 HIGH（MED：Apple→LINE 文案，Will 2026-10-31 同意；LOW：刪 Apple 撤銷指引，依不提供 Apple 接受）；`rg -il clerk website .github/workflows` 從 73 檔剩 3 檔（兩個斷言「沒有 Clerk」的測試、一處歷史註解）。Will 2026-10-31 同意：連結區版面、Apple→LINE 文案、登入錯誤分三句 | 部署環境反代對 3xx、多個 `Set-Cookie` 的處理移到範圍的「延後」（觸發：第一次部署）。已補：`ListLinkedIdentities` integration 在 CI 綠；e2e 與 Storybook 在 CI 綠（run 41094311718 之後 `ff1c5b9` 全綠；中途兩個修正：e2e 的 `__Host-` cookie 改由回應的 Set-Cookie 設，CDP `addCookies` 會被拒；繞過 `mockGateway` 的 spec 補 `GetSession` 的訪客 mock）；2026-10-31 本機真人驗收——Google 登入建帳、帳號設定連結 LINE、登出（`SignOut`，session 歸零）、LINE 登入落在同一帳號；LINE email 權限已 Applied，「拒給 email」實測見 `spikes.md` | `8dfeb8f`、`10bae63`、`bd34c2f`、`7bfa416`、`e507060`、`3fdff43`、`ff1c5b9` |
| 4 後端刪除 Clerk | 完成 | 兩個單元各經 Sonnet 實作、主 session 重跑驗收、獨立審查：4a 刪 Clerk（`internal/clerk`、job `identity.clerk_user`、`accounts.clerk_user_id` 的 migration、`CLERK_*`、`clerk-env.sh`、Makefile、seed）；4b 刪 Apple 登入（`internal/apple`、`applerevoke`、`apple_tokens` 的 migration、`APPLE_*`、`IDENTITY_TOKEN_KEYS`／`Sealer`、identity 的 job worker、proto `LinkAppleCredential` 與三端生成、`hawkr_api` 與兩支 App 的交接呼叫；Apple Pay 未動）。主 session 重跑 gofmt、vet（含 `-tags integration`）、`go test ./...`、`make lint-monolith` 0 issues、`atlas migrate validate`、`buf lint`、`buf breaking`（v1alpha1 依 `ignore_unstable_packages` 不檢查）、兩支 App 的 analyze 與 deadcode、`hawkr_api` analyze；mutation 實跑（加回 `CLERK_*`／`APPLE_*` 讀取、拿掉 dev 金鑰與 Access 互斥、dev-token 對不存在帳號默默建立、seed 插入登入身分、`effects.go` 加回已刪 procedure）皆紅；審查 4a 2 個 LOW（過時註解，4b 修正）、4b 無缺陷。CI（`73d7453`）全綠，含 integration；首輪 `c7f4e35` 的 integration 7 個失敗皆為測試 fixture 沒跟上 dev 以帳號 ID 識別、seller-app coverage baseline 依 CI artifact 重算、instruction-lint 指向已改名的測試，`ac62f05`、`ea31a43`、`73d7453` 修正。決定：dev 使用者以帳號 ID 識別、不建假登入身分（`make dev-token args='-account <id>'`，Will 2026-10-31） | 盤點指令 `rg -il clerk backend -g '!docs/**'` 沒排除 `backend/docs`，實際用 `-g '!**/docs/**' -g '!*.md'`，殘留只剩既有 migration、`reserved` 歷史註解、斷言「沒有 Clerk／Apple 設定」的測試與舊 migration 的回歸測試；文件（含 `identity/AGENTS.md`、`CARRYOVER.md`、dev-token 用法、`.agents/skills/backend-e2e-verify/SKILL.md`）是步驟 5 | `2a2d7e5`、`69603ee`、`6767eef`、`11351de`、`b9c4e55`、`ac62f05`、`ea31a43`、`73d7453` |
| 5 文件 | 完成 | Sonnet 改 28 檔，主 session 重跑：步驟 5 的 Clerk rg 只剩 4 檔（`hawkr_auth` 列寫明步驟 6、三份 CARRYOVER 的歷史標記），`dev-token` rg 全為現行用法，instruction-lint exit 0，second-brain `check-links.py`、`lint-rules.py` exit 0；抽查 cookie 名、provider 環境變數、`PUBLIC_WEB_ORIGIN` 預設、`dev-token` 用法與程式一致。ADR 自建登入補「已知限制」（Workspace 刪人後新 `sub` 自動連結） | 計畫寫的 `scripts/check-agents-md.py` 不存在，以 CI 的 instruction-lint 代替；`website/.env.example` 的 `NEXT_PUBLIC_APPLE_*`、`secrets/INDEX.md` 的 Apple 鍵與舊微服務文件的 Apple 引用不是本步範圍，留到收尾 | |
| 6a Flutter 升級 | 完成 | 三輪 Sonnet；7 份 `.fvmrc` 升 3.47.7；連帶升級（Will 2026-10-31 逐項同意）：`hawkr_auth` 的 `test` 1.31.1、`hawkr_ui_text` 的 `intl` 0.20.3、seller-app 的 `flutter_quill` ^11.6.0、`extended_text_field` ^17.0.0；新 lint 的最小修正（cart、image processing 的 try 內 return 外移，`onReorder`→`onReorderItem`）。主 session 重跑：兩支 App analyze exit 0、`hawkr_auth`、`hawkr_ui_text` 測試 exit 0、seller `test/shared/widgets` 233 過；subagent 回報：五個套件 analyze／測試、兩支 App custom_lint、守門測試、`deadcode.sh`、build_runner 穩定、website `app-catalog-build` 皆過 | 兩支 App 以 `analysis_options_deprecated_plugins: ignore` 壓 custom_lint 舊式 plugin 的警告（觸發移除：升 custom_lint 到支援新 plugin 系統的版本）；Flutter 3.47 的 `pub get`／analyze 會往 `analysis_options.yaml` 加 `analyzer.exclude`，CI 的生成比對因此紅，已把它提交；buyer-app 的 `dart format` 在 Dart 3.13 多改一個檔；CI 首輪紅三處（生成比對、buyer `dart format`、OSV 新增 npm `glob`／`minimatch` medium，放基準至 2027-01-29），seller coverage baseline 依 CI artifact 重算（少 5 行已涵蓋的程式）；`9029b17` 全綠 | `1f20294`、`663c2dc`、`1c69f19`、`9029b17` |
| 6b 兩支 App | 程式完成待 CI 與真機 | Sonnet 實作＋一輪修正；主 session 重跑：`hawkr_auth` 24 測試、兩支 App analyze／format exit 0、改到的測試加守門測試（buyer 526、seller 270）全過、`rg -il clerk packages buyer-app seller-app` 0 筆；mutation：Dio 與 Connect 不帶 Bearer、token 非 secure storage、略過 Google nonce 重新 initialize 皆紅（Connect 那條是主 session 發現缺測試後補的） | App 個人資料頁不再顯示登入 email（identity 不把 email 給 App）；LINE 取消的錯誤碼判斷依 SDK 原始碼，待真機；Google iOS token 的 `aud`／`azp` 實測與 `spikes.md` 待真機；seller coverage baseline 待 CI artifact | |
| 7 收尾 | 未開始 | | | |
