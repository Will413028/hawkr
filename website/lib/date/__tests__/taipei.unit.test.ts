import { describe, expect, it } from "vitest";
import {
	formatTaipeiDate,
	formatTaipeiDateTime,
	formatTaipeiIsoDate,
	formatTaipeiMonthDay,
	formatTaipeiMonthDayTime,
	formatTaipeiShortDate,
	formatTaipeiTime,
	taipeiDayStartUtc,
	taipeiParts,
	toEpochMs,
} from "../taipei";

/**
 * Regression: hawkr-website#374 — 全站日期原本走
 * `new Date(x).toLocaleDateString("zh-TW")`，同一個 instant 的輸出跟著 runtime
 * 時區飄：
 *   - server render 的頁面（賣家評價第一頁）→ Workers(UTC) vs 瀏覽器時區文字不同
 *     → React #418，量大到把 Sentry ingest 打成 429。
 *   - server 算完才序列化下傳的頁面（PDP 評價）→ 沒有 mismatch，但全部使用者看到
 *     UTC 日期，台北使用者在跨日窗口看到差一天 —— 資料正確性缺陷。
 *
 * 這支測試釘住「輸出與 runtime 時區脫鉤」：
 *  - 反向驗證段先證明舊寫法真的會飄（否則本測試沒有守住任何東西）
 *  - 正向段釘住固定台北日期字串；以 `TZ=<任意值> vitest` 跑都必須一致
 */

// 2026-07-30T16:30:00Z：UTC 是 7/30、台北是 7/31 —— 跨日窗口的代表值。
const CROSS_MIDNIGHT_ISO = "2026-07-30T16:30:00Z";
// 2026-07-30T02:00:00Z：UTC 與台北同一天 —— 對照組。
const SAME_DAY_ISO = "2026-07-30T02:00:00Z";

describe("反向驗證：舊寫法（locale + runtime 時區）確實會飄", () => {
	it("同一個 instant 在 UTC 與 Asia/Taipei 產出不同日期文字", () => {
		const instant = new Date(CROSS_MIDNIGHT_ISO);
		const asUtc = new Intl.DateTimeFormat("zh-TW", {
			timeZone: "UTC",
		}).format(instant);
		const asTaipei = new Intl.DateTimeFormat("zh-TW", {
			timeZone: "Asia/Taipei",
		}).format(instant);

		// 這就是 #374 的來源：UTC runtime 與台北使用者看到的日期不同。
		expect(asUtc).not.toBe(asTaipei);
	});
});

describe("formatTaipeiDate", () => {
	it("跨日窗口固定回傳台北當日（不隨 runtime 時區改變）", () => {
		expect(formatTaipeiDate(CROSS_MIDNIGHT_ISO)).toBe("2026/07/31");
	});

	it("同日窗口回傳同一天", () => {
		expect(formatTaipeiDate(SAME_DAY_ISO)).toBe("2026/07/30");
	});

	it("以 UTC 欄位計算，因此不呼叫任何 local-time getter", () => {
		// 若實作誤用 getFullYear/getMonth/getDate，把 process.env.TZ 換掉就會炸。
		// 這裡直接以「同一 instant 的多種輸入型別」交叉核對輸出恆等。
		const iso = CROSS_MIDNIGHT_ISO;
		const ms = new Date(iso).getTime();

		expect(formatTaipeiDate(iso)).toBe("2026/07/31");
		expect(formatTaipeiDate(ms)).toBe("2026/07/31");
		expect(formatTaipeiDate(new Date(iso))).toBe("2026/07/31");
		// unix 秒（評價 / 優惠券 / 客戶列表的 wire 形狀）與純數字字串（會員卡明細）
		expect(formatTaipeiDate(Math.floor(ms / 1000))).toBe("2026/07/31");
		expect(formatTaipeiDate(String(ms))).toBe("2026/07/31");
		expect(formatTaipeiDate(String(Math.floor(ms / 1000)))).toBe("2026/07/31");
	});

	it("補零到兩位，對齊 repo 既有 YYYY/MM/DD 呈現", () => {
		expect(formatTaipeiDate("2026-01-05T00:00:00Z")).toBe("2026/01/05");
	});

	it("台北時間的月／年邊界不會被偏移算錯", () => {
		// 2025-12-31T16:00:00Z = 2026-01-01 00:00 台北
		expect(formatTaipeiDate("2025-12-31T16:00:00Z")).toBe("2026/01/01");
		// 2026-01-31T16:00:00Z = 2026-02-01 00:00 台北
		expect(formatTaipeiDate("2026-01-31T16:00:00Z")).toBe("2026/02/01");
		// 閏年 2/29 也在偏移後成立
		expect(formatTaipeiDate("2028-02-28T16:00:00Z")).toBe("2028/02/29");
	});

	it("空值與不可解析輸入回傳空字串（不渲染 Invalid Date）", () => {
		expect(formatTaipeiDate(undefined)).toBe("");
		expect(formatTaipeiDate(null)).toBe("");
		expect(formatTaipeiDate("")).toBe("");
		expect(formatTaipeiDate("not-a-date")).toBe("");
		expect(formatTaipeiDate(0)).toBe("");
		expect(formatTaipeiDate(Number.NaN)).toBe("");
	});
});

