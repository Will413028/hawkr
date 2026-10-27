# A3b 步驟 10c 開工的決定題

HEAD `6f159d2ff`。三份唯讀盤點（seller-app client、gateway 與 legacy、Dart 執行期）加上 website 10b 的對照只當線索，未入庫。起草者沒有參與盤點；引用的結論都先對過 SSOT：計畫 `2026-09-29-a3-buyer-seller.md:145-232`、`step10-decision-brief.md`、`step10b-decision-brief.md`、`seller.proto`、`identity.proto`、seller `AGENTS.md`／`CARRYOVER.md`、ADR `2026-10-02`（含 10b 補充）、`2026-10-03`、`2026-10-04`、`2026-09-27-identity-clerk`，以及原始碼。

## 結論

- 要 Will 決定的有 **3 項**：
  - D1：seller-app 照 website 10b 已核准的語意與文案改畫面（把盤點列的 16 項 UI 中真正要同意的 10 項收成一題）。
  - D2：賣家入駐的同意與告知：哪幾份文件、在開店與送出 KYC 的哪一步、伺服器記什麼。建議改成三份文件分開記錄（不只是 KYC 一個勾選框），而且隱私權政策要補上 KYC 資料。
  - D3：website 有、App 沒有的兩個功能。申訴佐證在 App 補上。PayUni 聯絡資訊經查是沿用舊設計的重複資料：建議把它併進店家資料、由同步 job 送給 PayUni，刪除 `UpdateMyPayUniProfile`，兩端的差異也就不存在了。
  - D2、D3 的根治都要改契約與後端，第 0 刀因此拆成 0a、0b（文末切分）。
- 知會 **K1–K24**，照 SSOT 或已核准的決定直接做。
- 範圍有兩處要修正：
  - 計畫的「刪除 `/app/sellers/*`」照字面不成立。這個前綴下有 67 個操作，只有 11 個屬於 seller 模組；連同 `/app/certifications*` 5 個、`/app/businessRegistration` 1 個，10c 一共刪 **17 個**，`maxRESTOperations` 從 525 降到 **508**。另外 56 個屬於 crm、post、forum、review、notification 等模組，還有消費者，留著。
  - buyer-app 的「刪除帳號」照 D4 放在 10c，盤點漏了（K21）。
- 切分成 7 個 commit（文末）。

---

## 照 SSOT 直接做、不問（知會）

不同意哪一項就說。

