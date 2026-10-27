# 目錄 enum 文字守門改依條件，與 seller-app 停權效果句進共用文案目錄，實作設計 v1.3

> **狀態**：v1.3。v1 經一輪獨立審查（附錄 A），v1.2 經開工前 design-review（F1-F7，重點是刀 1 契約；逐條對照程式查證後全部接受，對照見附錄 B；全文在 session scratchpad `enum-guard-design-review-start.md`，不入庫）。v1.1 的主要變更：規則 `E` 只算「確定等於某成員」的區域（K3）；目錄分界改在多語系 ADR D2 加文末修訂（K12，§4.1）；拆成契約、目錄與 client、守門三刀（§7）。**v1.3 的變更（Will 2026-10-22，§0.6）**：食品業者登錄不加可用性布林，改把 `open_food_registration` 拆成待審與被退回兩個欄位（Q4 改 (A)）；終止表單的契約只加依據的三態需求，不加 `requires_source_measure`，條件句寫進伺服器的 `TerminationOption.explanation`；admin 關聯帳號終止的說明句改由 `explanation` 提供，不進目錄。§5 #12-#16 的新句（zh-TW）Will 2026-10-22 已核可原文，**沒有待決項目**。

v1.3 讀取的是 `local/10e` 的 `b10f015bc`（v1.2、計畫與 ADR D4 修訂已提交）；v1.1 以前讀的是 `0a3265c1f`。計畫行號（`:383,399,400`）照 v1.2 時的內容；本文不改 ADR 與計畫。
SSOT：計畫 `2026-09-29-a3-buyer-seller.md:383,399`；ADR `backend/docs/adr/2026-10-17-multi-locale-architecture.md`（D2 分界 `:77`）；`step10e-Lprime-design.md`（下稱 L′：§1.2 key、§1.3 `params`、§1.4 ICU 子集（禁止 `{arg, date}`）、§5.2 守門、O4 `:835`、§7(d) `:716-721`、R6 `:856`）；`step10e-5d-design.md` G10；`backend/docs/開發規範/使用者面用語術語表.md`；根目錄與各 client 的 `AGENTS.md`；契約 `api/hawkr/admin/seller/v1alpha1/{measure,governance}.proto`、`api/hawkr/seller/v1alpha1/seller.proto`。
路徑縮寫：`W` worktree 根、`DT`＝`W/packages/hawkr_ui_text`、`TT`＝`W/packages/hawkr-ui-text-ts`、`C`＝`W/backend/internal/platform/uitext/catalog`、`AD`＝`W/admin/src/app/admin`、`SA`＝`W/seller-app/lib`、`WS`＝`W/website`、`FR`＝`W/backend/internal/modules/seller/internal/foodreg/foodreg.go`、`SM`＝`W/backend/internal/modules/seller/internal`。
標記：**[量測]** 讀碼或執行命令看到的；**[推論]** 由量測推出、沒有執行驗證；**[外部]** 業界做法。量測用的原型在 session scratchpad `eg/proto.mts`（TS，v1.1 的正向版）、`eg/proto-v1.mts`（v1 版）、`eg/dartproto/bin/proto.dart`（Dart，v1 版），輸出 `eg/ts-hits6.tsv`（v1.1）、`eg/ts-hits4.tsv`、`eg/dart-hits3.tsv`（v1）（不入庫）。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| 兩個守門工具 `DT/tool/check_enum_text.dart`（407 行）、`TT/tools/check-enum-text.ts`（485 行）與測試 | 沒有排程 | 會留下；規則比對的那一段重寫，import 解析、manifest、`--report`、R3、命令列沿用 |
| `scripts/check-enum-text-cross.sh` | 沒有 | 會留下；只改檔頭說明 |
| seller-app `SA/features/seller/data/measure_display.dart` | 沒有。已走 Connect 與目錄 | 會留下 |
| admin 的措施、終止、KYC、案件畫面（§2 列的 7 個檔） | 沒有。已走 Connect | 會留下 |
| 食品業者登錄：seller-app `SA/features/wallet/presentation/food_registration_form.dart`、website `WS/features/seller-center/components/FoodRegSupplement.tsx`、`backend/internal/httpapi/seller/app.go` 的 `toStore` 與賣家契約 `Store` | 沒有 | 會留下 |
| 終止合作的契約 `TerminationOption`（`measure.proto:236-242`）與 seller 模組 | 沒有 | 會留下 |

沒有程式會因為現在改而被丟棄［推論］。

## 0.1 範圍（三刀，依序；§7）

| 刀 | 內容 |
|---|---|
| 刀 1 契約 | Q2(b)：`TerminationOption` 帶依據（`TerminationInput.violation`）的三態需求，事由的條件句（重大違約的 7 日、關聯帳號的連帶撤銷）寫進伺服器的 `explanation`；Q4(A)：賣家 `Store` 的 `open_food_registration` 拆成 `pending_food_registration` 與 `rejected_food_registration`。契約、seller 模組與 httpapi、三個生成端、`make uitext`（manifest 與模組句子）、admin／seller-app／website 跟進。細節 §2.1。理由是「伺服器推導、client 只呈現」，不是守門（K13） |
| 刀 2 目錄與 client | ADR D2 文末修訂與 L′ §7(d) 文末修訂（K12）；目錄新 key（seller-app 三句）；`make uitext`；§2 的 client 修法（含 Q1、Q3）；術語表加一列 |
| 刀 3 守門 | 兩個工具的 D1、D2、R1、R2 刪除，換成規則 `E`（§1，含跨檔匯入的常數，K4）；測試、檔頭、跨端腳本檔頭、各 `AGENTS.md` 的守門說明。最後做：前兩刀把命中歸零後，換工具的 commit 才不會讓 CI 紅 |

## 0.2 不在範圍

- 硬寫中文的 ratchet（計畫的 I-CI，`:400`）：另一刀。本規則是「目錄 enum 的文字只有一個來源」；I-CI 是「會發佈 en 的介面不准新增中文字面值」。兩者互不取代（簡報「為什麼 P1 不能取代 P2」；ADR D4 文末修訂的「不在這項」）。
- 用 client 文字資源（ARB、next-intl）查詢的文字：現在 App 沒有 ARB、website 的 messages 只有 7 句［量測，簡報］；「產生文字」先只看漢字字面值與常數，資源查詢加進 K4 的時間：website 的 next-intl 查詢與計畫 I-CI 的 website 那一刀同一刀（從那時起新文字一律放進 messages，不加就能用 `t("…")` 繞過），App 的 ARB 查詢與計畫 I-app-0 同一刀（刀 3 完成前審查 F1；觸發寫在 §9）。
- website 的停權期間句（`停權期間：{start} 至 {end}` 等）：照 L′ §7(d) 留在 client（v1 的 K10 刪除；它在 K3 的正向定義下不命中，§2）。
- 日期依語系格式化：目錄的日期參數是 client 格式化好的字串（L′ §1.3 `{"type":"string"}`、§1.4 禁止 `{arg, date}`、ADR D2「數字與日期由各端 helper 格式化後當參數」）；三句沿用 `formatTaipeiTimestamp` 的 `YYYY/MM/DD HH:mm`，與現有的 `termination_effective` 相同。

---

## 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| K1 | 規則 | **一條規則 `E`**：使用者面文字（K4）出現在由**受管 enum 的某個成員確定成立**的區域裡（K3），就是命中。受管＝manifest `inCatalog \|\| serverOwned`，命中就失敗；其他 enum 的命中只回報（K8）。沒有範圍參數、沒有例外清單（沿用 L′ `:588`） | Will 決定 1。舊規則是形狀黑名單，if 形式已經漏了一處（seller-app，H3） |
| K2 | 選擇子：什麼算「受管 enum 的成員」 | `E.M`／`ns.E.M`（TS）、`E.M`／`p.E.M`（Dart），`E` 由 import 解析到 api 生成檔（沿用兩個工具現在的 `resolve`／`_importedProtoEnums`）。**值號 0 的成員不是選擇子**：名稱以 `UNSPECIFIED` 結尾。受管範圍含 `serverOwned` | 目錄不給值號 0 文字（`backend/cmd/uitext-gen/keys.go:122-124`），所以「請選擇」「所有類別」這類以 UNSPECIFIED 為值的選項不是目錄文字的複本；不排除時 TS 多 12 筆受管命中，全部是這種［量測］。名稱尾碼與值號 0 等價：buf lint STANDARD 的 `ENUM_ZERO_VALUE_SUFFIX`（`api/buf.yaml:10-17` 只忽略 `ENUM_VALUE_PREFIX`），`rg "= 0;" -g '*.proto' api \| rg -v UNSPECIFIED` 沒有輸出［量測，審查］。`serverOwned` 是 SSOT 的範圍，不是偏離：L′ O4 `:835`「四端一律擋在目錄與伺服器擁有的 enum」，現行 `forbidden()` 也是（`check-enum-text.ts:422-430`）［量測］；今天它的命中是 0［量測］ |
| K3 | 「選出」的精確定義 | **只算正向**：進入區域 R 的每一條路徑都已確定「x 等於成員 M」，R 才是被 M 選出。`implied(expr, polarity)` 回傳 expr 取該真假值時確定**相等**的成員：`x === M`／`x == M` 為真、`x !== M`／`x != M` 為假時回 M；集合字面值的成員檢查（TS `[E.A,…].includes(x)`、`new Set([…]).has(x)`；Dart `{E.A,…}.contains(x)`）為真時回各成員；`!a` 反轉；`a && b` 為真時合併 a、b；`a \|\| b` 為假時合併 a、b；其他回空。各語法的區域見 §1.2 | 補集區域（`x === M ? 甲 : 乙` 的乙、`if (x === M) return; 之後`、`default:`）代表「不是 M」，不是任何一個成員的說法；對照表靠正向分支就抓得到：`x === A ? "a" : "b"` 的 "a"、`x !== A ? "b" : "a"` 的 "a"（為假推出相等）、`if (x === A) return "a"; return "b";` 的 "a" 都命中。只漏「只有補集有文字」（`x === A ? null : "…"`），那不是對照表［推論，審查 F2］。量測：正向版比 v1 少 website `measureDisplay.ts:136`、admin `violation-measure.ts:190,192` 的 default、seller `measure_display.dart:203,209` 的 guard 與兩處 else 分支［量測：`ts-hits6.tsv` 對 `ts-hits4.tsv`；Dart 由極性推論］ |
| K4 | 「產生文字」 | 區域裡有含漢字的字串（TS：`StringLiteral`、樣板字串各段、`JsxText`；Dart：`SimpleStringLiteral`、`InterpolationString`；漢字範圍沿用兩個工具的 `HAN`），**或**引用同一個檔裡、作用域可見的 `const`（Dart 加 `final`，頂層或區域皆可），其初始值是含漢字的字串。不分是 return、賦值、JSX 子節點、widget 參數還是 `throw` | 舊 D1／R2 只看 return 與賦值（`check_enum_text.dart:110-134`），`case` 裡用 widget 或 `throw` 帶文字就漏。常數：量測有 6 處用同檔常數（其中 `kyc-application.ts:707,709` 在舊 R2 的 `case` 形狀裡、舊規則沒擋），加上後多 1 筆受管命中（`measure.ts:576`）；區域常數再多 1 筆（`KycApplicationCards.tsx:502`）［量測］。區域常數用 K5 同一套作用域查找（審查 F11）。跨檔匯入的常數也追（刀 2 完成前審查 F1：`ActiveMeasuresCard.tsx` 的 `VIOLATION_MEASURE_TEXT` 來自 `store-detail.ts`，只看同檔會漏掉；原型 `eg/proto-xfile.mts` 證明追跨檔常數剛好抓到這一筆）。範圍：import 解析到同一個 client 目錄樹內的檔案、該檔匯出的含漢字字串 `const`（TS）／頂層 `final`、`const`（Dart），遞迴一層即可 |
| K5 | 別名 | 條件裡的識別字若是作用域內、用到之前宣告的 `const`（TS）／`final`、`const`（Dart）區域變數，而且初始值是比較（K3）或 `cond ? x : null`（`undefined`），就以初始值代入：前者照 K3，後者的真值蘊含 cond 為真。別名的初始值裡又是別名時遞迴代入 | 只算直接比較時，`ActiveMeasuresCard.tsx:43`（`const isTermination = measure.kind === …` 再 `isTermination ? "合作已終止" : …`，正是目錄 `MeasureState.ACTIVE` 的字）與 `FoodRegSupplement.tsx:76`（`const pending = … === PENDING ? open : null`）都漏掉［量測］ |
| K6 | 提早 return | `if (c) {…; return/throw/break/continue}` 沒有 else、then 一定離開時，同一個區塊裡**之後的敘述**是 c 為假的區域；依 K3 只在 c 為假蘊含相等時（例如 `if (x !== M) return …;`）才有選擇子 | 它是 else 的另一種寫法；正向的情形（`if (kase.category !== FOOD_SAFETY) return null;` 之後）不算就能繞過［量測：`CaseSections.tsx:246`］ |
| K7 | 舊規則 | **D1、D2、R1、R2 刪除**，由 `E` 吸收：D1／R2＝§1.2 的 case；D2／R1＝key。`Hit.rule` 改成 `"E"` 加 `form`（`if`、`else`、`ternary`、`and`、`or`、`guard`、`case`、`key`、`pair`），訊息印 form。R3（`userMessage(…) ?? "…"`）不是 enum 規則，不動 | 不留兩套並存。原型在 admin 對舊規則的回報做對照：舊規則的 122 筆位置全部被新規則的 key／case 涵蓋，多 1 筆（`violation-measure.ts:192`，case 裡是 `throw`）［量測：`diff`］ |
| K8 | `--report` 與跨端檢查 | report 只放**對應形式**（case、key、pair）的命中，條件形式只用在 K1 的失敗判斷。`scripts/check-enum-text-cross.sh` 不改邏輯 | 跨端檢查問的是「同一個 enum 的值→文字對照表在兩個 client 各寫一份」（L′ `:589`）。條件形式在單端 enum 上（v1 量測）35 筆，抽看的都是行為上的分流（`DeleteAccountSection.tsx:56` 失敗訊息、`CertificationsClient.tsx:1145` 捨棄草稿按鈕）；放進 report 會讓 `KYCStatus`、`DeleteAccountResponse.DeletionState` 兩個單端 enum 因 admin＋website 跨端失敗［量測］。只放對應形式時跨端是 0［量測］ |
| K9 | seller-app 三句（Will 決定 2） | 進目錄，訊息句 key 掛在 `seller.Measure`：`suspension_active_until`（`停權中（至 {end}）`）、`suspension_active_open`（`停權中（未定結束時間，平台定期複審）`）、`suspension_ended`（`已於 {end} 結束`）；`end` 是 `{"type":"string"}`。`measureEffect` 改呼叫生成的函式 | 與 `seller.Measure.termination_effective`（`終止生效：{start}`）同一種形狀（L′ §1.3）。不用 `MeasureState` 的變體：變體要求每個非零值都有文字（L′ §1.3 `variants`）。`停權中（至 …）` 是目錄 `MEASURE_STATE_ACTIVE` 的「停權中」加期間，是真的重複（`C/zh-TW.json:653-654`）［量測］。website 與 admin **沒有**顯示這三句：website 顯示的是期間行，admin 沒有效果行［量測：`rg '停權中\|定期複審\|已於 '`］ |
| K10 | （v1 的 website 期間句進目錄） | **刪除**：照 L′ §7(d) 留在 client | K3 正向定義下不命中［量測］ |
| K11 | admin 與其他 client 的命中 | 依 §2 分類：(a) 以目錄 enum 成員選出的句子，改用目錄；(b) 依契約欄位或請求形狀改寫條件；(c) 由刀 1 的契約欄位決定 | 量測 §2 |
| K12 | 目錄分界 | **多語系 ADR D2 加文末修訂**（§4.1 起草；ADR 是分界的 SSOT，根 `AGENTS.md` 只是摘要）：進目錄的是「只由契約值決定，而且兩個以上 client 用到，**或**只在某個目錄 enum 成員成立時才出現的句子」。L′ §7(d) 加文末修訂（seller-app 三句改由目錄提供）。根 `AGENTS.md` 共用文案目錄列照 ADR 改寫（§4.2） | ADR `:77`「分界不變：只由契約值決定、兩個以上 client 用到的文字進目錄…；其他文字在各 client 的資源」；L′ `:147,203,720` 同［量測］。第一批只有 seller-app 三句（admin 關聯帳號說明句改由伺服器 `explanation` 提供，開工前審查 F4）。規則 `E` 本身蘊含分界改變：I-app-0 之後 K4 把 client 資源查詢也算成文字，被目錄 enum 成員選出的句子在 client 資源也放不了，只剩目錄（審查 F1）。範圍用 K3 的正向定義 |
| K13 | 契約改動的理由 | Q2(b)、Q4(A) 以「伺服器推導、client 只呈現」為理由，各自成立；不以「為了過守門」為理由。都**不新增平行機制**：終止沿用 `TerminationOption`（生效時間＋`explanation`）的模式；食品業者登錄用「欄位有沒有值」表達狀態，帳號能不能操作交給既有的 `GetMyStore.capabilities` | 被命中的是按鈕文字「撤回」與表單欄位，不是 enum 的說法。Q2：契約允許其他 21.3 事由選填依據（`governance.proto:885-886`），畫面只在重大違約顯示，是功能缺口［量測］。Q4：client 要比較 `PENDING`，根源是 `open_food_registration` 把待審與被退回合成一個欄位（`seller.proto:350-352`）；`Store` 已經用 `approved_`／`revoked_food_registration` 這樣以欄位切狀態［量測］。撤回還要 `CapStorefrontWrite`（`FR:61-67,146-150`），這由 `SellerCapability` 負責：契約寫明「client 以它決定畫面上哪些操作可用」（`seller.proto:231-234`），兩個 client 的錢包頁已經以 `STOREFRONT_WRITE` 擋住（H14）［量測］ |

