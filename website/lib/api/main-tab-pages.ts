import type { ProductRecord } from "@/lib/market/listFilters";
import type { PostsPage } from "@/lib/posts/repo";

type MainTabPageEnvelope<T> = {
	schemaVersion: "tab-page.v1";
	tab: "feed" | "market";
	data: T;
};

async function requestMainTabPage<T>(
	tab: "feed" | "market",
	cursor: { page: number } | { pageToken: string },
	signal?: AbortSignal,
): Promise<T> {
	const query =
		"page" in cursor
			? `page=${cursor.page}`
			: `pageToken=${encodeURIComponent(cursor.pageToken)}`;
	const response = await fetch(
		`/api/public/tab-bootstrap/v1/head?tab=${tab}&${query}`,
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

/** 匿名的 canonical 動態牆：第一頁 pageToken 是空字串，之後用上一頁回的 `nextPageToken`。 */
export function fetchCanonicalFeedPage(
	pageToken: string,
	signal?: AbortSignal,
): Promise<PostsPage> {
	return requestMainTabPage<PostsPage>("feed", { pageToken }, signal);
}

export async function fetchCanonicalMarketRecords(
	page: number,
	signal?: AbortSignal,
): Promise<ProductRecord[]> {
	const envelope = await requestMainTabPage<{
		data?: { records?: ProductRecord[] };
		records?: ProductRecord[];
	}>("market", { page }, signal);
	return envelope.data?.records ?? envelope.records ?? [];
}
