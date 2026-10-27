# i18n research: web notes (2026-10-17)

Legend: **V** = I read it on the page (quotes come from WebFetch extraction of that page). **I** = inference, search summary, or my own knowledge, not read on a primary page. **T** = I tested it locally.

## 1. next-intl routing

- V https://next-intl.dev/docs/routing/configuration
  - localePrefix `always` (default): "By default, pathnames always start with the locale (e.g. `/en/about`)."
  - `as-needed`: "no prefix for the default locale (e.g. `/about`) while keeping it for other locales"; caveat "make sure that your matcher detects unprefixed pathnames."
  - `never`: for when "You want to use domain-based routing and there's only one locale per domain"; alternate links "are disabled in this mode since URLs might not be unique per locale."
  - localeCookie: sets "a session cookie called NEXT_LOCALE"; sameSite "lax so that the cookie can be set when coming from an external site"; "does not set the `max-age` value of the cookie, making it a session cookie".
  - localeDetection: false lets you "rely entirely on the URL"; "This will disable locale detection based on the `accept-language` header".
  - domains: "Locales are required to be unique across domains"; "If no domain matches, the middleware will fall back to default locale matching based on prefixes".
  - alternateLinks: the middleware "automatically sets the `link` header to inform search engines"; opt out "by setting `alternateLinks` to `false`" when "You have pages that are only available for certain locales". x-default "is reserved for language selection & detection".
- V https://next-intl.dev/blog/next-intl-4-0 (v4 changes)
  - "The locale cookie now defaults to a session cookie that expires when the browser is closed."
  - "The locale cookie is now only set when a user switches to a locale that doesn't match the accept-language header."
  - "A locale can now only be used for a single domain." "Each domain now must specify its `locales`."
  - The x-default link now also appears on sub pages with `localePrefix: 'always'`.
- V https://next-intl.dev/docs/routing/middleware
  - "`proxy.ts` was called `middleware.ts` up until Next.js 16."
  - Detection order: (1) "A locale prefix is present in the pathname", (2) "A cookie is present that contains a previously detected locale", (3) "A locale can be matched based on the `accept-language` header", (4) "As a last resort, the `defaultLocale` is used". Matching uses the "best fit" algorithm of `@formatjs/intl-localematcher`.
  - Matcher: the middleware "is intended to only run on pages, not on arbitrary files" (the sample excludes api, _next, _vercel, and paths with a dot).
- V https://clerk.com/docs/reference/nextjs/clerk-middleware
  - "You can combine other Middleware with Clerk's Middleware by returning the second Middleware from `clerkMiddleware()`." The example runs `return intlMiddleware(req)` inside the `clerkMiddleware(async (auth, req) => {...})` callback.
  - "If you're using Next.js ≤15, name your file `middleware.ts` instead of `proxy.ts`."
- V https://next-intl.dev/docs/routing/setup
  - Without setRequestLocale, next-intl "will currently opt into dynamic rendering when APIs like `useTranslations` are used in Server Components."
  - "By using `setRequestLocale`, you can provide the locale that is received in layouts and pages via `params` to `next-intl`." The locale "should be validated" (hasLocale + notFound()). generateStaticParams lets "the routes ... be rendered at build time."
  - `next/root-params` "is available by default in Next.js 16.3 and later"; for the legacy API "it is recommended to use `next/root-params` instead."
- V https://next-intl.dev/docs/usage/configuration
  - Missing messages are handled through `onError` and `getMessageFallback`; the docs example treats `IntlErrorCode.MISSING_MESSAGE` as expected and only logs it. For fallback, the docs suggest "merging messages e.g. from the default locale with the ones for the current locale".
  - timeZone: "Specifying a time zone affects the rendering of dates and times." By default "the time zone of the server runtime will be used".
- I: With `as-needed`, zh-TW lives at `/` and English at `/en`, so zh-TW URLs need no changes. With `always`, every URL changes to `/zh-TW/...`. Both setups give each language its own URL, which is what Google requires (section 2).

## 2. Google Search Central

