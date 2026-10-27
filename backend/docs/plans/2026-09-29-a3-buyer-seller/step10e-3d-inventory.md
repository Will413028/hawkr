# 3d 盤點（唯讀，worktree hawkr-10e，HEAD 4cc2ec079）

路徑縮寫：R=<10e worktree>；M=R/backend/internal/modules/seller；MODEL=M/internal/model；Q=M/db/queries；SCHEMA=M/db/schema.sql；ADM=R/admin/src。
[量測]=我讀到該行；[推論]=由量測推出。

## 一、十一種待辦（逐種）

### 1. 待分案（通報）
- 判定式 [量測]：`Report.AwaitingTriage()` MODEL/report_case.go:208（= `State()==ReportStateAwaitingTriage`，State 在 :180；SQL 同義：`case_id IS NULL AND outcome IS NULL`，reports.sql AdminReports 的 state='AWAITING_TRIAGE'）。
- 開始：`Report.CreatedAt`（report_case.go:206 註解）。期限現況 [量測]：`Report.TriageDueAt()` report_case.go:211-218，只有 `category == FoodSafety` 才回 `CreatedAt+FoodSafetyTriageSLA`（4h，report_case.go:131）；其他類別 nil。D8 要的 72h 一般通報目前沒有（3-brief D8 表；3cd-brief §2 第12項）。
- proto [量測]：`Report.triage_due_time` governance.proto:390（OUTPUT_ONLY；註解「只在待分案時設」）；emitter httpapi/seller/governance.go:152 `optionalTimestamp(r.TriageDueAt())` -> 只在待分案時輸出（TriageDueAt 內判斷 AwaitingTriage）。開始時間 `Report.create_time` governance.proto:430。
- 索引 [量測]：`reports_awaiting_triage ON seller.reports (created_at, id) WHERE case_id IS NULL AND outcome IS NULL` SCHEMA:344。查詢範例：reports.sql:208 `AdminReportsQueue`（舊到新 keyset，帶店名 JOIN stores）。
- key 會不會變 [推論]：created_at 不變；待分案期間 category 不變（assessed_category 是分案時才設，分案即離開）。穩定。
- 連結：通報詳情 `/admin/violation-reports/{id}`（ADM/app/admin/violation-reports/[id]/page.tsx）。

### 2. 待決定（案件）
- 判定式 [量測]：`Case.AwaitingDecision()` report_case.go:348（`Open() && OpenInquiryID==nil`；Open :334 = `ClosedAt==nil`）。
- 開始 [量測]：`Case.DecisionClockStart()` report_case.go:351-356 = max(OpenedAt, LastClockRestart)；`LastClockRestart` 來自 `CaseDecisionClockStarts`（inquiries.sql:45-53：`max(reply_received_at)` 與 `max(withdrawn_at) FILTER (withdraw_cause='ADMIN')`，GROUP BY case_id），組裝在 M/internal/governance/governance.go:266-276。期限：沒有任何 Go 常數 [量測：rg ReviewPolicy 在 backend 只有 docs 命中]；依嚴重度 24h/3d/7d/14d 只存在文件（3ca-design.md:1450-1456，rb R3 起始值）。
- proto：`Case.decide_due_time` 不存在 [量測]；Case 只有 `open_time` governance.proto:466、`close_time` :468。
- 索引：`cases_open ON seller.cases (opened_at DESC, id DESC) WHERE closed_at IS NULL` SCHEMA:262（DESC 且鍵是 opened_at，而 3d 的鍵是 DecisionClockStart，不同）；`case_inquiries_one_open ... (case_id) WHERE reply_received_at IS NULL AND withdrawn_at IS NULL` SCHEMA:513（NOT EXISTS 用）。沒有一個索引的鍵是「max(opened_at, 最後回覆/撤回)」，UNION ALL 要用 LATERAL/子查詢算 start，無法 index-ordered [推論]。
- key 變動 [推論+量測]：start 只在案件離開待決定（建立詢問）期間改（3cb-design.md:1137）；severity 改動（ChangeCaseSeverity cases.sql:83-90，進行中就可改）會改「期限」但不改 start，且案件仍在佇列 -> due_time 會在佇列中變。這是 key 在佇列期間會變的一個例子。
- 連結：`/admin/violation-reports/cases/{id}`（cases/[id]/page.tsx）。

