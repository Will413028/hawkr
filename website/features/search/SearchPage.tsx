import "@/styles/features/market.css";
import "@/styles/features/market-products-list.css";
import "@/styles/features/feed-components.css";
import "@/styles/features/search-agent-entry.css";
import { getCachedHotProducts, getCachedPublicPosts } from "@/lib/api/cached";
import type { SearchProductHit } from "@/lib/search/repo";
import RecommendedPostRail from "./components/RecommendedPostRail";
import RecommendedProductRail from "./components/RecommendedProductRail";
import SearchClient from "./SearchClient";

/**
 * `/search` (noindex, ISR). The server renders only the query-less landing —
 * its two editorial rails are fetched through the shared `use cache`
 * layer (no request-time input) so the route stays statically renderable. The
 * query-driven surface (search box, hashtag rail, results) is owned by
 * `SearchClient`, which reads `?q` / `?sort` on the client and fetches results
 * without a per-request dynamic server render.
 */
export default async function SearchPage() {
	const [hotProducts, landingPosts] = await Promise.all([
		getCachedHotProducts(8).catch(() => []),
		getCachedPublicPosts(8).catch(() => ({
			pageSize: 8,
			nextPageToken: "",
			records: [],
		})),
	]);

	const landing = (
		<section
			className="w-[min(1440px,calc(100%-48px))] mx-auto max-md:w-[min(100%-28px,680px)] flex flex-col gap-10 pt-2"
			aria-label="探索內容"
		>
			<RecommendedProductRail products={hotProducts as SearchProductHit[]} />
			<RecommendedPostRail posts={landingPosts.records ?? []} />
		</section>
	);

	return <SearchClient landing={landing} />;
}
