import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import type { MarketOverviewData } from "@/lib/pricebook/repo";
import { MarketRadarHome } from "./MarketRadarHome";

const BASE_DATA: MarketOverviewData = {
	scope: "watchlist",
	windowDays: 7,
	marketOptions: [{ marketCode: "109", marketName: "台北一" }],
	items: [
		{
			watchTargetId: "wt-1",
			cropCode: "LA1",
			cropName: "甘藍",
			variantName: "初秋",
			isWatched: true,
			status: "stale",
			freshness: "stale",
			qualityReasonCodes: [],
			currency: "TWD",
			unit: "kg",
			latestActual: { tradeDate: "2026-08-25", avgPrice: 31.5 },
			change: { absolute: 1.5, percent: 5, direction: "up" },
			points: [
				{ tradeDate: "2026-08-24", avgPrice: 30 },
				{ tradeDate: "2026-08-25", avgPrice: 31.5 },
			],
		},
	],
	page: 1,
	pageSize: 20,
	totalItems: 1,
	hasNext: false,
	contractIssues: [],
};

const ALL_DATA: MarketOverviewData = {
	...BASE_DATA,
	scope: "all",
	marketCode: "109",
	items: [
		{
			cropCode: "SO1",
			cropName: "菠菜",
			variantName: "圓葉",
			marketCode: "109",
			marketName: "台北一",
			isWatched: false,
			status: "ready",
			freshness: "unavailable",
			qualityReasonCodes: ["SOURCE_DATA_TOO_OLD"],
			currency: "TWD",
			unit: "kg",
			latestActual: { tradeDate: "2026-08-25", avgPrice: 42 },
			change: { absolute: -1, percent: -2.3, direction: "down" },
			points: [
				{ tradeDate: "2026-08-24", avgPrice: 43 },
				{ tradeDate: "2026-08-25", avgPrice: 42 },
			],
		},
		{
			cropCode: "SE1",
			cropName: "莧菜",
			marketCode: "109",
			marketName: "台北一",
			isWatched: false,
			status: "unavailable",
			freshness: "unavailable",
			qualityReasonCodes: ["NO_OBSERVED_DATA"],
			currency: "TWD",
			unit: "kg",
			points: [],
		},
	],
};

vi.mock("@/components/shared/Link", () => ({
	default: ({ href, children, ...props }: Record<string, unknown>) =>
		createElement("a", { ...props, href }, children as never),
}));

vi.mock("@/lib/hooks/useMarketOverview", () => ({
	useMarketOverview: (request: { scope: "watchlist" | "all" }) => ({
		data: request.scope === "watchlist" ? BASE_DATA : ALL_DATA,
		loading: false,
		refreshing: false,
		retrying: false,
		error: null,
		refreshError: null,
		retry: vi.fn(),
	}),
}));

vi.mock("@/lib/hooks/useMarketWatchlist", () => ({
	useMarketWatchlist: () => ({
		toggle: vi.fn(),
		pending: false,
		pendingTarget: null,
	}),
}));

vi.mock("./usePrefersReducedMotion", () => ({
	usePrefersReducedMotion: () => true,
}));

describe("MarketRadarHome", () => {
	it("renders the official-price ledger, accessible follow control and exact detail link", () => {
		const html = renderToStaticMarkup(
			createElement(MarketRadarHome, {
				marketCode: "109",
				onMarketOptionsChange: vi.fn(),
			}),
		);

		expect(html).toContain("市場雷達");
		expect(html).toContain("最新官方價");
		expect(html).toContain('aria-pressed="true"');
		expect(html).toContain('aria-label="停止追蹤甘藍・初秋（全部市場，LA1）"');
		expect(html).toContain('aria-label="追蹤菠菜・圓葉（台北一，SO1）"');
		expect(html).toContain("查看甘藍・初秋（全部市場，LA1）的行情明細");
		expect(html).toContain(
			"甘藍・初秋（全部市場，LA1）近 7 天內有成交資料的官方平均價走勢",
		);
		expect(html).toContain("最新官方價每公斤 NT$ 31.5");
		expect(html).toContain("最新交易日2026-08-25");
		expect(html).toContain("資料狀態資料較舊");
		expect(html).toContain("資料狀態來源資料過舊");
		expect(html).toContain("資料狀態暫無官方成交資料");
		expect(html).toContain("全部市場 · LA1");
		const hrefs = [...html.matchAll(/href="([^"]+)"/g)].map(
			(match) => match[1] ?? "",
		);
		const allMarketHref = hrefs.find((href) => href.includes("crop=LA1"));
		const taipeiHref = hrefs.find((href) => href.includes("crop=SO1"));
		expect(allMarketHref).toContain("scope=crop");
		expect(allMarketHref).not.toContain("market=109");
		expect(taipeiHref).toContain("market=109");
		expect(html.match(/market=109/g)).toHaveLength(2);
		expect(html).not.toContain("D+1");
		expect(html).not.toContain("P50");
		expect(html).not.toContain('role="dialog"');
	});
});
