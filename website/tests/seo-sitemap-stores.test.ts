import assert from "node:assert/strict";
import { test } from "node:test";
import { SITEMAP_SHARD_THRESHOLD, storeSitemapEntries } from "../app/sitemap";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import {
	collectAllStores,
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

test("over the threshold every store is still emitted and an error is reported", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	const stores = Array.from(
		{ length: SITEMAP_SHARD_THRESHOLD + 1 },
		(_, i) => ({
			storeCode: String(i + 1),
		}),
	);
	const reports: string[] = [];
	const entries = storeSitemapEntries(
		stores,
		() => ["zh-TW"],
		(m) => reports.push(m),
	);
	assert.equal(entries.length, stores.length);
	assert.equal(reports.length, 1);
	const none: string[] = [];
	storeSitemapEntries(
		stores.slice(0, 10),
		() => ["zh-TW"],
		(m) => none.push(m),
	);
	assert.equal(none.length, 0);
	__setEnvForTest(null);
	__setFlavorForTest(null);
});
