import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { DeadlineSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/deadline_pb";
import {
	deadlineDisplay,
	deadlineView,
	durationText,
} from "../src/lib/deadline";

/**
 * 期限的顯示（日曆時間）：時長一律無條件捨去；逾期是 danger，剩下的時間不到整段期限的四分之一是 warning；
 * 沒有期限是「-」。時間由呼叫端傳入，邊界逐一釘死。
 */

const MIN = 60 * 1000;
const HOUR = 60 * MIN;
const DAY = 24 * HOUR;
/** 2026-10-20 12:00（台北）。 */
const NOW_MS = new Date("2026-10-20T04:00:00Z").getTime();

test("時長：滿 1 天寫天加小時，1 小時到 1 天寫小時加分，不到 1 小時寫分，不到 1 分寫「不到 1 分」；剛好整數省略後面的單位", () => {
	const rows: [number, string][] = [
		[0, "不到 1 分"],
		[MIN - 1, "不到 1 分"],
		[MIN, "1 分"],
		[59 * MIN + 59_999, "59 分"],
		[HOUR - 1, "59 分"],
		[HOUR, "1 小時"],
		[HOUR + MIN - 1, "1 小時"],
		[HOUR + MIN, "1 小時 1 分"],
		[23 * HOUR + 59 * MIN + 59_999, "23 小時 59 分"],
		[DAY - 1, "23 小時 59 分"],
		[DAY, "1 天"],
		[DAY + HOUR - 1, "1 天"],
		[DAY + HOUR, "1 天 1 小時"],
		[3 * DAY + 23 * HOUR + 59 * MIN, "3 天 23 小時"],
		[72 * HOUR, "3 天"],
	];
	for (const [ms, text] of rows) {
		assert.equal(durationText(ms), text, `${ms} ms`);
	}
});

test("時長：負數與不是數字當成 0", () => {
	assert.equal(durationText(-5 * MIN), "不到 1 分");
	assert.equal(durationText(Number.NaN), "不到 1 分");
	assert.equal(durationText(Number.POSITIVE_INFINITY), "不到 1 分");
});

const START = new Date("2026-10-18T02:00:00Z");
const DUE_4H = new Date(START.getTime() + 4 * HOUR);

test("沒有期限（或期限不是有效的時間）：顯示「-」，沒有色階", () => {
	const now = START;
	assert.deepEqual(deadlineDisplay({ due: undefined, start: START, now }), {
		text: "-",
		tone: "",
	});
	assert.deepEqual(
		deadlineDisplay({ due: new Date("invalid"), start: START, now }),
		{ text: "-", tone: "" },
	);
});

test("未逾期：「剩 {時長}」；剩下的時間不到整段期限的四分之一才轉 warning（剛好四分之一不轉）", () => {
	const at = (remaining: number) =>
		deadlineDisplay({
			due: DUE_4H,
			start: START,
			now: new Date(DUE_4H.getTime() - remaining),
		});
	assert.deepEqual(at(4 * HOUR), { text: "剩 4 小時", tone: "" });
	assert.deepEqual(at(3 * HOUR), { text: "剩 3 小時", tone: "" });
	assert.deepEqual(at(HOUR), { text: "剩 1 小時", tone: "" });
	assert.deepEqual(at(HOUR - 1), { text: "剩 59 分", tone: "warning" });
	assert.deepEqual(at(10 * MIN), { text: "剩 10 分", tone: "warning" });
	assert.deepEqual(at(0), { text: "剩 不到 1 分", tone: "warning" });
});

test("逾期：「已逾期 {時長}」，danger；現在晚於期限才算逾期", () => {
	const after = (late: number) =>
		deadlineDisplay({
			due: DUE_4H,
			start: START,
			now: new Date(DUE_4H.getTime() + late),
		});
	assert.deepEqual(after(0), { text: "剩 不到 1 分", tone: "warning" });
	assert.deepEqual(after(1), { text: "已逾期 不到 1 分", tone: "danger" });
	assert.deepEqual(after(90 * MIN), {
		text: "已逾期 1 小時 30 分",
		tone: "danger",
	});
	assert.deepEqual(after(2 * DAY + 5 * HOUR), {
		text: "已逾期 2 天 5 小時",
		tone: "danger",
	});
});

test("四分之一依整段期限算：72 小時的期限在剩下不到 18 小時時轉 warning", () => {
	const due = new Date(START.getTime() + 72 * HOUR);
	const at = (remaining: number) =>
		deadlineDisplay({
			due,
			start: START,
			now: new Date(due.getTime() - remaining),
		});
	assert.deepEqual(at(18 * HOUR), { text: "剩 18 小時", tone: "" });
	assert.deepEqual(at(18 * HOUR - 1), {
		text: "剩 17 小時 59 分",
		tone: "warning",
	});
	assert.deepEqual(at(71 * HOUR), { text: "剩 2 天 23 小時", tone: "" });
});

test("沒有開始時間，或開始時間不早於期限：不轉 warning（算不出整段期限）", () => {
	const now = new Date(DUE_4H.getTime() - MIN);
	assert.deepEqual(deadlineDisplay({ due: DUE_4H, start: undefined, now }), {
		text: "剩 1 分",
		tone: "",
	});
	assert.deepEqual(deadlineDisplay({ due: DUE_4H, start: DUE_4H, now }), {
		text: "剩 1 分",
		tone: "",
	});
	assert.deepEqual(
		deadlineDisplay({
			due: DUE_4H,
			start: new Date(DUE_4H.getTime() + HOUR),
			now,
		}),
		{ text: "剩 1 分", tone: "" },
	);
});

// 伺服器給的 Deadline{startTime, dueTime} 的顯示：開始時間就是 startTime，絕對時間是台北的日期時間。
test("deadlineView：未逾期、逾期 danger、沒有期限是「-」；開始時間用 startTime 算四分之一", () => {
	const at = (ms: number) => timestampFromDate(new Date(NOW_MS + ms));
	const now = new Date(NOW_MS);
	const deadline = (startMs: number, dueMs: number) =>
		create(DeadlineSchema, { startTime: at(startMs), dueTime: at(dueMs) });
	assert.deepEqual(deadlineView(deadline(-DAY, 2 * DAY), now), {
		text: "剩 2 天",
		tone: "",
		dueText: "2026/10/22 12:00",
	});
	assert.deepEqual(deadlineView(deadline(-3 * DAY, 2 * HOUR), now), {
		text: "剩 2 小時",
		tone: "warning",
		dueText: "2026/10/20 14:00",
	});
	assert.deepEqual(deadlineView(deadline(-5 * DAY, -3 * HOUR), now), {
		text: "已逾期 3 小時",
		tone: "danger",
		dueText: "2026/10/20 09:00",
	});
	assert.deepEqual(deadlineView(undefined, now), {
		text: "-",
		tone: "",
		dueText: undefined,
	});
	assert.deepEqual(deadlineView(create(DeadlineSchema, {}), now), {
		text: "-",
		tone: "",
		dueText: undefined,
	});
});