## 0.4 查證過的現況（`0a3265c1f`）

| # | 事實 | 證據 |
|---|---|---|
| H1 | 兩個工具今天對 website、admin、ts 套件、兩支 App、Dart 套件都是 exit 0；admin 的 report 有 35 個單端 enum | [量測] 在 `TT`、`DT` 執行工具帶 `--report` |
| H2 | 檔頭寫三元式「由 I-CI 的逐檔漢字 ratchet 補」；I-CI 現在排在計畫 `:400`，改成逐筆 baseline、而且不含 admin | [量測] `check_enum_text.dart:13`、`check-enum-text.ts:16`；計畫 `:400` |
| H3 | seller-app `measureEffect` 以 `if (state == MeasureState.X)` 選出三句漢字；`MeasureState` 在 manifest 是 `inCatalog: true` | [量測] `measure_display.dart:228-235`；`DT/tool/enum_manifest.g.json` |
| H4 | 目錄的日期參數是字串、由 client 格式化；ICU 不准 `{arg, date}`；`en.json` 只有 5d 的 4 個 key，`required` 只有 zh-TW | [量測] L′ §1.3、§1.4；`C/en.json`、`C/config.json` |
| H5 | 只比較目錄 enum、不產生文字的行：TS 148 行、Dart 37 行（名稱比對，上限）；新規則在這些上面不命中 | [量測] 簡報；原型的受管命中只有 §2 的位置 |
| H6 | `CreateMeasureRequest` 的種類是 oneof（`warning`／`suspension`），契約註解寫「不會有跨欄位的種類對應」；admin 的表單另外存 `kind: MeasureKind` 再在建請求時轉成 oneof | [量測] `governance.proto:850-851`；`violation-measure.ts:47,140,190-200` |
| H7 | `Measure.appeal_deadline_time`：終止合作才有，其他種類不設 | [量測] `measure.proto:155-156` |
| H8 | `Case` 已有 `can_record_fda_referral`、`can_export_fda`、`can_waive_fda_referral`、`can_record_fda_referral_reply` 與 `fda_referral`、`fda_referral_waiver`；`CaseReferralSection` 的按鈕已經只看 can_* | [量測] `governance.proto:493-509`；`CaseSections.tsx:236-246` |
| H9 | admin `Measure.end_time` 與 `period.end_time` 都來自 `EndsAt`；`MeasurePeriod.end_time` 在預告終止的店家關閉時由系統寫入、可能早於 `start_time`；停權的 `period.start_time` 是建立時間 | [量測] `backend/internal/httpapi/seller/governance_measures.go:101,116`；`seller.proto:1221-1226` |
| H10 | KYC 身分區塊的標題已依 subject 區分：「公司負責人身分資訊」／「賣家身分資訊」 | [量測] `KycApplicationCards.tsx:342,368` |
| H11 | 食品業者登錄的送出、撤回、清除三個 RPC 在模組裡都先 `authorize`（`CapStorefrontWrite`）；`Submit` 除此之外只有輸入驗證；賣家 `Store.open_food_registration`（27）是「待審或被退回」的那一筆，`Store` 的欄位編號到 28；admin 的 `Store.open_food_registration`（30）是另一個欄位 | [量測] `FR:61-67,74-110,134,146-150,163`；`seller.proto:345-356`；`admin/seller/v1alpha1/seller.proto:238` |
| H14 | 兩個 client 的錢包頁（放食品業者登錄的頁面）都以 `STOREFRONT_WRITE` 擋住：seller-app `core/router/route_capabilities.dart:34`、website `features/seller-center/components/pageCapabilities.ts:46`；兩個表單元件只用在錢包頁（`wallet_account_section.dart`、`WalletClient.tsx`） | [量測] |
| H15 | 終止的依據規則寫在 `SM/measure/create.go:147-156` 的 switch（`ContractOnly()` 不可帶依據；重大違約要引用至少一筆認定）；7 日在有狀態的檢查（`create.go:510-518`，`model.MaterialBreachCure`）。給員工的生效時機句只有 `model.TerminationEffect.Message` 一個產生點（`SM/model/termination_effect.go:44`，句子在 `SM/text/zh-TW.json:619-630`）。`available_terminations` 不含 `LINKED_ACCOUNT`，它只出現在 `LinkedAccount.termination_option`（`admin/seller/v1alpha1/seller.proto:336-339`、`measure.proto:229-231`） | [量測] |
| H16 | admin 終止表單把伺服器的 `explanation`、client 常數 `TERMINATION_LINKED_TEXT`、`TERMINATION_COMMON_TEXT` 串成說明（`measure.ts:557-580`）；重大違約的欄位帶 `emptyText`「這個帳號沒有生效中的認定，無法以重大違約終止。」與 `noteText`「至少選一筆；引用的認定必須未撤銷、已存在 7 日以上。」（`CreateTerminationDialog.tsx:199-206`） | [量測] |
| H12 | 以 enum 值當索引的文字表（`LABELS[m.kind]`）：搜到的都不是受管 enum（舊 REST 的訂單狀態字串、buyer-app 以字串碼為 key、admin `NO_ENTRY` 的值是布林） | [量測] `rg '\[[a-zA-Z_.?]+\.(kind\|state\|status\|ground\|reason)\]'` 於 admin、website、兩支 App |
| H13 | 術語表有 停權＝suspension、措施＝enforcement action、終止合作＝termination（「合作已終止」＝Partnership terminated），沒有「複審」 | [量測] 術語表 `:33-39`；`rg 複審` 沒有結果（審查） |

## 0.5 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| 工具只做語法分析（`parseString`、`createSourceFile`），沒有型別 | SELF-IMPOSED（我們選的工具，L′ §5.2）；換成型別分析要建整個專案的 program，CI 成本大；本刀不換，殘餘列 §1.6 | L′ §5.2 |
| 沒有範圍參數、沒有例外清單 | SELF-IMPOSED，但是好規則，保留 | L′ `:588` |
| admin 也要只用目錄的 enum 文字；受管含伺服器擁有的 enum | 已核准（L′ O4、Will） | L′ `:835`；`admin/AGENTS.md:21` |
| 目錄不給值號 0 文字 | 已核准的規則（L-brief Q4），生成器機械化 | `keys.go:122-124` |
| 目錄分界「兩個以上 client」 | 已 accepted 的 ADR D2；本文的修訂（K12）Will 2026-10-22 已核可 | ADR `:77` |
| 停權期間句兩端不同 | 已核准的版面決定（L′ §7(d)）；website 那組不變 | L′ `:716-721` |
| 改契約時同一次變更更新唯一來源與所有消費端 | REAL（根 `AGENTS.md` 規則） | 根 `AGENTS.md` |
| 版面、UX 文案與流程的改動要先取得同意 | 授權規則；§5 與 §10 | 根 `AGENTS.md` |

## 0.6 決策紀錄

| 日期 | 誰 | 決定 | 本文位置 |
|---|---|---|---|
| 2026-10-22 | Will | 不在既有工具上加一條三元式規則；兩個 AST 工具的形狀黑名單換成看條件的規則（分支或資料由目錄 enum 成員選出、其中產生文字就失敗；涵蓋 if／else、三元式、`&&`、switch 敘述與運算式、case、map／object key、`value: E.M` 與文字配對的集合字面值）；舊規則被涵蓋就刪；檔頭指向不存在 I-CI 的說明改寫 | K1-K8、§1 |
| 2026-10-22 | Will | seller-app `measure_display.dart:228-235` 的三句進共用文案目錄，帶日期參數，zh-TW 與 en；en 由本文提案、Will 核可 | K9、§3、§5 |
| 2026-10-22 | Will | ADR D2 文末修訂照 §4.1 原文核可 | K12、§4.1 |
| 2026-10-22 | Will | §5 文案全部核可：en 三句（Suspended until {end}／Suspended (no end date; under periodic review by the platform)／Ended on {end}）、admin 三處改字（「停權中」、「身分證正面照／反面照」、「補充文件 {n}」）、術語表新列「定期複審｜periodic review」 | §3、§5 |
| 2026-10-22 | Will | Q1 看得到的改變接受（停權多一列生效時間＝建立時間；已結束的預告終止多一列可能早於生效時間的結束時間；沒有結束時間的停權不再顯示「結束時間：-」） | §2 #3、§10 Q1 |
| 2026-10-22 | Will | 契約刀方向：Q2(b)（`TerminationOption` 帶認定需求與來源措施需求）、Q4(a)（店家帶送出、撤回、清除三個動作的可用性）。兩者的細節由下方開工前審查的 F1-F3 決定修正 | K13、刀 1、§10 Q2／Q4 |
| 2026-10-22 | Will | 開工前審查 F1 採 (A)：`open_food_registration` 拆成待審與被退回兩個欄位，以有沒有值表達，不加可用性布林；帳號的停權維持由既有的 `GetMyStore.capabilities`（`STOREFRONT_WRITE`）擋。刀 1 刪 Q4 的三個布林 | K13、§2.1、§10 Q4 |
| 2026-10-22 | Will | F2：刪 `requires_source_measure`；admin 終止表單的狀態照請求的形狀 | §2 #10、§2.1 |
| 2026-10-22 | Will | F3：`TerminationOption` 只帶依據的三態（必填／選填／不可帶）；「認定須已存在 7 日」這類條件寫進伺服器產生的 `explanation` 句，client 不寫任何規則。新句由本文起草（admin 只有 zh-TW），核可前列 N | §2.1、§5 #12-#16 |
| 2026-10-22 | Will | F4：admin 關聯帳號終止的說明句由 `TerminationOption.explanation` 提供（刀 1），不進共用目錄；從 ADR D2 修訂的第一批拿掉，`admin.seller` key 前綴的風險一併消失 | §2 #6、§3、§4.1 |
| 2026-10-22 | Will | §5 #12-#16 的文案照原文核可（伺服器 `explanation` 兩則：重大違約、關聯帳號；admin 終止表單「依據的認定」三句）；刀 1 可開工 | §5 #12-#16 |
| 2026-10-22 | controller | 開工前審查 F5-F7：刀 1 寫清楚規則抽成一個函式、驗證與輸出共用、加等價測試，欄位編號與命名照請求、enum 前綴照 API 設計規範；刀 1 跑 `make uitext` 並重生 manifest；§2 #11 只有撤回按鈕改條件，狀態行留在原條件 | §2.1、§6.4、§7、§8、附錄 B |
| 2026-10-22 | controller | 接受獨立審查 F1-F14：ADR D2 文末修訂（不只改 `AGENTS.md`）；規則只算正向區域、刪 K10；Q3 由 SSOT 回答；拆三刀（契約、目錄與 client、守門）；Q4 涵蓋送出、撤回、清除三個動作；§2 #3 用 `measure.state`；KYC「負責人」、區域常數、pair 範圍收斂、術語表、en 用字 | 附錄 A |
| 2026-10-22 | Will | 刀 2 修正輪 A（完成前審查 F1）：admin 生效中措施卡的違規依據句「因違規處分停權；違規認定只供查閱，撤銷要對這個措施進行。」改由伺服器提供：`Measure` 加 `end_unavailabilities`（照 `extend_unavailabilities` 的形狀），規則 `Measure.EndUnavailableReasons` 同時是 `CanEnd`、`End` 的前置檢查與 API 的來源，有等價測試；句子逐字沿用；admin 刪 client 的句子與 `MeasureBasis.VIOLATION` 分支，卡片顯示伺服器給的第一個原因 | 修訂（刀 2）#8 |
| 2026-10-22 | Will | 刀 2 修正輪 B（審查 F4）：措施詳情頁的「結束時間」列與卡片同一個規則：沒有 end_time 而有 review_by 時寫「未定（由平台定期複審）」；兩處共用 `endTimeText` | 修訂（刀 2）#9 |
| 2026-10-22 | controller | 刀 2 修正輪：對 `hawkr_ui_text.dart` 跑 `dart format`；L′ 修訂的 R6 一句改成第 3 刀起；卡片的申覆期限只看欄位；轉報區塊單獨觸發的測試；記錄 Q1 的兩個後果；刀 3 的工具要解析跨檔常數 | 修訂（刀 2）#7、#10-#12、§1.6、§6.1 |

