# 決策簡報 v2：enum 顯示文字與「為什麼不能做」的文字放哪裡（hawkr）

**Will 的決定（2026-10-17）：全照建議。** Q1 名詞文字採共用目錄（獨立套件、完整、不覆寫、生成 TS／Dart／Go），修訂 codemeta ADR D5；Q2 每個原因一句伺服器文字（`LocalizedMessage`），website 的短提示與整句收斂；Q3 `REVOKE_NOTE` 只由 enum 值決定，放進目錄；Q4 刀 L 插在 3a 與 3b 之間，一次搬完全部 proto enum。

**已被 `step10e-i18n-brief.md` 部分取代（Will 2026-10-17 決定多語系）**：D-L1 的 c 改為該文第 4 題的 b′，「只有 zh-TW」「第二語系才接 TMS 與 Accept-Language」的延後條件已成立，見該文 I1–I4。

**建議**：一條規則分兩類文字。**只由 enum 值決定、client 在沒有回應時也需要的名詞**（篩選、表單、徽章）→ 契約外的一份共用文案目錄，生成 TS、Dart 與 Go 的型別化存取，**目錄必須完整、不做依端覆寫**，哪個值不顯示由 client 決定。**伺服器對某個資源此刻狀態的判斷與說明**（不能操作的原因、帶參數的句子）→ 伺服器在原因 enum 旁回文字，與 `failed_precondition` 錯誤用同一個 Go 函式產生。3a 提交前先拿掉新加的 proto 標籤與 regex 測試，`MeasureActionUnavailableReason` 在 3a 就改成帶文字；接著插一刀「L」，**一次搬完所有 proto enum**（舊 REST 的 CodeMeta 值隨模組搬遷，已有排定的退場）。

基準：worktree `hawkr-10e`，HEAD `36fb5d894` 加上未提交的 3a。只讀調查。前提：未上線、只有 zh-TW、`v1alpha1` 不受 `buf breaking`；「與現有程式一致」「改動量」「工作量」一律 SELF-IMPOSED；Will 2026-10-15：能不能做與原因由伺服器推導，client 只呈現。

## 結論

| 決策 | 建議 | 與 v1 草稿的差別 |
|---|---|---|
| D-L1 名詞 | 共用目錄（標準 i18n 訊息格式的 key 集合）＋生成 TS／Dart／Go 存取；生成器以 buf image 驗「每個非 UNSPECIFIED 的值都有文字」；不做 `@seller` 覆寫、不做「明列不顯示」 | 拿掉覆寫與隱藏；Go 輸出改為**現在就要**（理由是 D-L2 的伺服器句子會嵌名詞，不是 legacy）；放獨立套件而非 `@hawkr/api` |
| D-L2 原因 | `repeated XxxUnavailability { reason enum; LocalizedMessage message; }`，文字由前置檢查同一個函式產生 | 增加「一個原因一句話，client 不再有兩個欄位（短提示 vs 整句說明）」需 Will 拍板；伺服器句子不寫 client 的導覽名稱 |
| 切分 | 3a 內修正 → 刀 L（目錄＋全部 proto enum＋賣家三個原因 enum＋ADR 修訂）→ 3b | L 一次搬完全部 proto enum，不分 governance 先行 |

---

## D-L1：enum 名詞的顯示文字

### 現況（已逐一查證）

