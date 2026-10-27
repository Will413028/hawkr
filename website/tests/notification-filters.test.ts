import assert from "node:assert/strict";
import { test } from "node:test";
import { buildNotificationFilters } from "../lib/notifications/filters";

test("prepends 全部 and maps category+title", () => {
	const out = buildNotificationFilters([
		{ category: "order_update", title: "新訂單更新", enabled: true },
		{ category: "coupon", title: "優惠卷通知", enabled: true },
	]);
	assert.deepEqual(out, [
		{ value: "", label: "全部" },
		{ value: "order_update", label: "新訂單更新" },
		{ value: "coupon", label: "優惠卷通知" },
	]);
});

test("falls back to category key when title missing", () => {
	const out = buildNotificationFilters([{ category: "system", enabled: true }]);
	assert.deepEqual(out[1], { value: "system", label: "system" });
});

test("falls back to category key when title is empty string", () => {
	const out = buildNotificationFilters([
		{ category: "promo", title: "", enabled: true },
	]);
	assert.deepEqual(out[1], { value: "promo", label: "promo" });
});

test("empty prefs yields only 全部", () => {
	assert.deepEqual(buildNotificationFilters([]), [
		{ value: "", label: "全部" },
	]);
});
