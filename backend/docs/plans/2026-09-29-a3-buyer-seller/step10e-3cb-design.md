# 10e 第 3c-b 刀：詢問、轉報 v2 與主管機關來函，實作設計 v3

> **狀態**：v3，Will 2026-10-19 對文末的「§5 頁面與文案摘要」與「給 Will 的決定題」回「照建議」：Q1 (a)、Q2 (a)、Q3 (a)、Q4 (a)、Q5 (b)、Q6 照列的值，§5 的頁面與文案全部照建議（M6、M8、M7 因此都需要）。已處理兩輪獨立審查（第一輪 1 個 blocker、8 個 major、14 個 minor；第二輪 0、1、7）與開工前的 design-review（7 項，v3.1），對照見附錄 A。design-review 新增的伺服器句子 `authority_order.target_overlap` 與 3c-b-3 新增的涵蓋狀態「保存期限已過，個資已依保留期限清除」Will 2026-10-19 核准。

讀取的是 `local/10e`（main `f0405c1e5` 合進來之後），worktree 沒有未提交的改動［量測：`git status --short`］。
SSOT（Will 2026-10-18 對 R1–R12 回「照建議」）：`step10e-reports-brief.md` v3（下稱 rb；3c-b 的範圍在 rb §4，被改寫的既有決定在 rb §3，3c-a 的實作修訂在文末「實作時的修訂（3c-a）」）；研究 `step10e-reports-research.md`（下稱 rr）、現況 `step10e-reports-facts.md`；`step10e-3cd-brief.md`（下稱 3cd：D′4、D′7、D′8 與 §2）；3c-a 的設計 `step10e-3ca-design.md`（下稱 3ca，C1–C29 與附錄 D 是本刀的地基）；ADR `2026-10-18-seller-governance-consolidated`（下稱 gov ADR）、`2026-09-24-go-modular-monolith` 文末「修訂：跨模組讀取改用擁有者發布的 pub_*、D4 收窄」（下稱 D3／D4 修訂）、`2026-10-02-seller-kyc-payuni-in-monolith`「個資保留」；〈API 設計規範（Connect）〉；術語表。
路徑縮寫：`W` 是 worktree 根目錄；`S`＝`W/backend/internal/modules/seller`；`H`＝`W/backend/internal/httpapi`；`A`＝`W/admin/src/app/admin`；`P`＝`W/api/hawkr/admin/seller/v1alpha1`。
標記：**[量測]** 這次讀到原始碼或跑了指令；**[推論]** 由讀到的內容推得，沒有執行；**[外部]** 讀了外部來源（附出處）。
判定原則：上線前沒有相容性約束；每個取捨照業界從零的做法；只有基準定不了的產品或法務選擇、或兩份 SSOT 互相牴觸的，列在 §10 給 Will。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| 3c-a 的案件模型（`cases`、`case_referrals`、`CaseFacts.Availability`、`Case.AwaitingDecision`／`DecisionClockStart`） | 沒有排程；rb §4 本來就把詢問、轉報 v2 排在它上面，3c-a 留了掛點（`S/internal/model/report_case.go:274-278`、`:438-442` 的註解寫「3c-b 加…」）［量測］ | 擴充，不重寫 |
| `report_cooperation_requests` | 3c-a 刪了 RPC 與事件，表留給 3c-b（gov ADR「事件」段、`S/db/schema.sql:422`）；沒有任何程式讀寫［量測：`rg` 只命中 migration、devseed 的 DELETE、測試］ | 本刀刪除（§2.5） |
| 個資清理 `RetireDueStores` 與 `StoreRetention`（30 天） | 沒有排程；詐欺條例第 37 條第 1 項要改保存期（rb R5，REAL） | 本刀改 |
| 個資清理與既有的申覆 legal hold 的競態（`purge.go` 的 `batch`） | main 上的缺陷，與 3c-b 無關（§2.4 的 G23） | 獨立的第一個 commit 修（§8 的 3c-b-0），本刀的來函保存建在修好的機制上 |
| 賣家端的詢問 API 與畫面 | 10f（rb §4、計畫書 10f 段） | 本刀不碰；契約只放 admin 端，資料模型留好 10f 要的欄位形狀（§2.1 的「10f 會改的」） |
| 商品限制措施、商品頁的保存 | B4（rb R6） | 來函的「限制或移除商品」在 B4 之前只能人工經 legacy 強制下架；本刀只記事實 |
| 待辦頁（`ListReviewQueue`） | 3d | 本刀只交判定式、開始時間、期限與索引（§6），同 3ca Q5 (a) |
| admin 的共用 `<Dialog>` | 另一刀（3ca 附錄 D #6），可能正在進行 | 新對話框照實作當時 main 上的寫法：共用元件已落地就用它，還沒落地就照 3c-a 的對話框骨架；不為了等它延後，也不先做一半 |
| 法律文件（隱私權政策、賣家身分認證告知事項、食品業者登錄告知事項、賣家合作條款附件） | rb R5 約束帳「隱私權政策要改：I-legal」、rb §4 把法律文字排在 I-legal（已核准） | 照 rb：本刀只改程式，文字在 I-legal 改；後果與觸發見 B21 |

結論：本刀改的都是會留下的程式，照從零的最佳做法做，不做拋棄式的過渡。

---

## 0.1 範圍

| 部分 | 內容 |
|---|---|
| 詢問（R4） | `CaseInquiry`：平台對賣家的「提供紀錄」或「請說明」，有期限（預設 3 天）、一則回覆（文字＋附件）、可撤回、結案時自動撤回；admin 建立、撤回、記下系統外收到的回覆；附件是機敏佐證（讀取紀錄、檔案路由）；`CaseInquiryCreated` 事件；案件的「等賣家」推導與清單篩選 |
| 轉報 v2（R5） | 轉報紀錄加追蹤日期（預設 30 天後）、主管機關的回覆（可多則）；「不轉報」的出口（原因）；轉報資料寫明以平台名義轉報 |
| 主管機關來函（R5） | `AuthorityOrder`：獨立登記（機關、文號、收文時間、種類、期限、指名的帳號或店家、商品）、完成（時間與摘要）、保存期限（預設 184 天，延長留歷史）；認定可以引用來函；措施可以記來源來函；保存只加在 `RetireDueStores` 的清理那一層（不影響恢復營業），清理在鎖住之後再判斷一次 |
| 詐欺條例的保存期（R5） | 關店後的個資保留從 30 天改成 184 天（任何連續六個曆月的最大長度）；法律文字照 rb 在 I-legal（B21）；範圍見 Q2 |
| 配合要求的退場 | 開發資料的檢查（§2.5）與刪表 |
| 既有缺陷（獨立 commit） | 清理與申覆 legal hold 的競態（§2.4）：清理鎖住候選之後再判斷一次；`SubmitAppeal` 在鎖之後取現在時間 |
| 3d 的前置 | 「詢問逾期」「主管機關來函」的判定式、開始時間、期限與索引；「待決定」「待轉報」的判定式更新（§6） |
| admin | 案件詳情的「詢問賣家」區塊與 3 個對話框、轉報區塊的 3 個對話框；「主管機關來函」清單、詳情與 4 個對話框；側欄；從來函建立措施的三個入口 |
| 文件 | gov ADR 文末修訂（詢問、轉報 v2、來函與保存）、`2026-10-02-seller-kyc-payuni-in-monolith` 個資保留的修訂、設計書 32、食安 runbook、seller `AGENTS.md`、CARRYOVER、術語表 |

## 0.2 不在範圍

- 10f（信任與安全上線前）：賣家端的詢問 API 與畫面（期限改成從出現在賣家中心起算、`SELLER_CENTER` 回覆管道、賣家上傳附件）；檢舉表單的同意勾選、聯絡方式快照與 1919 說明；通報人的「我的通報」；權利人通報與工作日行事曆。
- A8：`CaseInquiryCreated` 的訂閱者（通知賣家）；通知被處置者的送達（DSA 9(5) 的「處理之後仍要告知被處置的賣家」靠措施本身的理由與 A8 的措施通知）。
- B4：商品限制措施與商品頁的保存（草案第 43-1 條的「違法資訊、網頁資料」）。
- 3d：`ReviewPolicy`、來源查詢、合併層、待辦頁、`ReviewQueueItem.subject` 的 `inquiry_id`、`authority_order_id`。
- 非賣家帳號（只是買家）的資料保存：登記照收，保存待定（Q3）。
- 詐欺條例第 37 條第 1 項在 seller 以外的部分（identity 的註冊與登入資料、買家的資料、交易紀錄）：Q4。
- 法律文字的改動：I-legal（rb §4）。

---

## 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| B1 | 詢問掛在哪裡 | 案件的子資源 `case_inquiries`，複合外鍵 `(case_id, store_id)` 指向 `cases (id, store_id)`；不掛在通報上 | rb R2「轉報、詢問…都掛在案件上：一次事件…只詢問一次」；Zendesk 的 Pending 是工單（處理單位）的狀態，不是使用者回覆的狀態 [外部，rr §4] |
| B2 | 對象 | 只有賣家，**不加 `addressee` 欄**。必要條件：出現第二種對象。rb R4 的「之後也可以是通報人」是那時的觸發條件，屆時加欄位、讓「等通報人」成立，並把 B4 的 partial UNIQUE 改成每個 (案件, 對象) 一筆（rb R3：「等賣家、等通報人…兩者可以同時成立」） | necessity condition；上線前加欄位只是一支 migration |
| B3 | 種類 | `RECORDS`（提供紀錄：條款 11.3 的來源、批次、效期、進出貨及流向）、`EXPLANATION`（請說明） | rb R4 |
| B4 | 同時幾筆 | 每個案件同時最多一筆還沒結束的詢問（partial UNIQUE `case_inquiries_one_open`）；要同時要紀錄又要說明，寫在同一則說明裡，種類選主要的那個 | 「等賣家」是二值；Amazon 的 compliance request 一次一件、各自有期限（SECONDARY）；多筆並存會讓「最後一筆被回覆」的起算點出現部分回覆的歧義。觸發：營運需要對同一案件並行兩個期限不同的要求 |
| B5 | 期限 | 存 `due_at`；建立時預設 `created_at + 72h`；日曆時間（rb R5 約束帳：工作日行事曆是 SELF-IMPOSED，先用日曆天）。rb R4「預設 3 天，可以改」我讀成「建立時可以改」，建立後不能改；可指定的範圍（1 到 14 天）不在 rb，是起始值，列在 Q6 | rb R4；Zendesk SLA 依日曆時間 [外部，3cd D′5] |
| B6 | 回覆 | 一筆詢問一則回覆，回覆即結束（不是對話）；回覆的欄位在詢問列上；admin 記下的回覆帶管道（電話、email、通訊軟體、其他）、**收到的時間**（不早於建立、不晚於現在）與記下的時間；過了期限仍可回覆（逾期只是待辦） | rb R4「管道、時間、內容、附件」；Amazon 的 compliance request 是交件即結束（SECONDARY）；ServiceNow 的 Awaiting Info 在客戶回覆後回到 Open [外部，rr §4]；要再問就開新的一筆 |
| B7 | 附件 | `case_inquiry_reply_evidence`，最多 `model.MaxEvidence` 份（現在是 5，與通報、申覆同一個常數；份數是起始值，Q6），機敏檔，照通報佐證：轉正時要求 `is_secure`、admin 檔案路由、先記讀取紀錄再讀；`evidence_accesses` 的擁有者加 `inquiry_id`。上傳路徑見 B27 | rb R4、§3 第 2 列；gov ADR「佐證」 |
| B8 | 轉述 | 詢問只帶管理員寫給賣家的說明，不附通報原文、佐證與通報人；賣家的回覆與附件也不轉給通報人（通報人只看到案件的結果說明）。兩個方向都由管理員轉述 | rb R4（DSA 前言 54 [外部，rr §1]）；反向同理：賣家的紀錄含營業資料與第三人個資，資料最小化 |
| B9 | 撤回 | 管理員可以撤回還沒回覆的詢問（`withdraw_cause = ADMIN`）；`DecideCase` 與移動造成的 `NO_REPORTS` 關閉在同一個 transaction 撤回（`CASE_CLOSED`） | rb R2「案件結案時，還沒回覆的詢問自動撤回」 |
| B10 | 版本 | 建立、記下回覆、撤回都讓 `cases.version` 與 `updated_at` 前進（它們改變決定的依據與「可以決定」）；詢問的指令本身不帶 etag：建立帶 `request_id`（AIP-155），回覆與撤回是一次性的轉移，重送回原本的結果 | 3ca C11「案件層級的處理前進」；先例 `RecordFDAReferral` 的 `already_recorded` |
| B11 | 推導 | 案件 `awaiting_seller`＝有還沒結束的詢問；`AwaitingDecision`＝進行中而且不在等賣家；`DecisionClockStart`＝`max(opened_at, 最後一筆詢問的 reply_received_at 或 ADMIN 撤回時間)`。用**收到**的時間，不用記下的時間：晚記下的回覆讓案件回到「待決定」時，期限已經用掉一部分甚至逾期，這是刻意的（期限算的是平台拿到資訊之後的處理時間，不因晚登記而延長；Zendesk 以顧客回覆的時間起算），§7.4 有測試 | rb R3（Zendesk next reply time、JSM SLA cycles）；3ca §6 |
| B12 | 事件 | `CaseInquiryCreated`（rb §3 的改名）在建立的 transaction 內發；回覆與撤回不發（沒有消費者需要被推播；賣家端讀取時看到狀態）。觸發：A8 要通知賣家「已撤回」 | 單體規則 4；gov ADR「通知與評分」 |
| B13 | 轉報紀錄 v2 | `case_referrals` 一個案件一列（主鍵保證互斥），加 `outcome`：`REFERRED`（管道、選填案號、追蹤日期）或 `WAIVED`（不轉報的原因）；主管機關的回覆是只新增的 `case_referral_replies`（可以多則：移送通知與結果通知是兩次） | rb R5；檢舉辦法：7 日內移送並通知、30 日內通知結果 [外部，rr §2] |
| B14 | 追蹤日期 | 台北日期（`DATE`），記錄轉報時預設轉報日＋30 天，可指定；記下主管機關的回覆時可以換成下一個追蹤日期或清掉；只顯示在案件上，不進待辦 | rb R5「追蹤日期顯示在案件上，不進待辦」 |
| B15 | 不轉報 | 只在食安案件、還沒有轉報紀錄、不是以 `NO_REPORTS` 關閉時可做；**決定前後都可以**：這是對 rb R3 的延伸（rb 的「待轉報」只在決定之後才出現），理由同 3ca §3.1「記錄轉報不要求先決定」——主管機關可能在決定之前就已經介入；記下後不能改成已轉報（要轉報時寫註記，觸發：第一次出現「不轉報之後又要轉報」） | rb R5「待轉報可以標記不轉報並寫原因」 |
| B16 | 主管機關來函 | 新資源 `authority_orders`：機關、文號、收文時間（管理員填）、種類（限制或移除商品、調取資料、暫停帳號、其他）、內容摘要、期限、完成（時間、管理員、處理摘要）。期限的預設依種類：限制、暫停 48 小時，調取 72 小時（這三個是 rb R3 的值）；「其他」與「來函自訂期限時可指定的範圍」不在 rb，是起始值（Q6）。不走通報的去重、限流、濫報、事件與聯絡方式清除 | rb R5、R3；詐欺條例第 37 條第 2 項「三日內提供」、草案第 43-1 條「二個工作日」、食藥署「通常 2 天內」[外部，rr §2] |
| B17 | 指名對象 | `authority_order_targets`：帳號（必填，**任何帳號**，以 identity 驗證存在）＋選填的店家（必須屬於該帳號；不填＝帳號全部的 stint）＋選填的商品 ID（不透明字串，同 `reports.product_id`）。保存只涵蓋 seller 的資料；帳號沒有店家時，對象照樣登記、待辦照樣出現，詳情頁標「平台沒有這個帳號的店家資料；買家與帳號資料的保存尚未支援」。非賣家資料的保存見 Q3 | rb R5「記下…指名的店家或帳號」「只凍結被指名者」；審查 major：只擋保存不擋登記，3 日的期限才看得到 |
| B18 | 保存 | `hold_until` 存在來函上，預設 `received_at + 4416h`（184 天：詐欺條例的 6 個月與草案的 180 日取較長者，6 個曆月最長 184 天，同 `AppealWindowHours`）；「調取資料」的 6 個月從哪裡起算見 Q5（建議取收文與提供兩者較晚的）；只能延長，每次延長留一列歷史（`authority_order_hold_extensions`：前值、新值、原因、誰、何時），訴訟時人工延長到裁判確定後 3 個月；完成不解除保存。`RetireDueStores` 加一條、只在清理時生效：帳號上有 `hold_until > now` 的指名（不指定店家或就是這個店家）時不清理；**保存不影響恢復營業**（恢復只問保留期是否已過，§2.4）；刪除 2 年期滿的食品登錄審核紀錄同一條件 | rb R5；詐欺條例第 37 條第 2 項「應於收受調取通知之日起三日內提供資料，並保存六個月…發生訴訟時，應延長保存至裁判確定後三個月」（G12）；gov ADR「判斷式只有 `RetireDueStores` 一支」（改寫成「保留期的式子只有一份」）；legal hold 只凍結刪除（Purview、Vault）；延長是來函上唯一可變的法律欄位，照 `severity_from`／`severity_to` 的先例存結構化的前後值，不寫成自由文字 |
| B19 | 認定引用來函 | `finding_authority_orders(violation_id, order_id)`，`NewFinding.source_authority_order_ids`（最多 5 筆，集合）；驗證：來函指名了這個措施的帳號 | rb R5「另一張關聯表，比照 `finding_moderation_reports`」；3cd §2 第 10 項不合成一張 `source_kind` 表的理由（FK 保不住） |
| B20 | 措施記來源來函 | `measures.source_authority_order_id`（任何依據；暫停帳號的來函最常見的是 RISK 停權，沒有認定可引用）；`ListMeasures.source_authority_order_id` 篩選＝明確連結 ∪ 認定引用。**這修訂了 rb §2 第 1 點**（「可以推導，不加欄位」）：rb 只想到經認定引用；RISK 與 CONTRACT 沒有認定（3ca F23），推導不到。做法與理由同 3ca C20／Q4 (i)（Will 核准的案件先例），寫進 rb「實作時的修訂（3c-b）」與 gov ADR 的修訂段 | 3ca C20；DSA 17(3)(b) 措施要說得出來源 [外部，rr §1] |
| B21 | 詐欺條例第 37 條第 1 項 | `StoreRetention` 從 720 小時改成 4416 小時，所有關店都一樣；清理的範圍不變（店家聯絡、KYC 個人欄位與證件、PayUni 自由文字、食品登錄字號、同意紀錄的 IP 與 User-Agent 一起保存到期滿；範圍的取捨見 Q2）。法律文字照 rb §4 在 I-legal 改：**後果**是從 3c-b-4 到 I-legal 之間，程式保存 6 個月而公告的四份文字寫 30 天（G13）；系統沒有上線、沒有讀者，所以沒有人受影響。**觸發**：I-legal 那一刀，最晚在上線前（列為上線的阻擋條件，寫進計畫書 I-legal 段） | rb R5（Will 照建議：關店後 6 個月，上線前請律師確認）、rb §4 |
| B22 | 配合要求的開發資料 | 不轉成詢問：舊列沒有給賣家的說明、沒有期限，轉成詢問等於編內容。migration 有任何列就失敗並列出 `report_id`，沒有就刪表 | 3ca C2、§2.3 的 W2 先例（沒有誠實的轉換就不寫）；量測：沒有任何寫入者（3c-a 起沒有 RPC，之前唯一的呼叫方是打 gateway 的舊頁，見 §0.4 G6） |
| B23 | 鎖順序 | seller 內的全序加兩層：identity 帳號列 → 店家列（帳號全部的 stint）→ 通報列 → 案件列 → **詢問列** → 措施列；**來函列**只由完成與延長鎖（`FOR UPDATE`），排在店家列之後；建立來函與延長保存先以 `stint.LockAccounts`（`account_id` 升序）鎖被指名帳號的店家列，讓它們和清理在同一組列上競爭 | D3／D4 修訂 (c) 第 3 層；`S/AGENTS.md:44`；`S/internal/stint/stint.go:137` |
| B24 | 可用性 | 詢問、來函各一個 `Availability()`；案件的 `Availability()` 加「詢問」「不轉報」「記下主管機關回覆」；API 的 `can_*` 是原因為空 | 3ca C9；前提（Will 2026-10-15） |
| B25 | 伺服器句子 | 進 seller `internal/text` 的 `staff` 段，新檔的 `apperr` 沒有中文字面值 | 3ca C24 |
| B26 | 清理與保存的競態 | 清理的 transaction 在 `RetireDueStores` 鎖住候選之後，以**同一支查詢**（`store_ids = 候選`）再判斷一次，只清兩次都成立的；寫入保存條件的一方（建立與延長來函；既有的申覆）先鎖同一組店家列。理由與證明見 §2.4 | PG 文件（G23）；審查 blocker |
| B27 | admin 上傳附件 | 沿用 admin 既有的機敏上傳：`admin/src/services/media-upload.ts` 的 `uploadImageToMedia(file, {isSecure: true})`（經單體轉給 gateway 的 `/api/v1/app/media/upload_sessions` 與 `/secure_upload/{id}`，取得 staged token）；單體以 `media.Uploader{ID: adminID, Admin: true}` 轉正（商家名單與 content 的先例）。`mediaref.FinalizeEvidence` 改收 `media.Uploader` | 量測見 G24；這是所有佐證共用的路徑（買家的通報佐證也經同一個 media-service），不是新的依賴；media-service 在 B1 才搬，屆時只換 `Media` 背後的 adapter（media `AGENTS.md`） |
| B28 | 通知被指名的賣家 | 沒有措施的來函（調取資料、B4 之前人工處理的限制商品）賣家不會知道；要不要記、怎麼記見 Q1 | rb R5「處理之後仍要告知被處置的賣家（DSA 9(5)）」；DSA 10(5) |
| B30 | 冪等鍵的範圍 | `CreateCaseInquiry`、`CreateAuthorityOrder` 的 `request_id` 範圍是 (建立的管理員, request_id)，同 `CreateMeasure` 的 `measures_request`；另一位管理員帶同一個值不會讀回別人建立的資源 | `S/db/schema.sql:501`；開工前的 design-review #6 |
| B31 | 新資料的保存期 | 本刀不加保存或刪除機制：詢問的說明、回覆與附件，主管機關對轉報的回覆，來函的摘要、處理摘要與對象，都沿用「治理資料目前沒有保存政策」（CARRYOVER 的治理佐證列）。全部列進計畫書 I-legal 的檢查清單（上線的阻擋條件，B21），觸發是 I-legal 定隱私權政策的保存期那一刀；CARRYOVER 加一列 | gov ADR「後果」的佐證保存期；個資法第 8 條（§0.5）；開工前的 design-review #4 |
| B32 | 來函的擁有者 | 暫時放在 seller（措施、認定與清理都在這裡，現在的來函幾乎都指名賣家）。觸發：第一份指名非賣家帳號的來函，或 Q4 的 identity 保存那一步開工時，重新評估來函本身（不只 hold）是否改由平台層或 identity 層的法遵模組擁有；CARRYOVER 加一列 | 單體規則 2「先檢查資料是不是放錯了擁有者」；開工前的 design-review #5 |
| B29 | 店家關閉時的詢問 | 不因關店擋下：帳號還在，條款 11.3 的義務與案件都還在，關店也不該是躲避詢問的方法。只在店家個資已清理（`pii_purged_at`）時擋下新的詢問（沒有聯絡資料可以問）；已開著的詢問在關店與清理時都不動，照常逾期、照常可以記回覆或撤回 | 審查 minor 4；條款 11.3 |

## 0.4 查證過的現況

