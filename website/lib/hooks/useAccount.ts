"use client";

import type {
	Favorite,
	Follow,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import {
	type InfiniteData,
	type UseInfiniteQueryResult,
	useInfiniteQuery,
	useMutation,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import { useMemo } from "react";
import { accountQueryFn, accountQueryPending } from "@/lib/auth/account-query";
import { useAuth } from "@/lib/auth/context";
import type { AuthStatus } from "@/lib/auth/session-state";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import {
	type CouponItem,
	claimCoupon,
	fetchAvailableCoupons,
	fetchMyCoupons,
	type MyCouponItem,
} from "@/lib/coupons/repo";
import { fetchFavorites, removeFavorite } from "@/lib/favorites/repo";
import { fetchFollowing, setSellerFollow } from "@/lib/follow/repo";
import { fetchMemberCards } from "@/lib/member-cards/repo";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	fetchProfile,
	type ProfileEdit,
	updateBuyerAvatar,
	updateProfile,
} from "@/lib/profile/repo";
import { queryKeys } from "@/lib/query/keys";
import {
	clearHistory,
	fetchHistory,
	type HistoryItem,
} from "@/lib/visits/repo";
import { errorMessage } from "./queryError";
import { favoriteIdKey } from "./useFavoriteIds";
import { useQueryIdentity } from "./useQueryIdentity";

function useBuyerQueryScope() {
	const { activeRole } = useAuth();
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	return {
		identity,
		activeRole,
		status,
		scope: { activeRole: activeRole ?? "" },
	};
}

/**
 * Load-more controls shared by the buyer cursor lists. A tap while the next
 * page is in flight is ignored (`cancelRefetch: false`), so double taps never
 * request the same page twice.
 */
function cursorListControls(
	status: AuthStatus,
	query: Pick<
		UseInfiniteQueryResult,
		| "hasNextPage"
		| "isFetchingNextPage"
		| "isLoading"
		| "fetchNextPage"
		| "refetch"
	>,
) {
	return {
		loadMore: async () => {
			if (!query.hasNextPage || query.isFetchingNextPage) return;
			await query.fetchNextPage({ cancelRefetch: false });
		},
		loading: accountQueryPending(status, query.isLoading),
		refresh: async () => {
			await query.refetch();
		},
	};
}

/* ===== Coupons ===== */

interface CouponState {
	my: MyCouponItem[];
	available: CouponItem[];
}

const EMPTY_COUPONS: CouponState = { my: [], available: [] };

export function useCoupons() {
	const { identity, status, scope } = useBuyerQueryScope();
	const signedIn = status === "signedIn";
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.coupons(identity, scope);
	const query = useQuery({
		queryKey,
		queryFn: accountQueryFn(status, async (): Promise<CouponState> => {
			const [my, available] = await Promise.all([
				fetchMyCoupons({ pageSize: 50 }),
				fetchAvailableCoupons({ pageSize: 50 })
					.then((r) => r.records)
					.catch(() => []),
			]);
			return { my, available };
		}),
	});
	const claimMutation = useMutation({
		mutationFn: (id: number | string) => claimCoupon(id),
		onSuccess: () => {
			void queryClient.invalidateQueries({ queryKey });
		},
	});

	const my = useMemo(
		() => (signedIn && query.data?.my) || [],
		[signedIn, query.data?.my],
	);
	const available = useMemo(
		() => (signedIn && query.data?.available) || [],
		[signedIn, query.data?.available],
	);

	return {
		my,
		available,
		loading: accountQueryPending(status, query.isPending),
		error: errorMessage(query.error, "載入優惠券失敗"),
		refresh: async () => {
			await query.refetch();
		},
		claim: async (id: number | string) => {
			await claimMutation.mutateAsync(id);
		},
	};
}

/* ===== Favorites ===== */

export function useFavorites(options: { enabled?: boolean } = {}) {
	const { identity, status, scope } = useBuyerQueryScope();
	const signedIn = status === "signedIn";
	const enabled = options.enabled ?? true;
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.favorites(identity, scope);
	const query = useInfiniteQuery({
		queryKey,
		enabled,
		initialPageParam: "",
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: string }) =>
			fetchFavorites({ pageSize: 50, pageToken: pageParam }),
		),
		getNextPageParam: (last) => last.nextPageToken || undefined,
	});
	const removeMutation = useMutation({
		mutationFn: (productId: number | string) => removeFavorite(productId),
		// Keep the initiating row mounted while the request is pending so its
		// Button can expose the shared branded loading state. Remove it only after
		// the backend confirms success; a failure therefore needs no visual rollback.
		onSuccess: async (_result, productId) => {
			await queryClient.cancelQueries({ queryKey });
			queryClient.setQueryData<
				InfiniteData<{ items: Favorite[]; nextPageToken: string }, string>
			>(
				queryKey,
				(current) =>
					current && {
						...current,
						pages: current.pages.map((page) => ({
							...page,
							items: page.items.filter(
								(it) => it.product?.id !== String(productId),
							),
						})),
					},
			);
			queryClient.setQueryData(favoriteIdKey(identity, productId), false);
		},
	});

	const remove = (productId: number | string) =>
		removeMutation.mutateAsync(productId);

	const items = useMemo(
		() => (signedIn && query.data?.pages.flatMap((page) => page.items)) || [],
		[signedIn, query.data],
	);

	return {
		items,
		hasMore: signedIn && Boolean(query.hasNextPage),
		...cursorListControls(status, query),
		error: errorMessage(query.error ?? removeMutation.error, "更新收藏失敗"),
		remove,
	};
}

/* ===== Follow ===== */

