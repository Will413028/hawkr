import { describe, expect, it, vi } from "vitest";
import { apiClient } from "@/lib/api/client";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchCategoryPriceRollup,
	fetchCategoryResolution,
	fetchForecastCurve,
	fetchObservedPriceHistory,
	fetchSpatialSurface,
	isSpatialSurfaceUnavailable,
	SpatialSurfaceUnavailableError,
	selectionKey,
	spatialSelectionKey,
} from "./repo";

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: { authed: { get: vi.fn() } },
	};
});

describe("fetchCategoryResolution", () => {
	it("passes category_id as a query param", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				categoryId: "123",
				status: "resolved",
				groupCode: "G1",
				label: "包心白",
				mappingVersion: "pricebook-category-v1",
			},
		} as never);

		const result = await fetchCategoryResolution("123");

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/category-resolution",
			{ query: { category_id: "123" } },
		);
		expect(result.status).toBe("resolved");
		expect(result.groupCode).toBe("G1");
	});

	it("throws when data is missing rather than returning a guessed default", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
		} as never);

		await expect(fetchCategoryResolution("123")).rejects.toThrow();
	});
});

describe("fetchCategoryPriceRollup", () => {
	it("passes category_id as a query param and parses a ready snapshot", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				categoryId: "456",
				level: 2,
				status: "ready",
				latestPrice: "45.2",
				prevPrice: "42.1",
				points: [
					{ tradeDate: "2026-08-29", avgPrice: "44.0" },
					{ tradeDate: "2026-08-30", avgPrice: "45.2" },
				],
				sourceGroupCount: 6,
				computedAt: "2026-08-30T21:15:03Z",
			},
		} as never);

		const result = await fetchCategoryPriceRollup("456");

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/category-price-rollup",
			{ query: { category_id: "456" } },
		);
		expect(result.status).toBe("ready");
		expect(result.latestPrice).toBe(45.2);
		expect(result.prevPrice).toBe(42.1);
		expect(result.sourceGroupCount).toBe(6);
		expect(result.points).toEqual([
			{ tradeDate: "2026-08-29", avgPrice: 44.0 },
			{ tradeDate: "2026-08-30", avgPrice: 45.2 },
		]);
	});

	it("returns unavailable status with no price fields when the batch has no data for this category", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				categoryId: "456",
				level: 1,
				status: "unavailable",
				points: [],
				sourceGroupCount: 0,
				computedAt: "2026-08-30T21:15:03Z",
			},
		} as never);

		const result = await fetchCategoryPriceRollup("456");

		expect(result.status).toBe("unavailable");
		expect(result.latestPrice).toBeUndefined();
		expect(result.prevPrice).toBeUndefined();
		expect(result.points).toEqual([]);
	});

	it("throws when data is missing rather than returning a guessed default", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
		} as never);

		await expect(fetchCategoryPriceRollup("456")).rejects.toThrow();
	});

	it("treats a missing/malformed status as unavailable rather than fabricating ready", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {},
		} as never);

		const result = await fetchCategoryPriceRollup("456");

		expect(result.status).toBe("unavailable");
	});

	it("drops a point whose avgPrice is a malformed string rather than fabricating a number", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				categoryId: "456",
				level: 2,
				status: "ready",
				latestPrice: "45.2",
				points: [
					{ tradeDate: "2026-08-29", avgPrice: "not-a-number" },
					{ tradeDate: "2026-08-30", avgPrice: "45.2" },
				],
				sourceGroupCount: 6,
				computedAt: "2026-08-30T21:15:03Z",
			},
		} as never);

		const result = await fetchCategoryPriceRollup("456");

		expect(result.points).toEqual([
			{ tradeDate: "2026-08-29" },
			{ tradeDate: "2026-08-30", avgPrice: 45.2 },
		]);
	});

	it("parses forecastPoints alongside the existing price fields", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				categoryId: "456",
				level: 2,
				status: "ready",
				latestPrice: "45.2",
				points: [],
				sourceGroupCount: 6,
				computedAt: "2026-08-30T21:15:03Z",
				forecastPoints: [
					{
						horizonDays: 1,
						targetDate: "2026-09-01",
						status: "ready",
						p10: "40.5",
						p50: "51.3",
						p90: "62.6",
					},
				],
			},
		} as never);

		const result = await fetchCategoryPriceRollup("456");

		expect(result.forecastPoints).toEqual([
			{
				horizonDays: 1,
				targetDate: "2026-09-01",
				status: "ready",
				p10: 40.5,
				p50: 51.3,
				p90: 62.6,
			},
		]);
	});

	it("returns an empty forecastPoints array when the field is absent", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				categoryId: "456",
				level: 2,
				status: "ready",
				points: [],
				sourceGroupCount: 6,
				computedAt: "2026-08-30T21:15:03Z",
			},
		} as never);

		const result = await fetchCategoryPriceRollup("456");

		expect(result.forecastPoints).toEqual([]);
	});
});

