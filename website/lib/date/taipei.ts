/**
 * 台北日期格式化（輸出與 runtime 時區 / locale / ICU 版本全脫鉤）。
 *
 * 為什麼不用 `toLocaleDateString("zh-TW")`（hawkr-website#374）：
 *   1. **hydration mismatch**：server render 的頁面在 Cloudflare Workers runtime
 *      一律 UTC、client 端跟著瀏覽器時區，同一個 instant 在 16:00Z 之後就會產出
 *      差一天的文字 → React #418（量大到把 Sentry ingest 打成 429）。
 *      `toLocaleDateString` 的輸出還受 runtime ICU 版本影響，是第二條 mismatch
 *      途徑。
 *   2. **純 server render 也會錯**：字串在 server 算完才序列化下傳（PDP 評價
 *      日期就是這條路），此時沒有 mismatch，但**所有**使用者都看到 UTC 日期
 *      —— 台北使用者在 16:00Z~24:00Z 的跨日窗口會看到差一天的日期，是資料
 *      正確性缺陷而非只是視覺不一致。
 *
 * 兩者都靠「自己組字串 + 固定台北時區」根除：只用 `getUTC*`，把 instant 先加上
 * 固定的 +08:00 再讀取，因此不論 `process.env.TZ` / 瀏覽器時區 / ICU 版本為何，
 * 同一個 instant 永遠得到同一串文字。台灣自 1979 年起無日光節約時間，固定偏移
 * 即為正解（也比 `timeZone: "Asia/Taipei"` 少一份對 runtime tzdata 的依賴）。
 *
 * 同一個 root cause 有三種長相，本檔三種都取代：
 *   (a) `toLocale*` 家族（#391 第一輪）
 *   (b) `getFullYear` / `getMonth` / `getDate` / `getHours` / `getMinutes` /
 *       `getDay` 等 **local-time getter** 自己組字串（#391 第二輪）。
 *       它們不呼叫 `Intl`，所以躲得過針對 (a) 的護欄，但輸出照樣跟著瀏覽器時區跑。
 *   (c) 把**沒有時區標記的牆上時間字串**（`2026/07/19`）丟給 `Date.parse`。
 *       它是 (b) 的隱藏版：`Date.parse` 對這一族的解讀就是「runtime 本地時區的
 *       那一刻」，所以輸入端就已經吃進了瀏覽器時區，後面的 `getUTC*` 再乾淨也
 *       救不回來。見 `toEpochMs` 的 `WALL_CLOCK_RE` 段。
 *
 * 對外提供八種輸出，涵蓋本站全部「日期／日期＋時間」呈現需求，全部共用同一套
 * 位移與補零規則，對齊 repo 內既有的呈現慣例
 * （`features/after-sales/components/AfterSaleCaseCard.tsx`、
 * `features/messages/components/ChatCards.tsx` 的 `formatDate`、
 * `features/seller-center/orders/orderDisplay.ts` 的 24 小時制時間）：
 *
 *   `formatTaipeiDate`          → `2026/07/30`
 *   `formatTaipeiIsoDate`       → `2026-07-30`
 *   `formatTaipeiDateTime`      → `2026/07/30 15:05`
 *   `formatTaipeiMonthDayTime`  → `07/30 15:05`
 *   `formatTaipeiMonthDay`      → `7月30日`
 *   `formatTaipeiShortDate`     → `7/30`
 *   `formatTaipeiTime`          → `15:05`
 *
 * 排版真的湊不出來時用 `taipeiParts(raw)` 自己組（它回傳台北牆上時間的
 * 年/月/日/時/分/星期）——**但不要回頭去碰 local-time getter**。
 * 新增呈現需求時**在本檔加一個具名輸出**，不要在呼叫點自己組 `Intl` / `toLocale*`
 * ——`tests/date-formatting-taipei.test.ts` 會擋下兩種裸呼叫（#391）。
 */

/** 台北固定偏移（UTC+8，無 DST）。 */
export const TAIPEI_UTC_OFFSET_MS = 8 * 60 * 60 * 1000;

/**
 * 秒／毫秒判別門檻。後端 wire 上同時存在兩種單位（評價 / 優惠券 / 客戶列表的
 * `created_at` 是 unix 秒，訂單類是毫秒），本 repo 原本在 5 個呼叫點各寫一份
 * `ts < 1e12 ? ts * 1000 : ts`，收斂到這裡。
 * 1e12 ms = 2001-09-09；1e12 s = 西元 33658 年 —— 兩個單位的合理值域不重疊。
 */
