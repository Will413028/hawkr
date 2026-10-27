# A3b 步驟 10e 第 3 刀（admin 治理介面）開工的決定題 v2

**Will 的決定（2026-10-16）：全照建議。** D8 的三個值照建議：申訴裁定 7 個日曆天、一般通報確認收件 72 小時、KYC 審核 7 個日曆天。v1（`step10e-3-brief-v1.md`）的 K1–K14 與 D6 未改動的各點仍有效。

Worktree `hawkr-10e`，HEAD `abb690858`。輸入：v1 簡報 `step10e-3-brief-v1.md`、從零研究 `step10e-3-research.md`、外部查證 `step10e-3-baseline-check.md`、盤點 `inv-admin.md`、`inv-backend.md`。起草者沒有參與 v1 與研究；下文的現況都重新對過原始碼與 SSOT。標記：**[VERIFIED]** 這次或 `step10e-3-baseline-check.md` 讀到原文；**[SEARCH]** 只讀到搜尋摘要或二手資料；**[INFERENCE]** 推論。

定位不變：系統沒有上線，「和既有程式一致」「改動比較小」「工作量」都是 SELF-IMPOSED。前提（Will 2026-10-15）：可用性、原因、期限一律由伺服器推導，client 只呈現。

## 結論

**和 v1 相比改了什麼**

1. **v1 D2 的「合併成一個清單被 REAL 擋下」撤回**（研究是對的）。CARRYOVER:88 否決的是「兩張表各自 limit／offset，再到應用層合併」，不是「一個查詢、一個 keyset」。所以要有一個跨種類的**待辦佇列**（新 D7）。
2. **但不採研究的 `review_tasks` 表**。我建議的做法是：佇列是從領域狀態**推導的讀取模型**，用一支 SQL 查詢把 5 種待處理事項 `UNION ALL` 起來，共用一個 keyset，不另外寫入。理由：
   - 這 5 種事項有 4 種的「待處理」部分取決於時間或其他 aggregate。複審到期看 `review_by ≤ now`；措施結束後，待處理的補正由推導變成 CLOSED，儲存值要等 5 分鐘的掃描（`measure/jobs.go:126`）；食品業者登錄的佇列看店家狀態與 KYC 狀態。
   - 另開一張表，就要在每條指令、每個 job、每個時間邊界上雙寫，維持「task OPEN ⇔ 領域待處理」。
   - 設計書 32:105 已經定了「期限讀取時推導，不另存」。
   - 等指派、認領、暫停這些**無法從領域狀態推導**的工作狀態出現時，才把佇列實體化（觸發條件見 D7）。
3. **期限由一份 SLA 政策統一擁有**（新 D8，取代 v1 D2 的「單一 7×24 小時」）：model 的一個函式，一律用日曆時間。各資源輸出自己的 `*_due_time`，佇列也用同一個函式。
4. **v1 D3（`ListReports.inbox`）作廢**，由佇列取代。`ListReports` 留著當瀏覽清單。
5. **v1 D4 改寫成 D4′**：註記掛在耐久的主體（通報、措施）上，不掛在 task 或案件上。儲存改成一張表加排他外鍵，`measure_notes` 併進去；API 仍然每個父資源一組。
6. **新增**：
   - D9 通報的平台判定類別：不改通報人填的類別。
   - D10 認定可以引用多筆通報。
   - D11 指派、案件聚合、等賣家回覆：都延後，各有觸發條件。
7. **D1、D5 不變。** D6 的文案因為新結構要改幾處（§D6′）。
8. **切分改成 4 段**：3a 措施 → 3b 建立 → 3c 通報 → 3d 待辦與 SLA。`maxRESTOperations` 492 → 488 → 487 → 480 → 480。

**要 Will 回答的題目**（每題第一行是建議）

| # | 題目 | 建議 |
|---|---|---|
| D1 | 措施詳情獨立頁（**不變**） | 照 v1 |
| D4′ | 註記的歸屬與儲存 | 掛在通報與措施；一張 `governance_notes`，`measure_notes` 併入 |
| D5 | 修改措施與不能操作的原因（**不變**） | 照 v1 |
| D6′ | UI 同意（v1 D6 加上本版修改） | 照 §D6′ |
| D7 | 待辦佇列的形狀 | 推導的讀取模型（UNION 查詢），一支 `ListReviewQueue`，5 種事項都進來 |
| D8 | SLA 政策與**值** | 一個 model 函式、日曆時間；**申訴裁定的天數由你定**（建議 7 天）、**一般通報確認收件的時數由你定**（建議 72 小時）、KYC 審核（建議 7 天） |
| D9 | 通報分案時改類別 | 不改通報人的類別；新增 `assessed_category`，確認收件時必填 |
| D10 | 認定與通報的對應 | 多對多，`source_report_ids` |
| D11 | 指派、案件、等賣家回覆 | 都不做，留觸發條件 |

---

## 照 SSOT 或前提直接做（知會）

v1 的 K1–K14 都保留。以下只列異動或新增，不同意哪一項就說。

| # | 項目 | 做法 | 來源 |
|---|---|---|---|
| K3′ | 通報指令 | 同 v1。確認收件時多一個必填的「平台判定類別」（D9） | D9 |
| K4′ | `Report.can_*` | 同 v1。`can_export_fda` 與 `can_record_fda_referral` 改看 `assessed_category` | D9 |
| K5′ | 店名 | 同 v1，佇列的列也帶 `store_name` | 先例 `FoodRegistration.store_name` |
| K9′ | 措施清單的「待裁定」 | `ListMeasures(request_state=PENDING)` 留著當措施頁的瀏覽篩選；「最急的是哪件」改由佇列回答 | D7 |
| K15 | 申訴的複核者 | **已經決定，不再重開**：複核者不擋。條款 18.4 寫「未直接作成原決定的人員**或具相當權限者**」，觸發條件是第二個角色（`step10e-2c-design.md:38` G21；計畫:330）。研究 §2 提議「在 task 關閉時記下裁定者 = 建立者」是多餘的：`measures.created_by_admin_id` 與 `measure_requests.decided_by_admin_id` 已經存了（`schema.sql:399,538`），隨時都能查 | 條款 18.4（`seller-terms.md:184`） |
| K16 | runbook 的多人組織前提 | Discord 輪值、主管、oncall 改寫成單人營運的版本，等觸發條件成立才適用的段落要標明。食安 runbook 的 :63-290 是舊 seller-service 的流程，整段重寫（v1 已列） | 研究 §2；`seller-violation-management-runbook.md:36` |
| K17 | 佇列的權限 | 新權限 `seller.review_queue.list`，只給同時擁有 5 種事項讀取權限的角色。列上只帶種類、ID、店名、時間，不帶內容。觸發條件：角色開始分工時，改成伺服器依呼叫者的權限過濾種類 | `adminauth.go:87-112`，一個方法一個權限 |
| K18 | 已過時的 SLA 文字 | 這幾處跟著 D8 在 3d 一起改：設計書 32:383 的「high 24h／medium 72h／low 7d」、`seller-violation-management-runbook.md:14-17,29-31,115,149-153,164`、`food-safety-incident-runbook.md:196,559-571` | D8 |
| K19 | 通報人的類別錯了怎麼辦 | runbook §6 末段寫的「改開一筆 `food_safety_reports`（手動建立）」在新契約做不到：admin 沒有建立通報的方法。這一段改成 D9 的流程 | `seller-violation-management-runbook.md:170` |

