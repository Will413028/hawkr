"use client";

import type {
	ExpressionSpecification,
	FillExtrusionLayerSpecification,
	FillLayerSpecification,
	FilterSpecification,
	LineLayerSpecification,
	StyleSpecification,
} from "maplibre-gl";
import * as maplibregl from "maplibre-gl";
import * as React from "react";
import MapLibreMap, {
	AttributionControl,
	Layer,
	type MapLayerMouseEvent,
	type MapRef,
	Marker,
	NavigationControl,
	Popup,
	Source,
} from "react-map-gl/maplibre";
import type { SpatialSurfaceData } from "@/lib/pricebook/repo";
import {
	BOUNDARY_INTERACTION_ID,
	boundaryInteractionCollection,
	boundaryPriceLabel,
	boundaryRegionName,
	contourPriceLabel,
	featureCollectionBounds,
	featurePriceDomain,
	type PriceMapLabel,
	pointPriceLabel,
	pricedFeatureCollection,
	priceScalePresentation,
	type RegionPriceLabel,
	spatialPropertyNumber,
	spatialPropertyString,
	thinPriceLabels,
} from "./spatialMapData";

// Prepared from this exact package by next.config.ts for webpack and Turbopack.
// Keep the worker beside its shared module and serve both from our own origin.
maplibregl.setWorkerUrl(
	`/generated/maplibre/${maplibregl.getVersion()}/maplibre-gl-worker.mjs`,
);

const BOUNDARY_LAYER_ID = "pricebook-boundary";
const BOUNDARY_PRICE_LAYER_ID = "pricebook-boundary-price";
const BOUNDARY_PRICE_LINE_LAYER_ID = "pricebook-boundary-price-lines";
const BOUNDARY_PRICE_SELECTED_LAYER_ID = "pricebook-boundary-price-selected";
const BOUNDARY_INFO_FILL_LAYER_ID = "pricebook-boundary-info-fill";
const BOUNDARY_INFO_HIT_LAYER_ID = "pricebook-boundary-info-hit";
const SURFACE_FILL_LAYER_ID = "pricebook-surface-fill";
const SURFACE_HEIGHT_LAYER_ID = "pricebook-surface-height";
const CONTOUR_LAYER_ID = "pricebook-contours";
const MARKET_LAYER_ID = "pricebook-markets";

const HEIGHT_CAMERA_PITCH = 54;
const HEIGHT_CAMERA_BEARING = -12;
const HEIGHT_FIT_PADDING = { top: 74, right: 48, bottom: 52, left: 48 };
const CONTOUR_FIT_PADDING = 34;

// maplibre style-spec 只接受色值、不解析 CSS 變數，所以地圖的墨色寫字面值（同 --color-ink-800）。
const PRICE_MAP_INK = "#323f4b";

const PRICE_COLORS = [
	"#e4e7eb",
	"#cbd2d9",
	"#9aa5b1",
	"#d7c567",
	"#df984d",
	"#c86643",
	"#903e39",
	"#6f2f31",
] as const;

const PRICEBOOK_MAP_STYLE: StyleSpecification = {
	version: 8,
	sources: {},
	layers: [
		{
			id: "pricebook-map-background",
			type: "background",
			paint: { "background-color": "#e4e7eb" },
		},
	],
};

const BASE_BOUNDARY_FILL: FillLayerSpecification = {
	id: BOUNDARY_LAYER_ID,
	type: "fill",
	source: "taiwan-boundary",
	paint: {
		"fill-color": "#f5f7fa",
		"fill-opacity": 0.72,
		"fill-outline-color": "#6f8178",
	},
};

const BASE_BOUNDARY_LINE: LineLayerSpecification = {
	id: `${BOUNDARY_LAYER_ID}-line`,
	type: "line",
	source: "taiwan-boundary",
	paint: {
		"line-color": "#616e7c",
		"line-opacity": 0.78,
		"line-width": ["interpolate", ["linear"], ["zoom"], 5, 0.7, 9, 1.45],
	},
};

// Hit geometry is deliberately wider than the unchanged visible boundary.
const BOUNDARY_INFO_FILL: FillLayerSpecification = {
	id: BOUNDARY_INFO_FILL_LAYER_ID,
	type: "fill",
	source: "boundary-info",
	paint: { "fill-opacity": 0 },
};

const BOUNDARY_INFO_HIT: LineLayerSpecification = {
	id: BOUNDARY_INFO_HIT_LAYER_ID,
	type: "line",
	source: "boundary-info",
	paint: { "line-width": 16, "line-opacity": 0 },
};

const CONTOUR_LAYER: LineLayerSpecification = {
	id: CONTOUR_LAYER_ID,
	type: "line",
	source: "price-contours",
	paint: {
		"line-color": PRICE_MAP_INK,
		"line-opacity": 0.84,
		"line-width": ["interpolate", ["linear"], ["zoom"], 5, 0.85, 9, 1.8],
		"line-dasharray": [2, 1.4],
	},
};

function finiteBreaks(surface: SpatialSurfaceData): number[] {
	return [...new Set(surface.scale.breaks.filter(Number.isFinite))].sort(
		(a, b) => a - b,
	);
}

