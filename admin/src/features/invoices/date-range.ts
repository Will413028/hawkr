/**
 * 發票查詢頁（A10 稽核紀錄／A12 文件查詢）共用的日期區間工具。
 *
 * ## 為什麼要有這一支，而不是各頁自己寫
 *
 * 兩支端點（47、61）的 `date_from` / `date_to` 都是 **`YYYY-MM-DD` 且嚴格驗證，
 * 不合法一律 400**。理由寫在 58A §A12：AMEGO 對日期格式完全不驗、垃圾值會靜默回
 * 全部歷史，那個習慣不能帶進我方查詢層。⇒ 前端是第一道，後端是第二道；兩頁的第一道
 * 必須是同一份實作，否則其中一頁遲早會漂成 ISO 8601 而每次都吃 400。
 *
 * ## 為什麼是 `<input type="date">` 而不是 58A 寫的 `datetime-local`
 *
 * 58A §A10 的「日期區間四件套」以 `SessionListConsole` 為樣板，那一組用的是
 * `datetime-local` —— **因為它服務的端點吃的是 instant**（`localInputToUtcInstant`
 * 轉成 ISO 8601 之後整串送出去）。發票這兩支吃的是**日期**，而且後端依 UTC+8 日界
 * 解讀（58A §A12 逐字）。
 *
 * 沿用 `datetime-local` 會造出一個靜默失效的 UI：管理員挑了「08-30 14:00」，時間那半
 * 被前端丟掉、實際查的是一整天。他不會收到任何訊號，只會覺得「筆數怎麼比我選的多」。
 * ⇒ 本波改用 `type="date"`，**其餘三件照做**（draft/applied 兩層、前端先驗
 * `from <= to`、明寫比對的是哪一個時間欄）。這一項刻意偏離 58A，已列入回報。
 *
 * ⚠️ 同理**不做本地→UTC 轉換**：`type="date"` 給的就是使用者看到的那個日曆日，而後端
 * 用 UTC+8 日界解讀它。多轉一次 UTC 反而會把 08-30 變成 08-29（台北 00:00–08:00 那段），
 * 也就是使用者選了 30 號卻查到 29 號——一個沒有任何錯誤訊息的偏移。
 */

/** `<input type="date">` 的值域就是 `YYYY-MM-DD`；這裡只擋掉手打／自動填入的雜訊。 */
const WIRE_DATE = /^\d{4}-\d{2}-\d{2}$/u;

export function isWireDate(value: string): boolean {
	return WIRE_DATE.test(value);
}

export interface DateRangeCheck {
	/** 可以送出。 */
	ok: boolean;
	/** 不能送出時要顯示的那句話（`.adm-operation-error`，`role="alert"`）。 */
	message: string;
}

/**
 * 送出前的第一道驗證。
 *
 * 兩種擋法都必要且不可互相取代：格式錯是「後端會回 400」，而 `from > to` 是
 * **後端不會報錯**——它會老實地回一個空集合，而空集合看起來就像「這段期間沒有紀錄」。
 */
export function checkDateRange(from: string, to: string): DateRangeCheck {
	for (const [value, name] of [
		[from, "起始日期"],
		[to, "結束日期"],
	] as const) {
		if (value !== "" && !isWireDate(value)) {
			return { ok: false, message: `${name}格式不正確，請使用 YYYY-MM-DD。` };
		}
	}
	if (from !== "" && to !== "" && from > to) {
		return {
			ok: false,
			message: "起始日期不能晚於結束日期，這樣的區間查不到任何資料。",
		};
	}
	return { ok: true, message: "" };
}
