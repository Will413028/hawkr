# 10f-5 權利人通報（R8）現況盤點

- 快照：dev `44dee13b`（2026-11-22）。唯讀盤點，沒有跑 build、測試、容器。
- 路徑都相對 repo 根目錄。「讀過」＝直接看到該行；「推論」另標。`rg` 一律在 repo 根目錄下。
- 需求出處：`backend/docs/plans/2026-09-29-a3-buyer-seller.md:441`（10f-5 一行）；`backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-reports-brief.md:319-347`（R8）。

---

## 1. seller 的通報／案件／措施模型

Schema：`backend/internal/modules/seller/db/schema.sql`

**reports**（`:210-258`）
- 一列＝一位檢舉人的一次送出。`reporter_buyer_id BIGINT NOT NULL`（`:217`），CHECK `> 0`（`:242`）。註解明寫「檢舉人一定是登入的買家…沒有匿名入口，也不存檢舉人的聯絡方式」（`:203-204`）。
- 對象：`store_id NOT NULL`（`:212`）；`product_id TEXT`、`order_no TEXT` 是選填的不透明參照，「不驗證存在」（`:209`、`:213-214`）。一筆通報只有一個 `product_id`（R8 要「一則可以列多個商品」，現有形狀放不下）。
- `category` 檢舉人可選 12 個值，含 `COUNTERFEIT`、`IP_INFRINGEMENT`（`:234-237`）。
- `outcome`：`ACTION_TAKEN`／`NO_VIOLATION`／`INSUFFICIENT`／`DUPLICATE`／`ABUSIVE`（`:238`）；`ACTION_TAKEN` 一定在案件裡（`:252`）。處理狀態讀取時推導、不存（`:207`）。
- 去重：同買家、同店、同類別、同台北日一筆，只比對沒有結果的（`:267-269`）。重送併成 `report_supplements`，每筆最多 10 則（`:273-285`）。
- 佐證 `report_evidence`：每次送出最多 5 張，機敏資產（`:288-305`）。
- 送出的模組規則：不能檢舉自己的店（`internal/governance/report.go:74`）；只能檢舉曾公開過的店（`backend/internal/httpapi/seller/reports.go:24` 註解）；額度 10 分鐘 5 次、1 小時 10 次，以帳號計（`backend/internal/httpapi/seller/reports.go:45-48`）。

**cases**（`:160-194`）
- 平台對「一家店 × 一個平台判定類別」的處理單位，同時最多一個進行中：`cases_one_open (store_id, category) WHERE closed_at IS NULL`（`:195`）。
- `category` 16 個值，含 `COUNTERFEIT`、`IP_INFRINGEMENT`（`:177-180`）；`severity` LOW/MEDIUM/HIGH/CRITICAL（`:181`）。
- 結案＝決定：`close_reason` `DECIDED`（`decision` = `ACTION_TAKEN`｜`NO_ACTION`）或 `NO_REPORTS`（`:184-189`）。結案是終態（註解 `:153`）。
- 案件沒有「對象商品」「權利人」「法定期限」這類欄位；案件以店為單位，不是以一則通知為單位。

**流程**：通報 → 分案 `Triage`（`internal/governance/triage.go:57`，掛到同店同類別的進行中案件或開新案件）或直接 `Resolve`（`resolve.go:30`）→ 案件 `Decide`（`case.go:52`）→ 措施由 `measures.source_case_id` 或認定的來源通報連回案件（schema 註解 `:603-604`）。`Move`（`move.go:19`）改判類別。

**violations（認定）**（`:393-423`）：只是事實，不帶處分；`category` 含 `IP_INFRINGEMENT`、`COUNTERFEIT`（`:410-413`）；狀態 `ENFORCED`→`REVOKED`（`:415`）。

