import "server-only";

import {
	getCachedCategoryTree,
	getCachedHotProducts,
	getCachedProductsFirstPage,
	getCachedPublicPosts,
	getCachedRecommendedSellers,
} from "@/lib/api/cached";
import type { HomeTabBootstrapV1 } from "@/lib/api/tab-bootstrap";
import { categoryGroupsFromTree } from "@/lib/market/categoryGroups";

/**
 * Empty-but-valid bootstrap for the build-phase `prerenderSafe` fallback only.
 * `buildHomeSnapshot` maps this to a content-free snapshot without crashing, so
 * `next build` never dies on a transient backend blip; ISR fills real data on
 * the first runtime request. Never used at runtime (see `loadHomeRenderBootstrap`).
 */
export const EMPTY_HOME_BOOTSTRAP: HomeTabBootstrapV1 = {
	schemaVersion: "home.v2",
	hotProducts: [],
	products: { data: { records: [] } },
	recommendedSellers: [],
	posts: { page: 1, pageSize: 8, total: 0, records: [] },
	categoryGroups: [],
};

/**
 * Strict, server-side assembly of the anonymous home bootstrap for the ISR page
 * render.
 *
 * Each `getCached*` helper wraps a `strict: true` fetcher, so a backend outage
 * throws here rather than resolving to a synthetic empty payload. That is
 * deliberate: this feeds the server-rendered `/` HTML, and an empty snapshot
 * cached as fresh would silently drop every product/seller/post card from the
 * SEO surface. On a throw the page render fails and ISR keeps serving the last
 * good prerender (stale-while-revalidate) — never a fresh empty homepage. Build
 * -phase resilience (no prior prerender to fall back to) is handled at the page
 * via `prerenderSafe`, which soft-fails ONLY during `next build`; at runtime it
 * re-throws so this strict contract holds.
 *
 * The args mirror the client bootstrap route exactly
 * (`app/api/public/tab-bootstrap/v1/home/route.ts`) so the server-rendered
 * snapshot and the browser refresh resolve identical data. These per-domain
 * cache entries are also shared with the market/search/feed pages and are
 * tag-invalidated by POST /api/revalidate, so no home-only cache is introduced.
 */
export async function loadHomeRenderBootstrap(): Promise<HomeTabBootstrapV1> {
	const [hotProducts, products, recommendedSellers, posts, categoryTree] =
		await Promise.all([
			getCachedHotProducts(8),
			getCachedProductsFirstPage(100),
			getCachedRecommendedSellers(24),
			getCachedPublicPosts(1, 8),
			getCachedCategoryTree(),
		]);
	return {
		schemaVersion: "home.v2",
		hotProducts,
		products,
		recommendedSellers,
		posts,
		categoryGroups: categoryGroupsFromTree(categoryTree),
	};
}
