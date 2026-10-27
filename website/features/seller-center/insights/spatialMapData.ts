import type {
	GeoJsonFeature,
	GeoJsonFeatureCollection,
	SpatialPriceScale,
	SpatialSurfaceStatistics,
} from "@/lib/pricebook/repo";
import {
	SPATIAL_ADAPTIVE_P50_FLAT_POLICY,
	SPATIAL_ADAPTIVE_P50_POLICY,
} from "@/lib/pricebook/repo";

export interface PriceMapLabel {
	id: string;
	longitude: number;
	latitude: number;
	price: number;
	name?: string;
}

export interface RegionPriceLabel extends PriceMapLabel {
	boundaryId?: string;
	lower?: number;
	upper?: number;
	supportingMarkets?: number;
	status?: string;
	area: number;
}

export interface PriceDomain {
	minimum: number;
	maximum: number;
}

export interface P50ScaleDomain extends PriceDomain {
	centre: number;
	isFlat: boolean;
}

export interface PriceScalePresentation {
	kind: "adaptive" | "flat" | "fixed";
	domain: P50ScaleDomain | null;
	isFlat: boolean;
}

export type FeatureBounds = [number, number, number, number];

/** Reads a numeric GeoJSON property without coercing null, booleans or an
 * empty string to zero. A zero is returned only when zero was explicitly
 * supplied by the spatial artifact. */
export function spatialPropertyNumber(
	properties: Record<string, unknown> | null | undefined,
	...keys: string[]
): number | undefined {
	for (const key of keys) {
		const value = properties?.[key];
		if (typeof value === "number" && Number.isFinite(value)) return value;
		if (typeof value !== "string" || value.trim() === "") continue;
		const parsed = Number(value);
		if (Number.isFinite(parsed)) return parsed;
	}
	return undefined;
}

export function spatialPropertyString(
	properties: Record<string, unknown> | null | undefined,
	...keys: string[]
): string | undefined {
	for (const key of keys) {
		const value = properties?.[key];
		if (typeof value === "string" && value.trim()) return value.trim();
	}
	return undefined;
}

export const BOUNDARY_INTERACTION_ID = "__hawkrBoundaryId";

/** Local selection IDs distinguish overlapping sources and duplicate names.
 * Keep the backend geometry, feature IDs and all metadata, including null prices. */
export function boundaryInteractionCollection(
	collection: GeoJsonFeatureCollection,
	source: "metadata" | "boundary",
): GeoJsonFeatureCollection {
	return {
		type: "FeatureCollection",
		features: collection.features.map((feature, index) => ({
			...feature,
			properties: {
				...feature.properties,
				[BOUNDARY_INTERACTION_ID]: `${source}:${index}`,
			},
		})),
	};
}

export function boundaryRegionName(
	properties: Record<string, unknown> | null | undefined,
): string {
	const region = spatialPropertyString(properties, "regionName", "region_name");
	if (region) return region;
	const county = spatialPropertyString(
		properties,
		"countyName",
		"county_name",
		"COUNTYNAME",
	);
	const town = spatialPropertyString(
		properties,
		"townName",
		"town_name",
		"TOWNNAME",
	);
	if (county && town)
		return county.endsWith(town) ? county : `${county}${town}`;
	return (
		county ??
		town ??
		spatialPropertyString(
			properties,
			"regionCode",
			"region_code",
			"town_code",
			"TOWNCODE",
			"county_code",
			"COUNTYCODE",
		) ??
		"行政區"
	);
}

export function pricedFeatureCollection(
	collection: GeoJsonFeatureCollection,
	...priceKeys: string[]
): GeoJsonFeatureCollection {
	return {
		type: "FeatureCollection",
		features: collection.features.filter(
			(feature) =>
				spatialPropertyNumber(feature.properties, ...priceKeys) != null,
		),
	};
}

/**
 * Derives the vertical domain from the currently rendered selection only.
 * This stays separate from the backend-owned colour/contour thresholds: a
 * market excluded from interpolation may still belong to the all-market P50
 * summary, while extrusion height should use only the visible surface.
 */
export function featurePriceDomain(
	collection: GeoJsonFeatureCollection,
	...priceKeys: string[]
): PriceDomain | null {
	let minimum = Number.POSITIVE_INFINITY;
	let maximum = Number.NEGATIVE_INFINITY;
	for (const feature of collection.features) {
		const price = spatialPropertyNumber(feature.properties, ...priceKeys);
		if (price == null) continue;
		minimum = Math.min(minimum, price);
		maximum = Math.max(maximum, price);
	}
	return Number.isFinite(minimum) &&
		Number.isFinite(maximum) &&
		minimum < maximum
		? { minimum, maximum }
		: null;
}

/** Validates the server-owned lower/P50/upper summary used to label the
 * selection-adaptive colour scale. P50 is a weighted median, not an
 * arithmetic mean. */
