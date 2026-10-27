import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it } from "node:test";

const source = readFileSync(
	join(
		import.meta.dirname,
		"../features/seller-center/insights/MarketComparisonPanel.tsx",
	),
	"utf-8",
);

describe("MarketComparisonPanel structural contract", () => {
	it("renders zero raw <svg> tags — bars are CSS width, not a second chart svg", () => {
		assert.doesNotMatch(source, /<svg\b/);
	});
});
