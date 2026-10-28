import { splitLocale, stripLocale } from "@/lib/intl/paths";

const ASSISTANT_ROUTE_ROOT = "/account/messages/assistant";
const PUBLIC_ASSISTANT_PATHS = new Set([
	`${ASSISTANT_ROUTE_ROOT}/new`,
	`${ASSISTANT_ROUTE_ROOT}/procurement`,
]);

const UTM_QUERY_KEYS = new Set([
	"utm_campaign",
	"utm_content",
	"utm_creative_format",
	"utm_id",
	"utm_marketing_tactic",
	"utm_medium",
	"utm_source",
	"utm_source_platform",
	"utm_term",
]);

/**
 * Manual campaign values are public URL input, so only reviewed categorical
 * tokens are retained. Google Ads campaign detail uses auto-tagging instead of
 * accepting arbitrary names, terms or content strings here.
 */
const SAFE_UTM_VALUES: Readonly<Record<string, ReadonlySet<string>>> = {
	utm_campaign: new Set([
		"2609_hk",
		"hk_tw_search_seller_core_202609",
		"seller_search",
	]),
	utm_creative_format: new Set(["image", "search", "text", "video"]),
	utm_marketing_tactic: new Set(["acquisition", "prospecting"]),
	utm_medium: new Set([
		"cpc",
		"email",
		"organic",
		"paid_search",
		"paid_social",
		"referral",
		"social",
	]),
	utm_source: new Set([
		"hawkr",
		"facebook",
		"google",
		"google_ads",
		"instagram",
		"internal",
		"line",
		"newsletter",
	]),
	utm_source_platform: new Set(["google_ads", "manual"]),
};

const CLICK_ATTRIBUTION_QUERY_KEYS = new Set([
	"dclid",
	"gad_source",
	"gbraid",
	"gclid",
	"wbraid",
]);

const EMAIL_LIKE_VALUE = /[^\s@]+@[^\s@]+\.[^\s@]+/i;
const PHONE_LIKE_VALUE = /(?:\+?\d[\s()._~-]*){8,}/;
const SAFE_CLICK_ID_VALUE =
	/^(?=.{20,256}$)(?=.*[A-Za-z])(?=.*\d)[A-Za-z0-9._~-]+$/;
const SAFE_GAD_SOURCE_VALUE = /^[1-9]$/;
const MAX_ATTRIBUTION_QUERY_LENGTH = 1024;

/**
 * Exact app routes that cannot contain user-controlled path segments.
 * New routes deliberately fail closed to `/:unknown` until reviewed here.
 */
const STATIC_ANALYTICS_ROUTES = new Set([
	"/",
	"/about",
	"/account",
	"/account/addresses",
	"/account/addresses/new",
	"/account/after-sales",
	"/account/coupons",
	"/account/favorites",
	"/account/help",
	"/account/messages",
	"/account/notifications",
	"/account/notifications/preferences",
	"/account/orders",
	"/account/profile",
	"/account/recent",
	"/account/reviews",
	"/account/settings",
	"/account/wallet",
	"/adm/enter",
	"/adm/unavailable",
	"/cart",
	"/checkout",
	"/checkout/fail",
	"/checkout/submitted",
	"/checkout/success",
	"/contact",
	"/faq",
	"/feed",
	"/login",
	"/login/sso-callback",
	"/market/products",
	"/payment/return",
	"/privacy",
	"/quote-confirm",
	"/return-policy",
	"/search",
	"/seller",
	"/seller-center",
	"/seller-center/ai",
	"/seller-center/coupons",
	"/seller-center/coupons/new",
	"/seller-center/customers",
	"/seller-center/dashboard",
	"/seller-center/inventory",
	"/seller-center/me",
	"/seller-center/me/certifications",
	"/seller-center/me/shipping",
	"/seller-center/messages",
	"/seller-center/notifications",
	"/seller-center/notifications/preferences",
	"/seller-center/orders",
	"/seller-center/posts",
	"/seller-center/posts/new",
	"/seller-center/preview",
	"/seller-center/products",
	"/seller-center/products/import",
	"/seller-center/products/new",
	"/seller-center/shipments/anomalies",
	"/seller-center/suspended",
	"/seller-center/wallet",
	"/seller-guide",
	"/seller-terms",
	"/seller/measures",
	"/seller/onboarding",
	"/seller/pricing",
	"/terms",
	"/user-policy",
]);