**measures（措施）**（`:605-681`）
- `kind IN ('WARNING','SUSPENSION','TERMINATION','CONTENT_TAKEDOWN')`（`:634`）；proto 同四種（`api/hawkr/seller/v1alpha1/seller.proto:979-989`）。
- WARNING：沒有限制（註解 `:586`）。SUSPENSION／TERMINATION：效果以**帳號**為單位（`:586-587`），不是商品。
- `CONTENT_TAKEDOWN`：對象在 `measure_contents`，`content_kind IN ('POST','COMMENT','FORUM_ENTRY','CHAT_MESSAGE','REVIEW_REPLY')`（`:737`）——**沒有 PRODUCT**。立即生效、沒有結束時間、解除只走申覆（`:664-665`）。一則內容同時只有一個生效中的下架（`:756`）。
- `risk_ground` 有 `INFRINGEMENT`（`:653`）：RISK 依據的停權可以用「侵權風險」當理由，一定有複審期限（`:592`、`:658-661`）。
- 賣家的救濟：`measure_requests` `APPEAL`（每個措施至多一次）與 `REMEDIATION`（只限 RISK）（`:822-854`）。
- `authority_orders.kind` 有 `RESTRICT_PRODUCT`（`:473`），但那是主管機關來函的登記，不是措施。

**結論（讀過）**：今天的措施對象只有「帳號／店」與「社群內容」，沒有商品層級；通報人只能是登入買家；一筆通報最多指一個商品。

---

## 2. 商品層級的下架

**有一條，但不是治理措施**：
- gateway `PUT /api/v1/admin/products/:id/status`（`backend/gateway/internal/handler/admin_product.go:151-190`；路由 `backend/gateway/internal/router/routes_product.go:105`；OpenAPI `backend/api/openapi/openapi.yaml:7449-7467`）。請求只有 `status` 一欄（`admin_product.go:17`，4003=下架、4004=上架）。
- Java `ProductService.updateProductStatus`（`backend/services/java/product-service/src/main/java/io/github/will413028/hawkr/product/service/ProductService.java:1422` 起）：不檢查擁有者，只改狀態。
- admin UI：`admin/src/app/admin/merchants/[id]/ProductForceUnpublishDialog.tsx`。檔內註解：沒有理由欄位（`:11-15`）；畫面文案「賣家仍可自行重新上架，要永久阻止販售請改用停權」（`:60`）。
- 所以：沒有理由、沒有紀錄、不能申覆、擋不住賣家重新上架。賣家自己的上下架在 `updateSellerProductStatus`（同檔 `:1579`）。

**停權連帶下架（帳號層級）**：Java `product_suspension_snapshot`（`backend/services/java/product-service/src/main/resources/db/migration/V10__add_product_suspension_snapshot.sql:1-10`）記停權時下架的商品，解除時還原；seller 的 `suspension_periods` 是它的 scope（schema 註解 `:892-895`）。

**計畫的位置**：「商品限制措施」排在 B4 catalog，狀態未開始。
- `backend/docs/plans/2026-09-24-go-modular-monolith.md:303` `- [ ] **B4 catalog**`
- 同檔 `:309`：「商品限制措施（…R6、…D7，Will 2026-10-18）：seller 的措施對象加商品（`measure_contents.content_kind` 加 PRODUCT），有理由、可以申覆、擋住重新上架；admin 的強制下架對話框改成建立這個措施；…第一次部署前一定要有。」
- `step10e-reports-brief.md:280-283`（R6 排進 B4）；`step10e-measure-brief.md:185`：「REAL：執行者在 Java product，B4 前不在邊界內，現在包一層措施是拋棄式工作。**現在只記錄牴觸**」。
- `step10e-3cb-design.md:22`：「來函的『限制或移除商品』在 B4 之前只能人工經 legacy 強制下架」。

**與 R8 的張力**：著作權法定流程要「移除 → 轉送賣家 → 回復通知 → 恢復」，對象是商品；今天能做的只有上面那支沒有紀錄的 REST。

---

## 3. 不必登入的寫入端點、限流、人機驗證、email 驗證

**現有 `*PublicService` 的寫入 RPC（全部）**：
- `SellerPublicService.RecordProductView`（`api/hawkr/seller/v1alpha1/seller.proto:238`）
- `SocialPublicService.RecordPostView`（`api/hawkr/social/v1alpha1/social.proto:77`）
- `IdentityPublicService.IssueAppLoginNonce`、`ExchangeIdToken`（`api/hawkr/identity/v1alpha1/identity.proto:26`、`:32`）
- 其餘 Public RPC 都是讀取（Content `:16`、`:22`；Discovery `:19`；Social `:24-69`；Seller `:208-231`）。
- 沒有任何收「使用者自由文字＋聯絡方式」的公開寫入。

