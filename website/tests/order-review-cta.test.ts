import assert from "node:assert/strict";
import test from "node:test";
import { canWriteOrderReview, type Order } from "../lib/orders/types";

function order(overrides: Partial<Order> = {}): Order {
	return {
		orderNo: "HK20260627001",
		storeId: "seller-1",
		status: "completed",
		totalActualAmount: "100",
		createdAt: 0,
		items: [
			{
				orderItemId: "item-1",
				productId: "product-1",
				skuId: "sku-1",
				name: "番茄",
				price: "100",
				quantity: 1,
			},
		],
		canReview: true,
		...overrides,
	};
}

test("canWriteOrderReview allows review only when no reviewed signal exists", () => {
	assert.equal(canWriteOrderReview(order()), true);
	assert.equal(canWriteOrderReview(order({ canReview: false })), false);
	assert.equal(canWriteOrderReview(order({ isReviewed: true })), false);
	assert.equal(
		canWriteOrderReview(
			order({
				review: { rating: 5, content: "good", createdAt: "2026-06-24" },
			}),
		),
		false,
	);
	assert.equal(
		canWriteOrderReview(
			order({
				items: [
					{
						orderItemId: "item-1",
						productId: "product-1",
						skuId: "sku-1",
						name: "番茄",
						price: "100",
						quantity: 1,
						reviewed: true,
					},
				],
			}),
		),
		false,
	);
});
