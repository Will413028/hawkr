import assert from "node:assert/strict";
import { test } from "node:test";
import {
	orderTimelineCurrentIndex,
	shouldMergeAfterSaleIntoOrder,
} from "../features/orders/components/orderStatusTimelineModel";
import { AFTER_SALE_STATUS } from "../lib/after-sales/types";

test("after-sale branch merges into the order rail only when the case is closed", () => {
	assert.equal(shouldMergeAfterSaleIntoOrder(AFTER_SALE_STATUS.CLOSED), true);
	for (const status of [
		AFTER_SALE_STATUS.PENDING,
		AFTER_SALE_STATUS.RETURN_PENDING,
		AFTER_SALE_STATUS.RETURN_SHIPPING,
		AFTER_SALE_STATUS.RETURN_FAILED,
		AFTER_SALE_STATUS.REFUNDING,
		AFTER_SALE_STATUS.REJECTED,
		AFTER_SALE_STATUS.CANCELLED,
	]) {
		assert.equal(
			shouldMergeAfterSaleIntoOrder(status),
			false,
			`status ${status} should not merge`,
		);
	}
});

test("active after-sale cases hold the order rail before completed", () => {
	assert.equal(
		orderTimelineCurrentIndex({
			orderStatus: "completed",
			hasAfterSale: true,
			afterSaleStatus: AFTER_SALE_STATUS.REFUNDING,
		}),
		3,
	);
	assert.equal(
		orderTimelineCurrentIndex({
			orderStatus: "refunded",
			hasAfterSale: true,
			afterSaleStatus: AFTER_SALE_STATUS.REJECTED,
		}),
		3,
	);
});

test("closed after-sale cases complete the order rail", () => {
	assert.equal(
		orderTimelineCurrentIndex({
			orderStatus: "shipping",
			hasAfterSale: true,
			afterSaleStatus: AFTER_SALE_STATUS.CLOSED,
		}),
		4,
	);
	assert.equal(
		orderTimelineCurrentIndex({
			orderStatus: "completed",
			hasAfterSale: false,
		}),
		4,
	);
});