---

## 要 Will 決定

### D1. 資訊架構（不變）

**建議：措施詳情做成獨立頁 `/admin/measures/[id]`。刪除違規那兩頁。「建立措施」做成對話框，在三處掛載。**

內容同 v1 D1。另外一點：佇列的列連到各資源的詳情頁，這只是加強了「每個資源一個可連結的網址」這個理由。

### D4′. 內部註記的歸屬與儲存（改寫 v1 D4）

**建議：註記掛在耐久的主體（通報、措施）上。儲存用一張 `seller.governance_notes`，以 `num_nonnulls(report_id, measure_id) = 1` 排他，`measure_notes` 併進去。API 仍然每個父資源一組：既有的 `CreateMeasureNote`／`ListMeasureNotes` 不變，新增 `CreateReportNote`／`ListReportNotes`。**

**現況**
- `measure_notes` 只能新增（`schema.sql:513-523`），有兩支 RPC（`governance.proto:126-134`）。
- 通報只有 `resolution_note`，那是給通報人看的（`governance.proto:212-213`）。
- 兩者都不在個資清理的範圍：`internal/purge` 沒有引用 `measure_notes` 或 `reports`。
- 計畫 :265 寫的觸發條件（10e 做通報處理介面）已經成立。

**業界基準**
- Zendesk 的 internal note、JSM 的「Add internal note」都是工作實體（ticket 或 work item）底下的子項目 [VERIFIED JSM，`step10e-3-baseline-check.md` §2；Zendesk 未擷取原文]。
- 這兩家的工作實體同時就是業務實體，所以「掛在工作項目上」和「掛在主體上」在它們那裡是同一件事。我們的工作項目是暫時的：一個措施一生會先後有申訴、補正、複審好幾個待辦。掛在 task 上，時間軸會被切成好幾段 [INFERENCE]。
- AIP-122：子集合的 API 依父資源分開 [INFERENCE，未擷取]。儲存用排他外鍵是專案內的先例（`evidence_accesses_owner_check`，`schema.sql:587`）。

**為什麼現有機制存在**：4A-2 先把註記掛在違規上，2c 隨措施搬過去；通報那時還沒有介面。

| 選項 | 取捨 |
|---|---|
| (a) v1：另開 `report_notes` | 兩張同形的表、兩份 CHECK 與查詢；之後要把「通報與它導出的措施」的時間軸並排顯示，得查兩張表 |
| (b) **一張表加排他外鍵，API 依父資源分開** | 規則（1–2000 字、只新增）只有一份；之後要組時間軸只要一個查詢。代價是把 `measure_notes` 遷過去，外加一份 migration 測試 |
| (c) 研究的做法：掛在案件或 task 上 | 案件不存在（D11 延後）；task 是暫時的，而且在 D7 下根本沒有實體 |

**約束帳**
- 「沿用 `measure_notes`」：SELF-IMPOSED。
- 「沒有案件」：REAL，因為缺營運資料（D11）。這只擋 (c)，不擋 (b)。
- REAL：無其他。

**建議 (b)**。權限沿用每個父資源一組（`seller.measure_notes.*`，加上 `seller.report_notes.*`）。通報詳情顯示自己的註記；措施詳情顯示措施的註記。「通報與導出措施的合併時間軸」等 D11 的案件觸發條件成立時再做。

### D5. 措施修改介面與不能操作的原因（不變）

**建議：照 v1 D5。** Merchant API 的 `isAvailable` 加 `reasons[]` 已查證 [VERIFIED，`step10e-3-baseline-check.md` §3]。研究對 D5 也沒有異議。

### D7. 待辦佇列：形狀（新增，取代 v1 D2 的佇列部分與 v1 D3）

**建議：一支 `ListReviewQueue`，回傳從領域狀態推導的待辦項目：一個 SQL 查詢 `UNION ALL` 5 個分支，keyset 是 `(due_time 有無, due_time, open_time, kind, id)`，不另外寫入。5 種事項都進來。admin 側欄最上面加一頁「待辦」。**

**現況**
- 每個資源各自有一份 keyset：
  - 通報：新到舊（`governance.proto:22`）
  - 措施：`review_due`、`request_state` 兩種伺服器排序（`:72-74`、`:408-415`）
  - 食品業者登錄：`DECIDABLE`／`AWAITING_KYC`（`seller.proto:105-110`）
  - KYC：`SUBMITTED` 舊到新（`seller.proto:39-41`）
- 沒有地方能回答「現在最急的是哪一件」。
- 待處理與否怎麼判定：
  - 通報：`status`、`acknowledged_at`
  - 申訴與補正：伺服器推導的 `MeasureRequest.state`。措施結束後補正算 CLOSED，儲存值靠每 5 分鐘的掃描補上（`measure/jobs.go:126`）
  - 複審：`review_by ≤ now`，純粹由時間決定（`governance.proto:408`；設計書 32:417「不需要 job」）
  - 食品業者登錄：依店家營運狀態與 KYC 是否核准決定（`seller.proto:105-110`）
  - KYC：`state = SUBMITTED`
- 期限：設計書 32:105「讀取時推導，不另存」。

