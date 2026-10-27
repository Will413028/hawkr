# 10e 第 5b 刀：admin 商家頁的動作可用性與原因、PayUni 卡改由伺服器推導，實作設計 v1.3

> **狀態**：v1.3。v1 經一輪獨立審查（1 HIGH、6 MED、8 LOW，全部處理，對照見附錄 A；全文在 session scratchpad `5b-design-review.md`，不入庫）。v1.1 另依 controller 的裁定把 admin 的 PayUni 卡（徽章、問題文字、綁定失敗句）全部收進本刀（F11、F12）。**Will 2026-10-21 全照建議**：F3 暫停審核、F4 與 F6 顯示、F12 刪欄位，§5 的 N 類文案 15 列核可（見文末決定紀錄）。v1.2 處理開工前 design-review（8 項全改，附錄 B），其中兩項 **Will 2026-10-21 決定**：帳號停權時比照 F3 暫停審核（F3）；店家等待刪除時不能重新綁定 PayUni（F5，新句 N17 核可）；N12 刪除。v1.3 在 5a 合併之後（`2bba5d0b4`、`1fc535d87`）對照 HEAD 重新核對全部引用並改寫前提變了的段落（附錄 C）：`PayUniPhaseOf` 已由 5a 抽出，本刀沿用；5a 的 `Rebind` 申請檢查（F17）折成 `REBIND_PAYUNI` 的最後一個原因 `KYC_NOT_APPROVED`（沿用既有的值，不新增 `KYC_RESUBMITTING`）；`PayUniOutcome` 由本刀刪 proto enum；5a 的待辦 `PAYUNI_BIND_RETRY` 已存在，§9 改成單向的成員斷言。已核准的決定與文案（N1–N8、N10–N17）不變；**Will 2026-10-21 核可** N18：5a 的 #38 句子照原句顯示在 admin 的 PayUni 卡上（文末「待 Will 決定」）。

v1.1、v1.2 讀取的是 `local/10e` 的 `a6f7b3815`；**v1.3 對照 `1fc535d87`**：`git diff a6f7b3815 HEAD --stat` 131 個檔，本文引用的檔案裡有變的只有 seller 的 `model/payuni.go`、`model/payuni_state.go`（新）、`payuni/intent.go`、`kyc/kyc.go`、`model/kyc.go`、`db/objects.sql`、`internal/text/zh-TW.json`、`payuni_integration_test.go`、`review_policy_integration_test.go`、seller `AGENTS.md`／`CARRYOVER.md`、賣家端 `api/hawkr/seller/v1alpha1/seller.proto` 與計畫 10e；admin 契約（`P/`）、`H/admin.go`、`H/admin_kyc.go`、`admin/src`（除了 `queue/queue-item.ts`）、delegation、measure、kyc 的 admin 路徑、foodreg、closing、`stint.go` 都沒有變，那些行號照舊成立［量測：`git diff --name-only a6f7b3815 HEAD` 與逐檔 `git diff -U0`，有變的檔逐一重讀］。
盤點：2026-10-21 的唯讀盤點 `inv-store-actions.md`（session scratchpad，不入庫，下稱「盤點」）；本文用到的每一條都已對照 `a6f7b3815` 重新查過，結論收進 §0.4。
SSOT：計畫 10e 段（`2026-09-29-a3-buyer-seller.md:346-389`，其中 :381 顯示文字的歸屬、:383 L′ 範圍外清單、:384 本刀的待辦；5a 的完成紀錄在 :379 插入一行，v1.2 的 :380、:382、:383 都往後一行［量測］）；前提 2026-10-15「可用性與原因由伺服器給，client 只呈現」（`step10e-2c3-brief.md:247`，下稱「前提」）；`step10e-labels-brief.md:7`（只由 enum 值決定的名詞進共用目錄；伺服器對資源此刻狀態的判斷與說明由伺服器在原因 enum 旁回，與 `failed_precondition` 用同一個 Go 函式，下稱「labels」）；`step10e-Lprime-design.md`（§1.3 staff 只有 zh-TW、§3.2 model 的 `*Unavailability`、§5.2 守門與「單端的表允許」、:452「依優先序、第一個是某些原因表示沒有入口」的分支規則，下稱「L′」）；`step10e-decision-brief.md:157`（D4 第 13 點，下稱「db D4-13」）；姊妹刀 `step10e-5a-design.md`（下稱「5a」，F11、F12、F17、§7.1，已完成；與 5a v1.2 不同的地方以它的「實作時的修訂」與附錄 C 為準）；錯誤模型 ADR `2026-09-25-api-error-model`；`API 設計規範（Connect）.md`；術語表；root、admin、backend、seller、delegation、accountdeletion 的 `AGENTS.md`；設計書 56（代行營運）。
路徑縮寫：`W` worktree 根、`A`＝`W/admin/src`、`MD`＝`A/app/admin/merchants`、`M`＝`W/backend/internal/modules/seller`、`DL`＝`W/backend/internal/modules/delegation`、`H`＝`W/backend/internal/httpapi/seller`、`P`＝`W/api/hawkr/admin/seller/v1alpha1`。
標記：**[量測]** 讀過程式或跑過命令；**[推論]** 由程式讀出、沒有跑測試；**[外部]** 業界資料，本次沒有網路，全部未查證。判定原則同 4a、4b：上線前沒有相容性約束（root `AGENTS.md`「上線前不受相容性約束」）；照業界從零的做法；只有基準定不了的產品選擇、UI 改動、改契約或改寫已核准決定的列給 Will。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| `MD/[id]/store-detail.ts` 的 `storeActions`、`REVIEW_BLOCKED_*`、`reviewBlockedReason`；`MD/store-row.ts` 的 `lifecycleBlockedLabel` | 沒有；本刀刪 | 刪除，不在裡面最佳化 |
| `MD/[id]/kyc-application.ts` 的 `reviewActions`、`canRebind`、`PAYUNI_REONBOARDING_TEXT`、`payUniBadge` 的後備表與 `PAYUNI_FLOWS`、`payUniProblemText` | 沒有 | 改成只讀伺服器欄位；後備表、`PAYUNI_REONBOARDING_TEXT`、`payUniProblemText` 刪除（F5、F11） |
| `A/lib/food-registration.ts` 的 `approveBlockedText`、`APPROVE_BLOCKED_KYC_TEXT` | 沒有；本刀刪 | 刪除（F13） |
| `A/features/delegated-operations/policy.ts` 的 `delegationBlockedLabel` | 沒有；本刀刪 | 刪除 |
| `MerchantOperationEntry` 的 preflight 路徑（BFF `.../delegated-operation/preflight/route.ts` → gateway REST → `internal/legacy/delegationservice`） | **A10**（`backend/docs/plans/2026-09-24-go-modular-monolith.md:227`「從 gateway 搬出代行編排」；`DL/AGENTS.md:3`） | 傳輸層會被重寫，本刀不碰傳輸、不加新的 REST 欄位；只改 preflight 回傳的那一句的來源（F7） |
| delegation 模組的 preflight 判斷（`DL/internal/session/session.go:80-135`、`DL/delegation.go:57-73` `storeStateOf`） | A10 只搬編排，判斷留在模組 | 會留下；F7 改 `storeStateOf` 的店家狀態判斷改用 seller 的同一個函式 |
| `DL/internal/model/delegation_rules.go:229-252` `DeniedLabel` 的授權、資格、進行中句子 | 不在本刀：這些拒絕今天已經由伺服器推導，句子經 preflight 原樣送到畫面（BFF `preflight/route.ts:59-71`「不在前端建對照表」、`MerchantOperationEntry.tsx:176-180`），client 沒有自己推導，不違反前提；剩下的只是句子還沒搬進 delegation 的 `internal/text`，觸發是 A10 定 delegation 的 Connect 契約 | 不動 |
| seller 的 `GetStore`、`ListStores`、`GetKYCApplication`、`GetPayUniRegistration`、`GetFoodRegistration` 與 measure、kyc、payuni、foodreg 的前置檢查 | 沒有 | 會留下，照從零的做法做 |
| `M/internal/stint/stint.go:57-67` `LockOpenAccountOf` | 沒有呼叫端（§0.4 C22） | 本刀刪（附錄 A L2） |
| 5a 的 `Rebind` 申請檢查（`M/internal/payuni/intent.go:187-193`）、`model.ErrRebindApplicationNotApproved`（`M/internal/model/payuni_state.go:178-182`）、key `payuni.rebind_unavailable.application_not_approved`（`M/internal/text/zh-TW.json:443-446`） | 5a §7.1 排定由 5b 收成原因 | 本刀折進 `RebindUnavailableReasons`，函式與 key 刪除，句子沿用（F5、附錄 C #2） |
| `M/internal/model/payuni.go:505-526` `PayUniPhaseOf` | 沒有；5a 已抽出（5a「實作時的修訂」） | 沿用，不改（F11） |
| gateway 的 `pkg/delegation/store_state.go:28` `StoreOperable` | A10 刪（gateway 編排搬走） | 不動 |

結論：admin 的推導碼全部刪除；seller 這一側是終點；代行的傳輸與授權句子留給 A10（理由見上表），本刀只處理店家狀態那一半（計畫 :384「句子放 seller `text/`」也只涵蓋這一半，見 F7）。

---

## 0.1 範圍

| 部分 | 內容 |
|---|---|
| 契約 `P/seller.proto` | 新增 `StoreActionUnavailableReason` 與 `StoreActionUnavailability{reason, LocalizedMessage}`（F1）；`GetStoreResponse` 加建立措施的可用性（F2）；`StoreListItem` 加代行的店家狀態不可用原因（F7）；`KYCApplication` 加核准、退回、重新查詢的可用性（F3）；`PayUniRegistration` 加重新綁定的可用性（F5）與 `flow_status`（F11），F12 刪 `outcome`、`payment_enabled`、`needs_reonboarding`；`FoodRegistration` 加核准、退回的原因（F13）；enum 註解逐值標明哪些值表示沒有入口（F8）；`ListStoresRequest` 加 `kyc_awaiting_review`、`kyc_status` 不再接受 SUBMITTED，`GetStoreStatsResponse.kyc_submitted_stores` 改成 `kyc_awaiting_review_stores`（F3 的成員只有一份）；賣家端 `api/hawkr/seller/v1alpha1/seller.proto:746-757` 刪 `enum PayUniOutcome`（F12 之後沒有使用者，§9） |
| 契約 `P/measure.proto` | `CreateMeasure` 的店家已關閉改 `failed_precondition`（F2）的註解；`Measure.can_end` 註解寫明不看店家狀態（F4） |
| seller 模組 | `model/store_action.go`（新）：`StoreAction`、原因、有效組合表、`StoreActionUnavailableText`；建立措施、核准、退回、重新查詢、重新綁定、食品業者登錄裁定的前置檢查改走它；`End` 的句子移進 `text/`；建立終止的兩句改 `failed_precondition` 並移進 `text/`；案件的 `CaseBlockStoreClosed`、通報的 `ReportBlockStoreClosed` 改用 `AllowsMeasure()`，句子改用同一句（L6）；admin `flow_status` 沿用 5a 的 `model.PayUniPhaseOf`（F11）；5a 的 `Rebind` 申請檢查折成 `KYC_NOT_APPROVED`（F5）；`review_kyc_application`（店家營運中、帳號沒有停權）與兩支審核期限函式（F3）；`AdminStores` 的待審核篩選與 `AdminStoreStats` 的待審核家數改讀 `review_kyc_application`（F3）；刪 `LockOpenAccountOf` |
| delegation 模組 | `storeStateOf` 改用 seller 的 `StoreActionDelegate` 原因，已關閉是 `Found=true`＋原因；新拒絕 `DenialStoreClosed`；preflight 的店家狀態拒絕句改用 seller 的句子，刪 `DeniedLabel` 的 `DenialStoreNotOperable` 分支（F7）；legacy `delegationservice` 的 `bizCodes` 加 `DenialStoreClosed → 9265` |
| handler `H/` | `GetStore`、`ListStores`、`GetKYCApplication`（與核准、退回、重新查詢的回應）、`GetPayUniRegistration`（與重新綁定的回應）、食品業者登錄的讀取與裁定回應轉出新欄位 |
| admin | 商家清單的「待審核」分頁改送 `kycAwaitingReview`、兩張待審核統計卡改讀 `kycAwaitingReviewStores`（字不變，F3）；商家清單的代行入口、商家詳情的建立措施、結束措施、刪除帳號、KYC 審核區、退回對話框、PayUni 卡（徽章、問題文字、重新綁定）、食品業者登錄詳情的核准提示改讀伺服器欄位；`unavailableLabels` 搬到 `A/lib/unavailability.ts`（F8） |
| 刪除 | `storeActions`／`StoreActions`、`REVIEW_BLOCKED_*`、`reviewBlockedReason`、`lifecycleBlockedLabel`、`delegationBlockedLabel`、`canStartDelegatedOperation`、`PAYUNI_REONBOARDING_TEXT`、`canRebind`、`payUniBadge` 的後備表與 `PAYUNI_FLOWS`、`payUniProblemText`、`approveBlockedText`、`APPROVE_BLOCKED_KYC_TEXT`、`ActiveMeasureCards` 的 `canEndMeasure` prop、`LockOpenAccountOf`、`cases_dataset_integration_test.go` 的 `storeClosedNotFound` 分支、`model.ErrRebindApplicationNotApproved` 與 `text.PayuniRebindUnavailableApplicationNotApproved`、proto `PayUniOutcome`（Go 的 `model.PayUniOutcome` 留著：它是 DB CHECK 與 `PayUniPhaseOf` 的輸入） |
| 文件 | 計畫 10e 的 5b 完成紀錄；seller 與 delegation 的 `CARRYOVER.md`／`AGENTS.md` 對應句；設計書 56 的四處文字（:2398、:2459、:5813、:5858）與文末修訂；`M/internal/text/zh-TW.json` 的 `case.unavailable.store_closed`（`:339-342`）、`report.unavailable.store_closed`（`:519-522`）兩個 key 刪除（L6），`payuni.rebind_unavailable.application_not_approved`（`:443-446`）改名成 `store_action.unavailable.kyc_not_approved.rebind_payuni`、字不變（F5）；db D4-13 的修訂（F4、F6 照 Will 的決定） |

## 0.2 不在範圍

- 代行的授權、管理員資格、進行中的工作階段的不可用原因（`DeniedLabel` 其餘句子）與 delegation 的 Connect 契約：句子已由伺服器推導（§0 表），觸發是 A10。
- 清單上代行的授權狀態（批次 preflight）：清單維持 lazy（`A/features/delegated-operations/components/MerchantOperationEntry.tsx:66-78`），授權仍在點下去時由 preflight 判斷（F7）。觸發：營運反映清單上常點到「尚未開通」。
- 與 5a 的界線（依 controller 裁定）：**admin 看到的 PayUni 全部屬於 5b**（狀態徽章、問題文字、綁定失敗句、重新綁定的可用性與 `needs_reonboarding` 那一句，F5、F11、F12）；**5a 只處理賣家端**（已合併：賣家 `KYCFlow.payuni` 的 PayUni 狀態與說明、`resubmit_unavailabilities`，加上後台待辦 `PAYUNI_BIND_RETRY` 與 `Rebind` 的申請檢查；待辦屬 5a，本刀不重做，成員的對齊見 §9）。PayUni 終止狀態有兩句、受眾不同：賣家那一句屬 5a（`PayUniState` TERMINATED 的說明），員工那一句是本刀 §5 #N10；兩句都由註冊的同一個事實 `PayUniRegistration.NeedsReonboarding`（`M/internal/stint/flow.go:30`）決定，不互相引用（5a 已刪 `model.KYCFlow` 上的副本，5a「實作時的修訂」）［量測］。兩刀共同碰到的檔案與 rebase 之後要做的事見 §9「與 5a 的界線」。
- 終止合作的不可用原因：伺服器已經給 `can_create_termination`＋`available_terminations`（`P/seller.proto:280-287`），client 不寫原因句；本刀只改錯誤碼與句子的來源（F2 最後一點）。
- seller 端 `Authorize` 對已關閉店家回 `NotFound("店家已關閉")`（`M/internal/model/capability.go:181`、`M/internal/stint/stint.go:28-29` `stint.Lock`）：那是賣家「目前的店家」的語意（賣家的店家已關閉就等於沒有目前的店家），不是 admin 對歷史 stint 的操作；本刀只改 admin 路徑。
- `PayUniRegistration.bind_error_code` 的意義說明（員工看到錯誤碼要查表）：今天只顯示原碼給客服排查，沒有 client 推導；觸發是營運反映看不懂錯誤碼。

---

