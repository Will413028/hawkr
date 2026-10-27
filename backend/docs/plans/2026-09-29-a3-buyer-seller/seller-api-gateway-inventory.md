# A3b step 8 — gateway route inventory at HEAD (read-only)

Snapshot: `174adc432` (HEAD, 2026-10-03). Paths: gateway files relative to `backend/gateway/internal/`; module files relative to `backend/internal/modules/`. The old inventory's snapshot `bdf13a034` no longer resolves (`git log -1 bdf13a034` → unknown revision); the inventory itself was committed in `50a838f08` (2026-09-29), so diffs below are `git diff 50a838f08 HEAD`.

## 0. Summary

- **Route set reaching seller-service is unchanged: 92 routes** (16 public, 29 app, 47 admin), same as old §4.1. `git diff 50a838f08 HEAD -- backend/gateway/internal/router/routes_*.go` only removes buyer/ai routes (A3a step 5); no seller route added or removed. 88 are in `api/openapi/openapi.yaml` (536 ops now, was 569); the 4 delegated storefront routes are not.
- Plus **9 adjacent routes** listed for completeness: 6 vendor-portal RFQ routes (ai handlers, seller only via `middleware.VendorPortalSession`), 2 prospect media upload-session routes (media only), `PUT /app/buyers/me/seller_binding` (account only). Counted separately, not in the 92.
- Disposition counts (92, per-row column in §1): **a = 70** (public 9, app 24, admin 37; includes A10 and A17 and P12 tentatively, see Q11/Q12/Q23), **b = 10**, **c = 12**. After merging routes that collapse into one RPC, step 8 is ≈ 64–69 RPCs (§6).
- Module coverage after 7a–7d: of the 70 `a` routes, **63 have an implementing public API**; **7 are partly or fully MISSING**: P1 (batch store display read), P2 (lookup by `store_code`), A12 (crm/order legacy clients), A17 (GCIS), D2 (admin store list), D3 (admin store stats), D4 (KYC by store).
- Commands: route→handler map by a helper script (not kept; regex over `handler/*.go` for `.Seller.<RPC>(`, enclosing func); operationIds by parsing `paths:` of openapi.yaml, `/api/v1` + `{}`-normalized match); client counts by `rg -l --pcre2` of the full path, params as `[^/'"\`?\s]+`, must end at quote/`?`, in `website/`, `admin/`, `buyer-app/lib`, `seller-app/lib`, excluding tests/generated) plus `rg -l -F <segment>` spot checks for namespace-less callers. Counts are method-agnostic upper bounds; `w/a/b/s` = website/admin/buyer-app/seller-app files.

## 1. Route table (HEAD)

Legend: disposition `a` = new Connect RPC (service named), `b` = delete, `c` = other module/step. "client?" = cannot confirm a client caller with the cheap search. Module refs: `seller.go:257 Apply` = `internal/modules/seller/seller.go:257`.

### 1.1 Public (16)

| # | Method path (operationId) · handler | seller RPC · gateway orchestration | Module API at HEAD | Disposition | Clients w/a/b/s |
|---|---|---|---|---|---|
| P1 | GET /public/sellers/recommended (getRecommendedSellers) · `handler/recommended_sellers.go:21`, route `routes_seller.go:18` | GetRecommendedSellers | `discovery/discovery.go:76 RecommendedSellers(limit)`; display fields (name, logo, followers) MISSING a batch store read in seller (see §3) | a `DiscoveryPublicService.ListRecommendedSellers` (display assembled in `internal/httpapi/discovery` with `buyer.go:398 SubscriberCounts`, rating from the snapshot) | 5/0/0/0 |
| P2 | GET /public/sellers/:id (getSellerInfo) · `handler/seller.go:259` | GetSeller + fire-and-forget RecordStoreVisit (resolves visitor `BuyerID.CurrentBuyerID`, skips owner) | `seller.go:267 Store(storeID)`, `seller.go:274 ExposableStores`; MISSING: lookup by `store_code` (storefront URLs use it, `resolveStorefrontSellerID` comment `handler/seller.go:297`); rating via legacy `reviewservice.StoreRatingStats` (`internal/legacy/reviewservice/reviewservice.go:44`) | a `SellerPublicService.GetStore`; RecordStoreVisit part b (plan 不搬 `seller_store_visitors`) | 14/0/7/2 |
| P3 | GET /public/sellers/:id/stats (getSellerStats) · `handler/seller.go:389` | GetSellerStats (seller-service fills rating from `sellers.rating`, followers from buyer, review_count from review; totals unused) | composition only: `buyer.go:398 SubscriberCounts`, legacy `reviewservice.StoreRatingStats` | a fold into `SellerPublicService.GetStore` (or `GetStoreStats`); composed in `internal/httpapi/seller` | 3/0/2/2 |
| P4 | GET /public/sellers/:id/rating (getSellerRating) · `handler/seller.go:352` | GetSellerStats | same as P3 | a fold into P3 (same data) | 0/0/3/2 |
| P5 | GET /public/sellers/:id/products (getSellerProducts) · `handler/seller.go:809` | GetSeller only as id/store_code resolver; then `Product.GetPublicSellerProductListWithHashtags` | n/a | c catalog (B); stays in gateway, needs legacy `GetSeller` (step 9) | 3/0/5/1 |
| P6 | GET /public/sellers/:id/reviews (getPublicSellerReviews) · `handler/order_review.go:329` | GetSeller resolver | n/a | c review (A6); legacy `GetSeller` (step 9) | 1/0/0/0 |
| P7 | GET /public/sellers/:id/reviews/stats (getPublicSellerReviewStats) · `handler/order_review.go:392` | GetSeller resolver | n/a | c review (A6); legacy `GetSeller` | 2/0/0/0 |
| P8 | GET /public/sellers/:id/posts (getSellerPosts) · `handler/post.go:71` | GetSeller resolver + GetSellerBatch enrichment | n/a | c social (A7); legacy `GetSeller`, `GetSellerBatch` | 0/0/2/2 |
| P9 | GET /public/posts (getLatestPosts) · `handler/post.go:30` | GetSellerBatch enrichment (`enrichPostsWithSellerInfo`, `handler/marketplace.go:227`) | n/a | c social (A7); legacy `GetSellerBatch` | 7/0/1/1 |
| P10 | GET /public/marketplace (getMarketplace) · `handler/marketplace.go:29` | GetRecommendedSellers + GetSellerBatch; also `Product.GetPublicProductList`, `Social.GetPostList` | `discovery.go:76` for the ranking part only | c/a split: ranking section → P1's Connect RPC; posts + products stay gateway (A7/B). Design question Q10 | 3/0/4/1 |
| P11 | POST /public/products/:id/view (recordProductView) · `handler/product_view.go:18`, route `routes_product.go:27` | RecordProductView (no auth, no limit, errors swallowed) | `seller/views.go:28 RecordProductView(productID)` (store resolved by the rollup job) | a `SellerPublicService.RecordProductView` + IP limit at entry; guests only (D14) | website 1 (`features/product/ProductViewTracker.tsx:14`, namespace-less) |
| P12 | POST /public/food_safety_reports (submitFoodSafetyReport) · `handler/governance.go:365` | CreateFoodSafetyReport; `DualWindowRateLimit` 5/10 min + 10/h per IP (`routes_governance.go`) | `seller/governance.go:171 SubmitReport` (anonymous contact) | a `…ReportPublicService.SubmitReport` + dual-window IP limit at entry — or defer (plan: 開放哪些在步驟 8，client 入口另外取得同意) | client? (0 in web/apps; admin only displays) |
| P13 | GET /public/sellers/:id/violations/public (getPublicSellerViolations) · `handler/governance.go:599` | GetPublicSellerViolations | none (by design) | b — ADR `2026-10-18-seller-governance-consolidated` 公開違規列表刪除 | 0/0/0/0 |
| P14 | GET /public/vendor_portal/invites/preview (vendorPortalInvitePreview) · `handler/vendor_portal.go:236` | PreviewVendorPortalInvite; rate limit 20/5 min keyed `sha256(secret):ip` | `agent/agent.go:134 PreviewInvite` | a `hawkr.agent.v1alpha1.VendorPortalPublicService.PreviewInvite` + IP limit | 1/0/0/0 |
| P15 | POST /public/vendor_portal/invite_exchanges (vendorPortalInviteExchange) · `handler/vendor_portal.go:192` | ExchangeVendorPortalInvite; sets `hk_vendor_portal` cookie | `agent.go:129 ExchangeInvite` (returns `AbsoluteExpiresAt`) | a `VendorPortalPublicService.ExchangeInvite` (cookie set at entry) | 1/0/0/0 |
| P16 | POST /public/vendor_portal/step_up/google (vendorPortalStepUpGoogle) · `handler/vendor_portal_step_up.go:48` | session middleware → `Account.VerifyGoogleIdToken` (monolith legacy accountservice) → StepUpVendorPortalSession | `identity/identity.go:241 VerifyGoogle` + `agent.go:145 VerifySession(token, method, email)` | a `VendorPortalPublicService.VerifySession` (Google method) | 1/0/0/0 |

