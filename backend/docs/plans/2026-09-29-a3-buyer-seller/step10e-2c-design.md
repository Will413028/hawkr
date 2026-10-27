# 10e 第 2c 刀：可申訴的措施，實作設計規格 v2

讀取的是 `local/10e`（HEAD `35d937156`）。
SSOT：plan `backend/docs/plans/2026-09-29-a3-buyer-seller.md:295-302`（10e「可申訴的措施（Will 2026-10-12…」）；brief `…/step10e-measure-brief.md`（下稱 brief）；ADR `backend/docs/adr/2026-10-18-seller-governance-consolidated.md`（下稱 gov ADR；撰寫時依的是整合前的 `2026-10-02-seller-governance-in-monolith.md` 與當時的三次修訂）；錯誤模型 ADR `2026-09-25-api-error-model.md` D7；API 設計規範（Connect）；盤點 `2c-inventory.md`（下稱 inv）。路徑縮寫沿用 inv：`W`、`S`=`W/backend/internal/modules/seller`、`H`=`W/backend/internal/httpapi`。
判定原則（Will 2026-10-12）：每個取捨都以業界從零的做法定案，「和現有程式一致」不是理由，上線前沒有相容性約束。只有基準無法定案的產品或法務選擇才列給 Will（§9）。

本版重新核對過的外部基準：
- AIP-155（google.aip.dev/155，2026-10-13 重讀）：主要規則是「server **should** return the response for the previously successful request」；「Stale success responses」段允許「the method **may** return the current state of the resource instead」；「any reasonable timeframe」；request_id「must not be a field on resources」；**沒有**規定內容不同時怎麼處理。
- Stripe idempotent requests（docs.stripe.com/api/idempotent_requests，上一版已讀，critique 複核）：比對參數、不同就回錯（`idempotency_error`，HTTP 400）；驗證失敗不儲存；至少保留 24 小時。key 以帳號為範圍：**未驗證**，不作為依據。
- brief §2 已查過的 P2B 第 4、11 條、DSA 第 17、20、23 條、Google Merchant Center、Meta、YouTube 不重查。

---

## 0. 決策總表

| # | 缺口 | 決定 | 基準 |
|---|---|---|---|
| G1 | 措施狀態的唯一來源 | **刪除 `seller.suspensions`**。措施是唯一來源：儲存的只有 `measures.status`（ENFORCED／REVOKED）、`measures.created_at`（開始）與目前版本的 `ends_at`（版本表）。「停權效果是否生效」由這三者在讀取時推導，SQL 只寫在 `EffectiveMeasures` 一支。`suspension_periods` 保留，它是 Recompute 記下的帳號停權期間（事件身分），不是「是否停權」的第二來源 | gov ADR「停權狀態讀取時推導，處分表是唯一來源」；Wikipedia `page_latest`（內容只在版本表）；critique 修法 (a) |
| G2 | 對外狀態 | 新增 OUTPUT_ONLY 的 `MeasureState`：`ISSUED`（警告，未撤銷）、`ACTIVE`、`ENDED`、`REVOKED`（2c-3 加 `SCHEDULED`），由 `model.State(m, now)` 在讀取時推導，不儲存。`status` 不出現在契約。清單以 `state` 篩選，ACTIVE／ENDED 以 `EffectiveMeasures` 的結果定義 | AIP-216（狀態 OUTPUT_ONLY，由伺服器給）；gov ADR「讀取時推導」只限制儲存 |
| G3 | 認定被撤銷時引用它的措施 | 同一個 transaction 內，**撤銷所有引用該認定、仍 ENFORCED 的其他措施**，原因 `FINDING_REVOKED`（新值，只能由連帶撤銷寫入），不論是否還有其他有效的認定；那些措施上待處理的請求以 `CLOSED` 結案。仍有理由的話，管理員以剩下的認定另立新措施（新理由、新申覆權） | brief §2：Meta 申訴成立時「移除 strike 與限制」；YouTube 由 strike 導出的限制隨 strike 移除；P2B 4(3)「不遲延地恢復」；DSA 17(3)(d) 通知的事實已不成立 |
| G4 | 事件 | `MeasureCreated`、`MeasureRevised`、`MeasureEnded`、`MeasureRevoked`、`MeasureRequestSubmitted`、`MeasureRequestDecided`、`MeasureRequestClosed`（請求事件帶 `Kind`，2c-2 不必新增事件型別）；2c-3 加 `TerminationScheduled`。效果事件 `StoreSuspended`／`StoreUnsuspended` 不變。都在寫入的 transaction 內發 | gov ADR「通知走事件」；DSA 17(1)、Google：每個決定與變更都通知 |
| G5 | 權限 | `seller.measures.{list,get,create,update,end,revoke,schedule_review,decide_appeal}`、`seller.measure_notes.{create,list}`、`seller.measure_request_evidence.get`；刪除 `seller.stores.suspend`、`seller.suspensions.{list,lift}`、`seller.violations.{create,update,revoke,decide_appeal}`、`seller.violation_notes.*`、`seller.appeal_evidence.get`；保留 `seller.violations.{list,get}`。2c-2 加 `seller.measures.decide_remediation`；2c-3 刪 `seller.stores.close` | `adminperm.go:3-4` 一個方法一個權限；critique 已驗證刪除安全（只有 OWNER，無外部引用） |
| G6 | 複審期限 | RISK 必填 `review_by`，寫入時 ∈ (now, now＋30 天]；`ScheduleMeasureReview(review_by, note, etag)` 同 transaction 寫 review_by 與一則註記。只給後台看。到期進收件匣 `ListMeasures(review_due=true)`，不需要 job | brief §5；plan「上限 30 天」；Google MC 不公開複審日；AIP-136 |
| G7 | 延長與縮短 | `CanExtend`、`CanShorten` 都要求 `state==ACTIVE`。新的 `end_time` 必須晚於伺服器的 now（不得縮到過去：那會改寫賣家實際經歷的期間）。已結束的措施不能延長，再限制要 `CreateMeasure`（可引用同一筆認定） | brief §3「加重是新的措施」；DSA 17(1) 每個限制決定都要附理由通知 |
| G8 | `EndMeasure` 的範圍 | 只限 RISK（brief D4 `can_end`（RISK））。End 的語意是「風險已排除」；VIOLATION 依據是固定期間的處分，比例調整用縮短，措施有錯用撤銷 | brief §6 D4、§5；Google MC 的 issue resolved |
| G9 | request_id | 選填 `string request_id`（`field_info.format = UUID4`），範圍 (管理員, request_id)，存在措施列，永久保存。**先查再驗證**：只做無狀態的格式驗證就查 request_id；命中時比對 hash，相同就回傳**措施目前的狀態**（AIP-155 的 stale-success 例外，不是主要規則），不同回 `invalid_argument`（欄位 `request_id`，規則來自 brief D5 與 Stripe，AIP-155 沒有規定）；沒命中才做有狀態的驗證並寫入 | AIP-155；Stripe；brief §6 D5 (a)；錯誤模型 ADR D7（帶 request_id 的方法會自動重試） |
| G10 | 註記與 etag | 寫註記不帶 etag、不動 `measures`（`version` 不變），**兩種 etag 都不變**。`ScheduleMeasureReview` 改的是根上的 `review_by`，後台 etag 變、賣家 etag 不變 | gov ADR 修訂 2026-10-11（註記在 aggregate 邊界外）；AIP-154 |
| G11 | 賣家與後台的 etag | 賣家 etag 由 `current_revision`（賣家看得到的版本）產生；後台 etag 由 `version` 產生（版本、狀態、review_by、請求狀態改變時加一） | AIP-154：etag 反映 client 看到的表示 |
| G12 | 升級規則的鍵 | 改以**帳號**計：`violations` 加 `account_id NOT NULL`，索引 `violations_escalation (account_id, created_at)`，在 `LockAccountStores` 的帳號鎖內計數 | YouTube、Meta 的 strike 以帳號累計；gov ADR 2a 修訂（效果以帳號為範圍，新 stint 繼承）；以店計會讓「關店重開」歸零 |
| G13 | 依據的不變量 | 所有認定只由 `CreateMeasure` 建立；`violations.origin_measure_id NOT NULL`；`measure_findings` 存全部依據；VIOLATION 依據至少一筆由 `measures.lead_finding_id` 加可延遲複合 FK 保證 | brief §3 E；repo 的 `violations_current_revision_fkey` 先例 |
| G14 | 申訴、補正、撤銷的互動 | 每個措施同時最多一筆待處理的請求（不分種類，DB 唯一索引）。待處理的**申訴**擋 `RevokeMeasure`（只能經 `DecideAppeal`）；待處理的**補正**不擋，撤銷時以 `CLOSED` 結案 | 4A-3 修訂（申訴只由裁定結束）；Google MC 同時一個審查 |
| G15 | 終止（2c-3） | 預告 14 天（21.2）或立即（21.3）；生效時停權效果開始並進入「待關閉」；申訴不暫停排程。**生效後接受申訴＝恢復**：撤銷措施、取消待關閉；店已關閉時重新開啟同一個店（同 store_id、店號與評價），KYC 個資已清理則回到需補 KYC 的狀態。重新入駐 → W1 | P2B 4(3)「不遲延地恢復」；DSA 20(4)；條款 21.2–21.4 |
| G16 | 賣家 GetMyStore | 刪除 `Suspension`、`SuspensionSource`、`SuspensionReason`；新增 OUTPUT_ONLY `bool suspended`，與 capability 的 `NOT_SUSPENDED` 由同一個 `stint.Derive` 結果產生（測試鎖住）。公告讀 `ListMyMeasures(state=ACTIVE)` | brief D6；AIP-216（client 不從能力清單反推帳號狀態） |
| G17 | legacy 二分 | 模組輸出 `Suspended bool`＋生效中措施的依據清單；`internal/legacy` 推導：有 RISK（2c-3 起含 TERMINATION）→ 9603 與 reasonLabel＝風險依據名稱；只有 VIOLATION → `SUSPENDED_GOVERNANCE` | brief §8 legacy（Java 不依代碼分支，已驗證） |
| G18 | 申訴佐證 | 掛在請求上（`measure_request_evidence`），`evidence_accesses.request_id`，media ref_id＝request_id；回填時 measure_id、request_id 都沿用 violation_id | ADR 2026-10-03（media 引用以擁有者為準） |
| G19 | 本機資料 | 違規 1:1 回填成措施（機械）；VIOLATION 處分列併入措施（核對不一致就 RAISE）。**ADMIN 處分列沒有誠實可用的 `risk_ground`** → W2（§9）；其他欄位的轉換已定義（§1 G19） | brief §8；gov ADR 2026-10-11 修訂的「對不上就 RAISE，不寫出錯誤的歷史」先例 |
| G20 | 條款改版 | 改正本的「版本：」行 → `pnpm gen:legal` → `agreement_gen.go`；不改舊版內容；上線前不做既有賣家重新同意 | 條款 22、23；`model/agreement.go:28-71` |
| G21 | 複核者、申訴範圍 | 照 brief：複核者不擋（18.4「或具相當權限者」，觸發是第二個角色）；WARNING 可申訴；VIOLATION 依據不提供補正 | brief §5、§6 D6 |
| — | 先行刀「接受申訴必附理由」 | 不另開：新 `measure_requests` 表一開始就帶 CHECK | 避免同一條 CHECK 搬兩次 |

