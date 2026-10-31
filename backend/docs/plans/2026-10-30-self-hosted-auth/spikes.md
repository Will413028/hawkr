# Step 1a spikes：官方文件查證（2026-10-30）

證據等級：「實測」= 本次 curl 拿到的原始回應；「文件」= 官方頁面原文；「論壇」= Apple Developer Forums（非規格）；「未在官方文件找到」照字面。

## a. LINE Login v2.1

事實
- ID token 簽章：web login 回 **HS256**（用 channel secret 驗）；native app／LINE SDK／LIFF 回 **ES256**（用 JWKS `https://api.line.me/oauth2/v2.1/certs` 依 `kid` 驗）。來源：https://developers.line.biz/en/docs/line-login/verify-id-token/
- 但 discovery 的 `id_token_signing_alg_values_supported` 只列 `["ES256"]`（實測 https://access.line.me/.well-known/openid-configuration）。跟上一條矛盾，以 verify-id-token 頁為準。
- Verify endpoint：`POST https://api.line.me/oauth2/v2.1/verify`，參數 `id_token`、`client_id`，回傳 claims（含 `sub`，授權時含 `email`）；也可以自己驗。來源同上。
- nonce：可選，「returned in an ID token」。PKCE：支援，`code_challenge_method` 只收 `S256`；不帶就不走 PKCE。來源：https://developers.line.biz/en/docs/line-login/integrate-line-login/ ；discovery 實測 `code_challenge_methods_supported: ["S256"]`。
- Email 權限：要在 Console（Basic settings > OpenID Connect > Apply）申請，附上「怎麼收集、怎麼用 email」的截圖，通過後才顯示 Applied。同意畫面上 email 沒有標「(Required)」，使用者可以不給；授權了 `email` 也不會出現在 `scope` 回傳值裡。來源：integrate-line-login 頁。使用者拒絕時具體回什麼：**未在官方文件找到**（只說 app 要處理只授權部分權限的情況）。
- `sub`（userId）：同一個 provider 底下的 channel 給同一個使用者的 ID 相同，不同 provider 就不同；channel 不能換 provider。來源：https://developers.line.biz/en/docs/messaging-api/getting-user-ids/ 、https://developers.line.biz/en/docs/line-login/getting-started/ （WebSearch 摘要，沒逐字核對原頁）。
- Authorization response 帶 `iss`：文件列的參數只有 `code`、`state`、`friendship_status_changed`、`liffClientId`、`liffRedirectUri`，**沒有 iss**。discovery 沒有 `authorization_response_iss_parameter_supported`（實測）。

對設計的影響
- LINE 驗章要分兩條路：web 用 HS256＋channel secret，App 用 ES256＋JWKS。同一個驗證器不能只看 discovery。web 那條也可以不驗 ID token 簽章，改成 token endpoint 走 TLS 直接拿 ID token（OIDC Core 3.1.3.7 允許）。
- web 跟 App 要用**同一個 provider** 底下的 channel，`sub` 才會一致；(provider, sub) 當外部身分鍵。
- email 可能拿不到。帳號模型不能假設 LINE 一定給 email。
- 實測（2026-10-31，channel `Hawkr Login (dev)` 的 email 權限為 Applied，本機 `http://localhost:3000`）：同意畫面上 Email address 是可關的開關，其餘兩項標 Required。關掉 email 後按 Allow：callback 照常 302、登入成功（`identity.sessions` 新增一列），`identity.account_identities` 的 LINE 列 `email` 為 NULL、`email_verified=f`，沒有錯誤也沒有另建帳號。拒給 email 不是錯誤回應，只是 ID token 沒有 `email` claim；符合 D4（照樣建帳、email 留空）。

## b. Sign in with Apple（web）

