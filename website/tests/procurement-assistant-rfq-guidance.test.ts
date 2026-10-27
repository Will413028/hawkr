import assert from "node:assert/strict";
import { test } from "node:test";
import {
	RFQ_TEMPERATURE_UNKNOWN_COPY,
	validateBuyerDesiredRange,
} from "../features/procurement-assistant/rfq-guidance";

test("成交溫度未知不得被 0% 或 score 取代", () => {
	assert.equal(RFQ_TEMPERATURE_UNKNOWN_COPY, "成交溫度計：未知");
});

test("買家希望區間只做本地格式檢查，不計算 Pricebook 或機率", () => {
	assert.equal(validateBuyerDesiredRange("", ""), null);
	assert.equal(validateBuyerDesiredRange("32000", "43000"), null);
	assert.equal(
		validateBuyerDesiredRange("43000", "32000"),
		"希望上限不可低於下限。",
	);
	for (const [lower, upper] of [
		["-1", "20"],
		["abc", "20"],
		["10", "Infinity"],
	] as const) {
		assert.equal(
			validateBuyerDesiredRange(lower, upper),
			"請輸入大於 0 的新台幣金額，最多保留兩位小數。",
		);
	}
});