**要 Will 決定的只有 W1、W2**（§9）。

---

## 1. 逐項說明（只寫不明顯的理由與限制）

### G1 唯一來源
- 現況（`suspensions.sql:1` 的 `EffectiveSuspensions` 只看 `lifted_at`、`ends_at`）把「效果」存成另一張表；措施化之後 `lifted_at` 與 `measures.status`、`suspensions.ends_at` 與目前版本的 `ends_at` 兩兩重複，一對一的唯一索引等於影子表。從零的做法是決定與效果同一列：效果是決定在某個時間點的推導。
- gov ADR 2026-10-11 修訂保留 `suspensions.ends_at` 的理由是「版本是賣家看到的歷史快照，處分表是運作狀態」。措施化後版本不可變、`current_revision` 前進，目前版本的 `ends_at` 就是運作狀態，那個理由消失。2c-1 的 ADR 修訂明寫取代這一段。
- 生效的唯一定義（`EffectiveMeasures`，取代 `EffectiveSuspensions`）：
  `m.status='ENFORCED' AND m.kind='SUSPENSION' AND m.created_at <= @now AND (r.ends_at IS NULL OR r.ends_at > @now)`，`r` 是 `(m.id, m.current_revision)` 的版本。`now` 由呼叫方注入（沿用）。2c-3 加 `kind='TERMINATION' AND m.effective_at <= @now`。
- 消費者全部改經它：`stint.Derive`／`DeriveMany`、`Recompute`、身分快照清理、`ListMeasures`／`ListMyMeasures` 的 `state` 篩選。
- `measure_identities(measure_id FK measures, kind, key_id, hash)` 取代 `suspension_identities`（換 FK，語意不變）；`kyc.sql:280-298` 改 JOIN `measures`（直接用 `m.account_id`）。
- `suspended_by_admin_id`、`lifted_by_admin_id` 隨表刪除：誰建立在 `measures.created_by_admin_id`，誰結束在版本的作者，誰撤銷在 `revoked_by_admin_id`。`rg` 確認的讀者只有 `suspensions.sql` 與 CARRYOVER 文字（`rg -n "suspended_by_admin_id|lifted_by_admin_id" backend/internal -g '!**/gen/**' -g '!*_test.go' -g '!**/migrations/**'`）。
- 規模：對帳掃描所有 ENFORCED 的停權措施（已到期的仍是 ENFORCED），部分索引 `measures (account_id) WHERE status='ENFORCED' AND kind='SUSPENSION'`。治理措施的量級（每帳號個位數）下每小時掃一次可以忽略。這不是暫時寫法：不存推導值是刻意的；若量級改變，正解是對帳只掃「有進行中期間」與「最近 N 天有寫入」的帳號，而不是存一份到期旗標。
- 限制清單：無推離基準的限制。

### G2 MeasureState
| 儲存 | kind | 時間 | state |
|---|---|---|---|
| REVOKED | 任何 | — | REVOKED |
| ENFORCED | WARNING | — | ISSUED |
| ENFORCED | SUSPENSION | 在 `EffectiveMeasures` 內 | ACTIVE |
| ENFORCED | SUSPENSION | 不在 | ENDED |
| ENFORCED | TERMINATION（2c-3） | now < effective_at | SCHEDULED |
- `model.State` 與 `EffectiveMeasures` 是兩處寫法，以邊界測試（`ends_at == now`、±1µs、`created_at == now`）鎖住「`State==ACTIVE` ⇔ 在 `EffectiveMeasures` 結果內」。清單的 `state=ACTIVE` 實作成 `m.id = ANY(@active_ids)`（`ENDED` 是 `<> ALL`），`@active_ids` 在同一個 transaction 由 `EffectiveMeasures` 取得，所以 SQL 的定義只有一份。
- 警告用 `ISSUED` 而不是 ACTIVE：警告沒有限制，標成「生效中」會讓停權公告誤列。

### G3 連帶撤銷
- 觸發點只有兩個：`RevokeMeasure(revoke_findings=true)` 與 `DecideAppeal(ACCEPTED, revoke_findings=true)`，撤銷 `origin_measure_id = 本措施` 的認定。被引用的舊認定不因新措施的申訴被撤銷（brief §3，不變）。
- 對每一筆被撤銷的認定 F：`SELECT measure_id FROM measure_findings WHERE violation_id = F` 之中 ENFORCED、不是本措施者，一律撤銷（`FINDING_REVOKED`、`revoked_by_admin_id`＝本次的管理員），遞迴不會發生（連帶撤銷不撤銷認定）。
- 為什麼「還有其他有效認定」也撤銷：措施是管理員對整組依據做的比例判斷（17.3），通知給賣家的事實（DSA 17(3)(d)）已經有一部分不成立；保留原措施等於讓賣家繼續承受依據已變的決定。另立新措施成本低（可引用剩下的認定），而且賣家因此取得新的申覆權。排除的選項：拒絕撤銷 F（會擋住 M1 的申訴裁定）；只放進收件匣（需要新的儲存旗標，且限制在等待期間持續）。
- 待處理請求的結案：`CLOSED`（`decided_at`、`decided_by_admin_id` 有值，`decision_reason` NULL），發 `MeasureRequestClosed`。賣家看到的是措施 `REVOKED`、原因 `FINDING_REVOKED`。
- 升級標記不重算：`escalated_from` 記的是當時的計數，嚴重度不機械決定措施（brief §1），重算不改變任何措施。觸發：嚴重度開始自動決定措施時。

