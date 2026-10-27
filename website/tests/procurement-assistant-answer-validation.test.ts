import assert from "node:assert/strict";
import { test } from "node:test";
import { validateFreeTextAnswer } from "../features/procurement-assistant/answer-validation";

test("一般文字 slot 接受自然語答案，但保留 200 字上限", () => {
	assert.equal(validateFreeTextAnswer("item", "珍珠芭樂"), null);
	assert.match(validateFreeTextAnswer("item", "果".repeat(201)) ?? "", /200/);
});

test("quantity 與 budget 的 /answers 值必須是純數字，避免真後端 strconv.Atoi 422", () => {
	assert.equal(validateFreeTextAnswer("quantity", "30"), null);
	assert.match(validateFreeTextAnswer("quantity", "30 箱") ?? "", /只輸入數字/);
	assert.equal(validateFreeTextAnswer("budget_per_unit", "800"), null);
	assert.match(validateFreeTextAnswer("budget_per_unit", "0") ?? "", /介於 1/);
});

test("空答案與超出後端 domain 上限會在送出前就近說明", () => {
	assert.match(validateFreeTextAnswer("quantity", "") ?? "", /先輸入/);
	assert.match(
		validateFreeTextAnswer("quantity", "1000001") ?? "",
		/1,000,000/,
	);
	assert.match(
		validateFreeTextAnswer("budget_per_unit", "10000001") ?? "",
		/10,000,000/,
	);
});
