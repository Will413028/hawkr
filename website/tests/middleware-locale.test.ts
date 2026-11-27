import assert from "node:assert/strict";
import { afterEach, beforeEach, mock, test } from "node:test";
import { NextRequest } from "next/server";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { __resetForumThreadMemoForTest } from "../lib/forum/thread-guard";
import { __setPublishedLocalesForTest } from "../lib/intl/locales";
import { __resetStorefrontGuardMemoForTest } from "../lib/seller/storefront-guard";
import { siteMiddleware } from "../middleware";

process.env.NEXT_PUBLIC_API_BASE_URL = "/api/v1";

/**
 * Locale routing as the middleware composes it (ADR 2026-10-17 D1): the
 * site's own decisions (the session cookie's presence) run on the locale-free pathname, and whatever passes is next-intl's
 * response. The test process runs the dev flavor, which routes every supported locale, so `/en`
 * is a locale here; the prod behaviour of the same code (`/en` is not a locale) is the unit
 * tests of lib/intl and the built-server smoke.
 */

const SIGNED_IN = { cookie: "__Host-hk_session=opaque" };

function request(path: string, headers: Record<string, string> = {}) {
	return new NextRequest(`http://localhost:3000${path}`, {
		headers: { host: "localhost:3000", ...headers },
	});
}

function location(response: Response): URL | null {
	const value = response.headers.get("location");
	return value ? new URL(value) : null;
}

/** The internal path next-intl rewrote the request to (`x-middleware-rewrite`), if any. */
function rewrittenTo(response: Response): string | null {
	const value = response.headers.get("x-middleware-rewrite");
	return value ? new URL(value).pathname : null;
}

const realFetch = globalThis.fetch;

/** Nothing in this file may reach the network: a case that needs a backend answer installs its own fetch. */
const noNetwork = (async (input: RequestInfo | URL) => {
	const url = input instanceof Request ? input.url : String(input);
	throw new Error(`middleware test made an unexpected request: ${url}`);
}) as typeof fetch;

beforeEach(() => {
	globalThis.fetch = noNetwork;
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), serverApiBase: "http://127.0.0.1:9/api/v1" });
});

afterEach(() => {
	globalThis.fetch = realFetch;
	__resetForumThreadMemoForTest();
	__resetStorefrontGuardMemoForTest();
	__setPublishedLocalesForTest(null);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("an unprefixed page is zh-TW: next-intl rewrites it to /zh-TW/… without redirecting", async () => {
	for (const [path, internal] of [
		["/", "/zh-TW"],
		["/about", "/zh-TW/about"],
		["/market/products", "/zh-TW/market/products"],
		["/products/123", "/zh-TW/products/123"],
		["/login", "/zh-TW/login"],
	] as const) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 200, path);
		assert.equal(location(response), null, path);
		assert.equal(rewrittenTo(response), internal, path);
	}
});

test("a prefixed page is served as is; the locale comes from the URL", async () => {
	for (const path of ["/en", "/en/about", "/en/products/123", "/en/login"]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 200, path);
		assert.equal(location(response), null, path);
		assert.equal(rewrittenTo(response), null, path);
	}
});

test("an explicit /zh-TW prefix is redirected to the unprefixed URL, before any other decision", async () => {
	for (const [path, to] of [
		["/zh-TW", "/"],
		["/zh-TW/about", "/about"],
		["/zh-TW/account/orders", "/account/orders"],
	] as const) {
		const response = await siteMiddleware(request(path));
		assert.ok(
			[307, 308].includes(response.status),
			`${path} ${response.status}`,
		);
		assert.equal(location(response)?.pathname, to, path);
	}
});

test("a metadata image route keeps its /zh-TW prefix: Next writes it into the HTML, so it is served, not redirected", async () => {
	for (const path of [
		"/zh-TW/sellers/9/apple-icon-v673w8",
		"/zh-TW/sellers/9/apple-icon-v673w8?x=1",
	]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 200, path);
		assert.equal(location(response), null, path);
		assert.equal(rewrittenTo(response), null, path);
	}
	const page = await siteMiddleware(request("/zh-TW/sellers/9"));
	assert.ok([307, 308].includes(page.status));
	const order = await siteMiddleware(request("/zh-TW/account/orders/icon"));
	assert.ok([307, 308].includes(order.status));
	assert.equal(location(order)?.pathname, "/account/orders/icon");
});

