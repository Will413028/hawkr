# 獨立審查：step10e-3d-queue-brief.md v1（HEAD 4cc2ec079，唯讀）

路徑縮寫：W=<10e worktree>；S=W/backend/internal/modules/seller；I=S/internal；P=W/backend/docs/plans/2026-09-29-a3-buyer-seller。
標記：[量測]=我讀到該行或跑了指令；[推論]=由讀到的內容推得；[外部]=這次 WebFetch 讀到。

## 一、問題表

| 嚴重度 | 位置 | 問題 | 證據 | 建議修法 |
|---|---|---|---|---|
| BLOCKER | §1.3 寫入點、Q1「代價：約 25 個操作」、§3 | 寫入點低估約一半，而且漏掉的正好是會讓待辦**默默出錯**的轉移。實際是約 55 個（種類 × 轉移），分布在約 35 個函式（見第二節）。漏掉的包括：恢復營業 `closing.Reopen`（登錄回到待辦）、帳號刪除 `BeginDeletion`（店家進入 PENDING_DELETION，登錄離開）、終止合作生效的 job `closeTerminated`、第一段清理的 `WithdrawPendingFoodRegistrations`、`kyc.SaveDraft` 把已核准的 KYC 退回 DRAFT（登錄不能再裁定）、措施 `Update`（改 ends_at）與 `End`、`DecideRemediation` 接受（措施結束，複審項目要關）、`DecideAppeal` 接受（撤銷）、`SubmitAppeal` 先把已結束的解除請求結案、`Decide`／`Move` 裡由系統撤回詢問（詢問逾期項目要關，而且**不能**重新起算待決定）、`Move` 開新案件、seller 自己撤回或清除登錄（`foodreg.Withdraw`、`Clear`）、devseed。Will 看到的成本數字和實際差一倍 | [量測] `rg -n "q\.(…)\(" -g '!*_test.go' S/internal`（第二節每列都有 file:line）；I/closing/closing.go:72,217,358；I/closing/termination.go:118,142；I/purge/purge.go:230；I/kyc/kyc.go:131-171（APPROVED→DRAFT，:163 註解）；I/measure/update.go:55,180；I/measure/remediation.go:45；I/measure/appeal.go:127；I/governance/case.go:104、move.go:106 | 寫入點表放進簡報主文（函式、file:line、對每種待辦的效果：開／重新起算／關／改期限），數字改成實測值；Q1 的代價照實寫 |
| MAJOR | §1.2 close_reason「`CANCELLED`…例如店家關閉」、§1.1 4 條路徑的反駁 | 店家關閉不是「義務不存在了」。關店不會撤回待審的登錄（只有第一段清理才撤回，purge.sql `WithdrawPendingFoodRegistrations`「之後不會再重開」）；撤銷終止合作時 `closing.Reopen` 會讓 CLOSED → ACTIVE，同一筆 PENDING 登錄又可以裁定。照簡報的做法以 CANCELLED 關閉之後，沒有任何寫入點會重新開啟它，只能等核對 job「修」回來。3b D7(c) 說的「店家關閉讓登錄離開」其實是**雙向**的成員資格（關店、恢復營業、等待刪除、退場），簡報只處理了單向 | [量測] S/db/queries/stores.sql:116-142；S/db/queries/food_registrations.sql（`WithdrawPendingFoodRegistrations`：`s.retired_at IS NOT NULL`）；I/model/foodreg.go:187（`CanReject` = PENDING ∧ 店家是 ACTIVE） | 把「登錄可以裁定」的成員資格列成 (店家 ACTIVE) ∧ (KYC APPROVED) ∧ PENDING，並逐一列出改變三個條件的全部轉移；店家關閉改用 STOPPED，恢復營業時重新開啟 |
| MAJOR | §1.1「4 條路徑」第一列：「`auto_close_at` 只給解除請求」 | 複審到期也會因為措施結束而離開（成員資格要求措施生效中：`id=ANY(active_ids)`），所以也需要 `auto_close_at`；而且兩種的 `auto_close_at`（= ends_at）會被 `Update`、`End`、`DecideRemediation` 接受、closing 的 `endNotices` 改掉，Reschedule 只改 `due` 不夠（延後複審要同時改 `visible_from` 與 `due_at`）。「用 visible_from 表達、不需要 job」只對一部分成立：時間進入用 visible_from 可以，**時間離開**要用另一個會變動的欄位，而這個欄位本身又是一個要維護的寫入點 | [量測] S/db/queries/measures.sql:421-459（AdminMeasuresReviewDue）；I/measure/update.go:110,218,273；I/closing/closing.go:259-280；S/db/objects.sql:44（effective_measures 用 ends_at） | API 改成 `Reschedule(kind, subject, visible_from, due_at, auto_close_at)`；把 ends_at 的每一個寫入點列進寫入點表 |
| MAJOR | §1.4 核對：「比對種類、對象、開始、期限…修正成事實的樣子」與 §1.1「工作項目不是投影」 | 自相矛盾。(1) 如果工作項目是指令的結果、不是投影（用來反駁 ADR D4），核對 job 就不能拿事實去「修正」它；如果可以從事實重建，它就是投影（研究 B1 自己寫「the work item is a projection that must be rebuildable」）。(2) 核對會比較期限：`ReviewPolicy` 一改，`OpenObligations` 會算出新的期限，核對就把所有還開著的項目的期限改寫掉，正是簡報用來支持 (c) 的理由（「規則改了不改寫歷史」）被自己的修復機制推翻。(3) 自動修正會把第一列那類漏寫藏起來（開發期沒人看 error log） | [量測] brief :54、:97-99；ir B1 「Materialized…Drift」、B 段末「the work item is a projection that must be rebuildable」 | 決定它是什麼：權威紀錄（核對只告警、不修，比對時排除期限，或比對時用當時的政策版本）或投影（那就適用 ADR D4 與 ADR 2026-09-24 修訂第 5 點）。測試與開發環境讓不一致直接失敗，不自動修 |
| MAJOR | §1.1 約束帳「單體規則 2、D3 修訂第 1 點」；§2 E12「ADR 2026-09-24 不必加」 | 把 (c) 歸到 D3 修訂第 1 點（「資料的擁有者對嗎」）有爭議。期限由 seller 的 `ReviewPolicy` 從 seller 的事實算出，而且簡報要從 seller 的事實修復它，所以 seller 才是擁有者，`reviewqueue` 存的是副本，也就是第 5 點的讀取模型（「只在量測觸發時才建…用 outbox 非同步投影…這是終局」）。(c) 是同步寫入、沒有量測觸發，等於偏離第 5 點，簡報卻寫成「符合」 | [量測] W/backend/docs/adr/2026-09-24-go-modular-monolith.md:310-316 | 在約束帳明列「偏離 D3 修訂第 5 點（SELF-IMPOSED）」並附修訂文字；或論證為什麼它是第 1 點、不是第 5 點 |
| MAJOR | 開頭「本稿重開的是 3b D7(b)、D8 與 3cd D′1(c)」、§2 E12 | 簡報**沒有提**它推翻的一份已核准 ADR：ADR 2026-10-18-seller-governance-consolidated:113「推導，不儲存：…『待決定』的開始時間是 max(…)…詢問的…逾期…同樣在讀取時推導」，以及 :29 否決「存推導值、到期由 job 推回（兩份事實會分歧）」。rb R3 也寫「本模組的原則是讀取時推導（ADR 2026-10-02、設計書 32）」，而 rb R3 表後方的選項 (b) 明寫「存『等賣家』並暫停期限：期限要另存，會觸發 D7 與 D′1 的實體化」，也就是 rb R3 是刻意選了「可以推導的重新起算」，不是前提失效 | [量測] W/backend/docs/adr/2026-10-18-seller-governance-consolidated.md:29,113；P/step10e-reports-brief.md:186-201 | 在「重開的決定」補上 ADR 2026-10-18 consolidated（:29、:113）與 rb R3 選項 (b)，E12 加上對它的修訂 |
| MAJOR | §1.1「這些理由現在還成立嗎」最後一列「前提已不成立」、Q1「推導只被當成第一步」 | (1) D′1 自己訂的終局觸發（指派、認領、暫停時鐘、來源算不出的鍵、p95>300ms）一個都沒發生：重新起算已經推導出來了（`CaseDecisionClockStarts`，I/governance/governance.go:266-276），不是暫停。這次重開的真正原因是 Will 問「最好的做法」，簡報應該照實寫，不要寫成「前提失效」。(2) rb R3 的校準是「累積 20 筆結案取 P90」，用的是**實際處理時長**（開始到決定），兩個時間都已經是事實，不需要存下的期限；「當時有沒有準時」才需要，而這可以用帶生效日的政策版本推導出來，而且在上線前沒有任何資料。(3)「業界只當第一步」引的是 ir C，那是研究者自己的 [INFERENCE] 建議段，不是業界的證據 | [量測] P/step10e-3cd-brief.md 終局與觸發條件段；ir C 「Constraints that could justify deviating」（INFERENCE）；rb :184 | 把那一列改成「沒有觸發條件成立；重開是因為 Will 要求從零比對」；「第一步」改標 [推論，ir C]；達成率的需要寫成上線的觸發，而不是現在的事實 |
| MAJOR | §1.1 業界 第一段「ServiceNow、Zendesk、JSM…都是這個形狀」 | 業界基準被說過頭。ServiceNow 的 `task_sla`、Zendesk 的 policy metric、JSM 的 SLA 週期，都是**掛在業務實體上的 SLA 紀錄**；它們的佇列（Zendesk views、JSM 的 JQL queues、ServiceNow 的 task 清單）是對業務實體做的查詢，也就是推導出來的成員資格。只有 Salesforce Omni（`PendingServiceRouting`／`AgentWork`）、Coop 的 job、Stripe review 是獨立的工作項目，而且前兩者是為了路由與指派。3b 的查證（P/step10e-3-brief.md:357）已經寫過「Zendesk、JSM、ServiceNow 的工作實體就是業務實體」，本簡報與它相反卻沒有回應。業界對推導佇列的正面論點也被省略：「views 就是存下來的查詢」、不會漂移、overdue 只是 `due_at < now()` | [外部] Zendesk 5604663490458：「SLA targets have different statuses on a ticket」、view 依 SLA 遞增排序；ServiceNow DkrtAbz5…：「Each task_sla record represents an SLA that is attached to a task」；Coop reports.md：「create a moderation job, which gets routed to the appropriate review queue」；ir B1 第一列；P/step10e-3-brief.md:357 | 業界基準拆成兩件事分開寫：(i) SLA 紀錄另存（業界共識，支持存期限與週期）；(ii) 成員資格另存（只有需要路由、指派的產品這樣做）。回應 3b:357 |
| MAJOR | §1.1 選項表 | 漏了兩個可信的中間選項，正好落在上一列兩件事的中間：**(d)** 期限與週期存在 seller（每個義務的 `due_at`、`original_due_at` 存在領域列上或一張 seller 自己的 SLA 週期表），成員資格照舊推導；11 種全在 seller 的 schema 內，一支 `UNION ALL ORDER BY (due_at, id)` 就能走索引，不需要合併層與哨兵（3d v2 的合併層只是為了 A7 的第二個來源）。**(e)** (c) 的「重算」變體：照本模組現有的兩個先例 `exposure.Announce`（12 個呼叫點，把規則 SQL 的結果與上次存的值比對）與 `suspension.Recompute`＋對帳 job（ADR 2026-10-18:60），每個寫入點呼叫的是 `Sync(subject)`，從事實重算後寫入差異，而不是手寫 Open／Restart／Close 的語意。這樣漏寫只剩「漏呼叫」一種，不會有「呼叫了但語意寫錯」 | [量測] `rg -n "\.Announce\(ctx" -g '!*_test.go' S` 12 行；I/exposure/exposure.go:1-5；ADR 2026-10-18 consolidated:60；盤點 §15 第 1 點（8 種已有索引） | 把 (d)、(e) 列進選項表與約束帳；如果仍然建議 (c)，至少寫入 API 改採 (e) 的形狀 |
| MAJOR | §0 前提題「發票申訴 REAL（邊界外）」、選項表「(a) 發票進不來」、約束帳 | 依規則，我們自己的計畫分段不是 REAL。依賴表已經有 `invoice` 模組，搬遷計畫也有 A5 invoice，所以「邊界外」是暫時的計畫分段，是 SELF-IMPOSED。搬進來之後 (a) 也能把它加成一個來源，「(a) 進不來」不成立。用它當 (c) 的優點是說過頭 | [量測] W/backend/internal/arch/modules.go `"invoice": {…}`；W/backend/docs/plans/2026-09-24-go-modular-monolith.md:203 | 改判 SELF-IMPOSED（計畫分段，A5），從 (c) 的優點中拿掉 |
| MAJOR | §1.4、§1.3 鎖 | 核對 job 的修復沒有規定鎖。它在快照裡算 `OpenObligations`，再寫 `reviewqueue`，中間如果有指令 commit，就會把剛關掉的項目重新開啟，或關掉剛開啟的。現有指令都先鎖店家列（`stint.Lock`／`LockAccountStores`）再寫，所以每個項目的寫入都靠店家鎖排序；核對與第一段清理的批次更新（`FOR UPDATE SKIP LOCKED`＋`= ANY(ids)`）如果同時批次更新 `reviewqueue`、又沒有固定順序，也可能互鎖 | [量測] I/measure/jobs.go:38-50（先鎖帳號再重讀的先例）；I/purge/purge.go:12-14,199-233 | 規定「寫 reviewqueue 之前先鎖該項目所屬的店家列」，核對 job 照 closeEnded 的模式逐帳號鎖住再重讀；批次更新依 id 排序 |
| MINOR | §1.3「重複開啟回錯」、§6 測試「關閉不存在的」 | 冪等重送的語意沒有定。`CloseForDeletion`「已關閉時成功」、`BeginDeletion` 非 ACTIVE 時直接 return、建立措施、建立詢問、建立來函的 request_id replay：Close 一筆不存在的項目要報錯（重送就會失敗）還是靜默成功（會藏住漏開）？單體規則 3：撞唯一鍵後不能在同一個 transaction 繼續下 SQL，所以「重複開啟回錯」等於整筆指令失敗 | [量測] I/closing/closing.go:66-70,104-117；I/measure/create.go:314；W/backend/AGENTS.md:29 | 寫明每支 API 遇到重複或不存在時的行為，以及 replay 路徑在寫入之前就 return 的不變式 |
| MINOR | §1.2「回覆或撤回時開新的一列」 | 要限定為**管理員**撤回。`Decide`／`Move` 在結案時由系統撤回詢問，不能重新起算（ADR 2026-10-18:113「系統撤回（結案）不重新起算」；`withdraw_cause='ADMIN'`）；`ChangeSeverity` 在等賣家期間沒有開著的待決定列，Reschedule 要能處理這種情況 | [量測] I/governance/inquiry.go:400-403；P/../inventory §2（`FILTER (withdraw_cause='ADMIN')`） | 文字寫明這兩點 |
| MINOR | §2 E2「詳情頁顯示的就是工作項目的期限，一個來源」 | 不是一個來源。來函的 `due_at`、詢問的 `due_at` 是 seller 的領域事實（管理員輸入），詢問詳情的 `due_time` 是 `due_at`，待辦是 `due_at+24h`（3cb §6）。改成向 reviewqueue 取之後，來函會有兩份存下的期限 | [量測] P/step10e-3cb-design.md:1143；S/db/queries/authority_orders.sql:4-6 | 列出哪些資源的期限仍由 seller 輸出、哪些改由 reviewqueue 輸出 |
| MINOR | §1.1 選項表 (a)「排序鍵…不能用索引」 | 說過頭。盤點 §15 第 1 點：11 種裡有 8 種可以走既有的部分索引，算不出索引鍵的只有待決定（max 子查詢）與食品業者登錄 | 盤點 §15 第 1 點 | 改成「11 種裡有 2 種」 |
| MINOR | §0「§3 的表」、全文「設計書 32:105」 | §3 沒有表（寫入點清單在 §1.3）。設計書 32 的那句話現在在 :114，:105 是「通報沒有狀態欄」 | [量測] `sed -n 105p;114p` 設計書 32 | 改指 §1.3 與 32:114 |
| MINOR | 未交代的待決事項 | (1) 商家認證 SUBMITTED 的店家關閉之後，KYC 仍然是 SUBMITTED（清理只清個資、不改 status），兩種做法都會一直列在待辦裡，是一個產品問題；(2) 10f 會把詢問的期限改成「從出現在賣家中心起算」（3d v2 前提題），之後 visible_from 的語意會變，前提題沒有列；(3) 回填與 devseed（I/kyc/devseed.go）要寫入；(4) 依單體規則 4，下層模組的列要先鎖，reviewqueue 是 seller 的下層卻在 seller 之後寫，只有在「每個項目只有一個寫入模組」時才安全，要寫成不變式 | [量測] S/db/queries/purge.sql PurgeKYCApplications（不改 status）；P/step10e-3d-design.md 前提題第 3 列；W/backend/AGENTS.md:30 | 列進「沒有驗證的部分」或決定題 |
| MINOR | Q1、Q2 | Q1 大致是白話，但代價寫錯（見 BLOCKER），而且沒有列 (d)、(e)；Q1 的 (a) 只有一行、沒有它的優點（天生一致、不碰寫入路徑）。Q2 的 (ii)「necessity 不成立」是內部術語 | brief :154-161 | Q1 每個選項各列一行優點與代價；Q2 用白話寫 |

