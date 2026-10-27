import { beforeEach, describe, expect, it, vi } from "vitest";
import { apiClient } from "@/lib/api/client";
import {
	createMarketWatchTarget,
	deleteMarketWatchTarget,
	fetchMarketOverview,
	normalizeMarketOverviewData,
} from "./repo";

vi.mock("@/lib/api/client", () => ({
	apiClient: {
		authed: {
			get: vi.fn(),
			post: vi.fn(),
			delete: vi.fn(),
		},
	},
}));

const REQUEST = {
	scope: "watchlist" as const,
	windowDays: 7 as const,
	page: 1,
	pageSize: 20,
};

describe("normalizeMarketOverviewData", () => {
	it("reads string pagination counts without accepting fractional pages", () => {
		const result = normalizeMarketOverviewData(
			{ items: [], page: "2", pageSize: "10", totalItems: "21" },
			REQUEST,
		);
		expect(result).toMatchObject({
			page: 2,
			pageSize: 10,
			totalItems: 21,
			hasNext: true,
		});
		expect(
			normalizeMarketOverviewData({ items: [], page: "2.5" }, REQUEST).page,
		).toBe(1);
	});

	it("keeps valid official values, trading-day gaps and market options", () => {
		const result = normalizeMarketOverviewData(
			{
				source: "MOA_PRICEBOOK",
				generatedAt: "2026-08-25T16:31:00Z",
				sourceDataAsOf: "2026-08-25T16:30:00Z",
				page: 1,
				pageSize: 20,
				hasNext: false,
				marketOptions: [{ marketCode: "109", marketName: "台北一" }],
				items: [
					{
						watchTargetId: "9007199254740995",
						cropCode: "LA1",
						cropName: "甘藍",
						isWatched: true,
						freshnessStatus: "fresh",
						qualityReasonCodes: [],
						currency: "TWD",
						unit: "kg",
						sourceDataAsOf: "2026-08-25T16:30:00Z",
						latestActual: {
							tradeDate: "2026-08-25",
							avgPrice: "31.5",
							volumeKg: "100.25",
						},
						change: {
							absolute: "1.5",
							percent: "5",
							direction: "up",
							basis: "previous_actual",
						},
						points: [
							{ tradeDate: "2026-08-25", avgPrice: "31.5" },
							{ tradeDate: "2026-08-24", avgPrice: null },
							{ tradeDate: "2026-08-23", avgPrice: "30" },
						],
					},
				],
			},
			REQUEST,
		);

		expect(result.currency).toBe("TWD");
		expect(result.unit).toBe("kg");
		expect(result.source).toBe("MOA_PRICEBOOK");
		expect(result.generatedAt).toBe("2026-08-25T16:31:00Z");
		expect(result.sourceDataAsOf).toBe("2026-08-25T16:30:00Z");
		expect(result.marketOptions).toEqual([
			{ marketCode: "109", marketName: "台北一" },
		]);
		expect(result.items[0]?.points).toEqual([
			{ tradeDate: "2026-08-23", avgPrice: 30 },
			{ tradeDate: "2026-08-24" },
			{ tradeDate: "2026-08-25", avgPrice: 31.5 },
		]);
		expect(result.items[0]).toMatchObject({
			watchTargetId: "9007199254740995",
			freshness: "fresh",
			currency: "TWD",
			unit: "kg",
			latestActual: {
				tradeDate: "2026-08-25",
				avgPrice: 31.5,
				volumeKg: 100.25,
			},
			change: {
				absolute: 1.5,
				percent: 5,
				direction: "up",
				basis: "previous_actual",
			},
		});
		expect(result.contractIssues).toEqual([]);
	});

	it("withholds invalid identities and never coerces invalid decimals or null into zero", () => {
		const result = normalizeMarketOverviewData(
			{
				items: [
					{ cropCode: "", cropName: "未知", points: [] },
					{
						cropCode: "LA1",
						cropName: "甘藍",
						isWatched: false,
						freshnessStatus: "fresh",
						currency: "TWD",
						unit: "kg",
						qualityReasonCodes: [],
						points: [
							{ tradeDate: "2026-08-24", avgPrice: null },
							{ tradeDate: "2026-08-25", avgPrice: "31.5kg" },
						],
						latestActual: { tradeDate: "2026-08-25", avgPrice: null },
					},
				],
			},
			REQUEST,
		);

		expect(result.items).toHaveLength(1);
		expect(result.items[0]?.latestActual).toEqual({
			tradeDate: "2026-08-25",
		});
		expect(result.items[0]?.points).toEqual([
			{ tradeDate: "2026-08-24" },
			{ tradeDate: "2026-08-25" },
		]);
		expect(result.items[0]?.points).not.toContainEqual(
			expect.objectContaining({ avgPrice: 0 }),
		);
		expect(result.contractIssues).toContain(
			"items[0] 缺少 cropCode 或 cropName",
		);
		expect(result.contractIssues).toContain(
			"items[1].points[1].avgPrice 不是有效十進位數值",
		);
	});

	it("preserves the backend unavailable freshness state", () => {
		const result = normalizeMarketOverviewData(
			{
				items: [
					{
						cropCode: "LA1",
						cropName: "甘藍",
						isWatched: false,
						freshnessStatus: "unavailable",
						qualityReasonCodes: ["SOURCE_DATA_TOO_OLD"],
						points: [],
					},
				],
			},
			REQUEST,
		);

		expect(result.items[0]?.freshness).toBe("unavailable");
	});
});

