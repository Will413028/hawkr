import { apiClient } from "@/lib/api/client";

export interface CatalogMarketOption {
	marketCode: string;
	marketName: string;
}

export type CategoryResolutionStatus = "resolved" | "unmapped" | "ambiguous";

export interface CategoryCropVariety {
	cropCode: string;
	name: string;
	variantName?: string;
}

export interface CategoryResolutionData {
	categoryId: string;
	status: CategoryResolutionStatus;
	groupCode?: string;
	label?: string;
	mappingVersion: string;
	cropVarieties?: CategoryCropVariety[];
}

/** `/sellers/pricebook/category-resolution` — 後端權威的 categoryId→Pricebook
 *  品項解析，取代前端過渡的名稱猜測（issue #2786）。查無、歧義或目標分類已
 *  停用一律回 `unmapped`／`ambiguous`，不回舊值。純 fetcher。 */
export async function fetchCategoryResolution(
	categoryId: string,
): Promise<CategoryResolutionData> {
	const env = await apiClient.authed.get<CategoryResolutionData>(
		"/sellers/pricebook/category-resolution",
		{ query: { category_id: categoryId } },
	);
	if (!env.data) {
		throw new Error("分類解析服務未回傳資料");
	}
	return env.data;
}

export type CategoryPriceRollupStatus = "ready" | "unavailable";

export interface CategoryPriceRollupPoint {
	tradeDate: string;
	avgPrice?: number;
}

export interface CategoryPriceRollupData {
	categoryId: string;
	level: number;
	status: CategoryPriceRollupStatus;
	latestPrice?: number;
	prevPrice?: number;
	points: CategoryPriceRollupPoint[];
	sourceGroupCount: number;
	computedAt: string;
	forecastPoints: ForecastCurvePoint[];
}

function categoryPriceRollupPoint(
	value: unknown,
): CategoryPriceRollupPoint | null {
	const record = overviewRecord(value);
	const tradeDate = overviewDate(record?.tradeDate);
	if (!tradeDate) return null;
	const avgPrice = overviewDecimal(record?.avgPrice, { minimum: 0 });
	return { tradeDate, ...(avgPrice == null ? {} : { avgPrice }) };
}

function categoryForecastPoint(value: unknown): ForecastCurvePoint | null {
	const record = overviewRecord(value);
	const targetDate = overviewDate(record?.targetDate);
	if (!targetDate) return null;
	const horizonDays = overviewNumber(record?.horizonDays, {
		integer: true,
		minimum: 1,
	});
	if (horizonDays == null) return null;
	const status = overviewText(record?.status) ?? "";
	const p10 = overviewDecimal(record?.p10, { minimum: 0 });
	const p50 = overviewDecimal(record?.p50, { minimum: 0 });
	const p90 = overviewDecimal(record?.p90, { minimum: 0 });
	return {
		targetDate,
		horizonDays,
		status,
		...(p10 == null ? {} : { p10 }),
		...(p50 == null ? {} : { p50 }),
		...(p90 == null ? {} : { p90 }),
	};
}

/** `/sellers/pricebook/category-price-rollup` — 分類樹任一層（L1/L2/L3）的每日
 *  彙整快照；`status:"unavailable"` 代表底下沒有任何有價分類，不是查詢失敗。
 *  金額欄位是後端字串化數字（避免 JS 精度問題），比照 `overviewDecimal` 既有的
 *  防呆解析——不信任型別，遇到非十進位字串一律捨棄該欄位而非硬湊數字。 */
export async function fetchCategoryPriceRollup(
	categoryId: string,
): Promise<CategoryPriceRollupData> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		"/sellers/pricebook/category-price-rollup",
		{ query: { category_id: categoryId } },
	);
	const record = overviewRecord(env.data);
	if (!record) {
		throw new Error("分類彙整價格服務未回傳資料");
	}
	const points = Array.isArray(record.points)
		? record.points
				.map((point) => categoryPriceRollupPoint(point))
				.filter((point): point is CategoryPriceRollupPoint => point != null)
		: [];
	return {
		categoryId: overviewText(record.categoryId) ?? categoryId,
		level: overviewNumber(record.level, { integer: true, minimum: 1 }) ?? 0,
		status: record.status === "ready" ? "ready" : "unavailable",
		latestPrice: overviewDecimal(record.latestPrice, { minimum: 0 }),
		prevPrice: overviewDecimal(record.prevPrice, { minimum: 0 }),
		points,
		forecastPoints: Array.isArray(record.forecastPoints)
			? record.forecastPoints
					.map((point) => categoryForecastPoint(point))
					.filter((point): point is ForecastCurvePoint => point != null)
			: [],
		sourceGroupCount:
			overviewNumber(record.sourceGroupCount, { integer: true, minimum: 0 }) ??
			0,
		computedAt: overviewText(record.computedAt) ?? "",
	};
}

/** 三選一的行情品項選取結果——cropName 是舊的種子表俗名路徑，目前沒有任何
 *  UI 路徑會產生 `{kind:"cropName"}`（InsightsClient 一律走 crop/group，直接
 *  帶 category-resolution 回傳的原始代碼查詢，繞過種子表解析）；這個 variant
 *  只被 repo.unit.test.ts 覆蓋，仍是後端合約的一部分故保留。
 *
 *  crop 的 marketCode 是選填篩選：省略代表全市場的伺服器權威彙整，指定後才
 *  只查單一市場。前端不得用 raw market rows 自行平均成全市場價格。 */
export type PricebookSelection =
	| { kind: "cropName"; cropName: string }
	| { kind: "crop"; cropCode: string; marketCode?: string }
	| { kind: "group"; groupCode: string; marketCode?: string };

/** Stable cache-key string for a PricebookSelection — used as the
 *  TanStack Query key's differentiator (a plain object can't be a query
 *  key segment). */
export function selectionKey(selection: PricebookSelection): string {
	switch (selection.kind) {
		case "cropName":
			return `cropName:${selection.cropName}`;
		case "crop":
			return `crop:${selection.cropCode}:market:${selection.marketCode ?? "@all"}`;
		case "group":
			return `group:${selection.groupCode}:market:${selection.marketCode ?? "@all"}`;
	}
}

/** National spatial artifacts are keyed by crop/group only. A comparison
 * market changes the rest of the workspace, but it must neither refetch nor
 * fork the cache for the same nationwide crop surface. */
export function spatialSelectionKey(selection: PricebookSelection): string {
	switch (selection.kind) {
		case "cropName":
			return `cropName:${selection.cropName}`;
		case "crop":
			return `crop:${selection.cropCode}`;
		case "group":
			return `group:${selection.groupCode}`;
	}
}

function selectionQuery(selection: PricebookSelection): Record<string, string> {
	switch (selection.kind) {
		case "cropName":
			return { cropName: selection.cropName };
		case "crop":
			return selection.marketCode
				? { cropCode: selection.cropCode, marketCode: selection.marketCode }
				: { cropCode: selection.cropCode };
		case "group":
			return selection.marketCode
				? { groupCode: selection.groupCode, marketCode: selection.marketCode }
				: { groupCode: selection.groupCode };
	}
}