| # | 事實 | 證據 |
|---|---|---|
| G1 | `Case.AwaitingDecision()` 只回 `Open()`、`DecisionClockStart()` 回 `OpenedAt`，註解寫 3c-b 要加詢問的條件；`CaseFacts.AwaitingReferral()` 是食安、決定採取措施、`Referral == nil` | [量測] `S/internal/model/report_case.go:274-278`、`:438-442` |
| G2 | `CaseBlock` 有 8 個值（`CLOSED`…`REFERRAL_RECORDED`）；`CaseAvailability` 的欄位是 `Decide`、`ChangeSeverity`、`LinkMeasure`、`CreateMeasure`、`RecordReferral`、`Export` | [量測] `report_case.go:343-361`、`:445-448` |
| G3 | `Decide` 的 transaction：讀案件 → 鎖店家列 → 鎖成員通報 → 鎖案件（版本）→ `Availability` → `DecideCase` → `ResolveCaseReports` 並逐筆發 `ReportResolved` | [量測] `S/internal/governance/case.go:48-121` |
| G4 | `RecordFDAReferral`：讀案件 → 鎖店家 → 鎖案件（不比版本）→ 已有紀錄回 `AlreadyRecorded` → `InsertCaseReferral`（`ON CONFLICT DO NOTHING`）→ `BumpCase` | [量測] `case.go:185-243`；`S/db/queries/cases.sql:172-183` |
| G5 | `case_referrals` 一案一列，複合外鍵 `(case_id, category)` 加 `category = 'FOOD_SAFETY'`；註解「3c-b 加追蹤日期、主管機關的回覆與不轉報」 | [量測] `S/db/schema.sql:406-420` |
| G6 | `report_cooperation_requests` 沒有 sqlc 查詢、沒有 RPC；devseed 只刪不寫；3c-a 的 migration 只讀它做檢查；`CooperationTemplate`、`RequestCooperation`、`SellerCooperationRequested` 在 backend、api、admin 的程式都沒有命中 | [量測] `rg -n report_cooperation_requests`、`rg -n 'CooperationTemplate\|RequestCooperation\|SellerCooperationRequested' backend api admin/src`（只在 `.md`） |
| G7 | `evidence_accesses_owner_check` 是 `num_nonnulls(report_id, request_id) = 1`；通報佐證以 `(report_id, submission_seq, position)`、申覆佐證以 `(request_id, position)` 定址 | [量測] `schema.sql:715-737` |
| G8 | 佐證的 admin 路由 `GET /api/files/admin/seller/reports/{report_id}/evidence/{position}` 與申覆的那條由 `NewAdminEvidence` 處理，`server.go` 掛上時宣告權限；`ReadEvidence` 先在 transaction 內寫讀取紀錄、commit 後 `mediaref.ReadSecure` | [量測] `H/seller/evidence.go:19-67`；`H/server.go:164-170`；`S/internal/governance/evidence.go:20-55` |
| G9 | 佐證的 media 引用 `mediaref.EvidenceRef(refID)`（ref type `RefTypeGovernanceEvidence`，ref ID 是通報或請求 ID）；`FinalizeEvidence` 在 transaction 外轉正並要求機敏；`EvidenceAssets` 是 `report_evidence` 與 `measure_request_evidence` 的 UNION | [量測] `S/internal/mediaref/mediaref.go:65`、`:128-133`、`:285-310`；`S/db/queries/reports.sql:273-277` |
| G10 | `RetireDueStores`：已關閉、還沒清理、`now > retire_after`、帳號沒有 legal hold（終止的待處理申覆）；清理與恢復營業共用；`retention_hours`、`appeal_window_hours` 由 Go 常數傳入；`StoreRetention = 30*24h`、`AppealWindow = 4416h` | [量測] `S/db/queries/purge.sql:1-32`；`S/internal/model/measure.go:89-104`；呼叫點 `S/internal/purge/purge.go:98`、`S/internal/closing/closing.go:328` |
| G11 | 清理的範圍：店家聯絡與地址、店名與說明、KYC 個人欄位、身分 hash、證件、PayUni 自由文字、食品登錄字號與理由；同意紀錄的 `client_ip`、`user_agent`；食品登錄審核紀錄在關店 2 年後整列刪除 | [量測] `purge.sql:34-140`；`S/AGENTS.md:8`、`:16`、`:23` |
| G12 | 詐欺犯罪危害防制條例第 37 條：第 1 項「…應採可行之技術，於合理期間保存電磁紀錄、文書、影像、物品、用戶註冊及確認身分程序所得資料、連線紀錄或交易紀錄等足以重建使用者及個別交易之有關資料。但法律另有較長保存期間規定者，從其規定。」第 2 項「…應於收受調取通知之日起三日內提供資料，並保存六個月…發生訴訟時，應延長保存至裁判確定後三個月。」第 36 條第 3 項：經通知應配合，並對涉詐帳號「於合理期間暫停提供服務」 | [外部] 全國法規資料庫 pcode `D0080226`（`law.moj.gov.tw/LawClass/LawSingle.aspx?pcode=D0080226&flno=37`；審查者讀過，與 6laws.net 的轉載一致；v1 用錯 pcode 才讀不到） |
| G13 | 現行法律文字寫「關店後 30 天」：隱私權政策「賣家的店家聯絡資料、身分認證資料與食品業者登錄字號保存至關閉商店後 30 天」；賣家身分認證告知事項第 4 點；食品業者登錄告知事項第 4 點；賣家合作條款附件「關店三十日後刪除」 | [量測] `W/website/features/legal/pages/PrivacyPage.tsx:105-110`；`W/website/docs/legal/seller-kyc-notice.md:13`；`seller-food-registration-notice.md:10`；`seller-terms.md:332` |
| G14 | `finding_reports` 是 `(violation_id, report_id)`；`NewFinding.source_report_ids` 最多 20 筆；`lockSourceReports` 在店家列之後依 id 升序鎖；`checkSourceCase` 驗同店與進行中 | [量測] `schema.sql:475-482`；`P/governance.proto:694-701`；`S/internal/measure/create.go:575-607` |
| G15 | `RiskGround` 有 `FRAUD`、`FOOD_SAFETY` 等；RISK 停權沒有認定 | [量測] `W/api/hawkr/seller/v1alpha1/seller.proto:1062-1076`；3ca F23 |
| G16 | `Case` 的欄位號到 28（19 未用、已 reserved 的說明在 3ca 附錄 D #8）；`FDAReferral` 是 `channel 1, reference_no 2, reported_by_admin_id 3, report_time 4`；`ListCasesRequest` 到 5 | [量測] `P/governance.proto:258-264`、`:355-402`、`:555-565` |
| G17 | admin 的案件頁：`DialogState` 聯集在 `A/violation-reports/cases/[id]/page.tsx:70-79`，按鈕在 `:313-376` 依 `can_*`；轉報區塊只顯示文字（`CaseSections.tsx:215-228`）；記錄轉報對話框在 `RecordFDAReferralDialog.tsx`，驗證在 `report-actions.ts:380-416` | [量測] |
| G18 | admin 還沒有共用的 `<Dialog>`；`admin-governance-modal-a11y.test.ts` 的 `GOVERNANCE_SOURCES` 要每個列出的對話框檔自己有 overlay 的 `ref`、`useFocusTrap`＋`useModalIsolation` | [量測] `admin/tests/admin-governance-modal-a11y.test.ts:23-137` |
| G19 | 工作日：單體內沒有；只有 invoice-service 的 `bizday.go` | [量測] `rg -n 'workday\|business.?day' backend/internal` 沒有命中；`backend/services/invoice-service/internal/domain/bizday.go:73-161` |
| G20 | 目前沒有任何 `ReviewQueueItem`、`ReviewPolicy` | [量測] `rg` 在 `api/`、`backend/internal` 沒有命中 |
| G21 | seller 最新的 migration 是 `20261018170000_seller_supplements_seen_seq.sql` | [量測] |
| G22 | admin 的佐證連結是 `<a href target="_blank">`，網址由 `documentSrc(viewUri)` 轉成 `/api/bff…` | [量測] `A/violation-reports/[id]/page.tsx:295-313`；`A/merchants/[id]/kyc-application.ts:660-665` |
| G23 | 清理的 transaction（`InTx`，`pool.Begin` 沒有指定隔離層級，即 PG 預設的 READ COMMITTED）先跑 `RetireDueStores`（一個 `SELECT … NOT EXISTS(申覆的 legal hold) … FOR UPDATE OF s SKIP LOCKED`），再以候選呼叫 `PurgeStores`，後者不再判斷保留期或 hold。PG 文件：READ COMMITTED 下「a SELECT query sees a snapshot of the database as of the instant the query begins to run」；`SELECT FOR UPDATE` 找到被別的 transaction 更新的列時，只對**那一列**的新版本重新判斷 WHERE，「it does not see effects of those commands on other rows in the database」；每個新語句取新的快照。所以：申覆的 transaction 只鎖店家列、不更新它（寫的是 `measure_requests`），在清理語句開始之後、掃到那一列之前 commit 時，那一列已沒有鎖（`SKIP LOCKED` 不跳過）、沒有被更新（不觸發重新判斷），即使觸發也不會重跑子查詢對 `measure_requests` 的判斷：清理照語句開始時的快照判定「沒有 hold」，接著清掉 | [量測] `S/internal/purge/purge.go` 的 `batch`；`S/db/queries/purge.sql:1-56`；`W/backend/internal/platform/db/db.go` 的 `InTx`；`S/internal/measure/appeal.go:53-110`、`measure.go:342-356`（`lockMeasure` 先 `stint.LockAccountOf` 再鎖措施）。[外部] PostgreSQL 18 文件〈Transaction Isolation〉的 Read Committed 一節。競態本身是 [推論]，沒有重現 |
| G24 | admin 的上傳只有一條：`uploadImageToMedia`，機敏檔走 `POST /api/v1/app/media/upload_sessions`（`isSecure: true`）再 `PUT /api/v1/app/media/secure_upload/{sessionId}` 直接拿 staged token（發票取消證據是目前唯一的呼叫方，註解記 dev 實測過）；單體的 media 仍是 Java media-service 的 client，B1 才改寫；以管理員身分轉正的先例是 `media.Uploader{ID: adminID, Admin: true}`（商家名單圖片、content 的圖片，都是公開檔） | [量測] `W/admin/src/services/media-upload.ts:1-60`；`W/backend/internal/modules/media/AGENTS.md`；`S/internal/prospect/images.go:108`；`W/backend/internal/modules/content/internal/images/images.go:26`；`S/internal/mediaref/mediaref.go:307`（現在只收帳號 ID） |
| G25 | `SubmitAppeal` 在 transaction 內先取 `now` 再鎖帳號的店家列（`lockMeasure`） | [量測] `S/internal/measure/appeal.go:85-88` |

## 0.5 約束帳（本刀新出現的）

| 約束 | 判定 | 來源 |
|---|---|---|
| 詐欺條例第 37 條第 1 項的保存義務 | **REAL**（現行法）；期間「合理期間」由我們定，6 個月是 Will 照建議定的起始值，上線前請律師確認 | G12；rb R5 |
| 第 37 條第 2 項：收到調取 3 日內提供、保存 6 個月、訴訟延長 | **REAL** | G12 |
| 草案第 43-1 條（2 個工作日、180 日） | REAL 的可能性高（行政院已通過，沒找到三讀），所以期限與保存做成常數加可指定 | rr §2；rb §5 |
| 現行法律文字寫 30 天 | SELF-IMPOSED：文字是我們的，系統沒有上線，沒有人依它行事；rb §4 已定在 I-legal 改（B21 的後果與觸發）。上線時才變成對外承諾，所以 I-legal 是上線的阻擋條件 | G13；rb R5 約束帳 |
| 個資法第 8 條第 1 項第 4 款「利用之期間」要告知 | REAL（上線之後）：所以 I-legal 必須在上線前完成 | [外部] 全國法規資料庫 pcode `I0050021` 第 8 條（審查者讀過） |
| gov ADR「保留的判斷式只有 `RetireDueStores` 一支」 | SELF-IMPOSED（我們的 ADR），符合基準（一條規則一個家），保留；Q2 (b) 的代價之一就是要打破它 | gov ADR「終止合作」 |
| 清理在 READ COMMITTED 下以語句快照判斷 | REAL（PG 的語意）：所以要在鎖住之後再判斷（B26） | G23 |
| 賣家端畫面在 10f | SELF-IMPOSED（我們的分段），rb §4 已核准，照做；在那之前 admin 的文案照實際行為寫（3cd D′7） | rb R4 |
| 工作日行事曆不在單體 | SELF-IMPOSED；日曆天比工作日短，對我們的期限只會更嚴，先用日曆天（rb R5 約束帳） | G19 |
| 只有一位營運人員 | REAL（缺第二方）；不做詢問的指派 | 3cd D′1 |
| Atlas 社群版略過 view 與函式 | SELF-IMPOSED（工具選擇）；本刀不新增 `pub_*`，不受影響 | D3／D4 修訂 |
| 單體規則 2：SQL 只碰自己的 schema | SELF-IMPOSED，符合基準，保留：保存只看 seller 自己的表 | `backend/AGENTS.md:28` |

---

## 1. 契約

### 1.1 Service 的變動（`P/governance.proto` 的 `SellerGovernanceAdminService`）

新訊息放在新檔 `P/case_inquiry.proto` 與 `P/authority_order.proto`（同一個 package，service 仍只在 `governance.proto`：proto 的 service 只能定義在一個檔）。

| RPC | 動作 | AIP |
|---|---|---|
| `CreateCaseInquiry` | 新：對案件的賣家發一筆詢問 | 133（`request_id`，AIP-155） |
| `ListCaseInquiries` | 新：案件的詢問，舊到新 | 132 |
| `RecordCaseInquiryReply` | 新：記下系統外收到的回覆 | 136 |
| `WithdrawCaseInquiry` | 新：撤回還沒回覆的詢問 | 136 |
| `RecordFDAReferral` | 改：加 `follow_up_date` | 136 |
| `WaiveFDAReferral` | 新：標記不轉報 | 136 |
| `RecordFDAReferralReply` | 新：記下主管機關的回覆 | 136 |
| `CreateAuthorityOrder`、`ListAuthorityOrders`、`GetAuthorityOrder` | 新 | 133（`request_id`）、132、131 |
| `CompleteAuthorityOrder` | 新：標記完成並寫處理摘要 | 136 |
| `ExtendAuthorityOrderHold` | 新：延長保存期限 | 136 |
| `ListCases` | 改：加 `awaiting_seller` 篩選 | 132 |
| `CreateMeasure` | 改：加 `source_authority_order_id`；`NewFinding` 加 `source_authority_order_ids` | 133 |
| `ListMeasures` | 改：加 `source_authority_order_id` 篩選 | 132 |

方法名用完整的資源名（`CaseInquiry`），不用 `Inquiry`：rb R4 的命名理由（避開 `MeasureRequest`）同樣適用於未來的其他詢問。

### 1.2 詢問（`P/case_inquiry.proto`）

```proto
// CaseInquiryKind 是平台要賣家做的事（step10e-reports-brief R4）。
enum CaseInquiryKind {
  CASE_INQUIRY_KIND_UNSPECIFIED = 0;
  // 提供紀錄：賣家合作條款第 11.3 條的來源、批次、效期、進出貨及流向。
  CASE_INQUIRY_KIND_RECORDS = 1;
  // 請說明。
  CASE_INQUIRY_KIND_EXPLANATION = 2;
}
// CaseInquiryState 由伺服器推導（不儲存）。逾期不是狀態：OPEN 而且現在晚於 due_time。
enum CaseInquiryState { CASE_INQUIRY_STATE_UNSPECIFIED = 0; CASE_INQUIRY_STATE_OPEN = 1; CASE_INQUIRY_STATE_REPLIED = 2; CASE_INQUIRY_STATE_WITHDRAWN = 3; }
// 回覆是怎麼收到的。10f 加 SELLER_CENTER（賣家在賣家中心回覆）。
enum CaseInquiryReplyChannel {
  CASE_INQUIRY_REPLY_CHANNEL_UNSPECIFIED = 0;
  CASE_INQUIRY_REPLY_CHANNEL_PHONE = 1;
  CASE_INQUIRY_REPLY_CHANNEL_EMAIL = 2;
  CASE_INQUIRY_REPLY_CHANNEL_MESSAGING = 3;   // 通訊軟體
  CASE_INQUIRY_REPLY_CHANNEL_OTHER = 4;
}
// 撤回的原因：管理員撤回，或案件結案（決定或沒有通報）時系統撤回。
enum CaseInquiryWithdrawCause { CASE_INQUIRY_WITHDRAW_CAUSE_UNSPECIFIED = 0; CASE_INQUIRY_WITHDRAW_CAUSE_ADMIN = 1; CASE_INQUIRY_WITHDRAW_CAUSE_CASE_CLOSED = 2; }

message CaseInquiry {
  string id = 1 [IDENTIFIER];
  string case_id = 2 [OUTPUT_ONLY];
  string store_id = 3 [OUTPUT_ONLY];
  CaseInquiryKind kind = 4 [OUTPUT_ONLY];
  // 給賣家的說明。只寫管理員的轉述：不放通報原文、佐證或通報人（R4）。
  string message = 5 [OUTPUT_ONLY];
  google.protobuf.Timestamp create_time = 6 [OUTPUT_ONLY];
  string created_by_admin_id = 7 [OUTPUT_ONLY];
  // 期限，只在 OPEN 時設（3cd §2 第 12 項：義務還在時才輸出）；開始時間是 create_time。
  google.protobuf.Timestamp due_time = 8 [OUTPUT_ONLY];
  CaseInquiryState state = 9 [OUTPUT_ONLY];
  CaseInquiryReply reply = 10 [OUTPUT_ONLY];        // 只在 REPLIED
  CaseInquiryWithdrawCause withdraw_cause = 11 [OUTPUT_ONLY];  // 只在 WITHDRAWN
  google.protobuf.Timestamp withdraw_time = 12 [OUTPUT_ONLY];
  string withdrawn_by_admin_id = 13 [OUTPUT_ONLY];  // 系統撤回時是結案的管理員
  bool can_record_reply = 14 [OUTPUT_ONLY];
  bool can_withdraw = 15 [OUTPUT_ONLY];
}
message CaseInquiryReply {
  string body = 1;                                  // 回覆內容（admin 記下的是摘錄或轉錄），1–4000 字
  repeated Evidence evidence = 2;                   // 附件，機敏檔，只經 view_uri 讀（每次讀取記錄）
  CaseInquiryReplyChannel channel = 3;
  google.protobuf.Timestamp receive_time = 4;       // 收到的時間；「待決定」從它重新起算
  google.protobuf.Timestamp record_time = 5;        // 記進系統的時間
  string recorded_by_admin_id = 6;
}

message CreateCaseInquiryRequest {
  string case_id = 1 [REQUIRED];
  CaseInquiryKind kind = 2 [REQUIRED];
  string message = 3 [REQUIRED];                    // 1–2000 字
  // 選填：期限。不設時是建立後 72 小時；設定時必須在建立後 24 小時到 14 天之間。
  google.protobuf.Timestamp due_time = 4;
  // 選填，UUID（AIP-155）：同一位管理員以同一個 request_id 重送回原本那一筆；內容不同回 invalid_argument。範圍是 (建立的管理員, request_id)，同 CreateMeasure。
  string request_id = 5;
}
message CreateCaseInquiryResponse { CaseInquiry inquiry = 1; Case case = 2; }
message ListCaseInquiriesRequest { string case_id = 1 [REQUIRED]; int32 page_size = 2; string page_token = 3; }  // 預設 50，上限 100
message ListCaseInquiriesResponse { repeated CaseInquiry inquiries = 1; string next_page_token = 2; }
message RecordCaseInquiryReplyRequest {
  string id = 1 [REQUIRED];
  CaseInquiryReplyChannel channel = 2 [REQUIRED];
  google.protobuf.Timestamp receive_time = 3 [REQUIRED];   // 不早於詢問建立、不晚於現在
  string body = 4 [REQUIRED];
  repeated string evidence_tokens = 5;              // staged token，最多 5 份
}
message RecordCaseInquiryReplyResponse { CaseInquiry inquiry = 1; Case case = 2; bool already_recorded = 3; }
message WithdrawCaseInquiryRequest { string id = 1 [REQUIRED]; }
message WithdrawCaseInquiryResponse { CaseInquiry inquiry = 1; Case case = 2; bool already_withdrawn = 3; }
```

- 回應帶 `Case`：案件的 `awaiting_seller`、`etag` 與 `decision_options` 都因詢問改變，client 不另讀。
- `RecordCaseInquiryReply` 重送：已經是 REPLIED 時回原本那一則與 `already_recorded = true`，不比較內容（先例 `RecordFDAReferral`；第二次送出的附件沒有引用，由 media 的 GC 回收，先例同 3ca §3.3）。已撤回時回 `failed_precondition`。
- `WithdrawCaseInquiry` 重送：已經是 WITHDRAWN 時回 `already_withdrawn = true`；已回覆時 `failed_precondition`。
- 檔頭註解寫明：賣家端畫面上線（10f）之前，系統不會通知賣家；期限從建立起算，10f 改成從出現在賣家中心起算。

`Case` 加欄位（`P/governance.proto`）：

```proto
  // 有還沒結束的詢問（rb R3 的「等賣家」，推導）；這時案件不在「待決定」。
  bool awaiting_seller = 29 [OUTPUT_ONLY];
  string open_inquiry_id = 30 [OUTPUT_ONLY];        // awaiting_seller 時那一筆
  bool can_create_inquiry = 31 [OUTPUT_ONLY];
  FDAReferralWaiver fda_referral_waiver = 32 [OUTPUT_ONLY];  // 與 fda_referral 至多一個
  bool can_waive_fda_referral = 33 [OUTPUT_ONLY];
  bool can_record_fda_referral_reply = 34 [OUTPUT_ONLY];
```

`ListCasesRequest` 加 `bool awaiting_seller = 6;`：true 只列有還沒結束詢問的進行中案件（和 `state = CLOSED` 一起設回 `invalid_argument`）。這是 3cd D′2 經 rb §3 改寫的「等賣家（推導）」篩選；不加進 `CaseState`，因為它和「進行中」同時成立。

### 1.3 轉報 v2（`P/governance.proto`）

```proto
message FDAReferral {
  FDAChannel channel = 1; string reference_no = 2; string reported_by_admin_id = 3; google.protobuf.Timestamp report_time = 4;
  // 追蹤日期（台北日期）；沒有時不設。到了這天還沒收到主管機關的回覆，就聯絡主管機關。
  google.type.Date follow_up_date = 5;
  repeated FDAReferralReply replies = 6;            // 舊到新
}
message FDAReferralReply {
  string body = 1;                                  // 1–2000 字
  google.type.Date receive_date = 2;                // 收到的日期（台北），不早於轉報日、不晚於今天
  string recorded_by_admin_id = 3; google.protobuf.Timestamp record_time = 4;
}
message FDAReferralWaiver { string reason = 1; string waived_by_admin_id = 2; google.protobuf.Timestamp waive_time = 3; }

message RecordFDAReferralRequest {
  string case_id = 1 [REQUIRED]; FDAChannel channel = 2 [REQUIRED]; string reference_no = 3;
  // 選填：追蹤日期，不早於今天；不設時是轉報日（台北）＋30 天。
  google.type.Date follow_up_date = 4;
}
message WaiveFDAReferralRequest { string case_id = 1 [REQUIRED]; string reason = 2 [REQUIRED]; }   // 1–2000 字
message WaiveFDAReferralResponse { Case case = 1; bool already_waived = 2; }
message RecordFDAReferralReplyRequest {
  string case_id = 1 [REQUIRED]; string body = 2 [REQUIRED]; google.type.Date receive_date = 3 [REQUIRED];
  // 下一個追蹤日期：設定時取代原本的（不早於今天）；clear_follow_up 為 true 時清掉；兩者都不設時不變。
  google.type.Date next_follow_up_date = 4;
  bool clear_follow_up = 5;
}
message RecordFDAReferralReplyResponse { Case case = 1; }
```

- 日期用 `google.type.Date`：追蹤是「哪一天」，不是時刻；client 的轉換用 `@hawkr/api/proto/date`（跨專案契約表）。
- `RecordFDAReferralReply` 只新增，不帶 etag（同註記，2c G10）；重送會多一則，接受（只有一位營運人員，重送由對話框的送出中狀態防止）。
- `RecordFDAReferralResponse.already_recorded`：已有 `WAIVED` 的列時回 `failed_precondition`（不是 already）。

### 1.4 主管機關來函（`P/authority_order.proto`）

