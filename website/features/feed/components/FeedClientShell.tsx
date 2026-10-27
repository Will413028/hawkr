"use client";

import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import FeedStories, {
	type StoryTabKey,
} from "@/features/feed/components/FeedStories";
import InfiniteFeed from "@/features/feed/components/InfiniteFeed";
import type { StoryItem } from "@/features/feed/components/StoryRail";
import { useFeedUiStore } from "@/features/feed/feedUiStore";
import { fetchSellers, type MarketplaceSeller } from "@/lib/api";
import type { PostListItem } from "@/lib/api/openapi-types";
import {
	type FeedListSnapshot,
	feedListSnapshotForInitialRender,
	peekFeedListSnapshot,
	takeFeedListSnapshot,
} from "@/lib/feed/listSnapshot";
import { useHydrationSafeClientRender } from "@/lib/hooks/useHydrationSafeClientRender";
import { queryKeys } from "@/lib/query/keys";
import { recommendedSellerLogoUrl } from "@/lib/seller/recommended-logo";

interface FeedClientShellProps {
	stories: StoryItem[];
	initialPosts: PostListItem[];
	initialPage?: number;
	initialHasMore?: boolean;
	pageSize: number;
	sellerId?: string;
	hashtag?: string;
	hasActiveFilter?: boolean;
	/** The server seed timed out or failed. A valid empty feed keeps the normal
	 * empty state; a failed seed mounts the client query so it can recover. */
	initialSeedFailed?: boolean;
	/** When set, InfiniteFeed auto-opens the post detail viewer on the
	 *  matching post once it appears in the loaded set. Used by deep
	 *  links like /feed?post=123 (megamenu's 本週主題 CTA). */
	initialPostId?: string;
	/** Client-fetched deep-linked post (ISR route reads `?post` on the client);
	 *  prepended into the feed so the viewer can open it. */
	deepLinkPost?: PostListItem | null;
}

function readNumber(
	source: Record<string, unknown>,
	keys: string[],
): number | undefined {
	for (const key of keys) {
		const raw = source[key];
		const value = typeof raw === "string" ? Number(raw) : raw;
		if (typeof value === "number" && Number.isFinite(value)) return value;
	}
	return undefined;
}

function buildSellerStories(sellers: MarketplaceSeller[]): StoryItem[] {
	return sellers
		.map((seller, index) => {
			const raw = seller as MarketplaceSeller & Record<string, unknown>;
			const rawId = seller.storeId;
			const id =
				typeof rawId === "string" || typeof rawId === "number" ? rawId : index;
			return {
				id,
				// StoryRail feeds this straight into `sellerPath()`, so omitting it
				// silently drops every avatar in the feed rail back onto its storeId
				// — one 308 per tap (#365).
				storeCode: seller.storeCode?.trim() || undefined,
				name: seller.storeName || "Hawkr 商家",
				avatar: recommendedSellerLogoUrl(seller),
				city: seller.city,
				district: seller.district,
				// 推薦商家 record 沒有座標欄（recommended_sellers.go:40-54）；
				// 保留這兩個讀取是為了與 /market 的同一條 rail 共用形狀。
				latitude: readNumber(raw, ["latitude", "lat"]),
				longitude: readNumber(raw, ["longitude", "lng"]),
				// 刻意不設 hasUnread：推薦商家清單沒有「未讀 / 有新動態」訊號，
				// 全部設 true 會讓整排都亮著狀態環（環就失去語意），而且
				// /market 走同一份資料卻沒有這個欄位 → 兩個頁面長得不一樣。
			} satisfies StoryItem;
		})
		.slice(0, 12);
}

/**
 * Glue layer between the (top) story rail tabs and the (bottom) post
 * grid. Lifts `activeTab` out of `FeedStories` so the post grid can
 * react to the same state.
 *
 * Tabs map to post-grid behaviour like this:
 *   - 推薦  / 附近商家  → no post filter (full feed shown)
 *   - 追蹤中            → posts filtered to those whose `seller_id`
 *                          is in the buyer's followed-seller set
 *
 * Per the OpenAPI audit, `/public/posts` does NOT accept a
 * subscribed/seller_ids query param, so the filter is currently
 * client-side over already-fetched pages. A backend-side filter is
 * tracked separately (see docs/backend-issues/feed-subscribed.md).
 */
