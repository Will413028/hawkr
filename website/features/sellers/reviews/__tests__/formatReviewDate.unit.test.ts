import { describe, expect, it } from "vitest";
import { formatReviewDate } from "../formatReviewDate";

/**
 * Regression: hawkr-website#374 — 賣家評價列表用
 * `new Date(createdAt).toLocaleDateString("zh-TW")` 渲染日期。第一頁是 server
 * render（Workers runtime = UTC）、hydration 後由瀏覽器時區重算，16:00Z 之後
 * 的評價兩邊差一天 → React #418，且量大到把 Sentry ingest 打成 429。
 *
 * 這支測試釘住「輸出與 runtime 時區脫鉤」：
 *  - 反向驗證段先證明舊寫法真的會隨時區飄（否則本測試沒有守住任何東西）
 *  - 正向段釘住固定台北日期字串；CI／本地以 `TZ=<任意值> vitest` 跑都必須一致
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

		// 這就是 #418 的來源：server(UTC) 與 client(台北) 的文字節點不同。
		expect(asUtc).not.toBe(asTaipei);
	});
});

describe("formatReviewDate", () => {
	it("跨日窗口固定回傳台北當日（不隨 runtime 時區改變）", () => {
		expect(formatReviewDate(CROSS_MIDNIGHT_ISO)).toBe("2026/07/31");
	});

	it("同日窗口回傳同一天", () => {
		expect(formatReviewDate(SAME_DAY_ISO)).toBe("2026/07/30");
	});

	it("以 UTC 欄位計算，因此不呼叫任何 local-time getter", () => {
		// 若實作誤用 getFullYear/getMonth/getDate，把 process.env.TZ 換掉就會炸。
		// 這裡直接以「同一 instant 的三種輸入型別」交叉核對輸出恆等。
		const iso = CROSS_MIDNIGHT_ISO;
		const ms = new Date(iso).getTime();
		const dateObject = new Date(iso);

		expect(formatReviewDate(iso)).toBe("2026/07/31");
		expect(formatReviewDate(ms)).toBe("2026/07/31");
		expect(formatReviewDate(dateObject)).toBe("2026/07/31");
	});

	it("補零到兩位，對齊 repo 既有 YYYY/MM/DD 呈現", () => {
		expect(formatReviewDate("2026-01-05T00:00:00Z")).toBe("2026/01/05");
	});

	it("台北時間的月／年邊界不會被偏移算錯", () => {
		// 2025-12-31T16:00:00Z = 2026-01-01 00:00 台北
		expect(formatReviewDate("2025-12-31T16:00:00Z")).toBe("2026/01/01");
		// 2026-01-31T16:00:00Z = 2026-02-01 00:00 台北
		expect(formatReviewDate("2026-01-31T16:00:00Z")).toBe("2026/02/01");
	});

	it("空值與不可解析輸入回傳空字串（不渲染 Invalid Date）", () => {
		expect(formatReviewDate(undefined)).toBe("");
		expect(formatReviewDate(null)).toBe("");
		expect(formatReviewDate("")).toBe("");
		expect(formatReviewDate("not-a-date")).toBe("");
	});
});