---

## 1. 規則 `E`

### 1.1 兩端共同的模型

一次走訪一個檔：先收 import（沿用）、可見的含漢字常數（K4）、可代入的區域別名（K5）；再走訪語法樹，每遇到一個選擇構造就算出 (選擇子集合, 區域集合, form)，區域裡有文字（K4）就對每個選擇子記一筆 `Hit{file, line, rule:"E", form, enumName, member}`，`line` 是條件、key 或配對成員所在的行。同一個 (file, line, form, enum, member) 只記一次。巢狀時內外層各自判斷。

`implied(expr, polarity)` 照 K3：去括號、`!`、TS 的 `as`／`!`／`satisfies`；`&&`、`||` 照 K3 組合；比較只在確定相等時回成員（值號 0 除外）；成員檢查只在為真時回成員；別名照 K5 代入；其他回空。

### 1.2 選擇構造與區域

| 構造 | TS 節點 | Dart 節點（`package:analyzer` AST） | 區域 | form |
|---|---|---|---|---|
| if | `IfStatement` | `IfStatement`（含 if-case）、集合裡的 `IfElement` | then ← `implied(cond, true)`；else ← `implied(cond, false)`（例如 `x !== M` 的 else）。Dart if-case：then ← 樣式裡的相等選擇子＋`when` 的 `implied(…, true)`；else 不算 | `if`、`else` |
| 提早 return | `IfStatement` 沒有 else、then 最後一句是 return／throw／break／continue；父節點是 `Block`／`SourceFile`／`CaseClause`／`DefaultClause` | 同左；throw 是 `ExpressionStatement(ThrowExpression)`；父節點是 `Block` | 之後的同層敘述 ← `implied(cond, false)` | `guard` |
| 三元式 | `ConditionalExpression` | `ConditionalExpression` | 兩支，同 if | `ternary` |
| 短路 | `BinaryExpression` `&&`、`\|\|` | `BinaryExpression` `&&`、`\|\|` | `a && b`：b ← `implied(a, true)`；`a \|\| b`：b ← `implied(a, false)` | `and`、`or` |
| switch | `CaseBlock`：`CaseClause` 的運算式是選擇子，或（`switch (true)`）是比較 | `SwitchStatement`（`SwitchCase`、`SwitchPatternCase`）、`SwitchExpression`；樣式裡任何位置的 `ConstantPattern`、`RelationalPattern` `==` 都算，含 or／and／record／object／cast／null-check 樣式；`when` 照 `implied(…, true)` | case 的敘述或運算式（fall-through 的前幾個 case 合併）。`default:`、`_` 不算（補集，K3） | `case` |
| key | `PropertyAssignment` 的 computed key 是選擇子 | `MapLiteralEntry` 的 key 是選擇子 | 值 | `key` |
| 配對 | `ObjectLiteralExpression` 有屬性值是選擇子；`ArrayLiteralExpression` 有元素是選擇子；`CallExpression`／`NewExpression` 有引數是選擇子；JSX 元素有屬性 `{E.M}` | `RecordLiteral` 的欄位、`ArgumentList` 的引數（具名或位置；涵蓋建構子呼叫與 `MapEntry(E.M, …)`）、`ListLiteral` 的元素是選擇子；`SetOrMapLiteral` 的字串 key 對應值是選擇子 | 同一個物件／陣列／參數列／record／map 裡的**其他成員**，但不進入巢狀的函式本體（箭頭函式、`function`、Dart `FunctionExpression`）；JSX 只看其他屬性的值與**直接的**文字子節點（`JsxText`、只含字串或識別字的 `JsxExpression`），不進入子元素 | `pair` |

配對涵蓋 Will 列的「`value: E.M` 與文字配對的集合字面值」，也涵蓋 `new Map([[E.M, "…"]])`、`(E.M, '…')`、`Option(value: E.M, label: '…')`、`<option value={E.M}>…</option>`。範圍收斂的理由：`f(E.M, () => {…大段 JSX…})`、`<X kind={E.M}>…整棵子樹…</X>` 不是對照表（審查 F12）；選擇子仍由條件決定，不是形狀黑名單。今天的 pair 受管命中在收斂前後相同［推論：四筆都是直接的屬性值或屬性字串，§2 #4、#5］。

### 1.3 兩端的實作差異

- 作用域（K4 的常數、K5 的別名共用）：TS 從使用處往上找 `Block`／函式本體／`SourceFile`，在使用處之前的 `VariableStatement`（`NodeFlags.Const`）。Dart 往上找 `Block`／`FunctionBody`／`CompilationUnit`，在使用處之前的 `VariableDeclarationStatement` 或 `TopLevelVariableDeclaration`（`isFinal || isConst`）。量測的原型用整個檔的名稱表，是上限。
- 常數的初始值：TS `StringLiteral`／`NoSubstitutionTemplateLiteral`／`TemplateExpression`；Dart `StringLiteral`。被 `PropertyAccess` 的名稱位置或宣告本身引用時不算。
- Dart 沒有 `satisfies`；TS 沒有 switch 運算式與 if-case。

### 1.4 失敗與回報

- 失敗：受管 enum 的任何 form 的命中。訊息：`<file>:<line>: E/<form> <enum>.<member> 成立時產生使用者面文字；這個 enum 的文字在共用目錄（…）或由伺服器擁有`。結尾提示沿用現有句子，加一句：「文字不是這個值的說法時，改成依契約的欄位或伺服器給的可用性分流」。
- `--report`：所有 enum 的 `case`、`key`、`pair` 命中（K8）；格式不變。
- R3 照舊，與 `E` 同一個命令列輸出。

### 1.5 誤報怎麼控制（量測）

- 不產生文字的比較不命中：H5 的 TS 148 行、Dart 37 行裡，新規則的受管命中只有 §2 的位置［量測］。
- 值號 0 不是選擇子（K2）：排除 12 筆 TS 受管命中，全部是 `<option value={E.UNSPECIFIED}>請選擇</option>` 一類［量測］。
- 只算正向（K3）：補集區域不算；`a && b` 為假、`a \|\| b` 為真不蘊含任何成員［量測］。
- pair 不進巢狀函式與子元素（§1.2）。
- 剩下的「依 enum 改版面、文字不是該值的說法」：§2 類 (b)(c)，修法由契約欄位或請求形狀決定。

### 1.6 已知不涵蓋（只做語法分析的極限）

- 包在函式、getter、類別欄位裡的條件（`isActive(m) ? "…" : …`、`bool get isPending => state == E.M`）；`let`／`var` 變數；先把 enum 對應成本地的值、再以本地值選文字；以 enum 值當索引的陣列或物件（`LABELS[m.kind]`，今天沒有受管實例，H12）。
- 這些是「沒有型別與資料流」的極限，不是形狀黑名單的漏洞。會發佈 en 的介面另有 I-CI（任何新增的中文字面值都失敗，不論形狀）；admin 不在 I-CI 內，所以 admin 的這些殘餘只靠 code review。觸發見 §9。
- 修正輪後仍不涵蓋的：跨檔的物件常數與 class 靜態欄位、`export … from` 的 re-export、namespace import；第二個 client 對只有一個 client 用的 enum，以條件形式寫出值的說法（跨端檢查只比對對照表，K8 的取捨，看不到條件形式）。Dart 的 dot shorthand（`.M`）要 Dart 3.10 才能寫，觸發：任一 Dart 專案的 SDK 下限提到 3.10 以上時，以成員名支援（buf 強制 `ENUM_VALUE_PREFIX`，成員名對得回唯一的 enum）。具名的成員集合（`const S = new Set([E.A]); S.has(k)`）、`export { U }` 的常數、同檔物件常數的屬性（`TEXT.key`）、解構參數的遮蔽已在修正輪補上。

## 2. 今天的命中與修法

原型（`eg/proto.mts`，正向版）在 `0a3265c1f` 對 website、admin、`packages/hawkr-api-ts/src` 的結果［量測：`ts-hits6.tsv`］；Dart 用 v1 版原型（`dart-hits3.tsv`）再依極性篩掉補集區域［推論：`:203,209` 是 `==` 的提早 return，補集］。buyer-app、所有套件 0 筆；單端 enum 的命中只回報，跨端 0。受管命中如下（行號是條件或成員的位置）。

