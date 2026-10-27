# A3b 步驟 10e 第 3 刀（admin 治理介面改用措施契約）開工的決定題

**已被 `step10e-3-brief.md`（v2）取代的部分**：D2、D3、D4、D6 第 1、2、4 點。其餘照 v2 的說明仍有效。

Worktree `hawkr-10e`，HEAD `abb690858`（`local/10e` 與 `main` 同一個 commit）。兩份唯讀盤點（`inv-admin.md`、`inv-backend.md`）只當線索；起草者沒有參與盤點，下文的事實都已對過原始碼與 SSOT：計畫 `2026-09-29-a3-buyer-seller.md`（:265、:275、:317、10e 段）、`step10e-decision-brief.md`（K11–K14、K22、D4 第 11 點、D5、切分）、`step10e-measure-brief.md` §8、`step10e-2c2-brief.md`（D1、D2）、`api/hawkr/admin/seller/v1alpha1/{governance,measure}.proto`、runbook `seller-violation-management-runbook.md:135`。

定位：系統沒有上線，「和既有程式一致」「改動比較小」都只算 SELF-IMPOSED。前提（Will 2026-10-15）：可用性與原因一律由伺服器推導，client 只呈現。外部基準已連網查證（`step10e-3-baseline-check.md`）：Zendesk、JSM、Amazon 部分成立（差異已註明），Merchant API 與 DSA 成立；條文沒有要求內部註記，D4 那句是推論。

## 結論

- 要 Will 決定的有 **6 項**，每項一句建議：
  - **D1 資訊架構**：措施詳情做成獨立頁 `/admin/measures/[id]`，取代 `MeasureDetailDialog`；刪除 `/admin/violations/new` 與 `/admin/violations/[id]` 兩頁，「建立違規措施」做成一個對話框元件，在商家詳情、通報詳情、發票申訴頁三處掛載。
  - **D2 措施收件匣與處理時限**：不跨資源合併成一個清單。措施收件匣是一頁（待裁定｜複審到期｜生效中｜全部）。處理時限改由伺服器輸出 `MeasureRequest.decide_due_time`，client 只拿它和現在比較。這會**重開 2c-2 D2「契約不變」的那一半**；時限同時套用到申訴。
  - **D3 通報收件匣的佇列與排序**：`ListReports` 加伺服器定義的佇列（比照 `FoodRegistrationInbox`），「待確認收件」依期限舊到新排序。
  - **D4 通報與調查的內部註記**（計畫 :265 的觸發條件已經成立）：新增只能新增的 `report_notes` 時間軸，形狀與措施註記相同。
  - **D5 措施修改介面**：一個「修改措施」對話框，依 `can_update_statement`、`can_extend`、`can_shorten` 決定哪些欄位能改；不能改的原因由伺服器輸出（新的 `*_unavailable_reasons`），不能改時欄位反灰並顯示原因，不隱藏。
  - **D6 UI 同意**：版面、文案、流程一次列齊（§D6）。
- 知會 **K1–K14**，照 SSOT 或前提直接做（§知會）。
- 切分成 3 段，各自 commit。`maxRESTOperations` 492 → 488 → 487 → 480（§切分）。
- 盤點有 4 處錯誤或過時（附錄 A）。最重要的一處：這 12 條路由今天打的是單體 legacy server 上**沒有實作**的方法，不是舊的 seller-service。所以舊頁面現在已經不能用，切換期間不會造成功能退步。

---

## 照 SSOT 或前提直接做（知會）

不同意哪一項就說。