- ADR `backend/docs/adr/2026-09-25-codemeta-retirement.md:60`（D5）文字在各 client 呈現層、TS `Record<Enum,string>`；`:61`（D6）proto 註解只寫語意；`:48-49` 否決 C（custom option）與 D（以值名為 key 的 label 目錄，「漂移成為實際問題時的後備」）。
- 實際做法違反 D6：`rg -c '標籤「' api/` → `seller.proto` 60、`measure.proto` 10、`error_reason.proto` 2，共 72。`error_reason.proto:35-40` 甚至把**整句錯誤文字**也稱「標籤」，與 `model/model.go:120,122`、seller-app `seller_onboarding.dart:28-33` 三處同一句手抄。
- 綁定只有 3 個 enum〔VERIFIED〕：`model/measure_test.go:378` `TestRiskGroundLabelsEqualTheProtoLabels`（RiskGround）、`admin/tests/violation-label-proto.test.ts`（3a 新檔，ViolationCategory／Severity，以 regex 解析註解排版）。`TerminationGround.Label()`（`model/measure.go:95`）在伺服器有手寫表但沒有測試綁住。
- website 的對照是 `Partial<Record<…>>`〔VERIFIED，`website/features/seller/measures/measureDisplay.ts:30,40,48,55,60,80,86,197,211`〕，關掉了 D5 的窮舉；admin 的正確寫法是 `Record<Exclude<E, E.UNSPECIFIED>, string>`（`admin/src/app/admin/measures/measure-actions.ts:41-47`）。
- 規模（`rg` 每個 `api/` enum 在四個 client 非生成、非測試程式的引用）：約 59 個 enum，3 端 18、2 端 11、1 端 17、0 端 13；與 v1 的 17／11／16 相差在計數口徑，「引用」不等於有文字。TS 端 `Record<X, string>` 的 key 型別約 100 種，混有 CodeMeta 舊值（`SellerCancelReasonValue`、`ShippingTemperatureValue`……）與純 UI 型別，**只有 proto enum 能以值名為 key**。
- 「依端不同」的實例都不是同一值不同文字：`BASIS_LABEL` 只列 CONTRACT（`measureDisplay.ts:55-57`，VIOLATION／RISK 由別的欄位說明）、`REVOKE_NOTE` 是句子不是名詞（`:80-83`）、COOLDOWN 是帶參數的句子（`website/lib/seller/measures.ts:238-241`）。
- 已有 i18n 框架但是空殼：website 裝了 `next-intl`、`website/messages/zh-TW.json` 只有 348 bytes 的 scaffold；admin 沒有；seller-app 只有 `flutter_localizations`、沒有 `l10n.yaml`；buyer-app 都沒有。
- 伺服器用到名詞的地方：`legacy/sellerreason/reason.go:76,89`（隨舊契約退場）、`measure/create.go:108`（錯誤訊息嵌 `TerminationGround` 名詞）、`measure/update.go:172`（「警告沒有結束時間」嵌 MeasureKind 名詞）。另有 delegation 模組的 `Label()`（`modules/delegation/internal/model/*.go`）經舊 REST 把文字送給 client，是伺服器擁有名詞文字的現存例子，不在本題範圍但要知道。

### 業界基準

- 名詞的顯示文字是 **client 的 i18n 資源**，以穩定 key 對應：Flutter `gen-l10n` 以 ARB key 生成 getter、next-intl／FormatJS 用 ICU MF1 `select`〔VERIFIED，見 `step10e-labels-research.md` §5〕。多平台共用同一組 key、同語言同一個翻譯，是 TMS 的核心功能（Lokalise：「this key cannot have different translations for the same language on iOS and Web」）〔VERIFIED〕。
- 所以真正的基準是「**一組跨平台共用的 key＋各平台的標準資源檔，由 TMS 或其來源檔管理**」，不是「生成器」本身。repo 內一份來源檔＋生成器**算不算**這個基準：算，前提是來源格式是 TMS 能直接匯入的標準格式（ICU JSON 或 ARB），key 跨平台一致；生成器在單一語系時只是代替 TMS 的匯出步驟〔INFERENCE〕。
- 不把 UX 文字放 IDL：AIP-126 沒有顯示名稱、`google/api/field_info.proto` 沒有；公開 API 沒看到 enum option 放顯示名稱〔VERIFIED／INFERENCE（否定搜尋不完整）〕。
- 依端覆寫：TMS 的作法是**不同 key**（例如 `seller.measure.kind.WARNING` 與 `admin.…`），不是同一 key 的覆寫語法〔INFERENCE，由 Lokalise 的限制推得〕。所以真有需要時用另一個 key，不需要在目錄發明 `@seller`。

### 為什麼現有機制存在

- D5「各端自己擁有」是為了切斷 CodeMeta 的集中擁有權，並假設「同一值在不同端文字本來就不同」（`:76`）。現況證據不支持這個假設（見上「依端不同」）。
- 2c-1 起 governance 用語是條款用語（第 21.2、17.1 條），三端必須相同，於是長出「proto 註解當唯一來源＋regex 測試」——走在 ADR 否決的方向上，又只綁了 3 個 enum。

### 選項

