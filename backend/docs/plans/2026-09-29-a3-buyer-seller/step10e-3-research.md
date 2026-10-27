# 賣家治理後台工作流程：從零對照業界基準（10e 第 3 刀簡報的上層檢查）

日期 2026-10-16。Worktree `hawkr-10e` @ `abb690858`（路徑相對於該 worktree）。唯讀。
標記：**[VERIFIED]** 這次已從來源讀到；**[SEARCH]** 只讀到搜尋摘要或文件索引，沒讀到內文；**[INFERENCE]** 推論；**[UNVERIFIED]** 沒查到。

---

## 0. 結論

1. **簡報的 D2 說「合併成一個清單被 REAL 擋下」，這個判定是錯的。** CARRYOVER:88 否決的是「兩張表各自 limit/offset，再到應用層合併」；它沒有否決「一張工作項目表、一個 keyset」。簡報自己也寫了「要合併就得新增跨資源的佇列表」。所以真正擋住的是「沒有這張表」，屬於 SELF-IMPOSED（改動大小、工作量）。
2. 業界的做法是把**領域資源**（通報、決定、申訴）和**工作層**（要有人處理的事項：佇列、SLA、指派、狀態、稽核）分開。我們只有領域資源。每個資源各自長出一個收件匣、一種排序、一個寫死的時限，總共有 5 個 seller 模組內的人工待辦來源：通報、措施請求、RISK 複審、食品業者登錄、KYC。
3. SLA 散在常數、proto 註解和 4 份 runbook 裡，而且**互相矛盾**（見 §3）。
4. 生命週期的拆法（通報 → 認定 → 措施（帶理由與版本）→ 請求）和 DSA 第 16、17、20 條同形，這部分是對的。結構上的缺口有四個：通報對認定只能 1:1、沒有案件層級的時間軸、通報不能改類別、要求賣家配合之後沒有「等賣家回覆」的狀態。
5. 建議在第 3 刀之前插一刀 **3-0「治理工作層」**，3a、3c 的 D2、D3、D4 改建在上面。指派、四眼、QA、升級這幾項只先留欄位或資料，等觸發條件成立再做，因為缺的是第二位人員，屬於 REAL。

---

## 1. 工作項目／案件層

### 業界基準
- **Zendesk**：一種實體（ticket），SLA 政策套用在「符合條件的 ticket」上。[VERIFIED] https://support.zendesk.com/hc/en-us/articles/4408829459866 原文："Each SLA policy consists of conditions that a ticket must meet and targets that measure metrics you define"。派工分兩種：push（指派給 agent）與 pull（agent 自己挑）；"Round robin ticket management ensures tickets are distributed evenly among agents"；也有 skills-based routing。[VERIFIED] https://support.zendesk.com/hc/en-us/articles/4408831658650
- **Jira Service Management**：每種 request type 對應一種 work type，所有 request 都是同一個空間裡的 work item，佇列與 SLA 套用在 work item 上。[SEARCH] Atlassian 的 "What's the difference between request types and work types?"、"About requests and work items"（https://support.atlassian.com/jira-service-management-cloud/docs/what-are-issues-and-requests/）。摘要寫的是 "One request type can only be connected to one work type"、一種 work type 可以服務多種 request type。內文沒讀到。佇列依 SLA 排序：[VERIFIED]（見 `step10e-3-baseline-check.md` §2）。
  - 所以簡報 D2 那句「兩者的佇列都是同一種資源的多個 view，不會把兩種資源混成一個清單」是**誤讀**。JSM 的佇列本來就混了 incident、service request、change 等不同類型，可以混，是因為它們底下是同一個 work item 實體。這正好是我們缺的那一層。
- **T&S 工具**：
  - Checkstep 的 glossary 把 Case 定義為 "A compiled record of all evidence related to a piece of content"，把 Appeal 定義為 "A user's request to reverse a moderation decision"。[VERIFIED] https://training.checkstep.com/help/glossary
  - Checkstep headless API：社群通報會掛到已存在的 content case 上（"A community report sent for content that Checkstep already has is linked to it"）；statement of reasons 是 case 的一個狀態檢視，階段是 no-incident → enforced → appealed → appeal-ruled。[VERIFIED] https://docs.checkstep.com/headless/
  - Cinder 的公開資料描述「case and investigation management、review queues、policy enforcement 在同一個平台」。[SEARCH] 只有新聞與公司介紹，沒有產品文件。
  - Tremau Nima、ActiveFence、Hive：[UNVERIFIED] 沒找到公開的產品文件。
