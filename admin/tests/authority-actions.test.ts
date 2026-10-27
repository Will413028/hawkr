import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	AuthorityHoldExtensionCause,
	AuthorityOrderKind,
	AuthorityOrderSchema,
	AuthorityOrderState,
	AuthorityStoreCoverage,
	AuthorityTargetNoticeKind,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import {
	buildComplete,
	buildCreateAuthorityOrder,
	buildExtend,
	buildNotice,
	buildRecordNotice,
	buildRevoke,
	EMPTY_NOTICE_FORM,
	emptyCompleteForm,
	parseProductIds,
	TARGET_NOTICE_REQUIRED_TEXT,
	validateComplete,
} from "../src/app/admin/authority-orders/authority-actions";
import {
	authorityCoverageText,
	authorityKindText,
	authorityNoticeKindText,
	authorityStateText,
	holdExtensionText,
	holdUntilText,
	measureStoreOptions,
	namedStoresText,
	targetLabel,
} from "../src/app/admin/authority-orders/authority-text";

const at = (local: string) => Date.parse(`${local}:00+08:00`) / 1000;

test("名稱：種類、狀態、告知方式逐字；涵蓋狀態的五個標籤（含 3c-b-3 核准的第四個與撤銷登記的第五個）", () => {
	assert.deepEqual(
		[
			AuthorityOrderKind.RESTRICT_PRODUCT,
			AuthorityOrderKind.DATA_REQUEST,
			AuthorityOrderKind.SUSPEND_ACCOUNT,
			AuthorityOrderKind.OTHER,
		].map(authorityKindText),
		["限制或移除商品", "調取資料", "暫停帳號", "其他"],
	);
	assert.equal(authorityStateText(1), "處理中");
	assert.equal(authorityStateText(2), "已完成");
	assert.equal(authorityStateText(AuthorityOrderState.REVOKED), "已撤銷登記");
	assert.deepEqual(
		[
			AuthorityStoreCoverage.HELD,
			AuthorityStoreCoverage.PURGED_BEFORE_REGISTRATION,
			AuthorityStoreCoverage.HOLD_ENDED,
			AuthorityStoreCoverage.PURGED_AFTER_HOLD_ENDED,
			AuthorityStoreCoverage.HOLD_RELEASED_BY_REVOCATION,
		].map(authorityCoverageText),
		[
			"保存中",
			"登記之前已依保留期限清除",
			"保存期限已過",
			"保存期限已過，個資已依保留期限清除",
			"撤銷登記時解除保存",
		],
	);
	assert.deepEqual(
		[
			AuthorityTargetNoticeKind.NOTIFIED,
			AuthorityTargetNoticeKind.WITHHELD,
			AuthorityTargetNoticeKind.MEASURE_IS_NOTICE,
		].map(authorityNoticeKindText),
		["已告知", "機關要求暫不告知", "由措施告知"],
	);
	assert.equal(authorityKindText(AuthorityOrderKind.UNSPECIFIED), "未知");
	assert.equal(
		authorityCoverageText(AuthorityStoreCoverage.UNSPECIFIED),
		"未知",
	);
});

test("登記的期限：不填就不帶（由伺服器依種類取預設，客戶端沒有預設常數）；填了才送", () => {
	const form = {
		agency: "a",
		documentNo: "d",
		receiveTime: "2026-10-19T09:00",
		kind: AuthorityOrderKind.SUSPEND_ACCOUNT,
		summary: "s",
		targets: [{ key: "k", accountId: "1", storeId: "", productIds: "" }],
		dueTime: "",
	};
	assert.equal("dueTime" in buildCreateAuthorityOrder("r", form), false);
	assert.equal(
		Number(
			buildCreateAuthorityOrder("r", { ...form, dueTime: "2026-10-21T09:00" })
				.dueTime?.seconds,
		),
		at("2026-10-21T09:00"),
	);
});

test("商品 ID：逗號（半形或全形）分隔，去掉空白與空項", () => {
	assert.deepEqual(parseProductIds(" p1, p2，,p3 ,"), ["p1", "p2", "p3"]);
	assert.deepEqual(parseProductIds(""), []);
});

