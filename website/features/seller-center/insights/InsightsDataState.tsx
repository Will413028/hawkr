"use client";

import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import type { HistoryRange } from "./priceTrendChartMath";

type InsightsSkeletonVariant = "chart" | "table" | "comparison";

const SKELETON_ITEM_COUNT: Record<InsightsSkeletonVariant, number> = {
	chart: 8,
	table: 6,
	comparison: 5,
};

const SKELETON_KEYS: Record<InsightsSkeletonVariant, string[]> = {
	chart: [
		"axis",
		"line-1",
		"line-2",
		"line-3",
		"line-4",
		"line-5",
		"legend",
		"caption",
	],
	table: ["row-1", "row-2", "row-3", "row-4", "row-5", "row-6"],
	comparison: ["market-1", "market-2", "market-3", "market-4", "market-5"],
};

export function InsightsMetricSkeleton({ label }: { label: string }) {
	return (
		<span
			className="seller-price-lookup__metric-skeleton"
			aria-label={`${label}讀取中`}
			role="status"
		>
			<span aria-hidden="true" />
		</span>
	);
}

export function InsightsPanelSkeleton({
	variant,
	label,
	chartKind = "history",
	tableKind = "observed",
}: {
	variant: InsightsSkeletonVariant;
	label: string;
	chartKind?: "history" | "forecast";
	tableKind?: "observed" | "forecast";
}) {
	if (variant === "table")
		return <InsightsTableSkeleton kind={tableKind} label={label} />;
	if (variant === "comparison")
		return (
			<div
				className="seller-price-lookup__comparison"
				role="status"
				aria-busy="true"
				aria-label={label}
			>
				<p className="seller-price-lookup__comparison-unit">
					<span className="seller-skeleton__bone h-3 w-3/4" />
				</p>
				{SKELETON_KEYS.comparison.map((key) => (
					<div
						key={key}
						className="seller-price-lookup__comparison-row"
						aria-hidden="true"
					>
						<span>
							<span className="seller-skeleton__bone h-3 w-20" />
						</span>
						<span className="seller-skeleton__bone h-2 w-3/4" />
						<b>
							<span className="seller-skeleton__bone h-3 w-12" />
						</b>
					</div>
				))}
			</div>
		);
	return (
		<div
			className={`seller-price-lookup__state-skeleton is-${variant}`}
			aria-busy="true"
			role="status"
		>
			<span className="sr-only">{label}</span>
			{variant === "chart" &&
				(chartKind === "forecast" ? (
					<div
						className="seller-price-lookup__chart-toolbar"
						aria-hidden="true"
						inert
					>
						<p>
							<span className="seller-skeleton__bone h-3 w-36" />
						</p>
						<div className="seller-price-lookup__chart-legend seller-price-lookup__chart-legend--interactive">
							{["P50 中位線", "P10～P90 區間"].map((text) => (
								<button
									key={text}
									type="button"
									tabIndex={-1}
									className="seller-price-lookup__chart-legend-item seller-skeleton__bone"
								>
									<span className="invisible">{text}</span>
								</button>
							))}
						</div>
					</div>
				) : (
					<div className="seller-price-lookup__chart-legend" aria-hidden="true">
						<span className="seller-price-lookup__chart-legend-item seller-skeleton__bone">
							<span className="invisible">官方實際平均價</span>
						</span>
					</div>
				))}
			<div
				className="seller-price-lookup__state-skeleton-body"
				aria-hidden="true"
			>
				{SKELETON_KEYS[variant]
					.slice(0, SKELETON_ITEM_COUNT[variant])
					.map((key) => (
						<span key={key} />
					))}
			</div>
		</div>
	);
}

export function InsightsInlineError({
	title,
	description,
	retryLabel,
	retrying,
	onRetry,
}: {
	title: string;
	description: string;
	retryLabel: string;
	retrying: boolean;
	onRetry: () => void;
}) {
	return (
		<div className="seller-price-lookup__inline-error" role="alert">
			<span
				className="seller-price-lookup__inline-state-icon"
				aria-hidden="true"
			>
				<AppIcon name="danger" size={20} />
			</span>
			<div>
				<strong>{title}</strong>
				<p>{description}</p>
			</div>
			<Button
				type="button"
				variant="bordered"
				color="danger"
				size="sm"
				isLoading={retrying}
				loadingLabel="重新載入中…"
				onClick={onRetry}
			>
				{retryLabel}
			</Button>
		</div>
	);
}

export function InsightsRefreshNotice({
	label,
	retrying,
	onRetry,
}: {
	label: string;
	retrying: boolean;
	onRetry: () => void;
}) {
	return (
		<div className="seller-price-lookup__refresh-notice" role="status">
			<span
				className="seller-price-lookup__inline-state-icon"
				aria-hidden="true"
			>
				<AppIcon name="warning" size={19} />
			</span>
			<p>
				<strong>{label}更新失敗</strong>
				<span>目前仍顯示上次成功載入的資料。</span>
			</p>
			<Button
				type="button"
				variant="light"
				color="warning"
				size="sm"
				isLoading={retrying}
				loadingLabel="重新載入中…"
				onClick={onRetry}
			>
				再試一次
			</Button>
		</div>
	);
}

