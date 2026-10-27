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
import { useAuth } from "@/lib/auth/context";
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
import {
	fetchProfile,
	type ProfileEdit,
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
	const { hydrated, activeRole, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	return {
		identity,
		activeRole,
		enabled: hydrated && isLoggedIn,
		scope: { activeRole: activeRole ?? "" },
	};
}

/**
 * Load-more controls shared by the buyer cursor lists. A tap while the next
 * page is in flight is ignored (`cancelRefetch: false`), so double taps never
 * request the same page twice.
 */
function cursorListControls(
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
		loading: query.isLoading,
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
	const { identity, enabled, scope } = useBuyerQueryScope();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.coupons(identity, scope);
	const query = useQuery({
		queryKey,
		enabled,
		placeholderData: EMPTY_COUPONS,
		queryFn: async (): Promise<CouponState> => {
			const [my, available] = await Promise.all([
				fetchMyCoupons({ pageSize: 50 }),
				fetchAvailableCoupons({ pageSize: 50 })
					.then((r) => r.records)
					.catch(() => []),
			]);
			return { my, available };
		},
	});
	const claimMutation = useMutation({
		mutationFn: (id: number | string) => claimCoupon(id),
		onSuccess: () => {
			void queryClient.invalidateQueries({ queryKey });
		},
	});

	return {
		my: enabled ? (query.data?.my ?? []) : [],
		available: enabled ? (query.data?.available ?? []) : [],
		loading: query.isLoading,
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
	const { identity, enabled: scopeEnabled, scope } = useBuyerQueryScope();
	const enabled = scopeEnabled && (options.enabled ?? true);
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.favorites(identity, scope);
	const query = useInfiniteQuery({
		queryKey,
		enabled,
		initialPageParam: "",
		queryFn: ({ pageParam }) =>
			fetchFavorites({ pageSize: 50, pageToken: pageParam }),
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

	return {
		items: enabled
			? (query.data?.pages.flatMap((page) => page.items) ?? [])
			: [],
		hasMore: enabled && Boolean(query.hasNextPage),
		...cursorListControls(query),
		error: errorMessage(query.error ?? removeMutation.error, "更新收藏失敗"),
		remove,
	};
}

/* ===== Follow ===== */

export function useFollowing(options: { enabled?: boolean } = {}) {
	const { identity, enabled: scopeEnabled, scope } = useBuyerQueryScope();
	const enabled = scopeEnabled && (options.enabled ?? true);
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.following(identity, scope);
	const query = useInfiniteQuery({
		queryKey,
		enabled,
		initialPageParam: "",
		queryFn: ({ pageParam }) =>
			fetchFollowing({ size: 50, pageToken: pageParam }),
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

	return {
		items: enabled
			? (query.data?.pages.flatMap((page) => page.items) ?? [])
			: [],
		hasMore: enabled && Boolean(query.hasNextPage),
		...cursorListControls(query),
		error: errorMessage(query.error ?? unfollowMutation.error, "更新追蹤失敗"),
		unfollow,
	};
}

/* ===== History ===== */

export function useHistory() {
	const { identity, enabled, scope } = useBuyerQueryScope();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.history(identity, scope);
	const query = useInfiniteQuery({
		queryKey,
		enabled,
		initialPageParam: "",
		queryFn: ({ pageParam }) =>
			fetchHistory({ size: 50, pageToken: pageParam }),
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

	return {
		items: enabled
			? (query.data?.pages.flatMap((page) => page.items) ?? [])
			: [],
		hasMore: enabled && Boolean(query.hasNextPage),
		...cursorListControls(query),
		error: errorMessage(query.error, "載入瀏覽紀錄失敗"),
		clear,
	};
}

/* ===== Member cards (會員卡) ===== */

export function useMemberCards() {
	const { identity, enabled, scope } = useBuyerQueryScope();
	const queryKey = queryKeys.buyer.memberCards(identity, scope);
	const query = useInfiniteQuery({
		queryKey,
		enabled,
		initialPageParam: "",
		queryFn: ({ pageParam }) =>
			fetchMemberCards({ size: 50, pageToken: pageParam }),
		getNextPageParam: (last) => last.nextPageToken || undefined,
	});

	return {
		cards: enabled
			? (query.data?.pages.flatMap((page) => page.records) ?? [])
			: [],
		hasMore: enabled && Boolean(query.hasNextPage),
		...cursorListControls(query),
		error: errorMessage(query.error, "載入會員卡失敗"),
	};
}

/* ===== Profile ===== */

/**
 * `enabled` lets a caller that doesn't always need the buyer profile skip the
 * `/app/buyers/me` request entirely — mirrors the active-role gate in
 * `lib/api/auth.ts` (background nickname/avatar backfill only runs when
 * `activeRole === "buyer"`, since a seller-only account 403s that endpoint).
 * Defaults to `true` so existing callers are unaffected.
 */
export function useProfile(options: { enabled?: boolean } = {}) {
	const { identity, enabled: scopeEnabled } = useBuyerQueryScope();
	const enabled = scopeEnabled && (options.enabled ?? true);
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.profile(identity);
	const query = useQuery({
		queryKey,
		enabled,
		queryFn: () => fetchProfile(),
	});
	const saveMutation = useMutation({
		mutationFn: (body: ProfileEdit) => updateProfile(body, query.data ?? {}),
		onSuccess: () => {
			void queryClient.invalidateQueries({ queryKey });
		},
	});

	return {
		profile: enabled ? (query.data ?? null) : null,
		loading: query.isLoading,
		saving: saveMutation.isPending,
		error: errorMessage(query.error ?? saveMutation.error, "載入個人資料失敗"),
		refresh: async () => {
			await query.refetch();
		},
		save: async (body: ProfileEdit) => {
			await saveMutation.mutateAsync(body);
		},
	};
}
