"use client";

import * as React from "react";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import { FeedGridSkeleton } from "@/components/shared/RouteLoadingSkeleton";
import Button from "@/components/ui/Button";
import { useFeedUiStore } from "@/features/feed/feedUiStore";
import type { PostListItem } from "@/lib/api/openapi-types";
import {
	type FeedListSnapshot,
	type FeedListSnapshotTab,
	saveFeedListSnapshot,
} from "@/lib/feed/listSnapshot";
import { useFeedPosts } from "@/lib/hooks/useFeedPosts";
import { useFollowedSellers } from "@/lib/hooks/useFollowedSellers";
import PostCardV2 from "./PostCardV2";
import PostDetailViewer, { type OpenOriginRect } from "./PostDetailViewer";

const toOriginRect = (r: DOMRect): OpenOriginRect => ({
	top: r.top,
	left: r.left,
	width: r.width,
	height: r.height,
});

export type FeedFilterMode = "all" | "followed";

interface InfiniteFeedProps {
	initialPosts: PostListItem[];
	/** Public feed opts in; seller storefronts keep their existing eager policy. */
	progressiveImages?: boolean;
	initialPage?: number;
	/** Explicit continuation state for a deep-linked post. It seeds the viewer
	 * immediately, then lets its existing near-end loader fetch page 1. */
	initialHasMore?: boolean;
	pageSize?: number;
	sellerId?: number | string;
	hashtag?: string;
	initialSeedFailed?: boolean;
	/** Set by routes whose server render already fetched page one for this
	 * scope (seller posts), so the seed is kept instead of being refetched. */
	initialPostsMatchQuery?: boolean;
	hasActiveFilter?: boolean;
	/**
	 * `"all"` shows every fetched post; `"followed"` keeps only posts
	 * whose `sellerId` is in `followedIds`. Filter runs client-side
	 * because `/public/posts` doesn't accept a subscribed query
	 * (tracked in docs/backend-issues/feed-subscribed.md).
	 */
	filterMode?: FeedFilterMode;
	activeTab?: FeedListSnapshotTab;
	/** Post id from a deep link (e.g. /feed?post=123 from the megamenu's
	 *  本週主題 card). When this id is present in `visiblePosts`, the
	 *  detail viewer opens to it on mount. Fires once per id. */
	initialPostId?: string;
	/** Deep-linked post fetched on the client (ISR route reads `?post` without a
	 *  server render). Prepended to the visible list so the viewer can open it
	 *  even when it is not part of the canonical feed page. */
	deepLinkPost?: PostListItem | null;
	restoredSnapshot?: FeedListSnapshot | null;
}