## 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| F1 | 原因的形狀 | 新增一個 enum `StoreActionUnavailableReason` 與 `StoreActionUnavailability{reason, google.rpc.LocalizedMessage message}`，跨「admin 對一個店家（帳號）與它的商家認證申請、PayUni 註冊、食品業者登錄做的動作」共用。Go 有 `StoreAction`（`CREATE_MEASURE`、`DELEGATE`、`APPROVE_KYC`、`REJECT_KYC`、`REQUERY_KYC`、`REBIND_PAYUNI`、`APPROVE_FOOD_REGISTRATION`、`REJECT_FOOD_REGISTRATION`）與一張**有效組合表** `storeActionRules map[StoreAction][]storeActionRule`：每個動作是一串依先後排列的 `(reason, applies(facts), kind)`，**推導就是依序跑這張表的 predicate**（附錄 B #4），所以每個動作「有哪些原因、什麼順序」只有這一份，四支推導函式只負責組出 facts、不另寫條件與順序。`kind` 只有兩種：`blocking`（方法回 failed_precondition，要有句子）與 `idempotent`（方法不回錯誤、回目前的狀態，沒有句子；目前只有 `REBIND_PAYUNI` 的 `PAYUNI_BIND_IN_PROGRESS`，附錄 B #6）。句子由 `StoreActionUnavailableText(reason, action)` 產生，是回應欄位與錯誤的 LocalizedMessage 唯一的來源；表外與 `idempotent` 的組合回零值。測試鎖住：`blocking` 組合都有非空句子、`idempotent` 與表外的組合回零值（附錄 A L5）。全部原因都列出（不是只列第一個） | 同 repo 先例 `MeasureActionUnavailability`（`P/measure.proto:177-203`：一個 enum 跨四個動作、依優先序、`ActionUnavailableText(reason, action)`，`M/internal/model/measure.go:692-729`）；labels:7 與計畫 :381「與 `failed_precondition` 由同一個 Go 函式產生」。**替代**：每個資源一個 enum（店家、KYC 申請、PayUni 註冊、食品業者登錄各一）——先例 `AuthorityOrderUnavailableReason`、`MeasureActionUnavailableReason` 都只用在一個資源。v1 不採用的理由「句子函式要四份」不成立（Go 的句子函式本來就能共用，附錄 A L5）。仍選一個 enum 的理由：28 個有效組合裡 17 個是店家或帳號的狀態（已關閉、等待刪除、停權）［量測：數 v1.3 §2.1 的表；v1 寫的「13 個裡 9 個」是 v1 的表，v1.1 之後沒有重數，附錄 C #9］，`KYC_NOT_APPROVED` 也跨食品業者登錄與 PayUni 兩個資源（F5），它們是同一個事實、同一句話的句型；四個 enum 會讓這三個值各出現四次，而 admin 的呈現規則（F8「哪些原因表示沒有入口」）要維護四份。代價是需要有效組合表；表本身就是規則（ordered rule list），不是推導之外的第二份宣告 |
| F2 | 建立措施的可用性與錯誤 | `GetStoreResponse` 加 `bool can_create_measure` 與 `repeated StoreActionUnavailability create_measure_unavailabilities`（只會有 `STORE_CLOSED`），規則是 `TerminationTarget{Closed}.AllowsMeasure()`（`M/internal/model/measure.go:417-419`，來函已經用它）。`CreateMeasure` 對已關閉的店家從 `NotFound("店家已關閉")` 改成 `FailedPrecondition`，句子是同一個 `StoreActionUnavailableText(STORE_CLOSED, CREATE_MEASURE)`（§5 #N1）。案件的 `CaseBlockStoreClosed`（`M/internal/model/report_case.go:570`）與通報的 `ReportBlockStoreClosed`（`:683-684`）改用 `AllowsMeasure()` 判斷，原因句改成回同一句 N1，刪掉案件與通報各自的句子（附錄 A L6：從案件或通報建立措施被擋下時，錯誤就是 `CreateMeasure` 的 N1，兩邊必須同一句）。建立終止的兩個 `NotFound`（`M/internal/measure/create.go:504,506`）一併改 `FailedPrecondition`，句子移進 `text/`（§5 #E3、#E4） | 店家存在（admin 讀得到它）而狀態不允許：AIP-193 與錯誤模型 ADR 的分類是 `failed_precondition`，`not_found` 只用在資源不存在［外部＋錯誤模型 ADR］；`P/seller.proto:280-283` 的 `can_create_termination` 已經是同形狀（伺服器推導、client 不從 `store.status` 推導）；ADR `2026-10-18-seller-governance-consolidated.md:142`（來函的 `can_create_measure` 與 `CreateMeasure` 同一份規則）。`can_*` 與清單並存照 `Measure`、`AuthorityTargetStore` 的先例（`P/authority_order.proto:166-169`「等於 … 是空的」） |
| F3 | 店家關閉、等待刪除或帳號停權時，審核中（SUBMITTED）的商家認證申請怎麼處理（**改伺服器行為**；併入審查 H1、M2；Will 2026-10-21 選 A，v1.2 擴到停權） | **決定：暫停審核**。店家不是營運中（已關閉或等待刪除）時，核准、退回、重新查詢都不可用（`STORE_CLOSED`／`STORE_PENDING_DELETION`），申請維持 SUBMITTED；待辦函式 `review_kyc_application` 加 `s.status = 'ACTIVE'`（同食品業者登錄 `objects.sql:240`），所以這段期間不在後台待辦、沒有審核期限（`decide_deadline` 用同一支函式，`db/queries/kyc.sql:363-366`）。店家恢復營業（撤銷終止合作，同一個 stint，`closing.go:357-376`）時申請自動回到待辦；期限從「送出」與「最近一次恢復營業」較晚的時間起算（`store_closures.reopened_at`），食品業者登錄的期限函式同一刀改成同一條（否則恢復營業的件一回來就逾期）。關店或刪除帳號沒有恢復的店家，申請就停在 SUBMITTED、不出現在任何待辦；個資清理照舊清個人欄位、保留 status（`purge.sql:95`）。**帳號停權（Will 2026-10-21，附錄 B #3）**：比照暫停：帳號有生效中的措施（`Store.Suspended()`，SQL 是 `seller.effective_measures(at)`，同一個定義）時申請不在待辦、沒有期限；停權結束（到期或撤銷）時自動回到待辦，期限從「送出」「最近一次恢復營業」「最近一次停權結束」三者最晚的時間起算。核准照舊擋（`ACCOUNT_SUSPENDED`，N7 的字照舊成立，不改），**退回與重新查詢照舊可用**（員工確定要退的件不必等停權結束）。**待審核的成員只有一份（附錄 B #1）**：後台待辦、`decide_deadline`、商家清單的「待審核」分頁（今天是 `AdminStores` 的 `k.status = kyc_status`，`stores.sql:200-205`，admin 傳 SUBMITTED，`A/lib/store-search.ts:60-61`）與兩張「待審核」統計卡（今天是 `AdminStoreStats.kyc_submitted`，只排除已關閉、含等待刪除，`stores.sql:226-238`；`A/app/admin/page.tsx:145-152`、`MD/page.tsx:225-233`）一律讀 `review_kyc_application`。契約上 `ListStoresRequest` 加 `bool kyc_awaiting_review`，`kyc_status` 改成只接受 APPROVED、REJECTED（SUBMITTED 回 invalid_argument：「待審核」只有一個意思）；`GetStoreStatsResponse.kyc_submitted_stores` 改名 `kyc_awaiting_review_stores`（舊編號 reserved）。畫面上的字（「待審核」「賣家待審核」）不變，只有家數與成員改變。成員對應的 Go 定義是 **`APPROVE_KYC` 的原因清單為空**（審核中、店家營運中、帳號沒有停權；身分命中不在原因裡、照舊留在待辦讓員工退回）——不是 `REJECT_KYC`：停權時退回仍可用，但件不在待辦 | **現況**（[量測]，§0.4 C8、C18）：關店不碰申請；賣家關店後撤不回（`Cancel` 走 `stint.Lock`，已關閉回 NotFound，`kyc/kyc.go:452`）；核准已擋關閉；所以今天唯一的出口是退回，而 v1 的 F3 把退回也擋掉，申請會永遠逾期留在待辦（審查 H1）。等待刪除只會往關閉走（`stores.sql:110-114` 只從 ACTIVE 轉入，沒有反向），恢復營業只發生在撤銷終止（`stores.sql:133-137`）。**業界基準**：審核佇列只放現在能裁定的件，主體停用時件被暫停或取消，SLA 計時在不能處理的期間不算［推論，沒有查證］。**同 repo 先例**：食品業者登錄的審核只在店家營運中（`objects.sql:240`、`model/foodreg.go:189-192`）；PayUni 綁定在關店期間暫停、恢復營業時由 `payuni.Resume` 補排（`intent.go:47-64`）。**停權的先例對照**：食品業者登錄的裁定不看停權（`FoodRegistrationReview.CanReject`／`CanApprove` 與 `Decide` 只看待審、店家營運中、KYC 已核准，`model/foodreg.go:189-200`、`foodreg/foodreg.go:365-373`；`review_food_registration` 也不看，`objects.sql:229-240`）［量測］，所以它的「待辦成員＝核准可以做」在停權時本來就成立、不用改；KYC 的核准被停權擋，所以照同一條規則要移出待辦。兩者對齊的是規則（成員＝核准可以做），不是逐條條件。**選項**：(A) 暫停（建議）：一條規則同時涵蓋兩種審核與 PayUni，恢復營業不需要賣家重做；代價是不會恢復的店家留著一筆 SUBMITTED（不在待辦、不在待審核分頁與統計：v1.1 說「統計已排除」只對已關閉成立，等待刪除仍被算進去，v1.2 改成三處同讀 `review_kyc_application`，附錄 B #1）。(B) 關店（與進入等待刪除）在同一個 transaction 把審核中的申請改成取消（系統作者，留審核紀錄）：狀態最乾淨；代價是新增系統取消的轉換與紀錄、恢復營業後賣家要重新送件（他們送件時沒有錯），而且食品業者登錄與 PayUni 是另一套語意。(C) 退回在已關閉時照舊可用（只改 client，伺服器不改）：員工被迫對沒有賣家可以回應的申請做「退回」，通知送給沒有賣家角色的帳號，待辦照樣逾期。(D) v1 原案（擋退回、不改待辦）：死路，不採用 |
| F4 | 結束措施在已關閉的店家（**要 Will 決定**：UI 出現與否） | client 改成只看 `measure.can_end`，刪除店家層級的 `canEndMeasure`（目前 client 在已關閉時藏起來）。伺服器規則不改。`End` 的硬寫句（`M/internal/measure/update.go:203`）移進 `text/`（§5 #E5）；不新增 `end_unavailabilities`：admin 的結束鍵不能做時是隱藏、不顯示原因（`A/app/admin/measures/measure-actions.ts:113` `visibleWhen(measure.canEnd)`），沒有消費端（必要條件不成立）。這會讓已關閉店家的詳情頁出現「結束」鍵（db D4-13 寫「停權、關店、核准等操作全部隱藏」，結束措施沒有明列） | **從零**（附錄 A M6）：業界的處分以帳號為範圍，關閉一個商店不解除帳號的處分，處分的結束是對處分本身的審查動作，與商店開關無關［外部，未查證］。我們的程式也是這個形狀——`closing.go:216`「停權不解除（停權以帳號為單位）」、生效中的停權會擋下同一帳號重新入駐（`OpeningBlocker` SUSPENDED，L′ §3.2）——但那是我們自己的設計（SELF-IMPOSED），論證不靠它：照業界基準，風險排除時員工要能從任何一個看得到這個措施的頁面結束它。`M/internal/model/measure.go:795-797` `CanEnd()`；`M/internal/measure/measure.go:349-362` 鎖帳號含已關閉的 stint［量測］ |
| F5 | 重新綁定 PayUni | `PayUniRegistration` 加 `can_rebind` 與 `rebind_unavailabilities`，先後是 `PAYUNI_MERCHANT_TERMINAL`（`needs_reonboarding`）、`STORE_CLOSED`、`STORE_PENDING_DELETION`、`PAYUNI_BIND_IN_PROGRESS`（PENDING，`idempotent`）、`PAYUNI_BOUND`（BOUND）、`KYC_NOT_APPROVED`（註冊所根據的申請不是已核准；v1.3 收進 5a F17 的檢查，附錄 C #2）。**申請不是已核准（5a F17）**：5a 在 `Rebind` 裡另寫了一個條件（`intent.go:187-193`：讀 `q.KYCApplicationByID(current.ApplicationID)`，不是 APPROVED 回 `model.ErrRebindApplicationNotApproved()`），因為賣家把綁定失敗的申請改回草稿時，綁定 job 會把還沒審核的草稿送給 PayUni；本刀把它收成表上的原因，所以 `RebindUnavailableReasons` 多一個輸入（註冊所根據的申請的狀態）。**值沿用 `KYC_NOT_APPROVED`，不新增 5a §7.1 舉例的 `KYC_RESUBMITTING`**：同一個事實——申請就地改回草稿（`UpdateKYCDraft` 改同一列，`db/queries/kyc.sql:43-48`），核准時 `UpsertPayUniIntent` 把註冊指向被核准的申請（`db/queries/payuni.sql:25-41`），所以「註冊的申請不是已核准」等於「店家目前沒有已核准的申請」［推論：讀 SQL，沒有跑測試；§6.1 的資料集測試鎖住］；句子依 (reason, action) 分開，沿用 5a 的 #38（§5 #N18）。**排在最後**（5a §7.1 寫「排在 `PAYUNI_BIND_IN_PROGRESS` 之前」，不採用）：PENDING 而申請不是已核准到不了（PENDING 只由核准或 `Rebind` 產生，兩者都要已核准；綁定中的 PayUni 狀態是 REVIEWING，賣家不能改回草稿）［推論］；BOUND 而申請是草稿到得了（PayUni 拒絕、賣家修正中，5a R3），這時重新綁定本來就不是可做的動作，第一個原因應該是結構性的 `PAYUNI_BOUND`、卡片不顯示句子；排在前面會讓卡片在 BOUND 時顯示「賣家正在修改…」。排在最後也與現行 `Rebind` 的檢查順序相同（終止、PENDING、不是 FAILED、申請，`intent.go:178-193`）［量測］，方法的行為不變。**等待刪除不能重新綁定（Will 2026-10-21，附錄 B #5）**：等待刪除只會走向關閉（`stores.sql:110-114`），開始一次新的 PayUni 綁定沒有目的；`stint.Lock` 只擋關閉是「關店即軟刪除」的沿用，不是針對 PayUni 的規則。句子是 §5 #N17。`can_rebind` 的意思是「此刻呼叫會開始一次新的綁定」。`Rebind` 改成鎖住店家（不擋關閉的 `q.LockStore`）與註冊、讀註冊的申請後呼叫同一個 `RebindUnavailableReasons`：清單空 → 重新綁定；第一個是 `PAYUNI_BIND_IN_PROGRESS`（表上標成 `idempotent`，不是失敗原因、沒有句子）→ 回目前的註冊、不另排 job（現行的重送安全，`intent.go:181-183`）；其他 → `FailedPrecondition`（第一句）。契約把這個例外寫進 `RebindPayUni` 與 `can_rebind` 的註解：「`PAYUNI_BIND_IN_PROGRESS` 時呼叫不回錯誤、回目前的註冊（重送安全）；其餘原因回 failed_precondition，訊息是第一句」。可用性與方法是同一個 Go 函式、同一個分支，規則一致（附錄 A M3）。client：清單空 → 顯示「重新綁定」；第一個原因是 `PAYUNI_BIND_IN_PROGRESS`、`PAYUNI_BOUND` → 沒有入口、不顯示句子；其他 → 顯示它的句子（取代 `PAYUNI_REONBOARDING_TEXT`，`MD/[id]/PayUniCards.tsx:130-134`）。`PAYUNI_MERCHANT_TERMINAL` 排在 `STORE_CLOSED` 前面，但它的句子不叫賣家關店（§5 #N10，附錄 A L7），已關閉而且終止的店家看到的句子也成立 | 契約 `RebindPayUni`（`P/seller.proto:84-87`，沒有 etag、由狀態保護、PENDING 重送回目前的註冊）；`intent.go:156-216`；`payuni_integration_test.go:632-641` 釘住 PENDING 的重送安全；5a F17 與 §7.1（申請檢查收成原因）。**替代**：PENDING 不列為原因（`can_rebind` 為 true）——畫面會在綁定中出現「重新綁定」，不採用；PENDING 改回 failed_precondition——改掉已定的重送安全語意，沒有新理由，不採用 |
| F6 | 刪除帳號在已關閉的店家（**要 Will 決定**：UI 出現與否） | 建議：刪除帳號的可用性不是店家狀態，seller 的回應不帶它；client 拿掉 `canDeleteAccount`（`known && !closed`），改成「帳號讀得到（`GetAccount` 成功，頁面已經在讀，`MD/[id]/store-detail.ts:211-224` `accountEmailOf`）才顯示」。已關閉的店家詳情頁會出現「刪除帳號」 | **從零**（附錄 A M6）：刪除是帳號層級的動作，當事人的刪除請求權不看他名下的商店是否還開著［外部，未查證；法規面是 REAL 但本次未查證條文］；平台的刪除流程本來就要處理「沒有開著的商店」的帳號。我們的程式是這個形狀——`DeleteAccount` 由 accountdeletion 擁有（`api/hawkr/admin/identity/v1alpha1/identity.proto:30-34`；`backend/internal/modules/accountdeletion/AGENTS.md:3-9`）；沒有目前的店家時 `withStore` 直接通過（`accountdeletion/internal/request/request.go:137-154`），已刪除的帳號回目前的狀態（`:113-115`）［量測］——但模組邊界是我們自己畫的（SELF-IMPOSED），論證不靠它。db D4-13 沒有明列刪除帳號。**替代**：維持隱藏——那是 client 自己的規則，沒有伺服器來源；要維持就得由 identity 的 `GetAccount` 給 `can_delete` 之類的欄位，判斷「從一個歷史 stint 的頁面刪帳號」是不是被允許，那是 identity 的產品決定 |
| F7 | 代行的店家狀態 | seller 擁有「店家此刻能不能被代行營運」的店家狀態那一半：`Store.ActionUnavailableReasons(StoreActionDelegate)` 由 `CapStorefrontWrite` 缺的條件對應（`STORE_ACTIVE` → `STORE_CLOSED`、`NOT_PENDING_DELETION` → `STORE_PENDING_DELETION`、`NOT_SUSPENDED` → `ACCOUNT_SUSPENDED`；`M/internal/model/capability.go:78`），根套件轉出。`StoreListItem` 加 `repeated StoreActionUnavailability delegation_unavailabilities`，註解寫明只含店家狀態，授權、管理員資格與進行中的代行由 preflight 判斷；**不加** `can_delegate`（只看店家狀態的 true 不代表能代行，bool 會誤導）。delegation 的 `storeStateOf` 改呼叫同一個函式（`Operable` = 清單為空），**狀態如實表達（附錄 B #2）**：已關閉是 `Found=true`、`Closed=true`、`Operable=false`，`StoreState` 帶第一個原因的句子 `Unavailable`；`Found=false` 只剩店家不存在（`Found` 的註解「不存在或已刪除」從此與內容一致）。模型新增拒絕 `DenialStoreClosed`（店家已關閉），preflight 在 `!Operable` 時依 `Closed` 選 `DenialStoreClosed` 或 `DenialStoreNotOperable`，`DeniedLabel` 一律是 `st.Unavailable`；開通授權（`authorization.go:234-240`）對已關閉也回 `DenialStoreClosed`，訊息同一句 N2。**舊錯誤碼只在 legacy 轉接層**：`internal/legacy/delegationservice/server.go:49-66` 的 `bizCodes` 加 `DenialStoreClosed: 9265`，對外的 9265／9272 不變（`server_integration_test.go:367-385` 照舊綠），模型裡沒有「已關閉＝不存在」。`DeniedLabel` 刪 `DenialStoreNotOperable` 分支（不可營運一定有原因；句子空時 preflight 回內部錯誤，同 §2.1 `errStoreActionUnavailable` 的規則），店家不存在仍是「此店家目前不可代行」 | 前提；labels:7（清單的句子與 `StartDelegation` 被拒時的錯誤要同一句：`session.go:181` 用 `pre.DeniedLabel` 組錯誤）；delegation 已經把店家狀態的判斷交給 seller（`DL/delegation.go:57-73` 的註解與 `st.Can(seller.CapStorefrontWrite)`）；計畫 :384「句子放 seller `text/`」。賣家端的 `SellerRequirement`（`api/hawkr/seller/v1alpha1/seller.proto:274-290`：STORE_ACTIVE、NOT_SUSPENDED、NOT_PENDING_DELETION）與這三個值一一對應，但那是賣家受眾的 enum，staff 的原因放 admin 契約。**替代 1**：delegation 擁有原因與句子（新的 `DL/internal/text`）——現在在 seller 的 proto 定 delegation 的 enum 是倒過來；不採用。**替代 2**：清單批次 preflight——違反清單 lazy 的既有設計、每列多一次授權查詢；列在 §0.2 的觸發。**替代 3**（v1.1 原案）：已關閉維持 `Found=false` 並掛句子——為了 9265 在模型裡保留 §0.5 已判為 SELF-IMPOSED 的「關店即軟刪除」，舊碼的對應應該在邊界的轉接層（附錄 B #2）；不採用 |
| F8 | admin 的呈現 | 一律「伺服器說能就顯示、說不能就依原因分支」：第一個原因是結構性的（`KYC_NOT_SUBMITTED`、`KYC_NOT_BUSINESS`、`PAYUNI_BIND_IN_PROGRESS`、`PAYUNI_BOUND`、`FOOD_REGISTRATION_NOT_PENDING`）＝沒有入口、不顯示句子；其他顯示 `message.message`，沒有 message 才顯示通用文字。**這個分類是伺服器的語意，SSOT 在契約**（附錄 B #7）：§1 的 enum 註解逐值標明「表示沒有入口」，照 L′ 的先例（`step10e-Lprime-design.md:452`）；admin 的 `A/lib/unavailability.ts` 照契約實作，寫成 `Record<StoreActionUnavailableReason, boolean>`（窮舉，契約新增值時 `tsc` 會要求決定），client 只有這一處。`unavailableLabels`（`A/app/admin/measures/measure-actions.ts:40-60`）搬到 `A/lib/unavailability.ts`，參數放寬成 `{ message?: LocalizedMessage }`，`GENERIC_UNAVAILABLE_LABEL` 一起搬（第二個使用者出現）。句子由伺服器給完整句（以「。」結尾），client 不加標點、不拼接成一句（§5 標點規則）。各處的接法見 §3 | `measure-actions.ts:33-34` 已寫明同一個前提；錯誤模型 ADR 的 client 規則（有文字就顯示，沒有才顯示通用文字） |
| F9 | 刪除到不了的分支 | `delegationBlockedLabel` 的「審核中」「審核未通過」只有測試釘住：唯一呼叫端永遠傳 `pendingReview:false, reviewRejected:false`（`MD/page.tsx:110-115`）；「尚未建立店鋪」也到不了（`H/admin.go:76` 的 `Id` 一定是正數的 store_id）；「未啟用」只在 status 是 UNSPECIFIED。整個函式與 `lifecycleBlockedLabel` 刪除（F7 取代） | §0.4 C11［量測］ |
| F10 | staff 句子的語系 | 句子放 seller `internal/text/zh-TW.json` 的 `staff` 段，`en.json` 不加（admin 只有 zh-TW）；函式沒有 locale 參數 | root `AGENTS.md`「使用者面文字」；L′ §1.3（`step10e-Lprime-design.md:133`）；先例 `text.ActionUnavailableAppealPending()` |
| F11 | admin PayUni 卡的徽章、問題文字、綁定失敗句（controller 裁定歸 5b） | 逐項分類（labels 的兩類）：**徽章**：名詞只由 `KYCFlowStatus` 的值決定，目錄已有 staff 名詞（`seller.KYCFlowStatus.*`，`flowBadge` 已在用）；但「這筆註冊現在在 PayUni 的哪個階段」是伺服器的判斷（綁定狀態優先、再看審核結果），client 的後備表（`kyc-application.ts:953-975`）是在重做伺服器的階段判斷，5a 已把它抽成 `model.PayUniPhaseOf(reg PayUniRegistration) KYCFlowStatus`（`model/payuni.go:505-526`；`KYCFlowStatusOf` 與 `sellerPayUniOf` 都呼叫它，`TestPayUniPhaseOf` 以 a6f7b3815 的 `DeriveKYCFlow` 為期望值鎖住全部 bind_status × outcome，`model/payuni_state_test.go:248-283`）［量測］。所以 admin `PayUniRegistration` 加 `hawkr.seller.v1alpha1.KYCFlowStatus flow_status`，直接用 `PayUniPhaseOf(reg)`（admin 讀的一定有註冊，不經 `KYCFlowStatusOf`）；client 一律 `flowBadge(registration.flowStatus)`，刪後備表、`PAYUNI_FLOWS` 與 `flow` 參數。**問題文字**：`payUniProblemText` 的「`problem.reason` 優先、再 `status_codes.reason`」是多餘的：`problem.reason` 就是 `Status.Reason` 去掉前後空白、空的時候不設（`model/payuni.go:558` 傳 `reg.Status.Reason`，5a 起 `problemOf` 經 `trimmedReason` 修剪，`:528-547`；`H/admin_kyc.go:470` 把同一個值原樣轉成 `status_codes.reason`）［量測］，而 `payUniProblemText` 最後也 `trim()`，所以結果永遠等於修剪後的 `status_codes.reason`［推論］。它是 PayUni 寫的原文（外部資料），不是我們的判斷或名詞；client 直接顯示 `statusCodes.reason`，刪 `payUniProblemText`。**綁定失敗句**「綁定失敗（錯誤碼 X），已嘗試 N 次。」與「需補資料：{欄位}」：是三個原始欄位（`bind_status`、`bind_error_code`、`bind_attempts`）與 `problem.fields` 的事實呈現，和「金流商家編號：」「最後同步：」同一類，欄位名已走目錄（`kycFieldLabel` → `seller.KYCField`）；不是判斷，留在 client（§5 #E7） | labels:7 的兩類；L′ §5.2「單端的表允許」只管名詞表，不涵蓋重做伺服器的推導；5a F12 交代「5b 開工時重看 admin 後備表是否還需要」，答案是不需要。顯示的差異只有一處：`outcome` 是 UNRECOGNIZED 時徽章由「狀態待確認」變成「PayUni 審核中」（`PayUniPhaseOf` 的既有規則，清單已經這樣顯示，§5 #C2）；其餘值的 staff 名詞與後備表的字相同（`uitext/catalog/zh-TW.json` 的 `staff` 段 `seller.KYCFlowStatus.*`：PayUni 綁定中、待重新綁定、已核准、PayUni 審核中、PayUni 待補件、PayUni 審核未通過）［量測，v1.3 重讀］ |
| F12 | 刪 admin `PayUniRegistration` 只為 client 推導存在的欄位（**改契約，要 Will 決定**） | 建議：刪 `outcome`（9）、`payment_enabled`（13）、`needs_reonboarding`（14），加 `reserved`。F5、F11 之後 admin 沒有讀者：`outcome` 只有後備表讀、`needs_reonboarding` 只有 `canRebind` 與 `PayUniCards.tsx:130` 讀、`payment_enabled` 沒有讀者（`rg -n "\.outcome\b\|needsReonboarding\|paymentEnabled" admin/src` 排除生成碼，§0.4 C20）。原始事實仍在 `status_codes`（PayUni 的原碼）。`PayUniOutcome` proto enum：5a 已刪賣家端的 `outcome`（賣家端 `PayUniRegistration` 整個刪除），admin 的 `outcome = 9` 是它最後一個使用者（`rg -n "PayUniOutcome" api` 只剩 admin `seller.proto:692` 與定義 `seller/v1alpha1/seller.proto:746-757`）［量測］，所以本刀刪 enum；Go 的 `model.PayUniOutcome` 留著（DB 的 `payuni_registrations_outcome_check`、`PayUniPhaseOf` 的輸入），只刪它對 proto 的對照（§9） | 同 5a F11 的判準（必要條件消失：留著的唯一作用是讓 client 再推導一次）；`v1alpha1` 不受 `buf breaking`（`api/buf.yaml:24-25`）。**替代**：保留當事實——`outcome` 是推導值不是原碼，員工排查看 `status_codes` |
| F13 | 食品業者登錄的核准提示（審查 M1 併入） | `FoodRegistration` 加 `approve_unavailabilities`、`reject_unavailabilities`（`can_approve`、`can_reject` 已在，`P/seller.proto:846-849`）。原因：`FOOD_REGISTRATION_NOT_PENDING`、`STORE_CLOSED`、`STORE_PENDING_DELETION`（現行「店家營運中」的兩種不成立），核准另有 `KYC_NOT_APPROVED`。`FoodRegistrationReview.CanReject`／`CanApprove`（`model/foodreg.go:189-200`）改成「原因清單為空」；`Decide`（`foodreg/foodreg.go:365-373`）與 `requireKYCApproved`（`:453-457`）的三句硬寫錯誤改走 `errStoreActionUnavailable`。client 刪 `approveBlockedText`、`APPROVE_BLOCKED_KYC_TEXT`，提示改成 `unavailableLabels(approve, reject)`（第一個原因結構性時不顯示）。已關閉、等待刪除的店家的件現在會顯示原因（現行兩鍵都不在時什麼都不說） | 審查 M1：同一種 client 推導原因的違規；v1 延後的理由「另一個資源」不是 REAL 約束（同模組、同 admin、F1 的 enum 已涵蓋「對店家做的動作」）。現行的伺服器句「店家的實名認證還沒有核准」與 client 句「店家還沒通過商家認證」說法不一，術語表:77 是「商家認證」，合併成一句（§5 #N15） |

