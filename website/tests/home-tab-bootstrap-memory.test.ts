import assert from "node:assert/strict";
import test from "node:test";
import {
	fetchHomeTabBootstrap,
	HOME_TAB_BOOTSTRAP_FRESH_MS,
	HOME_TAB_BOOTSTRAP_TTL_MS,
	preloadHomeTabBootstrap,
} from "../lib/api/tab-bootstrap";

const PAYLOAD = {
	schemaVersion: "home.v2",
	hotProducts: [],
	products: { data: { records: [] } },
	recommendedSellers: [],
	posts: { page: 1, pageSize: 8, total: 0, records: [] },
} as const;

function jsonResponse() {
	return new Response(JSON.stringify(PAYLOAD), {
		headers: { "content-type": "application/json" },
	});
}

test("home bootstrap shares inflight work, respects TTL and isolates caller abort", async (t) => {
	const realFetch = globalThis.fetch;
	const realNow = Date.now;
	t.after(() => {
		globalThis.fetch = realFetch;
		Date.now = realNow;
	});

	let now = 10_000;
	Date.now = () => now;
	let fetchCalls = 0;
	const requestInits: RequestInit[] = [];
	globalThis.fetch = (async (_input, init = {}) => {
		fetchCalls += 1;
		requestInits.push(init);
		return jsonResponse();
	}) as typeof fetch;

	const first = await fetchHomeTabBootstrap();
	const second = await preloadHomeTabBootstrap();
	assert.strictEqual(second, first);
	assert.equal(fetchCalls, 1, "warm memory entry must not refetch");
	assert.equal(requestInits[0]?.cache, "no-store");
	assert.equal(requestInits[0]?.credentials, "omit");
	assert.equal(requestInits[0]?.signal, undefined);

	now += HOME_TAB_BOOTSTRAP_FRESH_MS + 1;
	const staleWhileRefreshing = await preloadHomeTabBootstrap();
	assert.strictEqual(staleWhileRefreshing, first);
	assert.equal(
		fetchCalls,
		2,
		"a stale intent preload must start one background refresh",
	);
	await new Promise((resolve) => setImmediate(resolve));
	const refreshed = await fetchHomeTabBootstrap();
	assert.notStrictEqual(refreshed, first);
	assert.equal(
		fetchCalls,
		2,
		"the active query must join/reuse the completed background refresh",
	);

	now += HOME_TAB_BOOTSTRAP_TTL_MS + 1;
	const alreadyAborted = new AbortController();
	alreadyAborted.abort();
	await assert.rejects(
		fetchHomeTabBootstrap(alreadyAborted.signal),
		(error: unknown) =>
			error instanceof DOMException && error.name === "AbortError",
	);
	assert.equal(fetchCalls, 2, "an already-aborted caller must not start work");

	await fetchHomeTabBootstrap();
	assert.equal(fetchCalls, 3, "expired memory entry must refresh");

	now += HOME_TAB_BOOTSTRAP_TTL_MS + 1;
	let resolveShared: ((response: Response) => void) | undefined;
	globalThis.fetch = ((_input, init = {}) => {
		fetchCalls += 1;
		requestInits.push(init);
		return new Promise<Response>((resolve) => {
			resolveShared = resolve;
		});
	}) as typeof fetch;

	const controller = new AbortController();
	const abortedCaller = fetchHomeTabBootstrap(controller.signal);
	const survivingPreload = preloadHomeTabBootstrap();
	controller.abort();

	await assert.rejects(
		abortedCaller,
		(error: unknown) =>
			error instanceof DOMException && error.name === "AbortError",
	);
	assert.equal(fetchCalls, 4, "concurrent callers must share one request");
	assert.equal(
		requestInits[3]?.signal,
		undefined,
		"caller abort must not cancel shared transport",
	);

	resolveShared?.(jsonResponse());
	const warmed = await survivingPreload;
	assert.equal(warmed.schemaVersion, "home.v2");
	assert.strictEqual(await fetchHomeTabBootstrap(), warmed);
	assert.equal(fetchCalls, 4, "surviving preload must populate memory");
});
