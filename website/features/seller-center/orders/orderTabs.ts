import {
	AFTER_SALE_STATUS,
	type AfterSaleCase,
	afterSaleIsTerminal,
} from "@/lib/after-sales";
import type { Order } from "@/lib/orders/types";
import { isTcatCalled } from "@/lib/seller/shipment";

export type SellerOrderTabKey =
	| "pending"
	| "in_transit"
	| "completed"
	| "aftersale";

const PENDING_STATUSES = new Set(["paid", "preparing"]);
const IN_TRANSIT_STATUSES = new Set([
	"shipping",
	"waiting_pickup",
	"shipping_abnormal",
	"delivered",
]);
/** 第三桶的成員 — 注意 `cancelled` / `refunded` 也在裡面，所以它**不是**成交數。
 * 桶的語意是「賣家無需再動作」，顯示名見 {@link SELLER_ORDER_TAB_LABEL}。 */
const COMPLETED_STATUSES = new Set([
	"completed",
	"cancelled",
	"refunded",
	"partially_refunded",
	"after_sale",
]);

/**
 * 四個桶的顯示名 —— 狀態卡、tab 徽章與 `TAB_DEFS` 共用這一份，不得各自寫字面值。
 *
 * ⚠️ 第三桶叫「已結束」而**不是**「已完成」：它含 `cancelled` / `refunded`
 * （後端 order.status 80 已取消、70 全額退款，見 `COMPLETED_STATUSES` 與 03 §2.1 SSOT），
 * 叫「已完成」會讓賣家把它讀成成交數 —— dev 實測同一家店 81 筆時這張卡顯示 75，
 * 其中只有 19 筆真的成交、56 筆是已取消。「已完成」是 CodeMeta `OrderStatus` 50
 * 單一狀態的 Label，只在按狀態逐一計數的代行工作區「賣家總覽」才成立。
 *
 * 這份 map 與 `COMPLETED_STATUSES` 刻意放在同一個檔案：桶的成員與桶的名字要一起被看到，
 * 否則下一個人只看到集合就會自行編一個名字（這正是本次 bug 的來源）。
 * App 側對應 `lib/features/order/data/order_tab_mapper.dart` 的 `sellerOrderTabLabels`。
 */
export const SELLER_ORDER_TAB_LABEL: Readonly<
	Record<SellerOrderTabKey, string>
> = {
	pending: "待出貨",
	in_transit: "配送中",
	completed: "已結束",
	aftersale: "售後處理",
};

const NON_CLASSIFYING_AFTER_SALE_STATUSES = new Set([
	AFTER_SALE_STATUS.REJECTED,
	AFTER_SALE_STATUS.CANCELLED,
]);

export function afterSaleOverridesOrderLifecycle(
	afterSale: AfterSaleCase | undefined,
): afterSale is AfterSaleCase {
	return Boolean(
		afterSale &&
			!NON_CLASSIFYING_AFTER_SALE_STATUSES.has(afterSale.status as never),
	);
}

/**
 * Seller order tabs are order-lifecycle first, with after-sale cases layered
 * on top. An active after-sale case belongs to the 售後處理 case table, not the
 * generic 配送中 list. Once the case is terminal, a still-delivered order should
 * no longer inflate 配送中.
 */
export function sellerOrderTabFor(
	order: Order,
	afterSale?: AfterSaleCase,
): SellerOrderTabKey | null {
	if (afterSaleOverridesOrderLifecycle(afterSale)) {
		return afterSaleIsTerminal(afterSale) ? "completed" : "aftersale";
	}

	if (isTcatCalled(order.shipping)) return "in_transit";
	if (PENDING_STATUSES.has(order.status)) return "pending";
	if (IN_TRANSIT_STATUSES.has(order.status)) return "in_transit";
	if (COMPLETED_STATUSES.has(order.status)) return "completed";
	return null;
}
