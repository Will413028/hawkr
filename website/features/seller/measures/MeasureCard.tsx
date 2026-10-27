"use client";

import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { type UiText, useUiText } from "@/lib/i18n/ui-text";
import type { SellerMeasure } from "@/lib/seller/measures";
import AppealEntry from "./AppealEntry";
import {
	measureMeta,
	measurePeriodText,
	measureStatusChips,
	measureTitle,
	requestNote,
	revokeNote,
} from "./measureDisplay";
import RemediationEntry, { MEASURE_ACTION_CLASS } from "./RemediationEntry";

/** 說明行：期間、撤銷原因與請求進度，依序，沒有值的不顯示。 */
export function measureNotes(ut: UiText, measure: SellerMeasure): string[] {
	return [
		measurePeriodText(ut, measure),
		revokeNote(ut, measure),
		requestNote(ut, measure),
	].filter((note): note is string => Boolean(note));
}

/** 措施卡片：依據、措施、狀態、平台給賣家的理由與申覆、解除請求入口。列表與詳情頁共用；申覆與解除請求入口連到各自的獨立頁。 */
export default function MeasureCard({ measure }: { measure: SellerMeasure }) {
	const ut = useUiText();
	const chips = measureStatusChips(ut, measure);
	const meta = measureMeta(ut, measure);

	return (
		<Card className="flex flex-col gap-3 p-5">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div className="flex flex-col gap-1">
					<strong className="text-[0.9375rem] text-[var(--hkui-foreground)]">
						{measureTitle(ut, measure)}
					</strong>
					{meta && (
						<span className="text-xs text-[var(--hkui-foreground-muted)]">
							{meta}
						</span>
					)}
				</div>
				{chips.length > 0 && (
					<div className="flex flex-wrap items-center gap-2">
						{chips.map((chip) => (
							<Chip
								key={chip.label}
								variant="flat"
								color={chip.color}
								size="sm"
							>
								{chip.label}
							</Chip>
						))}
					</div>
				)}
			</div>

			{measure.statement && (
				<p className="m-0 text-sm leading-[1.6] text-[var(--hkui-foreground-soft)]">
					{measure.statement}
				</p>
			)}

			{measureNotes(ut, measure).map((note) => (
				<p
					key={note}
					className="m-0 text-xs leading-[1.6] text-[var(--hkui-foreground-muted)]"
				>
					{note}
				</p>
			))}

			<div className="flex flex-wrap items-center justify-end gap-3">
				<RemediationEntry
					measure={measure}
					buttonClassName={MEASURE_ACTION_CLASS}
				/>
				<AppealEntry measure={measure} buttonClassName={MEASURE_ACTION_CLASS} />
			</div>
		</Card>
	);
}
