# 身分驗證改用 Clerk：顧客與後台各一個 application，帳號、角色與停權留在 identity

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-27 |
| 修訂 | 2026-09-27：D1、D2 後台改用 Cloudflare Access（Will，見文末〈修訂：後台改用 Cloudflare Access〉）；2026-10-05：email 以 webhook 同步、作為通知的寄件地址（Will，見文末〈修訂：email 作為通知的寄件地址〉）；2026-10-08：Apple 授權的撤銷改回伺服器保存 refresh token、刪帳時撤銷（Will，見文末〈修訂：Apple 授權由伺服器保存並在刪帳時撤銷〉） |
| 影響範圍 | 單體的 identity 模組與 `internal/httpapi` 的認證；四個 client 的登入；設計書 17、51 |
| 上層決策 | `2026-09-24-go-modular-monolith.md` D1、D5；`2026-09-26-external-contract-protobuf-connect.md` D6 |

取代 `2026-09-27-identity-opaque-sessions.md` 的 D1、D2、D4、D5，D3 只留「停權、角色、刪帳以 identity 的資料為準，每個請求讀取」。

## 脈絡

A2 把 account-service 搬進單體時，登入、session、手機驗證都是自建的（上面那份 ADR）。設計審查留下的待決項，多數是自建才要回答的登入細節：1303 要不要改成伺服器端的待完成登入 token、1304 連結帳號的信任模型、管理員 session 的期限、驗證碼的保存方式。Will 2026-09-27 重新檢視前提：

- 拿掉：「新帳號要先驗手機」。新帳號不用驗手機（Will）。
- 拿掉：「簡訊一定要走三竹」。舊公司的帳號本來就不能用（D9），要重簽，而且三竹要固定的出口 IP。
- 新增：顧客登入要支援 LINE（台灣的主要通訊軟體），加上既有的 Google、Apple。
- 新增：App 之後改用 Swift 原生（Flutter 版是過渡）。

約束：

- 外部：App Store 要求 App 內可以刪帳，使用 Sign in with Apple 時要以 Apple 的 REST API 撤銷 token（Guideline 5.1.1(v)）。
- 外部：web 長效的憑證不能讓 JS 讀到；App 以 `Authorization` 帶憑證。（Clerk 的長效憑證是 Frontend API 網域上 httpOnly 的 `__client` cookie；60 秒的 session token 由 SDK 放在 JS 讀得到的地方，這是 Clerk 的設計，接受。）
- 繼承：其他模組以 identity 的 account ID 參照帳號；停權、移除角色要在下一個請求就生效。
- 繼承：部署以單一 image 為對象（遷移計畫），不另外維運身分服務。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 自建（現況）：`go-oidc` 驗 ID token、Postgres 的 opaque session；拿掉手機驗證後加 LINE | 帳號連結、管理員 session、未來的 passkey 與 MFA 都要自己設計與維護 | 否決（Will 2026-09-27） |
| **Clerk** | 每 60 秒換一次 session token 要連 Clerk，Clerk 停擺時已登入的使用者也會失效；正式環境要調整 session 期限需付費方案；使用者資料有兩份（Clerk 的 user、identity 的 account） | 採用 |
| Auth0 | 免費 25,000 MAU 之後費用陡升；每個登入方式各是一個使用者，連結要自己寫 Actions | 否決：成本曲線；Swift 版之後 Clerk 的主要缺點（Flutter SDK）不成立 |
| Firebase Authentication | LINE 要升級 Identity Platform 當 OIDC provider（第二級：50 MAU 之後每人 0.015 美元），LINE 網頁登入的 ID token 是 HS256，能否接受未確認；或後端自己驗 LINE 再簽 custom token | 否決：LINE 的支援等於還是自己做 |
| 自架開源 IdP（Zitadel、Ory Kratos、Keycloak） | 多一個要維運的服務，違反單一 image；角色與代行仍要自己寫 | 否決 |

## 決策