```proto
// AuthorityOrderKind 是來函要平台做的事（rb R5）。
enum AuthorityOrderKind {
  AUTHORITY_ORDER_KIND_UNSPECIFIED = 0;
  AUTHORITY_ORDER_KIND_RESTRICT_PRODUCT = 1;   // 限制或移除商品（食安法草案第 43-1 條、食藥署請下架）
  AUTHORITY_ORDER_KIND_DATA_REQUEST = 2;       // 調取資料（詐欺犯罪危害防制條例第 37 條第 2 項）
  AUTHORITY_ORDER_KIND_SUSPEND_ACCOUNT = 3;    // 暫停帳號（同條例第 36 條第 3 項）
  AUTHORITY_ORDER_KIND_OTHER = 4;
}
enum AuthorityOrderState { AUTHORITY_ORDER_STATE_UNSPECIFIED = 0; AUTHORITY_ORDER_STATE_OPEN = 1; AUTHORITY_ORDER_STATE_COMPLETED = 2; }

message AuthorityOrder {
  string id = 1 [IDENTIFIER];
  string agency = 2 [OUTPUT_ONLY];                  // 機關名稱，1–100 字
  string document_no = 3 [OUTPUT_ONLY];             // 文號，1–100 字
  google.protobuf.Timestamp receive_time = 4 [OUTPUT_ONLY];   // 收文時間（管理員填）；期限與保存都從它起算
  AuthorityOrderKind kind = 5 [OUTPUT_ONLY];
  string summary = 6 [OUTPUT_ONLY];                 // 來函內容摘要，1–4000 字
  repeated AuthorityOrderTarget targets = 7 [OUTPUT_ONLY];
  google.protobuf.Timestamp due_time = 8 [OUTPUT_ONLY];       // 只在 OPEN 時設
  AuthorityOrderState state = 9 [OUTPUT_ONLY];
  google.protobuf.Timestamp complete_time = 10 [OUTPUT_ONLY];
  string completed_by_admin_id = 11 [OUTPUT_ONLY];
  string resolution_summary = 12 [OUTPUT_ONLY];     // 處理摘要，1–4000 字
  // 保存期限：被指名者在 seller 的個資到這時之前不清理（預設收文後 184 天；只能延長）。
  google.protobuf.Timestamp hold_until = 13 [OUTPUT_ONLY];
  bool hold_active = 14 [OUTPUT_ONLY];              // 現在早於 hold_until
  string registered_by_admin_id = 15 [OUTPUT_ONLY];
  google.protobuf.Timestamp create_time = 16 [OUTPUT_ONLY];
  string etag = 17;
  bool can_complete = 18 [OUTPUT_ONLY];
  bool can_extend_hold = 19 [OUTPUT_ONLY];
  repeated AuthorityHoldExtension hold_extensions = 20 [OUTPUT_ONLY];   // 舊到新；只有 GetAuthorityOrder 帶
  // Q1 選 (a) 時：完成時記下每個對象是否已被告知（DSA 9(5)、10(5) 的基準），見 AuthorityOrderTarget.notice。
}
message AuthorityOrderTarget {
  string account_id = 1;                            // 被指名的帳號（任何帳號）
  string store_id = 2;                              // 選填：只指名這個店家（必須屬於帳號）；空字串＝帳號全部的店家
  string store_name = 3 [OUTPUT_ONLY];              // 清理後空字串
  repeated string product_ids = 4;                  // 選填：指名的商品（限制或移除商品的來函）
  // 保存涵蓋的範圍（伺服器推導，逐家列出，不聚合；審查 r2）：帳號在 seller 沒有店家時是空的，no_seller_data 為 true。
  repeated AuthorityTargetStore covered_stores = 5 [OUTPUT_ONLY];
  bool no_seller_data = 6 [OUTPUT_ONLY];           // 買家與帳號資料的保存不在 seller（Q3）
  AuthorityTargetNotice notice = 7 [OUTPUT_ONLY];  // Q1 (a)；完成前不設
}
message AuthorityTargetStore {
  string store_id = 1; string store_name = 2;      // 清理後空字串
  AuthorityStoreCoverage coverage = 3;
}
enum AuthorityStoreCoverage {
  AUTHORITY_STORE_COVERAGE_UNSPECIFIED = 0;
  AUTHORITY_STORE_COVERAGE_HELD = 1;                       // 保存中
  AUTHORITY_STORE_COVERAGE_PURGED_BEFORE_REGISTRATION = 2; // 登記之前已依保留期限清除（§2.4 的情形 3）
  AUTHORITY_STORE_COVERAGE_HOLD_ENDED = 3;                 // 保存期限已過，個資還沒被清理
  AUTHORITY_STORE_COVERAGE_PURGED_AFTER_HOLD_ENDED = 4;    // 登記之後、保存期限已過時個資已被清除（之後延長也回不來；實作時加，見修訂 10）
  AUTHORITY_STORE_COVERAGE_HOLD_RELEASED_BY_REVOCATION = 5; // 來函已撤銷登記，保存在撤銷時解除（hold_until 可能還沒到；與個資是否已清理無關，登記時已清理者仍是 PURGED_BEFORE_REGISTRATION；修正輪 3c-b 審查加）
}
message AuthorityHoldExtension {
  google.protobuf.Timestamp from_time = 1; google.protobuf.Timestamp to_time = 2;
  AuthorityHoldExtensionCause cause = 3;
  string reason = 4;                                // ADMIN 才有
  string admin_id = 5; google.protobuf.Timestamp create_time = 6;
}
// ADMIN：管理員延長（例如訴訟）；DATA_PROVIDED：調取資料的來函完成時，依 Q5 自動延到「提供之後 6 個月」。
enum AuthorityHoldExtensionCause { AUTHORITY_HOLD_EXTENSION_CAUSE_UNSPECIFIED = 0; AUTHORITY_HOLD_EXTENSION_CAUSE_ADMIN = 1; AUTHORITY_HOLD_EXTENSION_CAUSE_DATA_PROVIDED = 2; }
// Q1 (a)：每個對象一筆（被指名者不一定是賣家）。
//   NOTIFIED：已告知（時間與方式）。
//   WITHHELD：機關要求暫不告知（例如偵查不公開）；必填 recheck_date（向機關確認的日期），到期進 3d 待辦「告知被指名者」，
//             之後以 RecordAuthorityTargetNotice 改成 NOTIFIED，或寫新的 recheck_date（每次改變留在 history）。
//   MEASURE_IS_NOTICE：只限「暫停帳號」「限制或移除商品」的來函，而且完成時這個對象已有這筆來函導出的措施（伺服器以
//             ListMeasures(source_authority_order_id) 的同一個定義驗證）；措施的理由書就是告知。「調取資料」不能選它。
message AuthorityTargetNotice {
  AuthorityTargetNoticeKind kind = 1; google.protobuf.Timestamp notify_time = 2; string note = 3;
  google.type.Date recheck_date = 4;               // 只在 WITHHELD
}
enum AuthorityTargetNoticeKind { AUTHORITY_TARGET_NOTICE_KIND_UNSPECIFIED = 0; AUTHORITY_TARGET_NOTICE_KIND_NOTIFIED = 1; AUTHORITY_TARGET_NOTICE_KIND_WITHHELD = 2; AUTHORITY_TARGET_NOTICE_KIND_MEASURE_IS_NOTICE = 3; }

message CreateAuthorityOrderRequest {
  string agency = 1 [REQUIRED]; string document_no = 2 [REQUIRED];
  google.protobuf.Timestamp receive_time = 3 [REQUIRED];   // 不晚於現在、不早於 1 年前
  AuthorityOrderKind kind = 4 [REQUIRED];
  string summary = 5 [REQUIRED];
  repeated AuthorityOrderTarget targets = 6 [REQUIRED];    // 1–20 個；同一個 (帳號, 店家) 不可重複；同一個帳號已經整個列為對象（不指定店家）時，不能再列它的店家（重疊，invalid_argument）；帳號必須存在（identity），店家必須屬於帳號
  // 選填：來函自訂的期限，收文後 1 小時到 30 天。不設時依種類：限制或移除商品、暫停帳號 48 小時；調取資料、其他 72 小時。
  google.protobuf.Timestamp due_time = 7;
  string request_id = 8;                            // 選填，AIP-155；範圍是 (登記的管理員, request_id)，同 CreateMeasure
}
message CreateAuthorityOrderResponse { AuthorityOrder order = 1; }
message ListAuthorityOrdersRequest {
  int32 page_size = 1; string page_token = 2;       // 預設 20，上限 100
  AuthorityOrderState state = 3;                    // OPEN 依期限舊到新；其他依收文時間新到舊
  string account_id = 4;
}
message ListAuthorityOrdersResponse { repeated AuthorityOrder orders = 1; string next_page_token = 2; }
message GetAuthorityOrderRequest { string id = 1 [REQUIRED]; }
message GetAuthorityOrderResponse { AuthorityOrder order = 1; }
message CompleteAuthorityOrderRequest {
  string id = 1 [REQUIRED]; string resolution_summary = 2 [REQUIRED]; string etag = 3 [REQUIRED];
  // 選填：實際提供資料的時間（只給調取資料的來函，其他種類帶了回 invalid_argument；不設時是現在）；必須在 [收文時間, 現在] 之內。
  // Q5 (b)：hold_until 延到 max(目前, 提供時間＋4416h)，記一列 DATA_PROVIDED。
  google.protobuf.Timestamp provide_time = 4;
  repeated TargetNoticeInput target_notices = 5;   // Q1 (a) 時必填：每個對象一筆，依 targets 的順序
}
message TargetNoticeInput { int32 target_position = 1; AuthorityTargetNotice notice = 2; }
// Q1 (a) 時才有：WITHHELD 的後續。
message RecordAuthorityTargetNoticeRequest { string id = 1 [REQUIRED]; int32 target_position = 2; AuthorityTargetNotice notice = 3 [REQUIRED]; string etag = 4 [REQUIRED]; }  // notice 只能是 NOTIFIED 或新的 WITHHELD
message RecordAuthorityTargetNoticeResponse { AuthorityOrder order = 1; }
}
message CompleteAuthorityOrderResponse { AuthorityOrder order = 1; }
message ExtendAuthorityOrderHoldRequest {
  string id = 1 [REQUIRED];
  google.protobuf.Timestamp hold_until = 2 [REQUIRED];     // 必須晚於目前的 hold_until
  string reason = 3 [REQUIRED];                     // 例如「進入訴訟，延長到裁判確定後 3 個月」，1–2000 字，存在延長的那一列
  string etag = 4 [REQUIRED];
}
message ExtendAuthorityOrderHoldResponse { AuthorityOrder order = 1; }
```

- **不加來函的一般內部註記**（v1 有，審查指出必要條件沒有證據）：rb R5 的來函生命週期是「登記 → 完成與處理摘要」；唯一需要稽核軌跡的可變事實是保存期限，改用結構化的延長歷史（B18）。觸發：營運第一次需要在完成之前記處理經過（例如交付資料分兩次）。
- `etag` 只給完成與延長（它們依據管理員看到的期限與狀態）。
- 延長保存不擋「已到期」：資料可能還沒被清（店家還開著，或剛到期而清理還沒跑），延長仍有意義；延長在鎖住被指名帳號的店家列之後執行（B23），每個對象的 `covered_stores` 會逐家照實顯示已清理的店家。
- 來函建立後不能改（文號、種類、對象）。登記錯了：完成並在摘要寫明，再登記正確的一筆。觸發：營運第一次需要更正。

### 1.5 措施與認定的來源

```proto
message NewFinding {
  …（1–3 不變）
  // 成立這個認定的主管機關來函，最多 5 筆、不可重複；每一筆都必須指名這個措施的帳號（不限店家時任一店家都算）。可以不填。
  repeated string source_authority_order_ids = 4;
}
message CreateMeasureRequest {
  …
  // 選填：從主管機關來函建立時帶來函 id。任何種類與依據都可以帶；來函必須指名這個措施的帳號（invalid_argument）。
  // 來函已完成也可以（措施可能在完成之後才建立，例如調取之後才發現違規）。
  string source_authority_order_id = 15;
}
message ListMeasuresRequest {
  …
  // 只列這筆來函導出的措施：source_authority_order_id 是它，或依據的認定（含引用的既有認定）引用了它。
  string source_authority_order_id = 14;
}
```

- `requestHash` 編進 `source_authority_order_id` 與 `source_authority_order_ids`（3ca §1.4 的先例）。
- 「措施的來源是主管機關」由這兩條推導，不加 `source_kind` 之類的來源類型欄位；但 `measures.source_authority_order_id` 本身是一個新欄位，**修訂了 rb §2 第 1 點「可以推導，不加欄位」**（理由見 B20，先例 3ca C20）。

### 1.6 錯誤（不新增 `ErrorReason`）

| 情形 | code | details |
|---|---|---|
| 必填沒填、列舉值不合法、字數超過、期限或時間超出範圍、`awaiting_seller` 與 `state = CLOSED` 一起設 | invalid_argument | `BadRequest` 指向欄位 |
| 來函的對象：帳號不存在、店家不屬於帳號、重複、重疊（整個帳號加上它的店家）、超過 20 個、商品超過 50 個；建立措施帶的來函沒有指名這個帳號；`source_authority_order_ids` 同上 | invalid_argument | `targets[i].account_id`／`.store_id`／`.product_ids`；`targets`；`source_authority_order_id`；`new_finding.source_authority_order_ids[i]` |
| Q1 (a) 時：完成沒帶某個對象的告知、`NOTIFIED` 沒帶時間、`WITHHELD` 沒帶確認日期、`MEASURE_IS_NOTICE` 用在調取資料或這個對象還沒有導出的措施；`provide_time` 用在非調取資料或超出 [收文, 現在] | invalid_argument | `target_notices[i]`；`provide_time` |
| 同一個 `request_id` 內容不同 | invalid_argument | `request_id` |
| 可用性不成立（案件已結案、已有進行中的詢問、詢問已回覆或已撤回、不是食安、已有轉報或不轉報紀錄、沒有轉報紀錄就記回覆、以 `NO_REPORTS` 關閉、來函已完成、新的保存期限不晚於目前） | failed_precondition | 只有 `LocalizedMessage`（第一個原因的句子；3ca §1.7 的先例） |
| etag 不符 | aborted | `ErrorInfo{VERSION_CONFLICT}`，`metadata.conflictTarget = authority_order`（新值，寫進 `ErrorReason.VERSION_CONFLICT` 的註解） |
| 案件、詢問、來函不存在 | not_found | — |

### 1.7 權限（`adminperm`、`adminauth`）

| 方法 | 權限 |
|---|---|
| `CreateCaseInquiry`、`ListCaseInquiries`、`RecordCaseInquiryReply`、`WithdrawCaseInquiry` | `seller.case_inquiries.create`、`.list`、`.record_reply`、`.withdraw` |
| 詢問附件的檔案路由 | `seller.case_inquiry_evidence.get` |
| `WaiveFDAReferral`、`RecordFDAReferralReply` | `seller.cases.waive_fda_referral`、`seller.cases.record_fda_referral_reply` |
| `CreateAuthorityOrder`、`ListAuthorityOrders`、`GetAuthorityOrder`、`CompleteAuthorityOrder`、`ExtendAuthorityOrderHold` | `seller.authority_orders.create`、`.list`、`.get`、`.complete`、`.extend_hold` |
新增 12 個（Q1 選 (a) 時加 `RecordAuthorityTargetNotice` 的 `seller.authority_orders.record_target_notice`，共 13 個），都進 `Catalog()`，OWNER 擁有全部（一支方法一個權限，`TestEachPermissionBelongsToOneMethod`）。

---

## 2. Schema 與 migration

慣例同 3ca §2：只有 up；表由 `atlas migrate diff` 產生，在 DDL 之間手寫轉換，重算 `atlas.sum`；沒有 `pub_*`；時間戳晚於 rebase 當下 main 最新的一支（目前 G21）；`schema.sql` 由 `TestSchemaFileMatchesMigrations` 守。

### 2.1 詢問（`<ts>_seller_case_inquiries.sql`）

```sql
-- 詢問（rb R4）：平台對案件的賣家的一次要求，有期限、一則回覆、可撤回；每個案件同時最多一筆還沒結束的。
-- 回覆在 3c-b 只由管理員記下（系統外收到）；10f 加賣家在賣家中心的回覆（channel = SELLER_CENTER、replied_by_account_id）。
CREATE TABLE seller.case_inquiries (
    id                     UUID PRIMARY KEY,
    case_id                UUID NOT NULL,
    store_id               BIGINT NOT NULL,
    kind                   TEXT NOT NULL,
    message                TEXT NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL,
    created_by_admin_id    BIGINT NOT NULL,
    due_at                 TIMESTAMPTZ NOT NULL,
    request_id             UUID,
    request_hash           BYTEA,
    reply_body             TEXT,
    reply_channel          TEXT,
    reply_received_at      TIMESTAMPTZ,
    reply_recorded_at      TIMESTAMPTZ,
    reply_recorded_by_admin_id BIGINT,
    withdrawn_at           TIMESTAMPTZ,
    withdrawn_by_admin_id  BIGINT,
    withdraw_cause         TEXT,
    CONSTRAINT case_inquiries_case_fkey FOREIGN KEY (case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT case_inquiries_kind_check CHECK (kind IN ('RECORDS', 'EXPLANATION')),
    CONSTRAINT case_inquiries_message_check CHECK (char_length(message) BETWEEN 1 AND 2000),
    CONSTRAINT case_inquiries_due_check CHECK (due_at > created_at),
    CONSTRAINT case_inquiries_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT case_inquiries_request_key UNIQUE (created_by_admin_id, request_id),   -- 冪等鍵的範圍同 measures_request（schema.sql:501）
    CONSTRAINT case_inquiries_reply_check CHECK (
      (reply_body IS NULL AND reply_channel IS NULL AND reply_received_at IS NULL AND reply_recorded_at IS NULL AND reply_recorded_by_admin_id IS NULL) OR
      (char_length(reply_body) BETWEEN 1 AND 4000 AND reply_channel IN ('PHONE', 'EMAIL', 'MESSAGING', 'OTHER')
       AND reply_received_at >= created_at AND reply_recorded_at >= reply_received_at AND reply_recorded_by_admin_id > 0)) IS TRUE,
    CONSTRAINT case_inquiries_withdraw_check CHECK (
      (withdrawn_at IS NULL AND withdrawn_by_admin_id IS NULL AND withdraw_cause IS NULL) OR
      (withdrawn_at >= created_at AND withdrawn_by_admin_id > 0 AND withdraw_cause IN ('ADMIN', 'CASE_CLOSED'))) IS TRUE,
    CONSTRAINT case_inquiries_terminal_check CHECK (reply_received_at IS NULL OR withdrawn_at IS NULL),
    CONSTRAINT case_inquiries_admin_check CHECK (created_by_admin_id > 0)
);
CREATE UNIQUE INDEX case_inquiries_one_open ON seller.case_inquiries (case_id) WHERE reply_received_at IS NULL AND withdrawn_at IS NULL;
CREATE INDEX case_inquiries_case ON seller.case_inquiries (case_id, created_at, id);
-- 3d「詢問逾期」的來源：還沒結束的詢問依期限。
CREATE INDEX case_inquiries_open_due ON seller.case_inquiries (due_at, id) WHERE reply_received_at IS NULL AND withdrawn_at IS NULL;

-- 回覆的附件（機敏，同通報佐證）：只在記下回覆的 transaction 寫入，之後只讀。
CREATE TABLE seller.case_inquiry_reply_evidence (
    inquiry_id UUID NOT NULL REFERENCES seller.case_inquiries (id),
    position   INTEGER NOT NULL,
    asset_id   TEXT NOT NULL,
    object_key TEXT NOT NULL,
    PRIMARY KEY (inquiry_id, position),
    CONSTRAINT case_inquiry_reply_evidence_position_check CHECK (position BETWEEN 0 AND 4),
    CONSTRAINT case_inquiry_reply_evidence_asset_check CHECK (asset_id <> '' AND object_key <> '')
);

-- 讀取紀錄的擁有者加詢問的附件。
ALTER TABLE seller.evidence_accesses ADD COLUMN inquiry_id UUID,
    ADD CONSTRAINT evidence_accesses_inquiry_fkey FOREIGN KEY (inquiry_id, position) REFERENCES seller.case_inquiry_reply_evidence (inquiry_id, position),
    DROP CONSTRAINT evidence_accesses_owner_check,
    ADD CONSTRAINT evidence_accesses_owner_check CHECK (num_nonnulls(report_id, request_id, inquiry_id) = 1);
CREATE INDEX evidence_accesses_inquiry ON seller.evidence_accesses (inquiry_id, accessed_at DESC) WHERE inquiry_id IS NOT NULL;
```

- 回覆放在詢問列上而不是另一張表：一筆詢問一則回覆（B6），一列一個事實，CHECK 直接保證「回覆與撤回互斥」；10f 改成多則回覆時才拆表（觸發：賣家要能補交）。
- `store_id` 是案件店家的複本，由複合外鍵保證相等：附件的 media 引用與 10f 賣家端「這是不是我的店」的授權都要它，不必 join 案件（先例 `measures.account_id`）。
- `mediaref.EvidenceRef` 的 ref ID 加詢問 ID（同一個 ref type，ID 是 UUIDv7 不會撞）；`EvidenceAssets` 的 UNION 加這張表，`currentAssets` 才會保留它的引用。
- 10f 會改的：`reply_channel` 加 `SELLER_CENTER`、加 `replied_by_account_id`（與 `reply_recorded_by_admin_id` 二選一）、`due_at` 的起算。

### 2.2 轉報 v2（`<ts>_seller_case_referral_v2.sql`）

```sql
ALTER TABLE seller.case_referrals
    ADD COLUMN outcome TEXT NOT NULL DEFAULT 'REFERRED',
    ALTER COLUMN channel DROP NOT NULL,
    ADD COLUMN follow_up_on DATE,
    ADD COLUMN waiver_reason TEXT,
    ADD CONSTRAINT case_referrals_outcome_check CHECK (
      (outcome = 'REFERRED' AND channel IS NOT NULL AND waiver_reason IS NULL) OR
      (outcome = 'WAIVED' AND channel IS NULL AND reference_no IS NULL AND follow_up_on IS NULL
       AND char_length(waiver_reason) BETWEEN 1 AND 2000)) IS TRUE;
ALTER TABLE seller.case_referrals ALTER COLUMN outcome DROP DEFAULT;
-- 從零的欄名：兩種都是「記下的時間與管理員」，所以改名，不沿用只描述轉報的舊名（上線前沒有相容性約束）。
ALTER TABLE seller.case_referrals RENAME COLUMN referred_at TO recorded_at;
ALTER TABLE seller.case_referrals RENAME COLUMN admin_id TO recorded_by_admin_id;
-- 既有列一律 REFERRED，follow_up_on 為 NULL（轉換不編日期）。

-- 主管機關對轉報的回覆（只新增；移送通知與結果通知可以是兩則）。
ALTER TABLE seller.case_referrals ADD CONSTRAINT case_referrals_outcome_key UNIQUE (case_id, outcome);   -- 回覆的複合外鍵
CREATE TABLE seller.case_referral_replies (
    id          UUID PRIMARY KEY,
    case_id     UUID NOT NULL,
    -- 固定是 REFERRED 的複本：複合外鍵帶它，DB 保證回覆只掛在已轉報的列（同 case_referrals 以 (case_id, category) 加 CHECK 只掛食安案件的寫法，schema.sql:406-420）。
    outcome     TEXT NOT NULL DEFAULT 'REFERRED',
    body        TEXT NOT NULL,
    received_on DATE NOT NULL,
    admin_id    BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT case_referral_replies_body_check CHECK (char_length(body) BETWEEN 1 AND 2000),
    CONSTRAINT case_referral_replies_admin_check CHECK (admin_id > 0),
    CONSTRAINT case_referral_replies_outcome_check CHECK (outcome = 'REFERRED'),
    CONSTRAINT case_referral_replies_referral_fkey FOREIGN KEY (case_id, outcome) REFERENCES seller.case_referrals (case_id, outcome)
);
CREATE INDEX case_referral_replies_case ON seller.case_referral_replies (case_id, recorded_at, id);
```

- 「回覆只能掛在 REFERRED 的列」由 DB 保證（開工前的 design-review #3）：外鍵不能帶常數，所以回覆存一欄固定為 `REFERRED` 的 `outcome`（CHECK），以 `(case_id, outcome)` 複合外鍵指向 `case_referrals` 的 `UNIQUE (case_id, outcome)`；`case_referrals` 一案一列，所以這個 UNIQUE 不改變任何既有的唯一性。模組的 `Availability` 照樣先擋（給句子），DB 是保底。`TestSchemaConstraints` 加「回覆掛在 WAIVED 的列」的反例。
- `received_on` 不早於 `recorded_at` 的台北日期：同上，模組檢查。sqlc 的 `InsertCaseReferral`、`CaseReferrals`、`toReferral`（`S/internal/governance/governance.go:176`）跟著改欄名。
- 3d「待轉報」的判定式不變（`NOT EXISTS case_referrals`）：「不轉報」也是一列，所以自然離開待轉報（3ca §6 的「3c-b 加不轉報」就是這樣滿足）。

