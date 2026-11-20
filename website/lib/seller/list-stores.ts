import { sellerPublicClient } from "@/lib/connect/seller-public";
import type { PublicTransportOptions } from "@/lib/connect/transport";

/** `ListStores` 的 page_size 上限（seller.proto `ListStoresRequest`）。 */
export const LIST_STORES_PAGE_SIZE = 1000;

/** sitemap 需要的店家欄位：網址上的 store_code 與最後更新時間（毫秒，可序列化進快取）。 */
export interface StoreSitemapRow {
	storeCode: string;
	updatedAtMs?: number;
}

export interface StoresPage {
	stores: readonly {
		storeCode: string;
		updateTime?: { seconds: bigint; nanos: number };
	}[];
	nextPageToken: string;
}

/** `PublicStore` 整理成 sitemap 列；沒有 store_code 的店不是店面網址，略過。 */
export function storeSitemapRows(page: StoresPage): StoreSitemapRow[] {
	return page.stores
		.filter((s) => s.storeCode !== "")
		.map((s) => ({
			storeCode: s.storeCode,
			...(s.updateTime
				? {
						updatedAtMs:
							Number(s.updateTime.seconds) * 1000 +
							Math.floor(s.updateTime.nanos / 1e6),
					}
				: {}),
		}));
}

/**
 * 讀完所有頁：結束只看 next_page_token 是不是空字串（一頁可以少於 page_size甚至是空的，
 * 見 seller.proto）。任何一頁失敗整個拋出，不回部分清單。
 */
export async function collectAllStores(
	fetchPage: (pageToken: string) => Promise<StoresPage>,
): Promise<StoreSitemapRow[]> {
	const rows: StoreSitemapRow[] = [];
	const seen = new Set<string>();
	let token = "";
	do {
		const page = await fetchPage(token);
		rows.push(...storeSitemapRows(page));
		token = page.nextPageToken;
		if (token !== "") {
			if (seen.has(token)) {
				throw new Error(`ListStores repeated next_page_token "${token}"`);
			}
			seen.add(token);
		}
	} while (token !== "");
	return rows;
}

type ListStoresClient = {
	listStores(
		req: { pageSize: number; pageToken: string },
		opts: { signal?: AbortSignal },
	): Promise<StoresPage>;
};

/** `pageSignal` 每頁呼叫一次，每頁各有自己的逾時。 */
export function fetchAllStoresWith(
	client: ListStoresClient,
	pageSignal?: () => AbortSignal,
): Promise<StoreSitemapRow[]> {
	return collectAllStores((pageToken) =>
		client.listStores(
			{ pageSize: LIST_STORES_PAGE_SIZE, pageToken },
			{ signal: pageSignal?.() },
		),
	);
}

export function fetchAllStoresForSitemap(
	opts: PublicTransportOptions & { pageSignal?: () => AbortSignal } = {},
): Promise<StoreSitemapRow[]> {
	const { pageSignal, ...transport } = opts;
	return fetchAllStoresWith(sellerPublicClient(transport), pageSignal);
}