---

## 0.4 查證過的現況（`a6f7b3815`；v1.3 對 `1fc535d87` 複核，有變的列已改寫，C28–C30 是新增）

| # | 事實 | 證據 |
|---|---|---|
| C1 | `storeActions` 的六個旗標全部由 `store.status` 與 `active_measures` 推導；`canCreateMeasure`／`canEndMeasure`／`canDeleteAccount`／`canMaintainKyc` = 狀態已知而且沒關閉，`canReviewKyc` = ACTIVE 而且沒有生效中的措施 | [量測] `MD/[id]/store-detail.ts:99-115` |
| C2 | 三句 `REVIEW_BLOCKED_*` 與 `reviewBlockedReason` 只在申請是 SUBMITTED 時顯示 | [量測] `store-detail.ts:117-139`；`MD/[id]/page.tsx:269-278,986-991` |
| C3 | `GetStoreResponse` 只有 `can_create_termination`＋`available_terminations`，沒有建立措施、審核的可用性；`StoreListItem` 只有 `store`、`kyc_flow_status`、`suspended` | [量測] `P/seller.proto:204-213,270-288`；`H/admin.go:163-171,187-226` |
| C4 | `AllowsMeasure()` 是建立警告、停權、風險措施的唯一規則，來函已在用；案件與通報另外直接讀 `f.StoreClosed`，句子是各自的 `case.unavailable.store_closed`、`report.unavailable.store_closed`（description 寫「建立措施被擋下時照舊回店家已關閉」） | [量測] `M/internal/model/measure.go:417-419`；`model/authority_order.go:337-341`；`model/report_case.go:393-394,475-476,570-571,683-684`；`M/internal/text/zh-TW.json:339-342,519-522`（v1.3；5a 在前面加了 key） |
| C5 | 建立措施對已關閉的店家回 `apperr.NotFound("店家已關閉")`；建立終止的兩個擋下也是 `NotFound` | [量測] `M/internal/measure/create.go:487-507` |
| C6 | `CanEnd()` 不看店家；`End` 的錯誤是硬寫句，不經 `ActionUnavailableText`；admin 措施頁的結束鍵不能做時隱藏 | [量測] `M/internal/model/measure.go:795-797`；`M/internal/measure/update.go:202-203`；`A/app/admin/measures/measure-actions.ts:113` |
| C7 | 核准：鎖內依序檢查版本、check seq、SUBMITTED、`locked.Status != Active`「店家不在營業中，不能核准」、`stint.Derive` 停權「申請人的帳號停權中，不能核准」、身分命中；全部是沒有 reason 的 `FailedPrecondition` | [量測] `M/internal/kyc/admin.go:381-399,407-440` |
| C8 | 已關閉的店會留著 SUBMITTED 的申請，而且今天唯一的出口是退回：退回與重新查詢不看店家狀態（`lockApplication` 用 `LockAccountOf`）；關店（`close`）與 `CloseStore` 不動 `kyc_applications`；賣家撤不回（`Cancel` 走 `stint.Lock`，已關閉回 NotFound）；個資清理保留 status | [量測] `kyc/admin.go:370-375,469-500`、`kyc/registrycheck.go:123-141`、`closing/closing.go:214-256`、`db/queries/stores.sql:116-124`、`kyc/kyc.go:452`、`stint.go:28-29`、`db/queries/purge.sql:93-96`（v1 的 [推論] 改為量測，附錄 A L1） |
| C9 | 重新綁定：`stint.Lock` 對已關閉回 `NotFound`；`needs_reonboarding`、不是 FAILED 回 `FailedPrecondition`，PENDING 回目前的註冊（契約寫明重送安全，測試釘住）；**5a 起**，FAILED 而註冊所根據的申請不是 APPROVED 時回 `FailedPrecondition`（`model.ErrRebindApplicationNotApproved`，句子「賣家正在修改商家認證，重新核准後會自動重新綁定。」），檢查寫在方法裡、在前三個條件之後；client 的 `canRebind` = 沒關閉、FAILED、不是 needs_reonboarding（不看申請，所以今天賣家改回草稿時 admin 仍顯示「重新綁定」，按下去才被拒）；`PAYUNI_REONBOARDING_TEXT` 在 needs_reonboarding 時一律顯示 | [量測] `M/internal/payuni/intent.go:156-216`（申請檢查 `:187-193`）；`M/internal/model/payuni_state.go:178-182`；`M/internal/text/zh-TW.json:443-446`；`P/seller.proto:84-87`；`M/payuni_integration_test.go:632-641`；`M/internal/stint/stint.go:20-32`；`MD/[id]/kyc-application.ts:977-994`；`MD/[id]/PayUniCards.tsx:130-134,146` |
| C10 | 刪除帳號：admin 申請時店家是參與者，沒有目前的店家（已關閉）直接通過，已刪除回目前狀態；client 在已關閉時隱藏 | [量測] `accountdeletion/internal/request/request.go:107-154,174-177`；`MD/[id]/page.tsx:1062-1072` |
| C11 | `delegationBlockedLabel` 唯一的呼叫端是 `MD/page.tsx:110`，固定傳 `pendingReview:false, reviewRejected:false`；`StoreListItem.store.id` 一定有值 | [量測] `rg -n "delegationBlockedLabel\(" admin/src`（1 處）；`H/admin.go:74-76` |
| C12 | 清單的代行判斷 `isActive && !isSuspended && Boolean(storeId)` 與 delegation 的 `CapStorefrontWrite`（ACTIVE、沒停權、不是等待刪除）結果相同；清單的店家已帶條件（`stint.WithConditions`），`Store.Can` 可以直接用 | [量測] `MD/page.tsx:99-103`；`DL/delegation.go:57-73`；`model/capability.go:78`；`M/internal/profile/admin.go:91-117` |
| C13 | preflight：先管理員資格、再店家（已關閉與不存在都是 `Found=false` → 9265）、再讀授權、再 `!Operable` → 9272、再授權有效、再進行中；`StartDelegation` 的錯誤用 `pre.DeniedLabel` | [量測] `DL/internal/session/session.go:76,80-135,181`；`internal/legacy/delegationservice/server.go:60` |
| C14 | 「停權中，無法代行」在設計書 56 出現四處（:2398 是 `deniedLabel` 範例「此店家目前停權中，無法代行」、:2459、:5813、:5858）；:5370 是 9272 的說明，沒有這句；`DeniedLabel` 的「此店家已停權或關閉，無法代行」也被 `admin/tests/delegated-operation-preflight-denial.test.ts` 當 fixture；CodeMeta `錯誤碼.md:345` 也有這句，但 CodeMeta 凍結（root `AGENTS.md`），不改 | [量測] `rg -n "停權中，無法代行\|已停權或關閉，無法代行" backend/docs` |
| C15 | 商家詳情頁的代行入口已經只走 preflight（`resolveEligibility`），沒有本地判準 | [量測] `MD/[id]/page.tsx:977-985` |
| C16 | `ErrorReason` 沒有這些拒絕的值；本刀不新增（原因走回應欄位，錯誤只帶句子，同 `errActionUnavailable`） | [量測] `rg -n "WithReason" M/internal/kyc M/internal/measure`（這些路徑沒有命中）；`M/internal/measure/update.go:169-176` |
| C17 | `RejectKycDialog` 在 `blocked`（`!reviewable.canReject`）時顯示 client 寫的「這份申請目前不能退回（已經不是待審核狀態，或店家已關閉）。…」；申請被別人處理之後就會成立 | [量測] `MD/[id]/RejectKycDialog.tsx:331-337`；`page.tsx:1538` |
| C18 | 恢復營業只發生在撤銷終止合作：同一個 stint CLOSED → ACTIVE，KYC 與資料都在，PayUni 以 `Resume` 補排；等待刪除只從 ACTIVE 轉入、沒有反向；`store_closures.reopened_at` 記恢復時間 | [量測] `closing/closing.go:357-376`；`intent.go:47-64`；`db/queries/stores.sql:110-114,133-137`；`db/schema.sql:112,128-130` |
| C19 | 審核待辦：`review_kyc_application` 只看 SUBMITTED，不看店家狀態，期限送出後 168 小時；`decide_deadline` 用同一支函式；食品業者登錄的函式要 `s.status = 'ACTIVE'`，開始時間是 max(送出, KYC 核准)，不看恢復營業 | [量測] `M/db/objects.sql:228-252`；`db/queries/kyc.sql:363-366`；`kyc/admin.go:270` |
| C20 | admin PayUni 卡：徽章在流程不在 PayUni 階段時用 client 後備表（`bind_status` 再 `outcome`）；問題文字取 `problem.reason \|\| statusCodes.reason`，兩者是同一個值；綁定失敗句與「需補資料」是 client 的事實呈現；F5、F11 之後 admin 不再讀 `outcome`、`needsReonboarding`，`paymentEnabled` 現在就沒有讀者 | [量測] `MD/[id]/kyc-application.ts:931-1013`；`PayUniCards.tsx:103-135`；`M/internal/model/payuni.go:505-574`（v1.3：`PayUniPhaseOf`、`problemOf`、`ProblemOf`）；`H/admin_kyc.go:462-486`；`rg -n "\.outcome\b\|needsReonboarding\|paymentEnabled" admin/src`（排除生成碼：只有 `kyc-application.ts:963,988` 與 `PayUniCards.tsx:130`） |
| C21 | 食品業者登錄：`CanReject` = 待審而且店家 ACTIVE，`CanApprove` 再加 KYC 已核准；`Decide` 的三句硬寫錯誤；client 由 `canReject && !canApprove` 推出原因並寫自己的句子 | [量測] `M/internal/model/foodreg.go:189-200`；`M/internal/foodreg/foodreg.go:365-373,453-457`；`A/lib/food-registration.ts:20,169-176`；`A/app/admin/food-registrations/[id]/page.tsx:145,365-367` |
| C22 | `LockOpenAccountOf` 沒有呼叫端；它的註解「新的措施不能建在已關閉的 stint 上，回 NotFound」正是 F2 推翻的規則 | [量測] `rg -n "LockOpenAccountOf" backend`（只有 `stint.go:57-59` 的定義） |
| C23 | `Store.Suspended()` 與 `stint.Derive` 等價：`Derive` 就是 `Effective(...).Suspended()`；`Load` 用同一個 `Effective` 填 `Sanctions`；`Store.Suspended()` 就是 `Sanctions.Suspended()`。now 相同時結果一定相同 | [量測] `stint.go:198-201,217-223`；`model/model.go:58,220`（v1 的 [推論] 改為量測，附錄 A L1） |
| C24 | `gateway-json-contract-check.py` 不比對字串值：Check A 比對 JSON key 名稱，Check B 比對規格欄位與 handler 輸出的 key；gateway 原樣轉出 `DeniedLabel` | [量測] `backend/scripts/gateway-json-contract-check.py:13-30`；`backend/gateway/internal/handler/admin_delegation.go:535`（v1 的 [推論] 改為量測，附錄 A L1） |
| C25 | 「待審核」今天有三個定義：待辦 `review_kyc_application`（只看 SUBMITTED）、商家清單「待審核」分頁（`AdminStores` 的 `k.status = kyc_status`，admin 傳 SUBMITTED）、統計 `AdminStoreStats.kyc_submitted`（排除已關閉、含等待刪除），後者餵首頁「賣家待審核」與商家頁「待審核」兩張卡；`ListKYCApplications` 也能以 SUBMITTED 篩選，但 admin 沒有呼叫端（v1.2 前 §9 用 `rg "'SUBMITTED'"` 抓不到參數化的過濾） | [量測] `M/db/queries/stores.sql:200-205,226-238`；`A/lib/store-search.ts:60-61`；`A/app/admin/page.tsx:145-152`；`MD/page.tsx:225-233`；`rg -n "kyc_status\|kycSubmitted\|KYC_STATUS_SUBMITTED"`；`rg -n "listKYCApplications" admin/src`（排除生成碼，沒有命中） |
| C26 | 停權：`Store.Suspended()` 是帳號有任何生效中的措施（停權或終止），SQL 是 `seller.effective_measures(at)`（`ENFORCED`、`starts_at <= at`、`ends_at` 為空或晚於 at）；措施的結束是 `ends_at` 到期或撤銷（`status = 'REVOKED'`、`revoked_at`）。食品業者登錄的裁定與待辦都不看停權 | [量測] `model/model.go:58,220`；`db/objects.sql:44-50`；`db/schema.sql:735-785`；`model/foodreg.go:189-200`；`foodreg/foodreg.go:365-373`；`objects.sql:229-240` |
| C27 | 9265／9272 只在 legacy 轉接層對應：`bizCodes` 是 `DelegationDenial → int32`；delegation 模型裡已關閉的店家走 `Found=false` → `DenialStoreNotDelegable`，開通授權也是（`authorization.go:238-239`） | [量測] `internal/legacy/delegationservice/server.go:49-66`；`DL/delegation.go:56-73`；`DL/internal/session/session.go:88-111` |
| C28 | 5a 的模型（v1.3 新增）：`PayUniPhaseOf(reg PayUniRegistration) KYCFlowStatus` 已存在，`KYCFlowStatusOf(status, reg)` 是只要階段的入口（批次讀 `stint.FlowStatusesOf` 只呼叫它），`DeriveKYCFlow` 的 `CanResubmit` 呼叫內部的 `sellerPayUniOf(reg)`；賣家看的狀態由 `payuni.SellerViewOf` 一次載入註冊（`withSellerType` 已讀註冊所根據的申請）後推導；`model.KYCFlow` 只剩 `Status`、`CanResubmit`、`IdentityLocked` | [量測] `M/internal/model/payuni.go:463-526`；`M/internal/model/payuni_state.go:77-138`；`M/internal/stint/flow.go:35-58`；`M/internal/payuni/intent.go:235-251,288-330` |
| C29 | 5a 的後台待辦 `seller.review_payuni_bind_retry(at)`：FAILED 而且 `bind_error_code = 'PAYUNI_UNAVAILABLE'`、店家 `ACTIVE`、註冊所根據的申請 `APPROVED`；Go 的成員定義是賣家的 PayUni 狀態 RETRY_LATER；生命週期測試斷言申請改回草稿時 `RebindPayUni` 被拒、訊息是 #38 那一句 | [量測] `M/db/objects.sql:255-268`；`M/review_policy_integration_test.go:715-737,744-788`；`M/payuni_state_integration_test.go:113` |
| C30 | `PayUniOutcome`（proto）只剩 admin `PayUniRegistration.outcome` 一個使用者；Go 端的對照只有 `H/admin_kyc.go:472` 的轉換、`H/kyc_test.go:468` 的 `EnumMismatches` 與 `H/admin_kyc_test.go:448` 的斷言；ui-text 的兩份 enum manifest 列它但 `inCatalog: false`（目錄沒有它的名詞） | [量測] `rg -n "PayUniOutcome" -g '!**/gen/**'`；`packages/hawkr_ui_text/tool/enum_manifest.g.json:179-183` |

