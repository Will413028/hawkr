# 決策簡報：刀 L（共用顯示文字目錄）的未決取捨

**Will 的決定（2026-10-17）：Q1、Q3、Q4、Q5 照建議，並依 `step10e-i18n-brief.md`（多語系）I7 的增補；Q2 由該文第 4 題取代。**

基準：worktree `hawkr-10e`，HEAD `0b9e4675e`（3a 已提交）。只讀調查；盤點 盤點（未入庫） 的主張都回到程式碼查過，不符的列在附錄。
前提：還沒上線、只有 zh-TW、`v1alpha1`；「與現有程式一致」「改動量」「工作量」一律算 SELF-IMPOSED；Will 2026-10-15 的前提：能不能做與原因由伺服器推導，client 只呈現。
已定案、這裡不重談（SSOT `step10e-labels-brief.md`，Will 2026-10-17）：名詞放契約外的共用目錄，生成 TS／Dart／Go，完整、不做依端覆寫；原因文字由伺服器以 `LocalizedMessage` 給；`REVOKE_NOTE` 進目錄；刀 L 插在 3a 與 3b 之間，一次搬完。

## 結論

1. 「同一個值、不同端、不同文字」確實存在，但多數是**漂移**（同一個受眾、措辭不同），只有少數是**受眾視角**（裁定者和當事人看同一個狀態）。建議：漂移收斂成一份文字；視角差異用受眾 key（`staff`）。這是 SSOT 原本就寫好的觸發條件（「加另一個 key，不加覆寫語法」），不算推翻「不做覆寫」。切分的軸是**受眾**，不是 client：website 賣家中心與 seller-app 是同一個受眾。
2. 賣家條款（`website/docs/legal/seller-terms.md` §17.4、§18）用的詞是「措施」「申覆」「解除請求」。client 用的「處置」，以及所有地方的「申訴」，都和條款不一致。條款裡的「申訴」指的是**消費者申訴**（:61、:196）。盤點只看到兩組用詞分歧，實際有三組。
3. 「完整」改成以 enum 為單位：進了目錄的 enum，每個值都要有文字；伺服器擁有文字的 enum（`ErrorReason`、各 `*UnavailableReason`）由規則排除；client 只能經生成的存取取得文字。不建議替約 20 個從沒顯示過的 enum 編新文案。
4. UNSPECIFIED 不給文字（AIP-126：「Default value. This value is unused.」）。`KYCStatus` 的 UNSPECIFIED 在契約裡被當成「尚未申請」（`admin/seller/v1alpha1/seller.proto:832`），這是契約缺陷，在 L 裡改成以欄位存在與否表達。
5. 計畫書 :359 的 `Accept-Language` 是延後的觸發條件（「第二語系或非工程人員改文案時改用 TMS 並依 `Accept-Language` 選語系」），和簡報不衝突，不用改。

---

## Q1 同一個值，給不同受眾不同文字

**建議**：先逐值分成「漂移」與「視角」。漂移收斂成一份文字（屬 UX 文案，要 Will 核可）；視角差異用 `staff` 受眾 key，而且該 enum 的 staff 文字也要完整，不能缺值後退回預設。

**現況**（逐一比對過）
- `KYCField`：admin `admin/src/app/admin/merchants/[id]/kyc-application.ts:207-232` 和 website `website/lib/seller/kyc.ts:441-512`／seller-app `seller-app/lib/features/seller/data/kyc_fields.dart:73-116` 有 10 個值不同；website 和 seller-app 完全相同。
  - 漂移：姓名／真實姓名、出生年月日／出生日期、主要商品類型／主要產品類型、官網／官方網站、身分證（正面）／身分證正面照、公司文件／補充文件。
  - 精確度不同：`BUSINESS_REP_ID_CARD_NO` 在 admin 是「負責人身分證字號」，賣家端是「身分證字號」；隱私頁對賣家也寫「負責人身分證字號」（`website/features/legal/pages/PrivacyPage.tsx:74`），所以可以收斂成 admin 的寫法。
  - `BANK_NAME`／`BANK_CODE`：賣家端兩個值同寫「銀行代碼 / 金融機構」。這不只是文字：`missingItemsOf`（`kyc.ts:531-543`）**靠文字相同來去重**（「同一個名稱只列一次」）。收斂成兩個名詞之後，要改成依表單欄位去重，否則同一個缺項會列兩次。
