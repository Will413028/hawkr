"use client";

import type { ReactNode } from "react";
import { InvoiceModeBadge } from "@/features/invoices/components/InvoiceModeBadge";
import { MaskedValue } from "@/features/invoices/components/MaskedValue";
import { INVOICE_CHECKOUT_TEXT } from "@/features/invoices/i18n";
import { useOrderInvoice } from "@/lib/hooks/useOrderInvoice";
import {
	CHECKOUT_AWAITING_INVOICE_NOTICE,
	checkoutInvoiceCarrier,
	checkoutPaidInvoiceNotice,
} from "@/lib/invoices/checkoutInvoiceNotice";

const T = INVOICE_CHECKOUT_TEXT;

const ROW_CLASS =
	"flex justify-between items-center gap-3 border-t border-divider pt-3";
const LABEL_CLASS = "text-sm font-medium text-foreground-muted";
const VALUE_CLASS = "text-sm font-bold text-foreground";

/**
 * 錢的狀態決定狀態句從哪裡來，所以由呼叫頁指定而不是由 `state` 推導。
 *
 * `paid`（`/checkout/success`）走 `invoiceMode` 的四句話；`awaiting-payment`
 * （`/checkout/submitted`，ATM 待轉帳）逐字顯示後端的 `stateNotice`——那三句是
 * 「付款成功」情境的話，錢還沒到就套用是錯的。
 */
type CheckoutInvoiceVariant = "paid" | "awaiting-payment";

function Row({ label, children }: { label: string; children: ReactNode }) {
	return (
		<div className={ROW_CLASS}>
			<span className={LABEL_CLASS}>{label}</span>
			{children}
		</div>
	);
}

/**
 * B03｜付款完成頁的發票三列（58A §B03）。
 *
 * 掛在 `/checkout/success` 與 `/checkout/submitted` 既有資訊卡的「應付金額」列之後，
 * 每列與既有列同樣以 `border-t border-divider pt-3` 分隔。`/checkout/fail` **不掛**
 * ——付款失敗沒有發票可談。
 *
 * 三件事值得先讀：
 *
 *   1. **失敗一律靜默降級成不渲染**（本部分唯一一處）。這一頁的工作是告知「錢付了、
 *      單成立了」；發票查詢 5xx 不該在這裡製造焦慮，使用者到訂單詳情就會看到完整
 *      狀態。9301（沒有發票／不是你的訂單）同樣不渲染，且不得出現任何「沒有權限」
 *      字樣——端點 8 是 Tier A，越權與不存在回同一個碼，前端分得出來就等於把那個
 *      旗標補回去。
 *   2. **零刷新**。這是一次性落地頁，不訂閱 SSE、不輪詢、不給重新整理鍵；
 *      `useOrderInvoice` 自帶的 `refetchOnWindowFocus` 是 react-query 的既有行為，
 *      不是本頁另外加的輪詢。
 *   3. **不得把發票資料寫進 localStorage**（含買受人統編與載具識別碼）——`?order=`
 *      是網址上的可控參數，落地就等於把個資留在裝置上。本元件只讀 react-query 的
 *      記憶體快取。
 */
export function CheckoutInvoiceRows({
	orderNo,
	variant,
}: {
	orderNo: string;
	variant: CheckoutInvoiceVariant;
}) {
	// 刻意不取 `error`：5xx 與網路失敗在這一頁的處置與「還沒查到」完全相同
	// （不渲染），拿到手上反而會誘使人在這裡加一段錯誤訊息。
	const { invoice, loading, notFound } = useOrderInvoice(orderNo);

	// 9301：這筆訂單沒有發票（零元訂單天天都有），或不是你的訂單。整段不渲染。
	if (notFound) return null;

	if (!invoice) {
		// 查詢中：值顯示 `—` 而不是 skeleton（既有付款頁 loading 就是顯示字串，且
		// 資訊卡列高固定）。
		//
		// ⚠️ **只佔位「發票處理狀態」這一列**，刻意偏離 58A §B03「三列 value 顯示
		// `—`」的字面。理由是那句話自己給的：佔位的目的是「不會跳動」。而另外兩列
		// 在資料到手後都可能整列不渲染（載具在端點 8 只能從 `documents[]` 取，付款
		// 完成當下多半是空的；通知信箱為空代表還沒寄成功）⇒ 照字面先畫三列，結果
		// 必然是三列縮成一列，正是那句話要避免的事。只有這一列在 ready 態一定在，
		// 佔位它才真的守住了不跳動。
		if (loading) {
			return (
				<Row label={T.fieldState}>
					<span className={VALUE_CLASS}>{T.pendingValue}</span>
				</Row>
			);
		}
		// 5xx／網路失敗與「還沒開始查」都走這裡：不渲染，不報錯。
		return null;
	}

	const notice =
		variant === "paid"
			? checkoutPaidInvoiceNotice(invoice.invoiceMode)
			: invoice.stateNotice || CHECKOUT_AWAITING_INVOICE_NOTICE;
	const carrier = checkoutInvoiceCarrier(invoice.documents);

	return (
		<>
			<Row label={T.fieldState}>
				<span
					className={`${VALUE_CLASS} flex flex-wrap items-center justify-end gap-2 min-w-0 text-right`}
				>
					<InvoiceModeBadge mode={invoice.invoiceMode} />
					{notice}
				</span>
			</Row>
			{/* 端點 8 沒有訂單層級的載具欄位，唯一來源是 documents[].carrierSummary，
			    而付款完成當下 documents 多半是空的 ⇒ 取不到就整列不渲染，而不是印一個
			    猜出來的「無載具」——那會讓有載具的買家以為自己的載具沒帶上。 */}
			{carrier ? (
				<Row label={T.fieldCarrier}>
					<MaskedValue
						value={carrier}
						label={T.fieldCarrier}
						className={`${VALUE_CLASS} truncate min-w-0`}
					/>
				</Row>
			) : null}
			{/* 為空代表通知還沒寄成功 ⇒ 那一列整列不渲染。 */}
			{invoice.notifiedEmailMasked ? (
				<Row label={T.fieldNotifiedEmail}>
					<MaskedValue
						value={invoice.notifiedEmailMasked}
						label={T.fieldNotifiedEmail}
						className={`${VALUE_CLASS} truncate min-w-0`}
					/>
				</Row>
			) : null}
		</>
	);
}

export default CheckoutInvoiceRows;
