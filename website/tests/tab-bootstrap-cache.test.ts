import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const cachedPath = new URL("../lib/api/cached.ts", import.meta.url);
const homePath = new URL("../features/home/HomePage.tsx", import.meta.url);
const feedPath = new URL("../features/feed/FeedPage.tsx", import.meta.url);
const marketPath = new URL(
	"../features/market/components/MarketProductsSection.tsx",
	import.meta.url,
);
const searchPath = new URL(
	"../features/search/SearchPage.tsx",
	import.meta.url,
);
// The query-aware hashtag rail moved to a client component when /search became
// ISR (server renders only the landing; `?q` is read on the client).
const searchHashtagNavPath = new URL(
	"../features/search/components/SearchHashtagNav.tsx",
	import.meta.url,
);
const routePath = new URL(
	"../app/api/public/tab-bootstrap/v1/home/route.ts",
	import.meta.url,
);
const routeOwnerPath = new URL(
	"../features/home/server/home-tab-bootstrap-route.ts",
	import.meta.url,
);
const warmRoutePath = new URL(
	"../app/api/public/tab-bootstrap/v1/warm/route.ts",
	import.meta.url,
);
const headRoutePath = new URL(
	"../app/api/public/tab-bootstrap/v1/head/route.ts",
	import.meta.url,
);
const navPrefetchPath = new URL(
	"../lib/navigation/main-nav-prefetch.ts",
	import.meta.url,
);
const tabBootstrapClientPath = new URL(
	"../lib/api/tab-bootstrap.ts",
	import.meta.url,
);
const feedClientShellPath = new URL(
	"../features/feed/components/FeedClientShell.tsx",
	import.meta.url,
);
const feedPostsHookPath = new URL(
	"../lib/hooks/useFeedPosts.ts",
	import.meta.url,
);
const mainTabPagePaths = [
	"../app/[locale]/(public)/page.tsx",
	"../app/[locale]/(public)/feed/page.tsx",
	"../app/[locale]/(public)/market/products/page.tsx",
	"../app/[locale]/(public)/search/page.tsx",
].map((pathname) => new URL(pathname, import.meta.url));

// Every reader in lib/api/cached.ts is declared `export const <name> = cache(`:
// the React.cache wrapper that coalesces same-key reads inside one request sits
// outside the unstable_cache scope, so the declaration is a const, not a
// function. Slice a reader's body from its declaration to the next one — the
// tag/TTL invariants below are what these tests protect; the declaration shape
// is only how they are located.
const readerBody = (source: string, name: string) => {
	const start = source.indexOf(`export const ${name} = cache(`);
	assert.notEqual(start, -1, `missing ${name}`);
	const next = source.indexOf("\nexport const ", start + 1);
	return source.slice(start, next === -1 ? undefined : next);
};

// A landing read tags the data domains it embeds — never the page it renders on
// and never a legacy catch-all, so one mutation cannot shotgun unrelated caches.
test("public landing reads tag exactly the domains they embed", async () => {
	const source = await readFile(cachedPath, "utf8");

	// The literal contents of the fetcher's `tags: [...]` array (bracket-matched,
	// so nested arrays or later options never leak in).
	const tagsArrayOf = (name: string) => {
		const body = readerBody(source, name);
		const open = body.indexOf("tags: [");
		assert.notEqual(open, -1, `${name}: no tags array`);
		const start = open + "tags: [".length;
		let depth = 1;
		let cursor = start;
		while (cursor < body.length && depth > 0) {
			if (body[cursor] === "[") depth += 1;
			else if (body[cursor] === "]") depth -= 1;
			cursor += 1;
		}
		assert.equal(depth, 0, `${name}: unbalanced tags array`);
		return body.slice(start, cursor - 1);
	};
	const TAG_REF = /PUBLIC_CACHE_TAGS\.(\w+)/g;
	const expected: Array<[string, string[]]> = [
		["getCachedHotProducts", ["PUBLIC_CACHE_TAGS.products"]],
		["getCachedProductsFirstPage", ["PUBLIC_CACHE_TAGS.products"]],
		["getCachedStrictProductsPage", ["PUBLIC_CACHE_TAGS.products"]],
		["getCachedRecommendedSellers", ["PUBLIC_CACHE_TAGS.sellers"]],
		["getCachedPublicPosts", ["PUBLIC_CACHE_TAGS.posts"]],
		["getCachedPopularHashtags", ["PUBLIC_CACHE_TAGS.hashtags"]],
		["getCachedCategoryTree", ["PUBLIC_CACHE_TAGS.categories"]],
	];
	for (const [name, tags] of expected) {
		const declared = tagsArrayOf(name);
		const found = [...declared.matchAll(TAG_REF)].map(
			(match) => `PUBLIC_CACHE_TAGS.${match[1]}`,
		);
		// Exact set, not "contains": one extra domain tag on a landing fetcher
		// re-creates the shotgun blast this taxonomy removed (a product edit would
		// bust hashtags again), and a containment assertion cannot see it.
		assert.deepEqual(
			[...found].sort(),
			[...tags].sort(),
			`${name} tags: expected exactly ${tags.join(", ")}`,
		);
		// Nothing but PUBLIC_CACHE_TAGS entries in the array — catches a raw
		// "marketplace"/"home" string, a tab:* literal or a smuggled entity tag.
		const residual = declared.replace(TAG_REF, "").replace(/[\s,]/g, "");
		assert.equal(
			residual,
			"",
			`${name}: unexpected tag expression ${residual}`,
		);
	}
	assert.doesNotMatch(source, /LEGACY_HOME_CACHE_TAG/);
});