### 2.3 主管機關來函（`<ts>_seller_authority_orders.sql`）

```sql
-- 主管機關來函（rb R5）：獨立的收件，不是通報。期限與保存都從收文時間起算；完成不解除保存。
CREATE TABLE seller.authority_orders (
    id                     UUID PRIMARY KEY,
    agency                 TEXT NOT NULL,
    document_no            TEXT NOT NULL,
    received_at            TIMESTAMPTZ NOT NULL,
    kind                   TEXT NOT NULL,
    summary                TEXT NOT NULL,
    due_at                 TIMESTAMPTZ NOT NULL,
    hold_until             TIMESTAMPTZ NOT NULL,
    registered_at          TIMESTAMPTZ NOT NULL,
    registered_by_admin_id BIGINT NOT NULL,
    completed_at           TIMESTAMPTZ,
    completed_by_admin_id  BIGINT,
    resolution_summary     TEXT,
    version                BIGINT NOT NULL DEFAULT 1,
    request_id             UUID,
    request_hash           BYTEA,
    CONSTRAINT authority_orders_kind_check CHECK (kind IN ('RESTRICT_PRODUCT', 'DATA_REQUEST', 'SUSPEND_ACCOUNT', 'OTHER')),
    CONSTRAINT authority_orders_text_check CHECK (char_length(agency) BETWEEN 1 AND 100 AND char_length(document_no) BETWEEN 1 AND 100
                                                  AND char_length(summary) BETWEEN 1 AND 4000),
    CONSTRAINT authority_orders_times_check CHECK (registered_at >= received_at AND due_at > received_at AND hold_until > received_at),
    CONSTRAINT authority_orders_complete_check CHECK (
      (completed_at IS NULL AND completed_by_admin_id IS NULL AND resolution_summary IS NULL) OR
      (completed_at >= registered_at AND completed_by_admin_id > 0 AND char_length(resolution_summary) BETWEEN 1 AND 4000)) IS TRUE,
    CONSTRAINT authority_orders_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT authority_orders_request_key UNIQUE (registered_by_admin_id, request_id),   -- 同上
    CONSTRAINT authority_orders_admin_check CHECK (registered_by_admin_id > 0),
    CONSTRAINT authority_orders_version_check CHECK (version >= 1)
);
CREATE INDEX authority_orders_open_due ON seller.authority_orders (due_at, id) WHERE completed_at IS NULL;   -- 3d「主管機關來函」與清單 OPEN
CREATE INDEX authority_orders_received ON seller.authority_orders (received_at DESC, id DESC);

-- 指名的對象：任何帳號（存在與否由模組問 identity；不建跨 schema FK），選填店家（NULL＝帳號全部的 stint）。店家有值時複合外鍵保證它屬於帳號。
CREATE TABLE seller.authority_order_targets (
    order_id    UUID NOT NULL REFERENCES seller.authority_orders (id),
    position    SMALLINT NOT NULL,
    account_id  BIGINT NOT NULL,
    store_id    BIGINT,
    product_ids TEXT[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (order_id, position),
    CONSTRAINT authority_order_targets_store_fkey FOREIGN KEY (store_id, account_id) REFERENCES seller.stores (store_id, account_id),
    CONSTRAINT authority_order_targets_position_check CHECK (position BETWEEN 0 AND 19),
    CONSTRAINT authority_order_targets_products_check CHECK (cardinality(product_ids) <= 50),
    CONSTRAINT authority_order_targets_account_check CHECK (account_id > 0)
);
-- PG 15 起的 NULLS NOT DISTINCT（本機與 CI 是 PG 18），不用 COALESCE 的哨兵值。
CREATE UNIQUE INDEX authority_order_targets_unique ON seller.authority_order_targets (order_id, account_id, store_id) NULLS NOT DISTINCT;
-- 保存的判斷（RetireDueStores）：依帳號找有效的指名。
CREATE INDEX authority_order_targets_account ON seller.authority_order_targets (account_id, order_id);

-- 認定引用來函（比照 finding_reports；只在建立措施的 transaction 寫入）。
CREATE TABLE seller.finding_authority_orders (
    violation_id UUID NOT NULL REFERENCES seller.violations (id),
    order_id     UUID NOT NULL REFERENCES seller.authority_orders (id),
    PRIMARY KEY (violation_id, order_id)
);
CREATE INDEX finding_authority_orders_order ON seller.finding_authority_orders (order_id);

ALTER TABLE seller.measures ADD COLUMN source_authority_order_id UUID REFERENCES seller.authority_orders (id);
CREATE INDEX measures_source_authority_order ON seller.measures (source_authority_order_id) WHERE source_authority_order_id IS NOT NULL;

-- 保存期限的延長歷史（只新增）：保存期限是來函唯一可變的法律欄位，每次改變留下前後值、原因與作者。
CREATE TABLE seller.authority_order_hold_extensions (
    order_id   UUID NOT NULL REFERENCES seller.authority_orders (id),
    seq        SMALLINT NOT NULL,
    from_until TIMESTAMPTZ NOT NULL,
    to_until   TIMESTAMPTZ NOT NULL,
    cause      TEXT NOT NULL,
    reason     TEXT,
    admin_id   BIGINT NOT NULL,          -- DATA_PROVIDED 是完成來函的管理員
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (order_id, seq),
    CONSTRAINT authority_order_hold_extensions_order_check CHECK (to_until > from_until AND seq >= 1),
    CONSTRAINT authority_order_hold_extensions_cause_check CHECK (
      (cause = 'ADMIN' AND char_length(reason) BETWEEN 1 AND 2000) OR (cause = 'DATA_PROVIDED' AND reason IS NULL)) IS TRUE,
    CONSTRAINT authority_order_hold_extensions_admin_check CHECK (admin_id > 0)
);
-- Q1 (a) 時：authority_order_targets 加 notice（NOTIFIED／WITHHELD／MEASURE_IS_NOTICE）、notified_at、notice_note、recheck_on（DATE），
-- CHECK：NOTIFIED ⇔ notified_at 有值；WITHHELD ⇔ recheck_on 有值；另加只新增的 authority_target_notice_history（改變前的值、誰、何時）。
-- 「完成 ⇔ 每個對象都有 notice」跨兩張表，由模組在完成的 transaction 內保證並以資料集測試守。
-- 3d 的待辦「告知被指名者」：notice = WITHHELD 而且 recheck_on <= 今天（台北），開始時間 recheck_on，期限 recheck_on＋3 天（起始值，Q6）。
```

- 「`authority_orders.hold_until` 等於最後一列延長的 `to_until`（沒有延長時等於預設值）」由單一寫入路徑（延長與完成）與資料集測試守（J5），同措施的目前值與版本表（gov ADR「措施的目前值」）。

- `product_ids` 用陣列而不是子表：它只是記下來函寫了哪些商品，沒有自己的生命週期，也不被查詢；B4 加商品限制措施時，措施引用來函，不引用這個陣列。
- 「指名帳號」的驗證（措施的帳號、認定的來函）在模組：`EXISTS target WHERE account_id = m.account_id AND (store_id IS NULL OR store_id = m.store_id)`。注意 `store_id` 指定時只算那一家：同帳號的新 stint 不在指名內。

### 2.4 保存（`RetireDueStores` 與 `StoreRetention`）

```sql
-- RetireDueStores 加一個參數與一條（開工前的 design-review #1：保存只擋刪除，不改變業務結果）：
--   保留期是否已過（retire_after 與申覆的 legal hold）仍是唯一的式子，清理與恢復營業共用；
--   來函的保存只在 apply_preservation_hold = true（清理）時加上，恢復營業傳 false。
  AND (NOT sqlc.arg(apply_preservation_hold)::boolean OR NOT EXISTS (
    SELECT 1 FROM seller.authority_order_targets t
    JOIN seller.authority_orders o ON o.id = t.order_id
    WHERE t.account_id = s.account_id AND (t.store_id IS NULL OR t.store_id = s.store_id)
      AND o.hold_until > sqlc.arg(now)::timestamptz))
-- 食品登錄審核紀錄的 2 年刪除加同一條（訴訟延長可能超過 2 年），並改成同一支查詢呼叫兩次（見下）。
```

- `model.StoreRetention` 720 小時改成 4416 小時（B21）；`StoreRetentionHours` 跟著變，`retire_after` 的式子不變。終止關店的 `GREATEST(關店＋保留, 最晚的終止開始＋申覆期)`：兩個時數現在相等，第二項只在關店之後才建立的終止（21.3 事由、關聯終止）比較晚時起作用，照留。
- 判斷式仍只有一支，但回答兩個問題，由參數分開：
  - **保留期已過**（`apply_preservation_hold = false`）：`retire_after` 已過而且沒有申覆的 legal hold。恢復營業（`closing.go:328` 的 `ReopenPlan`）只問它。既有的申覆 hold 留在這一層，因為它代表爭議還沒結束，店家應該還能恢復（gov ADR「終止合作」；審查查過這兩種 hold 都符合這個前提）。
  - **可以清理**（`apply_preservation_hold = true`）：保留期已過，**而且**沒有有效的來函保存。只有清理問它。
  - 為什麼分開：來函的保存是第三方（警察、食藥署）的保存義務，和賣家的爭議無關。若它也影響恢復，一家保留期已滿、只因為被調取資料而還沒清的店，在終止被撤銷時會被恢復營業，同樣條件下沒有來函的店只會撤銷紀錄：恢復與否取決於一份無關的來函。業界的 legal hold 只凍結刪除、不改變使用者的狀態（Microsoft Purview、Google Vault 的 hold 只保存內容）。所以：**被保存的店，保留期滿之後和沒有保存的店一樣，撤銷終止時只撤銷紀錄、不恢復營業；它只是還沒被清理。**
  - gov ADR「判斷式只有 `RetireDueStores` 一支」改寫成「保留期的式子只有一份；保存只加在清理那一層」（3c-b-3 的 ADR 修訂段）。

**清理與保存的競態（B26；v1 的說法錯了，審查 blocker）**

v1 說「建立來函先鎖店家列，所以和清理串行」。在 READ COMMITTED 下不成立（G23）：清理的 `RetireDueStores` 是一個語句，hold 的 `NOT EXISTS` 用的是語句開始時的快照；來函的 transaction 若在語句開始之後、掃到那一列之前 commit，那一列已經沒有鎖、也沒有被更新，清理看不到 hold，接著 `PurgeStores` 不再判斷就清掉。既有的申覆 legal hold 是同一個洞（下面的「既有缺陷」）。

修法：鎖住之後以新的語句再判斷一次。

```go
// purge.batch（改）：
due := q.RetireDueStores(now, store_ids = NULL)          // 第一次：找候選並 FOR UPDATE SKIP LOCKED 鎖住
recheck := q.RetireDueStores(now, store_ids = candidates) // 第二次：新語句、新快照；候選已鎖在本 transaction
purge(only recheck)                                        // 只清兩次都成立的
```

證明（寫入 hold 的一方都先鎖同一組店家列）：
1. 寫入方在清理的第一個語句掃到那一列之前就拿到鎖：第一個語句 `SKIP LOCKED` 跳過，這一輪不清。
2. 寫入方在第一個語句開始之後、掃到那一列之前 commit：第二個語句開始時它已經 commit，新快照看得到 hold，不清。這就是 v1 的洞，由第二次判斷關上。
3. 寫入方在清理鎖住那一列之後才要鎖：它等到清理 commit。這時資料在寫入方登記之前已經依保留期限清掉，是登記太晚，不是競態。寫入方鎖到之後重讀 `pii_purged_at`，那一家店標 `PURGED_BEFORE_REGISTRATION`（§1.4），營運知道要回覆機關「資料已依保留期限刪除」。
4. 寫入方在第一個與第二個語句之間 commit：不可能。那時店家列已被清理鎖住，寫入方拿不到鎖，就沒有寫入。

第二次呼叫與第一次是同一支查詢（兩次都 `apply_preservation_hold = true`），所以述詞只有一份；第二次的 `FOR UPDATE` 鎖的是本 transaction 已經持有的列，不會等待。恢復營業傳 `false`，來函的保存對它沒有作用，所以上面四種情形的證明與恢復無關；恢復與清理之間仍由申覆的 hold 與店家列的鎖串行（3c-b-0 已落地）。

**既有缺陷：申覆的 legal hold（main 上，與 3c-b 無關）**
- 同一個洞：`SubmitAppeal` 鎖帳號的店家列（`lockMeasure` → `stint.LockAccountOf`）、插入 `measure_requests`，不更新店家列（G23、G25）。證據等級：PG 文件加程式碼的推論，沒有重現。
- 實際的時間窗很窄：清理要在申覆期限剛過的那一刻跑，而申覆要在期限前一刻送出（終止關店的 `retire_after` 通常就等於最晚的申覆期限）。窄，但不是零，而且被清的是不可逆的個資。
- 還有第二個窗（情形 3 的反方向）：清理先鎖住、申覆等鎖；`SubmitAppeal` 的 `now` 在鎖之前取（G25），等到清理 commit 之後仍以期限前的時間通過可用性檢查，在已清理的店家上留下待處理的申覆。修法不依賴時鐘：`submitRequest` 的申覆在鎖住之後重讀措施所在店家的 `pii_purged_at`，**只在措施是終止合作、而且店家目前這次關閉是它造成的**（`store_closures` 目前那一列 `closed_by = TERMINATION` 且 `closed_by_measure_id` 是這筆措施）時，已清理就回「申覆期限已過」（`APPEAL_WINDOW_ENDED`，句子沿用）。理由：`RetireDueStores` 對 `closed_by = TERMINATION` 的 `retire_after` 是 `greatest(關店後 30 天, 帳號上未撤銷終止中最晚的申覆期限)`，清理只在 `now > retire_after` 時發生，所以造成關店的那筆（未撤銷）終止的期限必已過；其他措施這個蘊含不成立，照常申覆（審查 3c-b-0：先前對所有措施檢查，停權／警告沒有申覆期限，賣家自己關店 30 天後被清理，帳號層級的停權仍生效卻再也不能申覆；預告期間內自己關店〔`closed_by = ACCOUNT_OWNER`，保留期只有 30 天〕清理時終止的期限也可能還沒到；`TestSuspensionAppealStillWorksAfterTheSelfClosedStoreWasPurged`、`TestNoticeTerminationAppealStillWorksAfterTheSelfClosedStoreWasPurged`）。帳號上別筆終止（關店之後才發現的 21.3，`starts_at` 可能晚於清理）也不套用這個檢查，只認造成關店的那一筆。只看措施自己的店家，不看帳號全部的 stint（帳號早先自己關閉、依 30 天清理的舊 stint 不代表這筆措施的期限已過；`TestAppealStillWorksWhenAnOlderStintOfTheAccountWasPurged`）。同時把 `now` 移到鎖之後，讓可用性的時間判斷也晚於清理。多實例的時鐘差不再影響結果（審查 r2）。
- 第三個窗：**關聯終止的來源申覆**。legal hold 的條件含「帳號上未撤銷的關聯終止，其來源有待處理申覆」，但對來源措施送申覆的 transaction 只鎖來源帳號的店家列，被關聯帳號的清理不和它競爭，再判斷也擋不住在途的那一筆。窗很窄（被關聯終止建立在來源之後，它的 `retire_after` 嚴格晚於來源的申覆期限），但被清的是不可逆的個資，所以修。做法照 gov ADR「撤銷涉及終止時」的先例：
  1. 鎖之前不加鎖預讀：以這個措施為來源、未撤銷的 `LINKED_ACCOUNT` 終止所在的帳號。
  2. 以 `stint.LockAccounts`（來源帳號 ∪ 被關聯帳號，`account_id` 升序，跨帳號的鎖序同 gov ADR）鎖住全部帳號的店家列，再鎖措施。
  3. 鎖後重讀被關聯帳號的集合；變了（剛好有新的關聯終止）整個 transaction 重來（`retryScope`，`measure` 套件既有的）。
  4. 被關聯帳號的店家已清理不擋來源的申覆（來源帳號的資料還在；被關聯帳號的資料在申覆之前就依期限清了，是情形 3）。
  這樣三種 hold 的寫入方（終止的申覆、關聯終止來源的申覆、來函）都先鎖被保護帳號的店家列，§2.4 的證明對它們都成立。
- 子表只清再判斷之後的名單：`PurgeStores` 收再判斷的結果，KYC、PayUni、食品登錄等子表的清理本來就只作用在店家列已清理的列（`purge.go` 的註解），所以名單一致。
- 修在獨立的第一個 commit（§8 的 3c-b-0），先有會失敗的測試（§7.3）。

**刪除 2 年期滿的食品登錄審核紀錄（`DeleteExpiredFoodRegistrations`）**
- 現況［量測：`purge.sql:140-153`］：一個 `DELETE … WHERE id IN (SELECT … FOR UPDATE OF f SKIP LOCKED)`，鎖的是 `food_registrations` 的列、不是店家列，條件只有時間（關店超過 2 年、已清過個資）。
- **main 上沒有洞**：今天的條件沒有任何 hold，只看單調遞增的時間，而且只作用在個資已清理的店家（已清理的店家不能恢復營業、也不會再有申覆，G10），沒有寫入方能讓條件變嚴。
- **本刀加 hold 之後就有洞**（審查 r2 major）：來函指名的店家不和它競爭同一組列，`SKIP LOCKED` 的情形 1 不成立，情形 2 的快照差也在。
- 修法：和清理一樣，**同一支查詢呼叫兩次**（開工前的 design-review #2：一個述詞一個定義，不用測試鎖住兩份）。
  ```sql
  -- name: ExpiredFoodRegistrations :many
  -- 2 年期滿、可以刪除的食品登錄審核紀錄：時間條件與來函保存的條件只寫在這裡。candidate_ids 為 NULL 時找候選並鎖住；
  -- 給定時只回其中仍然成立的（新語句、新快照，同 RetireDueStores 的再判斷）。
  SELECT f.id FROM seller.food_registrations f JOIN seller.stores s ON … JOIN seller.store_closures c ON …
  WHERE <時間條件> AND <來函保存的條件>
    AND (sqlc.narg(candidate_ids)::uuid[] IS NULL OR f.id = ANY(sqlc.narg(candidate_ids)::uuid[]))
  ORDER BY f.id LIMIT @batch_limit FOR UPDATE OF f, s SKIP LOCKED;
  -- name: DeleteFoodRegistrations :execrows（不帶條件，只刪再判斷回來的 id）
  DELETE FROM seller.food_registrations WHERE id = ANY(@ids);
  ```
  第一次鎖住登錄列與店家列；來函的寫入方先鎖同一組店家列，§2.4 的四種情形照樣成立。來函保存的述詞因此一共兩份（店家清理在 `RetireDueStores`、這裡），兩份對象不同（店家個資、審核紀錄），寫進 CARRYOVER 的沿用盤點，由同一份資料集測試鎖住兩者對「哪些來函算有效」一致；要合成一份得用 SQL 函式，Atlas 社群版略過函式（SELF-IMPOSED，§0.5），觸發：seller 第一個 SQL 函式（`pub_*`）落地時一併重估。

### 2.5 配合要求的退場（`<ts>_seller_drop_cooperation_requests.sql`）

```sql
-- 舊的配合要求沒有給賣家的說明與期限，不能誠實地轉成詢問（W2 先例）；有列就失敗並列出通報 ID，本機照 3ca §2.3 重建後重跑。
DO $$ DECLARE ids text; BEGIN
  SELECT string_agg(DISTINCT report_id::text, ', ' ORDER BY report_id::text) INTO ids FROM seller.report_cooperation_requests;
  IF ids IS NOT NULL THEN RAISE EXCEPTION '舊的要求賣家配合沒有對應的詢問內容，請重建本機資料庫或人工刪除後重跑：%', ids; END IF;
END $$;
DROP TABLE seller.report_cooperation_requests;
```

- 量測（G6）：沒有任何寫入者。8g 到 3c-a 之間唯一的呼叫方是舊通報頁，它打 gateway 的 REST（寫的是舊 seller-service 的 DB），所以單體的表只可能有手動的 Connect 呼叫或測試寫的列［推論］。CI 的 DB 是新建的。
- 刪 `devseed.go:98` 的那一句、`seller_integration_test.go:894` 的 CHECK 列。
- **3c-b-1 的一個步驟**（不是「照留」）：3c-a 的 migration 測試在 `migration_integration_test.go:1441`、`:1637`、`:1717`、`:1751` 寫入與讀取這張表。先讀測試 helper 怎麼決定套用到哪一版：只套到 3c-a 那支就照留；會套到最新的，就把那幾個案例改成在本刀的 migration 之前斷言，並加本刀 migration 自己的測試（§7.5）。

### 2.6 查詢（sqlc）

`db/queries/inquiries.sql`（新）
- `InsertCaseInquiry`（`ON CONFLICT (request_id) DO NOTHING`；`case_inquiries_one_open` 的衝突不在這裡處理：指令在鎖住案件之後先以 `Availability` 擋下，UNIQUE 只是保底，撞到就整筆失敗，3ca C26）、`CaseInquiryByRequestID`、`CaseInquiryByID`、`LockCaseInquiry`、`CaseInquiries(case_id, after)`、`OpenCaseInquiries(case_ids)`（清單的 `awaiting_seller` 與 `open_inquiry_id`）、`RecordCaseInquiryReply`（`WHERE reply_received_at IS NULL AND withdrawn_at IS NULL`）、`WithdrawCaseInquiry`、`WithdrawOpenCaseInquiry(case_id, cause, admin, now)`（結案用，`RETURNING id`）、`InsertCaseInquiryEvidence`、`CaseInquiryEvidence(inquiry_ids)`、`CaseDecisionClockStarts(case_ids)`（`max(reply_received_at)`、`max(withdrawn_at) FILTER (WHERE withdraw_cause = 'ADMIN')`）。

`db/queries/cases.sql`（改）
- `AdminCases` 加 `awaiting_seller` 條件（`EXISTS` 還沒結束的詢問）。
- `InsertCaseReferral` 加 `outcome`、`follow_up_on`；`InsertCaseReferralWaiver`（`ON CONFLICT DO NOTHING`）；`UpdateCaseReferralFollowUp`；`InsertCaseReferralReply`、`CaseReferralReplies(case_ids)`。

`db/queries/authority_orders.sql`（新）
- `InsertAuthorityOrder`（`ON CONFLICT (request_id) DO NOTHING`）、`AuthorityOrderByRequestID`、`InsertAuthorityOrderTargets`、`AuthorityOrderByID`、`AuthorityOrderTargets(order_ids)`（JOIN 店名）、`AdminAuthorityOrders`（OPEN：keyset `(due_at, id)` 舊到新；其他：`(received_at, id)` 新到舊；`account_id` 篩選 `EXISTS target`）、`CompleteAuthorityOrder`、`ExtendAuthorityOrderHold`（都帶 `version = @expected`）、`OrdersNamingAccount(order_ids, account_id, store_id)`（建立措施的驗證）。

`db/queries/measures.sql`（改）
- `InsertMeasure` 加 `source_authority_order_id`；`InsertFindingAuthorityOrders`。
- 三支措施清單加 `source_authority_order_id` 的條件：`m.source_authority_order_id = @id OR EXISTS (measure_findings mf JOIN finding_authority_orders fa ON fa.violation_id = mf.violation_id WHERE mf.measure_id = m.id AND fa.order_id = @id)`；定義與 `AuthorityOrderMeasureCounts`（來函詳情不需要計數，只列清單，所以不加這支）一致由資料集測試鎖住（同 3ca C20 的四處 SQL 規則，這裡是三處）。
- `InsertAuthorityHoldExtension`、`AuthorityHoldExtensions(order_ids)`；`AuthorityTargetCoverage(order_ids, now)`（每個對象的店家數、已清理數）。
- `purge.sql`：`RetireDueStores` 不改形狀，`purge.batch` 以候選再呼叫一次（§2.4）。

`devseed.go` 的 `clearChildrenSQL`：加 `case_inquiry_reply_evidence`、`case_inquiries`、`case_referral_replies` 的刪除；`evidence_accesses` 的那一句加詢問；來函只在指名種子店家的帳號時才是種子的子表——`authority_order_targets` 依 `store_id`／`account_id` 刪，`authority_order_hold_extensions` 與 `authority_orders` 刪掉沒有對象的列，`finding_authority_orders`、`measures.source_authority_order_id` 在措施之前清（`TestEverySellerTableHasASeedDecision` 會擋漏掉的表）。

### 2.7 DB 保證不了、由模組保證並以資料集測試守的不變量

