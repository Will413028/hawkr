"use client";

import { MeasureState } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import Link from "@/components/shared/Link";
import { useSellerMeasures } from "@/lib/hooks/useSellerMeasures";
import { useUiText } from "@/lib/i18n/ui-text";
import { measureHref, terminationNoticeText } from "./measureDisplay";

/**
 * 預告終止橫幅（賣家中心外框）：預定生效的終止合作（`ListMyMeasures(state=SCHEDULED)`）每筆一條，連到該措施
 * 的措施頁（申覆入口在那裡）。預告期間帳號不受限制，所以不擋任何頁面；讀取中或失敗不顯示。
 * 沿用站內 warning tone（同 `RealtimeSyncNotice`）。
 */
export default function TerminationNoticeBanner() {
	const ut = useUiText();
	const { records } = useSellerMeasures(MeasureState.SCHEDULED);
	const notices = records.flatMap((measure) => {
		const text = terminationNoticeText(ut, measure);
		return text ? [{ measure, text }] : [];
	});
	if (notices.length === 0) return null;
	return (
		<>
			{notices.map(({ measure, text }) => (
				<p
					key={measure.id}
					role="status"
					className="m-0 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-hkui-md bg-warning-50 px-3.5 py-2.5 text-[0.8125rem] font-semibold text-warning-700 dark:bg-[#a9842c]/26 dark:text-[#d6c27e]"
				>
					<span>{text}</span>
					<Link href={measureHref(measure)} className="font-bold underline">
						措施與申覆
					</Link>
				</p>
			))}
		</>
	);
}
