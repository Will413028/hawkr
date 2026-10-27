import * as React from "react";
import type { MarketOverviewPoint } from "@/lib/pricebook/repo";

const SPARKLINE_WIDTH = 148;
const SPARKLINE_HEIGHT = 44;
const SPARKLINE_PADDING = 3;

export function sparklinePath(points: readonly MarketOverviewPoint[]): string {
	const values = points
		.map((point) => point.avgPrice)
		.filter(
			(value): value is number => value != null && Number.isFinite(value),
		);
	if (values.length === 0) return "";
	const minimum = Math.min(...values);
	const maximum = Math.max(...values);
	const spread = maximum - minimum;
	const drawableWidth = SPARKLINE_WIDTH - SPARKLINE_PADDING * 2;
	const drawableHeight = SPARKLINE_HEIGHT - SPARKLINE_PADDING * 2;
	let drawing = false;
	return points
		.map((point, index) => {
			if (point.avgPrice == null || !Number.isFinite(point.avgPrice)) {
				drawing = false;
				return "";
			}
			const x =
				points.length === 1
					? SPARKLINE_WIDTH / 2
					: SPARKLINE_PADDING + (index / (points.length - 1)) * drawableWidth;
			const y =
				spread === 0
					? SPARKLINE_HEIGHT / 2
					: SPARKLINE_PADDING +
						((maximum - point.avgPrice) / spread) * drawableHeight;
			const command = drawing ? "L" : "M";
			drawing = true;
			return `${command}${x.toFixed(2)} ${y.toFixed(2)}`;
		})
		.filter(Boolean)
		.join(" ");
}

function lastVisiblePoint(points: readonly MarketOverviewPoint[]) {
	for (let index = points.length - 1; index >= 0; index -= 1) {
		const value = points[index]?.avgPrice;
		if (value != null && Number.isFinite(value)) return { index, value };
	}
	return null;
}

export function ObservedPriceSparkline({
	points,
	label,
}: {
	points: readonly MarketOverviewPoint[];
	label: string;
}) {
	const titleId = React.useId();
	const path = sparklinePath(points);
	const last = lastVisiblePoint(points);
	if (!path || !last) {
		return <span className="market-radar__sparkline-empty">尚無走勢</span>;
	}
	const values = points
		.map((point) => point.avgPrice)
		.filter(
			(value): value is number => value != null && Number.isFinite(value),
		);
	const minimum = Math.min(...values);
	const maximum = Math.max(...values);
	const spread = maximum - minimum;
	const drawableWidth = SPARKLINE_WIDTH - SPARKLINE_PADDING * 2;
	const drawableHeight = SPARKLINE_HEIGHT - SPARKLINE_PADDING * 2;
	const lastX =
		points.length === 1
			? SPARKLINE_WIDTH / 2
			: SPARKLINE_PADDING + (last.index / (points.length - 1)) * drawableWidth;
	const lastY =
		spread === 0
			? SPARKLINE_HEIGHT / 2
			: SPARKLINE_PADDING + ((maximum - last.value) / spread) * drawableHeight;

	return (
		<svg
			className="market-radar__sparkline"
			viewBox={`0 0 ${SPARKLINE_WIDTH} ${SPARKLINE_HEIGHT}`}
			role="img"
			aria-labelledby={titleId}
			preserveAspectRatio="none"
		>
			<title id={titleId}>{label}</title>
			<path d={path} vectorEffect="non-scaling-stroke" />
			<circle cx={lastX} cy={lastY} r="2.5" />
		</svg>
	);
}
