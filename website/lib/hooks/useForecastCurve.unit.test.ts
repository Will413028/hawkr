import { describe, expect, it } from "vitest";
import type { ForecastCurveData } from "@/lib/pricebook/repo";
import { mapForecastCurveQueryResult } from "./useForecastCurve";

const FORECAST: ForecastCurveData = {
	status: "ready",
	modelVersion: "pricebook-v1",
	points: [
		{
			targetDate: "2026-08-07",
			horizonDays: 1,
			status: "ready",
			p10: 30,
			p50: 40,
			p90: 50,
		},
	],
	marketComparison: [],
};

describe("mapForecastCurveQueryResult", () => {
	it("withholds keepPreviousData placeholders until the new selection resolves", () => {
		expect(
			mapForecastCurveQueryResult({
				data: FORECAST,
				isLoading: false,
				isFetching: true,
				isPlaceholderData: true,
				error: null,
			}),
		).toEqual({
			data: null,
			loading: true,
			refreshing: false,
			retrying: false,
			error: null,
			refreshError: null,
		});
	});

	it("exposes data that belongs to the active query key", () => {
		expect(
			mapForecastCurveQueryResult({
				data: FORECAST,
				isLoading: false,
				isFetching: false,
				isPlaceholderData: false,
				error: null,
			}),
		).toEqual({
			data: FORECAST,
			loading: false,
			refreshing: false,
			retrying: false,
			error: null,
			refreshError: null,
		});
	});

	it("maps query errors without fabricating data", () => {
		expect(
			mapForecastCurveQueryResult({
				data: undefined,
				isLoading: false,
				isFetching: false,
				isPlaceholderData: false,
				error: new Error("forecast failed"),
			}),
		).toEqual({
			data: null,
			loading: false,
			refreshing: false,
			retrying: false,
			error: "forecast failed",
			refreshError: null,
		});
	});

	it("keeps active data visible when a background refresh fails", () => {
		expect(
			mapForecastCurveQueryResult({
				data: FORECAST,
				isLoading: false,
				isFetching: false,
				isPlaceholderData: false,
				error: new Error("refresh failed"),
			}),
		).toEqual({
			data: FORECAST,
			loading: false,
			refreshing: false,
			retrying: false,
			error: null,
			refreshError: "refresh failed",
		});
	});

	it("marks an active same-key refresh without clearing confirmed data", () => {
		expect(
			mapForecastCurveQueryResult({
				data: FORECAST,
				isLoading: false,
				isFetching: true,
				isPlaceholderData: false,
				error: null,
			}),
		).toMatchObject({
			data: FORECAST,
			loading: false,
			refreshing: true,
			error: null,
			refreshError: null,
		});
	});
});
