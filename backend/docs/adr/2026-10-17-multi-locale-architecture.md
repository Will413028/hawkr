# 多語系架構：買家與賣家介面 zh-TW／en，語系由各介面自己的來源決定，admin 只有 zh-TW

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-17） |
| 日期 | 2026-10-17 |
| 影響範圍 | website（買家頁、賣家中心）、buyer-app、seller-app、`packages/hawkr-api-ts`、`packages/hawkr_api`、`packages/hawkr_auth`、單體的 `internal/httpapi/rpc` 與各模組的伺服器文字、identity（`preferred_locale`）、notification（A8）、`website/docs/legal`、共用文案目錄（刀 L′） |
| 詳細論證 | `docs/plans/2026-09-29-a3-buyer-seller/step10e-i18n-brief.md`（調查、業界基準、約束帳、未查證項；本 ADR 不複製） |
| 修訂其他 ADR | `2026-09-25-codemeta-retirement.md`、`2026-09-25-api-error-model.md`、`2026-10-05-notification-module.md` D1（皆為文末修訂） |
| 相關 | 術語表 `docs/開發規範/使用者面用語術語表.md`；`docs/plans/2026-09-29-a3-buyer-seller/step10e-labels-brief.md`、`step10e-L-brief.md` |

## 脈絡

需求（Will 2026-10-17）：買家面（website 買家頁、buyer-app）與賣家面（website 賣家中心、seller-app）要有英文介面；admin 只有 zh-TW；現在建架構，英文內容之後補；法律文件提供英文參考譯本，以中文為準。

三份已 accepted 的決策都以「只有 zh-TW」為前提，並各自寫了第二語系的重新評估條件，這個條件現在成立：codemeta ADR 的「繼承：只有 zh-TW」與重新評估條件、錯誤模型 ADR 的 `message` 與 `locale` 固定 zh-TW、通知 ADR D1 的「zh-TW 文案集中在一個訊息目錄」。

現況：website 只有 next-intl 的空殼（`NextIntlClientProvider` 與一律回預設訊息的 `i18n/request.ts`），`app/` 沒有 `[locale]`；兩支 App 的 `locale` 寫死；帳號沒有語系欄；伺服器文字是中文字面值（`apperr` 約 449 個呼叫點）；兩個 client 在 `FieldViolation` 沒有 `localized_message` 時退回 `description`。

約束：

- 外部：系統從未上線，沒有相容性約束（根目錄 `AGENTS.md`）。
- 外部：Google 的多語系指引（每個語言一個 URL、不依瀏覽器語言自動導向、只翻外框的頁面體驗差）；買家公開頁要被索引。
- 外部：website 必須用 edge middleware（OpenNext／workerd 不支援 `proxy.ts`）、全站純 ISR；部署時瀏覽器直接打單體，沒有 BFF 可以覆寫 header。
- 外部：Google、LINE 的授權頁語言由對方決定；AMEGO 電子發票與物流單據是第三方或政府格式，維持中文。
- 外部：沒有非工程人員的譯者與營運團隊；開對外帳號要先問 Will。
- 外部：消保法對定型化契約語言條款的效力，本次調查沒有找到法源（簡報 I6）。
- 繼承：舊服務與 CodeMeta 的文字隨模組搬遷退場，現在改它們是丟棄式工作。現在仍成立：搬遷依計畫排程。

## 考慮過的方案與決策

每個決策先列業界基準（同樣條件、假設還沒有程式碼時的做法）。完整比較表與約束帳見簡報對應的 I 節。

### D1 語系從哪來、存在哪（簡報 I1）

基準：每個語言一個 URL，不依瀏覽器語言自動導向（Google）；next-intl 的 `as-needed` 對應 Shopify 的「主語言在根路徑、其他在子目錄」；API 語系由 client 送 `Accept-Language`（AIP-193 允許依請求語系）。

