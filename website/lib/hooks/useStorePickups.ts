"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { accountQueryFn, accountQueryPending } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import {
	createStorePickup,
	deleteStorePickup,
	fetchStorePickups,
	type StorePickup,
	type StorePickupInput,
	updateStorePickup,
} from "@/lib/checkout/store-pickups";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseStorePickupsResult {
	pickups: StorePickup[];
	defaultPickup: StorePickup | null;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
	create: (body: StorePickupInput) => Promise<void>;
	update: (id: string, body: StorePickupInput) => Promise<void>;
	remove: (id: string) => Promise<void>;
}

/**
 * Buyer's saved 7-11 / 全家 pickup records. Mirrors `useAddresses` so the
 * cart can pick a saved store the same way it picks a saved home address.
 * Returns empty when the user isn't authed (the endpoint is auth-only).
 */
export function useStorePickups(): UseStorePickupsResult {
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const pickupsQuery = useQuery({
		queryKey: queryKeys.checkout.storePickups(identity),
		queryFn: accountQueryFn(status, fetchStorePickups),
	});

	const invalidatePickups = async () => {
		await queryClient.invalidateQueries({
			queryKey: queryKeys.checkout.storePickups(identity),
		});
	};

	const createMutation = useMutation({
		mutationFn: createStorePickup,
		onSuccess: invalidatePickups,
	});
	const updateMutation = useMutation({
		mutationFn: ({ id, body }: { id: string; body: StorePickupInput }) =>
			updateStorePickup(id, body),
		onSuccess: invalidatePickups,
	});
	const removeMutation = useMutation({
		mutationFn: deleteStorePickup,
		onSuccess: invalidatePickups,
	});

	const pickups = useMemo(
		() => (status === "signedIn" && pickupsQuery.data) || [],
		[status, pickupsQuery.data],
	);
	const mutationError =
		createMutation.error ?? updateMutation.error ?? removeMutation.error;
	const error =
		mutationError instanceof Error
			? mutationError.message
			: pickupsQuery.error instanceof Error
				? pickupsQuery.error.message
				: null;

	return {
		pickups,
		defaultPickup: pickups.find((p) => p.isDefault) ?? pickups[0] ?? null,
		loading: accountQueryPending(status, pickupsQuery.isPending),
		error,
		refresh: async () => {
			await pickupsQuery.refetch();
		},
		create: async (body) => {
			await createMutation.mutateAsync(body);
		},
		update: async (id, body) => {
			await updateMutation.mutateAsync({ id, body });
		},
		remove: async (id) => {
			await removeMutation.mutateAsync(id);
		},
	};
}
