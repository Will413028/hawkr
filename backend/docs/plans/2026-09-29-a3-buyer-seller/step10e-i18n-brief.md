建議：照草稿的大方向走（URL 決定語系、帳號偏好管非同步、伺服器依請求語系組字、repo 檔加 CI、台北時間、中文正本），但改五件事：L′ 不必等 `[locale]` 搬遷就能做（解開和 10d 的耦合）、`/en` 的 UGC 頁先 noindex、Accept-Language 要在 transport 設而不是在 BFF 覆寫、Dart 的目錄直接生成程式碼而不經 ARB、法律條款改成「列進律師審閱」而不是當成已定的機制。

**Will 的決定（2026-10-17）：全照建議（第 1–10 題）。** 範圍：買家與賣家介面多語系（website、buyer-app、seller-app），admin 只有繁中；架構現在做、英文內容之後補；法律文件出英文參考譯本、以中文為準（效力列入上線前律師審閱）。第 8 題同時定案 `step10e-L-brief.md` 的 Q1、Q3、Q4、Q5（照原建議加本文 I7 的增補），Q2 由本文第 4 題取代。

# 決策簡報 v2：多語系架構（zh-TW＋en）與刀 L 的重排（hawkr）

基準：worktree `hawkr-10e`，HEAD `a7d99873f`（草稿寫的 `0b9e4675e` 已經過時；本版的行號都在 a7d99873f 重查過）。只讀調查。草稿 `step10e-i18n-brief-v1.md`，筆記 `step10e-i18n-research-repo.md`（§R）、`step10e-i18n-research-web.md`（§1–12）。本版新查的來源標在各段。
需求（Will 2026-10-17）：買家面（website 買家頁、buyer-app）與賣家面（website 賣家中心、seller-app）要有英文介面；admin 只有 zh-TW；**現在建架構，英文內容之後補**；法律文件提供英文參考譯本，以中文為準。
前提：還沒上線、沒有相容性約束；「與現有程式一致」「改動量」「工作量」一律 SELF-IMPOSED；2026-10-15：能不能做、為什麼由伺服器推導，client 只呈現。
本簡報部分取代 `backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-labels-brief.md` D-L1 的 c（`:50`）與終局句（`:78`），以及 `step10e-L-brief.md` 知會的「不接 next-intl、不做 `Accept-Language`」。

## 結論

| # | 題目 | 建議（一句） |
|---|---|---|
| I1 | 語系從哪來、存在哪 | website 由 URL 決定（zh-TW 在 `/`、en 在 `/en`，next-intl `as-needed`、`localeDetection:false`），任何頁都不依瀏覽器語言自動導向；伺服器產生的連結（email、推播）帶收件人語系的前綴；App 預設跟裝置語言、可在 App 內另選；`identity.accounts.preferred_locale` 是非同步訊息的唯一來源；每個 RPC 由 **client transport** 送單一值 `Accept-Language` |
| I2 | 文字資源 | 共用目錄是多語系來源檔，生成到各端：TS 生成 next-intl namespace、Dart **直接生成程式碼**（不經 ARB）、Go 生成函式；client 自己的文字用各端原生的資源（website：next-intl JSON；App：ARB＋gen-l10n）。key 是不帶版本的 descriptor 路徑；說明從 proto 註解帶入；ICU MF1 受限子集，帶數量就寫 `plural` |
| I3 | 伺服器文字 | 同步回應依請求語系、非同步依帳號偏好、收件匣讀取時組字；句子放擁有它的模組，ICU 來源檔加生成的 Go 函式；`LocalizedMessage.locale` 是實際使用的語系；`Status.message` 與 `FieldViolation.description` 改成給開發者看，**同時拿掉兩個 client 在 description 上的退路** |
| I4 | TMS 與流程 | 現在不用 TMS；來源檔用帶說明的結構化 JSON（不另做 `*.meta.json` 側檔）；CI 一次建好，硬寫中文的 ratchet **以檔案為單位**；pseudo-locale 只在開發與 preview |
| I5 | 格式與內容資料 | 業務時間一律台北時間；`toLocal()` 是 57 處（測試另有 2 處），其中 8 處只算時間差、不是 bug，其餘是**潛在** bug（裝置時區不是台北時才會錯）；金額 TWD、0 位小數、兩語系都用 `NT$`；UGC 不翻；平台內容用 `LocalizedText` |
| I6 | 法律文件 | 中文正本＋英文參考譯本（記它翻的是哪一版）＋`displayed_locale`；「以中文為準」條款的效力**沒有找到法源**，列進上線前的律師審閱，不當成已經成立 |
| I7 | 排程 | I0（ADR＋術語表）→ L0（文案）→ **L′ 直接接在現有的 next-intl provider 上，不等 `[locale]` 搬遷** → 3b；`[locale]` 搬遷（I-web-0）在 10d 的 commit 邊界做一個只搬檔的 commit，不必等 10d 整段做完；英文發佈門檻＝該介面沒有未翻譯的文字 |

---

## I1 語系怎麼選、怎麼保存

**建議**：每個介面只有一個語系來源。website 看 URL；App 看裝置設定或 App 內的選擇；非同步訊息看帳號偏好；API 看 client 送的 header。admin 固定 zh-TW。

### 現況
- next-intl `^4.13.0`（`website/package.json:84`）只有空殼：`website/lib/intl/config.ts:1-12` 只有 `zh-TW`；`website/i18n/request.ts:4-10` 一律回預設訊息；`components/shared/AppProviders.tsx:50` 已經包了 `NextIntlClientProvider`（§R1）。`app/` 只有 route group，沒有 `[locale]`（重查：`ls website/app`）。
- 部署：OpenNext Cloudflare `1.20.6`（`website/package.json:110`）、`website/wrangler.jsonc`、`website/open-next.config.ts`：全站純 ISR、R2 incremental cache、`enableCacheInterception:true`，註解說攔截發生在 middleware **之後**（`open-next.config.ts` 末段，引用 `@opennextjs/aws routingHandler.js:176`）。
- `website/middleware.ts:15-18` 用 edge middleware（workerd 不支援 `proxy.ts`）；`:55-58` 是 `clerkMiddleware(...)`；`:187-190` 的 matcher 排除 `api/`。
- website 在部署時**不經 BFF**：瀏覽器直接打第一方反代 `/api/connect/*` 到單體（`website/lib/connect/transport.ts:38-41`）；本機才由 `app/api/connect`、`app/api/v1` 代行。`app/api/v1/[...path]/route.ts:86-88` 只把 `Accept-Language` 當快取 key。
- 登入只有 OAuth（Google、LINE：`packages/hawkr_auth/lib/clerk.dart:16-17`；website `features/auth/screens/login/LoginPage.tsx`），沒有 Clerk 的 email／SMS 驗證碼，也沒用 Clerk 預建元件（§R1）。
- seller-app 宣告 zh_TW、zh_CN、en_US，`locale` 寫死 zh_TW；buyer-app 沒有 delegates（§R3，未重查行號）。帳號、買家、店家、舊 `device_tokens` 都沒有語系欄（§R4）。

