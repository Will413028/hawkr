import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

const realFetch = globalThis.fetch;
const globals = globalThis as Record<string, unknown>;

let fetchCalls: Array<{ url: string; init: RequestInit }>;
let responseBody: unknown;

const fakeLocalStorage = {
	store: { "hkui:auth:active-role": "seller" } as Record<string, string>,
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

function bodyJson<T = unknown>(call: { init: RequestInit }): T {
	return JSON.parse(String(call.init.body)) as T;
}

beforeEach(() => {
	fetchCalls = [];
	responseBody = { code: 0, data: {} };
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	globals.window = {
		dispatchEvent() {},
		localStorage: fakeLocalStorage,
	};
	globals.localStorage = fakeLocalStorage;
	globalThis.fetch = async (
		url: string | URL | Request,
		init: RequestInit = {},
	) => {
		fetchCalls.push({ url: String(url), init });
		return new Response(JSON.stringify(responseBody), {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	};
});

afterEach(() => {
	globalThis.fetch = realFetch;
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete globals.window;
	delete globals.localStorage;
});

test("resolveAfterSale posts compensation-coupon decisions without leaking caseNo", async () => {
	responseBody = {
		code: 0,
		data: {
			return_shipment_no: "S001",
			partner_id: "P001",
			validation_no: "V001",
		},
	};

	const { resolveAfterSale } = await import("../lib/after-sales/seller-repo");
	const result = await resolveAfterSale({
		caseNo: "AS 001",
		resolutionType: "4",
		couponAmount: "80",
		couponMinSpend: "300",
		couponMessage: "售後補償",
	});

	assert.equal(fetchCalls.length, 1);
	assert.match(
		fetchCalls[0].url,
		/\/app\/seller\/after_sales\/AS%20001\/resolve$/,
	);
	assert.equal(fetchCalls[0].init.method, "POST");
	assert.deepEqual(bodyJson(fetchCalls[0]), {
		resolutionType: "4",
		couponAmount: "80",
		couponMinSpend: "300",
		couponMessage: "售後補償",
	});
	assert.deepEqual(result, {
		returnShipmentNo: "S001",
		partnerId: "P001",
		validationNo: "V001",
	});
});

test("confirmReceipt always sends inspectionPhotoTokens as an array", async () => {
	const { confirmReceipt } = await import("../lib/after-sales/seller-repo");
	const ok = await confirmReceipt({
		caseNo: "AS001",
		actualRefundAmount: "120",
		deductionReason: "商品已拆封",
	});

	assert.equal(ok, true);
	assert.match(
		fetchCalls[0].url,
		/\/app\/seller\/after_sales\/AS001\/confirm_receipt$/,
	);
	assert.deepEqual(bodyJson(fetchCalls[0]), {
		actualRefundAmount: "120",
		deductionReason: "商品已拆封",
		inspectionPhotoTokens: [],
	});
});

test("fetchRefundPage posts WEB_BROWSER and reads redirect aliases", async () => {
	responseBody = {
		code: 0,
		data: { redirect_url: "https://payuni.test/refund" },
	};

	const { fetchRefundPage } = await import("../lib/after-sales/buyer-repo");
	const result = await fetchRefundPage("ORD/001");

	assert.deepEqual(result, { redirectUrl: "https://payuni.test/refund" });
	assert.match(fetchCalls[0].url, /\/app\/orders\/ORD%2F001\/refund_page$/);
	assert.equal(fetchCalls[0].init.method, "POST");
	assert.deepEqual(bodyJson(fetchCalls[0]), { clientType: "WEB_BROWSER" });
});

test("buyer after-sale creation and follow-up actions keep their API bodies stable", async () => {
	responseBody = { code: 0, data: { case_no: "AS900" } };
	const {
		appealAfterSale,
		cancelAfterSale,
		createAfterSale,
		supplementAfterSale,
	} = await import("../lib/after-sales/buyer-repo");

	const created = await createAfterSale({
		orderNo: "O001",
		reason: "5001",
		description: "商品損壞",
	});
	responseBody = { code: 0, data: {} };
	await cancelAfterSale("AS900");
	await supplementAfterSale("AS900", {
		additionalDescription: "新增照片",
		photoTokens: ["tok-1"],
	});
	await appealAfterSale("AS900", {
		reason: "平台介入",
		description: "買賣雙方無共識",
		evidencePhotoTokens: ["arb-1"],
	});

	assert.deepEqual(created, { caseNo: "AS900" });
	assert.match(fetchCalls[0].url, /\/app\/after_sales$/);
	assert.deepEqual(bodyJson(fetchCalls[0]), {
		orderNo: "O001",
		reason: "5001",
		description: "商品損壞",
		photoTokens: [],
	});
	assert.match(fetchCalls[1].url, /\/app\/after_sales\/AS900\/cancel$/);
	assert.equal(fetchCalls[1].init.body, undefined);
	assert.deepEqual(bodyJson(fetchCalls[2]), {
		additionalDescription: "新增照片",
		photoTokens: ["tok-1"],
	});
	assert.deepEqual(bodyJson(fetchCalls[3]), {
		reason: "平台介入",
		description: "買賣雙方無共識",
		evidencePhotoTokens: ["arb-1"],
	});
});