- `KYCFlowStatus`：admin `admin/src/app/admin/merchants/store-row.ts:49-88` 和 website `website/lib/seller/kyc-flow.ts:148-198`（seller-app 相同）有 7 個值不同。
  - 漂移：填寫中／草稿中、已核准／已通過、已退回／未通過。
  - 視角：`SUBMITTED` 是待審核（後台的待辦）對平台審核中；`BIND_FAILED` 是待重新綁定（員工要做的事）對「PayUni 建檔失敗，處理中」。
  - 刻意的粗細：賣家端把 `BINDING`、`FORWARDED`、`UNDER_REVIEW` 都寫成「PayUni 審核中」，admin 分得比較細。
- `MeasureRequestState`：admin 是待裁定／成立／駁回／已結案（`admin/src/app/admin/measures/measure-inbox.ts:82-87`）；seller-app 是 `$subject審核中／已通過／已駁回`（`seller-app/lib/features/seller/data/measure_display.dart:190-199`）。這是裁定者和申請人的視角差異。
- `StoreStatus`：**不是**同一個值不同文字（盤點錯誤，見附錄）。

**業界基準**
- Lokalise：同一個 key 在同一個語言下，不能在 iOS 和 Web 有不同翻譯〔VERIFIED，`step10e-labels-research.md` §5〕。所以平台（client）不是切分文字的軸。
- Locize：「Creating new messages is cheaper than trying to manage shared messages」；建議依情境用不同的 key〔VERIFIED，[locize key naming](https://www.locize.com/blog/guide-to-i18n-key-naming)〕。
- Shopify Polaris 把文案分成「Shopify-to-merchant」（admin）和「merchant-to-customer」（結帳、出貨信）兩類〔VERIFIED，搜尋摘要；頁面已轉址到 [shopify.dev/docs/api/polaris](https://shopify.dev/docs/api/polaris)〕。訂單狀態在 admin 是 Unfulfilled／Fulfilled，在顧客的訂單狀態頁是 Confirmed／On its way〔INFERENCE：來源是社群文章，[Shopify Community](https://community.shopify.com/t/updating-order-status-page-from-confirmed-to-on-its-way-for-all-fulfilled-orders/287487)〕。也就是同一個狀態，依受眾給不同文字。
- GitHub Primer：「Refer to the same actions, sections, labels … the same way across the product. But don't sacrifice clarity over consistency」〔VERIFIED，搜尋摘要，[primer.style/foundations/content](https://primer.style/foundations/content)〕。用詞要一致，但清楚優先。
- Stripe 的 Dashboard 和 Checkout 怎麼對應：沒找到官方對照〔未查證〕。

**為什麼現有機制存在**：codemeta ADR D5 讓各端各自擁有文字，假設各端本來就不同。現況證據是：一部分確實不同（視角），大部分只是各自手寫產生的漂移。

| 選項 | 做法 | 取捨 |
|---|---|---|
| a. 全部收斂成一份 | 每個值只有一個詞 | 規則最簡單；但後台的「待審核」和賣家看到的「平台審核中」本來就是不同的話，硬合會讓其中一方讀錯 |
| **b. 漂移收斂＋視角用 `staff` key（enum 層級完整）** | 預設受眾＝當事人（賣家、買家），`staff`＝後台；某個 enum 一旦有 staff 文字，每個值都要有 | 和 Lokalise、Polaris 的模型一致；不會出現「缺值就退回預設」這種實質覆寫 |
| c. 依 client 覆寫 | `@admin` 覆寫預設 | 同一個受眾的兩個 client 可能被寫成不同文字；SSOT 已否決 |
| d. 照現狀把差異搬成 key | 不審文案，10＋7 個值機械照搬 | 把漂移固化成「刻意的差異」 |

**約束帳**
| 約束 | 判定 | 來源 |
|---|---|---|
| 裁定者和當事人看同一個狀態，要的資訊不同 | REAL | `store-row.ts:61-66` 對 `kyc-flow.ts:165-172` |
| 版面與 UX 文案改動要先取得同意 | REAL（repo 規則） | 根目錄 `AGENTS.md`「UI 改動」 |
| 缺項去重依賴文字相同 | REAL（行為），但它是設計缺陷，不是限制 | `kyc.ts:531-543` |
| 沿用現有文字、改動量 | SELF-IMPOSED | — |

**建議（只依 REAL 調整）**：採 b。受眾目前只分兩種：預設（當事人）與 `staff`；買家和賣家要分開，等到第一次出現兩者文字不同時再分。去重改成依欄位。
**終局與觸發**：出現第三種受眾（例如物流夥伴）時加一個受眾 namespace；交給 TMS 時，受眾就是 TMS 的 key 前綴。

## Q2 key 方案與目錄格式

**建議**：key 用 descriptor 的完整名稱 `<package>.<Enum>.<VALUE>`；來源檔是兩層 JSON（受眾 → 平面 key → 字串）；值用 ICU MessageFormat 1，只在需要參數或 `select` 時才用語法；不處理複數。

**現況**：`FoodRegistrationState` 在 `seller/v1alpha1/seller.proto:363` 和 `admin/seller/v1alpha1/seller.proto:755` 各定義一次，值名相同，所以 key 必須帶 package〔VERIFIED〕。TS 的成員名有時去前綴（`MeasureKind.WARNING`）、有時不去（`KYCField.KYC_FIELD_REAL_NAME`），Dart 一律用完整值名；所以 key 只能取自 descriptor，不能取自任何一端的生成碼。

**業界基準**
- Lokalise 上傳時會把巢狀 key 攤平；巢狀以 2–3 層為宜〔VERIFIED，搜尋摘要，[Lokalise blog](https://lokalise.com/blog/translation-keys-naming-and-organizing)、locize〕。
- Flutter ARB 和 next-intl 都用 ICU MF1 的 `select`，而且 `other` 是必要分支〔VERIFIED，research §5〕。
- MF2 已內建於 ICU4C／ICU4J，ECMA-402 是「soon」；沒有 Dart 實作〔VERIFIED，[messageformat.unicode.org](https://messageformat.unicode.org/)；Dart 部分是 INFERENCE〕。
- zh 在 CLDR 只有 `other` 一種複數類別〔INFERENCE，CLDR 常識，未在本次重抓〕。

| 選項 | 取捨 |
|---|---|
| **完整名稱（含版本）** | 和 descriptor 一一對應、不會撞；升 v1 時 key 跟著改名（生成器做） |
| 去掉版本段（`hawkr.seller.MeasureKind.WARNING`） | 升版時 key 不變；但要自訂「去前綴」規則，`v1alpha1` 和 `v1` 並存時會撞 |
| 語意 key（`measure.kind.warning`） | TMS 風格；需要另一張對照表，等於多一份要對齊的資料 |
| 格式：ARB | Flutter 原生；TS 端要轉；只在第二語系時才有價值 |

**約束帳**：只有 zh-TW（REAL）；需要帶參數的句子（REAL，見 Q3）；TMS 能直接匯入（終局，不是現在的約束）；與 twaddr 的格式一致（SELF-IMPOSED）。

**建議**：用完整名稱。`select` 用來處理「值的組合」（例如 Q3 的「合作已終止」），參數一律具名並由生成器加上型別（TS 函式、Dart 函式、Go 函式）。
**終局與觸發**：第二語系時，由生成器輸出 ARB 與 next-intl messages；MF2 在 ECMA-402 與 Dart 都穩定後再評估。

## Q3 哪些文字進目錄

**建議**：目錄收錄「**只由契約值（可以是幾個值的組合）決定、而且兩個以上 client 都用到**」的文字：enum 名詞、`KycFlowCopy`、`MeasureRequestState`×kind 的句子、「合作已終止」；只在單一畫面使用的版面文案留在 client。

**現況**
- `REVOKE_NOTE` 不是另一套句子：它的兩個值和 admin 的 `RevokeReason` 名詞一字不差（`website/features/seller/measures/measureDisplay.ts:81-82` 對 `admin/src/app/admin/merchants/[id]/measure.ts:302-303`）。也就是說，它就是 `RevokeReason` 的名詞，只是賣家端只顯示其中 2 個值，不用另開 key。
- `KycFlowCopy`（`kyc-flow.ts:90-198`、seller-app `kyc_flow.dart`）：每個值一組 action、banner、gate 文字，帶 `subject` 參數，website 和 seller-app 各手抄一份、內容相同。
- 「合作已終止」是 ACTIVE＋TERMINATION 的組合，三端各抄一份（盤點 §1a，web `measureDisplay.ts:204`）。
- seller-app 的 `$subject審核中`（`measure_display.dart:190-199`）是請求種類加狀態的組合。

**業界基準**：Locize 主張每個 UI 實例一個 key，依功能分 namespace〔VERIFIED〕；ICU 的 `select` 就是為「依值選文字」設計的〔VERIFIED，FormatJS〕；i18n 一般做法不拼接字串，用帶參數的模板〔INFERENCE〕。

| 選項 | 取捨 |
|---|---|
| a. 只收名詞 | 目錄最小；`KycFlowCopy` 和「合作已終止」仍在兩到三端手抄，正是目錄要消除的漂移 |
| **b. 收「只由契約值決定、兩端以上使用」的文字** | 有一條可機械判斷的規則；組合用 `select`，例如 `…MeasureState.MEASURE_STATE_ACTIVE: "{kind, select, TERMINATION{合作已終止} other{生效中}}"` |
| c. 所有 UI 文案 | 等於導入完整 i18n 執行期；SSOT 已判定第二語系前不做（c'） |
| d. 伺服器回 `KycFlowCopy` | gate 文字說明「為什麼不能做某件事」，形式上像原因；但它只由 enum 值決定、不依資源此刻的狀態，依 SSOT 的單一規則（「文字是誰的判斷的說明」）屬於目錄 |

**約束帳**：兩端相同的句子要一致（REAL，現況就是手抄）；`subject` 是 client 的動作名稱（REAL，client 擁有導覽與動作）；目錄要小（SELF-IMPOSED）。
**建議**：採 b。按鈕文字和 banner 的語氣（tone）這類非文字欄位留在 client；`KycFlowCopy` 只把文字欄位搬進目錄。
**終局與觸發**：第二語系時，整個 client 改用 i18n 資源，目錄就是那批資源的一部分。

## Q4 完整規則、從沒顯示過的 enum、UNSPECIFIED

**建議**：完整性以 enum 為單位（進了目錄的 enum 每個非 UNSPECIFIED 值都要有文字）；用規則排除伺服器擁有文字的 enum；CI 擋下 client 手寫的 proto enum 文字表；UNSPECIFIED 不給文字；`KYCStatus` 的「尚未申請」改用欄位存在與否表達。

**現況**
- 約 20 個 enum 沒有任何顯示文字，只用來分支或完全沒引用（盤點 §1b）。抽查：`SellerCapability` 只出現在 `can(...)` 分支（`seller-app/lib/features/wallet/presentation/wallet_screen.dart:96` 等）；`ProspectStatus`、`OutreachStatus` 在四個 client 都是 0 個檔案（`rg -lw` 排除 gen、test）。
- UNSPECIFIED 有文字的地方：
  - `admin/src/lib/food-registration.ts:40`「未知」：這是 fallback。
  - `admin/src/lib/food-registration.ts:107`「尚未申請」：契約明寫「UNSPECIFIED 表示店家還沒有申請」（`admin/seller/v1alpha1/seller.proto:832`），也就是拿零值承載業務意義。
  - `admin/src/app/admin/app-versions/page.tsx:32`「未指定」：也是 fallback。
- SSOT D-L1 原文是「生成器以 buf image 驗『每個非 UNSPECIFIED 的值都有文字』」。字面上涵蓋全部 60 個 enum，包含 `ErrorReason` 和原因 enum；但這和 D-L2「原因文字由伺服器給」互相矛盾。

**業界基準**：AIP-126 示範的零值註解是「Default value. This value is unused.」〔VERIFIED，[google.aip.dev/126](https://google.aip.dev/126)〕；要表達「沒有」，用欄位是否存在（proto3 `optional`）是常見做法〔INFERENCE〕。

| 選項 | 取捨 |
|---|---|
| a. 字面完整（60 個 enum 全收） | 要替沒人看過的值編約 100 條新文案；原因 enum 會在目錄和伺服器各有一份文字，和 D-L2 矛盾 |
| **b. 以 enum 為單位的完整＋規則排除＋CI 擋手寫表** | 只收真的會顯示的；「被排除」由結構規則決定（`ErrorReason`、出現在 `*Unavailability.reason` 的 enum），不是手寫清單；漏收由 CI 擋（client 碼中不得有以 proto enum 為 key 的文字表、Dart 不得有 enum→字串的 switch） |
| c. 手寫的「不顯示」清單 | SSOT 已拿掉 |

**約束帳**：原因文字由伺服器給（REAL，Will 2026-10-15、SSOT D-L2）；admin 專用的 enum 不進 Dart（REAL，`packages/hawkr_api/buf.gen.yaml` 的 exclude）；目錄要「看起來完整」（SELF-IMPOSED）。
**建議**：採 b，並在 L 裡把 `kyc_status` 改成 `optional`（或明確的 `NOT_APPLIED` 值），讓 admin 依欄位存在與否顯示「尚未申請」。不認得的值和 UNSPECIFIED，在 client 一律顯示同一個通用「未知」（目錄裡的一個 common key）。
**終局與觸發**：某個 enum 第一次需要顯示時，在目錄加上它的整組文字，CI 會要求補齊。

## Q5 正式用語

**建議**：以賣家條款為賣家面用語的唯一來源：「措施」「申覆」「解除請求」。staff 面用同一套詞，只在視角上不同（Q1）。「處置」「申訴（指賣家的請求時）」「補正（對使用者顯示時）」都退場；導覽名稱「處置與申訴」也跟著改，要 Will 核可。

**現況**
- 條款：「措施」15 次；「申覆」用在 §17.4、§18（:176、:184）；「解除請求」用在 :176；「處置」只出現 1 次，在個資事故通知（:139），和治理無關；「補正」只出現在 KYC 的「補正方式」（:50）；「申訴」4 次，都指消費者申訴或退換貨（:61、:106、:111、:196）。
- 使用次數（`rg -c`，排除測試）：

  | 詞 | website | seller-app | admin | Go seller 模組 |
  |---|---|---|---|---|
  | 處置 | 49 | 128 | 29 | 7 |
  | 措施 | 29 | 11 | 131 | 428 |
  | 申訴 | 76 | 127 | 90 | 80 |
  | 申覆 | 0 | 0 | 0 | 19（多為註解） |
  | 補正 | 18 | 2 | 1 | 67 |

- 設計書 32 用「措施」「申訴」「補正（解除請求）」（:33、:146、:155）；伺服器的錯誤句 `model/model.go:120` 還寫了「請至「處置與申訴」提出申訴」。

**業界基準**：Primer 要求同一件事在整個產品用同一個詞〔VERIFIED〕；介面用詞和約束使用者的條款一致，使用者才對得上自己依哪一條行使權利〔INFERENCE：DSA 第 20 條的內部申訴機制要求說明救濟方式，條款與介面不一致會削弱這個說明；未查證任何具體判決〕。

| 選項 | 取捨 |
|---|---|
| **a. UI 對齊條款（措施／申覆／解除請求）** | 條款是有約束力的文件；「申訴」在條款裡已經指消費者申訴，賣家介面沿用會讓同一個詞有兩種意思；代價是改 client 文案與導覽名稱 |
| b. 條款對齊 UI（改條款用「處置」「申訴」） | 上線前條款也可以改；但法律文字的修訂需要獨立審閱，而且「申訴」和消費者申訴撞名的問題仍在 |
| c. 賣家面「處置」、admin 面「措施」（盤點 Q-H 的建議） | 兩方對同一筆紀錄用不同的詞，客服溝通時對不上，條款也對不上 |

**約束帳**：條款是賣家契約文字（REAL，上線前可改，但改條款是法律文件變更，不在 L 的範圍）；UX 文案要同意（REAL）；client 的現行用詞（SELF-IMPOSED）。
**建議**：採 a。目錄裡的名詞（`MeasureRequestKind`：申覆／解除請求）與伺服器句子（D-L2）用同一套詞；Go 的內部識別字與註解不強制改，但對使用者顯示的字串一律走這套詞彙。
**終局與觸發**：條款修訂時，同一次變更裡更新詞彙與目錄。

## Q6 套件位置與生成器語言

**建議**：Go 寫的生成器讀 `buf build` 的 image，不 import Go 的生成碼；來源檔與生成器放在 `backend/internal/platform/uitext/`（比照 twaddr），輸出 `uitext_gen.go`、`packages/hawkr-ui-text-ts`（`@hawkr/ui-text`）、`packages/hawkr_ui_text`（排除 `hawkr.admin.*`）。Go 的 package **不能叫 `copy`**：會遮蔽內建函式 `copy`。

**現況**：先例有三種。
- twaddr：Go，`//go:build ignore`，一支程式輸出三份（`backend/internal/platform/twaddr/generate.go`）。
- legal：TS 輸出 Go（`website/scripts/generate-legal.ts:8,34` → `agreement_gen.go`）。
- image presets：mjs。

讀 descriptor 的先例是 `backend/pkg/cmd/protonumericlint`（從 stdin 讀 FileDescriptorSet）。`internal/modules` 沒有任何檔案 import `httpapi/gen`（`rg -l 'httpapi/gen' backend/internal/modules` 無結果），模組的 enum 是 string 型別，值是去前綴的值名（`model/measure.go:13-17`）。

**業界基準**：從 proto 推導程式碼的標準做法是 protoc／buf plugin（例如 protoc-gen-es）讀 `CodeGeneratorRequest` 或 image〔INFERENCE〕。

| 選項 | 取捨 |
|---|---|
| **a. Go 程式讀 `buf build -o -` 的 image** | 不依賴 Go 生成碼是否最新；有 protonumericlint 先例；drift CI 要裝 buf（版本鎖在 `api/.buf-version`） |
| b. Go 程式 import `protoregistry.GlobalFiles` | 不需要 buf CLI；但必須先 `make connect`，順序依賴容易漏 |
| c. 寫成 buf plugin | 最正統；多一個要維護的 plugin 執行檔，目前沒有需求 |
| d. TS／mjs 生成器 | 和 legal 的先例一致；Go 輸出要由 TS 產生，不比 a 好 |

**約束帳**：模組不能 import `httpapi/gen`（REAL，分層規則）；Go 輸出現在就要（REAL，D-L2 的伺服器句子會嵌入名詞）；沿用 twaddr 的位置（SELF-IMPOSED）。
**建議**：採 a。Go API 依 enum 生成以去前綴字串為 key 的查詢函式（例如 `uitext.MeasureKind(string(v)) (string, bool)`）。每個用到的模組加一個資料集測試，走訪自己的 `XxxValues()` 斷言每個值都查得到，因為跨套件的完整性在編譯期做不到。伺服器依 RPC 面選受眾：admin RPC 的句子用 `staff`。
**終局與觸發**：生成器需要讀 proto option，或要給第三方使用時，改寫成 buf plugin。

## Q7 子 commit 切分

**建議**：同一次 push、每個 commit 都綠：
- L0：Will 核可文案。把目錄 JSON 本身當文案稿審，包含 Q1 的收斂與 Q5 的詞彙。
- L1：目錄、生成器、兩個套件、Go 輸出、CI、ADR 文末修訂。
- L2：賣家端三個原因 enum 改成 D-L2 形狀，契約與四端同一個 commit。
- L3：`kyc_status` 改成 `optional`，契約與 admin 同一個 commit。
- L4：Go 改用目錄，刪 `Label()` 與 regex 測試。
- L5：website；L6：seller-app；L7：admin。
- L8：刪 proto 的「標籤」註解並重新生成三端、加上手寫表的 CI 檢查。

**現況**：根目錄 `AGENTS.md`「改契約時，同一次變更內更新唯一來源與所有消費端」。盤點的 L1–L7 沒有 L0 與 L3。L8 只刪註解，但生成碼含註解，所以要和三端重新生成放在同一個 commit。

| 選項 | 取捨 |
|---|---|
| **a. 依層切 commit、一次 push** | 每個 commit 範圍單純、可以分開審；commit 之間短暫有兩份文字，push 時已經消失 |
| b. 一個大 commit | 沒有中間狀態；約 60–90 個檔案，難審 |
| c. 依 enum 分批 | 會留下「有的在目錄、有的手抄」兩套規則，SSOT 已否決 |

**約束帳**：契約變更要同一個 commit（REAL，repo 規則）；文案要同意（REAL）；分 commit 的方式（SELF-IMPOSED）。
**建議**：採 a。L0 是阻擋性的前置條件：文案沒定，L5–L7 只能照搬，等於選了 Q1 的 d。

---

## Will 要回答的問題

1. **Q1**：同意「漂移收斂＋視角差異用 `staff` key（以 enum 為單位完整）」？漂移的收斂文字在 L0 交給你審。
2. **Q4**：把 SSOT 的「每個非 UNSPECIFIED 值都有文字」改為「以 enum 為單位完整＋用規則排除伺服器文字的 enum＋CI 擋手寫表」？同時把 `kyc_status` 改成 `optional`？
3. **Q5**：賣家面與 staff 面都對齊條款的「措施／申覆／解除請求」，導覽名稱「處置與申訴」跟著改？或改成條款對齊 UI（另開法律文字修訂）？
4. **Q3**：目錄收錄「只由契約值決定、兩端以上使用」的文字（含 `KycFlowCopy`、「合作已終止」用 `select`）？
5. **Q2／Q6／Q7**（照建議走即可，有異議再說）：key 用含版本的完整名稱、兩層 JSON＋ICU MF1；Go 生成器讀 buf image，套件叫 `uitext`；切分 L0–L8、一次 push。

## 知會（SSOT 已定、或不需要決定）

- 不做依 client 覆寫、不做手寫的「不顯示」清單；某個 client 不顯示某個值，是該 client 的程式碼（SSOT D-L1）。
- `REVOKE_NOTE` 進目錄：它就是 `RevokeReason` 的名詞，不用另開 key。
- 原因文字由伺服器以 `LocalizedMessage` 給，和錯誤用同一個函式產生；伺服器句子不寫 client 的導覽名稱，所以 `model/model.go:120` 要改（SSOT D-L2）。
- 不接 next-intl、不做 `Accept-Language`；計畫書 :359 寫的是延後的觸發條件，和簡報一致。
- 舊 REST 的 CodeMeta 值與 delegation 的 `Label()` 不在 L 的範圍（SSOT 切分）。
- `repeated enum` 換成 `repeated message` 時用新欄位號加 `reserved`（3a 的先例 39–42）。
- 範圍外但違反 2026-10-15 前提的發現，請另排一刀：admin 自己寫「不能做」的原因句，包括 `store-row.ts:121-127`「已關閉，無法代行」、`store-detail.ts:117-119`「店家已關閉，不能核准或退回身分認證」。依前提，這些應由伺服器推導。

## 附錄：盤點（盤點（未入庫））的錯誤與遺漏

1. `StoreStatus`「admin 內部兩處不同」不成立：兩處的名詞都是「已關閉／等待刪除」（`store-row.ts:103,109`、`store-detail.ts:145,147`）；「已關閉，無法代行」（`store-row.ts:125-127`）是不能代行的原因句，屬於上面「知會」的最後一項。
2. 盤點把 `REVOKE_NOTE` 歸為「句子」：它的文字和 admin 的 `RevokeReason` 名詞一字不差（`measureDisplay.ts:81-82` 對 `measure.ts:302-303`）。
3. Q-I 說計畫書 :359 和簡報「互相牴觸」：不成立，那一句是延後的觸發條件。
4. `KYCField` 在 admin 的位置是 `kyc-application.ts:207-232`，不是 `:8-33`。UNSPECIFIED 的行號偏一行（`food-registration.ts:40,107`、`page.tsx:32`）。
5. 遺漏「申訴」對「申覆」：條款用「申覆」，四端都用「申訴」，而條款的「申訴」指消費者申訴（Q5）。
6. Q-H 建議賣家面用「處置」：和條款牴觸；「處置」在條款中只出現在個資事故通知。
7. 遺漏 `KYCStatus` 的 UNSPECIFIED 是契約定義的業務意義（`admin/seller/v1alpha1/seller.proto:832`），不只是 client 的文字（Q4）。
8. 遺漏 `missingItemsOf` 以文字去重（`kyc.ts:531-543`）：收斂 `BANK_NAME`／`BANK_CODE` 的文字會改變行為（Q1）。
9. Q-D 建議的 `platform/copy`：Go 的 package 名稱 `copy` 會遮蔽內建函式。
10. Q-F 的豁免方案被描述成「明列不顯示的另一種形態」；用結構規則排除（伺服器文字的 enum）加上 CI 擋手寫表，不需要任何清單（Q4）。
11. 切分漏掉兩個前置：文案核可（L0）與 `kyc_status` 的契約修正（L3）。
