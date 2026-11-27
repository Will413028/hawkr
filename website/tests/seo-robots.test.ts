import assert from "node:assert/strict";
import { test } from "node:test";
import { buildRobots } from "../app/robots";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

process.env.NEXT_PUBLIC_API_BASE_URL = "/api/v1";

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
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	__setEnvForTest({ ...getEnv(), seoIndexable: indexable });
	try {
		const config = await import(`../next.config.ts?seoIndexable=${indexable}`);
		const rules = (await config.default.headers?.()) as HeaderRule[];
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
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const r = buildRobots(true);
	assert.deepEqual(r.rules, {
		userAgent: "*",
		allow: "/",
		disallow: ["/api/", "/adm", "/sentinel"],
	});
	assert.equal(r.sitemap, "https://example.com/sitemap.xml");
	assert.equal(r.host, "https://example.com");
	reset();
});

test("a closed gate also de-indexes over HTTP, where robots.txt can be overridden", async () => {
	const noindex = await xRobotsTagFor(false);
	assert.match(noindex?.value ?? "", /noindex/);
	assert.match(noindex?.value ?? "", /nofollow/);
});

test("an indexable deployment does not de-index itself", async () => {
	assert.equal(await xRobotsTagFor(true), undefined);
});