const DYNAMIC_ROUTE_PATTERNS: ReadonlyArray<readonly [RegExp, string]> = [
	[
		/^\/adm\/(?!api(?:\/|$)|enter(?:\/|$)|unavailable(?:\/|$))[^/]+\/orders\/[^/]+(?:\/.*)?$/,
		"/adm/:storeId/orders/:orderNo",
	],
	[
		/^\/adm\/(?!api(?:\/|$)|enter(?:\/|$)|unavailable(?:\/|$))[^/]+\/products\/[^/]+\/(edit|preview)\/?$/,
		"/adm/:storeId/products/:productId/$1",
	],
	[
		/^\/adm\/(?!api(?:\/|$)|enter(?:\/|$)|unavailable(?:\/|$))[^/]+\/products\/[^/]+(?:\/.*)?$/,
		"/adm/:storeId/products/:productId",
	],
	[
		/^\/adm\/(?!api(?:\/|$)|enter(?:\/|$)|unavailable(?:\/|$))[^/]+\/?$/,
		"/adm/:storeId",
	],
	[
		/^\/adm\/(?!api(?:\/|$)|enter(?:\/|$)|unavailable(?:\/|$))[^/]+(?:\/.*)?$/,
		"/adm/:storeId/:area",
	],
	[
		/^\/seller-center\/products\/import\/[^/]+\/items\/[^/]+(?:\/.*)?$/,
		"/seller-center/products/import/:jobId/items/:itemId",
	],
	[
		/^\/seller-product-import-preview\/[^/]+\/items\/[^/]+(?:\/.*)?$/,
		"/seller-product-import-preview/:jobId/items/:itemId",
	],
	[
		/^\/sellers\/[^/]+\/products\/[^/]+(?:\/.*)?$/,
		"/sellers/:sellerId/products/:productId",
	],
	[
		/^\/seller-center\/products\/import\/[^/]+(?:\/.*)?$/,
		"/seller-center/products/import/:jobId",
	],
	[
		/^\/seller-product-import-preview\/[^/]+(?:\/.*)?$/,
		"/seller-product-import-preview/:jobId",
	],
	[
		/^\/seller-center\/coupons\/[^/]+(?:\/.*)?$/,
		"/seller-center/coupons/:couponId",
	],
	[
		/^\/seller-center\/customers\/[^/]+(?:\/.*)?$/,
		"/seller-center/customers/:buyerId",
	],
	[
		/^\/seller-center\/messages\/[^/]+(?:\/.*)?$/,
		"/seller-center/messages/:targetId",
	],
	[
		/^\/seller-center\/orders\/[^/]+(?:\/.*)?$/,
		"/seller-center/orders/:orderNo",
	],
	[/^\/seller-center\/posts\/[^/]+(?:\/.*)?$/, "/seller-center/posts/:postId"],
	[/^\/seller\/measures\/[^/]+\/appeal$/, "/seller/measures/:measureId/appeal"],
	[
		/^\/seller\/measures\/[^/]+\/remediation$/,
		"/seller/measures/:measureId/remediation",
	],
	[/^\/seller\/measures\/[^/]+$/, "/seller/measures/:measureId"],
	[
		/^\/seller-center\/preview\/[^/]+(?:\/.*)?$/,
		"/seller-center/preview/:sellerId",
	],
	[
		/^\/seller-center\/products\/[^/]+(?:\/.*)?$/,
		"/seller-center/products/:productId",
	],
	[/^\/account\/addresses\/[^/]+(?:\/.*)?$/, "/account/addresses/:addressId"],
	[/^\/account\/after-sales\/[^/]+(?:\/.*)?$/, "/account/after-sales/:caseNo"],
	[/^\/account\/messages\/[^/]+(?:\/.*)?$/, "/account/messages/:targetId"],
	[/^\/account\/orders\/[^/]+(?:\/.*)?$/, "/account/orders/:orderNo"],
	[/^\/account\/wallet\/[^/]+(?:\/.*)?$/, "/account/wallet/:sellerId"],
	[/^\/orders\/[^/]+(?:\/.*)?$/, "/orders/:orderNo"],
	[/^\/products\/[^/]+(?:\/.*)?$/, "/products/:productId"],
	[/^\/sellers\/[^/]+(?:\/.*)?$/, "/sellers/:sellerId"],
];

export function isAssistantAnalyticsRoute(rawPathname: string): boolean {
	const pathname = stripLocale(rawPathname);
	return (
		pathname === ASSISTANT_ROUTE_ROOT ||
		pathname.startsWith(`${ASSISTANT_ROUTE_ROOT}/`)
	);
}

export interface PageViewLocationInput {
	pathname: string;
	/** URLSearchParams.toString() output, without the leading question mark. */
	query: string;
	origin: string;
}

export interface PageViewLocation {
	pagePath: string;
	pageLocation: string;
}

