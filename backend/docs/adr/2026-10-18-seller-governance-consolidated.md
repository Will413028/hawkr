# 單體裡的商家治理（整合版）：通報與案件、認定與措施分開、效果由措施推導、通知走事件

| 項目 | 值 |
|---|---|
| 狀態 | accepted（整合 `2026-10-02-seller-governance-in-monolith.md` 的原決策與 11 次修訂；整合不改任何決策，2026-10-18） |
| 日期 | 2026-10-18 |
| 影響範圍 | `internal/modules/seller`（治理子套件、停權推導）、`internal/httpapi`、`internal/platform/policy`、admin、website、seller-app |
| 取代 | `2026-10-02-seller-governance-in-monolith.md`（本檔取代它，它標 `superseded by` 本檔）；原 ADR 取代 `2026-09-18-seller-governance-schema-and-enforcement.md` 的決策二、三、七、八，決策四（停權推導）現在由措施推導；該舊 ADR 的決策一（治理與內容檢舉分開）、五（partial UNIQUE 去重）、六（公開端點的 IP 限流）仍有效 |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 7b、10c、10e |

為什麼要整合：原 ADR 修訂超過 3 段（`adr/README.md`），讀者要逐段對照前文才知道現況。下面只寫現行決策；括號內的日期是該決策來自哪一次修訂（原決策是 2026-10-02）。前後修訂互相取代的地方，一律以較晚的為準。只描述當時開發資料怎麼搬的 migration 步驟不收（見 seller 的 `CARRYOVER.md` 與舊檔）。

## 脈絡

治理（檢舉、食安通報、違規、申訴）從 seller-service 搬進 seller 模組。舊實作的問題多半來自資料形狀：處分嵌在違規列裡（改到期日不重算停權、六種處分只有兩種有執行者）、檢舉與食安通報分兩張表（admin 收件匣跨表分頁漏筆或重複）、通知送到從未宣告的 exchange、`sellers.rating` 是事件覆寫的副本。

約束：
- 外部：沒有台灣的平台治理法規可循；參照歐盟 DSA 的 Art.16（通報與處理：不需登入也能通報、處理結果告知通報人）、Art.17（每個限制都附理由通知被處分者）、Art.20（申訴）、Art.23（防濫用）；匿名聯絡方式的儲存限制（GDPR 5(1)(e)）。
- 繼承：依賴表 `seller → identity、media`，review 依賴 seller（單體 ADR D2），仍成立。停權以帳號為單位（新 stint 繼承舊 stint 的措施）、只有 `suspension.Recompute` 一個推導點、停權期間是停權事件的身分（計畫 7a），仍成立。
- 不再成立：舊決策二「兩種通報共用表要寫條件式 CHECK」（成本遠小於合併分頁與重複的狀態機）；舊決策七的 advisory lock（店家列就是可鎖的 aggregate 列）。系統從未上線，沒有資料要搬，不受相容性約束。

## 考慮過的方案

| 決定 | 基準（業界做法） | 其他方案與判定 |
|---|---|---|
| 通報入口 | 一個入口（DSA Art.16），通報人是登入買家或匿名聯絡方式，類別決定優先序；代價是 CHECK 表達「通報人二擇一」「FDA 轉報只在食安類別」。✓ | 沿用兩張表（跨表分頁、兩份狀態機）✗；依 D8 刪掉沒有 client 的入口（治理只剩空殼）✗ |
| 通報與處理單位 | 通報與案件兩個資源（Zendesk 的「使用者回覆」與「工單處理」分開）：一次事件對主管機關只轉報一次、對賣家只處理一次。✓（2026-10-18） | 一筆通報就是一個處理流程 ✗ |
| 認定與限制 | 認定（finding）與限制（restriction）是兩個物件，限制引用認定並有理由、期間、申覆與覆核（Meta Account Quality、Stripe Radar、DSA Art.17）。✓（2026-10-12） | 處分嵌在違規列、管理員停權另一張表（兩套解除、版本與申訴規則；管理員停權沒有理由與申訴）✗；保留六種處分（四種沒有執行者，欄位與 CHECK 只是佔位）✗ |
| 停權狀態 | 有時效的限制在檢查時比對到期時間，衍生值不存（GCP IAM Conditions、AWS `DateLessThan`、SpiceDB expiring relationships、JWT `exp`）。✓ | 存推導值、到期由 job 推回（兩份事實會分歧、worker 時鐘偏慢時白跑）✗ |
| 措施的目前值 | SQL:2011 系統版本表：主表存目前值、歷史表存每次被改掉的版本，同一個寫入維持一致。✓（2026-10-14） | 目前值只放版本表（「生效中」的讀取要 join、RISK 那段無法用索引條件排除已結束與已撤銷，讀取量隨歷史成長）✗；用 trigger 補（隱藏的寫入路徑）✗ |
| 通知 | 領域在寫入的 transaction 內發事件，通知系統負責送達與紀錄。✓ | 保留 `seller_notification_log`（通知系統上線前記的是沒送出的通知）✗ |
| 店家評分 | 評價的擁有者維護彙總，只顯示的消費端在呈現層組裝。✓ | 單體接舊 AMQP 事件寫副本（亂序、依賴方向相反）✗ |
| 佐證 | 機敏檔，只經後台擁有者端點讀，每次讀取先記錄再讀（Amazon 侵權通報、Meta、YouTube 的附件只給審查人員與提交者）。✓（2026-10-04） | 公開圖片 ✗；通報機敏、申訴公開（兩套規則）✗；再加 KYC 式理由與限時授權（審案時每件都要做，目前只有 OWNER）✗；短效簽章網址（記不到讀取）✗ |
| 內部說明 | 只新增的註記時間軸（Zendesk internal note），在 aggregate 邊界之外，不動版本。✓（2026-10-11，現掛在措施或案件） | 內嵌在違規並用版本保護（賣家看不到卻讓賣家的申訴 etag 過時）✗ |

## 決策

### 通報與案件（2026-10-02 原決策，2026-10-18 改為兩個資源）