- **TSPA**：申訴應該直接連到原本的違規通知（"By linking the appeal request directly to a specific violation notification, most of the critical data to begin an appeal can be sourced automatically"）。[VERIFIED] https://www.tspa.org/curriculum/ts-fundamentals/content-moderation-and-operations/user-appeals/

基準可以整理成兩層：(1) **案件／調查**把同一個對象的多筆通報、證據、註記、決定聚在一起；(2) **工作項目／佇列**是要有人做的那一步，帶 SLA、指派與狀態，所有種類共用同一個引擎。

### 我們的現況
- 通報：`ListReports` 固定新到舊，只能篩狀態、類別、店家（governance.proto:23、:226-236）。
- 措施：`ListMeasures` 有 review_due 與 request_state 兩種伺服器排序（governance.proto:72-74、:408-415）。
- 食品業者登錄：有 `FoodRegistrationInbox`（admin seller.proto:106-110、:769-770）。
- KYC：`ListKYCApplications`，依送出時間排序（admin seller.proto:39-41）。
- 四個資源各自一份 keyset。人工待辦沒有一張共同的表。唯一能「跨資源」的先例是 `evidence_accesses` 的多型外鍵，用 `num_nonnulls(report_id, request_id) = 1` 保證剛好指向一種（schema.sql:577-587）。這可以直接當工作項目表的形狀。
- 通報到認定是 1:1：`Violation.source_report_id` 只有一個（measure.proto:28-29、governance.proto:374-375）。通報這邊沒有欄位指出它導出了哪個措施。

### 為什麼現有機制存在
公司時期是兩張通報表，在應用層合併分頁，結果漏件（CARRYOVER:57、:88、:303）。8g 改成一個資源一份 keyset 加 Get，修好了漏件，但當時沒有收件匣的消費者，所以沒有討論工作層。

### 選項
| 選項 | 取捨 |
|---|---|
| (a) 一個資源一個收件匣（簡報的做法） | 每個資源各自加 inbox、due、排序，D2、D3 各做一次，食品業者登錄與 KYC 之後還要再做。管理員要巡 4 到 5 頁，沒有一個地方回答「現在最急的是哪一件」 |
| (b) **seller 模組內一張 `review_tasks` 表**：多型主體（report_id、measure_request_id、measure_id（複審）、food_registration_id、kyc_application_id，加 `num_nonnulls = 1`）、kind、state（OPEN／CLOSED）、opened_at、due_at（由 §3 的政策算）、closed_at、closed_by、assignee_admin_id（可為 NULL）。由領域指令在**同一個 transaction** 開啟與關閉。一支 `ListReviewTasks`：伺服器定義的佇列，keyset 是 `(due_at NULLS LAST, opened_at, id)` | 一個佇列、一套 SLA、一份 keyset 測試；之後要做指派、升級、QA、處理時間的中位數，都直接用這張表。代價是一張表、一個 model 函式（開啟與關閉的不變式）、一支 RPC，加上資料集測試鎖住「resource 待處理 ⇔ task 是 OPEN」 |
| (c) (b) 再加**調查案件**（`cases`：同一家店、同一件事，掛 1 到 n 筆通報，註記與時間軸掛在案件上，認定從案件建立） | 最接近 Checkstep 與 Zendesk 合併 ticket 的做法。食安事件常常是多位買家通報同一批貨，案件能把它們聚在一起。改動最大，要改 `source_report_id` 的語意 |

### 約束帳
| 約束 | 判定 | 來源 |
|---|---|---|
| 不能跨表分頁合併 | 只否決應用層合併；對 (b) 不成立 | CARRYOVER:88 |
| 需要新表 | SELF-IMPOSED（工作量） | 簡報 D2 約束帳 |
| 內容審核（social）與發票申訴（invoice-service）不在單體 | **REAL**：它們在邊界外，要等各自的搬遷步驟 | 簡報 K12；`routes_moderation.go` |
| 營運量未知 | REAL（缺資料），但只影響 (c) 的自動歸案規則，不影響 (b) | 系統從未上線 |

### 建議
做 (b)，放在 3-0。(c) 先只做最小的那部分：讓認定能引用**多筆**來源通報（`finding_reports` 關聯表，取代單一的 `source_report_id`），並在通報上輸出導出的措施。完整的案件聚合（自動歸案、合併）留到觸發條件成立：上線後第一次出現同一家店、同一事件有 3 筆以上通報。

終局：社群（social）模組搬進單體時，`review_tasks` 從 seller 模組提升成平台層的工作佇列，內容審核與發票申訴也接進來。

---

## 2. 指派、四眼、升級、QA 抽樣