### 業界基準
- Google：每個語言用不同 URL，不要用 cookie 或瀏覽器設定切換同一 URL 的內容〔VERIFIED，§2〕；「Avoid automatically redirecting」「don't redirect based on what you think the user's language may be」，改成提供連到其他語言版本的連結〔VERIFIED，本次重讀 managing-multi-regional-sites，草稿標成 INFERENCE 的那一句現在確認了〕。Googlebot 不送 `Accept-Language`、IP 在美國〔VERIFIED，§2〕。
- Google 同一頁：「Translating only the boilerplate text of your pages while keeping the bulk of your content in a single language」會造成不好的使用體驗，常見於 UGC 頁〔VERIFIED，本次重讀〕。**草稿沒有處理這點**：`/en` 的商品頁、店家頁、貼文頁主體是中文 UGC，正是這種情況。
- next-intl：`always`／`as-needed`／`never`；`localeDetection:false` 完全依 URL；middleware 自動輸出 hreflang 的 `Link` header，可以關掉；Clerk 示範在 `clerkMiddleware` 裡 `return intlMiddleware(req)`〔VERIFIED，§1〕。
- Shopify：主語言在根路徑、其他語言在子目錄，結帳與通知用顧客的語言〔VERIFIED／I，§8〕——也就是 `as-needed` 的形狀。
- AIP-193 的 `LocalizedMessage` 可以依請求的語系；AIP-143 的語言欄位叫 `language_code`〔VERIFIED，§5〕。`Accept-Language` 是 CORS-safelisted header，瀏覽器的 fetch 可以自己設〔INFERENCE，憑規格記憶〕。
- Clerk 的 `localization` 只影響 Clerk 元件，Account Portal 維持英文、功能標示 experimental〔VERIFIED，本次讀 clerk.com/docs/guides/customizing-clerk/localization〕；email／SMS 模板是每個 instance 一份，文件說可以用來改成另一個語言，也可以關掉由 Clerk 寄送〔I，搜尋摘要〕。

### 為什麼現有機制存在
公司時期的「只有 zh-TW」（codemeta ADR `:38`、錯誤模型 ADR `:32`）；next-intl 是先裝了、還沒用的骨架。

### 選項：website 的 URL

| 選項 | 做法 | 取捨 |
|---|---|---|
| **a. `as-needed`＋只看 URL** | zh-TW 在 `/…`、en 在 `/en/…`；`localeDetection:false`；有切換器；不導向 | 主市場的 URL 最短；`x-default` 就是 `/`；不對稱（`/zh-TW/…` 會 308 到 `/…`），但這是 next-intl 內建行為 |
| b. `always` | `/zh-TW/…`、`/en/…` | 對稱；根路徑 `/` 要選一個導向或語言選擇頁，等於又回到「要不要依語言導向」；所有 URL 變長。上線前改 URL 沒有成本，所以「現有 URL 不變」不是理由，b 輸在根路徑問題，不是改動量 |
| c. 只用 cookie | URL 不變 | Google 明說不要；否決 |
| d. 子網域 | `en.…` | Clerk、canonical host、noindex 判斷（`lib/seo/canonical-host.ts`）都要變多主機；單一市場沒有理由 |

### 選項：API 請求的語系

| 選項 | 取捨 |
|---|---|
| **a. client transport 送單一值的 `Accept-Language: <UI 語系>`**，伺服器 `language.Matcher` 協商，信心值 No 時用 zh-TW | 不改契約；**要在 transport 設**：website `lib/connect/transport.ts:92` 的 interceptor、Dart `packages/hawkr_api/lib/transport.dart`、兩支 App 的 Dio。草稿說「BFF 覆寫」，但部署時瀏覽器直接打單體，沒有 BFF 可以覆寫（現況第 4 點）；ISR 的伺服器端 fetch 也要明確帶 locale |
| b. 每個 request 加 `language_code` | 約 100 支 RPC 改契約；適合「內容本身依語言」的讀取 |
| c. 讀帳號偏好 | 未登入沒有帳號；同一人在兩個 client 用不同語言時會錯 |

### 選項：帳號偏好怎麼更新

| 選項 | 取捨 |
|---|---|
| a. 只在使用者明確切換時改（草稿） | 值穩定；但新裝置沿用裝置英文、帳號仍是 zh-TW 時，介面是英文、通知是中文，使用者不知道哪裡改 |
| **b. 記「最後一次有效的 UI 語系」**：登入或 App 回到前景時，若 transport 送的語系和帳號不同，就更新 | 通知跟著「我最近在用的語言」；共用裝置會來回切（家庭共用的生鮮 App 有可能，但兩人通常是兩個帳號）〔INFERENCE：Shopify 的 `Customer.locale` 依顧客最近的店面語言更新，§8 未查證〕 |

### 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| 買家公開頁要被索引 | REAL（產品需求） | `website/lib/seo/indexability.ts` |
| 必須用 edge middleware，不能用 `proxy.ts` | REAL（OpenNext／workerd，第三方） | `website/middleware.ts:15-18` |
| 全站純 ISR、cache interception 在 middleware 之後 | REAL（#338 的 Worker OOM 決定，現在改是另一個題目） | `website/open-next.config.ts` |
| 部署時瀏覽器直接打單體，不經 BFF | REAL（現行部署架構；改它不在這一步） | `website/lib/connect/transport.ts:38-41` |
| admin 只有 zh-TW；英文內容之後才補 | REAL（Will 2026-10-17） | 需求 |
| 現有 URL 不變、和 `lang="zh-Hant"` 一致 | SELF-IMPOSED | — |

