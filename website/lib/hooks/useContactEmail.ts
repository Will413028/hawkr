"use client";

import { useQuery } from "@tanstack/react-query";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { queryKeys } from "@/lib/query/keys";
import {
	type AccountContactEmail,
	fetchContactEmail,
} from "@/lib/seller/contact-email";

interface UseContactEmailResult {
	/** 帳號還沒有聯絡信箱是 null。 */
	contact: AccountContactEmail | null;
	loading: boolean;
	error: Error | null;
	refetch: () => Promise<unknown>;
}

/** 帳號的聯絡信箱（`GetContactEmail`）。 */
export function useContactEmail(): UseContactEmailResult {
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.seller.contactEmail(identity),
		queryFn: fetchContactEmail,
		staleTime: 30_000,
	});
	return {
		contact: query.data ?? null,
		loading: query.isLoading,
		error: query.error,
		refetch: query.refetch,
	};
}
