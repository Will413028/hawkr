# API 設計規範（Connect）

| 項目 | 值 |
|---|---|
| 狀態 | 現行（ADR `docs/adr/2026-09-26-external-contract-protobuf-connect.md`；Connect 試點 2026-09-26 通過）。新的對外 API 一律依本規範；〈API 設計規範〉只剩舊 gateway 的 REST 與文末列出仍然有效的條文 |
| 適用範圍 | 根目錄 `api/` 的 Protobuf 契約、單體 `internal/httpapi` 的 Connect handler、四個 client |
| 依據 | Google AIP（121–193）、Connect 協定、buf lint STANDARD |
| 範例 | `api/hawkr/content/v1alpha1/content.proto`、`api/hawkr/admin/content/v1alpha1/content.proto` |

## 一、檔案、package 與版本

- 一個模組、一個使用者類別一個 package，一個檔案：
  - 前台（不需要登入，或買家、賣家登入）：`api/hawkr/<模組>/<版本>/<模組>.proto`，package `hawkr.<模組>.<版本>`。
  - 後台（管理員）：`api/hawkr/admin/<模組>/<版本>/<模組>.proto`，package `hawkr.admin.<模組>.<版本>`。
  - 跨模組共用的訊息：`hawkr.common.<版本>`，只放真的被多個模組用到的（目前有圖片的 `Image`、錯誤原因的 `ErrorReason`、各處顯示一家店的 `StoreCard`、一組期限的 `Deadline`（開始與到期，伺服器給，client 只拿現在比較）與政策分類的 `PolicyCategory`——seller 與 A7 的 moderation 共用，Go 的值與各入口的子集在 `internal/platform/policy`）。
- 後台獨立成 `hawkr.admin.*`：同名資源在前後台的欄位不同（前台逐欄列出，見第五節），分開 package 才能各自叫 `Banner`；website 與兩支 App 以一條 `exclude_paths: [hawkr/admin]` 就不生成後台 API。
- 版本：上線前一律 `v1alpha1`（AIP-185），`buf breaking` 不檢查，契約可以直接改。上線時一次改名成 `v1`，之後 `WIRE_JSON` 的 breaking 才生效（`api/buf.yaml`）。
- lint 是 buf 的 STANDARD。唯一的例外是 `common` 的 `error_reason.proto` 不檢查 `ENUM_VALUE_PREFIX`：值名就是 `ErrorInfo.reason` 的字串（第八節）。proto 裡不寫語言相關的選項（`go_package` 等），由各消費端的 `buf.gen.yaml` 以 managed mode 決定。

## 二、Service 與授權

- 一個使用者類別一個 service，授權跟著 service 走：

  | service 名稱 | package | 誰能呼叫 | 單體 |
  |---|---|---|---|
  | `<模組>PublicService` | `hawkr.<模組>.*` | 任何人 | 不需要登入；有帶有效的 token 時 ctx 裡有呼叫者 |
  | `<模組>AppService` | `hawkr.<模組>.*` | 登入的帳號（買家、賣家） | 沒有登入回 `UNAUTHENTICATED`，管理員回 `PERMISSION_DENIED` |
  | `<模組>InviteeService` | `hawkr.<模組>.*` | 兌換了邀請的供應商（沒有帳號） | 不讀顧客身分；憑證是 service 自己的 interceptor 驗證的 cookie，沒有或無效回 `UNAUTHENTICATED`（見下方 vendor portal 的 session） |
  | `<模組>AdminService` | `hawkr.admin.<模組>.*` | 管理員 | 沒有登入回 `UNAUTHENTICATED`，帳號回 `PERMISSION_DENIED`；管理員的角色不允許該方法宣告的權限（`internal/httpapi/adminauth`，只有 `GetSession`、`SignOut` 豁免）也回 `PERMISSION_DENIED` |

