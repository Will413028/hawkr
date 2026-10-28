import { describe, expect, it } from "vitest";
import { rfqPricePreviewSchema } from "./rfq-price-contract";

const preview = {
	pricePreviewId: "7506912040892497920",
	version: "1",
	qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
	buyerRequestedRange: {
		currency: "TWD",
		priceComponent: "GOODS_ONLY",
		lowerBoundAmount: "90",
		upperBoundAmount: "110",
		basisQuantity: "1",
		basisUnit: "KG",
	},
	payloadHash:
		"58faddc469333851aac865e1e9d1719d383a2edeeaefd405efc682160500ebf7",
	quoteTotalRange: null,
};

describe("rfqPricePreviewSchema — 買家區間", () => {
	it("完整的區間解析得出來", () => {
		const parsed = rfqPricePreviewSchema.safeParse(preview);
		expect(parsed.success, JSON.stringify(parsed.error?.issues)).toBe(true);
	});

	it("區間是 null 或空物件都不得放行", () => {
		expect(
			rfqPricePreviewSchema.safeParse({ ...preview, buyerRequestedRange: null })
				.success,
		).toBe(false);
		expect(
			rfqPricePreviewSchema.safeParse({
				...preview,
				buyerRequestedRange: {
					currency: "",
					priceComponent: "",
					lowerBoundAmount: "",
					upperBoundAmount: "",
					basisQuantity: "",
					basisUnit: "",
				},
			}).success,
		).toBe(false);
	});
});
