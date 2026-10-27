import assert from "node:assert/strict";
import { test } from "node:test";
import { isFreeShippingForSubtotal } from "../lib/checkout/free-shipping";
import type { AvailableShippingOption } from "../lib/checkout/repo";
import { ShippingFeePayer } from "../lib/codemeta/enums";

const option = (
	patch: Partial<AvailableShippingOption>,
): AvailableShippingOption => ({
	shippingMethod: "3001",
	label: "黑貓宅急便",
	fee: 160,
	freeShippingEligible: false,
	...patch,
});

test("a buyer-paid shipping threshold never makes the cart free", () => {
	assert.equal(
		isFreeShippingForSubtotal(
			option({
				feePayer: ShippingFeePayer.BUYER,
				freeShippingThreshold: 3000,
			}),
			3000,
		),
		false,
	);
});

test("a seller-funded threshold becomes free only after the subtotal reaches it", () => {
	const sellerThreshold = option({
		feePayer: ShippingFeePayer.SELLER,
		freeShippingThreshold: 3000,
	});
	assert.equal(isFreeShippingForSubtotal(sellerThreshold, 2999), false);
	assert.equal(isFreeShippingForSubtotal(sellerThreshold, 3000), true);
});

test("the backend's free-shipping result remains authoritative", () => {
	assert.equal(
		isFreeShippingForSubtotal(
			option({
				feePayer: ShippingFeePayer.SELLER,
				freeShippingEligible: true,
			}),
			0,
		),
		true,
	);
});