function valueExpression(...keys: string[]): ExpressionSpecification {
	return [
		"coalesce",
		...keys.map(
			(key) => ["to-number", ["get", key]] as ExpressionSpecification,
		),
		-1,
	] as ExpressionSpecification;
}

function priceBandColors(bandCount: number): string[] {
	if (bandCount <= 1)
		return [PRICE_COLORS[Math.floor(PRICE_COLORS.length / 2)]];
	return Array.from({ length: bandCount }, (_, index) => {
		const paletteIndex = Math.round(
			(index * (PRICE_COLORS.length - 1)) / (bandCount - 1),
		);
		return PRICE_COLORS[paletteIndex];
	});
}

function adaptivePriceColorExpression(
	breaks: number[],
	value: ExpressionSpecification,
	isFlat: boolean,
): ExpressionSpecification {
	if (isFlat) {
		const neutral = PRICE_COLORS[Math.floor(PRICE_COLORS.length / 2)];
		return ["case", true, neutral, neutral] as ExpressionSpecification;
	}
	const colors = priceBandColors(breaks.length + 1);
	const expression: unknown[] = ["step", value, colors[0]];
	breaks.forEach((priceBreak, index) => {
		expression.push(priceBreak, colors[index + 1]);
	});
	return expression as ExpressionSpecification;
}

function formatMapPrice(value?: number): string {
	if (value == null || !Number.isFinite(value)) return "—";
	return `NT$${Math.round(value).toLocaleString("zh-TW")}`;
}

interface HoverDetails {
	boundaryId?: string;
	longitude: number;
	latitude: number;
	kind: "market" | "surface" | "region";
	name: string;
	price?: number;
	lower?: number;
	upper?: number;
	supportingMarkets?: number;
	status?: string;
}

export type PriceMapMode = "contour" | "height" | "points";

function regionHover(label: RegionPriceLabel): HoverDetails {
	return {
		boundaryId: label.boundaryId,
		longitude: label.longitude,
		latitude: label.latitude,
		kind: "region",
		name: label.name ?? "行政區",
		price: label.price,
		lower: label.lower,
		upper: label.upper,
		supportingMarkets: label.supportingMarkets,
		status: label.status,
	};
}

type InteractiveMapFeature = NonNullable<
	MapLayerMouseEvent["features"]
>[number];

function regionFeatureDetails(
	event: MapLayerMouseEvent,
	region: InteractiveMapFeature,
): HoverDetails {
	return {
		longitude: event.lngLat.lng,
		latitude: event.lngLat.lat,
		kind: "region",
		boundaryId: spatialPropertyString(
			region.properties,
			BOUNDARY_INTERACTION_ID,
		),
		name: boundaryRegionName(region.properties),
		price: spatialPropertyNumber(
			region.properties,
			"estimatedP50TwdPerKg",
			"estimated_p50_twd_per_kg",
		),
		lower: spatialPropertyNumber(
			region.properties,
			"bandLowerTwdPerKg",
			"band_lower_twd_per_kg",
		),
		upper: spatialPropertyNumber(
			region.properties,
			"bandUpperTwdPerKg",
			"band_upper_twd_per_kg",
		),
		supportingMarkets: spatialPropertyNumber(
			region.properties,
			"supportingMarketCount",
			"supporting_market_count",
		),
		status: spatialPropertyString(region.properties, "status"),
	};
}

function hoverDetails(
	event: MapLayerMouseEvent,
	preferRegion = false,
): HoverDetails | null {
	const features = event.features ?? [];
	const market = features.find(
		(feature) => feature.layer.id === MARKET_LAYER_ID,
	);
	if (market) {
		return {
			longitude: event.lngLat.lng,
			latitude: event.lngLat.lat,
			kind: "market",
			name:
				spatialPropertyString(market.properties, "marketName", "market_name") ??
				"批發市場",
			price: spatialPropertyNumber(market.properties, "p50"),
			lower: spatialPropertyNumber(market.properties, "p10"),
			upper: spatialPropertyNumber(market.properties, "p90"),
			status: spatialPropertyString(
				market.properties,
				"marketStatus",
				"market_status",
			),
		};
	}
	const regions = features.filter((feature) =>
		[BOUNDARY_INFO_FILL_LAYER_ID, BOUNDARY_INFO_HIT_LAYER_ID].includes(
			feature.layer.id,
		),
	);
	// Summary metadata is more informative than the overlapping raw boundary,
	// including when its price is null. Never synthesize a price from neighbors.
	const region =
		regions.find((feature) =>
			spatialPropertyString(
				feature.properties,
				BOUNDARY_INTERACTION_ID,
			)?.startsWith("metadata:"),
		) ?? regions[0];
	if (preferRegion && region) return regionFeatureDetails(event, region);
	const surface = features.find((feature) =>
		[SURFACE_FILL_LAYER_ID, SURFACE_HEIGHT_LAYER_ID].includes(feature.layer.id),
	);
	if (surface) {
		return {
			longitude: event.lngLat.lng,
			latitude: event.lngLat.lat,
			kind: "surface",
			name: "市場間空間插值",
			price: spatialPropertyNumber(
				surface.properties,
				"priceTwdPerKg",
				"price_twd_per_kg",
			),
			lower: spatialPropertyNumber(
				surface.properties,
				"bandLowerTwdPerKg",
				"lowerPriceTwdPerKg",
				"lower_price_twd_per_kg",
			),
			upper: spatialPropertyNumber(
				surface.properties,
				"bandUpperTwdPerKg",
				"upperPriceTwdPerKg",
				"upper_price_twd_per_kg",
			),
			supportingMarkets: spatialPropertyNumber(
				surface.properties,
				"supportingMarketCount",
				"supporting_neighbor_count",
			),
		};
	}
	return region ? regionFeatureDetails(event, region) : null;
}

