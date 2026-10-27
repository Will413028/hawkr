import type { Paginated, PostListItem } from "@/lib/api/openapi-types";
import type { ProductRecord } from "@/lib/market/listFilters";

type MainTabPageEnvelope<T> = {
	schemaVersion: "tab-page.v1";
	tab: "feed" | "market";
	data: T;
};

async function requestMainTabPage<T>(
	tab: "feed" | "market",
	page: number,
	signal?: AbortSignal,
): Promise<T> {
	const response = await fetch(
		`/api/public/tab-bootstrap/v1/head?tab=${tab}&page=${page}`,
		{
			method: "GET",
			credentials: "omit",
			cache: "no-store",
			headers: { accept: "application/json" },
			signal,
		},
	);
	if (!response.ok) {
		throw new Error(`tab page refresh failed (${response.status})`);
	}
	const payload = (await response.json()) as MainTabPageEnvelope<T>;
	if (payload.schemaVersion !== "tab-page.v1" || payload.tab !== tab) {
		throw new Error("unsupported tab page schema");
	}
	return payload.data;
}

export function fetchCanonicalFeedPage(
	page: number,
	signal?: AbortSignal,
): Promise<Paginated<PostListItem>> {
	return requestMainTabPage<Paginated<PostListItem>>("feed", page, signal);
}

export async function fetchCanonicalMarketRecords(
	page: number,
	signal?: AbortSignal,
): Promise<ProductRecord[]> {
	const envelope = await requestMainTabPage<{
		data?: { records?: ProductRecord[] };
		records?: ProductRecord[];
	}>("market", page, signal);
	return envelope.data?.records ?? envelope.records ?? [];
}