test("the browser's language never routes: Accept-Language: en on / stays zh-TW, with no redirect, cookie or Link header", async () => {
	for (const path of ["/", "/en", "/about"]) {
		const response = await siteMiddleware(
			request(path, { "accept-language": "en-US,en;q=0.9" }),
		);
		assert.equal(location(response), null, path);
		assert.equal(
			response.headers.get("set-cookie"),
			null,
			`${path} set-cookie`,
		);
		assert.equal(response.headers.get("link"), null, `${path} link`);
	}
	const home = await siteMiddleware(request("/", { "accept-language": "en" }));
	assert.equal(rewrittenTo(home), "/zh-TW");
	const withCookie = await siteMiddleware(
		request("/", { cookie: "NEXT_LOCALE=en" }),
	);
	assert.equal(rewrittenTo(withCookie), "/zh-TW");
});

test("a guest on a protected page is sent to the login of the SAME locale, and comes back to the prefixed URL", async () => {
	for (const [path, role] of [
		["/en/account/orders", "buyer"],
		["/en/checkout", "buyer"],
		["/en/seller-center/orders", "seller"],
	] as const) {
		const to = location(await siteMiddleware(request(path)));
		assert.ok(to, path);
		assert.equal(to.pathname, "/en/login", path);
		assert.equal(to.searchParams.get("role"), role, path);
		assert.equal(to.searchParams.get("redirect"), path, path);
	}
	const zh = location(
		await siteMiddleware(request("/account/orders?tab=open")),
	);
	assert.equal(zh?.pathname, "/login");
	assert.equal(zh?.searchParams.get("redirect"), "/account/orders?tab=open");
	for (const path of [
		"/en/account",
		"/account",
		"/en/seller-center/dashboard",
	]) {
		assert.equal(
			location(await siteMiddleware(request(path, SIGNED_IN))),
			null,
			path,
		);
	}
});

test("staff and infrastructure paths never reach next-intl: no rewrite, no redirect", async () => {
	for (const path of [
		"/adm",
		"/adm/enter",
		"/adm/2685914682/products",
		"/adm/unavailable",
		"/sentinel",
		"/store-map/return",
		"/api",
		"/api/health",
		"/.well-known/apple-developer-merchantid-domain-association",
		"/apple-icon",
		"/opengraph-image",
		"/apple-app-site-association",
	]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 200, path);
		assert.equal(location(response), null, path);
		assert.equal(rewrittenTo(response), null, path);
	}
});

test("a subpath of a single non-localized path is an ordinary unknown path: next-intl's twin, which has no route", async () => {
	for (const path of [
		"/sentinel/account/orders",
		"/apple-icon/products/1",
		"/store-map/return/x",
	]) {
		const response = await siteMiddleware(request(path));
		assert.equal(rewrittenTo(response), `/zh-TW${path}`, path);
	}
});

test("an alternate spelling of a path is judged as next-intl routes it: no way around the login guard, the locale rules or the non-localized list", async () => {
	for (const [path, login] of [
		["/%61ccount/orders", "/login"],
		["//account/orders", "/login"],
		["/%65n/account/orders", "/en/login"],
		["//en/account/orders", "/en/login"],
	] as const) {
		const response = await siteMiddleware(request(path));
		const to = location(response);
		assert.ok(to, path);
		assert.equal(to.pathname, login, path);
		assert.equal(
			to.searchParams.get("redirect"),
			path.replace(/^\/{2,}/, "/"),
			path,
		);
	}
	for (const path of ["/%61pi/health", "//api/health", "/%61dm/enter"]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 200, path);
		assert.equal(rewrittenTo(response), null, path);
	}
	const broken = await siteMiddleware(request("/%E0%A4%A"));
	assert.equal(rewrittenTo(broken), null);
	assert.equal(location(broken), null);
});

