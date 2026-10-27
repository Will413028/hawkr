# 單體裡的 vendor portal：以邀請為單位的存取、可重複兌換的連結

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-03） |
| 日期 | 2026-10-03 |
| 影響範圍 | `internal/modules/agent`（vendor portal）、`internal/modules/seller`（prospect 的封存與合併事件）、notification-service（`EnsureVendorPortalInvite`、`MarkVendorPortalInviteSecretDelivered` 的呼叫方）、website（`/vendor/rfq`） |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 7d；決定第 6 項 |

## 脈絡

vendor portal 讓沒有帳號的供應商從採購詢價（RFQ）的邀請信進來看詢價、寫草稿、正式送出。seller-service 的實作（盤點於 7d 開工時，快照 152dab261）：

- 三張表：`vendor_portal_identities`（每個 prospect 一筆，帶 email 快照）、`vendor_portal_invites`（每個 RFQ 收件人一筆，單次兌換）、`vendor_portal_sessions`（兌換時撤銷同一個 identity 的全部 session）。
- session 只能存取它那張邀請的 RFQ（`rfq_recipient_id`），identity 除了 email 只有一個從沒被寫入的 `REVOKED` 狀態。同一個業者有兩張 RFQ 時，點第二張會踢掉第一張已經 Google 驗證過的 session。
- 邀請兌換一次就失效；寄出後 `EnsureInvite` 鎖死，沒有重寄的路徑，換裝置或 session 過期的業者進不來。
- 正式送出要求 Google 登入的 email 等於邀請的 email；沒有 Google 帳號的信箱永遠不能送出。
- `EnsureInvite` 不在 transaction 內；已兌換或已撤銷的邀請被回成「已寄出」，邀請無聲地不寄；`expires_at` 不驗證；沒有任何寫入者撤銷邀請；三張表沒有清理，邀請的 email 永久保存。

約束：
- 繼承：觸發寄信的是 ai-service 的 outbox 事件，寄信在 notification-service；兩者分別在 A9、A8 才搬進單體。寄信前要拿到明文 secret，所以 Ensure（回傳 secret）與 Mark（標記已寄出）的兩段式在 A8 之前成立。
- 繼承：`rfq_recipient_id` 屬於 ai-service，agent 在 A9 之前無法驗證它存在。
- 不再成立：「每個 prospect 一個 identity」——它只帶 email 快照，存取範圍本來就是單張邀請。

## 考慮過的方案

**身分與 session 的範圍**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：存取權綁定單一資源，敏感動作再驗證身分（DocuSign 每份文件每個收件人各自驗證；Coupa 供應商不需帳號即可從信件回覆） | 業者每張 RFQ 各驗證一次 | ✓ 拿掉 identity 表，邀請存 `prospect_id` 與寄出時的 email 快照，session 與驗證都綁單張邀請 |
| 保留 identity，只撤銷同一張邀請的 session | 多一層沒有作用的表 | ✗ |
| 照舊撤銷 identity 的全部 session | 跨 RFQ 互踢 | ✗ |
| 供應商帳號（Ariba Network） | 需要長期往來時才值得；現在沒有這個需求 | ✗（A9 有需求時從 prospect 建立） |

**邀請連結**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：存取多日的商業文件用到期前可重複使用、可撤銷的連結，敏感動作再驗證（DocuSign、Coupa 信件連結）；單次的 magic link 是 15 分鐘的登入用途 | 被轉寄的連結在到期前都能看詢價、改草稿 | ✓ 每次兌換發新的 session，正式送出仍要驗證 email |
| 單次兌換＋重新索取連結 | 要寄信，寄信在 notification | ✗ |
| 照舊單次兌換 | 業者換裝置就進不來 | ✗ |

單次兌換並沒有比較安全：被轉寄的人先點一樣進得去。

**email 的驗證方式**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：寄一次性驗證碼到該信箱（email OTP）證明擁有信箱，Google 是捷徑 | 要寄信、要新畫面 | ✓ 方向；資料模型在 7d 記錄驗證方式與驗證的 email，OTP 的寄送與畫面在 A8 |
| 只支援 Google | 公司網域、hinet 信箱的業者不能送出 | ✗ |