- **D1 身分提供者**：顧客用 Clerk。後台（2026-09-27 修訂）改用 Cloudflare Access，不再有後台的 Clerk application；員工與顧客的身分仍然分開。
- **D2 登入方式**：顧客只開 Google、Apple、LINE（免費方案的社群登入上限是 3 個），不驗手機、沒有密碼與 email 驗證碼。後台（2026-09-27 修訂）由 Cloudflare Access 的 email 白名單決定誰能進，不再以 email 網域自動建立管理員。
- **D3 憑證**：Clerk 的 session token（RS256 的短效 JWT，SDK 自動換新）。顧客端都以 `Authorization: Bearer` 帶：瀏覽器每次請求前以 SDK 的 `getToken()` 取得（快過期時先換新），App 以 `clerk_auth` 取得。後台（2026-09-27 修訂）改帶 Cloudflare Access 的 JWT，見文末。Clerk 的 `__session` cookie 只在分頁有焦點時更新，閒置或睡眠之後的第一個請求可能帶著過期的它，所以不以它為主；單體仍收它（沒有 Bearer 時），給帶不了 header 的瀏覽器導覽（例如發票 PDF 的下載連結，點擊時分頁有焦點）；兩者都有時 Bearer 優先。單體以 instance 的公鑰在本機驗證（不打網路），檢查 `exp`、`nbf`、`iss`，web 另外檢查 `azp` 是允許的 origin。顧客 instance 的 token 只能呼叫 `*AppService` 與還沒搬的 REST，後台的憑證（修訂後是 Access 的 JWT）只能呼叫 `*AdminService`；登入與登出由 Clerk 的 SDK 處理，`IdentityPublicService` 刪除。
- **D4 帳號**：identity 保留自己的 account ID 與 schema，另存 `clerk_user_id`（UNIQUE）。第一個認證過的請求在 transaction 內建帳（同時第一次登入由 UNIQUE 收斂），需要的 email、登入方式由 Clerk 的 Backend API 讀一次。不接 webhook：Clerk 建議能不用就不用（最終一致），刪帳一律由我們發起。
- **D5 狀態與撤銷**：停權、角色、刪帳以 identity 的資料為準，每個請求讀取，下一個請求就生效。停權與刪帳在同一個 transaction 排 River job，呼叫 Clerk 的 Backend API 撤銷 session 或刪除使用者，讓 client 的 SDK 也登出。
- **D6 帳號連結**：交給 Clerk（provider 回報已驗證的 email 相同時自動連結）。1304 的流程、`ConfirmAccountLink` 與 `link_tokens` 刪除。
- **D7 session 期限**：開發期用 Clerk 預設（最長 7 天）。正式環境要把 App 的 session 拉長，需要付費方案，上線前由 Will 決定。
- **D8 client**：website 用 Clerk 的 Next.js SDK（admin 在 2026-09-27 修訂後不用 Clerk），畫面沿用自己的（以 Clerk 的 hooks 做自訂流程，不用 Clerk 的預設 UI）。兩支 Flutter App 過渡期用 `clerk_flutter`（社群維護的 beta，Will 2026-09-27），Swift 版改用官方的 `clerk-ios`。

## 後果

- 刪除：`SignIn`、`SignInAdmin`、`SignOut`、`ConfirmAccountLink`、`SendPhoneCode` 與 `PHONE_CODE_EXPIRED`；`Account.phone`；identity 的 session、驗證碼、連結 token、OAuth 連結、nonce 各表與程式；三竹的 client、發簡訊與 Apple 授權碼交換的 job；website 的提示 cookie 與 session 探測；admin BFF 的 cookie 轉送；四端的手機驗證與連結確認畫面。
- `GetSession` 保留：回傳 identity 的帳號、角色與 active role。
- 可用性：Clerk 停擺時新登入失敗，已登入的使用者在下一次換 token（最多約 60 秒）之後也失效。這是換掉自建 session 的主要代價。
- 費用：顧客的 development instance 免費。正式環境的 session 期限（D7）要付費。後台（2026-09-27 修訂）改用 Cloudflare Access，Zero Trust 50 人以下免費，不再有後台的 Clerk instance。
- Apple token 的撤銷：刪帳時 Clerk 會不會代為撤銷 Apple 的 token，還沒確認（計畫的前置驗證）。不會的話，照 Apple 的說明刪除資料並引導使用者手動撤銷。
- 舊 gateway 還在用的兩支 legacy RPC：
  - `GetAccountByPhone`（賣家 CRM 以手機找買家）：帳號沒有手機之後查不到，隨 A3 決定改用什麼資料。
  - `VerifyGoogleIdToken`（廠商入口的二次驗證）：保留驗 Google ID token 的程式，直到廠商入口搬遷。
