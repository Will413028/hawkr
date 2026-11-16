"use client";

import {
	type InfiniteData,
	useInfiniteQuery,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";
import { fetchCanonicalFeedPage } from "@/lib/api/main-tab-pages";
import type { PostListItem } from "@/lib/api/openapi-types";
import { useAuth } from "@/lib/auth/context";
import {
	type FeedHeadPage,
	feedHeadRequiresReplacement,
	mergeFeedHeadPages,
} from "@/lib/feed/headRefresh";
import type { FeedListSnapshot } from "@/lib/feed/listSnapshot";
import { useListFreshnessScope } from "@/lib/hooks/useListFreshnessScope";
import { useMainTabActivationRefresh } from "@/lib/hooks/useMainTabActivationRefresh";
import {
	MAIN_TAB_FULL_RECONCILE_MS,
	MAIN_TAB_STALE_MS,
} from "@/lib/navigation/main-tab-freshness";
import { fetchPostsPage } from "@/lib/posts/repo";
import { queryKeys } from "@/lib/query/keys";
import { readMainTabRuntime } from "@/lib/state/main-tab-freshness-store";

type FeedPostsPage = FeedHeadPage;
/** 頁碼只是畫面上的計數；真正續讀靠 page_token（伺服器沒有頁碼與總數）。 */
interface FeedPageParam {
	page: number;
	token: string;
}
type FeedQueryData = InfiniteData<FeedPostsPage, FeedPageParam>;

interface PendingFeedRefresh {
	data: FeedQueryData;
	newCount: number;
	generation: string;
	queryToken: object;
}

interface UseFeedPostsOptions {
	initialPosts: PostListItem[];
	initialPage?: number;
	/** The page_token after the server-rendered seed page; empty or omitted
	 * means the seed is the whole list. */
	initialNextPageToken?: string;
	pageSize: number;
	sellerId?: number | string;
	hashtag?: string;
	/** Omits synthetic empty initialData after a failed server seed so TanStack
	 * performs page one's client recovery request immediately. */
	initialSeedFailed?: boolean;
	/** Declares that `initialPosts` was fetched server-side for *this* scope
	 * (seller / hashtag), not for the canonical feed. Scoped routes that render
	 * their own page one (e.g. the seller posts tab) set it so the seed is
	 * trusted instead of being refetched on the client. */
	initialPostsMatchQuery?: boolean;
	restoredSnapshot?: FeedListSnapshot | null;
}

function dedupePosts(pages: FeedPostsPage[]) {
	const seen = new Set<string>();
	const posts: PostListItem[] = [];
	for (const page of pages) {
		for (const post of page.records) {
			if (seen.has(post.id)) continue;
			seen.add(post.id);
			posts.push(post);
		}
	}
	return posts;
}

/** 要重新對帳的頁數：1..N，N 是目前載入到的最後一頁（回上一頁還原時整串存成一頁，`page` 記最後一頁）。 */
function feedPageCountToReconcile(pages: FeedPostsPage[]): number {
	const onlyPage = pages.length === 1 ? pages[0] : undefined;
	if (onlyPage && onlyPage.page > 1) return onlyPage.page;
	return Math.max(1, ...pages.map((page) => page.page));
}

function supersededRefreshError() {
	return new DOMException("tab query changed", "AbortError");
}

export function useFeedPosts({
	initialPosts,
	initialPage = 1,
	initialNextPageToken = "",
	pageSize,
	sellerId,
	hashtag,
	initialSeedFailed = false,
	initialPostsMatchQuery = false,
	restoredSnapshot,
}: UseFeedPostsOptions) {
	// 同時帶店家與標籤時伺服器不接受（invalid_argument）：店家優先，標籤不套用。
	const scopedHashtag = sellerId ? undefined : hashtag;
	const isCanonical = !sellerId && !scopedHashtag && initialPage === 1;
	const queryClient = useQueryClient();
	// 登入者的列表依觀看者而定（封鎖的店不出現、viewer_* 欄位），不能用匿名的種子或匿名的共用答案：
	// 登入後換成自己的 query key，由瀏覽器帶 cookie 重新讀。auth 還沒確定（unknown）先當訪客。
	const viewer = useAuth().status === "signedIn";
	const queryKey = React.useMemo(
		() =>
			queryKeys.feed.posts({
				sellerId: sellerId ? String(sellerId) : "",
				hashtag: scopedHashtag ?? "",
				pageSize,
				viewer,
			}),
		[pageSize, scopedHashtag, sellerId, viewer],
	);
	const queryGeneration = React.useMemo(
		() => JSON.stringify(queryKey),
		[queryKey],
	);
	const restoredGenerationRef = React.useRef(queryGeneration);
	const restored =
		restoredGenerationRef.current === queryGeneration
			? (restoredSnapshot ?? null)
			: null;
	// A server seed is usable only when it was fetched for this very query. The
	// canonical (unscoped) feed always is: under ISR the route server-renders
	// canonical page 1 and a seller/hashtag scope is applied on the client, so a
	// scoped query there must fetch fresh page 1 rather than seed from posts
	// belonging to a different key. A route that server-renders its own scoped
	// page one instead declares `initialPostsMatchQuery`; without that, the seed
	// would be dropped and page one refetched on the client — replacing content
	// the server already delivered with a full-page loading skeleton. A
	// back-navigation snapshot still seeds any scope.
	const seedMatchesQuery = isCanonical || initialPostsMatchQuery;
	const hasInitialData =
		restored !== null || (!initialSeedFailed && seedMatchesQuery && !viewer);
	const seedPage = restored?.page ?? initialPage;
	const seedPosts = restored?.posts ?? initialPosts;
	const seedToken = restored?.nextPageToken ?? initialNextPageToken;
	const seedDone = restored?.done ?? seedToken === "";
	const queryGenerationRef = React.useRef(queryGeneration);
	queryGenerationRef.current = queryGeneration;
	const freshnessScope = useListFreshnessScope(queryKey);
	const seedTimestampRef = React.useRef({
		generation: queryGeneration,
		updatedAt: restored?.savedAt ?? Date.now(),
	});
	if (seedTimestampRef.current.generation !== queryGeneration) {
		seedTimestampRef.current = {
			generation: queryGeneration,
			updatedAt: restored?.savedAt ?? Date.now(),
		};
	}
	const initialDataUpdatedAt =
		restored?.savedAt ?? seedTimestampRef.current.updatedAt;
	const fetchPage = React.useCallback(
		async (requestedPage: number, token: string, signal?: AbortSignal) => {
			// 匿名的 canonical 第一頁走同源的 tab 路由；登入者與有範圍的查詢由瀏覽器直接讀。
			const response =
				isCanonical && !viewer
					? await fetchCanonicalFeedPage(token, signal)
					: await fetchPostsPage({
							pageToken: token,
							pageSize,
							storeId: sellerId ? String(sellerId) : undefined,
							tag: scopedHashtag,
							signal,
							strict: true,
							viewer,
						});
			return {
				records: response.records,
				page: requestedPage,
				pageSize: response.pageSize,
				hasMore: response.nextPageToken !== "",
				nextPageToken: response.nextPageToken,
			} satisfies FeedPostsPage;
		},
		[isCanonical, pageSize, scopedHashtag, sellerId, viewer],
	);

	const query = useInfiniteQuery({
		queryKey,
		queryFn: ({ pageParam, signal }) =>
			fetchPage(pageParam.page, pageParam.token, signal),
		// 沒用上種子（登入者、種子讀取失敗）時從第一頁開始：種子的 token 是它那一頁之後的，
		// 拿來讀會跳過第一頁。
		initialPageParam: {
			page: hasInitialData ? seedPage : initialPage,
			token: hasInitialData ? seedToken : "",
		} as FeedPageParam,
		initialData: hasInitialData
			? {
					pages: [
						{
							records: seedPosts,
							page: seedPage,
							pageSize,
							hasMore: !seedDone,
							nextPageToken: seedToken,
						},
					],
					pageParams: [{ page: seedPage, token: seedToken }],
				}
			: undefined,
		initialDataUpdatedAt: hasInitialData ? initialDataUpdatedAt : undefined,
		getNextPageParam: (lastPage) =>
			lastPage.hasMore
				? { page: lastPage.page + 1, token: lastPage.nextPageToken }
				: undefined,
		// Infinite-query refetch replays every loaded page in TanStack v5. Main
		// tab activation instead reconciles only page 1 below, preserving the
		// reader's accumulated pages and scroll position.
		staleTime: Number.POSITIVE_INFINITY,
		gcTime: 30 * 60 * 1000,
		refetchOnMount: false,
	});
	// Head refresh, occasional full reconciliation and load-more all write the
	// same InfiniteData object. One per-hook queue prevents a late page append
	// from restoring the pre-refresh page-one snapshot.
	const listWorkQueueRef = React.useRef<Promise<void>>(Promise.resolve());
	const enqueueListWork = React.useCallback(
		(work: () => Promise<void>): Promise<void> => {
			const run = listWorkQueueRef.current.catch(() => undefined).then(work);
			listWorkQueueRef.current = run.catch(() => undefined);
			return run;
		},
		[],
	);
	const [pendingRefresh, setPendingRefresh] =
		React.useState<PendingFeedRefresh | null>(null);
	const pendingRefreshRef = React.useRef<PendingFeedRefresh | null>(null);
	const commitPendingRefresh = React.useCallback(
		(next: PendingFeedRefresh | null) => {
			pendingRefreshRef.current = next;
			setPendingRefresh(next);
		},
		[],
	);
	const activePendingRefresh =
		pendingRefresh?.generation === queryGeneration ? pendingRefresh : null;
	React.useEffect(() => {
		if (
			pendingRefreshRef.current &&
			pendingRefreshRef.current.generation !== queryGeneration
		) {
			commitPendingRefresh(null);
		}
	}, [commitPendingRefresh, queryGeneration]);
	const refreshHead = React.useCallback(
		(signal: AbortSignal, onlyIfStale = false) => {
			const expectedGeneration = queryGeneration;
			return enqueueListWork(async () => {
				const queryToken = queryClient
					.getQueryCache()
					.find({ queryKey, exact: true });
				if (
					!queryToken ||
					signal.aborted ||
					queryGenerationRef.current !== expectedGeneration
				) {
					throw signal.reason ?? supersededRefreshError();
				}
				// Full reconciliation runs first when both windows expired. Check
				// again inside the shared queue so its success avoids a second page 1.
				if (
					onlyIfStale &&
					Date.now() -
						(queryClient.getQueryState(queryKey)?.dataUpdatedAt ?? 0) <
						MAIN_TAB_STALE_MS.feed
				) {
					return;
				}
				const freshPage = await fetchPage(1, "", signal);
				if (
					signal.aborted ||
					queryGenerationRef.current !== expectedGeneration ||
					queryClient.getQueryCache().find({ queryKey, exact: true }) !==
						queryToken
				) {
					throw supersededRefreshError();
				}
				const candidate: FeedQueryData = {
					pages: [freshPage],
					pageParams: [{ page: 1, token: "" }],
				};
				const autoApply =
					!restored && typeof window !== "undefined" && window.scrollY <= 120;
				let requiresReplacement = false;
				let newCount = 0;
				queryClient.setQueryData<FeedQueryData>(queryKey, (current) => {
					if (!current) return candidate;
					const first = current.pages[0];
					requiresReplacement = feedHeadRequiresReplacement(
						first?.records ?? [],
						freshPage.records,
					);
					const existingIds = new Set(
						current.pages.flatMap((page) =>
							page.records.map((record) => String(record.id)),
						),
					);
					newCount = freshPage.records.filter(
						(record) => !existingIds.has(String(record.id)),
					).length;
					if (autoApply) return candidate;
					return {
						...current,
						pages: mergeFeedHeadPages(current.pages, freshPage.records, false)
							.pages,
					};
				});
				commitPendingRefresh(
					!autoApply && requiresReplacement
						? {
								data: candidate,
								newCount,
								generation: expectedGeneration,
								queryToken,
							}
						: null,
				);
			});
		},
		[
			commitPendingRefresh,
			enqueueListWork,
			fetchPage,
			queryClient,
			queryGeneration,
			queryKey,
			restored,
		],
	);
	const refreshAllPages = React.useCallback(
		(signal: AbortSignal) => {
			const expectedGeneration = queryGeneration;
			return enqueueListWork(async () => {
				const queryToken = queryClient
					.getQueryCache()
					.find({ queryKey, exact: true });
				if (
					!queryToken ||
					signal.aborted ||
					queryGenerationRef.current !== expectedGeneration
				) {
					throw signal.reason ?? supersededRefreshError();
				}
				const current = queryClient.getQueryData<FeedQueryData>(queryKey);
				if (!current || current.pages.length === 0) return;
				const freshPages: FeedPostsPage[] = [];
				// page_token 要用上一頁回的那一個，所以 1..N 依序讀。
				let token = "";
				const pageCount = feedPageCountToReconcile(current.pages);
				for (let page = 1; page <= pageCount; page += 1) {
					if (
						signal.aborted ||
						queryGenerationRef.current !== expectedGeneration ||
						queryClient.getQueryCache().find({ queryKey, exact: true }) !==
							queryToken
					) {
						throw signal.reason ?? supersededRefreshError();
					}
					const fresh = await fetchPage(page, token, signal);
					freshPages.push(fresh);
					if (!fresh.hasMore) break;
					token = fresh.nextPageToken;
				}
				if (
					signal.aborted ||
					queryGenerationRef.current !== expectedGeneration ||
					queryClient.getQueryCache().find({ queryKey, exact: true }) !==
						queryToken
				) {
					throw supersededRefreshError();
				}
				const candidate: FeedQueryData = {
					pages: freshPages,
					pageParams: freshPages.map((fresh, index) => ({
						page: fresh.page,
						token:
							index === 0 ? "" : (freshPages[index - 1]?.nextPageToken ?? ""),
					})),
				};
				const requiresReplacement =
					current.pages.length !== freshPages.length ||
					current.pages.some((page, index) =>
						feedHeadRequiresReplacement(
							page.records,
							freshPages[index]?.records ?? [],
						),
					);
				const existingIds = new Set(
					current.pages.flatMap((page) =>
						page.records.map((record) => String(record.id)),
					),
				);
				const newCount = freshPages
					.flatMap((page) => page.records)
					.filter((record) => !existingIds.has(String(record.id))).length;
				const autoApply =
					!restored && typeof window !== "undefined" && window.scrollY <= 120;
				if (autoApply) {
					queryClient.setQueryData(queryKey, candidate);
					commitPendingRefresh(null);
					return;
				}
				queryClient.setQueryData<FeedQueryData>(queryKey, {
					...current,
					pages: mergeFeedHeadPages(
						current.pages,
						freshPages.flatMap((page) => page.records),
						false,
					).pages,
				});
				commitPendingRefresh(
					requiresReplacement
						? {
								data: candidate,
								newCount,
								generation: expectedGeneration,
								queryToken,
							}
						: null,
				);
			});
		},
		[
			commitPendingRefresh,
			enqueueListWork,
			fetchPage,
			queryClient,
			queryGeneration,
			queryKey,
			restored,
		],
	);
	const fullActivationRefresh = useMainTabActivationRefresh({
		scope: `${freshnessScope}:full`,
		// Seed a scope only once. A remount or head update must not postpone
		// the independent deadline for reconciling the already-loaded tail.
		freshAt:
			readMainTabRuntime(`${freshnessScope}:full`).lastSuccessAt ||
			query.dataUpdatedAt,
		staleMs: MAIN_TAB_FULL_RECONCILE_MS,
		enabled: initialPage === 1 && query.data !== undefined,
		refresh: refreshAllPages,
	});
	const activationRefresh = useMainTabActivationRefresh({
		scope: freshnessScope,
		freshAt: query.dataUpdatedAt,
		staleMs: MAIN_TAB_STALE_MS.feed,
		enabled: initialPage === 1 && query.data !== undefined,
		refresh: (signal) => refreshHead(signal, true),
	});
	const applyPendingFresh = React.useCallback(() => {
		if (!activePendingRefresh) return;
		void enqueueListWork(async () => {
			const latest = pendingRefreshRef.current;
			if (
				!latest ||
				latest.generation !== queryGeneration ||
				queryGenerationRef.current !== queryGeneration ||
				queryClient.getQueryCache().find({ queryKey, exact: true }) !==
					latest.queryToken
			) {
				return;
			}
			queryClient.setQueryData(queryKey, latest.data);
			commitPendingRefresh(null);
			window.scrollTo({ top: 0, behavior: "auto" });
		});
	}, [
		activePendingRefresh,
		commitPendingRefresh,
		enqueueListWork,
		queryClient,
		queryGeneration,
		queryKey,
	]);

	const pages = React.useMemo(
		() => query.data?.pages ?? [],
		[query.data?.pages],
	);
	const posts = React.useMemo(() => dedupePosts(pages), [pages]);
	const lastPage = pages.at(-1);
	const page = lastPage?.page ?? seedPage;
	const nextPageToken = lastPage?.nextPageToken ?? seedToken;
	const refresh = React.useCallback(async () => {
		const controller = new AbortController();
		await refreshHead(controller.signal);
	}, [refreshHead]);
	const loadMore = React.useCallback(async () => {
		await enqueueListWork(async () => {
			if (!query.isFetchingNextPage && query.hasNextPage) {
				await query.fetchNextPage();
			}
		});
	}, [
		enqueueListWork,
		query.fetchNextPage,
		query.hasNextPage,
		query.isFetchingNextPage,
	]);

	return {
		posts,
		page,
		nextPageToken,
		initialLoading: query.isPending,
		loading: query.isFetchingNextPage,
		refreshing:
			activationRefresh.refreshing || fullActivationRefresh.refreshing,
		done: !query.hasNextPage,
		error: query.error instanceof Error ? query.error.message : null,
		hasPendingFresh: activePendingRefresh !== null,
		pendingNewCount: activePendingRefresh?.newCount ?? 0,
		applyPendingFresh,
		refresh,
		loadMore,
	};
}
