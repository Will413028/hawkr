"use client";

import {
	hashKey,
	type QueryCache,
	type QueryKey,
	useQueryClient,
} from "@tanstack/react-query";
import { useEffect } from "react";
import { useMainTabFreshnessStore } from "@/lib/state/main-tab-freshness-store";

const observedCaches = new WeakSet<QueryCache>();

/** The query hash includes every filter and any viewer scope. Keep scheduling
 * metadata only as long as its QueryCache entry, including after unmount when
 * the inactive query waits for GC. One listener belongs to each cache rather
 * than to a component, so it also sees later GC / identity teardown / clear. */
export function useListFreshnessScope(queryKey: QueryKey): string {
	const queryClient = useQueryClient();
	useEffect(() => {
		const cache = queryClient.getQueryCache();
		if (observedCaches.has(cache)) return;
		observedCaches.add(cache);
		cache.subscribe((event) => {
			if (event.type !== "removed") return;
			const scope = `list:${event.query.queryHash}`;
			useMainTabFreshnessStore
				.getState()
				.removeScopes([scope, `${scope}:full`]);
		});
	}, [queryClient]);
	return `list:${hashKey(queryKey)}`;
}