## 0.5 約束帳（本刀新出現的）

| 約束 | 判定 | 來源 |
|---|---|---|
| 身分命中不透露 | **REAL**（反探測的安全需求，已在契約） | `P/seller.proto:64-66`；`model/kyc.go:701` |
| 刪除請求權不看商店狀態 | **REAL**（法規），條文本次未查證［外部］ | F6 |
| 措施以帳號為範圍、關店不解除停權 | SELF-IMPOSED（我們的資料模型）；F4 改用業界基準論證，結論與它一致（附錄 A M6） | `closing/closing.go:216` |
| 刪除帳號歸 accountdeletion、帳號層級 | SELF-IMPOSED（我們畫的模組邊界，`internal/arch/modules.go`）；F6 改用業界基準論證 | accountdeletion `AGENTS.md:3-9` |
| `RebindPayUni` 的 PENDING 重送安全 | SELF-IMPOSED（我們的契約）；保留的理由是從零也會這樣設計：沒有 etag 的狀態轉換靠狀態冪等（F5） | `P/seller.proto:84-87` |
| 代行的 REST 傳輸與 9265／9272 對應 | SELF-IMPOSED（計畫分段：A10 才搬）；本刀不碰，理由是 A10 會整個重寫，不是外部限制 | 遷移計畫 :227 |
| 清單代行入口 lazy、不批次 preflight | SELF-IMPOSED（`MerchantOperationEntry` 的設計取捨） | `MerchantOperationEntry.tsx:66-78` |
| 設計書 56 的「停權中，無法代行」 | SELF-IMPOSED（我們的設計書）；F7 改四處，文末修訂 | 設計書 56 `:2398,2459,5813,5858` |
| 已關閉時隱藏所有操作 | SELF-IMPOSED（db D4-13，Will 2026-10-09 核可；但它寫的是「停權、關店、核准等」）；F4、F6 偏離的部分列給 Will | db D4-13 |
| 「店家已關閉」回 `NotFound` | SELF-IMPOSED（舊 seller-service「關店即軟刪除」的沿用）；F2 在 admin 路徑改 | `stint.go:29`；`server_integration_test.go:367` 的註解 |
| delegation 模型裡「已關閉＝找不到」（`Found=false`） | SELF-IMPOSED（同上一列的沿用，只為 9265）；F7 改成 `Found=true`＋`DenialStoreClosed`，9265 只留在 legacy 轉接層（附錄 B #2） | `DL/delegation.go:56-57` 的註解 |
| 上線前的相容性 | 不是約束 | root `AGENTS.md` |

---

## 1. 契約（F1、F2、F3、F5、F7、F11、F12、F13）

`P/seller.proto`：

```proto
// StoreActionUnavailableReason 是 admin 此刻不能對店家（或它的帳號、商家認證申請、PayUni 註冊、食品業者登錄）做某個動作的原因
// （StoreActionUnavailability.reason）。每個動作只會出現它相關的原因，依該動作的先後排列，與對應方法的前置檢查是同一個規則
// （model/store_action.go 的有效組合表）。client 用它分支，不用它組顯示文字。分支規則：依先後，第一個是下面標明「表示沒有入口」的值
// （KYC_NOT_SUBMITTED、KYC_NOT_BUSINESS、PAYUNI_BIND_IN_PROGRESS、PAYUNI_BOUND、FOOD_REGISTRATION_NOT_PENDING）時，畫面不顯示這個動作的
// 入口，也不顯示句子；其餘的值顯示 message。新增值時要在它的註解寫明是否表示沒有入口。
enum StoreActionUnavailableReason {
  STORE_ACTION_UNAVAILABLE_REASON_UNSPECIFIED = 0;
  // 商家認證申請不在審核中（SUBMITTED）。表示沒有入口。
  STORE_ACTION_UNAVAILABLE_REASON_KYC_NOT_SUBMITTED = 1;
  // 不是公司型的申請，沒有登記資料可以查詢。表示沒有入口。
  STORE_ACTION_UNAVAILABLE_REASON_KYC_NOT_BUSINESS = 2;
  // PayUni 端的商店已是終止狀態（needs_reonboarding）。
  STORE_ACTION_UNAVAILABLE_REASON_PAYUNI_MERCHANT_TERMINAL = 3;
  // PayUni 註冊正在綁定（PENDING）。表示沒有入口；不是失敗原因、沒有 message：RebindPayUni 此時不回錯誤，回目前的註冊（重送安全）。
  STORE_ACTION_UNAVAILABLE_REASON_PAYUNI_BIND_IN_PROGRESS = 4;
  // PayUni 註冊已綁定（BOUND）。表示沒有入口。
  STORE_ACTION_UNAVAILABLE_REASON_PAYUNI_BOUND = 5;
  // 食品業者登錄不在待審。表示沒有入口。
  STORE_ACTION_UNAVAILABLE_REASON_FOOD_REGISTRATION_NOT_PENDING = 6;
  // 店家目前的商家認證還沒核准（核准食品業者登錄；重新綁定 PayUni 時是註冊所根據的申請不是已核准，例如賣家正在修改）。
  STORE_ACTION_UNAVAILABLE_REASON_KYC_NOT_APPROVED = 7;
  // 店家已關閉。
  STORE_ACTION_UNAVAILABLE_REASON_STORE_CLOSED = 8;
  // 店家等待刪除（帳號刪除在等未完成的訂單）。
  STORE_ACTION_UNAVAILABLE_REASON_STORE_PENDING_DELETION = 9;
  // 帳號有生效中的停權措施。
  STORE_ACTION_UNAVAILABLE_REASON_ACCOUNT_SUSPENDED = 10;
}

// StoreActionUnavailability 是此刻不能做某個動作的一個原因；message 與該方法被拒絕時錯誤的 LocalizedMessage 是同一句（同一個 Go 函式）。
message StoreActionUnavailability {
  StoreActionUnavailableReason reason = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 給員工看的完整一句話（admin RPC 固定 zh-TW，以「。」結尾），有主詞、不含 client 的畫面名稱；client 直接顯示、不加標點，
  // 沒有 message 時才顯示通用文字。PAYUNI_BIND_IN_PROGRESS 沒有 message（不是失敗原因）。
  google.rpc.LocalizedMessage message = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
}
```

欄位（編號取各 message 下一個可用值；註解句型照 `can_create_termination`）：

- `GetStoreResponse`：`can_create_measure`、`create_measure_unavailabilities`（「與 CreateMeasure 的前置條件同一份規則：店家還沒關閉。client 只呈現，不從 store.status 推導。」）。
- `StoreListItem`：`delegation_unavailabilities`（「店家此刻不能被代行營運的店家狀態原因（已關閉、等待刪除、停權），與代行 preflight 的店家狀態檢查同一個規則；空不表示能代行：授權、管理員資格與進行中的代行在 preflight 判斷。」）。
- `KYCApplication`：`can_approve`／`approve_unavailabilities`、`can_reject`／`reject_unavailabilities`、`can_requery_registration`／`requery_registration_unavailabilities`；`can_approve` 註解加「身分命中別的帳號生效中的處分時核准仍回 failed_precondition，不列在原因裡（不透露）」。
- `ListStoresRequest`（F3）：`bool kyc_awaiting_review`（「true 只列商家認證申請在後台待辦裡的店家：審核中、店家營運中、帳號沒有停權，與後台待辦、`GetStoreStatsResponse.kyc_awaiting_review_stores` 同一個成員；false 不篩選」）；`kyc_status` 的註解改成只接受 APPROVED、REJECTED，SUBMITTED 與 DRAFT、CANCELLED 一樣回 invalid_argument（「待審核」用 `kyc_awaiting_review`）。
- `GetStoreStatsResponse`（F3）：`kyc_submitted_stores = 2` 改成 `reserved 2; reserved "kyc_submitted_stores";`，新增 `int32 kyc_awaiting_review_stores`（「商家認證申請在後台待辦裡的店家數，與 `ListStoresRequest.kyc_awaiting_review` 同一個成員」）。
- `PayUniRegistration`：`can_rebind`（「此刻呼叫 RebindPayUni 會開始一次新的綁定」）／`rebind_unavailabilities`；`hawkr.seller.v1alpha1.KYCFlowStatus flow_status`（「這筆註冊所在的 PayUni 階段：BINDING、BIND_FAILED、FORWARDED、UNDER_REVIEW、PENDING_INFO、APPROVED、PAYUNI_REJECTED 之一，與 KYCFlow 在申請已核准時的推導同一個函式（`PayUniPhaseOf`）；client 用它顯示徽章，不從 bind_status、status_codes 推導。」）；F12 `reserved 9, 13, 14; reserved "outcome", "payment_enabled", "needs_reonboarding";`。
- `FoodRegistration`：`approve_unavailabilities`、`reject_unavailabilities`；`can_approve`、`can_reject` 的註解改成「等於對應的 `*_unavailabilities` 是空的」。
- RPC 註解：`ApproveKYC`、`RejectKYC`、`RequeryKYCRegistration`、`DecideFoodRegistration` 寫明「不能做時回 failed_precondition，訊息是對應 `*_unavailabilities` 的第一句」；`RejectKYC`、`RequeryKYCRegistration` 補「店家不是營運中」（F3）；`RebindPayUni` 改寫成 F5 的語意（`PAYUNI_BIND_IN_PROGRESS` 回目前的註冊，其餘原因 failed_precondition；「店家已關閉」由 not_found 改成 failed_precondition，等待刪除、註冊所根據的申請不是已核准也是 failed_precondition）；`ApproveKYC`、`RejectKYC` 的註解補「帳號停權時不能核准、可以退回；這段期間申請不在後台待辦」。

`P/measure.proto`：`CreateMeasure` 的店家已關閉改 failed_precondition（句子同 `GetStoreResponse.create_measure_unavailabilities`）；`can_end` 註解補「不看店家是否關閉（措施以帳號為範圍）」。

賣家端 `api/hawkr/seller/v1alpha1/seller.proto:746-757`：刪整個 `enum PayUniOutcome`（F12，C30；proto 沒有對型別名的 reserved，上線前沒有相容性約束，root `AGENTS.md`）。

三個生成端重新生成（同 4b §1），`make -C backend uitext` 重新生成兩份 enum manifest（`PayUniOutcome` 那一項消失）；`buf breaking`：`v1alpha1` 不受檢查（`api/buf.yaml:24-25`），F12 的欄位刪除照 `reserved` 寫法。

## 2. 單體

### 2.1 seller 模組

