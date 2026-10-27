import assert from "node:assert/strict";
import { test } from "node:test";
import { returnFailureGuidance } from "../lib/after-sales/return-failure-guidance";
import { ShippingMethod } from "../lib/codemeta/enums";
import { TCAT_BRANCH_LOOKUP_URL } from "../lib/seller/shipment";

test("returnFailureGuidance reuses shipment address guidance for 黑貓 address range failures", () => {
	const guidance = returnFailureGuidance({
		method: ShippingMethod.TAQBIN,
		providerMessage: "呼叫黑貓失敗：寄件地址不在簽約可收貨範圍，請聯繫簽約站所",
	});

	assert.ok(guidance.cause.includes("簽約"));
	assert.ok(guidance.steps.some((step) => step.includes("PayUni")));
	assert.ok(
		guidance.links.some((link) => link.href === TCAT_BRANCH_LOOKUP_URL),
	);
});

test("returnFailureGuidance gives 7-11 return-failure steps without changing logistics method", () => {
	const guidance = returnFailureGuidance({
		method: ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP,
		providerMessage: "退貨代碼逾期",
	});

	assert.ok(guidance.cause.includes("退貨物流"));
	assert.ok(guidance.steps.some((step) => step.includes("同原訂單物流")));
	assert.deepEqual(guidance.links, []);
});

test("returnFailureGuidance falls back to a generic safe handling path", () => {
	const guidance = returnFailureGuidance({});

	assert.ok(guidance.cause.includes("不能直接"));
	assert.ok(guidance.steps.some((step) => step.includes("客服")));
});
