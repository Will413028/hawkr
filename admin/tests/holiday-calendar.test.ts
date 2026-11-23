import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	buildImportRequest,
	coverageBadge,
	coverageDaysLeft,
	defaultImportYear,
	yearOptions,
} from "../src/app/admin/holiday-calendar/holiday-calendar";

test("匯入請求帶整數年份與檔案全文", () => {
	assert.deepEqual(buildImportRequest("2026", "x"), {
		year: 2026,
		csvContent: "x",
	});
});

test("距耗盡天數與色標", () => {
	assert.equal(coverageDaysLeft("2026-12-31", "2026-12-01"), 30);
	assert.equal(coverageDaysLeft("2026-12-31", "2027-01-01"), -1);
	assert.equal(coverageDaysLeft("", "2026-12-01"), -1);
	assert.equal(coverageBadge(-1).variant, "gray");
	assert.equal(coverageBadge(6).variant, "danger");
	assert.equal(coverageBadge(59).variant, "warning");
	assert.equal(coverageBadge(60).variant, "success");
});

test("頁面呼叫 BizCalendarAdminService，不再打舊 gateway REST", async () => {
	const src = await readFile("src/app/admin/holiday-calendar/page.tsx", "utf8");
	assert.match(src, /BizCalendarAdminService\.method\.importHolidayCalendar/);
	assert.match(src, /BizCalendarAdminService\.method\.previewHolidayCalendar/);
	assert.match(
		src,
		/BizCalendarAdminService\.method\.getHolidayCalendarCoverage/,
	);
	assert.doesNotMatch(src, /parseCalendarCsv|buildPreview|yearRowsOf/);
	assert.doesNotMatch(src, /\/api\/v1\/admin\/holidays/);
	assert.doesNotMatch(src, /services\/api"/);
});

test("匯入年份選項是今年前 1 年到後 2 年", () => {
	assert.deepEqual(yearOptions(2026), [2025, 2026, 2027, 2028]);
});

test("預設匯入年份取涵蓋最後一年的下一年，超出選項則今年", () => {
	assert.equal(defaultImportYear(2026, undefined), 2026);
	assert.equal(defaultImportYear(2026, { year: 2026 }), 2027);
	assert.equal(defaultImportYear(2026, { year: 2027 }), 2028);
	assert.equal(defaultImportYear(2026, { year: 2028 }), 2026);
	assert.equal(defaultImportYear(2026, { year: 2020 }), 2026);
	assert.equal(defaultImportYear(2026, { year: 2024 }), 2025);
	assert.equal(defaultImportYear(2026, { year: 2040 }), 2026);
});
