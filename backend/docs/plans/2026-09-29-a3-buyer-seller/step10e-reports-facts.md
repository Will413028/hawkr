# Reports repo facts (read-only inventory, 2026-10-18)

[M] = measured (read). [I] = inferred.
- W = （10e worktree） (branch local/10e, HEAD 65be53b77)
- M = （主 checkout） (HEAD eb8594011)
- S = W/backend/internal/modules/seller
- P = W/backend/docs/plans/2026-09-29-a3-buyer-seller
- D32 = W/backend/docs/詳細設計書/32-商家治理與食安通報（Seller Governance & Food Safety）詳細設計書.md

## Summary
1. Statuses PENDING → INVESTIGATING → VERIFIED | DISMISSED. VERIFIED only from INVESTIGATING. DISMISSED from PENDING (acknowledged or not) or INVESTIGATING. Acknowledged is a flag + severity while status stays PENDING. StartInvestigation requires acknowledgement. FDA referral has no status/ack precondition. Cooperation request allowed in every status except DISMISSED.
2. Events ReportAcknowledged, ReportResolved, SellerCooperationRequested — no subscribers. No reporter template anywhere (legacy included). A8 (notifications) comes after A7 and A6.
3. Sellers cannot read or answer cooperation requests: no API, no client.
4. Admin forced unpublish → legacy gateway → Java product-service; only changes status (4003 「未上架」); no reason, no measure, no appeal; seller can re-list. A7-D7 「內容下架」 never enumerates content_kind; scope is social content, not products.
5. CreateReport: any logged-in account (buyer identity), so sellers can report; no self-report check found [I]. SubmitFoodSafetyReport anonymous, food safety only. No rights-holder path, though seller terms 15.5 and website 使用者政策 §十二 promise one.
6. No client calls ListMyReports; the duplicate-report error tells users to check 「我的通報」.
7. No recall / affected-buyer notification in code or plans; only seller terms 第十一條.
8. D11 case aggregation trigger: 「上線後第一次出現同一家店、同一事件 3 筆以上的通報」.
9. 7 categories; no client shows them to reporters; apps' only reporting screens are legacy content-report sheets.
10. Admin report pages still call legacy REST; only SourceReportsField uses Connect ListReports.

## 1. Lifecycle
- Statuses: `S/internal/model/governance.go:56-73`, `S/db/schema.sql:240`, `W/api/hawkr/seller/v1alpha1/report.proto:34-45`. No ACKNOWLEDGED status; acknowledging keeps PENDING (`report.go:179`; `S/db/queries/reports.sql:57-63`).
- Transitions (`S/internal/governance/report.go`):
  - Submit `:86-150`: outside tx (`:94-96`); store must exist (`:99-101`) and have been public once (`S/governance.go:384-398`). PENDING by default. No event.
  - Acknowledge `:164-196`: PENDING and acknowledged_at NULL (`:179`; SQL guard `reports.sql:62`). Writes acknowledged_at, admin, severity. Event ReportAcknowledged (ReporterBuyerID, Category, Severity) `:190-193`.
  - StartInvestigation `:198-218`: PENDING and acknowledged (`:207`; `reports.sql:71`). status=INVESTIGATING, investigating_at. No event.
  - Resolve `:220-262`: VERIFIED only from INVESTIGATING; DISMISSED from PENDING or INVESTIGATING (`:241`); `WHERE status = from` (`reports.sql:83`). Writes resolved_at, admin, resolution_note (「給通報人的結果說明」 `model:286`). Event ReportResolved (ReporterBuyerID, Outcome) `:256-259`.
  - RecordFDAReferral `:264-300`: FOOD_SAFETY only (`:283`), once (`:286-289`); no status/ack check. No event.
  - RequestCooperation `:302-347`: any status except DISMISSED (`:318`), also unacknowledged (test `S/governance_integration_test.go:394-399`); once per (report, template) (`schema.sql:315`). Event SellerCooperationRequested with AccountID `:336-344`.