| # | 項目 | 做法 | 來源 |
|---|---|---|---|
| K1 | 通報篩選 | 嚴重度篩選拿掉（嚴重度欄保留），「來源」改「類別」，路由 `[source]/[id]` 改 `[id]`、改讀 `GetReport` | 已決定的 K11（`step10e-decision-brief.md`）；`governance.proto:220-229` |
| K2 | FDA 轉報 | `GetFDAExport` 只回 `Report`＋`Store`，`.md` 由 client 組（`governance.proto:46-47`「轉報文件的格式由 client 呈現」）。記錄用 `FDAChannel`，已記過就顯示原本的紀錄（`already_recorded`） | K13；`governance.proto:241-249` |
| K3 | 通報指令 | PATCH 自由字串改成三個指令：確認收件（選嚴重度）、開始調查、結案（查證屬實／不成立，給通報人的說明可以不填）。「要求賣家配合」對所有還沒判定不成立的通報都顯示 | D4 第 11 點；`report.go` 的前置檢查 |
| K4 | **通報的 `can_*`（契約新增）** | `Report` 加 OUTPUT_ONLY 的 `can_acknowledge`、`can_start_investigation`、`can_verify`、`can_dismiss`、`can_export_fda`、`can_record_fda_referral`、`can_request_cooperation`、`can_cite_as_source`（能否當 `NewFinding.source_report_id`）。這些和 `internal/governance/report.go` 的前置檢查抽成同一份 model 函式。今天每個前置檢查都寫在 service 裡（`internal/governance/report.go:180,208,243,284,319,383`、`internal/measure/create.go:484`），client 只能照狀態自己推導，違反前提 | 前提 2026-10-15；先例 `Measure.can_*`（`measure.proto:127`） |
| K5 | 列上的店名（契約新增） | `Report`、`Measure` 加 OUTPUT_ONLY 的 `store_name`（清單也帶）。收件匣不為了店名逐列呼叫 `GetStore` | 先例 `FoodRegistration.store_name`（`seller.proto:814`） |
| K6 | 撤銷與裁定 | `RevokeReason` 用既有的 `REVOKE_REASON_LABEL`；裁定用 `ACCEPTED`／`REJECTED`，理由必填（賣家看得到），帶 etag；`VERSION_CONFLICT` 時重讀並保留輸入（`measure.ts` 的 `MEASURE_CONFLICT_TEXT`、`runMeasureAction`） | K14；`governance.proto:472-504` |
| K7 | 建立違規措施 | 只有警告與停權（VIOLATION 依據）：新認定（類別、嚴重度、來源通報）加引用的認定（`CitedFindingsField`），`statement` 與第一則註記都必填，`request_id` 冪等。罰款、部分停權、「同意公開」都刪除 | K12；`governance.proto:313-370` |
| K8 | 認定沒有獨立指令 | 不做 `CreateViolation`、`RevokeViolation`、`UpdateViolation`。撤銷認定只能經 `RevokeMeasure`／`DecideAppeal` 的 `revoke_findings`。`PATCH /admin/violations/:id` 沒有消費者，直接刪 | K22；`measure.proto` 的 `Violation` 註解 |
| K9 | 待裁定只有一個分頁 | 申訴與補正合在同一個分頁（`request_state=PENDING`），伺服器已經排好序：複審到期最前，其次依請求提出時間舊到新。列上顯示請求種類，client 不重排 | 2c-2 的設計審查 D1；`governance.proto:72-74`、`:410-414` |
| K10 | 商家詳情的措施歷史 | `MeasureHistoryCard` 今天只顯示申訴的狀態（`:96`），改成顯示 `requests` 全部種類；列點下去進 D1 的詳情頁 | 2c-2 的設計審查 D1 |
| K11 | 佐證 | 通報與請求的佐證都經 `/api/bff` 讀 `view_uri`，沿用 KYC 的同源 helper（`kyc-application.ts:675`） | `measure.proto` 的 `Evidence` |
| K12 | 內容審核 `/admin/reports` | 不在本刀範圍。它是 social-service 的貼文與使用者檢舉（`routes_moderation.go`），單體沒有對應的模組，也沒有 RPC，跟著 social 的搬遷處理 | `routes_moderation.go:9-19`；`backend/internal/modules/` 只有 content、seller 等 9 個模組 |
| K13 | gateway 的殘留 | 刪 12 個 `Seller.Admin*` wrapper、只有它們在用的 DTO（`toUnionReportRowDTOs`、`toAdminViolationDTO`）、`governance_test.go` 的 admin 部分、`admin/tests/governance-wire-contract.test.ts`（以及它在 legacy baseline 裡的那一筆）。`mockSellerServiceClient` 留著：它要滿足整個 pb interface。10d 的 K1（刪非 admin 的 4 條）與本刀誰後落地，誰就刪掉整個 `routes_governance.go` | `inv-backend.md` §1；`step10d-decision-brief.md:44` |
| K14 | seller-service 的治理 RPC | gateway 不再呼叫之後，它們就成了死碼。程式在步驟 11 隨 seller-service 一起刪除，本刀不動 | 計畫:139 |

文件跟著改：`food-safety-incident-runbook.md`（:10-45 的速查表、:63-172、:200-290 的 SQL SOP 都還是舊的 seller-service 流程）、`seller-violation-management-runbook.md`、設計書 32、seller `CARRYOVER.md`、計畫 10e 的完成記錄。

