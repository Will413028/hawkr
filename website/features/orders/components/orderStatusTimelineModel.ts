import { AFTER_SALE_STATUS } from "@/lib/after-sales/types";

export const ORDER_STATUS_TIMELINE_STEPS: Array<{
	key: string;
	label: string;
}> = [
	{ key: "pending_payment", label: "待付款" },
	{ key: "paid", label: "已付款" },
	{ key: "preparing", label: "準備中" },
	{ key: "shipping", label: "運送中" },
	{ key: "completed", label: "已完成" },
];

/**
 * Folds granular backend statuses onto the 5-step display rail.
 * `delivered` collapses into the final "已完成" milestone.
 */
export const ORDER_STATUS_TIMELINE_INDEX: Record<string, number> = {
	pending_payment: 0,
	paid: 1,
	preparing: 2,
	shipping: 3,
	// 待取貨（超商包裹配達門市待取）與配送異常皆屬「運送中」階段，折疊到運送中里程碑；
	// 若缺這兩個 key，`?? 0` 會讓進度條錯誤倒退回「待付款」（[B-207] dev 驗證發現）。
	waiting_pickup: 3,
	shipping_abnormal: 3,
	delivered: 4,
	completed: 4,
};

export function shouldMergeAfterSaleIntoOrder(
	afterSaleStatus?: string | null,
): boolean {
	return afterSaleStatus === AFTER_SALE_STATUS.CLOSED;
}

export function orderTimelineCurrentIndex({
	orderStatus,
	hasAfterSale,
	afterSaleStatus,
}: {
	orderStatus: string;
	hasAfterSale: boolean;
	afterSaleStatus?: string | null;
}): number {
	const rawIndex =
		orderStatus === "after_sale" || orderStatus === "refunded"
			? 3
			: (ORDER_STATUS_TIMELINE_INDEX[orderStatus] ?? 0);
	if (!hasAfterSale) return rawIndex;
	if (shouldMergeAfterSaleIntoOrder(afterSaleStatus)) return 4;
	return Math.min(rawIndex, 3);
}