- 單體是對外的入口（遷移計畫 A2）：登入由 identity 處理（Google、LINE 聯合登入，憑證是 opaque session），入口的中介層以 identity 解析 session token（App 以 `Authorization: Bearer` 帶；web 是同源的 httpOnly cookie `__Host-hk_session`，瀏覽器直接導覽的下載也靠它；兩者都有時以 Bearer 為準，query、`token` header 與其他 scheme 一律不讀；cookie 請求的 CSRF 規則見 ADR D6），把呼叫者放進 ctx（`rpc.PrincipalFrom`）。決策見 ADR `2026-10-30-identity-self-hosted-federated-login.md`。token 無效、逾時或帳號已刪除時以訪客繼續，停權的帳號照樣認證、授權依動作性質（A7-D18），後台的使用者不是允許的帳號時回 `PERMISSION_DENIED`，問不到公鑰或資料庫時回 `UNAVAILABLE`，不當成訪客。帳號與管理員的 ID 各自發號，呼叫者一定連同類型判斷。
- 角色與視角：每個帳號都是買家（帳號活著就是買家，ADR `2026-09-30-account-created-provisions-buyer`），買家不是角色；角色只有要另外申請的賣家（`Role.ROLE_SELLER`），每個請求從資料庫讀，變更下一個請求就生效。Connect 沒有視角（ADR `2026-10-07-connect-without-perspective`）：Connect 的服務名稱（`BuyerAppService`、`SellerAppService`）本身就表示以哪個身分呼叫（資源被兩種身分共用的模組，例如 social，用 `<模組>AppService`，身分由伺服器依資源歸屬推導，見 ADR `2026-10-07-connect-without-perspective` 文末修訂），client 不送 `X-Active-Role`，伺服器不讀它，`GetSession` 不回視角；「正在看哪個工作區」是 client 的 UI 狀態。只有賣家能做的事，由模組檢查帳號有沒有賣家角色（`Principal.HasRole`），不是授權的另有「視角」。`X-Active-Role`（`BUYER` 或 `SELLER`）只留在舊 REST、SSE、AI chat 與 chat ws-ticket 的送出端：舊 gateway 在同一條路由內以 `X-Login-Role` 選擇以買家或賣家身分動作，`internal/legacy/gatewayproxy` 從轉發的請求讀它、以 `internal/legacy/accounttype.OfLoginRole` 推導（`SELLER` 而且帳號有賣家角色時是賣家，其他情況是買家，管理員沒有）。最後一個讀 loginRole 的 gateway handler 或 IM 消費端退場時，header 與推導一起刪除。
- service 以外更細的條件（例如只有賣家、只能改自己的資料）在模組裡判斷。
- 賣家區的能力宣告（ADR `2026-09-29-seller-suspension-and-business-identity-in-monolith.md`；文末修訂：資源被買家與店家共用的模組，其 `AppService` 也可以宣告以店家身分動作時需要的賣家能力，由擁有資料的模組推導身分後再向 seller 檢查，宣告表各 service 一張，下面描述 `SellerAppService` 的做法；沿用 `2026-09-18-seller-capability-authorization.md` 的 D1「宣告＝掛載」）：`SellerAppService` 的每個方法在 `internal/httpapi/seller` 的宣告表（key 是生成的 procedure 常數）宣告需要的賣家能力（`seller.Cap*`），或明列豁免與理由。只掛在這個 service、位在 `rpc.NewInterceptor` 之內的 interceptor 依宣告呼叫 `seller.ActingStore`，把店家放進 ctx；handler 只從 ctx 取店家，不自己呼叫 `ActingStore`。豁免的方法不查店家，也不要求賣家角色（例如申請開店）。啟動時以 service descriptor 雙向對照宣告表，方法缺宣告或宣告指向不存在的方法，`NewHandler` 回錯誤。能力由 `seller.CapabilitiesOf` 推導：生命週期、停權與前置條件（物流建店要 KYC 流程核准），`GetMyStore.capabilities` 回報每個能力的 `{active, missing}`（`missing` 是 `SellerRequirement`），狀態 `PUBLIC_LISTING`（買家看得到店面）只出現在那裡，方法不能宣告它。停權的拒絕是 `permission_denied`，`ErrorInfo.reason` 是 `SELLER_SUSPENDED`；其他前置條件沒有滿足（等待刪除、KYC 未核准）是 `failed_precondition`，`ErrorInfo.reason` 是 `SELLER_REQUIREMENT_MISSING`，metadata 的 `missingRequirements` 是缺的條件（`SellerRequirement` 的值名，不加前綴，依列舉的數字由小到大以逗號連接，例如 `KYC_APPROVED,NOT_PENDING_DELETION`）。
- 越權存取（已登入、資源存在但不屬於呼叫者）預設回 `not_found`，code、`LocalizedMessage` 與 details 都和「不存在」完全相同，寫入的方法也一樣，不能讓人以回應的差異探測資源是否存在（防枚舉）。只有資源的存在與 ID 本身就公開（例如公開的貼文）時才回 `permission_denied`。未登入、使用者類別不符不適用本條，照常回 `unauthenticated`、`permission_denied`；以伺服器簽發的憑證（例如 staged token）識別的資源，憑證無效回 `invalid_argument`。
- 瀏覽器跨來源：CORS 只在入口處理，允許設定的 web 與後台的 origin 帶 cookie；CSRF 以 Go 標準庫的 `http.CrossOriginProtection`（`Sec-Fetch-Site` 或 Origin）擋下其他 origin 的非安全請求，金流與物流的瀏覽器回跳列為例外。client 不必送 `X-Requested-With`。
- **單體→gateway 的身分**（過渡期；還沒搬的 REST、SSE 與 WebSocket 由 `internal/legacy/gatewayproxy` 轉給 gateway）：入口先刪掉 client 帶來的 `X-Account-Id`、`X-Account-Type`、`X-Login-Role`、`X-Active-Role`、`X-Buyer-Id`、`X-Store-Id`、`X-Seller-Suspended`、`X-Seller-Capabilities`，登入時再設上：

  | header | 值 |
  |---|---|
  | `X-Account-Id` | 帳號或管理員的 ID |
  | `X-Account-Type` | 舊契約的角色 bitmask：帳號一定有買家 1，是賣家時再加 2（兩者 3）；管理員 4。買家位元由 `internal/legacy/accounttype` 合成，identity 不存 |
  | `X-Login-Role` | 這個請求的視角：買家 1、賣家 2、管理員 4 |
  | `X-Buyer-Id` | 帳號（非管理員）現任買家的正整數 ID |
  | `X-Store-Id` | 非管理員且具有 SELLER 角色的帳號，其現任店家的正整數 ID；沒有角色時不設（有 SELLER 卻沒有現任店家是不變量被破壞，入口回 500、不轉發） |
  | `X-Seller-Suspended` | 現任店家停權時的原因：管理員停權是 `9601`、`9602`、`9603`，治理停權是 `SUSPENDED_GOVERNANCE`；沒有此 header 表示未停權 |
  | `X-Seller-Capabilities` | 現任店家目前的能力，逗號分隔的能力 token（`CAP_PRODUCT_MANAGE`、`CAP_ORDER_FULFILL` 等，與 gateway 的 `authz` 能力 token 同一組字串），由 seller 模組推導（生命週期、停權與前置條件，例如物流建店要 KYC 流程核准；等待刪除沒有經營類能力），只放啟用的動作類能力（不含店家狀態 `PUBLIC_LISTING`），是推導的唯一一處。與 `X-Store-Id` 同時出現：有 `X-Store-Id` 卻沒有此 header、此 header 空值、帶不認得的 token、沒有 `X-Store-Id` 卻有此 header，都是契約錯誤，gateway 當成沒有身分（fail-closed）。gateway 只檢查要求的能力是否在集合內，不自己從停權或生命週期推導；被拒絕時店家停權（有 `X-Seller-Suspended`）回業務碼 9240，其餘（例如等待刪除、前置條件沒有滿足）回 403。header 不往下游服務傳 |

  業務身分只在轉發路徑解析，不影響 Connect handler；每個帳號都查 buyer_id，有賣家角色才查店家，管理員與訪客不查也不帶上述的店家與買家 header。帳號活著 ⇔ 買家活著（ADR `docs/adr/2026-09-30-account-created-provisions-buyer.md`）：存活的帳號查不到現任買家是不變量被破壞，入口記 error log 並回 HTTP 500，JSON 為 `{"code":500,"message":"系統錯誤，請稍後再試"}`，不轉發。有 SELLER 角色的帳號查不到現任店家（沒有或已關閉）同樣是不變量被破壞（關店與移除 SELLER 在同一個 transaction），回 HTTP 500、不轉發；其他錯誤（含停權原因缺失或不合法、店家沒有任何能力）由入口記 log 並回 HTTP 503，JSON 為 `{"code":503,"message":"服務暫時無法使用，請稍後再試"}`，不轉發。gateway 驗證 ID 為正 int64、停權原因符合值域、能力集合的每個 token 都認得，並記錄身分來自單體；解析呼叫者自己的業務 ID 時使用此快照，header 缺省即不存在，不再查 RPC。其他帳號或非單體身分來源（例如 ws-ticket）直接查 RPC，不使用 Redis 快取。ws-ticket 沒有店家與能力集合，要求能力的賣家路由一律拒絕（聊天的 WebSocket 升級本來就不要求能力）。

  gateway 信任它們的前提是只有單體連得到 gateway。gateway 與舊服務不會部署（後端 CD 以遷移完成後的單一 image 為對象），這個前提只在本機與 CI 成立；「只有入口連得到 gateway、入口刪掉 client 帶的身分 header」仍列在 CD 的待辦，以防順序改變。

