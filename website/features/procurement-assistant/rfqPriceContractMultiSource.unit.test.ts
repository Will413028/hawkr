import { describe, expect, it } from "vitest";
import {
	rfqMarketPriceBasisSchema,
	rfqPricePreviewSchema,
} from "./rfq-price-contract";

// 多來源序列的契約。

const BASE_PREVIEW = {
	pricePreviewId: "1",
	version: "1",
	pricingMode: "PRICEBOOK_GUIDED",
	referenceAvailability: "AVAILABLE",
	buyerRequestedRange: {
		currency: "TWD",
		priceComponent: "GOODS_ONLY",
		lowerBoundAmount: "90",
		upperBoundAmount: "120",
		basisQuantity: "1",
		basisUnit: "KG",
	},
	dealTemperature: {
		status: "AVAILABLE",
		reasonCode: "",
		probabilityLowerBps: "10000",
		probabilityUpperBps: "10000",
		band: "HOT",
		outcomeDefinition: "COVERAGE",
		outcomeDefinitionVersion: "v1",
		policyVersion: "v1",
		bandVersion: "v1",
	},
	payloadHash: "a".repeat(64),
	marketPriceBasis: {
		scope: "NATIONAL",
		priceLayer: "live_animal",
		regionName: "",
		supportingMarketCount: "19",
		sourceDataAsOf: "2026-09-12",
		freshnessStatus: "fresh",
	},
};

describe("referencePriceRange 的可選上下界", () => {
	// 🔴 source_passthrough 序列只公布一個成交均價。上下界為空字串是合法的，
	// 不得被 schema 判成契約違反——否則整份 preview 解析失敗，買家什麼都看不到。
	it("接受只有中位數、上下界為空字串", () => {
		const got = rfqPricePreviewSchema.safeParse({
			...BASE_PREVIEW,
			referencePriceRange: {
				lowerBoundAmount: "",
				midpointAmount: "106",
				upperBoundAmount: "",
			},
		});
		expect(got.success).toBe(true);
	});

	it("中位數仍是必填——它是這條序列唯一確定有的數字", () => {
		const got = rfqPricePreviewSchema.safeParse({
			...BASE_PREVIEW,
			referencePriceRange: {
				lowerBoundAmount: "",
				midpointAmount: "",
				upperBoundAmount: "",
			},
		});
		expect(got.success).toBe(false);
	});

	it("既有的三分位形狀仍然合法", () => {
		const got = rfqPricePreviewSchema.safeParse({
			...BASE_PREVIEW,
			referencePriceRange: {
				lowerBoundAmount: "30",
				midpointAmount: "36",
				upperBoundAmount: "42",
			},
		});
		expect(got.success).toBe(true);
	});
});

describe("marketPriceBasis 的 scope 與 priceLayer", () => {
	it("全國行情：scope=NATIONAL、regionName 為空、帶價格層級", () => {
		const got = rfqMarketPriceBasisSchema.parse({
			scope: "NATIONAL",
			priceLayer: "live_animal",
			regionName: "",
			supportingMarketCount: "19",
			sourceDataAsOf: "2026-09-12",
			freshnessStatus: "fresh",
		});
		expect(got.scope).toBe("NATIONAL");
		expect(got.priceLayer).toBe("live_animal");
	});

	// 舊 preview 沒有這兩個欄位，必須仍然解析得過並退回空字串，
	// 否則既有的詢價草稿一律讀不回來。
	it("舊 preview 缺 scope／priceLayer 時退回空字串", () => {
		const got = rfqMarketPriceBasisSchema.parse({
			regionName: "桃園市",
			supportingMarketCount: "3",
			sourceDataAsOf: "2026-09-12",
			freshnessStatus: "fresh",
		});
		expect(got.scope).toBe("");
		expect(got.priceLayer).toBe("");
		expect(got.regionName).toBe("桃園市");
	});
});
