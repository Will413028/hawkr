# GA4 Analytics Events — Reference

Living reference for the GA4 ecommerce instrumentation on hawkr-website.
For the original design rationale see
`docs/superpowers/specs/2026-06-07-ga4-ecommerce-tracking-design.md` (removed from the repo; the original is in the private archive).

## Where the data goes (read this first)

Events are sent **client-side, directly to the Google Analytics 4 property**
via the single `next/script` Google tag loader in `Analytics.tsx`. They do **not** touch the
hawkr backend or any app database. To view them, use the GA4 web UI
(Reports / Realtime / DebugView) or GA4's BigQuery export — **not** the admin
dashboard. (The admin app reads the app backend, which never sees these
beacons. Surfacing analytics inside admin would require either embedding GA4
reports or building a separate server-side event pipeline — see Pending.)

## Gating (prod-only by default)

All dispatch goes through `lib/analytics/gtag.ts`, gated on
`env.gaId = NEXT_PUBLIC_GA_MEASUREMENT_ID` being non-empty.

- dev / stage leave the var **unset** → `isEnabled()` is false → every event is
  a no-op and the Google tag `<Script>` is not mounted.
- prod sets it → events fire.
- Local QA: set `NEXT_PUBLIC_GA_MEASUREMENT_ID=G-XXXX` in `.env.local`, run
  `pnpm dev`, watch GA4 DebugView, then unset (do not commit).

## The layer

```
lib/analytics/
  types.ts                # GA4Item, CURRENCY="TWD", AnalyticsRole
  gtag.ts                 # isEnabled / track / setUserProperties /
                          #   setConsent  (the only dispatch seam)
  items.ts                # domain → GA4Item mappers (price/id/variant rules)
  events.ts               # typed trackX() functions + buildPurchaseParams
  NavigationEvents.tsx    # SPA page_view on route change
  useAnalyticsIdentity.ts # auth → anonymous GA4 role-property bridge
  Analytics.tsx           # compose: next/script Google tag + page_view + role
  purchase-store.ts       # sessionStorage: cross-redirect orderNos + dedup
components/analytics/
  ViewItemTracker.tsx     # PDP view_item beacon (server builds the item)
  PurchaseTracker.tsx     # success-page purchase beacon (per-orderNo dedup)
```

## Event catalog (15 analytics events)

`currency` is always the constant `"TWD"`. `item_id` is always a **stringified**
int64 snowflake (precision-safe). `value` is numeric NT$.

| Event | Trigger (file) | Key params | Timing / dedup |
|---|---|---|---|
| `page_view` | `lib/analytics/NavigationEvents.tsx` | `page_path`, `page_location`, `page_referrer`, `page_title` | once per normalized pathname/search change; auth hydration and role switches do not retrigger it |
| `login` | `lib/api/auth.ts` (`completeSignIn`, after the Clerk OAuth redirect) | `method` (google/apple/line) | fresh login only — **not** session refresh |
| `view_item_list` | `features/market/components/MarketProductsList.tsx` | `items[]`, `item_list_id/name` | **client-only**, fires only the newly-appended slice on grow; resets on filter change |
| `select_item` | `features/market/components/MarketProductCard.tsx` | `items[1]`, `item_list_name` | on card click, before navigation |
| `view_item` | `app/[locale]/(public)/products/[id]/page.tsx` → `ViewItemTracker` | `items[1]`, `value` | once on mount (server-built item) |
| `add_to_cart` | `components/shared/AddToCartButton.tsx` | `items[1]`, `value=price×qty` | **after** server confirm + SKU resolve (not optimistic; no `CART_UPDATED` listener) |
| `remove_from_cart` | `features/cart/CartClient.tsx` | `items[1]`, `value` | user remove (per-tile + bulk "remove selected"). **Excludes** sold-out auto-remove. Save-for-later & clear-cart not counted (see Pending). |
| `view_cart` | `features/cart/CartClient.tsx` | `items[all]`, `value=cart total` | once on mount, after cart loads with ≥1 item |
| `add_to_wishlist` | `components/shared/FavoriteButton.tsx` | `items[1]` | add branch only, after server confirm |
| `begin_checkout` | `features/checkout/screens/CheckoutClient.tsx` | `items[]`, `value`, `coupon` | once after preview returns |
| `add_shipping_info` | `features/checkout/screens/CheckoutClient.tsx` | `shipping_tier`, `value`, `coupon` | once per distinct shipping method (when preview value known) |
| `add_payment_info` | `features/checkout/screens/CheckoutClient.tsx` (3 pay paths) | `payment_type` ∈ {CREDIT_CARD, ATM, APPLE_PAY, GOOGLE_PAY}, `value` | before each `createPayment` |
| `purchase` | `features/checkout/screens/CheckoutSuccessClient.tsx` → `PurchaseTracker` | `transaction_id=orderNo`, `value=totalActualAmount`, `items[]` | **one per `orderNo`** (multi-seller split); sessionStorage dedup survives the PayUni redirect and refresh. ATM `/checkout/submitted` does **not** fire (pending). |
| `generate_lead` | `features/seller/pricing/SellerPricingPage.tsx` | `lead_source=seller_pricing`, plan/billing/service categories | only after `/api/public/seller-leads` returns HTTP success and `{ok:true}`; no contact or brand fields |
| `seller_onboarding_completed` | `features/seller/onboarding/SellerOnboardingClient.tsx` | `completion_source=seller_self_service`, `seller_profile_action` | only after seller persistence and onboarding completion both succeed; custom event because an existing buyer can add a seller role without creating a new account |