| 方案 | 判定 |
|---|---|
| **website：`as-needed`＋只看 URL**（zh-TW 在 `/`、en 在 `/en`，`localeDetection:false`，有切換器、不導向）；賣家中心用同一套 | **採用** |
| `always`（`/zh-TW`、`/en`） | 否決：根路徑又得選一個導向或語言選擇頁，等於回到「依語言導向」；輸在根路徑問題，不是改動量 |
| 只用 cookie | 否決：Google 明說不要 |
| 子網域 | 否決：Clerk、canonical host、noindex 判斷都要變多主機，單一市場沒有理由 |
| **API：client transport 送單一值 `Accept-Language`**，伺服器用 `language.Matcher` 協商，信心值 No 時退回 zh-TW，回應帶 `Vary: Accept-Language`，gateway 原樣轉送 | **採用**：header 一定在 transport 設（website `lib/connect/transport.ts` 的 interceptor、`hawkr_api` 的 transport、兩支 App 的 Dio），不在 BFF 覆寫 |
| 每個 request 加 `language_code` | 否決（現在）：約 100 支 RPC 改契約；觸發見後果 |
| 讀帳號偏好 | 否決：未登入沒有帳號、同一人兩個 client 用不同語言會錯 |
| **帳號偏好：記「最後一次有效的 UI 語系」**（登入或 App 回前景時，若 transport 送的語系與帳號不同就更新） | **採用**：通知跟著最近使用的語言 |
| 只在使用者明確切換時更新 | 否決：新裝置沿用裝置英文、帳號仍是 zh-TW 時，介面英文、通知中文 |

細則：

- **語系標籤**：資料、URL、訊息檔、API 一律 `zh-TW`、`en`；Flutter 宣告 `Locale.fromSubtags('zh','Hant','TW')`（`zh_Hant_TW`）與 `en`；`<html lang>` 跟 locale；刪 seller-app 的 `zh_CN`。解析規則四端相同：`zh*` → `zh-TW`；其他語言在 `en` 已發佈時 → `en`，否則 `zh-TW`。
- **預設語系 zh-TW**（不是 AIP-193 的 en-US：主市場是台灣）。
- **`published locales`**：每個介面（website 買家頁、website 賣家中心、buyer-app、seller-app）各有一份已發佈語系清單，初始為 `[zh-TW]`。未發佈的語系：路由不產生（website 的 `/en` 在 prod 回 404）、hreflang 不列、App 的語言選項不出現、解析規則視為不存在。
- **發佈門檻（Will 2026-10-17）**：某個介面在英文下看不到任何未翻譯的文字，才能把 `en` 加進該介面的 `published locales`。不做「缺翻譯就退回中文」（較早但中英混雜；這個市場沒有英文使用者在等）。實際會落在相關模組搬進單體、舊 REST 與 CodeMeta 文字退場之後。
- **admin 只有 zh-TW**：不裝 next-intl，吃共用目錄的 zh-TW 常數表；admin RPC 固定 zh-TW 並使用 `staff` 受眾；`staff` 受眾的文字只有 zh-TW。
- **SEO**：hreflang 只列已發佈語系，`x-default` → `/`；`/en` 的平台撰寫頁（首頁、分類、FAQ、賣家指南、法律頁）可索引；`/en` 的 UGC 頁（商品、店家、貼文）先 `noindex` 並排除在 sitemap 外，不用跨語言 canonical 取代；`openGraph.locale`、JSON-LD `inLanguage`、標題模板依 locale。
- **伺服器產生的連結**（email、推播、站內通知）依收件人 `preferred_locale` 加前綴；universal link／App Link 同時接受 `/en/…`。
- **App**：預設跟裝置語言；App 內設定在 Android 用 per-app language，iOS 開「設定」頁；選擇存在裝置。
- **帳號偏好欄位**：`identity.accounts.preferred_locale`（`TEXT NOT NULL`，CHECK 在支援清單內），identity 擁有，A8 讀它；不放 Clerk metadata。一人一個值。
- **Clerk**：只用 OAuth，沒有 Clerk 寄出的信要本地化；`hawkr_auth` 與 website 依 Clerk 錯誤 code 對應自己的文字，不顯示 Clerk 的 `message`。

### D2 文字資源（簡報 I2）

基準：每個 client 一套格式化執行期（website：next-intl；Flutter：`intl`／gen-l10n），key 穩定、每個 key 有譯者說明。

| 方案 | 判定 |
|---|---|
| a. 依 locale 的型別化查表 | 否決：client 有兩套 i18n，`plural`／`select` 要在 TS 與 Dart 各實作 |
| b. TS 生成 next-intl namespace；Dart 生成 ARB 再跑 gen-l10n | 否決：Dart 兩段生成、ARB key 要轉 camelCase、多一個 delegate 碰 `main.dart`／`app.dart` |
| **b′. TS 生成 next-intl namespace；Dart 由生成器直接輸出 Dart（`plural` 呼叫 `Intl.pluralLogic`）；Go 生成函式** | **採用**：格式化執行期仍只有各端一套；窮舉由 Dart `switch` 與 TS `Record` 保證 |
| 目錄停用，各端自擁 | 否決：失去跨端一致 |

