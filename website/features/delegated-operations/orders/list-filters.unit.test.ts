import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { OrderStatus } from "@/lib/codemeta/enums";
import type { DelegatedOrderSearchFilters } from "./filters";
import {
	createDelegatedOrderRepo,
	fetchDelegatedOrderStatusTotals,
} from "./repo";

/**
 * 代行訂單列表的搜尋與日期範圍（56 號 §6.1.10 的 Query 契約，backend#2478）。
 *
 * 這一組守三件事，三件都是「畫面看起來正常、答案卻是錯的」那一類：
 *   1. 條件真的送到**伺服器**（不是前端 `.filter()` 當前這一頁）
 *   2. KPI 五張卡與清單吃**同一組**條件（否則兩個數字並排互相矛盾）
 *   3. 日期以**台北曆日**進、以 **UTC instant** 出（差 8 小時，畫面上看不出來）
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));

interface Call {
	path: string;
	query: Record<string, string>;
}

/**
 * 回一個最小的 `response.Page` 形狀，並側錄打出去的 path 與 query。
 *
 * ⚠️ `RepoFetcher` 的 query 走**第二個參數**（`delegatedFetch(path, { query })`）而不是
 * 併進 path —— 側錄 path 就好會讓底下每一條斷言都拿到 `null`，看起來像「篩選沒送出去」，
 * 實際上是量錯了地方。
 */
function recordingFetcher(calls: Call[], total = "7") {
	return async (path: string, opts?: { query?: Record<string, string> }) => {
		calls.push({ path, query: { ...(opts?.query ?? {}) } });
		return { data: { records: [], total, page: "1", pageSize: "1" } };
	};
}

function queryOf(call: Call): URLSearchParams {
	return new URLSearchParams(call.query);
}

const FILTERS: DelegatedOrderSearchFilters = {
	keyword: "番茄",
	dateFrom: "2026-08-18",
	dateTo: "2026-08-19",
};

describe("列表：篩選條件送到伺服器", () => {
	it("keyword 與日期範圍都在 query 上", async () => {
		const calls: Call[] = [];
		const repo = createDelegatedOrderRepo(
			"2685914682",
			recordingFetcher(calls) as never,
		);
		await repo.list({ page: 1, pageSize: 10, status: "20", ...FILTERS });

		expect(calls).toHaveLength(1);
		const qs = queryOf(calls[0]);
		expect(qs.get("keyword")).toBe("番茄");
		expect(qs.get("status")).toBe("20");
		expect(qs.get("date_from")).toBe("2026-08-17T16:00:00Z");
		expect(qs.get("date_to")).toBe("2026-08-19T15:59:59Z");
	});

	/**
	 * 台北曆日 → UTC instant。使用者選 8/19 指的是台北的 8/19，而後端比對的是
	 * `created_at` 這個 instant：不換算的話 `2026-08-19` 會被當成 UTC 午夜，篩到的
	 * 其實是台北 8/19 08:00 起算的那 24 小時 —— 少掉當天早上八小時的訂單、多算了
	 * 隔天早上八小時，而畫面上完全看不出來。
	 */
	it("日期以台北曆日進、UTC instant 出（差的正是 8 小時）", async () => {
		const calls: Call[] = [];
		const repo = createDelegatedOrderRepo(
			"2685914682",
			recordingFetcher(calls) as never,
		);
		await repo.list({ dateFrom: "2026-08-19", dateTo: "2026-08-19" });

		const qs = queryOf(calls[0]);
		// 台北 2026-08-19 00:00:00 → 前一日 16:00Z
		expect(qs.get("date_from")).toBe("2026-08-18T16:00:00Z");
		// 台北 2026-08-19 23:59:59 → 當日 15:59:59Z
		expect(qs.get("date_to")).toBe("2026-08-19T15:59:59Z");
	});

	it("空條件不上 query（送空字串等於篩選了一個空值）", async () => {
		const calls: Call[] = [];
		const repo = createDelegatedOrderRepo(
			"2685914682",
			recordingFetcher(calls) as never,
		);
		await repo.list({ keyword: "", dateFrom: "", dateTo: "", status: "" });

		const qs = queryOf(calls[0]);
		expect([...qs.keys()].sort()).toEqual(["page", "page_size"]);
	});

	/**
	 * 只打了幾個空白也算沒搜。不 trim 的話 `keyword=%20` 會送到後端，`ILIKE '% %'`
	 * 命中一堆含空白的品名 —— 使用者以為自己還沒開始搜，清單卻已經被篩過一輪。
	 */
	it("只有空白的關鍵字不送", async () => {
		const calls: Call[] = [];
		const repo = createDelegatedOrderRepo(
			"2685914682",
			recordingFetcher(calls) as never,
		);
		await repo.list({ keyword: "   " });

		expect(queryOf(calls[0]).has("keyword")).toBe(false);
	});

	it("keyword 前後的空白在送出前被剪掉", async () => {
		const calls: Call[] = [];
		const repo = createDelegatedOrderRepo(
			"2685914682",
			recordingFetcher(calls) as never,
		);
		await repo.list({ keyword: "  O005001417  " });

		expect(queryOf(calls[0]).get("keyword")).toBe("O005001417");
	});
});