### G7、G8 期間的規則
- `UpdateMeasure` 的 `end_time`：設值時必須 > now；比目前晚是延長（需 `CanExtend`），比目前早是縮短（需 `CanShorten`）；RISK 從無期限改成有期限算縮短；RISK 清除 `end_time`（mask 含 end_time、值未設）算延長；VIOLATION 依據不能清除；WARNING 不能設。
- End 只給 RISK：VIOLATION 依據若要立刻停止，表示措施有錯（撤銷）或經申訴裁定；其餘用縮短。這樣「縮到現在」與 End 不會是兩條產生同一種版本的路徑（縮短必須晚於 now）。

### G9 request_id 的流程
1. handler 與模組的**無狀態**驗證（REQUIRED、長度、enum、UUID 格式、oneof）。結果與時間無關，重送時一定與第一次相同。
2. 在新 transaction 讀 `(created_by_admin_id, request_id)`：
   - 命中、hash 相同 → 回傳措施**目前**的狀態與目前的 etag。選例外而非主要規則：主要規則要存第一次的回應，回應裡的 etag 一旦措施被改過就是舊的，client 拿它做下一次寫入一定 aborted；措施本身就是回應的內容，不需要另存回應本文。
   - 命中、hash 不同 → `invalid_argument`，BadRequest 欄位 `request_id`（brief D5；Stripe 的 `idempotency_error`；AIP-155 沒有規定）。
3. 沒命中 → 有狀態的驗證（店家開著、`review_by` ∈ (now, now+30d]、引用的認定同帳號且 ENFORCED）→ 寫入。驗證失敗不留紀錄（Stripe），重送照常驗證。
4. 並行：第二個寫入撞 `measures_request`（23505）→ 新 transaction 回到步驟 2。
- hash：正規化後的 `CreateMeasureInput`（不含 request_id、時間取微秒、repeated 照輸入順序）以固定欄位順序編碼的 SHA-256，只在模組一處計算。
- 保存：永久（措施不刪），在 AIP-155「any reasonable timeframe」內，涵蓋 Stripe 的 24 小時下限。
- 選填：AIP-155「should be optional」；admin client 一律帶（測試保護）。

### G12 升級以帳號計
- `CountRecentMediumViolations` 改以 `account_id` 查 90 天窗口，鎖改成 `LockAccountStores`（建立措施本來就要鎖帳號）。`violations.store_id` 保留，記認定發生的 stint。回填 `account_id` 取自 `stores`。
- 寫進 `CARRYOVER.md` 沿用盤點：原為以店計（設計書 32），從零的答案是以帳號計。

### G15 終止（2c-3 的形狀）
- `kind=TERMINATION`、不可變的 `effective_at`、`TerminationGround`（21.2 `NOTICE`，或 21.3 各事由）；21.2 是建立＋14 天，21.3 立即。依據照常存在。
- 生效（`state` 由 SCHEDULED 變 ACTIVE，推導）：停權效果開始，job `seller.termination_effect`（以 measure_id 為唯一鍵）把店家設為「待關閉」，未結訂單完成後關店（21.4）。等待刪除的「等未結訂單」機制能否共用：**未驗證**，2c-3 開工時盤點。
- 申訴不暫停排程（DSA 20 沒有暫停效力；預告期就是申訴期間）。生效前接受申訴：撤銷，排程自然失效（state 推導）。生效後接受申訴：撤銷、取消待關閉；已關閉就重新開啟同一個店。P2B 4(3) 要求恢復，所以這不是 Will 的選項；機制（關閉是否可逆、`stores_closure_check` 的改法、KYC 已清理時的狀態）是 2c-3 的實作盤點。
- `CloseStore`（admin）與 `CloseStoreDialog` 刪除，改建立 TERMINATION 措施。

### G19 本機資料的回填
- 違規 → 措施：id 沿用；VIOLATION 依據；kind 取 penalty；`lead_finding_id`＝自己；`measure_findings(id,id)`；`origin_measure_id=id`；版本與註記改表名；`violations.account_id` 由 stores 帶入。申訴欄位 → `measure_requests`（id 沿用 violation_id）。已接受但沒附理由的申訴（舊 CHECK 允許）填「（裁定時未記錄理由）」：文字照實描述，migration 測試斷言、CARRYOVER 註明。
- VIOLATION 處分列 → 併入措施，刪除列。先核對：`suspensions.ends_at` 等於該違規目前版本的 `sanction_ends_at`；`lifted_at IS NOT NULL` ⇔ 違規 REVOKED。對不上就 `RAISE EXCEPTION` 列出 ID（gov ADR 2026-10-11 的先例：不寫出錯誤的歷史）。`suspension_identities` 改掛 measure_id。
- ADMIN 處分列（W2 決定前的已定部分）：id 沿用 suspension_id；basis RISK；第 1 版的 `statement` 取 `detail`，沒有時取原因標籤（brief §8），作者 `suspended_by_admin_id`、時間 `suspended_at`、`ends_at` NULL；已解除的再寫第 2 版 `ends_at = lifted_at`、作者 `lifted_by_admin_id`（等同 End）；`review_by` ＝ migration 執行時間，也就是**立即到期**：這些措施從未經過新規則的複審，誠實的狀態是「待複審」，它們會立刻出現在收件匣，管理員結束或以 `ScheduleMeasureReview` 寫理由延後。不用「migration 時間＋30 天」：那會憑空給出一段沒人做過的複審。
- 不在 migration 內刪資料：brief §8 要求轉換，repo 沒有授權刪除的規則（上一版引用的「本機資料可以清掉」規則不存在，已撤回）。

---

## 2. 切分與審查

| 刀 | 範圍 | 為什麼可以獨立 CI 綠燈、不需要轉接層 |
|---|---|---|
| **2c-1 措施模型** | schema 與 migration（含回填、測試）、measure 服務、三份 proto、handler、adminauth、adminperm、declaration、檔案路由、legacy 三處、devseed；admin 4 個元件與 store-row；website 違規頁、申訴、公告、store.ts（吸收 4C 公告與 :105 文案）；seller-app 4B 畫面、公告、入口、my_store；文件（gov ADR 修訂、CARRYOVER、`S/AGENTS.md`、設計書 32 §3.2/§3.3/§3.5/§4.6/§5.1/§6.2/§13、plan） | 契約一次到位、所有消費端同一個 commit；任何再切都要 adapter（critique 已確認） |
| **2c-2 補正請求** | `measure_requests.kind` 加 `REMEDIATION`（只增值的 migration）、`RequestRemediation`、`DecideRemediation`、`can_request_remediation`／`can_decide_remediation`、權限；website 與 seller-app 入口；條款 17.4 改版 | 只新增 RPC 與欄位 |
| **2c-3 終止** | `TERMINATION`、`TerminationGround`、`effective_at`、`SCHEDULED`、job 與待關閉、恢復、刪 `CloseStore`；條款 21 改版 | 等 W1 |
| 之後 | 10e 第 3 刀：admin 治理介面 | 功能未做，不是暫時寫法 |

**2c-1 的審查（一個 commit、分層審）**：
1. 實作可平行（backend、admin、website、seller-app 各一個 subagent，以同一份 proto 為準），合成一個工作區 diff。
2. commit 前四個**沒參與實作**的 fresh reviewer 各審一層：backend（schema、migration、模組、handler、legacy）、admin、website、seller-app。每位拿到本規格 §3–§6 與自己那層的 diff，回報以契約與驗收測試為準的問題。
3. 再跑 design-review skill（沿用機制是否重新評估、有沒有繞過結構的補丁、有沒有留待決）。
4. 全部 CI 等價檢查綠燈後才 commit；`git log -1 --stat` 核對。