- seller 擁有一張通報表（`reports`）：一位檢舉人的一次送出。通報人是登入的買家或匿名聯絡方式（電話或 email），CHECK 保證剛好一種；匿名不收佐證；登入的檢舉人不能檢舉自己的店。嚴重度不由通報人填。通報沒有狀態欄與嚴重度，有自己的結果：已採取措施、未違規、內容不足、重複、濫報，設定後不能改。
- 案件（`cases`）是平台對「這家店、這個平台判定類別」的處理單位，同時最多一個進行中（`cases_one_open`）。嚴重度（開案時決定）、決定、內部註記、FDA 轉報（`case_referrals`，只在食安類別，`case_referrals_category_check`）與導出的措施都掛在案件上；導出的措施是 `measures.source_case_id` 指向它，或措施依據的認定以它的成員為來源。決定（採取措施或不採取措施）即結案，結案是終態（同店同類別的新通報開新案件），把還沒有結果的成員帶入「已採取措施」或「未違規」。
- 分案把待分案的通報放進案件（這家店這個類別沒有進行中的案件時開新的並決定嚴重度）或直接設定結果；移動等於改這筆的平台判定類別，已是認定來源的通報不能移，通報都移走的案件以 `NO_REPORTS` 自動關閉（不算一次決定）。「已採取措施」只能設在案件裡的通報，而且案件已有作為（有導出的措施，不論狀態，或已有成員的結果是已採取措施）；「不採取措施」只在案件沒有未撤銷的導出措施、也沒有已採取措施的成員時可選，所以每個進行中的案件至少有一個可選的決定。
- 處理狀態與此刻做得到的動作都在讀取時推導、不存：只有 `ReportFacts.Availability`、`CaseFacts.Availability` 回答，API 的 `can_*`、`available_outcomes`、`decision_options` 與指令的前置檢查看同一份結果。
- 去重沿用 partial UNIQUE，只比對還沒有結果的通報，日與週以台北時間為界。去重期間內同一位檢舉人再次送出是**補充**：併成原通報的 `report_supplements` 一列（只新增，每筆通報最多 10 則，匿名超過的內容不存、只計次數），原通報的版本前進，案件不鎖也不改版本，只顯示有補充還沒被看過的提醒。
- 認定的來源通報是「同一家店，結果不是未違規、內容不足或濫報」（`Report.Supporting`），API 的 `can_cite_as_source` 是同一個規則加上店家沒有關閉；建立措施可連到案件（`source_case_id`，同店、進行中，任何依據都能連）。
- 匿名檢舉人的聯絡欄位清成 NULL、不做 hash（保留期限後沒有人需要比對）。清除條件（2026-10-18，取代「沒有結果的不清」）：有結果的在 `greatest(建立 + 90 天, 結果 + 30 天)` 之後；不論有沒有結果，建立滿 365 天（剛好 365 天不清，索引 `reports_contact_cap`）。清理 job 回報只靠上限清掉的筆數，大於 0 記 warn log。
- 事件：每次送出發 `ReportReceived`（新的、補充或丟棄；匿名帶這次送出的 email 給 A8 寄確認信，不落表），每筆通報有了結果發一次 `ReportResolved`。`ReportAcknowledged`、`SellerCooperationRequested` 已刪除；要求賣家配合是案件的詢問（見文末「修訂：案件的詢問」；詢問不發事件），舊的 `report_cooperation_requests` 已刪。
- 對外入口（2026-10-04）：登入的買家以 `SellerReportAppService.CreateReport` 通報任何類別、可附佐證；匿名的入口只收食安（`SellerReportPublicService.SubmitFoodSafetyReport`：它是公共衛生的緊急管道，其他類別要能追蹤與附圖，需要登入），回應沒有內容，新的、補充與丟棄的送出無法區分。兩者只能通報曾經公開過的店（與店家卡片同一個規則；從沒公開過的與不存在的同為 NotFound，公開入口不能拿來探測店家 ID）；額度沿用舊決策六的兩個時間窗（10 分鐘 5 次、1 小時 10 次），匿名以來源 IP、登入以帳號計數。
- 鎖順序：identity 帳號、店家（帳號全部的 stint）、通報（id 升序）、案件（id 升序）、詢問、措施；跨帳號依 account_id 升序。分案、移動、決定與建立措施先鎖店家（同店同類別的開案由它串行）；補充只鎖原通報。
- 升級規則（90 天內第 3 次 medium 升為 high）以店家列的鎖串行，不用 advisory lock，以帳號計（`violations.account_id`），只計 ENFORCED（撤銷的不計，另立的措施以自己的立案時間計入窗口）。

### 政策分類（2026-10-18）

- `hawkr.common.v1alpha1.PolicyCategory`（13 個值）與 `internal/platform/policy` 取代 seller 自己的違規類別；檢舉人選的類別、平台判定類別與認定的類別共用，放 common 與 platform 是因為 A7 的 moderation 也共用（platform 只依賴 platform）。每個值對應賣家合作條款第十一、十六條的依據（DSA 17(3)(e)）。
- 各入口只接受自己的子集，伺服器驗證（`CreateReport` 的類別、`ListReports` 的篩選、`CreateMeasure` 的新認定）：`ReporterSelectable`（12 個，沒有 `PLATFORM_ABUSE`：只有平台看得出來）、`Assessable`（12 個，沒有 `OTHER`：認定的理由指不到條款，所以分案時要改判成有依據的類別或直接設定結果）。`reports_category_check`、`violations_category_check` 各綁自己的子集。client 的子集目前是 admin 常數；第二個 client 要用時改成 proto custom option 由 `uitext-gen` 生成。`UNAUTHORIZED_SALE` 改名 `UNLICENSED_SALE`；賣家端顯示的類別名稱取自共用文案目錄。

### 認定、措施與效果（2026-10-12，其後各次修訂）

