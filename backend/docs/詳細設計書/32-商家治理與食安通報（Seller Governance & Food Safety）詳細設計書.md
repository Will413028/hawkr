# 商家治理與食安通報（Seller Governance & Food Safety）詳細設計書

> 為食品電商補齊商家層級違規通報、食安緊急通報、admin 處罰、賣家申訴流程。
> 與 31 號（內容檢舉）目標互補：B-135 處理 UGC 內容，本文件處理商家實體治理。

> 現行決策見 ADR [`docs/adr/2026-10-18-seller-governance-consolidated.md`](../adr/2026-10-18-seller-governance-consolidated.md)（單體的 seller 模組；取代舊 ADR [`2026-09-18-seller-governance-schema-and-enforcement.md`](../adr/2026-09-18-seller-governance-schema-and-enforcement.md) 的決策二、三、七、八，決策一、五、六仍有效）。資料表與 CHECK 的唯一來源是 `backend/internal/modules/seller/db/schema.sql`，模組的不變量見 `internal/modules/seller/AGENTS.md`。
>
> §4.1–§4.3、§5.2–§5.3 描述的是舊 gateway 的 REST 與時序，通報的對外 API 在遷移計畫 A3b 步驟 8 依本文件 §3 的資料模型以 Connect 重新設計（現行契約是 `api/hawkr/seller/v1alpha1/report.proto` 與 `api/hawkr/admin/seller/v1alpha1/governance.proto`）；通報、案件、措施與認定的 API（§4.4、§4.6）、§5.1 與 §3 是現行規格。通報與案件分開（ADR 2026-10-18-seller-governance-consolidated「決策」的「通報與案件」）之後，本文件舊的「確認收件、調查」流程不再適用，§3、§4.6、§6.1、§7 已改。

---

## 1. 範圍與邊界

本模組負責商家層級的違規通報、食安緊急通報、admin 處罰與賣家申訴。
31 號（內容檢舉）處理 UGC，本文件處理**商家實體**治理，兩者互補。

### 1.1 三條檢舉通道

| 通道 | 通報人 | 備註 |
|---|---|---|
| 登入端通報 | 登入的買家（`reporter_buyer_id`） | 可附佐證圖片；同店家、同類別一個台北日一筆 |
| 匿名通報（含食安） | 留下電話或 email 的匿名者 | 不收佐證；同電話或 email、同店家一個台北 ISO 週一筆；入口有 IP 限流（舊 ADR 決策六） |
| admin 建立措施時新增的認定 | admin | 可以不帶來源通報 |

兩種通報人共用同一張通報表，`FOOD_SAFETY` 是類別之一；通報與案件分開，通報有結果、不再有狀態機（ADR 2026-10-18-seller-governance-consolidated「決策」的「通報與案件」）。

### 1.2 現行值域與邊界

| 項目 | 現行規格 |
|---|---|
| 通報與認定的類別 | 平台共用的政策分類 `PolicyCategory`（契約 `api/hawkr/common/v1alpha1/policy_category.proto`，Go 的 `internal/platform/policy`），13 個值，每個對應賣家條款的依據，給賣家的理由要能指到條款（用語見〈使用者面用語術語表〉「治理」）。各入口只接受自己的子集，伺服器逐入口驗證：檢舉人可選 12 個（沒有 `PLATFORM_ABUSE`），平台判定與認定可用 12 個（沒有 `OTHER`，`OTHER` 只給檢舉人選）；ADR 2026-10-18-seller-governance-consolidated「決策」的「政策分類」 |
| 措施種類 | 警告（`WARNING`，沒有限制）與停權（`SUSPENSION`，VIOLATION 依據有固定結束時間、RISK 依據沒有固定結束時間但有複審期限）；其他種類等有執行者時再加 |
| 申訴 | 每個**措施**限申訴一次（`measure_requests`）；接受申訴＝撤銷該措施 |
| 補正後請求解除 | 只限依據風險類型（RISK，含條款第 17.5 條的外部要求）的措施，可重複提出；接受補正＝措施結束（不是撤銷）；同一措施同時只有一筆待處理的請求（申訴與補正合計）；累計駁回超過兩次後有冷卻（條款第 17.4 條） |
| 公開違規紀錄 | **不公開**（ADR 2026-10-02）；停權的店家依停權事件下架 |
| 審閱期限 | 11 種待辦各有營運的處理期限（日曆時間，讀取時推導、不另存）：分案（檢舉人選食安 4 小時、其他 72 小時）、決定（依嚴重度 24 小時／3 天／7 天／14 天，從可以決定起算）、詢問逾期（賣家期限後 24 小時）、轉報（決定後 24 小時）、來函（來函的期限）、告知被指名者（確認日期起 3 天）、申覆與解除請求（7 天）、複審（`review_by`，前 24 小時進入）、食品業者登錄（可以裁定起 7 天）、商家認證（送出起 7 天）。規則只寫在 seller 的 `review_<種類>(at)` 函式；後台待辦 `/admin/queue` 依期限排序（ADR `2026-10-20-admin-review-queue`）；催辦手段是 admin UI 的視覺強調與排班輪詢，不推播 |
| 詢問 | 平台對案件的賣家的一次要求（條款第 11.3 條）：期限預設 72 小時，一則回覆（管理員記下系統外收到的內容），可撤回；每個案件同時最多一筆還沒結束的；賣家端的畫面與通知在 10f、A8 之前沒有，請管理員另外聯絡賣家 |
| 主管機關來函 | 主管機關給平台的公文（限制或移除商品、調取資料、暫停帳號、其他），獨立的收件，不是通報：指名的帳號或店家（任何帳號）、期限（48／72 小時）、完成與每個對象的告知；**保存期限**（預設收文後 184 天，只能延長）讓被指名者在 seller 的個資不被關店後的清理刪除，不影響恢復營業 |
| FDA 通報 | **手動流程**：模組提供轉報資料（案件、案件裡結果沒有被否定的通報與店家），admin 在案件上記下管道、案號與追蹤日期（預設轉報日加 30 天），每個案件只記一次，也可以標記「不轉報」（附原因，與轉報互斥），轉報之後可以只新增記下主管機關的回覆；**不**串接 FDA API |
| 機器人防護 | 匿名入口的雙層 IP 限流 ＋ DB partial UNIQUE 去重；**不**使用 captcha |
| 個資保留 | 匿名檢舉人的電話、email、姓名在 `greatest(建立 + 90 天, 結果 + 30 天)` 之後清成 NULL；不論有沒有結果，建立滿 365 天一律清（絕對上限，2026-10-18） |

### 1.3 與 31 號（內容檢舉）的對照

| 維度 | 31 號 內容檢舉 | 本文件 商家治理 |
|---|---|---|
| 對象 | UGC（POST / COMMENT / MESSAGE / REVIEW / USER） | 店家（商品、訂單只是參照） |
| 入口 | 必登入 | 登入的買家 ＋ 匿名（留聯絡方式） |
| 嚴重度 | spam / hate / adult / fraud / harassment / other | `LOW` / `MEDIUM` / `HIGH` / `CRITICAL`，是案件的欄位，admin 開案時決定 |
| 處分結構 | hide / warn / ban / dismiss | 措施：警告、停權（依據是認定或風險類型） |
| 透明化 | 不公開（隱藏內容） | 不公開；給賣家的措施理由（`statement_to_seller`，存在措施版本）必填 |
| 擁有者 | social | 單體的 seller 模組（`internal/governance`） |

---

---

## 2. 架構總覽

治理是 seller 模組的子套件（`internal/modules/seller/internal/governance`），與店家、措施在同一個 schema、同一個 transaction：

- **通報與案件**：通報是一位檢舉人的一次送出（提出、同一位檢舉人再次送出的補充、設定結果）；案件是平台對「這家店、這個平台判定類別」的處理單位（分案、移動、嚴重度、決定即結案、內部註記、FDA 轉報）。要求賣家配合是案件的**詢問**（§3.4、§4.6）：平台對案件的賣家的一次要求，有期限、一則回覆、可撤回。
- **認定**：平台對店家某類違規的認定（類別、嚴重度）、升級規則、撤銷；只讀，只經措施建立與撤銷。
- **措施**：admin 對帳號的決定（警告或停權），依據是認定或風險類型；給賣家的理由與結束時間、內部註記、複審期限、撤銷，以及賣家的申訴與 admin 的裁定（§3.3）。停權狀態不存，每次讀取時由措施推導（§3.5），停權期間與事件只由 `suspension.Recompute` 維護。
- **佐證**：通報、申訴與詢問回覆的附件經 media 轉正，以通報、申訴請求或詢問的 ID 登記引用（`GOVERNANCE_EVIDENCE`）。
- **事件**：治理的狀態改變在寫入的 transaction 內發模組事件（§7）。

**結構性規則**：