- **推翻 L-brief 原 Q2**：key 用**不帶版本的 descriptor 路徑**（例如 `seller.MeasureKind.MEASURE_KIND_WARNING`）；否決含版本的完整名稱（升 `v1` 全部改名、TMS 歷史斷掉）與手寫語意 key（要另一張對照表）。
- **說明**從 proto 值的註解帶入（codemeta ADR D6：註解只寫語意），來源檔可覆寫；受眾（預設／`staff`）寫進說明。
- **來源格式**：帶說明的結構化 JSON（`{ "message", "description" }`），由同一支工具生成 next-intl 用的平面 JSON；不做 `*.meta.json` 側檔。App 自己的文字用 ARB 與 `@key.description`，`required-resource-attributes: true`。
- **分界不變**：只由契約值決定、兩個以上 client 用到的文字進目錄，加上 Connect code 的預設錯誤文字（`common.error.<CODE>`，取代 TS 與 Dart 各一份）；其他文字在各 client 的資源。
- **格式**：ICU MF1 受限子集，只允許 `{arg}`、`select`、`plural`；數字與日期由各端 helper 格式化後當參數；帶數量一律寫 `plural`（含中文）；禁止拼接。
- **完整性以（受眾、語系）為單位**：預設受眾在每個已發佈語系都要完整，每個已發佈語系有通用的「未知」。

### D3 伺服器產生的文字（簡報 I3）

基準：AIP-193——`LocalizedMessage` 可依請求語系，`Status.message` 給開發者。

| 方案 | 判定 |
|---|---|
| go-i18n v2 | 否決：模板是 `text/template`，與 TS／Dart 的 ICU 兩套語法、譯者兩套 |
| `x/text/message`＋gotext | 否決：工具鏈少人用、不是 ICU |
| **ICU 受限子集＋生成的 Go 函式** | **採用**：與目錄同格式、同生成器，參數是 struct 編譯期檢查；`plural` 規則（zh、en）寫在生成器裡，不依賴 `x/text/feature/plural` |
| 字面值查 map | 否決：沒有參數型別，缺翻譯到執行期才知道 |

- 語系：同步回應依請求協商結果；非同步（推播、email）依收件人 `preferred_locale`；收件匣讀取時依讀者語系組字；`LocalizedMessage.locale` 是實際使用的語系。
- 句子放擁有它的模組（`text/zh-TW.json`、`text/en.json`，生成 `text_gen.go`）；`apperr` 不再接受字面值，改收依 tag 延後組字的文字值；CI 擋 `apperr` 與 `FieldViolation` 的中文字面值。
- **開發者訊息**：`Status.message` 與 `FieldViolation.description` 由 `ErrorInfo.reason`＋metadata 自動產生，給開發者看，不隨請求語系變；**同一刀拿掉兩個 client 在 `description` 上的退路**，每個要給使用者看的 `FieldViolation` 必須帶 `localized_message`。
- 通知（A8 的設計輸入）：參數快照只存原始值；email 用 `html/template` 加 locale；投遞紀錄存實際送出的文字與 locale；推播文字由伺服器組好，不用 APNs `loc-key`／FCM `body_loc_key`（文字會分家到原生資源，也違反通知 ADR D1）。
- 範圍外：舊服務與 CodeMeta 的文字隨模組搬遷退場；AMEGO 電子發票維持中文。

### D4 TMS 與流程（簡報 I4）

基準：Weblate、Crowdin 以 git 同步；pseudo-localization 抓未抽出的字與截斷。

| 方案 | 判定 |
|---|---|
| **repo 檔＋CI，之後再接 TMS** | **採用**：零外部帳號，一位維護者、兩個語系，PR 審查就是翻譯流程；英文由 LLM 依術語表起草、Will 審 |
| 現在自架 Tolgee／Weblate | 否決：多一個服務要維運，英文內容還沒開始 |
| 雲端免費方案 | 否決：要開對外帳號（每次問 Will），沒有譯者 |

