import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { NextRequest } from "next/server";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { __setPublishedLocalesForTest } from "../lib/intl/locales";
import { __resetSellerStoreCodeMemoForTest } from "../lib/seller/canonical-redirect";
import { siteMiddleware } from "../middleware";

/**
 * Locale routing as the middleware composes it (ADR 2026-10-17 D1): the
 * site's own decisions (the session cookie's presence) run on the locale-free pathname, and whatever passes is next-intl's
 * response. The test process runs the dev flavor, which routes every supported locale, so `/en`
 * is a locale here; the prod behaviour of the same code (`/en` is not a locale) is the unit
 * tests of lib/intl and the built-server smoke.
 */

// 登入與否只看 session cookie 在不在（middleware.ts；值不驗，權威是 GetSession）。
const SIGNED_IN = { cookie: "__Host-hk_session=opaque" };

function request(path: string, headers: Record<string, string> = {}) {
	// A request with no Host header is an unidentifiable client and middleware
	// de-indexes it (lib/seo/canonical-host.ts); real ones always send it.
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
	// the API base is loopback whatever the flavor's default is (tests/setup/hermetic.ts pins it too)
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), serverApiBase: "http://127.0.0.1:9/api/v1" });
});

afterEach(() => {
	globalThis.fetch = realFetch;
	__resetSellerStoreCodeMemoForTest();
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
		// not turned into a login redirect carrying a /zh-TW return path
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
	// ...while a page under the same explicit prefix is still redirected
	const page = await siteMiddleware(request("/zh-TW/sellers/9"));
	assert.ok([307, 308].includes(page.status));
	// and a page whose dynamic segment happens to be called icon is not an image route: it
	// is redirected to its canonical URL, where the login guard applies
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
	// and a stale NEXT_LOCALE cookie from an older deployment does not route either
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
	// the unprefixed (zh-TW) flow is unchanged
	const zh = location(
		await siteMiddleware(request("/account/orders?tab=open")),
	);
	assert.equal(zh?.pathname, "/login");
	assert.equal(zh?.searchParams.get("redirect"), "/account/orders?tab=open");
	// a signed-in visitor passes in either locale
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
			// the requested spelling, minus leading slashes (a `//x` return target is a URL)
			path.replace(/^\/{2,}/, "/"),
			path,
		);
	}
	// the infrastructure paths stay out of next-intl in either spelling
	for (const path of ["/%61pi/health", "//api/health", "/%61dm/enter"]) {
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 200, path);
		assert.equal(rewrittenTo(response), null, path);
	}
	// a path that cannot be decoded is left to Next, which answers 400
	const broken = await siteMiddleware(request("/%E0%A4%A"));
	assert.equal(rewrittenTo(broken), null);
	assert.equal(location(broken), null);
});

test("a prefix spelled in another case is not a way around anything: next-intl moves it to the canonical case, where the guards run", async () => {
	for (const [path, to] of [
		["/EN/account/orders", "/en/account/orders"],
		["/En/about", "/en/about"],
		// two hops: to the canonical-case prefix, then (default locale) to the unprefixed URL, then the guard
		["/ZH-tw/account/orders", "/zh-TW/account/orders"],
	] as const) {
		const response = await siteMiddleware(request(path));
		assert.ok(
			[307, 308].includes(response.status),
			`${path} ${response.status}`,
		);
		assert.equal(location(response)?.pathname, to, path);
	}
	// a locale that is not at the front is just a path segment (and, being unknown, a 404 page)
	const nested = await siteMiddleware(request("/en/en/account"));
	assert.equal(location(nested)?.pathname, "/en/login");
});

test("a root-level path that is no file is rewritten to its /zh-TW twin, which has no route: a plain 404, never the [locale] route", async () => {
	for (const path of ["/favicon.ico", "/foo.json", "/ads.txt"]) {
		const response = await siteMiddleware(request(path));
		assert.equal(rewrittenTo(response), `/zh-TW${path}`, path);
	}
	// and an asset-looking tail does not take a guarded page out of the guard
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

test("a store_id storefront URL is 308'd to its store_code and keeps the locale prefix, tab and query", async () => {
	globalThis.fetch = (async () =>
		new Response(
			JSON.stringify({
				code: 0,
				message: "ok",
				data: { storeCode: "2600123456" },
			}),
			{ status: 200, headers: { "content-type": "application/json" } },
		)) as typeof fetch;
	for (const [path, to] of [
		[
			"/sellers/1234567890123456789/reviews?utm_source=x",
			"/sellers/2600123456/reviews",
		],
		[
			"/en/sellers/1234567890123456789/reviews?utm_source=x",
			"/en/sellers/2600123456/reviews",
		],
	] as const) {
		__resetSellerStoreCodeMemoForTest();
		const response = await siteMiddleware(request(path));
		assert.equal(response.status, 308, path);
		const target = location(response);
		assert.equal(target?.pathname, to, path);
		assert.equal(target?.search, "?utm_source=x", path);
	}
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
	// The dev flavor routes every locale; the per-surface guard is exercised in the
	// prod flavor, where only the published lists route: en for buyers only.
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
	// zh-TW is unaffected on the seller surface
	const zh = await siteMiddleware(request("/seller/pricing"));
	assert.equal(zh.status, 200);
});

test("a non-default locale's UGC page answers X-Robots-Tag: noindex on an indexable deployment, and nothing else does", async () => {
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), seoIndexable: true });
	// storefront paths resolve their store_code first; answer it locally (store 9 is its own code)
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
