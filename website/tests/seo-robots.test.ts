import assert from "node:assert/strict";
import { test } from "node:test";
import { buildRobots } from "../app/robots";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

// The prod flavor requires its own web origin (NEXT_PUBLIC_WEB_ORIGIN); these cases describe a deployment at this one.
process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

function reset() {
	__setEnvForTest(null);
	__setFlavorForTest(null);
}

interface HeaderRule {
	source: string;
	headers: Array<{ key: string; value: string }>;
}

/**
 * Read the `X-Robots-Tag` a deployment with `seoIndexable = <indexable>` would
 * send, for BOTH values of the gate, inside one process.
 *
 * `next.config.ts` decides that header ONCE, in its module body — so the
 * ambient environment of the test run used to pick which single direction was
 * observable, and the un-run direction was the dangerous one: a `noindex` on an
 * indexable production deployment silently removes the whole site from Google.
 * `__setEnvForTest` alone cannot reach an already-evaluated module body, so each
 * direction gets its own module instance via a distinct import specifier (the
 * `?seoIndexable=` suffix defeats the ESM module cache) with the env injected
 * BEFORE that instance is evaluated. Both directions therefore run in every CI
 * run, on default env, with no separate prod-env job.
 */
async function xRobotsTagFor(indexable: boolean) {
	// buildEnvConfig reads the active flavor, and only prod can ever be
	// indexable — pin it so the injected env is a shape that can really occur.
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), seoIndexable: indexable });
	try {
		const config = await import(`../next.config.ts?seoIndexable=${indexable}`);
		const rules = (await config.default.headers?.()) as HeaderRule[];
		// The de-index header rides on the catch-all security rule. Anchor on that
		// rule itself rather than on its position: `/adm/enter` now appends a
		// narrower Referrer-Policy override after it (a later match wins per
		// header key), so "last rule" stopped being the same thing as "the
		// catch-all security rule". Still discriminating — if the catch-all rule
		// disappears, `security` is undefined and every assertion below fails.
		const security = rules.findLast(
			(rule) =>
				rule.source === "/(.*)" &&
				rule.headers.some(
					(header) => header.key.toLowerCase() === "referrer-policy",
				),
		);
		assert.equal(security?.source, "/(.*)");
		return security?.headers.find(
			(header) => header.key.toLowerCase() === "x-robots-tag",
		);
	} finally {
		reset();
	}
}

test("gate closed blocks all crawling (non-prod, or un-launched prod)", () => {
	const r = buildRobots(false);
	assert.deepEqual(r.rules, { userAgent: "*", disallow: "/" });
	assert.equal(r.sitemap, undefined);
});

test("gate open allows all but /api and points at the sitemap", () => {
	// siteUrl() reads the active flavor, so pin it to prod for the assertion.
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const r = buildRobots(true);
	assert.deepEqual(r.rules, {
		userAgent: "*",
		allow: "/",
		// /sentinel = 快取新鮮度 canary 探測點（lib/canary/sentinel.ts），無讀者價值
		// /adm = ADM 代行工作區（56 號），短效工作階段背後的他人商店資料
		disallow: ["/api/", "/adm", "/sentinel"],
	});
	assert.equal(r.sitemap, "https://example.com/sitemap.xml");
	assert.equal(r.host, "https://example.com");
	reset();
});

test("a closed gate also de-indexes over HTTP, where robots.txt can be overridden", async () => {
	// robots.txt is not enough on its own: a body is merged per user-agent group
	// (RFC 9309), and Cloudflare's zone-level Managed robots.txt prepends its own
	// `User-Agent: *` group whose `Allow: /` cancels ours — measured on web-dev,
	// which serves allow-all while this gate is closed. The header is outside
	// that merge. Both must be driven by env.seoIndexable and nothing else.
	//
	// Reach, measured (hawkr-website#367 item 4): this covers every response
	// Next produces, NOT literally every response. `/_next/static/*` and files
	// shipped in `public/` are answered by Cloudflare's asset layer before the
	// Worker runs, so no `headers()` rule can decorate them — next.config.ts's
	// "also covers non-HTML responses" is true of Next's own non-HTML responses
	// only. Harmless (a JS chunk or an image is not indexable content, and every
	// HTML entry point that links them is covered), but not a guarantee.
	const noindex = await xRobotsTagFor(false);
	assert.match(noindex?.value ?? "", /noindex/);
	assert.match(noindex?.value ?? "", /nofollow/);
});

test("an indexable deployment does not de-index itself", async () => {
	// The direction that costs the most when it breaks: shipping this header on
	// launched production drops every page out of Google. It used to be
	// unreachable in a default CI run (the header is decided at module load, so
	// only the ambient env's side ran) — the launch itself was the first
	// execution. Now it runs on every push.
	assert.equal(await xRobotsTagFor(true), undefined);
});
