# 10e 第 5a 刀：PayUni 狀態由伺服器推導（取代 website 與 seller-app 各自的 `payuniKindOf`／`payuniCopy`），實作設計 v1.2

> **狀態**：v1.2。v1 經一輪獨立審查（1 HIGH、7 MED、5 LOW，全部處理，對照見附錄 A；審查全文在 session scratchpad `5a-design-review.md`，不入庫）；修訂時另查出 2 項（附錄 A 的 R1、R2）；與 5b v1.1 的界線已對齊（F11、F12、§7.1）。**Will 2026-10-21 對 v1.1 文末「待 Will 決定」1–7 全照建議**：F5 選 (a)（`MEMBER_99` 不能自行重送，KYC ADR 文末修訂）、F7、F9、F10、F11 照做、R1 延後、§5 的 N 類文案 14 列核可。**開工前 design-review（7 項全改，附錄 B）**；其中 **Will 2026-10-21 的兩個決定**：(1) PayUni 狀態放進 `KYCFlow`（`GetMyKYC` 一次回），取代 v1.1 的「放在 `GetMyPayUni`、client 合併兩份回應」；(2) `SaveKYCDraft`／`SubmitKYC` 不能做時用「`can_resubmit`＋原因句」（lb D-L2 的形狀），取代 v1.1 F8 的「狀態說明當錯誤」。修訂時另查出 1 項（附錄 B 的 R3：PayUni 拒絕後存草稿，申請回到 DRAFT，v1.1 會讓問題框消失）。v1.2。**5a 完成**：完成前 design-review 8 項（附錄 C）與兩輪 code-review 全部處理，與 v1.2 不同的地方在「實作時的修訂」；5a-2 文件已寫入計畫 10e 與 L′ §8；5b、5c 未做。 新增與改寫的文案見 §5 末「待 Will 核可（v1.2 新增）」。

讀取的是 `local/10e` 的 `a6f7b3815`，worktree 沒有未提交的程式改動［量測：`git status --short` 只有 5a、5b、5c 三份設計書］。
盤點：2026-10-21 的唯讀盤點（下稱「盤點」，session scratchpad `inv-payuni.md`，不入庫）。本文只把自己重新讀過程式的事實標成［量測］，其餘引用盤點的標［推論］或註明出處。
SSOT：計畫 10e 段（`2026-09-29-a3-buyer-seller.md:346-387`，PayUni 這一項在 :382，admin 原因句在 :383）；`step10e-labels-brief.md`（下稱 lb：:7 的規則、D-L2）；`step10e-L-brief.md`（下稱 Lb：Q3、「知會」）；`step10e-Lprime-design.md`（下稱 L′：§1.1、§1.3、§1.5、§2.2、§3、§5.2、§8 範圍外 :822）；ADR `2026-10-17-multi-locale-architecture.md`（下稱 i18n ADR：D1 :55 發佈門檻、D2 :78 禁止拼接、D3、:116 UGC 與第三方原文）；ADR `2026-10-19-seller-kyc-payuni-consolidated.md`（下稱 KYC ADR：:81；取代 10-02 那份）；`step10c-decision-brief.md` D1 第 1 點（下稱 10c-D1）；`step10b-decision-brief.md` K4；`step10e-3d-design.md`（後台待辦，下稱 3d）；API 規範 `backend/docs/開發規範/API 設計規範（Connect）.md`（:102 未知值）；術語表；root、backend、seller 模組、website、seller-app、admin 的 `AGENTS.md`；seller `CARRYOVER.md:256`、`:334`；姊妹刀 `step10e-5b-design.md`。
路徑縮寫：`W` worktree 根、`M`＝`W/backend/internal/modules/seller`、`H`＝`W/backend/internal/httpapi/seller`、`WS`＝`W/website`、`SA`＝`W/seller-app/lib`、`RQ`＝`W/backend/internal/modules/reviewqueue`、`AQ`＝`W/admin/src/app/admin/queue`。
標記：**[量測]** 讀了程式或跑了指令；**[推論]** 從程式推出、沒有執行；**[外部]** 業界資料（Stripe、AIP-216 由審查者 2026-10-21 以 WebFetch 查過，本文沒有再查）。判定原則同 4b：上線前沒有相容性約束（root `AGENTS.md`「上線前不受相容性約束」）；照業界從零的做法；基準定不了的產品選擇、UI 改動、改契約、改寫已核准決定的列給 Will。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| `WS/lib/seller/payuni.ts` 的 `payuniKindOf`、`payuniCopy`、`payuniBannerOf`、`payuniProblemFieldLabels`、`payuniReason`、`fetchMyPayUni`；`WS/lib/hooks/usePayuniStatus.ts` | 本刀刪（`GetMyPayUni` 刪除，F1、F11）。沒有本刀的話，I-sweep 會先把這些中文字面值抽出來，等於替要刪的程式做 i18n | 刪除，不在裡面最佳化 |
| `SA/features/seller/data/payuni.dart` 的 `PayuniKind`、`payuniKindOf`、`payuniPanelCopy`、欄位與原因 helper、`accountPayuniProvider`／`currentPayuniProvider` | 本刀刪；I-app 的抽字同上 | 刪除 |
| `M/internal/model/payuni.go` 的推導（`DeriveKYCFlow`、`ProblemOf`、`ClassifyPayUniCode`） | 沒有。模組的終點 | 會留下，本刀在旁邊加狀態推導 |
| `M/internal/kyc/kyc.go` 的 `editable`（:228-256）與 `Submit` 的狀態檢查（:381-395） | 沒有；`apperr` 字面值的清理（I-server）會逐模組改 | 會留下，本刀改成同一個原因函式（F8） |
| `H/payuni.go`（`GetMyPayUni`、`toPayUni`）、`M/internal/payuni/intent.go` 的 `Mine` | 本刀刪（F11） | 刪除 |
| 目錄 `seller.KYCFlowStatus` 的 `banner_*`、`gate_*` 變體；`WS/lib/seller/kyc-flow.ts` 的 `kycFlowBanner`、`kycFlowGate`；`SA/features/seller/data/kyc_flow.dart` 的 `kycFlowCopy` | 沒有排程；本刀依 F9 拿掉 PayUni 階段的 6 個值 | 會留下，型別改成「可以沒有文字」（F15） |
| `M/db/objects.sql` 的 `pub_review_items`（3d 的後台待辦） | 沒有；3d 是終點 | 會留下，本刀加一個成員（F16） |
| admin 的 PayUni 文字（`payUniBadge`、`payUniProblemText`、綁定失敗句、`canRebind`、`PAYUNI_REONBOARDING_TEXT`） | 第 5b 刀 | 本刀不動（F12）；`AQ/queue-item.ts` 只加一個種類（F16） |

結論：client 這一側是刪除；伺服器這一側是終點，照從零的做法做。

---

## 0.1 範圍

| 部分 | 內容 |
|---|---|
| 契約 `api/hawkr/seller/v1alpha1/seller.proto` | 新 enum `PayUniState`（8 個值，F1）與 message `SellerPayUni`；`KYCFlow.payuni`、`KYCFlow.resubmit_unavailabilities`，新 enum `KYCResubmitUnavailableReason` 與 message `KYCResubmitUnavailability`（F8）；**刪 `GetMyPayUni`、`GetMyPayUniRequest`／`Response` 與賣家端 `PayUniRegistration`**（F11）；刪 `PayUniProblem.remedy` 與 `PayUniRemedy`；`PayUniProblem` 註解改成 admin 用（F13） |
| 契約 `api/hawkr/admin/reviewqueue/v1alpha1/review_queue.proto` | `REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY = 12`（F16） |
| seller 模組 | `model.PayUniPhaseOf`（若 5b 還沒合併，F2）、`model.SellerPayUniOf`、`PayUniState.Explanation`、`model.ResubmitUnavailableReasons` 與原因句（`internal/text`，F7、F8）；`DeriveKYCFlow` 的 PayUni 階段 `CanResubmit` 改由同一個推導決定（F5 (a)、F17）；`editable` 與 `Submit` 改回第一個原因句；`ProblemOf` 去掉 `reason` 前後空白；身分鎖的欄位清單搬到 `model`（F14）；`Rebind` 在申請不是已核准時拒絕（F17）；`payuni_registrations.bind_failed_at` 欄位與 `review_payuni_bind_retry` 待辦成員（F16）；刪 `payuni.Service.Mine`、`Module.MyPayUniRegistration` |
| reviewqueue 模組與 handler | `Kind` 加 `PAYUNI_BIND_RETRY`，`kindPermissions` 對到重新綁定的讀取權（F16） |
| 共用目錄 | `seller.PayUniState` 8 個名詞（F6）；兩個小標題（F7）；`seller.KYCFlowStatus.banner_title`／`banner_description`／`gate_heading`／`gate_message` 拿掉 PayUni 階段的 6 個值（F9） |
| website | 錢包狀態面板、儀表板提示條、認證頁的鎖住畫面與問題框、物流與錢包的認證閘門、`walletShowsTransactions`、`Link.tsx` 的預取改讀 `KYCFlow.payuni`；刪 `usePayuniStatus`、`queryKeys.seller.payuni`、`payuniKindOf` 等；`kycFlowBanner`／`kycFlowGate` 改成可以沒有文字（§3.1） |
| seller-app | 錢包狀態面板、`walletCanWithdraw`、認證閘門改讀 `KYCFlow.payuni`；認證頁補上 PayUni 狀態（F10）；刪 `GetMyPayUni` 的 provider 與 `seller_status_refresh` 的 PayUni 失效；`kycFlowCopy` 的橫幅與閘門欄位改成可空（§3.2） |
| admin | `AQ/queue-item.ts` 的 `QUEUE_KIND_TEXT` 加一個種類（F16）；其餘不動（F12） |
| 文件 | 計畫 10e 的完成紀錄；10c-D1 第 1 點被取代的紀錄（F13）；seller `CARRYOVER.md:334` 改寫（F1：原依據不成立）；seller `AGENTS.md` 的 PayUni 段；KYC ADR :81 文末修訂（F5 (a)、F17）；L′ §8 範圍外那一條的 Pending 結案 |

## 0.2 不在範圍

- **admin 看到的所有 PayUni 文字與可用性歸第 5b 刀**（F12）：`payUniBadge`（`admin/src/app/admin/merchants/[id]/kyc-application.ts:944-977`）、`payUniProblemText`（`:997-1003`）、`payUniProblemFields`（`:1005-1012`）、`PayUniCards.tsx:115` 起的「綁定失敗…已嘗試 N 次」句、`canRebind` 與 `PAYUNI_REONBOARDING_TEXT` 都在 5b（`step10e-5b-design.md` §0.2 與 F5）。5a 只處理賣家面，加上後台待辦的一個種類（F16，§7.1 寫明與 5b 的分工）。
- buyer-app `_labelOf`（計畫 :382 並列的另一項，L′ §8）：觸發是 website 地址改用 proto enum。
- **PayUni 拒絕之後重新送出、平台再核准、PayUni 還沒重審的這段期間**（R1）：註冊維持 BOUND 和舊的拒絕結果（`M/db/queries/payuni.sql:25-41`），流程回到 PAYUNI_REJECTED、`CanResubmit` 為真，所以本刀之後畫面仍是 FIXABLE。這是 seller `CARRYOVER.md:256` 已記的缺口，觸發不變：拿到 PayUni sandbox、確認 `update_merchant` 之後狀態怎麼變，再以 `resubmitted_at` 加一個值（Will 2026-10-21 同意延後）。
- 模組裡兩處 `strings.Join(labels, "、")`（`kyc.go:635` 賣家面、`model/prospect.go:455` 後台）：本刀不新增列表格式（F7 把欄位清單從句子裡拿出來），兩處隨 I-server 的 `apperr` 清理處理；屆時若句子仍要嵌清單，才新增 `uitext` 的列表格式並一次改這兩處。
- `FIX_VIA_SUPPORT` 指出店家欄位、賣家修正之後由平台重新綁定（不是自動）：本刀照「修正後請聯繫客服」；要改成店家欄位改完自動重新綁定是另一個機制（`BumpPayUniIntent` 對 FAILED 不排 job，P4），沒有排程，不是本刀承諾的事。

（v1.1 的第 4 點閘門文字、第 5 點沒有讀者的欄位、第 7 點 `FIX_VIA_SUPPORT` 指出申請欄位，依開工前 design-review 改在本刀處理：F9、F11、F17。）

---

## 0.3 決策總表

