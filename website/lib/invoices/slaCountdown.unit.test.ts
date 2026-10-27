import { describe, expect, it } from "vitest";
import { foldSlaCountdown } from "./slaCountdown";

/**
 * 這張表守的是 §0.3 的四條規則與 §B-1.0.9 B06t 的那個坑：
 * `remainingBusinessDays` 是**數字字串且可為負**，直接拿去比大小是**字典序**——
 * `"-2" >= "3"` 為 false，而 `"10" >= "3"` 也為 false ⇒ 逾期與充裕會被歸成同一
 * 級，而畫面看起來完全正常。
 */
describe("工作天倒數的折疊表", () => {
	it("後端算不出來（空字串）時顯示「處理期限計算中」，不退化成日曆天", () => {
		const display = foldSlaCountdown("", "");
		expect(display.unknown).toBe(true);
		expect(display.text).toBe("處理期限計算中");
		// 不是 danger：算不出來不等於逾期。
		expect(display.tone).toBe("neutral");
		// 沒有任何數字被算出來。
		expect(display.text).not.toMatch(/\d/);
	});

	it("非數字字串一樣退回計算中，不顯示 NaN", () => {
		expect(foldSlaCountdown("abc", "").text).toBe("處理期限計算中");
		expect(foldSlaCountdown("abc", "").text).not.toContain("NaN");
	});

	it("字典序陷阱：兩位數是充裕、負數是逾期，兩者不可同級", () => {
		expect(foldSlaCountdown("10", "").tone).toBe("neutral");
		expect(foldSlaCountdown("-2", "2").tone).toBe("danger");
		// 若拿字串直接比大小，這兩個會落進同一個分支。
		expect(foldSlaCountdown("10", "").tone).not.toBe(
			foldSlaCountdown("-2", "2").tone,
		);
	});

	it("分級：>= 3 neutral／1–2 warning／逾期 danger，且都同時輸出文字", () => {
		expect(foldSlaCountdown("3", "").tone).toBe("neutral");
		expect(foldSlaCountdown("2", "").tone).toBe("warning");
		expect(foldSlaCountdown("1", "").tone).toBe("warning");
		expect(foldSlaCountdown("0", "").tone).toBe("warning");
		expect(foldSlaCountdown("3", "").text).toBe("剩餘 3 個工作天");
		expect(foldSlaCountdown("1", "").text).toBe("剩餘 1 個工作天");
	});

	it("0 天是「今天到期」不是「剩餘 0 個工作天」", () => {
		// 「剩餘 0 個工作天」會被讀成「還有時間」，而它其實是今天最後一天。
		expect(foldSlaCountdown("0", "").text).toBe("今天到期");
		expect(foldSlaCountdown("0", "").text).not.toContain("0");
		// 今天到期還沒逾期 ⇒ 不是 danger。
		expect(foldSlaCountdown("0", "").tone).toBe("warning");
		expect(foldSlaCountdown("0", "").unknown).toBe(false);
	});

	it("逾期用 overdueBusinessDays 的正數表達，後端沒給就用 remaining 的絕對值", () => {
		expect(foldSlaCountdown("-2", "2").text).toBe("已逾期 2 個工作天");
		expect(foldSlaCountdown("-5", "").text).toBe("已逾期 5 個工作天");
		// 不得出現負號——「已逾期 -5 個工作天」讀不通。
		expect(foldSlaCountdown("-5", "").text).not.toContain("-");
	});
});