### 3. 詢問逾期（案件詢問）
- 判定式 [量測]：`CaseInquiry.Overdue(now)` case_inquiry.go:136（`Open() && !now.Before(DueAt)`）；`CaseInquiry.Open()` :122。
- 開始/期限 [量測]：開始 `DueAt`；待辦期限 `DueAt + InquiryOverdueGrace`（24h，case_inquiry.go:84）。詢問自己的期限：預設 `InquiryDefaultDue`=72h :79，範圍 24h–14d :81-82。
- proto [量測]：`CaseInquiry.due_time` case_inquiry.proto:61（OUTPUT_ONLY，「只在 OPEN 時設」，開始是 create_time :58）；emitter httpapi/seller/governance_inquiries.go:42-44 `if i.Open() { out.DueTime=... }` -> 只在義務還在時輸出。測試 governance_inquiries_test.go:111,117,131 鎖住 REPLIED/WITHDRAWN 時為 nil。
- 索引 [量測]：`case_inquiries_open_due ON seller.case_inquiries (due_at, id) WHERE reply_received_at IS NULL AND withdrawn_at IS NULL` SCHEMA:516。
- key 變動：due_at 建立後不能改（authority/inquiries.sql 只有 INSERT 寫 due_at，rg `due_at` inquiries.sql 只命中 :4-5）。穩定。注意待辦的「期限」= due_at+24h，詳情頁的 due_time = due_at（兩個不同的數）。
- 連結：案件詳情 `/admin/violation-reports/cases/{case_id}`；`CaseInquiries.tsx` 有 `data-inquiry-id` (line 68)，沒有 `#inquiry-{id}` 錨點 [量測：rg 在該檔 `inquiry-` 只命中 data 屬性與 import]；brief 寫的 `#inquiry-{id}` 目前不存在。

### 4. 待轉報（食安案件）
- 判定式 [量測]：`CaseFacts.AwaitingReferral()` report_case.go:540（`Category==FoodSafety && Decision==ActionTaken && Referral==nil`）；不轉報也是一列 case_referrals（SCHEMA:419 註解、:421）。
- 開始：`Case.ClosedAt`；期限 +24h 只在文件（3cb-design.md:1137 表、3ca-design.md:1450-1456）；Go 沒有常數 [量測]（rg 找不到 FDAReferralDue）。
- proto：`Case.fda_referral_due_time` 不存在 [量測：rg `fda_referral_due` api 無]；`Case.close_time` governance.proto:468。
- 索引 [量測]：`cases_decided_food_safety ON seller.cases (closed_at, id) WHERE category='FOOD_SAFETY' AND decision='ACTION_TAKEN'` SCHEMA:266（SCHEMA:265 註解寫明「待轉報的存取路徑（3d 的來源）」）+ NOT EXISTS case_referrals（`case_referrals_case_fkey`；case_id 是否有索引未查）。
- key 變動 [推論]：closed_at 結案後不變。穩定。
- 連結：案件詳情。

### 5. 主管機關來函
- 判定式 [量測]：`AuthorityOrder.Open()` authority_order.go:241（`Completed==nil && Revoked==nil`）。
- 開始 `received_at`；期限 `due_at`；預設 `DefaultAuthorityDue(kind)` authority_order.go:36（限制/暫停 48h、調取/其他 72h），範圍 1h–30d :58-59。
- proto [量測]：`AuthorityOrder.due_time` authority_order.proto:109（OUTPUT_ONLY，只在 OPEN 時設，開始是 receive_time :103）；emitter httpapi/seller/governance_authority.go:75-77 `if o.Open()`；測試 governance_authority_test.go:134,151,432。
- 索引 [量測]：`authority_orders_open_due ON seller.authority_orders (due_at, id) WHERE completed_at IS NULL AND revoked_at IS NULL` SCHEMA:622（SCHEMA:621 註解「3d 來源」）；現有 keyset 範例 authority_orders.sql:62-82 `AdminAuthorityOrders`（OPEN 依 (due_at,id)）。
- key 變動：due_at 建立後不改（authority_orders.sql 只在 INSERT :4-6 寫 due_at）。穩定。
- 連結：`/admin/authority-orders/{id}`。

### 6. 告知被指名者（來函對象，WITHHELD 且 recheck_on <= 台北今日）
- 判定式 [量測]：`AuthorityOrderTarget.NoticeFollowUpDue(now)` authority_order.go:202（`Notice.Kind==Withheld && RecheckOn!=nil && !RecheckOn.After(twtime.Day(now))`）；開始 `RecheckStart()` :192（recheck_on 的台北 00:00）；期限 +`AuthorityNoticeFollowUpGrace` 72h（:73）。`twtime.Day` platform/twtime/twtime.go:34。
- proto：沒有 due 欄位 [量測：due_time 只在 3 個檔]；對象 proto `AuthorityOrderTarget` authority_order.proto:138，輸入用 `recheck_date` :192（google.type.Date）。
- 索引 [量測]：`authority_order_targets_withheld ON seller.authority_order_targets (recheck_on, order_id, position) WHERE notice = 'WITHHELD'` SCHEMA:661（SCHEMA:660 註解「3d 告知被指名者的來源」）。CHECK 保證 WITHHELD 必有 recheck_on（SCHEMA:~650-653）。
- key 變動 [量測+推論]：記下新的確認日期時（ReplaceWithheldAuthorityTargetNotice authority_orders.sql:135-140，舊值寫歷史）recheck_on 會變：若新日期仍 <= 今日，項目留在佇列但 start 變；通常新日期在未來，項目離開再進入。
- 風險 [推論]：WITHHELD 只在完成來函時寫入、撤銷只在完成前（SCHEMA:619 `authority_orders_terminal_check`；model RecordNotice 對 revoked 回 AuthorityBlockRevoked authority_order.go:344-345），所以 WITHHELD 對象不會在已撤銷來函上；來源查詢不一定要 JOIN 來函，但不 JOIN 就沒有鎖住這個不變式的測試。
- 連結：來函詳情 `/admin/authority-orders/{id}`（[id]/page.tsx:457-475 有「告知被指名者」「記下告知」）；對象以 (order_id, position) 識別。