export function p50ScaleDomain(
	minimum: number | undefined,
	centre: number | undefined,
	maximum: number | undefined,
): P50ScaleDomain | null {
	if (
		minimum == null ||
		centre == null ||
		maximum == null ||
		!Number.isFinite(minimum) ||
		!Number.isFinite(centre) ||
		!Number.isFinite(maximum) ||
		minimum < 0 ||
		minimum > centre ||
		centre > maximum
	) {
		return null;
	}
	return {
		minimum,
		centre,
		maximum,
		isFlat: minimum === maximum,
	};
}

/** One eligibility rule shared by the panel controls and MapLibre layers.
 * Exact adaptive policies use interior thresholds; a quantised near-flat
 * response intentionally presents the reviewed fallback as one neutral band. */
export function priceScalePresentation(
	scale: SpatialPriceScale,
	statistics: SpatialSurfaceStatistics,
): PriceScalePresentation | null {
	const domain = p50ScaleDomain(
		statistics.minimumP50,
		statistics.p50,
		statistics.maximumP50,
	);
	if (scale.policyVersion === SPATIAL_ADAPTIVE_P50_FLAT_POLICY) {
		return scale.breaks.length === 0 && domain
			? { kind: "flat", domain, isFlat: true }
			: null;
	}
	if (scale.breaks.length < 2) return null;
	if (
		scale.breaks.some(
			(priceBreak, index) =>
				!Number.isFinite(priceBreak) ||
				(index > 0 && priceBreak <= scale.breaks[index - 1]),
		)
	) {
		return null;
	}
	if (scale.policyVersion === SPATIAL_ADAPTIVE_P50_POLICY) {
		if (
			!domain ||
			domain.isFlat ||
			scale.breaks.some(
				(priceBreak) =>
					priceBreak <= domain.minimum || priceBreak >= domain.maximum,
			)
		) {
			return null;
		}
		return { kind: "adaptive", domain, isFlat: false };
	}
	if (scale.policyVersion.startsWith(`${SPATIAL_ADAPTIVE_P50_POLICY}-`)) {
		return null;
	}
	// Compatibility path for the reviewed fixed policies already in service.
	// This lets the website roll out first; once the backend starts returning
	// selection-p50-v1 the same map switches to adaptive labels and thresholds.
	return { kind: "fixed", domain, isFlat: false };
}

/** Finds the viewport extent of the rendered data instead of framing the full
 * governance boundary (whose offshore islands leave the main island shifted
 * and visually small). */
export function featureCollectionBounds(
	...collections: GeoJsonFeatureCollection[]
): FeatureBounds | null {
	let minimumLongitude = Number.POSITIVE_INFINITY;
	let minimumLatitude = Number.POSITIVE_INFINITY;
	let maximumLongitude = Number.NEGATIVE_INFINITY;
	let maximumLatitude = Number.NEGATIVE_INFINITY;

	const visit = (coordinates: unknown): void => {
		if (!Array.isArray(coordinates)) return;
		if (
			coordinates.length >= 2 &&
			typeof coordinates[0] === "number" &&
			typeof coordinates[1] === "number" &&
			Number.isFinite(coordinates[0]) &&
			Number.isFinite(coordinates[1])
		) {
			minimumLongitude = Math.min(minimumLongitude, coordinates[0]);
			minimumLatitude = Math.min(minimumLatitude, coordinates[1]);
			maximumLongitude = Math.max(maximumLongitude, coordinates[0]);
			maximumLatitude = Math.max(maximumLatitude, coordinates[1]);
			return;
		}
		coordinates.forEach((coordinate) => {
			visit(coordinate);
		});
	};

	collections.forEach((collection) => {
		collection.features.forEach((feature) => {
			visit(feature.geometry.coordinates);
		});
	});
	return Number.isFinite(minimumLongitude) &&
		Number.isFinite(minimumLatitude) &&
		Number.isFinite(maximumLongitude) &&
		Number.isFinite(maximumLatitude) &&
		minimumLongitude < maximumLongitude &&
		minimumLatitude < maximumLatitude
		? [minimumLongitude, minimumLatitude, maximumLongitude, maximumLatitude]
		: null;
}

export function pointPriceLabel(
	feature: GeoJsonFeature,
	index: number,
): PriceMapLabel | null {
	if (feature.geometry.type !== "Point") return null;
	const [longitude, latitude] = feature.geometry.coordinates;
	const price = spatialPropertyNumber(feature.properties, "p50");
	if (![longitude, latitude, price].every(Number.isFinite)) return null;
	const marketCode = spatialPropertyString(
		feature.properties,
		"marketCode",
		"market_code",
		"market_id",
	);
	return {
		// This id is used as the selected market code by the map, URL, rail and
		// table. GeoJSON feature.id is only a rendering identity and may differ.
		id: marketCode ?? String(feature.id ?? index),
		longitude,
		latitude,
		price: price as number,
		name: spatialPropertyString(
			feature.properties,
			"marketName",
			"market_name",
		),
	};
}