### 建議（只依 REAL 調整）
1. **標籤**：資料、URL、訊息檔、API 都用 `zh-TW`、`en`；Flutter 宣告 `Locale.fromSubtags('zh','Hant','TW')` 與 `en`；`<html lang>` 跟著 locale；刪 seller-app 的 zh_CN。解析規則四端相同：`zh*` → zh-TW；其他語言在 en **已發佈**時 → en，否則 zh-TW。
2. **website** 採 a，`app/` 移進 `app/[locale]/`（含賣家中心；它是 noindex，用同一套規則的成本為零）。`middleware.ts` 在 `clerkMiddleware` 的 callback 裡先跑 next-intl、再跑現有的 `siteMiddleware`（canonical 308、登入導向、noindex 都要認得 `/en` 前綴：`lib/auth/redirect.ts` 的 `requiredAccessForPath`、`lib/seller/canonical-redirect.ts`）。`setRequestLocale`＋`generateStaticParams` 只產生**已發佈**的語系，未發佈的 `/en` 在 prod 回 404。
3. **SEO（草稿漏掉）**：hreflang 只列已發佈語系，`x-default` → `/`；`/en` 的**平台撰寫頁**（首頁、分類、FAQ、賣家指南、法律頁）可以索引；`/en` 的 **UGC 頁**（商品、店家、貼文、食譜如果是店家寫的）先 `noindex` 並排除在 sitemap 外，觸發條件是 UGC 有翻譯（見 I5）。不要用跨語言的 canonical 取代 noindex。`openGraph.locale`、JSON-LD 的 `inLanguage`、標題模板裡的品牌名都依 locale（品牌英文名要 Will 定，見問題 9）。
4. **伺服器產生的連結（草稿漏掉）**：email、推播、站內通知裡的 website 連結，由伺服器依收件人的 `preferred_locale` 加前綴（`localeDetection:false` 下，沒前綴就是中文）。App 的 universal link／App Link 要同時接受 `/en/…`（AASA 與 assetlinks 的路徑規則、App 內的路由解析）。
5. **App**：預設跟裝置語言；App 內設定在 Android 用 per-app language，在 iOS 開「設定」頁；選擇存在裝置。
6. **帳號偏好**：`identity.accounts.preferred_locale`（`TEXT NOT NULL`，CHECK 在支援清單內），identity 擁有，A8 讀它；更新採 b（最後一次有效的 UI 語系）。不放 Clerk metadata：Clerk 只當身分供應者，寄信路徑不多一個外部依賴（通知 ADR D8 同一個理由）。一人一個值：同一個人同時是買家與店主時，兩邊用同一個偏好。
7. **API** 採 a，transport 送 header；單體在 `httpapi/rpc` 加 interceptor 把協商結果放進 context，回應帶 `Vary: Accept-Language`；gateway 原樣轉送；admin 的 RPC 一律 zh-TW。
8. **Clerk**：只用 OAuth，沒有 Clerk 寄出的驗證信要處理（草稿的未查證項關閉）。剩下的是 Clerk API 的英文錯誤：`hawkr_auth` 與 website 依 Clerk 錯誤 **code** 對應自己的文字，不顯示 Clerk 的 `message`。Google／LINE 的授權頁語言由對方決定（REAL，第三方）。

**終局與觸發**：出現第三個語系，或某支讀取的內容要用和 UI 不同的語言時，那支 RPC 才加 `language_code`。若日後加 email／密碼登入，Clerk 的模板要關掉 Clerk 寄送、改由 A8 依 `preferred_locale` 寄〔I〕。

---

## I2 文字資源

**建議**：一個 client 只有一套格式化執行期；共用目錄生成到這套執行期，不是另一套查表。key 不帶版本；每個 key 有說明。

### 現況
- SSOT D-L1 c（`step10e-labels-brief.md:50`）：來源 `packages/hawkr-copy/zh-TW.json`，生成各端的型別化查表；c′（生成進 next-intl／ARB）被否決的理由是「為單語系導入整套 i18n 執行期」（`:51`）；終局句（`:78`）的觸發「第二語系」現在成立。L-brief Q2 建議 key 用含版本的完整名稱。
- 各端的硬寫中文：website 1,000 個以上的檔案、buyer-app 約 3,341 行、seller-app 約 4,741 行（§R1、§R3）；共用套件的預設錯誤文字在 `packages/hawkr-api-ts/src/connect/errors.ts:95` 起與 `packages/hawkr_api/lib/errors.dart:201-209`。

### 業界基準
- gen-l10n：ARB key 必須是合法的 Dart 識別字；`@key.description` 強烈建議，可用 `required-resource-attributes` 強制；3.32 起 synthetic package 棄用、要 `generate: true`〔VERIFIED，§3〕。`generate: true` 只對執行 `flutter` 指令的那個專案跑 gen-l10n，依賴的套件要自己提交生成檔〔INFERENCE〕。
- next-intl 的 messages 是每個 locale 一份 JSON，沒有 metadata 欄位；ICU 參數的型別靠 `createMessagesDeclaration`〔VERIFIED，§1、§7〕。
- 帶說明的 JSON：WebExtension（Chrome）JSON 的 `{ "message", "description" }` 是 Weblate、Crowdin 都能匯入的格式〔INFERENCE，憑記憶，未重查〕。
- MF2 在 CLDR 是 stable，但 TC39 還在 Stage 1、Dart 沒有實作〔VERIFIED，§4〕；zh 只有 `other`，en 有 `one`／`other`〔VERIFIED＋T，§4〕。

### 為什麼現有機制存在
L 是在「只有 zh-TW」下為消除四端手抄與條款用語漂移而設計的，所以選最小的執行期；那個前提被本需求推翻，L 自己寫的觸發（`:78`）成立。

### 選項：目錄怎麼進 client

| 選項 | 做法 | 取捨 |
|---|---|---|
| a. 原本的 c：依 locale 的型別化查表 | `label(locale, v)` | client 有兩套 i18n；`plural`／`select` 要在 TS 與 Dart 各自實作 |
| b. 草稿的 c′：TS 生成 next-intl namespace；Dart 生成 ARB，再在套件裡跑 gen-l10n 產生 delegate | — | TS 部分好；Dart 是兩段生成（JSON → ARB → Dart），而且 ARB 的 key 要再轉成 camelCase，套件的生成檔仍要提交 |
| **b′. TS 同 b；Dart 由生成器直接輸出 Dart**（`extension on MeasureKind { String label(Locale l) }`，`plural` 呼叫 `intl` 的 `Intl.pluralLogic`，也就是 gen-l10n 生成碼用的同一個函式） | ARB 只留給 App 自己的文字 | Dart 只有一段生成；窮舉由 Dart 的 `switch` 保證；不需要另一個 `LocalizationsDelegate`（少碰 `main.dart`／`app.dart` 這兩個 10d 熱點）；格式化執行期仍然只有 `intl` 一套 |
| c. 目錄停用 | 各端自擁 | 失去跨端一致；否決 |

### 選項：key（重新檢討 L-brief Q2）

| 選項 | 取捨 |
|---|---|
| 含版本的完整名稱 | 升 `v1` 時全部改名，TMS 的歷史斷掉 |
| **不帶版本的 descriptor 路徑**（`seller.MeasureKind.MEASURE_KIND_WARNING`） | 穩定、機械推導、不撞名；enum 值改名本來就是語意改變，key 跟著改是對的 |
| 手寫語意 key | 要另一張對照表 |