- **vendor portal 的 session**（ADR `2026-10-03-vendor-portal-invite-scoped-access.md` 2026-10-04 兩次修訂）：供應商沒有帳號，憑 `VendorPortalPublicService.ExchangeInvite` 設定的 cookie 進入，不經顧客登入，也不是 ctx 裡的呼叫者（`rpc.PrincipalFrom`）（`rpc.Credential` 不變）。瀏覽器可以同時持有多張邀請的 cookie，每個以 cookie 認證的請求都指名要用哪一張。

  | 項目 | 值 |
  |---|---|
  | cookie | 每張邀請一個：`__Host-hk_vp_<rfq_recipient_id>`（十進位，名稱與屬性只在 `rpc.VendorPortalCookieName`、`rpc.NewVendorPortalCookie` 建構）。`Path=/`、`Secure`、`HttpOnly`、`SameSite=Lax`，不帶 `Domain`；`Max-Age` 是 session 到絕對期限（已被邀請的到期時間截斷）剩餘的秒數，至少 1。`__Host-` 讓兄弟子網域塞不進同名的 cookie；`Path=/` 是為了同時送到 Connect 與 gateway 的 RFQ 路由（Path 不能用來區分邀請）。同名的 cookie 出現不只一個視同沒有 |
  | 怎麼選 | Connect：請求訊息的 `rfq_recipient_id`（`VendorPortalInviteeService` 每個方法的請求都有，必填、十進位字串）；gateway 的 RFQ 路由：路徑 `/api/v1/public/vendor_portal/rfqs/{rfq_recipient_id}/…` 的 `{rfq_recipient_id}`。只讀名稱含那個 ID 的 cookie，其他邀請的 cookie 不讀 |
  | 誰讀 | `VendorPortalInviteeService` 的 interceptor（`internal/httpapi/agent`）：授權跟著 service 的邊界，這個 service 的每個方法都需要 session，沒有逐方法的宣告；沒有指名的 cookie、session 無效或 session 的收件人不是指名的那個，一律回同一個 `unauthenticated`；請求沒有 `rfq_recipient_id` 欄位（新增了不帶它的方法）回 `internal`，不放行。轉給 gateway 的 RFQ 路由由 `gatewayproxy` 以路徑的 ID 驗證（見下） |
  | 兌換 | `ExchangeInvite` 先解出 secret 對應的邀請；請求帶這張邀請的 cookie 而且它是這張邀請的有效 session 就沿用：不發新的 session、不重設 cookie（它的 `Max-Age` 已是同一個絕對期限），回應是那個 session 的驗證狀態與 `google_nonce`；沒有、無效或不是這張邀請的就發新的 session（新的 `Set-Cookie` 蓋掉舊的）。別張邀請的 cookie 不動 |
  | 上限 | 請求帶的 `__Host-hk_vp_*` cookie 超過 20 個時，`ExchangeInvite` 的回應以 `Max-Age=0` 清掉多出的，不含這次發出或沿用的那個。看不到 cookie 的年齡，所以清收件人 ID 最小的（ID 依序發號，小的是較舊的邀請），格式不對的名稱最先清；結果只取決於請求帶的名稱（`rpc.ExcessVendorPortalCookies`） |
  | Google 驗證的 nonce | `google_nonce` = base64url（無 padding）的 SHA-256（用途前綴 `"hawkr vendor-portal google nonce\x00"` 加 session token），每次由 token 算、不儲存，與資料庫存的 token 雜湊是不同的輸入。`ExchangeInvite` 與 `GetSession` 回給 client，client 傳給 Google Identity Services 的 `initialize({nonce})`；`VerifySession` 驗 ID token 時要求 `nonce` claim 相等、`iat` 不早於 10 分鐘前（OIDC Core §3.2.2.1、§15.5.2），不符與其他無效 token 同樣回 `permission_denied`（同一段訊息） |
  | 快取 | `PublicService` 的兩個方法是 POST、回應 `private, no-store`；`InviteeService` 的回應一律 `private, no-store`，包括可用 GET 的 `GetSession`（第九節） |
  | 額度 | `PreviewInvite` 與 `ExchangeInvite` 共用每個來源 5 分鐘 20 次；`VerifySession` 每個來源 10 分鐘 10 次；`GetSession` 與 `InviteeService` 的其他方法用該類別的一般額度，每個來源每分鐘 300 次（與 App、Admin 相同，單獨計數） |

  **單體→gateway 的 vendor portal 身分**（過渡期，到 A9 把 RFQ 路由改成 Connect 為止）：入口對每個轉給 gateway 的請求先刪掉所有以 `X-Vendor-Portal-` 開頭的 header（不分大小寫）；只有路徑在 `/api/v1/public/vendor_portal/rfqs/{rfq_recipient_id}/` 底下、`{rfq_recipient_id}` 是正整數的十進位寫法、那張邀請的 cookie 是有效 session 而且 session 的收件人就是 `{rfq_recipient_id}` 時才設上（只讀 `__Host-hk_vp_<{rfq_recipient_id}>`，別張邀請的 cookie 不能為這個路徑帶身分）：

  | header | 值 |
  |---|---|
  | `X-Vendor-Portal-Rfq-Recipient-Id` | 邀請對應的 RFQ 收件人 ID（十進位 int64） |
  | `X-Vendor-Portal-Session-Id` | session 的 UUID |
  | `X-Vendor-Portal-Verification-Method` | session 已驗證時：`GOOGLE`；未驗證時沒有 |
  | `X-Vendor-Portal-Verified-Email` | session 已驗證時：Google 驗證過的 email；未驗證時沒有 |
  | `X-Vendor-Portal-Verified-At` | session 已驗證時：驗證時間（RFC 3339，UTC，保留奈秒：與 agent 記錄的時刻相同）；未驗證時沒有 |

  沒有 cookie、session 無效、路徑的 ID 格式不對或 session 屬於別的收件人時不設任何 `X-Vendor-Portal-*`（gateway 回 401）；驗證 session 時資料庫等出錯，入口記 log 並回 HTTP 503，JSON 為 `{"code":503,"message":"服務暫時無法使用，請稍後再試"}`，不轉發。信任的前提與上面的身分 header 相同。