---

## 要 Will 決定

### D1. 資訊架構：從以違規為中心改成以措施為中心

**建議：措施詳情做成獨立頁 `/admin/measures/[id]`。刪除 `/violations/new`、`/violations/[id]`。「建立違規措施」做成對話框元件，在三處掛載。**

**現況**
- 違規頁的形狀是舊模型：`violations/[id]/page.tsx:76,87,96` 打 `GET`、`revoke`、`appeal/decide`，但申訴現在掛在措施上（`DecideAppeal.measure_id`），撤銷也在措施上（`RevokeMeasure.revoke_findings`）。
- `violations/new/page.tsx:104` 用 query 預填（`seller_id`、`category`、`severity`、`source_report_id`）。發票申訴頁有兩處導到這裡（`invoices/complaints/[complaintId]/page.tsx:527,554`），舊通報詳情在查證屬實後也連過來（`violation-reports/[source]/[id]/page.tsx:381-403`）。
- 措施詳情目前是商家詳情裡的 `MeasureDetailDialog.tsx`（190 行）：沒有版本、請求與佐證、註記、認定，也沒有任何依 `can_*` 的操作。單體這邊已經有 `GetMeasure`（含 `revisions`、`requests`、`findings`、`linked_accounts`）與 `ListMeasureNotes`。
- 認定是只讀的（`Violation.origin_measure_id` 指回建立它的措施），而且清單本身就內嵌了 `Measure.findings`。

**業界基準（已查證，見 `step10e-3-baseline-check.md`）**：Zendesk 的 ticket、Jira Service Management 的 request，都是一個可以直接連結的獨立頁，收件匣、通知、關聯項目都連到同一個網址。「建立」是表單或 modal，從需要它的上下文開啟，並帶入預填值。Amazon Account Health 是每個問題一頁。

**為什麼現有機制存在**：公司時期治理的主體是違規，違規同時是認定、處分與申訴，所以每筆違規一頁。2c-1 把措施和認定拆開之後，申訴、版本、註記、效果都改掛在措施上。對話框是 2c 在商家詳情上補的最小介面，當時第 3 刀還沒做。

| 選項 | 取捨 |
|---|---|
| (a) **措施詳情做成獨立頁**，商家詳情與收件匣都連過去；`MeasureDetailDialog` 刪除；違規兩頁刪除；建立用對話框元件，在三處掛載 | 詳情內容多（版本對照、請求與佐證、註記、關聯帳號、7 種操作），modal 放不下；收件匣、通報詳情、之後 A8 的通知都要能直接連到一筆措施。建立時有預填、從原本的頁面開啟，關掉就回到原來的地方，不需要帶 query 的路由 |
| (b) 維持對話框，加大 | 不能直接連結；收件匣點進去要先跳到商家詳情再開對話框 |
| (c) 措施詳情與建立都做成頁（`/admin/measures/new?store_id=…`） | 發票申訴頁那兩處的導頁可以原封不動，只改網址；但多一條帶 query 參數的路由，而且建立完要再導回來 |

**約束帳**
- 「沿用 2c 的對話框」：SELF-IMPOSED（和既有程式一致）。
- 「發票申訴頁的導頁參數 `seller_id`」：SELF-IMPOSED。計畫 10e 的切分表本來就要求參數一起改。
- REAL：無。

**建議 (a)**。認定在措施詳情的「依據」區塊內嵌顯示，連到它的 `origin_measure_id`；不另做認定頁。觸發條件：出現「一筆認定被多個措施引用，需要從認定反查」的營運需求時，再加認定頁，以及 `ListMeasures` 的 cited-finding 篩選。

### D2. 措施收件匣與處理時限

**建議：措施收件匣是獨立的一頁，不和通報合併。處理時限改由伺服器輸出 `MeasureRequest.decide_due_time`，client 只拿它和現在比較，申訴也適用。**