- `violations` 只是認定。措施（`measures`）是限制，種類為警告、停權、終止合作（2026-10-15），依據是一到多筆認定、風險類型或契約條款（`CONTRACT`，只配終止）；另有 `measure_revisions`（版本歷史）、`measure_findings`、`measure_requests`（申訴與補正）、`measure_request_evidence`、`measure_identities`，以及內部註記 `governance_notes`（掛在措施或案件，剛好一個；建立措施時第一則必填，只新增、不能改刪）。管理員停權就是 RISK 措施（沒有固定結束時間、必須有複審期限、風險排除時結束）；同一帳號可有多個生效中的措施。公開的違規列表刪除（主流電商不公開賣家處分；公布食安違規是主管機關職責；平台自行公布在誤判時有名譽風險），`public_summary` 是給賣家的處分理由。
- 效果只由措施推導，不建效果表、不存 `stores.suspension`。「生效中」只有一個式子：`ENFORCED ∧ kind IN (SUSPENSION, TERMINATION) ∧ starts_at <= now ∧ (ends_at IS NULL ∨ ends_at > now)`（2026-10-15 加入不可變的 `starts_at`，取代「`created_at` 是開始」）；`EffectiveMeasures`、`EffectiveMeasuresAll`、`ScheduledMeasureIDs` 與 `model.StateOf` 同一個定義，由資料集測試鎖住。`MeasureState` 含 `SCHEDULED`，`StateOf` 順序是已撤銷、警告、`ENDED`、`SCHEDULED`、`ACTIVE`。停權期間（`suspension_periods`）與 `StoreSuspended`／`StoreUnsuspended` 只由 `Recompute` 產生：措施寫入的 transaction 內呼叫，到期由排在 `ends_at` 的 job 以 `max(ends_at, now)` 判斷，對帳 job 修補不一致；job 晚到只讓事件晚到，不影響權限。
- 目前值（`statement_to_seller`、`ends_at`）存在 `measures`，`measure_revisions` 是不可變的快照歷史（含目前那一版）；寫入新版本只有 `InsertMeasure`（第 1 版）與 `AdvanceMeasureRevision` 兩條各以一個 SQL 語句（CTE）雙寫的路徑，`current_revision` 經可延遲的複合外鍵指向歷史（2026-10-14）。所有「生效中」的讀取只讀 `measures`，由部分索引 `measures_active_timed`、`measures_active_open` 支撐；`measures_period_check` 讓警告一定沒有 `ends_at`、依據認定的停權一定有，所以沒有固定期間的只有 RISK 停權與終止，不需要跨表約束。`ends_at` 為 NULL 時外鍵不檢查，所以目前值等於歷史那一版由單一寫入路徑與測試維持。
- 版本表存在的理由（2026-10-11）：申訴與補正提出時記下措施當時的版本（`measure_requests.revision`，複合外鍵指向歷史，之後再改也不變）；admin 讀單筆措施時內嵌全部版本，裁定者對照賣家提出時看到的內容與目前的內容。沒有改變的修改不寫入、不產生新版本。
- 兩個 etag：後台是 `measures.version`，賣家是 `current_revision`；註記不改任何 etag（它在 aggregate 邊界之外，先例是 `evidence_accesses`、`kyc_reviews`，所以措施不內嵌註記，另以清單方法分頁讀）。撤銷帶 `revoke_findings` 時連帶撤銷引用同一批認定的其他措施（`FINDING_REVOKED`），待處理的請求以 `CLOSED` 結案。建立措施可帶 `request_id`（AIP-155）。

### 申訴、補正與凍結（2026-10-12、2026-10-15）

- 賣家的 `can_appeal` 與前置檢查共用同一個函式，client 不自己推導；申訴的可用性由伺服器推導（`AppealUnavailableReason`，依序 `REVOKED`、`ALREADY_APPEALED`、`APPEAL_WINDOW_ENDED`、`REQUEST_PENDING`）。只有終止有申覆期限（見終止合作），其他種類（含已結束的措施）不設期限。
- VIOLATION 是懲罰型（期間本身就是處分），只有申訴，同一措施申覆一次為限；**接受申訴即撤銷措施**（`APPEAL_ACCEPTED`，同一個 transaction 解除效果，可一併撤銷措施自己建立的認定），駁回時措施不變。RISK 是條件型，可補正後請求解除（含條款第 17.5 條的外部要求，可重複）；申訴與補正合計同時最多一筆待處理（`measure_requests_one_pending`），DB 以 `basis` 與複合外鍵保證補正只掛風險措施。**接受補正是措施結束**（寫一版 `ends_at` 是現在、發 `MeasureEnded`），不是撤銷，認定不動。
- 請求以 `CLOSED` 結案時記 `close_cause`：`REVOKED`（措施撤銷，一定有 admin）或 `ENDED`（措施已結束、補正失去標的，可以是系統結案）。`DecideRemediation` 不帶 `request_id`：同一措施至多一筆待處理請求，etag 保證裁定的是管理員看到的那一筆，而且兩個 client 的執行期會把名為 `request_id` 的欄位當冪等鍵自動重試。
- 凍結：有申訴（待裁定或已駁回）後，改理由或延長（加重）回 `failed_precondition`，只能縮短或撤銷；撤銷在申訴待裁定時也回 `failed_precondition`（只能經 `DecideAppeal` 結束）。待處理的補正凍結加重與結束（改理由、延長、`EndMeasure`：先裁定，接受補正就是結束）；縮短、撤銷與接受補正不受限。理由：只在駁回後才凍結，待裁定期間的加重會在駁回後變成不能再申覆的措施；有新事實另立新措施（可再申覆），原理由有誤就依實情裁定申訴、接受即撤銷。`failed_precondition` 不帶欄位，也不另加 `ErrorReason`（client 不依原因分流）。
- 冷卻（條款第 17.4 條）：同一措施被駁回的請求（申訴與補正合計，`CLOSED` 不計）累計超過兩次後，每再被駁回一次起算冷卻 7、14、28 天（上限 28 天），由請求推導、不存，`next_remediation_time` 與 `rejection_cooldown` 由伺服器給。補正只在措施生效中才有標的，由每個需要請求名額的寫入路徑強制（`CanAppeal` 不算已結束措施上待處理的補正、`SubmitAppeal` 先以 `CLOSED`（`ENDED`）結案、`DecideRemediation` 要求生效中）；請求對外狀態在讀取時由 `Measure.EffectiveRequest` 推導，儲存值由 `measure` 套件的週期 job `seller.measure_close_ended_requests` 收斂，停權的到期 job 與對帳不管請求。

