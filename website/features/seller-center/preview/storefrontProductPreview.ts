/**
 * Only the canonical public PDP href can open the seller-preview drawer.
 * Keeping this parser strict makes the capture boundary fail closed for every
 * other link/button on the buyer storefront.
 */
export function productIdFromStorefrontPreviewHref(
	href: string | null,
): string | null {
	if (!href || href.startsWith("//")) return null;
	const match = /^\/products\/([^/?#]+)\/?$/.exec(href);
	if (!match) return null;

	try {
		const id = decodeURIComponent(match[1]);
		return /^\d+$/.test(id) ? id : null;
	} catch {
		return null;
	}
}