- Terminal: second Resolve fails (test `:301-303`); no reopen.
- CHECKs `schema.sql:237-276`: value lists; exactly one reporter kind (`:246-250`); ack fields all-or-none (`:256-258`); status vs timestamps (`:259-264`; VERIFIED needs acknowledged_at, investigating_at, resolved_at); FDA columns only FOOD_SAFETY, all-or-none (`:269-273`).
- Timestamps: created_at, updated_at, acknowledged_at, investigating_at, resolved_at, fda_reported_at, contact_erased_at, cooperation requested_at. Ack deadline derived, food safety only (4h, `model:155-156`, `:241-254`). D8 plan: 72h all categories; no investigation deadline (`P/step10e-3-brief.md:230-232`).
- Justification of INVESTIGATING/ack: D32 describes but never argues (`:59`, `:95-96`, `:35`/`:75`). D32 `:325` legacy timeline 「ack 5 min → 1h 內聯繫通報者 → 4h 內 investigating」. Company era (D32 at 1674e2f04): D9 「ack 即計時結束」 (line 114); ack only on food_safety_reports (「4h SLA 量測」 lines 168-169); legacy transition table `W/backend/services/seller-service/internal/domain/seller_violation_report.go:110-124`; ack-before-investigating only for food safety (`usecase/food_safety_report.go:147-149`). Runbooks (2087adf03): investigating = 「我已經看到這筆、正在調查」 to stop two admins handling the same report (`seller-violation-management-runbook.md:50`); ack = 「admin 已收到、正在看」 (`food-safety-incident-runbook.md:100`); pending→investigating requires 「ack 後且已聯繫通報者」 (`:117`); runbook `resolved` status never existed (`:110-121`). Monolith extended ack to all categories because severity is set at triage (`S/CARRYOVER.md:61`). External basis cited: DSA Art.16 (ADR 2026-10-02 `:21`), DSA 16(4)/16(5) (`P/step10e-3-research.md:159`).

## 2. Events and subscribers
- Emitted `S/events.go:33-40`; StartInvestigation, RecordFDAReferral, Submit emit nothing.
- Subscribers: none. `W/backend/cmd/hawkr/seller_events.go:29-41` only store/PayUni/KYC; `W/backend/internal/legacy/sellerrelay/relay.go:9-11` deliberately not forwarding governance events (「A8 的 notification 訂閱」); only test recorders; publishing without subscriber is a no-op (`W/backend/internal/platform/events/bus.go:50-63`). Docs agree: `S/AGENTS.md:148`, `CARRYOVER.md:71`, ADR 2026-10-02 `:72`, `:79` (「A8 之前…通報人不會收到結果」).
- Design: D32 `:344-362` ReportAcknowledged/ReportResolved → logged-in reporter; SellerCooperationRequested → seller; 「匿名通報人的聯絡方式不放進事件」 (`:362`). ADR `W/backend/docs/adr/2026-10-05-notification-module.md:55`: anonymous reporters get outcome by email if left; phone-only get nothing. Gap [I]: event carries no contact; contact erased after 90 days; D32 §7 omits anonymous reporters, disagreeing with that ADR.
- Anonymous submit response is empty (`report.proto:112-113`): no ID or token.
- Legacy: notification-service has no governance/report templates (only `post.hidden_by_platform` to seller, `db/migrations/20260709120000_content_hidden_seller_template.sql:21-22`); legacy seller-service publishes 5 `seller.governance.*` keys to a never-created exchange (`W/backend/docs/plans/2026-09-24-go-modular-monolith/event-anomalies.md:7`; `usecase/violation.go:547-549`); original D32 had 5 templates, none for reporters (1674e2f04 lines 453-465); company runbook expected admins to manually tell the reporter on dismissal 「若願留聯絡方式」 (`seller-violation-management-runbook.md:59`).
- A8 plan: `W/backend/docs/plans/2026-09-24-go-modular-monolith.md:221` bare unchecked item; order A7 → A6 → A8 (`:204-221`); moving A8 earlier not recommended (`M/backend/docs/plans/2026-10-18-a7-social/decision-brief.md:130-132`).