describe("fetchObservedPriceHistory", () => {
	it("sends cropName for a cropName selection", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { observations: [] },
		} as never);

		await fetchObservedPriceHistory({ kind: "cropName", cropName: "小黃瓜" });

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/observed-price",
			{ query: { cropName: "小黃瓜", history: true } },
		);
	});

	it("sends cropCode + marketCode for a crop selection", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { observations: [] },
		} as never);

		await fetchObservedPriceHistory({
			kind: "crop",
			cropCode: "A1",
			marketCode: "109",
		});

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/observed-price",
			{ query: { cropCode: "A1", marketCode: "109", history: true } },
		);
	});

	it("omits marketCode for an all-market crop history", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { observations: [] },
		} as never);

		await fetchObservedPriceHistory({ kind: "crop", cropCode: "A1" });

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/observed-price",
			{ query: { cropCode: "A1", history: true } },
		);
	});

	it("sends groupCode for a group selection", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { observations: [] },
		} as never);

		await fetchObservedPriceHistory({ kind: "group", groupCode: "g1" });

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/observed-price",
			{ query: { groupCode: "g1", history: true } },
		);
	});

	it("sends groupCode + marketCode for a group selection with marketCode set", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { observations: [] },
		} as never);

		await fetchObservedPriceHistory({
			kind: "group",
			groupCode: "g1",
			marketCode: "109",
		});

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/observed-price",
			{ query: { groupCode: "g1", marketCode: "109", history: true } },
		);
	});
});

describe("fetchForecastCurve", () => {
	it("sends cropCode + marketCode for a crop selection and surfaces marketComparison", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				status: "ready",
				modelVersion: "v1",
				points: [],
				marketComparison: [
					{ marketCode: "110", marketName: "台北二", p50: 42 },
				],
			},
		} as never);

		const result = await fetchForecastCurve({
			kind: "crop",
			cropCode: "A1",
			marketCode: "109",
		});

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/forecast-curve",
			{ query: { cropCode: "A1", marketCode: "109" } },
		);
		expect(result.marketComparison).toEqual([
			{ marketCode: "110", marketName: "台北二", p50: 42 },
		]);
	});

	it("defaults marketComparison to an empty array when absent", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { status: "ready", modelVersion: "v1", points: [] },
		} as never);

		const result = await fetchForecastCurve({ kind: "group", groupCode: "g1" });

		expect(result.marketComparison).toEqual([]);
	});

	it("omits marketCode for an all-market crop forecast", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				status: "ready",
				resolvedCropCode: "A1",
				modelVersion: "v1",
				points: [],
				marketComparison: [],
			},
		} as never);

		await fetchForecastCurve({ kind: "crop", cropCode: "A1" });

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/forecast-curve",
			{ query: { cropCode: "A1" } },
		);
	});

	it("passes resolvedCropGroupCode through for a group forecast", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				status: "ready",
				resolvedCropGroupCode: "g1",
				modelVersion: "v1",
				points: [],
				marketComparison: [
					{ marketCode: "110", marketName: "台北二", p50: 42 },
				],
			},
		} as never);

		const result = await fetchForecastCurve({ kind: "group", groupCode: "g1" });

		// groupMarketComparison 的 staleness guard (InsightsClient) 靠這個欄位
		// 判斷「目前 forecast payload 真的對應目前選取的大類」；沒有這個欄位就
		// 會一直判定過期，跨市場比較永遠顯示空清單。
		expect(result.resolvedCropGroupCode).toBe("g1");
	});
});

