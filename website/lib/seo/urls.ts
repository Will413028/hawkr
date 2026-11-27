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
 * SELF-canonical URL for a seller route, from the store `getCachedStore` returned
 * plus the segment the request arrived on: the payload's `storeCode`, which is
 * the segment `GetStore` was asked for. It is one function so the field is read
 * in one place a test can pin.
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
 * Root-relative storefront path for an INTERNAL link (`<Link href>`), built from
 * the seller's `store_code`. Empty when there is none: a store_id is not a
 * storefront URL segment, so a payload without a code produces no link rather
 * than one that answers 404. Callers render the seller without a link then.
 */
export function sellerPath(
	storeCode: string | number | null | undefined,
	tail = "",
): string {
	const segment = String(storeCode ?? "").trim();
	if (!segment) return "";
	return `/sellers/${encodeURIComponent(segment)}${tail}`;
}

/** Root-relative counterpart of sellerCardUrl() for internal links. */
export function sellerCardPath(
	storeCode: string | number | null | undefined,
): string {
	return sellerPath(storeCode, "/card");
}
