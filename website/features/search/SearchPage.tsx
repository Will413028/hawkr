import "@/styles/features/market.css";
import "@/styles/features/market-products-list.css";
/* RecommendedPostRail renders <PostCardV2>, which relies on
 * feed-components.css for badge / like / author absolute positioning.
 * Without this import the card chrome collapses to a plain block stack. */
import "@/styles/features/feed-components.css";
import "@/styles/features/search-agent-entry.css";
import { getCachedMarketplace } from "@/lib/api/cached";
import { fetchPublicPostsTyped } from "@/lib/api/posts";
import type { SearchProductHit } from "@/lib/search/repo";
import RecommendedPostRail from "./components/RecommendedPostRail";
import RecommendedProductRail from "./components/RecommendedProductRail";
import SearchClient from "./SearchClient";

/**
 * `/search` (noindex, ISR). The server renders only the query-less landing —
 * its two editorial rails are fetched through the shared `unstable_cache`
 * layer (no request-time input) so the route stays statically renderable. The
 * query-driven surface (search box, hashtag rail, results) is owned by
 * `SearchClient`, which reads `?q` / `?sort` on the client and fetches results
 * without a per-request dynamic server render.
 */
export default async function SearchPage() {
	// The two landing rails fetch concurrently but commit as one server tree,
	// then ship inside the static HTML.
	const [marketplace, landingPosts] = await Promise.all([
		getCachedMarketplace(4, 4, 8).catch(() => null),
		fetchPublicPostsTyped({ page: 1, pageSize: 8 }).catch(() => ({
			page: 1,
			pageSize: 8,
			total: 0,
			records: [],
		})),
	]);

	const landing = (
		<section
			className="w-[min(1440px,calc(100%-48px))] mx-auto max-md:w-[min(100%-28px,680px)] flex flex-col gap-10 pt-2"
			aria-label="探索內容"
		>
			<RecommendedProductRail
				products={(marketplace?.hotProducts ?? []) as SearchProductHit[]}
			/>
			<RecommendedPostRail posts={landingPosts.records ?? []} />
		</section>
	);

	return <SearchClient landing={landing} />;
}
