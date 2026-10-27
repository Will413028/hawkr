# A3b 10d 公開讀取 — client 盤點（唯讀，HEAD af1b0dcc7）

Sources: plan `backend/docs/plans/2026-09-29-a3-buyer-seller.md:297`; step10-decision-brief.md (B3/B6/B11/B19, 附錄 A, B20 10d row :64); step10b-decision-brief.md:162 (zero-consumer governance routes → 10d); step10c-decision-brief.md:29 (K4: `/public/sellers/*` kept for 10d); contract `api/hawkr/seller/v1alpha1/seller.proto:115-146,975-1039`, `common/v1alpha1/store_card.proto`, `discovery/v1alpha1/discovery.proto`.

## 0. Routes being deleted (gateway, `backend/gateway/internal/router`)
- routes_seller.go:18 `GET /public/sellers/recommended`; :22 `/:id`; :23 `/:id/stats`; :24 `/:id/rating`
- routes_marketplace.go:13 `GET /public/marketplace`
- routes_product.go:27 `POST /products/:id/view`
- zero-consumer per 10b brief:162: routes_governance.go:59 `GET /public/sellers/:id/violations/public`, :45 `POST /public/food_safety_reports`, app `violation_reports` (2). Consumers found: none in website/buyer-app/seller-app; admin uses only `/admin/food_safety_reports/*` (10e scope).
- STAY (other modules, numeric store id): `/public/sellers/:id/products|reviews|reviews/stats|posts|forum`, `/public/sellers/forum/:id/replies`.

## Contract notes found while reading
- C1 discovery.proto service comment says "回應裡的評價統計與排名來自同一份快照，不另查評價" but `RecommendedSeller` reserved 2,3 and ratings now come from `StoreCard` (10a T2 (a)). Doc comment stale.
- C2 local/10e removes `PublicStore.seo_title/seo_description` (reserved) per ADR `2026-10-09-store-seo-platform-generated.md` (only on local/10e). 10d storefront metadata must build on 10e's `lib/seo/store-summary.ts` (inputs name/description/county/town/rating → from town_code + stats.rating_average).
- C3 `RecordProductView` server already returns success without counting for signed-in callers (seller.proto:142-145, httpapi/seller/public.go:152); client split by login is SSOT (附錄 A) but not needed for correctness. IP limit uses ClientIP (httpapi/subject.go:42); website `publicTransport` has `visitorIp` (lib/connect/transport.ts:81-91).

## 1. seller-app (done by orchestrator)
| file:line | now | replacement | gap |
|---|---|---|---|
| seller-app/lib/features/seller/data/seller_repository.dart:51-65 `getUserProfileMap(accountId)` → `GET /api/v1/public/sellers/$accountId` | only caller page_echo_tab; id is forum record `userId`, which is store_id for seller replies and buyer_id for buyer comments (mixed namespaces, no discriminator in gateway `ForumRecord`, backend/gateway/internal/handler/seller_forum.go:15-33) | BatchGetStores for store ids only — but client can't tell which ids are stores | **G-S1**: drop the fallback entirely (record already carries userName/avatarKey as authority; self handled by `myStoreSnapshotProvider`) vs. add author type to forum record (social module, legacy) |
| seller-app/lib/features/post/presentation/widgets/page_echo_tab.dart:153-173, 295-345 (`_ensureUserProfile`, `AuthStorage.get/saveUserProfileFor` persistent cache), 354-375 `_logoSourceFromProfile`, 380+ `_nameFromProfile`, call sites :269, :456 | fallback name/avatar for comment authors | see above; if deleted, also `AuthStorage.saveUserProfileFor/getUserProfileFor` (core/storage/auth_storage.dart:69,90) lose their only caller | same |
| seller-app/test/features/post/page_echo_input_test.dart:31,77-100,460-470 | mocks `getUserProfileMap`, asserts own reply never loads profile | update with chosen design | — |
| seller-app/lib/features/post/presentation/page_home_feed_screen.dart:258 | share URL `https://example.com/sellers/$sellerId/posts#post-…` with numeric own store id — relied on the store_id→store_code redirect being deleted | `myStoreSnapshot.storeCode` (core/session/my_store.dart:49) | own store not yet exposable → link 404s (design Q: hide share when not OPEN?) |
| seller-app/lib/features/product/presentation/product_preview_screen.dart:300 | stale comment mentions `/public/sellers/{id}/rating`; code calls `/public/products/$productId/rating` (product_repository.dart:320) | comment fix only | — |
| seller-app/lib/core/networking/connect_provider.dart:50-53 | `sellerPublicClient` provider already exists (GetSellerAgreements) | reuse | — |
| seller-app/lib/features/chat/data/chat_picker_remote_service.dart:28 | `/public/sellers/$storeId/products` | STAYS | — |

## 2. admin
No call sites of deleted routes; no links to public storefront (rg `storefront|/sellers/${` in admin/src). admin `/admin/food_safety_reports/*` and `/admin/sellers/*` are 10e. Count 0.

## 3. packages
- `packages/hawkr-api-ts/src/gen/hawkr/{seller,discovery,common}/v1alpha1/*_pb.ts` and `packages/hawkr_api/lib/gen/hawkr/{seller,discovery}/v1alpha1/*.connect.client.dart` already contain GetStore/BatchGetStores/RecordProductView/SuggestRecommendedSellers — no generation needed unless contract changes.
- local/10e regenerates `seller_pb.ts` and Dart `seller.pb*.dart` (proto changes) → any 10d contract change conflicts there.
- town_code display: website `lib/addresses/twaddr.ts`; buyer-app `packages/hawkr_twaddr`; local/10e adds `packages/hawkr-twaddr-ts` (new TS package) — 10d website should use whichever lands first.

## 4. Overlap with local/10e (merge-base d700a7caa, 3 commits behind main; 447 files)
Exact file overlap with 10d website touch list: `website/app/(public)/sellers/[id]/(storefront)/page.tsx`, `website/features/sellers/storefront/metadata.ts`, `website/lib/api/index.ts` (fetchSeller seo fields), `website/lib/seo/metadata.ts` (sellerSeoTitle removed), `website/lib/analytics/page-view-privacy.ts`, `website/tests/e2e/helpers/mock-gateway.ts`, `website/tests/e2e/protected-dynamic-routes.spec.ts`, `website/tests/seo/seller-seo-override.spec.ts` (deleted on 10e). Likely too: `website/lib/query/keys.ts` (sellers.detail key), new `website/lib/seo/store-summary.ts`, `api/hawkr/seller/v1alpha1/seller.proto` + both generated seller files, `seller-app/lib/core/session/my_store.dart`, `seller-app/lib/core/router/app_router.dart`. buyer-app: no overlap (10e touches no buyer-app file).

---
# A3b 10d — website inventory (seller public reads → Connect)

HEAD af1b0dcc7, read-only. Paths are relative to `website/` unless noted. `W=<repo>/website`.

Contract checked: `api/hawkr/seller/v1alpha1/seller.proto` (SellerPublicService: GetStore, BatchGetStores, RecordProductView), `report.proto` (SellerReportAppService.CreateReport, SellerReportPublicService.SubmitFoodSafetyReport — **both exist**), `discovery/v1alpha1/discovery.proto` (SuggestRecommendedSellers), `common/v1alpha1/store_card.proto`, `common/v1alpha1/image.proto` (Image = `uri_template` only).
Website today has **no** client for SellerPublicService / DiscoveryPublicService / SellerReport* (rg below finds zero). Existing pattern to copy: `lib/connect/content.ts` (`createClient(ContentPublicService, publicTransport(opts))`); `publicTransport` already supports `visitorIp` (lib/connect/transport.ts:81-91), which middleware needs.

---

## 1. Deleted-route call sites (production code)

