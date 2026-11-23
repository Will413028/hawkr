"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { queryKeys } from "@/lib/query/keys";
import { type CustomerTag, listTags } from "@/lib/seller/crm";
import { useCrmStoreId } from "./useCrmStoreId";
import { useQueryIdentity } from "./useQueryIdentity";

const NO_TAGS: CustomerTag[] = [];

interface UseSellerTagsResult {
	tags: CustomerTag[];
	/** 讀取失敗的文字；失敗時 `tags` 是空陣列，但與「店家沒有標籤」不同，呼叫端要顯示這段。 */
	error: string | null;
}

/**
 * The seller's whole tag pool (`CrmAppService.ListTags`, at most 200, not paged) — powers the
 * customer-list filter chips and the tag-editor suggestions. Both mount at once while the
 * customer drawer is open, so they share one identity-scoped cache entry.
 */
export function useSellerTags(): UseSellerTagsResult {
	const identity = useQueryIdentity();
	const { storeId, error: storeError } = useCrmStoreId();
	const defaults = useDefaultErrorText();
	const query = useQuery({
		queryKey: queryKeys.seller.tags(identity),
		queryFn: () => listTags(storeId as string),
		enabled: identity.activeRole === "seller" && Boolean(storeId),
		staleTime: 60_000,
	});
	const failure = storeError ?? query.error;
	return useMemo(
		() => ({
			tags: query.data ?? NO_TAGS,
			error: failure ? errorText(failure, defaults) : null,
		}),
		[query.data, failure, defaults],
	);
}