**現況**
- `ListMeasures` 的 `request_state=PENDING` 已經是申訴與補正合一的佇列，`review_due=true` 是複審佇列，兩者各有伺服器的排序（`governance.proto:72-74`）。篩選條件之間是 AND，所以「待裁定或複審到期」要分兩次查詢。
- 通報與措施是兩個資源，各有自己的 keyset。CARRYOVER:88 已經把「兩張表各自分頁再合併」判定為不成立（舊收件匣的跨表分頁 bug）。
- 處理時限：runbook `:135` 寫的是「待處理的補正 7 個日曆天內」「由 `MeasureRequest.submit_time` 算，不需要新欄位」。依據是 2c-2 D2（Will 2026-10-15）：「契約不變、不加撤回」。
- 先例：通報確認收件的期限由伺服器輸出（`Report.acknowledge_due_time`，「是否逾時由 client 以目前時間判斷」，`governance.proto:200-201`）。admin 的 `SlaCountdown` 只接受伺服器算好的工作天，「前端一個字都不算」（`SlaCountdown.tsx:13-27`），不能拿來顯示日曆天。
- 申訴目前沒有內部時限，但條款 17.4 的「甲方應於合理期間內…回覆」緊接在申覆與解除請求之後（`seller-terms.md:176`），讀起來兩者都適用。

**業界基準（已查證，見 `step10e-3-baseline-check.md`）**：Zendesk 的 SLA 政策在伺服器設定，API 的 `policy_metrics[].breach_at` 給到期時間，view 可加 SLA 欄由急到緩排序。Jira Service Management 的 SLA 由伺服器計算，每個 queue 一個 JQL，可以依剩餘時間排序。兩者的佇列都是同一種資源的多個 view，不會把兩種資源混成一個清單。DSA 第 20(4) 條要求申訴「及時」處理。

**為什麼現有機制存在**：2c-2 D2 要解決的是「補正要有時限」，當時的取捨是「要不要給賣家撤回」，所以結論寫成「不改契約」。那時沒有比較「時限由誰擁有」。

| 選項 | 取捨 |
|---|---|
| (a) client 由 `submit_time` 自己算，7 天寫在 admin 的常數 | 契約不動（照 2c-2 D2）。7 天會同時出現在 runbook 與 admin 程式兩處；之後要加超時通知時（runbook 寫的觸發條件是「有營運人員時」），伺服器還是得再定義一次 |
| (b) **伺服器輸出 `MeasureRequest.decide_due_time`**（OUTPUT_ONLY，只在 PENDING 時設，等於 `submit_time` 加 7×24 小時），client 只比較現在 | 和 `acknowledge_due_time` 同一種形狀；規則只有一個擁有者（model）；之後的超時通知與排序可以直接用。代價是一個 admin 契約欄位，外加 runbook 一句話要改 |
| (c) 伺服器直接輸出 `overdue` 布林值 | 是否逾時取決於現在的時間，清單快取之後就會變舊。業界一定給到期時間（Zendesk `breach_at`、JSM `breachTime`）；JSM 另在讀取時算 `breached`、Zendesk 算有正負號的剩餘分鐘，主因是時鐘會暫停或只算營業時間。本案固定 7×24 小時、不暫停，到期時間就足夠 |

「已等候 N 天」只是呈現：取 `now − submit_time` 的完整天數（不用台北日期相減：用日期相減的話，23:59 提出的請求隔天早上就算 1 天，和到期時間的 7×24 小時對不起來）。0 天顯示「今天提出」。

**約束帳**
- 「契約不變」（2c-2 D2、runbook:135）：SELF-IMPOSED。這是 Will 前一天的決定，但它的理由是不加撤回，並不是要求時限由 client 擁有。
- 「SLO 不寫進契約與條款」（runbook:135）：條款的部分照舊，`decide_due_time` 只在 admin 契約，賣家看不到。這條不是約束。
- 「通報與措施合併成一個清單」：被 REAL 擋下。兩個獨立的 keyset 合併分頁，正是 CARRYOVER:88 已經判定不成立的做法；要合併就得新增跨資源的佇列表。
- REAL：無其他。

**建議 (b)，並且申訴也適用同一個 7 天時限**（同一個佇列只有一條規則；條款 17.4 的回覆義務涵蓋兩者）。收件匣的列顯示「已等候 N 天」，超過時限時加一個有文字的 danger 徽章。食品業者登錄的收件匣也有同樣的 7 天 SLO（`seller-food-registration-runbook.md:24`），但目前沒有顯示。觸發條件：本刀之後如果 Will 要它也顯示，就比照加 `FoodRegistration.decide_due_time`；本刀不做。

### D3. 通報收件匣的佇列與排序

**建議：`ListReports` 加伺服器定義的 `inbox`。「待確認收件」依確認期限舊到新排序。**

