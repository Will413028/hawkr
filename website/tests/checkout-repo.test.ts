import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

const realFetch = globalThis.fetch;
let calls: Array<{ url: string; init: RequestInit }> = [];

const fakeLocalStorage = {
	store: { "hkui:auth:active-role": "buyer" } as Record<string, string>,
	getItem(key: string) {
		return this.store[key] ?? null;
	},
	setItem(key: string, value: string) {
		this.store[key] = value;
	},
	removeItem(key: string) {
		delete this.store[key];
	},
};

beforeEach(() => {
	calls = [];
	__setEnvForTest(null);
	__setFlavorForTest("dev");
	getEnv();
	globalThis.fetch = async (
		url: string | URL | Request,
		init: RequestInit = {},
	) => {
		calls.push({ url: String(url), init });
		const data = String(url).includes("/checkout/preview")
			? {
					checkoutId: "preview-123",
					subtotal: "900",
					shippingFee: "80",
					discountAmount: "0",
					totalPayAmount: "980",
				}
			: { orderNos: ["O123456789"], totalPayAmount: "999" };
		return new Response(
			JSON.stringify({
				code: 0,
				data,
			}),
			{ status: 200, headers: { "content-type": "application/json" } },
		);
	};
	(globalThis as Record<string, unknown>).window = {
		dispatchEvent() {},
		localStorage: fakeLocalStorage,
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
	};
	(globalThis as Record<string, unknown>).localStorage = fakeLocalStorage;
});

afterEach(() => {
	globalThis.fetch = realFetch;
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete (globalThis as Record<string, unknown>).window;
	delete (globalThis as Record<string, unknown>).localStorage;
});

test("confirmCheckout sends OpenAPI string quantities while preserving 64-bit ids as strings", async () => {
	const { confirmCheckout } = await import("../lib/checkout/repo");

	const result = await confirmCheckout({
		checkoutId: "checkout-123",
		items: [
			{
				skuId: "101",
				productId: "843856384063471760",
				quantity: 9,
			},
		],
		shippingMethod: "3001",
		deliveryTemp: "3202",
		receiverZipCode: "330",
		shippingNote: "",
		orderRemark: "",
		recipientName: "Buyer",
		recipientPhone: "0900000000",
		shippingAddress: "Taoyuan",
	});

	assert.deepEqual(result.orderNos, ["O123456789"]);
	assert.equal(result.totalPayAmount, 999);
	assert.equal(calls.length, 1);
	assert.equal(calls[0].url, `${getEnv().apiBaseUrl}/app/checkout/confirm`);
	const body = JSON.parse(String(calls[0].init.body));
	assert.deepEqual(body.items, [
		{
			skuId: "101",
			productId: "843856384063471760",
			quantity: "9",
		},
	]);
	assert.equal(typeof body.items[0].quantity, "string");
	assert.equal(typeof body.items[0].productId, "string");
	assert.equal(body.shippingNote, "");
	assert.equal(body.orderRemark, "");
});

test("confirmCheckout always includes OpenAPI-required recipient fields", async () => {
	const { confirmCheckout } = await import("../lib/checkout/repo");

	await confirmCheckout({
		checkoutId: "checkout-123",
		items: [
			{
				skuId: "101",
				productId: "843856384063471760",
				quantity: 9,
			},
		],
		shippingMethod: "3001",
		deliveryTemp: "3202",
	});

	assert.equal(calls.length, 1);
	const body = JSON.parse(String(calls[0].init.body));
	assert.equal(body.recipientName, "");
	assert.equal(body.recipientPhone, "");
});

test("previewCheckout keeps legacy string quantities for backend preview validation", async () => {
	const { previewCheckout } = await import("../lib/checkout/repo");

	const result = await previewCheckout({
		items: [
			{
				skuId: "101",
				productId: "843856384063471760",
				quantity: 9,
			},
		],
		shippingMethod: "3001",
		deliveryTemp: "3202",
		receiverZipCode: "330",
	});

	assert.equal(result.checkoutId, "preview-123");
	assert.equal(result.totalPayAmount, 980);
	assert.equal(calls.length, 1);
	assert.equal(calls[0].url, `${getEnv().apiBaseUrl}/app/checkout/preview`);
	const body = JSON.parse(String(calls[0].init.body));
	assert.deepEqual(body.items, [
		{
			skuId: "101",
			productId: "843856384063471760",
			quantity: "9",
		},
	]);
	assert.equal(typeof body.items[0].quantity, "string");
});
