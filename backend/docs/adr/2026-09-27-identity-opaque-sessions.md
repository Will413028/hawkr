# 登入憑證一律是存在 Postgres 的 opaque session；OTP 也存 Postgres，簡訊由 River job 發送

| 項目 | 值 |
|---|---|
| 狀態 | D1、D2、D3 恢復有效（2026-10-30，Will，`2026-10-30-identity-self-hosted-federated-login.md`）；D4、D5 作廢（手機 OTP 與三竹簡訊隨 `2026-09-27-identity-clerk.md` 移除，未恢復） |
| 日期 | 2026-09-27 |
| 影響範圍 | 單體的 identity 模組與 `internal/httpapi` 的認證；四個 client 的登入與憑證處理；設計書 17、51 |
| 上層決策 | `2026-09-24-go-modular-monolith.md` D1、D5；`2026-09-26-external-contract-protobuf-connect.md` D6（入口在 A2 移到單體） |

取代 `2026-09-18-web-session-ssot-and-cache-layering.md`（三個決策的前提都不成立，見脈絡）與 `2026-09-18-mitake-sms-otp-backend-issued.md` 的 D6。設計書 17 的 JWT、refresh、Bearer 撤銷與 Redis 快取各節，以及設計書 51 的 Redis key 設計，在 identity 搬進單體（遷移計畫 A2）時由本檔取代，兩份設計書各加單體註記。

## 脈絡

公司時期的憑證分兩套：App 是 HS256 的 JWT（7 天，`accountType` 烘在 claim 裡）加 refresh token 輪替；web 是 gateway 持有的 opaque session。JWT 要能撤銷，於是長出 `revoked_tokens`、`accounts.auth_epoch`、`accounts.roles_epoch`、Redis 的 `jwt:revoked`、`auth:epoch`、`auth:roles-epoch`、`account:blocked`，再加上 gateway 的讀穿（`CheckCredential`）、per-pod 的判決快取、singleflight 與 token bucket。Redis 對撤銷名單只能證明「已作廢」、不能證明「有效」，所以穩態下每個 Bearer 請求都要讀穿到 Postgres（設計書 17「Bearer 憑證撤銷」）。web session 的權威是 `web_sessions`，Redis 是加速層，因為 gateway 與 account-service 是兩個 process，每次驗證都是一次 gRPC。

約束：

- 繼承：入口在 A2 移到單體（ADR `2026-09-26-external-contract-protobuf-connect.md` D6）。驗證憑證的是 identity 模組的公開 API，在同一個 process 內、直接讀自己的 schema。
- 繼承：單體有多個 replica（serve）與 worker（ADR `2026-09-24-go-modular-monolith.md` D1），共用同一個 Postgres。
- 外部：web 的憑證不能讓 JS 讀到（httpOnly cookie）；App 以 `Authorization` 帶憑證（RFC 6750）。
- 外部：三竹沒有 sandbox，每一次發送都扣點；`clientid` 在 12 小時內可以去重（設計書 51 §7.4）。
- 拿掉：「JWT 讓驗證不需要查詢」。撤銷的需求已經讓每個請求都要查詢，而且在單體裡查詢是同一個 process 內的一次索引讀取。
- 拿掉：「Redis 當 session 的加速層」（被取代的 ADR 的前提）。那是為了省下 gateway 到 account-service 的遠端呼叫；該 ADR 自己也寫明「session 權威再次搬家」時要重新檢查。
- 拿掉：「OTP 是單把 Redis key 的短生命週期狀態，沒有跨 key 的交易需求」（設計書 51 §5.2）。實際上核碼之後要寫帳號，碼卻在 DB transaction 之前就被消耗（transaction 失敗時碼已經燒掉），而且「讀出、比對、刪除」三步不是原子的，同一個碼可以被併發使用兩次（#2510）。

## 考慮過的方案