test("canonical navbar destinations keep Feed bounded and reuse safe cache entries", async () => {
	const [feed, market, search, searchHashtagNav] = await Promise.all([
		readFile(feedPath, "utf8"),
		readFile(marketPath, "utf8"),
		readFile(searchPath, "utf8"),
		readFile(searchHashtagNavPath, "utf8"),
	]);

	assert.match(feed, /AbortSignal\.timeout\(FEED_INITIAL_DEADLINE_MS\)/);
	assert.match(feed, /fetchPublicPostsTyped\(\{[\s\S]*page: 1,[\s\S]*signal,/);
	assert.match(feed, /strict: true/);
	assert.match(feed, /initialSeedFailed = true/);
	assert.doesNotMatch(feed, /fetchSellers/);
	assert.doesNotMatch(feed, /getCachedStrictPublicPostsPage/);
	assert.match(market, /getCachedProductsFirstPage\(100\)/);
	assert.match(search, /getCachedHotProducts\(8\)/);
	assert.match(search, /fetchPublicPostsTyped\(\{ page: 1, pageSize: 8 \}\)/);
	// Live, query-aware hashtags (not the cached popular set) — now client-side.
	assert.match(searchHashtagNav, /searchHashtags\(query\)/);
	assert.doesNotMatch(searchHashtagNav, /getCachedPopularHashtags/);
	assert.doesNotMatch(search, /getCachedPublicPosts\(1, 8\)/);
	assert.doesNotMatch(search, /getCachedPopularHashtags\(10\)/);
});

test("home bootstrap is versioned, anonymous and browser no-store", async () => {
	const [home, route, handler] = await Promise.all([
		readFile(homePath, "utf8"),
		readFile(routePath, "utf8"),
		readFile(routeOwnerPath, "utf8"),
	]);

	assert.match(home, /fetchHomeTabBootstrap\(controller\.signal\)/);
	// The client abort deadline is per-flavor config, never a literal here: a
	// local number can be shortened below the server budget, turning the tail of
	// that budget into an answer nobody is still listening for.
	assert.match(home, /homeSnapshotDeadlineMs\(\)/);
	assert.doesNotMatch(home, /DEADLINE_MS\s*=\s*\d/);
	assert.match(route, /export const dynamic = "force-dynamic"/);
	assert.match(
		route,
		/export \{ GET \} from "@\/features\/home\/server\/home-tab-bootstrap-route"/,
	);
	assert.doesNotMatch(route, /unstable_cache|fetchHotProducts|function GET/);
	assert.match(handler, /^import "server-only";/);
	assert.match(handler, /schemaVersion: "home\.v2"/);
	assert.match(handler, /fetchHotProducts\(8, \{ strict: true \}\)/);
	assert.match(
		handler,
		/fetchProducts\("", 1, 100, undefined, undefined, \{ strict: true \}\)/,
	);
	assert.match(
		handler,
		/fetchPublicPostsTyped\(\{ page: 1, pageSize: 8, strict: true \}\)/,
	);
	// Live aggregate keeps the live-commerce revalidate window (the shared
	// CACHE_TTL model, now an unstable_cache option rather than a cacheLife profile).
	assert.match(handler, /revalidate: CACHE_TTL\.COMMERCE_LIVE/);
	const liveStart = handler.indexOf(
		"getCachedHomeLiveBootstrap = unstable_cache",
	);
	const discoveryStart = handler.indexOf(
		"getCachedHomeDiscoveryBootstrap = unstable_cache",
	);
	const fallbackStart = handler.indexOf("async function fallbackLiveBootstrap");
	assert.ok(liveStart >= 0 && discoveryStart > liveStart);
	assert.ok(fallbackStart > discoveryStart);
	assert.doesNotMatch(
		handler.slice(liveStart, fallbackStart),
		/\bgetCached(?:HotProducts|Products|PublicPosts|Recommended)/,
		"aggregate cache owners must call raw fetchers, not nested SWR caches",
	);
	assert.match(handler, /PUBLIC_CACHE_TAGS\.products/);
	assert.match(handler, /PUBLIC_CACHE_TAGS\.posts/);
	assert.match(handler, /PUBLIC_CACHE_TAGS\.sellers/);
	assert.doesNotMatch(handler, /tab(?:Home|Feed|Market|Search)|"home"/);
	assert.match(handler, /"cache-control": "private, no-store"/);
	assert.doesNotMatch(handler, /cookies\(|headers\(|Authorization|credentials/);
});

test("canonical freshness keeps Feed direct and bounded while Market stays shared", async () => {
	const [cached, route] = await Promise.all([
		readFile(cachedPath, "utf8"),
		readFile(headRoutePath, "utf8"),
	]);
	assert.match(route, /fetchPublicPostsTyped\(\{[\s\S]*strict: true,/);
	assert.match(route, /AbortSignal\.timeout\(FEED_REFRESH_DEADLINE_MS\)/);
	assert.doesNotMatch(route, /getCachedStrictPublicPostsPage/);
	assert.match(route, /getCachedStrictProductsPage\(input\.page, 24\)/);
	assert.match(route, /schemaVersion: "tab-page\.v1"/);
	assert.match(route, /"cache-control": "private, no-store"/);
	for (const name of ["getCachedStrictProductsPage"]) {
		const body = readerBody(cached, name);
		assert.match(body, /revalidate: CACHE_TTL\.COMMERCE_LIVE/);
		assert.match(body, /strict: true/);
	}
});

test("seller storefront product cards use live commerce cache windows", async () => {
	const source = await readFile(cachedPath, "utf8");
	for (const name of [
		"getCachedSellerProducts",
		"getCachedSellerStorefrontProducts",
	]) {
		const body = readerBody(source, name);
		assert.match(body, /revalidate: CACHE_TTL\.COMMERCE_LIVE/);
		// products:list (busted by the revalidate parser for ANY product mutation)
		// + seller:<id> replace the result-derived per-product tags that
		// unstable_cache cannot attach dynamically.
		assert.match(body, /PUBLIC_CACHE_TAGS\.products/);
		assert.match(body, /sellerTag\(id\)/);
		assert.doesNotMatch(body, /revalidate: CACHE_TTL\.SELLER_PROFILE/);
	}
});

test("idle route warming avoids an extra data warm while Feed and touch retain their boundaries", async () => {
	const [warmRoute, navPrefetch, tabBootstrapClient, feedShell, feedHook] =
		await Promise.all([
			readFile(warmRoutePath, "utf8"),
			readFile(navPrefetchPath, "utf8"),
			readFile(tabBootstrapClientPath, "utf8"),
			readFile(feedClientShellPath, "utf8"),
			readFile(feedPostsHookPath, "utf8"),
		]);

	assert.match(warmRoute, /tab === "feed"/);
	assert.match(warmRoute, /tab === "market"/);
	assert.doesNotMatch(warmRoute, /fetchPublicPostsTyped|fetchSellers/);
	assert.doesNotMatch(warmRoute, /getCachedStrictPublicPostsPage/);
	assert.match(warmRoute, /getCachedHotProducts\(8\)/);
	assert.match(warmRoute, /status: 204/);
	assert.match(warmRoute, /"cache-control": "private, no-store"/);
	assert.doesNotMatch(tabBootstrapClient, /"\/feed": "feed"/);
	assert.match(feedShell, /initialSeedFailed/);
	assert.match(feedShell, /queryKeys\.feed\.recommendedSellers\(\)/);
	assert.match(feedShell, /fetchRecommendedStores\(24, \{ signal \}\)/);
	assert.match(feedHook, /initialData: hasInitialData[\s\S]*: undefined/);
	assert.match(feedHook, /initialLoading: query\.isPending/);
	assert.match(navPrefetch, /if \(intent === "touch"\)/);
	assert.match(navPrefetch, /router\.prefetch\(href\)/);
	assert.doesNotMatch(
		navPrefetch,
		/\bpreloadMainTabData\b/,
		"idle route prefetch must not add a second data-warming request wave",
	);
});

test("public main tabs retain their rendered tree for instant revisits", async () => {
	const pages = await Promise.all(
		mainTabPagePaths.map((pathname) => readFile(pathname, "utf8")),
	);

	for (const source of pages) {
		assert.match(source, /export const unstable_dynamicStaleTime = 300/);
	}
});