**業界基準**
- **工作實體同時就是業務實體**，所有種類共用同一個父型別：
  - Zendesk 的 ticket [VERIFIED SLA 文件]
  - JSM 的 work item，佇列混合不同的 request type [SEARCH，研究 §1]
  - ServiceNow 的 `task` 基礎表，incident、problem、change 繼承它 [SEARCH，二手資料：Packt《ServiceNow Application Development》等]
- **獨立的工作項目指向業務記錄**：Salesforce Omni-Channel 的 `AgentWork.WorkItemId` 指向被路由的記錄 [SEARCH，摘要沒有寫明是多型]。它是**路由與指派**的實體，記錄推派、接受、拒絕。
- **T&S**：Checkstep 以 content case 為中心，通報掛在 case 上 [VERIFIED，docs.checkstep.com/headless：「the community report will be associated with that piece of content」]。文件**沒有**描述獨立的 queue 或 task 實體 [VERIFIED：頁面上沒有]。
- **CQRS／DDD 的慣例**：跨 aggregate 的工作清單是讀取模型。一致性的要求只在單一 aggregate 內，清單由狀態投影出來 [INFERENCE，通論]。

研究把這些整理成「業界都有一個獨立的工作層」，這太強了。比較精確的說法有兩點：
- 每家都有**一個統一的佇列**，這一點成立。
- 要另存一個工作實體，只在需要存「不屬於業務狀態」的東西時才出現：指派、路由、認領、暫停、升級。Salesforce 的 AgentWork 正是為了路由而存在。我們現在這些全都沒有（D11）。

**為什麼現有機制存在**：8g 修掉舊收件匣的跨表分頁漏件，改成一個資源一個 keyset。當時沒有「跨種類最急」的消費者，所以沒有討論工作層。

| 選項 | 一致性 | 符合前提 | 可測性 | 成本 |
|---|---|---|---|---|
| (a) v1：一個資源一個收件匣 | 不需要 | 符合 | 每個資源各一份 keyset 測試 | 管理員要巡 4 到 5 頁；每加一種事項就要再做一次 |
| (b) **推導的讀取模型：一個 UNION ALL 查詢** | **天生一致**：成員資格就是領域的判定式，時間邊界（複審到期、措施結束）讀取時就成立 | 符合：期限與成員資格都由伺服器的同一份判定式算出 | 資料集測試：每種領域狀態的 fixture 進不進佇列，必須等於 model 的判定式；跨種類的 keyset 不漏不重 | 一支查詢（5 個分支），各分支的判定式和既有清單共用（例如食品業者登錄的 DECIDABLE）；期限的長度由 Go 的政策函式以參數傳進查詢 |
| (c) 研究的 `review_tasks`：同一個 transaction 寫入 | 雙寫。至少有 4 條路徑不是指令：複審到期（純時間）、措施到期後補正失去標的、店家關閉讓登錄離開佇列、KYC 核准讓登錄進入可裁定。每一條都要 job 或 hook 去開關 task，否則就違反「OPEN ⇔ 待處理」 | 期限存成欄位，和設計書 32:105 牴觸；暫停時要重算 | 要逐一證明每條寫入路徑（含 job）都維持不變式 | 一張表、開關的 model 函式、每條指令與 job 都要改、回填 |
| (d) 完整的案件聚合（通報、認定、措施、申訴都掛在 case 下） | aggregate 很大，跨 store 與 stint 的鎖會變複雜 | 符合 | 難 | 要改寫 2c 剛完成的措施模型。自動歸案的規則沒有資料可以訂（D11） |

**約束帳**
| 約束 | 判定 | 來源 |
|---|---|---|
| 不能跨表分頁再合併 | 只否決應用層合併；(b) 是一個查詢、一個 keyset，不受影響 | CARRYOVER:88 |
| 期限不另存 | 現行的設計規則（SSOT）。它不是外部約束，但 (c) 要推翻它卻拿不出理由 | 設計書 32:105 |
| 內容審核（social）與發票申訴（invoice-service）不在單體 | **REAL**：在邊界外，要等各自的搬遷步驟 | v1 K12；`routes_moderation.go` |
| 只有一位營運人員，沒有指派或路由的需求 | **REAL**：缺第二方。這正是不必實體化的原因 | 研究 §2 |
| KYC 與食品業者登錄「不在治理範圍」 | SELF-IMPOSED（範圍）。它們在同一個 seller 模組，判定式已經存在 | `internal/kyc`、`internal/foodreg` |

**建議 (b)**，5 種事項與它們的開始時間：

| kind | 判定式（和既有的共用） | open_time |
|---|---|---|
| `REPORT_ACKNOWLEDGE` | `status = PENDING ∧ acknowledged_at IS NULL` | `created_at` |
| `REPORT_INVESTIGATE` | `status = PENDING ∧ acknowledged_at IS NOT NULL`，或 `status = INVESTIGATING` | `acknowledged_at` |
| `FDA_REFERRAL` | `assessed_category = FOOD_SAFETY ∧ status = VERIFIED ∧ fda_reported_at IS NULL` | `resolved_at` |
| `MEASURE_REQUEST` | 推導的 `MeasureRequest.state = PENDING`（申訴與補正） | `submitted_at` |
| `MEASURE_REVIEW` | RISK、生效中、`review_by ≤ now` | `review_by` |
| `FOOD_REGISTRATION` | DECIDABLE 的判定式 | max(`submitted_at`, KYC 核准時間) |
| `KYC_APPLICATION` | `state = SUBMITTED` | `submitted_at` |

（`REPORT_*` 有兩個 kind，所以總共是 7 列。食品業者登錄的時鐘從「能裁定」開始，等 KYC 的時間不算在平台頭上。）

**契約**：`ListReviewQueueRequest { page_size, page_token, kind（選填篩選）}`，回傳 `ReviewQueueItem`：
- `kind`
- `oneof subject { report_id | measure_id(+request_id) | food_registration_id | kyc_application_id }`
- `store_id`、`store_name`
- `open_time`、`due_time`（沒有時限時不設）

方法放在 admin 的 `SellerAdminService`（seller 模組的 admin 介面）；放在哪個 service 是實作細節。不提供 Get：項目不是一個儲存的資源。

