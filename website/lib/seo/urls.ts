/**
 * Single source of truth for every URL the site emits (canonical, sitemap,
 * JSON-LD @id, og:url). Built from route params only — never from the request
 * URL — so tracking/query params can never leak into a canonical.
 *
 * Every ABSOLUTE-URL builder takes the locale of the page it describes, and it
 * is required — an omitted locale would silently describe the zh-TW page: a
 * page's canonical is its own locale's URL, never the zh-TW one (ADR
 * 2026-10-17 D1, as-needed prefixes — lib/intl/paths.ts `withLocale`).
 *
 * The root-relative builders for INTERNAL links (`sellerPath`, `sellerCardPath`)
 * take no locale: they return a locale-free path, and the navigation layer
 * (lib/intl/navigation.ts) adds the prefix of the language the visitor reads.
 */
import { getEnv } from "@/lib/config/env";
import {
	type AppLocale,
	defaultLocale,
	publishedFor,
} from "@/lib/intl/locales";
import { surfaceOfPath, withLocale } from "@/lib/intl/paths";
import { isPrivatePath, isUgcPath } from "./indexability";

/** Active site origin without a trailing slash, e.g. https://example.com. */
export function siteUrl(): string {
	return getEnv().webOrigin.replace(/\/$/, "");
}

/**
 * Absolute URL for a root-relative path in `locale`. `path` must start with "/".
 * The home path "/" returns the bare origin (no trailing slash); in a prefixed
 * locale it is the bare prefix (`<origin>/en`).
 */
export function absoluteUrl(path: string, locale: AppLocale): string {
	if (!path.startsWith("/")) {
		throw new Error(`absoluteUrl: path must start with "/" (got "${path}")`);
	}
	// The home path "/" maps to the bare origin (no trailing slash) so it matches
	// the root canonical Next.js actually renders — it strips the root slash, e.g.
	// https://example.com (not …/). Keeps canonical, og:url and sitemap aligned.
	const localized = withLocale(path, locale);
	if (localized === "/") {
		return siteUrl();
	}
	return `${siteUrl()}${localized}`;
}

/**
 * hreflang alternates for the page at `path` (`metadata.alternates.languages`):
 * one entry per locale PUBLISHED for the page's surface, plus `x-default` → the
 * default-locale URL of the same page. `undefined` — emit nothing — when there
 * is nothing to alternate between: a single published locale, a private page,
 * or a UGC page (its non-default versions are noindex, and an hreflang cluster
 * may only list indexable pages; ADR 2026-10-17 D1 "SEO").
 *
 * Reads the published list, not the routable one: a dev-only `/en` must never
 * be announced to crawlers. `published` is injectable for the tests.
 */
export function languageAlternates(
	path: string,
	published: readonly AppLocale[] = publishedFor(surfaceOfPath(path)),
): Record<string, string> | undefined {
	if (published.length < 2 || isPrivatePath(path) || isUgcPath(path)) {
		return undefined;
	}
	return {
		...Object.fromEntries(
			published.map((locale) => [locale, absoluteUrl(path, locale)]),
		),
		"x-default": absoluteUrl(path, defaultLocale),
	};
}

/** Canonical product-detail URL. */
export function productUrl(id: string, locale: AppLocale): string {
	return absoluteUrl(`/products/${id}`, locale);
}

/**
 * Canonical seller storefront URL. `tail` names a storefront tab
 * (`/reviews`, `/posts`, `/messages`); omit it for the products tab, which is
 * the index route. A tab passes its own suffix so it stays SELF-canonical —
 * pointing a tab at the index would ask search engines to drop that tab's URL,
 * which is a different decision from the store_id → store_code consolidation
 * these canonicals exist for.
 */
export function sellerUrl(id: string, locale: AppLocale, tail = ""): string {
	return absoluteUrl(`/sellers/${id}${tail}`, locale);
}

/**
 * Public one-page business card URL for a seller.
 *
 * Kept separate from the full storefront URL so QR codes and social-profile
 * links land on the compact identity + featured-products surface, while the
 * storefront remains the complete shopping destination.
 */
export function sellerCardUrl(id: string, locale: AppLocale): string {
	return sellerUrl(id, locale, "/card");
}

/**
 * SELF-canonical URL for a seller route, from the payload `getCachedSeller`
 * returned plus the id segment the request actually arrived on.
 *
 * WHY THE PAYLOAD AND NOT THE REQUEST. Deriving the canonical from `id` alone
 * is correct only while middleware has already canonicalized the request — and
 * middleware deliberately fails OPEN: a rate-limited, 5xx, timed-out or
 * unreadable lookup yields `{ kind: "unknown" }`, which passes the request
 * through untouched (`lib/seller/canonical-redirect.ts`). The page still
 * renders on that path, at the **store_id** URL, and a canonical read off the
 * request would then point canonical + og:url + JSON-LD at the non-canonical
 * id — the exact consolidation #365 exists to achieve, undone precisely when
 * the backend is unhealthy. The payload always knows the answer.
 *
 * WHY IT IS A FUNCTION AND NOT `sellerUrl(s.storeCode || id)` INLINE. The four
 * seller routes each spelled that expression out, and each of them spelled the
 * field `store_code` — the pre-backend#2235 name. `fetchSeller` does NOT rename
 * it (`lib/api/index.ts` keeps `storeCode` as it arrives; only `logo_url` is a
 * deliberate display alias), and the payload is loosely typed, so the misread
 * neither fails to compile nor throws: it resolves to `undefined` and falls
 * through to `id`, which LOOKS right on every canonicalized request. One place
 * to read the field is one place a test can pin.
 */