| # | 題目 | 決定 | 從零的理由／必要條件 |
|---|---|---|---|
| F1 | 狀態放哪裡（**Will 2026-10-21 決定**） | **放在 `KYCFlow`**：`KYCFlow.payuni`（新 message `SellerPayUni`，`OUTPUT_ONLY`）帶 `state`（新 enum `PayUniState`，頂層，值帶 `PAY_UNI_STATE_` 前綴：buf lint STANDARD 的 `ENUM_VALUE_PREFIX` 與 API 規範 :101）、`explanation`（`google.rpc.LocalizedMessage`）、`reason`（PayUni 的審核意見原文，標 zh-TW）、`fix_fields`／`fix_store_fields`（賣家此刻能改的欄位，F14）。`GetMyKYC` 與 `SaveKYCDraft`、`SubmitKYC`、`CancelKYC` 的回應（都回 `KYCApplication`）一次帶到。**`payuni` 只在 PayUni 階段（申請已核准），或賣家正在修正 PayUni 拒絕的那份申請（草稿，R3）時設**（F3）。8 個值：REVIEWING、PENDING_INFO、FIXABLE、FIX_VIA_SUPPORT、CONTACT_SUPPORT、RETRY_LATER、TERMINATED、APPROVED，**每個值對賣家只有一種動作**（等、看 PayUni 的信、改申請後重送、改店家設定後找客服、找客服、等平台重送、關店重新入駐、無）。**`GetMyPayUni` 刪除**：Will 的決定是它只留還需要的東西，而查證後沒有剩下的——聯絡資料早已是店家欄位（賣家端 `PayUniRegistration` 的 `reserved 9-14` 是 `contact_*`、`mer_*`；RPC 註解「送給 PayUni 的聯絡資料是店家的欄位（UpdateMyStore），不在這裡」，P24），其餘欄位在本刀之後沒有讀者（F11）。比較過的替代：(b) 擴充 `KYCFlowStatus` 的值：它給閘門、admin 清單篩選、`store-row` 分組與 5 組目錄變體用（P11），補上補救方式會讓每個 switch 一起膨脹；(c) v1.1 的放在 `GetMyPayUni`：依據是 seller `CARRYOVER.md:334`「沒有畫面需要在同一個呼叫拿到兩者」，今天不成立（三個畫面都同時讀兩者，P25），而且逼出 client 的合併規則、快取時差規則、送出後的交叉失效與 not_found 後備。權限：`GetMyKYC` 要 `CapAccountSelfRead`、`GetMyPayUni` 要 `CapOrderFulfill`，兩者的條件都只有 `RequirementActive`，看得到的賣家是同一群（P23），所以搬過去不改變誰看得到。`KYCFlow` 也是 admin `KYCApplication.flow` 的型別（P16）：`payuni` 與 `resubmit_unavailabilities` 是賣家受眾，admin 的轉換不設，契約註解寫明，handler 測試釘住；admin 的 PayUni 呈現是 5b 的 `PayUniRegistration.flow_status` | 業界：一個值是另一個值的細分時放在同一個資源、一次讀到（Stripe Account 上同時有 `requirements.disabled_reason` 與 `charges_enabled`［外部］；AIP-216 的 state 是資源本身的 `OUTPUT_ONLY` 欄位［外部］）。必要條件：兩個 client 現在各自從 5 個欄位加 `can_resubmit` 推導同一個狀態（P1），違反 lb :7 |
| F2 | 推導規則 | `model.SellerPayUniOf(app KYCStatus, sameApplication bool, reg *PayUniRegistration) (SellerPayUni, bool)`，`sameApplication` 是 `reg.ApplicationID == 目前申請的 ID`。**PayUni 的階段只由 `PayUniPhaseOf(reg)` 決定**（5b F11 從 `DeriveKYCFlow` 抽出；5a 先合併時由 5a 抽出、5b 沿用，§7.1），本函式不自己讀 `BindStatus`、`Outcome` 判斷階段，也不以假造的「已核准」呼叫 `DeriveKYCFlow`（`CARRYOVER.md:334` 刪掉的就是那個寫法）。何時有值：申請 APPROVED（`reg == nil` 時階段是 BINDING）；或申請 DRAFT、`sameApplication`、而且推導結果是 FIXABLE（R3）；其他沒有。推導依序：①`NeedsReonboarding` → TERMINATED；②`PaymentEnabled` → APPROVED；③階段 BINDING、FORWARDED、UNDER_REVIEW → REVIEWING；④PENDING_INFO → PENDING_INFO；⑤PAYUNI_REJECTED：補救方式是 CONTACT_SUPPORT → CONTACT_SUPPORT，否則 FIXABLE（F5 (a)）；⑥BIND_FAILED：補救方式 RETRY_LATER → RETRY_LATER；SELLER_FIX 而且 `fix_fields` 非空 → FIXABLE；SELLER_FIX 而且只有 `fix_store_fields` → FIX_VIA_SUPPORT；其他（CONTACT_SUPPORT、不認得的碼、SELLER_FIX 卻沒有任何可改的欄位）→ CONTACT_SUPPORT（F17）。**`DeriveKYCFlow` 在 PayUni 階段的 `CanResubmit` 改成「同一個推導的結果是 FIXABLE」**，所以「能不能重送」只有一個判斷；`payuni.Editable` 改回 `flow.CanResubmit`（申請已核准時）。①–⑤與兩個 client 的分支逐一對應（`WS/lib/seller/payuni.ts:62-85`、`SA/features/seller/data/payuni.dart:39-64`），⑥是 F17 的新規則 | 一個推導只有一個實作（seller `CARRYOVER.md:334`）；`can_resubmit` 的規則是 KYC ADR :81（本刀文末修訂），不在 client 重算 |
| F3 | 申請不在 PayUni 階段時（重新送出之後、被平台退回、取消） | **不加 RESUBMITTED；`payuni` 不設，畫面看 `KYCFlowStatus`**（§3.0）。例外是 R3：PayUni 拒絕（或 F17 的綁定失敗）之後賣家按「前往補件」存草稿，申請回到 DRAFT（`UpdateKYCDraft` 一律 `SET status = 'DRAFT'`，P26），註冊仍是那份申請的拒絕；這時賣家正在做 FIXABLE 的動作，問題框（審核意見、欄位）要留著，所以 `sameApplication` 的草稿仍回 FIXABLE。送出（SUBMITTED）之後回到平台審核，`payuni` 不設；被平台退回（REJECTED）、取消（NOT_STARTED）也不設，舊的 PayUni 問題不會出現；取消後重開的新申請 `sameApplication` 為假。v1.1 說「草稿而且註冊是 PayUni 拒絕到不了」不成立（附錄 B 的 R3） | 計畫 :382 只要求伺服器推導 PayUni 狀態；流程狀態已是兩條軸合成的結果（`DeriveKYCFlow`），在 PayUni 階段之外它就是唯一來源，草稿的例外由伺服器同一個函式決定 |
| F4 | RETRY_LATER 的說明 | 值保留、說明是 v1.1 核可的 #10「…平台會協助重新送出，你不需要重新送出。」**「平台會協助」改由機制成立**：重試用完的綁定失敗進後台待辦（F16），有期限，管理員重新綁定後待辦消失 | 文案要描述伺服器的實際行為；v1.1 引用的 `PayUniBindFailed` 事件沒有訂閱者（P30），承諾只靠員工主動看清單 |
| F5 | PayUni 拒絕、補救方式是聯絡客服、但規則允許重送 | **(a)（Will 2026-10-21）**：PayUni 拒絕而且補救方式是 CONTACT_SUPPORT 時不能重送；v1.2 由 F2 ⑤ 與「`CanResubmit` ＝ 推導結果是 FIXABLE」一起表達，不另寫條件。KYC ADR :81 文末修訂 | KYC ADR :81；P5；CodeMeta `業務狀態碼.md:276-296` |
| F6 | 標題 | 只由 enum 值決定 → 共用目錄 `seller.PayUniState.<VALUE>`（預設受眾，zh-TW；en 照 L′ §1.1 先空），8 個值都要有（L′ §2.2；APPROVED 不顯示也要有名詞）。REVIEWING 統一成「PayUni 審核中」 | lb :7；Lb Q3；L′ §5.2 守門 |
| F7 | 說明句、審核意見與欄位清單 | **說明句是伺服器 `LocalizedMessage`**（seller 模組 `internal/text`，預設受眾），**只由 `PayUniState` 決定**：`func (s PayUniState) Explanation(l uitext.Locale) (uitext.Message, bool)`，key `payuni.state.<state>`，**7 則**（APPROVED 沒有說明：畫面不顯示面板，v1.1 為錯誤路徑寫的 #13 隨 F8 刪除），沒有參數。審核意見（`reason`）與要修正的欄位（`fix_fields`、`fix_store_fields`）是 `SellerPayUni` 的獨立欄位，client 在說明下方分開顯示：審核意見照 i18n ADR :116「維持原文、容器標語系」（website `lang="zh-TW"`、seller-app `Text` 帶 `Locale.fromSubtags(languageCode: 'zh', scriptCode: 'Hant', countryCode: 'TW')`）；欄位以目錄名詞列成清單，各端照自己的表單控制項去重（P14）。兩個小標題「PayUni 的審核意見」「請修正以下資料」放共用目錄（Lb Q3） | lb :7、D-L2；i18n ADR D2、D3、:116；Stripe `requirements` 的 `disabled_reason`、`currently_due`、`errors[].reason` 分開［外部］；先例 `OpeningBlocker.Text(l)`（`M/internal/model/model.go:117-130`） |
| F8 | `SaveKYCDraft`／`SubmitKYC` 不能做時（**Will 2026-10-21 決定：can＋原因句**） | 兩個 RPC 能不能做是同一個條件（`editable` 與 `Submit` 都放行 DRAFT、REJECTED 與「已核准而且可以重送」，P33），所以沿用既有的 `KYCFlow.can_resubmit`（意思寫成「此刻 `SaveKYCDraft` 與 `SubmitKYC` 會被接受」），旁邊加 `repeated KYCResubmitUnavailability resubmit_unavailabilities`（`can_resubmit` 為假時至少一個，依先後；為真時是空的），形狀照 lb D-L2 與 seller 既有的 `StoreOpeningUnavailability`（`seller.proto:470-475`）。原因 enum `KYCResubmitUnavailableReason`：`UNDER_REVIEW_CANCELABLE`、`UNDER_REVIEW`、`CANCELLED`、`PAYUNI_REVIEWING`、`PAYUNI_PENDING_INFO`、`PAYUNI_FIX_VIA_SUPPORT`、`PAYUNI_CONTACT_SUPPORT`、`PAYUNI_RETRY_LATER`、`PAYUNI_TERMINATED`、`PAYUNI_APPROVED`（PayUni 的 7 個對到 `PayUniState` 裡不能重送的 7 個值；送審中依身分鎖分兩個，現行 `editable` 本來就分兩句）。`model.ResubmitUnavailableReasons(status, flow) []KYCResubmitUnavailableReason` 是唯一的判斷，`func (r KYCResubmitUnavailableReason) Text(l) uitext.Message`（key `kyc.resubmit_unavailable.<reason>`，§5 #27–#36）是唯一的句子；`editable` 與 `Submit` 都呼叫它，不能做時回 `apperr.FailedPrecondition` 帶第一句（照 `OpeningBlocker.Err`，`model.go:132-147`）。到不了的分支（現行 `editable` 的 default）回 internal，不留字面值。原因句回答「為什麼不能改或重送」，不是狀態描述，所以不再需要 v1.1 的 #13（APPROVED 只為錯誤寫的說明）與 #25（把狀態說明當錯誤）。client 不依原因分支，只顯示句子（「前往補件」看 `can_resubmit`，F14） | lb :7「與 `failed_precondition` 錯誤用同一個 Go 函式產生」；lb D-L2；5b F5 的 `can_rebind`／`rebind_unavailabilities` 同形狀；AIP-193 的錯誤 message 描述失敗原因［外部］ |
| F9 | 同一個狀態只留一個文字來源 | 有 `KYCFlow.payuni` 時，所有 PayUni 狀態的呈現（錢包面板、儀表板提示條、認證頁鎖住畫面與問題框、**錢包與物流的認證閘門**）一律用 `PayUniState` 的標題與 `explanation`。目錄 `seller.KYCFlowStatus` 的 `banner_title`、`banner_description`、`gate_heading`、`gate_message` 在 BINDING、BIND_FAILED、FORWARDED、UNDER_REVIEW、PENDING_INFO、PAYUNI_REJECTED 六個值以 `omit` 拿掉（共 24 則）：這六個值就是 PayUni 階段，伺服器在這時一定設 `payuni`（F2，含 `reg == nil` 的 BINDING → REVIEWING），所以 v1.1 保留的 BINDING 後備也不再需要。閘門的 `{subject}`（「通過後就能設定物流與退貨」）在 PayUni 階段不再出現：狀態說明已經說了賣家要做什麼，閘門的按鈕照現行（`action` 變體不動）。DRAFT 的閘門與橫幅照舊（R3 的草稿有 `payuni`，畫面照 §3.0 用 PayUni 文字）。理由：v1.1 延後的閘門矛盾今天就在（`gate_message.PAYUNI_REJECTED`＝「認證未通過，請修改資料後重新送審。」，而 PAYUNI_REJECTED 可能是 CONTACT_SUPPORT、TERMINATED；BIND_FAILED 的「PayUni 金流審核中」可能是 RETRY_LATER），觸發「有人回報」在上線前不會發生 | 同一個判斷一個來源；`omit` 只能用在核准過「沒有文字」的位置（L′ §1.3），所以要 Will 核可（§5 #20、#26） |
| F10 | seller-app 與 website 對齊 | v1.1 核可：認證頁鎖住畫面在有 `payuni` 時顯示 PayUni 狀態；表單可編輯而且有 `payuni` 時（只會是 FIXABLE），表單上方顯示 PayUni 問題框，沿用 `_RejectionBanner` 的卡片樣式；`_RejectionBanner`（`seller_certification_upload_screen.dart:668-690`）只在 `rejection != null` 時顯示 | 10c-D1；root `AGENTS.md`「UI 改動」 |
| F11 | 刪掉沒有讀者的契約（**依「沒有讀者 → 刪」一致處理**） | 刪 **`GetMyPayUni` RPC、`GetMyPayUniRequest`／`GetMyPayUniResponse`、賣家端 `PayUniRegistration` 整個 message**：它的欄位逐一沒有讀者——`bind_status`、`outcome`、`payment_enabled`、`needs_reonboarding`、`problem` 只有 `payuniKindOf` 與 `CertificationsClient.tsx:361` 讀（P8，本刀刪）且事實改由 `KYCFlow.payuni` 給；`bind_error_code`、`sync_pending`、`sync_error_code`、`bind_time`、`status_time`、`check_time` 在 website、seller-app 的正式程式沒有讀者，Go 只有 handler 測試讀（P32）。刪 `PayUniProblem.remedy`（`reserved 2; reserved "remedy";`）與 `enum PayUniRemedy`（契約裡沒有其他使用者；模組內部的 `model.PayUniRemedy` 照舊）。`PayUniProblem` 留給 admin `PayUniRegistration.problem`（admin `seller.proto:698`）。`PayUniBindStatus` 還有 admin 的 `bind_status` 與 `ListStoresRequest.payuni_bind_status`，保留。`PayUniOutcome` 只剩 admin 一個使用者：**後合併的那一刀刪 `PayUniOutcome`**（照建議 5a 先合併，由 5b 刪；5b F12 不採用時保留）。模組刪 `payuni.Service.Mine`、`Module.MyPayUniRegistration`、`H/payuni.go`、`declaration.go:70` 那一列 | 必要條件消失：本刀之後沒有讀者（P8、P32）；`v1alpha1` 不受 `buf breaking`（`api/buf.yaml:24-25`）；root `AGENTS.md`「不留技術債」 |
| F12 | 與第 5b 刀的界線 | 5a 只改賣家面，加上後台待辦一個種類（F16）與 `Rebind` 的申請狀態檢查（F17）；admin 的 PayUni 文字與可用性全部在 5b。`PayUniState` 是賣家受眾，員工要的是流程位置，兩刀不共用 enum；共用的是階段判斷 `PayUniPhaseOf(reg)`（F2）。兩刀共同碰到的檔案、順序與後合併那一刀要做的事見 §7.1 | 計畫 :383；5b §0.2、F5、F11、F12、§9；lb D-L2 |
| F13 | 10c-D1 第 1 點與契約註解 | 10c-D1 第 1 點的後半（不再用舊服務沒有結構的整句）仍成立；前半（client 依 `code`、`remedy` 決定文字）由 lb（Will 2026-10-17）的規則取代。`PayUniProblem` 註解（`seller.proto:796-810`）改成 admin 用：「後台看的 PayUni 問題的事實：`code` 給員工與客服排查；`fields`、`store_fields` 是碼指出的欄位（未依身分鎖過濾）；`reason` 是 PayUni 的審核意見原文（去掉前後空白，沒有時是空字串）。賣家看的是 `KYCFlow.payuni`。」計畫 10e 的完成紀錄寫明這兩個舊決定被取代的部分 | 後來的決定優先 |
| F14 | client 剩下的呈現規則（**能不能做與欄位由伺服器決定**） | **伺服器只回賣家此刻能改的欄位**：`fix_fields` 只在 FIXABLE 有值，`fix_store_fields` 只在 FIXABLE、FIX_VIA_SUPPORT 有值；兩者都依賣家型態過濾（現行 `ProblemOf` 已做，`payuni.go:542`），`fix_fields` 再依身分鎖過濾（`flow.IdentityLocked` 時去掉鎖住的欄位；鎖住的清單從 `kyc/diff.go:23-26` 搬到 `model.LockedIdentityFields()`，`checkIdentityUnchanged` 與這裡共用一份）；碼指出的是「整份表單」（`MER_8`、`MER_91`、`MER_92`、`MEMBER_5`、`MEMBER_8`、`MEMBER_92`，`payuni.go:205`）時兩個清單都是空的——錯誤目錄的那一組加旗標 `wholeForm`，`ProblemOf` 給 admin 的 `fields` 不變（admin 讀它，`kyc-application.ts:1009`，歸 5b）。**client 不再有「值 → 動作」表**：「前往補件」只看 `flow.can_resubmit`（R3 的草稿也是真）；「前往店家設定」只看 `fix_store_fields` 有認得的值；欄位清單在清單非空時顯示；審核意見在 `reason` 非空時顯示。client 只留樣式：REVIEWING 不是警示，其他是警示（APPROVED 不顯示面板）。`walletShowsTransactions`／`walletCanWithdraw` 改成「已認證而且 `flow.payuni.state == APPROVED`」。不認得的 `state`（含 UNSPECIFIED）照 API 規範 :102 走「未知」：標題 `common.unknown`、顯示 `explanation`（有的話）；入口照常讀 `can_resubmit` 與清單（它們不依 state） | Will 2026-10-15「能不能做與原因由伺服器推導，client 只呈現」（lb :5）；Stripe `requirements.currently_due`／`errors[]` 只列真的要補的項目［外部］；API 規範 :102 |
| F15 | 拿掉目錄文字之後的 client API | `kycFlowBanner`（`WS/lib/seller/kyc-flow.ts:177-187`）回 `CertStatusBanner \| null`、`kycFlowGate`（`:190-207`）的 `copy` 可以是 null；`kycFlowCopy`（`SA/features/seller/data/kyc_flow.dart:62-77`）的 `bannerTitle`／`bannerDescription`／`gateHeading`／`gateMessage` 改成可空：沒有文字就是 null，不再退回「未知」。呼叫端在 PayUni 階段改用 `payuni`（§3.0）。測試以生成的 enum 值清單逐一檢查：每個值的橫幅與閘門不是「未知」，沒有文字的值必須正好是 F9 的六個 | 不靠呼叫端「不去呼叫」維持正確；L′ §2.2 的完整性由生成器保證目錄，client API 的型別要反映 `omit` |
| F16 | 「平台會協助重新送出」的機制（綁定重試用完 → 後台待辦） | 3d 的後台待辦是「營運此刻要處理的事」由各來源的 `review_<種類>(at)` 函式推導、`pub_review_items` 合併（`M/db/objects.sql:228-302`，P30）；用它，不用事件：狀態本身就是待辦的來源，處理完（重新綁定）待辦自然消失，不需要訂閱者或已讀狀態。**新成員 `seller.review_payuni_bind_retry(at)`**，種類 `PAYUNI_BIND_RETRY`：註冊 `bind_status = 'FAILED'` 而且 `bind_error_code = 'PAYUNI_UNAVAILABLE'`（重試用完，`sync.go:76-83`）、店家 `ACTIVE`、註冊的申請是店家目前的申請而且 `APPROVED`（賣家改回草稿時不列，F17）。對象沿用 `kyc_application`（`store_id`、`application_id`），admin 的 `itemHref` 已經把它連到 `/admin/merchants/{storeId}#kyc`（`AQ/queue-item.ts:69-70`），不必改。開始時間是新欄位 **`payuni_registrations.bind_failed_at`**（`MarkPayUniBindFailed` 寫入，`RebindPayUni`、`UpsertPayUniIntent` 清空；CHECK `(bind_status = 'FAILED') = (bind_failed_at IS NOT NULL)`）。必要條件：待辦的開始時間要是失敗的時刻；表裡沒有這個時間（`schema.sql:1370-1394`），`updated_at` 在 FAILED 期間會被 `BumpPayUniIntent`（店家改地址，`payuni.sql:54`）推後，用它會讓待辦的期限往後跑。期限：開始後 **24 小時**（上線前的起始值，同 3d 的時長慣例，上線後依 rb R3 的觸發校準；**數字要 Will 確認**）。其他綁定失敗（FIX_VIA_SUPPORT、CONTACT_SUPPORT）賣家被告知找客服，由客服接手，不進待辦。**擁有者是 5a**：契約 `ReviewQueueItemKind` 加 `PAYUNI_BIND_RETRY = 12`、`RQ/internal/model/model.go` 的 `Kinds`、reviewqueue handler 的 `kindPermissions`（對到 `adminperm.SellerPayUniRegistrationsGet`：待辦只列呼叫者對來源有讀取權的種類，`adminperm.go:63`）與 subject 組成（沿用 `KYCApplication` 那一支）、admin `QUEUE_KIND_TEXT`（`Record` 對 enum 是窮舉的，生成後少一個 key `tsc` 失敗，所以同一個 commit 一定要加，§5 #37）。`PayUniBindFailed` 事件保留（發出端不變），仍沒有訂閱者；本刀不新增訂閱 | 3d 的「規則只有一個家」（成員條件只寫在函式，資料集測試鎖 Go 定義）；necessity：`bind_failed_at` 的條件見左欄 |
| F17 | 綁定失敗指出**申請欄位**時（`FIX_VIA_SUPPORT` 的「一個值一種動作」） | **改成 FIXABLE：賣家自己修正申請後重新送出**。綁定失敗時 PayUni 沒有收下這家店（`IdentityLockedBy` 在 FAILED 為假，`payuni.go:450-452`），身分欄位本來就解鎖；重新核准時 FAILED 的註冊回到 PENDING 重新綁定（`payuni.sql:25-41`），所以「改申請 → 送出 → 平台審核 → 自動重新綁定」整條路今天就通，只差 `CanResubmit`。`DeriveKYCFlow` 的 BIND_FAILED 分支改成 `CanResubmit = 推導結果是 FIXABLE`（F2 ⑥），`payuni.Editable` 改回 `flow.CanResubmit`，KYC ADR :81 文末修訂一起寫。FIX_VIA_SUPPORT 從此只剩「綁定失敗而且只指出店家欄位」：改店家設定，再找客服讓平台重新綁定，一種動作。替代：(i) 申請欄位 → CONTACT_SUPPORT：客服也改不了賣家的申請（後台的核准與退回只對 SUBMITTED，`kyc/admin.go:396`），是死路；(ii) 照舊延後：觸發是上線後的事件。**新增的保護**：賣家把綁定失敗的申請改回草稿之後，管理員的 `RebindPayUni` 若照舊放行，綁定 job 會以 `reg.application_id` 讀到還沒審核的草稿送給 PayUni（`sync.go:230` 的 `BindingData` 不看申請狀態，P29）。所以 `Rebind` 在註冊的申請不是已核准時回 `FailedPrecondition`（§5 #38）；5b 的 `RebindUnavailableReasons` 吸收成一個原因（§7.1） | 「每個值對賣家只有一種動作」（F1）；Stripe 的 `requirements` 由帳戶持有人自己補［外部］ |