## 三、方法

- 標準方法照 AIP：`Get`（131）、`List`（132）、`Create`（133）、`Update`（134）、`Delete`（135）。其他動作是自訂方法（136），動詞加名詞，例如 `CheckAppVersion`。
- 每支 RPC 有自己的 `<方法>Request`、`<方法>Response`（buf lint 的 `RPC_REQUEST_STANDARD_NAME`、`RPC_REQUEST_RESPONSE_UNIQUE`）。與 AIP 的差異：`Get`、`Create`、`Update` 回傳 `{ <資源> <資源> = 1; }`，不直接回資源；`Delete` 回傳空的 `DeleteXResponse`，不用 `google.protobuf.Empty`。
- 集合的讀取依語意分成三種，不混用：

  | 方法 | 用途 | 分頁 |
  |---|---|---|
  | `List`、`Search` | 列舉整個集合，或符合條件的全部結果 | 一律分頁（第六節） |
  | `Suggest` | 依相關度排序的前幾筆（搜尋建議） | 不分頁，`max_results` |
  | `Sample` | 隨機挑幾筆 | 不分頁，`max_results` |

  集合大小由代碼表固定的 `List`（例如 AppType × AppPlatform 的版本門檻）可以不分頁，在方法說明寫明上限與依據。
- 沒有副作用的讀取標 `option idempotency_level = NO_SIDE_EFFECTS;`，client 以 GET 呼叫（第九節）。讀取不得順便寫入（例如記錄瀏覽數）；要記錄就另外設計寫入的方法。請求帶 bearer 憑證的讀取（例如以邀請的 secret 預覽）不標，以 POST 呼叫、憑證放在請求訊息：GET 會把憑證放進 URL，公開 service 的 GET 還會被共享快取（第九節；ADR `2026-10-03-vendor-portal-invite-scoped-access.md` 修訂）。請求可能超過網址長度上限的讀取（例如帶整張表單內容的比對預覽 `PreviewProspect`）也不標 `NO_SIDE_EFFECTS`，改標 `option idempotency_level = IDEMPOTENT;`、以 POST 呼叫，仍可自動重試（第八節）。憑證在 cookie 的讀取（`InviteeService.GetSession`）屬於 `InviteeService`，可以標 `NO_SIDE_EFFECTS` 以 GET 呼叫：回應一律 `private, no-store`。

## 四、資源與欄位