### 7. 裁定申覆（措施請求 kind=APPEAL，PENDING）
- 判定式 [量測]：`Measure.Pending()` measure.go:582（回傳 `EffectiveRequest` 後 state==PENDING 的請求，同時最多一筆；`EffectiveRequest` :531）。申覆不受措施生效中影響；SQL `AdminMeasuresPendingRequests` measures.sql:384-419：`q.state='PENDING' AND (q.kind='APPEAL' OR m.id=ANY(active_ids))`。
- 開始 `measure_requests.submitted_at`（proto `MeasureRequest.submit_time` measure.proto:77）。期限：Go 沒有 [量測：measure.go 的 AppealWindow=4416h(:94) 是「提出」申覆的窗口，`AppealDeadline()` :859，不是裁定 SLA]；D8 的 7 天只在文件。
- proto：`MeasureRequest.decide_due_time` 不存在 [量測]。
- 索引 [量測]：`measure_requests_pending ON seller.measure_requests (submitted_at, id) WHERE state='PENDING'` SCHEMA:936；`measure_requests_one_pending ... (measure_id) WHERE state='PENDING'` :935。keyset 現況 measures.sql:384 以 (q.submitted_at, m.id) 為鍵 -> 待辦用 request.id 或 measure.id 當 id 要先決定。
- key 變動：submitted_at 不變（decision_check 約束 SCHEMA 內 decided_at>=submitted_at）。穩定。
- 連結：措施詳情 `/admin/measures/{id}`。

### 8. 裁定解除請求（kind=REMEDIATION，PENDING 且措施仍生效中）
- 判定式 [量測]：`Measure.PendingRemediation()` measure.go:592（`Pending()` 且 Kind==REMEDIATION）；「措施已不在生效中」的解除請求推導為 CLOSED（proto 註解 measure.proto:~70；SQL measures.sql:384-386 用 `active_ids`）。
- 開始/期限/proto/索引同 7。解除請求 CHECK：`measure_requests_remediation_basis_check` 只允許 basis='RISK'（SCHEMA ~919）。
- 與 7 不同 [推論]：成員資格依賴「措施生效中」= `seller.effective_measures(at)`（objects.sql:44，SQL 函式，measure SQL 現況用 Go 先算 `active_ids` 再 `= ANY`，measures.sql:386、list.go:159/165）。UNION ALL 單一查詢要改成 `m.id IN (SELECT id FROM seller.effective_measures($now))`（EffectiveMeasuresAll 已這樣用 suspensions.sql:19-28）。
- key 變動：措施到期時項目離開（時間觸發）；start 不變。
- 連結：`/admin/measures/{id}`。

### 9. 複審停權（RISK 措施 review_by，到期前 24h 進入，D′11）
- 判定式 [量測]：現況只有 SQL `AdminMeasuresReviewDue` measures.sql:421-459：`basis='RISK' AND id=ANY(active_ids) AND review_by <= now`，ORDER BY (review_by,id)。Go 沒有 `ReviewDue` 判定方法 [量測：rg `ReviewDue` 只命中 list input/proto 與 list.go:159-166]。D′11 的「提前 24h」尚未實作（3cd-brief.md:292-306 建議）。
- 開始/期限：現況兩者都是 review_by；D′11 建議 start=review_by-24h、due=review_by。`ReviewWindow`=30d（measure.go:270，上限）；預設 7 天在 admin（merchants/[id]/measure.ts:279-286）。
- proto [量測]：`Measure.review_by` measure.proto:112（OUTPUT_ONLY；非 `*_due_time`，只有 RISK 才有值，不是「義務結束就不輸出」，措施撤銷/結束後仍輸出）；emitter governance_measures.go:103-104。
- 索引 [量測]：`measures_review ON seller.measures (review_by) WHERE status='ENFORCED' AND basis='RISK'` SCHEMA:829（註解「複審到期的收件匣」）；生效中的另靠 `measures_active_open`/`measures_active_timed`（SCHEMA:819-820）。
- key 變動 [量測]：會。`SetMeasureReview`（measures.sql:183-190，延後複審，進行中 RISK 可重排）改 review_by：新值仍在 now+24h 內 -> 留在佇列、key 變；超過則離開。這是 brief「8 種只有 KYC 會換 key」的反例 [推論，建議更正該句]。
- 連結：`/admin/measures/{id}`（MeasureSections.tsx:164-165 顯示「複審期限」，ScheduleReviewDialog 延後）。

