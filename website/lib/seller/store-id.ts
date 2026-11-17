/**
 * The storefront URL segment is the store_code; social RPCs and cache tags are keyed by the
 * numeric store_id carried on the public seller payload. Returns "" when the payload has none.
 */
export function storeIdOfSeller(
	seller: { storeId?: unknown } | null | undefined,
): string {
	const raw = String(seller?.storeId ?? "").trim();
	return /^[1-9]\d*$/.test(raw) ? raw : "";
}
