import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import {
	ALL_MARKETS_VALUE,
	InsightsMarketFilter,
	marketFilterItems,
} from "./InsightsMarketFilter";

describe("marketFilterItems", () => {
	it("keeps all markets as an explicit first option", () => {
		const items = marketFilterItems([
			{ marketCode: "109", marketName: "台北一" },
			{ marketCode: "104", marketName: "台北二" },
		]);

		expect(items[0]).toMatchObject({
			value: ALL_MARKETS_VALUE,
			label: "全部市場",
		});
		expect(items.slice(1).map((item) => item.value)).toEqual(["109", "104"]);
	});

	it("deduplicates entries and preserves a deep-linked selected market", () => {
		const items = marketFilterItems(
			[
				{ marketCode: "109", marketName: "台北一" },
				{ marketCode: "109", marketName: "重複名稱" },
			],
			"800",
		);

		expect(items[0].value).toBe(ALL_MARKETS_VALUE);
		expect(new Set(items.slice(1).map((item) => item.value))).toEqual(
			new Set(["109", "800"]),
		);
	});

	it("announces the current market scope on the collapsed trigger", () => {
		const html = renderToStaticMarkup(
			React.createElement(InsightsMarketFilter, {
				marketCode: "109",
				entries: [{ marketCode: "109", marketName: "臺北一" }],
				loading: false,
				onChange: () => undefined,
			}),
		);

		expect(html).toContain('aria-label="市場範圍：臺北一"');
	});
});
