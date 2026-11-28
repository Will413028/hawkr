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
	const deadline = "2026-10-11T15:59:00Z";

	it("counts Taipei day boundaries, not 24-hour multiples", () => {
		const before = Date.parse("2026-08-30T15:50:00Z");
		const after = Date.parse("2026-08-30T16:10:00Z");

		expect(taipeiDaysUntil(deadline, before)).toBe(42);
		expect(taipeiDaysUntil(deadline, after)).toBe(41);
	});

	it("goes negative once the deadline has passed", () => {
		const now = Date.parse("2026-10-13T04:00:00Z");
		expect(taipeiDaysUntil(deadline, now)).toBe(-2);
	});

	it("returns null (render nothing) when the deadline is missing or unparseable", () => {
		expect(taipeiDaysUntil("")).toBeNull();
		expect(taipeiDaysUntil("not-a-date")).toBeNull();
	});
});

describe("買受人統編的可見性（#3012）", () => {
	const B2C_SENTINEL = "0000000000";

	it("B2C（二聯式）不顯示——十個零是哨兵值不是統編", () => {
		expect(hasRecipientTaxId(InvoiceDocumentKind.B2C, B2C_SENTINEL)).toBe(
			false,
		);
	});

	it("陰性對照：B2B（三聯式）必須仍顯示真統編", () => {
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
		expect(shipmentStatusDisplay("9999", false)).toEqual({
			kind: "status",
			label: "9999",
		});
	});

	it("判定順序：degraded 先判", () => {
		expect(shipmentStatusDisplay("210", true)).toEqual({ kind: "degraded" });
	});
});