describe("selectionKey", () => {
	it("isolates all-market and selected-market crop queries", () => {
		expect(selectionKey({ kind: "crop", cropCode: "A1" })).toBe(
			"crop:A1:market:@all",
		);
		expect(
			selectionKey({ kind: "crop", cropCode: "A1", marketCode: "109" }),
		).toBe("crop:A1:market:109");
		expect(
			selectionKey({ kind: "crop", cropCode: "A1", marketCode: "all" }),
		).not.toBe(selectionKey({ kind: "crop", cropCode: "A1" }));
	});

	it("isolates all-market and selected-market group queries", () => {
		expect(selectionKey({ kind: "group", groupCode: "g1" })).toBe(
			"group:g1:market:@all",
		);
		expect(
			selectionKey({ kind: "group", groupCode: "g1", marketCode: "109" }),
		).toBe("group:g1:market:109");
		expect(
			selectionKey({ kind: "group", groupCode: "g1", marketCode: "all" }),
		).not.toBe(selectionKey({ kind: "group", groupCode: "g1" }));
	});
});

describe("fetchSpatialSurface", () => {
	function marketPrice(marketCode: string, p50: number) {
		return {
			marketCode,
			marketName: `市場 ${marketCode}`,
			categoryCode: "N04",
			predictionStatus: "ready",
			marketStatus: "open",
			p10: null,
			p50,
			p90: null,
			recentVolumeKg: null,
			usedForSurface: true,
		};
	}

	function validSurface(overrides: Record<string, unknown> = {}) {
		return {
			status: "ready",
			displayMode: "contours_and_points",
			selectionKind: "crop",
			targetDate: "2026-08-07",
			horizonDays: 1,
			currency: "TWD",
			unit: "kg",
			scale: {
				scaleId: "adaptive-p50-test",
				policyVersion: "selection-p50-v1",
				breaks: [20, 30, 40],
			},
			boundary: {
				version: "boundary-v1",
				digest: "abc",
				bboxWgs84: [119, 21, 123, 26],
				geojson: {
					type: "FeatureCollection",
					features: [
						{
							type: "Feature",
							properties: { regionName: "臺灣" },
							geometry: {
								type: "Polygon",
								coordinates: [
									[
										[120, 22],
										[122, 22],
										[122, 25],
										[120, 25],
										[120, 22],
									],
								],
							},
						},
					],
				},
			},
			boundaryPriceGeojson: { type: "FeatureCollection", features: [] },
			marketPointsGeojson: {
				type: "FeatureCollection",
				features: [
					{
						type: "Feature",
						properties: {
							marketCode: "109",
							marketName: "台北一",
							p50: 30,
						},
						geometry: { type: "Point", coordinates: [121.5, 25.1] },
					},
				],
			},
			surfaceCellsGeojson: { type: "FeatureCollection", features: [] },
			contourLinesGeojson: {
				type: "FeatureCollection",
				features: [
					{
						type: "Feature",
						properties: { level_price_twd_per_kg: 30 },
						geometry: {
							type: "LineString",
							coordinates: [
								[120.5, 23],
								[121.5, 24],
							],
						},
					},
				],
			},
			marketPrices: [
				marketPrice("109", 30),
				marketPrice("220", 10),
				marketPrice("800", 50),
			],
			statistics: {
				p10: null,
				p50: 30,
				p90: null,
				minimumP50: 10,
				maximumP50: 50,
				pricedMarketCount: 3,
				closedMarketCount: 0,
				totalMarketCount: 3,
				surfaceCellCount: 0,
				contourSegmentCount: 1,
			},
			qualityReasonCodes: [],
			disclosures: [],
			supportedLandAreaRatio: null,
			excludedMarketCodes: [],
			lineage: {
				artifactId: "surface-1",
				modelVersion: "model-v1",
				datasetDigest: "dataset-abc",
				boundaryDigest: "abc",
			},
			...overrides,
		};
	}

	it("requests a national crop surface, forwards cancellation, and normalizes null gaps", async () => {
		const surface = validSurface();
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: surface,
		} as never);
		const controller = new AbortController();

		const result = await fetchSpatialSurface(
			{ kind: "crop", cropCode: "A1", marketCode: "109" },
			1,
			controller.signal,
		);

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/spatial-surface",
			{
				query: { cropCode: "A1", horizonDays: 1 },
				signal: controller.signal,
			},
		);
		expect(result.marketPrices[0]).toMatchObject({
			p10: undefined,
			p50: 30,
			p90: undefined,
			recentVolumeKg: undefined,
		});
		expect(result.statistics.p10).toBeUndefined();
		expect(result.supportedLandAreaRatio).toBeUndefined();
	});

	it("uses one national cache key when only the comparison market changes", () => {
		const identity = { accountId: "7", activeRole: "seller" as const };
		const first = queryKeys.pricebook.spatialSurface(
			identity,
			spatialSelectionKey({
				kind: "crop",
				cropCode: "A1",
				marketCode: "109",
			}),
			3,
		);
		const second = queryKeys.pricebook.spatialSurface(
			identity,
			spatialSelectionKey({
				kind: "crop",
				cropCode: "A1",
				marketCode: "800",
			}),
			3,
		);
		const allMarkets = queryKeys.pricebook.spatialSurface(
			identity,
			spatialSelectionKey({ kind: "crop", cropCode: "A1" }),
			3,
		);

		expect(first).toEqual(second);
		expect(first).toEqual(allMarkets);
		expect(first).toContain("crop:A1");
		expect(first).not.toContain("109");
		expect(first).not.toContain("800");
	});

	it("keeps a defensible degraded table response while requiring no map breaks", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: validSurface({
				status: "degraded",
				displayMode: "table_only",
				scale: { scaleId: "price-v1", policyVersion: "v1", breaks: [] },
				contourLinesGeojson: { type: "FeatureCollection", features: [] },
			}),
		} as never);

		const result = await fetchSpatialSurface(
			{ kind: "crop", cropCode: "A1", marketCode: "109" },
			1,
		);

		expect(result.status).toBe("degraded");
		expect(result.displayMode).toBe("table_only");
		expect(result.scale.breaks).toEqual([]);
	});

	it("accepts a flat response as one neutral band with no contours", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: validSurface({
				displayMode: "points_only",
				scale: {
					scaleId: "adaptive-flat",
					policyVersion: "selection-p50-v1-flat-fallback",
					breaks: [],
				},
				contourLinesGeojson: { type: "FeatureCollection", features: [] },
				marketPrices: [
					marketPrice("109", 30),
					marketPrice("220", 30),
					marketPrice("800", 30),
				],
				statistics: {
					p50: 30,
					minimumP50: 30,
					maximumP50: 30,
					pricedMarketCount: 3,
					closedMarketCount: 0,
					totalMarketCount: 3,
					surfaceCellCount: 0,
					contourSegmentCount: 0,
				},
			}),
		} as never);

		const result = await fetchSpatialSurface(
			{ kind: "crop", cropCode: "A1", marketCode: "109" },
			1,
		);

		expect(result.scale.breaks).toEqual([]);
		expect(result.contourLinesGeojson.features).toEqual([]);
	});

	it("accepts seven thresholds for the eight-colour palette boundary", async () => {
		const sevenBreaks = [10, 20, 30, 40, 50, 60, 70];
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: validSurface({
				scale: {
					scaleId: "price-v1",
					policyVersion: "v1",
					breaks: sevenBreaks,
				},
			}),
		} as never);

		const result = await fetchSpatialSurface(
			{ kind: "crop", cropCode: "A1", marketCode: "109" },
			1,
		);

		expect(result.scale.breaks).toEqual(sevenBreaks);
	});

	it("keeps each selection's adaptive scale isolated in its own response", async () => {
		const first = validSurface({
			scale: {
				scaleId: "adaptive-a1",
				policyVersion: "selection-p50-v1",
				breaks: [50, 65, 80],
			},
			statistics: {
				p50: 65,
				minimumP50: 40,
				maximumP50: 90,
				pricedMarketCount: 3,
				closedMarketCount: 0,
				totalMarketCount: 3,
				surfaceCellCount: 0,
				contourSegmentCount: 1,
			},
			marketPrices: [
				marketPrice("109", 65),
				marketPrice("220", 40),
				marketPrice("800", 90),
			],
			contourLinesGeojson: {
				type: "FeatureCollection",
				features: [
					{
						type: "Feature",
						properties: { levelPriceTwdPerKg: 65 },
						geometry: {
							type: "LineString",
							coordinates: [
								[120.5, 23],
								[121.5, 24],
							],
						},
					},
				],
			},
		});
		const second = validSurface({
			scale: {
				scaleId: "adaptive-b1",
				policyVersion: "selection-p50-v1",
				breaks: [120, 150, 190],
			},
			statistics: {
				p50: 150,
				minimumP50: 100,
				maximumP50: 210,
				pricedMarketCount: 3,
				closedMarketCount: 0,
				totalMarketCount: 3,
				surfaceCellCount: 0,
				contourSegmentCount: 1,
			},
			marketPrices: [
				marketPrice("109", 150),
				marketPrice("220", 100),
				marketPrice("800", 210),
			],
			contourLinesGeojson: {
				type: "FeatureCollection",
				features: [
					{
						type: "Feature",
						properties: { levelPriceTwdPerKg: 150 },
						geometry: {
							type: "LineString",
							coordinates: [
								[120.5, 23],
								[121.5, 24],
							],
						},
					},
				],
			},
		});
		vi.mocked(apiClient.authed.get)
			.mockResolvedValueOnce({ code: 0, data: first } as never)
			.mockResolvedValueOnce({ code: 0, data: second } as never);

		const a = await fetchSpatialSurface(
			{ kind: "crop", cropCode: "A1", marketCode: "109" },
			1,
		);
		const b = await fetchSpatialSurface(
			{ kind: "crop", cropCode: "B1", marketCode: "109" },
			1,
		);

		expect(a.scale.breaks).toEqual([50, 65, 80]);
		expect(a.statistics).toMatchObject({
			minimumP50: 40,
			p50: 65,
			maximumP50: 90,
		});
		expect(b.scale.breaks).toEqual([120, 150, 190]);
		expect(b.statistics).toMatchObject({
			minimumP50: 100,
			p50: 150,
			maximumP50: 210,
		});
	});

	it("fails closed when the BFF returns no spatial payload", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({ code: 0 } as never);

		await expect(
			fetchSpatialSurface({ kind: "group", groupCode: "N04" }, 3),
		).rejects.toThrow("空間行情服務未回傳資料");
	});

	it("fails closed when the BFF explicitly reports unavailable", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { status: "unavailable" },
		} as never);

		await expect(
			fetchSpatialSurface(
				{ kind: "crop", cropCode: "A1", marketCode: "109" },
				1,
			),
		).rejects.toThrow("空間行情目前不可用");
	});

	it("carries diagnostic detail on the unavailable error instead of discarding it", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: {
				status: "unavailable",
				targetDate: "2026-08-10",
				qualityReasonCodes: [
					"insufficient_spatial_coverage",
					"insufficient_spatial_points",
				],
				statistics: {
					closedMarketCount: 12,
					totalMarketCount: 15,
				},
			},
		} as never);

		let caught: unknown;
		try {
			await fetchSpatialSurface(
				{ kind: "crop", cropCode: "A1", marketCode: "109" },
				2,
			);
		} catch (error) {
			caught = error;
		}

		expect(caught).toBeInstanceOf(SpatialSurfaceUnavailableError);
		expect(isSpatialSurfaceUnavailable(caught)).toBe(true);
		const detail = caught as SpatialSurfaceUnavailableError;
		expect(detail.targetDate).toBe("2026-08-10");
		expect(detail.qualityReasonCodes).toEqual([
			"insufficient_spatial_coverage",
			"insufficient_spatial_points",
		]);
		expect(detail.closedMarketCount).toBe(12);
		expect(detail.totalMarketCount).toBe(15);
	});

	it("degrades diagnostic detail gracefully when the unavailable payload omits it", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { status: "unavailable" },
		} as never);

		let caught: unknown;
		try {
			await fetchSpatialSurface({ kind: "group", groupCode: "N04" }, 3);
		} catch (error) {
			caught = error;
		}

		expect(caught).toBeInstanceOf(SpatialSurfaceUnavailableError);
		const detail = caught as SpatialSurfaceUnavailableError;
		expect(detail.targetDate).toBeUndefined();
		expect(detail.qualityReasonCodes).toEqual([]);
		expect(detail.closedMarketCount).toBeUndefined();
		expect(detail.totalMarketCount).toBeUndefined();
	});

	it("does not treat other spatial-surface errors as expected", () => {
		expect(isSpatialSurfaceUnavailable(new Error("boom"))).toBe(false);
		expect(isSpatialSurfaceUnavailable(null)).toBe(false);
	});

	it.each([
		["unknown status", { status: "stale" }, "status"],
		["wrong currency", { currency: "USD" }, "currency"],
		["wrong unit", { unit: "box" }, "unit"],
		["wrong requested horizon", { horizonDays: 2 }, "horizonDays"],
		[
			"malformed feature collection",
			{ marketPointsGeojson: { type: "FeatureCollection", features: null } },
			"marketPointsGeojson",
		],
		[
			"unordered server scale",
			{ scale: { scaleId: "price-v1", policyVersion: "v1", breaks: [40, 20] } },
			"scale.breaks",
		],
		[
			"scale exceeding the eight-colour palette",
			{
				scale: {
					scaleId: "price-v1",
					policyVersion: "v1",
					breaks: [10, 20, 30, 40, 50, 60, 70, 80],
				},
			},
			"scale.breaks",
		],
		[
			"contour threshold from another scale",
			{
				contourLinesGeojson: {
					type: "FeatureCollection",
					features: [
						{
							type: "Feature",
							properties: { levelPriceTwdPerKg: 35 },
							geometry: {
								type: "LineString",
								coordinates: [
									[120.5, 23],
									[121.5, 24],
								],
							},
						},
					],
				},
			},
			"contourLinesGeojson",
		],
		[
			"missing P50 scale domain",
			{
				statistics: {
					p50: 30,
					pricedMarketCount: 1,
					closedMarketCount: 0,
					totalMarketCount: 1,
					surfaceCellCount: 0,
					contourSegmentCount: 1,
				},
			},
			"statistics",
		],
		[
			"P50 endpoints unrelated to market rows",
			{
				statistics: {
					p50: 30,
					minimumP50: 10,
					maximumP50: 60,
					pricedMarketCount: 3,
					closedMarketCount: 0,
					totalMarketCount: 3,
					surfaceCellCount: 0,
					contourSegmentCount: 1,
				},
			},
			"statistics",
		],
		[
			"adaptive threshold outside the P50 domain",
			{
				scale: {
					scaleId: "bad-domain",
					policyVersion: "selection-p50-v1",
					breaks: [5, 30, 40],
				},
			},
			"scale.breaks",
		],
		[
			"unsupported adaptive policy suffix",
			{
				scale: {
					scaleId: "future-policy",
					policyVersion: "selection-p50-v1-experimental",
					breaks: [20, 30, 40],
				},
			},
			"scale.policyVersion",
		],
		[
			"flat policy carrying legacy thresholds",
			{
				scale: {
					scaleId: "bad-flat",
					policyVersion: "selection-p50-v1-flat-fallback",
					breaks: [20, 30, 40],
				},
			},
			"scale.breaks",
		],
		[
			"boundary lineage mismatch",
			{
				lineage: {
					artifactId: "surface-1",
					modelVersion: "model-v1",
					datasetDigest: "dataset-abc",
					boundaryDigest: "different",
				},
			},
			"lineage.boundaryDigest",
		],
		[
			"missing lineage dataset digest",
			{
				lineage: {
					artifactId: "surface-1",
					modelVersion: "model-v1",
					boundaryDigest: "abc",
				},
			},
			"lineage.datasetDigest",
		],
		[
			"used market with null price",
			{
				marketPrices: [
					{
						marketCode: "109",
						marketName: "台北一",
						predictionStatus: "ready",
						marketStatus: "open",
						p50: null,
						usedForSurface: true,
					},
				],
			},
			"marketPrices[0].p50",
		],
		[
			"contradictory statistics",
			{
				statistics: {
					pricedMarketCount: 2,
					closedMarketCount: 0,
					totalMarketCount: 1,
					surfaceCellCount: 0,
					contourSegmentCount: 1,
				},
			},
			"statistics",
		],
	])("rejects %s", async (_case, override, expectedField) => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: validSurface(override),
		} as never);

		await expect(
			fetchSpatialSurface(
				{ kind: "crop", cropCode: "A1", marketCode: "109" },
				1,
			),
		).rejects.toThrow(expectedField);
	});
});