### 10. 審核食品業者登錄
- 判定式 [量測]：`FoodRegistrationReview.CanReject()` MODEL/foodreg.go:187（`State==Pending && StoreStatus==Active`）；`CanApprove()` :195（再加 KYC 已核准）。SQL `AdminFoodRegistrationInbox` food_registrations.sql:102-114：`state='PENDING' AND s.status='ACTIVE'` 並以 `awaiting_kyc` 分 DECIDABLE / AWAITING_KYC（MODEL/foodreg.go:33-36）。要決定 3d 的成員是 DECIDABLE only 或兩者皆是（D8：7 天「自可裁定起算」，3-brief D8；inventory.md:156-157 寫起算 = max(submitted, KYC 核准)）。
- 開始：`submitted_at`（現況）；D8 要 max(submitted_at, 店家 KYC APPROVED 的 decided_at)，KYC 在同一 seller schema（kyc_applications.decided_at）-> SQL 可 JOIN，不跨模組。期限只在文件；Go 無常數 [量測]。
- proto [量測]：`FoodRegistration.submit_time` seller.proto:819（OUTPUT_ONLY）；沒有 due 欄位。
- 索引 [量測]：`food_registrations_inbox ON seller.food_registrations (submitted_at, id) WHERE state='PENDING'` SCHEMA:212；`food_registrations_one_pending` :209。
- key 變動 [推論]：submitted_at 不變，但「可裁定起算」若採 max(submitted, KYC 核准)：KYC 被核准後若 PayUni 拒絕回到 DRAFT 重送（UpdateKYCDraft kyc.sql:43-50，APPROVED->DRAFT 清 decided_at；kyc.go:242-250 editable）再核准，decided_at 會變 -> 佇列中 key 會變。
- 連結：`/admin/food-registrations/{id}`。

### 11. 審核商家認證（KYC，SUBMITTED）
- 沒有獨立「審核中」狀態 [量測]：`KYCStatusSubmitted` MODEL/kyc.go:26 = 審核中；列表篩選 `OldestFirst` kyc.go:488-490（status==SUBMITTED）。成員 = `status='SUBMITTED'`（AdminKYCApplicationsOldestFirst kyc.sql:215-232：`status<>'DRAFT' AND submitted_at IS NOT NULL AND status=...`）。
- 開始 `submitted_at`；期限：Go 無（D8：7 天，rg `docs/runbooks` 無 KYC 期限）。
- proto [量測]：`KYCApplication.submit_time` seller.proto:521、`KYCApplicationSummary.submit_time` :554；無 due 欄位。
- 索引 [量測]：`kyc_applications_status_submitted ON seller.kyc_applications (status, submitted_at, application_id) WHERE status<>'DRAFT' AND submitted_at IS NOT NULL` SCHEMA:1128-1129（status 是首欄 -> `status='SUBMITTED'` 等值後 (submitted_at, application_id) 有序）。另 `kyc_applications_submitted` :1130。
- key 變動 [量測]：SUBMITTED 期間不能編輯（kyc.go:234-243 一律 FailedPrecondition，只能先取消），`TransitionKYCApplication` 只有 DRAFT->SUBMITTED 才寫新 submitted_at（kyc.sql:83-93；kyc.go:543-546）。所以在佇列期間穩定；重送是「離開(REJECTED/DRAFT)後再進入」，不是佇列中換 key [推論]。
- 連結：商家詳情 `/admin/merchants/{store_id}`（沒有 query 參數；KYC 區在同頁，merchants/[id]/page.tsx:931 顯示「提交時間」，審核卡 KycApplicationCards.tsx / RejectKycDialog.tsx）。

## 二、既有 `*_due_time` 一覽 [量測]
`rg -n "due_time" api --glob '*.proto'` -> 7 行命中，3 個檔：
- governance.proto:390 `Report.triage_due_time`（OUTPUT_ONLY；只待分案；僅食安非 nil）；:777 `CreateCaseInquiryRequest.due_time`（輸入）；:361 reserved "acknowledge_due_time"。
- authority_order.proto:109 `AuthorityOrder.due_time`（OUTPUT_ONLY，OPEN 才設）；:209 輸入。
- case_inquiry.proto:61 `CaseInquiry.due_time`（OUTPUT_ONLY，OPEN 才設）；:22 註解。
`rg -n "due_time|_due" api -l` 只有上述三個檔。3d 需新增：Report 擴 72h、`Case.decide_due_time`、`Case.fda_referral_due_time`、MeasureRequest/FoodRegistration/KYCApplication 的 decide_due_time 等（brief §2 第12項、inventory.md:159-175）。

