import { describe, expect, it } from "vitest";
import type {
	MarketOverviewData,
	MarketOverviewItem,
} from "@/lib/pricebook/repo";
import { optimisticallyUpdateMarketOverview } from "./useMarketWatchlist";

function item(
	marketCode?: string,
	overrides: Partial<MarketOverviewItem> = {},
): MarketOverviewItem {
	return {
		cropCode: "LA1",
		cropName: "甘藍",
		marketCode,
		...(marketCode ? { marketName: marketCode } : {}),
		isWatched: false,
		freshness: "fresh",
		qualityReasonCodes: [],
		points: [],
		...overrides,
	};
}

function data(
	scope: "watchlist" | "all",
	items: MarketOverviewItem[],
	marketCode?: string,
): MarketOverviewData {
	return {
		scope,
		windowDays: 7,
		marketCode,
		currency: "TWD",
		unit: "kg",
		marketOptions: [],
		items,
		page: 1,
		pageSize: 20,
		hasNext: false,
		contractIssues: [],
	};
}

describe("optimisticallyUpdateMarketOverview", () => {
	it("updates only the matching crop and market", () => {
		const taipei = item("109");
		const taichung = item("400");
		const result = optimisticallyUpdateMarketOverview(
			data("all", [taipei, taichung], "109"),
			{
				item: taipei,
				nextWatched: true,
				windowDays: 7,
			},
		);

		expect(result?.items[0]?.isWatched).toBe(true);
		expect(result?.items[1]?.isWatched).toBe(false);
	});

	it("removes the watched row from the personal ledger and adjusts total", () => {
		const watched = item("109", {
			isWatched: true,
			watchTargetId: "wt-1",
		});
		const source = { ...data("watchlist", [watched]), totalItems: 1 };
		const result = optimisticallyUpdateMarketOverview(source, {
			item: watched,
			nextWatched: false,
			windowDays: 7,
		});

		expect(result?.items).toEqual([]);
		expect(result?.totalItems).toBe(0);
	});

	it("does not confuse an all-market watch with a selected-market row", () => {
		const selectedMarketRow = item(undefined, { isWatched: true });
		const allMarketWatch = item(undefined, {
			isWatched: true,
			watchTargetId: "wt-all",
		});
		const source = data("all", [selectedMarketRow], "109");

		const result = optimisticallyUpdateMarketOverview(source, {
			item: allMarketWatch,
			nextWatched: false,
			windowDays: 7,
		});

		expect(result).toBe(source);
		expect(result?.items[0]?.isWatched).toBe(true);
	});

	it("inserts a selected-market target into the unfiltered watchlist", () => {
		const taipei = item("109");
		const result = optimisticallyUpdateMarketOverview(
			{ ...data("watchlist", []), totalItems: 0 },
			{
				item: taipei,
				nextWatched: true,
				windowDays: 7,
			},
		);

		expect(result?.items).toHaveLength(1);
		expect(result?.items[0]).toMatchObject({
			cropCode: "LA1",
			marketCode: "109",
			isWatched: true,
		});
		expect(result?.totalItems).toBe(1);
	});
});