Adjacent (not in the 92): 6 RFQ routes `GET /public/vendor_portal/rfqs/:id` (vendorPortalRFQ), `GET|PUT …/response_draft`, `POST …/response_submissions`, `POST …/seller_responses/:id/withdrawals`, `POST …/declines` (`routes_vendor_portal.go`) — handlers are ai-service (A9); only `middleware.VendorPortalSession` (`middleware/vendor_portal_session.go:32`) calls `ValidateVendorPortalSession`, and the 3 writes add `RequireVendorPortalGoogleVerified`. Disposition c (A9) with step-8 work: session validation must move off seller-service (`agent.go:139 ValidateSession`) and the 3 writes must forward the verification evidence to ai (plan step 8; ai.proto + ai-service change). Clients: website `lib/vendor-portal/repo.ts` has submissions; withdrawals/declines client?.

### 1.2 App (29)

| # | Method path (operationId) · handler | seller RPC · gateway orchestration | Module API at HEAD | Disposition | Clients w/a/b/s |
|---|---|---|---|---|---|
| A1 | POST /app/sellers (applySeller) · `handler/seller.go:729` | CreateSeller; no capability guard (it grants the role) | `seller.go:257 Apply` | a `SellerAppService.ApplyStore` (exempt from capability check: no store yet) | 1/0/0/2 |
| A2 | GET /app/sellers/me (getMySellerInfo) · `handler/seller.go:429` | GetSeller + GetCertificationDraft (draft id) | `seller.go:262 CurrentStore` (+ `kyc.go:110 MyKYC` for flow) | a `SellerAppService.GetMyStore` (CAP_ACCOUNT_SELF_READ, allowed when suspended; returns suspension + capabilities so clients drop 9240 heuristics) | 22/0/0/12 (shared path) |
| A3 | PUT /app/sellers/me (updateMySeller) · `handler/seller.go:474` | UpdateSeller; `sellerHandle(CapStorefrontWrite)` | `seller.go:286 UpdateProfile` (field mask + `ExpectedVersion`) | a `SellerAppService.UpdateMyStore` (field mask, etag) | shared |
| A4 | PUT /app/sellers/me/food-business-registration (updateMyFoodBusinessRegistration) · `handler/seller.go:543` | UpdateSellerFoodRegistration; CapStorefrontWrite | `UpdateProfile` (`FoodRegistration` field) | a merged into A3 (CARRYOVER line 29) | 1/0/0/0 |
| A5 | DELETE /app/sellers/me (deleteSellerAccount) · `handler/seller_delete.go:21` | DeleteSellerAccount(IsAdmin=false); 9204/9205 | `seller.go:321 CloseCurrent` (open-order port, fail-closed) | a `SellerAppService.CloseMyStore` (CAP_ACCOUNT_CLOSE, allowed when suspended); `ErrorReason` for open orders if client branches | shared |
| A6 | PUT /app/sellers/me/logo (updateMyLogo) · `handler/seller.go:609` | UpdateLogo; CapStorefrontWrite | `seller.go:291 UpdateLogo` (staged token, version) | a `SellerAppService.UpdateMyStoreLogo` (or one `UpdateMyStoreImage` with slot) | 0/0/0/1 |
| A7 | PUT /app/sellers/me/cover (updateMyCover) · `handler/seller.go:656` | UpdateCover | `seller.go:296 UpdateCover` | a (with A6) | 0/0/0/1 |
| A8 | PATCH /app/sellers/me/onboarding (completeOnboarding) · `handler/seller.go:700` | CompleteOnboarding | `UpdateProfile.CompleteOnboarding` | a merged into A3 | 1/0/0/1 |
| A9 | GET /app/sellers/me/payuni/status (getPayuniStatus) · `handler/seller.go:955`; `PrivateNoStore` | GetPayuniStatus | `seller/payuni.go:135 MyPayUniRegistration` | a `SellerAppService.GetMyPayUni` (no-store) | 5/0/0/4 |
| A10 | POST /app/sellers/me/payuni/status_syncs (refreshPayuniStatus) · `handler/seller.go:982` | RefreshPayuniStatus (old: 30-min min interval) | `payuni.go:150 RefreshPayUniStatus(storeID)` — no interval (CARRYOVER line 117: 賣家自己刷新的入口屬步驟 8) | a or b — Q11 | 1/0/0/1 |
| A11 | PATCH /app/sellers/me/payuni/merchant_profile (patchPayuniMerchantProfile) · `handler/seller.go:1111` | UpdateMerchantProfile; CapStorefrontWrite | `payuni.go:130 UpdatePayUniProfile` | a `SellerAppService.UpdateMyPayUniProfile` | 2/0/0/0 |
| A12 | GET /app/sellers/me/dashboard (getSellerDashboard) · `seller_dashboard.go:48`; CapMarketing | GetProductViewStats; plus Crm.GetSellerStats, Order.GetSellerOrderStatusCounts, Buyer.GetSubscriberDashboardStats (now monolith legacy), Crm/Order GetSellerDashboardGrowth (6 goroutines, best-effort) | `views.go:39 ProductViewStats`, `buyer.go:403 SubscriberDashboardStats`; MISSING legacy clients for crm stats/growth and order status counts/growth | a `SellerAppService.GetDashboard` composed in `internal/httpapi/seller`, or split — Q9 | 5/0/0/2 |
| A13 | GET /app/sellers/me/store_visitors (getStoreVisitors) · `handler/seller.go:1268` | GetStoreVisitors | none | b — plan 不搬 `seller_store_visitors` and its read route | 0/0/0/0 |
| A14 | GET /app/share_url (getShareURL) · `handler/share.go:70,160` | GetSeller for `store_code` (post & seller branches); Social, Product calls | n/a | c stays in gateway (cross-cutting share links); legacy `GetSeller` (step 9) | 0/0/1/2 (+namespace-less `share_url` in 3 Dart repos) |
| A15 | GET /app/address/suggestions (addressSuggestions) · `handler/address_geocode.go:49` | SuggestAddress (TGOS); buyer branch now 400 | none | b — decision 9 (TGOS removed) | 1/0/0/1 |
| A16 | GET /app/address/validate (addressValidate) · `handler/address_geocode.go:109` | ValidateAddress | none | b — decision 9 | 1/0/0/1 |
| A17 | GET /app/businessRegistration/:ban (businessRegistrationLookup) · `handler/business_registration.go:37` | LookupBusinessRegistration (GCIS `data.gcis.nat.gov.tw`) | MISSING — CARRYOVER line 82 says "GCIS 仍去向 7c 的後續階段", nothing implemented | a `SellerAppService.LookupBusinessRegistration` or b — Q12 | 1/0/0/1 |
| A18 | POST /app/certifications (createCertificationDraft) · `handler/certification.go:69`; `PrivateNoStore` | CreateCertificationDraft | `kyc.go:95 SaveKYCDraft` (ExpectedVersion 0 creates) | a `SellerAppService.SaveKYCDraft` (no-store) | 1/0/0/1 |
| A19 | GET /app/certifications/:id (getCertificationApplication) · `handler/certification.go:414` | GetCertificationApplication; gateway ownership check `verifyCertificationOwnership` (`certification.go:109`) | `kyc.go:110 MyKYC(accountID)` (account-scoped, no app id) | a `SellerAppService.GetMyKYC` | 1/0/0/1 |
| A20 | GET /app/certifications/:id/reviews (getMyCertificationReviews) · `handler/certification.go:454` | GetCertificationApplication + GetCertificationReviews | `MyKYC` → `KYCApplication.Rejection` (latest only) | a merged into A19 | 1/0/0/1 |
| A21 | PUT /app/certifications/:id/progress (saveCertificationProgress) · `handler/certification.go:238` | Get + SaveCertificationProgress | `SaveKYCDraft` | a merged into A18 | 1/0/0/1 |
| A22 | PATCH /app/certifications/:id (submitCertificationApplication) · `handler/certification.go:341` | Get + SubmitCertificationApplication | `kyc.go:100 SubmitKYC` | a `SellerAppService.SubmitKYC` + per-account rate limit (ADR `2026-10-02-seller-kyc-payuni-in-monolith`, seller `AGENTS.md:106`) | 1/0/0/1 |
| A23 | DELETE /app/certifications/:id (cancelCertification) · `handler/certification.go:379` | Get + CancelCertification | `kyc.go:105 CancelKYC` | a `SellerAppService.CancelKYC` | 1/0/0/1 |
| A24 | POST /app/violation_reports (submitViolationReport) · `handler/governance.go:274` | CreateViolationReport | `governance.go:171 SubmitReport` (buyer reporter) | a `…ReportAppService.SubmitReport` or defer (no client) — Q2/Q23 | client? 0 |
| A25 | GET /app/violation_reports/me (listMyViolationReports) · `handler/governance.go:318` | ListMyViolationReports | `governance.go:216 BuyerReports` | a with A24 or defer | client? 0 |
| A26 | GET /app/sellers/me/violations (sellerListMyViolations) · `handler/governance.go:484` | SellerListMyViolations (no `sellerHandle`) | `governance.go:272 AccountViolations` | a `SellerAppService.ListMyViolations` (must NOT use ActingStore/CAP gate; seller `AGENTS.md:60`) | 2/0/0/3 |
| A27 | POST /app/sellers/me/violations/:id/appeal (sellerSubmitAppeal) · `handler/governance.go:525` | SellerSubmitAppeal | `governance.go:247 SubmitAppeal` | a `SellerAppService.SubmitAppeal` (same no-gate rule) | 1/0/0/1 |
| A28 | GET /app/sellers/me/violations/:id/appeal (sellerGetAppeal) · `handler/governance.go:565` | SellerGetAppeal | `governance.go:262 AccountViolation` (appeal embedded) | a `SellerAppService.GetMyViolation` | 1/0/0/1 |
| A29 | POST /app/chat/messages (sendChatMessage) · `handler/chat.go:143` (`resolveChatTargetID`) | GetSeller resolver; `injectSellerSuspension` (`routes_chat.go:26`) | n/a | c im (A?); legacy `GetSeller` + entry suspension header (step 9) | 0/0/1/1 |

