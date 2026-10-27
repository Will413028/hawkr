import { describe, expect, it } from "vitest";
import { formatTaipeiDateTime } from "./format";

/**
 * 邀請到期時間在 wire 上是 RFC3339 UTC（`2026-09-19T04:12:07Z`）。直接印給
 * 站外業者看等於要他自己換算時區——而設計書對所有 buyer／seller-facing 時間
 * 的要求就是標明 Asia/Taipei。
 */
describe("formatTaipeiDateTime", () => {
	it("把 UTC 時間換算成台北時間", () => {
		expect(formatTaipeiDateTime("2026-09-19T04:12:07Z")).toBe(
			"2026-09-19 12:12",
		);
	});

	it("跨日的換算不會少一天", () => {
		expect(formatTaipeiDateTime("2026-09-19T16:30:00Z")).toBe(
			"2026-09-20 00:30",
		);
	});

	it("解析不出來就原樣回傳，不讓畫面炸掉也不假裝換算過", () => {
		expect(formatTaipeiDateTime("not-a-date")).toBe("not-a-date");
		expect(formatTaipeiDateTime("")).toBe("");
	});
});