| 選項 | 做法 | 取捨 |
|---|---|---|
| a. 現狀 | proto 註解＋手抄＋regex | 違反 D6；只綁 3 個；`Partial` 失去窮舉 |
| b. ADR D5 字面 | 刪 proto 標籤與 regex，各端自擁、改回完整 `Record` | 條款用語改字要改 3–4 處，沒有跨端綁定；「同一詞」靠人 |
| **c. 共用目錄＋生成（無覆寫）** | 來源 `packages/hawkr-copy/zh-TW.json`（ICU MF1 相容的平面 key＝proto 完整值名），生成 TS（獨立套件 `@hawkr/copy`）、Dart（`packages/hawkr_copy`，排除 `hawkr.admin.*`）、Go（單體的 platform 套件）；生成器讀 buf image 驗完整，CI 比對生成檔 drift | 一處改字；缺值在生成時失敗（比 client 的編譯期更強，Dart 也有）；格式可直接交 TMS |
| c'. 同 c，但生成進既有 i18n 資源（next-intl messages、ARB） | 名詞走 `t('…')` | 現在三端只有空殼，等於為單語系導入整套 i18n 執行期；型別化存取較弱。第二語系時才划算 |
| d. proto custom option | `(label) = "…"` | UX 文案進 IDL、改字重生四端契約碼；無公開先例 |
| e. 伺服器回 label | 每欄位旁 `*_label` 或目錄 RPC | 篩選與表單要先打 RPC；名詞在 client 沒有窮舉 |

### 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| 只有 zh-TW | REAL（產品範圍） | codemeta ADR 繼承約束 |
| 篩選、建立表單、離線畫面在沒有資料時也要名詞 | REAL | `admin/src/app/admin/measures/measure-inbox.ts:71-79`、3b 的建立措施對話框 |
| 條款用語三端必須相同 | REAL（契約條款第 17、21 條是法律文字） | `measureDisplay.ts:59,85` 註解 |
| admin 專用 enum 不進 Dart | REAL（現行 Dart 生成設定，合理） | `packages/hawkr_api/buf.gen.yaml` `exclude_paths` |
| 舊 REST 的 CodeMeta 值沒有 proto 值名 | REAL 直到該模組搬進單體 | codemeta ADR 過渡步驟 3 |
| 伺服器句子嵌名詞（D-L2 之後所有原因句子都在伺服器） | REAL（D-L2 的後果） | `update.go:172`、`create.go:108` |
| 某些值在某端不顯示 | REAL，但屬 client 的呈現決定，不是目錄的屬性 | `measureDisplay.ts:55-57,76-84` |
| 與 D5、2c-1 慣例一致；生成器維護；改動量 | SELF-IMPOSED | — |
| 「改字就是契約變更」 | SELF-IMPOSED（上線前），且 c 已把文字移出契約 | `api/buf.yaml:24-25` |

### 建議（只依 REAL 調整）

採 **c**：
1. 目錄完整（每個非 UNSPECIFIED 值都有文字），不做覆寫、不做「明列不顯示」。client 選擇不顯示某值是 client 的程式碼。觸發覆寫的條件：第一次出現「同一值、不同端、不同文字」的需求時，加**另一個 key**，不加覆寫語法。
2. 目錄只放名詞；句子（`REVOKE_NOTE`、COOLDOWN）不進目錄——REVOKE_NOTE 屬於伺服器判斷的說明，併入 D-L2 或留 client 呈現，見問題 Q3。
3. Go 輸出現在就做，因為 D-L2 的伺服器句子嵌名詞；取代 `RiskGround.Label()`、`TerminationGround.Label()` 的手寫表與 `TestRiskGroundLabelsEqualTheProtoLabels`。這是 ADR D5 末句「伺服器產生的文字由產生它的模組擁有」的修訂：**句子**仍由模組擁有，句子裡的**名詞**從目錄取。
4. 獨立套件（比照 twaddr 的 `-ts`／`_dart` 雙套件與 `twaddr-generated-diff.yml`），不放進 `@hawkr/api`：該套件是契約生成物，放 UX 文字等於把 C 的問題搬回來。
5. client 一律 `Record<Exclude<E, UNSPECIFIED>, string>` 的生成型別；不認得的值走 ADR D1 的「未知」呈現。

**終局與觸發**：第二語系或非工程人員改文案 → 來源檔交 TMS，生成改成輸出 next-intl messages 與 ARB（c'）；伺服器加 locale 協商。在那之前不做 `Accept-Language`（YAGNI，REAL 約束只有 zh-TW）。

---

## D-L2：「不能操作」的原因與說明

### 現況（已查證）