Adjacent: `PUT /app/buyers/me/seller_binding` (bindSeller) · `handler/seller_binding.go:11`, route `routes_buyer.go:15` → monolith legacy `Account.BindSeller` only. b (plan 範圍; website `lib/seller/repo.ts:391 bindSellerRole` removed in step 10). Clients 1/0/0/0.

### 1.3 Admin (47)

Admin auth for all: `admin.Use(JWTAuth + requireAdmin)`; Connect `*AdminService` audience covers it (`internal/httpapi/rpc/rpc.go:43 AudienceOf`).

| # | Method path (operationId) · handler | seller RPC · gateway orchestration | Module API at HEAD | Disposition | Clients (admin) |
|---|---|---|---|---|---|
| D1 | DELETE /admin/accounts/:id (deleteAccount) · `handler/admin_account.go:34` | DeleteSellerAccount(IsAdmin, OverrideActiveOrders=true) → Buyer.DeleteBuyerAccount (now monolith legacy = `accountdeletion.Request`) → `Account.SoftDeleteAccount` fallback | `accountdeletion/accountdeletion.go:88 Request` (requester `Admin`) | a `IdentityAdminService.DeleteAccount` (mirrors `IdentityAppService.DeleteAccount`, `internal/httpapi/identity/app.go:38`); legacy `DeleteBuyerAccount` deleted with it | 2 |
| D1' | (new) admin "刪賣家角色" | — | `seller.go:321 CloseCurrent(accountID)` (rejects with open orders) | a `SellerAdminService.CloseStore` — Q15 | — |
| D2 | GET /admin/sellers (listAdminSellers) · `handler/admin_seller.go:119` | ListSellersForAdmin | MISSING (no admin store list/search in seller) | a `SellerAdminService.ListStores` — needs module API | 5 |
| D3 | GET /admin/sellers/stats (getAdminSellerStats) · `handler/admin_seller.go:84` | GetSellerStatsForAdmin | MISSING (no counts API) | a `SellerAdminService.GetStoreStats` (or drop if unused) — needs module API | 3 |
| D4 | GET /admin/sellers/:store_id (getAdminSellerProfile) · `handler/admin_seller.go:243` | GetSeller + GetCertificationApplicationForAdmin + GetPayuniStatus + `Social.GetPostList` (KYC precondition count) | `Store`, `payuni.go:140 PayUniRegistrationOf`; KYC-by-store MISSING (`kyc.go:120 AdminKYCApplication` takes application UUID; `KYCListInput` has no store filter); social count obsolete (precondition deleted 2026-10-03) | a `SellerAdminService.GetStore` (+ active suspension, KYC summary) | 4 |
| D5 | PATCH /admin/sellers/:store_id/seo (updateSellerSEO) · `handler/admin_seller.go:387` | ResolveSellerAccountID + UpdateSellerSEO | `UpdateProfile` (SEO fields) | a `SellerAdminService.UpdateStore` with mask limited to SEO (CARRYOVER line 29) | 1 |
| D6 | POST /admin/sellers/:store_id/membership (assignSellerMembershipPlan) · `admin_seller_membership.go:59` | AssignSellerMembershipPlan | none | b — decision 7 | 0 |
| D7 | GET /admin/sellers/:store_id/membership (getSellerMembershipPlan) · `admin_seller_membership.go:100` | GetSellerMembershipPlan | none | b — decision 7 | 0 |
| D8 | GET /admin/seller_membership_plans (listSellerMembershipPlans) · `admin_seller_membership.go:158` | ListSellerMembershipPlans | none | b — decision 7 | 1 |
| D9 | PATCH /admin/seller_membership_plans/:plan_code (updateSellerMembershipPlan) · `admin_seller_membership.go:198` | UpdateSellerMembershipPlan | none | b — decision 7 | 1 |
| D10 | GET /admin/certifications (listAdminCertifications) · `handler/certification.go:643` | ListCertificationsForAdmin | `kyc.go:115 AdminKYCApplications` (status, real-name ILIKE, keyset) | a `SellerAdminService.ListKYCApplications` | 1 |
| D11 | GET /admin/certifications/:id (getAdminCertificationDetail) · `handler/certification.go:760` | GetCertificationApplicationForAdmin + GetSeller + `Social.GetPostList` | `kyc.go:120 AdminKYCApplication` (reviews, reveals, bank flag) | a `SellerAdminService.GetKYCApplication` | 1 |
| D12 | GET /admin/certifications/:id/audit-logs (getAdminCertificationReviews) · `handler/certification.go:854` | GetCertificationReviews | `KYCAdminApplication.Reviews` | a merged into D11 | 1 |
| D13 | PATCH /admin/certifications/:id (reviewCertification) · `handler/certification.go:521` | Approve/RejectCertification | `kyc.go:130 ApproveKYC`, `kyc.go:135 RejectKYC` | a `ApproveKYC` + `RejectKYC` (2 RPCs) | 1 |
| D14 | GET /admin/seller_prospects (listAdminSellerProspects) · `admin_seller_prospect.go:347` | AdminListSellerProspects | `prospect.go:164 Prospects` | a `SellerProspectAdminService.ListProspects` | 2 (BASE const in `admin/src/services/seller-prospects.ts:19`) |
| D15 | POST /admin/seller_prospects (createAdminSellerProspect) · `admin_seller_prospect.go:422` | CreateSellerProspect | `prospect.go:149 CreateProspect` | a `CreateProspect` | via BASE |
| D16 | POST /admin/seller_prospects/preview (previewAdminSellerProspect) · same helper | CreateSellerProspect (dry run) | `prospect.go:154 PreviewProspect` | a `PreviewProspect` | via BASE (`seller-prospects.ts:148`) |
| D17 | GET /admin/seller_prospects/duplicates (listAdminProspectDuplicates) · `:478` | AdminListDuplicateProspects | `prospect.go:195 DuplicateProspects` | a `ListDuplicateProspects` | via BASE |
| D18 | GET /admin/seller_prospects/reconciliation (listAdminProspectTaxIDMatches) · `:515` | AdminListProspectTaxIdMatches | `prospect.go:200 ProspectTaxIDMatches` | a `ListProspectTaxIDMatches` | via BASE |
| D19 | POST /admin/seller_prospects/merge (mergeAdminSellerProspects) · `:556` | AdminMergeSellerProspects | `prospect.go:185 MergeProspects` | a `MergeProspects` | via BASE |
| D20 | GET /admin/seller_prospects/:id (getAdminSellerProspect) · `:593` | AdminGetSellerProspect | `prospect.go:159 Prospect` | a `GetProspect` | via BASE |
| D21 | PUT /admin/seller_prospects/:id (updateAdminSellerProspect) · `:629` | AdminUpdateSellerProspect | `prospect.go:170 UpdateProspect` | a `UpdateProspect` | via BASE |
| D22 | DELETE /admin/seller_prospects/:id (archiveAdminSellerProspect) · `:713` | AdminArchiveSellerProspect | `prospect.go:175 ArchiveProspect` | a `ArchiveProspect` | via BASE |
| D23 | POST /admin/seller_prospects/:id/restore (restoreAdminSellerProspect) · `:713` | AdminArchiveSellerProspect(false) | `prospect.go:180 RestoreProspect` | a `RestoreProspect` | via BASE |
| D24 | POST /admin/seller_prospects/:id/convert (convertAdminSellerProspect) · `:748` | AdminConvertSellerProspect | `prospect.go:190 ConvertProspect` | a `ConvertProspect` | via BASE |
| D25 | PUT /admin/seller_prospects/:id/media (updateAdminProspectMedia) · `:967` | AdminUpdateSellerProspectMedia | `prospect.go:216 UpdateProspectLogo`, `:221 UpdateProspectCover` | a `UpdateProspectImages` (or 2 RPCs) | `seller-prospect-media.ts:58` |
| D26 | POST /admin/seller_prospects/:id/invites (createAdminProspectInvite) · `:789` | AdminCreateProspectInvite | none | b — 7c 邀請子系統整套刪除 | via BASE (panel removed step 10) |
| D27 | DELETE /admin/seller_prospects/:id/invites/:invite_id (revokeAdminProspectInvite) · `:831` | AdminRevokeProspectInvite | none | b — same | via BASE |
| D28 | GET /admin/violation_reports (adminListViolationReports) · `handler/governance.go:633` | AdminListViolationReports (merged cross-table) | `governance.go:211 AdminReports` | a `SellerGovernanceAdminService.ListReports` | 2 (+4 namespace-less) |
| D29 | PATCH /admin/seller_violation_reports/:id (adminPatchSellerViolationReportStatus) · `:671` | AdminPatch…Status | `governance.go:181 StartInvestigation`, `:186 ResolveReport` | a `StartInvestigation` + `ResolveReport` (status PATCH split into commands) | 1 |
| D30 | PATCH /admin/food_safety_reports/:id (adminPatchFoodSafetyReportStatus) · `:710` | AdminPatchFoodSafetyReportStatus | same as D29 (reports unified) | a merged into D29 | 1 |
| D31 | POST /admin/food_safety_reports/:id/ack (adminAcknowledgeFoodSafetyReport) · `:746` | AdminAcknowledgeFoodSafetyReport | `governance.go:176 AcknowledgeReport` (sets severity) | a `AcknowledgeReport` | 1 |
| D32 | GET /admin/food_safety_reports/:id/fda_export (adminGenerateFDAReport) · `:781` | AdminGenerateFDAReport (server Markdown) | `governance.go:206 FDAExport` (structured) | a `GetFDAExport` — format Q22 | 1 |
| D33 | POST /admin/food_safety_reports/:id/mark_fda_reported (adminMarkFDAReported) · `:818` | AdminMarkFDAReported | `governance.go:191 RecordFDAReferral` | a `RecordFDAReferral` | 1 |
| D34 | POST /admin/food_safety_reports/:id/notify_seller (adminNotifySeller) · `:861` | AdminNotifySeller | `governance.go:196 RequestCooperation` | a `RequestCooperation` | 1 |
| D35 | POST /admin/violations (adminCreateViolation) · `:912` | AdminCreateViolation | `governance.go:232 CreateViolation` | a `CreateViolation` | 4 |
| D36 | GET /admin/violations/:id (adminGetViolation) · `:958` | AdminGetViolation | `governance.go:257 Violation` | a `GetViolation` | 4 |
| D37 | PATCH /admin/violations/:id (adminUpdateViolation) · `:994` | AdminUpdateViolation | `governance.go:237 UpdateViolation` | a `UpdateViolation` | 4 |
| D38 | POST /admin/violations/:id/revoke (adminRevokeViolation) · `:1037` | AdminRevokeViolation | `governance.go:242 RevokeViolation` | a `RevokeViolation` | 1 |
| D39 | POST /admin/violations/:id/appeal/decide (adminDecideAppeal) · `:1079` | AdminDecideAppeal | `governance.go:252 DecideAppeal` | a `DecideAppeal` | 1 |
| D40 | POST /admin/sellers/:store_id/suspensions (suspendSellerForAdmin) · `admin_seller_suspension.go:86` | ResolveSellerAccountID + SuspendSellerForAdmin | `seller.go:301 Suspend` | a `SellerAdminService.SuspendStore` | 2 |
| D41 | GET /admin/sellers/:store_id/suspensions (listSellerSuspensionsForAdmin) · `:166` | Resolve + ListSellerSuspensionsForAdmin | `seller.go:316 Suspensions` | a `ListSuspensions` | 2 |
| D42 | POST /admin/sellers/:store_id/suspensions/:id/lift (liftSellerSuspensionForAdmin) · `:127` | Resolve + LiftSellerSuspensionForAdmin | `seller.go:306 Lift` | a `LiftSuspension` | 1 |
| D43 | GET /admin/sellers/:store_id/suspension (getActiveSellerSuspensionForAdmin) · `:196` | Resolve + GetActiveSellerSuspensionForAdmin | `seller.go:311 ActiveSuspension` | a merged into D4 `GetStore` (or keep separate) | 1 |
| D44 | GET /admin/sellers/:store_id/storefront (not in openapi) · `admin_delegated_storefront.go` (`readStorefront:319`), route `routes_delegated.go:92` | delegated guard `GetStoreStateForGuard` + GetSellerForDelegation | `Store` (A10) | c delegation A10 — Q17 | website 1 (`app/(delegated)/adm/[storeId]/storefront`) |
| D45 | PATCH /admin/sellers/:store_id/storefront · `admin_delegated_storefront.go:101` | guard + UpdateSellerForDelegation (expectedVersion) | `UpdateProfile` | c A10 — Q17 | 1 |
| D46 | POST /admin/sellers/:store_id/storefront/logo · `:201` | guard + UpdateLogoForDelegation | `UpdateLogo` (UploaderIsAdmin) | c A10 — Q17 | 1 |
| D47 | POST /admin/sellers/:store_id/storefront/cover · `:236` | guard + UpdateCoverForDelegation | `UpdateCover` | c A10 — Q17 | 1 |