### 建議（只依 REAL 調整）
1. 目錄採 b′、key 採不帶版本的 descriptor 路徑。admin 吃 zh-TW 的常數表（admin 沒有 next-intl 執行期，也不需要）。
2. **說明**：目錄的說明從 proto 值的註解帶入（codemeta ADR D6 規定註解只寫語意），可以在來源檔覆寫；受眾（預設／`staff`）寫進說明。
3. **來源格式**：website 與 TS 套件自己寫的文字也用帶說明的結構化 JSON（`{ "message", "description" }`），由同一支工具生成 next-intl 用的平面 JSON。不做草稿的 `*.meta.json` 側檔（兩份要對齊、TMS 能不能匯入沒查證）。App 的文字用 ARB 原生的 `@key.description`，`required-resource-attributes: true`。
4. **分界不變**：「只由契約值決定、兩個以上 client 用到」的文字進目錄（L-brief Q3），加上 Connect code 的預設錯誤文字（`common.error.<CODE>`，取代 TS 與 Dart 各一份）。其他文字在各 client：website 依 feature 分 namespace；兩支 App 各自的 ARB；`hawkr_auth` 的文字也進目錄或自己的生成檔（它是套件，不是 App）。
5. **格式**：ICU MF1 受限子集，只允許 `{arg}`、`select`、`plural`；數字與日期由各端 helper 格式化後當參數傳；帶數量的中文原文也寫 `{count, plural, other {# 件}}`；禁止拼接（例如 seller-app `measure_display.dart:190-199` 的 `$subject審核中`）。
6. **完整性以（受眾、語系）為單位**：預設受眾在每個已發佈語系都要完整；`staff` 只有 zh-TW。

**終局與觸發**：ECMA-402 與 Dart 都有穩定的 MF2 實作時再評估。

---

## I3 伺服器產生的文字

**建議**：依「誰看、何時看」決定語系；句子放在擁有它的模組，和目錄同一支生成器產生有型別的 Go 函式。

### 現況
- `backend/internal/httpapi/rpc/errors.go:28-33` 的 `errorLocale` 固定 zh-TW；`:107`、`:128` 寫入 `LocalizedMessage`。單體 `internal/modules` 有 449 個 `apperr.` 呼叫點（不含測試，本次重數），字面值是中文（§R4）。
- **兩個 client 在 `FieldViolation` 沒有 `localized_message` 時退回 `description`**：`packages/hawkr-api-ts/src/connect/errors.ts:157`、`packages/hawkr_api/lib/errors.dart:102-104`。`description` 改成給開發者看之後，不拿掉這條退路，英文的開發者訊息會出現在畫面上。
- 錯誤模型 ADR `:51`（message 不另寫英文版）、`:59`（locale 固定）、`:131`（第二語系時由請求決定）；`api/hawkr/admin/seller/v1alpha1/measure.proto:172` 寫 locale 固定 zh-TW（§R4）。
- 通知 ADR D1 文案集中、D5 存型別與參數、讀取時組字；A8 還沒開工（§R4）。
- 時間格式化：草稿說「取代 63 處 `Format("2006…")`」，重數後**單體裡只有 1 處**（`internal/legacy/buyerserver/server.go:71`），其餘 62 處在要退場的舊服務。`twtime` 的格式化 helper 仍然要做（伺服器句子裡的時間），但不是在「取代 63 處」。

### 業界基準
- AIP-193：`locale` 與 `message` 都要填；可以用請求指定的語系；`Status.message` 給開發者、應該是英文〔VERIFIED，§5〕。
- Go：`x/text/language` 解析與比對〔VERIFIED〕；go-i18n v2 支援 CLDR 複數、extract／merge，但模板是 `text/template`，不是 ICU〔VERIFIED／I，§5〕。

### 為什麼現有機制存在
只有 zh-TW，所以字面值最簡單；`LocalizedMessage` 的形狀就是為了第二語系時不必改形狀（`step10e-labels-brief.md:128`）。

### 選項

| 選項 | 取捨 |
|---|---|
| a. go-i18n v2 | 成熟；語法和 TS／Dart 的 ICU 不同，目錄名詞要跨兩種語法，譯者兩套 |
| b. `x/text/message`＋gotext | 工具鏈少人用、不是 ICU |
| **c. ICU 受限子集＋生成的 Go 函式** | 與目錄同格式、同生成器；參數是 struct，編譯期檢查；`plural` 只需 zh 與 en 兩組規則，直接寫在生成器裡即可，不必依賴 `x/text/feature/plural` |
| d. 字面值查 map | 沒有參數型別，缺翻譯到執行期才知道 |

### 建議（只依 REAL 調整）
1. 語系：同步回應依 context 的協商結果；非同步（A8 推播、email）依收件人 `preferred_locale`；收件匣讀取時依讀者語系組字；admin RPC 固定 zh-TW＋`staff`。
2. 每個模組一個 `text/` 目錄（`zh-TW.json`、`en.json`，帶說明的結構化格式），生成 `text_gen.go`；`apperr` 不再接受字面值，改收「依 tag 延後組字」的文字值；CI 擋 `apperr` 與 `FieldViolation` 的中文字面值。
3. `Status.message` 與 `FieldViolation.description` 由 `ErrorInfo.reason`＋metadata 自動產生（log 不隨請求語系變）；**同一刀拿掉 client 在 `description` 上的退路**，每個要給使用者看的 `FieldViolation` 都必須帶 `localized_message`（伺服器端用測試或 CI 擋）。
4. 通知（A8 的設計輸入）：參數快照只存原始值；email 用 `html/template` 加 locale；投遞紀錄存實際送出的文字與 locale；連結依 I1 建議 4 加前綴。
5. 推播文字由伺服器組好（不用 APNs `loc-key`／FCM `body_loc_key`：文字得放進原生資源，和 ARB 分家，也違反通知 ADR D1）。
6. 範圍外（REAL，排定的搬遷）：舊服務與 CodeMeta 的文字隨模組搬遷退場；AMEGO 電子發票維持中文（第三方、政府格式）。

**終局與觸發**：有非工程人員的譯者時，模組的 `text/*.json` 和目錄、client 資源一起交給 TMS。

---

## I4 TMS 與流程

**建議**：現在不用 TMS；repo 檔就是來源；CI 一次建好；英文由 LLM 依術語表起草、Will 審。

### 業界基準
- Weblate、Crowdin 以 git／PR 同步，支援 ARB 與 JSON；Tolgee 雲端免費 3 萬字；Lokalise 149 美元／月起〔VERIFIED／I，§6〕。
- Pseudo-localization：Microsoft 經驗值加長 40%，短字串 2–4 倍；ICU `en-XA`〔VERIFIED，§7〕。
- 缺 key：next-intl 型別化 key、`@lingual/i18n-check`；Flutter `untranslated-messages-file`〔VERIFIED／I，§7〕。

### 選項

| 選項 | 取捨 |
|---|---|
| **a. repo 檔＋CI，之後再接 TMS** | 零外部帳號；一位維護者、兩個語系，PR 審查就是翻譯流程 |
| b. 現在自架 Tolgee／Weblate | 多一個服務要維運；英文內容還沒開始，in-context 編輯沒有東西可編 |
| c. 雲端免費方案 | 要開對外帳號（每次問 Will） |

