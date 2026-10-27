/**
 * 站外業者看得到的時間一律標台北時間。
 *
 * wire 上的到期時間是 RFC3339 UTC；直接印出來等於要業者自己換算時區，
 * 而 v6.0 對 buyer／seller-facing 時間的要求是明確標示 Asia/Taipei。
 */
const TAIPEI_FORMATTER = new Intl.DateTimeFormat("en-CA", {
	timeZone: "Asia/Taipei",
	year: "numeric",
	month: "2-digit",
	day: "2-digit",
	hour: "2-digit",
	minute: "2-digit",
	hour12: false,
});

export function formatTaipeiDateTime(iso: string): string {
	const parsed = new Date(iso);
	// 解析不出來就原樣回傳：顯示一個看得懂但沒換算過的字串，好過顯示
	// "Invalid Date" 或假裝它已經是台北時間。
	if (Number.isNaN(parsed.getTime())) return iso;
	return TAIPEI_FORMATTER.format(parsed).replace(",", "");
}
