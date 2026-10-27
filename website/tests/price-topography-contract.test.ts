import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it } from "node:test";

const mapSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/TaiwanPriceMap.tsx",
	),
	"utf-8",
);
const panelSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/PriceTopographyPanel.tsx",
	),
	"utf-8",
);
const repoSource = readFileSync(
	join(import.meta.dirname, "../lib/pricebook/repo.ts"),
	"utf-8",
);
const spatialHookSource = readFileSync(
	join(import.meta.dirname, "../lib/hooks/useSpatialSurface.ts"),
	"utf-8",
);
const spatialMapDataSource = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/spatialMapData.ts",
	),
	"utf-8",
);
const reactMapLibrePatch = readFileSync(
	join(
		import.meta.dirname,
		"../../patches/@vis.gl__react-maplibre@8.1.2.patch",
	),
	"utf-8",
);
const workspaceSource = readFileSync(
	join(import.meta.dirname, "../../pnpm-workspace.yaml"),
	"utf-8",
);

describe("Price topography structural contract", () => {
	it("renders the original MapLibre engine with boundary, surface, contours and market layers", () => {
		assert.match(mapSource, /react-map-gl\/maplibre/);
		assert.match(mapSource, /id="taiwan-boundary"/);
		assert.match(mapSource, /id="price-surface"/);
		assert.match(mapSource, /id="price-contours"/);
		assert.match(mapSource, /id="market-points"/);
	});

	it("provides the MapLibre runtime up front to shorten asynchronous map initialization", () => {
		assert.match(mapSource, /import \* as maplibregl from "maplibre-gl"/);
		assert.equal((mapSource.match(/mapLib=\{maplibregl\}/g) ?? []).length, 3);
	});

	it("guards the MapLibre container after async resolution in every shipped entry point", () => {
		assert.match(
			workspaceSource,
			/'@vis\.gl\/react-maplibre@8\.1\.2': patches\/@vis\.gl__react-maplibre@8\.1\.2\.patch/,
		);
		for (const path of [
			"dist/components/map.js",
			"dist/index.cjs",
			"src/components/map.tsx",
		]) {
			assert.match(reactMapLibrePatch, new RegExp(`diff --git a/${path}`));
		}
		assert.equal(
			(
				reactMapLibrePatch.match(
					/^\+\s*const container = containerRef\.current;$/gm,
				) ?? []
			).length,
			3,
		);
		assert.equal(
			(
				reactMapLibrePatch.match(
					/^\+\s*if \(!isMounted \|\| !container\) \{$/gm,
				) ?? []
			).length,
			3,
		);
		assert.equal(
			(reactMapLibrePatch.match(/^\+.*reuse\(props, container\);$/gm) ?? [])
				.length,
			3,
		);
		assert.equal(
			(reactMapLibrePatch.match(/^\+.*props, container\);$/gm) ?? []).length,
			6,
		);
	});

	it("shows contour and market prices directly instead of hover-only values", () => {
		assert.match(mapSource, /seller-price-map__contour-label/);
		assert.match(mapSource, /seller-price-map__market-label/);
		assert.match(mapSource, /seller-price-map__region-label/);
		assert.match(mapSource, /行政區邊界價格/);
		assert.match(mapSource, /levelPriceTwdPerKg/);
		assert.match(mapSource, /estimatedP50TwdPerKg/);
	});

	it("uses one backend-owned adaptive scale for map colours, contours and legend", () => {
		assert.match(mapSource, /surface\.scale\.breaks/);
		assert.match(spatialMapDataSource, /priceScalePresentation/);
		assert.match(repoSource, /selection-p50-v1/);
		assert.match(mapSource, /adaptivePriceColorExpression\(/);
		assert.match(mapSource, /breaks\.forEach/);
		assert.doesNotMatch(mapSource, /breaks\.slice\(1\)/);
		assert.match(mapSource, /目前品項自適應 P50 色階/);
		assert.match(
			repoSource,
			/validateContourScale\(contourLinesGeojson, breaks\)/,
		);
		assert.match(
			panelSource,
			/priceScalePresentation\(data\.scale, data\.statistics\)/,
		);
		assert.match(panelSource, /scalePresentation\?\.kind !== "flat"/);
		assert.match(mapSource, /hasPriceScale && !isFlatScale/);
	});

	it("adapts colour thresholds and visible extrusion height without inventing either", () => {
		const heightStart = mapSource.indexOf("const surfaceHeight");
		const heightEnd = mapSource.indexOf("const boundaryPriceFill", heightStart);
		const heightLayer = mapSource.slice(heightStart, heightEnd);
		assert.match(spatialMapDataSource, /p50ScaleDomain\(/);
		assert.match(mapSource, /adaptivePriceColorExpression\(\s*breaks/);
		assert.match(mapSource, /featurePriceDomain\(\s*surfaceCells/);
		assert.match(heightLayer, /heightMinimum/);
		assert.match(heightLayer, /heightMaximum/);
		assert.doesNotMatch(heightLayer, /breaks\[0\]/);
		assert.match(mapSource, /顏色與等高線依目前品項及日期的 P50 尺度調整/);
	});

	it("refits a rendered-data viewport whenever the item, day or view mode changes", () => {
		assert.match(
			mapSource,
			/featureCollectionBounds\(surfaceCells, marketPoints\)/,
		);
		assert.match(mapSource, /map\.fitBounds\(bounds/);
		assert.match(mapSource, /onLoad=\{\(\) => setMapReady\(true\)\}/);
		assert.match(mapSource, /if \(selectedMarketLabel\)/);
		assert.match(mapSource, /map\.easeTo\(/);
		assert.match(panelSource, /spatialSelectionKey\(selection\)/);
		assert.match(
			panelSource,
			/key=\{`\$\{spatialSelectionKey\(selection\)\}:D\+\$\{data\.horizonDays\}`\}/,
		);
		assert.match(panelSource, /viewIdentity=/);
	});

	it("keeps an exact-value table and honest points/table fallbacks", () => {
		assert.match(panelSource, /D\+\$\{data\.horizonDays\} 市場價格明細/);
		assert.match(panelSource, /data\.displayMode === "table_only"/);
		assert.match(mapSource, /位置輪廓，不會用假資料填價/);
		assert.match(panelSource, /空白區域表示支撐不足，不代表價格為零/);
		assert.match(panelSource, /disabled=\{!canContour\}/);
		assert.match(panelSource, /disabled=\{!canHeight\}/);
		assert.match(panelSource, /aria-pressed=\{effectiveMode === "contour"\}/);
		assert.match(panelSource, /左右滑動可查看\s*P10、P50、P90/);
	});

	it("calls the Seller BFF and documents that client-side interpolation is forbidden", () => {
		assert.match(repoSource, /\/sellers\/pricebook\/spatial-surface/);
		assert.match(repoSource, /never be\n \* guessed from marketComparison/);
	});

	it("refetches the backend-owned scale by exact selection and horizon", () => {
		assert.match(spatialHookSource, /spatialSelectionKey\(selection\)/);
		assert.match(spatialHookSource, /horizonDays/);
		assert.match(
			spatialHookSource,
			/fetchSpatialSurface\(selection, horizonDays, signal\)/,
		);
		assert.match(spatialHookSource, /staleTime:\s*60_000/);
		assert.doesNotMatch(spatialHookSource, /placeholderData|keepPreviousData/);
	});

	it("keeps map, rail and table selection synchronized with one effective market code", () => {
		assert.match(panelSource, /const effectiveSelectedMarketCode/);
		assert.doesNotMatch(
			panelSource,
			/find\(\(market\) => market\.usedForSurface\)/,
		);
		assert.match(
			panelSource,
			/selectedMarketCode=\{effectiveSelectedMarketCode\}/,
		);
		assert.ok(
			(
				panelSource.match(
					/effectiveSelectedMarketCode === market\.marketCode/g,
				) ?? []
			).length >= 3,
		);
		assert.match(mapSource, /aria-pressed=\{selected\}/);
		assert.match(panelSource, /<th scope="row">/);
	});

	it("draws administrative boundary lines above the surface and contours but below market points", () => {
		const surfaceIndex = mapSource.indexOf('id="price-surface"');
		const contourIndex = mapSource.indexOf('id="price-contours"');
		const boundaryLineIndex = mapSource.indexOf(
			"<Layer {...boundaryPriceLine} />",
		);
		const marketIndex = mapSource.indexOf('id="market-points"');
		assert.ok(surfaceIndex >= 0);
		assert.ok(contourIndex > surfaceIndex);
		assert.ok(boundaryLineIndex > contourIndex);
		assert.ok(marketIndex > boundaryLineIndex);
		assert.match(mapSource, /hoverDetails\(event, true\)/);
	});

	it("keeps the unselected map focused on one action and uses an honest generic failure state", () => {
		const previewStart = panelSource.indexOf(
			"export function PriceTopographyPreview",
		);
		const previewEnd = panelSource.indexOf(
			"export default function PriceTopographyPanel",
		);
		const previewSource = panelSource.slice(previewStart, previewEnd);
		assert.match(previewSource, /<TaiwanPriceMapPreview \/>/);
		assert.match(previewSource, /onClick=\{onRequestSearch\}/);
		assert.doesNotMatch(
			previewSource,
			/<aside|map-layer-list|MapLibre|Pricebook/,
		);
		assert.match(mapSource, /空間行情暫時無法顯示/);
		assert.doesNotMatch(mapSource, /目前 BFF\s*尚未提供/);
	});
});