| 規則 | 內容 |
|---|---|
| 表歸屬 | 治理的表在 schema `seller`，與 social 的內容檢舉分開（舊 ADR 決策一） |
| 一張通報表 | 通報人剛好一種（CHECK）；通報有結果、沒有狀態欄；FDA 轉報在案件上，只在食安案件（CHECK） |
| 通報與案件分開 | 通報（`reports`）與案件（`cases`）是兩個資源；同一家店、同一個平台判定類別同時最多一個進行中的案件（`cases_one_open`）；處理狀態與可以做的動作在讀取時由事實推導（`ReportFacts`、`CaseFacts` 的 `Availability`） |
| 認定與措施分開 | 認定只記「平台認定了什麼」；限制、理由、結束時間與申訴在措施。沒有獨立的處分表：措施是停權狀態的唯一來源 |
| 升級規則串行 | 帳號 90 天內第 3 次 `MEDIUM` 升為 `HIGH`，以**帳號全部 stint 的鎖**串行（不用 advisory lock） |
| 措施的狀態 | 儲存的只有 `ENFORCED` / `REVOKED`；是否生效由 `created_at`、`measures.ends_at`（目前值）與傳入的時間在讀取時判斷（`EffectiveMeasures`；問全部帳號用同一個定義的 `EffectiveMeasuresAll`） |
| 去重 | partial UNIQUE（舊 ADR 決策五），只比對還沒有結果的通報，日與週以台北時間為界；撞到時併成補充 |
| 食安 SLA | 4 小時，分案即計時結束；不推播 |
| 通知 | 模組事件；送達與送達紀錄屬於 A8 的 notification，沒有 `seller_notification_log` |
| 評分 | seller 不存店家評分，也不處理 review 事件；店家頁的評分在呈現層組裝 |

---


## 3. 資料模型

表、欄位與 CHECK 的唯一來源是 `internal/modules/seller/db/schema.sql`；本節說明語意。通報、認定與措施的 ID 是 UUIDv7（應用層產生，row-identity ADR）；店家與停權期間（`period_id`）的 ID 仍是 IDENTITY（Java 以 BIGINT 存：product 的下架快照以 `period_id` 為 scope）。儲存的代碼都是 UPPER_SNAKE 加 CHECK。

### 3.1 `seller.reports`（通報）

一列是一位檢舉人的一次送出（同一位檢舉人在去重期間內再次送出的是補充，不是新的一列）。

| 欄位 | 說明 |
|---|---|
| `store_id` | 被通報的店家（必填、必須存在；已關閉的也可以通報；登入的檢舉人不能檢舉自己的店） |
| `product_id`、`order_no` | 不透明的參照，不驗證存在 |
| `category` | 檢舉人選的政策分類，12 個值：`FOOD_SAFETY`、`PROHIBITED_GOODS`、`UNLICENSED_SALE`、`MISLEADING_INFO`、`FAKE_CERTIFICATION`、`COUNTERFEIT`、`IP_INFRINGEMENT`、`FRAUD`、`OFF_PLATFORM_TRADE`、`PRIVACY`、`IMPERSONATION`、`OTHER`（`reports_category_check`；沒有只給平台判定的 `PLATFORM_ABUSE`；舊的 `UNAUTHORIZED_SALE` 改名 `UNLICENSED_SALE`） |
| `reporter_buyer_id` | 登入的檢舉人（業務鍵）；與下列匿名聯絡欄位剛好一種 |
| `reporter_phone`、`reporter_email`、`reporter_name` | 匿名檢舉人的聯絡方式（電話或 email 至少一個；電話正規化成 `0` 開頭、email 小寫）；在 `greatest(建立 + 90 天, 結果 + 30 天)` 之後清成 NULL 並記下 `contact_erased_at`；不論有沒有結果，建立滿 365 天一律清 |
| `case_id`、`assessed_category`、`triaged_at`、`triaged_by_admin_id` | 分案：所在的案件與平台判定類別（等於案件的類別；`(case_id, store_id, assessed_category)` 複合外鍵保證與案件同店同類別）、第一次分案的時間與管理員（移動不改）；全有或全無（`reports_triage_check`） |
| `outcome`、`resolved_at`、`resolved_by_admin_id`、`resolution_note` | 結果（`ACTION_TAKEN`、`NO_VIOLATION`、`INSUFFICIENT`、`DUPLICATE`、`ABUSIVE`，`reports_outcome_check`）與設定的時間、管理員，全有或全無（`reports_resolved_check`）；`resolution_note` 是給檢舉人的結果說明；`ACTION_TAKEN` 一定在案件裡（`reports_action_taken_check`）。結果設定後不能改 |
| `discarded_submissions`、`version` | 補充已達上限而沒有存下的匿名送出次數；通報的版本（API 的 etag），補充、分案、設定結果與移動都讓它前進 |

沒有狀態欄：通報此刻是待分案（沒有案件也沒有結果）、在案件中（有案件、沒有結果）或已有結果，讀取時由事實推導（`Report.State`）。

去重（partial UNIQUE，只比對還沒有結果的通報；日與週以台北時間為界）：

- 登入：`(reporter_buyer_id, store_id, category, 台北日)`
- 匿名：`(reporter_phone, store_id, 台北 ISO 週)` 與 `(reporter_email, store_id, 台北 ISO 週)`

撞到時不是失敗也不是新通報，而是**補充**：這次的內容併成原通報的 `report_supplements` 一列（§3.1.1），原通報的版本前進。一筆通報最多 10 則補充：超過時登入的回 `failed_precondition`，匿名的內容不存、只計入 `discarded_submissions`。匿名時入口不回不同的結果（新的、補充與丟棄都回空的回應），避免洩漏電話用過；登入的補充回原通報（去重的鍵含買家，一定是呼叫者自己的）與一句說明。原通報已經有結果時不再去重，這次是新的檢舉。

分案期限是建立後 4 小時（檢舉人選了食安）或 72 小時，只在待分案時有，讀取時由 `seller.review_report_triage(at)` 推導、不另存（`Report.triage_deadline`）；所有待辦的期限見 §1「審閱期限」。

#### 3.1.1 補充與移動

- `seller.report_supplements`：同一位檢舉人在去重期間內再次送出、併入原通報的內容（`description`、`matched_by`〔匿名：以電話、email 或兩者比對到，登入的沒有〕、`created_at`），只新增，不存聯絡方式。佐證的 `report_evidence.supplement_id` 指向帶來它的補充（原通報的佐證是 NULL）。
- `seller.report_case_moves`：通報移到其他案件的紀錄（`report_id`、`from_case_id`、`to_case_id`、`admin_id`、`moved_at`），只新增；通報與兩個案件必須同店（複合外鍵）。

#### 3.1.2 `seller.cases`（案件）與 `seller.case_referrals`（FDA 轉報）

案件是平台對「這家店、這個平台判定類別」的處理單位：同一家店、同一個類別同時最多一個進行中（`cases_one_open`），決定即結案，結案是終態。

| 欄位 | 說明 |
|---|---|
| `store_id`、`category` | 案件的店家與平台判定類別（政策分類中平台判定可用的 12 個值，沒有 `OTHER`） |
| `severity` | `LOW`、`MEDIUM`、`HIGH`、`CRITICAL`；開案時由 admin 決定，之後只經 `ChangeCaseSeverity`（必須附原因，寫成案件的內部註記，註記帶改前改後） |
| `opened_at`、`opened_by_admin_id` | 開案 |
| `closed_at`、`closed_by_admin_id`、`close_reason`、`decision`、`decision_note` | 結案欄位，全有或全無（`cases_close_check`）：`close_reason` 是 `DECIDED`（管理員決定，`decision` 是 `ACTION_TAKEN` 或 `NO_ACTION`，`decision_note` 是給檢舉人的說明）或 `NO_REPORTS`（通報都移到其他案件，系統關閉，沒有決定） |
| `version`、`updated_at` | 案件的版本（API 的 etag）與上一次處理的時間：成員進出、成員有了結果、改嚴重度、記錄轉報、標記不轉報、記下主管機關的回覆與結案讓它們前進；補充、純註記與措施不會（案件顯示 `unseen_supplement_report_count` 提醒結案前先看） |

`seller.case_referrals`（`case_id` 為主鍵，每個案件最多一筆）：食安案件向主管機關的轉報紀錄，`outcome` 是 `REFERRED`（已轉報：管道 `HOTLINE_1919`、`ONLINE_FORM`、`FAX_HEALTH_DEPT`、`IN_PERSON`、`OTHER`、選填案號、追蹤日期 `follow_up_on`）或 `WAIVED`（不轉報：原因 1–2000 字，沒有管道、案號與追蹤日期；CHECK 保證兩種互斥），加上記下的時間 `recorded_at` 與管理員 `recorded_by_admin_id`；案件的類別必須是食安（複合外鍵加 CHECK）。重送回原紀錄，不覆寫。

