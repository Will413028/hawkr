import {
	ORDER_CANCEL_REASON_LABEL,
	ORDER_STATUS_LABEL,
	OrderCancelByRole,
	OrderStatus,
	PAYMENT_METHOD_LABEL,
	SHIPPING_METHOD_LABEL,
	SHIPPING_STATUS_LABEL,
	SHIPPING_TEMPERATURE_LABEL,
} from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { type DelegatedOrderFilters, hasDelegatedOrderFilter } from "./filters";

/**
 * 代行訂單面的顯示層（mockup §4.8 ＋ 照抄禁區 D5）。
 *
 * ⚠️ **D5**：mockup 上的狀態文字（`待處理`／`配送中`／`已完成`／`異常`）是意譯，
 * 一律以 CodeMeta 的 Label 為準 —— `OrderStatus` 20 是 **`待出貨`** 不是「待處理」。
 * 而且**優先用後端下發的 `statusLabel`**：後端已經在 DTO 裡送了它，前端另建一份
 * 對照表就會在後端新增狀態時顯示成空白或代碼（L3549 同源裁決）。這裡的
 * `ORDER_STATUS_LABEL` 只是後端沒送 label 時的退路。
 */

/** 一律先用後端 label；退路才查 CodeMeta；再不行就把碼原樣顯示（不編字）。 */
export function orderStatusText(status: string, statusLabel: string): string {
	if (statusLabel) return statusLabel;
	return ORDER_STATUS_LABEL[status] ?? status;
}

/**
 * 物流狀態。與 `orderStatusText` 同一個優先序：後端 label → CodeMeta → 原碼。
 *
 * ⚠️ **空字串是合法值**（還沒建立物流單的訂單，`shippingStatus` 就是空的），此時回空
 * 字串讓呼叫端顯示 `—`。不得退到 `ShippingStatus.PENDING_SHIPMENT`（`待出貨`）之類的
 * 猜測值：那個字面與訂單狀態的 `待出貨` 一模一樣，操作者會以為物流單已經建好了，而
 * 「卡在哪」正是這一欄存在的理由。
 */
export function shippingStatusText(
	status: string,
	statusLabel: string,
): string {
	if (statusLabel) return statusLabel;
	if (!status) return "";
	return SHIPPING_STATUS_LABEL[status] ?? status;
}

/**
 * 狀態 → 既有 `.seller-status--*` 的色調。沿用賣家中心的四個 modifier，不新增色面。
 */
export function orderStatusTone(
	status: string,
): "pending" | "shipping" | "complete" | "cancelled" {
	switch (status) {
		case OrderStatus.PENDING_PAYMENT:
		case OrderStatus.PAID:
		case OrderStatus.PENDING_SHIPMENT:
			return "pending";
		case OrderStatus.IN_TRANSIT:
		case OrderStatus.WAITING_PICKUP:
		case OrderStatus.DELIVERED:
			return "shipping";
		case OrderStatus.COMPLETED:
			return "complete";
		default:
			// 物流異常／退款／取消都走同一個「需要注意」的色調。
			return "cancelled";
	}
}

/**
 * 終態：這筆訂單不會再有任何履約動作，**對誰都不會有**。
 *
 * CodeMeta `OrderStatus` 標了「（終態）」的恰為三個。判準寫成清單而不是「> 某個數字」：
 * 60 部分退款排在 50 已完成之後卻不是終態，用大小比較會把它一起劃進來。
 */
const TERMINAL_ORDER_STATUSES: ReadonlySet<string> = new Set<string>([
	OrderStatus.COMPLETED,
	OrderStatus.FULLY_REFUNDED,
	OrderStatus.CANCELLED,
]);

export function isTerminalOrderStatus(status: string): boolean {
	return TERMINAL_ORDER_STATUSES.has(status);
}

/**
 * 取消說明：`{角色}取消訂單：{原因}：{備註}`。三段都可能缺，缺了就不出現。
 *
 * 與賣家中心的 `OrderStatusTimeline` 同一組來源（`ORDER_CANCEL_REASON_LABEL` 由
 * CodeMeta 生成），措辭刻意對齊 —— 代行者與賣家看同一筆訂單時，讀到的取消原因必須是
 * 同一句話，否則兩邊對話會對不上。
 *
 * 買家自願取消未付款訂單時 `cancelReason` 與 `cancelRemark` 皆為 NULL（CodeMeta
 * `OrderCancelReason` 的說明），此時只剩「買家取消訂單」這個標題，仍值得顯示。
 */