- 待決項的去向（A2 計畫的設計審查段）：
  - 1303 的 pending token、1304 的信任模型、驗證碼的保存：不再成立。
  - 管理員 session 的期限：由 Access application 的設定決定（2026-09-27 修訂）。
  - 代行 credential、compose 驗收、特性測試、website 預設連舊公司、強制登出的提示、iOS 重裝後仍登入：仍待 Will 決定。
- 應該重新評估的條件：
  - Clerk 的可用性或費用成為問題；
  - `clerk_flutter` 在過渡期擋住 App 的開發；
  - Clerk 在 Cloudflare Workers（OpenNext）上無法運作。

## 修訂：後台改用 Cloudflare Access（2026-09-27）

**脈絡**：D2 原本沿用公司時期「Workspace 網域內第一次登入就建立管理員」。公司結束後沒有 Workspace 網域，這個前提不成立：填個人信箱的網域（`gmail.com`）等於任何人都能成為管理員。另外後台的 Clerk application 必須開放註冊，第一次登入才建得出使用者，任何人都能在它裡面建帳號（後端再回 403）。免費方案的 Clerk 不能設白名單、MFA 與 session 期限（固定 7 天），這些都要 Pro（每月 25 美元）。

內部後台的業界做法：員工與顧客的身分分開、明確開通（邀請或 IdP 的群組，不以「能登入」自動開通）、強制 MFA、較短的 session、以 zero-trust 閘道（Cloudflare Access、Google IAP）在網路層再擋一道、最小權限與稽核。

| 方案 | 代價 | 判定 |
|---|---|---|
| 保留後台的 Clerk，改成邀請制（Require invitations，免費） | MFA 與較短的 session 要 Pro；後台頁面仍對外公開 | 否決 |
| 顧客與後台合成一個 Clerk，以角色區分 | 員工與顧客的使用者池混在一起，顧客的政策（開放註冊、7 天 session）套到管理員 | 否決 |
| **Cloudflare Access 擋在整個後台前面**（Zero Trust 50 人以下免費） | 後台與 Cloudflare 綁在一起（後台本來就部署在 Cloudflare Workers）；MFA 取決於各人 Google 帳號的設定；本機沒有 Access，要另外的開發路徑 | 採用（Will 2026-09-27） |

**決策**（取代 D1、D2 的後台部分，以及 D3、D8、〈後果〉中關於後台 Clerk instance 的句子；原文已就地加註）：

