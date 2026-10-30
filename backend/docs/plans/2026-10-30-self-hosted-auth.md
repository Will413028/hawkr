# 自建身分驗證取代 Clerk

**Goal：** 顧客以 Google、Apple、LINE 聯合登入，憑證是 identity 模組存在 Postgres 的 opaque session（web 是 httpOnly cookie，App 是 Bearer）；repo 裡沒有 Clerk 的依賴、設定、欄位與程式（收尾步驟的 `rg -il clerk` 檢查）。後台維持 Cloudflare Access。

**上層：** `2026-09-24-go-modular-monolith.md`（全域約束與完成定義）；決定來源是 `2026-10-18-a7-social.md` 的 A7-D19。主清單見本機個人脈絡檔。

**附件：** `2026-10-30-self-hosted-auth/`：`clerk-inventory.md`（Clerk 使用點盤點，快照 `9b133ff`）。

## 前提（開工時核對過，2026-10-30）

- 會改到或取代的既有部署是否在線：不在線（系統從未上線，根目錄 `AGENTS.md`「上線前不受相容性約束」）。不需要相容性窗口與資料搬移；開發資料裡以 `clerk_user_id` 建的帳號直接捨棄（seed 重建）。
- 不用 Clerk，改自建 Google、Apple、LINE 聯合登入加 Postgres opaque session，重用 `identity/internal/oidc` 與 `internal/apple`；後台維持 Cloudflare Access（A7-D19，Will 2026-10-30；已確認）。
- 帳號只在 Google 回報 email 已驗證時自動連結；LINE、Apple 只能登入後手動連結（A7-D19，Will 2026-10-30；已確認）。
- 後端與 website 一波切換，不留同時接受新舊憑證的相容層；兩支 App 在步驟 6 改完之前本機登不進（A7-D19，Will 2026-10-30；已確認）。
- 停權者的授權依動作性質（`internal/httpapi/effects.go`，`withSession` 是唯一執行點），和用哪一家登入無關，沿用（A7-D18，Will 2026-10-30；已確認）。
- 方法或設計取捨交給 Will 之前，先寫方法簡報（前提題、業界基準、REAL／SELF-IMPOSED 約束帳）並由 fresh subagent 批判（Will 2026-10-30；已確認）。
- 安全參數照業界預設（預設，可推翻）：session token 32 bytes CSPRNG、只存 SHA-256，閒置逾時 web 7 天、App 14 天、絕對逾時 90 天、續期一天最多寫一次（ADR `2026-09-27-identity-opaque-sessions` D2；[OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)）；web 的 OAuth 是 authorization code ＋ PKCE（S256）＋ `state` ＋ `nonce`（[RFC 9700](https://www.rfc-editor.org/rfc/rfc9700) §2.1）；cookie `__Host-hk_session`、`Secure`、`HttpOnly`、`SameSite=Lax`；改變狀態的請求檢查 `Origin`（[OWASP CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)）；登入後換發新 session（防 session fixation）。
- 工具鏈（預設，可推翻）：OAuth 用 `golang.org/x/oauth2`，ID token 用既有的 `internal/oidc`；本機開發與 e2e 繼續用 dev token（`provider/dev.go`），改成簽發 dev session。

**驗收基線：** 開工時以 `9b133ff` 為準：CI 6/6 綠（A7-1d″ 收尾時的紀錄，本計畫開工時重看一次各 job 結論）。本機基線在步驟 2 開工時跑 backend monolith 的 gofmt、vet、`go test ./...`、`-tags integration -p 2`、`make lint-monolith`，既有失敗記在進度表，不在本階段修。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 憑證只在 `withSession` 解析一次，之後只看 principal | `internal/httpapi/session.go`、`rpc/session.go`、`effects.go` 的動作性質宣告、`rpc.CheckDeclarations`、舊 gateway 轉送（`/api/v1/*`） |
| 停權、角色、刪帳以 identity 的資料為準，下一個請求就生效 | `account_suspensions`、`account_roles`、`accountsuspension`、`accountdeletion` 模組、`GetSession` |
| 其他模組只以 identity 的 account ID 參照帳號 | buyer、seller、social（`2026-10-17-social-actor-identity`）、通知的 email 來源（clerk ADR 2026-10-05 修訂） |
| website 經第一方同源反代打單體（`/api/connect/*`、`/api/v1/*`） | `__Host-` cookie 落在 web host；website middleware 判斷登入；SSE、媒體上傳、AI chat 的 transport |
| 刪帳時撤銷 Apple 授權（App Store 5.1.1(v)） | `apple_tokens`、`internal/apple`、`applerevoke`、`LinkAppleCredential` |

## 範圍

- 做：新 ADR；後端的 session、聯合登入（web redirect 流程與 App 的 token 交換）、手動連結、登出；website 切換；刪除 Clerk；兩支 App 改用原生 SDK；文件。
- 不做：passkey、MFA、密碼、email 驗證碼（Clerk ADR D2 起就不提供，需求沒變）；後台的登入（維持 Access）。
- 延後：
  - 解除連結（觸發：有使用者需要換掉某個登入方式；那時決定「最後一個登入方式不能解除」等規則）。
  - 「登出所有裝置」與裝置清單（觸發：產品要求或安全事件處理需要）。
  - session 讀取的快取（觸發：量測顯示 session 讀取是熱路徑瓶頸；opaque-sessions ADR 的重新評估條件）。
  - Swift 原生 App 的登入（觸發：Swift 版開工）。

## 步驟

步驟 1a、1b 完成前不開步驟 2。步驟 2、3、4 必須連著做：步驟 2 合進 main 後 website 就登不進，到步驟 3 完成為止。

- [ ] **1a. 前置驗證**（被擋於：無）
  - 範圍：
    - spike（產出事實，不是產品程式）：
      - a. LINE Login v2.1：ID token 的簽章（web channel 是否 HS256＋channel secret、原生 SDK 是否 ES256＋JWKS）、`nonce`、email 要申請的權限與沒給 email 時的行為、`sub` 在同一 provider 底下跨 channel 是否相同。
      - b. Sign in with Apple 的 web 流程：Services ID、`response_mode=form_post`（callback 是跨站 POST，`SameSite=Lax` 的狀態 cookie 帶不過去，要確認 `state` 的保存方式）、return URL 是否接受 `localhost`／非 HTTPS（本機驗收怎麼做）、姓名只在第一次給。
      - c. Google：web client 的 `localhost` redirect、`email_verified` 與 `hd` 的語意。
      - d. website 在 Cloudflare Workers（OpenNext）的同源反代：`Set-Cookie` 與 302 是否原樣轉給瀏覽器、`__Host-` 是否成立。
  - 消費端：無（spike）。
  - 不能動：產品程式碼。spike 的程式放 `2026-10-30-self-hosted-auth/spike/`，結論寫進 ADR 後刪除。
  - 驗收：每個 spike 一段「事實、來源（官方文件 URL 或實測指令與輸出）、對設計的影響」寫在附件 `spikes.md`。
  - 停止條件：LINE 或 Apple 的事實讓「帳號只在 Google 的 email 已驗證時自動連結」或「web 用 redirect 流程」做不到時，停下帶事實回報 Will。
  - 需要人做的事：Will 在 Google Cloud Console、Apple Developer（Services ID、Key）、LINE Developers 建立開發用的 OAuth client；client secret 與 Apple 私鑰放本機 `secrets/`，回覆 `done`。

- [ ] **1b. 新 ADR**（被擋於：步驟 1a、D1–D5）
  - 範圍：
    - 方法簡報（前提題、業界基準、REAL／SELF-IMPOSED 約束帳），fresh subagent 批判後交 Will 拍板 D2–D5（D1 可在 1a 之前拍）。
    - 寫新 ADR（decision-log skill）取代 `2026-09-27-identity-clerk.md`：寫明它的各修訂哪些沿用（後台 Access、Apple refresh token 由伺服器保存並在刪帳時撤銷、受限登入、email 當通知寄件地址改為登入時取得），哪些作廢（D1–D8 的 Clerk 部分、job `identity.clerk_user`）；`2026-09-27-identity-opaque-sessions.md` 的狀態改成依 D1 的結論恢復。兩份舊 ADR 只改狀態列並加取代註記。
    - A7-D17「登出是 client 端，無伺服器 RPC」改為伺服器端 `SignOut`（刪 session 列）：在 `2026-10-18-a7-social.md` 的決定表加修訂（日期、Will），ADR 一併寫明。
  - 消費端：無（文件）。
  - 驗收：ADR 照 second-brain `.agents/guidance/adr-gate.md` 的清單逐項勾；A7 計畫的 A7-D17 列有修訂日期。

- [ ] **2. 後端：session 與聯合登入**（被擋於：步驟 1b）
  - 範圍：
    - schema（identity 模組，Atlas migration）：`sessions`（token 雜湊、account、client kind、建立、最後使用、絕對到期）、`account_identities`（provider、`sub`、UNIQUE(provider, sub)、email、`email_verified`）、web 登入流程的暫存狀態（依 D3）。
    - identity 公開 API：解析 session（一併讀帳號狀態與角色）、以 provider 的身分登入或建帳（自動連結只在 Google 且 `email_verified` 為真時，比對既有帳號的已驗證 email）、手動連結（已登入，provider 的 `sub` 已屬別的帳號時回錯誤）、登出（刪列）；停權、刪帳刪掉該帳號全部 session（同一個 transaction）。
    - 入口：web 的 `GET /api/auth/{provider}/start`、`GET|POST /api/auth/{provider}/callback`（純 HTTP handler，不走 Connect；依 D3），callback 設 `__Host-hk_session` 後 302 回 website；App 的 token 交換 RPC（原生 SDK 給的 ID token／授權碼換 session，依 D2）；`SignOut`、`LinkIdentity` RPC。
    - `rpc/session.go`：Bearer 或 `__Host-hk_session`，其他位置不收；`__session` 刪除。CSRF：只對以 cookie 認證的請求檢查——非 GET／HEAD 的請求 `Origin` 必須是 website 的 origin，缺 `Origin` 拒絕；以 Bearer 認證的請求（App、admin BFF 轉的 Access JWT）不帶 ambient credential，不檢查。
    - Apple（依 D5）：web callback 與 App 的 token 交換都把授權碼以 `internal/apple` 換 refresh token 存進 `apple_tokens`，`LinkAppleCredential` 併入登入流程後刪除。
    - start／callback 是純 HTTP 路由，不在 `rpc.CheckDeclarations` 的對照內：在 `internal/httpapi` 以測試列出全部 `/api/auth/*` 路由並斷言它們不經 `withSession` 的寫入路徑；`ratelimit.go` 為它們加每 IP 的限流；CORS 不放行（只走同源反代）。
    - 舊 gateway：`backend/internal/legacy/gatewayproxy` 照舊只轉單體解析出的 principal，不把 `__Host-hk_session` 或 Bearer 原樣轉出。
    - 動作性質宣告表（`effects.go`）補新 procedure：登入與 token 交換是公開、`SignOut` 與 `LinkIdentity` 是自助。
    - dev token 改成直接建 dev session（`make dev-token` 介面不變）。
  - 消費端：`rg -n 'withSession|rpc\.Session|SessionFrom|__session' backend/internal`；`rg -n 'GetSession' api website/lib buyer-app/lib seller-app/lib packages`；舊 gateway 轉送 `rg -n 'Authorization|Cookie' backend/internal/legacy/gatewayproxy`；Apple `rg -n 'LinkAppleCredential|applerevoke' api backend/internal website/lib buyer-app/lib seller-app/lib packages`。
  - 不能動：`effects.go` 的語意與 `withSession` 是唯一執行點；account ID；`GetSession` 的回應形狀（A7-1d″ 的巢狀 message）。
  - 驗收：backend monolith CI 等價（gofmt、vet、test、`-tags integration -p 2`、`make lint-monolith` 0 issues、`GOWORK=off go mod tidy` 無 diff、`buf lint`；`buf breaking` 照 `api-contract.yml` 現行設定跑，契約有破壞性變更時照上線前規則在 commit 訊息寫明）。integration 測試：三家各一條登入建帳、Google 已驗證 email 自動連結、Google 未驗證與 LINE、Apple 相同 email 不連結而建新帳、手動連結衝突、停權後 session 讀到停權狀態且寫入回 `ACCOUNT_SUSPENDED`、刪帳後 session 失效、閒置與絕對逾時、`state`／`nonce`／PKCE 不符拒絕、cookie 請求 `Origin` 不符或缺少時拒絕、Bearer 請求不帶 `Origin` 放行、web 與 App 的 Apple 登入後 `apple_tokens` 有列、刪帳排撤銷 job、`/api/auth/*` 超過限流回 429。mutation：拿掉 Origin 檢查要紅、Apple 登入不存 refresh token 要紅、自動連結條件拿掉 `email_verified` 要紅、自動連結放行 LINE 要紅、停權不刪 session 且解析不讀狀態要紅、不檢查 `state` 要紅、token 從 query 也收要紅、登入不換發新 session 要紅。
  - 停止條件：步驟 1 的 spike 結論與本步驟寫法不同時，先改計畫再做。
  - 完成後的狀態：website 與兩支 App 登不進（Clerk 的 token 不再被接受），必須接著做步驟 3。

- [ ] **3. website 切換**（被擋於：步驟 2）
  - 範圍：移除 `@clerk/nextjs` 與 `clerkMiddleware`；登入頁的三個按鈕導向 `/api/auth/{provider}/start`，SSO callback 頁刪除；middleware 只看 `__Host-hk_session` 在不在做樂觀導向（不打 API），權威判斷是頁面與 client 呼叫 `GetSession`（收到 `unauthenticated` 導回登入）；`lib/auth/*` 改成以 `GetSession` 判斷登入、`SignOut` 登出；`lib/api`、`lib/connect`、SSE、AI chat、媒體上傳的 transport 不再帶 Bearer（同源 cookie）；`next.config.ts` 的 CSP 拿掉 Clerk 網域；`lib/config/env.ts`、`website-e2e.yml` 的 `CLERK_*` 刪除；e2e helper `clerk-session.ts` 換成 session cookie 的 mock；帳號設定加「連結 LINE／Apple／Google」（UI 改動，畫面先給 Will 看過）。
  - 消費端：`rg -il clerk website .github/workflows`（盤點 72 檔）；Storybook `rg -n 'clerk' website/.storybook`。
  - 不能動：畫面版面與既有文案（連結區以外）；同源反代的路徑。
  - 驗收：website CI 等價的 lint、typecheck、`test:unit`（node:test 指令）、vitest、`pnpm run build`（帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值 `NEXT_PUBLIC_API_BASE_URL`）、`design:check --apps-root ..`；e2e 與 Storybook 交給 CI（`website-e2e.yml`），回報為待 CI。真人登入：Will 在本機以三家各登入一次、登出、連結一次；Apple 依 spike b 的結論在本機或以 tunnel 的 HTTPS 網址驗，兩者都不行時 Apple 的真人登入列為待驗並寫進進度表。mutation：transport 又帶 Bearer 而 cookie 缺時要紅（未登入 e2e）、middleware 不認 cookie 要紅。
  - 需要人做的事：Will 的真人登入驗收；連結區的畫面同意。
  - 完成後的狀態：web 可登入；兩支 App 仍登不進。

- [ ] **4. 後端刪除 Clerk**（被擋於：步驟 3）
  - 範圍：刪 `internal/clerk`、`Clerk`／`ClerkIdentity` 型別、River job `identity.clerk_user`（含佇列裡殘留的 job：上線前，直接清）、`accounts.clerk_user_id` 與其索引（migration）、`readClerk` 與 `CLERK_*`、`scripts/clerk-env.sh`、`Makefile:260-313` 的 Clerk 部分、seed 改用 `account_identities`；`arch/modules.go` 的規則。
  - 消費端：`rg -il clerk backend -g '!docs/**'`。
  - 不能動：`internal/oidc` 給 Access 與廠商入口用的部分。
  - 驗收：同步驟 2 的 backend CI 等價；`rg -il clerk backend -g '!docs/**'` 沒有結果；`atlas migrate validate`。
  - 完成後的狀態：後端與 website 完成切換。

- [ ] **5. 文件**（被擋於：步驟 4）
  - 範圍：`docs/modules/identity.md`、identity 的 `CARRYOVER.md` 沿用盤點（session、`internal/oidc`、`internal/apple` 各寫「從零設計會怎麼做」）、`backend/AGENTS.md`、根目錄 `AGENTS.md` 的身分驗證契約列、admin 文件、wiki `local-runbook.md`「Clerk 真人登入」改寫（second-brain 端）、本機個人脈絡檔的 Clerk 條目。
  - 消費端：`rg -il clerk -g '*.md' . -g '!backend/docs/adr/**' -g '!backend/docs/plans/**' -g '!buyer-app/**' -g '!seller-app/**' -g '!packages/hawkr_auth/**'`（App 的文件在步驟 6 改）。
  - 驗收：上面那個 rg 只剩明確標為歷史的段落；`python3 scripts/check-agents-md.py`。

- [ ] **6. 兩支 App**（被擋於：步驟 4、D2）
  - 範圍：`packages/hawkr_auth` 改用原生 SDK（`google_sign_in`、`sign_in_with_apple`、LINE SDK，依 D2）取得 ID token／授權碼，呼叫步驟 2 的 token 交換 RPC，session token 存 secure storage；`clerk_auth` 移除；兩支 App 與 `hawkr_auth` 的 README、ARCHITECTURE 一併改；兩支 App 的 session provider、interceptor、測試 fake 改寫；收到 `unauthenticated` 回登入頁。
  - 消費端：`rg -il clerk packages buyer-app seller-app`（盤點 56 檔）。
  - 驗收：兩支 App 的 analyze、custom_lint、改到的測試加以字串讀原始碼的守門測試、`bash scripts/deadcode.sh`、build_runner 生成比對（diff 有 `part` 宣告的檔時）；完整測試與 coverage ratchet 交給 CI（seller 依 CI 的 artifact 重算 baseline）。真機或模擬器登入：Will。mutation：interceptor 不帶 Bearer 要紅、token 存非 secure storage 的守門測試要紅。
  - 需要人做的事：Will 以模擬器或真機三家各登入一次；iOS 的 Apple capability 與 LINE 的 URL scheme 設定。

- [ ] **7. 收尾**：完成定義逐項核對；獨立設計審查（design-review skill）；`rg -il clerk -g '!node_modules' -g '!gen/**' -g '!*.g.dart' -g '!pnpm-lock*' . -g '!backend/docs/adr/**' -g '!backend/docs/plans/**'` 沒有結果，盤點每列去向填完；延後項附觸發條件搬回主清單；需要人做的事列給 Will（刪除開發用 Clerk instance 是不可逆的對外動作，列給 Will 做）。

## 決定

- **D1 恢復 opaque-sessions ADR 的哪幾條**（狀態：未決；擋住步驟：1；需要的事實：無）
  - 背景：A7-D19 寫「恢復 D1、D2、D4、D5」，但那份 ADR 的 D4 是手機 OTP 的儲存、D5 是三竹簡訊；手機驗證與三竹已隨 Clerk ADR 刪除（「新帳號不用驗手機」，Will 2026-09-27），本計畫的登入方式也沒有手機。
  - A：恢復 D1（憑證）、D2（期限）、D3 全文（撤銷即刪列）；D4、D5 維持作廢——符合現在的登入方式（建議）。
  - B：照字面恢復 D4、D5——要重新引入三竹與手機驗證，與 A7-D19 的登入方式矛盾。
  - 結論：（待 Will）

- **D2 App 怎麼取得 provider 的身分**（狀態：未決；擋住步驟：2、6；需要的事實：spike a）
  - A：原生 SDK（Google Sign-In、Sign in with Apple、LINE SDK）在裝置上取得 ID token（Apple 另帶授權碼），App 呼叫 token 交換 RPC 換 session——業界最常見（Firebase、Supabase 的原生登入都這樣），體驗最好；後端要接受各 provider 給原生 client 的 audience（建議）。
  - B：系統瀏覽器走與 web 相同的 authorization code＋PKCE 流程，callback 以 App 的 universal link 帶回一次性 code 再換 session（[RFC 8252](https://www.rfc-editor.org/rfc/rfc8252)）——後端只有一條流程；但 Apple 的 HIG 要求 iOS 上用原生按鈕，Google 原生 SDK 的帳號選擇體驗較好。
  - 結論：（待 Will）

- **D3 web 的登入流程放在哪一層**（狀態：未決；擋住步驟：2、3；需要的事實：spike b、d）
  - A：單體負責 start 與 callback（純 HTTP handler，經同源反代），流程暫存狀態放在短效（10 分鐘）的簽章 cookie 或 identity 的一列，callback 直接設 session cookie——憑證只由後端處理，website 沒有 secret（建議）。
  - B：website 的 route handler 做 OAuth，再以 ID token 呼叫單體換 session——website 要持有三家的 client secret，Workers 上要多管一份 secret。
  - 結論：（待 Will）

- **D4 Apple 與 LINE 沒有可用 email 時**（狀態：未決；擋住步驟：2；需要的事實：spike a、b）
  - 背景：通知以 email 寄送（clerk ADR 2026-10-05 修訂）；Apple 可能給 relay 地址，LINE 要申請權限、使用者可以不同意。
  - A：照樣建帳，email 留空，通知以 App 推播或站內為主，需要寄信時在帳號設定補填並驗證（建議；不擋登入）。
  - B：沒有 email 就要求使用者當場填寫並驗證——要做 email 驗證流程，與「沒有 email 驗證碼」的範圍衝突。
  - 結論：（待 Will）

- **D5 Apple 授權碼與 `LinkAppleCredential` 的去向**（狀態：未決；擋住步驟：2、6；需要的事實：spike b）
  - 背景：現在 App 登入 Apple 後另外呼叫 `LinkAppleCredential`（`httpapi/identity/app.go:72`）交出授權碼，伺服器換 refresh token 存 `apple_tokens`，刪帳時撤銷（clerk ADR 2026-10-08 修訂）。自建後授權碼本來就送到後端。
  - A：在登入流程內交換（web callback 收到 code、App 的 token 交換 RPC 帶 code），同一個 transaction 寫 `apple_tokens`；刪除 `LinkAppleCredential`——少一次 RPC，不會有「登入了卻沒存 token」的中間狀態（建議）。
  - B：保留 `LinkAppleCredential`，web 也另外呼叫——兩條路徑，web 要把 code 交還給前端。
  - 結論：（待 Will）

拍板順序：D1 現在可拍；D2–D5 等步驟 1a 的 spike 之後一起交，附方法簡報與 subagent 批判。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1a 前置驗證 | 未開始 | | | |
| 1b 新 ADR | 未開始 | | | |
| 2 後端：session 與聯合登入 | 未開始 | | | |
| 3 website 切換 | 未開始 | | | |
| 4 後端刪除 Clerk | 未開始 | | | |
| 5 文件 | 未開始 | | | |
| 6 兩支 App | 未開始 | | | |
| 7 收尾 | 未開始 | | | |
