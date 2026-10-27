# Step 8 client inventory — seller / vendor portal / marketplace / admin REST call sites (HEAD 174adc432, read-only)

Sections: 0 summary and cross-client findings; 1 website; 2 admin; 3 buyer-app; 4 seller-app. Sections 2-4 were produced by per-client passes and are appended verbatim after section 1. Every section lists the rg commands used.

## 0. Summary

### 0.1 Counts vs old inventory (`backend/docs/plans/2026-09-29-a3-buyer-seller/seller-interface-inventory.md` §5)

| client | old (paths / files) | now: distinct (method,path) pairs in scope / files with a real call site | note |
|---|---|---|---|
| website | 34 / 41 | 36 live + 3 dead / 13 (28 files consume `/me`, 17+ consume public seller entity) | old count included consumers, tests, comments. Delegated BFF (`/admin/sellers/{id}/storefront`) and vendor portal RFQ routes (rfqs/{id}, response_draft, response_submissions) are new vs the old route table |
| admin | 35 / 19 | 40 / 14 | sellers 8, certifications 2, accounts 1, membership plans 2, prospects 16, governance 7, violations 4. All via BFF `/api/bff` + `/api/v1/admin/...` |
| buyer-app | 11 / 17 | 14 / 15 (25 call sites; 9 callers of `GET /public/sellers/{id}`) | follow list/favorites/visits/address already Connect; only per-store merchant info and marketplace are REST |
| seller-app | 26 / 25 | 30 / 11 literal-path files (+~14 files that read `/me` fields) | 34 files in lib contain `/api/v1/` literals total (rest out of scope) |

None of the four clients has a seller / agent / discovery Connect client; `api/hawkr/` has only admin, buyer, common, content, identity protos.

### 0.2 Paths seen in clients that do NOT appear in `backend/api/openapi/openapi.yaml`
Check: `rg -n "^  /api/v1/<path>" backend/api/openapi/openapi.yaml`.
- website (delegated BFF to gateway admin, handled by delegation module, not seller): `GET|PATCH /admin/sellers/{id}/storefront`, `POST /admin/sellers/{id}/storefront/logo|cover`, `/admin/sellers/{id}/products...`, `/admin/sellers/{id}/posts...` (only `/admin/sellers/{store_id}/orders` and `/delegated-operation/preflight` exist in openapi).
- admin: none. buyer-app: none. seller-app: none (base constants `/app/media`, `/app/logistics`, `/app/sellers/me/procurement` take suffixes).
- openapi routes with NO client call site anywhere in the four clients (dead gateway routes, nothing to port): `POST /public/food_safety_reports`, `GET /public/sellers/{id}/violations/public`, `POST /app/violation_reports`, `GET /app/violation_reports/me`, `GET /app/sellers/me/store_visitors`, `GET /app/share_url` (0 REST hits in all four; website builds the share URL locally in `lib/seller/share-card.ts`), `/admin/sellers/{id}/membership` GET+POST, `GET /admin/certifications` and `/admin/certifications/{id}` (admin only calls `PATCH /admin/certifications/{id}` and `GET .../{id}/audit-logs`; it reads the certification object embedded in `GET /admin/sellers/{id}`), `/public/vendor_portal/rfqs/{id}/declines` and `.../seller_responses/{id}/withdrawals`, `GET /app/sellers/me/violations/{id}/appeal`, `/public/products/{id}/view` only in website, `/public/sellers/recommended` only in website.

### 0.3 Notable dead code
- website: `GET /app/sellers/me/violations/{id}/appeal` (none), declines/withdrawals vendor routes (none).
- seller-app: `refreshPayuniStatus` (`POST payuni/status_syncs`), `cancelCertification` (`DELETE /app/certifications/{id}`), `getCertificationApplication` have no callers; `PayuniReviewStatusScreen` is registered but has no entry navigation; `wallet_screen` reads `result['paymentStatus']` from the wrong level (always null).
- buyer-app: `getFansCount` (`follow_repository.dart:66`) and `getSellerProducts` (`seller_repository.dart:23`) have no callers; `seller_suspension_codes.dart` says "no caller" (stale: 9241 is used in cart/checkout).
- admin: `/admin/violations/[id]` page has no list page or sidebar entry (reachable only after create-violation redirect).

### 0.4 Fields the UI needs that the backend modules no longer have (decides what Connect messages must add, drop, or compose in `internal/httpapi/seller`)
Backend `seller.Store` has: StoreName, Description, ContactPhone, TownCode (8-digit), AddressDetail, Prize, WebsiteURL, SEOTitle, SEODescription, IsProcessedFood, FoodBusinessRegNo, OnboardingCompleted, StoreID, StoreCode, AccountID, Version, Status (ACTIVE / PENDING_DELETION / CLOSED), Suspension (NOT_SUSPENDED / SUSPENDED_BY_ADMIN / SUSPENDED_BY_GOVERNANCE), LogoKey, CoverKey, timestamps (`backend/internal/modules/seller/internal/model/model.go:84-105`).

| UI field | read by | backend status |
|---|---|---|
| rating | website (public seller, stats, recommended, marketplace, metadata), buyer-app (home header, follow page, PDP seller card), seller-app (page header, product preview, shop tab: public rating summary), admin (merchant list/detail) | removed from seller by design (`seller/CARRYOVER.md:43`: composed in `internal/httpapi/seller` step 8; ranking by discovery) |
| followers / subscriberCount | website (stats, recommended), buyer-app (`/stats`), seller-app (fans count), dashboard `subscriberCount` | not in seller; buyer module owns subscriber counts |
| city, district (names), address (composed), postalCode | website (/me, public, recommended, share card, onboarding, profile PUT/POST bodies), seller-app (session, wallet, shipping prefill, profile), buyer-app (follows, favorites, visits, PDP, category), admin | Store has only TownCode; requests send names today, must become TownCode (`platform/twaddr`); read side must derive county/district names (first 5 digits = city per CARRYOVER:25) and a composed `address`; postalCode only derivable via twaddr, and the old gateway `PublicSellerInfo` already returns it empty |
| certified / overallCertificationStatus / certificationStatus / paymentStatus / certificationDraftId / currentCertificationId | website (cert gate, invoices, shipping, wallet, onboarding), seller-app (gates product create, shipping settings, wallet, shop tab), buyer-app (certified badge on shop page), admin (list filters, detail) | not on Store; derive from KYC flow (`KYCApplication.Status`, `Flow`) + PayUni; backend `Exposable` rule is the only public "certified" notion (ExposableStore) |
| lifecycleStatus (`active|suspended|suspended_governance|closed`) + suspensionReasonCode | website (suspended notice, invoices hooks), seller-app (suspension notice, router redirect), admin (merchant detail, suspension reason CodeMeta `SellerSuspensionReason`) | replaced by Status x Suspension (+ SuspensionRecord.Reason); `suspended_governance` string no longer exists, closed/pending-deletion are new states the UIs do not understand |
| needsOnboarding | website, seller-app | inverse of `OnboardingCompleted` |
| sellerName, contactPerson, phone, location, email (read AND written by seller-app profile PUT with `status:1`), contactEmail (admin list/prospects) | seller-app, website, admin | no such fields on Store (email on identity, contactEmail on prospect/KYC `ContactEmail`) |
| membership plan / tier | not read by any client UI | backend deleted (CARRYOVER decision 7). BUT admin still has a live page `/admin/seller-membership-plans` (Sidebar:175) calling GET + PATCH `/admin/seller_membership_plans*` (planCode, displayName, tierRank, monthlyOrderCap, monthlyPriceTwd, isPurchasable): page must be deleted with the Connect move; `/admin/sellers/{id}/membership` has no call site |
| invite codes (seller-level) | not read by any client | n/a. Prospect invitations panel in admin uses `/admin/seller_prospects/{id}/invites` (agent module `EnsureInvite`) |
| coverUrl / logoUrl (display URLs) | all | Store holds keys only; URL composition belongs in the Connect response (`common/image.proto`) |
| version (`If-Match` / `expectedVersion`) | delegated BFF only | backend `UpdateProfile/UpdateLogo/UpdateCover` REQUIRE `ExpectedVersion`; seller-app PUT and website PUT /me send none today -> GET /me must return version and both clients must send it |
| SellerViolation.description | website violations page, seller-app | backend `Violation.Description` is INTERNAL; seller-facing text is `StatementToSeller` -> map to the field the UI calls description |
| KYC `stepData` JSON blob, TSID `id`, `result`/`comment` on reviews, snake `requiredFields` labels | website cert wizard, seller-app | backend KYC is typed (UUID id, `KYCApplication` columns, `KYCRejection{Reason, RequiredFields}`, `KYCReview{Action, Reason, RequiredFields}`); wizard persistence (`stepData` per step) has no equivalent -> design decision for Connect (typed draft fields + currentStep) |
| vendor preview `brand` | website vendor landing | `agent.InvitePreview` has only MaskedEmail, ExpiresAt; brand must be a constant/config in the response |
| vendor RFQ summary / response_draft / response_submissions / step_up | website vendor portal | NOT in `agent` module public API (agent has EnsureInvite, MarkInviteDelivered, ExchangeInvite, PreviewInvite, ValidateSession, VerifySession, OnProspect*, PurgeExpired only); `rg response_draft backend/internal --glob '!legacy'` -> no hits, so RFQ read/draft/submit are still legacy |
| admin sellers list / stats / PayUni observation / certification object / admin account delete | admin | no `AdminStores`/`ListStores`/stats in seller module (`rg -n "ListStores|AdminStores|SellerStats" backend/internal/modules/seller --glob '!*_test.go'` -> none); only Store(id), Suspensions, KYC admin, PayUni observations, prospects, governance exist. Account deletion lives in `accountdeletion` module |
| seller dashboard (sales, growth, pending shipment counts), CRM, store_visitors | website, seller-app | not in seller module (views.go has only `ProductViewStats` per store) |
| `GET /app/businessRegistration/{ban}` (MOF tax-id lookup) | website CertificationsClient, seller-app | only in legacy (`backend/api/proto/seller/seller.proto`, `internal/legacy/gen`); no monolith module |

