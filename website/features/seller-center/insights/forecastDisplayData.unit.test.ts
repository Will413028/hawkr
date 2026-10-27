import { describe, expect, it } from "vitest";
import { resolveForecastDisplayData } from "./forecastDisplayData";

const READY_POINT = {
	targetDate: "2026-08-07",
	horizonDays: 1,
	status: "ready",
	p10: 58,
	p50: 65,
	p90: 73,
};

const MARKET_COMPARISON = [
	{ marketCode: "104", marketName: "台北二", p50: 66, isApproximate: false },
];

function payload(overrides: Record<string, unknown> = {}) {
	return {
		status: "ready",
		modelVersion: "forecast-v1",
		points: [READY_POINT],
		marketComparison: MARKET_COMPARISON,
		...overrides,
	};
}

describe("resolveForecastDisplayData", () => {
	it("preserves unavailable comparison market identity and status without inventing a price", () => {
		const comparison = {
			marketCode: "400",
			marketName: "臺中市場",
			status: "unavailable",
			p50: null,
			isApproximate: false,
		};
		const result = resolveForecastDisplayData(
			payload({ marketComparison: [...MARKET_COMPARISON, comparison] }),
		);

		expect(result.marketRoster[1]).toMatchObject({
			marketCode: "400",
			marketName: "臺中市場",
			status: "unavailable",
		});
		expect(result.marketComparison[1]?.p50).toBeUndefined();
		expect(result.marketComparison[0]?.p50).toBe(66);
	});

	it.each([
		"ready",
		"partial",
	])("allows a non-empty %s payload with a finite price-ready P50", (status) => {
		const result = resolveForecastDisplayData(payload({ status }));

		expect(result.isPriceReady).toBe(true);
		expect(result.issue).toBeNull();
		expect(result.points).toEqual([READY_POINT]);
		expect(result.marketComparison).toEqual(MARKET_COMPARISON);
		expect(result.marketRoster).toEqual(MARKET_COMPARISON);
	});

	it("fails an empty ready point array closed", () => {
		const result = resolveForecastDisplayData(payload({ points: [] }));

		expect(result.isPriceReady).toBe(false);
		expect(result.issue).toBe("no_price_ready_point");
		expect(result.points).toEqual([]);
		expect(result.marketComparison).toEqual([]);
	});

	it("fails a ready payload with only null prices closed", () => {
		const result = resolveForecastDisplayData(
			payload({
				points: [
					{
						...READY_POINT,
						p10: null,
						p50: null,
						p90: null,
					},
				],
			}),
		);

		expect(result.isPriceReady).toBe(false);
		expect(result.issue).toBe("no_price_ready_point");
		expect(result.points).toEqual([]);
		expect(result.marketComparison).toEqual([]);
	});

	it("fails a malformed point payload closed", () => {
		const result = resolveForecastDisplayData(
			payload({ points: [{ ...READY_POINT, p50: "65" }] }),
		);

		expect(result.isPriceReady).toBe(false);
		expect(result.issue).toBe("invalid_points");
		expect(result.points).toEqual([]);
		expect(result.marketComparison).toEqual([]);
	});

	it("accepts zero and equal visible quantiles", () => {
		const result = resolveForecastDisplayData(
			payload({
				points: [{ ...READY_POINT, p10: 0, p50: 0, p90: 0 }],
			}),
		);

		expect(result.isPriceReady).toBe(true);
		expect(result.issue).toBeNull();
		expect(result.points[0]).toMatchObject({ p10: 0, p50: 0, p90: 0 });
	});

	it("fails a negative visible quantile closed", () => {
		const result = resolveForecastDisplayData(
			payload({ points: [{ ...READY_POINT, p10: -1 }] }),
		);

		expect(result.isPriceReady).toBe(false);
		expect(result.issue).toBe("invalid_points");
		expect(result.points).toEqual([]);
		expect(result.marketComparison).toEqual([]);
	});

	it.each([
		{ p10: 11, p50: 10, p90: 20 },
		{ p10: 10, p50: 21, p90: 20 },
		{ p10: 20, p50: undefined, p90: 10 },
	])("fails inverted visible quantiles closed: %o", (prices) => {
		const result = resolveForecastDisplayData(
			payload({ points: [{ ...READY_POINT, ...prices }] }),
		);

		expect(result.isPriceReady).toBe(false);
		expect(result.issue).toBe("invalid_points");
		expect(result.points).toEqual([]);
	});

	it("keeps a partial point with only a finite P50", () => {
		const result = resolveForecastDisplayData(
			payload({
				status: "partial",
				points: [
					{
						...READY_POINT,
						status: "partial",
						p10: null,
						p90: null,
					},
				],
			}),
		);

		expect(result.isPriceReady).toBe(true);
		expect(result.points[0]).toMatchObject({
			status: "partial",
			p10: undefined,
			p50: 65,
			p90: undefined,
		});
	});

	it.each([
		"stale",
		"unavailable",
	])("never exposes prices or market comparison for top-level %s", (status) => {
		const result = resolveForecastDisplayData(payload({ status }));

		expect(result.isPriceReady).toBe(false);
		expect(result.points).toEqual([]);
		expect(result.marketComparison).toEqual([]);
		expect(result.marketRoster).toEqual(MARKET_COMPARISON);
	});

	it("removes prices attached to a non-price-ready day", () => {
		const result = resolveForecastDisplayData(
			payload({
				points: [
					READY_POINT,
					{
						targetDate: "2026-08-08",
						horizonDays: 2,
						status: "market_closed",
						p10: 990,
						p50: 999,
						p90: 1_010,
					},
				],
			}),
		);

		expect(result.isPriceReady).toBe(true);
		expect(result.points[1]).toMatchObject({
			status: "market_closed",
			p10: undefined,
			p50: undefined,
			p90: undefined,
		});
	});

	it("withholds malformed market comparison without hiding valid forecast points", () => {
		const result = resolveForecastDisplayData(
			payload({
				marketComparison: [
					{
						marketCode: "104",
						marketName: "台北二",
						p50: "66",
						isApproximate: false,
					},
				],
			}),
		);

		expect(result.isPriceReady).toBe(true);
		expect(result.points).toHaveLength(1);
		expect(result.marketComparison).toEqual([]);
		expect(result.marketRoster).toEqual([]);
	});

	it("filters out a market comparison entry missing isApproximate", () => {
		const result = resolveForecastDisplayData(
			payload({
				marketComparison: [
					{ marketCode: "104", marketName: "台北二", p50: 66 },
				],
			}),
		);

		expect(result.isPriceReady).toBe(true);
		expect(result.points).toHaveLength(1);
		expect(result.marketComparison).toEqual([]);
		expect(result.marketRoster).toEqual([]);
	});
});

