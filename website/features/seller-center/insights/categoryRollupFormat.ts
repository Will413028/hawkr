export interface RollupChange {
	absolute: number;
	percent: number;
	direction: "up" | "down" | "flat";
}

/** Client-side change derivation for the simplified category rollup card —
 *  `GetCategoryPriceRollup` only carries `latestPrice`/`prevPrice`, unlike
 *  `MarketOverviewChange` which the server already computes. Returns `null`
 *  when either price is missing rather than fabricating a change from a
 *  partial pair. */
export function computeRollupChange(
	latestPrice: number | undefined,
	prevPrice: number | undefined,
): RollupChange | null {
	if (latestPrice == null || prevPrice == null) return null;
	const absolute = latestPrice - prevPrice;
	const percent = prevPrice === 0 ? 0 : (absolute / prevPrice) * 100;
	const direction = absolute > 0 ? "up" : absolute < 0 ? "down" : "flat";
	return { absolute, percent, direction };
}