事實
- 前提：web 要先有一個上架 App Store、啟用 Sign in with Apple 的 App；Services ID 綁那個 primary App ID，在上面設定 registered domains 和 return URLs；return URL 必須是絕對網址，要有 scheme、host、path。來源：https://developer.apple.com/documentation/signinwithapple/configuring-your-environment-for-sign-in-with-apple.md
- `redirect_uri`：「must include a domain name and can't be an IP address or `localhost`」，也不能帶 `#`。來源：https://developer.apple.com/documentation/signinwithapple/incorporating-sign-in-with-apple-into-other-platforms.md 。允不允許 http：**未在官方文件找到**（範例全是 https）。domain verification（要不要放驗證檔）：本次沒在上述頁面找到明文，**未在官方文件找到**。
- `response_mode`：「If you requested any scopes, the value must be `form_post`」。來源同上。
- 回應（POST）帶 `code`、`id_token`、`state`、`user`（JSON 字串，含 name）。「name 只在第一次授權給」：**未在官方文件找到**明文，只有論壇在說（https://developer.apple.com/forums/thread/118209）。
- `iss` in authorization response：文件的回應欄位沒有；discovery 沒有 `authorization_response_iss_parameter_supported`（實測）。
- PKCE：discovery **沒有** `code_challenge_methods_supported`（實測 https://appleid.apple.com/.well-known/openid-configuration ）；授權請求參數文件也沒列 `code_challenge`。→ 官方沒有宣告支援。
- nonce：支援，「associate a client session with an ID token … mitigate replay attacks」。來源：incorporating 頁。
- Server-to-server notifications：在 Certificates, Identifiers & Profiles 的 App ID 設定裡填 endpoint，要 TLS 1.2 以上，同一個 URL 可以給多個 team 和 app 共用。POST body 是 `{"payload":"<JWT>"}`，JWS 由 Apple 私鑰簽，依 header `alg` 驗、用 Apple 公鑰（`https://appleid.apple.com/auth/keys`）驗；`iss` 是 `https://appleid.apple.com`；事件放在 `events` claim 的 `type`：`email-disabled`、`email-enabled`、`consent-revoked`、`account-deleted`（注意：不是舊名 `account-delete`，論壇也回報實際收到的是 `account-deleted`）。email 的兩個事件只會發給選了 Hide My Email 的使用者（WWDC20）。來源：https://developer.apple.com/documentation/signinwithapple/processing-changes-for-sign-in-with-apple-accounts.md 、https://developer.apple.com/documentation/signinwithapple/configuring-your-environment-for-sign-in-with-apple.md 、https://developer.apple.com/videos/play/wwdc2020/10173/ 。重試政策：**未在官方文件找到**。
- Revoke：`POST https://appleid.apple.com/auth/revoke`（form-urlencoded：`client_id`、`client_secret`（JWT）、`token`、`token_type_hint`），需要有效的 refresh token 或 access token；沒有就先拿 code 換。來源：https://developer.apple.com/documentation/signinwithapplerestapi/revoke-tokens.md

對設計的影響
- web callback 一定是**跨站 POST（form_post）**：callback 要收 POST body；state／PKCE 的 cookie 要 `SameSite=None; Secure`，不然跨站 POST 時瀏覽器不會帶（這是業界常識，推論）；或者改把 state 綁在伺服器端、用 state 查。
- 本機開發沒辦法用 localhost 測 Apple web：要一個真網域（tunnel 或 preview 網域）。
- PKCE 不能當 Apple 的唯一防線：state＋nonce 要必驗；client_secret JWT 是 confidential client。
- 註冊時第一次拿到的 `user`（name）要當場存。
- 要存 Apple refresh token（加密）才能在刪帳號時 revoke（App Review 5.1.1(v) 要求）；要做 notifications endpoint，處理四種事件，`consent-revoked`／`account-deleted` 當成撤銷所有 session。

## c. Google