**現況**：`ListReports` 只能篩狀態、類別、店家，固定新到舊（`governance.proto:22`、`:220-229`）。「待確認收件」（PENDING 而且還沒確認收件）沒辦法篩選：`status=PENDING` 也包含已確認、還沒開始調查的通報。食安通報有 4 小時的確認期限（`acknowledge_due_time`），但在新到舊的清單裡，快到期的那幾筆會被新進的通報往後擠。舊頁面用 client 端合併最多 50 筆、每秒倒數（`violation-reports/page.tsx:135`）。

**業界基準（已查證，見 `step10e-3-baseline-check.md`）**：Zendesk 與 Jira SM 的工作佇列預設依 SLA 剩餘時間排序（最急的在最上面），新到舊只用在「全部」這種瀏覽用的 view。專案內的先例是 `FoodRegistrationInbox`（伺服器定義 `DECIDABLE`／`AWAITING_KYC` 兩個佇列，seller `AGENTS.md:23`）。

**為什麼現有機制存在**：8g 先把 `ListReports` 做成瀏覽用的清單，取代會漏件的跨表分頁。當時還沒有收件匣的消費者。

| 選項 | 取捨 |
|---|---|
| (a) 維持新到舊，client 篩 `status=PENDING`，在列上標出期限 | 不改契約；但分頁之後，快到期的食安通報可能落在第 2 頁，而且已確認、未調查的通報也混在一起 |
| (b) **`ListReportsRequest.inbox`**：`NEEDS_ACKNOWLEDGEMENT`（依 `acknowledge_due_time` 舊到新，沒有期限的排在後面，再依建立時間舊到新）、`AWAITING_INVESTIGATION`、`INVESTIGATING`（舊到新）；沒設 inbox 時維持新到舊加上原本的篩選 | 和食品業者登錄的先例同形；keyset 是 `(due nulls last, created_at, id)`，要一支新的查詢與分頁測試 |
| (c) client 拉多頁之後自己排序 | 就是舊收件匣那種跨頁排序，不成立 |

**約束帳**：「ListReports 新到舊」：SELF-IMPOSED（8g 沿用）。REAL：無。

**建議 (b)**。待確認收件分頁每 30 秒重新整理一次，視窗回到前景時也重讀（沿用舊頁的間隔）。逾時以分鐘為單位顯示，不做每秒的倒數。

### D4. 通報與調查的內部註記

**建議：新增 `report_notes`，只能新增的時間軸，形狀和措施註記相同。**

**現況**：註記只掛在措施上（`CreateMeasureNote`／`ListMeasureNotes`，權限 `seller.measure_notes.*`）。通報只有 `resolution_note`，那是**給通報人看的**（`governance.proto:212-213`、`:236-237`），結案時可以不填。判定不成立的通報，系統裡完全沒有內部的判斷依據；調查期間的聯絡（食藥署、賣家）、轉報前的討論也沒有地方記。舊頁面也沒有內部註記，只有 `adminResolutionSummary`。計畫 :265：「通報與調查是否也要註記，觸發是 10e 做 admin 的通報處理介面。admin 的註記 UI 與文案在 10e 先問 Will。」

**業界基準（已查證，見 `step10e-3-baseline-check.md`）**：Zendesk 每張 ticket 都可以加 internal note，和公開回覆分開；Jira SM 分「內部留言」與「回覆客戶」。DSA 第 16(5)–(6) 條要求通報的處理要勤勉、客觀，並通知通報人處理結果。條文沒有要求內部註記；以內部紀錄證明勤勉與客觀是推論。

**為什麼現有機制存在**：4A-2 把註記設計成只能新增的時間軸，掛在違規上；2c 跟著搬到措施。通報當時沒有處理介面，所以刻意延到這一刀再決定。

| 選項 | 取捨 |
|---|---|
| (a) **`report_notes`**（作者、時間、內容，1 到 2000 字，不能改也不能刪），`CreateReportNote`／`ListReportNotes`，權限 `seller.report_notes.create`／`list`；不帶 etag，也不改通報的狀態；結案後仍可以寫 | 和措施註記同形，UI 元件兩邊共用（措施詳情本來就要在這一刀做註記時間軸）。多一張表、兩支 RPC、兩個權限 |
| (b) 不加，內部判斷寫在後續措施的第一則註記 | 判定不成立或還沒成立措施的通報沒有內部紀錄；調查過程沒有地方記 |
| (c) 結案時加一個必填的內部說明欄 | 只能在結案那一刻記一次，調查期間的經過還是沒有地方記 |