- J1 案件已結案 ⇒ 沒有還沒結束的詢問（`DecideCase` 與 `NO_REPORTS` 關閉在同一個 transaction 撤回）。
- J2 詢問的回覆與附件只在同一個 transaction 寫入；附件數 ≤ 5（CHECK 是位置上界）。
- J3 `received_on` 不早於轉報日（「只掛在 REFERRED 的列」改由 DB 保證，§2.2）。
- J4 措施的 `source_authority_order_id` 與它依據的認定所引用的來函，都指名了措施的帳號。
- J5 `hold_until` 只增不減，而且等於最後一列延長的 `to_until`（沒有延長時是 `received_at + AuthorityHold`）；只有 `ExtendAuthorityOrderHold` 與調取資料的 `CompleteAuthorityOrder` 改它，同一個 transaction 寫延長的一列。
- J6 被清理的店家、被刪除的食品登錄審核紀錄，清理或刪除當下都沒有任何 `hold_until > now` 的指名，也沒有申覆的 legal hold（§2.4 的修法保證，§7.3 的固定交錯測試守）。

---

## 3. 模組設計

### 3.1 model（`S/internal/model/case_inquiry.go`、`authority_order.go` 新，`report_case.go` 改）

```go
type InquiryKind string            // RECORDS、EXPLANATION
type InquiryReplyChannel string    // PHONE、EMAIL、MESSAGING、OTHER
type InquiryWithdrawCause string   // ADMIN、CASE_CLOSED
const (
	InquiryDefaultDue = 72 * time.Hour
	InquiryMinDue     = 24 * time.Hour
	InquiryMaxDue     = 14 * 24 * time.Hour
	InquiryOverdueGrace = 24 * time.Hour   // 3d：詢問逾期的期限＝due_at＋24 小時（rb R3）
	ReferralFollowUpDays = 30
)

type CaseInquiry struct {
	ID, CaseID uuid.UUID; StoreID int64; Kind InquiryKind; Message string
	CreatedAt time.Time; CreatedByAdminID int64; DueAt time.Time
	Reply *InquiryReply; Withdrawn *InquiryWithdrawal
}
func (i CaseInquiry) Open() bool          // 沒有回覆也沒有撤回
func (i CaseInquiry) State() InquiryState
// Overdue 是 3d 的「詢問逾期」：還沒結束而且 now >= DueAt。開始時間是 DueAt，期限是 DueAt + InquiryOverdueGrace。
func (i CaseInquiry) Overdue(now time.Time) bool
// ClockRestart 是這筆詢問讓「待決定」重新起算的時間：回覆收到的時間，或管理員撤回的時間；系統撤回（結案）與還沒結束的是 nil。
func (i CaseInquiry) ClockRestart() *time.Time

type InquiryFacts struct { Inquiry CaseInquiry; CaseOpen bool }
func (f InquiryFacts) Availability() InquiryAvailability   // RecordReply、Withdraw []InquiryBlock
```

`Case` 加 `OpenInquiryID *uuid.UUID`、`LastClockRestart *time.Time`（`CaseDecisionClockStarts`）、`Referral` 換成 `*CaseReferral{Outcome, FDA *FDAReferral（含 FollowUpOn *civil.Date、Replies）, Waiver *ReferralWaiver}`：

```go
func (c Case) AwaitingSeller() bool     { return c.Open() && c.OpenInquiryID != nil }
func (c Case) AwaitingDecision() bool   { return c.Open() && c.OpenInquiryID == nil }
func (c Case) DecisionClockStart() time.Time  // max(OpenedAt, LastClockRestart)
// AwaitingReferral 不變：Referral == nil（不轉報也是一筆 Referral）。
```

`CaseAvailability` 加 `CreateInquiry`、`WaiveReferral`、`RecordReferralReply []CaseBlock`；`CaseBlock` 加 `INQUIRY_OPEN`、`STORE_PURGED`、`REFERRAL_WAIVED`、`NO_REFERRAL`；`CaseFacts` 加 `StorePurged bool`。

`AuthorityOrder`、`AuthorityOrderTarget`、`AuthorityOrderKind`、`DefaultAuthorityDue(kind) time.Duration`（48h／72h）、`AuthorityHold = 4416 * time.Hour`（與 `AppealWindow` 同值但是另一個常數：理由不同，一個改了另一個不必跟著改）、`(o AuthorityOrder) Open()`、`HoldActive(now)`、`(t AuthorityOrderTarget) Coverage(stores, now)`、`Availability()`（`Complete`：`COMPLETED`；`ExtendHold`：沒有原因，新值的比較在指令裡做，因為它依賴輸入）、`HoldAfterProvision(provideTime)`（Q5 (b)）。

規則表（`Availability` 的內容；原因依序，錯誤取第一個）：

| 動作 | 原因（依序） |
|---|---|
| 案件：詢問賣家 | `CLOSED`、`STORE_PURGED`（店家個資已清理，沒有聯絡資料；只是關店不擋，B29）、`INQUIRY_OPEN` |
| 案件：記錄轉報 | 3c-a 的 `NOT_FOOD_SAFETY`、`NO_REPORTS`、`REFERRAL_RECORDED`；加 `REFERRAL_WAIVED`（不是 already，回錯誤） |
| 案件：不轉報 | `NOT_FOOD_SAFETY`、`NO_REPORTS`、`REFERRAL_RECORDED`、`REFERRAL_WAIVED`（後者在指令裡是 already） |
| 案件：記下主管機關的回覆 | `NOT_FOOD_SAFETY`、`NO_REFERRAL`（沒有轉報紀錄）、`REFERRAL_WAIVED` |
| 詢問：記下回覆 | `WITHDRAWN`、`REPLIED`（指令裡是 already） |
| 詢問：撤回 | `REPLIED`、`WITHDRAWN`（指令裡是 already） |
| 來函：完成 | `COMPLETED` |
| 來函：延長保存 | 沒有（已完成也能延長：訴訟在完成之後才發生） |

- 案件的「結案」不因有還沒回覆的詢問而擋下（rb R2：結案時自動撤回）。
- 「記下回覆」不看案件是否進行中：案件結案時詢問已被撤回，原因就是 `WITHDRAWN`；結案後才收到的回覆寫在案件的內部註記。

### 3.2 指令：transaction 與鎖順序

全部在 `S/internal/governance`（`inquiry.go`、`referral.go`、`authority.go` 新）。鎖順序（B23）：identity 帳號列（需要時）→ 店家列（`stint.LockAccountOf`）→ 通報列 → 案件列 → 詢問列 → 措施列。

| 指令 | transaction 內的步驟 | 寫入 | 事件 |
|---|---|---|---|
| `CreateCaseInquiry` | 有 `request_id` 先查重送（同 `CreateMeasure`，內容相同回原本那一筆，不同回 `invalid_argument`）→ 讀案件 → 鎖店家 → 鎖案件 → `caseFactsOf`（含店家狀態與還沒結束的詢問）→ `Availability().CreateInquiry` | `case_inquiries`；`BumpCase` | `CaseInquiryCreated{InquiryID, CaseID, StoreID, AccountID, Kind, DueAt}` |
| `RecordCaseInquiryReply` | transaction 外：驗證、轉正附件（`FinalizeEvidence`，uploader 是管理員）→ transaction：讀詢問 → 鎖店家 → 鎖案件 → 鎖詢問 → `InquiryFacts.Availability().RecordReply`（REPLIED 回原本那一則）→ `receive_time` 在 `[created_at, now]` | 回覆欄位、`case_inquiry_reply_evidence`、`mediaref.Changed(EvidenceRef(inquiryID))`；`BumpCase` | — |
| `WithdrawCaseInquiry` | 讀詢問 → 鎖店家 → 鎖案件 → 鎖詢問 → `Availability().Withdraw` | 撤回欄位（`ADMIN`）；`BumpCase` | — |
| `DecideCase`（改） | 照 G3；在鎖案件之後鎖還沒結束的詢問（最多一筆） | 加 `WithdrawOpenCaseInquiry(CASE_CLOSED)` | 不變 |
| `MoveReport`（改） | 原案件因沒有成員而以 `NO_REPORTS` 關閉時，同上撤回 | 同上 | 不變 |
| `RecordFDAReferral`（改） | 照 G4；`follow_up_date` 不設時算轉報日（台北，`platform/twtime`）＋30 天 | `outcome = REFERRED`、`follow_up_on` | — |
| `WaiveFDAReferral` | 讀案件 → 鎖店家 → 鎖案件 → `Availability().WaiveReferral`（已 WAIVED 回原本的） | `case_referrals`（`WAIVED`）；`BumpCase` | — |
| `RecordFDAReferralReply` | 讀案件 → 鎖店家 → 鎖案件 → `Availability().RecordReferralReply` → 日期檢查 | `case_referral_replies`；`follow_up_on` 依請求更新或清掉；`BumpCase` | — |
| `CreateAuthorityOrder` | 無狀態驗證 → 有 `request_id` 先查重送 → transaction 外問 identity 每個帳號是否存在（`invalid_argument`）→ transaction：`stint.LockAccounts(帳號, account_id 升序)`（沒有店家的帳號沒有列可鎖，也沒有資料要保存）→ 鎖住之後讀每個帳號的店家：指定的店家不屬於帳號 → `invalid_argument` → 插入 → 讀回每個對象逐家的涵蓋狀態（`PURGED_BEFORE_REGISTRATION` 就是 §2.4 的情形 3） | `authority_orders`、`authority_order_targets` | — |
| `CompleteAuthorityOrder` | `stint.LockAccounts(被指名的帳號)` → 鎖來函（版本）→ `Availability().Complete` → Q5 (b)：調取資料時算 `HoldAfterProvision`，晚於目前就延長 | 完成欄位（Q1 (a) 時每個對象的告知，`MEASURE_IS_NOTICE` 在鎖內以導出措施的定義驗證）、版本＋1；延長時 `hold_until` 與一列 `DATA_PROVIDED` | — |
| `ExtendAuthorityOrderHold` | `stint.LockAccounts(被指名的帳號)` → 鎖來函（版本）→ 新值晚於目前 | `hold_until`、版本＋1；一列 `ADMIN` 延長（前值、新值、原因） | — |
| `CreateMeasure`（改） | 照現行；帶 `source_authority_order_id` 或 `NewFinding.source_authority_order_ids` 時，在 `lockTarget`（帳號 stint）之後讀來函與對象（不鎖：來函建立後對象不變），驗證指名了措施的帳號 | `measures.source_authority_order_id`、`finding_authority_orders` | 照現行 |

- `RecordCaseInquiryReply` 鎖店家列：結案（`DecideCase`）先鎖店家列，所以「結案撤回」與「記下回覆」在店家列上串行，不會出現結案後才寫進回覆的詢問（J1）。
- 建立、完成、延長來函鎖帳號的店家列是為了和清理競爭同一組列（§2.4 的證明要「寫入 hold 的一方都先鎖店家列」）；鎖序是店家列 → 來函列，同 B23。不檢查店家狀態（已關閉、已清理的店家也可以被指名：清理過的資料已經不在，來函照樣要登記與回覆，逐家的涵蓋狀態照實顯示）。
- `RecordCaseInquiryReply` 的附件由 `FinalizeEvidence(ctx, media, media.Uploader{ID: adminID, Admin: true}, tokens)` 轉正（B27）；轉正回報不是機敏時拒絕，同通報佐證。
- 撞唯一鍵後不在同一個 transaction 繼續下 SQL（規則 3）：`request_id` 與轉報都用 `ON CONFLICT DO NOTHING`，`case_inquiries_one_open` 只是保底。

### 3.3 事件

```go
// CaseInquiryCreated（rb §3 的改名，取代已刪除的 SellerCooperationRequested）在建立詢問的 transaction 內發出。
// A8 訂閱它通知賣家；不帶給賣家的說明全文（通知只說「平台有一則詢問，請到賣家中心查看」，內容在 10f 的讀取裡）。
type CaseInquiryCreated struct { OccurredAt time.Time; EventID, InquiryID, CaseID uuid.UUID; StoreID, AccountID int64; Kind InquiryKind; DueAt time.Time }
```
- 照現行：`db.InTx` 內 `events.Publish`，型別在 `model`，`S/events.go` re-export；沒有訂閱者（gov ADR「後果」）。
- 來函、轉報、回覆、撤回都不發事件：沒有消費者，讀取時都推導得到（單體規則 4 的必要條件不成立）。

### 3.4 讀取

- `CaseInquiries(caseID)`、`AuthorityOrder(id)`、`AdminAuthorityOrders` 只由 `H/seller` 的 admin handler 呼叫，照 3ca C26：外層自己開 `InReadSnapshot`，本體在 `db.Conn(ctx)` 上（3d 的來源可以重用）。
- `Case(id)`、`AdminCases` 每頁加兩支固定查詢：`OpenCaseInquiries(case_ids)`、`CaseDecisionClockStarts(case_ids)`；`CaseReferrals` 帶回覆（`CaseReferralReplies(case_ids)`）。
- 詢問附件的 `view_uri` 由 `H/seller/governance.go` 的 URI builder 組（同 G8）。
- `AuthorityOrderTarget.store_name` JOIN `seller.stores`；`hold_active` 用請求的 `now`。

### 3.5 伺服器句子（seller `internal/text` 的 `staff` 段）

句子與 key 見 §5.4 (e)，`make uitext` 生成。新檔的 `apperr` 沒有中文字面值。

---

## 4. httpapi（`H/seller`）

- `governance.go`：`GovernanceModule` 介面加 §1.1 的方法；`toCase` 加 §1.2 的欄位；`toCaseInquiry`、`toAuthorityOrder`。
- 欄位對應（`rpc.FieldPaths`）：`CreateCaseInquiry`（`Kind`→`kind`、`Message`→`message`、`DueAt`→`due_time`、`RequestID`→`request_id`）；`RecordCaseInquiryReply`（`Channel`→`channel`、`ReceivedAt`→`receive_time`、`Body`→`body`、`EvidenceTokens[i]`→`evidence_tokens[i]`）；`WaiveFDAReferral`（`Reason`→`reason`）；`RecordFDAReferralReply`（`Body`、`ReceivedOn`→`receive_date`、`NextFollowUp`→`next_follow_up_date`）；`RecordFDAReferral` 加 `FollowUpOn`→`follow_up_date`；`CreateAuthorityOrder`（`Targets[i].AccountID`→`targets[i].account_id` 等）；`ExtendAuthorityOrderHold`（`HoldUntil`→`hold_until`、`Reason`→`reason`）；`CreateMeasure` 加 `SourceAuthorityOrderID`、`NewFinding.SourceAuthorityOrderIDs[i]`；`ListCases` 加 `AwaitingSeller`。
- page token 綁篩選（`rpc.FilterScope`）：`ListCases` 加 `awaiting_seller`；`ListAuthorityOrders` 綁 state、account；`ListMeasures` 加 source_authority_order。
- 檔案路由：`H/seller/evidence.go` 加 `EvidenceInquiryRoute = "GET /api/files/admin/seller/case_inquiries/{inquiry_id}/evidence/{position}"`，`server.go` 宣告 `seller.case_inquiry_evidence.get`；`ReadEvidence` 的擁有者加詢問（`evidenceAt` 多一個分支）。
- `adminauth.go`：§1.7 的對應；啟動檢查擋漏掉或多餘的宣告。
- `EnumMismatches` 加 `CaseInquiryKind`、`CaseInquiryReplyChannel`、`CaseInquiryWithdrawCause`、`AuthorityOrderKind`；推導的 `CaseInquiryState`、`AuthorityOrderState` 以表格測試對 `State()`、`Open()`。
- 日期：`google.type.Date` 經 `rpc` 既有的轉換（`civil.Date`）［推論：實作時確認 `H/rpc` 已有 Date 的 helper，沒有就加一支，不在 handler 手寫］。

---

## 5. admin UI

版面、流程與文案都是新的，照根目錄規則要先取得同意（rb 狀態列：3c-b 開工前提一次案件頁的文案）。重用：3c-a 的案件頁區塊與 `run()`、`DialogState`、`GovernanceNoticeBanner`、`NotesSection`、`DeadlineText`（`admin/src/lib/deadline.ts`）、`DetailBits` 的 `Field`／`Section`、`SourceReportsField` 的形狀、措施頁的分頁籤與 keyset 分頁、佐證連結（G22）、`CreateMeasureDialog` 的 `initial`。對話框骨架見 §0 的前提題（共用 `<Dialog>` 已落地就用，否則照 3c-a；全部加進 a11y 測試的 `GOVERNANCE_SOURCES`）。

### 5.1 資訊架構與檔案

| 路徑 | 內容 |
|---|---|
| `/admin/violation-reports/cases/[id]` | 案件詳情：加「詢問賣家」區塊（在「通報」之後）、轉報區塊加追蹤日期、主管機關回覆與「不轉報」 |
| `/admin/violation-reports`（案件分頁籤） | 狀態篩選加「等賣家」：全部｜進行中｜等賣家｜已結案（3cd D′2 經 rb §3 改寫，已核准）；狀態欄在等賣家時顯示「等賣家」 |
| `/admin/authority-orders` | 新：主管機關來函，分頁籤 處理中｜已完成；每 30 秒自動更新 |
| `/admin/authority-orders/new` | 新：登記來函（表單頁：對象有多列，對話框放不下） |
| `/admin/authority-orders/[id]` | 新：來函詳情 |

| 檔案 | 動作 |
|---|---|
| `A/violation-reports/cases/[id]/page.tsx`、`CaseSections.tsx` | 加 `CaseInquiriesSection`、轉報區塊改寫；`DialogState` 加 `inquiry`、`reply`、`withdraw`、`waive`、`referralReply` |
| `A/violation-reports/cases/[id]/CreateCaseInquiryDialog.tsx`、`RecordInquiryReplyDialog.tsx`、`WithdrawInquiryDialog.tsx`、`WaiveFDAReferralDialog.tsx`、`RecordFDAReferralReplyDialog.tsx` | 新 |
| `A/violation-reports/cases/[id]/RecordFDAReferralDialog.tsx` | 加追蹤日期（預設轉報日＋30 天，可改） |
| `A/violation-reports/inquiry-text.ts` | 新：admin 單端表 `CaseInquiryKind`、`CaseInquiryState`、`CaseInquiryReplyChannel`、`CaseInquiryWithdrawCause`（`Record<Exclude<E, UNSPECIFIED>, string>`） |
| `A/violation-reports/report-actions.ts`、`fda-export.ts` | 驗證與請求組裝加 5 種；轉報資料加「以平台名義轉報」一句（§5.4 (d)） |
| `A/authority-orders/page.tsx`、`new/page.tsx`、`[id]/page.tsx`、`CompleteAuthorityOrderDialog.tsx`、`ExtendHoldDialog.tsx`、`authority-text.ts`、`authority-actions.ts` | 新 |
| `A/merchants/[id]/CreateMeasureDialog.tsx`、`CreateRiskMeasureDialog.tsx`、`CreateTerminationDialog.tsx`、`measure.ts`、`violation-measure.ts` | 加 `sourceAuthorityOrderId?`（違規依據的另帶 `initial.sourceAuthorityOrderIds`）；request key 帶上 |
| 上傳附件 | `uploadImageToMedia(file, { isSecure: true })`（B27、G24：經單體的 gatewayproxy 到 media-service 的 `upload_sessions`＋`secure_upload`，這條路徑在發票取消證據上 dev 實測過）。admin 呼叫 gateway 要 `legacy.gateway.forward`（`backend/AGENTS.md` 的 adminauth），OWNER 已有。**還沒驗證的一點**：media-service 對 `Admin: true` 的上傳者轉正機敏 staged token、ref type `RefTypeGovernanceEvidence` 是否回 `is_secure = true`。現有的 `Admin: true` 先例都是公開檔。所以 3c-b-1 的第一步是用 `backend-e2e-verify` 在本機打一次這條鏈（上傳 → `RecordCaseInquiryReply` → 檔案路由讀回位元組）；不通時先在 3c-b-1 補 media 的入口，不改成公開檔。media-service 是 B1 才搬的模組，這是所有佐證共用的既有依賴 |
| `W/admin/src/components/Layout/Sidebar.tsx` | 「賣家 / 商品」組，「商家通報」之後加「主管機關來函」 |

### 5.2 頁面與狀態

**案件詳情：詢問賣家**
- 位置：「通報」區塊之後、「移動紀錄」之前（rb R4 的詢問是案件處理的一步，放在通報旁邊）。
- 內容：`ListCaseInquiries(case_id)` 舊到新；每筆一張卡：種類・狀態、建立（管理員・時間）、期限（OPEN 時用 `DeadlineText`，開始時間是 `create_time`）、給賣家的說明、回覆（管道、收到的時間、記下的時間、內容、附件連結）或撤回（原因、時間）。
- 區塊頂端的固定說明（3cd D′7 的句子照 rb R4）。
- 按鈕：頁首「詢問賣家」（`can_create_inquiry`）；每筆的「記下回覆」（`can_record_reply`）、「撤回」（`can_withdraw`）。
- 讀取失敗顯示「詢問讀取失敗」加重試，不影響其他區塊。
- 等賣家時，頁首的狀態旁加「等賣家」標記；結案對話框在 `awaiting_seller` 時加一句提示（不擋送出）。

**案件詳情：轉報主管機關**（只在食安）
- 已轉報：3c-a 的那一句＋「追蹤日期 {日期}」（今天晚於追蹤日期而還沒有回覆時，日期用 warning）＋主管機關的回覆列表（收到的日期・內容）。
- 不轉報：「已於 {時間} 標記不轉報」＋原因。
- 按鈕：頁首「記錄已轉報主管機關」（`can_record_fda_referral`）、「不轉報」（`can_waive_fda_referral`）；區塊內「記下主管機關的回覆」（`can_record_fda_referral_reply`）。

**主管機關來函（清單）**
- 處理中：期限舊到新；欄位：機關｜文號｜種類｜指名的店家｜期限（`DeadlineText`，開始時間是收文時間）｜收文時間。
- 已完成：收文時間新到舊；欄位：機關｜文號｜種類｜指名的店家｜完成時間｜保存到。
- 頁首「登記來函」（連到 `/new`）。

**登記來函（表單頁）**
- 欄位：機關、文號、收文時間（預設現在）、種類（選了之後期限欄顯示預設值，可改）、內容摘要、對象（每列：賣家帳號 ID、店家（選填，帳號的店家下拉，讀 `ListStores(account_id)`）、商品 ID（選填，逗號分隔））、期限。
- 送出成功導向詳情頁；`request_id` 在頁面開啟時產生、內容改了就換新的（同 `CreateMeasureDialog`）。

**來函詳情**
- 區塊：來函（機關、文號、收文時間、種類、內容摘要、期限）；對象（帳號、商品 ID；逐家列出店家，連到商家頁，各自標涵蓋狀態）；處理（狀態、完成、處理摘要；Q1 (a) 時每個對象的告知與暫不告知的確認日期）；保存（保存到 {時間}、保存中／已到期、延長歷史）；導出的措施（`ListMeasures(source_authority_order_id)`）。沒有內部註記（§1.4）。
- 按鈕：完成（`can_complete`）、延長保存（`can_extend_hold`）、建立措施／建立風險措施／終止合作（對象只有一個帳號時直接開；多個時先選帳號；帶 `sourceAuthorityOrderId`，違規依據的帶 `initial.sourceAuthorityOrderIds`）。
- 「限制或移除商品」的來函：B4 之前，處理方式寫在完成的摘要（例如「已於商家頁強制下架商品 X」）；不新增按鈕。

成功不另外提示（3cd #71）；版本衝突重讀並顯示衝突句、對話框保留輸入（`runMeasureAction`）；`already_*` 顯示橫幅。

### 5.3 對話框

新的 7 個：詢問賣家、記下回覆、撤回詢問、不轉報、記下主管機關的回覆、完成來函、延長保存。驗證與請求組裝在 `*-actions.ts`（字數用 `@hawkr/api/text/server-text`，日期用 `@hawkr/api/proto/date`）。選項（種類、管道）只列 admin 單端表的值。

### 5.4 文案表（一次核准，全部 **待核准**）

來源欄：**沿用**（已上線或 3c-a 已核准的原文）、**改寫**（3cd 已核准、照 rb 改寫的）、**新提案**（我的建議，要你核准）。用語照術語表：「措施」不用「處置」；後台用「通報」、對賣家與買家用「檢舉」（3ca Q6 (a)）；「記錄」作動詞（3cd §2 第 5 項）。

**(a) 案件詳情：詢問賣家**