事實
- Web client redirect URI：必須 https，「Localhost URIs (including localhost IP address URIs) are exempt」，可以用 `http://localhost:8080` 測。來源：https://developers.google.com/identity/protocols/oauth2/web-server
- `email_verified`：「True if the user's email address has been verified」。`hd`：限制網域時「must check this claim」，沒有這個 claim 代表不是 Google hosted domain 帳號。不要拿 email 當唯一識別，「Always use the `sub` field」。來源：https://developers.google.com/identity/openid-connect/openid-connect
- Authorization response 帶 `iss`：OIDC 文件範例只有 `state`、`code`、`scope`；但 discovery **宣告** `authorization_response_iss_parameter_supported: true`（實測 https://accounts.google.com/.well-known/openid-configuration ）。PKCE：discovery 宣告 `plain`、`S256`（實測）。
- Native `aud`：`aud` 必須是你 app 的某一個 client ID（OIDC 頁）。Android 用 `requestIdToken(<web client ID>)`，後端用 WEB_CLIENT_ID 驗 → aud = web client ID。來源：https://developers.google.com/identity/sign-in/android/backend-auth 。iOS 的 aud（iOS client ID 還是 serverClientID）：**未在官方文件找到**（本次沒查到）。

對設計的影響
- 後端驗 Google ID token 時，aud 要接受一組白名單（web client ID，iOS 可能還要加上 iOS client ID），並且把 Android／iOS SDK 都設定成 serverClientID＝web client ID 讓 aud 統一（建議，iOS 那邊待查）。
- 只有 `email_verified=true` 才可以拿 email 做帳號比對或合併；身分鍵用 `sub`。

## d. OpenNext／Cloudflare Workers 與 repo 現況

事實（官方）
- OpenNext 的 route handler 怎麼處理多個 Set-Cookie、會不會原樣傳 302 Location：**未在官方文件找到**（本次沒查 opennext.js.org）。Workers 的 `Headers.getSetCookie()` 是 Workers runtime 支援的標準 API（推論，未查證）。

事實（repo，實測讀檔）
- 部署環境：`/api/connect/*`、`/api/v1/*` 由「web host 的第一方反向代理」處理（去掉 `/api/connect` 前綴轉到單體根目錄；`/api/v1` 原樣轉送）；Next 的 route 只在本機／Playwright 代行，部署環境一律 404。`website/lib/connect/transport.ts:39-44`、`website/app/api/connect/[...path]/route.ts:1-15`。那個反向代理的程式碼在哪裡：repo 裡沒找到（只有註解提到；`rg -n "api/" website/wrangler.jsonc` 沒有命中）。
- `next.config.ts:438-450` 的 rewrites 只有 Apple Pay domain association，沒有 API rewrite；`app/api/v1/[...path]/route.ts:2-3` 的註解說「next.config rewrites silently drop Set-Cookie from external origins」，所以才用 route handler。
- Set-Cookie：兩個 route 都用 `upstream.headers.getSetCookie()` 逐筆 append（`app/api/connect/[...path]/route.ts:38-42`、`app/api/v1/[...path]/route.ts:299-309`）。
- **302 Location：兩個 route 的 `fetch` 都沒設 `redirect: "manual"`**（`app/api/connect/[...path]/route.ts:23-28`、`app/api/v1/[...path]/route.ts:282-287`；`rg -n redirect` 在兩個檔案裡都沒有命中）→ 預設是 follow，上游回的 302 會被代理自己跟過去，瀏覽器收不到 Location。
- 伺服器端呼叫：`lib/api/client.ts:416-437` 在 server（`typeof window === "undefined"`）時加上 `ORIGIN_AUTH_HEADER` 簽章（rate-limit 主體證明）；登入憑證走 `withClerkAuthorization`（`lib/api/client.ts:490`、`:508`）；server action 用 Clerk `auth()`（`features/seller-center/profile/actions.ts:21`）；`lib/api/home-bootstrap.server.ts` 是 server-only 的 bootstrap。

對設計的影響
- OAuth 的 start（302 到 IdP）和 callback（Set-Cookie＋302 回站內）如果經過本機代理，要把 `redirect: "manual"` 加進兩個 route，否則本機流程會壞；部署環境要確認那個反向代理（不在 repo 裡）會原樣傳 3xx 和多個 Set-Cookie，這是 1a 還沒驗到的地方。
- 另一個選項：callback 直接打單體的路徑（同源前綴），不經 Next。
- 換成自建 session 以後，server 端呼叫要從 Clerk `auth()`／Bearer 改成轉送 session cookie；`client.ts` 和 server actions 都是要改的地方。