const SECONDS_TO_MS_THRESHOLD = 1e12;

export type DateLike = string | number | Date | null | undefined;

/**
 * 沒有時區標記的「牆上時間」字串：`2026/07/19`、`2026-07-19`、
 * `2026-07-19 08:30`、`2026-07-19T08:30:00`、`2026-07-19T08:30:00.123`。
 *
 * 帶 `Z` 或 `+08:00` 的字串**不會**命中（`$` 錨在時間之後），它們是真正的
 * instant，仍舊交給 `Date.parse`。
 */
const WALL_CLOCK_RE =
	/^(\d{4})[/-](\d{1,2})[/-](\d{1,2})(?:[T ](\d{1,2}):(\d{2})(?::(\d{2})(?:\.(\d{1,3})\d*)?)?)?$/;

/**
 * 把 wire 上會出現的各種時間表示法正規化成 epoch 毫秒；無法解讀回 `null`。
 *
 * 接受：`Date`、unix 秒、unix 毫秒、純數字字串（同樣秒／毫秒判別）、
 * 無時區標記的牆上時間字串（視為台北牆上時間，見下），以及 `Date.parse`
 * 能解析的其餘字串（帶 `Z` / 位移的 ISO 8601 為主）。
 *
 * **為什麼牆上時間字串不能交給 `Date.parse`**：規格對「日期＋無位移」與非 ISO
 * 的 `YYYY/MM/DD` 一律以 **runtime 本地時區**解讀，於是同一份 wire 資料在不同
 * 瀏覽器時區會變成不同的 instant，`partsFromMs` 再乾淨也已經來不及 ——
 * UTC+8 以東的使用者會少一天（實測：`orderDate="2026/07/19"` 在
 * `Asia/Tokyo` 顯示 `2026/07/18`；本站主要使用者確實有人常駐日本）。
 * 本站的牆上時間一律是台北業務時間（例：buyer 模組消費明細的
 * `orderDate` 契約就是 `YYYY/MM/DD（Asia/Taipei）`），所以這裡固定以
 * `Date.UTC(...) - 08:00` 還原成 instant，輸出因此與 runtime 時區脫鉤。
 * 純日期（`YYYY-MM-DD`，如生日 / 交易日）視為台北當日 00:00，日期欄位的
 * 呈現結果與舊行為相同。
 *
 * `0` 視為「無值」而非 1970-01-01：epoch 0 在本站不是合法業務時間，且所有呼叫點
 * 原本都以 `if (!ts)` 擋掉它。
 */
export function toEpochMs(raw: DateLike): number | null {
	if (raw === null || raw === undefined || raw === "") return null;

	if (raw instanceof Date) {
		const ms = raw.getTime();
		return Number.isFinite(ms) && ms !== 0 ? ms : null;
	}

	if (typeof raw === "number") {
		if (!Number.isFinite(raw) || raw === 0) return null;
		return Math.abs(raw) < SECONDS_TO_MS_THRESHOLD ? raw * 1000 : raw;
	}

	const text = raw.trim();

	// 純數字字串是 epoch（走同一條秒／毫秒判別）。
	// 先分流才正確：`new Date("1785000000")` 不是 epoch 解析。
	if (/^-?\d+$/.test(text)) return toEpochMs(Number(text));

	const wall = WALL_CLOCK_RE.exec(text);
	if (wall) {
		const year = Number(wall[1]);
		const month = Number(wall[2]);
		const day = Number(wall[3]);
		const hour = Number(wall[4] ?? 0);
		const minute = Number(wall[5] ?? 0);
		const second = Number(wall[6] ?? 0);
		// 語法對但值不存在（`2026-02-30` / `2026-13-01` / `08:99`）時 `Date.UTC`
		// 會靜默進位成另一個看起來很合理的時刻，而舊路徑的 `Date.parse` 對這些
		// 一律回 NaN。範圍先擋掉時分秒，日期進位再靠下面回讀一次擋掉 —— 兩者
		// 都做才與舊行為一致，只做一半會讓 `08:99` 悄悄渲染成 `09:39`。
		if (hour > 23 || minute > 59 || second > 59) return null;
		const utcMs =
			Date.UTC(
				year,
				month - 1,
				day,
				hour,
				minute,
				second,
				Number((wall[7] ?? "").padEnd(3, "0") || 0),
			) - TAIPEI_UTC_OFFSET_MS;
		if (!Number.isFinite(utcMs) || utcMs === 0) return null;
		const back = new Date(utcMs + TAIPEI_UTC_OFFSET_MS);
		if (
			back.getUTCFullYear() !== year ||
			back.getUTCMonth() + 1 !== month ||
			back.getUTCDate() !== day
		) {
			return null;
		}
		return utcMs;
	}

	const parsed = Date.parse(text);
	return Number.isFinite(parsed) && parsed !== 0 ? parsed : null;
}

