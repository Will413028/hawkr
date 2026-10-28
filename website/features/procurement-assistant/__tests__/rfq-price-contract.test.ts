import { describe, expect, it } from "vitest";
import {
	hasConfirmablePriceBasis,
	rfqConfirmationSchema,
	rfqPricePreviewSchema,
	validateBuyerDesiredRange,
} from "../rfq-price-contract";

const previewHash = "a".repeat(64);

function basePreview() {
	return {
		pricePreviewId: "1234567890123456801",
		version: "1",
		buyerRequestedRange: {
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: "30",
			upperBoundAmount: "42",
			basisQuantity: "1",
			basisUnit: "KG",
		},
		payloadHash: previewHash,
		quoteTotalRange: {
			currency: "TWD",
			lowerBoundAmount: "15000",
			upperBoundAmount: "21000",
			quantity: "500",
			quantityUnit: "KG",
		},
	};
}

describe("RFQ price preview contract", () => {
	it("accepts the backend projection and treats its basis as confirmable", () => {
		const parsed = rfqPricePreviewSchema.parse(basePreview());
		expect(parsed.buyerRequestedRange.lowerBoundAmount).toBe("30");
		expect(hasConfirmablePriceBasis(parsed)).toBe(true);
	});

	it("keeps an empty basis unconfirmable", () => {
		const parsed = rfqPricePreviewSchema.parse({
			...basePreview(),
			buyerRequestedRange: {
				...basePreview().buyerRequestedRange,
				basisQuantity: "",
				basisUnit: "",
			},
		});
		expect(hasConfirmablePriceBasis(parsed)).toBe(false);
	});

	it("requires the buyer range: a preview without one is rejected", () => {
		const { buyerRequestedRange: _range, ...withoutRange } = basePreview();
		expect(rfqPricePreviewSchema.safeParse(withoutRange).success).toBe(false);
		expect(
			rfqPricePreviewSchema.safeParse({
				...basePreview(),
				buyerRequestedRange: null,
			}).success,
		).toBe(false);
	});

	it("rejects a descending buyer range", () => {
		expect(
			rfqPricePreviewSchema.safeParse({
				...basePreview(),
				buyerRequestedRange: {
					...basePreview().buyerRequestedRange,
					lowerBoundAmount: "43",
					upperBoundAmount: "42",
				},
			}).success,
		).toBe(false);
	});

	it.each([
		["1", "KG", true],
		["1", "CATTY", true],
		["1000", "GRAM", true],
		["1", "GRAM", false],
		["1000", "KG", false],
		["1", "BOX", false],
	] as const)("checks canonical basis %s %s", (basisQuantity, basisUnit, expected) => {
		const parsed = rfqPricePreviewSchema.parse({
			...basePreview(),
			buyerRequestedRange: {
				...basePreview().buyerRequestedRange,
				basisQuantity,
				basisUnit,
			},
		});
		expect(hasConfirmablePriceBasis(parsed)).toBe(expected);
	});

	it("normalises a missing total range to null", () => {
		const { quoteTotalRange: _total, ...withoutTotal } = basePreview();
		const parsed = rfqPricePreviewSchema.parse(withoutTotal);
		expect(parsed.quoteTotalRange).toBeNull();
	});

	it.each([
		{
			quoteTotalRange: {
				...basePreview().quoteTotalRange,
				lowerBoundAmount: "21000",
				upperBoundAmount: "15000",
			},
		},
		{
			quoteTotalRange: { ...basePreview().quoteTotalRange, quantity: "0" },
		},
		{
			quoteTotalRange: {
				...basePreview().quoteTotalRange,
				quantityUnit: "",
			},
		},
	])("rejects an inconsistent total-range projection", (override) => {
		expect(
			rfqPricePreviewSchema.safeParse({ ...basePreview(), ...override })
				.success,
		).toBe(false);
	});

	it("binds a confirmable basis to the persisted request unit", () => {
		const parsed = rfqPricePreviewSchema.parse(basePreview());
		expect(hasConfirmablePriceBasis(parsed, "KG")).toBe(true);
		expect(hasConfirmablePriceBasis(parsed, "CATTY")).toBe(false);
	});

	it.each([
		"01",
		" 1",
		"1 ",
		"1e3",
		"-1",
		"0",
		"1.12345",
		"1000000000000",
	])("rejects a non-canonical server TWD amount %s", (amount) => {
		expect(
			rfqPricePreviewSchema.safeParse({
				...basePreview(),
				buyerRequestedRange: {
					...basePreview().buyerRequestedRange,
					lowerBoundAmount: amount,
				},
			}).success,
		).toBe(false);
	});

	it("rejects decimal amounts in the buyer range", () => {
		expect(
			rfqPricePreviewSchema.safeParse({
				...basePreview(),
				buyerRequestedRange: {
					...basePreview().buyerRequestedRange,
					lowerBoundAmount: "30.25",
				},
			}).success,
		).toBe(false);
	});

	it.each([
		"0",
		"abc",
		" 1",
		"9223372036854775808",
		"10000000000000000000",
	])("rejects a non-canonical or out-of-range int64 id %s", (id) => {
		expect(
			rfqPricePreviewSchema.safeParse({
				...basePreview(),
				pricePreviewId: id,
			}).success,
		).toBe(false);
	});
});

describe("RFQ confirmation and manual input", () => {
	it("accepts a confirmed projection hash that differs from the preview hash", () => {
		const result = rfqConfirmationSchema.parse({
			confirmedRfqSnapshotId: "1234567890123456802",
			payloadHash: "b".repeat(64),
		});
		expect(result.payloadHash).not.toBe(previewHash);
	});

	// R-01 ⑤「有效至」的來源。docx V.7.5 把 expiry 列為 snapshot 自己的固定屬
	// 性，所以它落在 confirmation 回應而不是 price preview（backend PR #3381）。
	it("accepts the snapshot expiry the confirmation response carries", () => {
		const result = rfqConfirmationSchema.parse({
			confirmedRfqSnapshotId: "1234567890123456802",
			payloadHash: "b".repeat(64),
			expiresAt: "2026-08-22T12:36:00Z",
		});
		expect(result.expiresAt).toBe("2026-08-22T12:36:00Z");
	});

	// 與 qualityKnowledgeState 同一個慣例：後端加這個欄位之前確認的快照沒有它，
	// 畫面整列不顯示，而不是補一個當下時間假裝快照有到期日。
	it("normalises a missing snapshot expiry to an empty string", () => {
		const result = rfqConfirmationSchema.parse({
			confirmedRfqSnapshotId: "1234567890123456802",
			payloadHash: "b".repeat(64),
		});
		expect(result.expiresAt).toBe("");
	});

	it("requires a complete ascending positive-integer buyer range", () => {
		expect(validateBuyerDesiredRange("30", "42")).toBeNull();
		expect(validateBuyerDesiredRange("", "42")).toContain("完整填寫");
		expect(validateBuyerDesiredRange("1.5", "42")).toContain("整數");
		expect(validateBuyerDesiredRange("43", "42")).toContain("不可低於");
	});
});
