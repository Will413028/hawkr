import assert from "node:assert/strict";
import { test } from "node:test";
import { sellerOrderTabFor } from "../features/seller-center/orders/orderTabs";
import type { AfterSaleCase } from "../lib/after-sales/types";
import { AFTER_SALE_STATUS } from "../lib/after-sales/types";
import type { Order } from "../lib/orders/types";

function makeOrder(overrides: Partial<Order> = {}): Order {
	return {
		orderNo: "O001",
		storeId: "S001",
		status: "delivered",
		totalActualAmount: "999",
		createdAt: Date.now(),
		items: [],
		...overrides,
	};
}

function makeCase(overrides: Partial<AfterSaleCase> = {}): AfterSaleCase {
	return {
		caseNo: "A001",
		orderNo: "O001",
		buyerId: "B001",
		buyerName: "Buyer",
		status: AFTER_SALE_STATUS.PENDING,
		reason: "5001",
		description: "",
		refundableAmount: "999",
		returnShipmentNo: "",
		returnShippingMethod: "",
		createdAt: "2026-06-25T00:00:00Z",
		...overrides,
	};
}

test("delivered orders without after-sale cases stay in the shipping tab", () => {
	assert.equal(sellerOrderTabFor(makeOrder()), "in_transit");
});

test("active after-sale cases leave the generic shipping tab", () => {
	assert.equal(
		sellerOrderTabFor(
			makeOrder({ status: "delivered" }),
			makeCase({ status: AFTER_SALE_STATUS.PENDING }),
		),
		"aftersale",
	);
	assert.equal(
		sellerOrderTabFor(
			makeOrder({ status: "shipping" }),
			makeCase({ status: AFTER_SALE_STATUS.REFUNDING }),
		),
		"aftersale",
	);
});

test("closed coupon compensation cases resolve delivered orders as completed", () => {
	assert.equal(
		sellerOrderTabFor(
			makeOrder({ status: "delivered" }),
			makeCase({ status: AFTER_SALE_STATUS.CLOSED }),
		),
		"completed",
	);
});

test("rejected and cancelled cases do not rewrite the order lifecycle tab", () => {
	assert.equal(
		sellerOrderTabFor(
			makeOrder({ status: "delivered" }),
			makeCase({ status: AFTER_SALE_STATUS.REJECTED }),
		),
		"in_transit",
	);
	assert.equal(
		sellerOrderTabFor(
			makeOrder({ status: "delivered" }),
			makeCase({ status: AFTER_SALE_STATUS.CANCELLED }),
		),
		"in_transit",
	);
});

test("called Tcat orders still classify as shipping while backend status is preparing", () => {
	assert.equal(
		sellerOrderTabFor(
			makeOrder({
				status: "preparing",
				shipping: { method: "3001", shipmentStatus: "210" },
			}),
		),
		"in_transit",
	);
});