- 欄位名稱是 snake_case；JSON 由 proto3 的對應轉成 lowerCamel（`button_text` → `buttonText`）。
- ID 是 `string id`，值是 UUID，client 當成不透明的字串，不解析、不轉數字（ADR `2026-09-25-row-identity-by-data-kind.md` D5）。不用 AIP-122 的資源名稱（`recipes/{id}`）：資源都只有一層，資源名稱只會讓每個 client 多一段拆解。以自然鍵識別的資源，直接用鍵的欄位（例如版本門檻的 `app_type`）。
- 時間是 `google.protobuf.Timestamp`，名稱以 `_time` 結尾（AIP-142）：`create_time`、`update_time`、`start_time`。JSON 是 RFC 3339 的 UTC（`2026-09-25T04:30:00Z`）。
- 數字：金額與可能超過 2^53 的值用 `int64`（JSON 是字串，JavaScript 不會失真）；筆數、分鐘、份量這類有界的值用 `int32`（JSON 是數字）。這是 proto3 JSON 的對應，與 `backend/AGENTS.md`「數量、計數用字串」不同：那條規則要防的是 JavaScript 的精度，`int32` 沒有這個問題。金額是新台幣「元」的整數，禁止浮點。
- 布林不加 `is_` 前綴（AIP-140）：`active`，不是 `is_active`。
- `optional`：零值是合法的值、而且要與「沒有填」區分時用，例如 `optional int32 servings`；以及 enum 的「沒有值」是業務狀態（使用者沒有填、店家還沒有申請）時，例如 `optional Gender gender`、`optional KYCStatus kyc_status`，這時不接受 `UNSPECIFIED`。後者是本庫的取捨（L-brief Q4，Will 2026-10-17）：AIP-149 只建議整數與浮點數用 `optional`，AIP-126 的零值是「不使用」；我們選擇以欄位有沒有設值表達業務上的「沒有」，讓零值不承載意義、client 不必替它找文字。訊息型別（`Timestamp`）本來就能區分有沒有值。
- 列舉（ADR `2026-09-25-codemeta-retirement.md`）：
  - client 會分支的封閉集合用 proto enum，放在擁有它的模組的 package；零值 `<ENUM>_UNSPECIFIED = 0`，值帶型別名前綴。client 遇到 `UNSPECIFIED` 或不認得的值，走「未知」的呈現，不當成錯誤。`optional` 的 enum 欄位，JSON 解碼時不認得的值與沒有設值無法分辨，呈現同「沒有設值」。
  - 開放集合（第三方的原生碼、分類、常變的選項）不做成 enum：用有文件說明的 `string`，或做成可查詢的資源（AIP-126）。
  - 資料庫存的值＝模組 Go 常數的值＝proto 值名去掉型別前綴（UPPER_SNAKE），轉換由傳輸層做。新契約不引用 CodeMeta。
- 圖片（ADR `2026-09-25-api-image-uri-template.md`）：回共用的 `hawkr.common.<版本>.Image`，欄位用語意名（`preview_image`），沒有圖時不設；`uri_template` 由伺服器組，client 只把 `{preset}` 換成 preset 名稱，不解析、不改寫其他部分。object key 不在對外契約。上傳後的新圖以 `<語意>_staged_token` 傳入；沿用目前的圖時不把該欄位列進 `update_mask`。換圖與文字欄位在同一個 `Update`：模組先在 transaction 外轉正 staged token，再於同一個版本條件的 transaction 寫入文字與圖片的 key，版本衝突時整筆不寫入，轉正之後沒有被引用的新圖與被換掉的舊圖由收斂 job 釋放。圖片要附帶隨圖一起換的業務資料（例如商家名單的來源網址）時，新圖改以 `INPUT_ONLY` 的子訊息 `<語意>_upload` 傳入（`staged_token` 加那些欄位），回應另以 `OUTPUT_ONLY` 欄位呈現；業務資料存在引用方，不在 media（ADR `2026-10-03-media-references-owner-side.md`）。
- 欄位的行為用 AIP-203 的 `google.api.field_behavior` 標在 proto 上，不寫在註解（Will 2026-09-26 決定）：
  - `IDENTIFIER`：資源的 `id`。`OUTPUT_ONLY`：只出現在回應（圖片、`create_time`、`update_time`）。`INPUT_ONLY`：只用在請求（`*_staged_token`）。`IMMUTABLE`：建立後不能修改（版本門檻的 `app_type`）。`REQUIRED`：建立時必填（依方法才必填的，例如 `Update` 的 `etag`，寫在請求訊息的註解）。
  - 傳輸層依標註處理 `update_mask`（`rpc.ApplyUpdateMask`）：`OUTPUT_ONLY`、`IDENTIFIER` 的欄位列在 mask 裡也不複製、不回錯；`IMMUTABLE` 的欄位與目前的值不同時回 `INVALID_ARGUMENT`。
  - 驗證只在模組一處：`REQUIRED` 是契約的宣告，由各消費端的測試確認模組真的會擋（content 的 `TestRequiredFieldsAreEnforced` 從 descriptor 找出每個 `REQUIRED` 欄位，清掉之後錯誤要指出它的路徑）。不用 protovalidate：規則會分成 proto 與模組兩處（模組在單體內也會被直接呼叫），Dart 也沒有官方的執行環境。

## 五、前台與後台的訊息分開

- 前台的訊息逐欄列出前台需要的欄位，不與後台共用型別（deny-by-default：沒有明列的欄位就不對外）：例如前台的 `Banner` 沒有排程、啟用狀態、編輯器狀態、etag。
- 值型別（例如 `BannerAction`、`Ingredient`）兩邊相同時，後台 import 前台的定義，不複製一份。

## 六、分頁（AIP-158）

- 請求：`int32 page_size`、`string page_token`；回應：`string next_page_token`，空字串表示最後一頁。
- `page_size` 為 0 時用預設值；超過上限時以上限計；負數回 `INVALID_ARGUMENT`。預設值與上限寫在欄位註解。
- `page_token` 是不透明的字串：內容是上一頁最後一筆的排序鍵（keyset，不是 offset，翻頁途中有新增或刪除也不會重複或漏掉）與請求其他參數的指紋。其他參數與取得 token 時不同，或 token 無法解讀，回 `INVALID_ARGUMENT`。
- 需要總數時才加 `int32 total_size`；目前沒有畫面需要。

## 七、修改與並發