- V https://developers.google.com/search/docs/specialty/international/managing-multi-regional-sites
  - "Google recommends using different URLs for each language version of a page" "rather than using cookies or browser settings to adjust the content language on the page".
  - "If you prefer to dynamically change content or reroute the user based on language settings, be aware that Google might not find and crawl all your variations."
  - Subdirectories are an option (`example.com/de/`). URL parameters are "Not recommended."
- I (from memory, not re-read this session): the same page also advises against automatic redirects based on perceived language. Offer links to the other language versions instead.
- V https://developers.google.com/search/docs/specialty/international/localized-versions
  - "Each language version must list itself as well as all other language versions."
  - x-default is "used when no other language/region matches the user's browser setting."
  - Codes: "the language code (in ISO 639-1 format)" plus an optional "region code (in ISO 3166-1 Alpha 2 format)". Scripts use ISO 15924, e.g. "`zh-Hant`: Chinese (Traditional)".
  - "There are three ways": HTML `<link>`, the HTTP `Link:` header, and a sitemap.
- V https://developers.google.com/search/docs/specialty/international/locale-adaptive-pages
  - "the default IP addresses of the Googlebot crawler appear to be based in the USA."
  - "the crawler sends HTTP requests without setting `Accept-Language` in the request header."
  - "We recommend using separate locale URL configurations and annotating them with" rel=alternate hreflang.
- I: Consequence: if `/` negotiated its language from Accept-Language or a cookie, Googlebot (which sends no Accept-Language) would always get the default locale. That is safe when the default is zh-TW, but the English version would only be indexed through `/en` URLs plus hreflang.

## 3. Flutter gen-l10n / ARB

- V https://docs.flutter.dev/ui/internationalization
  - untranslated-messages-file: "Using this option creates a JSON file at the target location" in the format `"locale": ["message_1", ...]`. Without it, "a summary of the messages that haven't been translated are printed on the command line."
  - nullable-getter: "By default, this value is true ... If this value is false, then a null check is performed on the returned value of `Localizations.of(context)`".
  - The page still describes `synthetic-package` as "`true` by default" (the docs are stale; see the next entry).
  - ARB metadata: `"@hello": {"description": ..., "placeholders": {"userName": {"type": "String", "example": "Bob"}}}`. On metadata: "it's highly recommended as this provides context for the meaning of a message". There is a `required-resource-attributes` option to require it.
  - Plural: `{count, plural, =0{..} =1{..} few{..} many{..} other{..}}`, and "Only the `messageOther` field is required." Select: "comparison ... is case-sensitive."
  - supportedLocales: the app follows a device locale change "only ... if the new locale is a member of this list. If an exact match ... isn't found, then the first supported locale with a matching `languageCode` is used. If that fails, then the first element of the `supportedLocales` list is used."
  - Chinese: "fully differentiating all variants of Chinese requires specifying the language code, script code, and country code". Example: `Locale.fromSubtags(languageCode: 'zh', scriptCode: 'Hant', countryCode: 'TW') // 'zh_Hant_TW'`.
  - iOS: "you need to add the supported languages in the Xcode project. This ensures your entry in the App Store correctly displays the supported languages." (Xcode > Runner > Info > Localizations.) The page does not mention CFBundleLocalizations.
  - The docs show `Localizations.override` for a subtree. An app-wide override uses `MaterialApp.locale` (I: standard API; not shown on this page).
  - The page does not cover per-app language settings.
- V https://docs.flutter.dev/release/breaking-changes/flutter-generate-i10n-source
  - "The `flutter` tool will no longer generate a synthetic `package:flutter_gen`". The change landed in 3.28.0-0.0.pre, reached stable in 3.32.0, and "In the next stable release after this change lands, `package:flutter_gen` support will be removed." Migration: `synthetic-package: false` plus `arb-dir`/`output-dir`, and import from source (`import 'i18n/app_localizations.dart'`).
  - "the property `generate: true` is now required".
