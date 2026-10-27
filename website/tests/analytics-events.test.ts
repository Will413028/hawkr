import assert from "node:assert/strict";
import { test } from "node:test";
import { buildPurchaseParams, roleProperty } from "../lib/analytics/events";
import type { Order } from "../lib/orders/types";

const ORDER: Order = {
	orderNo: "O123",
	storeId: "9",
	status: "paid",
	totalActualAmount: "980",
	shippingFee: "80",
	discount: "0",
	createdAt: 0,
	items: [
		{
			orderItemId: 1,
			productId: "555",
			skuId: "777",
			name: "蘋果",
			price: "450",
			quantity: 2,
		},
	],
};

test("buildPurchaseParams maps transaction_id, value, items, currency", () => {
	const params = buildPurchaseParams(ORDER);
	assert.equal(params.transaction_id, "O123");
	assert.equal(params.value, 980);
	assert.equal(params.currency, "TWD");
	assert.equal(params.shipping, 80);
	assert.equal(params.items.length, 1);
	assert.equal(params.items[0].item_id, "555");
});

test("analytics role is categorical and clears to anonymous", () => {
	assert.deepEqual(roleProperty("seller"), { role: "seller" });
	assert.deepEqual(roleProperty(null), { role: "anonymous" });
});