| # | 項目 | 做法 | 來源 |
|---|---|---|---|
| K1 | 店家 ID 的型別（G1） | `SessionModel.sellerId int?` 改成 `storeId String?`，client 不解析、不轉數字。舊 REST 模組（chat、post 等）的 payload 還是整數，只在那些 REST 邊界用 `toString()` 比對，等各模組搬進單體時移除。`rg -n "\.sellerId\b" lib` 不含生成檔，共 85 處 | `seller.proto:7-8`；ADR `2026-09-25-row-identity-by-data-kind` D5 |
| K2 | 圖片（G2） | 讀 `Image.uri_template` 加 `ImagePreset`（`packages/hawkr_api/lib/images.dart`）；以 objectKey 為鍵的快取與 `buildImageUrl` 刪除；換 logo、封面改用 `UpdateMyStore` 的 `*_staged_token` | ADR `2026-09-25-api-image-uri-template`；`seller.proto:247-257` |
| K3 | etag 寫入與版本衝突（G4、G5） | 所有帶 etag 的寫入（`UpdateMyStore`、`CompleteMyStoreOnboarding`、`SaveKYCDraft`、`SubmitKYC`、`CancelKYC`、`SubmitAppeal`）遇到 `VERSION_CONFLICT` 時，重讀最新版本、換上新的 etag，保留使用者的輸入。TS 的 `connect/submit` 把錯誤分成 conflict、fields、message 三類，Dart 版放進 `packages/hawkr_api`，update_mask 與 `google.type.Date` 的 helper 也放進去（buyer-app 現在手寫） | 10b K6；API 規範（Connect）；10a T1「Dart 套件與 TS 同形」 |
| K4 | 自己店家的統計 | 在 10c 搬：`profile_screen.dart:424`、`seller_page_stats_remote_service.dart:17`、`page_header.dart:800`、`product_preview_screen.dart:398` 改讀 `GetMyStoreResponse.stats`。這些都是自己的店：`PageScreen` 只在沒有 `sellerData` 時建立（`app_router.dart:458`），所以 `_isOwnPage` 永遠成立；`page_screen.dart:281` 的 `getSeller(id)` 是死碼，刪掉。gateway 的 `/public/sellers/*` 留到 10d（website、buyer-app 還在用） | B2；ADR `2026-10-03`:36「賣家預覽自己的店讀 `GetMyStore`」 |
| K5 | 認證狀態的來源 | 是否已認證看 `exposure.missing_requirements` 有沒有 `KYC_APPROVED`；細分狀態看 `GetMyKYC.flow.status`，not_found 就是 `NOT_STARTED`。舊字串（`submitted`、`forwarded` 等）的對照刪除；`closed` 改看 `store.status` | 10b K1；`step10-decision-brief.md:224` |
| K6 | B9 與 D3 的閘（物流閘的推導由 3-0 取代，見計畫 10c「賣家功能的前置條件由伺服器判斷」） | 拿掉建立商品的 KYC 閘（`dashboard_screen.dart:58-59,117-133`、`page_shop_tab.dart:154-160`）。物流閘改由 `flow.status == APPROVED` 推導（`seller_certification_gate.dart`）。優惠券的 KYC 閘拿掉：今天是錢包整頁只在核准後才顯示，所以閘是隱含的（`wallet_screen.dart:1938-1939`）；優惠券要放在哪裡見 D1。D3 的文字沒有限定 website | B9；D3 |
| K7 | KYC 單例 | 刪掉 `cert_draft_id_$sellerId`（`auth_storage.dart:248-261`）。建立就是不帶 etag 的 `SaveKYCDraft`，讀取用 `GetMyKYC` | `seller.proto:619-620`「每個店家同時最多一筆」 |
| K8 | 續填與補件的位置 | 續填的位置由 `missing_fields` 推導；退回要補的欄位看 `last_rejection.required_fields`，PayUni 問題要補的看 `problem.fields`。`currentStep`、`stepData`、路由的 `initialStep`、`retryErrorMessage` 刪除。`_fieldLabel` 改以 `KYCField` 為鍵 | `CARRYOVER.md:249,255`；`seller.proto:450-453` |
| K9 | 組織型態（D1） | 刪下拉選單與 `mapBusinessTypeToOption`。統編查詢依 `postal_address.town_code` 帶入，不顯示 `organization_type` | D1；KYC ADR 10b 補充 |
| K10 | 送出（E4） | 直接呼叫 `SubmitKYC(etag)`，包含 `flow.can_resubmit` 為真的申請；不再「先存檔再 PATCH」 | E4 |
| K11 | 聯絡 email 與手機 | 個人和企業都必填 `contact_email`、`contact_phone`（09 開頭）；`companyEmail` 刪除。個人流程照契約加這兩個欄位 | 10b K7；`CARRYOVER.md:108-109` |
| K12 | 個人賣家的文件 | 刪掉個人流程的 6 格上傳。契約只有企業有文件（`business.company_documents`），8b 刻意這樣切，website 10b 也只對企業顯示。如果要留，就是改契約 | `CARRYOVER.md:256`；`seller.proto:613-614` |
| K13 | 地址 | 加上 `hawkr_twaddr` 依賴，寫入改用 `PostalAddress{town_code, address_detail}`。TGOS 驗證刪除，手抄表 `taiwan_address_data.dart` 刪除。物流的 `taiwan_cities_data.dart` 不動：黑貓的契約仍收名稱，不屬於 seller 模組。物流表單的郵遞區號改由 `postalCodeByTownCode` 取得 | ADR `2026-10-04`；`CARRYOVER.md:108`（決定第 9 項） |
| K14 | 「發布動態」前置條件 | `post_or_website_dialog.dart` 刪除，送出 409「請先發布一則動態」的分支一起刪 | `CARRYOVER.md:131`；10b K5 |
| K15 | REST 錯誤碼 | 刪掉 9233 與裸 404 的判斷（`session_provider.dart:27-72`）、`sellerProfileExists`、`friendlyDeleteAccountError`、9204／9205 的註解、KYC 的 428／409 字串比對，以及沒用到的 9241。**9240 保留**：10c 之後 `sellerHandle` 還有其他模組的呼叫點（盤點 backend §7）。`onSellerSuspendedSignal` 改成讀 `GetMyStore`。過時的註解（`seller_suspension_codes.dart:6`、`connect_provider.dart:21`、`seller_certification_upload_screen.dart:84-86`）一併修 | B1；計畫:224「給 10c」 |
| K16 | 刪帳號的順序 bug（G6） | 已驗證：`profile_other_options_card.dart:282-298` 先撤銷 Apple 授權再刪除。B14 之後，還有店的帳號會先被撤銷授權，再被 `ACCOUNT_HAS_OPEN_STORE` 拒絕刪除。修法：先讀 `GetMyStore`；店還開著就引導去關店，不跳 Apple。之後才撤銷並呼叫 `DeleteAccount`，處理 `DELETED`、`PENDING_ORDERS`；沒收到回應時以 `GetSession` 確認。補一個先紅後綠的回歸測試。App 原生就能撤銷，不需要 E1 給 web 的提示 | `identity.proto:28-32`；B14 |
| K17 | 錢包的死碼 | `wallet_screen.dart:153` 在最上層讀 `paymentStatus`，但值包在 `data` 裡（`seller_repository.dart:423-435`），永遠是 null，所以 `:1966-1983` 的分支是死碼，刪除。B13 的成交餘額卡（`:1528-1566`）連同它對儀表板的呼叫一起刪掉，「前往提領」保留 | B13；原始碼 |
| K18 | 契約沒有的帳號欄位 | 店家和帳號都沒有 email，identity 也沒有電話。所以：個人檔案的 email 列刪除（今天的編輯本來就無效：gateway 的白名單丟掉 email，`handler/seller.go:473-484`）；`sellerName`、`birthday`、`phone` 的快取刪除；錢包「請查收信箱」改讀 `GetMyPayUni.mer_email`，還沒綁定 PayUni 時讀 KYC 的 `contact_email` | `CARRYOVER.md:20`；`identity.proto:49-52` |
| K19 | PayUni 狀態、儀表板的資料、違規 | PayUni：`GetMyPayUni` 加 `flow`；MerID 不放進 session；推播的整數只當觸發訊號。儀表板：區塊不設時顯示「—」，不用 `_toInt` 補 0。違規：分頁讀完（每頁 100），`SubmitAppeal` 帶 etag，用 `statement` 欄位；類別、嚴重度、處分的文字放在 client 的對照表 | `seller.proto:359-361`；10b K11、K14 |
| K20 | 停權時能進哪些頁 | `redirect_logic.dart:59-80` 寫死的白名單改成由 `capabilities` 推導；有 `ACCOUNT_CLOSE` 時可以關店。B15 依 `Suspension.source／reason` 顯示，用 website 已核准的文案 | `seller.proto:170-171`；10b K13；B15 |
| K21 | buyer-app 的刪除帳號 | 在 10c 做。buyer-app 能用 Apple 登入，卻沒有刪除帳號的入口（`rg -n "刪除帳號\|deleteAccount\|revokeApple" buyer-app/lib` 沒有命中），不符合 App Store 5.1.1(v)。照抄 seller-app 修好之後的流程（K16），文案與 website 相同；入口放哪裡見 D1 | B14；D4「buyer-app 的刪除帳號在 10c」 |
| K22 | gateway 的刪除範圍 | 刪 17 個操作，525 → 508（盤點 backend §2；`routes_seller.go:47-53,72,84`、`routes_admin.go:53-57`、`routes_governance.go:75-76`、`routes_business_registration.go`）；其他模組的 56 個留著 | B20 |
| K23 | 只為儀表板存在的 legacy RPC | 刪 crm 的 `GetSellerStats`、`GetSellerDashboardGrowth`，以及 order 的 `GetSellerDashboardGrowth`，連同 Java 與 Go 的實作鏈和兩份 proto（先例 `477520ae2`），numeric allowlist 同步更新。gateway 對 `GetProductViewStats`、`GetSubscriberDashboardStats`、`GetSellerOrderStatusCounts` 的包裝刪除，但這三個的 server 留著：前一個到步驟 11，後兩個還有 seller-service 或單體在呼叫 | 計畫:145 |
| K24 | 盤點 CSV | 只刪路由的 commit 不動 CSV（10b `7a2e9cac0` 的先例）。刪 RPC 的 commit 照 8e、8i 的先例（`d1a6986a5`、`232711bd4`）：`rpc-servers.csv` 的 :233、:234、:420 標 `removed: …`；:144 拿掉「與 gateway」；:595 `GetProductViewStats` 不動，等步驟 11；`rpc-calls.csv` 刪掉 gateway 那幾列（:235、:300、:301、:419、:421、:576），順便刪 10b 漏掉的 :584、:594（那兩個 gateway 呼叫已經不存在，`rg GetStoreVisitors\|RefreshPayuniStatus gateway/internal` 沒有命中）。被刪的 handler 是最後呼叫者的那些 seller-service RPC，留到步驟 11 隨 seller-service 一起處理 | 計畫:145「盤點 CSV 的去向同步更新」 |