- 並發修改以 etag 做樂觀鎖，規則在 ADR `2026-09-25-optimistic-concurrency-etag.md`，這裡是 Connect 的對應：
  - 可修改的資源帶 `string etag`，client 當成不透明的字串。
  - `Update`：資源裡的 `etag` 必填，沒帶回 `INVALID_ARGUMENT`；與目前版本不符回 `ABORTED`，`ErrorInfo.reason` 是 `VERSION_CONFLICT`，不寫入。
  - `Delete`：請求的 `etag` 選填，帶了就同樣檢查。
  - 改變狀態的自訂方法（核准、合併、封存等）：決定依據的內容在狀態不變時仍可能被改的，請求的 `etag` 必填，檢查同 `Update`；決定只依據狀態機已經檢查的狀態、或依據不可變的內容的，不帶 etag（ADR `2026-09-25-optimistic-concurrency-etag.md` 修訂）。
  - 以 key 識別的資源（ADR 的 D3）：`Create` 與 `Update` 分開。`Create` 不帶 etag、只建立，已經存在回 `ALREADY_EXISTS`（AIP-133），`ErrorInfo.reason` 同樣是 `VERSION_CONFLICT`；`Update` 必須帶 etag。
  - client 依 `reason` 是 `VERSION_CONFLICT` 處理，不看是 `ABORTED` 還是 `ALREADY_EXISTS`：保留使用者的輸入，並顯示最新版本讓使用者對照後再送出，不直接重新載入。
  - 以呼叫者為範圍、可以取消後重建的資源（例如店家的 KYC 申請：每個店家同時最多一筆，沒有 ID 的定址）只有一支 `Save<資源>`，沒有 `Create`。這與 AIP-156 的單例資源不同（AIP-156 的單例隨父資源隱含存在：Get 一定成功、只有 Update、沒有建立與刪除）；它是以呼叫者為範圍的 upsert，近似 AIP-134 的 `allow_missing`：還沒有資源時，不帶 etag 的呼叫建立它（建立必填的欄位在方法說明寫明）；已經有資源卻沒帶 etag 是 `ALREADY_EXISTS`（`VERSION_CONFLICT`，與以 key 建立撞到既有同形狀）；帶了過期的 etag 是 `ABORTED`（`VERSION_CONFLICT`）。取消是狀態（取消後 Get 回 `NOT_FOUND`），之後可以再建立。
- 部分更新用 `google.protobuf.FieldMask update_mask`（AIP-134）：
  - 只改列出的欄位。沒帶時視為列出所有非零值的欄位，所以要清空或改成零值的欄位必須列出來。`"*"` 表示所有可修改的欄位。
  - 路徑只支援最上層的欄位（`action` 整個替換，不支援 `action.url`），唯一的例外是下到 `oneof` 的子訊息（`individual.id_card_no`，見下面「互斥的型態」）；不認得的路徑回 `INVALID_ARGUMENT`。
  - repeated 欄位在 mask 裡時整批替換。
  - 表單送出時列出表單上的欄位，不用 `"*"`：日後資源新增欄位時，舊版的 client 不會把它清掉。
- 單體的實作：transport 讀目前的資源、把 mask 內的欄位套上去，再以整筆的內容與 etag 代表的版本呼叫模組；版本由模組比對：轉正圖片之前比對一次，最後以條件式 UPDATE（`WHERE version = $n`）為準。回應的資源與 etag 取自寫入當下（`RETURNING`），不在 commit 之後另外讀。
- 單體實作的第二種做法：資源有 `INPUT_ONLY` 的欄位，或只讀得到遮蔽後的值（身分證號、銀行帳號）時，目前的值不在請求裡，無法「先合併再整筆送」。這時 transport 以 `rpc.WritableFields` 取得 mask 選到、而且呼叫者可以寫入的欄位路徑，只把選到的欄位送給模組：沒選到的是「保留」，選到卻沒有值的是「清掉」。可寫的欄位集合完全取自 proto 的 descriptor 與 `field_behavior`，handler 不維護欄位清單：`OUTPUT_ONLY` 與 `IDENTIFIER` 在這個 helper 裡略過（只有這一處）；`IMMUTABLE` 的欄位，第二種做法拿不到目前值、無法比對新舊，所以 mask 明列它時回 `INVALID_ARGUMENT`（沒帶 mask 或 `"*"` 時略過；有 `IMMUTABLE` 欄位又需要比對的資源，用上一條的做法）。其他資源用上一條的做法。
- 互斥的型態（個人或企業賣家）用 `oneof` 的兩個子訊息表達，型態就是設定的是哪一支，沒有另外的型態欄位；兩種型態共用的欄位在最上層，各自專屬的在那一支裡面。`update_mask` 可以下到那一支的欄位（`individual.id_card_no`、`business.tax_id`），列到那一支本身是它全部可寫的欄位；`"*"` 沒有特例：所有最上層的可寫欄位，oneof 只展開設定的那一支，沒有帶 mask 時是有值的欄位。其他子訊息（食品業者、日期、證件）整個換掉，路徑停在那個欄位，不能再往下。mask 列到的欄位在另一支底下是請求自相矛盾（`INVALID_ARGUMENT`）。建立時必須設定其中一支；後來設定另一支並且 mask 列到它的欄位就是切換型態，模組清掉原本那一支專屬的欄位。
- 一起修改的欄位（食品業者的「是不是加工食品」與登錄字號）做成一個子訊息，mask 列子訊息的欄位就整個換掉；不為 mask 加「要一起列」的規則。證件圖片是狀態而不是指令：單張的位置是欄位，列進 mask 就整個換成請求的那一張；多張的位置（repeated）整批換成請求的清單，每個元素是現有的圖片（asset ID，保留）或新上傳的圖片（staged token）；同一張圖片不能放進兩個位置。

## 八、錯誤

規則與理由在 ADR `2026-09-25-api-error-model.md`，這裡是摘要：

