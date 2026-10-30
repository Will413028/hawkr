# 身分驗證自建：Google、LINE 聯合登入，憑證是 identity 的 opaque session，OAuth 由單體做

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-10-30 |
| 決定者 | Will（A7-D19；D1–D6 依方法簡報與 fresh subagent 批判拍板，2026-10-30） |
| 影響範圍 | identity 模組、`internal/httpapi` 的認證與 CSRF；website 的登入與 transport；`packages/hawkr_auth` 與兩支 App |
| 上層決策 | `2026-09-24-go-modular-monolith.md` D1、D5；`2026-09-26-external-contract-protobuf-connect.md` D6 |

取代 `2026-09-27-identity-clerk.md`（全文，沿用部分見「沿用與作廢」）。恢復 `2026-09-27-identity-opaque-sessions.md` 的 D1（憑證）、D2（期限）、D3 全文（撤銷即刪列）；它的 D4、D5（手機 OTP、三竹簡訊）維持作廢。計畫：`docs/plans/2026-10-30-self-hosted-auth.md`。

## 脈絡

Clerk 時期的問題（A7-D19，Will 2026-10-30）：session 期限要付費方案才能拉長、`clerk_flutter` 是社群 beta、停權與刪帳要靠 job 同步到第三方、web 的短效 token 放在 JS 讀得到的地方。改回自建的前提：

- 系統未上線，不受相容性約束；後端與 website 一波切換，不留相容層。
- 只提供 Google、LINE。不提供 Sign in with Apple：沒有付費的 Apple Developer 帳號（Will 2026-10-30）。
- 後台維持 Cloudflare Access，不在本決策範圍。

約束：

- 外部：LINE web channel 的 ID token 是 HS256（channel secret），原生 SDK 是 ES256（JWKS）；同一個 LINE provider 底下的 channel `sub` 才相同；使用者可以拒給 email（`docs/plans/2026-10-30-self-hosted-auth/spikes.md` a）。
- 外部：三家只有 Google 在授權回應帶 `iss`（RFC 9207，spike e）。
- 外部：Google 在嵌入式 WebView 擋 OAuth。
- 繼承：其他模組只以 account ID 參照帳號；停權、角色、刪帳下一個請求就生效；`withSession` 是唯一的授權執行點（A7-D18）。

## 考慮過的方案

| 題目 | 方案 | 代價 | 結論 |
|---|---|---|---|
| 身分提供者 | 續用 Clerk | 期限要付費、Flutter SDK 是 beta、撤銷靠外部同步 | 否決（A7-D19） |
| | 自建聯合登入＋opaque session | 自己負責 OAuth 的安全細節 | 採用 |
| D2 App 取得身分 | 原生 SDK 拿 ID token，換 session | 後端多一條驗證路徑（LINE ES256、Google `azp`） | 採用：Firebase、Supabase、Auth0 的原生登入都這樣；帳號選擇與 LINE 跳 App 的體驗最好 |
| | 系統瀏覽器＋PKCE，以自訂 scheme 或 universal link 帶回 | 後端只一條流程；但 Google 選帳號、LINE 跳 App 的體驗較差，universal link 還要另設關聯檔 | 否決 |
| D3 web 的 OAuth 在哪層 | 單體的 start／callback（經同源反代） | 依賴反代原樣傳 3xx 與多個 `Set-Cookie` | 採用：做 OAuth 的是持有 session 與帳號的那層（Devise、allauth、BFF 模式的共通原則），website 不持 secret |
| | website route handler 做 OAuth，再以 ID token 換 session | Workers 多持兩家 secret；cookie 與 session 由不同層發，憑證邊界跨兩層 | 否決 |
| D3 流程暫存 | identity 的一列（單次使用） | 多一張短效表 | 採用：`state` 單次使用需要伺服器端狀態 |
| | 簽章 cookie | 無法單次使用，重放只能靠過期 | 否決 |
| D4 LINE 沒給 email | 照樣建帳，只收站內通知 | 只用 LINE 的人收不到交易 email | 採用：沿用 `2026-10-05-notification-module.md` 決策 8「沒有 email 的帳號只收站內通知，註冊時不強迫補填」 |
| | 登入當下強制填寫並驗證 | 要做 email 驗證碼，超出範圍 | 否決 |
| D6 Next 伺服器端寫入 | 伺服器端只讀，寫入由瀏覽器直接送 | 未來想做無 JS 表單時要改 | 採用：現況伺服器端對單體的寫入 0 處（推論：唯一的 server action 只做 `revalidateTag`） |
| | 以 Worker 簽章讓單體信任轉送的 `Origin` | 多一個信任邊界與 CSRF 例外 | 否決 |

## 決策