**限流的宣告方式**：`backend/internal/httpapi/ratelimit.go`
- `procedureTiers`（`:47-52`）：procedure → `rateTier`，目前四筆（上面四支）；其餘依 service 的使用者類別，公開的是 `tierPublic`（`:54-65`）。
- 級別只有一個固定窗 `rateWindow = time.Minute`；每個 instance 各自計數，是粗略防護，全站限流打算放 CDN（檔頭註解 `:18-20`）。
- 10f-2a 拿掉了「一支 procedure 多個時間窗」的能力（見下）。

**CAPTCHA／Turnstile**：程式裡沒有。
- `rg -n -i -l "turnstile|captcha" -g '!node_modules' -g '!*.md' -g '!pnpm-lock.yaml' -g '!*.lock' .` → 只有 `backend/go.work.sum`。
- 決策紀錄：`docs/adr/2026-09-18-seller-governance-schema-and-enforcement.md:70-73`「決策六：公開食安端點用雙層 IP rate limit，不上 captcha」，理由是食安是緊急管道。那個理由是針對食安的，沒有對權利人通報做過決定。

**email 所有權驗證（一次性代碼／連結）**：沒有。
- `rg -n -i "email.?verif|verif.*email|EmailVerified|email_verified" backend/internal/modules/identity` → 只有 provider 回報的 `email_verified`（`backend/internal/modules/identity/db/schema.sql:51-60`）與 `ContactEmailVerified`（`identity.go:99-100`），都是 Google／LINE 登入帶回來的，不是平台自己寄信驗證。
- 計畫中的唯一 email OTP 是 vendor portal，屬於 A8（`docs/adr/2026-10-05-notification-module.md:56`）。

**commit b6ebd5a8（10f-2a）刪了什麼**（`git -C <repo> show --stat b6ebd5a8`：58 檔，+995／−3120）
- commit 訊息：刪 `SellerReportPublicService/SubmitFoodSafetyReport`、它的限流與 viewer 分支、聯絡方式清除 job、365 天上限；`reporter_buyer_id` 改 NOT NULL；刪 `phone/email/name/contact_erased_at/discarded_submissions` 欄位、`report_supplements.matched_by`、匿名去重索引；migration 先刪既有匿名列。
- 檔案：`seller/internal/governance/jobs.go`（−90，清除 job）、`report_email_test.go`（−39）、migration `20261121014310_seller_reports_buyer_only.sql`（+21）、`ratelimit.go`（刪 `tierReportSubmit` 與 `rateTier.window`／`span()`，即多時間窗）。
- 明文保留給 10f-5 的話：`docs/adr/2026-10-18-seller-governance-consolidated.md:199`「10f-5 的權利人通報不預先保留這些欄位（它的儲存在 10f-5 決定簡報裡定）」。

---

## 4. 對外寄信

**單體今天寄不出任何 email**。
- 沒有 notification 模組：`ls backend/internal/modules` 無 `notification`；A8 未開始（`2026-09-24-go-modular-monolith.md:245`）。
- `rg -n -i -l "net/smtp|sendgrid|mailgun|resend\.com|postmark|sesv2|gomail|SendEmail|EmailSender|mailer" backend/internal backend/cmd -g '!gen/**' -g '!*_test.go'` → 無。
- `backend/internal/platform/emailaddr` 只有 `Normalize`（`emailaddr.go:16`），是位址正規化。

**舊程式**：`backend/pkg/mail/`（`resend`、`logmail`、`fake`、`guard`）存在，但沒有任何套件 import 它：`rg -n 'hawkr/pkg/mail"|hawkr/pkg/mail/' --glob '*.go' -g '!pkg/mail/**' backend` → 無。而且 Resend 帳號與寄件網域屬於舊公司、不得使用（`docs/adr/2026-10-05-notification-module.md:27`）；新的寄信帳號、網域、SPF/DKIM/DMARC、退信 webhook 是 A8 的前置（同檔 `:65`）。

**ADR 對「沒有帳號的收件人」的規定**（同檔 `:56`）：只允許兩種外部收件人——vendor portal 的業者，與匿名通報人；後者在 10f-2a 已刪。權利人不在清單裡。

**語系**：買賣家介面 zh-TW／en（根 `AGENTS.md`「使用者面文字」）。寄給非帳號的信要用哪個語系，沒有現成機制（沒有寄信，就沒有 email 模板）。舊 notification-service 的模板與語系處理我沒有細看。

**結論**：R8 的「email 驗證」「通知補正」「轉送回復通知給權利人」都需要對非帳號位址寄信，今天做不到；這是 A8 的平台工作。