---

## 0.4 查證過的現況（`a6f7b3815`）

| # | 事實 | 證據 |
|---|---|---|
| P1 | 兩個 client 的狀態分支逐一相同：`needs_reonboarding` → 終止、`payment_enabled` → 核准、依 `problem.remedy` 與 `outcome` 分支，加上呼叫端傳的 `canResubmit` | [量測] `WS/lib/seller/payuni.ts:62-85`；`SA/features/seller/data/payuni.dart:39-64` |
| P2 | `DeriveKYCFlow` 對 DRAFT 一律 `CanResubmit: true`；「`canResubmit` 或草稿」出自 `213445c8b`（10b 第 3 刀審查修正），沒有設計書 | [量測] `M/internal/model/payuni.go:473-474`；`CertificationsClient.tsx:358-365` |
| P3 | 從 PayUni 拒絕可直接送出；送出後申請是 SUBMITTED、`CanResubmit` 為假，註冊仍是拒絕；現行認證頁鎖住畫面、儀表板、錢包都會顯示「無法自行重新送出」 | [量測] `kyc.go:381-395`、`payuni.go:475-476`；畫面 [推論] |
| P4 | 綁定的暫時性錯誤在 job 內重試；最後一次仍失敗時記成 FAILED＋`PAYUNI_UNAVAILABLE`，之後 claim 跳過 FAILED（`BumpPayUniIntent` 也不會讓它重跑） | [量測] `M/internal/payuni/sync.go:76-83`、`:98`；`model/payuni.go:212` |
| P5 | outcome REJECTED 且非終止只發生在 MerStatus 1／9 而 MemberStatus 8、92、99；`MEMBER_99` 的補救方式是 CONTACT_SUPPORT；MemberStatus 99 是「會員已關閉或停權」 | [量測] `model/payuni.go:205-206`、`:500-503`；CodeMeta `業務狀態碼.md:295` |
| P6 | 同一狀態兩套字：website 認證頁鎖住畫面先用 PayUni 文案再用流程橫幅；seller-app 鎖住畫面只用流程橫幅 | [量測] `CertificationsClient.tsx:1041-1046`；`seller_certification_upload_screen.dart:843-898` |
| P7 | seller-app 認證頁沒有 PayUni 問題框；`_RejectionBanner` 在 REJECTED 或 PAYUNI_REJECTED 時都顯示 | [量測] `seller_certification_upload_screen.dart:668-690` |
| P8 | 去掉 `payuni.ts`、`payuni.dart` 與 `CertificationsClient.tsx:361` 之後，seller client 的正式程式不讀 `needs_reonboarding`、`payment_enabled`、`outcome`、`bind_status`、`remedy`、`problem` | [量測] `rg -n "\b(bindStatus\|bindErrorCode\|paymentEnabled\|needsReonboarding\|syncPending\|syncErrorCode\|bindTime\|statusTime\|checkTime\|storeFields\|hasProblem\|\.problem\b)" website/lib website/features website/components website/app seller-app/lib`（排除生成碼、測試、`openapi-types.ts`、codemeta）：PayUni 的命中只在 `payuni.ts:67-111`、`payuni.dart:41-90`，其餘是物流的同名欄位 |
| P9 | admin 不讀 `PayUniProblem.remedy`；`PayUniProblem` 是 admin 與賣家共用的 message；admin 讀 `problem.reason` 與 `problem.fields` | [量測] `rg -n "\.remedy\|PayUniRemedy" admin/src`（排除 gen）沒有結果；admin `seller.proto:698`；`kyc-application.ts:997-1012` |
| P10 | 伺服器沒有對應 kind 的東西；目錄沒有 PayUni enum；模組 `text/` 沒有 PayUni key | 盤點 §4；[量測] 目錄的 key 以 python 列出 |
| P11 | `KYCFlowStatus` 有 5 組變體（`action`、`banner_title`、`banner_description`、`gate_heading`、`gate_message`），另有 staff 名詞 | [量測] 目錄 `variants` 段 |
| P12 | 先例：`OpeningBlocker.Text(l)`／`Err(l)`（`model.go:117-147`）；`rpc.LocalizedMessage(m uitext.Message)`（`backend/internal/httpapi/rpc/errors.go:34`）；`uitext.Message.Locale` 是實際使用的語系，en 缺字時是 zh-TW | [量測] `backend/internal/platform/uitext/uitext.go:62-69` |
| P14 | website 的欄位清單依表單控制項去重（`WS/lib/seller/kyc.ts:546-562`）；seller-app 的銀行名稱與代碼是兩個控制項 | [量測] `SA/features/seller/data/kyc_fields.dart:70-74` |
| P15 | `GetMyPayUni` 在沒有註冊時回 not_found；核准在同一個 transaction 寫綁定意圖；`DeriveKYCFlow` 在 `reg == nil` 時只回 BINDING | [量測] `seller.proto:113-118`、`kyc/admin.go:441-450`、`payuni.go:481-483` |
| P16 | `KYCFlow` 也是 admin `KYCApplication.flow` 的型別，`H/admin_kyc.go:380` 與賣家端共用 `toFlow`（`H/kyc.go:331-333`）；admin 不讀 `can_resubmit` | [量測] admin `seller.proto:520`；`rg -n canResubmit admin/src` 沒有結果 |
| P17 | gateway 沒有賣家 PayUni 狀態的路由 | [量測] `rg -n -i payuni backend/gateway/internal/router/*.go` 只有 webhook 與付款 |
| P18 | 正式程式畫不出 kind `none` 的文案 | [量測] `payuni.ts:206-214`、`WalletClient.tsx:146-152`、`CertificationsClient.tsx:1453-1462` |
| P19 | 已綁定的註冊在重新核准時維持 BOUND 與舊的 PayUni 結果；沒綁定過的（PENDING 或 FAILED）回到 PENDING 重新綁定 | [量測] `M/db/queries/payuni.sql:25-41`；seller `CARRYOVER.md:256` |
| P20 | 取消只允許草稿與送審中，而且註冊已有 MerID（綁定不是 FAILED）時被擋 | [量測] `kyc.go:475-488` |
| P21 | `lastRejection` 只在最後一筆審核紀錄（略過 REBIND）是 REJECT 時有值，所以 PAYUNI_REJECTED 時沒有 | [量測] `M/internal/kyc/issues.go:109-120`、`convert.go:75-81` |
| P22 | 錢包在不顯示 PayUni 面板時顯示認證閘門（website `SellerCertGate`、seller-app `_CertificationGate`） | [量測] `WalletClient.tsx:92-100`；`wallet_screen.dart:120-128` |
| P23 | `GetMyKYC` 宣告 `CapAccountSelfRead`、`GetMyPayUni` 宣告 `CapOrderFulfill`；兩個能力的條件都只有 `RequirementActive` | [量測] `H/declaration.go:68,70`；`M/internal/model/capability.go:69-70` |
| P24 | 賣家端 `PayUniRegistration` 的聯絡資料欄位已經移除（`reserved 9-14`：`contact_name`、`contact_mobile`、`contact_tel`、`mer_email`、`mer_tel`、`etag`）；RPC 註解寫明聯絡資料是店家欄位 | [量測] `seller.proto:948-951`、`:114-115` |
| P25 | 三個畫面都同時讀流程與 PayUni：錢包 `useCertGate()`＋`usePayuniStatus()`；儀表板 `certGate.canResubmit`＋`usePayuniStatus()`；認證頁 `flow`＋`usePayuniStatus` | [量測] `WalletClient.tsx:57-77`；`DashboardPage.tsx:77-80`；`CertificationsClient.tsx:353-366` |
| P26 | `SaveKYCDraft` 對已核准而可以重送的申請一律改回 DRAFT（`UpdateKYCDraft` 的 `SET status = 'DRAFT'`），註冊不變；website 現行在 DRAFT 仍顯示 PayUni 問題（「`canResubmit` 或草稿」，P2） | [量測] `M/db/queries/kyc.sql:43-48`；`CertificationsClient.tsx:358-366` |
| P27 | 身分鎖：`IdentityLockedBy` ＝ `BindStatus != FAILED`；鎖住的申請欄位清單只在 `kyc/diff.go`；`ProblemOf` 只依賣家型態過濾欄位，不依鎖；「整份表單」的碼帶 `allFields`＋`allStoreFields` | [量測] `payuni.go:447-452`、`:525-548`、`:205`；`M/internal/kyc/diff.go:23-26` |
| P28 | `editable` 與 `Submit` 放行的狀態相同（DRAFT、REJECTED、已核准而 `payuni.Editable`）；SUBMITTED 依身分鎖回兩句，CANCELLED 一句，已核准不能改回「申請已核准，不能修改」／「…不能重新送出」 | [量測] `kyc.go:228-256`、`:381-395` |
| P29 | 後台的核准與退回只對 SUBMITTED；綁定 job 以 `reg.application_id` 讀申請資料，不檢查申請狀態；管理員 `RebindPayUni` 只看註冊是 FAILED | [量測] `M/internal/kyc/admin.go:396`；`sync.go:230`；`payuni.sql:45-52` |
| P30 | `PayUniBindFailed` 沒有訂閱者；後台待辦是 `pub_review_items` 的 11 個成員，種類在契約、Go 與 admin 各一份（admin 的 `QUEUE_KIND_TEXT` 是對 enum 窮舉的 `Record`）；`kycApplication` 的 subject 連到店家頁；註冊表沒有失敗時間，`BumpPayUniIntent` 會改 `updated_at` | [量測] `model/payuni.go:626-628`；`rg -n PayUniBindFailed backend` 只有發出端與 relay 註解；`M/db/objects.sql:278-302`；`review_queue.proto:25-47`；`RQ/internal/model/model.go:14-33`；`backend/internal/httpapi/reviewqueue/review_queue.go:34-46,220-224`；`AQ/queue-item.ts:10-25,69-70`；`schema.sql:1370-1394`；`payuni.sql:54-59,80` |
| P31 | 閘門文字的消費端：website `kycFlowGate` → `SellerCertGate`（錢包、物流）；seller-app `kycFlowCopy.gateHeading`／`gateMessage` → 錢包 `_CertificationGate`、`shippingCertificationGateCopy`（儀表板、個人頁）。PAYUNI_REJECTED 的 `gate_message` 是「認證未通過，請修改資料後重新送審。」 | [量測] `WS/lib/seller/kyc-flow.ts:190-207`；`WalletClient.tsx:99`；`ShippingClient.tsx:138`；`SA/features/seller/data/kyc_flow.dart:73-75`；`wallet_screen.dart:298-301`；`seller_certification_gate.dart:110-121`；目錄以 python 列出 |
| P32 | 賣家端 `bind_error_code`、`sync_pending`、`sync_error_code`、`bind_time`、`status_time`、`check_time` 沒有讀者 | [量測] P8 的指令對這六個欄位沒有 PayUni 的命中；`rg -n "GetSyncPending\|GetSyncErrorCode\|GetBindErrorCode\|GetBindTime\|GetStatusTime\|GetCheckTime" backend --glob '*.go'`（排除生成碼）在 seller 只命中 `H/kyc_test.go:503-504`、`kyc_integration_test.go:391,410`（測試）與 admin 的測試（admin 的 message，不受影響）；e2e 只有 `WS/tests/e2e/helpers/mock-seller-payuni.ts` 的 mock |