---

## 3. 2c-1 契約草圖

### 3.1 共用列舉（`hawkr.seller.v1alpha1`）
```proto
enum MeasureKind  { MEASURE_KIND_UNSPECIFIED = 0; MEASURE_KIND_WARNING = 1; MEASURE_KIND_SUSPENSION = 2; } // 2c-3: TERMINATION = 3
enum MeasureBasis { MEASURE_BASIS_UNSPECIFIED = 0; MEASURE_BASIS_VIOLATION = 1; MEASURE_BASIS_RISK = 2; }
// 條款 17.1 的風險類型與 17.5 的外部要求。
enum RiskGround { RISK_GROUND_UNSPECIFIED = 0; TRANSACTION = 1; FOOD_SAFETY = 2; FRAUD = 3; PERSONAL_DATA = 4;
  INFRINGEMENT = 5; SECURITY = 6; FULFILLMENT = 7; EXTERNAL_ORDER = 8; }       // 值名省略前綴，實際加 RISK_GROUND_
// 伺服器在讀取時推導，不儲存（AIP-216）。
enum MeasureState { MEASURE_STATE_UNSPECIFIED = 0; ISSUED = 1; ACTIVE = 2; ENDED = 3; REVOKED = 4; } // 2c-3: SCHEDULED = 5
enum MeasureRequestState { ..._UNSPECIFIED = 0; PENDING = 1; ACCEPTED = 2; REJECTED = 3; CLOSED = 4; }
```
刪除：`ViolationPenalty`、`SuspensionSource`、`SuspensionReason`、`AppealStatus`、`SanctionPeriod`。保留 `ViolationCategory`、`ViolationSeverity`、`ViolationStatus`（認定）。

### 3.2 賣家（`SellerAppService`）
```proto
rpc ListMyMeasures(ListMyMeasuresRequest) returns (ListMyMeasuresResponse) { option idempotency_level = NO_SIDE_EFFECTS; }
rpc GetMyMeasure(GetMyMeasureRequest) returns (GetMyMeasureResponse)       { option idempotency_level = NO_SIDE_EFFECTS; }
rpc SubmitAppeal(SubmitAppealRequest) returns (SubmitAppealResponse);     // measure_id, reason REQUIRED, evidence_staged_tokens(≤5), etag REQUIRED

message MeasureFinding { string id = 1; ViolationCategory category = 2; ViolationSeverity severity = 3; google.protobuf.Timestamp create_time = 4; }
message MeasurePeriod  { google.protobuf.Timestamp start_time = 1; google.protobuf.Timestamp end_time = 2; } // 無期限不設 end_time；全部 OUTPUT_ONLY
message MeasureRequest { string id = 1; MeasureRequestState state = 2; string reason = 3; google.protobuf.Timestamp submit_time = 4;
                         string decision_reason = 5; google.protobuf.Timestamp decide_time = 6; }
message Measure {
  string id = 1 [IDENTIFIER];
  MeasureKind kind = 2; MeasureBasis basis = 3;
  repeated MeasureFinding findings = 4;   // VIOLATION 依據；第一筆是主要依據
  RiskGround risk_ground = 5;
  string statement = 6;                   // 目前那一版
  MeasureState state = 7;
  MeasurePeriod period = 8;               // WARNING 不設
  MeasureRequest appeal = 9;
  google.protobuf.Timestamp create_time = 10;
  string etag = 11;                       // 由 current_revision 產生
  bool can_appeal = 12;
  RevokeReason revoke_reason = 13;        // REVOKED 時；FINDING_REVOKED 讓賣家知道為什麼撤銷
}                                         // 除 id 外全部 OUTPUT_ONLY
message ListMyMeasuresRequest { int32 page_size = 1; string page_token = 2; MeasureState state = 3; } // UNSPECIFIED＝不篩
```
`GetMyStoreResponse`：`reserved 2; reserved "suspension";`，新增 `bool suspended = 6 [OUTPUT_ONLY]`（與 `NOT_SUSPENDED` 同源）。service 說明改成「措施與申訴以帳號為範圍」。`declaration.go` 三支方法宣告 `exempt(appealReason)`。

### 3.3 後台（`SellerGovernanceAdminService`）
```proto
rpc CreateMeasure(CreateMeasureRequest) returns (CreateMeasureResponse);
rpc GetMeasure(...)   { NO_SIDE_EFFECTS }   rpc ListMeasures(...) { NO_SIDE_EFFECTS }
rpc UpdateMeasure(UpdateMeasureRequest) returns (UpdateMeasureResponse);       // mask：statement、end_time；etag 必填
rpc EndMeasure(EndMeasureRequest) returns (EndMeasureResponse);                // id、etag；只限 RISK
rpc RevokeMeasure(RevokeMeasureRequest) returns (RevokeMeasureResponse);       // id、reason（不可 APPEAL_ACCEPTED／FINDING_REVOKED）、revoke_findings、etag
rpc ScheduleMeasureReview(...) returns (...);                                  // id、review_by、note REQUIRED、etag
rpc DecideAppeal(DecideAppealRequest) returns (DecideAppealResponse);          // measure_id、decision、reason REQUIRED、revoke_findings、etag
rpc CreateMeasureNote / ListMeasureNotes;                                      // 不帶 etag（G10）
rpc GetViolation / ListViolations;                                             // 只讀認定（store_id、account_id、status）
// 刪除：CreateViolation、UpdateViolation、RevokeViolation、CreateViolationNote、ListViolationNotes

enum RevokeReason { …; REVOKE_REASON_FINDING_REVOKED = 5; }  // 只由連帶撤銷寫入

message CreateMeasureRequest {
  string request_id = 1 [(google.api.field_info).format = UUID4];
  string store_id = 2 [REQUIRED];
  MeasureKind kind = 3 [REQUIRED];
  string statement = 4 [REQUIRED];                 // 1–2000
  google.protobuf.Timestamp end_time = 5;          // VIOLATION＋SUSPENSION 必填；WARNING 不可設；RISK 選填
  google.protobuf.Timestamp review_by = 6;         // RISK 必填 (now, now+30d]；其他不可設
  string note = 7 [REQUIRED];                      // 第一則內部註記
  oneof basis { ViolationBasis violation = 8; RiskBasis risk = 9; }
}
message ViolationBasis { NewFinding finding = 1; repeated string cited_finding_ids = 2; } // 合計至少一筆
message NewFinding { ViolationCategory category = 1 [REQUIRED]; ViolationSeverity severity = 2 [REQUIRED]; string source_report_id = 3; }
message RiskBasis { RiskGround ground = 1 [REQUIRED]; }

message Measure {   // 後台
  id [IDENTIFIER]; store_id, account_id; kind, basis; repeated Violation findings; RiskGround risk_ground;
  string statement; google.protobuf.Timestamp end_time;          // 只有這兩個經 UpdateMeasure 修改
  google.protobuf.Timestamp review_by;
  MeasureState state; RevokeReason revoke_reason; revoked_by_admin_id; revoke_time;
  MeasurePeriod period; AdminMeasureRequest appeal;              // 含 evidence、revision、decided_by_admin_id
  created_by_admin_id, create_time, update_time;
  string etag;                                                   // 由 version 產生
  int32 current_revision; repeated MeasureRevision revisions;    // 只在單筆讀取與寫入回應
  bool can_update_statement, can_extend, can_shorten, can_end, can_revoke, can_schedule_review, can_decide_appeal;
}                                                                // statement、end_time、etag 以外 OUTPUT_ONLY；不輸出 request_id（AIP-155）
message Violation { id; store_id; account_id; source_report_id; category; severity; escalated_from; ViolationStatus status;
                    RevokeReason revoke_reason; revoke_time; string origin_measure_id; create_time; } // 全部 OUTPUT_ONLY
message ListMeasuresRequest { page_size; page_token; string store_id; string account_id; MeasureState state; MeasureBasis basis;
                              MeasureRequestState appeal_state; bool review_due; }
```
排序：新到舊；`appeal_state=PENDING` 時舊到新（2c-2 起改為 `request_state=PENDING`，申訴與補正同一個收件匣）；`review_due=true` 時依 review_by 舊到新。`review_due` ＝ RISK、`state=ACTIVE`、`review_by ≤ now`。