- 後台的網域整個放在 Cloudflare Access 的 application 後面：登入方式 Google，policy 是 email 白名單，session 期限在 Access 設定。沒通過 Access 的請求到不了後台的頁面與 BFF；workers.dev 與 preview 的網址要關掉或同樣保護。
- 誰是管理員只有一個來源：Access 的白名單。單體驗 Access 簽的 JWT（team domain 的公鑰、`aud` 是這個 application 的 AUD tag、`iss`、效期），第一次看到一個 Access 使用者就建立管理員列（以 JWT 的 `sub` 對應，存 email 供顯示）。`ADMIN_GOOGLE_DOMAINS` 與後台的 Clerk application、key 刪除。
- 後台的 BFF 把 Access 放進請求的 `Cf-Access-Jwt-Assertion` 以 `Authorization: Bearer` 帶給單體，不自己判斷身分；單體一定驗章，BFF 被繞過也偽造不了。
- 登出：先呼叫 `IdentityAdminService.SignOut`（撤銷代行），成功才導到 Access 的 `/cdn-cgi/access/logout`。
- 離職與 Access 端的登出：Access 沒有 session 結束的通知，單體只在後台的登出按鈕撤銷代行。離職的做法是移出白名單、在 Zero Trust 撤銷這個人的 Access session、從後台撤銷他進行中的代行；沒撤到的代行由代行本身的效期收斂（30 分鐘，閒置 15 分鐘）。
- 本機開發：沒有 Access；BFF 在本機以 `make dev-token -admin` 簽的 dev token 代替（只在 dev key 存在時啟用，與正式設定互斥，同 `DEV_AUTH_KEY_FILE` 的規則）。
- 帳號分級（角色）等需要時再加，屆時由單體的管理員列決定，不放進 Access。
- MFA：Access 的 policy 要求 MFA（以 authentication method 的規則，要求 IdP 回報的 `amr` 含 MFA），不只依賴各人的 Google 帳號設定；列在上線前的待辦。

**後果**：admin 的 Clerk 登入頁、sign-in／sign-up 轉換、`@clerk/nextjs` 刪除；登入頁改成 Access 的畫面（版面改動，Will 已同意這個方向）。identity 的 `admins.clerk_user_id` 改成 Access 的 subject；Access 的 JWT 只帶 email，管理員的姓名與大頭照（DB 欄位與契約）刪除，後台顯示 email。上線前要做：Cloudflare Zero Trust 的 team、Access application、白名單與要求 MFA 的 policy、後台的網域（wiki 的 CD 待辦）。重新評估的條件：管理員超過 50 人、需要角色分級或稽核匯出、後台不再部署在 Cloudflare、Access 無法強制 MFA（上線前設定 policy 時確認）。


## 修訂：C9 驗證、建帳與入口（2026-09-28）

本節修訂 D3、D4 與前一節的驗證細節；原決策段保留歷史。