## 二、寫入點逐條（11 種 × 轉移）

「在簡報？」：Y＝§1.3 有列；P＝只有概括、沒有點到這個效果；N＝沒有。

| 種類 | 轉移 | 函式 | file:line | 在簡報？ |
|---|---|---|---|---|
| 1 待分案 | 開 | governance.Submit（InsertReport） | I/governance/report.go:92,132 | Y |
| 1 | 關 | Triage | I/governance/triage.go:57,106 | Y |
| 1 | 關 | Resolve（直接設定結果） | I/governance/resolve.go:30,72 | Y |
| 2 待決定 | 開（新案件） | Triage → InsertCase | I/governance/triage.go:49 | Y |
| 2 | 開（新案件） | Move（target OpenNew） | I/governance/move.go:19 | P |
| 2 | 關 NO_REPORTS | Move → CloseEmptyCase | I/governance/move.go:102 | Y |
| 2 | 停（等賣家） | CreateInquiry | I/governance/inquiry.go:130,191 | Y |
| 2 | 新週期（收到回覆的時間） | RecordInquiryReply | I/governance/inquiry.go:251,305 | Y |
| 2 | 新週期（只限 ADMIN 撤回） | WithdrawInquiry | I/governance/inquiry.go:333,355 | Y（沒有寫限定條件） |
| 2 | 關 | Decide | I/governance/case.go:52,95 | Y |
| 2 | 改期限（或改下一個週期） | ChangeSeverity | I/governance/case.go:130,169 | Y |
| 3 詢問逾期 | 開（visible_from=due_at） | CreateInquiry | I/governance/inquiry.go:191 | Y |
| 3 | 關 | RecordInquiryReply／WithdrawInquiry | inquiry.go:305,355 | Y |
| 3 | 關（系統撤回） | Decide → withdrawOpenInquiry | I/governance/case.go:104；inquiry.go:402 | N |
| 3 | 關（系統撤回） | Move → withdrawOpenInquiry | I/governance/move.go:106 | N |
| 4 待轉報 | 開（食安 ∧ ACTION_TAKEN ∧ 沒有不轉報） | Decide | I/governance/case.go:52 | P（決定前就可以不轉報，B15） |
| 4 | 關 | RecordFDAReferral／WaiveReferral | I/governance/referral.go:53,90；:107,132 | Y |
| 5 來函 | 開 | CreateAuthorityOrder（含 replay） | I/governance/authority.go:263 | Y |
| 5 | 關 | CompleteAuthorityOrder／RevokeAuthorityOrder | authority.go:491；:691 | Y |
| 6 告知被指名者 | 開（每個 WITHHELD 對象，visible_from=recheck 台北 00:00） | CompleteAuthorityOrder | authority.go:491 | Y（記下告知） |
| 6 | 換日期（關＋開）或關 | RecordAuthorityTargetNotice（ReplaceWithheld…／SetAuthorityTargetNotice） | authority.go:637；authority_orders.sql:135-140 | Y |
| 7 申覆 | 開 | SubmitAppeal → submitRequest | I/measure/appeal.go:40,131 | Y |
| 7 | 關 | DecideAppeal | appeal.go:238,282 | Y |
| 7 | 關 | Revoke／cascadeRevoke → closePendingOfRevoked | I/measure/revoke.go:232,279,294 | Y |
| 8 解除請求 | 開（auto_close_at=ends_at） | RequestRemediation → submitRequest | I/measure/remediation.go:32 | Y |
| 8 | 關 | DecideRemediation | remediation.go:45,77 | Y |
| 8 | 關 | Revoke（CloseRequests） | revoke.go:294 | Y |
| 8 | 關（已結束的） | SubmitAppeal → closeEndedRemediations | appeal.go:127 | N |
| 8 | 關 | CloseEndedRequests job | I/measure/jobs.go:34-50,92 | Y |
| 8 | 改 auto_close_at | measure.Update（AdvanceMeasureRevision） | I/measure/update.go:55,110 | N |
| 8 | 關 | measure.End → endNow | update.go:180,217 | N |
| 9 複審到期 | 開（visible_from=review_by−24h，auto_close_at=ends_at） | measure.Create（RISK） | I/measure/create.go:280,396 | Y |
| 9 | 改 visible_from＋due | ScheduleReview | update.go:240,273 | Y（API 只改 due） |
| 9 | 改 auto_close_at／關 | Update／End | update.go:55；:180 | N |
| 9 | 關 | Revoke／cascade | revoke.go:232,279 | Y |
| 9 | 關（措施結束） | DecideRemediation 接受 | remediation.go:45-92 | N |
| 9 | 關 | DecideAppeal 接受（撤銷） | appeal.go:238-295 | N |
| 10 食品業者登錄 | 開／換一筆 | foodreg.Submit（先撤回待審的再新增） | I/foodreg/foodreg.go:74,98,113,117 | Y |
| 10 | 關 | foodreg.Withdraw／Clear（賣家） | foodreg.go:134,149；:163,185 | N |
| 10 | 關 | foodreg.Decide | foodreg.go:308,361,367 | Y |
| 10 | 開／重新起算 | kyc Approve | I/kyc/admin.go:377 | Y |
| 10 | 停（KYC 不再是 APPROVED） | kyc.SaveDraft APPROVED→DRAFT（PayUni 拒絕） | I/kyc/kyc.go:131-171 | N |
| 10 | 停（店家不是 ACTIVE） | BeginDeletion（PENDING_DELETION） | I/closing/closing.go:72,86 | N |
| 10 | 停 | closing.close（CloseCurrent、CloseForDeletion、終止合作生效的 job closeTerminated） | closing.go:116,162,217；termination.go:118,142 | P（簡報寫 CANCELLED，也沒有列 job） |
| 10 | 重新開啟 | closing.Reopen（撤銷終止合作） | closing.go:358,366 | N |
| 10 | 關 | purge retireBatch → WithdrawPendingFoodRegistrations（批次） | I/purge/purge.go:199,230 | N |
| 11 商家認證 | 開 | kyc.Submit | I/kyc/kyc.go:359 | Y |
| 11 | 關 | kyc.Cancel／Approve／Reject | kyc.go:452；admin.go:377,438 | Y |
| 1–11 | 開（開發資料） | kyc devseed SeedApproved 等 | I/kyc/devseed.go:16-41 | N（只有提到回填） |