### 終止合作（2026-10-15，條款第 21 條）

- 終止一定有事由（`termination_ground`），永遠不是 `RISK`，所以沒有複審期限、補正與 `EndMeasure`；`NOTICE` 與 `LINKED_ACCOUNT` 一定是 `CONTRACT`，`MATERIAL_BREACH` 一定是 `VIOLATION`，其他事由有認定是 `VIOLATION`、沒有是 `CONTRACT`；這些組合與 `(source_measure_id IS NOT NULL) = (事由是 LINKED_ACCOUNT)` 由 CHECK 保證。建立時沒有 `ends_at`，API 不接受終止的 `end_time`（只有系統在店家關閉時為預告終止寫入）。預告終止（第 21.2 條）的 `starts_at` 是 `created_at` 加 720 小時（固定時數：timestamptz 加日數依 session 時區計算）。
- 申覆期限是生效後 4416 小時（184 天，任何連續六個曆月的最大長度，不短於條款的六個月與 DSA 20(1)），推導、不存（`Measure.appeal_deadline_time`）；期限內含預告期間可申訴，期限內提出的申訴期滿後照常裁定。
- 生效中的終止在沒有未結訂單時由 job `seller.termination_effect` 關店（每小時 `seller.reconcile_terminations` 保底，`order.completed` 事件加速；River 去重的狀態含 running，錯過的由掃描補，方向是晚關）：未結訂單在 transaction 外問且 fail-closed，關店的 transaction 先鎖帳號與帳號全部的 stint、重讀措施再判斷一次；「待關閉」不存。
- 關店可逆，不可逆的點是退場（關店後的第一段清理，個資在第二段保留期滿才清）：關店不釋放 logo、封面與 media 引用，relay 不在關店發 `seller.deleted`；退場時才釋放並發 `StoreRetired`（relay 轉成 `seller.deleted`）。撤銷終止的三條路徑（`RevokeMeasure`、接受申訴、`FINDING_REVOKED` 連帶撤銷）在同一個 transaction 恢復店家（`closing.Reopen`：同一個 `store_id`，結束目前的關店紀錄、重新授予 SELLER、重新公告可曝光、發 `StoreReopened`、補排被放棄的 PayUni 綁定），條件是造成關店的那一筆（`store_closures.closed_by_measure_id`）已撤銷、撤銷之後帳號沒有其他生效中的終止；帳號已開新店、帳號在等待刪除或保留期已滿時只撤銷。恢復與否看撤銷之前的資料（`ReopenPlan` 在改動任何請求之前決定），所以期滿前提出、期滿後裁定的申覆仍能恢復。
- 保留期與 legal hold 見 `2026-10-19-seller-kyc-payuni-consolidated.md`「關店與個資保留」與它的 2026-10-20 修訂（關店後兩段清理：30 天退場、184 天清個資；legal hold 擋兩段）（2026-10-15 修訂當時在 `2026-10-02-seller-kyc-payuni-in-monolith`）：`TERMINATION` 關閉的 stint 保留到 `greatest(關閉後保留期, 未撤銷終止中最晚的申覆期限)`（已撤銷的終止不延長），其他關閉為保留期（2026-10-20 起兩段：關店後 30 天退場、184 天清個資）；帳號上有終止的待處理申覆（或帳號上的關聯終止其來源有待處理申覆）時，帳號全部已關閉的 stint 兩段都不清理。時間條件只有 `RetireDueStores` 一支、保存條件只有 `StoreHolds` 一支，清理與恢復共用。
- 第 21.2 條在單一關店路徑結束：`closing.close()`（賣家自己、終止、刪帳）把帳號上尚未結束的預告終止寫一版 `ends_at = 關店時間`（作者是系統，`system_cause = STORE_CLOSED`）、發 `MeasureEnded` 並重算停權，否則 `stint.Derive` 會永遠擋住重新申請。第 21.3 條措施永不結束、只能撤銷，以 `stint.Derive` 擋同帳號開店、以措施的身分快照擋其他帳號以同一身分證號或統編送 KYC；快照逐措施在建立時取（停權與非預告的終止），保留到措施撤銷。
- 關聯帳號（`LINKED_ACCOUNT`）：措施詳情的 `linked_accounts` 反查身分快照命中的其他帳號，管理員對每個帳號另建一筆終止；來源必須是另一個帳號、未撤銷、事由不是預告或關聯帳號的終止（不成鏈），伺服器再驗證這個帳號的 KYC 身分確實命中來源的快照，同一來源對同一帳號同時只能有一筆未撤銷的（部分唯一索引；撤銷後可以依新的事證再建）。來源經上述三條路徑撤銷時同一個 transaction 連帶撤銷（`SOURCE_REVOKED`），待處理請求以 `CLOSED` 結案、申覆期間內的關聯店家恢復；關聯措施自己的申訴成立只撤銷它。
- 撤銷涉及終止時，先不加鎖預讀要碰的帳號（措施、連帶撤銷的對象、以它們為來源的關聯終止），依 account_id 升序鎖 identity 帳號與帳號全部的 stint，鎖後重讀；範圍變了就整個 transaction 重來。identity 帳號鎖先於店家列（恢復營業要授予 SELLER）。
- 管理員直接關店整組刪除，平台關店一律經終止措施（理由與作者在措施裡）。事件 `MeasureCreated` 加 `StartsAt` 與事由（不新增 `TerminationScheduled`：沒有訂閱者），新增 `StoreReopened`、`StoreRetired`；legacy 契約把終止視為管理員停權，標籤優先用終止事由。