**約束帳**：「註記只掛在措施」（計畫 :265）：SELF-IMPOSED，而且明寫延到這個觸發點再決定。REAL：無。

**建議 (a)**。文案見 D6。個資：註記會跟著通報的保留期限一起清嗎？目前通報只清通報人的聯絡方式（`contact_erase_time`），通報本身與內部註記都保留。註記的提示文字會提醒「不要寫通報人的聯絡方式」。

### D5. 措施修改介面（`UpdateMeasure`）與不能操作的原因

**建議：一個「修改措施」對話框，依伺服器給的 `can_*` 決定欄位能不能改；不能改時欄位反灰，旁邊顯示伺服器給的原因。原因需要在契約新增欄位。**

**現況**：`Measure` 有 `can_update_statement`、`can_extend`、`can_shorten`、`can_end`、`can_revoke`、`can_schedule_review`、`can_decide_appeal`、`can_decide_remediation`（`measure.proto:127-136`），但**沒有不能操作的原因**。不能操作的情況有好幾種，而且有些不直觀：提出申訴之後凍結，只能縮短（`governance.proto:79-81`）；申訴待裁定時不能撤銷（`:90`）；警告不能設結束時間；終止合作要撤銷後重建。賣家那邊已經有「布林值加依序的原因」的先例（`remediation_unavailable_reasons`、`appeal_unavailable_reasons`，2c-2 D1、2c-3）。計畫 :317 要求編輯介面的「能否延長、能否改理由」由伺服器輸出，介面不自己推導。admin 目前沒有任何 UI 呼叫 `UpdateMeasure`。

**業界基準（已查證，`step10e-3-baseline-check.md`）**：Google Merchant API 每個動作都帶 `isAvailable` 加 `reasons[]`（每個原因有 `message`、`detail`、修正用的 `action`），文件要求「shows a disabled button … should also show reasons」。GitHub 必要的 review 或檢查沒過就不能 merge，但 API 只給粗粒度的 `MergeStateStatus`，不是原因清單，所以先例以 Merchant 為主。

**為什麼現有機制存在**：2c-1 先把 `can_*` 做成布林值，給 2c 的「結束」按鈕用；原因只在賣家端做了，因為賣家端是先有消費者的那一邊。

| 選項 | 取捨 |
|---|---|
| (a) 只看布林值：不能用就隱藏 | 不改契約；管理員看不出為什麼不能延長或撤銷，只能去問人或查 runbook |
| (b) **布林值加伺服器的原因**：admin `Measure` 加 `update_unavailable_reasons`（statement）、`extend_unavailable_reasons`、`shorten_unavailable_reasons`、`revoke_unavailable_reasons`，型別是一個共用的 enum（例：`REQUEST_PENDING_FROZEN`、`APPEAL_PENDING`、`NOT_ACTIVE`、`KIND_HAS_NO_END_TIME`、`TERMINATION_RECREATE`），依序排列（結構性的原因在前），和前置檢查共用同一份 model 函式；反灰並顯示原因 | 和賣家端同一種形狀；多一個 enum 與 4 個欄位，外加資料集測試，鎖住「原因為空 ⇔ `can_*` 為 true」 |
| (c) client 依 `requests` 的狀態自己寫說明 | 違反前提 |

**約束帳**：「can_* 只有布林值」：SELF-IMPOSED（2c-1 的範圍）。REAL：無。

**建議 (b)**，範圍只限修改與撤銷這 4 種。`end`、`schedule_review`、兩種裁定的入口本身就是由狀態決定的（待處理的請求就在畫面上），不能用時隱藏即可。觸發條件：管理員回報看不懂為什麼某顆按鈕不見了，再把原因延伸到其他動作。

對話框的行為：理由欄只在 `can_update_statement` 時可以編輯；結束時間在 `can_extend` 時不設上限，只有 `can_shorten` 時上限是目前的結束時間、下限是現在；`update_mask` 只送實際改過的欄位；沒有改任何欄位時送出鍵停用；顯示「賣家會看到新的理由與期間，並產生新的版本」。

### D6. UI 同意（版面、文案、流程）

每一點都沿用既有元件：`food-registrations/page.tsx` 的分頁與 keyset、`MerchantManagement.css`、`StatusBadge`、`Button`、`StateRow`、`GovernanceNoticeBanner`，以及 `lib/a11y.ts` 的 modal 骨架。

