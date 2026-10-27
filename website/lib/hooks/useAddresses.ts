"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
	type Address,
	type AddressInput,
	createAddress,
	deleteAddress,
	fetchAddresses,
	setDefaultAddress,
	updateAddress,
} from "@/lib/addresses/repo";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseAddressesResult {
	addresses: Address[];
	defaultAddress: Address | null;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
	create: (body: AddressInput) => Promise<void>;
	update: (
		id: string | number,
		etag: string,
		body: AddressInput,
	) => Promise<void>;
	remove: (id: string | number) => Promise<void>;
	setDefault: (id: string | number) => Promise<void>;
}

type AddressListSnapshot = Address[] | undefined;

function applyDefault(addresses: Address[], id: string | number): Address[] {
	const targetId = String(id);
	return addresses.map((address) => ({
		...address,
		isDefault: String(address.id) === targetId,
	}));
}

export function useAddresses(): UseAddressesResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.addresses(identity);
	const addressesQuery = useQuery({
		queryKey,
		queryFn: fetchAddresses,
		enabled: hydrated && isLoggedIn,
		placeholderData: [] as Address[],
	});

	const invalidateAddresses = async () => {
		await queryClient.invalidateQueries({
			queryKey,
		});
	};

	const createMutation = useMutation({
		mutationFn: createAddress,
		onSuccess: (created) => {
			if (!created) return;
			queryClient.setQueryData<Address[]>(queryKey, (current = []) => {
				const next = created.isDefault
					? current.map((address) => ({ ...address, isDefault: false }))
					: current;
				if (next.some((address) => address.id === created.id)) return next;
				return [created, ...next];
			});
		},
		onSettled: invalidateAddresses,
	});
	const updateMutation = useMutation({
		mutationFn: ({
			id,
			etag,
			body,
		}: {
			id: string | number;
			etag: string;
			body: AddressInput;
		}) => updateAddress(id, etag, body),
		onMutate: async ({ id, body }) => {
			await queryClient.cancelQueries({ queryKey });
			const previous = queryClient.getQueryData<Address[]>(queryKey);
			queryClient.setQueryData<Address[]>(queryKey, (current = []) =>
				current.map((address) => {
					if (String(address.id) !== String(id)) {
						return body.isDefault ? { ...address, isDefault: false } : address;
					}
					return { ...address, ...body, id: String(id) };
				}),
			);
			return { previous };
		},
		onError: (
			_error,
			_variables,
			context?: { previous: AddressListSnapshot },
		) => {
			queryClient.setQueryData(queryKey, context?.previous);
		},
		onSettled: invalidateAddresses,
	});
	const removeMutation = useMutation({
		mutationFn: deleteAddress,
		onMutate: async (id) => {
			await queryClient.cancelQueries({ queryKey });
			const previous = queryClient.getQueryData<Address[]>(queryKey);
			queryClient.setQueryData<Address[]>(queryKey, (current = []) => {
				const next = current.filter(
					(address) => String(address.id) !== String(id),
				);
				if (next.some((address) => address.isDefault)) return next;
				return next.map((address, index) => ({
					...address,
					isDefault: index === 0,
				}));
			});
			return { previous };
		},
		onError: (
			_error,
			_variables,
			context?: { previous: AddressListSnapshot },
		) => {
			queryClient.setQueryData(queryKey, context?.previous);
		},
		onSettled: invalidateAddresses,
	});
	const setDefaultMutation = useMutation({
		mutationFn: async (id: string | number) => {
			const cached = queryClient.getQueryData<Address[]>(queryKey);
			const target = cached?.find(
				(address) => String(address.id) === String(id),
			);
			if (!target) throw new Error("地址版本資訊不存在，請重新載入");
			await setDefaultAddress(id, target.etag);
		},
		onMutate: async (id) => {
			await queryClient.cancelQueries({ queryKey });
			const previous = queryClient.getQueryData<Address[]>(queryKey);
			queryClient.setQueryData<Address[]>(queryKey, (current = []) =>
				applyDefault(current, id),
			);
			return { previous };
		},
		onError: (
			_error,
			_variables,
			context?: { previous: AddressListSnapshot },
		) => {
			queryClient.setQueryData(queryKey, context?.previous);
		},
		onSettled: invalidateAddresses,
	});

	const addresses = hydrated && isLoggedIn ? (addressesQuery.data ?? []) : [];
	const mutationError =
		createMutation.error ??
		updateMutation.error ??
		removeMutation.error ??
		setDefaultMutation.error;
	const error =
		addressesQuery.error instanceof Error
			? addressesQuery.error.message
			: mutationError instanceof Error
				? mutationError.message
				: null;

	return {
		addresses,
		defaultAddress: addresses.find((a) => a.isDefault) ?? null,
		loading: addressesQuery.isLoading,
		error,
		refresh: async () => {
			await addressesQuery.refetch();
		},
		create: async (body) => {
			await createMutation.mutateAsync(body);
		},
		update: (id, etag, body) => updateMutation.mutateAsync({ id, etag, body }),
		remove: (id) => removeMutation.mutateAsync(id),
		setDefault: (id) => setDefaultMutation.mutateAsync(id),
	};
}