- `M/internal/model/store_action.go`（新）：
  ```go
  type StoreAction string // CREATE_MEASURE、DELEGATE、APPROVE_KYC、REJECT_KYC、REQUERY_KYC、REBIND_PAYUNI、APPROVE_FOOD_REGISTRATION、REJECT_FOOD_REGISTRATION
  type StoreActionUnavailableReason string // 值與 proto 一致（rpc.EnumToProto）

  // storeActionFacts 是推導需要的事實；各動作只填它用得到的部分。
  // PayUniApp 是 PayUni 註冊所根據的申請的狀態（REBIND_PAYUNI 的 KYC_NOT_APPROVED，5a F17）。
  type storeActionFacts struct { Store Store; KYC *KYCApplicationState; PayUni *PayUniRegistration; PayUniApp KYCStatus; FoodReg *FoodRegistrationReview }
  type storeActionRule struct {
      Reason  StoreActionUnavailableReason
      Applies func(storeActionFacts) bool
      Kind    ruleKind // blocking（回 failed_precondition、有句子）或 idempotent（回目前的狀態、沒有句子）
  }
  // storeActionRules 是有效組合表，也是規則本身：推導＝依序跑 Applies，成立的 Reason 依表的順序收集。順序與組合只有這一份。
  var storeActionRules = map[StoreAction][]storeActionRule{ … }
  func unavailableReasons(a StoreAction, f storeActionFacts) []StoreActionUnavailableReason

  // 以下四支只組 facts 再呼叫 unavailableReasons，不寫條件與順序。
  func (s Store) ActionUnavailableReasons(a StoreAction) []StoreActionUnavailableReason   // CREATE_MEASURE、DELEGATE：只看店家
  func KYCActionUnavailableReasons(s Store, app KYCApplicationState, a StoreAction) []StoreActionUnavailableReason
  func RebindUnavailableReasons(s Store, reg PayUniRegistration, app KYCStatus) []StoreActionUnavailableReason // app：註冊所根據的申請的狀態
  func FoodRegistrationUnavailableReasons(s Store, r FoodRegistrationReview, a StoreAction) []StoreActionUnavailableReason
  func StoreActionUnavailableText(r StoreActionUnavailableReason, a StoreAction) uitext.Message // 表外與 idempotent 的組合回零值
  ```
  有效組合表（先後即順序；每列帶自己的條件，附錄 B #4）：

  | 動作 | 原因與條件（依序） |
  |---|---|
  | CREATE_MEASURE | `STORE_CLOSED`：`!TerminationTarget{Closed}.AllowsMeasure()` |
  | DELEGATE | `STORE_CLOSED`：缺 `STORE_ACTIVE`；`STORE_PENDING_DELETION`：缺 `NOT_PENDING_DELETION`；`ACCOUNT_SUSPENDED`：缺 `NOT_SUSPENDED`（`missingFor(CapStorefrontWrite, s)`，F7） |
  | APPROVE_KYC | `KYC_NOT_SUBMITTED`：申請不是 SUBMITTED；`STORE_CLOSED`：`Status == Closed`；`STORE_PENDING_DELETION`：`Status == PendingDeletion`；`ACCOUNT_SUSPENDED`：`Store.Suspended()` |
  | REJECT_KYC | `KYC_NOT_SUBMITTED`、`STORE_CLOSED`、`STORE_PENDING_DELETION`（條件同上；停權不擋退回，F3） |
  | REQUERY_KYC | `KYC_NOT_SUBMITTED`；`KYC_NOT_BUSINESS`：申請不是公司型；`STORE_CLOSED`、`STORE_PENDING_DELETION`（條件同上） |
  | REBIND_PAYUNI | `PAYUNI_MERCHANT_TERMINAL`：`NeedsReonboarding`；`STORE_CLOSED`；`STORE_PENDING_DELETION`（F5，附錄 B #5）；`PAYUNI_BIND_IN_PROGRESS`：`bind_status == PENDING`，**idempotent**；`PAYUNI_BOUND`：`bind_status == BOUND`；`KYC_NOT_APPROVED`：`PayUniApp != APPROVED`（v1.3，5a F17，順序的理由見 F5） |
  | APPROVE_FOOD_REGISTRATION | `FOOD_REGISTRATION_NOT_PENDING`；`STORE_CLOSED`；`STORE_PENDING_DELETION`；`KYC_NOT_APPROVED`：店家目前的申請不是 APPROVED |
  | REJECT_FOOD_REGISTRATION | `FOOD_REGISTRATION_NOT_PENDING`；`STORE_CLOSED`；`STORE_PENDING_DELETION` |

  - 單元測試 `store_action_test.go`：`blocking` 組合有非空、以「。」結尾的句子；`idempotent` 與表外的組合回零值（N12 刪除，附錄 B #6）；推導就是跑表，所以不另測「依表的順序」（v1.1 的那個突變刪除），改成每個動作在狀態組合的表格上斷言**第一個原因**（例：已關閉而且 PENDING 的 REBIND 第一個是 `STORE_CLOSED`、停權而且已關閉的 APPROVE_KYC 第一個是 `STORE_CLOSED`、BOUND 而且申請是草稿的 REBIND 第一個是 `PAYUNI_BOUND`、FAILED 而且申請是草稿的 REBIND 只有 `KYC_NOT_APPROVED`）；DELEGATE 與 `Store.Can(CapStorefrontWrite)` 在所有狀態組合上等價（**突變**：把 DELEGATE 改回只看 `Status` 時要失敗）；`APPROVE_KYC` 清單為空與 `review_kyc_application` 的成員在 §6.1 的資料集測試比對。
- `errStoreActionUnavailable(reasons, action)`：`apperr.FailedPrecondition("%s", StoreActionUnavailableText(reasons[0], action).Text)`；清單空時回內部錯誤（L′ §3.2 同一個規則：不寫通用句）。
- 前置檢查改走它：
  - `measure/create.go:492-494`：`STORE_CLOSED`；`:503-506` 改 `FailedPrecondition`，句子用 `text.TerminationUnavailableStoreClosed()`、`text.TerminationUnavailableStoreSuperseded()`（§5 #E3、#E4）。
  - `kyc/admin.go` 的 `lockForReview`：版本與 check seq 照舊先檢查（etag 不符先於狀態）；之後 `stint.Load(locked)` 得到 `Store`，`Approve`、`Reject` 各自呼叫 `KYCActionUnavailableReasons`；`Approve` 的停權判斷改用載入的 `Store.Suspended()`，刪 `stint.Derive` 呼叫（等價已量測，§0.4 C23）。身分命中照舊在之後。
  - `kyc/registrycheck.go:136-141`：同上，`REQUERY_KYC`。
  - `payuni/intent.go:160-216` `Rebind`：`stint.Lock`（`:167`）改成不擋關閉的 `q.LockStore` 加 `stint.Load`；鎖註冊（`:170`）、讀註冊所根據的申請（現行 `:187` 的 `q.KYCApplicationByID`，移到判斷之前、對每個狀態都讀）後呼叫 `RebindUnavailableReasons(store, current, app.Status)`，依 F5 分三支（重新綁定、`PAYUNI_BIND_IN_PROGRESS` 回目前的註冊、其他 `errStoreActionUnavailable`）。刪現行的 `switch`（`:178-186`）與申請檢查（`:187-193`），刪 `model.ErrRebindApplicationNotApproved`（`model/payuni_state.go:178-182`）。鎖的順序不變（先店家列、再註冊列），賣家的 `SaveKYCDraft`／`SubmitKYC` 也先鎖店家列（`kyc/kyc.go:121`、`:364`），所以申請狀態在兩邊之間是序列化的［推論：讀程式，沒有跑並行測試］。5a 完成前 design-review 指出的缺口（v1.2 的兩個輸入看不到申請狀態，F17 會留成方法裡的第二個條件，違反附錄 A M3）由這一條補上。
  - `foodreg/foodreg.go:365-373,453-457`：`Decide` 依結果（核准、退回）呼叫 `FoodRegistrationUnavailableReasons`；`requireKYCApproved` 併進去；`FoodRegistrationReview.CanApprove`／`CanReject` 改成清單為空。
  - `measure/update.go:203`：`text.MeasureEndUnavailable()`（§5 #E5）。
  - `model/report_case.go:570-571`：`!TerminationTarget{Closed: f.StoreClosed}.AllowsMeasure()`；`:683-684` 同；`:393-394`、`:475-476` 的句子改回 `StoreActionUnavailableText(STORE_CLOSED, CREATE_MEASURE)`（L6），刪 `text.CaseUnavailableStoreClosed`、`text.ReportUnavailableStoreClosed` 與兩個 key。
- 審核期限與待辦（F3）：`db/objects.sql` 的 `review_kyc_application` 加 `AND s.status = 'ACTIVE' AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = s.account_id)`（停權的 SQL 定義只有 `effective_measures`，同 `pub_exposable_stores`，`objects.sql:64`），函式註解寫明成員對應的 Go 定義是 `APPROVE_KYC` 的原因清單為空。開始時間：`review_kyc_application` 是 greatest(送出, 這個 stint 最近一次 `store_closures.reopened_at`, 帳號最近一次停權結束)；`review_food_registration` 是 greatest(送出, KYC 核准, 最近一次 `reopened_at`)（不看停權：它的裁定本來就不看停權，C26）。「最近一次停權結束」＝帳號在 at 以前已結束的停權與終止措施的結束時間的最大值（到期是 `ends_at`、撤銷是 `revoked_at`；只算真的生效過的，即結束時間晚於 `starts_at`），寫成 `seller.account_sanction_lifted_at(account_id, at)` 一支函式，只有這裡用；它是「措施生效中」定義的第三種讀法，加進 seller `AGENTS.md` 那條清單並以邊界測試與 `effective_measures` 鎖住（§6.1）。新增一支 migration（照 seller `AGENTS.md` 的 schema 流程；兩支函式被 `pub_review_items` 依賴，寫法參考 5a 的 `db/migrations/20261021105752_seller_payuni_bind_retry_review.sql`，它 drop 再建 `pub_review_items`），`make sqlc`。
- 待審核的其他讀者（F3，附錄 B #1）：`AdminStores` 加 `sqlc.arg(kyc_awaiting_review)::bool` 與 `now`，條件 `(NOT kyc_awaiting_review OR EXISTS (SELECT 1 FROM seller.review_kyc_application(now) r WHERE r.store_id = st.store_id))`；`profile/admin.go:134-135` 的 `filterableKYCStatuses` 刪 SUBMITTED。`AdminStoreStats` 的 `kyc_submitted` 改成 `kyc_awaiting_review`：`count(*) FILTER (WHERE EXISTS (SELECT 1 FROM seller.review_kyc_application(sqlc.arg(now)) r WHERE r.store_id = st.store_id))`，`StoreStats.KYCSubmitted` 改名 `KYCAwaitingReview`。`AdminKYCApplications` 的狀態篩選是儲存的事實、admin 沒有呼叫端（C25），不改；觸發是 admin 開始用它當審核佇列。
- `model.PayUniPhaseOf(reg PayUniRegistration) KYCFlowStatus`（F11）：5a 已抽出（`model/payuni.go:505-526`，C28），本刀只在 admin 的讀取呼叫它，不改函式。
- `stint.LockOpenAccountOf`（`stint.go:57-67`）刪除（C22）。
- 讀取：`GetKYCApplication` 的 admin view（`kyc/admin.go` 的 `adminViewWith`）載入店家後算三組原因；核准、退回、重新查詢的回應也帶（同一個 view）。`GetPayUniRegistration`（`payuni.Service.Registration`，`intent.go:218-232`）與 `RebindPayUni` 的回應同理，另帶 `PayUniPhaseOf`：`withSellerType`（`:235-251`）本來就讀註冊所根據的申請，改成一併回傳它的狀態（同一次讀取，不另查），店家以 `stint.Load` 載入。KYC 的三組原因要停權，`adminViewWith` 以 `stint.Load` 載入店家（含 `Sanctions`）。食品業者登錄的 `review` 讀取帶兩組原因。`StoreWithActive`（`M/internal/measure/list.go:207-251`）已有店家，`StoreMeasures` 加 `CreateMeasure []StoreActionUnavailableReason`。`AdminStores` 的列已有條件與停權（C12），handler 直接呼叫 `item.Store.ActionUnavailableReasons(StoreActionDelegate)`。
- 根套件轉出型別與 `StoreActionUnavailableText`（handler 不 import `internal/`，同 `seller/governance.go:108-109`）。
- `text/zh-TW.json` 的 `staff` 段新增 §5 的 key（lower_snake 點分路徑，L′ §2.2）；5a 的 `payuni.rebind_unavailable.application_not_approved`（`:443-446`）改名成 `store_action.unavailable.kyc_not_approved.rebind_payuni`，message 不變、description 改寫成原因與動作；`make uitext` 重新生成 `text_gen.go`（`text.PayuniRebindUnavailableApplicationNotApproved` 消失）。5a 的兩個整合測試以子字串比對這一句（`payuni_state_integration_test.go:113`、`review_policy_integration_test.go:771`），字不變所以照舊綠。

### 2.2 delegation 模組（F7）

- `DL/delegation.go` `storeStateOf`：只有 seller 回 NotFound 時 `Found=false`；其餘 `reasons := st.ActionUnavailableReasons(seller.StoreActionDelegate)`，回 `Found=true`、`Closed: st.Status == seller.Closed`、`Operable: len(reasons) == 0`，不可營運時帶 `Unavailable: seller.StoreActionUnavailableText(reasons[0], seller.StoreActionDelegate).Text`。註解刪「不存在與已關閉都是找不到（9265，與舊 seller-service 關店即軟刪除一致）」。
- `DL/internal/model/delegation_rules.go`：`StoreState` 加 `Closed bool`、`Unavailable string`，`Found` 的註解改成「店家不存在」；新增 `DenialStoreClosed`（註解「店家已關閉」），錯誤分類與 `DenialStoreNotOperable` 同為 failed_precondition（`:196-199`），根套件轉出（`delegation.go:258-260` 同列）。`DeniedLabel` 刪 `DenialStoreNotOperable` 的 case；店家狀態的兩種拒絕不經 `DeniedLabel`。
- `session.go` preflight：`!st.Found` → `DenialStoreNotDelegable`（只剩不存在）；之後照舊讀授權；`!st.Operable` 時 `st.Unavailable` 空 → 內部錯誤（不寫通用句，同 §2.1 的規則），否則 `Closed` ? `DenialStoreClosed` : `DenialStoreNotOperable`，`DeniedLabel` 是 `st.Unavailable`。已關閉的店家因此有 `TargetKnown`（顯示店家名稱、授權的集合），這是店家存在的事實，legacy 的 `PreflightData` 形狀不變。
- `authorization.go:234-240` 開通授權：`!st.Found` 照舊 `DenialStoreNotDelegable`；`st.Closed` → `Deny(DenialStoreClosed, st.Unavailable)`（N2，不新增句子）。
- legacy `internal/legacy/delegationservice/server.go:49-66`：`bizCodes` 加 `DenialStoreClosed: 9265`（`TestEveryDenialHasABusinessCode` 會要求）。舊碼的對應只在這一層，A10 刪 legacy 時跟著刪。
- delegation `AGENTS.md`（`:44`）／`CARRYOVER.md`（`:23`）：改成「已關閉是 `DenialStoreClosed`，9265 只在 legacy `bizCodes`；店家狀態的原因與句子來自 seller 的 `StoreActionDelegate`」；seller `CARRYOVER.md:282` 的「已關閉與不存在一樣是 9265」同改。

### 2.3 handler `H/`

- `admin.go` `ListStores`：每列 `DelegationUnavailabilities`；請求的 `kyc_awaiting_review` 轉進 `StoreListInput`（page token 綁篩選，同既有欄位）；`GetStore`：`CanCreateMeasure`、`CreateMeasureUnavailabilities`；`GetStoreStats`（`:182`）：`KycAwaitingReviewStores: stats.KYCAwaitingReview`。
- `admin_kyc.go`：`toKYCApplication` 加三組欄位；`toAdminPayUni`（`:462-486`）加 `CanRebind`、`RebindUnavailabilities`、`FlowStatus`，F12 刪三個欄位的賦值（`:472-473`，含 `rpc.EnumToProto[pb.PayUniOutcome]`）；`kyc_test.go:468` 的 `"PayUniOutcome"` 一列（`EnumMismatches`）與 `admin_kyc_test.go:448` 的 `GetOutcome()` 斷言刪除（C30）。
- `foodreg.go:108`：加兩組原因。
- 共用 `toStoreActionUnavailabilities(reasons, action)`：`rpc.LocalizedMessage(seller.StoreActionUnavailableText(r, action))`（同 `governance_measures.go:68-77`）。

## 3. admin

### 3.1 商家清單 `MD/page.tsx`（F3、F7、F9）

- `A/lib/store-search.ts:60-61` 的 `pending` 分頁改成 `{ kycAwaitingReview: true }`；`filterFields` 的 `Pick` 加 `kycAwaitingReview`。兩張統計卡（`A/app/admin/page.tsx:152`、`MD/page.tsx:232`）讀 `kycAwaitingReviewStores`，標籤字不變（F3）。

- 刪 `canStartDelegatedOperation`、`lifecycleBlockedLabel`、`delegationBlockedLabel` 的 import。
- `delegationBlockedReason = m.delegationUnavailabilities.length > 0 ? unavailableLabels(m.delegationUnavailabilities)[0] : undefined`；`MerchantOperationEntry` 照舊（入口永遠佔位、停用按鈕寫原因，設計書 56 §6.1）。
- `store-row.ts`：刪 `lifecycleBlockedLabel`；`policy.ts`：刪 `delegationBlockedLabel` 與它的說明註解（`:40-63`）。

### 3.2 商家詳情 `MD/[id]/`

- `store-detail.ts`：刪 `StoreActions`、`storeActions`、`REVIEW_BLOCKED_*`、`reviewBlockedReason`。`page.tsx:930` 的 `actions.closed`（徽章與關店資訊，是狀態的顯示，不是可用性）改成 `store.status === StoreStatus.CLOSED`。
- 建立措施兩個鍵（`page.tsx:1028,1041`）：`detail.canCreateMeasure`。
- 結束（F4）：`ActiveMeasureCards`／`ActiveMeasureCard` 拿掉 `canEndMeasure` prop，只看 `measure.canEnd`（`ActiveMeasuresCard.tsx:27,42,93,153-168`）。
- 刪除帳號（F6）：照 Will 的決定；建議版是 `accountEmail.state === "ready"`。
- KYC（F3、F8）：`reviewActions(application)` 只收申請，回 `{canApprove: application.canApprove && 有 id 與 etag, canReject: …, canRequery: …}`（id、etag 的檢查是資料完整性，不是業務規則，留著）。原因提示（`page.tsx:986-991` 的 `kyc-review-blocked`）：`approve_unavailabilities` 的第一個原因不是結構性時，顯示 `unavailableLabels(approve, reject)`（依序合併、相同的句子只列一次；店家不是營運中時三個動作同一句，只出現一次，§5 #N5、#N6），每句一行。
- `RejectKycDialog`（C17）：`blocked` 改成收 `reasons: string[]`（`unavailableLabels(reject)`）；橫幅是伺服器的句子（每句本身以「。」結尾）後接 client 的固定句（§5 #N16），兩句分開排，client 不加標點。
- PayUni 卡（F5、F11）：`canRebind` 刪除，呼叫端讀 `registration.canRebind`；`PayUniCards.tsx:130-134` 改成第一個原因不是結構性時顯示它的 `message.message`（N10、N11、N17，與 v1.3 的 N18：賣家改回草稿時「重新綁定」鍵消失、改顯示那一句）；刪 `PAYUNI_REONBOARDING_TEXT`。徽章 `flowBadge(registration.flowStatus)`，刪 `payUniBadge` 的後備表、`PAYUNI_FLOWS`、`PayUniOutcome` 的 import（`kyc-application.ts:25`）與 `flow` prop（`GetStore.kyc.flow_status` 不再傳進卡片）。問題文字直接顯示 `registration.statusCodes?.reason`（trim 後非空才顯示），刪 `payUniProblemText`。綁定失敗句與「需補資料」不變（F11）。