### 約束帳
沒有譯者與營運團隊（REAL，通知 ADR 前提）；開對外帳號要問（REAL，授權規則）；工具成本（SELF-IMPOSED）。

### 建議
採 a。CI 檢查：
1. **硬寫中文的 ratchet，以檔案為單位**：每個檔案的漢字字串字面值數只能減少；新檔案必須是 0。全域總數會讓「刪 A 檔、加 B 檔」過關，所以不用總數。偵測：TS 用 AST（TypeScript compiler API 走過 `StringLiteral`、`TemplateLiteral`、`JsxText`，排除註解、測試、生成檔、`messages/`）；Dart 用 `analyzer` 套件走 `StringLiteral`；Go 用 `go/ast`。不用 regex（會把註解算進去）。範圍：website（排除 admin 路由與 `(delegated)/adm`）、buyer-app、seller-app、共用套件、單體的 `apperr`。
2. ICU 語法、受限子集、各語系 key 與參數一致。
3. 每個 key 有說明。
4. 已發佈語系 100% 覆蓋；未發佈的只回報。
5. 沒人用的 key；目錄與生成檔沒有 drift。
6. Pseudo-locale（只在開發與 preview）：由 zh-TW 產生，加括號、加長，用來抓沒抽出來的字（沒有括號的就是硬寫）與被截斷的版面。Playwright 與 Flutter golden test 各跑一次 pseudo-locale（見 I8 的測試策略）。

**終局與觸發**：第一位非工程人員的譯者，或第三個語系 → 自架 Tolgee 或 Weblate（git 同步，repo 檔仍是來源）。

---

## I5 格式與內容資料

**建議**：業務時間一律台北時間；金額 TWD、0 位小數、`NT$`；UGC 不翻；平台內容用 `LocalizedText`。

### 現況（重查）
- `website/lib/date/taipei.ts` 刻意不用 `Intl`（Workers 是 UTC、ICU 版本和瀏覽器不同，#374、#391）。
- `toLocal()`：`buyer-app/lib`、`seller-app/lib`、`packages/*/lib` 共 **57 處**，另有 2 處在 `packages/hawkr_api/test/taipei_time_test.dart`（本次重數：`rg '\.toLocal\(\)'` 為 59）。其中 **8 處**後面接 `.difference(DateTime.now())`（例如 `buyer-app/lib/features/order/data/order_model.dart:218,242`、`after_sale_detail_screen.dart:697,707`、`shared/widgets/live_countdown_line.dart:97`），時間差和時區無關，**不是 bug**。其餘約 49 處把時間點換成裝置時區再顯示（例如 `buyer-app/lib/shared/utils/date_format_utils.dart:14`、`seller-app/lib/features/order/presentation/shared_order_detail_screen.dart:899,1044`）：裝置時區是台北時輸出正確，**只有裝置時區不是台北時才錯**，所以是潛在 bug，不是已觀察到的錯誤。`packages/hawkr_api/lib/taipei_time.dart:9,22-23` 是正確寫法。
- 金額：website `lib/products/pricing.ts:91` 的 `NT$ ${n.toLocaleString()}` 跟著 runtime 語系（§R1）。

### 業界基準
- W3C：時區和語系是兩個獨立因素〔VERIFIED，§11〕。`Intl.NumberFormat` 的 TWD：en `NT$1,234.50`、zh-TW `$1,234.50`，預設 2 位小數〔T，§11〕。
- UGC：Airbnb、Etsy 自動翻譯並保留原文；用 Google 翻譯必須標示〔VERIFIED／I，§8–9〕。

### 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| 配送時段、截單、營業時間都是台北時間 | REAL（單一市場） | 產品 |
| Workers 與瀏覽器的 ICU 不同 | REAL（runtime） | `taipei.ts` |
| 台灣沒有日光節約時間 | REAL | — |
| 物流單據用中文地址 | REAL（第三方） | — |
| 分類在 Java product-service，步驟 B 才搬 | REAL（排定的搬遷） | 計畫 |
| 商品檢索是 `searchable_text`＋`pg_trgm`，內容是中文 | REAL 到分類搬遷為止 | `backend/docs/詳細設計書/02-產品微服務（Product Service）詳細設計書.md:63` |

### 建議（只依 REAL 調整）
1. **時間**：四端業務時間一律台北時間。website 擴充 `taipei.ts`，加 en 的樣式表，不用 `Intl`。App 的約 49 處顯示用 `toLocal()` 改用 `taipei_time.dart`（加 `intl` 的 `DateFormat` 處理 en 月份名稱）；8 處時間差維持不變，或統一改成 `toUtc()` 避免誤讀。裝置或瀏覽器時區不是台北時，在 client 標「台北時間／Taipei time」。聊天訊息的時間也用台北時間：一致比「跟裝置走」重要，而且使用者幾乎都在台灣〔INFERENCE〕。
2. **金額**：每端一個 `formatMoney`：TWD、`maximumFractionDigits:0`、兩語系都顯示 `NT$`（zh-TW 的 CLDR 預設是 `$`，但「NT$」在台灣也通行，統一一種最簡單）。website 的 `formatMoney` 不用 `Intl`（同時間的 hydration 理由），自己插千分位。
3. **UGC**（商品名、店家介紹、貼文、評價、聊天、admin 寫給賣家的說明）：不翻、不做機器翻譯；容器標 `lang="zh-TW"`（UGC 的語言未知時用平台主語言，這是近似值）。admin 寫給英文賣家的說明（措施理由等）仍是中文：admin 只有 zh-TW（REAL），賣家端在這類文字旁不加任何「譯文」標示。觸發：英文買家比例或回饋顯示需要時，評估機器翻譯加「顯示原文」，同時解除 I1 建議 3 的 UGC 頁 noindex。
4. **檢索（草稿漏掉）**：英文介面的使用者輸入英文關鍵字，`pg_trgm` 對中文 `searchable_text` 找不到東西。現在不處理（UGC 不翻）；分類搬進單體並有 `LocalizedText` 時，把分類的英文名稱併進 `searchable_text`。英文介面的搜尋框提示文字要寫明「請用中文搜尋」之類的說明〔要 Will 定，屬 UX 文案〕。
5. **平台內容**：`LocalizedText`（JSONB `{ "zh-TW": …, "en": … }`，CHECK 必有 zh-TW），伺服器依請求語系回單一字串，缺 en 時退回 zh-TW；content 模組在 I-content 做，分類在步驟 B。
6. **地址**：存代碼，依 locale 顯示縣市與鄉鎮名稱；twaddr 的英文名稱需要官方資料來源（中華郵政的英譯，未查證）；物流單據永遠中文。

**終局與觸發**：三個以上語系或要經 TMS 同步時，`LocalizedText` 改成翻譯表 `(entity, field, locale, text)`。