---

## 要 Will 決定

### D1. seller-app 照 website 10b 的語意與文案

**現況**
- 計畫 10c 列寫「同 10b」。可是 D2、E2、E3 與 website 的文案都是針對 website 寫的，而根 `AGENTS.md` 規定版面、文案與流程的改動要先取得同意。
- 盤點 §5 列了 16 項需要同意的 UI 改動，其中 6 項其實已由 SSOT 或已核准的決定涵蓋（見附錄），剩下的收成下面 10 點：
  1. **KYC 狀態與 PayUni 的文字**：依 `KYCFlowStatus` 用一張對照表，決定狀態標籤、鎖定樣式與動作，取代 `_resolveLockedStatusStyle`、`payuniLockedNoticeFor`。鎖定改看 `flow.identity_locked`：店名與介紹在拿到 MerID 之後才鎖，不是送出就鎖。PayUni 問題依 `code` 加 `remedy` 呈現，取代伺服器回傳的 `errorDisplayMessage`（G7）。文字一律用 website `kyc-flow.ts`、`payuni.ts` 已核准的版本。
  2. **wizard 的內容**：
     - 店家地址留在 wizard 裡，送出時寫進 `UpdateMyStore`。理由是 PayUni 取店家地址（10b K8），website 也這樣做。
     - 商店介紹移出 wizard，改在商店資料頁編輯。
     - 證號與銀行帳號在續填與確認時顯示 `*_masked`，使用者重新輸入才送出。
  3. **進度卡**：B8 的清單放在「認證與收款」卡的位置。店面頁的 readiness card（`page_shop_tab.dart:295-330`）照 D2 分成「公開前必須」與「建議完成」；「您尚未實名認證」那個商品空狀態刪除（B9 之後不成立）。
  4. **優惠券**：錢包不論 KYC 狀態都顯示優惠券區塊，由 `MARKETING` 能力決定。
  5. **開店頁**：拿掉「沿用登入時完成驗證的聯絡資料」那一列手機（identity 沒有電話）。聯絡電話改在商店資料頁填，website 開店也不收。
  6. **關店與刪帳號**：設定頁分成「關閉商店」與「刪除帳號」兩個入口，文案沿用 website 的 `CloseStoreSection`、`DeleteAccountSection`（含有店時「請先關閉商店」）。buyer-app 的入口放在個人資料編輯頁的最下方，同一套文案。
  7. **等待刪除**：沿用 `suspension_notice_screen` 的版型，用 E2 的文案「帳號刪除處理中，等待未結訂單完成；期間只能處理訂單與售後」。可以進哪些頁由能力推導（K20）。
  8. **儀表板**：用 B12 的文案。App 有、website 沒有的三張卡改名：「訂單」→「近 7 日訂單」、「客戶」→「近 7 日顧客」、「訂閱」→「新追蹤者」。它們的數字來源已經變了（`sales.orders`、`sales.customers`，`followers` 的新增數），舊的「客戶」是 CRM 的累計，不再成立。每張卡下方都顯示期間，說明改成「關鍵數據與較前 7 日變化」。
  9. **已認證徽章**：自己的店面預覽拿掉「已認證」（`PageHeader.isVerified`，`profile_screen.dart:1037`、`page_screen.dart:178`），和 B19 之後的買家店面一致。認證狀態改看第 3 點的認證卡。
  10. **版本衝突**：提示用 website 的 `versionConflictNotice` 句型。

