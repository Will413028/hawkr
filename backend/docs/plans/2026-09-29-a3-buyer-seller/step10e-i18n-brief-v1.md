# 決策簡報：多語系架構（zh-TW＋英文）與刀 L 的重排（hawkr）

基準：worktree `hawkr-10e`，HEAD `0b9e4675e`（3a 已提交）。只讀調查。repo 盤點與網路來源的筆記在同目錄 `step10e-i18n-research-repo.md`（§R）與 `step10e-i18n-research-web.md`（§1–12，V＝讀過原頁、T＝本機實測、I＝推論或搜尋摘要）。
新需求（Will 2026-10-17）：買家面（website 買家頁、buyer-app）與賣家面（website 賣家中心、seller-app）要有英文介面；admin 只有 zh-TW。**現在就建多語系架構**，英文內容之後再補：一開始只有 zh-TW 有內容，但任何地方都不能假設只有一個語系。法律文件（條款、隱私）提供英文參考譯本，以中文為準。
前提：系統還沒上線，沒有相容性約束；「與現有程式一致」「改動量」「工作量」一律算 SELF-IMPOSED；Will 2026-10-15 定的前提：能不能做、為什麼不能做都由伺服器推導，client 只負責呈現。
本簡報**推翻**了幾件已定案的事：`step10e-labels-brief.md` 的終局句與 c' 的否決理由、`step10e-L-brief.md` 知會的「不接 next-intl、不做 `Accept-Language`」，以及三份 ADR 裡「只有 zh-TW」的繼承約束。細節見 I7。

## 結論

| # | 題目 | 建議（一句） |
|---|---|---|
| I1 | 語系怎麼選、存在哪 | website 的語系由 URL 決定：zh-TW 在 `/`、英文在 `/en`（next-intl `as-needed`），不依瀏覽器語言自動導向；App 預設跟裝置語言，App 內可以另選；推播與 email 等非同步文字依**帳號的 `preferred_locale`**；每個 API 請求都帶明確的 `Accept-Language`，由伺服器協商；admin 固定 zh-TW |
| I2 | 文字資源 | 每個 client 只用一套 i18n 執行期（website：next-intl；App：gen-l10n／ARB）。共用目錄改成**多語系的來源檔**，生成進各端的執行期：TS 產生 next-intl 的 namespace，Dart 產生套件自己的 ARB 與 delegate，Go 產生依 locale 的函式。key 改成**不帶版本的穩定 key**，翻譯說明由 proto 註解自動帶入。格式用 ICU MF1 的受限子集；中文原文凡是帶數量，一律寫成 `plural` |
| I3 | 伺服器產生的文字 | 同步回應依請求 locale；非同步（A8 的推播、email）依帳號偏好；收件匣在讀取時依讀者的 locale 組字。句子放在擁有它的模組，用 ICU 來源檔，由同一支生成器產生有型別的 Go 函式。`LocalizedMessage.locale` 改成實際用的 locale。開發者看的 `message` 改由 `ErrorInfo` 自動產生，不再另外寫 |
| I4 | TMS 與流程 | 現在不開 TMS 帳號；repo 裡的檔案都用 TMS 能直接匯入的格式。CI 擋：新增的硬寫中文、ICU 語法、各語系 key 與參數一致、每個 key 必須有說明、已發佈的語系覆蓋率必須 100%；開發版有 pseudo-locale。英文由 LLM 依術語表起草、Will 審 |
| I5 | 格式與內容資料 | 時間一律以 Asia/Taipei 顯示，不隨語言或裝置時區改變；幣別永遠是 TWD、0 位小數，各端各有一個格式化 helper。UGC 不翻譯、標上 `lang`；平台撰寫的內容（分類、方案名、banner、食譜）用 `LocalizedText`，在各自的模組做 |
| I6 | 法律文件 | 正本是中文 Markdown，英文是 `*.en.md` 參考譯本並標明它翻的是哪一版；兩份都寫「以中文版為準」條款；買家的四份手寫 TSX 改成走產生器；同意紀錄加 `displayed_locale` |
| I7 | 刀 L 與計畫 | 先出 I0（ADR＋術語表）。L 改為 L′，一開始就是多語系形狀。基礎建設切成 I-server、I-account、I-web-0、I-app-0、I-CI，其中 website 的 `[locale]` 搬遷要和 10d 排開。字串抽取依區域分批，由 CI 的 ratchet 收斂。英文的**發佈**以「該介面看不到任何未翻譯文字」為門檻，實際上會落在相關模組搬進單體之後 |

---

## I1 語系怎麼選、怎麼保存

**建議**：每個介面都有一個清楚的「語系從哪來」：website 看 URL、App 看裝置設定或 App 內選擇、非同步訊息看帳號偏好、API 看請求 header。admin 固定 zh-TW。

### 現況
- website 裝了 next-intl 4.13，但只有空殼：`website/lib/intl/config.ts:1-12` 定義 `locales=["zh-TW"]`；`website/i18n/request.ts:4-10` 一律回預設訊息；沒有 `routing.ts`，`app/` 也沒有 `[locale]` 段，只用 route group（§R1）。
- `website/app/layout.tsx:79` 與 `admin/src/app/layout.tsx:40` 寫的是 `lang="zh-Hant"`，和 config 的 `zh-TW` 不是同一個標籤。
- `website/middleware.ts:15-18`：OpenNext／workerd 不支援 Next 16 的 `proxy.ts`，所以用 edge middleware 跑 Clerk、canonical 308 與 noindex。
- SEO：`website/lib/seo/urls.ts` 是 URL 的唯一來源；`lib/seo/metadata.ts:186-192` 只有 canonical，沒有任何 hreflang；`:123,196` 的 `openGraph.locale` 寫死 `zh_TW`。
- seller-app 的 `lib/app.dart:318-323` 宣告 zh_TW、zh_CN、en_US，卻把 `locale` 寫死成 `zh_TW`；buyer-app 的 `lib/main.dart:389` 完全沒有設定。兩支 App 都沒有語言設定頁。
- 沒有任何請求帶 locale：Dart 的 transport（`packages/hawkr_api/lib/transport.dart`）與兩支 App 的 Dio 都沒有。website BFF 只把 `Accept-Language` 當快取 key（`app/api/v1/[...path]/route.ts:86-88`）；後端完全不讀（§R4）。
- `identity.accounts`（`backend/internal/modules/identity/db/schema.sql:28-41`）、`buyer.buyers`、`seller.stores`、舊的 `device_tokens` 都沒有語系欄位。