---

## 5. 詢問（R4）

**表**：`seller.case_inquiries`（`schema.sql:329-372`）、`case_inquiry_reply_evidence`（`:379-387`）。
- 掛在**案件**上（`case_id`＋`store_id` 複合外鍵，`:349`）；每個案件同時最多一筆未結束的（`:373`）。
- `kind IN ('RECORDS','EXPLANATION')`（`:350`）；`message` ≤2000；`due_at`；一則回覆（`reply_body` ≤4000）或撤回，互斥（`:370`）。
- 回覆作者二選一：管理員代記（PHONE/EMAIL/MESSAGING/OTHER）或賣家在賣家中心回（`SELLER_CENTER`）（`:357-365`）。附件最多 5 張，機敏（`:385`）。
- 註解：「10f 改成多則回覆時才拆表（觸發：賣家要能補交）」（`:327`）。

**期限**：純時數，不用工作日。`InquiryDefaultDue = 72h`、`InquiryMinDue = 24h`、`InquiryMaxDue = 14×24h`（`internal/model/case_inquiry.go:85-89`）。管理員建立時可指定（`internal/governance/inquiry.go:133`）。不發事件，沒有通知（`inquiry.go:134`）。

**賣家端**：
- API：`SellerAppService.ListMyInquiries`／`GetMyInquiry`／`ReplyToInquiry`（`api/hawkr/seller/v1alpha1/seller.proto:168`、`:173`、`:180`）；模組 `inquiry_seller.go:85`、`:129`、`:154`。
- 後台：`CreateCaseInquiry`／`ListCaseInquiries`／`RecordCaseInquiryReply`／`WithdrawCaseInquiry`（`api/hawkr/admin/seller/v1alpha1/governance.proto:85-99`）。
- website：`website/app/[locale]/(seller)/seller/inquiries/page.tsx`、`[inquiryId]/page.tsx`；`website/features/seller/inquiries/`（`SellerInquiriesPage.tsx`、`SellerInquiryDetailPage.tsx`、`InquiryReplyForm.tsx`）。
- seller-app：`seller-app/lib/features/seller/presentation/seller_inquiries_screen.dart`、`seller_inquiry_detail_screen.dart`、`seller_inquiry_reply_screen.dart`。

**能不能直接表達「通知＋有期限的回覆」**：部分可以（推論）。
- 可以：對賣家送一段文字、設期限、收一則附檔回覆、三端畫面都在。
- 不行：(a) 期限是時數不是工作日；(b) 種類只有 RECORDS／EXPLANATION，沒有「回復通知」這種有法律效果的回覆；(c) 一則回覆、不能補交；(d) 一定要先有案件，而案件是「店 × 類別」不是「一則通知」；(e) 回覆不會被轉送給第三方；(f) 沒有通知，賣家要自己進來看。

---

## 6. 工作日行事曆

套件：`backend/internal/platform/bizcalendar`（10f-4a）。

`calendar.go`：
- `func NewCalendar(days []Day) (*Calendar, error)`（`:37`）
- `func (c *Calendar) Years() []int`（`:60`）、`Covers(year int) bool`（`:63`）、`Coverage() []YearCoverage`（`:69`）
- `func (c *Calendar) LastCoveredDate() (time.Time, bool)`（`:87`）
- `func (c *Calendar) IsBusinessDay(t time.Time) (business, ok bool)`（`:95`）
- `func (c *Calendar) AddBusinessDays(from time.Time, n int) (time.Time, bool)`（`:105`）
- `func (c *Calendar) BusinessDaysBetween(from, to time.Time) (int, bool)`（`:125`）
- `func (c *Calendar) HolidaysBetween(from, to time.Time) []time.Time`（`:147`）

`bizcalendar.go`：`New(d *db.DB, audit *adminaudit.Log) *Store`（`:53`）、`(*Store).Import(ctx, actor, year, content) (ImportResult, error)`（`:94`）、`(*Store).Load(ctx) (*Calendar, error)`（`:126`）。`parse.go:30` `ParseOfficeCalendar(year int, content string) ([]Day, error)`。`mirror.go`：推給 invoice-service 的鏡像（`:21-106`）。