`seller.case_referral_replies`（只新增）：主管機關對轉報的回覆，內容 1–2000 字、收到的日期 `received_on`（台北日期）、記下的時間與管理員。固定為 `REFERRED` 的 `outcome` 欄加複合外鍵 `(case_id, outcome)` 指向 `case_referrals`，DB 保證回覆只掛在已轉報的列，不能掛在不轉報的列。

決定的規則：「採取措施」要案件已有作為（有導出的措施，不論狀態，或已有成員的結果是已採取措施）；「不採取措施」要案件沒有未撤銷的導出措施、也沒有已採取措施的成員。所以每個進行中的案件至少有一個可選的決定。決定把還沒有結果的成員帶入「已採取措施」或「未違規」。案件導出的措施是 `measures.source_case_id` 指向它，或措施依據的認定以它的成員為來源。

### 3.2 `seller.violations`（認定）

違規只是**認定**：平台認定某店家有某類違規。限制與申訴在措施（§3.3），認定本身不帶處分、申訴與版本。

| 欄位 | 說明 |
|---|---|
| `account_id`、`store_id` | 被認定的帳號與認定時的店家（只能建在還沒關閉的 stint）；升級規則以 `account_id` 計 |
| 來源通報（關聯表 `finding_reports`：`violation_id`、`report_id`） | 成立這個認定的通報，多對多，一個認定最多 20 筆、可以沒有；每一筆都必須是同一個店家、結果不能是未違規、內容不足或濫報（`Report.Supporting`）。只在建立措施的 transaction 寫入（先鎖店家列，再依通報 id 升序鎖住這些通報列後檢查），之後只讀；API 是 `NewFinding.source_report_ids` 與 `Violation.source_report_ids`（依通報 id 排序） |
| `category` | 政策分類中平台判定與認定可用的 12 個值：`FOOD_SAFETY`、`PROHIBITED_GOODS`、`UNLICENSED_SALE`、`MISLEADING_INFO`、`FAKE_CERTIFICATION`、`COUNTERFEIT`、`IP_INFRINGEMENT`、`FRAUD`、`OFF_PLATFORM_TRADE`、`PRIVACY`、`IMPERSONATION`、`PLATFORM_ABUSE`（`violations_category_check`；沒有 `OTHER`：認定的理由要指到條款，所以不能是「無法歸類」；建立措施帶 `OTHER` 回 `invalid_argument`） |
| `severity`、`escalated_from` | `severity` 是套用的嚴重度；升級規則把 `MEDIUM` 升為 `HIGH` 時 `escalated_from = 'MEDIUM'` |
| `origin_measure_id` | 建立這個認定的措施（措施撤銷並帶 `revoke_findings` 時，只撤銷自己建立的認定） |
| `status`、`revoked_reason`、`revoked_by_admin_id`、`revoked_at` | `ENFORCED` → `REVOKED`；撤銷只經措施的撤銷（`revoke_findings`），原因取 `APPEAL_ACCEPTED`、`ADMIN_CORRECTION`、`EXTERNAL_DECISION`、`OTHER`（`FINDING_REVOKED` 是措施的撤銷原因，認定沒有） |

升級規則：**帳號**在 90 天內（所有 stint 一起算）已有 2 筆未撤銷、原本是 `MEDIUM` 的認定時，這一筆 `MEDIUM` 升為 `HIGH`；建立時先鎖帳號全部的 stint，計數與新增串行（索引 `violations_escalation (account_id, created_at)`）。

### 3.3 `seller.measures`（措施）與申訴

措施是**效果的唯一來源**（ADR 2026-10-18-seller-governance-consolidated「決策」的「認定、措施與效果」）：沒有獨立的處分表，也沒有效果表。

| 表 | 說明 |
|---|---|
| `measures` | 一列一個措施：`kind`（`WARNING`／`SUSPENSION`／`TERMINATION`）、`basis`（`VIOLATION`／`RISK`／`CONTRACT`，CONTRACT 只配終止）、`starts_at`（不可變的開始時間）、`termination_ground`（終止才有）、`source_measure_id`（關聯帳號的終止才有）、`risk_ground`（RISK 才有，沿用既有的風險類型值）、`lead_finding_id`（VIOLATION 的主要依據）、`review_by`（RISK 必填的複審期限，上限 30 天）、`statement_to_seller` 與 `ends_at`（賣家看得到的內容的目前值；`ends_at` 空是沒有固定期間，警告一定空、依據認定的停權一定有）、`status`（`ENFORCED`／`REVOKED`）、撤銷紀錄、`account_id`、`store_id`、`created_at`。`version` 是後台的 etag（新版本、撤銷、複審期限、請求的提出、裁定與系統結案時加一，註記不加），`current_revision` 是賣家的 etag |
| `measure_revisions` | 只新增的版本歷史（SQL:2011 系統版本表的歷史表）：每一版的完整快照（`statement_to_seller`、`ends_at`）與誰、何時，含目前那一版；建立時第 1 版，每次實際改了理由或結束時間，與更新 `measures` 目前值在同一個語句寫下一版 |
| `measure_findings` | 措施引用的認定（一到多筆，可引用既有的），是集合，沒有順序；主要依據記在 `measures.lead_finding_id`，建立時由規則決定：有新的認定時是新的認定，否則是引用的認定中最嚴重的（CRITICAL > HIGH > MEDIUM > LOW），同嚴重度取最新建立的；RISK 沒有 |
| `governance_notes` | 只新增的內部註記（目前掛在措施，`measure_id`），必填第一則；不動 `measures`；API 與權限仍是 `CreateMeasureNote`、`ListMeasureNotes`、`seller.measure_notes.*`（通報的註記隨 10e 第 3c 刀加入同一張表） |
| `measure_requests` | 賣家對措施的請求：`kind` 是 `APPEAL`（每個措施一次）或 `REMEDIATION`（只限 RISK，可重複）；`PENDING` → `ACCEPTED`｜`REJECTED`｜`CLOSED`，`CLOSED` 帶 `close_cause`：`REVOKED`（措施被撤銷，帶 admin）或 `ENDED`（措施已結束而補正失去標的，系統結案，沒有 admin）；`basis` 由措施帶入（複合外鍵 `(measure_id, basis)` 加 CHECK 保證補正只掛 RISK）；`revision` 是提出時賣家看到的那一版；佐證在 `measure_request_evidence`（與申訴共用） |
| `measure_identities` | 停權措施建立時快照的身分 blind index，措施結束或撤銷就刪 |

**狀態在讀取時推導**（`model.StateOf`，順序：已撤銷、警告、`ENDED`、`SCHEDULED`、`ACTIVE`；不認得的種類 panic）：`ISSUED`（警告）、`SCHEDULED`（預定生效：`starts_at` 在未來，目前只有預告終止）、`ACTIVE`（停權與終止：已開始、`ends_at` 為空或在未來）、`ENDED`（預告期間內因店家關閉而結束的措施 `ends_at` 早於 `starts_at`，也是 `ENDED`）、`REVOKED`。「生效中」的 SQL 定義有 `EffectiveMeasures`（指定帳號）與 `EffectiveMeasuresAll`（全部帳號）兩段，`StateOf` 是同一個定義的 Go 寫法；三者的生效中集合一致由 `TestEffectiveMeasuresPerAccountAllAccountsAndStateOfAgreeOnADataset` 在含邊界的資料集上鎖住。

**前置條件**寫在 `model.Measure`，方法與 API 的 `can_*` 共用（改理由、延長、縮短、撤銷四種的規則在下一段的 `*UnavailableReasons`，`Can*` 是「清單為空」；其餘各種直接是 `Can*` 的判斷）：改理由（未撤銷、警告或預定生效或生效中的措施、沒有申訴、沒有待處理的補正）、延長（生效中、沒有申訴、沒有待處理的補正）、縮短（生效中）、結束（生效中的 RISK、沒有待處理的補正）、撤銷（未撤銷、沒有待裁定的申訴）、重排複審（生效中的 RISK）、裁定申訴（有待裁定的申訴）、賣家申訴（未撤銷、沒申訴過、沒有待處理的請求；已結束措施上待處理的補正不算；已結束的措施也能申訴）、賣家請求補正（RISK、生效中、沒有待處理的請求、不在冷卻）、裁定補正（有待處理的補正、措施生效中）。提出申訴起措施的理由與延長凍結；補正待處理期間同樣凍結加重（理由、延長、結束），縮短、撤銷與接受補正不受影響（解除不被程序擋住）。

