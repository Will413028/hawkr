import { parseDate } from "@hawkr/api/proto/date";

/** 匯入的請求：year 是宣告年份（整數），csvContent 是檔案全文。 */
export function buildImportRequest(declaredYear: string, csvContent: string) {
	return { year: Number(declaredYear), csvContent };
}

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * 距耗盡天數：伺服器回報的 covered_through（從今天起連續涵蓋的最後一天）與今天（台北日期，呼叫端傳入）的天數差；沒有涵蓋（空字串）或已過期回 -1。
 */
export function coverageDaysLeft(
	coveredThrough: string,
	today: string,
): number {
	const last = parseDate(coveredThrough);
	const now = parseDate(today);
	if (!last || !now) return -1;
	const diff = Math.round(
		(Date.UTC(last.year, last.month - 1, last.day) -
			Date.UTC(now.year, now.month - 1, now.day)) /
			DAY_MS,
	);
	return diff < 0 ? -1 : diff;
}

export function coverageBadge(days: number): {
	variant: "danger" | "warning" | "success" | "gray";
	label: string;
} {
	if (days < 0) return { variant: "gray", label: "無資料" };
	if (days < 7) return { variant: "danger", label: `距耗盡 ${days} 天` };
	if (days < 60) return { variant: "warning", label: `距耗盡 ${days} 天` };
	return { variant: "success", label: `距耗盡 ${days} 天` };
}

const YEARS_BEFORE = 1;
const YEARS_AFTER = 2;

/** 可選的匯入年份：台北今年的前 1 年到後 2 年。 */
export function yearOptions(thisYear: number): number[] {
	return Array.from(
		{ length: YEARS_BEFORE + YEARS_AFTER + 1 },
		(_, i) => thisYear - YEARS_BEFORE + i,
	);
}

/** 預設匯入年份：已涵蓋最後一年的下一年（在可選範圍內），否則今年。 */
export function defaultImportYear(
	thisYear: number,
	coveredThrough: { year: number } | undefined,
): number {
	const next = coveredThrough ? coveredThrough.year + 1 : thisYear;
	return yearOptions(thisYear).includes(next) ? next : thisYear;
}