## 三、已有 / 沒有 ReviewPolicy
`rg -n "ReviewPolicy|review_policy" backend --glob '!*.pb.go'` -> 命中全在 backend/docs（plans）；程式 0 命中 [量測]。期限常數現況：FoodSafetyTriageSLA 4h（report_case.go:131，governance.go:266 轉出）、InquiryDefaultDue 72h/InquiryOverdueGrace 24h（case_inquiry.go:79/84）、DefaultAuthorityDue 48/72h（authority_order.go:36）、AuthorityNoticeFollowUpGrace 72h（:73）、ReviewWindow 30d（measure.go:270）。AppealWindow=4416h(measure.go:94) 是提出窗口、不是裁定 SLA。

## 六、模組
`ls backend/internal/modules` -> accountdeletion agent buyer content delegation discovery identity media seller（9 個）。KYC 與食品登錄屬 seller [量測]：M/kyc.go、M/foodreg.go、M/internal/kyc/、M/internal/foodreg/（schema 同在 seller.*）。其他模組有無 queue-like 種類：`rg -il "inbox|queue|待辦|待審|awaiting" --glob '!*_test.go' --glob '!seller/**'` 在 modules 命中的是 river job/佇列字樣（media/accountdeletion 的週期核對）與 CARRYOVER/AGENTS 文字；沒有 admin 待辦清單 [量測，限於 modules/ 底下]。api/hawkr/admin 只有 buyer/content/identity/seller 四個 v1alpha1 [量測 ls]。A7 的 moderation 模組目前不存在（brief D′1）。

## 七、platform / rpc 分頁
- `rg -n -i "keyset|pagetoken|page_token" backend/internal/platform` -> 0 行 [量測]。platform 沒有 keyset 比較函式；platform/pagination/pagination.go 只有 `PageSize(size, def, max int32) (int32, error)`（:7）。
- 分頁 token 在 httpapi/rpc/paging.go [量測]：`EncodePageToken(scope string, key []byte) string` :20；`DecodePageToken(scope, token string) ([]byte, error)` :33；`PageCursor[T any](scope, token string) (*T, error)` :52；`TokenFor(scope string, cursor any) string` :65；`ValidCursor[T interface{Valid() bool}](scope, token string) (*T, error)` :71；`PageScope(procedure string, ownerID int64) string` :83；`FilterScope(procedure string, filters ...string) string` :89（以 \x00 串接，filters 位置固定、沒設定的條件也佔位）。token = version byte + sha256(scope)[:8] + key。
- `(*DB).InReadSnapshot(ctx, fn func(ctx) error) (err error)` platform/db/db.go:149（REPEATABLE READ、唯讀；已在 tx 內回 `ErrSnapshotInTx` :141）。`db.Conn(ctx)` :103、`InTx` :112。注意 backend/AGENTS.md 規則 3：會被別的模組呼叫的公開方法不得用 InReadSnapshot；3d 計畫是 httpapi 合併層自己開 snapshot、來源方法跑在 `db.Conn(ctx)`（3cb-design.md:~1146，C26）。目前 httpapi 層沒有任何地方呼叫 InReadSnapshot（`rg "InReadSnapshot\(" --glob '!*_test.go'` 只命中 platform/db 與 modules 內部 20 處）[量測]。
- keyset 範例：httpapi `ListMeasures` governance_measures.go:349-405（FilterScope + ValidCursor[model.MeasureCursor] + TokenFor；`MeasureCursor` MODEL/measure.go:~1090-1099 `{SortAt, ID}`、`Valid()`）；模組側 M/internal/measure/list.go:106-200；SQL row-value `(q.submitted_at, m.id) > (...)`（measures.sql:~404）。`TestAdminInboxKeysetHasNoGapsOrDuplicates` 存在：M/governance_integration_test.go:644（9 筆通報、雙方向、頁大小 4）；另有 `TestAdminStoresKeysetPagesHaveNoGapsOrDuplicates` M/admin_integration_test.go:168。
- twtime：`Day` twtime.go:34、`Midnight` :40、`MinuteCeil` :25（台北日）。

