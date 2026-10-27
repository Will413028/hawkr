# Enum labels & "why unavailable" text — web research (2026-10-17)

V = VERIFIED (read on page / source file). I = INFERENCE.

## 1. Google AIP-126 / AIP-193 / error_details.proto
- AIP-126 https://google.aip.dev/126 (V): naming rules only ("All enum values must use UPPER_SNAKE_CASE."; "_UNSPECIFIED" zero value; "Enums should document whether the enum is frozen or they expect to add values in the future."). No guidance at all on display names / localization of enum values (V, absence on page).
- AIP-193 https://google.aip.dev/193 (V):
  - "The `message` field is a developer-facing, human-readable "debug message" which **should** be in English."
  - "Localized messages are expressed using a `LocalizedMessage` within the `details` field."
  - "The `LocalizedMessage` payload **should** contain the complete resolution to the error."
  - "Where the locale is not specified by the user, it **should** be `en-US` (US English)."
  - "Regardless of how the locale for the message was determined, both the `locale` and `message` fields **must** be populated."
  - "Any dynamic aspects of the message must be included as metadata within the `ErrorInfo` that appears in details." / "This practice is critical so that machine actors do not need to parse error messages to extract information."
  - "`Status.message` is implicitly part of the API contract, so **must not** be updated" (once clients depend on it).
  - Who localizes: implicitly the server (LocalizedMessage is server-populated). (I)
- error_details.proto https://github.com/googleapis/googleapis/blob/master/google/rpc/error_details.proto (V):
  - LocalizedMessage: "Provides a localized error message that is safe to return to the user which can be attached to an RPC error."
  - ErrorInfo.reason: "This is a constant value that identifies the proximate cause of the error." UPPER_SNAKE_CASE, ≤63 chars.
- Net: Google design = machine code (reason+metadata) always; developer message in English; user-facing text optional, server-localized, in LocalizedMessage. Both coexist. (I)

## 2. Google Merchant API issueresolution (proto source)
Source: https://github.com/googleapis/googleapis/blob/master/google/shopping/merchant/issueresolution/v1/issueresolution.proto (V)
- RenderProductIssues: "This content and actions are meant to be rendered and shown in third-party applications."
- RenderAccountIssuesRequest.language_code: "The IETF BCP-47 language code used to localize issue resolution content."
- prerendered_content: "Details of the issue as a pre-rendered HTML." / "Always sanitize the HTML before embedding it directly to your application."
- Action.is_available: "Controlling whether the button is active or disabled."; Action.button_label: "Label of the action button."
- Action.reasons: "List of reasons why the action is not available." "If there are more reasons, all of them should be displayed, for example in a pop-up dialog."
- Action.Reason has ONLY text + optional action, NO enum code (V): message "Messages summarizing the reason, why the action is not available."; detail "Detailed explanation of the reason. Should be displayed as a hint if present."; action "This action will always be available. Should be rendered as a link or button next to the summarizing message."
- Context (I): this API exists for third-party apps that cannot know Google's policy catalog, many locales, policy text changes often → server-rendered text is the right call there. That condition (client doesn't own the domain vocabulary) differs from a first-party 4-client monorepo.
- Reference HTML pages under developers.google.com/merchant/api/reference/rpc/... returned 404 to fetch; used proto source.

## 3. Stripe
- https://docs.stripe.com/api/errors (V): `message`: "A human-readable message providing more details about the error. For card errors, these messages can be shown to your users." `code`: "For some errors that could be handled programmatically, a short string indicating the error code reported." `decline_code`: "...a short string indicating the card issuer's reason for the decline if they provide one." `param`: "...you can use this to display a message near the correct form field."
- https://docs.stripe.com/declines/codes (V): per-code "Next steps" are written for the integrator to craft customer text, and some codes must be masked: `fraudulent` "Don't report more detailed information to your customer. Instead, present it in the same manner as `generic_decline`." `lost_card` "The specific reason for the decline shouldn't be reported to the customer."
- Terminal SDK localization (from search summary of stripe.dev Terminal docs; not page-read): API errors localized by Accept-Language, fallback en-US; advises not to parse message text, use codes. (I — search summary only)
- Net: Stripe = code for logic + optionally user-displayable message for a narrow error class; mapping/masking policy per code is left to the client. (I)