- **D1 憑證**：沿用 opaque-sessions ADR 的 D1–D3（32 bytes CSPRNG、只存 SHA-256；web `__Host-hk_session` httpOnly、`SameSite=Lax`，App `Authorization: Bearer`，其他位置不收；閒置 web 7 天、App 14 天，絕對 90 天；撤銷即刪列）。登入後一律換發新 session。provider 的 token 登入後即丟。
- **D2 App**：`hawkr_auth` 以原生 SDK（`google_sign_in`、`flutter_line_sdk`）取得 ID token，呼叫交換 RPC 換 session。後端先發短效、單次使用的 nonce，ID token 的 `nonce` 必須相符；Google 的 SDK 帶不了 nonce 時，改以「短效 token＋token 雜湊的單次使用紀錄」防重放（步驟 6 實測後定）。驗證：Google `iss` 接受 `https://accounts.google.com` 與 `accounts.google.com`、`aud` 為 web client ID、`azp` 在允許清單；LINE `aud` 為 channel ID、原生 ES256 以 JWKS 驗。
- **D3 web**：單體提供 `GET /api/auth/{provider}/start` 與 callback（純 HTTP handler，每家獨立路徑防 mix-up）。暫存列記 `state`、PKCE verifier、`nonce`、預期 provider、返回網址，10 分鐘、單次使用、失敗即作廢。start 另設短效 `__Host-` 登入前 cookie，callback 比對它與暫存列，防 login CSRF。返回網址只收 `/` 開頭且非 `//` 的相對路徑。
- **D4 email**：LINE 沒給 email 照樣建帳。寄件地址由每次登入取得的已驗證 email 更新（取代 Clerk `user.updated` webhook）。Google 只在 `email_verified` 為真且是 `gmail.com` 或帶 `hd` 時，以 email 自動連結既有帳號；LINE 只能登入後手動連結。
- **D5**：（作廢：Apple 授權碼的去向。不提供 Apple 登入，`apple_tokens`、`LinkAppleCredential` 與相關程式刪除。）
- **D6 CSRF**：以 cookie 認證的請求，非 GET／HEAD 必須 `Sec-Fetch-Site` 為 `same-origin`（瀏覽器沒送時 `Origin` 必須是 website 的 origin，兩者都沒有就拒絕）；cookie 請求只有宣告 `NO_SIDE_EFFECTS` 的 procedure 能走 GET。Bearer 不檢查。Next.js 伺服器端只讀。
- **D7 登出與 session 管理**：`SignOut` 是伺服器 RPC（刪 session 列；取代 A7-D17 的「登出是 client 端」）；`ListSessions`、`RevokeSession`（終止前 15 分鐘內要重新驗證）。停權、刪帳在同一個 transaction 刪該帳號全部 session。

## 沿用與作廢（`2026-09-27-identity-clerk.md`）

- 沿用：後台改用 Cloudflare Access 的修訂；受限登入的修訂（停權者可以登入，授權依動作性質，A7-D18）；「email 只供顯示、搜尋與寄信，不作識別或授權」；C9 修訂裡 Access 的驗證規則（jwx verifier、`iss` 精確比對、未知 kid 刷新限頻）。
- 作廢：D1–D8 的 Clerk 部分；job `identity.clerk_user`；`__session` cookie 邊界；2026-10-05 修訂的 webhook 同步（改為登入時更新，D4）；2026-10-08 Apple 授權由伺服器保存的修訂（不提供 Apple 登入）。

## 後果

- 正面：session 期限、撤銷、停權即時生效都在我們的 DB；web 長效憑證不讓 JS 讀到；不再有第三方使用者資料要同步。
- 負面：OAuth 的安全細節（mix-up、login CSRF、重放、返回網址）由我們維護，靠步驟 2 的 integration 與 mutation 測試守。部署環境的第一方反代必須原樣傳 3xx 與多個 `Set-Cookie`（步驟 3 驗收）。只用 LINE 登入且拒給 email 的使用者收不到交易 email。
- 兩支 App 在步驟 6 完成前登不進。

## 已知限制（2026-10-31 補記）

- Google Workspace 的管理員刪除一個使用者之後，同一個地址若再建立新帳號，Google 會給新的 `sub`。D4 對帶 `hd` 的 Google 身分以已驗證 email 自動連結既有帳號，所以新的 `sub` 會自動連到舊帳號，繼承舊帳號的資料與角色。這是信任 `hd`（組織替成員背書 email）的固有結果：我們分辨不出「同一個人」和「同地址的新使用者」。
- 現況接受這個限制：系統未上線，帶 `hd` 的 Workspace 登入沒有實際使用者；`gmail.com` 的地址由 Google 保證屬於該帳號本人，不受影響。
- 重新評估的觸發：開放給帶 `hd` 的 Workspace 使用者實際登入，或有使用者回報地址重新發放。屆時的選項：帶 `hd` 的身分不自動連結，只能登入後手動連結；或自動連結改要求使用者在舊帳號的有效 session 內確認。

## 重新評估的條件

- 要上架 App Store：加回 Sign in with Apple（App Store 審核準則 4.8）。
- web 與 API 不再同源、API 要給第三方用、或要代表使用者長期呼叫第三方 API：另立 BFF 或改由 website 做 OAuth。
- 要支援沒有原生 SDK 的 provider，或 provider 撤掉原生 ID token：App 改走系統瀏覽器＋PKCE。
- 要讓只用 LINE 的帳號收交易 email：做補填與驗證 email 的流程。
- 出現必須由 Next.js 伺服器端寫入的需求：重議 D6。
- 量測顯示 session 讀取是熱路徑瓶頸：加 session 快取。

## 相關

- 計畫與事實：`docs/plans/2026-10-30-self-hosted-auth.md`、附件 `spikes.md`、`clerk-inventory.md`
- 決定來源：`docs/plans/2026-10-18-a7-social.md` A7-D17、A7-D18、A7-D19
- 被取代：`2026-09-27-identity-clerk.md`；部分恢復：`2026-09-27-identity-opaque-sessions.md`
- 通知的 email：`2026-10-05-notification-module.md` 決策 8
