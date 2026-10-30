/**
 * lib/seller/repo.ts — seller coupon request shape regression tests.
 */

import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

const realFetch = globalThis.fetch;
let calls: Array<{ url: string; init: RequestInit }> = [];

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
	__setEnvForTest(null);
	__setFlavorForTest("dev");
	getEnv();
	globalThis.fetch = async (
		url: string | URL | Request,
		init: RequestInit = {},
	) => {
		calls.push({ url: String(url), init });
		return jsonResponse({ code: 0, data: { templateId: "123" } });
	};
	fakeLocalStorage.store = { "hkui:auth:active-role": "seller" };
	(globalThis as Record<string, unknown>).window = {
		dispatchEvent() {},
		localStorage: fakeLocalStorage,
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

test("toCouponDateTime emits API-standard UTC Z timestamps", async () => {
	const { toCouponDateTime } = await import("../lib/seller/repo");
	const seconds = Date.parse("2026-06-19T00:00:00Z") / 1000;

	assert.equal(toCouponDateTime(seconds), "2026-06-19T00:00:00Z");
	assert.equal(toCouponDateTime(seconds, true), "2026-06-19T23:59:59Z");
});

test("createSellerCoupon POSTs UTC Z startAt/endAt instead of offset timestamps", async () => {
	const { createSellerCoupon } = await import("../lib/seller/repo");

	await createSellerCoupon({
		name: "6月回購券",
		type: "amount",
		value: 100,
		min_spend: 500,
		usage_limit: 100,
		starts_at: Date.parse("2026-06-19T00:00:00Z") / 1000,
		expires_at: Date.parse("2026-07-09T00:00:00Z") / 1000,
		targetType: "all",
		status: "active",
	});

	assert.equal(calls.length, 1);
	assert.equal(calls[0].url, `${getEnv().apiBaseUrl}/app/coupon_templates`);
	assert.equal(String(calls[0].init.method).toUpperCase(), "POST");

	const headers = new Headers(calls[0].init.headers);
	assert.equal(headers.get("X-Active-Role"), "SELLER");

	const body = JSON.parse(String(calls[0].init.body));
	assert.equal(body.startAt, "2026-06-19T00:00:00Z");
	assert.equal(body.endAt, "2026-07-09T23:59:59Z");
	assert.ok(!String(body.startAt).includes("+08:00"));
	assert.ok(!String(body.endAt).includes("+08:00"));
});

test("createSellerCoupon sends selected buyer ids as targetType=3", async () => {
	const { createSellerCoupon } = await import("../lib/seller/repo");

	await createSellerCoupon({
		name: "VIP 專屬券",
		type: "amount",
		value: 120,
		min_spend: 600,
		starts_at: Date.parse("2026-06-19T00:00:00Z") / 1000,
		expires_at: Date.parse("2026-07-09T00:00:00Z") / 1000,
		targetType: "specific",
		buyerAccountIds: ["101", "102"],
		status: "active",
	});

	assert.equal(calls.length, 1);
	const body = JSON.parse(String(calls[0].init.body));
	assert.equal(body.targetType, "3");
	assert.deepEqual(body.buyerAccountIds, ["101", "102"]);
});

test("fetchSellerCustomersPage sends CRM filters and preserves total", async () => {
	globalThis.fetch = async (
		url: string | URL | Request,
		init: RequestInit = {},
	) => {
		calls.push({ url: String(url), init });
		return jsonResponse({
			code: 0,
			data: {
				list: [
					{
						// `id` is the crm row PK — normalizeSellerCustomer must NOT
						// key the customer off it (nor off the seller's accountId).
						// `buyerId` is a TSID string on the wire.
						id: 900,
						accountId: 555,
						buyerId: "101",
						nickname: "Cora Chou",
						phone: "0977777777",
						orderCount: 3,
						customerType: "vip",
						tags: [{ id: 7, name: "高消費" }],
					},
				],
				total: "12",
				page: "2",
				pageSize: "50",
			},
		});
	};
	const { fetchSellerCustomersPage } = await import("../lib/seller/repo");

	const page = await fetchSellerCustomersPage({
		page: 2,
		pageSize: 50,
		keyword: "Cora",
		tagId: 7,
		customerType: "vip",
		sortBy: "order_count",
		sortOrder: "desc",
	});

	assert.equal(calls.length, 1);
	const url = new URL(calls[0].url, "http://localhost");
	assert.equal(url.pathname, "/api/v1/app/sellers/me/customers");
	assert.equal(url.searchParams.get("page"), "2");
	assert.equal(url.searchParams.get("page_size"), "50");
	assert.equal(url.searchParams.get("keyword"), "Cora");
	assert.equal(url.searchParams.get("tag_id"), "7");
	assert.equal(url.searchParams.get("customer_type"), "vip");
	assert.equal(url.searchParams.get("sort_by"), "order_count");
	assert.equal(page.total, 12);
	assert.equal(page.page, 2);
	assert.equal(page.pageSize, 50);
	assert.equal(page.records[0].id, "101");
	assert.equal(page.records[0].name, "Cora Chou");
	assert.equal(page.records[0].customerType, "vip");
	assert.deepEqual(page.records[0].tags, ["高消費"]);
});

test("fetchSellerCustomerLookup resolves the current buyer profile by buyer id", async () => {
	globalThis.fetch = async (
		url: string | URL | Request,
		init: RequestInit = {},
	) => {
		calls.push({ url: String(url), init });
		return jsonResponse({
			code: 0,
			data: {
				uid: "101",
				buyerName: "Hawkr 買家",
				avatarUrl: "https://cdn/buyer.png",
				phone: "0977777777",
			},
		});
	};
	const { fetchSellerCustomerLookup } = await import("../lib/seller/repo");

	const profile = await fetchSellerCustomerLookup("101");

	assert.equal(calls.length, 1);
	const url = new URL(calls[0].url, "http://localhost");
	assert.equal(url.pathname, "/api/v1/app/sellers/me/customers/lookup");
	assert.equal(url.searchParams.get("buyer_id"), "101");
	assert.deepEqual(profile, {
		id: "101",
		name: "Hawkr 買家",
		avatar: "https://cdn/buyer.png",
		phone: "0977777777",
	});
});
