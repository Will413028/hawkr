import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it } from "node:test";

const source = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/ObservedPriceTable.tsx",
	),
	"utf-8",
);

describe("ObservedPriceTable structural contract", () => {
	it("exposes official tiers, date, market and volume without model fallback", () => {
		assert.match(source, /交易日/);
		assert.match(source, /市場/);
		assert.match(source, /平均價/);
		assert.match(source, /官方上價/);
		assert.match(source, /官方中價/);
		assert.match(source, /官方下價/);
		assert.match(source, /成交量/);
		assert.doesNotMatch(source, /p10|p50|p90/i);
	});

	it("provides an accessible table caption with price and volume units", () => {
		assert.match(source, /<caption className="visually-hidden">/);
		assert.match(source, /價格單位為新臺幣每公斤/);
		assert.match(source, /成交量單位為公斤/);
	});
});