| # | 位置 | zh-TW（建議） | 來源 |
|---|---|---|---|
| I1 | 區塊標題 | 詢問賣家 | 新提案 |
| I2 | 區塊說明（固定） | 這裡只記下詢問；賣家端的畫面上線前，系統不會通知賣家，請另外聯絡賣家。詢問只寫你轉述的內容，不要附上通報原文或通報人的資料。 | 改寫（3cd D′7 的句子照 rb R4） |
| I3 | 頁首按鈕 | 詢問賣家 | 新提案 |
| I4 | 空狀態 | 還沒有詢問 | 新提案 |
| I5 | 卡片標頭 | {種類}・{狀態} | 新提案 |
| I6 | 種類 | 提供紀錄｜請說明 | 新提案（rb R4 原文） |
| I7 | 狀態 | 等賣家回覆｜已回覆｜已撤回 | 新提案 |
| I8 | 欄位 | 建立｜期限｜給賣家的說明 | 新提案 |
| I9 | 回覆的欄位 | 回覆｜管道｜收到的時間｜記下｜附件 | 新提案 |
| I10 | 管道 | 電話｜Email｜通訊軟體｜其他 | 新提案 |
| I11 | 附件項目 | 附件 {n} | 新提案（句型同 B14「佐證 {n}」） |
| I12 | 撤回 | 已於 {時間} 撤回（{原因}） | 新提案 |
| I13 | 撤回原因 | 管理員撤回｜案件結案時撤回 | 新提案 |
| I14 | 卡片按鈕 | 記下回覆｜撤回 | 新提案 |
| I15 | 讀取失敗 | 詢問讀取失敗（加「重試」） | 新提案（句型同 A19） |
| I16 | 頁首狀態旁的標記 | 等賣家 | 新提案（rb §3 的 D′2 改寫） |
| I17 | 結案對話框的提示（`awaiting_seller`） | 還有一筆詢問在等賣家回覆；結案會撤回它。 | 新提案 |

**(b) 對話框：詢問、回覆、撤回**

| # | 位置 | zh-TW（建議） | 來源 |
|---|---|---|---|
| J1 | 詢問：標題 | 詢問賣家 | 新提案 |
| J2 | 詢問：欄位 | 種類｜給賣家的說明｜期限 | 新提案 |
| J3 | 詢問：種類的說明 | 提供紀錄：商品的來源、批次、效期、進出貨及流向（賣家合作條款第 11 條第 3 項）。請說明：請賣家說明事實或處理情形。 | 新提案 |
| J4 | 詢問：期限的說明 | 預設 3 天後；可以設在 1 天到 14 天之間。 | 新提案 |
| J5 | 詢問：送出 | 記下詢問 | 新提案（照 3cd D′7「記下要求」的句型） |
| J6 | 詢問：沒填說明 | 請填寫給賣家的說明。 | 新提案 |
| J7 | 回覆：標題 | 記下賣家的回覆 | 新提案 |
| J8 | 回覆：欄位 | 管道｜收到的時間｜回覆內容｜附件（選填，最多 5 份） | 新提案 |
| J9 | 回覆：說明 | 記下在系統外收到的回覆。記下之後這筆詢問就結束；要再問，請新增一筆詢問。 | 新提案 |
| J10 | 回覆：送出 | 記下回覆 | 新提案 |
| J11 | 回覆：已記過（橫幅） | 這筆詢問先前已記下回覆，這次沒有再記一筆。 | 新提案（句型同 3cd #68） |
| J12 | 撤回：標題與說明 | 撤回詢問｜撤回之後賣家不必再回覆。賣家端的畫面上線前，請另外告知賣家。 | 新提案 |
| J13 | 撤回：送出 | 撤回 | 新提案 |
| J14 | 撤回：已撤回（橫幅） | 這筆詢問先前已撤回。 | 新提案 |
| J15 | 失敗（網路層） | 記下失敗，沒有寫入；請確認網路狀態後再試一次。 | 新提案（句型同 3cd #69） |

**(c) 轉報主管機關**

| # | 位置 | zh-TW（建議） | 來源 |
|---|---|---|---|
| K1 | 已轉報 | 已於 {時間} 記錄轉報（{管道}，案號 {no}）；沒有案號時省略 | 沿用（3c-a） |
| K2 | 追蹤日期 | 追蹤日期 {日期}；過了而還沒有回覆時 warning | 新提案 |
| K3 | 主管機關的回覆 | 主管機關的回覆｜{日期}：{內容}｜還沒有回覆 | 新提案 |
| K4 | 不轉報 | 已於 {時間} 標記不轉報：{原因} | 新提案 |
| K5 | 按鈕 | 不轉報｜記下主管機關的回覆 | 新提案 |
| K6 | 記錄轉報：新欄位 | 追蹤日期（預設 30 天後） | 新提案 |
| K7 | 不轉報：標題、欄位、說明 | 不轉報｜原因｜例如主管機關已經介入。標記之後不能再記錄轉報。 | 新提案（「主管機關已經介入」是 rb R5 原文） |
| K8 | 不轉報：送出 | 標記不轉報 | 新提案 |
| K9 | 回覆：標題、欄位 | 記下主管機關的回覆｜收到的日期｜內容｜下一個追蹤日期（選填）｜不再追蹤 | 新提案 |
| K10 | 轉報資料 `.md` 開頭加一句 | 本轉報由 Hawkr 平台以平台名義提出；通報人的聯絡方式不在本檔。 | 新提案（rb R5「以平台名義轉報」、3cd D′8） |

**(d) 主管機關來函**

| # | 位置 | zh-TW（建議） | 來源 |
|---|---|---|---|
| L1 | 側欄與頁面標題 | 主管機關來函 | 新提案（rb R5 原文） |
| L2 | 頁面說明 | 主管機關的來函：限制或移除商品、調取資料、暫停帳號等。來函指名的帳號或店家，資料會保存到保存期限，不會被關店後的清理刪除。每 30 秒自動更新。 | 新提案 |
| L3 | 分頁籤 | 處理中｜已完成 | 新提案 |
| L4 | 欄位（處理中） | 機關｜文號｜種類｜指名的店家｜期限｜收文時間 | 新提案 |
| L5 | 欄位（已完成） | 機關｜文號｜種類｜指名的店家｜完成時間｜保存到 | 新提案 |
| L6 | 種類 | 限制或移除商品｜調取資料｜暫停帳號｜其他 | 新提案（rb R5 原文） |
| L7 | 指名多家時 | {第一家} 等 {n} 家；只指名帳號時：帳號 {id} 的全部店家 | 新提案 |
| L8 | 空狀態 | 沒有處理中的來函｜尚無來函 | 新提案 |
| L9 | 頁首按鈕、登記頁標題 | 登記來函 | 新提案 |
| L10 | 登記：欄位 | 機關｜文號｜收文時間｜種類｜內容摘要｜指名的對象｜期限 | 新提案 |
| L11 | 登記：對象的欄位 | 賣家帳號 ID｜店家（選填，不選＝這個帳號的全部店家）｜商品 ID（選填，以逗號分隔） | 新提案 |
| L12 | 登記：期限的說明 | 依種類預設：限制或移除商品、暫停帳號 48 小時；調取資料、其他 3 天。來函寫了期限時，照來函填。 | 新提案 |
| L13 | 登記：保存的說明 | 登記之後，指名對象在平台的店家與認證資料會保存到收文後 184 天；進入訴訟時可以延長。 | 新提案 |
| L14 | 登記：送出 | 登記 | 新提案 |
| L15 | 詳情：區塊 | 來函｜指名的對象｜處理｜保存｜導出的措施 | 新提案 |
| L16 | 詳情：保存 | 保存到 {時間}（保存中）｜保存到 {時間}（已到期） | 新提案 |
| L17 | 詳情：按鈕 | 完成｜延長保存｜建立措施｜建立風險措施｜終止合作 | 後三個沿用（3c-a 的入口文字），前兩個新提案 |
| L18 | 完成：標題、欄位、說明 | 完成來函｜處理摘要｜寫下做了什麼、何時回覆機關。完成之後保存期限不變。 | 新提案 |
| L19 | 延長保存：標題、欄位、說明 | 延長保存｜保存到｜原因｜只能延長。例如進入訴訟時，延長到裁判確定後 3 個月。每次延長會留下前後的期限與原因。 | 新提案（「裁判確定後 3 個月」是詐欺條例第 37 條第 2 項） |
| L20 | 多個帳號時建立措施 | 先選帳號：{帳號 ID}・{店名} | 新提案 |
| L21 | 版本衝突 | 這筆來函剛被更新，頁面已重新載入；請確認最新狀態後再操作。 | 新提案（句型同 B23） |
| L22 | 讀取失敗 | 來函讀取失敗（加「重試」） | 新提案 |

**(e) 伺服器句子（`staff` 段）**

| key | zh-TW（建議） |
|---|---|
| `case.unavailable.inquiry_open` | 這個案件已有一筆詢問在等賣家回覆。 |
| `case.unavailable.store_purged` | 店家資料已依保留期限清除，沒有聯絡資料可以詢問賣家。 |
| `case.unavailable.referral_waived` | 這個案件已標記不轉報。 |
| `case.unavailable.no_referral` | 這個案件還沒有記錄轉報。 |
| `inquiry.unavailable.withdrawn` | 這筆詢問已撤回。 |
| `inquiry.unavailable.replied` | 這筆詢問已記下回覆。 |
| `inquiry.receive_time_range` | 收到的時間不能早於詢問建立，也不能晚於現在。 |
| `inquiry.due_range` | 期限要在建立後 1 天到 14 天之間。 |
| `authority_order.unavailable.completed` | 這筆來函已完成。 |
| `authority_order.version_conflict` | 這筆來函剛被更新，請重新讀取後再送出。 |
| `authority_order.hold_not_later` | 新的保存期限必須晚於目前的保存期限。 |
| `authority_order.target_store_mismatch` | 這個店家不屬於這個帳號。 |
| `authority_order.target_duplicate` | 同一個帳號或店家重複了。 |
| `authority_order.target_overlap` | 這個帳號已經整個列為對象，不用再列它的店家。（Will 2026-10-19 核准） |
| `measure.source_authority_order_not_naming` | 這筆來函沒有指名這個帳號。 |
| `referral.receive_date_range` | 收到的日期不能早於轉報日，也不能晚於今天。 |
| `referral.follow_up_past` | 追蹤日期不能早於今天。 |

**(f) 術語表**

| 詞 | 英文 | 說明 |
|---|---|---|
| 詢問 | inquiry | 平台對賣家（之後也可能是檢舉人）的一次要求，有期限與一則回覆；不叫「要求」，避開申覆、解除請求 |
| 主管機關來函 | authority order | 主管機關給平台的公文：限制商品、調取資料、暫停帳號等；不是通報 |
| 保存期限 | preservation period | 依法或依來函暫停個資清理的期間 |

**(g) 審查指出的缺漏（v2 新增）**

| # | 位置 | zh-TW（建議） | 來源 |
|---|---|---|---|
| M1 | 不轉報：已標記過（橫幅） | 這個案件先前已標記不轉報。 | 新提案（句型同 J14） |
| M2 | 從來函開三個建立措施對話框時頂端 | 這個措施會記為主管機關來函 {文號} 導出的措施。 | 新提案（句型同 3ca #E22） |
| M3 | 延長歷史的每一列 | {時間}・{管理員 ID}：保存到 {前值} → {新值}（{原因}）；自動延長：提供資料之後 6 個月 | 新提案 |
| M4 | 詢問已撤回而賣家之後才回覆（記下回覆的卡片位置不出現按鈕時，卡片下方的說明） | 結案或撤回之後才收到的回覆，請寫在案件的內部註記。 | 新提案 |
| M5 | 對象逐家的涵蓋狀態；沒有店家時 | 保存中｜登記之前已依保留期限清除｜保存期限已過｜保存期限已過，個資已依保留期限清除（第四個狀態，見 3c-b-3 修訂 10；Will 2026-10-19 核准）；沒有店家時：這個帳號在平台沒有店家資料；買家與帳號資料的保存尚未支援 | 新提案 |
| M6 | 完成來函：每個對象的告知（Q1 (a) 時） | 告知被指名者｜已告知（時間、方式）｜機關要求暫不告知（向機關確認的日期）｜由措施告知（只在暫停帳號、限制商品，而且已建立措施） | 新提案 |
| M8 | 暫不告知的後續（Q1 (a) 時） | 按鈕「記下告知」；待辦種類「告知被指名者」；說明「機關要求暫不告知；到了確認日期請向機關確認能否告知。」 | 新提案 |
| M7 | 完成來函：提供資料的時間（調取資料） | 提供資料的時間｜保存期限會延到提供後 6 個月。 | 新提案（Q5 (b)） |

伺服器句子補（`staff` 段）：

| key | zh-TW（建議） |
|---|---|
| `authority_order.receive_time_range` | 收文時間不能晚於現在，也不能早於 1 年前。 |
| `authority_order.due_range` | 期限要在收文後 1 小時到 30 天之間。 |
| `authority_order.targets_count` | 指名的對象要有 1 到 20 個。 |
| `authority_order.products_count` | 每個對象的商品最多 50 個。 |
| `authority_order.account_not_found` | 找不到這個帳號。 |
| `authority_order.target_notice_required` | 請為每個指名的對象選擇告知的方式。 |
| `authority_order.measure_is_notice_invalid` | 只有暫停帳號或限制商品的來函、而且已建立措施時，才能選「由措施告知」。 |
| `authority_order.recheck_date_required` | 暫不告知時，請填寫向機關確認的日期。 |
| `authority_order.provide_time_range` | 提供資料的時間不能早於收文，也不能晚於現在。 |
| `authority_order.provide_time_not_data_request` | 只有調取資料的來函可以填寫提供資料的時間。 |
| `case.awaiting_seller_with_closed` | 「等賣家」只能和進行中的案件一起篩選。 |
| `referral.follow_up_past`（`RecordFDAReferral` 也用） | 追蹤日期不能早於今天。 |

v1 的 `authority_order.target_no_store` 不再需要（B17 改成照樣登記）。詢問或來函不存在的 `not_found` 用 Connect 預設文字（共用目錄的 Connect code 文字），不另加句子。

（法律文字照 rb 在 I-legal 改，不在本刀的文案表。）

---

## 6. 3d 會用到的

同 3ca §6：本刀交判定式、開始時間、期限與存取路徑；來源查詢與成員的資料集測試在 3d。

| 種類（rb R3） | 對象 | 成員（Go 判定式） | 開始時間 | 期限 | 存取路徑 | key 在待辦期間是否不變 |
|---|---|---|---|---|---|---|
| 待決定（改） | 案件 | `Case.AwaitingDecision()`：進行中而且沒有還沒結束的詢問 | `Case.DecisionClockStart()`：`max(開案, 最後一筆詢問的回覆收到時間或管理員撤回時間)` | 開始時間＋依嚴重度（3d 的 `ReviewPolicy`） | `cases_open` 加 `NOT EXISTS case_inquiries_one_open 的列` | 不變：開始時間只在案件不在待辦（等賣家）期間改變；改嚴重度是 rb R3 允許的例外 |
| 待轉報（改） | 案件 | `CaseFacts.AwaitingReferral()`：`Referral == nil`（不轉報也是一筆） | `ClosedAt` | ＋24 小時 | 不變 | 不變 |
| 詢問逾期（新） | 詢問 | `CaseInquiry.Overdue(now)`：還沒結束而且 `now >= due_at` | `due_at` | `due_at + InquiryOverdueGrace`（24 小時） | `case_inquiries_open_due` | 不變（期限建立後不能改，B5） |
| 主管機關來函（新） | 來函 | `AuthorityOrder.Open()`：還沒完成 | `received_at` | `due_at` | `authority_orders_open_due` | 不變（來函建立後不能改） |
| 告知被指名者（新，只在 Q1 (a)） | 來函的對象 | 告知是 `WITHHELD` 而且 `recheck_on` 不晚於今天（台北） | `recheck_on` 的台北 00:00 | 開始時間＋3 天（起始值，Q6） | 對象表的部分索引 `(recheck_on) WHERE notice = 'WITHHELD'` | 不變；記下新的確認日期時離開再進入（key 是 (來函, 對象位置)，`ReviewQueueItem.subject` 用 `authority_order_id` 加位置） |

- 期限輸出：`CaseInquiry.due_time`、`AuthorityOrder.due_time` 由本刀輸出（只在 OPEN）；`Case.decide_due_time` 在 3d。**開始時間由伺服器輸出**（3ca 附錄 D #2）：3d 輸出 `decide_due_time` 時一併輸出開始時間；詢問與來函的開始時間就是 `create_time`／`receive_time`，client 拿得到，本刀的 `DeadlineText` 用它們。
- 「詢問逾期」的期限是 `due_at + 24h`，但詢問本身的 `due_time` 是 `due_at`：待辦列的「期限」是營運要處理逾期的時間，詳情頁的「期限」是賣家的期限。3d 的待辦列要用前者（`ReviewQueueItem` 的期限欄位由來源給）。
- 時間觸發的種類（詢問逾期）：3cd §3 那一列的分析照用——推導＋keyset 合併下不需要 job；若 3d 因 ADR D3 的比對改成投影，「詢問逾期」要由 seller 的 job 在 `due_at` 發事件。
- `ReviewQueueItem.subject` 加 `inquiry_id`（連到案件詳情的那一筆，`/admin/violation-reports/cases/{case_id}#inquiry-{id}`，所以待辦列也要帶 `case_id`）與 `authority_order_id`。

---

## 7. 測試

### 7.1 model（unit）
- `InquiryFacts.Availability()`、`CaseFacts.Availability()` 新增的列、`AuthorityOrder.Availability()`：逐格斷言原因與順序。
- `CaseInquiry.Overdue`（邊界 ±1µs）、`ClockRestart`（回覆、管理員撤回、系統撤回、還沒結束）；`Case.DecisionClockStart`（沒有詢問、回覆晚於開案、撤回、兩筆取最晚）；`AwaitingDecision` 與 `AwaitingSeller` 互斥（進行中時剛好一個成立）。
- `DefaultAuthorityDue` 每個種類；`AuthorityHold == 4416h`。

### 7.2 資料集測試（integration）
- `ListCases(awaiting_seller=true)` 的集合等於 `Case.AwaitingSeller()`；`ListCases(state=OPEN)` 減去它等於 `AwaitingDecision()`。翻頁不漏不重。
- `CaseDecisionClockStarts` 等於 Go 的 `DecisionClockStart`（**mutation**：SQL 把系統撤回也算進去，測試要失敗）。
- `ListMeasures(source_authority_order_id)` 三條清單路徑：資料集含明確連結（RISK 停權）、認定引用、引用既有認定的措施、已撤銷的措施、別的來函（**mutation**：少掉其中一條 EXISTS，測試要失敗）。
- `ListAuthorityOrders(state=OPEN)` 等於 `Open()`。
- 不變量 J1–J5（每個指令之後檢查）。

### 7.3 保存與清理的競態
- **固定交錯的兩條連線**（取代 v1 的 200 次隨機並行；B26）。`purge` 的 `Deps` 加一個只在測試設定的 hook `afterCandidates func()`，在第一次 `RetireDueStores` 之後、再判斷之前執行：
  - `TestPurgeRechecksTheAuthorityHoldAfterLockingTheCandidates`：連線 B 開 transaction、插入指名這個帳號的來函與對象（**不鎖店家列**：模擬最壞的寫入方），不 commit → 連線 A 跑清理的第一個語句，拿到候選並鎖住 → hook 裡讓 B commit → A 再判斷 → 這家店不被清（**mutation**：拿掉再判斷，店家被清，測試失敗）。
  - `TestPurgeRechecksTheAppealHoldAfterLockingTheCandidates`（3c-b-0）：同上，B 插入終止措施的待處理申覆。
  - `TestAppealAfterThePurgeLockedTheStoreSeesItPurged`（3c-b-0）：A 鎖住候選（hook 停住）→ B 送申覆（阻塞在店家列），B 的時鐘**刻意設得比 A 早**→ A commit → B 回「申覆期限已過」，沒有待處理的申覆留在已清理的店家上（**mutation**：拿掉鎖後的 `pii_purged_at` 檢查，測試失敗：證明結果不依賴時鐘）。
  - `TestLinkedSourceAppealBlocksThePurgeOfTheLinkedAccount`（3c-b-0）：A（清理被關聯帳號）鎖住候選 → B 對來源措施送申覆，因為鎖被關聯帳號的店家列而阻塞；反向：B 先鎖住、A 的第一個語句 `SKIP LOCKED` 跳過；以及 B 在 A 的第一個語句之後 commit、A 的再判斷看到 hold（**mutation**：申覆只鎖來源帳號，測試失敗）。另測 `retryScope`：預讀之後剛好新增一筆關聯終止，申覆重來一次並鎖到它。
  - `TestFoodReviewDeletionRechecksTheHoldAfterLocking`：同第一個測試的交錯，對象是 2 年期滿的食品登錄審核紀錄；B 是建立來函（**mutation**：拿掉以候選再呼叫 `ExpiredFoodRegistrations`，測試失敗）。
- `TestReopenIgnoresTheAuthorityHold`：保留期已滿、被來函保存的店家，撤銷造成關店的終止 → 只撤銷紀錄、不恢復營業，與沒有來函的同一個情境結果相同；保留期內被保存的店照常恢復（**mutation**：`ReopenPlan` 傳 `apply_preservation_hold = true`，被保存而保留期已滿的店被恢復，測試失敗）。
  - `TestAuthorityOrderRegisteredAfterThePurgeShowsAlreadyPurged`：A 鎖住候選 → B 建立來函（阻塞）→ A commit → B 成功，那一家店的涵蓋狀態是 `PURGED_BEFORE_REGISTRATION`。
- `TestRetentionIsSixMonthsAfterAnyClosure`：關店 4416 小時整不清、多 1µs 清（**mutation**：常數改回 720）。
- `TestAuthorityHoldStopsThePurgeOnlyForTheNamedStoreOrAccount`：指名帳號（全部 stint 都不清）、指名店家（同帳號的另一個 stint 照清）、別的帳號照清、`hold_until` 剛到期照清（**mutation**：拿掉 `t.store_id IS NULL OR` 或改成只看帳號，測試要失敗）。
- `TestExtendingTheHoldKeepsTheDataPastTheDefault`、`TestCompletingADataRequestExtendsTheHoldToSixMonthsAfterProvision`（Q5 (b)）、`TestFoodReviewDeletionRespectsTheHold`；延長歷史與 `hold_until` 一致（J5）。

### 7.4 指令與並行
- 詢問：建立的三個原因（含只關店不擋、已清理才擋，B29）；`request_id` 重送回同一筆、內容不同回 `invalid_argument`；`case_inquiries_one_open` 的保底（兩個 goroutine 同一案件建立 → 只有一筆，另一個是 `failed_precondition`）。
- 記下回覆：`receive_time` 的範圍；附件以 `Admin: true` 轉正為機敏、引用登記在詢問 ID、讀取先記紀錄（`TestInquiryEvidenceReadIsRecordedBeforeTheRead`）；重送回 `already_recorded`；已撤回回錯誤。
- `TestALateRecordedReplyStartsTheDecisionClockAtTheReceiveTime`：收到 2 天前、今天才記下 → `DecisionClockStart` 是 2 天前（B11）。
- 結案與回覆並行（200 次，先例 `TestDecideAndAConcurrentTriageNeverLeaveAnUnresolvedMemberInAClosedCase`）：不會出現已結案而詢問還沒結束（J1）、也不會有回覆寫在系統撤回之後。
- 移動造成 `NO_REPORTS` 時撤回詢問。
- 版本：建立、回覆、撤回讓案件的 etag 前進（**mutation**：拿掉 `BumpCase`，用舊 etag 結案要失敗而沒有失敗）。
- 轉報：不轉報與記錄轉報互斥（兩個方向）；決定之前可以不轉報；回覆只在 REFERRED；追蹤日期的預設（台北日期，跨午夜的邊界）、更新、清掉。
- 來函：對象驗證（帳號不存在、店家不屬於帳號、重複、數量上限）；沒有店家的帳號照樣登記（`no_seller_data`）而且出現在 3d 的判定式裡；完成與延長的 etag；延長只增；Q1 (a) 時：每個對象都要有告知、`MEASURE_IS_NOTICE` 的兩個條件、`WITHHELD` 到確認日期進入判定式、`RecordAuthorityTargetNotice` 只能改成 NOTIFIED 或新的 WITHHELD；`provide_time` 的範圍與種類。
- 建立措施：來函沒有指名這個帳號回 `invalid_argument`；指名店家時同帳號的另一個 stint 的措施被拒；request hash 含來函（**mutation**）。

