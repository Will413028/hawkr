import assert from "node:assert/strict";
import { test } from "node:test";
import { formatNullableCount } from "../lib/seller/order-dashboard-counts";

test("renders a known seller order tab count", () => {
	assert.equal(formatNullableCount(0), "0");
	assert.equal(formatNullableCount(15), "15");
	assert.equal(formatNullableCount(1234), "1,234");
});

test("renders an em dash while a seller order tab count is still loading", () => {
	assert.equal(formatNullableCount(null), "—");
});