**終局與觸發條件**
- **實體化**：出現第一個無法推導的工作狀態時，把佇列實體化成事件餵入的讀取模型或工作項目表。可能是指派、認領、暫停時鐘、升級（D11），或是 social、invoice 搬進單體之後要跨模組合併。那時佇列移到平台層，由各模組的事件投影，API 形狀不變。
- **跨模組**：模組不能互相 UNION 對方的表，所以 social 或發票申訴搬進單體的那一步，就是實體化的觸發條件。

### D8. SLA 政策與值（新增，取代 v1 D2 的時限部分）

**建議：model 一個 `ReviewPolicy(kind, category) → window` 函式，一律日曆時間、不暫停，期限 = open_time + window，讀取時推導。各資源輸出 OUTPUT_ONLY 的 `*_due_time`，佇列用同一個函式。值如下表，其中三個要你定。**

**現況：期限散在各處，而且互相牴觸**（研究 §3 的表我逐條驗證過）：

| 事項 | 現在的值 | 在哪裡 | 出處與時間 | 判定 |
|---|---|---|---|---|
| 食安通報確認收件 | 4 小時 | `model/governance.go` 的 `FoodSafetyAckSLA`、`governance.proto:204`、設計書 32:35,105 | 現行 SSOT | 保留 |
| 一般通報確認收件 | high 24h／medium 72h／low 7 個工作天 | `seller-violation-management-runbook.md:14-17`、設計書 32:383 | 2026-05 公司時期 | **循環依賴已證實**：嚴重度要等確認收件時才決定（`reports_acknowledged_check`，`schema.sql:256`；CARRYOVER:61），確認收件之前沒有嚴重度可以算。今天契約對一般通報**沒有任何期限** |
| 申訴裁定 | 7 個**工作天** | `seller-violation-management-runbook.md:115,151` | blame：`2087adf034`，2026-05-07，公司時期 | 沿用，沒有重新評估過 |
| 申訴裁定 | 14 天 | `food-safety-incident-runbook.md:196` | 同一個 commit，2026-05-07。在舊 seller-service 的流程章節裡（同節還有已刪除的 `partial` 裁定與 `seller_violation_appeals` 表） | **過時**，不是有效的 SSOT |
| 補正裁定 | 7 個**日曆天** | `seller-violation-management-runbook.md:135` | `00fa6b1318`，2026-10-15，Will 的 2c-2 D2。理由：「取日曆天免去假日曆」 | 現行，最新的決定 |
| 食品業者登錄 | 7 個日曆天 | `seller-food-registration-runbook.md:24`（「比照措施補正」） | 10d | 現行 |
| KYC 審核 | 沒有 | rg 整個 `docs/runbooks` 都沒有 | — | 缺 |
| RISK 複審 | 每筆的 `review_by` | `measure.proto` | 現行 | 期限 = `review_by` 本身 |
| 查證屬實後轉報主管機關 | 24 小時 | `seller-violation-management-runbook.md:166` | 公司時期 | 契約沒有追蹤 |

**條款（SSOT）怎麼寫申訴的期限**
- 條款 17.4（`seller-terms.md:176`）：「申覆……**不設期限**」，這是賣家**提出**申訴的期限；「甲方應於**合理期間**內……回覆」，申覆與解除請求都適用。
- 條款 18.4（`:184`）沒有寫時間。
- 終止合作的申覆**提出**期間是生效後 6 個月（21.2，`:205`）。
- 所以條款沒有給裁定的天數，內部 SLO 由 Will 決定。研究說「兩份 runbook 牴觸，要由 Will 定」，結論對，但兩個值都是公司時期留下來的，其中 14 天已經過時。真正的選擇是：比照 Will 2026-10-15 的日曆天規則（7 天），或是保留工作天。

**業界基準**
- Zendesk 的政策在伺服器設定；可以選日曆時間或營業時間；輸出 `breach_at` [VERIFIED]。
- JSM 有 start、pause、stop 條件，輸出 `breachTime` [VERIFIED]。
- DSA 第 16(6)、20(4) 條只要求 timely [VERIFIED]。DSA 對我們是基準，不是法律約束：我們不在歐盟，第 19 條也豁免小型平台 [VERIFIED 條文，INFERENCE 適用性]。

**為什麼現有機制存在**：
- 食安的 4 小時寫在 model，因為它是契約欄位。
- 其他時限是公司時期的 runbook。那時候嚴重度由通報人填，所以「依嚴重度定確認收件期限」行得通；CARRYOVER:61 把嚴重度改成分案時決定之後，這條規則就失去前提，但 runbook 沒有跟著改。

| 選項 | 取捨 |
|---|---|
| (a) v1：每個資源各自加一個 `*_due_time`，值各自寫死 | 規則還是散在各處 |
| (b) **一個政策函式，各資源與佇列都從它推導** | 值只在一處；要改值只改一處，再加一份資料集測試 |
| (c) 政策放 DB，後台可以設定 | 沒有設定的人，過度設計 |
| (d) 工作天 | 依賴行事曆，而行事曆在 invoice-service（`SlaCountdown.tsx:13-27`）：REAL，在邊界外；而且違反 Will 2026-10-15「取日曆天」的理由 |

**約束帳**
- 「契約不變」（2c-2 D2）：SELF-IMPOSED。它的理由是不加撤回，和時限由誰擁有無關（同 v1）。
- 「工作天的行事曆」：只有選 (d) 時才是 REAL。
- 「SLO 不寫進條款」：不衝突，`*_due_time` 只出現在 admin 契約。

**建議 (b)，值**：