### 業界基準
- **指派**：Zendesk 有 push 與 pull、round robin、skills routing。[VERIFIED] 4408831658650
- **申訴由不同的人複審**：Meta 的流程是，第一位複審者如果認為原決定不該成立，內容會再送給**另一位**複審者決定（"the content will go to another reviewer"）。[SEARCH] https://transparency.meta.com/policies/improving/appealed-content-metric/
- **DSA 第 20(6) 條**：申訴的決定 "are taken under the supervision of appropriately qualified staff, and not solely on the basis of automated means"。[VERIFIED] springlex Art.20。條文**沒有**要求由不同的人裁定。
- **TSPA**：
  - 申訴與原審共用同一批人會造成問題（"if both moderation and appeals use the same reviewers and resources to make decisions"）。[VERIFIED] user-appeals 章
  - QA 的常見做法是抽樣重審（"take regular samples of decisions being made and have them reviewed again"）；樣本可以加權；也有 reverse sampling（把已知答案的案例送進一般流程，看會不會被正確處理）。[VERIFIED] https://www.tspa.org/curriculum/ts-fundamentals/content-moderation-and-operations/content-moderation-quality-assurance/
  - 升級：高風險、高優先的升級案由專門團隊處理。[VERIFIED] setting-up-content-moderation-teams 章

### 我們的現況
- 整個 schema 沒有 assignee 或 claim 欄位。
- `measure/appeal.go` 不檢查裁定者是不是建立措施的人：`rg -n "CreatedBy" internal/measure/appeal.go` 沒有命中。
- 權限只到「能不能呼叫這個方法」（`httpapi/adminauth/adminauth.go:87-112`）。
- 升級只寫在 runbook：`seller-governance-shared.md:42`「不確定 → 升級給主管」，`food-safety-incident-runbook.md:544-551` 是人工打電話。
- runbook 預設的是一個多人組織：值班、主管、Discord 上 pin 的本週 owner（`seller-violation-management-runbook.md:36`、`food-safety-incident-runbook.md:70-76`）。這是公司時期**沿用下來的前提**，現在不成立。
- 稽核：KYC 有 reveals 與 accesses，佐證有 `evidence_accesses`，措施有 revisions；通報的狀態轉換只存在欄位上（acknowledged_at 等），沒有事件紀錄。

### 判定與觸發條件
| 項目 | 現在做什麼 | 判定 | 觸發條件 |
|---|---|---|---|
| 指派／認領 | `review_tasks.assignee_admin_id` 先放（可為 NULL），沒有 UI | 延後 UI：REAL（只有一位營運人員，沒有第二方） | 有第 2 位具裁定權限的管理員 |
| 四眼（申訴不由原建立者裁定；接受申訴撤銷終止合作時要第二人） | 不擋。裁定時把「裁定者 = 建立者」記在 task 關閉紀錄上，之後可以查 | REAL（沒有第二位人員）；條文不要求 | 有第 2 位管理員時，`CanDecideAppeal` 加上排除建立者，OWNER 可以覆寫並留紀錄 |
| 升級 | task 加 `priority`；升級 = 提高 priority 並留註記 | 延後：REAL（沒有主管這個角色） | 出現主管角色 |
| QA 抽樣 | 不做；`review_tasks` 的關閉紀錄就是之後的抽樣母體 | 延後：REAL（缺第二位重審者）。資料現在就存在 | 有第 2 位管理員，或每週決定超過約 50 件 |
| runbook 的多人流程 | 改寫成單人營運的版本，標明哪些段落等觸發條件成立才適用 | 沿用的前提已經失效 | 本刀 |

---

## 3. SLA

### 業界基準
- **Zendesk**：
  - 指標有 first reply、next reply、requester wait、agent work、periodic update。
  - 每個 priority 可以選 "calendar hours or business hours"。
  - agent work time 在 On-hold 或 Pending 時暫停（"pauses when the ticket has an On-hold or Pending status"）。
  - 政策有先後，"most restrictive policies at the top"。
  - API 輸出 `policy_metrics[].breach_at`。
  - [VERIFIED] 4408829459866、`step10e-3-baseline-check.md` §1
- **JSM**：SLA 有 start、pause、finish 三種條件（"Pause counting time when … waiting for a customer to respond"）、calendars、goals 依 JQL 設定；API 輸出 `breachTime`、`paused`、`withinCalendarHours`。[VERIFIED] https://support.atlassian.com/jira-service-management-cloud/docs/set-up-sla-conditions/（offset 100000）、swagger
- **DSA**：通報與申訴都要 "timely"（16(6)、20(4)）；透明度報告要列 "the median time needed for taking the action"（通報）與申訴的 "median time needed for taking those decisions"（15(1)(b)、(d)）。[VERIFIED] springlex Art.15、16、20

