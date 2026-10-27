/** BuyerAppService visit methods; history contains product visits. */

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { buyerAppClient } from "../connect/buyer";
import { imageUri } from "../media/image";

export interface HistoryItem {
	id: string;
	productId: string;
	productName: string;
	/**
	 * §231 flat-pair: `imageKey` (bare objectKey, SSOT) + `imageUrl` (Gateway
	 * URL). 49 §6.6.1 — 這張卡只有一組商品圖，所以 `product` 前綴不帶資訊，
	 * 欄名收斂成 `imageKey`/`imageUrl`；同物件的賣家 logo 走 `logoKey`/`logoUrl`，
	 * 兩者不會撞名。
	 */
	imageKey?: string;
	imageUrl?: string;
	minPrice: string;
	maxPrice: string;
	rating: string;
	sellerId: string;
	sellerName: string;
	/**
	 * 賣家店面 canonical 路徑短碼（`sellers.store_code`）。
	 *
	 * `sellerId` 是 store_id（內部代理鍵），賣家頁 canonical 網址走 storeCode。
	 * buyer-service 為了 sellerName/logo 本來就會呼叫 GetSellerBatch，值同源；
	 * 取不到時後端送空字串（→ 不連結，不回退 sellerId）。
	 */
	storeCode?: string;
	/** B-238 canonical seller-logo display URL for history rows. */
	logoUrl?: string;
	visitedAt: string;
	/** Backend may omit; `false` (or a missing productName) means the
	 *  product is no longer visible/available and should be hidden. */
	available?: boolean;
}

export async function fetchHistory({
	pageToken = "",
	size = 30,
}: {
	pageToken?: string;
	size?: number;
} = {}): Promise<{ items: HistoryItem[]; nextPageToken: string }> {
	const response = await buyerAppClient().listVisits({
		pageSize: size,
		pageToken,
	});
	return {
		items: response.visits.map((visit) => {
			const product = visit.product;
			const store = product?.store;
			return {
				id: product?.id ?? "",
				productId: product?.id ?? "",
				productName: product?.name ?? "",
				imageKey: imageUri(product?.image, "card"),
				imageUrl: imageUri(product?.image, "card"),
				minPrice: String(product?.minPrice ?? 0),
				maxPrice: String(product?.maxPrice ?? 0),
				rating: String(product?.rating ?? ""),
				sellerId: product?.storeId ?? "",
				sellerName: store?.name ?? "",
				storeCode: store?.storeCode,
				logoUrl: imageUri(store?.logo, "avatar"),
				visitedAt: visit.visitTime
					? timestampDate(visit.visitTime).toISOString()
					: "",
				available: product?.available ?? false,
			};
		}),
		nextPageToken: response.nextPageToken,
	};
}

export async function pushHistory(productId: string): Promise<void> {
	await buyerAppClient().recordVisit({ productId });
}

export async function clearHistory(): Promise<void> {
	await buyerAppClient().clearVisits({});
}