export function formatCancelNote(
	cancelByRole: string,
	cancelReason: string,
	cancelRemark: string,
): string {
	const title = cancelActorTitle(cancelByRole);
	const reason = cancelReason
		? (ORDER_CANCEL_REASON_LABEL[cancelReason] ?? cancelReason)
		: "";
	const detail = [reason, informativeRemark(cancelRemark, title, reason)]
		.filter(Boolean)
		.join("：");
	return detail ? `${title}：${detail}` : title;
}

/**
 * 備註裡**還沒被前面兩段講過**的部分。全部講過就回空字串。
 *
 * ⚠️ 後端的系統取消會把角色與原因再抄一次進備註：`SellerCancelService:220` 是
 * `"系統自動取消：" + cancelReason.label()`，於是 dev 實測畫面上出現
 * 「系統自動取消訂單：出貨逾時自動取消：系統自動取消：出貨逾時自動取消」——
 * 同一句話講兩遍，讀的人會以為這筆訂單被取消了兩次。
 *
 * 判準不是「備註 === 原因」（那只擋得住完全相等，而後端存的是**超集**），而是逐段
 * 比對：以全／半形冒號與分號切開備註，丟掉「與原因相同」或「已被標題涵蓋」的段落，
 * 剩下的才是新資訊。這樣賣家手寫的備註（「颱風停止出貨，已與買家溝通」）一個字都
 * 不會被吃掉 —— 它切不出任何一段對得上標題或原因。
 */
function informativeRemark(
	cancelRemark: string,
	title: string,
	reason: string,
): string {
	const remark = cancelRemark.trim();
	if (!remark) return "";
	const kept = remark
		.split(/[：:；;]/)
		.map((segment) => segment.trim())
		.filter(
			(segment) =>
				segment !== "" && segment !== reason && !title.includes(segment),
		);
	return kept.length === 0 ? "" : kept.join("：");
}

function cancelActorTitle(cancelByRole: string): string {
	switch (cancelByRole) {
		case OrderCancelByRole.BUYER:
			return "買家取消訂單";
		case OrderCancelByRole.SELLER:
			return "賣家取消訂單";
		case OrderCancelByRole.SYSTEM:
			return "系統自動取消訂單";
		default:
			return "訂單已取消";
	}
}

/**
 * 訂單列表零筆時的文案。**有篩選條件時不得說「目前沒有訂單」。**
 *
 * dev 實測這家店「全部訂單 81」，篩「部分退款」零命中，畫面卻是變體預設的
 * 「目前沒有訂單／買家下單後會自動出現在這裡」—— 那是在替目標店家宣稱它還沒開始賣，
 * 而正確的下一步（清掉篩選）也就想不到。回 `null` 代表「真的沒有篩選」，呼叫端就用
 * 變體預設文案。
 *
 * ⚠️ 判準吃的是**整組**條件不是只有 `status`：關鍵字打錯一個字、日期範圍選到店家還沒
 * 開張的那一週，同樣會落到零筆，而那兩種情況下 `status` 是空的 —— 只看 `status` 就會
 * 退回「這家店沒有訂單」，正是上面那個錯誤換一個外殼再犯一次。
 */
export function delegatedOrdersEmptyCopy(
	filters: DelegatedOrderFilters,
): { title: string; description: string } | null {
	if (!hasDelegatedOrderFilter(filters)) return null;
	const conditions: string[] = [];
	if (filters.status) {
		conditions.push(`狀態為「${orderStatusText(filters.status, "")}」`);
	}
	const keyword = filters.keyword.trim();
	if (keyword) conditions.push(`符合「${keyword}」`);
	const range = dateRangeText(filters.dateFrom, filters.dateTo);
	if (range) conditions.push(range);
	return {
		title: "目前的篩選條件沒有符合的訂單",
		description: `沒有${conditions.join("、")}的訂單。清除或放寬上方的篩選條件再看一次。`,
	};
}

