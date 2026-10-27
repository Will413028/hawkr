import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	MeasureRequestSchema,
	MeasureSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	MeasureRequestKind,
	MeasureRequestState,
	MeasureState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	MEASURE_LIST_TABS,
	measureListRequest,
	pendingRequestOf,
	requestKindLabel,
	requestStateText,
	rowWaitingText,
	storeColumnText,
	waitingDays,
	waitingText,
} from "../src/app/admin/measures/measure-inbox";

/**
 * 措施收件匣（/admin/measures）的純邏輯：分頁對應的伺服器篩選、請求文字、已等候天數。
 * 時間都用注入的 now，不讀時鐘，也不斷言實際耗時。
 */

const HOUR = 60 * 60 * 1000;
const SUBMIT = new Date("2026-10-10T02:00:00Z");

function pendingMeasure(submit: Date, kind = MeasureRequestKind.APPEAL) {
	return create(MeasureSchema, {
		id: "m1",
		requests: [
			create(MeasureRequestSchema, {
				id: "r0",
				kind: MeasureRequestKind.APPEAL,
				state: MeasureRequestState.REJECTED,
				submitTime: timestampFromDate(new Date("2026-09-01T00:00:00Z")),
			}),
			create(MeasureRequestSchema, {
				id: "r1",
				kind,
				state: MeasureRequestState.PENDING,
				submitTime: timestampFromDate(submit),
			}),
		],
	});
}

test("分頁依序是 待裁定｜複審到期｜生效中｜全部，各自只送自己的伺服器篩選", () => {
	assert.deepEqual(
		MEASURE_LIST_TABS.map((t) => t.label),
		["待裁定", "複審到期", "生效中", "全部"],
	);
	const requests = MEASURE_LIST_TABS.map((t) =>
		measureListRequest(t, "tok", 20),
	);
	assert.deepEqual(requests, [
		{
			requestState: MeasureRequestState.PENDING,
			pageToken: "tok",
			pageSize: 20,
		},
		{ reviewDue: true, pageToken: "tok", pageSize: 20 },
		{ state: MeasureState.ACTIVE, pageToken: "tok", pageSize: 20 },
		{ pageToken: "tok", pageSize: 20 },
	]);
});

test("待裁定不指定請求種類（申覆與解除請求合在同一個分頁）；空狀態文字", () => {
	const [pending, due] = MEASURE_LIST_TABS;
	assert.equal(
		(pending?.filter as Record<string, unknown>).requestKind,
		undefined,
	);
	assert.equal(pending?.emptyText, "沒有待裁定的申覆或解除請求");
	assert.equal(due?.emptyText, "沒有到期的複審");
});

test("已等候天數：現在減提出時間的完整天數，剛好 24 小時才算 1 天", () => {
	const at = (ms: number) => new Date(SUBMIT.getTime() + ms);
	assert.equal(waitingDays(SUBMIT, SUBMIT), 0);
	assert.equal(waitingDays(SUBMIT, at(24 * HOUR - 1)), 0);
	assert.equal(waitingDays(SUBMIT, at(24 * HOUR)), 1);
	assert.equal(waitingDays(SUBMIT, at(24 * HOUR + 1)), 1);
	assert.equal(waitingDays(SUBMIT, at(48 * HOUR - 1)), 1);
	assert.equal(waitingDays(SUBMIT, at(7 * 24 * HOUR)), 7);
});

test("已等候天數不用台北日期相減：23:59 提出、隔天早上仍是 0 天", () => {
	// 台北 2026-10-10 23:59 = 15:59Z；隔天（台北 10-11）08:00 = 00:00Z+1。
	const submit = new Date("2026-10-10T15:59:00Z");
	const now = new Date("2026-10-11T00:00:00Z");
	assert.equal(waitingDays(submit, now), 0);
});

test("時鐘差造成的負值算 0", () => {
	assert.equal(waitingDays(SUBMIT, new Date(SUBMIT.getTime() - HOUR)), 0);
});

test("已等候文字：0 天是「今天提出」，其餘「已等候 N 天」", () => {
	assert.equal(waitingText(0), "今天提出");
	assert.equal(waitingText(1), "已等候 1 天");
	assert.equal(waitingText(12), "已等候 12 天");
});