**業界基準**
- Shopify admin 的 web 與行動 App 用同一套狀態、同一張 Setup guide、同樣的文字，只有排版依螢幕調整。
- Stripe Dashboard 的 web 與 iOS 也用同一份 `requirements` 狀態。
- 原則是一份語意、各平台各自排版。

**既有機制為什麼存在**：公司時期 App 與 website 由不同的人分頭做，各自解讀 REST 的狀態字串。wizard 的切法、鎖定時點、徽章都是在那時候各自定的，不是產品上刻意要不同。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 10 點照上面做：語意與文字和 website 相同，版面用 App 既有的元件與 token | 兩端只有一套說法；Will 一次同意；App 的排版仍是 App 的 |
| (b) 每點各自設計 App 版 | 可以針對手機調整；10 次同意，兩端的文字會再分歧 |
| (c) 只換資料來源，畫面盡量不動：介紹留在 wizard、優惠券仍在核准後才出現、徽章保留 | 改動最少；但會留下與契約、D3、B19 相反的行為，例如優惠券的閘和 D3 衝突 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| UI 改動要 Will 同意 | REAL（repo 規則） | 根 `AGENTS.md`「UI 改動」 |
| 契約沒有商店介紹、帳號電話、徽章資料的 KYC 欄位 | REAL（契約） | `seller.proto:628-668`、`identity.proto:49-52` |
| 手機版面與 web 不同 | REAL（只影響排版，不影響語意） | — |
| 保留 App 現有的 wizard 與卡片結構 | SELF-IMPOSED（一致性；上線前不受相容性約束） | 根 `AGENTS.md`「上線前不受相容性約束」 |

**建議 (a)**。
**終點與觸發**：文字對照表在 TS 與 Dart 各一份。等到第三次需要同步文案，或兩端的文案出現不一致的 bug，再考慮從一份來源生成。

### D2. 賣家入駐的同意與告知

**現況**
- App 的 wizard 第 2 步有「同意身分驗證條款」的勾選框（`seller_certification_upload_screen.dart:360,2450,2983`），條款是寫死的 4 句話（`:1680`），結果只存在已刪除的 `stepData`（K8），伺服器沒有任何紀錄。
- website 的開店與 KYC 都沒有勾選框，只有登入頁的「繼續代表你同意服務條款與隱私政策」（`LoginPage.tsx:350`）。這句是帳號層級的，也沒有記錄。
- 現行沒有賣家契約。公司時期有一份〈賣家合作暨平台服務契約〉v1.1（私人封存的 `website/docs/legal/hawkr-seller-platform-service-agreement-template.md`，在 `6c82ba1ed` 隨公司時期的業務工具一起刪掉）。它的生效條件是「完成簽署或完成平台電子同意之日」，第 23 條第 2 項要平台「保存契約版本、完整內容、簽署帳號、同意時間、驗證方式、來源紀錄」。現行的 `TermsPage`、`UserPolicyPage` 有一部分賣家義務，但沒有涵蓋開店契約該有的內容。
- 隱私權政策（`PrivacyPage.tsx:70-71,104-108`）列出的賣家資料只有「姓名、聯絡資訊、食品業者登錄字號、撥款銀行帳號」，**沒有身分證字號、證件影像與公司登記文件**；提供給 PAYUNi 的資料也只寫「付款、退款及金流對帳所必要之交易資訊」，**沒寫 KYC 的身分、聯絡與銀行資料會送給 PAYUNi 做子商店註冊**（`bind_merchant` 的 `Name`、`UBN`＝身分證號或統編、`Email`，代理商文件 §7.3）。
- PayUni 的代理商 API 文件沒有提到子商店要同意 PayUni 的條款。不過 `bind_merchant` 會替賣家**自動建立 PayUni 會員**（§7.1），而規範雙方的〈代理商合作業務契約〉（§1.6）不在 repo 裡，公司結束後也沒有現行的版本。
- repo 裡已經有伺服器記錄同意的先例：舊 invoice 的自行開票條款（`backend/api/proto/invoice/invoice.proto:709-727`）。伺服器提供 `current_terms_version`，client 回傳簽署的版本，伺服器記下 `terms_version`、`accepted_at`、`accepted_by_account_id`。同一份檔案裡，「只有告知、沒有法源」的結帳發票告知已經改成只顯示、不勾選（`:296-307`，DEC-20260831-210934）：告知和同意在這裡就分開處理了。

**業界基準（從零設計）**：入駐有三種不同的文件，各在自己的時點處理。
1. **平台的賣家契約**：在建立賣家身分的時候 clickwrap。Shopify 在建立商店時同意 Terms of Service；Amazon 在註冊 Seller Central 時同意 Business Solutions Agreement；Uber Eats 與 foodpanda 在商家線上簽約時同意商家條款；蝦皮在開通賣家時同意賣家條款。
2. **金流商的子商戶條款**：在啟用收款、送出 KYC 的時候同意。Stripe 要求平台向 connected account 出示 Stripe Connected Account Agreement、取得明示同意，並把同意記錄到 `tos_acceptance`（`date`、`ip`，另有 `user_agent`、`service_agreement`）。帳號換手或稅籍號碼變更時要重新同意（docs.stripe.com/connect/updating-service-agreements）。Stripe 也要求平台的隱私權政策載明 Stripe 會處理這些資料。Shopify Payments 在啟用時另外同意 Shopify Payments 條款。
3. **個資的告知**：在蒐集的當下告知，這不是同意。依個資法第 8 條，直接向當事人蒐集時要告知機關名稱、目的、類別、期間、地區、對象、方式、當事人權利，以及不提供的影響。KYC 的蒐集依據是第 19 條第 1 項第 2 款（契約或類似契約關係）與第三方支付的身分確認義務，不需要另外取得同意；身分證號也不是第 6 條的特種個資。業界的做法是在蒐集畫面放「我已閱讀個資告知事項」，並記下顯示的是哪一個版本，作為已經告知的證據。