### 7.5 migration
- `TestCaseReferralV2KeepsExistingReferralsAsReferred`（含欄名改成 `recorded_at`、`recorded_by_admin_id`）。
- `TestCooperationRequestsMigrationRefusesExistingRows`：有列時失敗並列出通報 ID；沒有列時刪表。
- `TestSchemaFileMatchesMigrations`、`TestChecksMatchGo`（新列舉）、`TestSchemaConstraints` 加新表的每一條約束各一個只違反它的案例（3ca 附錄 C 的寫法：`wantViolates` 與 `PgError.ConstraintName`）。

### 7.6 httpapi、client
- `CheckFieldPaths`、REQUIRED 欄位、page token 綁篩選、`adminauth` 啟動檢查、`TestEveryServiceInTheContractIsMounted`、`EnumMismatches`；檔案路由的權限與 404。
- admin：新頁與對話框的請求、`can_*` 對按鈕、a11y 清單加 7 個對話框、`inquiry-text.ts`／`authority-text.ts` 完整（`Record<Exclude<…>>`）、`fda-export.ts` 的新句子、追蹤日期的預設、`sourceAuthorityOrderId` 進請求與 request key、上傳走 `isSecure: true`（**mutation**：拿掉旗標，測試要失敗）。

---

## 8. Commit 計畫

每個 commit 跑 §9 與自己範圍相符的檢查，綠燈才 commit；`git log -1 --stat` 核對。審查：3c-b-0、3c-b-1、3c-b-3、3c-b-5 各做 code-review；3c-b-1 開工前與 3c-b-5 完成後各跑一次 design-review。

| # | commit | 內容 | 為什麼可以獨立綠燈 |
|---|---|---|---|
| 3c-b-0 | `fix(seller): 個資清理在鎖住候選之後再判斷一次 legal hold，申覆鎖住被保護帳號的店家列並在鎖後確認店家沒有被清理` | §2.4「既有缺陷」的三個窗：`purge.batch` 再判斷（子表只用再判斷的名單）；`submitRequest` 鎖後檢查 `pii_purged_at`（只用在造成店家目前關閉的終止合作上）、`now` 移到鎖之後；來源申覆預讀被關聯帳號、`LockAccounts` 升序、重讀、`retryScope`；測試 hook；先提交會失敗的三個固定交錯測試（§7.3）再修；seller `AGENTS.md` 的鎖順序與個資清理段。食品登錄的刪除不在這裡：main 上它沒有 hold，沒有洞（§2.4） | main 上的缺陷，與 3c-b 無關；只動 purge 與 measure 的申覆 |
| 3c-b-1 | `feat(seller): 案件的詢問與賣家回覆的記錄，刪除舊的配合要求（R4）` | 第一步：`backend-e2e-verify` 驗 B27 的上傳鏈（不通就在這個 commit 補 media 的入口）；§1.2、§2.1、§2.5（含 3c-a migration 測試的處理）、§3（詢問）、§4（詢問）；`AwaitingDecision`／`DecisionClockStart`；`FinalizeEvidence` 改收 `media.Uploader`；權限 5 個；devseed | 只加 RPC 與 `Case` 的欄位，admin 不用改就能編譯 |
| 3c-b-2 | `feat(seller): 轉報紀錄加追蹤日期、主管機關的回覆與不轉報（R5）` | §1.3、§2.2、§3（轉報）；權限 2 個；`RecordFDAReferral` 的新欄位是選填；admin 的轉報欄名跟著生成碼改 | 同上 |
| 3c-b-3 | `feat(seller): 主管機關來函、保存期限與措施的來源來函（R5）` | §1.4、§1.5、§2.3、§2.4 的 hold（`apply_preservation_hold` 參數、`ReopenPlan` 傳 false 與 `TestReopenIgnoresTheAuthorityHold`；食品登錄刪除改成同一支查詢呼叫兩次與它的固定交錯測試）、§3（來函）；Q1、Q3、Q5 的結論；權限 5 個（Q1 (a) 時 6 個） | 建在 3c-b-0 的再判斷上；`StoreRetention` 不在這裡改 |
| 3c-b-4 | `feat(seller): 關店後的個資保存改為 6 個月（詐欺犯罪危害防制條例第 37 條）` | `StoreRetention`（Q2 選 (b) 時改成兩段清理，這個 commit 會變大）；ADR 修訂；計畫書 I-legal 段記「四份文字的保存期、上線阻擋條件」（B21）與 B31 的新資料類別清單（詢問的說明與回覆、回覆附件、主管機關對轉報的回覆、來函的摘要、處理摘要與對象）；**不改法律文字**（rb §4） | 常數一處 |
| 3c-b-5 | `feat(admin): 案件的詢問與轉報、主管機關來函頁` | §5 全部；三個建立措施對話框的 `sourceAuthorityOrderId`；側欄 | 只動 admin |

文件跟著各自的 commit：gov ADR 文末「修訂：詢問、轉報 v2 與主管機關來函」（3c-b-1～3，含 B20 對 rb §2 第 1 點的修訂）、`2026-10-02-seller-kyc-payuni-in-monolith` 個資保留的修訂（3c-b-4）、設計書 32、食安 runbook（轉報追蹤、不轉報、來函的處理）、seller `AGENTS.md`（擁有的資料、鎖順序、個資清理）、CARRYOVER（配合要求的列改成「刪除」、保留期列、新的沿用盤點）、術語表。最後在計畫書 10e 段記完成紀錄，在 rb 加「實作時的修訂（3c-b）」（B5 的讀法、B15 的延伸、B20）。

**實作時的修訂（3c-b-1）**：實作與上文不同的三處，以這份為準。

1. 詢問的四個 RPC 的請求與回應 message 放在 `P/governance.proto`，不在 `case_inquiry.proto`（§1.1、§1.2）：回應帶 `Case`（`governance.proto` 定義），而 `governance.proto` 為了 service 要 import 請求型別，放在 `case_inquiry.proto` 會形成 import 循環。`CaseInquiry`、`CaseInquiryReply` 與列舉仍在 `case_inquiry.proto`。
2. 不做 `InquiryFacts`（§2.6、§3.1）：`CaseOpen` 沒有讀者（§3.1 本來就寫記下回覆不看案件是否進行中），可用性由 `CaseInquiry.Availability()` 單獨判斷。
3. 冪等的衝突目標是 `(created_by_admin_id, request_id)`（§2.1、§3.2）：唯一約束本來就是這個組合鍵，`ON CONFLICT (request_id)` 對不上約束，編不過。

**實作時的修訂（3c-b-2）**：實作與上文不同的地方，以這份為準。

1. 回覆表的管理員欄叫 `recorded_by_admin_id`（§2.2 寫 `admin_id`）：與 `case_referrals` 改名後的欄名一致，兩種紀錄、兩張表都是「記下的時間與管理員」。
2. 日期在模組裡是 UTC 午夜的 `time.Time`，不是 `civil.Date`（§3.1）：go.mod 沒有 `cloud.google.com/go/civil`，sqlc 已把 `DATE` 對到 `time.Time`（UTC 午夜），台北日期用 `model.TaipeiDay`。
3. `google.type.Date` 不是有效日期（例如 2 月 30 日）時，handler 轉成零值，模組回該欄位原本的範圍句子（`referral.receive_date_range`、`referral.follow_up_past`）：不另加「日期無效」的一種錯誤。
4. 最後一筆通報的移動只在已轉報時擋下（§3.1 沒有寫）：標記不轉報的案件空了照樣以 `NO_REPORTS` 關閉。原句子「已記錄轉報」對不轉報不成立，為它另加句子不值得（沒有後果：不轉報的原因留在案件上）。
5. 輸入驗證新增三句伺服器句子（`referral.waiver_reason_length`、`referral.reply_body_length`、`referral.follow_up_conflict`），§5.4 沒有；Will 2026-10-19 已核准（見文案核准表）。
6. 這一刀的冪等鍵不適用：三個轉報指令都不帶 `request_id`（§1.3）；範圍是（管理員, request_id）的規則（B30）只在 3c-b-1 的詢問與 3c-b-3 的來函。

**實作時的修訂（3c-b-3）**：實作與上文不同的地方，以這份為準。

1. 來函的請求與回應 message 放在 `P/authority_order.proto`（§1.1 說放新檔，與設計一致）；回應只帶 `AuthorityOrder`，不帶 `Case`，所以沒有 import 循環。`RecordAuthorityTargetNotice` 的請求在同一個檔（§1.4 的草圖有多餘的 `}`，已忽略）。
2. `CompleteAuthorityOrder` 的 SQL 帶 `hold_until`（完成與調取資料的延長是同一個語句，version 只加一次）；沒有延長時帶目前的值，CHECK 以 `hold_until <= 新值` 擋回頭。
3. 告知另存 `notice_recorded_at`、`notice_recorded_by_admin_id`（誰、何時記下目前這個告知），`authority_target_notice_history` 存被取代的完整值（含它當時的作者）加 `replaced_at`／`replaced_by_admin_id`；§2.3 只列了 `notice`、`notified_at`、`notice_note`、`recheck_on`。
4. 完成時告知的輸入只看種類對應的欄位（已告知看告知時間、暫不告知看確認日期），其餘忽略；暫不告知的確認日期不限制不能是過去（過去的日期只是立刻進待辦）。告知時間要在 [收文, 現在]。
5. （審查修正取代原做法）涵蓋狀態不比較時間：登記時在鎖住店家列之後，把「這時個資已清理」的店家寫進 `authority_order_prepurged_stores (order_id, store_id)`（事實快照，只新增）；清理與登記可能在不同實例，應用程式時鐘有差，比 `pii_purged_at <= registered_at` 會把登記之前清掉的店家標錯。見修訂 10。
6. 帳號存在以 `identity.Account` 驗證，已刪除的墓碑也算存在（帳號存在過，來函照樣要登記）；找不到才回 `authority_order.account_not_found`。
7. 輸入驗證新增 12 句伺服器句子，§5.4 沒有；Will 2026-10-19 核准（三句改字，見文案核准表）：`authority_order.kind_required`、`agency_length`、`document_no_length`、`summary_length`、`resolution_length`、`hold_reason_length`、`notify_time_range`、`notice_note_length`、`target_position_invalid`、`notice_not_withheld`，與 `measure.source_authority_orders_count`、`measure.source_authority_order_duplicate`。冪等鍵的句子沿用已核准的 `inquiry.request_id_reused`（描述改成涵蓋來函）。
8. `Names`（對象是否指名了某帳號的某店家）與 SQL 的 `OrdersNamingMeasureTarget` 是同一個定義；`AuthorityOrderMeasureTargets`（驗證「由措施告知」）與 `ListMeasures(source_authority_order_id)` 的定義一致由 `TestMeasureSourceAuthorityOrderNamingAndListFilter` 鎖住。
9. 食品登錄審核紀錄刪除的 hook 是另一個（`SetPurgeAfterFoodCandidates`），店家清理的 hook 不變。
10. 涵蓋狀態定義（與 `hold_active` 一致；`StoreCoverageOf(purgedAt, purgedAtRegistration, now)`，依序判斷）：登記時快照記為已清理 → `PURGED_BEFORE_REGISTRATION`；否則已清理（`pii_purged_at` 非空）→ 新增的 `PURGED_AFTER_HOLD_ENDED`（清理只會在保存期限已過時發生，之後即使延長保存，清掉的資料也回不來，所以已清理的店家不論目前 `hold_active` 都不是 HELD 或 HOLD_ENDED）；否則保存期限未到 → `HELD`；否則 → `HOLD_ENDED`（期限已過、還沒清）。這個新狀態的標籤（M5 第四項）「保存期限已過，個資已依保留期限清除」Will 2026-10-19 核准，由 3c-b-5 的 P6 使用。
11. 對象的位置連續由 DB 保證：`authority_order_targets` 有產生欄位 `prev_position`（位置大於 0 時是前一位置）與自我參照的外鍵 `(order_id, prev_position) → (order_id, position)`，所以不可能有缺號；程式以位置為索引取對象是安全的。devseed 因此刪除任何碰到種子店家（或其帳號）的整份來函，不只刪那幾個對象。
**實作時的修訂（3c-b 修正輪，Will 2026-10-19 決定 A–E；design-review `3cb-design-review-final` 與 3c-b-5 code-review）**：實作與上文不同的地方，以這份為準。

1. **Q2 改選 (b)，關店後兩段清理**（取代 B21、§2.4 與 Q2 (a) 的「整個 stint 一起延到 184 天」）：Q2 (a) 的分析「多保存的只有店家說明與圖片」不成立——清理的同一個 transaction 發 `StoreRetired`，下游（im 遮蔽聊天個資、product、social、ai）跟著晚 5 個月。第一段（退場）在關店後 `StoreRetireAfter` = 720 小時：清說明、獎項、網站，釋放 logo 與封面，發 `StoreRetired`，記 `stores.retired_at`，待審的食品登錄撤回、核准中的結束（`STORE_RETIRED`）；退場之後不能恢復營業。第二段在關店後 `StoreRetention` = 4416 小時：清店名、聯絡、地址、KYC、登記資料查詢、PayUni 自由文字、食品登錄字號與理由、同意紀錄的 IP 與 User-Agent（店名歸第二段：它是送交金流審核的身分欄位）。兩段的 `TERMINATION` 都用 `greatest(關店後該段時數, 最晚的申覆期限)`。資料庫：`stores_retired_check`、`stores_closed_images_check` 改看 `retired_at`、`stores_pii_purged_check` 要求已退場；migration 以 `retired_at = pii_purged_at` 補既有列。`PurgePII` 回第二段清了個資的店家數。
2. **每一種保存擋哪一段**：申覆的 legal hold 擋兩段（恢復營業：第一段的時間條件成立而且沒有申覆 hold 才算保留期已過）；來函的保存擋兩段與食品登錄審核紀錄的刪除，不影響恢復營業。來函的涵蓋狀態仍只看第二段（`pii_purged_at`），已退場而個資還在的店顯示保存中或保存期限已過（觸發：機關第一次要求調取聊天或商品資料而店家已退場時，加一個涵蓋狀態；記在 KYC ADR 的 2026-10-20 修訂）。
3. **保存條件只有一支查詢**（取代 §2.4 的 `apply_preservation_hold` 與「同一支查詢呼叫兩次」、CARRYOVER 的「述詞有兩份」）：`StoreHolds(store_ids, now)` 回每家店的 `appeal_hold` 與 `preservation_hold`；兩段清理的時間條件是 `RetireDueStores` 的兩段（`second_stage`），審核紀錄的時間條件是 `ExpiredFoodRegistrations`，都不帶保存；每一批在鎖住候選之後以同一支時間查詢再判斷、再問 `StoreHolds`（新的語句，§2.4 的四種情形照樣成立），候選查詢以 `store_id`／`id` 做 keyset（被保存而略過的列不會讓「取不滿才停」的迴圈一直選到同一批）。固定交錯測試每段各有：第一段 `TestPurgeRechecksTheAuthorityHoldAfterLockingTheCandidates`、`TestPurgeRechecksTheAppealHoldAfterLockingTheCandidates`、`TestAppealAfterTheRetirementLockedTheStoreSeesItRetired`；第二段 `TestPurgeStageTwoRechecksTheAuthorityHoldAfterLockingTheCandidates`、`TestPurgeStageTwoRechecksTheAppealHoldAfterLockingTheCandidates`、`TestAuthorityOrderRegisteredAfterThePurgeShowsAlreadyPurged`。申覆的鎖後檢查改看 `retired_at`（第二段一定在退場之後）。
4. **撤銷登記（登記錯誤）**（取代 §1.4 的「登記錯了：完成並在摘要寫明，再登記正確的一筆」：完成不解除保存，錯誤的對象會被保存至少 184 天）：`RevokeAuthorityOrder{id, reason, etag}`，只在完成之前（`AuthorityOrderAvailability.Revoke`：已撤銷 `REVOKED`、已完成 `COMPLETED`），原因 1 到 2000 字；`authority_orders` 加 `revoked_at`、`revoked_by_admin_id`、`revoke_reason`（`authority_orders_revoke_check`、與完成互斥的 `authority_orders_terminal_check`），`authority_orders_open_due` 排除已撤銷的；狀態加 `REVOKED`，清單可依它篩選；撤銷之後保存解除、不能完成、延長或記下告知、不能再當作措施的來源（`OrdersNamingMeasureTarget` 以 `FOR SHARE` 讀來函列，撤銷以 `FOR UPDATE` 鎖同一列）。冪等以管理員為範圍：同一位管理員已撤銷時回原本的紀錄與 `already_revoked`（不比 etag 與原因），別人撤銷的回 `failed_precondition`。不鎖店家列（解除保存只會讓清理更早，清理鎖住候選之後才問保存）。權限 `seller.authority_orders.revoke`（admin 方法 92 支）。
5. **`RecordFDAReferralReply` 帶 `request_id`**（取代 §1.3 與 3c-b-2 修訂 6 的「只新增，重送會多一則」：只處理連點，沒有處理回應遺失）：選填 UUIDv4，`case_referral_replies` 加 `request_id`、`request_hash` 與 `UNIQUE (recorded_by_admin_id, request_id)`；鎖住案件之後先查重送，相同內容回案件目前的狀態、不多記一則，不同回 `inquiry.request_id_reused`。admin 在對話框開啟時產生、內容改了就換新的。
6. **伺服器推導的可用性與上限**（3c-b-5 code-review D1–D3、D5）：`FDAReferral.follow_up_overdue`（`case_referrals.follow_up_set_at`：記錄轉報或換成下一個追蹤日期的時間；逾期＝今天〔台北〕晚於追蹤日期而且這個時間之後沒有回覆，所以換日期之後重新判斷）；`AuthorityOrderTarget.can_record_notice` 與 `record_notice_unavailabilities`（`REVOKED`、`NOTICE_NOT_WITHHELD`），`AuthorityOrder.can_revoke` 與 `revoke_unavailabilities`（`AuthorityOrderUnavailability{reason, LocalizedMessage}`）；`AuthorityTargetStore.can_create_measure`（`TerminationTarget.AllowsMeasure`：店家沒有關閉，與 `CreateMeasure` 同一份）與 `can_create_termination`（`AvailableTerminationGrounds` 不為空），撤銷的來函都不行；`CaseInquiry.reply_evidence_max`（模組的 `MaxEvidence`）。admin：期限不填就是伺服器的預設（L12 的說明句不變）、不在 client 擋對象數；多家可以建立措施的店時不預設第一家；附件超過上限在上傳之前擋下；上傳失敗一律顯示 J15。
7. **`TaipeiDay` 移到 `platform/twtime`**：`twtime.Day`（UTC 午夜的日期值）與 `twtime.Midnight`（台北 00:00 的時刻），取代 `model.TaipeiDay` 與 5 處 inline 的寫法（`kyc.go`、`kyc_registration.go`、`views.go` 兩處、`httpapi/seller/dashboard.go`）。日期值型別沒做（取代 3c-b-2 修訂 2 的「go.mod 沒有 civil」：那是 SELF-IMPOSED；觸發記在 CARRYOVER）。
8. **刪除 `CaseInquiryCreated`**（取代 B12、§3.3）：沒有消費者，與同一個模組「消費者出現時再加它需要的事件」的規則衝突；A8 通知賣家時再加（遷移計畫 A8 段同步改寫）。
9. **記進 CARRYOVER、不改**：對象位置連續的自我參照外鍵只用在 `authority_order_targets`（3c-b-3 修訂 11）；來函不加 `(agency, document_no)` 的唯一性（同一份公文可能要分開登記；重複登記用撤銷登記更正）。
10. 3c-b-0 的措辭以階段寫準：「已清理 ⇒ 申覆期限已過」改成「已退場 ⇒ 申覆期限已過」，賣家自己關店的退場只看關店後 30 天（appeal.go、AGENTS、CARRYOVER、測試名 `…WasRetired`、`…TheRetirementLockedTheStoreSeesItRetired`）。

**文案核准（Will 2026-10-19，3c-b-1 補進目錄的句子與 §5.4 之外新增的句子）**

| key | zh-TW | 狀態 |
|---|---|---|
| `measure.retry_exhausted` | 措施剛剛有變動，請重試。（句尾加句號） | 已核准 |
| `authority_order.target_overlap` | 這個帳號已經整個列為對象，不用再列它的店家。（3c-b-3 實作） | 已核准 |
| `inquiry.kind_required`、`inquiry.message_length`、`inquiry.channel_required`、`inquiry.body_length` | 照目錄現文 | 已核准 |
| `inquiry.request_id_reused` | 這次送出的內容和同一個請求先前送出的不同，請重新整理頁面後再試。 | 已核准 |
| `case.awaiting_seller_with_closed` | 「等賣家」只能和進行中的案件一起篩選。 | 已核准 |
| `referral.waiver_reason_length` | 不轉報的原因必須是 1 到 2000 字。（3c-b-2 實作） | 已核准 |
| `referral.reply_body_length` | 回覆內容必須是 1 到 2000 字。（3c-b-2 實作） | 已核准 |
| `referral.follow_up_conflict` | 不能同時設定下一個追蹤日期和不再追蹤。（3c-b-2 實作） | 已核准 |
| `authority_order.kind_required`、`agency_length`、`document_no_length`、`summary_length`、`resolution_length`、`hold_reason_length`、`notify_time_range`、`notice_note_length`、`target_position_invalid`（3c-b-3 實作） | 照目錄現文 | 已核准 |
| `authority_order.notice_not_withheld` | 這個對象不是暫不告知的狀態，不能再記下告知。（3c-b-3 實作，Will 改字） | 已核准 |
| `measure.source_authority_order_duplicate` | 引用的來函不能是空的，也不能重複。（3c-b-3 實作，Will 改字） | 已核准 |
| `measure.source_authority_orders_count` | 一個認定最多引用 5 份來函。（3c-b-3 實作，Will 改字） | 已核准 |
| 3c-b-5 的「新增對象」「移除」（登記頁對象列的按鈕） | 照現文 | 已核准（Will 2026-10-19，修正輪 E） |
| 3c-b-5 的網路失敗句（不轉報、記下主管機關的回覆、來函的登記／完成／延長／記下告知／撤銷登記、附件上傳失敗） | 沿用 J15「記下失敗，沒有寫入；請確認網路狀態後再試一次。」 | 已核准（同上） |
| 「記下告知」對話框 | 標題與送出鍵「記下告知」；欄位「告知被指名者」「時間」「方式」「向機關確認的日期」 | 已核准（同上） |
| 由措施告知的限制句 | 只限暫停帳號或限制商品，而且已經從這份來函建立措施。（Will 改字） | 已核准（同上） |
| 記下主管機關的回覆的送出鍵、完成來函的送出鍵 | 記下回覆｜完成 | 已核准（同上） |
| 下拉選單的空選項（含店家下拉） | 請選擇 | 已核准（同上） |
| 來函詳情的標題 | 來函 {文號} | 已核准（同上） |
| 完成列的告知 | {對象}・告知被指名者：{方式} | 已核准（同上） |
| 「等賣家」標記 | 放在「進行中」旁邊的第二個標記（不取代） | 已核准（同上） |
| 撤銷登記：按鈕、對話框標題與送出鍵 | 撤銷登記 | **已核准**（Will 2026-10-20 授權由 lead 定稿） |
| 撤銷登記：說明 | 只在登記錯誤時撤銷（例如對象或文號登記錯了）。撤銷之後這筆來函的保存解除，不再是處理中，也不能再當作措施的來源；紀錄會保留。正確的來函請另外登記。 | **已核准**（Will 2026-10-20 授權由 lead 定稿） |
| 撤銷登記：欄位 | 原因 | **已核准**（Will 2026-10-20 授權由 lead 定稿） |
| 撤銷登記：已撤銷的橫幅、詳情的處理區與保存區、狀態、清單分頁與欄位 | 這筆來函先前已撤銷登記。｜已於 {時間} 撤銷登記：{原因}｜保存到 {時間}（撤銷登記時解除）｜已撤銷登記（狀態與分頁）｜撤銷時間 | **已核准**（Will 2026-10-20 授權由 lead 定稿；保存區那一句是 smoke 找到「已撤銷的來函寫已到期」之後加的） |
| `authority_order.unavailable.revoked` | 這筆來函已撤銷登記。 | **已核准**（Will 2026-10-20 授權由 lead 定稿） |
| `authority_order.revoke_reason_length` | 撤銷的原因必須是 1 到 2000 字。 | **已核准**（Will 2026-10-20 授權由 lead 定稿） |
| `measure.source_authority_order_revoked` | 這筆來函已撤銷登記，不能當作措施的來源。 | **已核准**（Will 2026-10-20 授權由 lead 定稿） |
| 撤銷登記：對象逐家的涵蓋標籤（`AUTHORITY_STORE_COVERAGE_HOLD_RELEASED_BY_REVOCATION`） | 撤銷登記時解除保存 | **已核准**（Will 2026-10-20 授權由 lead 定稿；撤銷之後 `hold_until` 可能還沒到，不能沿用「保存期限已過」） |
| 回覆附件超過伺服器上限（上傳之前） | 附件最多 {n} 份，請移除多的檔案。 | 已核准（Will 2026-10-20） |
| 來函詳情的頁首、對象列 | {機關}・收文 {時間}｜帳號 {id}｜・商品 {ids} | 已核准（Will 2026-10-20） |