test("清單列的已等候取待處理請求的提出時間（不是舊的已駁回請求）；沒有待處理請求沒有文字", () => {
	const m = pendingMeasure(SUBMIT);
	assert.equal(
		rowWaitingText(m, new Date(SUBMIT.getTime() + 3 * 24 * HOUR + HOUR)),
		"已等候 3 天",
	);
	assert.equal(rowWaitingText(m, SUBMIT), "今天提出");
	const none = create(MeasureSchema, {
		requests: [
			create(MeasureRequestSchema, {
				kind: MeasureRequestKind.APPEAL,
				state: MeasureRequestState.ACCEPTED,
				submitTime: timestampFromDate(SUBMIT),
			}),
		],
	});
	assert.equal(pendingRequestOf(none), undefined);
	assert.equal(rowWaitingText(none, SUBMIT), undefined);
});

test("請求種類與狀態的文字（共用目錄的後台員工受眾）；UNSPECIFIED 與不認得的值是「未知」", () => {
	assert.equal(requestKindLabel(MeasureRequestKind.APPEAL), "申覆");
	assert.equal(requestKindLabel(MeasureRequestKind.REMEDIATION), "解除請求");
	assert.equal(requestKindLabel(MeasureRequestKind.UNSPECIFIED), "未知");
	assert.equal(requestKindLabel(99 as MeasureRequestKind), "未知");
	const text = (kind: MeasureRequestKind, state: MeasureRequestState) =>
		requestStateText({ kind, state });
	assert.equal(
		text(MeasureRequestKind.APPEAL, MeasureRequestState.PENDING),
		"申覆待裁定",
	);
	assert.equal(
		text(MeasureRequestKind.APPEAL, MeasureRequestState.ACCEPTED),
		"申覆成立",
	);
	assert.equal(
		text(MeasureRequestKind.APPEAL, MeasureRequestState.REJECTED),
		"申覆駁回",
	);
	assert.equal(
		text(MeasureRequestKind.APPEAL, MeasureRequestState.CLOSED),
		"申覆已結案",
	);
	assert.equal(
		text(MeasureRequestKind.REMEDIATION, MeasureRequestState.PENDING),
		"解除請求待裁定",
	);
	assert.equal(
		text(MeasureRequestKind.UNSPECIFIED, MeasureRequestState.PENDING),
		"未知",
	);
	assert.equal(
		text(MeasureRequestKind.APPEAL, MeasureRequestState.UNSPECIFIED),
		"未知",
	);
	assert.equal(
		text(MeasureRequestKind.APPEAL, 99 as MeasureRequestState),
		"未知",
	);
	// 不認得的種類（新版伺服器才有的值）用目錄 other 的文字，不當成申覆或解除請求。
	assert.equal(
		text(99 as MeasureRequestKind, MeasureRequestState.PENDING),
		"請求待裁定",
	);
});

test("店家欄：店名；沒有店名是「店家 {store_id}」，不退回措施 ID", () => {
	const m = create(MeasureSchema, { id: "measure-id", storeId: "77" });
	assert.equal(storeColumnText(m), "店家 77");
	assert.equal(
		storeColumnText(create(MeasureSchema, { ...m, storeName: "阿里山農場" })),
		"阿里山農場",
	);
});

test("請求狀態窮舉：每個非 UNSPECIFIED 的狀態、每個種類都有文字（新增 enum 值會在此轉紅）", () => {
	const states = Object.values(MeasureRequestState).filter(
		(v): v is MeasureRequestState =>
			typeof v === "number" && v !== MeasureRequestState.UNSPECIFIED,
	);
	assert.ok(states.length >= 4);
	for (const kind of [
		MeasureRequestKind.APPEAL,
		MeasureRequestKind.REMEDIATION,
	]) {
		for (const state of states) {
			assert.notEqual(
				requestStateText({ kind, state }),
				"未知",
				`${MeasureRequestKind[kind]} ${MeasureRequestState[state]}`,
			);
		}
	}
});