### 0.5 Error-handling summary (all clients)
- **9240 / HTTP 409 seller suspended**: website `lib/api/client.ts:41,606,632` -> event -> `SellerCenterShell`; seller-app global Dio interceptor (`lib/core/networking/dio_provider.dart` ~201-287) -> `markSuspended()` + re-fetch /me -> router redirect to suspension notice; buyer-app: only 9241 (cart/checkout) — 9240 defined, unused; admin: none. Connect target (ADR api-error-model D10): `permission_denied` + ErrorInfo.reason=SELLER_SUSPENDED, so both interceptors change.
- **9233 SellerNotFound** (or bare 404 on /me) => needs onboarding: website `lib/seller/repo.ts:40-55`, seller-app `session_provider.dart:30,255`. Connect: NOT_FOUND on CurrentStore.
- **9266 version conflict**: website delegated edit only; seller-app product upload only; seller profile writes have no conflict handling today although backend requires ExpectedVersion.
- **HTTP 409**: website appeal-already-submitted, vendor draft CAS; seller-app KYC submit ("請先發布一則動態"), 428 on certification draft create; admin prospects reconciliation/media.
- **Open orders on close store**: no website UI; seller-app `DELETE /app/sellers/me` documents 9205/9204 only in comments, shows generic `messageZh`; admin has no close flow other than delete account (no special branch). Backend: `CloseCurrent` / `closing` module with orders port (`closing_orders_integration_test.go`) will need a dedicated error reason.
- admin: no 9240/9265/9266/9272 branches for sellers/KYC/suspension/delete; shows backend message + "(code N)". Delegation 92xx table lives in `admin/src/features/delegated-operations/authorization.ts:651-675`.

### 0.6 Surprises
- Website seller self-service PUT/POST send city/district names; backend takes TownCode. Same in seller-app.
- seller-app writes `email` and `status:1` on profile PUT and echoes the whole /me map back.
- admin violation report detail has no GET by id: it pulls `limit=200` and filters client-side (older reports vanish); admin deletes accounts and loads merchant products/orders by accountId, not storeId; the `monthly` tab on merchant list sends no filter.
- buyer-app does N+1 `GET /public/sellers/{id}` (tracking, favorites, history, post feed, category page, comment avatars) just to show name/logo/rating/city: a batch Connect RPC is the natural replacement; its `getMerchantInfo` swallows all errors into null (shop page shows "not found" on any failure).
- Vendor portal RFQ read/draft/submit are still outside the monolith; only invites/session exist in `agent`.
- `GET /app/sellers/me` in the website also drives session bootstrap (`lib/api/auth.ts:72`) and middleware canonical redirect uses `GET /public/sellers/{id}` just for `storeCode`.

---

## 1. website/ (Next.js) — HEAD 174adc432

Method (rg commands):
- `rg -n "[\"'\`]/(app|public|admin)/" website/{lib,features,app,components} -g '*.ts' -g '*.tsx' -g '!**/*test*'` (explicit-path call sites)
- `rg -n "apiClient\.public\.\w+" website -g '*.ts*'` (public client uses base-relative paths such as `/sellers/${id}`; base = `/api/v1/public`)
- per-family greps: `vendor_portal`, `sellers/me`, `/view`, `share_url`, `store_visitors`, `violation_reports`, `food_safety`, `seller_prospects`, `admin/(sellers|certifications|accounts)`
- field-read greps: `rg -o "\b(self|seller|me|serverSelf|profile)\??\.(rating|city|...)" features/seller-center components lib/hooks lib/seller`

All calls go through `lib/api/client.ts` (`authedFetch` / `apiClient.public`) -> first-party proxy `app/api/v1/[...path]/route.ts` -> gateway. There is no Connect client for seller (`website/lib/gen/hawkr/` has only buyer, common, content, identity; root `api/hawkr/` has no seller proto).

### 1.1 Call sites (in scope)

| # | file:line | method + path | feature / UI | response fields the UI reads (-> Connect fields) | reachable? | errors |
|---|---|---|---|---|---|---|
| W1 | lib/seller/repo.ts:231 (`fetchSellerSelf`; via `useSellerSelf`, `lib/api/auth.ts:72`, `components/shared/Link.tsx:235/264` prefetch) | GET /app/sellers/me | seller-center shell, navbar, profile, dashboard, cert gate, share card, wallet, shipping | Normalised (repo.ts:130-172): storeId, storeCode, storeName, sellerName, logoUrl, coverUrl, city, district, address, addressDetail, postalCode, contactPhone, prize, description, rating, status, needsOnboarding, overallCertificationStatus, isProcessedFood, foodBusinessRegNo, certificationDraftId, currentCertificationId, lifecycleStatus, suspensionReasonCode, websiteUrl. **Actually read by UI**: storeName, sellerName, storeCode/storeId (useSellerIdentity:25), logoUrl, coverUrl, city, district, contactPhone, addressDetail, description, prize, websiteUrl, foodBusinessRegNo, isProcessedFood, needsOnboarding, overallCertificationStatus, certificationDraftId, currentCertificationId, suspensionReasonCode, lifecycleStatus (only `isSellerSuspendedLifecycle`). **Normalised but not read**: rating, status(number), postalCode, coverKey; `address` only as parse fallback (CertificationsClient:246) | yes | 404 / code 404 / 9233 => `isSellerProfileMissing` (repo.ts:43) -> onboarding; lifecycle `suspended`/`suspended_governance` => suspended notice |
| W2 | lib/seller/repo.ts:231,268-277 | PUT /app/sellers/me | SellerProfileClient:267; CertificationsClient:803 (profile write before cert submit) | request (always full merge, `mergeSellerProfileUpdate`:323): storeName, description, contactPhone, **city, district (names)**, addressDetail, prize, websiteUrl. Response unused | yes | - |
| W3 | lib/seller/repo.ts:356 | PUT /app/sellers/me/food-business-registration | `features/seller-center/components/FoodRegSupplement.tsx:77` (wallet, dashboard) | req isProcessedFood, foodBusinessRegNo; resp unused | yes | - |
| W4 | lib/seller/repo.ts:231 (`finalizeSellerImage`) | PUT /app/sellers/me/logo, PUT /app/sellers/me/cover | `features/seller/components/SellerProfileBranding.tsx:203/207` | req stagedToken; resp `logoKey` / `coverKey` only | yes | - |
| W5 | lib/seller/repo.ts:381 | PATCH /app/sellers/me/onboarding | `SellerOnboardingClient.tsx:217` | none | yes | - |
| W6 | lib/seller/repo.ts:392 | PUT /app/buyers/me/seller_binding | `SellerOnboardingClient.tsx:204` (adds seller role before apply) | none | yes | buyer/identity concern; listed because it precedes apply |
| W7 | lib/seller/repo.ts:410 | POST /app/sellers (apply) | `SellerOnboardingClient.tsx:214` | req storeName, description, contactPhone, **city, district (names)**, addressDetail; resp unused (re-reads GET /me, `lib/seller/onboarding.ts:56,76,89` for idempotent re-apply) | yes | - |
| W8 | lib/seller/repo.ts:514 | GET /app/sellers/me/payuni/status (`throwOnHttpError:false`) | polled by `usePayuniStatus`; dashboard task card, WalletClient, CertificationsClient:447, `Link.tsx:240` | UI reads: stage (derived from `stage`/`overallStatus`/`payuniStatus`/`payuniMerStatus`), rejectedReason / reason / errorDisplayMessage (lib/seller/payuni-reason.ts), approvedAt, lastSyncedAt, freshness, allowedActions; MerchantProfileEditCard reads the 7 contact fields + merchantProfileVersion (contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel). Typed but not read: merId, checkedAt, accountId, errorCode, errorRemedy, merStatus, memberStatus, creditStatus, applepay/googlepay/atmStatus, registeredAt | yes | - |
| W9 | lib/seller/repo.ts:520 | POST /app/sellers/me/payuni/status_syncs | only `WalletClient.tsx:115` `onSync={syncFromProvider}` | same PayuniStatus | yes | - |
| W10 | lib/seller/repo.ts:553 | PATCH /app/sellers/me/payuni/merchant_profile | `MerchantProfileEditCard.tsx:113` | req any of the 7 contact fields; headers `Idempotency-Key`, `If-Match: merchantProfileVersion`; resp message, merchantProfileVersion | yes | `classifyMerchantProfileError` (lib/seller/payuni-merchant-profile.ts) |
| W11 | lib/seller/repo.ts:1386 | GET /app/sellers/me/dashboard | `DashboardPage.tsx:1368 buildStats`; `SellerCenterShell`, `SellerOrdersPage:482`, WalletClient, `Link.tsx:248` | read: salesAmount, salesAmountGrowth, pageViewCount, pageViewCountGrowth, subscriberCount, pendingShipmentCount, aftersalesCount, inProgressCount, completedCount. Typed but unread: orderCount(+Growth), customerCount(+Growth), subscriberCountGrowth | yes | skipped while suspended |
| W12 | lib/seller/certification.ts:300 | POST /app/certifications | CertificationsClient:556 | req `{sellerType}`; resp (`pickDraft`): id, sellerType, currentStep, stepData (JSON object), status, createdAt, updatedAt, idCardFrontUrl, idCardBackUrl, bankBookUrl, documentUrls, *AssetId(s) | yes | - |
| W13 | certification.ts:313 | GET /app/certifications/{id} | CertificationsClient:395 (resume) | same draft | yes | - |
| W14 | certification.ts:391 | PUT /app/certifications/{id}/progress | CertificationsClient:501,817 | req `{currentStep, stepData: JSON string}` with snake `*_staged_token`; resp draft | yes | gateway 400 on URL values in stepData |
| W15 | certification.ts:441 | PATCH /app/certifications/{id} (submit) | CertificationsClient:863 | req typed body (realName, idCardNo, *AssetId, birthDate, idCardIssue*, englishName, bankCode/Name/Account, companyType/Name, taxId, companyRegisterDate, companyCity/District/AddressDetail, companyEmail/Phone, productType, isProcessedFood, foodBusinessRegNo); resp draft | yes | - |
| W16 | certification.ts:453 | DELETE /app/certifications/{id} (optional `reason` body) | CertificationsClient:889 | none | yes | - |
| W17 | certification.ts:491 | GET /app/certifications/{id}/reviews | CertificationsClient:470 | id, applicationId, action, result, comment, rejectionReason, requiredFields[] (snake labels tax_id, id_front...), createdAt | yes | - |
| W18 | certification.ts:555 | GET /app/businessRegistration/{ban} (`throwOnEnvelopeError:false`) | CertificationsClient:649 (8-digit tax-id auto-fill) | businessNm/businessName/companyName, businessAddress, businessSetupDate, businessType, capitalAmount, businessStatus, mainBusiness | yes | non-zero code => null, UI falls back to manual entry |
| W19 | lib/seller/violations.ts:86 | GET /app/sellers/me/violations?limit&offset | `useSellerViolations:34` -> SellerViolationsPage | records[]: id, category, severity, description, status (enforced/revoked), appealed, appealStatus, appealDecisionReason, createdAtMs (string) | yes | - |
| W20 | violations.ts:108 | POST /app/sellers/me/violations/{id}/appeal | SellerAppealDialog (`useSellerViolations:79`) | req appealReason, appealEvidenceAssetIds[]; resp `{violation}` same item | yes | HTTP 409 = already appealed (`isAppealAlreadySubmittedError`) |
| W21 | - | GET /app/sellers/me/violations/{id}/appeal | **no call site** (doc comment/tests only) | - | NO | - |
| W22 | lib/vendor-portal/repo.ts:70 | GET /public/vendor_portal/invites/preview (`Authorization: Invite <secret>`) | `features/vendor-portal/VendorRfqLanding` (route app/vendor/rfq/page.tsx) | brand, maskedEmail, expiresAt | yes | - |
| W23 | repo.ts:84 | POST /public/vendor_portal/invite_exchanges (credentials include; Set-Cookie `hk_vendor_portal`) | VendorRfqLanding | rfqRecipientId | yes | - |
| W24 | repo.ts:95 | GET /public/vendor_portal/rfqs/{rfqRecipientId} | VendorRfqLanding | UI reads item, itemLabel, quantity, unit, packaging, deliveryCity, deliveryDate, responseDeadline, buyerNote, referencePriceRange, buyerRequestedRange, publicationVersion (qualityKnowledgeState typed, not rendered) | yes | 404 on session/id mismatch |
| W25 | repo.ts:140 / :165 | GET + PUT /public/vendor_portal/rfqs/{id}/response_draft | VendorRfqLanding | resp draftVersion, updatedAt, draft (17-field quote, lib/vendor-portal/draft.ts); PUT req expectedDraftVersion, draft | yes | GET 404 = no draft; PUT 409 = draft version conflict (reload, never auto-overwrite) |
| W26 | repo.ts:190 | POST /public/vendor_portal/step_up/google | `VendorGoogleStepUp.tsx` | req idToken; resp assurance | yes | always 403, deliberately unspecific |
| W27 | repo.ts:226 | POST /public/vendor_portal/rfqs/{id}/response_submissions (`Idempotency-Key`) | VendorRfqLanding | req expectedPublicationVersion, expectedSellerResponseVersion, quote; resp sellerResponseId, sellerResponseVersion, status | yes | - |
| W28 | - | POST .../rfqs/{id}/declines; .../seller_responses/{id}/withdrawals | **no website call site** (openapi only) | - | NO | - |
| W29 | lib/api/index.ts:377 (`fetchSeller`; `getCachedSeller` cached.ts:282) | GET /public/sellers/{id} | storefront layout/metadata/reviews/posts/messages pages, SellerProfileV2, card page, apple-icon, PDP SellerIntroAsync, ChatContextPanel:72, ThreadClient:231, StorefrontProductPreviewDrawer:130 | UI reads: name (<- storeName), logoUrl, coverKey/coverUrl, description, prize, websiteUrl, seoTitle, seoDescription, city, district, storeCode, storeId, **rating** (SellerProfileV2, SellerCardPage, SellerReviewsPage, lib/seo/metadata.ts) | yes | softFailNotFound -> null |
| W30 | lib/seller/canonical-redirect.ts:147 (middleware) | GET /public/sellers/{id} | `/sellers/{storeId}` -> `/sellers/{storeCode}` redirect | storeCode only | yes | 404 vs outage separated by message guard |
| W31 | lib/api/index.ts:554 (cached.ts:310) | GET /public/sellers/{id}/stats | SellerProfileV2, SellerCardPage, ProductSellerIntro | followers (only source of subscriber count), rating; typed unread: totalOrders, totalProducts, totalSales, storeId | yes | 404 -> null |
| W32 | index.ts:486 (cached.ts:297) | GET /public/sellers/{id}/reviews/stats | SellerProfileV2, SellerCardPage, SellerReviewsPage, PDP | avgRating (+counts) | yes | review module, not seller |
| W33 | index.ts:426,451 | GET /public/sellers/{id}/products, /reviews | storefront, reviews tab | product / review modules | yes | - |
| W34 | index.ts:527 (cached.ts:515; home bootstrap, tab-bootstrap warm) | GET /public/sellers/recommended?limit | home recommended-sellers strip (`features/home/homeSnapshot.ts`) | records[]: storeId, storeCode, storeName, logoUrl, coverUrl, description, city, district, **rating**, followers, certified (prize comes from /public/sellers/{id}) | yes | - |
| W35 | index.ts:171 (cached.ts:418; HomePage, SearchPage, CartPage, home bootstrap) | GET /public/marketplace?seller_limit&post_limit&product_limit | home / search / cart | recommendedSellers[] (= W34 shape), hotProducts[], latestPosts[] | yes | - |
| W36 | features/product/ProductViewTracker.tsx:14 | POST /public/products/{id}/view (beacon) | PDP, not mounted in `previewMode` | none | yes | errors swallowed |
| W37 | lib/seller/shipping.ts:165,204 | GET /app/address/validate, GET /app/address/suggestions | ShippingClient, SellerOnboardingClient, AddressAutocompleteField | validate: city, district, postalCode; suggestions: fullAddress, county, district, detail, lat, lng | yes | non-throwing |
| W38 | lib/chat/repo.ts:175 | POST /app/chat/messages | chat | chat module | yes | - |
| W39 | app/(delegated)/adm/api/storefront/[storeId]/route.ts:24,36 -> lib/seller-data/delegated-repos.ts:236,245 | GET + PATCH /admin/sellers/{id}/storefront | delegated (ADM acting for a store) seller-center profile | GET 12-field allow-list: storeId, name, description, prize, contactPhone, city, district, addressDetail, websiteUrl, logoKey, coverKey, version. PATCH req: name, description, prize, contactPhone, city, district, addressDetail, websiteUrl + expectedVersion; resp `{auditLogId, version, resource}` | yes | 9260, **9266**, 9264/9265/9269/9272/9286 (features/delegated-operations/load-failure.ts) |
| W40 | .../storefront/[storeId]/logo/route.ts:22, cover/route.ts:19 | POST /admin/sellers/{id}/storefront/logo, /cover | delegated branding | req stagedToken; resp resource.logoKey/coverKey + version | yes | 9266 |