### 業界基準
- Google 建議每個語言用不同的 URL，不要用 cookie 或瀏覽器設定切換同一個 URL 的內容。Googlebot 不送 `Accept-Language`，預設 IP 在美國。hreflang 的每個版本都要列出自己與其他所有版本〔VERIFIED，§2〕。不要依語言自動導向〔INFERENCE，記憶中同一頁有寫，本次沒有重讀〕。
- next-intl 有三種前綴：`always`、`as-needed`（預設語系不加前綴）、`never`（要搭配網域，而且會關掉 alternate links）；偵測順序是前綴 → cookie → `accept-language` → 預設；`localeDetection:false` 表示完全依 URL；middleware 會自動輸出 hreflang 的 `Link` header〔VERIFIED，§1〕。Clerk 文件示範在 `clerkMiddleware` 裡 `return intlMiddleware(req)`〔VERIFIED〕。
- Shopify 每個已發佈的語言都有自己的 URL，結帳與通知信用顧客的語言〔VERIFIED，§8〕；Amazon 依帳號保存語言偏好〔INFERENCE〕。
- Flutter：裝置語言不在 `supportedLocales` 時，先找語言碼相同的，再退到清單第一個；繁中台灣的完整寫法是 `zh_Hant_TW`〔VERIFIED，§3〕。Android 13 有每個 App 各自的語言設定（AppCompat 1.6+ 可向下相容）〔VERIFIED〕；iOS 在 bundle 宣告多個 localization 後，「設定」裡就有該 App 的語言〔INFERENCE〕。
- API：AIP-193 說 `LocalizedMessage`「may use a locale specified in the request」，沒指定時用 en-US；AIP-143 規定語言欄位叫 `language_code`；Places 用 `languageCode` 請求欄位〔VERIFIED，§5〕。Connect 沒有 locale 的慣例〔INFERENCE〕。
- 語系標籤：W3C 建議「盡量短」；Go 的 matcher 會把 `zh-Hant-TW`、`zh-Hant` 都精確對到 `zh-TW`，`zh-HK` 是 High〔VERIFIED／T，§5、§12〕。

### 為什麼現有機制存在
繼承自公司時期「只有 zh-TW」的約束：codemeta ADR `:38`、錯誤模型 ADR `:32`。next-intl 是先裝了、還沒用上的骨架。

### 選項：website 的 URL

| 選項 | 做法 | 取捨 |
|---|---|---|
| **a. `as-needed`＋URL 是唯一依據** | zh-TW 在 `/…`、英文在 `/en/…`；`localeDetection:false`；有語言切換器；`Accept-Language` 偏好英文時在 client 顯示「View in English」提示，不自動導向 | 現有 URL、canonical、外部連結都不變；符合 Google 的建議；Googlebot 永遠先看到 zh-TW |
| b. `always` | `/zh-TW/…`、`/en/…` | 兩個語系對稱；但每個現有 URL 都要 308，canonical 與 sitemap 全部改，換到的只有對稱這一點 |
| c. 只用 cookie（`never`） | URL 不變，語言存在 cookie | Google 明說不建議；hreflang 無法標；買家頁要 SEO，否決 |
| d. 子網域或另一個網域 | `en.hawkr…` | Clerk、Access、canonical host、noindex 的邏輯都多一倍；next-intl v4 規定一個 locale 只能對一個網域；單一市場沒有理由 |
| a′. 公開頁用 a，私有頁（賣家中心、帳號、結帳）用 cookie | 私有頁不需要 SEO | 同一個 app 用兩套規則，切換器與深連結要分支處理；私有頁本來就 noindex，統一用 a 的成本為零 |

### 選項：非同步訊息的語系（推播、email、取貨簡訊）

| 選項 | 取捨 |
|---|---|
| **a. 帳號的 `preferred_locale`** | 一個人一個值；第一次登入時記成當時 client 的有效語系，之後只在使用者明確切換時改；Shopify 與 Amazon 的模型 |
| b. 每個裝置 token 記 locale | 兩支手機語言不同時比較精準；但 email 沒有裝置可依；兩個來源 |
| c. 推播只送 loc-key，由裝置本地化（APNs `loc-key`、FCM `body_loc_key`） | 文字必須在 App 的原生資源裡（Android `res/values`、iOS `.strings`），不在 Flutter 的 ARB；改字要發版；違反通知 ADR D1「文案只有程式碼一個來源」 |

### 選項：API 請求的語系

| 選項 | 取捨 |
|---|---|
| **a. 每個 client 都送單一值的 `Accept-Language: <UI 語系>`，伺服器用 `language.Matcher` 協商，信心值是 No 時用 zh-TW** | 橫跨所有 RPC，不必改契約；website BFF 要**覆寫**瀏覽器送來的值（瀏覽器語言不等於使用者選的 UI 語系）；回應要帶 `Vary: Accept-Language` |
| b. 每個 request message 加 `language_code`（AIP-143） | 要改約 100 支 RPC 的契約；適合「回應內容本身依語言而不同」的讀取（例如 Places），我們的情況主要是錯誤與原因文字 |
| c. 讀帳號偏好 | 未登入的請求沒有帳號；同一個人在兩個 client 用不同語言時會錯 |

### 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| 買家公開頁要被搜尋引擎索引 | REAL（產品需求） | `lib/seo/indexability.ts`、SEO ADR |
| 不能用 `proxy.ts`，必須用 edge middleware | REAL（OpenNext 的限制，第三方） | `website/middleware.ts:15-18` |
| admin 只有 zh-TW | REAL（Will 2026-10-17） | 本需求 |
| 英文內容之後才補，一開始只有 zh-TW 有內容 | REAL（Will 2026-10-17） | 本需求 |
| 舊 REST 與 CodeMeta 的中文文字要等模組搬進單體才會退場 | REAL（排定的搬遷步驟；現在改是丟棄式工作） | codemeta ADR 過渡步驟 3；website 29 個檔、App 67 個檔引用 codemeta 文字 |
| 現有 URL 不要變 | SELF-IMPOSED（上線前沒有外部連結）；但 a 剛好不必改 | — |
| 和現有 `lang="zh-Hant"` 一致 | SELF-IMPOSED | — |