### 我們的現況：散在各處，而且互相矛盾
| 工作 | 時限 | 在哪裡 |
|---|---|---|
| 食安通報確認收件 | 4 小時 | `model/governance.go:155` `FoodSafetyAckSLA`、governance.proto:204-205 |
| 一般通報確認收件 | high 24h／medium 72h／low 7 個工作天 | 只在 `seller-violation-management-runbook.md:14-17`。**循環依賴**：嚴重度要等確認收件時才決定（governance.proto:202-203、CARRYOVER:61），所以確認收件之前沒有嚴重度可以算時限 |
| 申訴裁定 | 7 個**工作天** | `seller-violation-management-runbook.md:115`、`:149-153` |
| 申訴裁定 | **14 天** | `food-safety-incident-runbook.md:196` |
| 申訴裁定（簡報 D2 的提案） | 7×24 小時（日曆天） | 簡報 D2；和上面兩條都不一致 |
| 補正裁定 | 7 個日曆天 | `seller-violation-management-runbook.md:135` |
| 食品業者登錄 | 7 個日曆天 | `seller-food-registration-runbook.md:24` |
| RISK 複審 | 每筆措施各自的 `review_by` | measure.proto:106-107 |
| 查證屬實後轉報主管機關 | 24 小時 | `seller-violation-management-runbook.md`（§6 表）；契約沒有追蹤 |
| 工作天的行事曆 | 後端有行事曆表（發票申訴用） | `admin/src/components/UI/SlaCountdown.tsx:13-27`；在 invoice-service，不在單體 |

### 選項
- (a) 簡報的做法：每個資源各加一個 `*_due_time`。
- (b) **一份 SLA 政策**：model 裡一個函式 `Policy(kind, category, severity) → {ack, decide, clock}`，暫停規則也寫在同一處（例：要求賣家配合之後的「等賣家回覆」暫停調查時限）。結果寫進 `review_tasks.due_at`；暫停時重算，或另存 paused_at。client 只拿 due_at 和現在比較。
- (c) 政策放在 DB，可以在後台設定。現在沒有設定者，屬於過度設計。

### 約束帳
| 約束 | 判定 | 來源 |
|---|---|---|
| 工作天的行事曆不在單體 | 若選工作天：REAL（依賴在邊界外，invoice-service）。選日曆天就避開這條依賴 | `SlaCountdown.tsx:13-27` |
| 「契約不變」 | SELF-IMPOSED | 2c-2 D2 |
| 時限不寫進條款 | 不衝突，due_at 只在 admin 契約 | runbook:135 |

### 建議
做 (b)，一律用日曆時間。要決定的只有：申訴是 7 天還是 14 天（兩份 runbook 牴觸，必須由 Will 定）。

一般通報的「確認收件」時限改成**不依嚴重度**：所有類別同一個值，食安仍是 4 小時，其他類別建議 24 小時；嚴重度只決定確認收件之後的**處理時限**。這樣就解掉循環依賴。

觸發條件：要改用工作天，等行事曆搬進單體平台層再說。

---

## 4. 通報 → 調查 → 認定 → 措施 → 申訴／補正 → 複審

### 基準
- **DSA 第 17(3) 條**，理由說明至少要有：
  - (a) 措施種類、期間
  - (b) 依據的事實，並說明是依通報還是平台主動調查
  - (c) 是否用到自動化
  - (e) 依據的條款條號
  - (f) 救濟途徑
  - 原文例："the facts and circumstances relied on in taking the decision … whether the decision was taken pursuant to a notice"。[VERIFIED] springlex Art.17
- **DSA 第 24(5) 條**：決定與理由說明要送進執委會的資料庫，而且 "does not contain personal data"。[VERIFIED]
  - 資料庫的欄位有 `category`、`automated_detection`、`automated_decision`、`decision_ground_reference_url`、`end_date_account_restriction` 等；"Each statement of reasons needs to include at least one decision regarding a restriction"。[VERIFIED] https://transparency.dsa.ec.europa.eu/page/documentation
- **DSA 第 19(1) 條**：第三節（第 20–28 條，含第 24(5) 條）不適用於 micro 或 small enterprise 的平台。[VERIFIED] springlex Art.19。加上我們不在歐盟，DSA 只能當基準，**不是法律約束**。[INFERENCE]
- **P2B 第 4(5) 條**：理由要 "a reference to the specific facts or circumstances, including contents of third party notifications"。[VERIFIED] legislation.gov.uk
- **P2B 第 11(4) 條**：每年公開申訴的件數、主要類型、平均處理時間與結果。[VERIFIED]
- **DSA 第 16(4)、16(5) 條**：通報人留了聯絡方式時要確認收件；決定後要通知通報人。[VERIFIED]