| # | 位置 | enum.成員（form） | 內容 | 類 | 修法 | 刀 |
|---|---|---|---|---|---|---|
| 1 | `SA/features/seller/data/measure_display.dart:228,234` | `MeasureState.ACTIVE`、`ENDED`（if） | 三句停權效果 | (a) | K9：改呼叫 `sellerMeasureSuspensionActiveUntil(l, end:)`、`…ActiveOpen(l)`、`…Ended(l, end:)` | 2 |
| 2 | `AD/merchants/[id]/ActiveMeasuresCard.tsx:43,53` 標題 | `MeasureKind.TERMINATION`（ternary＋別名） | 「合作已終止」「賣家停權中」 | (a) | 改 `staffTextOrUnknown(sellerMeasureState(measure.state, {kind: measure.kind}))`（照契約的 state，不寫死 ACTIVE，審查 F9）：目錄的字是「合作已終止」「停權中」，「賣家停權中」→「停權中」已核可（§5 #4） | 2 |
| 3 | `AD/merchants/[id]/ActiveMeasuresCard.tsx` 內文、`AD/measures/[id]/MeasureSections.tsx:170` | `MeasureKind.TERMINATION`（ternary＋別名） | 終止與停權的欄位列不同（「生效時間」「結束時間」等） | (b) | Q1：欄位列依契約欄位有沒有值顯示（§10 Q1），刪 `appealDeadlineMissingText` 這種到不了的後備；兩個看得到的改變已核可（Will 2026-10-22） | 2 |
| 4 | `AD/merchants/[id]/KycApplicationCards.tsx:352,358` | `KYCField.ID_CARD_FRONT`、`ID_CARD_BACK`（pair） | `isBusiness ? "負責人身分證(正)" : "身分證(正)"`（反面同） | (a) | 改用目錄 `sellerKYCField(…)`：「身分證正面照」「身分證反面照」。「負責人」不會遺失：區塊標題已是「公司負責人身分資訊」／「賣家身分資訊」（H10）。改字已核可（§5 #5） | 2 |
| 5 | `AD/merchants/[id]/KycApplicationCards.tsx:467,502` | `KYCField.BANK_BOOK`、`BUSINESS_COMPANY_DOCUMENTS`（pair，JSX） | `alt="存摺封面"`；`alt={alt}`（區域常數 `` `文件 ${position + 1}` ``） | (a) | 存摺改用目錄，字相同；補充文件的 alt 改 `` `${補充文件} ${n}` ``（目錄名詞＋序號），alt「文件 1」→「補充文件 1」已核可（§5 #6） | 2 |
| 6 | `AD/merchants/[id]/measure.ts:576` | `TerminationGround.LINKED_ACCOUNT`（ternary，同檔常數 `TERMINATION_LINKED_TEXT`） | 「來源措施撤銷時，這筆會一併撤銷。賣家可對這筆單獨申覆。」 | (c) | 刀 1：句子併進伺服器給 LINKED_ACCOUNT 的 `explanation`（§2.1、§5 #13）；`terminationExplanation` 只串 `explanation` 與 `TERMINATION_COMMON_TEXT`，刪常數 | 1 |
| 7 | `AD/measures/[id]/MeasureSections.tsx:180` | `MeasureKind.TERMINATION`（and） | `<Field label="申覆期限">` | (b) | 條件改成 `measure.appealDeadlineTime &&`（H7）。畫面不變 | 2 |
| 8 | `AD/merchants/[id]/violation-measure.ts:140,192`；`AD/merchants/[id]/CreateMeasureDialog.tsx:158`（別名 `suspension`） | `MeasureKind.SUSPENSION` | 「請填寫結束時間。」、`throw "結束時間不是有效的時間"`、結束時間欄位 | (b) | 表單狀態改成契約的 oneof（H6）：`ViolationMeasureForm.measure` 是 `{ case: "warning" } \| { case: "suspension"; endTime: string }`，選單的值是 case 字串、標籤仍是目錄的 `sellerMeasureKind(MeasureKind.X)`；驗證、欄位顯示與建請求都看 `form.measure.case`，刪 `switch (form.kind)` 與「種類不是警告或停權」。從零做時表單就照請求的 oneof，不存一個要再轉換的 `kind`（沿用也是決策）。畫面不變［推論］ | 2 |
| 9 | `AD/violation-reports/cases/[id]/CaseSections.tsx:246` | `PolicyCategory.FOOD_SAFETY`（guard） | 食安案件才有「轉報主管機關」區塊 | (b) | Q3（由 SSOT 回答，不是待決）：區塊顯示條件改成 `kase.fdaReferral \|\| kase.fdaReferralWaiver \|\| kase.canRecordFdaReferral \|\| kase.canWaiveFdaReferral \|\| kase.canExportFda`（H8），不改契約。看得到的差別只有：伺服器說什麼都不能做、也沒有紀錄的食安案件，不再顯示空的區塊［推論］ | 2 |
| 10 | `AD/merchants/[id]/CreateTerminationDialog.tsx:199`；`AD/merchants/[id]/measure.ts:480-486` | `TerminationGround.MATERIAL_BREACH`、`LINKED_ACCOUNT`（and、if） | 重大違約才顯示「依據的認定」欄位與空清單句；「缺少來源措施。」「請選擇依據的認定。」 | (c) | 刀 1（§2.1）：欄位依 `option.violationRequirement` 顯示（`FORBIDDEN` 不顯示、`REQUIRED` 必填、`OPTIONAL` 選填），檢查是「`REQUIRED` 而沒有選」→「請選擇依據的認定。」；條件句（7 日）由 `explanation` 呈現，client 的 `noteText` 不再寫規則（§5 #14-#16）。表單狀態照請求的形狀並記住選項來自哪裡：`{ from: "available"; option; citedFindingIds } \| { from: "linked"; option; sourceMeasureId; }`，建請求時 `ground = option.ground`、`sourceMeasureId` 只在 `linked` 時帶，「缺少來源措施。」這個檢查不再存在（型別保證）。不再比較事由 | 1 |
| 11 | `SA/features/wallet/presentation/food_registration_form.dart:37`；`WS/features/seller-center/components/FoodRegSupplement.tsx:76` | `FoodRegistrationState.PENDING`（別名） | 審核中才顯示「撤回」按鈕 | (c) | 刀 1（§2.1）：「撤回」按鈕改看 `pendingFoodRegistration` 有沒有值。**只有按鈕改條件**：同一個區域裡的狀態行（目錄句，`food_registration_form.dart:110-131`、`FoodRegSupplement.tsx:208-226`）留在原本的位置，條件跟著改成同一個欄位有沒有值；被退回的區域改看 `rejectedFoodRegistration`。看得到的改變：沒有（錢包頁已以 `STOREFRONT_WRITE` 擋住，H14）［推論：website 頁內閘門沒有實跑］ | 1 |

v1 列的 website `measureDisplay.ts:136`（期間句）、`violation-measure.ts:190` 的 default、seller `measure_display.dart:203,209` 的 guard，在正向定義下不命中（K3）。`measure.ts:480` 仍在，因為 `:482` 的 `linked && …「缺少來源措施。」` 是正向［量測：原型把命中記在別名宣告那一行］，與 #10 一起由刀 1 解決。

### 2.1 刀 1 的契約與伺服器（開工前審查 F1-F5）

**終止合作（Q2(b)）**
- 規則：`SM/model/measure.go` 加 `func (g TerminationGround) ViolationRequirement() TerminationViolationRequirement`：`ContractOnly()`（NOTICE、LINKED_ACCOUNT）→ `FORBIDDEN`；`GroundMaterialBreach` → `REQUIRED`；其他 21.3 事由 → `OPTIONAL`。`create.go:147-156` 的兩個 case 改成依這個方法判斷（`FORBIDDEN` 而有 violation → 現有的錯誤；`REQUIRED` 而沒有引用 → 現有的錯誤），錯誤句不變。7 日仍在有狀態的檢查（H15），它的天數也由 `explanation` 的參數取自 `model.MaterialBreachCure`，同一個常數。
- 等價測試 `TestTerminationViolationRequirementIsTheSameRuleAsCreateMeasure`（`SM/measure`，照 seller `AGENTS.md:70` 的先例）：每個事由 ×（沒有 violation／violation 只有新認定／violation 有引用）跑 CreateMeasure 的無狀態驗證，斷言「通過 ⇔ 需求允許這種輸入」。
- 契約 `api/hawkr/admin/seller/v1alpha1/measure.proto`：
  - `enum TerminationViolationRequirement { TERMINATION_VIOLATION_REQUIREMENT_UNSPECIFIED = 0; TERMINATION_VIOLATION_REQUIREMENT_REQUIRED = 1; TERMINATION_VIOLATION_REQUIREMENT_OPTIONAL = 2; TERMINATION_VIOLATION_REQUIREMENT_FORBIDDEN = 3; }`（型別前綴與零值照 `API 設計規範（Connect）.md:102`）。註解：「建立這個事由的終止時，TerminationInput.violation 的需求；與 CreateMeasure 的驗證是同一個規則（model.TerminationGround.ViolationRequirement）。REQUIRED 時要引用至少一筆認定，條件寫在 explanation。client 只依它顯示與檢查欄位，不顯示它的文字。」
  - `TerminationOption` 加 `TerminationViolationRequirement violation_requirement = 4 [(google.api.field_behavior) = OUTPUT_ONLY];`（名稱對齊請求的 `termination.violation`）。`explanation`（3）的註解改成「生效時機與這個事由的建立條件（給員工）」。
- 句子：`model.TerminationEffect.Message` 改成依（生效時機, 事由）回一則完整的訊息，不拼接（ADR D2 禁止拼接）：預告終止、一般立即生效沿用現有兩句；重大違約與關聯帳號各一則新句（§5 #12、#13），放在 `SM/text/zh-TW.json`（模組句子，`make uitext` 生成）。`TerminationEffect` 已帶 `Ground`，不必改呼叫端。
- `toTerminationOption`（httpapi）設 `violation_requirement`。這個 enum 是 admin package 的新 enum，`make uitext` 會把它寫進 TS manifest（`inCatalog: false`、`serverOwned: false`，不受守門管；admin enum 不進 Dart manifest）［推論，開工前審查 F6］。

**食品業者登錄（Q4(A)）**
- 契約 `api/hawkr/seller/v1alpha1/seller.proto` 的 `Store`：`reserved 27; reserved "open_food_registration";`；加 `FoodRegistration pending_food_registration = 29 [(google.api.field_behavior) = OUTPUT_ONLY];`（「待審的登錄；沒有時不設。有值時可以撤回。」）與 `FoodRegistration rejected_food_registration = 30 [(google.api.field_behavior) = OUTPUT_ONLY];`（「被退回、之後沒有再送出的登錄；沒有時不設。」）。同檔 `:75` 的 RPC 註解與 `:373` 的 `FoodRegistration` 註解改成新欄位名。`v1alpha1` 上線前不檢查 breaking（`api/buf.yaml`）。
- 伺服器：`backend/internal/httpapi/seller/app.go` 的 `toStore`（`:119`）依 `s.OpenFoodRegistration` 的狀態放進其中一個欄位；模組的 `model.Store` 不變（admin 仍讀它）。
- admin 契約的 `Store.open_food_registration`（30）不改：admin 不在 §2 的命中裡，它把未結束的那一筆當一列顯示；不在本刀。
- client：seller-app `core/session/my_store.dart` 與 website `lib/seller` 的讀取改成兩個欄位；§2 #11。
- 刀 1 結束時跑 `make -C backend uitext`（manifest 加 `TerminationViolationRequirement`、模組句子），否則 `ui-text-generated-diff.yml` 紅（F6）。

## 3. 共用文案目錄（刀 2）

`C/zh-TW.json` 的 `default` 加（description 手寫，`-write` 不動）：

| key | message | params | description |
|---|---|---|---|
| `seller.Measure.suspension_active_until` | `停權中（至 {end}）` | `end: string` | 停權生效中、有結束時間時的效果行（seller-app 措施列表）。end 是台北時間（YYYY/MM/DD HH:mm）。 |
| `seller.Measure.suspension_active_open` | `停權中（未定結束時間，平台定期複審）` | — | 停權生效中、沒有結束時間時的效果行（由平台定期複審，不是永久）。 |
| `seller.Measure.suspension_ended` | `已於 {end} 結束` | `end: string` | 有結束時間的停權已結束時的效果行。end 同上。 |

`C/en.json` 加三個的 en（§5，Will 2026-10-22 已核可）。
admin 關聯帳號終止的說明句不進目錄（§2 #6），所以 v1.2 的 `admin.seller` key 前綴風險不存在。
術語表 `backend/docs/開發規範/使用者面用語術語表.md` 加一列：「定期複審｜periodic review｜沒有固定結束時間的停權，由平台定期檢查是否繼續｜賣家端措施效果句、admin 複審期限」（H13；新詞先進術語表，en 句引用它）。

## 4. 文件

### 4.1 ADR D2 文末修訂（刀 2；Will 2026-10-22 已核可，v1.3 依 F4 決定縮小第一批）

加在 ADR 文末（與 D4 的修訂並列；不動另一個 session 未提交的 D4 修改）：

> **修訂：目錄分界（2026-10-22，Will）**
> - D2「分界不變」改成：只由契約值決定，而且（兩個以上 client 用到，**或**只在某個目錄 enum 成員成立時才出現）的文字進目錄，加上 Connect code 的預設錯誤文字；其他文字在各 client 的資源。
> - 「只在某個目錄 enum 成員成立時才出現」：client 程式裡，某段文字所在的分支或資料，必須先確定某個欄位等於目錄 enum 的某個成員才會到達（`if (state == ACTIVE)`、`case ACTIVE`、`{ACTIVE: …}`、`{value: ACTIVE, label: …}`）。這種句子即使只有一個 client 用，也不放 client 資源：它是那個值的說法，放在 client 就和目錄的名詞成為兩個來源。補集（「不是 ACTIVE 時」）不算。
> - 理由：依條件判斷的守門（`docs/plans/2026-09-29-a3-buyer-seller/enum-text-guard-design.md`）在 I-app-0 之後連 client 資源查詢也算成文字，這類句子在 client 沒有合法的地方放。
> - 第一批：seller-app 停權效果句三句（`seller.Measure.suspension_*`）。website 的停權期間句不屬於這一類（終止合作提早 return 之後的補集），照 L′ §7(d) 留在 client。

L′ 文末加修訂：「§7(d)：seller-app 的三句（停權中（至 {end}）等）自本刀起由目錄提供（ADR D2 修訂）；website 的期間句不變。§5.2 的 R1／R2／D1／D2 由規則 `E` 取代（本文）；R6 的三元式漏網已補。」

### 4.2 其他文件

- 根 `AGENTS.md` 共用文案目錄列（刀 2，照 ADR）：定義改成「只由契約值決定，而且兩個以上 client 用到、或只在某個目錄 enum 成員成立時才出現的使用者面文字，加上 Connect code 的預設錯誤文字」；守門描述（刀 3）：「client 不得在由目錄或伺服器擁有的 enum 成員確定成立的分支或資料裡產生使用者面文字（`check-enum-text`、`check_enum_text`），同一個 enum 的對照表也不得在兩個以上 client 各寫一份（`scripts/check-enum-text-cross.sh`）」。
- 兩個工具的檔頭（刀 3）：規則段改寫成 §1 的摘要；「已知漏網：三元式…由 I-CI 補」刪除，改寫「不涵蓋」一段（§1.6）並指向本文。
- `scripts/check-enum-text-cross.sh` 檔頭（刀 3）：「那些規則在各 client 遷移到目錄的 commit 才接上」已不成立（四個 client 都接上了，`admin/package.json:10`、`_flutter-quality.yml:65-70`），改成現況；說明 report 只含對應形式（K8）。
- `seller-app/AGENTS.md:9`、`buyer-app/AGENTS.md:52`、`admin/AGENTS.md:21` 的「手寫文字表」說法改成規則 `E` 的說法（刀 3）。
- `measure_display.dart:180` 的註解「停權的效果句兩端不同，留在 client」改成「兩端不同；App 的效果句是目錄的 `seller.Measure.suspension_*`」（刀 2）。

## 5. 文案表（A＝已核准、C＝跟隨契約或 SSOT、E＝沿用、N＝新寫或改字，要 Will 核可（現在全部已核可））