這三種紀錄都要**存在伺服器上，以版本識別**；版本改了就要重新同意（Stripe 與公司時期契約的第 22 條都這樣規定）。勾選框本身不是證據。紀錄放在需要它當前提的那個 use case 的擁有者，和寫入在同一個 transaction 裡。

**現有機制為什麼存在**：App 的勾選框是公司時期的 wizard 自己加的，沒有對應任何正式條款。前一版的建議 (a) 只在 `SubmitKYC` 記一份版本，把三種文件混成一個，也漏掉了開店那一步。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 前一版：`SubmitKYC` 加必填的條款版本 | 有證據，但三份文件混成一個版本號：賣家契約、PayUni 條款與個資告知各自改版的時候，分不出是哪一份需要重新同意；開店時也沒有同意賣家契約 |
| (b) **分文件記錄**：開店時記賣家契約；送出 KYC 時記個資告知與 PayUni 條款。伺服器記錄文件種類、版本、時間、帳號、IP、User-Agent 與 client，版本以伺服器公布的為準 | 與 Stripe、Shopify 相同。要改契約、加一張表，兩端都要加勾選與連結；需要現行的賣家契約與 PayUni 條款的文字 |
| (c) 只在 client 擋 | 沒有證據力 |
| (d) 拿掉勾選框，靠登入頁那句話 | 開店與送 KYC 都沒有記錄；登入頁那句也沒有記錄 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| 蒐集身分證號、證件與銀行帳號時要告知，告知的內容要完整 | REAL（法規）。現行隱私權政策漏了 KYC 資料與提供給 PAYUNi 的部分，這是**內容缺漏，不是缺一個勾選框** | 個資法第 8 條；`PrivacyPage.tsx:70-71,104-108` |
| KYC 的蒐集需不需要「同意」 | REAL（法規），結論是不需要（第 19 條第 1 項第 2 款）。要律師確認的只有一點：把 KYC 資料提供給 PAYUNi 是否在契約的必要範圍內 | 個資法第 19、20 條 |
| PayUni 要求子商店同意它的條款 | REAL 但**缺資料**：API 文件沒寫，代理商契約不在手上。`bind_merchant` 會自動建立會員，依 Stripe 的類比，很可能要同意 | 代理商文件 §1.6、§7.1 |
| 正式的賣家契約文字與營運主體 | REAL（缺資料；文字要律師審）。v1.1 範本在 git 歷史中，是公司時期的主體 | `394c818be` |
| App 寫死的 4 句條款 | 不是正式條款，刪除 | `:1680` |
| 契約、migration、兩端的工作量；10c 是「同 10b」 | SELF-IMPOSED | 根 `AGENTS.md`「上線前不受相容性約束」 |

**建議 (b)**。業界基準只針對 REAL 的缺資料調整：PayUni 條款與賣家契約的正式文字還沒有，機制先做好，文件用下面的安全預設。
- **契約（`seller.proto`）**：
  - `enum AgreementKind { SELLER_TERMS; KYC_PRIVACY_NOTICE; PAYUNI_TERMS; }`，以及 `message AgreementAcceptance { kind; version; }`。
  - `ApplyStoreRequest` 加必填的 `accepted_agreements`（`SELLER_TERMS`）。
  - `SubmitKYCRequest` 加必填的 `accepted_agreements`（`KYC_PRIVACY_NOTICE`，在 PayUni 條款確定前加上 `PAYUNI_TERMS`）。
  - 版本不由 client 寫死：新增 `GetSellerAgreements`（不必有店家，開店前也要讀），回傳每種文件的現行版本與連結路徑。
  - 版本和伺服器現行的不同時，回 `failed_precondition`，`ErrorReason` 是 `AGREEMENT_OUTDATED`；client 的處理是重讀、請賣家再勾一次。
- **後端**：
  - 新表 `seller.agreement_acceptances`，主鍵 `(store_id, kind, version)`，欄位有 `account_id`、`accepted_at`、`client_ip`（`httpapi.ClientIP`）、`user_agent`、`client`，只有 INSERT。和 `Apply`、`Submit` 在同一個 transaction 寫入。
  - 現行版本是 Go 的常數表（`model.CurrentAgreements`），版本是文件的生效日。
  - 保存：和審核的事實一樣，不在 30 天的個資清理範圍內，期限併入 A8 的資料保存政策。
  - 擁有者是 seller：這兩個 use case 都在 seller。
- **website**：
  - 開店與 KYC 送出前，加上勾選框和連結。
  - 新增 `/seller-terms` 頁面，`/privacy` 補上 KYC 那一段。
  - 法務頁的版本與 Go 常數表的一致性，加進既有的 `legal-policy-consistency` 測試。