### 佐證（2026-10-04）

- 通報與申訴的佐證是機敏檔：轉正時要求 media 回報 `is_secure`，公開的佐證在寫入之前拒絕。後台以 (通報或違規, 位置) 定址的兩條 `/api/files/admin/seller/…` 端點讀，各宣告一個權限（ADR `2026-10-04-admin-roles.md`）；通報與申訴共用一張 append-only 的讀取紀錄表，先寫紀錄、commit 之後才讀 media。API 只回位置與檢視網址，不回 asset ID 或 object key；通報清單不帶匿名通報人的聯絡方式（只在讀單筆時出現）。提交者自看與賣家看通報佐證目前不提供。
- 認定與措施的修改紀錄由措施的版本歷史（`measure_revisions`）取代 2026-10-04 的 `violation_changes`；讀取紀錄（`evidence_accesses`）沒有對外的列表方法。

### 通知與評分

- 處分、撤銷、申訴的送出與裁定、通報的收件與結果，都在寫入的 transaction 內發模組事件，A8 的 notification 訂閱並保存送達紀錄；`seller_notification_log` 不搬。
- seller 不存評分，也不處理 `review.created`／`review.hidden`；店家頁的評分由 `internal/httpapi/seller` 組裝，推薦排行由 `discovery` 自行決定是否維護讀取模型。

## 後果與重新評估

- A8 之前沒有推播：賣家看到措施要打開頁面，通報人不會收到結果。A8 的 notification 要訂閱本模組的治理事件。
- 目前值與歷史是兩份資料，一致性靠單一寫入路徑與測試；若出現第二條寫入路徑，重新評估。
- 佐證：B1 之後只換 `media.ReadSecure` 背後的 adapter，由 media 回報型別與大小，擁有者在轉正時限制型別（圖片與 PDF）與大小；B1 之前若出現大於 10 MB 的佐證或後台讀取變慢，先在 client 限制大小。改成理由加限時授權的觸發：佐證開始收身分證件類文件，或第一個非 OWNER 的角色出現、要區分辦案與只讀。讀取紀錄的列表方法：第一個非 OWNER 的角色出現、需要在後台稽核時加。
- 提交者自看：client 要顯示自己交的佐證時，加以「本人或本店」授權的擁有者端點，同一張讀取紀錄表照記。賣家看通報材料：案件的詢問需要給賣家看材料時，另外決定遮蔽或轉述（避免認出通報人）。佐證含個資卻沒有保留期限：在隱私政策定稿或 A8 時與其他稽核文字一起評估。
- 再新增請求種類時加 `RequestKind` 值與對應的 `Can*`。
- 出現罰款、部分停權等需要執行者的措施時，加新的種類與它的執行點；出現依評分排序的需求時，由那個消費端建讀取模型。
- 條款第 17.4 條（2026-10-15 版）、第 21.2–21.4 條與賣家身分認證告知事項改版時不要求已開店的賣家重新同意；第 21.2–21.4 條與告知事項未經律師審閱。

## 修訂：案件的詢問（2026-10-19，A3b 步驟 10e 第 3c-b 刀；設計 `plans/2026-09-29-a3-buyer-seller/step10e-3cb-design.md`）

這一段取代上文「事件」一列裡「要求賣家配合是案件的詢問，3c-b 做」的預告；轉報 v2 與主管機關來函的修訂接在它後面（3c-b-2、3c-b-3）。

- **詢問（`case_inquiries`）掛在案件上**，不掛在通報上（一次事件的多筆通報只詢問一次）；複合外鍵 `(case_id, store_id)` 保證詢問的店家就是案件的店家。對象只有賣家，不加對象欄位（出現第二種對象時再加，並把 `case_inquiries_one_open` 改成每個（案件, 對象）一筆）。種類 `RECORDS`（提供紀錄：條款第 11.3 條）與 `EXPLANATION`（請說明）。每個案件同時最多一筆還沒結束的詢問（partial UNIQUE `case_inquiries_one_open`）；要同時要紀錄又要說明，寫在同一則說明裡。
- **期限**是存下來的 `due_at`（日曆時間；工作日行事曆還沒有）：建立時預設 72 小時，可指定在建立後 24 小時到 14 天之間，建立後不能改。**回覆**是一筆詢問一則，回覆即結束（不是對話），欄位在詢問列上，CHECK 保證回覆與撤回互斥；過了期限仍可記回覆（逾期只是待辦）。要再問就開新的一筆。回覆在 10f（賣家端畫面）之前只由管理員記下系統外收到的內容：管道（電話、email、通訊軟體、其他）、**收到的時間**（不早於詢問建立、不晚於現在）與記下的時間，附件最多 5 份。
- **只轉述，不轉交**：詢問只帶管理員寫給賣家的說明，不附通報原文、佐證與通報人；賣家的回覆與附件也不轉給通報人（DSA 前言 54 的遮蔽，反向同理：賣家的紀錄含營業資料與第三人個資）。附件是機敏檔，規則同通報佐證：轉正時要求 `is_secure`，以 `media.Uploader{ID: 管理員, Admin: true}` 轉正（admin 的機敏上傳經單體轉給 gateway 的 `upload_sessions` 與 `secure_upload`，已用 media-service 驗過這條鏈），media 引用以詢問 ID 登記在 `GOVERNANCE_EVIDENCE` 之下，後台只經 `/api/files/admin/seller/case_inquiries/{inquiry_id}/evidence/{position}` 讀，先寫 `evidence_accesses`（擁有者加 `inquiry_id`）再讀。
- **撤回**：管理員可以撤回還沒回覆的詢問；案件結案（決定，或通報都移走而以 `NO_REPORTS` 關閉）時，還沒結束的詢問在同一個 transaction 由系統撤回（`CASE_CLOSED`，作者是結案的管理員）。結案不因有還沒回覆的詢問而擋下。
- **推導，不儲存**：案件的「等賣家」＝進行中而且有還沒結束的詢問；「待決定」＝進行中而且不在等賣家，兩者在進行中時剛好一個成立。「待決定」的開始時間是 `max(開案, 最後一筆詢問被回覆或被管理員撤回的時間)`，用**收到**的時間而不是記下的時間：晚記下的回覆讓案件回到待決定時，期限已經用掉一部分甚至逾期，這是刻意的——期限算的是平台拿到資訊之後的處理時間，不因晚登記而延長；系統撤回（結案）不重新起算。詢問的狀態（等賣家回覆、已回覆、已撤回）與逾期（還沒結束而且現在不早於 `due_at`）同樣在讀取時推導；`CaseFacts.Availability().CreateInquiry` 與 `CaseInquiry.Availability()` 是唯一回答「此刻能做什麼」的地方，API 的 `can_*` 與指令的前置檢查看同一份結果。
- **店家關閉不擋詢問**：帳號還在，條款第 11.3 條的義務與案件都還在，關店也不該是躲避詢問的方法；只有店家個資已清理（`pii_purged_at`，沒有聯絡資料可以問）時擋下新的詢問，已開著的詢問在關店與清理時都不動，照常逾期、可以記回覆或撤回。
- **版本與冪等**：建立、記下回覆、撤回都讓案件的 `version` 與 `updated_at` 前進（它們改變「可以決定」與決定的依據）；詢問的指令本身不帶 etag：建立帶 `request_id`（AIP-155，範圍是（建立的管理員, request_id），同 `CreateMeasure`；重送先於有狀態的驗證），回覆與撤回是一次性的轉移，重送回原本的結果（`already_recorded`、`already_withdrawn`）。
- **事件**：詢問不發事件（2026-10-20 修正輪：3c-b-1 原本預先發出 `CaseInquiryCreated` 等 A8 訂閱；它沒有消費者，依同一個模組的規則「消費者出現時再加它需要的事件」刪除，A8 通知賣家時再加）；回覆與撤回同樣不發，讀取時推導得到。
- **鎖順序**：店家列 → 案件列 → 詢問列（結案先鎖店家，所以不會出現結案之後才寫進回覆的詢問）。
- 佐證保存期：詢問的說明、回覆與附件沿用「治理資料目前沒有保存政策」（見上文「後果」的佐證保存期），列進 I-legal 的檢查清單。