function spatialSelectionQuery(
	selection: PricebookSelection,
): Record<string, string> {
	switch (selection.kind) {
		case "cropName":
			return { cropName: selection.cropName };
		case "crop":
			// A national surface belongs to the crop, not to whichever comparison
			// market the seller currently has selected in the rest of the workspace.
			return { cropCode: selection.cropCode };
		case "group":
			return { groupCode: selection.groupCode };
	}
}

export interface PriceObservation {
	marketCode: string;
	cropCode: string;
	tradeDate: string;
	avgPrice?: number;
	upperPrice?: number;
	middlePrice?: number;
	lowerPrice?: number;
	volumeKg?: number;
}

export interface ObservedPriceHistoryData {
	observations: PriceObservation[];
}

/** `/sellers/pricebook/observed-price?history=true` — 歷史實際成交價（近
 *  30 天，伺服器固定窗口），純 fetcher。 */
export async function fetchObservedPriceHistory(
	selection: PricebookSelection,
): Promise<ObservedPriceHistoryData> {
	const env = await apiClient.authed.get<ObservedPriceHistoryData>(
		"/sellers/pricebook/observed-price",
		{ query: { ...selectionQuery(selection), history: true } },
	);
	return env.data ?? { observations: [] };
}

export type MarketOverviewScope = "watchlist" | "all";
export type MarketOverviewWindowDays = 7 | 14 | 30;
export type MarketOverviewDirection = "up" | "down" | "flat" | "unknown";
export type MarketOverviewFreshness =
	| "fresh"
	| "stale"
	| "unavailable"
	| "unknown";

export interface MarketOverviewQuery {
	scope: MarketOverviewScope;
	windowDays: MarketOverviewWindowDays;
	marketCode?: string;
	query?: string;
	page?: number;
	pageSize?: number;
}

export interface MarketOverviewActual {
	tradeDate: string;
	avgPrice?: number;
	volumeKg?: number;
}

export interface MarketOverviewChange {
	absolute?: number;
	percent?: number;
	direction: MarketOverviewDirection;
	basis?: string;
}

export interface MarketOverviewPoint {
	tradeDate: string;
	avgPrice?: number;
}

export interface MarketOverviewItem {
	watchTargetId?: string;
	cropCode: string;
	cropName: string;
	variantName?: string;
	marketCode?: string;
	marketName?: string;
	isWatched: boolean;
	status?: string;
	freshness: MarketOverviewFreshness;
	qualityReasonCodes: string[];
	currency?: "TWD";
	/** 後端送來的原始單位代碼（kg／catty／100kg）。
	 *
	 * 🔴 刻意不是字面型別 "kg"。擴充多來源之前這裡是 `unit?: "kg"`，把「目前
	 * 只有一個值」寫成了「契約只允許一個值」——雞蛋產地價是元／台斤，一接上來
	 * 就會被下面的正規化丟成 undefined，整排顯示「單位待確認」。
	 * 契約是「必須有單位」，不是「單位必須是公斤」。 */
	unit?: string;
	/** 價格層級。缺席時語意為 wholesale_auction——
	 * 擴充前唯一可能的值。 */
	priceLayer?: string;
	sourceDataAsOf?: string;
	latestActual?: MarketOverviewActual;
	previousActual?: MarketOverviewActual;
	change?: MarketOverviewChange;
	points: MarketOverviewPoint[];
}

export interface MarketOverviewData {
	snapshotId?: string;
	source?: string;
	generatedAt?: string;
	sourceDataAsOf?: string;
	scope: MarketOverviewScope;
	windowDays: MarketOverviewWindowDays;
	marketCode?: string;
	currency?: "TWD";
	/** 見 MarketOverviewItem.unit 的說明——同樣不是字面型別。 */
	unit?: string;
	marketOptions: CatalogMarketOption[];
	items: MarketOverviewItem[];
	page: number;
	pageSize: number;
	totalItems?: number;
	hasNext: boolean;
	/** Client-side contract findings. Invalid rows are withheld rather than
	 * coerced into a plausible price or identity. */
	contractIssues: string[];
}

export interface MarketWatchTargetResult {
	watchTargetId: string;
	created?: boolean;
}

type MarketOverviewRecord = Record<string, unknown>;

function overviewRecord(value: unknown): MarketOverviewRecord | null {
	return value !== null && typeof value === "object" && !Array.isArray(value)
		? (value as MarketOverviewRecord)
		: null;
}

