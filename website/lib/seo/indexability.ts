/**
 * Single source of truth for "what may be indexed". Consumed by:
 *   - route-group noindex layouts (private de-indexing via <meta robots>)
 *   - app/sitemap.ts (exclusion)
 *   - middleware.ts (`/en` UGC pages answer `X-Robots-Tag: noindex`)
 *   - lib/seo/urls.ts `languageAlternates` (hreflang clusters list indexable pages only)
 *   - the SEO CI assertions in PR6 (sitemap purity, robots-vs-money-URL)
 * This module does NOT generate robots.txt Disallow rules — private pages stay
 * crawlable so Google can read their noindex tag (see lib/seo/metadata.ts).
 *
 * Every `pathname` here is locale-free (`/products/1`, never `/en/products/1`);
 * the locale is a separate argument. Imports are relative: next.config.ts loads this file.
 */
import { type AppLocale, defaultLocale } from "../intl/locales";

/** Path prefixes that must never be indexed (private / thin / transactional). */
export const PRIVATE_PREFIXES = [
	"/account",
	"/seller-center",
	"/seller-product-import-preview",
	"/seller/measures",
	"/checkout",
	"/cart",
	"/payment",
	"/store-map",
	"/login",
	"/register",
	"/search",
	"/orders",
] as const;

/** True if `pathname` is private (never indexable in any locale). */
export function isPrivatePath(pathname: string): boolean {
	return PRIVATE_PREFIXES.some(
		(prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
	);
}

/**
 * Seller-written pages: products, stores (and their tabs and posts) and the posts
 * feed. Their body is untranslated UGC, so a non-default locale's version is
 * "translating only the boilerplate" — Google's documented bad experience — and is
 * kept out of the index (noindex, no sitemap entry, no hreflang) until the content has
 * a translation. NOT UGC: the home page, `/market/products` (the category listing),
 * and the platform-written pages (FAQ, guides, legal). ADR 2026-10-17 D1 "SEO".
 */
export const UGC_PREFIXES = ["/products", "/sellers", "/feed"] as const;

export function isUgcPath(pathname: string): boolean {
	return UGC_PREFIXES.some(
		(prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
	);
}

/**
 * True if `pathname` is kept out of the index only because of its locale: a
 * non-default locale's UGC page. (Private pages are noindex in every locale
 * through their own layouts; this is the part the locale adds.)
 */
export function isLocaleNoindex(pathname: string, locale: AppLocale): boolean {
	return locale !== defaultLocale && isUgcPath(pathname);
}

/** True if `pathname` in `locale` may appear in the index and the sitemap. */
export function isIndexable(
	pathname: string,
	locale: AppLocale = defaultLocale,
): boolean {
	return !isPrivatePath(pathname) && !isLocaleNoindex(pathname, locale);
}

/**
 * Static public routes that belong in the sitemap — real 200 pages only.
 * Redirect aliases (/daily-posts, /seasonal, /topics, /education,
 * /explore, /market, /farmers, bare /sellers) are deliberately
 * excluded: they return 3xx, so listing them would violate sitemap purity.
 * Legal pages are included (real content, already robots index:true). Dynamic
 * entity URLs (products / sellers) are added in a later PR once full-enumeration
 * endpoints + generateSitemaps sharding are wired.
 */
export const INDEXABLE_STATIC_ROUTES = [
	"/",
	"/market/products",
	"/feed",
	"/faq",
	"/seller",
	"/seller/pricing",
	"/seller-guide",
	"/about",
	"/contact",
	"/privacy",
	"/terms",
	"/return-policy",
	"/user-policy",
	"/seller-terms",
] as const;