### 對齊的部分
- 申訴的對象是措施，不是認定；和 TSPA、Checkstep 同形。
- 理由說明必填，而且有版本：`measure_revisions`（schema.sql:482-496）。
- 申訴時記下當時的版本：`MeasureRequest.revision`（measure.proto:82-83）。
- 認定和措施分開：measure brief §2 的共通形狀。

### 結構問題
1. **通報對認定只能 1:1，通報看不到後續**：P2B 4(5) 要的「第三方通報的內容」只能從一筆通報取得。通報結案時，通報人的通知不知道後來成立了什麼措施。建議：用 `finding_reports` 讓認定引用多筆通報，通報輸出 OUTPUT_ONLY 的 `resulting_measure_ids`。
2. **沒有案件層級的時間軸**：註記只掛在措施上；簡報 D4 要再加 `report_notes`，又是一張「各資源各自一份」的表。調查經過往往跨通報、跨措施，結果就是 client 要從 5 個來源自己拼時間軸，違反「伺服器推導、client 只呈現」的前提。建議：註記掛在案件（或 1 的通報群組）上，至少不要再新增第二張各資源專用的註記表。
3. **通報不能改類別**：runbook 的做法是另開一筆食安通報、把原本那筆判定不成立（`seller-violation-management-runbook.md` §6 末段）。這是舊模型的寫法，新契約沒有改類別的指令（governance.proto:22-136）。業界在分案時可以改類型。建議：`AcknowledgeReport` 加可選的 `category`（分案時決定類別與嚴重度）。
4. **要求賣家配合之後沒有「等賣家回覆」**：`RequestCooperation` 只記「要求過了」（governance.proto:177-182），沒有回覆期限，也沒有暫停時限。這就是 Zendesk 的 Pending、JSM 的 pause。建議：由 SLA 政策處理這個暫停，配合要求加回覆期限。
5. **透明度匯出**：REAL 判定不需要（第 19 條豁免、不在歐盟）。資料面要能算出處理時間的中位數，`review_tasks` 剛好提供。DSA 17(3) 的各項現在都能推導出來：kind、period、findings 的類別、risk_ground 或 termination_ground 對應條號、有沒有 source_report。觸發條件：歐盟營運，或台灣立法。

---

## 5. 簡報 D1–D6、K1–K14 的影響

| 項目 | 判定 |
|---|---|
| D1 措施詳情獨立頁 | 保留，和基準一致。另外加通報群組／案件的連結 |
| D2 | **改**：REAL 的判定撤回；`MeasureRequest.decide_due_time` 改成 `review_tasks.due_at`；「不混合清單」的業界引述要更正 |
| D3 `ListReports.inbox` | **改**：被 `ListReviewTasks` 的佇列取代；`ListReports` 保留當瀏覽用 |
| D4 `report_notes` | **改**：不加各資源專用的註記表，改成案件或通報群組的註記，措施註記也併過去（`measure_notes` 搬移） |
| D5 原因 enum | 保留（Merchant API 基準已查證）；可以參考 Merchant 每個原因帶 detail 與修正 action |
| D6 側欄 | **改**：一個「待辦」佇列頁，加通報、措施、登錄、KYC 的瀏覽頁；不是 2 到 4 個收件匣 |
| K3 | 加上分案時改類別（§4.3） |
| K9 | 被 task 佇列吸收 |
| 切分 | 新增 3-0（工作層、SLA 政策、`finding_reports`），3a、3c 建在上面 |
| runbook | 申訴 7 天或 14 天的牴觸、依嚴重度的確認收件時限、多人組織的前提，都要在同一次變更裡修正 |

---

## 6. 沒有驗證的部分
- Tremau Nima、ActiveFence、Hive、Cinder 的產品文件：沒找到，只有新聞與公司介紹。
- Amazon Account Health：官方頁面要登入。
- eBay、Etsy、Shopify：只找到論壇與第三方指南，不採用。
- JSM「request type 對應 work type、佇列跨類型」：只讀到搜尋摘要，Atlassian 內文沒讀到。
- Meta「另一位複審者」：只讀到搜尋摘要。
- 沒有確認 invoice-service 的行事曆表具體在哪裡。
- §1 (b) 的成本是估計，沒有寫任何程式碼。
- 沒有跑任何測試。
