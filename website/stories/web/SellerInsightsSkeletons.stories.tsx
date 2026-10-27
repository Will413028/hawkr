import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import { ForecastBandChart } from "@/features/seller-center/insights/ForecastBandChart";
import { HistoricalPriceChart } from "@/features/seller-center/insights/HistoricalPriceChart";
import {
	CategoryRollupSkeleton,
	InsightsMetricSkeleton,
	InsightsPanelSkeleton,
} from "@/features/seller-center/insights/InsightsDataState";
import { RadarRowsSkeleton } from "@/features/seller-center/insights/MarketRadarHome";
import "@/styles/features/seller-center.css";

const meta = {
	title: "Web/Feedback/SellerInsightsSkeletons",
	parameters: {
		layout: "fullscreen",
		docs: {
			description: {
				component:
					"行情骨架沿用正式 metric、圖例、260px Recharts 圖框與 radar grid。下方資料僅為固定幾何驗證 fixture，不代表行情或預測。",
			},
		},
	},
	decorators: [
		(Story) => (
			<div
				className="seller-shell"
				style={{ display: "block", minWidth: 0, padding: 16 }}
			>
				<Story />
			</div>
		),
	],
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;
const observations = [1, 2, 3].map((day) => ({
	marketCode: "fixture",
	cropCode: "fixture",
	tradeDate: `2026-01-0${day}`,
	avgPrice: 30 + day,
	volumeKg: 100,
}));
const forecast = [1, 2, 3].map((day) => ({
	targetDate: `2026-01-0${day}`,
	horizonDays: day,
	status: "available",
	p10: 20 + day,
	p50: 30 + day,
	p90: 40 + day,
}));

export const HistoricalChart: Story = {
	render: () => (
		<div className="grid gap-5">
			<div data-testid="loading">
				<InsightsPanelSkeleton variant="chart" label="正在載入固定圖表" />
			</div>
			<div data-testid="loaded">
				<HistoricalPriceChart observations={observations} range="all" />
			</div>
		</div>
	),
	play: async ({ canvasElement }) => {
		const loading = canvasElement
			.querySelector("[data-testid='loading']")
			?.getBoundingClientRect();
		const loaded = canvasElement
			.querySelector("[data-testid='loaded']")
			?.getBoundingClientRect();
		await expect(loading?.height).toBe(loaded?.height);
		await expect(
			canvasElement
				.querySelector(".seller-price-lookup__state-skeleton-body")
				?.getBoundingClientRect().height,
		).toBe(260);
	},
};
export const ForecastChart: Story = {
	render: () => (
		<div className="grid gap-5">
			<div data-testid="loading">
				<InsightsPanelSkeleton
					variant="chart"
					chartKind="forecast"
					label="正在載入固定預測圖表"
				/>
			</div>
			<div data-testid="loaded">
				<ForecastBandChart forecastPoints={forecast} />
			</div>
		</div>
	),
	play: async ({ canvasElement }) => {
		await expect(
			canvasElement
				.querySelector(".seller-price-lookup__state-skeleton-body")
				?.getBoundingClientRect().height,
		).toBe(260);
	},
};
export const MetricsAndCategory: Story = {
	render: () => (
		<div className="grid gap-5">
			<dl className="seller-price-lookup__metric-ledger">
				{["實際行情", "中位價", "區間寬度", "成交量"].map((label) => (
					<div className="seller-price-lookup__metric" key={label}>
						<dt>{label}</dt>
						<dd className="seller-price-lookup__metric-value">
							<InsightsMetricSkeleton label={label} />
						</dd>
					</div>
				))}
			</dl>
			<CategoryRollupSkeleton
				label="固定分類範例"
				level={1}
				historyRange={30}
			/>
		</div>
	),
	play: async ({ canvasElement }) => {
		for (const bone of canvasElement.querySelectorAll(
			".seller-price-lookup__metric-skeleton > span",
		))
			await expect(bone.getBoundingClientRect().width).toBeGreaterThan(0);
	},
};
export const Radar: Story = {
	render: () => (
		<div className="market-radar__ledger">
			<RadarRowsSkeleton count={3} />
			<div className="market-radar__row" data-testid="reference">
				<span className="market-radar__watch-button" />
				<div className="market-radar__row-link">
					<span className="market-radar__identity">
						<strong>固定品項</strong>
						<small>版面驗證</small>
					</span>
					<span className="market-radar__latest">
						<strong>NT$ 30</strong>
						<small>範例</small>
					</span>
					<span className="market-radar__change">—</span>
					<span className="market-radar__spark-cell" />
					<span className="market-radar__date">2026-01-03</span>
					<span className="market-radar__open-icon" />
				</div>
			</div>
		</div>
	),
	play: async ({ canvasElement }) => {
		const row = canvasElement.querySelector(
			".market-radar__rows-skeleton .market-radar__row",
		);
		const reference = canvasElement.querySelector("[data-testid='reference']");
		await expect(row?.getBoundingClientRect().height).toBe(
			reference?.getBoundingClientRect().height,
		);
		await expect(
			row?.querySelector(".market-radar__row-link")?.children.length,
		).toBe(6);
	},
};
export const TableAndComparison: Story = {
	render: () => (
		<div className="grid gap-5">
			<InsightsPanelSkeleton variant="table" label="固定表格載入範例" />
			<InsightsPanelSkeleton
				variant="table"
				tableKind="forecast"
				label="七日預測載入範例"
			/>
			<InsightsPanelSkeleton variant="comparison" label="固定比較載入範例" />
		</div>
	),
};