function overviewText(value: unknown): string | undefined {
	return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

function overviewNumber(
	value: unknown,
	options: { minimum?: number; integer?: boolean } = {},
): number | undefined {
	const parsed = overviewDecimal(value, options);
	if (parsed == null) return undefined;
	if (options.integer && !Number.isSafeInteger(parsed)) return undefined;
	return parsed;
}

const OVERVIEW_DECIMAL_PATTERN = /^[+-]?(?:0|[1-9]\d*)(?:\.\d+)?$/;

/** ProtoJSON encodes backend decimal values as strings. Numbers remain
 * accepted during the additive rollout, but no exponent, unit suffix,
 * whitespace, Infinity, NaN or null is coerced into a display price. */
function overviewDecimal(
	value: unknown,
	options: { minimum?: number } = {},
): number | undefined {
	let parsed: number;
	if (typeof value === "number") {
		if (!Number.isFinite(value)) return undefined;
		parsed = value;
	} else if (
		typeof value === "string" &&
		value === value.trim() &&
		OVERVIEW_DECIMAL_PATTERN.test(value)
	) {
		parsed = Number(value);
		if (!Number.isFinite(parsed)) return undefined;
	} else {
		return undefined;
	}
	if (options.minimum != null && parsed < options.minimum) return undefined;
	return Object.is(parsed, -0) ? 0 : parsed;
}

function overviewDate(value: unknown): string | undefined {
	const text = overviewText(value);
	if (!text || !/^\d{4}-\d{2}-\d{2}$/.test(text)) return undefined;
	const parsed = new Date(`${text}T00:00:00.000Z`);
	return Number.isNaN(parsed.getTime()) ||
		parsed.toISOString().slice(0, 10) !== text
		? undefined
		: text;
}

function overviewSourceTimestamp(value: unknown): string | undefined {
	const text = overviewText(value);
	if (!text) return undefined;
	if (overviewDate(text)) return text;
	if (
		!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(
			text,
		)
	) {
		return undefined;
	}
	return Number.isNaN(Date.parse(text)) ? undefined : text;
}

function normalizeOverviewActual(
	value: unknown,
	path: string,
	issues: string[],
): MarketOverviewActual | undefined {
	const record = overviewRecord(value);
	if (!record) return undefined;
	const tradeDate = overviewDate(record.tradeDate);
	if (!tradeDate) return undefined;
	const avgPrice = overviewDecimal(record.avgPrice, { minimum: 0 });
	if (record.avgPrice != null && avgPrice == null) {
		issues.push(`${path}.avgPrice 不是有效十進位數值`);
	}
	const volumeKg = overviewDecimal(record.volumeKg, { minimum: 0 });
	return {
		tradeDate,
		...(avgPrice == null ? {} : { avgPrice }),
		...(volumeKg == null ? {} : { volumeKg }),
	};
}

function normalizeOverviewPoint(
	value: unknown,
	path: string,
	issues: string[],
): MarketOverviewPoint | undefined {
	const record = overviewRecord(value);
	if (!record) return undefined;
	const tradeDate = overviewDate(record.tradeDate);
	if (!tradeDate) return undefined;
	const avgPrice = overviewDecimal(record.avgPrice, { minimum: 0 });
	if (record.avgPrice != null && avgPrice == null) {
		issues.push(`${path}.avgPrice 不是有效十進位數值`);
	}
	return { tradeDate, ...(avgPrice == null ? {} : { avgPrice }) };
}

function normalizeOverviewChange(
	value: unknown,
	path: string,
	issues: string[],
): MarketOverviewChange | undefined {
	const record = overviewRecord(value);
	if (!record) return undefined;
	const directionValue = overviewText(record.direction);
	const direction: MarketOverviewDirection =
		directionValue === "up" ||
		directionValue === "down" ||
		directionValue === "flat" ||
		directionValue === "unknown"
			? directionValue
			: "unknown";
	const absolute = overviewDecimal(record.absolute);
	const percent = overviewDecimal(record.percent);
	if (record.absolute != null && absolute == null) {
		issues.push(`${path}.absolute 不是有效十進位數值`);
	}
	if (record.percent != null && percent == null) {
		issues.push(`${path}.percent 不是有效十進位數值`);
	}
	if (absolute == null && percent == null && direction === "unknown") {
		return undefined;
	}
	return {
		direction,
		basis: overviewText(record.basis),
		...(absolute == null ? {} : { absolute }),
		...(percent == null ? {} : { percent }),
	};
}

function normalizeOverviewItem(
	value: unknown,
	index: number,
	issues: string[],
): MarketOverviewItem | undefined {
	const record = overviewRecord(value);
	if (!record) {
		issues.push(`items[${index}] 不是物件`);
		return undefined;
	}
	const cropCode = overviewText(record.cropCode);
	const cropName = overviewText(record.cropName);
	if (!cropCode || !cropName) {
		issues.push(`items[${index}] 缺少 cropCode 或 cropName`);
		return undefined;
	}
	const rawPoints = Array.isArray(record.points) ? record.points : [];
	if (!Array.isArray(record.points)) {
		issues.push(`items[${index}].points 缺失`);
	}
	const points = rawPoints
		.map((point, pointIndex) =>
			normalizeOverviewPoint(
				point,
				`items[${index}].points[${pointIndex}]`,
				issues,
			),
		)
		.filter((point): point is MarketOverviewPoint => point != null)
		.sort((left, right) => left.tradeDate.localeCompare(right.tradeDate));
	if (points.length !== rawPoints.length) {
		issues.push(`items[${index}].points 含無效交易日或價格`);
	}
	const freshnessValue = overviewText(
		record.freshnessStatus ?? record.freshness,
	);
	const freshness: MarketOverviewFreshness =
		freshnessValue === "fresh" ||
		freshnessValue === "stale" ||
		freshnessValue === "unavailable" ||
		freshnessValue === "unknown"
			? freshnessValue
			: "unknown";
	const qualityReasonCodes = Array.isArray(record.qualityReasonCodes)
		? record.qualityReasonCodes
				.map((reason) => overviewText(reason))
				.filter((reason): reason is string => reason != null)
		: [];
	const currency = record.currency === "TWD" ? "TWD" : undefined;
	// 單位原樣帶過。驗證的是「有沒有單位」而不是「是不是公斤」——後者在多來源
	// 之後會把合法的台斤價判成契約違反。
	const unit = overviewText(record.unit);
	if (record.currency != null && currency == null) {
		issues.push(`items[${index}].currency 必須是 TWD`);
	}
	if (record.unit != null && unit == null) {
		issues.push(`items[${index}].unit 必須是非空字串`);
	}
	const priceLayer = overviewText(record.priceLayer);
	const sourceDataAsOf = overviewSourceTimestamp(record.sourceDataAsOf);
	if (record.sourceDataAsOf != null && sourceDataAsOf == null) {
		issues.push(`items[${index}].sourceDataAsOf 不是有效時間`);
	}
	return {
		cropCode,
		cropName,
		watchTargetId: overviewText(record.watchTargetId),
		variantName: overviewText(record.variantName),
		marketCode: overviewText(record.marketCode),
		marketName: overviewText(record.marketName),
		isWatched: record.isWatched === true,
		status: overviewText(record.status),
		freshness,
		qualityReasonCodes,
		currency,
		unit,
		priceLayer,
		sourceDataAsOf,
		latestActual: normalizeOverviewActual(
			record.latestActual,
			`items[${index}].latestActual`,
			issues,
		),
		previousActual: normalizeOverviewActual(
			record.previousActual,
			`items[${index}].previousActual`,
			issues,
		),
		change: normalizeOverviewChange(
			record.change,
			`items[${index}].change`,
			issues,
		),
		points,
	};
}

/** Normalize the additive market-radar contract without manufacturing rows,
 * zero prices or market identities. Shape defects stay visible through
 * `contractIssues` while valid sibling rows remain usable. */
export function normalizeMarketOverviewData(
	value: unknown,
	request: MarketOverviewQuery,
): MarketOverviewData {
	const issues: string[] = [];
	const record = overviewRecord(value);
	if (!record) {
		return {
			scope: request.scope,
			windowDays: request.windowDays,
			marketCode: request.scope === "all" ? request.marketCode : undefined,
			marketOptions: [],
			items: [],
			page: request.page ?? 1,
			pageSize: request.pageSize ?? 20,
			hasNext: false,
			contractIssues: ["overview payload 缺失"],
		};
	}
	const rawItems = Array.isArray(record.items) ? record.items : [];
	if (!Array.isArray(record.items)) issues.push("items 缺失");
	const items = rawItems
		.map((item, index) => normalizeOverviewItem(item, index, issues))
		.filter((item): item is MarketOverviewItem => item != null);
	const rawMarketOptions = Array.isArray(record.marketOptions)
		? record.marketOptions
		: [];
	const marketOptions = rawMarketOptions
		.map((value, index) => {
			const option = overviewRecord(value);
			const marketCode = overviewText(option?.marketCode);
			const marketName = overviewText(option?.marketName);
			if (!marketCode || !marketName) {
				issues.push(`marketOptions[${index}] 缺少代碼或名稱`);
				return undefined;
			}
			return { marketCode, marketName };
		})
		.filter((option): option is CatalogMarketOption => option != null);
	const page =
		overviewNumber(record.page, { integer: true, minimum: 1 }) ??
		request.page ??
		1;
	const pageSize =
		overviewNumber(record.pageSize, { integer: true, minimum: 1 }) ??
		request.pageSize ??
		20;
	const totalItems = overviewNumber(record.totalItems, {
		integer: true,
		minimum: 0,
	});
	const topLevelCurrency = record.currency === "TWD" ? "TWD" : undefined;
	const topLevelUnit = overviewText(record.unit);
	if (record.currency != null && topLevelCurrency == null) {
		issues.push("currency 必須是 TWD");
	}
	if (record.unit != null && topLevelUnit == null) {
		issues.push("unit 必須是非空字串");
	}
	const currency =
		topLevelCurrency ??
		(items.length > 0 && items.every((item) => item.currency === "TWD")
			? "TWD"
			: undefined);
	// 頂層單位只有在**所有**品項單位相同時才推導得出來。多來源之後這個清單
	// 可能同時含元/公斤與元/台斤，此時頂層留白、由各品項自己帶——這正是
	// MarketOverviewItem 有 per-item unit 欄位的理由。
	const firstItemUnit = items.length > 0 ? items[0].unit : undefined;
	const unit =
		topLevelUnit ??
		(firstItemUnit != null && items.every((item) => item.unit === firstItemUnit)
			? firstItemUnit
			: undefined);
	return {
		snapshotId: overviewText(record.snapshotId),
		source: overviewText(record.source),
		generatedAt: overviewSourceTimestamp(record.generatedAt),
		sourceDataAsOf: overviewSourceTimestamp(record.sourceDataAsOf),
		scope: request.scope,
		windowDays: request.windowDays,
		marketCode: request.scope === "all" ? request.marketCode : undefined,
		currency,
		unit,
		marketOptions,
		items,
		page,
		pageSize,
		...(totalItems == null ? {} : { totalItems }),
		hasNext:
			typeof record.hasNext === "boolean"
				? record.hasNext
				: totalItems != null
					? page * pageSize < totalItems
					: false,
		contractIssues: issues,
	};
}

/** Batch official observed prices for the radar. One request serves every
 * visible sparkline; callers must not fan out `observed-price` per row. */
export async function fetchMarketOverview(
	request: MarketOverviewQuery,
): Promise<MarketOverviewData> {
	const env = await apiClient.authed.get<unknown>(
		"/sellers/pricebook/overview",
		{
			query: {
				scope: request.scope,
				window_days: request.windowDays,
				...(request.scope === "all" && request.marketCode
					? { market_code: request.marketCode }
					: {}),
				query: request.query?.trim() || undefined,
				page: request.page,
				page_size: request.pageSize,
			},
		},
	);
	return normalizeMarketOverviewData(env.data, request);
}

export async function createMarketWatchTarget(input: {
	cropCode: string;
	marketCode?: string;
}): Promise<MarketWatchTargetResult> {
	const env = await apiClient.authed.post<unknown>(
		"/sellers/pricebook/watch-targets",
		{ json: input },
	);
	const record = overviewRecord(env.data);
	const watchTargetId = overviewText(record?.watchTargetId);
	if (!watchTargetId) {
		throw new Error("追蹤服務未回傳有效的追蹤識別碼");
	}
	return {
		watchTargetId,
		...(typeof record?.created === "boolean"
			? { created: record.created }
			: {}),
	};
}

export async function deleteMarketWatchTarget(
	watchTargetId: string,
): Promise<void> {
	await apiClient.authed.delete(
		`/sellers/pricebook/watch-targets/${encodeURIComponent(watchTargetId)}`,
	);
}

export interface ForecastCurvePoint {
	targetDate: string;
	horizonDays: number;
	status: string;
	p10?: number;
	p50?: number;
	p90?: number;
}

export interface MarketComparisonEntry {
	marketCode: string;
	marketName: string;
	p50?: number | null;
	status?: string;
	isApproximate: boolean;
}

export interface ForecastCurveData {
	status: string;
	resolvedCropGroupCode?: string;
	resolvedCropCode?: string;
	marketCode?: string;
	modelVersion: string;
	points: ForecastCurvePoint[];
	marketComparison: MarketComparisonEntry[];
}

export type GeoJsonGeometry =
	| { type: "Point"; coordinates: number[] }
	| { type: "LineString"; coordinates: number[][] }
	| { type: "MultiLineString"; coordinates: number[][][] }
	| { type: "Polygon"; coordinates: number[][][] }
	| { type: "MultiPolygon"; coordinates: number[][][][] };

export interface GeoJsonFeature {
	type: "Feature";
	id?: string | number;
	properties: Record<string, unknown> | null;
	geometry: GeoJsonGeometry;
}

export interface GeoJsonFeatureCollection {
	type: "FeatureCollection";
	features: GeoJsonFeature[];
}

export interface SpatialPriceScale {
	scaleId: string;
	policyVersion: string;
	/** Server-owned upper-bound thresholds for this selection and horizon.
	 * The same response values must drive cells, contours, county rollups and
	 * the website legend; the client never recomputes them. */
	breaks: number[];
}

export const SPATIAL_ADAPTIVE_P50_POLICY = "selection-p50-v1";
export const SPATIAL_ADAPTIVE_P50_FLAT_POLICY =
	"selection-p50-v1-flat-fallback";

export interface SpatialBoundary {
	version: string;
	digest: string;
	source?: {
		label?: string;
		url?: string;
		license?: string;
	};
	bboxWgs84: number[];
	geojson: GeoJsonFeatureCollection;
}

export interface SpatialMarketPrice {
	marketCode: string;
	marketName: string;
	categoryCode?: string;
	predictionStatus: string;
	marketStatus: string;
	p10?: number;
	p50?: number;
	p90?: number;
	recentVolumeKg?: number;
	usedForSurface: boolean;
}

export interface SpatialSurfaceStatistics {
	p10?: number;
	p50?: number;
	p90?: number;
	minimumP50?: number;
	maximumP50?: number;
	pricedMarketCount: number;
	closedMarketCount: number;
	totalMarketCount: number;
	recentVolumeKg?: number;
	surfaceCellCount: number;
	contourSegmentCount: number;
}

export interface SpatialSurfaceLineage {
	artifactId: string;
	generatedAt?: string;
	sourceDataAsOf?: string;
	maxFeatureAvailableAt?: string;
	modelVersion: string;
	datasetDigest: string;
	boundaryDigest: string;
	schemaVersion?: string;
}

/** Seller-BFF spatial contract. The Transformer owns category-aware geography,
 * TWD97 IDW, clipping, contours and quality gating; the browser only renders
 * the returned RFC 7946 artifacts. See backend-payuni #2248. */
export interface SpatialSurfaceData {
	status: "ready" | "degraded" | "unavailable";
	displayMode:
		| "contours_and_points"
		| "surface_and_points"
		| "points_only"
		| "table_only";
	selectionKind: "crop" | "group";
	targetDate: string;
	horizonDays: number;
	currency: "TWD";
	unit: "kg";
	scale: SpatialPriceScale;
	boundary: SpatialBoundary;
	boundaryPriceGeojson: GeoJsonFeatureCollection;
	marketPointsGeojson: GeoJsonFeatureCollection;
	surfaceCellsGeojson: GeoJsonFeatureCollection;
	contourLinesGeojson: GeoJsonFeatureCollection;
	marketPrices: SpatialMarketPrice[];
	statistics: SpatialSurfaceStatistics;
	qualityReasonCodes: string[];
	disclosures: string[];
	supportedLandAreaRatio?: number;
	excludedMarketCodes: string[];
	lineage: SpatialSurfaceLineage;
}

type UnknownRecord = Record<string, unknown>;

const SPATIAL_STATUSES = new Set(["ready", "degraded"]);
const SPATIAL_DISPLAY_MODES = new Set([
	"contours_and_points",
	"surface_and_points",
	"points_only",
	"table_only",
]);

function spatialContractError(field: string, reason = "格式不正確"): Error {
	return new Error(`空間行情資料不可用：${field} ${reason}`);
}

/** `status:"unavailable"` 時後端仍會回傳完整的診斷欄位（哪些市場休市、品質
 *  代碼），只是不足以組出可信的地圖圖層。這個欄位不參與地圖繪製，只用於
 *  文案顯示，所以用寬鬆讀取（讀不到就是 undefined），不透過會 throw 的
 *  numberAt/stringArrayAt——那組是為「會被畫成地圖」的資料把關，這裡不是。 */
function looseStringArray(value: unknown): string[] {
	return Array.isArray(value)
		? value.filter((item): item is string => typeof item === "string")
		: [];
}

function looseNumber(value: unknown): number | undefined {
	return typeof value === "number" && Number.isFinite(value)
		? value
		: undefined;
}

/** `status:"unavailable"`（例：D+N 當天全國批發市場共同休市，沒有任何成交
 *  資料可插值）拋出時攜帶的診斷細節，讓呼叫端能顯示具體原因而不是通用錯誤
 *  文案。見 backend-payuni #2248 PR C 之後發現的 D+2 全休市案例。 */
export class SpatialSurfaceUnavailableError extends Error {
	readonly targetDate?: string;
	readonly qualityReasonCodes: string[];
	readonly closedMarketCount?: number;
	readonly totalMarketCount?: number;

	constructor(raw: UnknownRecord) {
		super("空間行情目前不可用");
		this.name = "SpatialSurfaceUnavailableError";
		this.targetDate =
			typeof raw.targetDate === "string" ? raw.targetDate : undefined;
		this.qualityReasonCodes = looseStringArray(raw.qualityReasonCodes);
		const stats =
			raw.statistics && typeof raw.statistics === "object"
				? (raw.statistics as UnknownRecord)
				: undefined;
		this.closedMarketCount = looseNumber(stats?.closedMarketCount);
		this.totalMarketCount = looseNumber(stats?.totalMarketCount);
	}
}

/** 這個狀態是後端業務邏輯的預期結果（例如當天全國市場共同休市），不是我方
 *  程式或契約壞掉——呼叫端應在 TanStack Query 的 `meta.isExpectedError` 掛上
 *  這個判斷式跳過 Sentry 上報，同「店鋪還沒建」（isStoreMissing）的
 *  既有作法，見 lib/query/client.ts。 */
export function isSpatialSurfaceUnavailable(error: unknown): boolean {
	return error instanceof SpatialSurfaceUnavailableError;
}

function recordAt(value: unknown, field: string): UnknownRecord {
	if (value === null || typeof value !== "object" || Array.isArray(value)) {
		throw spatialContractError(field);
	}
	return value as UnknownRecord;
}

function stringAt(value: unknown, field: string): string {
	if (typeof value !== "string" || !value.trim()) {
		throw spatialContractError(field, "不可為空");
	}
	return value;
}

function optionalStringAt(value: unknown, field: string): string | undefined {
	if (value == null) return undefined;
	return stringAt(value, field);
}

function numberAt(
	value: unknown,
	field: string,
	options: { integer?: boolean; min?: number; max?: number } = {},
): number {
	if (typeof value !== "number" || !Number.isFinite(value)) {
		throw spatialContractError(field, "必須是有限數值");
	}
	if (options.integer && !Number.isInteger(value)) {
		throw spatialContractError(field, "必須是整數");
	}
	if (options.min != null && value < options.min) {
		throw spatialContractError(field, `不得小於 ${options.min}`);
	}
	if (options.max != null && value > options.max) {
		throw spatialContractError(field, `不得大於 ${options.max}`);
	}
	return value;
}

function optionalNumberAt(
	value: unknown,
	field: string,
	options: { integer?: boolean; min?: number; max?: number } = {},
): number | undefined {
	// The BFF uses null for a closed/missing/unmodelled value. Normalize it to
	// undefined so no downstream Number(null) path can manufacture NT$0.
	if (value == null) return undefined;
	return numberAt(value, field, options);
}

function booleanAt(value: unknown, field: string): boolean {
	if (typeof value !== "boolean") throw spatialContractError(field);
	return value;
}

function stringArrayAt(value: unknown, field: string): string[] {
	if (!Array.isArray(value)) throw spatialContractError(field);
	return value.map((item, index) => stringAt(item, `${field}[${index}]`));
}

function isPosition(value: unknown): value is number[] {
	return (
		Array.isArray(value) &&
		value.length >= 2 &&
		value.every(
			(coordinate) =>
				typeof coordinate === "number" && Number.isFinite(coordinate),
		)
	);
}

function isLineStringCoordinates(value: unknown): value is number[][] {
	return Array.isArray(value) && value.length >= 2 && value.every(isPosition);
}

function isPolygonCoordinates(value: unknown): value is number[][][] {
	return (
		Array.isArray(value) &&
		value.length > 0 &&
		value.every((ring) => {
			if (!Array.isArray(ring) || ring.length < 4 || !ring.every(isPosition)) {
				return false;
			}
			const first = ring[0];
			const last = ring[ring.length - 1];
			return (
				first.length === last.length &&
				first.every((coordinate, index) => coordinate === last[index])
			);
		})
	);
}

function geometryAt(value: unknown, field: string): GeoJsonGeometry {
	const geometry = recordAt(value, field);
	switch (geometry.type) {
		case "Point":
			if (!isPosition(geometry.coordinates)) throw spatialContractError(field);
			break;
		case "LineString":
			if (!isLineStringCoordinates(geometry.coordinates))
				throw spatialContractError(field);
			break;
		case "MultiLineString":
			if (
				!Array.isArray(geometry.coordinates) ||
				geometry.coordinates.length === 0 ||
				!geometry.coordinates.every(isLineStringCoordinates)
			)
				throw spatialContractError(field);
			break;
		case "Polygon":
			if (!isPolygonCoordinates(geometry.coordinates))
				throw spatialContractError(field);
			break;
		case "MultiPolygon":
			if (
				!Array.isArray(geometry.coordinates) ||
				geometry.coordinates.length === 0 ||
				!geometry.coordinates.every(isPolygonCoordinates)
			)
				throw spatialContractError(field);
			break;
		default:
			throw spatialContractError(`${field}.type`, "不在允許清單");
	}
	return geometry as unknown as GeoJsonGeometry;
}

function featureCollectionAt(
	value: unknown,
	field: string,
	allowedGeometryTypes?: ReadonlySet<GeoJsonGeometry["type"]>,
): GeoJsonFeatureCollection {
	const collection = recordAt(value, field);
	if (
		collection.type !== "FeatureCollection" ||
		!Array.isArray(collection.features)
	) {
		throw spatialContractError(field);
	}
	const features = collection.features.map((rawFeature, index) => {
		const featureField = `${field}.features[${index}]`;
		const feature = recordAt(rawFeature, featureField);
		if (feature.type !== "Feature") throw spatialContractError(featureField);
		if (
			feature.id != null &&
			((typeof feature.id !== "string" && typeof feature.id !== "number") ||
				(typeof feature.id === "number" && !Number.isFinite(feature.id)))
		) {
			throw spatialContractError(`${featureField}.id`);
		}
		if (feature.properties !== null) {
			recordAt(feature.properties, `${featureField}.properties`);
		}
		const geometry = geometryAt(feature.geometry, `${featureField}.geometry`);
		if (allowedGeometryTypes && !allowedGeometryTypes.has(geometry.type)) {
			throw spatialContractError(
				`${featureField}.geometry.type`,
				"與圖層用途不符",
			);
		}
		return {
			type: "Feature" as const,
			...(feature.id == null ? {} : { id: feature.id as string | number }),
			properties: feature.properties as UnknownRecord | null,
			geometry,
		};
	});
	return { type: "FeatureCollection", features };
}

function dateAt(value: unknown, field: string): string {
	const date = stringAt(value, field);
	const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
	if (!match) throw spatialContractError(field, "必須是 YYYY-MM-DD");
	const [year, month, day] = match.slice(1).map(Number);
	const parsed = new Date(Date.UTC(year, month - 1, day));
	if (
		parsed.getUTCFullYear() !== year ||
		parsed.getUTCMonth() !== month - 1 ||
		parsed.getUTCDate() !== day
	) {
		throw spatialContractError(field, "不是有效日期");
	}
	return date;
}

function featureProperty(
	feature: GeoJsonFeature,
	keys: readonly string[],
): unknown {
	for (const key of keys) {
		if (feature.properties && Object.hasOwn(feature.properties, key)) {
			return feature.properties[key];
		}
	}
	return undefined;
}

function validateLayerProperties(
	collection: GeoJsonFeatureCollection,
	field: string,
	kind: "market" | "surface" | "contour" | "region",
): void {
	collection.features.forEach((feature, index) => {
		const itemField = `${field}.features[${index}].properties`;
		if (kind === "market") {
			stringAt(
				featureProperty(feature, ["marketCode", "market_code", "market_id"]),
				`${itemField}.marketCode`,
			);
			stringAt(
				featureProperty(feature, ["marketName", "market_name"]),
				`${itemField}.marketName`,
			);
			numberAt(featureProperty(feature, ["p50"]), `${itemField}.p50`, {
				min: 0,
			});
			return;
		}
		if (kind === "surface") {
			numberAt(
				featureProperty(feature, ["priceTwdPerKg", "price_twd_per_kg"]),
				`${itemField}.priceTwdPerKg`,
				{ min: 0 },
			);
			return;
		}
		if (kind === "contour") {
			numberAt(
				featureProperty(feature, [
					"levelPriceTwdPerKg",
					"level_price_twd_per_kg",
				]),
				`${itemField}.levelPriceTwdPerKg`,
				{ min: 0 },
			);
			return;
		}
		// A boundary region may legitimately be unpriced. If a price exists it
		// must be a real non-negative number; null remains an explicit gap.
		stringAt(
			featureProperty(feature, ["regionName", "region_name", "county_name"]),
			`${itemField}.regionName`,
		);
		optionalNumberAt(
			featureProperty(feature, [
				"estimatedP50TwdPerKg",
				"estimated_p50_twd_per_kg",
			]),
			`${itemField}.estimatedP50TwdPerKg`,
			{ min: 0 },
		);
	});
}

function matchesPriceBreak(value: number, breaks: readonly number[]): boolean {
	return breaks.some((priceBreak) => numbersClose(priceBreak, value));
}

function numbersClose(left: number, right: number): boolean {
	return (
		Math.abs(left - right) <=
		Math.max(1e-6, Math.max(Math.abs(left), Math.abs(right)) * 1e-9)
	);
}

function validateContourScale(
	collection: GeoJsonFeatureCollection,
	breaks: readonly number[],
): void {
	collection.features.forEach((feature, index) => {
		const field = `contourLinesGeojson.features[${index}].properties.levelPriceTwdPerKg`;
		const level = numberAt(
			featureProperty(feature, [
				"levelPriceTwdPerKg",
				"level_price_twd_per_kg",
			]),
			field,
			{ min: 0 },
		);
		if (!matchesPriceBreak(level, breaks)) {
			throw spatialContractError(field, "必須來自同一回應的 scale.breaks");
		}
	});
}

function parseSpatialSurfaceData(
	value: unknown,
	selection: PricebookSelection,
	requestedHorizonDays: number,
): SpatialSurfaceData {
	const data = recordAt(value, "payload");
	if (data.status === "unavailable") {
		throw new SpatialSurfaceUnavailableError(data);
	}
	if (typeof data.status !== "string" || !SPATIAL_STATUSES.has(data.status)) {
		throw spatialContractError("status", "不在允許清單");
	}
	if (
		typeof data.displayMode !== "string" ||
		!SPATIAL_DISPLAY_MODES.has(data.displayMode)
	) {
		throw spatialContractError("displayMode", "不在允許清單");
	}
	if (data.currency !== "TWD")
		throw spatialContractError("currency", "必須是 TWD");
	if (data.unit !== "kg") throw spatialContractError("unit", "必須是 kg");

	const expectedSelectionKind = selection.kind === "group" ? "group" : "crop";
	if (data.selectionKind !== expectedSelectionKind) {
		throw spatialContractError("selectionKind", "與查詢品項不一致");
	}
	const horizonDays = numberAt(data.horizonDays, "horizonDays", {
		integer: true,
		min: 1,
		max: 7,
	});
	if (horizonDays !== requestedHorizonDays) {
		throw spatialContractError("horizonDays", "與查詢日期不一致");
	}

	const scaleRecord = recordAt(data.scale, "scale");
	const scalePolicyVersion = stringAt(
		scaleRecord.policyVersion,
		"scale.policyVersion",
	);
	if (!Array.isArray(scaleRecord.breaks)) {
		throw spatialContractError("scale.breaks");
	}
	const breaks = scaleRecord.breaks.map((value, index) =>
		numberAt(value, `scale.breaks[${index}]`, { min: 0 }),
	);
	if (
		data.displayMode !== "table_only" &&
		scalePolicyVersion !== SPATIAL_ADAPTIVE_P50_FLAT_POLICY &&
		breaks.length < 2
	) {
		throw spatialContractError("scale.breaks", "地圖模式至少需要兩個價格門檻");
	}
	if (
		scalePolicyVersion === SPATIAL_ADAPTIVE_P50_FLAT_POLICY &&
		breaks.length !== 0
	) {
		throw spatialContractError(
			"scale.breaks",
			"近乎平坦的價格政策必須是零門檻的單一色帶",
		);
	}
	if (breaks.length > 7) {
		throw spatialContractError(
			"scale.breaks",
			"不得超過前端八色色盤的七個價格門檻",
		);
	}
	if (
		breaks.some(
			(priceBreak, index) => index > 0 && priceBreak <= breaks[index - 1],
		)
	) {
		throw spatialContractError("scale.breaks", "必須嚴格遞增且不可重複");
	}
	const scale: SpatialPriceScale = {
		scaleId: stringAt(scaleRecord.scaleId, "scale.scaleId"),
		policyVersion: scalePolicyVersion,
		breaks,
	};

	const boundaryRecord = recordAt(data.boundary, "boundary");
	if (
		!Array.isArray(boundaryRecord.bboxWgs84) ||
		boundaryRecord.bboxWgs84.length !== 4
	) {
		throw spatialContractError("boundary.bboxWgs84");
	}
	const bboxWgs84 = boundaryRecord.bboxWgs84.map((value, index) =>
		numberAt(value, `boundary.bboxWgs84[${index}]`),
	);
	if (bboxWgs84[0] >= bboxWgs84[2] || bboxWgs84[1] >= bboxWgs84[3]) {
		throw spatialContractError("boundary.bboxWgs84", "範圍上下限顛倒");
	}
	const boundary: SpatialBoundary = {
		version: stringAt(boundaryRecord.version, "boundary.version"),
		digest: stringAt(boundaryRecord.digest, "boundary.digest"),
		bboxWgs84,
		geojson: featureCollectionAt(
			boundaryRecord.geojson,
			"boundary.geojson",
			new Set(["Polygon", "MultiPolygon"]),
		),
	};
	if (boundaryRecord.source != null) {
		const source = recordAt(boundaryRecord.source, "boundary.source");
		boundary.source = {
			label: optionalStringAt(source.label, "boundary.source.label"),
			url: optionalStringAt(source.url, "boundary.source.url"),
			license: optionalStringAt(source.license, "boundary.source.license"),
		};
	}

	const boundaryPriceGeojson = featureCollectionAt(
		data.boundaryPriceGeojson,
		"boundaryPriceGeojson",
		new Set(["Polygon", "MultiPolygon"]),
	);
	const marketPointsGeojson = featureCollectionAt(
		data.marketPointsGeojson,
		"marketPointsGeojson",
		new Set(["Point"]),
	);
	const surfaceCellsGeojson = featureCollectionAt(
		data.surfaceCellsGeojson,
		"surfaceCellsGeojson",
		new Set(["Polygon", "MultiPolygon"]),
	);
	const contourLinesGeojson = featureCollectionAt(
		data.contourLinesGeojson,
		"contourLinesGeojson",
		new Set(["LineString", "MultiLineString"]),
	);
	validateLayerProperties(
		boundaryPriceGeojson,
		"boundaryPriceGeojson",
		"region",
	);
	validateLayerProperties(marketPointsGeojson, "marketPointsGeojson", "market");
	validateLayerProperties(
		surfaceCellsGeojson,
		"surfaceCellsGeojson",
		"surface",
	);
	validateLayerProperties(
		contourLinesGeojson,
		"contourLinesGeojson",
		"contour",
	);
	validateContourScale(contourLinesGeojson, breaks);

	if (
		data.displayMode !== "table_only" &&
		boundary.geojson.features.length === 0
	) {
		throw spatialContractError("boundary.geojson", "地圖模式不可為空");
	}
	if (
		(data.displayMode === "contours_and_points" &&
			(contourLinesGeojson.features.length === 0 ||
				marketPointsGeojson.features.length === 0)) ||
		(data.displayMode === "surface_and_points" &&
			(surfaceCellsGeojson.features.length === 0 ||
				marketPointsGeojson.features.length === 0)) ||
		(data.displayMode === "points_only" &&
			marketPointsGeojson.features.length === 0)
	) {
		throw spatialContractError("displayMode", "與實際空間圖層不一致");
	}

	if (!Array.isArray(data.marketPrices) || data.marketPrices.length === 0) {
		throw spatialContractError("marketPrices", "不可為空");
	}
	const marketPrices = data.marketPrices.map((value, index) => {
		const field = `marketPrices[${index}]`;
		const market = recordAt(value, field);
		const normalized: SpatialMarketPrice = {
			marketCode: stringAt(market.marketCode, `${field}.marketCode`),
			marketName: stringAt(market.marketName, `${field}.marketName`),
			categoryCode: optionalStringAt(
				market.categoryCode,
				`${field}.categoryCode`,
			),
			predictionStatus: stringAt(
				market.predictionStatus,
				`${field}.predictionStatus`,
			),
			marketStatus: stringAt(market.marketStatus, `${field}.marketStatus`),
			p10: optionalNumberAt(market.p10, `${field}.p10`, { min: 0 }),
			p50: optionalNumberAt(market.p50, `${field}.p50`, { min: 0 }),
			p90: optionalNumberAt(market.p90, `${field}.p90`, { min: 0 }),
			recentVolumeKg: optionalNumberAt(
				market.recentVolumeKg,
				`${field}.recentVolumeKg`,
				{ min: 0 },
			),
			usedForSurface: booleanAt(
				market.usedForSurface,
				`${field}.usedForSurface`,
			),
		};
		if (normalized.usedForSurface && normalized.p50 == null) {
			throw spatialContractError(`${field}.p50`, "參與價格面時不可缺值");
		}
		return normalized;
	});

	const statisticsRecord = recordAt(data.statistics, "statistics");
	const statistics: SpatialSurfaceStatistics = {
		p10: optionalNumberAt(statisticsRecord.p10, "statistics.p10", { min: 0 }),
		p50: optionalNumberAt(statisticsRecord.p50, "statistics.p50", { min: 0 }),
		p90: optionalNumberAt(statisticsRecord.p90, "statistics.p90", { min: 0 }),
		minimumP50: optionalNumberAt(
			statisticsRecord.minimumP50,
			"statistics.minimumP50",
			{ min: 0 },
		),
		maximumP50: optionalNumberAt(
			statisticsRecord.maximumP50,
			"statistics.maximumP50",
			{ min: 0 },
		),
		pricedMarketCount: numberAt(
			statisticsRecord.pricedMarketCount,
			"statistics.pricedMarketCount",
			{ integer: true, min: 0 },
		),
		closedMarketCount: numberAt(
			statisticsRecord.closedMarketCount,
			"statistics.closedMarketCount",
			{ integer: true, min: 0 },
		),
		totalMarketCount: numberAt(
			statisticsRecord.totalMarketCount,
			"statistics.totalMarketCount",
			{ integer: true, min: 1 },
		),
		recentVolumeKg: optionalNumberAt(
			statisticsRecord.recentVolumeKg,
			"statistics.recentVolumeKg",
			{ min: 0 },
		),
		surfaceCellCount: numberAt(
			statisticsRecord.surfaceCellCount,
			"statistics.surfaceCellCount",
			{ integer: true, min: 0 },
		),
		contourSegmentCount: numberAt(
			statisticsRecord.contourSegmentCount,
			"statistics.contourSegmentCount",
			{ integer: true, min: 0 },
		),
	};
	if (
		statistics.pricedMarketCount > statistics.totalMarketCount ||
		statistics.closedMarketCount > statistics.totalMarketCount
	) {
		throw spatialContractError("statistics", "市場計數彼此矛盾");
	}
	const hasCompleteP50Summary =
		statistics.minimumP50 != null &&
		statistics.p50 != null &&
		statistics.maximumP50 != null;
	if (statistics.pricedMarketCount > 0 && !hasCompleteP50Summary) {
		throw spatialContractError(
			"statistics",
			"有價格市場時必須提供 minimumP50、p50 與 maximumP50",
		);
	}
	if (
		statistics.minimumP50 != null &&
		statistics.p50 != null &&
		statistics.maximumP50 != null &&
		(statistics.minimumP50 > statistics.p50 ||
			statistics.p50 > statistics.maximumP50)
	) {
		throw spatialContractError("statistics", "P50 下限、中位與上限順序矛盾");
	}
	const pricedP50Values = marketPrices.flatMap((market) =>
		market.p50 == null ? [] : [market.p50],
	);
	if (pricedP50Values.length !== statistics.pricedMarketCount) {
		throw spatialContractError(
			"statistics.pricedMarketCount",
			"與 marketPrices 中有 P50 的市場數量不一致",
		);
	}
	if (
		hasCompleteP50Summary &&
		pricedP50Values.length > 0 &&
		(!numbersClose(
			statistics.minimumP50 as number,
			Math.min(...pricedP50Values),
		) ||
			!numbersClose(
				statistics.maximumP50 as number,
				Math.max(...pricedP50Values),
			))
	) {
		throw spatialContractError(
			"statistics",
			"P50 下限或上限與 marketPrices 不一致",
		);
	}
	if (
		scale.policyVersion.startsWith(`${SPATIAL_ADAPTIVE_P50_POLICY}-`) &&
		scale.policyVersion !== SPATIAL_ADAPTIVE_P50_FLAT_POLICY
	) {
		throw spatialContractError(
			"scale.policyVersion",
			"不在支援的自適應政策清單",
		);
	}
	if (scale.policyVersion === SPATIAL_ADAPTIVE_P50_POLICY) {
		if (
			!hasCompleteP50Summary ||
			statistics.minimumP50 === statistics.maximumP50 ||
			breaks.some(
				(priceBreak) =>
					priceBreak <= (statistics.minimumP50 as number) ||
					priceBreak >= (statistics.maximumP50 as number),
			)
		) {
			throw spatialContractError(
				"scale.breaks",
				"自適應價格門檻必須位於同一回應的 P50 上下限內",
			);
		}
	}
	if (
		scale.policyVersion === SPATIAL_ADAPTIVE_P50_FLAT_POLICY &&
		!hasCompleteP50Summary
	) {
		throw spatialContractError(
			"statistics",
			"近乎平坦的價格尺度仍必須提供完整 P50 摘要",
		);
	}

	const lineageRecord = recordAt(data.lineage, "lineage");
	const lineage: SpatialSurfaceLineage = {
		artifactId: stringAt(lineageRecord.artifactId, "lineage.artifactId"),
		generatedAt: optionalStringAt(
			lineageRecord.generatedAt,
			"lineage.generatedAt",
		),
		sourceDataAsOf: optionalStringAt(
			lineageRecord.sourceDataAsOf,
			"lineage.sourceDataAsOf",
		),
		maxFeatureAvailableAt: optionalStringAt(
			lineageRecord.maxFeatureAvailableAt,
			"lineage.maxFeatureAvailableAt",
		),
		modelVersion: stringAt(lineageRecord.modelVersion, "lineage.modelVersion"),
		datasetDigest: stringAt(
			lineageRecord.datasetDigest,
			"lineage.datasetDigest",
		),
		boundaryDigest: stringAt(
			lineageRecord.boundaryDigest,
			"lineage.boundaryDigest",
		),
		schemaVersion: optionalStringAt(
			lineageRecord.schemaVersion,
			"lineage.schemaVersion",
		),
	};
	if (lineage.boundaryDigest !== boundary.digest) {
		throw spatialContractError("lineage.boundaryDigest", "與邊界摘要不一致");
	}

	return {
		status: data.status as SpatialSurfaceData["status"],
		displayMode: data.displayMode as SpatialSurfaceData["displayMode"],
		selectionKind: data.selectionKind as SpatialSurfaceData["selectionKind"],
		targetDate: dateAt(data.targetDate, "targetDate"),
		horizonDays,
		currency: "TWD",
		unit: "kg",
		scale,
		boundary,
		boundaryPriceGeojson,
		marketPointsGeojson,
		surfaceCellsGeojson,
		contourLinesGeojson,
		marketPrices,
		statistics,
		qualityReasonCodes: stringArrayAt(
			data.qualityReasonCodes,
			"qualityReasonCodes",
		),
		disclosures: stringArrayAt(data.disclosures, "disclosures"),
		supportedLandAreaRatio: optionalNumberAt(
			data.supportedLandAreaRatio,
			"supportedLandAreaRatio",
			{ min: 0, max: 1 },
		),
		excludedMarketCodes: stringArrayAt(
			data.excludedMarketCodes,
			"excludedMarketCodes",
		),
		lineage,
	};
}

/** `/sellers/pricebook/forecast-curve` — 未來 7 天預測曲線，純 fetcher。
 *  data 缺失時退回 status="unavailable"、空 points——跟後端「業務結果一律
 *  走 status」的哲學一致，不假裝有資料。 */
export async function fetchForecastCurve(
	selection: PricebookSelection,
): Promise<ForecastCurveData> {
	const env = await apiClient.authed.get<ForecastCurveData>(
		"/sellers/pricebook/forecast-curve",
		{ query: selectionQuery(selection) },
	);
	const data = env.data;
	if (!data) {
		return {
			status: "unavailable",
			modelVersion: "",
			points: [],
			marketComparison: [],
		};
	}
	return { ...data, marketComparison: data.marketComparison ?? [] };
}

/** `/sellers/pricebook/spatial-surface` — audited, category-aware spatial
 * artifacts for MapLibre. This endpoint intentionally returns no client-side
 * fallback: missing geography/surface data must stay unavailable, never be
 * guessed from marketComparison. */
export async function fetchSpatialSurface(
	selection: PricebookSelection,
	horizonDays: number,
	signal?: AbortSignal,
): Promise<SpatialSurfaceData> {
	const env = await apiClient.authed.get<unknown>(
		"/sellers/pricebook/spatial-surface",
		{
			query: { ...spatialSelectionQuery(selection), horizonDays },
			signal,
		},
	);
	if (!env.data) {
		throw new Error("空間行情服務未回傳資料");
	}
	return parseSpatialSurfaceData(env.data, selection, horizonDays);
}