| kind | 期限 | 狀態 |
|---|---|---|
| `REPORT_ACKNOWLEDGE` 食安 | 4 小時 | 現行，不變 |
| `REPORT_ACKNOWLEDGE` 其他類別 | **Will 決定**：建議 **72 小時**，不依嚴重度 | 24 小時是研究的建議，對單人營運、跨週末來說太緊；72 小時是舊表的中間值。確認收件期限一律依**通報人填的**類別（分案之前只有這個資訊） |
| `REPORT_INVESTIGATE` | 不設 | 延後。缺「調查要多久」的資料（REAL：從未上線）；而且「等賣家回覆」的暫停規則還不存在（D11）。觸發條件：上線後累積 20 筆結案通報，取 P90 訂值，同時加暫停 |
| `FDA_REFERRAL` | 24 小時 | 沿用 runbook:166，知會 |
| `MEASURE_REQUEST` 補正 | 7 天 | 現行（2c-2 D2） |
| `MEASURE_REQUEST` 申訴 | **Will 決定**：建議 **7 個日曆天** | 一個收件匣只有一條規則；和補正、食品業者登錄同樣的日曆天理由。另一個選項是 10 個日曆天（約等於 7 個工作天） |
| `MEASURE_REVIEW` | `review_by` | 現行 |
| `FOOD_REGISTRATION` | 7 天，從可以裁定時起算 | 現行的值；起算點本刀明定 |
| `KYC_APPLICATION` | **Will 決定**：建議 **7 個日曆天** | 目前沒有值。不設的話，佇列裡的 KYC 一律排在有期限的事項後面 |

輸出欄位：
- `Report.acknowledge_due_time`：改成所有類別都設（原本只有食安）。
- 新增 `Report.fda_referral_due_time`、`MeasureRequest.decide_due_time`、`FoodRegistration.decide_due_time`、`KYCApplication.decide_due_time`、`ReviewQueueItem.due_time`。

不輸出 `overdue` 布林值，理由同 v1 D2 (c)：期限固定、不暫停，client 拿現在比較就是精確的。

觸發條件：
- 出現暫停規則（D11 的等賣家回覆）時，`breach_at` 就不再等於 open_time 加上固定值。那時 D7 要實體化，期限要另存。
- 要改用工作天：等行事曆搬進單體的平台層。

### D9. 通報分案時改類別（新增）

**建議：`reports.category` 保持通報人填的值，不可變。新增 `assessed_category`，確認收件時必填，預設等於通報人填的類別。FDA 的資格（CHECK）、`can_export_fda`、`FDA_REFERRAL` 佇列與建立措施時的預填，都改看 `assessed_category`。**

**現況**
- 新契約沒有改類別的指令（`governance.proto:22-136`）。
- runbook 的做法是「改開一筆食安通報、原本那筆判定不成立」（`:170`），但 admin 已經沒有建立通報的方法。而且把真實的通報判定為「不成立」，等於告訴通報人不實的結果。
- 類別參與去重的唯一索引：`reports_buyer_daily` 以 `(reporter_buyer_id, store_id, category, 台北日)` 為鍵（`schema.sql:285`）。
- FDA 欄位以 CHECK 限定食安類別（`reports_fda_check`）。

**業界基準**
- T&S 工具一般把「通報人選的理由」與「審核者判定的政策」分成兩個欄位 [INFERENCE：Checkstep glossary 有 Case 與 Appeal 的定義，但這次沒有讀到兩個欄位分開的原文]。
- Zendesk 讓 agent 直接改 ticket 的欄位，原值留在稽核紀錄 [INFERENCE]。
- 我們自己的模型裡已經有平台的判定：`Violation.category`。只是認定在查證屬實之後才建立，而分案（優先序、FDA）需要的是**更早**的判定。

| 選項 | 取捨 |
|---|---|
| (a) 研究：`AcknowledgeReport` 可以改 `category` | 會和去重索引衝突：同一位買家同一天對同一家店，已經通報過那個類別時，改類別會違反唯一索引；通報人看到的類別被改掉；原本填的值只能從稽核紀錄找回，而通報沒有稽核紀錄 |
| (b) **另存 `assessed_category`** | 通報的內容（DSA 第 16 條的 notice）保持原樣；去重的語意不變；分案結果明確存在一個欄位。代價是一欄、一個 CHECK 改寫、admin 契約一個欄位 |
| (c) 不做，誤分類的就判定不成立 | 告訴通報人不實的結果；食安事件失去 FDA 的流程 |

**約束帳**：「沿用 runbook 的另開一筆」：SELF-IMPOSED，而且在新契約裡做不到。REAL：無。

**建議 (b)**。確認收件的期限仍然看通報人填的 `category`（分案之前只有它）。CHECK 的寫法：`assessed_category IS NULL ⇔ acknowledged_at IS NULL`，併進 `reports_acknowledged_check`。

### D10. 認定與通報的對應（新增）

**建議：`NewFinding.source_report_id` 改成 `repeated source_report_ids`（上限 20），用 `finding_reports` 關聯表存；`Violation.source_report_ids` 跟著改；`Report` 輸出 OUTPUT_ONLY 的 `resulting_measure_ids`（經由 `finding_reports` 與 `measure_findings` 推導）。**

**現況**
- 一個認定最多一筆來源通報（`schema.sql:328`；`governance.proto:375`；`measure.proto:29`）。
- 只有 admin 契約用到這個欄位：消費者只有 admin 舊的兩頁，seller 與 website 的契約沒有它。
- 通報這邊看不到導出了哪個措施。

**業界基準**
- Checkstep：同一個內容的多筆通報掛到同一個 case [VERIFIED]。
- P2B 第 4(5) 條要求理由說明包含「contents of third party notifications」，用的是複數 [VERIFIED，研究 §4]。
- 食安事件常見的情形是多位買家通報同一批貨 [INFERENCE]。

| 選項 | 取捨 |
|---|---|
| (a) 維持 1:1 | 同一事件的其他通報沒辦法連到認定 |
| (b) **多對多** | 一張關聯表；admin 契約改欄位（還沒上線，沒有相容問題）；建立措施的對話框要能多選通報 |
| (c) 完整的案件聚合 | D11 延後 |

**約束帳**：「單一欄位」：SELF-IMPOSED。REAL：無。

**建議 (b)**，放在 3b（它的消費者是建立措施的對話框）。驗證：每筆來源通報都必須是同一家店，而且不是不成立的（和現在的單筆規則相同）。

### D11. 指派、案件聚合、等賣家回覆（新增，全部延後）

**建議：三項都不做，也不先加欄位；各留觸發條件。**

