import "server-only";

import { unstable_cache } from "next/cache";
import {
	fetchCategoryTree,
	fetchMarketplace,
	fetchProducts,
	fetchSellers,
	type MarketplaceSeller,
} from "@/lib/api";
import {
	CACHE_TTL,
	coalesceRead,
	getCachedCategoryTree,
	getCachedMarketplace,
	getCachedProductsFirstPage,
	getCachedPublicPosts,
	getCachedRecommendedSellers,
	timedFetch,
} from "@/lib/api/cached";
import type { Paginated, PostListItem } from "@/lib/api/openapi-types";
import { fetchPublicPostsTyped } from "@/lib/api/posts";
import type { HomeTabBootstrapV1 } from "@/lib/api/tab-bootstrap";
import { homeBootstrapBudget } from "@/lib/config/env";
import {
	type CategoryGroup,
	categoryGroupsFromTree,
} from "@/lib/market/categoryGroups";
import { PUBLIC_CACHE_TAGS } from "@/lib/seo/tags";

const EMPTY_POSTS: Paginated<PostListItem> = {
	page: 1,
	pageSize: 8,
	total: 0,
	records: [],
};

type HomeLiveBootstrap = Pick<HomeTabBootstrapV1, "marketplace" | "products">;
type HomeDiscoveryBootstrap = Pick<
	HomeTabBootstrapV1,
	"recommendedSellers" | "posts" | "categoryGroups"
>;

/** The live wave carries the commerce surface: an empty one reads to a visitor
 *  as "this shop has no products", so it is never fabricated (see GET). */
type LiveDomain = "marketplace" | "products";
/**
 * Community posts sit here, not in the live wave, and that is a deliberate
 * product call: an empty story rail reads as "nothing posted lately" — a state
 * the home page renders every day — whereas an empty catalogue misrepresents the
 * business. Treating posts as an outage domain traded a truthful-but-quiet rail
 * for a 503 that also stops the price/stock refresh the visitor came for, so a
 * sick social service would take down commerce freshness with it. Absence is
 * still never silent: it is labelled `degraded` and the client keeps the rail it
 * already has (lib/api/tab-bootstrap.ts) rather than painting over it.
 */
type DiscoveryDomain = "recommendedSellers" | "posts" | "categoryGroups";
type BootstrapDomain = LiveDomain | DiscoveryDomain;

const LIVE_DOMAINS: readonly BootstrapDomain[] = ["marketplace", "products"];

/** A wave plus the domains inside it that no source could supply. */
interface Degradable<T> {
	value: T;
	degraded: BootstrapDomain[];
}

class BootstrapBudgetExceededError extends Error {
	constructor(label: string, budgetMs: number) {
		super(`${label} exceeded its ${budgetMs}ms bootstrap budget`);
		this.name = "BootstrapBudgetExceededError";
	}
}

/**
 * Bound one read by wall clock.
 *
 * The reads below own no request-level signal (the cache wrappers take none), so
 * the deadline lives here: whatever has not resolved in `budgetMs` stops being
 * waited on. The loser of the race already has a rejection handler attached, so
 * a late failure cannot surface as an unhandled rejection.
 */
function withDeadline<T>(
	work: Promise<T>,
	budgetMs: number,
	label: string,
): Promise<T> {
	let timer: ReturnType<typeof setTimeout> | undefined;
	const deadline = new Promise<never>((_resolve, reject) => {
		timer = setTimeout(
			() => reject(new BootstrapBudgetExceededError(label, budgetMs)),
			budgetMs,
		);
	});
	return Promise.race([work, deadline]).finally(() => {
		if (timer !== undefined) clearTimeout(timer);
	});
}

/**
 * What is left of the request budget, floored so a cache-hit fallback still gets
 * a chance after the primary burnt everything. The floor — not another full
 * budget — is what keeps the worst case at `totalMs + fallbackFloorMs`.
 */
function remainingBudgetMs(deadlineAt: number, floorMs: number): number {
	return Math.max(deadlineAt - Date.now(), floorMs);
}

function orDegraded<T>(
	result: PromiseSettledResult<T>,
	domain: BootstrapDomain,
	degraded: BootstrapDomain[],
	empty: T,
): T {
	if (result.status === "fulfilled") return result.value;
	degraded.push(domain);
	return empty;
}

