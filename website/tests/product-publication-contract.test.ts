import assert from "node:assert/strict";
import test from "node:test";
import { derivePurchaseAvailability } from "../lib/products/publication";

test("temporarily unavailable products disable buying without offering restock", () => {
	assert.deepEqual(
		derivePurchaseAvailability({
			publicationState: "4013",
			purchasable: false,
			stock: 9,
		}),
		{
			unavailable: true,
			canSubscribeRestock: false,
			label: "暫不可購買",
		},
	);
});

test("public products with zero stock retain the restock flow", () => {
	assert.deepEqual(
		derivePurchaseAvailability({
			publicationState: "4012",
			purchasable: true,
			stock: 0,
		}),
		{
			unavailable: true,
			canSubscribeRestock: true,
			label: "已售完",
		},
	);
});