- 四個原因 enum，形狀「布林＋依序原因」，文字在 client：`StoreOpeningUnavailableReason`、`AppealUnavailableReason`、`RemediationUnavailableReason`（`seller.proto`）、`MeasureActionUnavailableReason`（admin `measure.proto`，3a 新增）。
- 伺服器的錯誤路徑已經從**同一份原因清單**產生文字：`measure/appeal.go:17-35`（`errAppealUnavailable` 取 `AppealUnavailableReasons()[0]`）、`measure/update.go:162-183`（`errActionUnavailable`）。
- 兩個作者、內容不一致〔VERIFIED〕：
  - `update.go:180`「申訴待裁定，請以裁定申訴處理」vs admin `measure-actions.ts:55`「申訴待裁定，請先裁定」。
  - `update.go:178`「賣家已提出申訴或有待處理的補正，不能%s；請先裁定，要加重請另立一筆措施」vs admin `:54`「賣家已提出請求，只能縮短期間」——**語意不同**，不只是措辭。
  - `appeal.go:30`「終止合作的申訴期限已過」vs website `measures.ts:192`「申訴期限已過」——這一組是不同 UI 位置（反灰按鈕旁的短提示 vs 失敗後的整句），不一定算錯，但同一事實兩個來源。
  - 開店三處同句手抄（見 D-L1 現況）。
- client 的分支是真的需要〔VERIFIED〕：website `remediationBlockedReason` 對 NOT_RISK／NOT_ACTIVE 不顯示入口、`appealClosedNotice` 對 ALREADY_APPEALED／REVOKED 用整句、其他用短提示（`measures.ts:200-245`）；seller-app `offersAppeal` 決定是否導向「處置與申訴」。
- 錯誤模型 ADR `2026-09-25-api-error-model.md:42` 否決「伺服器只回 code、文字全由 client 查表」：舊版 App 遇到新 reason 只能顯示通用文字、改字要發版、四端各一份表；`:58-62`（D3）使用者文字只走 `LocalizedMessage`、`locale` 固定 zh-TW；`:102` 理由。

### 業界基準

- Google Merchant API `issueresolution/v1`〔VERIFIED，proto 原始碼〕：`Action.is_available`「If the action is not available then a reason will be present」；`reasons`「If there is only one reason, it can be displayed next to the disabled button. If there are more reasons, all of them should be displayed」；`Reason` 只有 `message`、`detail`（「displayed as a hint」）、`action`，**沒有 enum**；`button_label` 也由伺服器給；`language_code` 未設定時 `en-US`。前提是第三方 client 不認識 Google 的政策目錄〔INFERENCE〕。
- GitHub GraphQL `viewerCannotUpdateReasons: [CommentCannotUpdateReason!]!`，只回碼〔VERIFIED〕：client 擁有詞彙、永遠是最新版時的做法。
- AIP-193〔VERIFIED，重抓 google.aip.dev/193〕：`LocalizedMessage`「should be localized to a user-specified locale where possible」；未指定時 en-US；「Any dynamic aspects of the message must be included as metadata within the ErrorInfo」。AIP-193 沒有規定誰本地化、沒有提 `Accept-Language`。
- 小結：碼兩派都有；文字由伺服器給的條件是「client 不能即時更新」或「同一判斷在別的路徑已由伺服器給文字」。本專案兩條都成立（上線後 App 有舊版；錯誤路徑已是伺服器文字）。

### 錯誤模型 ADR 的理由能不能搬過來

| ADR 理由 | 對可用性原因是否成立 |
|---|---|
| `message` 有四個寫入者、兩個不在控制內（`:16`、`:102`） | 不成立：可用性原因只有單體一個產生者 |
| 舊版 App 對新原因有正確說明（`:102`） | 成立：上線後 seller-app 會收到新原因；現在 seller-app 對不認得的原因「只擋、不顯示文字」 |
| 改字不必發版（`:102`） | 成立 |
| 四端各一份表（`:42`） | 成立，而且還多了與錯誤路徑不一致的第五份 |

三條成立、一條不適用，結論可以搬。

### 一致性：伺服器給原因文字，是否也要給名詞文字？

不用，規則是**文字是誰的判斷的說明**：名詞是 enum 值本身的名字，client 在沒有任何回應時也要用（篩選、表單），伺服器給反而製造執行期依賴（選項 e）；原因是伺服器對「這筆資源、此刻」的判斷，client 只在伺服器回 `can_*=false` 時才會看到，文字隨同一個回應到，沒有離線代價，且必須與同一判斷的錯誤文字相同。Merchant 連 `button_label` 都由伺服器給，是因為它的 client 連動作的名詞都不擁有，本專案不成立。