Adjacent: `POST /admin/seller_prospects/media/upload_sessions` (createAdminProspectUploadSession, `admin_seller_prospect.go:883`) and `…/:session_id/complete` (`:925`) call only `h.Media.*`. c media (B1) — stay in gateway; the staged token they produce feeds D25. Client `admin/src/services/seller-prospect-media.ts:22,44`.

### 1.4 Disposition tally (92)

| | a | b | c | total |
|---|---|---|---|---|
| public | 9 (P1–P4, P11, P12, P14–P16) | 1 (P13) | 6 (P5–P10) | 16 |
| app | 24 (A1–A12, A17, A18–A28) | 3 (A13, A15, A16) | 2 (A14, A29) | 29 |
| admin | 37 (D1–D5, D10–D25, D28–D43) | 6 (D6–D9, D26, D27) | 4 (D44–D47) | 47 |
| total | 70 | 10 | 12 | 92 |

Merges (one RPC serves several routes): A4, A8 → A3; A20 → A19; A21 → A18; P4 → P3 (→ P2 if folded); D12 → D11; D30 → D29; D43 → D4. Splits: D13 → Approve/Reject; D29 → StartInvestigation/Resolve. New with no old route: admin CloseStore, RevealKYCSensitive, RebindPayUni, GetReport, ListViolations. Side effects deleted: RecordStoreVisit inside P2. Adjacent deletes: `seller_binding`.