## 3. Cooperation requests
- Admin-only create/read: `W/api/hawkr/admin/seller/v1alpha1/governance.proto:52-54`, `:152-159`, `:230-231`. Seller-facing service has no report/cooperation method (`W/api/hawkr/seller/v1alpha1/seller.proto:28-152`).
- Clients: no seller-side code. Only legacy admin page `W/admin/src/app/admin/violation-reports/[source]/[id]/page.tsx:186-215` calls legacy `notify_seller`. Template not in shared catalog.
- Plans: ADR 2026-10-02 `:105` showing report material to sellers deferred (「遮蔽或轉述」); "waiting for the seller's reply" deferred (D11, `P/step10e-3-brief.md:310`); no plan for a seller-facing view.

## 4. Product-level enforcement
- Dialog `W/admin/src/app/admin/merchants/[id]/ProductForceUnpublishDialog.tsx` (comment `:11-17`: no reason field on purpose; text `:82`: 「賣家仍可自行重新上架」). Page `page.tsx:387-393` sends `PUT /api/v1/admin/products/{id}/status` `{status:"4003"}` → gatewayproxy → legacy gateway `W/backend/gateway/internal/handler/admin_product.go:16-18`, `:166`, `:179` → `product_client.go:361-372` → Java `ProductGrpcService.java:588-617` → `ProductService.java:1422-1486`: sets status, logs, clears caches; no reason/actor; seller not notified.
- Seller sees 4003 「未上架」 (covers never listed, unlisted by seller, rejected; `ProductStatus.java:17`); seller can re-list (`ProductService.java:1575`), only suspension blocks (`:1608-1609`).
- Measure kinds WARNING, SUSPENSION, TERMINATION only (`schema.sql:413`); no catalog module.
- Plans: `P/step10e-measure-brief.md:185`: 「現在只記錄牴觸…B4 catalog 時成為措施種類「商品限制」」; B4 checklist (`2026-09-24-go-modular-monolith.md:256-260`) does not include it [I: could get dropped].
- A7-D7 `measure_contents(measure_id, content_kind, content_id)` (`M/.../2026-10-18-a7-social.md:469`); content_kind never enumerated; scope = store social content (posts, comments, forum) (`P/step10d-a7-social-brief.md:271`); products excluded [I]; D7 undecided (`a7-social.md:455`).

## 5. Who can submit
- CreateReport: any logged-in account via buyer identity (`W/backend/internal/httpapi/seller/reports.go:96-106`; every account is a buyer `:31`); all 7 categories; ≤5 evidence images; 5/10min and 10/hour per account (`:42-46`, `:68-82`).
- SubmitFoodSafetyReport: recorded anonymous even when logged in (`reports.go:174-190`); FOOD_SAFETY forced (`:184`); phone or email required; no evidence (`report.go:60-79`); per-IP limit (`W/backend/internal/httpapi/ratelimit.go:49-62`).
- Not possible: anonymous non-food-safety report; admin-created report (`P/step10e-3-brief.md:59`); rights-holder/IP notice (no endpoint, no category; infringement only as a RISK ground `seller.proto:1083`; `S/internal/model/measure.go:118`).
- Seller terms (version 2026-10-16, not lawyer-reviewed `:3`): 第十五條 `:150-156`; 15.5 `:156` 「收到具體侵權通知時，甲方得先行保全證據、限制內容或要求補件，並應提供乙方合理申覆機會；依法須立即處理者不在此限。」 Also 5.5 `:61`, 16.5 `:166`, 17.1 `:173`.
- Buyer side: no separate buyer terms in `W/website/docs/legal`. Website 服務條款 §八 (`W/website/features/legal/pages/TermsPage.tsx:229-245`): reports via the App's 「檢舉」 button. 使用者政策 (`UserPolicyPage.tsx`, 2026-08-13): §十二 (`:316-323`) notice-and-takedown under the Copyright Act ISP chapter with required fields and counter-notice, but no channel; §十五 (`:380-397`) reporting 「商品、貼文、評論、訊息或帳號」 via App 「檢舉」. `P/step10d-a7-social-brief.md:283` says 「沒有買家條款」, conflicting with these pages [I].