## 八、httpapi
- Admin Connect service 掛載：R/backend/internal/httpapi/server.go `connectServices` :239-296；seller admin 在 :275（SellerAdminService）、:278（SellerGovernanceAdminService）、:286（SellerProspectAdminService）；`checkAdminDeclarations(out)` :294 呼叫（定義 :326-332，對照 adminauth 宣告表，宣告=掛載，不符無法啟動）。`mount` :336 要求 service 名稱符合 `rpc.AudienceOf`（rpc/rpc.go:46：`hawkr.admin.*` 且以 `AdminService` 結尾）-> 新 service 名要 `ReviewQueueAdminService`、package `hawkr.admin.reviewqueue.v1alpha1`。
- 跨模組讀取組裝範例 [量測]：`httpseller.Dashboard{Orders, Views, Followers, Now, Budget, Log}` server.go:253 / httpapi/seller/dashboard.go:46,63；`StatsSource` httpapi/seller/stats.go:24（deps.Stats 共用）；`httpdiscovery.NewPublic(deps.Discovery, deps.Seller, deps.Buyer, stats, ...)` server.go:260。這些是補顯示欄位型（依 ID 取資料），沒有「多來源 keyset 合併」的先例 [推論，rg 未找到名為 merge/compose 的 keyset 程式]。
- 權限宣告：platform/adminperm/adminperm.go（`Permission` 型別；`<模組>.<資源>.<動詞>`，檔頭 :1-5）；常數表 :21-…（例：`LegacyGatewayForward = "legacy.gateway.forward"` :43、`SellerProspectsList` :107）。方法->權限對照在 httpapi/adminauth/adminauth.go `declarations` map :41-…（`requires(adminperm.X)` / `exempt(reason)`；例 :87 `SellerGovernanceAdminServiceListReportsProcedure: requires(adminperm.SellerReportsList)`，:128 prospects）。RoleOwner 擁有目錄內全部權限（adminperm.go:~18）。
- admin 方法數 92 的斷言 [量測，兩處]：httpapi/identity/admin_roles_integration_test.go:80-81（`len(procedures) != 92`，"admin 方法 %d 支，預期 92"）；httpapi/adminauth/adminauth_test.go:65-66（`got != want || got != 92`）。`rg -n "\b92\b" httpapi platform/adminperm --glob '*.go'` 排除 gen 後只這兩處。加 `ListReviewQueue` 後兩處都要改 93。adminperm_test.go 未查是否有權限數斷言 [未確認]。
- `maxRESTOperations` = 481（backend/gateway/internal/router/openapi_coverage_test.go:28；只准變少，:171-174）。3d 預期不變（3cd-brief.md:344 與 §2 第9項）。

## 九、admin
- `ADM/lib/deadline.ts`：`DeadlineTone` :7、`DeadlineDisplay` :9、`durationText(ms)` :23（無條件捨去：N 天 H 小時／H 小時 M 分／M 分／不到 1 分）、`deadlineDisplay({due,start,now})` :44（無 due -> {text:"-"}；remaining<0 -> `已逾期 X` danger；剩餘 < (due-start)/4 且 start 有值 -> warning；時鐘由呼叫端傳入）。`DeadlineText` ADM/components/UI/DeadlineText.tsx:14（只渲染 display.text + `adm-sla--{tone}`，可帶 title 絕對時間）。`useNow(intervalMs)` ADM/lib/use-now.ts:4（setInterval 回傳 Date）。`SlaCountdown`（ADM/components/UI/SlaCountdown.tsx）是**工作天**專用（發票申訴 invoices/complaints 用，值全由後端給），與日曆時間的 DeadlineText 不同，3d 不用它。
- LIST_REFRESH_MS = 30*1000：ADM/app/admin/violation-reports/report-list.ts:215（authority-orders/page.tsx:13 匯入）。`refetchInterval: LIST_REFRESH_MS` 用在 violation-reports/page.tsx:68,75、authority-orders/page.tsx:93。
- 側欄：ADM/components/Layout/Sidebar.tsx；「賣家 / 商品」section :47，首項「賣家管理 / 審核」/admin/merchants，接著 商家通報、主管機關來函、措施與申覆、食品業者登錄審核、代行紀錄、站外業者、買家管理、商品管理、商品認證審核、分類審核。另「內容治理」section 有既有的 `/admin/reports` 「內容審核佇列」（Sidebar.tsx:33-37，社群內容；A7-4b 才會動）。
- `/admin/queue` 不存在 [量測：`ls ADM/app/admin/queue` -> No such file]；`rg "ReviewQueue|reviewqueue" api ADM backend/internal` 0 命中（只在 docs）。
- 可仿照的 keyset 清單頁：ADM/app/admin/authority-orders/page.tsx（`firstPage/currentToken/nextPage/previousPage/currentPage` 來自 ADM/lib/keyset-pages.ts:8-29；分頁列 :254-278「上一頁｜第 {page} 頁｜下一頁」，`(page>1||next)` 才顯示；tab 切換重置 `setPageTokens(firstPage())` :96-99；`useQuery(Service.method.x, req, {refetchInterval})`）；measures/page.tsx 同形。

