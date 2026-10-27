import assert from "node:assert/strict";
import test from "node:test";
import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	endDateOf,
	scheduleFromBanner,
	scheduleToBanner,
	startDateOf,
} from "../src/components/banner/schedule";

/**
 * Banner 排程上下架的契約：start_time（含）／end_time（不含）是 Timestamp，沒有值＝該端不設限。編輯器只有
 * 日期，以台北時間解讀：開始日寫成當天 00:00、結束日寫成隔天 00:00（結束日整天有效），讀回時結束顯示成
 * 最後一個有效的日子。
 */

const at = (iso: string) => timestampFromDate(new Date(iso));
const iso = (ts: ReturnType<typeof at> | undefined) =>
	ts && timestampDate(ts).toISOString();

test("送出：開始日是台北的 00:00，結束日寫成隔天的 00:00", () => {
	const out = scheduleToBanner({
		startDate: "2026-08-01",
		endDate: "2026-08-31",
		noExpiry: false,
	});
	assert.equal(iso(out.startTime), "2026-07-31T16:00:00.000Z");
	assert.equal(iso(out.endTime), "2026-08-31T16:00:00.000Z");
});

test("送出：起訖同一天就是那一整天", () => {
	const out = scheduleToBanner({
		startDate: "2026-09-26",
		endDate: "2026-09-26",
		noExpiry: false,
	});
	assert.equal(iso(out.startTime), "2026-09-25T16:00:00.000Z");
	assert.equal(iso(out.endTime), "2026-09-26T16:00:00.000Z");
});

test("讀回：換算成台北的日期，結束顯示成最後一個有效的日子", () => {
	const form = scheduleFromBanner({
		startTime: at("2026-07-31T16:00:00Z"),
		endTime: at("2026-08-31T16:00:00Z"),
	});
	assert.deepEqual(form, {
		startDate: "2026-08-01",
		endDate: "2026-08-31",
		noExpiry: false,
	});
	assert.deepEqual(scheduleFromBanner({}), {
		startDate: "",
		endDate: "",
		noExpiry: true,
	});
});

test("存了再讀回是同樣的日期", () => {
	const form = {
		startDate: "2026-12-31",
		endDate: "2027-01-01",
		noExpiry: false,
	};
	assert.deepEqual(scheduleFromBanner(scheduleToBanner(form)), form);
});

test("台北日期的界線：UTC 15:59 還是前一天，16:00 就是隔天", () => {
	assert.equal(startDateOf(at("2026-09-25T15:59:59Z")), "2026-09-25");
	assert.equal(startDateOf(at("2026-09-25T16:00:00Z")), "2026-09-26");
	// 結束不含：隔天 00:00（台北）結束，最後一個有效的日子是前一天。
	assert.equal(endDateOf(at("2026-09-26T16:00:00Z")), "2026-09-26");
});

test("送出：勾了無期限就沒有 end_time；沒填開始日就沒有 start_time", () => {
	const noExpiry = scheduleToBanner({
		startDate: "2026-08-01",
		endDate: "2026-08-31",
		noExpiry: true,
	});
	assert.ok(noExpiry.startTime);
	assert.equal(noExpiry.endTime, undefined);
	assert.deepEqual(
		scheduleToBanner({ startDate: "", endDate: "", noExpiry: true }),
		{ startTime: undefined, endTime: undefined },
	);
});