### 3.4 後台 `seller.proto`
刪除 `SuspendStore`、`LiftSuspension`、`ListSuspensions`、`StoreSuspension`。`StoreListItem`：`reserved 2`，新增 `bool suspended = 4`；`ListStores.suspended` 篩選保留（經 `EffectiveMeasures(all_accounts)`）。`GetStoreResponse`：`reserved 2, 5`，新增 `repeated Measure active_measures = 6`（同一次讀取）。歷史改用 `ListMeasures(account_id)`。

### 3.5 錯誤（不新增 `ErrorReason`）
| 情形 | code | details |
|---|---|---|
| request_id 相同、內容不同 | invalid_argument | BadRequest `request_id` |
| VIOLATION 沒有認定；引用的認定別的帳號或已撤銷 | invalid_argument | `violation.cited_finding_ids[i]` 或 `violation` |
| RISK 缺 review_by 或超出 (now, now+30d]；WARNING 設 end_time；VIOLATION 清除 end_time；end_time ≤ now | invalid_argument | 指向該欄位 |
| `RevokeMeasure` 帶 APPEAL_ACCEPTED／FINDING_REVOKED；RISK 措施帶 `revoke_findings=true` | invalid_argument | `reason`／`revoke_findings` |
| 凍結後延長或改理由；措施不是 ACTIVE 時延長、縮短、結束、複審；End 用在 VIOLATION；待處理申訴時撤銷；已撤銷 | failed_precondition | 只有 LocalizedMessage（4A-3：不帶欄位、不帶 reason） |
| etag 不符 | aborted | `VERSION_CONFLICT` |
`H/rpc/fields.go` 新增：`RequestID`、`Violation.CitedFindingIDs[]`、`ReviewBy`、`EndsAt`、`Note`、`StatementToSeller`、`RiskGround`、`Reason`、`RevokeFindings`。

### 3.6 狀態機與前置條件（`model/measure.go`，`can_*` 與方法共用）
```
CanUpdateStatement = status==ENFORCED && state∈{ISSUED,ACTIVE} && appeal==nil
CanExtend          = state==ACTIVE && appeal==nil
CanShorten         = state==ACTIVE
CanEnd             = state==ACTIVE && basis==RISK                // 2c-2：&& 沒有待處理的補正
CanRevoke          = status==ENFORCED && !(pending!=nil && pending.kind==APPEAL)
CanScheduleReview  = state==ACTIVE && basis==RISK
CanDecideAppeal    = appeal!=nil && appeal.state==PENDING
CanAppeal（賣家）   = status==ENFORCED && appeal==nil && pending==nil   // ENDED 也可申訴（17.4 不設期限）
```
- `DecideAppeal(ACCEPTED)`：撤銷（`APPEAL_ACCEPTED`），`revoke_findings=true` 時撤銷自己建立的認定並連帶撤銷（§1 G3）。`REJECTED` 時措施不變。
- `RevokeMeasure`：待處理的補正（2c-2）在同一個 transaction 以 `CLOSED` 結案。
- 跨表不變量（model 保證＋測試）：接受申訴 ⇔ `APPEAL_ACCEPTED`；`FINDING_REVOKED` ⇒ 依據裡有 REVOKED 的認定；REVOKED 的措施沒有 PENDING 的請求。

### 3.7 Schema 草圖（`S/db/schema.sql`）

> 2026-10-14 補註（Will 選 A，ADR 同日修訂「措施的目前值放在主表、版本表只存歷史」）：給賣家的理由與 `ends_at` 的目前值存在 `measures`（系統版本表的主表），`measure_revisions` 是含目前那一版的快照歷史；下方草圖與其中「`measure_revisions.ends_at` 是唯一的期間來源」以此為準，取代原草圖的說法（原草圖保留作歷史）。

```sql
CREATE TABLE seller.measures (
  id UUID PRIMARY KEY, account_id BIGINT NOT NULL, store_id BIGINT NOT NULL REFERENCES seller.stores,
  kind TEXT NOT NULL CHECK (kind IN ('WARNING','SUSPENSION')),
  basis TEXT NOT NULL CHECK (basis IN ('VIOLATION','RISK')),
  risk_ground TEXT CHECK (risk_ground IN ('TRANSACTION','FOOD_SAFETY','FRAUD','PERSONAL_DATA','INFRINGEMENT','SECURITY','FULFILLMENT','EXTERNAL_ORDER')),
  lead_finding_id UUID, review_by TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'ENFORCED' CHECK (status IN ('ENFORCED','REVOKED')),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version >= 1), current_revision INTEGER NOT NULL CHECK (current_revision >= 1),
  request_id UUID, request_hash BYTEA,
  created_by_admin_id BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
  revoked_reason TEXT, revoked_by_admin_id BIGINT, revoked_at TIMESTAMPTZ,
  CONSTRAINT measures_basis_check CHECK ((basis='RISK') = (risk_ground IS NOT NULL) AND (basis='VIOLATION') = (lead_finding_id IS NOT NULL)
                                         AND (basis='RISK') = (review_by IS NOT NULL) AND (basis <> 'RISK' OR kind = 'SUSPENSION')),
  CONSTRAINT measures_review_check CHECK (review_by IS NULL OR review_by > created_at),
  CONSTRAINT measures_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL) AND (request_hash IS NULL OR octet_length(request_hash) = 32)),
  CONSTRAINT measures_revoked_check CHECK (((status='REVOKED') = (revoked_reason IS NOT NULL)) AND …同 violations_revoked_check，5 個原因…),
  CONSTRAINT measures_admin_check …
);
ALTER TABLE seller.measures
  ADD FOREIGN KEY (id, current_revision) REFERENCES seller.measure_revisions (measure_id, revision) DEFERRABLE INITIALLY DEFERRED,
  ADD FOREIGN KEY (id, lead_finding_id) REFERENCES seller.measure_findings (measure_id, violation_id) DEFERRABLE INITIALLY DEFERRED;
CREATE UNIQUE INDEX measures_request ON seller.measures (created_by_admin_id, request_id) WHERE request_id IS NOT NULL;
CREATE INDEX measures_account ON seller.measures (account_id, created_at DESC, id DESC);
CREATE INDEX measures_created ON seller.measures (created_at DESC, id DESC);
CREATE INDEX measures_enforced_suspension ON seller.measures (account_id) WHERE status='ENFORCED' AND kind='SUSPENSION';
CREATE INDEX measures_review ON seller.measures (review_by) WHERE status='ENFORCED' AND basis='RISK';

CREATE TABLE seller.measure_findings (measure_id UUID REFERENCES seller.measures, violation_id UUID REFERENCES seller.violations,
                                      PRIMARY KEY (measure_id, violation_id));
CREATE INDEX measure_findings_violation ON seller.measure_findings (violation_id);
CREATE TABLE seller.measure_revisions (…同 violation_revisions，violation_id→measure_id，sanction_ends_at→ends_at；ends_at 是唯一的期間來源…);
CREATE TABLE seller.measure_notes (…同 violation_notes…);
CREATE TABLE seller.measure_requests (
  id UUID PRIMARY KEY, measure_id UUID NOT NULL REFERENCES seller.measures, account_id BIGINT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('APPEAL')),
  state TEXT NOT NULL CHECK (state IN ('PENDING','ACCEPTED','REJECTED','CLOSED')),
  reason TEXT NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 2000),
  revision INTEGER NOT NULL, submitted_at TIMESTAMPTZ NOT NULL,
  decision_reason TEXT, decided_by_admin_id BIGINT, decided_at TIMESTAMPTZ,
  FOREIGN KEY (measure_id, revision) REFERENCES seller.measure_revisions (measure_id, revision),
  CONSTRAINT measure_requests_decision_check CHECK (
    (state='PENDING' AND decision_reason IS NULL AND decided_by_admin_id IS NULL AND decided_at IS NULL) OR
    (state IN ('ACCEPTED','REJECTED') AND char_length(decision_reason) BETWEEN 1 AND 2000 AND decided_by_admin_id > 0 AND decided_at >= submitted_at) OR
    (state='CLOSED' AND decision_reason IS NULL AND decided_by_admin_id > 0 AND decided_at >= submitted_at))   -- 18.4：裁定必附理由
);
CREATE UNIQUE INDEX measure_requests_one_appeal  ON seller.measure_requests (measure_id) WHERE kind='APPEAL';
CREATE UNIQUE INDEX measure_requests_one_pending ON seller.measure_requests (measure_id) WHERE state='PENDING';
CREATE TABLE seller.measure_request_evidence (request_id UUID REFERENCES seller.measure_requests, position, asset_id, object_key, PRIMARY KEY (request_id, position), …);
CREATE TABLE seller.measure_identities (measure_id UUID NOT NULL REFERENCES seller.measures, kind, key_id, hash, PRIMARY KEY (measure_id, kind, hash), …同 suspension_identities 的 CHECK…);
-- evidence_accesses：violation_id → request_id，FK (request_id, position) → measure_request_evidence；owner_check 照舊
-- violations：DROP penalty, version, current_revision, appeal_*；ADD account_id BIGINT NOT NULL、origin_measure_id UUID NOT NULL，
--   FK (origin_measure_id, id) → measure_findings DEFERRABLE INITIALLY DEFERRED；violations_escalation 改 (account_id, created_at)
-- DROP violation_revisions、violation_notes、appeal_evidence、suspension_identities、suspensions
-- suspension_periods 不變
```
查詢：`EffectiveMeasures`（唯一定義，取代 `EffectiveSuspensions`）；對帳的 `AccountsWithAdminSanction`／`AccountsWithEffectiveViolationSanction` 合併為 `AccountsWithEnforcedSuspension`（走 `measures_enforced_suspension`，由 `EffectiveMeasures` 判斷）；到期 job `seller.measure_expiry` 的 args 為 (measure_id, ends_at)、ByArgs 唯一，執行時只呼叫 `Recompute`（以推導為準）。