## 十、現有期限顯示處（3cd §4.5 rows 120-126）
- 措施清單「複審到期」分頁：ADM/app/admin/measures/measure-inbox.ts:46-48 分頁（filter `reviewDue:true`）；列內 `reviewDueText` measure-inbox.ts:137-143 -> 「複審期限 {YYYY-MM-DD}」（formatDate，無剩餘/逾期色階）；page.tsx:141-142,152。欄位 page.tsx:~120-127：店家｜種類｜依據｜請求｜已等候｜狀態｜建立時間；「已等候」`rowWaitingText` measure-inbox.ts:126（待處理請求的天數）。
- 措施詳情待處理的請求：measures/[id]/MeasureSections.tsx:64 區塊，:77 只顯示「提出時間」；複審期限 :164-165 `Field 複審期限 timeText(reviewBy)`；沒有申覆/解除請求的裁定期限。
- 食品業者登錄清單：food-registrations/page.tsx 欄 :136-141 店家｜送出的字號｜實名認證｜送出時間｜狀態｜操作，`submitTimeText` (ADM/lib/food-registration.ts:199) 只顯示送出時間；詳情 [id]/page.tsx:227 「送出時間」。無期限。
- 商家詳情 KYC 審核區：merchants/[id]/page.tsx:931-937 只顯示「提交時間：…」；無審核期限。
- 已有 DeadlineText 的地方：violation-reports 清單 page.tsx:159（`triageDeadline` report-list.ts:200-209：due=triageDueTime，start=createTime）與詳情 [id]/page.tsx:353-354「分案期限：{日期}（{DeadlineText}）」；authority-orders 清單 page.tsx:158、詳情 [id]/page.tsx:355-358；案件詳情 CaseInquiries.tsx:97-100（詢問期限）。

## 十一、Runbook K18
`rg -n 'K18' backend/docs admin docs` -> 命中；與期限相關者：
- backend/docs/runbooks/food-safety-incident-runbook.md:160：「**申訴 SLA**：申訴提交後 admin 必須在 14 天內決議。數字待 3d 與申訴 SLA 一併校準（K18），這一刀不改。」
- 同檔其他數字（3d 需對齊）：:5「分案期限 4 小時（一般 72 小時）」、:12-13 待分案分頁 30 秒更新、剩不到 1 小時轉黃、:64,:72 「1/8」「四分之一」、:296-298 表（分案 4h；首次聯繫 1h；查證並決定 4h/8h 結案）、:395「不是食安類別…分案期限 72 小時…案件的決定期限與待辦頁在 3d」、:399 轉報後 48h 未回函致電。
- 3-brief D8 表（3-brief.md:~187-196）：申訴舊值 14 天（食安 runbook，標「過時」）與 7 個工作天（violation runbook :115,151）互相牴觸；補正 7 日曆天；食品登錄 7 天；KYC 無。K18 在 3-brief/3cd-brief 的歸屬：3cd-brief.md:344（3d 列「runbook K18」）。（`step10d-decision-brief.md:61` 與 `step10e-decision-brief.md:44` 的 K18 是別的東西：頭像、會員方案。）
- 未決：violation runbook (seller-violation-management-runbook.md) 的 7 工作天我沒有開檔核對 [未確認]。

## 十二、ADR / D′1 [量測]
ADR `backend/docs/adr/2026-09-24-go-modular-monolith.md`（355 行）：
- :124 `## D3`；:130-135 原「跨模組讀取的判斷順序」，第 3 點 :133-135（「必須拿下層模組的資料來篩選或分頁：在自己的 schema 維護讀取模型…」，原文保留作歷史）。
- :304 `## 修訂：跨模組讀取改用擁有者發布的 pub_*、D4 收窄（2026-10-18，Will）`，:306 說明本段「取代 D3 的判斷順序第 3 點」。新順序 :308-318：1 擁有者對嗎；2 依 ID 取資料；**:314「3. 各來源都擁有全部篩選與排序鍵：在 httpapi 依同一個全序做 keyset composition。」**；4 join `pub_*`；5 讀取模型只在量測觸發才建。:355 文末另有 2026-10-19 修訂（A′(i) 審查）。
- backend/AGENTS.md:28（規則 2）同步含「各來源都擁有全部篩選與排序鍵就在 httpapi 做 keyset composition」[量測]。A7 計畫 docs/plans/2026-10-18-a7-social.md:540-542 已寫「ADR 2026-09-24 的修訂段已寫（D3 判斷順序第 3 步 keyset composition 取代 10e D′1 的例外文字）」。
- ADR 修訂段**沒有**寫 brief 的細節：不變式（單一比較函式、rank 整數、一個 now、snapshot）、終局與觸發條件（來源算不出的鍵、指派/認領/暫停、p95>300ms）。這些目前只在 3cd-brief D′1。

