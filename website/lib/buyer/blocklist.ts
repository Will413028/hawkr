/**
 * 買家的封鎖名單（`BuyerAppService.ListBlockedStores`／`ListBlockedBuyers`／`UnblockStore`／`UnblockBuyer`）：
 * 店家與買家兩段各自分頁，新到舊。解除封鎖在本來就沒有封鎖時也成功。
 */

import type {
	BlockedBuyer,
	BlockedStore,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { buyerAppClient } from "../connect/buyer";

type BuyerClient = ReturnType<typeof buyerAppClient>;

/** 一頁的筆數（契約預設 20，上限 100）。 */
export const BLOCKLIST_PAGE_SIZE = 20;

export interface BlockedPage<T> {
	items: T[];
	nextPageToken: string;
}

export async function listBlockedStores(
	pageToken = "",
	client: () => BuyerClient = buyerAppClient,
): Promise<BlockedPage<BlockedStore>> {
	const res = await client().listBlockedStores({
		pageSize: BLOCKLIST_PAGE_SIZE,
		pageToken,
	});
	return {
		// 沒有店家 id 的項目無法解除封鎖，不是可操作的資料。
		items: res.blockedStores.filter((s) => s.storeId !== ""),
		nextPageToken: res.nextPageToken,
	};
}

export async function listBlockedBuyers(
	pageToken = "",
	client: () => BuyerClient = buyerAppClient,
): Promise<BlockedPage<BlockedBuyer>> {
	const res = await client().listBlockedBuyers({
		pageSize: BLOCKLIST_PAGE_SIZE,
		pageToken,
	});
	return {
		items: res.blockedBuyers.filter((b) => b.buyerId !== ""),
		nextPageToken: res.nextPageToken,
	};
}

export async function unblockStore(
	storeId: string,
	client: () => BuyerClient = buyerAppClient,
): Promise<void> {
	await client().unblockStore({ storeId });
}

export async function unblockBuyer(
	buyerId: string,
	client: () => BuyerClient = buyerAppClient,
): Promise<void> {
	await client().unblockBuyer({ buyerId });
}