**`ok=false` 的語意**：
- `IsBusinessDay`：那天的年份沒匯入 → `(false, false)`（`:94-100`）。
- `AddBusinessDays`：從 `from` 的**次日**開始數，`from` 不算，`n=1` 是之後第一個工作天；`n<0` 或數的途中碰到沒匯入的年份 → `ok=false`；`from` 自己不必在涵蓋範圍（`:103-104`）。回傳的是 UTC 午夜表示的台北日曆日，不是時刻。
- `BusinessDaysBetween`：區間內任何一天沒涵蓋 → `(0, false)`，「0 與算不出來不同」（`:123-124`）。
- 決定：platform 不替呼叫者選偏早或偏晚，「R8 平台義務偏早」由呼叫者決定（`2026-09-29-a3-buyer-seller.md:436`，D2）。

**目前的消費者**（`rg -n 'platform/bizcalendar"' --glob '*.go' -g '!internal/platform/bizcalendar/**' backend`，去掉測試）：
- `backend/cmd/hawkr/main.go:59`、`modules.go:18`（組裝與鏡像推送）
- `backend/internal/httpapi/server.go:67`、`backend/internal/httpapi/bizcalendar/admin.go:14`（後台「假日行事曆」頁）
- **沒有任何業務模組在算期限**。R8 會是第一個用 `AddBusinessDays` 的消費者。跨年的期限在下一年還沒匯入時會 `ok=false`，要有對策。

---

## 7. 未登入者的檔案上傳、佐證讀取紀錄

**上傳一定要登入**：
- gateway 的 media 路由全掛在 `JWTAuth()` 底下（`backend/gateway/internal/router/routes_media.go:19-32`）：`upload_sessions`、`secure_upload`、`secure_download`、`secure_preview`。函式有 `public` 參數但沒有掛任何路由。
- 轉正時 media 確認 staged token 屬於上傳者：`media.Uploader{ID int64; Admin bool}`（`backend/internal/modules/media/internal/model/` 的 `Uploader`，註解「轉正時 media 確認 staged token 屬於他」）。只有「帳號」與「管理員」兩種，沒有匿名。
- 所有現有呼叫都帶帳號或管理員 ID：`governance/report.go:77`、`:222`、`inquiry_seller.go:173`、`inquiry.go:280`、`measure/appeal.go:87`。
- media 實作還在 Java（`backend/internal/modules/media/AGENTS.md`「職責」），B1 才搬。
- 舊的匿名通報本來就「不收佐證」（schema 註解 `:291`）。

**evidence_accesses**（`schema.sql:871-887`）：append-only，後台每讀一張先寫一列（`admin_id`、`accessed_at`、`position`）。擁有者三選一：`report_id`＋`submission_seq`、`request_id`、`inquiry_id`，各有複合外鍵指向佐證列（`:881-885`）。加第四種佐證要加欄位與外鍵。守門測試 `TestEvidenceReadIsRecordedBeforeTheBytesAreRead`（`seller/internal/governance/AGENTS.md`）。

**結論**：權利證明文件（R8「email 驗證加權利證明」）今天沒有未登入的上傳路徑。

---

## 8. moderation（A7，UGC 檢舉）

- 範圍：社群內容、評價、聊天訊息與買家（`backend/internal/modules/moderation/AGENTS.md`「職責」）；「店家與商品的通報與案件留在 seller」（同檔「不負責」）。
- 檢舉人只能選 8 個原因：`CommunityReporterSelectable()` = Spam、Harassment、Hate、SexualContent、MisleadingInfo、FoodSafety、Fraud、Other（`backend/internal/platform/policy/policy.go:91-95`）。**沒有 `IP_INFRINGEMENT`**。
- 但平台**判定**可以用：`CommunityAssessable()` 含 `IPInfringement`（`policy.go:84-89`）。所以管理員可以把 UGC 案件認定為侵權並隱藏／下架，只是檢舉人入口選不到，要走「其他」。
- 檢舉人一定是登入買家（`reports.reporter_buyer_id`，去重 `UNIQUE (case_id, reporter_buyer_id)`，AGENTS.md「不變量」）。
- 店家寫的內容下架走 seller 的 `CONTENT_TAKEDOWN`（`backend/internal/modules/moderation/internal/handle/handle.go:376-382`）。