| 項目 | 研究的提議 | 判定 | 約束 | 終局與觸發條件 |
|---|---|---|---|---|
| 指派／認領 | 先放 `assignee_admin_id`，不做 UI | **不放**。沒有寫入者、也沒有讀者的欄位是推測性的設計；在 D7 下也沒有可以放它的表 | REAL：只有一位營運人員 | 第 2 位擁有裁定權限的管理員出現時：D7 實體化成工作項目表，同時加入指派、認領與 round robin |
| 升級的 `priority` | 加欄位 | 不加，理由同上 | REAL：沒有主管這個角色 | 出現主管角色 |
| 四眼 | 記下裁定者 = 建立者 | 已經決定（K15） | — | G21 的觸發條件 |
| QA 抽樣 | 不做 | 同意。領域資料（`decided_by`、`decided_at`）就是抽樣母體 | REAL：沒有第二位重審者 | 第 2 位管理員，或每週裁定超過約 50 件 |
| 案件聚合 | 只做 `finding_reports` | 同意，見 D10 | REAL：沒有資料可以訂歸案規則 | 上線後第一次出現同一家店、同一事件 3 筆以上的通報 |
| 等賣家回覆（配合要求的期限與暫停） | 用 SLA 政策暫停 | 延後：`REPORT_INVESTIGATE` 本來就沒有期限（D8），沒有東西可以暫停 | REAL：缺資料 | 和 D8 的調查期限同一個觸發條件 |
| 透明度匯出 | 不做 | 同意 | REAL：不在歐盟、第 19 條豁免 | 進入歐盟營運，或台灣立法 |

### D6′. UI 同意（v1 D6 加上本版修改）

v1 D6 的第 3、5、6、7、8、9、10 點**不變**。修改與新增如下：

1. **側欄**（改）：
   - 「賣家 / 商品」分組最上面加「待辦」→ `/admin/queue`。
   - 底下是「商家通報」（瀏覽）、「措施與申訴」（瀏覽），放在「食品業者登錄審核」上面；KYC 的入口不動。
   - 「內容審核佇列」不動。
2. **通報清單**（改）：拿掉 v1 的分頁「待確認收件｜待調查｜調查中｜全部」，改成狀態篩選（全部｜待處理｜調查中｜已結案），新到舊。期限欄照常顯示 `acknowledge_due_time`，「{HH:mm} 前確認」或 danger「已逾時 {h} 小時 {m} 分」。v1 的每 30 秒重新整理移到待辦頁。
3. **確認收件對話框**（改）：兩個必填欄位，「平台判定類別」（預設是通報人選的類別，旁邊註明「通報人選的是 {類別}」）與「嚴重度」。
4. **措施清單**（改）：分頁「待裁定｜複審到期｜生效中｜全部」保留（都是既有的伺服器篩選）。逾時的徽章從「超過 7 天」改成跟值無關的「已逾期」，依 `decide_due_time` 判斷；列上同時顯示「已等候 {N} 天」，0 天顯示「今天提出」。
5. **待辦頁**（新）：
   - 欄位：種類、店家、開始時間、期限。
   - 種類的標籤：「確認收件」「調查通報」「轉報主管機關」「裁定申訴」「裁定補正」「複審停權」「審核食品業者登錄」「審核身分認證」。
   - 期限的顯示：「{日期 HH:mm} 前」；逾時時顯示 danger「已逾期 {時間}」；沒有期限時顯示「—」。
   - 篩選只有種類。依伺服器的順序：有期限的在前，由急到緩。
   - 每 30 秒重新整理，視窗回到前景時重讀。
   - 列點下去進各自的詳情頁。
   - 空狀態：「目前沒有待辦」。
6. **建立措施對話框**（改，D10）：「來源通報」改成可以多選，只列同一家店、還沒判定不成立的通報。從通報詳情開啟時，預先勾選那一筆。
7. **通報詳情**（加）：「導出的措施」區塊，列出 `resulting_measure_ids`，連到措施詳情頁。

---

## 切分

規則同 v1：契約的新增和它的消費者放在同一段；每段刪掉最後一個消費者已經改完的路由。佇列是 Connect 方法，不算進 `maxRESTOperations`。

| 段 | admin UI | 契約與後端 | 刪除的 gateway 路由 | `maxRESTOperations` |
|---|---|---|---|---|
| 3a 措施 | `/admin/measures` 清單、`/admin/measures/[id]` 詳情（請求、裁定、修改、撤銷、結束、延後複審、註記）；刪除 `MeasureDetailDialog`、`violations/[id]`；K6、K9′、K10、K11 | D5 的原因 enum 與欄位；`Measure.store_name`；D4′ 的 `governance_notes`（併入 `measure_notes`，措施的 RPC 不變） | `GET`／`PATCH /admin/violations/:id`、`revoke`、`appeal/decide`（共 4 條） | 492 → 488 |
| 3b 建立 | 建立措施對話框（K7、D6′-6），掛在商家詳情與發票申訴頁；刪除 `violations/new` | D10：`finding_reports`、`source_report_ids` | `POST /admin/violations`（1 條） | 488 → 487 |
| 3c 通報 | 通報清單與詳情、各指令、FDA、配合要求、註記、導出的措施；「建立措施」掛 3b 的對話框 | K4′ 的 `Report.can_*`、`Report.store_name`、D9 的 `assessed_category`、`Report.resulting_measure_ids`、通報註記的 2 支 RPC 與 2 個權限 | 通報 7 條；`routes_governance.go` 的 admin 半部、wrapper、DTO、`governance-wire-contract.test.ts` 與它在 baseline 的那一筆 | 487 → 480 |
| 3d 待辦與 SLA | `/admin/queue`、側欄；措施清單與通報清單改用 `*_due_time` 顯示逾期 | D8 的 `ReviewPolicy` 與所有 `*_due_time`（一般通報的確認收件期限是新規則）；D7 的 `ListReviewQueue`、權限（K17）；K18 的 runbook 與設計書；`FoodRegistration` 與 `KYCApplication` 的 `decide_due_time` 由既有的 admin 頁顯示 | 無 | 480 |

- 3d 排在最後，因為佇列的每一種列都要連到已經存在的詳情頁。3a 到 3c 期間，「最急」由各清單既有的伺服器篩選撐著：系統沒有上線，這是同一步驟內的施工順序，不是技術債。
- 如果 10d 的 K1（4 條）先落地：484 → 483 → 476 → 476，3c 刪掉整個 `routes_governance.go`。每次 rebase 都以 openapi 的實際數目重算（現在 `openapi_coverage_test.go:27` 是 492）。
- 審查：3a、3c、3d 各做 code-review 與 design-review（3d 另外要有 keyset 跨種類不漏不重，以及「成員資格 ⇔ 判定式」的資料集測試）；3b 只做 code-review。
- 回退：每段都是單一 commit。3a 的 `measure_notes` 遷移要附 down migration；其餘照 v1。

