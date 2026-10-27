"use client";

import * as React from "react";
import {
	Area,
	CartesianGrid,
	ComposedChart,
	Line,
	ResponsiveContainer,
	Tooltip,
	XAxis,
	YAxis,
} from "recharts";
import type { ForecastCurvePoint } from "@/lib/pricebook/repo";
import { formatPrice } from "./insightsFormat";
import { computeValueRange } from "./priceTrendChartMath";
import { usePrefersReducedMotion } from "./usePrefersReducedMotion";

interface ForecastBandChartProps {
	forecastPoints: ForecastCurvePoint[];
	height?: number;
}

interface ForecastRow {
	date: string;
	horizonDays: number;
	status: string;
	p10?: number;
	p50?: number;
	p90?: number;
	bandBase?: number;
	bandRange?: number;
}

interface TooltipEntry<T> {
	payload?: T;
}

function shortDate(date: string): string {
	return date.slice(5).replace("-", "/");
}

function ForecastTooltip({
	active,
	payload,
}: {
	active?: boolean;
	payload?: TooltipEntry<ForecastRow>[];
}) {
	const row = payload?.[0]?.payload;
	if (!active || !row) return null;
	return (
		<div
			className="seller-price-lookup__chart-tooltip seller-price-lookup__chart-tooltip--recharts"
			role="status"
			aria-live="polite"
			aria-atomic="true"
		>
			<strong>
				{row.date} · D+{row.horizonDays}
			</strong>
			<span>P50 中位價 {formatPrice(row.p50)}／kg</span>
			<small>
				P10～P90 {formatPrice(row.p10)}～{formatPrice(row.p90)}
			</small>
			<small>狀態：{row.status || "未提供"}</small>
		</div>
	);
}

/** Original Pricebook Recharts stack: a togglable P50 line and stacked
 * P10–P90 band. It never fills missing days or treats a conditional forecast
 * as an official observed price. */