## 4. GitHub / Shopify
- GitHub REST https://docs.github.com/en/rest/using-the-rest-api/troubleshooting-the-rest-api (V): "The response body will include an `errors` property, which includes a `code` property to help you diagnose the problem." codes missing/missing_field/invalid/already_exists/unprocessable; `custom`: "Refer to the `message` property to diagnose the error." Developer-oriented, English. (I on audience)
- GitHub GraphQL public schema https://docs.github.com/public/fpt/schema.docs.graphql (V, downloaded and grepped):
  - `viewerCannotUpdateReasons: [CommentCannotUpdateReason!]!` — "Reasons why the current viewer can not update this comment."
  - enum CommentCannotUpdateReason: "The possible errors that will prevent a user from updating a comment." values ARCHIVED, DENIED, INSUFFICIENT_ACCESS, LOCKED, LOGIN_REQUIRED, MAINTENANCE, VERIFIED_EMAIL_REQUIRED (English descriptions only in schema docs, not in response).
  - => Direct precedent for "server-derived unavailable-reason enum, codes only, client renders text". (V for shape; I for client rendering)
- Shopify Admin GraphQL UserError https://shopify.dev/docs/api/admin-graphql/latest/objects/UserError (V): "An error in the input of a mutation."; field "The path to the input field that caused the error."; message "The error message." Newer typed *UserError objects add a `code` enum (e.g. ShopPolicyUserError; bulk userErrors got optional code in 2025-01 — changelog https://shopify.dev/changelog/bulk-query-operation-usererrors-now-includes-code-field, search summary). Whether message is localized per merchant locale: not verified. (I)

## 5. Client i18n practice
- Flutter gen-l10n https://docs.flutter.dev/ui/accessibility-and-internationalization/internationalization (V): "In the `arb` files, the key of each entry is used as the method name of the getter, while the value of that entry contains the localized message." select: "{selectPlaceholder, select, case{message} ... other{messageOther}}"; "comparison between the parameter and the actual value is case-sensitive" (falls to other).
- FormatJS ICU https://formatjs.github.io/docs/core-concepts/icu-syntax/ (V): "The `{key, select, matches}` is used to choose output by matching a value to one of many choices." "`other` is required as per `icu4j` implementation."
- next-intl https://next-intl.dev/docs/usage/messages (V): select "other case is required"; "Values are required to be alphanumeric and can contain underscores." Messages are JSON per locale.
- => Both Flutter ARB and next-intl JSON use ICU MessageFormat 1 `select`, so one enum→label catalog can be authored once as ICU and emitted to both (I).
- Lokalise https://docs.lokalise.com/en/articles/1400489-keys-and-platforms (V): "When you add a new key, it must be assigned to one or more platforms"; "One of the unique features of Lokalise is the ability to use similar keys across different platforms"; "this key cannot have different translations for the same language on iOS and Web." Lokalise exports Flutter .arb (https://lokalise.com/blog/flutter-i18n/: "choose **Flutter (.arb)** as the file format", V).
- Crowdin: bundles export one source to several formats (blog https://crowdin.com/blog/translate-app-content-with-bundles; search summary, I).
- MessageFormat 2: CLDR 47 https://cldr.unicode.org/index/downloads/cldr-47 (V): "MessageFormat has advanced from Final Candidate to Stable." but "Tech Preview implementations are available in C++, Java, and JavaScript." messageformat.unicode.org: "built directly into ICU4C, ICU4J, and soon, ECMA-402" (V). No Dart/Flutter MF2 support found (I). => today use ICU MF1 select.

## 6. Server-driven UI
- Airbnb GP (InfoQ summary https://www.infoq.com/news/2021/07/airbnb-server-driven-ui/, V): "enables our backend to control the data and how that data is displayed across all clients at the same time"; "controlled by a single backend response across our web, iOS, and Android apps". Primary Medium post 403 on fetch. No trade-off text found.
- Trade-offs (I, general knowledge, not sourced): server text = change without app release, single source, but requires locale negotiation on every endpoint, cannot render when offline/cached-stale, couples API to presentation, harder to test/screenshot, a11y semantics (labels vs hints) must be modeled; client text = works offline, i18n tooling (TMS, plural/gender), but needs release per wording change and exhaustive mapping with `other` fallback for unknown future enum values.

## 7. Display names in proto options
- google/api/field_info.proto (V): only `format` (UUID4, IPV4...) and `referenced_types`; no display names.
- Standard EnumValueOptions: deprecated, features, debug_redact; no display name (search summary, I).
- Searches found no well-known public API (googleapis, GitHub, Stripe) that puts user-facing display labels in proto enum options (I — negative from searches, not exhaustive). Google pattern instead: enum docs in comments; user-facing text either server-rendered (Merchant) or client-owned.
- Custom option pattern technically supported (e.g. json_enumvalue_options.proto in protobuf source with `[(pb.enumvalue.json).string = "..."]`), readable by protoc-gen-es / Dart protoc_plugin descriptors to generate catalogs (I).