- **seller-app**：開店與 wizard 的送出用同一套勾選與連結（開外部瀏覽器或 WebView），刪除寫死的條款。
- **律師要確認的三件事**：
  1. 賣家契約的正文與營運主體。安全預設：以 v1.1 範本改主體後當作草稿，正式上線前不得以範本當正式版本。
  2. KYC 的個資告知全文，以及提供給 PAYUNi 的範圍。安全預設：照第 8 條的六項逐項寫出，類別寫明身分證字號、證件影像、公司登記文件、銀行帳戶，對象寫明 PAYUNi（子商店註冊與審核），期間寫「關店後 30 天」（ADR 2026-10-02）。
  3. PayUni 是否要求子商店同意它的會員條款或合作商店條款，以及要用哪個版本。安全預設：保留 `PAYUNI_TERMS`，先連到 PAYUNi 公開的會員條款；簽代理商契約時依契約更正。
- 上面三份文字都屬於 UX 文案，要 Will 同意（根 `AGENTS.md`「UI 改動」）。

**終點與觸發**：
- 已經記錄的版本落後現行版本時，`Store` 帶 `pending_agreements`，並加上 `AcceptSellerAgreements`；未同意的重大變更擋下會受影響的能力。這部分的觸發是**上線前的檢查清單**，或上線後第一次重大改版。上線前沒有真的賣家，所以改版不需要重新同意。
- 帳號層級的服務條款（Clerk 的 legal consent 只記時間、不記版本）歸 identity，觸發同樣是上線前，不在 10c。
- invoice 的自行開票條款搬進單體時，若出現第三個消費者，再評估要不要把紀錄抽成共用模組。

### D3. website 有、App 沒有的兩個功能

**現況**
- 申訴：website 的 `SubmitAppeal` 可以附佐證照片（機敏上傳，10b K10），App 的 `seller_appeal_screen.dart` 只能寫文字。另外，website 寫死最多 3 張、500 字（`SellerAppealDialog.tsx:18-19`），契約是最多 5 張、2000 字（`seller.proto:875-878`）。client 比契約嚴，這個不一致不用另外決定。
- PayUni 聯絡資訊：website 以 `UpdateMyPayUniProfile` 編輯 `contact_name`、`contact_mobile`、`contact_tel`、`mer_email`、`mer_tel`（`MerchantProfileEditCard.tsx`），App 沒有。

**PayUni 聯絡資訊是不是重複的資料：是。** 下面每一項都已經在平台的資料裡：
- 核准時這 5 欄是從 KYC **複製**過來的（`payuni/intent.go:37`）：`ContactName` 來自 `real_name`，`ContactMobile` 來自 `contact_phone`，`ContactTel` 來自企業的 `company_phone`，`MerEmail` 來自 `contact_email`。KYC 的這兩欄本來就是為了 PayUni 才加的（ADR `2026-10-02`:87）。
- `MerTel`（商店電話）沒有初值，綁定時不送（`model/payuni.go:157`，歸類成「我方沒送」）。但店家已經有 `Store.contact_phone`。
- 地址在同一個模組裡已經是業界基準的做法：店家地址是唯一來源，地址改了由 `seller.payuni_sync` 送出（seller `AGENTS.md`:95）。聯絡資訊卻另外存了一份副本。
- 兩份副本已經造成矛盾：
  - PayUni 回聯絡類的錯誤時，`problem.fields` 指向 KYC 的 `contact_email`、`contact_phone`（`model/payuni.go:114,178`），但送給 PayUni 的其實是註冊列上的副本。
  - 重新核准時，副本「重設成這次申請的值」（CARRYOVER:113），會靜默蓋掉賣家用 `UpdateMyPayUniProfile` 改過的值。
  - 綁定時 `Email` 讀 KYC，`MerEmail` 讀副本（`payuni/sync.go:254`），同一個 email 有兩個來源。
- 為什麼會有這份副本：舊 seller-service 有 `UpdateMerchantProfile` 和 `payuni_merchant_profile_updates`。CARRYOVER:118 只重新評估了**機制**（冪等表、advisory lock 改成 etag 加 job），沒有問「這份資料該不該獨立存在」。這是 AGENTS「沿用也是決策」漏掉的地方，是沿用下來的設計缺陷。
- PayUni 特有的只有格式：`ContactName` 最多 20 字、電話是 `02-9888888#123` 的格式、`MerName` 要符合 BIG5（代理商文件 §7.3）。這些在驗證與同步時轉換就好，不需要另外存一份資料。

