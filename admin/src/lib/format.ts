/**
 * 顯示層的格式化小工具。
 *
 * 這裡的兩支在 ADM 各有三份同名同責的副本（`disputes/page.tsx`、
 * `disputes/[id]/page.tsx`、`anomalies/page.tsx`），且曾經漂過——幣別前綴一度
 * 有的是 `NT$ ` 有的是裸 `$`，而 `$225` 讀者無從判斷是新台幣還是美元。
 *
 * 本檔是收斂點：新頁面一律 import 這裡，不再抄第四份。既有三份沒有一併改，
 * 那會動到三個不相干頁面的 diff，不在發票這一波的範圍。
 */

/** 金額（後端一律字串）→ `NT$ 1,234`。無法解析時原樣回傳，不假裝成 0。 */
export function formatAmount(raw: string | undefined | null): string {
	const n = Number(raw);
	if (!Number.isFinite(n)) return raw || "—";
	return `NT$ ${n.toLocaleString()}`;
}

/** 台北固定偏移（UTC+8）。台灣自 1979 年起無日光節約時間 ⇒ 固定偏移即為正解。 */
const TAIPEI_UTC_OFFSET_MS = 8 * 60 * 60 * 1000;

/**
 * ISO 8601 UTC → **台北**時間字串（`2026/08/30 03:56:50`，24 小時制）。
 *
 * 🔴 **釘死台北，不跟著看的人的瀏覽器時區跑**（backend#3053）。
 *
 * 原本是 `toLocaleString("zh-TW", { hour12: false })`，那會用 runtime 本機時區
 * 格式化。實測 ADM 的「自動開票啟用切點」在 `Asia/Tokyo` 下顯示 04:56:50、
 * 在 `Asia/Taipei` 下顯示 03:56:50，而賣家端對同一組值固定顯示 03:56:50
 * ——兩個畫面同時看會對不起來。
 *
 * 這在發票模組不是「顯示習慣」而是正確性：發票日期有法律語意且已明確綁台北
 * （`invoice_documents.invoice_date_taipei`、`tax_period_code` 由台北日期推導），
 * 而「自動開票啟用切點」正是決定哪些訂單走自動開票的那條界線 ——
 * 讀到晚一小時的切點，就會把切點前後判錯一小時的訂單。本專案的使用者人在
 * 日本（UTC+9），這不是假想情境是預設情境。
 *
 * 🔴 **用固定偏移 ＋ `getUTC*` 自己組字串，不用 `timeZone: "Asia/Taipei"`**：
 * 後者的輸出仍受 runtime ICU 版本影響，而 ADM 走 Cloudflare Workers（runtime 一律
 * UTC）＋ client hydration 兩種 runtime ⇒ 同一個 instant 可能產出兩份文字。
 * 同一份理由的完整版見 hawkr-website `lib/date/taipei.ts` 的檔頭。
 *
 * 解析不出來時**回傳原字串**而不是空白：後端給了一個看不懂的值時，畫面要看得到
 * 那個值本身，否則沒有人會發現契約壞了。
 */
export type DateLike = string | number | Date | null | undefined;

/**
 * 秒／毫秒判別門檻。後端 wire 上兩種單位都有（貼文與商品的 `created_at` 是 unix 秒、
 * 其餘多為 ISO 字串）。1e12 ms ＝ 2001-09-09；1e12 s ＝ 西元 33658 年 —— 兩個單位的
 * 合理值域不重疊，所以這個門檻分得開。
 */
const SECONDS_TO_MS_THRESHOLD = 1e12;