export function TaiwanPriceMap({
	surface,
	mode,
	viewIdentity,
	selectedMarketCode,
	onMarketSelect,
}: {
	surface: SpatialSurfaceData;
	mode: PriceMapMode;
	viewIdentity: string;
	selectedMarketCode?: string | null;
	onMarketSelect?: (marketCode: string) => void;
}) {
	const mapRef = React.useRef<MapRef>(null);
	const mapContainerRef = React.useRef<HTMLDivElement>(null);
	const popupCloseRef = React.useRef<HTMLButtonElement>(null);
	const fittedViewIdentityRef = React.useRef<string | null>(null);
	const [mapReady, setMapReady] = React.useState(false);
	const [hover, setHover] = React.useState<HoverDetails | null>(null);
	const [selectedRegion, setSelectedRegion] =
		React.useState<HoverDetails | null>(null);
	const popupDetails = selectedRegion ?? hover;
	const closeRegion = () => {
		setSelectedRegion(null);
		setHover(null);
		mapRef.current?.getCanvas().focus();
	};
	const [compactLabels, setCompactLabels] = React.useState(false);
	const breaks = React.useMemo(() => finiteBreaks(surface), [surface]);
	const scalePresentation = React.useMemo(
		() => priceScalePresentation(surface.scale, surface.statistics),
		[surface.scale, surface.statistics],
	);
	const scaleDomain = scalePresentation?.domain ?? null;
	const isFlatScale = scalePresentation?.isFlat ?? false;
	const hasPriceScale = Boolean(scalePresentation);
	const legendColors = React.useMemo(
		() =>
			isFlatScale ? priceBandColors(1) : priceBandColors(breaks.length + 1),
		[breaks.length, isFlatScale],
	);
	const surfaceCells = React.useMemo(
		() =>
			pricedFeatureCollection(
				surface.surfaceCellsGeojson,
				"priceTwdPerKg",
				"price_twd_per_kg",
			),
		[surface.surfaceCellsGeojson],
	);
	const boundaryMetadata = React.useMemo(
		() =>
			boundaryInteractionCollection(surface.boundaryPriceGeojson, "metadata"),
		[surface.boundaryPriceGeojson],
	);
	const boundaryInfo = React.useMemo(
		() => ({
			type: "FeatureCollection" as const,
			features: [
				...boundaryInteractionCollection(surface.boundary.geojson, "boundary")
					.features,
				...boundaryMetadata.features,
			],
		}),
		[boundaryMetadata, surface.boundary.geojson],
	);
	const previousBoundaryInfo = React.useRef(boundaryInfo);
	React.useEffect(() => {
		if (previousBoundaryInfo.current === boundaryInfo) return;
		previousBoundaryInfo.current = boundaryInfo;
		// Local feature IDs belong to this data snapshot, including unpriced metadata.
		setSelectedRegion(null);
		setHover(null);
	}, [boundaryInfo]);
	const boundaryPrices = React.useMemo(
		() =>
			pricedFeatureCollection(
				boundaryMetadata,
				"estimatedP50TwdPerKg",
				"estimated_p50_twd_per_kg",
			),
		[boundaryMetadata],
	);
	const marketPoints = React.useMemo(
		() => pricedFeatureCollection(surface.marketPointsGeojson, "p50"),
		[surface.marketPointsGeojson],
	);
	const contourLines = React.useMemo(
		() =>
			pricedFeatureCollection(
				surface.contourLinesGeojson,
				"levelPriceTwdPerKg",
				"level_price_twd_per_kg",
			),
		[surface.contourLinesGeojson],
	);
	const adaptiveHeightDomain = React.useMemo(
		() =>
			isFlatScale
				? null
				: featurePriceDomain(surfaceCells, "priceTwdPerKg", "price_twd_per_kg"),
		[isFlatScale, surfaceCells],
	);
	const heightMinimum = adaptiveHeightDomain?.minimum ?? breaks[0] ?? 0;
	const heightMaximum = adaptiveHeightDomain?.maximum ?? breaks.at(-1) ?? 1;
	const bounds = React.useMemo(() => {
		const renderedBounds = featureCollectionBounds(surfaceCells, marketPoints);
		const [
			minimumLongitude,
			minimumLatitude,
			maximumLongitude,
			maximumLatitude,
		] =
			renderedBounds ??
			(surface.boundary.bboxWgs84.length >= 4
				? (surface.boundary.bboxWgs84 as [number, number, number, number])
				: [119.2, 21.75, 122.2, 25.7]);
		return [
			[minimumLongitude, minimumLatitude],
			[maximumLongitude, maximumLatitude],
		] as [[number, number], [number, number]];
	}, [marketPoints, surface.boundary.bboxWgs84, surfaceCells]);
	const hasSurface = hasPriceScale && surfaceCells.features.length > 0;
	const hasBoundaryPriceFill =
		hasPriceScale && boundaryPrices.features.length > 0;
	const hasContours =
		hasPriceScale && !isFlatScale && contourLines.features.length > 0;
	const surfaceColor = React.useMemo(
		() =>
			adaptivePriceColorExpression(
				breaks,
				valueExpression("priceTwdPerKg", "price_twd_per_kg"),
				isFlatScale,
			),
		[breaks, isFlatScale],
	);
	const boundaryColor = React.useMemo(
		() =>
			adaptivePriceColorExpression(
				breaks,
				valueExpression("estimatedP50TwdPerKg", "estimated_p50_twd_per_kg"),
				isFlatScale,
			),
		[breaks, isFlatScale],
	);

	const surfaceFill = React.useMemo<FillLayerSpecification>(
		() => ({
			id: SURFACE_FILL_LAYER_ID,
			type: "fill",
			source: "price-surface",
			layout: { visibility: mode === "contour" ? "visible" : "none" },
			paint: {
				"fill-color": surfaceColor,
				"fill-opacity": 0.78,
				"fill-outline-color": "rgba(255,255,255,0.14)",
			},
		}),
		[mode, surfaceColor],
	);
	const surfaceHeight = React.useMemo<FillExtrusionLayerSpecification>(
		() => ({
			id: SURFACE_HEIGHT_LAYER_ID,
			type: "fill-extrusion",
			source: "price-surface",
			layout: { visibility: mode === "height" ? "visible" : "none" },
			paint: {
				"fill-extrusion-color": surfaceColor,
				"fill-extrusion-height": isFlatScale
					? 500
					: [
							"interpolate",
							["linear"],
							valueExpression("priceTwdPerKg", "price_twd_per_kg"),
							heightMinimum,
							500,
							heightMaximum,
							18000,
						],
				"fill-extrusion-opacity": 0.84,
				"fill-extrusion-vertical-gradient": true,
			},
		}),
		[heightMaximum, heightMinimum, isFlatScale, mode, surfaceColor],
	);
	const boundaryPriceFill = React.useMemo<FillLayerSpecification>(
		() => ({
			id: BOUNDARY_PRICE_LAYER_ID,
			type: "fill",
			source: "boundary-prices",
			paint: { "fill-color": boundaryColor, "fill-opacity": 0.34 },
		}),
		[boundaryColor],
	);
	const selectedBoundaryLine = React.useMemo<LineLayerSpecification>(
		() => ({
			id: BOUNDARY_PRICE_SELECTED_LAYER_ID,
			type: "line",
			source: "boundary-info",
			filter: [
				"==",
				["get", BOUNDARY_INTERACTION_ID],
				selectedRegion?.boundaryId ?? "__none__",
			] as FilterSpecification,
			paint: {
				"line-color": PRICE_MAP_INK,
				"line-width": ["interpolate", ["linear"], ["zoom"], 5, 2.2, 9, 4],
				"line-opacity": 0.96,
			},
		}),
		[selectedRegion?.boundaryId],
	);
	const boundaryPriceLine = React.useMemo<LineLayerSpecification>(
		() => ({
			id: BOUNDARY_PRICE_LINE_LAYER_ID,
			type: "line",
			source: "boundary-prices",
			paint: {
				"line-color": "#3e4c59",
				"line-width": ["interpolate", ["linear"], ["zoom"], 5, 0.8, 9, 1.7],
				"line-opacity": 0.68,
			},
		}),
		[],
	);
	const marketLayer = React.useMemo(
		() => ({
			id: MARKET_LAYER_ID,
			type: "circle" as const,
			source: "market-points",
			paint: {
				"circle-radius": [
					"interpolate",
					["linear"],
					["zoom"],
					5,
					[
						"case",
						[
							"==",
							[
								"coalesce",
								["get", "marketCode"],
								["get", "market_code"],
								["get", "market_id"],
							],
							selectedMarketCode ?? "",
						],
						9,
						4.5,
					],
					9,
					[
						"case",
						[
							"==",
							[
								"coalesce",
								["get", "marketCode"],
								["get", "market_code"],
								["get", "market_id"],
							],
							selectedMarketCode ?? "",
						],
						11,
						7,
					],
				] as ExpressionSpecification,
				"circle-color": [
					"case",
					["boolean", ["get", "usedForSurface"], false],
					PRICE_MAP_INK,
					["boolean", ["get", "used_for_surface"], false],
					PRICE_MAP_INK,
					"#b9653f",
				] as ExpressionSpecification,
				"circle-stroke-color": "#fffdf7",
				"circle-stroke-width": 2,
			},
		}),
		[selectedMarketCode],
	);

	React.useEffect(() => {
		if (!selectedRegion) return;
		popupCloseRef.current?.focus({ preventScroll: true });
		const container = mapContainerRef.current;
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key !== "Escape") return;
			event.stopPropagation();
			setSelectedRegion(null);
			setHover(null);
			mapRef.current?.getCanvas().focus();
		};
		container?.addEventListener("keydown", onKeyDown);
		return () => container?.removeEventListener("keydown", onKeyDown);
	}, [selectedRegion]);

	React.useEffect(() => {
		const media = window.matchMedia("(max-width: 640px)");
		const update = () => setCompactLabels(media.matches);
		update();
		media.addEventListener("change", update);
		return () => media.removeEventListener("change", update);
	}, []);

	const allMarketLabels = React.useMemo(
		() =>
			marketPoints.features
				.map(pointPriceLabel)
				.filter((label): label is PriceMapLabel => label != null),
		[marketPoints.features],
	);
	const selectedMarketLabel = allMarketLabels.find(
		(label) => label.id.replace(/^moa:/, "") === selectedMarketCode,
	);
	React.useEffect(() => {
		if (!mapReady) return;
		setHover(null);
		setSelectedRegion(null);
		const frame = window.requestAnimationFrame(() => {
			const map = mapRef.current;
			if (!map) return;
			const identityChanged = fittedViewIdentityRef.current !== viewIdentity;
			fittedViewIdentityRef.current = viewIdentity;
			const reducedMotion = window.matchMedia(
				"(prefers-reduced-motion: reduce)",
			).matches;
			if (selectedMarketLabel) {
				map.easeTo({
					center: [selectedMarketLabel.longitude, selectedMarketLabel.latitude],
					zoom: identityChanged ? 6.35 : Math.max(map.getZoom(), 6.35),
					pitch: mode === "height" ? HEIGHT_CAMERA_PITCH : 0,
					bearing: mode === "height" ? HEIGHT_CAMERA_BEARING : 0,
					duration: reducedMotion ? 0 : 420,
				});
				return;
			}
			map.fitBounds(bounds, {
				padding: mode === "height" ? HEIGHT_FIT_PADDING : CONTOUR_FIT_PADDING,
				pitch: mode === "height" ? HEIGHT_CAMERA_PITCH : 0,
				bearing: mode === "height" ? HEIGHT_CAMERA_BEARING : 0,
				duration: reducedMotion ? 0 : identityChanged ? 420 : 360,
			});
		});
		return () => window.cancelAnimationFrame(frame);
	}, [bounds, mapReady, mode, selectedMarketLabel, viewIdentity]);
	const allRegionLabels = React.useMemo(
		() =>
			boundaryPrices.features
				.map(boundaryPriceLabel)
				.filter((label): label is RegionPriceLabel => label != null)
				.sort(
					(a, b) =>
						(b.supportingMarkets ?? 0) - (a.supportingMarkets ?? 0) ||
						b.area - a.area,
				),
		[boundaryPrices.features],
	);
	const allContourLabels = React.useMemo(
		() =>
			contourLines.features
				.map(contourPriceLabel)
				.filter((label): label is PriceMapLabel => label != null),
		[contourLines.features],
	);
	const contourLabels = React.useMemo(
		() =>
			thinPriceLabels(allContourLabels, {
				limit: compactLabels ? 4 : 10,
				longitudeDistance: compactLabels ? 0.55 : 0.35,
				latitudeDistance: compactLabels ? 0.32 : 0.2,
				reserved: selectedMarketLabel ? [selectedMarketLabel] : [],
			}),
		[allContourLabels, compactLabels, selectedMarketLabel],
	);
	const regionLabels = React.useMemo(
		() =>
			thinPriceLabels(allRegionLabels, {
				limit: compactLabels ? 4 : 8,
				longitudeDistance: compactLabels ? 0.75 : 0.45,
				latitudeDistance: compactLabels ? 0.48 : 0.32,
				reserved: [
					...contourLabels,
					...(selectedMarketLabel ? [selectedMarketLabel] : []),
				],
			}),
		[allRegionLabels, compactLabels, contourLabels, selectedMarketLabel],
	);
	const marketLabels = React.useMemo(() => {
		const ordered = selectedMarketLabel
			? [
					selectedMarketLabel,
					...allMarketLabels.filter(
						(label) => label.id !== selectedMarketLabel.id,
					),
				]
			: allMarketLabels;
		return thinPriceLabels(ordered, {
			limit: compactLabels ? 3 : 6,
			longitudeDistance: compactLabels ? 0.72 : 0.42,
			latitudeDistance: compactLabels ? 0.46 : 0.3,
			reserved: [...contourLabels, ...regionLabels],
		});
	}, [
		allMarketLabels,
		compactLabels,
		contourLabels,
		regionLabels,
		selectedMarketLabel,
	]);
	const interactiveLayerIds = React.useMemo(
		() => [
			MARKET_LAYER_ID,
			...(hasSurface ? [SURFACE_FILL_LAYER_ID, SURFACE_HEIGHT_LAYER_ID] : []),
			BOUNDARY_INFO_FILL_LAYER_ID,
			BOUNDARY_INFO_HIT_LAYER_ID,
		],
		[hasSurface],
	);
	return (
		<div ref={mapContainerRef} className="seller-price-map" data-lenis-prevent>
			<div className="seller-price-map__canvas">
				<MapLibreMap
					mapLib={maplibregl}
					ref={mapRef}
					onLoad={() => setMapReady(true)}
					initialViewState={{
						bounds,
						fitBoundsOptions: {
							padding:
								mode === "height" ? HEIGHT_FIT_PADDING : CONTOUR_FIT_PADDING,
						},
						pitch: mode === "height" ? HEIGHT_CAMERA_PITCH : 0,
						bearing: mode === "height" ? HEIGHT_CAMERA_BEARING : 0,
					}}
					mapStyle={PRICEBOOK_MAP_STYLE}
					minZoom={4.6}
					maxZoom={11}
					maxPitch={60}
					dragRotate={mode === "height"}
					touchPitch={mode === "height"}
					attributionControl={false}
					interactiveLayerIds={interactiveLayerIds}
					cursor={hover ? "pointer" : "grab"}
					onMouseMove={(event) => setHover(hoverDetails(event))}
					onClick={(event) => {
						// A click selects the administrative boundary when it overlaps the
						// continuous surface. Hover still favors the precise surface cell.
						const details = hoverDetails(event, true);
						setHover(details);
						setSelectedRegion(details?.kind === "region" ? details : null);
						const feature = event.features?.find(
							(item) => item.layer.id === MARKET_LAYER_ID,
						);
						const code = spatialPropertyString(
							feature?.properties,
							"marketCode",
							"market_code",
							"market_id",
						);
						if (code) onMarketSelect?.(code.replace(/^moa:/, ""));
					}}
					onMouseLeave={() => setHover(null)}
				>
					<Source
						id="taiwan-boundary"
						type="geojson"
						data={surface.boundary.geojson as never}
					>
						<Layer {...BASE_BOUNDARY_FILL} />
						<Layer {...BASE_BOUNDARY_LINE} />
					</Source>
					{hasBoundaryPriceFill ? (
						<>
							<Source
								id="boundary-prices"
								type="geojson"
								data={boundaryPrices as never}
							/>
							<Layer {...boundaryPriceFill} />
						</>
					) : null}
					{hasSurface ? (
						<Source
							id="price-surface"
							type="geojson"
							data={surfaceCells as never}
						>
							<Layer {...surfaceFill} />
							<Layer {...surfaceHeight} />
						</Source>
					) : null}
					<Source
						id="price-contours"
						type="geojson"
						data={contourLines as never}
					>
						<Layer
							{...CONTOUR_LAYER}
							layout={{ visibility: mode === "contour" ? "visible" : "none" }}
						/>
					</Source>
					{hasBoundaryPriceFill ? <Layer {...boundaryPriceLine} /> : null}
					<Source
						id="boundary-info"
						type="geojson"
						data={boundaryInfo as never}
					>
						<Layer {...BOUNDARY_INFO_FILL} />
						<Layer {...BOUNDARY_INFO_HIT} />
						<Layer {...selectedBoundaryLine} />
					</Source>
					<Source
						id="market-points"
						type="geojson"
						data={marketPoints as never}
					>
						<Layer {...marketLayer} />
					</Source>

					{regionLabels.map((label) => (
						<Marker
							key={label.id}
							longitude={label.longitude}
							latitude={label.latitude}
							anchor="center"
						>
							<button
								type="button"
								className={`seller-price-map__region-label${selectedRegion?.boundaryId === label.boundaryId ? " is-selected" : ""}`}
								aria-label={`${label.name ?? "行政區"}，D+${surface.horizonDays} 邊界價格 ${formatMapPrice(label.price)} 每公斤`}
								aria-pressed={selectedRegion?.boundaryId === label.boundaryId}
								onClick={(event) => {
									event.stopPropagation();
									setSelectedRegion(regionHover(label));
								}}
								onFocus={() => setHover(regionHover(label))}
							>
								<span>{label.name ?? "行政區"}</span>
								<strong>{formatMapPrice(label.price)}</strong>
							</button>
						</Marker>
					))}
					{mode === "contour"
						? contourLabels.map((label) => (
								<Marker
									key={label.id}
									longitude={label.longitude}
									latitude={label.latitude}
									anchor="center"
								>
									<span className="seller-price-map__contour-label">
										{formatMapPrice(label.price)}/kg
									</span>
								</Marker>
							))
						: null}
					{marketLabels.map((label) => {
						const selected =
							label.id.replace(/^moa:/, "") === selectedMarketCode;
						return (
							<Marker
								key={label.id}
								longitude={label.longitude}
								latitude={label.latitude}
								anchor="bottom"
								offset={[0, -8]}
							>
								<button
									type="button"
									className={`seller-price-map__market-label${selected ? " is-selected" : ""}`}
									aria-label={`${label.name ?? "批發市場"}，D+${surface.horizonDays} 中位價 ${formatMapPrice(label.price)} 每公斤`}
									aria-pressed={selected}
									onClick={() =>
										onMarketSelect?.(label.id.replace(/^moa:/, ""))
									}
								>
									{selected ? <span>{label.name ?? "已選市場"}</span> : null}
									<strong>{formatMapPrice(label.price)}</strong>
								</button>
							</Marker>
						);
					})}

					<NavigationControl
						position="top-right"
						showCompass={mode === "height"}
					/>
					<AttributionControl
						position="bottom-right"
						compact
						customAttribution={`行政界：${surface.boundary.source?.label ?? "來源見資料系譜"}；價格：Hawkr 行情`}
					/>
					{popupDetails ? (
						<Popup
							longitude={popupDetails.longitude}
							latitude={popupDetails.latitude}
							closeButton={false}
							closeOnClick={false}
							focusAfterOpen={false}
							offset={14}
							className={`seller-price-map__popup${selectedRegion ? " is-selected" : ""}`}
						>
							<section
								className="seller-price-map__tooltip"
								role={selectedRegion ? "dialog" : "tooltip"}
								aria-label={selectedRegion ? "行政區資訊" : undefined}
							>
								{selectedRegion ? (
									<button
										ref={popupCloseRef}
										type="button"
										className="seller-price-map__popup-close"
										aria-label="關閉地圖資訊"
										onClick={(event) => {
											event.stopPropagation();
											closeRegion();
										}}
									>
										<span aria-hidden="true">×</span>
									</button>
								) : null}
								<span>
									{popupDetails.kind === "market"
										? "批發市場預測"
										: popupDetails.kind === "region"
											? "行政區資訊"
											: "價格曲面"}
								</span>
								<strong>{popupDetails.name}</strong>
								{popupDetails.price != null ? (
									<b>{formatMapPrice(popupDetails.price)} / kg</b>
								) : (
									<b>目前沒有可用價格</b>
								)}
								{popupDetails.lower != null || popupDetails.upper != null ? (
									<small>
										顯示帶 {formatMapPrice(popupDetails.lower)}～
										{formatMapPrice(popupDetails.upper)}
									</small>
								) : null}
								{popupDetails.supportingMarkets != null ? (
									<small>{popupDetails.supportingMarkets} 個市場支撐</small>
								) : null}
								{popupDetails.status && popupDetails.status !== "ready" ? (
									<small>狀態：{popupDetails.status}</small>
								) : null}
							</section>
						</Popup>
					) : null}
				</MapLibreMap>

				<div className="seller-price-map__stamp" aria-hidden="true">
					<span>
						{mode === "height"
							? "立體價格高度"
							: mode === "contour"
								? "P50 價格等高線"
								: "市場與行政區價格"}
					</span>
					<strong>D+{surface.horizonDays}</strong>
					{mode === "height" && adaptiveHeightDomain ? (
						<small>
							{formatMapPrice(adaptiveHeightDomain.minimum)}～
							{formatMapPrice(adaptiveHeightDomain.maximum)} 自適應
						</small>
					) : null}
				</div>
				{!hasPriceScale ? (
					<div className="seller-price-map__overlay-note" role="status">
						<strong>價格色階尚未提供</strong>
						<span>缺少目前品項的完整 P50 尺度，因此只顯示精確價格。</span>
					</div>
				) : mode === "points" ? (
					<div className="seller-price-map__overlay-note" role="status">
						<strong>目前僅支援市場點與行政區價格</strong>
						<span>等高線與立體價格面未達品質門檻；空白不是零元。</span>
					</div>
				) : mode === "contour" && !hasContours ? (
					<div className="seller-price-map__overlay-note" role="status">
						<strong>等高線尚未提供</strong>
						<span>保留市場與行政區精確價格，不在前端自行插值。</span>
					</div>
				) : mode === "height" && !hasSurface ? (
					<div className="seller-price-map__overlay-note" role="status">
						<strong>立體價格面尚未提供</strong>
						<span>保留市場與行政區精確價格，不把缺值補成零元。</span>
					</div>
				) : null}
			</div>

			{hasPriceScale ? (
				<div
					className="seller-price-map__legend"
					role="img"
					aria-label={
						scalePresentation?.kind === "flat"
							? `目前品項的市場 P50 差異低於色階顯示精度，全市場 P50 約為 ${formatMapPrice(scaleDomain?.centre)}，每公斤`
							: scalePresentation?.kind === "fixed"
								? `固定參考價格色階：${breaks.map((price) => formatMapPrice(price)).join("、")}，每公斤`
								: `目前品項自適應 P50 色階：低值 ${formatMapPrice(scaleDomain?.minimum)}、全市場 P50 ${formatMapPrice(scaleDomain?.centre)}、高值 ${formatMapPrice(scaleDomain?.maximum)}；等高線門檻 ${breaks.map((price) => formatMapPrice(price)).join("、")}，每公斤`
					}
				>
					<div className="seller-price-map__legend-scale" aria-hidden="true">
						{legendColors.map((color) => (
							<i key={color} style={{ backgroundColor: color }} />
						))}
					</div>
					{scalePresentation?.kind === "fixed" ? (
						<>
							<span>低於 {formatMapPrice(breaks[0])}</span>
							<span>固定參考色階／公斤</span>
							<span>{formatMapPrice(breaks.at(-1))} 以上</span>
						</>
					) : (
						<>
							<span>低值 {formatMapPrice(scaleDomain?.minimum)}</span>
							<span>
								全市場 P50 {formatMapPrice(scaleDomain?.centre)}／公斤
							</span>
							<span>高值 {formatMapPrice(scaleDomain?.maximum)}</span>
						</>
					)}
				</div>
			) : null}
			<p className="seller-price-map__accessibility">
				{scalePresentation?.kind === "flat"
					? "本期市場 P50 差異低於色階顯示精度；地圖使用單一色帶與一致高度，不顯示等高線。完整數值另列於下方。"
					: scalePresentation?.kind === "fixed"
						? mode === "height"
							? "立體高度依可見價格面正規化；顏色與等高線暫用固定參考色階。完整數值另列於下方。"
							: "顏色與等高線暫用固定參考色階；市場點與行政區仍直接標價。"
						: mode === "height"
							? "立體高度依可見價格面正規化；顏色與等高線依目前品項及日期的 P50 尺度調整。完整數值另列於下方。"
							: "顏色與等高線依目前品項及日期的 P50 尺度調整；市場點與行政區仍直接標價，不需只靠顏色判讀。"}
			</p>
			{allRegionLabels.length ? (
				<details className="seller-price-map__region-index">
					<summary>行政區邊界價格（{allRegionLabels.length} 區）</summary>
					<ul>
						{allRegionLabels.map((label) => (
							<li key={label.id}>
								<span>{label.name ?? "行政區"}</span>
								<strong>{formatMapPrice(label.price)} / kg</strong>
							</li>
						))}
					</ul>
				</details>
			) : null}
		</div>
	);
}

