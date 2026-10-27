/**
 * 買家申訴的**賣家側**呈現：狀態 → Chip 顏色、三個分頁視圖、⑥ 是否給寫入入口。
 *
 * 🔴 **與買家面的 `lib/invoices/complaintStatus.ts` 是兩張不同的表，不可共用。**
 * 那一份回的是「發生了什麼、你現在該做什麼」的兩句買家文案，而且刻意讓
 * `RESOLVED` 與 `VIOLATION_CONFIRMED` 顯示成同一件事（買家面不得出現「違規」二字）。
 * 賣家面相反：`VIOLATION_CONFIRMED` 是平台對他自己的治理結論，折掉它等於讓賣家
 * 不知道自己被記了一次違規。所以本檔取 CodeMeta 的 7 個原值標籤。
 *
 * 🔴 中文一律取自 CodeMeta（§0.4 ⑤），前端只寫「值 → 顏色」那一半，且必須有
 * default 分支（§B-2.0 T13）。
 */

import type { ChipColor } from "@/components/ui/Chip";
import {
	INVOICE_COMPLAINT_STATUS_LABEL,
	InvoiceComplaintStatus,
} from "@/lib/codemeta/enums";

export interface InvoiceComplaintDisplay {
	label: string;
	color: ChipColor;
	/** 落到 default 分支（後端值域比前端新）時為 true，供除錯與測試辨識。 */
	unknown: boolean;
}

const COLOR_BY_STATUS: Record<string, ChipColor> = {
	// 該你回了
	[InvoiceComplaintStatus.OPEN]: "warning",
	[InvoiceComplaintStatus.OVERDUE]: "danger",
	// 你已經做完你該做的
	[InvoiceComplaintStatus.SELLER_RESPONDED]: "primary",
	[InvoiceComplaintStatus.ESCALATED_TO_ADM]: "primary",
	// 已結案
	[InvoiceComplaintStatus.RESOLVED]: "success",
	[InvoiceComplaintStatus.VIOLATION_CONFIRMED]: "danger",
	[InvoiceComplaintStatus.WITHDRAWN]: "default",
};

export function invoiceComplaintDisplay(
	status: string,
): InvoiceComplaintDisplay {
	const color = COLOR_BY_STATUS[status];
	if (!color) {
		return { label: status || "—", color: "default", unknown: true };
	}
	return {
		label: INVOICE_COMPLAINT_STATUS_LABEL[status] ?? status,
		color,
		unknown: false,
	};
}

/**
 * S09 的三個分頁。
 *
 * 🔴 `value` 是**視圖名不是 DB 狀態值**（端點 34 的 `status` query 吃的是視圖名，
 * 後端 `sellerComplaintViews` 再展開成 DB 值）。兩套字串不可互相代入，送 DB 值
 * 進去會拿到 400。
 *
 * 三視圖互斥且窮盡（7 個 DB 值全部落在且只落在一個視圖）：
 *   AWAITING_SELLER   ← OPEN, OVERDUE
 *   SELLER_RESPONDED  ← SELLER_RESPONDED, ESCALATED_TO_ADM
 *   CLOSED            ← RESOLVED, VIOLATION_CONFIRMED, WITHDRAWN
 *
 * `OVERDUE` 刻意留在「待處理」而不是自成第四個分頁——那會讓逾期件從「待回覆」
 * 消失，而那正是最該被看到的幾件。
 */
export const SELLER_COMPLAINT_VIEWS = [
	{ value: "AWAITING_SELLER", label: "待處理" },
	{ value: "SELLER_RESPONDED", label: "已回覆" },
	{ value: "CLOSED", label: "已結案" },
] as const;

export type SellerComplaintView =
	(typeof SELLER_COMPLAINT_VIEWS)[number]["value"];

export const DEFAULT_SELLER_COMPLAINT_VIEW: SellerComplaintView =
	"AWAITING_SELLER";

/** URL 的 `?tab=` 只接受三個視圖名，其餘一律退回預設（不把亂填的字串送進 API）。 */
export function normalizeSellerComplaintView(raw: string): SellerComplaintView {
	const hit = SELLER_COMPLAINT_VIEWS.find((view) => view.value === raw);
	return hit ? hit.value : DEFAULT_SELLER_COMPLAINT_VIEW;
}

/**
 * 「這件申訴現在還等著賣家動手嗎」。
 *
 * 🔴 判準必須是 **OR**：`status === OVERDUE` **或** `remainingBusinessDays` 為負。
 * `OVERDUE` 由後端掃描寫入，掃描與到期之間有一段窗口——此時 `dueAt` 已過但狀態
 * 還是 `OPEN`（§S09 的「逾期」態）。
 */
export function isComplaintOverdue(
	status: string,
	remainingBusinessDays: string,
): boolean {
	if (status === InvoiceComplaintStatus.OVERDUE) return true;
	if (remainingBusinessDays === "") return false;
	const remaining = Number(remainingBusinessDays);
	return Number.isFinite(remaining) && remaining < 0;
}

/** 已結案的三個狀態（⑥ 不渲染、② 換成結案句）。 */
export function isComplaintClosed(status: string): boolean {
	return (
		status === InvoiceComplaintStatus.RESOLVED ||
		status === InvoiceComplaintStatus.VIOLATION_CONFIRMED ||
		status === InvoiceComplaintStatus.WITHDRAWN
	);
}
