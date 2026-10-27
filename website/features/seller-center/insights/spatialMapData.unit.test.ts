import { describe, expect, it } from "vitest";
import type { GeoJsonFeature } from "@/lib/pricebook/repo";
import {
	BOUNDARY_INTERACTION_ID,
	boundaryInteractionCollection,
	boundaryPriceLabel,
	boundaryRegionName,
	featureCollectionBounds,
	featurePriceDomain,
	p50ScaleDomain,
	pointPriceLabel,
	pricedFeatureCollection,
	priceScalePresentation,
	spatialPropertyNumber,
	thinPriceLabels,
} from "./spatialMapData";

function regionFeature(properties: Record<string, unknown>): GeoJsonFeature {
	return {
		type: "Feature",
		properties,
		geometry: {
			type: "Polygon",
			coordinates: [
				[
					[120, 23],
					[121, 23],
					[121, 24],
					[120, 24],
					[120, 23],
				],
			],
		},
	};
}

describe("spatial map data", () => {
	it("never coerces null, empty strings or booleans into a zero price", () => {
		expect(spatialPropertyNumber({ p50: null }, "p50")).toBeUndefined();
		expect(spatialPropertyNumber({ p50: "" }, "p50")).toBeUndefined();
		expect(spatialPropertyNumber({ p50: "  " }, "p50")).toBeUndefined();
		expect(spatialPropertyNumber({ p50: false }, "p50")).toBeUndefined();
		expect(spatialPropertyNumber({ p50: 0 }, "p50")).toBe(0);
		expect(spatialPropertyNumber({ p50: "72.5" }, "p50")).toBe(72.5);
	});

	it("filters unpriced map features instead of painting them as zero", () => {
		const missing = regionFeature({ estimatedP50TwdPerKg: null });
		const priced = regionFeature({ estimatedP50TwdPerKg: 68 });
		expect(
			pricedFeatureCollection(
				{ type: "FeatureCollection", features: [missing, priced] },
				"estimatedP50TwdPerKg",
			).features,
		).toEqual([priced]);
	});

	it("retains unpriced boundary metadata and original geometry for interaction", () => {
		const missing = regionFeature({
			regionName: "嘉義縣",
			estimatedP50TwdPerKg: null,
			bandLowerTwdPerKg: 32,
			bandUpperTwdPerKg: 48,
			supportingMarketCount: 0,
			status: "insufficient_support",
		});
		missing.id = "backend-region";
		const [interactive] = boundaryInteractionCollection(
			{ type: "FeatureCollection", features: [missing] },
			"metadata",
		).features;
		expect(interactive.id).toBe("backend-region");
		expect(interactive.geometry).toBe(missing.geometry);
		expect(interactive.properties).toMatchObject(missing.properties ?? {});
		expect(interactive.properties?.estimatedP50TwdPerKg).toBeNull();
		expect(interactive.properties?.[BOUNDARY_INTERACTION_ID]).toBe(
			"metadata:0",
		);
		expect(missing.properties).not.toHaveProperty(BOUNDARY_INTERACTION_ID);
		expect(boundaryPriceLabel(interactive, 0)).toBeNull();
	});

	it("selects duplicate region names and overlapping sources independently", () => {
		const collection = {
			type: "FeatureCollection" as const,
			features: [
				regionFeature({ regionName: "中正區", estimatedP50TwdPerKg: 37 }),
				regionFeature({ regionName: "中正區", estimatedP50TwdPerKg: 45 }),
			],
		};
		const summaries = boundaryInteractionCollection(collection, "metadata");
		const raw = boundaryInteractionCollection(collection, "boundary");
		expect(
			new Set(
				[...summaries.features, ...raw.features].map(
					(feature) => feature.properties?.[BOUNDARY_INTERACTION_ID],
				),
			).size,
		).toBe(4);
		expect(boundaryPriceLabel(summaries.features[1], 1)?.boundaryId).toBe(
			"metadata:1",
		);
	});

	it("reads backend region names or combines raw county and town names", () => {
		expect(
			boundaryRegionName({ COUNTYNAME: "花蓮縣", TOWNNAME: "壽豐鄉" }),
		).toBe("花蓮縣壽豐鄉");
		expect(
			boundaryRegionName({ county_name: "嘉義縣", town_name: "中埔鄉" }),
		).toBe("嘉義縣中埔鄉");
		expect(
			boundaryRegionName({
				regionName: "後端行政區",
				COUNTYNAME: "花蓮縣",
				TOWNNAME: "壽豐鄉",
			}),
		).toBe("後端行政區");
		expect(
			boundaryRegionName({ countyName: "花蓮縣壽豐鄉", townName: "壽豐鄉" }),
		).toBe("花蓮縣壽豐鄉");
		expect(boundaryRegionName({ TOWNCODE: "10015060" })).toBe("10015060");
		expect(boundaryRegionName(null)).toBe("行政區");
	});

	it("keeps raw boundaries with no metadata clickable without inventing a price", () => {
		const feature = { ...regionFeature({}), properties: null };
		const [interactive] = boundaryInteractionCollection(
			{ type: "FeatureCollection", features: [feature] },
			"boundary",
		).features;
		expect(interactive.properties).toEqual({
			[BOUNDARY_INTERACTION_ID]: "boundary:0",
		});
		expect(boundaryPriceLabel(interactive, 0)).toBeNull();
		expect(feature.properties).toBeNull();
	});

	it("derives an adaptive height domain from the current priced surface", () => {
		const low = regionFeature({ priceTwdPerKg: "56.5" });
		const missing = regionFeature({ priceTwdPerKg: null });
		const high = regionFeature({ price_twd_per_kg: 81.25 });
		expect(
			featurePriceDomain(
				{ type: "FeatureCollection", features: [high, missing, low] },
				"priceTwdPerKg",
				"price_twd_per_kg",
			),
		).toEqual({ minimum: 56.5, maximum: 81.25 });
	});

	it("does not invent vertical variation when a surface has one price", () => {
		expect(
			featurePriceDomain(
				{
					type: "FeatureCollection",
					features: [
						regionFeature({ priceTwdPerKg: 68 }),
						regionFeature({ priceTwdPerKg: "68" }),
					],
				},
				"priceTwdPerKg",
			),
		).toBeNull();
		expect(
			featurePriceDomain(
				{
					type: "FeatureCollection",
					features: [regionFeature({ priceTwdPerKg: null })],
				},
				"priceTwdPerKg",
			),
		).toBeNull();
	});

	it("accepts an ordered adaptive P50 scale domain, including an honest flat field", () => {
		expect(p50ScaleDomain(37.9, 67.5, 72.5)).toEqual({
			minimum: 37.9,
			centre: 67.5,
			maximum: 72.5,
			isFlat: false,
		});
		expect(p50ScaleDomain(68, 68, 68)).toEqual({
			minimum: 68,
			centre: 68,
			maximum: 68,
			isFlat: true,
		});
	});

	it("rejects incomplete or contradictory P50 scale summaries", () => {
		expect(p50ScaleDomain(undefined, 60, 80)).toBeNull();
		expect(p50ScaleDomain(80, 60, 90)).toBeNull();
		expect(p50ScaleDomain(30, Number.NaN, 90)).toBeNull();
	});

	it("shares exact adaptive and quantised-flat scale eligibility", () => {
		expect(
			priceScalePresentation(
				{
					scaleId: "adaptive-a1",
					policyVersion: "selection-p50-v1",
					breaks: [47.7667, 57.6333, 67.5, 69.1667, 70.8333],
				},
				{
					p50: 67.5,
					minimumP50: 37.9,
					maximumP50: 72.5,
					pricedMarketCount: 3,
					closedMarketCount: 0,
					totalMarketCount: 3,
					surfaceCellCount: 10,
					contourSegmentCount: 5,
				},
			),
		).toMatchObject({ isFlat: false });
		expect(
			priceScalePresentation(
				{
					scaleId: "adaptive-flat",
					policyVersion: "selection-p50-v1-flat-fallback",
					breaks: [],
				},
				{
					p50: 68,
					minimumP50: 68,
					maximumP50: 68.0002,
					pricedMarketCount: 2,
					closedMarketCount: 0,
					totalMarketCount: 2,
					surfaceCellCount: 10,
					contourSegmentCount: 0,
				},
			),
		).toMatchObject({ isFlat: true });
	});

	it("keeps legacy fixed scales during rollout and rejects bad adaptive domains", () => {
		const statistics = {
			p50: 60,
			minimumP50: 40,
			maximumP50: 80,
			pricedMarketCount: 3,
			closedMarketCount: 0,
			totalMarketCount: 3,
			surfaceCellCount: 10,
			contourSegmentCount: 2,
		};
		expect(
			priceScalePresentation(
				{ scaleId: "legacy", policyVersion: "v1", breaks: [50, 70] },
				statistics,
			),
		).toMatchObject({ kind: "fixed", isFlat: false });
		expect(
			priceScalePresentation(
				{
					scaleId: "bad-domain",
					policyVersion: "selection-p50-v1",
					breaks: [30, 70],
				},
				statistics,
			),
		).toBeNull();
	});

	it("frames rendered mainland data instead of unrelated boundary outliers", () => {
		const mainland = regionFeature({ priceTwdPerKg: 68 });
		const market: GeoJsonFeature = {
			type: "Feature",
			properties: { p50: 70 },
			geometry: { type: "Point", coordinates: [121.8, 25.1] },
		};
		expect(
			featureCollectionBounds(
				{ type: "FeatureCollection", features: [mainland] },
				{ type: "FeatureCollection", features: [market] },
			),
		).toEqual([120, 23, 121.8, 25.1]);
	});

	it("derives a boundary label only from an explicit region price", () => {
		expect(
			boundaryPriceLabel(
				regionFeature({ regionName: "測試縣", estimatedP50TwdPerKg: 68 }),
				0,
			),
		).toMatchObject({ name: "測試縣", price: 68, longitude: 120.5 });
		expect(
			boundaryPriceLabel(
				regionFeature({ regionName: "缺值縣", estimatedP50TwdPerKg: null }),
				0,
			),
		).toBeNull();
	});

	it("uses the market code for selection even when GeoJSON feature.id differs", () => {
		const point: GeoJsonFeature = {
			type: "Feature",
			id: 1,
			properties: { marketCode: "109", marketName: "臺北一", p50: 68 },
			geometry: { type: "Point", coordinates: [121.5, 25.04] },
		};
		expect(pointPriceLabel(point, 0)?.id).toBe("109");
	});

	it("drops colliding labels while retaining the ordered priority", () => {
		const labels = [
			{ id: "selected", longitude: 121, latitude: 24, price: 70 },
			{ id: "overlap", longitude: 121.05, latitude: 24.02, price: 72 },
			{ id: "south", longitude: 120.4, latitude: 23, price: 65 },
		];
		expect(
			thinPriceLabels(labels, {
				limit: 3,
				longitudeDistance: 0.2,
				latitudeDistance: 0.1,
			}).map((label) => label.id),
		).toEqual(["selected", "south"]);
	});
});