/** Discoverable MapLibre preview shown before a crop is selected. It keeps the
 * original Pricebook map interaction visible without inventing a spatial
 * price surface. */
export function TaiwanPriceMapPreview() {
	return (
		<section
			className="seller-price-map seller-price-map--preview"
			data-lenis-prevent
			aria-label="全臺價格地圖預覽"
			aria-describedby="price-map-preview-help"
		>
			<div className="seller-price-map__canvas">
				<MapLibreMap
					mapLib={maplibregl}
					initialViewState={{
						bounds: [
							[119.2, 21.75],
							[122.2, 25.7],
						],
						fitBoundsOptions: { padding: 34 },
					}}
					mapStyle={PRICEBOOK_MAP_STYLE}
					minZoom={4.6}
					maxZoom={9}
					dragRotate={false}
					attributionControl={false}
				>
					<Source
						id="taiwan-boundary-preview"
						type="geojson"
						data="/data/taiwan.geojson"
					>
						<Layer
							{...BASE_BOUNDARY_FILL}
							id="pricebook-boundary-preview"
							source="taiwan-boundary-preview"
						/>
						<Layer
							{...BASE_BOUNDARY_LINE}
							id="pricebook-boundary-preview-line"
							source="taiwan-boundary-preview"
						/>
					</Source>
					<NavigationControl position="top-right" showCompass={false} />
					<AttributionControl
						position="bottom-right"
						compact
						customAttribution="位置輪廓；尚未載入價格"
					/>
				</MapLibreMap>
			</div>
			<p
				id="price-map-preview-help"
				className="seller-price-map__accessibility"
			>
				尚未選擇品項，目前只顯示臺灣輪廓。
			</p>
		</section>
	);
}

