import { describe, expect, it } from "vitest";
import { rfqPricePreviewSchema } from "./rfq-price-contract";

const commodityPreviewNoBuyerRange = {
	pricePreviewId: "7506912040892497920",
	version: "1",
	pricingMode: "PRICEBOOK_GUIDED",
	qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
	referenceAvailability: "AVAILABLE",
	referencePriceRange: {
		lowerBoundAmount: "",
		midpointAmount: "100",
		upperBoundAmount: "",
		basisQuantity: "1",
		basisUnit: "KG",
	},
	buyerRequestedRange: null,
	dealTemperature: { status: "UNKNOWN", reasonCode: "BUYER_RANGE_NOT_SET" },
	payloadHash:
		"58faddc469333851aac865e1e9d1719d383a2edeeaefd405efc682160500ebf7",
	marketPriceBasis: {
		scope: "NATIONAL",
		priceLayer: "live_animal",
		regionName: "",
		supportingMarketCount: "19",
		sourceDataAsOf: "2026-09-17",
		freshnessStatus: "",
	},
	quoteTotalRange: null,
};

describe("rfqPricePreviewSchema — 單點來源（毛豬／家禽／蛋品／糧價）", () => {
	it("買家略過預算時 buyerRequestedRange 是 null，仍然解析得出來", () => {
		const parsed = rfqPricePreviewSchema.safeParse(
			commodityPreviewNoBuyerRange,
		);
		expect(parsed.success, JSON.stringify(parsed.error?.issues)).toBe(true);
		if (parsed.success) {
			expect(parsed.data.buyerRequestedRange).toBeNull();
		}
	});

	it("freshnessStatus 空字串＝來源沒有新鮮度判斷，不是壞掉", () => {
		const parsed = rfqPricePreviewSchema.safeParse(
			commodityPreviewNoBuyerRange,
		);
		expect(parsed.success).toBe(true);
		if (parsed.success) {
			expect(parsed.data.marketPriceBasis?.freshnessStatus).toBe("");
		}
	});

	it("有填預算時仍然要求完整的區間，空物件不得放行", () => {
		const bad = {
			...commodityPreviewNoBuyerRange,
			buyerRequestedRange: {
				currency: "",
				priceComponent: "",
				lowerBoundAmount: "",
				upperBoundAmount: "",
				basisQuantity: "",
				basisUnit: "",
			},
		};
		expect(rfqPricePreviewSchema.safeParse(bad).success).toBe(false);
	});
});
