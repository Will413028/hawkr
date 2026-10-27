export type SellerCardProductStatus = "loaded" | "empty" | "unavailable";

interface ProductSource<T> {
	records: readonly T[];
}

export interface SellerCardProductState<T> {
	records: T[];
	status: SellerCardProductStatus;
}

/**
 * Prefer the richer market payload, but distinguish an authoritative empty
 * fallback from a failed fallback. Unknown product state must never be shown as
 * "沒有上架商品".
 */
export function resolveSellerCardProductState<T>(
	rich: ProductSource<T> | null,
	lean: ProductSource<T> | null | undefined,
): SellerCardProductState<T> {
	if (rich?.records.length) {
		return { records: [...rich.records], status: "loaded" };
	}
	if (lean) {
		return {
			records: [...lean.records],
			status: lean.records.length ? "loaded" : "empty",
		};
	}
	return { records: [], status: "unavailable" };
}