CI 檢查：硬寫中文的 ratchet **以檔案為單位**（每個檔案的漢字字串字面值只能減少，新檔必須 0；用 AST，不用 regex）；ICU 子集與各語系 key／參數一致；每個 key 有說明；已發佈語系 100% 覆蓋（未發佈只回報）；沒人用的 key 與 drift；pseudo-locale（只在開發與 preview）。

### D5 格式與內容資料（簡報 I5）

基準：時區與語系是兩個獨立因素（W3C）；UGC 自動翻譯須保留原文並標示（Airbnb、Etsy）。

- **時間**：四端業務時間一律台北時間（單一市場、無日光節約時間、Workers 與瀏覽器的 ICU 不同）。website 擴充 `lib/date/taipei.ts` 加 en 樣式表、不用 `Intl`；App 的顯示用 `toLocal()` 改用 `taipei_time.dart`；裝置或瀏覽器時區不是台北時，client 標「台北時間／Taipei time」。只算時間差的 `toLocal()` 不是 bug。
- **金額**：TWD、0 位小數，兩語系都顯示 `NT$`；每端一個 `formatMoney`，website 的不用 `Intl`。
- **UGC 不翻**，容器標 `lang="zh-TW"`；admin 寫給賣家的說明維持中文，賣家端不加「譯文」標示。觸發見後果。
- **平台內容**用 `LocalizedText`（JSONB `{ "zh-TW": …, "en": … }`，CHECK 必有 zh-TW，缺 en 退回 zh-TW）；content 模組先做，分類在步驟 B。
- **檢索**：`pg_trgm` 對中文 `searchable_text` 找不到英文關鍵字，現在不處理；分類搬進單體並有 `LocalizedText` 時併入英文名稱。
- **地址**存代碼、依 locale 顯示名稱，物流單據永遠中文。

### D6 法律文件（簡報 I6）

基準：政府法規英文版寫明以中文為準，但那不是私人間的定型化契約；消保法的有利解釋原則可能讓「以中文為準」對消費者不生效（推論，待律師確認）。

| 方案 | 判定 |
|---|---|
| **中文正本＋英文參考譯本，兩版寫語言條款；同意的對象是中文正本版本** | **採用** |
| 兩種語言同等效力 | 否決：每次修訂審兩份，不一致時沒依據 |
| 不提供英文 | 否決：違反需求 |

- 正本 `website/docs/legal/<doc>.md` 加 `<doc>.en.md`，front matter 寫 `translationOf: <正本版本>`；`en` 已發佈時產生器要求它等於正本現行版本，否則生成失敗。
- 四份買家文件改成 Markdown 正本，走同一支產生器。
- 英文頁最上方固定一行「參考譯本、以繁體中文版為準」並連到中文版。
- 同意紀錄保留 `kind`、`version`（中文正本），加 `displayed_locale` 與 `displayed_translation_version`。
- **風險控制是譯本的品質與版本對齊，不是語言條款本身。** 語言條款對消費者的效力、買家條款對照網路零售業應記載與不得記載事項、隱私告知（個資法第 8 條）對英文使用者的語言，三項列進上線前律師審閱；賣家條款（商業經營者）與買家條款（消費者契約）分開看。

### D7 排程與切分（簡報 I7）

I0（本 ADR、術語表）→ L0（文案核可）→ L′（共用目錄，直接接在現有 `NextIntlClientProvider` 上，不等 `[locale]` 搬遷）→ 3b。`[locale]` 搬遷（I-web-0）在 10d 的 commit 邊界做一個只搬檔的 commit，不等模組搬完。其餘刀（I-app-0、I-server、I-account、I-CI、I-legal、I-content、I-sweep-*）與依賴見簡報 I7 的表。I8 的面向（商店頁與權限說明的本地化、字型、無障礙、測試策略）併入對應的刀。

## 後果

