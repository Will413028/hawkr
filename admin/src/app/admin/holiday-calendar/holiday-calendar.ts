import { formatDate, parseDate } from "@hawkr/api/proto/date";

/** 辦公日曆表 CSV 的一列（預覽用；伺服器收到全文後才是唯一的驗證者）。 */
export interface CalendarRow {
	iso: string; // YYYY-MM-DD
	year: number;
	holiday: boolean; // 是否放假 = 2
	note: string;
}

export interface CalendarPreview {
	totalRows: number;
	/** 各年的列數，依列數降序、同數時年份大者在前。 */
	yearCounts: { year: number; count: number }[];
}

/**
 * 解析人事行政總處辦公日曆表逐日 CSV（可帶 BOM；表頭 `西元日期,星期,是否放假,備註`；日期 `YYYYMMDD`；是否放假 `0`／`2`）。
 * 只為預覽：略過表頭、空行與無法解析的列，嚴格的檢查（每天一列、年份一致）留給伺服器，失敗時以它的訊息為準。
 */
export function parseCalendarCsv(text: string): CalendarRow[] {
	const rows: CalendarRow[] = [];
	const lines = text.replace(/^﻿/, "").split(/\r\n|\r|\n/);
	for (const [i, line] of lines.entries()) {
		if (i === 0 || line.trim() === "") continue;
		const cols = line.split(",");
		if (cols.length < 3) continue;
		const m = /^(\d{4})(\d{2})(\d{2})$/.exec(cols[0].trim());
		if (!m) continue;
		const date = parseDate(`${m[1]}-${m[2]}-${m[3]}`);
		if (!date) continue;
		const flag = cols[2].trim();
		if (flag !== "0" && flag !== "2") continue;
		rows.push({
			iso: formatDate(date),
			year: date.year,
			holiday: flag === "2",
			note: cols.slice(3).join(",").trim(),
		});
	}
	return rows;
}

export function buildPreview(rows: CalendarRow[]): CalendarPreview {
	const perYear = new Map<number, number>();
	for (const r of rows) perYear.set(r.year, (perYear.get(r.year) ?? 0) + 1);
	const yearCounts = [...perYear.entries()]
		.map(([year, count]) => ({ year, count }))
		.sort((a, b) => b.count - a.count || b.year - a.year);
	return { totalRows: rows.length, yearCounts };
}

/** 宣告年份的列，依日期排序。 */
export function yearRowsOf(rows: CalendarRow[], year: number): CalendarRow[] {
	return rows
		.filter((r) => r.year === year)
		.sort((a, b) => a.iso.localeCompare(b.iso));
}

/** 匯入的請求：year 是宣告年份（整數），csvContent 是檔案全文。 */
export function buildImportRequest(declaredYear: string, csvContent: string) {
	return { year: Number(declaredYear), csvContent };
}

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * 距耗盡天數：最後涵蓋日與今天（台北日期，呼叫端傳入）的天數差；沒有涵蓋或已過期回 -1。
 */
export function coverageDaysLeft(lastCovered: string, today: string): number {
	const last = parseDate(lastCovered);
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