**admin 不能操作的原因**（10e 第 3a 刀，`Measure.update_statement_unavailabilities`、`extend_unavailabilities`、`shorten_unavailabilities`、`revoke_unavailabilities`，結束的 `end_unavailabilities` 在目錄 enum 文字守門第 2 刀加入，型別 `repeated MeasureActionUnavailability`，Will 2026-10-17 決定的形狀，`step10e-labels-brief.md` D-L2）：改理由、延長、縮短、撤銷、結束五種各有一個依優先序的原因清單，每一項是 `MeasureActionUnavailability { reason（MeasureActionUnavailableReason，給 client 分支）; message（google.rpc.LocalizedMessage，zh-TW，給使用者看的一句話）}`，兩者都是 OUTPUT_ONLY。`Can*` 就是「清單為空」，方法的前置檢查與 API 的 `can_*` 與原因都只看這一份（`model.Measure` 的 `*UnavailableReasons`）。每個適用的原因都列出，結構性原因在前（例如已撤銷且有申訴的措施，改理由是 `REVOKED`、`NOT_ACTIVE`、`REQUEST_PENDING_FROZEN`），賣家請求造成的凍結排最後；清單為空若且唯若可以做。**文字只有一個來源**：`model.ActionUnavailableText(reason, action)`，API 的 `message` 與方法被拒絕時 `failed_precondition` 的 `LocalizedMessage` 都由它產生，錯誤取清單第一項（`TestActionUnavailableErrorSaysTheFirstReasonText`）；client 直接顯示 `message`，沒有文字時才顯示通用文字「目前不能操作」，不持有原因對照表，也不自己組句子。句子有主詞、不寫 client 的畫面或選單名稱。結束的原因是 `REVOKED`、`NOT_ACTIVE`、依據不是風險類型（終止合作是 `TERMINATION_RECREATE`，依據違規認定的停權是 `VIOLATION_BASIS`，句子「因違規處分停權；違規認定只供查閱，撤銷要對這個措施進行。」）與待處理的解除請求（`REMEDIATION_PENDING`），`End` 的前置檢查取清單第一項，admin 的生效中措施卡顯示第一項；`schedule_review` 與兩種裁定的入口由狀態決定，不輸出原因。

| 值 | 伺服器的句子 | 條件 |
|---|---|---|
| `REVOKED` | 這筆措施已撤銷 | 措施已撤銷（四種動作） |
| `NOT_ACTIVE` | 這筆措施不在生效中 | 改理由：已結束（或已撤銷）；延長、縮短：不在生效中（已結束、已撤銷、還沒開始，警告也算） |
| `KIND_HAS_NO_END_TIME` | 警告沒有結束時間 | 延長、縮短（警告） |
| `TERMINATION_RECREATE` | 終止合作要改期間，請撤銷後重建 | 延長、縮短（未撤銷的終止合作，任何時間狀態；已撤銷的只有 `REVOKED`） |
| `REQUEST_PENDING_FROZEN` | 改理由：賣家提出過申覆，或有待處理的解除請求，給賣家的理由不能再改；延長：賣家提出過申覆，或有待處理的解除請求，結束時間不能再延長 | 賣家曾提出申訴（含已裁定），或有待處理的補正（解除請求）；縮短與撤銷不受影響 |
| `APPEAL_PENDING` | 申覆待裁定，請先裁定 | 撤銷（申訴待裁定，只經 `DecideAppeal`） |

`Measure.store_name`（OUTPUT_ONLY）在單筆與列表都帶，由一次批次查詢填入（與筆數無關）；個資清理之後是空字串。

**終止合作**（條款第 21 條，§14）：終止是措施的一種，不是另一套機制。

**撤銷與連帶撤銷**：撤銷帶 `revoke_findings` 時（伺服器以 `can_revoke_findings` 輸出是否可設：依據是 VIOLATION，而且措施自己建立了至少一筆尚未撤銷的認定；只引用別的措施建立的認定時不可設，回 invalid_argument，RevokeMeasure 與接受申訴相同），撤銷措施自己建立的認定，並以 `FINDING_REVOKED` 撤銷所有引用那些認定的 ENFORCED 措施，待處理的申訴以 `CLOSED` 結案，停權期間只重算一次。撤銷時待處理的補正也以 `CLOSED`（`REVOKED`）結案。

**補正**（條款第 17.4 條）：RISK 是條件型的限制，風險消除或外部要求解除後賣家可以檢具事證請求解除；VIOLATION 是懲罰型，期間本身就是處分，錯誤的決定走申訴或平台撤銷。接受補正＝寫一版 `ends_at` 是現在的新版本（作者是裁定的 admin，理由不變）、重算停權狀態、發 `MeasureEnded`；認定與撤銷原因不動，賣家仍可申訴，賣家與後台的 etag 都變。駁回只讓後台的 etag 加一，並計入冷卻。**冷卻**由請求推導、不存：同一措施上被駁回（申訴與補正合計，`CLOSED` 不計）的請求累計超過兩次後，每再被駁回一次就從那次起算冷卻 7、14、28 天，上限 28 天（小於 RISK 的 30 天複審上限）；`Measure.next_remediation_time`（冷卻中）與 `rejection_cooldown`（下一次若被駁回會開始的冷卻長度，給送出前的警告）由伺服器給，client 不自己算。**補正只在措施生效中才有標的**：措施已結束而仍待處理的補正不能裁定，不擋申訴（`CanAppeal` 不算它，`SubmitAppeal` 在同一個 transaction 先以 `CLOSED`（`ENDED`）結案）。**請求對外的狀態在讀取時推導**（`model.Measure.EffectiveRequest`，與停權狀態同一個做法）：已結束措施上儲存為 `PENDING` 的補正輸出 `CLOSED`／`ENDED`、`decide_time` 是措施的結束時間；儲存值與 `MeasureRequestClosed` 由 measure 自己的結案掃描（§6.2）收斂，寫回的 `decided_at` 就是 `ends_at`，所以收斂前後輸出相同。**契約形狀**：`Measure.requests`（新到舊，帶 `kind`）取代 `appeal` 與 `remediation_requests`；`remediation_unavailabilities`（`reason` 加伺服器組好的 `LocalizedMessage`；優先序：依據不是 RISK、不在生效中、待處理的請求、冷卻；結構性的原因在前，所以第一個是 NOT_RISK／NOT_ACTIVE 就沒有補正入口）與 `can_request_remediation` 來自同一個函式（空 ⇔ 可以請求），client 不自己推導；`ListMeasures` 以 `request_state`＋`request_kind` 篩選有效狀態，`request_state=PENDING` 是申訴與補正合一的收件匣。**時限（內部 SLO）**：待處理的補正 7 個日曆天內裁定，不寫進契約與條款（條款只寫「合理期間」）；後台收件匣（第 3 刀）顯示等候天數並標出超時的列；撤回與「申訴取代待處理補正」不做，觸發條件是第一個要求撤回的實際案例，或上線後補正裁定中位數超過 SLO。

**`request_id`**（AIP-155）：建立措施可帶 UUIDv4 的 `request_id`（範圍管理員、永久保存）；先查重送（輸入相同回原本措施，不同回 `invalid_argument`；所有 id 清單（來源通報、引用的既有認定）都是集合，順序不同的同一組算相同，沒有例外；主要依據由規則決定，不編碼在位置裡），再做有狀態的驗證，最後寫入。

### 3.4 佐證與詢問

- `seller.report_evidence`、`seller.measure_request_evidence`、`seller.case_inquiry_reply_evidence`：`(parent_id, position)`，每次送出最多 5 張（通報的補充也可以附，一筆通報合計最多 55 張），存 media 的 asset ID 與 object key；引用以通報、申訴請求或詢問的 ID 登記，寫入失敗或撞到重複時由收斂 job 釋放沒有列指著的引用。
- `seller.case_inquiries`（詢問，ADR「修訂：案件的詢問」）：掛在案件上，複合外鍵 `(case_id, store_id)`；`kind`（`RECORDS`、`EXPLANATION`）、給賣家的說明（1–2000 字，只寫管理員的轉述，不附通報原文、佐證或通報人）、期限 `due_at`（預設建立後 72 小時，可指定 24 小時到 14 天，建立後不能改）、冪等鍵 `(created_by_admin_id, request_id)`；回覆欄位（管道、內容 1–4000 字、**收到的時間**、記下的時間與管理員）與撤回欄位（時間、管理員、原因 `ADMIN`／`CASE_CLOSED`）在同一列，CHECK 保證兩者互斥；`case_inquiries_one_open` 保證每個案件同時最多一筆還沒結束的。狀態（等賣家回覆、已回覆、已撤回）與逾期在讀取時推導。
- 舊的 `report_cooperation_requests`（`ASSIST_INVESTIGATION`、`TEMPORARY_TAKEDOWN_SELF_CHECK`）已刪：它沒有給賣家的說明與期限，不能誠實地轉成詢問；migration 在有列時失敗並列出通報 ID。
- `seller.authority_orders`（主管機關來函，ADR「修訂：主管機關來函、保存期限與措施的來源來函」）：機關、文號、收文時間 `received_at`、種類（`RESTRICT_PRODUCT`、`DATA_REQUEST`、`SUSPEND_ACCOUNT`、`OTHER`）、摘要、期限 `due_at`、保存期限 `hold_until`、完成（時間、管理員、處理摘要）、撤銷登記（時間、管理員、原因；只在完成之前，與完成互斥，撤銷之後保存解除）、`version`（完成、延長、記下告知與撤銷登記的樂觀鎖）、`(registered_by_admin_id, request_id)` 冪等鍵。`authority_order_targets`（`(order_id, position)`，最多 20 個）：帳號（任何帳號）、選填店家（複合外鍵保證屬於帳號）、商品（最多 50 個）、告知（`NOTIFIED`、`WITHHELD` 必填 `recheck_on`、`MEASURE_IS_NOTICE`；CHECK 保證欄位與告知方式一致）；`UNIQUE NULLS NOT DISTINCT (order_id, account_id, store_id)`。`authority_order_hold_extensions` 與 `authority_target_notice_history` 只新增。`finding_authority_orders` 是認定引用的來函，`measures.source_authority_order_id` 是措施明確連結的來源來函。