test("登記的請求：欄位去掉前後空白；時間是台北；對象的帳號與店家與商品；不帶畫面用的列鍵", () => {
	const request = buildCreateAuthorityOrder("r-1", {
		agency: " 衛生局 ",
		documentNo: " 府衛字第 1 號 ",
		receiveTime: "2026-10-19T09:00",
		kind: AuthorityOrderKind.RESTRICT_PRODUCT,
		summary: " 請下架 ",
		targets: [
			{ key: "row-1", accountId: " 101 ", storeId: "", productIds: "p1, p2" },
			{ key: "row-2", accountId: "102", storeId: "55", productIds: "" },
		],
		dueTime: "2026-10-21T09:00",
	});
	assert.equal(request.requestId, "r-1");
	assert.equal(request.agency, "衛生局");
	assert.equal(request.documentNo, "府衛字第 1 號");
	assert.equal(request.summary, "請下架");
	assert.equal(Number(request.receiveTime?.seconds), at("2026-10-19T09:00"));
	assert.equal(Number(request.dueTime?.seconds), at("2026-10-21T09:00"));
	assert.deepEqual(request.targets, [
		{ accountId: "101", storeId: "", productIds: ["p1", "p2"] },
		{ accountId: "102", storeId: "55", productIds: [] },
	]);
});

test("告知：只送這個方式對應的欄位", () => {
	const form = {
		kind: AuthorityTargetNoticeKind.NOTIFIED,
		notifyTime: "2026-10-19T10:00",
		note: " 公文送達 ",
		recheckDate: "2026-11-01",
	};
	const notified = buildNotice(form) as Record<string, unknown>;
	assert.equal(notified.note, "公文送達");
	assert.equal(
		Number((notified.notifyTime as { seconds: bigint }).seconds),
		at("2026-10-19T10:00"),
	);
	assert.ok(!("recheckDate" in notified), "已告知不帶確認日期");

	const withheld = buildNotice({
		...form,
		kind: AuthorityTargetNoticeKind.WITHHELD,
	}) as Record<string, unknown>;
	assert.deepEqual(withheld.recheckDate, { year: 2026, month: 11, day: 1 });
	assert.ok(!("notifyTime" in withheld), "暫不告知不帶告知時間");

	assert.deepEqual(
		buildNotice({ ...form, kind: AuthorityTargetNoticeKind.MEASURE_IS_NOTICE }),
		{ kind: AuthorityTargetNoticeKind.MEASURE_IS_NOTICE },
	);
});

test("完成：每個對象都要選告知方式（沿用核准的伺服器句子）；請求依對象位置，調取資料才帶提供資料的時間", () => {
	const form = emptyCompleteForm(2);
	assert.equal(validateComplete(form), TARGET_NOTICE_REQUIRED_TEXT);
	const filled = {
		...form,
		resolutionSummary: " 已提供 ",
		provideTime: "2026-10-19T08:00",
		notices: [
			{
				...EMPTY_NOTICE_FORM,
				kind: AuthorityTargetNoticeKind.MEASURE_IS_NOTICE,
			},
			{
				...EMPTY_NOTICE_FORM,
				kind: AuthorityTargetNoticeKind.WITHHELD,
				recheckDate: "2026-11-01",
			},
		],
	};
	assert.equal(validateComplete(filled), undefined);
	const data = buildComplete(
		{ id: "o-1", etag: "e-1", kind: AuthorityOrderKind.DATA_REQUEST },
		filled,
	);
	assert.equal(data.id, "o-1");
	assert.equal(data.etag, "e-1");
	assert.equal(data.resolutionSummary, "已提供");
	assert.equal(Number(data.provideTime?.seconds), at("2026-10-19T08:00"));
	assert.deepEqual(
		data.targetNotices?.map((n) => [n.targetPosition, n.notice?.kind]),
		[
			[0, AuthorityTargetNoticeKind.MEASURE_IS_NOTICE],
			[1, AuthorityTargetNoticeKind.WITHHELD],
		],
	);
	assert.deepEqual(
		(data.targetNotices?.[1].notice as { recheckDate?: unknown }).recheckDate,
		{
			year: 2026,
			month: 11,
			day: 1,
		},
	);
	const other = buildComplete(
		{ id: "o-1", etag: "e-1", kind: AuthorityOrderKind.OTHER },
		filled,
	);
	assert.ok(
		!("provideTime" in other),
		"其他種類不帶提供資料的時間（伺服器會拒絕）",
	);
});

test("延長保存與記下告知的請求帶來函的 etag", () => {
	const extend = buildExtend(
		{ id: "o-1", etag: "e-2" },
		{ holdUntil: "2027-04-19T00:00", reason: " 進入訴訟 " },
	);
	assert.equal(extend.etag, "e-2");
	assert.equal(extend.reason, "進入訴訟");
	assert.equal(Number(extend.holdUntil?.seconds), at("2027-04-19T00:00"));
	const notice = buildRecordNotice({ id: "o-1", etag: "e-3" }, 1, {
		...EMPTY_NOTICE_FORM,
		kind: AuthorityTargetNoticeKind.MEASURE_IS_NOTICE,
	});
	assert.equal(notice.etag, "e-3");
	assert.equal(notice.targetPosition, 1);
});

