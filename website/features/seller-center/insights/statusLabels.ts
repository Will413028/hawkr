/** Forecast-curve point status -> 中文 label. Lives in its own module (not
 *  re-exported from InsightsClient.tsx) so SevenDayTable.tsx doesn't import
 *  a named export back out of a `"use client"` page component — works
 *  today since it's only dereferenced at render time, but a plain sibling
 *  module avoids the circular-looking import entirely. */
export const STATUS_LABELS: Record<string, string> = {
	ready: "已預測",
	partial: "部分預測（部分子品項無資料）",
	market_closed: "市場休市",
	insufficient_data: "近期資料不足",
	not_predicted: "尚未產生預測",
	stale: "預測資料過舊",
	unavailable: "目前無法估價",
};