### 選項

| 選項 | 做法 | 取捨 |
|---|---|---|
| a. 現狀 | enum＋各端對照 | 與錯誤路徑兩句話且已語意不一致；舊版 App 無說明 |
| **b. enum＋伺服器文字（一句）** | `repeated XxxUnavailability { reason; google.rpc.LocalizedMessage message; }`；Go 每個原因一個文字函式，錯誤與可用性共用 | 一個來源；client 不再有原因表，只剩分支與通用文字。網站現在的「短提示」與「整句」兩種呈現要收斂成一句 |
| b'. enum＋`message`＋`detail`（Merchant 形狀） | 短的放反灰按鈕旁、`detail` 是說明 | 保住兩種 UI 位置；伺服器要寫兩份文字 |
| c. 只回文字（Merchant 原樣） | 無 enum | client 失去分支，會退化成解析文字；否決 |
| d. 原因也進 D-L1 目錄 | 名詞目錄管句子 | 舊版 App 仍無文字、改字仍要發版、帶參數的句子要 client 組；否決 |

`LocalizedMessage` vs 純 `string`：前者帶 `locale`，與錯誤同一個 Go 值，第二語系時不必改形狀；用在非錯誤的回應裡是型別重用、AIP 沒有禁止〔INFERENCE〕。建議用 `LocalizedMessage`。

### 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| 能不能做與原因由伺服器推導，client 只呈現 | REAL | Will 2026-10-15 |
| client 依原因分支 | REAL | `measures.ts:200-245`、`seller_onboarding.dart` `offersAppeal` |
| 錯誤的使用者文字由伺服器以 `LocalizedMessage` 給 | REAL（accepted ADR） | `api-error-model.md:58-62` |
| 上線後舊版 App 收到新原因 | REAL（上線後） | codemeta ADR 外部約束 |
| 只有 zh-TW | REAL | 兩份 ADR |
| ADR D5「文字在 client」；改動量 | SELF-IMPOSED | D5 末句已把伺服器文字劃給模組 |

### 建議（只依 REAL 調整）

採 **b**（一句），除非 Will 要保留兩種 UI 位置（Q2 → b'）：
1. 保留 enum 給分支；文字並列。
2. 每個原因一個 Go 文字函式（可帶 action 與時間參數，以 Asia/Taipei 組好），錯誤與可用性共用；資料集測試鎖「原因為空 ⇔ `can_*`」「每個原因文字非空」「錯誤的 `LocalizedMessage` 等於該原因的文字」。
3. 伺服器句子**不寫 client 的導覽名稱**（`model.go:120`「請至「處置與申訴」提出申訴」）：去哪裡由 client 依 enum 給入口，否則 client 改選單名稱就讓伺服器句子過時。
4. 動態值依 AIP-193 精神另以結構化欄位給（COOLDOWN 已有 `next_remediation_time`），client 不解析文字。
5. client 規則與錯誤模型 D3 相同：有文字顯示、沒有顯示通用文字；刪原因對照表；`error_reason.proto:35-40` 的「標籤」與整句刪除。

**終局與觸發**：第二語系 → 伺服器依請求 locale 選文字（AIP-193 只說 may use request locale）；某原因需要「去哪裡修」超過一個固定入口 → 加 Merchant 的 `action`。

---

## 切分

### 3a 提交前必須改

1. 刪 3a 新增的 proto 標籤：`seller.proto` ViolationCategory／Severity 11 處、`measure.proto` 7 處（`git diff` 對 3a 的範圍確認）。
2. 不提交 `admin/tests/violation-label-proto.test.ts`。
3. `MeasureActionUnavailableReason` 的四個 `*_unavailable_reasons` 欄位改成 `repeated MeasureActionUnavailability`，Go 由 `errActionUnavailable` 抽出文字函式共用；admin 刪 `UNAVAILABLE_REASON_LABEL`，`measure-actions.ts` 只留通用文字與分支。這是 3a 新欄位，同一刀修形狀。
4. admin 在 3a 新增的 `VIOLATION_CATEGORY_LABEL`／`SEVERITY` 暫留（`git show HEAD:…measure.ts` 沒有它，是 3a 新加），由 L 立即取代；改為 `Record<Exclude<…>>`，不要 `Partial` cast（`measure.ts` diff 第 88 行）。

