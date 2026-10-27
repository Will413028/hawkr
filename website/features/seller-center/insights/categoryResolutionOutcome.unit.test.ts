import { describe, expect, it } from "vitest";
import type { CategoryResolutionData } from "@/lib/pricebook/repo";
import { resolveCategoryResolutionOutcome } from "./categoryResolutionOutcome";

function makeData(
	overrides: Partial<CategoryResolutionData> = {},
): CategoryResolutionData {
	return {
		categoryId: "123",
		status: "resolved",
		groupCode: "G1",
		label: "包心白",
		mappingVersion: "pricebook-category-v1",
		...overrides,
	};
}

describe("resolveCategoryResolutionOutcome", () => {
	it("resolves a group selection", () => {
		expect(resolveCategoryResolutionOutcome(makeData())).toEqual({
			kind: "resolved",
			selection: { kind: "group", groupCode: "G1" },
			label: "包心白",
			signature: "group:G1",
		});
	});

	it("falls back to the raw code as label when the backend omits one", () => {
		expect(
			resolveCategoryResolutionOutcome(makeData({ label: undefined })),
		).toEqual({
			kind: "resolved",
			selection: { kind: "group", groupCode: "G1" },
			label: "G1",
			signature: "group:G1",
		});
	});

	it("treats unmapped as unmapped", () => {
		expect(
			resolveCategoryResolutionOutcome(makeData({ status: "unmapped" })),
		).toEqual({ kind: "unmapped" });
	});

	it("treats ambiguous as ambiguous", () => {
		expect(
			resolveCategoryResolutionOutcome(makeData({ status: "ambiguous" })),
		).toEqual({ kind: "ambiguous" });
	});

	it("fails closed to invalid when resolved is missing its groupCode", () => {
		expect(
			resolveCategoryResolutionOutcome(makeData({ groupCode: undefined })),
		).toEqual({ kind: "invalid" });
	});

	it("never guesses from a null response", () => {
		expect(resolveCategoryResolutionOutcome(null)).toEqual({
			kind: "unmapped",
		});
	});
});

describe("resolveCategoryResolutionOutcome — 品種收窄", () => {
	it("恰好一個品種時鎖定該品種，而不是大類", () => {
		expect(
			resolveCategoryResolutionOutcome(
				makeData({
					cropVarieties: [
						{ cropCode: "LA5", name: "甘藍-甘藍心", variantName: "甘藍心" },
					],
				}),
			),
		).toEqual({
			kind: "resolved",
			selection: { kind: "crop", cropCode: "LA5" },
			label: "甘藍心",
			signature: "crop:LA5",
		});
	});

	it("兩個品種（國產＋進口）維持大類——單一碼表達不了集合，硬挑一個會排除另一半", () => {
		expect(
			resolveCategoryResolutionOutcome(
				makeData({
					label: "甘藍",
					cropVarieties: [
						{ cropCode: "LA1", name: "甘藍-初秋", variantName: "初秋" },
						{
							cropCode: "LA91",
							name: "甘藍-進口 初秋",
							variantName: "進口 初秋",
						},
					],
				}),
			),
		).toEqual({
			kind: "resolved",
			selection: { kind: "group", groupCode: "G1" },
			label: "甘藍",
			signature: "group:G1",
		});
	});

	it("空集合＝沒有收窄，維持大類（不是「查無品種」）", () => {
		expect(
			resolveCategoryResolutionOutcome(makeData({ cropVarieties: [] })),
		).toEqual({
			kind: "resolved",
			selection: { kind: "group", groupCode: "G1" },
			label: "包心白",
			signature: "group:G1",
		});
	});

	it("variantName 空白時退回 name", () => {
		const outcome = resolveCategoryResolutionOutcome(
			makeData({
				cropVarieties: [
					{ cropCode: "LA5", name: "甘藍-甘藍心", variantName: "  " },
				],
			}),
		);
		expect(outcome).toMatchObject({ label: "甘藍-甘藍心" });
	});
});