計數 [量測＋推論]：約 55 個（種類 × 轉移）效果，分布在約 35 個函式（3 個 River job：CloseEndedRequests、termination effect／reconcile、purge）；其中約 20 個效果是 N 或 P。

## 三、我驗證了什麼

- 讀了簡報全文、ir 全文、盤點全文、3d v2 :1-60、3b brief :13-58,143,178,247,305,357、3cd D′1 :54-136、rb R3 :155-201、3cb §6 :1131-1150、設計書 32 :36,105,114,188,195、ADR 2026-09-24 :304-318、ADR 2026-10-18 consolidated :29,60,113、modules.go、backend/AGENTS.md 規則 1-5。
- 用 `rg` 找出 seller 全部會改動相關表的 SQL（queries/*.sql 的 INSERT／UPDATE／DELETE），再找每支 SQL 在 internal/ 的呼叫點（排除 _test 與生成檔）；讀了 closing.go :1-130,217-300、measure/jobs.go 全文、kyc.go :87-260、purge.go :1-40,115-150 與 effective_measures、WithdrawPendingFoodRegistrations、PurgeKYCApplications、MarkPendingDeletion／CloseStore／ReopenStore 的 SQL。
- 3b D7(c) 的「4 條路徑」：KYC 核准（admin.go:377，在 InTx 裡，:421 有 Announce）與關店（closing.close，InTx）確實是 seller 在 transaction 內的寫入 [量測]；closeEndedWorker 逐帳號 InTx、先鎖再重讀，可以在裡面加 Close [量測]。但「店家關閉」是雙向的，複審到期也需要 auto_close_at（見表）。
- 外部 [WebFetch]：Zendesk 5604663490458、ServiceNow task_sla 欄位頁、ROOST Coop reports.md，三者都與 ir 的引述相符；ir C 的「first step」是研究者的 INFERENCE。
- 沒有做：沒有跑任何測試或 DB 查詢；沒有讀 A7 計畫；PayUni 的 observe 只確認它會呼叫 Announce，沒有確認它不改 KYC 的狀態（從 SQL 推論：只有 kyc.go 的 SaveDraft、Transition 會改）。
