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
import type { PriceObservation } from "@/lib/pricebook/repo";
import { formatPrice, formatVolume } from "./insightsFormat";
import {
	computeValueRange,
	filterPointsByRange,
	type HistoryRange,
} from "./priceTrendChartMath";
import { usePrefersReducedMotion } from "./usePrefersReducedMotion";

interface HistoricalPriceChartProps {
	observations: PriceObservation[];
	range: HistoryRange;
	height?: number;
}

interface HistoryRow {
	date: string;
	value: number;
	upper?: number;
	middle?: number;
	lower?: number;
	volume?: number;
}

interface TooltipEntry<T> {
	payload?: T;
}

function shortDate(date: string): string {
	return date.slice(5).replace("-", "/");
}

function HistoryTooltip({
	active,
	payload,
}: {
	active?: boolean;
	payload?: TooltipEntry<HistoryRow>[];
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
			<strong>{row.date}</strong>
			<span>官方平均價 {formatPrice(row.value)}／kg</span>
			<small>
				上／中／下價 {formatPrice(row.upper)}／{formatPrice(row.middle)}／
				{formatPrice(row.lower)}
			</small>
			<small>成交量 {formatVolume(row.volume)}</small>
		</div>
	);
}

/** Original Pricebook chart stack: responsive Recharts with an accessible
 * interaction layer. Official observations stay separate from forecasts. */
export function HistoricalPriceChart({
	observations,
	range,
	height = 260,
}: HistoricalPriceChartProps) {
	const reducedMotion = usePrefersReducedMotion();
	const gradientId = React.useId().replaceAll(":", "");
	const allRows = React.useMemo<HistoryRow[]>(
		() =>
			observations
				.filter((observation) => observation.avgPrice != null)
				.map((observation) => ({
					date: observation.tradeDate,
					value: observation.avgPrice as number,
					upper: observation.upperPrice,
					middle: observation.middlePrice,
					lower: observation.lowerPrice,
					volume: observation.volumeKg,
				}))
				.sort((a, b) => a.date.localeCompare(b.date)),
		[observations],
	);
	const maxDate = allRows.at(-1)?.date;
	const rows = React.useMemo(
		() => (maxDate ? filterPointsByRange(allRows, range, maxDate) : []),
		[allRows, maxDate, range],
	);
	const valueRange = computeValueRange(rows.map((row) => row.value));

	if (!rows.length || !valueRange) {
		return (
			<div className="seller-price-lookup__chart">
				<p className="seller-price-lookup__chart-empty">
					目前沒有可顯示的歷史資料。
				</p>
			</div>
		);
	}

	return (
		<div className="seller-price-lookup__chart">
			<div className="seller-price-lookup__chart-legend">
				<span className="seller-price-lookup__chart-legend-item seller-price-lookup__chart-legend-item--actual">
					官方實際平均價
				</span>
			</div>
			<figure className="seller-price-lookup__recharts" style={{ height }}>
				<figcaption className="sr-only">
					{`官方批發市場實際平均價趨勢圖，共 ${rows.length} 個交易日，單位為新臺幣每公斤；精確數值見市場比較分頁的實際行情明細`}
				</figcaption>
				<ResponsiveContainer width="100%" height="100%">
					<ComposedChart
						data={rows}
						accessibilityLayer
						aria-label={`官方批發市場實際平均價趨勢圖，共 ${rows.length} 個交易日`}
						margin={{ top: 12, right: 8, bottom: 0, left: -8 }}
					>
						<defs>
							<linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
								<stop
									offset="0%"
									stopColor="var(--hkui-primary-400, var(--hkui-primary))"
									stopOpacity={0.25}
								/>
								<stop
									offset="100%"
									stopColor="var(--hkui-primary-100, var(--hkui-primary))"
									stopOpacity={0.02}
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
							minTickGap={24}
						/>
						<YAxis
							axisLine={false}
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
							content={<HistoryTooltip />}
						/>
						<Area
							type="monotone"
							dataKey="value"
							stroke="none"
							fill={`url(#${gradientId})`}
							isAnimationActive={!reducedMotion}
							animationDuration={320}
						/>
						<Line
							type="monotone"
							dataKey="value"
							stroke="var(--hkui-primary-400, var(--hkui-primary))"
							strokeWidth={2.5}
							dot={{
								r: 2.3,
								fill: "var(--hkui-primary-400, var(--hkui-primary))",
								strokeWidth: 0,
							}}
							activeDot={{
								r: 5,
								fill: "var(--hkui-content1)",
								stroke: "var(--hkui-primary-700, var(--hkui-primary))",
								strokeWidth: 2,
							}}
							isAnimationActive={!reducedMotion}
							animationDuration={360}
						/>
					</ComposedChart>
				</ResponsiveContainer>
			</figure>
		</div>
	);
}