**業界基準**
- 資料的擁有：Stripe Custom 或 platform-controlled 的帳號裡，資料以平台為準，平台以 Accounts API 更新 connected account。`business_profile.support_email`、`support_phone` 是對客服的聯絡方式，`individual.email`、`phone` 是代表人的；改聯絡方式不會觸發重新審核，改身分或稅籍才會。Shopify Payments 用的是商店設定裡的聯絡 email 與地址，沒有另一個「金流聯絡資料」的編輯器。
- 申訴：Amazon 的 Account Health 與 eBay、蝦皮的違規申訴，都以理由加附件（照片、單據）為基本形式。
- 行動版與 web 的分工：Shopify 的 App 涵蓋商店日常與大多數設定，少數重度設定留在 admin。共通的原則是：**會擋住營運的事，以及有期限的補救（補件、申訴、處理金流問題），在賣家日常使用的每個 client 都要能完成**；長尾的設定才可以只放 web。申訴有期限（v1.1 第 17 條是 14 日），佐證通常是手機現拍的照片（商品、標示、單據）。所有店家都關閉的帳號照樣要能申訴（`seller.proto:21-24`），這種帳號可能只有 App。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 前一版：申訴佐證在 App 做，PayUni 聯絡資訊只在 web 編輯 | 保留重複資料與上面的三個矛盾；App 改不了送到 PayUni 的聯絡資訊，而聯絡類的同步錯誤是要補救的事 |
| (b) 兩個都在 App 做（照現有結構補一個 PayUni 編輯畫面） | 兩端一致，但把重複資料複製到第二個 client |
| (c) **根治**：申訴佐證在 App 做；聯絡資訊併進店家資料（唯一來源），由同步 job 送給 PayUni，刪除 `UpdateMyPayUniProfile` 與註冊列上的副本 | 與 Stripe、Shopify 和本模組的地址做法相同；不再有「金流聯絡資訊」這個獨立功能，App 在第 2 刀的店家設定裡就有了。要改契約、加 migration，website 已完成的編輯卡片要刪掉 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| `update_merchant` 只能改聯絡方式、地址與支付設定 | REAL（外部）。只限制能送什麼，不限制資料存在哪裡、在哪一端改 | 代理商文件 §8.1 |
| PayUni 的欄位長度與電話格式 | REAL（外部）。在店家欄位驗證（聯絡人最多 20 字）並在同步時轉換格式 | 代理商文件 §7.3 |
| 會員的 `Email` 綁定之後不能改 | REAL（外部；§8 的允許清單裡沒有它）。綁定時取店家的客服信箱，之後只改 `MerEmail` | 代理商文件 §7.3.1、§8.3 |
| 申訴的照片、字數上限 | REAL（契約：5 張、2000 字）；website 的 3 張、500 字是 SELF-IMPOSED | `seller.proto:875-878` |
| website 10b 已經做完 `MerchantProfileEditCard`；10c 的範圍；「少用的設定留 web」 | SELF-IMPOSED（上線前不受相容性約束；已經做完的不是理由） | 根 `AGENTS.md` |

**建議 (c)**。
- **契約（`seller.proto`）**：
  - `Store` 加 `contact_name`（最多 20 字）、`contact_mobile`（09 手機）、`support_email`。既有的 `contact_phone` 就是商店電話（對應 `MerTel`）。
  - 刪除 `KYCApplication.contact_email`、`contact_phone`，`KYCField` 13、14 改成 reserved。
  - 刪除 `UpdateMyPayUniProfile`，`PayUniRegistration` 的 9–14 號欄位改成 reserved。
  - `PayUniProblem` 加 `repeated StoreField store_fields`：聯絡類的錯誤碼改成指向店家欄位，`MerTel` 的錯誤碼從「聯絡客服」改成賣家可以自己修正。
- **後端**：
  - `stores` 加上三欄與 CHECK，`kyc_applications` 與 `payuni_registrations` 刪掉聯絡欄位。
  - `UpdateProfile` 改了聯絡欄位時，和改地址一樣登記意圖並排入 `seller.payuni_sync`。綁定與同步一律讀店家資料。
  - 送出 KYC 的前提加上店家的聯絡資料是否齊全（和地址一樣是店家欄位）。
  - 拿掉核准時的「重設聯絡欄位」。
  - 30 天的個資清理改清 `stores` 的這三欄。
  - `contactFields` 改成指向店家欄位。
  - 管理後台讀 KYC 聯絡欄位的地方改讀店家資料。要用 `rg -n "contactEmail|contact_email" admin/src` 逐一確認，包含 `merchants/[id]` 與 `seller-prospects`。
  - seller 的 `AGENTS.md`:95、CARRYOVER:109、113、118 改成現況，ADR `2026-10-02` 加一則修訂（聯絡資料的擁有者從 KYC 改成店家）。
- **website**：KYC 頁的聯絡欄位改成寫店家（同地址的做法：送出時寫進 `UpdateMyStore`），店家設定加上聯絡人，刪除 `MerchantProfileEditCard` 與 `lib/seller/payuni.ts` 的 update。申訴的上限改成契約的 5 張、2000 字。
- **seller-app**：第 2 刀的店家設定含聯絡欄位；第 3 刀的 wizard 聯絡步驟寫店家；第 4 刀的申訴重用 wizard 的機敏上傳，最多 5 張、2000 字。
- 店家設定多了聯絡人欄位，屬於版面改動，與 D1 一起同意。

**終點與觸發**：平台的店家資料是唯一來源，PayUni 只是同步的對象。換金流商，或接上第二家金流商時，只需要新增一個同步端，不必新增編輯器。

---

## 切分（B20：每刀在同一個 commit 刪掉最後一個消費者已經改完的路由，連同 openapi 與 `maxRESTOperations`）

