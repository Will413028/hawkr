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
