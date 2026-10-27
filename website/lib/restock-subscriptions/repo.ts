/** BuyerAppService restock methods; subscription state is separate from favorites. */

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { RestockSubscription } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { buyerAppClient } from "../connect/buyer";
import { imageUri } from "../media/image";

/**
 * Subscribe the current buyer to product restock notifications.
 * Idempotent — backend returns the existing subscription if already ACTIVE.
 */
export async function subscribeRestock(
	productId: number | string,
): Promise<void> {
	await buyerAppClient().subscribeRestock({ productId: String(productId) });
}

/**
 * Unsubscribe (cancel) the current buyer's restock subscription for this
 * product. Idempotent — backend returns 200 even if no ACTIVE row exists.
 */
export async function unsubscribeRestock(
	productId: number | string,
): Promise<void> {
	await buyerAppClient().unsubscribeRestock({ productId: String(productId) });
}

export interface RestockSubscriptionItem {
	subscriptionId: string;
	productId: string;
	productName: string;
	productImage: string;
	productStatus: string;
	createdAt: string;
	/** ProductStatus == "4004" AND !deleted — purchasable right now. */
	available: boolean;
}

export function restockFromConnect(
	row: RestockSubscription,
): RestockSubscriptionItem {
	return {
		subscriptionId: row.product?.id ?? "",
		productId: row.product?.id ?? "",
		productName: row.product?.name ?? "",
		productImage: imageUri(row.product?.image, "card") ?? "",
		productStatus: "",
		createdAt: row.createTime
			? timestampDate(row.createTime).toISOString()
			: "",
		available: row.product?.available ?? false,
	};
}

export interface RestockListPage {
	records: RestockSubscriptionItem[];
	hasMore: boolean;
	nextCursorId: string;
}

/**
 * GET buyer's ACTIVE restock subscriptions, cursor-paginated.
 * Backend response carries enrichment best-effort — product enrich failure
 * leaves productName / productImage as empty strings (still safe to render).
 */
export async function fetchRestockSubscriptions(
	opts: { cursorId?: string; pageSize?: number } = {},
): Promise<RestockListPage> {
	const page = await buyerAppClient().listRestockSubscriptions({
		pageToken: opts.cursorId ?? "",
		pageSize: opts.pageSize ?? 20,
	});
	const records = page.restockSubscriptions.map(restockFromConnect);
	return {
		records,
		hasMore: Boolean(page.nextPageToken),
		nextCursorId: page.nextPageToken,
	};
}
