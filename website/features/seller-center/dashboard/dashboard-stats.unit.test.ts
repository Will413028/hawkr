import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
	type Dashboard,
	DashboardSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { buildStats, formatGrowth, todosPhaseOf } from "./DashboardPage";

/**
 * 關鍵指標卡的口徑（B12、K14）。數字是最近 7 個完整的台北日曆日，成長是 client 以兩個期間算出的
 * 「較前 7 日」；四個區塊各自可能沒設，沒設顯示「—」，不是 0。
 */

type DashInit = MessageInitShape<typeof DashboardSchema>;

const PERIOD = {
	currentPeriod: {
		firstDate: { year: 2026, month: 9, day: 30 },
		lastDate: { year: 2026, month: 10, day: 6 },
	},
	previousPeriod: {
		firstDate: { year: 2026, month: 9, day: 23 },
		lastDate: { year: 2026, month: 9, day: 29 },
	},
} as const;

function dash(over: DashInit = {}): Dashboard {
	const base: DashInit = {
		...PERIOD,
		sales: { revenue: { current: BigInt(2200), previous: BigInt(2000) } },
		productViews: { current: 64, previous: 80 },
		followers: { total: 9, newFollowers: { current: 3, previous: 1 } },
		orderStatusCounts: { pendingShipmentCount: 4, aftersalesCount: 2 },
	};
	return create(DashboardSchema, { ...base, ...over } as DashInit);
}

const byLabel = (stats: ReturnType<typeof buildStats>, label: string) => {
	const item = stats.find((s) => s.label === label);
	if (!item) throw new Error(`沒有「${label}」卡片`);
	return item;
};

describe("formatGrowth", () => {
	it("百分比一律帶「較前 7 日」，不得裸寫", () => {
		expect(formatGrowth(-100).text).toBe("較前 7 日 -100%");
		expect(formatGrowth(79).text).toBe("較前 7 日 +79%");
		expect(formatGrowth(0).text).toBe("較前 7 日 0%");
	});

	it("0% 是中性、非 0 是方向性", () => {
		expect(formatGrowth(0).neutral).toBe(true);
		expect(formatGrowth(-100).neutral).toBe(false);
		expect(formatGrowth(12).neutral).toBe(false);
	});

	it("沒有基準時畫破折號，不掛期間詞", () => {
		expect(formatGrowth(undefined)).toEqual({ text: "—", neutral: true });
	});
});

describe("buildStats", () => {
	it("營收卡是「近 7 日營收」，值是目前期間、成長與前期比較、期間寫在卡片下方", () => {
		const revenue = byLabel(buildStats(dash()), "近 7 日營收");
		expect(revenue.value).toBe("NT$ 2,200");
		expect(revenue.change).toBe("較前 7 日 +10%");
		expect(revenue.caption).toBe("9/30–10/6");
		expect(buildStats(dash()).map((s) => s.label)).not.toContain("累計營收");
	});

	it("商品瀏覽與新追蹤者同樣帶期間；新追蹤者標「近 7 日」", () => {
		const stats = buildStats(dash());
		const views = byLabel(stats, "商品瀏覽");
		expect(views.value).toBe("64");
		expect(views.change).toBe("較前 7 日 -20%");
		expect(views.caption).toBe("9/30–10/6");
		const followers = byLabel(stats, "新追蹤者");
		expect(followers.value).toBe("+3");
		expect(followers.change).toBe("近 7 日");
		expect(followers.caption).toBe("9/30–10/6");
	});

	it("前期為 0 時不算百分比（不是 +∞、也不是 0%）", () => {
		const stats = buildStats(
			dash({
				sales: { revenue: { current: BigInt(2200), previous: BigInt(0) } },
				productViews: { current: 5, previous: 0 },
			}),
		);
		expect(byLabel(stats, "近 7 日營收").change).toBe("—");
		expect(byLabel(stats, "商品瀏覽").change).toBe("—");
		expect(byLabel(stats, "近 7 日營收").value).toBe("NT$ 2,200");
	});

	it("本期為 0 但有前期是真的 -100%，不是沒有資料", () => {
		const stats = buildStats(
			dash({
				sales: { revenue: { current: BigInt(0), previous: BigInt(500) } },
			}),
		);
		expect(byLabel(stats, "近 7 日營收").value).toBe("NT$ 0");
		expect(byLabel(stats, "近 7 日營收").change).toBe("較前 7 日 -100%");
	});

	it("區塊沒設：每張卡顯示「—」，不是 0", () => {
		const stats = buildStats(create(DashboardSchema, { ...PERIOD }));
		expect(byLabel(stats, "近 7 日營收").value).toBe("—");
		expect(byLabel(stats, "近 7 日營收").change).toBe("—");
		expect(byLabel(stats, "待出貨訂單").value).toBe("—");
		expect(byLabel(stats, "待出貨訂單").change).toBe("—");
		expect(byLabel(stats, "商品瀏覽").value).toBe("—");
		expect(byLabel(stats, "新追蹤者").value).toBe("—");
		expect(byLabel(stats, "新追蹤者").change).toBe("—");
	});

	it("沒有儀表板（尚未載入或讀取失敗）時不編數字", () => {
		const stats = buildStats(null);
		expect(stats.map((s) => s.value)).toEqual(["—", "—", "—", "—"]);
		expect(stats.every((s) => s.caption === undefined)).toBe(true);
	});

	it("只有單一區塊讀不到時，其他區塊照常顯示", () => {
		const stats = buildStats(dash({ sales: undefined }));
		expect(byLabel(stats, "近 7 日營收").value).toBe("—");
		expect(byLabel(stats, "商品瀏覽").value).toBe("64");
		expect(byLabel(stats, "待出貨訂單").value).toBe("4");
	});

	it("待出貨卡：訂單狀態快照沒有期間，副標是售後件數", () => {
		const pending = byLabel(buildStats(dash()), "待出貨訂單");
		expect(pending.value).toBe("4");
		expect(pending.change).toBe("2 筆售後");
		expect(pending.caption).toBeUndefined();
	});
});

describe("todosPhaseOf", () => {
	it("沒有資料也沒有失敗是讀取中", () => {
		expect(todosPhaseOf(null, false)).toBe("loading");
	});

	it("第一次讀取失敗是不知道，不是沒有待辦", () => {
		expect(todosPhaseOf(null, true)).toBe("unknown");
	});

	it("訂單狀態區塊沒設是不知道", () => {
		expect(todosPhaseOf(create(DashboardSchema, { ...PERIOD }), false)).toBe(
			"unknown",
		);
	});

	it("訂單狀態區塊有設（即使件數全是 0）才是讀到了", () => {
		expect(
			todosPhaseOf(
				dash({
					orderStatusCounts: { pendingShipmentCount: 0, aftersalesCount: 0 },
				}),
				false,
			),
		).toBe("ready");
	});
});