/** Retain the category title, ledger, disclaimer and range rail while data loads. */
export function CategoryRollupSkeleton({
	label,
	level,
	historyRange,
}: {
	label: string;
	level: 1 | 2;
	historyRange: HistoryRange;
}) {
	return (
		<section
			className="seller-price-lookup__category-rollup"
			aria-busy="true"
			aria-label={`${label}分類彙整走勢載入中`}
		>
			<h3>{`${label}（L${level}）分類彙整走勢`}</h3>
			<dl className="seller-price-lookup__metric-ledger">
				{[
					"最新大類彙整價",
					"D+1 模型中位價",
					"D+1 預測區間寬度",
					"近 30 日成交量",
				].map((text, index) => (
					<div
						key={text}
						className={`seller-price-lookup__metric ${index === 0 ? "seller-price-lookup__metric--primary" : ""}`}
					>
						<dt>{text}</dt>
						<dd>
							<span className="seller-skeleton__bone h-4 w-20" />
						</dd>
						{index !== 1 && (
							<dd className="contents">
								<small>
									<span className="seller-skeleton__bone h-3 w-24" />
								</small>
							</dd>
						)}
					</div>
				))}
			</dl>
			<p className="seller-price-lookup__category-rollup-disclaimer">
				<span className="seller-skeleton__bone h-4 w-3/4" />
			</p>
			<section aria-label="官方批發實際行情載入中">
				<fieldset
					className="seller-price-lookup__range-toggle"
					aria-label="時間範圍"
					disabled
				>
					{([7, 30, "all"] as const).map((range) => (
						<button
							key={range}
							type="button"
							aria-pressed={historyRange === range}
						>
							{range === "all" ? "全部" : `${range} 日`}
						</button>
					))}
				</fieldset>
				<InsightsPanelSkeleton variant="chart" label={`${label}行情彙整`} />
			</section>
		</section>
	);
}

function InsightsTableSkeleton({
	kind,
	label,
}: {
	kind: "observed" | "forecast";
	label: string;
}) {
	const forecast = kind === "forecast";
	const columns = forecast
		? ["目標日", "資料狀態", "預測下界 P10", "模型中位 P50", "預測上界 P90"]
		: [
				"交易日",
				"市場",
				"平均價",
				"官方上價",
				"官方中價",
				"官方下價",
				"成交量",
			];
	const rows = forecast ? [1, 2, 3, 4, 5, 6, 7] : [1, 2, 3, 4, 5, 6];
	const table = (
		<section
			className={`seller-price-lookup__table-scroll seller-price-lookup__table-scroll--${kind}`}
			aria-label={label}
		>
			<table
				className={`seller-price-lookup__table seller-price-lookup__table--${kind}`}
			>
				<thead>
					<tr>
						{columns.map((column) => (
							<th key={column} scope="col">
								{column}
							</th>
						))}
					</tr>
				</thead>
				<tbody>
					{rows.map((row) => (
						<tr key={row}>
							{columns.map((column, index) => (
								<td key={column}>
									<span className="seller-skeleton__bone h-3 w-16" />
									{forecast && index === 0 && (
										<small>
											<span className="seller-skeleton__bone h-3 w-8" />
										</small>
									)}
								</td>
							))}
						</tr>
					))}
				</tbody>
			</table>
		</section>
	);
	return (
		<div role="status" aria-busy="true" aria-label={label}>
			{forecast ? (
				<div className="seller-price-lookup__forecast-detail">
					{table}
					<ol
						className="seller-price-lookup__forecast-list"
						aria-label="七日預測明細載入中"
					>
						{rows.map((row) => (
							<li key={row}>
								<header>
									<span>
										<span className="seller-skeleton__bone h-4 w-24" />
										<small>D+{row}</small>
									</span>
									<strong>
										<span className="seller-skeleton__bone h-3 w-16" />
									</strong>
								</header>
								<dl>
									<div className="seller-price-lookup__forecast-median">
										<dt>模型中位 P50</dt>
										<dd>
											<span className="seller-skeleton__bone h-4 w-16" />
										</dd>
									</div>
									<div>
										<dt>預測區間 P10～P90</dt>
										<dd>
											<span className="seller-skeleton__bone h-4 w-24" />
										</dd>
									</div>
								</dl>
							</li>
						))}
					</ol>
				</div>
			) : (
				<details className="seller-price-lookup__details" open>
					<summary>正在載入實際行情明細</summary>
					{table}
				</details>
			)}
		</div>
	);
}
