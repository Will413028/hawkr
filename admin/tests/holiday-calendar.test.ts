import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	buildImportRequest,
	buildPreview,
	coverageBadge,
	coverageDaysLeft,
	parseCalendarCsv,
	yearRowsOf,
} from "../src/app/admin/holiday-calendar/holiday-calendar";

const CSV = [
	"﻿西元日期,星期,是否放假,備註",
	"20260101,四,2,開國紀念日",
	"20260102,五,0,",
	"20260103,六,2,",
	"",
	"20260230,一,0,壞日期",
	"20270101,五,2,元旦",
].join("\r\n");

test("解析辦公日曆表 CSV：BOM、空行、壞日期略過，是否放假 2 才是放假", () => {
	const rows = parseCalendarCsv(CSV);
	assert.deepEqual(
		rows.map((r) => [r.iso, r.holiday, r.note]),
		[
			["2026-01-01", true, "開國紀念日"],
			["2026-01-02", false, ""],
			["2026-01-03", true, ""],
			["2027-01-01", true, "元旦"],
		],
	);
});

test("是否放假不是 0 或 2 的列略過", () => {
	assert.equal(parseCalendarCsv("h\n20260101,四,1,x").length, 0);
});

test("預覽依列數降序、宣告年份只取該年並依日期排序", () => {
	const rows = parseCalendarCsv(CSV);
	const p = buildPreview(rows);
	assert.deepEqual(p.yearCounts, [
		{ year: 2026, count: 3 },
		{ year: 2027, count: 1 },
	]);
	assert.equal(p.totalRows, 4);
	assert.deepEqual(
		yearRowsOf(rows, 2027).map((r) => r.iso),
		["2027-01-01"],
	);
});

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
	assert.match(
		src,
		/BizCalendarAdminService\.method\.getHolidayCalendarCoverage/,
	);
	assert.doesNotMatch(src, /\/api\/v1\/admin\/holidays/);
	assert.doesNotMatch(src, /services\/api"/);
});