## 2. Diffs vs the old inventory §4 (snapshot of `50a838f08`)

Routes: none added or removed among the 92. `git diff 50a838f08 HEAD -- backend/gateway/internal/router/routes_*.go | rg '^[+-]\s.*\.(GET|POST|PUT|PATCH|DELETE)\('` shows only buyer/ai removals and `seller_binding` + buyer notification-preferences re-registered in `routes_buyer.go`.

Behavior and attribution changes (handlers, `git diff --stat 50a838f08 HEAD -- backend/gateway/internal/{router,handler,grpc,middleware,storeid,calleridentity}`):
1. Suspension guard (`router/seller_capability.go:15 requireSeller`) no longer calls seller-service or Redis: it reads `sellerSuspended` set from the monolith's `X-Seller-Suspended` header (step 1; `internal/legacy/gatewayproxy/proxy.go:41,99`). Old §4.3's Redis 500 ms / GetSeller 900 ms / fail-open chain is gone. `injectSellerSuspension` (`seller_capability.go:74`) and `sellerViewGuard` (`:62`) remain, reading the same context value.
2. Store resolver (`storeid/resolver.go:38`): the caller's own store comes from `X-Store-Id`; looking up another account still calls `GetCurrentStoreID` (21 `h.Store.CurrentStoreID` call sites, `rg -c` over `handler/*.go` non-test). Redis cache and `Invalidate` removed (ApplySeller, DeleteSellerAccount no longer bust).
3. `GET /app/sellers/me/dashboard`: old table listed `GetProductViewStats, GetSellerStats`; HEAD `seller_dashboard.go:48` calls only `GetProductViewStats` on seller (file unchanged since `50a838f08`, so the old row over-attributed). Followers come from `h.Buyer.GetSubscriberDashboardStats`, now served by the monolith's legacy buyer server.
4. `POST /app/chat/messages`: old attributed GetSeller to `inject`; actually `chat.go:143 resolveChatTargetID` → `resolveStorefrontSellerID` → GetSeller. Same RPC.
5. `GET /app/address/{suggestions,validate}`: buyer branch removed (`address_geocode.go`), buyer callers get 400; seller branch still calls TGOS RPCs.
6. `DELETE /admin/accounts/:id`: the buyer leg `h.Buyer.DeleteBuyerAccount` now lands in the monolith (`internal/legacy/buyerserver`, = `accountdeletion.Request`), so today admin delete = seller-service delete + monolith whole-account deletion — the "partial failure" D15 still exists on the seller leg until D1 moves.
7. `PUT /app/buyers/me/seller_binding` moved to `handler/seller_binding.go` (still account-only).
8. openapi has 536 ops (was 569; buyer ops removed). All 88 seller ops still present.

