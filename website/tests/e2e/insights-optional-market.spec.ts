import {
	expect,
	type JSHandle,
	type Locator,
	type Page,
	type Route,
	test,
} from "@playwright/test";
import type { Map as NativeMap } from "maplibre-gl";
import {
	mockMyKyc,
	mockMyStore,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

interface PricebookRequest {
	cropCode: string | null;
	groupCode: string | null;
	marketCode: string | null;
	history: string | null;
	horizonDays: string | null;
}

interface PricebookRequests {
	overview: Array<{
		scope: string | null;
		windowDays: string | null;
		marketCode: string | null;
		query: string | null;
	}>;
	watchAdds: Array<{ cropCode: string; marketCode: string | null }>;
	watchDeletes: string[];
	observed: PricebookRequest[];
	forecast: PricebookRequest[];
	spatial: PricebookRequest[];
}

interface InsightsFixtureOptions {
	administrativeBoundaries?: boolean;
	observedGate?: { allowSuccess: boolean };
	comparisonFailure?: boolean;
	emptyRadar?: boolean;
	unavailableWatch?: boolean;
	overviewFailure?: { scope: string; active: boolean };
	mutationFailure?: { method: "POST" | "DELETE"; active: boolean };
	beforeOverview?: (query: string | null) => Promise<void>;
	beforeCreate?: () => Promise<void>;
}

const CATEGORY_TREE = [
	{
		id: 1,
		name: "生鮮蔬果",
		children: [
			{
				id: 11,
				name: "茄果類",
				children: [
					{ id: 111, name: "番茄", children: [] },
					{ id: 112, name: "甜椒", children: [] },
					{ id: 113, name: "包心白-包白", children: [] },
				],
			},
		],
	},
];

// categoryId → 後端權威解析結果（issue #2786：category-resolution 取代前端
// exact-match 猜測，這裡直接固定唯一 canonical 對應，不再模擬「命中多筆再挑
// 一筆」的候選陣列）。每個 L3 分類一律解析到 Pricebook 大類（group），後端
// 不再回傳 crop-level 精度——CategoryResolutionData 本身也沒有
// selectionKind/cropCode 欄位，故這裡直接用 groupCode 表示。
const CATEGORY_RESOLUTIONS: Record<
	string,
	{ groupCode: string; label: string }
> = {
	"111": { groupCode: "GRP-111", label: "番茄" },
	"112": { groupCode: "GRP-112", label: "甜椒" },
	"113": { groupCode: "GRP-113", label: "包心白-包白" },
};

// categoryId → GetCategoryPriceRollup 快照，涵蓋 L1/L2 部分選取（尚未選到 L3）
// 時的簡化彙整卡；未列在這裡的 categoryId 一律回 unavailable。
const CATEGORY_ROLLUPS: Record<
	string,
	{
		level: number;
		latestPrice: string;
		prevPrice: string;
		points: Array<{ tradeDate: string; avgPrice: string }>;
		sourceGroupCount: number;
		forecastPoints: Array<{
			horizonDays: number;
			targetDate: string;
			status: string;
			p10: string;
			p50: string;
			p90: string;
		}>;
	}
> = {
	"1": {
		level: 1,
		latestPrice: "38.5",
		prevPrice: "40",
		points: [
			{ tradeDate: "2026-08-29", avgPrice: "40" },
			{ tradeDate: "2026-08-30", avgPrice: "38.5" },
		],
		sourceGroupCount: 9,
		forecastPoints: [
			{
				horizonDays: 1,
				targetDate: "2026-09-01",
				status: "ready",
				p10: "35",
				p50: "39",
				p90: "43",
			},
		],
	},
	"11": {
		level: 2,
		latestPrice: "45.2",
		prevPrice: "40",
		points: [
			{ tradeDate: "2026-08-29", avgPrice: "40" },
			{ tradeDate: "2026-08-30", avgPrice: "45.2" },
		],
		sourceGroupCount: 3,
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
};

const RADAR_MARKET_OPTIONS = [
	{ marketCode: "109", marketName: "臺北一" },
	{ marketCode: "400", marketName: "臺中市場" },
] as const;

const RADAR_ITEMS = [
	{
		watchTargetId: "8001",
		cropCode: "LA1",
		cropName: "甘藍",
		variantName: "初秋",
		isWatched: true,
		status: "ready",
		freshnessStatus: "fresh",
		qualityReasonCodes: [],
		currency: "TWD",
		unit: "kg",
		sourceDataAsOf: "2026-08-25T15:59:59Z",
		latestActual: {
			tradeDate: "2026-08-25",
			avgPrice: "61.5",
			volumeKg: "1200",
		},
		previousActual: { tradeDate: "2026-08-24", avgPrice: "58.2" },
		change: {
			absolute: "3.3",
			percent: "5.7",
			direction: "up",
			basis: "latest_vs_previous_observed_trade_date",
		},
		points: [
			{ tradeDate: "2026-08-20", avgPrice: "57.1" },
			{ tradeDate: "2026-08-21", avgPrice: "56.4" },
			{ tradeDate: "2026-08-22", avgPrice: "58.9" },
			{ tradeDate: "2026-08-24", avgPrice: "58.2" },
			{ tradeDate: "2026-08-25", avgPrice: "61.5" },
		],
	},
	{
		cropCode: "A1",
		cropName: "番茄",
		variantName: "牛番茄",
		isWatched: false,
		status: "ready",
		freshnessStatus: "fresh",
		qualityReasonCodes: ["VOLUME_WEIGHT_FALLBACK"],
		currency: "TWD",
		unit: "kg",
		sourceDataAsOf: "2026-08-25T15:59:59Z",
		latestActual: { tradeDate: "2026-08-25", avgPrice: "35.8", volumeKg: "0" },
		previousActual: { tradeDate: "2026-08-23", avgPrice: "35.8" },
		change: {
			absolute: "0",
			percent: "0",
			direction: "flat",
			basis: "latest_vs_previous_observed_trade_date",
		},
		points: [
			{ tradeDate: "2026-08-19", avgPrice: "35.9" },
			{ tradeDate: "2026-08-23", avgPrice: "35.8" },
			{ tradeDate: "2026-08-25", avgPrice: "35.8" },
		],
	},
] as const;

function fulfillData(route: Route, data: unknown) {
	return route.fulfill({
		status: 200,
		contentType: "application/json",
		body: JSON.stringify({ code: 0, data }),
	});
}

function requestQuery(route: Route): PricebookRequest {
	const params = new URL(route.request().url()).searchParams;
	return {
		cropCode: params.get("cropCode"),
		groupCode: params.get("groupCode"),
		marketCode: params.get("marketCode"),
		history: params.get("history"),
		horizonDays: params.get("horizonDays"),
	};
}

function forecastData(
	identity: string,
	marketCode: string | null,
	kind: "crop" | "group",
) {
	const offset = identity === "GRP-112" ? 8 : 0;
	return {
		status: "ready",
		...(kind === "group"
			? { resolvedCropGroupCode: identity }
			: { resolvedCropCode: identity }),
		...(marketCode ? { marketCode } : {}),
		modelVersion: "e2e-fixture-v1",
		points: Array.from({ length: 7 }, (_, index) => ({
			targetDate: `2026-09-${String(index + 1).padStart(2, "0")}`,
			horizonDays: index + 1,
			status: "ready",
			p10: 48 + offset + index,
			p50: 58 + offset + index,
			p90: 68 + offset + index,
		})),
		marketComparison: [
			{
				marketCode: "109",
				marketName: "臺北一",
				status: "ready",
				p50: 62 + offset,
				isApproximate: kind === "group",
			},
			{
				marketCode: "400",
				marketName: "臺中市場",
				status: "ready",
				p50: 55 + offset,
				isApproximate: kind === "group",
			},
		],
	};
}

function spatialData(
	identity: string,
	horizonDays: number,
	kind: "crop" | "group",
) {
	const isPepper = identity === "GRP-112";
	const prices = isPepper ? [100, 150, 210] : [40, 65, 90];
	const breaks = isPepper ? [120, 150, 190] : [50, 65, 80];
	const markets = [
		{ marketCode: "109", marketName: "臺北一", coordinates: [121.53, 25.05] },
		{ marketCode: "400", marketName: "臺中市場", coordinates: [120.67, 24.15] },
		{ marketCode: "800", marketName: "高雄市場", coordinates: [120.3, 22.63] },
	] as const;
	return {
		status: "ready",
		displayMode: "points_only",
		selectionKind: kind,
		targetDate: `2026-09-${String(horizonDays).padStart(2, "0")}`,
		horizonDays,
		currency: "TWD",
		unit: "kg",
		scale: {
			scaleId: isPepper ? "adaptive-b1" : "adaptive-a1",
			policyVersion: "selection-p50-v1",
			breaks,
		},
		boundary: {
			version: "e2e-boundary-v1",
			digest: "e2e-boundary",
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
			features: markets.map((market, index) => ({
				type: "Feature",
				properties: {
					marketCode: market.marketCode,
					marketName: market.marketName,
					p50: prices[index],
				},
				geometry: { type: "Point", coordinates: market.coordinates },
			})),
		},
		surfaceCellsGeojson: { type: "FeatureCollection", features: [] },
		contourLinesGeojson: { type: "FeatureCollection", features: [] },
		marketPrices: markets.map((market, index) => ({
			marketCode: market.marketCode,
			marketName: market.marketName,
			categoryCode: "N04",
			predictionStatus: "ready",
			marketStatus: "open",
			p10: null,
			p50: prices[index],
			p90: null,
			recentVolumeKg: null,
			usedForSurface: true,
		})),
		statistics: {
			p10: null,
			p50: prices[1],
			p90: null,
			minimumP50: prices[0],
			maximumP50: prices[2],
			pricedMarketCount: 3,
			closedMarketCount: 0,
			totalMarketCount: 3,
			recentVolumeKg: null,
			surfaceCellCount: 0,
			contourSegmentCount: 0,
		},
		qualityReasonCodes: ["insufficient_spatial_coverage"],
		disclosures: ["wholesale_market_location_not_production_origin"],
		supportedLandAreaRatio: null,
		excludedMarketCodes: [],
		lineage: {
			artifactId: isPepper ? "surface-b1" : "surface-a1",
			modelVersion: "e2e-spatial-v1",
			datasetDigest: isPepper ? "dataset-b1" : "dataset-a1",
			boundaryDigest: "e2e-boundary",
		},
	};
}

function administrativeBoundaryData(
	identity: string,
	horizonDays: number,
	kind: "crop" | "group",
) {
	const data = spatialData(identity, horizonDays, kind);
	const polygon = (
		properties: Record<string, unknown>,
		[west, south, east, north]: [number, number, number, number],
	) => ({
		type: "Feature" as const,
		properties,
		geometry: {
			type: "Polygon" as const,
			coordinates: [
				[
					[west, south],
					[east, south],
					[east, north],
					[west, north],
					[west, south],
				],
			],
		},
	});
	// Deliberately separate three contracts: an estimate, unpriced summary
	// metadata, and a raw administrative shape without any price-summary record.
	const priced = polygon(
		{
			regionCode: "priced-region",
			regionName: "臺中市",
			estimatedP50TwdPerKg: 37,
			bandLowerTwdPerKg: 30,
			bandUpperTwdPerKg: 45,
			supportingMarketCount: 6,
			status: "approximate",
		},
		[120.5, 23.65, 121.05, 24.35],
	);
	const unpriced = polygon(
		{
			regionCode: "unpriced-region",
			regionName: "宜蘭縣",
			estimatedP50TwdPerKg: null,
			bandLowerTwdPerKg: 32,
			bandUpperTwdPerKg: 48,
			supportingMarketCount: 0,
			status: "insufficient_support",
		},
		[121.15, 24.35, 121.65, 24.9],
	);
	const rawOnly = polygon(
		{
			COUNTYCODE: "10015",
			COUNTYNAME: "花蓮縣",
			TOWNCODE: "10015060",
			TOWNNAME: "壽豐鄉",
		},
		[121.15, 23.4, 121.65, 23.95],
	);
	return {
		...data,
		boundary: {
			...data.boundary,
			geojson: {
				type: "FeatureCollection",
				features: [
					{
						...priced,
						properties: { regionCode: "priced-region", regionName: "臺中市" },
					},
					{
						...unpriced,
						properties: {
							regionCode: "unpriced-region",
							regionName: "宜蘭縣",
						},
					},
					rawOnly,
				],
			},
		},
		boundaryPriceGeojson: {
			type: "FeatureCollection",
			features: [priced, unpriced],
		},
	};
}

async function prepareInsightsFixture(
	page: Page,
	baseURL: string,
	options: InsightsFixtureOptions = {},
): Promise<PricebookRequests> {
	const requests: PricebookRequests = {
		overview: [],
		watchAdds: [],
		watchDeletes: [],
		observed: [],
		forecast: [],
		spatial: [],
	};
	const watchedTargets = new Map<string, string>([["LA1:", "8001"]]);
	if (options.emptyRadar) watchedTargets.clear();
	const targetKey = (cropCode: string, marketCode?: string | null) =>
		`${cropCode}:${marketCode ?? ""}`;
	// Lowest-priority safety net for seller-shell requests unrelated to Pricebook.
	await page.route("**/api/v1/**", (route) => fulfillData(route, {}));
	await setupGatewayMocks(page, {
		identity: {
			accountId: "9101",
			roles: ["seller"],
		},
		session: "identity",
	});

	// Playwright resolves the last-added matching route first, so these focused
	// handlers override both catch-alls above.
	await mockMyStore(page, { id: "9101", storeName: "行情測試商店" });
	await mockMyKyc(page, { status: "APPROVED" });
	await page.route("**/api/v1/public/categories/tree", (route) =>
		fulfillData(route, CATEGORY_TREE),
	);
	await page.route(
		"**/api/v1/app/sellers/pricebook/category-resolution**",
		(route) => {
			const categoryId = new URL(route.request().url()).searchParams.get(
				"category_id",
			);
			const resolution = categoryId
				? CATEGORY_RESOLUTIONS[categoryId]
				: undefined;
			if (!resolution) {
				return fulfillData(route, {
					categoryId: categoryId ?? "",
					status: "unmapped",
					mappingVersion: "e2e-v1",
				});
			}
			return fulfillData(route, {
				categoryId,
				status: "resolved",
				...resolution,
				mappingVersion: "e2e-v1",
			});
		},
	);
	await page.route(
		"**/api/v1/app/sellers/pricebook/category-price-rollup**",
		(route) => {
			const categoryId = new URL(route.request().url()).searchParams.get(
				"category_id",
			);
			const rollup = categoryId ? CATEGORY_ROLLUPS[categoryId] : undefined;
			if (!rollup) {
				return fulfillData(route, {
					categoryId: categoryId ?? "",
					level: 0,
					status: "unavailable",
					points: [],
					sourceGroupCount: 0,
					computedAt: "2026-08-30T21:15:03Z",
				});
			}
			return fulfillData(route, {
				categoryId,
				status: "ready",
				computedAt: "2026-08-30T21:15:03Z",
				...rollup,
			});
		},
	);
	await page.route(
		"**/api/v1/app/sellers/pricebook/overview**",
		async (route) => {
			const params = new URL(route.request().url()).searchParams;
			const scope = params.get("scope");
			const windowDays = params.get("window_days");
			const marketCode = params.get("market_code");
			if (
				params.has("windowDays") ||
				params.has("marketCode") ||
				params.has("pageSize") ||
				!params.has("page_size") ||
				!params.has("page") ||
				(scope !== "all" && scope !== "watchlist") ||
				!windowDays ||
				(scope === "watchlist" && marketCode != null)
			) {
				return route.fulfill({ status: 400, body: "invalid overview query" });
			}
			const query = params.get("query");
			requests.overview.push({ scope, windowDays, marketCode, query });
			await options.beforeOverview?.(query);
			if (
				options.overviewFailure?.active &&
				options.overviewFailure.scope === scope
			) {
				return route.fulfill({
					status: 503,
					json: { code: 503, message: "行情服務暫時無法使用" },
				});
			}
			// #2784: discovery cards all use the selected market scope; watchlist
			// cards retain the identity stored by POST, independent of that filter.
			const targets =
				scope === "watchlist"
					? [...watchedTargets.keys()].reverse().flatMap((key) => {
							const [cropCode, savedMarketCode] = key.split(":");
							const item = RADAR_ITEMS.find(
								(entry) => entry.cropCode === cropCode,
							);
							return item
								? [{ item, itemMarketCode: savedMarketCode || undefined }]
								: [];
						})
					: [...RADAR_ITEMS]
							.sort((a, b) => a.cropCode.localeCompare(b.cropCode))
							.map((item) => ({
								item,
								itemMarketCode: marketCode ?? undefined,
							}));
			const resolvedItems = targets.map(({ item, itemMarketCode }) => {
				const key = targetKey(item.cropCode, itemMarketCode);
				const watchTargetId = watchedTargets.get(key);
				return {
					...item,
					...(options.unavailableWatch && item.cropCode === "LA1"
						? {
								status: "unavailable",
								freshnessStatus: "unavailable",
								qualityReasonCodes: [],
								sourceDataAsOf: undefined,
								latestActual: undefined,
								previousActual: undefined,
								change: undefined,
								points: [],
							}
						: {}),
					marketCode: itemMarketCode,
					marketName: RADAR_MARKET_OPTIONS.find(
						(market) => market.marketCode === itemMarketCode,
					)?.marketName,
					isWatched: watchTargetId != null,
					watchTargetId,
				};
			});
			const filteredItems = query
				? resolvedItems.filter(
						(item) =>
							item.cropName.includes(query) || item.cropCode.includes(query),
					)
				: resolvedItems;
			const items = options.emptyRadar
				? []
				: scope === "watchlist"
					? filteredItems.filter((item) => item.isWatched)
					: filteredItems;
			return fulfillData(route, {
				snapshotId: `radar-${scope ?? "unknown"}-${windowDays ?? "unknown"}`,
				source: "MOA_WHOLESALE_DAILY",
				generatedAt: "2026-08-26T02:30:00Z",
				sourceDataAsOf: items
					.flatMap((item) => (item.sourceDataAsOf ? [item.sourceDataAsOf] : []))
					.sort()
					.at(-1),
				scope,
				windowDays,
				marketCode: marketCode ?? undefined,
				currency: "TWD",
				unit: "kg",
				marketOptions: RADAR_MARKET_OPTIONS,
				items,
				page: "1",
				pageSize: "20",
				totalItems: String(items.length),
				hasNext: false,
			});
		},
	);
	await page.route(
		"**/api/v1/app/sellers/pricebook/watch-targets",
		async (route) => {
			if (route.request().method() !== "POST") return route.fallback();
			const body = route.request().postDataJSON() as {
				cropCode?: unknown;
				marketCode?: unknown;
			};
			if (
				typeof body.cropCode !== "string" ||
				(body.marketCode !== undefined &&
					typeof body.marketCode !== "string") ||
				Object.keys(body).some(
					(key) => key !== "cropCode" && key !== "marketCode",
				)
			) {
				return route.fulfill({ status: 400, body: "invalid cropCode" });
			}
			const marketCode =
				typeof body.marketCode === "string" ? body.marketCode : null;
			requests.watchAdds.push({ cropCode: body.cropCode, marketCode });
			await options.beforeCreate?.();
			if (
				options.mutationFailure?.active &&
				options.mutationFailure.method === "POST"
			) {
				return route.fulfill({
					status: 503,
					json: { code: 503, message: "追蹤服務暫時無法使用" },
				});
			}
			const key = targetKey(body.cropCode, marketCode);
			const existingId = watchedTargets.get(key);
			const watchTargetId = existingId ?? "8002";
			watchedTargets.set(key, watchTargetId);
			return fulfillData(route, {
				watchTargetId,
				created: existingId == null,
			});
		},
	);
	await page.route(
		"**/api/v1/app/sellers/pricebook/watch-targets/*",
		(route) => {
			if (route.request().method() !== "DELETE") return route.fallback();
			const watchTargetId = decodeURIComponent(
				new URL(route.request().url()).pathname.split("/").at(-1) ?? "",
			);
			requests.watchDeletes.push(watchTargetId);
			if (
				options.mutationFailure?.active &&
				options.mutationFailure.method === "DELETE"
			) {
				return route.fulfill({
					status: 503,
					json: { code: 503, message: "追蹤服務暫時無法使用" },
				});
			}
			for (const [key, id] of watchedTargets) {
				if (id === watchTargetId) watchedTargets.delete(key);
			}
			return fulfillData(route, {});
		},
	);
	await page.route(
		"**/api/v1/app/sellers/pricebook/observed-price**",
		(route) => {
			const query = requestQuery(route);
			requests.observed.push(query);
			if (options.observedGate && !options.observedGate.allowSuccess) {
				return route.fulfill({
					status: 503,
					contentType: "application/json",
					body: JSON.stringify({
						code: 503,
						message: "官方行情暫時無法讀取",
					}),
				});
			}
			const identity = query.groupCode ?? query.cropCode ?? "";
			return fulfillData(route, {
				observations: [
					{
						marketCode: query.marketCode ?? "",
						cropCode: identity,
						tradeDate: "2026-08-25",
						avgPrice: identity === "GRP-112" ? 66 : 61.5,
						volumeKg: 1200,
					},
				],
			});
		},
	);
	await page.route(
		"**/api/v1/app/sellers/pricebook/forecast-curve**",
		(route) => {
			const query = requestQuery(route);
			requests.forecast.push(query);
			const identity = query.groupCode ?? query.cropCode ?? "";
			const kind: "crop" | "group" = query.groupCode ? "group" : "crop";
			const data = forecastData(identity, query.marketCode, kind);
			return fulfillData(route, {
				...data,
				marketComparison: data.marketComparison.map((entry) =>
					options.comparisonFailure && entry.marketCode === "400"
						? { ...entry, status: "unavailable", p50: null }
						: entry,
				),
			});
		},
	);
	await page.route(
		"**/api/v1/app/sellers/pricebook/spatial-surface**",
		(route) => {
			const query = requestQuery(route);
			requests.spatial.push(query);
			const identity = query.groupCode ?? query.cropCode ?? "";
			const kind: "crop" | "group" = query.groupCode ? "group" : "crop";
			return fulfillData(
				route,
				(options.administrativeBoundaries
					? administrativeBoundaryData
					: spatialData)(identity, Number(query.horizonDays ?? "1"), kind),
			);
		},
	);

	await page
		.context()
		.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
	await seedStoredActiveRole(page, "seller");
	await page.addInitScript(() => {
		sessionStorage.setItem("hawkr:initial-loader-seen", "1");
	});

	return requests;
}

async function chooseDropdownOption(
	trigger: Locator,
	page: Page,
	optionName: string,
) {
	await trigger.click();
	await page.getByRole("option", { name: optionName, exact: true }).click();
}

async function chooseTaipeiMarketWithKeyboard(trigger: Locator, page: Page) {
	await trigger.focus();
	await page.keyboard.press("Enter");
	const target = page.getByRole("option", { name: "臺北一", exact: true });
	await expect(target).toBeVisible();
	const listbox = page.getByRole("listbox");
	await expect(listbox).toBeFocused();
	const options = page.getByRole("option");
	const targetIndex = await options.evaluateAll((nodes) =>
		nodes.findIndex((node) => node.textContent?.trim() === "臺北一"),
	);
	expect(targetIndex).toBeGreaterThanOrEqual(0);
	const targetId = await target.getAttribute("id");
	if (!targetId) throw new Error("臺北一 option must expose a stable id");
	// Exercise the focused listbox with stable keyboard navigation while allowing
	// the product API to return markets in any legitimate display order. Wait for
	// React to commit the active descendant before Enter; otherwise a loaded mobile
	// runner can submit the previously selected row while ArrowDown is still pending.
	await page.keyboard.press("Home");
	for (let index = 0; index < targetIndex; index += 1) {
		await page.keyboard.press("ArrowDown");
	}
	await expect(listbox).toHaveAttribute("aria-activedescendant", targetId);
	await page.keyboard.press("Enter");
	await expect(trigger).toContainText("臺北一");
}

function hasRequest(
	requests: readonly PricebookRequest[],
	identity: string,
	marketCode: string | null,
) {
	return requests.some(
		(request) =>
			(request.cropCode === identity || request.groupCode === identity) &&
			request.marketCode === marketCode,
	);
}

async function expectNoHorizontalOverflow(page: Page) {
	await expect
		.poll(() =>
			page.evaluate(() => {
				const viewportWidth = document.documentElement.clientWidth;
				return (
					Math.max(
						document.documentElement.scrollWidth,
						document.body.scrollWidth,
					) - viewportWidth
				);
			}),
		)
		.toBeLessThanOrEqual(1);
}

/** Read the existing react-map-gl ref only to project fixture coordinates.
 * The test still uses a real canvas click: no map.fire or product test hooks. */
function nativePriceMap(container: Element): NativeMap | null {
	interface Hook {
		memoizedState?: {
			current?: { map?: { getMap?: () => NativeMap } };
		};
		next?: Hook;
	}
	interface Fiber {
		return?: Fiber;
		alternate?: Fiber;
		memoizedState?: Hook;
	}
	const key = Object.keys(container).find((name) =>
		name.startsWith("__reactFiber$"),
	);
	let fiber = key
		? (container as unknown as Record<string, Fiber>)[key]
		: undefined;
	const canvas = container.querySelector("canvas");
	for (let depth = 0; fiber && depth < 30; depth++, fiber = fiber.return) {
		for (const candidate of [fiber, fiber.alternate]) {
			let hook = candidate?.memoizedState;
			for (let count = 0; hook && count < 30; count++, hook = hook.next) {
				const ref = hook.memoizedState?.current?.map;
				if (typeof ref?.getMap !== "function") continue;
				const map = ref.getMap();
				if (map.getCanvas() === canvas && map.isStyleLoaded()) return map;
			}
		}
	}
	return null;
}

async function clickAdministrativeOutline(
	page: Page,
	map: JSHandle<NativeMap | null>,
	coordinates: [number, number],
	isMobile: boolean,
) {
	const canvas = page.locator(".seller-price-map__canvas canvas");
	await canvas.scrollIntoViewIfNeeded();
	await expect
		.poll(() => map.evaluate((instance) => instance?.isMoving() === false))
		.toBe(true);
	const point = await map.evaluate((instance, location) => {
		if (!instance) throw new Error("Price map was not initialized");
		const position = instance.project(location);
		const features = instance.queryRenderedFeatures(position);
		return {
			x: position.x,
			y: position.y,
			// This must hit a drawn administrative outline, not a DOM price label
			// or the interior of a polygon, before the real browser click occurs.
			outlined: features.some(
				(feature) =>
					feature.layer.type === "line" &&
					["taiwan-boundary", "boundary-prices"].includes(feature.source),
			),
		};
	}, coordinates);
	expect(point.outlined, "fixture coordinate must hit a rendered outline").toBe(
		true,
	);
	const position = { x: point.x, y: point.y };
	if (isMobile) await canvas.tap({ position });
	else await canvas.click({ position });
}

test.describe("市場雷達：失敗恢復與請求隔離", { tag: ["@ui"] }, () => {
	const discovery = (page: Page) =>
		page.getByRole("region", { name: "全部官方行情" });
	const watchlist = (page: Page) =>
		page.getByRole("region", { name: "我的追蹤" });
	const tomato = "番茄・牛番茄（全部市場，A1）";
	const cabbage = "甘藍・初秋（全部市場，LA1）";
	async function open(
		page: Page,
		baseURL: string | undefined,
		options: InsightsFixtureOptions = {},
	) {
		if (!baseURL) throw new Error("Playwright baseURL is required");
		const requests = await prepareInsightsFixture(page, baseURL, options);
		await page.goto("/seller-center/insights", {
			waitUntil: "domcontentloaded",
		});
		await expect(page.getByRole("heading", { name: "市場雷達" })).toBeVisible();
		return requests;
	}

	test("雙 scope 空態不補價格，瀏覽 CTA 聚焦搜尋", async ({
		page,
		baseURL,
	}) => {
		await open(page, baseURL, { emptyRadar: true });
		await expect(watchlist(page)).toContainText("還沒有追蹤品相");
		await expect(discovery(page)).toContainText("目前沒有可顯示的官方行情");
		await expect(page.locator(".market-radar__row")).toHaveCount(0);
		await expect(page.locator(".market-radar")).not.toContainText("NT$ 0");
		await watchlist(page)
			.getByRole("button", { name: "瀏覽可追蹤品相" })
			.click();
		await expect(discovery(page).getByRole("searchbox")).toBeFocused();
	});

	test("搜尋無結果與清空恢復，不影響既有追蹤", async ({ page, baseURL }) => {
		const requests = await open(page, baseURL);
		const search = discovery(page).getByRole("searchbox");
		await search.fill("不存在品相");
		await expect(discovery(page)).toContainText("找不到符合的官方行情");
		await expect(
			watchlist(page).getByRole("button", { name: `停止追蹤${cabbage}` }),
		).toBeVisible();
		await search.fill("");
		await expect(discovery(page).locator(".market-radar__row")).toHaveCount(2);
		expect(
			requests.overview
				.filter((r) => r.scope === "watchlist")
				.every((r) => r.query == null),
		).toBe(true);
	});

	for (const scope of ["all", "watchlist"] as const) {
		test(`${scope} 初次失敗局部重試，另一 scope 保持可用`, async ({
			page,
			baseURL,
		}) => {
			const gate = { scope, active: true };
			const requests = await open(page, baseURL, { overviewFailure: gate });
			const failed = scope === "all" ? discovery(page) : watchlist(page);
			const healthy = scope === "all" ? watchlist(page) : discovery(page);
			await expect(
				failed.getByRole("button", { name: "重新載入" }),
			).toBeVisible();
			await expect(healthy.locator(".market-radar__row").first()).toBeVisible();
			const prior = requests.overview.filter((r) => r.scope === scope).length;
			gate.active = false;
			await failed.getByRole("button", { name: "重新載入" }).click();
			await expect(failed.locator(".market-radar__row").first()).toBeVisible();
			await expect(
				failed.getByRole("button", { name: "重新載入" }),
			).toHaveCount(0);
			expect(
				requests.overview.filter((r) => r.scope === scope).length,
			).toBeGreaterThan(prior);
		});
	}

	for (const method of ["POST", "DELETE"] as const) {
		test(`${method} 失敗還原兩個 scope 的追蹤狀態，使用者可重試`, async ({
			page,
			baseURL,
		}) => {
			const gate = { method, active: true };
			let pauseRefresh = false;
			let release = () => {};
			const refreshGate = new Promise<void>((resolve) => {
				release = resolve;
			});
			const requests = await open(page, baseURL, {
				mutationFailure: gate,
				beforeOverview: async () => {
					if (pauseRefresh) await refreshGate;
				},
			});
			await expect(watchlist(page).locator(".market-radar__row")).toHaveCount(
				1,
			);
			const label = method === "POST" ? tomato : cabbage;
			const original = `${method === "POST" ? "追蹤" : "停止追蹤"}${label}`;
			pauseRefresh = true;
			try {
				await discovery(page)
					.getByRole("button", { name: original, exact: true })
					.click();
				await expect(
					page.getByText(method === "POST" ? "無法加入追蹤" : "無法停止追蹤", {
						exact: true,
					}),
				).toBeVisible();
				await expect(
					discovery(page).getByRole("button", { name: original, exact: true }),
				).toBeEnabled();
				await expect(
					watchlist(page).getByRole("button", {
						name: `停止追蹤${label}`,
						exact: true,
					}),
				).toHaveCount(method === "POST" ? 0 : 1);
			} finally {
				pauseRefresh = false;
				release();
			}
			gate.active = false;
			await discovery(page)
				.getByRole("button", { name: original, exact: true })
				.click();
			await expect(
				discovery(page).getByRole("button", {
					name: `${method === "POST" ? "停止追蹤" : "追蹤"}${label}`,
					exact: true,
				}),
			).toBeEnabled();
			await expect(
				watchlist(page).getByRole("button", {
					name: `停止追蹤${label}`,
					exact: true,
				}),
			).toHaveCount(method === "POST" ? 1 : 0);
			expect(
				method === "POST"
					? requests.watchAdds.length
					: requests.watchDeletes.length,
			).toBe(2);
		});
	}

	test("unavailable 追蹤缺值不補零，仍以保存 ID 刪除", async ({
		page,
		baseURL,
	}) => {
		const requests = await open(page, baseURL, { unavailableWatch: true });
		await expect(watchlist(page)).toContainText("暫無官方成交資料");
		await expect(
			watchlist(page).locator(".market-radar__latest strong"),
		).toHaveText("—");
		await expect(watchlist(page)).not.toContainText("NT$ 0");
		await watchlist(page)
			.getByRole("button", { name: `停止追蹤${cabbage}` })
			.click();
		await expect(watchlist(page)).toContainText("還沒有追蹤品相");
		await expect.poll(() => requests.watchDeletes).toEqual(["8001"]);
	});

	test("30 天與搜尋市場交叉切換，watchlist 不帶 discovery 篩選", async ({
		page,
		baseURL,
	}) => {
		const requests = await open(page, baseURL);
		await expect(
			page.getByRole("button", { name: "近 7 天", exact: true }),
		).toHaveAttribute("aria-pressed", "true");
		await expect(
			discovery(page).locator(".market-radar__column-head"),
		).toContainText("近 7 天");
		await discovery(page).getByRole("searchbox").fill("  番茄  ");
		await expect(discovery(page).locator(".market-radar__row")).toHaveCount(1);
		await chooseTaipeiMarketWithKeyboard(
			page.getByRole("button", { name: "市場範圍" }),
			page,
		);
		await page.getByRole("button", { name: "近 30 天", exact: true }).click();
		await expect
			.poll(() => requests.overview)
			.toContainEqual({
				scope: "all",
				windowDays: "30",
				marketCode: "109",
				query: "番茄",
			});
		await expect
			.poll(() => requests.overview)
			.toContainEqual({
				scope: "watchlist",
				windowDays: "30",
				marketCode: null,
				query: null,
			});
		await expect(
			discovery(page).locator(".market-radar__column-head"),
		).toContainText("近 30 天");
		await expect(
			watchlist(page).getByRole("img", {
				name: `${cabbage}近 30 天內有成交資料的官方平均價走勢`,
				exact: true,
			}),
		).toBeVisible();
		await expect(
			watchlist(page).getByRole("button", { name: `停止追蹤${cabbage}` }),
		).toBeVisible();
		await expect(
			discovery(page).getByRole("button", {
				name: "追蹤番茄・牛番茄（臺北一，A1）",
				exact: true,
			}),
		).toBeVisible();
		expect(
			requests.overview
				.filter((r) => r.scope === "watchlist")
				.every((r) => r.marketCode == null && r.query == null),
		).toBe(true);
	});

	test("POST 未完成時鎖定追蹤操作，成功後使用正式 ID", async ({
		page,
		baseURL,
	}) => {
		let release = () => {};
		const gate = new Promise<void>((resolve) => {
			release = resolve;
		});
		const requests = await open(page, baseURL, { beforeCreate: () => gate });
		try {
			await discovery(page)
				.getByRole("button", { name: `追蹤${tomato}`, exact: true })
				.click();
			await expect.poll(() => requests.watchAdds.length).toBe(1);
			await expect(
				discovery(page).getByRole("button", {
					name: `停止追蹤${tomato}`,
					exact: true,
				}),
			).toBeDisabled();
			await expect(
				watchlist(page).getByRole("button", { name: `停止追蹤${cabbage}` }),
			).toBeDisabled();
		} finally {
			release();
		}
		await expect(
			discovery(page).getByRole("button", {
				name: `停止追蹤${tomato}`,
				exact: true,
			}),
		).toBeEnabled();
		await discovery(page)
			.getByRole("button", { name: `停止追蹤${tomato}`, exact: true })
			.click();
		await expect.poll(() => requests.watchDeletes).toEqual(["8002"]);
		expect(requests.watchAdds).toHaveLength(1);
	});

	test("較舊搜尋晚到不覆蓋新搜尋結果", async ({ page, baseURL }) => {
		let release = () => {};
		const gate = new Promise<void>((resolve) => {
			release = resolve;
		});
		const requests = await open(page, baseURL, {
			beforeOverview: async (query) => {
				if (query === "甘藍") {
					await gate;
				}
			},
		});
		const lateResponse = page.waitForResponse(
			(response) =>
				new URL(response.url()).searchParams.get("query") === "甘藍",
		);
		try {
			await discovery(page).getByRole("searchbox").fill("甘藍");
			await expect
				.poll(() => requests.overview.some((r) => r.query === "甘藍"))
				.toBe(true);
			await discovery(page).getByRole("searchbox").fill("番茄");
			await expect(discovery(page).locator(".market-radar__row")).toHaveCount(
				1,
			);
			await expect(
				discovery(page).getByRole("button", {
					name: `追蹤${tomato}`,
					exact: true,
				}),
			).toBeVisible();
		} finally {
			release();
		}
		await (await lateResponse).finished();
		await page.evaluate(
			() =>
				new Promise<void>((resolve) =>
					requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
				),
		);
		await expect(
			discovery(page).locator(".market-radar__identity strong"),
		).toHaveText("番茄");
	});
});

test.describe("市場行情：品項先完成、可選市場", { tag: ["@ui"] }, () => {
	test("行政區輪廓可點選、保留無價格摘要，移出地圖後仍可閱讀並關閉", async ({
		baseURL,
		isMobile,
		page,
	}) => {
		if (!baseURL) throw new Error("Playwright baseURL is required");
		await prepareInsightsFixture(page, baseURL, {
			administrativeBoundaries: true,
		});
		await page.goto(
			"/seller-center/insights?tab=spatial&scope=group&group=GRP-111&category=111&label=%E7%95%AA%E8%8C%84",
			{ waitUntil: "domcontentloaded" },
		);
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();
		const mapContainer = page.locator(
			".seller-price-map__canvas .maplibregl-map",
		);
		await expect(mapContainer).toBeVisible();
		await expect
			.poll(async () => {
				const candidate = await mapContainer.evaluateHandle(nativePriceMap);
				try {
					return await candidate.evaluate((instance) => instance?.loaded());
				} finally {
					await candidate.dispose();
				}
			})
			.toBe(true);
		const map = await mapContainer.evaluateHandle(nativePriceMap);
		const popup = page.getByRole("dialog", { name: "行政區資訊", exact: true });
		try {
			await test.step("直接點有價格的輪廓，保留估價、顯示帶與支撐市場", async () => {
				await clickAdministrativeOutline(page, map, [121.05, 24], isMobile);
				await expect(popup).toBeVisible();
				await expect(popup).toContainText("臺中市");
				await expect(popup).toContainText("NT$37 / kg");
				await expect(popup).toContainText("顯示帶 NT$30～NT$45");
				await expect(popup).toContainText("6 個市場支撐");
				await expect(popup).toContainText("狀態：approximate");
				await page.mouse.move(0, 0);
				await expect(popup).toBeVisible();
				await popup.getByRole("button", { name: "關閉地圖資訊" }).click();
				await expect(popup).toHaveCount(0);
			});

			await test.step("缺估價的摘要仍保留後端狀態、顯示帶與零支撐市場", async () => {
				await clickAdministrativeOutline(page, map, [121.15, 24.65], isMobile);
				await expect(popup).toBeVisible();
				await expect(popup).toContainText("宜蘭縣");
				await expect(popup).toContainText("目前沒有可用價格");
				await expect(popup).toContainText("顯示帶 NT$32～NT$48");
				await expect(popup).toContainText("0 個市場支撐");
				await expect(popup).toContainText("狀態：insufficient_support");
				await expect(popup).not.toContainText(/NT\$\s*0\s*\/\s*kg/);
				await expect(popup).not.toContainText("NT$37 / kg");
				await page.mouse.move(0, 0);
				await expect(popup).toBeVisible();
				await popup.getByRole("button", { name: "關閉地圖資訊" }).click();
				await expect(popup).toHaveCount(0);
			});

			await test.step("只有原始行政區邊界也能點選，且不杜撰價格", async () => {
				await clickAdministrativeOutline(page, map, [121.15, 23.7], isMobile);
				await expect(popup).toBeVisible();
				await expect(popup).toContainText("花蓮縣壽豐鄉");
				await expect(popup).toContainText("目前沒有可用價格");
				await expect(popup).not.toContainText("NT$");
				await expect(popup).not.toContainText("個市場支撐");
				await page.mouse.move(0, 0);
				await expect(popup).toBeVisible();
				const canvas = page.locator(".seller-price-map__canvas canvas");
				await canvas.scrollIntoViewIfNeeded();
				const background = await map.evaluate((instance) => {
					if (!instance) throw new Error("Price map was not initialized");
					const canvas = instance.getCanvas();
					const rect = canvas.getBoundingClientRect();
					// A geographic blank may sit under mobile notes or controls.
					// Choose from at most 81 actual screen positions and require both
					// an empty map hit-test and an unobscured DOM target before tapping.
					for (let row = 1; row <= 9; row++) {
						for (let column = 1; column <= 9; column++) {
							const x = (rect.width * column) / 10;
							const y = (rect.height * row) / 10;
							if (
								document.elementFromPoint(rect.left + x, rect.top + y) !==
								canvas
							)
								continue;
							const featureCount = instance.queryRenderedFeatures([
								x,
								y,
							]).length;
							if (featureCount === 0) return { x, y, featureCount };
						}
					}
					return null;
				});
				if (!background)
					throw new Error("No visible, unobscured blank map target");
				expect(background.featureCount).toBe(0);
				const position = { x: background.x, y: background.y };
				if (isMobile) await canvas.tap({ position });
				else await canvas.click({ position });
				await expect(popup).toHaveCount(0);
			});
		} finally {
			await map.dispose();
		}
	});

	test("exact crop 全部市場載入官方行情與預測，失敗 comparison 保留市場選項但沒有假價格", async ({
		baseURL,
		page,
	}) => {
		if (!baseURL) throw new Error("Playwright baseURL is required");
		const requests = await prepareInsightsFixture(page, baseURL, {
			comparisonFailure: true,
		});
		await page.goto("/seller-center/insights?scope=crop&crop=A1&label=番茄", {
			waitUntil: "domcontentloaded",
		});
		await expect
			.poll(() =>
				requests.observed.some(
					(request) => request.cropCode === "A1" && request.marketCode === null,
				),
			)
			.toBe(true);
		await expect
			.poll(() =>
				requests.forecast.some(
					(request) => request.cropCode === "A1" && request.marketCode === null,
				),
			)
			.toBe(true);
		const marketFilter = page.getByRole("button", { name: "市場範圍" });
		await expect(marketFilter).toContainText("全部市場");
		await page.getByRole("tab", { name: "市場比較" }).click();
		const comparison = page.getByRole("region", { name: "跨市場比較" });
		await expect(
			comparison.getByRole("button", { name: /切換到臺北一/ }),
		).toBeVisible();
		await expect(
			comparison.getByRole("button", { name: /切換到臺中市場/ }),
		).toHaveCount(0);
		await marketFilter.click();
		await expect(page.getByRole("option", { name: /臺中市場/ })).toBeVisible();
	});

	test("進入行情首頁直接看到官方波動雷達與自選追蹤", async ({
		baseURL,
		page,
	}) => {
		if (!baseURL) throw new Error("Playwright baseURL is required");
		const requests = await prepareInsightsFixture(page, baseURL);

		await page.goto("/seller-center/insights", {
			waitUntil: "domcontentloaded",
		});
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();

		const radar = page.locator(".market-radar");
		await expect(
			radar.getByRole("heading", { name: "市場雷達" }),
		).toBeVisible();
		await expect(
			radar.getByText("NT$ 61.5", { exact: true }).first(),
		).toBeVisible();
		const watchlistRegion = radar.getByRole("region", { name: "我的追蹤" });
		const discoveryRegion = radar.getByRole("region", {
			name: "全部官方行情",
		});
		await expect(
			watchlistRegion.getByRole("button", {
				name: "停止追蹤甘藍・初秋（全部市場，LA1）",
			}),
		).toBeVisible();
		const tomatoWatchButton = discoveryRegion.getByRole("button", {
			name: "追蹤番茄・牛番茄（全部市場，A1）",
		});
		await expect(tomatoWatchButton).toBeVisible();
		await expect(
			radar
				.getByRole("img", {
					name: /甘藍・初秋（全部市場，LA1）近 7 天內有成交資料的官方平均價走勢/,
				})
				.first(),
		).toBeVisible();
		await expect(radar).not.toContainText("P50");
		await expect(radar).not.toContainText("D+1");

		const marketFilter = page.getByRole("button", { name: "市場範圍" });
		await expect(marketFilter).toBeEnabled();
		await expect(marketFilter).toContainText("全部市場");
		await expect
			.poll(() =>
				requests.overview.some((request) => request.scope === "watchlist"),
			)
			.toBe(true);
		await expect
			.poll(() => requests.overview.some((request) => request.scope === "all"))
			.toBe(true);

		await tomatoWatchButton.click();
		await expect
			.poll(() => requests.watchAdds)
			.toContainEqual({ cropCode: "A1", marketCode: null });
		const tomatoUnwatchButton = discoveryRegion.getByRole("button", {
			name: "停止追蹤番茄・牛番茄（全部市場，A1）",
		});
		await expect(tomatoUnwatchButton).toBeVisible();
		await expect(
			watchlistRegion.getByRole("button", {
				name: "停止追蹤番茄・牛番茄（全部市場，A1）",
			}),
		).toBeVisible();
		await tomatoUnwatchButton.click();
		await expect.poll(() => requests.watchDeletes).toContain("8002");
		await expect(
			discoveryRegion.getByRole("button", {
				name: "追蹤番茄・牛番茄（全部市場，A1）",
			}),
		).toBeVisible();
		await expect(
			watchlistRegion.getByRole("button", {
				name: "停止追蹤番茄・牛番茄（全部市場，A1）",
			}),
		).toHaveCount(0);

		await discoveryRegion.getByRole("searchbox").fill("番茄");
		await expect
			.poll(() =>
				requests.overview.some(
					(request) => request.scope === "all" && request.query === "番茄",
				),
			)
			.toBe(true);
		await chooseTaipeiMarketWithKeyboard(marketFilter, page);
		await expect
			.poll(() =>
				requests.overview.some(
					(request) => request.scope === "all" && request.marketCode === "109",
				),
			)
			.toBe(true);

		await radar.getByRole("button", { name: "近 30 天" }).click();
		await discoveryRegion
			.getByRole("button", {
				name: "追蹤番茄・牛番茄（臺北一，A1）",
			})
			.click();
		await expect
			.poll(() => requests.watchAdds)
			.toContainEqual({ cropCode: "A1", marketCode: "109" });
		await expect(
			watchlistRegion.getByRole("button", {
				name: "停止追蹤番茄・牛番茄（臺北一，A1）",
			}),
		).toBeVisible();
		await expect(
			watchlistRegion.getByRole("button", {
				name: "停止追蹤甘藍・初秋（全部市場，LA1）",
			}),
		).toBeVisible();
		await expect
			.poll(() =>
				requests.overview.some(
					(request) => request.scope === "all" && request.windowDays === "30",
				),
			)
			.toBe(true);
		expect(
			requests.overview
				.filter((request) => request.scope === "watchlist")
				.every((request) => request.marketCode == null),
		).toBe(true);
		await expectNoHorizontalOverflow(page);
	});

	test("未選品項時快速離開價格地圖不會以空 container 初始化地圖", async ({
		baseURL,
		page,
	}) => {
		if (!baseURL) throw new Error("Playwright baseURL is required");
		await prepareInsightsFixture(page, baseURL);
		const invalidMapContainerErrors: string[] = [];
		const collectInvalidContainerError = (message: string) => {
			if (message.includes("Invalid type: 'container'")) {
				invalidMapContainerErrors.push(message);
			}
		};
		page.on("console", (message) => {
			if (message.type() === "error") {
				collectInvalidContainerError(message.text());
			}
		});
		page.on("pageerror", (error) =>
			collectInvalidContainerError(error.message),
		);

		await page.goto("/seller-center/insights", {
			waitUntil: "domcontentloaded",
		});
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();

		await page.getByRole("tab", { name: "價格地圖" }).click();
		await page.getByRole("tab", { name: "趨勢總覽" }).click();
		await expect(page.getByRole("heading", { name: "市場雷達" })).toBeVisible();
		await page.evaluate(
			() =>
				new Promise<void>((resolve) => {
					requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
				}),
		);

		expect(invalidMapContainerErrors).toEqual([]);
	});

	test("三級分類選品後以全部市場載入，市場 filter 可選、清除且不跨品項殘留", async ({
		baseURL,
		isMobile,
		page,
	}) => {
		test.skip(
			Boolean(isMobile),
			"This bounded contract exercises the desktop picker",
		);
		if (!baseURL) throw new Error("Playwright baseURL is required");
		const requests = await prepareInsightsFixture(page, baseURL);
		const invalidMapContainerErrors: string[] = [];
		page.on("console", (message) => {
			if (
				message.type() === "error" &&
				message.text().includes("Invalid type: 'container'")
			) {
				invalidMapContainerErrors.push(message.text());
			}
		});

		await page.goto("/seller-center/insights", {
			waitUntil: "domcontentloaded",
		});
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();
		await expect(
			page.getByRole("heading", { name: "市場行情工作台" }),
		).toBeVisible();

		const catalogSelector = page.getByRole("group", { name: "商品分類" });
		await expect(catalogSelector).toBeVisible();
		const marketFilter = page.getByRole("button", { name: "市場範圍" });
		await expect(marketFilter).toBeVisible();
		await expect(marketFilter).toBeEnabled();
		await expect(marketFilter).toContainText("全部市場");
		await expect(
			page.getByRole("dialog", { name: "選擇行情品項" }),
		).toHaveCount(0);

		await chooseDropdownOption(
			catalogSelector.getByLabel("分類", { exact: true }),
			page,
			"生鮮蔬果",
		);
		await chooseDropdownOption(
			catalogSelector.getByLabel("子分類", { exact: true }),
			page,
			"茄果類",
		);
		const leafTrigger = catalogSelector.getByLabel("細項", { exact: true });
		await chooseDropdownOption(leafTrigger, page, "番茄");
		await expect(
			catalogSelector.getByLabel("行情品相", { exact: true }),
		).toHaveCount(0);

		await expect
			.poll(() => new URL(page.url()).searchParams.get("group"))
			.toBe("GRP-111");
		expect(new URL(page.url()).searchParams.get("category")).toBe("111");
		expect(new URL(page.url()).searchParams.get("market")).toBeNull();
		expect(new URL(page.url()).searchParams.get("focusMarket")).toBeNull();
		await expect
			.poll(() => hasRequest(requests.observed, "GRP-111", null))
			.toBe(true);
		await expect
			.poll(() => hasRequest(requests.forecast, "GRP-111", null))
			.toBe(true);
		await expect(marketFilter).toBeEnabled();
		const metricLedger = page.locator(".seller-price-lookup__metric-ledger");
		await expect(
			metricLedger.locator(".seller-price-lookup__metric--primary"),
		).toContainText("NT$ 61.5");
		await expect(
			metricLedger.locator(".seller-price-lookup__metric--forecast"),
		).toContainText("NT$ 58");
		const spatialTab = page.getByRole("tab", { name: "價格地圖" });
		await spatialTab.click();
		await expect
			.poll(() => hasRequest(requests.spatial, "GRP-111", null))
			.toBe(true);
		const adaptiveLegend = page.locator(".seller-price-map__legend");
		await expect(adaptiveLegend).toHaveAttribute(
			"aria-label",
			/目前品項自適應 P50 色階：低值 NT\$40、全市場 P50 NT\$65、高值 NT\$90/,
		);
		expect(
			requests.observed.find(
				(request) =>
					request.groupCode === "GRP-111" && request.marketCode === null,
			)?.history,
		).toBe("true");

		await expect(marketFilter).toContainText("全部市場");
		await chooseTaipeiMarketWithKeyboard(marketFilter, page);
		await expect
			.poll(() => new URL(page.url()).searchParams.get("market"))
			.toBe("109");
		expect(new URL(page.url()).searchParams.get("focusMarket")).toBe("109");
		await expect
			.poll(() => hasRequest(requests.observed, "GRP-111", "109"))
			.toBe(true);
		await expect
			.poll(() => hasRequest(requests.forecast, "GRP-111", "109"))
			.toBe(true);

		await page.getByRole("tab", { name: "市場比較" }).click();
		const comparison = page.getByRole("region", { name: "跨市場比較" });
		await expect(comparison).toBeVisible();
		await expect(comparison).toContainText("不同品項預測分別平均");

		await marketFilter.click();
		await page.getByRole("option", { name: /全部市場/ }).click();
		await expect
			.poll(() => new URL(page.url()).searchParams.get("market"))
			.toBeNull();
		expect(new URL(page.url()).searchParams.get("focusMarket")).toBeNull();
		await expect(marketFilter).toContainText("全部市場");

		// Seed both market and map focus again, then change the crop. The crop
		// commit must clear both URL dimensions instead of leaking A1's market to B1.
		await chooseDropdownOption(marketFilter, page, "臺北一");
		await expect
			.poll(() => new URL(page.url()).searchParams.get("focusMarket"))
			.toBe("109");

		await chooseDropdownOption(
			catalogSelector.getByLabel("細項", { exact: true }),
			page,
			"甜椒",
		);

		await expect
			.poll(() => new URL(page.url()).searchParams.get("group"))
			.toBe("GRP-112");
		expect(new URL(page.url()).searchParams.get("category")).toBe("112");
		expect(new URL(page.url()).searchParams.get("market")).toBeNull();
		expect(new URL(page.url()).searchParams.get("focusMarket")).toBeNull();
		await expect
			.poll(() => hasRequest(requests.observed, "GRP-112", null))
			.toBe(true);
		await expect
			.poll(() => hasRequest(requests.forecast, "GRP-112", null))
			.toBe(true);
		await expect(marketFilter).toContainText("全部市場");

		await spatialTab.click();
		await expect
			.poll(() => hasRequest(requests.spatial, "GRP-112", null))
			.toBe(true);
		await expect(adaptiveLegend).toHaveAttribute(
			"aria-label",
			/目前品項自適應 P50 色階：低值 NT\$100、全市場 P50 NT\$150、高值 NT\$210/,
		);
		await expect(adaptiveLegend).not.toHaveAttribute(
			"aria-label",
			/NT\$40|NT\$65|NT\$90/,
		);
		expect(
			requests.spatial.find(
				(request) =>
					request.groupCode === "GRP-112" && request.marketCode === null,
			)?.horizonDays,
		).toBe("1");

		// The backend's category-resolution response is the sole authority on the
		// crop identity now — L3 must apply it directly and never reveal a second
		// "行情品相" control.
		await chooseDropdownOption(leafTrigger, page, "包心白-包白");
		await expect
			.poll(() => new URL(page.url()).searchParams.get("group"))
			.toBe("GRP-113");
		expect(new URL(page.url()).searchParams.get("category")).toBe("113");
		expect(new URL(page.url()).searchParams.get("market")).toBeNull();
		await expect
			.poll(() => hasRequest(requests.forecast, "GRP-113", null))
			.toBe(true);
		await expect(marketFilter).toContainText("全部市場");
		await expect(page.getByRole("group", { name: "確認行情對應" })).toHaveCount(
			0,
		);

		await page.getByRole("tab", { name: "趨勢總覽" }).click();
		await expect(
			page.locator(".seller-price-lookup__metric-ledger"),
		).toBeVisible();
		await page.evaluate(
			() =>
				new Promise<void>((resolve) => {
					requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
				}),
		);
		expect(invalidMapContainerErrors).toEqual([]);
	});

	test("只選到 L1 或 L1+L2（尚未到 L3）時，總覽分頁顯示分類彙整卡取代市場雷達", async ({
		baseURL,
		isMobile,
		page,
	}) => {
		test.skip(
			Boolean(isMobile),
			"This bounded contract exercises the desktop picker",
		);
		if (!baseURL) throw new Error("Playwright baseURL is required");
		await prepareInsightsFixture(page, baseURL);

		await page.goto("/seller-center/insights", {
			waitUntil: "domcontentloaded",
		});
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();
		const radar = page.locator(".market-radar");
		await expect(
			radar.getByRole("heading", { name: "市場雷達" }),
		).toBeVisible();

		const catalogSelector = page.getByRole("group", { name: "商品分類" });
		await chooseDropdownOption(
			catalogSelector.getByLabel("分類", { exact: true }),
			page,
			"生鮮蔬果",
		);

		const rollupCard = page.getByRole("region", {
			name: "生鮮蔬果分類彙整走勢",
		});
		await expect(rollupCard).toBeVisible();
		await expect(rollupCard).toContainText("生鮮蔬果（L1）分類彙整走勢");
		await expect(rollupCard).toContainText("NT$ 38.5");
		await expect(rollupCard).toContainText("來源 9 個大類平均");
		await expect(rollupCard).toContainText("D+1 模型中位價");
		await expect(rollupCard).toContainText("NT$ 39");
		await expect(rollupCard).toContainText("D+1 預測區間寬度");
		await expect(rollupCard).toContainText("NT$ 8"); // 43 - 35
		await expect(radar).toHaveCount(0);

		await chooseDropdownOption(
			catalogSelector.getByLabel("子分類", { exact: true }),
			page,
			"茄果類",
		);
		const l2RollupCard = page.getByRole("region", {
			name: "茄果類分類彙整走勢",
		});
		await expect(l2RollupCard).toBeVisible();
		await expect(l2RollupCard).toContainText("茄果類（L2）分類彙整走勢");
		await expect(l2RollupCard).toContainText("NT$ 45.2");
		await expect(l2RollupCard).toContainText("來源 3 個大類平均");

		// Completing L3 hands off entirely to the full resolved-crop/group view —
		// the simplified rollup card must not linger alongside it.
		await chooseDropdownOption(
			catalogSelector.getByLabel("細項", { exact: true }),
			page,
			"番茄",
		);
		await expect(
			page.getByRole("region", { name: "茄果類分類彙整走勢" }),
		).toHaveCount(0);
		await expect(
			page.locator(".seller-price-lookup__metric-ledger"),
		).toBeVisible();
	});

	test("mobile inline 分類選品後可切換選填市場且版面不產生水平溢出", async ({
		baseURL,
		isMobile,
		page,
	}) => {
		test.skip(!isMobile, "This bounded contract exercises the mobile layout");
		if (!baseURL) throw new Error("Playwright baseURL is required");
		const requests = await prepareInsightsFixture(page, baseURL);

		await page.goto("/seller-center/insights", {
			waitUntil: "domcontentloaded",
		});
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();
		await expectNoHorizontalOverflow(page);

		const catalogSelector = page.getByRole("group", { name: "商品分類" });
		await expect(
			catalogSelector.getByLabel("分類", { exact: true }),
		).toBeVisible();
		await expect(
			page.getByRole("dialog", { name: "選擇行情品項" }),
		).toHaveCount(0);

		await chooseDropdownOption(
			catalogSelector.getByLabel("分類", { exact: true }),
			page,
			"生鮮蔬果",
		);
		await chooseDropdownOption(
			catalogSelector.getByLabel("子分類", { exact: true }),
			page,
			"茄果類",
		);
		await chooseDropdownOption(
			catalogSelector.getByLabel("細項", { exact: true }),
			page,
			"番茄",
		);
		await expect(
			catalogSelector.getByLabel("行情品相", { exact: true }),
		).toHaveCount(0);

		await expect
			.poll(() => new URL(page.url()).searchParams.get("group"))
			.toBe("GRP-111");
		expect(new URL(page.url()).searchParams.get("market")).toBeNull();
		expect(new URL(page.url()).searchParams.get("focusMarket")).toBeNull();
		await expect
			.poll(() => hasRequest(requests.forecast, "GRP-111", null))
			.toBe(true);

		const marketFilter = page.getByRole("button", { name: "市場範圍" });
		await expect(marketFilter).toContainText("全部市場");
		await chooseTaipeiMarketWithKeyboard(marketFilter, page);
		await expect
			.poll(() => new URL(page.url()).searchParams.get("market"))
			.toBe("109");
		expect(new URL(page.url()).searchParams.get("focusMarket")).toBe("109");
		await expect
			.poll(() => hasRequest(requests.forecast, "GRP-111", "109"))
			.toBe(true);

		await marketFilter.click();
		await page.getByRole("option", { name: /全部市場/ }).click();
		await expect
			.poll(() => new URL(page.url()).searchParams.get("market"))
			.toBeNull();
		expect(new URL(page.url()).searchParams.get("focusMarket")).toBeNull();
		await expect(marketFilter).toContainText("全部市場");
		await expectNoHorizontalOverflow(page);

		await page.getByRole("tab", { name: "七日預測" }).click();
		const forecastList = page.getByRole("list", { name: "七日預測明細" });
		await expect(forecastList).toBeVisible();
		const firstDay = forecastList.getByRole("listitem").first();
		await expect(firstDay).toContainText("D+1");
		await expect(firstDay).toContainText("模型中位 P50");
		await expect(firstDay).toContainText("NT$ 58");
		await expect(firstDay).toContainText("NT$ 48～NT$ 68");
		await expect
			.poll(() =>
				forecastList.evaluate((node) => node.scrollWidth - node.clientWidth),
			)
			.toBeLessThanOrEqual(1);
	});

	test("官方行情失敗只留在自己的區塊，手動重試後恢復且不遮住模型預測", async ({
		baseURL,
		isMobile,
		page,
	}) => {
		test.skip(
			Boolean(isMobile),
			"This bounded contract exercises one viewport",
		);
		if (!baseURL) throw new Error("Playwright baseURL is required");
		const observedGate = { allowSuccess: false };
		const requests = await prepareInsightsFixture(page, baseURL, {
			observedGate,
		});
		const query = new URLSearchParams({
			scope: "crop",
			crop: "A1",
			label: "番茄（牛番茄）",
		});

		await page.goto(`/seller-center/insights?${query}`, {
			waitUntil: "domcontentloaded",
		});
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();

		const observedSection = page.getByRole("region", {
			name: "官方批發實際行情",
		});
		const observedError = observedSection.getByRole("alert");
		await expect(observedError).toContainText("官方實際行情載入失敗");
		await expect(
			page
				.locator(".seller-price-lookup__metric--forecast")
				.getByText("NT$ 58", { exact: true }),
		).toBeVisible();
		await expect(
			page.getByRole("region", { name: "七日條件式價格預測" }),
		).toContainText("7/7 日具有可用 P50 價格");

		const failedRequestCount = requests.observed.length;
		observedGate.allowSuccess = true;
		await observedError
			.getByRole("button", { name: "重新載入官方行情" })
			.click();
		await expect
			.poll(() => requests.observed.length)
			.toBeGreaterThan(failedRequestCount);
		await expect(observedError).toHaveCount(0);
		await expect(
			page.locator(".seller-price-lookup__metric--primary"),
		).toContainText("NT$ 61.5");
	});

	test("961px 時市場欄與行情狀態不重疊且頁面無水平溢出", async ({
		baseURL,
		isMobile,
		page,
	}) => {
		test.skip(
			Boolean(isMobile),
			"This contract targets the desktop breakpoint",
		);
		if (!baseURL) throw new Error("Playwright baseURL is required");
		await page.setViewportSize({ width: 961, height: 900 });
		const requests = await prepareInsightsFixture(page, baseURL);
		const query = new URLSearchParams({
			scope: "crop",
			crop: "A1",
			label: "番茄（牛番茄）",
		});

		await page.goto(`/seller-center/insights?${query}`, {
			waitUntil: "domcontentloaded",
		});
		await expect(page.locator('[data-auth-hydrated="true"]')).toBeAttached();
		await expect
			.poll(() => hasRequest(requests.forecast, "A1", null))
			.toBe(true);

		const marketFilter = page.locator(".seller-price-lookup__market-filter");
		const scopeSummary = page.locator(".seller-price-lookup__scope-summary");
		await expect(marketFilter).toBeVisible();
		await expect(scopeSummary).toBeVisible();
		await page.evaluate(() => document.fonts.ready.then(() => undefined));

		const marketBox = await marketFilter.boundingBox();
		const summaryBox = await scopeSummary.boundingBox();
		expect(marketBox).not.toBeNull();
		expect(summaryBox).not.toBeNull();
		if (!marketBox || !summaryBox) {
			throw new Error("Scope controls must have measurable layout boxes");
		}
		const overlaps =
			marketBox.x < summaryBox.x + summaryBox.width - 1 &&
			marketBox.x + marketBox.width > summaryBox.x + 1 &&
			marketBox.y < summaryBox.y + summaryBox.height - 1 &&
			marketBox.y + marketBox.height > summaryBox.y + 1;
		expect(overlaps).toBe(false);
		await expectNoHorizontalOverflow(page);
	});
});