### 憑證

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：伺服器端的 session，client 只持有 opaque token，伺服器存它的雜湊；閒置逾時加絕對逾時（[OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)；[The Copenhagen Book](https://thecopenhagenbook.com/sessions)）。web 以 httpOnly cookie、App 以 Bearer 帶同一種 token | 每個認證過的請求一次有索引的讀取；滑動續期要寫入 | 採用 |
| 短效 JWT（15 分鐘）不查撤銷，加 refresh 輪替與重放偵測 | 撤銷、停權、角色變更最多延遲一個 access 的效期；web 仍是 session，兩套模型並存；refresh 的 single-flight 與重放偵測留在兩支 App 與伺服器 | 否決：省下的查詢在單體裡只是一次索引讀取，換來的延遲與兩套模型不值得 |
| 照搬：JWT 7 天，加上 epoch、撤銷名單、Redis 快取與讀穿 | 為了讓無狀態的 token 能撤銷，又把狀態放回每個請求；四層快取各有 fail-open 的面（gateway 盤點：讀穿答不出來時放行、Touch 失敗放行、登出只改本 pod 的判決快取 300 秒） | 否決：必要條件（驗證方與 DB 在不同的 process）不成立 |

### OTP 的儲存與發送

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：一次性的驗證碼是一列資料，驗證與使用在同一個 transaction 內以條件式 UPDATE 消耗；外部呼叫先寫 intent 再由 job 執行（單體 ADR D5） | 每次發送多一列寫入；限流的計數以查詢算出，要有索引 | 採用 |
| 照搬：Redis 的 `otp:{phone}`，發送在請求內同步、失敗不重試 | 不能與寫入帳號放進同一個 transaction；需要 Lua 或 `GETDEL` 才能原子消耗 | 否決 |
| OTP 存 Postgres，但在 commit 之後同步呼叫三竹、不重試 | 使用者當下看得到發送失敗；請求要等三竹最多 10 秒；程式崩潰在 commit 與呼叫之間時，列存在但從未發送，沒有東西會補送 | 否決（Will 2026-09-27） |

## 決策

- **D1 憑證**：web 與 App 用同一種 opaque session，存在 identity 模組的表。token 是 32 bytes 的 CSPRNG，只存 SHA-256；web 放在 httpOnly cookie（`__Host-hk_session`，`SameSite=Lax`），App 放在 `Authorization: Bearer`，其他位置（query、`token` header）一律不接受。每個請求由 `internal/httpapi` 呼叫 identity 解析 session，一併讀出帳號狀態與角色。
- **D2 期限**：閒置逾時 web 7 天、App 14 天（沿用 web 的 7 天與 App refresh 的 14 天）；絕對逾時一律 90 天（沿用 refresh family 的 90 天，web 原本沒有）。續期最多一天寫一次。
- **D3 撤銷即刪列**：登出、停權、刪除帳號、移除角色都在同一個 transaction 內刪掉（或改變）session 讀到的列，下一個請求就生效。沒有 JWT、refresh token、epoch、撤銷名單，也不用 Redis。
- **D4 OTP**：驗證碼是 identity 的一列（碼只存雜湊），綁手機與用途；核碼以條件式 UPDATE 在寫入帳號的同一個 transaction 內消耗。冷卻、每日上限與每個 subject 的每小時上限，從同一張表以索引查詢算出。
- **D5 簡訊**：寫入 OTP 的同一個 transaction 排入 River job；job 以 OTP 的 ID 當三竹的 `clientid`，重試只在 OTP 的有效期內。取代 mitake ADR 的 D6。

## 後果

- 被取代的機制：`revoked_tokens`、`refresh_token_families`、`refresh_tokens`、`auth_epoch`、`roles_epoch`、`CheckCredential`、`RevokeToken`、`RefreshToken`、gateway 的 web session 快取、負向快取、判決快取與 blacklist 讀穿，以及 Redis 的 `session:*`、`jwt:revoked:*`、`auth:epoch:*`、`auth:roles-epoch:*`、`account:blocked:*`、`otp:*`。兩支 App 的 refresh 與 single-flight 刪除，收到 `unauthenticated` 就回到登入。
- 每個認證過的請求多一次 DB 讀取（取代原本的 Redis pipeline 加讀穿）。Postgres 不可用時整個單體本來就不能服務，憑證沒有另外的降級路徑。
- 使用者感受得到的差異：web 多了 90 天的絕對逾時；簡訊發送失敗時，使用者看不到立即的錯誤，要等 60 秒的冷卻之後重送。
- web 在各環境都經第一方同源反代打單體（〈系統架構與資料流向〉§2.4），`__Host-` 的 cookie 因此落在 web 的 host，website 的 middleware 看得到它；部署時反代必須把 `/api/connect/*`（去掉前綴）與 `/api/v1/*` 轉給單體（2026-09-27 design-review 補記）。
- D2 的 web 7 天同時是 website 提示 cookie 的上限：WebKit 把 JS 寫的 cookie 限在 7 天，web 的閒置逾時調長之前，要先改提示 cookie 的做法（2026-09-27 design-review 補記）。
- D4 與實作不一致：實作以明文保存驗證碼（6 位數的雜湊能窮舉，發簡訊的 job 也要讀出碼），目前也只有 1303 一個用途。明文或加密保存待 Will 決定（2026-09-27 design-review）。
- 應該重新評估的條件：量測顯示 session 的讀取是熱路徑的瓶頸（先考慮 process 內的短 TTL 快取，並重新接受它的撤銷延遲）；或驗證憑證的地方又和 identity 的資料分到不同的 process。

## 修訂（2026-10-28）

設計書 17（登入與帳號建立流程）已刪除，登入、帳號與 session 見 `docs/modules/identity.md`，買家建立見 `docs/modules/buyer.md`。本文中「設計書 17」與它的章節編號指向已刪除的文件，內容見 git 歷史。

設計書 51（簡訊發送與手機 OTP）已刪除：手機 OTP 與三竹簡訊已隨 Clerk 移除。本文中「設計書 51」、「51 號」與它的章節編號指向已刪除的文件，內容見 git 歷史。
