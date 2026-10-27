import {
	getCachedCategoryTree,
	getCachedMarketplace,
	getCachedPopularHashtags,
	getCachedProductsFirstPage,
	getCachedRecommendedSellers,
} from "@/lib/api/cached";

type WarmableTab = "feed" | "market" | "search";

function isWarmableTab(value: string | null): value is WarmableTab {
	return value === "feed" || value === "market" || value === "search";
}

/**
 * Data-only idle warmer for the anonymous main tabs.
 *
 * `router.prefetch()` remains reserved for hover/focus intent because those
 * signals provide real dwell time before navigation. Idle warming uses this
 * endpoint; touch pointer-down starts no speculative request so the following
 * click owns the connection. Market/Search fill the exact safe Cache-Components
 * entries used by their canonical landings. `feed` stays accepted for older
 * clients but is a no-op: its bounded direct gateway response is private and
 * cannot warm browser or Cache Components state, so speculative reads would
 * only duplicate the real navigation.
 */
export async function GET(request: Request) {
	const tab = new URL(request.url).searchParams.get("tab");
	if (!isWarmableTab(tab)) {
		return Response.json(
			{ code: 400, message: "invalid tab" },
			{
				status: 400,
				headers: { "cache-control": "private, no-store" },
			},
		);
	}

	if (tab === "feed") {
		// Backward-compatible no-op for already-loaded clients.
	} else if (tab === "market") {
		await Promise.all([
			getCachedProductsFirstPage(100).catch(() => null),
			getCachedCategoryTree().catch(() => null),
			getCachedPopularHashtags(8).catch(() => null),
			getCachedRecommendedSellers(12).catch(() => null),
		]);
	} else {
		await getCachedMarketplace(4, 4, 8).catch(() => null);
	}

	return new Response(null, {
		status: 204,
		headers: {
			"cache-control": "private, no-store",
			"x-content-type-options": "nosniff",
		},
	});
}