// This aggregate is the sole cache owner for the live wave. Calling raw, strict
// anonymous fetchers avoids nested SWR windows re-caching stale price/stock data
// for an additional cycle. The `unstable_cache` entry is shared across sessions
// and tag-invalidated by POST /api/revalidate; the CACHE_TTL.COMMERCE_LIVE
// revalidate is the TTL safety net for a missed invalidation, not the freshness
// path.
const getCachedHomeLiveBootstrap = unstable_cache(
	(): Promise<HomeLiveBootstrap> =>
		// Timed as one unit: the raw fetchers below deliberately bypass the
		// per-domain cached wrappers (where slow-miss timing normally lives), so
		// without this the home live fan-out could stall past a client loader
		// timeout and emit no slow-cache-miss beacon / cache-health signal.
		timedFetch(
			"home-live-bootstrap",
			async () => {
				const [marketplace, products] = await Promise.all([
					fetchMarketplace(8, 6, 8, undefined, true),
					fetchProducts("", 1, 100, undefined, undefined, { strict: true }),
				]);
				return { marketplace, products };
			},
			{
				key: ["home-live-bootstrap"],
				endpoint: "/public/marketplace, /public/products",
			},
		),
	["home-live-bootstrap"],
	{
		// The marketplace payload folded in here embeds recommended sellers and
		// latest posts alongside its hot-product list, so all three domains tag
		// this entry even though the standalone seller / post lists live in the
		// discovery aggregate.
		tags: [
			PUBLIC_CACHE_TAGS.products,
			PUBLIC_CACHE_TAGS.sellers,
			PUBLIC_CACHE_TAGS.posts,
		],
		revalidate: CACHE_TTL.COMMERCE_LIVE,
	},
);

const getCachedHomeDiscoveryBootstrap = unstable_cache(
	(): Promise<HomeDiscoveryBootstrap> =>
		timedFetch(
			"home-discovery-bootstrap",
			async () => {
				const [recommendedSellers, posts, categoryTree] = await Promise.all([
					fetchSellers(24, undefined, true),
					fetchPublicPostsTyped({ page: 1, pageSize: 8, strict: true }),
					fetchCategoryTree(true),
				]);
				return {
					recommendedSellers,
					posts,
					// Rolled up here, not in the browser: the shelves need only the
					// lv1 → descendant-id index, never the ~200KB tree itself.
					categoryGroups: categoryGroupsFromTree(categoryTree),
				};
			},
			{
				key: ["home-discovery-bootstrap"],
				endpoint:
					"/public/sellers/recommended, /public/posts, /public/categories/tree",
			},
		),
	["home-discovery-bootstrap"],
	{
		// A post write busts this entry through the posts tag; CACHE_TTL.DISCOVERY
		// is only the safety net for a missed invalidation, as it is for sellers
		// and the category tree.
		tags: [
			PUBLIC_CACHE_TAGS.sellers,
			PUBLIC_CACHE_TAGS.posts,
			PUBLIC_CACHE_TAGS.categories,
		],
		revalidate: CACHE_TTL.DISCOVERY,
	},
);

/**
 * Second source for the live wave: the per-domain cache entries.
 *
 * `budgetMs` is what remains of the request, not a fresh allowance — the primary
 * has just demonstrated that the upstream is unreachable right now, so paying a
 * second full upstream wait would only extend the visitor's stall. A cache hit
 * answers inside the floor; anything slower is reported as unsupplied rather
 * than waited out.
 */
async function fallbackLiveBootstrap(
	budgetMs: number,
): Promise<Degradable<HomeLiveBootstrap>> {
	const [marketplace, products] = await Promise.allSettled([
		withDeadline(getCachedMarketplace(8, 6, 8), budgetMs, "marketplace"),
		withDeadline(getCachedProductsFirstPage(100), budgetMs, "products"),
	]);
	const degraded: BootstrapDomain[] = [];
	return {
		value: {
			marketplace: orDegraded(marketplace, "marketplace", degraded, null),
			products: orDegraded<unknown>(products, "products", degraded, {
				data: { records: [] },
			}),
		},
		degraded,
	};
}

