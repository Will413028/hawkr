import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { withApiBase } from "./helpers/default-api-base";

let calls: Array<{ url: string; init: RequestInit }> = [];
const realFetch = globalThis.fetch;
let dispatchedEvents: Event[] = [];
let restoreApiBasePins: () => void = () => {};

const fakeLocalStorage = {
	store: { "hkui:auth:active-role": "seller" } as Record<string, string>,
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
	calls = [];
	dispatchedEvents = [];
	// prod has no default gateway: its base comes from env, not from the loopback pins of the test guard
	restoreApiBasePins = withApiBase("https://api.example.com/api/v1");
	__setEnvForTest(null);
	__setFlavorForTest("prod"); // absolute base so URL assertions are stable
	getEnv();
	// @ts-expect-error test stub
	globalThis.fetch = async (url: string, init: RequestInit = {}) => {
		calls.push({ url: String(url), init });
		return new Response(JSON.stringify({ code: 0, data: { ok: true } }), {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	};
	// stub window with dispatchEvent capture + localStorage
	(globalThis as Record<string, unknown>).window = {
		dispatchEvent(evt: Event) {
			dispatchedEvents.push(evt);
		},
		localStorage: fakeLocalStorage,
	};
	(globalThis as Record<string, unknown>).localStorage = fakeLocalStorage;
});

afterEach(() => {
	globalThis.fetch = realFetch;
	restoreApiBasePins();
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete (globalThis as Record<string, unknown>).window;
	delete (globalThis as Record<string, unknown>).localStorage;
});

test("public GET goes direct to gateway WITHOUT credentials", async () => {
	const { apiClient } = await import("../lib/api/client");
	await apiClient.public.get("/products");
	const c = calls[0];
	assert.equal(c.url, `${getEnv().gatewayHost}/api/v1/public/products`);
	// Anonymous read must not carry the session cookie (cache hygiene + exposure).
	assert.equal(
		(c.init as RequestInit & { credentials: string }).credentials,
		"omit",
	);
});

test("public POST honours an explicit credentials:'include' (login Set-Cookie)", async () => {
	// A session-establishing login (POST /public/auth/web/*) must keep "include"
	// so the gateway's Set-Cookie response is stored; the public default is "omit".
	const { apiClient } = await import("../lib/api/client");
	await apiClient.public.post("/auth/web/google", {
		json: { idToken: "x" },
		credentials: "include",
	});
	const c = calls[0];
	assert.equal(
		(c.init as RequestInit & { credentials: string }).credentials,
		"include",
	);
});

test("browser login accepts a same-origin relative API base", async () => {
	const absolute = getEnv();
	__setEnvForTest({
		...absolute,
		apiBaseUrl: "/api/v1",
		publicApi: "/api/v1/public",
		appApi: "/api/v1/app",
	});
	const { apiClient } = await import("../lib/api/client");

	await apiClient.public.post("/auth/web/google", {
		json: { idToken: "x" },
		credentials: "include",
	});

	assert.equal(calls[0]?.url, "/api/v1/public/auth/web/google");
});

test("authed POST carries X-Active-Role from stored role and no CSRF header", async () => {
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.post("/carts/items", { json: { sku: 1 } });
	const c = calls[0];
	assert.equal(c.url, `${getEnv().gatewayHost}/api/v1/app/carts/items`);
	const h = new Headers(c.init.headers);
	// The monolith's CSRF check reads Sec-Fetch-Site / Origin, not a custom header.
	assert.equal(h.get("X-Requested-With"), null);
	// The credential is the HttpOnly session cookie the browser attaches itself.
	assert.equal(h.get("Authorization"), null);
	assert.equal(h.get("X-Active-Role"), "SELLER");
	assert.equal(
		(c.init as RequestInit & { credentials: string }).credentials,
		"include",
	);
});

test("authed GET on 401 throws UnauthorizedError and emits AUTH_EXPIRED_EVENT", async () => {
	// @ts-expect-error test stub
	globalThis.fetch = async (url: string, init: RequestInit = {}) => {
		calls.push({ url: String(url), init });
		return new Response(
			JSON.stringify({ code: 401, message: "Unauthorized" }),
			{
				status: 401,
				headers: { "content-type": "application/json" },
			},
		);
	};
	const { apiClient, UnauthorizedError, AUTH_EXPIRED_EVENT } = await import(
		"../lib/api/client"
	);
	await assert.rejects(
		() => apiClient.authed.get("/profile"),
		(err: unknown) => {
			assert.ok(
				err instanceof UnauthorizedError,
				"should be UnauthorizedError",
			);
			assert.equal((err as UnauthorizedError).status, 401);
			return true;
		},
	);
	assert.equal(dispatchedEvents.length, 1, "should emit exactly one event");
	assert.equal(
		calls.length,
		1,
		"a 401 is the answer: no token refresh, no retry",
	);
	assert.equal((dispatchedEvents[0] as CustomEvent).type, AUTH_EXPIRED_EVENT);
});

test("authed request with suppressAuthExpired:true throws but does NOT emit", async () => {
	// @ts-expect-error test stub
	globalThis.fetch = async (url: string, init: RequestInit = {}) => {
		calls.push({ url: String(url), init });
		return new Response(
			JSON.stringify({ code: 401, message: "Unauthorized" }),
			{
				status: 401,
				headers: { "content-type": "application/json" },
			},
		);
	};
	const { apiClient, UnauthorizedError } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.authed.get("/profile", { suppressAuthExpired: true }),
		(err: unknown) => {
			assert.ok(
				err instanceof UnauthorizedError,
				"should be UnauthorizedError",
			);
			return true;
		},
	);
	assert.equal(dispatchedEvents.length, 0, "should NOT emit when suppressed");
});