### 3.3 食品業者登錄詳情 `A/app/admin/food-registrations/[id]/page.tsx`（F13）

- `blocked` 改成：`approveUnavailabilities` 的第一個原因不是結構性時 `unavailableLabels(approve, reject)`；刪 `approveBlockedText`、`APPROVE_BLOCKED_KYC_TEXT`。

### 3.4 `A/lib/unavailability.ts`（新，搬移）

- `GENERIC_UNAVAILABLE_LABEL`、`unavailableLabels(...lists: ReadonlyArray<readonly { message?: LocalizedMessage }[]>)`、`firstIsStructural(list)`（F8 的結構性集合，只在這裡）；`measure-actions.ts` 改 import，行為不變。

## 4. gateway

不改。preflight 的 REST 形狀不變，只有 `denied_label` 的文字內容改變（F7）；`gateway-json-contract-check.py` 不比對字串值［量測，§0.4 C24］，照跑確認。

---

## 5. 文案表（A＝已核准、C＝跟隨契約或 SSOT、E＝沿用、**N＝新寫或新顯示，要 Will 核可**）

用語：「商家認證」（術語表:77）、「代行」（術語表:53）、「停權」（:36）。staff 句子有主詞、不含畫面名稱（`P/measure.proto:182`）。
**標點規則（一條）**：伺服器的句子一律是完整句、以「。」結尾，包括清單按鈕上的代行原因；client 不加、不刪標點，要接自己的句子時分開排（附錄 A M5）。所以 E 類句子只多一個句號，其餘不變。

| # | 位置（key 暫定，`store_action.unavailable.*`） | 改前 → 改後 | 類 |
|---|---|---|---|
| N1 | `store_closed.create_measure`（GetStore、CreateMeasure 錯誤、案件與通報的原因） | 錯誤「店家已關閉」，畫面隱藏不說明；案件「店家已關閉，不能再從這個案件建立警告、停權或風險措施。」；通報「店家已關閉，不能再引用它的通報建立警告或停權。」 → 「店家已關閉，不能建立警告、停權或風險措施。」 | N（三句合一，L6） |
| N2 | `store_closed.delegate`（清單按鈕、preflight） | 「已關閉，無法代行」／preflight「此店家目前不可代行」 → 「店家已關閉，無法代行。」 | N |
| N3 | `store_pending_deletion.delegate` | 「等待刪除，無法代行」／preflight「此店家已停權或關閉，無法代行」 → 「店家等待刪除，無法代行。」 | N |
| N4 | `account_suspended.delegate` | 「停權中，無法代行」（設計書 56）／preflight「此店家已停權或關閉，無法代行」 → 「帳號停權中，無法代行。」 | N（改設計書的字，文末修訂） |
| N5 | `store_closed.{approve,reject,requery}_kyc`（同一句） | 「店家已關閉，不能核准或退回商家認證。」 → 「店家已關閉，這份商家認證申請暫停審核，店家恢復營業後才能審核或重新查詢。」 | N（F3 A 的措辭） |
| N6 | `store_pending_deletion.{approve,reject,requery}_kyc`（同一句） | 「店家等待刪除，不能核准商家認證（仍可退回）。」／錯誤「店家不在營業中，不能核准」 → 「店家等待刪除，這份商家認證申請不再審核。」 | N（F3 A 的措辭） |
| N7 | `account_suspended.approve_kyc` | 「帳號停權中，不能核准商家認證（仍可退回）；停權解除之後才能核准。」／錯誤「申請人的帳號停權中，不能核准」 → 「帳號停權中，停權解除之後才能核准商家認證。」 | A（「仍可退回」由畫面上的退回鍵表達；v1.2 停權改成暫停審核之後字仍成立：核准要等停權解除、退回照舊可用，不改） |
| N8 | `kyc_not_submitted.{approve,reject}_kyc` | 錯誤「只有審核中的申請能核准或退回」 → 「只有審核中的申請能核准或退回。」 | N（字沿用 `kyc/admin.go:397`，但現在會顯示在退回對話框的橫幅，C17） |
| E2 | `kyc_not_submitted.requery_kyc`、`kyc_not_business.requery_kyc` | 「只有審核中的申請能重新查詢登記資料」、「只有公司型的申請有登記資料可以查詢」 → 加「。」 | E（`registrycheck.go:137,140`；結構性，畫面不顯示，只在錯誤） |
| N10 | `payuni_merchant_terminal.rebind_payuni`（PayUni 卡、Rebind 錯誤） | 卡片「PayUni 端的商家已是終止狀態，這家店重送也綁不上；賣家需要先關閉店家，再重新入駐。」／錯誤「PayUni 端的商店已是終止狀態，無法重新綁定，請關店後重新入駐」 → 「PayUni 端的商店已是終止狀態，這家店無法再綁定；賣家要重新入駐才能收款。」 | N（兩句合一；不叫賣家關店，已關閉的店家也成立，L7） |
| N11 | `store_closed.rebind_payuni` | 錯誤「店家已關閉」（not_found），卡片什麼都不說 → 「店家已關閉，不能重新綁定 PayUni。」 | N（現在會顯示在已關閉店家的卡片上） |
| N17 | `store_pending_deletion.rebind_payuni` | （沒有：今天等待刪除可以重新綁定）→ 「店家等待刪除，不能重新綁定 PayUni。」 | A（Will 2026-10-21，F5，附錄 B #5） |
| N18 | `kyc_not_approved.rebind_payuni`（PayUni 卡、Rebind 錯誤） | 錯誤「賣家正在修改商家認證，重新核准後會自動重新綁定。」（5a #38，Will 2026-10-21 核可，只在按下「重新綁定」之後以錯誤出現），卡片照舊顯示「重新綁定」鍵 → 字不變，**改成直接顯示在卡片上、取代「重新綁定」鍵** | **N（v1.3：字沿用，新顯示；待 Will 核可）** |
| E6 | `payuni_bound.rebind_payuni` | 「只有綁定失敗的註冊能重新綁定」 → 加「。」 | E（`intent.go:185`；結構性，畫面不顯示） |
| E8 | `food_registration_not_pending.{approve,reject}_food_registration` | 「這筆食品業者登錄已不在待審」 → 加「。」 | E（`foodreg.go:366`；結構性，畫面不顯示） |
| N13 | `store_closed.{approve,reject}_food_registration` | 錯誤「店家不在營業中，不能裁定」，畫面不說明 → 「店家已關閉，不能裁定食品業者登錄。」 | N |
| N14 | `store_pending_deletion.{approve,reject}_food_registration` | 同上 → 「店家等待刪除，不能裁定食品業者登錄。」 | N |
| N15 | `kyc_not_approved.approve_food_registration` | client「店家還沒通過商家認證，無法核准」／錯誤「店家的實名認證還沒有核准，不能核准食品業者登錄」 → 「店家的商家認證還沒核准，不能核准食品業者登錄。」 | N（兩句合一，術語改「商家認證」） |
| E3 | `termination.unavailable.store_closed` | 「店家已關閉」（not_found → failed_precondition） → 加「。」 | E |
| E4 | `termination.unavailable.store_superseded` | 「店家已關閉，帳號有較新的店家」 → 加「。」 | E |
| E5 | `measure.end_unavailable` | 「只有生效中、依據風險類型的措施可以結束」 → 加「。」 | E（移進 `text/`） |
| N16 | `RejectKycDialog` 橫幅 | 「這份申請目前不能退回（已經不是待審核狀態，或店家已關閉）。你輸入的內容還在，可以關閉視窗後確認申請的狀態。」 → 「{伺服器的句子}」＋「你輸入的內容還在，可以關閉視窗後確認申請的狀態。」 | N（前半由伺服器給，後半沿用） |
| C1 | `MD/[id]` 詳情頁的代行入口 | 被拒時顯示 preflight 的句子（N2、N3、N4 跟著變） | C |
| C2 | PayUni 卡的徽章 | client 後備表 → 目錄 `seller.KYCFlowStatus` 的 staff 名詞（字相同）；唯一的差異：審核結果是 UNRECOGNIZED 時「狀態待確認」→「PayUni 審核中」 | C（F11） |
| E7 | PayUni 卡的綁定失敗句、「需補資料：」、問題文字 | 不變（問題文字的來源改成直接讀 `status_codes.reason`，顯示的字相同） | E（F11） |

核可狀態：v1.1 的 N1–N8、N10–N16 Will 已核可（決定紀錄）；v1.2 刪 N12（`PAYUNI_BIND_IN_PROGRESS` 不是失敗原因、沒有讀者，附錄 B #6），新增 N17（Will 已核可）。N7 的字不變（上表）。v1.3 新增 N18（字是 5a 已核可的 #38，新的是顯示位置），**Will 2026-10-21 核可照原句**（文末「待 Will 決定」）；沒有待核可的文案。F4、F6、F13 與 N11 的出現與否已隨決定紀錄定案；N11、N17、N18 會顯示在 PayUni 卡上（第一個原因不是結構性）。

---

## 6. 測試

### 6.1 backend

- `M/internal/model/store_action_test.go`（新）：§2.1 的各條（`blocking` 有句子、`idempotent` 與表外回零值、各動作的第一個原因、DELEGATE 與 `CapStorefrontWrite` 等價，一個突變）。`PayUniPhaseOf` 與 `DeriveKYCFlow` 的等價 5a 已寫成 `TestPayUniPhaseOf`（`model/payuni_state_test.go:248-283`，C28），本刀不另寫。REBIND 的 `KYC_NOT_APPROVED`：FAILED × 申請 DRAFT／SUBMITTED／REJECTED／CANCELLED 都有、APPROVED 沒有；BOUND × DRAFT 的第一個是 `PAYUNI_BOUND`（**突變**：把 `KYC_NOT_APPROVED` 移到 `PAYUNI_BIND_IN_PROGRESS` 前面時要失敗）。
- 錯誤形狀改變（NotFound → failed_precondition）會轉紅、要改斷言的既有測試（附錄 A M4，[量測] 逐行讀過）：`M/measure_integration_test.go:307`（已關閉的 stint 建立三種措施）、`:528`（沒有 request_id 的重送遇到已關閉）；`M/seller_integration_test.go:812`（停權已關閉的店）；`M/termination_integration_test.go:495`（預告終止建在已關閉的店）、`:509`（舊 stint 有較新的店家）；`M/termination_availability_integration_test.go:112`（「不可用時要回 NotFound」改成 failed_precondition，並斷言訊息等於句子）。`M/cases_dataset_integration_test.go`：`blockedProperly` 刪 `storeClosedNotFound` 參數與 :976-983 的放行例外，:607-608、:778、:867-868 的呼叫與註解跟著改（兩個動作「建立警告」「當認定的來源」都走 `CreateMeasure`，`:627-630`）。`M/payuni_integration_test.go` 若有已關閉重新綁定的 NotFound 斷言同改（實作時 `rg -n "KindNotFound" M/payuni_integration_test.go` 逐條看；`:661` 是店家不存在，不改）。`H/kyc_test.go:468`、`H/admin_kyc_test.go:448` 隨 `PayUniOutcome` 刪除（§2.3）。
- `M/` 整合測試（新增或擴充）：
  - 已關閉的店建立警告回 `failed_precondition`，訊息等於 `GetStore.create_measure_unavailabilities[0].message`，也等於案件 `CaseBlockStoreClosed` 與通報 `ReportBlockStoreClosed` 的句子（**同一句的斷言**，照 L′ 對 appeal 的寫法）。
  - F3：送件 → 終止合作生效關店：`GetKYCApplication` 的三組原因都是 `STORE_CLOSED`；核准、退回、重新查詢回 `failed_precondition` 且訊息相同；`review_kyc_application` 與 `decide_deadline` 沒有這筆；撤銷終止恢復營業之後回到待辦，開始時間是恢復時間；等待刪除同樣不在待辦。食品業者登錄的恢復營業期限同一條（`review_policy_integration_test.go` 的資料集擴充，`:364`、`:1207` 附近；5a 已在同一個 `reviewWorld` 加了 PayUni 綁定的夾具，`:196-199`、`:415-450`）。
  - F3 停權（附錄 B #3）：送件 → 停權生效：`approve_unavailabilities` 是 `ACCOUNT_SUSPENDED`、`reject_unavailabilities` 與 `requery_registration_unavailabilities` 為空；退回成功；不在待辦、`decide_deadline` 為空；停權到期與停權撤銷兩種結束各一例，回到待辦且開始時間是結束時間（**突變**：開始時間不取停權結束時要失敗）；停權在送件前就結束的，開始時間仍是送出。`account_sanction_lifted_at` 與 `effective_measures` 的邊界一致（照 `TestEffectiveMeasuresAndStateAgreeAtTheBoundaries` 的寫法：`ends_at`、`revoked_at` 等於 at 的瞬間、預告期間內結束的措施）。
  - 待審核的成員只有一份（附錄 B #1）：在同一組資料集（營運中、已關閉、等待刪除、停權、恢復營業、身分命中各一）上，`review_kyc_application(now)` 的店家集合 ＝ `AdminStores{kyc_awaiting_review}` 的店家集合，數量 ＝ `AdminStoreStats.kyc_awaiting_review` ＝ 以 Go 對每筆 SUBMITTED 申請算 `APPROVE_KYC` 原因清單為空的店家集合（**突變**：統計改回 `k.status = 'SUBMITTED'` 時要失敗）。`TestAdminStoreStats`（`M/admin_integration_test.go:356-382`）與 `TestAdminStoresFilterByKYCStatusAndShowTheFlow`（`:313`）改用新欄位，SUBMITTED 的 `kyc_status` 篩選改斷言 InvalidInput。
  - 核准在等待刪除、停權時的原因與錯誤訊息相同。
  - 重新綁定：已關閉 → `failed_precondition`（不再是 not_found）；等待刪除 → `failed_precondition`、訊息是 N17，`can_rebind` 為 false（附錄 B #5）；needs_reonboarding 而且已關閉 → 第一句是 N10；BOUND → failed_precondition；PENDING → 回目前的註冊、不排 job（`TestRebindOnlyFromFailed`，`:632` 照跑），同時 `rebind_unavailabilities` 第一個是 `PAYUNI_BIND_IN_PROGRESS`、`can_rebind` 為 false；綁定失敗而賣家把申請改回草稿（5a 的可達路徑，`payuni_state_integration_test.go:113` 附近）→ `GetPayUniRegistration` 的 `rebind_unavailabilities` 是 `KYC_NOT_APPROVED`、message 等於 `RebindPayUni` 錯誤的訊息（N18，**同一句的斷言**）、`can_rebind` 為 false。
  - 待辦與重新綁定的成員（5a §7.1 交給後合併的一刀）：在 `review_policy_integration_test.go` 的同一組資料集上，`review_payuni_bind_retry(at)` 的每一列，其店家的 `RebindUnavailableReasons` 都是空的（單向：其他綁定失敗的碼刻意不進待辦，5a F16）；**突變**：把函式的 `k.status = 'APPROVED'` 拿掉時要失敗（夾具 `bindRetryDraftStore` 會進待辦而原因是 `KYC_NOT_APPROVED`）。
  - 食品業者登錄：KYC 沒核准、已關閉、等待刪除、不在待審的原因與 `DecideFoodRegistration` 的錯誤訊息相同（`foodreg_integration_test.go:479` 附近的關店案例擴充）。
- `DL/`：`delegation_integration_scenarios_test.go:282-295` 補斷言已關閉的拒絕是 `DenialStoreClosed`、等待刪除與停權是 `DenialStoreNotOperable`，`DeniedLabel` 依店家狀態是 N2／N3／N4，已關閉時 `TargetKnown`；開通授權對已關閉回 `DenialStoreClosed`、訊息 N2，對不存在仍是 `DenialStoreNotDelegable`；`StoreState` 不可營運而句子空時回內部錯誤（以假的 `Seller` 驗）。`internal/legacy/delegationservice/server_integration_test.go:364-388` 的 9265／9272 不變（舊碼的對應在 `bizCodes`）、補 `denied_label` 的斷言；`TestEveryDenialHasABusinessCode` 涵蓋 `DenialStoreClosed`。
- `H/`：`admin_integration_test.go` 補 `ListStores` 的 `delegation_unavailabilities` 與 `kyc_awaiting_review` 篩選、`GetStoreStats` 的 `kyc_awaiting_review_stores`（`:437` 改欄位）、`GetStore` 的 `create_measure_unavailabilities`、食品業者登錄兩組原因（`:859` 附近）的 wire 斷言；`admin_kyc_test.go:828` `TestGetKYCApplicationCarriesServerComputedRules` 擴充三組原因、`can_rebind`、`flow_status`；`governance_cases_test.go:108-116` 的已關閉句子改成 N1。

### 6.2 admin（`admin/tests`）