## 6. 我的通報
- API `report.proto:19-22`; reporter view (`:47-64`) omits severity, admin, FDA, cooperation, ack time.
- No client. Apps' 「我的檢舉」 call legacy social content reports (buyer-app `settings_screen.dart:390`, `social_repository.dart:590-604`; seller-app `profile_other_options_card.dart:84-86`, `report_repository.dart:24`, `:71`). Duplicate error: 「可以在我的通報查看處理進度」 (`reports.go:145`).
- Plans: `P/step10d-decision-brief.md:309` 「`ListMyReports` 沒有入口，B6 也沒有要求，10d 不做。」 No step schedules it.

## 7. Food safety recall
- Nothing in backend/internal, backend/docs, api for 回收/召回/recall/通知買家/受影響.
- Seller terms 第十一條 (`seller-terms.md:116-123`): 11.4 seller must stop shipping and provide batch, distribution, stock data; 11.5 `:122` 「甲方得立即停止相關商品交易、通知買家、保全紀錄、配合主管機關或要求乙方執行回收」.
- `order_no`, `product_id` on reports are opaque, unchecked (`schema.sql:209-210`); nothing links a report to affected buyers [I].

## 8. D11 (verbatim, `P/step10e-3-brief.md:299-311`)
- 指派／認領: 不放；REAL 只有一位營運人員；第 2 位擁有裁定權限的管理員出現時 D7 實體化並加指派、認領、round robin.
- 升級 priority: 不加；REAL 沒有主管角色.
- 四眼: K15 已決定.
- QA 抽樣: 不做；第 2 位管理員或每週裁定超過約 50 件.
- 案件聚合: 只做 finding_reports；REAL 沒有資料可以訂歸案規則；上線後第一次出現同一家店、同一事件 3 筆以上的通報.
- 等賣家回覆（配合要求的期限與暫停）: 延後；REPORT_INVESTIGATE 沒有期限（D8）；和 D8 的調查期限同一個觸發條件（「上線後累積 20 筆結案通報，取 P90 訂值，同時加暫停」 `:232`）.
- 透明度匯出: 不做；不在歐盟、第 19 條豁免.

## 9. Categories
- ViolationCategory (`seller.proto:1004-1013`): FOOD_SAFETY, FAKE_CERTIFICATION, MISLEADING_INFO, COUNTERFEIT, UNAUTHORIZED_SALE, FRAUD, OTHER; Go model `S/internal/model/governance.go:8-35`; CHECKs `schema.sql:237-239`, `:340-342`; CreateReport takes a category (`report.proto:69`); anonymous always FOOD_SAFETY. zh-TW labels catalog `:881-905`.
- No client shows them to reporters; website has no report screen. App screens use legacy content-report reasons SPAM, SEXUAL, FRAUD, HARASSMENT, FOOD_SAFETY, POLITICAL, FAKE_NEWS, OTHER, targets REVIEW, POST, POST_COMMENT, SELLER_FORUM, CHAT_MESSAGE, USER — no PRODUCT or STORE (`W/seller-app/lib/core/codemeta/code_meta.dart:4430-4488`).
- Planned, not built: 「⋯ → 檢舉商家」 on store and product pages (`P/step10d-decision-brief.md:247-313`); 10d step unchecked (`W/backend/docs/plans/2026-09-29-a3-buyer-seller.md:297`).
- A7 Q5 PolicyCategory (`M/.../decision-brief.md:170`, `:336-339`).
