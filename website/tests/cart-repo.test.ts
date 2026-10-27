/**
 * lib/cart/repo.ts — request shapes, response normalization and error paths
 * for the `/public/carts/*` HTTP repository. Mirrors the fetch-mock pattern
 * used by tests/checkout-repo.test.ts.
 */

import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

const realFetch = globalThis.fetch;
let calls: Array<{ url: string; init: RequestInit }> = [];
let respond: (url: string, init: RequestInit) => Response;

function jsonResponse(payload: unknown, status = 200): Response {
	return new Response(JSON.stringify(payload), {
		status,
		headers: { "content-type": "application/json" },
	});
}

const fakeLocalStorage = {
	store: {} as Record<string, string>,
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
	respond = () => jsonResponse({ code: 0, data: {} });
	__setEnvForTest(null);
	__setFlavorForTest("dev");
	getEnv();
	globalThis.fetch = async (
		url: string | URL | Request,
		init: RequestInit = {},
	) => {
		calls.push({ url: String(url), init });
		return respond(String(url), init);
	};
	fakeLocalStorage.store = { "hkui:auth:active-role": "buyer" };
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

/* ===== fetchCart ===== */

test("fetchCart GETs /public/carts and returns the normalized summary", async () => {
	const { fetchCart } = await import("../lib/cart/repo");
	respond = () =>
		jsonResponse({
			code: 0,
			data: {
				groups: [
					{
						storeId: "42",
						sellerName: "鮮果舖",
						items: [
							{ skuId: "101", name: "蘋果", price: "120", quantity: "2" },
						],
					},
				],
				totalItems: 2,
				estimatedTotalAmount: "240",
			},
		});

	const result = await fetchCart();

	assert.equal(calls.length, 1);
	assert.equal(calls[0].url, `${getEnv().apiBaseUrl}/public/carts`);
	assert.equal(String(calls[0].init.method).toUpperCase(), "GET");
	assert.equal(result.groups.length, 1);
	assert.equal(result.groups[0].storeId, "42");
	assert.equal(result.totalItems, 2);
	assert.equal(result.estimatedTotalAmount, "240");
});

test("fetchCart falls back to an empty summary when data is missing", async () => {
	const { fetchCart } = await import("../lib/cart/repo");
	respond = () => jsonResponse({ code: 0, data: null });

	const result = await fetchCart();

	assert.deepEqual(result, {
		groups: [],
		totalItems: 0,
		estimatedTotalAmount: "0",
	});
});

test("fetchCart surfaces 401 as UnauthorizedError", async () => {
	const { fetchCart } = await import("../lib/cart/repo");
	const { UnauthorizedError } = await import("../lib/api/client");
	respond = () => jsonResponse({ code: 401, message: "expired" }, 401);

	await assert.rejects(fetchCart(), (err: unknown) => {
		assert.ok(err instanceof UnauthorizedError);
		assert.equal(err.status, 401);
		return true;
	});
});

/* ===== addCartItem ===== */

test("addCartItem POSTs OpenAPI string quantity and coerces totalItems to number", async () => {
	const { addCartItem } = await import("../lib/cart/repo");
	respond = () => jsonResponse({ code: 0, data: { totalItems: "5" } });

	const result = await addCartItem({ skuId: "sku-9", quantity: 3 });

	assert.equal(calls.length, 1);
	assert.equal(calls[0].url, `${getEnv().apiBaseUrl}/public/carts/items`);
	assert.equal(String(calls[0].init.method).toUpperCase(), "POST");
	const body = JSON.parse(String(calls[0].init.body));
	assert.deepEqual(body, { skuId: "sku-9", quantity: "3" });
	assert.equal(typeof body.quantity, "string");
	assert.deepEqual(result, { totalItems: 5 });
});

test("addCartItem sends the active-role header on the write", async () => {
	const { addCartItem } = await import("../lib/cart/repo");
	respond = () => jsonResponse({ code: 0, data: { totalItems: 1 } });

	await addCartItem({ skuId: "101", quantity: 1 });

	const headers = new Headers(calls[0].init.headers);
	assert.equal(headers.get("X-Active-Role"), "BUYER");
	assert.equal(headers.get("Content-Type"), "application/json");
});

test("addCartItem defaults totalItems to 0 when the envelope has no data", async () => {
	const { addCartItem } = await import("../lib/cart/repo");
	respond = () => jsonResponse({ code: 0, data: {} });

	const result = await addCartItem({ skuId: "101", quantity: 1 });

	assert.deepEqual(result, { totalItems: 0 });
});

test("addCartItem throws ApiClientError carrying the backend envelope code", async () => {
	const { addCartItem } = await import("../lib/cart/repo");
	const { ApiClientError } = await import("../lib/api/client");
	// 6005 CHECKOUT_LOCK_FAILED — the guest/member cart mixup guard.
	respond = () =>
		jsonResponse({ code: 6005, msg: "CHECKOUT_LOCK_FAILED", data: null });

	await assert.rejects(
		addCartItem({ skuId: "101", quantity: 1 }),
		(err: unknown) => {
			assert.ok(err instanceof ApiClientError);
			assert.equal(err.code, 6005);
			assert.equal(err.message, "CHECKOUT_LOCK_FAILED");
			return true;
		},
	);
});

/* ===== setCartItemQuantity ===== */

test("setCartItemQuantity PUTs string quantity to the encoded sku path", async () => {
	const { setCartItemQuantity } = await import("../lib/cart/repo");
	respond = () => jsonResponse({ code: 0, data: { totalItems: 7 } });

	const result = await setCartItemQuantity("sku/詭異 id", 2);

	assert.equal(calls.length, 1);
	assert.equal(
		calls[0].url,
		`${getEnv().apiBaseUrl}/public/carts/items/${encodeURIComponent("sku/詭異 id")}/quantity`,
	);
	assert.equal(String(calls[0].init.method).toUpperCase(), "PUT");
	const body = JSON.parse(String(calls[0].init.body));
	assert.deepEqual(body, { quantity: "2" });
	assert.equal(typeof body.quantity, "string");
	assert.deepEqual(result, { totalItems: 7 });
});

test("setCartItemQuantity coerces a string totalItems from the wire", async () => {
	const { setCartItemQuantity } = await import("../lib/cart/repo");
	respond = () => jsonResponse({ code: 0, data: { totalItems: "12" } });

	const result = await setCartItemQuantity("101", 4);

	assert.deepEqual(result, { totalItems: 12 });
});

/* ===== removeCartItem ===== */

test("removeCartItem DELETEs the encoded sku path and returns totalItems", async () => {
	const { removeCartItem } = await import("../lib/cart/repo");
	respond = () => jsonResponse({ code: 0, data: { totalItems: 3 } });

	const result = await removeCartItem("sku 9");

	assert.equal(calls.length, 1);
	assert.equal(
		calls[0].url,
		`${getEnv().apiBaseUrl}/public/carts/items/sku%209`,
	);
	assert.equal(String(calls[0].init.method).toUpperCase(), "DELETE");
	assert.equal(calls[0].init.body, undefined);
	assert.deepEqual(result, { totalItems: 3 });
});

test("removeCartItem defaults totalItems to 0 on an empty envelope", async () => {
	const { removeCartItem } = await import("../lib/cart/repo");
	respond = () => jsonResponse({ code: 0, data: null });

	const result = await removeCartItem("101");

	assert.deepEqual(result, { totalItems: 0 });
});
