import { describe, expect, it } from "vitest";
import type { MarketOverviewData } from "@/lib/pricebook/repo";
import { mapMarketOverviewQueryResult } from "./useMarketOverview";

const DATA: MarketOverviewData = {
	scope: "watchlist",
	windowDays: 7,
	currency: "TWD",
	unit: "kg",
	marketOptions: [],
	items: [],
	page: 1,
	pageSize: 20,
	hasNext: false,
	contractIssues: [],
};

describe("mapMarketOverviewQueryResult", () => {
	it("keeps last-good data visible after a refresh error", () => {
		expect(
			mapMarketOverviewQueryResult({
				data: DATA,
				isLoading: false,
				isFetching: false,
				error: new Error("refresh failed"),
			}),
		).toEqual({
			data: DATA,
			loading: false,
			refreshing: false,
			retrying: false,
			error: null,
			refreshError: "refresh failed",
		});
	});

	it("exposes a blocking error only before any overview succeeds", () => {
		expect(
			mapMarketOverviewQueryResult({
				data: undefined,
				isLoading: false,
				isFetching: false,
				error: new Error("overview failed"),
			}),
		).toMatchObject({
			data: null,
			error: "overview failed",
			refreshError: null,
		});
	});
});
