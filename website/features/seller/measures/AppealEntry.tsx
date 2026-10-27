"use client";

import * as React from "react";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { useUiText } from "@/lib/i18n/ui-text";
import type { SellerMeasure } from "@/lib/seller/measures";
import { appealEntry, appealHref } from "./measureDisplay";

export const APPEAL_BUTTON_LABEL = "提出申覆";

/**
 * 申覆入口：一律顯示。能用時是連到申覆頁的連結，不能用時是反灰的按鈕並附伺服器給的原因；終止合作另附申覆期限。
 * 能不能用、原因與期限都讀伺服器的欄位（`canAppeal`、`appealUnavailabilities`、`appealDeadlineTime`），client 不推導。
 * 樣式二選一：`buttonClassName`（沿用解除請求入口的次要動作樣式；`solid` 為實心底，連結要帶 `data-hkui-button`），
 * 或不傳 `buttonClassName` 用主要按鈕（公告頁）。
 */
export default function AppealEntry({
	measure,
	buttonClassName,
	solid = false,
}: {
	measure: SellerMeasure;
	buttonClassName?: string;
	solid?: boolean;
}) {
	const noteId = React.useId();
	const ut = useUiText();
	const entry = appealEntry(ut, measure);
	const note = [entry.reason, entry.deadline].filter(Boolean).join(" · ");
	return (
		<>
			{note && (
				<span
					id={noteId}
					className="text-xs text-[var(--hkui-foreground-muted)]"
				>
					{note}
				</span>
			)}
			{buttonClassName === undefined ? (
				entry.enabled ? (
					<Button href={appealHref(measure)} className="w-fit">
						{APPEAL_BUTTON_LABEL}
					</Button>
				) : (
					<Button
						disabled
						className="w-fit"
						aria-describedby={note ? noteId : undefined}
					>
						{APPEAL_BUTTON_LABEL}
					</Button>
				)
			) : entry.enabled ? (
				<Link
					href={appealHref(measure)}
					className={buttonClassName}
					{...(solid ? { "data-hkui-button": "" } : {})}
				>
					{APPEAL_BUTTON_LABEL}
				</Link>
			) : (
				<button
					type="button"
					disabled
					aria-describedby={note ? noteId : undefined}
					className={`${buttonClassName} disabled:cursor-not-allowed disabled:opacity-50`}
				>
					{APPEAL_BUTTON_LABEL}
				</button>
			)}
		</>
	);
}