**合併時的邀請**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：合併保留一筆（survivor），指向被合併紀錄的引用一律解析到它（MDM survivorship；Salesforce、HubSpot 的 duplicate merge 改掛子記錄） | | ✓ 改掛到贏家 |
| 撤銷輸家的邀請 | 業者進行中的詢價中斷，只能重新邀請 | ✗ |

## 決策

- **邀請**：每個 RFQ 收件人一張（`rfq_recipient_id` UNIQUE），存 `prospect_id`（不設跨模組 FK，email 由 `seller.ProspectContact` 取）、寄出時的 email 快照、secret 的 SHA-256、到期時間（必須在未來、不超過上限）、撤銷時間與原因。到期或撤銷之前可以重複兌換。
- **Ensure／Mark**：保留兩段式，給 notification 呼叫。Ensure 在 transaction 內鎖住那張邀請，並在同一個 transaction 以共享鎖讀 prospect：只有進行中的 prospect 能建立或換 secret（與封存序列化，封存之後不會再冒出有效的邀請）；沒有就建立，還沒寄出就換 secret 並重讀 email，既有邀請的 `prospect_id` 與輸入不同時拒絕；已寄出、已撤銷、prospect 不是進行中、沒有可寄的 email 各回不同的結果。Mark 帶著寄出的那一把 secret，只有它仍是目前的 secret 才標記為已寄出，過時的 Mark 不生效。
- **session**：兌換發一個新的 session（不透明 token，存 SHA-256），只屬於那張邀請；idle 24 小時、絕對 7 天；同一張邀請的有效 session 有上限，超過時撤銷最舊的。
- **驗證**：session 記錄驗證方式（現在只有 `GOOGLE`）、驗證的 email 與時間；驗證的 email 與邀請的快照不分大小寫、去掉前後空白後相等才成立。正式送出、撤回、婉拒要求已驗證的 session（由步驟 8 的入口檢查）。Google 的 token 由入口經 identity 驗證，agent 只收已驗證的 email。
- **撤銷與合併**：seller 在 prospect 封存的 transaction 內發出事件，agent 在同一個 transaction 撤銷該 prospect 的邀請與 session。合併時邀請改掛到贏家（Will 2026-10-03：MDM 的 survivorship，引用一律解析到保留的那筆，與 seller 改掛識別鍵一致），email 快照與 session 不變；之後贏家封存才撤銷。
- **驗證的證據**：正式送出是不可撤回的商業承諾，證據跟著報價保存（Will 2026-10-03，電子簽署的慣例，例如 DocuSign 的 Certificate of Completion）：步驟 8 的入口在送出、撤回、婉拒的請求帶上 session 的驗證紀錄（方式、email、時間、session ID），ai 存在不可變的報價版本上。session 上的紀錄只用於授權，隨 session 清理。
- **邀請信內容**：信件是 preview 的子集：品項、數量、單位、截止日、遮罩後的買家顯示名、連結與到期時間；不放配送地址、任何價格、買家身分、Pricebook 參考區間與買家希望區間。信會被轉寄、留在收件匣、隨信箱外洩，揭露範圍不該比要兌換才看得到的 preview 更寬。發票信放完整內容、不放連結，是因為那邊的收件人登不進平台；這裡有 portal 可進（沿用 E1 設計書 §5.6，`backend/docs/superpowers/specs/2026-09-11-vendor-portal-e1-invite-email-design.md`（私人封存））。
- **清理**：River 定期刪除超過絕對期限的 session，以及到期或撤銷超過 90 天的邀請（連同 email）。

## 後果

- 業者在不同裝置、不同 RFQ 各要驗證一次。
- 被轉寄的連結在到期或撤銷前可以看詢價、改草稿，不能正式送出。
- 兌換次數與 Google 驗證的限流屬於入口（步驟 8），模組不做。
- 重新評估：A8 notification 搬進單體時，建立邀請與寄信改成同一個 transaction 加 outbox，拿掉 `secret_delivered_at` 與換 secret 的邏輯，並加入 email OTP；A9 採購搬進單體時，邀請在 RFQ 發布的 transaction 內建立、驗證 `rfq_recipient_id`，買家取消發布時撤銷邀請。