- **單一 verifier**：使用 `github.com/lestrrat-go/jwx/v3` v3.3.0，以 issuer、key source 與 claim policy 組裝同一個 verifier，移除 go-oidc。顧客 Clerk 使用 `CLERK_JWT_KEY` 的 RSA PEM public key，本機驗章完全不連網；Access、Google、Apple 使用 `jwk.Cache`，自動更新最短間隔一分鐘，未知 kid 的強制更新也以一分鐘限頻（包含失敗），不隨偽造 token 每次抓取。換鍵後未知 kid 最多等這個間隔。dev token 與 Apple client secret 也用 jwx 簽章；兩種 dev token 共用簽章路徑。`jwk.Cache.Refresh` 沒有限頻，未知 kid 的手寫刷新閘門因此保留；快取的生命週期綁定 serve／worker context，關閉時停止背景更新。Clerk 不自動輪替 JWT 簽章金鑰；需要換鍵時先向 Clerk 要求輪替，再更新部署的 `CLERK_JWT_KEY` 並重新部署。新鑰簽出的 token 在重新部署前會驗證失敗。
- **claims**：只接受 RS256、`iss` 精確命中各 verifier 明列的允許值（Google 接受 `https://accounts.google.com` 與 `accounts.google.com`；Clerk、Access、Apple 各只接受一個）、非空 `sub`、必要的 `exp`，驗證 `exp`／`nbf`，容許五秒時鐘誤差。Access 的 `aud` 必須包含 application AUD tag；Apple／Google 必須命中 configured audiences。Clerk 設定時 `CLERK_AUTHORIZED_PARTIES` 必填，`azp` 有帶時必須在允許的 web origin 清單中；native token 沒有 `azp` 可以接受。`sts` 有帶時只接受 `active`。
- **JIT 建帳**：以驗證後的 session custom claims `email`、`name` 建立快照，不呼叫 Backend API 讀 user。Clerk dashboard 的 Sessions → Customize session token 加上 `{"email":"{{user.primary_email_address}}","name":"{{user.full_name}}"}`。缺少這兩個 claim 時以空字串建帳，LINE 沒有 email 是合法情況。email 只供顯示／搜尋，不作識別或授權。`CLERK_SECRET_KEY` 仍用於停權／刪帳的 BanUser、DeleteUser job。
- **cookie 邊界（D3）**：Bearer 優先；`__session` 只在沒有 Bearer、`GET`、路徑屬於 `/api/v1/` 且 `Sec-Fetch-Mode` 為 `navigate` 或 `no-cors` 時接受，供瀏覽器導覽下載與圖片等子資源。`cors`／`same-origin` 的 JS fetch、POST 與所有 Connect 路徑忽略 cookie；沒有 Fetch Metadata 的瀏覽器不能使用 cookie 導覽下載或圖片；目前主流瀏覽器都會送 `Sec-Fetch-Mode`，接受此限制。第一方反代（目前 website 的 `app/api/v1` route，日後正式環境的同源反代）須原樣轉送 `Sec-Fetch-Mode`，否則 cookie 下載與圖片請求無法取得 session。Clerk 的 `__session` 為 SameSite=Lax，跨站子資源請求不帶此 cookie。
- **錯誤**：停權回 PermissionDenied 與「帳號已停權，請聯絡客服」；無效／過期 token 與已刪除帳號回 Unauthenticated。入口保留單一 PermissionDenied 分支，服務停權與後台不允許登入兩種情況。client 依 code 顯示／登出，沒有相同 code 下必須分支的原因，因此不新增 ErrorReason。`apperr.WithCause` 保留公開錯誤分類，把內部原因留在診斷文字，避免內部錯誤覆蓋公開訊息。
- **限流**：已認證的 principal 優先（account `a:<id>`、admin `m:<id>`），未認證才用 website origin-auth 的 visitor／SSR／CI，最後回到 IP（IPv6 /64）。admin 刪除 origin-auth；BFF 只帶 Access Bearer。legacy gateway 依單體已驗證且重設的身分 header 使用同樣的 principal key，保留原有 tier／額度；不讓所有管理員共用 Worker 出口 IP。website SSR／CI 的簽章維持。
- **管理員稽核 IP（Will 2026-09-28）**：admin 操作不記錄操作者 IP，刪除代行 session 與操作稽核的 IP 欄位；Cloudflare Access 的登入稽核紀錄是登入時的 IP 證據來源。訪客的簽章 IP 仍用於 vendor portal 的限流鍵。
- **exhaustive**：錯誤分類與代行拒絕的列舉映射維持明列所有值；新增值時由 lint 迫使決定對外行為，不使用全域 `default-signifies-exhaustive`。
- **原「仍待 Will 決定」清單的去向**：代行 credential 已由主計畫 0.12 及 delegation ADR 修訂解決，在 handoff redemption 用 CSPRNG 產生。強制登出的提示與 iOS 重裝後仍登入仍交由 Will 決定，觸發條件為第一次 TestFlight 外部測試或 App Store 送審之前（A2 計畫 C9）。

## 修訂：email 作為通知的寄件地址（2026-10-05，Will）

脈絡：C9 修訂定 email 只是建帳時的快照、只供顯示與搜尋。ADR `2026-10-05-notification-module.md` 決策 8 要以帳號的已驗證主要 email 寄交易與安全通知，快照不同步會把信寄到使用者已經換掉的地址。

決策：推翻 D4 的「不接 webhook」。identity 以 Clerk 的 `user.updated` webhook（驗證簽章）同步已驗證的主要 email，作為通知的寄件地址；不在寄信時呼叫 Clerk 的 Backend API（寄信路徑不多一個外部依賴）。D4 的理由是 webhook 只有最終一致；這裡可以接受：同步落後的期間，信寄到使用者剛換掉、但仍是他本人驗證過的地址，不影響識別或授權。代價是多一個公開的 webhook endpoint 與簽章 secret（放部署設定）。email 仍不作識別或授權。沒有 email 的帳號（LINE 可能不回傳）只收站內通知。原決策段保留歷史。