| # | file:line | route | repo fn → consumers | replacement |
|---|---|---|---|---|
| 1 | lib/api/index.ts:365-410 | GET `/public/sellers/{id}` | `fetchSeller(id, strict)` (renames to `name`, `coverImage`=coverKey, `logo_url`=logoUrl, strips logo fields; `isSellerAccountId` digit guard) | GetStore(store_code) for storefront; BatchGetStores(ids) where only id is known |
| 2 | lib/api/cached.ts:275-286 | same | `getCachedSeller` (React cache + unstable_cache, key `["seller",id]`, tag `seller:<id>`, TTL SELLER_PROFILE, `.catch(prerenderSafe(null))`) | GetStore |
| 3 | lib/api/index.ts:538-558 | GET `/public/sellers/{id}/stats` | `fetchSellerStats` / `SellerStats{storeId,followers,rating,totalOrders,totalProducts,totalSales}`; only `followers` is read anywhere | GetStore `stats.follower_count` |
| 4 | lib/api/cached.ts:305-316 | same | `getCachedSellerStats` key `["seller-stats",id]` | GetStore stats |
| 5 | lib/api/index.ts:158-182 | GET `/public/marketplace` | `fetchMarketplace(sellerLimit,postLimit,productLimit)` → `MarketplaceData{hotProducts,latestPosts,recommendedSellers}` | compose (see §3) |
| 6 | lib/api/cached.ts:396-434 | same | `getCachedMarketplace` key `["marketplace",…]` tags products/sellers/posts | compose |
| 7 | lib/api/index.ts:520-536 | GET `/public/sellers/recommended` | `fetchSellers(limit)` → `MarketplaceSeller[]` | SuggestRecommendedSellers(max_results) |
| 8 | lib/api/cached.ts:507-522 | same | `getCachedRecommendedSellers` key `["recommended-sellers",limit]` tag `sellers:list` TTL DISCOVERY | same |
| 9 | lib/seller/canonical-redirect.ts:142-195 | GET `/public/sellers/{id}` | private `fetchSeller` → `lookupSeller` (memo 30 min / 500) → `resolveSellerRequest` → middleware.ts:98-129 (308 to store_code, rewrite 404) | redirect deleted; 404 via GetStore(store_code) not_found |
| 10 | features/product/ProductViewTracker.tsx:14 | POST `/products/{id}/view` | fires for everyone (anon + logged-in) | RecordProductView, only when not logged in |

Zero website consumers (rg below): `/public/sellers/{id}/rating`, `/public/sellers/{id}/violations/public`, `/public/food_safety_reports`.

### Consumers of the deleted repo functions

`getCachedSeller` (14):
- app/(public)/sellers/[id]/(storefront)/layout.tsx:29 — chrome; passes `seller` + route `id` to SellerProfileV2; `notFound()` if null
- app/(public)/sellers/[id]/(storefront)/page.tsx:36 — JSON-LD sellerSchema, breadcrumb, sr-only h1, `sellerCanonicalUrl(seller,id)`
- app/(public)/sellers/[id]/(storefront)/{posts,reviews,messages}/page.tsx:20 (3)
- features/sellers/storefront/metadata.ts:19 — generateMetadata (seoTitle/seoDescription, rating summary)
- features/sellers/posts/SellerPostsPage.tsx:18, features/sellers/reviews/SellerReviewsPage.tsx:30, features/sellers/messages/SellerMessagesPage.tsx:16
- app/(public)/sellers/[id]/card/page.tsx:34,56 — card metadata + page
- app/(public)/sellers/[id]/apple-icon.tsx:92 — avatar icon
- app/(seller)/seller-center/preview/[id]/layout.tsx:23 — seller preview (public read)
- features/product/SellerIntroAsync.tsx:26 — PDP seller card

`fetchSeller` client-side (3): features/account/screens/messages/ThreadClient.tsx:229-247 (React Query `queryKeys.sellers.detail(targetId)` = `["sellers","detail",id]`, name/logo/storeCode for header + shop link); features/messages/components/ChatContextPanel.tsx:72 (name/logo/storeCode + `fetchSellerProducts`); features/seller-center/preview/StorefrontProductPreviewDrawer.tsx:130.

`getCachedSellerStats`/`fetchSellerStats` (4): features/sellers/card/SellerCardPage.tsx:55; features/product/SellerIntroAsync.tsx:28; features/seller/components/SellerProfileV2.tsx:140; StorefrontProductPreviewDrawer.tsx:132.

`getCachedMarketplace`/`fetchMarketplace` (6): lib/api/home-bootstrap.server.ts:62; features/home/server/home-tab-bootstrap-route.ts:152 (raw, inside `getCachedHomeLiveBootstrap`), :234 (fallback); app/api/public/tab-bootstrap/v1/warm/route.ts:52; features/cart/CartPage.tsx:90 (`(0,0,6)` → hotProducts only); features/search/SearchPage.tsx:33 (hotProducts only).

`getCachedRecommendedSellers`/`fetchSellers` (6): home-bootstrap.server.ts:65; home-tab-bootstrap-route.ts:186 (raw), :261 (fallback); warm/route.ts:47; features/market/products/MarketProductsListPage.tsx:40; features/feed/components/FeedClientShell.tsx:133 (React Query `queryKeys.feed.recommendedSellers()` = `["feed","recommended-sellers"]`).

Types to retire/replace: `MarketplaceSeller`, `MarketplaceData`, `MarketplacePost` (lib/api/index.ts:74-123), `SellerStats` (:539), `HomeTabBootstrapV1.marketplace/recommendedSellers` (lib/api/tab-bootstrap.ts:16-19, RETAINABLE_DOMAINS :49-53), `recommendedSellerLogoUrl` (lib/seller/recommended-logo.ts — reads `logoUrl` string; StoreCard has `Image.uri_template`).

---

## 2. store_id used for public navigation / URLs / SEO

The whole `/sellers/[id]` subtree treats the segment as "store_id or store_code" because the legacy endpoint matched `store_id = $1 OR store_code = $1`. GetStore takes store_code only, and the staying REST (`/products`, `/posts`, `/forum`, `/reviews`, `/reviews/stats`) takes the numeric store id → every tab must get `PublicStore.id` from GetStore and stop passing the route segment downstream.

Route segment passed to staying REST (must switch to `store.id`):
- features/sellers/storefront/StorefrontPage.tsx:34,39 (`getCachedSellerStorefrontProducts(id)` = `/public/products?store_ids=`, `getCachedSellerProducts(id)`), → SellerProductsClient.tsx:58-74 (client paging, same ids)
- features/sellers/posts/SellerPostsPage.tsx:52-55 (`fetchPublicPostsTyped({sellerId:id})`), :72
- features/sellers/reviews/SellerReviewsPage.tsx:78-81 (`/reviews`, `/reviews/stats`), :138 → ReviewsClient.tsx:43
- features/sellers/messages/SellerMessagesPage.tsx:50 (`fetchSellerForum(id)`), :62 → SellerMessageTab.tsx:44
- features/seller/components/SellerProfileV2.tsx:120 (`getCachedSellerReviewStats(sellerId)`), :262-268 `SellerSubscribeButton sellerId={sellerId}` (follow is store-id keyed) — **route id passed to follow**
- features/sellers/card/SellerCardPage.tsx:49-63 already reads `seller.storeId` (good), but :82 uses route id as link fallback
- cache tags `seller:<id>` (lib/seo/tags.ts:11) — monolith already sends both `seller:<store_id>` and `seller:<store_code>` (backend/internal/platform/revalidate/revalidate.go:42-44), so keying a GetStore cache by store_code still busts.

store_id → store_code redirect machinery (decided: delete, keep real 404):
- middleware.ts:84-129 (`/sellers/` branch, 308 + 404 rewrite) and lib/seller/canonical-redirect.ts (whole file: memo, `lookupSeller`, `resolveSellerRequest`, `isSellerKeyShape`, `MAX_SELLER_KEY`)
- app/(public)/sellers/[id]/layout.tsx:28 `isSellerKeyShape(id)` backstop
- lib/seo/urls.ts:186-193 `canonicalSellerRedirectTarget`
- lib/seo/urls.ts:81-92 `sellerCanonicalUrl(seller,id,tail)` — falls back to route id; callers: storefront page.tsx:39, metadata.ts:25, SellerPostsPage:32, SellerReviewsPage:44, SellerMessagesPage:30, card/page.tsx:37,59 (7)

`sellerPath(storeCode, fallbackId)` fallback (decided: delete fallback; id-only sources resolve via BatchGetStores) — lib/seo/urls.ts:150-159; `sellerCardPath` (:162) has **no production caller** (dead, only tests/seo-urls.test.ts:55-56). Callers passing a fallback id (9):

| file:line | source of data | has store_code today? | after 10d |
|---|---|---|---|
| features/home/homeSnapshot.ts:323 | recommended sellers | yes (storeCode) | StoreCard.store_code (always OPEN per discovery.proto) |
| features/feed/components/StoryRail.tsx:301 (built at FeedClientShell.tsx:58-84, MarketProductsListPage.tsx:50-99) | recommended sellers | yes | StoreCard.store_code |
| features/market/components/MarketProductCard.tsx:215 | product REST `seller.storeCode` (lib/market/listFilters.ts:128-133) | when product wire carries it | **gap Q7** |
| features/market/components/OriginSellerPreview.tsx:41 | `/public/seller-cities/summary.previewStoreIds` (ids only) + code borrowed from a product in the window (lib/origin/aggregateOriginCounts.ts:130-136,177-178) | often no | BatchGetStores(previewStoreIds) |
| features/cart/components/SellerGroupCard.tsx:156 | cart groups `g.storeCode` (lib/cart/types.ts:120-121) | usually | **gap Q7** |
| features/account/screens/favorites/FavoritesClient.tsx:290 | buyer ListMyFollows → `Follow.store` (StoreCard) | yes | StoreCard + B11 states |
| features/messages/components/ChatContextPanel.tsx:97 | fetchSeller | yes | BatchGetStores([sellerId]) + B11 |
| features/product/components/ProductSellerIntro.tsx:180-187 | fetchSeller ∪ product wire seller block | yes | GetStore(code) or product wire; **gap Q5** |
| features/sellers/card/SellerCardPage.tsx:82 | fetchSeller | yes | GetStore.store_code |