---

## I6 法律文件

**建議**：中文正本＋英文參考譯本，譯本記它翻的是哪個正本版本；同意紀錄加 `displayed_locale`；「以中文為準」的語言條款寫進兩版，但它對消費者的效力**列進上線前的律師審閱**，不在 ADR 裡當成已成立的機制。

### 現況
- `website/scripts/generate-legal.ts:3-9` 只處理三份賣家文件；買家的 `/terms`、`/privacy`、`/return-policy`、`/user-policy` 是手寫 TSX（`website/features/legal/pages/*.tsx`）；沒有語言條款；同意紀錄 `seller.agreement_acceptances` 沒有 locale（§R2）。`seller-terms.md:3` 註明「未經律師審閱」。

### 業界基準
- 台北市法規英文版寫明以中文版為準〔VERIFIED，搜尋片段〕；但那是政府的法規翻譯，不是私人間的定型化契約。
- 消保法第 11 條第 2 項：定型化契約條款有疑義時，作有利於消費者的解釋；第 12 條：違反誠信、對消費者顯失公平的條款無效；第 17 條：主管機關公告的應記載、不得記載事項〔INFERENCE，憑條文記憶；本次只查到第 17 條與第 56-1 條的摘要〕。是否有「定型化契約必須用中文」的明文：**本次重查仍沒找到**（搜尋 `laws.gov.taipei` 的消保處函釋頁只列相關法條）。
- 網路零售業的定型化契約有經濟部公告的應記載與不得記載事項〔INFERENCE，未查證名稱與現行版本〕：買家條款要對照它，這和語言無關，但同一次律師審閱要一起做。
- 推論：英文使用者看的是英文頁，若英文譯本比中文對消費者有利，依第 11 條第 2 項，「以中文為準」可能不被採用〔INFERENCE，要律師確認〕。所以真正的風險控制是**譯本的品質與版本對齊**，不是那一句條款。
- 賣家條款的對象多半是商業經營者（不適用消保法的消費者定義），買家條款才是消費者契約〔INFERENCE〕。兩者的風險不同，律師審閱分開看。

### 選項

| 選項 | 取捨 |
|---|---|
| **a. 中文正本＋英文參考譯本，兩版寫語言條款；同意的對象是中文正本版本** | 一份有拘束力的文字；版本一條線；條款效力待律師確認 |
| b. 兩種語言同等效力 | 每次修訂審兩份；不一致時沒依據 |
| c. 不提供英文 | 違反需求 |

### 建議
採 a：
1. 正本 `website/docs/legal/<doc>.md` 加 `<doc>.en.md`，譯本 front matter 寫 `translationOf: <正本版本>`；en 已發佈時，產生器要求它等於正本現行版本，否則生成失敗。
2. 四份買家文件改成 Markdown 正本，走同一支產生器。
3. 英文頁最上方固定一行「參考譯本、以繁體中文版為準」並連到中文版。
4. 同意紀錄保留 `kind`、`version`（中文正本），加 `displayed_locale` 與 `displayed_translation_version`（譯本對應的正本版本；不一致在生成時已被擋，所以這欄是證據，不是判斷依據）。
5. 律師審閱清單加三項：語言條款對消費者的效力、買家條款對照網路零售的應記載與不得記載事項、隱私告知（個資法第 8 條）對英文使用者是否要用他看得懂的語言告知〔INFERENCE〕。

---

## I7 刀 L 與計畫怎麼重排

### 和草稿最大的不同：L′ 不必等 `[locale]` 搬遷
- website 已經有 `NextIntlClientProvider`（`AppProviders.tsx:50`），`i18n/request.ts` 已經回傳 messages。L′ 只要把目錄生成的 namespace 併進這份 messages，元件就能 `useTranslations('enum')`；語系固定 zh-TW，等 I-web-0 接上路由後自動依 URL。
- Dart 採 I2 的 b′（直接生成程式碼、不需要 delegate），App 端只需要讀 `Localizations.localeOf(context)`，不必先做 I-app-0 的整套接線。
- 所以 L′ 不依賴 10d 正在改的熱點，3b 不會被 10d 擋住。草稿把 L′ 排在 I-web-0／I-app-0 接線之後，等於讓 3b 等 10d。

### `[locale]` 搬遷什麼時候做
- 搬遷是 `git mv app/(…) app/[locale]/(…)` 加 root layout 調整。git 的 merge-ort 會偵測改名，10d 只改內容、沒有改名的檔案，合併時會套到新路徑〔INFERENCE，git 的 rename detection；相似度低於門檻時會變成衝突〕。所以不需要「10d 暫停」，只需要在 10d 的一個 commit 邊界（它的 website 改動已合進 main、還沒開始下一段 website 改動時）做，並告知 10d 先 rebase。
- 現在做還是等模組搬完再做：搬遷的成本幾乎不隨時間增加（整個目錄一起搬），但越晚做，越多新頁面是用沒有 locale 的寫法（`Link`、`redirect`、`urls.ts`）寫的，之後要再改一次。建議**早做**：L′ 之後、3b 之前或與 3b 並行。

### L-brief 各題怎麼變（Q1／Q3／Q4／Q5 仍待 Will 回答，L0 被它們擋住）

| 題目 | L-brief 建議 | 現在 |
|---|---|---|
| Q1 受眾 | 漂移收斂、視角差異用 `staff` | 不變；`staff` 只有 zh-TW |
| Q2 key 與格式 | 含版本的完整名稱、ICU MF1、不處理複數 | **推翻**：不帶版本的 descriptor 路徑；每個語系一份；說明從 proto 註解帶入；受限 ICU 子集；要處理複數 |
| Q3 收錄範圍 | 契約值決定、兩端以上使用 | 不變，加 Connect code 預設錯誤文字；組合文字寫成整句 `select` |
| Q4 完整性 | 以 enum 為單位、規則排除伺服器文字的 enum、CI 擋手寫表；`kyc_status` 改 `optional` | 不變；每個已發佈語系都要有通用的「未知」 |
| Q5 正式用語 | 對齊條款：措施／申覆／解除請求 | 不變，加英文術語表（下） |
| Q6 生成器 | Go 讀 buf image，套件 `uitext` | 生成器不變；輸出改成 TS namespace＋key helper、Dart 直接生成、Go 函式；同一支也處理模組的 `text/` |
| Q7 切分 | L0–L8 | 照下表 |

**英文術語（草案）**：措施＝restriction；申覆＝appeal；解除請求＝request to lift；消費者申訴＝customer complaint；認定＝finding；警告＝warning；終止合作＝termination；代行＝**待定**（assisted operation 讀起來不像產品用語，候選：「managed by Hawkr」「operated on your behalf」）。DSA 第 20 條的「internal complaint-handling system」是讓使用者對平台的決定（移除、限制、停權、終止）提出 complaint〔VERIFIED，本次搜尋 springlex.eu 的條文摘要〕。所以 DSA 的 complaint 等於我們的申覆。面向使用者的英文用 appeal（平台常用，避免和消費者申訴撞名）；術語表要寫明這個對應。

