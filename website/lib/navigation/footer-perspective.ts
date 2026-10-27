import { stripLocale } from "@/lib/intl/paths";

export type FooterPerspective = "buyer" | "seller";
export type FooterActiveRole = FooterPerspective | null | undefined;

const SELLER_CENTER_ROOT = "/seller-center";
const BUYER_STOREFRONT_PREVIEW_ROOT = "/seller-center/preview";

function isRouteWithin(pathname: string, routeRoot: string): boolean {
	return pathname === routeRoot || pathname.startsWith(`${routeRoot}/`);
}

/**
 * Resolve footer chrome from the perspective represented by the page and the
 * workspace the signed-in user is actively viewing.
 *
 * The storefront preview lives under the authenticated seller-center route,
 * but deliberately renders the buyer-facing storefront. It therefore keeps
 * the canonical buyer footer while the rest of seller-center uses the
 * canonical seller footer. Outside that explicit preview, public/legal pages
 * keep the active workspace's footer so a seller following a support link does
 * not suddenly land on buyer navigation.
 */
export function footerPerspectiveForPathname(
	rawPathname: string | null | undefined,
	activeRole?: FooterActiveRole,
): FooterPerspective {
	if (!rawPathname) return activeRole === "seller" ? "seller" : "buyer";
	const pathname = stripLocale(rawPathname);

	const isSellerCenter = isRouteWithin(pathname, SELLER_CENTER_ROOT);
	const isBuyerStorefrontPreview = isRouteWithin(
		pathname,
		BUYER_STOREFRONT_PREVIEW_ROOT,
	);

	if (isBuyerStorefrontPreview) return "buyer";
	if (isSellerCenter) return "seller";
	return activeRole === "seller" ? "seller" : "buyer";
}