- Connect code 表示類別，依 gRPC 的判準選：重試這一次就可能成功用 `unavailable`，要重讀再重做修改用 `aborted`，要等狀態被修正用 `failed_precondition`。
- details 只用 `google/rpc/error_details.proto` 的標準型別，每種最多一個：
  - `ErrorInfo`：client 需要在同一個 code 底下分支時才帶；`reason` 是 `hawkr.common.<版本>.ErrorReason` 的值名（不加前綴），`domain` 是 `hawkr`，`metadata` 放 client 需要的動態值，key 是 lowerCamelCase，清單的值以逗號連接、由小到大排列（例如 `PROSPECT_MATCH_FOUND` 的 `prospectIds`）。client 依它做事的 reason 只搭配 4xx 的 code。
  - `LocalizedMessage`（`zh-TW`）：伺服器給使用者的文字，只從這裡來。
  - `BadRequest`：`invalid_argument` 能歸到欄位時，`field` 是請求訊息的 proto 路徑，每個違規帶 `localized_message`。模組以自己輸入的欄位名回報（`apperr` 的 `InvalidInputOn`、`InvalidInputFields`，欄位只能在建立錯誤時給），handler 以 `rpc.WithFieldPaths` 對應到請求的路徑；對應不到的欄位不送出，interceptor 以 error 等級記 log。repeated 欄位裡單一元素的問題，`field` 帶索引（AIP-193，例如 `evidence_staged_tokens[1]`），模組回報 `X[i]`，`FieldPaths` 要明列 repeated 目標，每個 repeated 欄位只列一筆（key `X[]`、value 以 `[]` 結尾，例如 `"EvidenceTokens[]": "evidence_staged_tokens[]"`），沒有明列的索引欄位（單值欄位被回報成 `Reason[2]`）當成對應不到；`CheckFieldPaths` 以 descriptor 驗證帶 `[]` 的每一段都是 repeated 欄位；整個欄位的問題（張數上限）不帶索引（模組回報 `X`），由同一筆 `X[]` 對應（去掉結尾的 `[]`，例如 `evidence_staged_tokens`），不必另列不帶 `[]` 的 key。
  - `RetryInfo`：伺服器知道多久之後能重試時（入口的限流、驗證碼的冷卻）。
- `message` 給開發者看，client 不顯示、不解析。trace id 在回應的 `X-Trace-Id` header。
- 每個錯誤回應有一筆 log：interceptor 記它經手的；connect-go 在 interceptor 之前就回的錯誤（請求無法解碼、超過大小上限）由 HTTP 層的 `rpc.LogUnhandled` 補上。
- 單體的轉換點（`internal/httpapi/rpc`）只轉換模組的 `apperr` 與傳輸層以 `rpc.Error` 建立的錯誤，其他一律 `internal`（訊息固定、細節只進 log），下游回的 Connect 錯誤也不例外：

  | 模組錯誤（apperr） | Connect | details |
  |---|---|---|
  | `InvalidInput` | `invalid_argument` | `LocalizedMessage`；有欄位時加 `BadRequest` |
  | `NotFound` | `not_found` | `LocalizedMessage` |
  | `VersionConflict` | `aborted` | `ErrorInfo{VERSION_CONFLICT}`、`LocalizedMessage` |
  | `AlreadyExists` | `already_exists` | 模組帶的原因（以 key 建立時是 `VERSION_CONFLICT`）、`LocalizedMessage` |
  | `FailedPrecondition` | `failed_precondition` | `LocalizedMessage`；模組帶原因時加 `ErrorInfo`（例如 `PROSPECT_MATCH_FOUND`、`SELLER_REQUIREMENT_MISSING`，metadata 放 client 要的動態值） |
  | `Unauthenticated` | `unauthenticated` | `LocalizedMessage`（例如登入用的 token 無效） |
  | `PermissionDenied` | `permission_denied` | `LocalizedMessage`（例如帳號已停權）；賣家停權加 `ErrorInfo{SELLER_SUSPENDED}` |
  | `Unavailable` | `unavailable` | `LocalizedMessage`（例如沒有設定的登入方式） |
  | `ResourceExhausted` | `resource_exhausted` | `LocalizedMessage`；知道多久後能再試時加 `RetryInfo` |
  | 沒有分類 | `internal` | 無 |
  | （interceptor）沒有身分 | `unauthenticated` | `LocalizedMessage` |
  | （interceptor）身分不符 | `permission_denied` | `LocalizedMessage` |

- 入口的中介層在 interceptor 之前擋下的請求（限流、認證時問不到資料庫、不是允許的後台帳號）由 connect-go 的 `ErrorWriter` 寫出 Connect 的錯誤（`rpc.RetryLater`、`rpc.UserError`），不帶 `ErrorInfo`。
- client（四端相同）：`canceled` 不處理；方法可安全重送（`NO_SIDE_EFFECTS`、`IDEMPOTENT` 或帶 `request_id`）且錯誤可重試（`unavailable`、沒收到回應的傳輸失敗、帶 `RetryInfo` 的 `resource_exhausted`）才自動重試；`unauthenticated` 回到登入（session 由伺服器發放與撤銷，ADR `2026-10-30-identity-self-hosted-federated-login.md`）；認得的 reason 走它的流程；有 `BadRequest` 就標在欄位；其他顯示 `LocalizedMessage`，沒有就顯示該 code 的預設文字。

## 九、讀取與快取

- `NO_SIDE_EFFECTS` 的方法 client 以 GET 呼叫（connect-es 的 `useHttpGet`、connect-go 的 `WithHTTPGet`），請求編進 query。
- 回應的 `Cache-Control`：

  | 類別 | 值 |
  |---|---|
  | `PublicService` 的讀取 | `public, no-cache`，加弱 `ETag`；client 與 CDN 每次都向來源確認，沒變時回 304 |
  | `AppService`、`AdminService`、`InviteeService` 的所有回應 | `private, no-store`；`InviteeService` 的讀取可以用 GET（以 cookie 認證），回應仍不可快取，沒有 `ETag` |

  公開讀取先不設 `max-age`：website 在寫入後以 tag 立即失效（`platform/revalidate`），而 CDN 若在 `max-age` 內回舊資料，website 會把舊資料再快取一輪。要設 `max-age` 的方法，連同 website 的失效流程在 CD 驗證 Cloudflare 的行為時一起決定。