- **接受的代價**：website 要搬 `app/[locale]`、middleware 組合 next-intl 與 Clerk；四端各維護一套 `NT$`／台北時間 helper；`apperr` 的 449 個呼叫點逐模組改；英文發佈晚於架構完成；`/en` 的 UGC 頁在 UGC 有翻譯前不被索引；帳號偏好在共用裝置上可能來回切換。
- **推翻或修改的既有文字**：`step10e-labels-brief.md` 的 D-L1 c 與終局句（觸發成立）；`step10e-L-brief.md` 知會的「不接 next-intl、不做 `Accept-Language`」；L-brief Q2。三份 ADR 以文末修訂處理，計畫書 `2026-09-29-a3-buyer-seller.md` 的對應句改連到本 ADR，`measure.proto` 的 locale 註解改為 admin RPC 固定 zh-TW。
- **待定項**（原留給 L0；Will 2026-10-17 已核可，見 `step10e-L0-copy.md` D6、D7 與術語表）：英文品牌名 `Hawkr`、標題模板 `%s | Hawkr`、預設標題 `Hawkr — A marketplace for local artisans`；英文介面搜尋框的提示（方案 A）；「代行」的英文（delegated session 標籤、on your behalf 句子）。
- **開工前要驗證的未查證項**（簡報「未查證」）：next-intl middleware 在 workerd 與 OpenNext cache interception 下是否正常（I-web-0 第一個 spike；`/` 改寫到 `[locale]=zh-TW` 後仍命中 ISR）；iOS 與 Android 對繁體中文（台灣）解析到的 locale；git 對「改名加內容修改」的合併在搬遷規模下的表現。
- **重新評估的條件**：
  - 出現第三個語系，或某支讀取的內容要用和 UI 不同的語言：該 RPC 加 `language_code`；`LocalizedText` 改成翻譯表 `(entity, field, locale, text)`（三個以上語系或要經 TMS 同步時）。
  - 第一位非工程人員的譯者，或第三個語系：自架 Tolgee 或 Weblate（git 同步，repo 檔仍是來源）；模組的 `text/*.json` 與目錄、client 資源一起交給 TMS。
  - 英文買家比例或回饋顯示需要：評估 UGC 機器翻譯加「顯示原文」，同時解除 `/en` UGC 頁的 noindex。
  - ECMA-402 與 Dart 都有穩定的 MF2 實作：評估取代 ICU MF1 子集。
  - 日後加 email／密碼登入：Clerk 的模板關掉 Clerk 寄送，改由 A8 依 `preferred_locale` 寄。
  - 律師審閱推翻語言條款的假設：重新檢視 D6。

## 修訂：Dart 與 TS 的窮舉都由生成器保證（2026-10-17，刀 L′-1）

D2 b′ 的「窮舉由 Dart `switch` 保證」不精確：protoc_plugin 生成的 enum 是 `class X extends ProtobufEnum`，不是 Dart `enum`，`switch` 不檢查窮舉，名稱還可能被 `omit_enum_names` 拿掉。改為由生成器保證（完整性驗證，實作設計 `docs/plans/2026-09-29-a3-buyer-seller/step10e-Lprime-design.md` §2.2）：生成碼比對值號 `value` 而不是名稱，`_` 分支回「未知」。TS 也一樣：生成的 `keys.gen.ts`、`zh-tw.gen.ts` 以值號為 key，型別是 `Readonly<Partial<Record<number, …>>>`，型別系統不檢查窮舉，完整性同樣由生成器的驗證保證，不靠 `Record` 的型別。

## 修訂：刀 I-web-0 的實作補充（2026-10-18）

`[locale]` 搬遷與路由接線時，實作與本 ADR 原文有下列需要寫清楚的地方：

