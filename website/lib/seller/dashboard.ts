/**
 * 賣家首頁儀表板（`SellerAppService.GetMyDashboard`）。賣家路徑直接使用生成的 `Dashboard`：四個區塊來自不同
 * 的模組，讀不到的區塊不設，不是 0，所以每個讀取都從「有沒有設」開始，沒設就是 undefined。
 */

import type { Date as ProtoDate } from "@hawkr/api/gen/google/type/date_pb";
import type {
	Dashboard,
	DashboardPeriod,
	PeriodAmount,
	PeriodCount,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerAppClient } from "@/lib/connect/seller";

export type SellerDashboard = Dashboard;

type SellerClient = ReturnType<typeof sellerAppClient>;

export async function fetchSellerDashboard(
	client: () => SellerClient = sellerAppClient,
): Promise<SellerDashboard> {
	const res = await client().getMyDashboard({});
	if (!res.dashboard) throw new Error("GetMyDashboard 的回應沒有儀表板");
	return res.dashboard;
}

/**
 * 較前 7 日的成長百分比（整數）。區塊沒設或前期為 0 時回 undefined：沒有可比的基準，
 * 不是 0%。
 */
export function periodGrowthPercent(
	pair: PeriodAmount | PeriodCount | undefined,
): number | undefined {
	if (!pair) return undefined;
	const current = Number(pair.current);
	const previous = Number(pair.previous);
	if (!Number.isFinite(current) || !Number.isFinite(previous)) return undefined;
	if (previous === 0) return undefined;
	return Math.round(((current - previous) / previous) * 100);
}

function monthDay(date: ProtoDate | undefined): string | undefined {
	if (!date || date.month === 0 || date.day === 0) return undefined;
	return `${date.month}/${date.day}`;
}

/** 期間的顯示文字（台北日曆日，含頭含尾），例如「9/30–10/6」；沒設或日期不完整時回 undefined。 */
export function periodLabel(
	period: DashboardPeriod | undefined,
): string | undefined {
	const first = monthDay(period?.firstDate);
	const last = monthDay(period?.lastDate);
	if (!first || !last) return undefined;
	return `${first}–${last}`;
}