1. **側欄**（`Sidebar.tsx:33-39`）：「內容治理」分組裡的「商家違規 / 調查」，改成兩項，移到「賣家 / 商品」分組，放在「食品業者登錄審核」上面：
   - 「商家通報」→ `/admin/violation-reports`
   - 「措施與申訴」→ `/admin/measures`

   「內容審核佇列」不動（K12）。
2. **通報收件匣**：分頁「待確認收件｜待調查｜調查中｜全部」，篩選只留類別（K1）。欄位：店家、類別、嚴重度（還沒確認時顯示「未分級」）、狀態、確認期限、建立時間。期限的顯示：「{HH:mm} 前確認」；逾時時顯示 danger 徽章「已逾時 {h} 小時 {m} 分」。
3. **通報詳情**：指令照狀態依序出現（K3、K4）。
   - 確認收件：對話框，必選「嚴重度」。
   - 開始調查。
   - 結案：「查證屬實」或「不成立」，欄位「給通報人的結果說明（選填，通報人看得到）」。
   - 「建立措施」：開 D1 的對話框，預填店家、類別、來源通報。
   - 食安通報另有兩項：「下載轉報資料（.md）」與「記錄已轉報主管機關」（管道、案號）。已記錄過時顯示「已於 {時間} 記錄轉報（{管道}，案號 {no}）」。
   - 「要求賣家配合」：兩種範本，已經要求過的那一種顯示「已於 {時間} 要求」。

   舊頁的 `window.confirm`／`alert` 全部改成對話框與錯誤橫幅。
4. **措施收件匣**：分頁「待裁定｜複審到期｜生效中｜全部」。
   - 欄位：店家、種類、依據、請求（申訴／補正）、已等候、狀態、建立時間。
   - 「已等候 {N} 天」，0 天顯示「今天提出」；超過時限時顯示 danger 徽章「超過 7 天」。複審到期的列顯示「複審期限 {日期}」。
   - 空狀態：「沒有待裁定的申訴或補正」「沒有到期的複審」。
5. **措施詳情頁**：由上到下依序是
   - 待處理的請求（理由、佐證、提出時的版本；「賣家提出時是第 {r} 版，目前是第 {c} 版」，有差異時並列兩版的理由與期間）
   - 措施內容
   - 依據的認定
   - 版本紀錄
   - 相同身分的其他帳號（沿用 2c-3）
   - 內部註記

   操作列依 `can_*` 顯示：修改、結束、撤銷、延後複審。
6. **裁定對話框**：
   - 標題「裁定申訴」或「裁定補正請求」。
   - 選項：申訴是「接受（撤銷這個措施）」與「駁回」；補正是「接受（結束這個措施）」與「駁回」。
   - 理由欄「裁定理由（賣家看得到）」。
   - 只在接受申訴時出現的勾選：「一併撤銷這個措施建立的認定（引用它們的其他措施也會撤銷）」。
   - 駁回補正時提示：「駁回會計入冷卻，賣家可能要等一段時間才能再請求」。
7. **修改措施對話框**（D5）：
   - 欄位「給賣家的理由」「結束時間」。
   - 原因的標籤（依伺服器 enum）：「賣家已提出請求，只能縮短期間」「申訴待裁定，請先裁定」「措施不在生效中」「警告沒有結束時間」「終止合作要改期間，請撤銷後重建」。
8. **延後複審**：欄位「新的複審期限（30 天內）」「為什麼延後（內部註記，必填）」。
9. **內部註記**（措施與通報共用，D4）：
   - 區塊標題「內部註記」。
   - 說明「只有後台看得到。送出後不能修改或刪除，要更正請再寫一則。不要寫通報人的聯絡方式。」
   - 輸入框 placeholder「寫下調查經過或判斷依據（最多 2000 字）」。
   - 按鈕「新增註記」；空狀態「還沒有註記」；每則註記顯示「{admin ID}・{時間}」。
10. **發票申訴頁**（`:527,554`）：「依建議前往建立違規紀錄」改成「依建議建立措施」；「前往建立違規紀錄」改成「建立措施」。兩者都在原頁開 D1 的對話框，說明文字「要對這個賣家做處分，到既有的違規紀錄頁建立」改成「在這裡建立措施，期間與自動解除由賣家治理處理」。

---

## 切分（每段在同一個 commit 刪掉最後一個消費者已經改完的路由，連同 handler、wrapper、openapi 與 `maxRESTOperations`；契約的新增和它的消費端放在同一段）