- `merchant-store-detail.test.ts`：刪 `:37-42` 的 import、`:70-153`（四種狀態的 `storeActions`、`reviewBlockedReason` 三句與狀態不明）；`:257` 的原始碼斷言改成「建立鍵看 `canCreateMeasure`、終止看 `terminationAvailability`」；`:361-371` 改成結束只看 `measure.canEnd`；`:437-441` 改成核准、退回讀 `application.canApprove`／`canReject`。新增：伺服器給 `STORE_CLOSED` 時提示只出現一句（N5，三個動作去重）；第一個原因是 `KYC_NOT_SUBMITTED` 時沒有提示（**突變**：client 改成只要清單非空就顯示時要失敗）。
- `merchant-store-row.test.ts:13,154-161`：刪。
- `merchant-store-list.test.ts`：`filterFields("pending")` 是 `{ kycAwaitingReview: true }`、不再送 `kycStatus`；統計卡讀 `kycAwaitingReviewStores`（F3）。
- `unavailability.test.ts`（新）：結構性集合恰好是契約註解標明「表示沒有入口」的五個值（逐值列出，對照 §1）；第一個是 `PAYUNI_BIND_IN_PROGRESS`（沒有 message）時不顯示任何句子，也不顯示通用文字。
- `delegated-operation-entry-gating.test.ts`：刪 `:5` import 與 `:297-341`（五句的鑑別力測試，函式已刪）；`:343-372` 的原始碼 regex 改成斷言 `blockedReason` 來自 `delegationUnavailabilities`（例如 `/delegationBlockedReason\s*=\s*m\.delegationUnavailabilities\.length/u`），並斷言 `merchants/page.tsx` 不再出現 `StoreStatus.`、`isSuspended` 這類本地判準（**突變**：加回 `rowState(m).isActive &&` 時要失敗）；`:151-192` 的 fixture 字改成 N4。
- `delegated-operation-preflight-denial.test.ts`：fixture 是伺服器句子，換成 N4（只改資料）。
- `merchant-kyc-application.test.ts`：`:234-279` 的 `reviewActions` 改成讀申請上的旗標；`:1015-1060` 的 `canRebind` 改成 `can_rebind` 與原因句的顯示（BOUND 而且 needs_reonboarding 仍顯示 N10；第一個是 `PAYUNI_BOUND`、`PAYUNI_BIND_IN_PROGRESS` 時不顯示句子：**突變**：結構性集合漏掉 `PAYUNI_BIND_IN_PROGRESS` 時要失敗）；第一個原因是 `KYC_NOT_APPROVED` 時沒有「重新綁定」鍵、顯示伺服器的句子（N18）；`payUniBadge` 的測試改成只斷言 `flowBadge(flowStatus)`，刪後備表的案例與 `PayUniOutcome` 的 fixture（`:27`、`:989`、`:1123`、`:1130`）；`payUniProblemText` 的測試刪，改成卡片顯示 `statusCodes.reason`；`:1100,1177` 跟著改。
- `food-registration.test.ts:16,108-116`：刪 `approveBlockedText` 的案例；新增詳情頁在第一個原因是 `KYC_NOT_APPROVED` 時顯示伺服器句、是 `FOOD_REGISTRATION_NOT_PENDING` 時不顯示。
- `merchant-active-measures-card.test.ts:21,51-57`、`merchant-create-termination-dialog.test.ts:736`：拿掉 `canEndMeasure` prop。
- `measure-actions` 的既有測試照跑（`unavailableLabels` 搬家後行為不變）。
- 讀 `canCreateMeasure` 的其他頁（`authority-order-pages.test.ts`、`authority-actions.test.ts`、`case-detail-page.test.ts`、`report-pages-wiring.test.ts`）讀的是別的 message 的欄位，不受影響；`case-detail-page.test.ts`、`report-pages-wiring.test.ts` 若有已關閉句子的 fixture 換成 N1。

## 7. Commit 計畫

| # | 內容 | 審查 |
|---|---|---|
| 5b-0 | 契約（§1）與三個生成端、`make -C backend uitext`；seller `store_action.go`（規則表，含 5a `Rebind` 申請檢查收成的 `KYC_NOT_APPROVED`）、`text/`（含 #38 的 key 改名）、前置檢查與讀取、objects.sql 與 migration（F3，含停權）、`AdminStores`／`AdminStoreStats` 的待審核成員、刪 `LockOpenAccountOf`、handler；delegation 的 `storeStateOf`、`DenialStoreClosed`、preflight 與開通的句子，legacy `bizCodes`；刪 proto `PayUniOutcome` 與它在 handler、測試的對照（§1、§2.3、§9）；§6.1 | code-review |
| 5b-1 | admin 全部（§3）與 §6.2 | code-review |
| 5b-2 | 文件：計畫 10e 的 5b 完成紀錄與 :384 待辦勾除；seller `CARRYOVER.md` 沿用盤點（`NotFound("店家已關閉")` 在 admin 路徑改 failed_precondition、審核在店家不是營運中時暫停、PayUni 重新綁定的語意，含 5a F17 的申請檢查改成原因）、seller `AGENTS.md` 的 `review_kyc_application` 成員條件（營運中、沒有停權、對應 `APPROVE_KYC` 清單為空）、「措施生效中」清單加 `account_sanction_lifted_at`、後台清單篩選與統計改成 `kyc_awaiting_review`；seller `CARRYOVER.md:282`、`:290`（統計口徑）；delegation `AGENTS.md:44`／`CARRYOVER.md:23`（F7：已關閉是 `DenialStoreClosed`，9265 只在 legacy）；設計書 56 的 :2398、:2459、:5813、:5858 與文末修訂（N4）；db D4-13 的修訂（F4、F6 照 Will 的決定） | 併入完成前的 design-review |

design-review 兩次：Will 決定後、派 5b-0 之前一次；5b-2 之後看整刀。實作派 Sonnet subagent，主 session 寫派工、驗證與審查（同 4b）。

## 8. 驗證命令（對照 `a6f7b3815` 的 `.github/workflows/`；5a 沒有改 workflow，`1fc535d87` 相同［量測：`git diff --name-only a6f7b3815 HEAD` 沒有 `.github/`］）

- api（`api-contract.yml`）：`buf lint api`、`buf breaking`、三個生成端的 drift；`@hawkr/api` 的 `tsc --noEmit`、`biome check`、`test`；`packages/hawkr_api` 的 `dart analyze`、`dart format`、`dart test`。
- 單體（`backend-monolith.yml`）：golangci-lint 兩個 step（一般與 integration tag，`:56-77`）；gofmt、`go vet`（含 `-tags integration`）、`go test -race ./...`、`go test -race -timeout 15m -tags integration ./...`（帶 `DOCKER_HOST`）（`:80-111`）；Connect 生成物 drift `make connect`（`:122-129`）；sqlc 生成物 drift `make sqlc`（`:133-139`，F3 改 objects.sql 與查詢時必跑）；`GOWORK=off go mod tidy` 無 diff（`:166-`）。`backend-proto-generated-diff.yml` 與 `backend-sqlc-generated-diff.yml` 管的是舊的 `backend/api/proto`、`backend/services/*`，本刀不觸發（附錄 A L3）。
- schema：新 migration 由 seller 的 `migration_integration_test.go` 在上一條的 integration test 裡驗證；實作時依 seller `AGENTS.md` 的 schema 流程產生並更新 atlas.sum。
- 文案：`ui-text-generated-diff.yml`（`make -C backend uitext` 後 `git diff --exit-code`）、`ui-text-guard.yml`、`scripts/check-enum-text-cross.sh`。
- admin（`admin-ci.yml`）：`pnpm lint`、`pnpm test`、`pnpm build`（在 `admin/`）。
- 改 `api/**` 觸發的 `website-ci.yml`、`ts-packages.yml`；改 `packages/hawkr_api/**` 觸發的 `buyer-ci.yml`、`seller-ci.yml`（同 4b §8）。
- `backend-gateway-json-contract.yml`：`python3 backend/scripts/gateway-json-contract-check.py`（預期無變化，§4）。
- 派工前逐一打開上面的 workflow 核對實際命令與行號。

## 9. 沒有驗證的部分與待決問題

- 業界基準（F3 的暫停、F4 的帳號範圍、F6 的刪除請求權）本次沒有網路，全部未查證。
- F3 的待審核讀者盤點（v1.1 以 `rg "'SUBMITTED'"` 只抓到字面值，漏了參數化的過濾）改用 `rg -n "kyc_status|KYC_STATUS_SUBMITTED|kycSubmitted"`：讀者是待辦、`decide_deadline`、商家清單「待審核」分頁、兩張統計卡，四者在 v1.2 同讀 `review_kyc_application`（C25、§2.1）；`ListKYCApplications` 沒有 admin 呼叫端，不改。不會恢復的店家的 SUBMITTED 不出現在任何「待審核」，由 §6.1 的資料集測試確認。
- §6.1 列的 NotFound 斷言之外，`M/payuni_integration_test.go` 是否還有已關閉重新綁定的斷言，留給實作時逐條看（已寫明命令）。
- **與 5a 的界線（v1.3：5a 已合併，`2bba5d0b4`、`1fc535d87`；以下是 HEAD 上的事實與本刀要做的事）**：
  1. **起點**：5b 從 `1fc535d87` 開工，不需要 rebase；5a 的契約、生成物與 `make uitext` 的結果已在 HEAD。5b-0 之後照 §8 重跑全部生成物 drift，並跑 5a 的 `TestSellerPayUniOf`、`TestSellerPayUniInvariants`、`TestPayUniPhaseOf`（`M/internal/model/payuni_state_test.go`）與 `payuni_state_integration_test.go`［量測：測試名以 `rg -n "^func Test"` 列出；v1.2 寫的 `TestPayUniStateOf` 不存在］。
  2. **本刀刪 proto `PayUniOutcome`**：5a 刪了賣家端 `PayUniRegistration`，F12 刪 admin 的 `outcome` 之後沒有使用者（C30），5b-0 刪 enum、生成碼、`H/admin_kyc.go:472` 的轉換與兩個 handler 測試的對照；Go 的 `model.PayUniOutcome` 留著（DB CHECK、`PayUniPhaseOf` 的輸入）。`PayUniBindStatus` 仍有 admin `bind_status` 與 `ListStoresRequest.payuni_bind_status`，保留。
  3. **PayUni 階段只判斷一次，經 `PayUniPhaseOf`**：5a 已抽出並以測試鎖住（C28），本刀沿用、不改；admin 的 `flow_status` 直接呼叫它（F11）。兩刀不共用 enum（受眾不同，5a F12）。
  4. **`Rebind` 的申請檢查（5a F17）收成原因**：`KYC_NOT_APPROVED` 排在 `REBIND_PAYUNI` 的最後，句子沿用 #38（§5 #N18），刪 `ErrRebindApplicationNotApproved` 與舊 key（F5、§2.1）；不新增 5a §7.1 舉例的 `KYC_RESUBMITTING`、也不照它「排在 `PAYUNI_BIND_IN_PROGRESS` 之前」（理由在 F5）。
  5. admin 的 PayUni 卡（徽章後備表、問題文字、綁定失敗句、重新綁定）全在本刀 F5、F11；5a 沒有碰 admin 的卡片（`admin/src` 只改了 `queue/queue-item.ts`）［量測］。5a 刪的 `PayUniProblem.remedy` 與 `KYCResubmitUnavailableReason.CANCELLED` 本刀都不讀；5a 給 `problem.reason` 加的修剪不改變 F11 的結論。
  6. **後台待辦 `PAYUNI_BIND_RETRY`（5a F16）**：已存在（C29），屬 5a，本刀不改它的條件、種類、期限與畫面。v1.2 寫的「成員應該與 `REBIND_PAYUNI` 清單為空同一個定義」不成立：5a 刻意只收重試用完的 `PAYUNI_UNAVAILABLE`，其他綁定失敗由客服接手（Will 2026-10-21 核可 5a F16），所以兩者是單向關係「待辦有列 ⇒ 重新綁定此刻可做」，照 5a §7.1 由本刀在 §6.1 加這條資料集斷言。函式的條件（FAILED、店家 `ACTIVE`、申請 `APPROVED`）對到表上的 `STORE_CLOSED`、`STORE_PENDING_DELETION`、`PAYUNI_BIND_IN_PROGRESS`、`PAYUNI_BOUND`、`KYC_NOT_APPROVED` 都不成立；`PAYUNI_MERCHANT_TERMINAL` 在碼是 `PAYUNI_UNAVAILABLE` 時只剩「`mer_status` 是終止狀態」一條路（`stint/flow.go:30`），FAILED 的註冊沒有綁定過、應該沒有 `mer_status`［推論，沒有跑；斷言會抓到反例，抓到時改函式的條件對齊 Go，不放寬斷言］。

## 待 Will 決定（v1.3）——Will 2026-10-21 核可 N18 照原句

1. **§5 #N18：admin PayUni 卡在「賣家正在修改商家認證」時的顯示**。今天（5a 之後）綁定失敗而賣家把申請改回草稿時，卡片照舊顯示「重新綁定」鍵，按下去才回錯誤「賣家正在修改商家認證，重新核准後會自動重新綁定。」（5a #38，你已核可這句字）［量測：C9］。5b 之後卡片照 F8 的規則改成不顯示鍵、直接顯示這一句。字不變，改的是顯示的位置與時機，屬 UI 改動，所以列給你。**建議：核可**（同 N11、N17 的處理：伺服器說不能做就顯示原因，不讓員工按了才知道）。附帶一點：賣家在綁定失敗之後送件、再自己取消申請時（可達［推論］），這句的「正在修改」不完全準確，但「重新核准後會自動重新綁定」仍成立；不另寫句子，觸發是營運反映看不懂。

其餘 v1.3 的變更都是照 repo 規則（伺服器推導可用性與原因、沿用也是決策、不留技術債、上線前不受相容性約束）直接定的，理由在附錄 C。

## 決定紀錄

- Will 2026-10-21：F3 選 A（暫停審核，含等待刪除）；F4、F6 照建議顯示；F12 刪 admin `PayUniRegistration` 的 `outcome`、`payment_enabled`、`needs_reonboarding`；§5 N1–N8、N10–N16 核可。
- Will 2026-10-21（開工前 design-review，附錄 B）：帳號停權比照 F3 暫停審核（不在待辦、沒有期限，停權結束自動回來，期限從送出、最近一次恢復營業、最近一次停權結束三者最晚起算；核准擋、退回照舊可用；N7 的字不變）；店家等待刪除時不能重新綁定 PayUni，新句 N17「店家等待刪除，不能重新綁定 PayUni。」核可；其餘 6 項照 controller 的裁定改（N12 刪除）。

---

## 附錄 A：審查意見的處理（v1 → v1.1）

| 編號 | 意見 | 處理 |
|---|---|---|
| H1 | F3 擋掉退回後，已關閉店家的待審申請永遠無法處理，卻仍在待辦與期限上 | 改。F3 重寫：列出四個選項（暫停、關店時取消、退回照舊可用、v1 原案），建議「店家不是營運中時暫停審核、不在待辦、恢復營業時回來、期限從恢復時起算」，同食品業者登錄與 PayUni 的先例；改 `review_kyc_application` 與兩支期限函式（§2.1）、補整合測試（§6.1）；仍要 Will 決定 |
| M1 | 食品業者登錄的 `approveBlockedText` 被延後 | 改。沒有找到 REAL 約束，併入本刀（F13）：契約兩組原因、`FoodRegistrationReview` 與 `Decide` 改走同一個函式、client 刪推導；文案 E8、N13、N14、N15 |
| M2 | 「KYC 退回在等待刪除時要不要擋」留成待決 | 改。併進 F3 一次決定：建議 A 下等待刪除與已關閉同一條（審核暫停）；§9 的待決刪除 |
| M3 | F5 把 PENDING 列成不可用，方法卻成功 | 改。原因拆成 `PAYUNI_BIND_IN_PROGRESS` 與 `PAYUNI_BOUND`；`can_rebind` 定義成「會開始新的綁定」；方法與可用性用同一個 `RebindUnavailableReasons`，PENDING 的重送安全寫進契約（F5、§1、§2.1） |
| M4 | NotFound → failed_precondition 的測試清單不完整；`storeClosedNotFound` 成了死分支 | 改。§6.1 逐一列出 6 處斷言（重讀過）與 `cases_dataset` 的參數、例外與註解刪除 |
| M5 | E1 會在退回對話框顯示，標點不一致 | 改。E1 改列 N8；訂一條標點規則（伺服器完整句以「。」結尾、client 不加標點、兩句分開排），E 類一律加句號，清單按鈕也一樣；N16 改成兩句分開 |
| M6 | 約束帳把自己的程式算成 REAL | 改。「措施以帳號為範圍」「刪除帳號歸 accountdeletion」改 SELF-IMPOSED；F4、F6 改用業界基準論證（處分以帳號為範圍、刪除是帳號層級），法規的刪除請求權另列 REAL（未查證） |
| L1 | 三條 [推論] 讀碼就能定案 | 改。C8、C23、C24 改成量測並附 file:line；§2.1 刪「實作時先以整合測試確認」 |
| L2 | §0.2 引錯 `stint.go:65`；`LockOpenAccountOf` 死碼 | 改。§0.2 改引 `stint.go:28-29`（`stint.Lock`）；`LockOpenAccountOf` 本刀刪（C22） |
| L3 | 驗證命令寫錯 workflow | 改。§8 改成 `backend-monolith.yml` 的 golangci-lint、Connect drift、sqlc drift 各自的行號，註明兩個舊 workflow 不觸發 |
| L4 | 設計書 56 只修一處 | 改。查證四處有這句（:2398、:2459、:5813、:5858）；:5370 是 9272 的說明、沒有這句，不改；CodeMeta `錯誤碼.md:345` 凍結，不改（C14、§7） |
| L5 | 共用 enum 的多數組合沒有意義，測試自相矛盾 | 改。保留一個 enum（理由重寫，承認 v1 的理由不成立），加 Go 的有效組合表；測試改成「表內有句子、表外回零值、推導只出現表內原因且依序」（F1、§2.1） |
| L6 | 「建立措施被擋」有兩句話 | 改。案件與通報的原因句改回 N1（同一個函式），刪兩個 key；斷言三處同一句（F2、§6.1） |
| L7 | 已關閉而且 PayUni 終止時，句子叫賣家「關閉店家」 | 改。N10 改寫成不叫賣家關店，已關閉與營運中都成立 |
| L8 | 5a 的邊界只有單方面宣告 | 改。5a 設計書已存在；§0.2 依 controller 裁定改寫（admin PayUni 全歸 5b），兩邊的不一致列在 §9 給 controller |
| A10 理由 | 代行授權那一半延後的理由寫成「A10 才有契約」 | 改。§0 與 §0.2 改成「這些拒絕的句子今天已由伺服器的 `DeniedLabel` 經 preflight 原樣送到畫面，client 沒有推導」 |
| controller D4 | admin 的 `payUniProblemText` 與綁定失敗句沒有人認領 | 改。F11 逐項分類：徽章的階段判斷改由伺服器 `flow_status`、名詞走目錄；問題文字是 PayUni 原文，直接讀 `status_codes.reason`；綁定失敗句與「需補資料」是事實呈現，留 client。F12 刪 admin 三個只為推導存在的欄位（要 Will 決定）。§0.2 原 :47 的界線改寫 |

## 附錄 B：開工前的 design-review（v1.1 → v1.2，8 項全改；全文在 session scratchpad `5b-design-review-start.md`）

