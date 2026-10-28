import assert from "node:assert/strict";
import { test } from "node:test";
import { NextRequest } from "next/server";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { isCanonicalHost } from "../lib/seo/canonical-host";
import { compiledMatcher } from "./helpers/middleware-matcher";

/** Clerk 沒有 session 的請求（middleware 的 Clerk 包裝交給 Clerk，這裡只測網站自己的判斷）。 */
const signedOut = async () => ({ userId: null });

const PROD_ORIGIN = "https://example.com";
const PREVIEW_HOST = "hawkr-website-prod.example.workers.dev";

/**
 * Put the process into the ONE configuration this feature exists for: a
 * launched production build (`seoIndexable = true`), where `next.config.ts`'s
 * blanket `X-Robots-Tag` is absent and middleware is the only thing that can
 * tell the preview host from the real one.
 */
function applyLaunchedProdEnv() {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), seoIndexable: true, webOrigin: PROD_ORIGIN });
}

function reset() {
	__setEnvForTest(null);
	__setFlavorForTest(null);
}

test("canonical hosts are the deployment's own origin, its www sibling, and loopback", () => {
	for (const host of [
		"example.com",
		"Example.COM",
		"www.example.com",
		"example.com:443",
		"localhost:3000",
		"127.0.0.1:3000",
		"[::1]:3000",
	]) {
		assert.equal(
			isCanonicalHost(host, PROD_ORIGIN),
			true,
			`${host} must stay indexable`,
		);
	}
});

test("preview and look-alike hosts are not canonical", () => {
	for (const host of [
		PREVIEW_HOST,
		"hawkr-website-dev.example.workers.dev",
		// Suffix/prefix look-alikes: a substring match would wrongly admit these.
		"evil-example.com",
		"example.com.attacker.example",
		"web-dev.example.com",
		"",
	]) {
		assert.equal(
			isCanonicalHost(host, PROD_ORIGIN),
			false,
			`${host || "<empty>"} must not be treated as canonical`,
		);
	}
	// A request with no Host header at all is an unidentifiable client, not a
	// licence to index.
	assert.equal(isCanonicalHost(null, PROD_ORIGIN), false);
});

/**
 * The header is what search engines actually obey, so assert on a real
 * middleware invocation rather than on the predicate alone — and assert BOTH
 * directions. The negative case is the load-bearing one: a bug that de-indexed
 * `example.com` would remove the whole site from Google, which is far worse
 * than the leak this change closes.
 */
async function xRobotsTagFor(host: string): Promise<string | null> {
	applyLaunchedProdEnv();
	try {
		const { siteMiddleware } = await import("../middleware");
		const response = await siteMiddleware(
			new NextRequest(`https://${host}/`, { headers: { host } }),
			signedOut,
		);
		return response.headers.get("x-robots-tag");
	} finally {
		reset();
	}
}

test("a launched prod build de-indexes the preview host", async () => {
	assert.equal(await xRobotsTagFor(PREVIEW_HOST), "noindex, nofollow");
});

test("a launched prod build leaves the canonical host indexable", async () => {
	assert.equal(await xRobotsTagFor("example.com"), null);
	assert.equal(await xRobotsTagFor("www.example.com"), null);
});

/**
 * The matcher is half the fix: the header can only be added to requests
 * middleware actually sees. Before #355 it saw six prefixes, so the home page —
 * the page most likely to be indexed — was never reachable by this code.
 */
function matcherRegex(): RegExp {
	return compiledMatcher();
}

test("middleware covers every page route, including the ones it never saw before", () => {
	const regex = matcherRegex();
	for (const pathname of [
		"/",
		"/search",
		"/about",
		"/products/123",
		"/login",
		"/sellers/abc",
		"/account/orders",
		"/checkout/confirm",
		"/seller-center/dashboard",
		"/seller-product-import-preview/1",
	]) {
		assert.ok(regex.test(pathname), `${pathname} 應納入 middleware`);
	}
});

test("middleware stays off the asset layer and the API proxy", () => {
	const regex = matcherRegex();
	for (const pathname of [
		"/api/v1/public/products",
		"/api/csp-report",
		"/adm/api/orders/1",
		"/_next/static/chunks/main.js",
		"/_next/image",
		"/_next/data/x/y.json",
		"/brand/logo-mark.svg",
		"/placeholders/a/b.png",
		"/templates/jobs.csv",
		"/robots.txt",
		"/sitemap.xml",
		"/icon.svg",
		"/manifest.webmanifest",
	]) {
		assert.equal(regex.test(pathname), false, `${pathname} 不應進 middleware`);
	}
});

test("a path that only looks like an asset is judged like any other unknown path", () => {
	// Nothing serves these, so left to the router they would reach the dynamic [locale] route:
	// a 404 that ISR stores per URL, and for a guarded [param] route no login guard at all.
	const regex = matcherRegex();
	for (const pathname of [
		"/favicon.ico",
		"/foo.json",
		"/ads.txt",
		"/x.png",
		"/zzz/products/1.png",
		"/zzz/seller-guide/icon-1leq8f.svg",
		"/-/products/1",
		"/-/seller-guide/other.svg",
		"/account/orders/HK-1.png",
		"/en/account/orders/x.svg",
		"/seller-center/orders/x.json",
	]) {
		assert.ok(regex.test(pathname), `${pathname} 應納入 middleware`);
	}
});

