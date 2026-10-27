"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { checkFollowed, setSellerFollow } from "@/lib/follow/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface SellerSubscriptionState {
	/** Optimistic-aware subscribed flag. `false` for guests / before load. */
	active: boolean;
	/** True once the status read has resolved (or there is nothing to load). */
	ready: boolean;
	/** A toggle request is currently in flight. */
	pending: boolean;
	/**
	 * Flip subscribed⇄unsubscribed. The cache updates synchronously
	 * (optimistic), then reconciles to the server truth — or rolls back and
	 * rethrows on failure so the caller can surface a notice.
	 */
	toggle: () => Promise<boolean>;
}

/**
 * One seller's follow state on TanStack Query (the foundation-stack data
 * layer) — a per-seller `GET /subscriptions/{id}` read plus an optimistic
 * POST/DELETE mutation. It shares the per-seller query key with the batched
 * follow chips and feed filter (`useFollowedSellers.ts`), so a toggle here is
 * what they show too.
 */
export function useSellerSubscription(
	sellerId: string | number,
): SellerSubscriptionState {
	const { isLoggedIn, activeRole, hydrated } = useAuth();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const isBuyer = activeRole === "buyer";
	const enabled =
		hydrated && isLoggedIn && isBuyer && sellerId != null && sellerId !== "";
	const queryKey = React.useMemo(
		() => queryKeys.buyer.subscription(identity, sellerId),
		[identity, sellerId],
	);

	const query = useQuery({
		queryKey,
		queryFn: () => checkFollowed(sellerId, "seller"),
		enabled,
		placeholderData: false,
	});

	const mutation = useMutation({
		mutationFn: (next: boolean) => setSellerFollow(sellerId, next),
	});
	const togglePendingRef = React.useRef(false);

	const active = enabled ? (query.data ?? false) : false;

	const toggle = React.useCallback(async () => {
		if (togglePendingRef.current) {
			return queryClient.getQueryData<boolean>(queryKey) ?? false;
		}
		togglePendingRef.current = true;
		const before = queryClient.getQueryData<boolean>(queryKey) ?? false;
		const next = !before;
		queryClient.setQueryData(queryKey, next);
		try {
			const result = await mutation.mutateAsync(next);
			queryClient.setQueryData(queryKey, result);
			return result;
		} catch (error) {
			queryClient.setQueryData(queryKey, before);
			throw error;
		} finally {
			togglePendingRef.current = false;
		}
	}, [mutation, queryClient, queryKey]);

	return {
		active,
		ready: enabled ? query.isSuccess : true,
		pending: mutation.isPending,
		toggle,
	};
}