- 資源的 etag（第七節）與 HTTP 的 `ETag`（條件請求、304）是兩件事：前者是版本，後者是回應內容的雜湊。

## 十、大小與逾時

- 請求大小上限由單體的 `connect.WithReadMaxBytes` 決定，2 MiB，與 gateway 的 `BodyGuard` 一般路由相同；模組的文字欄位不設長度上限，只檢查產品規格訂的上限。
- 逾時：client 以 `Connect-Timeout-Ms` 帶；沒帶時單體套用預設的上限（interceptor）。

## 十一、wire 格式與串流

- web 用 JSON（除錯方便）。App 也用 JSON，並接受 gzip：gzip 之後 binary 只再小 15–20%，解碼的差距以微秒計（試點 6.7 的量測）。出現大量或頻繁的回應（清單分頁、串流），或裝置上的解碼成為可見的延遲時，重新評估 binary。
- 串流只用 server streaming。AI 助理的 AG-UI 事件格式是否改成 proto 串流另外決定。
- 檔案上傳用 presigned URL 直接上傳到儲存空間，API 只處理 metadata；媒體的 Range／206 由儲存空間或 CDN 直接提供。

## 十二、生成

- 每個生成端用自己目錄的 `buf.gen.yaml` 生成，外掛版本鎖在各自的 lockfile，生成物不得手改，CI 檢查 drift（根目錄 `AGENTS.md` 的跨專案契約表）。TypeScript 只有一個生成端：共用套件 `@hawkr/api`（`packages/hawkr-api-ts`，指令 `pnpm --filter @hawkr/api gen:api`），website 與 admin 經它 import，不各自生成。
- googleapis（`field_behavior`、`error_details`）的版本：`api/buf.yaml` 的 deps 鎖在 `api/buf.lock`，TS 套件與 Dart 生成 `error_details` 的 `buf.gen.yaml` 釘同一個 commit，API contract CI 檢查一致。Go 用 genproto 的 `annotations` 與 `errdetails`，不自己生成（`buf.gen.connect.yaml` 的 managed mode 排除 googleapis）。

## 十三、檔案回應

- 回應是檔案本身、不能把 bytes 包成 Connect 訊息的欄位（base64 膨脹，也進不了瀏覽器的圖片快取）時，不經 Connect：路由放在 `/api/files/<使用者類別>/…` 底下（`/api/files/` 不是舊 gateway 的 `/api/v1/`，不轉給 gateway），例如 `GET /api/files/admin/seller/kyc_applications/{application_id}/documents/{slot}/{position}`。使用者類別同第二節（`public`、`app`、`admin`）。
- 與 Connect 同一條入口中介鏈：tracing、CORS、CSRF、session（`Authorization: Bearer`）、限流（級別同該使用者類別的 Connect 方法，超過回 429 加 `Retry-After`）。授權由擁有這份檔案的 handler 做（先確認使用者類別，再做業務授權），不依賴路徑。錯誤只回 HTTP 狀態，不帶細節（沒有授權與不存在不可區分）。
- 每條 `/api/files/admin/…` 端點宣告一個權限（`platform/adminperm` 目錄裡的 `<模組>.<資源>.<動詞>`，例如 `seller.kyc_documents.get`），宣告在 `internal/httpapi` 的 `fileRoute.permission`：入口在 handler 之前檢查呼叫者是管理員而且角色允許它（`adminauth.GuardFile`），模組自己的授權（例如 KYC 的限時 reveal）在那之後。啟動時檢查每條 `/api/files/admin/` 路由都宣告了目錄裡的權限、其他路由沒有宣告，不合法就不能啟動；新增端點沒寫宣告等於不能啟動。與 admin Connect 方法的宣告同一套目錄與規則（ADR `2026-10-04-admin-roles`）。
- 機敏檔案回應 `Cache-Control: private, no-store`，加 `X-Content-Type-Options: nosniff`；`Content-Type` 只放行確定安全的型別，其餘一律 `application/octet-stream`。
- 以業務資源定址（申請、位置），不以 asset ID：asset ID 只在擁有者的資料表裡，不出現在 URL 與回應。每次讀取是否留稽核由擁有者決定；要稽核的先寫紀錄再送內容（寫不進去就不送）。
- 取得網址的方式是擁有者的 Connect 方法回傳（例如 KYC 的 `RevealKYCSensitive` 回 `view_uri`），不由 client 自己組。

## 與〈API 設計規範〉的關係

- 本規範取代〈API 設計規範〉的：§3 namespace（改成第二節的 service）、§7–§11 路徑與方法（第三、四、六節）、§12 envelope（第八節）、§14 錯誤轉換（第八節）、§15 版本（第一節）、§16 Swagger（proto 的註解）、§17 AI 條款、§18 快取（第九節）。這些條文只適用舊 gateway 的 REST，模組搬進單體時隨它的路由退場。
- §3.1 的 deny-by-default 與 §14.1 的防枚舉，原則已寫進本規範（第五節、第二節），不再引用該規範。
- §2.4 gateway 的職責邊界、§4 的角色模型（gateway 的 JWT）由第二節取代（identity 搬進單體，遷移計畫 A2）；只在還沒搬的 REST 經 gateway 時適用。
- 〈多媒體與圖片 URL 規範〉：已依 ADR `2026-09-25-api-image-uri-template.md` 改寫 §2、§4（組裝點是單體的傳輸層，對外回 `Image.uri_template`）。
- `backend/AGENTS.md`「型別、金額與時間」：Connect 的數字依第四節（`int64` 是字串、`int32` 是數字），舊 gateway 的 REST 維持字串。