/**
 * The matcher went from six literal prefixes to a site-wide pattern, so the
 * redirects that lived behind those prefixes have to be shown still working —
 * and shown working THROUGH the new wrapper, since every response now passes
 * through it on its way out.
 *
 * The storefront 308 is the one that cannot be exercised against dev today:
 * every seller there has `store_code == store_id`, so the backend never asks
 * for a redirect. Stubbing the lookup at the network boundary is what makes the
 * branch reachable at all.
 */
async function middlewareFor(
	pathname: string,
	host: string,
	storeCode: string | null,
) {
	applyLaunchedProdEnv();
	const realFetch = globalThis.fetch;
	globalThis.fetch = (async (input: RequestInfo | URL) => {
		const url = String(input instanceof Request ? input.url : input);
		if (!url.includes("/sellers/")) return realFetch(input as RequestInfo);
		return new Response(
			JSON.stringify({
				code: 0,
				message: "success",
				data: { storeCode },
			}),
			{ status: 200, headers: { "content-type": "application/json" } },
		);
	}) as typeof fetch;
	try {
		const { siteMiddleware } = await import("../middleware");
		const { __resetSellerStoreCodeMemoForTest } = await import(
			"../lib/seller/canonical-redirect"
		);
		__resetSellerStoreCodeMemoForTest();
		return await siteMiddleware(
			new NextRequest(`https://${host}${pathname}`, { headers: { host } }),
			signedOut,
		);
	} finally {
		globalThis.fetch = realFetch;
		reset();
	}
}

test("the storefront canonical 308 still fires, and carries the host verdict", async () => {
	const response = await middlewareFor(
		"/sellers/2685914682/reviews",
		PREVIEW_HOST,
		"2600000001",
	);
	assert.equal(response.status, 308);
	assert.equal(
		new URL(response.headers.get("location") as string).pathname,
		"/sellers/2600000001/reviews",
	);
	assert.equal(response.headers.get("x-robots-tag"), "noindex, nofollow");
});

test("the login bounce still fires for a guarded path on the canonical host", async () => {
	applyLaunchedProdEnv();
	try {
		const { siteMiddleware } = await import("../middleware");
		const host = new URL(PROD_ORIGIN).host;
		const response = await siteMiddleware(
			new NextRequest(`https://${host}/account/orders`, { headers: { host } }),
			signedOut,
		);
		assert.equal(response.status, 307);
		const location = new URL(response.headers.get("location") as string);
		assert.equal(location.pathname, "/login");
		assert.equal(location.searchParams.get("role"), "buyer");
		assert.equal(location.searchParams.get("redirect"), "/account/orders");
		// Canonical host — the redirect must NOT be de-indexed.
		assert.equal(response.headers.get("x-robots-tag"), null);
	} finally {
		reset();
	}
});

test("a Clerk session passes a guarded path through", async () => {
	applyLaunchedProdEnv();
	try {
		const { siteMiddleware } = await import("../middleware");
		const host = new URL(PROD_ORIGIN).host;
		const response = await siteMiddleware(
			new NextRequest(`https://${host}/account/orders`, { headers: { host } }),
			async () => ({ userId: "user_1" }),
		);
		assert.equal(response.headers.get("location"), null);
		assert.equal(response.status, 200);
	} finally {
		reset();
	}
});

test("hidden Agent restore ids are removed before the login bounce", async () => {
	applyLaunchedProdEnv();
	try {
		const { siteMiddleware } = await import("../middleware");
		const host = new URL(PROD_ORIGIN).host;
		const response = await siteMiddleware(
			new NextRequest(
				`https://${host}/account/messages?requestId=req-secret-1&utm_source=internal&requestId=req-secret-2`,
				{ headers: { host } },
			),
			signedOut,
		);

		assert.equal(response.status, 307);
		const cleanLocation = new URL(response.headers.get("location") as string);
		assert.equal(cleanLocation.pathname, "/account/messages");
		assert.equal(cleanLocation.searchParams.get("requestId"), null);
		assert.equal(cleanLocation.searchParams.get("utm_source"), "internal");
		assert.doesNotMatch(cleanLocation.href, /req-secret|requestId/);

		const loginResponse = await siteMiddleware(
			new NextRequest(cleanLocation, { headers: { host } }),
			signedOut,
		);
		assert.equal(loginResponse.status, 307);
		const loginLocation = new URL(
			loginResponse.headers.get("location") as string,
		);
		assert.equal(loginLocation.pathname, "/login");
		assert.equal(
			loginLocation.searchParams.get("redirect"),
			"/account/messages?utm_source=internal",
		);
		assert.doesNotMatch(loginLocation.href, /req-secret|requestId/);
	} finally {
		reset();
	}
});

test("the retired custom dev host is an ordinary host, not loopback", () => {
	assert.equal(isCanonicalHost("web.localhost", PROD_ORIGIN), false);
	assert.equal(isCanonicalHost("web.localhost:3000", PROD_ORIGIN), false);
});