## 3. Gateway orchestration that must move into the monolith

| Mechanism | Today | Target in step 8 | Status in monolith |
|---|---|---|---|
| Capability declaration per method | `sellerHandle(g, method, path, cap, h)` registers `(method, path, cap)` in `authz` registry and mounts `requireSeller(cap)`; boot assertion (`seller_route_assert.go`, ADR `2026-09-18-seller-capability-authorization.md` D1/D2/D9); 13 seller-route declarations in `routes_seller.go` + governance routes have none | every `SellerAppService` method declares a capability (or an explicit exemption: ApplyStore, own-violation reads/appeal), checked at startup; handler calls `seller.go:280 ActingStore(account, cap)` which returns the store and PermissionDenied/FailedPrecondition | `seller.go:230 Capabilities`, `:234 CapabilitiesOf`, `:239–252` constants exist; no declaration mechanism in `internal/httpapi` (`rg -i 'capab|ActingStore' internal/httpapi cmd/hawkr` → none) |
| SELLER_SUSPENDED | 9240 + HTTP 409 (`seller_capability.go:51 denySuspended`) | `permission_denied` + `ErrorInfo.reason = SELLER_SUSPENDED`, value added to `api/hawkr/common/v1alpha1/error_reason.proto` (today only `VERSION_CONFLICT = 1`) | module returns PermissionDenied (seller `AGENTS.md:124`) |
| Store open-orders on close | 9205 bizcode | `ErrorReason` if client branches (plan step 8) | `CloseCurrent` returns FailedPrecondition |
| Anonymous product-view limit | none (`routes_product.go:27`) | per-IP limit at entry; guests only | `views.go:28 RecordProductView`; entry limiter is per-audience tier only (`internal/httpapi/ratelimit.go:31 tierOf`) |
| Public food-safety report limit | `DualWindowRateLimit("food_safety_report", 5/10 min, 10/h)` per IP (`routes_governance.go`) | same at entry (CARRYOVER line 76) | not in httpapi |
| SubmitKYC per-account limit | none (old service had no identity matching) | required before opening `SubmitKYC` (seller `AGENTS.md:106`, CARRYOVER line 164) | not in httpapi |
| Vendor portal limits | preview/exchange 20/5 min keyed `sha256(secret):ip`; step-up none | per-IP, plus a narrower limit on the verification endpoint (agent CARRYOVER lines 18, 45) | not in httpapi |
| Vendor portal session | `middleware.VendorPortalSession` → gRPC `ValidateVendorPortalSession` per request (7 routes), cookie `hk_vendor_portal` | entry middleware calls `agent.go:139 ValidateSession`; writes require `Verification != nil`; cookie attributes decided in step 8 (agent CARRYOVER lines 39–42) | module ready; no transport |
| Google step-up chain | gateway → monolith legacy `VerifyGoogleIdToken` → seller-service `StepUp…` | entry: `identity.VerifyGoogle` → `agent.VerifySession` | both exist |
| Store-card enrichment (buyer favorites/follows, posts, marketplace) | gateway `GetSellerBatch`; buyer-app calls `/public/sellers/{id}` once per card (plan step 8 note, A3a N8); monolith `httpapi/buyer` uses legacy `sellerservice.Stores` (`cmd/hawkr/buyer_ports.go:41`) | typed batch Public read; switch `buyerStores` port to seller module | MISSING batch `Stores(ids)` in seller (only `Store(id)` and `ExposableStores(ids)` → IDs + since) |
| Store id / store_code resolution | `resolveStorefrontSellerID` (`handler/seller.go:306`) accepts store_id or store_code via GetSeller | Public `GetStore` by opaque id/code | MISSING by-code lookup |
| Admin store_id → account_id | `ResolveSellerAccountID` before suspend/lift/list/seo (`admin_seller_suspension.go:11`, `admin_seller.go:387`) | not needed: module APIs take `store_id` | done (by design) |
| Admin delete account | seller → buyer → account cascade in gateway (`admin_account.go:34`) | `accountdeletion.Request(Admin)` in one tx; admin "close store" separately | `accountdeletion.go:88` ready |
| PII no-store | `PrivateNoStore` on `/app/certifications`, `/app/sellers/me/payuni` (`routes_admin.go:54`, `routes_seller.go:66`; test `routes_kyc_payuni_cache_control_test.go`) | Connect responses for KYC/PayUni must be `no-store` | check `internal/httpapi/httpcache.go` per-method policy |
| RecordStoreVisit side effect | goroutine in `GetSellerInfo` | delete | — |

Not moving in step 8 (step 9 per plan/CARRYOVER lines 181–184): `injectSellerSuspension`, `sellerViewGuard`, delegated guard `GetStoreStateForGuard`, entry `CurrentStore` swap from legacy `sellerservice` to the module.

## 4. Module public APIs with no route mapped

| API | Why no route | Proposed |
|---|---|---|
| `kyc.go:125 RevealKYCSensitive` | new (7c: 後台顯示全文要寫稽核) | a `SellerAdminService.RevealKYCSensitive` |
| `payuni.go:124 RebindPayUni` | replaces old RetryPayuniRegistration (no caller) | a admin `RebindPayUni` |
| `payuni.go:145 PayUniObservations` | new observation log | a admin (in GetStore or `ListPayUniObservations`) or internal |
| `payuni.go:140 PayUniRegistrationOf` | admin detail | used by D4 |
| `governance.go:201 Report` | old admin had list-only (`admin/.../violation-reports/[source]/[id]` page) | a admin `GetReport` |
| `governance.go:267 AdminViolations` | old had no admin violations list route | a admin `ListViolations` (client?) |
| `prospect.go:205 SearchProspects`, `:211 ProspectContact` | ai-service / agent internal | step 9 legacy (ai), internal (agent) |
| `agent.go:118 EnsureInvite`, `:124 MarkInviteDelivered` | notification-service | step 9 legacy |
| `views.go:34 IngestProductView`, `:45/51/56` rollup/rebuild/purge | buyer event / jobs | internal |
| `seller.go:274 ExposableStores` | discovery + public reads | used by P1/P2 composition |
| `seller.go:262 CurrentStore`, `:280 ActingStore` | entry / handlers | step 8 handlers, step 9 entry |
| `seller.go:326 BeginDeletion`, `:331 CloseForDeletion` | accountdeletion | internal |
| `discovery.go:85 RecomputeSellerScores` | job | internal (admin trigger not needed) |
| `accountdeletion.go:91 CompletePending` | legacy consumer | internal |
| `buyer.go:398 SubscriberCounts`, `:403 SubscriberDashboardStats` | composition inputs | httpapi/seller, httpapi/discovery |