- V https://developer.android.com/guide/topics/resources/app-languages
  - Android 13 per-app languages. With AGP 8.1+, `androidResources { generateLocaleConfig = true }` generates the locale config automatically ("off by default"; "the build fails if you do manually create a `LocaleConfig` file"). Manual setup: `res/xml/locale_config.xml` plus `android:localeConfig`.
  - Backward compatibility via `AppCompatDelegate.setApplicationLocales` (AppCompat 1.6+), which needs an AppCompatActivity. On Android 13+ the API is `LocaleManager.applicationLocales`.
  - Tag format: language, then optional ISO 15924 script, then optional region, e.g. `zh-Hant-MO`. The resourceConfigurations form is `b+zh+Hant+MO`.
  - I: For a Flutter app, generateLocaleConfig derives the list from `res/values-*` folders. Flutter strings live in Dart, not Android resources, so you probably need a manual locale_config.xml, or placeholder values-xx dirs, or a plugin. Not verified. If the user picks a language in system settings, Flutter sees it as the platform locale (I).
- I (search only, Apple primary docs not reached): iOS 13+ shows a per-app "Preferred Language" in Settings > app when the bundle declares more than one localization (Xcode project localizations / `CFBundleLocalizations` in Info.plist). Flutter plugins tell you to add CFBundleLocalizations. An in-app picker can deep-link to `UIApplication.openSettingsURLString`.

## 4. MessageFormat / CLDR plurals