Not found in website (`rg -n "share_url|store_visitors|violation_reports|food_safety|violations/public|seller_prospects|admin/certifications|admin/accounts|membership" website -g '*.ts*' -g '!tests'`): `GET /app/share_url`, `GET /app/sellers/me/store_visitors`, `POST /app/violation_reports`, `GET /app/violation_reports/me`, `POST /public/food_safety_reports`, `GET /public/sellers/{id}/violations/public`, `GET /public/sellers/{id}/posts` (website uses `/public/posts`), `DELETE /app/sellers/me` (no close-store UI in website), any /admin/* seller-management path other than the delegated storefront/products/posts/orders BFF.

Other `/app/sellers/me/*` used by website but outside this task (other modules, all in openapi): customers/*, tags/*, posts, notification-preferences, assistant/product-listing/chat, `/app/sellers/posts`, `/app/sellers/reviews/{id}/reply`, `/public/sellers/{id}/forum`.

### 1.2 Website counts
- Distinct (method, path) pairs in scope: 36 live (W1-W20 = 21 pairs counting GET/PUT /me separately; W22-W27 = 7; W29-W37 = 9 incl. address x2; W39-W40 = 4 incl. PATCH) plus 2 dead pairs (W21, W28 x2).
- Files with a real HTTP call site (non-test): 13 -> lib/seller/{repo,certification,violations,shipping,canonical-redirect}.ts, lib/vendor-portal/repo.ts, lib/api/index.ts, lib/api/cached.ts (labels only), lib/chat/repo.ts, lib/seller-data/delegated-repos.ts, features/product/ProductViewTracker.tsx, app/(delegated)/adm/api/storefront/[storeId]/{route,logo/route,cover/route}.ts.
- Old inventory: 34 paths / 41 files (heuristic that also counted UI consumers, tests, comments). Consumer-level here: 28 files use `useSellerSelf`/`useSellerIdentity` (`rg -l "useSellerSelf|useSellerIdentity"`), 17+ read the public seller entity (W29).
- Test files pinning these REST contracts (must change with Connect): 40+ (`rg -l "sellers/me|vendor_portal|/public/sellers|/app/certifications|businessRegistration|/public/marketplace" website -g '*test*' -g '*.spec.ts'`), notably tests/seller-profile-contract.test.ts, seller-violation-appeal-contract.test.ts, seller-logo-finalize-contract.test.ts, certification-id.test.ts, certification-step-data-no-urls.test.ts, lib/vendor-portal/repo.unit.test.ts (484 lines), lib/seller-data/seller-wire-contract.unit.test.ts, e2e/vendor-rfq-landing.spec.ts, e2e/seller-wallet-merchant-profile-edit.spec.ts.

### 1.3 Error-code handling in website (`rg "9240|SELLER_SUSPENDED|9233|9266|9241" website`)
- 9240 seller suspended: `lib/api/client.ts:40-41,372-374,606-607,632-633` (authed response code 9240 -> window event `hk-seller:suspended`) -> `features/seller-center/components/SellerCenterShell.tsx:96` redirect to `/seller-center/suspended` (whitelist) and suspends queries; also derived client-side from `lifecycleStatus` (`isSellerSuspendedLifecycle`, used by `lib/hooks/useSellerInvoices.ts`); `useSellerIdentity.ts:39` reads `suspensionReasonCode`; labels via CodeMeta `SELLER_SUSPENSION_REASON_LABEL` (`SellerSuspendedNotice.tsx:23-30`). Needs: Connect `permission_denied` + ErrorInfo.reason SELLER_SUSPENDED mapped in client.ts, plus a way to read suspension reason (GetStore field).
- 9233 SellerNotFound: `lib/seller/repo.ts:40,48-55`.
- 9241 (seller suspended, product unavailable; buyer side): `features/cart/CartClient.tsx:716`, `features/checkout/screens/CheckoutClient.tsx:948`.
- 9266 version conflict / 9260 missing version: delegated edit only (`features/seller-center/products/ProductsPage.tsx:333`, `features/delegated-operations/products/DelegatedProductEditPage.tsx:318`). Note: the seller self-service profile PUT sends NO version today, while backend `UpdateProfile` requires `ExpectedVersion` -> website needs a version in GET /me.
- HTTP 409: violation appeal already submitted (`lib/seller/violations.ts:99-104`); vendor draft CAS conflict.
- Open-orders-on-close: not handled (no close-store UI).

---

## 2. admin/ (Next.js) — per-client pass

# Step 8 admin REST inventory (HEAD, read-only)

Commands: `rg -n -e '/api/v1/admin/(sellers|certifications|seller_prospects|...)' -e '/public/(sellers|marketplace)' -e vendor_portal -e '/app/sellers' src -g '!*.test.*'`, `rg -n -i 'sellers|certifications|accounts/|seller_prospect|marketplace' src`, `rg 'api/v1/(public|app)'`, then per-file reads. openapi check: `rg -n 'admin/(sellers|seller_|certif|accounts|violation|food|delegated)' backend/api/openapi/openapi.yaml` plus awk method listing.

## Transport (resolved base path)
- Browser: `services/api.ts` `request()` -> `fetch(environmentService.getGatewayUrl() + endpoint)`; `GATEWAY_URL` = same-origin `/api/bff` (services/environment.ts). Every call below is therefore `/api/bff/api/v1/admin/...`, BFF (`app/api/bff/[...path]/route.ts`) adds Bearer and forwards to `BACKEND_BASE_URL/api/v1/admin/...` (monolith, proxies to legacy gateway). Table paths are shown as gateway path `/api/v1/admin/...`.
- Envelope: `{code,message,data,traceId}`; `request()` throws `ApiClientError(message, code, traceId, data)` when `code!==0` (business code in body) or !ok; 401 -> sessionExpired; 404 -> ApiClientError with body message. No call site branches on 9240/9265/9266/9272 (only delegated-operations code does). Generic display = `governanceErrorText()` -> "message（代碼 N）（追蹤編號 T）" (merchants/[id]/GovernanceNotice.tsx).
- No admin use of: /public/sellers, /public/marketplace, vendor_portal, /app/sellers*. Only `/api/v1/app/media/*` (services/media-upload.ts, not seller domain). `src/lib/gen` Connect stubs exist only for admin buyer/content/identity: NO seller Connect client in admin.

## A. Call-site table (40 distinct method+path; 14 files + 1 server-side delegated file)

### A1. Sellers admin / certifications / account delete
| file:line | method path | UI | response fields read / request fields | reachable | errors |
|---|---|---|---|---|---|
| admin/merchants/page.tsx:122 | GET /admin/sellers?page&limit&keyword&status=(approved\|pending\|forwarded\|forward_failed\|rejected)&lifecycle_status=suspended | /admin/merchants list (Sidebar:54) | data.{list,total}; per row: accountId, storeId, storeCode, shopName, logoKey, contactEmail, sellerType, city, district, lifecycleStatus(suspended\|suspended_governance), overallCertificationStatus, status, merStatus, rating, createdAt. Declared but NOT rendered: contactPhone, addressDetail, payuniStatus, updatedAt | yes | none specific (react-query default); code!==0 -> empty list |
| merchants/page.tsx:151 | GET /admin/sellers/stats | stats cards | data.{total,pending,approved,rejected,monthlyNew} (strings, Number()) | yes | code!==0 -> zeros |
| admin/page.tsx:41 | GET /admin/sellers/stats | dashboard (/admin) | data.total, data.pending | yes | try/catch console.warn |
| analytics/page.tsx:66 | GET /admin/sellers/stats | /admin/analytics | data.total | yes (Sidebar:23) | try/catch console.error |
| seo/page.tsx:35 | GET /admin/sellers?page=1&limit=100[&keyword] | /admin/seo (Sidebar:157) | data.list[]: accountId, storeId, shopName, logoKey, description, coverKey, seoTitle, seoDescription | yes | code!==0 -> [] |
| seo/page.tsx:66 | PATCH /admin/sellers/{store_id}/seo | seo edit modal | req {seoTitle, seoDescription}; response unused | yes | console.warn only (silent) |
| seller-prospects/[id]/ConvertPanel.tsx:88 | GET /admin/sellers?keyword&page=1&limit=10 | prospect "attribution" search | data.list[]: storeId, shopName, contactEmail, contactPhone | yes (when canConvert) | governanceErrorText |
| invoices/issuances/new/page.tsx:336 | GET /admin/sellers/{store_id} | manual invoice issuer lookup | data.{storeId,storeName} | yes (/admin/invoices/issuances/new) | governanceErrorText |
| merchants/[id]/page.tsx:483 | GET /admin/sellers/{store_id} | merchant detail (KYC review page) | see B2 (SellerProfile + nested certification) | yes (row click from list) | isError screen |
| merchants/[id]/page.tsx:500 | GET /admin/certifications/{certId}/audit-logs | restores last review | data[0].{requiredFields[], rejectionReason} (takes newest) | yes | try/catch -> null (swallowed) |
| merchants/[id]/page.tsx:627 | PATCH /admin/certifications/{certId} (approve) | KYC approve | req {status:"approved", comment:"", payuniWebsiteUrl}; response unused | yes (canApprove: submitted\|forward_failed) | governanceErrorText notice |
| merchants/[id]/page.tsx:647 | PATCH /admin/certifications/{certId} (reject) | KYC reject | req {status:"rejected", rejectionReason, comment, requiredFields[]} | yes | governanceErrorText notice |
| merchants/[id]/page.tsx:600 | GET /admin/sellers/{store_id}/suspension | active suspension card | data.suspension (may be {} / null): id, reasonCode, reasonDetail, suspendedAt, suspendedByAdminId (+ declared accountId,status,liftedByAdminId,liftedAt,createdAt) | yes | none (query error silent) |
| merchants/[id]/page.tsx:614 | GET /admin/sellers/{store_id}/suspensions | suspension history table | data.suspensions[]: id,reasonCode,reasonDetail,status,suspendedAt,suspendedByAdminId,liftedAt,createdAt | yes | none |
| merchants/[id]/page.tsx:678 | POST /admin/sellers/{store_id}/suspensions | suspend modal | req {reasonCode (CodeMeta value, default POLICY_VIOLATION), reasonDetail}; response unused. Success text: "商品已全部下架，未付款訂單已取消" | yes | governanceErrorText (no code branching; no "already suspended"/open-orders special case) |
| merchants/[id]/page.tsx:690 | POST /admin/sellers/{store_id}/suspensions/{suspension_id}/lift | lift modal | req {} | yes | governanceErrorText |
| merchants/[id]/page.tsx:712 | DELETE /admin/accounts/{accountId}?reason= | SellerAccountDeleteDialog | req query reason (required non-empty client side); response body unused. NOTE id is seller.accountId, not storeId | yes | governanceErrorText shown in dialog; no special handling of open-orders / 9xxx / 409 |
| (adjacent, other domain) merchants/[id]/page.tsx:523,543,571,585,669 | GET /admin/products?owner_id=accountId..., GET /admin/orders?store_id=accountId..., PUT /admin/products/{id}/status {status:"4003"} | detail tabs | out of scope but depends on accountId | yes | |

### A2. Seller membership plans
| file:line | method path | UI | fields | reachable | errors |
|---|---|---|---|---|---|
| seller-membership-plans/page.tsx:116 | GET /admin/seller_membership_plans | /admin/seller-membership-plans (Sidebar:175) | data.items[]: planCode, displayName, tierRank, monthlyOrderCap, monthlyPriceTwd, isPurchasable, createdAt, updatedAt | yes | listError shown |
| seller-membership-plans/page.tsx:149 | PATCH /admin/seller_membership_plans/{plan_code} | edit modal | req {monthlyOrderCap, monthlyPriceTwd, isPurchasable} (strings) | yes | toast err.message |
| (none) | /admin/sellers/{id}/membership GET/POST | exists in openapi, NO admin call site | - | - | - |

### A3. Seller prospects (13 routes + 3 media) via services/seller-prospects.ts BASE=/api/v1/admin/seller_prospects, services/seller-prospect-media.ts
Types: lib/seller-prospects.ts. Wire: numbers as strings (total/page/pageSize), camelCase.
| service fn (file:line) | method path | UI consumer | request / response fields read | reachable | errors |
|---|---|---|---|---|---|
| listSellerProspects (seller-prospects.ts:60) | GET /admin/seller_prospects?page&page_size&status&outreach_status&vendor_type&category&region&keyword | seller-prospects/page.tsx:100 (merchant list, Sidebar:67); ProspectFilters | data.{records,total,page,pageSize}; rows: id,displayName,vendorType,status,outreachStatus,taxId,category,region,updatedAt,mergedIntoProspectId,logoUrl | yes | default |
| createSellerProspect (:~125) | POST /admin/seller_prospects | CreateProspectDialog:117 | req UpsertProspectPayload {displayName,vendorType,taxId,publicEmail,publicPhone,publicWebsite,publicSocial,category,region,productSummary,positioning,researchSummary,priorityLetter,priorityStars}; resp data.{prospect(ProspectPreview),matches[{prospect,tier,matchedOn}]} | yes | governanceErrorText |
| previewSellerProspect (:144) | POST /admin/seller_prospects/preview | CreateProspectDialog:188 (live dedupe) | same body; resp data.matches[] | yes | |
| get (:~195) | GET /admin/seller_prospects/{id} | [id]/page.tsx:135, ProspectMediaPanel:70 | data.prospect (full SellerProspect incl. convertedStoreId, convertedAt, mergedIntoProspectId, logoUrl/coverUrl/mediaVersion/mediaEditable, createdBy/updatedByAccountId), data.invites[] | yes | 404 -> not-found view (page.tsx:146) |
| update | PUT /admin/seller_prospects/{id} | ProspectEditForm:117 | req Upsert + outreachStatus (never CONVERTED); resp SellerProspect | yes | 404 handled (ProspectEditForm:164); else notice |
| archive | DELETE /admin/seller_prospects/{id} | ArchiveDialog:49 | resp SellerProspect | yes | text from backend message (409 shown verbatim) |
| restore | POST /admin/seller_prospects/{id}/restore | ArchiveDialog:50 | resp SellerProspect | yes | 409 (merged-away) shown verbatim |
| convert | POST /admin/seller_prospects/{id}/convert | ConvertPanel:97, reconciliation/page.tsx:92 | req {storeId}; resp ignored | yes | reconciliation: `ApiClientError.code===409` locks the row (lines 136,158); ConvertPanel generic notice |
| createInvite | POST /admin/seller_prospects/{id}/invites | InvitePanel:97 | req {validDays?:string}; resp data.{invite{id,prospectId,expiresAt,sentAt,clickedAt,consumedAt,revokedAt,createdAt}, token} (token shown once in InviteTokenReveal) | yes | generic |
| revokeInvite | DELETE /admin/seller_prospects/{id}/invites/{invite_id} | InvitePanel:108 | resp none | yes | consumed invite -> backend 409, UI pre-disables button |
| duplicates | GET /admin/seller_prospects/duplicates | duplicates/page.tsx:54, seller-prospects/page.tsx:111 (badge count) | data.{list[{left,right,matchedOn}],truncated} | yes | |
| reconciliation | GET /admin/seller_prospects/reconciliation | reconciliation/page.tsx:84, page.tsx:116 | data[] {prospect, storeId, storeName, certificationCompanyName, certificationStatus} | yes | |
| merge | POST /admin/seller_prospects/merge | duplicates/MergeDialog:99 | req {winnerId,loserId}; resp SellerProspect | yes | 409 verbatim |
| stageProspectImage (seller-prospect-media.ts:20) | POST /admin/seller_prospects/media/upload_sessions | ProspectMediaPanel:133 | req {filename,contentType,fileSize:string}; resp data.{sessionId,uploadUrl(https only)} | yes | thrown Error strings |
| (same :43) | PUT {uploadUrl} (presigned, not gateway) | | | | |
| (same :43) | POST /admin/seller_prospects/media/upload_sessions/{session_id}/complete | | req {}; resp data.stagedToken | yes | |
| saveProspectImage (:57) | PUT /admin/seller_prospects/{id}/media | ProspectMediaPanel:136 | req UpdateProspectMediaPayload (lib/seller-prospect-media.ts); resp SellerProspect verified by assertProspectMediaSaved | yes | `code===409` handled at ProspectMediaPanel:160 |
Dead code in this area: none found (every `sellerProspectsApi` member and both media fns have a rendered consumer). `src/lib/seller-prospect-media.ts` is helper only.

### A4. Governance
| file:line | method path | UI | fields | reachable | errors |
|---|---|---|---|---|---|
| violation-reports/page.tsx:135 | GET /admin/violation_reports?limit=50&offset=0&status&severity&source | /admin/violation-reports (Sidebar:38), 30s poll | data.rows[]{source, severity, created_at_ms, seller_report{id,status,category}, food_safety{id,status,acknowledged_at_ms}} (SNAKE_CASE) | yes | none |
| violation-reports/[source]/[id]/page.tsx:93 | GET /admin/violation_reports?source=&limit=200 (list used as detail lookup, finds id client-side) | report detail | seller_report/food_safety: id,target_seller_id,target_product_id,target_order_no,target_post_id,reporter_account_id,category,severity,description,evidence_asset_ids,status,investigating_at_ms,resolved_at_ms,admin_resolution_summary,created_at_ms,reporter_phone/email/name,acknowledged_at_ms,acknowledged_by_admin_id,fda_reported_at_ms,fda_report_channel,fda_reference_no | yes | not found if >200 rows |
| same:122 | PATCH /admin/seller_violation_reports/{id} | status advance | req {status, adminResolutionSummary} | yes | alert(e.message) |
| same:122 | PATCH /admin/food_safety_reports/{id} | same | same | yes | alert |
| same:115 | POST /admin/food_safety_reports/{id}/ack | ack | req {} | yes | alert |
| same:156 | GET /admin/food_safety_reports/{id}/fda_export | FDA md | data.markdown | yes | alert |
| same:177 | POST /admin/food_safety_reports/{id}/mark_fda_reported | | req {fdaReportChannel, fdaReferenceNo} | yes | alert |
| same:188 | POST /admin/food_safety_reports/{id}/notify_seller | | req {templateKey: assist_investigation\|temporary_takedown_self_check} | yes | alert |
| violations/new/page.tsx:104 | POST /admin/violations | create violation (linked from report detail:388, invoices/complaints:527,554) | req {sellerId, category, severity, penaltyType(warning\|full_suspension), description, publicSummary?, expiresAtMs?, penaltyAmountCents?, sourceSellerReportId?, sourceFoodSafetyReportId?}; resp data.{violation.id, severityAutoEscalated, originalSeverity, appliedSeverity} | yes | alert(message) |
| violations/[id]/page.tsx:76 | GET /admin/violations/{id} | violation detail | data.violation: id,seller_id,source_seller_report_id,source_food_safety_report_id,category,severity,penalty_type,penalty_amount_cents,description,public_summary,effective_at_ms,expires_at_ms,status,revoked_at_ms,revoked_reason,appealed,appeal_reason,appeal_evidence_asset_ids,appeal_status,appeal_submitted_at_ms,appeal_decided_at_ms,appeal_decision_reason,created_by_admin_id,created_at_ms (snake_case) | PARTIAL: no list page, no sidebar; reachable only by redirect after create (new/page.tsx:111) | |
| same:87 | POST /admin/violations/{id}/revoke | | req {revokedReason} | as above | alert |
| same:96 | POST /admin/violations/{id}/appeal/decide | | req {accept:boolean, decisionReason} | as above | alert |

### A5. Delegated operations (NOTE ONLY; not via generic BFF, which 403s that namespace)
Browser -> `/api/admin/delegated-operations/**` (services/delegated-operations.ts) -> server routes -> `features/delegated-operations/server/upstream.ts` gateway paths: `/api/v1/admin/delegated-operations` (+/audit-logs, /capabilities, /authorizations, /authorizations/{store_id}[/revoke|/history], /{session}/audit-logs|summary|revoke) and `GET /api/v1/admin/sellers/{store_id}/delegated-operation/preflight` (route `/api/admin/merchants/[storeId]/delegated-operation/preflight`). These DO branch on 9264/9265/9268/9269/9272/9285/9286/9287/9260. Merchant list/detail only show MerchantOperationEntry (preflight authoritative; detail comment: suspended/closed store -> 9272, no governance permission -> 9265).

## B. Counts
- Distinct files with seller/cert/prospect/governance REST call sites: 14 (merchants/page, merchants/[id]/page, seo/page, analytics/page, admin/page, invoices/issuances/new/page, ConvertPanel, seller-membership-plans/page, violation-reports/page, violation-reports/[source]/[id]/page, violations/new/page, violations/[id]/page, services/seller-prospects.ts, services/seller-prospect-media.ts) + features/delegated-operations/server/upstream.ts (note only).
- Distinct (method,path) pairs, excl. delegated: 40 = sellers 8 (GET list, GET stats, GET {id}, PATCH seo, GET suspension, GET suspensions, POST suspensions, POST lift) + certifications 2 + accounts 1 + membership plans 2 + prospects 16 + governance 7 + violations 4. (+1 preflight GET if counting delegated; + 11 more delegated upstream paths.)
- Paths in admin NOT in openapi.yaml: NONE (all present as `/api/v1/admin/...`; openapi uses `{id}` for lift and `{session_id}` for media complete, equivalent). All methods match openapi (checked via awk).
- In openapi but unused by admin: GET /admin/certifications, GET /admin/certifications/{id}, GET+POST /admin/sellers/{id}/membership, GET /admin/sellers/{id}/orders, PATCH /admin/violations/{id}.

## C. Fields UI reads that the new seller Store model lacks (model: StoreName, Description, ContactPhone, TownCode, AddressDetail, Prize, WebsiteURL, SEOTitle, SEODescription, IsProcessedFood, FoodBusinessRegNo, OnboardingCompleted, StoreID, StoreCode, AccountID, Version, Status, Suspension, LogoKey, CoverKey, timestamps)
| UI field | where | gap |
|---|---|---|
| shopName | list, seo, ConvertPanel | naming only: model has StoreName (detail & invoices page already use storeName) |
| city, district | list col "地點" (merchants/page:569), detail address line (1837) | no city/district names; model has TownCode only -> need lookup or derive |
| rating | list (594), detail card (1211,1237) | NOT in model |
| contactEmail | list (531), detail, ConvertPanel/ConvertConfirmDialog ("店名可重複，信箱才是身分欄位") | NOT in model (identity module) |
| sellerType (business\|individual) | list badge | NOT in model (comes from KYC certification) |
| lifecycleStatus (suspended\|suspended_governance), status (0/1/2 number in detail), overallCertificationStatus (approved/pending/forwarded/forward_failed/rejected) | list badge + filters `status=` / `lifecycle_status=`; detail canApprove/canReject FSM | model Status = ACTIVE/PENDING_DELETION/CLOSED; certification FSM states + governance-suspension flavour not in model |
| merStatus, payuniStatus, payuniMerId, payuniReason, memberStatus, errorMessage, errorDisplayMessage, lastSyncedAt, registeredAt, approvedAt, freshness, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel | list PayUni badge; detail finance/PayUni cards | all PayUni/payment data, not in model |
| certification{id,applicationNo,status,sellerType,realName,idCardNo,birthDate,idCard*Url,bankBookUrl,bankName,bankAccount,companyName,taxId,company* ,productType,idCardIssue*,companyRegisterDate,documentUrls,submittedAt,websiteUrl,hasPublishedPost,payuniWebsiteUrl,platformStorefrontUrl,foodBusinessRegNo} | detail page KYC review (whole page core) | only foodBusinessRegNo, websiteUrl exist on Store; KYC lives elsewhere. documentUrls are fetched as gateway paths through `/api/bff` (`fetchSecureAsset`, merchants/[id]/page.tsx:190) -> binary download route also needed |
| stats.{total,pending,approved,rejected,monthlyNew} | stats cards | pending/approved/rejected are certification counts; `monthly` tab filter sends NO query param (merchants/page.tsx:122-136) so it lists all |
| ReconciliationMatch.storeName, certificationCompanyName, certificationStatus | reconciliation page, ConvertConfirmDialog | cert fields not in seller model |
| Not found in admin: followers, invite codes on seller (invites are prospect-level only), membership plan/tier on merchants pages (plans only on seller-membership-plans page), certified flags except certificationStatus above |

## D. Surprises
1. Violation/report payloads are snake_case with `_ms` timestamps and no `{code,data}` camelCase consistency; reports detail has no GET-by-id: it lists `limit=200` and finds client-side (breaks for old rows).
2. `/admin/violations/{id}` has no list page/sidebar link; reachable only after creating a violation.
3. No error branching for 409 / 9240 / 9265 / 9266 / 9272 / open-orders in sellers/certifications/suspension/account-delete flows: UI just prints backend message + `（代碼 N）`. Only prospects reconciliation (409 lock) and media (409) branch. Contract must keep human-readable `message` + numeric business code in envelope.
4. DELETE /admin/accounts/{id} takes the seller's accountId (not storeId); orders/products tabs also pass `accountId` into `store_id`/`owner_id` query params.
5. SEO PATCH failures are silent (console.warn); audit-logs failure silently ignored.
6. Detail page: success copy for suspend asserts "商品已全部下架，未付款訂單已取消" (side effects the new module must preserve or text changes).
7. Admin has no Connect usage for sellers; everything is hand-typed REST.

---

## 3. buyer-app/ (Flutter) — per-client pass

# buyer-app step 8 inventory (HEAD, read-only)
Root: <repo>/buyer-app/lib (paths below relative to lib/features unless noted)
Commands: rg for `'/(app|public)/...'` literals, `getMerchantInfo|fetchSellerInfo|getShareUrl|getSellerProducts|getFansCount`, `924\d|409`; openapi checked with rg on `^  /api/v1/...` (openapi paths carry /api/v1 prefix; Dio baseUrl supplies it).
Not found anywhere in lib: /public/sellers/recommended, /public/sellers/{id}/reviews, reviews/stats, /violations/public, /public/products/{id}/view, /public/food_safety_reports, /app/violation_reports, /app/address/suggestions|validate, /app/businessRegistration, vendor_portal. (rg for each: 0 hits.)
/app/chat/messages: not called by Dio. chat_repository uses /app/chat/{ws-ticket,history/:id,conversations,unread,conversations/:id PATCH} (chat_repository.dart:558 _httpBase) — out of scope list but seller-adjacent.
Connect (BuyerAppService) already used for: follow list/follow/unfollow/checkFollow (follow/data/follow_repository.dart:28-52), favorites + visits (visit/data/visit_repository.dart), address (address/data/address_repository.dart). Follow rows give store.id/name/logo via Connect, not REST.

## A. REST (Dio) call sites
| # | file:line | method path | screen/feature | keys read | reachable | error handling |
|---|---|---|---|---|---|---|
| 1 | seller/data/seller_repository.dart:54 (getMerchantInfo, returns whole envelope Map or null) | GET /public/sellers/{id} | shared helper; see callers C1-C9 | see C | yes | DioException -> null (debugPrint only); no 9240/409 |
| 2 | seller/data/seller_repository.dart:73 (getRatingStats) | GET /public/sellers/{id}/rating | my_subscription_screen.dart:653 (follows tab fallback) | RatingStats.fromJson: storeId, rating, reviewCount, fiveStarPct..oneStarPct (only rating+reviewCount used) | yes (settings -> MySubscriptionScreen, router app_router.dart:341) | code!=0 -> ServerException; 401 -> Unauthorized; else NetworkException; caller swallows `catch(_){}` |
| 3 | seller/presentation/merchant_screen.dart:160 | GET /public/sellers/{id}/rating | MerchantScreen header (route app_router.dart:171) | data.rating, data.reviewCount | yes | catchError -> null (rating 0) |
| 4 | seller/presentation/merchant_screen.dart:165 | GET /public/sellers/{id}/stats | MerchantScreen header | data.followers, data.totalProducts (totalProducts read into _cachedTotalProducts, never rendered in header) | yes | catchError -> null |
| 5 | follow/data/follow_repository.dart:69 (getFansCount) | GET /public/sellers/{id}/stats | none | data.followers | DEAD: no callers (rg getFansCount) | code!=0 ServerException; 401; NetworkException |
| 6 | seller/data/seller_repository.dart:26 (getSellerProducts) | GET /public/sellers/{id}/products | none | list via data/records|items|list|rows -> Product.fromJson | DEAD: no callers | 401/Network |
| 7 | seller/presentation/merchant_screen.dart:422 | GET /public/sellers/{id}/products ?page=1&page_size=50&status=1 | MerchantScreen prefetch for shop tab | extractList(data.records|items|list|rows) | yes | catch -> prefetchReady, no UI error |
| 8 | seller/presentation/widgets/merchant_shop_tab.dart:193 | same as 7 | MerchantShopTab (merchant_screen.dart:594) | per item: title/name/skuName..; priceMin, priceMax, imageKeys/imageUrls/imageKey/imageUrl, skuId/id, merchantName/storeName/brand, listPriceMin/Max, salePriceMin/Max, isRestockSubscribed, reviewCount, rating/averageScore, stock, skus[].stockQty, isFavorited | yes | catch -> empty list, HKLog |
| 9 | product/data/product_repository.dart:74 (getMerchantProducts) | GET /public/sellers/{id}/products (no params) | chat_thread_screen.dart:977 product picker; getMerchantFirstProductImage (131) used chat_thread_screen.dart:496 | list; image keys imageKeys/imageUrls/imageKey/imageUrl; picker reads product map | yes (chat thread) | DioException swallowed -> [] |
| 10 | seller/presentation/widgets/merchant_home_tab.dart:132 | GET /public/sellers/{id}/posts | MerchantHomeTab | data.records[]: id/postId, isLiked, likeCount, commentCount, product{id,name,...}, images | yes | 404 -> empty list; other rethrow -> '載入失敗' |
| 11 | social/data/social_repository.dart:26 (getPosts merchantId!=null) | GET /public/sellers/{id}/posts ?page&page_size | social/presentation/post_feed_controller.dart:28,44 -> merchant_posts_feed_screen (app_router.dart:471) | records/rows/list/items; sellerName, sellerId, logoUrl, product, Post.fromJson | yes | code!=0 ServerException; 401; NetworkException |
| 12 | social/data/social_repository.dart:128 (_hydrateSellers) | GET /public/sellers/{id} (per post when sellerName empty; N+1) | posts feed / home post preview (home/data/home_post_preview_repository.dart:24 calls getPosts w/o merchantId, also hydrates) | sellerName, storeName, logoKey (writes post.sellerName, post.logoUrl) | yes | catch(_) swallowed |
| 13 | category/data/category_repository.dart:408 (fetchSellerInfo) | GET /public/sellers/{id} (per distinct storeId of listed products; N+1) | CategoryScreen (main_navigation_screen.dart:77,133; category_screen.dart:359) product card seller name/logo | data.storeName, sellerName, logoKey, logoUrl | yes | exception -> (name:'',logo:'') ; HKLog |
| 14 | category/data/category_repository.dart:262 (fetchRecommendedSellers) | GET /public/marketplace ?seller_limit=10&post_limit=0&product_limit=0 | CategoryScreen recommended-seller rail (category_screen.dart:200) | data.recommendedSellers[]: storeId, storeName, logoKey, logoUrl, city, district | yes | exception caught, rail empty |
| 15 | category/data/category_repository.dart:320 | GET /public/marketplace ?seller_limit=0&post_limit=0&product_limit=12 | category_landing_screen (via category_marketplace_controller.dart:22) | data.hotProducts[]: id,name,imageKey,imageUrl,imageKeys,storeId,seller{storeId,name},sellerName,priceMin,priceMax | yes | code!=0 -> FormatException -> error/retry state |
| 16 | product/presentation/search_screen.dart:504 | GET /public/marketplace ?product_limit=12 | search landing recommendations | data.hotProducts[]: id,name,imageKey,imageUrl,storeId,seller{name,storeId},sellerName,priceMin,priceMax | yes | catch ignore |
| 17 | product/data/product_repository.dart:321 (getShareUrl) | GET /app/share_url ?type=product|post&id | callers: product/presentation/merchant_item_detail_screen.dart:307, social/presentation/merchant_posts_feed_screen.dart:346, social/presentation/post_detail_screen.dart:188 | data.title, description, imageUrl, url | yes | DioException -> null, fallback to https://example.com/... URL |
| 18 | seller/presentation/widgets/merchant_chat_tab.dart:162 | GET /public/sellers/{id}/forum | MerchantChatTab (merchant_screen.dart:603) | data.records[] comment node fields (id,userId,userName,userAvatar,replyCount,...) | yes | catch -> loading false |
| 19 | merchant_chat_tab.dart:197 | GET /public/sellers/forum/{id}/replies | same | records[] | yes | swallowed |
| 20 | merchant_chat_tab.dart:246 | GET /public/sellers/{uid} (uid = commenter user id, per-commenter; N+1; commenter is a user, not necessarily a store) | same | storeName, sellerName, logoKey, logoUrl | yes | catch(_) swallowed |
| 21 | merchant_chat_tab.dart:286 | POST /app/sellers/forum/{id}/reply | same | status only | yes | catch -> snackbar |
| 22 | merchant_chat_tab.dart:294 | POST /app/sellers/{id}/forum | same | status only | yes | catch -> snackbar |
| 23 | merchant_chat_tab.dart:318 | PUT /app/sellers/forum/{id} | same (edit) | status only | yes | catch -> snackbar |
| 24 | merchant_chat_tab.dart:411 | DELETE /app/sellers/forum/{id} | same | status only | yes | snackbar '刪除失敗: $e' |
| 25 | merchant_chat_tab.dart:361 | POST /app/posts/comments/{id}/like | same | (post-comment API, adjacent) | yes | - |
Adjacent seller-bearing product/post calls: GET /public/products (category_repository.dart:146,178 list; :178 fetchOriginCities reads raw.seller{city,district,storeId,name,logoKey,logoUrl}, raw.sellerName, raw.sellerLogoKey/Url, raw.storeId; product_repository.dart:32; home/data/home_product_repository.dart:30), GET /public/products/{id} (product_repository.dart:57,161; social_repository.dart:149; merchant_home_tab.dart:230; reads seller{storeId,name,city,district}, storeId), GET /public/posts (social_repository.dart:26), /public/products/{id}/{brief,rating,reviews} (product_repository.dart:145,194; review_repository.dart:68,90,103).

## C. getMerchantInfo / GET /public/sellers/{id} caller key tables (all read data.* via `resp['data'] is Map ? ... : resp`)
C1 MerchantScreen settings merchant_screen.dart:143,_applyMerchantData(222-330): storeName, city, district, addressDetail, address, postalCode, description, coverKey/coverUrl, logoKey/logoUrl, prize, storeId, rating, overallCertificationStatus ('approved' -> certified badge). Followers/rating count/total products come from #3/#4. Header gets address, location(city+district), rating, ratingCount, followerCount, isCertified, award.
C2 my_subscription follows tab: my_subscription_screen.dart:589 (_fetchFollowingPage): logoKey/logoUrl, storeName (if Connect name empty), city, district, addressDetail (address line), rating (then #2 fallback for rating/reviewCount). Card (1514-1620) renders name, avatar, address, rating, reviewCount. Followers/certified/storeCode not read.
C3 my_subscription favorites tab: :754 (_fetchLikesPage): storeName, city+district -> 'distance', logoKey/logoUrl -> sellerAvatar. (Card at ~1955 renders storeName, sellerAvatar, distance, rating(product), reviewCount(product).)
C4 my_subscription recent-views tab: :959 (_fetchRecentPage): same keys as C3.
C5 merchant_item_detail_screen.dart:469 (location fallback): city, district. :1992 (_MerchantInfoCardNew._loadData): logoKey/logoUrl, storeName, rating (+ debug sellerName).
C6 notifications_screen.dart:59 (kind=='seller'): store_name, storeName, merchantName, sellerName, name; logoKey, logoUrl, sellerAvatarUrl, merchantAvatar, avatarUrl, avatar_url, avatar, logo.
C7 chat_thread_screen.dart:456: logoKey/logoUrl, storeName.
C8 post_detail_screen.dart:107: storeName/sellerName, logoKey/logoUrl.
C9 social_repository.dart:128 (#12), category_repository.dart:408 (#13), merchant_chat_tab.dart:246 (#20): see above.
Gateway PublicSellerInfo (openapi.yaml:43377) fields: city, coverKey, coverUrl, description, district, foodBusinessRegNo, isProcessedFood, logoKey, logoUrl, overallCertificationStatus, prize, rating, sellerName, sellerType, seoDescription, seoTitle, storeCode, storeId, storeName, websiteUrl. (addressDetail/address/postalCode are NOT in gateway either.)

## D. Fields UI reads that do not exist in new backend seller Store
- rating (C1,C2,C5, #3 data.rating, #2) — no rating in Store; /rating endpoint is review-derived (review module), /stats.followers is follow-derived.
- city, district (C1-C5, #14 recommendedSellers, fetchOriginCities seller.city/district, _buildSellerLocation) — Store has only TownCode; names must be resolved from TownCode.
- overallCertificationStatus -> certified badge (C1) — not in Store.
- postalCode, address (full), addressDetail partly (Store has AddressDetail: ok; address/postalCode not) (C1, C2).
- followers (#4, #5) — not seller; follow module. totalProducts (#4, unused) — not seller.
- sellerName / sellerType (fallbacks C-all) — not in Store (StoreName only).
- logoKey ok (LogoKey), coverKey ok, description ok, prize ok, storeId ok, storeName ok. logoUrl/coverUrl are derived URLs (ok if gateway emits pair).
- membership plan, storeCode: not read by buyer UI. storeCode never read.
- Not read: contactPhone, websiteUrl, seo*, isProcessedFood, foodBusinessRegNo.

## E. Counts
Distinct files with Dio seller/marketplace/share_url/forum calls (non-generated): 15 — seller_repository.dart, follow_repository.dart (dead method), product_repository.dart, category_repository.dart, social_repository.dart, merchant_screen.dart, merchant_shop_tab.dart, merchant_home_tab.dart, merchant_chat_tab.dart, search_screen.dart, my_subscription_screen.dart (via getMerchantInfo/getRatingStats), plus getMerchantInfo-only callers merchant_item_detail_screen.dart, notifications_screen.dart, chat_thread_screen.dart, post_detail_screen.dart (those 4 = 15 total). Files containing a literal path: 11; via helper: +4.
Distinct (method,path) pairs, in-scope: 14 — GET /public/sellers/{id}; GET .../{id}/rating; GET .../{id}/stats; GET .../{id}/products; GET .../{id}/posts; GET .../{id}/forum; GET /public/sellers/forum/{id}/replies; GET /public/marketplace; GET /app/share_url; POST /app/sellers/{id}/forum; POST /app/sellers/forum/{id}/reply; PUT /app/sellers/forum/{id}; DELETE /app/sellers/forum/{id}; (+ POST /app/posts/comments/{id}/like adjacent = 14).
Call sites (rows): 25; dead: #5 getFansCount, #6 getSellerProducts. (getMerchantFirstProductImage reachable.)

## F. Paths in buyer-app NOT in openapi.yaml
None: all above exist in backend/api/openapi/openapi.yaml with /api/v1 prefix (public/sellers/{id}[/rating|/stats|/products|/posts|/forum], public/sellers/forum/{id}/replies, public/marketplace, app/share_url, app/sellers/{id}/forum (29062), app/sellers/forum/{id} (24823, delete+put), .../reply (24955), public/posts, public/products*). Note openapi also lists public/sellers/recommended, {id}/reviews, reviews/stats, violations/public, products/{id}/view, app/chat/messages that buyer-app never calls (no buyer consumer -> those are not buyer-blocked).

## G. Error handling for 9240/9241/409/92xx
- None on seller/marketplace paths. core/exceptions/seller_suspension_codes.dart defines 9240/9241 (file comment "no caller" is stale): used at checkout/cart — cart/presentation/cart_controller.dart:911-945, cart/presentation/widgets/checkout_payment_sheet.dart:607 (sellerSuspendedProductUnavailable 9241 only; 9240 unused outside definition). These are order/checkout, not seller reads.
- No 409 handling on seller paths. 92xx in code_meta.dart:1090-1102 are unrelated (asset states). getMerchantInfo swallows all errors to null; a suspended seller returning an error shows MerchantScreen empty state (_merchantNotFound) or missing name/avatar.

---

## 4. seller-app/ (Flutter) — per-client pass

# seller-app gateway REST inventory (HEAD 174adc432, lib/ only, generated files skipped)

Commands: `rg -n "/api/v1/" lib --glob '!*.g.dart' --glob '!*.freezed.dart'` (139 lines, 34 files incl. out-of-scope);
`rg -n "marketplace|store_visitors|violation_reports|food_safety|food-business|merchant_profile|/view'|vendor_portal|status_syncs|share_url|businessRegistration" lib`;
per-method caller search `rg -n "\bMETHOD\(" lib test`; openapi check by script (normalising {param}) against backend/api/openapi/openapi.yaml.
Envelope: every call reads gateway `{code, data, message, messageZh}` (ApiRequestHelper, validateStatus always true; non-2xx -> `{code: body.code ?? httpStatus, httpStatus}`; network -> code -1).

## Counts
- In-scope distinct (method,path) pairs: 30 (list below). In-scope files holding the literal path: 11
  (seller_repository, session_provider, payuni_status_notifier, shipping_settings_remote_service, product_repository, post_repository, tgos_address_service, chat_repository, chat_picker_remote_service, seller_page_stats_remote_service, dashboard_order_stats_provider). Consumers reading /me fields: ~14 more files.
- Whole lib: 34 files with `/api/v1/` literals (rest = orders, products, coupons, notifications, media ... out of scope).
- Paths NOT in openapi.yaml: none in scope. Every in-scope path+method exists (me, logo, cover, onboarding, payuni/status, status_syncs, dashboard, violations, violations/{id}/appeal GET+POST, certifications*, businessRegistration/{ban}, share_url, address/*, chat/messages POST+PATCH, public/sellers/{id}[/rating|/stats|/products|/forum], forum/{id}/replies). Only "misses" are base-path constants (`/api/v1/app/media`, `/app/logistics`, `/app/sellers/me/procurement`) that get suffixes appended = false positives.
- Spec has but seller-app never calls: me/food-business-registration, me/payuni/merchant_profile, me/store_visitors, GET violations/{id}/appeal, violation_reports, food_safety_reports, /public/marketplace, public/products/{id}/view, vendor_portal. (rg: 0 hits for all of these in lib.)

## Call-site table (file:line | call | feature | response fields read / request sent | reachable? | errors)
All repo paths in lib/features/seller/data/seller_repository.dart (SR) unless stated.

1. SR:100 | POST /app/sellers | onboarding create store (seller_info_screen.dart:202, route /onboard) | send storeName, contactPhone, city, district, addressDetail (description/prize optional, not sent by UI); read data.storeId (seller_info_screen:221) | yes | only code==0 check; else generic snack. Uses isSellerNotFoundCode(9233|404) in the /me pre-check to pick create vs PUT.
2. GET /app/sellers/me (13 call sites). SR:48/54/398/435/452 (getSellerInfo, getMySeller, getUserProfileMap, getSellerInfoOptional, _getSellerCertificationStatus), SR:135 (getUserProfile, called only profile_screen:363), session_provider.dart:243 (fetchAndSyncSeller, direct dio), shipping_settings_remote_service.dart:22 (fetchSellerProfile). All reachable. See "profile reads" below.
3. SR:80 PUT /app/sellers/me (saveSellerInfo) | callers: profile_screen:258 (syncSellerSummary) and :571 (save), seller_info_screen:198 (update), wallet_screen:2060 (address dialog), post_or_website_dialog:52 (websiteUrl), certification screen ~1950 (address sync) | sends: storeName, description, contactPhone, addressDetail, prize, city, district, websiteUrl; profile_screen:258 additionally echoes the whole GET /me map (minus logo*/cover*) plus `status:1`, `email` | reachable | error: only `code != 0` -> res.message toast; no Version sent (no optimistic-lock/9266 handling for seller; 9266 only exists for products). 9240 via global interceptor.
4. SR:220 DELETE /app/sellers/me (deleteAccount = "close store/delete account") | profile_other_options_card.dart:298 | no request body; reads code, messageZh/message | reachable (settings) | success -> logout + go /auth. Failure -> friendlyDeleteAccountError(): messageZh, else strip `[NNNN]` prefix, else generic. 9205 (open orders)/9204 (already deleting) are only in a comment (seller_repository:217) - NO code-specific branch; the dialog text itself says open orders block deletion. Revokes Apple auth first.
5. SR:114 PATCH /app/sellers/me/onboarding | seller_info_screen:309,334 (submit / skip) | no body; result ignored (try/catch, then local finishOnboarding()) | yes | errors swallowed.
6. SR:168 PUT /app/sellers/me/logo | profile_screen:822 | send stagedToken; read data.logoKey | yes | code check only.
7. SR:179 PUT /app/sellers/me/cover | profile_screen:890 | send stagedToken; read data.coverKey | yes | code check only.
8. GET /app/sellers/me/payuni/status | (a) SR:159 getPayuniStatus <- payuni_review_status_screen:133; (b) payuni_status_notifier.dart:260 _fetchPayuniStatus (direct dio, used by wallet_screen:281 fetchStatus + 60s polling) | reads data.payuniStatus (int or string, e.g. "active"), merStatus, payuniMerId, approvalStatus, errorCode, errorDisplayMessage, errorRemedy, retryStep | (b) reachable via wallet_screen; (a) PayuniReviewStatusScreen route `/seller/payuni/status` is registered (app_router:1077) but NO push/go to it found anywhere in lib -> effectively orphan screen (only deep-link/manual) | PayUni errors surfaced only as errorDisplayMessage/errorRemedy/retryStep strings from this payload (no HTTP-code branching); non-2xx returns body or code -1.
9. SR:443 POST /app/sellers/me/payuni/status_syncs (refreshPayuniStatus) | DEAD: zero callers in lib or test.
10. GET /app/sellers/me/dashboard | SR:384 getDashboardStats <- wallet_screen:237 (reads data.salesAmount); dashboard_order_stats_provider.dart:68 (direct apiGet; dashboard tab) reads salesAmount, orderCount, pendingShipmentCount, inProgressCount, completedCount, customerCount, pageViewCount, aftersalesCount, salesAmountGrowth, orderCountGrowth, customerCountGrowth, pageViewCountGrowth, subscriberCount, subscriberCountGrowth | reachable | code!=0 -> empty stats.
11. SR:62 GET /app/sellers/me/violations | seller_appeal_screen:47 | reads data.records[] -> SellerViolation {id, category, severity, description, status, appealed, appealStatus, appealDecisionReason, createdAtMs}; canAppeal = !appealed && status=='enforced' | reachable only via suspension_notice_screen:99 push(SellerRoutes.appeal) (shown when session.isSuspended, redirect_logic:60) | error: message string.
12. SR:71 POST /app/sellers/me/violations/{id}/appeal | seller_appeal_screen:152 | send appealReason | same | code==0 else toast message; comment says duplicate appeal -> 409, no special branch. GET .../appeal is never called.
13. SR:235 POST /app/certifications (createCertificationDraft) | seller_certification_upload_screen:869 | send sellerType ('individual'|'business'); read data.id, status, stepData, currentStep, idCardFrontUrl, idCardBackUrl, bankBookUrl, documentUrls | reachable (wallet_screen:381 push /seller/certification/upload; payuni_review_status_screen:197; page_shop_tab:585 -> /seller/certification route which redirects to /main/wallet) | errors: 401 -> "login expired"; **428** (already forwarded/approved; HTTP 409 body.code 428) -> messageZh/message shown; others -> messageZh/message. 
14. SR:244 GET /app/certifications/{id} (getCertificationDraft) | same screen :841 draft restore | same fields as 13 | reachable | non-0 -> clear stored draft id, POST new. (SR:355 getCertificationApplication duplicates this: DEAD, no caller.)
15. SR:250 GET /app/certifications/{id}/reviews | screen:741 (only when status=='rejected') | reads data[0].rejectionReason, data[0].requiredFields[] | reachable | silent on failure.
16. SR:268 PUT /app/certifications/{id}/progress | screen:657, :1862 | send currentStep, stepData (JSON string: name, birthDate, idNumber, issueDate, issueLocation, issueType, bank*, foreignName, storeDesc, sellerCity/District/Address, *ReferenceId, documentReferenceIds...) | reachable | failures swallowed.
17. SR:349 PATCH /app/certifications/{id} (submitCertification, KYC submit) | screen:1877 | send snake_case: real_name, id_card_no, id_card_*_staged_token, birth_date, id_card_issue_*, bank_code/name/account, bank_book_staged_token, english_name, document_staged_tokens, company_type/name, tax_id, company_register_date, company_city/district/address_detail/email/phone, product_type, is_processed_food, food_business_reg_no | reachable | code==0 success; **409 + message contains "請先發布一則動態"** (screen:2048) special flow (post_or_website_dialog); else messageZh/message.
18. SR:367 DELETE /app/certifications/{id} (cancelCertification) | DEAD, no caller.
19. SR:259 GET /app/businessRegistration/{ban} | screen:957 (auto lookup on 8-digit tax id) | reads businessNm, businessAddress, businessSetupDate, businessType (raw or in data) | reachable | httpStatus 404 -> inline "not found"; else silent.
20. GET /app/share_url?type&id | product_repository.dart:156 (<- page_shop_tab:1733), post_repository.dart:566 (<- page_home_feed_screen:247) | reads data.title, description, imageUrl, url | reachable | null on failure.
21. GET /app/address/suggestions (tgos_address_service.dart:95; query, county, district, limit) and GET /app/address/validate (:129; address) | seller_info_screen:71, profile_address_edit_dialog:132, shipping settings address_autocomplete_field | reads data[] list / AddressValidation.fromJson(data) incl. city, district | reachable | code!=0 -> [].
22. POST /app/chat/messages (chat_repository.dart:746) send {toUserId, content, type, extra}; PATCH same path :994 {messageIds} mark read | chat | reachable | 9202 BlockedMessageNotAllowed, 9210 ModerationRejected via ChatSendException.code, reads messageZh/message.
23. GET /public/sellers/{id} | SR:50/226/402/437 (getSellerInfo, getSeller, getUserProfileMap, getSellerInfoOptional for non-self) <- page_screen:281, profile_screen:194/444, dashboard_user_profile_provider:87, page_echo_tab:315, product_preview (seller name/avatar) | reads logoKey, logoUrl, storeName/sellerName, prize, description, coverUrl, address/city/district, overallCertificationStatus, storeCode | reachable.
24. GET /public/sellers/{id}/rating (SR:374 <- page_header:800, product_preview:398) | reads data.rating, data.reviewCount.
25. GET /public/sellers/{id}/stats (SR:379 <- profile_screen:424; seller_page_stats_remote_service:17 <- seller_page_stats_repository) | reads data.followers (only).
26. GET /public/sellers/{id}/products (chat_picker_remote_service:28) ; GET /public/sellers/{id}/forum (post_repository:510) ; GET /public/sellers/forum/{id}/replies (post_repository:524) | reachable | out of the Store-field question.
27. Not called at all: me/food-business-registration, me/payuni/merchant_profile, me/store_visitors, /violation_reports, /public/food_safety_reports, /public/marketplace, /public/products/{id}/view, vendor_portal.

