import { describe, expect, it } from "vitest";
import type { PriceObservation } from "@/lib/pricebook/repo";
import { mapObservedPriceHistoryQueryResult } from "./useObservedPriceHistory";

const OBSERVATION: PriceObservation = {
	tradeDate: "2026-08-25",
	marketCode: "109",
	cropCode: "A1",
	avgPrice: 61.5,
	volumeKg: 1_200,
};

describe("mapObservedPriceHistoryQueryResult", () => {
	it("treats a successful empty observation list as active data", () => {
		expect(
			mapObservedPriceHistoryQueryResult({
				data: { observations: [] },
				isLoading: false,
				isFetching: false,
				error: null,
			}),
		).toEqual({
			observations: [],
			loading: false,
			refreshing: false,
			retrying: false,
			error: null,
			refreshError: null,
		});
	});

	it("keeps the last successful observations after a refresh error", () => {
		expect(
			mapObservedPriceHistoryQueryResult({
				data: { observations: [OBSERVATION] },
				isLoading: false,
				isFetching: false,
				error: new Error("refresh failed"),
			}),
		).toEqual({
			observations: [OBSERVATION],
			loading: false,
			refreshing: false,
			retrying: false,
			error: null,
			refreshError: "refresh failed",
		});
	});

	it("exposes a blocking error only when there is no active data", () => {
		expect(
			mapObservedPriceHistoryQueryResult({
				data: undefined,
				isLoading: false,
				isFetching: false,
				error: new Error("history failed"),
			}),
		).toMatchObject({
			observations: [],
			error: "history failed",
			refreshError: null,
		});
	});
});
