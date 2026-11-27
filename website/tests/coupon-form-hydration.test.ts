import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function source(path: string): Promise<string> {
	return readFile(new URL(path, import.meta.url), "utf8");
}

test("coupon form first render does not wait on certification state", async () => {
	const form = await source(
		"../features/seller-center/coupons/CouponFormClient.tsx",
	);

	assert.match(form, /useHydrationSafeClientRender/);
	assert.match(form, /if \(!hydrationSafeClientRender \|\| loading\)/);
});

test("coupon surfaces carry no KYC gate", async () => {
	for (const path of [
		"../features/seller-center/coupons/CouponFormClient.tsx",
		"../features/seller-center/coupons/SellerCouponsClient.tsx",
	]) {
		const code = await source(path);
		assert.doesNotMatch(
			code,
			/useCertGate|certGate|SellerCertGate|CertGateDialog/,
		);
	}
});