test("清單與詳情的句子：指名的店家（L7）、保存（L16）、延長歷史（M3）、建立措施的店家選項（L20）", () => {
	const order = create(AuthorityOrderSchema, {
		targets: [
			{
				accountId: "101",
				storeId: "55",
				storeName: "阿里山農場",
				coveredStores: [
					{
						storeId: "55",
						storeName: "阿里山農場",
						coverage: AuthorityStoreCoverage.HELD,
						canCreateMeasure: true,
						canCreateTermination: true,
					},
				],
			},
			{
				accountId: "102",
				storeId: "",
				coveredStores: [
					{
						storeId: "60",
						storeName: "",
						coverage: AuthorityStoreCoverage.PURGED_AFTER_HOLD_ENDED,
					},
					{
						storeId: "61",
						storeName: "玉山果園",
						coverage: AuthorityStoreCoverage.HELD,
						canCreateMeasure: true,
					},
				],
			},
		],
	});
	assert.equal(targetLabel(order.targets[0]), "阿里山農場");
	assert.equal(targetLabel(order.targets[1]), "帳號 102 的全部店家");
	// n 是不重複的店家數（3 家），不是對象數（2 個）。
	assert.equal(namedStoresText(order), "阿里山農場 等 3 家");
	// 一個整帳號的對象涵蓋 2 家：不是「帳號 102 的全部店家 等 1 家」，而是第一家的名稱加店家數。
	assert.equal(namedStoresText({ targets: [order.targets[1]] }), "60 等 2 家");
	// 只有一家店：就是它的名稱（整帳號對象也一樣）。
	const single = create(AuthorityOrderSchema, {
		targets: [
			{
				accountId: "103",
				storeId: "",
				coveredStores: [
					{
						storeId: "70",
						storeName: "梨山茶園",
						coverage: AuthorityStoreCoverage.HELD,
					},
				],
			},
		],
	});
	assert.equal(namedStoresText(single), "梨山茶園");
	// 沒有店家資料：第一個對象的名稱。
	const none = create(AuthorityOrderSchema, {
		targets: [{ accountId: "104", storeId: "", noSellerData: true }],
	});
	assert.equal(namedStoresText(none), "帳號 104 的全部店家");
	assert.equal(namedStoresText({ targets: [] }), "-");
	assert.equal(
		holdUntilText("2027/04/19 00:00", true),
		"保存到 2027/04/19 00:00（保存中）",
	);
	assert.equal(
		holdUntilText("2027/04/19 00:00", false),
		"保存到 2027/04/19 00:00（已到期）",
	);
	// 撤銷登記之後保存已解除：日期還在未來也不能寫「已到期」。
	assert.equal(
		holdUntilText("2027/04/19 00:00", false, true),
		"保存到 2027/04/19 00:00（撤銷登記時解除）",
	);
	const row = {
		createTime: "2026/10/19 10:00",
		adminId: "9001",
		fromTime: "2027/04/21 00:00",
		toTime: "2027/10/19 00:00",
	};
	assert.equal(
		holdExtensionText({
			...row,
			cause: AuthorityHoldExtensionCause.ADMIN,
			reason: "進入訴訟",
		}),
		"2026/10/19 10:00・9001：保存到 2027/04/21 00:00 → 2027/10/19 00:00（進入訴訟）",
	);
	assert.equal(
		holdExtensionText({
			...row,
			cause: AuthorityHoldExtensionCause.DATA_PROVIDED,
			reason: "",
		}),
		"2026/10/19 10:00・9001：保存到 2027/04/21 00:00 → 2027/10/19 00:00（自動延長：提供資料之後 6 個月）",
	);
	assert.deepEqual(
		measureStoreOptions(order, "canCreateMeasure").map((o) => [
			o.storeId,
			o.accountId,
			o.label,
		]),
		[
			["55", "101", "101・阿里山農場"],
			["61", "102", "102・玉山果園"],
		],
		"只列伺服器說能建立措施的店（60 已清除、旗標為假）",
	);
	assert.deepEqual(
		measureStoreOptions(order, "canCreateTermination").map((o) => o.storeId),
		["55"],
		"終止合作用自己的旗標",
	);
});

test("撤銷登記的請求：id、etag，原因去掉前後空白（長度由伺服器判斷）", () => {
	assert.deepEqual(
		buildRevoke({ id: "o-1", etag: "e-1" }, "  對象登記錯了\u3000"),
		{
			id: "o-1",
			etag: "e-1",
			reason: "對象登記錯了",
		},
	);
});
