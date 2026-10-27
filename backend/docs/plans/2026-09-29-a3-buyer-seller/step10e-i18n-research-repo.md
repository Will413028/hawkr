# i18n 簡報的原始筆記（2026-10-17）

worktree `hawkr-10e`，HEAD `0b9e4675e`。只讀。路徑相對於 repo 根。網路來源的逐條筆記在 `step10e-i18n-research-web.md`（同目錄），本檔 §W 是摘要。

## R1 website
- next-intl 4.13（`website/package.json:84`），plugin `website/next.config.ts:6,18`；`website/i18n/request.ts:4-10` 一律回 `defaultMessages`；沒有 `routing.ts`／`defineRouting`。
- `website/lib/intl/config.ts:1-12`：`defaultLocale="zh-TW"`、`locales=[defaultLocale]`。
- `website/messages/zh-TW.json`：19 行 scaffold；`useTranslations`／`getTranslations` 呼叫 0 處（`website/features/invoices/i18n.ts:4` 的註解自己也這麼說）。各 feature 自建中文常數物件（`features/invoices/i18n.ts`、`features/seller-center/products-import/i18n.ts:5`）。
- `website/app/layout.tsx:79` `lang="zh-Hant"`（與 config 的 `zh-TW` 不同字）；`:195` `AppProviders locale={defaultLocale}`；`components/shared/AppProviders.tsx:50` `NextIntlClientProvider`。
- `website/middleware.ts`：Clerk `clerkMiddleware`；`:15-18`「OpenNext / workerd 不支援 Next 16 的 Node-runtime proxy（proxy.ts）」→ edge middleware；做 agent deep link gate、`/sellers/{id}` 308 canonical、登入導向、非 canonical host noindex。沒有 locale。
- `app/` 只有 route groups，沒有 `[locale]`：買家 `(public)`、`(commerce)`、`(account)`、`(auth)`；內容 `(content)`（faq、privacy、terms、return-policy、user-policy、seller-guide、seller-terms）；`(seller)/seller`（行銷）與 `(seller)/seller-center/*`；`(delegated)/adm`、`vendor/rfq`、`(canary)`。
- 含漢字的 .ts/.tsx（不含測試）：app 168/206、features 503/585、components 93/130、lib 242/393。
- SEO：`lib/seo/metadata.ts:123,196` `openGraph.locale:"zh_TW"`；`:186-192` 只有 canonical；全站沒有 `languages`／hreflang。`lib/seo/urls.ts` 是 URL 唯一來源；`lib/seo/indexability.ts` 的 `PRIVATE_PREFIXES`、`INDEXABLE_STATIC_ROUTES`（15）餵 `app/sitemap.ts`、`app/robots.ts`。`step10e-seo-brief.md` 只談店家頁 meta（標題模板含「Hawkr」），不談語系。
- 日期：`lib/date/taipei.ts:1-30` 刻意不用 `toLocale*`／`Intl`：Workers runtime 是 UTC、ICU 版本與瀏覽器不同 → hydration mismatch（#374、#391）；固定 +08:00 自己組字串；輸出含 `M月D日`（`:276`）。
- 金額：`lib/products/pricing.ts:91,93,141` `` `NT$ ${n.toLocaleString()}` ``（runtime locale）；`NT$` 字面 website 178、admin 32；裸 `toLocaleString()` 132（website＋admin）；引號內 `'zh-TW'` website 36（30 檔）、admin 2。
- Accept-Language：website BFF `app/api/v1/[...path]/route.ts:86-88` 只用在快取 key；`app/api/public/seller-leads/route.ts:300` 對外寫死；admin BFF 轉送（`admin/src/features/delegated-operations/server/bff-boundary.ts:22`）。沒有人依它選語系。
- Clerk：`ClerkProvider` 在 `app/layout.tsx`；沒有用 `<SignIn>` 等預建元件（只有 prototype `features/procurement-rfq-prototype/ExternalVendorPortal.tsx`），沒有 `localization` prop。

## R2 legal
- `website/scripts/generate-legal.ts:3-9`：來源 `website/docs/legal/{seller-terms,seller-kyc-notice,seller-food-registration-notice}.md`＋`seller-agreements.json`；輸出 `features/legal/documents/*.gen.ts`、`sellerAgreements.gen.ts`、`backend/internal/modules/seller/internal/model/agreement_gen.go`。版本＝正本的生效日（`seller-terms.md:3`「版本：2026-10-16（未經律師審閱…）」），不記 hash；`--check`。
- 買家的 `/terms`、`/return-policy`、`/user-policy`、`/privacy` 是手寫 TSX（`website/features/legal/pages/*.tsx`，284／291／519／310 行），不經產生器；`PrivacyPage` 內嵌兩份生成的賣家告知。
- 沒有「以中文為準」條款；`seller-terms.md:227-231` 只有準據法（中華民國法律）與管轄（桃園地院）；`:240`「以雙方可下載、保存及日後查驗的同一版本為準」。
- 同意紀錄 `backend/internal/modules/seller/db/schema.sql:1277+` `seller.agreement_acceptances(kind, version, account_id, accepted_at, client_ip, user_agent…)`，沒有 locale；`model/agreement.go:79` 版本不是現行就 `ReasonAgreementOutdated`。

