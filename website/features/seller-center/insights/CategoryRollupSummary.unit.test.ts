import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import type { CategoryPriceRollupData } from "@/lib/pricebook/repo";
import { CategoryRollupSummary } from "./CategoryRollupSummary";

const mockUseCategoryPriceRollup = vi.fn();

vi.mock("@/lib/hooks/useCategoryPriceRollup", () => ({
	useCategoryPriceRollup: (...args: unknown[]) =>
		mockUseCategoryPriceRollup(...args),
}));

const READY_DATA: CategoryPriceRollupData = {
	categoryId: "11",
	level: 2,
	status: "ready",
	latestPrice: 45.2,
	prevPrice: 40,
	points: [
		{ tradeDate: "2026-08-29", avgPrice: 44 },
		{ tradeDate: "2026-08-30", avgPrice: 45.2 },
	],
	sourceGroupCount: 6,
	computedAt: "2026-08-30T21:15:03Z",
	forecastPoints: [],
};

function renderSummary() {
	return renderToStaticMarkup(
		createElement(CategoryRollupSummary, {
			categoryId: "11",
			level: 2,
			label: "瓜果類",
			historyRange: 30,
			onHistoryRangeChange: vi.fn(),
		}),
	);
}

describe("CategoryRollupSummary", () => {
	it("renders the latest price, change and disclaimer for a ready rollup", () => {
		mockUseCategoryPriceRollup.mockReturnValue({
			data: READY_DATA,
			loading: false,
			error: null,
			retry: vi.fn(),
		});

		const html = renderSummary();

		expect(html).toContain("瓜果類（L2）分類彙整走勢");
		expect(html).toContain("NT$ 45.2");
		expect(html).toContain("+5.2 元");
		expect(html).toContain("+13%");
		expect(html).toContain("來源 6 個大類平均，不代表單一品項或市場價格");
	});

	it("shows an empty-data message when the batch marked this category unavailable", () => {
		mockUseCategoryPriceRollup.mockReturnValue({
			data: {
				...READY_DATA,
				status: "unavailable",
				latestPrice: undefined,
				prevPrice: undefined,
				points: [],
			},
			loading: false,
			error: null,
			retry: vi.fn(),
		});

		const html = renderSummary();

		expect(html).toContain("「瓜果類」目前沒有可用的彙整行情資料");
	});

	it("shows a loading state while the first fetch is in flight", () => {
		mockUseCategoryPriceRollup.mockReturnValue({
			data: null,
			loading: true,
			error: null,
			retry: vi.fn(),
		});

		const html = renderSummary();

		expect(html).toContain('aria-busy="true"');
	});

	it("shows an inline error when the fetch fails", () => {
		mockUseCategoryPriceRollup.mockReturnValue({
			data: null,
			loading: false,
			error: "分類彙整價格服務未回傳資料",
			retry: vi.fn(),
		});

		const html = renderSummary();

		expect(html).toContain("無法載入分類彙整價格");
		expect(html).toContain("分類彙整價格服務未回傳資料");
	});

	it("renders D+1 KPI cards and both charts when forecast data is ready", () => {
		mockUseCategoryPriceRollup.mockReturnValue({
			data: {
				...READY_DATA,
				forecastPoints: [
					{
						targetDate: "2026-09-01",
						horizonDays: 1,
						status: "ready",
						p10: 40.5,
						p50: 51.3,
						p90: 62.6,
					},
				],
			},
			loading: false,
			error: null,
			retry: vi.fn(),
		});

		const html = renderToStaticMarkup(
			createElement(CategoryRollupSummary, {
				categoryId: "11",
				level: 2,
				label: "瓜果類",
				historyRange: 30,
				onHistoryRangeChange: vi.fn(),
			}),
		);

		expect(html).toContain("D+1 模型中位價");
		expect(html).toContain("NT$ 51.3");
		expect(html).toContain("D+1 預測區間寬度");
		expect(html).toContain("NT$ 22.1"); // 62.6 - 40.5
		expect(html).toContain("近 30 日成交量");
		expect(html).toContain("大類層級不提供");
	});
});
