import { requiredAccessForPath } from "@/lib/auth/redirect";
import { stripLocale } from "@/lib/intl/paths";

export const HOME_CONTENT_READY_SELECTOR =
	'[data-hk-initial-content-ready="home"]';
export const SEARCH_CONTENT_READY_SELECTOR =
	'[data-hk-page-content-ready="search"]';
export const PRODUCT_CONTENT_READY_SELECTOR =
	'[data-hk-page-content-ready="product-detail"]';
export const SELLER_PROFILE_READY_SELECTOR = "[data-hk-seller-profile-ready]";
export const SELLER_STOREFRONT_CONTENT_READY_SELECTOR =
	'[data-hk-page-content-ready="seller-storefront"]';
export const ROUTE_TERMINAL_READY_SELECTOR = "[data-hk-route-terminal-ready]";
export const PRODUCT_DETAIL_ID_ATTRIBUTE = "data-hk-product-detail-id";

/** Shared allowlist for the document bootstrap and React handoff. Never infer
 * public readiness from the absence of an account prefix or from any marker. */
export const PUBLIC_INITIAL_CONTENT_RULES = [
	{ pattern: "^/$", selectors: [HOME_CONTENT_READY_SELECTOR] },
	{ pattern: "^/feed$", selectors: ['[data-hk-tab-content-ready="feed"]'] },
	{
		pattern: "^/market/products$",
		selectors: ['[data-hk-tab-content-ready="market"]'],
	},
	{ pattern: "^/search$", selectors: [SEARCH_CONTENT_READY_SELECTOR] },
	{
		pattern: "^/products/([^/]+)$",
		selectors: [PRODUCT_CONTENT_READY_SELECTOR],
		product: true,
	},
	{
		pattern: "^/sellers/[^/]+/products/([^/]+)$",
		selectors: [PRODUCT_CONTENT_READY_SELECTOR],
		product: true,
	},
	{
		pattern: "^/sellers/[^/]+(?:/(?:messages|posts|reviews))?$",
		selectors: [
			SELLER_PROFILE_READY_SELECTOR,
			SELLER_STOREFRONT_CONTENT_READY_SELECTOR,
		],
	},
] as const;

const EXACT_ROUTE_CONTENT_READY_SELECTORS: Record<string, readonly string[]> = {
	"/account/notifications": ['[data-hk-page-content-ready="notifications"]'],
	"/seller-center/notifications": [
		'[data-hk-page-content-ready="notifications"]',
	],
	"/seller-center/wallet": ['[data-hk-page-content-ready="seller-wallet"]'],
};

export function productIdFromPathname(rawPathname: string): string | null {
	const segments = stripLocale(rawPathname).split("/").filter(Boolean);
	const encodedId =
		segments[0] === "products" && segments.length === 2
			? segments[1]
			: segments[0] === "sellers" &&
					segments.length === 4 &&
					segments[2] === "products"
				? segments[3]
				: undefined;
	if (!encodedId) return null;
	try {
		return decodeURIComponent(encodedId);
	} catch {
		return encodedId;
	}
}

/**
 * A product route is ready only when the marker belongs to the requested
 * product and has left React's hidden Flight staging container.
 */
export function isProductDetailReady(
	pathname: string,
	root: ParentNode = document,
): boolean {
	const productId = productIdFromPathname(pathname);
	if (!productId) return false;
	return Array.from(
		root.querySelectorAll<HTMLElement>(PRODUCT_CONTENT_READY_SELECTOR),
	).some((marker) => {
		if (marker.getAttribute(PRODUCT_DETAIL_ID_ATTRIBUTE) !== productId) {
			return false;
		}
		if (!marker.isConnected || marker.closest("[hidden]")) return false;
		return marker.getClientRects().length > 0;
	});
}

export function publicRouteContentReadySelectors(
	rawPathname: string,
): string[] {
	const pathname = stripLocale(rawPathname);
	const normalized =
		pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
	const rule = PUBLIC_INITIAL_CONTENT_RULES.find(({ pattern }) =>
		new RegExp(pattern).test(normalized),
	);
	return rule ? [...rule.selectors] : [];
}

/** Hidden marker spans are valid; markers inside hidden Flight trees are not. */
export function isPublicInitialContentReady(
	pathname: string,
	root: ParentNode = document,
): boolean {
	const selectors = publicRouteContentReadySelectors(pathname);
	if (selectors.length === 0) return false;
	const productId = productIdFromPathname(pathname);
	const hasCommittedMarker = (selector: string, expectedId?: string | null) =>
		Array.from(root.querySelectorAll(selector)).some(
			(marker) =>
				marker.isConnected &&
				marker.parentElement &&
				!marker.parentElement.closest("[hidden]") &&
				(!expectedId ||
					marker.getAttribute(PRODUCT_DETAIL_ID_ATTRIBUTE) === expectedId),
		);
	return (
		hasCommittedMarker(ROUTE_TERMINAL_READY_SELECTOR) ||
		selectors.every((selector) => hasCommittedMarker(selector, productId))
	);
}

/**
 * Content signals that own the initial hard-load handoff for a pathname.
 *
 * Only routes whose request-time tree can outlive DOMContentLoaded belong
 * here. Their marker is emitted with the complete first content tree, so the
 * document splash remains the sole visual owner while a PPR/RSC hole is still
 * being filled. Soft navigations use the separate PageTransition contract.
 */
export function routeContentReadySelectors(rawPathname: string): string[] {
	const pathname = stripLocale(rawPathname);
	const normalizedPathname =
		pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
	const selectors = [
		...publicRouteContentReadySelectors(normalizedPathname),
		...(EXACT_ROUTE_CONTENT_READY_SELECTORS[normalizedPathname] ?? []),
	];

	const access = requiredAccessForPath(normalizedPathname);
	if (access) {
		selectors.push(`[data-hk-role-zone-ready="${access}"]`);
	}
	if (
		normalizedPathname === "/seller-center" ||
		normalizedPathname.startsWith("/seller-center/")
	) {
		selectors.push("[data-hk-seller-workspace-ready]");
	}

	return [...new Set(selectors)];
}
