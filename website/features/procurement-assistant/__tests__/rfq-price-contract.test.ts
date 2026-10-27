import { describe, expect, it } from "vitest";
import {
	hasConfirmablePriceBasis,
	rfqConfirmationSchema,
	rfqPricePreviewSchema,
	validateBuyerDesiredRange,
} from "../rfq-price-contract";

const previewHash = "a".repeat(64);

function guidedPreview() {
	return {
		pricePreviewId: "1234567890123456801",
		version: "1",
		pricingMode: "PRICEBOOK_GUIDED",
		referenceAvailability: "AVAILABLE",
		referencePriceRange: {
			lowerBoundAmount: "30",
			midpointAmount: "36",
			upperBoundAmount: "42",
		},
		buyerRequestedRange: {
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: "30",
			upperBoundAmount: "42",
			basisQuantity: "1",
			basisUnit: "KG",
		},
		dealTemperature: { status: "UNKNOWN", reasonCode: "" },
		payloadHash: previewHash,
		marketPriceBasis: {
			regionName: "桃園市",
			supportingMarketCount: "3",
			sourceDataAsOf: "2026-08-14T00:00:00Z",
			freshnessStatus: "fresh",
		},
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
	it("accepts the backend guided projection with an empty temperature reason", () => {
		const parsed = rfqPricePreviewSchema.parse(guidedPreview());
		expect(parsed.dealTemperature.reasonCode).toBe("");
		expect(hasConfirmablePriceBasis(parsed)).toBe(true);
	});

	it("accepts a fully described AVAILABLE deal temperature", () => {
		const parsed = rfqPricePreviewSchema.parse({
			...guidedPreview(),
			dealTemperature: {
				status: "AVAILABLE",
				reasonCode: "",
				probabilityLowerBps: "4100",
				probabilityUpperBps: "7800",
				band: "WARM",
				outcomeDefinition: "BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE",
				outcomeDefinitionVersion: "1",
				policyVersion: "DEAL_TEMPERATURE_COVERAGE_V1",
				bandVersion: "BANDS_V1",
			},
		});
		expect(parsed.dealTemperature.status).toBe("AVAILABLE");
		if (parsed.dealTemperature.status !== "AVAILABLE") {
			throw new Error("unreachable");
		}
		expect(parsed.dealTemperature.band).toBe("WARM");
	});

	it("fails closed when an AVAILABLE deal temperature omits a version field", () => {
		expect(() =>
			rfqPricePreviewSchema.parse({
				...guidedPreview(),
				dealTemperature: {
					status: "AVAILABLE",
					reasonCode: "",
					probabilityLowerBps: "4100",
					probabilityUpperBps: "7800",
					band: "WARM",
					outcomeDefinition: "BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE",
					outcomeDefinitionVersion: "1",
					policyVersion: "DEAL_TEMPERATURE_COVERAGE_V1",
					// bandVersion 缺席
				},
			}),
		).toThrow();
	});

	it("still parses PRICEBOOK_GUIDED with the new UNKNOWN reason code", () => {
		const parsed = rfqPricePreviewSchema.parse({
			...guidedPreview(),
			dealTemperature: { status: "UNKNOWN", reasonCode: "BUYER_RANGE_NOT_SET" },
		});
		expect(parsed.dealTemperature.status).toBe("UNKNOWN");
	});

	it("fails closed when an UNKNOWN deal temperature carries AVAILABLE-only fields", () => {
		// zod 的預設行為是靜默剝掉 schema 沒宣告的欄位，不是拒絕它們——UNKNOWN
		// 分支沒列出機率／band／版本欄位，不代表帶著這些欄位的 payload 會被拒絕。
		// 這支測試釘住 fail-closed 的第二個方向：兩個分支都要 `.strict()`。
		expect(() =>
			rfqPricePreviewSchema.parse({
				...guidedPreview(),
				dealTemperature: {
					status: "UNKNOWN",
					reasonCode: "",
					probabilityLowerBps: "4100",
					probabilityUpperBps: "7800",
					band: "WARM",
					outcomeDefinition: "BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE",
					outcomeDefinitionVersion: "1",
					policyVersion: "DEAL_TEMPERATURE_COVERAGE_V1",
					bandVersion: "BANDS_V1",
				},
			}),
		).toThrow();
	});

	it("accepts canonical fixed-decimal Pricebook wire amounts", () => {
		const parsed = rfqPricePreviewSchema.parse({
			...guidedPreview(),
			referencePriceRange: {
				lowerBoundAmount: "30.25",
				midpointAmount: "35.5",
				upperBoundAmount: "41.75",
			},
			buyerRequestedRange: {
				...guidedPreview().buyerRequestedRange,
				lowerBoundAmount: "30.25",
				upperBoundAmount: "41.75",
			},
		});
		expect(parsed.referencePriceRange?.midpointAmount).toBe("35.5");
	});

	it("accepts no-reference wire data but keeps an empty basis unconfirmable", () => {
		const parsed = rfqPricePreviewSchema.parse({
			...guidedPreview(),
			pricingMode: "BUYER_ENTERED_NO_REFERENCE",
			referenceAvailability: "UNAVAILABLE",
			referencePriceRange: null,
			marketPriceBasis: null,
			buyerRequestedRange: {
				...guidedPreview().buyerRequestedRange,
				basisQuantity: "",
				basisUnit: "",
			},
			dealTemperature: {
				status: "UNKNOWN",
				reasonCode: "MAPPING_MISSING",
			},
		});
		expect(hasConfirmablePriceBasis(parsed)).toBe(false);
	});

	it.each([
		{ referenceAvailability: "UNAVAILABLE" },
		// dealTemperature.reasonCode 曾與 pricingMode 耦合在這條 superRefine 裡；
		// Task 6 拆掉了這條耦合（reasonCode 現在完全獨立於 pricingMode），所以
		// 這個 override 不再是「不一致」——移除這個 case，不是掩蓋回歸。
		{
			referencePriceRange: {
				lowerBoundAmount: "42",
				midpointAmount: "36",
				upperBoundAmount: "30",
			},
		},
		{
			buyerRequestedRange: {
				...guidedPreview().buyerRequestedRange,
				lowerBoundAmount: "43",
				upperBoundAmount: "42",
			},
		},
	])("rejects inconsistent or descending projections", (override) => {
		expect(
			rfqPricePreviewSchema.safeParse({ ...guidedPreview(), ...override })
				.success,
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
			...guidedPreview(),
			buyerRequestedRange: {
				...guidedPreview().buyerRequestedRange,
				basisQuantity,
				basisUnit,
			},
		});
		expect(hasConfirmablePriceBasis(parsed)).toBe(expected);
	});

	it("normalises both v6 additions to null when the backend omits them", () => {
		const {
			marketPriceBasis: _basis,
			quoteTotalRange: _total,
			...withoutAdditions
		} = guidedPreview();
		const parsed = rfqPricePreviewSchema.parse({
			...withoutAdditions,
			referenceAvailability: "UNAVAILABLE",
			referencePriceRange: null,
			pricingMode: "BUYER_ENTERED_NO_REFERENCE",
			dealTemperature: { status: "UNKNOWN", reasonCode: "MAPPING_MISSING" },
		});
		expect(parsed.marketPriceBasis).toBeNull();
		expect(parsed.quoteTotalRange).toBeNull();
	});

	it("keeps the total range independent of the market basis", () => {
		const parsed = rfqPricePreviewSchema.parse({
			...guidedPreview(),
			pricingMode: "BUYER_ENTERED_NO_REFERENCE",
			referenceAvailability: "UNAVAILABLE",
			referencePriceRange: null,
			marketPriceBasis: null,
			dealTemperature: { status: "UNKNOWN", reasonCode: "MAPPING_MISSING" },
		});
		expect(parsed.marketPriceBasis).toBeNull();
		expect(parsed.quoteTotalRange?.lowerBoundAmount).toBe("15000");
	});

	it("accepts a market basis whose source data date is absent", () => {
		const parsed = rfqPricePreviewSchema.parse({
			...guidedPreview(),
			marketPriceBasis: {
				...guidedPreview().marketPriceBasis,
				sourceDataAsOf: null,
				freshnessStatus: "unavailable",
			},
		});
		expect(parsed.marketPriceBasis?.sourceDataAsOf).toBeNull();
	});

	it.each([
		{ marketPriceBasis: null },
		{
			marketPriceBasis: {
				...guidedPreview().marketPriceBasis,
				sourceDataAsOf: "",
			},
		},
		{
			marketPriceBasis: {
				...guidedPreview().marketPriceBasis,
				freshnessStatus: "unknown",
			},
		},
		{
			marketPriceBasis: {
				...guidedPreview().marketPriceBasis,
				supportingMarketCount: "many",
			},
		},
		{
			quoteTotalRange: {
				...guidedPreview().quoteTotalRange,
				lowerBoundAmount: "21000",
				upperBoundAmount: "15000",
			},
		},
		{
			quoteTotalRange: { ...guidedPreview().quoteTotalRange, quantity: "0" },
		},
		{
			quoteTotalRange: {
				...guidedPreview().quoteTotalRange,
				quantityUnit: "",
			},
		},
	])("rejects an inconsistent v6 price-basis projection", (override) => {
		expect(
			rfqPricePreviewSchema.safeParse({ ...guidedPreview(), ...override })
				.success,
		).toBe(false);
	});

	it("binds a confirmable basis to the persisted request unit", () => {
		const parsed = rfqPricePreviewSchema.parse(guidedPreview());
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
				...guidedPreview(),
				buyerRequestedRange: {
					...guidedPreview().buyerRequestedRange,
					lowerBoundAmount: amount,
				},
			}).success,
		).toBe(false);
	});

	it("rejects decimal amounts in buyer-entered no-reference mode", () => {
		expect(
			rfqPricePreviewSchema.safeParse({
				...guidedPreview(),
				pricingMode: "BUYER_ENTERED_NO_REFERENCE",
				referenceAvailability: "UNAVAILABLE",
				referencePriceRange: null,
				marketPriceBasis: null,
				buyerRequestedRange: {
					...guidedPreview().buyerRequestedRange,
					lowerBoundAmount: "30.25",
					basisQuantity: "",
					basisUnit: "",
				},
				dealTemperature: {
					status: "UNKNOWN",
					reasonCode: "MAPPING_MISSING",
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
				...guidedPreview(),
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