test("a prefix spelled in another case is not a way around anything: next-intl moves it to the canonical case, where the guards run", async () => {
	for (const [path, to] of [
		["/EN/account/orders", "/en/account/orders"],
		["/En/about", "/en/about"],
		["/ZH-tw/account/orders", "/zh-TW/account/orders"],
	] as const) {
		const response = await siteMiddleware(request(path));
		assert.ok(
			[307, 308].includes(response.status),
			`${path} ${response.status}`,
		);
		assert.equal(location(response)?.pathname, to, path);
	}
	const nested = await siteMiddleware(request("/en/en/account"));
	assert.equal(location(nested)?.pathname, "/en/login");
});

test("a root-level path that is no file is rewritten to its /zh-TW twin, which has no route: a plain 404, never the [locale] route", async () => {
	for (const path of ["/favicon.ico", "/foo.json", "/ads.txt"]) {
		const response = await siteMiddleware(request(path));
		assert.equal(rewrittenTo(response), `/zh-TW${path}`, path);
	}
	const guarded = await siteMiddleware(request("/account/orders/HK-1.png"));
	assert.equal(location(guarded)?.pathname, "/login");
});

test("a malformed storefront id is a 404 in every locale, answered by the middleware", async () => {
	for (const path of [
		"/sellers/abc",
		"/en/sellers/abc",
		"/en/sellers/0/posts",
	]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 404, path);
		assert.equal(rewrittenTo(response), "/_not-found", path);
	}
});

const getStoreOk = () =>
	new Response(
		JSON.stringify({
			store: { id: "9", storeCode: "2600123456", storeName: "x" },
		}),
		{ status: 200, headers: { "content-type": "application/json" } },
	);

test("a store_code the backend does not hold is a real 404 in every locale, without saying why", async () => {
	for (const [status, code] of [[404, "not_found"]] as const) {
		for (const prefix of ["", "/en"]) {
			const calls: string[] = [];
			globalThis.fetch = (async (input: RequestInfo | URL) => {
				calls.push(input instanceof Request ? input.url : String(input));
				return new Response(JSON.stringify({ code, message: "x" }), {
					status,
					headers: { "content-type": "application/json" },
				});
			}) as typeof fetch;
			const path = `${prefix}/sellers/2600123456/reviews?utm_source=x`;
			const response = await siteMiddleware(request(path));
			assert.equal(response.status, 404, `${code} ${path}`);
			assert.equal(rewrittenTo(response), "/_not-found", path);
			assert.ok(
				calls.some((u) => u.includes("SellerPublicService/GetStore")),
				"asks GetStore",
			);
		}
	}
});

test("an invalid_argument answer from GetStore is unknown, not absent: the storefront passes through", async () => {
	for (const prefix of ["", "/en"]) {
		globalThis.fetch = (async () =>
			new Response(JSON.stringify({ code: "invalid_argument", message: "x" }), {
				status: 400,
				headers: { "content-type": "application/json" },
			})) as typeof fetch;
		const response = await siteMiddleware(
			request(`${prefix}/sellers/2600123456/reviews`),
		);
		assert.notEqual(response.status, 404, prefix);
	}
});

test("a present store_code storefront passes through with no redirect and keeps its tab and query", async () => {
	globalThis.fetch = (async () => getStoreOk()) as typeof fetch;
	for (const path of [
		"/sellers/2600123456/reviews?utm_source=x",
		"/en/sellers/2600123456/reviews?utm_source=x",
	]) {
		const response = await siteMiddleware(request(path));
		assert.notEqual(response.status, 308, path);
		assert.notEqual(response.status, 404, path);
	}
});

test("a store_id URL is a 404, not a redirect, and costs no backend call", async () => {
	let called = 0;
	globalThis.fetch = (async () => {
		called++;
		return getStoreOk();
	}) as typeof fetch;
	const response = await siteMiddleware(
		request("/sellers/7487768729113814280/reviews"),
	);
	assert.equal(response.status, 404);
	assert.equal(called, 0);
});

test("an outage reading the store is not a 404", async () => {
	globalThis.fetch = (async () =>
		new Response("{}", { status: 503 })) as typeof fetch;
	const response = await siteMiddleware(request("/sellers/2600123456"));
	assert.notEqual(response.status, 404);
});