Code-only callers already correct (no fallback): RecentClient.tsx:161-163 (Connect Visit → StoreCard), AccountOverviewClient.tsx:694, OrderDetailClient.tsx:72, OrderQuickActionDrawer.tsx:148, CouponsClient.tsx:151, HomeStageFeedCard.tsx:68, PostDetailViewer.tsx:496-498, ThreadClient.tsx:261, procurement-assistant/CandidateBrowser.tsx:380 (hand-built `/sellers/${group.storeCode}`).

Other numeric-id public URLs:
- app/(public)/sellers/[id]/(storefront)/layout.tsx:47 and app/(seller)/seller-center/preview/[id]/layout.tsx:40 — `AppDownloadBannerV2 deepLink={hawkr://seller/${id}}` — after 10d this becomes the store_code (**Q9**: what key does buyer-app's deep link accept?)
- lib/seller/share-card.ts:52-78 `buildSellerShareUrl(displayId || sellerId)` → `sellerCardUrl`; SellerProfileV2.tsx:144-157 passes route id + `s.storeCode` as displayId → fallback to id to delete; also SellerShareCardPreview (seller center, GetMyStore) uses the same builder
- features/sellers/components/SellerMessageTab.tsx:183 login redirect `/sellers/${sellerId}/messages` — fine only while `sellerId` is the route segment; breaks if the prop becomes `store.id`
- features/seller/components/SellerProfileTabs.tsx:41 tab root `/sellers/${sellerId}` — same caveat
- app/(seller)/seller-center/preview/[id]/* + features/seller-center/preview/SellerStorefrontPreviewPage.tsx:21 — `/seller-center/preview/{store_id}` (seller-only URL, but renders via public `getCachedSeller`) — **Q3**
- features/feed/FeedParamController.tsx:89 `/feed?sellerId=` reader keyed by numeric id for `/public/posts` (staying REST; no in-site generator found) — out of 10d scope, listed for completeness
- app/(public)/sellers/[id]/products/[pid]/page.tsx — legacy URL, does not read the seller, canonicalizes to `/products/{pid}`; no internal linker (rg below). Unaffected except the segment semantic.
- sitemap.ts/robots.ts: no seller URLs (sitemap.ts:7 "added in a later PR"). Analytics lib/analytics/page-view-privacy.ts:184,231 normalizes to `/sellers/:sellerId` (template only, no change). lib/navigation/route-transition-policy.ts:36,48,80 pattern-only.
- SEO images: lib/seo/metadata.ts:48-79 `sellerSocialImage` uses `buildImageUrl(objectKey, preset)`; PublicStore gives `Image.uri_template` → switch to the existing `imageUri(image, preset)` helper (used by FavoritesClient/visits).

---

## 3. Marketplace / recommendation composition

`/public/marketplace` (gateway backend/gateway/internal/handler/marketplace.go) = recommendedSellers (GetRecommendedSellers) + latestPosts + hotProducts (`GetPublicProductList(... "sales_desc")`).

| part | website consumer | replacement |
|---|---|---|
| `hotProducts` | homeSnapshot.ts:492-523 (editor picks, hot deals), CartPage.tsx:89-94, SearchPage.tsx:49 | existing REST `fetchProducts(..., sort SALES)` (`/public/products`, already used; `sort` supports SALES per lib/api/index.ts:206-221) |
| `recommendedSellers` | homeSnapshot.ts:524-530 merges it with the separate `recommendedSellers` (dup source) | drop; SuggestRecommendedSellers once |
| `latestPosts` | **no consumer** (only the type) | drop |

Home bootstrap shape: features/home/server/home-tab-bootstrap-route.ts — live wave (`marketplace, products, sellerCitySummary`, :151-171) + discovery wave (`recommendedSellers, recipeHits, posts, categoryTree`, :184-215) + fallbacks (:229-290) + degraded-domain labels (`"marketplace"`, `"recommendedSellers"`) + `x-bootstrap-degraded`; lib/api/tab-bootstrap.ts RETAINABLE_DOMAINS; lib/api/home-bootstrap.server.ts:54-75. Removing `marketplace` changes the versioned DTO (`schemaVersion: "home.v1"`) — **Q10**.

Recommended-seller fields read today vs. contract (`RecommendedSeller{StoreCard store, optional follower_count}`):

| read today (where) | in contract? |
|---|---|
| storeId, storeCode, storeName, logoUrl | yes: id, store_code, name, logo (Image) |
| coverUrl (homeSnapshot.ts:307, quality score :375) | yes: cover_image |
| rating (homeSnapshot.ts:313, :381) | yes: store.rating_average (+review_count) |
| followers (homeSnapshot.ts:314, :382) | yes: follower_count |
| city/district (homeSnapshot.ts:302-303, FeedClientShell.tsx:73-74, MarketProductsListPage.tsx:86-88; StoryRail 附近商家 tab) | town_code → names via existing `lib/addresses/twaddr.ts` (`townByCode`, `countyByCode`) |
| certified (homeSnapshot.ts:304,321,376) | no — B3 removes from card |
| description (homeSnapshot.ts:311-313,377) | no — B3 removes |
| prize (homeSnapshot.ts:322,378) | no — B3 removes |
| latitude/longitude (MarketProductsListPage.tsx:55-72, FeedClientShell.tsx:75-77) | no — and **never present today either** (comment says always undefined) → dead reads |

Client-side re-ranking: homeSnapshot.ts:367-384 `sellerQualityScore` reorders the server ranking by cover/certified/description/logo/prize/rating/followers — **Q4**.

---

## 4. Product view

- features/product/ProductViewTracker.tsx:11-15 — beacon for every viewer via `apiClient.public.post`; :17-26 logged-in viewers also `record()` (buyer RecordVisit via lib/visits/repo.ts:79-81).
- Mounted once: features/product/ProductDetailLayout.tsx:336 (skipped in previewMode).
- lib/hooks/useRecordHistory.ts:35 returns `enabled: hydrated && isLoggedIn`, does **not** expose `hydrated`. Decided split "anonymous only sends RecordProductView" needs `hydrated && !isLoggedIn`; using `!enabled` would fire before hydration for logged-in users. (Server also ignores logged-in callers per seller.proto:141-144, so a stray call is harmless but contradicts the decision.) Doc comment useRecordHistory.ts:11 mentions the old route.
- No test/e2e covers the view beacon (rg below).

---

## 5. Store cards

Card-like seller surfaces and the decided B3/B11 rules:

| surface | file | data today | after 10d |
|---|---|---|---|
| Home 推薦商家 rail | features/home/components/HomeCompactMerchants.tsx:216-312 (+ `MarketSellerItem` type in features/market/components/MarketSellerCard.tsx:11-29) | cover, location chip, logo, name, certified badge (:272), prize pill (:283-289), "商品 N · 評分 x · 追蹤人數 N" (:296-302), SellerFollowChip(store id) | B3: drop certified/prize/description; rating from card, followers from follower_count; all OPEN (recommended) |
| `MarketSellerCard` default component | features/market/components/MarketSellerCard.tsx:64 | — | **no importer of the component** (only its type) — dead code |
| Feed/market StoryRail | features/feed/components/StoryRail.tsx:22-24,83,301 | avatar, name, city/district | StoreCard (OPEN) |
| Favorites follow list | FavoritesClient.tsx:284-300 | StoreCard already; no status handling | B11 states |
| Recent visits / member cards | lib/visits/repo.ts:54-67, lib/member-cards/repo.ts:70-77 → RecentClient.tsx:161 | StoreCard already; no status handling | B11 states (already-migrated surfaces; not deleted-route consumers) |
| Chat header / context panel | ThreadClient.tsx:229-263, ChatContextPanel.tsx:57-110 | fetchSeller | BatchGetStores([id]) + B11 |
| PDP seller intro | ProductSellerIntro.tsx:155-260 | name, logo, city/district/address, rating (reviews/stats ∪ seller.rating), review count, subscribers (stats.followers), storefront link | **Q5** |
| Origin map preview | OriginSellerPreview.tsx | ids + borrowed code | BatchGetStores |
| Seller card page `/sellers/[id]/card` | SellerCardPage.tsx:20-140 | city/district, prize pill (:131-137), description, rating/reviews (reviews/stats), followers | GetStore (prize is a PublicStore field; this is the storefront card, not a list card — B3 scope? **Q6**) |
| Storefront header | SellerProfileV2.tsx:130-275 | name, prize pill (:214-219), location, rating/reviews (reviews/stats), subscribers, website, description, share card | GetStore: all fields present (prize, description, website_url, town_code, stats) |

`StoreCardStatus` / 「已關閉的店家」/「暫停營業」: **zero occurrences** in website (rg below) → B11 rendering is new everywhere.

---

## 6. Tests / mocks touching this scope

Unit/contract (node:test / vitest):
- tests/api-fetch-resilience.test.ts:28-88 (fetchSeller ×7 tests)
- tests/seller-link-canonical.test.ts (16 hits; pins sellerPath/canonical reads)
- tests/seo-urls.test.ts (12 hits; sellerPath/sellerCardPath fallback)
- lib/seller/canonical-redirect.unit.test.ts (msw `${serverApiBase}/public/sellers/:id`)
- tests/preview-host-noindex.test.ts:187 (imports canonical-redirect)
- features/sellers/__tests__/tab-canonical.unit.test.ts (mocks getCachedSeller)
- tests/seller-card-page-contract.test.ts:43-45; tests/navbar-streaming-cache.test.ts:37-38; tests/home-streaming-resilience.test.ts:51-52; tests/home-server-render.test.ts:57-59; tests/tab-bootstrap-cache.test.ts:99-275 (8 hits); features/home/server/__tests__/home-bootstrap-budget.test.ts (21 hits; marketplace/recommended mocks, degraded labels); features/home/homeSnapshot-shelves.unit.test.ts:27-30
- tests/recommended-seller-logo-contract.test.ts:85-92 (MarketplaceSeller + MarketSellerCard)
- tests/seller-preview-ssr-production.test.ts:66,526-532 (route list incl. `/public/sellers/${id}` and `/stats`)
- tests/transport-readhang.test.ts:48,60 and tests/transport-softfail.test.ts:42,151 use `/marketplace` only as a sample path for transport behaviour (swap path)
- tests/api-prerender.test.ts:44, tests/media-objectkey-contract.test.ts:392, lib/api/origin-auth.unit.test.ts:85 (sample URL) — comments/sample only
- tests/seller-pricing.test.ts — false positive (no deleted route; rg count came from other patterns)

E2E / SEO (Playwright):
- tests/e2e/helpers/canonical-seller.ts (whole helper: `canonicalSellerSegment` via `/api/v1/public/sellers/{id}`) ← used by site-links.spec.ts:232, navigation-observability.spec.ts:108, public-dynamic-routes.spec.ts:124, site-stability.spec.ts:507
- tests/e2e/site-links.spec.ts:766-784 (asserts store_id → store_code 308 — decided deleted)
- tests/e2e/protected-dynamic-routes.spec.ts:88 `PUBLIC_SELLER`, :621,:1227,:1264 mock `/public/sellers/${SELLER_ID}`
- tests/e2e/protected-static-routes.spec.ts:52,352-359 (`/seller-center/preview/{id}` + public seller URL)
- tests/e2e/site-stability.spec.ts:426,1148-1212; tests/e2e/masonry-column-parity.spec.ts:171,261-274; tests/e2e/reliability.spec.ts:228,374
- tests/seo/h1-uniqueness.seo.spec.ts:9-30 (live `/public/marketplace?recommended_sellers=5` to discover ids, then `/sellers/{storeId}`); tests/seo/_helpers.ts:134-176 (`/sellers/recommended?limit=1`, storeId); tests/seo/seller-seo-override.spec.ts:37,76; tests/seo/entity-templates.seo.spec.ts:74-80; tests/nav-overlap.spec.ts:52; tests/visual/_helpers.ts (1 hit)
- tests/e2e/helpers/mock-gateway.ts has no seller-public/discovery mocks yet (only GetMyStore etc.); `sellerMethod()` helper exists for Connect GET mocks (:88-95).
- stories: stories/web/InlineLoading.stories.tsx:333 `SellerStats` is a story name, unrelated.

---

## 7. Gaps / design questions (not solved)

- **Q1 Middleware 404 lookup.** Keep a middleware GetStore(store_code) call per `/sellers/*` request (with memo + `visitorIp`) to get a real 404, or drop the middleware branch and accept the prerender's 200-soft-404? The old fail-open/fail-closed split (settled 404 vs outage) maps to Connect `not_found` vs other codes; `unimplemented` (route not deployed) must not 404. Also: keep a shape check? store_code shape is not specified in the proto (`isSellerKeyShape` assumed ≤19 digits).
- **Q2 Non-exposable stores now 404.** GetStore returns not_found for paused/suspended/unverified/incomplete stores; today those storefronts render. Consistent with B11 (PAUSED not clickable) but any old link to a paused store becomes a 404 page — intended?
- **Q3 Seller preview.** `/seller-center/preview/[id]` server layout reads the public seller (SSR, unauthenticated cache). An unexposed own store (the main reason to preview) will 404 under GetStore. Source should be GetMyStore (authed) — does the preview become client-rendered, or server with the Clerk token, and does the URL keep a numeric id at all? Also StorefrontProductPreviewDrawer.tsx:130-132.
- **Q4 Client re-ranking of recommended sellers.** homeSnapshot.ts `sellerQualityScore` reorders the server's scored ranking using fields B3 removes (certified/description/prize). Delete and trust SuggestRecommendedSellers order (ADR recommended-sellers-scoring)?
- **Q5 PDP seller intro.** PDP has only the numeric store id (`getProductSellerStoreId`) plus the product wire's embedded seller (often with storeCode). Followers exist only on GetStore (needs store_code) and RecommendedSeller; StoreCard has no follower_count. Options: GetStore(productSeller.storeCode) when present / BatchGetStores(id) (no followers) / drop subscriber count from PDP. Rating: StoreCard/GetStore give rating_average+review_count, today read from REST `/reviews/stats` — keep REST or switch?
- **Q6 Prize/description on storefront & card page.** B3 removes description/prize/certified from *list cards*. The storefront header (SellerProfileV2) and `/sellers/[id]/card` read them from PublicStore (fields exist) — confirm B3 does not apply there. `certified` has no source anywhere now (storefront doesn't show it today, card list did).
- **Q7 Product/cart seller blocks without store_code.** MarketProductCard.tsx:215 and SellerGroupCard.tsx:156 fall back to id when the legacy product/cart wire lacks `storeCode`. With the fallback deleted: render no link, or BatchGetStores per page of products/cart groups? Does the staying product REST now always embed store_code (every exposable store has one)?
- **Q8 Location.** All `city`/`district` strings become `town_code`; website has `lib/addresses/twaddr.ts` (`townByCode`/`countyByCode`) so it is resolvable, but StoryRail's 附近商家 sorting and origin-map grouping use city names / `seller_city_code` (legacy city code space, `/public/seller-cities/summary`) — mixed code spaces?
- **Q9 App deep link.** `hawkr://seller/{segment}` (2 sites) will carry store_code; does buyer-app's handler accept store_code?
- **Q10 Home bootstrap DTO.** Dropping `marketplace` from `HomeTabBootstrapV1` (`schemaVersion: "home.v1"`) and its degraded label; hot products become a new live-wave read (`/public/products?sort=SALES&page_size=8`). Bump schemaVersion or just change (pre-launch, no compat)?
- **Q11 Cache placement for Connect reads.** GetStore/BatchGetStores/SuggestRecommendedSellers server reads need unstable_cache wrappers with strict/softFail semantics equivalent to `getCachedSeller` (`prerenderSafe`, outage throws, not_found cacheable). Review/follow changes: does review/follow writing bust `seller:<code>` too? Monolith seller module sends both tags; legacy review-service sends only `seller:<id>` (backend/pkg/revalidate) — a store_code-keyed entry would miss review-count busts until TTL.
- **Q12 B6 placement.** No "more" menu exists today on the storefront (SellerProfileV2 actions: chat/share/subscribe) or PDP — the menu itself is new UI; only `components/ui/Dropdown.tsx` exists to reuse. Where does the anonymous 食安通報 form live (dialog on storefront/PDP vs. own page), and CreateReport's evidence images (`evidence_staged_tokens`) — which staged-upload flow? Both RPCs need `store_id` (= PublicStore.id / StoreCard.id), available after GetStore.
- **Q13 ThreadClient seller-side.** ThreadClient fetches the public seller only for buyer→seller; BatchGetStores returns closed/suspended stores too — PAUSED/CLOSED header copy in chat follows B11?

---

## 8. UX approval notes (root AGENTS.md: layout/copy/flow need approval)

Already approved:
- B11 card states (OPEN link via store_code; PAUSED name+「暫停營業」, no link; CLOSED「已關閉」, no link; name unset「已關閉的店家」) — applies to Favorites, Recent, member cards, chat header/context panel, origin preview, home rail (recommended are always OPEN).
- B3 removal of description/prize/certified from card UI (home rail `HomeCompactMerchants`, `toSeller` location label 「實名認證 / 本週推薦」 homeSnapshot.ts:304, desc fallback copy :311-313).
- B6 「檢舉商家」 entry (logged in, CreateReport) and anonymous food-safety form (SubmitFoodSafetyReport).
- store_id → store_code redirect deletion, sellerPath fallback deletion, real 404 kept.

Still needs Will (layout/flow not covered by the above):
- Creating the "more" menu itself on storefront header and PDP (doesn't exist), report dialog/form layout and copy, where the food-safety form lives, success/duplicate copy (already_exists for CreateReport; identical response for food safety).
- PDP seller intro if subscriber count/rating source changes or disappears (Q5).
- Storefront/card page treatment of prize/description if B3 is meant to extend there (Q6); `certified` badge loss on home rail is covered by B3.
- 404 for paused/non-exposable storefronts (Q2) and seller preview behaviour change (Q3).
- Location text format if town names replace free-text city/district (e.g. 「臺北市 大安區」 from twaddr vs. seller-entered) — likely contract-following, flag only.
- Recommended rail order change if client re-ranking is dropped (Q4) — behaviour, not layout.

Contract-following (no approval needed): swapping fetchers to Connect, `uri_template` images, ProductViewTracker split, marketplace decomposition with identical rendered output, test/mock rewrites.

---

## 9. Counts

- Deleted-route fetch sites in production code: **6** (lib/api/index.ts ×4 routes in 4 functions, canonical-redirect.ts ×1, ProductViewTracker ×1) + **4** cached wrappers. Zero consumers: `/rating`, `/violations/public`, `/food_safety_reports`.
- Consumers of deleted repo functions: getCachedSeller **14**, fetchSeller (client) **3**, seller stats **4**, marketplace **6**, recommended **6** → **33**.
- React Query keys tied to deleted routes: **2** (`sellers.detail`, `feed.recommendedSellers`).
- `sellerPath` calls with an id fallback: **9**; `sellerCanonicalUrl(…, id)`: **7**; dead `sellerCardPath`: 1; dead `MarketSellerCard` component: 1.
- Route-segment-as-store-id passes to staying REST / follow: **~10** (StorefrontPage, SellerProductsClient, SellerPostsPage, SellerReviewsPage, ReviewsClient, SellerMessagesPage, SellerMessageTab, SellerProfileV2 review stats + subscribe, preview layout).
- Other numeric-id public URLs: deep link ×2, share card fallback ×1, seller preview URL ×1, middleware 308 ×1.
- Test/e2e/SEO files touching scope: **~33** (unit/contract ≈19, e2e ≈10, seo ≈5).
- Gaps/design questions: **13**.

---

## Evidence (commands run; `W` as above)

```
rg -n --glob '!node_modules' --glob '!.next' -e '/public/sellers' -e '/marketplace' -e 'sellers/recommended' -e '/view\b' -e "products/\$\{[a-zA-Z]+\}/view" -e 'food_safety' -e 'violations/public' $W
rg -n --glob '!node_modules' --glob '!.next' -w -e getCachedSeller -e fetchSeller -e getCachedSellerStats -e fetchSellerStats -e getCachedMarketplace -e fetchMarketplace -e getCachedRecommendedSellers -e fetchSellers -e MarketplaceSeller -e MarketplaceData -e MarketplacePost -e SellerStats $W
rg -n 'rpc ' api/hawkr/seller/v1alpha1/*.proto api/hawkr/discovery/v1alpha1/*.proto
rg -n 'message (GetStoreRequest|PublicStore|StoreStats|BatchGetStoresRequest|RecordProductViewRequest)' -A30 api/hawkr/seller/v1alpha1/seller.proto
rg -n 'message StoreCard' -A40 api/hawkr/common/v1alpha1/*.proto ; rg -n 'message Image ' -A14 api/hawkr/common/v1alpha1/*.proto
rg -n --glob '!node_modules' --glob '!.next' -i -e SellerPublicService -e DiscoveryPublicService -e SellerReport -e CreateReport -e SubmitFoodSafetyReport -e RecordProductView -e BatchGetStores -e 'GetMyStore\b' -e '檢舉' -e '食安' -e 'food.?safety' $W
rg -n --glob '!node_modules' --glob '!.next' --glob '!docs/**' -i -e 'reportSeller|ReportDialog|report-seller|/reports\b|檢舉商家|舉報' $W
rg -n ... -e 'sellerPath\(' -e 'sellerCardPath\(' -e 'sellerUrl\(' -e 'sellerCardUrl\(' -e 'sellerCanonicalUrl\(' -e '`/sellers/\$' -e '"/sellers/"' -e 'seller-center/preview/\$' $W   (non-test)
rg -n --glob '!node_modules' --glob '!.next' -il 'town_?code|twaddr' $W ; rg -n '^export' $W/lib/addresses/twaddr*.ts
rg -n -i 'hot|latest|recommend|Sort' backend/gateway/internal/handler/marketplace.go
rg -n --glob '!node_modules' --glob '!.next' -e 'marketplace\b' -e 'hotProducts' -e 'latestPosts' -e 'recommendedSellers' $W
rg -n --glob '!node_modules' --glob '!.next' -e 'products/[^"`]*/view' -e 'ProductViewTracker' $W/tests $W/features $W/lib
rg -n --glob '!node_modules' --glob '!.next' -e '/rating\b' -e 'violations/public' -e 'food_safety_reports' $W
rg -n --glob '!node_modules' --glob '!.next' -l 'StoreCardStatus|STORE_CARD_STATUS|已關閉的店家' $W      # → no output
rg -n --glob '!node_modules' 'import MarketSellerCard|MarketSellerCard"' $W                               # → type imports only
rg -n --glob '!node_modules' 'sellerCardPath\(' $W --glob '!tests/**'                                     # → definition only
rg -n 'seller:|SellerTag|StoreTags' backend/internal/platform/revalidate/revalidate.go backend/pkg/revalidate/tags.go
rg -n 'sellers' $W/lib/query/keys.ts ; rg -n 'queryKeys\.sellers\.|queryKeys\.feed\.recommendedSellers' $W
rg -n --glob '!node_modules' --glob '!.next' 'hawkr://seller' $W
rg -c (deleted-route + function patterns) $W/tests $W/features $W/lib $W/components $W/app $W/stories   # test file list in §6
```

---
# A3b 10d — buyer-app 盤點（公開讀取改 Connect）

HEAD af1b0dcc7（main），唯讀。路徑相對於 `buyer-app/`，另有註明者除外。

## 0. 可用的 Connect client（packages/hawkr_api）

- `seller/v1alpha1/seller.connect.client.dart`：`SellerPublicServiceClient.getStore`（:336）、`batchGetStores`（:357）、`recordProductView`（:397）——三支都已生成。
- `discovery/v1alpha1/discovery.connect.client.dart:17` `suggestRecommendedSellers`。
- `seller/v1alpha1/report.connect.client.dart:15` `SellerReportAppService.createReport`（登入），`:56` `SellerReportPublicService.submitFoodSafetyReport`。
- `common/v1alpha1/store_card.pb.dart`：StoreCard／StoreCardStatus。
- buyer-app 現行的接法：`lib/core/networking/connect_provider.dart` 提供 `connectTransportProvider`（公開）與 `connectAuthTransportProvider`（登入）；只有 content／identity 有具名 provider，其他 repository 自己建 client（例如 `product_repository.dart:15-18` `BuyerAppServiceClient(ref.watch(connectAuthTransportProvider))`、`follow_repository.dart:14-17`）。Seller／Discovery 的公開 client 要用 `connectTransportProvider`。
- buyer-app 已經用 `hawkr_twaddr`（pubspec:66；address、cart、profile_edit 在用），有 `townByCode`、`countyByCode`、`formatAddress`。

## 1. 被刪除路由的呼叫點

### 1a. GET /public/sellers/{id}

| # | file:line | 現在做什麼 | 替代 |
|---|---|---|---|
| 1 | `lib/features/seller/data/seller_repository.dart:55` `getMerchantInfo(String)` | 原始 envelope Map，下面 9 個消費端都經過它 | 刪除，依消費端拆開（見下） |
| 1.1 | `lib/features/seller/presentation/merchant_screen.dart:144` | 店面頁：店名、city/district/addressDetail/address/postalCode、description、coverKey、logoKey、prize、storeId（:258 回填 `_resolvedMerchantId`）、rating、`overallCertificationStatus`（:265 → `_isCertified` :330） | `GetStore(store_code)` → PublicStore＋StoreStats；`PublicStore.id` 給仍走 REST 的路由、追蹤、聊天、檢舉使用 |
| 1.2 | `lib/features/product/presentation/merchant_item_detail_screen.dart:469` | 商品詳情的「產地」退路：商品 payload 的 seller 沒有 city/district 時，用 storeId 再讀 city+district（`_buildSellerLocation` :349） | 改讀商品 REST 的 `seller.city/district`（gateway `buildSellerInfoItem` 有）；或用 `BatchGetStores([id]).town_code` → twaddr |
| 1.3 | `merchant_item_detail_screen.dart:1992` `_MerchantInfoCardNew._loadData` | 商品詳情的商家卡：logoKey、storeName、rating | `BatchGetStores([storeId])` → name、logo、rating_average（＋status、store_code，給「進店」按鈕使用）；或 name/logo 直接用商品 payload，只有評分需要 RPC |
| 1.4 | `lib/features/notification/presentation/notifications_screen.dart:59`（key 由 `:822-840` `_senderLookupKey` 建立） | 通知寄件者名稱與頭像；id 從 `sellerId/seller_id/sellerAccountId/merchantAccountId/storeId/store_id...` 擇一取 | `BatchGetStores`；但 key 混了 account id（見 G7） |
| 1.5 | `lib/features/chat/presentation/chat_thread_screen.dart:456` `loadMerchantProfile` | 聊天室標題的店名與 logo | `BatchGetStores([id])`，或由對話列表（REST 已帶 `storeName`/`logoKey`/`storeCode`）經 extra 傳入；id 的語意要確認（G8） |
| 1.6 | `lib/features/social/presentation/post_detail_screen.dart:107` | 貼文詳情的店名與頭像沒有傳進來時補讀 | 貼文 REST 已帶 sellerName/logo；剩下的空值改 `BatchGetStores` |
| 1.7 | `lib/features/settings/presentation/my_subscription_screen.dart:478` `_getMerchantInfoCached` → `:589`（追蹤）、`:754`（收藏）、`:959`（瀏覽紀錄） | 逐店 N+1：logo、店名、city+district(+addressDetail)、rating | 全部可以刪：`buyer.Follow.store`、`ProductCard.store` 已經是 StoreCard（name、logo、town_code、review_count、rating_average、status、store_code）。不需要新 RPC |
| 2 | `lib/features/category/data/category_repository.dart:408` `fetchSellerInfo` → `lib/features/category/presentation/category_screen.dart:359` `_fetchSellerInfoForProducts`（:345，快取 `_sellerCache` :80，讀取於 :865） | 市集商品列表逐店補店名與 logo（`product_card.dart:30-37` 註解已說明 /public/products 本身就成對帶 sellerName＋sellerLogoKey） | 刪除；直接用 `ProductItem.sellerName/sellerAvatar` |
| 3 | `lib/features/social/data/social_repository.dart:128` `_hydrateSellers`（由 `getPosts` :66 呼叫） | 貼文沒有 sellerName 時逐店補 | 刪除，或改 `BatchGetStores`（一次批次，≤100） |
| 4 | `lib/features/seller/presentation/widgets/merchant_chat_tab.dart:246` `_ensureUserProfile` | 用**留言者的 userId**（:468 `m['userId']`）打 `/public/sellers/$uid`，鍵就錯了（account id ≠ store id）：現有的 bug | 刪除；留言者名稱與頭像由 forum record 本身提供（`test/shared/utils/avatar_wire_test.dart:11`） |

### 1b. GET /public/sellers/{id}/rating

| file:line | 現在 | 替代 |
|---|---|---|
| `seller_repository.dart:74` `getRatingStats(int)` → `my_subscription_screen.dart:654`（追蹤卡補評論數） | `RatingStats`（freezed，`seller_model.dart:6-27`） | `Follow.store.review_count/rating_average`；刪除 `RatingStats`（改 freezed 要跑 build_runner） |
| `merchant_screen.dart:160`（dio 直接打） | 店面的 rating＋reviewCount | `GetStore.stats.rating_average/review_count` |

### 1c. GET /public/sellers/{id}/stats

| file:line | 現在 | 替代 |
|---|---|---|
| `merchant_screen.dart:165` | followers、totalProducts（存在 `_cachedTotalProducts` :201，**畫面上沒有顯示**） | `GetStore.stats.follower_count`；totalProducts 直接刪（沒人讀）。brief 說「totalProducts 改從 /products 的 total 取」：目前沒有任何 UI 顯示它，不需要替代 |
| `lib/features/follow/data/follow_repository.dart:69` `getFansCount` | **零消費端**（`merchant_icon_policy_test.dart:141` 斷言店面頁不呼叫它） | 刪除 |

### 1d. GET /public/marketplace

| file:line | 參數／讀取 | 消費端 | 替代 |
|---|---|---|---|
| `category_repository.dart:262` `fetchRecommendedSellers` | seller_limit=10，讀 `recommendedSellers[].storeId/storeName/logoKey/city/district` | `category_screen.dart:200` → `_MarketSellerRail`（:738），點擊 `:740` `/merchant/${seller.id}` | `DiscoveryPublicService.SuggestRecommendedSellers(max_results:10)` → `RecommendedSeller.store`（一律 OPEN）。rail 只讀 id/name/avatar（city/district 沒有顯示） |
| `category_repository.dart:320` `_fetchMarketplaceRecommendations` | product_limit=12，`hotProducts` | `category_marketplace_controller.dart:22,30,36` → `category_landing_screen.dart:897-922` | 既有 REST `GET /public/products?sort=SALES&page_size=12`（gateway 的 marketplace 內部就是 `GetPublicProductList(... "sales_desc")`，`backend/gateway/internal/handler/marketplace.go:116`）。`CategoryMarketplaceProduct.fromJson` 的欄位與 ProductItem 相同 |
| `lib/features/product/presentation/search_screen.dart:504` `_loadRecommendedProducts` | product_limit=12，`hotProducts` | 搜尋頁「推薦商品」 | 同上，`/public/products?sort=SALES` |

latestPosts：buyer-app 從來沒有向 marketplace 要過貼文（`post_limit:0`），不需要組合。

### 1e. POST /api/v1/products/{id}/view

零呼叫點。商品詳情 `merchant_item_detail_screen.dart:167` `_loadProductDetail().then(_recordProductVisit)`；`:170-176` 訪客直接 `return`，登入後走 `ProductRepository.visitProduct`（`product_repository.dart:260`，Connect `BuyerAppService.RecordVisit`）。
→ 10d 是**新增**：在 `:173` `if (!hasUsableSession(session))` 分支呼叫 `SellerPublicService.RecordProductView(product_id: widget.skuId)`，best-effort、吞錯（含 resource_exhausted）。

### 1f. 零消費端

`/public/sellers/{id}/violations/public`、`POST /public/food_safety_reports`、`violation_reports`：lib/test 都沒有命中。現在也沒有任何「檢舉商家」入口（`ReportTargetType` 只有 review/post/postComment/sellerForum/chatMessage/user，`lib/core/codemeta/code_meta.dart:4432-4442`）。

## 2. 以數字 store_id 導航的地方

路由：`lib/core/routing/app_router.dart:167-176` `GoRoute('/merchant/:sellerId')` → `MerchantScreen(sellerId:int?, sellerIdStr)`（`merchant_screen.dart:27`）。存取政策：`route_access_policy.dart` 不需要登入（沒有 /merchant 前綴規則，改名不影響）。

| # | file:line | id 來源 | 來源是否已經帶 store_code |
|---|---|---|---|
| N1 | `merchant_item_detail_screen.dart:2046` `enterStore` | `widget.merchantId`（商品 REST `seller.storeId`） | 商品 REST `seller.storeCode` 有（gateway `product.go:581`），buyer-app 沒解析 |
| N2 | `lib/features/social/presentation/merchant_posts_feed_screen.dart:798` | post `sellerId` | 貼文 REST 有 `storeCode`（gateway `post.go:1163`）；`social_model.dart:15` 沒解析 |
| N3 | `post_detail_screen.dart:232` | 同上 | 同上 |
| N4 | `lib/features/cart/presentation/widgets/merchant_group_card.dart:168` | cart `group.storeId` | cart REST `storeCode`（gateway `cart.go:60`）；沒解析 |
| N5 | `category_screen.dart:740` 推薦 rail | marketplace `storeId` | 改走 SuggestRecommendedSellers 後，`store.store_code` 有 |
| N6 | `lib/features/wallet/presentation/wallet_screen.dart:772` 會員卡頭像 | `card.sellerId` = `MemberCard.store_id`（`member_card_repository.dart:32`） | `MemberCard.store.store_code` 有，repository 沒映射 |
| N7 | `lib/features/order/presentation/order_detail_screen.dart:727` `_goToStore` | `order.storeId` | order REST `storeCode,omitempty`（gateway `order.go:58,148`）；沒解析 |
| N8 | `lib/features/order/presentation/order_card.dart:172` | `order.storeId` | 同上 |
| N9 | `my_subscription_screen.dart:1523` 追蹤卡（`canOpenMerchant = int.tryParse(merchantId) != null` :1521） | `Follow.store_id` | `Follow.store.store_code` 有；`_followingItem` :483 沒放 |
| N10 | `my_subscription_screen.dart:1796` `_openMerchant`（← :1981 收藏／瀏覽卡；:1979 `int.tryParse` 守衛） | `ProductCard.store_id` | `_favoriteItem` :511、`_visitItem` :531 **已經放了 `storeCode`**，只是沒用 |

分享連結的退路（伺服器 `/app/share_url` 失敗時）：`merchant_posts_feed_screen.dart:356`、`post_detail_screen.dart:198` 組成 `https://example.com/sellers/$merchantId/posts#post-…`，用的是數字 id。website 的路由是 `app/(public)/sellers/[id]/...`；ADR 2026-10-03 規定公開網址的鍵是 store_code，store_id → code 的轉址在步驟 10 刪除，所以這兩個退路改用 storeCode。

沒有命中：推播點擊導向店面（`notification/data/push/router.dart` 沒有 seller 目的地）；平台 app link（Android manifest 沒有 intent-filter host，iOS 只有 `hawkrbuyer` scheme，只給付款 webview 使用）；analytics 只送店名（`core/analytics/ga4_item.dart:50,61`），沒有 store id。

店面內部仍需要數字 id（從 `GetStore.store.id` 取得）：`merchant_shop_tab.dart:193`（`/public/sellers/$id/products`）、`merchant_screen.dart:422` 預先載入、`merchant_home_tab.dart:132`（posts）、`merchant_chat_tab.dart:162,197`（forum）、追蹤 `checkFollow/follow`（`follow_repository.dart:35-51`，store_id）、聊天 extra `merchantId`（`merchant_screen.dart:528`）、B6 的 `CreateReport.store_id`。目前的 `_resolvedMerchantId` 回填機制（:258、:314-320 → `_startShopPrefetch(force:true)`）可以沿用：route 參數改成 code，id 改由 GetStore 回填。

## 3. 推薦／市集的組合

- 推薦店家：`SuggestRecommendedSellers` 取代 `fetchRecommendedSellers`；`MarketSellerPreview`（`category_repository.dart:11-27`）改成帶 storeCode；city/district 欄位在 rail 沒有顯示，可以刪，也可以改 town_code。
- 推薦商品（2 處）：`/public/products?sort=SALES`。
- 產地城市（`category_repository.dart:174-215` `fetchOriginCities`）走 `/public/products` 的 `seller.city`，不在 10d 範圍；`taiwan_origin_picker_sheet.dart:401-430` 的店家列不能點。

## 4. 商品瀏覽

見 1e。只有一處要新增：`merchant_item_detail_screen.dart:173`。

## 5. 店家卡片（B3／B11／B19）

| 卡片 | file:line | 現在讀的欄位 | StoreCard／PublicStore 供給 |
|---|---|---|---|
| 追蹤卡 | `my_subscription_screen.dart:1513-1530`（`_buildFollowingCard`），資料 `:483-490`＋補讀 `:575-665` | name、avatar、address（city+district+addressDetail）、rating、reviewCount、merchantId | name/logo/town_code/review_count/rating_average/status/store_code 都有；addressDetail **沒有**（B3：所在地只用 town_code） |
| 收藏卡 | `:498-520`＋`:740-775` | sellerName、logo、storeCode、`distance`＝city+district（:765） | `ProductCard.store` 全有；town_code → twaddr |
| 瀏覽紀錄卡 | `:522-540`＋`:950-980` | 同上，`distance`（:966） | 同上 |
| 會員卡 | `member_card_repository.dart:30-43`、`wallet_screen.dart:560-580,772` | name、logo、cover | StoreCard 全有，沒有映射 status/store_code（B11 狀態要套用） |
| 推薦 rail | `category_screen.dart:995-1060` | id、name、avatar | `RecommendedSeller.store`（一律 OPEN） |
| 商品詳情商家卡 | `merchant_item_detail_screen.dart:1975-2050` | logo、name、rating | `BatchGetStores` |
| 店面 header | `merchant_screen.dart:546-565` → `merchant_header.dart` | banner、avatar、name、address、location、description、award(prize)、rating、ratingCount、followerCount、isCertified | PublicStore：cover、logo、name、town_code、description、prize；StoreStats：rating/review/follower（optional）。address（addressDetail／完整地址／郵遞區號）**沒有**；isCertified 沒有（B19 刪除） |

- B19：徽章在 `merchant_header.dart:495-520`（`Key('merchant_certification_badge')`），資料來自 `merchant_screen.dart:265,330,561`（brief 寫的 `:265` 是 `certStatus` 的讀取；徽章 widget 在 header :495）。
- B11 的套用點：N9／N10（追蹤、收藏、瀏覽）、會員卡（N6）；目前的守衛是 `int.tryParse(merchantId) != null`（:1521、:1979、:1795），要改成 `status == OPEN && store_code != ''`。

## 6. 測試／mock

| file:line | 內容 | 處置 |
|---|---|---|
| `test/integration/hermetic_app_smoke_test.dart:43,69-73` | 允許清單＋mock `/public/marketplace`（recommendedSellers/hotProducts） | 改 mock `/public/products?sort=SALES`＋Connect SuggestRecommendedSellers（目前 hermetic 測試對 Connect 的 mock 方式要確認） |
| `test/features/category/category_marketplace_repository_test.dart:44-190` | `fetchMarketplaceRecommendations` 的快取、併發、路徑 | 改路徑、斷言 sort 參數 |
| `test/features/category/category_marketplace_section_test.dart:51-173` | 同上（section） | 同上 |
| `test/features/category/category_repository_filter_contract_test.dart:172-201` | `fetchRecommendedSellers(limit:6)` 的契約 | 改成 Discovery client 的測試 |
| `test/features/category/category_screen_filter_test.dart:100-102` | mock recommendedSellers | 改 |
| `test/features/product/search_screen_layout_test.dart:139-146` | mock hotProducts | 改 |
| `test/features/settings/my_subscription_social_screen_test.dart:29-53,413-464,501,548` | mock `SellerRepository.getMerchantInfo/getRatingStats`；fixture `storeCode:'42'` | 刪 mock，fixture 改用 StoreCard 欄位，加 B11 狀態案例 |
| `test/features/moderation/frosted_more_entry_points_test.dart:65` | mock `/public/sellers/7` | 刪除該行（商品詳情商家卡改 BatchGetStores 後要 mock Connect） |
| `test/features/seller/merchant_icon_policy_test.dart:136-148` | 斷言 `'/public/sellers/$currentId/stats'` 恰好出現 1 次 | 改成斷言改用 GetStore／不打 stats |
| `test/features/seller/merchant_layout_widget_test.dart:181` | 量 `merchant_certification_badge` 的尺寸 | B19 刪除 |
| `test/core/routing/redirect_logic_test.dart:142`、`route_access_policy_test.dart:9` | `/merchant/2683382637` 數字 | 改成 store_code 範例 |
| `test/features/social/post_detail_screen_test.dart:43,108-110,309`、`test/features/order/order_card_redesign_test.dart:44-47,274` | 自建 `/merchant/:sellerId` 路由，斷言 `'店舖 7'` | 改成斷言 store_code |
| `test/architecture/dead_gateway_endpoint_guardrail_test.dart:12-31` | 死端點清單 | 新增 `/public/sellers/$x'`（裸 id）、`/rating`、`/stats`、`/public/marketplace`，防止回流（注意不能誤擋 `/products`、`/posts`、`/forum`、`/reviews`） |

## 7. 缺口／設計問題（不在此解決）

- **G1 route 參數**：`/merchant/:sellerId` 改成 `/merchant/:storeCode`（或改名成 `/store/:code`）。改名會動到路由結構，buyer-app AGENTS.md 要求先確認。MerchantScreen 的建構子 `sellerId:int?/sellerIdStr` 要換掉。
- **G2 legacy REST 的 `storeCode` 是否等於 Connect 的 `store_code`**：N1/N2/N4/N7/N8 要靠 gateway REST 回的 `storeCode`。order 的 `storeCode` 是 order-service 的快照（`order.go:56` 註解），店家重新入駐（新 stint → 新 code，ADR 2026-10-03 :43）之後，舊訂單的 code 會導到 not_found。另外 order 是 `omitempty`、商品 `seller` 可能是 nil：code 空的時候怎麼處理（不可點，或用 `BatchGetStores([store_id])` 解析）？測試 fixture 的 storeCode 都等於數字 id（`2685914682`），看不出真實形狀。
- **G3 store_code 空值或店家非 OPEN 時的進店行為**：REST 來源（訂單、購物車、貼文、優惠券）沒有 StoreCard status。brief 決議「只有 store_id 的來源經 BatchGetStores 解析」：這些來源每次點擊前都打一次 BatchGetStores？還是列表載入時批次打？
- **G4 店面 header 的地址**：PublicStore 只有 town_code，沒有 addressDetail／完整地址／郵遞區號。`merchantAddress` 會從「縣市區＋詳細地址」縮成「縣市區」。B3 只講到卡片，店面 header 也這樣改嗎？
- **G5 StoreStats 欄位沒設時的顯示**：follower_count／review_count／rating_average 讀取失敗時是 unset（不是 0）。現在的 UI 一律顯示數字（失敗就顯示 0）。unset 時 header 要隱藏還是顯示「—」？
- **G6 店面 not_found**：GetStore 對不可曝光的店一律回 not_found。現在的空狀態文案是「商家可能已下架或暫停營業」（`merchant_screen.dart:~500`），可以沿用；但從 B11 PAUSED 卡片（不可點）以外的入口（訂單、購物車）點進來也會落到這裡。要不要先用 StoreCard status 擋掉？
- **G7 通知寄件者 key 混用 account id**：`notifications_screen.dart:827-838` 的 key 依序取 `sellerAccountId/merchantAccountId/...storeId`。BatchGetStores 只吃 store id；推播 payload 實際帶哪個鍵，要由通知 SSOT 確認。
- **G8 聊天室 `merchantId` 的語意**：`chat_thread_screen.dart:456` 用它打 `/public/sellers/{id}`，同時又拿它當 `participantId`（:477）。對話 REST 的 `targetUserId` 是 store id 還是 account id？（fixture 兩者相同）。對話列表已經帶 `storeName/logoKey/storeCode`，可以經 extra 傳入，省掉 RPC。
- **G9 B6 檢舉商家沒辦法直接重用 `showReportSheet`**：`report_sheet.dart:271` 的鍵是 `ReportTargetType`＋CodeMeta `ReportReasonCode`（8 個值，走 `POST /app/reports`，`social_repository.dart:534-578`，description 可以不填，不能附證據）；`CreateReport` 要 `store_id`＋`ViolationCategory`（7 個值，`seller.proto:851-860`）＋**必填** description（≤2000）＋選填 product_id／order_no／evidence_staged_tokens（≤5；buyer-app 有 `MediaRepository` 的 stagedToken 上傳）。重用的方式要決定：同一個 sheet 加 variant，還是另做一張共用版面的 sheet；buyer-app 也還沒有 `SellerReportAppServiceClient` 的 provider。
- **G10 B6 的入口位置**：店面 header（`merchant_header.dart:191-285`，只有返回、購物車、聊天三個按鈕）和商品詳情頁層級（只有分享按鈕；`:2562-2577` 的 `moreHoriz` 是評論列上的）**現在都沒有「更多」選單**。B6 是要新增選單按鈕，還是附掛在既有的分享或其他控制上？商品詳情的檢舉可以順手帶 `product_id`。
- **G11 熱銷排序語意**：marketplace 內部送 `sales_desc`，REST `/public/products` 白名單是 `SALES`（`product.go:40-46`）。改用 REST 後兩者是否同義，要由 product 擁有者確認。
- **G12 沒在用的 REST 方法**：`SellerRepository.getSellerProducts`（`seller_repository.dart:23`）零消費端（路由保留，但方法是死碼）。10d 順手刪，還是留給 product 模組搬遷？
- **G13 Connect mock 的基礎設施**：hermetic smoke 與 widget 測試目前都用 Dio adapter mock；改成 SellerPublic／Discovery client 之後，要找出既有的 Connect fake 方式（例如 follow/buyer 測試怎麼做）。

## 8. UX 授權註記

已授權：
- B3：卡片拿掉 description/prize/certified，所在地改 town_code（追蹤、收藏、瀏覽、會員卡）。
- B11：四種狀態（OPEN 可點、PAUSED 店名＋「暫停營業」不可點、CLOSED「已關閉」不可點、name 未設「已關閉的店家」）。
- B19：刪掉店面的「已認證」徽章。
- B6：店面／商品「更多」加「檢舉商家」。

需要 Will 確認：
- G10：兩處現在都沒有「更多」按鈕，新增按鈕本身是版面改動（B6 的文字是否涵蓋新增按鈕？）。
- G9：檢舉 sheet 的原因清單（ViolationCategory 的 7 個中文標籤是新文案）、description 必填的提示、要不要附圖。
- G4：店面 header 的地址縮成縣市區。
- G5：統計 unset 時的顯示（隱藏或「—」）。
- G1：路由改名（buyer-app AGENTS.md：改路由結構前先確認）。
- G6：非 OPEN 店家從 REST 入口點進來的行為。

純契約跟隨，不需要授權：刪 N+1 補讀、改用 StoreCard 欄位、推薦改 Discovery、熱銷改 REST、RecordProductView、測試 mock 更新、刪 forum 錯鍵的查詢（bug）。

## 9. 數量

- 被刪路由在 lib 的請求點：11 處（`/public/sellers/{id}` 4：seller_repository:55、category_repository:408、social_repository:128、merchant_chat_tab:246；`/rating` 2；`/stats` 2；`/public/marketplace` 3），分布在 8 個檔案。
- 經 `getMerchantInfo` 的消費端：7 處呼叫（my_subscription 再分出 3 個 tab）；`getRatingStats` 1；`fetchSellerInfo` 1；`fetchRecommendedSellers` 1；`fetchMarketplaceRecommendations` 3（controller）＋1 個畫面（category_landing）。
- `POST /products/{id}/view`：0 處現有呼叫，新增 1 處。
- 零消費端的路由：violations/public、food_safety_reports、violation_reports。
- 數字 id 導航：1 個路由定義＋10 處 push＋2 處分享退路 URL。
- 店家卡片：7 類（追蹤、收藏、瀏覽、會員卡、推薦 rail、商品詳情商家卡、店面 header）。
- 要改的測試檔：15 個（含死端點 guardrail）。
- 設計問題：13 個（G1–G13）。

## 附錄：執行過的 rg 指令

```
rg -n "/public/sellers|public/marketplace|/view'|/view\"|/view\b|violation_reports|food_safety_reports|violations/public" lib test integration_test tool
rg -n "/sellers|marketplace|recommended" lib --type dart | grep -v "\.g\.dart\|\.freezed\.dart"
rg -n "products/\\\$|/view|recordView|recordProductView|RecordProductView|productView|viewCount" lib --type dart
rg -n "'/products/|\"/products/|hasUsableSession" lib test --type dart
rg -n "visitProduct|browse-history|visitMerchant|visits" lib --type dart
rg -n "class .*Client|Future<" <hawkr_api>/seller/... discovery/...   (generated client RPCs)
rg -n "service |rpc " api/hawkr/seller/v1alpha1/{seller,report}.proto api/hawkr/discovery/v1alpha1/discovery.proto
rg -n "sellerRepositoryProvider|getSellerInfo|getSellerRating|getSellerProducts|SellerRepository" lib test --type dart
rg -n "'/merchant/|\"/merchant/|/merchant/\\\$|/merchant/\\$\{" lib test --type dart
rg -n "MerchantScreen\(|'/merchant'|\"merchant/|merchant/\\\$|/merchant\?" lib --type dart
rg -n "deep|app_links|uni_links|appLinks|getInitialLink|share_url|getShareUrl" lib --type dart
rg -n "showReportSheet|createReport|CreateReport|SellerReport|檢舉|report_sheet|violation" lib --type dart
rg -n "food_safety|foodSafety|violation|ReportTargetType\.(seller|store|product)" lib test --type dart
rg -n "moreHoriz|moreVert" lib/features --type dart
rg -n -i "store_?code" lib test --type dart
rg -n 'json:"storeCode' backend/gateway/internal --type go
rg -n "StoreCard|store_id|store_code" api/hawkr/{buyer,content,discovery}/v1alpha1/*.proto
rg -n "_getMerchantInfoCached\(|_hydrateSellers\(|getSellerProducts\(" lib --type dart
rg -n "fetchSellerInfo|fetchRecommendedSellers|fetchMarketplaceRecommendations|invalidateMarketplaceRecommendations|MarketSellerPreview|CategoryMarketplaceProduct\b" lib --type dart
rg -n "public/sellers/...|/stats'|/rating'|public/marketplace|getMerchantInfo|getRatingStats|fetchRecommendedSellers|fetchSellerInfo|fetchMarketplaceRecommendations|recommendedSellers|hotProducts|overallCertificationStatus|merchant_certification_badge" test integration_test --type dart
rg -n "'/merchant/|merchant/:sellerId|MerchantScreen\(" test integration_test --type dart
rg -n "merchant|seller|store" lib/features/notification/data/push/*.dart
rg -n "seller|store|merchant" -i lib/core/analytics
rg -n "TotalProducts|totalProducts|productCount" lib/features/seller --type dart
rg -n "hawkr_twaddr|hawkr_api" pubspec.yaml
rg -n -i "staged|StageUpload|stagedToken" lib --type dart
rg -n "productSortAllowed = " -A 10 backend/gateway/internal/handler/product.go
```