**共用的政策分類契約**：
- proto：`api/hawkr/common/v1alpha1/policy_category.proto`（`COUNTERFEIT = 6` `:20`；`IP_INFRINGEMENT = 7` `:22`，註解 `:21`「侵害他人商標、著作權、專利、營業秘密、肖像或其他權利的使用（第十六條第 5 款）…仿冒商品本身是 COUNTERFEIT」）。
- Go：`backend/internal/platform/policy/policy.go`（`Counterfeit` `:36`、`IPInfringement` `:38`；子集函式 `:62-95`）。
- 決定：`docs/adr/2026-10-18-seller-governance-consolidated.md:53-56`。
- 注意：`IP_INFRINGEMENT` 一個值涵蓋著作權、商標、專利、營業秘密、肖像；R8 要著作權走法定流程、商標專利走平台流程，現有分類分不出來。

---

## 9. 重複侵權／累計

- 唯一的累計規則：帳號 90 天內第 3 次 **MEDIUM** 認定升為 HIGH，記 `escalated_from`（`schema.sql:391-392`、`:417`、索引 `:428-429`；`internal/measure/create.go:329`；`backend/docs/modules/seller.md:114`；`CARRYOVER.md:101`）。以帳號計、只計 ENFORCED、不分類別。它只改嚴重度，**不會自動建立措施**。
- 帳號層級的終止：`TERMINATION` 措施，`termination_ground` 11 種（`schema.sql:636-639`），沒有「重複侵權」這個事由；最接近的是 `MATERIAL_BREACH`（必須依據認定，`:643`）與 `OTHER_SERIOUS`。終止都是管理員手動建立。
- 買家側：social 的 `sanctions.kind` 只有 `WARNING`（`backend/internal/modules/social/db/schema.sql:301`）；停權經 `accountsuspension`。沒有看到累計規則：`rg -n -i -c "escalat" backend/internal/modules/social/internal backend/internal/modules/accountsuspension -g '!*_test.go'` → 無。
- 「三次侵權終止」：`rg -n "三次|第 3 次|第三次|three.?strike|repeat.?infring|累犯|屢次|重複侵權" backend/internal backend/docs/modules docs/adr website/features/legal website/docs/legal` → 只命中上面的 MEDIUM 升級規則與無關項目。**沒有按侵權次數計數或終止的機制，也沒有這樣的文字。**

---

## 10. 法律文字

**重要：簡報引用的條文今天不在 repo 裡。** 所有法律頁在 2026-10-25 改成「示範專案說明」，條文全部拿掉。
- `website/features/legal/pages/UserPolicyPage.tsx`（36 行）：只有一節「一、示範專案說明」（`:24-33`），`lastUpdated="2026-10-25"`（`:21`）。**沒有 §十二。**
- `website/features/legal/pages/TermsPage.tsx`（47 行）：兩節——示範說明、費用（`:23-44`）。**沒有 15.5／16.1／16.5，也沒有 §八。**
- `website/docs/legal/seller-terms.md`（11 行）：「賣家合作暨平台服務契約（示範說明）…示範專案，無法律效力」，五點示範聲明。**沒有第十五、十六、二十一條。**
- `PrivacyPage.tsx` 55 行、`SellerTermsPage.tsx` 56 行，同樣是示範版。共用文字在 `website/features/legal/DemoNotice.tsx`：「本頁只用來展示{topic}的流程與版面，不構成任何契約或法律義務」。
- 搜尋：`rg -l "智慧財產|著作權" website -g '!node_modules' -g '!.next'` → 無；`rg -n "智慧財產|著作權|侵權|權利人|三次|聯繫窗口" website/features/legal/documents website/docs/legal` → 無。
- git 歷史也找不回：這些檔只有一個 commit `9f23df58 2026-10-27 feat: Hawkr 初始版本`；`git log -S"三次" -- website/features/legal website/docs/legal` → 無。
- 另：I-legal 提到的 `website/docs/legal/seller-food-registration-notice.md` 已不存在（目錄現有 `self-issue-attestation-terms.md`、`seller-agreements.json`、`seller-kyc-notice.md`、`seller-terms.md`）。

**舊條文唯一留下的引述**（二手，來自 2026-10 的盤點，不是現行文字）：`backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-reports-facts.md:60-61`
- 賣家條款 15.5：「收到具體侵權通知時，甲方得先行保全證據、限制內容或要求補件，並應提供乙方合理申覆機會；依法須立即處理者不在此限。」
- 使用者政策 §十二：「notice-and-takedown under the Copyright Act ISP chapter with required fields and counter-notice, but no channel」（英文摘要，沒有原句）。
- 三次侵權條款、聯繫窗口：當時就沒有（`step10e-reports-brief.md:328`「現在兩份文件都沒有這些」）。