### 3.5 停權狀態（讀取時推導）與停權期間

停權狀態**不存**（ADR 2026-10-18-seller-governance-consolidated「考慮過的方案」的「停權狀態」與「決策」的「認定、措施與效果」）：帳號任何一個 stint（含已關閉的）在「現在」有生效中的停權措施（`ENFORCED`、`SUSPENSION`、已開始、`measures.ends_at`（目前值）為 NULL 或晚於現在）就是停權。「現在」是注入的 clock，不是 SQL 的 `now()`。推導只有一個點：`stint.Derive`（指定帳號的查詢 `EffectiveMeasures`，結果放進 `Store.Sanctions`，停權與生效中的措施是同一個來源），能力檢查（`ActingStore`）、讀店家（`Store.Suspended()`、`Store.EffectiveMeasures()`）、開店與 `suspension.Recompute` 都經它；能力是 `CapabilitiesOf(store, flow)`。措施到期不需要任何 job：時間一到，下一次讀取就看到恢復。

排程只負責副作用——**停權期間**（`seller.suspension_periods`）與事件：`suspension.Recompute` 先鎖住帳號全部的 stint、再讀措施，帳號從未停權變成停權時開始一段期間、發 `StoreSuspended`，恢復時結束那一段、發帶同一個期間 ID 的 `StoreUnsuspended`。下游（商品下架、未付款取消、券停用、通知、IM）依這兩個事件處理。

- 期間記在帳號目前的 stint 上；停權期間內 stint 關閉時，期間留在那個 stint 直到措施結束。帳號沒有目前的 stint（所有 stint 都已關閉）而被再次停權（延長已到期的措施）時不開始期間：沒有店面可以下架，開店由推導擋下。
- 持續停權中新增、結束或撤銷其中一個措施（只要還有一個生效中）：同一段期間，不發事件；`StoreSuspended` 不帶停權的種類，要依據的讀 `ListMeasures`。
- 措施到期後、job 還沒結束期間時開了新的 stint：舊 stint 上的期間由之後第一次 `Recompute`（新 stint 再被停權、到期 job、對帳 job）以 `StoreUnsuspended` 結束，新的 stint 開始自己的一段。job 晚到只讓事件晚到，不影響權限。
- 重算的時機：措施的建立、結束、撤銷、改結束時間，到期 job（§6.2），對帳 job（§6.2）。

---


## 4. API 規格

> **§4 型別合規（前端可控 request id 一律 string）**：經 Gateway 暴露給前端的 request body / path 業務 ID 欄位為 **string**（TSID 18-19 位 > 2^53，JSON number 會精度丟失）。涉及：`CreateViolationReport`(`target_seller_id`/`target_product_id`/`target_post_id`)、`CreateFoodSafetyReport`(`target_seller_id`/`target_product_id`)、`SellerSubmitAppeal`/`SellerGetAppeal`/`AdminDecideAppeal`(`violation_id`)、`GetPublicSellerViolations`(`seller_id` path)、`AdminPatchReportStatus`/`AdminAcknowledge`/`AdminGenerateFDAReport`/`AdminMarkFDAReported`/`AdminNotifySeller`(`report_id`)、`AdminCreateViolation`(`seller_id`/`source_seller_report_id`/`source_food_safety_report_id`)、`AdminUpdateViolation`/`AdminRevokeViolation`(`id`)。`reporter_account_id` / appeal 的 `seller_id` / `admin_id` / `created_by_admin_id` 由 Gateway JWT 注入，屬 §4.1 例外①，維持 int64。buyer 模組同模式：gRPC mapper 邊界 ParseInt 成 int64 給 usecase（DB bigint 不變）。

### 4.1 統一回應格式

成功：`{ code: 0, data: {...} }`
失敗：`{ code: <error>, msg: "...", traceId: "..." }`

常見 code：
- `0`：success
- `400`：invalid input（含 phone format / penalty 欄位互斥 / 缺必填）
- `401`：unauthorized
- `403`：forbidden（非 own violation 申訴）
- `404`：not found
- `409`：conflict（duplicate / status transition / already acked）
- `503`：endpoint disabled by feature flag

### 4.2 登入端檢舉

`POST /api/v1/app/violation_reports`（buyer/seller JWT）

```json
// Request
{
  "target_seller_id": 12345,
  "target_product_id": 999,    // optional
  "category": "misleading_info",
  "severity": "medium",
  "description": "商品描述誇大",
  "evidence_asset_ids": ["asset_a", "asset_b"]
}

// Success
{ "code": 0, "data": { "report_id": "<TSID>" } }

// Duplicate (同一日曆日)
{ "code": 4090, "msg": "duplicate report within same calendar day",
  "data": { "report_id": "<既有 TSID>", "duplicate": true } }
```

`GET /api/v1/app/violation_reports/me?limit=20&offset=0`

### 4.3 公開食安通報

`POST /api/v1/public/food_safety_reports`（**無 JWT**，雙層 IP rate limit）

```json
// Request
{
  "target_seller_id": 12345,    // optional
  "target_product_id": 999,     // optional
  "reporter_phone": "0912345678",  // 必填，台灣手機格式
  "reporter_email": "victim@example.com",
  "reporter_name": "王小明",
  "description": "食用後腹瀉，懷疑沙門氏菌"
}
// 註：本 API 不接受佐證附件（evidence_asset_ids）；帶入將被拒（fail-closed，見 §3.2 附註）。
//     需附佐證圖請登入後改走 POST /app/violation_reports（category=food_safety）。

// Success
{ "code": 0, "data": {
    "report_id": "<TSID>",
    "ack_estimated_at": <unix_ms>   // NOW + 4h
}}

// Silent Drop (ISO week unique 觸發 — 仍回 200，避免暴露 phone 已用過)
{ "code": 0, "data": {
    "report_id": "<既有 TSID>",
    "ack_estimated_at": "..."
}}
```

### 4.4 賣家側

Connect `SellerAppService`：`ListMyMeasures`（`state` 篩選）、`GetMyMeasure`、`SubmitAppeal`、`RequestRemediation`（etag 是賣家的 `current_revision`，必填；提出請求不改變 etag，重送回 `failed_precondition`）；`GetMyStore.suspended` 是帳號此刻是否停權。舊的 `/api/v1/app/sellers/me/violations` 在 4B 已刪除。

### 4.5 公開賣家頁

公開的違規列表**刪除**（ADR 2026-10-02）。停權的店家依停權事件下架。

### 4.6 Admin 治理

通報與案件（Connect `SellerGovernanceAdminService`，權限 `seller.reports.*`、`seller.cases.*`；每個通報與案件都帶推導出的 `can_*` 與 etag）：