describe("market overview transport", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("uses one batch GET for the visible ledger", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { currency: "TWD", unit: "kg", items: [] },
		} as never);

		await fetchMarketOverview({
			scope: "all",
			windowDays: 30,
			marketCode: "109",
			query: "甘藍",
			page: 2,
			pageSize: 20,
		});

		expect(apiClient.authed.get).toHaveBeenCalledTimes(1);
		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/overview",
			{
				query: {
					scope: "all",
					window_days: 30,
					market_code: "109",
					query: "甘藍",
					page: 2,
					page_size: 20,
				},
			},
		);
	});

	it("never sends the selected discovery market to the unfiltered watchlist", async () => {
		vi.mocked(apiClient.authed.get).mockResolvedValueOnce({
			code: 0,
			data: { items: [], page: 1, pageSize: 20, hasNext: false },
		} as never);

		await fetchMarketOverview({
			scope: "watchlist",
			windowDays: 7,
			marketCode: "109",
			page: 1,
			pageSize: 20,
		});

		expect(apiClient.authed.get).toHaveBeenCalledWith(
			"/sellers/pricebook/overview",
			{
				query: {
					scope: "watchlist",
					window_days: 7,
					query: undefined,
					page: 1,
					page_size: 20,
				},
			},
		);
	});

	it("creates and deletes a server-owned watch target", async () => {
		vi.mocked(apiClient.authed.post).mockResolvedValueOnce({
			code: 0,
			data: {
				watchTargetId: "9007199254740995",
				created: true,
			},
		} as never);
		vi.mocked(apiClient.authed.delete).mockResolvedValueOnce({
			code: 0,
			data: undefined,
		} as never);

		await expect(
			createMarketWatchTarget({ cropCode: "LA1", marketCode: "109" }),
		).resolves.toEqual({
			watchTargetId: "9007199254740995",
			created: true,
		});
		expect(apiClient.authed.post).toHaveBeenCalledWith(
			"/sellers/pricebook/watch-targets",
			{ json: { cropCode: "LA1", marketCode: "109" } },
		);

		await deleteMarketWatchTarget("wt/1");
		expect(apiClient.authed.delete).toHaveBeenCalledWith(
			"/sellers/pricebook/watch-targets/wt%2F1",
		);
	});

	it("fails closed when a successful watch response has no identifier", async () => {
		vi.mocked(apiClient.authed.post).mockResolvedValueOnce({
			code: 0,
			data: { created: true },
		} as never);

		await expect(createMarketWatchTarget({ cropCode: "LA1" })).rejects.toThrow(
			"追蹤服務未回傳有效的追蹤識別碼",
		);
	});
});

// 多來源擴充：單位從白名單改為透傳。
describe("normalizeMarketOverviewData 的多來源單位", () => {
	// 🔴 這是擴充前最具體的失效：unit 曾經是字面型別 "kg"，正規化寫成
	// `record.unit === "kg" ? "kg" : undefined`。雞蛋產地價是元／台斤，
	// 一接上來就被丟成 undefined、記一筆契約違反，行情雷達整排顯示
	// 「— / 單位待確認」——資料沒錯，畫面看起來像壞了。
	it("保留非公斤的單位，不丟成 undefined 也不記契約違反", () => {
		const result = normalizeMarketOverviewData(
			{
				items: [
					{
						cropCode: "EGG",
						cropName: "雞蛋",
						currency: "TWD",
						unit: "catty",
						priceLayer: "farmgate",
						freshnessStatus: "fresh",
						qualityReasonCodes: [],
					},
				],
			},
			REQUEST,
		);
		expect(result.items[0].unit).toBe("catty");
		expect(result.items[0].priceLayer).toBe("farmgate");
		expect(result.contractIssues.some((issue) => issue.includes("unit"))).toBe(
			false,
		);
	});

	// 空字串仍是契約違反——「必須有單位」才是真正的契約。
	it("空白單位仍記契約違反", () => {
		const result = normalizeMarketOverviewData(
			{
				items: [
					{
						cropCode: "X",
						cropName: "X",
						unit: "   ",
						freshnessStatus: "fresh",
						qualityReasonCodes: [],
					},
				],
			},
			REQUEST,
		);
		expect(result.items[0].unit).toBeUndefined();
		expect(result.contractIssues.some((issue) => issue.includes("unit"))).toBe(
			true,
		);
	});

	// 同一份清單可能同時含元／公斤與元／台斤；此時頂層單位必須留白，
	// 由各品項自己帶——推導出一個並不成立的共同單位比留白更糟。
	it("品項單位不一致時頂層單位留白", () => {
		const mixed = normalizeMarketOverviewData(
			{
				items: [
					{
						cropCode: "A",
						cropName: "A",
						unit: "kg",
						freshnessStatus: "fresh",
						qualityReasonCodes: [],
					},
					{
						cropCode: "B",
						cropName: "B",
						unit: "catty",
						freshnessStatus: "fresh",
						qualityReasonCodes: [],
					},
				],
			},
			REQUEST,
		);
		expect(mixed.unit).toBeUndefined();

		const uniform = normalizeMarketOverviewData(
			{
				items: [
					{
						cropCode: "A",
						cropName: "A",
						unit: "catty",
						freshnessStatus: "fresh",
						qualityReasonCodes: [],
					},
					{
						cropCode: "B",
						cropName: "B",
						unit: "catty",
						freshnessStatus: "fresh",
						qualityReasonCodes: [],
					},
				],
			},
			REQUEST,
		);
		expect(uniform.unit).toBe("catty");
	});
});
