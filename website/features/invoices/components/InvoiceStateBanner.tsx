import type * as React from "react";
import { foldOrderInvoiceState } from "@/lib/invoices/orderInvoiceState";

const TONE_CLASS = {
	info: "border-info-200 bg-info-50 text-info-700",
	warning: "border-warning-200 bg-warning-50 text-warning-700",
	danger: "border-danger-200 bg-danger-50 text-danger-700",
} as const;

interface InvoiceStateBannerProps {
	/** 端點 8 的 `state`（買家可見投影 8 值）。 */
	state: string;
	/** 端點 8 的 `stateNotice`。**逐字顯示，前端不自組這句**。 */
	notice: string;
	/** `PROCESSING` 時的「重新整理」文字鍵；其餘狀態不渲染。 */
	onRefresh?: () => void;
	/** 橫幅底部的動作（例：提出申訴）。 */
	actions?: React.ReactNode;
}

/**
 * B04 的狀態橫幅（58A §B04 八態表）。
 *
 * 樣態折疊在 `lib/invoices/orderInvoiceState.ts` 一處（含 default 分支）；本檔
 * 只負責把它畫出來。`ISSUED`／`VOIDED`／`ALLOWANCE_APPLIED` 折疊結果是 `null`
 * ⇒ 整個橫幅不渲染，狀態由文件卡自己的 Chip 表達。
 *
 * 標題句必有 `<strong>`：狀態不可只靠顏色表達（§B-1.0.8）。
 */
export function InvoiceStateBanner({
	state,
	notice,
	onRefresh,
	actions,
}: InvoiceStateBannerProps) {
	const banner = foldOrderInvoiceState(state);
	if (!banner) return null;
	return (
		<section
			className={`rounded-hkui-md border p-4 text-sm ${TONE_CLASS[banner.tone]}`}
			role={banner.role}
		>
			<strong className="block text-base">{banner.title}</strong>
			{notice ? <p className="m-0 mt-1 leading-relaxed">{notice}</p> : null}
			{banner.showRefresh && onRefresh ? (
				<button
					type="button"
					className="mt-2 inline-flex min-h-[44px] items-center bg-transparent p-0 text-sm font-semibold underline underline-offset-2"
					onClick={onRefresh}
				>
					重新整理
				</button>
			) : null}
			{actions ? (
				<div className="mt-3 flex flex-wrap gap-2">{actions}</div>
			) : null}
		</section>
	);
}
