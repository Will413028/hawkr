import { describe, expect, it } from "vitest";
import {
	coerceWeightUnitCode,
	formatUnitLabel,
	SELLER_SUSPENSION_REASON_LABEL,
	SellerSuspensionReason,
} from "../enums";

describe("formatUnitLabel", () => {
	it("maps WeightUnit codes to Chinese labels", () => {
		expect(formatUnitLabel("3301")).toBe("公斤");
		expect(formatUnitLabel("3302")).toBe("台斤");
		expect(formatUnitLabel("3303")).toBe("公克");
	});
	it("passes through non-code legacy labels unchanged", () => {
		expect(formatUnitLabel("件")).toBe("件");
		expect(formatUnitLabel("盒")).toBe("盒");
	});
	it("returns empty string for empty/undefined", () => {
		expect(formatUnitLabel("")).toBe("");
		expect(formatUnitLabel(undefined)).toBe("");
		expect(formatUnitLabel(null)).toBe("");
	});
});

describe("coerceWeightUnitCode", () => {
	it("keeps valid WeightUnit codes", () => {
		expect(coerceWeightUnitCode("3301")).toBe("3301");
		expect(coerceWeightUnitCode("3302")).toBe("3302");
		expect(coerceWeightUnitCode("3303")).toBe("3303");
	});
	it("maps legacy Chinese weight labels back to codes", () => {
		expect(coerceWeightUnitCode("公斤")).toBe("3301");
		expect(coerceWeightUnitCode("台斤")).toBe("3302");
		expect(coerceWeightUnitCode("公克")).toBe("3303");
	});
	it("falls back to KG for non-weight units, empty, or undefined", () => {
		expect(coerceWeightUnitCode("件")).toBe("3301");
		expect(coerceWeightUnitCode("盒")).toBe("3301");
		expect(coerceWeightUnitCode("")).toBe("3301");
		expect(coerceWeightUnitCode(undefined)).toBe("3301");
	});
});

// 回歸釘（issue #1514 驗收，反轉 #1495 舊裁決）：後端 /app/sellers/me 的
// suspensionReasonCode 傳輸值已統一改送 CodeMeta Value 數字碼（"9601" 等），
// 不再是 Name 字串（"POLICY_VIOLATION"）。SellerSuspendedNotice 以該原始字串
// 直接查 SELLER_SUSPENSION_REASON_LABEL——這組測試釘住「數字碼鍵可命中」，
// 防止未來誤用手寫 Name 鍵 shadow 版本造成停權原因顯示回歸（./enums.ts 已移除
// #1495 當時為了 shadow 舊傳輸值而加的手寫覆蓋，改用 ./generated 帶出的版本）。
describe("SELLER_SUSPENSION_REASON_LABEL", () => {
	it("is keyed by the wire value (CodeMeta numeric Value) the backend sends", () => {
		// 模擬 SellerSuspendedNotice：以後端原樣透傳的字串直接查表
		const wireLookup = (code: string): string | undefined =>
			(SELLER_SUSPENSION_REASON_LABEL as Record<string, string>)[code];
		expect(wireLookup("9601")).toBe("違反平台規範");
		expect(wireLookup("9602")).toBe("屢勸不改");
		expect(wireLookup("9603")).toBe("其他");
	});
	it("covers every SellerSuspensionReason value (no silent fallback for known codes)", () => {
		for (const wireValue of Object.values(SellerSuspensionReason)) {
			expect(SELLER_SUSPENSION_REASON_LABEL[wireValue]).toBeTruthy();
		}
	});
	it("misses on unknown/legacy Name codes so the notice falls back to generic copy", () => {
		// SellerSuspendedNotice.tsx reasonLabel(): 查不中 → "帳號已被平台停權"
		const table: Record<string, string> = SELLER_SUSPENSION_REASON_LABEL;
		for (const unknown of ["POLICY_VIOLATION", ""]) {
			expect(table[unknown]).toBeUndefined();
		}
	});
});