### 已定案中被推翻或修改的部分
- `step10e-labels-brief.md`：D-L1 的 c 改成 b′；`:78` 的觸發成立；D-L2 不變，只是 `LocalizedMessage.locale` 改成依請求。
- `step10e-L-brief.md` 知會的「不接 next-intl、不做 `Accept-Language`」：推翻。
- 計畫 `2026-09-29-a3-buyer-seller.md:359` 末句：觸發成立，改成連到多語系 ADR。
- ADR 文末修訂（不改原文）：codemeta `:38`、`:87`；錯誤模型 `:32`、`:51`、`:59`；通知 ADR D1。
- 3a 已提交的部分：admin 四個 `MeasureActionUnavailability` 屬 `staff`、只有 zh-TW，不用改；`measure.proto:172` 的註解改成「admin RPC 固定 zh-TW」。

### 新的切分（每一刀都要綠）

| 刀 | 內容 | 依賴與協調 |
|---|---|---|
| **I0** | 多語系 ADR（I1–I6）、各 ADR 文末修訂、術語表、語系標籤與 `published locales` 的定義 | 純文件 |
| **L0** | 文案核可（L-brief Q1、Q5）＋英文術語 | Will |
| **L′** | 原 L1–L8 改成多語系形狀：目錄 `zh-TW.json`＋`en.json`（en 可以空）、key、說明、生成器（TS namespace 併進現有 messages、Dart 直接生成、Go）、CI；client 的 `errors` 拿掉 `description` 退路也可以放這刀 | 只依賴 I0、L0；**不依賴** I-web-0／I-app-0 |
| **3b** | 原計畫 | L′ 之後 |
| **I-web-0** | `app/[locale]` 搬遷（只搬檔的 commit）、routing（`as-needed`、關偵測）、middleware 組合、`urls.ts`／sitemap／hreflang／UGC 頁 noindex／`lang`、transport 送 header | 10d 的 commit 邊界；先 spike 驗證 next-intl middleware 在 workerd 與 cache interception 下正常（`/` 改寫到 `[locale]=zh-TW` 後仍命中 ISR） |
| **I-app-0** | 兩支 App：`l10n.yaml`（`synthetic-package:false`）、ARB 骨架、`supportedLocales`、解析規則、App 內語言設定、transport header、iOS `CFBundleLocalizations` 與 `InfoPlist.strings`（權限說明）、Android locale config、universal link 接受 `/en`、約 49 處 `toLocal()`、`formatMoney` | 10d 的 App 改動合進 main 之後 |
| **I-server** | locale interceptor、context、`LocalizedMessage` 依 tag、模組 `text/`、`apperr` 不收字面值、`message` 與 `description` 自動產生、`twtime` 格式化 | 只動單體；449 個呼叫點逐模組改，CI ratchet 擋新增 |
| **I-account** | `preferred_locale`、更新規則（I1 建議 6）、讀取 RPC | identity；A8 讀它 |
| **I-CI** | I4 的檢查 | 和 L′、I-web-0、I-app-0 一起長出來 |
| **I-legal** | I6 | website 與 seller 模組 |
| **I-content** | content 模組的 `LocalizedText`；分類在步驟 B | — |
| **I-sweep-*** | 依區域抽字串（website 買家頁、賣家中心、buyer-app、seller-app），每區一刀 | 排在該區的遷移步驟之後；之後動到哪個畫面，同一個 commit 把它抽成資源 |

**英文發佈門檻**：某個介面在英文下看不到任何未翻譯的文字，才能把 en 加進那個介面的 `published locales`；實際上會落在相關模組搬進單體、舊 REST 與 CodeMeta 文字退場之後（REAL：現在改舊服務是丟棄式工作）。替代方案是「缺翻譯就退回中文」（Shopify 的做法），可以更早發佈，代價是英文使用者看到中英混雜，這是產品取捨（問題 5）。

---

## I8 草稿漏掉的面向

| 面向 | 建議 |
|---|---|
| App Store／Play 的商店頁 | 商店標題、描述、截圖要 en 版；iOS 權限說明（相機、相簿、定位）在 `InfoPlist.strings` 依語系；Android 權限說明在 `res/values-*`。放進 I-app-0 |
| 字型與排版 | website 字型堆疊要有 Latin 字型（CJK 字型的 Latin 字形通常不理想）；按鈕與標籤允許換行或用 design token 的最小寬度；pseudo-locale 會抓出截斷。英文段落行寬約 60–75 字元〔INFERENCE〕 |
| 無障礙 | `<html lang>` 跟 locale；UGC 容器 `lang`；Flutter 用 `Semantics` 的 `locale` 或 `Localizations` 讓螢幕閱讀器用對語音 |
| 品牌名 | 英文介面的品牌名（例如「Hawkr」）與標題模板，要 Will 定 |
| 測試策略 | 生成器：單元測試（ICU 子集解析、缺值失敗、複數）；Go：每個模組的 `text` 在 zh-TW 與 en 都有測試；website：Playwright 跑一次 `/en`（preview 開 en）與 pseudo-locale，檢查沒有未抽的字、版面沒溢出；App：widget／golden test 跑 zh-TW 與 pseudo-locale；transport：測 header 一定存在且等於 UI 語系；伺服器：測 `Accept-Language` 協商（`fr` → zh-TW、`zh-HK` → zh-TW、`en-US` → en）|
| RTL | 不需要（只有 zh 與 en） |

---

## Will 要回答的問題

1. **URL**：`as-needed`（zh-TW 在 `/`、en 在 `/en`）、任何頁都不依瀏覽器語言導向、賣家中心也用同一套？（建議：同意）
2. **`/en` 的 UGC 頁**：商品、店家、貼文頁在英文介面可以看，但先 noindex、不進 sitemap，等 UGC 有翻譯才開放索引？（建議：同意；Google 明說只翻外框會造成不好的體驗）
3. **帳號偏好的更新規則**：記「最後一次有效的 UI 語系」，還是只在使用者明確切換時改？（建議：前者，通知跟著使用者最近用的語言）
4. **目錄的形狀**：SSOT D-L1 的 c 改成 b′（TS 生成 next-intl namespace、Dart 直接生成程式碼、Go 函式），key 用不帶版本的 descriptor 路徑、說明從 proto 註解帶入？（建議：同意；推翻 L-brief Q2）
5. **英文發佈門檻**：該介面沒有任何未翻譯文字才發佈（較晚、乾淨），還是允許缺翻譯時顯示中文（較早、中英混雜）？（建議：前者；這個市場沒有英文使用者在等，混雜的代價比晚發佈高）
6. **開發者訊息**：`Status.message` 與 `FieldViolation.description` 改由 `ErrorInfo` 自動產生，client 拿掉 `description` 退路？（建議：同意；修訂錯誤模型 ADR `:51`）
7. **排程**：I0 → L0 → L′ → 3b，`[locale]` 搬遷在 10d 的 commit 邊界早做，不等模組搬完？（建議：同意）
8. **L-brief 的 Q1、Q3、Q4、Q5**：仍待回答，L0 被它們擋住；建議照 L-brief 原建議，加上本簡報 I7 表格的增補。英文術語照草案，「代行」的英文另提候選給你選。
9. **品牌英文名與英文介面的搜尋提示**：要定一個英文品牌名；英文介面的搜尋框要不要提示「請用中文搜尋」？（建議：都要；文字在 L0 一起審）
10. **法律**：中文正本＋英文參考譯本＋`displayed_locale`，語言條款的效力與另外兩項列進上線前的律師審閱？（建議：同意）

