import type { CartItem } from "@/lib/cart";

export type CartSelection = Record<string, boolean>;

export function cartSellerKeyOf(item: {
	sellerStoreId?: string;
	sellerName?: string;
}): string {
	return item.sellerStoreId || item.sellerName || "unknown";
}

/** Select one store atomically. Other stores stay in the cart, unselected. */
export function selectCartSeller(
	items: CartItem[],
	previous: CartSelection,
	sellerKey: string,
	next: boolean,
): CartSelection {
	return Object.fromEntries(
		items.map((item) => [
			item.id,
			cartSellerKeyOf(item) === sellerKey ? next : !next && !!previous[item.id],
		]),
	);
}

export function selectCartItem(
	items: CartItem[],
	previous: CartSelection,
	item: CartItem,
	next: boolean,
): CartSelection {
	const sellerKey = cartSellerKeyOf(item);
	return Object.fromEntries(
		items.map((candidate) => [
			candidate.id,
			candidate.id === item.id
				? next
				: !!previous[candidate.id] &&
					(!next || cartSellerKeyOf(candidate) === sellerKey),
		]),
	);
}

/** Reconcile server rows without undoing an explicit deselection or choosing
 * another store after removal. Only a new cart with no saved choice defaults. */
export function reconcileCartSelection(
	items: CartItem[],
	previous: CartSelection,
	selectDefault: boolean,
): CartSelection {
	const retained = items.find(
		(item) => previous[item.id] && item.purchasable !== false,
	);
	const defaultItem =
		selectDefault && Object.keys(previous).length === 0
			? items.find((item) => item.purchasable !== false)
			: undefined;
	const sellerKey = retained
		? cartSellerKeyOf(retained)
		: defaultItem
			? cartSellerKeyOf(defaultItem)
			: null;

	return Object.fromEntries(
		items.map((item) => [
			item.id,
			item.purchasable !== false &&
				cartSellerKeyOf(item) === sellerKey &&
				(!!defaultItem || !!previous[item.id]),
		]),
	);
}