## 修訂：轉報紀錄加追蹤日期、主管機關的回覆與不轉報（2026-10-19，A3b 步驟 10e 第 3c-b-2 刀；設計同上 §1.3、§2.2）

這一段取代上文「FDA 轉報每個案件只記一次、記過不能修改」裡「不能修改」的絕對說法：轉報本身仍只記一次、不覆寫，但之後可以只新增主管機關的回覆並調整追蹤日期。

- **不轉報也是一筆轉報紀錄**（`case_referrals.outcome` 是 `REFERRED` 或 `WAIVED`，同一列）：每個案件至多一列，所以「記錄轉報」與「標記不轉報」互斥由主鍵保證，CHECK 保證 `REFERRED` 有管道、`WAIVED` 有原因（1–2000 字）且沒有管道、案號與追蹤日期。待轉報的判定式（沒有轉報紀錄）不改，不轉報因此自然離開待轉報。標記過再標記回原本的紀錄（`already_waived`）；已轉報後標記、已不轉報後記錄轉報都是錯誤（`failed_precondition`），不是 already。決定之前就可以標記。兩種都不要求先決定。
- **追蹤日期**（`follow_up_on`，台北日期）：到了這天還沒收到主管機關的回覆就聯絡主管機關。記錄轉報時不設，預設是轉報日（台北）加 30 天，明確設定時不早於今天（台北）；轉換前就有的紀錄是 NULL（轉換不編日期）。這刀只存與回報，不進待辦（3d 決定要不要把「追蹤逾期」做成待辦種類）。「追蹤逾期」由伺服器在讀取時推導（2026-10-20 修正輪，`FDAReferral.follow_up_overdue`）：有追蹤日期、今天（台北）晚於它，而且設定它（`follow_up_set_at`：記錄轉報，或記回覆時換成下一個）之後沒有記下回覆——記回覆時換成下一個追蹤日期，逾期就以新的日期重新判斷。
- **主管機關的回覆**（`case_referral_replies`，只新增）：移送通知與結果通知可以是兩則；內容 1–2000 字、收到的日期（台北，不早於轉報日、不晚於今天）、記下的時間與管理員。**只能掛在已轉報的列由 DB 保證**：回覆存一欄固定為 `REFERRED` 的 `outcome`（CHECK），以 `(case_id, outcome)` 複合外鍵指向 `case_referrals` 的 `UNIQUE (case_id, outcome)`（外鍵不能帶常數，所以存一欄；先例是 `case_referrals` 以 `(case_id, category)` 加 CHECK 只掛食安案件）；`Availability` 先擋並給句子（`NO_REFERRAL`、`REFERRAL_WAIVED`），DB 是保底。記回覆時可以同時取代追蹤日期（不早於今天）或清掉（不再追蹤），兩者都不設時不變。
- **欄名**：兩種紀錄都是「記下的時間與管理員」，所以 `referred_at`、`admin_id` 改名 `recorded_at`、`recorded_by_admin_id`（上線前沒有相容性約束）；回覆表同名。
- **版本與鎖**：標記不轉報與記回覆都讓案件的 `version` 與 `updated_at` 前進，不帶 etag（只新增或一次性轉移）；記回覆帶選填的 `request_id`（AIP-155，範圍是（記下的管理員, request_id），2026-10-20 修正輪：原本「重送會多一則，由送出中狀態防止」只處理連點，沒有處理回應遺失之後的重送）；鎖順序同記錄轉報：店家列 → 案件列。不發事件（沒有消費者）。
- **移動最後一筆通報**：只有已轉報的案件擋下（句子是「已記錄轉報」）；標記不轉報的案件空了照樣以 `NO_REPORTS` 關閉，不轉報的原因留在案件上。
- 新資料（不轉報的原因、主管機關的回覆）沿用「治理資料目前沒有保存政策」，列進 I-legal 的檢查清單（設計 B31）。