| # | 位置 | 改前 → 改後 | 類 |
|---|---|---|---|
| 1 | seller-app 措施效果行 `suspension_active_until` | zh 不變；en（新）**Suspended until {end}** | zh E；en A（Will 2026-10-22） |
| 2 | `suspension_active_open` | zh 不變；en **Suspended (no end date; under periodic review by the platform)** | zh E；en A（Will 2026-10-22） |
| 3 | `suspension_ended` | zh 不變；en **Ended on {end}** | zh E；en A（Will 2026-10-22） |
| 4 | admin 生效中措施卡標題（§2 #2） | 「賣家停權中」→**「停權中」**（目錄字）；「合作已終止」不變 | A（Will 2026-10-22） |
| 5 | admin KYC 身分證照片標籤（§2 #4） | 「負責人身分證(正)」「身分證(正)」→**「身分證正面照」**；反面同 →**「身分證反面照」**（「負責人」由區塊標題表達） | A（Will 2026-10-22） |
| 6 | admin 補充文件圖片的 `alt`（§2 #5） | 「文件 {n}」→**「補充文件 {n}」** | A（Will 2026-10-22） |
| 7 | 術語表新列「定期複審｜periodic review」（§3） | 新列 | A（Will 2026-10-22） |
| 8 | admin 存摺 `alt`（§2 #5） | 不變，改由目錄提供 | E |
| 9 | §2 #7、#8 | 沒有看得到的改變 | C |
| 10 | §2 #9（Q3） | 伺服器什麼都不能做、也沒有紀錄的食安案件不再顯示空區塊 | C（由 SSOT 推出） |
| 11 | §2 #3（Q1）、#10（Q2）、#11（Q4） | Q1 見 §10；Q2 的欄位改依三態顯示（其他 21.3 事由出現選填欄位，Will 2026-10-22 已核可）；Q4 沒有看得到的改變 | A（Will 2026-10-22） |
| 12 | 伺服器 `TerminationOption.explanation`：重大違約（`SM/text/zh-TW.json` 新 key `termination.effect.immediate_material_breach`，`days` 取自 `model.MaterialBreachCure`） | 改前：「立即生效：店家暫停營業。」＋ client `noteText` 的 7 日規則 → 改後：**「{days, plural, other {立即生效：店家暫停營業。必須引用至少一筆未撤銷、已存在 {days} 日以上的認定。}}」**（顯示為「…已存在 7 日以上的認定。」） | A（Will 2026-10-22） |
| 13 | 同上：關聯帳號（新 key `termination.effect.immediate_linked_account`） | 改前：「立即生效：店家暫停營業。」＋ client 常數 → 改後：**「立即生效：店家暫停營業。來源措施撤銷時，這筆會一併撤銷。賣家可對這筆單獨申覆。」**（後兩句與現在的 client 常數同字，合成一則訊息） | A（Will 2026-10-22） |
| 14 | admin 終止表單「依據的認定」沒有可選的認定（`REQUIRED`） | 「這個帳號沒有生效中的認定，無法以重大違約終止。」→ **「這個帳號沒有生效中的認定，無法以這個事由終止。」** | A（Will 2026-10-22） |
| 15 | 同上（`OPTIONAL`，新） | **「這個帳號沒有生效中的認定。」** | A（Will 2026-10-22） |
| 16 | 同欄位的說明 `noteText` | `REQUIRED`：「至少選一筆；引用的認定必須未撤銷、已存在 7 日以上。」→ **「至少選一筆。」**（條件改由 #12 的說明句呈現）；`OPTIONAL`（新）：**「選填。」** | A（Will 2026-10-22） |

en 的日期仍是 `YYYY/MM/DD HH:mm`（台北時間），與現有的終止句相同（§0.2）。`MEASURE_STATE_ACTIVE` 目前沒有 en；之後補它時用 `Suspended`／`Partnership terminated`（後者是術語表已定），與 #1、#2 一致［審查 F13］。en 沒有發佈（`publishedLocales` 只有 zh-TW），寫進 `en.json` 只影響覆蓋率與 golden（5d 附錄 B）。

## 6. 測試（突變寫明測試要抓的改壞方式）

### 6.1 守門工具（刀 3；`DT/test/check_enum_text_test.dart`、`TT/tests/check-enum-text.test.ts`）

每一種 form 一組：**正例**（必須命中，斷言 `E:<form>:<enum>.<member>@<line>`）＋**反例**（同形狀但區域沒有漢字、或 enum 不是 api 生成的、或成員是值號 0、或區域是補集，必須不命中）。兩端各自寫，形狀照該語言。

| 組 | TS 正例 | Dart 正例 | 突變（改壞工具，該組正例或反例要變紅） |
|---|---|---|---|
| if／else | `if (k === E.M) return "警告";`；`if (k !== E.M) {} else { t = "警告"; }`（else） | 同；加 if-case `if (k case E.M) …`、集合 `[if (k == E.M) Text('警告')]` | 拿掉 if 的處理 → 正例紅；else 不算 → else 正例紅 |
| 補集不算 | `k === E.M ? null : "其他"`、`if (k === E.M) {} else { t = "其他"; }`、`switch (k) { case E.M: return null; default: return "其他"; }` **都不命中** | 同（`_ => '其他'`） | 補集也算（v1 行為）→ 反例紅 |
| 蘊含 | `if (k === E.M && ok) {} else { t = "其他"; }` 不命中；`if (k !== E.M \|\| ok) {} else { t = "警告"; }` 命中；`if (!(k !== E.M)) t = "警告";` 命中 | 同 | `&&` 為假也蘊含 → 反例紅；`!` 不反轉 → 正例紅 |
| guard | `if (k !== E.M) return x; return "警告";` 命中；`if (k === E.M) return x; return "其他";` 不命中 | 同，含 `throw` 結尾 | 拿掉 guard → 正例紅；then 沒有離開也算 → 反例（then 不 return）紅 |
| 三元式 | `k === E.M ? "警告" : ""`、`k == E.M ? …`、`E.M !== k ? "" : "警告"` | 同 | 拿掉三元式 → 紅；只看 `===` 不看 `==`／`!==` → 紅 |
| 短路 | `k === E.M && "警告"`、`k !== E.M \|\| "警告"`、JSX `{k === E.M && <b>警告</b>}` | `k == E.M && f('警告')` | 拿掉 `&&` 或 `\|\|` → 紅 |
| 集合成員 | `[E.A, E.B].includes(k) ? "限制" : ""`、`new Set([E.A]).has(k) && "限制"`；`![E.A].includes(k) ? "限制" : ""` 不命中 | `const {E.A}.contains(k) ? '限制' : ''` | 拿掉成員檢查 → 紅；為假也回成員 → 反例紅 |
| switch | 沿用舊 R2 三筆＋`case` 裡 `throw new Error("限制")`＋`switch (true) { case k === E.M: … }` | 沿用舊 D1 三筆＋object 樣式 `Measure(kind: E.M) => '警告'`、relational `== E.M`、`when k == E.M` | 回到「只看 return 與賦值」→ `throw` 那筆紅；不遞迴樣式 → object 樣式紅 |
| key | 沿用舊 R1 | 沿用舊 D2 | 拿掉 → 紅 |
| pair | `{ value: E.M, label: "警告" }`、`[[E.M, "警告"]]`、`f(E.M, "警告")`、`<option value={E.M}>警告</option>`；**不命中**：`f(E.M, () => "警告")`、`<X kind={E.M}><p>警告</p></X>` | `(value: E.M, label: '警告')`、`Option(value: E.M, label: '警告')`、`MapEntry(E.M, '警告')`、`{'value': E.M, 'label': '警告'}`；不命中：`f(E.M, () => '警告')` | 每一種配對各自拿掉 → 對應正例紅；進入巢狀函式或子元素 → 反例紅 |
| 值號 0 | `<option value={E.UNSPECIFIED}>請選擇</option>` 不命中 | `f(E.MEASURE_KIND_UNSPECIFIED, '請選擇')` 不命中 | 拿掉排除 → 反例紅 |
| 別名 | `const t = k === E.M; t ? "警告" : ""`；`const p = k === E.M ? o : null; p && "撤回"`；`let t = …` 不命中 | `final t = k == E.M; …`；`var` 不命中 | 拿掉別名 → 正例紅；`let`／`var` 也代入 → 反例紅 |
| 跨檔常數 | 檔 A `export const T = "警告"`，檔 B `import { T } from "./a"; k === E.M ? T : ""` 命中；import 的是不含漢字的常數、或 enum 不是 api 生成的，不命中；Dart 同（`import` 的頂層 `const`／`final`） | 同 | 不解析 import → 正例紅；把任何匯入的識別字都當常數 → 反例紅 |
| 常數 | 頂層 `const T = "警告"; k === E.M ? T : ""`；函式內 `const t = "警告"; return k === E.M ? t : "";` | 同（頂層與區域 `final`） | 拿掉常數追蹤 → 紅；只追頂層 → 函式內那筆紅 |
| 受管與 report | 單端 enum 的 ternary 命中**不在** report、`forbidden` 為空；單端的 key 在 report；`serverOwned` 的 ternary 被擋 | 同 | 條件形式也寫進 report → 紅；拿掉 `serverOwned` → 紅 |

舊測試的「已知漏網：三元式不擋」（Dart `:89`、TS `:158`）改成「三元式要擋」；`D1:`、`D2:`、`R1`、`R2` 的期望改成 `E:<form>`。目錄 enum 數的兩個斷言（Dart `:173`、TS `format.test.ts:189`）仍是 17。

### 6.2 目錄（刀 2）

golden 自動含新 key 的 zh-TW 與 en，三端各自斷言。**突變**：手改 `zh-TW.json` 一個字不重新生成 → `ui-text-generated-diff.yml` 紅；`en.json` 的 `{end}` 打錯成 `{date}` → 生成失敗（參數集合必須相同，L′ §1.3）。

### 6.3 clients（刀 2）

- seller-app `test/features/seller/data/measure_display_test.dart`：既有四個斷言（`:40,49,56,64`）不變（字相同），另加 `UiLocale.en` 三句的英文。**突變**：`measureEffect` 改回寫死中文 → en 斷言紅（刀 3 之後守門也紅）。
- admin：生效中卡標題依 `measure.state`（ACTIVE 終止／停權各一筆，SCHEDULED 一筆確認不寫死 ACTIVE）；KYC 標籤；`MeasureSections` 依欄位有無的列（含 H9 兩種情形）；violation 表單（oneof：警告送 `warning`、停權送 `suspension` 與結束時間、停權沒有結束時間時「請填寫結束時間。」）；案件轉報區塊（沒有 can_*、沒有紀錄時不顯示；`canRecordFdaReferral` 為真時顯示）。**突變**：標題改回 `MeasureState.ACTIVE` → SCHEDULED 那筆紅；表單 case 對錯 → 建請求測試紅；申覆期限條件改回只看種類 → 「終止但沒有期限」的測試紅；轉報區塊改回看類別 → 「食安但沒有 can_* 與紀錄」紅。

### 6.4 契約（刀 1）

- seller 模組：`TestTerminationViolationRequirementIsTheSameRuleAsCreateMeasure`（§2.1）；`TerminationEffect.Message` 對四種（預告、一般立即、重大違約、關聯帳號）各一筆，重大違約的 `days` 等於 `MaterialBreachCure` 的天數。**突變**：`ViolationRequirement` 把某個 21.3 事由改成 `REQUIRED`，或 `create.go` 改回自己的 switch 而規則不同 → 等價測試紅；`MaterialBreachCure` 改成 14 日而句子寫死 7 → 句子測試紅。
- httpapi：`available_terminations` 與 `LinkedAccount.termination_option` 帶 `violation_requirement` 與新的 `explanation`；`toStore` 依狀態放進 `pending_`／`rejected_food_registration`（待審、被退回、沒有三種情形）。**突變**：被退回放進 pending → 測試紅。
- admin 終止表單：三態各一筆（`FORBIDDEN` 不顯示欄位、`REQUIRED` 沒選時「請選擇依據的認定。」、`OPTIONAL` 不選也能送出）；關聯帳號的選項送出時帶 `sourceMeasureId`；說明是 `explanation` 加共同句。**突變**：改回比較 `TerminationGround.MATERIAL_BREACH` → `OPTIONAL` 那筆紅。
- seller-app、website：有 pending 時顯示撤回與狀態行、有 rejected 時顯示被退回的狀態、兩個都沒有時都不顯示。**突變**：撤回改看 rejected → 紅。
- `api-contract.yml`（`buf lint`、`buf breaking`、生成物 drift）；`ui-text-generated-diff.yml`（manifest 與模組句子）。

## 7. Commit 計畫