function safeAttributionQuery(query: string): string {
	const safe = new URLSearchParams();
	for (const [rawKey, value] of new URLSearchParams(query)) {
		const key = rawKey.toLowerCase();
		const isUtm = UTM_QUERY_KEYS.has(key);
		if (!isUtm && !CLICK_ATTRIBUTION_QUERY_KEYS.has(key)) {
			continue;
		}
		// Campaign values are user-controlled URL input. Manual UTM values must match
		// a reviewed categorical allowlist; click IDs must match a provider-shaped
		// grammar. Property-level email/query redaction remains a second defence.
		if (!value || value.length > 256) continue;
		if (EMAIL_LIKE_VALUE.test(value)) continue;
		let safeValue = value;
		if (isUtm) {
			safeValue = value.toLowerCase();
			if (!SAFE_UTM_VALUES[key]?.has(safeValue)) continue;
		}
		if (
			!isUtm &&
			(key === "gad_source"
				? !SAFE_GAD_SOURCE_VALUE.test(value)
				: !SAFE_CLICK_ID_VALUE.test(value))
		) {
			continue;
		}
		if (safe.has(key)) continue;
		if (
			safe.toString().length + key.length + safeValue.length + 2 >
			MAX_ATTRIBUTION_QUERY_LENGTH
		) {
			continue;
		}
		safe.append(key, safeValue);
	}
	return safe.toString();
}

function safeRoutePattern(pathname: string): string | null {
	for (const [pattern, replacement] of DYNAMIC_ROUTE_PATTERNS) {
		if (pattern.test(pathname)) return pathname.replace(pattern, replacement);
	}
	return null;
}

/**
 * Collapse route segments that may contain account or business identifiers. The routes are
 * classified on the locale-free path, and the page's language prefix is kept on the result
 * (`/en/products/:productId`), so reports can still split by language.
 */
export function privacySafeRoutePathname(rawPathname: string): string {
	const { prefix, pathname } = splitLocale(rawPathname);
	const safe = privacySafeAppPathname(pathname);
	return safe === "/" ? prefix || "/" : `${prefix}${safe}`;
}

function privacySafeAppPathname(pathname: string): string {
	const normalizedPathname =
		pathname !== "/" ? pathname.replace(/\/+$/, "") : pathname;
	if (isAssistantAnalyticsRoute(normalizedPathname)) {
		if (
			normalizedPathname === ASSISTANT_ROUTE_ROOT ||
			PUBLIC_ASSISTANT_PATHS.has(normalizedPathname)
		) {
			return normalizedPathname;
		}
		// Only exact static paths are public. Collapse the complete remainder so a
		// future nested route cannot accidentally expose a second identifier.
		return `${ASSISTANT_ROUTE_ROOT}/:conversationId`;
	}
	if (STATIC_ANALYTICS_ROUTES.has(normalizedPathname))
		return normalizedPathname;
	return safeRoutePattern(normalizedPathname) ?? "/:unknown";
}

/** Prevent identifiers embedded in dynamic-page titles from reaching GA. */
export function privacySafePageTitle(title: string, pathname: string): string {
	const safePathname = privacySafeRoutePathname(pathname);
	if (
		safePathname !== pathname ||
		EMAIL_LIKE_VALUE.test(title) ||
		PHONE_LIKE_VALUE.test(title)
	) {
		return safePathname;
	}
	return title.slice(0, 300);
}

/**
 * GA page-view location for every route.
 *
 * Only campaign-attribution query keys are retained. Account, checkout,
 * procurement, coupon and invitation parameters therefore cannot reach GA4,
 * and dynamic identifiers in known product routes are collapsed to templates.
 */
export function privacySafePageViewLocation({
	pathname,
	query,
	origin,
}: PageViewLocationInput): PageViewLocation {
	const safePathname = privacySafeRoutePathname(pathname);
	const safeQuery = safeAttributionQuery(query);

	const pagePath = safeQuery ? `${safePathname}?${safeQuery}` : safePathname;
	return {
		pagePath,
		pageLocation: new URL(pagePath, origin).href,
	};
}

/**
 * Referrers can carry the same sensitive query values as the current URL.
 * Same-origin referrers keep a templated path; cross-origin referrers keep only
 * their origin because an external path is outside our privacy contract.
 */
export function privacySafePageReferrer(
	referrer: string,
	currentOrigin: string,
): string {
	if (!referrer) return "";
	try {
		const url = new URL(referrer);
		if (url.protocol !== "https:" && url.protocol !== "http:") return "";
		if (url.origin !== currentOrigin) return `${url.origin}/`;
		return privacySafePageViewLocation({
			pathname: url.pathname,
			query: url.search.replace(/^\?/, ""),
			origin: url.origin,
		}).pageLocation;
	} catch {
		return "";
	}
}

/** One normalized URL produces at most one page_view per component lifetime. */
export function shouldTrackPageView(
	lastPageLocation: string | null,
	nextPageLocation: string,
): boolean {
	return lastPageLocation !== nextPageLocation;
}