## 知會（照建議做，有異議再說）
- 標籤 `zh-TW`、`en`；Flutter `zh_Hant_TW`；刪 seller-app 的 zh_CN；`<html lang>` 跟 locale（現在 website 與 admin 寫 `zh-Hant`）。
- 伺服器 matcher 信心值 No 時自己退回 zh-TW（Go 不會退到清單第一個，§5 T）；預設語系 zh-TW，不是 AIP-193 的 en-US（主市場是台灣，REAL）。
- admin 只有 zh-TW：不裝 next-intl、吃目錄的 zh-TW 常數表、RPC 固定 zh-TW。
- Accept-Language 由 client transport 送，不在 BFF 覆寫；回應帶 `Vary`。
- 推播由伺服器組字，不用 loc-key。
- Clerk 只用 OAuth，沒有 Clerk 寄的信要本地化；Clerk 錯誤依 code 對應自己的文字。
- 時間一律台北時間；App 約 49 處顯示用的 `toLocal()` 在 I-app-0 改。
- 金額 TWD、0 位小數、兩語系都 `NT$`；website 不用 `Intl` 格式化金額。
- UGC 不翻；平台內容 `LocalizedText`；地址存代碼、物流單據中文；AMEGO 發票中文。
- seller-app OCR 的 `'language': 'zh-Hant'` 是辨識語言，不用改。

## 未查證
- next-intl middleware 在 workerd 的 edge middleware 與 OpenNext cache interception 下是否正常（I-web-0 的第一個 spike）。
- iOS 裝置設成繁體中文（台灣）時解析到哪個 locale；Android 13 per-app language 對 Flutter 要不要手寫 `locale_config.xml`。
- `generate: true` 在依賴套件不會自動跑 gen-l10n（推論；b′ 不受影響）。
- WebExtension JSON 格式在 Weblate／Crowdin 的支援（憑記憶）。
- 消保法第 11、12 條的條文內容（憑記憶）；網路零售業應記載與不得記載事項的現行名稱；「以中文為準」條款的效力；個資法告知的語言。
- twaddr 英文名稱的官方來源；Shopify `Customer.locale` 的更新規則。
- git merge-ort 對「改名＋內容修改」的合併在這次搬遷規模下的實際表現（I-web-0 前用 10d 的分支試合併一次）。

---

## 附錄：和草稿的分歧

1. **基準 HEAD**：草稿寫 `0b9e4675e`，現在是 `a7d99873f`（10d 3a-①b 已經合進來）。
2. **L′ 的依賴**：草稿讓 L′ 等 I-web-0／I-app-0 的接線，等於讓 3b 等 10d；現有的 `NextIntlClientProvider` 已經足夠，Dart 直接生成也不需要 delegate（I7）。
3. **Accept-Language 在哪裡設**：草稿說 BFF 覆寫；部署時瀏覽器直接打單體（`website/lib/connect/transport.ts:38-41`），要在 transport interceptor 設（I1）。
4. **`/en` 的 UGC 頁**：草稿沒處理；Google 明說只翻外框不好（本次重讀原頁），建議先 noindex（I1 建議 3）。
5. **自動導向**：草稿標成 INFERENCE 的 Google 指引，本次在原頁確認（「Avoid automatically redirecting」）。
6. **伺服器產生的連結要帶語系前綴、App 的 universal link 要接受 `/en`**：草稿沒寫（I1 建議 4）。
7. **`FieldViolation.description` 的退路**：草稿把開發者訊息改成自動產生，但沒發現兩個 client 會在沒有 `localized_message` 時顯示 `description`（`errors.ts:157`、`errors.dart:102-104`），不一起改會讓開發者英文出現在畫面上（I3）。
8. **Dart 的目錄**：草稿是 JSON → ARB → gen-l10n 兩段生成加一個 delegate；b′ 直接生成，少一段、少碰熱點檔（I2）。
9. **`*.meta.json` 側檔**：改成帶說明的結構化 JSON 來源，生成 next-intl 的平面 JSON（I2、I4）。
10. **`toLocal()` 57 處**：數字對（lib 57、測試 2），但其中 8 處只算時間差、不是 bug；其餘是「裝置時區不是台北才錯」的潛在 bug，草稿寫成「現存的 bug」語氣太強（I5）。
11. **「取代 63 處 `Format("2006…")`」**：單體裡只有 1 處，其餘 62 處在要退場的舊服務（I3）。
12. **ratchet 的單位**：草稿用總數；總數會讓「刪 A 加 B」過關，改成以檔案為單位、新檔必須 0，並用 AST 而不是 regex（I4）。
13. **帳號偏好的更新規則**：草稿只在明確切換時改；新裝置會出現介面英文、通知中文的狀況，建議記最後一次有效語系（I1、問題 3）。
14. **Clerk**：草稿列為未查證的「Clerk 驗證信的語系」不適用——只用 OAuth；真正要處理的是 Clerk 的英文錯誤訊息（I1 建議 8）。
15. **法律**：草稿把「以中文為準」當成機制；本次仍找不到法源，而且消保法的有利解釋原則可能讓它對消費者不生效，所以改成「寫進去，但效力列進律師審閱」，另加網路零售應記載事項與個資告知兩項（I6）。賣家條款與買家條款的適用法不同，分開看。
16. **`[locale]` 搬遷需要 10d 暫停**：git 的改名偵測讓它只需要一個 commit 邊界（推論，要試合併驗證）（I7）。
17. **術語**：DSA 第 20 條的 complaint 是對平台決定的申覆，本次查到條文摘要；「代行」的 assisted operation 不像產品用語，另提候選（I7）。
18. **草稿漏掉的面向**：商店頁與權限說明的本地化、英文檢索、字型、品牌英文名、測試策略（I8、I5 建議 4）。
19. **Plural 的 Go 實作**：不必依賴 `x/text/feature/plural`，兩個語系的規則寫在生成器裡即可（I3）。