### 3.8 Migration
`atlas migrate diff seller_measures`，DDL 之間手寫回填（§1 G19），更新 `atlas.sum`。ADMIN 列的 `risk_ground` 依 W2。測試 `TestMeasuresMigrationBackfills…`（比照 `migration_integration_test.go:319`）：塞入 WARNING、SUSPENSION、三種申訴狀態、已撤銷、有佐證與讀取紀錄的違規，ADMIN 列（生效中、已解除、有身分快照），套用後逐欄斷言；另一個測試塞入 `suspensions.ends_at` 與版本不一致的列，斷言 RAISE。

---

## 4. 2c-1 各消費端的改動清單

**Backend（模組）**
- `model`：新增 `measure.go`（Measure、MeasureRequest、Revision、Note、Input、`State`、`can_*`、事件）；`governance.go` 的 Violation 只留認定（加 AccountID、OriginMeasureID）。刪除 `Suspension` 列舉、`SanctionSource`、`SuspensionReason`、`SuspensionRecord`、`SuspendInput`/`LiftInput`/`ListInput`（`model.go:29-81,220-284`）、`exposure.go:125 AdminSuspension()`；店家狀態改成 `Suspended bool`＋`EffectiveMeasures []EffectiveMeasure{ID, Basis, RiskGround, LeadCategory}`。
- `internal/measure/`（新增）：Create（request_id 流程、升級、引用、身分快照、到期 job、Recompute）、Update、End、Revoke（含連帶撤銷）、ScheduleReview、SubmitAppeal、DecideAppeal、Notes、讀取。`governance/violation.go` 縮成只讀認定。`suspension/` 刪 `Suspend`/`Lift`/`List`/`validateSuspend`/`suspensionConflict`/`Record`，`Recompute`、`RecordIdentities`、到期與對帳改讀 `EffectiveMeasures`。`stint.go:90-130` 改依措施推導。
- `mediaref.go:130-135`：ref_id 是通報 ID 或請求 ID；`reports.sql:162,166` 改查 `measure_request_evidence`；`kyc.sql:250-330` 改 `measure_identities` JOIN `measures`。
- `seller.go`、`governance.go`、`events.go` 重新匯出；`S/AGENTS.md`（:84 身分快照段、:3-31 停權段）、`S/CARRYOVER.md`（:16-19、:69-76、:98、:135 改成措施，並加沿用盤點：效果表刪除、升級改帳號）。
- devseed：`clearChildrenSQL`（devseed.go:80-87）、`lifecycleStatesSQL`（:189-198）改新表，提示（:249）改 `EndMeasure`／`RevokeMeasure`。