// 序列能力：由後端宣告，前端只照做。
describe("resolveForecastDisplayData 的序列能力", () => {
	const base = {
		status: "ready",
		points: [
			{
				targetDate: "2026-09-13",
				horizonDays: 1,
				status: "ready",
				p50: 106,
				derivation: "source_passthrough",
			},
		],
	};

	it("照搬後端宣告的能力與 reason code", () => {
		const got = resolveForecastDisplayData({
			...base,
			unit: "kg",
			priceLayer: "live_animal",
			capabilities: {
				marketComparison: false,
				spatial: false,
				reasonCodes: ["SINGLE_SERIES_ONLY", "NO_APPROVED_GEOGRAPHY"],
			},
		});
		expect(got.capabilities).toEqual({
			marketComparison: false,
			spatial: false,
			reasonCodes: ["SINGLE_SERIES_ONLY", "NO_APPROVED_GEOGRAPHY"],
		});
		expect(got.unit).toBe("kg");
		expect(got.priceLayer).toBe("live_animal");
		expect(got.derivation).toBe("source_passthrough");
	});

	// 「沒宣告」與「宣告為 false」是不同的意思：前者維持既有行為（全部顯示），
	// 後者才關掉分頁。舊快取沒有這個欄位，不該因此少掉分頁。
	it("完全沒有 capabilities 時回 undefined，讓呼叫端維持既有行為", () => {
		expect(resolveForecastDisplayData(base).capabilities).toBeUndefined();
	});

	// 個別布林缺席時往保守方向倒：顯示一個其實不可用的分頁，使用者點進去會撞
	// 到空面板或錯誤；少顯示一個只是少一個入口。
	it("布林欄位缺席視為 false，不視為 true", () => {
		const got = resolveForecastDisplayData({ ...base, capabilities: {} });
		expect(got.capabilities).toEqual({
			marketComparison: false,
			spatial: false,
			reasonCodes: [],
		});
	});
});