## 修訂：主管機關來函、保存期限與措施的來源來函（2026-10-19，A3b 步驟 10e 第 3c-b-3 刀；設計 `step10e-3cb-design.md` §1.4、§1.5、§2.3、§2.4）

- **來函是獨立的收件**（`authority_orders`，不是通報）：機關、文號、收文時間（管理員填）、種類（限制或移除商品、調取資料、暫停帳號、其他）、內容摘要、期限（依種類 48／72 小時，可自訂 1 小時到 30 天）、指名的對象、完成（時間、管理員、處理摘要）。對象（`authority_order_targets`）是**任何帳號**（identity 驗證存在，含已刪除的墓碑），選填店家（必須屬於帳號；指定時只算那一家，同帳號的新 stint 不在指名內）與商品；`UNIQUE NULLS NOT DISTINCT (order_id, account_id, store_id)`，整個帳號加上它的店家是重疊，拒絕（`authority_order.target_overlap`）。帳號在 seller 沒有店家時照樣登記（`no_seller_data`），買家與帳號資料的保存尚未支援（Q3 (a)：identity 的 hold 不在這刀，觸發是第一份指名非賣家帳號的來函或 Q4 那一步）。來函建立後不能改。
- **保存期限**（`hold_until`，預設收文後 4416 小時 = 184 天）：指名的帳號或店家在 seller 的個資在這之前不被關店後的清理刪除；只能延長（`authority_order_hold_extensions` 只新增，前值、新值、原因、作者；`hold_until` 等於最後一列的 `to_until`）；完成不解除保存；已完成或已到期也能延長。調取資料的來函完成時，保存期限延到 `max(目前, 提供時間 + 4416 小時)`（Q5 (b)，記一列 `DATA_PROVIDED`；提供時間必須在 [收文, 現在]，只有調取資料的來函可以帶）。
- **保存只擋刪除、不改變業務結果**（2026-10-20 修正輪改寫，原本的 `apply_preservation_hold` 參數刪除）：保存條件只有一支查詢 `StoreHolds`（每家店回 `appeal_hold` 與 `preservation_hold`），清理在鎖住候選之後才問它；候選查詢只有時間條件（關店後的兩段清理是 `RetireDueStores` 的兩段，見 `2026-10-19-seller-kyc-payuni-consolidated.md` 的 2026-10-20 修訂），以 `store_id` 做 keyset，被保存的店不會讓批次一直選到同一批。`appeal_hold` 擋兩段清理，也讓恢復營業（`closing.ReopenPlan`）視為保留期還沒過；`preservation_hold`（帳號上有沒撤銷登記、`hold_until` 晚於現在的來函指名，不指定店家或就是這家店）擋兩段清理與食品登錄審核紀錄（關店 2 年）的刪除，恢復營業不看它。保存是第三方（警察、食藥署）的保存義務：被保存而保留期已滿的店家，撤銷終止時和沒有來函的店家一樣只撤銷紀錄、不恢復營業，只是還沒被清理（業界的 legal hold 只凍結刪除；`TestReopenIgnoresTheAuthorityHold`）。食品登錄審核紀錄的刪除兩步：`ExpiredFoodRegistrations` 找候選並鎖住登錄列與店家列（keyset），鎖住之後以候選的店家問 `StoreHolds`，`DeleteFoodRegistrations` 只刪沒有被保存的。
- **寫入保存條件的一方先鎖被指名帳號的店家列**（登記、完成、延長：`stint.LockAccounts`，account_id 升序），再鎖來函列；與清理的「鎖住候選之後再判斷一次」合起來，READ COMMITTED 下不會有清理看不到保存的窗（證明見設計 §2.4；`TestPurgeRechecksTheAuthorityHoldAfterLockingTheCandidates`、`TestFoodReviewDeletionRechecksTheHoldAfterLocking`、`TestAuthorityOrderRegisteredAfterThePurgeShowsAlreadyPurged`）。登記時間在鎖之後才取；個資在登記之前就清掉的店家，涵蓋狀態是 `PURGED_BEFORE_REGISTRATION`（來不及，要回覆機關資料已依保留期限刪除）。
- **告知被指名者**（Q1 (a)，DSA 9(5)、10(5) 的基準，機關要求時可延後）：完成來函時每個對象必填一筆告知——已告知（時間與方式）、機關要求暫不告知（必填向機關確認的日期，到期是 3d 的待辦「告知被指名者」，開始時間是該日台北 00:00，期限再 3 天；之後以 `RecordAuthorityTargetNotice` 改成已告知或寫新的日期，被取代的值留在 `authority_target_notice_history`）、由措施告知（只限暫停帳號、限制或移除商品，而且這個對象已有這份來函導出的措施，驗證用的導出定義與 `ListMeasures(source_authority_order_id)` 相同）。「來函完成 ⇔ 每個對象都有告知」跨兩張表，由模組在完成的 transaction 內保證。
- **措施的來源來函**（`measures.source_authority_order_id`）與認定引用（`finding_authority_orders`，一個認定最多 5 筆）：來函必須指名這個措施的帳號（指名店家時只算措施的那一家）。**這修訂了 rb §2 第 1 點「可以推導，不加欄位」**：RISK 停權與 CONTRACT 終止沒有認定，只經認定推導不到來函；做法與理由同案件的 `source_case_id`（3ca C20）。`ListMeasures.source_authority_order_id` = 明確連結 ∪ 認定引用（三支清單查詢一致）。
- **冪等鍵**：`CreateAuthorityOrder` 的 `request_id` 範圍是（登記的管理員, request_id），同 `CreateMeasure`。版本衝突的 `conflictTarget` 新值 `authority_order`。權限 7 個（`seller.authority_orders.{create,list,get,complete,extend_hold,record_target_notice,revoke}`）。
- **撤銷登記**（2026-10-20 修正輪，Will 2026-10-19 決定 B；取代上文「登記錯了：完成並在摘要寫明，再登記正確的一筆」，那樣做錯誤的保存仍生效至少 184 天，沒有路徑解除）：`RevokeAuthorityOrder` 只在完成之前可以（`can_revoke` 與不能的原因由伺服器給），必附原因、帶 etag；撤銷之後保存解除（`StoreHolds` 不看已撤銷的來函）、不在處理中（清單 `OPEN` 與 3d 的「主管機關來函」排除）、不能完成、延長保存或記下告知，也不能再當作措施的來源（建立措施以 `FOR SHARE` 讀來函列，撤銷以 `FOR UPDATE` 鎖同一列，兩者串行）；紀錄（時間、管理員、原因）保留，已導出的措施不受影響。冪等以管理員為範圍：已由同一位管理員撤銷時回原本的紀錄（`already_revoked`，不比對 etag 與原因），已由別人撤銷回 `failed_precondition`。撤銷不鎖店家列：解除保存只會讓清理更早可以進行，清理在鎖住候選之後才問保存，看到撤銷之前或之後都正確。業界的 legal hold 也可以釋放並留下稽核紀錄（Microsoft Purview、Google Vault）。同一份公文不加 `(agency, document_no)` 的唯一性（分批指名不同對象時要分開登記）。
- **可用性由伺服器給**（2026-10-20 修正輪）：每個對象的 `can_record_notice`（來函已完成、沒有撤銷，而且目前是暫不告知）與原因；對象範圍內每家店的 `can_create_measure`（店家沒有關閉）與 `can_create_termination`（同 `GetStore.can_create_termination` 的規則），與 `CreateMeasure` 是同一份規則（`TerminationTarget.AllowsMeasure`／`BlockFor`），撤銷登記的來函都不行。
- 來函與其子表的資料沿用「治理資料目前沒有保存政策」，列進 I-legal 的檢查清單（設計 B31）；來函暫時留在 seller（設計 B32，觸發同 Q3）。