describe("formatTaipeiIsoDate（YYYY-MM-DD，表單控制項的值）", () => {
	it("與 formatTaipeiDate 同一份計算，只差分隔符", () => {
		expect(formatTaipeiIsoDate(CROSS_MIDNIGHT_ISO)).toBe("2026-07-31");
		expect(formatTaipeiIsoDate(CROSS_MIDNIGHT_ISO)).toBe(
			formatTaipeiDate(CROSS_MIDNIGHT_ISO).replaceAll("/", "-"),
		);
		expect(formatTaipeiIsoDate(SAME_DAY_ISO)).toBe("2026-07-30");
	});

	it("補零，兩個值因此可以直接做字串比較（「不得早於今天」靠這件事）", () => {
		expect(formatTaipeiIsoDate("2026-01-05T00:00:00Z")).toBe("2026-01-05");
		expect(formatTaipeiIsoDate("2026-01-04T00:00:00Z") < "2026-01-05").toBe(
			true,
		);
	});

	it("`taipeiDayStartUtc` 收得回它自己的輸出（兩支互為反向）", () => {
		const day = formatTaipeiIsoDate(CROSS_MIDNIGHT_ISO);
		expect(formatTaipeiIsoDate(taipeiDayStartUtc(day))).toBe(day);
	});

	it("空值與不可解析輸入回傳空字串", () => {
		expect(formatTaipeiIsoDate(null)).toBe("");
		expect(formatTaipeiIsoDate("")).toBe("");
		expect(formatTaipeiIsoDate("not-a-date")).toBe("");
		expect(formatTaipeiIsoDate(0)).toBe("");
	});
});

/**
 * #391：16 個「日期＋時間」呼叫點原本散落三種 `toLocaleString("zh-TW", …)` 變體
 * （`2026/7/30 下午3:05:09` / `2026/7/30 15:05:09` / `2026/07/30 下午03:05`），
 * 全部收斂到下面這三個具名輸出。時間部分一律 24 小時制、補零、不含秒。
 */
describe("formatTaipeiDateTime（YYYY/MM/DD HH:mm）", () => {
	it("跨日窗口的日期與時間都以台北牆上時間計算", () => {
		// 16:30Z → 台北 00:30（隔天）：日期進位、時間歸零附近，兩邊同時驗證。
		expect(formatTaipeiDateTime(CROSS_MIDNIGHT_ISO)).toBe("2026/07/31 00:30");
	});

	it("同日窗口：02:00Z → 台北 10:00", () => {
		expect(formatTaipeiDateTime(SAME_DAY_ISO)).toBe("2026/07/30 10:00");
	});

	it("24 小時制、時分補零，且不含秒", () => {
		// 舊寫法在這個 instant 會產出 `2026/7/30 下午3:05:09`（依 runtime 而定）。
		expect(formatTaipeiDateTime("2026-07-30T07:05:09Z")).toBe(
			"2026/07/30 15:05",
		);
		expect(formatTaipeiDateTime("2026-07-30T00:01:00Z")).toBe(
			"2026/07/30 08:01",
		);
	});

	it("接受與 formatTaipeiDate 相同的輸入型別，輸出的日期段一致", () => {
		const ms = new Date(CROSS_MIDNIGHT_ISO).getTime();
		for (const raw of [
			CROSS_MIDNIGHT_ISO,
			ms,
			new Date(CROSS_MIDNIGHT_ISO),
			Math.floor(ms / 1000),
			String(ms),
		]) {
			expect(formatTaipeiDateTime(raw).slice(0, 10)).toBe(
				formatTaipeiDate(raw),
			);
		}
	});

	it("空值與不可解析輸入回傳空字串（呼叫端用 `|| raw` 決定要不要照登原字串）", () => {
		expect(formatTaipeiDateTime(undefined)).toBe("");
		expect(formatTaipeiDateTime(null)).toBe("");
		expect(formatTaipeiDateTime("")).toBe("");
		expect(formatTaipeiDateTime("not-a-date")).toBe("");
		expect(formatTaipeiDateTime(0)).toBe("");
	});
});