export default function InfiniteFeed({
	initialPosts,
	progressiveImages = false,
	initialPage = 1,
	initialHasMore,
	pageSize = 12,
	sellerId,
	hashtag,
	initialSeedFailed = false,
	initialPostsMatchQuery = false,
	hasActiveFilter = false,
	filterMode = "all",
	activeTab = "recommend",
	initialPostId,
	deepLinkPost,
	restoredSnapshot,
}: InfiniteFeedProps) {
	const snapshotRef = React.useRef<FeedListSnapshot | null | undefined>(
		undefined,
	);
	if (snapshotRef.current === undefined) {
		snapshotRef.current = restoredSnapshot ?? null;
	}
	const restored = snapshotRef.current;
	const {
		posts,
		page,
		loading,
		initialLoading,
		error,
		done,
		loadMore,
		refresh,
		refreshing,
		hasPendingFresh,
		pendingNewCount,
		applyPendingFresh,
	} = useFeedPosts({
		initialPosts,
		initialPage,
		initialHasMore,
		pageSize,
		sellerId,
		hashtag,
		initialSeedFailed,
		initialPostsMatchQuery,
		restoredSnapshot: restored,
	});
	const following = useFollowedSellers(
		filterMode === "followed",
		posts.map((post) => post.sellerId),
	);
	const followedIds = following.ids;
	const followingLoading = following.loading;
	const followingReady = following.ready;
	const [manualRetrying, setManualRetrying] = React.useState(false);
	const retryInitialLoad = React.useCallback(async () => {
		if (manualRetrying) return;
		setManualRetrying(true);
		try {
			await refresh();
		} catch {
			// Keep the explicit error state visible; the next tap retries in place.
		} finally {
			setManualRetrying(false);
		}
	}, [manualRetrying, refresh]);
	const openPostId = useFeedUiStore((state) => state.openPostId);
	const setOpenPostId = useFeedUiStore((state) => state.setOpenPostId);
	// Box of the tapped card's thumbnail, in viewport px. The viewer scales the
	// whole detail surface out of this box on open (and back into it on close).
	const [openOrigin, setOpenOrigin] = React.useState<OpenOriginRect | null>(
		null,
	);

	React.useEffect(() => {
		if (!restored) return;
		const targetY = restored.scrollY;
		let raf = 0;
		let frames = 0;
		let settled = 0;
		const pin = () => {
			const root = document.documentElement;
			const previousBehavior = root.style.scrollBehavior;
			root.style.scrollBehavior = "auto";
			window.scrollTo(0, targetY);
			root.style.scrollBehavior = previousBehavior;
			frames += 1;
			settled = Math.abs(window.scrollY - targetY) < 2 ? settled + 1 : 0;
			if ((settled >= 3 && frames > 4) || frames > 60) return;
			raf = window.requestAnimationFrame(pin);
		};
		raf = window.requestAnimationFrame(pin);
		return () => window.cancelAnimationFrame(raf);
	}, [restored]);

	const snapshotStateRef = React.useRef({
		posts,
		page,
		done,
		activeTab,
		filterMode,
	});
	snapshotStateRef.current = { posts, page, done, activeTab, filterMode };
	const saveSnapshot = React.useCallback(() => {
		const { posts, page, done, activeTab, filterMode } =
			snapshotStateRef.current;
		if (typeof window === "undefined" || posts.length === 0) return;
		saveFeedListSnapshot({
			key: window.location.pathname + window.location.search,
			posts,
			page,
			done,
			scrollY: window.scrollY,
			activeTab,
			filterMode,
		});
	}, []);

	React.useEffect(() => {
		const onClickCapture = (event: MouseEvent) => {
			if (
				event.defaultPrevented ||
				event.button !== 0 ||
				event.altKey ||
				event.ctrlKey ||
				event.metaKey ||
				event.shiftKey
			) {
				return;
			}
			if (!(event.target instanceof Element)) return;
			const anchor = event.target.closest<HTMLAnchorElement>("a[href]");
			if (!anchor) return;
			const href = anchor.getAttribute("href") ?? "";
			// 進詳情（返回時還原）與底部 Tab 切走（切回時像 App Tab 原地接續，
			// 見 PageTransition 的 TAB_ROOTS）都要存快照；還原端以 key 精確比對
			// 且有 5 分鐘時效，對不上就自動作廢，多存無害。
			const isDetailNav = /^\/(products|sellers)\//.test(href);
			const isTabNav = href === "/" || href === "/market" || href === "/search";
			if (!isDetailNav && !isTabNav) return;
			saveSnapshot();
			setOpenPostId(null);
		};
		document.addEventListener("click", onClickCapture, true);
		return () => document.removeEventListener("click", onClickCapture, true);
	}, [saveSnapshot, setOpenPostId]);

	const visiblePosts = React.useMemo(() => {
		const scoped =
			filterMode !== "followed" || !followedIds
				? posts
				: posts.filter((p) => followedIds.has(String(p.sellerId)));
		// A client deep link (`/feed?post=`) may target a post outside the
		// canonical page; surface it at the top so the viewer can open it and it
		// reads as a normal card behind the viewer, matching the old server seed.
		if (!deepLinkPost) return scoped;
		if (scoped.some((p) => String(p.id) === String(deepLinkPost.id)))
			return scoped;
		return [deepLinkPost, ...scoped];
	}, [posts, filterMode, followedIds, deepLinkPost]);
	const activeIndex = React.useMemo(() => {
		if (!openPostId) return null;
		const idx = visiblePosts.findIndex((p) => String(p.id) === openPostId);
		return idx >= 0 ? idx : null;
	}, [openPostId, visiblePosts]);

	const openAtPost = (post: PostListItem, mediaRect?: DOMRect) => {
		const idx = visiblePosts.findIndex((p) => p.id === post.id);
		if (idx < 0) return;
		saveSnapshot();
		setOpenPostId(post.id);
		setOpenOrigin(mediaRect ? toOriginRect(mediaRect) : null);
	};

	// The viewer animates its own close (shrink back into the card) and calls
	// this once the tween finishes. We refresh the origin to the *currently*
	// active post's card so paging inside the viewer still shrinks to the right
	// place; the lookup happens inside the viewer via `data-feed-card-id`.
	const closeViewer = () => {
		setOpenOrigin(null);
		setOpenPostId(null);
	};

	// Deep-link support: open the viewer on `initialPostId` as soon as
	// the post appears in `visiblePosts`. Fires once per id so the user
	// can close the viewer without it re-opening on every re-render.
	const openedDeepLinkRef = React.useRef<string | null>(null);
	React.useEffect(() => {
		if (!initialPostId) return;
		if (openedDeepLinkRef.current === initialPostId) return;
		const idx = visiblePosts.findIndex((p) => String(p.id) === initialPostId);
		if (idx >= 0) {
			openedDeepLinkRef.current = initialPostId;
			setOpenPostId(initialPostId);
		}
	}, [initialPostId, setOpenPostId, visiblePosts]);

	// Empty-state copy when the followed filter yields no matches.
	// Wait for the follow set to load before showing it so we don't
	// flash "no posts" before the first /subscriptions response.
	const isFollowFilterActive = filterMode === "followed";
	const showFollowEmpty =
		isFollowFilterActive &&
		followingReady &&
		!followingLoading &&
		visiblePosts.length === 0;
	const showInitialRecovery =
		(initialLoading || refreshing || manualRetrying) && posts.length === 0;
	const showInitialError = Boolean(error) && posts.length === 0;
	const showFeedEmpty =
		!showFollowEmpty && !showInitialError && visiblePosts.length === 0;

	return (
		<>
			{hasPendingFresh ? (
				<div
					className="pointer-events-none fixed left-1/2 top-[calc(var(--site-nav-height,72px)+10px)] z-40 -translate-x-1/2"
					aria-live="polite"
				>
					<Button
						className="pointer-events-auto shadow-lg"
						variant="solid"
						color="primary"
						radius="full"
						onClick={applyPendingFresh}
						aria-label={
							pendingNewCount > 0
								? `顯示 ${pendingNewCount} 則新動態並回到頂端`
								: "套用最新動態並回到頂端"
						}
					>
						{pendingNewCount > 0
							? `有 ${pendingNewCount} 則新動態`
							: "動態內容已更新"}
					</Button>
				</div>
			) : null}
			<span
				data-hk-feed-background-refresh={refreshing ? "true" : "false"}
				hidden
			/>
			{showInitialRecovery ? (
				<FeedGridSkeleton />
			) : showInitialError ? (
				<div>
					<EmptyStatePanel
						className="hkui-feed-end hkui-feed-end--empty"
						icon="document"
						title="動態暫時載入不到"
						description="目前畫面會保留，請稍後再試；資料恢復後會直接在這裡更新。"
						iconSize={108}
					/>
					<div className="mt-3 flex justify-center">
						<Button
							variant="bordered"
							color="primary"
							radius="full"
							isLoading={manualRetrying}
							loadingLabel="重新載入動態中…"
							onClick={() => void retryInitialLoad()}
						>
							重新載入動態
						</Button>
					</div>
				</div>
			) : showFollowEmpty ? (
				<EmptyStatePanel
					className="hkui-feed-end hkui-feed-end--empty"
					icon="document"
					title="追蹤的商家最近沒發新動態"
					description={`目前只看到全站前 ${posts.length} 則貼文裡沒有你追蹤的商家。試試「推薦」看看其他人在發什麼。`}
					iconSize={108}
				/>
			) : showFeedEmpty ? (
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
			) : (
				<div className="hkui-feed-grid">
					{visiblePosts.map((p, index) => (
						<PostCardV2
							key={p.id}
							post={p}
							imageLoading={!progressiveImages || index < 4 ? "eager" : "lazy"}
							imagePriority={index === 0}
							onOpen={openAtPost}
						/>
					))}
				</div>
			)}
			{!done && (
				<div className="hkui-feed-sentinel">
					<Button
						variant="bordered"
						color="primary"
						radius="full"
						isLoading={loading || (isFollowFilterActive && followingLoading)}
						loadingLabel="載入動態中…"
						onClick={() => void loadMore()}
					>
						載入更多動態
					</Button>
				</div>
			)}
			{done && visiblePosts.length > 0 && (
				<div className="hkui-feed-end">— 已經到底了 —</div>
			)}
			<PostDetailViewer
				posts={visiblePosts}
				initialIndex={activeIndex}
				onClose={closeViewer}
				hasMore={false}
				openRect={openOrigin}
			/>
		</>
	);
}
