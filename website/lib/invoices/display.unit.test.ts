import { describe, expect, it } from "vitest";
import { InvoiceDocumentKind } from "@/lib/codemeta/generated";
import {
	formatFileSize,
	formatInvoiceAmount,
	hasRecipientTaxId,
	shipmentStatusDisplay,
	taipeiDaysUntil,
} from "./display";

describe("formatInvoiceAmount", () => {
	it("adds thousands separators without touching the value", () => {
		expect(formatInvoiceAmount("4300")).toBe("NT$ 4,300");
		expect(formatInvoiceAmount("0")).toBe("NT$ 0");
		expect(formatInvoiceAmount("-150")).toBe("NT$ -150");
	});

	it("passes non-numeric input through instead of rendering NaN", () => {
		expect(formatInvoiceAmount("")).toBe("");
		expect(formatInvoiceAmount("abc")).toBe("abc");
	});
});

describe("formatFileSize", () => {
	it("renders bytes / KB / MB", () => {
		expect(formatFileSize("512")).toBe("512 B");
		expect(formatFileSize("2048")).toBe("2 KB");
		expect(formatFileSize("3145728")).toBe("3.0 MB");
	});

	it("returns empty for missing or unparseable sizes", () => {
		expect(formatFileSize("")).toBe("");
		expect(formatFileSize("0")).toBe("");
	});
});

describe("taipeiDaysUntil", () => {
	// 58A §S01 驗收第 2 項的機械版：期限固定，「現在」跨過台北日界，天數必須 -1。
	// 用 `Math.ceil(diff / 86400000)` 的寫法在這兩個時刻都會給 42（§B-2.0 T12）。
	const deadline = "2026-10-11T15:59:00Z"; // 台北 2026/10/11 23:59

	it("counts Taipei day boundaries, not 24-hour multiples", () => {
		// 台北 2026/08/30 23:50 ＝ UTC 15:50 同日
		const before = Date.parse("2026-08-30T15:50:00Z");
		// 台北 2026/08/31 00:10 ＝ UTC 16:10 前一日（僅 20 分鐘後）
		const after = Date.parse("2026-08-30T16:10:00Z");

		expect(taipeiDaysUntil(deadline, before)).toBe(42);
		expect(taipeiDaysUntil(deadline, after)).toBe(41);
	});

	it("goes negative once the deadline has passed", () => {
		const now = Date.parse("2026-10-13T04:00:00Z"); // 台北 10/13 12:00
		expect(taipeiDaysUntil(deadline, now)).toBe(-2);
	});

	it("returns null (render nothing) when the deadline is missing or unparseable", () => {
		// 🔴 不得退回 0 —— 「剩餘 0 天」會被讀成「今天到期」，而實際情況是我們根本
		// 不知道期限。
		expect(taipeiDaysUntil("")).toBeNull();
		expect(taipeiDaysUntil("not-a-date")).toBeNull();
	});
});

describe("買受人統編的可見性（#3012）", () => {
	// 🔴 十個零是 chk_invdoc_b2c 強制的哨兵值，不是資料。它是非空字串 ⇒ truthy，
	// 所以「純真值判斷」這個 bug 只會在 B2C 上現形，B2B 看起來一切正常。
	const B2C_SENTINEL = "0000000000";

	it("B2C（二聯式）不顯示——十個零是哨兵值不是統編", () => {
		expect(hasRecipientTaxId(InvoiceDocumentKind.B2C, B2C_SENTINEL)).toBe(
			false,
		);
	});

	it("陰性對照：B2B（三聯式）必須仍顯示真統編", () => {
		// 這一條是修復不得把 B2B 一起關掉的保證。整列拿掉是最容易的「修法」，
		// 而它會讓三聯式的買受人統編從賣家與買家兩個畫面同時消失。
		expect(hasRecipientTaxId(InvoiceDocumentKind.B2B, "87654321")).toBe(true);
	});

	it("RECEIPT（平台代製收據）不顯示——買受人是自然人", () => {
		expect(hasRecipientTaxId(InvoiceDocumentKind.RECEIPT, B2C_SENTINEL)).toBe(
			false,
		);
		expect(hasRecipientTaxId(InvoiceDocumentKind.RECEIPT, "87654321")).toBe(
			false,
		);
	});

	it("資料漂移時 fail-closed：kind 是 B2B 但值不是 8 碼統編也不顯示", () => {
		// chk_invdoc_b2b 的鏡像。這一格擋的是「B2B 卻帶著哨兵值」——
		// 只靠 documentKind 一個條件的話，十個零會從三聯式那條路溜出去。
		expect(hasRecipientTaxId(InvoiceDocumentKind.B2B, B2C_SENTINEL)).toBe(
			false,
		);
		expect(hasRecipientTaxId(InvoiceDocumentKind.B2B, "")).toBe(false);
		expect(hasRecipientTaxId(InvoiceDocumentKind.B2B, "8765432")).toBe(false);
		expect(hasRecipientTaxId(InvoiceDocumentKind.B2B, "876543210")).toBe(false);
		expect(hasRecipientTaxId(InvoiceDocumentKind.B2B, "8765432A")).toBe(false);
	});

	it("後端新增第四種 documentKind 時 fail-closed，不是樂觀顯示", () => {
		expect(hasRecipientTaxId("B2G", "87654321")).toBe(false);
	});
});

// --- #3019：出貨狀態的三態 ---------------------------------------------------

// 🔴 風險不在「有沒有畫」，在**空字串代表什麼**。
// shipping-service 抖動時後端整批回空字串並把 shipmentStatusDegraded 設 true；
// 只看 code 的實作會讓每一列都畫成「還沒出貨」——一句我們無從得知的話。
// 這一欄在 backend#3019 之前後端恆為空字串，正是為了不說這句話而只畫 `—`。
describe("shipmentStatusDisplay", () => {
	it("降級與『還沒出貨』是兩種顯示，不得塌成同一格", () => {
		expect(shipmentStatusDisplay("", true)).toEqual({ kind: "degraded" });
		expect(shipmentStatusDisplay("", false)).toEqual({ kind: "none" });
	});

	it("有值時走 CodeMeta 查表，不把碼值畫給賣家看", () => {
		expect(shipmentStatusDisplay("210", false)).toEqual({
			kind: "status",
			label: "已叫件",
		});
		// 表上沒有的碼原樣顯示，不猜也不吞。
		expect(shipmentStatusDisplay("9999", false)).toEqual({
			kind: "status",
			label: "9999",
		});
	});

	it("判定順序：degraded 先判", () => {
		// 降級時就算後端漏回了某一列的碼值，也不該拿它去說「這一列出貨了」——
		// 整批都是不可信的。
		expect(shipmentStatusDisplay("210", true)).toEqual({ kind: "degraded" });
	});
});