async function fallbackDiscoveryBootstrap(
	budgetMs: number,
): Promise<Degradable<HomeDiscoveryBootstrap>> {
	const [recommendedSellers, posts, categoryGroups] = await Promise.allSettled([
		withDeadline(getCachedRecommendedSellers(24), budgetMs, "sellers"),
		withDeadline(getCachedPublicPosts(1, 8), budgetMs, "posts"),
		withDeadline(
			getCachedCategoryTree().then(categoryGroupsFromTree),
			budgetMs,
			"categories",
		),
	]);
	const degraded: BootstrapDomain[] = [];
	return {
		value: {
			recommendedSellers: orDegraded(
				recommendedSellers,
				"recommendedSellers",
				degraded,
				[] as MarketplaceSeller[],
			),
			posts: orDegraded(posts, "posts", degraded, EMPTY_POSTS),
			categoryGroups: orDegraded(
				categoryGroups,
				"categoryGroups",
				degraded,
				[] as CategoryGroup[],
			),
		},
		degraded,
	};
}

/**
 * Anonymous, same-origin bootstrap for the canonical home tab.
 *
 * The response itself remains browser no-store. Each public domain read is
 * split into one live and one discovery aggregate cached through
 * `unstable_cache` (shared across sessions, tag-invalidated). The split
 * preserves commerce freshness while reducing the hot path from five R2 reads
 * to two.
 */
export async function GET() {
	// One budget for the whole request, shared by both attempts. The primary and
	// the cache fallback are consecutive tries at the same data, so the fallback
	// inherits what the primary left rather than opening a second allowance —
	// otherwise a stalled upstream costs a visitor two full waits for one answer.
	const budget = homeBootstrapBudget();
	const deadlineAt = Date.now() + budget.totalMs;
	const intact = <T>(value: T): Degradable<T> => ({ value, degraded: [] });
	// `coalesceRead` (backend#2264): `unstable_cache` does not coalesce concurrent
	// cold reads of the same key, so two visitors arriving on the home tab in one
	// isolate each opened their own gateway fan-out. Keyed on the same names the
	// cache entries use.
	const [live, discovery] = await Promise.all([
		withDeadline(
			coalesceRead(["home-live-bootstrap"], getCachedHomeLiveBootstrap),
			budget.totalMs,
			"live",
		)
			.then(intact)
			.catch(() =>
				fallbackLiveBootstrap(
					remainingBudgetMs(deadlineAt, budget.fallbackFloorMs),
				),
			),
		withDeadline(
			coalesceRead(
				["home-discovery-bootstrap"],
				getCachedHomeDiscoveryBootstrap,
			),
			budget.totalMs,
			"discovery",
		)
			.then(intact)
			.catch(() =>
				fallbackDiscoveryBootstrap(
					remainingBudgetMs(deadlineAt, budget.fallbackFloorMs),
				),
			),
	]);

	const degraded = [...live.degraded, ...discovery.degraded];
	const unavailable = degraded.filter((domain) =>
		LIVE_DOMAINS.includes(domain),
	);
	if (unavailable.length > 0) {
		// No source could supply the commerce wave. Answering 200 with empty
		// records would be indistinguishable from a genuinely empty catalogue and
		// would overwrite the snapshot the visitor is already looking at; an
		// explicit outage lets the caller keep what it has and retry.
		return Response.json(
			{ error: "bootstrap_unavailable", unavailable },
			{
				status: 503,
				headers: {
					"cache-control": "private, no-store",
					"x-content-type-options": "nosniff",
					"retry-after": "1",
				},
			},
		);
	}

	// Discovery is recoverable rather than load-bearing: marketplace already
	// carries recommended sellers, and an absent story rail is a state
	// the page renders normally. So a missing discovery wave stays a 200 — but it
	// is labelled, and `lib/api/tab-bootstrap.ts` reads the label to keep the rail
	// it already holds instead of caching the gap as the new truth.
	const payload: HomeTabBootstrapV1 & { degraded?: BootstrapDomain[] } = {
		schemaVersion: "home.v1",
		...live.value,
		...discovery.value,
		...(degraded.length > 0 ? { degraded } : {}),
	};

	return Response.json(payload, {
		headers: {
			"cache-control": "private, no-store",
			"x-content-type-options": "nosniff",
			...(degraded.length > 0
				? { "x-bootstrap-degraded": degraded.join(",") }
				: {}),
		},
	});
}