- **`x-default` 指向「同一頁的預設語系網址」，不是首頁。** Will 2026-10-18 同意：x-default 指向同一頁的預設語系網址（取代 D1 的「x-default → /」）。hreflang 是每一頁各自一組，`x-default` 是該頁的預設語系版本（`/faq` 的是 `/faq`，只有首頁的是 `/`）。實作在 `lib/seo/urls.ts` 的 `languageAlternates`。已發佈語系只有一個時整組不輸出；私有頁與 UGC 頁也不輸出，因為 hreflang 叢集只能列可索引的頁（UGC 頁的 `/en` 版本是 noindex）。
- **「已發佈」和「會路由」是兩份清單。** `published locales` 管 hreflang、sitemap 與索引規則；prod 只路由已發佈的語系（`/en` 回 404，連 `/en/…` 的語系前綴都不認），dev flavor 路由全部支援的語系，未發佈的語系才能在開發與 preview 測。website 的已發佈清單分買家頁與賣家中心兩份（`lib/intl/locales.ts`，`surfaceOfPath` 依路徑分），對應 D1 的「每個介面各有一份」；現在都是 `["zh-TW"]`。
- **`/en` 的 UGC 頁用 `X-Robots-Tag: noindex` 回應**（middleware，與預覽網域的 noindex 同一個機制），不是各頁的 `<meta robots>`：判斷只依語系與路徑，不需要頁面資料，而且對 OpenNext 快取命中的 HTML 一樣生效。UGC 的範圍是 `/products`、`/sellers`、`/feed`（`lib/seo/indexability.ts` 的 `UGC_PREFIXES`）；`/market/products` 當分類頁，可索引。
- **分享圖由根 layout 自己指定。** `app/opengraph-image.tsx` 是檔案慣例，Next 只把它併進「目錄裡有這個檔案」那一層（`app/`）的 metadata；根 layout 搬進 `app/[locale]`、`app/(delegated)` 之後在它下一層，它的 `openGraph` 會整個取代上層解出來的那份，所以沒有自己 `openGraph` 的頁面（帳號、結帳、搜尋、登入、賣家中心、路由內的 404…）原本會失去 `og:image` 與 twitter 的對應標籤。`buildRootMetadata` 現在帶 `images`（同一張圖，加上檔案慣例會產生的寬、高、型別與 alt），`twitter` 刻意不帶：Next 只在 twitter 沒有 `images` 時才從 `openGraph` 補，根的 `twitter.images` 會讓每個沒宣告 `twitter` 的頁面（有自己商品照片的商品頁）的 twitter:image 變成站台圖。與搬遷前唯一的差別是這些頁面的 `og:image` 網址不再帶檔案慣例附的內容雜湊；與搬遷前就自己宣告 `openGraph` 的那些路由一致。`app/global-not-found.tsx` 和檔案在同一層，不指定 `images`，仍由檔案慣例提供。
- **`<html lang>` 的值是語系標籤本身**（`zh-TW`、`en`），取代原本的 `zh-Hant`；代行工作區（`/adm`）與找不到路由的 404（`global-not-found`）固定 `zh-TW`。
- **英文的品牌名、標題模板與預設標題已落進 `lib/seo/metadata.ts`**（`siteNameFor`、`defaultTitleFor`）：站名後綴、`og:siteName`、JSON-LD 的 `name`、商品與店家標題的後綴、首頁的 `{ absolute }` 標題都跟頁面語系走。術語表沒有英文描述，英文的 `description` 暫時沿用中文，由發佈門檻（某介面沒有未翻譯文字才發佈英文）補上。
- **賣家的 JSON-LD（`ProfilePage`）不帶 `inLanguage`**：頁面是訪客的語言，描述的是賣家自己寫的 zh-TW 內容（D5），兩個值都只對一半。
- **`usePathname()` 用 next-intl 的**：預設語系沒有前綴，middleware 把 `/about` 改寫成 `/zh-TW/about`，預先渲染與 ISR 重新驗證都在 `/zh-TW/about` 上跑；Next 的 `usePathname()` 在伺服器端讀到 `/zh-TW/about`、瀏覽器第一次渲染讀到 `/about`，導覽列的首頁版本、作用中的分頁、登入連結的 `redirect` 就會拿不同的 HTML hydrate。
- **zh-TW 的 `<head>` 與搬遷前只差三處：`<html lang>`／JSON-LD 的 `inLanguage`、`og:image` 網址的內容雜湊**（見上）；FAQ 頁 `<body>` 裡的 JSON-LD（`FAQPage`）同樣帶 `inLanguage`，這是刻意保留的，不算 head 的差異。
- **用戶端送的語系來自路由的語系，不讀 DOM**（`<html lang>` 會被瀏覽器翻譯改掉，D1 的 `preferred_locale` 會因此記錯）：瀏覽器端由 `AppProviders` 在 render 時設一次，server 端每個 transport 明確傳 `locale`。
- **站內導覽在這一刀補上**：站內路徑在程式裡不含語系，連結、router、server `redirect`、整頁導覽、`<form action>`、內嵌 boot script 與依瀏覽器網址判斷頁面類型的函式，都經 `website/lib/intl/navigation.ts` 一帶的導覽層（next-intl `createNavigation(routing)`）加上或認得語系前綴，可重複套用（見 `website/AGENTS.md`「多語系路由」）。要保留 `next/link` 自己行為（視窗內預取、沒有轉場）的連結走 `lib/intl/native-link`，它只多了前綴；絕對網址不處理，同源的也一樣；檔案與頁面的分野看明列的清單（與 middleware 的 matcher 同一份），不看副檔名。`tests/navigation-layer.test.ts` 擋繞過出口的寫法；`tests/locale-launch-gate.test.ts` 在 `LAUNCH_BLOCKERS`（`lib/intl/locales.ts`）非空時不讓非預設語系進已發佈清單。
- **不在這一刀**（列在 `LAUNCH_BLOCKERS`，發佈 en 之前補）：語言切換器（已發佈只有 zh-TW，不會出現，設計要 Will 同意）、server 端讀取與快取依路由語系（快取 key 含語系、server 呼叫端送正確的 `Accept-Language`）、`/en` 底下找不到路由的 404 頁（目前固定 zh-TW）、該介面英文文字齊全。`uitext` 目錄依路由切分不擋發佈，另排。