---

## 附錄 A：我和研究（`step10e-3-research.md`）不同意的地方

1. **工作層的基準**（§1、§0-2、§0-5）：研究把「業界有獨立的工作層」當成基準。查證的結果，Zendesk、JSM、ServiceNow 的工作實體**就是**業務實體，只是所有種類共用一個父型別；Checkstep 文件沒有描述獨立的 task 實體；只有 Salesforce 的 AgentWork 是獨立的工作項目，而它存在是為了路由與指派。研究自己引用的 JSM（混合多種 request type，因為底下是同一個 work item）支持的是「一個統一的父型別或統一的 view」，不是「一張指向業務記錄的表」。這部分我改成 D7 (b)。
2. **雙寫的不變式**（§1 (b)）：研究說「由領域指令在同一個 transaction 開啟與關閉」。但至少有 4 種成員資格的變化不是指令：複審到期、措施到期讓補正失去標的、店家關閉、KYC 核准。研究低估了成本，也沒有處理和設計書 32:105「期限不另存」的牴觸。
3. **`assignee_admin_id` 與 `priority` 先放欄位**（§2）：沒有寫入者、也沒有讀者，是推測性的設計，不是基準。基準是「有指派需求時才有指派實體」。
4. **四眼**（§2）：條款 18.4 對裁定者的要求，研究只引了 DSA 第 20(6) 條，沒有看我們自己的條款；而這件事 G21 已經決定過。「在 task 關閉紀錄記下裁定者 = 建立者」是多餘的，現有欄位就能查。
5. **註記掛在案件或 task 上，`measure_notes` 搬移**（§4.2、§5 D4）：案件不存在，task 是暫時的。我改成掛在耐久主體上、一張儲存表、API 依父資源分開（D4′）。研究說「client 從 5 個來源自己拼時間軸違反前提」：把伺服器已經排好序的資料並排顯示是呈現，不是推導可用性或原因，所以這條指控不成立。
6. **`AcknowledgeReport` 加可選的 `category`**（§4.3）：直接改會和去重索引衝突，也會改掉通報的原始內容。我改成另存 `assessed_category`（D9）。
7. **申訴 7 天或 14 天「兩份 runbook 牴觸」**（§3）：14 天在舊 seller-service 流程的章節裡，和 7 個工作天是同一個公司時期的 commit（2026-05-07），已經過時。真正要決定的是「7 個工作天，或比照 2026-10-15 的日曆天」。
8. **一般通報確認收件建議 24 小時**（§3）：對單人營運太緊。我建議 72 小時，由 Will 定。
9. **透明度**（§4.5）說「`review_tasks` 剛好提供處理時間的中位數」：領域表已經有 `submitted_at` 與 `decided_at`，不需要另一張表。
10. **3-0 放在最前面**（§5）：佇列的列要連到 3a、3c 才會有的詳情頁，所以改成放在最後的 3d。另外，計畫:243 已經有別的步驟用過「3-0」這個名稱，容易混淆。

研究對的地方，本版採納：
- 撤回 v1 的 REAL 判定
- SLA 散在各處
- 一般通報期限的循環依賴
- 通報不能改類別的缺口
- 認定只能對應一筆通報
- runbook 的多人組織前提
- D1、D5 保留

## 附錄 B：沒有驗證的部分

- ServiceNow 的 `task` 繼承：只讀到二手資料（Packt、教學網站），沒有讀到 ServiceNow 官方文件。Salesforce `AgentWork.WorkItemId`：官方頁面回 403，只有搜尋摘要，沒有確認它是多型。
- Checkstep 是否另有 queue 實體：headless 文件沒有寫，OpenAPI 規格沒有讀。
- 「T&S 工具把通報理由和判定政策分成兩個欄位」：推論，沒有原文。
- UNION ALL 查詢在 Postgres 的執行計畫（各分支的索引與 Merge Append）：沒有實測。在預期的量下不是問題，但 keyset 跨分支、NULL 期限的排序寫法要在 3d 用測試鎖住。
- `adminauth` 能不能在 handler 裡取得呼叫者的權限、依權限過濾種類（K17 的終局）：沒有確認。
- KYC 的期限與食品業者登錄的起算點（KYC 核准時間取自 `kyc_applications.decided_at` 或審核紀錄）：沒有逐欄確認。
- 研究引用的 TSPA、Meta、P2B 原文：沿用研究的標記，這次沒有重讀。
- 沒有跑任何測試；480 是用目前的 492 做的算術。

---

## 實作時的修訂（3a）

實作時與上文不同、或上文沒寫而實作補上的規則。