| 刀 | commit | 內容 | 審查 |
|---|---|---|---|
| 1 | `feat(api)`（契約＋所有消費端同一個 commit，根 `AGENTS.md`「改契約時…同一次變更」） | Q2(b)、Q4(A)（§2.1）：proto、seller 模組（`ViolationRequirement`、`TerminationEffect.Message`、`text/zh-TW.json` 兩個新 key）與 httpapi、`make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 生成、`make -C backend uitext`（manifest 與模組句子）；admin 終止表單（§2 #6、#10）、seller-app 與 website 食品業者登錄（§2 #11）。§5 #12-#16 已核可（Will 2026-10-22） | code-review；走 feature 流程 |
| 2 | `docs(backend)` | ADR D2 文末修訂（§4.1）、L′ 文末修訂、術語表一列、根 `AGENTS.md` 目錄定義 | §4.1 已核可（Will 2026-10-22） |
| 2 | `feat(uitext)` | 目錄 3 個 key（zh-TW 與 en，已核可）與 `make uitext` 生成物；§2 #1-#9 的 client 修法 | code-review |
| 3 | `feat(ui-text)` | 兩個工具（規則 `E`、刪 D1／D2／R1／R2、檔頭）與測試；跨端腳本檔頭；各 client `AGENTS.md` 與根 `AGENTS.md` 的守門描述。前兩刀落地後命中是 0，這個 commit 的 CI 是綠的 | code-review |
| 3 | `docs(backend)` | 計畫 `:383`、`:399` 標完成並記錄範圍；本文的實作修訂；seller CARRYOVER 沿用盤點補一列「停權效果句寫在 client｜兩端資訊設計不同｜成立，但只在目錄 enum 成員成立時出現的句子由目錄提供（ADR D2 修訂）｜進目錄」 | 併入完成前的 design-review |

刀 2 依賴刀 1 的只有 §2 #10（終止表單）；若刀 1 延後，刀 2 先做其他項，刀 3 等兩刀都完成。

## 8. 驗證命令（派工前照當時的 `.github/workflows/` 再核對）

- 刀 1：`make -C backend uitext` 後 `ui-text-generated-diff.yml` 綠；`api-contract.yml`（`buf lint`、`buf breaking`、兩個生成套件的 drift 與測試）；`backend-monolith.yml`（gofmt、`go vet`（含 `-tags integration`）、`go test ./internal/...`、`make lint-monolith`、`GOWORK=off go mod tidy` 無 diff）；`admin-ci.yml`、`website-ci.yml`、`seller-ci.yml`。
- 刀 2：`make -C backend uitext`（`backend/Makefile:364-366`），覆蓋率輸出貼進驗證紀錄；`ui-text-generated-diff.yml`；`go test ./internal/platform/uitext/...`；`admin-ci.yml`（`pnpm lint`、`test`、`build`）、`seller-ci.yml`（`_flutter-quality.yml`）。
- 刀 3：`ts-packages.yml`（`pnpm --filter @hawkr/ui-text lint`、`test`）；`dart-packages.yml`（`DT` 的 `flutter pub get`、`dart analyze`、`dart format --output=none --set-exit-if-changed lib test tool`、`dart test`）；守門本身：`TT` 的 `pnpm exec tsx tools/check-enum-text.ts ../../website`、`../../admin`、`../hawkr-api-ts/src`，`DT` 的 `dart run tool/check_enum_text.dart ../../seller-app/lib`、`../../buyer-app/lib`、三個 Dart 套件，`bash scripts/check-enum-text-cross.sh`（`ui-text-guard.yml`），全部 exit 0；四個 client 的 CI。
- **反向驗證**（刀 3）：在暫存副本把 `measure_display.dart` 的修法還原，Dart 工具必須 exit 1 並點名 `E/if`；把 `CaseSections.tsx` 的條件還原，TS 工具必須點名 `E/guard`。
- 看 CI 時看每個 job 自己的結論。

## 9. 沒有驗證的部分

- §2 的命中來自原型，不是最終工具；Dart 的數字由 v1 原型依極性推論（§2 開頭）。刀 3 以最終工具重跑，數字不同就更新本文。 **刀 3 已以最終工具重跑**：受管命中 0，單端與跨端的數字見「實作時的修訂（刀 3）」#8，與原型一致。
- §2 #11「沒有看得到的改變」：seller-app 的路由閘門讀過碼；website 錢包頁的頁內閘門行為沒有實跑（H14）。
- §2.1 manifest 的變動（`TerminationViolationRequirement` 只進 TS manifest）是讀生成器推得，刀 1 跑 `make uitext` 時確認。
- §2 #8「畫面不變」、#9「只少了空區塊」是讀碼推論。
- §1.6 的殘餘只量了 `LABELS[m.kind]` 一種（H12）。**觸發**：code-review 或 I-CI 抓到一個以 §1.6 形式寫的目錄 enum 文字，就評估改用型別分析（TS program＋type checker、Dart resolved AST）；查詢加進 K4 的時間見 §0.2（website 與 I-CI 的 website 那一刀、App 與 I-app-0，兩者都已是計畫項目）。 跨檔匯入的常數的觸發已成立（刀 2 完成前審查 F1），改收進 K4 與刀 3 的測試（§6.1），不等型別分析。

## 實作時的修訂（刀 1，2026-10-22）

| # | 設計的寫法 | 實作 | 原因 |
|---|---|---|---|
| 1 | §2.1：等價測試斷言「通過 ⇔ 需求允許這種輸入」 | `TestTerminationViolationRequirementIsTheSameRuleAsCreateMeasure` 除了等價比對，另釘住每個事由的需求（獨立於實作的預期表，十一個事由） | `validateTermination` 與 API 都讀同一個 `ViolationRequirement()`，等價比對在「規則本身改錯」時兩邊一起變、不會紅；§6.4 的突變「把某個 21.3 事由改成 `REQUIRED`」要靠預期表才抓得到。`create.go` 改回自己的 switch 且規則不同，則由等價比對抓到（兩個突變都實際跑過，見驗證紀錄） |
| 2 | §2 #10：表單狀態 `{ from: "available"; option; citedFindingIds } \| { from: "linked"; option; sourceMeasureId }` | 照做：`TerminationChoice`，`TerminationForm.choice` 為 `undefined` 表示還沒選；`buildCreateTermination` 與 `terminationRequestKey` 收 `ReadyTerminationForm`（`choice` 必有）。下拉與說明、欄位全由選項決定：`CreateTerminationDialog` 的 `availableGrounds` 與 `effects` 兩個 props 合成一個 `options`，`terminationAvailability` 回 `{ canCreate, options }`（刪 `grounds`、`effects`），`availableTerminationGroundOptions` 改成 `availableTerminationOptions`（回整筆選項），`validateTermination` 刪 `selectable` 參數 | 一個選項同時帶事由、說明句與依據的需求，拆成平行的 props 就是 v1.2 F4 要消除的平行機制；`citedFindingsFieldFor(option)` 把 §5 #14-#16 的三組文字集中成一個函式（需求為 `FORBIDDEN` 與不認得的值不顯示欄位）。選填時殘留的勾選不送：請求只在需求不是 `FORBIDDEN`、且有勾選時帶 `violation` |
| 3 | §2.1：`TerminationEffect.Message` 依（生效時機, 事由）回一則完整訊息 | key 是 `termination.effect.immediate_material_breach`（`days` integer＋plural，取自 `MaterialBreachCure`）與 `termination.effect.immediate_linked_account`，原文照 §5 #12、#13；`termination.effect.immediate`（一般立即生效）與 `.scheduled` 不變 | 如設計 |
| 4 | §2.1：`Store` 加 `reserved 27; reserved "open_food_registration";` | 併進 `Store` 開頭既有的 `reserved` 兩行（編號與名稱），不另開兩個 `reserved` 敘述 | 同一個 message 的 reserved 寫在一處；語意相同 |
| 5 | §2.1：client 讀兩個欄位 | seller-app `MyStore` 的 `openFoodRegistration` getter 換成 `pendingFoodRegistration` 與 `rejectedFoodRegistration`；website 直接讀兩個欄位。兩端的狀態行與撤回按鈕條件都是「欄位有沒有值」，不再 import `FoodRegistrationState`（seller-app 表單、website 元件）。測試的假伺服器（`wallet_screen_test.dart`）仍以一個 `foodRequest` 變數描述情境，由它依狀態放進兩個欄位，模擬伺服器的對應 | 設計 §2 #11 |
| 6 | §2.1 與 §9：admin 的 `TerminationViolationRequirement` 只進 TS manifest（`inCatalog: false`、`serverOwned: false`） | `make uitext` 後只有 `packages/hawkr-ui-text-ts/src/enum-manifest.gen.json` 多一筆，Dart manifest 與 golden 沒有變；模組句子只有 `text/zh-TW.json` 與 `text_gen.go` 的變動 | §9 的推論已由生成結果確認 |
| 7 | 刀 1 的文件 | 更新 seller 模組 `AGENTS.md` 的終止合作可用性段與食品業者登錄段、`CARRYOVER.md` 的 `TerminationOption` 列與食品登錄狀態模型列（記沿用盤點：client 自寫的條件改由伺服器推導）、詳細設計書 18 §10 的契約形狀句（`open_food_registration` 拆成兩個欄位） | 同一個變更內更新所有說到這個契約的文件 |
| 8 | §2.1：「7 日仍在有狀態的檢查」 | 加 `TerminationGround.CitedFindingMinAge()`（`REQUIRED` 的事由是 `MaterialBreachCure`，其他 0），與 `ViolationRequirement` 同一個規則的兩面；`checkTermination`、`TerminationEffect.Message` 的天數與兩則錯誤句（`create.go`，原寫死「七日」，現為「已存在 N 日以上」）都讀它，另有等價與天數測試 | 完成前 design-review：「引用的認定須存在多久」原本在三處各自以 `Ground == GroundMaterialBreach` 判斷，錯誤句的天數沒連到常數 |
| 9 | 刀 1 未列：單元層級的需求值對照 | `model.TerminationViolationRequirementValues()`（經 seller 公開套件匯出）加 `TestTerminationViolationRequirementMatchesTheContract`（`rpc.EnumMismatches`，並確認每個事由的選項都不是 UNSPECIFIED） | code review：model 字串改名時 `EnumToProto` 靜默回 UNSPECIFIED，client 會隱藏欄位，原本只有 integration 測試抓得到 |
| 10 | §2 #10：下拉的事由清單 | 刪 admin 的 `TERMINATION_GROUND_OPTIONS`：下拉照伺服器給的 `available_terminations` 的順序與內容，只丟掉沒有顯示文字的事由（`availableTerminationOptions`）。契約註解（admin `seller.proto` 的 `available_terminations`、`governance.proto` 的 `TerminationInput.violation`）不再重述規則，改指向 `violation_requirement` 與 `explanation`。`validateTermination` 回傳 `{ error } \| { form }`（通過時是已選到終止合作的輸入），對話框不再重複檢查「請選擇終止事由。」 | 該清單是伺服器規則（排序、排除 `LINKED_ACCOUNT`）在 client 的副本；契約同一條規則不該有三種寫法 |
| 11 | 刀 1 未列：admin 契約 `Store.open_food_registration`（30） | 維持合併欄位，理由與重新評估的觸發記進 seller `CARRYOVER.md`：admin 把它顯示成一列、用目錄的狀態名詞當標籤，店家頁沒有對它的操作；觸發是「admin 店家頁要對待審或被退回的登錄提供操作時」 | design-review：原理由「admin 不在 §2 的命中裡」是守門的理由，不是設計的理由 |
| 12 | 運維文件 | `seller-violation-management-runbook.md` 的終止事由表：停業、解散、破產、許可撤銷與其他 21.3 事由一樣是「依據的認定選填」。Will 的 controller 決定：契約與詳細設計書 32 是 SSOT（`OPTIONAL`），runbook 照改 | 刀 1 讓 admin 對這四個事由也出現選填欄位（§5 #11 已核可），runbook 原寫「契約條款（沒有認定）」與契約不符 |

## 實作時的修訂（刀 2，2026-10-22）

| # | 設計的寫法 | 實作 | 原因 |
|---|---|---|---|
| 1 | §2 #3：欄位列依契約欄位有沒有值顯示；刪 `appealDeadlineMissingText` | 照做：`MeasureContentSection` 四列（生效時間、結束時間、複審期限、申覆期限）各看自己的欄位；生效中措施卡加「生效時間」列、申覆期限只在有值時顯示，`appealDeadlineMissingText` 與卡片的「申覆期限：-」占位刪除。卡片另保留既有的「結束時間：未定（由平台定期複審）」，條件是沒有結束時間而有複審期限（既有測試守的行為，也是欄位有沒有值） | 設計只寫「欄位列依欄位」；卡片這句是已核可畫面的一部分，不屬於兩個已接受的看得到的改變 |
| 2 | §2 #2：卡片內文不再分終止與停權 | （修正輪取代）原本保留 `isViolation = kind !== TERMINATION && basis === VIOLATION`，理由「條件裡沒有文字」不成立：真分支的文字是 `store-detail.ts` 的 `VIOLATION_MEASURE_TEXT`，只是原型看不到跨檔常數（完成前審查 F1）。修正輪 A：刪這個分支與常數，句子改由伺服器的 `end_unavailabilities` 提供（#8） | 審查 F1 |
| 3 | §2 #9：區塊顯示條件列五個欄位 | 加第六個 `canRecordFdaReferralReply`（H8 列出的 can_*，區塊裡有「記下主管機關的回覆」按鈕只看它） | 否則只有它為真的案件（測試 fixture 以外，伺服器在有轉報紀錄時才給）區塊會消失；與「伺服器允許任何動作就顯示」的意思一致 |
| 4 | §2 #8：`ViolationMeasureForm.measure` 是 oneof 形狀 | 照做，另外 `CreateMeasureInitial` 也改成 `measure?: {case:"warning"} \| {case:"suspension"; endTime?}`（取代 `kind`、`endTime`）；種類選單的值是 case 字串（`""` 是沒選），`VIOLATION_MEASURE_KIND_OPTIONS` 改成 `VIOLATION_MEASURE_OPTIONS`（`{case, kind}`，標籤取目錄的 `kind`） | 預填與表單同一個形狀，不另存一個要轉換的 `kind`；呼叫端只有發票申訴頁，它走 `measureInitialFromSuggestion` |
| 5 | §6.3：admin 測試 | 新增 `measure-content-section.test.ts`、卡片標題與時間列、KYC 標籤測試；既有的 `case-detail-page`、`case-inquiry-page` 測試原本以「沒有 can_* 也沒有紀錄的食安案件有區塊」為前提，改成給 `canRecordFdaReferral`，並新增「食安但什麼都不能做、也沒有紀錄不顯示」 | 設計 §2 #9 的看得到的差別 |
| 6 | §2 #1-#9 命中歸零 | 以 `eg/proto.mts`（TS，website、admin、`hawkr-api-ts/src`）與 `eg/dartproto/bin/proto.dart`（seller-app、buyer-app）重跑：受管 enum（`CAT`）命中 0；TS 只剩單端 enum 的 160 筆（admin）加 1 筆（website `CertificationsClient.tsx:1145` 的 `KYCStatus` 捨棄草稿），只回報不失敗（K1、K8）。突變：把 `measure_display.dart` 與 `CaseSections.tsx` 還原後，原型各自回報 `if`／`guard-cont` 與 `guard` | 修正輪後加跑含跨檔常數的原型 `eg/proto-xfile.mts`（website、admin、`hawkr-api-ts/src`）：受管命中也是 0（完成前審查 F1 的那一筆已由 #8 消除）。刀 3 以最終工具重跑，數字不同再更新 |
| 7 | §8 刀 2 驗證：`dart format --output=none --set-exit-if-changed` | `packages/hawkr_ui_text/lib/hawkr_ui_text.dart` 回報 Changed。原先推論「本機 Dart 與 CI 版本不同」不成立：`.fvmrc` 的 3.41.2 與本機同為 Dart 3.11.0，差異是 `resolveUiLocale(...) {` 正好 80 字元（`65be53b77` 起就是），是既有檔案的格式問題。修正輪用 FVM 的 `dart format` 修了這個檔（併入刀 2 的 commit），之後 `dart format --output=none --set-exit-if-changed lib test tool` exit 0 | 完成前 design-review F2 |
| 8 | （修正輪 A）卡片的違規依據句 | 契約：`Measure.end_unavailabilities`（45，`MeasureActionUnavailability`），`MeasureActionUnavailableReason` 加 `VIOLATION_BASIS`（7）與 `REMEDIATION_PENDING`（8）。規則 `model.Measure.EndUnavailableReasons`：已撤銷、不在生效中、依據不是風險類型（終止合作是既有的 `TERMINATION_RECREATE`，其餘是 `VIOLATION_BASIS`）、待處理的解除請求（`REMEDIATION_PENDING`）；`CanEnd` 是清單為空，`End` 的前置檢查改取清單第一項（刪除 `measure.end_unavailable` 單一句），API 的 `can_end` 與清單同一份。`VIOLATION_BASIS` 的句子逐字是原 admin 常數；`REMEDIATION_PENDING` 的句子「這筆措施有待處理的解除請求，請先裁定」是新句（清單要窮舉 `CanEnd` 為假的原因，等價測試要求每個原因有文字），Will 2026-10-22 核可（連同下列看得到的改變）。等價測試 `TestEndUnavailableReasonsAgreeWithTheLegacyPredicateOnEveryCell`（種類 × 依據 × 狀態 × 補正請求，每一格與舊判斷一致，內容與順序有獨立預期表，`VIOLATION_BASIS` 句子釘住原字）；httpapi 的 `TestAdminMeasureCarriesTheActionUnavailabilitiesAndStoreName` 加結束；admin 卡片顯示第一個原因（`unavailableLabels`，沒有句子時是通用文字，與其他措施卡一致），刪 `VIOLATION_MEASURE_TEXT`。看得到的改變：違規依據的停權卡從「只有那一句」變成與其他措施一樣的依據、理由、時間列，加上那一句；終止合作與有待處理解除請求的風險停權卡多一行不能結束的原因 | Will 2026-10-22 A |
| 9 | （修正輪 B）詳情頁的結束時間 | 新增 `endTimeText(measure)`（`measure.ts`）：有 end_time 顯示時間；沒有而有 review_by 顯示「未定（由平台定期複審）」；都沒有回 undefined。卡片與 `MeasureContentSection` 都用它 | Will 2026-10-22 B |
| 10 | §2 #3：看得到的改變 | Q1 核可的後果補記：(1) 狀態卡上的終止合作也多一列「生效時間」（改前卡片沒有）；(2) 有結束時間而沒有複審期限的風險停權，原本的「· 複審期限：-」不再出現 | code review #1、#2 |
| 11 | §2 #3：`appealDeadlineText` | 拿掉種類檢查，與詳情頁一樣只看 `appeal_deadline_time` 有沒有值 | code review #3 |
| 12 | §6.3 轉報區塊測試 | 新增「轉報區塊的顯示條件」測試：只有轉報紀錄、只有不轉報紀錄、只能記錄、只能標記不轉報、只能記下回覆、只能匯出，各自單獨成立就顯示（用非食安案件，證明不看類別）；全部沒有不顯示。突變：把五個條件各拿掉一個，測試都紅 | code review #4 |


## 實作時的修訂（刀 3，2026-10-22）

| # | 設計的寫法 | 實作 | 原因 |
|---|---|---|---|
| 1 | §1.1：`Hit{file, line, rule:"E", form, enumName, member}` | 照做。TS `Hit.form` 是字串聯集；Dart `Form` 是 enum（`Form.caseBranch` 等，`label` 是設計的 form 名稱，`if`、`else`、`ternary`、`and`、`or`、`guard`、`case`、`key`、`pair`）。跨檔常數由呼叫端給 `ConstResolver`（TS `scanSource(file, text, manifest, consts?)`、Dart `scanSource(path, source, manifest, consts:)`），`scanSource` 不碰檔案系統；`scanDir` 用 `makeConstResolver(root)` 提供。`exportedHanConsts` 也匯出供測試 | 工具的純函式部分可以不建暫存目錄就測；跨檔那一段另有 `scanDir` 的整合測試 |
| 2 | K4：跨檔匯入的常數，「import 解析到同一個 client 目錄樹內的檔案」 | TS：具名 import（含 `as`），說明符是相對路徑或被掃描目錄的 `tsconfig.json` `paths` 別名（website 的 `@/*`→`./*`、admin 的 `@/*`→`./src/*`），目標必須在掃描的目錄樹內，被匯入的檔只取 `export const X = "含漢字字串"`，不再展開它自己的 import。Dart：`import` 的相對路徑或 `package:<本 App>/…`（名稱取自掃描目錄上一層的 `pubspec.yaml`，對應掃描目錄），只取頂層公開 `final`／`const`，遵守 `show`／`hide` 與 `as` 前綴（`p.T`）。不涵蓋：TS 的 namespace import 與 re-export、Dart 的 class 靜態欄位與 re-export | 兩個 client 目前的跨檔常數都是這兩種形式；其餘形式沒有實例，列進檔頭的「不涵蓋」，觸發沿用 §9 |
| 3 | K4／§1.3：作用域「在使用處之前的宣告」 | 區塊內照做（`st.end <= 使用處`）；SourceFile／CompilationUnit 層級不看位置 | 函式本體在呼叫時才執行，頂層常數宣告在函式之後仍然成立；只看使用處之前會漏掉它。參數與 `let`／`var` 遮住同名的常數照設計 |
| 4 | K5：別名 `cond ? x : null` 的「真值蘊含 cond 為真」 | TS 的 `alias &&`、`alias ?`（原樣）之外，TS `alias != null`、`alias !== undefined` 與 `alias == null` 為假，Dart `alias != null`（Dart 沒有真值）同樣代入；只在別名的初始值是條件式時才成立，比較式的別名不當成「有值」 | Dart 的真值只能寫成 `!= null`；沒有這條，Dart 版 K5 沒有可用的形式。兩端對稱，各有正例與反例 |
| 5 | K3：`default:`、`_` 不算 | 另外：`default` 子句會清掉前面 fall-through 進來的成員（`case E.A: default: return "…"` 的文字不屬於 A）；`switch (true)` 的 case 比較只在 switch 運算式本身是字面值 `true` 時才當條件 | 進入 default 的路徑不是確定相等（K3 的同一個理由）；非 `true` 的 switch 的 case 運算式不是條件 |
| 6 | §6.1：突變測試 | 兩個工具各跑一輪：TS 33 個突變、Dart 45 個突變（每個形式拿掉、補集也算、`&&` 為假與 `\|\|` 為真也蘊含、`!` 不反轉、guard 不看離開、default 不特別處理、UNSPECIFIED 不排除、別名／常數／跨檔／show／hide／前綴／樹邊界各拿掉、report 含條件形式、不擋 `serverOwned`、pair 進入巢狀函式等），測試全部變紅。第一輪 TS 有 3 個、Dart 有 5 個突變沒有變紅（`&&` 為假的 `!==` 那一支、`default` 之後還有 case、樹邊界的測試檔放錯位置、Dart 的集合 `if` 的 else 與 if-case、`||` 為真的 `==`、`pb.E.UNSPECIFIED`、`show`／`hide`），補上正反例後重跑全紅。Dart 的「default 之後還有 case」不可能寫（Dart 的 `default` 必須是最後一個），改以「拿掉 `SwitchDefault` 的特別處理」的突變抓。突變是手動以腳本跑，不進 CI | 突變只證明這一版測試抓得到這些改壞，不證明規則沒有別的洞（§1.6、§9 的殘餘仍在） |
| 7 | §8 反向驗證 | 以第 2 刀以前的檔案放進暫存目錄：`measure_display.dart`（`0a3265c1f`）→ Dart 工具 exit 1，`E/if` 點名 `MEASURE_STATE_ACTIVE`（`:228`）與 `ENDED`（`:234`）；`CaseSections.tsx`（`0a3265c1f`）→ TS 工具 exit 1，`E/guard` 點名 `PolicyCategory.FOOD_SAFETY`（`:246`）；`ActiveMeasuresCard.tsx` 與 `store-detail.ts`（`9e425da10^`，含跨檔常數 `VIOLATION_MEASURE_TEXT`）→ TS 工具點名 `E/ternary`（`MeasureKind.TERMINATION` 與 `MeasureBasis.VIOLATION` 的那一筆就是跨檔常數） | 設計 §8 與刀 2 完成前審查 F1 |
| 8 | §2、§9：命中來自原型 | 最終工具：受管 enum 命中 0（website、admin、`packages/hawkr-api-ts/src`、seller-app、buyer-app、三個 Dart 套件）。非受管 enum（不失敗）：對照表形式寫進 report 的 admin 136 筆（key 105、case 21、pair 10）；條件形式不回報，admin 27 筆（and 14、if 6、ternary 6、guard 1）、website 1 筆（`KYCStatus` 的 and），兩支 App 與 Dart 套件 0（修正輪 DR F2 更正分類；163＝136＋27；修正輪之後的數字見修正輪末段）。與原型的位置逐筆相同（`comm` 比對）。`scripts/check-enum-text-cross.sh`：跨端 0，單端的對照表 37 筆 | 設計 §9 要求以最終工具重跑 |
| 9 | §4.2：各 `AGENTS.md`、跨端腳本檔頭 | 改寫：根 `AGENTS.md` 共用文案目錄列、`seller-app`、`buyer-app`、`admin`、`website` 的 `AGENTS.md` 守門說明；`scripts/check-enum-text-cross.sh` 檔頭（接上與 report 只含對照表的形式）；`ui-text-guard.yml` 的說明註解與 `_flutter-quality.yml` 的 step 名稱（`Guard catalog enum text (rule E)`）。兩個工具的檔頭改寫成規則 E 的摘要、「不涵蓋」段與用法，不再說三元式由 I-CI 補；seller `CARRYOVER.md` 沿用盤點補「停權效果句」與「守門依語法形狀」兩列 | 設計 §4.2、§7 |
| 10 | 設計沒寫：報錯訊息 | `<file>:<line>: E/<form> <enum>.<member> 成立時產生使用者面文字；這個 enum 的文字在共用目錄…或由伺服器擁有`，結尾加「文字不是這個值的說法時，改成依契約的欄位或伺服器給的可用性分流」 | §1.4 |


### 修正輪（刀 3 完成前審查，2026-10-22）

審查：獨立程式審查（M1、S1-S4、CI 缺口）與 design-review（F1-F8）。每一條先用 fixture 或讀碼查證，成立的都修，不留技術債。

| # | 意見 | 查證 | 處理 |
|---|---|---|---|
| CR M1 | 具名的成員集合（`const S = new Set([E.A]); S.has(k)`、`L.includes(k)`、Dart `_s.contains(k)`）不是選擇子 | 成立：`implied` 只收字面值接收者；repo 的慣用寫法是具名集合（`store-row.ts` 的 `PENDING_FLOWS.has`、`pageCapabilities.ts`、`onboarding_tasks.dart` 的 `_knownRequirements.contains`），今天沒有選文字，所以 0 命中是對的，下一個會漏 | 兩端的接收者改走 K5 的作用域查找：識別字查到 `const`／`final` 的初始值，（去 `new Set(…)`、`as const`、括號後）是陣列／list／set 字面值就用它的元素；最多追 4 層（`new Set(L)`）。命中行是集合宣告裡成員所在的行。測試：正例（Set、陣列、`new Set(L)`、區域 `final`）、`!S.has` 反例、參數或 `let`／`var` 遮住與內容不是成員的反例；突變：不查識別字 → 紅 |
| 修正輪自查 | 成員檢查接上具名集合後，`if (!OPTIONS.includes(x)) return "請選擇…"; …` 這種選項驗證的 guard 在 admin 多出 18 筆偽陽性（`RISK_GROUND_OPTIONS`、`VIOLATION_SEVERITY_OPTIONS`、`KYC_ISSUE_CODE` 的選項清單，含受管的 `RiskGround`、`ViolationSeverity`，會讓 admin lint 紅） | 成立，以工具實跑量到：guard 之後的敘述是集合裡任何一個值共用的（驗證其他欄位的錯誤句），不是某個值的說法；K3 的 guard 本意是 `x !== M` 這種單一成員的相等 | `guard` 不用成員檢查（`implied` 的 `membership` 參數為假）；`if`、三元式、`&&`、`case` 照舊用。測試：選項驗證的 guard 不報、同一個集合的 `if` 報；突變：guard 也用成員檢查 → 紅。之後新增的命中只有 S3 與具名集合帶出來的非受管 enum 的 3 筆（`AuthorityOrderState.REVOKED` 的 key、`ProspectStatus.ACTIVE`／`CONVERTED` 的三元式，都是同檔物件常數的屬性）。受管命中仍是 0 |
| CR S1 | `export { U }`（宣告在別處）不算跨檔常數 | 成立 | `exportedHanConsts` 另讀沒有 `from` 的 `export { U, V as W }`，名稱用匯出名。`export … from` 的 re-export 仍不涵蓋。測試（scanDir）與突變 |
| CR S2 | Dart guard 在 switch case 裡漏掉（TS 有） | 成立：`parent is Block` | 父節點可以是 `SwitchMember`，與 TS 對稱。測試與突變 |
| CR S3、DR F5 | 同檔物件常數的屬性（`TEXT.term`）不算文字 | 成立；刀 2 完成前審查 F1 的 `VIOLATION_MEASURE_TEXT` 是同一類 | TS：`TEXT.key`、`TEXT["key"]`，`TEXT` 是同檔 `const` 物件字面值，該屬性的值含漢字；Dart：record 常數的 `text.key`、map 常數的 `text['key']`。跨檔的物件常數仍不涵蓋（沒有實例），列進檔頭與 §1.6。測試與突變（每個形式各一） |
| CR S4 | 解構參數遮住匯入常數的偽陽性 | 成立：`lookupVar` 只看 Identifier 參數 | 遮蔽改用綁定名稱（含解構）：函式參數、catch 變數、`const { T } = o` 這類解構宣告。Dart 沒有解構參數，不需要。測試與三個突變 |
| CR CI 缺口 | `packages/hawkr-api-ts/src` 與三個 Dart 套件只在跨端腳本掃，exit 1 被當成功，所以共用套件的命中哪裡都不失敗 | 成立；以暫存的違規檔實測（兩種套件各一）原本 exit 0 | `check-enum-text-cross.sh`：`ts-packages`、`dart-packages` 的工具 exit 1 就是失敗（印工具的訊息）；四個 client 的 exit 1 仍由它們自己的 lint／step 負責。檔頭說明哪些由誰擋。暫存違規檔實測兩種套件都 exit 1，之後已刪 |
| DR F1 | 把 client 資源查詢加進「產生文字」沒有計畫項目承接，website 的時機也寫錯 | 成立：計畫沒有 I-app-0 項目；I-CI 的 website 那一刀起新文字放進 messages，`t("…")` 在目錄 enum 分支裡就繞過守門 | 計畫 I-CI 加一句：website 的部分與規則 E 加入 next-intl 查詢同一刀；新增計畫項目 I-app-0（觸發：10d 的 App 改動合進 main 之後；同一刀加入 ARB 查詢、啟動 I-CI 的 App 部分）；§0.2、§9、Pending、兩個工具的檔頭同步；多語系 ADR 在文末加一條「觸發的更正」，不改原文。website 提前到 I-CI 那一刀是時程判斷，Will 不同意就改成「提」 |
| DR F2 | 「單端 enum 只回報」與行為不符 | 成立：條件形式不失敗也不出現在任何輸出；report 只有對照表形式（admin 136 筆）；條件形式 admin 27 筆、website 1 筆共 28 筆算完就丟 | 全部改成「條件形式在非受管 enum 上不失敗也不回報；對照表形式寫進 report」，admin 163＝136＋27（#8 的數字不變，只是分類）。殘餘「第二個 client 對只有一個 client 用的 enum，以條件形式寫出值的說法」寫進 §1.6 與兩個檔頭 |
| DR F3 | Dart dot shorthand 的觸發沒寫 | 成立：SDK 下限 3.8，要 3.10 才寫得出 | 寫進 §1.6 與 Dart 檔頭：任一 Dart 專案的 SDK 下限提到 3.10 以上時，以成員名支援 |
| DR F4 | 檔頭說殘餘由 I-CI 把關，但 admin 不在 I-CI | 成立 | 兩個檔頭與 §1.6 寫明 admin 的殘餘只靠 code review |
| DR F6 | 跨端腳本的執行期訊息還寫「手寫文字表」 | 成立 | 兩條訊息改「對照表」 |
| DR F7 | 計畫紀錄的 `git log --grep` 是前向引用 | 計畫的片語已是「目錄 enum 文字守門第 3 刀」 | commit 訊息要逐字含這幾個字；commit 後用這個 grep 驗證一次 |
| DR F8 | seller `CARRYOVER.md` 的「守門依語法形狀」一列放錯模組 | 審查已駁回（設計書有完整紀錄，這一列無害） | 不動 |

修正輪後的數字：非受管 enum（不失敗）admin 對照表形式 137 筆（key 106、case 21、pair 10）、條件形式 29 筆（and 14、ternary 8、if 6、guard 1）、website 條件形式 1 筆（合計 admin 166）；受管命中 0；TS 工具 40 個測試、突變 40 個全紅；Dart 33 個測試、突變 52 個全紅。

---

## Pending

沒有。§9 的觸發屬於已排的 I-CI 與 I-app-0（計畫兩個項目，I-app-0 在刀 3 完成前審查後補成項目），不另開待辦；刀 1 完成前 design-review 提出的 runbook 與 admin 合併欄位兩項已解決（實作時的修訂 #11、#12）。

## 10. Will 的決定（2026-10-22；全部已核可）

| # | 題目 | 內容 |
|---|---|---|
| D2 | ADR D2 文末修訂 | **已核可（Will 2026-10-22）**：照 §4.1，刀 2 寫入 ADR。v1.3 依 Will 的 F4 決定，第一批拿掉 admin 關聯帳號說明句（只剩 seller-app 三句） |
| 文案 | §5 #1-#7 | **已核可（Will 2026-10-22）**：en 三句、admin 三處改字、術語表一列 |
| 文案 | §5 #12-#16 | **已核可（Will 2026-10-22）**：伺服器給員工的兩則終止說明句、admin 終止表單三句，照 §5 原文 |
| Q1 | 措施欄位列（§2 #3） | 欄位列依契約欄位有沒有值顯示：`period.start_time` 有值顯示「生效時間」、`end_time` 有值顯示「結束時間」、`review_by` 有值顯示「複審期限」、`appeal_deadline_time` 有值顯示「申覆期限」（規則由契約欄位註解推出，不是守門的要求）。看得到的改變：(1) 停權多一列「生效時間」，值等於建立時間（H9）；(2) 已結束的預告終止（店家關閉）多一列「結束時間」，可能早於「生效時間」（H9）；沒有結束時間的停權不再顯示「結束時間：-」。**已核可（Will 2026-10-22）** |
| Q2 | 終止表單的事由需求（§2 #10，刀 1） | (b)，v1.3 依 Will 的 F2、F3 決定修正：`TerminationOption` 只加 `violation_requirement`（三態），**不加** `requires_source_measure`（表單狀態照請求的形狀）；條件句寫進伺服器的 `explanation`（§2.1）。看得到的改變：其他 21.3 事由出現選填的「依據的認定」欄位；說明句的內容見 §5 #12-#16。**方向已核可（Will 2026-10-22）** |
| Q4 | 食品業者登錄（§2 #11，刀 1） | **(A)（Will 2026-10-22，取代 v1.2 的三個可用性布林）**：`open_food_registration` 拆成 `pending_food_registration` 與 `rejected_food_registration`；帳號能不能操作維持由 `GetMyStore.capabilities` 的 `STOREFRONT_WRITE` 擋。v1.2 寫的看得到的改變（「停權時三個按鈕不顯示」）撤回：兩個 client 的錢包頁本來就以 `STOREFRONT_WRITE` 擋住（H14），所以現在就是如此 |

v1 的 Q3 由 SSOT 回答（§2 #9），Q5（縮小規則）由 K3 的正向定義與 §1.2 的 pair 收斂取代，都不再列。

---

## 附錄 A：審查意見的處理（v1 → v1.1；審查全文在 session scratchpad `enum-guard-design-review.md`）

每一條都先對照程式或 SSOT 查證。

| # | 嚴重度 | 意見 | 查證 | 處理 |
|---|---|---|---|---|
| F1 | Critical | K12 與 ADR D2 分界牴觸，只改 `AGENTS.md` 不夠 | ADR `:77`「分界不變…兩個以上 client」；工作區 ADR 的未提交修改只有 D4［量測］ | 接受：K12 改成 ADR 文末修訂，§4.1 起草；L′ §7(d) 文末修訂；`AGENTS.md` 照 ADR。不動 ADR 檔 |
| F2 | Major | 補集方向不該算選出 | 正向版原型重跑，少的命中與審查列的相同；`measure.ts:480` 仍在，因為 `:482` 是正向［量測：`ts-hits6.tsv`］ | 接受：K3 改正向、刪 `default` form 與 K10；§2 重算；網站期間句的 en 刪除 |
| F3 | Major | Q3 可由現有契約回答 | `Case` 的 can_* 與紀錄欄位（H8）［量測］ | 接受：§2 #9，不再是待決 |
| F4 | Major | Q2、Q4 是契約改動，要以前提本身成立並拆成先行的刀；Q4 不只撤回 | 契約允許選填、畫面沒做（`governance.proto:885-886`、`CreateTerminationDialog.tsx:199`）；三個 RPC 都 `authorize`（H11）［量測］ | 接受：K13、刀 1；Q4 涵蓋送出、撤回、清除 |
| F5 | Major | 應拆刀，「同一 commit 否則 CI 紅」不成立 | 先修違規、最後開規則，換工具時命中是 0［推論］ | 接受：§0.1、§7 三刀 |
| F6 | Major | Q1 沒指明哪個 end_time、終止合作的情形沒列 | `Measure.end_time` 與 `period.end_time` 同源（`governance_measures.go:101,116`）；預告終止關店時 end 可能早於 start；停權的 start 是建立時間［量測］ | 接受：§10 Q1 列兩個看得到的改變 |
| F7 | Minor | `serverOwned` 不是偏離 | L′ O4 `:835`［量測］ | 接受：K2 改引 O4 |
| F8 | Minor | UNSPECIFIED 排除的根據成立 | 同審查的命令［量測］ | 接受：K2 補量測。manifest 加零值成員名不做：命名由 buf lint 機械化保證，多一個欄位沒有增加保證 |
| F9 | Minor | §2 #3 寫死 `MeasureState.ACTIVE` | `ActiveMeasuresCard` 讀 `measure`［量測］ | 接受：§2 #2 用 `measure.state` |
| F10 | Minor | 「負責人」資訊可能遺失 | 區塊標題已是「公司負責人身分資訊」／「賣家身分資訊」（`KycApplicationCards.tsx:368`）［量測］ | 接受查證，結論是不會遺失；不需要改區塊標題 |
| F11 | Minor | K4 只追頂層常數 | 原型改成追所有 `const` 後多 1 筆受管命中（`KycApplicationCards.tsx:502`）［量測］ | 接受：K4 用 K5 的作用域；§2 #5 加這一筆與 alt 改字 |
| F12 | Minor | pair 會深入巢狀函式與 JSX 子樹 | 今天 pair 受管命中都是直接屬性［量測］ | 接受：§1.2 收斂 |
| F13 | Minor | en 與術語表 | 術語表沒有「複審」［量測］ | 接受：術語表加一列；#2 改 `under periodic review`；網站期間句的 en 隨 F2 刪除 |
| F14 | Minor | §1.6 沒有實例量測 | `LABELS[m.kind]` 形狀搜尋，沒有受管實例（H12）［量測］ | 接受：H12；其他殘餘形狀仍未量測，§9 |

## 附錄 B：開工前 design-review 的處理（v1.2 → v1.3；全文在 session scratchpad `enum-guard-design-review-start.md`）

每一條都先對照程式查證。

| # | 嚴重度 | 意見 | 查證 | 處理 |
|---|---|---|---|---|
| F1 | High | Q4(a) 三個裸布林與 `SellerCapability` 平行，只沿用了賣家端 `can_X＋X_unavailabilities` 的一半 | `SellerCapability` 註解「client 以它決定畫面上哪些操作可用」（`seller.proto:231-234`）；兩個 client 的錢包頁以 `STOREFRONT_WRITE` 擋（H14）；`open_food_registration` 合併待審與被退回（`seller.proto:350-352`）［量測］ | 接受，Will 選 (A)：拆欄位、不加布林；§2.1、§2 #11、§10 Q4；v1.2 的看得到的改變撤回 |
| F2 | Medium | `requires_source_measure` 多餘 | `available_terminations` 不含 LINKED（`admin/seller/v1alpha1/seller.proto:336-339`），LINKED 只在 `LinkedAccount.termination_option`（`measure.proto:229-231`）［量測］ | 接受（Will）：刪欄位；表單狀態記住選項來源，`sourceMeasureId` 只在 linked 時存在（§2 #10） |
| F3 | Medium | 三態沒帶 7 日規則；選填路徑的文案沒有定 | `CreateTerminationDialog.tsx:199-206` 的 `noteText` 寫死 7 日；7 日在 `create.go:510-518`（H15、H16）［量測］ | 接受（Will）：條件寫進伺服器的 `explanation`（§5 #12、#13），client 不寫規則；表單三句（§5 #14-#16）。全部列 N 待核可 |
| F4 | Medium | 關聯帳號說明句進目錄，與 `TerminationOption.explanation` 平行 | `terminationExplanation` 把伺服器句與 client 常數串在一起（`measure.ts:557-580`）；給員工的事由說明只有 `TerminationEffect.Message` 一個產生點（`termination_effect.go:44`）［量測］ | 接受（Will）：§2 #6 改由刀 1 的 `explanation`；§3 刪 key；§4.1 第一批拿掉；`admin.seller` 前綴風險消失 |
| F5 | Medium | 刀 1 對實作者不夠明確 | 規則在 `create.go:147-156` 的 switch［量測］；`toStore` 有 12 個呼叫點［量測］，但 (A) 不需要在每條路徑推導能力 | 接受：§2.1 寫明 `ViolationRequirement` 方法、驗證與輸出共用、等價測試名、欄位編號（`TerminationOption` 4、`Store` 29／30，27 reserved）、enum 前綴（`API 設計規範（Connect）.md:102`）、命名對齊請求的 `violation`。F5.2（`Authorize` 與 `Conditions.Derived`）因 F1 採 (A) 不適用 |
| F6 | Medium | 新的 api enum 會讓 manifest 漂移 | manifest 收錄 schema 的所有 enum（審查量測 `manifest.go:29-45`）；刀 1 另有模組新句子 | 接受：刀 1 跑 `make -C backend uitext`，§7、§8 加 `ui-text-generated-diff.yml` |
| F7 | Low | #11 的 client 修法要拆區域 | 兩端的 pending 區域同時有狀態行與撤回按鈕（`FoodRegSupplement.tsx:208-237`、`food_registration_form.dart:110-131,226`）［量測，審查］ | 接受：§2 #11 寫明只有按鈕改條件，狀態行留在原位置、條件改成同一個欄位有沒有值 |