const pad = (n: number) => String(n).padStart(2, "0");

export interface TaipeiParts {
	year: number;
	/** 1-12（已 +1，呼叫端不必再處理 `getUTCMonth` 的 0-based）。 */
	month: number;
	day: number;
	hour: number;
	minute: number;
	/** 0=週日 … 6=週六（台北牆上時間的星期）。 */
	weekday: number;
}

/**
 * 把 epoch 毫秒拆成台北牆上時間的年月日時分＋星期。
 *
 * 全檔唯一讀取時間欄位的地方，且**只用 `getUTC*`**：先把 instant 位移 +08:00，
 * 再以 UTC 欄位讀出該牆上時間。任何 `getFullYear` / `getMonth` / `getHours`
 * 之類的 local-time getter 都會讓輸出跟著 `process.env.TZ` / 瀏覽器時區跑，
 * 也就是 #374 的成因本身。
 */
function partsFromMs(ms: number): TaipeiParts {
	const t = new Date(ms + TAIPEI_UTC_OFFSET_MS);
	return {
		year: t.getUTCFullYear(),
		month: t.getUTCMonth() + 1,
		day: t.getUTCDate(),
		hour: t.getUTCHours(),
		minute: t.getUTCMinutes(),
		weekday: t.getUTCDay(),
	};
}

/**
 * 台北牆上時間的各欄位；無法解讀回 `null`。
 *
 * 給「需要自訂排版、既有 formatter 湊不出來」的呼叫點用的逃生門
 * （例：賣家儀表板的 `YYYY-MM-DD 週X`）。**它存在的意義是讓那些呼叫點不必回頭
 * 去碰 `getFullYear` / `getHours` 家族** —— 那一族的輸出跟著瀏覽器時區跑，
 * 是 #374／#391 的同一個 root cause，且 `tests/date-formatting-taipei.test.ts`
 * 會擋下來。需求夠通用就回本檔加一個具名 formatter，不要在呼叫點長出第二套。
 */
export function taipeiParts(raw: DateLike): TaipeiParts | null {
	const ms = toEpochMs(raw);
	return ms === null ? null : partsFromMs(ms);
}

/**
 * 以台北牆上時間輸出 `YYYY/MM/DD`；無法解讀的輸入回空字串（不渲染
 * `Invalid Date`）。呼叫端若要「解不出來就原字串照登」，用
 * `formatTaipeiDate(raw) || raw`。
 */
export function formatTaipeiDate(raw: DateLike): string {
	const ms = toEpochMs(raw);
	if (ms === null) return "";

	const { year, month, day } = partsFromMs(ms);
	return `${year}/${pad(month)}/${pad(day)}`;
}

/**
 * 以台北牆上時間輸出 `YYYY-MM-DD`；無法解讀的輸入回空字串。
 *
 * 與 `formatTaipeiDate` 只差分隔符，但用途不同：那一支是給人看的呈現，這一支是
 * **表單控制項的值**——`<input type="date">` 的 `value` / `min` / `max` 只認
 * `YYYY-MM-DD`，餵 `2026/07/30` 會被整個忽略（不報錯，就是沒有限制）。
 *
 * 它同時是 `taipeiDayStartUtc` / `taipeiDayEndUtc` 的反向：那兩支把使用者選的
 * 台北曆日還原成 instant，這一支把 instant 讀回台北曆日。因為固定補零、固定
 * 長度，兩個同格式的值可以直接做字串比較——「不得早於今天」這類前端限制因此
 * 不必回頭碰 `getFullYear` 家族（#391 的 root cause）。
 */
export function formatTaipeiIsoDate(raw: DateLike): string {
	const ms = toEpochMs(raw);
	if (ms === null) return "";

	const { year, month, day } = partsFromMs(ms);
	return `${year}-${pad(month)}-${pad(day)}`;
}

/**
 * 以台北牆上時間輸出 `YYYY/MM/DD HH:mm`（24 小時制、不含秒）。
 *
 * 這是 #391 的收斂目標：原本 16 個「日期＋時間」呼叫點散落三種
 * `toLocaleString("zh-TW", …)` 變體，輸出分別是
 * `2026/7/30 下午3:05:09`（無 options）、`2026/7/30 15:05:09`（`hour12:false`）、
 * `2026/07/30 下午03:05`（`{year,month,day,hour,minute}`）—— 同一件事三種樣子，
 * 而且三種都跟著 runtime 時區與 ICU 版本飄。統一成 24 小時制、補零、不含秒，
 * 對齊 repo 內既有的時間呈現慣例（`features/seller-center/orders/orderDisplay.ts`、
 * `features/orders/components/OrderStatusTimeline.tsx`）。
 */