1. **`can_revoke_findings` 與 `revoke_findings=true` 的拒絕規則**：`Measure.can_revoke_findings` 從「依據是 VIOLATION」收窄為「依據認定、而且措施自己建立了至少一筆尚未撤銷的認定」；`RevokeMeasure` 與 `DecideAppeal` 在 `revoke_findings=true` 而 `can_revoke_findings` 為 false 時回 `invalid_argument`（欄位 `revoke_findings`），原本是靜默不做事。runbook `seller-violation-management-runbook.md` 的裁定表已記。
2. **管理員撤銷的輸入列舉**：`RevokeMeasureRequest.reason` 改用 admin governance.proto 的 `AdminRevokeReason`（`UNSPECIFIED` 加 `ADMIN_CORRECTION`、`EXTERNAL_DECISION`、`OTHER`），讓請求表示不出 `APPEAL_ACCEPTED`、`FINDING_REVOKED`、`SOURCE_REVOKED`；admin 的選項取自生成的列舉值，不再自己維護一份清單。儲存與回應仍是 seller 的 `RevokeReason`。模組的 `Revoke` 仍自己檢查 `ManualRevokeReasonValues()`（不依賴傳輸層，領域不變量；第 4 輪修正還原）。（沿用盤點：`CARRYOVER.md`）
3. **migration 只有 up（待 Will 確認）**：上文「回退」一列原文要求 3a 的 `measure_notes` 遷移附 down migration。seller 模組沒有任何 down migration（`ls backend/internal/modules/seller/db/migrations` 36 個檔，全是 `<時間戳>_<名稱>.sql` 加 `atlas.sum`，沒有 down 或 rollback 檔；內容也沒有 down 段），3a 的遷移 `20261016120000_seller_governance_notes.sql` 照模組慣例只有 up；回退是 `git revert` 加一支新的往前 migration。業界基準：Atlas 與 Flyway 的社群版都以往前修正（roll-forward）為主，Atlas 的 down 是由 schema 差異即時產生而不是手寫檔。與原文不同，待 Will 確認；若要 down，需要先為整個模組訂慣例，不是只補這一支。
4. **不能操作的原因帶伺服器的文字**（Will 2026-10-17，`step10e-labels-brief.md` D-L2，3a 提交前修正）：四個 `*_unavailable_reasons` 欄位（enum 清單）改成 `*_unavailabilities`，型別 `repeated MeasureActionUnavailability { reason; google.rpc.LocalizedMessage message }`（欄位號 39–42 沿用：它們從未進過任何 commit）。文字由 `model.ActionUnavailableText(reason, action)` 一個函式產生，API 的 `message` 與被拒絕時錯誤的 `LocalizedMessage`（取第一項）共用；admin 刪 `UNAVAILABLE_REASON_LABEL`，直接顯示 `message`，沒有文字才顯示通用文字。「賣家已提出請求，只能縮短期間」不準確（凍結條件是提出過申訴，或有待處理的解除請求；警告提出申訴後也不能縮短），改成依動作的兩句。proto 的「標籤」註解（`ViolationCategory`／`ViolationSeverity`、`MeasureActionUnavailableReason`）一併刪除；admin 的認定類別／嚴重度對照表暫留，由刀 L 的共用文案目錄取代。

---

## 實作時的修訂（3b）

1. **`finding_reports` 的 migration 只有 up**：同 3a 修訂 3（`20261018000223_seller_finding_reports.sql`：建表、搬資料、刪 `violations.source_report_id`；回退是 `git revert` 加新的往前 migration）。同一家店與「不是不成立」的規則仍在應用層檢查（鎖通報列之後），沒有為它再加複合外鍵。
2. **建立措施對話框的新文字**（簡報沒有原文，實作者補，Will 2026-10-18 核准）：成功提示「已建立措施。」；欄位標籤「種類」（沿用 D6 第 4 點的欄名）；來源通報的選項「通報 {id}・{類別}・建立 {時間}」、說明「選填；最多 20 筆，只列這家店還沒判定不成立的通報。」、空狀態「這家店沒有可以當來源的通報。」、載入「載入通報中...」；引用認定的說明「選填；引用的認定必須未撤銷。」、空狀態「這個帳號沒有生效中的認定。」；驗證訊息「請選擇種類。」「請選擇類別。」「請選擇嚴重度。」「請填寫結束時間。」「來源通報最多 20 筆。」。其餘（給賣家的理由、內部註記、結束時間、停權警告、失敗後備句）沿用 `CreateRiskMeasureDialog` 與 `store-detail.ts` 已核准的原文。
3. **發票申訴頁**：兩顆鈕與「賣家治理」說明照 D6 第 10 點；說明的後半句「處分才會帶有到期時間與自動恢復」併入新句，不再重複。同頁與 `admin-actions.ts` 其餘提到「賣家治理頁建立違規紀錄」「違規表單」的句子（`complaints/[complaintId]/page.tsx` 的停權建議提示、建議理由欄標題、確認勾選，`features/invoices/admin-actions.ts` 的兩句動作說明）簡報沒有改文；Will 2026-10-18 核准的新文：停權建議提示「這是一份建議，**還沒有任何人被停權**。建立措施之後，處分才會生效。」、欄標題「建議理由（可直接複製到措施的理由）」、確認勾選「…實際處分要另外建立措施。」、兩句動作說明的後半改成「處分要另外建立措施。」與「實際處分要另外建立措施。」。
4. **舊的通報詳情頁**（`violation-reports/[source]/[id]`）原本連到 `violations/new` 的「後續：建立違規處罰紀錄」區塊隨路由一併刪除（那頁用舊 REST 的整數 id，與新的通報 id 不互通，3c 整頁改寫）；3c 的通報詳情掛 `CreateMeasureDialog`（`initialSourceReportIds`）。
5. **審查後的修正**（Will 2026-10-18 核准文案與做法）：
   - 發票總覽「因發票違規停權」卡片的說明改成「點擊後看到的是全部停權中的賣家；個別賣家的措施請到商家詳情的「措施歷史」查看。」（原句指向已不存在的違規頁）。
   - 發票契約的 `governance_endpoint`（值是已刪的 `/api/v1/admin/violations`，沒有 client 讀它）整條刪除：invoice proto（`reserved`）、Go 與 Java 生成物、invoice-service、gateway DTO、openapi、admin 型別、設計書 58 與 58A。
   - 冪等：來源通報與引用的認定是集合，伺服器的 request hash 與 admin 的請求鍵都先排序（建立措施與終止合作）；之前取消再勾回後重送會換新的 request_id，可能建出第二個措施。主要依據改由規則決定：有新認定用新認定，否則取最嚴重、同嚴重度取最新的引用認定；所有 id 清單都是集合，不再以位置表示主要依據。
   - 來源通報清單：載入的通報全被濾掉、但還有下一頁時，仍顯示「載入更多」。店家資料讀取中或失敗時，「依據的認定」顯示既有的載入與失敗狀態（可重試），不再整欄消失。
   - `food-safety-incident-runbook.md` 建立停權的段落改成「建立措施」對話框與 `CreateMeasure`；設計書 32 刪除對舊 `/admin/violations/*` 的說明。
6. **排進 3c**（Will 2026-10-18 核准）：
   - 發票建議卡的「依建議建立措施」開啟對話框時預先填入：種類＝停權、結束時間＝現在＋建議天數、給賣家的理由＝建議理由，都可修改；類別不預填（類別沒有發票相關的值，由管理員選）。建議卡的欄標題同時改回「建議理由」。對話框為此加初始值的 props，和通報詳情的 `initialSourceReportIds` 同一套。
   - 來源通報清單的「不是不成立」改看伺服器給的 `Report.can_cite_as_source`（K4），client 不再自己依狀態判斷。