## 0.5 約束帳（本刀新出現的）

| 約束 | 判定 | 來源 |
|---|---|---|
| 伺服器對資源此刻狀態的判斷與說明由伺服器給，與 `failed_precondition` 同一個函式；只由 enum 值決定的名詞進目錄 | **REAL**（Will 2026-10-17） | lb :3、:7；計畫 :380 |
| 不能操作的原因是 `{reason, LocalizedMessage}`，句子與錯誤同一個函式 | **REAL**（Will 2026-10-21 對本刀的決定；lb D-L2） | lb D-L2 |
| 伺服器句子放模組 `text/`、ICU 受限子集、禁止拼接；目錄以（受眾、語系）完整 | **REAL**（已核准的 ADR） | i18n ADR D2、D3；L′ §1.4、§2.2 |
| UGC 與第三方原文不翻、容器標語系 | **REAL**（已核准的 ADR） | i18n ADR :116 |
| 英文要等介面沒有未翻譯的文字才發佈 | **REAL**（Will 2026-10-17） | i18n ADR :55 |
| 非終止的 PayUni 拒絕可以回草稿重送；綁定失敗不能 | **REAL**（已核准的 ADR）；F5 (a) 收緊、F17 放寬，KYC ADR 文末修訂 | KYC ADR :81 |
| client 不能手寫在目錄的 enum 的文字表 | **REAL**（CI 守門） | L′ §5.2；`ui-text-guard.yml` |
| PayUni 在 `update_merchant` 之後狀態怎麼變不知道 | **REAL**（外部行為） | seller `CARRYOVER.md:256` |
| `update_merchant` 改不了身分欄位，有 MerID 之後身分欄位鎖住 | **REAL**（PayUni 的 API） | `intent.go:119-124`；PayUni 代理商專用 API 總覽 §8 |
| 「client 依 code 與 remedy 呈現文案」 | SELF-IMPOSED，被 lb 取代（F13） | 10c-D1 第 1 點；`seller.proto:796-802` |
| `KYCFlow` 只放流程事實、PayUni 細節只在 `GetMyPayUni` | SELF-IMPOSED，**依據已不成立**（P25），F1 改掉，`CARRYOVER.md:334` 改寫 | seller `CARRYOVER.md:334` |
| 後台待辦的成員條件只寫在 `review_<種類>` 函式 | SELF-IMPOSED（3d 的設計），從零也這樣做（規則一個家），F16 照它 | 3d §2.1；`objects.sql:270-275` |
| website banner 與 panel 兩組審核中文字；說明句裡嵌原因與欄位 | SELF-IMPOSED（10b 的畫面選擇） | `payuni.ts:142-198` |
| 欄位清單依各端表單控制項去重 | SELF-IMPOSED（client 的版面知識） | P14 |
| 目錄 `KYCFlowStatus` 橫幅與閘門的已核准文字 | SELF-IMPOSED（文案可改，改要 Will 核可） | L0 D.1；L′ §1.5 |
| 與現有程式一致、改動量 | SELF-IMPOSED | root `AGENTS.md`「沿用也是決策」 |

---

## 1. 契約（F1、F8、F11、F13、F16）

```proto
// PayUniState 是賣家看到的 PayUni 收款狀態，一個值對應賣家的一種動作。伺服器由申請與 PayUni 註冊推導。
enum PayUniState {
  PAY_UNI_STATE_UNSPECIFIED = 0;
  // 綁定中或 PayUni 審核中，賣家等結果。
  PAY_UNI_STATE_REVIEWING = 1;
  // PayUni 要求補件，賣家依 PayUni 的通知信處理。
  PAY_UNI_STATE_PENDING_INFO = 2;
  // 申請的資料有問題，賣家修正申請後自己重新送出。
  PAY_UNI_STATE_FIXABLE = 3;
  // 店家設定的資料有問題，賣家修正後由平台重新處理，不能自己重新送出。
  PAY_UNI_STATE_FIX_VIA_SUPPORT = 4;
  // 沒有賣家能修正的資料，賣家要聯絡客服，不能自己重新送出。
  PAY_UNI_STATE_CONTACT_SUPPORT = 5;
  // PayUni 暫時無法處理，等平台重新送出。
  PAY_UNI_STATE_RETRY_LATER = 6;
  // PayUni 端的商店已是終止狀態，同一個店家無法再綁定。
  PAY_UNI_STATE_TERMINATED = 7;
  // 可以收款。
  PAY_UNI_STATE_APPROVED = 8;
}

// SellerPayUni 是賣家看到的 PayUni 狀態與要處理的事。
message SellerPayUni {
  PayUniState state = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 給賣家看的狀態說明（locale 是實際用的語系），只由 state 決定；state 是 APPROVED 時不設。
  google.rpc.LocalizedMessage explanation = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  // PayUni 針對這家店寫的審核意見原文（去掉前後空白；沒有時是空字串）。第三方原文、不翻譯，client 另外顯示並標 zh-TW。
  string reason = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 賣家此刻能在申請上修正的欄位（已依賣家型態與身分鎖過濾）；沒有具體欄位時是空的，client 不顯示清單。
  repeated KYCField fix_fields = 4 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 賣家此刻能在店家設定（UpdateMyStore）修正的欄位；沒有時是空的。
  repeated StoreField fix_store_fields = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
}

enum KYCResubmitUnavailableReason {
  KYC_RESUBMIT_UNAVAILABLE_REASON_UNSPECIFIED = 0;
  // 申請送審中，可以先取消再修改。
  KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW_CANCELABLE = 1;
  // 申請送審中，PayUni 已收下店家，不能取消。
  KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW = 2;
  KYC_RESUBMIT_UNAVAILABLE_REASON_CANCELLED = 3;
  // 以下對應 PayUniState 裡不能重送的值。
  KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_REVIEWING = 4;
  KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_PENDING_INFO = 5;
  KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_FIX_VIA_SUPPORT = 6;
  KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_CONTACT_SUPPORT = 7;
  KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_RETRY_LATER = 8;
  KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_TERMINATED = 9;
  KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_APPROVED = 10;
}

message KYCResubmitUnavailability {
  // 原因。伺服器一律設，不會是 UNSPECIFIED。
  KYCResubmitUnavailableReason reason = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 給使用者看的完整一句話（locale 是實際用的語系），有主詞、不含 client 的畫面或選單名稱；client 直接顯示。
  google.rpc.LocalizedMessage message = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// KYCFlow 是申請的流程狀態與衍生旗標：KYC 與 PayUni 兩條軸合成的結果。
message KYCFlow {
  KYCFlowStatus status = 1;
  // 此刻 SaveKYCDraft 與 SubmitKYC 會被接受（草稿、被退回、PayUni 拒絕或綁定失敗而且賣家能自己修正）。
  bool can_resubmit = 2;
  bool identity_locked = 3;
  // can_resubmit 為假時的原因，依先後，至少一個；為真時是空的。SaveKYCDraft、SubmitKYC 不能做時回的
  // failed_precondition 是第一個的 message。只在賣家的回應設，admin 的 KYCApplication.flow 不設。
  repeated KYCResubmitUnavailability resubmit_unavailabilities = 4 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 申請在 PayUni 階段（已核准），或賣家正在修正 PayUni 拒絕、綁定失敗的那份申請（草稿）時才設。只在賣家的回應設，
  // admin 的 KYCApplication.flow 不設（後台的 PayUni 呈現在 PayUniRegistration）。
  SellerPayUni payuni = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
}
```