export interface PriceMapUnavailableReason {
	/** 具體原因（例：「D+2（2026-08-10）15 個市場中有 12 個休市」）；沒有就
	 *  退回通用文案，維持既有網路/契約錯誤的行為不變。 */
	summary: string;
	/** 品質代碼的人類可讀說明，沿用 PriceTopographyPanel 既有的 QUALITY_LABELS。 */
	details: string[];
}

/** MapLibre fallback for the current BFF gap, or for a genuine backend
 * status:"unavailable" (`reason` present — most commonly every wholesale
 * market observing a shared rest day for the requested horizon). The bundled
 * county file is used as a visual outline only; it is never treated as an
 * approved interpolation boundary and receives no fabricated prices. */
export function TaiwanPriceMapUnavailable({
	onRetry,
	reason,
}: {
	onRetry: () => void;
	reason?: PriceMapUnavailableReason;
}) {
	return (
		<div
			className="seller-price-map seller-price-map--unavailable"
			data-lenis-prevent
		>
			<div className="seller-price-map__canvas">
				<MapLibreMap
					mapLib={maplibregl}
					initialViewState={{
						bounds: [
							[119.2, 21.75],
							[122.2, 25.7],
						],
						fitBoundsOptions: { padding: 34 },
					}}
					mapStyle={PRICEBOOK_MAP_STYLE}
					minZoom={4.6}
					maxZoom={9}
					attributionControl={false}
				>
					<Source
						id="taiwan-boundary"
						type="geojson"
						data="/data/taiwan.geojson"
					>
						<Layer {...BASE_BOUNDARY_FILL} />
						<Layer {...BASE_BOUNDARY_LINE} />
					</Source>
					<NavigationControl position="top-right" showCompass={false} />
					<AttributionControl
						position="bottom-right"
						compact
						customAttribution="位置輪廓示意；未作價格插值"
					/>
				</MapLibreMap>
				<div className="seller-price-map__unavailable-panel" role="status">
					<strong>
						{reason ? "本期暫無空間資料" : "空間行情暫時無法顯示"}
					</strong>
					<p>
						{reason
							? reason.summary
							: "核准邊界、品類感知市場座標或等高線尚未取得；可能是服務未接通、資料未達品質門檻，或暫時載入失敗。地圖只顯示位置輪廓，不會用假資料填價。"}
					</p>
					{reason?.details.length ? (
						<ul className="seller-price-map__unavailable-reasons">
							{reason.details.map((detail) => (
								<li key={detail}>{detail}</li>
							))}
						</ul>
					) : null}
					<button type="button" onClick={onRetry}>
						重新載入空間資料
					</button>
				</div>
			</div>
			<p className="seller-price-map__accessibility">
				歷史行情、七日預測與市場比較仍可在其他分頁使用。
			</p>
		</div>
	);
}
