import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import {
	ObservedPriceSparkline,
	sparklinePath,
} from "./ObservedPriceSparkline";

describe("sparklinePath", () => {
	it("does not connect across a missing official-price observation", () => {
		const path = sparklinePath([
			{ tradeDate: "2026-08-23", avgPrice: 20 },
			{ tradeDate: "2026-08-24" },
			{ tradeDate: "2026-08-25", avgPrice: 24 },
		]);

		expect(path.match(/M/g)).toHaveLength(2);
		expect(path).not.toContain("L");
	});

	it("keeps a real zero value instead of treating it as missing", () => {
		const path = sparklinePath([
			{ tradeDate: "2026-08-24", avgPrice: 0 },
			{ tradeDate: "2026-08-25", avgPrice: 1 },
		]);

		expect(path).toContain("L");
	});

	it("renders an accessible label for the code-native SVG", () => {
		const html = renderToStaticMarkup(
			createElement(ObservedPriceSparkline, {
				label: "甘藍近 7 天內有成交資料的官方平均價走勢",
				points: [
					{ tradeDate: "2026-08-24", avgPrice: 20 },
					{ tradeDate: "2026-08-25", avgPrice: 24 },
				],
			}),
		);

		expect(html).toContain('role="img"');
		expect(html).toContain("甘藍近 7 天內有成交資料的官方平均價走勢");
	});
});