export function ForecastBandChart({
	forecastPoints,
	height = 260,
}: ForecastBandChartProps) {
	const [showMedian, setShowMedian] = React.useState(true);
	const [showBand, setShowBand] = React.useState(true);
	const reducedMotion = usePrefersReducedMotion();
	const gradientId = React.useId().replaceAll(":", "");
	const rows = React.useMemo<ForecastRow[]>(
		() =>
			forecastPoints
				.map((point) => ({
					date: point.targetDate,
					horizonDays: point.horizonDays,
					status: point.status,
					p10: point.p10,
					p50: point.p50,
					p90: point.p90,
					bandBase: point.p10,
					bandRange:
						point.p10 != null && point.p90 != null
							? point.p90 - point.p10
							: undefined,
				}))
				.sort((a, b) => a.date.localeCompare(b.date)),
		[forecastPoints],
	);
	const readyDays = rows.filter((row) => row.p50 != null).length;
	const valueRange = computeValueRange(
		rows.flatMap((row) =>
			[row.p10, row.p50, row.p90].filter(
				(value): value is number => value != null,
			),
		),
	);

	if (!readyDays || !valueRange) {
		return (
			<p className="seller-price-lookup__chart-empty">
				目前沒有可顯示的預測資料。
			</p>
		);
	}

	return (
		<div className="seller-price-lookup__chart">
			<div className="seller-price-lookup__chart-toolbar">
				<p aria-live="polite">{readyDays}/7 日具有可用 P50 價格</p>
				<fieldset className="seller-price-lookup__chart-legend seller-price-lookup__chart-legend--interactive">
					<legend className="sr-only">預測圖層</legend>
					<button
						type="button"
						aria-pressed={showMedian}
						className="seller-price-lookup__chart-legend-item seller-price-lookup__chart-legend-item--forecast"
						onClick={() => setShowMedian((current) => !current)}
					>
						P50 中位線
					</button>
					<button
						type="button"
						aria-pressed={showBand}
						className="seller-price-lookup__chart-legend-item seller-price-lookup__chart-legend-item--band"
						onClick={() => setShowBand((current) => !current)}
					>
						P10～P90 區間
					</button>
				</fieldset>
			</div>
			<figure className="seller-price-lookup__recharts" style={{ height }}>
				<figcaption className="sr-only">
					七日條件式價格預測圖，{readyDays} 天具有 P50；可切換 P50 中位線與 P10
					到 P90 區間，精確數值見下方七日預測明細
				</figcaption>
				<ResponsiveContainer width="100%" height="100%">
					<ComposedChart
						data={rows}
						accessibilityLayer
						aria-label={`七日條件式價格預測圖，${readyDays} 天具有 P50`}
						margin={{ top: 12, right: 8, bottom: 0, left: -8 }}
					>
						<defs>
							<linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
								<stop
									offset="0%"
									stopColor="var(--hkui-primary-300, var(--hkui-primary))"
									stopOpacity={0.32}
								/>
								<stop
									offset="100%"
									stopColor="var(--hkui-warning-600)"
									stopOpacity={0.07}
								/>
							</linearGradient>
						</defs>
						<CartesianGrid
							stroke="var(--hkui-divider)"
							strokeDasharray="3 5"
							vertical={false}
						/>
						<XAxis
							dataKey="date"
							axisLine={false}
							tickLine={false}
							tick={{
								fill: "var(--hkui-foreground-soft)",
								fontSize: "0.6875rem",
							}}
							tickFormatter={shortDate}
							minTickGap={18}
						/>
						<YAxis
							axisLine={false}
							allowDataOverflow
							tickLine={false}
							tick={{
								fill: "var(--hkui-foreground-soft)",
								fontSize: "0.6875rem",
							}}
							tickFormatter={(value: number) => `NT$${Math.round(value)}`}
							domain={[valueRange.min, valueRange.max]}
							width={58}
						/>
						<Tooltip
							isAnimationActive={!reducedMotion}
							cursor={{
								stroke: "var(--hkui-primary-400, var(--hkui-primary))",
								strokeDasharray: "3 4",
							}}
							content={<ForecastTooltip />}
						/>
						{showBand ? (
							<>
								<Area
									type="monotone"
									dataKey="bandBase"
									stackId="forecast-band"
									stroke="none"
									fill="transparent"
									isAnimationActive={false}
								/>
								<Area
									type="monotone"
									dataKey="bandRange"
									stackId="forecast-band"
									stroke="none"
									fill={`url(#${gradientId})`}
									isAnimationActive={!reducedMotion}
									animationDuration={360}
								/>
								<Line
									type="monotone"
									dataKey="p10"
									stroke="var(--hkui-warning-600)"
									strokeWidth={1.2}
									strokeDasharray="4 4"
									dot={false}
									isAnimationActive={false}
								/>
								<Line
									type="monotone"
									dataKey="p90"
									stroke="var(--hkui-accent-600)"
									strokeWidth={1.2}
									strokeDasharray="4 4"
									dot={false}
									isAnimationActive={false}
								/>
							</>
						) : null}
						{showMedian ? (
							<Line
								type="monotone"
								dataKey="p50"
								stroke="var(--seller-price-lookup-forecast-stroke, var(--hkui-primary-700, var(--hkui-primary)))"
								strokeWidth={2.8}
								dot={{
									r: 3,
									fill: "var(--hkui-content1)",
									stroke:
										"var(--seller-price-lookup-forecast-stroke, var(--hkui-primary-700, var(--hkui-primary)))",
									strokeWidth: 2,
								}}
								activeDot={{
									r: 5,
									fill: "var(--hkui-content1)",
									stroke: "var(--hkui-accent-600)",
									strokeWidth: 2,
								}}
								isAnimationActive={!reducedMotion}
								animationDuration={420}
							/>
						) : null}
					</ComposedChart>
				</ResponsiveContainer>
			</figure>
		</div>
	);
}