test("the agent-gate cleanup keeps the locale prefix", async () => {
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), hawkrAgentEnabled: false });
	for (const [path, to] of [
		["/account/messages?requestId=abc&utm=1", "/account/messages?utm=1"],
		["/en/account/messages?requestId=abc&utm=1", "/en/account/messages?utm=1"],
	] as const) {
		const response = await siteMiddleware(request(path, SIGNED_IN));
		assert.equal(response.status, 307, path);
		const target = location(response);
		assert.equal(`${target?.pathname}${target?.search}`, to, path);
	}
});

test("a locale one surface has published and another has not is a 404 on the second (per-surface publication)", async () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	__setPublishedLocalesForTest({ buyer: ["zh-TW", "en"], seller: ["zh-TW"] });
	const buyer = await siteMiddleware(request("/en/faq"));
	assert.equal(buyer.status, 200);
	assert.equal(rewrittenTo(buyer), null);
	for (const path of [
		"/en/seller",
		"/en/seller/pricing",
		"/en/seller-center/orders",
	]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 404, path);
		assert.equal(rewrittenTo(response), "/_not-found", path);
	}
	const zh = await siteMiddleware(request("/seller/pricing"));
	assert.equal(zh.status, 200);
});

test("a non-default locale's UGC page answers X-Robots-Tag: noindex on an indexable deployment, and nothing else does", async () => {
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), seoIndexable: true });
	globalThis.fetch = (async () =>
		new Response(
			JSON.stringify({ code: 0, message: "ok", data: { storeCode: "9" } }),
			{ status: 200, headers: { "content-type": "application/json" } },
		)) as typeof fetch;
	for (const path of [
		"/en/products/1",
		"/en/sellers/9",
		"/en/feed",
		"/%65n/products/1",
		"//en/products/1",
	]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.headers.get("x-robots-tag"), "noindex", path);
	}
	for (const path of [
		"/products/1",
		"/sellers/9",
		"/feed",
		"/en/faq",
		"/en",
		"/en/market/products",
	]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.headers.get("x-robots-tag"), null, path);
	}
});

const THREAD_ID = "00000000-0000-4000-8000-000000000001";

/** Backend stand-in: GetStore answers the store, GetForumThread answers `thread`; every call is recorded. */
function backend(thread: () => Response, calls: string[]) {
	globalThis.fetch = (async (input: RequestInfo | URL) => {
		const url = input instanceof Request ? input.url : String(input);
		calls.push(url);
		if (url.includes("GetForumThread")) return thread();
		return getStoreOk();
	}) as typeof fetch;
}

const connectError = (status: number, code: string) =>
	new Response(JSON.stringify({ code, message: "x" }), {
		status,
		headers: { "content-type": "application/json" },
	});

test("a forum thread the backend does not hold is a real 404 in every locale", async () => {
	for (const code of ["not_found", "invalid_argument"]) {
		for (const prefix of ["", "/en"]) {
			__resetForumThreadMemoForTest();
			const calls: string[] = [];
			backend(
				() => connectError(code === "not_found" ? 404 : 400, code),
				calls,
			);
			const path = `${prefix}/sellers/2600123456/messages/${THREAD_ID}`;
			const response = await siteMiddleware(request(path));
			assert.equal(response.status, 404, `${code} ${path}`);
			assert.equal(rewrittenTo(response), "/_not-found", path);
			const threadCall = calls.find((u) => u.includes("GetForumThread"));
			assert.ok(threadCall, "asks the backend");
			assert.match(threadCall, /storeId%22%3A%229%22|storeId=9|"storeId":"9"/);
		}
	}
});

test("a thread URL request calls GetStore exactly once", async () => {
	const calls: string[] = [];
	backend(() => connectError(404, "not_found"), calls);
	await siteMiddleware(request(`/sellers/2600123456/messages/${THREAD_ID}`));
	assert.equal(calls.filter((u) => u.includes("GetStore")).length, 1);
	assert.equal(calls.filter((u) => u.includes("GetForumThread")).length, 1);
});