```
ListReports(state, category, store_id, case_id) / GetReport     通報（清單不帶聯絡方式與佐證；GetReport 帶補充與佐證）
TriageReport          分案：判定平台判定類別，放進這家店、這個類別的進行中案件，沒有時開新案件（開新案件要給嚴重度）；etag 必填
ResolveReport         直接設定結果（已採取措施只能設在案件裡的通報，而且案件要已有作為）；etag 必填
MoveReport            移到另一個類別的案件；通報都移走的案件以 NO_REPORTS 關閉；etag 必填
ListCases / GetCase   案件（帶 decision_options、成員數、導出的措施數與 unseen_supplement_report_count）
DecideCase            決定即結案：採取措施或不採取措施，還沒有結果的成員帶入結果；etag 必填
ChangeCaseSeverity    改嚴重度：必須附原因，寫成案件的註記；etag 必填
RecordFDAReferral     記錄轉報（食安案件，每個案件一筆，重送回 already_recorded；追蹤日期選填，預設轉報日（台北）加 30 天）
WaiveFDAReferral      標記不轉報（附原因；與轉報互斥；重送回 already_waived）
RecordFDAReferralReply 記下主管機關對轉報的回覆（只新增；可同時更新或清掉追蹤日期；request_id 冪等，範圍是管理員）
CreateCaseInquiry     對案件的賣家發詢問（提供紀錄或請說明；期限預設 72 小時；每個案件同時最多一筆還沒結束的；request_id 冪等；案件進入「等賣家」）
ListCaseInquiries     案件的詢問，舊到新
RecordCaseInquiryReply 記下系統外收到的回覆（管道、收到的時間、內容、最多 5 份機敏附件），詢問就結束；重送回 already_recorded
WithdrawCaseInquiry   撤回還沒回覆的詢問；案件結案時還沒結束的詢問由系統撤回
CreateAuthorityOrder  登記主管機關來函（對象任何帳號；保存期限預設收文後 184 天；request_id 冪等，範圍是管理員）
ListAuthorityOrders   來函清單（處理中依期限、其餘依收文時間新到舊；可依指名的帳號篩選）
GetAuthorityOrder     讀來函：對象逐家的涵蓋狀態、告知、延長歷史
CompleteAuthorityOrder 完成來函：處理摘要、每個對象的告知；調取資料可帶提供時間，保存期限延到提供之後 184 天（取較晚）
ExtendAuthorityOrderHold 延長保存期限（只能延長，留前後值與原因）
RecordAuthorityTargetNotice 記下暫不告知的對象後來的告知（依對象的 can_record_notice）
RevokeAuthorityOrder  撤銷登記錯誤的來函（只在完成之前、附原因、etag 必填；保存解除、紀錄保留；同一位管理員重送回 already_revoked）
GetFDAExport          轉報資料：案件、案件裡結果沒有被否定的通報（不含聯絡方式）與店家
CreateCaseNote / ListCaseNotes   案件的內部註記（只新增，不帶 etag）
```

措施與認定（Connect，權限 `seller.measures.*`、`seller.measure_notes.*`；註記存在 `governance_notes`，掛在措施或案件剛好一個）：

```
CreateMeasure         建立措施（violation 依據：新建或引用既有認定；risk 依據：風險類型；request_id 冪等；第一則內部註記必填；可帶 source_case_id 連到案件）
GetMeasure / ListMeasures(account_id, store_id, kind, basis, state, request_state, request_kind, review_due, source_case_id, source_report_id…)
UpdateMeasure         update_mask 只能列 statement、end_time；etag（measures.version）必填；提出申訴後只能縮短 end_time
EndMeasure            結束生效中的 RISK 措施
RevokeMeasure         撤銷（revoke_findings 連帶撤銷，只在 can_revoke_findings 時可設）；申訴待裁定時回 failed_precondition
ScheduleMeasureReview 重排 RISK 的複審期限（必填內部註記）
DecideAppeal          裁定申訴（接受 ⇔ 撤銷，APPEAL_ACCEPTED）
DecideRemediation     裁定補正（接受 ⇒ 措施結束，不是撤銷；不帶 request_id；權限 seller.measures.decide_remediation）
CreateMeasureNote / ListMeasureNotes   內部註記（只新增，不帶 etag）
GetViolation / ListViolations          認定（唯讀）
```

佐證讀取：`GET /api/files/admin/seller/reports/{report_id}/evidence/{position}`、`GET /api/files/admin/seller/measure_requests/{request_id}/evidence/{position}`、`GET /api/files/admin/seller/case_inquiries/{inquiry_id}/evidence/{position}`（權限 `seller.case_inquiry_evidence.get`），每次讀取先寫 `evidence_accesses`。管理員停權就是建立 RISK 停權措施（§13），沒有獨立的停權方法。

---

## 5. 跨服務時序圖

### 5.1 admin 建立措施

在同一個 transaction：鎖帳號全部的 stint（已關閉的 stint 拒絕）→ 鎖來源通報列並檢查 → 升級規則的計數 → 新增認定（VIOLATION 依據且未引用既有認定時）→ 新增措施、第 1 版、依據、第一則註記 → 停權措施快照身分並排入排在 `ends_at` 的到期 job → `suspension.Recompute`（帳號從未停權變成停權時開始停權期間、發 `StoreSuspended`）→ 發 `MeasureCreated`。任何一步失敗整筆回滾，沒有 commit 後的補送。

### 5.2 食安公開通報（含 silent drop）

```
公開使用者              Gateway          單體 seller 模組         Postgres
  │                       │                   │                      │
  │ POST /public/food_safety_reports          │                      │
  ├──────────────────────▶│ 雙層 IP rate limit                       │
  │                       │ (10min/5 burst + 1h/10 sustained)        │
  │                       │ 通過 ───────────▶│ NormalizePhone        │
  │                       │                   │ INSERT food_safety_reports
  │                       │                   │ ◀──── 23505 (partial unique)   ──── │
  │                       │                   │ Application catch ErrDuplicate         │
  │                       │                   │ GetExistingByPhoneAndSellerThisWeek    │
  │                       │ ◀ 200 OK ────────│ 回 既有 report_id（silent drop）         │
  │ ◀────────────────────│                   │                                          │
  │
  │ 後台怎麼處理：見 §5.3
```

### 5.3 食安事件的後台處理（取代 push）

食安事件不發 push（D9：避免半夜的誤報騷擾）。「商家通報」頁（§8）每 30 秒自動更新，營運人員靠清單與期限看見新的通報；單人營運，沒有輪值，期限只顯示、不強制（決策簡報 `plans/2026-09-29-a3-buyer-seller/step10e-reports-brief.md` R3）。

```
營運人員              admin UI（/admin/violation-reports）              單體（seller 模組）
  │ 待分案分頁：檢舉人選了食安的通報有分案期限（建立後 4 小時，Report.triage_due_time）
  │   「剩 {時長}」→ 剩下不到整段期限的四分之一（1 小時）轉 warning → 逾期「已逾期 {時長}」danger
  │   絕對時間在 title；時長無條件捨去；每 30 秒重讀，相對時間跟著走
  │ 通報詳情 → 分案（平台判定類別、嚴重度）── TriageReport ──────▶ 加入這家店的食安案件，沒有就開一個
  │ 案件詳情 → 建立措施（警告、停權、風險措施、終止合作）──────▶ CreateMeasure(source_case_id)：記為案件導出的措施
  │ 案件詳情 → 結案（採取措施／不採取措施）── DecideCase ───────▶ 還沒有結果的通報帶入結果，每筆發 ReportResolved
  │ 食安案件（進行中或已結案，有通報）→ 下載轉報資料（.md，client 組）
  │   → 系統外轉報（1919、線上表單、傳真衛生局）→ 記錄已轉報主管機關（追蹤日期，預設 30 天後）── RecordFDAReferral
  │   → 追蹤日期過了而還沒有主管機關的回覆：日期用 warning → 聯絡主管機關 → 記下主管機關的回覆 ── RecordFDAReferralReply
  │   → 決定不轉報（例如主管機關已經介入）→ 不轉報（原因）── WaiveFDAReferral（與記錄轉報互斥）
  │ 案件詳情 → 詢問賣家（提供紀錄／請說明，期限預設 3 天）── CreateCaseInquiry：案件進入「等賣家」
  │   → 系統外收到賣家的回覆 → 記下回覆（管道、收到的時間、內容、最多 5 份機敏附件）── RecordCaseInquiryReply
  │   → 不必再回覆 → 撤回 ── WithdrawCaseInquiry；結案時由系統撤回還沒結束的詢問
  │ 主管機關來函（公文）→ /admin/authority-orders → 登記來函 ── CreateAuthorityOrder：指名的帳號與店家進入保存期限
  │   → 完成（處理摘要、每個對象的告知）／延長保存／記下告知／從來函建立措施（CreateMeasure(source_authority_order_id)）
```

所有類別的期限與待辦頁在 3d；3c-a 只有既有的食安 4 小時。

---

## 6. 排程與 job（River）

### 6.1 `seller.erase_report_contacts`（每小時，啟動時先跑一次）

把到期、還沒清的匿名通報的 `reporter_phone`、`reporter_email`、`reporter_name` 清成 NULL，記下 `contact_erased_at`；有結果的到期是 `greatest(建立 + 90 天, 結果 + 30 天)`；不論有沒有結果，建立滿 365 天一律清（絕對上限）；分批，每批一個 transaction。不做 hash。描述與補充是自由文字，不在範圍內；匿名通報沒有佐證，所以沒有圖片要刪。

**絕對上限（Will 2026-10-18）**：分案的期限只顯示、不強制，案件也沒有總時長上限（同一份簡報的 R3），所以通報可以長期沒有結果；為了讓保留有期限（GDPR 5(1)(e) 儲存限制），匿名聯絡方式建立滿 365 天就清，不論有沒有結果、案件有沒有決定（邊界是嚴格大於 365 天）。清理 job 回報其中**只靠這條上限**清掉的筆數（有結果而且結果規則也到期的不算），大於 0 時記一筆 warn log——沒有結果卻拖滿一年，代表處理流程卡住了，這是觀測點。現在的隱私權政策沒有寫匿名檢舉人聯絡方式的保存天數，保留規則改了不必改法律文字。

