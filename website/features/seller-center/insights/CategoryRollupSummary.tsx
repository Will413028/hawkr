"use client";

import { useCategoryPriceRollup } from "@/lib/hooks/useCategoryPriceRollup";
import { computeRollupChange, type RollupChange } from "./categoryRollupFormat";
import { categoryRollupPointsToObservations } from "./categoryRollupObservations";
import { ForecastBandChart } from "./ForecastBandChart";
import { HistoricalPriceChart } from "./HistoricalPriceChart";
import {
	CategoryRollupSkeleton,
	InsightsInlineError,
} from "./InsightsDataState";
import type { HistoryRange } from "./priceTrendChartMath";

export interface CategoryRollupSummaryProps {
	categoryId: string;
	level: 1 | 2;
	label: string;
	historyRange: HistoryRange;
	onHistoryRangeChange: (range: HistoryRange) => void;
}

function formatRollupPrice(value: number | undefined): string {
	if (value == null) return "—";
	return `NT$ ${value.toLocaleString("zh-TW", { maximumFractionDigits: 1 })}`;
}

function formatRollupChangeText(change: RollupChange | null): string {
	if (!change) return "—";
	const sign = change.absolute > 0 ? "+" : "";
	const absoluteText = change.absolute.toLocaleString("zh-TW", {
		maximumFractionDigits: 1,
	});
	const percentText = change.percent.toLocaleString("zh-TW", {
		maximumFractionDigits: 1,
	});
	return `${sign}${absoluteText} 元・${sign}${percentText}%`;
}

const RANGE_OPTIONS: Array<{ value: HistoryRange; label: string }> = [
	{ value: 7, label: "7 日" },
	{ value: 30, label: "30 日" },
	{ value: "all", label: "全部" },
];

export function CategoryRollupSummary({
	categoryId,
	level,
	label,
	historyRange,
	onHistoryRangeChange,
}: CategoryRollupSummaryProps) {
	const rollup = useCategoryPriceRollup(categoryId);

	if (rollup.loading && !rollup.data) {
		return (
			<CategoryRollupSkeleton
				label={label}
				level={level}
				historyRange={historyRange}
			/>
		);
	}

	if (rollup.error) {
		return (
			<InsightsInlineError
				title="無法載入分類彙整價格"
				description={rollup.error}
				retryLabel="重新載入"
				retrying={rollup.loading}
				onRetry={rollup.retry}
			/>
		);
	}

	if (!rollup.data || rollup.data.status === "unavailable") {
		return (
			<p className="seller-price-lookup__category-rollup-empty" role="status">
				{`「${label}」目前沒有可用的彙整行情資料。`}
			</p>
		);
	}

	const { latestPrice, prevPrice, points, sourceGroupCount, forecastPoints } =
		rollup.data;
	const change = computeRollupChange(latestPrice, prevPrice);
	const horizon1 = forecastPoints.find((p) => p.horizonDays === 1) ?? null;
	const intervalWidth =
		horizon1?.p10 != null && horizon1?.p90 != null
			? horizon1.p90 - horizon1.p10
			: null;
	const observations = categoryRollupPointsToObservations(points);

	return (
		<section
			className="seller-price-lookup__category-rollup"
			aria-label={`${label}分類彙整走勢`}
		>
			<h3>{`${label}（L${level}）分類彙整走勢`}</h3>
			<dl className="seller-price-lookup__metric-ledger">
				<div className="seller-price-lookup__metric seller-price-lookup__metric--primary">
					<dt>最新大類彙整價</dt>
					<dd>{formatRollupPrice(latestPrice)}</dd>
					<dd className="contents">
						<small>{formatRollupChangeText(change)}</small>
					</dd>
				</div>
				<div className="seller-price-lookup__metric">
					<dt>D+1 模型中位價</dt>
					<dd>{formatRollupPrice(horizon1?.p50)}</dd>
				</div>
				<div className="seller-price-lookup__metric">
					<dt>D+1 預測區間寬度</dt>
					<dd>{formatRollupPrice(intervalWidth ?? undefined)}</dd>
					<dd className="contents">
						<small>P10～P90，非官方上／下價</small>
					</dd>
				</div>
				<div className="seller-price-lookup__metric">
					<dt>近 30 日成交量</dt>
					<dd>—</dd>
					<dd className="contents">
						<small>大類層級不提供</small>
					</dd>
				</div>
			</dl>
			<p className="seller-price-lookup__category-rollup-disclaimer">
				{`來源 ${sourceGroupCount} 個大類平均，不代表單一品項或市場價格。`}
			</p>
			<section aria-label="官方批發實際行情">
				<fieldset
					aria-label="時間範圍"
					className="seller-price-lookup__range-toggle"
				>
					{RANGE_OPTIONS.map((opt) => (
						<button
							key={String(opt.value)}
							type="button"
							aria-pressed={historyRange === opt.value}
							onClick={() => onHistoryRangeChange(opt.value)}
						>
							{opt.label}
						</button>
					))}
				</fieldset>
				<HistoricalPriceChart
					observations={observations}
					range={historyRange}
				/>
			</section>
			{forecastPoints.length > 0 ? (
				<section aria-label="D+1～D+7 條件式預測">
					<ForecastBandChart forecastPoints={forecastPoints} />
				</section>
			) : null}
		</section>
	);
}