test("a non-UUID thread id is a 404 without asking the backend for the thread", async () => {
	const calls: string[] = [];
	backend(() => connectError(404, "not_found"), calls);
	const response = await siteMiddleware(
		request("/sellers/2600123456/messages/not-a-uuid"),
	);
	assert.equal(response.status, 404);
	assert.equal(rewrittenTo(response), "/_not-found");
	assert.equal(calls.filter((u) => u.includes("GetForumThread")).length, 0);
});

test("an existing forum thread passes through, and the answer is memoised", async () => {
	const calls: string[] = [];
	backend(
		() =>
			new Response(JSON.stringify({}), {
				status: 200,
				headers: { "content-type": "application/json" },
			}),
		calls,
	);
	const path = `/sellers/2600123456/messages/${THREAD_ID}`;
	for (let i = 0; i < 2; i++) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 200);
		assert.notEqual(rewrittenTo(response), "/_not-found");
	}
	assert.equal(calls.filter((u) => u.includes("GetForumThread")).length, 1);
});

test("a thread lookup that fails or times out passes through instead of 404", async () => {
	for (const thread of [
		() => connectError(503, "unavailable"),
		() => {
			throw new TypeError("fetch failed");
		},
	]) {
		__resetForumThreadMemoForTest();
		backend(thread, []);
		const response = await siteMiddleware(
			request(`/sellers/2600123456/messages/${THREAD_ID}`),
		);
		assert.equal(response.status, 200);
		assert.notEqual(rewrittenTo(response), "/_not-found");
	}
});

test("other storefront routes never ask for a thread", async () => {
	const calls: string[] = [];
	backend(() => connectError(404, "not_found"), calls);
	for (const path of [
		"/sellers/2600123456/messages",
		"/sellers/2600123456/reviews",
	]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 200, path);
	}
	assert.equal(calls.filter((u) => u.includes("GetForumThread")).length, 0);
});

const THREAD_CALLS = (calls: string[]) =>
	calls.filter((u) => u.includes("GetForumThread")).length;

test("a present thread is re-asked after 60 s, so a deleted thread 404s promptly", async () => {
	mock.timers.enable({ apis: ["Date"], now: 1_000_000 });
	try {
		const calls: string[] = [];
		let gone = false;
		backend(
			() =>
				gone
					? connectError(404, "not_found")
					: new Response(JSON.stringify({}), {
							status: 200,
							headers: { "content-type": "application/json" },
						}),
			calls,
		);
		const path = `/sellers/2600123456/messages/${THREAD_ID}`;
		assert.equal((await siteMiddleware(request(path))).status, 200);
		gone = true;
		mock.timers.tick(59_000);
		assert.equal((await siteMiddleware(request(path))).status, 200);
		assert.equal(THREAD_CALLS(calls), 1);
		mock.timers.tick(2_000);
		assert.equal((await siteMiddleware(request(path))).status, 404);
		assert.equal(THREAD_CALLS(calls), 2);
	} finally {
		mock.timers.reset();
	}
});

test("an absent thread is never memoised: every request asks again", async () => {
	const calls: string[] = [];
	backend(() => connectError(404, "not_found"), calls);
	const path = `/sellers/2600123456/messages/${THREAD_ID}`;
	for (let i = 0; i < 3; i++) {
		assert.equal((await siteMiddleware(request(path))).status, 404);
	}
	assert.equal(THREAD_CALLS(calls), 3);
});

const LETTER_ID = "0a1b2c3d-0000-4000-8000-00000000abcd";

test("an uppercase thread id is a 308 to the lowercase URL, locale prefix and query kept", async () => {
	const calls: string[] = [];
	backend(() => connectError(404, "not_found"), calls);
	for (const prefix of ["", "/en"]) {
		const response = await siteMiddleware(
			request(
				`${prefix}/sellers/2600123456/messages/${LETTER_ID.toUpperCase()}?utm_source=x`,
			),
		);
		assert.equal(response.status, 308, prefix);
		const target = location(response);
		assert.equal(
			target?.pathname,
			`${prefix}/sellers/2600123456/messages/${LETTER_ID}`,
		);
		assert.equal(target?.search, "?utm_source=x");
	}
	assert.equal(THREAD_CALLS(calls), 0);
});
