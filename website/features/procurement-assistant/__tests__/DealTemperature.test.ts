/** @vitest-environment jsdom */

// ⚠️ 檔名刻意是 `.test.ts`（不是 brief 原先寫的 `.test.tsx`）：
// vitest.config.ts 的 include 只收 `**/__tests__/**/*.test.ts` 與
// `**/*.unit.test.ts`，兩者都不吃 `.tsx`；一旦用 `.tsx` 這支測試會被 vitest
// 靜默忽略（`vitest run <path>.tsx` 直接印 "No test files found"），完全跑
// 不到 RED。本檔改用這個 repo既有的 render 測試慣例（見同目錄
// `AgentTaskPanel.unit.test.ts`）：`.ts` 副檔名 + `React.createElement`（.ts
// 檔不能解析 JSX 語法）+ 檔頭 `@vitest-environment jsdom` pragma（全域
// environment 是 node）。vitest.config.ts 不在本次改動範圍內，所以用這個
// convention 對齊，而不是去動 include glob。

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";
import { DealTemperature } from "../DealTemperature";
import type { RfqPricePreview } from "../rfq-price-contract";

afterEach(cleanup);

const available = {
	status: "AVAILABLE" as const,
	reasonCode: "",
	probabilityLowerBps: "4100",
	probabilityUpperBps: "7800",
	band: "WARM" as const,
	outcomeDefinition: "BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE",
	outcomeDefinitionVersion: "1",
	policyVersion: "DEAL_TEMPERATURE_COVERAGE_V1",
	bandVersion: "BANDS_V1",
};

describe("DealTemperature", () => {
	it("AVAILABLE 只顯示一個數字：出價上限所在的行情位置", () => {
		render(
			React.createElement(DealTemperature, {
				preview: { dealTemperature: available } as unknown as RfqPricePreview,
			}),
		);
		expect(screen.getByText("78%")).toBeTruthy();
		expect(screen.getByText(/出價位置：溫/)).toBeTruthy();
		expect(screen.getByText(/行情有 78% 落在你的出價上限之下/)).toBeTruthy();
	});

	// probabilityLowerBps（41%）照收但**不顯示**：它對買家是反指標（出價遠高於
	// 行情時反而下降），留在 wire 上只是為了成為 outcomeDefinitionVersion = 2
	// 的 feature。ADR 2026-09-09 的 Amendment（2026-09-10）。
	it("嚴格界的數字不出現在畫面上", () => {
		const { container } = render(
			React.createElement(DealTemperature, {
				preview: { dealTemperature: available } as unknown as RfqPricePreview,
			}),
		);
		expect(container.textContent).not.toContain("41%");
		expect(container.textContent).not.toContain("41% – 78%");
	});

	it("BUYER_RANGE_NOT_SET 顯示引導文案且不顯示任何百分比", () => {
		render(
			React.createElement(DealTemperature, {
				preview: {
					dealTemperature: {
						status: "UNKNOWN",
						reasonCode: "BUYER_RANGE_NOT_SET",
					},
				} as unknown as RfqPricePreview,
			}),
		);
		expect(screen.getByText("成交溫度計：未知")).toBeTruthy();
		expect(screen.getByText("調整希望詢價區間以查看出價位置。")).toBeTruthy();
		expect(screen.queryByText(/%/)).toBeNull();
	});

	it("preview 為 null（尚未取得任何試算）時顯示未知文案，不出現任何百分比", () => {
		render(React.createElement(DealTemperature, { preview: null }));
		expect(screen.getByText("成交溫度計：未知")).toBeTruthy();
		expect(screen.getByText("目前尚無足夠資料判斷出價位置。")).toBeTruthy();
		expect(screen.queryByText(/%/)).toBeNull();
	});
});

describe("DealTemperature — 0%–100% 量表", () => {
	it("AVAILABLE 渲染固定 0%–100% 的軸，刻度依 server 的寬鬆界定位", () => {
		const { container } = render(
			React.createElement(DealTemperature, {
				preview: { dealTemperature: available } as unknown as RfqPricePreview,
			}),
		);

		expect(
			container.querySelector(".assistant-rfq-guidance__temperature-gauge"),
		).toBeTruthy();
		expect(screen.getByText("0%")).toBeTruthy();
		expect(screen.getByText("100%")).toBeTruthy();

		// 單一刻度：量表上只有一個點，落在 F(買家上限) 的位置。
		const points = container.querySelectorAll(
			".assistant-rfq-guidance__temperature-point",
		);
		expect(points).toHaveLength(1);
		expect((points[0] as HTMLElement).style.left).toBe("78%");
	});

	// ADR 2026-09-09 invariant：client 不得由 bps 自行推導色帶。這支把它釘成
	// 可執行的：三組 band 配**同一組 bps**，畫面上的 data-band 必須跟著 server
	// 走。如果哪天有人改成從 bps 算冷熱，這裡的 COLD／HOT 兩例會紅。
	it("色帶完全由 server 的 band 決定，不由 bps 推導", () => {
		for (const band of ["COLD", "WARM", "HOT"] as const) {
			const { container } = render(
				React.createElement(DealTemperature, {
					preview: {
						dealTemperature: { ...available, band },
					} as unknown as RfqPricePreview,
				}),
			);
			expect(
				container
					.querySelector(".assistant-rfq-guidance__temperature")
					?.getAttribute("data-band"),
			).toBe(band);
			cleanup();
		}
	});

	it("UNKNOWN 不渲染量表，也不掛 data-band", () => {
		const { container } = render(
			React.createElement(DealTemperature, {
				preview: {
					dealTemperature: {
						status: "UNKNOWN",
						reasonCode: "RANGE_OUTSIDE_REFERENCE",
					},
				} as unknown as RfqPricePreview,
			}),
		);

		expect(
			container.querySelector(".assistant-rfq-guidance__temperature-gauge"),
		).toBeNull();
		expect(
			container
				.querySelector(".assistant-rfq-guidance__temperature")
				?.getAttribute("data-band"),
		).toBeNull();
	});
});