**後果**：程式碼與 proto 註解還在引用不存在的條號——`policy_category.proto:21`「第十六條第 5 款」、`policy.go:35-37`、schema 註解 `:586`「賣家合作條款第 21 條」、`:817`「條款 18.4」「17.4」。R8 的前提「條款與使用者政策告知著作權保護措施、三次侵權、聯繫窗口」（著作權法 90-4 四款）今天沒有任何文字可以「補齊」，要從零寫，而且頁面現在聲明自己沒有法律效力。

**誰擁有修改**：I-legal 步驟，未開始。
- `backend/docs/plans/2026-09-29-a3-buyer-seller.md:443`：標題只寫「詐欺犯罪危害防制條例第 37 條第 1 項的保存期」，三個子項都是保存期（`:444-446`），**沒有列 R8 的著作權文字**。
- R8 的文字歸 I-legal 只寫在簡報：`step10e-reports-brief.md:67`（「使用者政策 §十二、條款 15.5…I-legal 那一刀，要你同意」）、`:466`。
- I-legal 還引用已經不存在的句子（「關閉商店後 30 天」「關店三十日後刪除」），本身也過期了。

---

## 11. website／admin 的現有介面

**website 不必登入的表單**：
- 路由群組：`website/app/[locale]/` 下 `(account)`、`(auth)`、`(commerce)`、`(content)`、`(public)`、`(seller)`。
- `(content)` 有 `contact`、`faq`、`privacy`、`return-policy`、`seller-guide`、`seller-terms`、`terms`、`user-policy`。
- 唯一的公開表單是聯絡頁 `website/features/content/contact/ContactPage.tsx`：**送到 Google Forms**（`:9` formResponse 網址、`:140-145` `entry.*` 欄位），不經過單體；另有 `mailto:supports@example.com`（`:22`）。
- 沒有任何公開表單寫進單體（與第 3 題一致）。

**10f-2b 的檢舉表單元件**：`website/features/report/`
- `ReportFormParts.tsx`（共用表單零件）、`ReportDialog.tsx`、`SupplementDialog.tsx`（兩者都用 `ReportFormParts`）、`ReportMenu.tsx`、`api.ts`、`hooks.ts`、`report-logic.ts`。
- 是登入後的對話框，掛在店面與商品頁「⋯ → 檢舉」（`2026-09-29-a3-buyer-seller.md:409`）。佐證走機敏上傳（要登入）。
- 「我的檢舉」：`/account/reports`、`/account/reports/[id]`（同檔 `:425`）。

**admin**（`admin/src/app/admin/`）：
- `violation-reports/`：通報清單 `page.tsx`、詳情 `[id]/`、`cases/`、詢問（`inquiry-actions.ts`、`use-case-inquiries.ts`）。
- `governance/`（註記、共用零件）、`measures/`、`authority-orders/`、`moderation/cases/`、`queue/`（待辦）、`holiday-calendar/`、`products/`、`merchants/[id]/`（含強制下架對話框）。
- 沒有權利人通報的頁面。

**adminperm**（`backend/internal/platform/adminperm/adminperm.go`）：
- 通報 `:64-69`：`seller.reports.list`／`get`／`mark_supplements_seen`／`triage`／`resolve`／`move`；佐證 `seller.report_evidence.get`（`:88`）。
- 案件 `:70-75`：`seller.cases.list`／`get`／`decide`／`change_severity`，`seller.case_notes.create`／`list`。
- 詢問 `:76-80`；來函 `:81-87`；認定 `:89-90`；措施 `:91-100`。
- moderation `:43-46`；bizcalendar `:25-26`。
- 規則：每支 `*AdminService` 方法都要宣告權限，新的後台寫入宣告 `audited` 並同 transaction 記 `adminaudit`（`backend/AGENTS.md`「單體的結構」）。

---

## 12. 相關 ADR

`rg -l "權利人|著作權|智慧財產|notice-and-takedown|侵權" docs/adr` → 只有兩份，而且都只是順帶提到，**沒有任何 ADR 對權利人通報做過決定**。