## 修訂：Apple 授權由伺服器保存並在刪帳時撤銷（2026-10-08，Will）

脈絡：「後果」記載刪除 Clerk 使用者不會撤銷 Apple 的 token，Clerk 也不交出；2026-09-27 的計畫決定改成刪帳前由 App 再以 Sign in with Apple 取得授權碼、呼叫 `RevokeAppleSignIn`。這個順序在 App 做不到安全：撤銷發生在伺服器檢查前置條件（開著的店、`PENDING_ORDERS`）之前，刪除被拒絕時使用者已被撤銷授權；兩支 App 各要維護一段重新授權流程；管理員刪除與未結訂單完成後的最終刪除沒有 App 在場，根本不撤銷。

決策：推翻該決定，回到 Apple ADR（2026-09-18）的基準「換 token 保存、刪帳時撤銷」。
- App 每次以 Apple 原生登入成功、帳號建立之後，呼叫 `LinkAppleCredential` 交出 identity token 與 authorization code（不阻擋登入，失敗只記 log，下次登入再交）。
- identity 驗證 identity token（簽章、issuer、audience），把它的 `sub` 與 audience 連同加密的 code 排入 job；請求內不呼叫 Apple。job 換 refresh token 後，驗證 Apple 同一次簽發的 ID token 與連結時驗證的 `sub`、audience 相同才以專用金鑰加密（AAD 綁表、欄與帳號 ID）存入 `identity.apple_tokens`（每個帳號一筆，重新連結時取代）；不符或帳號已刪除時當場撤銷換得的 token；暫時性失敗（公鑰連不到、保存失敗）不撤銷（Apple 撤銷會撤掉使用者剛給的整個授權），code 只能用一次所以改排保存的 job，帶加密的 token 回應重試，最後仍失敗就放棄、下一次原生登入再交；每種失敗以 error 等級記 log（原因與帳號 ID，不含 code 與 token）。不呼叫 Clerk Backend API 核對 `sub`：連結由已認證的 session 以驗證過的 identity token 發出，code 與 identity token 的綁定由 ID token 保證，保存的就是呼叫者自己的授權，請求路徑不依賴 Clerk Backend API（同 email 修訂）。Apple 設定在部署的環境（有顧客 Clerk、沒有 `DEV_AUTH_KEY_FILE`）必填，本機開發可全部不設，此時 `LinkAppleCredential` 回 `unavailable`；啟動時檢查保存的密文與排隊中 job 參數的密文用到的金鑰都在 `IDENTITY_TOKEN_KEYS`。
- 所有刪除路徑（本人立即刪除、管理員刪除、未結訂單完成後的最終刪除）都經過 identity 的 `DeleteAccount`；它在刪帳號的同一個 transaction 內取走保存的 token、刪列並排入撤銷 job。受理 `PENDING_ORDERS` 與被前置條件拒絕時還沒有刪帳號，不撤銷。刪帳號永遠不因 Apple 阻擋：沒有保存的 token（沒交出過、Apple 沒設定）就沒有東西可撤銷。
- 刪除 `RevokeAppleSignIn` 與兩支 App 的重新授權流程，刪帳號只剩 `DeleteAccount`，前置條件由伺服器在鎖內檢查。
- 只用 web 登入 Apple 的使用者，授權碼在 Clerk（web 的 Clerk 流程不經過 App）：伺服器沒有 token 可撤銷，website 維持人工提示。觸發條件：Clerk 支援 web 以原生 id_token 登入時，同樣改成交出憑證。

沿用的 `AppleTokens` client、加密與 `identity.revoke_apple_token` job 不變。原決策段保留歷史。

## 修訂（2026-10-28）

設計書 17（登入與帳號建立流程）已刪除，登入、帳號與 session 見 `docs/modules/identity.md`，買家建立見 `docs/modules/buyer.md`。本文中「設計書 17」與它的章節編號指向已刪除的文件，內容見 git 歷史。
