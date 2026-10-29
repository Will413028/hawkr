"use client";

import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { useMemo } from "react";
import { queryKeys } from "@/lib/query/keys";
import { fetchStoreCards, normalizeStoreIds } from "./fetchStoreCards";
import {
	type StoreCardText,
	type StoreCardView,
	storeCardView,
} from "./storeCardView";

export function useStoreCardText(): StoreCardText {
	const t = useTranslations("storeCard");
	return useMemo(
		() => ({
			paused: t("paused"),
			closed: t("closed"),
			closedStore: t("closedStore"),
		}),
		[t],
	);
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