### 建議（只依 REAL 調整）
1. **語系標籤**：資料、URL、訊息檔、API 一律用 `zh-TW` 與 `en`（短標籤）；Flutter 宣告 `Locale.fromSubtags('zh','Hant','TW')` 與 `en`；`<html lang>` 跟著 locale 走。刪除 seller-app 的 zh_CN。解析規則四端相同：任何 `zh*` → zh-TW；其他語言在英文**已發佈**時 → en，否則 → zh-TW。
2. **website** 採 a，`app/` 全部移進 `app/[locale]/`（含賣家中心）。Clerk 與 next-intl 在 `middleware.ts` 組合；`urls.ts` 改成依 locale 產生 URL，canonical、sitemap 與 hreflang（含 `x-default`→zh-TW）都從它產生。用 `setRequestLocale` 加 `generateStaticParams` 保住靜態渲染。
3. **發佈語系**：一份設定 `published locales`，現在只有 zh-TW。en 與 pseudo-locale 只在開發與 preview 環境開；`/en`、hreflang、App 的英文選項都由這份設定決定。發佈的門檻見 I4。
4. **App**：預設跟裝置語言；App 內的語言設定在 Android 用 per-app language API，在 iOS 開「設定」頁；選擇存在裝置上，同時呼叫 RPC 更新帳號偏好。
5. **帳號偏好**：`identity.accounts.preferred_locale`（`TEXT NOT NULL`，CHECK 在支援清單內）。由 identity 擁有，A8 讀它。不放 Clerk metadata：寄信路徑不要多一個外部依賴，與通知 ADR D8 同一個理由。
6. **API**：採 a。單體在 `httpapi/rpc` 加一個 interceptor 把協商結果放進 context；gateway 把 header 原樣轉給舊服務。admin 的 BFF 固定送 zh-TW，伺服器對 admin 的 RPC 也固定用 zh-TW。

**終局與觸發**：出現第三個語系，或某個讀取的**內容本身**依語言不同（例如平台內容有多語版本，且要指定和 UI 不同的語言）時，那支 RPC 才加 `language_code` 欄位。

---

## I2 文字資源：共用目錄與各端資源

**建議**：一個 client 一套 i18n 執行期，共用目錄是生成進這套執行期的來源，而不是另一套並行的查表機制。key 改成不帶版本的穩定 key，每個 key 都要有翻譯說明。

### 現況
- 已核准的 L（`step10e-labels-brief.md` D-L1 c）：來源是 `zh-TW.json`，生成**各自的型別化查表**（TS 的 `Record`、Dart、Go），刻意不走 next-intl 或 ARB（c′ 被否決的理由是「為單語系導入整套 i18n 執行期」，見 `:51`）。L-brief Q2 建議 key 用含版本的完整名稱，例如 `hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_WARNING`。
- 各端的使用者文字：website 有漢字的檔案超過 1,000 個（§R1）；buyer-app 約 3,341 行、seller-app 約 4,741 行（§R3）；共用套件裡也有，例如 `packages/hawkr-api-ts/src/connect/errors.ts:96` 的 `defaultErrorText`、`packages/hawkr_api/lib/errors.dart:201-209`、`packages/hawkr_auth`。
- 有拼接字串的寫法：seller-app 的 `$subject審核中`（`measure_display.dart:190-199`）。

### 業界基準
- Flutter gen-l10n 以 ARB 的 key 當 getter 名稱，所以 **key 必須是合法的 Dart 識別字**，不能有點；`@key` 的 `description` 與 `placeholders` 是「highly recommended」，也可以用 `required-resource-attributes` 強制要求；從 3.32 起 synthetic package 已棄用，必須 `generate: true`〔VERIFIED，§3〕。
- next-intl 的 messages 是每個 locale 一份 JSON，key 用點表示巢狀；可以靠型別確保 key 存在，ICU 參數的型別要靠 `createMessagesDeclaration`〔VERIFIED，§1、§7〕。
- Lokalise：同一個 key 在同一個語言下，不能在不同平台有不同譯文〔VERIFIED，`step10e-labels-research.md` §5〕；Locize 主張 key 依情境而不同、不要硬共用〔VERIFIED，L-brief〕。
- MF2 的規格在 CLDR 是 stable，但 TC39 的 `Intl.MessageFormat` 還在 Stage 1，Dart 沒有實作；next-intl、gen-l10n 都用 MF1〔VERIFIED／I，§4〕。
- CLDR：zh 的複數類別只有 `other`，en 有 `one`／`other`〔VERIFIED＋T，§4〕。

### 為什麼現有機制存在
L 是在「只有 zh-TW」的前提下，為了消除四端手抄與條款用語漂移而設計的，所以選了最小的執行期。這個前提已經被本需求推翻；它自己寫的觸發條件「第二語系 → 生成改成輸出 next-intl messages 與 ARB（c′）」（`step10e-labels-brief.md:78`）現在成立了。

### 選項：目錄怎麼進入 client

| 選項 | 做法 | 取捨 |
|---|---|---|
| a. 照原本的 c：生成依 locale 的型別化查表 | `label(locale, v)` | client 會有兩套 i18n（next-intl／ARB 加上自製查表）；ICU 的 `select`、`plural` 要自己在 TS 與 Dart 實作一份格式器 |
| **b. c′：生成進各端的執行期** | TS：`@hawkr/ui-text` 輸出 `messages/<locale>.json`（website 併進 next-intl 的 `enum` namespace）和型別化的 key helper（`measureKindKey(v)` 回傳 key 的字面型別）；Dart：套件 `hawkr_ui_text` 用自己的 `l10n.yaml` 跑 gen-l10n，輸出 ARB 與 `UiText` delegate，再生成 `extension on MeasureKind { String label(UiText t) }`；Go：依 locale 的函式；admin 只吃 zh-TW 的常數表 | ICU 格式化用各端現成的；窮舉檢查留在生成器（缺值就生成失敗），client 端由 key 的字面型別與 Dart 的 switch 窮舉保證 |
| c. 目錄停用，名詞回到各端的 i18n 資源 | 每端自己的 messages／ARB | 失去 L 原本要解決的跨端一致；否決 |

### 選項：key 方案（重新檢討 L-brief Q2）

