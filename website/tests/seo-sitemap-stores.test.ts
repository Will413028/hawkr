import assert from "node:assert/strict";
import { test } from "node:test";
import {
	buildSitemap,
	loadSitemapStores,
	reportSitemapSize,
	SITEMAP_SHARD_THRESHOLD,
	staticSitemapEntries,
	storeSitemapEntries,
} from "../app/sitemap";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import {
	collectAllStores,
	fetchAllStoresWith,
	type StoresPage,
	storeSitemapRows,
} from "../lib/seller/list-stores";

process.env.NEXT_PUBLIC_API_BASE_URL = "/api/v1";
process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

const store = (code: string, seconds?: number) => ({
	storeCode: code,
	...(seconds !== undefined
		? { updateTime: { seconds: BigInt(seconds), nanos: 500_000_000 } }
		: {}),
});

test("rows carry store_code and update_time in ms; codeless stores are dropped", () => {
	const rows = storeSitemapRows({
		stores: [store("12", 10), store(""), store("13")],
		nextPageToken: "",
	});
	assert.deepEqual(rows, [
		{ storeCode: "12", updatedAtMs: 10_500 },
		{ storeCode: "13" },
	]);
});

test("pagination follows tokens past a short non-final page and an empty page", async () => {
	const pages: Record<string, StoresPage> = {
		"": { stores: [store("1"), store("2")], nextPageToken: "a" },
		a: { stores: [store("3")], nextPageToken: "b" },
		b: { stores: [], nextPageToken: "c" },
		c: { stores: [store("4")], nextPageToken: "" },
	};
	const seen: string[] = [];
	const rows = await collectAllStores(async (t) => {
		seen.push(t);
		return pages[t];
	});
	assert.deepEqual(seen, ["", "a", "b", "c"]);
	assert.deepEqual(
		rows.map((r) => r.storeCode),
		["1", "2", "3", "4"],
	);
});

test("no stores gives an empty list; a failing page rejects", async () => {
	assert.deepEqual(
		await collectAllStores(async () => ({ stores: [], nextPageToken: "" })),
		[],
	);
	await assert.rejects(
		collectAllStores(async (t) => {
			if (t) throw new Error("boom");
			return { stores: [store("1")], nextPageToken: "x" };
		}),
		/boom/,
	);
});

test("store entries: default locale only for UGC, lastModified from update_time", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	const entries = storeSitemapEntries(
		[{ storeCode: "7", updatedAtMs: 1000 }, { storeCode: "8" }],
		() => ["zh-TW", "en"],
	);
	assert.equal(entries.length, 2);
	assert.equal(entries[0].url, "https://example.com/sellers/7");
	assert.equal(entries[0].lastModified?.valueOf(), 1000);
	assert.equal("lastModified" in entries[1], false);
	assert.equal("alternates" in entries[0], false);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("over the threshold every entry is still emitted and an error is reported", () => {
	const reports: string[] = [];
	reportSitemapSize(SITEMAP_SHARD_THRESHOLD + 1, (m) => reports.push(m));
	assert.equal(reports.length, 1);
	reportSitemapSize(SITEMAP_SHARD_THRESHOLD, (m) => reports.push(m));
	assert.equal(reports.length, 1);
});

test("the threshold counts the whole file: static plus store entries", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	const staticCount = staticSitemapEntries().length;
	const storeCount = SITEMAP_SHARD_THRESHOLD - staticCount + 1;
	assert.ok(storeCount <= SITEMAP_SHARD_THRESHOLD);
	const rows = Array.from({ length: storeCount }, (_, i) => ({
		storeCode: String(i + 1),
	}));
	const reports: string[] = [];
	const entries = buildSitemap(rows, (m) => reports.push(m));
	assert.equal(entries.length, SITEMAP_SHARD_THRESHOLD + 1);
	assert.equal(reports.length, 1);
	const quiet: string[] = [];
	buildSitemap(rows.slice(1), (m) => quiet.push(m));
	assert.equal(quiet.length, 0);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("the threshold counts emitted URL entries, not stores", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	const stores = [
		{ storeCode: " " },
		...Array.from({ length: SITEMAP_SHARD_THRESHOLD }, (_, i) => ({
			storeCode: String(i + 1),
		})),
	];
	const entries = storeSitemapEntries(stores, () => ["zh-TW"]);
	assert.equal(stores.length, SITEMAP_SHARD_THRESHOLD + 1);
	assert.equal(entries.length, SITEMAP_SHARD_THRESHOLD);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});

test("a repeated next_page_token throws instead of looping", async () => {
	let calls = 0;
	await assert.rejects(
		collectAllStores(async (t) => {
			if (++calls > 20) throw new Error("runaway pagination");
			return {
				stores: [store(`s${t}`)],
				nextPageToken: t === "" ? "a" : t === "a" ? "b" : "a",
			};
		}),
		/repeated next_page_token/,
	);
});

test("each ListStores page gets its own signal", async () => {
	const signals: (AbortSignal | undefined)[] = [];
	const pages: Record<string, StoresPage> = {
		"": { stores: [store("1")], nextPageToken: "a" },
		a: { stores: [store("2")], nextPageToken: "" },
	};
	let made = 0;
	const rows = await fetchAllStoresWith(
		{
			listStores: async (req, o) => {
				signals.push(o.signal);
				return pages[req.pageToken];
			},
		},
		() => {
			made++;
			return new AbortController().signal;
		},
	);
	assert.equal(rows.length, 2);
	assert.equal(made, 2);
	assert.notEqual(signals[0], signals[1]);
});

test("runtime store failure still yields static entries, reported once", async () => {
	const reports: unknown[] = [];
	const prev = process.env.NEXT_PHASE;
	delete process.env.NEXT_PHASE;
	try {
		const stores = await loadSitemapStores(
			async () => {
				throw new Error("upstream down");
			},
			(e) => reports.push(e),
		);
		assert.deepEqual(stores, []);
		assert.equal(reports.length, 1);
		assert.ok(staticSitemapEntries(() => ["zh-TW"]).length > 0);
		process.env.NEXT_PHASE = "phase-production-build";
		await loadSitemapStores(
			async () => {
				throw new Error("x");
			},
			(e) => reports.push(e),
		);
		assert.equal(reports.length, 1);
	} finally {
		if (prev === undefined) delete process.env.NEXT_PHASE;
		else process.env.NEXT_PHASE = prev;
	}
});

test("store path comes from sellerPath; blank codes are skipped", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	const entries = storeSitemapEntries(
		[{ storeCode: "  " }, { storeCode: "a b" }],
		() => ["zh-TW"],
	);
	assert.equal(entries.length, 1);
	assert.equal(entries[0].url, "https://example.com/sellers/a%20b");
	__setEnvForTest(null);
	__setFlavorForTest(null);
});
