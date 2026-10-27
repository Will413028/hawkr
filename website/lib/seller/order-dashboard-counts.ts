/**
 * 賣家工作台四張狀態卡 / tab 徽章的數字（來源＝後端 dashboard，03 §2.1 SSOT）。
 * 顯示名一律取 `features/seller-center/orders/orderTabs.ts` 的 `SELLER_ORDER_TAB_LABEL`。
 */
export interface SellerOrderTabCounts {
	pending: number | null;
	in_transit: number | null;
	/** 「已結束」桶（賣家無需再動作）。⚠️ 含 order.status 80 已取消、70 全額退款，
	 * **不是成交數**；wire 欄位是 `order_status_counts.closed_count`，語意以本註解為準。 */
	completed: number | null;
	aftersale: number | null;
}

export function formatNullableCount(value: number | null): string {
	return value == null ? "—" : value.toLocaleString();
}