export default function FeedClientShell({
	stories,
	initialPosts,
	initialPage,
	initialHasMore,
	pageSize,
	sellerId,
	hashtag,
	hasActiveFilter = false,
	initialSeedFailed = false,
	initialPostId,
	deepLinkPost,
}: FeedClientShellProps) {
	const hydrationSafeClientRender = useHydrationSafeClientRender();
	const [restoredSnapshot] = React.useState<FeedListSnapshot | null>(() =>
		feedListSnapshotForInitialRender(hydrationSafeClientRender, () =>
			peekFeedListSnapshot(window.location.pathname + window.location.search),
		),
	);
	React.useEffect(() => {
		// Hard hydration deliberately ignored any pre-existing browser snapshot
		// so its first client tree matched SSR. Consume that stale entry now.
		// A client-only mount already peeked the same entry synchronously above;
		// consuming after commit preserves no-flash back restoration while
		// remaining safe under Strict Mode/concurrent render retries.
		takeFeedListSnapshot(window.location.pathname + window.location.search);
	}, []);
	const canEnrichStories =
		!sellerId && !hashtag && (!initialSeedFailed || Boolean(restoredSnapshot));
	const sellersQuery = useQuery({
		queryKey: queryKeys.feed.recommendedSellers(),
		queryFn: ({ signal }) => fetchSellers(24, signal, true),
		enabled: canEnrichStories,
		staleTime: 10 * 60 * 1000,
		gcTime: 30 * 60 * 1000,
		refetchOnMount: false,
	});
	const enrichedStories = React.useMemo(() => {
		const sellers = sellersQuery.data ?? [];
		return sellers.length > 0 ? buildSellerStories(sellers) : stories;
	}, [sellersQuery.data, stories]);
	const feedScopeKey = `${sellerId ?? ""}:${hashtag ?? ""}`;
	const storeActiveTab = useFeedUiStore((state) =>
		state.activeTabScopeKey === feedScopeKey ? state.activeTab : "recommend",
	);
	const setActiveTab = useFeedUiStore((state) => state.setActiveTab);
	const setScopedActiveTab = React.useCallback(
		(nextTab: StoryTabKey) => setActiveTab(nextTab, feedScopeKey),
		[feedScopeKey, setActiveTab],
	);
	const [snapshotTabApplied, setSnapshotTabApplied] = React.useState(false);
	const activeTab = (
		!snapshotTabApplied && restoredSnapshot?.activeTab
			? restoredSnapshot.activeTab
			: storeActiveTab
	) as StoryTabKey;
	React.useEffect(() => {
		if (!restoredSnapshot?.activeTab || snapshotTabApplied) return;
		setActiveTab(restoredSnapshot.activeTab, feedScopeKey);
		setSnapshotTabApplied(true);
	}, [feedScopeKey, restoredSnapshot, setActiveTab, snapshotTabApplied]);
	const isFollowMode = activeTab === "follow";
	const showFeed =
		initialPosts.length > 0 || Boolean(restoredSnapshot) || initialSeedFailed;

	return (
		<>
			{enrichedStories.length > 0 ? (
				<FeedStories
					items={enrichedStories}
					activeTab={activeTab}
					onTabChange={setScopedActiveTab}
					// When the rail will be enriched with region-bearing sellers
					// client-side, reserve the region sub-line from the server render
					// so the swap doesn't grow the rail 16px and push the grid down.
					reserveRegion={canEnrichStories}
				/>
			) : null}
			{showFeed ? (
				<InfiniteFeed
					progressiveImages
					initialPosts={initialPosts}
					initialPage={initialPage}
					initialHasMore={initialHasMore}
					pageSize={pageSize}
					sellerId={sellerId}
					hashtag={hashtag}
					initialSeedFailed={initialSeedFailed}
					hasActiveFilter={hasActiveFilter}
					filterMode={isFollowMode ? "followed" : "all"}
					activeTab={activeTab}
					initialPostId={initialPostId}
					deepLinkPost={deepLinkPost}
					restoredSnapshot={restoredSnapshot}
				/>
			) : (
				<EmptyStatePanel
					className="hkui-feed-end hkui-feed-end--empty"
					icon="document"
					title={hasActiveFilter ? "目前沒有符合條件的動態" : "目前沒有動態"}
					description={
						hasActiveFilter
							? "換個主題、商家或回到推薦動態再試一次。"
							: "新的店家故事準備好後會出現在這裡。"
					}
					iconSize={108}
				/>
			)}
		</>
	);
}