---

## 9. 驗證命令（與 CI 相同）

- **3ca §9 的整段原樣照跑**：backend 單體（gofmt、vet、unit、integration、`make connect`、`make sqlc`、`make uitext`、`go mod tidy`、`make lint-monolith`、`atlas migrate validate`）、gateway（不改，確認 `maxRESTOperations` 仍是 481）、契約（`buf lint`、`@hawkr/api` 的 gen／tsc／biome／test、`packages/hawkr_api` 的 generate／analyze／test）、共用文案、admin、repo。
- **加兩支 App 的整段**（不是條件式）：proto 改了，`packages/hawkr_api` 重新生成，`seller-ci.yml` 與 `buyer-ci.yml` 都會觸發：`(cd seller-app && fvm flutter analyze && fvm flutter test && bash scripts/deadcode.sh)`、`(cd buyer-app && …)` 同。
- website：本刀不改法律文字，`@hawkr/api` 重新生成仍會讓 website CI 跑，照 3ca §9 的 website 段跑一次。
- 3c-b-1 刪表：本機套用一次（本機有列時照 3ca §2.3 重建）；B27 的 e2e 證據照 `backend-e2e-verify` skill 留下。
- admin 的手動 smoke（desktop、mobile、keyboard）：案件頁的詢問三個對話框與轉報三個對話框、來函的清單、登記頁、詳情與兩個對話框、從來函建立三種措施，各走一次，含讀取失敗與空狀態；上傳一個 PDF 與一張圖片，讀回位元組相同。

---

## 10. 待決題的說明（給 Will）

照 rb 或先例自己決定的：法律文字照 rb §4 在 I-legal（B21，後果與觸發寫在那裡）；舊的配合要求不轉換（B22，W2 先例）；店家關閉不擋詢問（B29）——個資清理之後，已開著的詢問仍留著、逾期後會一直在 3d 的待辦裡，直到管理員撤回（刻意的：這時該做的是撤回並在案件寫註記，待辦提醒這件事）；不加來函的一般註記（§1.4）；關聯終止來源申覆的競態照撤銷的先例修掉（§2.4，v2 原本接受它）。

**Q1 來函要不要記「是否已告知被指名者」？**
- 現況：調取資料、B4 之前人工處理的限制商品，不會產生措施，被指名者不會知道；被指名的也可能不是賣家（B17）。rb R5 寫「處理之後仍要告知被處置的賣家（DSA 9(5)）」。
- 業界基準：DSA 9(5)（對違法內容的命令）與 10(5)（資訊命令）要求平台「at the latest when effect is given」告知被處置者，**機關要求時可以延後**（是延後，不是永不告知）（rr §1）。台灣的刑事偵查不公開（刑事訴訟法第 245 條）可能要求暫不告知。
- 約束：DSA 不適用（SELF-IMPOSED 的基準）；機關的保密要求是 REAL（個案）；DSA 9(5) 是 rb R5 已核准的方向。
- 選項：
  - **(a) 建議：完成來函時，每個對象必填告知方式**：已告知（時間與方式）；機關要求暫不告知（必填向機關確認的日期，到期進 3d 的待辦「告知被指名者」，之後記下已告知或新的確認日期，每次改變留歷史）；由措施告知（只限暫停帳號、限制商品的來函，而且這個對象已有這筆來函導出的措施；調取資料不能選）。A8 之前由營運人工告知，A8 加寄送。成本：對象表四個欄位與一張歷史表、一支 RPC 與一個權限（`seller.authority_orders.record_target_notice`）、3d 多一個待辦種類、對話框一個區塊。
  - (b) 不記，留到 A8 再定。代價：在那之前沒有任何紀錄能說明有沒有告知，暫不告知的也沒有人會想起來。

**Q2 6 個月的範圍：整個店家的個資，還是只有註冊、身分與連線資料？**
- 現況：清理是一次清掉整個 stint 的個資（G11）。rb R5 寫「KYC 個人資料的保存期」。
- 業界基準：第 37 條第 1 項的範圍是「用戶註冊及確認身分程序所得資料、連線紀錄或交易紀錄等」（G12）；資料最小化（GDPR 5(1)(c)）只留必要的。店家聯絡資料是註冊資料、同意紀錄的 IP 與 User-Agent 是連線紀錄；店家說明、logo 不是。
- 約束：第 37 條 REAL；「判斷式只有一支」SELF-IMPOSED。
- 選項：
  - **(a) 建議：整個 stint 一起延長到 6 個月。** 多保存的只有店家說明與圖片這類本來就公開過的內容。
  - (b) 分兩段：30 天清店家說明與圖片，6 個月清其餘。要兩個 `retire_after` 與兩段清理，換到少保存 5 個月的公開資訊。

**Q3 指名非賣家帳號的來函**
- 已決定：登記照收、3d 的待辦照樣出現（B17），只有保存待定。
- 業界基準：legal hold 由紀錄管理系統統一施加在所有保存處（例如 Google Vault、Microsoft Purview 的 hold 跨服務生效）；在我們這裡，最底層的 identity 擁有 hold，所有模組清理前都能問到。
- 約束：identity 與 accountdeletion 目前沒有任何保存期規則（SELF-IMPOSED 的現況）；第 37 條第 2 項的保存義務是 REAL。
- 選項：
  - **(a) 建議：本刀不做 identity 的 hold，詳情頁明示「買家與帳號資料的保存尚未支援」**；觸發：第一份指名非賣家帳號的來函，或下一題的 identity 保存規則那一刀，兩者先到者。那時 hold 移到 identity，seller 改問它。
  - (b) 本刀就在 identity 建 hold，seller 與 accountdeletion 都問它：範圍擴到另外兩個模組。

**Q4 第 37 條第 1 項在 seller 以外的部分（不需要來函就存在的義務）**
- 現況：帳號刪除立刻清掉 identity 的註冊與登入資料（隱私權政策「帳號及個人檔案原則上保存至帳號刪除」，`PrivacyPage.tsx:105`）；買家沒有任何保存期；交易紀錄在 Java order-service。
- 約束：REAL（現行法，「足以重建使用者及個別交易」）。
- 選項：
  - **(a) 建議：不在 3c-b 做，在計畫書加一個上線前的步驟「identity 與買家的保存期（詐欺條例第 37 條第 1 項）」**，擁有者是 identity（帳號與連線紀錄）與 B8（交易紀錄），觸發是**上線的阻擋條件**（與 B21 的 I-legal 同一個，寫進計畫書上線前的檢查清單）；保存期與 seller 一致（6 個月，律師確認）。
  - (b) 3c-b 一起做：本刀擴到 identity、buyer 與 accountdeletion。

**Q5 調取資料的 6 個月從哪裡起算**
- 條文：「應於收受調取通知之日起三日內提供資料，並保存六個月」（G12）。「六個月」可以讀成從收受起算，也可以讀成從提供起算（提供最晚是收受後 3 天）。
- 約束：REAL（條文），起算點沒有解釋可查（我們沒有找到函釋）。
- 選項：
  - **(b) 建議：取兩者較晚的**：登記時 `收文＋184 天`，完成調取資料的來函時延到 `提供＋184 天`（記一列 `DATA_PROVIDED` 的延長）；提供時間必須在 [收文, 現在] 之內（伺服器檢查，句子 `authority_order.provide_time_range` 待核准）。多保存最多 3 天，兩種讀法都合規。
  - (a) 只從收文起算：提供在收文後才發生時，可能少 3 天。

**Q6 起始值（rb 沒有給的值）**
- rb 給的：詢問預設 3 天、追蹤 30 天、來函的限制與暫停 48 小時、調取 3 天。
- 我提的、沒有資料依據的：詢問期限可指定 1 到 14 天，而且只在建立時指定（rb「可以改」的讀法）；附件最多 5 份（沿用通報與申覆的 `MaxEvidence`）；來函「其他」72 小時；來函自訂期限 1 小時到 30 天；對象最多 20 個、每個對象的商品最多 50 個；收文時間最多回溯 1 年；（Q1 (a) 時）暫不告知的待辦期限是確認日期後 3 天。
- 約束：全部 SELF-IMPOSED（沒有外部依據）。
- 建議：照上面的值；觸發是上線後第一次碰到範圍外的實際需求（例如賣家要求延期、紀錄超過 5 份），屆時改常數或加「延期」指令。

---

## 附錄 A：審查意見的處理（v1 → v2）

| 項目 | 處理 |
|---|---|
| BLOCKER 清理與保存的競態 | §2.4 重寫：鎖住之後以同一支查詢再判斷；寫入 hold 的一方（建立、完成、延長來函）先鎖店家列；四種交錯的證明；固定交錯的測試（§7.3）。查了既有的申覆 legal hold：同一個洞（PG 文件＋程式碼推論，沒有重現），另有「申覆在鎖之前取 now」的第二個窗，以獨立的 3c-b-0 修 |
| MAJOR Q1 法律文字 | 照 rb §4 在 I-legal；約束改 SELF-IMPOSED；後果與觸發寫在 B21；個資法第 8 條改引法條 |
| MAJOR 告知賣家 | 新的 Q1 |
| MAJOR 非賣家帳號 | 登記與待辦照收（B17、`coverage`）；保存是新的 Q3 |
| MAJOR 第 37 條第 1 項的範圍 | 新的 Q4 |
| MAJOR 保存的起算 | 新的 Q5，建議取較晚的 |
| MAJOR §1.5 的矛盾 | 明寫修訂 rb §2 第 1 點，先例 3ca C20（B20、§1.5） |
| MAJOR 來函的註記 | 一般註記拿掉；延長改成結構化的歷史表 |
| MAJOR 上傳路徑 | 量測（G24）：admin 的機敏上傳經 gateway 到 media-service；單體以 `Admin: true` 轉正；剩下的一點以 3c-b-1 的第一步驗證（B27、§5.1） |
| minor 1、2 | B5 寫明讀法；起始值併成 Q6 |
| minor 3 | 交叉引用改成 §2.5 |
| minor 4 | B29：只在個資已清理時擋，開著的詢問不動 |
| minor 5 | `case_referrals` 欄名改 `recorded_at`、`recorded_by_admin_id` |
| minor 6 | B15 寫明是對 rb R3 的延伸 |
| minor 7 | B11 寫明用收到的時間，§7.4 加測試 |
| minor 8 | B2 的觸發條件加「UNIQUE 改成每個 (案件, 對象)」 |
| minor 9 | 改用 `stint.LockAccounts` |
| minor 10 | 延長保存不擋已到期，但先鎖店家列，`coverage` 照實顯示 |
| minor 11 | 改引全國法規資料庫 `I0050021` 第 8 條 |
| minor 12 | Q2 的約束標 SELF-IMPOSED |
| minor 13 | §5.4 (g) 補 7 句介面文案與 8 句伺服器句子 |
| minor 14 | §8 加 3c-b-0、3c-b-1 的上傳驗證與 migration 測試處理、3c-b-4 依 Q2 變大；§9 寫明 3ca §9 整段加兩支 App |

### 第二輪審查（v2 → v3：0 個 blocker、1 個 major、7 個 minor）

| 項目 | 處理 |
|---|---|
| MAJOR 食品登錄刪除不在競態的證明裡 | §2.4 拆成兩步（鎖登錄列與店家列 → 新語句只刪仍成立的），固定交錯測試，J6 擴到它；main 上沒有洞（條件只有單調的時間、只作用在已清理的店家），所以放 3c-b-3 不放 3c-b-0 |
| 申覆的修法依賴時鐘 | 鎖後檢查 `pii_purged_at`，測試刻意讓申覆的時鐘比清理早；檢查限縮在「造成關店的終止」（其他措施不套用） |
| 關聯終止的殘餘窗 | 不再接受：照撤銷的先例預讀、升序鎖、重讀、`retryScope`，進 3c-b-0 |
| Q1 暫不告知沒有後續；「已有措施」太寬；非賣家對象 | 告知改成每個對象一筆；`WITHHELD` 必填確認日期、到期進 3d 待辦、`RecordAuthorityTargetNotice`；`MEASURE_IS_NOTICE` 只限暫停與限制商品、而且已有導出的措施；用語改「被指名者」 |
| Q4 的觸發 | 改成上線的阻擋條件，與 B21 相同 |
| Q5 的 `provide_time` | 範圍 [收文, 現在]、只限調取資料，兩句伺服器句子 |
| L15 殘留「內部註記」 | 刪 |
| 涵蓋狀態聚合 | 改成逐家列出（`covered_stores`），不聚合 |
| B29 已清理後仍開著的詢問 | 寫進 §10 的自行決定說明 |
| 子表只用再判斷的名單 | §2.4 明寫 |
| 實作 3c-b-0 時的修訂 | §2.4 第二個窗的鎖後檢查只看措施所在的店家（見該段）；§7.3 的「B 先鎖住、A 跳過」與 `retryScope` 各有確定性測試（measure 側加 `SetRequestHooks`：預讀之後、鎖成功之後；hook 與 setter 在 `hooks_integration.go`，只在 integration build tag 編譯，正式程式是空的 `hooks_prod.go`）；`retryScope` 次數用完的句子進模組目錄 `measure.retry_exhausted`（Will 2026-10-19 已核准，見本節文末的文案核准表） |

### 開工前的 design-review（v3 → v3.1，7 項）

| # | 處理 |
|---|---|
| 1 來函的保存影響恢復營業 | 改：`RetireDueStores` 加 `apply_preservation_hold`，保留期已過（恢復與清理共用）與可以清理（加保存）分開；被保存而保留期已滿的店撤銷終止時只撤銷紀錄；`TestReopenIgnoresTheAuthorityHold`（§2.4、B18） |
| 2 食品登錄刪除寫了兩份述詞 | 改：`ExpiredFoodRegistrations` 以候選再呼叫一次，刪除不帶條件（§2.4） |
| 3 回覆只掛在 REFERRED 交給測試 | 改：回覆存固定的 `outcome` 加 CHECK、複合外鍵指向 `case_referrals (case_id, outcome)`（§2.2） |
| 4 新資料沒有保存期 | 記錄：B31，列進計畫書 I-legal 的檢查清單與 CARRYOVER；本刀不加機制 |
| 5 來函的擁有者 | 記錄：B32 與 CARRYOVER 的一列，觸發同 Q3／Q4 |
| 6 冪等鍵的範圍 | 改：(建立的管理員, request_id)，同 `CreateMeasure`（B30） |
| 7 對象的唯一鍵用 COALESCE、沒擋重疊 | 改：`UNIQUE NULLS NOT DISTINCT`；整個帳號加它的店家是重疊，回 `authority_order.target_overlap`（句子 Will 2026-10-19 核准） |

## 附錄 B：沒有驗證的部分

- 沒有跑任何 build、測試、sqlc 或 Atlas；§2 的 SQL 是草圖，Atlas 產生的 DDL 以實際輸出為準（`CHECK (...) IS TRUE` 的寫法照 3ca 附錄 C 的 NULL 修正，實作時以 `TestSchemaConstraints` 逐條驗證）。
- 清理與 hold 的競態（新的與既有的）是依 PG 文件與程式碼推論，沒有重現；3c-b-0 的固定交錯測試會先證明它失敗。
- （3c-b-1 已量測，不再是推論）media-service 對 `Admin: true` 的機敏轉正（B27）：本機的 Java media-service 加 Go gateway（loopback）實跑，經 gateway 建 `isSecure` session、`PUT secure_upload` 取得 stagedToken，再用單體的 `FinalizeEvidence`（`Uploader{Admin:true}`）轉正，`Asset.Secure = true`；別的帳號 ID 轉正被拒。**仍未驗證**：R2 位元組讀回（`ReadSecure` 與檔案路由回位元組；離線沒有 R2，讀回路徑只以單體測試的 media 替身驗到「先記讀取紀錄再讀」）；觸發條件：B1 media 搬進單體，或第一次在有 R2 的環境部署。其餘仍是推論，實作時確認：`H/rpc` 是否已有 `google.type.Date` 的轉換（§4）、3c-a 的 migration 測試套用到哪一版（§2.5）、恢復營業呼叫 `RetireDueStores` 時是否已持有店家列的鎖（§2.4）：推論，實作時確認。
- 我們有沒有其他「連線紀錄」要保存：seller 裡只量到同意紀錄的 IP 與 User-Agent；identity 與 legacy 服務沒有查（併入 Q4）。
- 本機 DB 有沒有 `report_cooperation_requests` 的列：沒有查；有的話 migration 會失敗並列出，照 3ca §2.3 重建。
- §5.4 的句子只照術語表檢查用詞，沒有和條款逐字對照；「Hawkr」是隱私權政策裡的平台名稱，`.md` 的那一句沿用它。

---

## §5 頁面與文案摘要（一次核准）

頁面與區塊：

| # | 位置 | 內容 | 建議 |
|---|---|---|---|
| P1 | 案件詳情：新區塊「詢問賣家」（「通報」之後） | 每筆詢問一張卡；頁首「詢問賣家」；卡片上「記下回覆」「撤回」 | 照 §5.2 |
| P2 | 案件詳情：轉報區塊改寫 | 追蹤日期、主管機關的回覆、「不轉報」；兩個新按鈕 | 照 §5.2 |
| P3 | 商家通報的案件分頁籤 | 狀態篩選加「等賣家」 | 已核准（3cd D′2 經 rb §3） |
| P4 | 新頁 `/admin/authority-orders` | 處理中｜已完成兩個分頁籤 | 照 §5.2 |
| P5 | 新頁 `/admin/authority-orders/new` | 登記來函的表單頁（對象有多列） | 照 §5.2 |
| P6 | 新頁 `/admin/authority-orders/[id]` | 來函、指名的對象（逐家涵蓋狀態）、處理、保存（延長歷史）、導出的措施；完成、延長保存、三個建立措施的入口 | 照 §5.2 |
| P7 | 側欄 | 「賣家 / 商品」組，「商家通報」之後加「主管機關來函」 | 照 §5.1 |
| P8 | 對話框 7 個 | 詢問賣家、記下回覆、撤回詢問、不轉報、記下主管機關的回覆、完成來函、延長保存 | 照 §5.3 |

待核准的文案（全文見 §5.4 (a)–(g)，這裡列每一句的建議）：

| # | 位置 | 文案 | 建議 |
|---|---|---|---|
| I1、I3、J1 | 區塊、按鈕、對話框標題 | 詢問賣家 | 照用 |
| I2 | 區塊說明 | 這裡只記下詢問；賣家端的畫面上線前，系統不會通知賣家，請另外聯絡賣家。詢問只寫你轉述的內容，不要附上通報原文或通報人的資料。 | 照用（3cd D′7 句型） |
| I4 | 空狀態 | 還沒有詢問 | 照用 |
| I6 | 種類 | 提供紀錄｜請說明 | 照用（rb 原文） |
| I7 | 狀態 | 等賣家回覆｜已回覆｜已撤回 | 照用 |
| I8–I9 | 欄位 | 建立｜期限｜給賣家的說明；回覆｜管道｜收到的時間｜記下｜附件 | 照用 |
| I10 | 管道 | 電話｜Email｜通訊軟體｜其他 | 照用 |
| I11–I13 | 附件、撤回 | 附件 {n}；已於 {時間} 撤回（{原因}）；管理員撤回｜案件結案時撤回 | 照用 |
| I14–I16 | 按鈕、讀取失敗、標記 | 記下回覆｜撤回；詢問讀取失敗；等賣家 | 照用 |
| I17 | 結案對話框提示 | 還有一筆詢問在等賣家回覆；結案會撤回它。 | 照用 |
| J2–J6 | 詢問對話框 | 種類的說明（條款 11.3）、「預設 3 天後；可以設在 1 天到 14 天之間。」、送出「記下詢問」、「請填寫給賣家的說明。」 | 照用 |
| J7–J11 | 回覆對話框 | 記下賣家的回覆；說明「記下之後這筆詢問就結束；要再問，請新增一筆詢問。」；送出「記下回覆」；已記過的橫幅 | 照用 |
| J12–J15 | 撤回對話框、失敗 | 撤回之後賣家不必再回覆…請另外告知賣家；送出「撤回」；已撤回的橫幅；網路失敗句 | 照用 |
| K1 | 已轉報 | 沿用 3c-a | 已核准 |
| K2–K6 | 轉報區塊 | 追蹤日期 {日期}；主管機關的回覆／還沒有回覆；已於 {時間} 標記不轉報：{原因}；按鈕「不轉報」「記下主管機關的回覆」；「追蹤日期（預設 30 天後）」 | 照用 |
| K7–K9 | 不轉報、回覆對話框 | 「例如主管機關已經介入。標記之後不能再記錄轉報。」；送出「標記不轉報」；回覆的欄位與「不再追蹤」 | 照用 |
| K10 | 轉報資料 `.md` 的開頭句 | 本轉報由 Hawkr 平台以平台名義提出；通報人的聯絡方式不在本檔。 | 照用 |
| L1–L3 | 側欄、標題、說明、分頁籤 | 主管機關來函；頁面說明（保存不被清理）；處理中｜已完成 | 照用 |
| L4–L8 | 清單欄位、種類、多家、空狀態 | 見 §5.4 (d) | 照用 |
| L9–L14 | 登記頁 | 登記來函；欄位；對象的欄位；期限與保存的說明；送出「登記」 | 照用 |
| L15–L17 | 詳情 | 區塊名（不含內部註記）；保存到 {時間}（保存中／已到期）；按鈕 | 照用 |
| L18–L22 | 完成、延長、多帳號、衝突、讀取失敗 | 見 §5.4 (d) | 照用 |
| M1–M5 | 審查補的介面句 | 已標記不轉報的橫幅；「這個措施會記為主管機關來函 {文號} 導出的措施。」；延長歷史；結案後的回覆寫註記；逐家的涵蓋狀態 | 照用 |
| M6、M8 | 告知被指名者（Q1 (a) 時） | 已告知｜機關要求暫不告知｜由措施告知；「記下告知」與待辦「告知被指名者」 | Q1 選 (a) 才需要 |
| M7 | 提供資料的時間 | 「保存期限會延到提供後 6 個月。」 | Q5 選 (b) 才需要 |
| (e)、(g) | 伺服器句子（約 30 句，`staff` 段） | 見 §5.4 (e) 與 (g) 的兩張表 | 照用 |
| (e) 新增 | 對象重疊（`authority_order.target_overlap`） | 這個帳號已經整個列為對象，不用再列它的店家。 | 照用（design-review #7 新增，Will 2026-10-19 核准） |
| (f) | 術語表 | 詢問、主管機關來函、保存期限 | 照用 |

## 給 Will 的決定題

| # | 題目 | 建議 |
|---|---|---|
| Q1 | 來函完成時，要不要逐一記下「是否已告知被指名者」 | (a) 必填：已告知／暫不告知（要填確認日期，到期進待辦）／由措施告知（只限暫停、限制商品而且已有措施） |
| Q2 | 6 個月的保存涵蓋整個店家的個資，還是只有註冊、身分與連線資料 | (a) 整個 stint 一起 |
| Q3 | 指名非賣家帳號的來函：登記與待辦照收，非賣家資料的保存怎麼辦 | (a) 本刀不做 identity 的 hold，畫面明示；第一份這種來函或 Q4 那一刀先到者做 |
| Q4 | 詐欺條例第 37 條第 1 項在 identity 與買家的部分 | (a) 計畫書加上線前的步驟（identity＋B8），列為上線阻擋條件 |
| Q5 | 調取資料的 6 個月從收文還是從提供起算 | (b) 取較晚的，完成時自動延長，提供時間限在 [收文, 現在] |
| Q6 | rb 沒給的起始值 | 照 Q6 列的值；第一次碰到範圍外的需求再改 |
| — | §5 的頁面與文案 | 已核准（2026-10-19；design-review 新增的 `target_overlap` 與修正輪的句子 2026-10-19、10-20 核准） |