## 5. Routes needing not-yet-migrated services

| Route | Needs | Existing legacy client |
|---|---|---|
| P2/P3/P4 store page rating | review | `internal/legacy/reviewservice/reviewservice.go:44 StoreRatingStats` (batch, ≤ MaxStoreIDs) — enough |
| P1 recommended display | ranking ports (order, product, review) | `cmd/hawkr/discovery.go:20` wires `orderservice.StoreOrderStats` (`orderservice.go:279`), `productservice.StoresWithPublicProducts` (`productservice.go:71`), `reviewservice` — done; display needs seller batch read (MISSING) + `buyer.SubscriberCounts` |
| A12 dashboard | crm `GetSellerStats`, `GetSellerDashboardGrowth`; order `GetSellerOrderStatusCounts`, `GetSellerDashboardGrowth` | MISSING: `internal/legacy/crmservice` has only `VIPStatuses` (`crmservice.go:29`); `orderservice` has only `HasOpenOrders`, `MerchantSummary`, `SpendingRecords`, `HasOpenStoreOrders`, `StoreOrderStats` |
| A5 / D1' close store | order open orders | `orderservice.go:209 HasOpenStoreOrders` — done (compose check pending step 9) |
| P11 product views | product→store | `seller.ProductStores` port wired in `cmd/hawkr/seller.go:112` via `productservice.Cards` |
| D4/D11 admin detail | social post count | none needed: precondition deleted (plan 7c, 2026-10-03) |
| P5–P10, A14, A29 (stay in gateway) | seller `GetSeller`/`GetSellerBatch`/`GetCurrentStoreID` from the gateway | step 9 legacy SellerService server (plan lists gateway's 4) |
| D44–D47 delegated storefront | seller writes from the gateway | NOT in the plan's legacy subset (`UpdateSeller`, `UpdateLogo`, `UpdateCover` would be needed) — Q17 |
| RFQ routes (ai) | session validation + verification evidence | ai.proto/ai-service change (plan step 8) |

## 6. Proposed split of step 8

Sizing basis: A3a step 3 = 29 RPCs (`BuyerAppService` 25 + admin 3 + `DeleteAccount`) in one session including contract, handlers, httpapi composition, tests. Step 8 ≈ 52 RPCs plus cross-cutting entry work (capability registry, three new limiters, cookie session). Seven sub-steps; each ends with `buf lint`, `make connect`, handler tests, `TestEveryServiceInTheContractIsMounted`, `go test -race ./...`, integration, `make lint-monolith`, code-review + design-review per W7.

| Sub-step | Scope (RPCs) | Acceptance (beyond the common checks) |
|---|---|---|
| **8a Seller App core + capability framework** | `hawkr.seller.v1alpha1` package, shared address/twaddr messages (with the buyer address change, plan step 8), `ErrorReason` `SELLER_SUSPENDED` (+ open-orders reason if needed); per-method capability declaration + startup check; `SellerAppService`: ApplyStore, GetMyStore, UpdateMyStore (mask covers SEO-less fields, food reg, onboarding), UpdateMyStoreLogo, UpdateMyStoreCover, CloseMyStore (≈6) | startup test fails when a seller-region method lacks a declaration; suspended account gets `permission_denied`+`SELLER_SUSPENDED` on write, succeeds on GetMyStore/CloseMyStore; etag/VersionConflict; Apply with bad town code creates nothing; buyer address proto switched to town codes |
| **8b KYC, PayUni (self), seller-side governance** | SaveKYCDraft, GetMyKYC, SubmitKYC, CancelKYC, GetMyPayUni, UpdateMyPayUniProfile, (RefreshMyPayUniStatus per Q11), (LookupBusinessRegistration per Q12), ListMyViolations, GetMyViolation, SubmitAppeal (≈9–11) | per-account SubmitKYC limiter with test; KYC/PayUni responses `no-store`; `google.protobuf.Date` for dates; appeal methods reachable by an account whose stores are all closed (seller `AGENTS.md:60`) |
| **8c Public reads + composition + discovery** | `SellerPublicService`: GetStore (by id or code, exposure rule, rating via review legacy, followers via buyer), BatchGetStores, RecordProductView (guest, IP limit); `DiscoveryPublicService.ListRecommendedSellers`; `SellerAppService.GetDashboard` (Q9); switch `httpapi/buyer` `Stores` port to the seller module (≈5) | new seller APIs `StoreByCode`, `Stores(ids)` with tests; non-exposable/closed store behavior per Q6; product-view IP limit test; recommended list returns names/logos/followers; buyer handlers no longer import `legacy/sellerservice` |
| **8d Admin stores, suspension, KYC review, PayUni admin, account deletion** | `hawkr.admin.seller.v1alpha1.SellerAdminService`: ListStores, GetStoreStats (Q14), GetStore, UpdateStore (SEO mask), SuspendStore, LiftSuspension, ListSuspensions, CloseStore, ListKYCApplications, GetKYCApplication, ApproveKYC, RejectKYC, RevealKYCSensitive, RebindPayUni; `IdentityAdminService.DeleteAccount` (≈15) | new module APIs for admin list/stats and KYC-by-store; reveal writes audit; delete legacy `DeleteBuyerAccount`; one-tx admin delete test |
| **8e Admin governance + reporter endpoints** | `SellerGovernanceAdminService`: ListReports, GetReport, AcknowledgeReport, StartInvestigation, ResolveReport, RecordFDAReferral, GetFDAExport, RequestCooperation, CreateViolation, GetViolation, ListViolations, UpdateViolation, RevokeViolation, DecideAppeal (14); reporter SubmitReport (app + public) and ListMyReports if Will opens them (Q23) | dual-window IP limiter for anonymous submit; duplicate anonymous submit indistinguishable from new (CARRYOVER line 55); FDA export rendering decided |
| **8f Admin prospects (merchant list)** | `SellerProspectAdminService`: List, Create, Preview, Get, Update, Archive, Restore, Merge, Convert, ListDuplicates, ListTaxIDMatches, UpdateProspectImages (12) | admin upload sessions stay on gateway media; invite routes deleted from openapi/gateway in step 10 |
| **8g Vendor portal** | `hawkr.agent.v1alpha1.VendorPortalPublicService`: PreviewInvite, ExchangeInvite, VerifySession (Google) (3); entry session middleware for the 6 ai RFQ routes (forward session + verification evidence); ai.proto + ai-service accept evidence on submit/withdraw/decline | per-IP limits on preview/exchange and a narrower one on verify; cookie attributes; unauthenticated errors indistinguishable; ai stores evidence on the immutable quote version |

Ordering: 8a first (framework), then 8b/8c/8d/8e/8f/8g in any order; 8g is independent of seller. Total ≈ 6+10+5+15+16+12+3 ≈ 67 including optional ones.

Sequencing caveat: until step 9 swaps the entry (`gatewayproxy` `CurrentStore`) and the legacy server, Connect handlers read module data while the entry and the remaining gateway routes read seller-service — two copies of seller data (same situation as A3a step 3→4). Either accept "new API not usable against real accounts until step 9" (A3a precedent) or pull the entry swap into 8a.

## 7. Design questions ("reuse or redesign")

Q1 **Capability declaration mechanism.** Old: `sellerHandle` = declaration and mounting in one call + boot assertion; exists because every seller route must decide behavior under suspension and gateway was a separate process reading a cached flag. Options: (a) proto method option `(hawkr.seller.capability)` read by an interceptor + startup reflection check; (b) Go map procedure→capability in `internal/httpapi/seller` + startup check against the service descriptor; (c) each handler calls `ActingStore(cap)` and a test greps. (a)/(b) give declaration-at-contract; handlers still need the store returned by `ActingStore`.

Q2 **Service partition.** Old: one REST namespace; buyer reporters and sellers share `/app/...` without seller guard. Putting buyer-facing reporting into `SellerAppService` breaks "every seller-region method declares a capability". Options: separate `SellerReportAppService` (audience app, no capability) vs exemption list. Same for appeal (no CAP gate by decision).

Q3 **PII cache policy.** Old: `PrivateNoStore` middleware on route groups + test floor of 9 routes. Need the Connect equivalent per method (KYC, PayUni, Reveal).

Q4 **Per-method rate limits.** Old: route-specific gin middlewares (dual-window food-safety; vendor portal 20/5 min); none for product views; nothing for SubmitKYC. Current httpapi limiter only tiers by audience, in-memory per instance. SubmitKYC per-account must be reliable across instances → DB counter (identity's SMS precedent) vs in-memory.

Q5 **Public store identifier.** Old accepted store_id (pre-2026-07-23 links) or store_code (`handler/seller.go:297`) because URLs migrated to the short code. New IDs are opaque strings; store_code is also PayUni's external MerID source. Pick one public key (code) or accept both; module needs a by-code read.

Q6 **Public visibility.** Old `GetSeller` returned any store including closed/uncertified. New rule `model.Exposable`. Should `GetStore` 404 non-exposable stores, return a "not available" state, or only hide closed ones (seller `AGENTS.md:120`: 對外隱藏已關閉店家的資料由呈現層決定)? Favorites/follows of closed stores need a state, not a 404.

Q7 **Rating/followers composition.** Old: `sellers.rating` replica (deleted 7b) + `GetSellerStats` calling buyer and review; three REST shapes (`/:id`, `/stats`, `/rating`). Fold into one `GetStore` vs separate stats RPC; review is per-request legacy call (batch-capable).

Q8 **Batch store cards.** Old: buyer-app N+1 `/public/sellers/{id}`; gateway `GetSellerBatch` for posts. New `BatchGetStores` — returns non-exposable entries flagged? max batch size? Shared with discovery display and `httpapi/buyer`.

Q9 **Seller dashboard.** Old: gateway fan-out over crm, order, buyer, seller, best-effort per section. Options: (a) Connect `GetDashboard` in `internal/httpapi/seller` with 4 new legacy client methods (crm×2, order×2) — temporary clients until A6/B; (b) Connect only for views + followers, gateway keeps the rest until crm/order migrate (two calls on one screen); (c) leave dashboard in gateway and have the gateway call the monolith for view stats (new reverse dependency).

Q10 **Marketplace aggregate.** Old: one gateway endpoint for ranking + posts + products. Options: client composes (Discovery Connect + remaining REST) vs keep gateway aggregate calling the monolith. discovery `AGENTS.md:36` says "marketplace 的榜單同一步處理".

Q11 **Seller self PayUni refresh.** Old: seller button with 30-min minimum interval (protect PayUni 2 QPS). New `RefreshPayUniStatus` designed for admin (no interval; webhook + sweep keep state fresh). Keep a seller RPC with a throttle, or drop it (b).

Q12 **GCIS business-registration lookup.** Old: seller-service client to GCIS for KYC autofill. CARRYOVER line 82 deferred it to "7c 後續" and nothing landed. Implement `platform/gcis` (external call outside tx, public government API, D9-clean) or delete (manual entry).

Q13 **KYC addressing.** Old: application ID in path + gateway ownership check (D22 IDOR). New: account-scoped `MyKYC`, no ID; review history only latest rejection for the seller. Confirm `GetMyKYC` replaces `/:id` and `/:id/reviews`, and drop `certificationDraftId` from GetMyStore.

Q14 **Admin store list and stats.** Old `ListSellersForAdmin` (offset, filters incl. derived cert status via `expandApprovalStatusFilter`) and `GetSellerStatsForAdmin`. Module has neither; filters on the derived KYC flow need a query over two axes (CARRYOVER line 119 defers to step 8). Redesign with keyset + which filters; are stats still used by admin (3 files)?

Q15 **Admin close store vs delete account.** Plan: two admin actions. `CloseCurrent` rejects on open orders and pending deletion; old admin path force-overrode open orders (`OverrideActiveOrders=true`, deleted in 7c). What does admin "刪賣家角色" do with open orders — reject, or wait like accountdeletion?

Q16 **Vendor portal session transport.** Old: HttpOnly cookie read by gateway middleware calling gRPC. New: a non-account principal in httpapi (new Audience/Principal kind) for `VendorPortalPublicService`, and how the 6 ai RFQ routes (A9) get the session — entry validates and forwards trusted headers (like `X-Store-Id`) vs legacy `ValidateVendorPortalSession` server.

Q17 **Delegated storefront (A10).** Old gateway writes seller-service directly with `expectedVersion`. After seller-service deletion these 4 routes break unless (a) step 9 legacy server also serves `GetSeller`/`UpdateSeller`/`UpdateLogo`/`UpdateCover` backed by the module, (b) admin storefront delegation moves to Connect now (A10 early), or (c) routes are unavailable until A10. Plan's gateway legacy subset lists only 4 read RPCs.

Q18 **Cutover order.** Step 8 handlers on module data vs entry/gateway on seller-service until step 9 (see §6 caveat).

Q19 **ErrorReason set.** Only `SELLER_SUSPENDED` is decided. Candidates: store has open orders (close), account pending deletion (apply), KYC identity blocked (sanctioned identity match must not be distinguishable → probably no reason), duplicate report (must not be distinguishable for anonymous).

Q20 **Product-view endpoint shape.** Old: product-scoped public REST, swallowed errors. New: seller-owned `RecordProductView(productID)`; lives in `SellerPublicService` though it is a product-page beacon; guests only, logged-in views via buyer's `ProductViewed` (double count until website stops beacon for logged-in users, step 10).

Q21 **Admin "status PATCH" vs commands.** Old PATCH status for two report tables; new unified reports with explicit commands (Acknowledge/StartInvestigation/Resolve). Confirm command-style RPCs (admin UI rework in step 10).

Q22 **FDA export format.** Old: server-generated Markdown. New `FDAExport` structured; render on server (string field) or in admin client.

Q23 **Reporter endpoints without clients.** `POST /app/violation_reports`, `GET /app/violation_reports/me`, `POST /public/food_safety_reports` have no client (old §5 and HEAD rg). Open the Connect RPCs now (backend ready, UI later with consent) or defer.
