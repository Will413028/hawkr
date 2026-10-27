import assert from "node:assert/strict";
import test from "node:test";
import { NextRequest } from "next/server";
import createIntlMiddleware from "next-intl/middleware";
import { routing } from "../i18n/routing";
import { routingPathname } from "../lib/intl/paths";

/**
 * `routingPathname` (lib/intl/paths.ts) reproduces how next-intl normalises a path before it routes
 * (`decodeURI`, then its private `sanitizePathname`: backslash, tab/newline, repeated slashes), so the
 * site's own checks (login guard, locale rules, noindex) judge the same string next-intl rewrites.
 * That copy can drift when next-intl is upgraded. This runs next-intl's own middleware on the same
 * inputs and requires its rewrite target to be the path `routingPathname` computes.
 */
const intl = createIntlMiddleware(routing);

/** Where next-intl sends the request: the rewrite target, or the request's own path when it leaves it as is. */
function rewriteTarget(path: string): string {
	const request = new NextRequest(`http://localhost:3000${path}`, {
		headers: { host: "localhost:3000" },
	});
	const rewrite = intl(request).headers.get("x-middleware-rewrite");
	return new URL(rewrite ?? request.url).pathname;
}

/** What next-intl should route the request to, from `routingPathname`: the default locale is rewritten to /zh-TW. */
function expectedTarget(path: string): string {
	const routed = routingPathname(path);
	assert.ok(routed, `routingPathname rejected ${path}`);
	if (/^\/(?:en)(?:\/|$)/.test(routed)) return routed;
	return routed === "/" ? "/zh-TW" : `/zh-TW${routed}`;
}

test("routingPathname is the path next-intl rewrites: unprefixed (default locale) and prefixed requests", () => {
	for (const path of [
		"/about",
		"/%61bout",
		"/products/%31%32%33",
		"//about",
		"///market///products",
		"/a%2Fb",
		"/a%252Fb",
		"/a%5Cb",
		"/%E7%B4%84",
		"/ja/%E7%B4%84",
		"/en/about",
		"/%65n/about",
		"//en//about",
		"/en/products/1",
		"/account/orders/%44F-1",
	]) {
		// compared in URL form: the rewrite is a URL, `routingPathname` is a decoded path
		const expected = new URL(expectedTarget(path), "http://localhost").pathname;
		assert.equal(rewriteTarget(path), expected, path);
	}
});