Out of scope /app/sellers/me/*, noted only: customers (+ /{id}, meta, tags, orders, coupons, lookup), tags, notification-preferences, assistant/*, procurement, reviews (order_repository:240), posts-related; plus /app/seller/after_sales.

## Profile reads of GET /app/sellers/me (data.*)
session_provider._fetchAndSyncSeller (session_model has no fromJson of /me; fields copied into SessionModel):
- storeId (REQUIRED; missing/<=0 -> sync fails), storeName (fallback sellerName), address (!), contactPhone (fallback phone), prize, payuniMerId, lifecycleStatus, suspensionReasonCode, needsOnboarding (all via resolveNeedsOnboarding: `data.needsOnboarding == true`; 9233 or 404 -> needsOnboarding=true).
- SessionModel.isSuspended = lifecycleStatus.startsWith('suspended'); values assumed: active / suspended / suspended_governance / closed. Suspension reason codes assumed POLICY_VIOLATION / REPEATED_WARNING_IGNORED / OTHER.
Other /me readers: overallCertificationStatus ('approved' gate; profile_screen:406, wallet_screen, page_screen:178, shop_tab_repository:50, seller_certification_gate:20 used by dashboard_screen product-create and shipping gate), certificationStatus, paymentStatus, realName, idCardNo (seller_repository _getSellerCertificationStatus; wallet reads `result['paymentStatus']` at top level of a {code,data} map so it is always null - latent bug), storeName, city, district, addressDetail, contactPerson, contactPhone/phone, email, description, prize, coverKey/coverUrl, logoKey/logoUrl, storeCode, id (page_screen:~300 `remote['id']` sellerId correction), location/address (page_screen:_resolvedAddress), sellerName, postalCode (shipping_settings_repository:95), websiteUrl (written).

## Fields the UI reads/sends that do NOT exist on the new seller module (Store: StoreName, Description, ContactPhone, TownCode, AddressDetail, Prize, WebsiteURL, SEO*, IsProcessedFood, FoodBusinessRegNo, OnboardingCompleted, StoreID, StoreCode, Version, Status ACTIVE/PENDING_DELETION/CLOSED, Suspension, LogoKey, CoverKey)
Read from /me (or public seller):
- lifecycleStatus (string active/suspended/suspended_governance/closed) and suspensionReasonCode -> new model has Status + Suspension object; closed/pending-deletion not understood. The 9240 interceptor path also depends on this: markSuspended() then /me refetch.
- address (composed string; session, wallet ensureAddressSet, shipping settings, page_screen), city, district (names; new has TownCode only; profile_screen:273-275 split address UI, shipping settings home address prefill, page_screen:450), postalCode (shipping_settings_repository:95)
- email (profile_screen:291/232/238 also WRITTEN in PUT; wallet_screen:167 email for verification flow)
- contactPerson (always empty per comment), phone (account phone), sellerName, location
- followers (public stats; profile_screen fans count, seller_page_stats_repository)
- rating, reviewCount (public rating summary; page_header, product_preview, page_shop_tab) - no rating on Store
- overallCertificationStatus (approved|...), certificationStatus, paymentStatus, realName, idCardNo -> certification/KYC state not on Store; gates product create, shipping settings, wallet, shop tab.
- payuniMerId (session; actually sourced from payuni/status per comment)
- needsOnboarding (Store has OnboardingCompleted: invert), storeId int (StoreID ok)
- coverUrl/logoUrl (display URLs; Store has keys only; code falls back to key -> buildImageUrl so tolerable)
- id (page_screen `remote['id']`; storeId expected instead)
- birthday/gender mentioned in PUT comments but not sent by UI.
Not read anywhere (so no breakage): membership plan, invite codes, email-verified etc. -> rg found no membershipPlan/inviteCode/referral key reads in lib.
Sent in writes that the new module lacks: POST /app/sellers and PUT /me send city + district (names), PUT sends email and status:1 (profile_screen:233); profile PUT echoes whole /me map. No Version sent anywhere for seller PUT.

## Error-handling summary
- 9240 (409): global Dio interceptor dio_provider.dart:~201-225 (onResponse + onError; also bytes/String bodies) -> session.markSuspended(), then fire-and-forget fetchAndSyncSeller(); router redirect -> /seller/suspension-notice (redirect_logic:60); appeal route allowed. seller_suspension_codes.dart: 9240 operation blocked, 9241 product unavailable (no caller).
- 9233 / bare 404 on /me -> needsOnboarding=true (session_provider:255; seller_info_screen sellerProfileExists picks POST vs PUT).
- 9266: product upload only (product_upload_service, product_setup_spec_screen); not seller.
- 428: certification draft create. 409+"請先發布一則動態": KYC submit.
- 9205/9204 on DELETE /me: documented only; generic messageZh path.
- 9202/9210: chat send. 9250-9253: AI image (out of scope). Other 92xx: none seller-related.
- PayUni: only string fields from payuni/status (errorDisplayMessage, errorRemedy, retryStep, errorCode); notifier maps merStatus 91/92/8 as failure from silent push.