| 選項 | 取捨 |
|---|---|
| 含版本的完整名稱 | 升 `v1` 時每個 key 都改名，TMS 裡的翻譯歷史、註解、截圖都斷掉（翻譯記憶還在，但只能比對原文）；而且 ARB 不接受點 |
| **不帶版本的 descriptor 路徑：`<去掉 hawkr 與版本的 package>.<Enum>.<VALUE>`**，例如 `seller.MeasureKind.MEASURE_KIND_WARNING`、`admin.seller.FoodRegistrationState.…` | 穩定、可以機械推導、不撞名（admin 與 seller 的同名 enum 在不同 package）；ARB 的名稱由生成器轉成 camelCase；上線前不會有 `v1alpha1` 和 `v1` 並存 |
| 手寫的語意 key（`measure.kind.warning`） | TMS 風格，但需要另一張和 proto 的對照表 |

### 建議（只依 REAL 調整）
1. 採 b 與不帶版本的 key。**每個 key 的翻譯說明**由生成器從 proto 值的註解帶入（codemeta ADR D6 規定 proto 註解只寫語意，剛好就是譯者要的情境）；受眾（預設／`staff`）也寫進說明。
2. **分界不變**：「只由契約值決定、兩個以上的 client 都用到」的文字放目錄（L-brief Q3 b），另外再加上 Connect code 的預設錯誤文字（`common.error.<CODE>`，取代 TS 與 Dart 各一份的 `defaultErrorText`）。其他文字放各 client 自己的資源：website 的 next-intl JSON 依 feature 分 namespace；兩支 App 各自的 ARB；`hawkr_auth` 有自己的 ARB。
3. **key 的寫法**：語意化、依 feature 分 namespace（`checkout.payment.submit`），不用原文當 key。說明必填：ARB 用 `@key.description`；next-intl JSON 沒有 metadata 欄位，用同名的 `*.meta.json` 側檔，由 CI 檢查兩邊 key 一致（TMS 能不能匯入這種側檔：**未查證**）。
4. **格式**用 ICU MF1 的受限子集：只允許 `{arg}`、`select`、`plural`。訊息裡不放 `number` 或 `date` 的 skeleton，數字和日期由各端的格式化 helper 處理（見 I5），再當參數傳進去。帶數量的訊息，中文原文也必須寫成 `{count, plural, other {# 件}}`，英文才有地方放 `one`。禁止拼接字串；`$subject審核中` 這類寫法要改成一整句帶參數的訊息。
5. **完整性以（受眾、語系）為單位**：預設受眾的條目在每個**已發佈**的語系都要完整；`staff` 受眾只有 zh-TW（admin 只有 zh-TW）。未發佈的語系可以不完整，CI 只回報覆蓋率。

**終局與觸發**：ECMA-402 與 Dart 都有穩定的 MF2 實作時，再評估換成 MF2。

---

## I3 伺服器產生的文字

**建議**：伺服器用「這段文字要給誰看、什麼時候看」來決定語系；句子放在擁有它的模組，和目錄用同一支生成器，產生有型別的 Go 函式。

### 現況
- `backend/internal/httpapi/rpc/errors.go:29` `const errorLocale = "zh-TW"`；模組的使用者文字經 `apperr.Error.Message`（`internal/platform/apperr/apperr.go:124-140`）送出，`internal/modules` 裡約有 449 個 `apperr.` 呼叫點都直接寫中文字面值（§R4）。
- 錯誤模型 ADR `:51` 規定 `message` 給開發者看、不另寫英文版；`:59` 規定 `locale` 固定 zh-TW；`:131` 的觸發條件是「需要第二個語系時，locale 改由請求決定」。`api/hawkr/admin/seller/v1alpha1/measure.proto:172` 的註解也寫 locale 固定 zh-TW。
- 通知 ADR（`2026-10-05-notification-module.md`）：D1 的文案集中在一個 zh-TW 訊息目錄；D5 收件匣存型別與參數，讀取時才組字。`:41` 否決「存渲染快照」，理由之一正是「日後多語系不能回溯」。A8 還沒開工。舊的 notification-service 把模板存在 DB 的單一中文欄，收件匣存渲染後的文字（§R4），它會在 A8 退場。
- Email 只有 vendor invite，用 Sprintf 組字，到期時間原樣輸出、沒有轉台北時間（`services/notification-service/internal/worker/vendor_invite.go:76-78,107-108`）。電子發票 PDF 由 AMEGO 產生；Go 端沒有 SMS client。
- 舊服務的使用者文字：ai 1584 行、pricebook 442、invoice 256、seller 198（§R4），這些會隨各自的模組搬遷退場。

### 業界基準
- AIP-193：`LocalizedMessage` 的 `locale` 與 `message` 都必須填；`locale` 可以依請求決定；`Status.message` 給開發者看，應該是英文；動態值放在 `ErrorInfo.metadata`〔VERIFIED，§5〕。
- 通知：Shopify 依顧客的語言寄信〔VERIFIED〕；Discourse 與 GitHub 存型別與資料、讀取時渲染（通知 ADR 已採用）。
- Go 的函式庫：`golang.org/x/text/language` 負責解析 `Accept-Language` 與比對〔VERIFIED〕；nicksnyder/go-i18n v2 支援 CLDR 複數與 extract／merge，但模板是 Go `text/template` 語法，不是 ICU〔VERIFIED／I〕；`x/text/message` 的 catalog 工具（gotext）較少人用〔I〕。

### 為什麼現有機制存在
只有 zh-TW，所以 Go 直接寫字面值最簡單；錯誤模型 ADR 已經預留 `LocalizedMessage` 的形狀，就是為了「第二語系時不必改形狀」（`step10e-labels-brief.md:128`）。

### 選項：Go 端的文字

| 選項 | 取捨 |
|---|---|
| a. go-i18n v2 | 成熟、支援 CLDR 複數；但語法和 TS／Dart 的 ICU 不同，目錄名詞要在兩種語法之間轉換，譯者要學兩套 |
| b. `x/text/message`＋gotext | 標準庫血統；工具鏈較少人用，也不是 ICU |
| **c. ICU 來源檔＋生成器輸出有型別的 Go 函式** | 和目錄同一種格式、同一支生成器（L′ 本來就要輸出 Go）；受限子集只有 `{arg}`／`select`／`plural`，plural 用 CLDR 規則（`x/text/feature/plural`，**未查證** API 是否合用；否則只用 zh 與 en 兩組固定規則）；參數是 struct，編譯期檢查 |
| d. 繼續寫字面值，用 key 查 map | 沒有參數型別；缺翻譯要到執行期才知道 |

