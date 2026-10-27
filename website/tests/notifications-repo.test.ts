import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { fetchNotifications } from "../lib/notifications/repo";

const realFetch = globalThis.fetch;
const globals = globalThis as Record<string, unknown>;
let fetchCalls: Array<{ url: string }>;

const fakeLocalStorage = {
	store: { "hkui:auth:active-role": "buyer" } as Record<string, string>,
	getItem(k: string) {
		return this.store[k] ?? null;
	},
	setItem(k: string, v: string) {
		this.store[k] = v;
	},
	removeItem(k: string) {
		delete this.store[k];
	},
};

beforeEach(() => {
	fetchCalls = [];
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	globals.window = {
		dispatchEvent() {},
		localStorage: fakeLocalStorage,
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
	};
	globals.localStorage = fakeLocalStorage;
	globalThis.fetch = async (url: string | URL | Request) => {
		fetchCalls.push({ url: String(url) });
		return new Response(JSON.stringify({ code: 0, data: { records: [] } }), {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	};
});

afterEach(() => {
	globalThis.fetch = realFetch;
});

test("fetchNotifications adds category query when provided", async () => {
	await fetchNotifications({ category: "order_update" });
	assert.ok(
		fetchCalls[0].url.includes("category=order_update"),
		fetchCalls[0].url,
	);
});

test("fetchNotifications omits category query when empty", async () => {
	await fetchNotifications({});
	assert.ok(!fetchCalls[0].url.includes("category="), fetchCalls[0].url);
});

test("fetchNotifications preserves a 64-bit cursor without Number coercion", async () => {
	const cursor = "9223372036854775807";
	await fetchNotifications({ cursorId: cursor });
	assert.equal(new URL(fetchCalls[0].url).searchParams.get("cursorId"), cursor);
});

test("fetchNotifications derives the next cursor from the exact string TSID", async () => {
	const cursor = "9223372036854775807";
	globalThis.fetch = async () =>
		new Response(
			JSON.stringify({
				code: 0,
				data: {
					records: [
						{
							id: cursor,
							type: "system",
							title: "游標測試",
							isRead: false,
							createdAt: "2026-08-31T00:00:00Z",
						},
					],
					hasMore: true,
				},
			}),
			{
				status: 200,
				headers: { "content-type": "application/json" },
			},
		);

	const result = await fetchNotifications({});
	assert.equal(result.nextCursorId, cursor);
});