export function contourPriceLabel(
	feature: GeoJsonFeature,
	index: number,
): PriceMapLabel | null {
	const lines =
		feature.geometry.type === "LineString"
			? [feature.geometry.coordinates]
			: feature.geometry.type === "MultiLineString"
				? feature.geometry.coordinates
				: [];
	const line = [...lines].sort((a, b) => b.length - a.length)[0];
	if (!line?.length) return null;
	const point = line[Math.floor(line.length / 2)];
	const price = spatialPropertyNumber(
		feature.properties,
		"levelPriceTwdPerKg",
		"level_price_twd_per_kg",
	);
	if (!point || price == null) return null;
	return {
		id: String(feature.id ?? `contour-${index}`),
		longitude: point[0],
		latitude: point[1],
		price,
	};
}

interface RingSummary {
	longitude: number;
	latitude: number;
	area: number;
}

function summarizeRing(ring: number[][]): RingSummary | null {
	const points = ring.filter(
		(point) =>
			point.length >= 2 &&
			Number.isFinite(point[0]) &&
			Number.isFinite(point[1]),
	);
	if (points.length < 3) return null;

	let crossSum = 0;
	let longitudeSum = 0;
	let latitudeSum = 0;
	for (let index = 0; index < points.length; index += 1) {
		const current = points[index];
		const next = points[(index + 1) % points.length];
		const cross = current[0] * next[1] - next[0] * current[1];
		crossSum += cross;
		longitudeSum += (current[0] + next[0]) * cross;
		latitudeSum += (current[1] + next[1]) * cross;
	}

	const area = Math.abs(crossSum) / 2;
	if (area > Number.EPSILON) {
		return {
			longitude: longitudeSum / (3 * crossSum),
			latitude: latitudeSum / (3 * crossSum),
			area,
		};
	}

	return {
		longitude: points.reduce((sum, point) => sum + point[0], 0) / points.length,
		latitude: points.reduce((sum, point) => sum + point[1], 0) / points.length,
		area: 0,
	};
}

function featurePosition(feature: GeoJsonFeature): RingSummary | null {
	if (feature.geometry.type === "Point") {
		const [longitude, latitude] = feature.geometry.coordinates;
		return Number.isFinite(longitude) && Number.isFinite(latitude)
			? { longitude, latitude, area: 0 }
			: null;
	}
	const rings =
		feature.geometry.type === "Polygon"
			? feature.geometry.coordinates.slice(0, 1)
			: feature.geometry.type === "MultiPolygon"
				? feature.geometry.coordinates.flatMap((polygon) => polygon.slice(0, 1))
				: [];
	return (
		rings
			.map(summarizeRing)
			.filter((summary): summary is RingSummary => summary != null)
			.sort((a, b) => b.area - a.area)[0] ?? null
	);
}

export function boundaryPriceLabel(
	feature: GeoJsonFeature,
	index: number,
): RegionPriceLabel | null {
	const position = featurePosition(feature);
	const price = spatialPropertyNumber(
		feature.properties,
		"estimatedP50TwdPerKg",
		"estimated_p50_twd_per_kg",
	);
	if (!position || price == null) return null;
	const name = boundaryRegionName(feature.properties);
	const code = spatialPropertyString(
		feature.properties,
		"regionCode",
		"region_code",
		"county_code",
		"COUNTYCODE",
	);
	return {
		id: String(feature.id ?? code ?? `region-${index}`),
		boundaryId: spatialPropertyString(
			feature.properties,
			BOUNDARY_INTERACTION_ID,
		),
		...position,
		price,
		name: name ?? code ?? "行政區",
		lower: spatialPropertyNumber(
			feature.properties,
			"bandLowerTwdPerKg",
			"band_lower_twd_per_kg",
		),
		upper: spatialPropertyNumber(
			feature.properties,
			"bandUpperTwdPerKg",
			"band_upper_twd_per_kg",
		),
		supportingMarkets: spatialPropertyNumber(
			feature.properties,
			"supportingMarketCount",
			"supporting_market_count",
		),
		status: spatialPropertyString(feature.properties, "status"),
	};
}

export function thinPriceLabels<T extends PriceMapLabel>(
	labels: T[],
	options: {
		limit: number;
		longitudeDistance: number;
		latitudeDistance: number;
		reserved?: PriceMapLabel[];
	},
): T[] {
	const occupied = [...(options.reserved ?? [])];
	const accepted: T[] = [];
	for (const label of labels) {
		const collides = occupied.some(
			(other) =>
				Math.abs(other.longitude - label.longitude) <
					options.longitudeDistance &&
				Math.abs(other.latitude - label.latitude) < options.latitudeDistance,
		);
		if (collides) continue;
		accepted.push(label);
		occupied.push(label);
		if (accepted.length >= options.limit) break;
	}
	return accepted;
}
