"use client";

import * as React from "react";
import Link from "@/components/shared/Link";
import type { SellerMeasure } from "@/lib/seller/measures";
import { remediationEntry, remediationHref } from "./measureDisplay";

/** 措施卡片上的次要動作（申覆、請求解除）的樣式。 */
export const MEASURE_ACTION_CLASS =
	"inline-flex h-8 items-center rounded-[var(--hkui-radius-md)] border border-[var(--hkui-primary)] px-3 text-xs font-semibold text-[var(--hkui-primary)] transition-colors hover:bg-[var(--hkui-primary-50)] focus-visible:!outline-none focus-visible:![box-shadow:var(--hkui-focus-ring)]";

export const REMEDIATION_BUTTON_LABEL = "已改善，請求解除";

/**
 * 解除請求入口：可以用時是連到解除請求頁的連結，不能用時是反灰的按鈕並附原因（審核中或冷卻到某時刻）。
 * 伺服器說不是 RISK、不是生效中的措施不渲染。能不能用與原因都讀伺服器的欄位，client 不推導。
 */
export default function RemediationEntry({
	measure,
	buttonClassName,
	solid = false,
}: {
	measure: SellerMeasure;
	buttonClassName: string;
	/** `buttonClassName` 是實心底的樣式：連結要帶 `data-hkui-button`，dark 下白字才不會被換色。 */
	solid?: boolean;
}) {
	const reasonId = React.useId();
	const entry = remediationEntry(measure);
	if (!entry) return null;
	return (
		<>
			{entry.reason && (
				<span
					id={reasonId}
					className="text-xs text-[var(--hkui-foreground-muted)]"
				>
					{entry.reason}
				</span>
			)}
			{entry.enabled ? (
				<Link
					href={remediationHref(measure)}
					className={buttonClassName}
					{...(solid ? { "data-hkui-button": "" } : {})}
				>
					{REMEDIATION_BUTTON_LABEL}
				</Link>
			) : (
				<button
					type="button"
					disabled
					aria-describedby={entry.reason ? reasonId : undefined}
					className={`${buttonClassName} disabled:cursor-not-allowed disabled:opacity-50`}
				>
					{REMEDIATION_BUTTON_LABEL}
				</button>
			)}
		</>
	);
}