### 6.2 措施的到期與對帳

- `seller.measure_expiry`：建立停權措施、改結束時間時，在同一個 transaction 排入排在 `ends_at` 的 job（以措施與到期時間為唯一鍵）；執行時以 `max(排入時的 ends_at, 現在)` 為判斷時間重算停權期間。舊的到期時間的 job 照樣執行：措施後來延後時判斷時間仍是舊的 `ends_at`，新的結束時間更晚，措施繼續生效。權限不靠它（讀取時推導），它讓期間與事件準時追上來。
- `seller.reconcile_suspensions`（每小時，啟動時先跑一次）：修補停權期間與措施不一致的帳號。只掃描有進行中期間（`AccountsWithOpenPeriod`）或此刻有生效中停權措施的帳號（`EffectiveMeasuresAll`，讀取量隨生效中的措施成長，不讀歷史），各自以 `Recompute` 重算（先鎖再讀）。
- `seller.measure_close_ended_requests`（measure 自己的 job，每 5 分鐘，啟動時先跑一次）：只掃已結束（`EffectiveMeasures` 的結果之外）而仍有儲存為 `PENDING` 補正的措施，每個帳號在自己的 transaction 內先鎖再重讀，把補正以 `CLOSED`（`ENDED`、`decided_at = ends_at`）結案、發 `MeasureRequestClosed`、後台的 etag 加一；停權的到期 job 與對帳不管請求。這只決定儲存值與通知的時效，對外的狀態由讀取時推導，job 遺失時正確性不受影響。

---

## 7. 通知

治理的狀態改變在寫入的 transaction 內發模組事件；A8 的 notification 訂閱、決定內容並保存送達紀錄。在那之前賣家、檢舉人與 admin 都以拉取的方式看到結果。

| 事件 | 時機 | 收件人（A8） |
|---|---|---|
| `ReportReceived` | 每次送出：新的檢舉、補充（帶 `SupplementID`）、補充已達上限而沒有存下的匿名送出（`Discarded`）；匿名的帶這次送出的 email | 檢舉人（登入的站內通知；匿名的確認信寄到這次送出留的 email） |
| `ReportResolved` | 通報有了結果，每筆一次：個別設定，或案件結案時帶入 | 登入的檢舉人 |
| `MeasureCreated` | 建立措施 | 賣家 |
| `MeasureRevised` | 措施賣家看得到的內容（理由、結束時間）實際改變 | 賣家 |
| `MeasureEnded` | admin 結束 RISK 措施或接受補正（自然到期不發） | 賣家 |
| `MeasureRevoked` | admin 撤銷、接受申訴或連帶撤銷 | 賣家 |
| `MeasureRequestSubmitted` | 賣家提出申訴或補正（`Kind`） | admin |
| `MeasureRequestDecided` | admin 裁定申訴或補正（`Kind`、`Outcome`；接受申訴同時發 `MeasureRevoked`，接受補正同時發 `MeasureEnded`） | 賣家 |
| `MeasureRequestClosed` | 待處理的請求由平台結案（`Cause`：`REVOKED` 措施被撤銷，`ENDED` 措施已結束而補正失去標的） | 賣家 |
| `StoreSuspended`／`StoreUnsuspended` | 帳號從未停權變成停權、恢復（含到期；以停權期間為單位） | 賣家與下游 |

事件不帶原檢舉人的聯絡方式；`ReportReceived` 只帶這次送出留的 email（匿名），給 A8 在同一個 transaction 排入確認信的 job，不落 seller 的表。

---


## 8. Admin UI（hawkr-admin / Next.js 16）

Admin UI 頁面（全部走單體的 Connect；舊的兩頁與 gateway 的 7 條 admin 通報 REST 已刪除）：

| 路徑 | 對應功能 |
|---|---|
| `/admin/violation-reports` | 「商家通報」：待分案｜案件｜所有通報三個分頁（類別篩選，案件分頁另有狀態篩選），keyset 分頁，每 30 秒自動更新；待分案分頁有分案期限（日曆時間，剩 {時長}／已逾期 {時長}） |
| `/admin/violation-reports/[id]` | 通報詳情：通報內容、補充（匿名的補充標示比對方式、丟掉的次數）、檢舉人、佐證、處理；分案、設定結果、移到其他案件三個對話框，查看案件 |
| `/admin/violation-reports/cases/[id]` | 案件詳情：案件、案件裡的通報（各有設定結果與移到其他案件）、移動紀錄、轉報主管機關（食安）、導出的措施（`ListMeasures(source_case_id)`）、內部註記；結案、改嚴重度、建立措施、建立風險措施、終止合作、下載轉報資料（.md）、記錄已轉報主管機關 |
| `/admin/violation-reports/cases/[id]`（3c-b） | 案件詳情加：「詢問賣家」區塊（`ListCaseInquiries`，舊到新，每筆一張卡：種類・狀態、建立、期限、給賣家的說明、回覆〔管道、收到的時間、記下、內容、機敏附件的連結〕或撤回〔時間、原因〕）與頁首的「詢問賣家」「等賣家」標記；轉報區塊加追蹤日期（伺服器的 `follow_up_overdue` 為真時用 warning）、主管機關的回覆與不轉報；對話框：詢問賣家、記下賣家的回覆（附件在送出時才上傳成機敏檔，`isSecure` 的 secure_upload，失敗後重送不重傳；份數上限取自 `CaseInquiry.reply_evidence_max`，超過在上傳之前擋下；上傳失敗顯示網路失敗句，不顯示內部錯誤）、撤回詢問、不轉報、記下主管機關的回覆；結案對話框在等賣家時提示會撤回詢問；商家通報的案件分頁狀態篩選加「等賣家」。轉報資料（.md）開頭加「以平台名義提出」一句 |
| `/admin/authority-orders` | 「主管機關來函」：處理中（期限舊到新）｜已完成｜已撤銷登記（收文時間新到舊）三個分頁，keyset 分頁，每 30 秒自動更新；欄位機關、文號、種類、指名的店家，處理中有期限（開始時間是收文時間）與收文時間，已完成有完成時間與保存到 |
| `/admin/authority-orders/new` | 登記來函（表單頁）：機關、文號、收文時間（預設現在）、種類、期限（不填就是伺服器依種類的預設，說明句寫出預設值）、內容摘要、指名的對象（多列：賣家帳號 ID、店家〔選填，讀這個帳號營運中與已關閉的店家〕、商品 ID）；`request_id` 在頁面開啟時產生，成功導向詳情頁 |
| `/admin/authority-orders/[id]` | 來函詳情：來函、指名的對象（逐家的涵蓋狀態，沒有店家資料時明示）、處理（完成、每個對象的告知）、保存（保存到、延長歷史）、導出的措施（`ListMeasures(source_authority_order_id)`）；完成、延長保存、記下告知（暫不告知的後續，依對象的 `can_record_notice`）、撤銷登記（`can_revoke`）四個對話框；建立措施／建立風險措施／終止合作三個入口帶 `source_authority_order_id`（違規依據另在新認定帶來源來函），每家店能不能建立由伺服器給（`AuthorityTargetStore.can_create_measure`、`can_create_termination`），可以的店有多家時先選（不預設第一家）。沒有內部註記 |
| `/admin/merchants/[id]` | 店家詳情的治理區：生效中的措施（`GetStore.active_measures`）、該帳號的措施歷史（`ListMeasures(account_id)`，顯示 `state`）、建立 RISK 停權措施（風險類型、給賣家的理由、選填結束時間、必填複審期限與第一則內部註記）、結束措施（依 `can_end`）。建立對話框開啟時產生 `request_id`，同一對話框重送沿用 |

關鍵 UX：
- 按鈕只看伺服器的 `can_*`：不能操作時隱藏、不顯示原因；選項也來自伺服器（設定結果只列 `available_outcomes`，結案只列 `decision_options`，終止合作的事由來自 `GetStore`），前端不複製前置條件。
- 分案與移動的對話框開啟時讀這家店進行中的案件，說明會加入哪個案件或開新案件（沒有進行中的案件時要選嚴重度）；結案的對話框開啟時重讀案件（補充不改案件的版本，有新的補充時提醒結案前先看）；設定結果的對話框並排列出這筆通報當來源的措施，提醒設定結果不會撤銷措施（這是伺服器的關聯資料，不是推導規則）。
- 版本衝突（`aborted`、`VERSION_CONFLICT`）：重讀、對話框留在原地並保留輸入、顯示衝突句；其他失敗顯示伺服器的句子，網路層失敗用該操作的固定句。成功時不另外提示，只有「記錄轉報已記過」與「移動讓原案件關閉」顯示橫幅。
- 從案件開三種建立措施的對話框都帶 `source_case_id`（頂端說明這個措施會記為案件導出的措施）；建立措施（違規依據）預填案件的類別與案件裡最早的 20 筆可以當來源的通報。
- 給賣家的理由（`statement`）必填，欄位旁 link 到 `seller-governance-shared.md` 的撰寫指引；理由只給受影響的賣家看，不公開。措施表單的欄位規則與 §3.3、§4.6 一致：複審期限預設建立後 7 天、上限 30 天。
- 轉報資料（.md）由 client 組：只列結果不是未違規、內容不足或濫報的通報，不放檢舉人的聯絡方式，佐證只寫份數、另附。
- 食安事件結案後的 postmortem 不放在 UI，寫在 runbook（`docs/runbooks/food-safety-incident-runbook.md`）。

