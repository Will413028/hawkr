import { describe, expect, it } from "vitest";
import { formatNullableCount } from "@/lib/seller/order-dashboard-counts";
import { delegatedProductKpi } from "./delegated-product-kpi";

/**
 * 代行商品頁 KPI 卡：「四支探詢的 total 還不知道」不得被渲染成 `0`。
 *
 * 這一支守的是 2026-08-16 dev 驗收實測到的畫面（session `OPS-1KXUSQJXJ7C3K`）：
 * gateway 對 product-service 的斷路器處於 half-open，四支 `page_size=1` 探詢全部
 * 拿到 9280／502，而畫面畫出「總商品數 15 / 啟用 0 / 未啟用 0 / 草稿 0」——
 * 同一時刻 DB 是 4004=1／4003=10／4001=4，而那 15 筆就列在同一個畫面的清單裡。
 *
 * 對平台管理員來說 `啟用 0` 不是「還在載入」，是「這家店沒有任何上架商品」。
 */
describe("delegatedProductKpi", () => {
	it("四支探詢都成功 → 逐格回真值", () => {
		expect(
			delegatedProductKpi({ all: 15, active: 1, off: 10, draft: 4 }),
		).toEqual({ total: 15, active: 1, off: 10, lowStock: null, draft: 4 });
	});

	it("真正的 0 仍然是 0（不得被當成未知）", () => {
		const kpi = delegatedProductKpi({ all: 0, active: 0, off: 0, draft: 0 });
		expect(kpi).toEqual({
			total: 0,
			active: 0,
			off: 0,
			lowStock: null,
			draft: 0,
		});
		// 一家真的沒有商品的店，四格必須是 "0" 而不是破折號 —— 否則這個修法就把
		// 「已知為零」也一起藏掉了，那是反方向的同一個謊。
		expect(formatNullableCount(kpi.active)).toBe("0");
		expect(formatNullableCount(kpi.total)).toBe("0");
	});

	it("未知（載入中／探詢失敗）→ 四格全 null，而且不借用清單的 total", () => {
		expect(delegatedProductKpi(null)).toEqual({
			total: null,
			active: null,
			off: null,
			lowStock: null,
			draft: null,
		});
	});

	it("未知渲染成破折號，不是 0", () => {
		const kpi = delegatedProductKpi(null);
		expect(formatNullableCount(kpi.active)).toBe("—");
		expect(formatNullableCount(kpi.off)).toBe("—");
		expect(formatNullableCount(kpi.draft)).toBe("—");
		// 總商品數也一起 —— 四格同生共死。舊實作以清單 `total` 兜底，於是畫出
		// 「總數 15、其餘 0」這種自相矛盾的一列（而清單的 total 是「當前篩選」的
		// 總數，點過任何一張卡之後它連總數都不是）。
		expect(formatNullableCount(kpi.total)).toBe("—");
	});

	it("代行不渲染庫存預警，該格恆為未知", () => {
		expect(
			delegatedProductKpi({ all: 15, active: 1, off: 10, draft: 4 }).lowStock,
		).toBeNull();
	});
});