## R3 Flutter
- 兩支 App 都沒有 `l10n.yaml`、ARB、`generate: true`、直接的 `intl` 依賴。
- buyer-app：沒有 `flutter_localizations`；`lib/main.dart:389-392` `MaterialApp.router` 沒有 delegates／supportedLocales／locale。
- seller-app：`pubspec.yaml:36` `flutter_localizations`；`lib/app.dart:313-323` supportedLocales zh_TW、zh_CN、en_US，但 `locale: const Locale('zh','TW')` 寫死（:323）。
- 硬寫中文字串行（不含註解與生成檔）：buyer 190 檔約 3,341 行；seller 268 檔約 4,741 行；packages 22 檔約 213 行（多為 twaddr）。
- 時間：沒有 `DateFormat`／`NumberFormat`；`packages/hawkr_api/lib/taipei_time.dart:9` 固定 UTC+8（只有 seller-app 用，`measure_display.dart:2,257,276`）；其餘 57 處 `toLocal()`（裝置時區）手組，如 `buyer-app/lib/shared/utils/date_format_utils.dart:14`、`seller-app/lib/features/order/presentation/shared_order_detail_screen.dart:899,1044`。
- 金額：`buyer-app/lib/shared/utils/amount_utils.dart:38,44` `formatNTD`；兩支 App 各一份 `price_display.dart`。
- 請求沒有 locale header：`packages/hawkr_api/lib/transport.dart:91,197,55`；Dio `buyer-app/lib/core/networking/dio_provider.dart:74`、`seller-app/.../dio_provider.dart:44`；裝置註冊只送 token、platform（`buyer-app/lib/features/notification/data/push/device_registrar.dart:47`、seller `:77`）。
- 推播文字來自伺服器（`seller-app/lib/features/notification/data/push/manager.dart:351-352`、`buyer-app/.../background_handler.dart:85-95`）。
- 共用套件的使用者文字：`packages/hawkr_api/lib/errors.dart:201-209`（code 預設文字）、`submit.dart:58`；`packages/hawkr_auth` `clerk.dart:237,273,283,315,320`、`oauth.dart:52,78,90,98,100`。強制更新畫面 `buyer-app/lib/features/app_update/presentation/force_update_screen.dart:26-29,46`。
- twaddr CSV 只有中文欄（`county_code,county_name,town_code,town_name`），沒有英文名。
- seller OCR 寫死 `'language': 'zh-Hant'`（`seller-app/lib/features/seller/data/universal_ocr_service.dart:91`）。

## R4 backend
- `internal/httpapi/rpc/errors.go:29` `const errorLocale = "zh-TW"`；`:32-33` `LocalizedMessage(text)`；固定傳輸文字 `:37-40`。模組文字經 `apperr.Error.Message`、`FieldViolation.Message`（`internal/platform/apperr/apperr.go:124-140`「Message 會回給使用者」）；`internal/modules` 約 449 個 `apperr.` 呼叫點。
- 漢字字串行：modules 980（seller 469、delegation 170、identity 143、content 81、buyer 50…）；platform 1298（codemeta 825、twaddr 371）；httpapi 208；legacy 190；services：ai 1584、pricebook 442、invoice 256、seller 198、social 62、notification 22。
- 沒有任何 Accept-Language、go-i18n、`language_code`；`x/text` 只用於 Unicode 正規化。
- `api/hawkr/admin/seller/v1alpha1/measure.proto:172` 註解「locale 固定 zh-TW」。
- 舊 notification-service：模板存 DB 單一中文欄（`services/notification-service/db/schema/schema.sql:49-59`），77 個 migration 種子；收件匣存渲染後文字（`:21-22`、`:99-100`）；`device_tokens`（`:1-12`）沒有 locale。ADR `2026-10-05-notification-module.md` D1（:53）「zh-TW 文案集中在一個訊息目錄」、D5「存型別與參數、讀取時組文字」、`:41` 否決存渲染快照的理由之一是「日後多語系不能回溯」；A8 尚未開工。
- Email：只有 vendor invite（`services/notification-service/internal/worker/vendor_invite.go:76-78` Sprintf；到期時間原樣字串 `:107-108`）。PDF：電子發票由 AMEGO 產生（`services/invoice-service/internal/client/amego/client.go:617`），repo 沒有 PDF 產生器。SMS：Go 端沒有三竹 client。
- 時間：`internal/platform/twtime/twtime.go:13` `Taipei`（內嵌 tzdata），無格式化函式；63 處 `Format("2006…`。`backend/AGENTS.md:69` 時間以 UTC 為準。
- 帳號：`identity.accounts`（`internal/modules/identity/db/schema.sql:28-41`）只有 clerk_user_id、email、狀態時間；`buyer.buyers`、`seller.stores` 都沒有語系欄。
- 平台撰寫的內容都是單一語言欄：`category.name`（Java product-service `V1__init_product_schema.sql:21-36`）、`holidays.name`、`buyer.membership_plans.display_name`（`buyer/db/schema.sql:94-97`）、`content.banners.title`、`recipes.title`、`recipe_ingredients.name`、`app_version_gates.notice_title`（content schema `:14,49,72,91`）。
- ADR：codemeta `:38` 只有 zh-TW、`:87` 重新評估條件「需要第二個語系」；錯誤模型 `:32` 只有 zh-TW、`:51` message 不另寫英文版、`:59` `locale` 固定 `zh-TW`、`:131` 第二語系時 locale 由請求決定。
- 計畫 `backend/docs/plans/2026-09-29-a3-buyer-seller.md:359` 末句「第二語系或非工程人員改文案時改用 TMS 並依 `Accept-Language` 選語系」。

## R5 並行
- 10d 在 `hawkr-10d3a1b` worktree 做 social 第 3a 刀（website 貼文頁、buyer-app、seller-app 的社群畫面、gateway），接著 3a-②、身分第 1、2 刀（`step10d-3a-actor-brief.md`）。