export function formatTaipeiDateTime(raw: DateLike): string {
	const ms = toEpochMs(raw);
	if (ms === null) return "";

	const { year, month, day, hour, minute } = partsFromMs(ms);
	return `${year}/${pad(month)}/${pad(day)} ${pad(hour)}:${pad(minute)}`;
}

/**
 * 以台北牆上時間輸出 `MM/DD HH:mm`（省略年份的緊湊版）。
 *
 * 用於空間有限、且年份對讀者沒有資訊量的列表（賣家中心對話列表的「最後訊息時間」）。
 */
export function formatTaipeiMonthDayTime(raw: DateLike): string {
	const ms = toEpochMs(raw);
	if (ms === null) return "";

	const { month, day, hour, minute } = partsFromMs(ms);
	return `${pad(month)}/${pad(day)} ${pad(hour)}:${pad(minute)}`;
}

/**
 * 以台北牆上時間輸出 `M月D日`（不補零，對齊 `zh-TW` 的
 * `{ month: "short", day: "numeric" }` 原本的呈現）。
 *
 * 用於摘要文案裡的短日期（會員總覽的「上次 7月30日」）。
 */
export function formatTaipeiMonthDay(raw: DateLike): string {
	const ms = toEpochMs(raw);
	if (ms === null) return "";

	const { month, day } = partsFromMs(ms);
	return `${month}月${day}日`;
}

/**
 * 以台北牆上時間輸出 `M/D`（不補零）。
 *
 * 用於「相對時間超過 7 天後改顯示日期」的列表尾巴（通知中心、貼文留言、
 * 對話側欄）與出貨日期 chip。
 */
export function formatTaipeiShortDate(raw: DateLike): string {
	const ms = toEpochMs(raw);
	if (ms === null) return "";

	const { month, day } = partsFromMs(ms);
	return `${month}/${day}`;
}

/**
 * 以台北牆上時間輸出 `HH:mm`（24 小時制、補零）。
 *
 * 用於同一天內只需要時間的場合（聊天氣泡的時間戳，日期由分隔線提供）。
 */
export function formatTaipeiTime(raw: DateLike): string {
	const ms = toEpochMs(raw);
	if (ms === null) return "";

	const { hour, minute } = partsFromMs(ms);
	return `${pad(hour)}:${pad(minute)}`;
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** epoch 毫秒 → `YYYY-MM-DDTHH:mm:ssZ`（秒精度，API 規範 §2 的時間 wire 格式）。 */
function isoSecondsUtc(ms: number): string {
	const t = new Date(ms);
	return (
		`${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())}` +
		`T${pad(t.getUTCHours())}:${pad(t.getUTCMinutes())}:${pad(t.getUTCSeconds())}Z`
	);
}

/**
 * 台北曆日 `YYYY-MM-DD`（`<input type="date">` 的值）→ 該日在台北的**起點**，
 * 輸出 ISO 8601 UTC。解不出來回空字串，呼叫端據此不送這個條件。
 *
 * 這兩支是 `formatTaipei*` 的反向：那些把 instant 讀成台北牆上時間，這些把使用者
 * 選的台北曆日還原成 instant 再送給後端（後端一律以 UTC instant 比對 `created_at`）。
 * **不要在呼叫點自己組**：`new Date("2026-08-19")` 的 UTC 語意會讓台北使用者選
 * 「8/19」卻篩到 8/18 08:00 起算的那 24 小時，而畫面上完全看不出錯。
 */
export function taipeiDayStartUtc(day: string): string {
	const ms = toEpochMs(day);
	return ms === null ? "" : isoSecondsUtc(ms);
}

/**
 * 台北曆日 → 該日在台北的**終點**（`23:59:59`），輸出 ISO 8601 UTC。
 *
 * 取 `23:59:59` 而不是隔日 `00:00:00`：後端的區間比對是閉區間（`date_from` 不得晚於
 * `date_to`，見 gateway `auditDateRange`），送隔日起點會把隔天第一秒的訂單一起篩進來。
 */
export function taipeiDayEndUtc(day: string): string {
	const ms = toEpochMs(day);
	return ms === null ? "" : isoSecondsUtc(ms + DAY_MS - 1000);
}