### 刀 L（3a 之後、3b 之前）

- 目錄、生成器（TS／Dart／Go）、CI drift job、獨立兩套件。
- **一次搬完全部 proto enum 的名詞文字**（四端 + Go 的 `Label()`），刪全部 proto 標籤與 RiskGround regex 測試。理由：搬遷是機械的、生成器保證完整；分批會留下「有的在目錄、有的手抄」的雙規則，正是「不留技術債」要避免的。工作量是 SELF-IMPOSED。
- 賣家端三個原因 enum 改 D-L2。
- 修訂 codemeta ADR（文末修訂，不改原文）：D5 改成上述規則；D6 重申；D 選項的觸發改判為成立；更正 C／D 否決理由中屬工作量的部分。錯誤模型 ADR 不改，只被引用。
- 不在 L：舊 REST 的 CodeMeta 值與 delegation 的伺服器 label，隨各模組搬進單體（codemeta ADR 過渡步驟 3）。

### 測試策略

- 生成器：缺值、多餘 key（值被刪）都失敗；CI 重跑生成比對 drift（比照 `twaddr-generated-diff.yml`）。
- client：不再測個別文字，只測「不認得的值 → 未知／通用文字」與分支。
- 伺服器：上面 D-L2 第 2 點三條；website e2e 的 mock 要帶 `message`，否則 mock 測的是舊形狀（`website-e2e-verify` 的 wire-shape 規則）。
- 無障礙：反灰按鈕要有可讀的原因（`aria-describedby` 指向伺服器文字），不只靠顏色；伺服器句子要能獨立讀懂（有主詞、不靠位置）。

## Will 要回答的問題

1. **D-L1**：同意 c（完整目錄、無覆寫、Go 一起生成、獨立套件），並修訂 codemeta ADR D5？
2. **D-L2 形狀**：一句（b，網站的短提示與整句收斂成一句）還是 Merchant 的 `message`＋`detail`（b'）？
3. `REVOKE_NOTE`（撤銷原因的說明句）：併入伺服器文字（措施回應帶說明），還是留 client 呈現？
4. 刀 L 一次搬完全部 proto enum（建議），或 governance 先行、其餘在 10e 結束前？

## 附錄：與 v1 草稿不同意之處

1. **依端覆寫與「明列不顯示」**：v1 把它列為 REAL 需求；證據裡沒有同一值不同文字的例子，「不顯示」是 client 的呈現決定。拿掉，給觸發條件。
2. **Go 輸出的理由**：v1 說「需要時再生成」、理由是 legacy；真正的理由是 D-L2 讓所有原因句子在伺服器，而句子嵌名詞（`update.go:172`、`create.go:108`）。所以現在就要。
3. **業界基準的表述**：v1 把「生成器」當基準；基準是「跨平台共用 key＋標準資源格式（TMS）」，生成器只是單語系時的匯出步驟。v1 沒提 website 已裝 next-intl（空殼），所以沒有比較 c'。
4. **套件歸屬**：v1 放進 `@hawkr/api` 的 subpath；那是契約生成物，放 UX 文字等於把被否決的 C 換個位置。
5. **D-L2 的單句假設**：v1 沒發現網站對同一原因有兩種 UI 位置（短提示與整句說明），單一 `LocalizedMessage` 不能直接取代；要 Will 決定收斂或採 b'。
6. **一致性問題**：v1 沒回答「伺服器給原因文字，為何不給名詞」；v2 給出單一規則（誰的判斷的說明）。
7. **伺服器句子寫 client 導覽名稱**：v1 沒指出，這是新的耦合。
8. **不一致的程度**：v1 把 `appeal.go:30` 也列為不一致；那一組是不同 UI 位置，不算錯；真正語意不同的是 `update.go:178` vs admin「只能縮短期間」。
9. **切分**：v1 建議 governance 先搬、其餘「隨模組或 10e 結束」；v2 建議 L 一次搬完全部 proto enum，並把舊 REST 值排除的理由寫成 REAL 約束（沒有 proto 值名）。
10. **3a 細節**：v1 沒指出 3a 新加的 admin 對照用了 `Partial` cast（`measure.ts` diff），也沒列 e2e mock 與無障礙。
11. **數字**：v1 的 17／11／16 與重算的 18／11／17 口徑不同，結論不變；delegation 模組的伺服器 `Label()` v1 沒列。