### 建議（只依 REAL 調整）
1. **語系怎麼決定**：同步回應（錯誤、原因、`LocalizedMessage`）依 context 裡協商出的 locale；非同步（A8 的推播、email、SMS）依收件帳號的 `preferred_locale`；收件匣在讀取時依讀者請求的 locale 組字（D5 已經是讀取時渲染，不用改）；admin 的 RPC 固定 zh-TW 加 `staff`。
2. **文字放在哪**：每個模組一個 `text/` 目錄，放 `zh-TW.json`、`en.json`（ICU 受限子集），由 L′ 的生成器產生 `text_gen.go`；例如 `text.AppealUnavailableCooldown(tag, params) string`。句子裡的名詞呼叫目錄的 Go 函式，用同一個 tag。`apperr` 的使用者文字改成「已經依 tag 組好的 `LocalizedMessage`」，或改成會延後組字的文字函式，不再接受字面值；CI 擋 `apperr` 的中文字面值。
3. **`message`（給開發者）**：由 `ErrorInfo.reason` 與 metadata 自動產生（例如 `SELLER_SUSPENDED store_id=123`），不另外寫英文。這樣 log 不會隨請求語系改變，也不必寫兩份文字。這符合 AIP-193「message should be English」的用意，又不增加撰寫量。
4. **通知（A8 的設計輸入）**：參數快照只存原始值（金額的整數、時間戳、名稱），不存格式化後的字串，讓不同語系各自格式化；email 用 `html/template` 加 locale；投遞紀錄存實際送出的文字與 locale。
5. **伺服器格式化**：在 `twtime` 加依 locale 的格式化 helper（台北時區），取代 63 處各自寫的 `Format("2006…")`。
6. **範圍外（REAL）**：舊服務的文字與 CodeMeta 的名稱隨模組搬遷退場，不在舊服務上做 i18n；AMEGO 的電子發票是政府格式、第三方產生，維持中文。

**終局與觸發**：出現非工程人員的譯者時，模組的 `text/*.json` 和目錄、client 的資源一起交給 TMS（I4）。

---

## I4 TMS 與流程

**建議**：現在不用 TMS；repo 裡的檔案就是來源，格式都是 TMS 能直接匯入的；把 CI 檢查一次建好；英文由 LLM 依術語表起草、Will 審。

### 現況
沒有任何 i18n 的 CI；website 已經有一支 ratchet 型的檢查先例（`scripts/check-legacy-endpoints.py`）。

### 業界基準
- Weblate 與 Crowdin 都能以 git 或 PR 同步，支援 ARB 與 ICU JSON；Tolgee 可以自架，免費雲端方案 3 萬字；Lokalise 每月 149 美元起、Phrase 每月 525 起，都只有試用〔VERIFIED／I，§6〕。
- Pseudo-localization：Microsoft 的經驗值是加長 40%，短字串可能變成 2 到 4 倍；ICU 的 `en-XA` 會加重音符號、加長、加括號〔VERIFIED，§7〕。中文翻英文會變長很多，但找不到權威比例〔I〕。
- 缺 key 的檢查：next-intl 的型別化 key、`@lingual/i18n-check`；Flutter 的 `untranslated-messages-file`〔VERIFIED／I，§7〕。

### 選項

| 選項 | 取捨 |
|---|---|
| **a. 現在只用 repo 檔＋CI；之後再接 TMS** | 零外部帳號；只有一位維護者、只有兩個語系，PR 審查就是翻譯流程 |
| b. 現在就自架 Tolgee 或 Weblate | 多一個要維運的服務；可以在畫面上直接編輯（in-context），但 Tolgee 的 Flutter in-context 還是 alpha |
| c. Crowdin 或 Tolgee 雲端免費方案 | 要開對外帳號（每次都要問 Will）；私有 repo 拿不到 Crowdin 的開源方案 |

### 約束帳
沒有翻譯人員，也沒有營運團隊（REAL，通知 ADR 的前提）；開對外帳號每次都要問（REAL，授權規則）；工具成本（SELF-IMPOSED）。

### 建議
採 a。CI（一個 job 加上各端的 lint）檢查：
- (1) **硬寫中文的 ratchet**：買家與賣家 client（website 排除 admin、buyer-app、seller-app、共用套件）的 UI 程式碼裡，漢字字面值的數量只能減少，不能增加。基準數由腳本記錄，比照 legacy-endpoints。
- (2) ICU 語法、受限子集、各語系的 key 與參數一致。
- (3) 每個 key 都有說明。
- (4) 已發佈的語系覆蓋率 100%；未發佈的只回報。
- (5) 不用的 key。
- (6) 目錄與生成檔沒有 drift。

開發與 preview 版加一個 pseudo-locale（`en-XA`，由 zh-TW 產生：加括號、轉成拉丁字母並加長 2 倍），用來抓漏抽的字串和被截斷的版面。英文起草要先有術語表（I7 Q5）。

**終局與觸發**：出現第一位非工程人員的譯者，或第三個語系 → 自架 Tolgee 或 Weblate（以 git 同步，repo 檔仍是來源）。

---

## I5 格式化與內容資料

**建議**：時間一律以台北時間顯示；幣別固定 TWD、0 位小數；格式化集中在各端的 helper，依 locale 選樣式。使用者產生的內容不翻譯；平台撰寫的內容用 `LocalizedText`。

### 現況
- website 的 `lib/date/taipei.ts:1-30` 刻意不用 `toLocale*` 與 `Intl`：Workers 的 runtime 是 UTC，而且 ICU 版本和瀏覽器不同，會造成 hydration mismatch（#374、#391）。它用固定的 +08:00 自己組字串，部分輸出把中文直接寫在格式裡（`M月D日`，`:276`）。
- 金額：`website/lib/products/pricing.ts:91` 的 `NT$ ${n.toLocaleString()}` 會跟著 runtime 的 locale；`NT$` 字面值在 website 有 178 處、admin 32 處。App 端有 `formatNTD`（`buyer-app/lib/shared/utils/amount_utils.dart:38`），兩支 App 各有一份 `price_display.dart`。
- App 的時間有 **57 處 `toLocal()`**，跟著裝置時區，是現存的 bug；`packages/hawkr_api/lib/taipei_time.dart:9` 是正確的固定 UTC+8 寫法，但只有 seller-app 一處在用（§R3）。
- 平台撰寫的內容都是單一語言欄：`category.name`（Java product-service）、`buyer.membership_plans.display_name`、content 模組的 `banners.title`、`recipes.title`、`recipe_ingredients.name`、`app_version_gates.notice_title`（§R4）。twaddr 的 CSV 只有中文名稱。