## 修訂：入口的 cookie 與 RFQ 路由、session 上限、撤回與婉拒的證據（2026-10-04，Will）

步驟 8i 開工的盤點發現三處原決策沒有處理到的情況。原決策段保留歷史。

**入口**
- cookie 要同時送到 Connect（瀏覽器路徑 `/api/connect/…`，前綴由 web host 的反代處理，單體不知道）與 gateway 的 RFQ 路由（`/api/v1/public/vendor_portal/…`），舊的 `Path=/api/v1/public/vendor_portal` 涵蓋不到前者。
  - 基準：OWASP Session Management 建議 `__Host-` 前綴（RFC 6265bis：必須 `Secure`、不帶 `Domain`、`Path=/`），cookie 只屬於這個 host；Path 本來就不是安全邊界（RFC 6265 §8.5）。
  - 方案：`__Host-` 加 `Path=/`（✓，擋得住兄弟子網域塞同名 cookie；代價是 token 也跟著 Next 頁面與 route handler 的請求送出）；`__Secure-` 加 `Path=/api`（✗，送出的範圍窄一點，但擋不住 cookie tossing）。
  - 決策：`__Host-hk_vendor_portal`，`Path=/`、`Secure`、`HttpOnly`、`SameSite=Lax`。
- ai 的 6 條 RFQ 路由到 A9 都在 gateway。
  - 基準：邊界驗證 session，把已驗證的身分以受信任的 header 交給後端（Google Cloud IAP、oauth2-proxy、Cloudflare Access）。
  - 方案：入口驗 agent 的 session 後設上 `X-Vendor-Portal-*`，gateway 信任（✓，入口對每個轉給 gateway 的請求先刪掉同名 header，與 `X-Account-Id` 同一個前提：gateway 不部署）；現在就把 6 條改成 Connect、單體經新的 client 呼叫 ai-service（✗，ai-service 在 A9 才進邊界，現在要設計 17 欄報價的對外契約，A9 重新判斷 ai 的模型時可能重做）。
  - 決策：前者。A9 把 RFQ 路由改成 Connect 時，cookie 認證的方法不放在 `PublicService`：第九節會讓公開的 GET 回 `public, no-cache`，私人資料可能被共享快取存下；那時為受邀供應商另立使用者類別（回應一律 `private, no-store`）。
- `PreviewInvite` 用 POST，secret 放在請求訊息（API 規範 §三）：GET 會把 secret 放進 URL（RFC 9110 §17.9、CWE-598）。

**session 上限**：照原決策「撤銷最舊的」，拿到轉寄連結的人兌換 5 次就能擠掉業者已經 Google 驗證過的 session。改成：撤銷候選排除本次新發的 session（它在兌換當下一定未驗證），先撤最舊的未驗證，全部已驗證時才撤最舊的。低保證的新憑證不擠掉高保證的既有 session。

**撤回與婉拒的證據**：原決策寫「ai 存在不可變的報價版本上」，但只有送出產生版本列；撤回只改可變身分列上的 `withdrawn_version`／`withdrawn_at`（之後還能再送出、再撤回），婉拒只關閉回覆，兩者都沒存 `client_request_id` 與 `reason_code`。
- 基準：電子簽署的稽核軌跡是只追加的事件紀錄，每個動作各一筆（DocuSign 的 envelope history）；金流的退款、爭議是引用原交易的獨立不可變物件（Stripe 的 `Refund`、`Dispute`）。
- 方案：送出存在版本列，撤回、婉拒存在新的只追加指令紀錄（✓）；三個動作共用一份指令紀錄（✗，要重做送出既有的冪等機制，而 A9 會重新判斷 ai 的模型）；證據欄位加在身分列（✗，再次撤回會被覆蓋）。
- 決策：送出的證據存在報價版本上，撤回與婉拒存在追加的指令紀錄上（連同 `client_request_id`、`reason_code`），都不可變；撤回的事實來源是指令紀錄，身分列的撤回欄位在同一個 transaction 由它推出。A9 搬遷時把三者統一成一份稽核軌跡。
- ai 裡證據的 email 跟著報價保存，A9 為採購紀錄訂保存期限時一併處理。

