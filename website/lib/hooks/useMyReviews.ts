"use client";

import { useInfiniteQuery, useQueries } from "@tanstack/react-query";
import { useMemo } from "react";
import { accountQueryFn, accountQueryPending } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import { fetchOrder } from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";
import { queryKeys } from "@/lib/query/keys";
import { fetchMyReviews, type MyReview } from "@/lib/reviews/repo";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseMyReviewsResult {
	reviews: MyReview[];
	loading: boolean;
	loadingMore: boolean;
	error: string | null;
	total: number;
	hasMore: boolean;
	loadMore: () => Promise<void>;
}

function needsOrderSnapshot(review: MyReview): boolean {
	return (
		Boolean(review.orderNo) &&
		(!review.skuName || !review.productImageKey) &&
		Boolean(review.orderItemId || review.productId)
	);
}

function findSnapshotItem(order: Order | undefined, review: MyReview) {
	if (!order) return undefined;
	if (review.orderItemId) {
		const byOrderItemId = order.items.find(
			(item) => String(item.orderItemId) === review.orderItemId,
		);
		if (byOrderItemId) return byOrderItemId;
	}
	if (review.productId) {
		const byProductId = order.items.find(
			(item) => String(item.productId) === review.productId,
		);
		if (byProductId) return byProductId;
	}
	return order.items.length === 1 ? order.items[0] : undefined;
}

function mergeOrderSnapshots(
	reviews: MyReview[],
	ordersByNo: Map<string, Order>,
): MyReview[] {
	return reviews.map((review) => {
		if (!needsOrderSnapshot(review)) return review;
		const item = findSnapshotItem(ordersByNo.get(review.orderNo), review);
		if (!item) return review;
		return {
			...review,
			skuName: review.skuName || item.name,
			productImageKey: review.productImageKey || item.image || "",
			price: review.price || item.price,
		};
	});
}

/** Reviews the logged-in buyer has written (GET /app/me/reviews, paged). */
export function useMyReviews(): UseMyReviewsResult {
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const query = useInfiniteQuery({
		queryKey: queryKeys.buyer.myReviews(identity),
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: number }) =>
			fetchMyReviews({ page: pageParam, pageSize: 20 }),
		),
		initialPageParam: 1,
		getNextPageParam: (lastPage) => {
			const loaded = lastPage.page * lastPage.pageSize;
			return loaded < lastPage.total ? lastPage.page + 1 : undefined;
		},
	});
	const pages = query.data?.pages;
	const rawReviews = useMemo(
		(): MyReview[] =>
			status === "signedIn" ? (pages?.flatMap((p) => p.records) ?? []) : [],
		[status, pages],
	);
	const snapshotOrderNos = useMemo(
		() =>
			Array.from(
				new Set(
					rawReviews.filter(needsOrderSnapshot).map((review) => review.orderNo),
				),
			),
		[rawReviews],
	);
	const snapshotQueries = useMemo(
		() =>
			snapshotOrderNos.map((orderNo) => ({
				queryKey: queryKeys.buyer.order(identity, orderNo, {
					scope: "review-snapshot",
				}),
				queryFn: () => fetchOrder(orderNo),
			})),
		[identity, snapshotOrderNos],
	);
	const snapshotResults = useQueries({ queries: snapshotQueries });
	const snapshots = useMemo(
		() => snapshotResults.map((result) => result.data),
		[snapshotResults],
	);
	const ordersByNo = useMemo(() => {
		const map = new Map<string, Order>();
		for (const order of snapshots) {
			if (order?.orderNo) map.set(order.orderNo, order);
		}
		return map;
	}, [snapshots]);
	const reviews = useMemo(
		() => mergeOrderSnapshots(rawReviews, ordersByNo),
		[ordersByNo, rawReviews],
	);
	return {
		reviews,
		loading: accountQueryPending(status, query.isLoading),
		loadingMore: query.isFetchingNextPage,
		error: query.error instanceof Error ? query.error.message : null,
		total: pages?.[0]?.total ?? 0,
		hasMore: Boolean(query.hasNextPage),
		loadMore: async () => {
			if (!query.isFetchingNextPage && query.hasNextPage) {
				await query.fetchNextPage();
			}
		},
	};
}
