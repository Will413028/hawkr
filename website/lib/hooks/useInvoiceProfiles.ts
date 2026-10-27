"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	createInvoiceProfile,
	deleteInvoiceProfile,
	fetchInvoiceProfiles,
	setDefaultInvoiceProfile,
	updateInvoiceProfile,
} from "@/lib/invoices/buyer-repo";
import type {
	CreateInvoiceProfileInput,
	InvoiceProfile,
	UpdateInvoiceProfileInput,
} from "@/lib/invoices/types";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseInvoiceProfilesResult {
	profiles: InvoiceProfile[];
	defaultProfile: InvoiceProfile | null;
	loading: boolean;
	/** 端點 1 本身失敗的訊息。mutation 的錯誤由呼叫端 catch（要分業務碼）。 */
	error: string | null;
	refresh: () => Promise<void>;
	create: (
		input: CreateInvoiceProfileInput,
		idempotencyKey: string,
	) => Promise<InvoiceProfile>;
	update: (
		profileId: string,
		input: UpdateInvoiceProfileInput,
		idempotencyKey: string,
	) => Promise<InvoiceProfile>;
	remove: (profileId: string, idempotencyKey: string) => Promise<boolean>;
	setDefault: (
		profileId: string,
		idempotencyKey: string,
	) => Promise<InvoiceProfile>;
}

/**
 * 常用發票資料的 react-query 包裝（58A §B08）。三層分工照
 * `lib/hooks/useAddresses.ts`：repo 打 HTTP、hook 管快取、元件只拿資料。
 *
 * 🔴 `setDefault` **不做 optimistic update**（§B-1.0.9 B10t）：它同時改兩筆
 * （新預設 true、舊預設 false），本地模擬互斥一旦錯了就會出現「兩個預設」或
 * 「零個預設」，而畫面看起來完全正常，下一次結帳才靜默開錯載具。等後端回來
 * 再 invalidate 才是唯一不會說謊的做法。
 */
export function useInvoiceProfiles(): UseInvoiceProfilesResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.invoiceProfiles(identity);

	const profilesQuery = useQuery({
		queryKey,
		queryFn: async () => (await fetchInvoiceProfiles()).records,
		enabled: hydrated && isLoggedIn,
	});

	const invalidate = async () => {
		await queryClient.invalidateQueries({ queryKey });
	};

	const createMutation = useMutation({
		mutationFn: ({
			input,
			idempotencyKey,
		}: {
			input: CreateInvoiceProfileInput;
			idempotencyKey: string;
		}) => createInvoiceProfile(input, idempotencyKey),
		onSuccess: (created) => {
			queryClient.setQueryData<InvoiceProfile[]>(queryKey, (current = []) => {
				const next = created.isDefault
					? current.map((profile) => ({ ...profile, isDefault: false }))
					: current;
				if (next.some((profile) => profile.profileId === created.profileId)) {
					return next;
				}
				return [...next, created];
			});
		},
		onSettled: invalidate,
	});

	const updateMutation = useMutation({
		mutationFn: ({
			profileId,
			input,
			idempotencyKey,
		}: {
			profileId: string;
			input: UpdateInvoiceProfileInput;
			idempotencyKey: string;
		}) => updateInvoiceProfile(profileId, input, idempotencyKey),
		onSuccess: (updated) => {
			queryClient.setQueryData<InvoiceProfile[]>(queryKey, (current = []) =>
				current.map((profile) =>
					profile.profileId === updated.profileId ? updated : profile,
				),
			);
		},
		onSettled: invalidate,
	});

	const removeMutation = useMutation({
		mutationFn: ({
			profileId,
			idempotencyKey,
		}: {
			profileId: string;
			idempotencyKey: string;
		}) => deleteInvoiceProfile(profileId, idempotencyKey),
		onSuccess: (deleted, { profileId }) => {
			if (!deleted) return;
			queryClient.setQueryData<InvoiceProfile[]>(queryKey, (current = []) =>
				current.filter((profile) => profile.profileId !== profileId),
			);
		},
		onSettled: invalidate,
	});

	const setDefaultMutation = useMutation({
		mutationFn: ({
			profileId,
			idempotencyKey,
		}: {
			profileId: string;
			idempotencyKey: string;
		}) => setDefaultInvoiceProfile(profileId, idempotencyKey),
		onSettled: invalidate,
	});

	const profiles = hydrated && isLoggedIn ? (profilesQuery.data ?? []) : [];

	return {
		profiles,
		defaultProfile: profiles.find((profile) => profile.isDefault) ?? null,
		loading: profilesQuery.isLoading,
		error:
			profilesQuery.error instanceof Error ? profilesQuery.error.message : null,
		refresh: async () => {
			await profilesQuery.refetch();
		},
		create: (input, idempotencyKey) =>
			createMutation.mutateAsync({ input, idempotencyKey }),
		update: (profileId, input, idempotencyKey) =>
			updateMutation.mutateAsync({ profileId, input, idempotencyKey }),
		remove: (profileId, idempotencyKey) =>
			removeMutation.mutateAsync({ profileId, idempotencyKey }),
		setDefault: (profileId, idempotencyKey) =>
			setDefaultMutation.mutateAsync({ profileId, idempotencyKey }),
	};
}
