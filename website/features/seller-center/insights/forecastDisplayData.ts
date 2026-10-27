import type {
	ForecastCurvePoint,
	MarketComparisonEntry,
} from "@/lib/pricebook/repo";
import type { InsightsSeriesCapabilities } from "./InsightsWorkspaceTabs";

const PRICE_READY_STATUSES = new Set(["ready", "partial"]);

export type ForecastDisplayIssue =
	| "invalid_points"
	| "no_price_ready_point"
	| null;

export interface ForecastDisplayData {
	status: string | null;
	isPriceReady: boolean;
	issue: ForecastDisplayIssue;
	/** 後端送來的單位代碼（kg／catty／100kg）。缺席時顯示端一律顯示
	 * 「單位待確認」，不假設公斤——多來源之後這條曲線可能是台斤價。 */
	unit?: string;
	/** 價格層級。🔴 毛豬回的是活體拍賣均價，不是豬肉
	 * 分切零售價；顯示價格卻不說層級就是讓賣家誤讀。 */
	priceLayer?: string;
	/** 這個預測值的來源：model（模型分位推估）或 source_passthrough
	 * （直接採用來源公布的成交均價）。後者只有 P50、只有 D+1。 */
	derivation?: string;
	/** 這條序列能安全提供哪些能力，由後端宣告。缺席時（舊快取、尚未選定品項）
	 * 呼叫端維持既有行為顯示全部分頁。 */
	capabilities?: InsightsSeriesCapabilities;
	points: ForecastCurvePoint[];
	marketComparison: MarketComparisonEntry[];
	/** Validated crop-market roster for the optional filter. Kept even when
	 * the aggregate forecast is not price-ready; comparison prices still stay
	 * behind marketComparison's stricter gate. */
	marketRoster: MarketComparisonEntry[];
}

function isNullableFiniteNumber(value: unknown) {
	return value == null || (typeof value === "number" && Number.isFinite(value));
}

function hasValidVisibleQuantiles(point: Record<string, unknown>) {
	if (!PRICE_READY_STATUSES.has(point.status as string)) return true;
	const visiblePrices = [point.p10, point.p50, point.p90];
	if (
		visiblePrices.some(
			(value) =>
				value != null &&
				(typeof value !== "number" || !Number.isFinite(value) || value < 0),
		)
	) {
		return false;
	}

	const p10 = typeof point.p10 === "number" ? point.p10 : undefined;
	const p50 = typeof point.p50 === "number" ? point.p50 : undefined;
	const p90 = typeof point.p90 === "number" ? point.p90 : undefined;
	return (
		(p10 == null || p50 == null || p10 <= p50) &&
		(p50 == null || p90 == null || p50 <= p90) &&
		(p10 == null || p90 == null || p10 <= p90)
	);
}

function isValidForecastCurvePoint(value: unknown) {
	if (!value || typeof value !== "object") return false;
	const point = value as Record<string, unknown>;
	return (
		typeof point.targetDate === "string" &&
		/^\d{4}-\d{2}-\d{2}$/.test(point.targetDate) &&
		typeof point.horizonDays === "number" &&
		Number.isInteger(point.horizonDays) &&
		point.horizonDays >= 1 &&
		point.horizonDays <= 7 &&
		typeof point.status === "string" &&
		point.status.length > 0 &&
		isNullableFiniteNumber(point.p10) &&
		isNullableFiniteNumber(point.p50) &&
		isNullableFiniteNumber(point.p90) &&
		hasValidVisibleQuantiles(point)
	);
}

function finiteNumberOrUndefined(value: unknown) {
	return typeof value === "number" && Number.isFinite(value)
		? value
		: undefined;
}

/** Copy only the validated point fields and remove any prices attached to a
 * closed, stale, unavailable, or otherwise non-price-ready day. */
function normalizePoint(value: unknown): ForecastCurvePoint {
	const point = value as Record<string, unknown>;
	const canShowPointPrice = PRICE_READY_STATUSES.has(point.status as string);
	return {
		targetDate: point.targetDate as string,
		horizonDays: point.horizonDays as number,
		status: point.status as string,
		p10: canShowPointPrice ? finiteNumberOrUndefined(point.p10) : undefined,
		p50: canShowPointPrice ? finiteNumberOrUndefined(point.p50) : undefined,
		p90: canShowPointPrice ? finiteNumberOrUndefined(point.p90) : undefined,
	};
}

function isValidMarketComparisonEntry(value: unknown) {
	if (!value || typeof value !== "object") return false;
	const entry = value as Record<string, unknown>;
	return (
		typeof entry.marketCode === "string" &&
		entry.marketCode.length > 0 &&
		typeof entry.marketName === "string" &&
		entry.marketName.length > 0 &&
		isNullableFiniteNumber(entry.p50) &&
		(entry.p50 == null || (entry.p50 as number) >= 0) &&
		typeof entry.isApproximate === "boolean"
	);
}