## 修訂：硬寫中文的 ratchet 改成逐筆 baseline（2026-10-22，Will 決定；排程見計畫書 `docs/plans/2026-09-29-a3-buyer-seller.md` 的 I-CI）

D4「CI 檢查」的第一項原文是「**以檔案為單位**（每個檔案的漢字字串字面值只能減少，新檔必須 0）」。改為：

- **baseline 逐筆記錄既有的違規**：每一筆是「檔案＋字面值內容」（同一句在同一個檔出現兩次就記兩筆，不記行號，改行不影響）。出現 baseline 沒有的一筆就失敗，baseline 只能減少；新檔必須 0；用 AST，不用 regex（不變）。
- **理由**：每檔計數分不出哪一筆是新的，同一個檔刪一句、加一句（例如新的三元式文字表）數字不變就過（5d 完成前 design-review 的發現）。ESLint 的 bulk suppressions 也是每檔每規則存計數，官方說明寫了同一個限制：超過時只能把那檔該規則的違規全部報出來，分不出新舊（[ESLint：Introducing bulk suppressions](https://eslint.org/blog/2025/04/introducing-bulk-suppressions/)）。
- **範圍**：會發佈 en 的介面——website 的買家頁與賣家中心、buyer-app、seller-app；admin 只有 zh-TW，不在內。
- **排程**（取代 D7 的「I-CI 和 L′、I-web-0、I-app-0 一起長出來」）：website 先做；兩支 App 和 I-app-0 同一刀，因為 App 在 I-app-0 之前沒有 ARB，新檔沒有合法的地方放文字。某介面的 baseline 歸零後，該介面改成沒有 baseline 的禁止。
- **不在這項**：目錄 enum 的文字只能從共用文案目錄取得，由另一套依條件判斷的守門負責（admin 也在範圍內，且 I-app-0 之後要擋 client 自己資源查詢的重複文字，字面值 ratchet 擋不到），設計見 `docs/plans/2026-09-29-a3-buyer-seller/enum-text-guard-design.md`。

## 修訂：目錄分界（2026-10-22，Will）

- D2「分界不變」改成：只由契約值決定，而且（兩個以上 client 用到，**或**只在某個目錄 enum 成員成立時才出現）的文字進目錄，加上 Connect code 的預設錯誤文字；其他文字在各 client 的資源。
- 「只在某個目錄 enum 成員成立時才出現」：client 程式裡，某段文字所在的分支或資料，必須先確定某個欄位等於目錄 enum 的某個成員才會到達（`if (state == ACTIVE)`、`case ACTIVE`、`{ACTIVE: …}`、`{value: ACTIVE, label: …}`）。這種句子即使只有一個 client 用，也不放 client 資源：它是那個值的說法，放在 client 就和目錄的名詞成為兩個來源。補集（「不是 ACTIVE 時」）不算。
- 理由：依條件判斷的守門（`docs/plans/2026-09-29-a3-buyer-seller/enum-text-guard-design.md`）在 I-app-0 之後連 client 資源查詢也算成文字，這類句子在 client 沒有合法的地方放。
- 第一批：seller-app 停權效果句三句（`seller.Measure.suspension_*`）。website 的停權期間句不屬於這一類（終止合作提早 return 之後的補集），照 L′ §7(d) 留在 client。
- 觸發的更正（2026-10-22，守門第 3 刀完成前審查）：上面「理由」的「I-app-0 之後」只對 App 成立。client 資源查詢算成文字的時間是：website 的 next-intl 查詢（`t("…")`、`useTranslations`）與計畫 I-CI 的 website 那一刀同一刀（從那時起 website 的新文字一律放進 messages）；App 的 ARB 查詢與計畫 I-app-0 同一刀。