- `docs/adr/2026-10-18-seller-governance-consolidated.md`（治理的現行 ADR）
  - `:39-51` 通報與案件是兩個資源；案件＝店 × 平台判定類別。
  - `:53-56` 政策分類放 common 與 platform，seller 與 moderation 共用。
  - `:58-64` 措施種類、效果只由措施推導。
  - `:34` 佐證是機敏檔，只經後台端點讀、先記錄再讀（引 Amazon 侵權通報的附件處理當基準）。
  - `:91-93` 通知由 A8 訂閱事件。
  - `:151` 起 修訂：內容下架措施與社群認定類別（A7）。
  - `:190-199` 修訂：匿名入口刪除；`:199`「10f-5 的權利人通報不預先保留這些欄位」。
- `docs/adr/2026-10-02-seller-governance-in-monolith.md:87`：佐證分級（已被上一份合併）。
- `docs/adr/2026-10-27-general-ecommerce-scope-cut.md:35`：D3 刪食品業者登錄與 FDA 轉報；「通報、案件、處分、主管機關來函保存保留」。
- `docs/adr/2026-09-18-seller-governance-schema-and-enforcement.md:70-73`：決策六，公開食安端點用雙層 IP 限流、不上 captcha（端點已刪；理由只針對食安）。
- `docs/adr/2026-10-05-notification-module.md:56`、`:61-65`：email 只寄交易與安全類；非帳號收件人只有兩種；寄信的平台工作在 A8。
- `docs/adr/2026-10-03-media-references-owner-side.md`：media 終點是不認識業務，機敏下載由擁有者端點代理（經 `media/AGENTS.md` 引述，我沒有讀全文）。
- `docs/adr/2026-11-14-platform-admin-audit-log.md`：後台寫入記平台層稽核（經 `backend/AGENTS.md` 引述，沒有讀全文）。
- `docs/adr/2026-10-29-account-suspension-and-moderation.md`、`2026-09-18-content-moderation-consolidation.md`：檔名相關，沒有讀。

---

## 13. 之後的步驟會不會重寫 10f-5 碰到的程式

主計畫 `backend/docs/plans/2026-09-24-go-modular-monolith.md`：

- **B4 catalog（未開始，`:303`）— 影響最大**。`:309` 會把商品限制做成 seller 措施（`measure_contents.content_kind` 加 PRODUCT），並把強制下架對話框改掉。10f-5 若現在做商品移除／恢復，只能包在 Java 的舊 REST 外面，B4 時重做；`step10e-measure-brief.md:185` 已經把這種做法判為「拋棄式工作」。
- **A8 notification（未開始，`:245`）**。`:250`：「治理的通知…訂閱 seller 的 `ReportReceived`、`ReportResolved`（通報人：登入的用站內通知、匿名但留了 email 的用 email，確認信依收件 email 節流），以及在 seller 加上詢問的事件並訂閱」。這段的匿名半段已過期（10f-2a）。10f-5 需要的所有寄信都卡在這裡。
- **B1 media（未開始，`:287`）**。`:288`：media 改成不認識業務，引用改成確認／刪除介面。10f-5 若為未登入者開新的上傳路徑，會開在 Java media-service 與 gateway 上，B1 時重做。
- **A6 review、im、crm（進行中，`:208`）**。`:209`：moderation 加聊天與評價對象。只在 10f-5 要涵蓋 UGC 侵權時才相關。
- **A7 延後項 `:231`**：「案件改判…觸發：通報人對處置的申訴（10e R12）」。
- seller 的通報／案件／措施本身：沒有看到排定重寫。只有「來函暫時留在 seller（設計 B32）」待重新評估（`docs/adr/2026-10-18-seller-governance-consolidated.md:144`）。
- **10f-6（`2026-09-29-a3-buyer-seller.md:442`）**：收尾，10f-5 完成後做 design-review。

---

## 沒有驗證到的

- 沒有讀 `step10e-reports-research.md` 全文，也沒有核對 R8 引用的法條內容。
- 沒有讀 Java `updateProductStatus` 第 1450 行之後（是否發事件、是否留紀錄）；「擋不住重新上架」依據的是 admin 對話框的文案與 `updateSellerProductStatus` 存在，不是實測。
- 沒有看 CSRF／origin 檢查對公開寫入的處理（`backend/internal/httpapi/origin.go` 存在，未讀）。
- 沒有看舊 notification-service 的模板與語系。
- buyer-app／seller-app 有沒有不必登入的表單沒有查（只查了 website）。
- 沒有逐一核對 `backend/docs/modules/seller.md` 規格與 schema 是否一致，以 schema 為準。
