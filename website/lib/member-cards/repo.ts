/** BuyerAppService member cards and spending records mapped to wallet views. */

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { StoreCard } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { buyerAppClient } from "../connect/buyer";
import { imageUri } from "../media/image";

/** OpenAPI: handler.MemberCardItemResponse. */
export interface MemberCard {
	sellerId: string;
	sellerName: string;
	sellerLogo: string;
	coverUrl: string;
	/** Seller's canonical path segment (gateway `memberCardItem.storeCode`).
	 * `sellerId` routes the in-site card (`/account/wallet/{sellerId}`) and keys
	 * the spending-records API; only this reaches the seller's storefront
	 * without a 308. Empty when seller 模組 was unreachable — the same
	 * degrade that blanks `sellerName`/`sellerLogo`. */
	storeCode: string;
	/** 會員卡隨附的店家卡片；B11 的暫停營業、已關閉由它的 status 決定（`storeCardView`）。 */
	store?: StoreCard;
	/** Cumulative spend at this seller, kept as a string (backend ships it as one). */
	totalSpending: string;
	isVip: boolean;
}

export interface SpendingRecord {
	orderDate: string;
	orderSn: string;
	amount: string;
	refund: string;
	hasCoupon: boolean;
}

export interface SpendingRecordsResult {
	records: SpendingRecord[];
	totalSpending: string;
	totalRefund: string;
	nextPageToken: string;
}

export function normalizeMemberCard(input: unknown): MemberCard {
	const raw = (input ?? {}) as Record<string, unknown>;
	return {
		sellerId: String(raw.sellerId ?? ""),
		sellerName: String(raw.sellerName ?? ""),
		// B-238: member-card seller identity uses only the canonical URL.
		sellerLogo: typeof raw.logoUrl === "string" ? raw.logoUrl.trim() : "",
		// §231: the bare objectKey is the SSOT. The paired `coverUrl` is
		// `buildURL(base, CoverKey)` (member_card.go:20-21) — derived from this
		// key, so reading it as a fallback could only ever mask a contract break.
		coverUrl: String(raw.coverKey ?? ""),
		storeCode: String(raw.storeCode ?? ""),
		totalSpending: String(raw.totalSpending ?? "0"),
		isVip: Boolean(raw.isVip ?? false),
	};
}

export async function fetchMemberCards({
	pageToken = "",
	size = 20,
}: {
	pageToken?: string;
	size?: number;
} = {}): Promise<{
	records: MemberCard[];
	nextPageToken: string;
}> {
	const result = await buyerAppClient().listMemberCards({
		pageSize: size,
		pageToken,
	});
	return {
		records: result.memberCards.map((card) => ({
			sellerId: card.storeId,
			sellerName: card.store?.name ?? "",
			sellerLogo: imageUri(card.store?.logo, "avatar") ?? "",
			coverUrl: imageUri(card.store?.coverImage, "card") ?? "",
			storeCode: card.store?.storeCode ?? "",
			store: card.store,
			totalSpending: String(card.totalSpent),
			isVip: card.vip,
		})),
		nextPageToken: result.nextPageToken,
	};
}

export async function fetchSpendingRecords({
	sellerId,
	pageToken = "",
	size = 20,
}: {
	sellerId: string;
	pageToken?: string;
	size?: number;
}): Promise<SpendingRecordsResult> {
	const result = await buyerAppClient().listSpendingRecords({
		storeId: sellerId,
		pageSize: size,
		pageToken,
	});
	return {
		records: result.records.map((record) => ({
			orderDate: record.orderTime
				? timestampDate(record.orderTime).toISOString()
				: "",
			orderSn: record.orderNumber,
			amount: String(record.amount),
			refund: String(record.refundedAmount),
			hasCoupon: record.couponApplied,
		})),
		totalSpending: String(result.totalSpent),
		totalRefund: String(result.totalRefunded),
		nextPageToken: result.nextPageToken,
	};
}