**Backend（傳輸層與 legacy）**
- `H/seller/governance.go` 改措施 handler；`H/seller/violations.go` 改 `ListMyMeasures`/`GetMyMeasure`/`SubmitAppeal`；`H/seller/admin.go` 刪停權三支與 `suspensionSource`、`toSuspension`，ListStores 改 `suspended`、GetStore 改 `active_measures`；`H/seller/app.go:158-185` 改 `suspended`；`H/seller/evidence.go:20`＋`H/server.go:169-171` 改路由 `/api/files/admin/seller/measure_requests/{request_id}/evidence/{position}` 與權限；`declaration.go`；`adminauth.go:70-72,95-102`；`adminperm.go`；`H/rpc/fields.go`。
- legacy：`sellerreason/reason.go:12-33` 改 `Of(effective []EffectiveMeasure)`；`sellerserver/server.go:105-120`、`gatewayproxy/proxy.go:99-112` 改用 legacy 的 `classify`；`sellerrelay/relay.go:322-336` 改呼叫新 `Of`。
- 生成：`make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `../../api/generate.sh buf.gen.yaml`。
- gateway：不改（admin violations 5 條打舊服務，第 3 刀）。`maxRESTOperations` 不變：**未驗證**，實作時以 openapi coverage 測試確認。

**admin**（`admin/src/app/admin/merchants/[id]/`）
- `SuspendStoreDialog.tsx` → `CreateRiskMeasureDialog`：風險依據、給賣家的理由（必填 1–2000）、結束時間（選填）、複審期限（必填，預設 now＋7 天，上限 30 天）、內部註記（必填）；開啟時 `crypto.randomUUID()` 當 request_id，同一對話框的重送沿用。
- `LiftSuspensionDialog.tsx` → `EndMeasureDialog`（帶 etag，依 `can_end`）。
- `SuspensionHistoryCard.tsx` → `MeasureHistoryCard`（`ListMeasures(account_id)`，顯示 `state`）；`SuspensionStatusCard.tsx` → `ActiveMeasuresCard`（`GetStore.active_measures`）。
- `store-detail.ts`、`page.tsx`、`use-store-detail.ts`、`merchants/store-row.ts:5,42,106-109`；測試 `merchant-store-detail`、`merchant-suspension-status-card`（改名）、`merchant-store-row`、`admin-governance-modal-a11y.test.ts:29-31`。

**website**
- `lib/seller/violations.ts` → `measures.ts`；`useSellerViolations` → `useSellerMeasures`（`state` 篩選）；`lib/query/keys.ts:334-338`。
- `features/seller-center/violations/*` → `measures/*`：頁名「處置與申訴」，依據顯示認定類別或風險依據，狀態讀 `state`，無期限顯示「未定結束時間，平台定期複審」，`FINDING_REVOKED` 顯示「依據的認定已撤銷」；路由 `/seller-center/measures`（不做轉址）。
- `SellerSuspendedNotice.tsx:56-152`：讀 `ListMyMeasures(state=ACTIVE)`，每筆一張卡連到該措施，依 `can_appeal` 顯示申訴；刪 :105 文案；單元測試 :132-145 反轉。
- `lib/seller/store.ts:110-177`、`useSellerIdentity.ts`、`pageCapabilities.unit.test.ts`、`tests/seller-e2e-fixtures.test.ts`、`tests/e2e/helpers/mock-gateway.ts:193-256`。

**seller-app**
- `features/seller/data/violations.dart` → `measures.dart`（重新產生 `.g.dart`）；`violation_display.dart` → `measure_display.dart`；`seller_appeal_screen.dart`／`seller_appeal_form_screen.dart` 改指向措施（`VERSION_CONFLICT` 重讀沿用）。
- `suspension_notice_screen.dart:58-106,184-212`：列 ACTIVE 措施、刪導到 `SettingsRoutes.help` 與「只能由管理員解除」。
- `core/session/my_store.dart:155,244-262`：`isSuspended` 讀 `suspended`。
- `app_router.dart:155-162,1046-1053`（`/seller/measures/:measureId`）、`route_capabilities.dart`、`redirect_logic.dart`；入口文字「處置與申訴」（`seller_info_screen.dart:53,187-194`、`profile_other_options_card.dart:92`、`onboarding_progress_card.dart:73`）；inv §1.6 的全部測試。

**文件**：gov ADR 新增修訂「措施與認定分開、效果由措施推導」（取代 :69 處分表、:109-111 ADMIN 唯一索引、2026-10-11 修訂的 `suspensions.ends_at` 段、註記掛在違規）；設計書 32 各段；plan 10e 與 4B、4C 段。

---

## 5. 2c-2、2c-3 的契約增量
- 2c-2：賣家 `RequestRemediation(measure_id, reason, evidence_staged_tokens, etag)`、`Measure.requests`（新到舊、帶 `kind`，取代 `appeal` 與 `remediation_requests`，見 step10e-2c2-brief.md 末節「實作後的設計審查」）、`can_request_remediation` 與 `remediation_unavailable_reasons`（RISK、ACTIVE、沒有待處理請求、不在冷卻）；後台 `DecideRemediation(measure_id, decision, reason REQUIRED, etag)`、`can_decide_remediation`、`ListMeasures.request_state`＋`request_kind`（取代 `appeal_state`、`remediation_state`）。接受補正＝寫一版 `ends_at=now`、發 `MeasureEnded`（不是撤銷，認定不受影響）。條款 17.4 加一句並改版本行。
- 2026-10-15 補註（Will 全照建議）：2c-2 以 `step10e-2c2-brief.md` 為準，取代本節 2c-2 一行與 G21 的「VIOLATION 依據不提供補正」範圍說明中不一致的部分：補正開放給 RISK（含 17.5 外部要求）；`DecideRemediation` 不帶 `request_id`；加冷卻與 `next_remediation_time`；`measure_requests` 加 `basis`（不再只是增值的 migration）。
- 2c-3：§1 G15；`TerminationGround`、`Measure.effective_time`、`SCHEDULED`、job `seller.termination_effect`、待關閉與恢復；刪 `CloseStore` 與 `seller.stores.close`；條款 21 改版（寫明可依 18.4 申覆、生效後申覆成立即恢復，以及 W1 的結論）。
- 2026-10-15 補註（Will 全照建議）：2c-3 以 `step10e-2c3-brief.md`（定案版）為準，取代本設計書下列 2c-3 的說法：G4 的「2c-3 加 `TerminationScheduled`」（不加，`MeasureCreated` 加 `StartsAt`）；G15 的「預告 14 天」（改 30 日）、「進入待關閉」（不儲存，推導）與「店已關閉時重新開啟同一個店……KYC 個資已清理則回到需補 KYC」（改為終止生效後 6 個月的申覆期間內恢復同一個店，期滿只撤銷紀錄與禁止入駐；關店可逆、刪除移到清理）；§1 G1 的「2c-3 加 `kind='TERMINATION' AND m.effective_at <= @now`」、G2 表的「now < effective_at」與 G15 的「不可變的 `effective_at`」（改為所有措施的 `starts_at`）；G15 的「申訴不暫停排程」不變，但終止的申覆有期限；§5 本節 2c-3 一行的 `Measure.effective_time`（改用 `MeasurePeriod.start_time`）與「待關閉」；§6 的 2c-3 三條驗收（由簡報的驗收測試取代）；§7「生效後接受終止申訴不重開」的改判與 §8 第一項（已由簡報 T1 解決）。另新增本設計書沒有的：`MeasureBasis.CONTRACT`、`LINKED_ACCOUNT` 與 `source_measure_id` 的連帶撤銷、`appeal_unavailable_reasons`、依關閉原因的保留期與 legal hold、`StoreRetired`。

---

## 6. 驗收測試（含 mutation 檢查）

**2c-1**
- 唯一來源：
  - `EffectiveMeasures` 與 `model.State` 在邊界時間一致（`ends_at == now`、±1µs、`created_at == now`）。**mutation**：把 SQL 的 `>` 改 `>=`，測試要失敗。
  - 撤銷、End、縮短後 `stint.Derive`、`GetMyStore.suspended`、capability `NOT_SUSPENDED`、`ListStores(suspended)` 同時改變。**mutation**：讓 `suspended` 改由別的查詢算，測試要失敗。
  - schema 裡不存在 `suspensions`（`TestSchemaFileMatchesMigrations` 已涵蓋 drift）。
- 同一帳號建立兩筆 RISK 都成功（舊程式下會失敗）。
- DB 層：接受或駁回沒附理由被 CHECK 擋；沒有 `measure_findings` 的 VIOLATION 措施在 commit 時失敗；RISK 沒有 review_by 被擋。
- 連帶撤銷：
  - F 由 M1 建立、被 M2 引用（M2 另引用 G）；接受 M1 申訴且 `revoke_findings=true` → M2 為 REVOKED、`FINDING_REVOKED`，效果解除，`StoreUnsuspended` 只發一次（M1、M2 都是停權時），G 仍 ENFORCED。
  - M2 有待處理申訴 → 該申訴變 `CLOSED`，發 `MeasureRequestClosed`。
  - `revoke_findings=false` → M2 不變。**mutation**：拿掉連帶撤銷，第一個測試要失敗。
- request_id：
  - 同 id、同內容 → 同一個 measure_id、一列、事件一次。
  - 先建立、再關店（或把 review_by 推到過去、或撤銷引用的認定）、再同 id 重送 → 回傳原措施（不跑有狀態驗證）。**mutation**：把查詢移到有狀態驗證之後，測試要失敗。
  - 同 id、不同內容 → invalid_argument，欄位 `request_id`。**mutation**：拿掉 hash 比對，要失敗。
  - 另一位管理員用同 id → 各自建立；兩個 goroutine 並行 → 一列。
  - hash 正規化：同一時間以不同奈秒精度送兩次，視為相同。
- `can_*` 與前置檢查一致：表格驅動，每個狀態（ISSUED、ACTIVE、ENDED、REVOKED × RISK／VIOLATION × 有無申訴、待處理）對每個 `can_X` 斷言「`can_X == 方法成功`」。**mutation**：反轉任一 `Can*` 條件，要失敗。必含的格：ENDED 時延長失敗（G7）；VIOLATION 的 End 失敗（G8）；ENDED 可申訴。
- 期間：`end_time ≤ now` 回 invalid_argument；RISK 清除 end_time 在凍結後失敗；VIOLATION 清除失敗。
- 凍結：申訴後延長與改理由回 failed_precondition；縮短、End、ScheduleMeasureReview 成功。
- etag：只寫註記時賣家與後台 etag 都不變；ScheduleMeasureReview 只改後台 etag；admin 改理由後，賣家帶舊 etag 申訴回 aborted。**mutation**：讓註記寫入使 version 加一，要失敗。
- 升級以帳號計：帳號在店 A 有 2 筆 MEDIUM，關店、開店 B，第 3 筆 MEDIUM 升為 HIGH。**mutation**：查詢改回 store_id，要失敗。
- 撤銷：`revoke_findings=true` 只撤銷自己建立的認定；被撤銷的不計入升級；接受申訴 ⇔ `APPEAL_ACCEPTED`；待處理申訴時 Revoke 回 failed_precondition。
- review_by：≤ now、超過 30 天回 invalid_argument；`review_due` 只列 RISK、ACTIVE、到期的；migration 轉換的 ADMIN 措施立即出現在 `review_due`。
- 效果：RISK 開放式措施 `EndMeasure` 後 `StoreUnsuspended` 一次、期間結束、`measure_identities` 清掉。
- legacy：RISK → 9603／`suspended`；只有 VIOLATION → `SUSPENDED_GOVERNANCE`；並存 → 9603；sellerrelay 的 reasonCode、reasonLabel 正確。
- migration 測試（§3.8，含 RAISE 的反例與「（裁定時未記錄理由）」的斷言）；`TestSchemaFileMatchesMigrations`。
- 契約：`buf lint`、`buf breaking`（v1alpha1，預期的破壞）、`make connect` 無 drift、`TestEveryServiceInTheContractIsMounted`、REQUIRED 欄位測試、`CheckFieldPaths`、adminauth 與 adminperm 啟動檢查。
- admin：對話框重送 request_id 不變（**mutation**：每次重新產生，要失敗）；review_by 預設與上限；依 `can_end` 顯示；store-row 依 `suspended`。
- website：公告每筆 ACTIVE 措施一張卡、連到 `/seller-center/measures/{id}`，依 `can_appeal` 顯示申訴（反轉 :132-145）；申訴的 VERSION_CONFLICT；e2e mock 換成新 wire 形狀後通過（website-e2e-verify 的 mutation 規則）。
- seller-app：公告列表、申訴表單、`isSuspended` 讀 `suspended`；router 測試。
- compose 環境：RISK 措施建立與結束、連帶撤銷各抓一次 `seller.suspended`／`unsuspended` 的 wire payload（backend-e2e-verify）。

**2c-2**
- 同一措施同時一筆待處理請求（DB 唯一索引）；**mutation**：拿掉 `measure_requests_one_pending`，並行測試要失敗。
- 補正可重複（駁回後再送）；VIOLATION 依據 `can_request_remediation=false`、送出回 failed_precondition。
- 接受補正：寫新版、`MeasureEnded`、認定不受影響；待處理補正時 `can_end=false`。
- 撤銷時待處理補正變 `CLOSED`。
- 條款：`legal-generated-diff` 通過；`AGREEMENT_OUTDATED` 只在 ApplyStore 帶舊版本時出現。

**2c-3**
- 21.2 終止 14 天內 `state=SCHEDULED`、賣家能力不受影響；到期變 ACTIVE、停權開始、有未結訂單時等完成才關店；21.3 立即。
- 生效前接受申訴：REVOKED、不關店。生效後接受申訴：取消待關閉；已關閉時重新開啟同一個店（store_id、店號不變）。
- job 以 measure_id 唯一，重複執行無副作用；`CloseStore` 不在契約裡。

---

## 7. 對 critique 的處理

| critique | 處理 |
|---|---|
| HIGH G12 刪 ADMIN 列違反 brief §8、引用不存在的規則 | 採納。撤回刪除與那條規則；照 brief §8 轉換，已定義 statement、版本、review_by（立即到期）；`risk_ground` 沒有誠實值 → W2（§9） |
| HIGH 措施 status 與效果列兩份事實 | 採納修法 (a)：刪 `suspensions`，效果由措施推導，唯一 SQL 定義 `EffectiveMeasures`（G1） |
| HIGH 撤銷認定未處理引用它的措施 | 採納，以基準定案（Meta、YouTube、P2B 4(3)）：連帶撤銷、`FINDING_REVOKED`、待處理請求 `CLOSED`（G3），不列 W |
| MED 註記改後台 etag 與 ADR 牴觸 | 採納：註記兩種 etag 都不變；ScheduleMeasureReview 只改後台（G10） |
| MED 狀態要 client 組 | 採納：`MeasureState` OUTPUT_ONLY 推導（G2），契約不出現儲存的 status |
| MED 延長已到期的措施＝重新停權 | 採納：`CanExtend`、`CanShorten` 要求 ACTIVE；新 end_time 必須 > now（G7） |
| MED EndMeasure 不限依據 | 採納 brief D4：只限 RISK（G8） |
| MED request_id 查詢順序 | 採納：先查再做有狀態驗證，並加「建立→關店→重送」測試；回應標明是 AIP-155 的例外分支（G9） |
| MED 升級以 store 計 | 採納：改以帳號計，寫進沿用盤點（G12） |
| MED 生效後接受終止申訴不重開 | 改判：P2B 4(3) 要求恢復，所以定案為重開同一個店（G15），不列 W；重新入駐仍是 W1 |
| LOW 待處理補正時不能撤銷 | 採納：只有待處理申訴擋撤銷，補正以 CLOSED 結案（G14） |
| LOW `suspended` 與 capabilities 重複 | 保留 bool，與 `NOT_SUSPENDED` 同一個 `stint.Derive` 結果，測試鎖住（G16）；理由：client 需要帳號層級的狀態，從能力清單反推是 client 推導 |
| LOW 無期限 ↔ 有期限的規則 | 採納：§1 G7、§3.5、表格測試 |
| LOW 回填佔位文字 | 保留，migration 測試斷言、CARRYOVER 註明（§1 G19） |
| LOW `SanctionID?` | 刪除；事件改列於 G4 |
| 切分：一個 commit、分層審查 | 採納（§2） |

---

## 8. 沒有驗證的部分
- 等待刪除「等未結訂單完成」的機制能否給 2c-3 待關閉共用；關閉的店能否重新開啟（`stores_closure_check`、KYC 清理後的狀態）。
- 2c-1 之後 gateway `maxRESTOperations` 不變。
- Stripe key 以帳號為範圍（沒有作為依據）。
- 本機 DB 的實際筆數與 ADMIN 列的原因分布（依指示沒有讀 DB）。
- 合併後各 client 檔案的行號沿用 inv 的快照，實作時重新定位。

---

## 9. 要 Will 決定的事

**W1（只影響 2c-3）：終止措施生效後，帳號能不能重新開店？**
- 現況：關店後帳號可重新申請開新店（`onboarding.go:66-69`，停權中不行）；身分快照只在停權生效中擋其他帳號（`measure_identities`）。條款 21 條沒寫。
- 基準：YouTube 終止後禁止使用或建立任何其他頻道（已查）；Amazon、Google MC 未查證。P2B、DSA 不規定。
- 選項：(a) 終止＝帳號層級，效果無期限，不能重新入駐（解除只靠撤銷）；(b) 只結束這次合作，可重新申請並重新 KYC；(c) 依事由：21.3（詐欺、食安等）禁止重新入駐，21.2 無過失終止可以。
- 取捨：(a) 防止換店重來，但 21.2 的無過失終止也被永久排除；(b) 讓 21.3 的問題賣家容易回來；(c) 需在條款 21 條寫明。
- 建議：(c)。

**W2（只影響 2c-1 的 migration）：既有 ADMIN 停權列的 `risk_ground` 怎麼填？**
- 事實：brief §8 要求轉成 RISK 措施，但舊的三個原因（POLICY_VIOLATION、REPEATED_WARNING_IGNORED、OTHER）都對不到條款 17.1 的七種風險或 17.5；前兩種在新模型屬於 VIOLATION 依據，卻沒有認定可以引用（造一筆認定要捏造類別與嚴重度，還會影響升級計數）。其餘欄位的轉換已定義（§1 G19）。只有本機與測試資料，CI 的 DB 是新建的。
- 選項：
  - (a) migration 在有 ADMIN 列時 `RAISE EXCEPTION` 列出 ID，開發者自行清掉本機資料後重跑。先例：gov ADR 2026-10-11 修訂「對不上就 RAISE，不寫出錯誤的歷史」。與 brief §8 的「轉換」不同，所以要 Will 同意。
  - (b) 新增永久的 `RISK_GROUND_OTHER`：轉換完整，但新措施也能選一個條款 17.1 沒有的依據（DSA 17(3)(e) 要求指出條款依據），等於為了舊資料降低以後每一筆的品質。
  - (c) 只允許舊資料的「未分類」值（CHECK 綁 `created_at` 早於切換時間），API 輸出 UNSPECIFIED：client 要永久處理一個只有舊資料才有的值，屬於技術債。
- 建議：(a)。理由：系統沒上線，這些列只是開發時手動測 2a 留下的；(b)、(c) 都為了不會再出現的資料永久改變契約或規則。

**Will 的決定（2026-10-12）：W1、W2 都照建議。** W1 選 (c)：21.3 的事由禁止重新入駐，21.2 的終止可重新申請並重新 KYC，條款第 21 條一起修訂（2c-3）。W2 選 (a)：migration 遇到既有 ADMIN 處分列時失敗並列出 ID，清掉本機資料後再跑，不新增條款沒有的依據值（2c-1）。
