import assert from "node:assert/strict";
import { test } from "node:test";
import { getCartCouponAvailability } from "../features/cart/couponAvailability";
import type { CartAvailableCoupon } from "../lib/coupons/repo";

function coupon(
	overrides: Partial<CartAvailableCoupon> = {},
): CartAvailableCoupon {
	return {
		id: 1,
		name: "測試優惠券",
		kind: "cash",
		estimatedDiscount: 80,
		...overrides,
	};
}

const context = {
	orderAmount: 500,
	shippingFee: 120,
	now: Date.UTC(2026, 6, 11),
};

test("cart coupon is usable when backend returns a positive discount", () => {
	assert.deepEqual(getCartCouponAvailability(coupon(), context), {
		usable: true,
		reason: null,
	});
});

test("cart coupon explains an unmet order threshold", () => {
	assert.deepEqual(
		getCartCouponAvailability(
			coupon({ thresholdAmount: 650, estimatedDiscount: 0 }),
			context,
		),
		{
			usable: false,
			reason: "再購買 NT$ 150 即可使用",
		},
	);
});

test("claim-required coupon cannot be selected before it is claimed", () => {
	assert.deepEqual(
		getCartCouponAvailability(
			coupon({ claimRequired: true, claimed: false }),
			context,
		),
		{
			usable: false,
			reason: "需先到領券中心領取",
		},
	);
});

test("expired coupon is unavailable even with a backend discount", () => {
	assert.deepEqual(
		getCartCouponAvailability(
			coupon({ expires_at: Date.UTC(2026, 6, 10) }),
			context,
		),
		{
			usable: false,
			reason: "優惠券已逾期",
		},
	);
});