function normalizeMarketComparisonEntry(value: unknown): MarketComparisonEntry {
	const entry = value as Record<string, unknown>;
	return {
		marketCode: entry.marketCode as string,
		marketName: entry.marketName as string,
		p50: finiteNumberOrUndefined(entry.p50),
		...(typeof entry.status === "string" ? { status: entry.status } : {}),
		isApproximate: entry.isApproximate === true,
	};
}

/**
 * Final browser-side forecast display policy. The repository validates the
 * transport contract; this second boundary protects the UI from stale cached
 * or manually injected data. A ready/partial status alone is never enough:
 * the point array must be non-empty, structurally valid, and contain at least
 * one price-ready day with a finite P50.
 */
export function resolveForecastDisplayData(data: unknown): ForecastDisplayData {
	const record =
		data && typeof data === "object" ? (data as Record<string, unknown>) : null;
	const status = typeof record?.status === "string" ? record.status : null;
	const text = (value: unknown): string | undefined =>
		typeof value === "string" && value.trim().length > 0
			? value.trim()
			: undefined;
	const unit = text(record?.unit);
	const capabilities = normalizeCapabilities(record?.capabilities);
	const priceLayer = text(record?.priceLayer);
	// derivation 住在每個 point 上，但一條曲線內必定一致（同一條序列只有一種
	// 產生方式），取第一個有值的即可。
	const derivation = Array.isArray(record?.points)
		? record.points
				.map((p) =>
					p && typeof p === "object"
						? text((p as Record<string, unknown>).derivation)
						: undefined,
				)
				.find((v) => v != null)
		: undefined;
	const hasPriceReadyStatus =
		status != null && PRICE_READY_STATUSES.has(status);
	const rawPoints = record?.points;
	const hasPointArray = Array.isArray(rawPoints);
	const hasNonEmptyPoints = hasPointArray && rawPoints.length > 0;
	const hasValidPoints =
		hasNonEmptyPoints && rawPoints.every(isValidForecastCurvePoint);
	const normalizedPoints = hasValidPoints
		? rawPoints
				.map(normalizePoint)
				.sort(
					(a, b) =>
						a.horizonDays - b.horizonDays ||
						a.targetDate.localeCompare(b.targetDate),
				)
		: [];
	const hasPriceReadyPoint = normalizedPoints.some(
		(point) =>
			PRICE_READY_STATUSES.has(point.status) &&
			typeof point.p50 === "number" &&
			Number.isFinite(point.p50),
	);
	const isPriceReady =
		hasPriceReadyStatus && hasValidPoints && hasPriceReadyPoint;

	let issue: ForecastDisplayIssue = null;
	if (
		hasPriceReadyStatus &&
		(!hasPointArray || (hasNonEmptyPoints && !hasValidPoints))
	) {
		issue = "invalid_points";
	} else if (
		hasPriceReadyStatus &&
		(!hasNonEmptyPoints || !hasPriceReadyPoint)
	) {
		issue = "no_price_ready_point";
	}

	const rawMarketComparison = record?.marketComparison;
	const hasValidMarketComparison =
		Array.isArray(rawMarketComparison) &&
		rawMarketComparison.every(isValidMarketComparisonEntry);
	const marketComparison =
		isPriceReady && hasValidMarketComparison
			? rawMarketComparison.map(normalizeMarketComparisonEntry)
			: [];
	const marketRoster = hasValidMarketComparison
		? rawMarketComparison.map(normalizeMarketComparisonEntry)
		: [];

	return {
		status,
		isPriceReady,
		issue,
		unit,
		priceLayer,
		derivation,
		capabilities,
		points: isPriceReady ? normalizedPoints : [],
		marketComparison,
		marketRoster,
	};
}

/** 解析後端宣告的序列能力。
 *
 * 兩個布林值缺席時視為 false 而不是 true——這條路徑上「不確定」應該往保守的
 * 方向倒：顯示一個其實不可用的分頁，使用者點進去會撞到空面板或錯誤；少顯示
 * 一個分頁只是少一個入口。整個 capabilities 物件缺席才是「沒宣告」，
 * 由呼叫端維持既有行為。 */
function normalizeCapabilities(
	value: unknown,
): InsightsSeriesCapabilities | undefined {
	if (!value || typeof value !== "object") return undefined;
	const record = value as Record<string, unknown>;
	return {
		marketComparison: record.marketComparison === true,
		spatial: record.spatial === true,
		reasonCodes: Array.isArray(record.reasonCodes)
			? record.reasonCodes.filter(
					(code): code is string => typeof code === "string",
				)
			: [],
	};
}