/** 起訖兩端都可能單獨成立，三種講法都要有；全空回空字串。 */
function dateRangeText(dateFrom: string, dateTo: string): string {
	if (dateFrom && dateTo) return `建立於 ${dateFrom} 至 ${dateTo}`;
	if (dateFrom) return `建立於 ${dateFrom} 之後`;
	if (dateTo) return `建立於 ${dateTo} 之前`;
	return "";
}

/** `"2480"` → `"NT$ 2,480"`。空值回 `"NT$ 0"`（金額是必有欄位，不是可選資訊）。 */
export function formatOrderAmount(raw: string): string {
	return `NT$ ${Number(raw || 0).toLocaleString()}`;
}

/** ISO 8601（UTC）→ 台北時間。空值回空字串，呼叫端據此不渲染那一行。 */
export function formatOrderDateTime(raw: string): string {
	return raw ? formatTaipeiDateTime(raw) : "";
}

export function shippingMethodText(code: string): string {
	return code ? (SHIPPING_METHOD_LABEL[code] ?? "") : "";
}

export function deliveryTempText(code: string): string {
	return code ? (SHIPPING_TEMPERATURE_LABEL[code] ?? "") : "";
}

export function paymentMethodText(code: string): string {
	return code ? (PAYMENT_METHOD_LABEL[code] ?? "") : "";
}

/** 買家頭像的單字。拿不到名字時回空字串 —— 不用「?」之類的佔位字元。 */
export function buyerInitial(displayName: string): string {
	return displayName.trim().slice(0, 1);
}

/**
 * 列表商品欄的第二行（「等 N 件」）該不該出現。
 *
 * `itemCount` 是**品項數**（order-service 送的是 `items.size()`，不是數量加總），與
 * `firstProductName` 同源 —— 所以 N > 1 才代表「第一件之外還有別的」。
 *
 * 走字串轉數字而不是 `itemCount !== "1"`：這一欄是契約規定的數字字串，而空字串、
 * `"0"`、非數字都必須落在「不顯示」那一邊。`Number("") === 0` 與 `Number.NaN > 1 === false`
 * 兩者都已經是安全的那一側，這裡把它寫明是為了擋住日後有人改成 `!== "1"`——那會讓
 * 一筆聚合失敗（空字串）的訂單長出一行「等  件」。
 */
export function hasMoreThanOneItem(itemCount: string): boolean {
	return Number(itemCount) > 1;
}

/**
 * KPI 四張卡（mockup §4.8.3）。
 *
 * 每一張綁**單一** `OrderStatus` 碼：一張卡一個 status filter，數字就是那支列表的
 * `total`，不做任何前端加總 —— 加總會讓卡片數與點進去看到的列數對不上。
 * 標籤一律取 CodeMeta Label（D5），所以圖上的「待處理」在這裡是「待出貨」、
 * 「異常」是「物流異常」。
 */
export const DELEGATED_ORDER_STAT_CARDS = [
	// ⚠️ 第一張是「全部」（`status` 空字串＝不帶篩選）。少了它，四張單一狀態的卡在
	// 一家沒有進行中訂單的店裡會全部是 0，而列表就在下面滿滿一頁 —— 操作者看到的是
	// 「統計壞了」，實際上四張卡加起來本來就涵蓋不了已取消／退款／待付款那些狀態。
	// 它不多打一支端點類型：同樣是列表 `total`（B15 明列的唯一合法作法）。
	{ status: "", label: "全部訂單", icon: "document", tint: "primary" },
	{ status: OrderStatus.PENDING_SHIPMENT, icon: "bag", tint: "accent" },
	{ status: OrderStatus.IN_TRANSIT, icon: "truck", tint: "info" },
	{ status: OrderStatus.COMPLETED, icon: "check", tint: "info" },
	{ status: OrderStatus.SHIPPING_ABNORMAL, icon: "danger", tint: "secondary" },
] as const;

/** 篩選下拉的選項：全部 ＋ CodeMeta 的全部 `OrderStatus`，順序照生命週期。 */
export const DELEGATED_ORDER_STATUS_OPTIONS: ReadonlyArray<{
	value: string;
	label: string;
}> = [
	{ value: "", label: "全部狀態" },
	...Object.values(OrderStatus).map((code) => ({
		value: code,
		label: ORDER_STATUS_LABEL[code] ?? code,
	})),
];