## URL and referrer privacy

- `page_location` and `page_path` retain only reviewed categorical UTM values
  plus provider-shaped Google click IDs (`gclid`, `gbraid`, `wbraid`, `dclid`,
  `gad_source`). Free-form campaign/content/term values, contact-like values,
  `gad_campaignid`, unknown `utm_*` fields and all other query values are
  removed. Google Ads campaign attribution should use auto-tagging.
- Known account, seller and commerce identifiers in paths are collapsed to
  route templates before any event is dispatched.
- Known static routes are allowlisted. Unknown and 404 paths fail closed to
  `/:unknown`, so user-controlled path text cannot become analytics data.
- Same-origin referrers use the same sanitizer; cross-origin referrers retain
  only their origin.
- In GA4 Enhanced Measurement, only page loads and scrolls remain enabled.
  Browser-history page views, site search, outbound clicks, form interactions,
  video engagement and file downloads are disabled because those automatic
  events bypass the app sanitizer and can attach raw URLs.

## Item mapping (`items.ts`)

- `price`: numeric `priceValue` / `parsePriceValue()` — **never** the formatted
  `"NT$1,200"` string. Falsy (0) price is omitted.
- `item_id`: `productId` as string.
- `item_variant`: `skuId` (omitted in list context that has no SKU).
- `item_brand` / `affiliation`: seller name.
- `quantity`: numeric; default 1 for view_item(_list).

## Anonymous audience properties (`useAnalyticsIdentity.ts`)

- Stable Hawkr account identifiers are not sent as GA4 `user_id`.
- user property `role` = active role (buyer/seller), updates on role switch and
  resets to `anonymous` on logout.
- user property `login_method` = google/apple/line, set in `trackLogin`.

## Consent and advertising use

- The tag sets `allow_ad_personalization_signals=false` before initialization,
  so Analytics measurement is retained without sending remarketing signals.
- Google Signals stays on, which is what feeds the GA4 cross-device reports and
  the demographics/interests reports. It is a separate control from ad
  personalization: with `allow_ad_personalization_signals=false` the tag still
  syncs against `stats.g.doubleclick.net/g/collect`, and the enforced CSP was
  blocking exactly that request (backend-payuni#3231), so `connect-src`
  allowlists that one exact host. Measurement is kept, remarketing is not.
- No consent-management banner is implemented yet. `gtag.setConsent(granted)`
  is an update hook only; a regional/default-denied policy must be paired with
  a real consent UI and reviewed against the regions the service targets.

## Known limitations / open decisions

See the hawkr-website wiki `## Pending` for the live list:
- `remove_from_cart` does not count save-for-later or clear-cart.
- `purchase` fail-path does not clear pending sessionStorage (off-happy-path
  phantom risk only).
- Refunds are not emitted client-side; an authoritative refund event belongs in
  the backend/payment pipeline.
- Surfacing GA4 data inside the admin dashboard is a separate decision (GA4 data
  lives in Google's platform, not the app backend).