## 修訂：FDA 轉報刪除（2026-10-28，ADR `2026-10-27-general-ecommerce-scope-cut` D3；計畫 `plans/2026-10-27-ecommerce-scope-cut.md` 步驟 4）

示範品類改成一般電商，食安法的轉報義務不成立。本 ADR 的轉報（「通報與案件」的轉報、2026-10-19「轉報紀錄加追蹤日期、主管機關的回覆與不轉報」修訂整段）不再適用：`case_referrals`、`case_referral_replies`、`RecordFDAReferral`、`WaiveFDAReferral`、`RecordFDAReferralReply`、`GetFDAExport`、待辦種類 `CASE_FDA_REFERRAL` 與案件的 `fda_referral_deadline` 一併刪除；案件的可用性少了記錄轉報、不轉報、記回覆與匯出，「已記錄轉報的案件最後一筆通報不能移走」也跟著拿掉。通報（含檢舉人選的食安類別與它的 4 小時分案期限）、案件、處分、申訴與主管機關來函不變；清理與來函保存的鎖順序不變（清理的參與者剩兩段清理）。

## 修訂：內容下架措施與社群認定類別（2026-10-29，A7 步驟 A7-1c；計畫 `plans/2026-10-18-a7-social.md`、A7-D7）

社群內容（貼文、留言、討論區留言）的下架是店家的處分，要有理由、可以申覆，所以沿用本 ADR 的認定、措施與申覆，不另開一套下架機制。

- **措施種類 `CONTENT_TAKEDOWN`**：不限制帳號，不算 `Restrictive`，不進停權期間與曝光；建立即生效、沒有結束時間；依據一定是認定；解除只走申覆（補正與結束本來就只給 RISK 依據）。對象存在 `measure_contents`，建立後不可修訂，要改就撤銷再建立（不可變，因為對象是處分「下架了什麼」的事實，改了等於改寫處分）。逐項特例與理由在 seller `CARRYOVER.md`「A7-1c」。
- **社群類別進認定與案件**：`policy.Assessable()` 是全部除了 `Other`，含 `SPAM`、`HARASSMENT`、`HATE`，`violations` 與 `cases` 的類別 CHECK 跟著改；檢舉人在 seller 通報可選的子集不含它們。認定可引用 moderation 的檢舉（`finding_moderation_reports`，不透明的外部參照，不加 FK，因為單體規則 2 不建跨 schema FK；不合併進 `finding_reports`，免得失去它對 `seller.reports` 的 FK）。升級規則仍是單一累計，社群類別的 MEDIUM 照樣計入。
- **同 transaction 的建立入口 `CreateInTx`**：給 moderation 讓處理與措施同生共死；加入呼叫方的 transaction、不收 `request_id`。
- **`request_id` 流程改寫**：舊流程在同一個 use case 內「先插入、撞唯一鍵、再重讀」，撞鍵之後在同一個 transaction 繼續下 SQL，違反單體規則 3。改成先鎖目標帳號的全部 stint、再查 `(admin, request_id)` 重送、插入用 `ON CONFLICT DO NOTHING`；落空（只剩目標在不同帳號的並行請求）回 `InvalidInput` 並回滾。關聯終止的唯一鍵改成鎖住後先檢查，索引只是保底。
- **對象不可變**：見第一項；`UpdateMeasure` 沒有對象欄位。
- **social 以 D-sync 複本取代讀取時 join**：原設計是 social 的讀取 join `seller.pub_content_takedowns(at)`。改成 social 的同 transaction 訂閱者維護自己的 `content_takedowns`（建立插入、撤銷刪除、同則內容被多個措施下架時只刪該措施的列），旗標與讀取只看自己的表；`seller.pub_content_takedowns()` 不收 `at`，只給每日對帳核對。理由：social 的旗標與 `reevaluate_*` 是資料庫函式，不能參照 seller 的表，而 join 會讓每個公開讀取多一次跨 schema 探測；撤銷要立即生效（SD13）。與買家刪帳的 `inactive_authors` 同形，差別是可刪除。
