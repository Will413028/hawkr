"use client";

import {
	type QueryObserverResult,
	useQueries,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { createMembershipBatcher } from "@/lib/connect/batch-membership";
import { batchCheckFollowedStoreIds } from "@/lib/follow/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

const checkVisibleFollows = createMembershipBatcher(batchCheckFollowedStoreIds);

export interface FollowedSellersState {
	ids: Set<string>;
	loading: boolean;
	error: string | null;
	ready: boolean;
}

/**
 * Check only sellers in the loaded feed pages. Each seller is its own query
 * (shared with `useIsFollowedSeller`), so a new feed page checks only its new
 * sellers and the already-checked ones keep their answer.
 */
export function useFollowedSellers(
	enabled: boolean,
	sellerIds: Array<string | number>,
): FollowedSellersState {
	const { isLoggedIn, activeRole, hydrated } = useAuth();
	const identity = useQueryIdentity();
	// Keyed by content, not by array identity: callers derive `sellerIds` from
	// their posts on every render, and a new `ids` would re-run `combine` and
	// hand back a new `ids` Set, re-running everything downstream of it.
	const idsKey = [...new Set(sellerIds.map(String).filter(Boolean))].join(",");
	const ids = React.useMemo(() => (idsKey ? idsKey.split(",") : []), [idsKey]);
	const shouldFetch =
		enabled && hydrated && isLoggedIn && activeRole === "buyer";
	const scope = `${identity.accountId}:${identity.activeRole}`;
	const combine = React.useCallback(
		(results: Array<QueryObserverResult<boolean>>): FollowedSellersState => {
			const followed = new Set<string>();
			if (shouldFetch) {
				results.forEach((result, index) => {
					if (result.data) followed.add(ids[index]);
				});
			}
			const failed = results.find((result) => result.error instanceof Error);
			return {
				ids: followed,
				loading: shouldFetch && results.some((result) => result.isLoading),
				error: failed?.error instanceof Error ? failed.error.message : null,
				ready:
					!enabled ||
					(hydrated &&
						(!shouldFetch || results.every((result) => result.isSuccess))),
			};
		},
		[enabled, hydrated, ids, shouldFetch],
	);
	return useQueries({
		queries: ids.map((id) => ({
			queryKey: queryKeys.buyer.subscription(identity, id),
			queryFn: () => checkVisibleFollows(scope, id),
			enabled: shouldFetch,
			staleTime: Number.POSITIVE_INFINITY,
		})),
		combine,
	});
}

const LEGACY_FLAG_PREFIX = "hawkr:seller-follow:";
let legacyFlagsSwept = false;

export function useIsFollowedSeller(
	sellerId: string | number,
): boolean | undefined {
	const { isLoggedIn, activeRole, hydrated } = useAuth();
	const identity = useQueryIdentity();
	const id = String(sellerId);
	React.useEffect(() => {
		if (legacyFlagsSwept) return;
		legacyFlagsSwept = true;
		try {
			for (const key of Object.keys(localStorage)) {
				if (key.startsWith(LEGACY_FLAG_PREFIX)) localStorage.removeItem(key);
			}
		} catch {
			// Storage may be blocked; these flags were only cosmetic.
		}
	}, []);
	const query = useQuery({
		queryKey: queryKeys.buyer.subscription(identity, id),
		queryFn: () =>
			checkVisibleFollows(`${identity.accountId}:${identity.activeRole}`, id),
		enabled: hydrated && isLoggedIn && activeRole === "buyer" && id !== "",
		staleTime: Number.POSITIVE_INFINITY,
	});
	return query.isSuccess ? query.data : undefined;
}

export function useSetFollowedSeller(): (
	sellerId: string | number,
	followed: boolean,
) => void {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	return React.useCallback(
		(sellerId, followed) => {
			queryClient.setQueryData(
				queryKeys.buyer.subscription(identity, sellerId),
				followed,
			);
		},
		[identity, queryClient],
	);
}