export function sellerCanonicalUrl(
	seller: { storeCode?: unknown } | null | undefined,
	id: string,
	locale: AppLocale,
	tail = "",
): string {
	const raw = seller?.storeCode;
	const code =
		typeof raw === "string" || typeof raw === "number"
			? String(raw).trim()
			: "";
	return sellerUrl(code || id, locale, tail);
}

/**
 * Root-relative storefront path for an INTERNAL link (`<Link href>`), built
 * canonical-first: the seller's `store_code` when the payload being rendered
 * carries one, otherwise whatever key the caller holds (`store_id` /
 * `account_id`).
 *
 * `fallbackId` is OPTIONAL and omitting it is the preferred call for any
 * payload whose owning service looks the seller up — those always carry the
 * code unless the seller block is missing wholesale, and a missing block
 * should produce no link rather than a slower one. Pass a fallback only when
 * the payload can legitimately hold a key without a code (see below).
 *
 * Linking a store_id is not broken — middleware answers it with a `308` to the
 * store_code (middleware.ts → lib/seller/canonical-redirect.ts) — but it makes
 * every tap pay for that redirect: a second navigation for the visitor, and
 * internal links that point away from the canonical URL for crawlers. That is
 * exactly the consolidation #365 exists to finish, so a seller href is built
 * here rather than interpolated at each call site.
 *
 * The fallback is correct rather than defensive: middleware only redirects a
 * key it could resolve a code for, so a seller with no store_code serves its
 * storefront from the store_id path.
 *
 * KEEP THE FALLBACK — but note what it is now FOR. Every surface that renders
 * a seller link from a payload whose owning service looks the seller up has
 * been taught to carry store_code (#2098 phase 3, then #2137: orders, coupons,
 * browsing history and im conversations all lift it off the GetSellerBatch /
 * batchGetSellerMeta response they already make for the store name and logo).
 * Those call sites pass ONLY the code and render nothing when it is absent.
 *
 * What remains is the payload that carries a seller key but no lookup at all —
 * recommended-seller rows, product wire seller blocks, legacy cart groups —
 * where the code can be genuinely missing from a row the caller must still
 * link. For those the fallback produces a WORKING link: `/sellers/{store_id}`
 * 308s to `/sellers/{store_code}` (verified on dev), so falling back costs one
 * redirect, not a dead link. That is correct handling of an absent value, not
 * an old-key compatibility layer, and only the latter would be debt.
 *
 * The bar for adding a surface to the canonical set is unchanged: "already
 * fetched, merely dropped" qualifies; "could fetch it if we paid for a
 * round-trip" does not. `/public/posts/{id}` is the one endpoint still on the
 * wrong side of that line — it makes no seller lookup — and no caller builds a
 * seller href from it today (the post viewer is always fed from a list).
 *
 * Empty when neither key is usable — callers that can be handed a seller-less
 * row must keep their own guard rather than render a link to `/sellers/`.
 *
 * ⚠️ POSTS ARE THE EXCEPTION, AND THEY MUST NOT PASS `fallbackId`. Everywhere
 * above, an absent `store_code` means "this seller has no minted code", and the
 * fallback yields a working 308. On `/public/posts` the gateway fills
 * `store_code` ONLY when `GetSellerBatch` found the seller, so absent means NOT
 * FOUND — `/sellers/{store_id}` then 404s, and the fallback manufactures a dead
 * link out of a value that was telling you not to link at all. `post.go`'s field
 * comment states it directly: 此時前端必須不渲染賣家連結，而不是退回 seller_id.
 * `tests/post-seller-link.test.ts` pins the post-facing callers to one argument.
 */
export function sellerPath(
	storeCode: string | number | null | undefined,
	fallbackId?: string | number | null,
	tail = "",
): string {
	const segment =
		String(storeCode ?? "").trim() || String(fallbackId ?? "").trim();
	if (!segment) return "";
	return `/sellers/${encodeURIComponent(segment)}${tail}`;
}

/** Root-relative counterpart of sellerCardUrl() for internal links. */
export function sellerCardPath(
	storeCode: string | number | null | undefined,
	fallbackId?: string | number | null,
): string {
	return sellerPath(storeCode, fallbackId, "/card");
}

/**
 * Returns the canonical `/sellers/{store_code}{tail}` path to redirect to when
 * `id` (the current request's path segment) isn't already the seller's
 * store_code — e.g. a legacy store_id link, or an internal href not yet
 * migrated. Returns null when no redirect is needed.
 *
 * `tail` is everything the request asked for after the id segment — a tab
 * (`/reviews`, `/posts`, `/messages`), a product detail path
 * (`/products/{pid}`), or "" for the products tab, which is the index route.
 * It is carried through verbatim: dropping it silently returned every deep
 * link to the products tab.
 *
 * Root-relative on purpose. An absolute URL sends the visitor to the configured
 * site origin, so a localhost or preview session gets thrown onto the deployed
 * host mid-navigation. Absolute seller URLs stay with `sellerUrl()`, which
 * feeds canonical/og:url/JSON-LD/sitemap — those do need the origin.
 */
export function canonicalSellerRedirectTarget(
	id: string,
	storeCode: string | null | undefined,
	tail = "",
): string | null {
	if (!storeCode || storeCode === id) return null;
	return `/sellers/${storeCode}${tail}`;
}