### 業界基準
- W3C 把時區和語系當成兩個獨立的因素；有時區的值，顯示時用它的時區〔VERIFIED，§11〕。next-intl 沒設 `timeZone` 時用伺服器的時區〔VERIFIED〕。
- `Intl.NumberFormat` 的 TWD：en 輸出 `NT$1,234.50`、zh-TW 輸出 `$1,234.50`，預設帶 2 位小數〔T，§11〕。
- UGC：Airbnb 與 Etsy 會自動翻譯並保留原文，人工翻譯優先；使用 Google 翻譯必須標示是機器翻譯〔VERIFIED／I，§8–9〕。Shopify 缺翻譯時退回預設語言〔VERIFIED〕。

### 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| 配送時段、截單時間、店家營業時間都是台北當地時間 | REAL（單一市場） | 產品 |
| Workers 和瀏覽器的 ICU 不同，會造成 hydration mismatch | REAL（runtime） | `taipei.ts:4-12` |
| 台灣沒有日光節約時間，固定偏移就是正解 | REAL | `taipei.ts:17` |
| 物流與超商的收件標籤是中文地址 | REAL（第三方） | — |
| 分類在 Java 的 product-service，要到步驟 B 才搬 | REAL（排定的搬遷步驟） | 計畫 |
| 沿用 `NT$ ` 加空格的寫法 | SELF-IMPOSED | — |

### 建議（只依 REAL 調整）
1. **時間**：四端一律以 Asia/Taipei 顯示。website 擴充 `taipei.ts`，加上依 locale 的樣式表（en 的月份縮寫等），不用 Intl，保住確定性的輸出。App 的 57 處 `toLocal()` 全部改用共用的 Taipei helper，再加 `intl` 的 `DateFormat`，傳入先換算好的台北時間。標示台北時間的時機：在 client 偵測到裝置或瀏覽器時區不是台北時，才加「台北時間／Taipei time」（只在 client 渲染）。
2. **金額**：每端一個 `formatMoney`：固定 TWD、`maximumFractionDigits:0`、符號在兩個語系都是 `NT$`（zh-TW 的 CLDR 預設 `$`，在台灣一般讀得懂，但英文版必須是 `NT$`；兩語系統一用 `NT$` 最簡單）。數字分隔用 locale 的規則。
3. **UGC**（商品名、店家介紹、貼文、評價、聊天、admin 寫給賣家的說明）：不翻譯、不做機器翻譯；容器標上 `lang` 屬性（螢幕閱讀器與字型需要）。觸發條件：英文買家的比例或回饋顯示需要時，再評估機器翻譯加「顯示原文」。
4. **平台內容**：用 `LocalizedText`，以 JSONB 存 `{ "zh-TW": "…", "en": "…" }`，CHECK 要求一定有 zh-TW；伺服器依請求 locale 回**單一字串**，缺英文時退回 zh-TW（Shopify 的行為）。content 模組現在已在單體，在 I-content 做；分類在步驟 B 搬遷時做。
5. **地址**：資料一律存代碼；選單和顯示依 locale 呈現縣市與鄉鎮名稱。要有英文名稱，twaddr 需要官方英文名稱的資料來源（中華郵政的英譯，**未查證**）。寄件標籤與物流的單據永遠用中文。

**終局與觸發**：平台內容需要三個以上語系，或要經 TMS 同步時，改成翻譯表 `(entity, field, locale, text)`。

---

## I6 法律文件

**建議**：中文 Markdown 是正本；英文是附上版本對照的參考譯本；兩個版本都寫「以中文版為準」；四份買家文件也改成走產生器；同意紀錄加上當時顯示的語系。

### 現況
- 產生器 `website/scripts/generate-legal.ts:3-9` 只處理三份賣家文件加 manifest，輸出 TS 投影與 Go 的 `CurrentAgreements`；版本是正本的生效日（`seller-terms.md:3`，並註明「未經律師審閱」）。
- 買家的 `/terms`、`/privacy`、`/return-policy`、`/user-policy` 是手寫的 TSX（`website/features/legal/pages/*.tsx`，約 1,400 行），沒有經過產生器。
- 沒有語言優先條款；`seller-terms.md:227-231` 只有準據法與管轄。
- 同意紀錄 `seller.agreement_acceptances`（`backend/internal/modules/seller/db/schema.sql:1277+`）沒有 locale 欄位。

### 業界基準
- 台北市法規的英文版寫明「in case of any discrepancy … the Chinese version shall prevail」〔VERIFIED，搜尋片段〕。國際平台多半以英文為準〔I〕。台灣平台（LINE、蝦皮、momo）的條款裡有沒有語言條款：**沒查到**。
- 消保法有沒有規定定型化契約必須用中文：**沒查到**明文〔I〕；商品標示法規定「應以中文為主」，但管的是商品標示，不是契約〔VERIFIED〕。
- 推論：消保法第 11 條第 2 項規定定型化契約有疑義時，應作有利於消費者的解釋。英文譯本如果和中文不一致，「以中文為準」條款對消費者的效力可能受限〔INFERENCE，要律師確認〕。

### 選項

| 選項 | 取捨 |
|---|---|
| **a. 中文正本＋英文參考譯本，兩版都寫「以中文為準」；同意的對象永遠是中文版本** | 一份有拘束力的文字；英文使用者看得懂；版本只有一條線 |
| b. 兩種語言有同等效力 | 每次修訂要法律審兩份；不一致時沒有依據 |
| c. 不提供英文 | 違反本需求 |

### 建議
採 a：
1. 來源 `website/docs/legal/<doc>.md`（正本）加 `<doc>.en.md`。英文版的 front matter 寫 `translationOf: <正本版本>`；英文**已發佈**時，產生器要求它等於正本的現行版本，否則生成失敗（譯本落後就不能部署）。
2. 四份買家文件改成 Markdown 正本，走同一支產生器；manifest 加上買家的文件類型。
3. 頁面：英文頁的最上方固定顯示「This translation is provided for reference only; the Traditional Chinese version prevails.」，並連到中文版。
4. 同意紀錄：`kind` 加 `version`（中文正本的版本）不變，另加 `displayed_locale`，留下使用者實際讀到哪個語言的證據。
5. 條款本文加上語言條款（中英兩版都要），上線前和其他條款一起請律師審。