test("authed GET retries once on 503 then succeeds (B-199)", async () => {
	// A transient Redis blip makes the gateway answer 503; the client absorbs it
	// with a single retry so the user never sees the failure.
	let attempt = 0;
	// @ts-expect-error test stub
	globalThis.fetch = async (url: string, init: RequestInit = {}) => {
		calls.push({ url: String(url), init });
		attempt += 1;
		if (attempt === 1) {
			return new Response(
				JSON.stringify({
					code: 503,
					message: "service temporarily unavailable",
				}),
				{ status: 503, headers: { "content-type": "application/json" } },
			);
		}
		return new Response(JSON.stringify({ code: 0, data: { ok: true } }), {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	};
	const { apiClient } = await import("../lib/api/client");
	const res = await apiClient.authed.get<{ ok: boolean }>("/profile");
	assert.equal(res.code, 0);
	assert.equal(calls.length, 2, "should retry exactly once");
});

test("authed GET on persistent 503 throws ApiClientError, does NOT emit, retries once (B-199)", async () => {
	// @ts-expect-error test stub
	globalThis.fetch = async (url: string, init: RequestInit = {}) => {
		calls.push({ url: String(url), init });
		return new Response(
			JSON.stringify({ code: 503, message: "service temporarily unavailable" }),
			{ status: 503, headers: { "content-type": "application/json" } },
		);
	};
	const { apiClient, ApiClientError, UnauthorizedError } = await import(
		"../lib/api/client"
	);
	await assert.rejects(
		() => apiClient.authed.get("/profile"),
		(err: unknown) => {
			assert.ok(err instanceof ApiClientError, "should be ApiClientError");
			assert.ok(
				!(err instanceof UnauthorizedError),
				"503 must NOT be an UnauthorizedError",
			);
			assert.equal((err as ApiClientError).status, 503);
			return true;
		},
	);
	assert.equal(calls.length, 2, "1 attempt + 1 retry");
	assert.equal(
		dispatchedEvents.length,
		0,
		"503 must NOT emit AUTH_EXPIRED_EVENT (it is not a logout)",
	);
});

test("authed POST does NOT retry on 503 (non-idempotent) (B-199)", async () => {
	// @ts-expect-error test stub
	globalThis.fetch = async (url: string, init: RequestInit = {}) => {
		calls.push({ url: String(url), init });
		return new Response(JSON.stringify({ code: 503, message: "down" }), {
			status: 503,
			headers: { "content-type": "application/json" },
		});
	};
	const { apiClient } = await import("../lib/api/client");
	await assert.rejects(() =>
		apiClient.authed.post("/carts/items", { json: { sku: 1 } }),
	);
	assert.equal(calls.length, 1, "POST must not be auto-retried");
});

test("public 401 throws but does NOT emit", async () => {
	// @ts-expect-error test stub
	globalThis.fetch = async (url: string, init: RequestInit = {}) => {
		calls.push({ url: String(url), init });
		return new Response(
			JSON.stringify({ code: 401, message: "Unauthorized" }),
			{
				status: 401,
				headers: { "content-type": "application/json" },
			},
		);
	};
	const { apiClient, UnauthorizedError } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.public.get("/products"),
		(err: unknown) => {
			assert.ok(
				err instanceof UnauthorizedError,
				"should be UnauthorizedError",
			);
			return true;
		},
	);
	assert.equal(
		dispatchedEvents.length,
		0,
		"public 401 should NOT emit AUTH_EXPIRED_EVENT",
	);
});