describe("formatTaipeiMonthDayTime（MM/DD HH:mm）", () => {
	it("省略年份，其餘與 formatTaipeiDateTime 同一份計算", () => {
		expect(formatTaipeiMonthDayTime(CROSS_MIDNIGHT_ISO)).toBe("07/31 00:30");
		expect(formatTaipeiMonthDayTime("2026-07-30T07:05:09Z")).toBe(
			"07/30 15:05",
		);
		expect(formatTaipeiMonthDayTime(CROSS_MIDNIGHT_ISO)).toBe(
			formatTaipeiDateTime(CROSS_MIDNIGHT_ISO).slice(5),
		);
	});

	it("無值回空字串", () => {
		expect(formatTaipeiMonthDayTime(null)).toBe("");
	});
});

describe("formatTaipeiMonthDay（M月D日）", () => {
	it("不補零，對齊原本 zh-TW `{month:'short', day:'numeric'}` 的呈現", () => {
		expect(formatTaipeiMonthDay(CROSS_MIDNIGHT_ISO)).toBe("7月31日");
		expect(formatTaipeiMonthDay("2026-01-05T00:00:00Z")).toBe("1月5日");
	});

	it("無值回空字串", () => {
		expect(formatTaipeiMonthDay(0)).toBe("");
	});
});

/**
 * #391 第二輪：另外 23 個渲染點原本用 `getFullYear` / `getHours` 家族自己組字串
 * （不呼叫 `Intl`，所以躲過第一輪的護欄，但輸出照樣跟著瀏覽器時區跑）。
 * 下面兩個 formatter 與 `taipeiParts` 逃生門是那一輪的收斂目標。
 */
describe("formatTaipeiShortDate（M/D，不補零）", () => {
	it("跨日窗口取台北當日", () => {
		expect(formatTaipeiShortDate(CROSS_MIDNIGHT_ISO)).toBe("7/31");
	});

	it("個位數月日不補零（對齊原本 getMonth()+1 與 getDate() 直接串接的呈現）", () => {
		expect(formatTaipeiShortDate("2026-01-05T00:00:00Z")).toBe("1/5");
	});

	it("無值回空字串", () => {
		expect(formatTaipeiShortDate(null)).toBe("");
		expect(formatTaipeiShortDate("not-a-date")).toBe("");
	});
});

describe("formatTaipeiTime（HH:mm）", () => {
	it("以台北牆上時間輸出時分，跨日窗口回 00:30 而非 16:30", () => {
		expect(formatTaipeiTime(CROSS_MIDNIGHT_ISO)).toBe("00:30");
		expect(formatTaipeiTime("2026-07-30T07:05:09Z")).toBe("15:05");
	});

	it("與 formatTaipeiDateTime 的時間段完全一致", () => {
		expect(formatTaipeiTime(CROSS_MIDNIGHT_ISO)).toBe(
			formatTaipeiDateTime(CROSS_MIDNIGHT_ISO).slice(-5),
		);
	});

	it("無值回空字串", () => {
		expect(formatTaipeiTime(0)).toBe("");
	});
});

describe("taipeiParts（自訂排版用的逃生門）", () => {
	it("回傳台北牆上時間的年月日時分與星期", () => {
		// 2026-07-30T16:30:00Z → 台北 2026/07/31 00:30，2026-07-31 是週五(5)。
		expect(taipeiParts(CROSS_MIDNIGHT_ISO)).toEqual({
			year: 2026,
			month: 7,
			day: 31,
			hour: 0,
			minute: 30,
			weekday: 5,
		});
	});

	it("weekday 以台北的日期為準，不是 UTC 的日期", () => {
		// 同一個 instant 在 UTC 還是 7/30（週四=4），台北已跨到 7/31（週五=5）。
		expect(new Date(CROSS_MIDNIGHT_ISO).getUTCDay()).toBe(4);
		expect(taipeiParts(CROSS_MIDNIGHT_ISO)?.weekday).toBe(5);
	});

	it("month 已 +1（1-12），呼叫端不必再處理 0-based", () => {
		expect(taipeiParts("2026-01-05T00:00:00Z")?.month).toBe(1);
		expect(taipeiParts("2026-12-05T00:00:00Z")?.month).toBe(12);
	});

	it("無法解讀回 null（不是丟例外，也不是回 1970）", () => {
		expect(taipeiParts(null)).toBeNull();
		expect(taipeiParts("not-a-date")).toBeNull();
		expect(taipeiParts(0)).toBeNull();
	});

	it("與各 formatter 同源：組出來的字串與具名輸出相同", () => {
		const p = taipeiParts(CROSS_MIDNIGHT_ISO);
		if (!p) throw new Error("unreachable");
		const pad = (n: number) => String(n).padStart(2, "0");
		expect(`${p.year}/${pad(p.month)}/${pad(p.day)}`).toBe(
			formatTaipeiDate(CROSS_MIDNIGHT_ISO),
		);
		expect(`${pad(p.hour)}:${pad(p.minute)}`).toBe(
			formatTaipeiTime(CROSS_MIDNIGHT_ISO),
		);
	});
});