## 修訂：每張邀請一個 cookie、Google 驗證綁定 session、受邀供應商的使用者類別（2026-10-04，Will）

步驟 8i 完成前的 design review 發現：瀏覽器只有一個 vendor portal cookie，「session 屬於一張邀請、不跨 RFQ 互踢」只在伺服器端成立。

**cookie**
- 現象：業者重開同一封信、或打開另一張 RFQ，新的兌換會蓋掉已經 Google 驗證過的 cookie；兩個分頁各開一張 RFQ 時，在 A 分頁驗證會驗到 B 的 session。單一 cookie 沿用自舊模型（兌換時撤銷同一業者的全部 session），那個前提已經不成立。
- 基準：瀏覽器同時持有多個授與，每個請求指名要用哪一個，建立一個不使另一個失效（Google 多帳號的 `/u/N/`、DocuSign 每個收件人各自的簽署 session）。
- 方案：每張邀請一個 cookie（✓，session 仍屬於一張邀請，cookie 與 session 一對一、期限一致）；一個 cookie 對應伺服器端多個授與（✗，安全性相同，但要重寫模組的 session 不變量，改動約 2–3 倍）；接受並只修錯驗（✗，與否決「跨 RFQ 互踢」的理由矛盾）。依 Path 區分不可行：`__Host-` 規定 `Path=/`。
- 決策：cookie 名稱 `__Host-hk_vp_<rfq_recipient_id>`，屬性同前一個修訂。每個以 cookie 認證的請求都指名收件人：gateway 路由從 path 讀，Connect 方法帶 `rfq_recipient_id`。兌換時請求已帶這張邀請的有效 session 就沿用、不發新的；帶了無效的就清除。瀏覽器的 vendor cookie 超過 20 個時，兌換回應清掉多出的。website 的網址帶收件人 ID（Will 同意），不再以 localStorage 記一個 ID。
- 重新評估的觸發：A9 從 prospect 建立供應商帳號、改以帳號登入時。

**Google 驗證綁定 session**
- 基準：ID token 直接交到瀏覽器的流程，OIDC Core 1.0 §3.2.2.1 規定必須帶 `nonce`，§15.5.2 建議以 HttpOnly session cookie 值的雜湊作為 nonce；Google Identity Services 的 `initialize` 接受 `nonce`。舊實作沒有 nonce，是沿用平台登入的形狀；step-up 是提升既有 session 的等級，token 與 session 的綁定才有意義。
- 方案：由 session token 推導 nonce（✓，不新增欄位）；伺服器發一次性 challenge 存 DB（✗，單次使用已由驗證的 CAS 提供，多一個方法與欄位）；不做（✗，與 OIDC 的 REQUIRED 不一致，而驗證紀錄是正式送出的證據）。
- 決策：nonce 是 session token 加上用途前綴後的 SHA-256，伺服器交給 client，client 傳給 GSI；identity 驗證 Google token 時比對 nonce（必填，空值拒絕），並拒絕 `iat` 超過 10 分鐘的 token（GSI 不提供 `max_age`，`iat` 是唯一的新鮮度訊號）。
- 觸發：`GOOGLE_CLIENT_IDS` 要加第二個 client 時，先確認所有 Google 驗證的呼叫方都帶 nonce。

**受邀供應商的使用者類別提前到 8i**：前一個修訂把它排在 A9，但這只是排程，類別與 handler 都在單體內。client 需要在重新整理後讀到 session 的驗證狀態與 nonce，這個讀取是 cookie 認證的，不能放在 `PublicService` 的 GET（第九節）。決策：新增以 cookie 認證的 service（回應一律 `private, no-store`，讀取可以 GET），提供讀取 session 的方法並收下 `VerifySession`；`VendorPortalPublicService` 只留以 secret 為憑證的 Preview 與 Exchange。A9 把 RFQ 路由改成 Connect 時加到這個 service。
