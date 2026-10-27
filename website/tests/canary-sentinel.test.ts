import assert from "node:assert/strict";
import { test } from "node:test";
import { buildRobots } from "../app/robots";
import { staticSitemapEntries } from "../app/sitemap";
import { isAllowedRevalidationTag } from "../lib/api/revalidate-request";
import {
	buildSentinelBody,
	buildSentinelEntry,
	CANARY_PATH,
	CANARY_TAG,
	SENTINEL_RESPONSE_HEADERS,
} from "../lib/canary/sentinel";
import { __setFlavorForTest } from "../lib/config/flavor";

test("payload 帶得出「這個 PoP 供的內容有多舊」", () => {
	const entry = buildSentinelEntry(1_000, "deploy-1");
	const body = buildSentinelBody(entry, 3_500);
	assert.equal(body.generatedAt, 1_000);
	assert.equal(body.ageMs, 2_500);
	assert.equal(body.tag, CANARY_TAG);
	// 時鐘倒退（跨機器/跨 PoP）不得產生負數 age 汙染統計。
	assert.equal(buildSentinelBody(entry, 500).ageMs, 0);
	assert.equal(buildSentinelEntry(1_000, undefined).buildId, "unknown");
});

test("sentinel 回應不可被索引、不可被任何回應層快取", () => {
	assert.match(SENTINEL_RESPONSE_HEADERS["x-robots-tag"], /noindex/);
	assert.match(SENTINEL_RESPONSE_HEADERS["cache-control"], /no-store/);
});

test("canary tag 永遠不得被 /api/revalidate 接受（否則會展開成正式 aggregate）", () => {
	assert.equal(isAllowedRevalidationTag(CANARY_TAG), false);
	assert.equal(isAllowedRevalidationTag("product:123"), true);
});

test("robots 明確 disallow sentinel，且 sitemap 不會收錄它", () => {
	__setFlavorForTest("prod");
	const rules = buildRobots(true).rules as { disallow: string[] };
	assert.ok(rules.disallow.includes(CANARY_PATH));
	// 關閉態（未上線 / 非 prod）本來就全站封鎖。
	assert.deepEqual(buildRobots(false).rules, { userAgent: "*", disallow: "/" });

	const urls = staticSitemapEntries().map((entry) => entry.url);
	assert.equal(
		urls.some((url) => url.endsWith(CANARY_PATH)),
		false,
	);
	__setFlavorForTest(null);
});