| 段 | 內容 | 契約與後端 | 刪除的 gateway 路由 | `maxRESTOperations` |
|---|---|---|---|---|
| 3a 措施 | `/admin/measures` 收件匣、`/admin/measures/[id]` 詳情（請求、裁定、修改、撤銷、結束、延後複審、註記時間軸）；商家詳情的兩張卡連到詳情頁；刪除 `MeasureDetailDialog`、`violations/[id]`；K6、K9、K10、K11 | D2 `decide_due_time`、D5 的原因 enum 與欄位、K5 的 `Measure.store_name`；runbook:135 改一句 | `GET`／`PATCH /admin/violations/:id`、`revoke`、`appeal/decide`（共 4 條） | 492 → 488 |
| 3b 建立 | 建立違規措施對話框（K7），掛在商家詳情與發票申訴頁；刪除 `violations/new` | 無 | `POST /admin/violations`（1 條） | 488 → 487 |
| 3c 通報 | 通報收件匣與詳情、指令、FDA、配合要求、註記；「建立措施」掛 3b 的對話框；K1–K4；側欄；runbook 與設計書 32 | K4 的 `Report.can_*`、K5 的 `Report.store_name`、D3 的 `inbox`、D4 的 `report_notes`（表、2 支 RPC、2 個權限） | 通報 7 條；`routes_governance.go` 的 admin 半部、wrapper、DTO、`governance-wire-contract.test.ts` 與它的 baseline 那一筆 | 487 → 480 |

- 如果 10d 的 K1（4 條）先落地，三段依序是 484 → 483 → 476，而本刀的 3c 負責刪掉整個檔案。每次 rebase 都以 openapi 的實際數目重算（現在是 492，以 python 計算，等於常數）。
- 3a 和 3c 都同時動到契約與 admin，各自做 code-review 與 design-review；3b 只做 code-review。
- 回退：每段都是單一 commit，`git revert` 就能回退。舊路由後面本來就是沒有實作的方法（附錄 A1），回退不會把功能帶回來，也不會造成資料不一致。

---

## 附錄 A：盤點錯誤或過時的地方

1. `inv-backend.md` §1 說這 12 條路由打到舊的 seller-service。實際上 gateway 的 `SELLER_SERVICE_HOST:9941`（`docker-compose-local.yml:70-71`）是單體的 `LEGACY_GRPC_ADDR = :9941`（`backend/Makefile:252-254`）；`internal/legacy/sellerserver/server.go:1-3` 寫明這些方法「stay Unimplemented」；compose 裡也已經沒有 seller-service（計畫:139）。所以 6 個舊頁面現在就已經不能用了。
2. 「非 admin 的 6 條留著」：實際只有 4 條（`routes_governance.go:46,53,60,61`），而且 10d 的 K1 已經決定要刪（`step10d-decision-brief.md:44`），在 `abb690858` 還沒落地。
3. 切分表的 485 已經過時：現在的常數是 492（`openapi_coverage_test.go:27`，openapi 的實數也是 492），刪掉 12 條之後是 480。
4. `step10e-decision-brief.md` 的 D5（`ListViolations` 篩 `appeal_status=PENDING` 當申訴佇列）已經被 2c 取代：申訴現在在措施上，`ListViolationsRequest` 沒有申訴篩選（`governance.proto:546-556`），申訴佇列改由 `ListMeasures(request_state=PENDING)` 提供（`step10e-measure-brief.md` §8）。

## 附錄 B：沒有驗證的部分

- Zendesk、Jira SM、Google Merchant API、GitHub、Amazon、DSA 第 16、20 條的描述：都來自記憶，這次沒有連網。
- `GetFDAExport` 回傳的 `Report`＋`Store`，是否涵蓋舊 `.md` 用到的全部欄位（舊頁 `:156` 的格式）：沒有逐欄比對。
- 措施請求的佐證網址 `/api/files/admin/seller/measure_requests/...` 是否已經被 `/api/bff/[...path]` 放行：KYC 用的是同一個前綴 `/api/files/admin/`（`kyc-application.ts:675`），推定已經涵蓋，但沒有實測。
- 沒有跑任何測試；480 是以目前的 492 做的算術。
- D4 的通報註記要不要跟著保留期清理：通報目前只清聯絡方式，沒有找到「通報本體的保留期限」的規則（沒有搜尋設計書 32 的全文）。
