"use client";

import { useQuery } from "@tanstack/react-query";
import type { SearchProductHit, SearchSuggestion } from "@/lib/search/repo";

const STALE_MS = 30_000;
const GC_MS = 5 * 60_000;

export const homeSearchQueryKeys = {
	// This BFF adds related category products; it is not the full keyword-only
	// search transport, even when both surfaces happen to use the same text.
	products: (query: string) =>
		[
			"search",
			"home-popover",
			"related-products",
			{ query, page: 1, pageSize: 6 },
		] as const,
	suggestions: (query: string) =>
		["search", "home-popover", "suggestions", { query }] as const,
};

function debounceRequest(ms: number, signal: AbortSignal): Promise<void> {
	return new Promise((resolve, reject) => {
		signal.throwIfAborted();
		const abort = () => {
			clearTimeout(timer);
			reject(signal.reason);
		};
		const timer = setTimeout(() => {
			signal.removeEventListener("abort", abort);
			resolve();
		}, ms);
		signal.addEventListener("abort", abort, { once: true });
	});
}

/** The query key changes immediately; only a missing/stale request waits for
 * typing to settle. Fresh cached results can therefore paint on the same render
 * as reopening, without showing another keyword during the debounce window. */
export function useHomeSearchQueries(open: boolean, requestQuery: string) {
	// A disabled observer alone does not cancel an in-flight request. Detaching
	// from the active key on close lets Query abort it when no other popover is
	// observing it, while retaining the successful result for the next opening.
	const query = open ? requestQuery.trim() : "";
	const enabled = open && Boolean(query);
	const suggestions = useQuery({
		queryKey: homeSearchQueryKeys.suggestions(query),
		queryFn: async ({ signal }) => {
			await debounceRequest(250, signal);
			const response = await fetch(
				`/api/public/search/suggest?q=${encodeURIComponent(query)}`,
				{ signal, cache: "no-store" },
			);
			if (!response.ok) throw new Error("suggestions unavailable");
			const data = (await response.json()) as {
				suggestions?: SearchSuggestion[];
			};
			signal.throwIfAborted();
			return Array.isArray(data.suggestions) ? data.suggestions : [];
		},
		enabled,
		staleTime: STALE_MS,
		gcTime: GC_MS,
	});
	const products = useQuery({
		queryKey: homeSearchQueryKeys.products(query),
		queryFn: async ({ signal }) => {
			await debounceRequest(260, signal);
			const response = await fetch(
				`/api/public/search/products?q=${encodeURIComponent(query)}&page=1&page_size=6`,
				{ signal, cache: "no-store" },
			);
			if (!response.ok) throw new Error("products unavailable");
			const data = (await response.json()) as {
				records?: SearchProductHit[];
				total?: number;
			};
			signal.throwIfAborted();
			const records = Array.isArray(data.records) ? data.records : [];
			return {
				records,
				total: Number(data.total ?? records.length) || records.length,
			};
		},
		enabled,
		staleTime: STALE_MS,
		gcTime: GC_MS,
	});
	return { products, suggestions };
}
