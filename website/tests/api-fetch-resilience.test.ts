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

test("fetchSeller soft-fails to null on 5xx (build immunity)", async () => {
	respond = () => new Response("bad gateway", { status: 502 });
	const { fetchSeller } = await import("../lib/api/index");
	const result = await fetchSeller("1");
	assert.equal(result, null);
});

test("fetchSeller still returns null on 404", async () => {
	respond = () => new Response("not found", { status: 404 });
	const { fetchSeller } = await import("../lib/api/index");
	assert.equal(await fetchSeller("1"), null);
});

// getCachedSeller reads through fetchSeller(id, true): a 5xx must THROW so the
// unstable_cache entry keeps its prior stale seller rather than caching an
// outage-time null; a real 404 stays a cacheable null.
test("fetchSeller(strict) THROWS on 5xx (cache must not swallow an outage)", async () => {
	respond = () => new Response("bad gateway", { status: 502 });
	const { fetchSeller } = await import("../lib/api/index");
	await assert.rejects(() => fetchSeller("1", true));
});

test("fetchSeller(strict) still returns null on 404 (cacheable not-found)", async () => {
	respond = () => new Response("not found", { status: 404 });
	const { fetchSeller } = await import("../lib/api/index");
	assert.equal(await fetchSeller("1", true), null);
});

test("fetchSeller rejects malformed ids without calling the backend", async () => {
	const { fetchSeller } = await import("../lib/api/index");
	assert.equal(await fetchSeller("styles.css.map"), null);
	assert.equal(lastInit, undefined);
});

test("fetchSeller attaches an abort signal (timeout)", async () => {
	const { fetchSeller } = await import("../lib/api/index");
	await fetchSeller("1");
	assert.ok(lastInit?.signal, "expected a timeout AbortSignal on the request");
});

test("fetchSeller exposes only canonical logo_url to UI consumers", async () => {
	respond = () =>
		new Response(
			JSON.stringify({
				code: 0,
				data: {
					id: "1",
					logoUrl: " https://media.example.com/i/avatar/store.png ",
					logoKey: "public/store.png",
					logo: "https://legacy.example/logo.png",
					logo_url: "https://legacy.example/snake.png",
					avatar: "https://legacy.example/avatar.png",
				},
			}),
			{
				status: 200,
				headers: { "content-type": "application/json" },
			},
		);
	const { fetchSeller } = await import("../lib/api/index");
	const seller = await fetchSeller("1");

	assert.equal(
		seller?.logo_url,
		"https://media.example.com/i/avatar/store.png",
	);
	for (const nonDisplayField of ["logoKey", "logo", "logoUrl", "avatar"]) {
		assert.equal(Object.hasOwn(seller ?? {}, nonDisplayField), false);
	}
});

const okEnvelope = () =>
	new Response(JSON.stringify({ code: 0, data: {} }), {
		status: 200,
		headers: { "content-type": "application/json" },
	});

// A mutation on one URI does not invalidate a sibling GET in the browser HTTP
// cache, so authed/dynamic gateway reads must opt out of it — otherwise a soft
// navigation serves stale data until a hard reload (the 下架→編輯 stale-status bug).
test("browser-side requests opt out of the HTTP cache (no-store)", async () => {
	respond = okEnvelope;
	(globalThis as { window?: unknown }).window = {
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
	};
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.get("/products/1");
	assert.equal(
		lastInit?.cache,
		"no-store",
		"a soft-nav GET must not be served from a stale browser cache",
	);
});

test("server-side requests do not force no-store (Data Cache intact)", async () => {
	// No window stub → server branch. Forcing no-store here would opt cached
	// routes out of Next's Data Cache / Cache Components.
	respond = okEnvelope;
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.get("/products/1");
	assert.equal(lastInit?.cache, undefined);
});

test("an explicit cache option overrides the browser default", async () => {
	respond = okEnvelope;
	(globalThis as { window?: unknown }).window = {
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
	};
	const { apiClient } = await import("../lib/api/client");
	await apiClient.authed.get("/products/1", { cache: "force-cache" });
	assert.equal(lastInit?.cache, "force-cache");
});

// Regression: callers that treat "no session" as a normal outcome pass
// throwOnHttpError:false and must NOT get a throw on the 401 — else they abort
// mid-flight and Sentry logs a spurious UnauthorizedError. (logout() takes the
// throwing default instead and tolerates UnauthorizedError itself, so a 503 —
// the gateway saying it did NOT revoke — cannot pass for a sign-out.)
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
			headers: { "cf-ray": "a1feaf706bfbd6a9-TPE" },
		});
	const { apiClient, UnauthorizedError } = await import("../lib/api/client");
	await assert.rejects(
		() => apiClient.authed.get("/products/1"),
		(e: unknown) =>
			e instanceof UnauthorizedError &&
			e.cloudflareRayId === "a1feaf706bfbd6a9",
	);
});
