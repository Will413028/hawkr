import assert from "node:assert/strict";
import test from "node:test";
import {
	fetchHomeTabBootstrap,
	HOME_TAB_BOOTSTRAP_DEGRADED_FRESH_MS,
	HOME_TAB_BOOTSTRAP_FRESH_MS,
	HOME_TAB_BOOTSTRAP_TTL_MS,
	preloadHomeTabBootstrap,
} from "../lib/api/tab-bootstrap";

const POSTS = {
	page: 1,
	pageSize: 8,
	total: 1,
	records: [{ id: "8001" }],
};
const EMPTY_POSTS = { page: 1, pageSize: 8, total: 0, records: [] };
const SELLERS = [{ storeId: 7001 }];

/** A healthy answer: every domain supplied, no label. */
function intact(productId: string) {
	return {
		schemaVersion: "home.v1",
		marketplace: { hot_products: [], recommended_sellers: [] },
		products: { data: { records: [{ id: productId }] } },
		recommendedSellers: SELLERS,
		posts: POSTS,
	};
}

/**
 * What the route actually emits when a domain could not be supplied: the field is
 * present but empty, and the gap is named. Commerce stays fresh (different
 * product id) so a test can tell retained fields from overwritten ones.
 */
function degraded(productId: string, domains: string[]) {
	return {
		...intact(productId),
		...(domains.includes("posts") ? { posts: EMPTY_POSTS } : {}),
		...(domains.includes("recommendedSellers")
			? { recommendedSellers: [] }
			: {}),
		degraded: domains,
	};
}

test("a degraded home bootstrap is labelled, short-lived and never blanks the rails", async (t) => {
	const realFetch = globalThis.fetch;
	const realNow = Date.now;
	t.after(() => {
		globalThis.fetch = realFetch;
		Date.now = realNow;
	});

	let now = 10_000;
	Date.now = () => now;
	let fetchCalls = 0;
	let nextBody: Record<string, unknown> = intact("9001");
	let nextHeaders: Record<string, string> = {};
	globalThis.fetch = (async () => {
		fetchCalls += 1;
		return new Response(JSON.stringify(nextBody), {
			headers: { "content-type": "application/json", ...nextHeaders },
		});
	}) as typeof fetch;

	const intactAt = now;
	const first = await fetchHomeTabBootstrap();
	assert.deepEqual(first.recommendedSellers, SELLERS);
	assert.deepEqual(first.posts, POSTS);

	// 1. A degraded answer keeps the fresh commerce half and carries the labelled
	//    domains forward, so the seller / story rails do not blink to empty.
	now += HOME_TAB_BOOTSTRAP_FRESH_MS + 1;
	const degradedAt = now;
	nextBody = degraded("9002", ["posts", "recommendedSellers"]);
	const merged = await fetchHomeTabBootstrap();
	assert.equal(fetchCalls, 2);
	assert.deepEqual(merged.products, { data: { records: [{ id: "9002" }] } });
	assert.deepEqual(merged.posts, POSTS, "labelled gap must not blank");
	assert.deepEqual(
		merged.recommendedSellers,
		SELLERS,
		"labelled gap must not blank",
	);

	// 2. It is trusted for seconds, not the full minute a healthy answer earns.
	now += HOME_TAB_BOOTSTRAP_DEGRADED_FRESH_MS - 1;
	await preloadHomeTabBootstrap();
	assert.equal(fetchCalls, 2, "inside the degraded window: no refetch");

	// 3. The header alone is enough of a label (an intermediary may strip the
	//    body field), and the retry happens while a healthy answer would still be
	//    considered fresh.
	now += 2;
	assert.ok(
		now - degradedAt < HOME_TAB_BOOTSTRAP_FRESH_MS,
		"the retry must land long before a healthy answer would have gone stale",
	);
	nextBody = { ...intact("9003"), recommendedSellers: [] };
	nextHeaders = { "x-bootstrap-degraded": "recommendedSellers" };
	const headerLabelled = await fetchHomeTabBootstrap();
	assert.equal(fetchCalls, 3, "an expired degraded window must refetch");
	assert.deepEqual(headerLabelled.products, {
		data: { records: [{ id: "9003" }] },
	});
	assert.deepEqual(
		headerLabelled.recommendedSellers,
		SELLERS,
		"header-only label must be honoured too",
	);

	// 4. Retention is bounded: past one TTL from the last intact read the carried
	//    content may no longer exist, so the gap is served as it is.
	now = intactAt + HOME_TAB_BOOTSTRAP_TTL_MS + 1;
	nextBody = degraded("9004", ["recommendedSellers"]);
	nextHeaders = {};
	const unretained = await fetchHomeTabBootstrap();
	assert.equal(fetchCalls, 4);
	assert.deepEqual(
		unretained.recommendedSellers,
		[],
		"retention must not chain past one TTL of real age",
	);

	// 5. A healthy answer restarts that clock.
	now += HOME_TAB_BOOTSTRAP_DEGRADED_FRESH_MS + 1;
	nextBody = intact("9005");
	const recovered = await fetchHomeTabBootstrap();
	assert.equal(fetchCalls, 5);
	assert.deepEqual(recovered.recommendedSellers, SELLERS);

	now += HOME_TAB_BOOTSTRAP_FRESH_MS + 1;
	nextBody = degraded("9006", ["recommendedSellers"]);
	const retainedAgain = await fetchHomeTabBootstrap();
	assert.equal(fetchCalls, 6);
	assert.deepEqual(
		retainedAgain.recommendedSellers,
		SELLERS,
		"a fresh intact read must re-arm retention",
	);
});