export function useFollowing(options: { enabled?: boolean } = {}) {
	const { identity, status, scope } = useBuyerQueryScope();
	const signedIn = status === "signedIn";
	const enabled = options.enabled ?? true;
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.following(identity, scope);
	const query = useInfiniteQuery({
		queryKey,
		enabled,
		initialPageParam: "",
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: string }) =>
			fetchFollowing({ size: 50, pageToken: pageParam }),
		),
		getNextPageParam: (last) => last.nextPageToken || undefined,
	});
	const unfollowMutation = useMutation({
		mutationFn: ({ id }: { id: number | string }) => setSellerFollow(id, false),
		// As with favourites, retain the row until success so the initiating
		// control remains visible and can be locked against duplicate requests.
		onSuccess: async (_result, { id }) => {
			await queryClient.cancelQueries({ queryKey });
			queryClient.setQueryData<
				InfiniteData<{ items: Follow[]; nextPageToken: string }, string>
			>(
				queryKey,
				(current) =>
					current && {
						...current,
						pages: current.pages.map((page) => ({
							...page,
							items: page.items.filter((it) => it.storeId !== String(id)),
						})),
					},
			);
			queryClient.setQueryData(
				queryKeys.buyer.subscription(identity, id),
				false,
			);
		},
	});

	const unfollow = (id: number | string) =>
		unfollowMutation.mutateAsync({ id });

	const items = useMemo(
		() => (signedIn && query.data?.pages.flatMap((page) => page.items)) || [],
		[signedIn, query.data],
	);

	return {
		items,
		hasMore: signedIn && Boolean(query.hasNextPage),
		...cursorListControls(status, query),
		error: errorMessage(query.error ?? unfollowMutation.error, "更新追蹤失敗"),
		unfollow,
	};
}

/* ===== History ===== */

export function useHistory() {
	const { identity, status, scope } = useBuyerQueryScope();
	const signedIn = status === "signedIn";
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.history(identity, scope);
	const query = useInfiniteQuery({
		queryKey,
		initialPageParam: "",
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: string }) =>
			fetchHistory({ size: 50, pageToken: pageParam }),
		),
		getNextPageParam: (last) => last.nextPageToken || undefined,
	});
	const clearMutation = useMutation({
		mutationFn: () => clearHistory(),
		onMutate: async () => {
			await queryClient.cancelQueries({ queryKey });
			const previous =
				queryClient.getQueryData<
					InfiniteData<
						{
							items: HistoryItem[];
							nextPageToken: string;
						},
						string
					>
				>(queryKey);
			queryClient.setQueryData(queryKey, {
				pages: [{ items: [], nextPageToken: "" }],
				pageParams: [""],
			});
			return { previous };
		},
		onError: (_e, _v, ctx) => {
			if (ctx) queryClient.setQueryData(queryKey, ctx.previous);
		},
	});

	const clear = () => clearMutation.mutateAsync();

	const items = useMemo(
		() => (signedIn && query.data?.pages.flatMap((page) => page.items)) || [],
		[signedIn, query.data],
	);

	return {
		items,
		hasMore: signedIn && Boolean(query.hasNextPage),
		...cursorListControls(status, query),
		error: errorMessage(query.error, "載入瀏覽紀錄失敗"),
		clear,
	};
}

/* ===== Member cards (會員卡) ===== */

export function useMemberCards() {
	const { identity, status, scope } = useBuyerQueryScope();
	const signedIn = status === "signedIn";
	const queryKey = queryKeys.buyer.memberCards(identity, scope);
	const query = useInfiniteQuery({
		queryKey,
		initialPageParam: "",
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: string }) =>
			fetchMemberCards({ size: 50, pageToken: pageParam }),
		),
		getNextPageParam: (last) => last.nextPageToken || undefined,
	});

	const cards = useMemo(
		() => (signedIn && query.data?.pages.flatMap((page) => page.records)) || [],
		[signedIn, query.data],
	);

	return {
		cards,
		hasMore: signedIn && Boolean(query.hasNextPage),
		...cursorListControls(status, query),
		error: errorMessage(query.error, "載入會員卡失敗"),
	};
}

/* ===== Profile ===== */

/**
 * `enabled` lets a caller that doesn't always need the buyer profile skip the
 * `/app/buyers/me` request entirely (a seller-only account 403s that endpoint).
 * Defaults to `true` so existing callers are unaffected.
 *
 * The header's nickname and avatar come from the server's session answer, so every
 * successful profile change (`save`, `updateAvatar`) asks the server again via
 * `router.refresh()`.
 */
export function useProfile(options: { enabled?: boolean } = {}) {
	const { identity, status } = useBuyerQueryScope();
	const signedIn = status === "signedIn";
	const enabled = options.enabled ?? true;
	const queryClient = useQueryClient();
	const router = useObservedRouter("profile_refresh");
	const queryKey = queryKeys.buyer.profile(identity);
	const query = useQuery({
		queryKey,
		enabled,
		queryFn: accountQueryFn(status, () => fetchProfile()),
	});
	const syncHeader = () => router.refresh();
	const saveMutation = useMutation({
		mutationFn: (body: ProfileEdit) => updateProfile(body, query.data ?? {}),
		onSuccess: () => {
			void queryClient.invalidateQueries({ queryKey });
			syncHeader();
		},
	});
	const updateAvatar = async (stagedToken: string) => {
		await updateBuyerAvatar(stagedToken, query.data ?? {});
		syncHeader();
	};

	return {
		profile: signedIn ? (query.data ?? null) : null,
		loading: accountQueryPending(status, query.isLoading),
		saving: saveMutation.isPending,
		error: errorMessage(query.error ?? saveMutation.error, "載入個人資料失敗"),
		refresh: async () => {
			await query.refetch();
		},
		save: async (body: ProfileEdit) => {
			await saveMutation.mutateAsync(body);
		},
		updateAvatar,
	};
}
