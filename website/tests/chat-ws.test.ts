import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { withApiBase } from "./helpers/default-api-base";

let nextResponse: { status: number; body: unknown };
const realFetch = globalThis.fetch;

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

let restoreApiBase: () => void = () => {};

beforeEach(() => {
	restoreApiBase = withApiBase("https://api.example.com/api/v1");
	__setEnvForTest(null);
	__setFlavorForTest("prod"); // gatewayHost = origin of the API base env
	getEnv();
	nextResponse = {
		status: 200,
		body: { code: 0, data: { ticket: "TICKET123", userId: "42", userType: 1 } },
	};
	// @ts-expect-error test stub
	globalThis.fetch = async () =>
		new Response(JSON.stringify(nextResponse.body), {
			status: nextResponse.status,
			headers: { "content-type": "application/json" },
		});
	(globalThis as Record<string, unknown>).window = {
		dispatchEvent() {},
		localStorage: fakeLocalStorage,
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
	};
	(globalThis as Record<string, unknown>).localStorage = fakeLocalStorage;
});

afterEach(() => {
	restoreApiBase();
	globalThis.fetch = realFetch;
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete (globalThis as Record<string, unknown>).window;
	delete (globalThis as Record<string, unknown>).localStorage;
});

test("buildWsUrl mints a ticket and returns a wss URL with the ticket query", async () => {
	const { buildWsUrl } = await import("../lib/chat/ws");
	const built = await buildWsUrl();
	assert.ok(built, "expected a built URL");
	assert.equal(
		built?.url,
		"wss://api.example.com/api/v1/app/chat/ws?ticket=TICKET123",
	);
	assert.equal(built?.userId, "42");
	assert.equal(built?.userType, 1);
});

test("buildWsUrl returns null when the mint request is unauthorized", async () => {
	nextResponse = { status: 401, body: { code: 401, message: "Unauthorized" } };
	const { buildWsUrl } = await import("../lib/chat/ws");
	const built = await buildWsUrl();
	assert.equal(built, null);
});

test("buildWsUrl returns null when a 200 response carries no ticket", async () => {
	nextResponse = { status: 200, body: { code: 0, data: {} } };
	const { buildWsUrl } = await import("../lib/chat/ws");
	const built = await buildWsUrl();
	assert.equal(built, null);
});