| 刀 | 內容 | 刪除的路由 | `maxRESTOperations` |
|---|---|---|---|
| 0a | D3 (c)：聯絡資料併進店家資料，跨端一次完成：契約、seller migration、payuni 同步讀店家、刪除 `UpdateMyPayUniProfile`、admin 改讀店家、website 的 KYC 頁、店家設定、刪除編輯卡片、申訴上限改成 5 張與 2000 字，以及 ADR 修訂與 AGENTS、CARRYOVER | — | 525 |
| 0b | D2 (b)：同意紀錄：契約（`AgreementKind`、`GetSellerAgreements`、`ApplyStore` 與 `SubmitKYC` 的 `accepted_agreements`）、`seller.agreement_acceptances`、website 的勾選框、`/seller-terms`、`/privacy` 補 KYC 段落（文字要 Will 同意；正式版要律師確認） | — | 525 |
| 1 | 共用基礎：`packages/hawkr_api` 的 submit 分類器、update_mask 與 Date 的 helper（K3）；seller-app 加 twaddr、seller 的 Connect client、`storeId String`（K1） | — | 525 |
| 1b | TS 的 Date 與 update_mask helper 與 Dart 同形（10a T1；第 1 刀的 design-review 提出），website 與 admin 的消費端一起改；update_mask 的路徑檢查兩端都照 API 設計規範收窄 | — | 525 |
| 2 | 店家資料，分 2-1 到 2-4 各自 commit（第 2 刀開工的決定）：2-1 session 改走 `GetMyStore`（刪 JSON 序列化與 K18 的快取）、由能力推導的路由、停權與等待刪除公告（K20、D1 第 7 點、B15）；2-2 開店（賣家契約勾選，D2）、完成新手引導（刪 REST 的 `completeOnboarding`、`sellerProfileExists`、`isSellerNotFoundCode`）、`UpdateMyStore`（含 logo、封面、店家聯絡欄位 D3、`identity_locked`；`SellerAvatarHelper` 改吃 `Image`，不再由 session 同步靜態的 sellerId）、自己店的統計（K4）；2-3 關店與刪帳號（B14、K16；同時修 `login_flow_test`「冷啟動持續限流」用真實時間等 10.7 秒、只比第三次重試多 200ms 的偶發失敗，改用 fake async）；2-4 認證狀態改讀 `GetMyKYC`（K5、K6；與 2-2 店名鎖定讀的 `GetMyKYC` 合併成一個 provider）、KYC 狀態標籤表、進度卡（B8、D1 第 3 點；取代 App 自己的「商店資料完整度」卡）。店面、商品預覽、儀表板的 `GET /me` 讀取在這一刀改完；錢包、物流設定、`page_echo_tab` 的 self 分支隨第 3 刀 | `POST /app/sellers`；`DELETE /me`；`/me/logo`；`/me/cover`；`/me/onboarding`（5 個） | 520 |
| 2b | buyer-app 刪除帳號（K21） | — | 520 |
| 3-0 | 賣家功能的前置條件由伺服器判斷（計畫 10c「賣家功能的前置條件由伺服器判斷」）：能力帶狀態與缺的條件、`SHIPPING_SETUP`、`exposure` 收成 `PUBLIC_LISTING`；契約、單體、gateway、website、seller-app 一起改；K6 的物流閘作廢 | — | 520 |
| 3 | KYC 與 PayUni：wizard（聯絡資料寫店家，送出時勾選個資告知與 PayUni 條款，刪除寫死的條款；D2、D3）、統編查詢、`GetMyPayUni`、優惠券（D1 第 4 點）、錢包與物流設定的 `GET /me` 讀取（含 K17）、`page_echo_tab` 的 self 分支；wizard 的地址改用 2-2 的 twaddr 對話框，刪除 `profile_address_edit_dialog.dart`、TGOS 服務與 `taiwan_address_data.dart`；進度卡的「食品加工登錄字號」在核准後指向 App 的食品登錄補件入口（website `FoodRegSupplement` 的對應；2-4 在核准後不給按鈕）；`routes_kyc_payuni_cache_control_test.go` 與相關常數刪除；`check-legacy-endpoints.py --update` | `/app/certifications` 5 個；`/app/businessRegistration/:ban`；`/me/payuni/status`；`GET`、`PUT /me`（9 個） | 511 |
| 4 | 違規與申訴（D3：機敏上傳的佐證，最多 5 張）、儀表板（B12、D1 第 8 點）、B13 | `/me/violations`；`/me/violations/:id/appeal`；`/me/dashboard`（3 個） | 508 |
| 5 | 只為儀表板存在的 legacy RPC 鏈、proto（Go 與 Java）與生成碼、numeric allowlist、盤點 CSV（K23、K24） | — | 508 |

每一刀各自做 code-review 與 design-review。

---

## 附錄：盤點錯誤或過時的地方

- client §5 的 16 項裡，有 6 項不用另外同意：
  - 第 1 項（續填位置）：`CARRYOVER.md:249,255` 已經定了（K8）。
  - 第 4 項（個人文件）：`CARRYOVER.md:256` 刻意只給企業（K12）。
  - 第 5 項（聯絡 email 與手機）：10b K7 已經涵蓋，依據是 `CARRYOVER.md:108-109`（K11）。
  - 第 14 項（email）：`CARRYOVER.md:20`（K18）。
  - 第 16 項（分頁）：純粹跟隨契約（K19）。
  - 第 2 項只對一半：website 已經決定店家地址留在 KYC 頁、送出時寫進 `UpdateMyStore`（`CertificationsClient.tsx:674-689`），只有商店介紹要移出 wizard。
- client §4 把 E3 當成只屬於 website。E3 背後的原則（畫面可用的操作由能力推導）來自 `seller.proto:170-171` 與 10b K13，App 一樣適用（K20）。
- client 盤點漏了 buyer-app 的刪除帳號：D4 把它分在 10c（K21）。
- 計畫 10c 列的「刪除 `/app/sellers/*`」照字面不成立：73 個操作裡只有 17 個屬於 seller 模組（backend §1）。
- `.sellerId` 的引用數：盤點寫 87；`rg -n "\.sellerId\b" lib -g '!*.g.dart' -g '!*.freezed.dart'` 是 85。
- backend 盤點說 `rpc-calls.csv` 是快照（README:3、9）。但 8e、8i 的先例在刪除呼叫時都會刪對應的列，10b 漏刪了 :584、:594（K24）。
- 已經驗證成立的：G6 的順序 bug、錢包的 `paymentStatus` 死碼、`PageScreen` 一定是自己的店、17 個路由與 `maxRESTOperations = 525`、`redirect_logic.dart` 寫死的白名單、App 已經能原生撤銷 Apple 授權。