## e. RFC 9207 與 discovery

事實
- RFC 9207：authorization response 加一個 `iss` 參數，client 拿它比對預期的 AS issuer，用來防 mix-up 攻擊（client 接多個 AS 時，攻擊者把一個 AS 的 code 送到另一個 AS 的流程）；AS 在 metadata 裡用 `authorization_response_iss_parameter_supported` 宣告；宣告了的話，client 必須檢查 `iss`（MUST），沒帶就拒絕。來源：https://www.rfc-editor.org/rfc/rfc9207 （依 RFC 內容整理，本次沒重抓原文）。
- 實測 discovery：Google `true`；Apple 沒有這個欄位；LINE 沒有這個欄位。

對設計的影響
- 三家只有 Google 會帶 `iss`。防 mix-up 的主要手段要改成**每家用不同的 redirect URI（per-provider callback path）＋ state 綁 provider**（RFC 9207 §1／OAuth Security BCP 列的替代做法）；Google 那條再多驗 `iss`。

## f. govulncheck 基線（2026-10-30，`9b133ff`）

指令：`cd backend && GOWORK=off go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`，exit 1，14 筆可到達的漏洞（GO-2026-6253 到 GO-2026-6700），都有修正版：

| 模組 | 現版 | 修正版 |
|---|---|---|
| `github.com/rabbitmq/amqp091-go` | v1.10.0 | v1.13.0（一筆 v1.14.0） |
| `golang.org/x/crypto` | v0.55.0 | v0.56.0 |
| `go.opentelemetry.io/otel/sdk` | v1.44.0 | v1.45.0 |
| `github.com/moby/go-archive` | v0.2.0 | v0.3.0 |

對設計的影響：不在本計畫修（依賴升級交給供應鏈掃描的 Pending）；`x/crypto` 在步驟 2 開工前升到修正版，因為登入流程會用到。

## g. App 原生登入實測（2026-10-31，iOS 模擬器 iPhone 17／iOS 26.0，dev flavor，`cfbe20b` 加 h2c `324e46b`）

單體以 `GOOGLE_APP_CLIENT_IDS`（buyer、seller 的 iOS client）啟動；App 經 `IssueAppLoginNonce` 取 nonce 後走原生 SDK，`ExchangeIdToken` 換 session，`GetSession` 帶 Bearer。

| App | 動作 | 結果（單體 log／DB） |
|---|---|---|
| buyer | Google（Gmail 帳號，與 web 測試的帳號不同） | `ExchangeIdToken` ok，建帳號 2；登入身分有 email |
| buyer | 登出 | `SignOut` ok，帳號 2 的 session 歸零 |
| buyer | LINE（與 web 步驟 3 同一個 LINE 使用者） | 落在既有帳號 1，不建帳；這次同意了 email，身分補上 email |
| seller | Google（同 buyer 的 Gmail） | 落在帳號 2；沒有賣家角色，導到「建立店舖」 |
| seller | 後端刪 session 後重開 | `GetSession` 回 `unauthenticated`，App 清 token 回登入頁 |
| seller | LINE | 落在帳號 1 |

- Google iOS client 的 ID token：單體只接受 `aud`＝web client 且 `azp` 在 `GOOGLE_APP_CLIENT_IDS` 的 token，四次交換都通過，所以 `aud` 是 web client、`azp` 是 iOS client（由驗證規則推得，未解開 token 檢視）。
- Google 的 nonce 每次登入重新 `initialize` 後帶得進 ID token（交換時 nonce 比對通過）。
- 首次實測發現 App 的 Connect（HTTP/2）打不到本機單體：`:8080` 原本只開 HTTP/1.1，`324e46b` 讓主 listener 也收 h2c。
