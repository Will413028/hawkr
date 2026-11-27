import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";

const realFetch = globalThis.fetch;
let respond: (url: string, init: RequestInit) => Response;
let lastInit: RequestInit | undefined;

beforeEach(() => {
	respond = () =>
		new Response(JSON.stringify({ data: { id: "1", name: "S" } }), {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	globalThis.fetch = (async (
		url: string | URL | Request,
		init: RequestInit = {},
	) => {
		lastInit = init;
		return respond(String(url), init);
	}) as typeof fetch;
});
afterEach(() => {
	globalThis.fetch = realFetch;
	lastInit = undefined;
	delete (globalThis as { window?: unknown }).window;
});

const okEnvelope = () =>
	new Response(JSON.stringify({ code: 0, data: {} }), {
		status: 200,
		headers: { "content-type": "application/json" },
	});

test("browser-side requests opt out of the HTTP cache (no-store)", async () => {
	respond = okEnvelope;
	(globalThis as { window?: unknown }).window = {};
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.get("/products/1");
	assert.equal(
		lastInit?.cache,
		"no-store",
		"a soft-nav GET must not be served from a stale browser cache",
	);
});

test("server-side requests do not force no-store (Data Cache intact)", async () => {
	respond = okEnvelope;
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.get("/products/1");
	assert.equal(lastInit?.cache, undefined);
});

test("an explicit cache option overrides the browser default", async () => {
	respond = okEnvelope;
	(globalThis as { window?: unknown }).window = {};
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.get("/products/1", { cache: "force-cache" });
	assert.equal(lastInit?.cache, "force-cache");
});

test("401 with throwOnHttpError:false returns the envelope instead of throwing", async () => {
	respond = () =>
		new Response(
			JSON.stringify({ code: 401, message: "missing or invalid credentials" }),
			{
				status: 401,
				headers: { "content-type": "application/json" },
			},
		);
	const { apiClient } = await import("../lib/api/client");
	const res = await apiClient.authed.delete("/auth/sessions", {
		throwOnHttpError: false,
		throwOnEnvelopeError: false,
		suppressAuthExpired: true,
	});
	assert.equal(res.code, 401);
});

test("401 still throws UnauthorizedError by default (throwOnHttpError unset)", async () => {
	respond = () =>
		new Response(JSON.stringify({ code: 401 }), {
			status: 401,
		});
	const { apiClient, UnauthorizedError } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.authed.get("/products/1"),
		(e: unknown) => e instanceof UnauthorizedError,
	);
});

test("server side: an authed GET forwards the cookie the caller handed over, and never a Bearer", async () => {
	respond = okEnvelope;
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.get("/products/1", {
		cookie: "__Host-hk_session=opaque",
	});
	const headers = new Headers(lastInit?.headers);
	assert.equal(headers.get("Cookie"), "__Host-hk_session=opaque");
	assert.equal(headers.get("Authorization"), null);
});

test("server side: an authed write does not get the cookie", async () => {
	respond = okEnvelope;
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.post("/products/1", {
		json: {},
		cookie: "__Host-hk_session=opaque",
	});
	assert.equal(new Headers(lastInit?.headers).get("Cookie"), null);
});

test("server side: without a handed-over cookie, or on a public read, no Cookie is sent", async () => {
	respond = okEnvelope;
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.get("/products/1");
	assert.equal(new Headers(lastInit?.headers).get("Cookie"), null);
	await apiClient.public.get("/products/1", {
		cookie: "__Host-hk_session=opaque",
	});
	assert.equal(new Headers(lastInit?.headers).get("Cookie"), null);
});
