import assert from "node:assert/strict";
import { describe, it } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { SevenDayTable } from "../features/seller-center/insights/SevenDayTable";

describe("SevenDayTable", () => {
	it("keeps desktop table semantics and exposes a complete mobile daily ledger", () => {
		const markup = renderToStaticMarkup(
			React.createElement(SevenDayTable, {
				unit: "kg",
				points: [
					{
						targetDate: "2026-09-01",
						horizonDays: 1,
						status: "ready",
						p10: 48,
						p50: 58,
						p90: 68,
					},
					{
						targetDate: "2026-09-02",
						horizonDays: 2,
						status: "market_closed",
					},
				],
			}),
		);

		assert.match(markup, /<table/);
		assert.match(markup, /價格單位為新臺幣／公斤/);
		assert.match(markup, /<ol[^>]*aria-label="七日預測明細"/);
		assert.match(markup, /D\+1/);
		assert.match(markup, /模型中位 P50/);
		assert.match(markup, /預測區間 P10～P90/);
		assert.match(markup, /NT\$ 58/);
		assert.match(markup, /NT\$ 48～NT\$ 68/);
		assert.match(markup, /休市/);
		assert.match(markup, /—～—/);
	});
});
