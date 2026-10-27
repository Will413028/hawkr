import {
	type Timestamp,
	timestampFromMs,
	timestampMs,
} from "@bufbuild/protobuf/wkt";

/**
 * Banner 排程上下架的契約 ⇄ 表單轉換。
 *
 * 契約（ContentAdminService 的 Banner）：
 *   - `start_time`（含）/ `end_time`（不含）是 `google.protobuf.Timestamp`，沒有值＝該端不設限：
 *     沒有 start_time＝立即生效、沒有 end_time＝無期限
 *   - 前台的 ListLiveBanners 依此過濾（開始 ≤ 現在 < 結束）
 *
 * 編輯器只有日期，以營運的時區（台北，UTC+8，沒有夏令時間）解讀：開始日寫成當天 00:00，結束日寫成隔天
 * 00:00，所以結束日整天有效、「起訖同一天」就是那一天。讀回時換算成台北的日期，結束顯示成前一天（最後一個
 * 有效的日子）。清單與編輯器共用這裡的換算，對同一筆 banner 顯示同一天。
 */

const TAIPEI_OFFSET_MS = 8 * 60 * 60 * 1000;
const DAY_MS = 24 * 60 * 60 * 1000;

/** 契約上的排程欄位。 */
export interface BannerSchedule {
	startTime?: Timestamp;
	endTime?: Timestamp;
}

/** `<input type="date">` 綁的表單狀態。日期部分為 `YYYY-MM-DD`，空字串＝未設定。 */
export interface BannerScheduleForm {
	startDate: string;
	endDate: string;
	noExpiry: boolean;
}

function taipeiDate(ms: number): string {
	return new Date(ms + TAIPEI_OFFSET_MS).toISOString().slice(0, 10);
}

function taipeiMidnight(date: string): number {
	return Date.parse(`${date}T00:00:00+08:00`);
}

/** 開始時間 → 台北的日期（`YYYY-MM-DD`）；沒有值回空字串。 */
export function startDateOf(ts: Timestamp | undefined): string {
	return ts ? taipeiDate(timestampMs(ts)) : "";
}

/** 結束時間（不含）→ 最後一個有效的日子（台北的日期）；沒有值回空字串。 */
export function endDateOf(ts: Timestamp | undefined): string {
	return ts ? taipeiDate(timestampMs(ts) - 1) : "";
}

/** 契約 → 表單狀態。沒有 end_time 即「無期限」。 */
export function scheduleFromBanner(b: BannerSchedule): BannerScheduleForm {
	return {
		startDate: startDateOf(b.startTime),
		endDate: endDateOf(b.endTime),
		noExpiry: !b.endTime,
	};
}

/** 表單狀態 → 契約的排程欄位。勾了無期限就不帶 end_time。 */
export function scheduleToBanner(form: BannerScheduleForm): BannerSchedule {
	return {
		startTime: form.startDate
			? timestampFromMs(taipeiMidnight(form.startDate))
			: undefined,
		endTime:
			form.noExpiry || !form.endDate
				? undefined
				: timestampFromMs(taipeiMidnight(form.endDate) + DAY_MS),
	};
}
