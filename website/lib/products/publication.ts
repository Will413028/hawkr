export const PUBLICATION_STATE = {
	PUBLIC: "4012",
	TEMPORARILY_UNAVAILABLE: "4013",
} as const;

export interface PurchaseAvailability {
	unavailable: boolean;
	canSubscribeRestock: boolean;
	label: "已售完" | "暫不可購買";
}

/** Combines publication policy and physical inventory for buyer purchase UI. */
export function derivePurchaseAvailability({
	publicationState,
	purchasable,
	stock,
}: {
	publicationState?: string;
	purchasable?: boolean;
	stock?: number;
}): PurchaseAvailability {
	const temporarilyUnavailable =
		publicationState === PUBLICATION_STATE.TEMPORARILY_UNAVAILABLE &&
		purchasable === false;
	const soldOut = typeof stock === "number" && stock <= 0;
	return {
		unavailable: purchasable === false || soldOut,
		canSubscribeRestock: soldOut && !temporarilyUnavailable,
		label: temporarilyUnavailable ? "暫不可購買" : "已售完",
	};
}
