import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const openNextConfigPath = new URL("../open-next.config.ts", import.meta.url);
const wranglerConfigPath = new URL("../wrangler.jsonc", import.meta.url);
const staticHeadersPath = new URL("../public/_headers", import.meta.url);

async function readConfigs() {
	const [openNext, wrangler, staticHeaders] = await Promise.all([
		readFile(openNextConfigPath, "utf8"),
		readFile(wranglerConfigPath, "utf8"),
		readFile(staticHeadersPath, "utf8"),
	]);
	return { openNext, wrangler, staticHeaders };
}

/** Return the complete options argument passed to doShardedTagCache(...). */
function shardedTagCacheOptions(openNext: string): string {
	const call = "doShardedTagCache(";
	const start = openNext.indexOf(call);
	assert.notEqual(start, -1, "open-next.config.ts must call doShardedTagCache");

	let depth = 0;
	for (let i = start + call.length - 1; i < openNext.length; i++) {
		const char = openNext[i];
		if (char === "(") depth++;
		else if (char === ")") {
			depth--;
			if (depth === 0) return openNext.slice(start + call.length, i);
		}
	}
	throw new Error("unbalanced parentheses in the doShardedTagCache(...) call");
}

test("Workers Cache is disabled", async () => {
	const { wrangler } = await readConfigs();

	assert.doesNotMatch(
		wrangler,
		/"cache"\s*:/,
		"Workers Cache must not participate in release correctness",
	);
});

test("OpenNext application cache bindings are declared", async () => {
	const { openNext, wrangler } = await readConfigs();

	assert.match(
		openNext,
		/incrementalCache:\s*withRegionalCache\(r2IncrementalCache,/,
	);
	assert.equal(
		wrangler.match(/"binding": "NEXT_INC_CACHE_R2_BUCKET"/g)?.length,
		1,
		"the config must bind the incremental-cache R2 bucket",
	);
	assert.equal(
		wrangler.match(/"name": "NEXT_TAG_CACHE_DO_SHARDED"/g)?.length,
		1,
		"the config must bind the sharded tag cache",
	);
	assert.equal(
		wrangler.match(/"binding": "WORKER_SELF_REFERENCE"/g)?.length,
		1,
		"the config must retain the self-reference service",
	);
});

test("time-based revalidation uses the cached Durable Object queue", async () => {
	const { openNext, wrangler } = await readConfigs();

	assert.match(
		openNext,
		/import doQueue from "@opennextjs\/cloudflare\/overrides\/queue\/do-queue"/,
	);
	assert.match(
		openNext,
		/import queueCache from "@opennextjs\/cloudflare\/overrides\/queue\/queue-cache"/,
	);
	assert.match(openNext, /\bqueue:\s*queueCache\(\s*doQueue,/);
	assert.match(openNext, /\bregionalCacheTtlSec:\s*60\b/);
	assert.match(openNext, /\bwaitForQueueAck:\s*true\b/);
	assert.equal(
		wrangler.match(/"name": "NEXT_CACHE_DO_QUEUE"/g)?.length,
		1,
		"the config must bind NEXT_CACHE_DO_QUEUE",
	);
	assert.match(
		wrangler,
		/\{\s*"tag": "v1",\s*"new_sqlite_classes": \["DOShardedTagCache"\]\s*\},\s*\{\s*"tag": "v2",\s*"new_sqlite_classes": \["DOQueueHandler"\]\s*\},\s*\{\s*"tag": "v3",\s*"new_sqlite_classes": \["BucketCachePurge"\]\s*\}/,
	);
});

test("regional cache keeps edge hits fast without an unbounded lazy R2 read", async () => {
	const { openNext } = await readConfigs();

	assert.match(
		openNext,
		/incrementalCache:\s*withRegionalCache\(r2IncrementalCache,\s*\{\s*mode:\s*"short-lived",\s*shouldLazilyUpdateOnCacheHit:\s*false,\s*bypassTagCacheOnCacheHit:\s*false,\s*\}\)/,
	);
});

test("cache interception serves ISR hits without the NextServer, which only pure ISR allows", async () => {
	const { openNext } = await readConfigs();

	// 攔截與 PPR 互斥：#338 定案全站純 ISR、cacheComponents 關閉，才允許開。
	assert.match(openNext, /\benableCacheInterception:\s*true\b/);
	const nextConfig = await readFile(
		new URL("../next.config.ts", import.meta.url),
		"utf8",
	);
	assert.doesNotMatch(
		nextConfig,
		/^\s*cacheComponents:\s*true/m,
		"cacheComponents must stay off while cache interception is enabled",
	);
});

test("cache-tag purge is buffered, deduplicated and 429-retryable through a Durable Object", async () => {
	const { openNext, wrangler } = await readConfigs();

	assert.match(
		openNext,
		/cachePurge:\s*purgeCache\(\{\s*type:\s*"durableObject"\s*\}\)/,
	);
	assert.doesNotMatch(
		openNext,
		/cachePurge:\s*purgeCache\(\{\s*type:\s*"direct"\s*\}\)/,
	);
	assert.equal(
		wrangler.match(/"name": "NEXT_CACHE_DO_PURGE"/g)?.length,
		1,
		"the config must bind NEXT_CACHE_DO_PURGE",
	);
	assert.equal(
		wrangler.match(/"NEXT_CACHE_DO_PURGE_BUFFER_TIME_IN_SECONDS": "5"/g)
			?.length,
		1,
		"the config must use the bounded purge buffer",
	);
});

test("tag-cache filters unused implicit path tags", async () => {
	const { openNext } = await readConfigs();

	assert.match(
		openNext,
		/import\s*\{\s*softTagFilter,\s*withFilter,\s*\}\s*from "@opennextjs\/cloudflare\/overrides\/tag-cache\/tag-cache-filter"/,
	);
	assert.match(
		openNext,
		/tagCache:\s*withFilter\(\{\s*tagCache:\s*doShardedTagCache\(/,
	);
	assert.match(openNext, /\}\),\s*filterFn:\s*softTagFilter,\s*\}\)/);

	const options = shardedTagCacheOptions(openNext);
	assert.match(options, /baseShardSize:\s*4/);
	assert.match(options, /regionalCache:\s*true/);
});

test("sharded tag cache replication is either absent or genuinely regional", async () => {
	const { openNext } = await readConfigs();
	const options = shardedTagCacheOptions(openNext);

	const declaresReplication = /\bshardReplication\s*:/.test(options);
	const declaresRegional = /\bregionalReplication\s*:/.test(options);
	assert.equal(
		declaresRegional && !declaresReplication,
		false,
		"regionalReplication is only read from inside shardReplication; on its own it is dead config",
	);

	if (!declaresReplication) return;

	assert.match(
		options,
		/shardReplication\s*:\s*\{[^}]*regionalReplication\s*:\s*\{/,
		"shardReplication without regionalReplication leaves every Durable Object name unchanged",
	);
	assert.match(
		options,
		/regionalReplication\s*:\s*\{[^}]*defaultRegion\s*:\s*"apac"/,
		'defaultRegion falls back to "enam" when omitted',
	);
});

test("fingerprinted Next assets are immutable in the browser cache", async () => {
	const { staticHeaders } = await readConfigs();

	assert.match(staticHeaders, /\/_next\/static\/\*/);
	assert.match(
		staticHeaders,
		/Cache-Control: public, max-age=31536000, immutable/,
	);
});
