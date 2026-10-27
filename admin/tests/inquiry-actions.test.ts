import assert from "node:assert/strict";
import test from "node:test";
import {
	CaseInquiryKind,
	CaseInquiryReplyChannel,
	CaseInquiryState,
	CaseInquiryWithdrawCause,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import {
	fromLocalInput,
	toLocalInputSeconds,
} from "../src/app/admin/merchants/[id]/measure";
import {
	buildCreateInquiry,
	buildInquiryReply,
	emptyReplyForm,
	inquiryRequestKey,
	validateInquiry,
} from "../src/app/admin/violation-reports/inquiry-actions";
import {
	INQUIRY_CHANNEL_OPTIONS,
	INQUIRY_KIND_OPTIONS,
	inquiryChannelText,
	inquiryKindText,
	inquiryStateText,
	inquiryWithdrawCauseText,
} from "../src/app/admin/violation-reports/inquiry-text";

test("詢問的名稱：種類、狀態、管道、撤回原因逐字；UNSPECIFIED 與不認得的值是「未知」", () => {
	assert.deepEqual(INQUIRY_KIND_OPTIONS.map(inquiryKindText), [
		"提供紀錄",
		"請說明",
	]);
	assert.deepEqual(
		[
			CaseInquiryState.OPEN,
			CaseInquiryState.REPLIED,
			CaseInquiryState.WITHDRAWN,
		].map(inquiryStateText),
		["等賣家回覆", "已回覆", "已撤回"],
	);
	assert.deepEqual(INQUIRY_CHANNEL_OPTIONS.map(inquiryChannelText), [
		"電話",
		"Email",
		"通訊軟體",
		"其他",
	]);
	assert.deepEqual(
		[CaseInquiryWithdrawCause.ADMIN, CaseInquiryWithdrawCause.CASE_CLOSED].map(
			inquiryWithdrawCauseText,
		),
		["管理員撤回", "案件結案時撤回"],
	);
	assert.equal(inquiryKindText(CaseInquiryKind.UNSPECIFIED), "未知");
	assert.equal(inquiryStateText(99 as CaseInquiryState), "未知");
});

test("詢問：只擋沒填說明（J6）；種類與長度由伺服器判斷", () => {
	const base = {
		kind: CaseInquiryKind.UNSPECIFIED,
		message: "",
		dueTime: "",
	};
	assert.equal(validateInquiry(base), "請填寫給賣家的說明。");
	assert.equal(
		validateInquiry({ ...base, message: " \n " }),
		"請填寫給賣家的說明。",
	);
	assert.equal(validateInquiry({ ...base, message: "請說明" }), undefined);
});

test("詢問的請求：說明去掉前後空白；期限是台北時間的 datetime-local，沒填就不帶", () => {
	const form = {
		kind: CaseInquiryKind.RECORDS,
		message: " 請提供 ",
		dueTime: "2026-10-25T10:00",
	};
	const request = buildCreateInquiry("r-1", "c-1", form);
	assert.equal(request.requestId, "r-1");
	assert.equal(request.caseId, "c-1");
	assert.equal(request.message, "請提供");
	assert.equal(
		Number(request.dueTime?.seconds),
		Date.parse("2026-10-25T10:00:00+08:00") / 1000,
	);
	assert.ok(
		!("dueTime" in buildCreateInquiry("r-1", "c-1", { ...form, dueTime: "" })),
	);
});

test("詢問的 request_id 鍵：說明前後空白不算不同的請求，種類或期限改了才算", () => {
	const form = { kind: CaseInquiryKind.RECORDS, message: "a", dueTime: "" };
	const key = inquiryRequestKey("c-1", form);
	assert.equal(inquiryRequestKey("c-1", { ...form, message: " a " }), key);
	assert.notEqual(
		inquiryRequestKey("c-1", { ...form, kind: CaseInquiryKind.EXPLANATION }),
		key,
	);
	assert.notEqual(
		inquiryRequestKey("c-1", { ...form, dueTime: "2026-10-25T10:00" }),
		key,
	);
	assert.notEqual(inquiryRequestKey("c-2", form), key);
});

test("帶秒的時間：預設「現在」精確到秒，解析時保留秒（整分的預設會早於同一分鐘內剛建立的詢問）", () => {
	const now = new Date("2026-10-19T16:51:37.900Z");
	assert.equal(toLocalInputSeconds(now), "2026-10-20T00:51:37");
	assert.equal(
		fromLocalInput("2026-10-20T00:51:37")?.toISOString(),
		"2026-10-19T16:51:37.000Z",
	);
	assert.equal(
		fromLocalInput("2026-10-20T00:51")?.toISOString(),
		"2026-10-19T16:51:00.000Z",
		"沒有秒就是整分",
	);
	assert.equal(fromLocalInput("2026-10-20T00:51:3"), undefined);
	assert.equal(fromLocalInput(""), undefined);
	const request = buildInquiryReply(
		"i-1",
		{ ...emptyReplyForm(toLocalInputSeconds(now)), body: "x" },
		[],
	);
	assert.equal(
		Number(request.receiveTime?.seconds),
		Math.floor(now.getTime() / 1000),
	);
});

test("回覆的請求：管道、收到的時間（台北）、內容去掉前後空白、staged token 原樣", () => {
	const form = {
		...emptyReplyForm("2026-10-19T09:30"),
		channel: CaseInquiryReplyChannel.PHONE,
		body: " 電話回覆 ",
	};
	const request = buildInquiryReply("i-1", form, ["t-1", "t-2"]);
	assert.equal(request.id, "i-1");
	assert.equal(request.channel, CaseInquiryReplyChannel.PHONE);
	assert.equal(request.body, "電話回覆");
	assert.deepEqual(request.evidenceTokens, ["t-1", "t-2"]);
	assert.equal(
		Number(request.receiveTime?.seconds),
		Date.parse("2026-10-19T09:30:00+08:00") / 1000,
	);
	assert.ok(
		!(
			"receiveTime" in
			buildInquiryReply("i-1", { ...form, receiveTime: "" }, [])
		),
	);
});
