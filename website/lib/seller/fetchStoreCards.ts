import type { StoreCard } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { sellerPublicClient } from "@/lib/connect/seller-public";
import type { PublicTransportOptions } from "@/lib/connect/transport";

const BATCH_LIMIT = 100;

/** 排序去重並丟掉空字串：同一組店家不論傳入順序都得到同一個 query 鍵。 */
export function normalizeStoreIds(ids: readonly string[]): string[] {
	return [...new Set(ids.map((id) => id.trim()).filter(Boolean))].sort();
}

/**
 * 批次讀店家卡片（每次最多 100 家，超過分批）。沒有卡片的店（沒公開過、不存在）不在結果裡，
 * 呼叫端不區分兩者。
 */
export async function fetchStoreCards(
	ids: readonly string[],
	opts: PublicTransportOptions & { signal?: AbortSignal } = {},
): Promise<Map<string, StoreCard>> {
	const { signal, ...transport } = opts;
	const unique = normalizeStoreIds(ids);
	const client = sellerPublicClient(transport);
	const out = new Map<string, StoreCard>();
	for (let i = 0; i < unique.length; i += BATCH_LIMIT) {
		const res = await client.batchGetStores(
			{ ids: unique.slice(i, i + BATCH_LIMIT) },
			{ signal },
		);
		for (const card of res.stores) out.set(card.id, card);
	}
	return out;
}
