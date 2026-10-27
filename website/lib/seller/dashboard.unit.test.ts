import { create } from "@bufbuild/protobuf";
import {
	DashboardPeriodSchema,
	DashboardSchema,
	PeriodAmountSchema,
	PeriodCountSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";
import {
	fetchSellerDashboard,
	periodGrowthPercent,
	periodLabel,
} from "./dashboard";

const amount = (current: bigint, previous: bigint) =>
	create(PeriodAmountSchema, { current, previous });
const count = (current: number, previous: number) =>
	create(PeriodCountSchema, { current, previous });

describe("periodGrowthPercent", () => {
	it("較前期的整數百分比（四捨五入）", () => {
		expect(periodGrowthPercent(amount(BigInt(2200), BigInt(2000)))).toBe(10);
		expect(periodGrowthPercent(count(64, 80))).toBe(-20);
		expect(periodGrowthPercent(count(2, 3))).toBe(-33);
		expect(periodGrowthPercent(count(5, 5))).toBe(0);
		expect(periodGrowthPercent(count(0, 5))).toBe(-100);
	});

	it("前期為 0 或區塊沒設：沒有基準，回 undefined", () => {
		expect(periodGrowthPercent(amount(BigInt(100), BigInt(0)))).toBeUndefined();
		expect(periodGrowthPercent(count(0, 0))).toBeUndefined();
		expect(periodGrowthPercent(undefined)).toBeUndefined();
	});

	it("幅度不設上限：前期 1、本期 7 是 +600%", () => {
		expect(periodGrowthPercent(count(7, 1))).toBe(600);
		expect(periodGrowthPercent(count(601, 100))).toBe(501);
	});
});

describe("periodLabel", () => {
	it("台北日曆日，含頭含尾", () => {
		expect(
			periodLabel(
				create(DashboardPeriodSchema, {
					firstDate: { year: 2026, month: 9, day: 30 },
					lastDate: { year: 2026, month: 10, day: 6 },
				}),
			),
		).toBe("9/30–10/6");
	});

	it("沒設或日期不完整時回 undefined", () => {
		expect(periodLabel(undefined)).toBeUndefined();
		expect(periodLabel(create(DashboardPeriodSchema, {}))).toBeUndefined();
		expect(
			periodLabel(
				create(DashboardPeriodSchema, {
					firstDate: { year: 2026, month: 9, day: 30 },
				}),
			),
		).toBeUndefined();
	});
});

describe("fetchSellerDashboard", () => {
	it("回傳 GetMyDashboard 的 dashboard", async () => {
		const dashboard = create(DashboardSchema, {});
		const getMyDashboard = vi.fn(async () => ({ dashboard }));
		const result = await fetchSellerDashboard(
			() => ({ getMyDashboard }) as never,
		);
		expect(result).toBe(dashboard);
		expect(getMyDashboard).toHaveBeenCalledWith({});
	});

	it("回應沒有 dashboard 是契約違反，拋錯而不是補空值", async () => {
		await expect(
			fetchSellerDashboard(
				() => ({ getMyDashboard: async () => ({}) }) as never,
			),
		).rejects.toThrow("GetMyDashboard");
	});
});
