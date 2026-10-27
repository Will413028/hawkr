import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

const realFetch = globalThis.fetch;
const globals = globalThis as Record<string, unknown>;

let fetchCalls: Array<{ url: string; init: RequestInit }>;
let responseBody: unknown;

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

function bodyJson(call: { init: RequestInit }): unknown {
	return call.init.body ? JSON.parse(String(call.init.body)) : undefined;
}

beforeEach(() => {
	fetchCalls = [];
	responseBody = { code: 0, data: {} };
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	globals.window = {
		dispatchEvent() {},
		localStorage: fakeLocalStorage,
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
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

test("fetchAvailableCoupons normalizes cart template records and compensation fields", async () => {
	responseBody = {
		code: 0,
		data: {
			total: 1,
			records: [
				{
					template: {
						templateId: "tpl-comp-1",
						name: "售後補償券",
						type: "1",
						discountAmount: "80",
						thresholdAmount: "300",
						targetType: "4",
						issuerType: "2",
						claimRequired: "true",
						claimed: "false",
						sellerName: "小農商店",
					},
					estimatedDiscount: "80",
				},
			],
		},
	};

	const { fetchAvailableCoupons } = await import("../lib/coupons/repo");
	const result = await fetchAvailableCoupons({
		storeId: "seller-1",
		orderAmount: 265,
	});

	assert.equal(result.total, 1);
	assert.equal(fetchCalls.length, 1);
	assert.match(fetchCalls[0].url, /\/app\/coupons\/available\?/);
	assert.match(fetchCalls[0].url, /store_id=seller-1/);
	assert.match(fetchCalls[0].url, /order_amount=265/);

	const coupon = result.records[0];
	assert.equal(coupon.templateId, "tpl-comp-1");
	assert.equal(coupon.name, "售後補償券");
	assert.equal(coupon.kind, "cash");
	assert.equal(coupon.value, 80);
	assert.equal(coupon.min_spend, 300);
	assert.equal(coupon.targetType, "4");
	assert.equal(coupon.issuerType, "2");
	assert.equal(coupon.claimRequired, true);
	assert.equal(coupon.claimed, false);
	assert.equal(coupon.estimatedDiscount, 80);
});

test("fetchAvailableCoupons short-circuits when backend cart context is missing", async () => {
	const { fetchAvailableCoupons } = await import("../lib/coupons/repo");
	const result = await fetchAvailableCoupons({ pageSize: 50 });
	assert.deepEqual(result, { records: [], total: 0 });
	assert.equal(fetchCalls.length, 0);
});

test("fetchMyCoupons reads usedRecords for the used tab", async () => {
	responseBody = {
		code: 0,
		data: {
			records: [
				{
					instanceId: "ci-unused",
					templateId: "tpl-unused",
					name: "未使用券",
					type: "2",
					discountPercent: "10",
					status: "0",
				},
			],
			usedRecords: [
				{
					instanceId: "ci-used",
					templateId: "tpl-used",
					name: "已使用補償券",
					type: "1",
					discountAmount: "80",
					target_type: "4",
					claim_required: false,
					is_claimed: true,
				},
			],
		},
	};

	const { fetchMyCoupons } = await import("../lib/coupons/repo");
	const records = await fetchMyCoupons({
		status: "used",
		page: 2,
		pageSize: 10,
	});

	assert.equal(records.length, 1);
	assert.match(fetchCalls[0].url, /status=2/);
	assert.match(fetchCalls[0].url, /page=2/);
	assert.equal(records[0].instanceId, "ci-used");
	assert.equal(records[0].status, "used");
	assert.equal(records[0].targetType, "4");
	assert.equal(records[0].claimRequired, false);
	assert.equal(records[0].claimed, true);
});

test("claimCoupon posts to the encoded claim endpoint", async () => {
	responseBody = { code: 0, data: { success: true } };

	const { claimCoupon } = await import("../lib/coupons/repo");
	const ok = await claimCoupon("tpl comp/1");

	assert.equal(ok, true);
	assert.equal(fetchCalls.length, 1);
	assert.match(fetchCalls[0].url, /\/app\/coupons\/tpl%20comp%2F1\/claim$/);
	assert.equal(fetchCalls[0].init.method, "POST");
	assert.equal(bodyJson(fetchCalls[0]), undefined);
});