- V https://unicode.org/reports/tr35/tr35-messageFormat.html: labelled "Version 48.2", "This is a stable document". Stability policy: "Updates to this specification will not make any valid message become not valid."
- I (search summary of the Unicode blog https://blog.unicode.org/2025/03/): CLDR 47 (2025-03-13) moved MF2 from Final Candidate to Stable. ICU 77 had only tech-preview MF2 implementations.
- V https://github.com/tc39/proposal-intl-messageformat: "Stage: 1" (last listed update Feb 2024).
- I: next-intl, Flutter gen-l10n, and go-i18n all use the ICU MF1 syntax (plural/select). MF2 is not a practical choice yet.
- V https://www.unicode.org/cldr/charts/latest/supplemental/language_plural_rules.html
  - zh cardinal: only `other`. zh ordinal: only `other`.
  - en cardinal: `one` (1) and `other`. en ordinal: one/two/few/other (1st, 2nd, 3rd...).
- T (Node v26): `new Intl.PluralRules('en').resolvedOptions().pluralCategories` gives `one,other`, and for zh-TW it gives `other`.
- I: Implication: zh-TW source strings never need plural syntax, but English does ("1 item" vs "2 items"). Source strings with counts must be written in plural form from the start, or translators cannot add the forms. Gen-l10n and ICU require `other`.

## 5. Go

- V https://pkg.go.dev/golang.org/x/text/language
  - `func ParseAcceptLanguage(s string) (tag []Tag, q []float32, err error)`
  - NewMatcher: "The first language is used as fallback." "it is important to use the same language tag for the selection of any locale-specific services." "Match and MatchString both return the index of the matched supported tag". The result can carry a `-u-rg` extension (e.g. "en-GB-u-rg-auzzzz"), so use the index, not the tag string.
  - "the parent of "zh-TW" is "zh-Hant"".
- T (x/text v0.43.0, supported = [zh-TW, en]):
  - zh-Hant-TW -> zh-TW Exact; zh-Hant -> zh-TW Exact; zh-HK -> zh-TW High; zh-CN -> zh-TW Low; zh -> zh-TW Low
  - `en-US,en;q=0.9` -> en (idx 1) Exact; `ja,en;q=0.5` -> en Exact; `fr` -> en (idx 1) confidence No (note: not the first entry, despite the "fallback" wording; check the confidence value and apply your own default when it is No)
  - Chrome-style `zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7` -> zh-TW Exact
- V https://github.com/nicksnyder/go-i18n
  - "Supports pluralized strings for all 200+ languages in the Unicode Common Locale Data Repository (CLDR)." "Supports message files of any format (e.g. JSON, TOML, YAML)."
  - "Use `goi18n extract` to extract all i18n.Message struct literals in Go source files to a message file for translation". `goi18n merge active.en.toml translate.es.toml` produces the files still to translate.
  - `i18n.NewLocalizer(bundle, lang, accept)` with `r.Header.Get("Accept-Language")`.
  - Activity: 3.5k stars; the page states no release cadence.
- V https://google.aip.dev/193 (LocalizedMessage): "both the locale and message fields must be populated." "Where the locale is not specified by the user, it should be en-US". "LocalizedMessage may use a locale specified in the request". The locale "following IETF bcp47". The page does not mention Accept-Language.
- V https://google.aip.dev/143: "Fields representing spoken languages must use IETF BCP-47 language codes ... and the field must be called language_code." Region uses CLDR region codes as `region_code` (not `country_code`). Time zones use IANA as `time_zone`.
- V https://developers.google.com/maps/documentation/places/web-service/place-details: languageCode = "The language in which to return results." "If `languageCode` is not supplied, the API defaults to `en`." regionCode is "used to format the response" and has "no default value".
- I: Connect and gRPC define no locale convention. Connect-go exposes ordinary HTTP headers, so `Accept-Language` reaches unary and streaming handlers (gRPC metadata in gRPC mode). Google-style APIs make the language an explicit `language_code` request field when the response content depends on it (Places, Merchant). AIP-193 errors pick the locale "specified in the request". A common hybrid: an explicit field for content that gets persisted or sent later (for example, the stored notification language on the user profile), and a header for transient UI messages.

## 6. TMS (brief)

- **Tolgee**: V https://tolgee.io/pricing: Free cloud plan "30 000 words", "3 seats", "€0". There is a "Free for open-source" plan and an "EE licence". The paid Translate plan is "€58/mo, billed annually". I (search, https://tolgee.io/apps-integrations/flutter): the vendor says it supports ARB; the in-context Flutter SDK is "Alpha". It is open source and self-hostable (core Apache-2.0 with EE parts, I). Its in-context editing is mainly for web (React/Next). It supports ICU, and it has a CLI and GitHub Action for sync (I).
- **Weblate**: V https://weblate.org/en/hosting/: Libre plan free "only for public projects"; paid cloud "From €39 A MONTH"; "Licensed GNU GPLv3+". V https://docs.weblate.org/en/latest/formats.html lists "ARB File", "i18next JSON files", and "JSON files". I (docs search): an ICU MessageFormat check exists via the `icu-message-format` flag (since 4.9). Weblate is git-native: it commits to the repo and can open PRs (I).
- **Crowdin**: V https://support.crowdin.com/github-integration/: it uses a service branch (`l10n_` prefix) and "Crowdin sends the pull request with translations to your version control system." I: free for OSS with a license request (public repo, OSI license). The general free plan is unclear (a third party says 60K hosted words and 1 private project). Supports ARB and ICU JSON, screenshots, and MT (I).
- **Lokalise**: V https://lokalise.com/pricing: there is no free plan, "free 14-day trial"; the cheapest plan is "Explorer at $149/mo". Has GitHub sync, screenshots, and ARB support (I).
- **Phrase Strings**: V https://phrase.com/pricing/: the cheapest Strings plan is "Software UI/UX" at 525/month billed annually (15 seats, 1M words); there is no free plan, 14-day trial. That is enterprise pricing.
- I: For a private, single-maintainer repo with about 2 locales, the realistic options are: (a) no TMS (an AI or human translator edits JSON/ARB in PRs, with CI gates), (b) self-hosted Tolgee or Weblate, (c) Tolgee or Crowdin on a free tier.

## 7. Pseudo-localization and missing-key checks

- V https://learn.microsoft.com/en-us/globalization/methodology/pseudolocalization
  - "When the source language is English, a good heuristic is to lengthen the text by 40%. In practice ... 200% or even 400% longer". "Strings that are one or two words commonly grow proportionally longer". It also suggests adding delimiters (to reveal truncation and concatenation) and unique IDs. Pseudo locales: qps-ploc, qps-plocm, qps-ploca. ICU uses en-XA (accents) and en-XB (bidi).
- V https://developer.android.com/guide/topics/resources/pseudolocales: en-XA "adds Latin accents ..., expands the original text ..., and brackets each message unit". Enabled with `pseudoLocalesEnabled` on the debug build type. I: this applies only to Android resources, not Flutter Dart strings.
- I (search, no primary figure): zh-to-English is the reverse of the usual case. English versions of short zh labels are often 2–3x longer in characters, and still noticeably wider when rendered, although CJK glyphs are wider than Latin ones. There is no authoritative zh-to-en percentage. A practical approach: pseudo-locale the zh source with a factor of about 2x or more for short strings, or just test with real English.
- V https://next-intl.dev/docs/workflows/typescript: "Messages can be strictly typed to ensure you're using valid keys". Typing ICU arguments needs `allowArbitraryExtensions` plus `createMessagesDeclaration` (it generates a `.d.json.ts` file). AppConfig augmentation covers Messages and Locale.
- I (search; next-intl workflows/messages page): next-intl docs point to `@lingual/i18n-check` (https://github.com/lingualdev/i18n-check) for missing keys and inconsistent ICU arguments (`i18n-check --source en --locales messages`). `--unused` is opt-in. V: next-intl's linter supports both `useTranslations` and `useExtracted`, and "Both `.json` and `.po` catalogs are supported out of the box."
- I: Flutter has `untranslated-messages-file` (see section 3). In CI, fail if that JSON is non-empty for locales that are meant to be complete. Leave it non-blocking while English is only partly filled.

## 8. Marketplace locale UX

- V https://help.shopify.com/en/manual/international/languages: "Each published language gets its own URLs"; "When translations don't exist, your store displays content in your default language."
- V https://help.shopify.com/en/manual/international/languages/notifications: "After you publish a language, your checkout and email notifications are displayed in the customer's language."
- I (search: shopify.dev markets, community): languages appear as subfolders (e.g. /fr, /en-ca). Customer records carry a locale used for notifications (Admin API `Customer.locale`; I, not verified this session).
- I (Etsy help search https://help.etsy.com/hc/articles/360000343048, page 403 to fetch): listings are "automatically translated"; "any manual translations you provide will override automatic translations". Region, language, and currency settings are per account, or per browser cookie when logged out. Sellers can disable auto-translation.
- I (Airbnb newsroom search https://news.airbnb.com/?p=338456): Translation Engine "removed the need for click-to-translate buttons" and translates descriptions and reviews automatically (2022). The original text is kept. I did not verify the "show original" UI.
- I (Amazon help search): language preference is per store and account, from the flag menu. "some website features or communications may not be available in your preferred language."
- Shopee TW, momo, PChome: NOT VERIFIED. Shopee SG and TH help pages describe Me > Settings > Language (the app restarts). I could not confirm whether Shopee TW offers English. I: momo and PChome appear to be zh-TW only (no English UI known). Treat this as unverified.

## 9. UGC translation norms

- V https://docs.cloud.google.com/translate/attribution: "you must make it clear to users that they are viewing automatic translations from Google Translate"; the "powered by Google Translate" graphic "must always be displayed adjacent any translation results"; mark indexable machine translations with the documented HTML markup.
- V https://www.airbnb.com/help/article/2908 (Airbnb ToS): "Some translations on the Airbnb Platform are powered by Google." "Airbnb does not guarantee the accuracy or quality of generated Content or translations".
- I: Shopify Translate & Adapt auto-translates into 2 languages for free (community and third-party sources). The industry norm is: keep the original, label machine output, provide a "show original" toggle, let manual translations override MT, and never let MT overwrite the stored original. Seller-entered fields (product names, descriptions) in a zh-only market would stay in zh unless MT is added.

## 10. Legal language of record

- V https://www.6laws.net/6law/law/商品標示法.htm: "商品標示所用文字，應以中文為主，得輔以英文或其他外文。" (The article number is shown as §11 in one version and §7 in another on that page. Check law.moj.gov.tw before citing.) This rule is about product labels, not contracts. I: the Food Safety Act likewise requires Chinese food labels.
- I (search): the Consumer Protection Act (消保法) §11–17 cover standard contracts (定型化契約): review period, disclosure, unfair terms, 應記載/不得記載事項. My search found NO explicit statutory requirement that a standard contract be in Chinese. Not confirmed either way; ask a lawyer before relying on this.
- V (search snippet, laws.gov.taipei): Taipei City laws state "in case of any discrepancy between the English version and the Chinese version, the Chinese version shall prevail."
- I (search): many international platforms make English prevail (Yelp, Shopify; conductatlas). I could not find LINE TW, Shopee TW, or momo clauses. I did not find a language-precedence clause in the visible text of the Airbnb or Uber US terms (pages truncated).
- I: The usual pattern for a zh-primary Taiwan business: the zh-TW terms are authoritative, and the English is labelled "for reference only; the Chinese version prevails", with Taiwan governing law.

## 11. Time zone and currency formatting

- T (Node v26.10.0 ICU):
  - `Intl.NumberFormat('en', {style:'currency', currency:'TWD'}).format(1234.5)` gives `NT$1,234.50` (same for en-US and en-TW). zh-TW gives `$1,234.50`.
  - With `maximumFractionDigits:0`: en gives `NT$1,234`, zh-TW gives `$1,234`.
  - DateTimeFormat medium/short in `Asia/Taipei`: en `Oct 17, 2026, 11:30 AM`, zh-TW `2026年10月17日 上午11:30`.
- I: CLDR TWD defaults to 2 fraction digits (cashDigits 0), so integer prices need explicit `maximumFractionDigits: 0`. Do not rely on the locale default.
- V https://www.w3.org/TR/timezone/: "Allow the user to choose a time zone and keep it associated with the user's session or profile if possible." For zoned values, "The time zone should be used when formatting the value for display." "When in doubt, use UTC for serializing, storing, and exchanging". It treats time zone and locale as separate factors.
- I: For a single-market marketplace (delivery slots, cut-off times), show times in Asia/Taipei regardless of UI language or device time zone, and pass `timeZone: 'Asia/Taipei'` explicitly. next-intl otherwise uses the server runtime time zone (V, section 1), and Workers run in UTC (I).

## 12. BCP 47 tags

- V https://www.w3.org/International/questions/qa-choosing-language-tags: "the golden rule is to keep your language tag as short as possible"; "Script subtags should only be used ... when the script adds some useful distinguishing information"; region "only ... if it contributes information".
- V (Google hreflang): `zh-Hant` is an accepted script form, and so is the language-region form (zh-TW).
- V (Flutter): full Chinese differentiation is `zh_Hant_TW` (language + script + country).
- V (Go): the parent of zh-TW is zh-Hant. T: the Go matcher maps zh-Hant-TW and zh-Hant to zh-TW (Exact).
- T: `new Intl.Locale('zh-TW').maximize()` gives `zh-Hant-TW`.
- I (Apple forums, old): a `zh-Hant.lproj` folder alone historically did not catch zh-TW devices in some versions. Modern iOS reports `zh-Hant-TW` as the preferred language, and Xcode offers "Chinese, Traditional (zh-Hant)" and "Chinese (Taiwan) zh-TW"? (unverified). Browsers in Taiwan send `zh-TW,zh;q=0.9,...` (Chrome) or `zh-TW` (Safari) (I).
- I: Recommendation basis: use `zh-TW` as the web URL and message-catalog id (short, matches Accept-Language and hreflang) and `en` (not en-US). In Flutter, declare `Locale.fromSubtags('zh','Hant','TW')` or at least `zh_TW`, plus `en`. Name ARB files `app_zh_TW.arb` or `app_zh_Hant_TW.arb`, and test resolution on an iOS device set to Traditional Chinese (Taiwan) to confirm.
