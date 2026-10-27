"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "@/lib/feedback/toast";
import {
	createMarketWatchTarget,
	deleteMarketWatchTarget,
	type MarketOverviewData,
	type MarketOverviewItem,
	type MarketOverviewWindowDays,
	type MarketWatchTargetResult,
} from "@/lib/pricebook/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface MarketWatchlistMutationInput {
	item: MarketOverviewItem;
	nextWatched: boolean;
	windowDays: MarketOverviewWindowDays;
}

interface MarketWatchlistMutationContext {
	snapshots: Array<[readonly unknown[], MarketOverviewData | undefined]>;
}

function sameTarget(
	left: Pick<MarketOverviewItem, "cropCode" | "marketCode">,
	right: Pick<MarketOverviewItem, "cropCode" | "marketCode">,
): boolean {
	return (
		left.cropCode === right.cropCode &&
		(left.marketCode ?? "") === (right.marketCode ?? "")
	);
}

/** A discovery ledger can omit the row market because the whole response is
 * already scoped to one market. A watchlist ledger is deliberately unfiltered,
 * so an omitted row market must continue to mean an all-market target. */
function targetInOverview(
	item: MarketOverviewItem,
	data: MarketOverviewData,
): Pick<MarketOverviewItem, "cropCode" | "marketCode"> {
	return {
		cropCode: item.cropCode,
		marketCode:
			item.marketCode ?? (data.scope === "all" ? data.marketCode : undefined),
	};
}

export function optimisticallyUpdateMarketOverview(
	data: MarketOverviewData | undefined,
	input: MarketWatchlistMutationInput,
): MarketOverviewData | undefined {
	if (!data) return data;
	const optimisticId = `optimistic:${input.item.cropCode}:${input.item.marketCode ?? "all"}`;
	const matchingIndex = data.items.findIndex((item) =>
		sameTarget(targetInOverview(item, data), input.item),
	);

	if (data.scope === "watchlist") {
		if (!input.nextWatched) {
			const items = data.items.filter(
				(item) =>
					item.watchTargetId !== input.item.watchTargetId &&
					!sameTarget(targetInOverview(item, data), input.item),
			);
			if (items.length === data.items.length) return data;
			return {
				...data,
				items,
				totalItems:
					data.totalItems == null
						? undefined
						: Math.max(0, data.totalItems - 1),
			};
		}
		const canInsert = data.windowDays === input.windowDays;
		if (matchingIndex >= 0) {
			return {
				...data,
				items: data.items.map((item, index) =>
					index === matchingIndex
						? { ...item, isWatched: true, watchTargetId: optimisticId }
						: item,
				),
			};
		}
		if (!canInsert) return data;
		return {
			...data,
			items: [
				{
					...input.item,
					isWatched: true,
					watchTargetId: optimisticId,
				},
				...data.items,
			],
			totalItems: data.totalItems == null ? undefined : data.totalItems + 1,
		};
	}

	if (matchingIndex < 0) return data;
	return {
		...data,
		items: data.items.map((item, index) =>
			index === matchingIndex
				? {
						...item,
						isWatched: input.nextWatched,
						watchTargetId: input.nextWatched ? optimisticId : undefined,
					}
				: item,
		),
	};
}

export interface UseMarketWatchlistResult {
	toggle: (
		item: MarketOverviewItem,
		context: {
			windowDays: MarketOverviewWindowDays;
		},
	) => Promise<void>;
	pending: boolean;
	pendingTarget: Pick<MarketOverviewItem, "cropCode" | "marketCode"> | null;
}

/** One-click, optimistic watchlist mutations with a full identity-scoped cache
 * rollback. The server remains authoritative: every settled mutation refetches
 * both the watchlist and discovery ledgers. */
export function useMarketWatchlist(): UseMarketWatchlistResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const overviewRoot = React.useMemo(
		() => queryKeys.pricebook.overviews(identity),
		[identity],
	);
	const mutation = useMutation<
		MarketWatchTargetResult | undefined,
		unknown,
		MarketWatchlistMutationInput,
		MarketWatchlistMutationContext
	>({
		mutationFn: async ({ item, nextWatched }) => {
			if (nextWatched) {
				return createMarketWatchTarget({
					cropCode: item.cropCode,
					marketCode: item.marketCode,
				});
			}
			if (!item.watchTargetId) {
				throw new Error("這筆追蹤缺少識別碼，請重新整理後再試一次");
			}
			await deleteMarketWatchTarget(item.watchTargetId);
			return undefined;
		},
		onMutate: async (input) => {
			await queryClient.cancelQueries({ queryKey: overviewRoot });
			const snapshots = queryClient.getQueriesData<MarketOverviewData>({
				queryKey: overviewRoot,
			});
			queryClient.setQueriesData<MarketOverviewData>(
				{ queryKey: overviewRoot },
				(data) => optimisticallyUpdateMarketOverview(data, input),
			);
			return { snapshots };
		},
		onError: (_error, _input, context) => {
			for (const [key, data] of context?.snapshots ?? []) {
				queryClient.setQueryData(key, data);
			}
		},
		onSuccess: (result, input) => {
			if (!input.nextWatched || !result) return;
			queryClient.setQueriesData<MarketOverviewData>(
				{ queryKey: overviewRoot },
				(data) =>
					data
						? {
								...data,
								items: data.items.map((item) =>
									sameTarget(targetInOverview(item, data), input.item)
										? { ...item, watchTargetId: result.watchTargetId }
										: item,
								),
							}
						: data,
			);
		},
		onSettled: () => {
			void queryClient.invalidateQueries({ queryKey: overviewRoot });
		},
	});

	const toggle = React.useCallback(
		async (
			item: MarketOverviewItem,
			context: {
				windowDays: MarketOverviewWindowDays;
			},
		) => {
			const nextWatched = !item.isWatched;
			try {
				await mutation.mutateAsync({ item, nextWatched, ...context });
				toast.success(
					nextWatched
						? `已將「${item.cropName}」加入追蹤`
						: `已停止追蹤「${item.cropName}」`,
				);
			} catch (error) {
				toast.error(nextWatched ? "無法加入追蹤" : "無法停止追蹤", {
					description:
						error instanceof Error ? error.message : "請稍後再試一次。",
				});
			}
		},
		[mutation],
	);

	return {
		toggle,
		pending: mutation.isPending,
		pendingTarget: mutation.variables
			? {
					cropCode: mutation.variables.item.cropCode,
					marketCode: mutation.variables.item.marketCode,
				}
			: null,
	};
}