---

## I7 刀 L 與計畫怎麼重排

### L-brief 各題怎麼變

| 題目 | 原本的建議 | 現在 |
|---|---|---|
| Q1 預設受眾與 `staff` | 漂移收斂，視角差異用 `staff` | **不變**，另外加上：`staff` 的條目只有 zh-TW（admin 只有 zh-TW），完整性以（受眾、語系）為單位 |
| Q2 key 與格式 | 含版本的完整名稱、兩層 JSON、ICU MF1、不處理複數 | **推翻**：改成不帶版本的穩定 key（I2）；每個語系一份檔案；說明從 proto 註解帶入；受限的 ICU 子集；**要處理複數**（英文需要） |
| Q3 收錄範圍 | 只由契約值決定、兩端以上使用的文字 | **不變**，另加 Connect code 的預設錯誤文字；組合文字一律寫成整句的 `select`，不拼接 |
| Q4 完整性 | 以 enum 為單位、規則排除、CI 擋手寫表 | **不變**，另加：每個已發佈的語系都要有通用的「未知」 |
| Q5 正式用語 | 對齊條款：措施／申覆／解除請求 | **不變**，另加英文術語表，見下 |
| Q6 生成器 | Go 程式讀 buf image，套件名 `uitext` | 生成器不變；**輸出改成** next-intl 的 namespace JSON 加 key helper、Dart 套件自己的 ARB 與 delegate、Go 依 locale 的函式；同一支生成器也處理各模組的 `text/*.json`（I3） |
| Q7 切分 | L0–L8 | 照下面的新切分 |

**Q5 的英文術語（草案，要 Will 定）**：措施＝`restriction`；申覆＝`appeal`；解除請求＝`request to lift`；消費者申訴＝`complaint`；認定＝`finding`；警告＝`warning`；終止合作＝`termination`；代行＝`assisted operation`。DSA 的英文用語是「restriction」「statement of reasons」，而且它的「internal complaint-handling」指的是**賣家對決定的申覆**〔INFERENCE，憑記憶，未重查〕。也就是說，英文的 complaint 和 appeal 也會撞到同樣的問題，所以術語表必須明確寫出：appeal 指賣家對措施的申覆，complaint 只指消費者申訴。

### 已定案中被推翻或修改的部分
- `step10e-labels-brief.md`：D-L1 的「生成型別化查表」改成 c′（生成進執行期）；`:78` 的終局句「第二語系前不做 `Accept-Language`」的觸發已經成立；D-L2 不變，只是 `LocalizedMessage` 的 locale 改成依請求。
- `step10e-L-brief.md` 知會「不接 next-intl、不做 `Accept-Language`」：推翻。
- 計畫 `2026-09-29-a3-buyer-seller.md:359` 的末句：觸發已經成立，改寫成「多語系 ADR」的連結。
- ADR 的文末修訂（不改原文）：codemeta `:38` 的繼承約束不再成立，`:87` 的觸發成立；錯誤模型 `:32`、`:51`（message 改成自動產生）、`:59`（locale 依請求）；通知 ADR D1（文案改成每個語系一份，收件人的語系依 `preferred_locale`）。
- 3a 已提交的部分：admin 的四個 `MeasureActionUnavailability` 屬於 `staff`、只有 zh-TW，**不用改**；只需要把 `measure.proto:172` 的「locale 固定 zh-TW」改成「admin RPC 固定 zh-TW」。
- 10e 的 3b（admin）不受影響。

### 新的切分（每一刀都要綠）

| 刀 | 內容 | 依賴與協調 |
|---|---|---|
| **I0** | 新 ADR「多語系架構」（I1–I6 的結論）、上面列的各 ADR 文末修訂、術語表（zh↔en）、語系標籤與 `published locales` 的定義 | 純文件；先於 L′ |
| **L0** | 文案核可（L-brief Q1 的漂移收斂、Q5 的用語），同時審英文術語 | Will |
| **L′** | 原本的 L1–L8，改成多語系形狀：目錄 `zh-TW.json`＋`en.json`（英文可以先留空）、不帶版本的 key、說明、生成器輸出 c′ 的形狀、CI。website 與 App 端要先有 I-web-0 與 I-app-0 的執行期接線 | 在 I-web-0／I-app-0 的**接線部分**之後 |
| **I-server** | locale interceptor、context、`LocalizedMessage` 依 tag、模組的 `text/` 加生成、`apperr` 改成不收字面值、`message` 自動產生、`twtime` 的格式化；約 449 個呼叫點在單體裡逐模組改 | 只動單體；10d 動的是 social 舊服務與 gateway，衝突小，但 gateway 的 header 轉送要告知 10d |
| **I-account** | `preferred_locale`、設定與讀取的 RPC、client 第一次登入時寫入 | identity 模組；A8 讀它 |
| **I-web-0** | `app/[locale]` 搬遷、next-intl routing（`as-needed`、關掉偵測）、和 Clerk 組合的 middleware、`urls.ts`／sitemap／hreflang／`lang`、`taipei.ts` 依 locale、`formatMoney`、BFF 覆寫 `Accept-Language` 加 `Vary` | **要和 10d 協調**：搬遷會動到 `app/` 的每一個路由檔，10d 的 website 貼文頁與公開讀取也在改這些檔。要在 10d 的 website 改動合進 main、10d 暫停的空檔，用一個只做搬移的 commit 完成，然後兩邊都 rebase |
| **I-app-0** | 兩支 App：`l10n.yaml`（`synthetic-package:false`）、ARB 骨架、`supportedLocales`、解析規則、App 內的語言設定、transport 的 header、iOS 與 Android 的語系宣告、57 處 `toLocal()` 改台北時間、`formatMoney` | **要和 10d 協調**：10d 在 buyer-app 與 seller-app 的社群畫面有改動；`main.dart`、`app.dart`、`transport.dart` 是熱點檔，要排在 10d 的 App 改動合進 main 之後 |
| **I-CI** | I4 列的 ratchet 與檢查 | 可以和 I-web-0、I-app-0 一起做 |
| **I-legal** | I6 | website 與 seller 模組 |
| **I-content** | content 模組的 `LocalizedText`；分類在步驟 B | — |
| **I-sweep-*** | 依區域把字串抽成資源（website 買家頁、賣家中心、buyer-app、seller-app），每一區一刀；ratchet 保證只減不增 | 每一區要排在該區的遷移步驟結束後，避免抽完又被重寫；以後動到哪個畫面，就在同一個 commit 把它抽成資源 |