describe("反向驗證：四個輸出都與 runtime 時區脫鉤", () => {
	it("以 UTC 欄位計算，所以 process.env.TZ 換掉不會改變輸出", () => {
		// 這一段是「守住了什麼」的證明：舊寫法在 UTC runtime 會產出 7/30，
		// 這裡四個輸出全部固定在台北的 7/31 —— 與 vitest 進程的 TZ 無關。
		const asUtcText = new Intl.DateTimeFormat("zh-TW", {
			timeZone: "UTC",
			dateStyle: "short",
		}).format(new Date(CROSS_MIDNIGHT_ISO));
		expect(asUtcText).toContain("7/30");

		expect(formatTaipeiDate(CROSS_MIDNIGHT_ISO)).toBe("2026/07/31");
		expect(formatTaipeiDateTime(CROSS_MIDNIGHT_ISO)).toBe("2026/07/31 00:30");
		expect(formatTaipeiMonthDayTime(CROSS_MIDNIGHT_ISO)).toBe("07/31 00:30");
		expect(formatTaipeiMonthDay(CROSS_MIDNIGHT_ISO)).toBe("7月31日");
		expect(formatTaipeiShortDate(CROSS_MIDNIGHT_ISO)).toBe("7/31");
		expect(formatTaipeiTime(CROSS_MIDNIGHT_ISO)).toBe("00:30");
	});

	it("反向對照：local-time getter 在 UTC runtime 下會產出不同的日／時", () => {
		// 這是 #391 第二輪要根除的寫法。vitest 進程的 TZ 由外部指定，這裡只斷言
		// 「兩者在非台北 runtime 下不相等」——TZ=Asia/Taipei 時本來就會相等，
		// 所以用 UTC 欄位當固定的對照組，不依賴進程 TZ。
		const d = new Date(CROSS_MIDNIGHT_ISO);
		const utcRendered = `${d.getUTCFullYear()}/${String(
			d.getUTCMonth() + 1,
		).padStart(2, "0")}/${String(d.getUTCDate()).padStart(2, "0")}`;
		expect(utcRendered).toBe("2026/07/30");
		expect(formatTaipeiDate(CROSS_MIDNIGHT_ISO)).toBe("2026/07/31");
		expect(utcRendered).not.toBe(formatTaipeiDate(CROSS_MIDNIGHT_ISO));
	});
});

describe("toEpochMs（秒／毫秒判別收斂點）", () => {
	// 期望值一律由 ISO 反推，不寫死憑印象的 epoch 數字。
	const MS = Date.parse(CROSS_MIDNIGHT_ISO);
	const SECONDS = Math.floor(MS / 1000);

	it("unix 秒放大 1000 倍、毫秒原樣通過", () => {
		expect(toEpochMs(SECONDS)).toBe(MS);
		expect(toEpochMs(MS)).toBe(MS);
	});

	it("純數字字串走同一條判別；非數字字串交給 Date.parse", () => {
		expect(toEpochMs(String(SECONDS))).toBe(MS);
		expect(toEpochMs(String(MS))).toBe(MS);
		expect(toEpochMs(CROSS_MIDNIGHT_ISO)).toBe(MS);
	});

	it("無值 / 不可解析 / epoch 0 一律 null", () => {
		expect(toEpochMs(null)).toBeNull();
		expect(toEpochMs(undefined)).toBeNull();
		expect(toEpochMs("")).toBeNull();
		expect(toEpochMs("not-a-date")).toBeNull();
		expect(toEpochMs(0)).toBeNull();
		expect(toEpochMs(new Date(Number.NaN))).toBeNull();
	});
});

describe("既有 re-export 名稱仍指向同一份實作", () => {
	it("features/sellers/reviews 的 formatReviewDate 與共用 helper 同輸出", async () => {
		const { formatReviewDate } = await import(
			"@/features/sellers/reviews/formatReviewDate"
		);
		expect(formatReviewDate(CROSS_MIDNIGHT_ISO)).toBe(
			formatTaipeiDate(CROSS_MIDNIGHT_ISO),
		);
		expect(formatReviewDate(CROSS_MIDNIGHT_ISO)).toBe("2026/07/31");
	});
});
