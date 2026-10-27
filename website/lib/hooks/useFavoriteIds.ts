"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { createMembershipBatcher } from "@/lib/connect/batch-membership";
import { batchCheckFavoriteProductIds } from "@/lib/favorites/repo";
import { type QueryIdentity, queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

const checkVisibleFavorites = createMembershipBatcher(
	batchCheckFavoriteProductIds,
);

export function favoriteIdKey(
	identity: QueryIdentity,
	productId: string | number,
) {
	return queryKeys.buyer.favorites(identity, {
		view: "membership",
		productId: String(productId),
	});
}

export function useIsFavorited(
	productId: string | number,
): boolean | undefined {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const id = String(productId);
	const query = useQuery({
		queryKey: favoriteIdKey(identity, id),
		queryFn: () =>
			checkVisibleFavorites(`${identity.accountId}:${identity.activeRole}`, id),
		enabled: hydrated && isLoggedIn && id !== "",
		staleTime: Number.POSITIVE_INFINITY,
	});
	return query.isSuccess ? query.data : undefined;
}

export function useSetFavorited(): (
	productId: string | number,
	favorited: boolean,
) => void {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	return React.useCallback(
		(productId, favorited) => {
			queryClient.setQueryData(favoriteIdKey(identity, productId), favorited);
		},
		[identity, queryClient],
	);
}