側欄「商家通報」在「賣家 / 商品」分組、「賣家管理 / 審核」之後（icon `ri-shield-cross-line`）；「主管機關來函」在「商家通報」之後（icon `ri-government-line`）。

3c-b 的畫面規則（設計書 `plans/2026-09-29-a3-buyer-seller/step10e-3cb-design.md` §5，文案 Will 2026-10-19 核准）：可用性與原因都來自伺服器（`can_*`、`awaiting_seller`、`CaseInquiry.can_*`、`AuthorityOrder.can_*`、錯誤訊息），前端不推導；欄位的長度與範圍由伺服器判斷並回已核准的句子；詢問回覆的「收到的時間」預設現在且精確到秒（整分的預設會早於同一分鐘內剛建立的詢問）；附件是機敏檔，連結在新分頁開啟（每次讀取伺服器都記錄）。

措施與認定的畫面（建立措施、複審收件匣、申訴裁定與註記）以 §4.6 的方法為準。

---

## 9. Runbook 與文件

相關 runbook 與 SQL SOP：

| 檔案 | 用途 |
|---|---|
| `docs/runbooks/food-safety-incident-runbook.md` | 食安主 runbook（Emergency Cheat Sheet + admin UI 流程 + 沒有 SQL 後備 + 轉報主管機關 decision tree + 單人營運與有第二位營運人員時的安排 + Postmortem + 逾期與升級） |
| `docs/runbooks/seller-violation-management-runbook.md` | 一般通報處理（分案、案件、移到其他案件、措施決策、申訴與補正；單人營運） |
| `docs/runbooks/seller-governance-shared.md` | 共用內容（措施種類與依據選用 / 給賣家的理由撰寫指引 / 申訴決議標準 / 升級規則） |
| `docs/postmortems/food-safety/_template.md` | 8 sections postmortem 模板 |

措施與認定沒有 SQL 後備流程：直接改表會繞過推導點、事件與稽核，也會讓停權期間與措施不一致；寫入只經 admin 的 Connect 方法（§4.6）。

---

## 10. 部署與上線

### 10.1 Migration

治理的表（通報、認定、措施與其版本、註記、請求、佐證、停權期間）由 seller 模組的 Atlas migration 建立（`internal/modules/seller/db/migrations`）。

### 10.2 匿名入口

匿名通報的 IP 限流與是否開放在步驟 8 的入口決定（舊 ADR 決策六）。

---


## 12. Related Documents

- Runbook 三份：`docs/runbooks/{food-safety-incident-runbook,seller-violation-management-runbook,seller-governance-shared}.md`
- Postmortem 模板：`docs/postmortems/food-safety/_template.md`
- 31 號（B-135 內容檢舉）：互補 — UGC vs 商家實體治理
- 46 號（賣家能力權限層）：停權的 Gateway enforcement、能力層、收斂點 backstop、業務碼分流
- 36 號（MQ & Event Catalog）：`seller.suspended` / `seller.unsuspended` 事件條目與 consumer 清單
- 02 / 03 / 07 / 12 / 14 號：停權連動 saga 各服務落地（商品下架 / 未付款取消 / 券停用 / 通知 / IM）

---

## 13. RISK 措施

管理員在不服勸導或有風險時建立**依據風險類型（RISK）的停權措施**，沒有固定結束時間，必須有複審期限（`review_by`，預設 7 天、上限 30 天）；風險排除時 `EndMeasure`，需要延後複審時 `ScheduleMeasureReview`（必填內部註記），到期未處理的出現在複審收件匣（`ListMeasures` 的複審到期篩選，不需要 job）。給賣家的理由必填（DSA Art.17），可以申訴（條款第 18.4 條）。風險類型（`RiskGround`）是條款第 17.1、17.5 條的類型。

- 與違規停權的區別只剩依據：RISK 沒有認定、沒有固定結束時間；兩種停權共用停權期間、事件、能力白名單（46 號 §3）與申訴；只有 RISK 提供補正（§3.3），RISK 的 `statement_to_seller` 要寫出補正方式（條款第 4.3 條）。
- 停權狀態的推導、期間與事件見 §3.5，連動 saga 與停權期間的售後／出貨邊界由 `StoreSuspended`／`StoreUnsuspended` 驅動。
- legacy 邊界：生效中的措施有 RISK 依據對 9603／`suspended`，只有認定依據的停權對 `SUSPENDED_GOVERNANCE`。
- 舊的管理員停權列（含已解除）沒有可誠實對應的風險類型：migration 遇到任何一列就整個失敗並列出 ID，須先人工處理再重跑。

## 14. 終止合作（TERMINATION，條款第 21 條）

ADR 2026-10-18-seller-governance-consolidated「決策」的「終止合作」；決策簡報 `plans/2026-09-29-a3-buyer-seller/step10e-2c3-brief.md`。

- **模型**：`kind = TERMINATION`，一定有事由（`NOTICE` 第 21.2 條預告，`MATERIAL_BREACH` 第 21.3 條第 1 款，`CEASED_BUSINESS`、`DISSOLUTION`、`BANKRUPTCY`、`LICENSE_REVOKED`、`FOOD_SAFETY_INCIDENT`、`FRAUD`、`PLATFORM_SECURITY`、`OTHER_SERIOUS`，以及 `LINKED_ACCOUNT`），永遠不是 RISK。`NOTICE` 與 `LINKED_ACCOUNT` 依據契約條款（`CONTRACT`），`MATERIAL_BREACH` 依據認定（至少一筆已存在 7 天以上、未撤銷的認定），其他事由有認定是 `VIOLATION`、沒有是 `CONTRACT`。沒有結束時間、複審期限、補正與 `EndMeasure`；只能改理由、撤銷、申訴。
- **生效**：`NOTICE` 建立後 30 日（720 小時）生效，期間是 `SCHEDULED`，店家照常營業、賣家可申訴；其他事由立即生效，店家暫停營業（停權的效果）。生效中的終止在沒有未結訂單時由 job `seller.termination_effect` 關店（`closed_by = TERMINATION`），有未結訂單就等（`order.completed` 事件加速、每小時 `seller.reconcile_terminations` 保底）。
- **申覆期間**：生效後 184 天（4416 小時）；`Measure.appeal_deadline_time`、`appeal_unavailabilities`（`reason` 為 `REVOKED`、`ALREADY_APPEALED`、`APPEAL_WINDOW_ENDED`、`REQUEST_PENDING`，加伺服器組好的 `LocalizedMessage`）由伺服器給。
- **關店可逆**：恢復營業 ⇔ 造成關店的那一筆措施（目前關店紀錄的 `closed_by_measure_id`）已撤銷（之前或同一個 transaction）∧ 撤銷後帳號上沒有 ACTIVE 的終止（SCHEDULED 不算）∧ 還沒到保留期；符合時撤銷或接受申訴在同一個 transaction 恢復同一個店家，否則只撤銷措施、店家繼續關著；保留期滿的個資清理才釋放圖片並發 `StoreRetired`（relay 轉成 `seller.deleted`）。保留期：`TERMINATION` 關閉的是 `greatest(關閉後 6 個月, 帳號上未撤銷終止的最晚申覆期限)`（已撤銷的終止不延長），其他關閉後 6 個月（184 天，ADR 2026-10-02 的 2026-10-19 修訂）；帳號上未撤銷終止有待處理申覆時暫停（legal hold）。期滿後撤銷只撤銷紀錄與禁止入駐。
- **第 21.2 條**隨任何關店結束（賣家自己、終止、刪帳），不取身分快照，之後可重新申請；**第 21.3 條**永不結束、取身分快照（保留到撤銷），禁止同帳號開店與同一身分證號或統編送 KYC。
- **關聯帳號**：措施詳情列出同身分的其他帳號，管理員另建 `LINKED_ACCOUNT` 終止；來源撤銷時同一個 transaction 連帶撤銷。
- **Runbook**：`runbooks/seller-violation-management-runbook.md` 的§4.1「終止合作」。

