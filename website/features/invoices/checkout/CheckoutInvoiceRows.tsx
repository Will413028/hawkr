"use client";

import type { ReactNode } from "react";
import { InvoiceModeBadge } from "@/features/invoices/components/InvoiceModeBadge";
import { MaskedValue } from "@/features/invoices/components/MaskedValue";
import { INVOICE_CHECKOUT_TEXT } from "@/features/invoices/i18n";
import { useOrderInvoice } from "@/lib/hooks/useOrderInvoice";
import {
	checkoutInvoiceCarrier,
	checkoutPaidInvoiceNotice,
} from "@/lib/invoices/checkoutInvoiceNotice";

const T = INVOICE_CHECKOUT_TEXT;

const ROW_CLASS =
	"flex justify-between items-center gap-3 border-t border-divider pt-3";
const LABEL_CLASS = "text-sm font-medium text-foreground-muted";
const VALUE_CLASS = "text-sm font-bold text-foreground";

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
 * 掛在 `/checkout/success` 既有資訊卡的「應付金額」列之後，
 * 每列與既有列同樣以 `border-t border-divider pt-3` 分隔。
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
export function CheckoutInvoiceRows({ orderNo }: { orderNo: string }) {
	const { invoice, loading, notFound } = useOrderInvoice(orderNo);

	if (notFound) return null;

	if (!invoice) {
		if (loading) {
			return (
				<Row label={T.fieldState}>
					<span className={VALUE_CLASS}>{T.pendingValue}</span>
				</Row>
			);
		}
		return null;
	}

	const notice = checkoutPaidInvoiceNotice(invoice.invoiceMode);
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

			{carrier ? (
				<Row label={T.fieldCarrier}>
					<MaskedValue
						value={carrier}
						label={T.fieldCarrier}
						className={`${VALUE_CLASS} truncate min-w-0`}
					/>
				</Row>
			) : null}

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