- 刪 `rpc GetMyPayUni`、`GetMyPayUniRequest`、`GetMyPayUniResponse`、賣家端 `message PayUniRegistration`（F11）；`SubmitKYC` 的註解「can_resubmit 為 false 的狀態回 failed_precondition」補「訊息是 `KYCFlow.resubmit_unavailabilities` 的第一句」，「PayUni 拒絕而且還能重新綁定的」改成「PayUni 拒絕或綁定失敗而且賣家能自己修正的」；`SaveKYCDraft` 註解補同一句。
- `PayUniProblem` 註解改成 admin 用（F13）；刪 `remedy = 2`（`reserved 2; reserved "remedy";`）與 `enum PayUniRemedy`。
- `review_queue.proto`：`// 重試用完的 PayUni 綁定失敗，等管理員重新綁定（subject 是 kyc_application）。` `REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY = 12;`（F16）。
- 值的註解只寫語意（codemeta ADR D6；lb D-L1）。
- 生成：backend `make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `../../api/generate.sh buf.gen.yaml`（PATH 與 Dart 暖機照 `AGENTS.local.md`）；目錄改動後 `make -C backend uitext`。不得手改生成檔。

## 2. 單體（seller 模組、reviewqueue、handler）

- `M/internal/model/payuni_state.go`（新）：`type PayUniState string`、8 個常數與 `PayUniStateValues()`；`type SellerPayUni struct{ State; Reason string; FixFields, FixStoreFields []string }`；`SellerPayUniOf(app KYCStatus, sameApplication bool, reg *PayUniRegistration) (SellerPayUni, bool)`（F2、F3、F14、F17）；`func (s PayUniState) Explanation(l uitext.Locale) (uitext.Message, bool)`（APPROVED 與不認得的值回 false）。
- `PayUniPhaseOf(reg *PayUniRegistration) KYCFlowStatus`：5b F11 的抽出；5a 先合併時由 5a 抽出（`DeriveKYCFlow` 改呼叫它，輸出不變，5b 的等價測試照寫）。
- `DeriveKYCFlow`（`payuni.go:470-522`）：PayUni 階段（APPROVED）的 `CanResubmit` 改成 `SellerPayUniOf(...).State == FIXABLE`（取代 `!reg.NeedsReonboarding`）；BIND_FAILED 分支同樣（F17）。`KYCFlow` 與 `editable` 的註解跟著改。
- 錯誤目錄（`payuni.go:205`）：「整份表單」那一組加 `wholeForm: true`，`ClassifyPayUniCode` 多回這個旗標；`ProblemOf` 給 admin 的 `Fields`／`StoreFields` 不變，`SellerPayUniOf` 在 `wholeForm` 時兩個清單給空（F14）。`ProblemOf`：`Reason` 去掉前後空白，空字串時是 nil。
- `model.LockedIdentityFields()`：從 `M/internal/kyc/diff.go:23-26` 搬來，`draftDiff.identity()` 與 `SellerPayUniOf` 共用。
- `M/internal/model/kyc_resubmit.go`（新）：`type KYCResubmitUnavailableReason string`、10 個常數與 `...Values()`；`ResubmitUnavailableReasons(status KYCStatus, flow KYCFlow, payuni *SellerPayUni) []KYCResubmitUnavailableReason`（SUBMITTED 依 `flow.IdentityLocked` 回 `UNDER_REVIEW` 或 `UNDER_REVIEW_CANCELABLE`；CANCELLED；已核准而不能重送時依 `payuni.State` 對到 PayUni 的 7 個；能做時 nil）；`func (r ...) Text(l uitext.Locale) uitext.Message`；`func ErrResubmitUnavailable(reasons, l) error`（`FailedPrecondition` 帶第一句）。
- `M/internal/kyc/kyc.go`：`editable`（:228-256）與 `Submit`（:381-395）改成呼叫 `ResubmitUnavailableReasons`，不能做時回 `ErrResubmitUnavailable(reasons, uitext.LocaleFrom(ctx))`；`default` 分支回 internal。`payuni.Editable`（`intent.go:300-308`）改成回 `flow.CanResubmit`，名稱照實作時的呼叫端整理。
- 申請的讀取（`kyc.Service.Mine` 與 `SaveDraft`、`Submit`、`Cancel` 的回傳）：flow 之外帶 `SellerPayUni`（同一次讀到的註冊，`sameApplication` 以申請 ID 比對）與原因清單；handler 不推導。
- `M/internal/payuni/intent.go`：刪 `Mine`（:243-261）；`Rebind`（:159 起）在鎖住註冊後讀註冊的申請，不是 APPROVED 時回 `FailedPrecondition`（§5 #38，staff 句子放模組 `internal/text`，staff 受眾）。`M/payuni.go`：刪 `MyPayUniRegistration`（:151-154）。
- `M/internal/text/zh-TW.json`：7 則 `payuni.state.<state>`（§5 #6–#11）、10 則 `kyc.resubmit_unavailable.<reason>`（§5 #27–#36）、1 則 staff `payuni.rebind_unavailable.application_not_approved`（§5 #38）；`en.json` 不加（L′ §1.3：`-write` 不幫 en 補 key；`Message.Locale` 會是 zh-TW，en 在發佈門檻前不發佈，i18n ADR :55）。`make uitext` 產生 `text_gen.go`。
- `H/kyc.go`：`toFlow` 分成共用的 `toFlow`（status、can_resubmit、identity_locked；admin 用）與賣家的 `toSellerFlow(ctx, ...)`（再加 `payuni`、`resubmit_unavailabilities`，`rpc.LocalizedMessage(...)`、語系 `uitext.LocaleFrom(ctx)`）；`GetMyKYC`、`SaveKYCDraft`、`SubmitKYC`、`CancelKYC` 的 `toKYC` 走賣家版。刪 `H/payuni.go`、`declaration.go:70`。admin 的 `toAdminPayUni`（`H/admin_kyc.go:237-247`）只因 `remedy` 刪除而改 `toProblem`。
- 資料庫：`M/db/schema.sql` 的 `payuni_registrations` 加 `bind_failed_at TIMESTAMPTZ` 與 CHECK `(bind_status = 'FAILED') = (bind_failed_at IS NOT NULL)`，`atlas migrate diff` 產生 migration（既有 FAILED 列在同一份 migration 以 `updated_at` 回填，再加 CHECK；上線前沒有正式資料，回填只為讓 CHECK 成立）；`payuni.sql` 的 `MarkPayUniBindFailed` 寫入、`RebindPayUni` 與 `UpsertPayUniIntent` 清空。`M/db/objects.sql` 加 `seller.review_payuni_bind_retry(at)`（單一 SELECT、`BEGIN ATOMIC`，條件與時長只寫在這裡，F16）並加進 `pub_review_items` 的 UNION ALL；`atlas migrate new` 手寫 migration（drop 再建 `pub_review_items`）、`atlas migrate hash`；`make sqlc`。註解「11 種」改成 12 種。
- reviewqueue：`RQ/internal/model/model.go` 加 `PayUniBindRetry Kind = "PAYUNI_BIND_RETRY"` 與 `Kinds`；`backend/internal/httpapi/reviewqueue/review_queue.go` 的 `kindPermissions` 加一列、subject 組成與 `KYCApplication` 共用一支。
- 文件同 commit：seller `AGENTS.md` 的 PayUni 段補「賣家看到的 PayUni 狀態在 `KYCFlow.payuni`，由 `SellerPayUniOf` 推導；沒有 `GetMyPayUni`」；`CARRYOVER.md:334` 的契約句改成：「`KYCFlow` 有流程的事實（`status`、`can_resubmit`、`identity_locked`），加上賣家受眾的 `payuni`（PayUni 狀態是流程在 PayUni 階段的細分）與 `resubmit_unavailabilities`；沒有 `GetMyPayUni`。原依據『沒有畫面需要在同一個呼叫拿到兩者』不成立：錢包、儀表板、認證頁都同時讀兩者，client 只能合併兩份回應、處理快取時差與交叉失效（5a 開工前 design-review，Will 2026-10-21）」；KYC ADR :81 文末修訂（F5 (a)、F17、`can_resubmit` 的原因清單）。

## 3. clients

### 3.0 可見條件（兩端相同）

「有 PayUni 狀態」＝ `flow.payuni` 有值而且 `state` 不是 APPROVED。只看伺服器一份回應：何時有值由 F2、F3 決定，client 不另判流程狀態，也沒有跨 query 的失效（存檔、送出、取消的回應就是新的申請，現行已寫進同一個快取，`useSetMyKyc`）。

| 畫面 | 條件 | 顯示 |
|---|---|---|
| 錢包 | 讀取中 | 現有的佔位 |
| 錢包 | 有 PayUni 狀態 | PayUni 面板：標題、說明、審核意見（`reason` 非空，標 zh-TW）、欄位清單（清單非空）、入口（F14） |
| 錢包 | `payuni.state == APPROVED` | 沒有面板；已認證時顯示交易與提領 |
| 錢包 | 其他（沒有 `payuni`） | 認證閘門（P22），依流程狀態 |
| 儀表板（website） | 有 PayUni 狀態 | 提示條：標題、說明；按鈕照現行 |
| 儀表板（website） | 其他 | 沒有提示條 |
| 認證閘門（錢包、物流設定；兩端） | 有 PayUni 狀態 | 標題是 PayUni 標題、內容是 `explanation`；按鈕照現行（F9） |
| 認證閘門 | 其他 | 目錄 `gate_*` 照現行 |
| 認證頁鎖住畫面（不能編輯或剛送出） | 未認證而且有 PayUni 狀態 | PayUni 標題、說明、審核意見、欄位清單 |
| 認證頁鎖住畫面 | 其他（含已認證、SUBMITTED） | 流程橫幅（`kycFlowBanner`／`kycFlowCopy`；PayUni 階段沒有橫幅，F9） |
| 認證頁表單（可編輯） | 有平台的退回（`lastRejection`） | 退回框（平台的原因與問題），兩端都只看 `lastRejection` |
| 認證頁表單（可編輯） | 有 PayUni 狀態（只會是 FIXABLE，含 R3 的草稿） | PayUni 問題框：標題、說明、審核意見、欄位清單 |
| 存檔、送出失敗（`failed_precondition`） | — | 顯示錯誤訊息（即 `resubmit_unavailabilities` 的第一句），照各頁現有的錯誤呈現 |

兩個表單框不會同時出現：`lastRejection` 只在最後的審核是退回時有值，那時申請是 REJECTED，`payuni` 不設（P21、F3）［推論］。

### 3.1 website

- `WS/lib/seller/payuni.ts`：只留 `payuniProblemPointsAtStore` 改名為 `payuniFixPointsAtStore(payuni)`（`fix_store_fields` 有認得的值）；刪 `fetchMyPayUni` 與其餘推導。新增 `payuniStateTitle(ut, state)`（目錄名詞，不認得 → `common.unknown`）、`payuniStateAlert(state)`、`payuniHasState(payuni)`（§3.0）、`payuniFixFieldLabels(ut, payuni)`（目錄名詞，照 `fieldsByControl` 去重）。刪 `lib/hooks/usePayuniStatus.ts`、`queryKeys.seller.payuni`。
- 共用呈現元件 `PayuniStatusBody`（標題、說明、審核意見 `<p lang="zh-TW">`、欄位清單 `<ul>`），錢包、儀表板、認證頁三處共用。
- `features/seller-center/wallet/WalletClient.tsx`：讀 `useMyKycFlow()` 的 `payuni`；面板照 §3.0；「前往補件」看 `flow.canResubmit`、「前往店家設定」看 `payuniFixPointsAtStore`。
- `features/seller-center/dashboard/DashboardPage.tsx:77-80,190-194`：提示條照 §3.0。
- `CertificationsClient.tsx`：刪 `usePayuniStatus`、`payuniRejected`、`payuniKind`、`payuniStatusCopy`；鎖住畫面與表單框讀 `flow.payuni`。
- `lib/seller/kyc-flow.ts`：`kycFlowBanner` 回 `CertStatusBanner | null`、`kycFlowGate` 收 `flow`（不再只收 status），有 PayUni 狀態時回 PayUni 的標題與說明，否則目錄文字（可以是 null，F15）；`SellerCertGate`（`features/seller-center/components/SellerCertGate.tsx`）與兩個呼叫端（`WalletClient.tsx:99`、`ShippingClient.tsx:138`）改傳 flow；`lib/seller/cert-gate.ts` 的 `useCertGate` 多回 `flow`。
- `lib/seller/wallet-transactions.ts`：`isStoreCertified(seller) && flow?.payuni?.state === PayUniState.APPROVED`；`components/shared/Link.tsx:221-302` 的預取改讀 KYC 的 query（`queryKeys.seller.kyc`），不再預取 PayUni。
- e2e mock：刪 `tests/e2e/helpers/mock-seller-payuni.ts`，KYC 的 mock 加 `flow.payuni`、`flow.resubmitUnavailabilities`（website-e2e-verify 的 wire-shape 規則）。

### 3.2 seller-app

- `SA/features/seller/data/payuni.dart`：刪 `PayuniKind`、`payuniKindOf`、`payuniProblemFieldLabels`、`payuniReason`、`payuniPanelCopy`、`accountPayuniProvider`／`currentPayuniProvider`（`payuni.dart:171-193`，連同 `payuni.g.dart` 以 build_runner 重生）；`walletCanWithdraw` 改成「已認證而且 `kyc.application.flow.payuni.state == APPROVED`」；新增 `payuniStateTitle(state, l)`、`payuniStateAlert(state)`、`payuniHasState(payuni)`、`payuniFixFieldLabels(payuni, l)`、`payuniFixPointsAtStore(payuni)`。共用元件 `PayuniStatusBody`（說明、審核意見 `Text(..., locale: zh_Hant_TW)`、欄位清單），錢包與認證頁共用。
- `SA/features/seller/data/current_kyc.dart`：`MyKyc` 加 `payuni`（取自 `application.flow`）。
- `SA/features/seller/data/seller_status_refresh.dart`：刪 `_invalidatePayuni` 與相關註解（PayUni 狀態隨 `GetMyKYC` 一起重讀）；推播 `PAYUNI_STATUS` 照舊觸發重讀店家與認證流程（`seller_status_push.dart`）。
- `SA/features/wallet/presentation/wallet_screen.dart:37-145,283-374`：照 §3.0；「前往補件」看 `canResubmit`；`_CertificationGate` 有 PayUni 狀態時用 PayUni 文字。
- `SA/features/seller/presentation/seller_certification_gate.dart`：`shippingCertificationGateCopy(kyc, l)` 有 PayUni 狀態時標題、內容用 PayUni 文字（F9）。
- `SA/features/seller/presentation/seller_certification_upload_screen.dart`：鎖住畫面（`_LockedStatusView`，:835-898）與表單上方的 PayUni 問題框照 §3.0（F10）；`_RejectionBanner`（:668-690）只看 `rejection != null`，類別註解改掉「或 PayUni 拒絕但還能重送的申請」。
- `SA/features/seller/data/kyc_flow.dart`：`KycFlowCopy` 的 `bannerTitle`／`bannerDescription`／`gateHeading` 改成 `String?`、`gateMessage` 回 `String?`（F15）。
- App 的文字都從目錄與伺服器來，本刀不新增 ARB。

### 3.3 admin

- `AQ/queue-item.ts`：`QUEUE_KIND_TEXT` 加 `[ReviewQueueItemKind.PAYUNI_BIND_RETRY]: "重新綁定 PayUni"`（§5 #37，admin 單端的 enum 文字表，同 3d §5 #104 的做法）；`itemHref` 的 `kycApplication` 分支已涵蓋，不改。
- 其餘不改（F12）。刪 `PayUniProblem.remedy` 後跑 admin 的 `tsc`、lint、test，確認沒有讀者（P9）。

## 4. gateway

沒有改動（P17）。

---

## 5. 文案表（A＝已核准、C＝跟隨契約或 SSOT 或修 bug、E＝沿用、**N＝v1.2 新寫、改寫或版面改動，要 Will 核可**）

已核准指 10b 的 website `payuni.ts`（10c-D1「已核准的版本」）、L0 的目錄文字，以及 Will 2026-10-21 核可的 v1.1 N 類 14 列（標「A（v1.1）」）。標題是目錄的名詞，說明與原因句是伺服器句子（seller 模組 `internal/text`，賣家受眾、zh-TW；en 不加 key，照 i18n ADR :55 在發佈門檻前不發佈）。

| # | 位置 | 改前 → 改後 | 類 |
|---|---|---|---|
| 1 | 標題 REVIEWING | 「PayUni 審核中」；錢包原「PayUni 建店中」→「PayUni 審核中」 | A（v1.1） |
| 2 | 標題 PENDING_INFO、FIXABLE、FIX_VIA_SUPPORT | 「PayUni 需要補件」 | A |
| 3 | 標題 CONTACT_SUPPORT／RETRY_LATER／TERMINATED | 「PayUni 申請未通過」／「PayUni 暫時無法處理」／「PayUni 收款帳戶已終止」 | A |
| 4 | 標題 APPROVED（目錄完整性，畫面不顯示） | 「PayUni 收款已啟用」 | A（v1.1） |
| 5 | （保留編號，v1 的 RESUBMITTED 標題已刪） | — | — |
| 6 | 說明 REVIEWING | 「平台審核已通過，PayUni 正在審查金流資料。」 | A（v1.1） |
| 7 | 說明 PENDING_INFO | 「請至信箱查看 PayUni 補件通知信件，依指示補齊資料後再次送出審核。」 | A |
| 8 | 說明 FIXABLE | v1.1「請依 PayUni 的審核意見修正資料後重新送出。」→「**請修正資料後重新送出，送出後平台會重新審核。**」（F17 之後 FIXABLE 也來自綁定失敗，那時沒有 PayUni 的審核意見，原句會指向不存在的內容） | **N** |
| 9 | 說明 FIX_VIA_SUPPORT／CONTACT_SUPPORT | 「修正後請聯繫客服重新處理，無法自行重新送出。」／「請聯繫客服了解原因，無法自行重新送出。」 | A |
| 10 | 說明 RETRY_LATER（F4） | 「PayUni 暫時無法處理這筆申請，平台會協助重新送出，你不需要重新送出。」（承諾由 F16 的待辦成立） | A（v1.1） |
| 11 | 說明 TERMINATED | 「請聯繫客服了解原因。」 | A |
| 12 | （保留編號） | — | — |
| 13 | 說明 APPROVED | v1.1 為錯誤路徑寫的「PayUni 收款已啟用，商家認證不需要再修改或重新送出。」→ 刪（F8 改用原因句 #36；APPROVED 沒有說明，F7） | C |
| 14 | PayUni 的審核意見小標題 | 「PayUni 的審核意見」；`reason` 空時整段不出現 | A（v1.1） |
| 15 | 要修正的欄位小標題 | 「請修正以下資料」；條件改成伺服器的清單非空（F14） | A（v1.1）＋C（條件） |
| 16 | 「前往補件」「前往店家設定」 | 字不變；「前往補件」看 `can_resubmit`，「前往店家設定」看 `fix_store_fields` 有認得的值（F14） | E（字）＋C（條件） |
| 17 | kind `none` 那一列 | 刪（正式程式畫不出來，P18） | C |
| 18 | 錢包：在 PayUni 階段而沒有註冊 | v1.1 的 BINDING 後備刪除：伺服器此時回 REVIEWING，顯示 #1、#6（字與 v1.1 核可的後備相同） | C |
| 19 | 錢包、儀表板：申請不在 PayUni 階段而有舊註冊 | 錢包顯示認證閘門、儀表板沒有提示條（F3） | C |
| 20 | 目錄 `seller.KYCFlowStatus.banner_title`、`banner_description` | v1.1 核可刪 5 個值（10 則）→ **刪 6 個值（12 則）**：加上 BINDING（「PayUni 審核中／平台審核已通過，PayUni 正在審查金流資料。」），因為 PayUni 階段一律有 `payuni`，不再需要後備（F9） | **N** |
| 21 | 兩端認證頁鎖住畫面，有 PayUni 狀態 | 顯示 #1–#15（F9） | A（v1.1） |
| 22 | seller-app 認證頁鎖住畫面 | 加上 PayUni 狀態（F10） | A（v1.1） |
| 23 | seller-app 認證表單上方 | 加 PayUni 問題框（F10） | A（v1.1） |
| 24 | seller-app `_RejectionBanner` 在 PAYUNI_REJECTED | 不顯示，改由 #23 說明 | A（v1.1） |
| 25 | v1.1 的「錯誤句＝狀態說明」 | 刪，改成 #27–#36 的原因句（F8） | C |
| 26 | 目錄 `seller.KYCFlowStatus.gate_heading`、`gate_message`（錢包與物流的認證閘門） | **刪 BINDING、BIND_FAILED、FORWARDED、UNDER_REVIEW、PENDING_INFO、PAYUNI_REJECTED 6 個值（12 則，`omit`）**，例：PAYUNI_REJECTED 的「商家認證需要補件／認證未通過，請修改資料後重新送審。」、BIND_FAILED 的「商家認證審核中／你的商家認證已通過平台審核，PayUni 金流審核中，通過後就能{subject}。」→ 閘門在這些狀態顯示 PayUni 標題（#1–#3）與說明（#6–#11），不再有 `{subject}`；按鈕不變（F9） | **N** |
| 27 | 原因 `UNDER_REVIEW_CANCELABLE` | 現行字面值「申請審核中，要修改請先取消申請」→「申請審核中，要修改請先取消申請。」 | E（補句號） |
| 28 | 原因 `UNDER_REVIEW` | 「申請審核中，請等待審核結果。」 | E |
| 29 | 原因 `PAYUNI_REVIEWING` | 「**PayUni 正在審查你的資料，審查期間不能修改或重新送出商家認證。**」 | **N** |
| 30 | 原因 `PAYUNI_PENDING_INFO` | 「**PayUni 要求補件，請依 PayUni 通知信的指示處理，商家認證不需要在這裡修改或重新送出。**」 | **N** |
| 31 | 原因 `PAYUNI_FIX_VIA_SUPPORT` | 「**要修正的是店家設定裡的資料，修正後請聯繫客服重新處理，商家認證不需要重新送出。**」 | **N** |
| 32 | 原因 `PAYUNI_CONTACT_SUPPORT` | 「**這個問題無法以修改資料解決，請聯繫客服了解原因。**」 | **N** |
| 33 | 原因 `PAYUNI_RETRY_LATER` | 「**PayUni 暫時無法處理，平台會協助重新送出，你不需要修改或重新送出。**」 | **N** |
| 34 | 原因 `PAYUNI_TERMINATED` | 「**PayUni 收款帳戶已終止，無法修改或重新送出，請聯繫客服。**」 | **N** |
| 35 | 原因 `PAYUNI_APPROVED` | 原「申請已核准，不能修改」／「申請已核准，不能重新送出」→「**商家認證已通過，不需要修改或重新送出。**」 | **N** |
| 36 | 原因 `CANCELLED` | 現行字面值「申請已取消，不能修改」→「申請已取消，不能修改。」 | E（補句號） |
| 37 | admin 後台待辦種類 `PAYUNI_BIND_RETRY`（F16，staff） | 「**重新綁定 PayUni**」 | **N** |
| 38 | admin `RebindPayUni` 在申請不是已核准時（F17，staff，`failed_precondition`） | 「**賣家正在修改商家認證，重新核准後會自動重新綁定。**」 | **N** |

### 待 Will 核可（v1.2 新增）——Will 2026-10-21 全部核可（12 列與 F16 期限 24 小時）

**12 列 N**：#8、#20、#26、#29、#30、#31、#32、#33、#34、#35、#37、#38（逐字見上表）。另有一個非文案的數字：F16 待辦的期限 24 小時（上線前的起始值）。

---

## 6. 測試

### 6.1 backend（`M/internal/model`、`H`、reviewqueue、模組 integration）

- `TestSellerPayUniOf`：表格驅動，覆蓋（申請狀態 DRAFT／SUBMITTED／REJECTED／APPROVED／CANCELLED）×（`sameApplication`）×（綁定 PENDING／BOUND／FAILED、`reg == nil`）×（outcome）×（碼：`MEMBER_8`、`MEMBER_99`、`MER_92`、`MER_91`、`PAYUNI_UNAVAILABLE`、`INCOMPLETE_DATA`（有、沒有缺的店家欄位）、`AGENT03001`（身分欄位）、`AGENT03005`（聯絡店家欄位）、不認得的碼）。必含的格：SUBMITTED＋PayUni 拒絕的註冊 → 沒有；DRAFT＋`sameApplication`＋PayUni 拒絕 `MEMBER_8` → FIXABLE（R3）；DRAFT＋不是同一份申請 → 沒有；APPROVED＋`reg == nil` → REVIEWING；APPROVED＋BOUND＋`MEMBER_8` → FIXABLE，`fix_fields`、`fix_store_fields` 都空（`wholeForm`）；APPROVED＋FAILED＋`AGENT03001` → FIXABLE，`fix_fields` 含身分欄位（FAILED 不鎖）；APPROVED＋FAILED＋`AGENT03005` → FIX_VIA_SUPPORT，只有 `fix_store_fields`；APPROVED＋FAILED＋`INCOMPLETE_DATA` 沒有缺的欄位 → CONTACT_SUPPORT；`PAYUNI_UNAVAILABLE` → RETRY_LATER；`MER_92` → TERMINATED；`MEMBER_99` → CONTACT_SUPPORT。
- 不變式（同一份資料集）：APPROVED 申請 ⇒ 一定有值；TERMINATED ⇔ `NeedsReonboarding`；APPROVED ⇔ `PaymentEnabled`；`flow.CanResubmit` ⇔（DRAFT、REJECTED，或已核准而 state 是 FIXABLE）；`fix_fields` 不含 `LockedIdentityFields()` 裡的欄位當 `IdentityLocked`；FIX_VIA_SUPPORT ⇒ `fix_fields` 空而 `fix_store_fields` 非空；`ResubmitUnavailableReasons` 為空 ⇔ `CanResubmit`。**突變**：拿掉身分鎖過濾、拿掉 `wholeForm`、把 BIND_FAILED 的 `CanResubmit` 寫回假、拿掉 F5 (a)，各自要有格失敗。
- `TestPayUniStateExplanation`：以 `PayUniStateValues()` 逐一檢查，除 APPROVED 外都有非空句子、`Locale` 是 zh-TW；APPROVED 回 false。
- `TestResubmitUnavailableText`：每個原因有非空、以「。」結尾的句子；`ErrResubmitUnavailable` 是 `FailedPrecondition` 而且訊息逐字等於第一個原因的 `Text`。
- `kyc` 的服務測試：已核准、PayUni 審核中時 `SaveDraft` 與 `Submit` 的錯誤訊息等於 #29；FAILED＋身分欄位的碼時 `SaveDraft` 放行、身分欄位可改，送出、核准後註冊回到 PENDING。
- `Rebind`：註冊 FAILED 而申請已改回 DRAFT 時回 `FailedPrecondition`（#38）；申請 APPROVED 時照舊。
- handler：`H/kyc_test.go:473` 的 enum 對照表刪 `PayUniRemedy`、加 `PayUniState`、`KYCResubmitUnavailableReason`；`GetMyKYC` 的 wire 測試斷言 `flow.payuni`（state、`explanation.locale`、`message`、`reason`、兩個清單）與 `resubmit_unavailabilities`；admin 的 `KYCApplication.flow` 兩者都不設（**突變**：admin 改用賣家版 `toSellerFlow` 時要失敗）。
- 後台待辦：`review_policy_integration_test.go` 的成員表（:36）加 `{"PAYUNI_BIND_RETRY", "review_payuni_bind_retry"}`，資料集加：FAILED＋`PAYUNI_UNAVAILABLE`＋申請 APPROVED＋店家 ACTIVE → 有一列，開始是 `bind_failed_at`、期限 +24 小時；其他碼、申請 DRAFT、店家關閉、PENDING／BOUND → 沒有；重新綁定後消失；店家改地址（`BumpPayUniIntent`）後開始時間不變。`:734` 的鍵對照加一列。`H/reviewqueue/review_queue_test.go` 的 `TestKindPermissionsCoverEveryKind` 與 subject 對照（:415）加新種類。
- 模組 integration：核准 → 綁定 → PayUni 拒絕（`MEMBER_8`）→ `GetMyKYC` 的 `payuni` 是 FIXABLE → `SaveKYCDraft` → 仍是 FIXABLE（R3）→ `SubmitKYC` → 沒有 `payuni`、流程是 SUBMITTED。重新核准之後是 FIXABLE（R1，已知缺口）。
- 既有的 `TestDeriveKYCFlow`（`M/internal/model/payuni_test.go:146`）：`MEMBER_99` 那格的 `CanResubmit` 改成假；FAILED＋身分欄位碼那格改成真。
- 目錄：生成器的完整性檢查涵蓋 8 個名詞、兩個小標題與兩組 `omit`（L′ §2.2）。

**會壞、要跟著改或刪的既有測試**［量測：`rg -n "GetRemedy|PayUniRemedy|GetPaymentEnabled|GetBindStatus|GetOutcome|GetSyncPending|GetMyPayUni" H M --glob '*_test.go'`；`rg -n "paymentEnabled|needsReonboarding|remedy|bindStatus|outcome|usePayuniStatus|getMyPayUni|PayuniProvider" website seller-app --glob '*test*'`］：

| 檔案 | 處理 |
|---|---|
| `H/kyc_test.go:473`、`:501-504` | 對照表改；賣家端 `toPayUni` 的測試刪 |
| `H/kyc_integration_test.go:391`、`:410-411` | 改成斷言 `GetMyKYC` 的 `flow.payuni` |
| `SA` 測試 `test/features/seller/data/seller_status_refresh_test.dart:423,447` | 刪 PayUni 失效的斷言 |
| `SA` 測試 `payuni_test.dart`、`wallet_screen_test.dart`、`wallet_top_surface_test.dart` | fixture 改成 `MyKyc` 帶 `flow.payuni` |
| `WS/lib/seller/payuni.unit.test.ts`、`wallet-transactions.unit.test.ts`、`WalletClient.unit.test.ts`、`CertificationsClient.unit.test.ts:614-618` | 同上 |
| `WS/tests/e2e/helpers/mock-seller-payuni.ts` | 刪，KYC mock 帶 `flow.payuni` |
| `WS/lib/seller/kyc-flow.unit.test.ts:37-43`、`SA` 測試 `kyc_flow_test.dart:136-141` | F15 改型別後改寫 |

`admin_kyc_test.go:448` 與 `admin_kyc_integration_test.go:528-549` 讀 admin 的 `PayUniRegistration`，不受影響。

### 6.2 website（只測呈現）

| 現行測試 | 處理 |
|---|---|
| `lib/seller/payuni.unit.test.ts` 的 `payuniKindOf`、`payuniCopy`、店家欄位與去重、`payuniBannerOf`、`fetchMyPayUni` | 刪；推導與句子由 §6.1 鎖。新增 `payuniHasState`（APPROVED 為假）、`payuniFixFieldLabels`（銀行名稱與代碼只列一個） |
| `lib/seller/wallet-transactions.unit.test.ts:22-48` | 改成依 `flow.payuni.state`；**突變**：FIXABLE 時要是假 |
| `WalletClient.unit.test.ts:129-224` | 改成 §3.0 錢包的每一列：標題是目錄名詞、說明是 mock 裡**獨特的伺服器句子**（逐字出現）；審核意見在 `lang="zh-TW"` 的元素裡；「前往補件」只跟 `canResubmit`（**突變**：mock 的 state 是 FIXABLE 而 `canResubmit` 為假時不出現）；「前往店家設定」只跟 `fixStoreFields`；沒有 `payuni` 時顯示閘門；不認得的 state → 「未知」加說明 |
| `CertificationsClient.unit.test.ts:830-860,914-935` | 改成 §3.0 認證頁的每一列：PAYUNI_REJECTED＋FIXABLE 有問題框；DRAFT＋`payuni` 仍有問題框（R3）；REJECTED＋`lastRejection` 只有退回框；SUBMITTED 的鎖住畫面是流程橫幅；存檔被拒時顯示伺服器句子 |
| `ShippingClient.unit.test.ts`（閘門） | 有 `payuni` 時閘門的標題與內容是 PayUni 標題與 mock 的說明，沒有 `{subject}`；沒有時照目錄 |
| `lib/seller/kyc-flow.unit.test.ts:37-43` | 以生成的 `KYCFlowStatus` 值清單逐一檢查：`kycFlowBanner`、`kycFlowGate` 不是 null 時文字不是「未知」；回 null 的值正好是 F9 的六個 |
| e2e `seller-dashboard-onboarding-actions.spec.ts:87,423` | mock 換成 KYC 的 `flow.payuni`；儀表板提示條的斷言改成伺服器句子 |
| 守門 | `PayUniState` 進目錄後，`check-enum-text` 擋 client 手寫它的表（L′ §5.2） |

### 6.3 seller-app（只測呈現）

| 現行測試 | 處理 |
|---|---|
| `test/features/seller/data/payuni_test.dart` 的 `payuniKindOf`、欄位與原因、`payuniPanelCopy` | 刪；新增 `payuniHasState`、`payuniFixFieldLabels`（銀行名稱與代碼兩個都列） |
| 同檔 `walletCanWithdraw` | 改成依 `flow.payuni.state`，同 website 的突變 |
| `wallet_screen_test.dart:745-843,873-900`（與 :559-560、:766） | 改成 §3.0 錢包的每一列，斷言同 website；閘門在有 `payuni` 時用 PayUni 文字 |
| `wallet_top_surface_test.dart:37,62`、`seller_status_refresh_test.dart:423,447` | fixture 改用 `flow.payuni`；刪 PayUni provider 的失效斷言 |
| `kyc_flow_test.dart:136-141` | 同 website 的 F15 檢查 |
| 認證閘門（`shippingCertificationGateCopy`） | 有 `payuni` 時用 PayUni 標題與說明 |
| 認證頁（新） | 鎖住畫面有 `payuni` 時顯示 PayUni 狀態、SUBMITTED 時是流程橫幅；可編輯＋FIXABLE 有問題框而**沒有** `_RejectionBanner`；DRAFT＋`payuni` 仍有問題框；REJECTED＋退回只有 `_RejectionBanner`；審核意見的 `Text` 帶 zh-TW locale |

### 6.4 admin

`tests/merchant-kyc-application.test.ts` 照跑（刪 `remedy` 後仍要通過）；`tests/review-queue-page.test.ts` 照跑；`QUEUE_KIND_TEXT` 是對 enum 窮舉的 `Record`，少了新種類 `tsc` 就失敗。

## 7. Commit 計畫

| # | 內容 | 審查 |
|---|---|---|
| 5a-1 | 契約（seller、reviewqueue）與三個生成端；模組 `PayUniPhaseOf`（5b 尚未合併時）、`SellerPayUniOf`、說明句、原因清單與句子、`editable`／`Submit`、`DeriveKYCFlow` 的 `CanResubmit`、`ProblemOf`、`LockedIdentityFields`、`Rebind` 的申請檢查、刪 `Mine`；`bind_failed_at` 的 schema、migration、queries；`review_payuni_bind_retry` 與 `pub_review_items` 的 migration、`make sqlc`；reviewqueue 的 `Kind` 與 handler；handler（賣家版 flow、刪 `H/payuni.go`）；目錄名詞、小標題、兩組 `omit` 與 `make uitext`；website（§3.1）；seller-app（§3.2）；admin `queue-item.ts`（§3.3）；§6 全部；seller `AGENTS.md`、`CARRYOVER.md:334`（改寫與理由，§2 末）；KYC ADR :81 文末修訂 | code-review；完成前的 design-review |
| 5a-2 | 文件：計畫 10e 的 5a 完成紀錄（含 F13 的取代紀錄、Will 2026-10-21 的兩個決定）、L′ §8 那一條 Pending 結案 | — |

契約與所有消費端在同一個 commit（root `AGENTS.md`「改契約時，同一次變更內更新唯一來源與所有消費端」）。design-review 兩次：開工前一次（已做，附錄 B）；5a-1 完成前一次。

### 7.1 與 5b 的合併順序與分工

兩刀都改 `M/internal/model/payuni.go`（5a 的 `SellerPayUniOf`、`CanResubmit`、`wholeForm`，5b 的 `PayUniPhaseOf`）、`M/internal/payuni/intent.go`（5a 的 `Rebind` 申請檢查與刪 `Mine`、`Editable`，5b 的 `Rebind` 改寫成 `RebindUnavailableReasons`）、`H/admin_kyc.go`（5a 因 `remedy` 刪除改 `toProblem`，5b 改 `toAdminPayUni`）與三個生成端；語意不衝突。**建議 5a 先合併**（Will 排定的順序）。

- **`PayUniPhaseOf`**：5a 先合併時由 5a 抽出（`DeriveKYCFlow` 輸出不變），5b 改成沿用、不重抽；5b 先合併時 5a 沿用。
- **後台待辦 `PAYUNI_BIND_RETRY`（F16）歸 5a**：契約、seller 的待辦函式、reviewqueue、admin `queue-item.ts` 都在 5a-1。5b 不碰 `AQ/` 與 `review_queue.proto`；5b 的 `can_rebind` 為真而且碼是 `PAYUNI_UNAVAILABLE` 時，店家頁的重新綁定就是處理這件待辦的動作，5b 不需要另外連結。待辦成員的條件（FAILED、店家 ACTIVE、申請 APPROVED）與 5b 的 `RebindUnavailableReasons` 為空（加上 F17 的新原因）一致，**後合併的那一刀**在資料集測試加一條「待辦有列 ⇒ `RebindUnavailableReasons` 為空」。
- **`Rebind` 的申請檢查（F17）**：5a 先合併時，5a 在現行 `Rebind` 加檢查（§5 #38 的 staff 句）；5b 改寫 `Rebind` 時把它收成 `StoreActionUnavailableReason` 的一個值（例 `KYC_RESUBMITTING`，排在 `PAYUNI_BIND_IN_PROGRESS` 之前），句子沿用 #38，不另寫。5b 先合併時，由 5a 直接加那個原因。
- 後合併的那一刀：rebase 後重新跑三個生成端與 `make -C backend uitext`、`make sqlc`，跑全部生成物 drift 與對方的測試；刪 `PayUniOutcome`（F11）。

## 8. 驗證命令（派工前照當時的 `.github/workflows/` 再核對；以下對照 `a6f7b3815` 的 workflow）

- api（`api-contract.yml`）：`buf lint api`、`buf breaking`（`v1alpha1` 不檢查，`api/buf.yaml:24-25`）、三個生成端的 drift、`packages/hawkr_api` 的 `dart analyze` 與 `dart format --set-exit-if-changed`。
- 生成物 drift：`backend-proto-generated-diff.yml`（`make connect` 後無 diff）、`ui-text-generated-diff.yml`（`make -C backend uitext` 後 `git diff --exit-code`）；`make sqlc` 後無 diff；`atlas migrate hash` 與 `atlas.sum` 一致。
- 單體（`backend-monolith.yml`）：gofmt、`go vet`（含 `-tags integration`）、`go test -race ./...`、`go test -race -timeout 15m -tags integration ./...`（`DOCKER_HOST`）、`make lint-monolith`、`GOWORK=off go mod tidy` 無 diff；`internal/arch` 的 `TestDesiredStateIsSplitBetweenSchemaAndObjects`。
- 守門：`ui-text-guard.yml`（`bash scripts/check-enum-text-cross.sh`）；website、admin 的 `lint` 末端的 `check-enum-text`。
- website（`website-ci.yml`）：`pnpm run lint`、`pnpm run design:check --apps-root ..`、`pnpm run test:unit`、`pnpm test`、`pnpm run build`（帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值的 `NEXT_PUBLIC_API_BASE_URL`）、`pnpm run test:build-output`；e2e 照 website-e2e-verify。
- seller-app（`seller-ci.yml` → `_flutter-quality.yml`）：build_runner 重生後無 diff、analyze、custom_lint、完整測試、`enum_text` step、`scripts/deadcode.sh`（`AGENTS.local.md` 的 commit 前清單）。
- admin（`admin-ci.yml`）：`pnpm --filter admin lint`、`test`、`build`（`queue-item.ts` 與 `remedy` 刪除）。
- 共用套件：`ts-packages.yml`、`dart-packages.yml`；`buyer-ci.yml`（`packages/hawkr_api/**` 觸發）。
- 每個 workflow 看 job 自己的結論（root `AGENTS.md`「驗證」）。

## 9. 沒有驗證的部分

- 本文沒有跑任何測試、build 或生成；行號以 `a6f7b3815` 為準。
- P3、P5「只有 `MEMBER_99`」、§3.0「兩個表單框不會同時出現」、F17「改申請 → 送出 → 核准 → 自動重新綁定整條路今天就通」是讀程式推出的，沒有跑；§6.1 的表格、不變式與 integration 測試會證實或推翻。
- F17「客服改不了賣家的申請」只查了後台的核准與退回只對 SUBMITTED（`kyc/admin.go:396`），沒有窮舉後台的所有寫入路徑。
- F16 的 `kindPermissions` 對到 `SellerPayUniRegistrationsGet`（`adminperm.go:63`）是比照其他種類「來源頁的讀取權」推的；現在只有 OWNER 一個角色，沒有可觀察的差別。
- `MEMBER_99` 的意思取自 CodeMeta 對 PayUni 文件的轉錄（2026-09-12 核對）。
- 重新核准之後 PayUni 重審的時間與狀態變化（R1）沒有 sandbox 不能驗證。
- [外部] 的業界做法（AIP-216、AIP-193、Stripe `requirements`）由審查者查過，本文沒有再查。
- seller-app `Text` 的 `locale` 參數對無障礙朗讀與字型選擇的實際效果沒有在裝置上看過。

---

## 附錄 A：審查意見的處理（v1 → v1.1；審查全文在 session scratchpad `5a-design-review.md`）

| # | 意見（嚴重度） | 處理 |
|---|---|---|
| D1 | `Explanation(l, p)` 的輸入選不出 #13、#14；句數 18 應為 19（HIGH） | 成立。改成說明句只由狀態決定（F7）：原因與欄位移出句子，key 只有 `payuni.state.<state>`；「能不能重送」這一維由 enum 承擔 |
| D2 | RESUBMITTED 把 KYC 軸的事實放進 PayUni 狀態，沒有評估替代（MED） | 成立。申請不在 PayUni 階段時 `state` 不設，畫面改看 `KYCFlowStatus`（F3、§3.0），刪 RESUBMITTED |
| D3 | PayUni 的審核意見併進本地化句子，違反 i18n ADR :116（MED） | 成立。`reason` 另外顯示、標 zh-TW（F7、§5 #14） |
| D4 | 與 5b 的界線互相推給對方（MED） | 成立。admin 的 PayUni 文字與可用性全部歸 5b，5a 只做賣家面（§0.2 第 1 點、F12） |
| D5 | F8「沒有同一個判斷的錯誤」不成立（MED） | 成立。v1.1 改成狀態說明當錯誤；v1.2 再改成原因句（附錄 B 第 3 項） |
| D6 | `FormatList` 必要條件不完整、en 是佔位、語系用錯（MED） | 成立。F7 把欄位清單移出句子後不再需要列表格式 |
| D7 | seller-app 認證頁會出現兩個狀態框（MED） | 成立。`_RejectionBanner` 只看 `rejection`（F10、§3.2、§5 #24） |
| D8 | 平台退回或取消時仍顯示舊的 PayUni 問題（MED） | 成立。F3 的「不在 PayUni 階段就沒有狀態」處理退回與取消 |
| D9 | #20 錢包改字被標成 C；同一情境兩句說明（LOW） | 成立。改標 N；REVIEWING 的說明改成與 BINDING 橫幅逐字相同 |
| D10 | 拿掉 5 個橫幅後回「未知」、測試仍通過（LOW） | 成立。F15：型別改成可以沒有橫幅；測試以生成的值清單逐一檢查 |
| D11 | commit 切分與 L′-2 先例不同、沒有理由（LOW） | 成立。契約與所有消費端一個 commit（§7） |
| D12 | 會壞的測試沒有列（LOW） | 成立。§6.1 末的表逐檔列出 |
| D13 | RESUBMITTED 時仍出現「前往店家設定」（LOW） | RESUBMITTED 已刪；v1.2 入口改成伺服器的清單非空（F14） |
| B 段 | P1 的 seller-app 行號 | 已改 |
| 5b | admin 後備表、`PayUniOutcome` 去留、階段判斷與合併順序 | F12、F11、F2、§7.1 |
| R1 | 再核准後註冊維持 BOUND 與舊拒絕，重新核准後仍是 FIXABLE | P19、§0.2、§6.1；觸發不變，Will 2026-10-21 同意延後 |
| R2 | `link-prefetch-certified.test.ts` 只比對原始碼 import | 從清單拿掉 |

## 附錄 B：開工前 design-review 的處理（v1.1 → v1.2；審查全文在 session scratchpad `5a-design-review-start.md`）

| 編號 | 發現 | 處理 |
|---|---|---|
| 1 | [A] PayUni 狀態放在 `GetMyPayUni`，依據 `CARRYOVER.md:334`「沒有畫面需要在同一個呼叫拿到兩者」已不成立；三個畫面要合併兩份回應、處理快取時差與交叉失效 | **Will 決定放進 `KYCFlow`**。F1、§1、§2、§3 改寫：`KYCFlow.payuni`（state、說明、審核意見、可改的欄位），刪 `payuniHasState(flowStatus, registration)`、快取時差規則、送出與取消後的交叉失效、not_found 後備；查證後 `GetMyPayUni` 沒有剩下的欄位（聯絡資料早已是店家欄位，P24），整支刪除（F11）；權限不變（P23）；`CARRYOVER.md:334` 改寫並寫明原依據為何不成立（§2 末、§7） |
| 2 | [B] client 依 state 值硬寫可做的動作；`ProblemOf` 對整份表單的碼回全部欄位、不依身分鎖過濾 | 改。伺服器只回賣家此刻能改的欄位（賣家型態、身分鎖、整份表單 → 空），「前往補件」讀 `can_resubmit`，清單與店家入口看清單非空，client 只留樣式（F14、§2、§6.1 的不變式與突變） |
| 3 | [A] 把狀態說明當 `failed_precondition`，APPROVED 為錯誤特寫一句；`ok == false` 留字面值 | **Will 決定 can＋原因句**。`can_resubmit`＋`resubmit_unavailabilities{reason, LocalizedMessage}`，`editable`、`Submit` 與回應用同一個函式（F8）；原因句 #27–#36 取代 #13、#25；到不了的分支回 internal |
| 4 | [C] 閘門 `gate_*` 的 PayUni 階段文字與狀態矛盾，延後的觸發上線前不會發生 | 改。閘門在 PayUni 階段用 PayUni 標題與說明，目錄 `gate_*` 六個值 `omit`；消費端是 website `kycFlowGate`→`SellerCertGate`（錢包、物流），seller-app `kycFlowCopy`→錢包閘門與 `shippingCertificationGateCopy`（P31、F9、§5 #26） |
| 5 | [C] 其餘沒有讀者的欄位（`sync_*`、`bind_error_code`、三個時間）延後 | 改。逐一以 rg 查證沒有讀者（P32），連同整個賣家端 `PayUniRegistration` 與 `GetMyPayUni` 刪除（F11） |
| 6 | [B] RETRY_LATER 承諾「平台會協助」，但 `PayUniBindFailed` 沒有訂閱者，只靠員工主動看 | 改。重試用完的綁定失敗進 3d 後台待辦（新成員 `review_payuni_bind_retry`、`bind_failed_at` 欄位、期限 24 小時），admin 的列表與連結已是通用的，只需加一個種類文字；歸 5a，與 5b 的分工見 §7.1（F16、F4） |
| 7 | [C] `FIX_VIA_SUPPORT` 指出申請欄位時賣家改不到，違反「一個值一種動作」 | 改。綁定失敗指出申請欄位時是 FIXABLE（FAILED 時身分不鎖，重新核准會自動重新綁定），`CanResubmit` 跟著；FIX_VIA_SUPPORT 只剩店家欄位；`Rebind` 在申請不是已核准時拒絕，避免把未審核的草稿送給 PayUni（F17、§5 #8、#38） |
| R3 | （修訂時查出）PayUni 拒絕後存草稿，申請一律回到 DRAFT（`kyc.sql:43-48`），v1.1 F2「草稿而且註冊是 PayUni 拒絕到不了」不成立，`payuni` 不設會讓賣家修正到一半時問題框消失 | 同一份申請的草稿仍回 FIXABLE（F2、F3、§6.1、§6.2） |

## 實作時的修訂

5a-1 實作與完成前 design-review、code-review 之後，與 v1.2 不同的地方（審查全文在 session scratchpad `5a-design-review-completion.md`、`5a-code-review.md`）：

- 5a-1：`PAYUNI_BIND_RETRY` 的權限維持設計的 `adminperm.SellerPayUniRegistrationsGet`（3d E13：呼叫者對來源的讀取權，來源是 PayUni 註冊，它唯一的讀取權限是 `.get`，沒有 list RPC；不新增沒有 RPC 的 `.list` 權限）。實作者曾為了讓 `TestKindsAreOneToOneWithTheContract` 的「結尾是 `.list`」通過而改對到 `SellerPayUniObservationsList`（觀測紀錄，與待辦揭露的事實無關），完成前 review 判為錯。改的是測試的規則：不再檢查後綴，改成逐種類寫死期望的來源讀取權限（新種類沒有列在表裡就失敗），`review_queue.go` 的註解跟著改。
- 5a-1：「申請此刻能不能儲存或重送」放在 `payuni` 套件而不是 `kyc` 的 `editable`：`payuni.CheckResubmit` 與 `payuni.SellerViewOf`（回流程、賣家的 PayUni 狀態、不能重送的原因）取代 `editable`、`Mine`、`MyPayUniRegistration`。`Submit` 的 `lockOwn` 白名單加入 SUBMITTED，讓送審中的申請也走 `CheckResubmit`，訊息出自原因句（`UNDER_REVIEW`、`UNDER_REVIEW_CANCELABLE`），不再是 `lockOwn` 的字面值。完成前 review F-2 之後，註冊只載入一次（帶賣家型態與店家缺的欄位）：`SellerViewOf` 的流程、`SellerPayUniOf` 與原因吃同一份，`DeriveKYCFlow` 的 `CanResubmit` 呼叫內部的 `sellerPayUniOf(reg)`；`stint.FlowOf` 刪除，`FlowsOf` 改名 `FlowStatusesOf` 只回階段；階段從 `DeriveKYCFlow` 拆成 `model.KYCFlowStatusOf(status, reg)`，批次讀（沒有賣家型態與店家缺的欄位）只呼叫它，不再拿沒補資料的註冊呼叫 `DeriveKYCFlow`（第 2 輪 N5）。
- 5a-1：`ClassifyPayUniCode` 的回傳值沒有加 `wholeForm`（§1 寫「多回這個旗標」）：它的呼叫者有十幾處不需要它，`problemOf` 直接讀 `payuniErrorIndex[code].wholeForm`。
- 5a-1：`SellerPayUniOf` 的 PAYUNI_REJECTED 分支（⑤）直接照設計寫：補救方式是 CONTACT_SUPPORT 給 CONTACT_SUPPORT，否則 FIXABLE（不列欄位；PAYUNI_REJECTED 只會是整份表單的碼），不經 `sellerFix` 再轉換；`withReason` 收算好的問題，不再重算（完成前 review F-6）。
- 5a-1：刪除 `KYC_RESUBMIT_UNAVAILABLE_REASON_CANCELLED`（enum 這次才新增、從沒發佈過，所以不保留編號與名稱，其餘值重新編號 1–9）、`ResubmitCancelled` 與句子 #36（`kyc.resubmit_unavailable.cancelled`）。已取消的申請不是目前的申請（`CurrentKYCApplication` 排除 CANCELLED，讀取回 not_found），這個原因只會出現在 CancelKYC 的回應，卻說「不能修改」而 SaveKYCDraft（版本 0）其實會被接受，違反 `can_resubmit` 的定義（完成前 review F-4）。已取消的流程與重讀的「沒有申請」一致：NOT_STARTED、**`can_resubmit` 為真**（可以開始，SaveKYCDraft 版本 0 會建立新的）、沒有原因（符合 proto「為假時至少一個原因、為真時是空的」）、沒有 `payuni`、`identity_locked` 為假（取消在身分鎖住時被擋下），由整合測試鎖住（第 2 輪 code-review N2 修正第一版的「不能重送、沒有原因」，那個值自己違反契約）。§5 的 #36 作廢。
- 5a-1：刪除 `model.KYCFlow` 的 `NeedsReonboarding`、`PaymentEnabled`、`BindStatus`、`Outcome`、`Problem` 五個欄位：沒有正式程式讀它們（`toFlow` 從沒映射），PayUni 的事實只在 `PayUniRegistration`，賣家的 PayUni 狀態是 `SellerPayUniOf`，流程不留第二份（`CARRYOVER.md` 的沿用盤點本來就寫「重複不成立」）。原本讀這些欄位的測試斷言移到註冊與 `SellerPayUniOf`（`TestDeriveKYCFlow`、`payuni_integration_test.go`）。5b 的設計不依賴它們。
- 5a-1：改了四個既有測試，沒有一個削弱斷言：(1) seller-app `report_status_label_test.dart` 的 `REVIEWING` 字面掃描加 `(?<![A-Z_])`，排除 `PAY_UNI_STATE_REVIEWING`（不同的 enum），獨立的 `'REVIEWING'` 字面值仍抓得到；(2) ui-text 的 TS `format.test.ts` 與 Dart `check_enum_text_test.dart` 的 `inCatalog` 15 → 16，因為目錄新增 `PayUniState`；(3) `exposure_rule_dataset_integration_test.go` 的夾具補 `bind_failed_at = updated_at`，新的 CHECK 要求 FAILED 列有它；(4) admin `review-queue-page.test.ts` 與 `review_queue_test.go` 的種類數 11 → 12（仍以「每種都有」對整個 enum 做全稱檢查）。另外 website `kyc-flow.unit.test.ts` 把「PayUni 階段沒有閘門文字（copy 為 null）」改成「閘門是『未知』」（L5，F9 的 omit 值仍是同一組六個，斷言換成對「未知」的全稱檢查）。
- 5a-1：website `kycFlowGate` 在目錄沒有閘門文字（PayUni 階段卻沒有 `payuni`，契約被違反）時回共用文案目錄的「未知」標題與內容，`SellerCertGate` 不再在這時什麼都不畫，與 seller-app 一致（code-review L5）；沒有新增文案。seller-app `PayuniStatusBody` 的欄位清單每列的 key 改成帶序號，原本多個欄位時同一個 `Column` 有重複 key（code-review H1）。
- 5a-1：§6.2 的 `ShippingClient.unit.test.ts` 閘門案例補上（實作時漏了，code-review L4）；重新綁定被擋下的測試補上可達的路徑（綁定失敗在申請欄位、賣家儲存草稿、管理員重新綁定被擋下並說明原因，F-8），原本的案例依賴直接改資料庫狀態的不可達狀態，只留給資料集測試。
- 5a-1（第 2 輪 N1）：兩個 client 的 `kycEditable` 改成 `app == null || app.flow.canResubmit`。第一版保留了「DRAFT、REJECTED、PAYUNI_REJECTED 才可編輯」的值 → 動作表，F17 的 BIND_FAILED + FIXABLE（`can_resubmit` 為真）在認證頁是鎖住畫面，沒有編輯入口。測試的申請夾具跟著伺服器的值：草稿與被退回的 `can_resubmit` 為真。

## 附錄 C：完成前的 design-review（8 項；全文在 session scratchpad `5a-design-review-completion.md`，不入庫）

| # | 發現 | 處理 |
|---|---|---|
| F-1 | `PAYUNI_BIND_RETRY` 的權限對到無關的 `SellerPayUniObservationsList`（B，中） | **改**：維持設計的 `SellerPayUniRegistrationsGet`，測試規則改成逐種類寫死期望權限 |
| F-2 | `can_resubmit` 與 `payuni` 吃兩份不同組裝的註冊（B，中） | **改**：註冊只載入一次，`payuni.SellerViewOf`，`DeriveKYCFlow` 呼叫內部的 `sellerPayUniOf` |
| F-3 | `model.KYCFlow` 留著五個沒有讀者的 PayUni 欄位（A，中） | **改**：刪除，斷言移到註冊與 `SellerPayUniOf` |
| F-4 | CANCELLED 原因從不可達的分支沿用（A，中） | **改**：刪原因、句子與契約值，已取消的流程是 NOT_STARTED 且 `can_resubmit` 為真 |
| F-5 | 沒有「實作時的修訂」（C，中） | **改**：補上本檔文末那一節 |
| F-6 | PAYUNI_REJECTED 分支繞過設計 ⑤（B，低） | **改**：照 ⑤ 直接寫，`withReason` 不再重算 |
| F-7 | KYC ADR 文末修訂的「補件」與程式矛盾（docs，低） | **改**：刪「補件」 |
| F-8 | 重新綁定被擋下的測試只走不可達的狀態（test，低） | **改**：補可達路徑（綁定失敗在申請欄位、儲存草稿、管理員重新綁定被擋下） |
