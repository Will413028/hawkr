"use client";

import { StoreCardStatus } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import {
	commonStoreCardClosedStore,
	commonStoreCardStatus,
	commonUnknown,
} from "@hawkr/ui-text/keys";
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { optionalText, useUiText } from "@/lib/i18n/ui-text";
import { queryKeys } from "@/lib/query/keys";
import { fetchStoreCards, normalizeStoreIds } from "./fetchStoreCards";
import {
	type StoreCardText,
	type StoreCardView,
	storeCardView,
} from "./storeCardView";

export function useStoreCardText(): StoreCardText {
	const ut = useUiText();
	return useMemo(() => {
		const status = (v: StoreCardStatus) =>
			optionalText(ut, commonStoreCardStatus(v)) ?? ut(commonUnknown());
		return {
			paused: status(StoreCardStatus.PAUSED),
			closed: status(StoreCardStatus.CLOSED),
			closedStore: ut(commonStoreCardClosedStore()),
		};
	}, [ut]);
}

/**
 * 一個畫面一次的 BatchGetStores，依 B11 轉成呈現。`views` 以店家 id 為鍵；沒有卡片的店不在裡面。
 * 讀取失敗或還在讀時 `views` 是空的，`settled` 為 false：呼叫端不顯示連結，不退回快照裡的店網址。
 */
export function useStoreCardViews(ids: readonly string[]): {
	views: ReadonlyMap<string, StoreCardView>;
	settled: boolean;
} {
	const text = useStoreCardText();
	const key = normalizeStoreIds(ids);
	const query = useQuery({
		queryKey: queryKeys.sellers.cards(key),
		queryFn: ({ signal }) => fetchStoreCards(key, { signal }),
		enabled: key.length > 0,
		staleTime: 60_000,
	});
	const views = useMemo(() => {
		const out = new Map<string, StoreCardView>();
		for (const [id, card] of query.data ?? []) {
			const view = storeCardView(card, text);
			if (view) out.set(id, view);
		}
		return out;
	}, [query.data, text]);
	return { views, settled: query.isSuccess };
}