/** 把各種輸入折成台北牆上時間的年月日時分秒；折不出來回 null。 */
function taipeiParts(raw: DateLike): {
	y: number;
	mo: string;
	d: string;
	h: string;
	mi: string;
	s: string;
} | null {
	if (raw === null || raw === undefined || raw === "") return null;
	let ms: number;
	if (raw instanceof Date) ms = raw.getTime();
	else if (typeof raw === "number") {
		ms = raw < SECONDS_TO_MS_THRESHOLD ? raw * 1000 : raw;
	} else {
		const n = Number(raw);
		// 純數字字串同樣可能是 unix 秒（`products/page.tsx` 的 `unixSeconds`）。
		ms =
			Number.isFinite(n) && raw.trim() !== "" && /^\d+$/.test(raw.trim())
				? n < SECONDS_TO_MS_THRESHOLD
					? n * 1000
					: n
				: new Date(raw).getTime();
	}
	if (!Number.isFinite(ms)) return null;
	const t = new Date(ms + TAIPEI_UTC_OFFSET_MS);
	if (Number.isNaN(t.getTime())) return null;
	const p = (n: number) => String(n).padStart(2, "0");
	return {
		y: t.getUTCFullYear(),
		mo: p(t.getUTCMonth() + 1),
		d: p(t.getUTCDate()),
		h: p(t.getUTCHours()),
		mi: p(t.getUTCMinutes()),
		s: p(t.getUTCSeconds()),
	};
}

export function formatDateTime(iso: DateLike): string {
	const t = taipeiParts(iso);
	if (!t) return typeof iso === "string" && iso ? iso : "—";
	return `${t.y}/${t.mo}/${t.d} ${t.h}:${t.mi}:${t.s}`;
}

/** `2026/09/03 08:30`（到分）。列表類畫面顯示秒是雜訊。 */
export function formatDateTimeShort(iso: DateLike): string {
	const t = taipeiParts(iso);
	if (!t) return typeof iso === "string" && iso ? iso : "—";
	return `${t.y}/${t.mo}/${t.d} ${t.h}:${t.mi}`;
}

/** `2026/09/03`。收 ISO 字串、unix 秒、毫秒或 Date。 */
export function formatDate(raw: DateLike): string {
	const t = taipeiParts(raw);
	if (!t) return typeof raw === "string" && raw ? raw : "—";
	return `${t.y}/${t.mo}/${t.d}`;
}

/** `2026-09-03`（ISO 8601 的日期）。收的值同 `formatDate`。 */
export function formatIsoDate(raw: DateLike): string {
	const t = taipeiParts(raw);
	if (!t) return typeof raw === "string" && raw ? raw : "—";
	return `${t.y}-${t.mo}-${t.d}`;
}

/** `2026年9月3日`（頁首那種人讀的長格式，月／日不補零）。 */
export function formatDateLabel(raw: DateLike): string {
	const t = taipeiParts(raw);
	if (!t) return "—";
	return `${t.y}年${Number(t.mo)}月${Number(t.d)}日`;
}

/** `星期四`。以台北日期推導，不用 `toLocaleDateString`（那會跟著 runtime 時區跑）。 */
export function formatWeekday(raw: DateLike): string {
	const t = taipeiParts(raw);
	if (!t) return "—";
	const dow = new Date(
		Date.UTC(t.y, Number(t.mo) - 1, Number(t.d)),
	).getUTCDay();
	return `星期${["日", "一", "二", "三", "四", "五", "六"][dow]}`;
}

/**
 * 與 `formatDateTime` 相同，但在字串尾端標明時區。
 *
 * 用在「值本身有法律或流程語意、讀錯一小時會做錯判斷」的欄位（A09 的啟用切點、
 * 驗證時間、憑證更新時間）。標了之後，就算未來有人把格式化改回本機時區也看得出來
 * ——一個沒有時區標記的時間，錯了不會有人發現。
 */
export function formatTaipeiDateTime(iso: string | undefined | null): string {
	const s = formatDateTime(iso);
	return s === "—" ? s : `${s} (台北)`;
}

/** 統計卡的數字：讀取失敗是 `-- ⚠`，還沒有值是 `--`。 */
export function formatStat(
	value: number | undefined,
	isLoading: boolean,
	isError: boolean,
): string {
	if (isError) return "-- ⚠";
	if (isLoading || value === undefined) return "--";
	return String(value);
}