**英文的發佈門檻**：某個介面（例如 buyer-app）在英文下看不到任何未翻譯的文字，才能把 en 加進那個介面的 `published locales`。也就是那個介面的 sweep 完成、目錄與伺服器文字的英文覆蓋率 100%，而且那個介面用到的舊 REST 或 CodeMeta 文字已經隨模組搬遷退場（REAL）。實際上，英文會在相關模組搬進單體之後才發佈；架構與 zh-TW 在這之前就全部走新路徑。

---

## Will 要回答的問題

1. **I1 website 的 URL**：同意 `as-needed`（zh-TW 在 `/`、英文在 `/en`）、不依瀏覽器語言自動導向，改成顯示切換提示，而且賣家中心與帳號頁也用同一套 URL？（建議：同意）
2. **I1 非同步訊息的語系**：推播、email 都依帳號的 `preferred_locale`（第一次登入時記成當時 client 的語系，之後只在使用者明確切換時改），而不是每個裝置各自記？（建議：帳號層級）
3. **I2 目錄的形狀**：把 SSOT D-L1 的 c 改成 c′（生成進 next-intl 與 ARB 的執行期），key 改成不帶版本的穩定 key、翻譯說明從 proto 註解帶入？（建議：同意；推翻 L-brief Q2）
4. **I3 開發者看的 `message`**：由 `ErrorInfo` 自動產生，不再等於使用者文字？（建議：同意；修訂錯誤模型 ADR `:51`）
5. **I7 發佈門檻與排程**：英文要等「該介面沒有任何未翻譯文字」才發佈（實際上會在相關模組搬進單體之後），字串抽取依區域、排在各區的遷移步驟之後，並由 ratchet 擋住新增的硬寫中文？或是你要現在就安排一個專門的抽取步驟？（建議：前者）
6. **I7 和 10d 的協調**：I-web-0 的 `app/[locale]` 搬遷與 I-app-0 的熱點檔，排在 10d 目前的 website 與 App 改動合進 main 之後、由一個只做搬移的 commit 完成？這需要 10d 那邊暫停一下。（建議：同意；先後順序是 I0 → L0 → I-web-0／I-app-0 的接線 → L′ → 3b）
7. **I7 Q5 英文術語**：措施＝restriction、申覆＝appeal、解除請求＝request to lift、消費者申訴＝complaint 等（草案在 I7）。（建議：定案前我先依 DSA 的英文用語再查一次）
8. **I6 法律**：中文正本＋英文參考譯本、「以中文為準」、同意的對象永遠是中文版本，並記錄 `displayed_locale`？語言條款對消費者的效力要列進上線前的律師審閱清單？（建議：同意）

## 知會（照建議做，有異議再說）
- 語系標籤用 `zh-TW` 與 `en`；Flutter 用 `zh_Hant_TW`；刪除 seller-app 的 zh_CN；`<html lang>` 跟著 locale 走（現在 website 與 admin 寫的是 `zh-Hant`）。
- 解析規則：任何 `zh*` → zh-TW；其他語言在英文已發佈時 → en，否則 → zh-TW。伺服器的 matcher 信心值是 No 時，自己退回 zh-TW（Go 不會自動退到清單第一個，§5 T）。
- 預設語系是 zh-TW，不是 AIP-193 的 en-US：主要市場是台灣（REAL）。
- admin 只有 zh-TW：不裝 next-intl；目錄的 `staff` 條目只有 zh-TW；admin 的 RPC 固定 zh-TW。
- 不用 TMS；CI 檢查一次建好；開發版有 pseudo-locale。
- 時間一律台北時間；App 的 57 處 `toLocal()` 是現存的 bug，在 I-app-0 修。
- 金額固定 TWD、0 位小數、兩個語系都顯示 `NT$`。
- UGC 不翻譯；平台內容用 `LocalizedText`；地址存代碼，物流單據用中文。
- 通知（A8）：參數只存原始值、收件匣在讀取時依語系組字、投遞紀錄存實際送出的 locale；舊 notification-service 的 DB 模板不做 i18n，A8 時退場。
- AMEGO 的電子發票維持中文（第三方、政府格式）。
- 推播不用 loc-key 由裝置本地化（文字要放進原生資源，和 Flutter 的 ARB 分家）。
- seller-app OCR 的 `'language': 'zh-Hant'`（`universal_ocr_service.dart:91`）是辨識文件的語言，不是 UI 語系，不用改。
- 範圍外但會碰到：website 的 `NT$ ${n.toLocaleString()}` 會跟著 runtime 的 locale，在 Workers 和瀏覽器之間可能不一致，I-web-0 的 `formatMoney` 會一起修掉。

## 未查證
- 蝦皮台灣、momo、PChome 有沒有英文介面（§8）。
- 台灣法規有沒有要求定型化契約必須用中文；「以中文為準」條款對消費者的效力（§10）。
- iOS 裝置設成「繁體中文（台灣）」時，實際會解析到 `zh_Hant_TW` 還是 `zh_TW` 的 ARB，要上機實測（§12）。
- Flutter App 在 Android 13 用每個 App 的語言設定時，要不要手寫 `locale_config.xml`（§3 I）。
- iOS 的每個 App 語言設定與 `CFBundleLocalizations`：只查到第三方資料。
- Clerk 寄出的驗證信或 OTP 能不能依使用者語系（Clerk 文件只說 Account Portal 維持英文；沒有查到 email 的部分）。
- Dart 套件用自己的 `l10n.yaml` 產生 delegate、再讓兩支 App 引用，在 synthetic package 移除之後是否順利（推論可行）。
- next-intl 的 middleware 在 OpenNext／workerd 的 edge middleware 裡能不能正常運作（推論可行，I-web-0 的第一個實驗要驗證）。
- `golang.org/x/text/feature/plural` 適不適合給生成的 Go 程式碼用。
- next-intl 的 `*.meta.json` 說明側檔，TMS 能不能直接匯入。
- twaddr 英文名稱的官方資料來源（中華郵政）。
- DSA 的英文用語（術語表草案依記憶）。
- Shopify Admin API 的 `Customer.locale`（搜尋摘要）。
- 本簡報還沒經過另一個 fresh subagent 審查；依全域規則，交給 Will 之前要先做。