describe("KPI 卡與清單吃同一組條件", () => {
	const STATUSES = [
		"",
		OrderStatus.PENDING_SHIPMENT,
		OrderStatus.IN_TRANSIT,
		OrderStatus.COMPLETED,
		OrderStatus.SHIPPING_ABNORMAL,
	];

	/**
	 * 本組的鑑別力驗證方式：把 `fetchDelegatedOrderStatusTotals` 裡的
	 * `keyword`／`dateFrom`／`dateTo` 任一個拿掉，這一條當場轉紅。
	 * 那正是要防的畫面 —— 搜尋「番茄」之後清單剩 3 筆，卡片仍寫著「全部 81 筆」。
	 */
	it("五支查詢每一支都帶著清單當下的 keyword 與日期範圍", async () => {
		const calls: Call[] = [];
		const repo = createDelegatedOrderRepo(
			"2685914682",
			recordingFetcher(calls) as never,
		);
		await fetchDelegatedOrderStatusTotals(repo, STATUSES, FILTERS);

		expect(calls).toHaveLength(STATUSES.length);
		for (const call of calls) {
			const qs = queryOf(call);
			expect(qs.get("keyword")).toBe("番茄");
			expect(qs.get("date_from")).toBe("2026-08-17T16:00:00Z");
			expect(qs.get("date_to")).toBe("2026-08-19T15:59:59Z");
			// KPI 只要 total，不要真的把資料撈回來。
			expect(qs.get("page_size")).toBe("1");
		}
	});

	it("每張卡各自帶自己的 status（「全部」那張不帶）", async () => {
		const calls: Call[] = [];
		const repo = createDelegatedOrderRepo(
			"2685914682",
			recordingFetcher(calls) as never,
		);
		const totals = await fetchDelegatedOrderStatusTotals(
			repo,
			STATUSES,
			FILTERS,
		);

		expect(calls.map((c) => queryOf(c).get("status"))).toEqual([
			null,
			OrderStatus.PENDING_SHIPMENT,
			OrderStatus.IN_TRANSIT,
			OrderStatus.COMPLETED,
			OrderStatus.SHIPPING_ABNORMAL,
		]);
		expect(totals[OrderStatus.COMPLETED]).toBe(7);
	});
});

/**
 * 型別層擋得住「KPI 少收一個條件」（必填欄位），擋不住「頁面拿兩份不同的條件物件
 * 分別餵給兩支 hook」—— 那是兩份都合法、只是內容會漂移。這一條把頁面上的那一份
 * 釘成單一來源。
 */
describe("頁面只組一份條件，兩支查詢共用", () => {
	const source = readFileSync(
		join(ROOT, "features/delegated-operations/orders/DelegatedOrdersPage.tsx"),
		"utf8",
	);

	it("清單與 KPI 都吃 filters", () => {
		expect(source).toContain("...filters,");
		expect(source).toContain(
			"useDelegatedOrderStatusTotals(STAT_STATUSES, filters)",
		);
	});

	it("搜尋走伺服端，頁面上沒有對 records 的前端過濾", () => {
		// 伺服端分頁下 `.filter()` 只篩得到當前這一頁，使用者搜一個在第 4 頁的訂單
		// 編號會得到「查無資料」——比沒有搜尋更糟。
		expect(source).not.toMatch(/records\s*\.\s*filter\(/);
	});
});
