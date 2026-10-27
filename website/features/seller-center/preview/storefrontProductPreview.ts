function asRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object"
		? (value as Record<string, unknown>)
		: {};
}

function idText(value: unknown): string {
	if (typeof value === "string") return value.trim();
	if (typeof value === "number" && Number.isFinite(value)) return String(value);
	return "";
}

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

/** Resolve the seller account id carried by the public product detail DTO. */
export function sellerIdFromPreviewProduct(product: unknown): string | null {
	const productRecord = asRecord(product);
	const sellerRecord = asRecord(productRecord.seller);
	const id = idText(
		sellerRecord.storeId ??
			productRecord.sellerAccountId ??
			productRecord.storeId ??
			productRecord.createdBy,
	);
	return /^\d+$/.test(id) ? id : null;
}
