import { describe, expect, it } from "vitest";
import type { CartItem } from "@/lib/cart";
import {
	reconcileCartSelection,
	selectCartItem,
	selectCartSeller,
} from "./cartSelection";

const item = (
	id: string,
	sellerStoreId: string,
	purchasable = true,
): CartItem => ({
	id,
	skuId: id,
	name: id,
	sellerStoreId,
	price: "NT$ 100",
	priceValue: 100,
	quantity: 1,
	purchasable,
});
const items = [item("a1", "a"), item("a2", "a", false), item("b1", "b")];

describe("cart store selection", () => {
	it("defaults only a new cart to one purchasable store", () => {
		expect(reconcileCartSelection(items, {}, true)).toEqual({
			a1: true,
			a2: false,
			b1: false,
		});
		expect(reconcileCartSelection([items[1], items[2]], {}, true)).toEqual({
			a2: false,
			b1: true,
		});
	});

	it("preserves a deliberate empty selection across hydration and refresh", () => {
		const previous = { a1: false, a2: false, b1: false };
		expect(reconcileCartSelection(items, previous, true)).toEqual(previous);
		expect(reconcileCartSelection(items, previous, false)).toEqual(previous);
	});

	it("does not silently choose another store after the last selected item is removed", () => {
		expect(
			reconcileCartSelection([items[2]], { a1: true, b1: false }, false),
		).toEqual({ b1: false });
	});

	it("normalizes a legacy cross-store selection to one store", () => {
		expect(
			reconcileCartSelection(items, { a1: true, a2: true, b1: true }, true),
		).toEqual({ a1: true, a2: false, b1: false });
	});

	it("switches stores atomically and retains unavailable rows for explicit bulk removal", () => {
		expect(selectCartSeller(items, { b1: true }, "a", true)).toEqual({
			a1: true,
			a2: true,
			b1: false,
		});
		expect(selectCartSeller(items, { a1: true, a2: true }, "a", false)).toEqual(
			{ a1: false, a2: false, b1: false },
		);
		expect(
			selectCartItem(items, { a1: true, a2: true }, items[2], true),
		).toEqual({ a1: false, a2: false, b1: true });
	});
});
