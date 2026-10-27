const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;

/** `google.type.Date` 的欄位；結構上可指定給生成的 `Date` 初始化形狀。 */
export type CalendarDate = { year: number; month: number; day: number };

/** `YYYY-MM-DD` 且是存在的日期（年份 1–9999）才回傳；其他一律 undefined（不 trim）。 */
export function parseDate(value: string): CalendarDate | undefined {
	const match = ISO_DATE.exec(value);
	if (!match) return undefined;
	const year = Number(match[1]);
	if (year < 1) return undefined; // google.type.Date 的 year 0 是「沒有年份」，不是 0000 年
	const month = Number(match[2]);
	const day = Number(match[3]);
	const check = new Date(0);
	check.setUTCFullYear(year, month - 1, day);
	if (
		check.getUTCFullYear() !== year ||
		check.getUTCMonth() !== month - 1 ||
		check.getUTCDate() !== day
	) {
		return undefined;
	}
	return { year, month, day };
}

/** 格式化成 `YYYY-MM-DD`；`date` 為 undefined 或年月日有任何一個是 0（未設定）時回空字串。 */
export function formatDate(date: CalendarDate | undefined): string {
	if (!date?.year || !date.month || !date.day) return "";
	const year = String(date.year).padStart(4, "0");
	const month = String(date.month).padStart(2, "0");
	const day = String(date.day).padStart(2, "0");
	return `${year}-${month}-${day}`;
}