| # | 發現 | 處理 |
|---|---|---|
| 1 | 「待審核」有三個定義：待辦、商家清單分頁（`AdminStores` 的 `kyc_status`）、統計卡（`AdminStoreStats` 含等待刪除）；§9 的盤點搜尋抓不到參數化過濾（C，高） | **改**：三處同讀 `review_kyc_application`（`ListStoresRequest.kyc_awaiting_review`、`kyc_awaiting_review_stores`，`kyc_status` 不再接受 SUBMITTED），字不變；成員對應的 Go 定義寫明是 `APPROVE_KYC` 清單為空；資料集測試鎖四者一致（F3、§1、§2.1、§3.1、§6.1、§9、C25） |
| 2 | delegation 把已關閉當 `Found=false` 再掛原因；`DeniedLabel` 留到不了的後備句（B，中） | **改**：已關閉是 `Found=true`＋`DenialStoreClosed`，9265 只在 legacy `bizCodes`；刪 `DeniedLabel` 的 `DenialStoreNotOperable` 分支，句子空時回內部錯誤（F7、§2.2、§6.1、C27） |
| 3 | 停權照現行：件在待辦、期限照算卻不能核准（A，中） | **改（Will 決定）**：比照 F3 暫停，退回照舊可用，期限從三者最晚起算；食品業者登錄的裁定不看停權，同一條規則下不用改；N7 不變（F3、§2.1、§6.1、C26） |
| 4 | 有效組合表與四支推導各自編碼順序，靠測試對齊（B，中） | **改**：表的每列帶條件，推導就是跑表，順序只有一份；刪「依表的順序」測試與突變（F1、§2.1） |
| 5 | 重新綁定沒有等待刪除，與 KYC、食品業者登錄不一致（A，中低） | **改（Will 決定）**：加 `STORE_PENDING_DELETION`，新句 N17（F5、§2.1、§5、§6.1） |
| 6 | N12 沒有讀者（B，低） | **改**：刪 N12；表上把 `PAYUNI_BIND_IN_PROGRESS` 標成 `idempotent`（不是失敗原因、沒有句子），測試規則跟著改（F1、F5、§1、§5） |
| 7 | 「哪些原因表示沒有入口」只寫在 admin（C，中） | **改**：enum 註解逐值標明，照 L′ 先例；admin 寫成窮舉的 `Record`（F8、§1、§6.2） |
| 8 | §9 與 5a 的界線是過時的待決（C，高） | **改**：§9 換成已定的結論：5a 先合併、5b 刪 `PayUniOutcome`、階段只經 `PayUniPhaseOf`；另寫 5a 若加綁定失敗待辦時的成員對齊（§9） |

## 附錄 C：5a 合併之後的對照（v1.2 → v1.3；對照 `1fc535d87`，5a 是 `2bba5d0b4`、`1fc535d87`）

方法：`git diff a6f7b3815 HEAD --stat`（131 個檔）與 `--name-only`，和本文引用的每個路徑取交集；交集裡的檔逐一 `git diff -U0` 看行號位移並重讀內容；沒有變的檔的行號照舊成立，另抽查 20 處（`measure.go:417-419`、`create.go:492-506`、`update.go:203`、`capability.go:78`、`foodreg.go:189-192,365-367,453-457`、`registrycheck.go:136-141`、`kyc/admin.go:396-399`、`stint.go:28-29`、`stores.sql:200-205`、`H/admin.go:182`、`store-search.ts:60-61`、`measure-actions.ts:113`、`PayUniCards.tsx:130-134` 等）全部相符［量測］。

| # | 變更 | 類型 | 位置 |
|---|---|---|---|
| 1 | `PayUniPhaseOf` 已由 5a 抽出（`model/payuni.go:505-526`）並有 `TestPayUniPhaseOf`；本刀改成沿用，刪「從 `DeriveKYCFlow` 抽出」的工作與等價測試 | 前提改變，改寫 | §0、§0.1、F11、§2.1、§6.1、§7、§9 #3、C28 |
| 2 | 5a F17 的 `Rebind` 申請檢查收成 `REBIND_PAYUNI` 的原因：沿用 `KYC_NOT_APPROVED`（不新增 `KYC_RESUBMITTING`：同一個事實，句子依 (reason, action) 分開）；排在最後（不照 5a §7.1 的「`PAYUNI_BIND_IN_PROGRESS` 之前」：BOUND 而申請是草稿到得了，排前面會在不可能重新綁定的卡片上顯示句子；也與現行檢查順序一致）；`RebindUnavailableReasons` 多 `app KYCStatus` 輸入（5a 完成前 review 指出的缺口）；刪 `ErrRebindApplicationNotApproved` 與舊 key，句子改 key、字不變 | 前提改變，決定（repo 規則：可用性與方法同一個函式、不留第二個條件） | F5、§0、§0.1、§1、§2.1、§5 N18、§6.1、§6.2、§9 #4、C9 |
| 3 | v1.2 在 §2.1 臨時加的「修訂（5a 完成前 design-review F-2/F-8）」子項併入正文後刪除 | 整理 | §2.1 |
| 4 | 本刀刪 proto `PayUniOutcome`（5a 已合併，admin `outcome` 是最後一個使用者）；Go `model.PayUniOutcome` 留著；列出要一起刪的 handler 轉換與兩個測試對照、兩份 enum manifest 由 `make uitext` 重新生成 | 前提改變，具體化 | §0.1、F12、§1、§2.3、§6.1、§7、§9 #2、C30 |
| 5 | 5a 的待辦 `PAYUNI_BIND_RETRY` 已存在：v1.2 §9 #5 的「成員應該與 `REBIND_PAYUNI` 清單為空同一個定義、由本刀改函式條件」改成單向斷言「待辦有列 ⇒ 清單為空」，不改 5a 的函式（5a F16 刻意只收 `PAYUNI_UNAVAILABLE`，Will 已核可） | 前提改變，改寫 | §0.2、§6.1、§9 #6、C29 |
| 6 | F11 的問題文字：5a 起 `problem.reason` 是修剪過的 `Status.Reason`，結論（client 直接顯示修剪後的 `status_codes.reason`）不變，證據改寫；徽章的 staff 名詞重讀，與後備表的字相同，唯一差異仍是 UNRECOGNIZED | 重新查證 | F11 |
| 7 | 5a 刪 `model.KYCFlow` 的 PayUni 欄位與 `CANCELLED` 原因：本刀不依賴（終止的事實讀 `PayUniRegistration.NeedsReonboarding`），只改 §0.2 的說法 | 重新查證 | §0.2、§9 #5 |
| 8 | 行號位移：計畫 10e `:346-387/:380/:382/:383` → `:346-389/:381/:383/:384`；`intent.go:46-63` → `:47-64`、`:159-185` → `:156-216`、`:170-185` → `:160-216`、`:178-180` → `:181-183`、`:184` → `:185`；`kyc/kyc.go:467` → `:452`；`model/kyc.go:697` → `:701`；`model/payuni.go:486-511` → `:505-526`、`:533` → `:558`、`:470-545` → `:505-574`；`text/zh-TW.json:275-278,451-454` → `:339-342,519-522`；`payuni_integration_test.go:631-640` → `:632-641`、`:660` → `:661`；`review_policy_integration_test.go:358,1095` → `:364,1207`；賣家端 `seller.proto` 的 `SellerRequirement` `:279-295` → `:274-290` | 修引用 | 全文 |
| 9 | F1 的「13 個有效組合裡 9 個是店家或帳號的狀態」是 v1 的表，重數 v1.3 的表是 28 個裡 17 個 | 修數字 | F1 |
| 10 | §9 #1 的測試名 `TestPayUniStateOf` 不存在，改成 5a 實際的測試名；5b 從 HEAD 開工，不需要 rebase | 修引用 | §9 #1 |
| 11 | `Rebind` 鎖順序與賣家儲存草稿的序列化、F3 migration 參考 5a 的 `pub_review_items` 重建寫法 | 補充 | §2.1 |
| 12 | 新增「待 Will 決定」：N18 的新顯示（字是已核可的 #38） | Will 2026-10-21 核可 | 文末、§5 |

## 實作時的修訂

5b 實作之後，與 v1.3 不同的地方（驗證與突變的結果在 session scratchpad `5b-impl-report.md`，不入庫）：

- 契約：`CreateMeasure` 的「店家已關閉回 failed_precondition」註解寫在 `governance.proto` 的 `CreateMeasure` RPC 上（RPC 在那裡），`measure.proto` 只補 `can_end` 的註解。`GetStoreStatsResponse.kyc_awaiting_review_stores` 用新編號 6，舊的 2 與名稱 reserved。
- seller 模型：`storeActionFacts` 的 `PayUni`、`FoodReg` 是值，不是指標；新增 `KYCApplicationState{Status, SellerType}`（申請的事實）。`FoodRegistrationUnavailableReasons(r, a)` 不收 `Store`：食品業者登錄的店家條件只看 `r.StoreStatus`，讀取路徑（清單、詳情）本來就沒有載入整個店家。另有 `StoreActionIdempotent(action, reason)`（`Rebind` 判斷第一個原因是不是 idempotent）與 `ErrStoreActionUnavailable(reasons, action)`（清單空或第一個原因沒有句子是內部錯誤，不寫通用句）。`FoodRegistrationReview` 加 `ApproveUnavailable()`、`RejectUnavailable()`，`CanApprove`／`CanReject` 是它們為空。
- DELEGATE 的原因順序是規則表的順序（已關閉、等待刪除、停權），不是 `Requirements()` 的順序；等價測試以集合比對（`Requirements()` 把 `NOT_SUSPENDED` 排在 `NOT_PENDING_DELETION` 前面），順序本身由規則表一處決定。
- 後台的註冊讀取與 `RebindPayUni` 的回應是新型別 `model.AdminPayUniRegistration`（內嵌 `PayUniRegistration`，加 `FlowStatus`、`RebindUnavailable`），不是把兩個欄位塞進 `PayUniRegistration`：後者也是賣家端與 job 用的儲存事實，不該帶只有後台讀取才推導的欄位。`withSellerType` 與新的 `withApplication` 同一次讀取申請的狀態與賣家型態；`Rebind` 以 `stint.Load` 載入店家（與讀取同一個 `adminViewOf`，鎖內、同一份事實）。
- `LockOpenAccountOf`、`ErrRebindApplicationNotApproved`、`text.PayuniRebindUnavailableApplicationNotApproved`、`CaseUnavailableStoreClosed`、`ReportUnavailableStoreClosed` 刪除；`payuni.rebind_unavailable.application_not_approved` 改名成 `store_action.unavailable.kyc_not_approved.rebind_payuni`，字不變。staff 句子的 key 把同一句的動作合成一個 key（`store_closed.review_kyc` 給核准、退回、重新查詢，`decide_food_registration` 給食品業者登錄的核准與退回，`kyc_not_submitted.review_kyc` 給核准與退回），不是每個動作一個 key；句子逐字是核可的文案。
- migration `20261021144547_seller_kyc_review_pause.sql` 用 `CREATE OR REPLACE FUNCTION` 改 `review_kyc_application` 與 `review_food_registration`（回傳型別不變，`pub_review_items` 對它們的依賴因此保留），不是 drop 再重建 `pub_review_items`。先例：`20261019042024_seller_effective_measures_single_select.sql` 同樣以 `CREATE OR REPLACE` 改函式本體；5a 要 drop 再重建，是因為它改了 `pub_review_items` 的形狀。`objects.sql` 與 migration 的定義相同。
- `AdminStores`／`AdminStoreStats` 的 `now` 由模組的時鐘傳入（`s.d.Now()`）。
- delegation：`DenialStoreClosed` 加在 `DelegationDenial` 的最後（不插在中間，避免改動既有值的編號）；`DeniedLabel` 刪 `DenialStoreNotOperable` 分支之後，兩個店家狀態的拒絕不經 `DeniedLabel`（preflight 直接用 `st.Unavailable`；修正輪改名為 `st.UnavailableText`）。「不可營運而句子空時回內部錯誤」的分支沒有另外用 stub 測：真的 `seller.Store` 在不可營運時一定有原因，而 `store_action_test` 鎖住「每個 blocking 組合都有非空句子」（DELEGATE 的三列都是 blocking），所以真的 seller 到不了這個分支，由那個測試保護。legacy `codemeta_test.go` 的名稱對照加 `DenialStoreClosed`（對到 `DELEGATION_STORE_NOT_DELEGABLE`，CodeMeta 凍結、不改）。
- admin：`unavailableLabels`、`GENERIC_UNAVAILABLE_LABEL` 搬到 `A/lib/unavailability.ts`，`measure-actions.ts` 與它的測試改 import（不留 re-export）。新增純函式讓行為可以直接測、不靠原始碼斷言（修正輪改名／作廢：`visibleUnavailableLabels`、`reviewBlockedReasons`、`decideBlockedReasons` 已由 `firstVisibleLabel`、`reviewBlockedLabel`、`decideBlockedLabel` 取代，只顯示第一個原因）：`visibleUnavailableLabels(primary, ...rest)`（是否顯示只看第一份清單的第一個原因）、`reviewBlockedReasons(application)`、`decideBlockedReasons(registration)`、`rebindPresentation(registration)`；PayUni 卡只顯示第一個原因的句子（不是整個清單）。商家詳情的核准與退回提示只併 `approve_unavailabilities` 與 `reject_unavailabilities`（不併 `requery_registration_unavailabilities`：個人型的重新查詢有 `KYC_NOT_BUSINESS` 的句子，併進來會在不相干的情況多出一句）。食品業者登錄詳情的裁定區在兩個旗標都是 false 但有原因（已關閉、等待刪除）時仍畫出來（停用的鍵加原因）：否則 §3.3 說的「現在會顯示原因」沒有地方放。`RowState.isSuspended`、`StoreActions.closed` 的用途消失，刪除；`isActive` 只留給按鈕與徽章的顏色。商家詳情「刪除帳號」的條件是 `accountEmail.value.state === "ready"`（F6 的建議）。
- 沒有做 ADR 修訂：設計 §0.1 的文件清單沒有 ADR；db D4-13（決策簡報第 13 點）、設計書 56 的四處與文末修訂、計畫 10e 的 5b 完成紀錄、seller 與 delegation 的 `AGENTS.md`／`CARRYOVER.md` 已改。
- 範圍外沒有動：website、seller-app、buyer-app 的原始碼（只有生成物因 proto `PayUniOutcome` 刪除而變，`rg "PayUniOutcome"` 在三個 client 與兩個共用套件的手寫碼沒有命中）、gateway（`gateway-json-contract-check.py` 照跑）。

### 修正輪（完成前 design-review 與 code-review 之後，2026-10-22）

- 伺服器仍列出全部原因，admin 只顯示第一個可見的原因（與 PayUni 卡、代行清單一致）。商家詳情的核准提示改成 `reviewBlockedLabel`（只讀 `approve_unavailabilities` 的第一個原因），退回對話框橫幅是 `reject_unavailabilities` 的第一個原因加固定的一句，食品業者登錄的提示是 `decideBlockedLabel`；共用的 `firstVisibleLabel` 取代 `visibleUnavailableLabels`（不再合併多份清單、不再列出多句）。原因：終止合作關店時帳號仍有生效中的措施，核准的原因是 [已關閉, 停權]，列兩句會同時說「暫停審核」與「停權解除之後才能核准」。測試用真實的伺服器形狀（[STORE_CLOSED, ACCOUNT_SUSPENDED]、[STORE_PENDING_DELETION, ACCOUNT_SUSPENDED]），舊行為（列出全部）下失敗（突變：`firstVisibleLabel` 改成串接全部句子，4 個測試失敗）。§3.2 與修訂裡「依序合併、相同的句子只列一次」的說法作廢。
- 食品業者登錄詳情（Will 2026-10-22）：兩個動作都不能做而且有句子時，裁定區只有標題與第一個原因的句子，沒有備註欄、沒有鍵；至少一個動作能做時維持原本的版面。上面「兩個旗標都是 false 但有原因時仍畫出完整裁定區」的說法作廢。
- 規則表的每一列帶自己的句子（`storeActionRule.Text`，nil 就是 idempotent）：`StoreActionUnavailableText` 與 `StoreActionIdempotent` 都只查表，原本獨立的 switch 與 `ruleKind` 刪除，句子、種類與組合只有一份。上面「句子 key 依動作組合併」的對應也在表裡。
- delegation `StoreState.Unavailable` 改名 `UnavailableText`（欄位裝的是句子，不是狀態）；`AGENTS.md`、`CARRYOVER.md` 的「沒有 Unavailable」改成「沒有『讀不到』的狀態」。
- `objects.sql` 審閱政策總說明的「一律日曆時間、不暫停」改成開始時間由各種類定義（KYC 與食品業者登錄從可以裁定時起算，店家不是營運中或停權時暫停，恢復後重新起算），只改 `objects.sql` 的註解（已提交的 `20261020062336_seller_review_functions.sql` 的註解保留舊句：已提交的 migration 是歷史紀錄，改它會讓已套用的資料庫校驗和對不上；5b 的 migration 不含這句）。`atlas.sum` 只多 5b 那一筆，`atlas migrate validate` 通過。`account_sanction_lifted_at` 的註解改成「第五種讀法（前四種見 AGENTS.md）」，與 AGENTS 的編號一致。
- `FoodRegistrationUnavailableReasons` 看不到停權（讀取路徑只有 `StoreStatus`）：規則表註解與 seller `CARRYOVER.md` 記錄，這兩個動作的規則不能用 `whenSuspended`；之後若要讓停權擋食品業者登錄，要同時讓讀取路徑載入停權並改 `review_food_registration`。
- `withSellerType` 併入呼叫端；`kyc/admin.go` 新增 `reviewFacts`，`adminViewWith` 與 `reviewReasons` 共用同一種事實組法。
- 測試：`TestFoodRegistrationReviewRestartsFromTheReopening`（送出、終止關店、48 小時後撤銷，開始是恢復營業的時刻）鎖住食品業者登錄的 `reopened_at` 項。KYC 的 `reopened_at` 項在現行路徑下冗餘（唯一的恢復營業路徑是撤銷終止，撤銷同時結束停權，`account_sanction_lifted_at` 給出同一個時刻），沒有可建構的區分案例，所以改正 `store_action_integration_test.go` 的註解如實說明。`TestDelegateReasonsFollowTheConditionsNotTheStatus` 讓條件與 Status 不一致，「DELEGATE 只看 Status」的突變下失敗。
- seller 與設計書 56 的文件：`CARRYOVER.md` 的 8b 列補上 `PayUniRemedy`（5a）與 `PayUniOutcome`（5b）已從 proto 刪除；設計書 56 的 9272 說明列改成停權／治理停權／等待刪除，已關閉是 9265。
- `RejectKycDialog` 的橫幅改用 `GovernanceNoticeBanner`（`text` 可以是多行陣列），不再自己畫一份標記。