3cd-brief `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3cd-brief.md`（614 行）需要改的位置（因為 ADR 已有修訂段，brief 仍寫成「待 Will 決定的偏離」）：
- :19「這偏離 ADR 2026-09-24 D3 第 3 點…附了修訂文字，要你定」-> 改為「ADR D3 已由 2026-10-18 修訂段（:304-318 第 3 點）涵蓋，不再是偏離」。
- :61「**ADR**：修訂 ADR D3 第 3 點與 backend 單體規則 2（文字見下）」-> 改為指向 ADR :314 與 backend/AGENTS.md:28，不再「修訂」。
- :75、:92（約束帳）仍引用舊第 3 點字面（「必須拿下層模組的資料來篩選或分頁時…讀取模型」）與 `backend/AGENTS.md:28` 舊句：改成現行 :133-135 已被取代，約束帳那列判定改為「已修訂，D′1 符合」。
- :99-100「**ADR D3 第 3 點的修訂文字**（建議，同時改 backend 單體規則 2 的措辭與 A7 計畫:71 不變量表那一列）：> 例外：…」整段 -> 改為「見 ADR 2026-09-24 文末『修訂：跨模組讀取改用擁有者發布的 pub_*』第 3 點（:314）」，並把例外句中的兩個條件（合併層不篩選/排序來源沒有的屬性；出現來源算不出的鍵或只屬於合併結果的狀態時改讀取模型）決定要不要補進 ADR :314 或 :318（目前 ADR 第 5 點只談「量測觸發」，沒有寫這兩個觸發）。
- :135「ADR D3 的修訂用這裡的文字，不另寫一份。」-> 與事實相反（ADR 已另寫了不同的文字，例外 vs 判斷順序第 3 步）；要改。
- :344（切分表 3d 列「ADR D3 修訂（D′1）」）與 :580（B1「D′1 偏離 ADR D3 第 3 點，約束帳的來源寫錯 -> 附 ADR 修訂文字，交你決定」）同步改。
- 未查 A7 計畫:71 不變量表那一列是否已更新 [未確認]。

## 十三、D′6（實名認證 -> 商家認證）[量測]
指令：`rg -n "實名認證" admin/src website seller-app/lib buyer-app/lib backend/internal/platform/uitext/catalog` -> 60 行。分布（`rg -c`）：admin/src 11、website 41（含 tests/e2e、*.test.ts、CSS 註解）、seller-app/lib 6、buyer-app/lib 0、uitext/catalog 2（zh-TW.json:120 message「平台會在實名認證通過後進行審核。」、:121 description）。「商家認證」現有 89 行。結論：**D′6 尚未做**（brief 估 admin 12/目錄 1 句+2 說明/賣家 website 6、seller-app 3/買家 website 6，約 30 行；實測計數方式不同，website 41 含大量測試與行銷文案）。admin UI 字串：food-registrations/page.tsx:102,138、[id]/page.tsx:253,258,266、lib/food-registration.ts:21,60,71,74、product-certifications.ts:14 註解、merchants/[id]/page.tsx:1040 註解。

## 十四、3cd §2 第17項（措施清單店家欄）[量測]
ADM/app/admin/measures/page.tsx:151 仍是 `{m.storeName || m.id}`（m.id 是措施 ID；brief 寫 :151 吻合）。`Measure.store_name` measure.proto:157（OUTPUT_ONLY）、emitter governance_measures.go:91 `StoreName: m.StoreName`；`Measure.store_id` measure.proto:98 存在，可作退路。**仍是 bug，未修**。

## 十五、其他給設計的觀察
1. 全序鍵「open_time」的實作成本不同：待決定（max 子查詢）、食品登錄（若採 max(submitted, KYC 核准) 需 JOIN kyc_applications）無法直接走單一部分索引排序；其他 8 種可走既有索引（見各條）。
2. 現有 keyset 範例的 id 型別：uuid（reports/cases/inquiries/orders/measures/food_registrations 用 uuid；stores 用 bigint store_id；kyc application_id uuid）。來函對象的識別是 (order_id uuid, position smallint)。UNION 的 id 欄需決定統一型別（text 或 uuid + position）。
3. 時間精度：Postgres timestamptz µs，Go 時間 ns；比較函式要截到 µs（brief D′1 已列）。
4. 現況 measures 的「生效中」集合在 Go 算（stint.ActiveMeasureIDs，list.go:56-65），單一 UNION 查詢要改用 `seller.effective_measures($now)`（objects.sql:44）以維持 index-ordered 與同一個 snapshot。
5. 解除請求/複審停權的 member 依賴 effective_measures，所以隨時間（措施 ends_at）離開佇列，不需 job。

## 未確認
- adminperm_test.go 與其他測試是否另有「權限總數」斷言（只確認 adminauth_test.go:65 與 admin_roles_integration_test.go:80 兩處 92）。
- violation runbook 的 7 工作天（:115,151）、A7 計畫 :71 不變量表目前文字。
- case_referrals 對 case_id 有無索引（影響 NOT EXISTS 的成本）；EXPLAIN 均未跑（沒有跑任何資料庫查詢）。
- 食品登錄 3d 成員是否只含 DECIDABLE 隊列（brief 沒明講；D8「自可裁定起算」暗示是）。
- website 的 41 行「實名認證」中哪些屬賣家面 UI、哪些是行銷/測試，未逐行分類。
- `/admin/merchants/{id}` 是否有可直達 KYC 區的 hash/anchor（只確認無 query 參數；未找 id=/hash）。
- 所有行號均為 HEAD 4cc2ec079 的工作樹；未執行任何測試或建置。
