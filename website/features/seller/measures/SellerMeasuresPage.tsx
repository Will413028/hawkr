"use client";

import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { SellerEmptyState } from "@/features/seller-center/components/SellerEmptyState";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import { SellerMobileDisclosureSkeleton } from "@/features/seller-center/components/SellerPageSkeletons";
import { SellerCardGridSkeleton } from "@/features/seller-center/components/SellerSkeletons";
import { useSellerMeasures } from "@/lib/hooks/useSellerMeasures";
import { useUiText } from "@/lib/i18n/ui-text";
import type { SellerMeasure } from "@/lib/seller/measures";
import AppealEntry from "./AppealEntry";
import MeasureCard, { measureNotes } from "./MeasureCard";
import {
	measureMeta,
	measureStatusChips,
	measureTitle,
} from "./measureDisplay";
import RemediationEntry from "./RemediationEntry";

function measureKey(measure: SellerMeasure): string {
	return measure.id;
}

/**
 * 賣家措施與申覆自助頁（`/seller/measures`）：全部措施的列表，掛在側欄「財務與設定」群組（停權公告的每張卡連到單筆措施頁）。
 * 措施與申覆以帳號為範圍，不在賣家中心外框裡：店家全關的帳號沒有賣家角色，一樣要讀得到。申覆與解除請求是獨立頁。
 */
export default function SellerMeasuresPage() {
	const ut = useUiText();
	const { records, loading, error } = useSellerMeasures();

	return (
		<SellerPageShell
			title="措施與申覆"
			subtitle="查看平台對你商店的措施與處理進度，符合條件的措施可提出申覆。"
		>
			{loading ? (
				<>
					<SellerCardGridSkeleton
						variant="panel"
						count={3}
						className="seller-mobile-disclosure-desktop flex flex-col gap-[var(--hkui-space-4)]"
					/>
					<SellerMobileDisclosureSkeleton count={3} />
				</>
			) : error ? (
				<Card className="p-6 text-center text-sm text-[var(--hkui-foreground-soft)]">
					{error}
				</Card>
			) : records.length === 0 ? (
				<SellerEmptyState
					variant="violations"
					title="目前沒有措施紀錄"
					description="保持良好營運狀況，這裡會維持空空如也。"
				/>
			) : (
				<>
					<div className="seller-mobile-disclosure-desktop flex flex-col gap-[var(--hkui-space-4)]">
						{records.map((measure) => (
							<MeasureCard key={measure.id} measure={measure} />
						))}
					</div>
					<SellerMobileDisclosureList
						items={records}
						getKey={measureKey}
						ariaLabel="措施紀錄"
						getSummary={(measure) => {
							const chips = measureStatusChips(ut, measure);
							return {
								label: measureTitle(ut, measure),
								leading: <SellerMobileDisclosureIcon name="shield-fail" />,
								title: measureTitle(ut, measure),
								meta: measureMeta(ut, measure),
								status:
									chips.length > 0 ? (
										<span className="inline-flex flex-wrap items-center gap-1.5">
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
										</span>
									) : undefined,
							};
						}}
						renderDetails={(measure) => (
							<MobileMeasureDetails measure={measure} />
						)}
					/>
				</>
			)}
		</SellerPageShell>
	);
}

function MobileMeasureDetails({ measure }: { measure: SellerMeasure }) {
	const ut = useUiText();
	return (
		<>
			<div className="seller-mobile-disclosure__body">
				{measure.statement && <p className="m-0">{measure.statement}</p>}
				{measureNotes(ut, measure).map((note) => (
					<p
						key={note}
						className="m-0 text-xs text-[var(--hkui-foreground-muted)]"
					>
						{note}
					</p>
				))}
			</div>
			<div className="seller-mobile-disclosure__actions">
				<RemediationEntry
					measure={measure}
					buttonClassName="seller-orders__action"
					solid
				/>
				<AppealEntry
					measure={measure}
					buttonClassName="seller-orders__action"
					solid
				/>
			</div>
		</>
	);
}
